//go:build darwin

package desktop

import (
	"os"
	"strconv"
	"syscall"
)

// folderIdentity is folder's identity on its volume, its device and inode ("<dev>:<ino>"), not following a link at
// folder itself: what a hire's row keeps beside its folder resolved (Hire.FolderID), so a real folder renamed into the
// place since its hire is told from the one hired into, which the path alone can't (FXRC3FIX, RT126RC3's F1). "" when
// it can't be read, or isn't a folder.
func folderIdentity(folder string) string {
	info, err := os.Lstat(folder)
	if err != nil || !info.IsDir() {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatInt(int64(st.Dev), 10) + ":" + strconv.FormatUint(st.Ino, 10)
}
