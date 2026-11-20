//go:build !windows

package guard

import (
	"errors"
	"os"
	"syscall"
)

// openGuardFile opens path for ReadFile: O_NOFOLLOW refuses a link as the last
// part (a link to /dev/zero included), O_NONBLOCK keeps a pipe from holding the open.
func openGuardFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, errFileLink
	}
	return f, err
}

// guardFileLinks is how many names the open file has, and whether that is known.
func guardFileLinks(_ *os.File, info os.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Nlink), true
}
