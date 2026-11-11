//go:build !windows

package desktop

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Fire, for a hire the engine made (Phase 6 T602, part C3), as the Mac's
// HireDesk.fire. It ends the hire only when its window is idle with nothing
// typed, and its worktree holds nothing uncommitted. At a question or a menu,
// or when the window can't be read, it always refuses; with a draft, at work,
// or a window that can't be told apart, or with uncommitted files, it refuses
// unless force is given. A refusal changes nothing. Nothing here ever answers
// a prompt (hard rules 1 and 2): only an idle, empty input box, looked at again
// just before, is typed into, and only with "/exit" and Return, so Claude Code
// ends its session itself and its SessionEnd hooks run (the event log's
// sessionStop), as the Mac's HireFire.fire does. At work or with a draft (with
// force) nothing is typed. Then, after the session ends or exitWithin, the
// hire's own tmux server is ended. All of it is on the hire's own socket only
// (hireOwnTmux), never a terminal window the owner types in; it looks again to
// see the session is gone, and the worktree goes only through removeHireWorktree.

// fireExitWithin is how long a fire waits for the session to end after "/exit" (HireFire.exitWithin).
const fireExitWithin = 10 * time.Second

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
	if firing.takingOver[h.ID] {
		firing.Unlock()
		return refuse(t3BeingTakenOver(h))
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	// Read again once this fire holds the hire: a Take over that finished meanwhile left it in T3 Code, or
	// waiting for Hand back, and it's handed back first (RM5c2 N1).
	for _, now := range readRegistry(registryPath(), false).hires {
		if now.ID == h.ID && (now.State == "fired" || now.State == "failed") {
			return refuse(calledHire(h) + " is already gone.")
		}
		if now.ID == h.ID && now.T3Session != "" {
			return refuse(t3FireTakenOver(now))
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
		// Git may have taken a while: look again just before typing or ending, so a question that came up meanwhile stops it.
		fresh, ok := tmux.capture(200)
		if message := windowRefusal(h, fresh, ok, force); message != "" {
			return refuse(message)
		}
		// Only an idle, empty box is typed into, and only if it still is: "/exit" and Return, then a bounded wait.
		if ReadPaneState(capture) == PaneIdle {
			if !ok || ReadPaneState(fresh) != PaneIdle {
				return refuse("Its window changed while it was being looked at, so nothing was sent. Try again.")
			}
			// Not into a mode of tmux's own (an owner's viewer scrolled back): the paste would go there. It is then only ended.
			if mode, known := tmux.pasteState(); known && !mode.inMode && tmux.paste("/exit") && tmux.submit() {
				waitForSessionEnd(tmux, o)
			}
		}
		if still, known := tmux.alive(); !known || still {
			tmux.endServer()
		}
		// An end that failed is not a fire: the session must be known to be gone.
		if still, known := tmux.alive(); !known || still {
			return refuse("Couldn't end " + calledHire(h) + "'s tmux session, so nothing was changed.")
		}
	}
	removed := h.Worktree != "" && !o.KeepWorktree && removeHireWorktree(o.Git, testPort, o.Worktrees)(h)
	writeRecap(h, capture, removed, o.Now())
	if h.Worktree != "" && !removed && !o.KeepWorktree {
		markFired(h.ID, firedKeptNote(h.Worktree))
		return firedWorktreeKept, nil
	}
	markFired(h.ID, "")
	return "", nil
}

// waitForSessionEnd waits up to fireExitWithin for the hire's session to end by itself after "/exit",
// looking every half second (HireFire.waitForEnd), on the clock and pause given.
func waitForSessionEnd(tmux *hireTmux, o engineOptions) {
	end := o.Now().Add(fireExitWithin)
	for {
		if up, known := tmux.alive(); known && !up || !o.Now().Before(end) {
			return
		}
		o.Pause(500 * time.Millisecond)
	}
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
// engine hire's, or a hire's in a window of its own, whatever the answer.
func (d *Deck) fireTab(tab string, force bool) (status int, message string, handled bool) {
	kind, _, socket := splitTab(tab)
	if kind != "tmux" {
		return d.fireOwnWindowTab(tab, force)
	}
	h, ok := engineHireAt(socket)
	if !ok {
		return d.fireOwnWindowTab(tab, force)
	}
	d.mu.Lock()
	options := d.engine
	options.Worktrees = d.worktreesFolder(options.Worktrees)
	d.mu.Unlock()
	if _, err := fireEngineHire(h, testCopyPort(), options, force); err != nil {
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	// Its windows' records go with it (hirewindows.go): it has none to close any more.
	forgetHireWindows(h.ID, windowsOfHire(h.ID))
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

// For the GM's fire route (gmfire.go): the engine's fire, or a hire's in a window of its own
// (hirefire_window.go, whose note says it was marked fired with no window closed), and whether a tab's socket is h's own.
func (d *Deck) fireTmuxHire(h Hire, force, keep bool) (kept, note string, err error) {
	if inOwnWindow(h) {
		// Its tab is never closed from here: only by folder could it be found, and that may be the owner's (RHIRE2 B1).
		return d.fireOwnWindowHire(h, "", "", force, keep)
	}
	d.mu.Lock()
	options := d.engine
	options.Worktrees = d.worktreesFolder(options.Worktrees)
	d.mu.Unlock()
	options.KeepWorktree = keep
	kept, err = fireEngineHire(h, testCopyPort(), options, force)
	return kept, "", err
}

func ownTmuxTab(h Hire, socket string) bool {
	found, ok := engineHireAt(socket)
	return ok && found.ID == h.ID
}
