package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
)

// noBusWithFakeLaunch leaves no session bus and puts a dbus-launch on PATH
// that only writes a marker; it gives the marker's path.
func noBusWithFakeLaunch(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	marker := filepath.Join(bin, "started")
	stub := "#!/bin/sh\necho started > " + marker + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "dbus-launch"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	saved := runUserDir
	t.Cleanup(func() { runUserDir = saved })
	runUserDir = func() string { return t.TempDir() }
	return marker
}

// With no session bus, as over SSH, reading or saving a secret says the store
// is down rather than having godbus start dbus-launch (SIERRA 1.23.0 m-8).
func TestTheSecretStoreStartsNoDbusLaunchWithoutASessionBus(t *testing.T) {
	marker := noBusWithFakeLaunch(t)
	if _, err := keyringStore.get(); !errors.Is(err, errNoSessionBus) {
		t.Errorf("summary key read without a bus: %v, want errNoSessionBus", err)
	}
	if err := keyringStore.set("sk-" + "ant-test0"); !errors.Is(err, errNoSessionBus) {
		t.Errorf("summary key saved without a bus: %v, want errNoSessionBus", err)
	}
	if err := keyringStore.delete(); !errors.Is(err, errNoSessionBus) {
		t.Errorf("summary key deleted without a bus: %v, want errNoSessionBus", err)
	}
	if _, err := viewerStore.get("id"); !errors.Is(err, errNoSessionBus) {
		t.Errorf("viewer secret read without a bus: %v, want errNoSessionBus", err)
	}
	if err := viewerStore.set("id", "x"); !errors.Is(err, errNoSessionBus) {
		t.Errorf("viewer secret saved without a bus: %v, want errNoSessionBus", err)
	}
	viewerStore.delete("id")
	if _, err := os.Stat(marker); err == nil {
		t.Error("dbus-launch was started")
	}
}

// With no session bus the tray is not started: systray would have godbus
// start dbus-launch. The stand-in for systray does what systray does first.
func TestTheTrayStartsNoDbusLaunchWithoutASessionBus(t *testing.T) {
	marker := noBusWithFakeLaunch(t)
	d := newTestDeck(t)
	browserOpens(t)
	saved := systrayRun
	t.Cleanup(func() { systrayRun = saved })
	started := false
	systrayRun = func(onReady, onExit func()) { started = true; _, _ = dbus.SessionBus() }
	failed := make(chan error, 1)
	failed <- errors.New("test server stopped")
	if code := runWithTray(d, failed, true); code != 1 {
		t.Errorf("returned %d, want 1 once the server stopped", code)
	}
	if started {
		t.Error("the tray was started with no session bus")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("dbus-launch was started")
	}
}

// A bus only at $XDG_RUNTIME_DIR/bus, which godbus doesn't look in, is handed
// to godbus in DBUS_SESSION_BUS_ADDRESS so the keyring and tray libraries
// connect to it rather than autolaunching dbus-launch.
func TestTheFoundSessionBusIsHandedToTheLibraries(t *testing.T) {
	marker := noBusWithFakeLaunch(t)
	dir := t.TempDir()
	sock := filepath.Join(dir, "bus")
	if err := os.WriteFile(sock, nil, 0o600); err != nil { // a stand-in; nothing connects
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", dir)
	want := "unix:path=" + dbus.EscapeBusAddressValue(sock)
	for _, before := range []string{"", "autolaunch:"} {
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", before)
		if sessionBusMissing() {
			t.Fatalf("bus at %s not found", sock)
		}
		if got := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); got != want {
			t.Errorf("DBUS_SESSION_BUS_ADDRESS = %q after %q, want %q", got, before, want)
		}
	}
	// What the libraries do first: with the address set, no autolaunch (the
	// connection itself fails, the stand-in socket being a plain file).
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	sessionBusMissing()
	_, _ = dbus.SessionBus()
	// An address already set stays.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/elsewhere")
	sessionBusMissing()
	if got := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); got != "unix:path=/elsewhere" {
		t.Errorf("a set address was replaced by %q", got)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("dbus-launch was started")
	}
}
