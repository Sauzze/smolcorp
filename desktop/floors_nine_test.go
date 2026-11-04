package desktop

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every floor has nine desks and a chill area, and each is arranged its own
// way on the setup page. The wire formats are the shared fixtures in
// Tests/fixtures/floors/, which the Mac app's tests also read: what each
// floor keeps of its arrangement, and where the people at desks 10 to 12 of
// a floor go now that a floor has nine.

// floorsFixture reads one of Tests/fixtures/floors/ into into.
func floorsFixture(t *testing.T, name string, into any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "floors", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatal(err)
	}
}

func TestEachFloorKeepsItsOwnArrangementAsTheMacDoes(t *testing.T) {
	var fixture struct {
		Given json.RawMessage `json:"given"`
		Kept  any             `json:"kept"`
	}
	floorsFixture(t, "floor-plans.json", &fixture)
	var room Room
	if err := json.Unmarshal(fixture.Given, &room); err != nil {
		t.Fatal(err)
	}
	if got := asJSONValue(t, room.FloorPlans); !reflect.DeepEqual(got, fixture.Kept) {
		t.Errorf("the floors keep\n%v\nwant\n%v", got, fixture.Kept)
	}
	// Sent from the setup page in a layout, and written with the room, it's the same
	layout, err := parseLayout([]byte(`{"office": true, "room": ` + string(fixture.Given) + `}`))
	if err != nil {
		t.Fatal(err)
	}
	written, _ := asJSONValue(t, layout).(map[string]any)
	sent, _ := written["room"].(map[string]any)
	if !reflect.DeepEqual(sent["floorPlans"], fixture.Kept) {
		t.Errorf("a layout sent writes the floors as\n%v\nwant\n%v", sent["floorPlans"], fixture.Kept)
	}
	// The ground floor's own arrangement stays its own
	if room.Plan.Wide["desk1"].X != 5 || len(room.Plan.Wide) != 1 {
		t.Errorf("the ground floor is arranged %v", room.Plan.Wide)
	}
}

func TestAnOldConfigLoadsAsBeforeWithEveryFloorAsItComes(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"layout": {"office": true, "room": {"corner": "jukebox", "plan": {"wide": {"desk2": {"x": 4}}}}}}`)
	config := loadConfig()
	room := config.Layout.Room
	if !config.Layout.Office || room.Corner != "jukebox" || room.Plan.Wide["desk2"].X != 4 || len(room.FloorPlans) != 0 {
		t.Fatalf("a config from before loads with the office %v, the corner %q, desk 2 at %d and the floors %v; want it as it was, every floor as it comes",
			config.Layout.Office, room.Corner, room.Plan.Wide["desk2"].X, room.FloorPlans)
	}
	if written := string(must(json.Marshal(room))); !strings.Contains(written, `"floorPlans":{}`) {
		t.Errorf("the room is written\n%s\nwant no floor arranged", written)
	}
	// Saved, and read again, it's still as it was
	config.save()
	if again := loadConfig().Layout.Room; !reflect.DeepEqual(asJSONValue(t, again), asJSONValue(t, room)) {
		t.Errorf("saved and read again, the room is %v, want %v", asJSONValue(t, again), asJSONValue(t, room))
	}
}

// floorsCase is one of Tests/fixtures/floors/desks-10-to-12.json: the desks
// kept across a restart and the floors people were put on, and where they
// are after, or nothing when nobody needs to move.
type floorsCase struct {
	Case   string          `json:"case"`
	Seats  map[string]Seat `json:"seats"`
	Floors map[string]int  `json:"floors"`
	After  *struct {
		Seats  map[string]Seat `json:"seats"`
		Floors map[string]int  `json:"floors"`
	} `json:"after"`
}

func TestPeopleAtDesksTenToTwelveMoveAsTheMacMovesThem(t *testing.T) {
	var cases []floorsCase
	floorsFixture(t, "desks-10-to-12.json", &cases)
	if len(cases) != 3 {
		t.Fatalf("%d cases, want 3", len(cases))
	}
	for _, c := range cases {
		seats, floors, moved := onFloorDesks(maps.Clone(c.Seats), maps.Clone(c.Floors))
		if c.After == nil {
			if moved || !maps.Equal(seats, c.Seats) || !maps.Equal(floors, c.Floors) {
				t.Errorf("%s:\nmoved %v, to %v and %v", c.Case, moved, seats, floors)
			}
			continue
		}
		if !moved || !maps.Equal(seats, c.After.Seats) || !maps.Equal(floors, c.After.Floors) {
			t.Errorf("%s:\nthe desks are %v\nand the floors %v\nwant %v\nand %v", c.Case, seats, floors, c.After.Seats, c.After.Floors)
		}
		// Moved once: from then on, everyone stays where they are
		if again, put, moved := onFloorDesks(maps.Clone(seats), maps.Clone(floors)); moved || !maps.Equal(again, seats) || !maps.Equal(put, floors) {
			t.Errorf("%s: moved again, to %v and %v", c.Case, again, put)
		}
	}
}

func TestASavedDeckIsMovedOnceAtLoad(t *testing.T) {
	var cases []floorsCase
	floorsFixture(t, "desks-10-to-12.json", &cases)
	first := cases[0]
	file := filepath.Join(t.TempDir(), "sessions.json")
	before := savedDeck{SavedAt: 1, Keys: map[string]int{}, Floors: first.Floors, Seats: first.Seats,
		Sessions: []savedSession{{ID: "m1", Agent: string(Claude), Project: "/work/api", Tab: "headless:m1", State: "idle"}}}
	writeFile(t, file, string(must(json.Marshal(before))))
	saved, ok := loadSavedDeck(file)
	if !ok || !maps.Equal(saved.Seats, first.After.Seats) || !maps.Equal(saved.Floors, first.After.Floors) || len(saved.Sessions) != 1 {
		t.Fatalf("loaded %v: the desks are %v and the floors %v, want %v and %v", ok, saved.Seats, saved.Floors, first.After.Seats, first.After.Floors)
	}
	// Written back as moved, the rest as it was
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var kept savedDeck
	if err := json.Unmarshal(data, &kept); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(kept.Seats, first.After.Seats) || !maps.Equal(kept.Floors, first.After.Floors) || kept.SavedAt != 1 ||
		len(kept.Sessions) != 1 || kept.Sessions[0].ID != "m1" {
		t.Errorf("the file keeps %+v", kept)
	}
	// Loaded again, nothing moves and nothing's written
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	again, ok := loadSavedDeck(file)
	now, _ := os.ReadFile(file)
	still, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !maps.Equal(again.Seats, saved.Seats) || !maps.Equal(again.Floors, saved.Floors) || string(now) != string(data) || !os.SameFile(info, still) {
		t.Errorf("loaded again, the desks are %v and the floors %v, and the file was written again: %v", again.Seats, again.Floors, !os.SameFile(info, still))
	}
	if _, ok := loadSavedDeck(filepath.Join(t.TempDir(), "none.json")); ok {
		t.Error("with no file, a deck was loaded")
	}
}

func TestEveryFloorUpstairsHasNineDesks(t *testing.T) {
	if desksPerFloor != 9 {
		t.Fatalf("a floor has %d desks, want 9: the office draws three rows of three on every floor", desksPerFloor)
	}
	var crowd []upstairsPerson[string]
	for i := range desksPerFloor + 1 {
		crowd = append(crowd, person(fmt.Sprintf("p%d", i), 2, nil))
	}
	floors, over, seats := seatBuilding(crowd, nil)
	last := fmt.Sprintf("p%d", desksPerFloor)
	if len(floors[2]) != desksPerFloor || over != 0 || seats[last] != (Seat{1, 0}) {
		t.Errorf("3F holds %d and %s sits at %v, %d over; want %d, and the last on the first floor with room", len(floors[2]), last, seats[last], over, desksPerFloor)
	}
	for id, seat := range seats {
		if seat.Desk < 0 || seat.Desk >= desksPerFloor {
			t.Errorf("%s sits at desk %d", id, seat.Desk+1)
		}
	}
}
