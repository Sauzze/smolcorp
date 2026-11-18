//go:build !windows

package desktop

import (
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Belt and braces (MSYGMEDIT1): CLAUDEPLANT1's B4b guards (a gone hire goes on record only once its session ended;
// a gone hire whose own tmux session is up counts as open at the gate) and GMEDIT1's refusal (the edit never marks a
// running desk gone) each hold on their own. Each row takes the other away and shows its own still holds, on the
// engine rig's stand-in tmux. Stand-ins only: every plant holds `{"hooks":"standin"}` or `#!/bin/sh`.

// keepEveryClaudePlant keeps everything held so far (it's yours), so the next gate answers for what comes after.
func keepEveryClaudePlant() {
	claudeHeld.Lock()
	ids := slices.Collect(maps.Keys(claudeHeld.plants))
	claudeHeld.Unlock()
	for _, id := range ids {
		_ = removeClaudePlant(id, true)
	}
}

// claudeSessionUpOn makes the gate's look at a gone hire's session the real one, on r's stand-in tmux.
func claudeSessionUpOn(t *testing.T, r *engineRig) {
	upWas := claudeSessionUp
	claudeSessionUp = func(h Hire) bool { return hireSessionUp(h, testCopyPort(), r.o) }
	t.Cleanup(func() { claudeSessionUp = upWas })
}

// GMEDIT1's refusal taken away (the edit's look says nothing runs, as before GMEDIT1): the GM's edit marks a desk
// fired while its tmux session runs. CLAUDEPLANT1 still holds: the edit doesn't put it on record, and what it plants
// after the edit is listed at the gate, by each B4b guard alone.
func TestCLAUDEPLANT1sGateHoldsWithoutGMEDIT1sRefusal(t *testing.T) {
	g := newGMRig(t, "Ada")
	freshClaudeHeld(t)
	claudeSessionUpOn(t, g.r)
	// With GMEDIT1, the GM's own route refuses it.
	if w := g.post(t, true, "/setup/gm/registry", map[string]any{"id": g.hire.ID, "state": "fired"}); w.Code != http.StatusConflict {
		t.Fatalf("with GMEDIT1: %d %s", w.Code, w.Body)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"`+g.hire.ID+`","state":"fired"}`), noDeskSession); refused != nil {
		t.Fatal(refused.message)
	}
	h := registered(t)[0]
	if h.State != "fired" || !g.r.tmux.up || g.r.tmux.ran("kill-server") {
		t.Fatalf("not marked fired with its session running: %+v up=%v", h, g.r.tmux.up)
	}
	if claudeListed(h.ID) {
		t.Error("the GM's edit put it on record as listed")
	}
	if !claudeSessionUp(h) {
		t.Error("its session runs, and the gate's look doesn't see it")
	}
	keepEveryClaudePlant()
	claudeTree(t, h.place(), ".claude/hooks/x.sh")
	if why := claudeGate(h.place(), officeHires()); !strings.Contains(why, filepath.Join(h.place(), ".claude")) {
		t.Errorf("planted after the edit, the gate let it through: %q", why)
	}
	// Option 2 alone: put on record (option 1 away), its session up still counts as open.
	keepEveryClaudePlant()
	markClaudeListed(h.ID)
	claudeTree(t, h.place(), "a/.claude/settings.json")
	if why := claudeGate(h.place(), officeHires()); !strings.Contains(why, filepath.Join("a", ".claude")) {
		t.Errorf("on record, its session up: the gate let it through: %q", why)
	}
	// Option 1 alone: off the record, its session not seen (option 2 away), it's still listed.
	keepEveryClaudePlant()
	claudeHeld.Lock()
	delete(claudeHeld.ended, h.ID)
	claudeHeld.Unlock()
	claudeSessionUp = func(Hire) bool { return false }
	claudeTree(t, h.place(), "b/.mcp.json")
	if why := claudeGate(h.place(), officeHires()); !strings.Contains(why, filepath.Join(h.place(), "b", ".mcp.json")) {
		t.Errorf("off the record, its session not seen: the gate let it through: %q", why)
	}
}

// CLAUDEPLANT1's B4b guards taken away (the hire on record as ended, its session never seen up): GMEDIT1 still
// holds. The GM's edit is refused while the desk's tmux session runs, its row stays open, and so the gate lists what
// it plants. Without GMEDIT1 too, the same plant goes through: the guards really were away.
func TestGMEDIT1sRefusalHoldsWithoutCLAUDEPLANT1sGuards(t *testing.T) {
	guardsAway := func(t *testing.T) *gmRig {
		g := newGMRig(t, "Ada")
		freshClaudeHeld(t)
		upWas := claudeSessionUp
		claudeSessionUp = func(Hire) bool { return false }
		t.Cleanup(func() { claudeSessionUp = upWas })
		markClaudeListed(g.hire.ID)
		return g
	}
	t.Run("without either", func(t *testing.T) {
		g := guardsAway(t)
		if _, refused := editHire(registryPath(), []byte(`{"id":"`+g.hire.ID+`","state":"fired"}`), noDeskSession); refused != nil {
			t.Fatal(refused.message)
		}
		// The edit asks for its listing, and the worker lists it once the registry's lock is let go
		// (registryFile.saveAsking): done before the plant, as a fire's is, or a slow worker lists the plant too.
		if !waitClaudeListings(func(p claudePending) bool { return p.Desk == g.hire.ID }, claudeCheckWait) {
			t.Fatal("the edit's listing wasn't done")
		}
		keepEveryClaudePlant()
		h := registered(t)[0]
		claudeTree(t, h.place(), ".claude/hooks/x.sh")
		if why := claudeGate(h.place(), officeHires()); why != "" {
			t.Errorf("the guards weren't away: %q", why)
		}
	})
	for _, state := range []string{"fired", "failed"} {
		t.Run(state, func(t *testing.T) {
			g := guardsAway(t)
			w := g.post(t, true, "/setup/gm/registry", map[string]any{"id": g.hire.ID, "state": state})
			want := "Ada's tmux session is still running, so it wasn't marked " + state + ": fire Ada, which ends it first."
			if w.Code != http.StatusConflict || w.Body.String() != want {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			h := registered(t)[0]
			if !isOpen(h) || !g.r.tmux.up {
				t.Fatalf("a refused edit changed something: %+v", h)
			}
			keepEveryClaudePlant()
			claudeTree(t, h.place(), ".claude/hooks/x.sh")
			if why := claudeGate(h.place(), officeHires()); !strings.Contains(why, filepath.Join(h.place(), ".claude")) {
				t.Errorf("its row open, the gate let its plant through: %q", why)
			}
		})
	}
}
