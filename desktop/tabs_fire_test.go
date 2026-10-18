//go:build !windows

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAnAgentIsItsOwnProgramOrItsOwnPackage(t *testing.T) {
	for _, c := range []struct {
		agent  Agent
		line   string
		itself bool
	}{
		{Claude, "claude --resume", true},
		{Claude, "/home/someone/.local/bin/claude", true},
		{Claude, "node /home/some one/.npm/lib/node_modules/@anthropic-ai/claude-code/cli.js", true},
		{Codex, "/usr/lib/node_modules/@openai/codex-linux-x64/vendor/x86_64/codex/codex", true},
		{Codex, "node /usr/lib/node_modules/@openai/codex/bin/codex.js", true},
		{Gemini, "node --max-old-space-size=8192 /usr/lib/node_modules/@google/gemini-cli/dist/index.js", true},
		{OpenCode, "opencode", true},
		{Grok, "bun /home/someone/.bun/install/global/node_modules/@vibe-kit/grok-cli/dist/index.js", true},
		{Claude, "-zsh", false},
		{Claude, "node /work/app/node_modules/vite/bin/vite.js", false},
		{Claude, "node /work/app/node_modules/.bin/next dev", false},
		{Claude, "node server.js", false},
		{Claude, "node /work/claude-app/server.js", false},
		{Claude, "node /usr/lib/node_modules/@musistudio/claude-code-router/dist/cli.js start", false},
		{Claude, "bun run dev", false},
		{Claude, "", false},
	} {
		if got := c.agent.isItself(c.line); got != c.itself {
			t.Errorf("%s isItself(%q) = %v, want %v", c.agent, c.line, got, c.itself)
		}
	}
}

func TestWhatsInFrontOfAPane(t *testing.T) {
	const ps = `  100   90  100  200 -zsh
  200  100  200  200 node /x/node_modules/vite/bin/vite.js
  201  200  200  200 esbuild --service
`
	if f := frontOf(ps, 100); f.is != inFrontJob || f.group != 200 || len(f.job) != 2 || f.own != "-zsh" {
		t.Errorf("with a job in front: %+v", f)
	}
	if f := frontOf(ps, 0); f.is != inFrontJob || f.group != 200 {
		t.Errorf("the pane's own process found as the one none of the others started: %+v", f)
	}
	if f := frontOf("  100   90  100  100 -zsh\n", 100); f.is != inFrontShell {
		t.Errorf("with the shell in front: %+v", f)
	}
	if f := frontOf("  100   90  100  100 -zsh\n  300   80  300  100 sleep 9\n", 0); f.is != inFrontUnknown {
		t.Errorf("two processes nobody on the tty started can't be told apart: %+v", f)
	}
	if f := frontOf("", 100); f.is != inFrontShell || f.own != "" {
		t.Errorf("nothing at all on the tty, the shell ended too: %+v", f)
	}
	if f := frontOf("  100   90  100  100 -zsh\n  300  100  300  100 node server.js\n", 100); f.is != inFrontShell || !f.behind {
		t.Errorf("a job in the background: %+v", f)
	}
	if f := frontOf("  100   90  100  100 -zsh\n", 100); f.behind {
		t.Errorf("nothing in the background: %+v", f)
	}
}

// tmuxPane is a pane running sh in a tmux server of the test's own.
func tmuxPane(t *testing.T) commandTab {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux here")
	}
	// Short: a socket's path has a length limit.
	dir, err := os.MkdirTemp("/tmp", "smt")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "s")
	tmux := func(args ...string) string {
		output, err := exec.Command("tmux", append([]string{"-S", socket, "-f", "/dev/null"}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(output))
	}
	tmux("new-session", "-d", "-x", "80", "-y", "24", "sh")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
		os.RemoveAll(dir)
	})
	return commandTab{kind: "tmux", id: tmux("display-message", "-p", "#{pane_id}"), socket: socket}
}

// startJob runs a process that ps shows as commandLine in the pane, and
// waits for it to be in front: its process group.
func startJob(t *testing.T, pane commandTab, commandLine string) int {
	t.Helper()
	// With no environment: macOS's ps shows what follows the name in memory.
	script := "env -i /usr/bin/perl -e '$0 = q{" + commandLine + "}; sleep 300'"
	if err := pane.run("send-keys", "-t", pane.id, "-l", script); err != nil {
		t.Fatal(err)
	}
	if err := pane.run("send-keys", "-t", pane.id, "Enter"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if f := pane.front(); f.is == inFrontJob && len(f.job) > 0 && strings.HasPrefix(f.job[0], commandLine) {
			return f.group
		}
	}
	t.Fatalf("%q never came to the front: %+v", commandLine, pane.front())
	return 0
}

func groupRuns(group int) bool { return syscall.Kill(-group, 0) == nil }

func TestFiringInTmuxStopsTheAgentAndClosesItsPane(t *testing.T) {
	pane := tmuxPane(t)
	group := startJob(t, pane, "node /x/node_modules/@anthropic-ai/claude-code/cli.js")
	if is, known := pane.runs(Claude); !is || !known {
		t.Errorf("the agent runs in its pane: %v, %v", is, known)
	}
	if err := pane.Close(Claude); err != nil {
		t.Fatalf("firing: %v", err)
	}
	if groupRuns(group) {
		t.Error("the agent still runs")
	}
	if pane.alive() {
		t.Error("its pane is still open")
	}
}

func TestFiringInTmuxLeavesSomeoneElsesWorkAlone(t *testing.T) {
	for _, work := range []string{
		"node /x/node_modules/vite/bin/vite.js",
		"node server.js",
		"node /x/node_modules/@musistudio/claude-code-router/dist/cli.js start",
	} {
		t.Run(work, func(t *testing.T) {
			pane := tmuxPane(t)
			group := startJob(t, pane, work)
			if is, known := pane.runs(Claude); is || !known {
				t.Errorf("taken for the agent: %v, %v", is, known)
			}
			if err := pane.Close(Claude); !errors.Is(err, errNotStopped) {
				t.Errorf("firing: %v, want errNotStopped", err)
			}
			if !groupRuns(group) || !pane.alive() {
				t.Error("someone's work, or its pane, was stopped")
			}
			_ = syscall.Kill(-group, syscall.SIGKILL)
		})
	}
}

func TestFiringInTmuxClosesAPaneLeftToItsShell(t *testing.T) {
	pane := tmuxPane(t)
	if f := pane.front(); f.is != inFrontShell {
		t.Fatalf("a new pane: %+v", f)
	}
	if err := pane.Close(Claude); err != nil {
		t.Fatalf("firing: %v", err)
	}
	if pane.alive() {
		t.Error("the pane is still open")
	}
}

func TestFiringInKittyLooksFirst(t *testing.T) {
	closed := filepath.Join(t.TempDir(), "closed")
	// kitty @ --to <socket> <command> …
	fakeTool(t, "kitty", `case "$4" in
ls) cat <<'JSON'
[{"id": 1, "tabs": [{"id": 1, "windows": [
  {"id": 3, "pid": 40, "cmdline": ["/bin/zsh"]},
  {"id": 4, "pid": 41, "cmdline": ["/bin/zsh"]},
  {"id": 5, "pid": 42, "cmdline": ["/bin/zsh"]}
]}]}]
JSON
;;
close-window) echo "$6" >> `+closed+` ;;
esac
`)
	fakePS(t, map[string]string{
		"40": "ttys900", "41": "ttys901", "42": "ttys902",
		"ttys900": "40 1 40 40 /bin/zsh\n",
		"ttys901": "41 1 41 60 /bin/zsh\n60 41 60 60 node /x/node_modules/vite/bin/vite.js\n",
		"ttys902": "42 1 42 42 /bin/zsh\n70 42 70 42 node server.js\n",
	})
	if err := (commandTab{kind: "kitty", id: "4", socket: "unix:/tmp/kitty"}).Close(Claude); !errors.Is(err, errNotStopped) {
		t.Errorf("a window with someone's work in front: %v, want errNotStopped", err)
	}
	if err := (commandTab{kind: "kitty", id: "5", socket: "unix:/tmp/kitty"}).Close(Claude); !errors.Is(err, errNotStopped) {
		t.Errorf("a window whose shell runs something in the background: %v, want errNotStopped", err)
	}
	if err := (commandTab{kind: "kitty", id: "3", socket: "unix:/tmp/kitty"}).Close(Claude); err != nil {
		t.Errorf("a window left to its shell: %v", err)
	}
	if got, _ := os.ReadFile(closed); strings.TrimSpace(string(got)) != "id:3" {
		t.Errorf("closed %q, want only id:3", got)
	}
}

func TestFiringInTmuxLeavesAShellsBackgroundJobsAlone(t *testing.T) {
	pane := tmuxPane(t)
	if err := pane.run("send-keys", "-t", pane.id, "-l", "env -i /usr/bin/perl -e '$0 = q{node server.js}; sleep 300' &"); err != nil {
		t.Fatal(err)
	}
	_ = pane.run("send-keys", "-t", pane.id, "Enter")
	for deadline := time.Now().Add(5 * time.Second); !pane.front().behind; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the background job never showed: %+v", pane.front())
		}
	}
	if err := pane.Close(Claude); !errors.Is(err, errNotStopped) {
		t.Errorf("firing: %v, want errNotStopped", err)
	}
	if !pane.alive() {
		t.Error("the pane, and the job in its background, were closed")
	}
}

func TestAnAgentStoppedWithCtrlZIsntThereForKeys(t *testing.T) {
	pane := tmuxPane(t)
	startJob(t, pane, "node /x/node_modules/@anthropic-ai/claude-code/cli.js")
	if is, known := pane.runs(Claude); !is || !known {
		t.Fatalf("in front: %v, %v", is, known)
	}
	_ = pane.run("send-keys", "-t", pane.id, "C-z")
	for deadline := time.Now().Add(5 * time.Second); pane.front().is != inFrontShell; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the shell never came back in front: %+v", pane.front())
		}
	}
	if is, known := pane.runs(Claude); is || !known {
		t.Errorf("stopped, with the shell in front, it still took keys: %v, %v", is, known)
	}
	if f := pane.front(); !f.behind {
		t.Errorf("the stopped agent isn't seen behind the shell: %+v", f)
	}
}
