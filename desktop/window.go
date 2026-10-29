package desktop

import (
	"fmt"
	"os"
	"strings"
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

// officeAddress is the office on this computer's own port, with the window's
// key after ?t=, as a phone's link carries its key. The key stays inside this
// process and the window: it is never logged or put on a command line.
func officeAddress(base, key string) string {
	if key == "" {
		return base + "/office"
	}
	return base + "/office?t=" + key
}

func localBase() string { return fmt.Sprintf("http://localhost:%d", Port) }

// ensureWindowKey is the key of the window's own paired device. It is read from
// the private file next to the other keys; if there is none, or no device holds
// it any more (Unpair All removed it), a new device is paired and its key kept.
// The caller holds the deck's lock.
func ensureWindowKey(phones *Phones, keyFile string) string {
	if saved, err := os.ReadFile(keyFile); err == nil {
		if key := strings.TrimSpace(string(saved)); phones.WithKey(key) != nil {
			return key
		}
	}
	key := phones.Pair(windowDeviceName)
	_ = writePrivate(keyFile, []byte(key+"\n"))
	return key
}

// phonesOnly counts the phones, not this app's own window.
func phonesOnly(phones *Phones) int {
	n := 0
	for _, p := range phones.All {
		if p.Name != windowDeviceName && p.Name != windowDeviceNameBefore {
			n++
		}
	}
	return n
}
