package desktop

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
)

// The Grok Bot app's bots, found as the Swift engine finds them (BotDiscoveryTests.swift).
var (
	grokMini  = grokRow{ID: "aa11", Name: "Smolcorp Mini"}
	grokScout = grokRow{ID: "bb22", Name: "X scout"}
	grokGroup = grokRow{ID: "cc33", Name: "Bot Room", IsGroup: true}
)

func foundBot(name, botID string, pinned bool) CloudBot {
	bot := standardCloudBot()
	bot.ID, bot.Name, bot.Discovered, bot.Pinned = freeCloudID(grokRow{ID: botID, Name: name}, nil), name, true, pinned
	bot.Source = CloudSource{Kind: "grokBotApp", Bot: name, BotID: botID, Counts: "unprompted"}
	return bot
}

func botIDs(bots []CloudBot) (ids []string) {
	for _, bot := range bots {
		ids = append(ids, bot.ID)
	}
	return ids
}

func TestEachBotInTheGrokBotAppGetsADeskAsOnTheMac(t *testing.T) {
	bots, noRoom := discoverCloudBots([]grokRow{grokMini, grokGroup, grokScout}, nil, nil)
	if !slices.Equal(botIDs(bots), []string{"smolcorp-mini", "x-scout"}) || len(noRoom) != 0 {
		t.Fatalf("found %v, no room %v", botIDs(bots), noRoom)
	}
	for _, bot := range bots {
		if !bot.Discovered || bot.Pinned || bot.Floor != "" || bot.Source.Kind != "grokBotApp" || !bot.Permanent || bot.Outfit != "grok" {
			t.Errorf("found %+v", bot)
		}
	}
	// Yours is never found twice, by its id or its name, and is given its id there by its exact name.
	yours := standardCloudBot()
	yours.ID, yours.Name, yours.Source = "mini", "Mini", CloudSource{Kind: "grokBotApp", Bot: "Smolcorp Mini", Counts: "unprompted"}
	byID := standardCloudBot()
	byID.ID, byID.Name, byID.Source = "scout", "Scout", CloudSource{Kind: "grokBotApp", BotID: "BB22", Counts: "unprompted"}
	bots, _ = discoverCloudBots([]grokRow{grokMini, grokScout}, []CloudBot{yours, byID}, nil)
	if !slices.Equal(botIDs(bots), []string{"mini", "scout"}) || bots[0].Source.BotID != "aa11" {
		t.Errorf("yours: %+v", bots)
	}
	// Gone from the app: a found one goes unless pinned; yours never.
	gone, kept := foundBot("Old", "dd44", false), foundBot("Kept", "ee55", true)
	bots, _ = discoverCloudBots([]grokRow{grokMini}, []CloudBot{gone, kept, byID}, nil)
	if !slices.Equal(botIDs(bots), []string{"kept", "scout", "smolcorp-mini"}) {
		t.Errorf("gone: %v", botIDs(bots))
	}
	// Renamed there: a found one follows, unless you renamed it.
	renamed := foundBot("Smolcorp Mini", "aa11", true)
	renamed.Name = "Boss"
	bots, _ = discoverCloudBots([]grokRow{{ID: "aa11", Name: "Mini Two"}, {ID: "bb22", Name: "Scout Two"}},
		[]CloudBot{renamed, foundBot("X scout", "bb22", true)}, nil)
	if bots[0].Name != "Boss" || bots[1].Name != "Scout Two" || bots[1].Source.Bot != "Scout Two" {
		t.Errorf("renamed: %+v", bots)
	}
	// Hidden: neither found nor kept. A group that had a desk loses it.
	group := foundBot("Bot Room", "cc33", true)
	bots, _ = discoverCloudBots([]grokRow{grokMini, grokScout, grokGroup}, []CloudBot{foundBot("Smolcorp Mini", "aa11", false), group}, []string{"AA11"})
	if !slices.Equal(botIDs(bots), []string{"x-scout"}) {
		t.Errorf("hidden: %v", botIDs(bots))
	}
	// Past the most, no room, and setup is told.
	var full []CloudBot
	for i := range cloudBotsMost {
		bot := standardCloudBot()
		bot.ID, bot.Name = "b"+strconv.Itoa(i), "B"+strconv.Itoa(i)
		full = append(full, bot)
	}
	if bots, noRoom := discoverCloudBots([]grokRow{grokMini, grokGroup}, full, nil); len(bots) != cloudBotsMost || !slices.Equal(noRoom, []grokRow{grokMini}) {
		t.Errorf("full: %d, no room %v", len(bots), noRoom)
	}
	if id := freeCloudID(grokRow{ID: "aa11", Name: "Smolcorp Mini!"}, []string{"smolcorp-mini"}); id != "smolcorp-mini-2" {
		t.Errorf("id %q", id)
	}
	if id := freeCloudID(grokRow{ID: "abcdef0123", Name: "✨✨"}, nil); id != "grok-abcdef01" {
		t.Errorf("id %q", id)
	}
}

// lookAgain has the deck look in the Grok Bot app now, once any look under way (as setup's switches start) is
// over: done when one has ended that began after this was asked.
func lookAgain(d *Deck) {
	idle := func(since time.Time) bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return !d.mac.discovering && !d.mac.lastDiscovery.Before(since)
	}
	for !idle(time.Time{}) {
		time.Sleep(time.Millisecond)
	}
	asked := time.Now()
	d.discoverBots(false)
	for deadline := time.Now().Add(5 * time.Second); !idle(asked) && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
}

// Under the Mac app, the bots are found through the app at the deck's start, setup says so, and its switches work.
func TestUnderTheMacAppTheGrokBotAppsBotsAreFound(t *testing.T) {
	mac := withMac(t)
	mac.mu.Lock()
	mac.roster = &[]grokRow{grokMini, grokScout, grokGroup}
	mac.mu.Unlock()
	d := newTestDeck(t)
	lookAgain(d)
	d.mu.Lock()
	got := botIDs(d.config.CloudBots)
	d.mu.Unlock()
	if !slices.Equal(got, []string{"smolcorp-mini", "x-scout"}) {
		t.Fatalf("found %v", got)
	}
	read := func(body []byte) map[string]any {
		var s struct{ CloudBots map[string]any }
		if err := json.Unmarshal(body, &s); err != nil {
			t.Fatal(err)
		}
		return s.CloudBots
	}
	bots := read(serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer}).Body.Bytes())
	if bots["detects"] != true || bots["autoDetect"] != true || len(bots["bots"].([]any)) != 2 {
		t.Errorf("setup's bots: %v", bots)
	}
	// Hidden: taken off and kept out by its id there.
	override := func(body string) map[string]any {
		answer := serve(t, d, call{method: "POST", target: "/setup/bots/override", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
		if answer.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", body, answer.Code, answer.Body)
		}
		return read(answer.Body.Bytes())
	}
	bots = override(`{"id": "smolcorp-mini", "hidden": true}`)
	lookAgain(d)
	d.mu.Lock()
	got, hidden := botIDs(d.config.CloudBots), slices.Clone(d.config.HiddenBots)
	d.mu.Unlock()
	if !slices.Equal(got, []string{"x-scout"}) || !slices.Equal(hidden, []string{"aa11"}) {
		t.Errorf("hidden: %v %v", got, hidden)
	}
	// Shown again, it's found again.
	override(`{"show": "AA11"}`)
	lookAgain(d)
	d.mu.Lock()
	got = botIDs(d.config.CloudBots)
	d.mu.Unlock()
	if !slices.Equal(got, []string{"x-scout", "smolcorp-mini"}) {
		t.Errorf("shown: %v", got)
	}
	// Off: nothing is looked for, and a bot made there since gets no desk.
	bots = override(`{"autoDetect": false}`)
	mac.mu.Lock()
	*mac.roster = append(*mac.roster, grokRow{ID: "dd44", Name: "New"})
	mac.mu.Unlock()
	lookAgain(d)
	d.mu.Lock()
	got = botIDs(d.config.CloudBots)
	d.mu.Unlock()
	if bots["autoDetect"] != false || len(got) != 2 {
		t.Errorf("off: %v %v", bots["autoDetect"], got)
	}
}

// An app that can't be read changes nothing; nor does a deck without the Mac app look.
func TestAGrokBotAppThatCantBeReadChangesNothing(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.CloudBots = []CloudBot{foundBot("Old", "cc33", false)}
	d.mu.Unlock()
	lookAgain(d)
	d.mu.Lock()
	got := botIDs(d.config.CloudBots)
	d.mu.Unlock()
	if !slices.Equal(got, []string{"old"}) || !slices.Contains(mac.actions(), "bots-roster") {
		t.Errorf("unreadable: %v, asked %v", got, mac.actions())
	}
	host = nil
	elsewhere := newTestDeck(t)
	elsewhere.discoverBots(false)
	var state struct{ CloudBots map[string]any }
	answer := serve(t, elsewhere, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	if json.Unmarshal(answer.Body.Bytes(), &state) != nil || state.CloudBots["detects"] != false {
		t.Errorf("without the Mac app setup says %v", state.CloudBots)
	}
}

// Setup's list sent back: a found bot changed there is yours, pinned; one sent back unchanged isn't (BotDiscoveryTests).
func TestABotChangedInSetupIsPinnedAsOnTheMac(t *testing.T) {
	found, other := foundBot("Smolcorp Mini", "aa11", false), foundBot("X scout", "bb22", false)
	renamed, unchanged, added := found, other, foundBot("New", "cc33", false)
	renamed.Name, renamed.Discovered, unchanged.Discovered = "Boss", false, false
	bots := pinnedIfChanged([]CloudBot{renamed, unchanged, added}, []CloudBot{found, other})
	if !bots[0].Pinned || bots[1].Pinned || bots[2].Pinned {
		t.Errorf("pinned: %v %v %v", bots[0].Pinned, bots[1].Pinned, bots[2].Pinned)
	}
}
