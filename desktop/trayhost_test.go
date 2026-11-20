package desktop

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// browserOpens swaps openURL for the test, and gives what it was asked to open.
func browserOpens(t *testing.T) *[]string {
	t.Helper()
	saved := openURL
	t.Cleanup(func() { openURL = saved })
	var links []string
	openURL = func(url string) error { links = append(links, url); return nil }
	return &links
}

func TestWithoutATraySetupOpensAndTheWaysInArePrinted(t *testing.T) {
	d := newTestDeck(t)
	links := browserOpens(t)
	var out bytes.Buffer
	withoutTray(d, &out, false)
	printed := out.String()
	for _, want := range []string{"no tray", "Open Setup…", "/setup#o="} {
		if !strings.Contains(printed, want) {
			t.Errorf("printed %q, which doesn't say %q", printed, want)
		}
	}
	if strings.Contains(printed, d.access.SetupKey) || strings.Contains(printed, "#k=") {
		t.Errorf("printed %q, which carries the setup key", printed)
	}
	if len(*links) != 1 || !strings.Contains((*links)[0], "/setup#o=") || strings.Contains((*links)[0], d.access.SetupKey) {
		t.Errorf("opened %v, want setup through a link that works once, without the key", *links)
	}
	// Opened already, as on the first run: not a second time.
	*links = nil
	withoutTray(d, &out, true)
	if len(*links) != 0 {
		t.Errorf("setup, open already, was opened again: %v", *links)
	}
}

func TestADesktopWithATrayIsLeftToIt(t *testing.T) {
	d := newTestDeck(t)
	links := browserOpens(t)
	saved, savedWait := hasTrayHost, trayHostWait
	t.Cleanup(func() { hasTrayHost, trayHostWait = saved, savedWait })
	// The panel comes up a moment after Smolcorp, as at login.
	calls := 0
	hasTrayHost = func() bool { calls++; return calls > 1 }
	trayHostWait = 5 * time.Second
	var out bytes.Buffer
	lookForTrayHost(d, &out, false)
	if out.Len() > 0 || len(*links) > 0 {
		t.Errorf("with a tray: printed %q, opened %v; want neither", out.String(), *links)
	}
}

// A program that only holds the setup key can't get a link into setup, so
// the command makes none: it says where to get one.
func TestSetupOpenFromATerminalMakesNoLink(t *testing.T) {
	d := newTestDeck(t)
	links := browserOpens(t)
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	writeFile(t, setupKeyFile, d.access.SetupKey+"\n")
	defer func(was string) { setupSystem = was }(setupSystem)
	setupSystem = "linux"
	var out bytes.Buffer
	setupClient{base: server.URL, http: server.Client()}.openSetup(&out)
	if len(*links) != 0 {
		t.Errorf("opened %v, want nothing: the command makes no link", *links)
	}
	for _, want := range []string{"Open Setup…", "tray", "as it starts", "With no tray, use the link smolcorp prints as it starts; for a new one, run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("printed %q, which doesn't say %q", out.String(), want)
		}
	}
	if strings.Contains(out.String(), "#o=") || strings.Contains(out.String(), "#k=") || strings.Contains(out.String(), d.access.SetupKey) {
		t.Errorf("printed %q, which carries a link or the key", out.String())
	}
	// The key opens no way to a link, by the old endpoint or any chain from it.
	for _, tool := range []bool{true, false} {
		got := serve(t, d, call{method: "POST", target: "/setup/link", host: "localhost:7710", from: thisComputer, asTool: tool,
			withoutSetupKey: !tool, headers: map[string]string{setupHeader: "1"}})
		if got.Code != http.StatusNotFound && got.Code != http.StatusUnauthorized {
			t.Errorf("POST /setup/link (tool %v): %d, want 404 or 401", tool, got.Code)
		}
		if strings.Contains(got.Body.String(), "#o=") || strings.Contains(got.Body.String(), "link") {
			t.Errorf("POST /setup/link (tool %v) gave %q, which looks like a link", tool, got.Body)
		}
	}
	if code := d.access.ActiveCode(); code != "" {
		t.Errorf("a pairing code exists: %q", code)
	}
}

// noTrayHost swaps hasTrayHost for the test: always the answer given, and no waiting.
func noTrayHost(t *testing.T, present bool) {
	t.Helper()
	saved, savedWait := hasTrayHost, trayHostWait
	t.Cleanup(func() { hasTrayHost, trayHostWait = saved, savedWait })
	hasTrayHost = func() bool { return present }
	trayHostWait = 0
}

// undoStopOnSignal gives SIGINT and SIGTERM back their default handling when
// the test ends: the fallback calls stopPortableOnSignal, which can't be
// undone itself, and its handler would exit the whole test binary. What is
// left is a goroutine parked on a channel nothing sends to, which does nothing.
func undoStopOnSignal(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { signal.Reset(os.Interrupt, syscall.SIGTERM) })
}

func TestATrayAlreadyThereIsLeftAloneAtOnce(t *testing.T) {
	d := newTestDeck(t)
	links := browserOpens(t)
	noTrayHost(t, true)
	var out bytes.Buffer
	lookForTrayHost(d, &out, false)
	if out.Len() > 0 || len(*links) > 0 {
		t.Errorf("with a tray: printed %q, opened %v; want neither", out.String(), *links)
	}
}

func TestWithNoTrayHostTheWaitEndsInTheFallback(t *testing.T) {
	undoStopOnSignal(t)
	d := newTestDeck(t)
	links := browserOpens(t)
	noTrayHost(t, false)
	var out bytes.Buffer
	lookForTrayHost(d, &out, false)
	if !strings.Contains(out.String(), noTrayMessage) || !strings.Contains(out.String(), "/setup#o=") {
		t.Errorf("printed %q, want the no-tray message and the setup link", out.String())
	}
	if len(*links) != 1 {
		t.Errorf("opened %v, want setup once", *links)
	}
	// First run: setup is open already, so it is printed but not opened again.
	*links = nil
	out.Reset()
	lookForTrayHost(d, &out, true)
	if !strings.Contains(out.String(), noTrayMessage) || len(*links) != 0 {
		t.Errorf("first run: printed %q, opened %v; want the message and no second opening", out.String(), *links)
	}
}

// The office's button on setup says whether the browser opened.
func TestSetupOfficeAnswersWhetherTheBrowserOpened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on Windows the office opens in a window of its own (window_windows.go), never the browser")
	}
	d := newTestDeck(t)
	opener := func(err error) *[]string {
		saved := openURL
		t.Cleanup(func() { openURL = saved })
		var links []string
		openURL = func(url string) error { links = append(links, url); return err }
		return &links
	}
	office := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/office", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
	}
	links := opener(nil)
	if got := office().Code; got != http.StatusNoContent {
		t.Errorf("the browser opened: %d, want %d", got, http.StatusNoContent)
	}
	if len(*links) != 1 || !strings.Contains((*links)[0], "/office?o=") {
		t.Errorf("opened %v, want the office through a link that works once", *links)
	}
	opener(errors.New("no xdg-open"))
	got := office()
	if got.Code != http.StatusBadGateway || !strings.Contains(got.Body.String(), "no xdg-open") {
		t.Errorf("the browser failed: %d %q, want %d saying why", got.Code, got.Body.String(), http.StatusBadGateway)
	}
	if strings.Contains(got.Body.String(), "/office") {
		t.Errorf("the failure answer points at a link: %q", got.Body.String())
	}
}

// The page's button reads the answer, and says it, pointing at the tray, or this computer's /office and Pair a phone, for a failure.
func TestSetupPageOfficeButtonReadsTheAnswer(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	i := strings.Index(page, "$('start-office').onclick")
	if i < 0 {
		t.Fatal("the setup page has no handler for the office button")
	}
	handler := page[i:]
	if j := strings.Index(handler, "$('start-gm')"); j > 0 {
		handler = handler[:j]
	}
	// A bare /office link that promises to let the user straight in is a dead end (the setup cookie is only for /setup):
	// the address is plain text, with the pairing code, never a link or a redirect.
	for _, bad := range []string{"<a ", "href", "location.href", "location.assign", "window.open", "straight in"} {
		if strings.Contains(handler, bad) {
			t.Errorf("the office button's handler makes a dead /office link (%q): %s", bad, handler)
		}
	}
	// The browser alone isn't so on Windows (a window of its own), but Windows without WebView2 falls back
	// to the browser, which the page can't tell: off the Mac, the words name both.
	if strings.Contains(strings.ReplaceAll(handler, "in its own window, or in your browser", ""), "in your browser") {
		t.Errorf("the office button's handler says only the browser, which isn't so on Windows: %s", handler)
	}
	if !strings.Contains(handler, "'The office is open: in its own window, or in your browser.'") {
		t.Errorf("off the Mac, the office button's handler doesn't say the window or the browser: %s", handler)
	}
	for _, want := range []string{"res.ok", "startSaid(", "Show Office", "or open ${location.origin}/office in a browser on this computer and enter the code from Pair a phone."} {
		if !strings.Contains(handler, want) {
			t.Errorf("the office button's handler doesn't use %q: %s", want, handler)
		}
	}
}

// A second start only raises the running Smolcorp and makes no setup link, and no command makes one,
// so the wording names the tray, or quitting and starting again, and never a second start or a command.
func TestTheSetupLinkWordingSaysHowToGetANewLink(t *testing.T) {
	isolate(t)
	d := newTestDeck(t)
	var out bytes.Buffer
	printSetupLink(&out, d)
	if want := "It works once, for " + demoWaitWords(setupTokenLifetime) + ". For a new one, choose Open Setup… from the tray icon; with no tray, run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again.\n"; !strings.Contains(out.String(), want) {
		t.Errorf("printed %q, want it to contain %q", out.String(), want)
	}
	lines := strings.Split(noTrayMessage, "\n")
	if len(lines) < 4 ||
		lines[1] != "Smolcorp is running, and its setup opens in your browser. For a new setup link, run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again." ||
		lines[2] != "To open the office, start Smolcorp again from your apps, or run: smolcorp" {
		t.Errorf("no-tray message is %q", noTrayMessage)
	}
	for _, goos := range []string{"linux", "windows"} {
		if strings.Contains(setupAdvice(goos), "setup --open") {
			t.Errorf("%s advice %q names a command, and no command makes a setup link", goos, setupAdvice(goos))
		}
	}
}
