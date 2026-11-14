//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"syscall"
)

// hideGitWindow is for Windows only.
func hideGitWindow(*exec.Cmd) {}

// stopGitGently makes a deadline stop git with SIGTERM, not SIGKILL: git's own handlers then remove the lock files it
// holds (a killed git leaves them, and they stop the owner's own git too). WaitDelay is the hard stop after it.
func stopGitGently(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
}

// cpDataFlags open a small data file of a git folder for the checkpoint store in a way that never waits: a fifo with no
// writer opens at once (O_NONBLOCK), and a terminal never becomes the app's (O_NOCTTY). cpReadData then judges the file
// opened.
const cpDataFlags = os.O_RDONLY | syscall.O_NONBLOCK | syscall.O_NOCTTY

func cpOpenData(path string) (*os.File, error) { return os.OpenFile(path, cpDataFlags, 0) }
