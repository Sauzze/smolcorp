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
