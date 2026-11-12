package desktop

import (
	"os/exec"
	"syscall"
)

// hideGitWindow keeps a git run from flashing a console window on Windows.
func hideGitWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
