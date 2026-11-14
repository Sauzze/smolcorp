//go:build !windows

package acp

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ownGroup starts the agent in its own process group, so killTree and
// stopGroup reach whatever it started too.
func ownGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killTree(p *os.Process) {
	syscall.Kill(-p.Pid, syscall.SIGKILL)
	p.Kill()
}

// stopGroup is for after the agent has been reaped: whatever it started is
// asked to stop (SIGTERM) and, if still there after grace, killed. A group id
// can't be handed to a new process while any member is alive, so this only
// ever reaches the agent's own leftovers; with none left the call finds no
// such group and does nothing.
func stopGroup(p *os.Process, grace time.Duration) {
	if syscall.Kill(-p.Pid, syscall.SIGTERM) != nil {
		return // nobody left
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		if syscall.Kill(-p.Pid, 0) != nil {
			return
		}
	}
	syscall.Kill(-p.Pid, syscall.SIGKILL)
}
