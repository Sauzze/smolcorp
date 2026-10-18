//go:build !windows

package desktop

import (
	"errors"
	"syscall"
	"time"
)

// paneStopGrace is how long a job in a pane is given to end when asked, then
// when made to, as the Mac app's Terminal.stopGrace.
var paneStopGrace = []struct {
	signal syscall.Signal
	wait   time.Duration
}{{syscall.SIGTERM, 3 * time.Second}, {syscall.SIGKILL, time.Second}}

// stopGroup stops the process group, and says whether none of it is left.
func stopGroup(group int) bool {
	if group <= 1 {
		return false
	}
	gone := func() bool { return errors.Is(syscall.Kill(-group, 0), syscall.ESRCH) }
	for _, step := range paneStopGrace {
		_ = syscall.Kill(-group, step.signal)
		for deadline := time.Now().Add(step.wait); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
			if gone() {
				return true
			}
		}
	}
	return gone()
}

// processGroup is the process group pid is in.
func processGroup(pid int) (int, error) {
	return syscall.Getpgid(pid)
}
