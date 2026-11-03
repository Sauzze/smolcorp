//go:build !windows

package desktop

import (
	"net/http"
	"path/filepath"
	"strings"
)

// Fire, for a hire the engine made (Phase 6 T602, part C3), as the Mac's
// HireDesk.fire. It ends the hire only when its window is idle with nothing
// typed, and its worktree holds nothing uncommitted. At a question or a menu,
// or when the window can't be read, it always refuses; with a draft, at work,
// or a window that can't be told apart, or with uncommitted files, it refuses
// unless force is given. A refusal changes nothing: nothing here ever answers
// a prompt or types into the window (hard rules 1 and 2), not even with
// force: the session is only ended. When in doubt it refuses. It ends the
// session on the hire's own socket only (hireOwnTmux), looks again to see it
// is gone, and the worktree goes only through removeHireWorktree.

// fireEngineHire fires h: a refusal says why and leaves everything as it was.
// Otherwise its session is ended (and seen to be gone), its worktree removed
// when clean (kept says why not), its recap written and it is marked fired.
func fireEngineHire(h Hire, testPort int, o engineOptions, force bool) (kept string, err error) {
	o = o.withDefaults()
	refuse := func(message string) (string, error) {
		return "", &hireRefusal{Status: http.StatusConflict, Message: message}
	}
	firing.Lock()
	if firing.ids[h.ID] {
		firing.Unlock()
		return refuse(calledHire(h) + " is being fired already.")
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	for _, now := range readRegistry(registryPath(), false).hires {
		if now.ID == h.ID && (now.State == "fired" || now.State == "failed") {
			return refuse(calledHire(h) + " is already gone.")
		}
	}
	unreached := "Couldn't reach " + calledHire(h) + "'s tmux session, so nothing was changed."
	tmux := hireOwnTmux(h, testPort, o.Tmux)
	if tmux == nil {
		return refuse(unreached)
	}
	up, known := tmux.alive()
	if !known {
		return refuse(unreached)
	}
	// What the window shows is read before anything is ended, and kept for the recap.
	var capture string
	readable := true
	if up {
		capture, readable = tmux.capture(200)
	}
	if h.Worktree != "" && !force {
		if message := uncommittedRefusal(h, o.Git); message != "" {
			return refuse(message)
		}
	}
	if up {
		if message := windowRefusal(h, capture, readable, force); message != "" {
			return refuse(message)
		}
		// Git may have taken a while: look again just before ending, so a question that came up meanwhile stops it.
		fresh, ok := tmux.capture(200)
		if message := windowRefusal(h, fresh, ok, force); message != "" {
			return refuse(message)
		}
		tmux.end()
		// A kill that failed is not a fire: the session must be known to be gone.
		if still, known := tmux.alive(); !known || still {
			return refuse("Couldn't end " + calledHire(h) + "'s tmux session, so nothing was changed.")
		}
	}
	removed := h.Worktree != "" && removeHireWorktree(o.Git, testPort, o.Worktrees)(h)
	writeRecap(h, capture, removed, o.Now())
	if h.Worktree != "" && !removed {
		kept = "it has changes, or git wouldn't remove it"
		markFired(h.ID, "Its worktree "+h.Worktree+" was kept: "+kept+".")
		return kept, nil
	}
	markFired(h.ID, "")
	return "", nil
}

// engineHireAt is the registry's hire whose recorded tmux socket is the tab's,
// the folders on the way resolved: whatever way the tab spells it, an engine
// hire is never taken for another tab. A row counts as an engine hire's when
// its socket is named like one (isHireSocket's name check implies it, so
// checking it too would add nothing), so a socket that has gone is refused by
// the fire, not closed the old way.
func engineHireAt(socket string) (Hire, bool) {
	if socket == "" || !filepath.IsAbs(socket) {
		return Hire{}, false
	}
	same := func(a, b string) bool {
		return filepath.Join(physical(filepath.Dir(filepath.Clean(a))), filepath.Base(a)) ==
			filepath.Join(physical(filepath.Dir(filepath.Clean(b))), filepath.Base(b))
	}
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.TmuxSocket != "" && same(h.TmuxSocket, socket) &&
			strings.HasPrefix(filepath.Base(h.TmuxSocket), hireSocketPrefix) {
			return h, true
		}
	}
	return Hire{}, false
}

// fireTab is /act?action=fire for a tab: handled is true when the tab is an
// engine hire's, whatever the answer.
func (d *Deck) fireTab(tab string, force bool) (status int, message string, handled bool) {
	kind, _, socket := splitTab(tab)
	if kind != "tmux" {
		return 0, "", false
	}
	h, ok := engineHireAt(socket)
	if !ok {
		return 0, "", false
	}
	d.mu.Lock()
	options := d.engine
	d.mu.Unlock()
	if _, err := fireEngineHire(h, testCopyPort(), options, force); err != nil {
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	// A fired GM's token stops working, and mail still waiting for the hire fails
	// now rather than on the pump's next round.
	if h.Role == gmRole {
		d.clearFiredGMToken()
	}
	if d.mail != nil {
		d.mail.failTo(h.ID)
	}
	d.mu.Lock()
	if d.sessions.Prune(func(t string) bool { return t != tab }, nowMillis()) {
		d.changed()
	}
	d.mu.Unlock()
	return http.StatusNoContent, "", true
}
