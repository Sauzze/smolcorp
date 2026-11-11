package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Take over in T3 Code, and Hand back (EXPERIMENTAL, T795 on the Go core): a Claude Code hire Smolcorp runs in a
// tmux session of its own goes on, the same session, in T3 Code, and later comes back to its tmux session.
//
// Take over: the hire's Claude Code is stopped as a fire stops it (only when its window is idle with nothing
// typed, and seen gone), and nothing may hold its session or work in its worktree after. Its fence goes on the
// worktree, as a hire into T3 Code's does (t3desk.go), before T3 Code hears of the folder; the worktree becomes
// a T3 project of its own; then THAT session is imported with T3 Code's internal import (t3internal.go), as the
// thread T3 Code names after it, import:claudeAgent:<session>. From then on it's a T3 desk, the lane's rules and
// all (t3desks.go): no keys, no fire, mail through T3 Code, Let go from its card.
//
// Hand back: T3 Code's hold is released with its internal Force stop, and its project removed (so no thread of
// T3 Code's is left that could open the session again); then the hard check, read from the system and nothing
// run: no process names the session or works in the worktree, and its transcript isn't being written. Only then
// does the fence come off the worktree, and the hire's Claude Code is resumed, `claude --resume <session>`, in its
// own tmux session with its own settings file (its fence). Any check that fails refuses, and says why.
//
// Two writers lose turns silently (S794), so each side starts only once the other is seen gone: Take over stops
// the hire before T3 Code is told, Hand back looks before it resumes. A step that fails after the hire was
// stopped hands it back at once; the registry keeps the session (t3Session) until it's resumed, so Hand back is
// offered until it is.

const (
	// t3ImportedThread is how T3 Code names the thread of a Claude Code session it imported: this, then the session.
	t3ImportedThread = "import:claudeAgent:"
	// t3QuietFor is how long a session's transcript must stay as it is before it's resumed.
	t3QuietFor    = time.Second
	t3TakeOverTag = "EXPERIMENTAL"
)

// t3TakeOverVersions are the t3 versions whose internal calls Take over was read and tried on (S794: v0.1.8's
// source and its install). Any other is refused, by name.
var t3TakeOverVersions = []string{"v0.1.8"}

// t3SessionShape is a Claude Code session id as T3 Code resumes one.
var t3SessionShape = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// t3Session is a hire's Claude Code session: its id and its transcript, as its hooks said.
type t3Session struct{ ID, Transcript string }

// t3NamesSession is whether a command line names session anywhere (claude --resume <id>, a tail of its file…).
func t3NamesSession(args []string, session string) bool {
	return session != "" && slices.ContainsFunc(args, func(arg string) bool { return strings.Contains(arg, session) })
}

// t3TakeOverGate is why nothing can be taken over or handed back now, in one sentence, or "".
func t3TakeOverGate(on bool, status t3Status) string {
	switch {
	case !on:
		return "T3 Code is off in Settings: turn it on to take a hire over in T3 Code."
	case status.State == t3None || status.State == "":
		return "Install T3 Code and its t3 command to take a hire over in T3 Code (t3.codes)."
	case status.State == t3AppOnlyIs:
		return "Taking a hire over in T3 Code needs its t3 command: in T3 Code, open Settings → About and install it."
	case status.Version == "":
		return "Smolcorp couldn't read t3's version, so it won't take a hire over in T3 Code."
	case !slices.Contains(t3TakeOverVersions, status.Version):
		return "Take over uses T3 Code's internal calls, tried on t3 " + strings.Join(t3TakeOverVersions, ", ") + " only, and this is t3 " + status.Version + ", so Smolcorp won't use them."
	}
	return ""
}

// t3TakeOverRefusal is why hire h can't be taken over, whatever's installed, or "". session is its Claude Code
// session as its hooks said; home, your home folder (T3 Code's agent works on ~/.claude's account).
func t3TakeOverRefusal(h Hire, session t3Session, home string, testCopy bool) string {
	name := calledHire(h)
	switch {
	case testCopy:
		return "A test copy doesn't take hires over in T3 Code: that's your real T3 Code."
	case !isOpen(h):
		return name + " is already gone."
	case h.Engine == t3Engine:
		return name + " already runs in T3 Code."
	case h.T3Session != "":
		return name + " is waiting to be handed back from T3 Code: hand it back first."
	case h.Role == gmRole:
		return "The General Manager isn't taken over in T3 Code."
	// T3 Code's agent works on your Claude sign-in, and Hand back resumes on the default account: an API-key hire
	// would come back on your subscription (RMSY S1).
	case h.Account == apiKeyAccountName:
		return name + " works on the API-key account, and T3 Code's agent works on your Claude account, so it can't take this session over."
	case h.ViaLauncher:
		return name + " runs in a window of Smolcorp's launcher: Take over reaches only a hire in a tmux session of its own, for now."
	case h.TmuxSocket == "" || !strings.HasPrefix(filepath.Base(h.TmuxSocket), hireSocketPrefix):
		return "Only a hire Smolcorp runs in a tmux session of its own can be taken over in T3 Code, and " + name + " isn't one."
	case h.Worktree == "":
		return name + " has no worktree of its own, and in T3 Code its fence stays on one, so it can't be taken over."
	// Where the hire's session started: its folder is the project (as the Mac's registry), so the
	// session's folder is the worktree plus the folder's place in the project (startedIn).
	case plainPath(filepath.Clean(startedIn(h))) != plainPath(filepath.Clean(h.Worktree)):
		return name + " works in a folder inside its worktree, and T3 Code takes over only a session started at a project's top."
	case session.ID == "":
		return "Smolcorp hasn't heard from " + name + "'s Claude Code yet, so it doesn't know which session to hand over."
	case !t3SessionShape.MatchString(session.ID):
		return name + "'s session id isn't one T3 Code resumes, so it can't be taken over."
	case !t3OnDefaultAccount(session, home):
		return name + " works on another Claude account, and T3 Code's agent works on your default one, so it can't take this session over."
	}
	return ""
}

// t3OnDefaultAccount is whether session's transcript is the default account's (~/.claude/projects/<folder>/<id>.jsonl):
// the only one T3 Code's import looks in.
func t3OnDefaultAccount(session t3Session, home string) bool {
	transcript := filepath.Clean(session.Transcript)
	return session.Transcript != "" && home != "" && filepath.Base(transcript) == session.ID+".jsonl" &&
		filepath.Dir(filepath.Dir(transcript)) == filepath.Join(filepath.Clean(home), ".claude", "projects")
}

// t3Handing is what Take over and Hand back use besides the hire: tests give their own.
type t3Handing struct {
	Machine t3Machine
	Git     gitRunner
	// Call is one call of T3 Code's internal API (t3Call).
	Call func(program, method string, payload map[string]any) (map[string]any, error)
	// NoCalls: Call isn't used, as T3 Code is off or t3 isn't the version Take over was tried on (Hand back's
	// fallback, RM5b S4).
	NoCalls bool
	// Holding is the processes that name session or work in folder (processesHolding); false when it can't look.
	Holding func(folder, session string) ([]int, bool)
	Name    func(int) string
	Stat    func(string) (os.FileInfo, error)
	Pause   func(time.Duration)
	Now     func() time.Time
	// Fence is the hire's fence (t3FenceSettings).
	Fence func() (map[string]any, error)
	// Stop ends the hire's Claude Code (session) in its own tmux, as a fire would without force: an error says why not.
	Stop func(h Hire, session string) error
	// Resume starts `claude --resume <session>` in the hire's own tmux, with its own settings file: its tmux socket
	// as tmux spells it, for the registry.
	Resume func(h Hire, session string) (string, error)
	// Save changes hire id's row in the registry: an error when it couldn't be written, or the row is gone or no
	// longer open (fired or failed meanwhile).
	Save func(id string, change func(*Hire)) error
}

// t3SaveMost is how many times the registry is tried right after the stop (RM5b S2).
const t3SaveMost = 3

// t3SaveTries runs save up to t3SaveMost times, a moment apart: whether it was saved.
func t3SaveTries(ways t3Handing, save func() error) bool {
	for try := 1; ; try++ {
		if save() == nil {
			return true
		}
		if try == t3SaveMost {
			return false
		}
		ways.Pause(200 * time.Millisecond)
	}
}

// t3UnsavedAfterStop is what to do when hire h was stopped but the registry couldn't be written, so Hand back
// isn't offered: resume it on its own settings file, its fence, never a bare claude --resume (RM5b S2).
func t3UnsavedAfterStop(h Hire, session string) string {
	settings := filepath.Join(hireFolder(supportDir, h.ID), hireSettingsFile)
	return calledHire(h) + " was stopped, but the registry couldn't be written, so nothing went to T3 Code. Resume it fenced, from its folder: claude --settings=" +
		shellQuoted(settings) + " --resume " + session + " (or fire it and hire again)."
}

func t3CantLook(name string) string {
	return "Smolcorp can't see which processes hold " + name + "'s session on this " + t3Here() + ", so nothing was changed: two agents on one session lose turns silently."
}

// t3StillHeld names what still holds a session: up to five processes, by name where it's known.
func t3StillHeld(pids []int, name func(int) string) string {
	var named []string
	for _, pid := range pids[:min(len(pids), 5)] {
		if called := name(pid); called != "" {
			named = append(named, called+", PID "+strconv.Itoa(pid))
		} else {
			named = append(named, "PID "+strconv.Itoa(pid))
		}
	}
	return strings.Join(named, "; ")
}

// t3WaitFree waits up to t3LeaveWithin for nothing to hold session or work in folder: what still does, and
// whether it could look at all.
func t3WaitFree(folder, session string, ways t3Handing) ([]int, bool) {
	end := ways.Now().Add(t3LeaveWithin)
	still, looked := ways.Holding(folder, session)
	for looked && len(still) > 0 && ways.Now().Before(end) {
		ways.Pause(500 * time.Millisecond)
		still, looked = ways.Holding(folder, session)
	}
	return still, looked
}

// t3Quiet is whether transcript is there and isn't being written: the same size and time, t3QuietFor apart.
func t3Quiet(transcript string, ways t3Handing) bool {
	before, err := ways.Stat(transcript)
	if err != nil {
		return false
	}
	ways.Pause(t3QuietFor)
	after, err := ways.Stat(transcript)
	return err == nil && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

// t3TakeOver takes hire h over in T3 Code, its session being session; it runs git, t3 and T3 Code's internal
// import. The thread T3 Code runs it in, or a refusal. h has passed t3TakeOverRefusal and the gate.
func t3TakeOver(h Hire, session t3Session, ways t3Handing) (string, error) {
	name := calledHire(h)
	git := ways.Git
	if git == nil {
		git = runGit
	}
	program := t3Program(ways.Machine)
	if program == "" {
		return "", t3Refused(http.StatusConflict, "T3 Code's t3 command isn't there, so nothing was changed.")
	}
	fence := filepath.Join(h.Worktree, filepath.FromSlash(t3FenceName))
	if _, err := os.Lstat(fence); err == nil {
		return "", t3Refused(http.StatusConflict, "This repository keeps its own "+t3FenceName+", so Smolcorp can't put "+name+"'s fence there, and nothing was changed.")
	}
	if t3HooksSwitchedOff(h.Worktree, git) {
		return "", t3Refused(http.StatusConflict, t3HooksOff)
	}
	if _, looked := ways.Holding(h.Worktree, session.ID); !looked {
		return "", t3Refused(http.StatusConflict, t3CantLook(name))
	}
	settings, err := ways.Fence()
	if err != nil {
		return "", t3Refused(http.StatusInternalServerError, name+"'s fence couldn't be made, so nothing was changed.")
	}
	// 1. Its Claude Code stopped in its tmux, as a fire would without force.
	if err := ways.Stop(h, session.ID); err != nil {
		return "", err
	}
	// From here it's on record as taken over: whatever stops part way is handed back, never lost.
	h.T3Session, h.T3Transcript = session.ID, session.Transcript
	if !t3SaveTries(ways, func() error {
		return ways.Save(h.ID, func(r *Hire) { r.T3Session, r.T3Transcript = session.ID, session.Transcript })
	}) {
		return "", t3Refused(http.StatusInternalServerError, t3UnsavedAfterStop(h, session.ID))
	}
	giveBack := func(status int, why string) (string, error) {
		h.Engine = ""
		if _, err := t3HandBack(h, ways); err != nil {
			why += " Then handing it back didn't finish either: " + err.Error()
		} else {
			why += " Nothing stayed in T3 Code, and " + name + " was resumed in its tmux session."
		}
		return "", t3Refused(status, why)
	}
	// 2. Nothing else holds its session, nor works in its worktree.
	if still, looked := t3WaitFree(h.Worktree, session.ID, ways); !looked || len(still) > 0 {
		why := t3CantLook(name)
		if looked {
			why = name + " was stopped, but a process still holds its session or works in its worktree (" + t3StillHeld(still, ways.Name) + "), so it wasn't handed to T3 Code."
		}
		return "", t3Refused(http.StatusConflict, why+" Hand it back once that's closed.")
	}
	// 3. The fence on its worktree, kept out of git, before T3 Code hears of the folder.
	if !t3WriteFence(settings, fence) || !t3Exclude(h.Worktree, git) {
		return giveBack(http.StatusInternalServerError, name+"'s fence couldn't be put on its worktree, so it wasn't handed to T3 Code.")
	}
	// 4. The worktree as a T3 project of its own.
	title := calledHire(h) + " · " + h.Branch
	added := ways.Machine.Run(program, []string{"project", "add", h.Worktree, "--title", title}, t3Within)
	project := ""
	if added.Ended && added.Code == 0 {
		project = t3AddedProject(added.Output, h.Worktree)
	}
	thread := t3ImportedThread + session.ID
	if project == "" {
		// Added but not as asked, or no answer (T3 Code may have added it): taken back as Smolcorp's. Only a project
		// T3 Code refused to add (one already there, someone else's) is left alone. Its thread goes on record first,
		// as below: if the Hand back stops too, the next one still removes the project (RM5c2 S1).
		if !added.Ended || added.Code == 0 {
			h.T3Thread = thread
			if !t3SaveTries(ways, func() error { return ways.Save(h.ID, func(r *Hire) { r.T3Thread = thread }) }) {
				return giveBack(http.StatusBadGateway, "T3 Code didn't add "+name+"'s worktree as its own project ("+t3Said(added)+"), so it wasn't handed over. The registry couldn't be written either: if T3 Code has a project for it, remove \""+title+"\" in T3 Code.")
			}
		}
		return giveBack(http.StatusBadGateway, "T3 Code didn't add "+name+"'s worktree as its own project ("+t3Said(added)+"), so it wasn't handed over.")
	}
	// The project is Smolcorp's: Hand back removes it (it never removes a project it didn't add). Its thread goes
	// on record before T3 Code is asked anything more, and in the last save too: a thread only in memory is a
	// project Hand back wouldn't know to remove (RM5c B1). Not written: handed back at once, from h.
	h.T3Thread = thread
	if !t3SaveTries(ways, func() error { return ways.Save(h.ID, func(r *Hire) { r.T3Thread = thread }) }) {
		return giveBack(http.StatusInternalServerError, "The registry couldn't be written, so "+name+" wasn't handed over.")
	}
	if !t3FenceHolds(fence, settings) {
		return giveBack(http.StatusConflict, name+"'s fence was gone or changed before T3 Code took the session, so it wasn't handed over.")
	}
	// 5. THAT session imported, by T3 Code's internal import, into the project just added.
	if _, err := ways.Call(program, "agentSessions.import", map[string]any{"projectId": project, "expectedWorkspaceRoot": h.Worktree}); err != nil {
		return giveBack(http.StatusBadGateway, "T3 Code didn't import "+name+"'s session: "+err.Error()+".")
	}
	shown := t3Show(thread, program, ways.Machine)
	if shown == nil {
		return giveBack(http.StatusBadGateway, "T3 Code didn't show "+name+"'s session as its thread after the import, so it wasn't handed over.")
	}
	if in, _ := shown["projectId"].(string); in != project {
		return giveBack(http.StatusBadGateway, "T3 Code put "+name+"'s session in another project, not its worktree's, so it wasn't handed over.")
	}
	if _, elsewhere := shown["worktreePath"].(string); elsewhere {
		return giveBack(http.StatusBadGateway, "T3 Code gave "+name+"'s session a worktree of its own, not the hire's, so it wasn't handed over.")
	}
	if !t3FenceHolds(fence, settings) {
		return giveBack(http.StatusConflict, name+"'s fence was gone or changed as T3 Code took the session, so it was handed back.")
	}
	link, _ := shown["link"].(string)
	// 6. A T3 desk now: the lane's follow, card and Let go.
	if ways.Save(h.ID, func(r *Hire) { r.Engine, r.T3Thread, r.T3Link, r.State = t3Engine, thread, link, "idle" }) != nil {
		return giveBack(http.StatusInternalServerError, "The registry couldn't be written, so "+name+" wasn't handed over.")
	}
	return thread, nil
}

// t3HandBack gives hire h back to its own tmux session: T3 Code's hold released and its project removed (only one
// Take over added, which recorded its thread), the hard check, the fence off, then its session resumed. A note
// when its fence couldn't be taken off, or a refusal. h has T3Session set.
func t3HandBack(h Hire, ways t3Handing) (string, error) {
	name := calledHire(h)
	session := h.T3Session
	program := t3Program(ways.Machine)
	if program == "" {
		return "", t3Refused(http.StatusConflict, "Handing "+name+" back needs T3 Code's t3 command, to remove its project there, and it isn't there, so nothing was changed: in T3 Code, open Settings → About and install it, then hand back again.")
	}
	if _, looked := ways.Holding(h.Worktree, session); !looked {
		return "", t3Refused(http.StatusConflict, t3CantLook(name))
	}
	if h.T3Thread != "" {
		// 1. T3 Code lets go of the session: its own Force stop, while its internal calls may be used (T3 Code on,
		// the version Take over was tried on). When they may not, or its server doesn't answer (its app closed),
		// the project's removal below ends T3 Code's agent for it all the same (T797a), and the hard check after it
		// is the guard (RM5b S4). Only an answer that says part of it is still held stops here.
		if !ways.NoCalls {
			value, err := ways.Call(program, "orchestration.forceStopThread", map[string]any{"threadId": h.T3Thread})
			if outcome, _ := value["outcome"].(string); err == nil && !slices.Contains([]string{"killed", "released", "releasing"}, outcome) {
				return "", t3Refused(http.StatusConflict, "T3 Code stopped only part of what holds "+name+"'s session ("+cmpOr(outcome, "it didn't say")+"), so it wasn't handed back.")
			}
		}
		// 2. Its project removed, so no thread of T3 Code's is left that could open the session again.
		removed := ways.Machine.Run(program, []string{"project", "remove", h.Worktree, "--force"}, t3Within)
		if !(removed.Ended && removed.Code == 0) && !t3NoProject(removed) {
			return "", t3Refused(http.StatusBadGateway, "T3 Code didn't remove "+name+"'s project ("+t3Said(removed)+"), so it wasn't handed back: T3 Code could still open its session.")
		}
		// Unwritten, the registry keeps a thread whose project is gone: harmless, as the next Hand back's removal
		// finds no project (t3NoProject), and step 4's save clears it before any resume.
		h.T3Thread = ""
		_ = ways.Save(h.ID, func(r *Hire) { r.T3Thread, r.T3Link = "", "" })
	}
	// 3. The hard check: nothing holds the session or works in the worktree, and nothing writes its transcript.
	still, looked := t3WaitFree(h.Worktree, session, ways)
	switch {
	case !looked:
		return "", t3Refused(http.StatusConflict, t3CantLook(name))
	case len(still) > 0:
		return "", t3Refused(http.StatusConflict, "A process still holds "+name+"'s session or works in its worktree ("+t3StillHeld(still, ways.Name)+"), so it wasn't resumed: two agents on one session lose turns silently. If that's yours, close it and hand back again.")
	case !t3Quiet(h.T3Transcript, ways):
		return "", t3Refused(http.StatusConflict, name+"'s session is still being written, or its file is gone, so it wasn't resumed.")
	}
	// 4. The fence off the worktree: in its tmux session, its own settings file is its fence.
	note := ""
	fence := filepath.Join(h.Worktree, filepath.FromSlash(t3FenceName))
	if !t3RemoveFence(fence, filepath.Dir(fence)) {
		note = t3FenceKept(h)
	}
	if ways.Save(h.ID, func(r *Hire) { r.Engine, r.T3Thread, r.T3Link, r.T3Floor, r.T3Desk = "", "", "", nil, nil }) != nil {
		return "", t3Refused(http.StatusInternalServerError, "The registry couldn't be written, so "+name+" wasn't resumed.")
	}
	// 5. Its session resumed in its own tmux session, with its own settings file.
	socket, err := ways.Resume(h, session)
	if err != nil {
		return "", t3Refused(http.StatusInternalServerError, name+"'s session is free, but it couldn't be resumed: "+err.Error()+" Hand it back again to retry.")
	}
	// Its socket as tmux spells it, as the engine keeps it, in the same checked save.
	if !t3SaveTries(ways, func() error {
		return ways.Save(h.ID, func(r *Hire) {
			r.T3Session, r.T3Transcript, r.State = "", "", "working"
			if socket != "" {
				r.TmuxSocket = socket
			}
		})
	}) {
		// It runs in its tmux; only the registry lags. Hand back again sees its own session there and clears it
		// (handBackFromT3), as nothing else would: Fire waits for Hand back (RM5c N2).
		note = strings.TrimSpace(note + " " + t3ResumedUnsaved(name))
	}
	return note, nil
}

// t3BeingTakenOver is a fire of hire h refused while it's being taken over in T3 Code.
func t3BeingTakenOver(h Hire) string {
	return calledHire(h) + " is being taken over in T3 Code right now, so it wasn't fired: try again once that's done."
}

// t3ResumedUnsaved is what Hand back says when the hire was resumed but the registry still has it as handed over.
func t3ResumedUnsaved(name string) string {
	return name + " was resumed in its tmux session, but the registry couldn't be written, so Hand back is still offered: hand it back again to clear that."
}

// t3WindowRefusal is why a hire's window isn't left as it is for Take over, or "": only an idle one is.
func t3WindowRefusal(name string, state PaneState, readable bool) string {
	switch {
	case !readable:
		return "Couldn't tell what " + name + "'s window holds, so nothing was changed. Look at it first."
	case state == PaneIdle:
		return ""
	case state == PaneMenu:
		return name + " is waiting for an answer in its window: answer it there first. Nothing was changed."
	case state == PaneDraft:
		return name + " has something typed in its window that isn't sent: send or clear it first. Nothing was changed."
	case state == PaneBusy:
		return name + " is working right now: take it over once it's done. Nothing was changed."
	}
	return "Couldn't tell what " + name + "'s window holds, so nothing was changed. Look at it first."
}

// MARK: The deck

// t3SessionOf is hire h's Claude Code session: the one session at a tab of its own tmux socket, and its transcript.
// None, or more than one, is none: then nothing says which session is the hire's.
func (d *Deck) t3SessionOf(h Hire) t3Session {
	same := func(a, b string) bool {
		return a != "" && b != "" && filepath.Base(a) == filepath.Base(b) &&
			physical(filepath.Dir(filepath.Clean(a))) == physical(filepath.Dir(filepath.Clean(b)))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var found []*Session
	for _, s := range d.sessions.All() {
		if kind, _, socket := splitTab(s.Tab); kind == "tmux" && same(socket, h.TmuxSocket) {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		return t3Session{}
	}
	return t3Session{ID: found[0].ID, Transcript: found[0].Transcript}
}

// t3SaveOpen changes hire id's row in the registry, only while it's open: a row gone, or fired or failed meanwhile
// (a registry edit), is an error and is left as it is, so Take over never goes on with a hire that's gone (RM5b N4).
func t3SaveOpen(id string, change func(*Hire)) error {
	gone := false
	if err := updateRegistry(func(f *registryFile) {
		i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id && isOpen(h) })
		if gone = i < 0; !gone {
			change(&f.hires[i])
		}
	}); err != nil {
		return err
	}
	if gone {
		return errors.New("that hire is gone from the registry")
	}
	return nil
}

// t3Stand's ways, for Take over and Hand back: tests replace them.
var t3HandingStand func(d *Deck, h Hire) t3Handing

// t3HandingFor is the ways for hire h: this computer's, or a test's.
func (d *Deck) t3HandingFor(h Hire) t3Handing {
	if t3HandingStand != nil {
		return t3HandingStand(d, h)
	}
	d.mu.Lock()
	saved := slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	machine := newT3Machine()
	return t3Handing{
		Machine: machine, Git: t3Stand.Git,
		Call: func(program, method string, payload map[string]any) (map[string]any, error) {
			return t3Call(machine, program, method, payload)
		},
		Holding: processesHolding, Name: processName, Stat: os.Stat, Pause: time.Sleep, Now: time.Now,
		Fence: func() (map[string]any, error) { return t3FenceSettings(h.ID, claudeDenyProfiles(saved)) },
		// Once it's seen gone, its desk's session ends: T3 Code's agent reports as the same session, and it's never
		// a terminal's desk (isT3HiresOwn). A resumed one comes back later, as a new desk.
		Stop: func(h Hire, session string) error {
			if err := d.t3StopInTmux(h); err != nil {
				return err
			}
			d.mu.Lock()
			d.sessions.end(session, nowMillis())
			d.changed()
			d.mu.Unlock()
			return nil
		},
		Resume: func(h Hire, session string) (string, error) {
			return d.t3ResumeInTmux(h, session, claudeDenyProfiles(saved))
		},
		Save: t3SaveOpen,
	}
}

// takeOverIntoT3 is Take over in T3 Code for hire id, from Setup's team row or its desk's card: the gate, the
// refusals, then t3TakeOver, after any hire or let go under way. 200 with its thread, or why not.
func (d *Deck) takeOverIntoT3(id string) (int, any) {
	asked, found := registryHire(id)
	if id == "" || !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	d.mu.Lock()
	on := t3CodeAllowed(d.config)
	d.mu.Unlock()
	ways := d.t3HandingFor(asked)
	if why := t3TakeOverGate(on, t3StatusNow(true, ways.Machine)); why != "" {
		return http.StatusConflict, why
	}
	session := d.t3SessionOf(asked)
	// The desk and the slot are let go by defer, so a panic inside t3TakeOver leaves neither held (FXTO2 N2).
	var thread string
	var err error
	func() {
		t3Desk.Lock()
		defer t3Desk.Unlock()
		// The hire held from fires until Take over is done: a fire under way refuses it, and a fire from now on is
		// refused, then reads the registry again and finds it taken over (RM5c2 N1).
		firing.Lock()
		beingFired := firing.ids[id]
		if !beingFired {
			firing.takingOver[id] = true
		}
		firing.Unlock()
		if !beingFired {
			defer func() {
				firing.Lock()
				delete(firing.takingOver, id)
				firing.Unlock()
			}()
		}
		// Read again once it's this one's turn, so a second click finds it already taken over.
		var hire Hire
		hire, found = registryHire(id)
		switch {
		case !found:
		case beingFired:
			err = t3Refused(http.StatusConflict, calledHire(hire)+" is being fired right now, so it wasn't taken over.")
		default:
			if why := t3TakeOverRefusal(hire, session, ways.Machine.Home, testCopyPort() != 0); why != "" {
				err = t3Refused(http.StatusConflict, why)
			} else {
				thread, err = t3TakeOver(hire, session, ways)
			}
		}
	}()
	d.mu.Lock()
	d.changed()
	d.mu.Unlock()
	if !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	if err != nil {
		return t3Answer(err)
	}
	return http.StatusOK, map[string]any{"ok": true, "thread": thread, "tag": t3TakeOverTag}
}

// handBackFromT3 is Hand back for hire id, taken over before: t3HandBack after any hire or let go under way.
func (d *Deck) handBackFromT3(id string) (int, any) {
	asked, found := registryHire(id)
	if id == "" || !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	if t3IsHiring(id) {
		return http.StatusConflict, t3StillHiring(calledHire(asked))
	}
	d.mu.Lock()
	on := t3CodeAllowed(d.config)
	d.mu.Unlock()
	ways := d.t3HandingFor(asked)
	// Not gated as Take over is: a hire taken over is handed back even once T3 Code is off, t3 was updated or its
	// app is closed, without T3 Code's internal calls then (RM5b S4). Without t3 at all, t3HandBack says why.
	// T3 Code off is known without asking t3 (RM5c N3).
	ways.NoCalls = !on || t3TakeOverGate(on, t3StatusNow(true, ways.Machine)) != ""
	t3Desk.Lock()
	hire, found := registryHire(id)
	var note string
	var err error
	switch {
	case !found:
	case hire.T3Session == "":
		err = t3Refused(http.StatusConflict, calledHire(hire)+" wasn't taken over from Smolcorp, so there's nothing to hand back.")
	case !isOpen(hire):
		err = t3Refused(http.StatusConflict, calledHire(hire)+" is already gone.")
	case t3AlreadyResumed(hire, d.t3SessionOf(hire)):
		// Resumed by an earlier Hand back whose last save failed (RM5c N2): its own Claude Code runs that session in
		// its own tmux, so only the registry is behind. Nothing is stopped, removed or resumed.
		if ways.Save(hire.ID, func(r *Hire) { r.T3Session, r.T3Transcript, r.State = "", "", "working" }) != nil {
			err = t3Refused(http.StatusInternalServerError, "The registry couldn't be written, so "+calledHire(hire)+" still shows as handed over.")
		}
	default:
		note, err = t3HandBack(hire, ways)
	}
	t3Desk.Unlock()
	d.mu.Lock()
	d.changed()
	d.mu.Unlock()
	if !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	if err != nil {
		return t3Answer(err)
	}
	return http.StatusOK, map[string]any{"ok": true, "note": note}
}

// t3AlreadyResumed is whether hire h, on record as waiting for Hand back, was in fact resumed: nothing of T3 Code's
// is on record (no engine, no thread), and the one session at its own tmux socket, as its hooks say, is the one
// handed over. Then it's its own Claude Code that holds it, and Hand back has only the registry left to do.
func t3AlreadyResumed(h Hire, live t3Session) bool {
	return h.T3Session != "" && h.Engine == "" && h.T3Thread == "" && live.ID == h.T3Session
}

// t3Answer is a refusal as a route says it.
func t3Answer(err error) (int, any) {
	var refused *hireRefusal
	if errors.As(err, &refused) {
		return refused.Status, refused.Message
	}
	return http.StatusInternalServerError, err.Error()
}

// t3TakeOverJSON is what a hire's row and card say of Take over: offered (only for a hire in a tmux session of its
// own, with T3 Code on), or Hand back offered, with the tag. nil when neither is.
func t3TakeOverJSON(h Hire, on bool) map[string]any {
	switch {
	case !on || !isOpen(h) || h.Role == gmRole:
		return nil
	case h.T3Session != "":
		return map[string]any{"tag": t3TakeOverTag, "handBack": true}
	case h.Engine == "" && !h.ViaLauncher && h.TmuxSocket != "" && h.Worktree != "":
		return map[string]any{"tag": t3TakeOverTag, "takeOver": true}
	}
	return nil
}

// takeOverAtDesk is /act?action=takeover or handback from a desk's card in the office's own window: the hire at a
// terminal's desk (takeover), or at a T3 desk (handback). False for a desk with no hire.
func (d *Deck) takeOverAtDesk(action, session string) (int, any, bool) {
	var h Hire
	var ok bool
	if action == "handback" {
		d.mu.Lock()
		h, ok = d.t3HireAtDesk(session)
		d.mu.Unlock()
	} else {
		h, ok = d.hireInSession(session)
	}
	if !ok {
		return 0, nil, false
	}
	if action == "handback" {
		status, answer := d.handBackFromT3(h.ID)
		return status, answer, true
	}
	status, answer := d.takeOverIntoT3(h.ID)
	return status, answer, true
}
