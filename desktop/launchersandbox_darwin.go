package desktop

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/aymanbagabas/go-pty"

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
}

// plainHireIDFor is a new plain hire's id (plainHireTmuxID): a variable, for the tests.
var plainHireIDFor = plainHireTmuxID

// launcherStartCommand makes the start's command (sandbox.MacCommand): a variable, for the tests.
var launcherStartCommand = sandbox.MacCommand

// newLauncherSandbox makes the sandbox of a plain hire working in folder, its agent being agent at program: everything
// up to its start. launcher is the launcher's own id, its tab's (SMOLCORP_SESSION). The error is the owner's words.
func newLauncherSandbox(folder string, gm bool, launcher string, agent Agent, program string) (_ *launcherSandbox, err error) {
	id := plainHireIDFor()
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
	at := deskAt{ID: id, GM: gm, Root: folder, Socket: s.socket, Accounts: claudeDenyProfiles(savedConfig().ClaudeProfiles),
		Git: runGit, Programs: []sandbox.Program{{Name: string(agent), Path: program}}}
	if status, top, ran := runGit(folder, "rev-parse", "--path-format=absolute", "--show-toplevel"); ran && status == 0 && filepath.IsAbs(top) {
		at.Hires = hiresFolder(top, savedConfig().HireWorktrees)
	}
	if s.desk, err = makeDeskSandbox(at); err != nil {
		return refuse(err.Error(), err.Error())
	}
	s.profile = filepath.Join(own, "sandbox.sb")
	if err := sandbox.PrepareProfile(s.desk.policy, s.profile, deskServices(s.desk.places, s.socket)); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its profile: "+err.Error())
	}
	if s.start, err = sandbox.NewStart(s.desk.checks(s.profile)); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its check: "+err.Error())
	}
	handler := relayHandler(launcher, agent, fmt.Sprintf("http://127.0.0.1:%d", Port), launcherHookKey, relayClient)
	if s.relay, err = startHookRelay(s.socket, handler); err != nil {
		return refuse(sandboxNotMade, "its hook relay: "+err.Error())
	}
	return s, nil
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

// takeAway is close's work, done once whichever way the launcher ends.
func (s *launcherSandbox) takeAway() {
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
	if s.own != "" {
		_ = os.RemoveAll(s.own)
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
