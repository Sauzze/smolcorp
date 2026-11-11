//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Not built on Windows, which has no FIFO to stand in for a folder that has gone away (a stale share).
// An account whose settings.json can't be opened without waiting must not stop the deck: the setup state looks in
// the account folders without holding d.mu.
func TestTheSetupStateDoesNotHoldTheDeckWhileItLooksInAnAccount(t *testing.T) {
	d := newTestDeck(t)
	work := claudeFolder(t, filepath.Join(homeDir, ".claude-work"))
	fifo := filepath.Join(work, "settings.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no FIFO here:", err)
	}
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	d.mu.Unlock()

	// A write end held open from before the setup state starts: its open of the FIFO goes through at once,
	// whenever it gets there, and its read then waits for data. Closing the write end afterwards is an end of
	// file, so the setup state always comes back, however late it reached the FIFO.
	w, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&router{deck: d}).setupState()
	}()
	// Give it time to reach the FIFO and wait in its read: a slow start only makes this run's check weaker, never fail.
	time.Sleep(300 * time.Millisecond)
	defer func() {
		w.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the setup state never came back")
		}
	}()

	locked := make(chan struct{})
	go func() {
		d.mu.Lock()
		d.mu.Unlock()
		close(locked)
	}()
	select {
	case <-locked:
	case <-time.After(2 * time.Second):
		t.Error("d.mu is held while the setup state waits on an account's settings.json")
	}
}
