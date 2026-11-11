//go:build darwin

package desktop

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// processesWorkingIn is the processes working in folder (or a folder inside it), or given it with --add-dir, as
// T3 Code's agent is (T797a). A process's folder is kept behind libproc, which this engine can't reach without
// cgo, so under the Mac app it asks the app, whose own Let go reads it (Processes.holding, over the bridge's
// read-only "processes"). Without the app (a bare core), or when the app can't look, the answer is "can't
// look" (false): Let go keeps the fence and says why. Nothing is looked at here, and nothing signalled.
func processesWorkingIn(folder string) ([]int, bool) { return processesOnTheMac(folder, "") }

// processesHolding is the processes that work in folder as processesWorkingIn finds them, or whose command line
// names Claude Code session session (Take over and Hand back's check, t3takeover.go), asked of the Mac app too.
func processesHolding(folder, session string) ([]int, bool) {
	return processesOnTheMac(folder, session)
}

// processesOnTheMac asks the Mac app which processes work in folder or name session: their pids, and whether
// it could look at all.
func processesOnTheMac(folder, session string) ([]int, bool) {
	if host == nil {
		return nil, false
	}
	found, err := hostProcesses(host, folder, session)
	if err != nil {
		return nil, false
	}
	var pids []int
	for _, process := range found {
		// The app never sends one; an answer that does isn't one to trust as "nothing holds it" (RM5c N4).
		if process.PID <= 0 {
			return nil, false
		}
		pids = append(pids, process.PID)
	}
	return pids, true
}

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
