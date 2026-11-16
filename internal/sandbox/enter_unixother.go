//go:build unix && !linux

package sandbox

import "syscall"

// fsType is only asked on Linux, where the desk's support folder is a tmpfs of its own.
func (realSystem) fsType(string) string { return "" }

// The seccomp and Landlock rows are Linux's alone: no start elsewhere has them, and here they fail.
func (realSystem) ioctl(int, uint64) error { return syscall.ENOSYS }
func (realSystem) landlock() error         { return syscall.ENOSYS }
