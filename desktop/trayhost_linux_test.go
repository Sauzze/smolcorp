package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/godbus/dbus/v5"
)

// fakeConnect swaps connectBus for one that records its addresses and fails.
func fakeConnect(t *testing.T) *[]string {
	t.Helper()
	saved := connectBus
	t.Cleanup(func() { connectBus = saved })
	var addrs []string
	connectBus = func(address string) (*dbus.Conn, error) {
		addrs = append(addrs, address)
		return nil, errors.New("no bus in tests")
	}
	return &addrs
}

func TestNoSessionBusMeansNoTrayAndNoConnectionAttempt(t *testing.T) {
	for _, addr := range []string{"", "autolaunch:"} {
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
		addrs := fakeConnect(t)
		if hasTrayHost() {
			t.Errorf("address %q: want no tray", addr)
		}
		if len(*addrs) != 0 {
			t.Errorf("address %q: connect tried with %v, want no attempt (it would autolaunch)", addr, *addrs)
		}
	}
}

func TestASessionBusAddressIsTheOnlyOneTried(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/test-bus")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	addrs := fakeConnect(t)
	if hasTrayHost() {
		t.Error("want no tray when the connection fails")
	}
	if len(*addrs) != 1 || (*addrs)[0] != "unix:path=/nonexistent/test-bus" {
		t.Errorf("connect tried %v, want exactly the address set", *addrs)
	}
}

func TestTheRuntimeDirBusSocketIsUsedWhenNoAddressIsSet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bus"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", dir)
	addrs := fakeConnect(t)
	hasTrayHost()
	want := "unix:path=" + dbus.EscapeBusAddressValue(filepath.Join(dir, "bus"))
	if len(*addrs) != 1 || (*addrs)[0] != want {
		t.Errorf("connect tried %v, want [%s]", *addrs, want)
	}
}

func TestTheRuntimeDirDbusSessionFileIsUsedWhenThereIsNoBusSocket(t *testing.T) {
	dir := t.TempDir()
	file := "DBUS_SESSION_BUS_ADDRESS=unix:abstract=/tmp/dbus-test0\n"
	if err := os.WriteFile(filepath.Join(dir, "dbus-session"), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", dir)
	addrs := fakeConnect(t)
	hasTrayHost()
	if len(*addrs) != 1 || (*addrs)[0] != "unix:abstract=/tmp/dbus-test0" {
		t.Errorf("connect tried %v, want the address in the dbus-session file", *addrs)
	}
}

func TestADbusSessionFileSayingAutolaunchOrNothingUsefulIsNoBus(t *testing.T) {
	for _, file := range []string{"DBUS_SESSION_BUS_ADDRESS=autolaunch:\n", "DBUS_SESSION_BUS_ADDRESS=\n", "junk\n"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "dbus-session"), []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
		t.Setenv("XDG_RUNTIME_DIR", dir)
		addrs := fakeConnect(t)
		if hasTrayHost() || len(*addrs) != 0 {
			t.Errorf("file %q: want no tray and no connect, tried %v", file, *addrs)
		}
	}
}

// fakeDbusLaunch puts a dbus-launch first on PATH that only writes a marker
// file and fails, and returns the marker's path. Nothing real is started.
func fakeDbusLaunch(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "launched")
	script := "#!/bin/sh\necho launched > '" + marker + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "dbus-launch"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func TestNoSessionBusNeverRunsDbusLaunch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs a shell script")
	}
	marker := fakeDbusLaunch(t)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	// The real connectBus: with the old code this is where godbus autolaunched.
	if hasTrayHost() {
		t.Error("want no tray")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("dbus-launch was run, want no autolaunch")
	}
}

func TestWithoutXDGRuntimeDirTheRunUserFolderIsLookedIn(t *testing.T) {
	dir := t.TempDir() // stands for /run/user/<uid>; never the real one
	if err := os.WriteFile(filepath.Join(dir, "bus"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	saved := runUserDir
	t.Cleanup(func() { runUserDir = saved })
	runUserDir = func() string { return dir }
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	addrs := fakeConnect(t)
	hasTrayHost()
	want := "unix:path=" + dbus.EscapeBusAddressValue(filepath.Join(dir, "bus"))
	if len(*addrs) != 1 || (*addrs)[0] != want {
		t.Errorf("connect tried %v, want [%s]", *addrs, want)
	}
}

func TestNoSessionBusMeansNoTrayHost(t *testing.T) {
	// A bus address with nothing listening: never the real session bus.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	if hasTrayHost() {
		t.Error("a tray host found on a session bus that isn't there")
	}
}
