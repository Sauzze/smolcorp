package desktop

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Seat is a desk above the ground floor: its floor, 1 for 2F up to 5 for 6F,
// and which of that floor's desks it is.
type Seat struct {
	Floor int `json:"floor"`
	Desk  int `json:"desk"`
}

// FloorRule is a folder whose sessions sit on a floor of their own: "Atlas
// projects go to the Atlas floor". Floor is 0 for the ground floor up to 5
// for 6F.
type FloorRule struct {
	Folder string `json:"folder"`
	Floor  int    `json:"floor"`
	// The folder alone, not those in it: as "Always put … here" remembers it
	// from the office, where a folder above a project's would otherwise take
	// all of them up. A rule added in setup holds the folders in it too.
	Exact bool `json:"exact,omitempty"`
}

const mostFloorRules = 64

// floorRuleList is the rules as config.json keeps them: one that can't be
// read loses only itself.
type floorRuleList []FloorRule

func (l *floorRuleList) UnmarshalJSON(data []byte) error {
	var list []json.RawMessage
	json.Unmarshal(data, &list)
	rules := floorRuleList{}
	for _, raw := range list {
		var rule FloorRule
		if json.Unmarshal(raw, &rule) == nil {
			rules = append(rules, rule)
		}
	}
	*l = rules
	return nil
}

// checkedFloorRules is the rules as they're kept: each folder a path, with no
// slash at its end, on a floor there is; a folder's rule for itself alone,
// and its rule for the folders in it too, one of each, the last given winning.
func checkedFloorRules(given []FloorRule) floorRuleList {
	rules := floorRuleList{}
	for _, rule := range given {
		folder, ok := ruleFolder(rule.Folder)
		if !ok || rule.Floor < 0 || rule.Floor >= floorCount {
			continue
		}
		// One of each kind a folder, however it's written: "Always put … here" for itself alone beside setup's for the
		// folders in it.
		rules = slices.DeleteFunc(rules, func(r FloorRule) bool { return sameFolder(r.Folder, folder) && r.Exact == rule.Exact })
		rules = append(rules, FloorRule{Folder: folder, Floor: rule.Floor, Exact: rule.Exact})
	}
	if len(rules) > mostFloorRules {
		rules = rules[len(rules)-mostFloorRules:]
	}
	return rules
}

// floorRulesSent is the rules POST /setup/floors sends, as {"rules": [...]},
// checked; false when there's no list of them, as from a setup page from
// before rules.
func floorRulesSent(body []byte) (floorRuleList, bool) {
	var sent struct {
		Rules *floorRuleList `json:"rules"`
	}
	if json.Unmarshal(body, &sent) != nil || sent.Rules == nil {
		return nil, false
	}
	return checkedFloorRules(*sent.Rules), true
}

// ruleFolder is a rule's folder as it's kept: a whole path, a Mac's or
// Linux's (/…), Windows' (C:\…, \\server\…) or under your home (~/…, ~\…),
// with no separator at its end; false for one that isn't. The same on every
// computer, as a config can go from one to another.
func ruleFolder(given string) (string, bool) {
	folder := strings.TrimSpace(given)
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, `~\`) {
		folder = homeDir + folder[1:]
	}
	windows := len(folder) >= 3 && folder[1] == ':' && (folder[2] == '\\' || folder[2] == '/') &&
		(folder[0] >= 'a' && folder[0] <= 'z' || folder[0] >= 'A' && folder[0] <= 'Z')
	if !strings.HasPrefix(folder, "/") && !strings.HasPrefix(folder, `\\`) && !windows {
		return "", false
	}
	for len(folder) > 1 && strings.ContainsAny(folder[len(folder)-1:], `/\`) && !(windows && len(folder) == 3) {
		folder = folder[:len(folder)-1]
	}
	return folder, true
}

// inFolder is whether project is folder or inside it, whichever way its
// separators lean; a Windows path's case aside, as Windows has it.
func inFolder(project, folder string) bool {
	if project == "" || folder == "" {
		return false
	}
	p, f := normalFolder(project), normalFolder(folder)
	return p == f || strings.HasPrefix(p, f+"/")
}

// sameFolder is whether project is folder itself, as inFolder compares them.
func sameFolder(project, folder string) bool {
	return project != "" && folder != "" && normalFolder(project) == normalFolder(folder)
}

// normalFolder is a path with its separators leaning one way and none at its
// end, in lower case if it's Windows', as Windows has it.
func normalFolder(path string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	if len(path) >= 2 && path[1] == ':' || strings.HasPrefix(path, "//") {
		path = strings.ToLower(path)
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// ruleFloor is the floor project goes to by its rules: the rule for the
// folder it's in, or the closest above it; false when no rule says.
func ruleFloor(project string, rules []FloorRule) (int, bool) {
	// The closest folder; on that folder itself, its own rule before the one for what's in it; then the later.
	best, exact, floor := -1, false, 0
	for _, rule := range rules {
		matches := inFolder(project, rule.Folder)
		if rule.Exact {
			matches = sameFolder(project, rule.Folder)
		}
		length := len(normalFolder(rule.Folder))
		if matches && (length > best || length == best && (rule.Exact || !exact)) {
			best, exact, floor = length, rule.Exact, rule.Floor
		}
	}
	return floor, best >= 0
}

// floorOfProject is the floor a session sits on as things stand: 0 for the
// ground floor. Where it was put, if it was (put >= 0); else where its
// folder's rule says; else false, for the ground floor's keys, then the
// floors above as they fill.
func floorOfProject(project string, put int, rules []FloorRule) (int, bool) {
	if put >= 0 && put < floorCount {
		return put, true
	}
	return ruleFloor(project, rules)
}

// upstairsPerson is someone to seat above the ground floor: id to know them
// by, the floor they were put on, if they were (1 to 5, else -1), and the
// desk they had, to keep it.
type upstairsPerson[Person any] struct {
	person Person
	id     string
	floor  int
	had    *Seat
	// On its floor because nobody put it there, as a cloud bot that goes where
	// its folder's rule or new bots go: on that floor full, it takes nobody's
	// desk, and sits where there's room, keeping that desk as others come and
	// go till one is free on its own floor. There only for want of room, it
	// makes way for anyone put on that floor (see seatBuilding).
	yields bool
}

// seatedPerson is someone seated above the ground floor, with their desk.
type seatedPerson[Person any] struct {
	person Person
	desk   int
}

// seatBuilding is the building above the ground floor, desksPerFloor desks a
// floor. Those put on a floor sit there first, at the desk they had if it's
// still free, else at its first free one; a floor that's full sends the rest
// to be seated with everyone else, past anyone there only for want of room
// elsewhere unless they yield. One who yields, sent on from its floor, keeps
// the desk it has on another as anyone not put on a floor does, making way for
// those put there, till its own has a desk for it. Then everyone else, in
// order: at the desk they had if it's free, else at the first free desk from
// 2F up. So nobody moves when someone else comes or goes. Each floor's people
// in the order of their desks; how many there's no desk for; and the desk each
// one has now.
func seatBuilding[Person any](placed, rest []upstairsPerson[Person]) ([][]seatedPerson[Person], int, map[string]Seat) {
	everyone := slices.Concat(placed, rest)
	taken := map[Seat]int{}
	seats := map[string]Seat{}
	free := func(seat Seat) bool { _, ok := taken[seat]; return !ok }
	sit := func(index int, seat Seat) {
		taken[seat] = index
		seats[everyone[index].id] = seat
	}
	valid := func(seat *Seat) bool {
		return seat != nil && seat.Floor >= 1 && seat.Floor < floorCount && seat.Desk >= 0 && seat.Desk < desksPerFloor
	}
	onAFloor := func(p upstairsPerson[Person]) bool { return p.floor >= 1 && p.floor < floorCount }
	seated := func(i int) bool { _, ok := seats[everyone[i].id]; return ok }
	// First everyone at the desk they had, so nobody moves for someone new: those put on a floor at theirs there,
	// then everyone else at theirs, wherever it is; then those who yield at theirs on a floor not their own, where
	// theirs had no room: after the rest, so one back at a desk it had takes no one's.
	for i, p := range placed {
		if onAFloor(p) && valid(p.had) && p.had.Floor == p.floor && free(*p.had) {
			sit(i, *p.had)
		}
	}
	for i := range rest {
		if had := rest[i].had; valid(had) && free(*had) {
			sit(len(placed)+i, *had)
		}
	}
	for i, p := range placed {
		if p.yields && onAFloor(p) && !seated(i) && valid(p.had) && free(*p.had) {
			sit(i, *p.had)
		}
	}
	// On floor only for want of room on its own: one who yields; and anyone not put on a floor, there for want of
	// room anywhere.
	away := func(i, floor int) bool { return i < len(placed) && everyone[i].yields && everyone[i].floor != floor }
	makesWay := func(i, floor int) bool { return i >= len(placed) || away(i, floor) }
	// A desk kept for someone put on another floor and sent on from it, who sits again at the desk they had
	// below: one who yields takes nobody's desk, theirs included.
	keptFor := func(seat Seat) bool {
		for j, q := range placed {
			if !q.yields && !seated(j) && valid(q.had) && *q.had == seat {
				return true
			}
		}
		return false
	}
	// The first free desk on floor (for one who yields, not a desk kept for someone), else, with gives, the first
	// whose one there gives it up; -1 for neither.
	deskOn := func(floor int, yields bool, gives func(i, floor int) bool) int {
		for desk := 0; desk < desksPerFloor; desk++ {
			if seat := (Seat{floor, desk}); free(seat) && !(yields && keptFor(seat)) {
				return desk
			}
		}
		for desk := 0; desk < desksPerFloor && gives != nil; desk++ {
			if at, ok := taken[Seat{floor, desk}]; ok && gives(at, floor) {
				return desk
			}
		}
		return -1
	}
	// Those put on a floor and new there: at a free desk on it; on a floor that's full, at the desk of someone there
	// only for want of room elsewhere, who goes to the floors with room, unless nobody put them there (yields).
	// With neither, where there's room.
	var unseated []int
	for i, p := range placed {
		if seated(i) || slices.Contains(unseated, i) {
			continue
		}
		if !onAFloor(p) {
			unseated = append(unseated, i)
			continue
		}
		gives := makesWay
		if p.yields {
			gives = nil // it takes nobody's desk
		}
		spot := deskOn(p.floor, p.yields, gives)
		if spot < 0 {
			unseated = append(unseated, i)
			continue
		}
		if other, ok := taken[Seat{p.floor, spot}]; ok {
			delete(seats, everyone[other].id) // seated again below, with everyone not seated yet
			if other < len(placed) {
				unseated = append(unseated, other)
			}
		}
		sit(i, Seat{p.floor, spot})
	}
	// Then those who yield and aren't on their floor, new or sent on from it: at a free desk there; or, two each on
	// the other's, at each other's desks. Again till nobody moves, as one going home can free a desk for another.
	for moved := true; moved; {
		moved = false
		for i, p := range placed {
			was, sat := seats[p.id]
			if !p.yields || !onAFloor(p) || sat && was.Floor == p.floor {
				continue
			}
			// One that yields too, on this floor, whose own is where p sits.
			homeFromHere := func(j, floor int) bool { return sat && away(j, floor) && everyone[j].floor == was.Floor }
			spot := deskOn(p.floor, true, homeFromHere)
			if spot < 0 {
				continue
			}
			other, swap := taken[Seat{p.floor, spot}]
			if sat {
				delete(taken, was)
			}
			sit(i, Seat{p.floor, spot})
			if swap {
				sit(other, was)
			}
			moved = true
		}
	}
	// Then everyone else not seated yet, and those whose floor was full: at the desk they had if it's still free
	// (so nobody sent on from a full floor shifts when a bot or anyone else arrives), else at the first free desk from 2F up.
	left := slices.DeleteFunc(unseated, seated)
	for i := range rest {
		if !seated(len(placed) + i) {
			left = append(left, len(placed)+i)
		}
	}
	for _, i := range left {
		if had := everyone[i].had; valid(had) && free(*had) {
			sit(i, *had)
		}
	}
	left = slices.DeleteFunc(left, seated)
	overflow := 0
	for _, i := range left {
		seated := false
		for floor := 1; floor < floorCount && !seated; floor++ {
			for desk := 0; desk < desksPerFloor && !seated; desk++ {
				if seat := (Seat{floor, desk}); free(seat) {
					sit(i, seat)
					seated = true
				}
			}
		}
		if !seated {
			overflow++
		}
	}
	floors := make([][]seatedPerson[Person], floorCount)
	for floor := range floors {
		floors[floor] = []seatedPerson[Person]{}
		for desk := 0; desk < desksPerFloor; desk++ {
			if i, ok := taken[Seat{floor, desk}]; ok {
				floors[floor] = append(floors[floor], seatedPerson[Person]{everyone[i].person, desk})
			}
		}
	}
	return floors, overflow, seats
}

// onFloorDesks is the desks above the ground floor kept across a restart, on
// floors of desksPerFloor desks: anyone at a desk past the last (desks 10 to 12
// of a floor, from when every floor had twelve) moves to the first free desk
// on the same floor, or else on the next floor up with one, or else on the
// nearest floor below with one. One put on their floor is put on the floor
// they move to. Their role, nickname and folder's rules go with them, being
// kept by session and folder rather than by desk. With no free desk anywhere,
// their desk is let go: they sit where there's room, as anyone new does, or
// wait in the elevator's list. Nobody disappears. False when nobody needs to
// move, so seats already moved stay as they are. The Mac app's
// Deck.onFloorDesks does the same.
func onFloorDesks(seats map[string]Seat, floors map[string]int) (map[string]Seat, map[string]int, bool) {
	upstairs := func(floor int) bool { return floor >= 1 && floor < floorCount }
	var movers []string
	taken := map[Seat]bool{}
	for id, seat := range seats {
		if upstairs(seat.Floor) && seat.Desk >= desksPerFloor {
			movers = append(movers, id)
		} else if upstairs(seat.Floor) && seat.Desk >= 0 {
			taken[seat] = true
		}
	}
	if len(movers) == 0 {
		return seats, floors, false
	}
	slices.SortFunc(movers, func(a, b string) int {
		if c := cmp.Compare(seats[a].Floor, seats[b].Floor); c != 0 {
			return c
		}
		if c := cmp.Compare(seats[a].Desk, seats[b].Desk); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	// The first free desk on floor, or -1 when it's full
	freeOn := func(floor int) int {
		for desk := 0; desk < desksPerFloor; desk++ {
			if !taken[Seat{floor, desk}] {
				return desk
			}
		}
		return -1
	}
	moved, put := maps.Clone(seats), maps.Clone(floors)
	if put == nil {
		put = map[string]int{}
	}
	for _, id := range movers {
		had := seats[id]
		delete(moved, id)
		// Its own floor, then those above it, then those below, nearest first
		order := []int{had.Floor}
		for floor := had.Floor + 1; floor < floorCount; floor++ {
			order = append(order, floor)
		}
		for floor := had.Floor - 1; floor >= 1; floor-- {
			order = append(order, floor)
		}
		for _, floor := range order {
			desk := freeOn(floor)
			if desk < 0 {
				continue
			}
			taken[Seat{floor, desk}] = true
			moved[id] = Seat{floor, desk}
			if was, ok := put[id]; ok && was == had.Floor && floor != had.Floor {
				put[id] = floor
			}
			break
		}
	}
	return moved, put, true
}

// floorRulesBody reads a request's body once, for both the floors and their rules.
func floorRulesBody(body io.Reader) []byte {
	data, _ := io.ReadAll(io.LimitReader(body, 1<<20))
	return data
}

// move puts session id on floor given, 0 for the ground floor up to 5 for
// 6F, from the office: until it ends, before its folder's rule; and with
// remember, its folder's rule too, so its next sessions go there. A cloud
// bot's floor is kept in setup's list, as it never ends: put there, it stays,
// whatever its rule or new bots' floor says. Refused when there's no desk for
// it there: the ground floor with every key taken, or a floor above with all
// its desks.
func (d *Deck) move(id, given string, remember bool) (int, string) {
	floor, err := strconv.Atoi(given)
	if err != nil || floor < 0 || floor >= floorCount {
		return http.StatusBadRequest, "Unknown floor"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	name := checkedFloors(d.config.Floors)[floor].Name
	full := fmt.Sprintf("%s is full.", name)
	if bot, ok := d.cloud.botOf(id); ok {
		if bot.floorIndex() == floor {
			return http.StatusNoContent, ""
		}
		if floor > 0 && !d.hasRoom(floor, id) {
			return http.StatusConflict, full
		}
		// At a key already, as where new bots go: it keeps that one.
		if floor == 0 && !d.hasFreeKey() && !d.atAKey(id) {
			return http.StatusConflict, "Every desk is taken. Fire someone, or add a desk, first."
		}
		bots := slices.Clone(d.config.CloudBots)
		for i := range bots {
			if bots[i].ID == bot.ID {
				bots[i].Floor = cloudFloorAt(floor)
			}
		}
		d.config.CloudBots = bots
		d.config.save()
		if seat, ok := d.seatOf[id]; ok && seat.Floor != floor {
			delete(d.seatOf, id)
		}
		d.cloud.configure(d.config.CloudBots)
		d.changed()
		return http.StatusNoContent, ""
	}
	s := d.sessions.Get(id)
	if s == nil {
		return http.StatusNotFound, "Session not found"
	}
	if now, ok := d.aboveGround(s); ok && now == floor && !remember {
		return http.StatusNoContent, ""
	}
	before, had := d.floorOf[id]
	if floor > 0 && !d.hasRoom(floor, id) {
		return http.StatusConflict, full
	}
	if floor == 0 && !s.Headless() {
		// Down on the ground floor, it needs a key: tried as it would sit, and put back if there's none.
		d.floorOf[id] = 0
		keys, _ := d.layout()
		if !slices.ContainsFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == id }) {
			if had {
				d.floorOf[id] = before
			} else {
				delete(d.floorOf, id)
			}
			return http.StatusConflict, "Every desk is taken. Fire someone, or add a desk, first."
		}
	}
	d.floorOf[id] = floor
	if seat, ok := d.seatOf[id]; ok && seat.Floor != floor {
		delete(d.seatOf, id)
	}
	if remember && s.Project != "" {
		// That folder alone: from a session in a folder above others, a rule for all of them would take them up too.
		d.config.FloorRules = checkedFloorRules(append(slices.Clone(d.config.FloorRules), FloorRule{s.Project, floor, true}))
		d.config.save()
	}
	d.changed()
	return http.StatusNoContent, ""
}

// hasFreeKey is whether the ground floor has a key for someone new: a free
// one, or with Auto, room for one more. With the lock held.
func (d *Deck) hasFreeKey() bool {
	keys, _ := d.layout()
	free := slices.ContainsFunc(keys, func(k deckKey) bool { return k.pin == nil && k.session == nil })
	return free || d.config.Layout.Keys == Auto && len(keys) < mostKeys
}

// hasRoom is whether floor, above the ground floor, has a desk for id: fewer
// than its desks are taken by others put there. Those seated there only for
// want of room elsewhere make way, to the floors with room. A bot nobody put
// on its floor counts where it sits: there, on its floor; sent on to another
// for want of room, on neither, as it makes way there and takes a desk on its
// own only once those put there have theirs (see seatBuilding). With the lock
// held.
func (d *Deck) hasRoom(floor int, id string) bool {
	put := 0
	for _, s := range d.sessions.All() {
		if f, ok := d.aboveGround(s); ok && f == floor && s.ID != id {
			put++
		}
	}
	bots, sitting := d.cloud.seated()
	for i, s := range bots {
		seat, ok := d.seatOf[s.ID]
		elsewhere := sitting[i].floorIndex() < 0 && ok && seat.Floor != floor
		if d.cloudFloor(sitting[i]) == floor && s.ID != id && !elsewhere {
			put++
		}
	}
	return put < desksPerFloor
}

// atAKey is whether session id has a key on the ground floor now. With the
// lock held.
func (d *Deck) atAKey(id string) bool {
	keys, _ := d.layout()
	return slices.ContainsFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == id })
}

// floorFolder is the folder an upstairs hire on floor starts in: the floor's
// one rule's, or "" when it has none or more than one.
func floorFolder(rules []FloorRule, floor int) string {
	folder := ""
	for _, rule := range rules {
		if rule.Floor == floor {
			if folder != "" && !sameFolder(folder, rule.Folder) {
				return ""
			}
			folder = rule.Folder
		}
	}
	return folder
}
