package desktop

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// On a Mac the hook names a session's tab by the Mac app's rules, as its
// report.sh does, so a session in Terminal, iTerm2 or Ghostty gets its desk
// and its controls under the Go engine as under the Swift one. The tab comes
// from the agent's tty: hooks have no terminal of their own, so it walks up
// the process tree to the first ancestor that has one. With a tty:
//   - tmux first, as it runs inside any of the others, whose variables it
//     then carries, stale: tmux:%3@<socket>;
//   - then WezTerm's and kitty's panes, as on Windows and Linux;
//   - then iTerm2's session (iterm:<its unique id>), Ghostty (ghostty:<tty>),
//     and Terminal (tty:<tty>), found by TERM_PROGRAM, or, where an agent has
//     dropped that variable, by Terminal.app further up the tree;
//   - else other:<tty>, a terminal the app can only watch.
//
// Each variable is taken only in its own shape: one carrying anything else, as
// a line break that would add a header of its own, gives other:<tty>.
//
// With no tty, the hook runs outside the session's process tree, as Codex's
// and OpenCode's do, and the app finds the tab by the session's folder; a
// claude run that says it is headless is named by its session ID; any other
// sends nothing, as it would land on the tab of an interactive session in its
// folder. A session the launcher started (`smolcorp run`) is the launcher's,
// first, as everywhere.

// macProcess is what the hook reads of a process: its parent, its tty as ps
// names it (ttys003, or ?? for none), and its program's path.
type macProcess func(pid int) (parent int, tty, program string, ok bool)

// psProcess reads a process with ps, as report.sh does.
func psProcess(pid int) (parent int, tty, program string, ok bool) {
	out, err := exec.Command("/bin/ps", "-o", "ppid=,tty=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", "", false
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return 0, "", "", false
	}
	parent, err = strconv.Atoi(fields[0])
	// The program's path may hold spaces: everything after the tty.
	_, rest, _ := strings.Cut(strings.TrimSpace(string(out)), fields[1])
	return parent, fields[1], strings.TrimSpace(rest), err == nil
}

var (
	macTTY          = regexp.MustCompile(`^tty[A-Za-z0-9]+$`)
	macTmuxPane     = regexp.MustCompile(`^%[0-9]+$`)
	macPaneNumber   = regexp.MustCompile(`^[0-9]+$`)
	macITermSession = regexp.MustCompile(`^[0-9A-Fa-f-]+$`)
)

// headerSafe is whether text has no control character, which would end a header line.
func headerSafe(text string) bool { return strings.IndexFunc(text, unicode.IsControl) < 0 }

// macHookTab is the session's tab and tty by the Mac app's rules, from env and
// the processes above start; send is false when nothing should be sent.
func macHookTab(agent Agent, event []byte, env func(string) string, process macProcess, start int) (tab, tty string, send bool) {
	for _, name := range []string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION"} {
		// Not for Codex: its shared hook service keeps a stale launcher variable (see below).
		if id := env(name); id != "" && agent != Codex {
			return "launcher:" + id, "", true
		}
	}
	pid, steps := start, 0
	for ; pid > 1 && steps < 64; steps++ {
		parent, t, _, ok := process(pid)
		if !ok {
			break
		}
		if macTTY.MatchString(t) {
			tty = "/dev/" + t
			break
		}
		pid = parent
	}
	// Codex runs its hooks in a shared background service, which keeps the
	// variables of whichever terminal started it: the app finds its tab by folder.
	if agent == Codex {
		return "", tty, true
	}
	if tty == "" {
		// As headlessRequested, from env.
		if agent == Claude && env("SMOLCORP_HEADLESS") == "1" {
			if id := sessionIDOf(event); headlessID.MatchString(id) {
				return "headless:" + id, "", true
			}
			return "", "", false
		}
		return "", "", agent == OpenCode
	}
	other := "other:" + tty
	if pane := env("TMUX_PANE"); pane != "" {
		socket, _, _ := strings.Cut(env("TMUX"), ",")
		if macTmuxPane.MatchString(pane) && socket != "" && headerSafe(socket) {
			return "tmux:" + pane + "@" + socket, tty, true
		}
		return other, tty, true
	}
	if pane := env("WEZTERM_PANE"); pane != "" {
		socket := env("WEZTERM_UNIX_SOCKET")
		if macPaneNumber.MatchString(pane) && socket != "" && headerSafe(socket) {
			return "wezterm:" + pane + "@" + socket, tty, true
		}
		return other, tty, true
	}
	if window, socket := env("KITTY_WINDOW_ID"), env("KITTY_LISTEN_ON"); window != "" && socket != "" {
		if macPaneNumber.MatchString(window) && headerSafe(socket) {
			return "kitty:" + window + "@" + socket, tty, true
		}
		return other, tty, true
	}
	if id := env("ITERM_SESSION_ID"); id != "" {
		// w0t1p0:<the session's unique ID>
		if _, session, found := strings.Cut(id, ":"); found {
			id = session
		}
		if macITermSession.MatchString(id) {
			return "iterm:" + id, tty, true
		}
		return other, tty, true
	}
	switch program := env("TERM_PROGRAM"); {
	case program == "ghostty":
		return "ghostty:" + tty, tty, true
	case program == "Apple_Terminal":
		return "tty:" + tty, tty, true
	case program != "":
		return other, tty, true
	}
	// No TERM_PROGRAM: Terminal itself, further up the tree from the tty's process.
	for ; pid > 1 && steps < 64; steps++ {
		parent, _, program, ok := process(pid)
		if !ok {
			break
		}
		if strings.Contains(program, "/Terminal.app/") {
			return "tty:" + tty, tty, true
		}
		pid = parent
	}
	return other, tty, true
}

// macParent is a process's parent and its program's name, as nearestAgent
// walks up for a headless run's agent, from ps.
func macParent(process macProcess) func(int) (int, string, bool) {
	return func(pid int) (int, string, bool) {
		parent, _, program, ok := process(pid)
		return parent, filepath.Base(program), ok
	}
}
