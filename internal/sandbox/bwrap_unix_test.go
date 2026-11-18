//go:build unix

package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// folderRow does the view stand-in's rows on a folder the start may hold (bwrap_linux_test.go): "renameover " makes a
// new folder beside it and renames it over it, "rmdir " removes it. Both ask the kernel itself, which answers EBUSY on
// a mount point. Go's os.Rename doesn't: on unix it looks at the target first and gives EEXIST for any folder, held or
// not, without calling rename(2) (RMSYSAL1 B1). Nothing can be linked at the folder's name without removing it first,
// and a link renamed over a folder is EISDIR before any mount is looked at, so "rmdir " is the row for "link over".
// ok is false for any other row.
func folderRow(name, at string) (ok bool, err error) {
	switch {
	case strings.HasPrefix(name, "renameover "):
		if err = os.Mkdir(at+".new", 0o700); err == nil {
			err = unix.Rename(at+".new", at)
		}
		return true, err
	case strings.HasPrefix(name, "rmdir "):
		return true, unix.Rmdir(at)
	}
	return false, nil
}

// RMSYSAL1 B1's control: on plain empty folders that nothing holds, both rows go through. So the view's EBUSY for them
// can only come from the start's --ro-bind on the placeholder, not from the row itself.
func TestTheFolderRowsGoThroughOnAPlainFolder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"renameover its registration's remotes", "rmdir its registration's branches"} {
		at := dir + "/" + strings.Fields(name)[0]
		if err := os.Mkdir(at, 0o700); err != nil {
			t.Fatal(err)
		}
		if ok, err := folderRow(name, at); !ok || err != nil {
			t.Errorf("%s, on a plain empty folder: %v (a row of the stand-in: %v), want it done", name, err, ok)
		}
	}
	if _, err := os.Stat(dir + "/renameover.new"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the new folder is still beside the old one: %v", err)
	}
	if _, err := os.Stat(dir + "/rmdir"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the removed folder is still there: %v", err)
	}
	if ok, _ := folderRow("rename its registration's branches", dir); ok {
		t.Error("a plain rename is taken as a folder row")
	}
}
