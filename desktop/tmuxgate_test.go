//go:build !windows

package desktop

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The release gate's test (TMUXGATE1, briefs/FXTMUXGATE1R1-report.md): the core never runs the bundled tmux without a
// SMOLCORP_TMUX_SERVER it gave itself. Its stand-in is a guarded tmux in all that matters here: it exits 98 without the
// variable, as the guard does, and it writes one line per run (the variable, whether stdin was /dev/null, the names in
// its environment, its words). The test drives the real core's every tmux site (a sandboxed desk's calls, an old desk's,
// a new desk's start outside the sandbox, a tab's calls and its paste, the preview's look, the window closer, mail, the
// respawn watch, a plain hire's session on its own server, the team grid) with that stand-in as both the bundled tmux and the one this
// system picks, and the recorder reports each run the guard stopped. With SMOLCORP_TMUXGATE_CONTROL=1 it then makes
// one start without the variable through the same recorder, which must report it. It also holds the grep row's reading
// of the core's code (tmuxclient_test.go), so a site it doesn't drive fails it too, and it drives a desk's start-only
// runner with calls that aren't its start (RT126SAM7R2 B1′), which must run nothing.

// tmuxGateRun is one run of the stand-in, as its line says it.
type tmuxGateRun struct {
	guard, stdin string
	env          []string
	args         string
}

// tmuxGateStdinCheck is the stand-in's look at its stdin: "null" when it is /dev/null, "stream" (read to its end)
// otherwise. On a Mac it asks fd 0 itself (stat with no file is fstat of stdin) and compares it with /dev/null: a look
// by name through /dev/stdin or /dev/fd/0 fails now and then with "Bad file descriptor" while other programs start
// (FXTMUXSTDIN: 0.2% of runs with /dev/null on stdin), which -ef took for a stream. Anything it can't tell is a stream.
// Elsewhere, -ef: Linux's /dev/stdin is /proc's link to the descriptor itself.
var tmuxGateStdinCheck = map[bool]string{
	true: `in=$(/usr/bin/stat -f '%d:%i' 2>/dev/null); null=$(/usr/bin/stat -f '%d:%i' /dev/null 2>/dev/null)
if [ -n "$in" ] && [ "$in" = "$null" ]; then stdin=null; else stdin=stream; /bin/cat >/dev/null; fi`,
	false: `if [ /dev/stdin -ef /dev/null ]; then stdin=null; else stdin=stream; /bin/cat >/dev/null; fi`,
}[runtime.GOOS == "darwin"]

// tmuxGateStandIn writes the stand-in guarded tmux into a folder of its own and gives its path and its log's.
func tmuxGateStandIn(t *testing.T) (program, log string) {
	t.Helper()
	dir := t.TempDir()
	program, log = filepath.Join(dir, "tmux"), filepath.Join(dir, "runs")
	script := `#!/bin/sh
names=$(/usr/bin/env | /usr/bin/sed -e 's/=.*//' -e '/^PWD$/d' -e '/^SHLVL$/d' -e '/^_$/d' -e '/^OLDPWD$/d' | /usr/bin/sort | /usr/bin/tr '\n' ',')
` + tmuxGateStdinCheck + `
printf '%s|%s|%s|%s\n' "${SMOLCORP_TMUX_SERVER-UNSET}" "$stdin" "$names" "$*" >> '` + log + `'
[ -n "${SMOLCORP_TMUX_SERVER:-}" ] || exit 98
case "$*" in
*'#{pane_id} #{pane_pid} #{socket_path}'*) echo "%3 4242 $(dirname "$0")/s" ;;
*'#{pane_id} #{socket_path}'*) echo "%7 $(dirname "$0")/default" ;;
*'#{pid}'*) echo 5252 ;;
*'#{pane_tty} #{pane_pid}'*) echo "/dev/ttys999 4242" ;;
*'#{pane_id} #{pane_pid}'*) echo "%1 4242" ;;
*'#{pane_in_mode} #{bracket_paste_flag}'*) echo "0 1" ;;
*'#{pane_pid}'*) echo 4242 ;;
*'#{pane_id}'*) echo "%1" ;;
*list-clients*) echo /dev/ttys998 ;;
*if-shell*) echo pasted ;;
esac
exit 0
`
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return program, log
}

// tmuxGateRuns reads the stand-in's log from line from on.
func tmuxGateRuns(t *testing.T, log string, from int) []tmuxGateRun {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var runs []tmuxGateRun
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if i < from || len(parts) != 4 {
			continue
		}
		runs = append(runs, tmuxGateRun{guard: parts[0], stdin: parts[1], env: strings.Split(strings.TrimSuffix(parts[2], ","), ","), args: parts[3]})
	}
	return runs
}

// listenIn makes a unix socket at path, its folder first, as a server's would be, for the core's own-socket checks.
func listenIn(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	listenAt(t, path)
}

// tmuxGateRig points the core's tmux at the stand-in and its sockets at short temporary folders: a sandboxed desk
// "ab12" (its server checked at 5151), an old desk "cd34" from before the sandbox (its row's tmux, no checked server),
// both rows naming the stand-in, both sockets there. Nothing of the core's own SMOLCORP_TMUX_SERVER is left.
func tmuxGateRig(t *testing.T) (program, log string, sandboxed, old Hire) {
	t.Helper()
	shortSupport(t)
	deskSockets(t)
	t.Setenv(tmuxGuardVariable, "")
	os.Unsetenv(tmuxGuardVariable)
	program, log = tmuxGateStandIn(t)
	folder, err := os.MkdirTemp("", "tg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(folder) })
	savedBundled, savedPath, savedFolder := bundledTmux, tmuxPath, hireSocketFolder
	t.Cleanup(func() { bundledTmux, tmuxPath, hireSocketFolder = savedBundled, savedPath, savedFolder })
	bundledTmux = func() string { return program }
	tmuxPath = func() string { return program }
	hireSocketFolder = func() string { return filepath.Join(physical(folder), "t") }
	if !ownSocketFolder(hireSocketFolder()) || !ownSocketFolder(deskSocketParent()) {
		t.Fatal("the socket folders")
	}
	sandboxed = Hire{ID: "ab12", Name: "Sandboxed", State: "working", TmuxSocket: deskSocketPath("ab12", 0), TmuxProgram: program, TmuxServer: 5151, TmuxPane: 4242}
	old = Hire{ID: "cd34", Name: "Old", State: "working", TmuxSocket: filepath.Join(hireSocketFolder(), "smolcorp-cd34"), TmuxProgram: program, TmuxPane: 4242}
	listenIn(t, sandboxed.TmuxSocket)
	listenIn(t, old.TmuxSocket)
	writeRegistryHires(t, sandboxed, old)
	return program, log, sandboxed, old
}

// driveEveryTmuxSite runs each of the core's tmux sites once on the rig's desks.
func driveEveryTmuxSite(t *testing.T, sandboxed, old Hire) {
	t.Helper()
	for _, h := range []Hire{sandboxed, old} {
		tmux := hireOwnTmux(h, 0, nil)
		if tmux == nil {
			t.Fatalf("%s: no tmux", h.ID)
		}
		tmux.alive()
		tmux.panePID()
		tmux.paneState()
		tmux.viewers()
		tmux.clientTTYs()
		tmux.detachViewers()
		tmux.pasteState()
		tmux.paste("hello")
		tmux.submit()
		hireClientTTY(h.TmuxSocket, 0)
		detachHireViewers(h, nil)
		pane := commandTab{kind: "tmux", id: "%1", socket: h.TmuxSocket}
		pane.alive()
		_, _ = pane.output("display-message", "-p", "-t", "%1", "#{pane_id}")
		_, _ = pane.phoneText("one line", false)
		_, _ = pane.phoneText("two\nlines", true)
		previewRootOf("tmux:%1@"+h.TmuxSocket, nil)
		tmux.end()
		tmux.endServer()
	}
	hiresToBackgroundClosing([]Hire{sandboxed, old}, &closerStandIn{}, nil)
	markRespawned(nil)
	// A tab on a tmux of the owner's own.
	(commandTab{kind: "tmux", id: "%1", socket: filepath.Join(t.TempDir(), "default")}).alive()
	// A new desk outside the sandbox (a computer with none yet): its start, then a call.
	if fresh := newHireTmux("ef56", 0, nil); fresh == nil {
		t.Error("no tmux for a new desk")
	} else if _, _, _, err := fresh.start(t.TempDir(), []string{"/bin/echo", "hi"}); err != nil {
		t.Errorf("a new desk's start: %v", err)
	} else {
		fresh.alive()
	}
	// A plain hire's session on a server of its own (SA-M10), and its end when its window doesn't open.
	if err := openHire([]string{"/bin/echo", "hi"}, t.TempDir(), hireOpener{}, false, true); err != nil {
		t.Errorf("a plain hire's session: %v", err)
	}
	noWindow := hireOpener{open: func(string, []string) (string, error) { return "", errUnsupported }}
	_ = openHire([]string{"/bin/echo", "hi"}, t.TempDir(), noWindow, true, true)
	// `smolcorp attach` outside a sandbox (SA-M8): an old desk's, a plain hire's, the team grid's; a sandboxed desk's is
	// refused here (no profile: nothing runs), and is attach_darwin_test.go's inside its sandbox.
	for _, words := range [][]string{{old.ID}, {"--read-only", old.ID}, {"--session", hireSession, "--socket", filepath.Join(t.TempDir(), "default")},
		{"--session", "smolcorp-0a1b2c3d", "--socket", filepath.Join(t.TempDir(), "default")}, {"--team-grid", "5252"}, {sandboxed.ID}} {
		runAttach(words, nil, io.Discard, io.Discard)
	}
	// The team grid, under a Mac app that opens no window.
	showTeamGrid(hostAnswering(func(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{}, errUnsupported }),
		[]string{sandboxed.ID, old.ID}, "terminal", 0)
}

func TestNoExecOfTheBundledTmuxWithoutSmolcorpTmuxServer(t *testing.T) {
	t.Run("the stand-in tells /dev/null from a stream", tmuxGateStandInReadsStdin)
	program, log, sandboxed, old := tmuxGateRig(t)
	// The recorder: every run the guard stopped, reported in the gate's words; nothing else is.
	seen := 0
	record := func() []tmuxGateRun {
		runs := tmuxGateRuns(t, log, seen)
		seen += len(runs)
		for _, run := range runs {
			if run.guard == "UNSET" || run.guard == "" {
				t.Error(tmuxControlLine)
			}
		}
		return runs
	}
	driveEveryTmuxSite(t, sandboxed, old)
	runs := record()
	// Each site ran, and each run was told what the core gave it: a start "off", a sandboxed desk's call its checked
	// server, any other call unchecked but for the sandboxed desk's server (FXTMUXDIR: "unchecked:5151", as its row
	// records it) or the grid's own server; one-shot calls with a clean environment and stdin from /dev/null, but for a
	// paste's buffer.
	clean := []string{"LANG", "PATH", tmuxGuardVariable, "TERM"}
	unchecked := tmuxUnchecked + ":5151"
	for _, want := range []string{"5151", unchecked, "off", "5252"} {
		if !slices.ContainsFunc(runs, func(r tmuxGateRun) bool { return r.guard == want }) {
			t.Errorf("no run was told %q: %+v", want, runs)
		}
	}
	for _, run := range runs {
		start := strings.Contains(run.args, "new-session")
		switch {
		case start && run.guard != "off":
			t.Errorf("a start told %q: %q", run.guard, run.args)
		case !start && run.guard == "off":
			t.Errorf("a one-shot call told off: %q", run.args)
		case run.guard == tmuxUnchecked:
			t.Errorf("a call told plain unchecked while a sandboxed desk's server is recorded: %q", run.args)
		case !start && !slices.Equal(run.env, clean):
			// Only the names that differ: the core's whole environment would bury them (RT126SAM7 nit 7).
			more := slices.DeleteFunc(slices.Clone(run.env), func(name string) bool { return slices.Contains(clean, name) })
			fewer := slices.DeleteFunc(slices.Clone(clean), func(name string) bool { return slices.Contains(run.env, name) })
			t.Errorf("a one-shot call's environment has %q besides %q, and lacks %q: %q", more, clean, fewer, run.args)
		case !start && !strings.HasPrefix(run.args, "-f /dev/null "):
			t.Errorf("a one-shot call without -f /dev/null: %q", run.args)
		case strings.Contains(run.args, sandboxed.TmuxSocket) && !start && run.guard != "5151":
			t.Errorf("a sandboxed desk's call told %q: %q", run.guard, run.args)
		case strings.Contains(run.args, old.TmuxSocket+" ") && !strings.Contains(run.args, "smolcorp-team") && run.guard != unchecked:
			t.Errorf("an old desk's call told %q: %q", run.guard, run.args)
		}
		if (run.stdin == "stream") != strings.Contains(run.args, "load-buffer") {
			t.Errorf("stdin %s: %q", run.stdin, run.args)
		}
	}
	if len(runs) < 40 {
		t.Errorf("only %d runs: a site didn't run", len(runs))
	}
	// A desk's start-only runner refuses any call but its start, with nothing run (RT126SAM7R2 B1′): the reviewer's
	// window, on the old desk's tmux and a new desk's, a start with another command after it, a start on another socket.
	for _, tmux := range []*hireTmux{hireOwnTmux(old, 0, nil), newHireTmux("ef57", 0, nil)} {
		if tmux == nil || tmux.startOnly == nil {
			t.Fatalf("no start-only runner: %+v", tmux)
		}
		probes := map[string]func() error{
			"the reviewer's window": func() error { return tmux.reviewersWindow(t.TempDir()) },
			"a start, then another command": func() error {
				_, _, err := tmux.startOnly(tmux.command(tmuxServerStart("new-session", "-d", "/bin/echo", "hi", ";", "kill-server")...))
				return err
			},
			"a start on another socket": func() error {
				_, _, err := tmux.startOnly(append([]string{"-S", sandboxed.TmuxSocket}, tmuxServerStart("new-session", "-d", "/bin/echo", "hi")...))
				return err
			},
		}
		for name, probe := range probes {
			err := probe()
			if ran := record(); err == nil || len(ran) != 0 {
				t.Errorf("%s through %s's start-only runner: %v, and it ran %+v", name, tmux.socket, err, ran)
			}
		}
	}
	// A site this run doesn't know of is the grep row's to find (RT126SAM7 B1): the gate holds it too, so a new tmux
	// outside the helper stops a release even where nothing above drives it.
	for _, problem := range tmuxExecProblems(coreExecSites(t), tmuxExecSites, tmuxExecAllowed) {
		t.Error(problem)
	}
	// The Mac app's Swift too (SA-M9): it runs tmux only through TmuxClient, told a server (tmuxgate_swift_test.go).
	for _, problem := range swiftTmuxProblems(readSwiftApp(t)) {
		t.Error(problem)
	}
	if os.Getenv("SMOLCORP_TMUXGATE_CONTROL") == "1" {
		broken := exec.Command(program)
		broken.Env = []string{}
		_ = broken.Run()
		record()
	}
}

// tmuxGateStandInReadsStdin is the rig's own row (FXTMUXSTDIN): the stand-in's look at its stdin (tmuxGateStdinCheck,
// as the stand-in runs it) says /dev/null when it is, and a stream for a stream or a stdin that isn't there. On a Mac,
// also with every look by name at /dev/stdin and /dev/fd refused (a Seatbelt profile, standing in for the "Bad file
// descriptor" such a look gets now and then on a busy Mac): -ef took that for a stream, which failed the gate's stdin
// row on a call that never had one (Studio 5437455b's race step, a kill-server).
func tmuxGateStandInReadsStdin(t *testing.T) {
	look := filepath.Join(t.TempDir(), "look")
	if err := os.WriteFile(look, []byte("#!/bin/sh\n"+tmuxGateStdinCheck+"\necho \"$stdin\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ways := map[string][]string{"as it is": {look}}
	if runtime.GOOS == "darwin" {
		refused := `(version 1)(allow default)(deny file-read-metadata file-read-data (literal "/dev/stdin") (regex #"^/dev/fd/"))`
		ways["its stdin's names refused"] = []string{"/usr/bin/sandbox-exec", "-p", refused, look}
	}
	for way, words := range ways {
		for _, c := range []struct {
			stdin, want string
		}{{"/dev/null", "null"}, {"a stream", "stream"}, {"none", "stream"}} {
			cmd := exec.Command(words[0], words[1:]...)
			switch c.stdin {
			case "a stream":
				cmd.Stdin = strings.NewReader("words")
			case "none":
				cmd = exec.Command("/bin/sh", append([]string{"-c", `exec "$@" <&-`, "sh"}, words...)...)
			}
			out, err := cmd.Output()
			if got := strings.TrimSpace(string(out)); err != nil || got != c.want {
				t.Errorf("%s, stdin %s: %q (%v), not %q", way, c.stdin, got, err, c.want)
			}
		}
	}
}

// reviewersWindow is RT126SAM7R2's B1′ as the reviewer spelled it, a one-shot call through the desk's start-only
// runner. It lives here, outside the core the grep row reads, for the gate to drive at run time: it must be refused.
func (h *hireTmux) reviewersWindow(folder string) error {
	_, _, err := h.startOnly(h.command("new-window", "-d", "-c", folder))
	return err
}
