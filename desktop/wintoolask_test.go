package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// A terminal's ask on Windows (FXWINTOOLASK, terminalask.go): answered in Smolcorp's own Settings window there, and
// only there, as on a Mac. These rows run in-process on every computer: "Windows with WebView2" is settingsWebView
// said true (ownsettings_test.go), and the program Windows would name is stood in. wintoolask_windows_test.go drives
// the same through Windows' own tables on the Windows CI.

// aWindowsAsker is the program Windows names for these rows' asks.
var aWindowsAsker = askAsker{tried: true, program: `C:\Users\fake\AppData\Local\Programs\Smolcorp\smolcorp.exe`,
	from: []string{"pwsh.exe", "WindowsTerminal.exe"}}

// standInAsker makes every ask come from asker, as Windows would say it.
func standInAsker(t *testing.T, asker askAsker) {
	t.Helper()
	saved := askingProgram
	askingProgram = func(*http.Request) askAsker { return asker }
	t.Cleanup(func() { askingProgram = saved })
}

// onWindowsWithItsWindow is a Windows deck with WebView2, a paired phone (to try it too), and the window's own
// session, opened as setup.html opens it there.
func onWindowsWithItsWindow(t *testing.T) (d *Deck, phoneKey, window string) {
	t.Helper()
	d, phoneKey = tapDeck(t)
	onWindowsWithWebView2(t)
	standInAsker(t, aWindowsAsker)
	window, opened := openWindowSession(t, d, d.newSettingsRun(""))
	if opened != http.StatusNoContent || window == "" {
		t.Fatalf("the window's session: %d", opened)
	}
	return d, phoneKey, window
}

// toolKeysGiven is how many tool keys the app has given.
func toolKeysGiven(d *Deck) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.access.toolKeys)
}

// On Windows with its own window: a terminal's ask is shown in the window with its code, kind, folder, and the program
// Windows says asked, and what started it; only that window answers it. A browser's Setup is refused in the plain
// words, and so is the phone, by either of its routes, and a tool's key. Allowed, the terminal gets a key once, which
// reads /setup/state; denied or run out, it gets nothing.
func TestOnWindowsATerminalsAskIsAnsweredOnlyInSmolcorpsOwnWindow(t *testing.T) {
	d, phoneKey, window := onWindowsWithItsWindow(t)
	code, asked := askFromTerminal(t, d, askTool, `C:\work\baguette`)
	ticket, _ := asked["ticket"].(string)
	shownCode, _ := asked["code"].(string)
	if code != http.StatusOK || ticket == "" || len(shownCode) != 7 {
		t.Fatalf("the ask on Windows: %d %v", code, asked)
	}
	if asked["where"] != "Smolcorp's Settings window (the tray icon, then Open Setup…)" {
		t.Errorf("the terminal is told to look in %q", asked["where"])
	}
	// The window's state: the ask, its code, kind and folder, and the program Windows says asked, never the ticket.
	state := setupStateOf(t, withSession(t, d, window, "GET", "/setup/state", ""))
	asks, _ := state["terminalAsks"].([]any)
	if len(asks) != 1 {
		t.Fatalf("the window shows %v", state["terminalAsks"])
	}
	shown := asks[0].(map[string]any)
	t.Logf("the window shows: %v", shown)
	if shown["code"] != shownCode || shown["kind"] != askTool || shown["folder"] != `C:\work\baguette` ||
		shown["program"] != aWindowsAsker.program || !slices.Equal(anyStrings(shown["startedFrom"]), aWindowsAsker.from) {
		t.Errorf("the window shows %v", shown)
	}
	if data, _ := json.Marshal(state); strings.Contains(string(data), ticket) || state["terminalAsksOff"] != nil {
		t.Errorf("the window's state has the ticket, or says asks are off: %v", state["terminalAsksOff"])
	}
	id, _ := shown["id"].(string)

	// A browser's Setup, the phone (its tap route and the page's), and a tool's key: refused, nothing given.
	browser := browserSession(d)
	if w := answerAsk(t, d, browser, id, shownCode, "allow"); w.Code != http.StatusForbidden ||
		!strings.Contains(w.Body.String(), "Settings window, not in a browser") {
		t.Errorf("a browser's Setup on Windows answered: %d %q", w.Code, w.Body)
	}
	phone := callerNamed(t, "phone-wireguard")
	for _, target := range []string{"/setup/terminal/answer?id=" + url.QueryEscape(id) + "&code=" + url.QueryEscape(shownCode) + "&answer=allow",
		tapSetupTarget(id, shownCode, "allow")} {
		if status, body := probeRouteWith(t, d, "POST", target, "", phoneKey, phone); letIn(status) && status/100 == 2 {
			t.Errorf("the phone answered at %s: %d %q", target, status, body)
		}
	}
	if _, body := phoneSetupAsks(t, d, phoneKey); strings.Contains(body, id) || strings.Contains(body, shownCode) {
		t.Error("the phone is shown the terminal's ask")
	}
	asTool := serve(t, d, call{method: "POST", target: "/setup/terminal/answer?id=" + id + "&code=" + url.QueryEscape(shownCode) + "&answer=allow",
		host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: map[string]string{setupHeader: "1", setupKeyHeader: d.access.SetupKey}})
	if asTool.Code != http.StatusForbidden {
		t.Errorf("a tool's key answered: %d", asTool.Code)
	}
	if given := toolKeysGiven(d); given != 0 {
		t.Fatalf("%d keys given before the window's Allow", given)
	}
	if code, said := waitFromTerminal(t, d, ticket); code != http.StatusOK || said["state"] != "waiting" {
		t.Errorf("after the refusals the terminal is told %d %v", code, said)
	}

	// The window's Allow: the key comes once, and reads Setup's state as a tool.
	if w := answerAsk(t, d, window, id, shownCode, "allow"); w.Code != http.StatusOK {
		t.Fatalf("the window's Allow: %d %q", w.Code, w.Body)
	}
	_, said := waitFromTerminal(t, d, ticket)
	key, _ := said["key"].(string)
	if said["state"] != "allowed" || key == "" || key == d.access.SetupKey {
		t.Fatalf("the terminal's answer: %v", said["state"])
	}
	if again, said := waitFromTerminal(t, d, ticket); again != http.StatusNotFound || said["key"] != nil {
		t.Errorf("the key came twice: %d", again)
	}
	read := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupKeyHeader: key}})
	if read.Code != http.StatusOK || strings.Contains(read.Body.String(), "terminalAsks\"") {
		t.Errorf("GET /setup/state with the tool's key: %d", read.Code)
	}

	// Denied: nothing. Run out: nothing, and nothing to answer.
	_, asked = askFromTerminal(t, d, askTool, `C:\work`)
	deniedTicket := asked["ticket"].(string)
	if w := answerAsk(t, d, window, d.asks.waiting()[0]["id"].(string), asked["code"].(string), "deny"); w.Code != http.StatusOK {
		t.Fatalf("the window's no: %d", w.Code)
	}
	if code, said := waitFromTerminal(t, d, deniedTicket); code != http.StatusOK || said["state"] != "denied" || said["key"] != nil {
		t.Errorf("after a no: %d %v", code, said)
	}
	_, asked = askFromTerminal(t, d, askTool, `C:\work`)
	lateTicket, lateCode, lateID := asked["ticket"].(string), asked["code"].(string), d.asks.waiting()[0]["id"].(string)
	d.asks.mu.Lock()
	for _, ask := range d.asks.all {
		ask.asked = ask.asked.Add(-askLifetime - time.Second)
	}
	d.asks.mu.Unlock()
	if w := answerAsk(t, d, window, lateID, lateCode, "allow"); w.Code != http.StatusNotFound {
		t.Errorf("an Allow after the ask ran out: %d", w.Code)
	}
	if code, said := waitFromTerminal(t, d, lateTicket); code != http.StatusNotFound || said["key"] != nil {
		t.Errorf("a terminal whose ask ran out: %d %v", code, said)
	}
	if given := toolKeysGiven(d); given != 1 {
		t.Errorf("%d keys given, want the one allowed", given)
	}
}

// windowOwner is the owner at Smolcorp's own window: each ask waiting is answered, over the app's real port, with the
// window's session, as setup.html answers it. The asks it saw are kept.
type windowOwner struct {
	mu   sync.Mutex
	seen []map[string]any
}

func (o *windowOwner) asks() []map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.seen)
}

func ownerAtTheWindow(t *testing.T, d *Deck, server *httptest.Server, window, answer string) *windowOwner {
	t.Helper()
	owner := &windowOwner{}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			for _, ask := range d.asks.waiting() {
				owner.mu.Lock()
				owner.seen = append(owner.seen, ask)
				owner.mu.Unlock()
				request, _ := http.NewRequest(http.MethodPost, server.URL+"/setup/terminal/answer?id="+url.QueryEscape(ask["id"].(string))+
					"&code="+url.QueryEscape(ask["code"].(string))+"&answer="+answer, nil)
				request.Host = "127.0.0.1"
				request.Header.Set(setupHeader, "1")
				request.AddCookie(&http.Cookie{Name: setupCookie(), Value: window})
				if response, err := server.Client().Do(request); err == nil {
					response.Body.Close()
				}
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	t.Cleanup(func() { close(stop); <-done })
	return owner
}

// cliEnv is the environment a command in a terminal has, aimed at server: vars, and the support folder.
func cliEnv(server *httptest.Server, vars map[string]string) gm.Env {
	all := map[string]string{"SMOLCORP_PORT": server.URL[strings.LastIndex(server.URL, ":")+1:]}
	for name, value := range vars {
		all[name] = value
	}
	return gm.Env{Getenv: func(name string) string { return all[name] }, SupportDir: func() (string, error) { return supportDir, nil }}
}

// statusOf is the HTTP status in a gm client's error, or 0.
func statusOf(err error) int {
	var e *gm.Error
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

// On Windows with its own window, a General Manager started by hand logs in as on a Mac (the owner's ruling, 18 Nov):
// "Allow a General Manager started by hand" is the window's to turn on (a browser's Setup is refused), `smolcorp gm
// login`'s ask is refused until it is, then allowed in the window. Its token works for the GM's reads and actions (team,
// hire, fire: refused for what they're asked, never for who asks), the token before stops working, and that terminal's
// `smolcorp status` and `desks` work too, each with a tool's key allowed in the window.
func TestOnWindowsAGMStartedByHandLogsInThroughItsOwnWindow(t *testing.T) {
	d, _, window := onWindowsWithItsWindow(t)
	managerOn(d)
	d.mu.Lock()
	d.config.HiringEnabled = true // hiring and firing on, as for a GM at work: refused only for what is asked
	d.mu.Unlock()
	shortAskWait(t)
	if code, said := askFromTerminal(t, d, askGM, `C:\work\gm`); code != http.StatusForbidden || len(d.asks.waiting()) != 0 {
		t.Errorf("a GM's login with the setting off: %d %v", code, said)
	}
	if words := gmLoginOff(false, true); !strings.Contains(words, "Settings window (the tray icon, then Open Setup…)") {
		t.Errorf("the GM's login is told %q", words)
	}
	if got := withSession(t, d, browserSession(d), "POST", "/setup/gm/by-hand?enabled=1", ""); got.Code != http.StatusForbidden ||
		strings.TrimSpace(got.Body.String()) != windowOnlyChange {
		t.Errorf("a browser turned on a GM started by hand: %d %q", got.Code, got.Body)
	}
	if got := withSession(t, d, window, "POST", "/setup/gm/by-hand?enabled=1", ""); got.Code != http.StatusOK {
		t.Fatalf("the window turns it on: %d %q", got.Code, got.Body)
	}
	if manager, _ := setupStateOf(t, withSession(t, d, window, "GET", "/setup/state", ""))["manager"].(map[string]any); manager["gmByHand"] != true {
		t.Errorf("the state's manager: %v", manager)
	}
	before := tokenNow(t, d)

	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	owner := ownerAtTheWindow(t, d, server, window, "allow")
	var said strings.Builder
	token, err := gm.AskForKey(cliEnv(server, nil), gm.AskGM, `C:\work\gm`, &said)
	if err != nil || token == "" {
		t.Fatalf("gm login's ask: %v\n%s", err, said.String())
	}
	t.Logf("gm login printed: %q", said.String())
	if !strings.Contains(said.String(), "Smolcorp's Settings window (the tray icon, then Open Setup…)") || !strings.Contains(said.String(), "Check that it shows this code") {
		t.Errorf("gm login printed %q", said.String())
	}
	if asks := owner.asks(); len(asks) == 0 || asks[0]["kind"] != askGM || asks[0]["program"] != aWindowsAsker.program {
		t.Errorf("the window was shown %v", asks)
	}
	if !worksAsGM(d, token) || worksAsGM(d, before) || tokenNow(t, d) != token {
		t.Errorf("the GM's token: works %v, the one before works %v", worksAsGM(d, token), worksAsGM(d, before))
	}

	// The GM, started with that token in its environment: its reads and actions are let in.
	c, err := gm.NewClient(cliEnv(server, map[string]string{"SMOLCORP_GM_TOKEN": token}))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := gm.Team(c); err != nil {
		t.Errorf("gm team: %v", err)
	}
	_, hireErr := gm.Hire(c, gm.HireOptions{Role: "fake-no-such-role", Floor: -1})
	_, fireErr := gm.Fire(c, "fake-no-such-hire", false, false)
	for name, status := range map[string]int{"gm hire": statusOf(hireErr), "gm fire": statusOf(fireErr)} {
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			t.Errorf("%s with the GM's token: %d, refused for who asks", name, status)
		}
	}
	t.Logf("gm hire of no role: %d %v; gm fire of nobody: %d %v", statusOf(hireErr), hireErr, statusOf(fireErr), fireErr)

	// That terminal's status and desks (the GM's token in its environment, never sent): each asks for a tool's key.
	for _, c := range [][]string{{"status"}, {"desks"}} {
		var out, errOut strings.Builder
		if code := gm.RunPeopleIn(cliEnv(server, map[string]string{"SMOLCORP_GM_TOKEN": token}), c[0], nil, &out, &errOut); code != 0 || out.String() == "" {
			t.Errorf("smolcorp %s from the GM's terminal: exit %d %q %q", c[0], code, out.String(), errOut.String())
		}
	}
	if given := toolKeysGiven(d); given != 2 {
		t.Errorf("%d tool keys given, want status's and desks'", given)
	}
}

// Linux, and a Windows without WebView2, are unchanged: no window of Smolcorp's own, so no terminal's ask is taken, and
// Setup and the terminal say why. Linux's words say there's no window of Smolcorp's own yet, as the release notes do,
// and never promise the phone (RT126FIXREVIEW-WINTOOLASKR1 O2), nor does docs/demo.md. A GM started by hand isn't
// offered there.
func TestWithoutItsOwnWindowATerminalsAskIsRefusedAsBefore(t *testing.T) {
	d := newTestDeck(t)
	withoutWebView2(t)
	standInAsker(t, aWindowsAsker)
	if code, _ := askFromTerminal(t, d, askTool, "/work"); code != http.StatusForbidden || len(d.asks.waiting()) != 0 {
		t.Errorf("a tool's ask without the window: %d", code)
	}
	if code, said := askFromTerminal(t, d, askGM, "/work"); code != http.StatusForbidden {
		t.Errorf("a GM's login without the window: %d %v", code, said)
	}
	if got := serve(t, d, call{method: "POST", target: "/setup/gm/by-hand?enabled=1", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}}); got.Code != http.StatusNotFound {
		t.Errorf("by hand without the window: %d", got.Code)
	}
	state := setupStateOf(t, serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer}))
	if state["terminalAsksOff"] != terminalAskOffHere() {
		t.Errorf("Setup says %q", state["terminalAsksOff"])
	}
	const linux = "On this computer, only a window of Smolcorp's own may let a command in a terminal into Smolcorp, and there's none " +
		"here yet. A setup page in a browser can't, as another program could be using it. So smolcorp demo, status and desks don't " +
		"run here: see your team and set things up in Setup (the tray icon, then Open Setup…)."
	if terminalAskOff("linux", true, false) != linux || terminalAskOff("linux", false, true) != linux {
		t.Errorf("Linux's words changed: %q", terminalAskOff("linux", true, false))
	}
	if words := terminalAskOff("linux", true, false); strings.Contains(words, "phone") {
		t.Errorf("Linux's words promise the phone, which never answers a terminal's ask: %q", words)
	}
	// The demo's page says the same (TAPASKFINAL's N2): Linux has no window to say yes in, and the phone isn't one.
	demo := repoText(t, "docs", "demo.md")
	run := demo[strings.Index(demo, "## Run it"):strings.Index(demo, "## What happens")]
	mustSay(t, "docs/demo.md", run, "On a Mac, and on Windows with the WebView2 Runtime.",
		"On Linux there's no window of Smolcorp's own to say yes in, so the demo says so and stops")
	if strings.Contains(run, "phone") {
		t.Errorf("docs/demo.md promises the phone, which never answers a terminal's ask: %q", run)
	}
	if words := gmLoginOff(false, false); !strings.Contains(words, "hire your General Manager from Setup") {
		t.Errorf("a GM's login without the window is told %q", words)
	}
}

// On Windows without its own window, the terminal and Setup say why the window isn't there, as it is
// (RT126FIXREVIEW-WINTOOLASK O6): no WebView2, the window broke though WebView2 is there, or smolcorp serve, which has
// no tray to name. No dangling "until then", and never the phone.
func TestOnWindowsTheTerminalIsToldWhyTheWindowIsntThere(t *testing.T) {
	noWebView2, broken, serve := terminalAskOff("windows", true, false), terminalAskOff("windows", true, true),
		terminalAskOff("windows", false, false)
	for name, words := range map[string]string{"no WebView2": noWebView2, "broken": broken, "serve": serve} {
		t.Logf("%s: %s", name, words)
		if !strings.Contains(words, "only Smolcorp's own Settings window may") || !strings.Contains(words, "browser can't") ||
			!strings.Contains(words, "Until the window can open, smolcorp demo, status and desks don't run here") ||
			strings.Contains(words, "Until then") || strings.Contains(words, "phone") {
			t.Errorf("%s: %q", name, words)
		}
	}
	if !strings.Contains(noWebView2, "needs the Microsoft Edge WebView2 Runtime") || !strings.Contains(noWebView2, "the tray icon") {
		t.Errorf("without WebView2: %q", noWebView2)
	}
	if strings.Contains(broken, "WebView2") || !strings.Contains(broken, "didn't open this time: quit Smolcorp and open it again") || !strings.Contains(broken, "the tray icon") {
		t.Errorf("WebView2 there, the window broken: %q", broken)
	}
	if strings.Contains(serve, "tray") || strings.Contains(serve, "WebView2") || !strings.Contains(serve, "smolcorp serve") {
		t.Errorf("under smolcorp serve: %q", serve)
	}
	// What this run says follows the app's flags.
	savedApp, savedBroken := settingsWindowApp.Load(), settingsWindowBroken.Load()
	t.Cleanup(func() { settingsWindowApp.Store(savedApp); settingsWindowBroken.Store(savedBroken) })
	settingsWindowApp.Store(true)
	settingsWindowBroken.Store(true)
	if got := terminalAskOffHere(); got != terminalAskOff(runtime.GOOS, true, true) {
		t.Errorf("this run says %q", got)
	}
}

// A terminal's ask while three wait is refused before anything is read of who asked (RT126FIXREVIEW-WINTOOLASK O3):
// a refused ask costs no look at Windows' tables.
func TestARefusedAskReadsNothingOfWhoAsked(t *testing.T) {
	d, _, _ := onWindowsWithItsWindow(t)
	var reads int
	var mu sync.Mutex
	saved := askingProgram
	askingProgram = func(*http.Request) askAsker {
		mu.Lock()
		reads++
		mu.Unlock()
		return aWindowsAsker
	}
	t.Cleanup(func() { askingProgram = saved })
	for i := range asksMost {
		if code, said := askFromTerminal(t, d, askTool, `C:\work`); code != http.StatusOK {
			t.Fatalf("ask %d: %d %v", i+1, code, said)
		}
	}
	mu.Lock()
	before := reads
	mu.Unlock()
	if before != asksMost {
		t.Fatalf("%d asks taken read the asker %d times", asksMost, before)
	}
	code, _ := askFromTerminal(t, d, askTool, `C:\work`)
	mu.Lock()
	after := reads
	mu.Unlock()
	if code != http.StatusTooManyRequests || after != before {
		t.Errorf("an ask past %d: %d, the asker read %d more time(s)", asksMost, code, after-before)
	}
}

// Which program asked, as the window shows it: Windows' program and what started it, each read as Windows says it.
// Off Windows nothing is read, and the ask shows its folder alone, as before. A process list's numbers given again,
// loops and gaps end the line of what started it.
func TestTheWindowSaysWhichProgramAskedAsWindowsSaysIt(t *testing.T) {
	list := map[uint32]listedProc{
		10: {name: "smolcorp.exe", created: 500, parent: 9},
		9:  {name: "pwsh.exe", created: 400, parent: 8},
		8:  {name: "claude.exe", created: 300, parent: 7},
		7:  {name: "WindowsTerminal.exe", created: 200, parent: 6},
		6:  {name: "explorer.exe", created: 100, parent: 5},
		5:  {name: "userinit.exe", created: 50, parent: 0},
		20: {name: "smolcorp.exe", created: 500, parent: 21},
		21: {name: "reused.exe", created: 600, parent: 6}, // started after its "child": a number given again
		30: {name: "a.exe", created: 500, parent: 31},
		31: {name: "b.exe", created: 400, parent: 30}, // a loop
	}
	for _, c := range []struct {
		pid  uint32
		want []string
	}{
		{10, []string{"pwsh.exe", "claude.exe", "WindowsTerminal.exe", "explorer.exe"}}, // at most askFromMost
		{20, nil},
		{30, []string{"b.exe"}},
		{99, nil},
	} {
		if got := startedFrom(c.pid, list); !slices.Equal(got, c.want) {
			t.Errorf("startedFrom(%d) = %v, want %v", c.pid, got, c.want)
		}
	}
	// What the page is given: nothing where nothing was read; the program, or "" and a plain line, where it was.
	plain := map[string]any{}
	askAsker{}.shown(plain)
	unread := map[string]any{}
	askAsker{tried: true}.shown(unread)
	odd := map[string]any{}
	askAsker{tried: true, program: "C:\\x\x1b[2J.exe", from: []string{"p\nwsh.exe"}}.shown(odd)
	if len(plain) != 0 || unread["program"] != "" || mustJSONList(t, unread["startedFrom"]) == nil || len(anyStrings(mustJSONList(t, unread["startedFrom"]))) != 0 ||
		odd["program"] != `C:\x[2J.exe` || !slices.Equal(anyStrings(mustJSONList(t, odd["startedFrom"])), []string{"pwsh.exe"}) {
		t.Errorf("shown: %v %v %v", plain, unread, odd)
	}
	// A request with no connection behind it names no program.
	if got := askerRead(httptest.NewRequest("POST", terminalAskPath, nil), func(string, string) (string, []string) {
		t.Error("read with no connection")
		return "x", nil
	}); !got.tried || got.program != "" {
		t.Errorf("no connection: %+v", got)
	}
	// Off Windows (a Mac, Linux), nothing is read: the window shows the folder alone, as before.
	if runtime.GOOS != "windows" {
		if got := askingProgramHere(httptest.NewRequest("POST", terminalAskPath, nil)); got.tried {
			t.Errorf("off Windows, the asker was read: %+v", got)
		}
		d := newMacTestDeck(t)
		askFromTerminal(t, d, askTool, "/work")
		if shown := shownAsks(t, d); len(shown) != 1 || shown[0]["program"] != nil || shown[0]["startedFrom"] != nil {
			t.Errorf("on a Mac the ask shows %v", shown)
		}
	}
}

// mustJSONList is a []string as the page gets it: through JSON.
func mustJSONList(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back any
	_ = json.Unmarshal(data, &back)
	return back
}
