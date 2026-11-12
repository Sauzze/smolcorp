//go:build !windows

package desktop

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// tmux runs here, so a hire can have a session of its own.
const hasTmuxHires = true

// The hire engine's launcher way is Windows's (hirelauncher.go builds everywhere, to be tested).
var launcherHires = false

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
	quoted := make([]string, len(command))
	for i, word := range command {
		quoted[i] = shellQuoted(word)
	}
	return []string{loginShell(), "-lic", "exec " + strings.Join(quoted, " ")}
}

// loginShell is your shell, as a terminal starts it: with no SHELL, the
// Mac's own (macShell), elsewhere bash where there is one.
func loginShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if runtime.GOOS == "darwin" {
		return macShell
	}
	if found, err := exec.LookPath("bash"); err == nil {
		return found
	}
	return "/bin/sh"
}

// How long the login shell has to say where it finds an agent.
const loginShellWait = 10 * time.Second

// foundMarker starts the line findInLoginShell reads, among whatever your
// shell's files print as it starts.
const foundMarker = "smolcorp-found:"

// findInLoginShell is where the login shell a hire opens in finds program,
// "" where it doesn't; known is false when that can't be told, as when your
// shell's files start something else in its place. It looks exactly as a
// hire does: your login shell, then exec, so what your shell's files set up
// is all that's kept. A variable, for the tests.
var findInLoginShell = func(program string) (found string, known bool) {
	ctx, cancel := context.WithTimeout(context.Background(), loginShellWait)
	defer cancel()
	args := inLoginShell([]string{"/bin/sh", "-c", `printf '\n%s%s\n' "$1" "$(command -v -- "$2")"`, "sh", foundMarker, program})
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = homeDir
	cmd.Env = hireEnvironment()
	cmd.WaitDelay = time.Second // what the shell's files started in the background mustn't hold it
	output, _ := cmd.Output()
	for _, line := range strings.Split(string(output), "\n") {
		if path, ok := strings.CutPrefix(line, foundMarker); ok {
			return strings.TrimSpace(path), true
		}
	}
	return "", false
}

// hireAgentMissing is why a hire of program can't start, for setup to say,
// or "" when it can, or when that can't be told. A login shell takes a
// moment to start: called without the lock.
func hireAgentMissing(program string) string {
	if program == "" {
		return ""
	}
	if found, known := findInLoginShell(program); !known || found != "" {
		return ""
	}
	return agentNotFoundMessage(program, loginShell(), whereAppFinds(program))
}

// prepareHireWindow: a window here is the terminal's own to open.
func prepareHireWindow(*exec.Cmd, string) {}
