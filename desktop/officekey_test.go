package desktop

import (
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// officeKeyFixture is Tests/fixtures/officekey.json, which the Mac app's OfficeKeyTests.swift reads too.
type officeKeyFixture struct {
	Names []string `json:"names"`
	Cases []struct {
		Keys    string  `json:"keys"`
		Status  int     `json:"status"`
		Held    string  `json:"held"`
		Saved   *string `json:"saved"`
		Refusal string  `json:"refusal"`
	} `json:"cases"`
}

func readOfficeKeyFixture(t *testing.T) officeKeyFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "officekey.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture officeKeyFixture
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) < 10 {
		t.Fatalf("the fixture: %v, %d cases", err, len(fixture.Cases))
	}
	return fixture
}

// The keys are checked as the Mac app checks them: the same names, and each ask held and kept, or refused in
// the same words.
func TestTheOfficeKeysAreCheckedAsTheSharedFixtureSays(t *testing.T) {
	fixture := readOfficeKeyFixture(t)
	if names := slices.Sorted(maps.Keys(officeKeyNames)); !slices.Equal(names, slices.Sorted(slices.Values(fixture.Names))) {
		t.Errorf("the keys: %v, the fixture's %v", names, fixture.Names)
	}
	for _, c := range fixture.Cases {
		keys, refusal := officeKeyChoice(c.Keys)
		if c.Status != http.StatusOK {
			if refusal != c.Refusal {
				t.Errorf("%q: refused %q, want %q", c.Keys, refusal, c.Refusal)
			}
			continue
		}
		saved := officeKeySaved(keys)
		want := ""
		if c.Saved != nil {
			want = *c.Saved
		}
		if refusal != "" || keys != c.Held || saved != want {
			t.Errorf("%q: held %q, kept %q, refused %q; want %q, %q", c.Keys, keys, saved, refusal, c.Held, want)
		}
	}
	// A hand-edited entry unfit to hold is ⌥⌘O, as on the Mac.
	for saved, want := range map[string]string{"": officeKeyDefault, "off": "off", "cmd+Q": officeKeyDefault, "nonsense": officeKeyDefault, "ctrl+alt+K": "ctrl+alt+K"} {
		if got := officeKeyChosen(saved); got != want {
			t.Errorf("kept %q: %q, want %q", saved, got, want)
		}
	}
}

// On Windows and Linux there's no key from anywhere: the keys are checked and kept for the page, nothing asked.
// Only the page may set them: a GM, a tool and a hire are refused.
func TestTheOfficeKeysAreKeptHereAndSetByThePageAlone(t *testing.T) {
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	for _, who := range []caller{callerGM, callerTool, callerHire} {
		c := asCaller(who, gm, hireKey)
		c.method, c.target, c.host, c.from = "POST", "/setup/officekey?keys=ctrl%2Balt%2BK", "localhost:7710", thisComputer
		if c.headers == nil {
			c.headers = map[string]string{}
		}
		c.headers[setupHeader] = "1"
		if code := serve(t, d, c).Code; code != http.StatusForbidden {
			t.Errorf("caller %s: %d, want 403", who.name(), code)
		}
	}
	if loadConfig().OfficeKey != "" {
		t.Fatalf("a refused caller set the keys: %q", loadConfig().OfficeKey)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=ctrl%2Balt%2BK"); code != http.StatusOK || loadConfig().OfficeKey != "ctrl+alt+K" {
		t.Fatalf("the page: %d, kept %q", code, loadConfig().OfficeKey)
	}
	state := (&router{deck: d}).setupState()["officeKey"].(map[string]any)
	if state["keys"] != "ctrl+alt+K" || state["default"] != officeKeyDefault {
		t.Errorf("the state: %v", state)
	}
	code, text := pageCall(t, d, "POST", "/setup/officekey?keys=cmd%2BQ")
	if code != http.StatusBadRequest || text != "⌘Q is the Mac's own, in every app: choose another." || loadConfig().OfficeKey != "ctrl+alt+K" {
		t.Errorf("⌘Q: %d %q, kept %q", code, text, loadConfig().OfficeKey)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=default"); code != http.StatusOK || loadConfig().OfficeKey != "" {
		t.Errorf("back to ⌥⌘O: %d, kept %q", code, loadConfig().OfficeKey)
	}
}

// Under the Mac app: the app holds the keys first, through the bridge, then they're kept; its no (keys macOS
// or another app has) is a 409 and nothing changes. A refused ask never reaches the app.
func TestTheMacAppHoldsTheOfficeKeysBeforeTheyreKept(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=off"); code != http.StatusOK || loadConfig().OfficeKey != "off" {
		t.Fatalf("off: %d, kept %q", code, loadConfig().OfficeKey)
	}
	if asked := mac.requests("office-key"); len(asked) != 1 || asked[0].Keys != "off" {
		t.Errorf("the app was asked %+v", asked)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=K"); code != http.StatusBadRequest || len(mac.requests("office-key")) != 1 {
		t.Errorf("no modifier: %d, the app asked %d times", code, len(mac.requests("office-key")))
	}
	taken := "⌃⌥T is already used by macOS or another app: choose another."
	mac.refusing(map[string]string{"office-key": taken})
	if code, text := pageCall(t, d, "POST", "/setup/officekey?keys=ctrl%2Balt%2BT"); code != http.StatusConflict || text != taken || loadConfig().OfficeKey != "off" {
		t.Errorf("taken: %d %q, kept %q", code, text, loadConfig().OfficeKey)
	}
	mac.refusing(nil)
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=default"); code != http.StatusOK || loadConfig().OfficeKey != "" {
		t.Errorf("back to ⌥⌘O: %d, kept %q", code, loadConfig().OfficeKey)
	}
	if asked := mac.requests("office-key"); len(asked) != 3 || asked[2].Keys != officeKeyDefault {
		t.Errorf("the app was asked %+v", asked)
	}
}
