//go:build windows

package desktop

// On Windows, what's in front of a pane can't be looked at, so its agent
// isn't stopped: firing leaves the pane open.
func stopGroup(group int) bool { return false }
