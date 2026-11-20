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

	tf "github.com/Sauzze/smolcorp/internal/sandbox/tmuxfake"
)

// FXTMUXDIR, ruling (7)'s race, with stand-ins only: tmux's own folder is a stand-in in the test's temp folder
// (TMUX_TMPDIR), never the owner's /private/tmp/tmux-<uid> or its servers. A plain hire's server there is a real tmux
// of the guarded build's, started by the test (its own pid); a sandboxed desk cd34's server is a fake of the repo's
// (internal/sandbox/tmuxfake) at its socket in the desks' socket folder, listening in this process, and its row records
// that pid as the server the core checked. The core looks at the plain hire's socket (a socket, not a link), then the
// desk flips it into a link to cd34's socket, and only then does the core's unchecked client connect, as a tab of the
// plain hire's is driven (tmuxFor: guard unchecked). Told plain "unchecked" (the core before FXTMUXDIR), the client
// reaches cd34's server and sends it its command; told as the core tells it now, it stops at the connect and cd34's
// server is sent nothing. The plain hire's own server, unflipped, is still driven.

// raceTmux is the guarded tmux for the row (SMOLCORP_TEST_TMUX, or this checkout's scripts/build-tmux.sh build), or the
// row is skipped (failed under SMOLCORP_SANDBOX_REQUIRED=1).
func raceTmux(t *testing.T) string {
	t.Helper()
	path := os.Getenv("SMOLCORP_TEST_TMUX")
	if path == "" {
		found, _ := filepath.Glob("../build/tmux/*/tmux")
		if len(found) > 0 {
			path, _ = filepath.Abs(found[len(found)-1])
		}
	}
	why := ""
	if path == "" {
		why = "set SMOLCORP_TEST_TMUX to a guarded tmux, or run scripts/build-tmux.sh"
	} else if _, err := os.Stat(path); err != nil {
		why = err.Error()
	}
	if why != "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(why)
		}
		t.Skip("SKIPPED: " + why)
	}
	return path
}

// plainTmuxServer starts a real server of tmux's on socket, as a plain hire's is started (its guard off for the start
// alone, no start file), and gives its pid. It is ended by that pid when the test ends.
func plainTmuxServer(t *testing.T, tmux, socket string) int {
	t.Helper()
	cmd := exec.Command(tmux, "-f", os.DevNull, "-S", socket, "new-session", "-d", "-P", "-F", "#{pid}", "-s", "plain", "cat")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "TERM=xterm-256color", "LANG=en_US.UTF-8", "HOME=" + filepath.Dir(socket), tmuxGuardVariable + "=off"}
	out, err := cmd.Output()
	pid, perr := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || perr != nil || pid <= 1 {
		t.Fatalf("the plain hire's stand-in server didn't start: %v %q", err, out)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 50 && syscall.Kill(pid, 0) == nil; i++ {
			time.Sleep(20 * time.Millisecond)
		}
		_ = syscall.Kill(pid, syscall.SIGKILL)
	})
	return pid
}

func TestAnUncheckedClientNeverReachesADesksServerThroughALinkFlippedAfterTheLook(t *testing.T) {
	tmux := raceTmux(t)
	shortSupport(t)
	deskSockets(t)
	base, err := os.MkdirTemp("", "tx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	t.Setenv("TMUX_TMPDIR", physical(base))
	folder := tmuxSocketFolder()
	if !strings.HasPrefix(folder, physical(base)+"/") || !ownSocketFolder(folder) {
		t.Fatalf("the stand-in tmux folder %q isn't the test's own", folder)
	}

	// Desk cd34 in its sandbox: its server a fake in this process, its row recording this pid as the one checked.
	deskSocket := deskSocketPath("cd34", 0)
	if err := makeDeskSocketFolder(deskSocket); err != nil {
		t.Fatal(err)
	}
	desk, err := tf.Listen(deskSocket, tf.Step{After: tf.MsgCommand, Send: []tf.Message{tf.Exit(0)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(desk.Close)
	writeRegistryHires(t, Hire{ID: "cd34", Name: "Sandboxed", State: "working", TmuxSocket: deskSocket, TmuxProgram: tmux,
		TmuxServer: os.Getpid(), Sandboxed: true})

	// A plain hire's server in tmux's own folder, driven unchecked, as its tabs are; it answers as itself.
	plain := filepath.Join(folder, hireSocketName("ab12", 0))
	plainPid := plainTmuxServer(t, tmux, plain)
	if _, guard := tmuxFor(plain); guard != tmuxUnchecked {
		t.Fatalf("a plain hire's socket is driven with guard %q, not unchecked", guard)
	}
	drive := func(guard string) (string, int, bool) {
		return oneShotTmux(tmux, func() string { return guard })([]string{"-S", plain, "display-message", "-p", "#{pid}"})
	}
	if out, status, ran := drive(tmuxUnchecked); !ran || status != 0 || out != strconv.Itoa(plainPid) {
		t.Fatalf("the plain hire's own server wasn't driven: %q %d %v", out, status, ran)
	}

	// The look: a socket, not a link, as any check the core makes before the call sees it.
	if info, err := os.Lstat(plain); err != nil || info.Mode().Type() != os.ModeSocket {
		t.Fatalf("the plain hire's socket at the look: %v %v", info, err)
	}
	// After the look, before the connect: the desk's flip, a link to cd34's socket at the plain hire's name.
	if err := os.Remove(plain); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(deskSocket, plain); err != nil {
		t.Fatal(err)
	}

	// As the core told it before FXTMUXDIR (plain "unchecked", which the guard takes as it ever did): the race is
	// open, the client reaches the desk's server and sends it its command.
	open := exec.Command(tmux, "-f", os.DevNull, "-S", plain, "has-session")
	open.Env = append(append([]string{}, tmuxClientEnvironment...), tmuxGuardVariable+"="+tmuxUnchecked)
	openErr := open.Run()
	time.Sleep(100 * time.Millisecond)
	reached := 0
	for _, s := range desk.Sessions() {
		if s.Has(tf.MsgCommand) {
			reached++
		}
	}
	t.Logf("before FXTMUXDIR (guard %q): %v, the desk's server was sent a command %d time(s)", tmuxUnchecked, openErr, reached)
	if openErr != nil || reached != 1 {
		t.Fatalf("the stand-in race didn't reach the desk's server as the core before the fix: %v, %d", openErr, reached)
	}

	// As the core tells it now, through the way a plain hire's tab is driven: stopped at the connect.
	sessions := len(desk.Sessions())
	out, status, ran := drive(tmuxUnchecked)
	time.Sleep(100 * time.Millisecond)
	t.Logf("now: %q, status %d, ran %v; the desk's server had %d more connection(s)", out, status, ran, len(desk.Sessions())-sessions)
	if !ran || status != 98 {
		t.Errorf("the core's unchecked client went through to the desk's server: %q %d %v", out, status, ran)
	}
	for _, s := range desk.Sessions()[sessions:] {
		if len(s.Received) != 0 {
			t.Errorf("the desk's server was sent %d messages by the core's unchecked client", len(s.Received))
		}
	}

	// The plain hire's server, put back at its name, is driven as before.
	if err := os.Remove(plain); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(folder, hireSocketName("ef56", 0))
	otherPid := plainTmuxServer(t, tmux, other)
	if out, status, ran := oneShotTmux(tmux, func() string { return tmuxUnchecked })([]string{"-S", other, "display-message", "-p", "#{pid}"}); !ran || status != 0 || out != strconv.Itoa(otherPid) {
		t.Errorf("a plain hire's server isn't driven any more: %q %d %v", out, status, ran)
	}
}
