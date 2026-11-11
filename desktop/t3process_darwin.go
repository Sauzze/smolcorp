//go:build darwin

package desktop

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// processesWorkingIn can't look on a Mac: a process's working folder is kept behind libproc, which this
// engine can't reach without cgo, so the answer is "can't look" (false), and Let go keeps the fence and says
// why. The Mac app reads it (T3Desk.swift's Processes.working); the Go core on a Mac would ask it through the
// bridge (M1). Nothing is looked at meanwhile (RM5a N4).
func processesWorkingIn(string) ([]int, bool) { return nil, false }

// processName is a process's name as the kernel keeps it (zsh, node…), or "".
func processName(pid int) string {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ""
	}
	name := p.Proc.P_comm[:]
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	return string(name)
}
