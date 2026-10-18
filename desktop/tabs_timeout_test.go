//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A terminal whose command line tool hangs, as kitty does while it asks on
// its own screen whether to allow remote control.
func hangingTerminal(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, name := range []string{"kitty", "tmux"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := terminalTimeout
	t.Cleanup(func() { terminalTimeout = saved })
	terminalTimeout = 300 * time.Millisecond
}

func TestAHungTerminalDoesntHoldUpTheKeys(t *testing.T) {
	hangingTerminal(t)
	started := time.Now()
	err := commandTab{kind: "kitty", id: "1", socket: "unix:/tmp/kitty"}.Press(press(Enter))
	if took := time.Since(started); took > terminalTimeout+2*time.Second {
		t.Fatalf("a hung kitty held the key for %v", took)
	}
	if err == nil {
		t.Fatal("a hung kitty's key was taken as pressed")
	}
}

func TestAHungTmuxDoesntCountItsPaneAsClosed(t *testing.T) {
	hangingTerminal(t)
	if !(commandTab{kind: "tmux", id: "%1", socket: "/tmp/tmux-sock"}).alive() {
		t.Fatal("a pane was taken as closed because tmux didn't answer in time")
	}
}
