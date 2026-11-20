package sandbox

import "golang.org/x/sys/unix"

// swapNames swaps the names a and b in one step, as a desk can: renameat2 with RENAME_EXCHANGE.
func swapNames(a, b string) error {
	return unix.Renameat2(unix.AT_FDCWD, a, unix.AT_FDCWD, b, unix.RENAME_EXCHANGE)
}
