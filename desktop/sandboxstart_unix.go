//go:build !windows

package desktop

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByAnother is whether the folder info describes belongs to another user: the link check passes over one it
// can't list, since the owner's user can't make a link inside it either (root's cron folders).
func ownedByAnother(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) != os.Getuid()
}
