package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Who sits on which floor: those put on one, by the office or by their
// folder's rule, and everyone else where there's room, each keeping their desk.

func person(id string, floor int, had *Seat) upstairsPerson[string] {
	return upstairsPerson[string]{person: id, id: id, floor: floor, had: had}
}

func namesOn(floor []seatedPerson[string]) []string {
	names := []string{}
	for _, one := range floor {
		names = append(names, one.person)
	}
	return names
}

func TestThosePutOnAFloorSitThereBeforeAnyoneElse(t *testing.T) {
	floors, over, seats := seatBuilding([]upstairsPerson[string]{person("nova", 3, nil)},
		[]upstairsPerson[string]{person("bot", -1, nil), person("run", -1, nil)})
	if got := namesOn(floors[3]); !slices.Equal(got, []string{"nova"}) {
		t.Errorf("4F is %v", got)
	}
	if got := namesOn(floors[1]); !slices.Equal(got, []string{"bot", "run"}) {
		t.Errorf("2F is %v", got)
	}
	if seats["nova"] != (Seat{3, 0}) || over != 0 {
		t.Errorf("nova sits at %v, %d over", seats["nova"], over)
	}
}

func TestEveryoneKeepsTheDeskTheyHadWhenSomeoneLeaves(t *testing.T) {
	a, c := &Seat{1, 0}, &Seat{1, 2}
	floors, _, _ := seatBuilding(nil, []upstairsPerson[string]{person("a", -1, a), person("c", -1, c)})
	if len(floors[1]) != 2 || floors[1][0].desk != 0 || floors[1][1].desk != 2 {
		t.Errorf("2F is %+v, want a and c where they were", floors[1])
	}
	// A newcomer takes the gap, not someone's desk.
	_, _, seats := seatBuilding(nil, []upstairsPerson[string]{person("a", -1, a), person("c", -1, c), person("d", -1, nil)})
	if seats["d"] != (Seat{1, 1}) || seats["c"] != (Seat{1, 2}) {
		t.Errorf("the seats are %v", seats)
	}
}

func TestSomeonePutOnAFloorTakesTheirOwnDeskBackFromAnyoneSeatedThereSince(t *testing.T) {
	_, _, seats := seatBuilding([]upstairsPerson[string]{person("nova", 1, &Seat{1, 4})},
		[]upstairsPerson[string]{person("run", -1, &Seat{1, 4})})
	if seats["nova"] != (Seat{1, 4}) || seats["run"] != (Seat{1, 0}) {
		t.Errorf("the seats are %v", seats)
	}
}

func TestAPutFloorThatIsFullSendsTheRestWhereThereIsRoom(t *testing.T) {
	var crowd []upstairsPerson[string]
	for i := range desksPerFloor + 1 {
		crowd = append(crowd, person("p"+strings.Repeat("x", i), 2, nil))
	}
	floors, _, _ := seatBuilding(crowd, nil)
	if len(floors[2]) != desksPerFloor || len(floors[1]) != 1 {
		t.Errorf("3F has %d and 2F %d, want %d and 1", len(floors[2]), len(floors[1]), desksPerFloor)
	}
}

func TestPastTheTopFloorThereIsNoDesk(t *testing.T) {
	var rest []upstairsPerson[string]
	everyDesk := (floorCount - 1) * desksPerFloor
	for i := range everyDesk + 1 {
		rest = append(rest, person(strings.Repeat("h", i+1), -1, nil))
	}
	_, over, seats := seatBuilding(nil, rest)
	if _, ok := seats[strings.Repeat("h", everyDesk+1)]; over != 1 || ok {
		t.Errorf("%d over", over)
	}
}

func TestARuleSendsAFolderAndWhatIsInItToItsFloor(t *testing.T) {
	rules := []FloorRule{{"/work/atlas", 2, false}, {"/work", 4, false}}
	for _, c := range []struct {
		project string
		floor   int
		ok      bool
	}{
		{"/work/atlas", 2, true}, {"/work/atlas/app", 2, true}, {"/work/other", 4, true},
		{"/elsewhere", 0, false}, {"", 0, false},
	} {
		if floor, ok := ruleFloor(c.project, rules); floor != c.floor || ok != c.ok {
			t.Errorf("%q goes to %d %v, want %d %v", c.project, floor, ok, c.floor, c.ok)
		}
	}
	if _, ok := ruleFloor("/work/atlasx", rules[:1]); ok {
		t.Error("a folder named alike went by the rule")
	}
	if floor, _ := floorOfProject("/work/atlas", 0, rules); floor != 0 {
		t.Errorf("brought down, it's on %d", floor)
	}
}

func TestRulesAreCheckedAsTheyAreKept(t *testing.T) {
	rules := checkedFloorRules([]FloorRule{{" /work/atlas/ ", 2, false}, {"relative", 1, false}, {"/work/a", 9, false}, {"/work/atlas", 3, false}})
	if !slices.Equal(rules, floorRuleList{{"/work/atlas", 3, false}}) {
		t.Errorf("the rules are %v", rules)
	}
	sent, ok := floorRulesSent([]byte(`{"rules": [{"folder": "/work/atlas", "floor": 2}, {"folder": 3}]}`))
	if !ok || !slices.Equal(sent, floorRuleList{{"/work/atlas", 2, false}}) {
		t.Errorf("setup sent %v", sent)
	}
	if _, ok := floorRulesSent([]byte(`{"floors": []}`)); ok {
		t.Error("a page from before rules sent some")
	}
}

func TestARuleSendsASessionUpstairsAndItsPinnedKeyStaysEmpty(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	pin := "/work/p1"
	d.config.Pins[1] = &pin
	d.config.FloorRules = floorRuleList{{"/work/p1", 3, false}}
	running(d, 3)
	d.mu.Lock()
	keys, overflow := d.layout()
	floors, _ := d.building(overflow)
	d.mu.Unlock()
	if keys[1].session != nil || keys[1].pin == nil {
		t.Errorf("the pinned key is %+v, want it kept, empty", keys[1])
	}
	if len(floors[3]) != 1 || floors[3][0].ID != "s1" {
		t.Errorf("4F is %v, want s1", floors[3])
	}
	// Brought down from the office, it goes before its rule.
	d.mu.Lock()
	d.floorOf["s1"] = 0
	keys, _ = d.layout()
	d.mu.Unlock()
	if keys[1].session == nil || keys[1].session.ID != "s1" {
		t.Errorf("brought down, its key is %+v", keys[1])
	}
}

func TestACloudBotPutOnAFloorSitsThere(t *testing.T) {
	d := newTestDeck(t)
	bot := standardCloudBotNamed("mini", "Smolcorp mini")
	bot.Floor = "5"
	withCloudBots(d, bot)
	if desks := floorDesks(t, d, 4); len(desks) != 1 || desks[0]["id"] != "cloud-mini" {
		t.Errorf("5F is %v", desks)
	}
	if bot.floorIndex() != 4 || cloudFloorAt(4) != "5" || cloudFloorAt(0) != "ground" {
		t.Errorf("5F is floor %d", bot.floorIndex())
	}
}

func TestSessionsAreBackOnTheirFloorsAfterARestart(t *testing.T) {
	d := newTestDeck(t)
	running(d, 2)
	d.mu.Lock()
	d.floorOf["s1"] = 2
	d.seatOf["s1"] = Seat{2, 5}
	d.mu.Unlock()
	savedFrom(t, d, 1000)

	saved := paneRunsAgent
	t.Cleanup(func() { paneRunsAgent = saved })
	paneRunsAgent = func(commandTab, Agent) bool { return true }
	after := startDeck(t)
	deadline := time.Now().Add(2 * time.Second)
	for {
		after.mu.Lock()
		count := len(after.sessions.All())
		after.mu.Unlock()
		if count == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	after.mu.Lock()
	defer after.mu.Unlock()
	_, overflow := after.layout()
	seated, _ := after.seatedBuilding(overflow)
	if len(seated[2]) != 1 || seated[2][0].person.ID != "s1" || seated[2][0].desk != 5 {
		t.Errorf("after a restart 3F is %+v", seated[2])
	}
}

func TestTheSavedDeckKeepsWhereEachSessionSitsUpstairs(t *testing.T) {
	d := newTestDeck(t)
	running(d, 1)
	d.mu.Lock()
	d.floorOf["s0"] = 2
	d.seatOf["s0"] = Seat{2, 5}
	d.mu.Unlock()
	d.saveSessions()
	data, err := os.ReadFile(d.sessionsFile)
	var saved savedDeck
	if err != nil || json.Unmarshal(data, &saved) != nil || saved.Floors["s0"] != 2 || saved.Seats["s0"] != (Seat{2, 5}) {
		t.Errorf("saved %s", data)
	}
}

func TestTheOfficePutsSomeoneOnAnotherFloorAndBringsThemBack(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	running(d, 3)
	if code, text := d.Perform("move", "s1", url.Values{"floor": {"3"}}); code != http.StatusNoContent {
		t.Fatalf("moving up gave %d %s", code, text)
	}
	if desks := floorDesks(t, d, 3); len(desks) != 1 || desks[0]["id"] != "s1" || desks[0]["drivable"] != true {
		t.Errorf("4F is %v", desks)
	}
	d.mu.Lock()
	keys, _ := d.layout()
	d.mu.Unlock()
	if keys[1].session != nil && keys[1].session.ID == "s1" {
		t.Error("moved upstairs, it kept its key")
	}
	if len(d.config.FloorRules) != 0 {
		t.Errorf("a move alone made the rules %v", d.config.FloorRules)
	}
	// Someone else takes the free key meanwhile: down, there's none for it.
	running(d, 4)
	floorDesks(t, d, 0) // the deck lays out on every change: s3 takes the free key
	if code, text := d.Perform("move", "s1", url.Values{"floor": {"0"}}); code != http.StatusConflict || text != "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room." {
		t.Errorf("moving down to a full ground floor gave %d %s", code, text)
	}
	if desks := floorDesks(t, d, 3); len(desks) != 1 || desks[0]["id"] != "s1" {
		t.Errorf("refused, 4F is %v, want s1 still there", desks)
	}
	if code, _ := d.Perform("move", "s1", url.Values{"floor": {"9"}}); code != http.StatusBadRequest {
		t.Errorf("a floor there isn't gave %d", code)
	}
}

func TestAMoveRememberedIsItsFoldersRule(t *testing.T) {
	d := newTestDeck(t)
	running(d, 2)
	if code, text := d.Perform("move", "s0", url.Values{"floor": {"2"}, "remember": {"1"}}); code != http.StatusNoContent {
		t.Fatalf("gave %d %s", code, text)
	}
	if !slices.Equal(d.config.FloorRules, floorRuleList{{"/work/p0", 2, true}}) {
		t.Errorf("the rules are %v", d.config.FloorRules)
	}
	if loaded := loadConfig(); !slices.Equal(loaded.FloorRules, floorRuleList{{"/work/p0", 2, true}}) {
		t.Errorf("saved, the rules are %v", loaded.FloorRules)
	}
}

func TestAFloorWithNinePutThereIsFull(t *testing.T) {
	d := newTestDeck(t)
	d.config.FloorRules = floorRuleList{{"/work", 1, false}}
	running(d, desksPerFloor+1)
	last := fmt.Sprintf("s%d", desksPerFloor)
	d.mu.Lock()
	d.floorOf[last] = 0
	d.mu.Unlock()
	if code, text := d.Perform("move", last, url.Values{"floor": {"1"}}); code != http.StatusConflict || !strings.Contains(text, "full") {
		t.Errorf("a full floor gave %d %s", code, text)
	}
}

func TestACloudBotMovedKeepsItsFloorInSetup(t *testing.T) {
	d := newTestDeck(t)
	withCloudBots(d, standardCloudBotNamed("mini", "Smolcorp mini"))
	if code, text := d.Perform("move", "cloud-mini", url.Values{"floor": {"4"}}); code != http.StatusNoContent {
		t.Fatalf("gave %d %s", code, text)
	}
	if d.config.CloudBots[0].Floor != "5" {
		t.Errorf("its floor is %q", d.config.CloudBots[0].Floor)
	}
	if desks := floorDesks(t, d, 4); len(desks) != 1 || desks[0]["id"] != "cloud-mini" {
		t.Errorf("5F is %v", desks)
	}
}

func TestTheCogOpensSetupFromTheOfficesOwnWindowOnly(t *testing.T) {
	d := newTestDeck(t)
	opened := 0
	saved := openSetupPage
	t.Cleanup(func() { openSetupPage = saved })
	openSetupPage = func(*Deck) error { opened++; return nil }
	phone := pair(t, d)
	asPhone := serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + phone, host: deckOnWiFi, from: aPhone})
	if asPhone.Code != http.StatusForbidden || opened != 0 {
		t.Errorf("from a phone: %d, opened %d times", asPhone.Code, opened)
	}
	asWindow := serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	if asWindow.Code != http.StatusNoContent || opened != 1 {
		t.Errorf("from the window: %d, opened %d times", asWindow.Code, opened)
	}
}

func TestTheCogSaysWhenTheBrowserWouldNotOpen(t *testing.T) {
	d := newTestDeck(t)
	saved := openURL
	t.Cleanup(func() { openURL = saved })
	openURL = func(string) error { return errors.New("no xdg-open") }
	got := serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusBadGateway || !strings.Contains(got.Body.String(), "no xdg-open") ||
		!strings.Contains(got.Body.String(), setupAdvice(runtime.GOOS)) {
		t.Errorf("the browser failed: %d %q, want %d saying why and the way in", got.Code, got.Body.String(), http.StatusBadGateway)
	}
	// The office page reads that answer and flashes it, instead of opening the phone's settings.
	page, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "res.status === 502) return flash(") {
		t.Error("the page does not show the failed answer")
	}
}

func TestSetupAdviceMatchesThePlatform(t *testing.T) {
	for goos, want := range map[string]string{
		"linux":   "choose Open Setup… from Smolcorp's tray icon once your browser opens links; with no tray, for a new link run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again",
		"windows": "choose Open Setup… from Smolcorp's tray icon once your browser opens links, or quit Smolcorp from its tray icon and start it again for a new link",
		"darwin":  "Settings…",
	} {
		if got := setupAdvice(goos); !strings.Contains(got, want) {
			t.Errorf("%s advice is %q, want it to contain %q", goos, got, want)
		}
	}
	if got := setupAdvice("darwin"); !strings.Contains(got, "AgentDeck --setup --open") || strings.Contains(got, "smolcorp setup") {
		t.Errorf("the Mac advice %q should name the app binary's --setup --open, not the smolcorp command", got)
	}
}

func TestFloorRulesWorkWithWindowsFolders(t *testing.T) {
	rules := checkedFloorRules([]FloorRule{{`C:\Users\me\proj\`, 2, false}, {`\\server\share\work`, 3, false}, {"~/notes", 4, false}, {`relative\proj`, 1, false}})
	if len(rules) != 3 || rules[0].Folder != `C:\Users\me\proj` || rules[1].Folder != `\\server\share\work` ||
		rules[2].Folder != homeDir+"/notes" {
		t.Fatalf("the rules are %v", rules)
	}
	for _, c := range []struct {
		project string
		floor   int
		ok      bool
	}{
		{`C:\Users\me\proj`, 2, true}, {`C:\Users\me\proj\app`, 2, true}, {`c:\users\ME\Proj\app`, 2, true},
		{`C:/Users/me/proj/web`, 2, true}, {`C:\Users\me\project`, 0, false}, {`\\server\share\work\x`, 3, true},
	} {
		if floor, ok := ruleFloor(c.project, rules); floor != c.floor || ok != c.ok {
			t.Errorf("%s goes to %d %v, want %d %v", c.project, floor, ok, c.floor, c.ok)
		}
	}
	if _, ok := ruleFloor("/Users/me/proj", rules); ok {
		t.Error("a Mac's path went by a Windows rule")
	}
}

func TestARememberedRuleIsItsFolderAloneWhileSetupsHoldsWhatsInIt(t *testing.T) {
	rules := []FloorRule{{"/Users/me/Projects", 3, true}, {"/work", 2, false}}
	if floor, ok := ruleFloor("/Users/me/Projects", rules); !ok || floor != 3 {
		t.Errorf("the folder itself goes to %d %v", floor, ok)
	}
	// The GM in ~/Projects remembered upstairs takes none of the projects in it with it.
	if _, ok := ruleFloor("/Users/me/Projects/app", rules); ok {
		t.Error("a project inside a remembered folder went by its rule")
	}
	if floor, ok := ruleFloor("/work/app", rules); !ok || floor != 2 {
		t.Errorf("a project inside a setup rule's folder goes to %d %v", floor, ok)
	}
	if !sameFolder(`C:\Users\me\proj\`, `c:/users/me/proj`) || sameFolder("/a/b", "/a") {
		t.Error("sameFolder")
	}
}

func TestSomeoneMovedOntoAFloorTakesAFreeDeskAndBumpsNobody(t *testing.T) {
	_, _, seats := seatBuilding([]upstairsPerson[string]{person("nova", 2, nil)}, []upstairsPerson[string]{person("run", -1, &Seat{2, 0})})
	if seats["run"] != (Seat{2, 0}) || seats["nova"] != (Seat{2, 1}) {
		t.Errorf("the seats are %v", seats)
	}
}

func TestOnAFullFloorSomeoneThereForWantOfRoomMakesWay(t *testing.T) {
	var runs []upstairsPerson[string]
	for i := range desksPerFloor {
		runs = append(runs, person(fmt.Sprintf("r%d", i), -1, &Seat{2, i}))
	}
	_, _, seats := seatBuilding([]upstairsPerson[string]{person("nova", 2, nil)}, runs)
	if seats["nova"] != (Seat{2, 0}) || seats["r0"] != (Seat{1, 0}) || seats["r1"] != (Seat{2, 1}) {
		t.Errorf("the seats are %v", seats)
	}
}

func TestACloudBotHasNoKeyWhenEveryDeskIsTaken(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	running(d, 3)
	withCloudBots(d, standardCloudBotNamed("mini", "Smolcorp mini"))
	if code, text := d.Perform("move", "cloud-mini", url.Values{"floor": {"0"}}); code != http.StatusConflict || text != "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room." {
		t.Errorf("down to a full ground floor gave %d %s", code, text)
	}
	if d.config.CloudBots[0].Floor != "" {
		t.Errorf("refused, its floor is %q, want where new bots go", d.config.CloudBots[0].Floor)
	}
}

func TestARememberedRuleBesideSetupsOnTheSameFolderLeavesItsFoldersWhereTheyWere(t *testing.T) {
	rules := checkedFloorRules([]FloorRule{{"/work", 2, false}, {"/work", 4, true}})
	if len(rules) != 2 {
		t.Fatalf("the rules are %v, want both kinds kept", rules)
	}
	if floor, _ := ruleFloor("/work", rules); floor != 4 {
		t.Errorf("the folder itself goes to %d, want 4 by its own rule", floor)
	}
	if floor, ok := ruleFloor("/work/app", rules); !ok || floor != 2 {
		t.Errorf("a folder in it goes to %d %v, want 2 by setup's rule", floor, ok)
	}
}

func TestWindowsRulesForOneFolderWrittenTwoWaysAreOneAndTheLaterWins(t *testing.T) {
	rules := checkedFloorRules([]FloorRule{{`C:\Work\App\`, 2, false}, {`c:/work/app`, 3, false}})
	if len(rules) != 1 || rules[0].Floor != 3 {
		t.Errorf("the rules are %v, want the later alone", rules)
	}
	if floor, _ := ruleFloor(`C:\Work\App\x`, []FloorRule{{`C:\Work\App`, 2, false}, {`c:\work\app`, 3, false}}); floor != 3 {
		t.Errorf("a tie went to %d, want the later rule's 3", floor)
	}
}

// Going home, a bot that yields never takes a desk kept for someone put on a
// floor and sent on from it, though the first one placed is a bot that yields
// for the floor it sits on; seated again as they now sit, nobody moves.
func TestGoingHomeABotTakesNoDeskKeptForSomeoneSentOn(t *testing.T) {
	yielding := func(p upstairsPerson[string]) upstairsPerson[string] { p.yields = true; return p }
	placed := []upstairsPerson[string]{
		yielding(person("A", 2, &Seat{2, 0})), // a bot for 3F, at 3F's first desk
		yielding(person("P", 1, &Seat{2, 1})), // a bot for 2F, at 3F's second
		person("H", 4, &Seat{1, 8}),           // put on 5F, sent on to 2F's last desk
	}
	for i := 0; i < desksPerFloor; i++ {
		placed = append(placed, person(fmt.Sprintf("f%d", i), 4, &Seat{4, i})) // 5F full
	}
	var rest []upstairsPerson[string]
	for i := 0; i < 8; i++ {
		rest = append(rest, person(fmt.Sprintf("r%d", i), -1, &Seat{1, i}))
	}
	_, _, seats := seatBuilding(placed, rest)
	if seats["H"] != (Seat{1, 8}) || seats["P"] != (Seat{2, 1}) {
		t.Fatalf("H is at %v and P at %v, want H at 2F's last desk and P at 3F's second", seats["H"], seats["P"])
	}
	for i := range placed {
		s := seats[placed[i].id]
		placed[i].had = &s
	}
	for i := range rest {
		s := seats[rest[i].id]
		rest[i].had = &s
	}
	_, _, again := seatBuilding(placed, rest)
	for id, seat := range seats {
		if again[id] != seat {
			t.Errorf("seated again, %s moved from %v to %v", id, seat, again[id])
		}
	}
}
