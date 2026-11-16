//go:build !unix

package sandbox

import "io/fs"

// fileOwner is -1 where files have no uid: CheckBwrap fails there, as it must.
func fileOwner(fs.FileInfo) int64 { return -1 }
