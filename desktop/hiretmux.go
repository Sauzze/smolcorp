//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
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
	// startOnly runs start's call alone, when it isn't run: a sandboxed desk's (desksandbox.go), inside its sandbox, or
	// the real tmux's with its guard off for the start only, and refuses any other (onlyStarts). nil: run starts it too
	// (a stand-in runner). Only start calls it: the grep row holds every place one is made, kept or called.
	startOnly tmuxStarter
	// sandboxed: a desk in its sandbox, on its own socket in deskSocketParent, started only through its sandbox (the
	// engine sets startOnly before start). Its server's pid, as the core checked it at the start, is server.
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

// deskShapedSocket says whether socket is where a sandboxed desk's socket would be, whatever its folder is called: a
// file named s in a folder right inside deskSocketParent, as this system finds the place (FXDESKCASE). A Mac's volume
// is case-insensitive as made, and folds as Unicode does: …/AB12/s, …/ab12/S and …/ab12/ſ are desk ab12's own socket
// there. So names are compared folded on a Mac and as they are elsewhere. A parent that is the same folder on the disk
// (the firmlink's /System/Volumes/Data spelling, say) counts, and so does a link to a desk's folder or socket. The place
// is also found as the kernel finds it (FXDESKCASER1): a relative socket from the core's current folder (tmux is run
// there), and a .. after a link back from the link's target, not cleaned away as text. If the current folder can't be
// read, a relative socket counts as a desk's: nothing is run on it. Only deskSocketID's spelling is driven, with its
// row's server; any other spelling of the place is given nothing (tmuxFor).
func deskShapedSocket(socket string) bool {
	raw := socket
	if !filepath.IsAbs(raw) {
		folder, err := workingFolder()
		if err != nil || !filepath.IsAbs(folder) {
			return true
		}
		raw = folder + string(filepath.Separator) + raw // joined as text: filepath.Join would clean a .. after a link away
	}
	socket = filepath.Clean(raw)
	parent := physical(deskSocketParent())
	places := []string{socket, filepath.Join(physical(filepath.Dir(socket)), filepath.Base(socket)), physical(socket)}
	// As the kernel walks the raw text: EvalSymlinks takes each link before the .. after it. For a socket that isn't
	// there yet, its folder walked so, and its name.
	if resolved, err := filepath.EvalSymlinks(raw); err == nil {
		places = append(places, resolved)
	} else if cut := strings.LastIndexByte(raw, filepath.Separator); cut >= 0 {
		if folder, err := filepath.EvalSymlinks(raw[:cut+1]); err == nil {
			places = append(places, filepath.Join(folder, raw[cut+1:]))
		}
	}
	for _, at := range places {
		if sameName(filepath.Base(at), deskSocketFile) && samePlace(physical(filepath.Dir(filepath.Dir(at))), parent) {
			return true
		}
	}
	return false
}

// workingFolder is the core's current folder, where a relative socket is found: a variable, for the tests.
var workingFolder = os.Getwd

// namesFold is whether this system's names compare folded: a Mac's, as its volume is made (a case-sensitive one only
// puts more spellings under a desk's place, never fewer). Linux's /tmp compares names as they are.
const namesFold = runtime.GOOS == "darwin"

// sameName says whether a and b name the same file in one folder, as namesFold says.
func sameName(a, b string) bool { return a == b || namesFold && strings.EqualFold(a, b) }

// samePlace says whether folders a and b are one: spelt alike (sameName), or the same folder on the disk.
func samePlace(a, b string) bool {
	if sameName(a, b) {
		return true
	}
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
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
// start-only runner (startOnly) before its start. errNoBundledTmux when there's none.
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
		h.run = oneShotTmux(program, h.guard)
	}
	return h, nil
}

// oneShotTmux is a desk's runner for the tmux at program, for every call but its start: each through tmuxClient, told
// what guard says at that call, and no call at all while guard says "" (a sandboxed desk before its start has checked
// a server, or a row that records none). The release gate (RT126SAL5, TMUXGATE1): the bundled tmux is never run
// without a SMOLCORP_TMUX_SERVER the core set.
func oneShotTmux(program string, guard func() string) tmuxRunner {
	return oneShotTmuxWithin(tmuxCallWait, program, guard)
}

// oneShotTmuxWithin is oneShotTmux with each call stopped after wait.
func oneShotTmuxWithin(wait time.Duration, program string, guard func() string) tmuxRunner {
	return func(args []string) (string, int, bool) {
		cmd, cancel, err := tmuxClient(wait, program, guard(), args...)
		if err != nil {
			return "", 0, false
		}
		defer cancel()
		return tmuxAnswer(cmd.Output())
	}
}

// guard is what the hire's guarded tmux is told of its server (SMOLCORP_TMUX_SERVER): the pid the core checked at its
// start; tmuxUnchecked for a desk outside the sandbox whose row records none (one started before 1.26's sandbox, which
// keeps its recorded tmux); "" for a sandboxed desk with none, whose calls aren't run. Never the core's own.
func (h *hireTmux) guard() string {
	switch {
	case h.server > 1:
		return strconv.Itoa(h.server)
	case h.sandboxed:
		return ""
	}
	return tmuxUnchecked
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
		h.run = oneShotTmux(program, h.guard)
		if !h.sandboxed {
			// The start alone runs its guard off: a guarded client never starts a server (third_party/tmux).
			h.startOnly = onlyStarts(h.socket, startingTmux(withoutStartFile(tmuxStartCall(program))))
		}
		h.program = recordableTmux(program)
	}
	return h
}

// tmuxFor is what a tab's tmux calls on socket run, and the server their guard is told (SA-M7): tmuxProgramFor's tmux;
// for a sandboxed desk's socket, the server pid its row records ("" with none, or with a recorded tmux that's gone:
// nothing is run); for any other spelling of a desk's place (deskShapedSocket: another case, a folder that isn't a
// desk's id, a link), "": nothing is run, never a tmux of your own on a desk's server (FXDESKCASE); for any other,
// tmuxUnchecked (a desk started before the sandbox, which keeps its recorded tmux, or a tmux of the owner's own, which
// a stock tmux ignores).
func tmuxFor(socket string) (program, guard string) {
	program, guard = tmuxProgramFor(socket), tmuxUnchecked
	if deskShapedSocket(socket) {
		guard = ""
		if deskSocketID(socket, testCopyPort()) == "" {
			return program, guard
		}
		if _, server := recordedDeskTmux(socket); server > 1 {
			guard = strconv.Itoa(server)
		}
	}
	return program, guard
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

// onlyStarts is start, for the desk on socket, refusing with nothing run any call but the one hireTmux.start makes:
// -S socket start-server ; set-environment -gu SMOLCORP_TMUX_SERVER ; new-session …, with no other command after it
// (a word that is or ends in ";" would start one, run with the guard off). A start-only runner is never a way to make
// another call (RT126SAM7R2 B1′): window, capture-pane and kill-server go through run, told the server they talk to.
func onlyStarts(socket string, start tmuxStarter) tmuxStarter {
	return func(args []string) (string, int, error) {
		if !serverStartCall(socket, args) {
			return "", 0, errTmuxDidntStart
		}
		return start(args)
	}
}

// serverStartCall says whether args are the start of the server on socket as hireTmux.start makes them, and nothing more.
func serverStartCall(socket string, args []string) bool {
	head := append([]string{"-S", socket}, tmuxServerStart("new-session")...)
	if socket == "" || len(args) <= len(head) || !slices.Equal(args[:len(head)], head) {
		return false
	}
	return !slices.ContainsFunc(args[len(head):], func(word string) bool { return strings.HasSuffix(word, ";") })
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
// A real start goes through h.startOnly, the start-only runner, never through run, which every other call takes: the
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
	case h.startOnly != nil:
		args = tmuxServerStart(args...)
		var server int
		if out, server, err = h.startOnly(h.command(args...)); err != nil {
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

// respawnCheckWait is how long the prune round's look at one desk's pane may take.
const respawnCheckWait = 2 * time.Second

// respawnUnsaved are the desks whose mark couldn't be saved (the records held), by row and pane: the log says so once.
var respawnUnsaved sync.Map

// paneStartedAt is processStartedAt: a variable, for the tests.
var paneStartedAt = processStartedAt

// paneRespawned is why desk h's pane no longer runs the process its start made, in words, or "": the pane's pid isn't
// the one its row records (something respawned the pane: RT126SG2R2 N2), or it is, but that pid's process started at
// another time (the pid was used again for another process: N1). "" too when nothing was recorded, or its own tmux
// couldn't say (an ended desk is for its watch and its fire to tell).
func paneRespawned(h Hire, run tmuxRunner) string {
	if h.TmuxPane <= 1 {
		return ""
	}
	tmux := hireOwnTmux(h, testCopyPort(), run)
	if tmux == nil {
		return ""
	}
	if run == nil {
		// A hung server holds the prune round up for this long at most, not a whole call's wait (RT126SAM7 nit 4).
		tmux.run = oneShotTmuxWithin(respawnCheckWait, tmux.program, tmux.guard)
	}
	pid, ok := tmux.panePID()
	switch {
	case !ok:
		return ""
	case pid != h.TmuxPane:
		return fmt.Sprintf("its tmux pane runs another process than the one Smolcorp started (pid %d, not %d): something "+
			"restarted it, so what runs there isn't guarded", pid, h.TmuxPane)
	case h.TmuxPaneStarted != 0:
		if now := paneStartedAt(pid); now != 0 && now != h.TmuxPaneStarted {
			return fmt.Sprintf("the process in its tmux pane has the pid Smolcorp started it with (%d) but started later: "+
				"that pid was used again, so what runs there isn't guarded", pid)
		}
	}
	return ""
}

// markRespawned marks each open desk whose pane no longer runs the process its start made (paneRespawned): its row's
// Unguarded says why, once, and so does the log. Detection, not prevention: any program of the owner's outside a
// sandbox can still respawn a pane (SANDBOX-PLAN §5.1); a desk in its sandbox can't reach another's socket (M2). run is
// the desks' tmux (nil: the real one). The deck runs it at each prune round.
func markRespawned(run tmuxRunner) {
	for _, h := range readRegistry(registryPath(), false).hires {
		if !isOpen(h) || h.T3Session != "" || h.Unguarded != "" || h.TmuxPane <= 1 {
			continue
		}
		why := paneRespawned(h, run)
		if why == "" {
			continue
		}
		marked := false
		err := updateRegistry(func(f *registryFile) {
			// Only the row as it was looked at: one started again meanwhile (Hand back) has a pane of its own.
			if i := slices.IndexFunc(f.hires, func(r Hire) bool { return r.ID == h.ID }); i >= 0 &&
				f.hires[i].TmuxPane == h.TmuxPane && f.hires[i].Unguarded == "" {
				f.hires[i].Unguarded = firstRunes(why, longestNotes)
				marked = true
			}
		})
		// Said once: when the mark is saved, or the first time it can't be (it's looked at again every round).
		switch {
		case err == nil && marked:
			log.Printf("tmux: %s: %s.", calledHire(h), why)
		case err != nil:
			if _, said := respawnUnsaved.LoadOrStore(h.ID+" "+strconv.Itoa(h.TmuxPane), true); !said {
				log.Printf("tmux: %s: %s. Smolcorp couldn't mark it (%v), and will once it can.", calledHire(h), why, err)
			}
		}
	}
}

// markRespawnedDesks is markRespawned with the engine's tmux.
func (d *Deck) markRespawnedDesks() {
	d.mu.Lock()
	run := d.engine.Tmux
	d.mu.Unlock()
	markRespawned(run)
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
	// What it started may still run in its sandbox once its session is gone: ended, and seen ended, before its
	// worktree goes (FXFIREREAPR1). One that can't be keeps it, and the row says so.
	if why := deskReap(hire); why != "" {
		fail(hire.ID, "Didn't start: "+reason+". "+why+startReapKept)
		return " " + why + startReapKept
	}
	left := ""
	if hire.Worktree != "" && !removeWorktree(hire) {
		left = " Its worktree " + hire.Worktree + " was kept: it has changes, or git wouldn't remove it."
	}
	fail(hire.ID, "Didn't start: "+reason+"."+left)
	return left
}
