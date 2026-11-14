package desktop

import (
	"encoding/json"
	"fmt"
	"slices"
)

// The office in a window of its own (Windows: window_windows.go; elsewhere the
// browser, window_other.go). What follows is the part that doesn't depend on
// the platform, so it is tested everywhere.

// windowDeviceName is how the setup page lists the window: a paired device of
// its own, made by this app for itself, never by a code on a screen.
// windowDeviceNameBefore is the name builds before 1.22 gave it, still the
// window's and not a phone.
const (
	windowDeviceName       = "Smolcorp (window)"
	windowDeviceNameBefore = "Smolcorp (fenêtre)"
)

// WindowState is where the office window was left, in screen pixels.
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
}

// screenRect is one screen, in virtual-desktop pixels.
type screenRect struct{ Left, Top, Right, Bottom int }

const (
	defaultWindowWidth  = 1100
	defaultWindowHeight = 760
	minWindowWidth      = 480
	minWindowHeight     = 320
	// How much of the title bar must still be on a screen for the window to be
	// reachable (to be grabbed and dragged).
	reachableWidth  = 120
	reachableHeight = 32
)

// fitWindow says where to open the window: the remembered place if some of its
// title bar is still on a screen that exists (a screen may have been unplugged
// or its resolution changed), with a size that fits it. ok is false when
// there's nothing to go on, and the window opens at the default size, centred.
func fitWindow(saved *WindowState, screens []screenRect) (state WindowState, ok bool) {
	if saved == nil || saved.Width <= 0 || saved.Height <= 0 || len(screens) == 0 {
		return WindowState{Width: defaultWindowWidth, Height: defaultWindowHeight}, false
	}
	state = *saved
	state.Width = max(state.Width, minWindowWidth)
	state.Height = max(state.Height, minWindowHeight)
	for _, s := range screens {
		if overlap(state.X, state.X+state.Width, s.Left, s.Right) >= min(reachableWidth, state.Width) &&
			overlap(state.Y, state.Y+reachableHeight, s.Top, s.Bottom) >= reachableHeight {
			// Never bigger than the screen it is on.
			state.Width = min(state.Width, s.Right-s.Left)
			state.Height = min(state.Height, s.Bottom-s.Top)
			return state, true
		}
	}
	return WindowState{Width: defaultWindowWidth, Height: defaultWindowHeight}, false
}

func overlap(a1, a2, b1, b2 int) int { return max(0, min(a2, b2)-max(a1, b1)) }

// centred is a size of window in the middle of a screen.
func centred(width, height int, s screenRect) WindowState {
	width = min(width, s.Right-s.Left)
	height = min(height, s.Bottom-s.Top)
	return WindowState{X: s.Left + (s.Right-s.Left-width)/2, Y: s.Top + (s.Bottom-s.Top-height)/2, Width: width, Height: height}
}

// officeAddress is the office on this computer's own port, for the window: the
// page alone (?deck, as a phone's), never its key (RT126SK2 B2). A WebView2
// keeps its address in its profile, a folder of files (its history and the
// pages to restore), so the key goes to the page in-process instead, by
// windowKeyScript. With no key (none could be paired), the pairing page: the
// window opens locked.
func officeAddress(base, key string) string {
	if key == "" {
		return base + "/office"
	}
	return base + "/office?deck&" + ownWindowMark
}

// windowKeyScript is the script the window runs before each page of its own,
// handed to WebView2 in this process (AddScriptToExecuteOnDocumentCreated, kept
// in memory, never in the profile): it gives the office its key, on this
// computer's own office page alone. deck.html reads it before ?t=. "" with no key.
func windowKeyScript(base, key string) string {
	if key == "" {
		return ""
	}
	origin, _ := json.Marshal(base)
	value, _ := json.Marshal(key)
	return fmt.Sprintf("if (location.origin === %s && location.pathname === '/office') "+
		"Object.defineProperty(window, 'smolcorpWindowKey', { value: %s, writable: false, enumerable: false });", origin, value)
}

// ownWindowMark marks the office's own window on this computer in its address, as
// the Mac's office window is marked by ?window: the page asks /t3code only there,
// so a browser paired with its own key never asks and is never refused (403). It
// grants nothing: the key still decides, on every request.
const ownWindowMark = "own"

func localBase() string { return fmt.Sprintf("http://localhost:%d", Port) }

// setupBase is where Setup opens: 127.0.0.1, never localhost, so its cookie is never sent to a desk's own server,
// whose preview is on localhost (server.go sends localhost's Setup on here).
func setupBase() string { return fmt.Sprintf("http://127.0.0.1:%d", Port) }

// setupOnLoopbackIP is what a request to Setup at localhost that carries its cookie is told.
func setupOnLoopbackIP() string {
	return "Setup opens at " + setupBase() + "/setup, never at localhost. Open it again from Smolcorp's menu."
}

// ensureWindowKey is the key of the window's own paired device, held in this
// process's memory alone (held): the window is handed it in-process, and no
// file keeps it (SANDBOX-PLAN §3.1, SK2). The first time in a run, or once no
// device holds it any more (Unpair All removed it), the window's devices from
// before are let go and a new one is paired. If none can be paired (Pair), it
// is "": the window opens locked, and its next opening tries again. The caller
// holds the deck's lock.
func ensureWindowKey(phones *Phones, held *string) string {
	if *held != "" && phones.WithKey(*held) != nil {
		return *held
	}
	dropWindowDevices(phones)
	// Its key goes over this computer's loopback only, bare, and never signs.
	*held = phones.Pair(windowDeviceName, false)
	return *held
}

// dropWindowDevices lets go of every device the office window paired, let in
// or not. NewAccess runs it at each start, on
// every computer (only Windows makes them): a window's key from the run before
// opens nothing in this one, even before the window is first opened
// (RT126SK2R2 B2). A 1.25 window put its key in its address, which WebView2
// keeps in its history. The rest are saved under a new record key, as a phone
// removed is (Phones.Remove), so a copy of phones.json from before no longer
// fits (records.go).
func dropWindowDevices(phones *Phones) {
	before := len(phones.All)
	phones.All = slices.DeleteFunc(phones.All, func(phone PairedPhone) bool { return phone.isWindowDevice() })
	if len(phones.All) == before {
		return
	}
	phones.keep(true)
}

// phonesOnly counts the phones, not this app's own window.
func phonesOnly(phones *Phones) int {
	n := 0
	for _, p := range phones.All {
		if !p.isWindowDevice() {
			n++
		}
	}
	return n
}

// phonesKnown is phonesOnly: since 1.26 no entry waits for the owner to confirm it (access.go), so the phones let
// in are all there are, where the count decides what to offer or keep open.
func phonesKnown(phones *Phones) int {
	return phonesOnly(phones)
}

// isWindowDevice is whether the paired device is the office window's own (ensureWindowKey), by either name.
func (p PairedPhone) isWindowDevice() bool {
	return p.Name == windowDeviceName || p.Name == windowDeviceNameBefore
}
