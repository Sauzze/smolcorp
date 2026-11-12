//go:build !windows

package desktop

import (
	"errors"
	"net/http"
)

// Fire, for a hire in a window of its own, not in tmux: one the Mac's Swift engine made in a Terminal tab, 1.24's
// default (hireInTmux off and no agent windows in the background), as its Deck.fire and HireDesk.settle did. First
// the refusals that need no look at a window (gone, changes nobody committed unless force); then, from the office's
// desk alone, where the owner points at it, its tab is closed through the Mac app, strictly (only with its agent in
// front there: a tab with only its shell left may be someone else's on a tty used again); and only once it has
// closed, its recap is written and it is marked fired, its worktree kept, as the Mac's office keeps it.
// The GM's and Setup's fires never close a tab (RHIRE2 B1). They could only find it by its folder, and the one
// session in the hire's worktree may be the owner's own Claude Code, opened there to look at the work, or the GM's:
// what this core has heard since it started can't tell it from the hire's, whose own session may never be heard
// again after an upgrade. They answer that the window can't be found and nothing was changed; fired again with force,
// it is marked fired, its worktree kept, and the answer says its window wasn't closed. A test copy never closes one:
// its tab is the owner's own.
// Only the Mac has such hires: on Linux every hire is on its own tmux server, and Windows' are its launcher's.

// inOwnWindow says whether h is a hire in a window of its own: no tmux server, no launcher and not in T3 Code.
func inOwnWindow(h Hire) bool { return h.TmuxSocket == "" && !h.ViaLauncher && h.Engine == "" }

// ownWindowNotFound is the refusal for a hire in a window of its own whose tab Smolcorp can't find.
func ownWindowNotFound(h Hire) string {
	return calledHire(h) + " runs in a window of its own, not in tmux, and Smolcorp can't find that window now, so nothing was changed. " +
		"If it's still open, close it there. Then fire again with force to mark " + calledHire(h) + " fired."
}

// ownWindowNotClosed is what a fire with force says when it closed no window.
func ownWindowNotClosed(h Hire) string {
	return calledHire(h) + " is marked fired, but Smolcorp couldn't find its window, so it wasn't closed. If it's still open, close it there."
}

// ownWindowKept is why the worktree stayed though its removal was asked: its window wasn't closed.
const ownWindowKept = "Its window wasn't closed, so it may still be at work in it."

// ownWindowTab is the tab of h's session: the one session working in its worktree, else "". Only the office's fire,
// where the owner points at the tab, goes by it.
func (d *Deck) ownWindowTab(h Hire) (tab string, agent Agent) {
	if h.Worktree == "" {
		return "", ""
	}
	tree := physical(h.Worktree)
	d.mu.Lock()
	defer d.mu.Unlock()
	var found *Session
	for _, s := range d.sessions.All() {
		if s.Folder != "" && physical(s.Folder) == tree || s.Project != "" && physical(s.Project) == tree {
			if found != nil {
				return "", "" // two in one worktree: neither is closed on a guess
			}
			found = s
		}
	}
	if found == nil {
		return "", ""
	}
	return found.Tab, found.Agent
}

// strictCloser is a tab the Mac app closes only with its agent in front (macTab).
type strictCloser interface{ CloseStrict(Agent) error }

// fireOwnWindowHire fires h (inOwnWindow): a refusal says why and leaves everything as it was. tab is its session's,
// "" when none was found; keep keeps its worktree. kept says why a worktree asked to go stayed; note, that it was
// marked fired with no window closed.
func (d *Deck) fireOwnWindowHire(h Hire, tab string, agent Agent, force, keep bool) (kept, note string, err error) {
	refuse := func(status int, message string) (string, string, error) {
		return "", "", &hireRefusal{Status: status, Message: message}
	}
	firing.Lock()
	if firing.ids[h.ID] {
		firing.Unlock()
		return refuse(http.StatusConflict, calledHire(h)+" is being fired already.")
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	// The registry is the truth, not the copy the caller holds. A row still "starting" is fired too: no start is in
	// flight for one (Swift moved it to working only at its first hook, and nothing here moves it).
	if now, found := hireByID(h.ID); !found || now.State == "fired" || now.State == "failed" {
		return refuse(http.StatusConflict, calledHire(h)+" is already gone.")
	}
	d.mu.Lock()
	o := d.engine
	o.Worktrees = d.worktreesFolder(o.Worktrees)
	d.mu.Unlock()
	o = o.withDefaults()
	if h.Worktree != "" && !force {
		if message := uncommittedRefusal(h, o.Git); message != "" {
			return refuse(http.StatusConflict, message)
		}
	}
	// A test copy never closes a tab of the owner's (RHIRE N8): testCopyKeepsTab, as inTab says it. The office's fire
	// is refused before this (Perform); this keeps any other way out, with nothing changed.
	if why := testCopyKeepsTab(tab); tab != "" && why != "" {
		return refuse(http.StatusConflict, why)
	}
	closed := false
	if tab != "" {
		var tried bool
		var closing error
		status, message := d.inTab(tab, func(t Tab) error {
			tried = true
			if closer, ok := t.(strictCloser); ok {
				closing = closer.CloseStrict(agent)
			} else {
				closing = errUnsupported // only the Mac app closes such a tab, and only strictly
			}
			return closing
		})
		switch {
		case !tried, errors.Is(closing, errTabClosed):
			// Its tab, or its agent, has gone already: nothing is left to close.
		case closing == nil:
			closed = true
		case errors.Is(closing, errNotStopped), errors.Is(closing, errUnsupported):
			return refuse(status, message) // not stopped, or a terminal Smolcorp can only watch: it stays open
		default:
			// The Mac app didn't answer, or not as asked: not the same as a window that has gone.
			return refuse(http.StatusBadGateway, "The Mac app didn't close "+calledHire(h)+"'s window, so nothing was changed. Try again in a moment.")
		}
	}
	if !closed && !force {
		return refuse(http.StatusConflict, ownWindowNotFound(h))
	}
	if !closed {
		// It may still be at work in a window nobody closed: its worktree stays, whatever was asked.
		note = ownWindowNotClosed(h)
		if h.Worktree != "" && !keep {
			kept = ownWindowKept
		}
		keep = true
	}
	removed := h.Worktree != "" && !keep && removeOwnWorktree(o.Git, o.Worktrees, h)
	writeRecap(h, "", removed, o.Now())
	switch {
	case note != "":
		markFired(h.ID, note)
	case h.Worktree != "" && !removed && !keep:
		markFired(h.ID, firedKeptNote(h.Worktree))
		kept = firedWorktreeKept
	default:
		markFired(h.ID, "")
	}
	if tab != "" {
		d.mu.Lock()
		if d.sessions.Prune(func(t string) bool { return t != tab }, nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
	}
	return kept, note, nil
}

// fireOwnWindowTab is the office's fire of a tab that is a hire's in a window of its own: handled is true then,
// whatever the answer. The office never asks for the worktree to go, so it's kept, as the Mac's office keeps it.
func (d *Deck) fireOwnWindowTab(tab string, force bool) (status int, message string, handled bool) {
	for _, h := range readRegistry(registryPath(), false).hires {
		if !inOwnWindow(h) || h.State == "fired" || h.State == "failed" {
			continue
		}
		mine, agent := d.ownWindowTab(h) // the owner points at this tab
		if mine != tab {
			continue
		}
		_, note, err := d.fireOwnWindowHire(h, tab, agent, force, true)
		if err != nil {
			if refused, ok := err.(*hireRefusal); ok {
				return refused.Status, refused.Message, true
			}
			return http.StatusInternalServerError, err.Error(), true
		}
		d.afterGMFire(h, "")
		if note != "" {
			return http.StatusOK, note, true
		}
		return http.StatusNoContent, "", true
	}
	return 0, "", false
}
