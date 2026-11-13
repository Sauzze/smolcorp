//go:build !windows

package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// askToEnd sends the process SIGTERM.
func askToEnd(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }

// thisHomeRunning is whether a running copy holds this HOME's instance lock
// (a shared lock can't be had while the copy holds its exclusive one). It
// never creates the file and never keeps the lock. A variable, for the tests.
var thisHomeRunning = func() (bool, error) {
	file, err := os.Open(instanceFile())
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	switch err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err {
	case nil:
		return false, nil // the lock was free; it is released with the file
	case syscall.EWOULDBLOCK:
		return true, nil
	default:
		return false, err
	}
}

// isSmolcorpProcess is whether the process runs a program called smolcorp,
// where /proc shows it (Linux), so a pid some other program has taken since
// is never signalled. The link says " (deleted)" for a program an update
// replaced, which is the very copy to stop. A Smolcorp keeps its memory from other processes on Linux
// (internal/procmem), and the link with it: then its name, as the system keeps it, says it. A variable, for the tests.
var isSmolcorpProcess = func(pid int) bool {
	program, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if errors.Is(err, fs.ErrPermission) {
		name, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
		return err == nil && strings.HasPrefix(strings.TrimSpace(string(name)), "smolcorp")
	}
	if err != nil {
		return false
	}
	return strings.HasPrefix(filepath.Base(strings.TrimSuffix(program, " (deleted)")), "smolcorp")
}
