package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureRow struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Callers []string `json:"callers"`
}

// routeFixture is Tests/fixtures/frontdesk/routes.json, the table the Mac app writes.
func routeFixture(t *testing.T) []fixtureRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Routes   []fixtureRow
		Unlisted struct{ Callers []string }
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Routes) < 30 || strings.Join(table.Unlisted.Callers, ",") != "page,tool" {
		t.Fatalf("routes.json has %d rows and unlisted callers %v: not the table this test knows", len(table.Routes), table.Unlisted.Callers)
	}
	return table.Routes
}

// concreteRoute is a request a row stands for: a prefix's gets something after it, and a row for any method is tried as a POST.
func concreteRoute(row fixtureRow) (method, path string) {
	method, path = row.Method, row.Path
	if method == "*" {
		method = http.MethodPost
	}
	if prefix, ok := strings.CutSuffix(path, "*"); ok {
		path = prefix + "anything"
	}
	return method, path
}

func TestTheRouteTableIsTheFixture(t *testing.T) {
	rows := routeFixture(t)
	if len(rows) != len(routeTable) {
		t.Fatalf("routes.json has %d rows, routeTable %d: copy the fixture again", len(rows), len(routeTable))
	}
	for i, row := range rows {
		got := routeTable[i]
		if got.method != row.Method || got.path != row.Path || strings.Join(got.callers, ",") != strings.Join(row.Callers, ",") {
			t.Errorf("row %d: routeTable has %s %s %v, routes.json %s %s %v", i, got.method, got.path, got.callers, row.Method, row.Path, row.Callers)
		}
	}
}

// callers is every caller the table knows, with how a test makes a request as each.
var allCallers = []caller{callerPage, callerTool, callerGM, callerHire, callerNobody}

// asCaller is a call made as one caller: the page with its cookie, a tool with the setup key,
// the GM with its token, a hire with its send key (the deck and registry must have them: see withGMAndAHire).
func asCaller(who caller, gm, hireKey string) call {
	switch who {
	case callerPage:
		return call{}
	case callerTool:
		return call{asTool: true}
	case callerGM:
		return call{withoutSetupKey: true, headers: map[string]string{gmTokenHeader: gm}}
	case callerHire:
		return call{withoutSetupKey: true, headers: map[string]string{hireKeyHeader: hireKey}}
	}
	return call{withoutSetupKey: true}
}

// withGMAndAHire gives a test deck a GM token and a registry with one open hire holding a send key.
func withGMAndAHire(t *testing.T, d *Deck) (gm, hireKey string) {
	t.Helper()
	if err := os.WriteFile(registryPath(), frontDeskFixture(t, "registry.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := newSendKey(registryPath(), "a1b2c3")
	if err != nil || key == "" {
		t.Fatalf("newSendKey: %q, %v", key, err)
	}
	return d.newGMToken(), key
}

// Every row, for each of the five callers. The gate is driven for the refusals, which stop before any handler
// runs; what a caller is let in to is asked of the table, so no test here hires, unpairs or writes a real hook.
func TestEveryRowOfTheTableAgainstEachCaller(t *testing.T) {
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	for _, row := range routeFixture(t) {
		method, path := concreteRoute(row)
		has := func(name string) bool {
			for _, c := range row.Callers {
				if c == name {
					return true
				}
			}
			return false
		}
		for _, who := range allCallers {
			if got := routeAllows(who, method, path); got != has(who.name()) {
				t.Errorf("%s %s: routeAllows(%s) = %v, routes.json says %v", method, path, who.name(), got, has(who.name()))
			}
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from = method, path, "localhost:7710", thisComputer
			if method != http.MethodGet {
				c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
			}
			w := serve(t, d, c)
			switch {
			case who == callerNobody:
				if w.Code != http.StatusUnauthorized {
					t.Errorf("%s %s: nobody got %d, want 401", method, path, w.Code)
				}
			case !has(who.name()):
				if w.Code != http.StatusForbidden || w.Body.Len() == 0 {
					t.Errorf("%s %s: %s got %d %q, want 403 and a reason", method, path, who.name(), w.Code, w.Body)
				}
			}
		}
		if !has("page") {
			t.Errorf("%s %s: the page can always do what a tool can", method, path)
		}
		// A made-up GM token or hire key is no one: 401, as nobody.
		for _, header := range []string{gmTokenHeader, hireKeyHeader} {
			c := call{method: method, target: path, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
				headers: map[string]string{header: "not-a-real-one"}}
			if method != http.MethodGet {
				c.headers[setupHeader] = "1"
			}
			if got := serve(t, d, c).Code; got != http.StatusUnauthorized {
				t.Errorf("%s %s with a made-up %s: %d, want 401", method, path, header, got)
			}
		}
	}
}

func mergeHeaders(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// Who is asking, by what they send; the order is the Mac's (SetupSessions.caller).
func TestWhoIsAskingIsDecidedByWhatTheySendInTheMacsOrder(t *testing.T) {
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	d.mu.Lock()
	cookie := &http.Cookie{Name: setupCookie, Value: d.access.NewSetupSession()}
	setupKey := d.access.SetupKey
	d.mu.Unlock()
	who := func(headers map[string]string, withCookie bool) caller {
		r := httptest.NewRequest("GET", "/setup/gm/team", nil)
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		if withCookie {
			r.AddCookie(cookie)
		}
		return d.callerOf(r)
	}
	cases := []struct {
		name    string
		headers map[string]string
		cookie  bool
		want    caller
	}{
		{"nothing", nil, false, callerNobody},
		{"the page's cookie", nil, true, callerPage},
		{"the cookie beats every header", map[string]string{gmTokenHeader: gm, hireKeyHeader: hireKey, setupKeyHeader: setupKey}, true, callerPage},
		{"the setup key", map[string]string{setupKeyHeader: setupKey}, false, callerTool},
		{"the GM's token", map[string]string{gmTokenHeader: gm}, false, callerGM},
		{"the GM's token beside the setup key is the tool (the key wins)", map[string]string{gmTokenHeader: gm, setupKeyHeader: setupKey}, false, callerTool},
		{"the GM's token beside a wrong setup key", map[string]string{gmTokenHeader: gm, setupKeyHeader: "wrong"}, false, callerGM},
		{"a hire's key", map[string]string{hireKeyHeader: hireKey}, false, callerHire},
		{"a hire's key beside the setup key", map[string]string{hireKeyHeader: hireKey, setupKeyHeader: setupKey}, false, callerHire},
		{"a hire's key beside the GM's token is never the GM", map[string]string{hireKeyHeader: hireKey, gmTokenHeader: gm}, false, callerHire},
		{"a wrong hire's key carries no weight beside the GM's token (as on the Mac)", map[string]string{hireKeyHeader: "wrong", gmTokenHeader: gm}, false, callerGM},
		{"the setup key as the GM's token", map[string]string{gmTokenHeader: setupKey}, false, callerNobody},
		{"the GM's token as the setup key", map[string]string{setupKeyHeader: gm}, false, callerNobody},
		{"the GM's token as a hire's key", map[string]string{hireKeyHeader: gm}, false, callerNobody},
		{"a hire's key as the GM's token", map[string]string{gmTokenHeader: hireKey}, false, callerNobody},
		{"an empty token and key", map[string]string{gmTokenHeader: "", hireKeyHeader: ""}, false, callerNobody},
	}
	for _, c := range cases {
		if got := who(c.headers, c.cookie); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got.name(), c.want.name())
		}
	}
	// A token replaced or cleared stops working at once.
	fresh := d.newGMToken()
	if who(map[string]string{gmTokenHeader: gm}, false) != callerNobody || who(map[string]string{gmTokenHeader: fresh}, false) != callerGM {
		t.Error("a new GM token should be the only valid one")
	}
	d.clearGMToken()
	if who(map[string]string{gmTokenHeader: fresh}, false) != callerNobody {
		t.Error("a cleared GM token still works")
	}
	if who(map[string]string{gmTokenHeader: ""}, false) != callerNobody {
		t.Error("with no token, an empty header made the GM")
	}
	if a, b := d.newGMToken(), d.newGMToken(); a == b || len(a) < 32 {
		t.Errorf("GM tokens are not fresh and long: %q %q", a, b)
	}
}

// What the GM reaches that this app serves, with its token: the same answers the page gets.
func TestTheGMReachesWhatThisAppServes(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	for _, target := range []string{"/setup/gm/team", "/setup/gm/events?wait=0", "/setup/gm/approvals", "/setup/gm/version"} {
		w := serve(t, d, call{method: "GET", target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{gmTokenHeader: gm}})
		if w.Code != http.StatusOK {
			t.Errorf("GET %s as the GM: %d %s, want 200", target, w.Code, w.Body)
		}
	}
	// The registry edit is the GM's too, and a hire's key is refused it.
	edit := `{"id":"a1b2c3","notes":"by the GM"}`
	w := serve(t, d, call{method: "POST", target: "/setup/gm/registry", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, body: edit,
		headers: map[string]string{gmTokenHeader: gm, setupHeader: "1"}})
	if w.Code != http.StatusOK {
		t.Errorf("the GM's registry edit: %d %s", w.Code, w.Body)
	}
	// A hire with its key is let in to /setup/gm/send (served now: an empty body is a 400, not a 403), to nothing else.
	_, hireKey := withGMAndAHire(t, d)
	send := call{method: "POST", target: "/setup/gm/send", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{hireKeyHeader: hireKey, setupHeader: "1"}}
	if got := serve(t, d, send).Code; got != http.StatusBadRequest {
		t.Errorf("a hire's send: %d, want 400 (past the gate, nothing to send)", got)
	}
	for _, target := range []string{"/setup/gm/team", "/setup/gm/events", "/setup/gm/contacts", "/setup/gm/registry"} {
		send.target, send.method = target, "POST"
		if got := serve(t, d, send).Code; got != http.StatusForbidden {
			t.Errorf("a hire's POST %s: %d, want 403", target, got)
		}
	}
}

// What the setup-key file alone was able to do before: each is now the page's.
func TestTheSetupKeyFileAloneCantDoWhatAHireCouldMisuse(t *testing.T) {
	d := newTestDeck(t)
	post := func(target, body string, tool bool) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: thisComputer, asTool: tool, body: body,
			headers: map[string]string{setupHeader: "1"}})
	}
	for _, path := range []string{"/setup/hire/gm", "/setup/hiring", "/setup/hiring/choose", "/setup/roles", "/setup/hooks", "/setup/unpair",
		"/setup/webhook", "/setup/webhook/test", "/setup/bots", "/setup/bots/override", "/setup/maxconfirm", "/setup/layout", "/setup/profiles",
		"/setup/pin", "/setup/unpin", "/setup/floors", "/setup/login", "/setup/summaries", "/setup/summary/provider", "/setup/summary/key",
		"/setup/viewer", "/setup/viewer/switch", "/setup/viewer/wipe", "/setup/viewer/secret", "/setup/viewer/rotate", "/setup/pair"} {
		if w := post(path, `{}`, true); w.Code != http.StatusForbidden {
			t.Errorf("POST %s with the setup-key file: %d, want 403", path, w.Code)
		}
	}
	// And nothing changed: the webhook, Summaries and the max-effort question are as they were.
	post("/setup/webhook", `{"enabled":true,"url":"http://127.0.0.1:1/x","secret":"s"}`, true)
	post("/setup/summary/key", `{"key":"sk-ant-test0"}`, true)
	post("/setup/maxconfirm?enabled=0", "", true)
	d.mu.Lock()
	changed := d.config.Webhook.URL != "" || d.config.Webhook.Secret != "" || !d.config.MaxConfirm
	d.mu.Unlock()
	if changed {
		t.Error("a tool with the setup-key file changed the webhook or max-effort setting")
	}
	// The page still can.
	if w := post("/setup/maxconfirm?enabled=0", "", false); w.Code != http.StatusOK || d.config.MaxConfirm {
		t.Errorf("the page turning the max-effort question off: %d, on = %v", w.Code, d.config.MaxConfirm)
	}
	// A tool still reads, and what isn't in the table keeps 1.23's rule.
	for _, target := range []string{"/setup/state", "/setup/selftest"} {
		if w := serve(t, d, call{method: "GET", target: target, host: "localhost:7710", from: thisComputer, asTool: true}); w.Code == http.StatusForbidden {
			t.Errorf("GET %s with the key: refused", target)
		}
	}
	if w := post("/setup/theme", `{}`, true); w.Code == http.StatusForbidden {
		t.Errorf("POST /setup/theme, not in the table, with the key: refused")
	}
}

// smolcorp setup repairs with auto=1, installs this copy and opens the firewall (the system asks you);
// a repair without auto=1, the Wi-Fi, login and Tailscale are the page's.
func TestAToolMayOnlyRepairWithAutoInstallAndOpenTheFirewall(t *testing.T) {
	if !fixAllowed(callerTool, fixRepair, "1") {
		t.Error("a tool's repair with auto=1 is refused")
	}
	for _, auto := range []string{"", "0", "true"} {
		if fixAllowed(callerTool, fixRepair, auto) {
			t.Errorf("a tool's repair with auto=%q is let in", auto)
		}
		if !fixAllowed(callerPage, fixRepair, auto) {
			t.Errorf("the page's repair with auto=%q is refused", auto)
		}
	}
	for _, fix := range []string{fixInstall, fixFirewall} {
		if !fixAllowed(callerTool, fix, "") || !fixAllowed(callerPage, fix, "") {
			t.Errorf("fix %q: refused to a tool or the page", fix)
		}
	}
	for _, fix := range []string{fixHomeWiFi, fixLogin, fixTailscale, "", "nope"} {
		if fixAllowed(callerTool, fix, "1") || !fixAllowed(callerPage, fix, "") {
			t.Errorf("fix %q: a tool with auto=1 or the page got the wrong answer", fix)
		}
	}
	d := newTestDeck(t)
	for _, target := range []string{"/setup/fix?fix=repair", "/setup/fix?fix=repair&auto=0", "/setup/fix?fix=login&auto=1", "/setup/fix?fix=homewifi&auto=1", "/setup/fix?fix=tailscale&auto=1", "/setup/fix"} {
		w := serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: thisComputer, asTool: true, headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusForbidden {
			t.Errorf("%s from a tool: %d, want 403", target, w.Code)
		}
	}
}

func TestTheWebhookSecretIsThePagesAlone(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.Webhook.Secret = "s3cret-test0"
	d.mu.Unlock()
	for _, tool := range []bool{false, true} {
		w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, asTool: tool})
		if got := strings.Contains(w.Body.String(), "s3cret-test0"); got == tool {
			t.Errorf("setup's state for a tool = %v: the secret is seen = %v", tool, got)
		}
	}
	if webhookSecretFor(callerPage, "x") != "x" || webhookSecretFor(callerTool, "x") != "" || webhookSecretFor(callerNobody, "x") != "" {
		t.Error("webhookSecretFor")
	}
}

// pagePost is a POST from the setup page's own session, to a test server.
func pagePost(t *testing.T, d *Deck, base, path string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	request.AddCookie(&http.Cookie{Name: setupCookie, Value: d.access.NewSetupSession()})
	d.mu.Unlock()
	request.Header.Set(setupHeader, "1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
