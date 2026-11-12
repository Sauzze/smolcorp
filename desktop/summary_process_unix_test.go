//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// MARK: The process group and the environment on Unix

// startFakeGroup runs a fake CLI in a group of its own, as spawnSummary does,
// and returns it with the pids its script wrote to a file (its own and the
// sleeping grandchild's). Whatever is left of the group is ended with the test.
func startFakeGroup(t *testing.T, leaves bool) (*exec.Cmd, int, int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("process groups are Unix")
	}
	dir := t.TempDir()
	pids := filepath.Join(dir, "pids")
	body := "sleep 60 &\necho $$ $! > " + pids + ".tmp\nmv " + pids + ".tmp " + pids + "\nwait"
	if leaves {
		body = "sleep 60 &\necho $$ $! > " + pids + ".tmp\nmv " + pids + ".tmp " + pids
	}
	cmd := exec.Command(fakeCLI(t, dir, "claude", body))
	cmd.Dir = dir
	ownGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	group := joinGroup(cmd)
	t.Cleanup(func() { _ = group.kill(); _ = cmd.Wait() })
	var child, grandchild int
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		text, err := os.ReadFile(pids)
		if err != nil {
			continue
		}
		if _, err := fmt.Sscan(string(text), &child, &grandchild); err == nil {
			break
		}
	}
	if child != cmd.Process.Pid || grandchild <= 0 {
		t.Fatalf("pids: %d (started %d), %d", child, cmd.Process.Pid, grandchild)
	}
	return cmd, child, grandchild
}

func waitGone(pid int) bool {
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if processGone(pid) {
			return true
		}
	}
	return false
}

func TestASummaryRunGetsAProcessGroupOfItsOwnThatItsChildrenShare(t *testing.T) {
	_, child, grandchild := startFakeGroup(t, false)
	own, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	group, err := syscall.Getpgid(child)
	if err != nil || group != child || group == own {
		t.Errorf("the child's group is %d (%v), its pid %d, Smolcorp's group %d", group, err, child, own)
	}
	if group, err := syscall.Getpgid(grandchild); err != nil || group != child {
		t.Errorf("the grandchild's group is %d (%v), not the child's %d", group, err, child)
	}
}

func TestKillingTheGroupEndsTheChildAndTheGrandchildItStarted(t *testing.T) {
	cmd, child, grandchild := startFakeGroup(t, false)
	if err := joinGroup(cmd).kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // reaps the child: a zombie would still answer signal 0
	if !waitGone(child) || !waitGone(grandchild) {
		t.Errorf("left running: child gone %v, grandchild gone %v", processGone(child), processGone(grandchild))
	}
	if err := syscall.Kill(-child, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("the group is still there: %v", err)
	}
}

func TestAGroupWhoseLeaderHasExitedIsStillEndedByClose(t *testing.T) {
	cmd, child, grandchild := startFakeGroup(t, true)
	_ = cmd.Wait() // the leader is gone, its grandchild goes on in the group
	if processGone(grandchild) {
		t.Fatal("the grandchild did not outlive the child")
	}
	joinGroup(cmd).close()
	if !waitGone(grandchild) {
		t.Errorf("the grandchild of %d is left running", child)
	}
}

func TestASummaryRunThatExitsLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child")
	quick := fakeCLI(t, dir, "claude", "sleep 60 >/dev/null 2>&1 &\necho $! > "+pidFile+"\necho done")
	ran := spawnSummary(quick, nil, nil, summaryEnvironment(os.Environ()), dir, 10*time.Second)
	text, _ := os.ReadFile(pidFile)
	child, _ := strconv.Atoi(strings.TrimSpace(string(text)))
	if ran.status != 0 || ran.timedOut || child == 0 {
		t.Fatalf("status %d, timed out %v, child %d", ran.status, ran.timedOut, child)
	}
	if !waitGone(child) {
		t.Error("what it started is still running after it exited")
	}
}

func TestTheRunsChildSeesOnlyWhatItShould(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTHROPIC_API_KEY", "sk-"+"ant-test0")
	t.Setenv("ANTHROPIC_DEFAULT_SONNET_MODEL", "x")
	t.Setenv("SMOLCORP_SESSION", "abc")
	t.Setenv("TMUX", "/tmp/fake,1,0")
	t.Setenv("SMOLCORP_SUMMARY", "0")
	printer := fakeCLI(t, dir, "claude", "env")
	ran := spawnSummary(printer, nil, nil, summaryEnvironment(os.Environ()), dir, 10*time.Second)
	seen := strings.Split(string(ran.output), "\n")
	for _, variable := range seen {
		name, _, _ := strings.Cut(variable, "=")
		if slices.Contains([]string{"ANTHROPIC_API_KEY", "ANTHROPIC_DEFAULT_SONNET_MODEL", "SMOLCORP_SESSION", "TMUX"}, name) {
			t.Errorf("the child gets %s", name)
		}
	}
	if strings.Contains(string(ran.output), "sk-"+"ant-test0") {
		t.Error("the key reaches the child")
	}
	if !slices.Contains(seen, "HOME="+home) || !slices.Contains(seen, "SMOLCORP_SUMMARY=1") || slices.Contains(seen, "SMOLCORP_SUMMARY=0") {
		t.Errorf("HOME or the summary mark is wrong: %q", seen)
	}
}

func TestAHiresEnvironmentLosesOnlyWhatMarksADeskOrATerminal(t *testing.T) {
	removed := []string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER",
		"TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON"}
	for _, name := range removed {
		t.Setenv(name, "x")
	}
	t.Setenv("tmux_pane", "x") // names are matched in any case
	t.Setenv("ANTHROPIC_API_KEY", "sk-"+"ant-test0")
	t.Setenv("OPENAI_API_KEY", "sk-test0")
	t.Setenv("ANTHROPIC_DEFAULT_SONNET_MODEL", "m")
	t.Setenv("SMOLCORP_SUMMARY", "1") // not in the hire's list: the summary's own mark
	env := hireEnvironment()
	for _, variable := range env {
		name, _, _ := strings.Cut(variable, "=")
		if slices.Contains(removed, strings.ToUpper(name)) {
			t.Errorf("a hire gets %s", name)
		}
	}
	for _, kept := range []string{"ANTHROPIC_API_KEY=sk-" + "ant-test0", "OPENAI_API_KEY=sk-test0", "ANTHROPIC_DEFAULT_SONNET_MODEL=m"} {
		if !slices.Contains(env, kept) {
			t.Errorf("a hire lost %s", kept)
		}
	}
	if os.Getenv("SMOLCORP_SESSION") != "x" {
		t.Error("Smolcorp's own environment lost a variable")
	}
}

func TestTheSummaryEnvironmentMatchesNamesInAnyCaseAndKeepsOtherVariables(t *testing.T) {
	env := summaryEnvironment([]string{"anthropic_api_key=sk-" + "ant-test0", "Anthropic_Default_Haiku_Model=x", "tmux=x",
		"smolcorp_summary=0", "ANTHROPIC_BASE_URLX=keep", "XDG_CONFIG_HOME=/tmp/c"})
	want := []string{"ANTHROPIC_BASE_URLX=keep", "XDG_CONFIG_HOME=/tmp/c", "SMOLCORP_SUMMARY=1"}
	if !slices.Equal(env, want) {
		t.Errorf("env: %q, want %q", env, want)
	}
}
