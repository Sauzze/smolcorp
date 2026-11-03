package desktop

import (
	"encoding/json"
	"fmt"
	"io"
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
}

// seatedPerson is someone seated above the ground floor, with their desk.
type seatedPerson[Person any] struct {
	person Person
	desk   int
}

// seatBuilding is the building above the ground floor, mostUpstairs desks a
// floor. Those put on a floor sit there first, at the desk they had if it's
// still free, else at its first free one; a floor that's full sends the rest
// to be seated with everyone else. Then everyone else, in order: at the desk
// they had if it's free, else at the first free desk from 2F up. So nobody
// moves when someone else comes or goes. Each floor's people in the order of
// their desks; how many there's no desk for; and the desk each one has now.
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
		return seat != nil && seat.Floor >= 1 && seat.Floor < floorCount && seat.Desk >= 0 && seat.Desk < mostUpstairs
	}
	onAFloor := func(p upstairsPerson[Person]) bool { return p.floor >= 1 && p.floor < floorCount }
	seated := func(i int) bool { _, ok := seats[everyone[i].id]; return ok }
	// First everyone at the desk they had, so nobody moves for someone new: those put on a floor at theirs there,
	// then everyone else at theirs, wherever it is.
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
	// Those put on a floor and new there: at a free desk on it; on a floor that's full, at the desk of someone there
	// only for want of room elsewhere, who goes to the floors with room. With neither, where there's room.
	var unseated []int
	for i, p := range placed {
		if seated(i) {
			continue
		}
		if !onAFloor(p) {
			unseated = append(unseated, i)
			continue
		}
		spot, other := -1, -1
		for desk := 0; desk < mostUpstairs && spot < 0; desk++ {
			if free(Seat{p.floor, desk}) {
				spot = desk
			}
		}
		for desk := 0; desk < mostUpstairs && spot < 0; desk++ {
			if at := taken[Seat{p.floor, desk}]; at >= len(placed) {
				spot, other = desk, at
			}
		}
		if spot < 0 {
			unseated = append(unseated, i)
			continue
		}
		if other >= 0 {
			delete(seats, everyone[other].id) // seated again below, with everyone not seated yet
		}
		sit(i, Seat{p.floor, spot})
	}
	// Then everyone else not seated yet, and those whose floor was full: at the first free desk from 2F up.
	left := unseated
	for i := range rest {
		if !seated(len(placed) + i) {
			left = append(left, len(placed)+i)
		}
	}
	overflow := 0
	for _, i := range left {
		seated := false
		for floor := 1; floor < floorCount && !seated; floor++ {
			for desk := 0; desk < mostUpstairs && !seated; desk++ {
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
		for desk := 0; desk < mostUpstairs; desk++ {
			if i, ok := taken[Seat{floor, desk}]; ok {
				floors[floor] = append(floors[floor], seatedPerson[Person]{everyone[i].person, desk})
			}
		}
	}
	return floors, overflow, seats
}

// floorRulesBody reads a request's body once, for both the floors and their rules.
func floorRulesBody(body io.Reader) []byte {
	data, _ := io.ReadAll(io.LimitReader(body, 1<<20))
	return data
}

// move puts session id on floor given, 0 for the ground floor up to 5 for
// 6F, from the office: until it ends, before its folder's rule; and with
// remember, its folder's rule too, so its next sessions go there. A cloud
// bot's floor is kept in setup's list, as it never ends. Refused when there's
// no desk for it there: the ground floor with every key taken, or a floor
// above with all its desks.
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
		if floor == 0 && !d.hasFreeKey() {
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
		delete(d.seatOf, id)
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
// want of room elsewhere make way, to the floors with room. With the lock held.
func (d *Deck) hasRoom(floor int, id string) bool {
	put := 0
	for _, s := range d.sessions.All() {
		if f, ok := d.aboveGround(s); ok && f == floor && s.ID != id {
			put++
		}
	}
	bots, floors := d.cloud.seatedAbove()
	for i, s := range bots {
		if floors[i] == floor && s.ID != id {
			put++
		}
	}
	return put < mostUpstairs
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
