package desktop

import (
	"os"
	"syscall"
)

// worktreeCheckedOutInCore is whether a hire's new worktree is checked out in a folder of the core's own, then carried
// into its tree (FXGMHIRENOGIT126R2, B1): on a Mac, where a General Manager runs in its sandbox and may still write the
// folder for hires of a repo outside its own places. Elsewhere no desk runs in a sandbox yet, so a GM can write
// whatever the owner can, and git checks the tree out where it is, as before (the rule's D2).
const worktreeCheckedOutInCore = true

// carriedStatKeys are the stat keys git's index keeps for a file that aren't its time or size, as the system gives
// them for info: change time (s, ns), device, inode, owner and group, each cut to 32 bits as git cuts them.
func carriedStatKeys(info os.FileInfo) ([6]uint32, bool) {
	if info == nil {
		return [6]uint32{}, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return [6]uint32{}, false
	}
	return [6]uint32{uint32(st.Ctimespec.Sec), uint32(st.Ctimespec.Nsec), uint32(st.Dev), uint32(st.Ino), st.Uid, st.Gid}, true
}
