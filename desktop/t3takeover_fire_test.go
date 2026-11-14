//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// RM5b S3, through the route: a hire stopped and waiting for Hand back isn't fired by Setup or the GM.
func TestT3AHireWaitingForHandBackIsntFiredThroughTheRoute(t *testing.T) {
	// Through the route, as the page (Setup) and the GM call it, force or not: refused, nothing asked of tmux, and the
	// hire stays, Hand back still offered.
	for _, asGM := range []bool{false, true} {
		g := newGMRig(t, "Rex")
		g.changeRow(func(h *Hire) { h.T3Session = t3TakeOverSession })
		w := g.fire(t, asGM, map[string]any{"force": true})
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Rex was taken over in T3 Code: hand it back first, then fire it.") || len(g.asked()) != 0 {
			t.Errorf("as the GM %v: %d %q, tmux asked %q", asGM, w.Code, w.Body, g.asked())
		}
		if h, _ := registryHire(g.hire.ID); !isOpen(h) || h.T3Session == "" {
			t.Errorf("%+v", h)
		}
	}
}

// RM5c2 N1: a fire racing a Take over. While Take over holds the hire, the fire is refused with nothing ended; a fire
// whose hire was read before a Take over finished reads the registry again once it holds the hire, and finds it
// taken over.
func TestT3AFireRacingATakeOverIsRefused(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	firing.Lock()
	firing.takingOver[hire.ID] = true
	firing.Unlock()
	_, err := fireEngineHire(hire, 0, r.o, true)
	firing.Lock()
	delete(firing.takingOver, hire.ID)
	firing.Unlock()
	if err == nil || err.Error() != "Janitor is being taken over in T3 Code right now, so it wasn't fired: try again once that's done." {
		t.Errorf("while taken over: %v", err)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires[0].T3Session = t3TakeOverSession }); err != nil {
		t.Fatal(err)
	}
	if _, err := fireEngineHire(hire, 0, r.o, true); err == nil || err.Error() != "Janitor was taken over in T3 Code: hand it back first, then fire it." {
		t.Errorf("taken over meanwhile: %v", err)
	}
	if r.tmux.ran("kill-server") || r.tmux.ran("send-keys") || !r.tmux.up || !isOpen(registered(t)[0]) {
		t.Errorf("something was ended: %q", r.tmux.asked)
	}
}

// RM5c2 N2: the registry's edit doesn't mark a hire taken over fired or failed (Hand back would never be offered
// again, and T3 Code's project would stay); its notes and nickname still change, and once handed back it can be.
func TestT3ARegistryEditDoesntMarkAHireTakenOverGone(t *testing.T) {
	isolate(t)
	h := Hire{ID: "abcdef123456", Name: "Ada", Role: "dev", Folder: "/w", State: "idle", Engine: t3Engine, T3Session: t3TakeOverSession}
	t3Registered(t, h)
	for _, state := range []string{"fired", "failed"} {
		_, refused := editHire(registryPath(), []byte(`{"id":"abcdef123456","state":"`+state+`"}`))
		if refused == nil || refused.status != http.StatusConflict || refused.message != "Ada was taken over in T3 Code: hand it back first, then mark it "+state+"." {
			t.Errorf("%s: %+v", state, refused)
		}
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"abcdef123456","notes":"x","nickname":"Ace"}`)); refused != nil {
		t.Errorf("notes: %+v", refused)
	}
	if after, _ := registryHire(h.ID); !isOpen(after) || after.T3Session == "" || after.Notes != "x" {
		t.Errorf("%+v", after)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires[0].T3Session, f.hires[0].Engine = "", "" }); err != nil {
		t.Fatal(err)
	}
	if got, refused := editHire(registryPath(), []byte(`{"id":"abcdef123456","state":"fired"}`)); refused != nil || got.State != "fired" {
		t.Errorf("handed back: %+v %+v", got, refused)
	}
}

// RMSY N1 and FXM5c, on the engine's stand-in tmux: Hand back's resume writes its start script under the resume's
// own name (so the next start's sweep knows a left one for a resume), and gives tmux's socket for Hand back's
// checked save rather than writing the registry itself.
func TestT3TheResumeNamesItsScriptAndGivesItsSocket(t *testing.T) {
	d, r, hire := hiredForFire(t, "")
	// Its tmux server gone, its socket with it.
	r.tmux.up = false
	_ = os.Remove(hire.TmuxSocket)
	before := registered(t)[0]
	socket, err := d.t3ResumeInTmux(hire, t3TakeOverSession, nil)
	if err != nil || socket == "" || !r.tmux.up {
		t.Fatalf("%q %v", socket, err)
	}
	// No script: its start is its ticket's, and its pane runs desk-start with that ticket alone (SG2).
	if scripts, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hire.ID), "*.sh")); len(scripts) != 0 {
		t.Errorf("resume scripts: %q", scripts)
	}
	plan, ok := issuedPlan(hire.ID)
	if !ok || !strings.Contains(strings.Join(plan.Words, " "), t3TakeOverSession) {
		t.Errorf("its ticket: %v %q", ok, plan.Words)
	}
	key := plan.Changes[slices.IndexFunc(plan.Changes, func(c envChange) bool { return c.Name == hireKeyEnvironment })].Value
	last := r.tmux.asked[len(r.tmux.asked)-1]
	if i := slices.Index(last, "desk-start"); i < 0 || len(last) != i+4 || key == "" || strings.Contains(strings.Join(last, " "), key) {
		t.Errorf("its pane's command: %q", last)
	}
	if after := registered(t)[0]; after.TmuxSocket != before.TmuxSocket {
		t.Errorf("written here: %+v", after)
	}
}
