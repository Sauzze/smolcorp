package acp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-M11: an agent started through a Runner (the sandbox's, on a Mac) is started from the Runner's command alone,
// with the Runner's environment, and is never talked to unless the Runner says its start held.

// testRunner is a Runner that makes the plain command, adds its own variable, and says what Started is told to.
type testRunner struct {
	refuse  error
	made    []string // the program and its arguments it was asked for
	started int      // the pid it was asked about
	closed  int
}

func (r *testRunner) Command(path string, args, env []string) (*exec.Cmd, error) {
	r.made = append([]string{path}, args...)
	cmd := disclaim.Cmd(exec.Command(path, args...))
	cmd.Env = append(slices.Clone(env), "SMOLCORP_ACP_RUNNER=1")
	return cmd, nil
}

func (r *testRunner) Started(pid int) error { r.started = pid; return r.refuse }
func (r *testRunner) Close()                { r.closed++ }

func TestAnAgentStartsThroughItsRunner(t *testing.T) {
	exe, _ := os.Executable()
	cmd := standInCommand(exe, "hello")
	cmd.Args = []string{"--an-argument"}
	r := &testRunner{}
	cmd.Runner = r
	c, err := Start(cmd, &recorder{}, Options{CallTimeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if r.started != c.cmd.Process.Pid || r.closed != 1 || !slices.Equal(r.made, []string{exe, "--an-argument"}) {
		t.Errorf("the runner made %q, was asked about pid %d (the agent's is %d), closed %d times", r.made, r.started, c.cmd.Process.Pid, r.closed)
	}
	if !slices.Contains(c.cmd.Env, "SMOLCORP_ACP_RUNNER=1") || c.cmd.Dir != cmd.Dir {
		t.Errorf("the runner's command wasn't the one started: env has its variable %v, folder %q",
			slices.Contains(c.cmd.Env, "SMOLCORP_ACP_RUNNER=1"), c.cmd.Dir)
	}
	if _, err := c.Initialize(context.Background()); err != nil {
		t.Errorf("an agent whose start held: %v", err)
	}
}

func TestAnAgentWhoseRunnerRefusesItsStartIsKilledAndNeverTalkedTo(t *testing.T) {
	exe, _ := os.Executable()
	cmd := standInCommand(exe, "hello")
	r := &testRunner{refuse: errors.New("it isn't in its sandbox")}
	cmd.Runner = r
	c, err := Start(cmd, &recorder{}, Options{CallTimeout: 10 * time.Second})
	if c != nil || err == nil || !strings.Contains(err.Error(), "it isn't in its sandbox") {
		t.Fatalf("a refused start: %v %v", c, err)
	}
	if r.started <= 0 || r.closed != 1 {
		t.Errorf("asked about pid %d, closed %d times", r.started, r.closed)
	}
	if stillAlive(r.started) {
		t.Errorf("the refused agent %d is still running", r.started)
	}
}

func TestAnAgentWhoseRunnerCantMakeItIsNeverStarted(t *testing.T) {
	exe, _ := os.Executable()
	cmd := standInCommand(exe, "hello")
	r := &failingRunner{}
	cmd.Runner = r
	if c, err := Start(cmd, &recorder{}, Options{}); c != nil || err == nil || !strings.Contains(err.Error(), "no sandbox here") || r.closed != 1 {
		t.Errorf("a start that can't be made: %v %v, closed %d times", c, err, r.closed)
	}
}

type failingRunner struct{ closed int }

func (r *failingRunner) Command(string, []string, []string) (*exec.Cmd, error) {
	return nil, errors.New("no sandbox here")
}
func (r *failingRunner) Started(int) error { panic("asked about a start that was never made") }
func (r *failingRunner) Close()            { r.closed++ }
