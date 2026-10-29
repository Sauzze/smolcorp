package desktop

import (
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// No tmux on Windows.
const hasTmuxHires = false

// hireOpeners are where a hire's window can open: Windows Terminal, where
// it's installed, then a console window of its own, which every Windows has.
var hireOpeners = []hireOpener{
	{id: "wt", name: "Windows Terminal", program: "wt.exe", args: func(folder string, command []string) []string {
		// Windows Terminal reads a ; as the start of another tab's command.
		escaped := func(word string) string { return strings.ReplaceAll(word, ";", `\;`) }
		args := []string{"wt.exe", "-d", escaped(folder)}
		for _, word := range command {
			args = append(args, escaped(word))
		}
		return args
	}},
	{id: "console", name: "a console window", args: func(_ string, command []string) []string { return slices.Clone(command) }},
}

// inLoginShell: Windows finds the agents through its own PATH.
func inLoginShell(command []string) []string { return command }

// prepareHireWindow gives a console hire a window of its own, rather than
// this app's, which may have none.
func prepareHireWindow(cmd *exec.Cmd, opener string) {
	if opener == "console" {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	}
}
