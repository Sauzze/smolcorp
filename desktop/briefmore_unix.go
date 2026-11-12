//go:build !windows

package desktop

import "syscall"

// noFollow makes opening a link fail, so a file checked a moment ago can't be swapped for one.
const noFollow = syscall.O_NOFOLLOW
