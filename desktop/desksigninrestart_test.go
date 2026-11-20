//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// SA-X9 stacked on SA-X14: "Restart in the sandbox" asks the desk's sign-in as every start does, and what Take over
// finds of a desk once a restart made its Claude folder anew. Every claude here is X9's stand-in (signInStandIn); keys
// are fake, and nothing reads the real ~/.claude.

// newSessions is how many tmux new-session calls the rig's tmux was asked for since from.
func newSessions(r *engineRig, from int) int {
	return len(slices.DeleteFunc(slices.Clone(r.tmux.asked[from:]), func(args []string) bool {
		return !slices.Contains(args, "new-session")
	}))
}

// takeOverNoting makes Take over's route reachable (T3 Code on, a version it was proven on) with every way it could
// take a stand-in that notes its name and fails, and gives back what was noted: a Take over refused before it touches
// anything notes nothing.
func takeOverNoting(t *testing.T, d *Deck) *[]string {
	t.Helper()
	called := &[]string{}
	note := func(way string) { *called = append(*called, way) }
	kept := t3HandingStand
	t3HandingStand = func(*Deck, Hire) t3Handing {
		return t3Handing{
			Machine: t3Machine{Home: homeDir, GOOS: "darwin", Now: time.Now, Getenv: func(string) string { return "" }},
			Call: func(string, string, map[string]any) (map[string]any, error) {
				note("call")
				return nil, errors.New("not here")
			},
			Holding: func(string, string) ([]int, bool) { note("holding"); return nil, false },
			Name:    func(int) string { return "" },
			Stat:    func(string) (os.FileInfo, error) { note("stat"); return nil, os.ErrNotExist },
			Pause:   func(time.Duration) {}, Now: time.Now,
			Fence:  func() (map[string]any, error) { note("fence"); return nil, errors.New("not here") },
			Stop:   func(Hire, string) error { note("stop"); return errors.New("not here") },
			Resume: func(Hire, string) (t3Resumed, error) { note("resume"); return t3Resumed{}, errors.New("not here") },
			Save:   func(string, func(*Hire)) error { note("save"); return errors.New("not here") },
		}
	}
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{State: t3Found, Version: t3TakeOverVersions[0]}, time.Now()
	t3Kept.Unlock()
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	t.Cleanup(func() {
		t3HandingStand = kept
		t3Kept.Lock()
		t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
		t3Kept.Unlock()
	})
	return called
}

// A restart whose new Claude folder Claude Code says isn't signed in is refused with the engine start's words (409),
// after the stop, and leaves nothing behind: no sandbox, no tmux session, no Claude folder, the row as it was and not
// held. The folder asked is its new one, seeded afresh, never its old one nor a missing one (RT126SAX9STACK B1). Once
// signed in, Restart again starts it in its sandbox.
func TestARestartOfADeskThatIsntSignedInIsRefusedAndLeavesNothing(t *testing.T) {
	d, r, old, made := oldDeskRig(t)
	// Its old folder, marked: the desk could have written its .claude.json while it ran without its sandbox.
	if err := os.WriteFile(claudeJSON(deskClaudeFolder(supportDir, old.ID)), []byte(`{"old-desk-marker":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	seed, err := deskClaude{Trusted: physical(old.Worktree.storedOnly())}.seed()
	if err != nil {
		t.Fatal(err)
	}
	got := signInStandIn(t, `{"loggedIn": false, "authMethod": "none"}`)
	from := len(r.tmux.asked)
	status, why := d.restartInSandbox(old.ID)
	words, _ := why.(string)
	if status != http.StatusConflict || !strings.HasPrefix(words, signInYourHires+" ") ||
		!strings.Contains(words, "Janitor was stopped") || !strings.Contains(words, "press Restart in the sandbox again, or fire it") {
		t.Fatalf("%d %v", status, why)
	}
	lines := askedLines(t, got)
	if len(lines) != 4 || lines[0] != "args: auth status --json" || lines[1] != "config: "+deskClaudeFolder(supportDir, old.ID) || lines[2] != "secure: " {
		t.Fatalf("asked with %q", lines)
	}
	if fresh := strings.ReplaceAll(string(seed), "\n", ""); lines[3] != fresh {
		t.Fatalf("asked a folder other than its new one, seeded afresh: its .claude.json %q, not %q", lines[3], fresh)
	}
	if !r.tmux.ran("kill-session") {
		t.Error("not stopped: the check is after the stop, as the folder it asks is made there")
	}
	if len(*made) != 0 || newSessions(r, from) != 0 {
		t.Errorf("something of it ran: %d sandboxes, tmux %q", len(*made), r.tmux.asked[from:])
	}
	if _, err := os.Lstat(deskClaudeFolder(supportDir, old.ID)); !os.IsNotExist(err) {
		t.Errorf("its new Claude folder stayed: %v", err)
	}
	if rows := registered(t); len(rows) != 1 || !isOpen(rows[0]) || rows[0].Sandboxed || rows[0].TmuxSocket != old.TmuxSocket {
		t.Errorf("its row: %+v", rows)
	}
	firing.Lock()
	held := firing.ids[old.ID] || firing.restarting[old.ID]
	firing.Unlock()
	if held {
		t.Error("still held after the refusal")
	}

	// Signed in now: Restart again starts it, in its sandbox, with its own Claude folder.
	got = signInStandIn(t, `{"loggedIn": true, "authMethod": "claude.ai"}`)
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux
	if status, answer := d.restartInSandbox(old.ID); status != http.StatusOK {
		t.Fatalf("again, signed in: %d %v", status, answer)
	}
	if rows := registered(t); len(rows) != 1 || !rows[0].Sandboxed || len(*made) != 1 || !exists(deskClaudeFolder(supportDir, old.ID)) {
		t.Errorf("again, signed in: %+v, %d sandboxes", rows, len(*made))
	}
	if lines := askedLines(t, got); len(lines) != 4 || lines[1] != "config: "+deskClaudeFolder(supportDir, old.ID) {
		t.Errorf("again, asked with %q", lines)
	}
}

// What Take over finds after a restart (X14's removeDeskClaude took the desk's old transcripts with its old folder):
// the old session ended with the stop, so it's never handed over, and its transcript, gone, is never read from
// anywhere else; Take over refuses before anything is stopped or copied. Once the new Claude Code is heard from, its
// session is the one found, with its transcript in the desk's new folder.
func TestTakeOverAfterARestartFindsOnlyTheNewSessionAndRefusesBeforeTouchingAnything(t *testing.T) {
	d, _, old, _ := oldDeskRig(t)
	own := deskClaudeFolder(supportDir, old.ID)
	if !exists(own) {
		t.Fatalf("the old desk has no Claude folder of its own: %s", own)
	}
	session := func(id, socket, transcript string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(transcript, []byte(`{"`+id+`":1}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// As its hooks send it, with its own hook key.
		d.ingestAs(currentHookDesks(), old.ID, map[string]any{"hook_event_name": "SessionStart", "session_id": id, "cwd": old.Worktree.storedOnly(),
			"transcript_path": transcript}, "claude", "tmux:%1@"+socket, old.Worktree.storedOnly(), 0, "", nowMillis())
	}
	const before, after = "11111111-2222-4333-8444-555555555555", "66666666-7777-4888-9999-aaaaaaaaaaaa"
	oldTranscript := filepath.Join(own, "projects", "-old-place", before+".jsonl")
	session(before, old.TmuxSocket, oldTranscript)
	if got := d.t3SessionOf(old); got.ID != before || got.Transcript != oldTranscript {
		t.Fatalf("before the restart, Take over finds %+v", got)
	}
	if status, answer := d.restartInSandbox(old.ID); status != http.StatusOK {
		t.Fatalf("the restart: %d %v", status, answer)
	}
	row := registered(t)[0]
	if exists(oldTranscript) {
		t.Fatal("the old transcript is still there: X14's removal changed, and this row no longer says what it says")
	}
	// The old session is over: Take over finds none, and refuses.
	found := d.t3SessionOf(row)
	if found != (t3Session{}) {
		t.Errorf("after the restart, before the new Claude Code is heard from, Take over finds %+v", found)
	}
	if why := t3TakeOverRefusal(row, found, homeDir, false); why == "" {
		t.Error("Take over isn't refused with no session")
	} else {
		t.Logf("before the new session: %s", why)
	}
	// A late event of the old session doesn't bring it back to the desk.
	d.ingestAs(currentHookDesks(), old.ID, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": before, "cwd": old.Worktree.storedOnly(),
		"transcript_path": oldTranscript}, "claude", "tmux:%1@"+row.TmuxSocket, old.Worktree.storedOnly(), 0, "", 0)
	if got := d.t3SessionOf(row); got.ID == before {
		t.Errorf("the old session came back: %+v", got)
	}
	// Its gone transcript is never copied from, nor looked for elsewhere: Take over's copy refuses it.
	to := filepath.Join(homeDir, ".claude", "projects", "-old-place", before+".jsonl")
	if err := t3CopyIn(oldTranscript, to); err == nil || exists(to) {
		t.Errorf("the gone transcript was copied: %v", err)
	}
	// The new Claude Code, heard from: its session, in the desk's new folder, is the one found.
	newTranscript := filepath.Join(own, "projects", "-new-place", after+".jsonl")
	session(after, row.TmuxSocket, newTranscript)
	found = d.t3SessionOf(row)
	if found.ID != after || found.Transcript != newTranscript || !t3InOwnClaude(row, found.Transcript) || !t3OnDefaultAccount(row, found, homeDir) {
		t.Errorf("after the new session, Take over finds %+v", found)
	}
	// A desk restarted in its sandbox isn't one Take over reaches for now: refused at its tmux check, through the
	// route, before any way is called (RT126SAX9STACK F1).
	called := takeOverNoting(t, d)
	status, answer := d.takeOverIntoT3(old.ID)
	if words, _ := answer.(string); status != http.StatusConflict || len(*called) != 0 ||
		words != "Only a hire Smolcorp runs in a tmux session of its own can be taken over in T3 Code, and Janitor isn't one." {
		t.Errorf("with the new session, Take over: %d %v, ways called %q", status, answer, *called)
	}
}

// Take over after a restart refused for its sign-in (RT126SAX9STACK P1): the row keeps its old smolcorp- socket, so
// the tmux check doesn't stop it, but its old session ended with the stop, and a late event of it doesn't bring it
// back: refused through the route as not heard from, before any way is called, with no copy made and the row as it was.
func TestTakeOverAfterARestartRefusedForItsSignInIsRefusedBeforeTouchingAnything(t *testing.T) {
	d, _, old, _ := oldDeskRig(t)
	const before = "11111111-2222-4333-8444-555555555555"
	oldTranscript := filepath.Join(deskClaudeFolder(supportDir, old.ID), "projects", "-old-place", before+".jsonl")
	if err := os.MkdirAll(filepath.Dir(oldTranscript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldTranscript, []byte(`{"`+before+`":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.ingestAs(currentHookDesks(), old.ID, map[string]any{"hook_event_name": "SessionStart", "session_id": before, "cwd": old.Worktree.storedOnly(),
		"transcript_path": oldTranscript}, "claude", "tmux:%1@"+old.TmuxSocket, old.Worktree.storedOnly(), 0, "", nowMillis())
	if got := d.t3SessionOf(old); got.ID != before {
		t.Fatalf("before the restart, Take over finds %+v", got)
	}
	signInStandIn(t, `{"loggedIn": false, "authMethod": "none"}`)
	if status, why := d.restartInSandbox(old.ID); status != http.StatusConflict {
		t.Fatalf("the restart: %d %v", status, why)
	}
	row := registered(t)[0]
	d.ingestAs(currentHookDesks(), old.ID, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": before, "cwd": old.Worktree.storedOnly(),
		"transcript_path": oldTranscript}, "claude", "tmux:%1@"+row.TmuxSocket, old.Worktree.storedOnly(), 0, "", 0)
	called := takeOverNoting(t, d)
	status, answer := d.takeOverIntoT3(old.ID)
	if words, _ := answer.(string); status != http.StatusConflict || len(*called) != 0 ||
		words != "Smolcorp hasn't heard from Janitor's Claude Code yet, so it doesn't know which session to hand over." {
		t.Errorf("Take over: %d %v, ways called %q", status, answer, *called)
	}
	if exists(filepath.Join(homeDir, ".claude", "projects", "-old-place", before+".jsonl")) {
		t.Error("a copy of the old transcript was made in ~/.claude")
	}
	if r := registered(t)[0]; r.T3Session != "" || r.Engine != "" || r.TmuxSocket != old.TmuxSocket || r.Sandboxed {
		t.Errorf("its row changed: %+v", r)
	}
}
