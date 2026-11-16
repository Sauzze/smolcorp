//go:build unix

package sandbox

import (
	"io/fs"
	"syscall"
)

// fileOwner is the uid that owns fi's file.
func fileOwner(fi fs.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(st.Uid)
	}
	return -1
}
