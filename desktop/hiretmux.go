//go:build !windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
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

// tmuxStarter is a desk's start-only runner (SA-M6): it runs the one call that starts the desk's server, never another,
// and says what tmux printed and the server's pid as the core checked it (0: not checked), or why nothing started, in
// the words the owner sees.
type tmuxStarter func(args []string) (output string, server int, err error)

type hireTmux struct {
	socket string
	run    tmuxRunner
	// program is the tmux that runs it, when it's the real one: what a hire's row records (TmuxProgram), and
	// what a window attaches with. "" for a stand-in runner.
	program string
	// starts runs start's call alone, when it isn't run: a sandboxed desk's (desksandbox.go), inside its sandbox, or
	// the real tmux's with its guard off for the start only. nil: run starts it too (a stand-in runner).
	starts tmuxStarter
	// sandboxed: a desk in its sandbox, on its own socket in deskSocketParent, started only through its sandbox (the
	// engine sets starts before start). Its server's pid, as the core checked it at the start, is server.
	sandboxed bool
	server    int
}

// errTmuxDidntStart is a start tmux didn't make: no session, or what it printed isn't one.
var errTmuxDidntStart = errors.New("tmux didn't start it")

// A sandboxed desk's socket (SA-M6, SANDBOX-PLAN §A3.1 and §A5.2 step 8): deskSocketParent()/<id>/s, a test copy's
// deskSocketParent()/<port>-<id>/s. The parent, /tmp/smolcorp-<uid>, is the core's alone (0700, checked at each
// start); each desk's own folder in it is made new for its start, and a start whose folder or socket is there already
// is refused. Its profile lets the desk connect to that exact path and write in its own folder alone (M2).
const deskSocketFile = "s"

// deskSocketParent is the folder every sandboxed desk's own folder is in, as the kernel spells it (/private/tmp on a
// Mac): the Policy's SocketParent. Tests point it elsewhere.
var deskSocketParent = func() string {
	return filepath.Join(physical("/tmp"), "smolcorp-"+strconv.Itoa(os.Getuid()))
}

// deskSocketPath is where desk hireID's socket goes when it starts in its sandbox; "" for an id that can't name one.
func deskSocketPath(hireID string, testPort int) string {
	if !plainHireID(hireID) {
		return ""
	}
	return filepath.Join(deskSocketParent(), testPortPart(testPort)+hireID, deskSocketFile)
}

// deskSocketID is the id of the sandboxed desk socket is the socket of, a test copy's on testPort, or "": a file
// named s in a folder right inside deskSocketParent, named as deskSocketPath names it, the folders on the way resolved.
func deskSocketID(socket string, testPort int) string {
	socket = filepath.Clean(socket)
	folder := filepath.Dir(socket)
	if !filepath.IsAbs(socket) || filepath.Base(socket) != deskSocketFile || physical(filepath.Dir(folder)) != physical(deskSocketParent()) {
		return ""
	}
	if id, ok := strings.CutPrefix(filepath.Base(folder), testPortPart(testPort)); ok && plainHireID(id) {
		return id
	}
	return ""
}

// makeDeskSocketFolder makes the own folder of the desk whose socket is socket, new, for its start (§A5.2 step 8): the
// parent made 0700 when it isn't there, then checked a folder, not a link, this user's, that nobody else can open; the
// desk's folder made by this call, and checked the same. Anything already at the desk's folder refuses the start.
func makeDeskSocketFolder(socket string) error {
	folder := filepath.Dir(socket)
	if !ownSocketFolder(filepath.Dir(folder)) {
		return fmt.Errorf("the socket folder %s isn't this user's alone", filepath.Dir(folder))
	}
	if err := os.Mkdir(folder, 0o700); err != nil {
		return fmt.Errorf("the desk's socket folder: %w", err)
	}
	if !ownSocketFolder(folder) {
		return fmt.Errorf("the desk's socket folder %s isn't this user's alone", folder)
	}
	return nil
}

// deskSocketGoneWithin is how long removeDeskSocketFolder waits, in all, for a server just ended to stop answering.
var deskSocketGoneWithin = 2 * time.Second

// removeDeskSocketFolder takes a sandboxed desk's own socket folder away once nothing answers there: its socket (one
// its ended server left) and then the folder, only while it's empty. A start that never made a server, a desk taken
// over or ended, leaves no folder to refuse its next start (Hand back's resume uses its id again).
func removeDeskSocketFolder(socket string) {
	if deskSocketID(socket, testCopyPort()) == "" {
		return
	}
	// One deadline for it all: something that holds the socket without answering doesn't hold up the engine.
	for until := time.Now().Add(deskSocketGoneWithin); ; {
		info, err := os.Lstat(socket)
		if err != nil || info.Mode().Type() != os.ModeSocket {
			break
		}
		conn, err := net.DialTimeout("unix", socket, 100*time.Millisecond)
		if err != nil {
			_ = os.Remove(socket)
			break
		}
		conn.Close()
		if time.Now().After(until) {
			return // a server answers there still
		}
		time.Sleep(100 * time.Millisecond) // a server just ended may still be going
	}
	_ = os.Remove(filepath.Dir(socket))
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
	// A server to be started: the tmux this system picks now, whatever an older row for this hire says.
	return hireTmuxUsing(filepath.Join(folder, name), testPort, run, func(string) string { return tmuxPath() })
}

// errNoBundledTmux is a sandboxed desk's start with no bundled tmux to start it: none falls back to another tmux.
var errNoBundledTmux = errors.New("This copy of Smolcorp is missing its tmux. Reinstall Smolcorp.")

// newDeskTmux is desk hireID's server in its sandbox (SA-M6): its socket at deskSocketPath, started and driven by the
// tmux bundled with Smolcorp alone (with run nil), never Homebrew's or one on a PATH. The engine gives it its
// start-only runner (starts) before its start. errNoBundledTmux when there's none.
func newDeskTmux(hireID string, testPort int, run tmuxRunner) (*hireTmux, error) {
	socket := deskSocketPath(hireID, testPort)
	if socket == "" || len(socket) > maxSocketPath {
		return nil, errors.New("Smolcorp couldn't set up a tmux server of its own for this hire: its socket's path is too long.")
	}
	h := &hireTmux{socket: socket, run: run, sandboxed: true}
	if run == nil {
		program := bundledTmux()
		if program == "" {
			return nil, errNoBundledTmux
		}
		h.program = recordableTmux(program)
		h.run = h.checkedTmux(program)
	}
	return h, nil
}

// checkedTmux is a sandboxed desk's runner for the bundled tmux at program: each call told the server pid the core
// checked (guard), and no call at all while there's none, before its start or for a row that records none. The
// release gate (RT126SAL5): the bundled tmux is never run without a SMOLCORP_TMUX_SERVER the core set.
func (h *hireTmux) checkedTmux(program string) tmuxRunner {
	run := withoutStartFile(guardedTmux(program, h.guard))
	return func(args []string) (string, int, bool) {
		if h.guard() == "" {
			return "", 0, false
		}
		return run(args)
	}
}

// guard is what the hire's guarded tmux is told of its server (SMOLCORP_TMUX_SERVER): the pid the core checked at its
// start, "" when there's none (the variable is then not given: never the core's own).
func (h *hireTmux) guard() string {
	if h.server > 1 {
		return strconv.Itoa(h.server)
	}
	return ""
}

// hireTmuxAt is the server on socket, when it is one of ours: in a hire
// socket folder, named like a hire's (a test copy owns only its own port's), never the default.
func hireTmuxAt(socket string, testPort int, run tmuxRunner) *hireTmux {
	return hireTmuxUsing(socket, testPort, run, tmuxDriving)
}

// hireTmuxUsing is hireTmuxAt with pick, which says what real tmux to run for a socket. A sandboxed desk's (on its
// socket in deskSocketParent) is driven by its recorded tmux alone, told the server pid its row records.
func hireTmuxUsing(socket string, testPort int, run tmuxRunner, pick func(socket string) string) *hireTmux {
	if !ownsHireSocket(socket, testPort) || len(socket) > maxSocketPath {
		return nil
	}
	h := &hireTmux{socket: filepath.Clean(socket), run: run, sandboxed: deskSocketID(socket, testPort) != ""}
	if run == nil {
		var program string
		if h.sandboxed {
			program, h.server = recordedDeskTmux(socket)
		} else {
			program = pick(socket)
		}
		if program == "" {
			return nil
		}
		if h.sandboxed {
			h.run = h.checkedTmux(program)
		} else {
			h.run = withoutStartFile(guardedTmux(program, h.guard))
			// The start alone runs its guard off: a guarded client never starts a server (third_party/tmux).
			h.starts = startingTmux(withoutStartFile(guardedTmux(program, func() string { return "off" })))
		}
		h.program = recordableTmux(program)
	}
	return h
}

// recordedDeskTmux is the tmux that started the sandboxed desk on socket and its server's pid, as its row records them,
// or "" when its row records no tmux that is still a program you can run: a sandboxed desk is never driven by another
// tmux (no fallback to tmuxPath's, which may be Homebrew's).
func recordedDeskTmux(socket string) (program string, server int) {
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.TmuxSocket == "" || !sameSocket(h.TmuxSocket, socket) {
			continue
		}
		if !filepath.IsAbs(h.TmuxProgram) || untrustedWhy(filepath.Dir(h.TmuxProgram), filepath.Base(h.TmuxProgram)) != "" {
			return "", 0
		}
		return h.TmuxProgram, h.TmuxServer
	}
	return "", 0
}

// startingTmux is a start-only runner from run: what tmux printed, or errTmuxDidntStart.
func startingTmux(run tmuxRunner) tmuxStarter {
	return func(args []string) (string, int, error) {
		out, status, ran := run(args)
		if !ran || status != 0 {
			return "", 0, errTmuxDidntStart
		}
		return out, 0, nil
	}
}

// ownsHireSocket says whether socket is one of ours: a hire's, in a hire socket
// folder, with a name of only a-z, 0-9 and -, not the user's default; or a sandboxed desk's (deskSocketID).
func ownsHireSocket(socket string, testPort int) bool {
	socket = filepath.Clean(socket)
	if deskSocketID(socket, testPort) != "" {
		return true
	}
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
// never inherit where Smolcorp was started or the GM's token. The bundled tmux's guard variable,
// SMOLCORP_TMUX_SERVER, is never the core's own: it is what guard says at each call, or not there when that's "".
func realTmux(path string) tmuxRunner { return guardedTmux(path, func() string { return "" }) }

// guardedTmux is realTmux, its guard variable what guard says at each call.
func guardedTmux(path string, guard func() string) tmuxRunner {
	return func(args []string) (string, int, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), tmuxCallWait)
		defer cancel()
		cmd := disclaim.Cmd(exec.CommandContext(ctx, path, args...))
		cmd.Env = withTmuxGuard(hireEnvironment(), guard())
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

// tmuxGuardVariable is the bundled tmux's guard (third_party/tmux/guard.patch): the server pid a client may talk to,
// "unchecked", or "off" for the call that starts a server.
const tmuxGuardVariable = "SMOLCORP_TMUX_SERVER"

// withTmuxGuard is env without any SMOLCORP_TMUX_SERVER of its own, and with guard's when it isn't "".
func withTmuxGuard(env []string, guard string) []string {
	env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(strings.ToUpper(kv), tmuxGuardVariable+"=") })
	if guard != "" {
		env = append(env, tmuxGuardVariable+"="+guard)
	}
	return env
}

// withoutStartFile runs tmux as run does, with no start file: a hire's server reads none of the owner's
// (~/.tmux.conf, ~/.config/tmux), whose commands would run at each desk's start (RT126SG2 B1, §4 g6). -f is read only
// by the call that starts the server; every other call ignores it.
func withoutStartFile(run tmuxRunner) tmuxRunner {
	return func(args []string) (string, int, bool) { return run(append([]string{"-f", os.DevNull}, args...)) }
}

// command is tmux's arguments for args on our server.
func (h *hireTmux) command(args ...string) []string { return append([]string{"-S", h.socket}, args...) }

func (h *hireTmux) ok(args ...string) bool {
	_, status, ran := h.run(h.command(args...))
	return ran && status == 0
}

// start starts the session in folder running command, two or more words, which tmux runs as they are: no shell,
// so no shell's start files (SANDBOX-PLAN §3.4, SG2). It gives the pane, the pid of the process tmux started for
// command, and the socket's path (as the deck addresses a tab: tmux:<pane>@<socket>), or false if tmux didn't
// start it. The pid is said by the same call that started the process, so no other call on the server comes in
// between: it is the process the desk's start ticket is for (deskstart.go), whatever the pane runs later.
//
// A real start goes through h.starts, the start-only runner, never through run, which every other call takes: the
// guard is off for that call alone (third_party/tmux), and the variable is taken out of the server's environment
// before its first pane (RT126SAX3 nit 1). A sandboxed desk is started only through its sandbox (desksandbox.go): with
// no runner for it, nothing starts. Its server's pid, as the core checked it, is kept in h.server before this returns,
// so the start ticket's look at the pane (deskstart.go, after deskStarts.pin) is told it. The error is the owner's words.
func (h *hireTmux) start(folder string, command []string) (pane, socket string, pid int, err error) {
	if len(command) < 2 {
		return "", "", 0, errTmuxDidntStart // one word would be run by a shell
	}
	args := append([]string{"new-session", "-d", "-P", "-F", "#{pane_id} #{pane_pid} #{socket_path}", "-s", hireSession, "-c", folder}, command...)
	var out string
	switch {
	case h.starts != nil:
		args = append([]string{"start-server", ";", "set-environment", "-gu", tmuxGuardVariable, ";"}, args...)
		var server int
		if out, server, err = h.starts(h.command(args...)); err != nil {
			return "", "", 0, err
		}
		h.server = server
	case h.sandboxed:
		// A sandboxed desk with no start-only runner of its own: nothing starts, never through run (RT126SAM6 B1).
		return "", "", 0, errTmuxDidntStart
	default:
		output, status, ran := h.run(h.command(args...))
		if !ran || status != 0 {
			return "", "", 0, errTmuxDidntStart
		}
		out = output
	}
	pane, rest, _ := strings.Cut(out, " ")
	said, socket, found := strings.Cut(rest, " ")
	pid, atoiErr := strconv.Atoi(said)
	if !found || !strings.HasPrefix(pane, "%") || socket == "" || atoiErr != nil || pid <= 1 || h.sandboxed && h.server <= 1 {
		if h.server > 1 {
			h.endServer() // the server the core checked, told its pid: nothing of the refused start stays running
		}
		return "", "", 0, errTmuxDidntStart
	}
	return pane, socket, pid, nil
}

// panePID is the pid of the process the session's pane runs, as the hire's own server says it.
func (h *hireTmux) panePID() (int, bool) {
	out, status, ran := h.run(h.command("display-message", "-p", "-t", hirePane, "#{pane_pid}"))
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	return pid, ran && status == 0 && err == nil && pid > 1
}

// alive says whether the session is there; known is false when tmux couldn't
// be asked. No server at all (the hire's ended) is a no.
func (h *hireTmux) alive() (alive, known bool) {
	_, status, ran := h.run(h.command("has-session", "-t", "="+hireSession))
	return ran && status == 0, ran
}

// capture is the last lines lines of the pane as text with its escapes (-e),
// so Claude Code's faint suggestion can be told from typed text
// (ReadStyledPaneState); paneText makes it plain.
func (h *hireTmux) capture(lines int) (string, bool) {
	lines = min(max(lines, 1), 5000)
	out, status, ran := h.run(h.command("capture-pane", "-p", "-e", "-t", hirePane, "-S", "-"+strconv.Itoa(lines)))
	return out, ran && status == 0
}

// paneState is what state the hire's window is in: PaneUnknown when it
// couldn't be read, so nothing is typed.
func (h *hireTmux) paneState() PaneState {
	screen, ok := h.capture(60)
	if !ok {
		return PaneUnknown
	}
	return ReadStyledPaneState(screen)
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
