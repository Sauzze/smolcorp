//go:build windows

package desktop

import (
	"errors"
	"os"
	"syscall"
)

// openWatched opens rel inside root for a look, but not a link: one swapped in
// doesn't lead out, and root refuses any part of rel that leaves it.
func openWatched(root *os.Root, rel string) (*os.File, error) {
	if info, err := root.Lstat(rel); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, errors.New("not a plain file")
	}
	return root.Open(rel)
}

// fileIdentity is what Windows knows of the open file: its volume and file
// index (what an inode is) and how many names it has.
func fileIdentity(f *os.File) (volume, index uint64, links uint32, ok bool) {
	var data syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &data) != nil {
		return 0, 0, 0, false
	}
	return uint64(data.VolumeSerialNumber), uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow), data.NumberOfLinks, true
}

// fileInode is the file's identity (the volume and file index folded together),
// never 0 (0 means "was not there").
func fileInode(f *os.File, _ os.FileInfo) uint64 {
	volume, index, _, ok := fileIdentity(f)
	if !ok {
		return 1
	}
	id := (index ^ volume<<48 ^ volume) * 0x100000001b3
	if id == 0 {
		return 1
	}
	return id
}

// fileLinks is how many names the file has (a hard link makes more than one).
func fileLinks(f *os.File, _ os.FileInfo) uint64 {
	if _, _, links, ok := fileIdentity(f); ok {
		return uint64(links)
	}
	return 1
}
