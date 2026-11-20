package desktop

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/acp"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// An ACP desk on a Mac, in its sandbox (SA-M11, SANDBOX-PLAN §A3.4's ACP row). The core holds the agent's process
// itself, with no tmux, so the agent's start, and each terminal the agent asks for (terminal/create), goes through the
// sandbox's runner as a desk's tmux server's start does (desksandbox_darwin.go), each with a canary and a handshake of
// its own. In order, each step refusing the start:
//
//  1. the desk's profile, from its Policy and what it may reach (deskServices, with no socket of its own), written
//     again to <support>/hires/<id>/sandbox.sb;
//  2. a fresh canary, nonce, handshake and stand-in other server (sandbox.NewStart, with the desk's checks), that
//     server in the socket folder, made or checked as this user's alone;
//  3. `smolcorp-spawn /usr/bin/sandbox-exec -f <profile> <smolcorp> sandbox enter … -- <program> …`
//     (sandbox.MacCommand), with the agent's environment;
//  4. `enter`'s full, passing report for this start;
//  5. the process the core started, its own child not yet waited for, in this start's sandbox: macOS, asked by its
//     pid, says it may not read the canary (sandbox.ChildIsOurs). A desk with no server has no socket to check.
//
// A step that fails kills what started: the agent isn't started (acp.Start fails with §A5.2's words), a terminal isn't
// made (the agent is told). Nothing falls back to a start outside the sandbox. The canary goes once its start is
// checked.
//
// Hiring an ACP desk (ACP5) makes its sandbox (makeDeskSandbox), then an acpSandbox, and passes Runner() in its
// acp.Command. The desk that answers the agent (ACP4b) checks the session and the guard, then hands fs/* to the
// acpSandbox's acpFiles and terminal/* to its terminals, and calls close when the desk ends.

// acpSandbox is one ACP desk's sandbox, for its agent's start, its files and its terminals.
type acpSandbox struct {
	acpFiles
	smolcorp string // Smolcorp's own program, whole, which runs `sandbox enter`

	mu     sync.Mutex
	env    []string // the agent's environment, as its start held: its terminals start from it
	terms  map[string]*acpTerminal
	closed bool
}

var (
	_ acp.FileHandler     = (*acpSandbox)(nil)
	_ acp.TerminalHandler = (*acpSandbox)(nil)
)

const (
	// acpTerminalsMost is how many terminals an agent may hold at once.
	acpTerminalsMost = 16
	// acpOutputDefault and acpOutputMost are what a terminal keeps of its output (its last bytes), by default and at
	// most, whatever the agent asks.
	acpOutputDefault = 1 << 20
	acpOutputMost    = 4 << 20
)

// newACPSandbox is desk's sandbox for an ACP agent, smolcorp being the app's own program.
func newACPSandbox(desk *deskSandbox, smolcorp string) *acpSandbox {
	return &acpSandbox{acpFiles: acpFiles{desk: desk}, smolcorp: smolcorp, terms: map[string]*acpTerminal{}}
}

// acpChildIsOurs is step 5: sandbox.ChildIsOurs, a stand-in in the tests.
var acpChildIsOurs = sandbox.ChildIsOurs

// command is steps 1 to 3 for program with args, in env: the command to start, and its start, which the caller
// closes and whose canary it removes.
func (sb *acpSandbox) command(program string, args, env []string) (*exec.Cmd, *sandbox.Start, error) {
	desk := sb.desk
	refuse := func(words, why string) (*exec.Cmd, *sandbox.Start, error) {
		log.Printf("sandbox: ACP desk %s: %s didn't start: %s", desk.at.ID, program, why)
		return nil, nil, errors.New(words)
	}
	profile := filepath.Join(hireFolder(supportDir, desk.at.ID), "sandbox.sb")
	if err := desk.prepareProfile(profile, deskServices(desk.places, "")); err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its profile: "+err.Error())
	}
	// The stand-in other server listens in the socket folder, as for a desk with a tmux server: made, or checked,
	// this user's alone.
	if !ownSocketFolder(desk.places.SocketParent) {
		return refuse(sandbox.NotOurServerRefusal, "the socket folder "+desk.places.SocketParent+" isn't this user's alone")
	}
	start, err := sandbox.NewStart(desk.checks(profile))
	if err != nil {
		return refuse(sandbox.SandboxExecRefusal, "its check: "+err.Error())
	}
	cmd, err := sandbox.MacCommand(start, profile, sb.smolcorp, env, program, args...)
	if err == nil {
		err = cmd.Err // disclaim.Cmd's: nothing may start
	}
	if err != nil {
		start.Close()
		start.Canary.Remove()
		return refuse(sandbox.SandboxExecRefusal, err.Error())
	}
	return cmd, start, nil
}

// started is steps 4 and 5, for the process pid that start's command started.
func (sb *acpSandbox) started(start *sandbox.Start, pid int) error {
	err := start.Wait(sandboxStartWait)
	if err == nil {
		err = acpChildIsOurs(pid, start.Canary)
	}
	if err != nil {
		log.Printf("sandbox: ACP desk %s: pid %d isn't in its sandbox: %v", sb.desk.at.ID, pid, err)
		return errors.New(sandbox.Refusal("darwin"))
	}
	return nil
}

// Runner is the agent's start through the sandbox, for its acp.Command: one start each.
func (sb *acpSandbox) Runner() acp.Runner { return &acpRunner{sb: sb} }

// acpRunner is one start of the agent: acp.Start asks Command, then Started, then Close.
type acpRunner struct {
	sb    *acpSandbox
	start *sandbox.Start
	env   []string
}

func (r *acpRunner) Command(path string, args, env []string) (*exec.Cmd, error) {
	cmd, start, err := r.sb.command(path, args, env)
	if err != nil {
		return nil, err
	}
	r.start, r.env = start, slices.Clone(env)
	return cmd, nil
}

func (r *acpRunner) Started(pid int) error {
	if err := r.sb.started(r.start, pid); err != nil {
		return err
	}
	r.sb.mu.Lock()
	r.sb.env = r.env
	r.sb.mu.Unlock()
	return nil
}

func (r *acpRunner) Close() {
	if r.start != nil {
		r.start.Close()
		r.start.Canary.Remove()
	}
}

// acpTerminal is one terminal an agent asked for: a command run in the desk's sandbox, its output kept.
type acpTerminal struct {
	session string
	cmd     *exec.Cmd
	out     *acpOutput
	done    chan struct{} // closed once the command has been waited for
	status  acp.TerminalExitStatus
}

// kill ends the terminal's command and what it started, while it hasn't been waited for (so its group is its own).
func (t *acpTerminal) kill() {
	select {
	case <-t.done:
	default:
		syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
		t.cmd.Process.Kill()
	}
}

// CreateTerminal is terminal/create: req's command, run in the desk's sandbox by the steps above, in req.Cwd (the
// desk's folder when it's not given; the Policy must let the desk read it), with the agent's environment and req.Env
// (none of Smolcorp's own variables, nor the loader's). A command that isn't a whole path is looked for on that
// environment's PATH, in folders the Policy lets the desk read.
func (sb *acpSandbox) CreateTerminal(_ context.Context, req acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	sb.mu.Lock()
	env, full := sb.env, sb.closed || len(sb.terms) >= acpTerminalsMost
	sb.mu.Unlock()
	if env == nil || full {
		return acp.CreateTerminalResponse{}, acpRefused("open another terminal now")
	}
	cwd := sb.desk.places.Root
	if req.Cwd != "" {
		var ok bool
		if cwd, ok = acpResolved(req.Cwd, false); !ok || !sb.allows(cwd, sandbox.Read) {
			return acp.CreateTerminalResponse{}, acpRefused("work in " + req.Cwd)
		}
	}
	env = acpTerminalEnv(env, req.Env)
	program := sb.program(req.Command, cwd, env)
	if program == "" {
		return acp.CreateTerminalResponse{}, acpRefused("run " + req.Command + ": it isn't found")
	}
	most := acpOutputDefault
	if req.OutputByteLimit != nil && *req.OutputByteLimit >= 0 {
		most = int(min(*req.OutputByteLimit, acpOutputMost))
	}
	cmd, start, err := sb.command(program, req.Args, env)
	if err != nil {
		return acp.CreateTerminalResponse{}, acpRefused("run " + req.Command + ": " + err.Error())
	}
	defer start.Canary.Remove()
	defer start.Close()
	out := &acpOutput{most: most}
	cmd.Dir, cmd.Stdout, cmd.Stderr = cwd, out, out
	cmd.WaitDelay = time.Second // something it left behind can't hold the wait open
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Printf("sandbox: ACP desk %s: %s didn't run: %v", sb.desk.at.ID, program, err)
		return acp.CreateTerminalResponse{}, acpRefused("run " + req.Command)
	}
	// Checked before anything waits for it: until then its pid, and its group, are its own.
	if err := sb.started(start, cmd.Process.Pid); err != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Process.Kill()
		cmd.Wait()
		return acp.CreateTerminalResponse{}, acpRefused("run " + req.Command + ": " + err.Error())
	}
	t := &acpTerminal{session: req.SessionID, cmd: cmd, out: out, done: make(chan struct{})}
	go func() {
		cmd.Wait()
		t.status = acpExitStatus(cmd.ProcessState)
		close(t.done)
	}()
	id := "term-" + randomHex(8)
	sb.mu.Lock()
	closed := sb.closed
	if !closed {
		sb.terms[id] = t
	}
	sb.mu.Unlock()
	if closed {
		t.kill()
		<-t.done
		return acp.CreateTerminalResponse{}, acpRefused("open another terminal now")
	}
	return acp.CreateTerminalResponse{TerminalID: id}, nil
}

// program is command as a whole path: as it is when whole, from cwd when it names a folder, else the first program of
// that name in env's PATH whose folder the Policy lets the desk read; "" when there's none.
func (sb *acpSandbox) program(command, cwd string, env []string) string {
	switch {
	case command == "":
		return ""
	case filepath.IsAbs(command):
		return filepath.Clean(command)
	case strings.Contains(command, "/"):
		return filepath.Join(cwd, command)
	}
	var path string
	for _, kv := range env {
		if value, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = value
		}
	}
	for _, folder := range filepath.SplitList(path) {
		resolved, ok := acpResolved(folder, false)
		if !ok || sb.desk.policy == nil {
			continue
		}
		if verdict, _ := sb.desk.policy.Decide(resolved, sandbox.Read); verdict != sandbox.Allow {
			continue
		}
		at := filepath.Join(resolved, command)
		if info, err := os.Stat(at); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return at
		}
	}
	return ""
}

// acpTerminalEnv is env with the agent's variables added, each replacing one of the same name. Left out: a name that
// isn't one, Smolcorp's own (SMOLCORP_…: the start's nonce, the tmux guard) and the loader's (DYLD_…), which would
// reach the programs that run before the sandbox holds.
func acpTerminalEnv(env []string, add []acp.EnvVariable) []string {
	env = slices.Clone(env)
	for _, v := range add {
		if v.Name == "" || strings.ContainsAny(v.Name, "=\x00") || strings.Contains(v.Value, "\x00") ||
			strings.HasPrefix(v.Name, "SMOLCORP_") || strings.HasPrefix(v.Name, "DYLD_") {
			continue
		}
		env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, v.Name+"=") })
		env = append(env, v.Name+"="+v.Value)
	}
	return env
}

// acpExitStatus is how a terminal's command ended, as ACP says it.
func acpExitStatus(state *os.ProcessState) acp.TerminalExitStatus {
	if state == nil {
		code := -1
		return acp.TerminalExitStatus{ExitCode: &code}
	}
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		name := unix.SignalName(ws.Signal())
		return acp.TerminalExitStatus{Signal: &name}
	}
	code := state.ExitCode()
	return acp.TerminalExitStatus{ExitCode: &code}
}

// terminal is the agent's terminal req names, made in req's session.
func (sb *acpSandbox) terminal(req acp.TerminalRequest) (*acpTerminal, error) {
	sb.mu.Lock()
	t := sb.terms[req.TerminalID]
	sb.mu.Unlock()
	if t == nil || t.session != req.SessionID {
		return nil, &acp.RPCError{Code: acp.CodeInvalidParams, Message: "no such terminal"}
	}
	return t, nil
}

// TerminalOutput is terminal/output: what the terminal kept, and how it ended once it has.
func (sb *acpSandbox) TerminalOutput(_ context.Context, req acp.TerminalRequest) (acp.TerminalOutputResponse, error) {
	t, err := sb.terminal(req)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	resp := acp.TerminalOutputResponse{}
	resp.Output, resp.Truncated = t.out.text()
	select {
	case <-t.done:
		status := t.status
		resp.ExitStatus = &status
	default:
	}
	return resp, nil
}

// WaitForTerminalExit is terminal/wait_for_exit: how the terminal's command ended, once it has.
func (sb *acpSandbox) WaitForTerminalExit(ctx context.Context, req acp.TerminalRequest) (acp.TerminalExitStatus, error) {
	t, err := sb.terminal(req)
	if err != nil {
		return acp.TerminalExitStatus{}, err
	}
	select {
	case <-t.done:
		return t.status, nil
	case <-ctx.Done():
		return acp.TerminalExitStatus{}, ctx.Err()
	}
}

// KillTerminal is terminal/kill: the command ends; the terminal stays until released.
func (sb *acpSandbox) KillTerminal(_ context.Context, req acp.TerminalRequest) error {
	t, err := sb.terminal(req)
	if err != nil {
		return err
	}
	t.kill()
	return nil
}

// ReleaseTerminal is terminal/release: the command ends, and the terminal is gone.
func (sb *acpSandbox) ReleaseTerminal(_ context.Context, req acp.TerminalRequest) error {
	t, err := sb.terminal(req)
	if err != nil {
		return err
	}
	sb.mu.Lock()
	delete(sb.terms, req.TerminalID)
	sb.mu.Unlock()
	t.kill()
	return nil
}

// close ends every terminal and takes no more: when the desk ends. Then every program still running in the desk's
// sandbox is ended, and seen ended (FXFIREREAPR1): a terminal's group kill doesn't reach what it started under setsid.
// "", or why the desk's sandbox couldn't be shown empty, for the owner.
func (sb *acpSandbox) close() string {
	sb.mu.Lock()
	sb.closed = true
	terms := sb.terms
	sb.terms = map[string]*acpTerminal{}
	sb.mu.Unlock()
	for _, t := range terms {
		t.kill()
		<-t.done
	}
	if left, sure := reapSandboxAt(hireFolder(supportDir, sb.desk.at.ID), true); len(left) != 0 || !sure {
		log.Printf("sandbox: ACP desk %s at its stop: its processes %v left, every process answered: %t", sb.desk.at.ID, left, sure)
		return reapLeftWords(Hire{Name: "the agent"}, left)
	}
	return ""
}

// acpOutput is a terminal's output: its last most bytes, cut at a character's start.
type acpOutput struct {
	mu        sync.Mutex
	most      int
	kept      []byte
	truncated bool
}

func (o *acpOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.kept = append(o.kept, p...)
	if over := len(o.kept) - o.most; over > 0 {
		for over < len(o.kept) && !utf8.RuneStart(o.kept[over]) {
			over++
		}
		o.kept = o.kept[:copy(o.kept, o.kept[over:])]
		o.truncated = true
	}
	return len(p), nil
}

func (o *acpOutput) text() (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return string(o.kept), o.truncated
}
