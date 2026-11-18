//go:build !unix

package sandbox

import (
	"io"
	"io/fs"
)

// HasCanary: this computer has no sandbox for `enter` to check, so NewCanary makes nothing (ErrNoCanary).
const HasCanary = false

// fileID: no canary here.
func fileID(fs.FileInfo) (dev, ino uint64, ok bool) { return 0, 0, false }

// enter: this computer has no sandbox for `enter` to check (Windows' is SA-W3's).
func enter(_ []string, stderr io.Writer) int {
	return refuseHere(stderr, "no sandbox on this computer")
}
