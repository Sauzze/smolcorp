package desktop

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"
)

// savedFrom writes the sessions and keys of d as a save would, with savedAt
// set back by ago.
func savedFrom(t *testing.T, d *Deck, ago float64) {
	t.Helper()
	d.mu.Lock()
	d.layout()
	saved := savedDeck{SavedAt: nowMillis() - ago, Sessions: d.sessions.Saved(), Keys: map[string]int{}}
	for id, key := range d.keyOf {
		saved.Keys[id] = key
	}
	d.mu.Unlock()
	data, _ := json.Marshal(saved)
	writeFile(t, sessionsFile, string(data))
}

func TestAHeadlessRunIsBackAfterARestartWhileItsProcessIs(t *testing.T) {
	s := NewSessions()
	s.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "abc-123", "cwd": "/work/tool"}, Claude,
		"headless:abc-123", "", 100)
	s.Get("abc-123").PID, s.Get("abc-123").ParentFolder = 4242, "/elsewhere/tool"
	withPID := s.Saved()[0]
	if !stillRunning(withPID, func(int) bool { return false }) || stillRunning(withPID, func(int) bool { return true }) {
		t.Error("a headless run is back exactly while its process is")
	}
	withoutPID := withPID
	withoutPID.PID = 0
	if stillRunning(withoutPID, func(int) bool { return false }) {
		t.Error("with no process to look at, a headless run is back only at its next event")
	}
	restored := NewSessions()
	restored.Restore([]savedSession{withPID}, true)
	if r := restored.Get("abc-123"); r == nil || !r.Headless() || r.PID != 4242 || r.ParentFolder != "/elsewhere/tool" || r.State != Thinking {
		t.Errorf("restored as %+v", r)
	}
}

func TestAnOldSaveIsShownAtRest(t *testing.T) {
	s := NewSessions()
	s.Apply(event("UserPromptSubmit"), Claude, tab, "", 100)
	s.Apply(event("PreToolUse", "tool_name", "Bash", "model", "claude-opus-5-5"), Claude, tab, "", 120)
	restored := NewSessions()
	restored.Restore(s.Saved(), false)
	if r := restored.Get("s1"); r == nil || r.State != Idle || r.TurnStartedAt != 0 || r.Model != "claude-opus-5-5" || r.Tab != tab {
		t.Errorf("an old save restored as %+v, want it idle with what it knew", r)
	}
	fresh := NewSessions()
	fresh.Restore(s.Saved(), true)
	if r := fresh.Get("s1"); r == nil || r.State != Thinking || r.TurnStartedAt != 100 {
		t.Errorf("a fresh save restored as %+v, want it as it was", r)
	}
	// One heard from since, or ended since, is left as it is.
	fresh.Get("s1").State = Unread
	if fresh.Restore(s.Saved(), true) || fresh.Get("s1").State != Unread {
		t.Error("a restore went over a session heard from since")
	}
}

func TestSessionsInPanesAreBackOnTheirKeysAfterARestart(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 6
	d.config.save()
	running(d, 3)
	d.mu.Lock()
	d.layout()
	d.keyOf["s2"] = 4 // as if it had been carried there
	d.mu.Unlock()
	savedFrom(t, d, 1000)

	saved := paneRunsAgent
	t.Cleanup(func() { paneRunsAgent = saved })
	// s1's pane has closed meanwhile.
	paneRunsAgent = func(pane commandTab, _ Agent) bool { return pane.id != "%1" }
	after := NewDeck()
	deadline := time.Now().Add(2 * time.Second)
	for {
		after.mu.Lock()
		count := len(after.sessions.All())
		after.mu.Unlock()
		if count == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	after.mu.Lock()
	defer after.mu.Unlock()
	keys, _ := after.layout()
	if keys[0].session == nil || keys[0].session.ID != "s0" || keys[4].session == nil || keys[4].session.ID != "s2" ||
		after.sessions.Get("s1") != nil {
		t.Errorf("after a restart the keys are %+v", keys)
	}
	if after.sessions.Get("s0").State != Thinking {
		t.Errorf("a session saved a second ago is back as %s, want as it was", after.sessions.Get("s0").State)
	}
}

func TestALaunchersSessionIsBackWhenItConnectsAgain(t *testing.T) {
	d := newTestDeck(t)
	d.launcherConnected("L1", Claude, "/work/api", "tester", "")
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"},
		Claude, "launcher:L1", "", nowMillis())
	d.launcherConnected("L2", Claude, "/work/web", "", "")
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s2", "cwd": "/work/web"},
		Claude, "launcher:L2", "", nowMillis())
	savedFrom(t, d, 0)

	after := NewDeck()
	if len(after.sessions.All()) != 0 {
		t.Fatal("a launcher's session is back before its launcher is")
	}
	after.launcherConnected("L1", Claude, "/work/api", "tester", "")
	if s := after.sessions.Get("s1"); s == nil || s.State != Thinking || after.sessions.Get("s2") != nil {
		t.Fatalf("after L1 connected again: %v", after.sessions.All())
	}
	// L2 doesn't connect in time: its agent ended while the app was away.
	after.restoring.until = time.Now().Add(-time.Second)
	after.launcherConnected("L2", Claude, "/work/web", "", "")
	if after.sessions.Get("s2") != nil {
		t.Error("a launcher connecting after the wait brought back its old session")
	}
}

func TestTheSessionsAreSavedAMomentAfterAChange(t *testing.T) {
	d := newTestDeck(t)
	file := sessionsFile
	d.mu.Lock()
	d.sessions.Apply(event("UserPromptSubmit"), Claude, tab, "", nowMillis())
	d.changed()
	d.mu.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if data, err := os.ReadFile(file); err == nil {
			var saved savedDeck
			if json.Unmarshal(data, &saved) != nil || len(saved.Sessions) != 1 || saved.Sessions[0].ID != "s1" || saved.Keys["s1"] != 0 {
				t.Errorf("saved %s", data)
			}
			// Windows has no Unix modes: Go reports any writable file as 0666 there,
			// and what keeps the file yours is your profile folder's own permissions.
			if info, _ := os.Stat(file); runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
				t.Errorf("the saved sessions can be read by others: %v", info.Mode())
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the sessions weren't saved")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
