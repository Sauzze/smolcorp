//go:build !windows

package desktop

import (
	"net/http"
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
