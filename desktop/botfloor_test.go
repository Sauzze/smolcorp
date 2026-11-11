package desktop

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// Bots on any floor (T762), as the Mac app has them (BotFloorTests.swift):
// each where you put it, or where a floor rule for its folder says, or where
// new bots go; a full floor sends it where there's room; and an old config's
// bots stay where they sat.

// MARK: Old configs

// Each config of the fixture the Mac app loads too loads, and as the app
// starts, its bots still "upstairs" go on the floors they sat on, by the desks
// saved for them, with the floor new bots go to; saved and started again,
// nothing changes.
func TestOldConfigsLoadWithTheirBotsWhereTheySitAndLoadAgainUnchanged(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "bots", "configs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		About  string          `json:"about"`
		Before json.RawMessage `json:"before"`
		// The desks saved across a restart (sessions.json's seats), by desk id: where each bot sat.
		Seats map[string]Seat `json:"seats"`
		After struct {
			BotFloor   int        `json:"botFloor"`
			Bots       [][]string `json:"bots"`
			Pinned     []string   `json:"pinned"`
			FloorNames []string   `json:"floorNames"`
			HireFolder *string    `json:"hireFolder"`
		} `json:"after"`
	}
	if err := json.Unmarshal(data, &cases); err != nil || len(cases) != 6 {
		t.Fatalf("the fixture's %d cases: %v", len(cases), err)
	}
	for _, c := range cases {
		isolate(t)
		if err := os.WriteFile(configFile, c.Before, 0o600); err != nil {
			t.Fatal(err)
		}
		check := func(when string, loaded *Config) {
			bots, pinned := [][]string{}, []string{}
			for _, bot := range loaded.CloudBots {
				bots = append(bots, []string{bot.ID, bot.Floor})
				if bot.Pinned {
					pinned = append(pinned, bot.ID)
				}
			}
			if fmt.Sprint(bots) != fmt.Sprint(c.After.Bots) || loaded.BotFloor != c.After.BotFloor {
				t.Errorf("%s (%s):\nbots %v, new bots on %d\nwant %v, on %d", c.About, when, bots, loaded.BotFloor, c.After.Bots, c.After.BotFloor)
			}
			if c.After.Pinned != nil && !slices.Equal(pinned, c.After.Pinned) {
				t.Errorf("%s (%s): pinned %v, want %v", c.About, when, pinned, c.After.Pinned)
			}
			if c.After.FloorNames != nil {
				names := []string{}
				for _, floor := range checkedFloors(loaded.Floors) {
					names = append(names, floor.Name)
				}
				if !slices.Equal(names, c.After.FloorNames) {
					t.Errorf("%s (%s): floors %v, want %v", c.About, when, names, c.After.FloorNames)
				}
			}
			if c.After.HireFolder != nil && loaded.HireFolder != *c.After.HireFolder {
				t.Errorf("%s (%s): the hire folder is %q", c.About, when, loaded.HireFolder)
			}
		}
		config := loadConfig()
		config.settleBots(c.Seats)
		check("as the app starts", config)
		config.save()
		started := loadConfig()
		if started.settleBots(c.Seats) {
			t.Errorf("%s: settled again", c.About)
		}
		check("saved and started again", started)
	}
}

// MARK: In the office

// floorRoom is as many desks as a floor above the ground floor has, as the
// building seats them.
func floorRoom() int {
	var crowd []upstairsPerson[int]
	for i := range 99 {
		crowd = append(crowd, upstairsPerson[int]{person: i, id: strconv.Itoa(i), floor: 1})
	}
	floors, _, _ := seatBuilding(crowd, nil)
	return len(floors[1])
}

// someBots is count bots with no floor of their own, b<first> on.
func someBots(count, first int) []CloudBot {
	bots := []CloudBot{}
	for i := first; i < first+count; i++ {
		bots = append(bots, standardCloudBotNamed("b"+strconv.Itoa(i), "B"+strconv.Itoa(i)))
	}
	return bots
}

// botSessions is the bots' desks' ids.
func botSessions(bots []CloudBot) []string {
	ids := []string{}
	for _, bot := range bots {
		ids = append(ids, bot.sessionID())
	}
	return ids
}

// seating is who sits where: at each key, and on each floor above, by their desks' ids.
func seating(d *Deck) (keys []string, floors [][]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	at, overflow := d.layout()
	for _, key := range at {
		id := ""
		if key.session != nil {
			id = key.session.ID
		}
		keys = append(keys, id)
	}
	seated, _ := d.seatedBuilding(overflow)
	for _, floor := range seated {
		ids := []string{}
		for _, one := range floor {
			ids = append(ids, one.person.ID)
		}
		floors = append(floors, ids)
	}
	return keys, floors
}

// botsAsGiven sets bots as they are, unchecked, so there may be more than the most there can be.
func botsAsGiven(d *Deck, bots ...CloudBot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config.CloudBots = bots
	d.cloud.configure(bots)
}

// seatsNow is where each sits above the ground floor, by their desks' ids: the
// floor, 1 for 2F up, and the desk there.
func seatsNow(d *Deck) map[string]Seat {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, overflow := d.layout()
	seated, _ := d.seatedBuilding(overflow)
	seats := map[string]Seat{}
	for floor, people := range seated {
		for _, one := range people {
			seats[one.person.ID] = Seat{floor, one.desk}
		}
	}
	return seats
}

// seatsShown is where the deck's last seating put each of ids, as the office
// was last drawn, with no seating of its own: for a change after which the
// deck seats the building by itself, as a bot's or a move.
func seatsShown(d *Deck, ids ...string) map[string]Seat {
	d.mu.Lock()
	defer d.mu.Unlock()
	shown := map[string]Seat{}
	for _, id := range ids {
		if seat, ok := d.seatOf[id]; ok {
			shown[id] = seat
		}
	}
	return shown
}

// runsIn starts headless runs in folder, one for each of ids, one after another from first.
func runsIn(d *Deck, folder string, first float64, ids ...string) {
	for i, id := range ids {
		e := map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": folder}
		d.sessions.Apply(e, Claude, "headless:"+id, "", first+float64(i))
	}
}

// numbered is count ids, prefix followed by 0 on.
func numbered(prefix string, count int) []string {
	ids := []string{}
	for i := range count {
		ids = append(ids, prefix+strconv.Itoa(i))
	}
	return ids
}

// fullSecondFloor fills 2F, where new bots go, with runs its folder's rule puts there.
func fullSecondFloor(t *testing.T, d *Deck) {
	t.Helper()
	d.config.FloorRules = checkedFloorRules([]FloorRule{{"/work/api", 1, false}})
	runsIn(d, "/work/api", 1000, numbered("h", floorRoom())...)
	if _, floors := seating(d); len(floors[1]) != floorRoom() {
		t.Fatalf("2F is %v, want it full of runs", floors[1])
	}
}

// cloudEvent tells bot id's desk of event, as its event route does.
func cloudEvent(d *Deck, id, event string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cloud.apply(event, id, nil)
}

func TestABotWithNoFloorSitsWhereNewBotsGoAndOnAFullFloorWhereThereIsRoom(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	d.config.BotFloor = 3
	withCloudBots(d, someBots(room+1, 0)...)
	_, floors := seating(d)
	if !slices.Equal(floors[3], botSessions(someBots(room, 0))) || !slices.Equal(floors[1], []string{fmt.Sprintf("cloud-b%d", room)}) {
		t.Errorf("4F, where new bots go, is %v and 2F %v: every desk on 4F, then one more where there's room", floors[3], floors[1])
	}
	// Where new bots go chosen again: none was put anywhere, so they all go there, the one already there at its desk.
	// The last, with 2F full, is on a floor not its own wherever it sits, so it keeps the desk it had, on 4F.
	d.config.BotFloor = 1
	_, floors = seating(d)
	if len(floors[1]) != room || floors[1][0] != fmt.Sprintf("cloud-b%d", room) || len(floors[2]) != 0 ||
		!slices.Equal(floors[3], []string{fmt.Sprintf("cloud-b%d", room-1)}) {
		t.Errorf("with new bots on 2F: 2F %v, 3F %v, 4F %v", floors[1], floors[2], floors[3])
	}
	if seat := seatsNow(d)[fmt.Sprintf("cloud-b%d", room-1)]; seat != (Seat{3, room - 1}) {
		t.Errorf("the last is at %v, want the desk it had, 4F's last", seat)
	}
}

// A bot nobody put on a floor takes nobody's desk: with 2F, where new bots go,
// full of runs, one more bot sits where there's room and nobody moves. Put on
// 2F, it's seated as anyone put there: a run there only for want of room
// makes way.
func TestABotNobodyPutOnAFullFloorTakesNobodysDesk(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	headless(d, room)
	_, before := seating(d)
	runs := before[1]
	if len(runs) != room {
		t.Fatalf("2F is %v, want it full of runs", runs)
	}
	withCloudBots(d, standardCloudBotNamed("new", "New"))
	_, floors := seating(d)
	if !slices.Equal(floors[1], runs) || !slices.Equal(floors[2], []string{"cloud-new"}) {
		t.Errorf("2F is %v and 3F %v: nobody on 2F moves for it, and it sits where there's room", floors[1], floors[2])
	}
	put := standardCloudBotNamed("new", "New")
	put.Floor = "2"
	withCloudBots(d, put)
	_, floors = seating(d)
	if floors[1][0] != "cloud-new" || !slices.Equal(floors[2], []string{"h0"}) {
		t.Errorf("put on 2F, 2F is %v and 3F %v: want it at a desk there, the run that sat there where there's room", floors[1], floors[2])
	}
}

// A bot nobody put on a floor, sent on from its full floor, keeps the desk it
// has as others come and go, as anyone not put on a floor does: one back at a
// desk it had takes it only if nobody's there.
func TestABotSentOnFromItsFullFloorKeepsItsDeskAsOthersComeAndGo(t *testing.T) {
	d := newTestDeck(t)
	fullSecondFloor(t, d)
	// A part-time one at work first, then another.
	part := standardCloudBotNamed("part", "Part")
	part.Permanent = false
	withCloudBots(d, part)
	cloudEvent(d, "part", cloudStart)
	seating(d)
	withCloudBots(d, part, standardCloudBotNamed("kept", "Kept"))
	if _, floors := seating(d); !slices.Equal(floors[2], []string{"cloud-part", "cloud-kept"}) {
		t.Fatalf("3F is %v, want both bots, 2F being full", floors[2])
	}
	cloudEvent(d, "part", cloudStop)
	if seat := seatsNow(d)["cloud-kept"]; seat != (Seat{2, 1}) {
		t.Errorf("the part-time one gone, the other is at %v, want its desk, 3F's second", seat)
	}
	// A run with no floor of its own sits at the desk the part-time one had, which comes back to another.
	runsIn(d, "/work/other", 3000, "x0")
	if seat := seatsNow(d)["x0"]; seat != (Seat{2, 0}) {
		t.Errorf("the run is at %v, want 3F's first desk", seat)
	}
	cloudEvent(d, "part", cloudStart)
	if seats := seatsNow(d); seats["cloud-part"] != (Seat{2, 2}) || seats["x0"] != (Seat{2, 0}) {
		t.Errorf("back, the part-time one is at %v and the run at %v: want it at a free desk, the run at its own", seats["cloud-part"], seats["x0"])
	}
	// An agent put on 3F: at a free desk there, every bot at its own.
	running(d, 1)
	if code, text := d.Perform("move", "s0", url.Values{"floor": {"2"}}); code != http.StatusNoContent {
		t.Fatalf("put on 3F: %d %s", code, text)
	}
	if seat := seatsNow(d)["s0"]; seat != (Seat{2, 3}) {
		t.Errorf("the agent is at %v, want 3F's free desk", seat)
	}
	if _, floors := seating(d); !slices.Equal(floors[2], []string{"x0", "cloud-kept", "cloud-part", "s0"}) {
		t.Errorf("3F is %v, want every bot at its own desk", floors[2])
	}
}

// A person sent on from a full floor keeps the desk they were given when a bot
// with no floor of its own arrives, or a part-time bot before them leaves.
func TestAPersonSentOnFromAFullFloorKeepsTheirDeskAsBotsComeAndGo(t *testing.T) {
	d := newTestDeck(t)
	fullSecondFloor(t, d)
	runsIn(d, "/work/api", 2000, "h9") // a tenth run for 2F: sent on to 3F
	if seat := seatsNow(d)["h9"]; seat != (Seat{2, 0}) {
		t.Fatalf("the tenth run is at %v, want 3F's first desk", seat)
	}
	withCloudBots(d, standardCloudBotNamed("new", "New"))
	if seats := seatsNow(d); seats["h9"] != (Seat{2, 0}) || seats["cloud-new"] != (Seat{2, 1}) {
		t.Errorf("a new bot came: the run is at %v and the bot at %v, want the run at its desk and the bot at the next", seats["h9"], seats["cloud-new"])
	}

	d = newTestDeck(t)
	fullSecondFloor(t, d)
	part := standardCloudBotNamed("part", "Part")
	part.Permanent = false
	withCloudBots(d, part)
	cloudEvent(d, "part", cloudStart)
	runsIn(d, "/work/api", 2000, "h9")
	if seats := seatsNow(d); seats["cloud-part"] != (Seat{2, 0}) || seats["h9"] != (Seat{2, 1}) {
		t.Fatalf("the part-time bot is at %v and the run at %v, want 3F's first and second desks", seats["cloud-part"], seats["h9"])
	}
	cloudEvent(d, "part", cloudStop)
	if seat := seatsNow(d)["h9"]; seat != (Seat{2, 1}) {
		t.Errorf("the part-time bot gone, the run is at %v, want its own desk, 3F's second", seat)
	}
}

// A person sent on from a full floor keeps their desk when a bot nobody put
// anywhere arrives whose own floor is the floor they were sent to, and when it
// stops and starts again.
func TestABotWhoseFloorIsWhereAPersonWasSentOnTakesAnotherDesk(t *testing.T) {
	d := newTestDeck(t)
	fullSecondFloor(t, d)
	runsIn(d, "/work/api", 2000, "h9") // a tenth run for 2F: sent on to 3F
	if seat := seatsNow(d)["h9"]; seat != (Seat{2, 0}) {
		t.Fatalf("the tenth run is at %v, want 3F's first desk", seat)
	}
	d.config.BotFloor = 2 // new bots go to 3F
	part := standardCloudBotNamed("part", "Part")
	part.Permanent = false
	withCloudBots(d, part)
	cloudEvent(d, "part", cloudStart)
	if seats := seatsNow(d); seats["h9"] != (Seat{2, 0}) || seats["cloud-part"] != (Seat{2, 1}) {
		t.Errorf("a bot for 3F came: the run is at %v and the bot at %v, want the run at its desk and the bot at the next", seats["h9"], seats["cloud-part"])
	}
	cloudEvent(d, "part", cloudStop)
	cloudEvent(d, "part", cloudStart)
	if seat := seatsNow(d)["h9"]; seat != (Seat{2, 0}) {
		t.Errorf("the bot stopped and started again: the run is at %v, want its own desk", seat)
	}
}

// A bot sent on from its full floor goes back when a desk is free there; the
// other stays at its own.
func TestABotSentOnFromItsFullFloorGoesBackWhenADeskIsFreeThere(t *testing.T) {
	d := newTestDeck(t)
	fullSecondFloor(t, d)
	withCloudBots(d, someBots(2, 0)...)
	if _, floors := seating(d); !slices.Equal(floors[2], []string{"cloud-b0", "cloud-b1"}) {
		t.Fatalf("3F is %v, want both bots, 2F being full", floors[2])
	}
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionEnd", "session_id": "h4"}, Claude, "headless:h4", "", 3000)
	if seats := seatsNow(d); seats["cloud-b0"] != (Seat{1, 4}) || seats["cloud-b1"] != (Seat{2, 1}) {
		t.Errorf("a run on 2F gone, the bots are at %v and %v: want the first back on 2F at the run's desk, the other at its own",
			seats["cloud-b0"], seats["cloud-b1"])
	}
}

// After a restart, a bot sent on from its full floor is back at the desk it
// had, as an agent still running is.
func TestABotSentOnFromItsFullFloorGetsItsDeskBackAfterARestart(t *testing.T) {
	d := newTestDeck(t)
	fullSecondFloor(t, d)
	withCloudBots(d, standardCloudBotNamed("kept", "Kept"))
	// Seated once as the app starts, at the first free desk, before what was saved is read.
	if seat := seatsNow(d)["cloud-kept"]; seat != (Seat{2, 0}) {
		t.Fatalf("the bot is at %v, want 3F's first desk", seat)
	}
	d.mu.Lock()
	d.putBackBots(savedDeck{Seats: map[string]Seat{"cloud-kept": {Floor: 2, Desk: 5}}})
	d.mu.Unlock()
	if seat := seatsNow(d)["cloud-kept"]; seat != (Seat{2, 5}) {
		t.Errorf("the bot is at %v, want the desk it had, 3F's sixth", seat)
	}
}

// On a full floor, one put there takes the desk of a bot there only for want
// of room on its own, as of a run there for want of room, be it a bot put
// there or a run its folder's rule puts there: the bot goes where there's room.
func TestOnAFullFloorABotSentOnFromItsOwnMakesWayForAnyonePutThere(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	fullSecondFloor(t, d)
	d.config.FloorRules = checkedFloorRules(append(d.config.FloorRules, FloorRule{"/work/web", 2, false}))
	runsIn(d, "/work/web", 2000, numbered("w", room-2)...)
	seating(d)
	withCloudBots(d, someBots(2, 0)...)
	if seats := seatsNow(d); seats["cloud-b0"] != (Seat{2, room - 2}) || seats["cloud-b1"] != (Seat{2, room - 1}) {
		t.Fatalf("the bots are at %v and %v, want 3F's last desks, 2F being full", seats["cloud-b0"], seats["cloud-b1"])
	}
	// Seated by the deck itself as the bots change: as drawn then, and seated again.
	put := standardCloudBotNamed("put", "Put")
	put.Floor = "3"
	withCloudBots(d, append([]CloudBot{put}, someBots(2, 0)...)...)
	shown, again := seatsShown(d, "cloud-put", "cloud-b0"), seatsNow(d)
	for _, seats := range []map[string]Seat{shown, again} {
		if seats["cloud-put"] != (Seat{2, room - 2}) || seats["cloud-b0"] != (Seat{3, 0}) {
			t.Errorf("the bot put on 3F is at %v and the first at %v: want the one put there at the first's desk, that one where there's room",
				seats["cloud-put"], seats["cloud-b0"])
		}
	}
	run := fmt.Sprintf("w%d", room-2)
	runsIn(d, "/work/web", 4000, run)
	seats := seatsNow(d)
	if seats[run] != (Seat{2, room - 1}) || seats["cloud-b1"] != (Seat{3, 1}) {
		t.Errorf("a run its folder's rule puts on 3F is at %v and the other bot at %v: want the run at the bot's desk, the bot where there's room",
			seats[run], seats["cloud-b1"])
	}
	if later := seatsNow(d); !maps.Equal(later, seats) {
		t.Errorf("seated again, %v, was %v", later, seats)
	}
}

// A bot sent on from its full floor takes no room there: one put on it has a
// desk past a run there only for want of room.
func TestABotSentOnFromItsFullFloorLeavesItsRoomToAnyonePutThere(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	d.config.FloorRules = checkedFloorRules([]FloorRule{{"/work/api", 1, false}})
	runsIn(d, "/work/api", 1000, numbered("h", room-1)...)
	runsIn(d, "/work/other", 3000, "x0")
	if _, floors := seating(d); len(floors[1]) != room || floors[1][room-1] != "x0" {
		t.Fatalf("2F is %v, want a run with no floor of its own at its last desk", floors[1])
	}
	withCloudBots(d, standardCloudBotNamed("kept", "Kept"))
	if _, floors := seating(d); !slices.Equal(floors[2], []string{"cloud-kept"}) {
		t.Fatalf("3F is %v, want the bot, 2F being full", floors[2])
	}
	running(d, 1)
	if code, text := d.Perform("move", "s0", url.Values{"floor": {"1"}}); code != http.StatusNoContent {
		t.Fatalf("put on 2F: %d %s, want room past the run there for want of room", code, text)
	}
	if seats := seatsNow(d); seats["s0"] != (Seat{1, room - 1}) || seats["cloud-kept"] != (Seat{2, 0}) {
		t.Errorf("the agent is at %v and the bot at %v: want the agent at the run's desk, the bot at its own", seats["s0"], seats["cloud-kept"])
	}
}

// Bots their folders' rules send elsewhere go there as desks come free: two
// each on the other's full floor change places; one going home frees a desk
// for another, and that one for a third, new there, all as it's seated once.
func TestBotsSentElsewhereGoHomeAsDesksComeFree(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	x, y, fresh := standardCloudBotNamed("x", "X"), standardCloudBotNamed("y", "Y"), standardCloudBotNamed("new", "New")
	rules := func(xFloor, yFloor int) floorRuleList {
		return checkedFloorRules([]FloorRule{{"/work/api", 1, false}, {"/work/web", 2, false}, {"/work/five", 4, false},
			{x.folder(), xFloor, false}, {y.folder(), yFloor, false}})
	}
	d.config.FloorRules = rules(1, 2)
	runsIn(d, "/work/api", 1000, numbered("h", room-1)...)
	runsIn(d, "/work/web", 2000, numbered("w", room-1)...)
	runsIn(d, "/work/five", 3000, numbered("v", room)...)
	seating(d)
	withCloudBots(d, y, x)
	at := func(when string, want map[string]Seat) {
		t.Helper()
		seats := seatsNow(d)
		for id, seat := range want {
			if seats[id] != seat {
				t.Errorf("%s: %s is at %v, want %v", when, id, seats[id], seat)
			}
		}
	}
	at("each on its floor", map[string]Seat{"cloud-x": {1, room - 1}, "cloud-y": {2, room - 1}})
	// Each on the other's floor, both full: they change places.
	d.config.FloorRules = rules(2, 1)
	at("each sent to the other's floor", map[string]Seat{"cloud-x": {2, room - 1}, "cloud-y": {1, room - 1}})
	// X's floor 5F, full, Y's 3F, where X sits: nobody moves till 5F has a desk. Then X goes, Y to its desk, and a new
	// bot to Y's.
	d.config.FloorRules = rules(4, 2)
	at("with 5F full", map[string]Seat{"cloud-x": {2, room - 1}, "cloud-y": {1, room - 1}})
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionEnd", "session_id": "v4"}, Claude, "headless:v4", "", 4000)
	withCloudBots(d, y, x, fresh)
	// Seated by the deck itself as the bots change: as drawn then, and seated again.
	want := map[string]Seat{"cloud-x": {4, 4}, "cloud-y": {2, room - 1}, "cloud-new": {1, room - 1}}
	shown, again := seatsShown(d, "cloud-x", "cloud-y", "cloud-new"), seatsNow(d)
	for id, seat := range want {
		if shown[id] != seat || again[id] != seat {
			t.Errorf("with a desk on 5F, %s is at %v as drawn and %v seated again, want %v", id, shown[id], again[id], seat)
		}
	}
}

func TestAFoldersRuleGivesABotItsFloorAndOneYouPutThereStays(t *testing.T) {
	d := newTestDeck(t)
	d.config.FloorRules = checkedFloorRules([]FloorRule{{"/cloud", 4, false}, {"/cloud/Scout", 0, true}})
	put := standardCloudBotNamed("put", "Put")
	put.Floor = "3"
	withCloudBots(d, standardCloudBotNamed("amber", "Amber"), standardCloudBotNamed("scout", "Scout"), put)
	keys, floors := seating(d)
	if !slices.Equal(floors[4], []string{"cloud-amber"}) {
		t.Errorf("5F is %v, want the bot the bots' folder's rule puts there, before where new bots go", floors[4])
	}
	if !slices.Equal(keys, []string{"cloud-scout"}) {
		t.Errorf("the keys are %v, want the bot its own folder's rule puts on the ground floor", keys)
	}
	if !slices.Equal(floors[2], []string{"cloud-put"}) || len(floors[1]) != 0 {
		t.Errorf("3F is %v and 2F %v: the one put on 3F stays there, whatever the rules say", floors[2], floors[1])
	}
}

// RUBY, the owner's Mini Manager, is on the ground floor because it was put
// there, never by its name.
func TestTheMiniManagerOnTheGroundFloorTakesAKeyAndLeavesTheBotsTheirRoom(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	d.config.BotFloor = 2
	manager := standardCloudBotNamed("manager", "Mini Manager")
	manager.Floor = "ground"
	// Past the 9 bots there could be, the manager among them.
	withCloudBots(d, append([]CloudBot{manager}, someBots(room, 0)...)...)
	if len(d.config.CloudBots) != room+1 {
		t.Fatalf("kept %d of %d bots", len(d.config.CloudBots), room+1)
	}
	keys, floors := seating(d)
	if !slices.Equal(keys, []string{"cloud-manager"}) {
		t.Errorf("the keys are %v, want the manager's", keys)
	}
	if !slices.Equal(floors[2], botSessions(someBots(room, 0))) {
		t.Errorf("3F is %v: every desk on the bots' floor is theirs", floors[2])
	}
}

// A bot never ends, so it comes back to the desk it had, as an agent still
// running after a restart does: the app starting on what it saved, with no
// sessions to put back.
func TestABotGetsTheDeskItHadBackAfterARestart(t *testing.T) {
	isolate(t)
	config := `{"layout": {"keys": 6}, "cloudBots": [{"id": "manager", "name": "Mini Manager", "floor": "ground"}, {"id": "up", "name": "Up", "floor": "3"}]}`
	saved := `{"savedAt": 0, "sessions": [], "keys": {"cloud-manager": 4, "gone": 1}, "seats": {"cloud-up": {"floor": 2, "desk": 5}}}`
	if os.WriteFile(configFile, []byte(config), 0o600) != nil || os.WriteFile(sessionsFile, []byte(saved), 0o600) != nil {
		t.Fatal("can't write the saved files")
	}
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	// config.json's fixed count is read but not kept (as the Mac's Layout): six keys here, to see each key's place.
	d.mu.Lock()
	d.config.Layout.Keys = 6
	d.mu.Unlock()
	keys, _ := seating(d)
	if !slices.Equal(keys, []string{"", "", "", "", "cloud-manager", ""}) {
		t.Errorf("the keys are %v, want the manager back at the fifth", keys)
	}
	d.mu.Lock()
	seated, _ := d.seatedBuilding(nil)
	d.mu.Unlock()
	if len(seated[2]) != 1 || seated[2][0].desk != 5 {
		t.Errorf("3F is %v, want the bot back at its sixth desk", seated[2])
	}
}

// A bot still "upstairs", from before bots could sit on any floor, stays as
// the app starts on the floor of the desk saved for it, at that desk, and the
// settings are saved so. One with no desk saved, or with nothing saved at
// all, goes where there's room among the bots.
func TestABotThatWasUpstairsStaysOnTheFloorItSatOnAsTheAppStarts(t *testing.T) {
	start := func(saved string) *Deck {
		isolate(t)
		config := `{"cloudBots": [{"id": "up", "name": "Up", "floor": "upstairs"}, {"id": "other", "name": "Other", "floor": "upstairs"}]}`
		if os.WriteFile(configFile, []byte(config), 0o600) != nil || saved != "" && os.WriteFile(sessionsFile, []byte(saved), 0o600) != nil {
			t.Fatal("can't write the saved files")
		}
		d := startDeck(t)
		d.summaryProviders.set(map[summaryProvider]providerState{})
		return d
	}
	floors := func(bots []CloudBot) []string {
		kept := []string{}
		for _, bot := range bots {
			kept = append(kept, bot.Floor)
		}
		return kept
	}
	d := start(`{"savedAt": 0, "sessions": [], "keys": {}, "seats": {"cloud-up": {"floor": 2, "desk": 4}}}`)
	if got := floors(d.config.CloudBots); !slices.Equal(got, []string{"3", "2"}) {
		t.Errorf("the bots' floors are %v, want 3F's for the one that sat there, 2F's for the other", got)
	}
	if got := floors(loadConfig().CloudBots); !slices.Equal(got, []string{"3", "2"}) {
		t.Errorf("saved as %v", got)
	}
	d.mu.Lock()
	seated, _ := d.seatedBuilding(nil)
	d.mu.Unlock()
	if len(seated[2]) != 1 || seated[2][0].person.ID != "cloud-up" || seated[2][0].desk != 4 {
		t.Errorf("3F is %v, want the bot back at the desk it had", seated[2])
	}
	d = start("")
	if got := floors(d.config.CloudBots); !slices.Equal(got, []string{"2", "2"}) {
		t.Errorf("with nothing saved, the bots' floors are %v, want where there's room", got)
	}
	// At 2F's eleventh desk, from when floors had twelve, with 2F's nine taken: moved first (see onFloorDesks), to
	// 3F, then kept there.
	full := savedDeck{Seats: map[string]Seat{"cloud-up": {Floor: 1, Desk: 10}}}
	for desk := range desksPerFloor {
		full.Seats[fmt.Sprintf("t%d", desk)] = Seat{Floor: 1, Desk: desk}
	}
	data, _ := json.Marshal(full)
	d = start(string(data))
	if got := floors(d.config.CloudBots); got[0] != "3" {
		t.Errorf("at a desk past the ninth on a full 2F, the bot's floor is %q, want 3F's, where it was moved", got[0])
	}
}

// Put where it already sits, where new bots go, a bot stays there from then
// on, at the desk it has, as new bots go elsewhere.
func TestABotPutWhereItSitsStaysThereAtItsDesk(t *testing.T) {
	d := newTestDeck(t)
	d.config.BotFloor = 2
	three := someBots(3, 0)
	withCloudBots(d, three...)
	seating(d)
	// The one between them gone: the last keeps its desk, the third, with a free one before it.
	withCloudBots(d, three[0], three[2])
	seating(d)
	if code, text := d.Perform("move", "cloud-b2", url.Values{"floor": {"2"}}); code != http.StatusNoContent {
		t.Fatalf("put on 3F, where it sits: %d %s", code, text)
	}
	if floor := d.config.CloudBots[1].Floor; floor != "3" {
		t.Errorf("its floor is %q, want 3F's", floor)
	}
	d.config.BotFloor = 4
	_, floors := seating(d)
	d.mu.Lock()
	seated, _ := d.seatedBuilding(nil)
	d.mu.Unlock()
	if len(seated[2]) != 1 || seated[2][0].person.ID != "cloud-b2" || seated[2][0].desk != 2 {
		t.Errorf("3F is %v, want the bot put there at the desk it had", seated[2])
	}
	if !slices.Equal(floors[4], []string{"cloud-b0"}) {
		t.Errorf("5F is %v, want the other, where new bots go now", floors[4])
	}
}

// A bot at a key, where new bots go on the ground floor, is put there though
// every key is taken: by itself among others.
func TestABotAtAKeyIsPutOnTheGroundFloorThoughEveryKeyIsTaken(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	d.config.BotFloor = 0
	running(d, 2)
	withCloudBots(d, standardCloudBotNamed("mini", "Mini Manager"))
	if keys, _ := seating(d); !slices.Equal(keys, []string{"s0", "s1", "cloud-mini"}) {
		t.Fatalf("the keys are %v", keys)
	}
	if code, text := d.Perform("move", "cloud-mini", url.Values{"floor": {"0"}}); code != http.StatusNoContent {
		t.Errorf("put on the ground floor, where it sits: %d %s", code, text)
	}
	if floor := d.config.CloudBots[0].Floor; floor != "ground" {
		t.Errorf("its floor is %q", floor)
	}
}

func TestABotWithNoDeskOnAnyFloorIsNamedForSetup(t *testing.T) {
	d := newTestDeck(t)
	room := floorRoom()
	d.config.Layout.Keys = 3
	// Every desk taken by bots put there: the three keys downstairs, and every floor above.
	var all []CloudBot
	for i := range 3 {
		bot := standardCloudBotNamed(fmt.Sprintf("g%d", i), fmt.Sprintf("G%d", i))
		bot.Floor = "ground"
		all = append(all, bot)
	}
	for floor := 1; floor < floorCount; floor++ {
		for i := range room {
			bot := standardCloudBotNamed(fmt.Sprintf("f%d-%d", floor, i), fmt.Sprintf("F%d %d", floor, i))
			bot.Floor = cloudFloorAt(floor)
			all = append(all, bot)
		}
	}
	botsAsGiven(d, all...)
	state := func() map[string]any {
		var got struct{ CloudBots map[string]any }
		w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.CloudBots
	}
	if noRoom := state()["noRoom"]; fmt.Sprint(noRoom) != "[]" {
		t.Errorf("with a desk for each, no room for %v", noRoom)
	}
	botsAsGiven(d, append(all, standardCloudBotNamed("late", "Late"))...)
	if noRoom := state()["noRoom"]; fmt.Sprint(noRoom) != "[Late]" {
		t.Errorf("one more, no room for %v, want Late", noRoom)
	}
}

// MARK: Setup

func TestSetupSetsTheFloorNewBotsGoToAndABotsFloorBackToThere(t *testing.T) {
	d := newTestDeck(t)
	put := standardCloudBotNamed("put", "Put")
	put.Floor = "4"
	withCloudBots(d, put)
	override := func(body string) (int, map[string]any) {
		w := serve(t, d, call{method: "POST", target: "/setup/bots/override", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
		var got struct{ CloudBots map[string]any }
		json.Unmarshal(w.Body.Bytes(), &got)
		return w.Code, got.CloudBots
	}
	if code, bots := override(`{"botFloor": 0}`); code != 200 || d.config.BotFloor != 0 || bots["botFloor"] != 0.0 {
		t.Errorf("new bots to the ground floor: %d, %d, setup says %v", code, d.config.BotFloor, bots["botFloor"])
	}
	if code, _ := override(`{"botFloor": 5}`); code != 200 || d.config.BotFloor != 5 {
		t.Errorf("new bots to 6F: %d, %d", code, d.config.BotFloor)
	}
	for _, bad := range []string{`{"botFloor": 6}`, `{"botFloor": -1}`} {
		if code, _ := override(bad); code != 200 || d.config.BotFloor != 5 {
			t.Errorf("%s: %d, new bots on %d, want it left as it was", bad, code, d.config.BotFloor)
		}
	}
	if code, _ := override(`{}`); code != 400 {
		t.Errorf("nothing to set gave %d", code)
	}
	if loadConfig().BotFloor != 5 {
		t.Errorf("not kept: %d", loadConfig().BotFloor)
	}
	// A bot put on a floor, sent back to where new bots go.
	if code, _ := override(`{"id": "put", "floor": ""}`); code != 200 || d.config.CloudBots[0].Floor != "" {
		t.Errorf("back to where new bots go: %d, its floor %q", code, d.config.CloudBots[0].Floor)
	}
	if code, _ := override(`{"id": "put", "floor": "upstairs"}`); code != 200 || d.config.CloudBots[0].Floor != "" {
		t.Errorf("upstairs, which is no floor now: %d, its floor %q", code, d.config.CloudBots[0].Floor)
	}
}
