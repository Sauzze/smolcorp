//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A test copy picks in its own hire's tmux pane, and in no other tmux server's: one on another port's socket or
// outside the hires' folder is the owner's.
func TestATestCopyPicksInItsOwnHiresPaneOnly(t *testing.T) {
	_, typed := fakePane(t, "bash\nclaude\n")
	sockets := t.TempDir()
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = func() string { return sockets }
	onMarkedPort(t, 7799)
	d := newTestDeck(t)
	for id, socket := range map[string]string{
		"own":     filepath.Join(sockets, hireSocketPrefix+"7799-ab12"),
		"real":    filepath.Join(sockets, hireSocketPrefix+"ab12"),
		"another": filepath.Join(sockets, hireSocketPrefix+"7798-ab12"),
		"default": "/tmp/tmux-501/default",
	} {
		d.mu.Lock()
		d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": id, "cwd": "/work/" + id, "tool_name": "Bash",
			"tool_input": map[string]any{"command": "touch x"}},
			Claude, "tmux:%1@"+socket, "", 200)
		d.mu.Unlock()
	}
	paneShows(t, permissionMenu("touch x"))
	for _, id := range []string{"real", "another", "default"} {
		if got := talkPickAt(t, d, id, "1", talkSinceOf(d, id)); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != testCopyKeepsTerminal {
			t.Errorf("%s: %d %s", id, got.Code, got.Body)
		}
	}
	if sent := typed(); sent != "" {
		t.Fatalf("the owner's panes were sent %q", sent)
	}
	if got := talkPickAt(t, d, "own", "1", talkSinceOf(d, "own")); got.Code != http.StatusNoContent {
		t.Fatalf("its own hire's pane: %d %s", got.Code, got.Body)
	}
	if sent := typed(); !strings.Contains(sent, "capture-pane -p -t %1") || !strings.Contains(sent, "send-keys -t %1 1") {
		t.Fatalf("its own hire's pane was sent %q, want its screen read and the digit 1", sent)
	}
}

// RT126P3 B1, in a real tmux pane's way: the screen is read by capture-pane, right before the key, and with no menu
// on it nothing is sent.
func TestAPickReadsTheTmuxPaneBeforeItsKey(t *testing.T) {
	_, typed := fakePane(t, "bash\nclaude\n")
	d := newTestDeck(t)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "p1", "cwd": "/work/api", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "touch x"}}, Claude, "tmux:%1@/tmp/tmux-sock", "", 200)
	d.mu.Unlock()
	paneShows(t, talkInputBox)
	if got := talkPickAt(t, d, "p1", "1", talkSinceOf(d, "p1")); got.Code != http.StatusConflict {
		t.Fatalf("no menu in the pane: %d %s", got.Code, got.Body)
	}
	if sent := typed(); !strings.Contains(sent, "capture-pane") || strings.Contains(sent, "send-keys") {
		t.Fatalf("no menu in the pane: sent %q, want it read and nothing pressed", sent)
	}
}

// paneShows has fakePane's pane show screen.
func paneShows(t *testing.T, screen string) {
	t.Helper()
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(tmux), "tmux-screen"), []byte(screen), 0o644); err != nil {
		t.Fatal(err)
	}
}
