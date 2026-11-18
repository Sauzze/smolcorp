//go:build windows

package desktop

import "golang.org/x/sys/windows"

// shellPath is a path as a hook command names it: the path itself, or its
// short form read back to be this very file, or the path quoted (see
// windowsHookPath).
func shellPath(path string) string {
	return windowsHookPath(path, shortPathName, getLongPathName)
}

// shortPathName is Windows' short (8.3) form of a path that's there.
func shortPathName(path string) (string, error) {
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	short := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(long, &short[0], uint32(len(short)))
	if err != nil {
		return "", err
	}
	if n == 0 || int(n) > len(short) {
		return "", windows.ERROR_INSUFFICIENT_BUFFER
	}
	return windows.UTF16ToString(short[:n]), nil
}
