package desktop

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// someClaudeSessions writes Claude Code's list of sessions as the test needs
// it: one named projects-28 with its socket, two that share a name, and a key
// file beside them that must never matter.
func someClaudeSessions(t *testing.T) {
	t.Helper()
	saved := claudeSessionsDir
	t.Cleanup(func() { claudeSessionsDir = saved })
	claudeSessionsDir = t.TempDir()
	writeFile(t, filepath.Join(claudeSessionsDir, "100.json"),
		`{"pid": 100, "sessionId": "s-recipient", "name": "projects-28", "messagingSocketPath": "/tmp/cc-socks/100.sock"}`)
	writeFile(t, filepath.Join(claudeSessionsDir, "200.json"), `{"pid": 200, "sessionId": "s-twin-a", "name": "twin"}`)
	writeFile(t, filepath.Join(claudeSessionsDir, "201.json"), `{"pid": 201, "sessionId": "s-twin-b", "name": "twin"}`)
	writeFile(t, filepath.Join(claudeSessionsDir, "100.abc.key"), "not json")
	// Too big to be a session's: never read, though it names projects-28 too.
	writeFile(t, filepath.Join(claudeSessionsDir, "300.json"),
		`{"pid": 300, "sessionId": "s-huge", "name": "projects-28", "padding": "`+strings.Repeat("x", 70<<10)+`"}`)
}

func TestASendMessageTargetIsFoundAmongClaudesSessions(t *testing.T) {
	someClaudeSessions(t)
	for target, want := range map[string]string{
		"projects-28":                  "s-recipient",
		"projects-28 [65584e]":         "s-recipient",
		"uds:/tmp/cc-socks/100.sock":   "s-recipient",
		"uds:/somewhere/else/100.sock": "s-recipient", // by its pid
		"twin":                         "",            // two share it: no guess
		"nobody":                       "",
		"":                             "",
	} {
		if got := claudePeer(target); got != want {
			t.Errorf("%q is %q, want %q", target, got, want)
		}
	}
}

func TestASessionMessagingAnotherIsWalkedOver(t *testing.T) {
	someClaudeSessions(t)
	d := newTestDeck(t)
	// Each in a launcher of its own, as sessions run on Windows and Linux.
	start := func(id, folder, launcher string) {
		d.launcherConnected(launcher, Claude, folder, "")
		d.Ingest(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": id, "cwd": folder},
			"claude", "launcher:"+launcher, "", 100)
	}
	start("s-sender", "/work/api", "L1")
	start("s-recipient", "/work/web", "L2")
	send := func(to string) {
		d.Ingest(map[string]any{"hook_event_name": "PreToolUse", "session_id": "s-sender", "cwd": "/work/api",
			"tool_name": "SendMessage", "tool_input": map[string]any{"to": to, "message": "hello"}}, "claude", "launcher:L1", "", 200)
	}
	send("twin")
	if s := d.sessions.Get("s-sender"); s.MessagedAt != 0 || s.Messaged != "" {
		t.Fatalf("a message to a name two share marked the sender with %v, %q", s.MessagedAt, s.Messaged)
	}
	send("projects-28")
	var state struct {
		Keys []struct {
			Session *struct {
				ID         string   `json:"id"`
				MessagedAt *float64 `json:"messagedAt"`
				Messaged   *string  `json:"messaged"`
			} `json:"session"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(d.snapshot(), &state); err != nil {
		t.Fatal(err)
	}
	for _, key := range state.Keys {
		if key.Session == nil || key.Session.ID != "s-sender" {
			continue
		}
		if key.Session.MessagedAt == nil || *key.Session.MessagedAt == 0 || key.Session.Messaged == nil || *key.Session.Messaged != "s-recipient" {
			t.Fatalf("the sender's desk says messagedAt %v, messaged %v; want now and s-recipient", key.Session.MessagedAt, key.Session.Messaged)
		}
		return
	}
	t.Fatal("the sender has no desk")
}
