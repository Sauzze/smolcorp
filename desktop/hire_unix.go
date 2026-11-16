//go:build !windows

package desktop

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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
	// Nothing of Smolcorp's in it: the lookup's own environment (opencode.go).
	cmd := lookupCommand(ctx, args[0], args[1:]...)
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

// windowScriptLines are a window's few lines: remove the script, point the
// `smolcorp` command at a test copy (its port and folder; the terminal's
// environment is its own, not this program's), then run command, each word
// quoted so it's only ever itself.
func windowScriptLines(command []string, testPort int, testSupport string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("nothing to run")
	}
	quoted := make([]string, len(command))
	for i, word := range command {
		quoted[i] = shellQuoted(word)
	}
	lines := "#!/bin/sh\nrm -f -- \"$0\"\n"
	if testPort != 0 {
		lines += "export SMOLCORP_PORT=" + shellQuoted(strconv.Itoa(testPort)) + "\n" +
			"export SMOLCORP_TEST_SUPPORT_DIR=" + shellQuoted(testSupport) + "\n"
	}
	return lines + "exec " + strings.Join(quoted, " ") + "\n", nil
}

// writeWindowScript writes a window's script into folder (made 0700 if need
// be), only yours to read or run, as the bridge's open requires.
func writeWindowScript(folder string, command []string, testPort int, testSupport string) (string, error) {
	lines, err := windowScriptLines(command, testPort, testSupport)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(folder, "window-*.sh")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := file.Chmod(0o700); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	if _, err := file.WriteString(lines); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// plainHireTmux is a plain hire's own tmux server (SA-M10), on a socket named for id in tmux's own folder, as a desk's
// outside the sandbox is (newHireTmux): never the owner's default server, whose start file, other sessions and clients
// are the owner's (RT126SG2R2 N3). Its start runs with the guard off for that call alone and reads no start file; every
// other call is a one-shot client's (tmuxClient), with no pid to check on that server. nil when there's no tmux or no
// safe folder for its socket. A variable, for the tests, which give it a stand-in runner.
var plainHireTmux = func(id string) *hireTmux { return newHireTmux(id, testCopyPort(), nil) }

// plainHireTmuxID is a new plain hire's server's id: "plain-" and twelve letters or digits, which no engine hire's id
// (twelve alone) can be.
func plainHireTmuxID() string { return "plain-" + strings.ToLower(newID())[:12] }

// openHireInTmux is openHire in tmux: command in a session on its own server, shown in a window of terminal when
// there is one.
func openHireInTmux(command []string, folder string, terminal hireOpener, hasTerminal bool) error {
	id := plainHireTmuxID()
	tmux := plainHireTmux(id)
	if tmux == nil {
		return errUnsupported
	}
	_, socket, pid, err := tmux.start(folder, command)
	if err != nil {
		return err
	}
	// The core's own record of the server it made, as its start said it: what the Mac app is told with a focus or close
	// on its pane (deskOfPane), as it has no registry row.
	notePlainTmux(socket, plainTmux{id: id, pane: pid, paneStarted: paneStartedAt(pid), folder: folder})
	if !hasTerminal {
		// Over SSH, say: the log names its socket, for `tmux -S <socket> attach`.
		log.Printf("tmux: a hire's session is on its own server, %s.", socket)
		return nil
	}
	// Its window runs `smolcorp attach` with exec, on the socket of its own server (SA-M8).
	if err := openWindow(terminal, folder, plainAttachLine(commandLineProgram(), hireSession, socket)); err != nil {
		tmux.endServer() // its own server: nothing else is on it
		forgetPlainTmux(socket)
		return err
	}
	return nil
}
