package desktop

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Smolcorp's own Settings window off the Mac (FXOWNSETTINGS, settingswindow.go). These rows run in-process on every
// computer: "Windows with WebView2" is settingsWebView said true, with a stand-in for the window, which opens nothing.
// settingswindow_windows_test.go holds Windows' own rows, and the live window, run by hand.

// standInSettings stands for the Settings window: opening it keeps the run it was opened with and says it's there;
// bringing it forward keeps what it was brought forward on. No window opens.
type standInSettings struct {
	mu       sync.Mutex
	runs     []settingsRun
	forwards []string
}

func (s *standInSettings) seen() ([]settingsRun, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]settingsRun{}, s.runs...), append([]string{}, s.forwards...)
}

// onWindowsWithWebView2 makes this test's computer a Windows with WebView2, with a stand-in window.
func onWindowsWithWebView2(t *testing.T) *standInSettings {
	t.Helper()
	saved := settingsWebView
	settingsWebView = func() bool { return true }
	t.Cleanup(func() { settingsWebView = saved })
	return standInSettingsWindow(t)
}

// standInSettingsWindow swaps the Settings window for a stand-in that opens nothing.
func standInSettingsWindow(t *testing.T) *standInSettings {
	t.Helper()
	savedStart := startSettingsWindow
	window := &standInSettings{}
	startSettingsWindow = func(_ *Deck, run settingsRun) error {
		window.mu.Lock()
		window.runs = append(window.runs, run)
		window.mu.Unlock()
		settingsWindow.opened(func(then string) {
			window.mu.Lock()
			window.forwards = append(window.forwards, then)
			window.mu.Unlock()
		})
		return nil
	}
	t.Cleanup(func() {
		startSettingsWindow = savedStart
		settingsWindow.closed()
	})
	return window
}

// withoutWebView2 is Linux, or a Windows without WebView2: no Settings window of Smolcorp's own.
func withoutWebView2(t *testing.T) {
	t.Helper()
	saved := settingsWebView
	settingsWebView = func() bool { return false }
	t.Cleanup(func() { settingsWebView = saved })
}

var windowTokenInScript = regexp.MustCompile(`value: ("[^"]*")`)

// openWindowSession is what setup.html does in the Settings window: it hands over the token the window's script gave
// it, and keeps the cookie it gets. "" when none is given.
func openWindowSession(t *testing.T, d *Deck, run settingsRun) (string, int) {
	t.Helper()
	found := windowTokenInScript.FindStringSubmatch(run.script)
	var token string
	if found == nil || json.Unmarshal([]byte(found[1]), &token) != nil {
		t.Fatalf("no token in the window's script %q", run.script)
	}
	return handOverSetup(t, d, windowSetupHeader, token)
}

// handOverSetup is POST /setup/session with a token in header, as the page sends it: the cookie it got, and the status.
func handOverSetup(t *testing.T, d *Deck, header, token string) (string, int) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", header: token}})
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == setupCookie() {
			return cookie.Value, w.Code
		}
	}
	return "", w.Code
}

// followLink is what setup.html does with a link the browser was opened on: a link's token after #o= is handed over;
// #tap has nothing to hand over (it asks the phone: setupask.go).
func followLink(t *testing.T, d *Deck, link string) string {
	t.Helper()
	_, fragment, _ := strings.Cut(link, "#")
	for _, part := range strings.Split(fragment, "&") {
		if token, ok := strings.CutPrefix(part, "o="); ok {
			session, _ := handOverSetup(t, d, setupTokenHeader, token)
			return session
		}
	}
	return ""
}

// The lockout (FXOWNSETTINGS' fail-first): on Windows, once a phone is paired, the tray's Open Setup…, the office's ⚙︎
// and `smolcorp setup --open` each open Smolcorp's own Settings window, whose session reads Setup and changes it.
// Before, each landed on /setup#tap in the browser, a page with no session (401), or opened nothing.
func TestOnWindowsWithAPhonePairedTheTrayTheGearAndTheCommandOpenSetupInItsOwnWindow(t *testing.T) {
	d := newTestDeck(t)
	pair(t, d)
	window := onWindowsWithWebView2(t)
	links := browserOpens(t)
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	writeFile(t, hookKeyFile, d.access.HookKey+"\n")
	defer func(was string) { setupSystem = was }(setupSystem)
	setupSystem = "windows"

	for _, opener := range []struct {
		name string
		open func() string
	}{
		{"the tray's Open Setup…", func() string { return errorText(d.openSetup("")) }},
		{"the office's ⚙︎", func() string {
			got := serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
			return http.StatusText(got.Code) + " " + strings.TrimSpace(got.Body.String())
		}},
		{"smolcorp setup --open", func() string {
			var out strings.Builder
			setupClient{base: server.URL, http: server.Client()}.openSetup(&out)
			return strings.TrimSpace(out.String())
		}},
	} {
		runsBefore, _ := window.seen()
		linksBefore := len(*links)
		said := opener.open()
		runs, _ := window.seen()
		var session, how string
		switch {
		case len(runs) > len(runsBefore):
			how = "the own window at " + runs[len(runs)-1].url
			session, _ = openWindowSession(t, d, runs[len(runs)-1])
		case len(*links) > linksBefore:
			how = "the browser at " + strings.SplitN((*links)[len(*links)-1], "#", 2)[1]
			session = followLink(t, d, (*links)[len(*links)-1])
		default:
			t.Errorf("%s opened nothing (%q)", opener.name, said)
			continue
		}
		reads := withSession(t, d, session, "GET", "/setup/state", "").Code
		changes := withSession(t, d, session, "POST", "/setup/pair", "").Code
		t.Logf("%s: %s, said %q; GET /setup/state %d, POST /setup/pair %d", opener.name, how, said, reads, changes)
		if reads != http.StatusOK || changes != http.StatusOK {
			t.Errorf("%s: %s reaches Setup with %d and changes it with %d, want 200 and 200", opener.name, how, reads, changes)
		}
		settingsWindow.closed() // the owner closes it: the next opens a new one
	}
	if len(*links) != 0 {
		t.Errorf("the browser opened %v, want only the window", *links)
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// No second window: the tray, the gear, the command and the tray's Pair New Phone… bring the open one forward, on what
// they open; once it's closed the next opens a new one. Its address holds no token; its script gives the token to
// Setup on 127.0.0.1 alone.
func TestTheOwnSettingsWindowIsNeverASecondOne(t *testing.T) {
	d := newTestDeck(t)
	window := onWindowsWithWebView2(t)
	links := browserOpens(t)
	if err := d.openSetup(""); err != nil {
		t.Fatal(err)
	}
	gear := serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	command := serve(t, d, call{method: "POST", target: raisePath + "?show=settings", host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey}})
	_ = d.openSetup("pair")
	runs, forwards := window.seen()
	t.Logf("opened %d, brought forward on %q; the gear %d, the command %d", len(runs), forwards, gear.Code, command.Code)
	if len(runs) != 1 || strings.Join(forwards, ",") != ",,pair" || gear.Code != http.StatusNoContent || command.Code != http.StatusNoContent || len(*links) != 0 {
		t.Errorf("opened %d windows, brought forward on %q, gear %d, command %d, browser %v; want 1, [\"\" \"\" pair], 204, 204, none",
			len(runs), forwards, gear.Code, command.Code, *links)
	}
	first := runs[0]
	if first.url != setupBase()+"/setup" || !strings.HasPrefix(first.script, "if (location.origin === \""+setupBase()+"\" && location.pathname === '/setup') ") {
		t.Errorf("the window's address %q and script %q", first.url, first.script)
	}
	if found := windowTokenInScript.FindStringSubmatch(first.script); found == nil || strings.Contains(first.url, strings.Trim(found[1], `"`)) {
		t.Errorf("the window's token: in its script %v, in its address %q", found, first.url)
	}

	settingsWindow.closed()
	_ = d.openSetup("viewer")
	if runs, _ = window.seen(); len(runs) != 2 || runs[1].url != setupBase()+"/setup#viewer" {
		t.Errorf("after closing, the tray's viewer item opened %d windows, the last at %q", len(runs), runs[len(runs)-1].url)
	}
	if reload := settingsWindowReload("pair"); reload != `location.hash = "pair"; location.reload();` {
		t.Errorf("the open window is brought forward on pair with %q", reload)
	}
}

// Strict on Windows, now that Smolcorp has its own window there (FXBROWSERSTRICT's class, ownSettingsWindow): a
// browser's Setup only looks, in plain words, and says where the window is; the window's session changes things; no
// tap opens Setup there; the window's token opens one session, once, and is no link's. Without WebView2 nothing
// changes: the browser's Setup is the page.
func TestOnWindowsWithItsOwnWindowSetupInABrowserOnlyLooks(t *testing.T) {
	turnOn(t, &spendAsksOn) // the ask it answers is a spend ask, off in this version (FXDORMANTOFF)
	d, _, id := deckWithAnAsk(t)
	onWindowsWithWebView2(t)
	browser := browserSession(d)
	run := d.newSettingsRun("")
	window, opened := openWindowSession(t, d, run)
	if opened != http.StatusNoContent || window == "" {
		t.Fatalf("the window's token: %d, session %v", opened, window != "")
	}
	for _, c := range []struct{ method, target, body, want string }{
		{"POST", "/setup/sandbox?on=0", "", strictChangeWords},
		{"POST", "/setup/pair", "", strictChangeWords},
		{"GET", "/setup/qr.png", "", strictReadWords},
		{"POST", "/setup/grants/answer?id=" + id + "&answer=yes", "", strictChangeWords},
		{"POST", "/setup/guard", `{}`, strictChangeWords},
	} {
		got := withSession(t, d, browser, c.method, c.target, c.body)
		if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != c.want {
			t.Errorf("a browser on Windows: %s %s: %d %q, want 403 %q", c.method, c.target, got.Code, got.Body, c.want)
		}
	}
	if got := withSession(t, d, window, "POST", "/setup/pair", ""); got.Code != http.StatusOK {
		t.Fatalf("the window: POST /setup/pair: %d %q", got.Code, got.Body)
	}
	seen := setupStateOf(t, withSession(t, d, browser, "GET", "/setup/state", ""))
	pairing, _ := seen["pairing"].(map[string]any)
	t.Logf("a browser on Windows: the note %q, a code shown %v", seen["windowOnly"], pairing["code"] != nil)
	if _, shown := pairing["code"]; shown || seen["windowOnly"] != windowOnlyNote(false) || !strings.Contains(windowOnlyNote(false), "the tray icon, then Open Setup…") {
		t.Errorf("a browser on Windows: pairing %v, note %v", pairing, seen["windowOnly"])
	}
	if own := setupStateOf(t, withSession(t, d, window, "GET", "/setup/state", "")); own["windowOnly"] != nil {
		t.Errorf("the window's state has the browser's note %v", own["windowOnly"])
	}
	if got := withSession(t, d, window, "POST", "/setup/grants/answer?id="+id+"&answer=yes", ""); got.Code/100 != 2 || grantState(t, d, id) != grantGranted {
		t.Errorf("the window answering the ask: %d %q", got.Code, got.Body)
	}

	// No tap opens Setup beside the window, as on a Mac.
	if code, said := askToOpenSetup(t, d); code != http.StatusForbidden {
		t.Errorf("a tap asked for beside the window: %d %v", code, said)
	}
	// The window's token works once, and only as the window's; a link's opens no window session.
	again, code := openWindowSession(t, d, run)
	link := d.newSettingsRun("")
	asLink, asLinkCode := handOverSetup(t, d, setupTokenHeader, strings.Trim(windowTokenInScript.FindStringSubmatch(link.script)[1], `"`))
	d.mu.Lock()
	linkToken := d.access.NewSetupToken()
	d.mu.Unlock()
	asWindow, asWindowCode := handOverSetup(t, d, windowSetupHeader, linkToken)
	// As a link's, it's refused as a link: 401, or 403 once a phone is paired (no link opens Setup then).
	t.Logf("the window's token again: %d; as a link's: %d; a link's as the window's: %d", code, asLinkCode, asWindowCode)
	if again != "" || code != http.StatusUnauthorized || asLink != "" || asLinkCode/100 != 4 ||
		asWindow != "" || asWindowCode != http.StatusUnauthorized {
		t.Errorf("the window's token again: %d; as a link's: %d; a link's as the window's: %d; want 401, a refusal, 401, no session",
			code, asLinkCode, asWindowCode)
	}

	// Without WebView2 (and on Linux), a browser's Setup is the page, as before.
	withoutWebView2(t)
	if got := withSession(t, d, browser, "POST", "/setup/pair", ""); got.Code != http.StatusOK {
		t.Errorf("without WebView2: the browser's POST /setup/pair: %d %q", got.Code, got.Body)
	}
	if plain := setupStateOf(t, withSession(t, d, browser, "GET", "/setup/state", "")); plain["windowOnly"] != nil {
		t.Errorf("without WebView2: the browser is told %v", plain["windowOnly"])
	}
}

// Without WebView2, `smolcorp setup --open` opens Setup in the browser as the tray would, and says so: with a link
// that works once, or, once a phone is paired, the phone's tap. An app from before (no /raise?show=settings) gets the
// tray's words, as before.
func TestWithoutWebView2SetupOpenOpensTheBrowserAndSaysSo(t *testing.T) {
	d := newTestDeck(t)
	withoutWebView2(t)
	links := browserOpens(t)
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	writeFile(t, hookKeyFile, d.access.HookKey+"\n")
	defer func(was string) { setupSystem = was }(setupSystem)
	setupSystem = "windows"
	command := func(base string) string {
		var out strings.Builder
		setupClient{base: base, http: server.Client()}.openSetup(&out)
		return strings.TrimSpace(out.String())
	}

	said := command(server.URL)
	t.Logf("no phone: %q, opened %v", said, *links)
	if said != settingsInBrowserWords(runtime.GOOS, settingsWindowApp.Load(), false) || len(*links) != 1 || !strings.Contains((*links)[0], "/setup#o=") {
		t.Errorf("no phone: said %q, opened %v", said, *links)
	}
	pair(t, d)
	said = command(server.URL)
	t.Logf("a phone paired: %q, opened %v", said, *links)
	if said != settingsInBrowserWords(runtime.GOOS, settingsWindowApp.Load(), true) || len(*links) != 2 || !strings.HasSuffix((*links)[1], "/setup#tap") {
		t.Errorf("a phone paired: said %q, opened %v", said, *links)
	}
	if !strings.Contains(settingsInBrowserWords("windows", true, false), "WebView2") {
		t.Errorf("Windows' words don't say what's missing: %q", settingsInBrowserWords("windows", true, false))
	}
	if serve := settingsInBrowserWords("windows", false, false); strings.Contains(serve, "WebView2") || !strings.Contains(serve, "smolcorp serve") {
		t.Errorf("smolcorp serve's words on Windows: %q, want why it has no window, never WebView2", serve)
	}

	older := httptest.NewServer(http.NotFoundHandler())
	defer older.Close()
	if said := command(older.URL); !strings.Contains(said, "choose Open Setup… from the Smolcorp icon in the tray") || len(*links) != 2 {
		t.Errorf("an older app: said %q, opened %v", said, *links)
	}
}

// `smolcorp serve` on Windows (RT126FIXREVIEW-OWNSETTINGS N1): it has no tray and no window, so even where WebView2 is
// installed, as on nearly every Windows, Setup stays the browser's: the link serve prints opens a Setup that changes
// things, as before; opening Setup, and the command, open the browser, never a window. Only the app, which says so as
// it starts (RunApp, never RunServer), has the window, and beside it a browser's Setup only looks.
func TestOnWindowsSmolcorpServeKeepsSetupInTheBrowserAsBefore(t *testing.T) {
	requireOnlyTheAppHasTheSettingsWindow(t)
	d := newTestDeck(t)
	savedInstalled, savedApp := settingsWebViewInstalled, settingsWindowApp.Load()
	settingsWebViewInstalled = func() bool { return true }
	t.Cleanup(func() {
		settingsWebViewInstalled = savedInstalled
		settingsWindowApp.Store(savedApp)
	})
	window := standInSettingsWindow(t)
	links := browserOpens(t)

	for _, c := range []struct {
		name            string
		app             bool
		change, raise   int
		window, browser int
		note            any
	}{
		{"smolcorp serve", false, http.StatusOK, http.StatusOK, 0, 2, nil},
		{"the app", true, http.StatusForbidden, http.StatusNoContent, 1, 0, windowOnlyNote(false)},
	} {
		settingsWindowApp.Store(c.app)
		runsBefore, _ := window.seen()
		var printed strings.Builder
		printSetupLink(&printed, d)
		fields := strings.Fields(strings.SplitN(printed.String(), "\n", 2)[0])
		session := followLink(t, d, fields[len(fields)-1])
		changed := withSession(t, d, session, "POST", "/setup/pair", "")
		note := setupStateOf(t, withSession(t, d, session, "GET", "/setup/state", ""))["windowOnly"]
		linksBefore := len(*links)
		_ = d.openSetup("")
		raised := serve(t, d, call{method: "POST", target: raisePath + "?show=settings", host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey}})
		runs, _ := window.seen()
		opened := len(runs) - len(runsBefore)
		t.Logf("%s, WebView2 installed: the printed link's POST /setup/pair %d, its note %v; Setup opened %d windows, the browser %d times; the command %d %q",
			c.name, changed.Code, note, opened, len(*links)-linksBefore, raised.Code, strings.TrimSpace(raised.Body.String()))
		if changed.Code != c.change || note != c.note {
			t.Errorf("%s: the printed link's Setup changes things with %d %q, note %v; want %d, note %v",
				c.name, changed.Code, changed.Body, note, c.change, c.note)
		}
		if opened != c.window || len(*links)-linksBefore != c.browser || raised.Code != c.raise {
			t.Errorf("%s: opening Setup and the command opened %d windows, the browser %d times, answered %d; want %d, %d, %d",
				c.name, opened, len(*links)-linksBefore, raised.Code, c.window, c.browser, c.raise)
		}
		if strings.Contains(raised.Body.String(), "WebView2") {
			t.Errorf("%s: the command says WebView2 is missing, and it's installed: %q", c.name, raised.Body)
		}
		settingsWindow.closed()
	}
}

// requireOnlyTheAppHasTheSettingsWindow stops the test unless the app, RunApp, says it's the app (settingsWindowApp)
// before its server starts, and nothing else in the package ever does: not RunServer, `smolcorp serve`.
func requireOnlyTheAppHasTheSettingsWindow(t *testing.T) {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var stores []string
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			serveAt := -1
			for i, statement := range fn.Body.List {
				ast.Inspect(statement, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.CallExpr:
						if isIdent(n.Fun, "Serve") && serveAt < 0 {
							serveAt = i
						}
						if pick, ok := n.Fun.(*ast.SelectorExpr); ok && isIdent(pick.X, "settingsWindowApp") && pick.Sel.Name == "Store" {
							where := fmt.Sprintf("%s in %s", source, fn.Name.Name)
							if len(n.Args) != 1 || !isIdent(n.Args[0], "true") || fn.Recv != nil || fn.Name.Name != "RunApp" || serveAt >= 0 {
								where += " (not RunApp's settingsWindowApp.Store(true) before its server starts)"
							}
							stores = append(stores, where)
						}
					}
					return true
				})
			}
		}
	}
	if len(stores) != 1 || stores[0] != "app.go in RunApp" {
		t.Fatalf("settingsWindowApp is set %v; want once, by RunApp (app.go) before its server starts, never by RunServer", stores)
	}
}

// setupReachableGaps are the ways into Setup this row knows have no page yet, each with the piece that builds it. A
// gap is a skip that says so by name, never a pass; and the row fails once the page is there while the gap is still
// listed, so the entry can't outlive it.
var setupReachableGaps = map[string]string{}

// The row the owner asked for: Setup is reachable after a phone is paired, on every platform, so a way in can't ship
// with its server half and no page again. For each platform it opens Setup as that platform's menu does, then does
// what the page (or the Mac app's window) does with what was opened, and reads and changes Setup with the session it
// got. In-process, so it runs on Mac, Linux and Windows CI alike; at #tap the page's half is setup.html's own start,
// in node against a live server of this package's router (tapAtSetupsStart), and is skipped without node.
func TestSetupIsReachableAfterAPhoneIsPairedOnEveryPlatform(t *testing.T) {
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	property := regexp.MustCompile(`defineProperty\(window, '([A-Za-z]+)'`).FindStringSubmatch(settingsWindowScript(setupBase(), "fake"))
	for _, platform := range []struct {
		name string
		set  func(t *testing.T, d *Deck)
	}{
		{"a Mac (Smolcorp's Settings window)", func(t *testing.T, d *Deck) { hostedByTheMacApp(d) }},
		{"Windows with WebView2 (Smolcorp's own Settings window)", func(t *testing.T, d *Deck) { onWindowsWithWebView2(t) }},
		{"Linux, and Windows without WebView2 (the browser)", func(t *testing.T, d *Deck) { withoutWebView2(t) }},
	} {
		t.Run(platform.name, func(t *testing.T) {
			d, phoneKey := tapDeck(t)
			links := browserOpens(t)
			platform.set(t, d)
			var session, way string
			switch {
			case d.macHost() != nil:
				// The Mac app's Settings… asks the core for a window session (core_darwin.go's setup-session), and
				// its window holds it as its cookie (SettingsWindow.swift): no page half to find in setup.html.
				way, session = "the Mac's Settings window", d.newSetupSession("")
			case settingsWebView():
				window := settingsRunsOf(t, d)
				way = "the own window at " + window.url
				if property == nil || !strings.Contains(string(page), "window."+property[1]) || !strings.Contains(string(page), "'"+windowSetupHeader+"'") {
					t.Fatalf("setup.html doesn't take the window's token (%v, %s)", property, windowSetupHeader)
				}
				session, _ = openWindowSession(t, d, window)
			default:
				if err := d.openSetup(""); err != nil || len(*links) != 1 {
					t.Fatalf("the tray opened %v (%v)", *links, err)
				}
				_, fragment, _ := strings.Cut((*links)[0], "#")
				way = "the browser at #" + fragment
				if fragment != setupTapFragment {
					session = followLink(t, d, (*links)[0])
					break
				}
				// The page's own start at the menu's #tap, in node, against a live server: it asks, the owner's yes on
				// the phone over Tailscale opens its session, and it loads Setup with it (tapAtSetupsStart).
				node := nodeOrSkip(t)
				waited, end, _ := tapAtSetupsStart(t, node, d, phoneKey, "#"+fragment, "allow")
				session = strings.TrimPrefix(end.Cookie, setupCookie()+"=")
				t.Logf("%s: the page showed %q, then its start %s, asked %v", way, waited, end.Ended, end.Asked)
				gap, listed := setupReachableGaps["tap"]
				if waited != nil && listed {
					t.Errorf("setup.html asks for the tap now: take \"tap\" out of setupReachableGaps")
				}
				if waited == nil {
					if !listed {
						t.Fatalf("%s: setup.html's start never asks for the tap, so no page reaches Setup (it asked %v)", way, end.Asked)
					}
					t.Skipf("%s: setup.html's start never asks for the tap. %s", way, gap)
				}
				if end.Ended != "returned" || end.Tapping || !end.Rendered {
					t.Fatalf("%s: after the owner's yes the page's start %s, tapping %v, drew Setup %v: it never loads Setup", way, end.Ended, end.Tapping, end.Rendered)
				}
			}
			reads := withSession(t, d, session, "GET", "/setup/state", "").Code
			changes := withSession(t, d, session, "POST", "/setup/pair", "").Code
			t.Logf("%s: %s; GET /setup/state %d, POST /setup/pair %d", platform.name, way, reads, changes)
			if reads != http.StatusOK || changes != http.StatusOK {
				t.Errorf("%s: %s reaches Setup with %d and changes it with %d, want 200 and 200", platform.name, way, reads, changes)
			}
		})
	}
}

// settingsRunsOf opens Setup from the tray with a stand-in window there, and is the run it opened.
func settingsRunsOf(t *testing.T, d *Deck) settingsRun {
	t.Helper()
	var opened []settingsRun
	saved := startSettingsWindow
	startSettingsWindow = func(_ *Deck, run settingsRun) error { opened = append(opened, run); return nil }
	defer func() { startSettingsWindow = saved; settingsWindow.closed() }()
	if err := d.openSetup(""); err != nil || len(opened) != 1 {
		t.Fatalf("the tray opened %d windows (%v)", len(opened), err)
	}
	return opened[0]
}
