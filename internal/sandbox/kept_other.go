//go:build !unix

package sandbox

import (
	"errors"
	"os"
)

// keptNoWait is nothing where there are no pipes to wait on.
const keptNoWait = 0

// renameKept can't rename from one held folder to another here: a set aside is copied, then removed, each by its
// folders as held. Only a Linux desk keeps places (L1GUARD1).
func renameKept(*os.Root, string, *os.Root, string) error { return errors.ErrUnsupported }

// linkCount is one here: CheckRefPlaces's hard links are a unix desk's.
func linkCount(os.FileInfo) uint64 { return 1 }

// userWrites says yes here: CheckRefPlaces follows links only for a Mac's Policy.
func userWrites(string) bool { return true }
