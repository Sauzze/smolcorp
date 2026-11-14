//go:build darwin || linux

package desktop

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serve.sock is made again at start: whatever was at its path (a socket left by the last run, or a file) goes, and
// the new one is the owner's alone. A path too long for a socket is refused, and nothing is made. On a Mac and on
// Linux alike.
func TestServeSockIsMadeAgainForTheOwnerAlone(t *testing.T) {
	dir := shortTempDir(t)
	path := filepath.Join(dir, serveSocketName)
	for _, left := range []string{"nothing", "a socket", "a file"} {
		if left == "a file" {
			if err := os.WriteFile(path, []byte("left"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		listener, err := openServeSocket(path)
		if err != nil {
			t.Fatalf("over %s: %v", left, err)
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode().Type() != os.ModeSocket || info.Mode().Perm() != 0o600 {
			t.Errorf("over %s: %v %v, want a socket, 0600", left, info.Mode(), err)
		}
		if _, ok := listener.(*serveListener); !ok {
			t.Errorf("over %s: %T doesn't identify what it accepts", left, listener)
		}
		if left == "nothing" {
			// Left behind, as a run that ended without closing it leaves it, for the next one to make again.
			listener.(*serveListener).Listener.(*net.UnixListener).SetUnlinkOnClose(false)
		}
		listener.Close()
		if _, err := os.Lstat(path); (err == nil) != (left == "nothing") {
			t.Errorf("over %s, closed: %v", left, err)
		}
	}
	long := filepath.Join(dir, strings.Repeat("d", sunPathMost), serveSocketName)
	if _, err := openServeSocket(long); err == nil || !strings.Contains(err.Error(), "too long") {
		t.Errorf("a path of %d bytes: %v", len(long), err)
	}
	if _, err := os.Lstat(filepath.Dir(long)); err == nil {
		t.Error("something was made for a path too long")
	}
}

// shortTempDir is a folder of the test's own whose paths fit a socket's: t.TempDir's, named after the test, can
// be longer than sun_path.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sn1-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
