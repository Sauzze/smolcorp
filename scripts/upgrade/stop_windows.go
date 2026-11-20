package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// On Windows, `smolcorp quit` never stops a copy: it only says to quit from the tray icon (desktop/quit.go,
// quitByTray), in 1.24.1, 1.24.2 and 1.26 alike, and a headless serve has no tray. So serve starts in a process
// group of its own, and is stopped as Ctrl+Break in its console would: Go delivers CTRL_BREAK_EVENT as os.Interrupt,
// which serve takes as a clean stop in each of those versions (desktop/app.go, stopPortableOnSignalNow: the portable
// office stopped, the pid record removed, and in 1.26 the hires waited for and the GM's token kept), the same handler
// `smolcorp quit`'s SIGTERM reaches on Linux. The event goes to that group only, not to this program.

// inItsOwnGroup starts serve in a process group of its own, whose id is its pid.
func inItsOwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// askToQuit sends Ctrl+Break to serve's group. How it asked; nothing is printed.
func (o *office) askToQuit() (string, []byte, error) {
	return "Ctrl+Break to its process group", nil, windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(o.cmd.Process.Pid))
}
