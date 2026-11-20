package desktop

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"slices"
)

// A hire's worktree checked out in a folder of the core's own, then carried into its tree (FXGMHIRENOGIT126R2, B1:
// carryWorktree). The carried files keep their times and sizes, but not their inode or change time, which git's index
// keeps too: git would take every file as changed until it had read each again, and `git diff-files` would list them
// all. So the index git wrote for the files in the core's folder is given the carried files' own stat keys, entry by
// entry, before it is carried: git finds them as it left them.

// refreshCarriedIndex is index, a worktree's index as git wrote it in the core's folder, with each entry's stat keys
// (change time, device, inode, owner, group) those of the file the core carried for it, as seen holds it (by its path
// in the tree, as the index names it: carryWorkTreeSeen). Only a plain file's or a link's entry, with no flag beyond
// its name's length and stage, that matches the carried one (refreshCarriedEntry) is changed; anything else is left
// as git wrote it, and git reads that file again, as it would anyway. An index this doesn't know (a version but 2 or
// 3, an entry that runs past its end) is given back as it was. hashSize is the repo's object name's size in bytes; the index's own checksum
// at its end is made again, unless git wrote none (index.skipHash).
func refreshCarriedIndex(index []byte, seen map[string]os.FileInfo, hashSize int) []byte {
	if hashSize == 0 || len(index) < 12+hashSize || string(index[:4]) != "DIRC" {
		return index
	}
	version := binary.BigEndian.Uint32(index[4:8])
	if version != 2 && version != 3 {
		return index
	}
	out := slices.Clone(index)
	body := len(out) - hashSize
	at := 12
	for range binary.BigEndian.Uint32(out[8:12]) {
		fixed := 40 + hashSize + 2 // the stat keys, the object name, the flags
		if at+fixed > body {
			return index
		}
		flags := binary.BigEndian.Uint16(out[at+40+hashSize:])
		start := at + fixed
		if flags&0x4000 != 0 { // extended flags (version 3)
			start += 2
		}
		if start > body {
			return index
		}
		end := bytes.IndexByte(out[start:body], 0)
		if end < 0 {
			return index
		}
		next := at + (start-at+end+8)&^7 // padded with NULs to a multiple of 8
		if next > body {
			return index
		}
		if flags&0xc000 == 0 {
			refreshCarriedEntry(out[at:at+40], seen[string(out[start:start+end])])
		}
		at = next
	}
	sum := out[body:]
	if bytes.Count(sum, []byte{0}) != len(sum) {
		switch hashSize {
		case sha1.Size:
			made := sha1.Sum(out[:body])
			copy(sum, made[:])
		case sha256.Size:
			made := sha256.Sum256(out[:body])
			copy(sum, made[:])
		default:
			return index
		}
	}
	return out
}

// refreshCarriedEntry gives entry's stat keys (an index entry's first 40 bytes) those of carried, the file the core
// carried for it: a plain file's entry whose time and size are carried's, or a link's whose size (its target's
// length) is carried's, the link the core wrote (its time is the carry's: a link's can't be set through a root). Else
// it leaves it as it is.
func refreshCarriedEntry(entry []byte, carried os.FileInfo) {
	keys, ok := carriedStatKeys(carried)
	word := func(at int) uint32 { return binary.BigEndian.Uint32(entry[at:]) }
	if !ok || word(36) != uint32(carried.Size()) {
		return
	}
	modified := carried.ModTime()
	switch {
	case carried.Mode().IsRegular() && word(24)&0o170000 == 0o100000:
		if word(8) != uint32(modified.Unix()) || word(12) != 0 && word(12) != uint32(modified.Nanosecond()) {
			return
		}
	case carried.Mode()&os.ModeSymlink != 0 && word(24)&0o170000 == 0o120000:
		binary.BigEndian.PutUint32(entry[8:], uint32(modified.Unix()))
		binary.BigEndian.PutUint32(entry[12:], uint32(modified.Nanosecond()))
	default:
		return
	}
	for i, at := range []int{0, 4, 16, 20, 28, 32} { // change time (s, ns), device, inode, owner, group
		binary.BigEndian.PutUint32(entry[at:], keys[i])
	}
}

// gitHashSize is the size in bytes of an object name in a repo of format (sha1, sha256); 0 for a format not known.
func gitHashSize(format string) int {
	switch format {
	case "sha1":
		return sha1.Size
	case "sha256":
		return sha256.Size
	}
	return 0
}
