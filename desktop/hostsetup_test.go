package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// macStandIn stands in for the Mac app on the actions for what only macOS
// does, as HostBridge.swift answers them, with the shared fixture's facts:
// tab actions it can't do (as a Terminal.app tab without the app), the rest
// done. It keeps what it was asked, and is safe to ask from the deck's rounds.
type macStandIn struct {
	mu        sync.Mutex
	asked     []bridgeRequest
	facts     macFacts
	abilities tabAbilities
	refuse    map[string]string // by action: the app's no
}

func (m *macStandIn) call(request bridgeRequest) (bridgeAnswer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asked = append(m.asked, request)
	if why, ok := m.refuse[request.Action]; ok {
		return bridgeAnswer{Error: why}, nil
	}
	switch request.Action {
	case "facts":
		facts := m.facts
		return bridgeAnswer{OK: true, Facts: &facts}, nil
	case "dock", "accessibility", "keystrokes-on", "terminal-command", "dictation":
		return bridgeAnswer{OK: true}, nil
	case "keystroke-apps":
		list := slices.DeleteFunc(slices.Clone(request.Apps), func(app string) bool { return strings.EqualFold(app, request.App) })
		if *request.On {
			list = append(list, request.App)
		}
		return bridgeAnswer{OK: true, Apps: list}, nil
	case "glossary":
		return bridgeAnswer{OK: true, Names: request.Names}, nil
	case "abilities":
		said := make([]tabAbilities, len(request.Tabs))
		for i := range said {
			said[i] = m.abilities
		}
		return bridgeAnswer{OK: true, Abilities: said}, nil
	case "bots-resolve":
		return bridgeAnswer{OK: true, Resolved: make([]*botResolution, len(request.Bots))}, nil
	}
	return bridgeAnswer{Error: "This terminal can't do that.", Unsupported: true}, nil
}

// actions is what it was asked, by action, in order.
func (m *macStandIn) actions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for _, request := range m.asked {
		names = append(names, request.Action)
	}
	return names
}

// bridgeCase is one of the shared fixture's requests and the Mac app's answer.
type bridgeCase struct {
	Name    string          `json:"name"`
	Request json.RawMessage `json:"request"`
	Answer  json.RawMessage `json:"answer"`
}

func readBridgeCases(t *testing.T) []bridgeCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "bridge", "actions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Cases []bridgeCase }
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) < 10 {
		t.Fatalf("the bridge's fixture: %v, %d cases", err, len(fixture.Cases))
	}
	return fixture.Cases
}

// standInFacts is the fixture's facts, as the Mac app answers them.
func standInFacts(t *testing.T) macFacts {
	t.Helper()
	var answer bridgeAnswer
	if err := json.Unmarshal(readBridgeCases(t)[0].Answer, &answer); err != nil || answer.Facts == nil {
		t.Fatalf("the fixture's first case isn't the facts: %v", err)
	}
	return *answer.Facts
}

// withMac puts the stand-in Mac app in place for one test.
func withMac(t *testing.T) *macStandIn {
	t.Helper()
	mac := &macStandIn{facts: standInFacts(t), abilities: tabAbilities{Keys: true, Text: true, Fire: true, Screen: true}}
	saved := host
	t.Cleanup(func() { host = saved })
	host = mac
	return mac
}

// fixtureHost answers one case's answer and keeps the request.
type fixtureHost struct {
	asked  *bridgeRequest
	answer bridgeAnswer
}

func (f *fixtureHost) call(request bridgeRequest) (bridgeAnswer, error) {
	f.asked = &request
	return f.answer, nil
}

// Each of the fixture's requests is what the core sends, and its answer is read as the core needs it.
func TestTheBridgeFixtureIsWhatTheCoreAsksAndReads(t *testing.T) {
	want := map[string]func(h hostBridge) (any, error){
		"setup's facts, the apps keys may go to left out of those running": func(h hostBridge) (any, error) {
			facts, err := hostFacts(h, []string{"com.example.notes"})
			return []any{facts.Accessibility, facts.TestCopy, facts.Dictation.Languages, facts.Dictation.CleanupProblem == nil,
				facts.Running, facts.Terminal.State, facts.Terminal.TestCopy}, err
		},
		"In the Dock, switched off":      func(h hostBridge) (any, error) { return nil, hostDock(h, false) },
		"Accessibility asked for":        func(h hostBridge) (any, error) { return nil, hostAskAccessibility(h) },
		"keys in other apps switched on": func(h hostBridge) (any, error) { return nil, hostKeystrokesOn(h) },
		"an app allowed keys": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes"}, "com.example.mail", true)
		},
		"an app taken off, any capitals": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes", "com.example.mail"}, "COM.EXAMPLE.NOTES", false)
		},
		"a terminal never allowed keys": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes"}, "com.apple.Terminal", true)
		},
		"dictation's names, checked": func(h hostBridge) (any, error) {
			return hostGlossary(h, []string{"Smolcorp", " smolcorp ", "Tailscale", ""})
		},
		"dictation got ready, with a problem":      func(h hostBridge) (any, error) { return hostPrepareDictation(h, "fr_FR") },
		"dictation in a language it doesn't know":  func(h hostBridge) (any, error) { return hostPrepareDictation(h, "xx_XX") },
		"Add smolcorp to Terminal, in a test copy": func(h hostBridge) (any, error) { return nil, hostTerminalCommand(h, true) },
		"the desks' terminals, from the drivers' tables": func(h hostBridge) (any, error) {
			return hostAbilities(h, []string{"iterm:AB12", "tty:/dev/ttys003", "headless:3f2a9c1e"})
		},
		"Grok Bots looked for in the Grok Bot app": func(h hostBridge) (any, error) {
			return hostResolveBots(h, []bridgeBot{{Bot: "Smolcorp Mini"}, {Bot: "Nobody Here"}})
		},
	}
	// What the core makes of each answer.
	got := map[string]any{
		"setup's facts, the apps keys may go to left out of those running": []any{false, true, []string{"fr_FR", "en_US"}, true,
			[]runningApp{{ID: "com.example.mail", Name: "Mail"}}, "absent", true},
		"an app allowed keys":                            []string{"com.example.notes", "com.example.mail"},
		"an app taken off, any capitals":                 []string{"com.example.mail"},
		"a terminal never allowed keys":                  []string{},
		"dictation's names, checked":                     []string{"Smolcorp", "Tailscale"},
		"dictation got ready, with a problem":            "Apple's model for French couldn't be downloaded.",
		"dictation in a language it doesn't know":        "",
		"the desks' terminals, from the drivers' tables": []tabAbilities{{true, true, true, true}, {true, true, true, true}, {}},
		"Grok Bots looked for in the Grok Bot app":       []*botResolution{{BotID: "0a1b-2c3d", Name: "Smolcorp Mini Manager", By: "guess"}, nil},
	}
	cases := readBridgeCases(t)
	if len(cases) != len(want) {
		t.Errorf("the fixture has %d cases, this test %d", len(cases), len(want))
	}
	for _, c := range cases {
		run, ok := want[c.Name]
		if !ok {
			t.Errorf("%q: not a case this test knows", c.Name)
			continue
		}
		fake := &fixtureHost{}
		if err := json.Unmarshal(c.Answer, &fake.answer); err != nil {
			t.Fatal(err)
		}
		result, err := run(fake)
		if fake.answer.OK != (err == nil) {
			t.Errorf("%q: error %v, for an answer ok=%v", c.Name, err, fake.answer.OK)
		}
		if refusal := appRefusal(""); !fake.answer.OK && !errors.As(err, &refusal) {
			t.Errorf("%q: %v isn't the app's refusal", c.Name, err)
		}
		if fake.asked == nil {
			t.Errorf("%q: nothing asked", c.Name)
			continue
		}
		sent, _ := json.Marshal(fake.asked)
		var asked, fixture map[string]any
		_ = json.Unmarshal(sent, &asked)
		_ = json.Unmarshal(c.Request, &fixture)
		if !reflect.DeepEqual(asked, fixture) {
			t.Errorf("%q: the core asks %s, the fixture %s", c.Name, sent, c.Request)
		}
		if expected, ok := got[c.Name]; ok && !reflect.DeepEqual(result, expected) {
			t.Errorf("%q: the core reads %#v, want %#v", c.Name, result, expected)
		}
	}
}

// Without the Mac app (Windows, Linux) nothing of the Mac's is said or served: nothing is made up.
func TestWithoutTheMacAppSetupAndTheOfficeHaveNoneOfTheMacs(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	running(d, 1)
	state := (&router{deck: d}).setupState()
	for _, field := range []string{"dictation", "keystrokes", "dock", "terminalCommand"} {
		if _, ok := state[field]; ok {
			t.Errorf("setup's state has %s", field)
		}
	}
	if _, ok := state["checklist"].(map[string]any)["accessibility"]; ok {
		t.Error("setup's checklist has accessibility")
	}
	if _, ok := state["cloudBots"].(map[string]any)["autoDetect"]; ok {
		t.Error("setup's bots have autoDetect")
	}
	for _, path := range []string{"/setup/dock?enabled=0", "/setup/accessibility", "/setup/keystrokes?enabled=1", "/setup/dictation?enabled=1",
		"/setup/terminal-command", "/setup/bots/resolve"} {
		if code, _ := pageCall(t, d, "POST", path); code != http.StatusNotFound {
			t.Errorf("POST %s: %d, want 404", path, code)
		}
	}
	d.mu.Lock()
	var snapshot map[string]any
	_ = json.Unmarshal(d.snapshot(), &snapshot)
	d.mu.Unlock()
	if _, ok := snapshot["dictation"]; ok {
		t.Error("the snapshot has dictation")
	}
	keys, _, _ := deckState(t, d)
	for _, field := range []string{"keys", "fire", "dictates", "choice"} {
		if _, ok := keys[0]["session"].(map[string]any)[field]; ok {
			t.Errorf("a desk has %s", field)
		}
	}
}

// Under the Mac app, setup's Mac switches are kept here and done by the app, as the Swift engine does them.
func TestTheMacsSwitchesAreKeptHereAndDoneByTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	state := (&router{deck: d}).setupState()
	if state["dock"].(map[string]any)["enabled"] != true || state["checklist"].(map[string]any)["accessibility"] != false ||
		state["cloudBots"].(map[string]any)["autoDetect"] != true || state["terminalCommand"].(terminalCmd).State != "absent" {
		t.Errorf("setup's state: %v %v %v %v", state["dock"], state["checklist"], state["cloudBots"], state["terminalCommand"])
	}
	dictation := state["dictation"].(map[string]any)
	if dictation["language"] != "en_US" || dictation["enabled"] != false || dictation["available"] != true {
		t.Errorf("dictation, before it's set: %v", dictation)
	}

	if code, text := pageCall(t, d, "POST", "/setup/dock?enabled=0"); code != http.StatusOK || loadConfig().ShowInDock == nil || *loadConfig().ShowInDock {
		t.Errorf("In the Dock off: %d %s, kept %v", code, text, loadConfig().ShowInDock)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes?enabled=yes"); code != http.StatusBadRequest {
		t.Errorf("keystrokes, neither 1 nor 0: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes?enabled=1"); code != http.StatusOK || !loadConfig().KeystrokesAllowed {
		t.Errorf("keystrokes on: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes/apps?add=com.example.mail"); code != http.StatusOK ||
		!slices.Equal(loadConfig().KeystrokeApps, []string{"com.example.mail"}) {
		t.Errorf("an app added: %d, kept %v", code, loadConfig().KeystrokeApps)
	}
	mac.refuse = map[string]string{"keystroke-apps": "Keys are never sent to a terminal."}
	if code, text := pageCall(t, d, "POST", "/setup/keystrokes/apps?add=com.apple.terminal"); code != http.StatusConflict || text != "Keys are never sent to a terminal." {
		t.Errorf("a terminal added: %d %q, want 409 in the app's words", code, text)
	}
	mac.refuse = nil
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes/apps"); code != http.StatusBadRequest {
		t.Errorf("neither add nor remove: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/dictation?language=xx_XX"); code != http.StatusBadRequest {
		t.Errorf("an unknown language: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/dictation?enabled=1&language=fr_FR&claude=1"); code != http.StatusOK {
		t.Errorf("dictation on: %d", code)
	}
	if c := loadConfig(); !c.DictationEnabled || c.DictationLanguage != "fr_FR" || !c.DictationForClaude {
		t.Errorf("dictation kept as %v %q %v", c.DictationEnabled, c.DictationLanguage, c.DictationForClaude)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/accessibility"); code != http.StatusNoContent {
		t.Errorf("Accessibility asked for: %d", code)
	}
	mac.refuse = map[string]string{"terminal-command": "This is a test copy of Smolcorp."}
	if code, text := pageCall(t, d, "POST", "/setup/terminal-command"); code != http.StatusConflict || text != "This is a test copy of Smolcorp." {
		t.Errorf("Add smolcorp to Terminal in a test copy: %d %q", code, text)
	}
	mac.refuse = nil
	var done []string
	for _, action := range mac.actions() {
		if action != "facts" && action != "bots-resolve" && action != "abilities" {
			done = append(done, action)
		}
	}
	want := []string{"dock", "keystrokes-on", "keystroke-apps", "keystroke-apps", "dictation", "accessibility", "terminal-command"}
	if !slices.Equal(done, want) {
		t.Errorf("the app was asked %v, want %v", done, want)
	}
	// Switched on: in the event log, as on the Mac.
	found := d.events.answer(d.events.wait(t.Context(), 0, 50, 0), 0)
	data, _ := json.Marshal(found)
	if !strings.Contains(string(data), `"detail":"switched on"`) || !strings.Contains(string(data), `"detail":"app allowed: com.example.mail"`) {
		t.Errorf("the event log: %s", data)
	}
}

// The desks' abilities are asked of the app once per set of tabs, on their own, never per desk or per snapshot.
func TestTheDesksAbilitiesAreAskedOncePerSetOfTabs(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	for i, tab := range []string{"tty:/dev/ttys901", "iterm:AB12"} {
		d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m" + string(rune('0'+i)), "cwd": "/work/mac"},
			Claude, tab, "", float64(100+i))
	}
	learned := func() map[string]any {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			d.mu.Lock()
			d.snapshot()
			learning := d.mac.learning || d.mac.abilities == nil
			d.mu.Unlock()
			if !learning {
				break
			}
		}
		keys, _, _ := deckState(t, d)
		return keys[0]["session"].(map[string]any)
	}
	desk := learned()
	if desk["keys"] != true || desk["fire"] != true || desk["dictates"] != false || desk["choice"] != nil {
		t.Errorf("a desk under the Mac app: %v", desk)
	}
	for range 5 {
		deckState(t, d)
	}
	count := func() int {
		n := 0
		for _, action := range mac.actions() {
			if action == "abilities" {
				n++
			}
		}
		return n
	}
	if count() != 1 {
		t.Errorf("asked %d times for the same tabs, want 1", count())
	}
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m9", "cwd": "/work/other"}, Claude, "tty:/dev/ttys909", "", 200)
	learned()
	if count() != 2 {
		t.Errorf("asked %d times after a new tab, want 2", count())
	}
	var snapshot map[string]any
	d.mu.Lock()
	_ = json.Unmarshal(d.snapshot(), &snapshot)
	d.mu.Unlock()
	if dictation, _ := snapshot["dictation"].(map[string]any); dictation["on"] != false || dictation["language"] != "" && dictation["language"] != "en_US" {
		t.Errorf("the snapshot's dictation: %v", snapshot["dictation"])
	}
}

// A Grok Bot set without its id is given the one the Grok Bot app has; the bots are answered with who they are there.
func TestGrokBotsAreFoundInTheGrokBotAppThroughTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	body := `{"bots": [{"id": "mini", "name": "Mini", "source": {"kind": "grokBotApp", "bot": "Smolcorp Mini"}}, {"id": "plain", "name": "Plain"}]}`
	w := serve(t, d, call{method: "POST", target: "/setup/bots", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: body})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"resolved":null`) {
		t.Fatalf("the bots: %d %s", w.Code, w.Body)
	}
	found := &fixtureHost{answer: bridgeAnswer{OK: true, Resolved: []*botResolution{{BotID: "0a1b-2c3d", Name: "Smolcorp Mini Manager", By: "guess"}}}}
	host = found
	code, text := pageCall(t, d, "POST", "/setup/bots/resolve")
	var answer struct {
		Bots               []map[string]any
		Filled, Unresolved []string
	}
	_ = json.Unmarshal([]byte(text), &answer)
	if code != http.StatusOK || !slices.Equal(answer.Filled, []string{"mini"}) || len(answer.Unresolved) != 0 {
		t.Fatalf("resolve: %d %s", code, text)
	}
	if got := loadConfig().CloudBots[0].Source; got.BotID != "0a1b-2c3d" || got.Bot != "Smolcorp Mini Manager" {
		t.Errorf("kept as %+v", got)
	}
	if found.asked == nil || found.asked.Action != "bots-resolve" {
		t.Errorf("the app was asked %+v", found.asked)
	}
	_ = mac
}
