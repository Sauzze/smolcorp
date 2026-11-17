//go:build !windows

package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// `smolcorp attach` (SA-M8): its forms; every window and pane line Smolcorp writes is `exec '<smolcorp>' attach …`;
// an attach outside a sandbox runs only through the one helper, told its server, with a clean environment; and a desk
// in its sandbox is never attached outside it. The attach inside the sandbox, T4-T6 through a real attach line, is
// attach_darwin_test.go's.

func TestSmolcorpAttachTakesItsFormsAndNothingElse(t *testing.T) {
	good := map[string]attachRequest{
		"ab12":             {desk: "ab12"},
		"--read-only ab12": {desk: "ab12", readOnly: true},
		"--team-grid 5252": {grid: 5252},
		"--session smolcorp-0a1b2c3d --socket /tmp/tmux-501/default":        {session: "smolcorp-0a1b2c3d", socket: "/tmp/tmux-501/default"},
		"--session hire --socket /tmp/tmux-501/smolcorp-plain-0a1b2c3d4e5f": {session: "hire", socket: "/tmp/tmux-501/smolcorp-plain-0a1b2c3d4e5f"},
	}
	for words, want := range good {
		if got, err := parseAttach(strings.Fields(words)); err != nil || got != want {
			t.Errorf("%q: %+v %v", words, got, err)
		}
	}
	for _, words := range []string{"", "ab12 cd34", "--read-only", "-r ab12", "--team-grid 1", "--team-grid 052", "--team-grid x",
		"--session smolcorp-0a1b2c3d --socket tmux/default", "--session other --socket /tmp/tmux-501/default",
		"--session hire2 --socket /tmp/tmux-501/default", "--session xhire --socket /tmp/tmux-501/default",
		"--session smolcorp-0a1b2c3d --socket /tmp/../etc/default", "--socket /s --session smolcorp-0a1b2c3d", "--read-only --team-grid"} {
		if got, err := parseAttach(strings.Fields(words)); err == nil {
			t.Errorf("%q was taken: %+v", words, got)
		}
	}
	var said bytes.Buffer
	if runAttach([]string{"-x"}, nil, &said, &said) != 1 || !strings.Contains(said.String(), attachUsage) {
		t.Errorf("a word it doesn't take: %q", said.String())
	}
}

// Every line Smolcorp writes to show a desk runs `smolcorp attach` with exec, so the tab ends with it: a desk's window
// (the engine's at its start, "Open a window"), a plain hire's, the team grid's window and its panes.
func TestEveryWindowOnADeskRunsSmolcorpAttachWithExec(t *testing.T) {
	_, _, sandboxed, old := tmuxGateRig(t)
	self := commandLineProgram()
	var opened [][]string
	opener := hireOpener{id: "terminal", open: func(folder string, command []string) (string, error) {
		opened = append(opened, command)
		return "", nil
	}}
	for _, h := range []Hire{sandboxed, old} {
		if err := openHireViewer(h, opener); err != nil {
			t.Fatalf("%s: %v", h.ID, err)
		}
	}
	if !slices.EqualFunc(opened, [][]string{{self, "attach", "ab12"}, {self, "attach", "cd34"}}, slices.Equal) {
		t.Errorf("Open a window ran %q", opened)
	}
	if lines, err := windowScriptLines(opened[0], 0, ""); err != nil || lines != "#!/bin/sh\nrm -f -- \"$0\"\nexec '"+self+"' 'attach' 'ab12'\n" {
		t.Errorf("the window's script:\n%s %v", lines, err)
	}
	if lines, _ := windowScriptLines(plainAttachLine(self, "smolcorp-0a1b2c3d", "/t/default"), 7771, "/s"); !strings.HasSuffix(lines,
		"exec '"+self+"' 'attach' '--session' 'smolcorp-0a1b2c3d' '--socket' '/t/default'\n") || !strings.Contains(lines, "SMOLCORP_TEST_SUPPORT_DIR='/s'") {
		t.Errorf("a plain hire's window, in a test copy:\n%s", lines)
	}
}

// An attach outside a sandbox: an old desk's (by its id, its name, read-only), a plain hire's, the team grid's. Each
// runs its tmux through the one helper, told its server ("unchecked", or the grid's pid), with stdin from what it's
// given, and an environment of PATH, LANG, TERM (the terminal's own) and the guard alone. tmux's exit status is the
// attach's. A desk in its sandbox is never attached outside it; a desk fired, unknown, or with no checked server, is
// never attached at all.
func TestAnAttachOutsideTheSandboxRunsOnlyThroughTheHelper(t *testing.T) {
	_, log, sandboxed, old := tmuxGateRig(t)
	t.Setenv("TERM", "xterm-ghostty")
	seen := 0
	attach := func(words ...string) (status int, said string, runs []tmuxGateRun) {
		var out bytes.Buffer
		status = runAttach(words, strings.NewReader("typed\n"), &out, &out)
		runs = tmuxGateRuns(t, log, seen)
		seen += len(runs)
		return status, out.String(), runs
	}
	clean := []string{"LANG", "PATH", tmuxGuardVariable, "TERM"}
	// An unchecked attach is told the sandboxed desks' servers, which it never talks to (FXTMUXDIR): here ab12's alone.
	unlisted := tmuxUnchecked + ":" + strconv.Itoa(sandboxed.TmuxServer)
	for words, want := range map[string]tmuxGateRun{
		"cd34":             {guard: unlisted, args: "-f /dev/null -S " + old.TmuxSocket + " attach-session -t =hire"},
		"old":              {guard: unlisted, args: "-f /dev/null -S " + old.TmuxSocket + " attach-session -t =hire"},
		"--read-only cd34": {guard: unlisted, args: "-f /dev/null -S " + old.TmuxSocket + " attach-session -r -t =hire"},
		"--session smolcorp-0a1b2c3d --socket /t/default": {guard: unlisted, args: "-f /dev/null -S /t/default attach-session -t =smolcorp-0a1b2c3d"},
		"--session hire --socket /t/plain":                {guard: unlisted, args: "-f /dev/null -S /t/plain attach-session -t =hire"},
		"--team-grid 5252":                                {guard: "5252", args: "-f /dev/null -S " + teamGridSocket(0) + " attach-session -r -t =team"},
	} {
		status, said, runs := attach(strings.Fields(words)...)
		if status != 0 || len(runs) != 1 || runs[0].guard != want.guard || runs[0].args != want.args || runs[0].stdin != "stream" ||
			!slices.Equal(runs[0].env, clean) {
			t.Errorf("%q: status %d %q, ran %+v", words, status, said, runs)
		}
	}

	// Never outside its sandbox, and never with no checked server, a fired desk, or one that isn't there.
	unchecked := sandboxed
	unchecked.ID, unchecked.Name, unchecked.TmuxServer, unchecked.TmuxSocket = "ef56", "Unchecked", 0, deskSocketPath("ef56", 0)
	listenIn(t, unchecked.TmuxSocket)
	fired := old
	fired.ID, fired.Name, fired.State = "gh78", "Fired", "fired"
	writeRegistryHires(t, sandboxed, old, unchecked, fired)
	inSandbox := map[string]string{"darwin": "couldn't show this desk inside its sandbox", "linux": "can't show on this computer"}[runtime.GOOS]
	for words, why := range map[string]string{
		"ab12": inSandbox, "Sandboxed": inSandbox, "ef56": "never checked", "gh78": "isn't on the team", "zz99": "no desk is called zz99",
	} {
		status, said, runs := attach(strings.Fields(words)...)
		if status != 1 || len(runs) != 0 || !strings.Contains(said, why) {
			t.Errorf("%q: status %d, said %q, ran %+v", words, status, said, runs)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(supportDir, "sandbox-canary-*")); len(left) != 0 {
		t.Errorf("a canary was left: %q", left)
	}
	if _, err := os.Stat(filepath.Join(supportDir, "hires", "ab12", "STATUS.md")); err == nil {
		t.Error("a refused attach wrote the desk's status")
	}
}
