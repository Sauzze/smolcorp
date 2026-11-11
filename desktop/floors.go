package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
)

// Floor is a floor of the building: the name its sign says, and its style.
// The ground floor is the office itself, with its desks and keys; the five
// above hold those who have no desk there (see buildingFloors).
type Floor struct {
	Name string `json:"name"`
	// Preset is "" for the same style as the office downstairs, "server" for
	// the server room floor 2 always had, or one of the office's styles.
	Preset string `json:"preset"`
}

const (
	floorCount = 6
	// desksPerFloor is as many desks as each floor has: three rows of three, as
	// the office draws them on every floor. The one number every floor's room
	// is counted from: seating, the moves between floors, a hire upstairs, the
	// bots' room. The Mac app's Floor.desks is the same.
	desksPerFloor  = 9
	floorNameLimit = 24
	serverFloor    = "server"
)

// floorName is a floor's own name until you give it one: 1F for the ground
// floor, 2F for the one above, and so on.
func floorName(index int) string { return fmt.Sprintf("%dF", index+1) }

// floorList is the floors as config.json keeps them: a floor that can't be
// read, or a name or style that isn't text, loses only itself, in its place.
type floorList []Floor

func (l *floorList) UnmarshalJSON(data []byte) error {
	var list []json.RawMessage
	json.Unmarshal(data, &list)
	floors := floorList{}
	for _, raw := range list {
		var fields map[string]json.RawMessage
		json.Unmarshal(raw, &fields)
		var floor Floor
		json.Unmarshal(fields["name"], &floor.Name)
		json.Unmarshal(fields["preset"], &floor.Preset)
		floors = append(floors, floor)
	}
	*l = floors
	return nil
}

// checkedFloors is exactly six floors, as they're kept: each name one line of
// 24 characters at most, or its default when empty; each style one there is,
// or the office's own. The ground floor's look is always the office's own,
// and Yours, the office as you furnished it, is the office's own too. Missing
// floors are the defaults; any past the sixth are left out.
func checkedFloors(given []Floor) floorList {
	floors := make(floorList, floorCount)
	for i := range floors {
		var floor Floor
		if i < len(given) {
			floor = given[i]
		}
		floor.Name = oneLine(floor.Name, floorNameLimit)
		if floor.Name == "" {
			floor.Name = floorName(i)
		}
		if i == 0 || floor.Preset == "yours" || floor.Preset != serverFloor && !slices.Contains(roomStyles, floor.Preset) {
			floor.Preset = ""
		}
		floors[i] = floor
	}
	return floors
}

// floorsSent is the floors POST /setup/floors sends, as {"floors": [...]} or
// the list alone, checked; false unless it's a list.
func floorsSent(body io.Reader) (floorList, bool) {
	var sent any
	if json.NewDecoder(body).Decode(&sent) != nil {
		return nil, false
	}
	if object, ok := sent.(map[string]any); ok {
		sent = object["floors"]
	}
	list, ok := sent.([]any)
	if !ok {
		return nil, false
	}
	data, _ := json.Marshal(list)
	var floors floorList
	json.Unmarshal(data, &floors)
	return checkedFloors(floors), true
}

// floorStyles are the office's styles a floor above the ground floor can
// have, besides the office's own ("") and the server room ("server").
func floorStyles() []string {
	return slices.DeleteFunc(slices.Clone(roomStyles), func(style string) bool { return style == "yours" })
}

// buildingFloors is the building's floors, the ground floor first and always
// empty here, as its desks are the keys; then 2F to 6F, desksPerFloor each,
// filled in order: cloud bots first, so a permanent one keeps its desk as
// runs come and go, then headless runs, oldest first, then the agents with no
// desk downstairs. And how many there's no floor for. None of them has a key.
func buildingFloors[Person any](cloud, headless, overflow []Person) ([][]Person, int) {
	var rest []upstairsPerson[Person]
	for i, p := range slices.Concat(cloud, headless, overflow) {
		rest = append(rest, upstairsPerson[Person]{person: p, id: strconv.Itoa(i), floor: -1})
	}
	seated, over, _ := seatBuilding(nil, rest)
	return peopleOf(seated), over
}

// peopleOf is each floor's people, without their desks.
func peopleOf[Person any](seated [][]seatedPerson[Person]) [][]Person {
	floors := make([][]Person, len(seated))
	for i, floor := range seated {
		floors[i] = []Person{}
		for _, one := range floor {
			floors[i] = append(floors[i], one.person)
		}
	}
	return floors
}

// secondFloor is the second floor alone, as phones from before the other
// floors see it: its desks, and how many sit above it or have no floor.
func secondFloor[Person any](floors [][]Person, over int) ([]Person, int) {
	for _, floor := range floors[2:] {
		over += len(floor)
	}
	return floors[1], over
}

// aboveGround is the floor s was put on, by the office, by its folder's
// rule, or, for a cloud bot, as cloudFloor says: 0 for the ground floor, 1
// to 5 above it. False for one that sits where there's room. With the lock
// held.
func (d *Deck) aboveGround(s *Session) (int, bool) {
	if bot, ok := d.cloud.botOf(s.ID); ok {
		return d.cloudFloor(bot), true
	}
	if hire, ok := d.t3Desks.hireOf(s.ID); ok {
		return t3FloorOf(hire), true
	}
	put, ok := d.floorOf[s.ID]
	if !ok {
		put = -1
	}
	return floorOfProject(s.Project, put, d.config.FloorRules)
}

// cloudFloor is the floor cloud bot bot sits on: the one it was put on, else
// its folder's rule's, else where new bots go. A floor that's full sends it
// on to one with room: one you put there as anyone put there, past those
// there only for want of room; any other taking nobody's desk (see
// seatBuilding). With the lock held.
func (d *Deck) cloudFloor(bot CloudBot) int {
	return bot.floorOn(d.config.FloorRules, d.config.BotFloor)
}

// building is who sits above the ground floor, floor by floor (see
// seatBuilding): first those put on a floor, the cloud bots first among
// them, then, where there's room, the headless runs, then overflow, the
// sessions the ground floor had no desk for. And the desk each has on its
// floor. With the lock held.
func (d *Deck) building(overflow []*Session) ([][]*Session, int) {
	seated, over := d.seatedBuilding(overflow)
	return peopleOf(seated), over
}

// seatedBuilding is building, with each one's desk. With the lock held.
func (d *Deck) seatedBuilding(overflow []*Session) ([][]seatedPerson[*Session], int) {
	all := d.sessions.All()
	headless := slices.DeleteFunc(slices.Clone(all), func(s *Session) bool { return !s.Headless() })
	up := func(s *Session, floor int) upstairsPerson[*Session] {
		p := upstairsPerson[*Session]{person: s, id: s.ID, floor: floor}
		if seat, ok := d.seatOf[s.ID]; ok {
			p.had = &seat
		}
		return p
	}
	var placed, rest []upstairsPerson[*Session]
	isPlaced := map[string]bool{}
	put := func(s *Session, yields bool) {
		if floor, ok := d.aboveGround(s); ok && floor > 0 && !isPlaced[s.ID] {
			p := up(s, floor)
			p.yields = yields
			placed = append(placed, p)
			isPlaced[s.ID] = true
		}
	}
	// Every bot has a floor: the ground floor's are at keys, or in overflow with none free. One nobody put on its
	// floor takes nobody's desk there (see upstairsPerson.yields).
	bots, of := d.cloud.seated()
	for i, s := range bots {
		put(s, of[i].floorIndex() < 0)
	}
	// A hire in T3 Code upstairs is on the floor it was hired onto, at the desk the registry kept for it.
	for _, one := range d.t3Seated() {
		if floor := t3FloorOf(one.hire); floor > 0 && !isPlaced[one.session.ID] {
			p := up(one.session, floor)
			if p.had == nil && one.hire.T3Desk != nil {
				p.had = &Seat{floor, *one.hire.T3Desk}
			}
			placed = append(placed, p)
			isPlaced[one.session.ID] = true
		}
	}
	for _, s := range headless {
		put(s, false)
	}
	// Sessions put upstairs are left out of the keys (see layout), so they're found here.
	for _, s := range all {
		if !s.Headless() {
			put(s, false)
		}
	}
	for _, s := range slices.Concat(headless, overflow) {
		if !isPlaced[s.ID] {
			rest = append(rest, up(s, -1))
		}
	}
	seated, over, seats := seatBuilding(placed, rest)
	for id, seat := range seats {
		d.seatOf[id] = seat
	}
	return seated, over
}

// botsWithNoDesk is the names of the bots at work with no desk on any floor,
// every one taken, for setup to say so. With the lock held.
func (d *Deck) botsWithNoDesk() []string {
	keys, overflow := d.layout()
	seated, _ := d.seatedBuilding(overflow)
	sitting := map[string]bool{}
	for _, key := range keys {
		if key.session != nil {
			sitting[key.session.ID] = true
		}
	}
	for _, floor := range seated {
		for _, one := range floor {
			sitting[one.person.ID] = true
		}
	}
	names := []string{}
	bots, of := d.cloud.seated()
	for i, s := range bots {
		if !sitting[s.ID] {
			names = append(names, of[i].Name)
		}
	}
	return names
}

// upperEntry is a desk above the ground floor, as the phone is told of it: a
// cloud bot's or a headless run's, watched, as there's nothing to type into;
// or an agent's with no key, put there or with no desk left downstairs, which
// is driven there as on the ground floor. With the lock held.
func (d *Deck) upperEntry(s *Session) map[string]any {
	if bot, ok := d.cloud.botOf(s.ID); ok {
		return d.cloudEntry(s, bot)
	}
	// Picked by a tap, upstairs as downstairs, so its thread opens: there's nothing to drive.
	if hire, ok := d.t3Desks.hireOf(s.ID); ok {
		entry := d.t3Entry(s, hire)
		entry["headless"], entry["drivable"] = false, true
		return entry
	}
	entry := d.sessionEntry(s)
	if !s.Headless() {
		floor, put := d.aboveGround(s)
		entry["headless"], entry["drivable"] = false, true
		// Up there only for want of a desk downstairs, rather than put there.
		entry["overflow"] = !put || floor == 0
		return entry
	}
	entry["headless"] = true
	for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary", "phoneMic"} {
		entry[control] = false
	}
	entry["role"] = nil
	entry["hiredBy"] = d.hiredBy(s)
	return entry
}

// floorsState is the six floors as the phone is told of them, the ground
// floor's desks being the keys; how many there's no floor for; and the
// second floor alone, for phones from before the other floors. With the lock
// held.
func (d *Deck) floorsState(overflow []*Session) ([]map[string]any, int, map[string]any) {
	seated, over := d.seatedBuilding(overflow)
	floors := peopleOf(seated)
	deskOf := map[string]int{}
	for _, floor := range seated {
		for _, one := range floor {
			deskOf[one.person.ID] = one.desk
		}
	}
	entries := func(people []*Session) []map[string]any {
		desks := []map[string]any{}
		for _, s := range people {
			entry := d.upperEntry(s)
			// The desk it has on its floor, so it stays there as others come and go.
			entry["desk"] = deskOf[s.ID]
			desks = append(desks, entry)
		}
		return desks
	}
	asking := func(people []*Session) int {
		count := 0
		for _, s := range people {
			if _, cloud := d.cloud.botOf(s.ID); s.State == Asking && !s.Headless() && !cloud {
				count++
			}
		}
		return count
	}
	shown := make([]map[string]any, floorCount)
	for i, floor := range checkedFloors(d.config.Floors) {
		// How many there wait on you, for the elevator's list to say so.
		shown[i] = map[string]any{"n": i + 1, "name": floor.Name, "preset": floor.Preset, "desks": entries(floors[i]),
			"asking": asking(floors[i])}
	}
	second, past := secondFloor(floors, over)
	return shown, over, map[string]any{"desks": entries(second), "overflow": past}
}
