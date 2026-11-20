//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXANSWER126, end to end: a Claude Code session in a tmux server of your own (not Smolcorp's, not a desk's), answered
// from the office's bubble through the guarded bundled tmux, as 1.26 drives every tmux tab (tmuxClient, "unchecked").
// The server is a stock tmux on a socket of its own, standing in for yours; its pane's claude is this test program
// (dictationCatArgument), with Claude Code's question menu printed above it. Nothing of yours is touched, and the
// server is ended by its PID.

// guardedTmuxForRows is the guarded tmux to drive with (SMOLCORP_TEST_TMUX, or this checkout's scripts/build-tmux.sh
// build), or the row is skipped.
func guardedTmuxForRows(t *testing.T) string {
	t.Helper()
	path := os.Getenv("SMOLCORP_TEST_TMUX")
	if path == "" {
		if found, _ := filepath.Glob("../build/tmux/*/tmux"); len(found) > 0 {
			path, _ = filepath.Abs(found[len(found)-1])
		}
	}
	if path == "" {
		t.Skip("SKIPPED: set SMOLCORP_TEST_TMUX to a guarded tmux, or run scripts/build-tmux.sh")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("SKIPPED: " + err.Error())
	}
	return path
}

// ownTmuxPane is a stock tmux server on a socket of its own and a pane showing menu, whose claude reads its keys into
// out: started by the pane's shell with exec (the agent the pane's own process), or as that shell's child, the shell
// then left in front (`sh -c 'claude …; exec sh'`).
func ownTmuxPane(t *testing.T, menu string, child bool) (tab, out string) {
	t.Helper()
	needTmux(t)
	// Short: a socket's path has a length limit. The system's temporary folder, never /tmp itself.
	dir, err := os.MkdirTemp("", "own")
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(dir, "claude")
	if err := os.Symlink(self, claude); err != nil {
		t.Fatal(err)
	}
	screen, socket := filepath.Join(dir, "menu"), filepath.Join(dir, "s")
	out = filepath.Join(dir, "out")
	if err := os.WriteFile(screen, []byte(menu), 0o644); err != nil {
		t.Fatal(err)
	}
	tmux := func(args ...string) string {
		output, err := exec.Command("tmux", append([]string{"-S", socket, "-f", "/dev/null"}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(output))
	}
	run := "exec " + claude + " " + dictationCatArgument + " > " + out
	if child {
		run = claude + " " + dictationCatArgument + " > " + out + "; exec sh"
	}
	tmux("new-session", "-d", "-x", "100", "-y", "40", "sh", "-c", "cat "+screen+"; stty raw -echo; touch "+out+".ready; "+run)
	pid, err := strconv.Atoi(tmux("display-message", "-p", "#{pid}"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		os.RemoveAll(dir)
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(out + ".ready"); err == nil {
			break
		}
	}
	time.Sleep(200 * time.Millisecond) // the claude started
	return "tmux:" + tmux("display-message", "-p", "#{pane_id}") + "@" + socket, out
}

func TestAnAnswerReachesASessionInYourOwnTmuxThroughTheGuardedTmux(t *testing.T) {
	guarded := guardedTmuxForRows(t)
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return guarded }
	menu := questionMenu("Put it live now?", "Yes", "Later")

	for _, child := range []bool{false, true} {
		t.Run(map[bool]string{false: "the agent in front", true: "the shell in front"}[child], func(t *testing.T) { ownTmuxPick(t, guarded, menu, child) })
	}
}

// ownTmuxPick is one pane's pick, as TestAnAnswerReachesASessionInYourOwnTmuxThroughTheGuardedTmux says it.
func ownTmuxPick(t *testing.T, guarded, menu string, child bool) {
	tab, out := ownTmuxPane(t, menu, child)
	if program, guard := tmuxFor(strings.SplitN(tab, "@", 2)[1]); program != guarded || guard != tmuxUnchecked {
		t.Fatalf("your own tmux is driven by %q, guard %q: want the guarded tmux, unchecked", program, guard)
	}
	d := newTestDeck(t)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PreToolUse", "session_id": "w1", "cwd": "/work/web", "tool_name": "AskUserQuestion",
		"tool_input": map[string]any{"questions": []any{talkQuestionOf("Put it live now?", false, "Yes", "Later")}}}, Claude, tab, "", 200)
	d.mu.Unlock()
	got := talkPickAt(t, d, "w1", "1", talkSinceOf(d, "w1"))
	// The tab still there through the guarded tmux: the 10-second look keeps the desk.
	d.pruneClosedTabs()
	d.mu.Lock()
	kept := d.sessions.Get("w1") != nil
	d.mu.Unlock()
	if !kept {
		t.Fatalf("child %v: the desk was ended (the pick said %d %q)", child, got.Code, got.Body)
	}
	if !child {
		if got.Code != http.StatusNoContent || quiet(out) != "1" {
			t.Fatalf("the agent in front: %d %q, its pane got %q: want 204 and the digit 1", got.Code, got.Body, quiet(out))
		}
		return
	}
	// The shell in front, the agent its child: nothing pressed, said, the desk still asking.
	if got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkNotInFront || quiet(out) != "" {
		t.Fatalf("the shell in front: %d %q, its pane got %q: want 409 %q and nothing", got.Code, got.Body, quiet(out), talkNotInFront)
	}
	if talkSinceOf(d, "w1") == 0 {
		t.Fatal("the shell in front: the desk stopped asking")
	}
}
