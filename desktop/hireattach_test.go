//go:build !windows

package desktop

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// HIREUX: a hire's life never depends on a window. The window Smolcorp opens for a tmux hire is only a `tmux attach`
// client; the hire is the pane's own process on its own tmux server. These use a real tmux on a socket of the test's
// own, with a fake client (tmux's control mode, which needs no terminal) in place of the terminal's.

// fakeAttachClient is an attach client of the session on h, as a terminal window's: it ends with Close (what a
// closed window does, SIGHUP) or by detach-client. Ctrl-D types a ^D into the pane, as a person in the window would.
type fakeAttachClient struct {
	cmd   *exec.Cmd
	input io.WriteCloser
	gone  chan struct{}
}

func attachFake(t *testing.T, tmux string, h *hireTmux) *fakeAttachClient {
	t.Helper()
	c := &fakeAttachClient{cmd: exec.Command(tmux, "-S", h.socket, "-C", "attach", "-t", "="+hireSession), gone: make(chan struct{})}
	c.cmd.Env = hireEnvironment()
	c.cmd.Stdout = io.Discard
	var err error
	if c.input, err = c.cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = c.cmd.Wait(); close(c.gone) }()
	t.Cleanup(func() { _ = c.cmd.Process.Kill() })
	untilHolds(t, "the client attaches", func() bool { return clientsOf(h) == 1 })
	return c
}

// clientsOf is how many clients are attached: by pid, as a control client has no tty for viewers to count.
func clientsOf(h *hireTmux) int {
	out, status, ran := h.run(h.command("list-clients", "-t", "="+hireSession, "-F", "#{client_pid}"))
	if !ran || status != 0 {
		return -1
	}
	return len(strings.Fields(out))
}

func (c *fakeAttachClient) Close() { _ = c.cmd.Process.Signal(syscall.SIGHUP) }

func (c *fakeAttachClient) Ctrl_D() { _, _ = io.WriteString(c.input, "send-keys C-d\n") }

func untilHolds(t *testing.T, what string, holds func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if holds() {
			return
		}
	}
	t.Fatalf("gave up waiting until %s", what)
}

func TestAHiresSessionOutlivesItsWindow(t *testing.T) {
	path := needTmux(t)
	shortSupport(t)
	saved := tmuxPath
	tmuxPath = func() string { return path }
	t.Cleanup(func() { tmuxPath = saved })
	h := newHireTmux("it-attached", 0, nil)
	if h == nil {
		t.Fatal("no hire tmux")
	}
	folder := t.TempDir()
	agent := filepath.Join(folder, "fake-agent.sh")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\nexec cat\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := h.start(folder, []string{"/bin/sh", agent}); !ok {
		t.Fatal("start failed")
	}
	t.Cleanup(func() { h.endServer() })

	// Closing the window: its client gets SIGHUP and goes; the hire carries on.
	window := attachFake(t, path, h)
	window.Close()
	<-window.gone
	untilHolds(t, "no window is attached", func() bool { return clientsOf(h) == 0 })
	if up, _ := h.alive(); !up {
		t.Fatal("closing the window ended the hire")
	}

	// Detaching it (switching to the background, or tmux's own detach): the same.
	window = attachFake(t, path, h)
	if !h.detachViewers() {
		t.Fatal("detach failed")
	}
	<-window.gone
	if up, _ := h.alive(); !up {
		t.Fatal("detaching the window ended the hire")
	}

	// What does end it: the agent itself ending, as Ctrl-D does at an empty prompt of a program that reads its
	// input (the pane has the agent, not a shell, so `exit` is only text to it). Setup and docs/team.md say so.
	window = attachFake(t, path, h)
	window.Ctrl_D()
	untilHolds(t, "the agent ended", func() bool { up, _ := h.alive(); return !up })
}
