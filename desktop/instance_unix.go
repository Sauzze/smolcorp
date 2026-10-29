//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"syscall"
)

// instanceFile is the lock file, next to the keys.
var instanceFile = func() string { return filepath.Join(supportDir, "instance.lock") }

// held keeps the file open, and so locked, for as long as the process lives
// (the system drops the lock when the process dies, however it dies).
var held *os.File

// acquireInstance takes an exclusive lock on the file. running is true when
// another Smolcorp holds it.
func acquireInstance() (running bool, err error) {
	file, err := os.OpenFile(instanceFile(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			return true, nil
		}
		return false, err
	}
	held = file
	return false, nil
}

// notifyUser: on these systems the message was printed to the terminal already.
func notifyUser(string) {}

// instanceName exists for the tests, which name the Windows mutex; here the lock is a file.
var instanceName = ""
