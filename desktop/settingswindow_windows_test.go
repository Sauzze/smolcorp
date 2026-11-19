//go:build windows

package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Windows' own rows for Smolcorp's Settings window (settingswindow_windows.go), for the Windows CI job. No row
// opens a real window unless asked by hand (TestLiveSettingsWindow), as TestLiveWindow's office.

// In a test binary the real window never opens by opening Setup: a test that wants it says so. The window opener is
// WebView2's, and with WebView2 said missing (or broken once) Setup opens in the browser, as before.
func TestOnWindowsNoTestOpensTheRealSettingsWindow(t *testing.T) {
	if settingsWebViewHere() {
		t.Fatal("settingsWebViewHere is true in a test binary: opening Setup in any test would open a real window")
	}
	if reflect.ValueOf(startSettingsWindow).Pointer() != reflect.ValueOf(startSettingsWindowHere).Pointer() ||
		reflect.ValueOf(settingsWebView).Pointer() != reflect.ValueOf(settingsWebViewInTheApp).Pointer() ||
		reflect.ValueOf(settingsWebViewInstalled).Pointer() != reflect.ValueOf(settingsWebViewHere).Pointer() {
		t.Fatal("the Settings window isn't WebView2's on Windows")
	}
	d := newTestDeck(t)
	links := browserOpens(t)
	if err := d.openSetup(""); err != nil || len(*links) != 1 || !strings.Contains((*links)[0], "/setup#o=") {
		t.Errorf("with no window: the tray opened %v (%v), want the browser with a link", *links, err)
	}
	t.Logf("WebView2 registered here: %v", webview2Installed())
}

// The Settings window's WebView2 keeps its storage in a folder of its own, never the office window's, so its session's
// cookie is in a jar of its own, as on the Mac (RT126FIXREVIEW-OWNSETTINGS O2). It stays on top when the office does,
// so the office's ⚙︎ never opens it underneath (O3). Its message with no window is its own, whole (O6).
func TestOnWindowsTheSettingsWindowHasItsOwnStorageAndStaysOnTopWithTheOffice(t *testing.T) {
	d := newTestDeck(t)
	for _, onTop := range []bool{false, true} {
		d.mu.Lock()
		d.config.AlwaysOnTop = onTop
		d.mu.Unlock()
		settings, office := d.settingsWindowRun(d.newSettingsRun("")), d.windowRun()
		t.Logf("keep on top %v: the Settings window's folder %q, on top %v; the office's %q, on top %v",
			onTop, settings.dataDir, settings.onTop, office.dataDir, office.onTop)
		if settings.dataDir == office.dataDir || filepath.Dir(settings.dataDir) != filepath.Dir(office.dataDir) {
			t.Errorf("the Settings window keeps its storage at %q, the office at %q: want a folder of its own beside it",
				settings.dataDir, office.dataDir)
		}
		if settings.onTop != onTop || office.onTop != onTop {
			t.Errorf("keep on top %v: the Settings window on top %v, the office %v", onTop, settings.onTop, office.onTop)
		}
		if settings.missing == nil || settings.title != "Smolcorp Settings" {
			t.Errorf("the Settings window: title %q, its own message with no window %v", settings.title, settings.missing != nil)
		}
	}
}

// TestLiveSettingsWindow opens the real Settings window on a server of this test's own, and checks the page in it
// swapped the window's token for the window's session and read Setup with it: no 401. It needs a screen, so it runs
// only when asked:
//
//	SMOLCORP_LIVE_WINDOW=<seconds the window stays> go test ./desktop -run TestLiveSettingsWindow
func TestLiveSettingsWindow(t *testing.T) {
	if os.Getenv("SMOLCORP_LIVE_WINDOW") == "" {
		t.Skip("set SMOLCORP_LIVE_WINDOW=<seconds> to open the window")
	}
	isolate(t)
	deck := startDeck(t)
	pair(t, deck)
	type hit struct {
		path   string
		status int
	}
	hits := make(chan hit, 256)
	router := &router{deck: deck}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		router.ServeHTTP(recorder, r)
		select {
		case hits <- hit{r.Method + " " + r.URL.Path, recorder.status}:
		default:
		}
	}))
	defer server.Close()
	defer server.CloseClientConnections()

	run := deck.newSettingsRun("")
	run.url = strings.Replace(run.url, setupBase(), server.URL, 1)
	run.script = strings.Replace(run.script, setupBase(), server.URL, 1)
	saved := settingsWebView
	settingsWebView = func() bool { return true }
	t.Cleanup(func() { settingsWebView = saved })
	if err := startSettingsWindowHere(deck, run); err != nil {
		t.Fatal(err)
	}
	stay := 10 * time.Second
	if seconds, err := time.ParseDuration(os.Getenv("SMOLCORP_LIVE_WINDOW") + "s"); err == nil {
		stay = seconds
	}
	deadline := time.Now().Add(stay)
	var swapped, read bool
	for time.Now().Before(deadline) && !read {
		select {
		case h := <-hits:
			t.Logf("%s %d", h.path, h.status)
			swapped = swapped || h.path == "POST /setup/session" && h.status == http.StatusNoContent
			read = read || swapped && h.path == "GET /setup/state" && h.status == http.StatusOK
		case <-time.After(200 * time.Millisecond):
		}
	}
	settingsWindow.mu.Lock()
	forward := settingsWindow.forward
	settingsWindow.mu.Unlock()
	if forward != nil {
		forward("") // brought forward, never a second
	}
	if !swapped || !read {
		t.Errorf("the window's page swapped its token %v, read Setup with its session %v", swapped, read)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
