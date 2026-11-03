//go:build windows

package desktop

import "os"

// openNoBlock opens a file to read. Windows has no pipes in a folder to be swapped in.
func openNoBlock(path string) (*os.File, error) { return os.Open(path) }
