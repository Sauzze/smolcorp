//go:build !darwin && !linux

package sandbox

import (
	"io/fs"
	"time"
)

// changeTime is when the entry was last written: this system's file info has no ctime.
func changeTime(info fs.FileInfo) time.Time { return info.ModTime() }

// birthTime is zero: a .git folder is the same one while it is the same file (os.SameFile).
func birthTime(fs.FileInfo) time.Time { return time.Time{} }
