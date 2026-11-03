package desktop

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// With a tray, as the app on a desktop, or where a process can't become
// another (Windows): a new copy, and this one leaves.
func TestARestartStartsTheSameModeAndLeaves(t *testing.T) {
	savedStart, savedLeave, savedArgs := startProgramWith, leave, os.Args
	t.Cleanup(func() { startProgramWith, leave, os.Args = savedStart, savedLeave, savedArgs })
	withTray(t, true, true)
	var started []string
	var env []string
	startProgramWith = func(program string, args []string, e ...string) error {
		started, env = append([]string{program}, args...), e
		return nil
	}
	left := make(chan bool, 1)
	leave = func() { left <- true }

	for _, c := range []struct {
		args []string
		want []string
	}{
		// smolcorp serve, over SSH or on a desktop with no tray: serve again.
		{[]string{"serve"}, []string{"/opt/smolcorp", "serve"}},
		// The app: the app again.
		{nil, []string{"/opt/smolcorp"}},
		// Nothing else is passed on.
		{[]string{"setup", "--open"}, []string{"/opt/smolcorp"}},
	} {
		os.Args = append([]string{"smolcorp"}, c.args...)
		if err := relaunch("/opt/smolcorp"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(started, c.want) || !slices.Contains(env, restartedVariable+"=1") {
			t.Errorf("run as %q, the restart starts %q with %q; want %q, told it takes another's place", c.args, started, env, c.want)
		}
		// And this copy leaves, so the new one gets the port.
		select {
		case <-left:
		case <-time.After(3 * time.Second):
			t.Fatalf("run as %q, the copy restarting never left", c.args)
		}
	}
}

// With no tray, as smolcorp serve: the same process becomes the new copy, in
// its terminal, so it prints the new setup link there and stops with Ctrl+C.
func TestARestartWithNoTrayIsTheSameProcess(t *testing.T) {
	if !canReplaceProcess {
		t.Skip("this system starts a new copy instead")
	}
	savedStart, savedReplace, savedArgs := startProgramWith, replaceProcess, os.Args
	t.Cleanup(func() {
		startProgramWith, replaceProcess, os.Args = savedStart, savedReplace, savedArgs
	})
	withTray(t, false, false)
	os.Args = []string{"smolcorp", "serve"}
	started := false
	startProgramWith = func(string, []string, ...string) error { started = true; return nil }
	replaced := make(chan []string, 1)
	replaceProcess = func(program string, args, env []string) error {
		if !slices.Contains(env, restartedVariable+"=1") {
			t.Errorf("the new copy isn't told it takes another's place: %q", env)
		}
		if !slices.ContainsFunc(env, func(v string) bool { return strings.HasPrefix(v, childrenVariable+"=") }) {
			t.Errorf("the new copy isn't handed the children to wait for: %q", env)
		}
		replaced <- append([]string{program}, args...)
		select {} // as exec: never back
	}
	if err := relaunch("/opt/smolcorp"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-replaced:
		if !slices.Equal(got, []string{"/opt/smolcorp", "serve"}) {
			t.Errorf("the process became %q, want serve again", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the process never became the new copy")
	}
	if started {
		t.Error("a second copy was started as well")
	}
}

// withTray sets whether the tray started and a host is there, for the test.
func withTray(t *testing.T, started, host bool) {
	t.Helper()
	savedStarted, savedHost := trayStarted.Load(), hasTrayHost
	t.Cleanup(func() { trayStarted.Store(savedStarted); hasTrayHost = savedHost })
	trayStarted.Store(started)
	hasTrayHost = func() bool { return host }
}

func TestATrayWithNoHostIsNoTray(t *testing.T) {
	withTray(t, true, false)
	if hasTray() {
		t.Error("a tray started on a desktop with no host counts as one to quit")
	}
	// A host comes later, after Smolcorp gave up waiting for one: the tray counts now.
	hasTrayHost = func() bool { return true }
	if !hasTray() {
		t.Error("with a host there now, the tray doesn't count")
	}
	// A host, but no tray started, as with no desktop: none.
	trayStarted.Store(false)
	if hasTray() {
		t.Error("a host with no tray started counts as a tray")
	}
}

func TestARestartThatCantHappenIsOnTheChecklist(t *testing.T) {
	savedRelaunch, savedProblem := relaunch, restartProblem.Load()
	t.Cleanup(func() { relaunch = savedRelaunch; restartProblem.Store(savedProblem) })
	restartProblem.Store(nil)
	if restartCheck() != nil {
		t.Fatal("with no restart asked for, the checklist has a line for one")
	}
	relaunch = func(string, ...string) error { return errors.New("no such file") }
	restartSoon()
	c := restartCheck()
	if c == nil || c.Status != checkFix || !strings.Contains(c.Detail, "no such file") || !strings.Contains(c.Detail, "open it again") {
		t.Fatalf("after a restart that failed, the checklist says %+v", c)
	}
	d := newTestDeck(t)
	if checks := d.selfTest(); checks[0].ID != "restart" || ready(checks) {
		t.Errorf("the self-test is %+v, want the restart first, and not ready", checks)
	}
}

func TestARestartInPlaceThatFailsTwiceIsOnTheChecklist(t *testing.T) {
	if !canReplaceProcess {
		t.Skip("this system starts a new copy instead")
	}
	savedStart, savedReplace, savedProblem := startProgramWith, replaceProcess, restartProblem.Load()
	t.Cleanup(func() {
		startProgramWith, replaceProcess = savedStart, savedReplace
		restartProblem.Store(savedProblem)
	})
	withTray(t, false, false)
	restartProblem.Store(nil)
	replaceProcess = func(string, []string, []string) error { return errors.New("exec failed") }
	startProgramWith = func(string, []string, ...string) error { return errors.New("start failed") }
	if err := relaunch("/opt/smolcorp"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(3 * time.Second); restartCheck() == nil; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("a restart that failed both ways said nothing")
		}
	}
	if c := restartCheck(); !strings.Contains(c.Detail, "exec failed") || !strings.Contains(c.Detail, "start failed") {
		t.Errorf("the checklist says %q, want both reasons", c.Detail)
	}
}
