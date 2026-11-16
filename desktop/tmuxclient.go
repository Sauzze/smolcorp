package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// tmuxCallWait is how long one tmux call of a desk's may take.
const tmuxCallWait = 10 * time.Second

// tmuxGuardVariable is the bundled tmux's guard (third_party/tmux/guard.patch): the server pid a client may talk to,
// "unchecked", or "off" for the call that starts a server.
const tmuxGuardVariable = "SMOLCORP_TMUX_SERVER"

// tmuxUnchecked is the guard told of a server whose pid the core never checked: a desk started before its sandbox (an
// old desk, a computer with no sandbox yet), a team grid's, or a tmux of the owner's own. The guard's refusals stay on;
// only its pid check is off. A stock tmux ignores the variable.
const tmuxUnchecked = "unchecked"

// tmuxClientEnvironment is all a one-shot tmux client gets besides its guard (SANDBOX-PLAN §A3.1): nothing of the core's
// own environment, so no TMUX, no keys, no variable the core inherited reaches it, or through it a server.
var tmuxClientEnvironment = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "TERM=xterm-256color", "LANG=en_US.UTF-8"}

// errNoTmuxGuard is a one-shot call with no server for its guard to talk to: it isn't run.
var errNoTmuxGuard = errors.New("no checked tmux server to talk to")

// tmuxClient is the one way the core runs a one-shot tmux client, any call but the one that starts a server (SA-M7):
// program with args (-f /dev/null first, so even a call that would start a server reads none of the owner's start
// files), told the server it may talk to (guard: a pid the core checked, or tmuxUnchecked; never "off", which only a
// start gives, and never one the core inherited), with tmuxClientEnvironment alone, and stdin from /dev/null (an
// exec.Cmd's nil Stdin) unless the caller hands it a stream (load-buffer -, or an attach's terminal). Disclaimed (ST1),
// and stopped after wait; with wait 0, only when cancelled (an attach, which lasts as long as its window: attach.go).
// errNoTmuxGuard, and nothing run, for any other guard or no program.
func tmuxClient(wait time.Duration, program, guard string, args ...string) (*exec.Cmd, context.CancelFunc, error) {
	if pid, err := strconv.Atoi(guard); program == "" || guard != tmuxUnchecked && (err != nil || pid <= 1 || strconv.Itoa(pid) != guard) {
		return nil, nil, errNoTmuxGuard
	}
	ctx, cancel := context.WithCancel(context.Background())
	if wait > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), wait)
	}
	cmd := disclaim.Cmd(exec.CommandContext(ctx, program, append([]string{"-f", os.DevNull}, args...)...))
	cmd.Env = append(append([]string{}, tmuxClientEnvironment...), tmuxGuardVariable+"="+guard)
	cmd.Stdin = nil
	cmd.WaitDelay = time.Second // and not held open by anything it started
	return cmd, cancel, nil
}

// tmuxStartCall runs the tmux at path for a call that starts a server outside any sandbox, and for nothing else: a desk
// on a computer with no sandbox yet (hireTmux.startOnly), a plain hire's session on the default server (runTmux), the team
// grid's. Its guard is off for that call alone (a guarded client never starts a server), and its environment is a
// hire's (hireEnvironment: no TMUX, no session variables, no GM token, never the core's own guard variable), which the
// server and its first pane take. Every other call goes through tmuxClient.
func tmuxStartCall(path string) func(args []string) (string, int, bool) {
	return func(args []string) (string, int, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), tmuxCallWait)
		defer cancel()
		cmd := disclaim.Cmd(exec.CommandContext(ctx, path, args...))
		cmd.Env = withTmuxGuard(hireEnvironment(), "off")
		return tmuxAnswer(cmd.Output())
	}
}

// tmuxServerStart is args, a call that may start a server, with the guard variable taken out of the server's own
// environment first, so "off" never reaches its panes (RT126SAX3 nit 1).
func tmuxServerStart(args ...string) []string {
	return append([]string{"start-server", ";", "set-environment", "-gu", tmuxGuardVariable, ";"}, args...)
}

// withTmuxGuard is env without any SMOLCORP_TMUX_SERVER of its own, and with guard's when it isn't "".
func withTmuxGuard(env []string, guard string) []string {
	env = slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(strings.ToUpper(kv), tmuxGuardVariable+"=") })
	if guard != "" {
		env = append(env, tmuxGuardVariable+"="+guard)
	}
	return env
}

// tmuxAnswer is what a tmux call printed, its exit status, and whether it ran at all.
func tmuxAnswer(out []byte, err error) (string, int, bool) {
	if err == nil {
		return strings.TrimRight(string(out), "\n"), 0, true
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return strings.TrimRight(string(out), "\n"), exit.ExitCode(), true
	}
	return "", -1, false
}
