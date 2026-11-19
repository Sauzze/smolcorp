//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// stubPlainHireTmux gives each plain hire's own tmux server a stand-in runner that keeps its calls in h (past the
// "-S <socket>" every call names) and its socket in h.sockets, and fails each call while h.fail is set.
func stubPlainHireTmux(t *testing.T, h *hiring) {
	t.Helper()
	saved := plainHireTmux
	t.Cleanup(func() { plainHireTmux = saved })
	plainHireTmux = func(id string) *hireTmux {
		socket := filepath.Join("/tmp/tmux-1000", hireSocketName(id, 0))
		h.sockets = append(h.sockets, socket)
		return &hireTmux{socket: socket, run: func(args []string) (string, int, bool) {
			if len(args) < 2 || args[0] != "-S" || args[1] != socket {
				t.Errorf("a plain hire's call on another socket: %q", args)
				return "", 1, true
			}
			h.tmux = append(h.tmux, args[2:])
			if h.fail != nil {
				return "", 1, true
			}
			return "%7 4242 " + socket, 0, true
		}}
	}
}

// A plain hire in tmux (Windows has none) gets a server of its own.
func TestAHireCanRunInTmux(t *testing.T) {
	d, h, folder := hiringDeck(t, "/usr/bin/tmux")
	d.config.HireInTmux = true
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("hire in tmux: %d %s", status, message)
	}
	if len(h.tmux) != 1 || h.tmux[0][0] != "new-session" || !slices.Contains(h.tmux[0], folder) {
		t.Fatalf("tmux was asked %q, want a session of its own in the folder", h.tmux)
	}
	// The launcher, as tmux runs several words: no shell (RT126SG2 B3).
	if at := slices.Index(h.tmux[0], folder); len(h.tmux[0]) < at+3 || !filepath.IsAbs(h.tmux[0][at+1]) || h.tmux[0][at+2] != "run" {
		t.Errorf("tmux runs %q, want the launcher with no shell", h.tmux[0])
	}
	// On a server of its own, never the owner's default one (SA-M10): the socket named for a plain hire's own id.
	if len(h.sockets) != 1 || !strings.HasPrefix(filepath.Base(h.sockets[0]), hireSocketPrefix+"plain-") {
		t.Fatalf("its server's sockets: %q, want one of its own", h.sockets)
	}
	// Its window runs `smolcorp attach` on the session and the socket of its own server (SA-M8).
	if want := plainAttachLine(commandLineProgram(), hireSession, h.sockets[0]); len(h.windows) != 1 || !slices.Equal(h.windows[0][3:], want) {
		t.Errorf("the window runs %q, want it to show the session", h.windows)
	}
	// With no desktop, as over SSH, the session alone: tmux attach shows it.
	canOpenWindows = func() bool { return false }
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.tmux) != 2 || len(h.windows) != 1 {
		t.Errorf("a hire with no desktop: %d, tmux %d, windows %d", status, len(h.tmux), len(h.windows))
	}
}

// The launcher stand-in's places (plainenv_test.go), told it by the test that runs it: its desks' socket folder, never
// the real /tmp/smolcorp-<uid> (with none given, every sandboxed start is refused); no login shell asked; and, where
// asked, a stand-in hook key, a stand-in secret and a stand-in fence in its support folder, whose paths its agent is
// told.
const (
	testDeskSocketsVariable = "SMOLCORP_TEST_DESK_SOCKETS"
	testHookKeyVariable     = "SMOLCORP_TEST_HOOK_KEY"
	testSecretVariable      = "SMOLCORP_TEST_SECRET"
	testFenceVariable       = "SMOLCORP_TEST_FENCE"
)

func launcherStandInPlaces() int {
	parent := os.Getenv(testDeskSocketsVariable)
	deskSocketParent = func() string { return parent }
	tmuxLoginPath = func() []string { return nil }
	if key := os.Getenv(testHookKeyVariable); key != "" {
		if os.WriteFile(hookKeyFile, []byte(key+"\n"), 0o600) != nil {
			return 3
		}
	}
	if os.Getenv(testSecretVariable) != "" {
		secret, fence := filepath.Join(supportDir, "setup-key"), filepath.Join(supportDir, hireGuardFile)
		if os.WriteFile(secret, []byte("fake-secret-1\n"), 0o600) != nil || os.WriteFile(fence, []byte("{}\n"), 0o600) != nil {
			return 3
		}
		os.Setenv(testSecretVariable, secret)
		os.Setenv(testFenceVariable, fence)
	}
	return 0
}

// launcherSandboxStandIn is what a test that runs the launcher stand-in adds to its environment, so a Mac's start of a
// plain hire's agent in its sandbox (SA-M10) can run: this test program as `smolcorp sandbox`, and a desks' socket
// folder of its own, short enough for a socket. A stand-in agent that is this test program again unsets
// deskStandInVariable, or it runs as `smolcorp sandbox` too.
func launcherSandboxStandIn(t *testing.T) []string {
	t.Helper()
	return []string{deskStandInVariable + "=1", testDeskSocketsVariable + "=" + shortDeskSockets(t)}
}
