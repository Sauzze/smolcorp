//go:build !windows

package guard

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// G2: a named pipe in the guard file's place, with no writer and with one that
// never writes, used to stall the guard at its open or its read. It is refused
// at once.
func TestAPipeGuardFileIsRefusedAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	refusedEverywhere(t, "pipe, no writer", path, "isn't a plain file")
	writer, err := os.OpenFile(path, os.O_RDWR, 0) // read-write: opens without waiting for a reader
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	refusedEverywhere(t, "pipe, a writer that never writes", path, "isn't a plain file")
}

// G3: a link to /dev/zero in the guard file's place used to fill the guard's
// memory (about 16 GB in 4 s). The link is refused, and so is the device named
// directly (as --rules).
func TestDevZeroAsTheGuardFileIsRefusedAtOnce(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skipf("no /dev/zero: %v", err)
	}
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.Symlink("/dev/zero", path); err != nil {
		t.Fatal(err)
	}
	refusedEverywhere(t, "a link to /dev/zero", path, "link")
	refusedEverywhere(t, "/dev/zero", "/dev/zero", "isn't a plain file")
}
