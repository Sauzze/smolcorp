//go:build !linux && !windows

package desktop

import "time"

// presence can't be told here: on a Mac, the Mac app tells it instead
// (macPresenceOf in hostshell.go), and a deck made without the app is a test's.
func presence() (idle time.Duration, locked, ok bool) { return 0, false, false }
