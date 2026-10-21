//go:build !windows

package desktop

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// paneStopGrace is how long a job in a pane is given to end when asked, then
// when made to, as the Mac app's Terminal.stopGrace.
var paneStopGrace = []struct {
	signal syscall.Signal
	wait   time.Duration
}{{syscall.SIGTERM, 3 * time.Second}, {syscall.SIGKILL, time.Second}}

// processGone is whether no process has the pid.
func processGone(pid int) bool {
	return pid > 0 && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

// processParent is a process's parent and the name of its program, where
// /proc shows them (Linux).
func processParent(pid int) (parent int, name string, ok bool) {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, "", false
	}
	// pid (comm) state ppid ...: comm may hold spaces, so it is read up to
	// the last parenthesis.
	start, end := bytes.IndexByte(stat, '('), bytes.LastIndexByte(stat, ')')
	if start < 0 || end < start {
		return 0, "", false
	}
	rest := strings.Fields(string(stat[end+1:]))
	if len(rest) < 2 {
		return 0, "", false
	}
	parent, err = strconv.Atoi(rest[1])
	return parent, string(stat[start+1 : end]), err == nil
}

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
