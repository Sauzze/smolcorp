//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A hire in a window of its own, not in tmux (a 1.24 Swift-made hire in a Terminal tab): the engine's hire, its row
// then as the Swift engine wrote it, with no tmux socket. tab is its session's, in its worktree ("" for none); the
// Mac app is a stand-in answering closed with fired.
func ownWindowHire(t *testing.T, tab, closed string) (*Deck, *gmRig, *fakeHost) {
	t.Helper()
	g := newGMRig(t, "Rex")
	g.changeRow(func(h *Hire) { h.TmuxSocket, h.State = "", "working" })
	g.hire.TmuxSocket = ""
	fake := withHost(t, bridgeAnswer{OK: true, Fired: closed})
	if tab != "" {
		g.d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "rex-1",
			"cwd": g.hire.Worktree}, Claude, tab, "", nowMillis())
	}
	return g.d, g, fake
}

func closedTabs(fake *fakeHost) []bridgeRequest {
	var closes []bridgeRequest
	for _, asked := range fake.asked {
		if asked.Action == "close" {
			closes = append(closes, asked)
		}
	}
	return closes
}

func TestTheGMFiresAHireInAWindowOfItsOwnByClosingItsTab(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if w := g.fire(t, true, nil); w.Code != http.StatusNoContent {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if closes := closedTabs(fake); len(closes) != 1 || closes[0].Tab != "tty:/dev/ttys009" || closes[0].Agent != Claude {
		t.Errorf("the app was asked %+v, want one close of its tab", fake.asked)
	}
	if got := registered(t)[0]; got.State != "fired" || got.Notes != "" {
		t.Errorf("row %+v, want fired with no note", got)
	}
	if _, err := os.Stat(filepath.Join(supportDir, "recaps", g.hire.ID+".md")); err != nil {
		t.Errorf("no recap: %v", err)
	}
	if _, err := os.Stat(g.hire.Worktree); err != nil {
		t.Errorf("the worktree went though nobody asked: %v", err)
	}
	if d.sessions.Get("rex-1") != nil {
		t.Error("its session is still on a desk")
	}
	if len(g.asked()) != 0 {
		t.Errorf("tmux was asked %q", g.asked())
	}
}

func TestAHireInAWindowOfItsOwnKeepsTheRefusalsThatNeedNoWindow(t *testing.T) {
	t.Run("uncommitted", func(t *testing.T) {
		_, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
		os.WriteFile(filepath.Join(g.hire.Worktree, "draft.txt"), []byte("half"), 0o600)
		w := g.fire(t, true, nil)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Rex has changes that aren't committed: ?? draft.txt") {
			t.Fatalf("fire: %d %q", w.Code, w.Body)
		}
		if len(closedTabs(fake)) != 0 || registered(t)[0].State != "working" {
			t.Errorf("closed %+v, row %+v", fake.asked, registered(t)[0])
		}
	})
	t.Run("starting", func(t *testing.T) {
		_, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
		g.changeRow(func(h *Hire) { h.State = "starting" })
		if w := g.fire(t, true, map[string]any{"force": true}); w.Code != http.StatusConflict || w.Body.String() != "Rex is still starting. Try again in a moment." {
			t.Fatalf("fire: %d %q", w.Code, w.Body)
		}
		if len(closedTabs(fake)) != 0 {
			t.Errorf("closed %+v", fake.asked)
		}
	})
	t.Run("not stopped", func(t *testing.T) {
		_, g, _ := ownWindowHire(t, "tty:/dev/ttys009", "not-stopped")
		if w := g.fire(t, true, map[string]any{"force": true}); w.Code != http.StatusConflict || w.Body.String() != "Couldn't stop the agent, so its tab is left open." {
			t.Fatalf("fire: %d %q", w.Code, w.Body)
		}
		if registered(t)[0].State != "working" {
			t.Errorf("row %+v, want it still working", registered(t)[0])
		}
	})
}

// With no tab to close (none in its worktree, or the tab has gone), nothing changes and it says so; force only marks it fired.
func TestAHireInAWindowOfItsOwnWithNoTabIsMarkedFiredOnlyWithForce(t *testing.T) {
	for _, c := range []struct{ name, tab, closed string }{{"no session", "", ""}, {"tab gone", "tty:/dev/ttys009", "gone"}} {
		t.Run(c.name, func(t *testing.T) {
			_, g, fake := ownWindowHire(t, c.tab, c.closed)
			w := g.fire(t, true, nil)
			want := "Rex runs in a window of its own, not in tmux, and Smolcorp can't find that window now, so nothing was changed. " +
				"If it's still open, close it there. Then fire again with force to mark Rex fired."
			if w.Code != http.StatusConflict || w.Body.String() != want {
				t.Fatalf("fire: %d %q", w.Code, w.Body)
			}
			if registered(t)[0].State != "working" {
				t.Fatalf("row %+v, want it still working", registered(t)[0])
			}
			if w := g.fire(t, false, map[string]any{"force": true}); w.Code != http.StatusNoContent {
				t.Fatalf("fire with force: %d %q", w.Code, w.Body)
			}
			if registered(t)[0].State != "fired" {
				t.Errorf("row %+v, want fired", registered(t)[0])
			}
			if c.tab == "" && len(closedTabs(fake)) != 0 {
				t.Errorf("closed %+v with no tab", fake.asked)
			}
		})
	}
}

func TestRemovingTheWorktreeOfAHireInAWindowOfItsOwnGoesAsTheEnginesDoes(t *testing.T) {
	_, g, _ := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if w := g.fire(t, true, map[string]any{"removeWorktree": true}); w.Code != http.StatusNoContent {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if _, err := os.Stat(g.hire.Worktree); !os.IsNotExist(err) {
		t.Errorf("the clean worktree is still there: %v", err)
	}
}

// The office's fire of its desk: the same, the worktree kept, as the Mac's office keeps it.
func TestTheOfficeFiresAHireInAWindowOfItsOwnFromItsDesk(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if status, message := d.Perform("fire", "rex-1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("fire: %d %q", status, message)
	}
	if len(closedTabs(fake)) != 1 || registered(t)[0].State != "fired" {
		t.Errorf("closed %+v, row %+v", fake.asked, registered(t)[0])
	}
	if _, err := os.Stat(g.hire.Worktree); err != nil {
		t.Errorf("the worktree went: %v", err)
	}
}

// Two sessions in one worktree: neither tab is closed on a guess.
func TestTwoSessionsInAWorktreeAreNeitherTakenForTheHiresTab(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "rex-2",
		"cwd": g.hire.Worktree}, Claude, "tty:/dev/ttys010", "", nowMillis())
	if w := g.fire(t, true, nil); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "can't find that window") {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if len(closedTabs(fake)) != 0 {
		t.Errorf("closed %+v", fake.asked)
	}
}
