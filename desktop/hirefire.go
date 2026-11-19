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
	kept, _, err = fireEngineHireSaid(h, testPort, o, force)
	return kept, err
}

// fireEngineHireSaid is fireEngineHire, with what the core's looks at a Linux desk's places no mount holds found and
// did (said, deskKept), for your card: in its notes and in the fire's answer. Those looks are the one change a refusal
// may leave: what the desk's guard stopped it for is put back, as its next start would.
func fireEngineHireSaid(h Hire, testPort int, o engineOptions, force bool) (kept, said string, err error) {
	o = o.withDefaults()
	// What the first look put back, said in every refusal after it: no later look finds it again (RT126L1GUARD1FIRE B1).
	wasPutBack := ""
	refuse := func(message string) (string, string, error) {
		return "", "", &hireRefusal{Status: http.StatusConflict, Message: joinNote(message, wasPutBack)}
	}
	firing.Lock()
	if firing.ids[h.ID] {
		defer firing.Unlock()
		return refuse(beingFiredAlready(h))
	}
	if firing.takingOver[h.ID] {
		firing.Unlock()
		return refuse(t3BeingTakenOver(h))
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	// Read again once this fire holds the hire: a Take over that finished meanwhile left it in T3 Code, or
	// waiting for Hand back, and it's handed back first (RM5c2 N1). A hire whose row already says gone is only
	// stopped, if its session still runs (gonedesk.go): its worktree, recap and row are left as they are.
	// One taken over whose worktree turned since its hire (hireTreeTurned) can't be handed back (t3HandBack refuses
	// it first), so its fire goes ahead: what's left of its tmux desk is ended, and its worktree kept with no git run
	// and nothing done there, as for any fire of a moved tree; T3 Code's project is yours to remove (FXGMHIRENOGIT126R8).
	gone, takenOver := false, false
	for _, now := range readRegistry(registryPath(), false).hires {
		if now.ID == h.ID && (now.State == "fired" || now.State == "failed") {
			h.State, gone = now.State, true
		} else if now.ID == h.ID && now.T3Session != "" {
			if !hireTreeTurned(now) {
				return refuse(t3FireTakenOver(now))
			}
			h, takenOver, o.KeepWorktree = now, true, true
		}
	}
	unreached := "Couldn't reach " + calledHire(h) + "'s tmux session, so nothing was changed."
	// A desk whose row says gone with nothing left running is looked at too before it's said to be gone
	// (L1GUARD1FIRE): a fire whose look refused asked for this retry, and its session is gone by then, its socket
	// with it once its stop ran (RMSYLCHAIN B1). A put back is said; a look that refuses still refuses.
	goneAlready := func() (string, string, error) {
		// What it started may still run in its sandbox: ended first, as at every fire (FXFIREREAP).
		if why := deskReap(h); why != "" {
			return refuse(firedRefusal(h, why))
		}
		looked, refused := deskKept(h, "fired")
		if refused != "" {
			return refuse(refused)
		}
		return refuse(joinNote(alreadyGone(h), keptSaid(h, looked)))
	}
	tmux := hireOwnTmux(h, testPort, o.Tmux)
	if tmux == nil && gone {
		return goneAlready()
	}
	// Take over ended its tmux session (a sandboxed desk's socket folder with it): with no socket of its own left,
	// nothing of it runs there to end.
	if tmux == nil && !takenOver {
		return refuse(unreached)
	}
	up, known := false, true
	if tmux != nil {
		up, known = tmux.alive()
	}
	if !known {
		return refuse(unreached)
	}
	if gone && !up {
		return goneAlready()
	}
	// What the window shows is read before anything is ended, and kept for the recap.
	var capture string
	readable := true
	if up {
		capture, readable = tmux.capture(200)
	}
	// A Linux desk's places no mount holds (L1GUARD1FIRE): what its guard stopped it for is put back before git runs
	// in its worktree, then looked at again once its session is gone. One that can't be put back refuses the fire. A
	// desk whose row already says gone is looked at too: its session still ran (GMEDIT1).
	looked, refused := deskKept(h, "fired")
	if refused != "" {
		return refuse(refused)
	}
	wasPutBack = keptSaid(h, keptOnce(looked, nil))
	hasTree := !h.Worktree.none()
	if hasTree && !force && !gone && !takenOver {
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
		if ReadStyledPaneState(capture) == PaneIdle {
			if !ok || ReadStyledPaneState(fresh) != PaneIdle {
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
	// Its session is gone, but what it started in the background may not be: on a Mac, every program still running in
	// its sandbox is ended now, and seen ended, before anything looks at its worktree or checks the repo's objects
	// (FXFIREREAP). One that can't be refuses the fire: nothing is checked, merged or removed.
	if why := deskReap(h); why != "" {
		return refuse(firedRefusal(h, why))
	}
	again, refused := deskKept(h, "fired")
	if refused != "" {
		if gone {
			// Its session was ended and seen gone: SA-M6's stop runs before the refusal, as it would below, so its
			// sandbox isn't left kept. The retry the refusal asks for puts back at goneAlready (RMSYLCHAIN B1). A
			// program that stop couldn't end is said too (FXFIREREAPR3).
			if why := deskStopped(h, o); why != "" {
				refused = joinNote(refused, firedRefusal(h, why))
			}
		}
		return refuse(refused)
	}
	said = keptSaid(h, keptOnce(looked, again))
	if gone {
		// Its session was ended and seen gone: as at every stop, its sandbox is let go and its socket folder taken away
		// (SA-M6). Its row stays as it was, and it stays off CLAUDEPLANT1's record (GMEDIT1). What the looks found is
		// said with it (L1GUARD1FIRE), and a program its stop couldn't end (FXFIREREAPR3).
		stopWords := deskStopped(h, o)
		stopped := stoppedGone(h, "tmux session")
		if stopWords != "" {
			stopped.Message = joinNote(stopped.Message, firedRefusal(h, stopWords))
		}
		stopped.Message = joinNote(stopped.Message, said)
		return "", said, stopped
	}
	if refused := heldFire(h, "session"); refused != nil {
		refused.Message = joinNote(refused.Message, said)
		return "", "", refused
	}
	// Before the merged look (the branch goes only when merged), the repo's stored objects are hashed again for a
	// worktree that is the hire's own: changed ones, or a look that can't be made, keep the worktree and the branch,
	// and say so (firefsck.go).
	// The removal's git in the desk's sandbox runs after this look, and can't run before it: this look decides whether
	// the worktree is removed at all, and the removal checks the tree in the desk's sandbox again only once it has
	// moved it aside, where no desk reaches (deskrun.go, removeDeskTree). That git is the hardened one (gitReadArgs:
	// no hook, fsmonitor, pager or helper), and its commands start nothing that outlives them; deskRun waits for it,
	// and the last stop's reap below looks again (FXFIREREAPR4 O3).
	storeKept := ""
	if hasTree && !o.KeepWorktree {
		if _, _, own := hiresOwnWorktree(o.Git, o.Worktrees, h); own {
			storeKept = fireStoreKept(h, o.StoreLook)
		}
	}
	removed := storeKept == "" && hasTree && !o.KeepWorktree && removeHireWorktree(o.Git, testPort, o.Worktrees)(h)
	// A moved tree's line, whether its removal was asked or not: the usual advice would run git where a link leads
	// (FXGMHIRENOGIT126R8's found 8).
	writeRecapMoved(h, paneText(capture), removed, hasTree && !removed && (takenOver || hireTreeMoved(h)), o.Now())
	// Its session is known to have ended: what's in its place is listed once more, and it's on record (CLAUDEPLANT1).
	// That listing comes first, while its place is as the desk left it: the folders below go only after it.
	worktreeKept := hasTree && !removed && !o.KeepWorktree
	if takenOver {
		said = joinNote(t3FireTurnedWords(h), said)
	}
	if worktreeKept && storeKept != "" {
		markFired(h.ID, joinNote(storeKept, said))
	} else if worktreeKept {
		kept = firedKept(o.Git, h)
		markFired(h.ID, joinNote(firedKeptNote(h.Worktree.storedOnly(), kept), said))
	} else {
		markFired(h.ID, said)
	}
	claudeHireEnded(h.ID)
	// Then its stop's listing, while its place is still as the desk left it, its sandbox let go and its socket
	// folder taken away (SA-M6). A program that stop couldn't end (the reap above found none, but one started
	// meanwhile) keeps its sandbox, and is said in the fire's answer (FXFIREREAPR3).
	if why := deskStopped(h, o); why != "" {
		said = joinNote(said, firedStopNote(why))
	}
	// Its own Claude folder goes with its session (SA-X8): its transcripts too, as NB6 plans.
	_ = removeDeskClaude(supportDir, h.ID)
	// A hire without a worktree, and the GM, keep their plans in a folder others share: its own go now (MSYSAX8).
	// Smolcorp's own folder, never a plant: it goes after the listing, so a .claude a desk made in it is held first.
	if !hasTree {
		removeHirePlans(h)
	}
	// A GM's workspace goes with its other files (FXGMROOT): its notes, which only it wrote.
	if h.Role == gmRole {
		removeGMWorkspace(h.ID)
	}
	if worktreeKept && storeKept != "" {
		return storeKept, joinNote(storeKept, said), nil
	}
	return kept, said, nil
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
	stopped := "" // its row already said gone and only its session was ended (gonedesk.go): cleaned up as a fire
	_, said, err := fireEngineHireSaid(h, testCopyPort(), options, force)
	if err != nil {
		refused, ok := err.(*hireRefusal)
		if ok && refused.Status == http.StatusOK {
			stopped = refused.Message // what the looks found is in it (fireEngineHireSaid)
		} else {
			d.failHeldMail(h, err)
			if ok {
				return refused.Status, refused.Message, true
			}
			return http.StatusInternalServerError, err.Error(), true
		}
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
	if stopped != "" {
		return http.StatusOK, stopped, true
	}
	if note := joinNote(heldFireNote(h), said); note != "" {
		return http.StatusOK, note, true
	}
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
	kept, said, err := fireEngineHireSaid(h, testCopyPort(), options, force)
	d.failHeldMail(h, err)
	if err == nil {
		note = joinNote(heldFireNote(h), said)
	}
	return kept, note, err
}

// tmuxDeskRunning is whether h's tmux session runs on its own server: one whose recorded socket isn't there, or
// isn't exactly its own, has no server Smolcorp could reach or end, and none is said to run.
func (d *Deck) tmuxDeskRunning(h Hire) (running, known bool) {
	d.mu.Lock()
	run := d.engine.Tmux
	d.mu.Unlock()
	tmux := hireOwnTmux(h, testCopyPort(), run)
	if tmux == nil {
		return false, true
	}
	return tmux.alive()
}

func ownTmuxTab(h Hire, socket string) bool {
	found, ok := engineHireAt(socket)
	return ok && found.ID == h.ID
}
