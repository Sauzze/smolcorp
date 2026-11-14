package procmem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// SK3's Linux row: another process of the same user reads a stand-in's environment and finds its program while
// it's dumpable, and can do neither once it isn't. Its name (comm) stays readable: `smolcorp quit` goes by it then
// (desktop/quit_unix.go). Root reads everything regardless, so as root only the protected side's flag is checked.
func TestANonDumpableProcessKeepsItsEnvironmentFromTheUsersOtherProcesses(t *testing.T) {
	asRoot := os.Geteuid() == 0
	for _, protected := range []bool{false, true} {
		pid := strconv.Itoa(startStandIn(t, protected).Process.Pid)
		environ, err := os.ReadFile(filepath.Join("/proc", pid, "environ"))
		_, linkErr := os.Readlink(filepath.Join("/proc", pid, "exe"))
		switch {
		case !protected && (err != nil || !strings.Contains(string(environ), fakeSecret)):
			t.Errorf("dumpable: its environment should be readable (the row means nothing otherwise): %v", err)
		case protected && !asRoot && (!errors.Is(err, fs.ErrPermission) || strings.Contains(string(environ), fakeSecret)):
			t.Errorf("non-dumpable: its environment was read, or failed otherwise: %v", err)
		case protected && !asRoot && !errors.Is(linkErr, fs.ErrPermission):
			t.Errorf("non-dumpable: its program's link was read, or failed otherwise: %v", linkErr)
		}
		if name, err := os.ReadFile(filepath.Join("/proc", pid, "comm")); err != nil || strings.TrimSpace(string(name)) == "" {
			t.Errorf("protected %v: its name should stay readable: %q, %v", protected, name, err)
		}
	}
}

// The flag itself, in this process: set, and read back as the system has it.
func TestNotDumpableClearsTheFlag(t *testing.T) {
	if !strings.HasSuffix(os.Args[0], ".test") {
		t.Skip("only in a test binary of its own")
	}
	if on, err := dumpable(); err != nil || !on {
		t.Skipf("this test process isn't dumpable to start with: %v, %v", on, err)
	}
	if err := NotDumpable(); err != nil {
		t.Fatal(err)
	}
	if on, err := dumpable(); err != nil || on {
		t.Errorf("still dumpable: %v, %v", on, err)
	}
}
