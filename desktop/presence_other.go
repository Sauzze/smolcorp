//go:build !linux && !windows

package desktop

import "time"

// presence can't be told here.
func presence() (idle time.Duration, locked, ok bool) { return 0, false, false }
