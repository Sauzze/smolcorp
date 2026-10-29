package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func people(prefix string, count int) []string {
	list := []string{}
	for i := range count {
		list = append(list, fmt.Sprintf("%s%d", prefix, i))
	}
	return list
}

func floorSizes(floors [][]string) []int {
	sizes := []int{}
	for _, floor := range floors {
		sizes = append(sizes, len(floor))
	}
	return sizes
}

func TestANewBuildingHasNobodyAboveTheGroundFloor(t *testing.T) {
	floors, over := buildingFloors[string](nil, nil, nil)
	if !reflect.DeepEqual(floorSizes(floors), []int{0, 0, 0, 0, 0, 0}) || over != 0 {
		t.Fatalf("an empty building is %v, %d over", floors, over)
	}
}

func TestTwelveFillTheSecondFloorAndNoMore(t *testing.T) {
	floors, over := buildingFloors(nil, people("h", 12), nil)
	if !reflect.DeepEqual(floors[1], people("h", 12)) || len(floors[2]) != 0 || over != 0 {
		t.Fatalf("twelve runs are %v, %d over", floors, over)
	}
}

func TestTheThirteenthGoesUpToTheThirdFloor(t *testing.T) {
	floors, over := buildingFloors(nil, people("h", 13), nil)
	if len(floors[1]) != 12 || !reflect.DeepEqual(floors[2], []string{"h12"}) || over != 0 {
		t.Fatalf("thirteen runs are %v, %d over", floors, over)
	}
	if _, past := secondFloor(floors, over); past != 1 {
		t.Errorf("%d past the second floor, want 1", past)
	}
}

func TestSixtyFillFiveFloorsAndTheSixtyFirstHasNone(t *testing.T) {
	floors, over := buildingFloors(nil, people("h", 60), nil)
	if !reflect.DeepEqual(floorSizes(floors), []int{0, 12, 12, 12, 12, 12}) || floors[5][11] != "h59" || over != 0 {
		t.Fatalf("sixty runs are %v, %d over", floorSizes(floors), over)
	}
	floors, over = buildingFloors(nil, people("h", 61), nil)
	if !reflect.DeepEqual(floorSizes(floors), []int{0, 12, 12, 12, 12, 12}) || over != 1 {
		t.Fatalf("sixty-one runs are %v, %d over", floorSizes(floors), over)
	}
	if _, past := secondFloor(floors, over); past != 49 {
		t.Errorf("%d past the second floor, want 49", past)
	}
}

// Cloud bots first, then headless runs, then the agents with no desk downstairs.
func TestCloudBotsGoUpFirstThenRunsThenThoseWithNoDesk(t *testing.T) {
	floors, over := buildingFloors(people("c", 2), people("h", 11), people("o", 3))
	if len(floors[0]) != 0 {
		t.Errorf("the ground floor holds %v: its desks are its keys", floors[0])
	}
	if want := append(people("c", 2), people("h", 10)...); !reflect.DeepEqual(floors[1], want) {
		t.Errorf("2F is %v, want %v", floors[1], want)
	}
	if want := append([]string{"h10"}, people("o", 3)...); !reflect.DeepEqual(floors[2], want) || over != 0 {
		t.Errorf("3F is %v, %d over, want %v", floors[2], over, want)
	}
}

func TestTheFloorsAre1FTo6FInTheOfficesOwnStyleUntilChanged(t *testing.T) {
	isolate(t)
	floors := loadConfig().Floors
	for i, floor := range floors {
		if floor != (Floor{Name: fmt.Sprintf("%dF", i+1)}) {
			t.Errorf("floor %d is %+v", i+1, floor)
		}
	}
	if len(floors) != 6 {
		t.Fatalf("%d floors, want 6", len(floors))
	}
}

// Six floors, each name one line of 24 at most, each style one there is; the
// ground floor's always the office's own. And what this version doesn't read is kept.
func TestTheFloorsAreCheckedAsTheyLoadAndKeptAsSaved(t *testing.T) {
	isolate(t)
	long := strings.Repeat("x", 30)
	file := `{"floors": [{"name": "Lobby", "preset": "space"}, {"name": "  Ops\n\u0007 ", "preset": "server"},
		{"name": "   ", "preset": "nowhere"}, {"name": "` + long + `", "preset": "yours"}, 42,
		{"name": "Roof", "preset": "pirates"}, {"name": "Seventh", "preset": "space"}], "somethingElse": 1}`
	if err := os.WriteFile(configFile, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	config := loadConfig()
	want := floorList{{"Lobby", ""}, {"Ops", "server"}, {"3F", ""}, {strings.Repeat("x", 24), ""}, {"5F", ""}, {"Roof", "pirates"}}
	if !reflect.DeepEqual(config.Floors, want) {
		t.Fatalf("the floors are %+v, want %+v", config.Floors, want)
	}
	if _, err := os.Stat(configFile + ".broken"); err == nil {
		t.Error("a floor that isn't one set the whole file aside")
	}
	config.save()
	if saved := loadConfig(); !reflect.DeepEqual(saved.Floors, want) || saved.unknown["somethingElse"] == nil {
		t.Errorf("saved and read back, the floors are %+v and the rest %v", saved.Floors, saved.unknown)
	}
	if short := checkedFloors([]Floor{{"Hall", "space"}}); short[0].Name != "Hall" || short[5].Name != "6F" {
		t.Errorf("a short list is %+v", short)
	}
}

// Setup sends the floors under "floors" or as the list alone, and is answered
// with its state; anything else isn't floors. The phones see them at once.
func TestSetupSendsTheFloorsAndThePhonesSeeThem(t *testing.T) {
	d := newTestDeck(t)
	send := func(body string) (int, map[string]any) {
		w := serve(t, d, call{method: "POST", target: "/setup/floors", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
		var state map[string]any
		json.Unmarshal(w.Body.Bytes(), &state)
		return w.Code, state
	}
	code, state := send(`{"floors": [{"name": "", "preset": "space"}, {"name": "Lab", "preset": "space"}]}`)
	floors, _ := state["floors"].([]any)
	if code != 200 || len(floors) != 6 {
		t.Fatalf("setup answered %d with floors %v", code, state["floors"])
	}
	if lab, _ := floors[1].(map[string]any); lab["name"] != "Lab" || lab["preset"] != "space" {
		t.Errorf("2F is %v", floors[1])
	}
	if presets, _ := state["presets"].([]any); slices.Contains(presets, any("yours")) || !slices.Contains(presets, any("space")) {
		t.Errorf("the styles offered are %v", state["presets"])
	}
	if code, _ := send(`[{"name": "Hall"}]`); code != 200 || d.config.Floors[0].Name != "Hall" || d.config.Floors[1].Name != "2F" {
		t.Errorf("a bare list gave %d and floors %+v", code, d.config.Floors)
	}
	for _, bad := range []string{`{"floors": "Lab"}`, `nope`} {
		if code, _ := send(bad); code != 400 {
			t.Errorf("%s gave %d, want 400", bad, code)
		}
	}
	if w := serve(t, d, call{method: "POST", target: "/setup/floors", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: `[]`, withoutSetupKey: true}); w.Code != 401 {
		t.Errorf("without the setup key, %d", w.Code)
	}
}

// The snapshot's floors: a desk each upstairs for the runs, 12 a floor, then
// the agents with no desk downstairs, watched, never driven; upstairs is the
// second floor alone, for older phones.
func TestTheSnapshotSaysWhoIsOnEachFloor(t *testing.T) {
	d := newTestDeck(t)
	d.config.Floors = checkedFloors([]Floor{{}, {"Ops", "server"}})
	running(d, mostKeys+1)
	headless(d, 12)
	d.mu.Lock()
	data := d.snapshot()
	d.mu.Unlock()
	var state struct {
		Overflow       int
		FloorsOverflow int
		Floors         []struct {
			N            int
			Name, Preset string
			Desks        []map[string]any
		}
		Upstairs struct {
			Desks    []map[string]any
			Overflow int
		}
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Floors) != 6 || state.Floors[0].N != 1 || len(state.Floors[0].Desks) != 0 || state.Floors[5].N != 6 {
		t.Fatalf("the floors are %+v", state.Floors)
	}
	if f := state.Floors[1]; f.Name != "Ops" || f.Preset != "server" || len(f.Desks) != 12 || f.Desks[0]["headless"] != true {
		t.Errorf("2F is %s %q with %d desks, the first %v", f.Name, f.Preset, len(f.Desks), f.Desks)
	}
	third := state.Floors[2].Desks
	if len(third) != 1 || third[0]["id"] != fmt.Sprintf("s%d", mostKeys) || third[0]["headless"] != false || third[0]["overflow"] != true {
		t.Fatalf("3F is %v, want the session with no desk downstairs", third)
	}
	for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary"} {
		if third[0][control] != false {
			t.Errorf("a desk with no key has %s %v", control, third[0][control])
		}
	}
	if state.Overflow != 1 || state.FloorsOverflow != 0 || len(state.Upstairs.Desks) != 12 || state.Upstairs.Overflow != 1 {
		t.Errorf("over %d downstairs and %d past 6F; upstairs %d desks and %d past it", state.Overflow, state.FloorsOverflow,
			len(state.Upstairs.Desks), state.Upstairs.Overflow)
	}
}

// The viewer's floors: your names for them only with names shown, and the
// desks numbered on from the floor below with names hidden.
func TestTheViewerNumbersHiddenDesksAcrossTheFloors(t *testing.T) {
	d := newTestDeck(t)
	d.config.Floors = checkedFloors([]Floor{{}, {}, {"Secret lab", "space"}})
	headless(d, 13)
	for _, hideNames := range []bool{false, true} {
		d.mu.Lock()
		snapshot := d.viewerSnapshot(hideNames, func(id string) string { return id })
		d.mu.Unlock()
		floors, _ := snapshot["floors"].([]map[string]any)
		if len(floors) != 6 {
			t.Fatalf("names hidden %v: %d floors", hideNames, len(floors))
		}
		name, desk := "Secret lab", "api"
		if hideNames {
			name, desk = "3F", "Upstairs 13"
		}
		third, _ := floors[2]["desks"].([]map[string]any)
		if floors[2]["name"] != name || floors[2]["preset"] != "space" || len(third) != 1 || third[0]["name"] != desk {
			t.Errorf("names hidden %v: 3F is %v", hideNames, floors[2])
		}
		if upstairs, _ := snapshot["upstairs"].(map[string]any); upstairs["overflow"] != 1 || snapshot["floorsOverflow"] != 0 {
			t.Errorf("names hidden %v: upstairs %v, %v past 6F", hideNames, upstairs["overflow"], snapshot["floorsOverflow"])
		}
	}
}
