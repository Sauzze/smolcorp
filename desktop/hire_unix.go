//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"strings"
)

// tmux runs here, so a hire can have a session of its own.
const hasTmuxHires = true

// hireOpeners are the terminals a hire's window can open in, the first
// installed being the one used until another is chosen. Each runs the
// command as its own arguments, never through a line it would read.
var hireOpeners = []hireOpener{
	{id: "gnome-terminal", name: "GNOME Terminal", program: "gnome-terminal", args: func(folder string, command []string) []string {
		return append([]string{"gnome-terminal", "--working-directory=" + folder, "--"}, command...)
	}},
	{id: "konsole", name: "Konsole", program: "konsole", args: func(folder string, command []string) []string {
		return append([]string{"konsole", "--workdir", folder, "-e"}, command...)
	}},
	{id: "xfce4-terminal", name: "Xfce Terminal", program: "xfce4-terminal", args: func(folder string, command []string) []string {
		return append([]string{"xfce4-terminal", "--working-directory=" + folder, "-x"}, command...)
	}},
	{id: "kitty", name: "kitty", program: "kitty", args: func(folder string, command []string) []string {
		return append([]string{"kitty", "--directory", folder, "--"}, command...)
	}},
	{id: "wezterm", name: "WezTerm", program: "wezterm", args: func(folder string, command []string) []string {
		return append([]string{"wezterm", "start", "--cwd", folder, "--"}, command...)
	}},
	{id: "alacritty", name: "Alacritty", program: "alacritty", args: func(folder string, command []string) []string {
		return append([]string{"alacritty", "--working-directory", folder, "-e"}, command...)
	}},
	{id: "foot", name: "foot", program: "foot", args: func(folder string, command []string) []string {
		return append([]string{"foot", "--working-directory=" + folder}, command...)
	}},
	{id: "xterm", name: "XTerm", program: "xterm", args: func(_ string, command []string) []string {
		return append([]string{"xterm", "-e"}, command...)
	}},
}

// inLoginShell runs command through your login shell, as the Mac app's hires
// do, so it finds what your PATH finds in a terminal: an app opened from the
// menu may not have the folder npm puts the agents in. Each word is quoted,
// so it's only ever itself.
func inLoginShell(command []string) []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if found, err := exec.LookPath("bash"); err == nil {
			shell = found
		} else {
			shell = "/bin/sh"
		}
	}
	quoted := make([]string, len(command))
	for i, word := range command {
		quoted[i] = shellQuoted(word)
	}
	return []string{shell, "-lic", "exec " + strings.Join(quoted, " ")}
}

// shellQuoted is text as one word to a shell, whatever it holds.
func shellQuoted(text string) string { return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'" }

// prepareHireWindow: a window here is the terminal's own to open.
func prepareHireWindow(*exec.Cmd, string) {}
