//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// `smolcorp attach` (SA-M8, SANDBOX-PLAN §A3.1): how a desk is shown in a terminal. Every window Smolcorp opens on a
// desk, every pane of the team grid, and the grid's own window run it as `exec '<app>/smolcorp' attach …`, so the tab
// ends with the attach and no shell is left in it for a desk to type into or read from. It finds the desk's socket and
// its server in the core's own records (the registry), never in its words, then:
//   - a desk in its sandbox: the guarded bundled tmux attaches inside that desk's sandbox (its profile, through
//     `smolcorp sandbox enter --attach <pid>` and that attach's own canary and rows), told the server the core checked
//     at the desk's start, so a server that isn't that one, or asks the client to read, write or run anything, is
//     refused (T4-T6), and typing into the terminal (TIOCSTI) stays inside;
//   - any other (a desk from before the sandbox, a plain hire's session on its own server, the team grid): its tmux
//     through tmuxClient, the one helper, told its server (a pid the core checked, or "unchecked"), with a clean
//     environment but for the terminal's TERM, and the terminal as its stdin.
// A hand-typed `smolcorp attach <desk>` does the same, by the desk's id or its name.

// attachUsage is what `smolcorp attach` says when its words aren't one of its forms.
const attachUsage = "usage: smolcorp attach [--read-only] <desk>"

// attachRequest is what `smolcorp attach` was asked: a desk by its id or name (read-only for a pane of the team grid);
// a plain hire's session, on the socket tmux made it on; or the team grid, by its server's pid.
type attachRequest struct {
	desk            string
	readOnly        bool
	session, socket string
	grid            int
}

// plainHireSession is a plain hire's session's name (openHire): hireSession, on a server of its own (SA-M10), or the
// name one had on the default server before.
var plainHireSession = regexp.MustCompile(`^(` + hireSession + `|smolcorp-[0-9a-z]{8})$`)

// parseAttach reads `smolcorp attach`'s words: [--read-only] <desk>; --session <name> --socket <path>, a plain hire's;
// or --team-grid <pid>. The last two are the lines Smolcorp writes itself.
func parseAttach(args []string) (attachRequest, error) {
	var r attachRequest
	switch {
	case len(args) == 1 && !strings.HasPrefix(args[0], "-"):
		r.desk = args[0]
	case len(args) == 2 && args[0] == "--read-only" && !strings.HasPrefix(args[1], "-"):
		r.desk, r.readOnly = args[1], true
	case len(args) == 4 && args[0] == "--session" && args[2] == "--socket":
		r.session, r.socket = args[1], args[3]
		if !plainHireSession.MatchString(r.session) || !filepath.IsAbs(r.socket) || filepath.Clean(r.socket) != r.socket {
			return r, errors.New("that isn't a hire's session of Smolcorp's")
		}
	case len(args) == 2 && args[0] == "--team-grid":
		pid, err := strconv.Atoi(args[1])
		if err != nil || pid <= 1 || strconv.Itoa(pid) != args[1] {
			return r, errors.New("that isn't the team grid's server")
		}
		r.grid = pid
	default:
		return r, errors.New(attachUsage)
	}
	return r, nil
}

// RunAttach is `smolcorp attach`, on the terminal it runs in: tmux's exit status, or 1 with why on stderr.
func RunAttach(args []string) int { return runAttach(args, os.Stdin, os.Stdout, os.Stderr) }

func runAttach(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	useTestCopySupport()
	r, err := parseAttach(args)
	if err == nil {
		var a attaching
		if a, err = attachCommand(r); err == nil {
			defer a.done()
			a.cmd.Stdin, a.cmd.Stdout, a.cmd.Stderr = stdin, stdout, stderr
			return runAttached(a.cmd, a.started)
		}
	}
	fmt.Fprintln(stderr, "smolcorp attach:", err)
	return 1
}

// useTestCopySupport points the support folder at a test copy's, when this runs in one's window (its port and folder,
// as windowScriptLines exports them).
func useTestCopySupport() {
	if dir := os.Getenv(testSupportVariable); testCopyPort() != 0 && filepath.IsAbs(dir) {
		useSupportDir(filepath.Clean(dir))
	}
}

// attaching is an attach to run: its command, not started; what to do once it has started, before it may go on (a
// sandboxed attach's check: nil for none); and what to do once it has ended.
type attaching struct {
	cmd     *exec.Cmd
	started func() error
	done    func()
}

// attachCommand is the attach r asks for, or why there is none, in the words the terminal shows.
func attachCommand(r attachRequest) (attaching, error) {
	switch {
	case r.grid != 0:
		return attachClient(tmuxPath(), strconv.Itoa(r.grid), "-S", teamGridSocket(testCopyPort()), "attach-session", "-r", "-t", "="+teamGridSession)
	case r.session != "":
		return attachClient(tmuxPath(), tmuxUnchecked, "-S", r.socket, "attach-session", "-t", "="+r.session)
	}
	h, err := attachedDesk(r.desk)
	if err != nil {
		return attaching{}, err
	}
	tmux := hireOwnTmux(h, testCopyPort(), nil)
	if tmux == nil || tmux.program == "" {
		return attaching{}, errors.New(calledHire(h) + " has no tmux session of Smolcorp's to show.")
	}
	guard := tmux.guard()
	if guard == "" {
		return attaching{}, errors.New("Smolcorp never checked " + calledHire(h) + "'s tmux server, so it doesn't show it.")
	}
	args := []string{"-S", tmux.socket, "attach-session"}
	if r.readOnly {
		args = append(args, "-r")
	}
	args = append(args, "-t", "="+hireSession)
	if tmux.sandboxed {
		return attachInSandbox(h, tmux.program, tmux.server, args)
	}
	return attachClient(tmux.program, guard, args...)
}

// attachedDesk is the desk asked for: by its id, else by its name or nickname when one desk alone has it, else, for a
// hire with no nickname of its own, by the office's name for its desk (FXGMNICKR1 O1). This command runs outside the
// app, with no sessions to look at: the office's name is the saved layout's for where the hire started, as `gm team`
// shows it for a hire away from its desk (nameRows). A desk that was fired, or failed, isn't shown.
func attachedDesk(asked string) (Hire, error) {
	hires := readRegistry(registryPath(), false).hires
	i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == asked })
	var team map[string]TeamMember
	for _, byOffice := range []bool{false, true} {
		if i >= 0 {
			break
		}
		if byOffice {
			team = savedConfig().Layout.Room.Team
		}
		for j, h := range hires {
			named := strings.EqualFold(h.Name, asked) || strings.EqualFold(h.Nickname, asked)
			if byOffice {
				office := officeName(team, h.storedStart()) // its desk's name, by its folder's name: compared only
				named = h.Nickname == "" && office != "" && strings.EqualFold(office, asked)
			}
			if h.State != "fired" && h.State != "failed" && named {
				if i >= 0 {
					return Hire{}, fmt.Errorf("more than one desk is called %s: name it by its id", asked)
				}
				i = j
			}
		}
	}
	if i < 0 {
		return Hire{}, fmt.Errorf("no desk is called %s here (smolcorp desks lists them)", asked)
	}
	if h := hires[i]; h.State != "fired" && h.State != "failed" {
		return h, nil
	}
	return Hire{}, errors.New(calledHire(hires[i]) + " isn't on the team any more.")
}

// attachClient is an attach of the tmux at program, outside any sandbox, through the one helper: told guard, its
// environment clean but for the terminal's TERM, with no time limit.
func attachClient(program, guard string, args ...string) (attaching, error) {
	cmd, cancel, err := tmuxClient(0, program, guard, args...)
	if err != nil {
		return attaching{}, errors.New("there's no tmux here to show it with")
	}
	cmd.Env = withTmuxGuard(attachEnvironment(), clientGuard(guard))
	return attaching{cmd: cmd, done: cancel}, nil
}

// attachEnvironment is an attach client's whole environment, its guard aside: a one-shot client's
// (tmuxClientEnvironment), with the terminal's own TERM, which an attach draws for.
func attachEnvironment() []string {
	env := slices.Clone(tmuxClientEnvironment)
	if term := os.Getenv("TERM"); term != "" && !strings.ContainsAny(term, "\x00\n") {
		env = append(slices.DeleteFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "TERM=") }), "TERM="+term)
	}
	return env
}

// runAttached starts cmd, then, once started, runs started (nil: nothing), waits for it and gives its exit status.
// While it runs, a signal this program gets (the window closed, an interrupt) goes on to it, and this program stays
// until it has ended: whatever is left to do after it (an attach's canary is gone already) is done.
func runAttached(cmd *exec.Cmd, started func() error) int {
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(cmd.Stderr, "smolcorp attach: tmux didn't start:", err)
		return 1
	}
	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()
	if started != nil {
		if err := started(); err != nil {
			_ = cmd.Process.Kill()
			<-ended
			fmt.Fprintln(cmd.Stderr, "smolcorp attach:", err)
			return 1
		}
	}
	for {
		select {
		case sig := <-signals:
			_ = cmd.Process.Signal(sig)
		case err := <-ended:
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return max(exit.ExitCode(), 1)
			} else if err != nil {
				return 1
			}
			return 0
		}
	}
}
