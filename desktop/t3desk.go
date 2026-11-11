package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The T3 Code desk (EXPERIMENTAL), as the Mac app's T3Desk.swift: a hire whose agent runs in T3 Code, one
// thread for its one task, in a worktree and branch of its own made as every hire's are (prepareHire).
// Smolcorp starts no agent itself here: it uses T3 Code's own t3 command, its arguments as a list, never
// through a shell, and only with the argument lists t3desk_test.go pins.
//
// The fence is T797a's method A. T3 Code runs Claude Code with its permissions bypassed, and the deny rules
// and the hook still apply from the worktree's .claude/settings.local.json. So that file gets exactly what
// hireSettingsObject gives a hire's own settings file (the deny rules, the guard before every tool call),
// written as the Mac writes it, byte for byte; it's kept out of commits by the repo's info/exclude, and it's
// on disk BEFORE T3 Code hears of the folder. Then the worktree is added as a T3 project of its own and the
// thread started there, without T3's --worktree. If the fence or the project can't be confirmed, everything
// made for the hire is taken back: a thread never starts without the fence in place.
//
// Only Claude Code reads that file, so a hire in T3 Code is always on T3 Code's Claude provider.

const (
	// t3Engine is the registry's word for a hire whose agent runs in T3 Code.
	t3Engine = "t3"
	// t3Provider is T3 Code's Claude provider: the fence is Claude Code's settings file, which no other agent reads.
	t3Provider = "claudeAgent"
	// t3DefaultModel is the model when neither the hire nor its role names one.
	t3DefaultModel = "claude-sonnet-5-5"
	// t3FenceName is the fence, in the worktree, as git and Claude Code name it.
	t3FenceName = ".claude/settings.local.json"
	t3Tag       = "EXPERIMENTAL · runs in T3 Code"
	// t3FollowEvery is how often a hire's thread is looked at, at most; t3FollowTick how often the
	// follow's clock ticks, so a due hire is looked at within it.
	t3FollowEvery = 10 * time.Second
	t3FollowTick  = 2 * time.Second
	// t3Within is how long one t3 may take.
	t3Within = 30 * time.Second
	// t3LeaveWithin is how long, after its project is removed, T3 Code's agent has to leave the worktree.
	t3LeaveWithin = 5 * time.Second

	t3SwitchedOff  = "T3 Code is off in Settings: turn it on to hire into T3 Code."
	t3FenceLost    = "The hire's fence was gone or changed before its thread could start, so no thread was started and the hire was taken back."
	t3HooksOff     = "This repository's own .claude/settings.local.json switches Claude Code's hooks off (disableAllHooks), which would compete with the hire's fence, so Smolcorp won't hire into T3 Code here."
	t3LetGoConfirm = "T3 Code's thread for this hire, and its history in T3 Code, are removed. Its worktree and branch stay."
	t3NotOnTeam    = "That hire isn't on the team."
)

// t3Supported are the t3 versions the fence was proven on (T797a). Any other is refused, by name.
var t3Supported = []string{"v0.1.8"}

// t3LaneRefusal is why a hire into T3 Code isn't offered or taken now, in one sentence; "" when it is. on is
// the T3 Code choice (t3CodeAllowed), status what's installed, with t3's version.
func t3LaneRefusal(on bool, status t3Status) string {
	if !on {
		return t3SwitchedOff
	}
	switch status.State {
	case t3None, "":
		return "Install T3 Code and its t3 command to hire into T3 Code (t3.codes)."
	case t3AppOnlyIs:
		return "Hiring into T3 Code needs its t3 command: in T3 Code, open Settings → About and install it."
	}
	if !slices.Contains(t3Supported, status.Version) {
		if status.Version == "" {
			return "Smolcorp couldn't read t3's version, so it won't hire into T3 Code."
		}
		return "t3 " + status.Version + " isn't a version Smolcorp's fence was proven on (" + strings.Join(t3Supported, ", ") + "), so it won't hire into T3 Code."
	}
	return ""
}

// t3HireRefusal is why this hire can't go into T3 Code, whatever's installed: only a Claude Code role (the
// fence is Claude Code's file), on T3 Code's own account, never the General Manager, never from a test copy
// (T3 Code is the real one). role is nil for none.
func t3HireRefusal(role *Role, account string, apiKey, testCopy bool) string {
	switch {
	case testCopy:
		return "A test copy doesn't hire into T3 Code: that's your real T3 Code."
	case role == nil:
		return "Pick a role: a hire in T3 Code starts on its brief."
	case role.ID == gmRole:
		return "The General Manager isn't hired into T3 Code."
	case agentOrClaude(role.Agent) != Claude:
		return "Only a Claude Code role is hired into T3 Code: the fence there is Claude Code's."
	case apiKey || account != "":
		return "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it."
	}
	return ""
}

// t3Hiring is one hire into T3 Code, as its route has checked it.
type t3Hiring struct {
	HireID string
	// Name is its worktree's and branch's name: the nickname, else the role.
	Name string
	// Called is what T3 Code's project is called by: "<called> · <branch>" (see t3Title).
	Called string
	// Folder is the folder it's hired into, inside a git repo.
	Folder string
	Model  string
	// Worktrees is the folder set for worktrees, "" for beside the repo; Taken the ports other hires hold.
	Worktrees string
	Taken     []string
}

// t3Brief is a hire's first message, and its brief's path ("" for none).
type t3Brief struct{ Message, Path string }

// t3Ways are what a hire into T3 Code does besides t3 and git, so tests give their own.
type t3Ways struct {
	Machine  t3Machine
	Git      gitRunner
	PortFree func(int) bool
	// Settings are the hire's settings, as hireSettingsObject makes them for it: its fence.
	Settings func() (map[string]any, error)
	// Brief writes its brief (as every managed hire's, in its folder in Smolcorp's).
	Brief        func(place hirePlace) (t3Brief, error)
	DiscardBrief func()
	// Register writes its registry entry, with engine t3, once its worktree is made.
	Register func(place hirePlace) error
	// Started gives its thread, link and brief, once the thread runs in its worktree.
	Started func(thread, link, brief string)
	// Fail says in the registry why it didn't start.
	Fail func(why string)
}

// t3Started is a hire whose thread runs in its worktree.
type t3Started struct {
	Place        hirePlace
	Thread, Link string
}

// The hires being hired into T3 Code now, and the one lock hires and let-gos take in turn: a let go comes
// strictly before a hire is registered or after its thread is known (the Mac's T3Desk.queue).
var (
	t3Desk     sync.Mutex
	t3InFlight = struct {
		sync.Mutex
		ids map[string]bool
	}{ids: map[string]bool{}}
)

// t3IsHiring is whether hire id is being hired into T3 Code now.
func t3IsHiring(id string) bool {
	t3InFlight.Lock()
	defer t3InFlight.Unlock()
	return t3InFlight.ids[id]
}

func t3StillHiring(name string) string { return name + " is still being hired, try again in a moment." }

// t3Refused is a refusal of a hire into T3 Code.
func t3Refused(status int, message string) error {
	return &hireRefusal{Status: status, Message: message}
}

// t3HireInto hires into T3 Code, in T797a's order: the worktree and branch, the registry entry, the fence and its
// exclude line, the brief, `t3 project add`, `t3 thread start` without --worktree, then a check that the
// thread runs in the worktree. Refused at any step with everything made taken back. It runs git and t3.
func t3HireInto(h t3Hiring, ways t3Ways) (t3Started, error) {
	machine, git := ways.Machine, ways.Git
	if git == nil {
		git = runGit
	}
	// While it's being hired, it isn't let go (t3Leave): a let go part way could take its fence away before
	// its thread starts.
	t3InFlight.Lock()
	t3InFlight.ids[h.HireID] = true
	t3InFlight.Unlock()
	defer func() {
		t3InFlight.Lock()
		delete(t3InFlight.ids, h.HireID)
		t3InFlight.Unlock()
	}()
	program := t3Program(machine)
	if program == "" {
		return t3Started{}, t3Refused(http.StatusConflict, t3NoneHint)
	}
	// The main checkout's own local settings merge with the fence (T797a, run 4): one that switches hooks off
	// competes with the guard.
	if t3HooksSwitchedOff(h.Folder, git) {
		return t3Started{}, t3Refused(http.StatusConflict, t3HooksOff)
	}
	// 1. The worktree and branch, named as every hire's. T3 Code's agent works on T3's own Claude account, so
	//    no account of Smolcorp's is trusted or checked for it.
	place, err := prepareHire(hireRequest{HireID: h.HireID, Name: h.Name, Folder: h.Folder, Worktree: true},
		hirePlaceOptions{Worktrees: h.Worktrees, Taken: h.Taken, Git: git, PortFree: ways.PortFree})
	if err != nil {
		var refused *hireRefusal
		if errors.As(err, &refused) {
			return t3Started{}, refused
		}
		return t3Started{}, t3Refused(http.StatusInternalServerError, err.Error())
	}
	if place.Worktree == "" {
		return t3Started{}, t3Refused(http.StatusConflict, "A hire in T3 Code works in a worktree of its own: hire into a folder in a git repository.")
	}
	worktree := place.Worktree
	fence := filepath.Join(worktree, filepath.FromSlash(t3FenceName))
	claudeFolder := filepath.Dir(fence)
	_, statErr := os.Stat(claudeFolder)
	madeClaudeFolder := errors.Is(statErr, os.ErrNotExist)
	title := h.Called + " · " + place.Branch
	// Only a fence Smolcorp wrote is ever taken away: never a file the repo had.
	wroteFence := false

	// takeBack takes back what was made, says why in the registry, and refuses. A project T3 Code may still
	// have keeps its fence and worktree.
	takeBack := func(status int, why string, project, brief bool) (t3Started, error) {
		message := why
		if project {
			removed := machine.Run(program, []string{"project", "remove", worktree, "--force"}, t3Within)
			if !(removed.Ended && removed.Code == 0) && !t3NoProject(removed) {
				message += " T3 Code didn't remove the project it was given (" + t3Said(removed) + "), so the hire's worktree and fence were kept: remove \"" + title + "\" in T3 Code."
				ways.Fail(message)
				return t3Started{}, t3Refused(status, message)
			}
		}
		if wroteFence {
			folder := ""
			if madeClaudeFolder {
				folder = claudeFolder
			}
			t3RemoveFence(fence, folder)
		}
		if brief {
			ways.DiscardBrief()
		}
		if !removeWorkspace(place.Repo, worktree, place.Branch, git) {
			message += " Its worktree " + worktree + " was kept: git wouldn't remove it."
		}
		ways.Fail(message)
		return t3Started{}, t3Refused(status, message)
	}

	// 2. The registry entry, with engine t3.
	if err := ways.Register(place); err != nil {
		removeWorkspace(place.Repo, worktree, place.Branch, git)
		return t3Started{}, t3Refused(http.StatusInternalServerError, "Couldn't save the hire in the registry.")
	}
	// 3. The fence, before T3 Code hears of the folder: never in a file the repo already has (it would be committed).
	if _, err := os.Lstat(fence); err == nil {
		return takeBack(http.StatusConflict, "This repository keeps its own "+t3FenceName+", so Smolcorp can't put the hire's fence there.", false, false)
	}
	settings, err := ways.Settings()
	if err != nil {
		var refused *hireRefusal
		if errors.As(err, &refused) {
			return takeBack(refused.Status, refused.Message, false, false)
		}
		return takeBack(http.StatusInternalServerError, "The hire's fence couldn't be made, so it wasn't started.", false, false)
	}
	wroteFence = true
	if !t3WriteFence(settings, fence) {
		return takeBack(http.StatusInternalServerError, "The hire's fence couldn't be written in its worktree, so it wasn't started.", false, false)
	}
	// 4. Never committed: the repo's info/exclude, which every worktree of it shares.
	if !t3Exclude(worktree, git) {
		return takeBack(http.StatusInternalServerError, "Smolcorp couldn't keep the hire's fence out of git, so it wasn't started.", false, false)
	}
	// 5. The brief, as every managed hire's.
	brief, err := ways.Brief(place)
	if err != nil {
		return takeBack(http.StatusInternalServerError, err.Error(), false, true)
	}
	// The fence is checked again right before T3 Code hears of the folder, and right before the thread starts.
	if !t3FenceHolds(fence, settings) {
		return takeBack(http.StatusConflict, t3FenceLost, false, true)
	}
	// 6. The worktree as a T3 project of its own: then the thread can't land in a project around it (T797a 2a).
	added := machine.Run(program, []string{"project", "add", worktree, "--title", title}, t3Within)
	if !added.Ended || added.Code != 0 {
		// "already exists" is someone else's project: it's left alone. No answer at all: T3 Code may have added it anyway.
		return takeBack(http.StatusBadGateway, "T3 Code didn't add the hire's worktree as a project: "+t3Said(added), !added.Ended, true)
	}
	project := t3AddedProject(added.Output, worktree)
	if project == "" {
		return takeBack(http.StatusBadGateway, "T3 Code didn't say it added the hire's worktree as its own project, so the thread wasn't started.", true, true)
	}
	if !t3FenceHolds(fence, settings) {
		return takeBack(http.StatusConflict, t3FenceLost, true, true)
	}
	// 7. The thread, in that folder, told to read its brief.
	start := machine.Run(program, []string{"thread", "start", worktree, brief.Message, "--provider", t3Provider, "--model", h.Model, "--json"}, t3Within)
	thread := ""
	if start.Ended && start.Code == 0 {
		thread, _ = t3Object(start.Output)["threadId"].(string)
	}
	if thread == "" {
		return takeBack(http.StatusBadGateway, "T3 Code didn't start the hire's thread: "+t3Said(start), true, true)
	}
	// 8. It runs in the worktree: T3 made no worktree of its own, and the thread is in the project just added.
	shown := t3Show(thread, program, machine)
	if shown == nil {
		return takeBack(http.StatusBadGateway, "Smolcorp couldn't confirm the thread runs in the hire's worktree, so it was taken back.", true, true)
	}
	if in, _ := shown["projectId"].(string); in != project {
		// Removing the hire's own project doesn't reach a thread in another one, and T3 Code has no stop: said as it is.
		which := cmpOr(in, "it didn't say which")
		return takeBack(http.StatusBadGateway, "T3 Code started the hire's thread in another project ("+which+"), not in its worktree; Smolcorp removed only the hire's own project, and T3 Code has no stop, so that thread may still be running there, outside the fence: stop it in T3 Code.", true, true)
	}
	if elsewhere, ok := shown["worktreePath"].(string); ok {
		// In the hire's project still, so removing that project ends it (T797a).
		return takeBack(http.StatusBadGateway, "T3 Code put the hire's thread in a worktree of its own ("+elsewhere+"), not the hire's, so it was taken back with its project.", true, true)
	}
	link, _ := shown["link"].(string)
	ways.Started(thread, link, brief.Path)
	return t3Started{Place: place, Thread: thread, Link: link}, nil
}

// t3FenceBytes are the fence as it's written: as the Mac's JSONSerialization writes it, byte for byte.
func t3FenceBytes(settings map[string]any) []byte { return swiftJSON(settings) }

// t3WriteFence writes the fence whole, private to you, and reads it back the same.
func t3WriteFence(settings map[string]any, fence string) bool {
	if os.MkdirAll(filepath.Dir(fence), 0o755) != nil || writePrivate(fence, t3FenceBytes(settings)) != nil {
		return false
	}
	return t3FenceHolds(fence, settings)
}

// t3FenceHolds is whether the fence is on disk and holds what was written, to the byte.
func t3FenceHolds(fence string, settings map[string]any) bool {
	back, err := os.ReadFile(fence)
	return err == nil && bytes.Equal(back, t3FenceBytes(settings))
}

// t3RemoveFence takes the fence away and, given (Smolcorp made it), its .claude folder while it's empty.
// Says whether the fence is gone.
func t3RemoveFence(fence, folder string) bool {
	_ = os.Remove(fence)
	if folder != "" {
		if entries, err := os.ReadDir(folder); err == nil && len(entries) == 0 {
			_ = os.Remove(folder)
		}
	}
	_, err := os.Lstat(fence)
	return errors.Is(err, os.ErrNotExist)
}

// t3HooksSwitchedOff is whether the main checkout of folder's repo has its own .claude/settings.local.json
// with disableAllHooks true. Claude Code merges that file with a worktree's own at the same level (T797a,
// run 4), so it would compete with the fence's false. Only that one key is read.
func t3HooksSwitchedOff(folder string, git gitRunner) bool {
	status, common, ok := git(folder, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok || status != 0 {
		return false
	}
	dir := filepath.Clean(strings.TrimSpace(common))
	// A bare repo has no main checkout.
	if filepath.Base(dir) != ".git" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(dir), filepath.FromSlash(t3FenceName)))
	if err != nil {
		return false
	}
	var local struct {
		DisableAllHooks any `json:"disableAllHooks"`
	}
	return json.Unmarshal(data, &local) == nil && local.DisableAllHooks == true
}

// t3Exclude puts .claude/settings.local.json in the repo's info/exclude (once), then asks git that it's
// ignored and not listed.
func t3Exclude(worktree string, git gitRunner) bool {
	status, named, ok := git(worktree, "rev-parse", "--git-common-dir")
	named = strings.TrimSpace(named)
	if !ok || status != 0 || named == "" {
		return false
	}
	base := named
	if !filepath.IsAbs(base) {
		base = filepath.Join(worktree, base)
	}
	file := filepath.Join(base, "info", "exclude")
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	text := string(data)
	listed := false
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if strings.TrimSpace(line) == t3FenceName {
			listed = true
		}
	}
	if !listed {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += t3FenceName + "\n"
		if os.MkdirAll(filepath.Dir(file), 0o755) != nil || os.WriteFile(file, []byte(text), 0o644) != nil {
			return false
		}
	}
	if status, _, ok := git(worktree, "check-ignore", "-q", t3FenceName); !ok || status != 0 {
		return false
	}
	status, shown, ok := git(worktree, "status", "--porcelain", "--untracked-files=all", "--", ".claude")
	return ok && status == 0 && strings.TrimSpace(shown) == ""
}

// t3AddedProject is the project's id when `t3 project add` says it added folder itself: "Added project <id>
// (<title>) at <folder>." (a title or a folder may hold " at ", so the folder is matched whole at the end);
// "" otherwise.
func t3AddedProject(output, folder string) string {
	ends := []string{" at " + folder + ".", " at " + physical(folder) + "."}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		rest, found := strings.CutPrefix(line, "Added project ")
		if !found || !slices.ContainsFunc(ends, func(end string) bool { return strings.HasSuffix(line, end) }) {
			continue
		}
		id, _, _ := strings.Cut(rest, " ")
		return id
	}
	return ""
}

// t3Show is the thread as `t3 thread show <id> --json` gives it, or nil.
func t3Show(thread, program string, machine t3Machine) map[string]any {
	ran := machine.Run(program, []string{"thread", "show", thread, "--json"}, t3Within)
	if !ran.Ended || ran.Code != 0 {
		return nil
	}
	shown, _ := t3Object(ran.Output)["thread"].(map[string]any)
	return shown
}

// t3Object is text read as a JSON object, or nil.
func t3Object(text string) map[string]any {
	var object map[string]any
	if json.Unmarshal([]byte(text), &object) != nil {
		return nil
	}
	return object
}

// t3NoProject is whether t3 said there's no such project: one already removed is as good as removed now.
func t3NoProject(ran t3Ran) bool {
	return strings.Contains(ran.Output+ran.Error, "No T3 Code project matches")
}

// t3Said is the first line t3 said, on its errors first, cut to 200 characters; what's said when it said nothing.
func t3Said(ran t3Ran) string {
	line := firstLine(cmpOr(ran.Error, ran.Output))
	switch {
	case line != "":
		return clip(line, 200)
	case !ran.Ended:
		return "it didn't answer"
	}
	return "it gave no reason"
}

// Following

// t3Glance is what the last look at a hire's thread found: the desk's word for it (starting, working, waiting,
// done, unknown), T3 Code's own ("running") or why it couldn't be seen, and its link.
type t3Glance struct{ Status, Raw, Link string }

// t3DeskStatus is T3 Code's thread status as the desk says it (T797a §4).
func t3DeskStatus(thread map[string]any) string {
	raw, _ := thread["status"].(string)
	pending, _ := thread["pendingRequestCount"].(float64)
	switch {
	case raw == "waiting" || pending > 0:
		return "waiting"
	case slices.Contains([]string{"preparing", "queued", "starting", "running"}, raw):
		return "working"
	case slices.Contains([]string{"completed", "failed", "interrupted", "cancelled", "rolled_back", "idle"}, raw):
		return "done"
	}
	return "unknown"
}

// t3State is the registry's state for what was seen: at work or waiting is working, done is idle; "" changes nothing.
func t3State(status string) string {
	switch status {
	case "working", "waiting":
		return "working"
	case "done":
		return "idle"
	}
	return ""
}

// t3Follower follows the hires in T3 Code: one round at a time, each hire no more than every t3FollowEvery.
type t3Follower struct {
	sync.Mutex
	following bool
	looked    map[string]time.Time
	seen      map[string]t3Glance
}

// newT3Follower is a deck's follower: each deck has its own, so one stopped never holds another's back.
func newT3Follower() *t3Follower {
	return &t3Follower{looked: map[string]time.Time{}, seen: map[string]t3Glance{}}
}

// lastSeen is what the last look at hire id's thread found.
func (f *t3Follower) lastSeen(id string) (t3Glance, bool) {
	if f == nil {
		return t3Glance{}, false
	}
	f.Lock()
	defer f.Unlock()
	look, ok := f.seen[id]
	return look, ok
}

// t3Seen is one hire's look.
type t3Seen struct {
	ID   string
	Look t3Glance
}

// tick starts a round on a goroutine of its own (spawn), unless the last one hasn't come back yet: then
// nothing, so rounds never pile up behind a slow T3 Code. What it saw, and whether a desk's look moved, go
// to apply. Says whether a round began. Hires and let-gos never wait for it: it doesn't take t3Desk.
func (f *t3Follower) tick(hires []Hire, machine t3Machine, now func() time.Time, spawn func(func()), apply func(seen []t3Seen, moved bool)) bool {
	f.Lock()
	if f.following {
		f.Unlock()
		return false
	}
	f.following = true
	f.Unlock()
	spawn(func() {
		var seen []t3Seen
		moved := false
		defer func() {
			f.Lock()
			f.following = false
			f.Unlock()
			apply(seen, moved)
		}()
		before := map[string]t3Glance{}
		for _, h := range hires {
			before[h.ID], _ = f.lastSeen(h.ID)
		}
		seen = f.round(hires, machine, now)
		for _, s := range seen {
			moved = moved || before[s.ID] != s.Look
		}
	})
	return true
}

// round looks at each open hire's thread, one at a time, each no more than every t3FollowEvery (a hire
// looked at sooner is left for a later round), and gives what was seen. It runs t3.
func (f *t3Follower) round(hires []Hire, machine t3Machine, now func() time.Time) []t3Seen {
	program := t3Program(machine)
	var found []t3Seen
	for _, h := range hires {
		if h.Engine != t3Engine || h.T3Thread == "" || !isOpen(h) {
			continue
		}
		at := now()
		f.Lock()
		last, ok := f.looked[h.ID]
		due := !ok || at.Sub(last) >= t3FollowEvery
		if due {
			f.looked[h.ID] = at
		}
		f.Unlock()
		if !due {
			continue
		}
		look := t3Glance{Status: "unknown", Raw: "t3 isn't there"}
		if program != "" {
			look.Raw = "T3 Code didn't answer"
			if thread := t3Show(h.T3Thread, program, machine); thread != nil {
				raw, _ := thread["status"].(string)
				link, _ := thread["link"].(string)
				look = t3Glance{Status: t3DeskStatus(thread), Raw: raw, Link: link}
			}
		}
		f.Lock()
		f.seen[h.ID] = look
		f.Unlock()
		found = append(found, t3Seen{h.ID, look})
	}
	return found
}

// The desk

// t3DeskJSON is what a T3 hire's desk shows besides its name and branch: the tag, its status, and whether it
// has a thread to open.
func t3DeskJSON(h Hire, f *t3Follower) map[string]any {
	look, seen := f.lastSeen(h.ID)
	status := look.Status
	if !seen {
		status = "unknown"
		if h.T3Thread == "" {
			status = "starting"
		}
	}
	return map[string]any{"tag": t3Tag, "status": status, "raw": look.Raw, "thread": h.T3Thread,
		"canOpen": t3DeepLink(h.T3Thread, h.T3Link) != ""}
}

// t3Decorate gives each T3 hire's row of the team, as the page gets it, its desk (t3).
func t3Decorate(rows []map[string]any, hires []Hire, f *t3Follower) []map[string]any {
	for _, row := range rows {
		if row["engine"] != t3Engine {
			continue
		}
		if i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == row["id"] }); i >= 0 {
			row["t3"] = t3DeskJSON(hires[i], f)
		}
	}
	return rows
}

// t3DeepLink is T3 Code's own link to a thread: t3code://threads/<env>/<thread id, URL-encoded>, the env being
// the first part of link's path after /threads/ (https://app.t3.codes/threads/<env>/<thread>). "" without
// both, or with an env that isn't plain letters, digits and dashes.
func t3DeepLink(thread, link string) string {
	if thread == "" {
		return ""
	}
	given, err := url.Parse(link)
	if err != nil || link == "" {
		return ""
	}
	// The path as sent, so an encoded slash stays inside its part.
	parts := strings.Split(given.EscapedPath(), "/")
	at := slices.Index(parts, "threads")
	if at < 0 || at+1 >= len(parts) || parts[at+1] == "" {
		return ""
	}
	env := parts[at+1]
	for _, r := range env {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return ""
		}
	}
	return "t3code://threads/" + env + "/" + t3Encoded(thread)
}

// t3Encoded is text with everything but ASCII letters, digits and -._~ percent-encoded.
func t3Encoded(text string) string {
	var b strings.Builder
	for _, c := range []byte(text) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~", c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteString("%" + strings.ToUpper(strconv.FormatUint(uint64(c)>>4, 16)+strconv.FormatUint(uint64(c)&15, 16)))
		}
	}
	return b.String()
}

// t3OpenThread is Open in T3 Code for a hire in T3 Code: its thread, by T3 Code's own link (the follow's
// latest, which its route puts in h), handed to this computer's opener. Never a shell, never t3.
func t3OpenThread(h Hire, status t3Status, machine t3Machine) (int, any) {
	if status.State != t3Found && status.State != t3AppOnlyIs {
		return http.StatusConflict, "T3 Code's app isn't on this computer, so the hire's thread can't be shown there."
	}
	deep := t3DeepLink(h.T3Thread, h.T3Link)
	if deep == "" {
		return http.StatusConflict, "T3 Code hasn't given this hire's thread a link yet."
	}
	if machine.OpenURL == nil || machine.OpenURL(deep) != nil {
		return http.StatusBadGateway, "T3 Code didn't take the link to the hire's thread."
	}
	return http.StatusOK, map[string]any{"ok": true, "opened": "link"}
}

// Mail

// t3Mail is the GM's mail to a hire in T3 Code: `t3 thread send <id> -`, the text on its standard input,
// never typed anywhere. A busy thread takes it as its next turn: delivered, and the outbox says it's queued
// there. commit counts the try, just before it's sent.
func t3Mail(h Hire, text string, commit func() bool, machine t3Machine) mailResult {
	if !commit() {
		return mailResult{Outcome: mailTrouble, Note: "The outbox couldn't be written, so nothing was sent."}
	}
	name := calledHire(h)
	if h.T3Thread == "" {
		return mailResult{Outcome: mailCannot, Note: name + " has no T3 Code thread, so mail can't reach it."}
	}
	program := t3Program(machine)
	if program == "" {
		return mailResult{Outcome: mailTrouble, Note: "T3 Code's t3 command isn't there, so mail can't reach " + name + " now."}
	}
	var ran t3Ran
	if machine.Feed != nil {
		ran = machine.Feed(program, []string{"thread", "send", h.T3Thread, "-", "--json"}, []byte(text), t3Within)
	}
	// No answer: it may have been taken, so it isn't sent again.
	if !ran.Ended {
		return mailResult{Outcome: mailCannot, Note: "T3 Code didn't answer, so it isn't known whether " + name + " got it; it won't be sent again."}
	}
	if ran.Code != 0 {
		return mailResult{Outcome: mailTrouble, Note: "T3 Code didn't take it: " + t3Said(ran)}
	}
	// T797a: "delivery":"started", or "queued" when the thread is busy.
	if delivery, _ := t3Object(ran.Output)["delivery"].(string); delivery == "queued" {
		return mailResult{Outcome: mailSent, Note: "queued: " + name + "'s thread in T3 Code was busy, so it's its next turn."}
	}
	return mailResult{Outcome: mailSent}
}

// Letting go

// t3Leaving is what letting go uses besides t3: the processes working in a folder now (read from the
// system, nothing run; false when it can't look) and their names, and the clock.
type t3Leaving struct {
	Machine t3Machine
	Working func(folder string) (pids []int, looked bool)
	Name    func(pid int) string
	Pause   func(time.Duration)
	Now     func() time.Time
}

func defaultT3Leaving() t3Leaving {
	return t3Leaving{Machine: newT3Machine(), Working: processesWorkingIn, Name: processName, Pause: time.Sleep, Now: time.Now}
}

// t3Gone is a hire let go: its recap, and what's to know besides (its fence couldn't be taken away), or "".
type t3Gone struct{ Recap, Note string }

func t3FenceKept(h Hire) string {
	return calledHire(h) + "'s fence couldn't be taken out of its worktree, so that folder is still fenced: " + filepath.Join(h.Worktree, filepath.FromSlash(t3FenceName)) + "."
}

// t3LetGo lets a hire in T3 Code go: T3 has no stop, so its project is removed with --force (which ends T3's
// agent for it, T797a), then no process may still work in its worktree; only then is the fence taken away,
// the recap written and the registry told (letGone). Any step that fails says so and stops there; a hire
// still being hired isn't let go at all.
func t3LetGo(h Hire, leaving t3Leaving, letGone func(t3Gone)) (t3Gone, error) {
	fenceGone, err := t3Leave(h, leaving)
	if err != nil {
		return t3Gone{}, err
	}
	gone := t3Gone{Recap: writeRecap(h, "", false, leaving.Now())}
	if !fenceGone {
		gone.Note = t3FenceKept(h)
	}
	letGone(gone)
	return gone, nil
}

// t3Leave is Let go's steps up to the fence: its project removed, nothing left working in its worktree, the
// fence taken away. Says whether the fence is gone.
func t3Leave(h Hire, leaving t3Leaving) (bool, error) {
	name := calledHire(h)
	switch {
	case h.Engine != t3Engine:
		return false, t3Refused(http.StatusConflict, name+" doesn't run in T3 Code.")
	case t3IsHiring(h.ID):
		return false, t3Refused(http.StatusConflict, t3StillHiring(name))
	case !isOpen(h):
		return false, t3Refused(http.StatusConflict, name+" is already gone.")
	case h.Worktree == "":
		return false, t3Refused(http.StatusConflict, name+" has no worktree on record, so nothing was changed.")
	}
	program := t3Program(leaving.Machine)
	if program == "" {
		return false, t3Refused(http.StatusConflict, "T3 Code's t3 command isn't there, so nothing was changed: "+name+" may still be running in T3 Code.")
	}
	removed := leaving.Machine.Run(program, []string{"project", "remove", h.Worktree, "--force"}, t3Within)
	if !(removed.Ended && removed.Code == 0) && !t3NoProject(removed) {
		return false, t3Refused(http.StatusBadGateway, "T3 Code didn't remove "+name+"'s project ("+t3Said(removed)+"), so nothing else was changed: it may still be running there.")
	}
	end := leaving.Now().Add(t3LeaveWithin)
	still, looked := leaving.Working(h.Worktree)
	for looked && len(still) > 0 && leaving.Now().Before(end) {
		leaving.Pause(500 * time.Millisecond)
		still, looked = leaving.Working(h.Worktree)
	}
	if !looked {
		return false, t3Refused(http.StatusConflict, "T3 Code removed the project, but Smolcorp can't see which processes work in "+name+"'s worktree on this computer, so its fence was kept and it isn't let go yet.")
	}
	if len(still) > 0 {
		// Named, so you know your own terminal or editor there when it's that.
		var named []string
		for _, pid := range still[:min(len(still), 5)] {
			if called := leaving.Name(pid); called != "" {
				named = append(named, called+", PID "+strconv.Itoa(pid))
			} else {
				named = append(named, "PID "+strconv.Itoa(pid))
			}
		}
		return false, t3Refused(http.StatusConflict, "T3 Code removed the project, but a process still works in "+name+"'s worktree ("+strings.Join(named, "; ")+"), so its fence was kept and it isn't let go yet: if that's yours, leave that folder and let go again.")
	}
	fence := filepath.Join(h.Worktree, filepath.FromSlash(t3FenceName))
	return t3RemoveFence(fence, filepath.Dir(fence)), nil
}

// t3Reason is why a hire is marked failed.
type t3Reason struct{ ID, Reason string }

// t3Reconcile lets go of the hires into T3 Code left part way at launch: registered (starting) but with no
// thread, as Smolcorp closed while hiring them (nothing is being hired at launch). Each is let go as Let go
// does; gives each one's reason to be marked failed with: what was done, or where it stopped.
func t3Reconcile(hires []Hire, leaving t3Leaving) []t3Reason {
	var reasons []t3Reason
	for _, h := range hires {
		if h.Engine != t3Engine || h.State != "starting" || h.T3Thread != "" || t3IsHiring(h.ID) {
			continue
		}
		reason := "Smolcorp closed while hiring it into T3 Code, so at the next launch its T3 Code project (if it had one) was removed and its fence taken away. Its worktree and branch stay."
		fenceGone, err := t3Leave(h, leaving)
		switch {
		case err != nil:
			reason = "Smolcorp closed while hiring it into T3 Code, and at the next launch it couldn't be let go: " + err.Error()
		case !fenceGone:
			reason = "Smolcorp closed while hiring it into T3 Code, so at the next launch its T3 Code project (if it had one) was removed. " + t3FenceKept(h)
		}
		reasons = append(reasons, t3Reason{ID: h.ID, Reason: reason})
	}
	return reasons
}

// t3PlacesOf are a folder as it's given and as it really is (links resolved): either may be what a process
// was started with.
func t3PlacesOf(folder string) []string {
	return slices.Compact([]string{physical(folder), folder})
}

// t3WorksIn is whether a process whose current folder is cwd and whose arguments are args works in one of
// folders (or a folder inside it), or was given one with --add-dir, as T3 Code's agent is (T797a). sep is
// the system's separator; fold, whether its paths ignore case (Windows).
func t3WorksIn(folders []string, cwd string, args []string, sep string, fold bool) bool {
	norm := func(path string) string {
		if len(path) > len(sep) {
			path = strings.TrimSuffix(path, sep)
		}
		if fold {
			path = strings.ToLower(path)
		}
		return path
	}
	here := norm(cwd)
	for _, folder := range folders {
		wanted := norm(folder)
		if wanted == "" || !filepath.IsAbs(folder) {
			continue
		}
		if here != "" && (here == wanted || strings.HasPrefix(here, wanted+sep)) {
			return true
		}
		for i, word := range args {
			given, found := strings.CutPrefix(word, "--add-dir=")
			if !found && word == "--add-dir" && i+1 < len(args) {
				given, found = args[i+1], true
			}
			if found && norm(given) == wanted {
				return true
			}
		}
	}
	return false
}
