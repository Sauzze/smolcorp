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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The route contract (M0 of the one-build plan): Tests/fixtures/contract/*.json is what the Mac answers on
// each route, written by the Mac's ContractTests.swift as shapes: the status, the kind of body, and for JSON
// its keys, their types and a closed set's values. Each fixture is asked of this app in process, and its answers checked against
// them. Where this app doesn't answer as the Mac does yet, the difference is a row of knownGaps, and of
// INVENTORY.md's "Known gaps" table, which must say the same: each M piece turns rows into passes, and a row
// that passes must come out of both.

// knownGap is one difference: in what (a route, or a shape by its name in contract.json), where (route: the
// route isn't served; status, kind, a refusal, or a field's path), and what this app has instead.
type knownGap struct{ subject, where, got, why string }

// knownGaps is every difference between the Mac's answers and this app's, today. Keep it in INVENTORY.md's
// order: TestTheKnownGapsAreInventorysGoStatus checks the two are the same table.
var knownGaps = []knownGap{
	{"setup-state", ".checklist.accessibility", "missing", "nothing off the Mac needs Accessibility"},
	{"setup-state", ".claudeAccounts", "missing", "Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2)"},
	{"setup-state", ".cloudBots.autoDetect", "missing", "only the Mac looks in the Grok Bot app"},
	{"setup-state", ".dictation", "missing", "dictation is Apple's speech recognition"},
	{"setup-state", ".dock", "missing", "In the Dock is a Mac setting"},
	{"setup-state", ".floorRules[].exact", "missing", "a rule's exact is left out when false (omitempty)"},
	{"setup-state", ".hiring.apiKey", "missing", "a hire on the API-key account is the Mac's (M4)"},
	{"setup-state", ".hiring.managedBeta", "extra", "the launcher's hire beta (Windows)"},
	{"setup-state", ".hiring.managedWindows", "extra", "the launcher's hire beta (Windows)"},
	{"setup-state", ".keystrokes", "missing", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"setup-state", ".launcher", "extra", "the command that starts an agent through the launcher, which the Mac has no use for"},
	{"setup-state", ".mailBeta", "extra", "mail to a hire is beta on Windows"},
	{"setup-state", ".manager", "missing", "no manager object here: the switch is in config.json, gmHired is top level (M2)"},
	{"setup-state", ".phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"setup-state", ".roles.limits.answer", "missing", "briefs aren't served here (M4)"},
	{"setup-state", ".roles.limits.briefFile", "missing", "briefs aren't served here (M4)"},
	{"setup-state", ".roles.limits.briefFileName", "missing", "briefs aren't served here (M4)"},
	{"setup-state", ".roles.shipped[].brief", "missing", "briefs aren't served here (M4)"},
	{"setup-state", ".summary.noStore", "extra", "says there's no secret store for the key, which a Mac always has"},
	{"setup-state", ".terminalCommand", "missing", "Add smolcorp to Terminal is the Mac's"},
	{"setup-state-tool", ".checklist.accessibility", "missing", "nothing off the Mac needs Accessibility"},
	{"setup-state-tool", ".claudeAccounts", "missing", "Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2)"},
	{"setup-state-tool", ".cloudBots.autoDetect", "missing", "only the Mac looks in the Grok Bot app"},
	{"setup-state-tool", ".dictation", "missing", "dictation is Apple's speech recognition"},
	{"setup-state-tool", ".dock", "missing", "In the Dock is a Mac setting"},
	{"setup-state-tool", ".floorRules[].exact", "missing", "a rule's exact is left out when false (omitempty)"},
	{"setup-state-tool", ".hiring.apiKey", "missing", "a hire on the API-key account is the Mac's (M4)"},
	{"setup-state-tool", ".hiring.managedBeta", "extra", "the launcher's hire beta (Windows)"},
	{"setup-state-tool", ".hiring.managedWindows", "extra", "the launcher's hire beta (Windows)"},
	{"setup-state-tool", ".keystrokes", "missing", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"setup-state-tool", ".launcher", "extra", "the command that starts an agent through the launcher, which the Mac has no use for"},
	{"setup-state-tool", ".mailBeta", "extra", "mail to a hire is beta on Windows"},
	{"setup-state-tool", ".manager", "missing", "no manager object here: the switch is in config.json, gmHired is top level (M2)"},
	{"setup-state-tool", ".pairing.codeExpiresIn", "extra", "a tool is told how long the code lasts; the Mac tells only the page"},
	{"setup-state-tool", ".phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"setup-state-tool", ".roles.limits.answer", "missing", "briefs aren't served here (M4)"},
	{"setup-state-tool", ".roles.limits.briefFile", "missing", "briefs aren't served here (M4)"},
	{"setup-state-tool", ".roles.limits.briefFileName", "missing", "briefs aren't served here (M4)"},
	{"setup-state-tool", ".roles.shipped[].brief", "missing", "briefs aren't served here (M4)"},
	{"setup-state-tool", ".summary.noStore", "extra", "says there's no secret store for the key, which a Mac always has"},
	{"setup-state-tool", ".terminalCommand", "missing", "Add smolcorp to Terminal is the Mac's"},
	{"snapshot", ".dictation", "missing", "dictation is Apple's speech recognition"},
	{"snapshot", ".floors[].desks[].choice", "missing", "a menu's options, read from the Mac's terminals (M3)"},
	{"snapshot", ".floors[].desks[].conversation", "missing", "the conversation view: the iPhone app's (personal build)"},
	{"snapshot", ".floors[].desks[].dictates", "missing", "dictation is Apple's speech recognition"},
	{"snapshot", ".floors[].desks[].fire", "missing", "what the desk's terminal lets the office do: said once here, as hiring.fire (M3)"},
	{"snapshot", ".floors[].desks[].keys", "missing", "what the desk's terminal lets the office do (M3)"},
	{"snapshot", ".floors[].desks[].phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"snapshot", ".floors[].desks[].voiceNote", "missing", "voice notes: the Mac's personal build"},
	{"snapshot", ".keys[].session.choice", "missing", "a menu's options, read from the Mac's terminals (M3)"},
	{"snapshot", ".keys[].session.conversation", "missing", "the conversation view: the iPhone app's (personal build)"},
	{"snapshot", ".keys[].session.dictates", "missing", "dictation is Apple's speech recognition"},
	{"snapshot", ".keys[].session.fire", "missing", "what the desk's terminal lets the office do: said once here, as hiring.fire (M3)"},
	{"snapshot", ".keys[].session.keys", "missing", "what the desk's terminal lets the office do (M3)"},
	{"snapshot", ".keys[].session.phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"snapshot", ".keys[].session.voiceNote", "missing", "voice notes: the Mac's personal build"},
	{"snapshot", ".phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"snapshot", ".upstairs.desks[].choice", "missing", "a menu's options, read from the Mac's terminals (M3)"},
	{"snapshot", ".upstairs.desks[].voiceNote", "missing", "voice notes: the Mac's personal build"},
	{"POST /setup/session", "refusal: no menu link's key: no session", "kind empty", "its 401 has no text; the Mac's says why"},
	{"POST /setup/phonemic", "route", "404", "the phone mic is the Mac's personal build"},
	{"GET /setup/qr.png", "refusal: no pairing code asked for", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
	{"GET /setup/logo/nope", "refusal: no such logo", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
	{"POST /setup/dictation", "route", "404", "dictation is Apple's speech recognition"},
	{"POST /setup/dictation/glossary", "route", "404", "dictation is Apple's speech recognition"},
	{"POST /setup/keystrokes", "route", "404", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"POST /setup/keystrokes/apps", "route", "404", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"GET /setup/guard", "route", "404", "the hire guard's settings are the Mac's (M4)"},
	{"POST /setup/guard", "route", "404", "the hire guard's settings are the Mac's (M4)"},
	{"GET /setup/claude-accounts", "route", "404", "Claude accounts are the Mac's (M2)"},
	{"POST /setup/claude-accounts", "route", "404", "Claude accounts are the Mac's (M2)"},
	{"GET /setup/bots", ".bots[].discovered", "missing", "left out when false or empty (omitempty)"},
	{"GET /setup/bots", ".bots[].mailbox", "missing", "left out when false or empty (omitempty)"},
	{"GET /setup/bots", ".bots[].pinned", "missing", "left out when false or empty (omitempty)"},
	{"GET /setup/bots", ".bots[].resolved", "missing", "only the Mac looks in the Grok Bot app"},
	{"POST /setup/bots", ".bots[].discovered", "missing", "left out when false or empty (omitempty)"},
	{"POST /setup/bots", ".bots[].mailbox", "missing", "left out when false or empty (omitempty)"},
	{"POST /setup/bots", ".bots[].pinned", "missing", "left out when false or empty (omitempty)"},
	{"POST /setup/bots", ".bots[].resolved", "missing", "only the Mac looks in the Grok Bot app"},
	{"POST /setup/bots/resolve", "route", "404", "only the Mac looks in the Grok Bot app"},
	{"GET /setup/hire/folders", "route", "404", "hiring from the setup page by role is the Mac's (M4)"},
	{"GET /setup/hire/gm/preflight", "refusal: no such Claude account", "status 200", "Claude accounts are the Mac's (M2): the GM's tools are checked for any account"},
	{"GET /setup/briefs", "route", "404", "briefs aren't served here (M4)"},
	{"POST /setup/briefs/preview", "route", "404", "briefs aren't served here (M4)"},
	{"POST /setup/hire", "route", "404", "hiring from the setup page by role is the Mac's (M4)"},
	{"POST /setup/notify", "route", "404", "a push to the iPhone app (personal build)"},
	{"GET /setup/nothing-here", "refusal: no such route", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
	{"GET /setup/gm/screens", "route", "404", "tiling the team is the Mac's (M4)"},
	{"POST /setup/gm/screen", "route", "404", "tiling the team is the Mac's (M4)"},
	{"POST /setup/gm/hire", "route", "404", "the GM's hiring is the Mac's (M4)"},
	{"POST /setup/gm/fire", "route", "404", "the GM's firing is the Mac's (M4)"},
	{"POST /setup/gm/brief", "route", "404", "a later brief is the Mac's (M4)"},
	{"GET /setup/gm/registry", "refusal: listed for the page, but nothing answers it", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
	{"POST /setup/gm/by-hand", "route", "404", "no GM token here (M4)"},
	{"POST /setup/gm/token/rotate", "route", "404", "no GM token here (M4)"},
	{"GET /setup/gm/nothing-here", "refusal: no such route", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
	{"POST /act", "refusal: unknown dictation language", "status 404", "no dictation here: action=language isn't known, so its session is looked for"},
	{"GET /glossary", "route", "404", "the iPhone app's speech names (personal build)"},
	{"GET /approvals", "route", "404", "over Tailscale only (personal build)"},
	{"GET /reviews", "route", "404", "over Tailscale only (personal build)"},
	{"GET /conversation", "route", "404", "the iPhone app's conversation (personal build)"},
	{"GET /session/info", "route", "404", "ⓘ Info (personal build)"},
	{"GET /notifications", "route", "404", "the iPhone app's push notifications (personal build)"},
	{"POST /notifications", "route", "404", "the iPhone app's push notifications (personal build)"},
	{"POST /dictate", "route", "404", "the phone mic (personal build)"},
	{"POST /peek", "route", "404", "the phone mic (personal build)"},
	{"POST /choose", "route", "404", "the iPhone app's typed answers (personal build)"},
	{"GET /choice", "route", "404", "the iPhone app's prompt details (personal build)"},
	{"GET /nothing-here", "refusal: no such route", "kind text", "its 404 says \"404 page not found\" in text; the Mac's is empty"},
}

// contractAsk is a fixture's request: who asks, its query, headers and body, and what's set up first.
type contractAsk struct {
	As      string            `json:"as"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
	Given   []string          `json:"given"`
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
	shapes map[string]any // by name: what a route's answer refers to
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
		Shapes map[string]struct{ Shape any } `json:"shapes"`
		Given  map[string]contractGiven       `json:"given"`
	}
	if err := json.Unmarshal(data, &shared); err != nil {
		t.Fatal(err)
	}
	c := contract{shapes: map[string]any{}, givens: shared.Given, files: map[string][]contractRoute{}}
	for name, s := range shared.Shapes {
		c.shapes[name] = s.Shape
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
	for _, name := range given {
		switch name {
		case "manager on":
			managerOn(cd.d)
		case "a paired phone":
			cd.phoneKey()
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
		headers["Cookie"] = setupCookie + "=" + cd.d.access.NewSetupSession()
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
		if object, ok := b.(map[string]any); ok && object["code"] == "$code" {
			cd.d.mu.Lock()
			code := cd.d.access.ActiveCode()
			cd.d.mu.Unlock()
			b = map[string]any{"code": code}
		}
		data, _ := json.Marshal(b)
		body = string(data)
	}
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host, r.RemoteAddr = host, from
	for k, v := range headers {
		r.Header.Set(k, v)
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
	return gaps
}

func gapKey(g knownGap) string { return g.subject + " | " + g.where + " | " + g.got }

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
			t.Errorf("not a known gap: {%q, %q, %q, \"\"},", g.subject, g.where, g.got)
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
type inventoryRoute struct{ fixture, goStatus string }

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
		case section == "Known gaps" && len(cells) == 4:
			inv.gaps = append(inv.gaps, knownGap{subject: cells[0], where: cells[1], got: cells[2], why: cells[3]})
		case strings.HasPrefix(section, "Fields of") && len(cells) == 3:
			name := "snapshot"
			if strings.Contains(section, "setup-state") {
				name = "setup-state"
			}
			inv.fields[name] = append(inv.fields[name], [3]string{cells[0], cells[1], cells[2]})
		case len(cells) >= 8 && slices.Contains([]string{"GET", "POST", "DELETE", "*"}, cells[0]):
			// Method, path, callers, request, answer, statuses, fixture, Go.
			inv.routes[cells[0]+" "+cells[1]] = inventoryRoute{fixture: cells[6], goStatus: cells[7]}
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
		case row.goStatus != want:
			t.Errorf("%s: INVENTORY.md says Go %q, the known gaps say %q", route, row.goStatus, want)
		case !strings.Contains(row.fixture, ".json"):
			t.Errorf("%s: INVENTORY.md names no fixture", route)
		}
	}
	// A route without a fixture says why, and one INVENTORY.md says this app doesn't serve answers 404 here
	// (with the manager on, as the GM's routes are 409 before anything else with it off).
	cd := newContractDeck(t, c, []string{"manager on"})
	for name, row := range inv.routes {
		if _, has := status[name]; has {
			continue
		}
		if !strings.HasPrefix(row.fixture, "none: ") || len(row.fixture) < len("none: ")+10 {
			t.Errorf("%s: no fixture, and INVENTORY.md says no why (a fixture cell \"none: <why>\")", name)
		}
		if row.goStatus != "404" {
			continue
		}
		method, path, _ := strings.Cut(name, " ")
		path = strings.NewReplacer("<name>", "contract", "<logo>", "keys", "<id>", "contract-bot", "*", "contract").Replace(path)
		caller := "page"
		if !isSetupRoute(path) {
			caller = "phone"
		}
		if got := cd.ask(method, path, contractAsk{As: caller}); got.status != http.StatusNotFound {
			t.Errorf("%s: INVENTORY.md says Go 404, but it answers %d", name, got.status)
		}
	}
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
