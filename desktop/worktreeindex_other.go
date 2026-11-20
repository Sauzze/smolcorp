//go:build !darwin

package desktop

import "os"

// worktreeCheckedOutInCore: off a Mac, git checks a hire's worktree out where it is (worktreeindex_darwin.go).
const worktreeCheckedOutInCore = false

// carriedStatKeys: nothing is carried off a Mac, so no index is given carried files' keys.
func carriedStatKeys(os.FileInfo) ([6]uint32, bool) { return [6]uint32{}, false }
