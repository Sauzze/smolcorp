package sandbox

import "golang.org/x/sys/unix"

// swapNames swaps the names a and b in one step, as a desk could on a Mac: renamex_np with RENAME_SWAP.
func swapNames(a, b string) error {
	return unix.RenamexNp(a, b, unix.RENAME_SWAP)
}
