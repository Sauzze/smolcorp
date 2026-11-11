package desktop

import (
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// What the hire engine shares between the tmux way (hireengine.go, hirefire.go;
// Linux and the Mac-like systems) and the launcher way (hirelauncher.go; the
// Windows beta): the request, the registry's small changes, the words a fire
// refuses in. They build on every OS, so the launcher way is tested everywhere.

// hireStartWithin is how long a hire has to be heard from, as the Mac's.
const hireStartWithin = 45 * time.Second

// plainHireID is whether an id is only a-z, 0-9 and -, up to 64: a name a
// socket, a recap or a folder can take.
func plainHireID(hireID string) bool {
	if hireID == "" || len(hireID) > 64 {
		return false
	}
	for _, r := range hireID {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// engineRequest is one hire.
type engineRequest struct {
	Role   Role
	How    roleStart
	Folder string
	// The account: a config folder, the default account, or neither (none named).
	ConfigDir      string
	DefaultAccount bool
	APIKey         bool
	// The port of a test copy, 0 for the office's own.
	TestPort int
	// GMToken makes the General Manager's token, new at its hire (the Deck's
	// newGMToken); asked only for the gm role, after the hire is written down.
	GMToken func() string
	// For its brief: the desk it was hired at on the ground floor (nil for none
	// or upstairs), the answers to its role's questions (nil when it asks them
	// itself) and the task whoever hired wrote for it ("" for none).
	Desk    *int
	Answers []string
	Task    string
	// Setup's own hire (POST /setup/hire, setuphire.go) may say more: the
	// nickname, kept in the registry and naming its worktree and branch; no
	// worktree of its own (the manager off and none asked for, or the GM, as
	// the Mac's); the account it works on, by name, for the registry; and the
	// Claude accounts whose settings and transcripts its fence denies.
	Nickname    string
	NoWorktree  bool
	AccountName string
	Profiles    []claudeProfile
}

// engineAutoMode is whether an engine hire's settings put it in auto mode: on
// a Mac, as the Mac's HireSettings.forHire always does, so a Mac hire's fence
// is the same file whichever engine starts it. Linux and Windows keep theirs.
const engineAutoMode = runtime.GOOS == "darwin"

// hireBrief writes a hire's brief where the Mac's HireDesk writes it: a
// managed hire's (one with a worktree) in its own folder in Smolcorp's, any
// other's in Smolcorp's briefs folder. Nil for a role on its own
// instructions, or an agent that takes no first message (Grok).
func hireBrief(r engineRequest, place hirePlace, hireID string, now time.Time) (*briefMade, error) {
	if Agent(r.Role.Agent) == Grok {
		return nil, nil
	}
	managed := ""
	if place.Worktree != "" {
		managed = hireFolder(supportDir, hireID)
	}
	return makeBrief(briefRequest{role: r.Role, library: editableBriefs(bundledBriefs(), briefTemplatesDir()), folder: place.Folder,
		pinned: r.How == hiredAtPinnedDesk, desk: r.Desk, answers: r.Answers, task: r.Task, managed: managed, date: now,
		kept: briefsKeptDir(), write: true})
}

// engineBrief is an engine hire's first message and its brief's path: to read
// the brief it was just written, or, for a role on its own instructions, the
// role's own first message ("" for the path).
func engineBrief(r engineRequest, place hirePlace, hireID string, now time.Time) (message, brief string, err error) {
	message = r.Role.firstMessage(place.Folder, r.How)
	if r.Answers != nil {
		message = r.Role.firstMessageAnswered(place.Folder, r.Answers)
	}
	made, err := hireBrief(r, place, hireID, now)
	if err != nil {
		return "", "", err
	}
	if made != nil {
		message, brief = made.firstMessage, made.names.brief
	}
	return message, brief, nil
}

// briefDesk is the desk a hire's brief is named for: one held on the ground floor, or none.
func briefDesk(held, floor int) *int {
	if floor > 0 || held < 0 {
		return nil
	}
	return &held
}

// takenPorts are the port ranges of hires still working.
func takenPorts() []string {
	var taken []string
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Ports != "" && h.State != "fired" && h.State != "failed" {
			taken = append(taken, h.Ports)
		}
	}
	return taken
}

// updateRegistry changes the registry under its lock.
func updateRegistry(change func(*registryFile)) error {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(registryPath(), true)
	change(&file)
	return file.save(registryPath())
}

// hireIsOpen is whether hire id is in the registry, neither fired nor failed.
func hireIsOpen(id string) bool {
	return slices.ContainsFunc(officeHires(), func(h Hire) bool { return h.ID == id && isOpen(h) })
}

// forgetHire takes hire id's row and its send key's hash out of the registry:
// for a hire that was refused before it started, so nothing is left of it.
func forgetHire(id string) {
	_ = updateRegistry(func(f *registryFile) {
		f.hires = slices.DeleteFunc(f.hires, func(h Hire) bool { return h.ID == id })
		delete(f.keys, id)
	})
}

// setHire gives hire id a state and its notes, only while it is still
// "starting": a hire the owner fired or changed meanwhile is left as it is.
func setHire(id, state, notes string) {
	_ = updateRegistry(func(f *registryFile) {
		if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id && h.State == "starting" }); i >= 0 {
			f.hires[i].State, f.hires[i].Notes = state, notes
		}
	})
}

// hireHeardFrom is whether a Claude session has started in the hire's folder
// since it was hired: a hook has posted.
func (d *Deck) hireHeardFrom(h Hire) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	folder := physical(h.Folder)
	return slices.ContainsFunc(d.sessions.All(), func(s *Session) bool {
		where := physical(s.Folder)
		return s.Agent == Claude && s.StartedAt >= h.Started && (where == folder || strings.HasPrefix(where, folder+string(filepath.Separator)))
	})
}

// calledHire is what a fire's answers call a hire: its nickname, else its
// name, never its role (T722).
func calledHire(h Hire) string {
	if h.Nickname != "" {
		return h.Nickname
	}
	return h.Name
}

// firing is the hires being fired right now, so two fires of one can't both go ahead.
var firing = struct {
	sync.Mutex
	ids map[string]bool
}{ids: map[string]bool{}}

// windowRefusal is why a window showing screen (readable false: it couldn't be
// looked at) may not be ended, or "". A question or a menu, or a window that
// can't be looked at, is never skipped; force skips the rest.
func windowRefusal(h Hire, screen string, readable, force bool) string {
	return stateRefusal(h, ReadPaneState(screen), readable, force)
}

// stateRefusal is windowRefusal for a window already read as state (a
// launcher sends only that).
func stateRefusal(h Hire, state PaneState, readable, force bool) string {
	name := calledHire(h)
	if !readable {
		return "Couldn't tell what " + name + "'s window holds, so nothing was ended. Look at it first."
	}
	switch {
	case state == PaneMenu:
		return name + " is waiting for an answer in its window. Answer it there first; nothing was ended."
	case force || state == PaneIdle:
		return ""
	case state == PaneDraft:
		return name + " has something typed in its window that isn't sent yet. Send or clear it, or fire with force."
	case state == PaneBusy:
		return name + " is working right now. Wait until it is done, or fire with force."
	}
	return "Couldn't tell what " + name + "'s window holds, so nothing was ended. Look at it, or fire with force."
}

// uncommittedRefusal is why a hire's worktree may not be left, or "": git
// lists changes nobody committed (as the Mac's fireRefusal), or can't say.
func uncommittedRefusal(h Hire, git gitRunner) string {
	status, out, ok := git(physical(h.Worktree), "status", "--porcelain")
	if !ok || status != 0 {
		return "Couldn't tell whether " + calledHire(h) + "'s worktree has unsaved work. Fire again with force to go ahead."
	}
	if strings.TrimSpace(out) == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	shown := strings.Join(lines[:min(len(lines), 10)], ", ")
	if len(lines) > 10 {
		shown += ", and " + strconv.Itoa(len(lines)-10) + " more"
	}
	return calledHire(h) + " has changes that aren't committed: " + shown + ". Commit them, or fire with force."
}

// markFired makes hire id fired, whatever it was doing (starting or working);
// one already fired or failed is left as it is. Unlike setHire, which only
// moves a hire still starting, a fire ends a working hire too. The notes are
// the owner's: they stay as the registry holds them now (read under its
// lock, not the copy read when the fire began), and a kept line, if given, goes
// before them, as the Mac's.
func markFired(id, keptLine string) {
	_ = updateRegistry(func(f *registryFile) {
		if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id && h.State != "fired" && h.State != "failed" }); i >= 0 {
			f.hires[i].State = "fired"
			if keptLine != "" {
				note := keptLine
				if f.hires[i].Notes != "" {
					note += "\n" + f.hires[i].Notes
				}
				f.hires[i].Notes = firstRunes(note, longestNotes)
			}
		}
	})
}

// clearFiredGMToken takes the token away after a GM was fired, unless another
// GM is open by now: one hired the moment this one was marked fired has its row
// written before its token is made (runManagedHire), and both that token and
// this look happen under d.mu, so its token is never the one taken away.
func (d *Deck) clearFiredGMToken() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !slices.ContainsFunc(officeHires(), gmHired) {
		d.gmToken = ""
	}
}
