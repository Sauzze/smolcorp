//go:build unix && !linux

package sandbox

import "syscall"

// fsType is only asked on Linux, where the desk's support folder is a tmpfs of its own.
func (realSystem) fsType(string) string { return "" }

// The seccomp and Landlock rows are Linux's alone: no start elsewhere has them, and here they fail.
func (realSystem) ioctl(int, uint64) error { return syscall.ENOSYS }
func (realSystem) landlock() error         { return syscall.ENOSYS }

// forward: the forwarder is Linux's alone, where a desk has a network of its own (forward_linux.go).
func (realSystem) forward(string, string) (string, error) { return "", syscall.ENOSYS }
