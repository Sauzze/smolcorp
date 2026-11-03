//go:build windows

package desktop

import (
	"errors"
	"os/exec"
)

// waitForChild: Windows never replaces a process, so no child is handed on.
func waitForChild(int) {}

// canReplaceProcess: Windows can't make a process another program: a restart
// starts a new copy.
const canReplaceProcess = false

var replaceProcess = func(string, []string, []string) error {
	return errors.New("Windows can't replace a process")
}

// detach: a program started on Windows outlives the console that started it.
func detach(*exec.Cmd) {}
