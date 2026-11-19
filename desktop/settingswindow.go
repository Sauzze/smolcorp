package desktop

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Smolcorp's own Settings window off the Mac (1.26, FXOWNSETTINGS; the owner's Strict, SANDBOX-PLAN §9 Q8: "only
// Smolcorp's window (Mac, Windows) and your phone can"). On Windows it is a second WebView2 window, beside the
// office's (settingswindow_windows.go), on Setup. It opens Setup as the Mac's Settings window does, with no link:
// a token that works once is handed to the page in this process (settingsWindowScript), never in an address, on a
// command line or in a file, and the page swaps it for a window session (server.go's /setup/session,
// windowSetupHeader). So, as on the Mac, a browser's Setup there only looks (routes.go's windowOnlyRefusal), and
// the tray's Open Setup…, the office's ⚙︎ and `smolcorp setup --open` open this window, or bring it forward.
// Where there's no webview (Linux, a Windows without WebView2), Setup opens in the browser as before: a link while
// no phone is paired, then the phone's tap (setupask.go).
//
// The window is also where a terminal's ask is answered off the Mac (FXWINTOOLASK): its session is a window session
// (Access.IsWindowSession), as the Mac window's is, so terminalask.go's isWindowPage check holds for it, and a GM
// started by hand may ask there too (gmbyhand.go).

// settingsWebView is whether Smolcorp's own Settings window can open off the Mac, from this process: the app, with
// its tray (settingsWindowApp), on Windows with WebView2 (settingswindow_windows.go), nowhere else. A variable, for
// the tests.
var settingsWebView = settingsWebViewInTheApp

func settingsWebViewInTheApp() bool { return settingsWindowApp.Load() && settingsWebViewInstalled() }

// settingsWindowApp is set as the app starts (RunApp), the one with a tray to open Setup from and a window to show it
// in. `smolcorp serve` (RunServer) never sets it: it has no tray and no window, so Setup there stays the browser's,
// opened with the link it prints, and changes things as before, even where WebView2 is installed
// (RT126FIXREVIEW-OWNSETTINGS N1).
var settingsWindowApp atomic.Bool

// settingsWebViewInstalled is whether this computer has the webview the window is made with: Windows with WebView2
// (settingswindow_windows.go), nowhere else. A variable, for the tests.
var settingsWebViewInstalled = settingsWebViewHere

// settingsWindowBroken is set once WebView2 was found but made no window: from then on, until Smolcorp starts
// again, Setup opens in the browser, and the browser's Setup changes things there, rather than being locked out of
// a window that won't come.
var settingsWindowBroken atomic.Bool

// settingsWebViewFolder is where, in the support folder, the Settings window's WebView2 keeps its storage: a folder
// of its own, never the office window's ("WebView2"), so the window's session is never in the office's cookie jar.
const settingsWebViewFolder = "WebView2 Settings"

// settingsWindowMissingWords is what the owner is told when WebView2 made no Settings window after all: the window's
// own words, whole, never joined to another message.
const settingsWindowMissingWords = "Smolcorp's Settings window couldn't open: the Microsoft Edge WebView2 Runtime isn't " +
	"installed on this computer, or didn't respond. Setup opens in your browser instead.\n\n" +
	"For the window, install the Microsoft Edge WebView2 Runtime, free from Microsoft, then choose Open Setup… again."

// ownSettingsWindow is whether Setup has Smolcorp's own Settings window on this computer: under the Mac app, its
// Settings window; on Windows with WebView2, the one here. Where it has, a browser's Setup only looks.
func (d *Deck) ownSettingsWindow() bool {
	return d.macHost() != nil || settingsWebView()
}

// ownSettingsWindowHeld is ownSettingsWindow for a caller that holds d.mu.
func (d *Deck) ownSettingsWindowHeld() bool {
	return d.mac.host != nil || settingsWebView()
}

// settingsRun is what the window is opened with: Setup's address, and the script that hands it its token.
type settingsRun struct {
	url    string // Setup on 127.0.0.1, with what it opens on after # (pair, unpair, viewer, guide): never a token
	script string // gives the page its token, in this process (settingsWindowScript)
	then   string
}

// startSettingsWindow opens the window for run, and calls opened once it's there, with how to bring it forward,
// and closed once it's gone (settingsWindow's). It says if it couldn't open at all. The platform's
// (settingswindow_windows.go); a variable, so a test opens no window.
var startSettingsWindow = startSettingsWindowHere

// settingsWindow is the one Settings window there can be.
var settingsWindow oneWindow

// oneWindow is a window there's only ever one of: a second ask brings the one there forward.
type oneWindow struct {
	mu      sync.Mutex
	busy    bool              // opening, open, or its message is on screen
	forward func(then string) // brings the open one forward, on then; nil until it's there
}

// claim is whether this ask opens the window. If one is opening or open, it is brought forward instead, on then.
func (o *oneWindow) claim(then string) bool {
	o.mu.Lock()
	if !o.busy {
		o.busy = true
		o.mu.Unlock()
		return true
	}
	forward := o.forward
	o.mu.Unlock()
	if forward != nil {
		forward(then)
	}
	return false
}

// opened says the window is there, and how to bring it forward.
func (o *oneWindow) opened(forward func(then string)) {
	o.mu.Lock()
	o.forward = forward
	o.mu.Unlock()
}

// closed says it's gone: the next ask opens a new one.
func (o *oneWindow) closed() {
	o.mu.Lock()
	o.busy, o.forward = false, nil
	o.mu.Unlock()
}

// showSettingsWindow opens Smolcorp's own Settings window on then, or brings the open one forward: never a second.
func (d *Deck) showSettingsWindow(then string) error {
	if !settingsWindow.claim(then) {
		return nil
	}
	if err := startSettingsWindow(d, d.newSettingsRun(then)); err != nil {
		settingsWindow.closed()
		return err
	}
	return nil
}

// newSettingsRun makes the window's address and script, with a new token for it.
func (d *Deck) newSettingsRun(then string) settingsRun {
	d.mu.Lock()
	token := d.access.NewWindowSetupToken()
	d.mu.Unlock()
	url := setupBase() + "/setup"
	if then != "" {
		url += "#" + then
	}
	return settingsRun{url: url, script: settingsWindowScript(setupBase(), token), then: then}
}

// settingsWindowScript is the script the Settings window runs before each page, handed to WebView2 in this process
// (kept in memory, never in its profile), as the office window's key is (windowKeyScript): it gives Setup, on this
// computer's own 127.0.0.1 alone, the token setup.html swaps for the window's session. "" with no token.
func settingsWindowScript(base, token string) string {
	if token == "" {
		return ""
	}
	origin, _ := json.Marshal(base)
	value, _ := json.Marshal(token)
	return fmt.Sprintf("if (location.origin === %s && location.pathname === '/setup') "+
		"Object.defineProperty(window, 'smolcorpWindowSetup', { value: %s, writable: false, enumerable: false });", origin, value)
}

// settingsWindowReload is what an open window runs to show then: its address's # changed, and the page loaded
// again, so it does what the menu asked as it starts (setup.html's start). Its session is kept.
func settingsWindowReload(then string) string {
	hash, _ := json.Marshal(then)
	return "location.hash = " + string(hash) + "; location.reload();"
}

// NewWindowSetupToken is a token for Smolcorp's own Settings window off the Mac, handed in-process alone: it opens
// one window session, once, for officeTokenLifetime.
func (a *Access) NewWindowSetupToken() string {
	return newToken(&a.windowSetupTokens, officeTokenLifetime)
}

// redeemWindowSetupToken is Access.RedeemSetupToken for the window's tokens, under the deck's lock.
func (d *Deck) redeemWindowSetupToken(candidate string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return redeemToken(d.access.windowSetupTokens, candidate)
}

// What a browser's Setup is told where Smolcorp has its own Settings window, and where to find that window.
func ownWindowWhere(mac bool) string {
	if mac {
		return "menu bar icon, then Settings…"
	}
	return "the tray icon, then Open Setup…"
}
