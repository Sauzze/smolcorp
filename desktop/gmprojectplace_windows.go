package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// projectsPlace is the GM's projects folder while a new project is made in it (see the Mac's). Windows has no folder
// handle to make a folder by name in, and no desk is sandboxed here, so a desk can already write wherever the owner
// can: the folder is named by its path, and the move in still never replaces anything at the name.
type projectsPlace struct{ path string }

func openProjectsPlace(path string) (*projectsPlace, error) { return &projectsPlace{path}, nil }

func (p *projectsPlace) close() {}

// lacks is nil when nothing is at name in the folder (a link or a junction counts), fs.ErrExist when something is.
func (p *projectsPlace) lacks(name string) error {
	_, err := os.Lstat(filepath.Join(p.path, name))
	switch {
	case err == nil:
		return fs.ErrExist
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return err
	}
}

func (p *projectsPlace) makeFolder(name string) error {
	return os.Mkdir(filepath.Join(p.path, name), 0o755)
}

// moveIn moves from to name in the folder with MoveFileEx and no MOVEFILE_REPLACE_EXISTING: it fails on anything at
// the name, a link or a junction included.
func (p *projectsPlace) moveIn(from, name string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(filepath.Join(p.path, name))
	if err != nil {
		return err
	}
	return windows.MoveFileEx(src, dst, 0)
}

// onDiskOf is true here: Windows has no st_dev to compare, and moveIn's ERROR_NOT_SAME_DEVICE still refuses two
// disks, after the clone.
func (p *projectsPlace) onDiskOf(string) (bool, error) { return true, nil }

// movedAcrossDisks is whether moveIn failed because the staging folder and the projects folder are on two disks.
func movedAcrossDisks(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
