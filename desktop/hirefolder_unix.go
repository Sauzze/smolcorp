//go:build !windows

package desktop

import "syscall"

// canWriteIn says whether this user may make files in folder.
func canWriteIn(folder string) bool { return syscall.Access(folder, 2) == nil } // 2 is W_OK
