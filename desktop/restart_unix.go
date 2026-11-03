//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"syscall"
)

// waitForChild waits for the child pid to end, so it isn't left a zombie.
func waitForChild(pid int) {
	var status syscall.WaitStatus
	for {
		if _, err := syscall.Wait4(pid, &status, 0, nil); err != syscall.EINTR {
			return
		}
	}
}

// canReplaceProcess: a program can become another here, keeping its process.
const canReplaceProcess = true

// replaceProcess makes this process program, with args and env added to this
// one's environment, and only comes back if it can't: a variable, for the
// tests. Files and sockets are closed on the way, the instance lock and the
// port with them, as Go opens them close-on-exec.
var replaceProcess = func(program string, args, env []string) error {
	return syscall.Exec(program, append([]string{program}, args...), append(os.Environ(), env...))
}

// detach starts cmd in a session of its own, so closing the terminal that
// ran smolcorp setup doesn't take the app with it.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
