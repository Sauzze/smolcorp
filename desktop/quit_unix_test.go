//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holdLock takes the instance lock the way a running copy does, until the test ends.
func holdLock(t *testing.T) {
	t.Helper()
	file, err := os.OpenFile(instanceFile(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
}

func quitWith(t *testing.T, record string) (code int, out string) {
	t.Helper()
	if record != "" {
		writeFile(t, pidFile(), record)
	}
	saved := quitWait
	quitWait = 3 * time.Second
	t.Cleanup(func() { quitWait = saved })
	read := captureStdout(t)
	code = RunQuit()
	return code, read()
}

// captureStdout redirects os.Stdout until the returned function is called.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = saved })
	return func() string {
		os.Stdout = saved
		w.Close()
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		return string(buf[:n])
	}
}

func TestQuitWithNoRecordSaysNotRunning(t *testing.T) {
	isolate(t)
	if code, out := quitWith(t, ""); code != 0 || !strings.Contains(out, "isn't running") {
		t.Fatalf("code %d, %q", code, out)
	}
}

func TestQuitRemovesAStaleRecord(t *testing.T) {
	isolate(t)
	gone := exec.Command("true")
	if err := gone.Run(); err != nil { // started and reaped: its pid is free
		t.Fatal(err)
	}
	signalled := false
	saved := isSmolcorpProcess
	isSmolcorpProcess = func(int) bool { signalled = true; return false }
	t.Cleanup(func() { isSmolcorpProcess = saved })
	code, out := quitWith(t, strconv.Itoa(gone.Process.Pid)+"\n")
	if code != 0 || !strings.Contains(out, "stale") || signalled {
		t.Fatalf("code %d, %q, identity asked %v", code, out, signalled)
	}
	if _, err := os.Stat(pidFile()); !os.IsNotExist(err) {
		t.Fatal("the stale record is still there")
	}
}

func TestQuitRefusesAPidThatIsNotSmolcorp(t *testing.T) {
	isolate(t)
	holdLock(t)
	other := exec.Command("sleep", "60") // a program of the test's own, not a Smolcorp
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = other.Wait(); close(done) }()
	t.Cleanup(func() { _ = other.Process.Kill(); <-done })
	code, out := quitWith(t, strconv.Itoa(other.Process.Pid)+"\n")
	if code != 1 || !strings.Contains(out, "isn't a Smolcorp") {
		t.Fatalf("code %d, %q", code, out)
	}
	select {
	case <-done:
		t.Fatal("the process that isn't a Smolcorp was signalled")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestQuitStopsTheRecordedSmolcorpAndNoOther(t *testing.T) {
	isolate(t)
	holdLock(t) // a copy for this HOME is running
	fake := exec.Command("sleep", "60")
	if err := fake.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = fake.Wait(); close(done) }() // reaped, so it's gone once it ends
	t.Cleanup(func() { _ = fake.Process.Kill(); <-done })
	pid := fake.Process.Pid
	var asked []int
	saved := isSmolcorpProcess
	isSmolcorpProcess = func(p int) bool { asked = append(asked, p); return p == pid }
	t.Cleanup(func() { isSmolcorpProcess = saved })
	code, out := quitWith(t, strconv.Itoa(pid)+"\n")
	if code != 0 || !strings.Contains(out, "stopped") {
		t.Fatalf("code %d, %q", code, out)
	}
	select {
	case <-done:
	default:
		t.Fatal("the fake Smolcorp is still running")
	}
	if len(asked) != 1 || asked[0] != pid {
		t.Fatalf("identity was asked of %v, want only %d", asked, pid)
	}
	if _, err := os.Stat(pidFile()); !os.IsNotExist(err) {
		t.Fatal("the record is still there")
	}
}

func TestTheRecordIsWrittenWholeAndRemovedByItsOwner(t *testing.T) {
	isolate(t)
	recordPid()
	if pid, ok := recordedPid(); !ok || pid != os.Getpid() {
		t.Fatalf("recorded %d, %v", pid, ok)
	}
	writeFile(t, pidFile(), "4242\n") // a copy that took this one's place
	forgetPid()
	if _, err := os.Stat(pidFile()); err != nil {
		t.Fatal("another copy's record was removed")
	}
	recordPid()
	forgetPid()
	if _, err := os.Stat(pidFile()); !os.IsNotExist(err) {
		t.Fatal("its own record stayed after a clean exit")
	}
}

func TestIdentityCheckKnowsThisTestIsNotSmolcorp(t *testing.T) {
	if isSmolcorpProcess(os.Getpid()) || isSmolcorpProcess(1<<30) {
		t.Fatal("the test binary or a missing pid taken for a Smolcorp")
	}
}

func TestQuitWithAFreeLockRemovesTheRecordAndSignalsNothing(t *testing.T) {
	isolate(t)
	fake := exec.Command("sleep", "60") // alive, and looks like a Smolcorp, but no copy holds this HOME's lock
	if err := fake.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = fake.Wait(); close(done) }()
	t.Cleanup(func() { _ = fake.Process.Kill(); <-done })
	saved := isSmolcorpProcess
	isSmolcorpProcess = func(int) bool { return true }
	t.Cleanup(func() { isSmolcorpProcess = saved })
	for _, lockFile := range []bool{false, true} { // no lock file at all, and a lock file nobody holds
		if lockFile {
			writeFile(t, instanceFile(), "")
		}
		code, out := quitWith(t, strconv.Itoa(fake.Process.Pid)+"\n")
		if code != 0 || !strings.Contains(out, "stale") {
			t.Fatalf("lock file %v: code %d, %q", lockFile, code, out)
		}
		if _, err := os.Stat(pidFile()); !os.IsNotExist(err) {
			t.Fatal("the record is still there")
		}
		select {
		case <-done:
			t.Fatal("a process of another copy was signalled")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func TestQuitSignalsOnlyWhileThisHomeHoldsTheLock(t *testing.T) {
	isolate(t)
	if running, err := thisHomeRunning(); err != nil || running {
		t.Fatalf("no lock file: %v, %v", running, err)
	}
	holdLock(t)
	if running, err := thisHomeRunning(); err != nil || !running {
		t.Fatalf("lock held: %v, %v", running, err)
	}
}

// On Windows quit says to use the tray, and never that a running Smolcorp isn't one.
func TestQuitOnWindowsSaysToUseTheTray(t *testing.T) {
	isolate(t)
	saved := quitByTray
	quitByTray = true
	t.Cleanup(func() { quitByTray = saved })
	writeFile(t, pidFile(), "4242\n")
	read := captureStdout(t)
	code := RunQuit()
	out := read()
	if code != 1 || !strings.Contains(out, "quit Smolcorp from its tray icon") || strings.Contains(out, "isn't a Smolcorp") {
		t.Errorf("code %d, %q", code, out)
	}
}
