package desktop

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// projectsPlace is the GM's projects folder, held open while a new project is made in it (see the Mac's). No desk is
// sandboxed on Linux, so a desk can already write wherever the owner can; the folder is held the same way, so the
// route works alike everywhere.
type projectsPlace struct{ fd int }

// openProjectsPlace opens the projects folder, already resolved, refusing a link at its last name.
func openProjectsPlace(path string) (*projectsPlace, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return &projectsPlace{fd}, nil
}

func (p *projectsPlace) close() { _ = unix.Close(p.fd) }

// lacks is nil when nothing is at name in the folder (a link counts), fs.ErrExist when something is.
func (p *projectsPlace) lacks(name string) error {
	var st unix.Stat_t
	switch err := unix.Fstatat(p.fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); {
	case err == nil:
		return fs.ErrExist
	case errors.Is(err, unix.ENOENT):
		return nil
	default:
		return err
	}
}

// makeFolder makes the empty folder name in the folder: mkdirat fails on anything at the name, a link included.
func (p *projectsPlace) makeFolder(name string) error { return unix.Mkdirat(p.fd, name, 0o755) }

// moveIn moves from to name in the folder with renameat2's RENAME_NOREPLACE, which fails on anything at the name. A
// disk that doesn't know that flag gets a plain renameat after a look that finds nothing there: the race left between
// the two gains a desk nothing it couldn't do itself here, with no sandbox.
func (p *projectsPlace) moveIn(from, name string) error {
	err := unix.Renameat2(unix.AT_FDCWD, from, p.fd, name, unix.RENAME_NOREPLACE)
	if !errors.Is(err, unix.EINVAL) && !errors.Is(err, unix.ENOSYS) {
		return err
	}
	if err := p.lacks(name); err != nil {
		return err
	}
	return unix.Renameat(unix.AT_FDCWD, from, p.fd, name)
}

// onDiskOf is whether dir is on the folder's disk (their st_dev), so a move from it can't fail for two disks.
func (p *projectsPlace) onDiskOf(dir string) (bool, error) {
	var held, other unix.Stat_t
	if err := unix.Fstat(p.fd, &held); err != nil {
		return false, err
	}
	if err := unix.Stat(dir, &other); err != nil {
		return false, &os.PathError{Op: "stat", Path: dir, Err: err}
	}
	return held.Dev == other.Dev, nil
}

// movedAcrossDisks is whether moveIn failed because the staging folder and the projects folder are on two disks.
func movedAcrossDisks(err error) bool { return errors.Is(err, unix.EXDEV) }

// openResolvedRoot opens folder as a root (see the Mac's): no desk starts in a sandbox here yet, so its path is
// followed as before.
func openResolvedRoot(folder string) (*os.Root, error) { return os.OpenRoot(folder) }
