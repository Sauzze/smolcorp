//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// The app reads the guard file as the guard does (SG0): a pipe (G2), a link to
// /dev/zero (G3), a hard link or a file over 64 KiB in its place is answered at
// once as a broken file, every group on, and a save from the page writes a
// plain file over it that the guard then reads.
func TestTheAppReadsTheGuardFileSafely(t *testing.T) {
	for name, plant := range map[string]func(path string) error{
		"pipe":      func(path string) error { return syscall.Mkfifo(path, 0o600) },
		"/dev/zero": func(path string) error { return os.Symlink("/dev/zero", path) },
		"hard link": func(path string) error {
			if err := os.WriteFile(path, guard.DefaultFile(), 0o600); err != nil {
				return err
			}
			return os.Link(path, path+".other")
		},
		"large": func(path string) error {
			return os.WriteFile(path, append(guard.DefaultFile(), make([]byte, 64<<10)...), 0o600)
		},
	} {
		path := filepath.Join(t.TempDir(), hireGuardFile)
		if err := plant(path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		done := make(chan map[string]any, 1)
		go func() { done <- guardState(path, runtime.GOOS) }()
		select {
		case state := <-done:
			if state["problem"] == nil {
				t.Errorf("%s: no problem said: %v", name, state)
			}
			for _, group := range state["groups"].([]map[string]any) {
				if group["on"] != true {
					t.Errorf("%s: %v is off", name, group["name"])
				}
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the app's read hung", name)
		}
		if answer := saveGuard([]byte(`{"groups":[{"name":"cloud","on":false}]}`), path, runtime.GOOS); answer.status != http.StatusOK {
			t.Fatalf("%s: save %d %s", name, answer.status, answer.message)
		}
		groups, problem := guard.Load(path)
		if problem != "" || guard.Check("aws s3 ls", groups) != nil || guard.Check("git push", groups) == nil {
			t.Errorf("%s: after a save: %q", name, problem)
		}
	}
}
