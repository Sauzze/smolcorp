//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A role's Claude hire at a desk chosen is made by the engine, as at the
// default desk (worktree, the hire's settings, a send key), and its session
// sits at the desk chosen, now and when the layout is recomputed.
func TestARoleHireAtAChosenDeskGoesThroughTheEngine(t *testing.T) {
	for _, role := range []string{"janitor", "gm"} {
		t.Run(role, func(t *testing.T) {
			d, r, h := engineDeck(t)
			d.engine.HeardFrom = func(Hire) bool { return true }
			d.mu.Lock()
			running(d, 1) // s0 at desk 0; desk 1 is the first free
			d.mu.Unlock()
			if status, message := d.Perform("hire", "", url.Values{"role": {role}, "desk": {"4"}}); status != http.StatusNoContent {
				t.Fatalf("hire: %d %s", status, message)
			}
			d.watching.Wait()
			got := registered(t)
			if len(got) != 1 || got[0].Role != role || got[0].Worktree == "" || !r.tmux.ran("new-session") || len(h.tmux) != 0 {
				t.Fatalf("not made by the engine: registry %+v, tmux %q, old tmux %q", got, r.tmux.asked, h.tmux)
			}
			settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, got[0].ID), hireSettingsFile))
			if role == "gm" {
				d.mu.Lock()
				token := d.gmToken
				d.mu.Unlock()
				if token == "" || !strings.Contains(r.script(got[0]), token) || !strings.Contains(string(settings), "--profile gm") {
					t.Errorf("the GM at a chosen desk has no token or no GM profile: %s", settings)
				}
			} else if scriptSendKey(r.script(got[0])) == "" || strings.Contains(string(settings), "--profile") {
				t.Errorf("the hire has no send key or the wrong profile: %s", settings)
			}
			// Its session appears, and takes the desk chosen.
			d.mu.Lock()
			defer d.mu.Unlock()
			d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": got[0].Folder},
				Claude, "tmux:%1@"+got[0].TmuxSocket, "", got[0].Started+1)
			for range 3 {
				keys, _ := d.layout()
				if seat := seatOfSession(keys, "hired"); seat != 4 {
					t.Fatalf("the hire is at desk %d, want 4", seat)
				}
			}
			if len(d.engineSeats) != 0 {
				t.Errorf("the desk is kept after it was taken: %v", d.engineSeats)
			}
		})
	}
}

// Upstairs too: an engine hire is on the floor and at the desk chosen.
func TestARoleHireUpstairsGoesThroughTheEngineAndSitsOnItsFloor(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}, "floor": {"2"}, "desk": {"3"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 || !r.tmux.ran("new-session") {
		t.Fatalf("not made by the engine: registry %+v", got)
	}
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": got[0].Folder},
		Claude, "tmux:%1@"+got[0].TmuxSocket, "", got[0].Started+1)
	d.layout()
	floor, seat := d.floorOf["hired"], d.seatOf["hired"]
	d.mu.Unlock()
	if floor != 2 || seat != (Seat{2, 3}) {
		t.Errorf("the hire is on floor %d at %v, want floor 2, desk 3", floor, seat)
	}
}

// The launcher's engine (Windows, beta) passes the desk chosen to the hire's
// token, so the launcher's session sits there.
func TestALauncherEngineHireAtAChosenDeskSitsThere(t *testing.T) {
	r := newLRig(t)
	r.launches = false // the window is not opened by the rig: the test connects it
	r.o.Within, r.o.Every = 2*time.Second, 5*time.Millisecond
	r.o.HeardFrom = func(Hire) bool { return true }
	r.d.launcherEngine = r.o
	r.d.mu.Lock()
	running(r.d, 1)
	status, message, handled := r.d.managedLauncherHire("janitor", r.repo, hireOpener{}, true, 5, 0)
	r.d.mu.Unlock()
	if !handled || status != http.StatusNoContent {
		t.Fatalf("%d %q %v", status, message, handled)
	}
	r.d.mu.Lock()
	var token string
	for kept, pending := range r.d.hires {
		if pending.hireID == "" || pending.desk != 5 {
			t.Fatalf("the pending hire is %+v", pending)
		}
		token = kept
	}
	r.d.mu.Unlock()
	if token == "" {
		t.Fatal("no hire was expected")
	}
	r.d.watching.Wait()
	r.d.launcherConnected("LX", Claude, r.repo, "janitor", token)
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	r.d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": r.repo}, Claude, "launcher:LX", "", 200)
	if keys, _ := r.d.layout(); seatOfSession(keys, "hired") != 5 {
		t.Errorf("the launcher hire is at desk %d, want 5", seatOfSession(keys, "hired"))
	}
}
