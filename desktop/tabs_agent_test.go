//go:build !windows

package desktop

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePane puts tmux and ps on PATH that show a pane on /dev/pts/7 running
// what's given, one command line each: the pane's own process first, the
// job in front after it. They keep every tmux command they're given.
func fakePane(t *testing.T, running string) (setRunning func(string), typed func() string) {
	t.Helper()
	bin := t.TempDir()
	says, sent := filepath.Join(bin, "ps-says"), filepath.Join(bin, "tmux-sent")
	scripts := map[string]string{
		"tmux": `#!/bin/sh
case "$*" in
  *%2*pane_tty*) ;;
  *pane_pid*) echo /dev/pts/7 100 ;;
  *pane_tty*) echo /dev/pts/7 ;;
  *pane_id*) echo %1 ;;
  *) echo "$*" >> "` + sent + `" ;;
esac
`,
		"ps": `#!/bin/sh
[ "$2" = "pts/7" ] || exit 1
cat "` + says + `"
`,
	}
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	setRunning = func(lines string) {
		if err := os.WriteFile(says, []byte(psRows(lines)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	setRunning(running)
	return setRunning, func() string { data, _ := os.ReadFile(sent); return string(data) }
}

func paneDeck(t *testing.T) *Deck {
	d := newTestDeck(t)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "p1", "cwd": "/work/api"},
		Claude, "tmux:%1@/tmp/tmux-sock", "", 100)
	return d
}

func TestKeysGoToAPaneOnlyWhileItsAgentRunsThere(t *testing.T) {
	setRunning, typed := fakePane(t, "bash\nclaude\n")
	d := paneDeck(t)
	if status, _ := d.Perform("accept", "p1", url.Values{}); status != http.StatusNoContent || !strings.Contains(typed(), "Enter") {
		t.Fatalf("accept with the agent running gave %d, typed %q", status, typed())
	}
	// It crashed: only the shell is left in the pane.
	setRunning("bash\n")
	before := typed()
	if status, message := d.Perform("esc", "p1", url.Values{}); status != http.StatusGone {
		t.Fatalf("a key for an agent that has gone gave %d %q", status, message)
	}
	if typed() != before {
		t.Fatalf("a key reached the shell left in the pane: %q", strings.TrimPrefix(typed(), before))
	}
	if d.sessions.Get("p1") != nil {
		t.Fatal("the session of an agent that has gone stayed on its desk")
	}
}

func TestAnAgentRunningOnNodeIsStillThere(t *testing.T) {
	// Claude Code installed from npm shows as node running its package, or bun.
	for _, shows := range []string{
		"-bash\nnode /usr/lib/node_modules/@anthropic-ai/claude-code/cli.js\n",
		"-bash\n/usr/local/bin/bun /home/someone/.bun/install/global/node_modules/@anthropic-ai/claude-code/cli.js\n",
	} {
		_, typed := fakePane(t, shows)
		d := paneDeck(t)
		if status, message := d.Perform("down", "p1", url.Values{}); status != http.StatusNoContent || !strings.Contains(typed(), "Down") {
			t.Fatalf("with the agent showing as %q, a key gave %d %q", strings.Split(shows, "\n")[1], status, message)
		}
		if d.sessions.Get("p1") == nil {
			t.Fatalf("a live agent showing as %q was taken as ended", strings.Split(shows, "\n")[1])
		}
	}
}

func TestNodeRunningSomethingElseIsntTheAgent(t *testing.T) {
	// It crashed, and a dev server was started in its pane: keys meant for it mustn't reach that.
	_, typed := fakePane(t, "-bash\nnode /work/app/node_modules/vite/bin/vite.js\n")
	d := paneDeck(t)
	if status, message := d.Perform("down", "p1", url.Values{}); status != http.StatusGone || strings.Contains(typed(), "Down") {
		t.Fatalf("a key with vite in the agent's pane gave %d %q, typed %q", status, message, typed())
	}
}

func TestAPaneThatCantBeAskedStillGetsItsKeys(t *testing.T) {
	_, typed := fakePane(t, "bash\n")
	d := paneDeck(t)
	// tmux says nothing of this pane's tty, so what runs there isn't known: unknown isn't gone
	d.mu.Lock()
	d.sessions.Get("p1").Tab = "tmux:%2@/tmp/tmux-sock"
	d.mu.Unlock()
	if status, _ := d.Perform("up", "p1", url.Values{}); status != http.StatusNoContent || !strings.Contains(typed(), "Up") {
		t.Fatalf("a key for a pane that couldn't be asked gave %d, typed %q", status, typed())
	}
}

func fakeTool(t *testing.T, name, script string) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestKittySaysWhatRunsInItsWindow(t *testing.T) {
	// As kitty @ ls answers: OS windows, their tabs, and each window's own process.
	fakeTool(t, "kitty", `cat <<'JSON'
[{"id": 1, "tabs": [{"id": 1, "windows": [
  {"id": 3, "pid": 40, "cmdline": ["/bin/zsh"]},
  {"id": 4, "pid": 41, "cmdline": ["/bin/zsh"]}
]}]}]
JSON
`)
	fakePS(t, map[string]string{
		"40": "ttys900", "41": "ttys901",
		"ttys900": "40 1 40 50 /bin/zsh\n50 40 50 50 /home/someone/.local/bin/claude --resume\n",
		"ttys901": "41 1 41 41 /bin/zsh\n",
	})
	for _, c := range []struct {
		id             string
		running, known bool
	}{{"3", true, true}, {"4", false, true}, {"9", false, false}} {
		running, known := commandTab{kind: "kitty", id: c.id, socket: "unix:/tmp/kitty"}.runs(Claude)
		if running != c.running || known != c.known {
			t.Errorf("kitty window %s: running %v, known %v; want %v, %v", c.id, running, known, c.running, c.known)
		}
	}
}

func TestWezTermSaysWhichTTYAPaneIsOn(t *testing.T) {
	fakeTool(t, "wezterm", `cat <<'JSON'
[{"window_id": 0, "tab_id": 0, "pane_id": 5, "title": "zsh", "tty_name": "/dev/pts/9"},
 {"window_id": 0, "tab_id": 1, "pane_id": 6, "title": "zsh", "tty_name": "/dev/pts/10"}]
JSON
`)
	if tty := (commandTab{kind: "wezterm", id: "6"}).tty(); tty != "/dev/pts/10" {
		t.Errorf("WezTerm pane 6 is on %q, want /dev/pts/10", tty)
	}
	if tty := (commandTab{kind: "wezterm", id: "7"}).tty(); tty != "" {
		t.Errorf("a WezTerm pane that isn't there is on %q", tty)
	}
}

// psRows are ps's lines for a tty running lines, as frontOn asks for them:
// the first the pane's own process, pid 100, and any others the job in
// front of it, process group 200.
func psRows(lines string) string {
	var rows strings.Builder
	commands := strings.Split(strings.TrimSpace(lines), "\n")
	front := 100
	if len(commands) > 1 {
		front = 200
	}
	for i, command := range commands {
		if i == 0 {
			fmt.Fprintf(&rows, "100 1 100 %d %s\n", front, command)
		} else {
			fmt.Fprintf(&rows, "%d 100 200 %d %s\n", 200+i, front, command)
		}
	}
	return rows.String()
}

// fakePS puts a ps on PATH that answers -p <pid> -o tty= and -t <tty> -o …
// from says, by the pid or the tty.
func fakePS(t *testing.T, says map[string]string) {
	t.Helper()
	var script strings.Builder
	script.WriteString("case \"$2\" in\n")
	for key, answer := range says {
		fmt.Fprintf(&script, "  %s) printf '%%s' '%s' ;;\n", key, answer)
	}
	script.WriteString("  *) exit 1 ;;\nesac\n")
	fakeTool(t, "ps", script.String())
}
