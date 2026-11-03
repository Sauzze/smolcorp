package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// `smolcorp quit` stops this copy of Smolcorp, as the one for this HOME: the
// running copy writes its pid next to the keys when it starts, and removes it
// when it ends cleanly. Quit reads that pid, and only that: it never looks for
// a process by name, and a record of another HOME is never in this one's folder.

// pidFile is the record of the running copy's pid.
var pidFile = func() string { return filepath.Join(supportDir, "smolcorp.pid") }

// quitWait is how long quit waits for the copy to end after asking it to: a
// variable, for the tests.
var quitWait = 8 * time.Second

// recordPid writes this process's pid, whole or not at all.
func recordPid() {
	if err := writePrivate(pidFile(), []byte(strconv.Itoa(os.Getpid())+"\n")); err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: couldn't record its pid (%v): smolcorp quit won't find this copy\n", err)
	}
}

// forgetPid removes the record, if it is this process's: a copy that took
// this one's place may have written its own.
func forgetPid() {
	if pid, ok := recordedPid(); ok && pid == os.Getpid() {
		_ = os.Remove(pidFile())
	}
}

// recordedPid is the pid in the record, if there is a good one.
func recordedPid() (int, bool) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil && pid > 1
}

// quitByTray: where quit only says to use the tray (Windows, which has no
// SIGTERM to ask with and no /proc to check the pid against).
var quitByTray = runtime.GOOS == "windows"

// RunQuit is `smolcorp quit`.
func RunQuit() int {
	if quitByTray {
		fmt.Println("On Windows, quit Smolcorp from its tray icon.")
		return 1
	}
	data, err := os.ReadFile(pidFile())
	if err != nil {
		fmt.Println("Smolcorp isn't running (no record of it for this user).")
		return 0
	}
	pid, ok := recordedPid()
	if !ok {
		_ = os.Remove(pidFile())
		fmt.Printf("Smolcorp isn't running: the record (%q) was damaged, and is removed.\n", strings.TrimSpace(string(data)))
		return 0
	}
	if pid == os.Getpid() {
		fmt.Println("The record names this very command, not a running Smolcorp; it was left alone.")
		return 1
	}
	running, err := thisHomeRunning()
	if err != nil {
		fmt.Printf("Couldn't tell whether Smolcorp runs for this user (%v), so nothing was signalled.\n", err)
		return 1
	}
	if !running || processGone(pid) {
		// No copy holds this HOME's lock, or the process is gone: the record is
		// left over from a crash, and its pid may belong to anyone by now.
		_ = os.Remove(pidFile())
		fmt.Printf("Smolcorp isn't running: the record (process %d) was stale, and is removed.\n", pid)
		return 0
	}
	if !isSmolcorpProcess(pid) {
		fmt.Printf("Process %d, named in the record, isn't a Smolcorp, so it was left alone (its number was reused). "+
			"If Smolcorp still runs, quit it from its tray menu, or with no tray press Ctrl+C in its terminal. The record is in %s.\n", pid, pidFile())
		return 1
	}
	if err := askToEnd(pid); err != nil {
		fmt.Printf("Smolcorp (process %d) couldn't be asked to quit: %v\n", pid, err)
		return 1
	}
	for deadline := time.Now().Add(quitWait); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if processGone(pid) {
			if now, ok := recordedPid(); ok && now == pid {
				_ = os.Remove(pidFile())
			}
			fmt.Printf("Smolcorp (process %d) stopped. Agents started with smolcorp run keep running.\n", pid)
			return 0
		}
	}
	fmt.Printf("Smolcorp (process %d) was asked to quit and is still running after %d s. Quit it from its tray menu, or with no tray press Ctrl+C in its terminal.\n", pid, int(quitWait.Seconds()))
	return 1
}
