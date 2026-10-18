package desktop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

// TestEchoArgsForCmdLine stands in for an agent: run as one, it prints the
// arguments it was given, after its own "--", and ends there.
func TestEchoArgsForCmdLine(t *testing.T) {
	if os.Getenv("SMOLCORP_ECHO_ARGS") != "1" {
		return
	}
	args := os.Args
	if i := slices.Index(args, "--"); i >= 0 {
		args = args[i+1:]
	}
	_ = json.NewEncoder(os.Stdout).Encode(args)
	os.Exit(0)
}

func TestARoleMessageComesThroughCmdExeAsItWas(t *testing.T) {
	// An agent npm installed is a .cmd script that hands %* on to its program,
	// here this test itself, under a folder with a space, as a user's can be.
	shim := filepath.Join(t.TempDir(), "Jean Dupont", "agent.cmd")
	writeFile(t, shim, "@echo off\r\n\"%SMOLCORP_TEST_BINARY%\" -test.run=TestEchoArgsForCmdLine -- %*\r\n")
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = `C:\Windows\System32\cmd.exe`
	}
	want := []string{"--model=opus", "--effort=high", "--",
		`Review "the changes" & say 100% ^ done, with spaces; (and) <more> | here!`, `ends with a backslash \`}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine(shell, shim, want)}
	cmd.Env = append(os.Environ(), "SMOLCORP_ECHO_ARGS=1", "SMOLCORP_TEST_BINARY="+os.Args[0])
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("cmd.exe running the script: %v\n%s", err, out)
	}
	var got []string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("the agent printed %q: %v", out, err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("through cmd.exe, the agent was given\n%q\nwant\n%q", got, want)
	}
}
