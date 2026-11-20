//go:build !windows

package desktop

import (
	"testing"
)

// A fire without force reads a window Claude Code drew as it does on Windows: an idle one is ended, one with a
// draft refuses as a draft (never "couldn't tell"), and one that can't be read still refuses, even with force.
func TestAFireWithoutForceReadsAWindowDrawnAsClaudeCodeDrawsIt(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	w.screen.Write([]byte(claudeStream(t, "a draft, its slip mended")))
	r.fireRefused(h, false, "Janitor has something typed in its window that isn't sent yet. Send or clear it, or fire with force.")
	r.unchanged(h, w)
	w.screen.Write([]byte("\x1b[3b"))
	r.fireRefused(h, true, "Couldn't tell what Janitor's window holds, so nothing was ended. Look at it first.")
	r.unchanged(h, w)

	stopDecksOf(t)
	r = newLRig(t)
	h, w = r.hire("janitor")
	w.screen.Write([]byte(claudeStream(t, "done: the spinner gone, the box idle")))
	if kept, err := r.d.fireLauncherHire(h, r.o, false); err != nil || kept != "" {
		t.Fatalf("fire: %q %v", kept, err)
	}
	if got := registered(t)[0]; got.State != "fired" || w.stopped() != 1 || len(w.typed()) != 0 {
		t.Errorf("state %q, stops %d, typed %q", got.State, w.stopped(), w.typed())
	}
}
