package desktop

import (
	"strconv"
	"strings"
	"time"
)

// parseLogindPresence reads loginctl's answer: IdleSinceHint is in
// microseconds since 1970.
func parseLogindPresence(output string, now time.Time) (idle time.Duration, locked, ok bool) {
	hints := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		if name, value, found := strings.Cut(strings.TrimSpace(line), "="); found {
			hints[name] = value
		}
	}
	if hints["LockedHint"] == "" && hints["IdleHint"] == "" {
		return 0, false, false
	}
	if hints["IdleHint"] == "yes" {
		if since, err := strconv.ParseInt(hints["IdleSinceHint"], 10, 64); err == nil && since > 0 {
			idle = max(0, now.Sub(time.UnixMicro(since)))
		}
	}
	return idle, hints["LockedHint"] == "yes", true
}
