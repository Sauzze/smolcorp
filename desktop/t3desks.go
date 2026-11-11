package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The office's desks for the hires in T3 Code (T3D-b, experimental), as the Mac's T3Desks. A hire in T3 Code
// has no terminal tab, and Sessions drops every session with none, so its desk is made here, as a cloud bot's
// is: from Smolcorp's own registry alone (engine t3, with its thread and its worktree), never from a hook, a
// name, a title or a folder guessed. Its Claude Code's own hook events are never a desk of their own
// (IngestFrom keeps them off every terminal's desk, isT3HiresOwn).
//
// The desk's look is what the lane's follow last saw of its thread (t3Follower): at work, waiting for an answer
// in T3 Code, or done. Nothing here types into anything: these sessions aren't Sessions', so no key, text,
// effort or fire can find them, and the office is told each control is off (t3Entry). A tap opens the hire's
// own thread in T3 Code; Let go is the lane's (letGoOfT3).

// t3DeskPrefix is how a T3 desk's session id starts: the rest is its hire's id. No hook's session id looks like this.
const t3DeskPrefix = "t3:"

func t3DeskID(hireID string) string { return t3DeskPrefix + hireID }

// t3ControlsOff are the controls a terminal's desk has, every one off at a T3 desk: there's no tab to type into.
var t3ControlsOff = []string{"keys", "fire", "mic", "dial", "dictates", "summary", "phoneMic", "conversation"}

// t3NoControls is why an office control can't reach desk name: nothing types into a hire in T3 Code.
func t3NoControls(name string) string {
	return name + " runs in T3 Code: Smolcorp doesn't type into it. Open it in T3 Code, or mail it."
}

// t3Seated is a T3 desk and its hire, as the registry says now.
type t3Seated struct {
	session *Session
	hire    Hire
}

// t3Desks keeps the T3 desks of a deck, under the deck's lock: each desk's session by its hire's id (so it keeps
// its place and the time it started), the hires as seated last found them, and those whose done was seen.
type t3Desks struct {
	registry string
	// The registry as last read, and the file it was read from, read again only when that changed.
	read     os.FileInfo
	hires    []Hire
	byHire   map[string]*Session
	open     map[string]Hire
	seenDone map[string]bool
}

func newT3Desks(registry string) *t3Desks {
	return &t3Desks{registry: registry, byHire: map[string]*Session{}, open: map[string]Hire{}, seenDone: map[string]bool{}}
}

// registryHires are the registry's hires, read again only when its file changed.
func (t *t3Desks) registryHires() []Hire {
	info, err := os.Stat(t.registry)
	if err != nil {
		t.read, t.hires = nil, nil
		return nil
	}
	if t.read == nil || !os.SameFile(t.read, info) || !t.read.ModTime().Equal(info.ModTime()) || t.read.Size() != info.Size() {
		t.read, t.hires = info, readRegistry(t.registry, false).hires
	}
	return t.hires
}

// t3HasDesk is whether h has a desk: the registry says it runs in T3 Code, with a thread and a worktree, and it's
// still on the team. Let go (fired) or failed, its desk is freed. Its worktree must be where hires' are made.
func t3HasDesk(h Hire, worktrees string) bool {
	return h.Engine == t3Engine && h.T3Thread != "" && isOpen(h) && t3InHiresFolder(h, worktrees)
}

// t3InHiresFolder is whether h's worktree is one Smolcorp would have made (FXT3Da N3): a folder of its own in
// <repo>-hires beside its repo, or in the repo's folder inside the folder set for hires' worktrees (hiresFolder).
// A registry row naming any other folder (the home folder, say) gets no desk and keeps no session off a desk.
func t3InHiresFolder(h Hire, worktrees string) bool {
	if h.Repo == "" || h.Worktree == "" {
		return false
	}
	// Compared as the hire's place keeps them: with no .. left, links resolved.
	plain := func(path string) string { return plainPath(filepath.Clean(path)) }
	worktree := filepath.Clean(h.Worktree)
	if name := filepath.Base(worktree); name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return false
	}
	parent := plain(filepath.Dir(worktree))
	repo := filepath.Clean(h.Repo)
	parents := []string{hiresFolder(repo, "")}
	if worktrees != "" {
		parents = append(parents, hiresFolder(repo, worktrees))
	}
	return slices.ContainsFunc(parents, func(p string) bool { return plain(p) == parent })
}

// seated is the desks, in the registry's order, each brought in line with it and with what the follow last saw.
func (t *t3Desks) seated(worktrees string, seen func(string) (t3Glance, bool)) []t3Seated {
	var open []Hire
	for _, h := range t.registryHires() {
		if t3HasDesk(h, worktrees) && !slices.ContainsFunc(open, func(o Hire) bool { return o.ID == h.ID }) {
			open = append(open, h)
		}
	}
	t.open = map[string]Hire{}
	for _, h := range open {
		t.open[h.ID] = h
	}
	for id := range t.byHire {
		if _, ok := t.open[id]; !ok {
			delete(t.byHire, id)
		}
	}
	for id := range t.seenDone {
		if _, ok := t.open[id]; !ok {
			delete(t.seenDone, id)
		}
	}
	desks := make([]t3Seated, 0, len(open))
	for _, h := range open {
		s := t.byHire[h.ID]
		if s == nil {
			s = &Session{ID: t3DeskID(h.ID), Agent: Claude, StartedAt: h.Started}
			t.byHire[h.ID] = s
		}
		look, looked := seen(h.ID)
		if !looked || look.Status != "done" {
			delete(t.seenDone, h.ID)
		}
		status := ""
		if looked {
			status = look.Status
		}
		s.Folder, s.Project, s.Model = h.Worktree, h.Worktree, h.Model
		s.State = t3DeskState(status, h.State, t.seenDone[h.ID])
		s.ToolActivity = t3Doing(s.State)
		desks = append(desks, t3Seated{session: s, hire: h})
	}
	return desks
}

// hireOf is the hire at desk id, while it has one (as seated last found it).
func (t *t3Desks) hireOf(id string) (Hire, bool) {
	if !strings.HasPrefix(id, t3DeskPrefix) {
		return Hire{}, false
	}
	h, ok := t.open[strings.TrimPrefix(id, t3DeskPrefix)]
	return h, ok
}

// tapped is a desk tapped: a done hire's news is seen.
func (t *t3Desks) tapped(id string) {
	if h, ok := t.hireOf(id); ok && t.byHire[h.ID] != nil && t.byHire[h.ID].State == Unread {
		t.seenDone[h.ID] = true
	}
}

// t3DeskState is a desk's colour for its thread: at work blue, waiting for an answer orange, done green till seen.
// Not seen yet (as after a restart), it's as the registry says; T3 Code not answering, white.
func t3DeskState(status, registry string, seen bool) SessionState {
	switch status {
	case "working":
		return Thinking
	case "waiting":
		return Asking
	case "done":
		if seen {
			return Idle
		}
		return Unread
	case "unknown":
		return Idle
	}
	if registry == "idle" {
		return Idle
	}
	return Thinking
}

// t3Doing is what its card says it's doing, for its state.
func t3Doing(state SessionState) string {
	switch state {
	case Thinking:
		return "At work in T3 Code"
	case Asking:
		return "Waiting for an answer in T3 Code"
	}
	return ""
}

// MARK: The deck

// t3Seated is the hires in T3 Code at their desks, as the registry says now. With the lock held.
func (d *Deck) t3Seated() []t3Seated {
	return d.t3Desks.seated(d.worktreesFolder(d.engine.worktrees()), d.t3.lastSeen)
}

// t3HireAtDesk is the hire at T3 desk id, as the registry says now: false for any other desk, or one let go
// since. With the lock held.
func (d *Deck) t3HireAtDesk(id string) (Hire, bool) {
	if !strings.HasPrefix(id, t3DeskPrefix) {
		return Hire{}, false
	}
	for _, one := range d.t3Seated() {
		if one.session.ID == id {
			return one.hire, true
		}
	}
	return Hire{}, false
}

// t3FloorOf is the floor a T3 hire's desk is on: 0 for the ground floor, as the registry keeps it.
func t3FloorOf(h Hire) int {
	if h.T3Floor == nil {
		return 0
	}
	return *h.T3Floor
}

// t3Entry is a hire in T3 Code's desk (experimental): its role's look, its name, and its thread's status, with
// every control off, as there's no tab to type into; its card says it runs in T3 Code, with its branch. No path,
// no thread id: those stay on this computer. With the lock held.
func (d *Deck) t3Entry(s *Session, h Hire) map[string]any {
	entry := d.sessionEntry(s)
	for _, control := range t3ControlsOff {
		entry[control] = false
	}
	entry["choice"] = nil
	desk := t3DeskJSON(h, d.t3)
	entry["t3"] = map[string]any{"tag": t3Tag, "experimental": true, "status": desk["status"], "branch": h.Branch, "canOpen": desk["canOpen"]}
	// Taken over from its tmux session (t3takeover.go): Hand back on its card, only then.
	if h.T3Session != "" {
		entry["t3"].(map[string]any)["handBack"] = true
	}
	return entry
}

// isT3HiresOwn is whether a hook event is a hire in T3 Code's own Claude Code: one the deck doesn't know yet, that
// started in (or works in) the worktree the registry gives an open hire in T3 Code. It's never a desk: the hire's
// desk is its registry's. Nor anyone else's: T3 Code's server started from a terminal hands that terminal's pane
// on in its environment, and the event would take that terminal's desk (RM5a §7). A session already at a desk that
// moves into such a worktree stays as it was. So a terminal you open in a T3 hire's worktree has no desk until the
// hire is let go. Only a worktree where hires' are made counts (t3InHiresFolder). With the lock held.
func (d *Deck) isT3HiresOwn(event map[string]any, startedIn string) bool {
	id, _ := event["session_id"].(string)
	if id == "" || d.sessions.Get(id) != nil || d.sessions.WasDropped(id) {
		return false
	}
	var worktrees []string
	for _, h := range d.t3Desks.registryHires() {
		if h.Engine == t3Engine && isOpen(h) && t3InHiresFolder(h, d.worktreesFolder(d.engine.worktrees())) {
			worktrees = append(worktrees, h.Worktree, plainPath(h.Worktree))
		}
	}
	if len(worktrees) == 0 {
		return false
	}
	cwd, _ := event["cwd"].(string)
	for _, place := range []string{startedIn, cwd} {
		if place == "" {
			continue
		}
		for _, p := range []string{place, plainPath(place)} {
			if slices.ContainsFunc(worktrees, func(w string) bool { return inFolder(p, w) }) {
				return true
			}
		}
	}
	return false
}

// keepT3Seats keeps where each hire in T3 Code sits now in the registry, so it sits there again after a restart.
// The registry is written only when one moved. With the lock held.
func (d *Deck) keepT3Seats() {
	seated := d.t3Seated()
	if len(seated) == 0 {
		return
	}
	keys, overflow := d.layout()
	// The building only when a desk is upstairs: on the ground floor, the keys are enough.
	upstairs := map[string]Seat{}
	if slices.ContainsFunc(seated, func(one t3Seated) bool { return t3FloorOf(one.hire) > 0 }) {
		building, _ := d.seatedBuilding(overflow)
		for floor, people := range building {
			for _, p := range people {
				upstairs[p.person.ID] = Seat{floor, p.desk}
			}
		}
	}
	for _, one := range seated {
		// Only a desk on its own floor: one upstairs for want of a key downstairs isn't where it was put.
		floor := t3FloorOf(one.hire)
		if key := slices.IndexFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == one.session.ID }); floor == 0 && key >= 0 {
			setT3Seat(d.t3Desks.registry, one.hire.ID, 0, &key)
		} else if seat, ok := upstairs[one.session.ID]; floor > 0 && ok && seat.Floor == floor {
			desk := seat.Desk
			setT3Seat(d.t3Desks.registry, one.hire.ID, floor, &desk)
		}
	}
}

// setT3Seat is where a hire in T3 Code's desk is now, in the registry at path; saved only when it moved. Says
// whether it did.
func setT3Seat(path, id string, floor int, desk *int) bool {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == id && h.Engine == t3Engine })
	if i < 0 {
		return false
	}
	h := &file.hires[i]
	if h.T3Floor != nil && *h.T3Floor == floor && (h.T3Desk == nil) == (desk == nil) && (desk == nil || *h.T3Desk == *desk) {
		return false
	}
	h.T3Floor, h.T3Desk = &floor, nil
	if desk != nil {
		at := *desk
		h.T3Desk = &at
	}
	return file.save(path) == nil
}

// moveT3 is a hire in T3 Code put on another floor, from its card: kept in the registry, so it stays there; at the
// floor's first free desk. There's no folder rule to remember for it. With the lock held.
func (d *Deck) moveT3(h Hire, id string, floor int, full string) (int, string) {
	if t3FloorOf(h) == floor {
		return http.StatusNoContent, ""
	}
	if floor > 0 && !d.hasRoom(floor, id) {
		return http.StatusConflict, full
	}
	if floor == 0 && !d.hasFreeKey() {
		return http.StatusConflict, "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room."
	}
	setT3Seat(d.t3Desks.registry, h.ID, floor, nil)
	delete(d.seatOf, id)
	delete(d.keyOf, id)
	delete(d.chosenDesk, id)
	d.changed()
	return http.StatusNoContent, ""
}

// letGoAtDesk is /act?action=letgo on a hire in T3 Code's desk, from its card after its confirm: the lane's own
// Let go, with its refusals. False for any other desk.
func (d *Deck) letGoAtDesk(id string) (int, any, bool) {
	d.mu.Lock()
	h, ok := d.t3HireAtDesk(id)
	d.mu.Unlock()
	if !ok {
		return 0, nil, false
	}
	status, answer := d.letGoOfT3(h.ID)
	return status, answer, true
}

// focusT3 is a tap on a hire in T3 Code's desk: its done seen, and its own thread opened in T3 Code, with T3 Code
// on. False for any other desk.
func (d *Deck) focusT3(id string) (int, string, bool) {
	d.mu.Lock()
	h, ok := d.t3HireAtDesk(id)
	if !ok {
		d.mu.Unlock()
		return 0, "", false
	}
	d.t3Desks.tapped(id)
	d.changed()
	on := t3CodeAllowed(d.config)
	d.mu.Unlock()
	// Its done is seen; a test copy only leaves T3 Code closed (RTCH N1, RWTC N9).
	if why := testCopyKeepsAway(testCopyKeepsT3Code); why != "" {
		return http.StatusConflict, why, true
	}
	if !on {
		return http.StatusConflict, t3Off, true
	}
	status, answer := t3OpenOnce(d.t3Latest(h), newT3Machine())
	if status < 300 {
		return http.StatusNoContent, "", true
	}
	text, _ := answer.(string)
	if text == "" {
		data, _ := json.Marshal(answer)
		text = string(data)
	}
	return status, text, true
}
