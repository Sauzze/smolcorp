package desktop

import (
	"os"
	"time"
)

// presence is how long this session's keyboard and mouse have gone unused,
// and whether its screen is locked, as the desktop tells logind (GNOME and
// KDE do): ok is false where it can't be told.
func presence() (idle time.Duration, locked, ok bool) {
	session := os.Getenv("XDG_SESSION_ID")
	if session == "" {
		session = "auto"
	}
	cmd, cancel := terminalCommand("loginctl", "show-session", session, "-p", "LockedHint", "-p", "IdleHint", "-p", "IdleSinceHint")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return 0, false, false
	}
	return parseLogindPresence(string(output), time.Now())
}
