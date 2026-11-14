package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// SK2 (SANDBOX-PLAN §3.1, §3.8): the setup key and the window's key in memory only, the check-only key, a terminal's
// ask for a key, and the setup link used first. Stand-ins only: fake keys, the test's own support folder, no window,
// no browser and no Keychain.

// No file Smolcorp keeps holds a key that opens anything: after a start, the setup key, the window's keys and the
// browser's key are in no file of the support folder, and the files earlier versions kept them in are gone. The
// check-only key's file is the one key file written, yours alone.
func TestNoKeyFileAfterStart(t *testing.T) {
	isolate(t)
	withMac(t)
	for _, old := range []string{setupKeyFile, windowKeyFile, gmTokenPath()} {
		writeFile(t, old, "fake-key-from-before-0123456789\n")
	}
	config := loadConfig()
	config.GMByHand = true
	config.save()
	d := startDeck(t)
	d.mu.Lock()
	windowDevice := ensureWindowKey(d.access.Phones, &d.access.windowDeviceKey)
	keys := map[string]string{"setup": d.access.SetupKey, "window": d.access.WindowKey, "browser": d.access.BrowserKey,
		"window device": windowDevice, "GM": d.gmToken}
	selftest := d.access.SelftestKey
	d.mu.Unlock()
	d.gmKept.flush(time.Minute)
	for _, old := range []string{setupKeyFile, windowKeyFile, gmTokenPath()} {
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("%s is still there after a start: %v", filepath.Base(old), err)
		}
	}
	_ = filepath.WalkDir(supportDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, _ := os.ReadFile(path)
		for name, key := range keys {
			if key != "" && strings.Contains(string(data), key) {
				t.Errorf("%s holds the %s key", path, name)
			}
		}
		return nil
	})
	saved, err := os.ReadFile(selftestKeyFile)
	if err != nil || strings.TrimSpace(string(saved)) != selftest || selftest == d.access.SetupKey {
		t.Fatalf("the selftest-key file: %v", err)
	}
	if err := yoursAlone(selftestKeyFile); err != nil {
		t.Errorf("the selftest-key file isn't yours alone: %v", err)
	}
}

// probedRoutes is every route a key could open: setup's tables, the phone's table, and the switch's own, each as one
// request.
func probedRoutes() [][2]string {
	var routes [][2]string
	add := func(method, path string) {
		path = strings.Replace(path, "/*", "/probe", 1)
		if method == "*" {
			routes = append(routes, [2]string{"GET", path}, [2]string{"POST", path})
			return
		}
		routes = append(routes, [2]string{method, path})
	}
	for _, row := range append(append([]routeRow{}, routeTable...), goOnlyRoutes...) {
		add(row.method, row.path)
	}
	for key := range phoneRouteTable {
		method, path, _ := strings.Cut(key, " ")
		add(method, path)
	}
	for _, path := range []string{"/setup/state", "/setup/selftest", "/setup/fix?fix=repair&auto=1", "/setup/hooks", "/setup/demo",
		"/bots/probe/event", "/hook", "/launcher", hireStartPath, screenAnswerPath, "/events", "/paired", "/focus", "/act?action=settings",
		"/summary", "/hire/window", "/t3code", "/t3code/open"} {
		add("GET", path)
		add("POST", path)
	}
	return routes
}

// The check-only key opens GET /setup/selftest, and nothing else: in any header, in the address or as a cookie, every
// other route answers it as it answers no key at all.
func TestTheCheckOnlyKeyOpensNothingElse(t *testing.T) {
	d := newTestDeck(t)
	key := d.access.SelftestKey
	ask := func(method, target string, headers map[string]string) int {
		all := map[string]string{setupHeader: "1"}
		for name, value := range headers {
			all[name] = value
		}
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: all}).Code
	}
	if got := ask("GET", "/setup/selftest", map[string]string{selftestKeyHeader: key}); got != http.StatusOK {
		t.Fatalf("the checklist with the check-only key: %d", got)
	}
	if got := ask("GET", "/setup/selftest", map[string]string{selftestKeyHeader: "fake-made-up-key"}); got != http.StatusUnauthorized {
		t.Errorf("the checklist with a made-up key: %d", got)
	}
	places := map[string]func(target string) (string, map[string]string){
		"its header": func(target string) (string, map[string]string) {
			return target, map[string]string{selftestKeyHeader: key}
		},
		"the setup key's header": func(target string) (string, map[string]string) {
			return target, map[string]string{setupKeyHeader: key}
		},
		"the hook's header": func(target string) (string, map[string]string) { return target, map[string]string{hookKeyHeader: key} },
		"the GM's header":   func(target string) (string, map[string]string) { return target, map[string]string{gmTokenHeader: key} },
		"a hire's header":   func(target string) (string, map[string]string) { return target, map[string]string{hireKeyHeader: key} },
		"the cookie": func(target string) (string, map[string]string) {
			return target, map[string]string{"Cookie": setupCookie() + "=" + key}
		},
		"the address": func(target string) (string, map[string]string) {
			sep := "?"
			if strings.Contains(target, "?") {
				sep = "&"
			}
			return target + sep + "t=" + url.QueryEscape(key), nil
		},
	}
	probed := 0
	for _, route := range probedRoutes() {
		method, target := route[0], route[1]
		bare := ask(method, target, nil)
		for place, put := range places {
			if method == "GET" && target == "/setup/selftest" && place == "its header" {
				continue
			}
			withKey, headers := put(target)
			if got := ask(method, withKey, headers); got != bare {
				t.Errorf("%s %s with the check-only key in %s: %d, with no key %d", method, target, place, got, bare)
			}
			probed++
		}
	}
	if probed < 700 {
		t.Errorf("only %d probes: not the route tables this test knows", probed)
	}
}

// Off a Mac a setup link lasts a minute (the browser's command line is anyone's of yours to read); under the Mac app,
// whose Settings opens with no command line, five.
func TestTheSetupLinkLastsAMinuteOffAMac(t *testing.T) {
	for goos, want := range map[string]time.Duration{"linux": time.Minute, "windows": time.Minute, "darwin": 5 * time.Minute} {
		if got := setupLinkLifetime(goos); got != want {
			t.Errorf("%s: %v, want %v", goos, got, want)
		}
	}
	if runtime.GOOS != "darwin" && setupTokenLifetime != time.Minute {
		t.Errorf("this computer's links last %v", setupTokenLifetime)
	}
}

// A link another program used first: when the owner's browser brings the same token, that program's session ends at
// once, the page says "Another program on this computer opened Setup with this link first", and lists what it
// changed. The ended session's cookie opens nothing, and its page is told why.
func TestALinkUsedFirstEndsThatSessionAndThePageSaysSo(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	handOver := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	}
	// Another program gets there first, and changes something.
	first := handOver()
	if first.Code != http.StatusNoContent || len(first.Result().Cookies()) != 1 {
		t.Fatalf("the first use: %d", first.Code)
	}
	cookie := setupCookie() + "=" + first.Result().Cookies()[0].Value
	withCookie := func(method, target string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", "Cookie": cookie}})
	}
	if changed := withCookie("POST", "/setup/scroll?enabled=1"); changed.Code != http.StatusOK {
		t.Fatalf("its change: %d", changed.Code)
	}
	if read := withCookie("GET", "/setup/state"); read.Code != http.StatusOK {
		t.Fatalf("its read: %d", read.Code)
	}
	// The owner's browser arrives with the same link.
	owner := handOver()
	var said struct {
		UsedFirst bool     `json:"usedFirst"`
		Message   string   `json:"message"`
		Changed   []string `json:"changed"`
	}
	if owner.Code != http.StatusConflict || json.Unmarshal(owner.Body.Bytes(), &said) != nil || !said.UsedFirst || len(owner.Result().Cookies()) != 0 {
		t.Fatalf("the owner's browser: %d %s", owner.Code, owner.Body)
	}
	if !strings.Contains(said.Message, "Another program on this computer opened Setup with this link first") ||
		len(said.Changed) != 1 || said.Changed[0] != "changed scroll in Setup" {
		t.Errorf("what the page is told: %+v", said)
	}
	// That session has ended: its cookie opens nothing, and its page is told why.
	ended := withCookie("GET", "/setup/state")
	if ended.Code != http.StatusUnauthorized || ended.Header().Get(sessionEndedHeader) != setupSessionEnded {
		t.Errorf("the ended session: %d %q", ended.Code, ended.Header().Get(sessionEndedHeader))
	}
	if again := withCookie("POST", "/setup/scroll?enabled=0"); again.Code != http.StatusUnauthorized {
		t.Errorf("the ended session changed something: %d", again.Code)
	}
	// A third use opens nothing either, and is told the same.
	if third := handOver(); third.Code != http.StatusConflict || !strings.Contains(third.Body.String(), "changed scroll in Setup") || len(third.Result().Cookies()) != 0 {
		t.Errorf("a third use: %d %s", third.Code, third.Body)
	}
	// The page shows it, and its locked note says why its own session ended.
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"if (res && res.status === 409) usedFirst(await res.json().catch(() => ({})));",
		"function usedFirst(answer) {", "'Meanwhile it changed:'", "res.headers.get('X-Smolcorp-Session-Ended')", "ended === 'used-first'"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("setup.html lacks %q", want)
		}
	}
}

// The program that used a link first can't hide that from the owner (RT126SK2 B1): using the link again itself, before
// the owner's browser arrives, ends its own session and leaves the same 409 for the owner, with what it did, its reads
// of a key among it. The warning stays in every Setup page's state and in the menu, and no page a link opened clears
// it: off a Mac the tray does; under the Mac app only the Settings window's own session.
func TestTheFirstUserOfALinkCantHideItFromTheOwner(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	handOver := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	}
	first := handOver()
	if first.Code != http.StatusNoContent {
		t.Fatalf("the first use: %d", first.Code)
	}
	cookie := setupCookie() + "=" + first.Result().Cookies()[0].Value
	withCookie := func(method, target string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", "Cookie": cookie}})
	}
	withCookie("GET", "/setup/viewer")
	withCookie("POST", "/setup/scroll?enabled=1")
	withCookie("POST", "/setup/scroll?enabled=0")
	// The first user uses the link again itself, to have the record gone before the owner comes.
	if again := handOver(); again.Code != http.StatusConflict {
		t.Fatalf("its own second use: %d", again.Code)
	}
	// The owner arrives after that: still the report, with what it did in words.
	owner := handOver()
	var said struct {
		Changed []string `json:"changed"`
	}
	_ = json.Unmarshal(owner.Body.Bytes(), &said)
	want := []string{"read the portable office's link, which holds its key", "changed scroll in Setup"}
	if owner.Code != http.StatusConflict || !slices.Equal(said.Changed, want) {
		t.Fatalf("the owner's browser: %d %s, want the changes %q", owner.Code, owner.Body, want)
	}
	// And whether or not the owner's browser ever comes: every Setup page and the menu show it.
	d.mu.Lock()
	owners := d.access.NewSetupSessionFrom("fake-owner-token")
	d.mu.Unlock()
	ownerPage := func(method, target string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", "Cookie": setupCookie() + "=" + owners}})
	}
	var state struct {
		UsedFirst *struct {
			Times   int      `json:"times"`
			Changed []string `json:"changed"`
			ClearIn string   `json:"clearIn"`
		} `json:"usedFirst"`
	}
	_ = json.Unmarshal(ownerPage("GET", "/setup/state").Body.Bytes(), &state)
	if state.UsedFirst == nil || state.UsedFirst.Times != 1 || !slices.Equal(state.UsedFirst.Changed, want) || state.UsedFirst.ClearIn != "tray" {
		t.Fatalf("Setup's state: %+v", state.UsedFirst)
	}
	if !d.menuState().UsedFirst {
		t.Error("the menu doesn't show it")
	}
	// A tool's read of the state never has it.
	if tool := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, asTool: true}); strings.Contains(tool.Body.String(), "usedFirst") {
		t.Error("a tool is shown the warning")
	}
	// No page a link opened clears it off a Mac: the tray does.
	if w := ownerPage("POST", "/setup/used-first/seen"); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "I've seen it") {
		t.Errorf("a browser's Setup cleared it: %d %s", w.Code, w.Body)
	}
	if !d.menuState().UsedFirst {
		t.Fatal("cleared by a page")
	}
	d.clearUsedFirst() // the tray's "I've seen it"
	if d.menuState().UsedFirst || strings.Contains(ownerPage("GET", "/setup/state").Body.String(), "usedFirst") {
		t.Error("still shown after the tray's I've seen it")
	}

	// Under the Mac app, the Settings window's own session clears it, never a browser's.
	d = newMacTestDeck(t)
	d.mu.Lock()
	d.access.noteUsedFirst([]string{"changed scroll in Setup"})
	browser, window := d.access.NewSetupSessionFrom("fake-link-token"), d.access.NewSetupSession()
	d.mu.Unlock()
	clear := func(session string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/used-first/seen", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", "Cookie": setupCookie() + "=" + session}})
	}
	if w := clear(browser); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Settings window") || !d.menuState().UsedFirst {
		t.Errorf("a browser's Setup on the Mac cleared it: %d %s", w.Code, w.Body)
	}
	if w := clear(window); w.Code != http.StatusOK || d.menuState().UsedFirst {
		t.Errorf("the window's I've seen it: %d %s", w.Code, w.Body)
	}
	// The page shows it, and clears it only through that route.
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="used-first-warning" role="alert"`, "renderUsedFirst();", "fetch('/setup/used-first/seen'", "report.clearIn === 'window'"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("setup.html lacks %q", want)
		}
	}
}

// newMacTestDeck is newTestDeck under a stand-in Mac app.
func newMacTestDeck(t *testing.T) *Deck {
	t.Helper()
	isolate(t)
	withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	return d
}

// askFromTerminal is POST /terminal/ask, as a terminal sends it.
func askFromTerminal(t *testing.T, d *Deck, kind, folder string) (int, map[string]any) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: terminalAskPath + "?kind=" + kind + "&folder=" + url.QueryEscape(folder),
		host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	var said map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &said)
	return w.Code, said
}

// waitFromTerminal is GET /terminal/ask with the ticket.
func waitFromTerminal(t *testing.T, d *Deck, ticket string) (int, map[string]any) {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: terminalAskPath + "?ticket=" + url.QueryEscape(ticket), host: "localhost:7710", from: thisComputer})
	var said map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &said)
	return w.Code, said
}

// answerAsk is the page's answer, with the session given (a window's, or a browser's).
func answerAsk(t *testing.T, d *Deck, session, id, code, answer string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, d, call{method: "POST", target: "/setup/terminal/answer?id=" + url.QueryEscape(id) + "&code=" + url.QueryEscape(code) + "&answer=" + answer,
		host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: map[string]string{setupHeader: "1", "Cookie": setupCookie() + "=" + session}})
}

// shownAsks is the asks the page's state shows.
func shownAsks(t *testing.T, d *Deck) []map[string]any {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	var state struct {
		Asks []map[string]any `json:"terminalAsks"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	return state.Asks
}

func shortAskWait(t *testing.T) {
	saved := askWait
	askWait = 10 * time.Millisecond
	t.Cleanup(func() { askWait = saved })
}

// A terminal (smolcorp demo, status, desks) gets a tool's key only after the owner's Allow, with the code it printed:
// the page sees the ask's folder and code, never its ticket; a wrong code or a no gives nothing; the key comes once,
// on the ticket alone, and opens what a tool's opens, not the page's own routes.
func TestATerminalGetsAToolKeyOnlyOnTheOwnersAllow(t *testing.T) {
	d := newMacTestDeck(t)
	shortAskWait(t)
	if w := serve(t, d, call{method: "POST", target: terminalAskPath + "?kind=tool", host: "localhost:7710", from: thisComputer}); w.Code != http.StatusForbidden {
		t.Errorf("an ask without the setup header (a web page's): %d", w.Code)
	}
	if w := serve(t, d, call{method: "POST", target: terminalAskPath + "?kind=tool", host: "192.168.1.20:7710", from: "192.168.1.20:50000",
		bare: true, headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusNotFound {
		t.Errorf("an ask from the Wi-Fi: %d", w.Code)
	}
	if code, _ := askFromTerminal(t, d, "everything", "/work"); code != http.StatusBadRequest {
		t.Errorf("an unknown kind: %d", code)
	}
	code, asked := askFromTerminal(t, d, askTool, "/work/demo\x1b[2J")
	ticket, _ := asked["ticket"].(string)
	shownCode, _ := asked["code"].(string)
	if code != http.StatusOK || ticket == "" || len(shownCode) != 7 {
		t.Fatalf("the ask: %d %v", code, asked)
	}
	shown := shownAsks(t, d)
	if len(shown) != 1 || shown[0]["code"] != shownCode || shown[0]["folder"] != "/work/demo[2J" || shown[0]["kind"] != askTool {
		t.Fatalf("the page shows %v", shown)
	}
	if data, _ := json.Marshal(shown); strings.Contains(string(data), ticket) {
		t.Error("the page is shown the ticket")
	}
	// A tool's read of the state never lists them.
	toolState := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, asTool: true})
	if strings.Contains(toolState.Body.String(), "terminalAsks") {
		t.Error("a tool is shown the asks")
	}
	if code, said := waitFromTerminal(t, d, ticket); code != http.StatusOK || said["state"] != "waiting" {
		t.Errorf("before an answer: %d %v", code, said)
	}
	d.mu.Lock()
	page := d.access.NewSetupSession() // the Settings window's
	d.mu.Unlock()
	id, _ := shown[0]["id"].(string)
	if w := answerAsk(t, d, page, id, "0-0-0-0", "allow"); w.Code != http.StatusNotFound || shownCode == "0-0-0-0" {
		t.Errorf("an Allow with another code: %d", w.Code)
	}
	// Only the page answers: a tool's key, the GM's token or no key can't.
	for name, headers := range map[string]map[string]string{"a tool": {setupKeyHeader: d.access.SetupKey}, "nobody": {}} {
		all := map[string]string{setupHeader: "1"}
		for k, v := range headers {
			all[k] = v
		}
		w := serve(t, d, call{method: "POST", target: "/setup/terminal/answer?id=" + id + "&code=" + url.QueryEscape(shownCode) + "&answer=allow",
			host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: all})
		if w.Code < 400 {
			t.Errorf("%s answered: %d", name, w.Code)
		}
	}
	if w := answerAsk(t, d, page, id, shownCode, "allow"); w.Code != http.StatusOK {
		t.Fatalf("the owner's Allow: %d %s", w.Code, w.Body)
	}
	if len(shownAsks(t, d)) != 0 {
		t.Error("an answered ask is still shown")
	}
	if code, said := waitFromTerminal(t, d, "fake-not-the-ticket"); code != http.StatusNotFound || said["key"] != nil {
		t.Errorf("another ticket: %d %v", code, said)
	}
	code, said := waitFromTerminal(t, d, ticket)
	key, _ := said["key"].(string)
	if code != http.StatusOK || said["state"] != "allowed" || key == "" || key == d.access.SetupKey {
		t.Fatalf("the terminal's key: %d %v", code, said["state"])
	}
	if again, said := waitFromTerminal(t, d, ticket); again != http.StatusNotFound || said["key"] != nil {
		t.Errorf("the key came twice: %d", again)
	}
	// It opens what a tool's opens, and no more.
	with := func(method, target string) int {
		return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupKeyHeader: key}}).Code
	}
	if got := with("GET", "/setup/state"); got != http.StatusOK {
		t.Errorf("the state with the terminal's key: %d", got)
	}
	if got := with("POST", "/setup/pair"); got != http.StatusForbidden {
		t.Errorf("pairing with the terminal's key: %d, want 403", got)
	}
	if got := with("POST", "/setup/terminal/answer?id=x&code=1&answer=allow"); got != http.StatusForbidden {
		t.Errorf("answering an ask with the terminal's key: %d, want 403", got)
	}

	// A no gives nothing, and says so.
	_, asked = askFromTerminal(t, d, askTool, "/work")
	ticket, _ = asked["ticket"].(string)
	shown = shownAsks(t, d)
	id, _ = shown[0]["id"].(string)
	if w := answerAsk(t, d, page, id, asked["code"].(string), "deny"); w.Code != http.StatusOK {
		t.Fatalf("the owner's no: %d", w.Code)
	}
	if code, said := waitFromTerminal(t, d, ticket); code != http.StatusOK || said["state"] != "denied" || said["key"] != nil {
		t.Errorf("after a no: %d %v", code, said)
	}
}

// Off a Mac no terminal is given a key (the owner's B8 STRICT): only Smolcorp's own window, where there is one, and the
// paired phone may answer, and neither can yet. The ask is refused at once with why, in words, which Setup says too,
// and no page answers one, whatever its session.
func TestOffAMacNoTerminalIsGivenAKey(t *testing.T) {
	d := newTestDeck(t)
	code, said := askFromTerminal(t, d, askTool, "/work")
	if code != http.StatusForbidden || said != nil && said["ticket"] != nil {
		t.Fatalf("a tool's ask off a Mac: %d %v", code, said)
	}
	w := serve(t, d, call{method: "POST", target: terminalAskPath + "?kind=tool", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if !strings.Contains(w.Body.String(), "paired phone") || !strings.Contains(w.Body.String(), "browser can't") {
		t.Errorf("the terminal is told %q", w.Body)
	}
	if len(d.asks.waiting()) != 0 {
		t.Error("an ask waits off a Mac")
	}
	linux, windows := terminalAskOff("linux"), terminalAskOff("windows")
	if !strings.Contains(linux, "only your paired phone") || strings.Contains(linux, "own window") || !strings.Contains(windows, "Smolcorp's own window or your paired phone") {
		t.Errorf("the words: linux %q, windows %q", linux, windows)
	}
	// An ask already waiting (as from before an update) is answered by no page off a Mac.
	ask, _ := d.asks.add(askTool, "/work")
	d.mu.Lock()
	browser, window := d.access.NewSetupSessionFrom("fake-link-token"), d.access.NewSetupSession()
	d.mu.Unlock()
	for name, session := range map[string]string{"a link's session": browser, "a window's session": window} {
		if w := answerAsk(t, d, session, ask.id, ask.code, "allow"); w.Code != http.StatusForbidden {
			t.Errorf("%s answered off a Mac: %d", name, w.Code)
		}
	}
	d.mu.Lock()
	given := len(d.access.toolKeys)
	d.mu.Unlock()
	if given != 0 {
		t.Errorf("%d tool keys given off a Mac", given)
	}
	// Setup says why, for the page alone, in the step of its own.
	page := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	if !strings.Contains(page.Body.String(), `"terminalAsksOff"`) {
		t.Error("Setup's state doesn't say why")
	}
	html, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="step-terminal-asks-off"`, "$('terminal-asks-off').textContent = state.terminalAsksOff || '';"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("setup.html lacks %q", want)
		}
	}
}

// Two answers to one ask (a double click on Allow, or a Deny racing it): the first alone is taken, so the GM's token
// is rotated once and the terminal is handed the token that works.
func TestAnAskIsAnsweredOnce(t *testing.T) {
	d := newMacTestDeck(t)
	shortAskWait(t)
	_, asked := askFromTerminal(t, d, askTool, "/work")
	shown := shownAsks(t, d)
	if len(shown) != 1 {
		t.Fatalf("shown %v", shown)
	}
	id, code := shown[0]["id"].(string), asked["code"].(string)
	claimed := d.asks.claim(id, code)
	if claimed == nil || d.asks.claim(id, code) != nil {
		t.Fatal("an ask was claimed twice")
	}
	if state, _, _ := d.asks.take(asked["ticket"].(string)); state != "waiting" {
		t.Errorf("its terminal is told %q while it is answered", state)
	}
	d.mu.Lock()
	window := d.access.NewSetupSession()
	d.mu.Unlock()
	if w := answerAsk(t, d, window, id, code, "deny"); w.Code != http.StatusNotFound {
		t.Errorf("a Deny racing the Allow: %d", w.Code)
	}
	d.asks.release(claimed) // as when the key couldn't be made
	if w := answerAsk(t, d, window, id, code, "allow"); w.Code != http.StatusOK {
		t.Errorf("the Allow after a release: %d %s", w.Code, w.Body)
	}
	if w := answerAsk(t, d, window, id, code, "allow"); w.Code != http.StatusNotFound {
		t.Errorf("a second Allow: %d", w.Code)
	}
}

// Under the Mac app only Smolcorp's own window answers an ask: Setup in a browser, whose cookie other programs can
// read, is refused (the owner's B8).
func TestUnderTheMacAppOnlyItsWindowAnswersATerminal(t *testing.T) {
	d := newMacTestDeck(t)
	_, asked := askFromTerminal(t, d, askTool, "/work")
	shown := shownAsks(t, d)
	if len(shown) != 1 {
		t.Fatalf("shown %v", shown)
	}
	id, code := shown[0]["id"].(string), asked["code"].(string)
	d.mu.Lock()
	browser, window := d.access.NewSetupSessionFrom("fake-link-token"), d.access.NewSetupSession()
	d.mu.Unlock()
	if w := answerAsk(t, d, browser, id, code, "allow"); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Settings window") {
		t.Errorf("a browser's Setup answered: %d %s", w.Code, w.Body)
	}
	if w := answerAsk(t, d, window, id, code, "allow"); w.Code != http.StatusOK {
		t.Errorf("the window's answer: %d %s", w.Code, w.Body)
	}
}

// smolcorp gm login: refused off a Mac and with "Allow a General Manager started by hand" off; once allowed in the
// window, the terminal gets a new GM token, and the one before stops working.
func TestGMLoginGivesANewTokenOnceAllowedInTheWindow(t *testing.T) {
	d := newTestDeck(t)
	if code, _ := askFromTerminal(t, d, askGM, "/work"); code != http.StatusForbidden {
		t.Errorf("a GM's login off a Mac: %d", code)
	}
	isolate(t)
	withMac(t)
	d = startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	shortAskWait(t)
	if code, _ := askFromTerminal(t, d, askGM, "/work"); code != http.StatusForbidden {
		t.Errorf("a GM's login with the setting off: %d", code)
	}
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/gm/by-hand?enabled=1", nil); code != http.StatusOK {
		t.Fatalf("by hand on: %d", code)
	}
	before := tokenNow(t, d)
	code, asked := askFromTerminal(t, d, askGM, "/work/gm")
	if code != http.StatusOK {
		t.Fatalf("the GM's ask: %d", code)
	}
	shown := shownAsks(t, d)
	if len(shown) != 1 || shown[0]["kind"] != askGM {
		t.Fatalf("shown %v", shown)
	}
	d.mu.Lock()
	window := d.access.NewSetupSession()
	d.mu.Unlock()
	if w := answerAsk(t, d, window, shown[0]["id"].(string), asked["code"].(string), "allow"); w.Code != http.StatusOK {
		t.Fatalf("the Allow: %d %s", w.Code, w.Body)
	}
	_, said := waitFromTerminal(t, d, asked["ticket"].(string))
	token, _ := said["key"].(string)
	if token == "" || token == before || !worksAsGM(d, token) || worksAsGM(d, before) {
		t.Errorf("the GM's token: given %v, new %v, works %v, the one before works %v", token != "", token != before, worksAsGM(d, token), worksAsGM(d, before))
	}
	if tokenNow(t, d) != token {
		t.Error("the token given isn't the GM's")
	}
}

// A program can't bury the owner's ask in its own, nor make one whose code is the owner's: at most three wait, and no
// two waiting share a code.
func TestAProgramCantBuryOrCopyTheOwnersAsk(t *testing.T) {
	d := newMacTestDeck(t)
	codes := map[string]bool{}
	for i := 0; i < asksMost; i++ {
		code, asked := askFromTerminal(t, d, askTool, "/work")
		if code != http.StatusOK {
			t.Fatalf("ask %d: %d", i+1, code)
		}
		codes[asked["code"].(string)] = true
	}
	if len(codes) != asksMost {
		t.Errorf("two asks share a code: %v", codes)
	}
	if code, _ := askFromTerminal(t, d, askTool, "/work"); code != http.StatusTooManyRequests {
		t.Errorf("one ask too many: %d", code)
	}
	if got := askFolder(strings.Repeat("a", 400)); len([]rune(got)) != askFolderMost+1 {
		t.Errorf("a long folder is shown as %d runes", len([]rune(got)))
	}
}

// The demo's office asks for its key once, and keeps it in memory: never a file.
func TestTheDemoAsksForItsKeyOnce(t *testing.T) {
	isolate(t)
	asked := 0
	key := askedOnce(func() (string, error) { asked++; return "fake-demo-key", nil })
	for range 3 {
		if got, err := key(); err != nil || string(got) != "fake-demo-key" {
			t.Fatalf("%q %v", got, err)
		}
	}
	if asked != 1 {
		t.Errorf("asked %d times", asked)
	}
	entries, _ := os.ReadDir(supportDir)
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(supportDir, entry.Name()))
		if strings.Contains(string(data), "fake-demo-key") {
			t.Errorf("%s holds the demo's key", entry.Name())
		}
	}
}

// Setup copies no key it can avoid copying, and beside the portable office's copy buttons says who can read what's
// copied.
func TestTheClipboardLineIsBesideThePortableOfficesCopy(t *testing.T) {
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, copyButton := range []string{`<button class="btn" id="viewer-link-copy">Copy</button>`, `<button class="btn" id="viewer-secret-copy">Copy</button>`} {
		at := strings.Index(html, copyButton)
		if at < 0 {
			t.Fatalf("no %s", copyButton)
		}
		if near := html[at:min(len(html), at+600)]; !strings.Contains(near, "any program running as you can read, your hires included") {
			t.Errorf("no clipboard line beside %s", copyButton)
		}
	}
	// The GM's token has no file to show or copy.
	if strings.Contains(html, "gm-token-copy") || strings.Contains(html, "Writes the token to the file above") {
		t.Error("the page still offers the GM's token file")
	}
	// Nor does Find a setting's jump to it name one (RMSY126k nit 3).
	if strings.Contains(html, "the file for one started by hand") || !strings.Contains(html, "What it is, and how one you start yourself gets it") {
		t.Error("Find a setting still names the GM's token file")
	}
}
