//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// recorder is a stand-in for tmux: it notes the arguments and gives one answer.
type recorder struct {
	asked  [][]string
	output string
	status int
	ran    bool
}

func newRecorder() *recorder { return &recorder{ran: true} }

func (r *recorder) run(args []string) (string, int, bool) {
	r.asked = append(r.asked, args)
	return r.output, r.status, r.ran
}

// shortSupport isolates the test, then moves the support folder to a short
// temporary one: a unix socket's path can't be much over 100 bytes, and a
// test's own folder is named after the test.
func shortSupport(t *testing.T) {
	t.Helper()
	isolate(t)
	dir, err := os.MkdirTemp("", "hs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	supportDir = dir
}

func fakeHireTmux(t *testing.T, r *recorder, testPort int) (*hireTmux, string) {
	t.Helper()
	shortSupport(t)
	h := newHireTmux("ab12-cd34", testPort, r.run)
	if h == nil {
		t.Fatal("no hire tmux")
	}
	return h, h.socket
}

func TestTheSocketIsInTheSupportFolderAndNamedAfterTheHire(t *testing.T) {
	r := newRecorder()
	h, _ := fakeHireTmux(t, r, 0)
	if want := filepath.Join(supportDir, "smolcorp-ab12-cd34"); h.socket != want {
		t.Errorf("socket %q, want %q", h.socket, want)
	}
	if h := newHireTmux("ab12-cd34", 7811, r.run); h.socket != filepath.Join(supportDir, "smolcorp-7811-ab12-cd34") {
		t.Errorf("test copy socket %q", h.socket)
	}
	for _, id := range []string{"", "../x", "A B", "a/b", "a;b", "é", strings.Repeat("a", 65)} {
		if newHireTmux(id, 0, r.run) != nil {
			t.Errorf("id %q made a socket", id)
		}
	}
	if newHireTmux("ab", 0, r.run) == nil {
		t.Error("a plain id is refused")
	}
	long := supportDir
	supportDir = filepath.Join(supportDir, strings.Repeat("d", 90))
	defer func() { supportDir = long }()
	if newHireTmux("ab", 0, r.run) != nil {
		t.Error("a socket path too long for a unix socket was accepted")
	}
}

func TestOnlyOurSocketsAreOwned(t *testing.T) {
	shortSupport(t) // a socket's path must fit a unix socket (a Mac's temp folder is long)
	in := func(name string) string { return filepath.Join(supportDir, name) }
	for _, c := range []struct {
		path string
		port int
		want bool
	}{
		{in("smolcorp-ab12"), 0, true},
		{in("smolcorp-7811-ab12"), 0, true},
		{in("default"), 0, false},
		{in("smolcorp-"), 0, false},
		{in("smolcorp-../x"), 0, false},
		{"", 0, false},
		{"/tmp/tmux-1000/smolcorp-ab12", 0, false}, // not in our folder
		{filepath.Join(os.TempDir(), "smolcorp-ab12"), 0, false},
		{in("smolcorp-7811-ab12"), 7811, true},
		{in("smolcorp-ab12"), 7811, false}, // a test copy never reaches a real install's
		{in("smolcorp-7812-ab12"), 7811, false},
	} {
		if got := ownsHireSocket(c.path, c.port); got != c.want {
			t.Errorf("owns(%q, %d) = %v", c.path, c.port, got)
		}
		if got := hireTmuxAt(c.path, c.port, newRecorder().run) != nil; got != c.want {
			t.Errorf("hireTmuxAt(%q, %d) = %v", c.path, c.port, got)
		}
	}
}

func TestStartIsDetachedInTheFolderOnItsOwnSocket(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	r.output = "%0 " + socket
	pane, got, ok := h.start("/work/app", "/bin/zsh", "/x/hire.sh")
	if !ok || pane != "%0" || got != socket {
		t.Fatalf("start gave %q %q %v", pane, got, ok)
	}
	want := []string{"-S", socket, "new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}",
		"-s", "hire", "-c", "/work/app", "/bin/zsh", "-lic", "exec '/x/hire.sh'"}
	if len(r.asked) != 1 || !reflect.DeepEqual(r.asked[0], want) {
		t.Errorf("asked %q, want %q", r.asked, want)
	}
}

func TestStartFailsWhenTmuxDoes(t *testing.T) {
	r := newRecorder()
	h, _ := fakeHireTmux(t, r, 0)
	for _, c := range []recorder{{status: 1, ran: true}, {output: "garbage", ran: true}, {output: "%0 ", ran: true}, {output: "0 /s", ran: true}, {}} {
		*r = c
		if _, _, ok := h.start("/w", "/bin/sh", "x"); ok {
			t.Errorf("start succeeded on %+v", c)
		}
	}
}

func TestAliveIsAskedByExactName(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	if up, known := h.alive(); !up || !known {
		t.Error("alive")
	}
	r.status = 1
	if up, known := h.alive(); up || !known {
		t.Error("no session is a no, not a doubt")
	}
	r.ran = false
	if up, known := h.alive(); up || known {
		t.Error("tmux that can't be run is unknown")
	}
	if want := []string{"-S", socket, "has-session", "-t", "=hire"}; !reflect.DeepEqual(r.asked[0], want) {
		t.Errorf("asked %q", r.asked[0])
	}
}

func TestCaptureReadsTheLastLinesAndTheStateFollows(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	r.output = "one\ntwo"
	if got, ok := h.capture(40); !ok || got != "one\ntwo" {
		t.Errorf("capture %q %v", got, ok)
	}
	if want := []string{"-S", socket, "capture-pane", "-p", "-t", "=hire:", "-S", "-40"}; !reflect.DeepEqual(r.asked[0], want) {
		t.Errorf("asked %q", r.asked[0])
	}
	r.asked = nil
	h.capture(0)
	h.capture(999999)
	if r.asked[0][len(r.asked[0])-1] != "-1" || r.asked[1][len(r.asked[1])-1] != "-5000" {
		t.Errorf("lines not clamped: %q", r.asked)
	}
	// The pane's screen goes through ReadPaneState, a real capture of a menu included.
	menu, err := os.ReadFile("../Tests/fixtures/panes/permission-menu.txt")
	if err != nil {
		t.Fatal(err)
	}
	r.output = string(menu)
	if got := h.paneState(); got != PaneMenu {
		t.Errorf("a permission menu reads %v", got)
	}
	idle, _ := os.ReadFile("../Tests/fixtures/panes/idle-empty.txt")
	r.output = string(idle)
	if got := h.paneState(); got != PaneIdle {
		t.Errorf("an idle window reads %v", got)
	}
	r.status = 1
	if got := h.paneState(); got != PaneUnknown {
		t.Errorf("a failed capture reads %v, want unknown", got)
	}
}

func TestEndNamesTheSessionAndNeverKillsTheServerByName(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	if !h.end() {
		t.Error("end failed")
	}
	if want := []string{"-S", socket, "kill-session", "-t", "=hire"}; len(r.asked) != 1 || !reflect.DeepEqual(r.asked[0], want) {
		t.Errorf("asked %q", r.asked)
	}
}

func TestEveryCallNamesOurSocket(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	r.output = "%0 " + socket
	h.start("/w", "/bin/sh", "x")
	h.alive()
	h.capture(3)
	h.paneState()
	h.end()
	if len(r.asked) != 5 {
		t.Fatalf("asked %d times", len(r.asked))
	}
	for _, args := range r.asked {
		if args[0] != "-S" || args[1] != socket {
			t.Errorf("call %q doesn't name our socket first", args)
		}
		for _, a := range args {
			if a == "kill-server" || a == "-L" {
				t.Errorf("call %q reaches beyond our session", args)
			}
		}
	}
}

// clock is a time that only the pause moves.
type clock struct{ at time.Time }

func (c *clock) now() time.Time        { return c.at }
func (c *clock) pause(d time.Duration) { c.at = c.at.Add(d) }

func TestWaitForStartHearsFromTheHire(t *testing.T) {
	c := &clock{at: time.Unix(1000, 0)}
	looks := 0
	started, reason := waitForStart(func() (bool, bool) { return true, true }, func() bool { looks++; return looks == 3 },
		hireStartWithin, time.Second, c.now, c.pause)
	if !started || reason != "" || c.at != time.Unix(1002, 0) {
		t.Errorf("started %v %q at %v", started, reason, c.at.Unix())
	}
}

func TestAHireThatNeverReportsFailsAfterTheWait(t *testing.T) {
	c := &clock{at: time.Unix(1000, 0)}
	started, reason := waitForStart(func() (bool, bool) { return true, true }, func() bool { return false }, 45*time.Second, 5*time.Second, c.now, c.pause)
	if started || reason != "no word from it in 45 seconds" || c.at != time.Unix(1045, 0) {
		t.Errorf("started %v %q at %v", started, reason, c.at.Unix())
	}
}

func TestAHireWhoseSessionDiedFailsAtOnceEvenIfHeardFrom(t *testing.T) {
	c := &clock{at: time.Unix(1000, 0)}
	pause := func(time.Duration) { t.Error("waited") }
	started, reason := waitForStart(func() (bool, bool) { return false, true }, func() bool { return true }, hireStartWithin, time.Second, c.now, pause)
	if started || reason != "its tmux session ended before it reported" {
		t.Errorf("started %v %q", started, reason)
	}
	// Not being able to ask tmux isn't a death.
	if started, _ := waitForStart(func() (bool, bool) { return false, false }, func() bool { return true }, hireStartWithin, time.Second, c.now, pause); !started {
		t.Error("an unanswered tmux counted as a death")
	}
}

func TestAHireThatDiesAfterReportingStillFails(t *testing.T) {
	c := &clock{at: time.Unix(1000, 0)}
	looks := 0
	started, _ := waitForStart(func() (bool, bool) { looks++; return looks < 2, true }, func() bool { return true }, hireStartWithin, time.Second, c.now, c.pause)
	if started {
		t.Error("a session that ended while it reported was taken as started")
	}
}

func TestCleaningUpAHireThatNeverReported(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	hire := Hire{ID: "ab12-cd34", Worktree: "/w/ab12", Branch: "hire/ab12"}
	var failed, removed []string
	remove := func(x Hire) bool { removed = append(removed, x.Worktree); return true }
	fail := func(id, why string) { failed = append(failed, id+": "+why) }

	left := cleanUpHire(hire, "no word from it in 45 seconds", h, remove, fail)
	if left != "" || !reflect.DeepEqual(removed, []string{"/w/ab12"}) || !reflect.DeepEqual(failed, []string{"ab12-cd34: Didn't start: no word from it in 45 seconds."}) {
		t.Errorf("left %q removed %q failed %q", left, removed, failed)
	}
	want := []string{"-S", socket, "kill-session", "-t", "=hire"}
	if got := r.asked[len(r.asked)-1]; !reflect.DeepEqual(got, want) {
		t.Errorf("last call %q, want %q", got, want)
	}

	// A session that is already gone isn't ended again; a worktree with changes stays, and the answer says so.
	r.asked, r.status = nil, 1
	failed = nil
	left = cleanUpHire(hire, "gone", h, func(Hire) bool { return false }, fail)
	if len(r.asked) != 1 || r.asked[0][2] != "has-session" {
		t.Errorf("a dead session was asked to end: %q", r.asked)
	}
	if !strings.Contains(left, "/w/ab12 was kept") || len(failed) != 1 || !strings.HasSuffix(failed[0], left) {
		t.Errorf("left %q failed %q", left, failed)
	}
	// No worktree and no tmux: only the failure is written down.
	failed = nil
	if left := cleanUpHire(Hire{ID: "x"}, "why", nil, func(Hire) bool { t.Error("removed"); return true }, fail); left != "" || len(failed) != 1 {
		t.Errorf("left %q failed %q", left, failed)
	}
}

// needTmux is the path of tmux for a test that runs a real one: without it the test skips, but in CI (CI set) it fails.
func needTmux(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("tmux")
	if err != nil && os.Getenv("CI") != "" {
		t.Fatal("tmux isn't installed: in CI a missing tmux is a failure, not a skip")
	} else if err != nil {
		t.Skip("tmux isn't installed")
	}
	return path
}

// With a real tmux, on a socket of the test's own, and a fake agent in place of claude.
func TestARealSessionOnItsOwnSocket(t *testing.T) {
	path := needTmux(t)
	shortSupport(t)
	saved := tmuxPath
	tmuxPath = func() string { return path }
	t.Cleanup(func() { tmuxPath = saved })
	h := newHireTmux("it-works", 0, nil)
	if h == nil {
		t.Fatal("no hire tmux")
	}
	if up, known := h.alive(); up || !known {
		t.Fatalf("a socket that was never made: alive %v known %v", up, known)
	}
	folder := t.TempDir()
	agent := filepath.Join(folder, "fake-agent.sh")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\necho 'fake agent is up'\nexec cat\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	pane, socket, ok := h.start(folder, "/bin/sh", agent)
	if !ok {
		t.Fatal("start failed")
	}
	// Ends only its own session, on its own socket, whatever else happens.
	t.Cleanup(func() { h.end() })
	if !strings.HasPrefix(pane, "%") || socket != h.socket {
		t.Errorf("pane %q socket %q, want %q", pane, socket, h.socket)
	}
	if up, _ := h.alive(); !up {
		t.Error("the session isn't alive")
	}
	// Polled through waitForStart, bounded by a deadline, with a short pause only between looks.
	var screen string
	shown, _ := waitForStart(func() (bool, bool) { return true, false }, func() bool {
		screen, _ = h.capture(20)
		return strings.Contains(screen, "fake agent is up")
	}, 5*time.Second, 10*time.Millisecond, time.Now, time.Sleep)
	if !shown {
		t.Errorf("the pane shows %q", screen)
	}
	if got := h.paneState(); got != PaneUnknown {
		t.Errorf("a plain program's screen reads %v, want unknown", got)
	}
	if !h.end() {
		t.Error("end failed")
	}
	if up, _ := h.alive(); up {
		t.Error("the session is still alive after end")
	}
}

// The tmux server's environment is a hire's: nothing of where Smolcorp started, and never the GM's token.
func TestTmuxIsGivenAHiresEnvironment(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	fake := filepath.Join(dir, "tmux")
	dump := filepath.Join(dir, "env")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nenv > "+dump+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SMOLCORP_GM_TOKEN", "SMOLCORP_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER", "TMUX", "TMUX_PANE", "WEZTERM_PANE", "KITTY_WINDOW_ID"} {
		t.Setenv(name, "fake-gm-token-1")
	}
	t.Setenv("HIRE_KEEPS_THIS", "yes")
	if _, status, ran := realTmux(fake)([]string{"-V"}); !ran || status != 0 {
		t.Fatalf("fake tmux ran %v status %d", ran, status)
	}
	got, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SMOLCORP_GM_TOKEN", "SMOLCORP_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER", "TMUX", "TMUX_PANE", "WEZTERM_PANE", "KITTY_WINDOW_ID"} {
		if strings.Contains("\n"+string(got), "\n"+name+"=") {
			t.Errorf("tmux was given %s", name)
		}
	}
	if !strings.Contains(string(got), "HIRE_KEEPS_THIS=yes") {
		t.Error("the rest of the environment was dropped")
	}
}

// Agent windows in the background (T796): a hire's viewers counted and detached on its own socket, by its session.
func TestHireViewersAreDetachedOnTheirOwnSocketOnly(t *testing.T) {
	calls := [][]string{}
	server := &hireTmux{socket: "/s/smolcorp-a", run: func(args []string) (string, int, bool) {
		calls = append(calls, args)
		if slices.Contains(args, "list-clients") {
			return "/dev/pts/3\n/dev/pts/4", 0, true
		}
		return "", 0, true
	}}
	if server.viewers() != 2 || !server.detachViewers() {
		t.Fatal("viewers or detach")
	}
	want := [][]string{
		{"-S", "/s/smolcorp-a", "list-clients", "-t", "=hire", "-F", "#{client_tty}"},
		{"-S", "/s/smolcorp-a", "detach-client", "-s", "=hire"},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls %v", calls)
	}
	for i := range want {
		if !slices.Equal(calls[i], want[i]) {
			t.Fatalf("call %d: %v, want %v", i, calls[i], want[i])
		}
		if slices.Contains(calls[i], "kill-session") || slices.Contains(calls[i], "kill-server") {
			t.Fatalf("killed: %v", calls[i])
		}
	}
}

// RWTC N1: only a tmux pane on one of the copy's own hire sockets is its own.
func TestATabIsATestCopysOwnOnlyAsATmuxPaneOnItsOwnHireSocket(t *testing.T) {
	isolate(t)
	onMarkedPort(t, 7799)
	own := "tmux:%3@" + filepath.Join(supportDir, hireSocketName("abcdef123456", 7799))
	if why := testCopyKeepsTab(own); why != "" {
		t.Errorf("its own hire's pane: %q", why)
	}
	for _, tab := range []string{"iterm:AB12", "tty:/dev/ttys003", "launcher:L1", "kitty:3@/tmp/kitty", "wezterm:4@/tmp/w", "tmux:%3",
		"tmux:%3@/tmp/tmux-501/default", "headless:abc", "",
		"tmux:%3@" + filepath.Join(supportDir, hireSocketName("abcdef123456", 0)),     // the office's own hire
		"tmux:%3@" + filepath.Join(supportDir, hireSocketName("abcdef123456", 7798)),  // another copy's
		"tmux:%3@" + filepath.Join(t.TempDir(), hireSocketName("abcdef123456", 7799)), // not in a hire socket folder
	} {
		if why := testCopyKeepsTab(tab); why != testCopyKeepsTerminal {
			t.Errorf("%q: %q", tab, why)
		}
	}
	onDefaultPort(t)
	if why := testCopyKeepsTab("iterm:AB12"); why != "" {
		t.Errorf("the office's own: %q", why)
	}
}
