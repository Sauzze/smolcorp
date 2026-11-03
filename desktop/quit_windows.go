//go:build windows

package desktop

import "errors"

// On Windows Smolcorp is quit from its tray menu.
func askToEnd(pid int) error { return errors.New("quit it from its tray menu") }

var isSmolcorpProcess = func(pid int) bool { return false }

// quit only says to use the tray here, so the answer doesn't matter.
var thisHomeRunning = func() (bool, error) { return true, nil }
