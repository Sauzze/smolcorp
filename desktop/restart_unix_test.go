//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestChildrenAreKeptTrackOfUntilTheyEnd(t *testing.T) {
	cmd := exec.Command("sleep", "0.3")
	if err := startAndWait(cmd); err != nil {
		t.Fatal(err)
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	if note := childrenNote(); !strings.Contains(","+strings.TrimPrefix(note, childrenVariable+"=")+",", ","+pid+",") {
		t.Errorf("while it runs, %s isn't in %q", pid, note)
	}
	for deadline := time.Now().Add(3 * time.Second); strings.Contains(childrenNote(), pid); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s is still in %q after it ended", pid, childrenNote())
		}
	}
}

// After a restart in place, a child of the program before is waited for, as
// the new program's own: none is left a zombie.
func TestAChildHandedOnIsWaitedFor(t *testing.T) {
	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil { // started, as by the program before, and never waited for
		t.Fatal(err)
	}
	done := make(chan bool)
	go func() { waitForChild(cmd.Process.Pid); done <- true }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the child handed on was never waited for")
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
		t.Errorf("the child is still there to wait for (%v): a zombie", err)
	}
	// Adopted, a child is handed on again by a second restart, while it runs.
	again := exec.Command("sleep", "0.3")
	if err := again.Start(); err != nil {
		t.Fatal(err)
	}
	pid := strconv.Itoa(again.Process.Pid)
	adoptChildren(pid)
	if note := childrenNote(); !strings.Contains(","+strings.TrimPrefix(note, childrenVariable+"=")+",", ","+pid+",") {
		t.Errorf("an adopted child, %s, isn't in %q for the next restart", pid, note)
	}
	for deadline := time.Now().Add(3 * time.Second); strings.Contains(childrenNote(), pid); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the adopted child %s is still in %q after it ended", pid, childrenNote())
		}
	}
	// The note's own reading: pids, a comma between, and nothing else.
	adoptChildren("")
	adoptChildren("not,a,pid,-3,0,4294967295,4294967296")
	for _, never := range []string{"-3", "4294967295", "4294967296"} {
		if strings.Contains(childrenNote(), never) {
			t.Errorf("%s was taken as a child: %q", never, childrenNote())
		}
	}
}

// The stages of a real restart in place, run as the test binary again (see
// TestARealRestartInPlaceKeepsTheProcessAndAdoptsItsChildren): the first
// starts a child and becomes the second, which adopts it.
const (
	stageVariable = "SMOLCORP_TEST_RESTART_STAGE"
	stageOutput   = "SMOLCORP_TEST_RESTART_OUT"
)

func TestRestartStageOne(t *testing.T) {
	if os.Getenv(stageVariable) != "one" {
		t.Skip("only as the first stage of a restart in place")
	}
	out := os.Getenv(stageOutput)
	child := exec.Command("sleep", "0.5")
	if err := startAndWait(child); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out+".child", []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// exec adds to this environment, and the first of a name wins: take the stage out.
	os.Unsetenv(stageVariable)
	// Never back, unless it couldn't.
	t.Fatal(replaceProcess(self, []string{"-test.run=^TestRestartStageTwo$", "-test.v"},
		[]string{stageVariable + "=two", restartedVariable + "=1", childrenNote()}))
}

func TestRestartStageTwo(t *testing.T) {
	if os.Getenv(stageVariable) != "two" {
		t.Skip("only as the second stage of a restart in place")
	}
	out := os.Getenv(stageOutput)
	note := os.Getenv(childrenVariable)
	if !takeStartNotes() {
		t.Error("the new copy wasn't told it takes another's place")
	}
	if os.Getenv(restartedVariable) != "" || os.Getenv(childrenVariable) != "" {
		t.Error("the notes were left in the environment for the agents this one starts")
	}
	raw, err := os.ReadFile(out + ".child")
	if err != nil {
		t.Fatal(err)
	}
	child := string(raw)
	if !strings.Contains(","+note+",", ","+child+",") {
		t.Errorf("the child %s wasn't handed on in %q", child, note)
	}
	for deadline := time.Now().Add(5 * time.Second); strings.Contains(childrenNote(), child); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the adopted child %s is still in %q after it ended", child, childrenNote())
		}
	}
	pid, _ := strconv.Atoi(child)
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
		t.Errorf("the adopted child is still there to wait for (%v): a zombie", err)
	}
	if err := os.WriteFile(out, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A restart in place is an exec: the new copy has the process of the old, its
// children are the new copy's to wait for, and none is left a zombie.
func TestARealRestartInPlaceKeepsTheProcessAndAdoptsItsChildren(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report")
	home := t.TempDir()
	cmd := exec.Command(self, "-test.run=^TestRestartStageOne$", "-test.v")
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"XDG_DATA_HOME="+filepath.Join(home, "data"), stageVariable+"=one", stageOutput+"="+out)
	got, err := cmd.CombinedOutput() // starts and waits for it, as this test's own child
	if err != nil {
		t.Fatalf("the restart failed: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), "--- PASS: TestRestartStageTwo") {
		t.Fatalf("the second stage never ran:\n%s", got)
	}
	pid, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the new copy reported nothing: %v\n%s", err, got)
	}
	if string(pid) != strconv.Itoa(cmd.Process.Pid) {
		t.Errorf("the new copy is process %s, the old was %d: not the same process", pid, cmd.Process.Pid)
	}
	// The process it started has ended too, and is waited for: no zombie.
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil); err != syscall.ECHILD {
		t.Errorf("the restarted process is still there to wait for (%v)", err)
	}
}

// An exec that can't happen comes back with the reason, and this process
// carries on, for relaunch to start a copy of its own.
func TestARestartInPlaceThatCantExecComesBack(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-smolcorp")
	if err := replaceProcess(missing, nil, []string{restartedVariable + "=1"}); err == nil {
		t.Fatal("a program that isn't there became the new copy")
	}
	notExecutable := filepath.Join(t.TempDir(), "smolcorp")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceProcess(notExecutable, nil, nil); err == nil {
		t.Fatal("a file that can't be run became the new copy")
	}
}

// A program that ends before the restart is no longer anyone's to hand on,
// and one handed on that's already gone doesn't hold anything up.
func TestAChildThatEndedIsNotHandedOn(t *testing.T) {
	cmd := exec.Command("true")
	if err := startAndWait(cmd); err != nil {
		t.Fatal(err)
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	for deadline := time.Now().Add(3 * time.Second); strings.Contains(childrenNote(), pid); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s is still in %q after it ended", pid, childrenNote())
		}
	}
	// A pid that's nobody's child here: waiting for it comes back at once.
	done := make(chan bool)
	go func() { waitForChild(cmd.Process.Pid); done <- true }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("waiting for a child that's already gone never came back")
	}
}
