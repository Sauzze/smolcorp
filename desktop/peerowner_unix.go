//go:build darwin || linux

package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
)

// openServeSocket removes whatever is at path and listens there, for the owner alone (0600), with each connection
// identified as it's accepted (serveListener). A path too long for a socket is refused rather than cut short. The same
// on a Mac and on Linux; only sunPathMost differs.
func openServeSocket(path string) (net.Listener, error) {
	if len(path) > sunPathMost {
		return nil, fmt.Errorf("%s is too long for a socket (%d bytes, at most %d)", path, len(path), sunPathMost)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return &serveListener{Listener: listener}, nil
}
