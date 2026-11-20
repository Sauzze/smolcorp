package desktop

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A plain hire's agent in its sandbox on a Mac (SA-M10, SB7's Mac part). A plain hire (a pinned desk, another agent,
// no role) is started by its launcher, `smolcorp run --hire`, in a window or in tmux (hire.go): the launcher stays
// outside, where it reaches the app as it always has, and starts its agent, the child it runs in a pseudo-terminal, in a
// sandbox made for it, as a desk's tmux server is (desksandbox_darwin.go):
//
//  1. its own id (plain-…), its own folder in the support folder, and its own socket folder, made new;
//  2. its Places, from the folder it works in, and its Policy; X12's link check;
//  3. its profile, at <support>/hires/<id>/sandbox.sb; a fresh canary, the start's nonce, its handshake, a stand-in
//     other server (§A5.2 step 2);
//  4. `smolcorp-spawn /usr/bin/sandbox-exec -f <profile> <smolcorp> sandbox enter … -- <agent> …` (sandbox.MacCommand),
//     on the pseudo-terminal: `enter` checks the sandbox holds and becomes the agent in place, so the pid the
//     launcher started is the agent's, and its fire (stopProcess) is the agent's;
//  5. the handshake, a full passing report for this start, before the launcher goes on: anything else ends it, and the
//     window says why in §A5.2's words.
//
// Its hooks reach the app through the launcher's relay on its own socket (hookrelay.go). The agent's own sandbox is
// switched off (agentownsandbox.go). A launcher of an engine hire (--hire-id, Windows's) and `smolcorp run` typed by
// hand are never sandboxed here: the first doesn't run on a Mac, and the second is the owner's own.

// launcherSandboxes is whether a plain hire's launcher starts its agent in a sandbox on this system.
const launcherSandboxes = true

// launcherSandbox is one plain hire's sandbox, made by its launcher before its agent starts.
type launcherSandbox struct {
	desk    *deskSandbox
	socket  string // its own socket: the hook relay's
	own     string // its own folder in the support folder, made by its start
	profile string
	relay   *hookRelay
	start   *sandbox.Start
	closing sync.Once
	// hookKey is this launcher's own hook key, which its relay sends with (FXSWEEP3P3 F-3), made new at each start
	// and never written down; connected is closed once the app has it (noteConnected).
	hookKey      string
	connected    chan struct{}
	connectedSay sync.Once
}

// relayKeyWait is how long an event the relay has before the app has the launcher's own key waits for it: less than
// the hook's own wait (hookTimeout), and far more than the launcher takes to reach the app after its agent starts.
const relayKeyWait = 1500 * time.Millisecond

// plainHireIDFor is a new plain hire's id (plainHireTmuxID): a variable, for the tests.
var plainHireIDFor = plainHireTmuxID

// launcherStartCommand makes the start's command (sandbox.MacCommand): a variable, for the tests.
var launcherStartCommand = sandbox.MacCommand

// newLauncherSandbox makes the sandbox of a plain hire working in folder, its agent being agent at program: everything
// up to its start. launcher is the launcher's own id, its tab's (SMOLCORP_SESSION). The error is the owner's words.
func newLauncherSandbox(folder string, gm bool, launcher string, agent Agent, program string) (*launcherSandbox, error) {
	return newLauncherSandboxAt("", "", folder, gm, launcher, agent, program)
}

// newLauncherSandboxAt is newLauncherSandbox for the desk the office named, desk (`--desk`, plainDeskID's shape), or
// a new one for "": an office's plain hire on a Mac works in its own worktree, on the branch the office made for that
// desk, hire/<desk>/… (FXHIREPROJECTSR7, ruling (W)), so its sandbox is that desk's. Its folder in the support folder
// is made new either way: a desk named twice doesn't start the second time. A desk the office made no worktree for
// (a folder in no repo, or a GM's own) works in folder itself, which must still resolve to place, the folder as the
// rule judged it at its hire (`--place`): its one start is checked as an engine desk's every start is
// (deskPlacedWords, FXGMHIRENOGIT126R4, R10).
func newLauncherSandboxAt(desk, place, folder string, gm bool, launcher string, agent Agent, program string) (*launcherSandbox, error) {
	return newLauncherSandboxPlaced(desk, place, "", folder, gm, launcher, agent, program)
}

// newLauncherSandboxPlaced is newLauncherSandboxAt with place's identity at the hire, placeID (`--place-id`): the
// start then also checks the folder at place is that one, as deskPlacedNow checks an engine desk's (FXRC3FIXR1,
// RT126RC3FIX's F-c). A real folder renamed into the place since the hire resolves to the same path, and only its
// identity tells it apart. "" checks the path alone.
func newLauncherSandboxPlaced(desk, place, placeID, folder string, gm bool, launcher string, agent Agent, program string) (_ *launcherSandbox, err error) {
	reapLaunchersGone()
	id := plainHireIDFor()
	if desk != "" {
		id = desk
	}
	s := &launcherSandbox{}
	refuse := func(words, why string) (*launcherSandbox, error) {
		log.Printf("sandbox: plain hire %s didn't start: %s", id, why)
		s.close()
		return nil, errors.New(words)
	}
	socket := deskSocketPath(id, testCopyPort())
	if socket == "" || len(socket) > maxSocketPath || !filepath.IsAbs(folder) || !filepath.IsAbs(program) {
		return refuse(sandboxNotMade, "its socket, folder or program isn't a whole path")
	}
	if err := makeDeskSocketFolder(socket); err != nil {
		return refuse(sandbox.NotOurServerRefusal, err.Error())
	}
	s.socket = socket // made by this start, so taken away by it
	own := hireFolder(supportDir, id)
	if err := os.MkdirAll(filepath.Dir(own), 0o700); err != nil {
		return refuse(sandboxNotMade, "the hires' folder: "+err.Error())
	}
	if err := os.Mkdir(own, 0o700); err != nil {
		return refuse(sandboxNotMade, "its own folder: "+err.Error())
	}
	s.own = own
	if err := writeLauncherRecord(own); err != nil {
		return refuse(sandboxNotMade, "its launcher's record: "+err.Error())
	}
	at := deskAt{ID: id, GM: gm, Root: folder, Socket: s.socket, Accounts: claudeDenyProfiles(savedConfig().ClaudeProfiles),
		Git: runGit, Programs: []sandbox.Program{{Name: string(agent), Path: program}}, Worktrees: savedConfig().HireWorktrees}
	// A desk the office made a worktree for has it on record (keepPlainHire): its start checks that tree is still
	// the one made, as an engine desk's does (deskTreeWords, FXGMHIRENOGIT126R3, B2), whatever git now finds at folder.
	// Its checkout and folder for hires are its record's, as deskAtOf gives an engine desk's, and a desk working in its
	// folder has that folder checked first: no git runs at folder before its check, which a link put there since its
	// hire would lead elsewhere (FXGMHIRENOGIT126R5, R13).
	repoAt := folder
	if desk != "" {
		// The record's tree as stored: the start check's input (deskStartPlaceWords, just below), as deskAtOf gives it.
		if record, found := plainHireRecord(desk); found && !record.Worktree.none() && record.Repo != "" {
			at.Tree, at.TreeRepo = record.Worktree.storedOnly(), record.Repo
		} else {
			at.CheckPlaced, at.Placed, at.PlacedID = true, place, placeID
			repoAt = place
		}
		if words := deskStartPlaceWords(at); words != "" {
			return refuse(words, words)
		}
	}
	if at.Tree != "" {
		// Hires is the folder beside the repo, as deskAtOf gives it: the core's own folder and a configured one are held
		// by Places.Worktrees (FXCOREWORKTREES126).
		at.Checkout, at.Hires = at.TreeRepo, legacyHiresFolder(at.TreeRepo)
	} else if repoAt != "" {
		at.Checkout, at.Hires = launcherRepo(repoAt, at.Worktrees)
	}
	if s.desk, err = makeDeskSandbox(at); err != nil {
		return refuse(err.Error(), err.Error())
	}
	s.profile = filepath.Join(own, "sandbox.sb")
	if err := s.desk.prepareProfile(s.profile, deskServices(s.desk.places, s.socket)); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its profile: "+err.Error())
	}
	if s.start, err = sandbox.NewStart(s.desk.checks(s.profile)); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its check: "+err.Error())
	}
	s.hookKey, s.connected = randomKey(), make(chan struct{})
	handler := relayHandler(launcher, agent, fmt.Sprintf("http://127.0.0.1:%d", Port), s.relayKey, relayClient)
	if s.relay, err = startHookRelay(s.socket, handler); err != nil {
		return refuse(sandboxNotMade, "its hook relay: "+err.Error())
	}
	return s, nil
}

// launcherRepo is, for a plain hire working in folder, the owner's checkout when folder is in a linked worktree of it
// ("" otherwise), and the folder beside that repo for its hires' worktrees (legacyHiresFolder): as the engine's hire in its worktree has
// them (deskAtOf), so the checkout and the other hires' worktrees are held from it as from the engine's
// (FXHIREPROJECTSR7, ruling (W): an office's plain hire on a Mac works in a worktree of its own). git is asked where
// the shared .git is; a checkout's is its own .git. "" for both when folder is in no repo, or git can't say. The rule
// and git see one spelling of folder, its links resolved once, and a folder whose links couldn't be resolved gets no
// git (FXGMHIRENOGIT126R2, B2).
func launcherRepo(folder, worktrees string) (checkout, hires string) {
	folder, resolved := resolvedFolder(folder)
	if !resolved || gmOwnsFolder(folder) {
		return "", "" // no git in a General Manager's own folder (hiregmfolder.go)
	}
	status, out, ran := runGit(folder, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	lines := strings.Split(out, "\n")
	if !ran || status != 0 || len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return "", ""
	}
	top, common := physical(lines[0]), physical(lines[1])
	if filepath.Base(common) == ".git" && filepath.Dir(common) != top {
		checkout = filepath.Dir(common)
		return checkout, legacyHiresFolder(checkout)
	}
	return "", legacyHiresFolder(top)
}

// relayKey is the key the relay sends with: the launcher's own, once the app has it (its connection named it,
// serveDeck), never the owner's, which would let the hire report as any of the owner's own sessions (FXSWEEP3P3 F-3).
// Until then, a little while, an event waits for it.
func (s *launcherSandbox) relayKey() (string, error) {
	select {
	case <-s.connected:
		return s.hookKey, nil
	case <-time.After(relayKeyWait):
		return "", errors.New("the app doesn't have the launcher's hook key yet")
	}
}

// ownHookKey is the key the launcher gives the app at each connection (launcherRelayKeyHeader), "" for none.
func (s *launcherSandbox) ownHookKey() string {
	if s == nil {
		return ""
	}
	return s.hookKey
}

// noteConnected is the app's answer to a connection that gave it the launcher's own key: the relay may send with it.
func (s *launcherSandbox) noteConnected() {
	if s != nil && s.connected != nil {
		s.connectedSay.Do(func() { close(s.connected) })
	}
}

// environment is env for the agent in its sandbox: its hooks' relay, and set (its own sandbox switched off).
func (s *launcherSandbox) environment(env []string, set map[string]string) []string {
	change := hireStartEnv{Set: map[string]string{hookRelayEnvironment: s.socket}, Unset: []string{hookKeyEnvironment}}
	for name, value := range set {
		change.Set[name] = value
	}
	return startEnvironment(env, change, false)
}

// command is the agent's start in its sandbox, from cmd as the launcher made it (its program, words, environment and
// folder), on terminal's pseudo-terminal, as go-pty would start it: a session of its own, the terminal its own.
func (s *launcherSandbox) command(cmd *pty.Cmd, terminal pty.Pty) (agentStart, error) {
	tty, ok := terminal.(pty.UnixPty)
	smolcorp, err := executable()
	if err == nil {
		smolcorp, err = filepath.EvalSymlinks(smolcorp)
	}
	if !ok || err != nil || !filepath.IsAbs(smolcorp) {
		return nil, errors.New(sandbox.SandboxExecRefusal)
	}
	c, err := launcherStartCommand(s.start, s.profile, smolcorp, cmd.Env, cmd.Path, cmd.Args[1:]...)
	if err != nil {
		log.Printf("sandbox: a plain hire's start: %v", err)
		return nil, errors.New(sandbox.SandboxExecRefusal)
	}
	c.Dir = cmd.Dir
	c.Stdin, c.Stdout, c.Stderr = tty.Slave(), tty.Slave(), tty.Slave()
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setsid, c.SysProcAttr.Setctty, c.SysProcAttr.Ctty = true, true, 0
	return &sandboxedAgent{cmd: c, start: s.start}, nil
}

// close takes away what the sandbox made: its relay, its canary, its own folder, and its socket folder once nothing
// answers there. It is said once the agent has ended, or its start was refused.
func (s *launcherSandbox) close() {
	if s == nil {
		return
	}
	s.closing.Do(s.takeAway)
}

// closeOnHangup takes the sandbox away when a signal ends the launcher (its window closed: SIGHUP; a kill: SIGTERM),
// which would otherwise end it without its deferred close and leave its folders behind. Its agent ends as before, when
// the launcher's pseudo-terminal goes with it.
func (s *launcherSandbox) closeOnHangup() {
	ends := make(chan os.Signal, 1)
	signal.Notify(ends, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		got := <-ends
		s.close()
		code := 1
		if n, ok := got.(syscall.Signal); ok {
			code = 128 + int(n)
		}
		os.Exit(code)
	}()
}

// takeAway is close's work, done once whichever way the launcher ends. First, every program still running in the
// agent's sandbox is ended, and seen ended, while its profile is there to say which they are (FXFIREREAPR1): the
// agent's stop (stopProcess) ends its pid alone, and what it started in the background would outlive it. One that
// can't be keeps its own folder, and the window says so.
func (s *launcherSandbox) takeAway() {
	kept := false
	if s.own != "" {
		if left, sure := reapSandboxAt(s.own, true); len(left) != 0 || !sure {
			log.Printf("sandbox: plain hire %s at its stop: its processes %v left, every process answered: %t", filepath.Base(s.own), left, sure)
			fmt.Fprintln(os.Stderr, launcherKeptWords(reapLeftWords(Hire{Name: "the agent"}, left)))
			kept = true
		}
	}
	s.relay.close()
	if s.start != nil {
		s.start.Close()
		_ = s.start.Canary.Remove()
	}
	if s.desk != nil {
		keptSandboxes.Lock()
		delete(keptSandboxes.of, s.desk.at.ID)
		keptSandboxes.Unlock()
	}
	if s.socket != "" {
		_ = os.Remove(s.socket) // the relay's, closed: nothing answers there
		removeDeskSocketFolder(s.socket)
	}
	if s.own != "" && !kept {
		_ = os.RemoveAll(s.own)
	}
}

// launcherKeptWords is what the window says when a plain hire's sandbox couldn't be shown empty, why being
// reapLeftWords' words. Its folder is kept, and the next plain hire's start looks again (reapLaunchersGone).
func launcherKeptWords(why string) string {
	return "smolcorp: " + why + reapRestartWords(why, launcherUnsure)
}

// launcherUnsure ends launcherKeptWords' words when no program was named (RT126FIXREVIEW-FIREREAPR4 O2).
const launcherUnsure = ". Smolcorp looks again when you next start a plain hire; if this keeps happening, restart your Mac to be sure."

// A launcher that ends without its takeAway (a crash, a SIGKILL: closeOnHangup catches only SIGHUP and SIGTERM)
// leaves what its agent started running in its sandbox, and a plain hire has no row for a fire to find
// (FXFIREREAPR3, RT126FIXREVIEW-FIREREAPR1 O1). So each launcher writes in its own folder which process it is, and
// each plain hire's start first ends what an earlier one whose launcher has ended left there, as that launcher's
// takeAway would have: the same reap, with the same proof, then its folders taken away. No desk may write in the hires
// folder, so no desk can write or change a record; and a launcher is named by its pid and its start time, so a pid
// another process has taken since never passes for it.

// launcherRecord is the file in a plain hire's own folder that says which process its launcher is: "<pid> <start
// time, to the microsecond>".
const launcherRecord = "launcher"

// launcherStarted is process pid's start time, to the microsecond, and whether it runs (not ended, waiting for its parent
// to read its end); known is false when macOS doesn't say. A pid no process has is known not running.
func launcherStarted(pid int) (start uint64, running, known bool) {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if errors.Is(err, unix.EIO) {
		return 0, false, true // no process has that pid: macOS gives back nothing
	}
	if err != nil || int(k.Proc.P_pid) != pid {
		return 0, false, false
	}
	start = uint64(k.Proc.P_starttime.Sec)*1_000_000 + uint64(k.Proc.P_starttime.Usec)
	return start, k.Proc.P_stat != 5, true // SZOMB, <sys/proc.h>
}

// writeLauncherRecord writes this launcher's record in its own folder, own.
func writeLauncherRecord(own string) error {
	start, running, known := launcherStarted(os.Getpid())
	if !running || !known || start == 0 {
		return errors.New("macOS didn't say when it started")
	}
	return os.WriteFile(filepath.Join(own, launcherRecord), []byte(fmt.Sprintf("%d %d\n", os.Getpid(), start)), 0o600)
}

// launcherEnded is whether the launcher whose own folder is own has ended, as its record names it: no process has its
// pid any more, another has it now (a start time not its own), or it ended and waits for its parent. False when there
// is no record to read (a launcher from before records, or one that hasn't written it yet) or macOS doesn't say.
func launcherEnded(own string) bool {
	record := filepath.Join(own, launcherRecord)
	if info, err := os.Lstat(record); err != nil || !info.Mode().IsRegular() {
		return false
	}
	text, err := os.ReadFile(record)
	if err != nil {
		return false
	}
	var pid int
	var start uint64
	if n, err := fmt.Sscanf(string(text), "%d %d\n", &pid, &start); err != nil || n != 2 || pid <= 1 || start == 0 ||
		string(text) != fmt.Sprintf("%d %d\n", pid, start) {
		return false
	}
	now, running, known := launcherStarted(pid)
	return known && (!running || now != start)
}

// reapLaunchersGone ends what the agent of each earlier plain hire whose launcher has ended left running in its
// sandbox, and takes its folders away, as that launcher's takeAway would have. One that can't be shown empty keeps its
// own folder, and the window says so. A plain hire whose launcher still runs, or that has no record, is left as it is.
func reapLaunchersGone() {
	entries, err := os.ReadDir(filepath.Join(supportDir, "hires"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(id, "plain-") || !plainHireID(id) {
			continue
		}
		own := hireFolder(supportDir, id)
		if !launcherEnded(own) {
			continue
		}
		if left, sure := reapSandboxAt(own, true); len(left) != 0 || !sure {
			log.Printf("sandbox: plain hire %s, its launcher ended: its processes %v left, every process answered: %t", id, left, sure)
			fmt.Fprintln(os.Stderr, launcherKeptWords(reapLeftWords(Hire{Name: "an earlier plain hire's agent"}, left)))
			continue
		}
		log.Printf("sandbox: plain hire %s, its launcher ended: nothing of it left in its sandbox, its folders taken away", id)
		removeDeskSocketFolder(deskSocketPath(id, testCopyPort()))
		_ = os.RemoveAll(own)
	}
}

// sandboxedAgent is the agent started in its sandbox.
type sandboxedAgent struct {
	cmd   *exec.Cmd
	start *sandbox.Start
}

// Start starts it and waits for `enter`'s report: nil only for a full, passing report of this start, and then the
// agent runs; anything else ends what started, in §A5.2's words.
func (a *sandboxedAgent) Start() error {
	if err := a.cmd.Start(); err != nil {
		log.Printf("sandbox: a plain hire's start: sandbox-exec didn't run: %v", err)
		return errors.New(sandbox.SandboxExecRefusal)
	}
	if err := a.start.Wait(sandboxStartWait); err != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
		log.Printf("sandbox: a plain hire's start: %v", err)
		return errors.New(sandbox.Refusal("darwin"))
	}
	_ = a.start.Canary.Remove() // nothing after the handshake reads it
	return nil
}

func (a *sandboxedAgent) Wait() error          { return a.cmd.Wait() }
func (a *sandboxedAgent) process() *os.Process { return a.cmd.Process }
