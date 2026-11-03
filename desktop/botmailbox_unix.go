//go:build !windows

package desktop

import (
	"os"
	"syscall"
)

// openNoBlock opens a file to read without waiting: a file swapped for a pipe with nobody writing
// to it would otherwise hold the look for good. A regular file reads the same either way.
func openNoBlock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
