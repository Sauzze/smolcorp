//go:build !windows

package desktop

import (
	"net/http"
)

// Fire, for a hire in a window of its own, not in tmux: one the Mac's Swift engine made in a Terminal tab, 1.24's
// default (hireInTmux off and no agent windows in the background), as its Deck.fire and HireDesk.settle did. First
// the refusals that need no look at a window (gone, still starting, changes nobody committed unless force); then its
// tab is closed the way the office closes any tab (on a Mac, the app's close, through the bridge); and only once it
// has closed, its worktree goes when that was asked and it's clean, its recap is written and it is marked fired.
// Its tab is the one session working in its worktree. A hire without a worktree can't be told from the owner's own
// session in that folder, and one whose tab has gone has nothing to close: then nothing is changed, and the answer
// says so; fired again with force, it is only marked fired (and its tab, if still open, is the owner's to close).
// Only the Mac has such hires: on Linux every hire is on its own tmux server, and Windows' are its launcher's.

// inOwnWindow says whether h is a hire in a window of its own: no tmux server, no launcher and not in T3 Code.
func inOwnWindow(h Hire) bool { return h.TmuxSocket == "" && !h.ViaLauncher && h.Engine == "" }

// ownWindowNotFound is the refusal for a hire in a window of its own whose tab Smolcorp can't find.
func ownWindowNotFound(h Hire) string {
	return calledHire(h) + " runs in a window of its own, not in tmux, and Smolcorp can't find that window now, so nothing was changed. " +
		"If it's still open, close it there. Then fire again with force to mark " + calledHire(h) + " fired."
}

// ownWindowTab is the tab of h's session: the one session working in its worktree, else "".
func (d *Deck) ownWindowTab(h Hire) (tab string, agent Agent) {
	if h.Worktree == "" {
		return "", ""
	}
	tree := physical(h.Worktree)
	d.mu.Lock()
	all := d.sessions.All()
	d.mu.Unlock()
	found := 0
	for _, s := range all {
		if s.Folder != "" && physical(s.Folder) == tree || s.Project != "" && physical(s.Project) == tree {
			tab, agent = s.Tab, s.Agent
			found++
		}
	}
	if found != 1 {
		return "", "" // two in one worktree: neither is closed on a guess
	}
	return tab, agent
}

// fireOwnWindowHire fires h (inOwnWindow): a refusal says why and leaves everything as it was. tab is its session's,
// "" when none was found; keep keeps its worktree. kept says why a worktree asked to go stayed.
func (d *Deck) fireOwnWindowHire(h Hire, tab string, agent Agent, force, keep bool) (kept string, err error) {
	refuse := func(status int, message string) (string, error) {
		return "", &hireRefusal{Status: status, Message: message}
	}
	firing.Lock()
	if firing.ids[h.ID] {
		firing.Unlock()
		return refuse(http.StatusConflict, calledHire(h)+" is being fired already.")
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	// The registry is the truth, not the copy the caller holds.
	now, found := hireByID(h.ID)
	switch {
	case !found || now.State == "fired" || now.State == "failed":
		return refuse(http.StatusConflict, calledHire(h)+" is already gone.")
	case now.State == "starting":
		return refuse(http.StatusConflict, calledHire(h)+" is still starting. Try again in a moment.")
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
	closed := false
	if tab != "" {
		switch status, message := d.inTab(tab, func(t Tab) error { return t.Close(agent) }); {
		case status < http.StatusMultipleChoices:
			closed = true
		case status == http.StatusNotFound || status == http.StatusGone:
			// Its tab or its agent has gone already: nothing is left to close.
		default:
			return refuse(status, message) // not stopped, or a terminal Smolcorp can only watch: it stays open
		}
	}
	if !closed && !force {
		return refuse(http.StatusConflict, ownWindowNotFound(h))
	}
	removed := h.Worktree != "" && !keep && removeOwnWorktree(o.Git, o.Worktrees, h)
	writeRecap(h, "", removed, o.Now())
	if h.Worktree != "" && !removed && !keep {
		markFired(h.ID, firedKeptNote(h.Worktree))
		kept = firedWorktreeKept
	} else {
		markFired(h.ID, "")
	}
	if tab != "" {
		d.mu.Lock()
		if d.sessions.Prune(func(t string) bool { return t != tab }, nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
	}
	return kept, nil
}

// fireOwnWindowTab is the office's fire of a tab that is a hire's in a window of its own: handled is true then,
// whatever the answer. The office never asks for the worktree to go, so it's kept, as the Mac's office keeps it.
func (d *Deck) fireOwnWindowTab(tab string, force bool) (status int, message string, handled bool) {
	for _, h := range readRegistry(registryPath(), false).hires {
		if !inOwnWindow(h) || h.State == "fired" || h.State == "failed" {
			continue
		}
		mine, agent := d.ownWindowTab(h)
		if mine != tab {
			continue
		}
		if _, err := d.fireOwnWindowHire(h, tab, agent, force, true); err != nil {
			if refused, ok := err.(*hireRefusal); ok {
				return refused.Status, refused.Message, true
			}
			return http.StatusInternalServerError, err.Error(), true
		}
		d.afterGMFire(h, "")
		return http.StatusNoContent, "", true
	}
	return 0, "", false
}
