//go:build windows

package desktop

import "os/exec"

// detach: a program started on Windows outlives the console that started it.
func detach(*exec.Cmd) {}
