package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The route contract (M0 of the one-build plan): Tests/fixtures/contract/*.json is what the Mac answered on
// each route, written by the Swift engine's ContractTests.swift as shapes (the status, the kind of body, and for
// JSON its keys, their types and a closed set's values) until M7 deleted that engine: every Mac runs this app
// since 1.25.0, and the fixtures are kept by hand. Each fixture is asked of this app in process, and its answers checked against
// them. Where this app doesn't answer as the Mac does yet, the difference is a row of knownGaps, and of
// INVENTORY.md's "Known gaps" table, which must say the same: each M piece turns rows into passes, and a row
// that passes must come out of both.

// knownGap is one difference: in what (a route, or a shape by its name in contract.json), where (route: the
// route isn't served; status, kind, a refusal, or a field's path), what this app has instead, why, and who
// closes it: the M piece (M2 server and setup, M3 office and bots, M4 hiring and the GM, M4a briefs, M4b hiring follow-ups, M5 T3, M6
// the personal shell), "via the bridge (M1)" for what only macOS can do, or "stays: <why>" for a Go-only extra.
type knownGap struct{ subject, where, got, why, owner string }

// knownGaps is every difference between the Mac's answers and this app's, today. Keep it in INVENTORY.md's
// order: TestTheKnownGapsAreInventorysGoStatus checks the two are the same table.
var knownGaps = []knownGap{
	{"setup-state", ".hiring.agentsChecked", "extra", "whether this computer checks agents' programs (a Mac checks Claude Code's signature, RT126SG2 B2), which the Swift engine never did", "stays: Go-only, the page's line under Let the office hire and fire; the Mac runs the Go core"},
	{"setup-state", ".hiring.claudeUnsigned", "extra", "an unsigned Claude Code refused after a signed one ran, for the owner to confirm (RT126SG2 B2), which the Swift engine never had", "stays: Go-only, the page's alone to confirm; the Mac runs the Go core"},
	{"setup-state", ".hiring.managedBeta", "extra", "the launcher's hire beta (Windows)", "stays: Go-only, the launcher's hire beta; the page doesn't read it"},
	{"setup-state", ".hiring.managedWindows", "extra", "the launcher's hire beta (Windows)", "stays: Go-only, the launcher's hire beta; the page doesn't read it"},
	{"setup-state", ".mailBeta", "extra", "mail to a hire is beta on Windows", "stays: Go-only, the page doesn't read it"},
	{"setup-state", ".wifiTyping", "extra", "Typing over Wi-Fi (SECWIFI), which the Swift engine never had", "stays: Go-only, the page's switch under Paired phones; the Mac runs the Go core"},
	{"setup-state", ".phones[].beforeSigning", "extra", "a phone paired before 1.25, whose key crossed the Wi-Fi in clear (FXSEC B1), which the Swift engine never had", "stays: Go-only, the page's Paired phones; the Mac runs the Go core"},
	{"setup-state", ".phones[].pairedAgain", "extra", "a phone of its name paired since, so setup offers to remove the old entry (FXSEC B1)", "stays: Go-only, the page's Paired phones; the Mac runs the Go core"},
	{"setup-state", ".summary.noStore", "extra", "says there's no secret store for the key, which a Mac always has", "stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false"},
	{"setup-state-tool", ".hiring.agentsChecked", "extra", "whether this computer checks agents' programs (a Mac checks Claude Code's signature, RT126SG2 B2), which the Swift engine never did", "stays: Go-only, the page's line under Let the office hire and fire; the Mac runs the Go core"},
	{"setup-state-tool", ".hiring.claudeUnsigned", "extra", "an unsigned Claude Code refused after a signed one ran, for the owner to confirm (RT126SG2 B2), which the Swift engine never had", "stays: Go-only, the page's alone to confirm; the Mac runs the Go core"},
	{"setup-state-tool", ".hiring.managedBeta", "extra", "the launcher's hire beta (Windows)", "stays: Go-only, the launcher's hire beta; the page doesn't read it"},
	{"setup-state-tool", ".hiring.managedWindows", "extra", "the launcher's hire beta (Windows)", "stays: Go-only, the launcher's hire beta; the page doesn't read it"},
	{"setup-state-tool", ".mailBeta", "extra", "mail to a hire is beta on Windows", "stays: Go-only, the page doesn't read it"},
	{"setup-state-tool", ".pairing.codeExpiresIn", "extra", "a tool is told how long the code lasts; the Mac tells only the page", "stays: Go-only, a tool's; the page has it in its own state on both apps"},
	{"setup-state-tool", ".wifiTyping", "extra", "Typing over Wi-Fi (SECWIFI), which the Swift engine never had", "stays: Go-only, the page's switch under Paired phones; the Mac runs the Go core"},
	{"setup-state-tool", ".phones[].beforeSigning", "extra", "a phone paired before 1.25, whose key crossed the Wi-Fi in clear (FXSEC B1), which the Swift engine never had", "stays: Go-only, the page's Paired phones; the Mac runs the Go core"},
	{"setup-state-tool", ".phones[].pairedAgain", "extra", "a phone of its name paired since, so setup offers to remove the old entry (FXSEC B1)", "stays: Go-only, the page's Paired phones; the Mac runs the Go core"},
	{"setup-state-tool", ".summary.noStore", "extra", "says there's no secret store for the key, which a Mac always has", "stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false"},
	{"GET /setup/gm/approvals", ".asks", "extra", "the asks a GM, an MCP key, a night job or a hire waits on, for the owner to answer (1.26 FS2), beside the sessions asking", "stays: Go-only; a reader that doesn't know the key leaves it alone, and the owner's asks are this app's record (grants.go)"},
}

// contractAsk is a fixture's request: who asks, its query, headers and body, and what's set up first.
type contractAsk struct {
	As      string            `json:"as"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	// Headers (named in Headers) that come twice in the request.
	Repeated []string `json:"repeated"`
	Body     any      `json:"body"`
	Given    []string `json:"given"`
}

type contractRoute struct {
	Route   string       `json:"route"`
	Callers []string     `json:"callers"`
	Request *contractAsk `json:"request"`
	Answer  *struct {
		Status int    `json:"status"`
		Kind   string `json:"kind"`
		Shape  any    `json:"shape"`
	} `json:"answer"`
	Refusals []struct {
		Request contractAsk `json:"request"`
		Status  int         `json:"status"`
		Kind    string      `json:"kind"`
		Shape   any         `json:"shape"` // a refusal in JSON's
		Why     string      `json:"why"`
	} `json:"refusals"`
}

// contractStep is one request a given is: as a caller, to a route, with its query, headers and body.
type contractStep struct {
	contractAsk
	Method string `json:"method"`
	Path   string `json:"path"`
}

// contractGiven is what a given sets up: requests both apps answer, a file in the support folder before the
// app reads it, or what each app does its own way (says).
type contractGiven struct {
	Requests []contractStep `json:"requests"`
	File     string         `json:"file"`
	Content  any            `json:"content"`
	Says     string         `json:"says"`
}

type contract struct {
	shapes map[string]any                 // by name: what a route's answer refers to
	ids    map[string]map[string][]string // by name: the given's sessions in each of the shape's lists, by where
	t3Desk map[string]*t3DeskRule         // by name: what a t3: desk in it must hold by value
	givens map[string]contractGiven
	files  map[string][]contractRoute
}

var contractFiles = []string{"state.json", "setup.json", "gm.json", "office.json"}

func contractFolder() string { return filepath.Join("..", "Tests", "fixtures", "contract") }

func readContract(t *testing.T) contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractFolder(), "contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var shared struct {
		Shapes map[string]struct {
			Shape  any
			IDs    map[string][]string `json:"ids"`
			T3Desk *t3DeskRule         `json:"t3Desk"`
		} `json:"shapes"`
		Given map[string]contractGiven `json:"given"`
	}
	if err := json.Unmarshal(data, &shared); err != nil {
		t.Fatal(err)
	}
	c := contract{shapes: map[string]any{}, ids: map[string]map[string][]string{}, t3Desk: map[string]*t3DeskRule{}, givens: shared.Given,
		files: map[string][]contractRoute{}}
	for name, s := range shared.Shapes {
		c.shapes[name] = s.Shape
		c.ids[name] = s.IDs
		if s.T3Desk != nil {
			c.t3Desk[name] = s.T3Desk
		}
	}
	for _, file := range contractFiles {
		data, err := os.ReadFile(filepath.Join(contractFolder(), file))
		if err != nil {
			t.Fatal(err)
		}
		var group struct{ Routes []contractRoute }
		if err := json.Unmarshal(data, &group); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		c.files[file] = group.Routes
	}
	if len(c.shapes) < 3 || len(c.givens["sessions"].Requests) == 0 || len(c.files["setup.json"]) < 30 {
		t.Fatalf("the contract has %d shapes, %d hooks, %d setup routes: not the files this test knows", len(c.shapes),
			len(c.givens["sessions"].Requests), len(c.files["setup.json"]))
	}
	return c
}

// MARK: Shapes, compared as ContractTests.swift compares them

// shapeDifferences is where got (an answer, as JSON) isn't as want (a shape) says, as "where: what" lines, the
// Mac's way: "string", "number", "bool", "null", 'x' for that string, alternatives joined by |; an object's keys,
// no more, no fewer; an array's elements as the different shapes they have, each element like one of them and
// each like some element ([] for none). Nothing stands for anything but "any", "object", "map" and "array": a
// null, an empty array or object where the shape has something else is a difference.
func shapeDifferences(want, got any, path string) []string {
	here := path
	if here == "" {
		here = "."
	}
	switch w := want.(type) {
	case string:
		alternatives := strings.Split(w, "|")
		literal := false
		for _, alternative := range alternatives {
			if shapeMatches(alternative, got) {
				return nil
			}
			literal = literal || strings.HasPrefix(alternative, "'")
		}
		return []string{here + ": " + describeValue(got, literal) + ", want " + w}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return []string{here + ": " + describeValue(got, false) + ", want object"}
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		var found []string
		for _, k := range sorted {
			wv, inWant := w[k]
			gv, inGot := g[k]
			switch {
			case inWant && inGot:
				found = append(found, shapeDifferences(wv, gv, path+"."+k)...)
			case inWant:
				found = append(found, path+"."+k+": missing")
			default:
				found = append(found, path+"."+k+": extra")
			}
		}
		return found
	case []any:
		elements, ok := got.([]any)
		if !ok {
			return []string{here + ": " + describeValue(got, false) + ", want array"}
		}
		if len(w) == 0 {
			if len(elements) == 0 {
				return nil
			}
			return []string{here + ": some, want none"}
		}
		if len(elements) == 0 {
			return []string{here + ": none, want some"}
		}
		var found []string
		add := func(lines []string) {
			for _, line := range lines {
				if !slices.Contains(found, line) {
					found = append(found, line)
				}
			}
		}
		// Each element like one of the Mac's; then, if each is, each of the Mac's like one of them. Where none
		// is, the closest says how: an array empty on one side only counts as far.
		far := func(lines []string) int {
			n := 0
			for _, line := range lines {
				if strings.HasSuffix(line, ": some, want none") || strings.HasSuffix(line, ": none, want some") {
					n += 10
				} else {
					n++
				}
			}
			return n
		}
		closest := func(each [][]string) []string {
			best := each[0]
			for _, lines := range each {
				if len(lines) == 0 {
					return nil
				}
				if far(lines) < far(best) {
					best = lines
				}
			}
			return best
		}
		for _, element := range elements {
			each := make([][]string, len(w))
			for i, variant := range w {
				each[i] = shapeDifferences(variant, element, path+"[]")
			}
			add(closest(each))
		}
		if len(found) == 0 {
			for _, variant := range w {
				each := make([][]string, len(elements))
				for i, element := range elements {
					each[i] = shapeDifferences(variant, element, path+"[]")
				}
				add(closest(each))
			}
		}
		return found
	}
	return []string{here + ": a shape this test doesn't know"}
}

func shapeMatches(alternative string, got any) bool {
	switch alternative {
	case "any":
		return true
	case "null":
		return got == nil
	case "string":
		_, ok := got.(string)
		return ok
	case "bool":
		_, ok := got.(bool)
		return ok
	case "number":
		_, ok := got.(float64)
		return ok
	case "object", "map":
		_, ok := got.(map[string]any)
		return ok
	case "array":
		_, ok := got.([]any)
		return ok
	}
	value, ok := strings.CutPrefix(alternative, "'")
	if value, closed := strings.CutSuffix(value, "'"); ok && closed {
		return got == value
	}
	return false
}

// describeValue is what a value is, said in a difference: its type, the string itself among strings of a closed
// set, {} and [] when empty.
func describeValue(got any, literal bool) string {
	switch g := got.(type) {
	case nil:
		return "null"
	case string:
		if literal {
			return "'" + g + "'"
		}
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	case map[string]any:
		if len(g) == 0 {
			return "{}"
		}
		return "object"
	case []any:
		if len(g) == 0 {
			return "[]"
		}
		return "array"
	}
	return "something else"
}

func TestShapesAreKeysTypesAndClosedSetsAsOnTheMac(t *testing.T) {
	parse := func(text string) any {
		var value any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, c := range []struct {
		want, got string
		lines     []string
	}{
		{`{"a": "number", "b": "string"}`, `{"a": "x", "c": true}`, []string{".a: string, want number", ".b: missing", ".c: extra"}},
		{`{"s": "'idle'|'asking'"}`, `{"s": "not-a-state"}`, []string{".s: 'not-a-state', want 'idle'|'asking'"}},
		{`{"s": "string|null"}`, `{"s": null}`, nil},
		// Nothing empty stands for anything.
		{`{"a": ["string"], "g": "number"}`, `{"a": [], "g": null}`, []string{".a: none, want some", ".g: null, want number"}},
		{`{"o": {"k": "bool"}}`, `{"o": {}}`, []string{".o.k: missing"}},
		{`{"o": {"k": "bool"}}`, `{"o": null}`, []string{".o: null, want object"}},
		{`[]`, `["x"]`, []string{".: some, want none"}},
		// Each element like one of the Mac's, and each of the Mac's like one of them.
		{`[{"x": "bool"}]`, `[{"x": 1}]`, []string{"[].x: number, want bool"}},
		{`[{"x": "bool"}, "null"]`, `[{"x": true}]`, []string{"[]: object, want null"}},
		{`["map", "any"]`, `[{}, 3]`, nil},
	} {
		if got := shapeDifferences(parse(c.want), parse(c.got), ""); !slices.Equal(got, c.lines) {
			t.Errorf("%s against %s: %q, want %q", c.got, c.want, got, c.lines)
		}
	}
	// An answer of {} or null, or with each field null, never passes the setup state or the snapshot.
	c := readContract(t)
	for _, name := range []string{"setup-state", "setup-state-tool", "snapshot"} {
		shape := c.shapes[name].(map[string]any)
		if len(shapeDifferences(shape, map[string]any{}, "")) < len(shape) || len(shapeDifferences(shape, nil, "")) != 1 {
			t.Errorf("%s: {} or null passes", name)
		}
		nulled, nullable := map[string]any{}, 0
		for key, field := range shape {
			nulled[key] = nil
			if s, ok := field.(string); ok && slices.Contains(strings.Split(s, "|"), "null") {
				nullable++
			}
		}
		if got := len(shapeDifferences(shape, nulled, "")); got != len(shape)-nullable {
			t.Errorf("%s: each field null gives %d differences, want %d", name, got, len(shape)-nullable)
		}
	}
}

// A desk's fields are typed by what the given fills: a null model, tool or activity at the pinned key (contract-a
// has each) is a difference, though the free key's desk (contract-c) has no model; and so is a pin of false at
// the free key. Asked of this app's own snapshot: each change adds a difference to what it already has.
func TestWhatTheGivenFillsAtADeskMustBeFilled(t *testing.T) {
	c := readContract(t)
	var events *contractRoute
	for i, route := range c.files["state.json"] {
		if route.Route == "GET /events" {
			events = &c.files["state.json"][i]
		}
	}
	if events == nil || events.Request == nil {
		t.Fatal("no GET /events in state.json")
	}
	got := newContractDeck(t, c, events.Request.Given).ask("GET", "/events", *events.Request)
	shape := c.shapes["snapshot"]
	before := shapeDifferences(shape, got.json, "")
	changed := func(change func(keys []any)) any {
		data, _ := json.Marshal(got.json)
		var answer map[string]any
		_ = json.Unmarshal(data, &answer)
		keys, _ := answer["keys"].([]any)
		change(keys)
		return answer
	}
	pinned, free := -1, -1
	answer, _ := got.json.(map[string]any)
	keys, _ := answer["keys"].([]any)
	for i, key := range keys {
		key, _ := key.(map[string]any)
		if _, ok := key["pin"].(string); ok {
			pinned = i
		} else if key["pin"] == nil && key["session"] != nil {
			free = i
		}
	}
	if pinned < 0 || free < 0 {
		t.Fatalf("the snapshot has no pinned key (%d) or no session at a free key (%d)", pinned, free)
	}
	adds := func(what string, answer any) {
		for _, line := range shapeDifferences(shape, answer, "") {
			if !slices.Contains(before, line) {
				return
			}
		}
		t.Errorf("%s passes the snapshot's shape", what)
	}
	for _, field := range []string{"model", "tool", "activity"} {
		adds("a null "+field+" at the pinned key", changed(func(keys []any) {
			keys[pinned].(map[string]any)["session"].(map[string]any)[field] = nil
		}))
	}
	adds("a pin of false at the free key", changed(func(keys []any) { keys[free].(map[string]any)["pin"] = false }))
	adds("a null pin at the pinned key", changed(func(keys []any) { keys[pinned].(map[string]any)["pin"] = nil }))
}

// MARK: Asking this app

type contractAnswer struct {
	status int
	kind   string
	json   any
}

// contractDeck is a test deck with what a fixture's given sets up, and the phone paired with it if asked.
type contractDeck struct {
	t     *testing.T
	d     *Deck
	c     contract
	phone string
	code  bool
}

// newContractDeck is a test deck of its own (isolated), with a given's files put in its folder before it starts,
// then the rest of what it's given, in order.
func newContractDeck(t *testing.T, c contract, given []string) *contractDeck {
	t.Helper()
	isolate(t)
	// The Mac's answers are the Go core's under the Mac app: what only macOS does is asked of a stand-in for the
	// app (hostsetup_test.go), so the darwin answers are proved here on any computer. Without the app, none of
	// it is said (TestWithoutTheMacAppSetupAndTheOfficeHaveNoneOfTheMacs).
	withMac(t)
	for _, name := range given {
		if g, ok := c.givens[name]; !ok {
			t.Fatalf("nothing gives %q", name)
		} else if g.File != "" {
			data, _ := json.Marshal(g.Content)
			if err := os.WriteFile(filepath.Join(supportDir, g.File), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	d := startDeck(t)
	// No coding agent is looked for, as in newTestDeck.
	d.summaryProviders.set(map[summaryProvider]providerState{})
	cd := &contractDeck{t: t, d: d, c: c}
	// The Mac's contract types from a phone on the Wi-Fi (I34): a setup from before Typing over Wi-Fi, which
	// keeps it on. A new setup's, off, is signed_test.go's.
	cd.d.mu.Lock()
	cd.d.config.PhoneTextWiFi = true
	cd.d.mu.Unlock()
	for _, name := range given {
		switch name {
		case "manager on":
			managerOn(cd.d)
		case "T3 Code on":
			cd.d.mu.Lock()
			cd.d.config.T3CodeEnabled = true
			cd.d.mu.Unlock()
		case "a paired phone":
			cd.phoneKey()
		case "the phone mic off":
			// config.json's phoneMicEnabled, which the Go core reads too since I5.
			cd.d.mu.Lock()
			cd.d.config.PhoneMicEnabled = false
			cd.d.mu.Unlock()
		case "the app's briefs":
			// Built in (bundledBriefs), and copied into the library you edit on first use: nothing to set up.
		case "sessions":
			// contract-a's tab is a hire's launcher, opened with the role its hooks will have.
			if cd.d.launcherConnected("contract-a", Claude, "/work/alpha", "reviewer", "") == nil {
				t.Fatal("contract-a's launcher")
			}
		}
		for _, step := range c.givens[name].Requests {
			if got := cd.ask(step.Method, step.Path, step.contractAsk); got.status >= 300 {
				t.Fatalf("%s: %s %s answered %d", name, step.Method, step.Path, got.status)
			}
		}
		if name == "a pairing code" {
			cd.code = true
		}
	}
	return cd
}

// phoneKey pairs a phone, once, as the Mac's test does: with a code open before, one is open after.
func (cd *contractDeck) phoneKey() string {
	if cd.phone == "" {
		cd.phone = pair(cd.t, cd.d)
		if cd.code {
			cd.ask("POST", "/setup/pair", contractAsk{As: "page"})
		}
	}
	return cd.phone
}

// request is a fixture's request as the caller it names sends it.
func (cd *contractDeck) request(method, path string, a contractAsk) *http.Request {
	query := url.Values{}
	for k, v := range a.Query {
		query.Set(k, v)
	}
	host, from := "localhost:7710", thisComputer
	headers := map[string]string{}
	switch a.As {
	case "page":
		cd.d.mu.Lock()
		headers["Cookie"] = setupCookie() + "=" + cd.d.access.NewSetupSession()
		cd.d.mu.Unlock()
	case "tool":
		headers[setupKeyHeader] = cd.d.access.SetupKey
	case "hook":
		headers[hookKeyHeader] = cd.d.access.HookKey
	case "phone":
		host, from = deckOnWiFi, aPhone
		query.Set("t", cd.phoneKey())
	case "nobody":
		if !strings.HasPrefix(path, "/setup") && !strings.HasPrefix(path, "/bots/") && path != "/hook" {
			host, from = deckOnWiFi, aPhone
		}
	default:
		cd.t.Fatalf("no caller %q", a.As)
	}
	if method != http.MethodGet {
		headers[setupHeader] = "1"
	}
	// Setup lives on 127.0.0.1 alone (server.go): a browser, a tool and the GM ask it there.
	if host == "localhost:7710" && isSetupRoute(path) {
		host = "127.0.0.1:7710"
	}
	for k, v := range a.Headers {
		headers[k] = v
	}
	var body string
	switch b := a.Body.(type) {
	case nil:
	case string:
		body = b
	default:
		// "$code" is the pairing code asked for.
		// It's typed on this computer: over the Wi-Fi, a typed code is refused, and the QR code pairs (SECWIFI).
		if object, ok := b.(map[string]any); ok && object["code"] == "$code" {
			cd.d.mu.Lock()
			code := cd.d.access.ActiveCode()
			cd.d.mu.Unlock()
			b = map[string]any{"code": code}
			host, from = "localhost:7710", thisComputer
		}
		data, _ := json.Marshal(b)
		body = string(data)
	}
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	// A phone on the Wi-Fi signs what it asks, as the deck does (signed.go).
	if a.As == "phone" {
		target = signedTarget(method, target, body, time.Now())
	}
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host, r.RemoteAddr = host, from
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	for _, k := range a.Repeated {
		r.Header.Add(k, headers[k])
	}
	return r
}

func (cd *contractDeck) ask(method, path string, a contractAsk) contractAnswer {
	cd.t.Helper()
	if method == http.MethodGet && path == "/events" {
		return cd.stream(cd.request(method, path, a))
	}
	w := httptest.NewRecorder()
	(&router{deck: cd.d}).ServeHTTP(w, cd.request(method, path, a))
	return answerOf(w.Code, w.Header().Get("Content-Type"), w.Body.Bytes())
}

func answerOf(status int, contentType string, body []byte) contractAnswer {
	answer := contractAnswer{status: status}
	switch {
	case strings.HasPrefix(contentType, "application/json") && json.Unmarshal(body, &answer.json) == nil:
		answer.kind = "json"
	case len(body) == 0:
		answer.kind = "empty"
	case strings.HasPrefix(contentType, "image/png"):
		answer.kind = "png"
	case strings.HasPrefix(contentType, "text/html"):
		answer.kind = "html"
	default:
		answer.kind = "text"
	}
	return answer
}

// contractStream is a response kept open, as /events is, read once its first message is flushed.
type contractStream struct {
	mu      sync.Mutex
	header  http.Header
	status  int
	body    bytes.Buffer
	flushed chan struct{}
	once    sync.Once
}

func (s *contractStream) Header() http.Header { return s.header }
func (s *contractStream) WriteHeader(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}
func (s *contractStream) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.body.Write(data)
}
func (s *contractStream) Flush() { s.once.Do(func() { close(s.flushed) }) }

// stream asks for /events and answers its status, and the first message's JSON: the snapshot.
func (cd *contractDeck) stream(r *http.Request) contractAnswer {
	ctx, cancel := context.WithCancel(r.Context())
	w := &contractStream{header: http.Header{}, flushed: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&router{deck: cd.d}).ServeHTTP(w, r.WithContext(ctx))
	}()
	select {
	case <-w.flushed:
	case <-done:
	case <-time.After(5 * time.Second):
		cd.t.Error("/events sent nothing in 5 seconds")
	}
	cancel()
	<-done
	w.mu.Lock()
	defer w.mu.Unlock()
	if !strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream") {
		return answerOf(w.status, w.header.Get("Content-Type"), w.body.Bytes())
	}
	answer := contractAnswer{status: w.status, kind: "event-stream"}
	line, _ := bufio.NewReader(&w.body).ReadString('\n')
	if data, ok := strings.CutPrefix(strings.TrimSpace(line), "data: "); ok {
		_ = json.Unmarshal([]byte(data), &answer.json)
	}
	return answer
}

// MARK: The comparison

// isSetupRoute is whether setup's table (frontdesk/routes.json, routeTable here) says who may call a path.
func isSetupRoute(path string) bool {
	return path == "/setup" || strings.HasPrefix(path, "/setup/") || strings.HasPrefix(path, "/bots/")
}

// contractGaps asks this app every fixture's requests and answers where it differs from the Mac, as gaps
// without their why. Each request is asked of a deck of its own.
func contractGaps(t *testing.T, c contract) []knownGap {
	// Nothing here starts a hire's window (no request reaches one, but a slip must not open a terminal).
	saved := startHire
	t.Cleanup(func() { startHire = saved })
	startHire = func(*exec.Cmd) error { return nil }
	// Linux and Windows serve Tailscale only: the pairing code's QR needs an address a phone could open, so one is
	// given, whether or not this computer has Tailscale.
	addresses := []string{"100.101.102.103"}
	withTailscale(t, &addresses)
	var gaps []knownGap
	seen := map[knownGap]bool{}
	add := func(g knownGap) {
		if !seen[g] {
			seen[g] = true
			gaps = append(gaps, g)
		}
	}
	for _, file := range contractFiles {
		for _, route := range c.files[file] {
			method, path, _ := strings.Cut(route.Route, " ")
			ask := func(a contractAsk) contractAnswer {
				return newContractDeck(t, c, a.Given).ask(method, path, a)
			}
			var found []knownGap
			// Setup's callers are its table's: the same here as on the Mac.
			if isSetupRoute(path) {
				callers := []string{"page", "tool"}
				if row := routeFor(method, path); row != nil {
					callers = row.callers
				}
				if !slices.Equal(callers, route.Callers) {
					found = append(found, knownGap{subject: route.Route, where: "callers", got: strings.Join(callers, ", ")})
				}
			}
			macServes, goServes := false, false
			if route.Request != nil && route.Answer != nil {
				got := ask(*route.Request)
				macServes = macServes || route.Answer.Status != http.StatusNotFound
				goServes = goServes || got.status != http.StatusNotFound
				switch {
				case got.status != route.Answer.Status:
					found = append(found, knownGap{subject: route.Route, where: "status", got: strconv.Itoa(got.status)})
				case got.kind != route.Answer.Kind:
					found = append(found, knownGap{subject: route.Route, where: "kind", got: got.kind})
				case route.Answer.Shape != nil:
					subject, want := route.Route, route.Answer.Shape
					if name, ok := want.(string); ok {
						subject, want = name, c.shapes[name]
					}
					for _, line := range shapeDifferences(want, got.json, "") {
						where, what, _ := strings.Cut(line, ": ")
						found = append(found, knownGap{subject: subject, where: where, got: what})
					}
					// The given's sessions, each in every list it's in: a shape alone passes a list that drops one.
					for where, ids := range c.ids[subject] {
						if have := valuesAt(where, got.json); !slices.Equal(have, ids) {
							found = append(found, knownGap{subject: subject, where: where, got: "ids " + strings.Join(have, ", ")})
						}
					}
					// A hire in T3 Code's desk, by value: run on every t3: desk this app answers, whatever else differs
					// (the shape's own pass from the Mac's side is skipped while any line is found).
					if rule := c.t3Desk[subject]; rule != nil {
						for _, line := range t3DeskDifferences(*rule, got.json) {
							where, what, _ := strings.Cut(line, ": ")
							found = append(found, knownGap{subject: subject, where: where, got: what})
						}
					}
				}
			}
			for _, refusal := range route.Refusals {
				got := ask(refusal.Request)
				// A stranger is turned away at the gate, before any route is looked for: that says nothing of the route.
				if refusal.Why != "not one of its callers" {
					macServes = macServes || refusal.Status != http.StatusNotFound
					goServes = goServes || got.status != http.StatusNotFound
				}
				switch {
				case got.status != refusal.Status:
					found = append(found, knownGap{subject: route.Route, where: "refusal: " + refusal.Why, got: "status " + strconv.Itoa(got.status)})
				case got.kind != refusal.Kind:
					found = append(found, knownGap{subject: route.Route, where: "refusal: " + refusal.Why, got: "kind " + got.kind})
				case refusal.Shape != nil:
					for _, line := range shapeDifferences(refusal.Shape, got.json, "") {
						where, what, _ := strings.Cut(line, ": ")
						found = append(found, knownGap{subject: route.Route, where: "refusal: " + refusal.Why + ", " + where, got: what})
					}
				}
			}
			// A route that answers nothing but 404 here, where the Mac answers it, isn't served: one gap, not one per request.
			if macServes && !goServes {
				found = []knownGap{{subject: route.Route, where: "route", got: "404"}}
			}
			for _, g := range found {
				add(g)
			}
		}
	}
	for _, g := range macDictationGaps(t, c) {
		add(g)
	}
	return gaps
}

// macDictationGaps is the office's dictation under the Mac app, which no fixture shows: the Mac's own test
// can't count on its speech recognition and microphone. With both (the stand-in app has them, learned as the
// deck learns them, from its facts), and dictation on for Claude Code too, the Mac's rule (Deck.dictates,
// Dictation.isOn) has the snapshot's dictation on and a Claude desk in a Terminal tab that types text dictate.
// A gap if this app's mic doesn't (closed in M7GAPA, hostdictation.go). Not contract-a, in a launcher's window:
// the Mac had no launchers, and this app keeps them out of dictation (RGAPA B2).
func macDictationGaps(t *testing.T, c contract) []knownGap {
	cd := newContractDeck(t, c, []string{"sessions"})
	d := cd.d
	d.mu.Lock()
	mac, _ := d.mac.host.(*macStandIn)
	d.mu.Unlock()
	if mac == nil {
		t.Fatal("no stand-in Mac app")
	}
	mac.mu.Lock()
	mac.facts.Dictation.Available, mac.facts.Dictation.Allowed = true, true
	mac.mu.Unlock()
	d.macFactsNow()
	const tab = "tty:/dev/ttys009"
	d.mu.Lock()
	d.config.DictationEnabled, d.config.DictationForClaude = true, true
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "contract-tty", "cwd": "/work/alpha"}, Claude, tab, "", 100)
	d.mac.abilities, d.mac.abilitiesFor = map[string]tabAbilities{tab: {Keys: true, Text: true, Fire: true}}, tab
	var state struct {
		Dictation struct{ On bool }
		Keys      []struct{ Session map[string]any }
	}
	_ = json.Unmarshal(d.snapshot(), &state)
	d.mu.Unlock()
	dictates := false
	for _, key := range state.Keys {
		if key.Session["id"] == "contract-tty" {
			dictates = key.Session["dictates"] == true
		}
	}
	if state.Dictation.On && dictates {
		return nil
	}
	return []knownGap{{subject: "POST /act", where: "mic: dictation on, under the Mac app", got: "not dictated"}}
}

// valuesAt is the values at where (".a[].b") in an answer, every element's, as text, sorted: as the Mac writes ids.
func valuesAt(where string, answer any) []string {
	texts := []string{}
	for _, value := range foundAt(where, answer) {
		if text, ok := value.(string); ok {
			texts = append(texts, text)
		} else {
			data, _ := json.Marshal(value)
			texts = append(texts, string(data))
		}
	}
	sort.Strings(texts)
	return texts
}

// foundAt is the values at where (".a[].b") in an answer, every element's, in order.
func foundAt(where string, answer any) []any {
	found := []any{answer}
	for rest := where; rest != ""; {
		if after, ok := strings.CutPrefix(rest, "[]"); ok {
			var elements []any
			for _, value := range found {
				list, _ := value.([]any)
				elements = append(elements, list...)
			}
			found, rest = elements, after
			continue
		}
		rest = strings.TrimPrefix(rest, ".")
		end := strings.IndexAny(rest, ".[")
		if end < 0 {
			end = len(rest)
		}
		key := rest[:end]
		var values []any
		for _, value := range found {
			if object, ok := value.(map[string]any); ok {
				if v, has := object[key]; has {
					values = append(values, v)
				}
			}
		}
		found, rest = values, rest[end:]
	}
	return found
}

// t3DeskRule is what every t3: desk in a snapshot must hold by value, as the Mac writes it beside the shape
// (FXT3Da N1): a shape alone passes a desk with no t3, or with its controls on, as like a plain desk's.
type t3DeskRule struct {
	ID      string   `json:"id"`
	False   []string `json:"false"`
	Null    []string `json:"null"`
	T3      []string `json:"t3"`
	NoSlash []string `json:"noSlash"`
}

// t3DeskDifferences is where a t3: desk in a snapshot (at a key, on a floor, upstairs) isn't as rule says, as
// "where: what" lines, as the Mac's ContractTests.t3DeskDifferences finds them: each false field false, each
// null field null, its t3 with each of those fields, no / in a noSlash field. None where there's no such desk:
// the ids say whether there should be one.
func t3DeskDifferences(rule t3DeskRule, snapshot any) []string {
	var found []string
	for _, list := range []string{".keys[].session", ".floors[].desks[]", ".upstairs.desks[]"} {
		for _, value := range foundAt(list, snapshot) {
			desk, _ := value.(map[string]any)
			id, _ := desk["id"].(string)
			if !strings.HasPrefix(id, "t3:") {
				continue
			}
			here := list + "[" + id + "]"
			shown := func(field string) string {
				v, has := desk[field]
				if !has {
					return "missing"
				}
				data, _ := json.Marshal(v)
				return string(data)
			}
			for _, field := range rule.False {
				if v, ok := desk[field].(bool); !ok || v {
					found = append(found, here+"."+field+": "+shown(field)+", want false")
				}
			}
			for _, field := range rule.Null {
				if v, has := desk[field]; !has || v != nil {
					found = append(found, here+"."+field+": "+shown(field)+", want null")
				}
			}
			t3, _ := desk["t3"].(map[string]any)
			for _, field := range rule.T3 {
				if t3[field] == nil {
					found = append(found, here+".t3."+field+": missing, want a value")
				}
			}
			for _, field := range rule.NoSlash {
				if v, ok := desk[field].(string); !ok || strings.Contains(v, "/") {
					found = append(found, here+"."+field+": "+shown(field)+", want a name with no /")
				}
			}
		}
	}
	return found
}

func gapKey(g knownGap) string { return g.subject + " | " + g.where + " | " + g.got }

// The Mac's rule for a hire in T3 Code's desk holds a t3: desk to its values: its controls off, no choice, its t3
// tag, status, branch and whether it opens, and names with no path. A desk that isn't a T3 hire's isn't held to it.
func TestAT3DeskIsCheckedByValueToo(t *testing.T) {
	rule := readContract(t).t3Desk["snapshot"]
	// Its t3 says it's experimental too (RT3Da2 n3), so a desk without the word fails.
	if rule == nil || !strings.HasPrefix(rule.ID, "t3:") || len(rule.False) == 0 || !slices.Contains(rule.T3, "experimental") {
		t.Fatalf("no T3 desk rule beside the snapshot's shape: %+v", rule)
	}
	good := map[string]any{"id": rule.ID, "name": "rev-c0ffee", "project": "rev-c0ffee"}
	for _, field := range rule.False {
		good[field] = false
	}
	for _, field := range rule.Null {
		good[field] = nil
	}
	t3 := map[string]any{}
	for _, field := range rule.T3 {
		t3[field] = "x"
	}
	good["t3"] = t3
	snapshot := func(desk map[string]any) any {
		return map[string]any{"keys": []any{map[string]any{"session": desk}}, "floors": []any{}, "upstairs": map[string]any{"desks": []any{}}}
	}
	if got := t3DeskDifferences(*rule, snapshot(good)); len(got) != 0 {
		t.Fatalf("a T3 desk as the Mac's: %v", got)
	}
	bad := map[string]any{}
	for k, v := range good {
		bad[k] = v
	}
	bad["keys"] = true
	delete(bad, rule.False[0])
	bad["choice"] = map[string]any{"options": []any{"Yes"}}
	bad["t3"] = map[string]any{"tag": "T3 Code"}
	bad["project"] = "/work/delta-hires/rev-c0ffee"
	var wheres []string
	for _, line := range t3DeskDifferences(*rule, snapshot(bad)) {
		where, _, _ := strings.Cut(line, ": ")
		wheres = append(wheres, where)
	}
	sort.Strings(wheres)
	want := []string{}
	fields := []string{"keys", rule.False[0], "choice", "project"}
	for _, field := range rule.T3 {
		if field != "tag" {
			fields = append(fields, "t3."+field)
		}
	}
	for _, field := range fields {
		want = append(want, ".keys[].session["+rule.ID+"]."+field)
	}
	sort.Strings(want)
	want = slices.Compact(want)
	if !slices.Equal(wheres, want) {
		t.Fatalf("a T3 desk with a control on, a choice, no t3 but its tag and a path: %v, want %v", wheres, want)
	}
	other := map[string]any{"id": "contract-x", "keys": true, "project": "/a/b"}
	if got := t3DeskDifferences(*rule, snapshot(other)); len(got) != 0 {
		t.Fatalf("a desk that isn't a T3 hire's: %v", got)
	}
}

// Every fixture asked of this app: what differs from the Mac must be a known gap, and every known gap must
// still differ, so a gap closed by an M piece is taken out of the table (and INVENTORY.md) with it.
func TestThisAppAnswersAsTheMacsContractSaysButForItsKnownGaps(t *testing.T) {
	found := contractGaps(t, readContract(t))
	known := map[string]bool{}
	for _, g := range knownGaps {
		known[gapKey(g)] = true
	}
	differs := map[string]bool{}
	for _, g := range found {
		differs[gapKey(g)] = true
		if !known[gapKey(g)] {
			t.Errorf("not a known gap: {%q, %q, %q, \"<why>\", \"<owner>\"},", g.subject, g.where, g.got)
		}
	}
	for _, g := range knownGaps {
		if !differs[gapKey(g)] {
			t.Errorf("no longer a gap: %s (take it out of knownGaps and INVENTORY.md)", gapKey(g))
		}
	}
}

// MARK: INVENTORY.md

// inventoryRoute is a row of INVENTORY.md's route tables: the path asked (a pattern's stand-in), and its
// fixture and Go cells.
type inventoryRoute struct{ callers, fixture, goStatus string }

// inventory is what INVENTORY.md says: its routes, its known gaps, and its field tables (field, type, Go).
type inventory struct {
	routes map[string]inventoryRoute
	gaps   []knownGap
	fields map[string][][3]string // by shape name
}

func readInventory(t *testing.T) inventory {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractFolder(), "INVENTORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	inv := inventory{routes: map[string]inventoryRoute{}, fields: map[string][][3]string{}}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimPrefix(line, "## ")
			continue
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(strings.ReplaceAll(cells[i], "`", ""))
		}
		header := cells[0] == "In" || cells[0] == "Field" || strings.HasPrefix(cells[0], "---")
		switch {
		case header:
		case section == "Known gaps" && len(cells) == 5:
			inv.gaps = append(inv.gaps, knownGap{subject: cells[0], where: cells[1], got: cells[2], why: cells[3], owner: cells[4]})
		case strings.HasPrefix(section, "Fields of") && len(cells) == 3:
			name := "snapshot"
			if strings.Contains(section, "setup-state") {
				name = "setup-state"
			}
			inv.fields[name] = append(inv.fields[name], [3]string{cells[0], cells[1], cells[2]})
		case len(cells) >= 8 && slices.Contains([]string{"GET", "POST", "DELETE", "*"}, cells[0]):
			// Method, path, callers, request, answer, statuses, fixture, Go.
			inv.routes[cells[0]+" "+cells[1]] = inventoryRoute{callers: cells[2], fixture: cells[6], goStatus: cells[7]}
		}
	}
	return inv
}

// inventoryRow is the INVENTORY.md row a fixture's route is: its own, or the pattern it stands in for.
func inventoryRow(route string) string {
	switch {
	case route == "GET /setup/gm/nothing-here":
		return "* /setup/gm/*"
	case strings.HasPrefix(route, "GET /setup/logo/"):
		return "GET /setup/logo/<logo>"
	case strings.HasPrefix(route, "POST /bots/"):
		return "POST /bots/<id>/event"
	}
	return route
}

// knownGaps is INVENTORY.md's "Known gaps" table, row for row; and each route with a fixture says in its Go
// column what the gaps say: 404 (not served), differs (a gap in it or in the shape it answers), or served.
func TestTheKnownGapsAreInventorysGoStatus(t *testing.T) {
	inv := readInventory(t)
	if len(inv.routes) < 100 {
		t.Fatalf("INVENTORY.md has %d routes: not the table this test knows", len(inv.routes))
	}
	if len(inv.gaps) != len(knownGaps) {
		t.Errorf("INVENTORY.md has %d known gaps, knownGaps %d", len(inv.gaps), len(knownGaps))
	}
	for i := 0; i < len(inv.gaps) && i < len(knownGaps); i++ {
		if inv.gaps[i] != knownGaps[i] {
			t.Errorf("gap %d: INVENTORY.md has %+v, knownGaps %+v", i+1, inv.gaps[i], knownGaps[i])
		}
	}
	// Every gap names who closes it, so none is a Mac feature that goes quietly when the Mac switches to this core.
	for _, g := range knownGaps {
		if problem := ownerProblem(g.owner, g.why); problem != "" {
			t.Errorf("%s: owner %q %s", gapKey(g), g.owner, problem)
		}
	}
	c := readContract(t)
	status := map[string]string{}
	for _, file := range contractFiles {
		for _, route := range c.files[file] {
			if strings.Contains(route.Route, " /nothing-here") || route.Route == "GET /setup/nothing-here" {
				continue
			}
			row := inventoryRow(route.Route)
			named := ""
			if route.Answer != nil {
				named, _ = route.Answer.Shape.(string)
			}
			if _, ok := status[row]; !ok {
				status[row] = "served"
			}
			for _, g := range knownGaps {
				switch {
				case g.subject == route.Route && g.where == "route":
					status[row] = "404"
				case (g.subject == route.Route || g.subject == named) && status[row] != "404":
					status[row] = "differs"
				}
			}
		}
	}
	for route, want := range status {
		row, ok := inv.routes[route]
		switch {
		case !ok:
			t.Errorf("%s has a fixture but no row in INVENTORY.md", route)
		// "served" may say where, as the Mac-only rows do: "served (under the Mac app, …; 404 on Linux and Windows)".
		case row.goStatus != want && !(want == "served" && strings.HasPrefix(row.goStatus, "served (")):
			t.Errorf("%s: INVENTORY.md says Go %q, the known gaps say %q", route, row.goStatus, want)
		case !strings.Contains(row.fixture, ".json"):
			t.Errorf("%s: INVENTORY.md names no fixture", route)
		}
	}
	// A route without a fixture says why. One INVENTORY.md says this app doesn't serve ("404, owner: <who>") names
	// who serves it, and answers 404 here (with the manager on, as the GM's routes are 409 before anything else
	// with it off). One it says this app serves is served: a file of the web folder is asked for, as reading it
	// changes nothing; a setup route has its callers in the route table, and its case in the router (asking it
	// would write the home folder, open a window, or ask the Keychain).
	cd := newContractDeck(t, c, []string{"manager on"})
	handlers := goSources(t)
	for name, row := range inv.routes {
		if _, has := status[name]; has {
			continue
		}
		if !strings.HasPrefix(row.fixture, "none: ") || len(row.fixture) < len("none: ")+10 {
			t.Errorf("%s: no fixture, and INVENTORY.md says no why (a fixture cell \"none: <why>\")", name)
		}
		method, path, _ := strings.Cut(name, " ")
		switch {
		case strings.HasPrefix(row.goStatus, "404"):
			owner, named := strings.CutPrefix(row.goStatus, "404, owner: ")
			if !named {
				t.Errorf("%s: INVENTORY.md says Go 404, and names no owner (\"404, owner: <who>\")", name)
			} else if problem := ownerProblem(owner, row.fixture); problem != "" {
				t.Errorf("%s: owner %q %s", name, owner, problem)
			}
			path = strings.NewReplacer("<name>", "contract", "<logo>", "keys", "<id>", "contract-bot", "*", "contract").Replace(path)
			caller := "page"
			if !isSetupRoute(path) {
				caller = "phone"
			}
			if got := cd.ask(method, path, contractAsk{As: caller}); got.status != http.StatusNotFound {
				t.Errorf("%s: INVENTORY.md says Go 404, but it answers %d", name, got.status)
			}
		case strings.HasPrefix(row.goStatus, "served"):
			if method == http.MethodGet && strings.Contains(row.fixture, "a file of the app's web folder") {
				path = strings.Replace(path, "/sounds/*", "/sounds/keys.m4a", 1)
				caller := "phone"
				if isSetupRoute(path) {
					caller = "page"
				}
				if got := cd.ask(method, path, contractAsk{As: caller}); got.status == http.StatusNotFound {
					t.Errorf("%s: INVENTORY.md says Go serves it, but it answers 404", name)
				}
				continue
			}
			if !isSetupRoute(path) {
				t.Errorf("%s: served, with no fixture: this test doesn't know how to check it", name)
				continue
			}
			// Its callers are the route table's, or for a route not in it, 1.23's rule: the page, or a tool.
			callers := []string{"page", "tool"}
			if row := routeFor(method, path); row != nil {
				callers = row.callers
			}
			if path != "/setup" && strings.Join(callers, ", ") != row.callers {
				t.Errorf("%s: INVENTORY.md says callers %s, the route table (routes.go) %s", name, row.callers, strings.Join(callers, ", "))
			}
			if !regexp.MustCompile(`[pP]ath == "` + regexp.QuoteMeta(path) + `"`).MatchString(handlers) {
				t.Errorf("%s: INVENTORY.md says Go serves it, but no case in the router answers path == %q", name, path)
			}
		}
	}
}

// goSources is this app's code, but for its tests and its route table: where the router's cases are.
func goSources(t *testing.T) string {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	var all strings.Builder
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "routes.go" {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(data)
	}
	return all.String()
}

// ownerNames is who may close a gap: an M piece, with what it does or "via the bridge (M1)", or "stays: <why>".
var ownerNames = regexp.MustCompile(`^(M[2-6][ab]?( fixes)?( via the bridge \(M1\)|: .{5,}|, .{5,})?|stays: .{10,})$`)

// ownerProblem is what's wrong with a gap's owner, given its why: none, or something only macOS does that
// doesn't go through the bridge (M1).
func ownerProblem(owner, why string) string {
	switch {
	case !ownerNames.MatchString(owner):
		return "names no M piece (M2 to M6, M4a, M4b) and no \"stays: <why>\""
	}
	for _, macOnly := range []string{"Accessibility", "speech recognition", "In the Dock", "Add smolcorp to Terminal", "Grok Bot app", "Terminal command"} {
		if strings.Contains(why, macOnly) && !strings.HasSuffix(owner, " via the bridge (M1)") {
			return "is something only macOS does: \"<M piece> via the bridge (M1)\""
		}
	}
	return ""
}

// INVENTORY.md's field tables are the shapes' fields, two levels down (a desk's session: three), each with its
// type as the shape says it, and "same" in the Go column, or "missing"/"extra" and the why of the known gap
// in it or above it.
func TestTheFieldTablesAreTheShapes(t *testing.T) {
	inv, c := readInventory(t), readContract(t)
	for _, name := range []string{"setup-state", "snapshot"} {
		want, kinds := map[string][2]string{}, map[string]string{}
		var walk func(shape any, path string, depth int)
		walk = func(shape any, path string, depth int) {
			object, ok := shape.(map[string]any)
			if !ok {
				return
			}
			for key, field := range object {
				here := path + "." + key
				// An array's elements of different shapes say a field's type each: the field has any of them.
				kind := fieldType(field)
				if seen, ok := kinds[here]; ok && seen != kind {
					alternatives := strings.Split(seen, "|")
					for _, alternative := range strings.Split(kind, "|") {
						if !slices.Contains(alternatives, alternative) {
							alternatives = append(alternatives, alternative)
						}
					}
					kind = strings.Join(alternatives, "|")
				}
				kinds[here] = kind
				want[here] = [2]string{saidType(kind), goField(name, here)}
				deeper := depth
				if here == ".keys[].session" {
					deeper--
				}
				if deeper < 1 && here != ".layout" && here != ".theme" && here != ".roles.roles" && here != ".roles.shipped" {
					if variants, ok := field.([]any); ok {
						for _, variant := range variants {
							walk(variant, here+"[]", deeper+1)
						}
					} else {
						walk(field, here, deeper+1)
					}
				}
			}
		}
		walk(c.shapes[name], "", 0)
		// Fields only this app has are the known gaps' extras, not rows here.
		got := map[string][2]string{}
		for _, row := range inv.fields[name] {
			got[row[0]] = [2]string{row[1], row[2]}
		}
		for field, w := range want {
			if g, ok := got[field]; !ok {
				t.Errorf("%s %s: no row in INVENTORY.md (| `%s` | %s | %s |)", name, field, field, w[0], w[1])
			} else if g != w {
				t.Errorf("%s %s: INVENTORY.md says %q, %q; the shape and gaps say %q, %q", name, field, g[0], g[1], w[0], w[1])
			}
		}
		for field := range got {
			if _, ok := want[field]; !ok {
				t.Errorf("%s %s: a row in INVENTORY.md for no field of the shape", name, field)
			}
		}
	}
}

// saidType is a field's type as INVENTORY.md says it, as a table's cells can't hold a |: "one of 'a', 'b'" for
// a closed set, "string or null" for alternatives.
func saidType(kind string) string {
	if strings.Contains(kind, "'") {
		return "one of " + strings.ReplaceAll(kind, "|", ", ")
	}
	return strings.ReplaceAll(kind, "|", " or ")
}

// fieldType is a field's type: the shape's own words, "object", or "array" ([] for none).
func fieldType(shape any) string {
	switch s := shape.(type) {
	case string:
		return s
	case map[string]any:
		return "object"
	case []any:
		if len(s) == 0 {
			return "[]"
		}
		return "array"
	}
	return "?"
}

// goField is a field's Go column: "same", or the known gap on it or above it in the shape, as "missing: why".
func goField(name, field string) string {
	for _, g := range knownGaps {
		if g.subject == name && (g.where == field || strings.HasPrefix(field, g.where+".") || strings.HasPrefix(field, g.where+"[]")) &&
			(g.got == "missing" || g.got == "extra") {
			return g.got + ": " + g.why
		}
	}
	return "same"
}
