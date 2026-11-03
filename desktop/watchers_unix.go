//go:build !windows

package desktop

import (
	"os"
	"syscall"
)

// openWatched opens rel inside root for a look. root refuses any part of rel
// that leaves it, O_NONBLOCK so a pipe put in its place can't hold the look,
// O_NOFOLLOW so a link as the last part isn't followed; the caller reads only
// plain files (Watchers.look).
func openWatched(root *os.Root, rel string) (*os.File, error) {
	return root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}

// fileInode is the file's inode, never 0 (0 means "was not there").
func fileInode(_ *os.File, info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Ino != 0 {
		return uint64(st.Ino)
	}
	return 1
}

// fileLinks is how many names the file has (a hard link makes more than one).
func fileLinks(_ *os.File, info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}
