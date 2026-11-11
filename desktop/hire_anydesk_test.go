package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// hireAt hires at desk with the Auto layout, which is the only one there is
// since desks left setup, and seats the new session as its launcher would.
func hireAt(t *testing.T, d *Deck, h *hiring, folder, desk, id string, window int) {
	t.Helper()
	d.lastHire = time.Time{}
	query := url.Values{}
	if desk != "" {
		query.Set("desk", desk)
	}
	if status, message := d.Perform("hire", "", query); status != http.StatusNoContent {
		t.Fatalf("hire at %q: %d %s", desk, status, message)
	}
	_, token := hiredWith(t, h.windows[window])
	d.launcherConnected("L"+id, Claude, folder, "", token)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": id, "cwd": folder},
		Claude, "launcher:L"+id, "", 200)
}

func seatOfSession(keys []deckKey, id string) int {
	for i, k := range keys {
		if k.session != nil && k.session.ID == id {
			return i
		}
	}
	return -1
}

// With Auto a free desk in the middle, with earlier ones free, is the hire's.
func TestAutoHireSitsAtAFreeDeskChosenBeforeTheFirstFreeOne(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	running(d, 1) // s0 at desk 0; desks 1.. are free
	hireAt(t, d, h, folder, "4", "hired", 0)
	keys, _ := d.layout()
	if got := seatOfSession(keys, "hired"); got != 4 {
		t.Fatalf("the hire is at desk %d, want the one chosen, 4", got)
	}
	if len(keys) != 5 || keys[0].session == nil || keys[0].session.ID != "s0" {
		t.Errorf("the keys are %d, with s0 first: %v", len(keys), keys)
	}
	// The next refresh, another session coming in, and the layout again: it stays.
	running(d, 3)
	for range 3 {
		keys, _ = d.layout()
	}
	if got := seatOfSession(keys, "hired"); got != 4 {
		t.Errorf("after the layout was recomputed the hire is at desk %d, want 4", got)
	}
	// Someone else takes the first free desks, not the hire's.
	if seatOfSession(keys, "s1") != 1 || seatOfSession(keys, "s2") != 2 {
		t.Errorf("the others went to %d and %d, want the first free desks 1 and 2", seatOfSession(keys, "s1"), seatOfSession(keys, "s2"))
	}
	// A desk someone's at is refused, with the existing text.
	d.lastHire = time.Time{}
	status, message := d.Perform("hire", "", url.Values{"desk": {"4"}})
	if status != http.StatusConflict || message != "Someone's at that desk already." {
		t.Errorf("hire at a taken desk: %d %q", status, message)
	}
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"9"}}); status != http.StatusBadRequest {
		t.Errorf("hire at desk 9: %d", status)
	}
}

// A hire with no desk, or at the first free one, still goes to the first free.
func TestAutoHireWithNoDeskTakesTheNextFreeOne(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	running(d, 2)
	hireAt(t, d, h, folder, "", "hired", 0)
	if keys, _ := d.layout(); seatOfSession(keys, "hired") != 2 {
		t.Errorf("a hire at no desk is at %d, want 2", seatOfSession(keys, "hired"))
	}
	d.lastHire = time.Time{}
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"3"}}); status != http.StatusNoContent {
		t.Fatalf("hire at the desk after the last: %d", status)
	}
	if _, token := hiredWith(t, h.windows[1]); d.hires[token].desk != -1 {
		t.Errorf("the first free desk was held: %d", d.hires[token].desk)
	}
}

// The chosen desk is kept across a restart, and goes when its session does.
func TestAChosenDeskIsKeptAcrossARestartAndForgottenWithItsSession(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	d.sessionsFile = t.TempDir() + "/sessions.json"
	running(d, 1)
	hireAt(t, d, h, folder, "6", "hired", 0)
	d.layout()
	d.saveSessions()
	data, err := os.ReadFile(d.sessionsFile)
	if err != nil {
		t.Fatal(err)
	}
	var saved savedDeck
	if json.Unmarshal(data, &saved) != nil || len(saved.Chosen) != 1 || saved.Chosen[0] != "hired" || saved.Keys["hired"] != 6 {
		t.Fatalf("saved %s", data)
	}
	// An app that starts again: the same sessions come back on their saved places. With the lock held, as
	// the app holds it: the restore asks for a save, which reads the sessions and desks a moment later.
	again := newTestDeck(t)
	again.mu.Lock()
	defer again.mu.Unlock()
	again.restore([]savedSession{{ID: "s0", Tab: "tmux:%0@/tmp/tmux", Agent: string(Claude)}, {ID: "hired", Tab: "tmux:%7@/tmp/tmux", Agent: string(Claude)}},
		savedDeck{Keys: saved.Keys, Chosen: saved.Chosen}, false)
	for _, id := range []string{"s0", "hired"} {
		again.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/" + id}, Claude, "tmux:%"+id+"@/tmp/tmux", "", 300)
	}
	keys, _ := again.layout()
	if got := seatOfSession(keys, "hired"); got != 6 {
		t.Errorf("after a restart the hire is at desk %d, want 6", got)
	}
	// When it ends the desks pack again.
	again.sessions.Apply(map[string]any{"hook_event_name": "SessionEnd", "session_id": "hired"}, Claude, "tmux:%hired@/tmp/tmux", "", 400)
	keys, _ = again.layout()
	if len(keys) != 1 || again.chosenDesk["hired"] {
		t.Errorf("after it ended there are %d keys and its choice is kept: %v", len(keys), again.chosenDesk)
	}
}
