package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// runUserDir is the runtime folder to look in when XDG_RUNTIME_DIR is unset.
// A variable, for the tests.
var runUserDir = func() string { return fmt.Sprintf("/run/user/%d", os.Getuid()) }

// sessionBusAddress is where the session bus is, or "" when there is none:
// DBUS_SESSION_BUS_ADDRESS, else the $XDG_RUNTIME_DIR/bus socket, else the
// address in $XDG_RUNTIME_DIR/dbus-session (as godbus looked). Never
// "autolaunch:", which would have godbus start dbus-launch, whose daemon
// outlives us and leaves sockets in /tmp (SSH and headless sessions).
func sessionBusAddress() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = runUserDir() // as godbus did when it is unset
	}
	if addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); addr != "" && addr != "autolaunch:" {
		return addr
	}
	if dir != "" {
		sock := filepath.Join(dir, "bus")
		if _, err := os.Stat(sock); err == nil {
			return "unix:path=" + dbus.EscapeBusAddressValue(sock)
		}
		// A text file holding DBUS_SESSION_BUS_ADDRESS=<address> of a bus already running.
		if b, err := os.ReadFile(filepath.Join(dir, "dbus-session")); err == nil {
			if addr, ok := strings.CutPrefix(string(b), "DBUS_SESSION_BUS_ADDRESS="); ok {
				if addr = strings.TrimRight(addr, "\r\n"); addr != "autolaunch:" {
					return addr
				}
			}
		}
	}
	return ""
}

// connectBus connects to the bus at address. A variable, for the tests.
var connectBus = func(address string) (*dbus.Conn, error) { return dbus.Connect(address) }

// hasTrayHost is whether this desktop has somewhere for the tray's icon: a
// StatusNotifierWatcher on the session bus, which KDE, Xfce's panel and most
// others run, and GNOME only with its AppIndicator extension. Without one the
// icon never shows, and nor does its menu. A variable, for the tests.
var hasTrayHost = func() bool {
	address := sessionBusAddress()
	if address == "" {
		return false
	}
	// A connection of its own: the tray's shared one must stay open.
	conn, err := connectBus(address)
	if err != nil {
		return false
	}
	defer conn.Close()
	var owned bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&owned)
	return err == nil && owned
}

// sessionBusMissing is whether there is no session bus to reach: godbus would
// then autolaunch dbus-launch, whose daemon outlives us (SIERRA 1.23.0 m-8).
// When we found the bus ourselves, say so in DBUS_SESSION_BUS_ADDRESS (as
// godbus does): it looks for the socket only in /run/user/<uid>, so with the
// bus elsewhere (a different $XDG_RUNTIME_DIR) it would autolaunch anyway.
// Every keyring and tray call is guarded by this, so it is set before them.
func sessionBusMissing() bool {
	addr := sessionBusAddress()
	if addr == "" {
		return true
	}
	if cur := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); cur == "" || cur == "autolaunch:" {
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
	}
	return false
}
