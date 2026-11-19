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
		return time.Unix(st.Ctimespec.Unix())
	}
	return info.ModTime()
}

// birthTime is when the entry was made.
func birthTime(info fs.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Birthtimespec.Unix())
	}
	return time.Time{}
}
