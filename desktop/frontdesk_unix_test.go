//go:build unix

package desktop

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAStatusFifoDoesNotHangTheTeamLine(t *testing.T) {
	folder := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(folder, "STATUS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, ".team"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, ".team", "STATUS.md"), []byte("one\nreal line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() { done <- statusLine(folder) }()
	select {
	case got := <-done:
		if got != "real line" {
			t.Fatalf("status line %q, want the plain file's", got)
		}
	case <-time.After(5 * time.Second):
		// Free the blocked open so the test leaves nothing behind.
		if f, err := os.OpenFile(filepath.Join(folder, "STATUS.md"), os.O_WRONLY, 0); err == nil {
			f.Close()
		}
		t.Fatal("statusLine hung on a FIFO")
	}
}
