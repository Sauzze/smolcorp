//go:build !windows

package desktop

import "syscall"

// canWriteIn says whether this user may make files in folder. Asking access()
// makes nothing, so it's the same answer whether the folder is being kept or not.
func canWriteIn(folder string, _ bool) bool { return syscall.Access(folder, 2) == nil } // 2 is W_OK
