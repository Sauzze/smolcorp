//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"syscall"
)

// ownGroup starts a summary's process in a process group of its own.
func ownGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// useCommandLine: only Windows gives cmd.exe a command line of its own.
func useCommandLine(*exec.Cmd, string) {}

// processGroup is a summary's process and all it started: its process group,
// whose id stays its own while any process in it is left.
type processGroup struct{ id int }

func joinGroup(cmd *exec.Cmd) processGroup { return processGroup{cmd.Process.Pid} }

// groupOf is the group of a process started as its group's first, as a pseudo-terminal's (signinrun.go).
func groupOf(process *os.Process) processGroup { return processGroup{process.Pid} }

func (g processGroup) kill() error {
	if g.id <= 0 {
		return nil
	}
	return syscall.Kill(-g.id, syscall.SIGKILL)
}

// close ends what it left running once it has exited.
func (g processGroup) close() { _ = g.kill() }
