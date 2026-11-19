package sandbox

import (
	"io/fs"
	"syscall"
	"time"
)

// changeTime is when the entry itself last changed (its ctime): a write, a chmod, a link or a rename in a folder all
// move it, and no program can set it back.
func changeTime(info fs.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Ctim.Unix())
	}
	return info.ModTime()
}

// birthTime is when the entry was made: Linux's stat doesn't say, so it is zero, and a .git folder is the same one
// while it is the same file (os.SameFile).
func birthTime(fs.FileInfo) time.Time { return time.Time{} }
