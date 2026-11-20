//go:build darwin

package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// makeCoreWorktrees makes the core's own folder for worktrees, and the folder above it, each its owner's alone (0700),
// when they aren't there; and checks that each is a real folder (no link) of this user's. On a Mac, before any desk's
// Policy is made (deskPlacesOf) and before a worktree is made there (prepareHire): a desk's Policy denies making,
// moving or writing either, so neither is ever made by a desk first. Elsewhere it makes nothing (coreworktrees_other.go).
func makeCoreWorktrees() error {
	top := filepath.Dir(coreWorktreesFolder())
	for _, folder := range []string{top, coreWorktreesFolder()} {
		if err := os.Mkdir(folder, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return errors.Join(err, errCoreWorktrees)
		}
		info, err := os.Lstat(folder)
		if err != nil || !info.IsDir() || !ownFolder(info) {
			return errCoreWorktrees
		}
		if info.Mode().Perm() != 0o700 {
			_ = os.Chmod(folder, 0o700)
		}
	}
	return nil
}

// ownFolder is whether info, a folder's Lstat, is this user's.
func ownFolder(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return !ok || int(st.Uid) == os.Getuid()
}
