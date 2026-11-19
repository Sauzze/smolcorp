package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
)

// A config from before floors had pins reads the same: its nine pins are the ground floor's, nothing is pinned
// upstairs, and saved again it's the same file, with no floorPins added.
func TestPinsFromBeforeFloorsHadThemReadTheSame(t *testing.T) {
	isolate(t)
	if err := os.WriteFile(configFile, []byte(`{"pins": ["/work/a", null, "/work/c"], "somethingElse": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config := loadConfig()
	if len(config.Pins) != mostKeys || *config.Pins[0] != "/work/a" || config.Pins[1] != nil || *config.Pins[2] != "/work/c" || len(config.FloorPins) != 0 {
		t.Fatalf("the pins are %v, upstairs %+v", config.Pins, config.FloorPins)
	}
	config.save()
	data, _ := os.ReadFile(configFile)
	var saved map[string]json.RawMessage
	json.Unmarshal(data, &saved)
	if _, ok := saved["floorPins"]; ok || saved["somethingElse"] == nil {
		t.Errorf("saved again, the file is %s", data)
	}
	if again := loadConfig(); !reflect.DeepEqual(again.Pins, config.Pins) {
		t.Errorf("read back, the pins are %v", again.Pins)
	}
}

// Pins upstairs are checked as they load: one that can't be read, or isn't on a desk upstairs, loses only itself;
// one a desk and one a folder, the last winning; a folder pinned downstairs keeps that pin. Saved, they're kept.
func TestPinsUpstairsAreCheckedAsTheyLoadAndKeptAsSaved(t *testing.T) {
	isolate(t)
	file := `{"pins": ["/work/down"], "floorPins": [42, {"floor": 0, "desk": 1, "folder": "/work/x"}, {"floor": 6, "desk": 1, "folder": "/work/x"},
		{"floor": 1, "desk": 9, "folder": "/work/x"}, {"floor": 1, "desk": 1, "folder": ""}, {"floor": 1, "desk": 1, "folder": "/work/down"},
		{"floor": 2, "desk": 3, "folder": "/work/a"}, {"floor": 2, "desk": 4, "folder": "/work/a"},
		{"floor": 5, "desk": 8, "folder": "/work/b"}, {"floor": 5, "desk": 8, "folder": "/work/c"}]}`
	if err := os.WriteFile(configFile, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	want := floorPinList{{Floor: 2, Desk: 4, Folder: "/work/a"}, {Floor: 5, Desk: 8, Folder: "/work/c"}}
	config := loadConfig()
	if !reflect.DeepEqual(config.FloorPins, want) || *config.Pins[0] != "/work/down" {
		t.Fatalf("the pins upstairs are %+v, downstairs %v", config.FloorPins, config.Pins)
	}
	config.save()
	if again := loadConfig(); !reflect.DeepEqual(again.FloorPins, want) {
		t.Errorf("saved and read back, %+v", again.FloorPins)
	}
	// Not a list: none, and the rest of the file is read.
	os.WriteFile(configFile, []byte(`{"pins": ["/work/down"], "floorPins": "nope"}`), 0o600)
	if config := loadConfig(); len(config.FloorPins) != 0 || *config.Pins[0] != "/work/down" {
		t.Errorf("a floorPins that isn't a list gave %+v and %v", config.FloorPins, config.Pins)
	}
	if _, err := os.Stat(configFile + ".broken"); err == nil {
		t.Error("a floorPins that isn't a list set the whole file aside")
	}
}

// A folder has one desk in the building: pinned on another floor, it leaves the one it had; a swap upstairs moves
// only the two desks' pins.
func TestAFolderHasOneDeskInTheBuilding(t *testing.T) {
	isolate(t)
	c := loadConfig()
	c.PinOn("/work/a", 0, 2)
	c.PinOn("/work/a", 3, 4)
	if c.Pins[2] != nil || c.pinAt(3, 4) == nil || *c.pinAt(3, 4) != "/work/a" {
		t.Fatalf("pinned upstairs, the ground floor has %v and 4F %+v", c.Pins, c.FloorPins)
	}
	c.PinOn("/work/a", 0, 1)
	if len(c.FloorPins) != 0 || *c.Pins[1] != "/work/a" {
		t.Fatalf("pinned downstairs again, 4F has %+v", c.FloorPins)
	}
	b := "/work/b"
	c.PinOn(b, 2, 0)
	a := "/work/a"
	c.SwapPinsOn(2, &b, 0, &a, 5)
	if *c.pinAt(2, 5) != b || *c.pinAt(2, 0) != a || c.Pins[1] != nil {
		t.Errorf("swapped on 3F: %+v, downstairs %v", c.FloorPins, c.Pins)
	}
	c.UnpinOn(2, 0)
	if c.pinAt(2, 0) != nil || *c.pinAt(2, 5) != b {
		t.Errorf("unpinned on 3F: %+v", c.FloorPins)
	}
}

// pinPost is a pin or unpin route, with the token or key given in the header given (none: the page's cookie).
func pinPost(t *testing.T, d *Deck, target, header, token string) int {
	t.Helper()
	c := call{method: "POST", target: target, host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}}
	if header != "" {
		c.withoutSetupKey, c.headers[header] = true, token
	}
	return serve(t, d, c).Code
}

// The page and the GM pin and unpin on any floor; no floor is the ground floor, as before. A hire and the
// setup-key file can't, on any floor.
func TestPinAndUnpinOnEveryFloorThroughTheRoutes(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, hireKey := withGMAndAHire(t, d)
	for _, who := range []struct{ name, header, token string }{{"the page", "", ""}, {"the GM", gmTokenHeader, gm}} {
		if got := pinPost(t, d, "/setup/pin?key=4&floor=2&folder=/work/mid", who.header, who.token); got != http.StatusNoContent {
			t.Fatalf("%s pinning on 3F: %d", who.name, got)
		}
		if pin := d.config.pinAt(2, 4); pin == nil || *pin != "/work/mid" {
			t.Fatalf("%s pinned on 3F: %+v", who.name, d.config.FloorPins)
		}
		if got := pinPost(t, d, "/setup/unpin?key=4&floor=2", who.header, who.token); got != http.StatusNoContent || d.config.pinAt(2, 4) != nil {
			t.Errorf("%s unpinning on 3F: %d, %+v", who.name, got, d.config.FloorPins)
		}
		// No floor: the ground floor's key, as callers from before send it.
		if got := pinPost(t, d, "/setup/pin?key=3&folder=/work/old", who.header, who.token); got != http.StatusNoContent || d.config.Pins[3] == nil {
			t.Errorf("%s pinning with no floor: %d, %v", who.name, got, d.config.Pins)
		}
		pinPost(t, d, "/setup/unpin?key=3", who.header, who.token)
		for _, bad := range []string{"/setup/pin?key=0&floor=6&folder=/x", "/setup/pin?key=0&floor=-1&folder=/x", "/setup/pin?key=0&floor=x&folder=/x",
			"/setup/pin?key=9&floor=1&folder=/x", "/setup/unpin?key=0&floor=6"} {
			if got := pinPost(t, d, bad, who.header, who.token); got != http.StatusBadRequest {
				t.Errorf("%s %s: %d, want 400", who.name, bad, got)
			}
		}
	}
	d.config.PinOn("/work/kept", 2, 4)
	for _, who := range []struct{ name, header, token string }{{"a hire", hireKeyHeader, hireKey}, {"the setup-key file", setupKeyHeader, d.access.SetupKey}} {
		for _, target := range []string{"/setup/pin?key=1&floor=2&folder=/work/x", "/setup/unpin?key=4&floor=2"} {
			if got := pinPost(t, d, target, who.header, who.token); got != http.StatusForbidden {
				t.Errorf("%s %s: %d, want 403", who.name, target, got)
			}
		}
	}
	if pin := d.config.pinAt(2, 4); pin == nil || *pin != "/work/kept" || d.config.pinAt(2, 1) != nil {
		t.Errorf("a refused caller changed the pins on 3F: %+v", d.config.FloorPins)
	}
}

// pinnedFloor is each desk on floor (0 for the ground floor) by its session's id, as the snapshot says, and its pins.
func pinnedFloor(t *testing.T, d *Deck, floor int) (map[float64]string, []any) {
	t.Helper()
	d.mu.Lock()
	data := d.snapshot()
	d.mu.Unlock()
	var state struct {
		Floors []struct {
			Desks []map[string]any
			Pins  []any
		}
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	desks := map[float64]string{}
	for _, desk := range state.Floors[floor].Desks {
		desks[desk["desk"].(float64)] = desk["id"].(string)
	}
	return desks, state.Floors[floor].Pins
}

// Carried from one desk to another upstairs, from the office's window or a phone, two agents swap and stay so,
// as downstairs: each folder pinned where the other was. Carried to an empty desk, it's kept for them.
func TestADragUpstairsSwapsTheDesksAndKeepsThem(t *testing.T) {
	d := newTestDeck(t)
	running(d, 3)
	d.config.FloorRules = checkedFloorRules([]FloorRule{{"/work/p1", 1, true}, {"/work/p2", 1, true}})
	desks, _ := pinnedFloor(t, d, 1)
	if desks[0] != "s1" || desks[1] != "s2" {
		t.Fatalf("2F is %v", desks)
	}
	phone := pair(t, d)
	swap := func(as, from, to, floor string) int {
		target := "/act?action=swap&from=" + from + "&to=" + to + "&floor=" + floor
		if as == "window" {
			return serve(t, d, call{method: "POST", target: target + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer}).Code
		}
		return serve(t, d, call{method: "POST", target: target + "&t=" + phone, host: deckOnWiFi, from: aPhone}).Code
	}
	if got := swap("window", "0", "1", "1"); got != http.StatusNoContent {
		t.Fatalf("the window's drag on 2F: %d", got)
	}
	desks, pins := pinnedFloor(t, d, 1)
	if desks[0] != "s2" || desks[1] != "s1" || pins[0] != "p2" || pins[1] != "p1" {
		t.Fatalf("after the drag 2F is %v, pinned %v", desks, pins)
	}
	// The phone's, to an empty desk: they sit there, and the desk they left is free.
	if got := swap("phone", "1", "6", "1"); got != http.StatusNoContent {
		t.Fatalf("the phone's drag on 2F: %d", got)
	}
	desks, pins = pinnedFloor(t, d, 1)
	if desks[6] != "s1" || desks[0] != "s2" || pins[1] != nil || pins[6] != "p1" {
		t.Errorf("after the phone's drag 2F is %v, pinned %v", desks, pins)
	}
	// The ground floor's keys were left alone, and a swap with no floor is still theirs.
	if d.config.Pins[0] != nil || d.config.Pins[1] != nil {
		t.Errorf("a drag upstairs pinned downstairs: %v", d.config.Pins)
	}
	for _, bad := range [][3]string{{"0", "1", "6"}, {"0", "1", "x"}, {"0", "9", "1"}, {"3", "3", "1"}} {
		if got := swap("phone", bad[0], bad[1], bad[2]); got != http.StatusBadRequest {
			t.Errorf("a swap of %v: %d, want 400", bad, got)
		}
	}
	// Two empty desks: nothing to do.
	if got := swap("window", "4", "5", "3"); got != http.StatusNoContent || len(d.config.FloorPins) != 2 {
		t.Errorf("two empty desks on 4F: %d, %+v", got, d.config.FloorPins)
	}
}

// A desk pinned upstairs is kept for its folder: its session sits there, on that floor even with no rule, and
// nobody else does meanwhile. The phones are told, as a key's pin, by the folder's name.
func TestADeskPinnedUpstairsIsKeptForItsFolder(t *testing.T) {
	d := newTestDeck(t)
	running(d, 2)
	d.config.PinOn("/work/p0", 2, 4)
	desks, pins := pinnedFloor(t, d, 2)
	if desks[4] != "s0" || pins[4] != "p0" {
		t.Fatalf("pinned to desk 5 on 3F with no rule, s0 is not there: %v %v", desks, pins)
	}
	d.mu.Lock()
	keys, _ := d.layout()
	d.mu.Unlock()
	for _, key := range keys {
		if key.session != nil && key.session.ID == "s0" {
			t.Errorf("s0 has a key downstairs too")
		}
	}
	// A rule says otherwise: it sits on its rule's floor, and its desk on 3F is kept empty, as a pinned key is
	// downstairs while its folder is upstairs.
	d.config.FloorRules = checkedFloorRules([]FloorRule{{"/work/p0", 1, true}, {"/work/p1", 2, true}})
	desks, _ = pinnedFloor(t, d, 2)
	if _, taken := desks[4]; taken || desks[0] != "s1" {
		t.Errorf("3F is %v: desk 5 should be kept for p0, s1 at the first free desk", desks)
	}
	if up, _ := pinnedFloor(t, d, 1); up[0] != "s0" {
		t.Errorf("2F is %v, want s0 there by its rule", up)
	}
}

// Seating: a pinned desk is its folder's alone, the one that had it first keeps it, and the floor's room counts it.
func TestSeatingKeepsPinnedDesks(t *testing.T) {
	pin := &Seat{1, 0}
	had := Seat{1, 0}
	placed := []upstairsPerson[string]{
		{person: "new", id: "new", floor: 1},
		{person: "twin", id: "twin", floor: 1, pin: pin},
		{person: "pinned", id: "pinned", floor: 1, pin: pin, had: &had},
		{person: "elsewhere", id: "elsewhere", floor: 2, pin: &Seat{1, 1}},
	}
	floors, over, seats := seatBuilding(placed, nil, Seat{1, 0}, Seat{1, 1})
	if seats["pinned"] != (Seat{1, 0}) || seats["new"] == (Seat{1, 1}) || seats["twin"] == (Seat{1, 1}) || seats["elsewhere"].Floor != 2 || over != 0 {
		t.Fatalf("seats %v", seats)
	}
	for _, one := range floors[1] {
		if one.desk == 1 {
			t.Errorf("%s sits at the desk kept for a folder on 3F", one.person)
		}
	}
	d := newTestDeck(t)
	for desk := range desksPerFloor - 1 {
		d.config.FloorPins = append(d.config.FloorPins, FloorPin{Floor: 1, Desk: desk, Folder: "/work/kept" + string(rune('a'+desk))})
	}
	running(d, 2)
	d.mu.Lock()
	room := d.hasRoom(1, "s0")
	d.floorOf["s0"] = 1
	full := !d.hasRoom(1, "s1")
	d.mu.Unlock()
	if !room || !full {
		t.Errorf("2F with eight desks kept: room for one %v, then full %v", room, full)
	}
}

// The GM's pin routes are the page's: pinned on a floor with the GM's token, the desk is kept there for its
// folder, and an unpin frees it.
func TestTheGMPinsSomeoneToTheMiddleDeskUpstairs(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	running(d, 1)
	if got := pinPost(t, d, "/setup/pin?key=4&floor=3&folder="+url.QueryEscape("/work/p0"), gmTokenHeader, gm); got != http.StatusNoContent {
		t.Fatalf("the GM's pin: %d", got)
	}
	if desks, pins := pinnedFloor(t, d, 3); desks[4] != "s0" || pins[4] != "p0" {
		t.Errorf("4F is %v %v", desks, pins)
	}
	pinPost(t, d, "/setup/unpin?key=4&floor=3", gmTokenHeader, gm)
	if desks, _ := pinnedFloor(t, d, 3); len(desks) != 0 || len(d.config.FloorPins) != 0 {
		t.Errorf("unpinned, 4F is %v and the pins %+v", desks, d.config.FloorPins)
	}
}
