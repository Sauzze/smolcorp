package sandbox

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// fsType is the file system path is on: "tmpfs" for the private view's support folder (SA-L1), else "".
func (realSystem) fsType(path string) string {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) == nil && st.Type == unix.TMPFS_MAGIC {
		return "tmpfs"
	}
	return ""
}

func (realSystem) ioctl(fd int, request uint64) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), 0); errno != 0 {
		return errno
	}
	return nil
}

// landlockScoped is struct landlock_ruleset_attr up to its scope (Landlock ABI 6): nothing of the files or the network
// handled, only the scope.
type landlockScoped struct{ accessFS, accessNet, scoped uint64 }

// landlock applies Landlock's abstract-socket scope to the calling thread: from it, and from every program it becomes,
// no abstract unix socket made outside the scope can be reached. no_new_privs, which Landlock needs, bwrap has set
// already; setting it again changes nothing.
func (realSystem) landlock() error {
	attr := landlockScoped{scoped: unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(fd))
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
