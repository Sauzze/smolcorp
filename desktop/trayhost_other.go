//go:build !linux

package desktop

// hasTrayHost: Windows always has a notification area for the tray's icon.
var hasTrayHost = func() bool { return true }

// sessionBusMissing: Windows' Credential Manager and notification area need no session bus.
func sessionBusMissing() bool { return false }
