package procmem

import "golang.org/x/sys/unix"

// NotDumpable makes this process non-dumpable (see the package's comment).
func NotDumpable() error { return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0) }

// DenyMemoryAccess is Windows' only: here, NotDumpable does it.
func DenyMemoryAccess() error { return nil }

// dumpable is whether this process is dumpable, as the system has it.
func dumpable() (bool, error) {
	value, err := unix.PrctlRetInt(unix.PR_GET_DUMPABLE, 0, 0, 0, 0)
	return value != 0, err
}
