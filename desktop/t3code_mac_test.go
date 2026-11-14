package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// t3MacStandIn stands in for the Mac app on T3 Code's actions (HostBridge.swift's t3-app, t3-open-app and
// t3-link): the app there or not, and what it was asked. Nothing real is found or opened.
type t3MacStandIn struct {
	mu     sync.Mutex
	asked  []bridgeRequest
	app    bool
	refuse bool
}

func (m *t3MacStandIn) call(request bridgeRequest) (bridgeAnswer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asked = append(m.asked, request)
	if m.refuse {
		return bridgeAnswer{Error: "The Mac didn't answer in time."}, nil
	}
	switch request.Action {
	case "t3-app":
		return bridgeAnswer{OK: true, Found: m.app}, nil
	case "t3-open-app", "t3-link":
		return bridgeAnswer{OK: true}, nil
	}
	return bridgeAnswer{Error: "This terminal can't do that.", Unsupported: true}, nil
}

func (m *t3MacStandIn) actions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for _, request := range m.asked {
		names = append(names, request.Action+" "+request.Link)
	}
	return names
}

func withT3Mac(t *testing.T, app bool) *t3MacStandIn {
	t.Helper()
	saved := host
	t.Cleanup(func() { host = saved })
	mac := &t3MacStandIn{app: app}
	host = mac
	return mac
}

// C4 A1: on a Mac, T3 Code's app is found by its bundle id, which only the Mac app can ask macOS for: its "launcher"
// is that id when the app says it's there, and nothing without the app, or when it says no.
func TestT3OnAMacTheAppIsFoundByItsBundleIDThroughTheMacApp(t *testing.T) {
	m := t3Machine{GOOS: "darwin", Home: t.TempDir()}
	saved := host
	host = nil
	if got := t3AppLauncher(m); got != "" {
		t.Errorf("a bare core found %q", got)
	}
	host = saved
	mac := withT3Mac(t, true)
	if got := t3AppLauncher(m); got != t3MacBundleID {
		t.Errorf("found %q", got)
	}
	mac.app = false
	if got := t3AppLauncher(m); got != "" {
		t.Errorf("not there: %q", got)
	}
	mac.app, mac.refuse = true, true
	if got := t3AppLauncher(m); got != "" {
		t.Errorf("the app's no: %q", got)
	}
	if got := mac.actions(); !slices.Equal(got, []string{"t3-app ", "t3-app ", "t3-app "}) {
		t.Errorf("asked %q", got)
	}
}

// C4 A1: so /setup/t3code says found with the app and t3, and appOnly, with the Mac's hint, with the app alone.
func TestT3OnAMacTheStateIsFoundOrAppOnlyAsOnTheSwiftEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a Mac's t3 is found by its exec bit, which a file on Windows doesn't carry")
	}
	withT3Mac(t, true)
	home := t.TempDir()
	m := t3Machine{GOOS: "darwin", Home: home, Run: func(string, []string, time.Duration) t3Ran {
		return t3Ran{Ended: true, Output: "t3 v0.1.8\n"}
	}}
	m.App = func() string { return t3AppLauncher(m) }
	if got := t3Detect(m); got != (t3Status{State: t3AppOnlyIs}) || t3JSON(got)["hint"] != t3AppOnly {
		t.Errorf("the app alone: %+v", got)
	}
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "t3"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := t3Detect(m); got != (t3Status{State: t3Found, Version: "v0.1.8"}) {
		t.Errorf("the app and t3: %+v", got)
	}
}

// C4 A1: first setup's T3 Code question is asked when only the app is there: choices.t3Installed asks the Mac app
// too. Without it, nothing is made up.
func TestT3OnAMacChoicesSeeTheAppAlone(t *testing.T) {
	saved := host
	host = nil
	if t3MacAppFound() {
		t.Error("found without the Mac app")
	}
	host = saved
	mac := withT3Mac(t, true)
	if !t3MacAppFound() {
		t.Error("the app, not seen")
	}
	mac.app = false
	if t3MacAppFound() {
		t.Error("seen when it isn't there")
	}
}

// C4 A3: the T3 Code sentences say "Mac" under the Mac app, as the Swift engine does, and "computer" elsewhere.
func TestT3SentencesSayMacUnderTheMacApp(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	h := Hire{ID: "abcdef12", Name: "Ada", Engine: t3Engine, T3Thread: "mcp:abc-1"}
	if _, said := t3OpenThread(h, t3Status{State: t3None}, t3Machine{}); said != "T3 Code's app isn't on this computer, so the hire's thread can't be shown there." {
		t.Errorf("%v", said)
	}
	withT3Mac(t, false)
	if status, said := t3OpenThread(h, t3Status{State: t3None}, t3Machine{}); status != http.StatusConflict ||
		said != "T3 Code's app isn't on this Mac, so the hire's thread can't be shown there." {
		t.Errorf("%d %v", status, said)
	}
	if t3CantLook("Ada") != "Smolcorp can't see which processes hold Ada's session on this Mac, so nothing was changed: two agents on one session lose turns silently." {
		t.Error(t3CantLook("Ada"))
	}
	d := newTestDeck(t)
	phone := pair(t, d)
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	got := serve(t, d, call{method: "GET", target: "/t3code?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), "T3 Code opens on your Mac.") {
		t.Errorf("%d %q", got.Code, got.Body)
	}
}
