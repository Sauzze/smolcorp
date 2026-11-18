//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// openClaudeLook opens the folder at path, the top of a plant walk's look, by its path as given.
func openClaudeLook(path string) (*os.File, error) {
	return os.Open(path)
}

// openClaudeIn opens the entry name in the open folder dir (or at the path name, dir nil), looked at as looked, for
// the plant walk: a plain file or a folder only, and only while it's still what was looked at (the same kind,
// os.SameFile), so a link or junction put in its place, which the open follows, can't be summed. Windows has no
// openat here: it's opened by its path. A folder the walk is in stays open while it looks below it, and Go opens
// without FILE_SHARE_DELETE, so Windows refuses to rename or remove it meanwhile: one above can't be swapped.
func openClaudeIn(dir *os.File, name string, looked fs.FileInfo) (*os.File, error) {
	path := name
	if dir != nil {
		path = filepath.Join(dir.Name(), name)
	}
	if kind := looked.Mode().Type(); kind != 0 && kind != fs.ModeDir {
		return nil, errors.New(path + " is neither a plain file nor a folder")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if now, err := f.Stat(); err != nil || now.Mode().Type() != looked.Mode().Type() || !os.SameFile(now, looked) {
		f.Close()
		return nil, fmt.Errorf("%s changed while it was looked at", path)
	}
	return f, nil
}

// openClaudeFolderIn opens the folder e, listed in the open folder dir, for the place walk, as openClaudeIn does: the
// listing gives what it is (FindNextFile), so it's no more costly here.
func openClaudeFolderIn(dir *os.File, e fs.DirEntry) (*os.File, error) {
	looked, err := e.Info()
	if err != nil {
		return nil, err
	}
	return openClaudeIn(dir, e.Name(), looked)
}

// readlinkIn is where the link name in the open folder dir (or at the path name, dir nil) leads.
func readlinkIn(dir *os.File, name string) (string, error) {
	if dir != nil {
		name = filepath.Join(dir.Name(), name)
	}
	return os.Readlink(name)
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

// claudeLinkSpelled is where a link leads, resolved by the plant walk w (claudeWalk.resolve), spelled as Windows
// has it (each name's case), as filepath.EvalSymlinks gives it: to has no link left in it, so that's one lookup a
// name, spent from w like the walk's own.
func claudeLinkSpelled(w *claudeWalk, to string) (string, error) {
	if !w.spend(1+strings.Count(to, string(os.PathSeparator)), 0, 0) {
		return "", errClaudeTooMuch
	}
	return filepath.EvalSymlinks(to)
}
