//go:build unix

package sandbox

import (
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// keptNoWait opens a kept file without waiting on a pipe, and without a device it may be swapped for meanwhile ever
// becoming the core's terminal (the file is then refused, by its fd's device and inode).
const keptNoWait = syscall.O_NONBLOCK | syscall.O_NOCTTY

// renameKept renames name in the folder from to toName in the folder to: one name in each, by the folders' own fds,
// so neither folder is found again by its path and neither name is followed (FXKEPTFD).
func renameKept(from *os.Root, name string, to *os.Root, toName string) error {
	if strings.Contains(name, "/") || strings.Contains(toName, "/") {
		return fmt.Errorf("%q or %q is more than one name", name, toName)
	}
	a, err := from.Open(".")
	if err != nil {
		return err
	}
	defer a.Close()
	b, err := to.Open(".")
	if err != nil {
		return err
	}
	defer b.Close()
	if err := unix.Renameat(int(a.Fd()), name, int(b.Fd()), toName); err != nil {
		return &os.LinkError{Op: "renameat", Old: path.Join(from.Name(), name), New: path.Join(to.Name(), toName), Err: err}
	}
	return nil
}
