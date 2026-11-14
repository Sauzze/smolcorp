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
// The checkpoint store writes its refs itself, never through a git, so no git of its own ever holds a ref's lock there.
func stopGitGently(*exec.Cmd) {}

// cpDataFlags are a plain open on Windows, which has no fifos to wait on; cpReadData then judges the file opened.
const cpDataFlags = os.O_RDONLY

func cpOpenData(path string) (*os.File, error) { return os.OpenFile(path, cpDataFlags, 0) }
