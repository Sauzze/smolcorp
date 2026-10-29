//go:build windows

package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestMonitorsAreFoundMainFirst(t *testing.T) {
	screens := monitors()
	if len(screens) == 0 {
		t.Skip("no screen in this session")
	}
	if s := screens[0]; s.Right <= s.Left || s.Bottom <= s.Top {
		t.Fatalf("an empty screen: %+v", s)
	}
}

func TestWebView2InstalledDoesNotPanic(t *testing.T) {
	t.Logf("WebView2 runtime registered: %v", webview2Installed())
}

// TestLiveWindow opens the real window on a server of this test's own (the
// office's real router, with its files and keys in temporary folders), so the
// running app and its port are left alone. It needs a screen, so it runs only
// when asked:
//
//	SMOLCORP_LIVE_WINDOW=<seconds the window stays> go test ./desktop -run TestLiveWindow
//
// Set SMOLCORP_LIVE_STATE to a file to read the window's place from, and
// SMOLCORP_LIVE_TOP=1 to open it on top.
func TestLiveWindow(t *testing.T) {
	seconds, _ := strconv.Atoi(os.Getenv("SMOLCORP_LIVE_WINDOW"))
	if seconds == 0 {
		t.Skip("set SMOLCORP_LIVE_WINDOW=<seconds> to open the window")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	isolate(t)
	deck := NewDeck()
	router := &router{deck: deck}
	hits := make(chan string, 256)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case hits <- r.URL.Path:
		default:
		}
		if r.URL.Path == "/hello" {
			_, _ = w.Write([]byte("<body style=background:#223;color:#fff><h1>Bonjour Smolcorp</h1>"))
			return
		}
		router.ServeHTTP(w, r)
	}))
	defer server.Close()
	defer server.CloseClientConnections() // the office's event stream would keep Close waiting

	deck.mu.Lock()
	key := ensureWindowKey(deck.access.Phones, windowKeyFile)
	deck.mu.Unlock()
	if os.Getenv("SMOLCORP_LIVE_HELLO") != "" {
		key = ""
	}
	placeFile := os.Getenv("SMOLCORP_LIVE_STATE")
	run := windowRun{
		url:      officeAddress(server.URL, key),
		fallback: officeAddress(server.URL, ""),
		onTop:    os.Getenv("SMOLCORP_LIVE_TOP") == "1",
		icon:     mustIcon(t),
		dataDir:  filepath.Join(supportDir, "WebView2"),
		save: func(s WindowState) {
			if placeFile != "" {
				_ = os.WriteFile(placeFile, []byte(fmt.Sprintf("%d %d %d %d %v", s.X, s.Y, s.Width, s.Height, s.Maximized)), 0o600)
			}
		},
		created: func(hwnd uintptr) {
			officeWindow.mu.Lock()
			officeWindow.hwnd = hwnd
			officeWindow.mu.Unlock()
			fmt.Printf("LIVE-HWND %d PID %d\n", hwnd, os.Getpid())
			go func() {
				time.Sleep(time.Duration(seconds) * time.Second)
				closeWindow(hwnd)
			}()
		},
	}
	if data, err := os.ReadFile(placeFile); err == nil {
		var s WindowState
		if n, _ := fmt.Sscanf(string(data), "%d %d %d %d %v", &s.X, &s.Y, &s.Width, &s.Height, &s.Maximized); n == 5 {
			run.state = &s
		}
	}
	if os.Getenv("SMOLCORP_LIVE_HELLO") != "" {
		run.url = server.URL + "/hello"
	}
	runOfficeWindow(run)

	// The office was asked for with the window's key, and answered with the deck, not the pairing page.
	seen := false
	for len(hits) > 0 {
		path := <-hits
		t.Logf("asked for %s", path)
		seen = seen || path == "/office"
	}
	if !seen {
		t.Fatal("the window never asked for /office")
	}
}

func mustIcon(t *testing.T) []byte {
	t.Helper()
	icon, err := trayIcons.ReadFile("tray-smolcorp.png")
	if err != nil {
		t.Fatal(err)
	}
	return icon
}

// closeWindow asks a window to close, as its close button does.
func closeWindow(hwnd uintptr) {
	const wmClose = 0x0010
	_, _, _ = user32.NewProc("PostMessageW").Call(hwnd, wmClose, 0, 0)
}

// TestLiveOpenTwice is the tray's Show Office clicked again: the window
// that is open comes back to the front, minimised or not, and there is never a second one.
// It pairs against nothing on the real port: only the window count and its state are looked at.
func TestLiveOpenTwice(t *testing.T) {
	if os.Getenv("SMOLCORP_LIVE_WINDOW") == "" {
		t.Skip("set SMOLCORP_LIVE_WINDOW=1 to open the window")
	}
	isolate(t)
	deck := NewDeck()
	deck.openOffice()
	deadline := time.Now().Add(20 * time.Second)
	var hwnd uintptr
	for time.Now().Before(deadline) && hwnd == 0 {
		time.Sleep(200 * time.Millisecond)
		officeWindow.mu.Lock()
		hwnd = officeWindow.hwnd
		officeWindow.mu.Unlock()
	}
	if hwnd == 0 {
		t.Fatal("the window never opened")
	}
	time.Sleep(3 * time.Second)
	t.Logf("windows after the 1st click: %d", ownWindows())

	// 2nd click with the window up, then minimised, then behind another window.
	deck.openOffice()
	time.Sleep(time.Second)
	_, _, _ = procShowWindow.Call(hwnd, 6) // SW_MINIMIZE
	time.Sleep(time.Second)
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic == 0 {
		t.Fatal("test set-up: the window should be minimised")
	}
	deck.openOffice()
	time.Sleep(time.Second)
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		t.Error("a click on Show Office must restore a minimised window")
	}
	front, _, _ := procGetForegroundWindow.Call()
	t.Logf("foreground is the office window: %v", front == hwnd)
	if n := ownWindows(); n != 1 {
		t.Errorf("%d office windows after 3 clicks, want 1", n)
	}
	closeWindow(hwnd)
	time.Sleep(2 * time.Second)
	if n := ownWindows(); n != 0 {
		t.Errorf("%d windows left after closing", n)
	}
	// Closed, the next click opens a new one.
	deck.openOffice()
	time.Sleep(8 * time.Second)
	officeWindow.mu.Lock()
	again := officeWindow.hwnd
	officeWindow.mu.Unlock()
	if again == 0 || again == hwnd || ownWindows() != 1 {
		t.Errorf("a click after closing must open a new window (hwnd %d, %d windows)", again, ownWindows())
	}
	closeWindow(again)
	time.Sleep(2 * time.Second)
}

// ownWindows counts this process's visible top-level windows titled Smolcorp.
func ownWindows() int {
	count := 0
	pid := uint32(os.Getpid())
	callback := windows.NewCallback(func(h, _ uintptr) uintptr {
		var owner uint32
		_, _, _ = user32.NewProc("GetWindowThreadProcessId").Call(h, uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return 1
		}
		if visible, _, _ := user32.NewProc("IsWindowVisible").Call(h); visible == 0 {
			return 1
		}
		title := make([]uint16, 64)
		n, _, _ := user32.NewProc("GetWindowTextW").Call(h, uintptr(unsafe.Pointer(&title[0])), 64)
		if windows.UTF16ToString(title[:n]) == "Smolcorp" {
			count++
		}
		return 1
	})
	_, _, _ = user32.NewProc("EnumWindows").Call(callback, 0)
	return count
}
