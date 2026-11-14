package desktop

import (
	"os"
	"os/exec"
	"syscall"
)

// hideGitWindow keeps a git run from flashing a console window on Windows.
func hideGitWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// stopGitGently is for the others: Windows can't ask a process to stop, so a deadline kills git as it always did.
// What keeps the owner's repo clear of a lock file there is that every ref write has a short deadline of its own.
func stopGitGently(*exec.Cmd) {}

// cpOpenData is a plain open on Windows, which has no fifos to wait on; cpReadData then judges the file opened.
func cpOpenData(path string) (*os.File, error) { return os.Open(path) }
