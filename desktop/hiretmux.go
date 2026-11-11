//go:build !windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// A hire's tmux session on a server of its own, as the Mac app's HireTmux:
// tmux -S <tmux's own folder>/smolcorp-<hire id> (a test copy's:
// smolcorp-<port>-<hire id>), the socket `tmux -L` would make, one session called "hire", started detached in
// the hire's folder. Every call names our socket with -S, so the user's own
// tmux server, the default socket and any socket not made by us are never
// reached. A session is ended only by its own name on that socket; nothing is
// ever killed by name or by search.
//
// Nothing calls this yet: the engine's wiring comes later.
//
// All of it goes through run, which takes tmux's arguments and says what tmux
// printed and its exit status (ran false when tmux couldn't be run at all),
// so tests give a stand-in.

const (
	// hireSession is the one session on a hire's server.
	hireSession = "hire"
	// hirePane is the exact pane target: "=" matches the name exactly, ":" makes it the session's active pane.
	hirePane = "=" + hireSession + ":"
	// A unix socket's path is cut off a little over 100 bytes.
	maxSocketPath = 100
	// How long one tmux call may take.
	tmuxCallWait = 10 * time.Second
)

type tmuxRunner func(args []string) (output string, status int, ran bool)

type hireTmux struct {
	socket string
	run    tmuxRunner
}

// hireSocketName is a hire's socket's file name, or "" for an id that
// couldn't be part of one (only a-z, 0-9 and - are). testPort is 0 outside a test copy.
func hireSocketName(hireID string, testPort int) string {
	if !plainHireID(hireID) {
		return ""
	}
	return hireSocketPrefix + testPortPart(testPort) + hireID
}

func testPortPart(testPort int) string {
	if testPort > 0 {
		return strconv.Itoa(testPort) + "-"
	}
	return ""
}

// hireSocketFolder is where a new hire's socket goes: tmux's own folder, as the Mac's HireTmux (`tmux -L`)
// puts it, so both engines' hires are in one place and a long support folder (a long user name, a test
// copy deep in a worktree) never makes the path too long for a socket. Tests point it elsewhere.
var hireSocketFolder = tmuxSocketFolder

// tmuxSocketFolder is tmux's own folder for this user: $TMUX_TMPDIR (or /tmp)/tmux-<uid>, as tmux works it
// out for -L. The hire's tmux is given the same environment (hireEnvironment), so it would pick the same.
func tmuxSocketFolder() string {
	base := os.Getenv("TMUX_TMPDIR")
	if !filepath.IsAbs(base) { // unset, or relative (it would mean the core's own folder)
		base = "/tmp"
	}
	return filepath.Join(base, "tmux-"+strconv.Itoa(os.Getuid()))
}

// hireSocketFolders are where a hire's socket may be: the folder new ones go in, then the support folder,
// where the Go core put them before 1.25 (a hire made then is still its own, to fire or clean up).
func hireSocketFolders() []string {
	return []string{filepath.Clean(hireSocketFolder()), filepath.Clean(supportDir)}
}

// ownSocketFolder makes folder if it isn't there (0700, as tmux makes it) and says whether it is safe to put
// a socket in: a real folder, not a link, this user's, that nobody else can open (tmux's own check).
func ownSocketFolder(folder string) bool {
	_ = os.Mkdir(folder, 0o700)
	info, err := os.Lstat(folder)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid()
}

// newHireTmux is the hire's server, its socket in hireSocketFolder, with
// the real tmux when run is nil; nil when the id can't make a socket's name,
// the folder isn't safe, the path is too long for one, or there is no tmux.
func newHireTmux(hireID string, testPort int, run tmuxRunner) *hireTmux {
	name := hireSocketName(hireID, testPort)
	folder := hireSocketFolder()
	if name == "" || !ownSocketFolder(folder) {
		return nil
	}
	return hireTmuxAt(filepath.Join(folder, name), testPort, run)
}

// hireTmuxAt is the server on socket, when it is one of ours: in a hire
// socket folder, named like a hire's (a test copy owns only its own port's), never the default.
func hireTmuxAt(socket string, testPort int, run tmuxRunner) *hireTmux {
	if !ownsHireSocket(socket, testPort) || len(socket) > maxSocketPath {
		return nil
	}
	if run == nil {
		path := tmuxPath()
		if path == "" {
			return nil
		}
		run = realTmux(path)
	}
	return &hireTmux{socket: filepath.Clean(socket), run: run}
}

// ownsHireSocket says whether socket is one of ours: a hire's, in a hire socket
// folder, with a name of only a-z, 0-9 and -, not the user's default.
func ownsHireSocket(socket string, testPort int) bool {
	socket = filepath.Clean(socket)
	if socket == "" || !inHireSocketFolder(socket) {
		return false
	}
	name := filepath.Base(socket)
	rest, ok := strings.CutPrefix(name, hireSocketPrefix+testPortPart(testPort))
	if !ok || hireSocketName(rest, 0) == "" {
		return false
	}
	return true
}

// realTmux runs tmux at path, in the environment a hire gets (hireEnvironment:
// no TMUX, no session variables, no GM token), so the hire's server and pane
// never inherit where Smolcorp was started or the GM's token.
func realTmux(path string) tmuxRunner {
	return func(args []string) (string, int, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), tmuxCallWait)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Env = hireEnvironment()
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimRight(string(out), "\n"), 0, true
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return strings.TrimRight(string(out), "\n"), exit.ExitCode(), true
		}
		return "", -1, false
	}
}

// command is tmux's arguments for args on our server.
func (h *hireTmux) command(args ...string) []string { return append([]string{"-S", h.socket}, args...) }

func (h *hireTmux) ok(args ...string) bool {
	_, status, ran := h.run(h.command(args...))
	return ran && status == 0
}

// start starts the session in folder, command run by shell as a login shell.
// It gives the pane and the socket's path (as the deck addresses a tab:
// tmux:<pane>@<socket>), or false if tmux didn't start it.
func (h *hireTmux) start(folder, shell, command string) (pane, socket string, ok bool) {
	out, status, ran := h.run(h.command("new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}",
		"-s", hireSession, "-c", folder, shell, "-lic", "exec "+shellQuoted(command)))
	if !ran || status != 0 {
		return "", "", false
	}
	pane, socket, found := strings.Cut(out, " ")
	if !found || !strings.HasPrefix(pane, "%") || socket == "" {
		return "", "", false
	}
	return pane, socket, true
}

// alive says whether the session is there; known is false when tmux couldn't
// be asked. No server at all (the hire's ended) is a no.
func (h *hireTmux) alive() (alive, known bool) {
	_, status, ran := h.run(h.command("has-session", "-t", "="+hireSession))
	return ran && status == 0, ran
}

// capture is the last lines lines of the pane as text, without colours.
func (h *hireTmux) capture(lines int) (string, bool) {
	lines = min(max(lines, 1), 5000)
	out, status, ran := h.run(h.command("capture-pane", "-p", "-t", hirePane, "-S", "-"+strconv.Itoa(lines)))
	return out, ran && status == 0
}

// paneState is what state the hire's window is in: PaneUnknown when it
// couldn't be read, so nothing is typed.
func (h *hireTmux) paneState() PaneState {
	screen, ok := h.capture(60)
	if !ok {
		return PaneUnknown
	}
	return ReadPaneState(screen)
}

// viewers is how many terminal windows are attached to the session; -1 when tmux couldn't say.
func (h *hireTmux) viewers() int {
	out, status, ran := h.run(h.command("list-clients", "-t", "="+hireSession, "-F", "#{client_tty}"))
	if !ran || status != 0 {
		return -1
	}
	return len(strings.Fields(out))
}

// clientTTYs are the ttys of the terminal windows attached to the session; false when tmux couldn't say.
func (h *hireTmux) clientTTYs() ([]string, bool) {
	out, status, ran := h.run(h.command("list-clients", "-t", "="+hireSession, "-F", "#{client_tty}"))
	if !ran || status != 0 {
		return nil, false
	}
	return slices.DeleteFunc(strings.Fields(out), func(tty string) bool { return !isTTYPath(tty) }), true
}

// detachViewers detaches the windows attached to the session, on this socket
// alone: the hire carries on with no window (agent windows in the background).
func (h *hireTmux) detachViewers() bool { return h.ok("detach-client", "-s", "="+hireSession) }

// end ends the hire's session by its own name, on this socket only. With no
// other session the server ends with it.
func (h *hireTmux) end() bool { return h.ok("kill-session", "-t", "="+hireSession) }

// endServer ends the hire's server, and so its session, as the Mac's HireTmux.kill: this socket's only.
func (h *hireTmux) endServer() bool { return h.ok("kill-server") }

// waitForStart waits for the hire to be alive and heard from. Dead at any
// look is a failure at once; not heard from by within is one too. The clock
// and the pause are given, so tests don't wait.
func waitForStart(alive func() (alive, known bool), heardFrom func() bool, within, every time.Duration,
	now func() time.Time, pause func(time.Duration)) (started bool, reason string) {
	const ended = "its tmux session ended before it reported"
	end := now().Add(within)
	for {
		if up, known := alive(); known && !up {
			return false, ended
		}
		if heardFrom() {
			if up, known := alive(); known && !up {
				return false, ended
			}
			return true, ""
		}
		if !now().Before(end) {
			return false, fmt.Sprintf("no word from it in %d seconds", int(within.Seconds()))
		}
		pause(every)
	}
}

// cleanUpHire ends a hire that never started: its session, then its worktree
// when it is new and clean (removeWorktree says whether it went), then
// fail says it failed and why. It gives what it left behind, if anything.
func cleanUpHire(hire Hire, reason string, tmux *hireTmux, removeWorktree func(Hire) bool, fail func(id, reason string)) string {
	if tmux != nil {
		if up, known := tmux.alive(); !known || up {
			tmux.end()
		}
	}
	left := ""
	if hire.Worktree != "" && !removeWorktree(hire) {
		left = " Its worktree " + hire.Worktree + " was kept: it has changes, or git wouldn't remove it."
	}
	fail(hire.ID, "Didn't start: "+reason+"."+left)
	return left
}
