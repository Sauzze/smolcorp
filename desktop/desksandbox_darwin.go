package desktop

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// sandboxNotMade is what the owner sees when a desk's sandbox can't be made: §A5.2's first row, as the sandbox's
// runner says it (one source).
const sandboxNotMade = sandbox.SandboxExecRefusal

// The Mac's start-only runner (SA-M6, SANDBOX-PLAN §A3.1 and §A5.2): how a desk's tmux server is started in its
// sandbox, the one call hireTmux.start runs through it. In order, each step refusing the start with §A5.2's words:
//
//  1. its socket's folder, made new in the parent the core alone can open (step 8): anything already there refuses;
//  2. its profile, from its Policy and what it may reach (Services), written to <support>/hires/<id>/sandbox.sb;
//  3. a fresh canary, the start's nonce, its handshake and a stand-in other server (sandbox.NewStart, step 2);
//  4. `smolcorp-spawn /usr/bin/sandbox-exec -f <profile> <smolcorp> sandbox enter … -- <bundled tmux> -S <socket> …`
//     (sandbox.MacCommand, step 1): `enter` runs its rows inside, reports them, and becomes the bundled tmux, whose
//     start call is the only one with its guard off;
//  5. the handshake, a full passing report for this start (step 7), then tmux's start call ending;
//  6. the server at its socket, ours: in this start's sandbox, by the canary, its pid taken (step 9).
//
// The canary goes once the server is checked: nothing after the start reads it (a later attach makes its own). A
// start that fails takes its socket folder away when nothing answers there.

// deskStartRun is how the runner starts and waits for its command: the real one runs it; tests give a stand-in.
type deskStartRun struct {
	// command makes the start's command (sandbox.MacCommand).
	command func(s *sandbox.Start, profile, smolcorp string, env []string, program string, args ...string) (startedCmd, error)
	// serverIsOurs is the server check (sandbox.ServerIsOurs).
	serverIsOurs func(socket string, c sandbox.Canary) (int, error)
}

// startedCmd is the part of an *exec.Cmd the runner uses.
type startedCmd interface {
	Start() error
	Wait() error
	Kill() error
	SetOutput(stdout, stderr io.Writer)
}

var deskStartRunner = deskStartRun{command: macStartCommand, serverIsOurs: sandbox.ServerIsOurs}

// starter is desk s's start-only runner on a Mac: program is the bundled tmux, smolcorp the app's own program.
func (s *deskSandbox) starter(program, smolcorp string) tmuxStarter {
	return func(args []string) (string, int, error) {
		return deskStartRunner.start(s, program, smolcorp, args)
	}
}

func (r deskStartRun) start(s *deskSandbox, program, smolcorp string, args []string) (out string, server int, err error) {
	socket := filepath.Join(s.places.OwnSockets, deskSocketFile)
	refuse := func(words, why string) (string, int, error) {
		log.Printf("sandbox: desk %s didn't start: %s", s.at.ID, why)
		return "", 0, errors.New(words)
	}
	if err := makeDeskSocketFolder(socket); err != nil {
		return refuse(sandbox.NotOurServerRefusal, err.Error())
	}
	defer func() {
		if err != nil {
			removeDeskSocketFolder(socket)
		}
	}()
	profile := filepath.Join(hireFolder(supportDir, s.at.ID), "sandbox.sb")
	if err := sandbox.PrepareProfile(s.policy, profile, deskServices(s.places, socket)); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its profile: "+err.Error())
	}
	start, err := sandbox.NewStart(s.checks(profile))
	if err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its check: "+err.Error())
	}
	defer start.Close()
	defer start.Canary.Remove()
	// No start file: the server reads none of the owner's (~/.tmux.conf), whose commands would run in it (RT126SG2 B1).
	args = append([]string{"-f", os.DevNull}, args...)
	cmd, err := r.command(start, profile, smolcorp, withTmuxGuard(hireEnvironment(), ""), program, args...)
	if err != nil {
		return refuse(sandbox.SandboxExecRefusal, err.Error())
	}
	var stdout, stderr bytes.Buffer
	cmd.SetOutput(&stdout, &stderr)
	if err := cmd.Start(); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "sandbox-exec didn't run: "+err.Error())
	}
	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()
	if err := start.Wait(sandboxStartWait); err != nil {
		_ = cmd.Kill()
		<-ended
		return refuse(sandbox.Refusal("darwin"), err.Error()+": "+strings.TrimSpace(stderr.String()))
	}
	select {
	case err := <-ended:
		if err != nil {
			return refuse(sandbox.Refusal("darwin"), "tmux's start: "+err.Error()+": "+strings.TrimSpace(stderr.String()))
		}
	case <-time.After(sandboxStartWait):
		_ = cmd.Kill()
		<-ended
		return refuse(sandbox.Refusal("darwin"), "tmux's start didn't end")
	}
	if server, err = r.serverIsOurs(socket, start.Canary); err != nil {
		return refuse(sandbox.NotOurServerRefusal, err.Error())
	}
	return strings.TrimRight(stdout.String(), "\n"), server, nil
}

// deskServices is what a desk at pl may reach beyond its files (SA-M2), from the lists (SA-X2): its own socket; the
// system's sockets and the agents' (the GPG agent's places too: whether the owner's git signs with gpg isn't
// asked); the Mach services and IOKit classes on the lists; the local ports denied (the
// databases' too: AQ21's opt-in is SA-M12's).
func deskServices(pl sandbox.Places, own string) sandbox.Services {
	svc := sandbox.Services{OwnSocket: own, DeniedPorts: sandbox.DeniedPorts(false)}
	for _, s := range sandbox.Sockets("darwin") {
		paths := sandbox.SocketPaths([]sandbox.Socket{s}, pl.Home, os.Getenv, true, false)
		if strings.Contains(s.Name, "agent") {
			svc.Agents = append(svc.Agents, paths...)
		} else {
			svc.Sockets = append(svc.Sockets, paths...)
		}
	}
	for _, m := range sandbox.MachServices() {
		svc.Mach = append(svc.Mach, m.Name)
	}
	for _, c := range sandbox.IOKitClasses() {
		svc.IOKit = append(svc.IOKit, sandbox.IOKitOpen{Class: c.Class, UserClient: c.UserClient})
	}
	return svc
}

// macCmd is sandbox.MacCommand's command, as the runner uses it.
type macCmd struct{ *exec.Cmd }

func (c macCmd) SetOutput(stdout, stderr io.Writer) { c.Stdout, c.Stderr = stdout, stderr }

func (c macCmd) Kill() error {
	if c.Process == nil {
		return nil
	}
	return c.Process.Kill()
}

func macStartCommand(s *sandbox.Start, profile, smolcorp string, env []string, program string, args ...string) (startedCmd, error) {
	cmd, err := sandbox.MacCommand(s, profile, smolcorp, env, program, args...)
	if err != nil {
		return nil, err
	}
	return macCmd{cmd}, nil
}

// deskChecks are what desk s's `smolcorp sandbox enter` tries (§A5.2 steps 2-5): its profile and the guard file (and
// the worktree's .claude/settings.json, when there is one) can't be opened for writing; its brief (else its profile)
// reads; its status writes; a stand-in other server in the socket folder can't be reached. The canary is the start's.
func (s *deskSandbox) checks(profile string) sandbox.Checks {
	own := hireFolder(supportDir, s.at.ID)
	c := sandbox.Checks{Support: supportDir, NoWrite: []string{profile}, Read: []string{profile},
		Write: []string{filepath.Join(own, "STATUS.md")}, Other: sandbox.OtherServerIn(s.places.SocketParent)}
	for _, fence := range []string{filepath.Join(supportDir, hireGuardFile), filepath.Join(s.at.Root, ".claude", "settings.json")} {
		if info, err := os.Lstat(fence); err == nil && info.Mode().IsRegular() {
			c.NoWrite = append(c.NoWrite, fence)
		}
	}
	if info, err := os.Lstat(s.at.Brief); s.at.Brief != "" && err == nil && info.Mode().IsRegular() {
		c.Read = []string{s.at.Brief}
	}
	return c
}
