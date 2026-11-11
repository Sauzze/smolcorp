package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func managerCall(t *testing.T, d *Deck, method, target string, asTool bool) (int, map[string]any) {
	t.Helper()
	c := call{method: method, target: target, host: "localhost:7710", from: thisComputer, asTool: asTool}
	if method == "POST" {
		c.headers = map[string]string{setupHeader: "1"}
	}
	w := serve(t, d, c)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

// The answer has the fixture's keys and types (the fixture wins); what differs,
// by the documented rule, is the features: only what this build serves.
func TestTheVersionAnswerHasTheFixturesShape(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	d := newTestDeck(t)
	for _, asTool := range []bool{false, true} {
		code, got := managerCall(t, d, "GET", "/setup/gm/version", asTool)
		if code != http.StatusOK {
			t.Fatalf("GET /setup/gm/version (tool %v): %d", asTool, code)
		}
		for key, want := range fixture {
			value, ok := got[key]
			if !ok || reflect.TypeOf(value) != reflect.TypeOf(want) {
				t.Errorf("key %q: got %v (%T), the fixture has %T", key, value, value, want)
			}
		}
		if len(got) != len(fixture) {
			t.Errorf("keys %v, the fixture has %d", got, len(fixture))
		}
		if got["version"] != Version || got["managerEnabled"] != false {
			t.Errorf("version %v, managerEnabled %v; want %q and false", got["version"], got["managerEnabled"], Version)
		}
		// Every feature named is one this build serves (none yet); an empty list, never null.
		if list, ok := got["features"].([]any); !ok || len(list) != len(features) {
			t.Errorf("features %v, want a list of this build's %d", got["features"], len(features))
		}
	}
}

func TestTheManagerSwitchTogglesAndPersists(t *testing.T) {
	d := newTestDeck(t)
	if code, got := managerCall(t, d, "POST", "/setup/gm/manager?enabled=1", false); code != http.StatusOK || got["managerEnabled"] != true {
		t.Fatalf("turning it on: %d %v", code, got)
	}
	if _, got := managerCall(t, d, "GET", "/setup/gm/version", false); got["managerEnabled"] != true {
		t.Errorf("the version answer after turning it on: %v", got)
	}
	saved, _ := os.ReadFile(configFile)
	if !strings.Contains(string(saved), `"managerEnabled": true`) {
		t.Errorf("not kept in the config: %s", saved)
	}
	// A fresh start reads it back.
	if !loadConfig().ManagerEnabled {
		t.Error("a restart lost the switch")
	}
	if code, got := managerCall(t, d, "POST", "/setup/gm/manager?enabled=0", false); code != http.StatusOK || got["managerEnabled"] != false {
		t.Fatalf("turning it off: %d %v", code, got)
	}
	if loadConfig().ManagerEnabled {
		t.Error("off was not kept")
	}
	for _, target := range []string{"/setup/gm/manager", "/setup/gm/manager?enabled=yes"} {
		if code, _ := managerCall(t, d, "POST", target, false); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want %d", target, code, http.StatusBadRequest)
		}
	}
}

func TestAToolCannotToggleTheManagerSwitch(t *testing.T) {
	d := newTestDeck(t)
	if code, _ := managerCall(t, d, "POST", "/setup/gm/manager?enabled=1", true); code != http.StatusForbidden {
		t.Errorf("a tool with the setup key: %d, want %d", code, http.StatusForbidden)
	}
	if code := serve(t, d, call{method: "POST", target: "/setup/gm/manager?enabled=1", host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1"}}).Code; code != http.StatusUnauthorized {
		t.Errorf("no key at all: %d, want %d", code, http.StatusUnauthorized)
	}
	if loadConfig().ManagerEnabled {
		t.Error("a refused call turned the manager on")
	}
}

// managerOffRows is every /setup/gm/ row of routes.json as a request, with the callers the table lets in.
func managerOffRows(t *testing.T) (rows []fixtureRow) {
	t.Helper()
	for _, row := range routeFixture(t) {
		if strings.HasPrefix(row.Path, "/setup/gm/") {
			rows = append(rows, row)
		}
	}
	if len(rows) < 20 {
		t.Fatalf("only %d /setup/gm/ rows in routes.json", len(rows))
	}
	return rows
}

func TestWithTheManagerOffEveryGMRouteIsA409ButTheOpenOnes(t *testing.T) {
	d := newTestDeck(t)
	const want = "The manager is off. Turn it on in Smolcorp's setup."
	for _, row := range managerOffRows(t) {
		method, path := concreteRoute(row)
		for _, asTool := range []bool{false, true} {
			if asTool && !routeAllows(callerTool, method, path) {
				continue
			}
			w := serve(t, d, call{method: method, target: path, host: "localhost:7710", from: thisComputer, asTool: asTool,
				headers: map[string]string{setupHeader: "1"}})
			// By hand stays open too, as on the Mac: served under the Mac app (gmbyhand.go), 404 here.
			open := method == "GET" && path == "/setup/gm/version" || method == "POST" && (path == "/setup/gm/manager" || path == "/setup/gm/by-hand")
			if open && w.Code == http.StatusConflict {
				t.Errorf("%s %s (tool %v): 409, it stays open", method, path, asTool)
			}
			if !open && (w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != want) {
				t.Errorf("%s %s (tool %v): %d %q, want 409 %q", method, path, asTool, w.Code, w.Body, want)
			}
		}
	}
	// Any method on the prefix, listed or not.
	for _, method := range []string{"GET", "POST", "DELETE"} {
		w := serve(t, d, call{method: method, target: "/setup/gm/nothing-here", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusConflict {
			t.Errorf("%s /setup/gm/nothing-here: %d, want 409", method, w.Code)
		}
	}
	// Not under /setup/gm/: untouched.
	if w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer}); w.Code != http.StatusOK {
		t.Errorf("/setup/state with the manager off: %d", w.Code)
	}
}

func TestWithTheManagerOnTheGMRoutesAreNotA409(t *testing.T) {
	d := newTestDeck(t)
	if code, _ := managerCall(t, d, "POST", "/setup/gm/manager?enabled=1", false); code != http.StatusOK {
		t.Fatalf("turning it on: %d", code)
	}
	for _, target := range []string{"/setup/gm/version", "/setup/gm/team", "/setup/gm/nothing-here"} {
		if code, _ := managerCall(t, d, "GET", target, false); code == http.StatusConflict {
			t.Errorf("GET %s with the manager on: 409", target)
		}
	}
	// And back off: the 409 returns.
	managerCall(t, d, "POST", "/setup/gm/manager?enabled=0", false)
	if code, _ := managerCall(t, d, "GET", "/setup/gm/team", false); code != http.StatusConflict {
		t.Errorf("team with it off again: %d, want 409", code)
	}
}

// The route table's gate comes first: a caller it refuses gets 403 (or 401 with no key), never the 409.
func TestARefusedCallerStillGets403BeforeTheManagerOff409(t *testing.T) {
	d := newTestDeck(t)
	refused := 0
	for _, row := range managerOffRows(t) {
		method, path := concreteRoute(row)
		if routeAllows(callerTool, method, path) {
			continue
		}
		refused++
		if code, _ := managerCall(t, d, method, path, true); code != http.StatusForbidden {
			t.Errorf("%s %s: a refused tool got %d, want 403", method, path, code)
		}
	}
	if refused == 0 {
		t.Fatal("no row refuses a tool")
	}
	w := serve(t, d, call{method: "GET", target: "/setup/gm/team", host: "localhost:7710", from: thisComputer, withoutSetupKey: true})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no key: %d, want 401", w.Code)
	}
}

// The event log keeps logging with the manager off; only the route answers 409.
func TestEventsStillLogWhileTheManagerIsOff(t *testing.T) {
	d := newTestDeck(t)
	d.events = newEventLog(t.TempDir(), eventsFileMost)
	d.events.append(eventEntry{Agent: "claude", At: 1793500000000, Kind: "sessionStart", Session: "s-1"})
	if code, _ := managerCall(t, d, "GET", "/setup/gm/events?since=0", false); code != http.StatusConflict {
		t.Fatalf("events while off: %d, want 409", code)
	}
	d.events.append(eventEntry{Agent: "claude", At: 1793500001000, Kind: "notification", Session: "s-1"})
	managerCall(t, d, "POST", "/setup/gm/manager?enabled=1", false)
	code, got := managerCall(t, d, "GET", "/setup/gm/events?since=0", false)
	if code != http.StatusOK {
		t.Fatalf("events once on: %d", code)
	}
	if list, _ := got["events"].([]any); len(list) != 2 {
		t.Errorf("the history after turning it on: %v, want both events", got)
	}
}

func TestTheVersionListsExactlyWhatIsServed(t *testing.T) {
	d := newTestDeck(t)
	_, got := managerCall(t, d, "GET", "/setup/gm/version", false)
	list, _ := got["features"].([]any)
	if !reflect.DeepEqual(list, []any{"events", "team"}) {
		t.Errorf("features %v, want [events team]", list)
	}
	// Under the Mac app, a new token and the screens and tiling too, in the Mac's words.
	withMac(t)
	d = newTestDeck(t)
	_, got = managerCall(t, d, "GET", "/setup/gm/version", false)
	if list, _ := got["features"].([]any); !reflect.DeepEqual(list, []any{"events", "team", "token", "screens", "tile"}) {
		t.Errorf("features under the Mac app %v", list)
	}
}

// managerOn turns the front desk on in a test deck, for the tests of what its routes answer.
func managerOn(d *Deck) {
	d.mu.Lock()
	d.config.ManagerEnabled = true
	d.mu.Unlock()
}
