//go:build !windows

package desktop

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// openWatched opens rel inside root for a look. root refuses any part of rel
// that leaves it, O_NONBLOCK so a pipe put in its place can't hold the look,
// O_NOFOLLOW so a link as the last part isn't followed; the caller reads only
// plain files (Watchers.look).
func openWatched(root *os.Root, rel string) (*os.File, error) {
	return root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}

// openClaudeLook opens the folder at path, the top of a plant walk's look, by its path as given (a link to it is
// followed, as os.ReadDir did), only as a folder and without waiting on a pipe.
func openClaudeLook(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
}

// openClaudeIn opens the entry name in the open folder dir (or at the path name, dir nil), looked at as looked, for
// the plant walk, with openWatched's flags: through no link (O_NOFOLLOW), without waiting on a pipe (O_NONBLOCK), a
// folder only as a folder (O_DIRECTORY), and only while it's still what was looked at (the same kind, os.SameFile).
// In dir it's opened through dir itself (openat): a folder above it swapped since changes nothing, and os.OpenRoot,
// which opens its folder plainly, isn't needed.
func openClaudeIn(dir *os.File, name string, looked fs.FileInfo) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NONBLOCK | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if looked.IsDir() {
		flags |= unix.O_DIRECTORY
	}
	f, err := openatIn(dir, name, flags)
	if err != nil {
		return nil, err
	}
	if now, err := f.Stat(); err != nil || now.Mode().Type() != looked.Mode().Type() || !os.SameFile(now, looked) {
		f.Close()
		return nil, fmt.Errorf("%s changed while it was looked at", f.Name())
	}
	return f, nil
}

// openClaudeFolderIn opens the folder e, listed by kind only in the open folder dir, for the place walk: through dir
// itself (openat), only as a folder and through no link (O_DIRECTORY|O_NOFOLLOW), without waiting on a pipe. One
// swapped since for a link, a file or a pipe isn't opened; one swapped for another folder is what's there now, and
// is walked as such (FXCLAUDESUMOPENR3 A).
func openClaudeFolderIn(dir *os.File, e fs.DirEntry) (*os.File, error) {
	return openatIn(dir, e.Name(), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC)
}

// openatIn opens name in the open folder dir (or at the path name, dir nil) with flags, through dir (openat), and
// takes O_NONBLOCK off what it opened: a plain file or a folder has nothing to wait on.
func openatIn(dir *os.File, name string, flags int) (*os.File, error) {
	path, fd, err := name, -1, error(nil)
	open := func(at int) {
		for fd, err = unix.Openat(at, name, flags, 0); err == unix.EINTR; fd, err = unix.Openat(at, name, flags, 0) {
		}
	}
	if dir == nil {
		open(unix.AT_FDCWD)
	} else {
		path = filepath.Join(dir.Name(), name)
		raw, rawErr := dir.SyscallConn()
		if rawErr == nil {
			rawErr = raw.Control(func(at uintptr) { open(int(at)) })
		}
		if rawErr != nil {
			return nil, rawErr
		}
	}
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	if err := unix.SetNonblock(fd, false); err != nil {
		unix.Close(fd)
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// readlinkIn is where the link name in the open folder dir (or at the path name, dir nil) leads, read through dir.
func readlinkIn(dir *os.File, name string) (string, error) {
	if dir == nil {
		return os.Readlink(name)
	}
	raw, err := dir.SyscallConn()
	if err != nil {
		return "", err
	}
	var target string
	var linkErr error
	err = raw.Control(func(at uintptr) {
		for size := 256; ; size *= 2 {
			buf := make([]byte, size)
			n, err := unix.Readlinkat(int(at), name, buf)
			if err == unix.EINTR {
				continue
			} else if err != nil {
				linkErr = &fs.PathError{Op: "readlinkat", Path: filepath.Join(dir.Name(), name), Err: err}
				return
			} else if n < size {
				target = string(buf[:n])
				return
			}
		}
	})
	if err == nil {
		err = linkErr
	}
	return target, err
}

// fileInode is the file's inode, never 0 (0 means "was not there").
func fileInode(_ *os.File, info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Ino != 0 {
		return uint64(st.Ino)
	}
	return 1
}

// fileLinks is how many names the file has (a hard link makes more than one).
func fileLinks(_ *os.File, info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}

// claudeLinkSpelled is where a link leads, resolved by the plant walk w (claudeWalk.resolve): as it is, here.
func claudeLinkSpelled(_ *claudeWalk, to string) (string, error) {
	return to, nil
}
