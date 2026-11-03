package desktop

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeXdgOpen puts an xdg-open running script alone on PATH, in a temporary folder.
func fakeXdgOpen(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "xdg-open"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func shortOpenWait(t *testing.T, wait time.Duration) {
	saved := xdgOpenWait
	t.Cleanup(func() { xdgOpenWait = saved })
	xdgOpenWait = wait
}

// With no browser, xdg-open exits with an error at once: the page's ⚙︎ and
// Open the office then hear it failed, not 204 (SIERRA 1.23.0 G-1).
func TestABrowserThatWontStartIsAnErrorNotASilentSuccess(t *testing.T) {
	shortOpenWait(t, 5*time.Second)
	fakeXdgOpen(t, "exit 3")
	start := time.Now()
	err := openURL("http://localhost:1/setup")
	if err == nil || !strings.Contains(err.Error(), "xdg-open") {
		t.Fatalf("openURL = %v, want an error naming xdg-open", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("a failing opener held the caller for %v, not until it exited", time.Since(start))
	}
}

func TestABrowserThatOpensIsSuccess(t *testing.T) {
	shortOpenWait(t, 5*time.Second)
	fakeXdgOpen(t, "exit 0")
	if err := openURL("http://localhost:1/setup"); err != nil {
		t.Errorf("openURL = %v, want success", err)
	}
}

// An opener that stays in the foreground is taken to have worked once the wait
// is over, the caller isn't held longer, and the child is reaped when it ends.
func TestAnOpenerStillRunningAfterTheWaitIsSuccessAndIsReaped(t *testing.T) {
	shortOpenWait(t, 200*time.Millisecond)
	pidFile := filepath.Join(t.TempDir(), "pid")
	fakeXdgOpen(t, "echo $$ > '"+pidFile+"'\nexec /bin/sleep 1")
	start := time.Now()
	if err := openURL("http://localhost:1/setup"); err != nil {
		t.Fatalf("openURL = %v, want success", err)
	}
	if time.Since(start) > 900*time.Millisecond {
		t.Errorf("held the caller for %v, past the wait", time.Since(start))
	}
	var raw []byte
	for i := 0; i < 100 && len(strings.TrimSpace(string(raw))) == 0; i++ {
		raw, _ = os.ReadFile(pidFile)
		time.Sleep(10 * time.Millisecond)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) }) // only our own child, by pid
	// Once it ends it must be gone, not a zombie, and no longer a child.
	deadline := time.Now().Add(5 * time.Second)
	for {
		children.Lock()
		tracked := children.pids[pid]
		children.Unlock()
		_, statErr := os.Stat("/proc/" + strconv.Itoa(pid))
		if !tracked && os.IsNotExist(statErr) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid %d still there after it ended (tracked %v, /proc: %v): not reaped", pid, tracked, statErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
