//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

// RT126CLAUDEPLANT1R2 B4b, option 1: a gone hire is put on record as listed only where its session is known to have
// ended (claudeHireEnded): the engine's fire, once its tmux session is seen gone; a launcher's fire, once its
// launcher is; a window fire whose tab the Mac app closed; a start that failed after its session was ended. A fire
// whose window nobody closed, or a failed start whose session couldn't be ended, leaves it off the record, so every
// gate near it lists it again. The rigs' stand-ins only: a stand-in tmux, launcher and Mac app.
func TestOnlyAGoneHireWhoseSessionEndedIsPutOnRecord(t *testing.T) {
	t.Run("the engine's fire", func(t *testing.T) {
		_, r, hire := hiredForFire(t, "")
		freshClaudeHeld(t)
		windowShows(r, pane(t, "idle-empty"), true)
		exitingPane(r, "0", true)
		if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
			t.Fatal(err)
		}
		if !claudeListed(hire.ID) {
			t.Error("fired, its session seen gone: not on record")
		}
	})
	for _, c := range []struct {
		name string
		ends bool
	}{{"a failed start, its session ended", true}, {"a failed start, its session not ended", false}} {
		t.Run(c.name, func(t *testing.T) {
			r := newEngineRig(t)
			freshClaudeHeld(t)
			engine := r.tmux.run
			r.o.Tmux = func(args []string) (string, int, bool) {
				if args[2] == "kill-session" && !c.ends {
					return "", 1, true
				}
				return engine(args)
			}
			r.o.Attach = func(string) error { return errors.New("standin: the window didn't open") }
			if _, err := runManagedHire(r.request("janitor"), r.o); err == nil {
				t.Fatal("hired")
			}
			got := registered(t)
			if len(got) != 1 || got[0].State != "failed" {
				t.Fatalf("registry %+v", got)
			}
			if claudeListed(got[0].ID) != c.ends {
				t.Errorf("on record: %v, want %v", claudeListed(got[0].ID), c.ends)
			}
		})
	}
	t.Run("a launcher's fire", func(t *testing.T) {
		r := newLRig(t)
		freshClaudeHeld(t)
		h, _ := r.hire("janitor")
		if _, err := r.d.fireLauncherHire(h, r.o, false); err != nil {
			t.Fatal(err)
		}
		if !claudeListed(h.ID) {
			t.Error("fired, its launcher seen gone: not on record")
		}
	})
	t.Run("a window fire, its tab closed", func(t *testing.T) {
		d, g, _ := ownWindowHire(t, "tty:/dev/ttys009", "fired")
		freshClaudeHeld(t)
		if status, message := d.Perform("fire", "rex-1", url.Values{}); status != http.StatusNoContent {
			t.Fatalf("fire: %d %q", status, message)
		}
		if !claudeListed(g.hire.ID) {
			t.Error("fired, its tab closed: not on record")
		}
	})
	t.Run("a window fire, its window nobody closed", func(t *testing.T) {
		_, g, _ := ownWindowHire(t, "", "")
		freshClaudeHeld(t)
		if w := g.fire(t, false, map[string]any{"force": true}); w.Code != http.StatusOK || registered(t)[0].State != "fired" {
			t.Fatalf("fire with force: %d %q", w.Code, w.Body)
		}
		if claudeListed(g.hire.ID) {
			t.Error("fired, its window maybe still at work: on record")
		}
	})
}
