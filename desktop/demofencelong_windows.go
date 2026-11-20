package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// longPathHere is a whole path with its short (8.3) names made long by this computer's disk: GetLongPathName on
// the longest part of it that's there, the rest kept as written (a name that isn't there has no short name yet). It
// fails when the disk doesn't answer for a part that's there, or can't say whether a part in a short name's shape
// is there (fail-closed: the fence reads the path as untold).
func longPathHere(name string) (string, error) {
	name = filepath.Clean(name)
	rest := ""
	for dir := name; ; dir = filepath.Dir(dir) {
		_, err := os.Lstat(dir)
		if err == nil {
			long, err := getLongPathName(dir)
			if err != nil {
				return "", err
			}
			return filepath.Join(long, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) && shortShape.MatchString(filepath.Base(dir)) {
			return "", err
		}
		if filepath.Dir(dir) == dir {
			return name, nil
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// getLongPathName is Windows' GetLongPathName of a path that's there.
func getLongPathName(name string) (string, error) {
	short, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH)
	for range 3 {
		n, err := windows.GetLongPathName(short, &buf[0], uint32(len(buf)))
		switch {
		case err != nil:
			return "", err
		case n == 0:
			return "", fmt.Errorf("no long name for %s", name)
		case int(n) < len(buf):
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n) // too short: n is the size it needs
	}
	return "", fmt.Errorf("no long name for %s", name)
}
