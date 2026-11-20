package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTheSetupStateHasTheManagerAsTheMacsHasIt(t *testing.T) {
	d := newTestDeck(t)
	manager, ok := (&router{deck: d}).setupState()["manager"].(map[string]any)
	if !ok || manager["enabled"] != false || manager["gmByHand"] != false || manager["gmStartedByHand"] != false ||
		manager["gmTokenFile"] != "" || manager["gmTokenFileShown"] != "" {
		t.Fatalf("manager: %v", manager)
	}
	d.mu.Lock()
	d.config.ManagerEnabled = true
	d.mu.Unlock()
	if on := (&router{deck: d}).setupState()["manager"].(map[string]any)["enabled"]; on != true {
		t.Errorf("manager.enabled with the switch on: %v", on)
	}
}

func TestTheSetupSideRefusesAsTheMacDoes(t *testing.T) {
	d := newTestDeck(t)
	// With the manager off, the GM's routes answer 409 before anything else; /nothing-here is the contract test's.
	d.mu.Lock()
	d.config.ManagerEnabled = true
	d.mu.Unlock()
	ask := func(method, target string, withoutKey bool) *httptest.ResponseRecorder {
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: withoutKey,
			headers: map[string]string{setupHeader: "1"}})
	}
	if got := ask("POST", "/setup/session", true); got.Code != http.StatusUnauthorized || got.Body.String() != "Only the menu's link opens a setup page." {
		t.Errorf("a session without a menu link's key: %d %q", got.Code, got.Body)
	}
	// The valid setup key opens no session: with no menu link's token it is the second 401.
	if got := ask("POST", "/setup/session", false); got.Code != http.StatusUnauthorized || got.Body.String() != "Only the menu's link opens a setup page." {
		t.Errorf("a session with the setup key and no menu link's token: %d %q", got.Code, got.Body)
	}
	for _, target := range []string{"/setup/logo/nope", "/setup/nothing-here", "/setup/gm/nothing-here", "/setup/qr.png"} {
		if got := ask("GET", target, false); got.Code != http.StatusNotFound || got.Body.Len() != 0 {
			t.Errorf("GET %s: %d %q, want an empty 404", target, got.Code, got.Body)
		}
	}
}

// A bot's floor sent as a number isn't refused: it changes nothing, as the Mac's reads only text (the page sends text).
func TestABotsFloorSentAsANumberIsIgnoredNotRefused(t *testing.T) {
	d := newTestDeck(t)
	page := func(path, body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: path, body: body, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
	}
	if got := page("/setup/bots", `{"bots":[{"id":"c2-bot2","name":"C2 Bot Two"}]}`); got.Code != http.StatusOK {
		t.Fatalf("the bot: %d %s", got.Code, got.Body)
	}
	floor := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.CloudBots[0].Floor
	}
	before := floor()
	for _, body := range []string{`{"id":"c2-bot2","floor":3}`, `{"id":"c2-bot2","floor":null}`, `{"id":"c2-bot2","floor":["3"]}`, `{"id":"c2-bot2","floor":true}`} {
		if got := page("/setup/bots/override", body); got.Code != http.StatusOK {
			t.Errorf("%s: %d %s", body, got.Code, got.Body)
		}
	}
	if floor() != before {
		t.Errorf("the floor moved to %q from %q", floor(), before)
	}
	// As text it still moves it.
	want := cloudFloors[len(cloudFloors)-1]
	if got := page("/setup/bots/override", `{"id":"c2-bot2","floor":"`+want+`"}`); got.Code != http.StatusOK || floor() != want {
		t.Errorf("as text: %d, floor %q, want %q", got.Code, floor(), want)
	}
	// And a bot that isn't there is as it was.
	if got := page("/setup/bots/override", `{"id":"nobody","floor":3}`); got.Code != http.StatusNotFound {
		t.Errorf("no such bot: %d %s", got.Code, got.Body)
	}
}
