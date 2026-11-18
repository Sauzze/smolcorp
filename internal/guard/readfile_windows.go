//go:build windows

package guard

import (
	"os"
	"syscall"
)

// openGuardFile opens path for ReadFile: FILE_FLAG_OPEN_REPARSE_POINT opens a
// link (or any reparse point) as itself, which is then refused, and only a file
// on disk passes (not a pipe, NUL or a console).
func openGuardFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var data syscall.ByHandleFileInformation
	kind, kindErr := syscall.GetFileType(syscall.Handle(f.Fd()))
	switch {
	case kindErr != nil || kind != syscall.FILE_TYPE_DISK:
		f.Close()
		return nil, errFileNotPlain
	case syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &data) != nil:
		f.Close()
		return nil, errFileNotPlain
	case data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		f.Close()
		return nil, errFileLink
	}
	return f, nil
}

// guardFileLinks is how many names the open file has, and whether that is known.
func guardFileLinks(f *os.File, _ os.FileInfo) (uint64, bool) {
	var data syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &data) != nil {
		return 0, false
	}
	return uint64(data.NumberOfLinks), true
}
