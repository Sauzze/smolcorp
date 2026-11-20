//go:build linux

package sandbox

import (
	"errors"
	"syscall"
)

// readUnixDiag asks the kernel for each unix socket's name and file (NETLINK_SOCK_DIAG, UDIAG_SHOW_NAME and
// UDIAG_SHOW_VFS: mask.go) and gives its dump's messages as they come, up to its end.
func readUnixDiag() ([]byte, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, sockDiag)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)
	if err := syscall.Sendto(fd, unixDiagRequest(), 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil, err
	}
	var dump []byte
	buf := make([]byte, 1<<16)
	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		switch {
		case err == syscall.EINTR:
			continue
		case err != nil:
			return nil, err
		case n == 0:
			return nil, errors.New("the dump stopped before its end")
		}
		dump = append(dump, buf[:n]...)
		if done, err := diagEnded(buf[:n]); err != nil || done {
			return dump, err
		}
	}
}
