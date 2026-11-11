package desktop

import (
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
)

// The T3 Code desk on the deck, as the Mac's Deck.hireIntoT3, startT3Follow and letGoOfT3: a hire into T3
// Code once its route has checked who and where; the follow of every T3 hire's thread; Let go. Hires and
// let-gos take t3Desk in turn; the follow has a deck's follower of its own and never waits for them.

// t3Ask is a hire into T3 Code as its route asked for it, its role found and its folder checked (POST
// /setup/hire's `engine: "t3"`, the Mac's: hireFromSetupSheetIntoT3 asks it).
type t3Ask struct {
	Role     Role
	Folder   string
	Nickname string
	// Model is the one asked for, "" for the role's (else t3DefaultModel).
	Model string
	Task  string
	// Answers to the role's questions, nil when it asks them itself.
	Answers []string
	// Pinned: hired at a desk pinned to its project. Desk: the desk it was hired at on the ground floor, or nil.
	Pinned bool
	Desk   *int
	// Floor is the floor it was hired onto (0 for the ground floor): its desk in the office is there.
	Floor int
	// An account or the API-key account asked for: refused, T3 Code's agent works on its own.
	Account string
	APIKey  bool
}

// t3Stand are the lane's ways out, which tests replace: nil and "" are this computer's.
var t3Stand struct {
	Git      gitRunner
	PortFree func(int) bool
	// Guard runs the guard for its groups; IsExec says the smolcorp command is there; CLI is it.
	Guard   guardRunner
	IsExec  func(string) bool
	CLI     string
	Leaving func() t3Leaving
}

func t3Leaver() t3Leaving {
	if t3Stand.Leaving != nil {
		return t3Stand.Leaving()
	}
	return defaultT3Leaving()
}

// t3FenceSettings are hire hireID's fence: its settings as a hire's own file has them. T3 Code's agent works on
// the default Claude account (~/.claude), but profiles, every saved account's (claudeDenyProfiles), are denied to
// it as to any hire, as the Mac's HireSettings.forHire does. With none saved, that's the default account alone.
func t3FenceSettings(hireID string, profiles []claudeProfile) (map[string]any, error) {
	return hireSettingsObject(hireSettingsOptions{HireID: hireID, Auto: t3FenceAuto(runtime.GOOS), Profile: guardHire,
		CLI: cmpOr(t3Stand.CLI, commandLineProgram()), Support: supportDir, Run: t3Stand.Guard, IsExec: t3Stand.IsExec,
		Profiles: profiles})
}

// t3FenceAuto is whether a hire's fence carries auto mode: as the Mac's, on a Mac. T3 Code runs its agent with
// its permissions bypassed anyway, so it only matters for the file being the Mac's, byte for byte.
func t3FenceAuto(system string) bool { return system == "darwin" }

// t3MarkFailed marks hire id failed in the registry at path, its reason before its notes.
func t3MarkFailed(path, id, reason string) {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	if i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == id }); i >= 0 {
		file.hires[i].State = "failed"
		file.hires[i].Notes = firstRunes(strings.Join(slices.DeleteFunc([]string{reason, file.hires[i].Notes}, func(s string) bool { return s == "" }), "\n"), longestNotes)
		_ = file.save(path)
	}
}

// hireIntoT3 hires into T3 Code (EXPERIMENTAL), in the Mac's Deck.hire order: hiring off, the per-hire refusals
// (no role is one), Claude Code's hooks, the cooldown, the gate asked fresh, then t3HireInto, after any hire or
// let go under way. 204 once its thread runs, else a status and why.
func (d *Deck) hireIntoT3(ask t3Ask) (int, string) {
	d.mu.Lock()
	off := !d.config.HiringEnabled
	d.mu.Unlock()
	if off {
		return http.StatusForbidden, hiringOff
	}
	role := ask.Role
	if ask.Model != "" {
		role.Model = ask.Model
	}
	picked := &role
	if role.ID == "" {
		picked = nil
	}
	if why := t3HireRefusal(picked, ask.Account, ask.APIKey, testCopyPort() != 0); why != "" {
		return http.StatusConflict, why
	}
	// Claude Code's hooks must report here, as for any hire (the Mac's hooksRefusal). Without the lock: it reads the
	// agent's settings files.
	if why := hooksNotConnected(Claude); why != "" {
		return http.StatusConflict, why
	}
	d.mu.Lock()
	on := t3CodeAllowed(d.config)
	if d.hiring || time.Since(d.lastHire) < hireCooldown {
		d.mu.Unlock()
		return http.StatusTooManyRequests, "Hired a moment ago. Try again in a few seconds."
	}
	d.lastHire = time.Now()
	// The saved accounts are copied under the lock, and looked in once it's let go (claudeaccounts.go).
	savedAccounts := slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	status, message := d.hireIntoT3Now(ask, role, on, claudeDenyProfiles(savedAccounts))
	d.mu.Lock()
	if status != http.StatusNoContent {
		d.lastHire = time.Time{} // nobody was hired, so the next try needn't wait
	}
	d.changed()
	d.mu.Unlock()
	return status, message
}

// hireIntoT3Now is the hire itself; profiles are the accounts its fence denies (claudeDenyProfiles).
func (d *Deck) hireIntoT3Now(ask t3Ask, role Role, on bool, profiles []claudeProfile) (int, string) {
	t3Desk.Lock()
	defer t3Desk.Unlock()
	machine := newT3Machine()
	if why := t3LaneRefusal(on, t3StatusNow(true, machine)); why != "" {
		return http.StatusConflict, why
	}
	hireID := strings.ToLower(newID())[:12]
	model := cmpOr(role.Model, t3DefaultModel)
	how := hiredIntoGeneralFolder
	if ask.Pinned {
		how = hiredAtPinnedDesk
	}
	hiring := t3Hiring{HireID: hireID, Name: cmpOr(ask.Nickname, role.ID), Called: cmpOr(ask.Nickname, role.Name), Folder: ask.Folder,
		Model: model, Taken: takenPorts()}
	ways := t3Ways{
		Machine: machine, Git: t3Stand.Git, PortFree: t3Stand.PortFree,
		Settings: func() (map[string]any, error) { return t3FenceSettings(hireID, profiles) },
		Brief: func(place hirePlace) (t3Brief, error) {
			message, brief, err := engineBrief(engineRequest{Role: role, How: how, Folder: ask.Folder, Desk: ask.Desk, Answers: ask.Answers, Task: ask.Task},
				place, hireID, time.Now())
			return t3Brief{Message: message, Path: brief}, err
		},
		DiscardBrief: func() { _ = os.RemoveAll(hireFolder(supportDir, hireID)) },
		Register: func(place hirePlace) error {
			hire := Hire{ID: hireID, Name: role.Name, Nickname: ask.Nickname, Role: role.ID, Folder: ask.Folder, Branch: place.Branch,
				Worktree: place.Worktree, Repo: place.Repo, Model: model, Ports: place.Ports, Started: float64(time.Now().UnixMilli()),
				State: "starting", Engine: t3Engine}
			// Its desk in the office, once its thread runs: on the floor it was hired onto (t3desks.go), kept there.
			floor := ask.Floor
			hire.T3Floor = &floor
			return updateRegistry(func(f *registryFile) { f.hires = append(f.hires, hire) })
		},
		Started: func(thread, link, brief string) {
			_ = updateRegistry(func(f *registryFile) {
				if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == hireID }); i >= 0 {
					f.hires[i].T3Thread, f.hires[i].T3Link, f.hires[i].Brief = thread, link, brief
				}
			})
		},
		Fail: func(why string) { t3MarkFailed(registryPath(), hireID, "Didn't start in T3 Code: "+why) },
	}
	if _, err := t3HireInto(hiring, ways); err != nil {
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message
		}
		return http.StatusInternalServerError, err.Error()
	}
	return http.StatusNoContent, ""
}

// registryHire is hire id as the registry has it now.
func registryHire(id string) (Hire, bool) {
	hires := readRegistry(registryPath(), false).hires
	if i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == id }); i >= 0 {
		return hires[i], true
	}
	return Hire{}, false
}

// letGoOfT3 is POST /setup/t3code/letgo {id}: a hire in T3 Code let go (t3LetGo), after any hire into T3 Code
// under way; the registry says so. The page alone, after its confirm.
func (d *Deck) letGoOfT3(id string) (int, any) {
	asked, found := registryHire(id)
	if id == "" || !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	// Not while it's being hired: said at once, rather than after the hire.
	if t3IsHiring(id) {
		return http.StatusConflict, t3StillHiring(calledHire(asked))
	}
	t3Desk.Lock()
	// Read again once it's this let go's turn, so a second click finds the hire already gone.
	hire, found := registryHire(id)
	var gone t3Gone
	var err error
	if found {
		gone, err = t3LetGo(hire, t3Leaver(), func(g t3Gone) {
			note := "Let go from T3 Code: its thread there was removed; its worktree and branch were kept."
			if g.Note != "" {
				note += " " + g.Note
			}
			markFired(hire.ID, note)
		})
	}
	t3Desk.Unlock()
	if !found {
		return http.StatusNotFound, t3NotOnTeam
	}
	d.mu.Lock()
	d.changed()
	d.mu.Unlock()
	if err != nil {
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message
		}
		return http.StatusInternalServerError, err.Error()
	}
	return http.StatusOK, map[string]any{"ok": true, "recap": gone.Recap, "note": gone.Note, "worktree": hire.Worktree, "branch": hire.Branch}
}

// startT3Follow looks at every T3 hire's thread, each no more than every t3FollowEvery, one round at a time off
// the deck's lock: its status for its desk, and the registry's state and link kept as T3 Code says. At launch,
// a hire left part way by a Smolcorp that closed is let go and marked failed. Both stop with the deck's rounds,
// and keep to the registry the deck started with.
func (d *Deck) startT3Follow() {
	registry := registryPath()
	var left []Hire
	for _, h := range readRegistry(registry, false).hires {
		if h.Engine == t3Engine && h.State == "starting" && h.T3Thread == "" {
			left = append(left, h)
		}
	}
	if len(left) > 0 {
		d.rounds.run(func() {
			t3Desk.Lock()
			reasons := t3Reconcile(left, t3Leaver())
			t3Desk.Unlock()
			for _, r := range reasons {
				t3MarkFailed(registry, r.ID, r.Reason)
			}
			d.mu.Lock()
			d.changed()
			d.mu.Unlock()
		})
	}
	d.rounds.every(t3FollowTick, func() { d.t3FollowTick(registry) })
}

// t3FollowTick is one tick of the follow, on the registry at path. Nothing while T3 Code is off, when t3 is never
// run (RM5a N7): its desks show what was last seen until it's back on.
func (d *Deck) t3FollowTick(registry string) {
	d.mu.Lock()
	on := t3CodeAllowed(d.config)
	d.mu.Unlock()
	if !on {
		return
	}
	var hires []Hire
	for _, h := range readRegistry(registry, false).hires {
		if h.Engine == t3Engine && h.T3Thread != "" && isOpen(h) {
			hires = append(hires, h)
		}
	}
	if len(hires) == 0 {
		return
	}
	d.t3.tick(hires, newT3Machine(), time.Now, d.rounds.run, func(seen []t3Seen, moved bool) {
		if t3Apply(registry, seen) || moved {
			d.mu.Lock()
			d.changed()
			d.mu.Unlock()
		}
	})
}

// t3Apply keeps the registry at path as the follow saw it, for hires still open: their state and their link.
// Says whether anything changed; the registry is written only then.
func t3Apply(path string, seen []t3Seen) bool {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	changed := false
	for _, s := range seen {
		i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == s.ID && isOpen(h) })
		if i < 0 {
			continue
		}
		if state := t3State(s.Look.Status); state != "" && file.hires[i].State != state {
			file.hires[i].State, changed = state, true
		}
		if s.Look.Link != "" && file.hires[i].T3Link != s.Look.Link {
			file.hires[i].T3Link, changed = s.Look.Link, true
		}
	}
	if changed {
		_ = file.save(path)
	}
	return changed
}

// t3Latest is h with the follow's latest link for its thread, for its desk's open.
func (d *Deck) t3Latest(h Hire) Hire {
	if look, ok := d.t3.lastSeen(h.ID); ok && look.Link != "" {
		h.T3Link = look.Link
	}
	return h
}

// t3FireRefusal is why a fire from the office, a phone or the GM doesn't reach a hire in T3 Code: it's let go
// from Setup's team list or its desk, after a confirm that says what's removed. "" for any other hire.
func t3FireRefusal(h Hire) string {
	if h.Engine != t3Engine {
		return ""
	}
	return calledHire(h) + " runs in T3 Code: let it go from Setup's team list or from its desk."
}

// gmT3FireRefusal is the GM's fire of a hire in T3 Code, refused in the Mac's words (Deck.fire), or "" for any other hire.
func gmT3FireRefusal(h Hire) string {
	if h.Engine != t3Engine {
		return ""
	}
	return calledHire(h) + " runs in T3 Code: let it go from Setup's team list or from its desk."
}
