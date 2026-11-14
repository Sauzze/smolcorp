package guard

import (
	"errors"
	"io"
)

// fileMost is the most a guard file may hold. The default is about 3 KiB, and
// the page keeps at most 40 patterns of 200 characters, so 64 KiB is room to spare.
const fileMost = 64 << 10

var (
	errFileLink     = errors.New("it is a link, and the guard reads only the file itself")
	errFileNotPlain = errors.New("it isn't a plain file")
	errFileNames    = errors.New("it has more than one name (a hard link)")
	errFileTooLarge = errors.New("it is larger than 64 KiB")
)

// ReadFile is the guard file at path, read so a hire can't stall or flood
// whoever reads it (SG0): the guard at each command, and the app. It opens the
// name itself, never what a link points to, and without waiting (a pipe put in
// its place can't hold the open), then reads only a plain file with exactly one
// name and at most 64 KiB, through a LimitReader in case it grows meanwhile.
// Anything else is an error at once, and every reader takes an error as a
// broken file: the guard then refuses every command. A file that isn't there
// keeps its not-exist error, as os.ReadFile's.
func ReadFile(path string) ([]byte, error) {
	f, err := openGuardFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errFileNotPlain
	}
	if names, ok := guardFileLinks(f, info); !ok || names != 1 {
		return nil, errFileNames
	}
	if info.Size() > fileMost {
		return nil, errFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, fileMost+1))
	if err != nil {
		return nil, err
	}
	if len(data) > fileMost {
		return nil, errFileTooLarge
	}
	return data, nil
}
