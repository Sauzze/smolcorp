//go:build !windows

package desktop

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The real-menu check (P3), run by hand only:
//
//	SMOLCORP_REAL_MENU=1 SMOLCORP_REAL_MENU_HOME=$HOME go test -run TestRealClaudeMenusTakeTheDigit -v ./desktop
//
// A test copy on port 7799 (this deck, served on 127.0.0.1 only, no app and no window) with its own hire: a real
// Claude Code in a tmux server on the copy's own hire socket, started with --setting-sources project,local, so its
// hooks report here and nowhere else, in a scratch folder under build/. The window's picks then answer Claude Code's
// two menus by their digit: a permission prompt's Yes (1) and No (Escape), and AskUserQuestion's second option.
// What the pane shows around each pick is logged, for the report. It costs three short Haiku turns.
func TestRealClaudeMenusTakeTheDigit(t *testing.T) {
	// The tests' home is a temp folder (TestMain): Claude Code is signed in in yours, given here.
	home := os.Getenv("SMOLCORP_REAL_MENU_HOME")
	if os.Getenv("SMOLCORP_REAL_MENU") != "1" || !filepath.IsAbs(home) {
		t.Skip("a real Claude Code: SMOLCORP_REAL_MENU=1 SMOLCORP_REAL_MENU_HOME=$HOME runs it")
	}
	claude, err := exec.LookPath("claude")
	tmux := findTmux()
	if err != nil || tmux == "" {
		t.Skip("needs claude and tmux")
	}
	d := newTestDeck(t)
	onMarkedPort(t, 7799)
	support := os.Getenv(testSupportVariable)
	d.mu.Lock()
	hookKey, window := d.access.HookKey, d.access.WindowKey
	d.mu.Unlock()
	if err := os.WriteFile(hookKeyFileFor(support), []byte(hookKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The copy, on this computer only.
	listener, err := net.Listen("tcp", "127.0.0.1:7799")
	if err != nil {
		t.Fatalf("port 7799: %v", err)
	}
	server := &http.Server{Handler: &router{deck: d}}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	// Its hook: this program's own, reporting to 7799 with the copy's key.
	bin := filepath.Join(t.TempDir(), "smolcorp")
	if out, err := exec.Command("go", "build", "-o", bin, "../cmd/smolcorp").CombinedOutput(); err != nil {
		t.Fatalf("building the hook: %v %s", err, out)
	}
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse", "PostToolUseFailure",
		"Notification", "Stop", "SessionEnd"} {
		hooks[event] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellQuoted(bin) + " hook --port 7799"}}}}
	}
	settings := filepath.Join(support, "claude-settings.json")
	data, _ := json.Marshal(map[string]any{"hooks": hooks})
	if err := os.WriteFile(settings, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// A scratch folder inside this repo, which Claude Code already trusts: a repo of its own would ask whether to
	// trust it, which nothing here answers.
	repo, err := filepath.Abs("../build/talkpick-real")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(repo)
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}

	// The copy's own hire: its socket named for port 7799, in tmux's folder, as a test copy's hire has it.
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = tmuxSocketFolder
	socket := filepath.Join(tmuxSocketFolder(), hireSocketPrefix+"7799-talkpick")
	if !ownsHireSocket(socket, 7799) {
		t.Fatalf("%s isn't the copy's own", socket)
	}
	run := func(args ...string) string {
		cmd := exec.Command(tmux, append([]string{"-S", socket}, args...)...)
		// Not this session's: no Claude Code of ours, no tmux of ours; and your home, not the tests'.
		cmd.Env = []string{"HOME=" + home}
		for _, v := range os.Environ() {
			name, _, _ := strings.Cut(v, "=")
			if !slices.Contains([]string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "APPDATA", "TMUX", "TMUX_PANE", "CLAUDECODE", "CLAUDE_PID",
				"CLAUDE_EFFORT"}, name) && !strings.HasPrefix(name, "CLAUDE_CODE_") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out))
	}
	command := strings.Join([]string{shellQuoted(claude), "--model", "haiku", "--permission-mode", "default",
		"--setting-sources", "project,local", "--settings", shellQuoted(settings)}, " ")
	run("new-session", "-d", "-s", "hire", "-x", "160", "-y", "45", "-c", repo, "-e", testSupportVariable+"="+support,
		"-e", "SMOLCORP_PORT=7799", "/bin/bash", "-c", command)
	serverPID := run("display-message", "-p", "#{pid}")
	t.Logf("the hire's tmux server: pid %s, socket %s", serverPID, socket)
	t.Cleanup(func() {
		run("kill-server")
		if pid := strings.TrimSpace(serverPID); pid != "" {
			_ = exec.Command("kill", pid).Run()
		}
		_ = os.RemoveAll(repo)
	})
	screen := func() string { return run("capture-pane", "-p", "-t", "hire") }
	say := func(text string) {
		run("send-keys", "-t", "hire", "-l", "--", text)
		time.Sleep(300 * time.Millisecond)
		run("send-keys", "-t", "hire", "Enter")
	}
	// The session, once it reported in.
	session := ""
	waitFor := func(what string, most time.Duration, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(most); !ok(); time.Sleep(500 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("no %s after %v; the pane:\n%s", what, most, screen())
			}
		}
	}
	// Its box, and never a question about trusting the folder, which nothing here answers.
	waitFor("ready prompt", 60*time.Second, func() bool {
		if strings.Contains(strings.ToLower(screen()), "trust") {
			t.Fatalf("Claude Code asks about the folder:\n%s", screen())
		}
		return strings.Contains(screen(), "❯")
	})
	asking := func() *talkAsking {
		d.mu.Lock()
		defer d.mu.Unlock()
		for _, s := range d.sessions.All() {
			if strings.HasSuffix(s.Tab, hireSocketPrefix+"7799-talkpick") {
				session = s.ID
				return talkAskingOf(s)
			}
		}
		return nil
	}
	pick := func(n int, since int64) int {
		target := fmt.Sprintf("http://127.0.0.1:7799/talk/pick?session=%s&n=%d&since=%d&t=%s", url.QueryEscape(session), n, since, url.QueryEscape(window))
		answer, err := http.Post(target, "text/plain", nil)
		if err != nil {
			t.Fatal(err)
		}
		answer.Body.Close()
		return answer.StatusCode
	}
	exists := func(name string) bool { _, err := os.Stat(filepath.Join(repo, name)); return err == nil }

	// 1. A permission prompt: Yes is the digit 1.
	say("Use the Bash tool to run exactly this command: touch yes-by-digit.txt . Do nothing else and do not explain.")
	waitFor("permission prompt", 90*time.Second, func() bool { a := asking(); return a != nil && a.Kind == "permission" })
	a := asking()
	t.Logf("asking: kind %s, tool %s, label %q, pickable %v, since %d\n--- the pane before Yes ---\n%s", a.Kind, a.Tool, a.Label, a.Pickable, a.Since, screen())
	if status := pick(1, a.Since); status != http.StatusNoContent {
		t.Fatalf("Yes: %d", status)
	}
	waitFor("the file Yes allowed", 30*time.Second, func() bool { return exists("yes-by-digit.txt") })
	waitFor("the turn's end", 60*time.Second, func() bool { return asking() == nil && strings.Contains(screen(), "❯") })
	t.Logf("--- the pane after Yes (yes-by-digit.txt made) ---\n%s", screen())

	// 2. A permission prompt: No is Escape, and the desk is free again.
	say("Use the Bash tool to run exactly this command: touch no-by-digit.txt . Do nothing else and do not explain.")
	waitFor("second permission prompt", 90*time.Second, func() bool { a := asking(); return a != nil && a.Kind == "permission" })
	a = asking()
	t.Logf("--- the pane before No ---\n%s", screen())
	if status := pick(2, a.Since); status != http.StatusNoContent {
		t.Fatalf("No: %d", status)
	}
	if again := pick(2, a.Since); again != http.StatusConflict {
		t.Errorf("a pick after No: %d, want 409 (it stopped asking)", again)
	}
	time.Sleep(3 * time.Second)
	if exists("no-by-digit.txt") {
		t.Fatal("No let the command run")
	}
	d.mu.Lock()
	state := d.sessions.Get(session).State
	d.mu.Unlock()
	t.Logf("--- the pane after No (state %s, no-by-digit.txt not made) ---\n%s", state, screen())
	if state == Asking {
		t.Fatal("still asking after No")
	}

	// 3. AskUserQuestion: its second option by the digit 2.
	say("Use the AskUserQuestion tool to ask me one single-choice question, \"Which fruit?\", with exactly three options " +
		"in this order: Apple, Banana, Cherry. After I answer, reply with only the fruit I chose, in capitals.")
	waitFor("AskUserQuestion", 90*time.Second, func() bool { a := asking(); return a != nil && a.Kind == "question" })
	a = asking()
	t.Logf("asking: kind %s, pickable %v, questions %+v\n--- the pane before 2 ---\n%s", a.Kind, a.Pickable, a.Questions, screen())
	if !a.Pickable || len(a.Questions) != 1 || len(a.Questions[0].Options) < 2 || a.Questions[0].Options[1].Label != "Banana" {
		t.Fatalf("the question's card: %+v", a)
	}
	if status := pick(2, a.Since); status != http.StatusNoContent {
		t.Fatalf("Banana: %d", status)
	}
	if again := pick(1, a.Since); again != http.StatusNoContent && again != http.StatusConflict {
		t.Errorf("a second pick: %d, want 204 (nothing pressed) or 409 (answered already)", again)
	}
	waitFor("the answer", 60*time.Second, func() bool { return asking() == nil && strings.Contains(screen(), "BANANA") })
	t.Logf("--- the pane after 2 ---\n%s", screen())
	if strings.Contains(screen(), "APPLE") {
		t.Fatal("the second pick reached the menu too")
	}
}
