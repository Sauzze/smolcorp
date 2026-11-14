package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// The window's address never holds its key (RT126SK2 B2): WebView2 keeps addresses in its profile, a folder of files.
// The key goes to the page in-process, by a script for this computer's own office page alone, which the page reads.
func TestOfficeAddress(t *testing.T) {
	const key = "fake-window-key-abc"
	address := officeAddress("http://localhost:7710", key)
	if address != "http://localhost:7710/office?deck&own" || strings.Contains(address, key) || strings.Contains(address, "t=") {
		t.Fatal(address)
	}
	if got := officeAddress("http://localhost:7710", ""); got != "http://localhost:7710/office" {
		t.Fatal("no key, the pairing page:", got)
	}
	script := windowKeyScript("http://localhost:7710", key)
	for _, want := range []string{`location.origin === "http://localhost:7710"`, `location.pathname === '/office'`, `'smolcorpWindowKey', { value: "` + key + `"`} {
		if !strings.Contains(script, want) {
			t.Errorf("the script lacks %q: %s", want, script)
		}
	}
	if windowKeyScript("http://localhost:7710", "") != "" {
		t.Error("a script with no key")
	}
	if got := windowKeyScript("http://localhost:7710", `x"; alert(1); "`); !strings.Contains(got, `"x\"; alert(1); \""`) {
		t.Errorf("a key is quoted as JSON: %s", got)
	}
	// The page alone answers the address, and reads the key the script gave before ?t=.
	d := newTestDeck(t)
	d.mu.Lock()
	windowKey := ensureWindowKey(d.access.Phones, &d.access.windowDeviceKey)
	d.mu.Unlock()
	if w := serve(t, d, call{method: "GET", target: "/office?deck&own", host: "localhost:7710", from: thisComputer, bare: true}); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "const givenKey = typeof window.smolcorpWindowKey === 'string'") {
		t.Errorf("the window's address: %d, or the page doesn't read its key", w.Code)
	}
	if w := serve(t, d, call{method: "GET", target: "/paired?t=" + windowKey, host: "localhost:7710", from: thisComputer, bare: true}); w.Code != http.StatusNoContent {
		t.Errorf("the key the script gives: %d", w.Code)
	}
}

// The office's and Setup's own scripts parse, in node: a name declared twice in one (the window's key beside the
// ⧉ window key's, in FXSK2R1) stops the whole page, on every device, and no string check sees it. Without node the
// test is skipped.
func TestTheOfficeAndSetupScriptsParse(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the pages' scripts can't be checked")
	}
	inline := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	for _, name := range []string{"deck.html", "setup.html"} {
		data, err := os.ReadFile(filepath.Join("..", "Resources", "web", name))
		if err != nil {
			t.Fatal(err)
		}
		scripts := inline.FindAllStringSubmatch(string(data), -1)
		if len(scripts) == 0 {
			t.Fatalf("%s has no script of its own", name)
		}
		for i, script := range scripts {
			file := filepath.Join(t.TempDir(), fmt.Sprintf("%s-%d.js", name, i))
			if err := os.WriteFile(file, []byte(script[1]), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(node, "--check", file).CombinedOutput(); err != nil {
				t.Errorf("%s's script %d doesn't parse: %v\n%s", name, i, err, out)
			}
		}
	}
}

// The window's key lives in this process's memory alone (1.26, SK2): one device per run, opened again with the same
// key, and after a restart a new device in place of the one before, whose key no longer opens anything.
func TestWindowKeyPairsOncePerRunAndIsInNoFile(t *testing.T) {
	isolate(t)
	phones := loadPhones()
	var held string
	key := ensureWindowKey(phones, &held)
	if key == "" || held != key || len(phones.All) != 1 || phones.All[0].Name != windowDeviceName {
		t.Fatalf("expected one device %q, got %+v", windowDeviceName, phones.All)
	}
	if phones.WithKey(key) == nil {
		t.Fatal("the key must open the device")
	}
	// Opened again: the same device, not a second one.
	if again := ensureWindowKey(phones, &held); again != key || len(phones.All) != 1 {
		t.Fatalf("a second opening must reuse the device: %d devices", len(phones.All))
	}
	// After a restart: nothing was kept, so a new device takes the old one's place, and the old key opens nothing.
	restarted := loadPhones()
	var heldAgain string
	fresh := ensureWindowKey(restarted, &heldAgain)
	if fresh == "" || fresh == key || len(restarted.All) != 1 || restarted.WithKey(key) != nil || restarted.WithKey(fresh) == nil {
		t.Fatalf("after a restart: %d devices, the old key still opens one: %v", len(restarted.All), restarted.WithKey(key) != nil)
	}
	// No file holds either key: phones.json the hash only, and no window-key file.
	data, _ := os.ReadFile(phonesFile)
	if strings.Contains(string(data), key) || strings.Contains(string(data), fresh) {
		t.Fatal("phones.json must hold the key's hash only")
	}
	if _, err := os.Stat(windowKeyFile); !os.IsNotExist(err) {
		t.Fatalf("a window-key file was written: %v", err)
	}
}

// A window's key from the run before opens nothing once Smolcorp starts again, at once, before the owner opens the
// window in this run (RT126SK2R2 B2): this build's, and a 1.25 window's, whose WebView2 history keeps it, as 1.25 put
// it in the address. Its devices are gone from phones.json, and a phone paired then still opens the office.
func TestTheRunBeforesWindowKeyIsRefusedAtStart(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	before := ensureWindowKey(d.access.Phones, &d.access.windowDeviceKey)
	phone := d.access.Phones.Pair("Phone", false)
	d.mu.Unlock()
	// A 1.25 window's entry, with no seal, as 1.25's list had it.
	const before125 = "fake-125-window-key"
	var listed []PairedPhone
	data, _ := os.ReadFile(phonesFile)
	if err := json.Unmarshal(data, &listed); err != nil || len(listed) != 2 {
		t.Fatalf("phones.json before the restart: %v, %d entries", err, len(listed))
	}
	listed = append(listed, PairedPhone{ID: "w125", Name: windowDeviceName, KeyHash: hashKey(before125)})
	data, _ = json.Marshal(listed)
	if err := os.WriteFile(phonesFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	opens := func(d *Deck, key string) int {
		return serve(t, d, call{method: "GET", target: "/paired?t=" + key, host: "localhost:7710", from: thisComputer, bare: true}).Code
	}
	restarted := startDeck(t) // the same folder: Smolcorp started again, its window not opened yet
	if code := opens(restarted, before); code != http.StatusUnauthorized {
		t.Errorf("the run before's window key, right after a restart: GET /paired %d, want 401", code)
	}
	if code := opens(restarted, before125); code != http.StatusUnauthorized {
		t.Errorf("a 1.25 window's key, right after a restart: GET /paired %d, want 401", code)
	}
	if code := opens(restarted, phone); code != http.StatusNoContent {
		t.Errorf("the phone, after a restart: GET /paired %d, want 204", code)
	}
	data, _ = os.ReadFile(phonesFile)
	if strings.Contains(string(data), windowDeviceName) || len(restarted.access.Phones.All) != 1 {
		t.Errorf("the window's devices are still listed: %s", data)
	}
	// Saved under the new record key as one: the phone still opens the office at the start after.
	if code := opens(startDeck(t), phone); code != http.StatusNoContent {
		t.Errorf("the phone, a start later: GET /paired %d, want 204", code)
	}
}

func TestUnpairAllRemovesTheWindowAndItComesBack(t *testing.T) {
	isolate(t)
	phones := loadPhones()
	phone := phones.Pair("Phone", true)
	var held string
	window := ensureWindowKey(phones, &held)
	if phonesOnly(phones) != 1 || len(phones.All) != 2 {
		t.Fatalf("the window is not a phone: %d phones of %d devices", phonesOnly(phones), len(phones.All))
	}
	phones.RemoveAll()
	if phones.WithKey(window) != nil || phones.WithKey(phone) != nil {
		t.Fatal("Unpair All must remove every device, the window's too")
	}
	fresh := ensureWindowKey(phones, &held)
	if fresh == window || phones.WithKey(fresh) == nil || len(phones.All) != 1 {
		t.Fatal("the next opening must pair a new device for the window")
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

// Setup's "Keep the office on top of other windows" is the tray's Always on
// Top: off to begin with, kept, and only where the office has a window of its
// own. Elsewhere setup is told there's none, to say why there's no switch.
func TestSetupKeepsTheOfficeOnTopWhereItHasAWindow(t *testing.T) {
	d := newTestDeck(t)
	shown := func() map[string]any {
		return (&router{deck: d}).setupState()["officeWindow"].(map[string]any)
	}
	turn := func(enabled string) int {
		return serve(t, d, call{method: "POST", target: "/setup/ontop?enabled=" + enabled, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	if w := shown(); w["available"] != hasNativeWindow || w["onTop"] != false || d.config.AlwaysOnTop {
		t.Fatalf("a new config: %v, saved %v; want off", w, d.config.AlwaysOnTop)
	}
	if !hasNativeWindow {
		if code := turn("1"); code != http.StatusNotFound || loadConfig().AlwaysOnTop {
			t.Fatalf("no window here, yet turned on: %d", code)
		}
		return
	}
	if code := turn("1"); code != http.StatusOK || shown()["onTop"] != true || !loadConfig().AlwaysOnTop {
		t.Fatalf("turned on: %d, %v, saved %v", code, shown(), loadConfig().AlwaysOnTop)
	}
	if code := turn("0"); code != http.StatusOK || shown()["onTop"] != false || loadConfig().AlwaysOnTop {
		t.Fatalf("turned off: %d, %v", code, shown())
	}
	// Only from this computer, as every setting is.
	if got := serve(t, d, call{method: "POST", target: "/setup/ontop?enabled=1", host: deckOnWiFi, from: aPhone,
		headers: map[string]string{setupHeader: "1"}}); got.Code == http.StatusOK || d.config.AlwaysOnTop {
		t.Fatalf("a phone turned it on: %d", got.Code)
	}
}
