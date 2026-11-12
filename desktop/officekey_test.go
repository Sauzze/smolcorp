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
	Names   []string                     `json:"names"`
	Layouts map[string]map[string]string `json:"layouts"`
	Cases   []struct {
		Layout  string  `json:"layout"`
		Keys    string  `json:"keys"`
		Status  int     `json:"status"`
		Held    string  `json:"held"`
		Saved   *string `json:"saved"`
		Symbols string  `json:"symbols"`
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
// the same words; on the layout the Mac app says is in use too (FXKSC B2), where the Mac's own keys are refused
// by what the key types there, and the keys are shown as printed.
func TestTheOfficeKeysAreCheckedAsTheSharedFixtureSays(t *testing.T) {
	fixture := readOfficeKeyFixture(t)
	if names := slices.Sorted(maps.Keys(officeKeyNames)); !slices.Equal(names, slices.Sorted(slices.Values(fixture.Names))) {
		t.Errorf("the keys: %v, the fixture's %v", names, fixture.Names)
	}
	if len(fixture.Layouts["azerty"]) == 0 {
		t.Fatal("the fixture has no AZERTY layout")
	}
	for _, c := range fixture.Cases {
		labels := fixture.Layouts[c.Layout]
		keys, refusal := officeKeyChoice(c.Keys, labels)
		if c.Symbols != "" && officeKeySymbols(keys, labels) != c.Symbols {
			t.Errorf("%q on %q: shown %q, want %q", c.Keys, c.Layout, officeKeySymbols(keys, labels), c.Symbols)
		}
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
		if got := officeKeyChosen(saved, nil); got != want {
			t.Errorf("kept %q: %q, want %q", saved, got, want)
		}
	}
	// On AZERTY, ⌘ on the US , key is its ⌘;, and stays; ⌘ on the US A key is its ⌘Q, and doesn't.
	for saved, want := range map[string]string{"cmd+,": "cmd+,", "cmd+A": officeKeyDefault} {
		if got := officeKeyChosen(saved, fixture.Layouts["azerty"]); got != want {
			t.Errorf("kept %q on AZERTY: %q, want %q", saved, got, want)
		}
	}
}

// A test copy of the Mac app holds no office shortcut (TKEY): whatever the app says it holds, setup says
// held: false, even with the keys off; the real app's word is believed.
func TestATestCopyOfTheMacHoldsNoOfficeKeys(t *testing.T) {
	for _, c := range []struct {
		saved, held string
		testCopy    bool
		want        bool
	}{
		{"", "alt+cmd+O", false, true},
		{"", "alt+cmd+O", true, false},
		{"off", "off", false, true},
		{"off", "off", true, false},
		{"ctrl+alt+K", "ctrl+alt+K", true, false},
		{"ctrl+alt+K", "alt+cmd+O", false, false},
	} {
		state := officeKeyState(c.saved, &macOfficeKey{Held: c.held}, c.testCopy)
		if state["held"] != c.want {
			t.Errorf("saved %q, held %q, test copy %v: held %v, want %v", c.saved, c.held, c.testCopy, state["held"], c.want)
		}
	}
	if state := officeKeyState("", nil, false); state["held"] != false {
		t.Errorf("no Mac app: %v", state["held"])
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
	// Nothing holds them here, and the keys are named by their US names (FXKSC N2, B2).
	state := (&router{deck: d}).setupState()["officeKey"].(map[string]any)
	if state["keys"] != "ctrl+alt+K" || state["default"] != officeKeyDefault || state["held"] != false || len(state["labels"].(map[string]string)) != 0 {
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
// has switched on) is a 409 and nothing changes. A refused ask never reaches the app. The app's facts say the
// layout in use (a French AZERTY here) and the keys it holds now: the Mac's own keys are refused by what the key
// types there, and setup shows those names and whether the keys kept are the ones held (FXKSC B2, N2).
func TestTheMacAppHoldsTheOfficeKeysBeforeTheyreKept(t *testing.T) {
	mac := withMac(t)
	mac.facts.TestCopy = false // the fixture's facts are a test copy's, which holds none (TKEY): this is the real app
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
	quit := "⌘Q is the Mac's own, in every app: choose another."
	if code, text := pageCall(t, d, "POST", "/setup/officekey?keys=cmd%2BA"); code != http.StatusBadRequest || text != quit || len(mac.requests("office-key")) != 1 {
		t.Errorf("⌘ on AZERTY's Q: %d %q, the app asked %d times", code, text, len(mac.requests("office-key")))
	}
	taken := "⌃⌥T is taken: macOS has it switched on, or won't let Smolcorp hold it. Choose another."
	mac.refusing(map[string]string{"office-key": taken})
	if code, text := pageCall(t, d, "POST", "/setup/officekey?keys=ctrl%2Balt%2BT"); code != http.StatusConflict || text != taken || loadConfig().OfficeKey != "off" {
		t.Errorf("taken: %d %q, kept %q", code, text, loadConfig().OfficeKey)
	}
	mac.refusing(nil)
	// The app holds ⌥⌘ on AZERTY's A (the fixture's facts), not "off": setup says so.
	state := func() map[string]any {
		return (&router{deck: d}).setupStateFor(callerPage)["officeKey"].(map[string]any)
	}
	if s := state(); s["keys"] != "off" || s["held"] != false || s["labels"].(map[string]string)["Q"] != "A" {
		t.Errorf("off, not held: %v", s)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=alt%2Bcmd%2BQ"); code != http.StatusOK || loadConfig().OfficeKey != "alt+cmd+Q" {
		t.Errorf("⌥⌘A: %d, kept %q", code, loadConfig().OfficeKey)
	}
	if s := state(); s["keys"] != "alt+cmd+Q" || s["held"] != true {
		t.Errorf("held: %v", s)
	}
	// The same word from a test copy of the Mac app: not held, as it holds none (TKEY).
	mac.facts.TestCopy = true
	if s := state(); s["keys"] != "alt+cmd+Q" || s["held"] != false {
		t.Errorf("a test copy's: %v", s)
	}
	mac.facts.TestCopy = false
	if code, _ := pageCall(t, d, "POST", "/setup/officekey?keys=default"); code != http.StatusOK || loadConfig().OfficeKey != "" {
		t.Errorf("back to ⌥⌘O: %d, kept %q", code, loadConfig().OfficeKey)
	}
	if asked := mac.requests("office-key"); len(asked) != 4 || asked[3].Keys != officeKeyDefault {
		t.Errorf("the app was asked %+v", asked)
	}
}
