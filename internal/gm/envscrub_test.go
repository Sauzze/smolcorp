package gm

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ENVSCRUB1: `smolcorp gm login` typed in a terminal inside a Claude Code session starts the GM as a session of its
// own: none of that session's markers reach it, the owner's provider does, and its token is in its environment
// (SIERRA: the scrub never touches a SMOLCORP_* variable; without its token a GM can run no `smolcorp gm` command).
// The start is given to a stand-in that writes down the environment it was started with; no value is printed.
func TestLoginStartsTheGMWithItsTokenAndNoClaudeSessionMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	start := stubStart(t)
	written := filepath.Join(t.TempDir(), "env")
	standIn := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(standIn, []byte("#!/bin/sh\nenv > '"+written+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	startGM = func(program string, args, env []string) error {
		start.calledTimes++
		cmd := exec.Command(program, args[1:]...)
		cmd.Env = env
		return cmd.Run()
	}
	markers := []string{"CLAUDECODE", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT", "CLAUDE_CODE_SESSION_ID",
		"CLAUDE_PID", "CLAUDE_EFFORT", "AI_AGENT"}
	for _, name := range markers {
		t.Setenv(name, "fake-marker")
	}
	// Smolcorp's own, inherited from the terminal (as a test copy's SMOLCORP_PORT is): the scrub never touches it.
	t.Setenv("SMOLCORP_ENVSCRUB1_PROBE", "kept")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "FAKEKEY-bedrock")
	t.Setenv("CLAUDE_CONFIG_DIR", "/fake/config")
	app := newFakeApp(t, allowing(fakeLoginToken, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	if code, _, errOut := runGM(app.env(t, nil), "login", "--", standIn); code != 0 || start.calledTimes != 1 {
		t.Fatalf("exit %d, started %d times: %q", code, start.calledTimes, errOut)
	}
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("the stand-in GM didn't run: %v", err)
	}
	env := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, name := range markers {
		if slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, name+"=") }) {
			t.Errorf("the GM got %s", name)
		}
	}
	for _, kept := range []string{"CLAUDE_CODE_USE_BEDROCK=1", "AWS_BEARER_TOKEN_BEDROCK=FAKEKEY-bedrock", "SMOLCORP_ENVSCRUB1_PROBE=kept",
		"CLAUDE_CONFIG_DIR=/fake/config"} {
		if !slices.Contains(env, kept) {
			t.Errorf("the GM lacks the inherited %s", strings.SplitN(kept, "=", 2)[0])
		}
	}
	if !slices.Contains(env, tokenEnv+"="+fakeLoginToken) {
		t.Errorf("the GM's %s isn't its token (its value isn't printed)", tokenEnv)
	}
}
