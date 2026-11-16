//go:build unix

package sandbox

import (
	"io/fs"
	"os"
	"syscall"
)

// fileOwner is the uid that owns fi's file.
func fileOwner(fi fs.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(st.Uid)
	}
	return -1
}

// fileGroup is the gid of fi's file's group.
func fileGroup(fi fs.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(st.Gid)
	}
	return -1
}

// openKept opens the file at to read, never through a link and never waiting on a pipe.
func openKept(at string) (*os.File, error) {
	return os.OpenFile(at, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// openKeptWrite opens the file at to write, with flag added, never through a link.
func openKeptWrite(at string, flag int) (*os.File, error) {
	return os.OpenFile(at, os.O_WRONLY|flag|syscall.O_NOFOLLOW, 0o600)
}

// fileIno is fi's file's inode number.
func fileIno(fi fs.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}
