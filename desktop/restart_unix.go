//go:build !windows

package desktop

import (
	"os/exec"
	"syscall"
)

// detach starts cmd in a session of its own, so closing the terminal that
// ran smolcorp setup doesn't take the app with it.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
