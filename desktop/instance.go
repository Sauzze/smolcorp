package desktop

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// One Smolcorp per session. A second one would find the port taken, and used
// to stay alive with no icon and no server. Now it asks the first one to show
// the office, says so, and leaves.
//
// The lock itself is the platform's (instance_windows.go: a named mutex of the
// Windows session; instance_unix.go: a lock on a file next to the keys).

// raisePath is what the second instance asks the first, on this computer's own
// port, with the hook's key, as /hook is asked: the first opens the office
// window, or brings the one that is open to the front.
const raisePath = "/raise"

// showOffice is what /raise does; a test swaps it so no window opens. It says
// if the browser could not be started (under the Mac app, if the app's window
// could not be opened), and the second instance is told. Under
// the Mac app, the office in the app's own window, as the menu's Show Office
// opens it, never the browser, whose office key ends up in its history and
// reaches what the window's does (RT126SWEEP3 P4 F3).
var showOffice = func(d *Deck) error {
	if h := d.macHost(); h != nil {
		return hostShowOffice(h)
	}
	return d.openOffice()
}

// showSettings is /raise?show=settings, `smolcorp setup --open` on Windows: Smolcorp's own Settings window, or the
// one open brought forward ("" said), as the tray's Open Setup… does. Under the Mac app, its own Settings window, as
// the office's ⚙︎ opens it (act.go), never the browser: Setup in a browser only looks on a Mac
// (RT126FIXREVIEW-OWNSETTINGS N2). With no window of its own here, Setup opens in the browser as the tray's would,
// and what it says is how. A variable, for the tests.
var showSettings = func(d *Deck) (instead string, err error) {
	if h := d.macHost(); h != nil {
		return "", hostShowSettings(h)
	}
	if settingsWebView() {
		return "", d.showSettingsWindow("")
	}
	instead = settingsInBrowserWords(runtime.GOOS, settingsWindowApp.Load(), d.setupLinkGone())
	return instead, d.openSetupInBrowser("")
}

// settingsInBrowserWords says Setup opened in the browser, not in a window of Smolcorp's own, and why, and how it lets
// you in: a link that works once, or, once a phone is paired, a yes on the phone. app is whether this is the app, with
// its tray (settingsWindowApp), or `smolcorp serve`, which has no window to open on any computer.
func settingsInBrowserWords(goos string, app, tap bool) string {
	words := "Smolcorp has no Settings window of its own on this computer, so Setup opened in your browser instead"
	switch {
	case goos == "windows" && !app:
		words = "Smolcorp is running as smolcorp serve, with no tray and no windows, so Setup opened in your browser instead"
	case goos == "windows":
		words = "Smolcorp's own Settings window needs the Microsoft Edge WebView2 Runtime, which this computer doesn't have, so Setup opened in your browser instead"
	}
	if tap {
		return words + ". Your phone is paired, so say yes on it when Setup asks."
	}
	return words + ", with a link that works once."
}

// instanceRunning is the message the second instance prints (and, with no
// console to print on, shows).
const instanceRunning = "Smolcorp is already running: look for its icon by the clock, or among the hidden icons. This second copy is closing."

// instanceRaised is what it says when the first one has just been asked to show
// the office.
const instanceRaised = "Smolcorp is already running: its office is coming to the front."

// instanceNotRaised is what it says when the first one was asked but no browser
// could open the office (see notRaisedMessage).
const instanceNotRaised = "Smolcorp is already running, but no browser could open its office"

// What /raise answers when the office couldn't be shown, before why: off the Mac app the browser didn't start; under
// it, the app's own office window didn't open, and no browser was tried (RT126SWEEP3FIXP4 O2).
const (
	raiseBrowserFailed = "The browser could not be started: "
	raiseWindowFailed  = "The office window could not be opened: "
)

// raiseFailedWords is which of them /raise answers: under the Mac app (mac), the window's.
func raiseFailedWords(mac bool) string {
	if mac {
		return raiseWindowFailed
	}
	return raiseBrowserFailed
}

// instanceWindowNotRaised is what the second one says when the first, under the Mac app, couldn't open its office
// window: no browser was tried, so there's no address to open by hand and nothing to pair.
const instanceWindowNotRaised = "Smolcorp is already running, but its office window didn't open"

// windowNotRaisedMessage says why the office window didn't open, and where else it opens from.
func windowNotRaisedMessage(failure string) string {
	return instanceWindowNotRaised + " (" + failure + ").\nTo open it, choose Show Office from Smolcorp's menu bar icon."
}

// notRaisedMessage says why no browser opened the office and gives its address
// on a line of its own, so it copies cleanly. The address carries no token, so
// a browser that isn't paired yet is asked to pair first.
func notRaisedMessage(failure string) string {
	return instanceNotRaised + " (" + failure + ").\n" + officeByHand(localBase())
}

// officeByHand is the office's plain address on a line of its own, for a
// computer with no browser to open it: no token or key in it.
func officeByHand(base string) string {
	return "To open it by hand, go to: " + base + "/office\nIt may ask you to pair this browser first."
}

// stopHint says, on Linux, how to stop the running copy (see quit.go).
var stopHint = func() string {
	if runtime.GOOS == "linux" {
		return " To stop the running copy, run: smolcorp quit"
	}
	return ""
}()

// handOverToRunning tells the user, and asks the running Smolcorp to show the
// office. Whatever happens it returns the exit code the second instance ends
// with: 0 when the office is coming, 1 when no browser could open it (the
// message and the plain address are printed all the same, and Smolcorp is
// running either way) or, under the Mac app, its office window didn't open.
func handOverToRunning(quiet bool) int {
	if answered, failure, window := askRunningToShowOffice(Port); answered {
		if failure == "" {
			fmt.Fprintln(os.Stderr, instanceRaised+stopHint)
			return 0
		}
		message := notRaisedMessage(failure)
		if window {
			message = windowNotRaisedMessage(failure)
		}
		fmt.Fprintln(os.Stderr, message+stopHint)
		if !quiet {
			notifyUser(message)
		}
		return 1
	}
	fmt.Fprintln(os.Stderr, instanceRunning+stopHint)
	if !quiet {
		notifyUser(instanceRunning)
	}
	return 0
}

// askRunningToShowOffice reports whether the running Smolcorp answered, and, if
// it couldn't open the office, why, and whether it was the Mac app's office
// window that didn't open (window) rather than a browser. A Smolcorp from before
// this route existed answers 404 or 401, which is no answer.
func askRunningToShowOffice(port int) (answered bool, failure string, window bool) {
	key, err := os.ReadFile(hookKeyFile)
	if err != nil {
		return false, "", false
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, raisePath), nil)
	if err != nil {
		return false, "", false
	}
	request.Header.Set(hookKeyHeader, strings.TrimSpace(string(key)))
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false, "", false
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNoContent:
		return true, "", false
	case http.StatusBadGateway:
		text, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		said := strings.TrimSpace(string(text))
		if failure, window = strings.CutPrefix(said, raiseWindowFailed); window {
			failure = strings.TrimSpace(failure)
			if failure == "" {
				failure = "the office window could not be opened"
			}
			return true, failure, true
		}
		failure = strings.TrimSpace(strings.TrimPrefix(said, raiseBrowserFailed))
		if failure == "" {
			failure = "the browser could not be started"
		}
		return true, failure, false
	}
	return false, "", false
}
