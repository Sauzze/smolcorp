package desktop

import (
	"os"
	"strings"
	"testing"
)

var oneScreen = []screenRect{{0, 0, 1920, 1040}}
var twoScreens = []screenRect{{0, 0, 1920, 1040}, {1920, 0, 3840, 1040}}

func TestFitWindowKeepsAPlaceThatIsOnAScreen(t *testing.T) {
	saved := &WindowState{X: 300, Y: 200, Width: 900, Height: 600, Maximized: true}
	got, ok := fitWindow(saved, oneScreen)
	if !ok || got != *saved {
		t.Fatalf("got %+v ok=%v, want the saved place", got, ok)
	}
}

func TestFitWindowDropsAPlaceOnAScreenThatIsGone(t *testing.T) {
	saved := &WindowState{X: 2500, Y: 100, Width: 900, Height: 600}
	if _, ok := fitWindow(saved, twoScreens); !ok {
		t.Fatal("a place on the second screen is fine while it is there")
	}
	got, ok := fitWindow(saved, oneScreen)
	if ok {
		t.Fatalf("the second screen is gone, yet the place was kept: %+v", got)
	}
	if got.Width != defaultWindowWidth || got.Height != defaultWindowHeight {
		t.Fatalf("expected the default size, got %+v", got)
	}
}

func TestFitWindowNeedsTheTitleBarOnAScreen(t *testing.T) {
	// Only the bottom of the window is on the screen: it couldn't be dragged back.
	if _, ok := fitWindow(&WindowState{X: 100, Y: 1030, Width: 900, Height: 600}, oneScreen); ok {
		t.Fatal("a window whose title bar is below the screen must not be kept")
	}
	if _, ok := fitWindow(&WindowState{X: 100, Y: -500, Width: 900, Height: 600}, oneScreen); ok {
		t.Fatal("a window whose title bar is above the screen must not be kept")
	}
	// A sliver at the edge is still too little to grab.
	if _, ok := fitWindow(&WindowState{X: 1900, Y: 100, Width: 900, Height: 600}, oneScreen); ok {
		t.Fatal("20 pixels on a screen are too few")
	}
}

func TestFitWindowShrinksToTheScreen(t *testing.T) {
	got, ok := fitWindow(&WindowState{X: 0, Y: 0, Width: 5000, Height: 4000}, oneScreen)
	if !ok || got.Width != 1920 || got.Height != 1040 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	got, _ = fitWindow(&WindowState{X: 10, Y: 10, Width: 10, Height: 10}, oneScreen)
	if got.Width != minWindowWidth || got.Height != minWindowHeight {
		t.Fatalf("a tiny window is brought up to the minimum, got %+v", got)
	}
}

func TestFitWindowWithNothingToGoOn(t *testing.T) {
	for name, c := range map[string]struct {
		saved   *WindowState
		screens []screenRect
	}{
		"never opened":   {nil, oneScreen},
		"empty size":     {&WindowState{X: 5, Y: 5}, oneScreen},
		"no screen info": {&WindowState{X: 5, Y: 5, Width: 800, Height: 600}, nil},
	} {
		if _, ok := fitWindow(c.saved, c.screens); ok {
			t.Errorf("%s: nothing to restore, yet ok", name)
		}
	}
}

func TestCentred(t *testing.T) {
	got := centred(1000, 600, screenRect{100, 50, 1100, 850})
	if got.X != 100 || got.Y != 150 {
		t.Fatalf("got %+v", got)
	}
	got = centred(5000, 5000, screenRect{0, 0, 800, 600})
	if got.Width != 800 || got.Height != 600 || got.X != 0 || got.Y != 0 {
		t.Fatalf("a window never bigger than its screen, got %+v", got)
	}
}

func TestOfficeAddress(t *testing.T) {
	if got := officeAddress("http://localhost:7710", "abc-_"); got != "http://localhost:7710/office?t=abc-_" {
		t.Fatal(got)
	}
	if got := officeAddress("http://localhost:7710", ""); got != "http://localhost:7710/office" {
		t.Fatal("no key, no ?t=:", got)
	}
}

func TestWindowKeyPairsOnceAndIsKept(t *testing.T) {
	isolate(t)
	phones := loadPhones()
	key := ensureWindowKey(phones, windowKeyFile)
	if key == "" || len(phones.All) != 1 || phones.All[0].Name != windowDeviceName {
		t.Fatalf("expected one device %q, got %+v", windowDeviceName, phones.All)
	}
	if phones.WithKey(key) == nil {
		t.Fatal("the key must open the device")
	}
	// Opened again: the same device, not a second one.
	if again := ensureWindowKey(phones, windowKeyFile); again != key || len(phones.All) != 1 {
		t.Fatalf("a second opening must reuse the device: %d devices", len(phones.All))
	}
	// After a restart too: the key is read from its file, the phones from theirs.
	if again := ensureWindowKey(loadPhones(), windowKeyFile); again != key {
		t.Fatal("the key must survive a restart")
	}
	// Only a hash is stored beside the phones: the key is in no other file.
	data, _ := os.ReadFile(phonesFile)
	if strings.Contains(string(data), key) {
		t.Fatal("phones.json must hold the key's hash only")
	}
}

func TestUnpairAllRemovesTheWindowAndItComesBack(t *testing.T) {
	isolate(t)
	phones := loadPhones()
	phone := phones.Pair("Phone")
	window := ensureWindowKey(phones, windowKeyFile)
	if phonesOnly(phones) != 1 || len(phones.All) != 2 {
		t.Fatalf("the window is not a phone: %d phones of %d devices", phonesOnly(phones), len(phones.All))
	}
	phones.RemoveAll()
	if phones.WithKey(window) != nil || phones.WithKey(phone) != nil {
		t.Fatal("Unpair All must remove every device, the window's too")
	}
	fresh := ensureWindowKey(phones, windowKeyFile)
	if fresh == window || phones.WithKey(fresh) == nil || len(phones.All) != 1 {
		t.Fatal("the next opening must pair a new device for the window")
	}
}

func TestWindowKeyFileIsPrivateAndNotALoginPath(t *testing.T) {
	isolate(t)
	phones := loadPhones()
	key := ensureWindowKey(phones, windowKeyFile)
	saved, err := os.ReadFile(windowKeyFile)
	if err != nil || strings.TrimSpace(string(saved)) != key {
		t.Fatalf("the key must be in %s: %v", windowKeyFile, err)
	}
}

func TestWindowStateIsKeptInTheConfig(t *testing.T) {
	isolate(t)
	c := loadConfig()
	if c.Window != nil || c.AlwaysOnTop {
		t.Fatal("a new config has no remembered window")
	}
	c.Window = &WindowState{X: 10, Y: 20, Width: 800, Height: 600, Maximized: true}
	c.AlwaysOnTop = true
	c.save()
	got := loadConfig()
	if got.Window == nil || *got.Window != *c.Window || !got.AlwaysOnTop {
		t.Fatalf("got %+v, top=%v", got.Window, got.AlwaysOnTop)
	}
}
