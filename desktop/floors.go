package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
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
	floorCount     = 6
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
// empty here, as its desks are the keys; then 2F to 6F, mostUpstairs each,
// filled in order: cloud bots first, so a permanent one keeps its desk as
// runs come and go, then headless runs, oldest first, then the agents with no
// desk downstairs. And how many there's no floor for. None of them has a key.
func buildingFloors[Person any](cloud, headless, overflow []Person) ([][]Person, int) {
	people := slices.Concat(cloud, headless, overflow)
	floors := make([][]Person, floorCount)
	for i := range floors {
		floors[i] = []Person{}
		if start := (i - 1) * mostUpstairs; i > 0 && start < len(people) {
			floors[i] = people[start:min(start+mostUpstairs, len(people))]
		}
	}
	return floors, max(0, len(people)-(floorCount-1)*mostUpstairs)
}

// secondFloor is the second floor alone, as phones from before the other
// floors see it: its desks, and how many sit above it or have no floor.
func secondFloor[Person any](floors [][]Person, over int) ([]Person, int) {
	for _, floor := range floors[2:] {
		over += len(floor)
	}
	return floors[1], over
}

// building is who sits above the ground floor, floor by floor (see
// buildingFloors): the cloud bots seated upstairs, the headless runs, then
// overflow, the sessions the ground floor had no desk for. With the lock held.
func (d *Deck) building(overflow []*Session) ([][]*Session, int) {
	headless := slices.DeleteFunc(d.sessions.All(), func(s *Session) bool { return !s.Headless() })
	return buildingFloors(d.cloud.seated("upstairs"), headless, overflow)
}

// upperEntry is a desk above the ground floor, as the phone is told of it: a
// cloud bot's, a headless run's, or an agent's with no desk downstairs, which
// is watched, never driven, as a headless run is. With the lock held.
func (d *Deck) upperEntry(s *Session) map[string]any {
	if bot, ok := d.cloud.botOf(s.ID); ok {
		return d.cloudEntry(s, bot)
	}
	entry := d.sessionEntry(s)
	entry["headless"] = true
	for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary"} {
		entry[control] = false
	}
	entry["role"] = nil
	entry["hiredBy"] = d.hiredBy(s)
	if !s.Headless() {
		entry["headless"], entry["overflow"] = false, true
	}
	return entry
}

// floorsState is the six floors as the phone is told of them, the ground
// floor's desks being the keys; how many there's no floor for; and the
// second floor alone, for phones from before the other floors. With the lock
// held.
func (d *Deck) floorsState(overflow []*Session) ([]map[string]any, int, map[string]any) {
	floors, over := d.building(overflow)
	entries := func(people []*Session) []map[string]any {
		desks := []map[string]any{}
		for _, s := range people {
			desks = append(desks, d.upperEntry(s))
		}
		return desks
	}
	shown := make([]map[string]any, floorCount)
	for i, floor := range checkedFloors(d.config.Floors) {
		shown[i] = map[string]any{"n": i + 1, "name": floor.Name, "preset": floor.Preset, "desks": entries(floors[i])}
	}
	second, past := secondFloor(floors, over)
	return shown, over, map[string]any{"desks": entries(second), "overflow": past}
}
