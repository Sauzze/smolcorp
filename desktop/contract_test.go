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
// its keys and their types. Each fixture is asked of this app in process, and its answers checked against
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
	{"snapshot", ".keys[].session.choice", "missing", "a menu's options, read from the Mac's terminals (M3)"},
	{"snapshot", ".keys[].session.conversation", "missing", "the conversation view: the iPhone app's (personal build)"},
	{"snapshot", ".keys[].session.dictates", "missing", "dictation is Apple's speech recognition"},
	{"snapshot", ".keys[].session.fire", "missing", "what the desk's terminal lets the office do: said once here, as hiring.fire (M3)"},
	{"snapshot", ".keys[].session.keys", "missing", "what the desk's terminal lets the office do (M3)"},
	{"snapshot", ".keys[].session.phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"snapshot", ".keys[].session.voiceNote", "missing", "voice notes: the Mac's personal build"},
	{"snapshot", ".phoneMic", "missing", "the phone mic is the Mac's personal build"},
	{"POST /setup/phonemic", "route", "404", "the phone mic is the Mac's personal build"},
	{"POST /setup/dictation", "route", "404", "dictation is Apple's speech recognition"},
	{"POST /setup/dictation/glossary", "route", "404", "dictation is Apple's speech recognition"},
	{"POST /setup/keystrokes", "route", "404", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"POST /setup/keystrokes/apps", "route", "404", "pressing keys in other apps needs Accessibility: the Mac's"},
	{"GET /setup/guard", "route", "404", "the hire guard's settings are the Mac's (M4)"},
	{"POST /setup/guard", "route", "404", "the hire guard's settings are the Mac's (M4)"},
	{"GET /setup/claude-accounts", "route", "404", "Claude accounts are the Mac's (M2)"},
	{"POST /setup/claude-accounts", "route", "404", "Claude accounts are the Mac's (M2)"},
	{"POST /setup/bots/resolve", "route", "404", "only the Mac looks in the Grok Bot app"},
	{"GET /setup/hire/folders", "route", "404", "hiring from the setup page by role is the Mac's (M4)"},
	{"GET /setup/briefs", "route", "404", "briefs aren't served here (M4)"},
	{"POST /setup/hire", "route", "404", "hiring from the setup page by role is the Mac's (M4)"},
	{"POST /setup/notify", "route", "404", "a push to the iPhone app (personal build)"},
	{"GET /setup/gm/screens", "route", "404", "tiling the team is the Mac's (M4)"},
	{"POST /setup/gm/screen", "route", "404", "tiling the team is the Mac's (M4)"},
	{"POST /setup/gm/hire", "route", "404", "the GM's hiring is the Mac's (M4)"},
	{"POST /setup/gm/fire", "route", "404", "the GM's firing is the Mac's (M4)"},
	{"POST /setup/gm/brief", "route", "404", "a later brief is the Mac's (M4)"},
	{"POST /setup/gm/by-hand", "route", "404", "no GM token here (M4)"},
	{"POST /setup/gm/token/rotate", "route", "404", "no GM token here (M4)"},
	{"POST /act", "refusal: unknown dictation language", "status 404", "no dictation here: action=language isn't known, so its session is looked for"},
	{"GET /glossary", "route", "404", "the iPhone app's speech names (personal build)"},
	{"GET /approvals", "route", "404", "over Tailscale only (personal build)"},
	{"GET /reviews", "route", "404", "over Tailscale only (personal build)"},
	{"GET /conversation", "route", "404", "the iPhone app's conversation (personal build)"},
	{"GET /session/info", "route", "404", "ⓘ Info (personal build)"},
	{"POST /dictate", "route", "404", "the phone mic (personal build)"},
	{"POST /peek", "route", "404", "the phone mic (personal build)"},
	{"POST /choose", "route", "404", "the iPhone app's typed answers (personal build)"},
	{"GET /choice", "route", "404", "the iPhone app's prompt details (personal build)"},
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
		Why     string      `json:"why"`
	} `json:"refusals"`
}

type contract struct {
	shapes map[string]any // by name: what a route's answer refers to
	hooks  []struct {
		Headers map[string]string `json:"headers"`
		Body    any               `json:"body"`
	}
	files map[string][]contractRoute
}

func contractFolder() string { return filepath.Join("..", "Tests", "fixtures", "contract") }

func readContract(t *testing.T) contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractFolder(), "contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var shared struct {
		Shapes map[string]struct{ Shape any } `json:"shapes"`
		Given  struct {
			Sessions struct {
				Hooks []struct {
					Headers map[string]string `json:"headers"`
					Body    any               `json:"body"`
				} `json:"hooks"`
			} `json:"sessions"`
		} `json:"given"`
	}
	if err := json.Unmarshal(data, &shared); err != nil {
		t.Fatal(err)
	}
	c := contract{shapes: map[string]any{}, hooks: shared.Given.Sessions.Hooks, files: map[string][]contractRoute{}}
	for name, s := range shared.Shapes {
		c.shapes[name] = s.Shape
	}
	for _, file := range []string{"state.json", "setup.json", "gm.json", "office.json"} {
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
	if len(c.shapes) < 3 || len(c.hooks) == 0 || len(c.files["setup.json"]) < 30 {
		t.Fatalf("the contract has %d shapes, %d hooks, %d setup routes: not the files this test knows", len(c.shapes), len(c.hooks), len(c.files["setup.json"]))
	}
	return c
}

// MARK: Shapes, as ContractTests.swift makes and compares them

// contractShape is a JSON value's shape: "string", "number", "bool" or "null" for a leaf; an object's keys,
// each with its shape; an array's elements merged into one shape, or [] for none.
func contractShape(value any) any {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "bool"
	case float64, json.Number:
		return "number"
	case []any:
		if len(v) == 0 {
			return []any{}
		}
		merged := contractShape(v[0])
		for _, e := range v[1:] {
			merged = mergeShapes(merged, contractShape(e))
		}
		return []any{merged}
	case map[string]any:
		shape := map[string]any{}
		for key, e := range v {
			shape[key] = contractShape(e)
		}
		return shape
	}
	return "any"
}

func mergeShapes(a, b any) any {
	if a == "null" {
		return b
	}
	if b == "null" {
		return a
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			merged := map[string]any{}
			for k, v := range am {
				merged[k] = v
			}
			for k, v := range bm {
				if old, ok := merged[k]; ok {
					merged[k] = mergeShapes(old, v)
				} else {
					merged[k] = v
				}
			}
			return merged
		}
	}
	if aa, ok := a.([]any); ok {
		if ba, ok := b.([]any); ok {
			if len(aa) == 0 {
				return b
			}
			if len(ba) == 0 {
				return a
			}
			return []any{mergeShapes(aa[0], ba[0])}
		}
	}
	if as, ok := a.(string); ok && a == b {
		return as
	}
	return "any"
}

// shapeDifferences is where got doesn't have want's shape, as "where: what" lines, the Mac's way. A null on
// either side, an empty array or object, or "any" stands for anything: values, not shapes.
func shapeDifferences(want, got any, path string) []string {
	here := path
	if here == "" {
		here = "."
	}
	if want == "null" || want == "any" || got == "null" || got == "any" {
		return nil
	}
	wm, wok := want.(map[string]any)
	gm, gok := got.(map[string]any)
	if wok && gok {
		if len(wm) == 0 || len(gm) == 0 {
			return nil
		}
		keys := map[string]bool{}
		for k := range wm {
			keys[k] = true
		}
		for k := range gm {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		var found []string
		for _, k := range sorted {
			w, inWant := wm[k]
			g, inGot := gm[k]
			switch {
			case inWant && inGot:
				found = append(found, shapeDifferences(w, g, path+"."+k)...)
			case inWant:
				found = append(found, path+"."+k+": missing")
			default:
				found = append(found, path+"."+k+": extra")
			}
		}
		return found
	}
	wa, wok := want.([]any)
	ga, gok := got.([]any)
	if wok && gok {
		if len(wa) == 0 || len(ga) == 0 {
			return nil
		}
		return shapeDifferences(wa[0], ga[0], path+"[]")
	}
	if w, g := shapeName(want), shapeName(got); w != g {
		return []string{here + ": " + g + ", want " + w}
	}
	return nil
}

func shapeName(shape any) string {
	switch s := shape.(type) {
	case string:
		return s
	case []any:
		return "array"
	}
	return "object"
}

func TestShapesAreKeysAndTypesAsOnTheMac(t *testing.T) {
	var value any
	_ = json.Unmarshal([]byte(`{"a": 1, "b": [true, null], "c": [{"d": "x"}, {"e": null}], "f": [], "g": null}`), &value)
	got, _ := json.Marshal(contractShape(value))
	if want := `{"a":"number","b":["bool"],"c":[{"d":"string","e":"null"}],"f":[],"g":"null"}`; string(got) != want {
		t.Errorf("shape %s, want %s", got, want)
	}
	differences := shapeDifferences(map[string]any{"a": "number", "b": "string"}, map[string]any{"a": "string", "c": "bool"}, "")
	if want := []string{".a: string, want number", ".b: missing", ".c: extra"}; !slices.Equal(differences, want) {
		t.Errorf("differences %q, want %q", differences, want)
	}
	if d := shapeDifferences(map[string]any{"a": []any{"string"}, "g": "number"}, map[string]any{"a": []any{}, "g": "null"}, ""); d != nil {
		t.Errorf("an empty array or a null stands for anything: %q", d)
	}
	if d := shapeDifferences([]any{map[string]any{"x": "bool"}}, []any{map[string]any{"x": "number"}}, ""); !slices.Equal(d, []string{"[].x: number, want bool"}) {
		t.Errorf("in an array: %q", d)
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

func newContractDeck(t *testing.T, c contract, given []string) *contractDeck {
	t.Helper()
	cd := &contractDeck{t: t, d: newTestDeck(t), c: c}
	for _, step := range given {
		switch step {
		case "sessions":
			for _, hook := range c.hooks {
				if got := cd.ask("POST", "/hook", contractAsk{As: "hook", Headers: hook.Headers, Body: hook.Body}); got.status != http.StatusNoContent {
					t.Fatalf("a hook for the sessions: %d", got.status)
				}
			}
		case "a pairing code":
			cd.ask("POST", "/setup/pair", contractAsk{As: "page"})
			cd.code = true
		case "a paired phone":
			cd.phoneKey()
		case "manager on":
			cd.d.mu.Lock()
			cd.d.config.ManagerEnabled = true
			cd.d.mu.Unlock()
		default:
			t.Fatalf("nothing gives %q", step)
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
	for _, file := range []string{"state.json", "setup.json", "gm.json", "office.json"} {
		for _, route := range c.files[file] {
			method, path, _ := strings.Cut(route.Route, " ")
			ask := func(a contractAsk) contractAnswer {
				return newContractDeck(t, c, a.Given).ask(method, path, a)
			}
			var found []knownGap
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
					for _, line := range shapeDifferences(want, contractShape(got.json), "") {
						where, what, _ := strings.Cut(line, ": ")
						found = append(found, knownGap{subject: subject, where: where, got: what})
					}
				}
			}
			for _, refusal := range route.Refusals {
				got := ask(refusal.Request)
				macServes = macServes || refusal.Status != http.StatusNotFound
				goServes = goServes || got.status != http.StatusNotFound
				if got.status != refusal.Status {
					found = append(found, knownGap{subject: route.Route, where: "refusal: " + refusal.Why, got: "status " + strconv.Itoa(got.status)})
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

// inventoryTables reads INVENTORY.md's routes table (method, path, Go status, fixture) and its known gaps.
func inventoryTables(t *testing.T) (routes map[string][2]string, gaps []knownGap) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractFolder(), "INVENTORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	routes = map[string][2]string{}
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
		switch {
		case section == "Known gaps" && len(cells) == 4 && cells[0] != "In" && !strings.HasPrefix(cells[0], "---"):
			gaps = append(gaps, knownGap{subject: cells[0], where: cells[1], got: cells[2], why: cells[3]})
		case len(cells) >= 8 && slices.Contains([]string{"GET", "POST", "DELETE", "*"}, cells[0]):
			// Method, path, callers, request, answer, statuses, fixture, Go.
			routes[cells[0]+" "+cells[1]] = [2]string{cells[7], cells[6]}
		}
	}
	return routes, gaps
}

// knownGaps is INVENTORY.md's "Known gaps" table, row for row; and each route with a fixture says in its Go
// column what the gaps say: 404 (not served), differs (a gap in it or in the shape it answers), or served.
func TestTheKnownGapsAreInventorysGoStatus(t *testing.T) {
	routes, gaps := inventoryTables(t)
	if len(routes) < 100 {
		t.Fatalf("INVENTORY.md has %d routes: not the table this test knows", len(routes))
	}
	if len(gaps) != len(knownGaps) {
		t.Errorf("INVENTORY.md has %d known gaps, knownGaps %d", len(gaps), len(knownGaps))
	}
	for i := 0; i < len(gaps) && i < len(knownGaps); i++ {
		if gaps[i] != knownGaps[i] {
			t.Errorf("gap %d: INVENTORY.md has %+v, knownGaps %+v", i+1, gaps[i], knownGaps[i])
		}
	}
	c := readContract(t)
	status := map[string]string{}
	for _, file := range []string{"state.json", "setup.json", "gm.json", "office.json"} {
		for _, route := range c.files[file] {
			if strings.Contains(route.Route, "nothing-here") || strings.Contains(route.Route, "/bots/") || strings.Contains(route.Route, "/setup/logo/") {
				continue
			}
			named := ""
			if route.Answer != nil {
				named, _ = route.Answer.Shape.(string)
			}
			if _, ok := status[route.Route]; !ok {
				status[route.Route] = "served"
			}
			for _, g := range knownGaps {
				switch {
				case g.subject == route.Route && g.where == "route":
					status[route.Route] = "404"
				case (g.subject == route.Route || g.subject == named) && status[route.Route] != "404":
					status[route.Route] = "differs"
				}
			}
		}
	}
	for route, want := range status {
		row, ok := routes[route]
		switch {
		case !ok:
			t.Errorf("%s has a fixture but no row in INVENTORY.md", route)
		case row[0] != want:
			t.Errorf("%s: INVENTORY.md says Go %q, the known gaps say %q", route, row[0], want)
		case row[1] == "—" || row[1] == "":
			t.Errorf("%s: INVENTORY.md names no fixture", route)
		}
	}
}
