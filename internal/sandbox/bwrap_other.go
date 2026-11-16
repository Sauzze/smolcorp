//go:build !unix

package sandbox

import (
	"io/fs"
	"os"
)

// fileOwner is -1 where files have no uid: CheckBwrap fails there, as it must.
func fileOwner(fs.FileInfo) int64 { return -1 }

// fileGroup is -1 there too.
func fileGroup(fs.FileInfo) int64 { return -1 }

// openKept opens the file at to read.
func openKept(at string) (*os.File, error) { return os.Open(at) }

// openKeptWrite opens the file at to write, with flag added.
func openKeptWrite(at string, flag int) (*os.File, error) {
	return os.OpenFile(at, os.O_WRONLY|flag, 0o600)
}
