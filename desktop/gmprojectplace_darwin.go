package desktop

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// projectsPlace is the GM's projects folder, held open while a new project is made in it (FXMAKEPROJECTR1 B2): the
// new folder is made, or the clone moved in, by its name in the folder held, never through the folder's path again.
// On a Mac a hire may write the projects folder's top, so a path read twice could meet a link put there in between.
type projectsPlace struct{ fd int }

// openProjectsPlace opens the projects folder, already resolved, with no link anywhere on its path (O_NOFOLLOW_ANY):
// one put in its way since it was resolved is refused, never followed.
func openProjectsPlace(path string) (*projectsPlace, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW_ANY|unix.O_CLOEXEC, 0)
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

// moveIn moves from (the clone, in the core's staging folder) to name in the folder in one step that fails on
// anything at the name, a link included, and never follows one there: renameatx_np with RENAME_EXCL.
func (p *projectsPlace) moveIn(from, name string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, from, p.fd, name, unix.RENAME_EXCL)
}

// movedAcrossDisks is whether moveIn failed because the staging folder and the projects folder are on two disks.
func movedAcrossDisks(err error) bool { return errors.Is(err, unix.EXDEV) }
