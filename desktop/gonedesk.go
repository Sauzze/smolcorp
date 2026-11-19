package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"sort"
	"sync"
	"time"
)

// A desk whose row says gone but whose session still runs (GMEDIT1, from RT126CLAUDEPLANT1R2 B4b). The GM's or the
// page's edit (POST /setup/gm/registry) only changes a row: marking a running desk fired or failed there left its
// session, its agent and anything it started running, and the fire then refused it as already gone. So:
//   - the edit refuses fired or failed while the hire's session runs, while its start is under way, or while that
//     can't be told, and points to the fire (editHire);
//   - every start reads its row again before its session starts, and starts nothing behind a gone row (runManagedHire,
//     takeStartEnv and hiredPending, t3HireInto);
//   - the fire of a desk whose row already says gone ends its session if it still runs, sees it gone, and leaves the
//     row as it is (fireEngineHire, fireLauncherHire, t3Leave);
//   - at each start, every such desk is listed to the owner: in the event log, and on Setup (sweepGoneDesks).
// The session kinds are the fire's: a tmux session on the hire's own server, a launcher's window, a T3 Code agent. A
// hire in a window of its own can't be told from the owner's own session in its worktree (RHIRE2 B1), nor can a
// launcher's window that connected again without its id: the edit refuses both, and says to close the window there.

// goneDesksSwept is whether a new deck sweeps for such desks as it starts: the app's always does. The tests' decks
// don't (isolate), so the sweep never asks a real tmux about a test's socket; a test sweeps itself.
var goneDesksSwept = true

// eventKindGoneDesk is a start's line about a desk whose row says gone but whose session still runs.
const eventKindGoneDesk = "gonedesk"

// t3DeskWhat is what a T3 hire's look sees: any program working in its worktree, its agent or another (t3Leaver).
const t3DeskWhat = "T3 Code agent, or another program in its worktree,"

// deskLook is what a look at a hire's session found (Deck.deskSession): what names the session in the owner's words,
// whether it runs, whether that could be told, and whether its start is still under way. advice is what the owner
// can do when it can't be told, "" for the fire's way.
type deskLook struct {
	what                     string
	running, known, starting bool
	advice                   string
}

// deskStartsUnderWay are the hires whose start is under way in this core, marked before the start writes its row and
// kept until its session can be looked at (its tmux server up and its row read again, its launcher's token waiting) or
// the start is over. No look sees a session that isn't there yet, so the edit refuses meanwhile, in its look and
// again under the registry's lock (RT126GMEDIT1 B1). A T3 hire's is t3InFlight.
var deskStartsUnderWay = struct {
	sync.Mutex
	ids map[string]int
}{ids: map[string]int{}}

// deskStarting marks hire id's start under way, and gives back what ends the mark.
func deskStarting(id string) (done func()) {
	deskStartsUnderWay.Lock()
	deskStartsUnderWay.ids[id]++
	deskStartsUnderWay.Unlock()
	return func() {
		deskStartsUnderWay.Lock()
		if deskStartsUnderWay.ids[id]--; deskStartsUnderWay.ids[id] <= 0 {
			delete(deskStartsUnderWay.ids, id)
		}
		deskStartsUnderWay.Unlock()
	}
}

// startUnderWay is whether hire id's start is under way here (deskStarting, or t3IsHiring). It takes no deck lock,
// so the edit asks it under the registry's lock too.
func startUnderWay(id string) bool {
	deskStartsUnderWay.Lock()
	marked := deskStartsUnderWay.ids[id] > 0
	deskStartsUnderWay.Unlock()
	return marked || t3IsHiring(id)
}

// deskSession is whether hire h's session still runs, whatever its row says, as the fire that would end it looks.
func (d *Deck) deskSession(h Hire) deskLook {
	if startUnderWay(h.ID) {
		return deskLook{known: true, starting: true}
	}
	switch {
	case h.Engine == t3Engine:
		if h.Worktree.none() {
			return deskLook{known: true} // nothing to look in, and nothing Let go could end
		}
		still, looked := t3Leaver().Working(h.Worktree.storedOnly()) // a process look by the path: nothing runs there
		return deskLook{what: t3DeskWhat, running: len(still) > 0, known: looked}
	case h.ViaLauncher:
		if id, _ := d.launcherOf(h.ID); id != "" {
			return deskLook{what: "window", running: true, known: true}
		}
		if d.launcherWaitsFor(h.ID) {
			return deskLook{known: true, starting: true} // its window may connect, and start it, any moment
		}
		if d.idlessWindowIn(h) { // B2
			return deskLook{what: "window", advice: "A window connected again in its worktree without its id, and Smolcorp can't end that: close it there, then try again."}
		}
		return deskLook{what: "window", known: true}
	case h.TmuxSocket != "":
		running, known := d.tmuxDeskRunning(h)
		return deskLook{what: "tmux session", running: running, known: known}
	case h.Engine == "": // in a window of its own (inOwnWindow): never told from the owner's own session there (C1)
		return deskLook{what: "window", advice: "Close it there, then fire " + calledHire(h) + " with force."}
	}
	return deskLook{known: true}
}

// launcherWaitsFor is whether a launcher's token for hire id is still waiting for its window (expectLauncherAt).
func (d *Deck) launcherWaitsFor(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, pending := range d.hires {
		if pending.hireID == id && time.Since(pending.at) <= hiredTabWait {
			return true
		}
	}
	return false
}

// idlessWindowIn is whether a window the office opened connected again without its hire's id (its token was used up)
// in h's worktree, or its folder with none: it may be h's, and can't be told or ended from here (RT126GMEDIT1 B2).
func (d *Deck) idlessWindowIn(h Hire) bool {
	tree := physical(h.storedPlace()) // compared only
	if tree == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, tab := range d.launchers {
		if tab.office && tab.hireID == "" && within(tree, physical(tab.folder)) {
			return true
		}
	}
	return false
}

// goneWhileRunning is the edit's refusal to mark hire id in the registry at path fired or failed (state, as sent)
// while its session runs, its start is under way, or that can't be told; or nil. looked is whether an open row was
// looked at and found ended: the edit's save, under the registry's lock, goes ahead for that row alone. Looked at
// before the lock is taken: the look may ask tmux, the launchers (under the deck's lock) or the processes.
func goneWhileRunning(path string, state json.RawMessage, id string, session func(Hire) deskLook) (refused *refusal, looked bool) {
	var text string
	if state == nil || json.Unmarshal(state, &text) != nil || text != "fired" && text != "failed" {
		return nil, false
	}
	hires := readRegistry(path, false).hires
	i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == id })
	if i < 0 || !isOpen(hires[i]) || hires[i].T3Session != "" {
		return nil, false
	}
	h := hires[i]
	// While the records' key can't be read, no hire is let go, whatever runs (records.go's refusedWhileHeld): that
	// answer comes first, as for every other revocation (SK2), and no session is looked at (MSYGMEDIT1).
	if recordsHeldOnly() {
		return &refusal{http.StatusConflict, recordsHeldAnswer}, false
	}
	look := session(h)
	how := "fire " + calledHire(h)
	if h.Engine == t3Engine {
		how = "let " + calledHire(h) + " go from Setup's team list"
	}
	switch {
	case look.starting:
		return stillStarting(h, text), false
	case !look.known:
		advice := look.advice
		if advice == "" {
			advice = "To stop it, " + how + "."
		}
		return &refusal{http.StatusConflict, "Smolcorp can't tell whether " + calledHire(h) + "'s " + look.what + " is still running, so it wasn't marked " + text + ". " + advice}, false
	case look.running:
		return &refusal{http.StatusConflict, calledHire(h) + "'s " + look.what + " is still running, so it wasn't marked " + text + ": " + how + ", which ends it first."}, false
	}
	return nil, true
}

// stillStarting is the edit's refusal to mark h gone (as state) while its start is under way.
func stillStarting(h Hire, state string) *refusal {
	how := "fire " + calledHire(h)
	if h.Engine == t3Engine {
		how = "let " + calledHire(h) + " go"
	}
	return &refusal{http.StatusConflict, calledHire(h) + " is still starting, so it wasn't marked " + state + ": try again in a moment, or " + how + " once it's up."}
}

// hireRowOpen is whether hire id's row is there and open: a start reads it again right before its session starts.
func hireRowOpen(id string) bool {
	h, found := hireByID(id)
	return found && isOpen(h)
}

// goneWhileStarting is a start's refusal when h's row turned gone before its session started: what was done about it.
func goneWhileStarting(h Hire, done string) *hireRefusal {
	return &hireRefusal{Status: http.StatusConflict, Message: calledHire(h) + " was marked gone while it was starting, so " + done + "."}
}

// stoppedGone is a fire's answer for a desk whose row already said gone: its session (what) was ended and seen
// gone, and its row left as it was. A 200, not a refusal: the office and the GM route pass it on as they get it.
func stoppedGone(h Hire, what string) *hireRefusal {
	forgetGoneDesk(h.ID)
	return &hireRefusal{Status: http.StatusOK, Message: goneStoppedNote(h, what)}
}

func goneStoppedNote(h Hire, what string) string {
	return calledHire(h) + " was already marked " + h.State + ", but its " + what + " was still running: it's ended now, and its row stays as it was."
}

// alreadyGone is a fire's answer for a desk whose row says gone with nothing left running: its start's line goes.
func alreadyGone(h Hire) string {
	forgetGoneDesk(h.ID)
	return calledHire(h) + " is already gone."
}

// goneDeskLine is how the start lists a desk whose row says gone while its session (what) runs.
func goneDeskLine(h Hire, what string) string {
	how := "Fire " + calledHire(h) + " again to end it"
	if h.Engine == t3Engine {
		how = "Let " + calledHire(h) + " go again from Setup's team list to end it"
	}
	return calledHire(h) + " is marked " + h.State + ", but its " + what + " is still running. " + how + "; its row stays as it is."
}

// goneDesksListed are the start's lines still standing, by hire: Setup shows them (hiring.goneDesks) until a fire
// of that desk ends its session or finds it gone (RT126GMEDIT1 C2).
var goneDesksListed = struct {
	sync.Mutex
	lines map[string]string
}{lines: map[string]string{}}

func forgetGoneDesk(id string) {
	goneDesksListed.Lock()
	delete(goneDesksListed.lines, id)
	goneDesksListed.Unlock()
}

// goneDesksStanding are the start's lines still standing, in order.
func goneDesksStanding() []string {
	goneDesksListed.Lock()
	defer goneDesksListed.Unlock()
	lines := []string{}
	for _, line := range goneDesksListed.lines {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

// sweepGoneDesks lists, once as the core starts, every desk whose row says gone while its session still runs, a
// line each in the event log and on Setup, for the owner. Only hires that left something to look at are looked at: a
// tmux socket file, or a T3 Code worktree that's still there. A launcher's window connects again after the start with
// no hire's id (its token was used up), so none is found here.
func (d *Deck) sweepGoneDesks() {
	for _, h := range readRegistry(registryPath(), false).hires {
		if isOpen(h) || h.T3Session != "" {
			continue
		}
		place := h.TmuxSocket
		if h.Engine == t3Engine {
			place = h.Worktree.storedOnly() // looked at only
		}
		if _, err := os.Lstat(place); place == "" || err != nil {
			continue
		}
		if look := d.deskSession(h); look.known && look.running {
			line := goneDeskLine(h, look.what)
			d.events.append(eventEntry{At: time.Now().UnixMilli(), Kind: eventKindGoneDesk, Folder: projectFolder(h), Detail: line})
			goneDesksListed.Lock()
			goneDesksListed.lines[h.ID] = line
			goneDesksListed.Unlock()
		}
	}
}
