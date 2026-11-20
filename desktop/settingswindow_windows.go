//go:build windows

package desktop

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

// Smolcorp's own Settings window on Windows (settingswindow.go): a second WebView2 window, made as the office's is
// (window_windows.go), on its own locked OS thread, on Setup at 127.0.0.1.

// settingsWebViewHere: WebView2 is installed and made a window before, if it was asked to. Never in a test binary,
// so no test opens a real window by opening Setup: a test that wants this window says so (settingsWebView).
func settingsWebViewHere() bool {
	if testing.Testing() || os.Getenv("SMOLCORP_TEST_NO_WEBVIEW2") != "" || settingsWindowBroken.Load() {
		return false
	}
	return webview2Installed()
}

// startSettingsWindowHere opens the window on its own thread; it returns at once.
func startSettingsWindowHere(d *Deck, s settingsRun) error {
	run := d.settingsWindowRun(s)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer settingsWindow.closed()
		runOfficeWindow(run)
	}()
	return nil
}

// settingsWindowRun is everything the window is opened with.
//   - Its WebView2 keeps its storage in a folder of its own (settingsWebViewFolder), never the office window's, so the
//     cookie of its session is in a jar of its own, as the Mac's Settings window has a store of its own
//     (RT126FIXREVIEW-OWNSETTINGS O2).
//   - It stays on top when the office does (the tray's Always on Top), so the office's ⚙︎ never opens it
//     underneath, and it is put back on top as it's brought forward (O3).
func (d *Deck) settingsWindowRun(s settingsRun) windowRun {
	d.mu.Lock()
	logo, onTop := d.config.Logo, d.config.AlwaysOnTop
	d.mu.Unlock()
	icon, _ := trayIcons.ReadFile("tray-" + logo + ".png")
	return windowRun{
		title:   "Smolcorp Settings",
		url:     s.url,
		script:  s.script,
		onTop:   onTop,
		icon:    icon,
		dataDir: filepath.Join(supportDir, settingsWebViewFolder),
		created: func(hwnd uintptr, view webview2.WebView) {
			settingsWindow.opened(func(then string) {
				d.mu.Lock()
				onTop := d.config.AlwaysOnTop
				d.mu.Unlock()
				applyTopmost(hwnd, onTop)
				raiseWindow(hwnd)
				if then != "" {
					view.Dispatch(func() { view.Eval(settingsWindowReload(then)) })
				}
			})
		},
		// WebView2 is there but made no window: say so, and open Setup in the browser, where it changes things
		// from now on (settingsWindowBroken). The words are the window's own, in plain English, never joined to
		// the office's reason (O6).
		missing: func(string) {
			settingsWindowBroken.Store(true)
			log.Printf("smolcorp: Settings window unavailable: WebView2 made no window")
			title, _ := windows.UTF16PtrFromString("Smolcorp")
			body, _ := windows.UTF16PtrFromString(settingsWindowMissingWords)
			_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
			_ = d.openSetupInBrowser(s.then)
		},
	}
}
