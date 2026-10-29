//go:build windows

package desktop

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The office in a WebView2 window (the Edge engine Windows already has), made
// without cgo: go-webview2 loads WebView2Loader.dll itself.
//
// The tray and the window each need a message loop of their own, so the window
// lives on a goroutine locked to its own OS thread, from creation to close.

const hasNativeWindow = true

var (
	procIsIconic                 = user32.NewProc("IsIconic")
	procIsWindow                 = user32.NewProc("IsWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetWindowPlacement       = user32.NewProc("GetWindowPlacement")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procEnumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procSetProcessDpiAwareness   = user32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	swShow            = 5
	swMaximize        = 3
	swRestore         = 9
	swpNoSize         = 0x0001
	swpNoMove         = 0x0002
	swpNoZOrder       = 0x0004
	swpNoActivate     = 0x0010
	wmSetIcon         = 0x0080
	iconSmall         = 0
	iconBig           = 1
	monitorPrimary    = 1
	perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4
)

var (
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	hwndNoTopmost = ^uintptr(1) // (HWND)-2
)

// officeWindow is the one window there can be.
var officeWindow struct {
	mu      sync.Mutex
	busy    bool    // a window is opening, open, or its message is on screen
	hwnd    uintptr // 0 until the window exists
	dpiOnce sync.Once
}

// windowRun is everything a window needs, so a test can open one on a server of
// its own, without the app's files.
type windowRun struct {
	url      string // the office, with the window's key
	fallback string // the office as the browser opens it: no key on a command line
	state    *WindowState
	onTop    bool
	icon     []byte // PNG
	dataDir  string // where WebView2 keeps the page's storage
	save     func(WindowState)
	created  func(hwnd uintptr)
}

// openOffice is the tray's Show Office: the window, or the one that is
// already open brought to the front.
func (d *Deck) openOffice() {
	officeWindow.mu.Lock()
	if officeWindow.busy {
		hwnd := officeWindow.hwnd
		officeWindow.mu.Unlock()
		if hwnd != 0 {
			raiseWindow(hwnd)
		}
		return
	}
	officeWindow.busy = true
	officeWindow.mu.Unlock()

	run := d.windowRun()
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer func() {
			officeWindow.mu.Lock()
			officeWindow.busy, officeWindow.hwnd = false, 0
			officeWindow.mu.Unlock()
		}()
		runOfficeWindow(run)
	}()
}

// setAlwaysOnTop is the menu's check box: kept in the config, and applied to the
// window if there is one.
func (d *Deck) setAlwaysOnTop(on bool) {
	d.mu.Lock()
	d.config.AlwaysOnTop = on
	d.config.save()
	d.mu.Unlock()
	officeWindow.mu.Lock()
	hwnd := officeWindow.hwnd
	officeWindow.mu.Unlock()
	if hwnd != 0 {
		applyTopmost(hwnd, on)
	}
}

func (d *Deck) windowRun() windowRun {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := ensureWindowKey(d.access.Phones, windowKeyFile)
	icon, _ := trayIcons.ReadFile("tray-" + d.config.Logo + ".png")
	var state *WindowState
	if d.config.Window != nil {
		copied := *d.config.Window
		state = &copied
	}
	return windowRun{
		url:      officeAddress(localBase(), key),
		fallback: localBase() + "/office?o=" + d.access.NewOfficeToken(), // works once, as window_other.go's: no code to type
		state:    state,
		onTop:    d.config.AlwaysOnTop,
		icon:     icon,
		dataDir:  filepath.Join(supportDir, "WebView2"),
		save: func(s WindowState) {
			d.mu.Lock()
			d.config.Window = &s
			d.config.save()
			d.mu.Unlock()
		},
		created: func(hwnd uintptr) {
			officeWindow.mu.Lock()
			officeWindow.hwnd = hwnd
			officeWindow.mu.Unlock()
		},
	}
}

// runOfficeWindow shows the window and returns when it is closed. It runs on
// its own locked OS thread.
func runOfficeWindow(r windowRun) {
	officeWindow.dpiOnce.Do(func() {
		// Sharp on a scaled screen, rather than stretched by Windows.
		_, _, _ = procSetProcessDpiAwareness.Call(perMonitorAwareV2)
	})
	// Test hook: pretend WebView2 isn't installed, to see the fallback.
	if os.Getenv("SMOLCORP_TEST_NO_WEBVIEW2") != "" || !webview2Installed() {
		missingWebView2(r, "WebView2 est introuvable sur cet ordinateur.")
		return
	}

	screens := monitors()
	state, placed := fitWindow(r.state, screens)
	if !placed && len(screens) > 0 {
		state = centred(defaultWindowWidth, defaultWindowHeight, screens[0])
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  r.dataDir,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Smolcorp",
			Width:  uint(state.Width),
			Height: uint(state.Height),
		},
	})
	if w == nil {
		missingWebView2(r, "WebView2 didn't respond.")
		return
	}
	hwnd := uintptr(w.Window())
	// The first ShowWindow of a process follows how the process was started
	// (hidden, from a launcher that asked for it); this one makes sure of it.
	_, _, _ = procShowWindow.Call(hwnd, swShow)
	setWindowIcon(hwnd, r.icon)
	if placed || len(screens) > 0 {
		_, _, _ = procSetWindowPos.Call(hwnd, 0, uintptr(state.X), uintptr(state.Y), uintptr(state.Width), uintptr(state.Height), swpNoZOrder|swpNoActivate)
	}
	if r.onTop {
		applyTopmost(hwnd, true)
	}
	if placed && state.Maximized {
		_, _, _ = procShowWindow.Call(hwnd, swMaximize)
	}
	if r.created != nil {
		r.created(hwnd)
	}
	w.Navigate(r.url)

	// Where the window is, kept while it is: by the time Run returns the window
	// is gone. Reading a window's place works from any thread.
	var mu sync.Mutex
	var last *WindowState
	remember := func() {
		if s, ok := placement(hwnd); ok {
			mu.Lock()
			last = &s
			mu.Unlock()
		}
	}
	remember()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(300 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				remember()
			}
		}
	}()

	w.Run()

	close(stop)
	<-done
	mu.Lock()
	defer mu.Unlock()
	if last != nil && r.save != nil {
		r.save(*last)
	}
}

// missingWebView2 tells why there's no window, then opens the office in the
// browser, with a link that works once instead of the window's key.
func missingWebView2(r windowRun, why string) {
	log.Printf("smolcorp: office window unavailable: %s", why)
	text := why + "\n\nSmolcorp's office window can't open, so the office opens in your browser instead.\n\nFor the window, install the Microsoft Edge WebView2 Runtime, free from Microsoft, then choose Show Office again."
	title, _ := windows.UTF16PtrFromString("Smolcorp")
	body, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
	openURL(r.fallback)
}

// webview2Installed: the runtime registers its version where the Edge updater
// keeps its clients, for everyone or for this user only.
func webview2Installed() bool {
	const client = `\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	places := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node` + client},
		{registry.LOCAL_MACHINE, `SOFTWARE` + client},
		{registry.CURRENT_USER, `Software` + client},
	}
	for _, p := range places {
		key, err := registry.OpenKey(p.root, p.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && version != "" && version != "0.0.0.0" {
			return true
		}
	}
	return false
}

// raiseWindow brings the window to the front, out of the taskbar if it was
// minimised. Windows lets only the foreground thread take the front, so this
// thread joins its input for the moment it takes.
func raiseWindow(hwnd uintptr) {
	if ok, _, _ := procIsWindow.Call(hwnd); ok == 0 {
		return
	}
	if minimised, _, _ := procIsIconic.Call(hwnd); minimised != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swRestore)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	this := windows.GetCurrentThreadId()
	front, _, _ := procGetForegroundWindow.Call()
	var frontThread uintptr
	if front != 0 {
		frontThread, _, _ = procGetWindowThreadProcessID.Call(front, 0)
	}
	attached := frontThread != 0 && uint32(frontThread) != this
	if attached {
		_, _, _ = procAttachThreadInput.Call(uintptr(this), frontThread, 1)
	}
	_, _, _ = procBringWindowToTop.Call(hwnd)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	if attached {
		_, _, _ = procAttachThreadInput.Call(uintptr(this), frontThread, 0)
	}
}

func applyTopmost(hwnd uintptr, on bool) {
	after := hwndNoTopmost
	if on {
		after = hwndTopmost
	}
	_, _, _ = procSetWindowPos.Call(hwnd, after, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
}

// setWindowIcon gives the window the tray's picture, a PNG Windows reads as it is.
func setWindowIcon(hwnd uintptr, png []byte) {
	if len(png) == 0 {
		return
	}
	for _, icon := range []struct{ kind, size uintptr }{{iconBig, 64}, {iconSmall, 32}} {
		handle, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, 0x00030000, icon.size, icon.size, 0)
		if handle != 0 {
			_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, icon.kind, handle)
		}
	}
}

type winRect struct{ Left, Top, Right, Bottom int32 }

// placement is where the window sits when it is not maximised, and whether it is.
func placement(hwnd uintptr) (WindowState, bool) {
	var p struct {
		Length, Flags, ShowCmd uint32
		MinPosition            [2]int32
		MaxPosition            [2]int32
		Normal                 winRect
	}
	p.Length = uint32(unsafe.Sizeof(p))
	if ok, _, _ := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p))); ok == 0 {
		return WindowState{}, false
	}
	n := p.Normal
	return WindowState{X: int(n.Left), Y: int(n.Top), Width: int(n.Right - n.Left), Height: int(n.Bottom - n.Top), Maximized: p.ShowCmd == swMaximize}, true
}

type foundMonitor struct {
	rect    screenRect
	primary bool
}

// The callback is made once (Windows allows only so many), and filled in under the lock.
var (
	monitorsMu       sync.Mutex
	monitorsFound    []foundMonitor
	monitorsCallback = windows.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
		var info struct {
			Size          uint32
			Monitor, Work winRect
			Flags         uint32
		}
		info.Size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
			w := info.Work
			monitorsFound = append(monitorsFound, foundMonitor{screenRect{int(w.Left), int(w.Top), int(w.Right), int(w.Bottom)}, info.Flags&monitorPrimary != 0})
		}
		return 1
	})
)

// monitors are the screens' work areas, the main one first.
func monitors() []screenRect {
	monitorsMu.Lock()
	defer monitorsMu.Unlock()
	monitorsFound = nil
	_, _, _ = procEnumDisplayMonitors.Call(0, 0, monitorsCallback, 0)
	all := monitorsFound
	sort.SliceStable(all, func(i, j int) bool { return all[i].primary && !all[j].primary })
	screens := make([]screenRect, len(all))
	for i, f := range all {
		screens[i] = f.rect
	}
	return screens
}
