package desktop

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// T126CHROMEKILL1: a browser a test starts never outlives it. A headless Chrome stopped by a deferred Kill of its
// main process was left running for 5 hours once (a `go test -timeout` panic runs no defers, nor does a killed test
// program), and the owner's own Chrome crashed with the Mac low on memory. Every browser a test starts goes through
// startHeadlessChrome (TestEveryBrowserATestStartsGoesThroughTheHelper holds it, headlesschrome_guard_test.go):
//   - its lifetime is the test's own: it's killed a margin before the test's deadline (-test.timeout);
//   - it runs in a process group of its own, and the test's cleanup kills that whole group and waits, on a pass, a
//     failure or a panic; and by pid, each process with the browser's own profile in its words: a helper of the
//     browser's that left the group (FXCHROMEKILL1R1 B1);
//   - and it's watched: on a Mac or Linux, a shell that leads the group holds a pipe from the test program and kills
//     the profile's processes and the group the moment the pipe ends, which it does when the test program does,
//     killed or timed out; on Windows, a job object that only the test program holds kills all of it when the
//     program ends (headlesschrome_unix_test.go, headlesschrome_windows_test.go).

// headlessBrowser is a browser started by startHeadlessChrome.
type headlessBrowser struct {
	profile string        // its --user-data-dir, a folder of this test's: every process of the browser's has it in its words
	ended   chan struct{} // closed once the browser has ended, by itself or killed
	why     string        // how it ended, once ended is closed
	output  lockedBuffer  // what it printed

	once   sync.Once
	waited chan error // what its start's Wait gave, once it's reaped
	cmd    *exec.Cmd
	group  browserGroup // its process group, and on a Mac or Linux the watchdog's pipe (per system)
}

// browserMargin is how long before the test's deadline its browser is killed: a quarter of what's left, at most 30 s.
func browserMargin(left time.Duration) time.Duration {
	return min(30*time.Second, left/4)
}

// startHeadlessChrome starts chrome (a chrome-headless-shell, from headlessChrome) headless, with words after its own (a
// window size, the page's address), and kills it and all it started when the test ends, whatever way it ends. Never a
// window, the Keychain or any other system dialog (_rules.md): a mock keychain, and a profile and a home of its own.
func startHeadlessChrome(t *testing.T, chrome string, words ...string) *headlessBrowser {
	t.Helper()
	var until time.Time
	if deadline, ok := t.Deadline(); ok {
		until = deadline.Add(-browserMargin(time.Until(deadline)))
	}
	return startBrowserUntil(t, chrome, until, words)
}

// startBrowserUntil is startHeadlessChrome with the moment it's killed at the latest (none if zero).
func startBrowserUntil(t *testing.T, chrome string, until time.Time, words []string) *headlessBrowser {
	t.Helper()
	// Started as a plain program, by its path, never through LaunchServices: a bundle a test made would be checked by
	// Gatekeeper, which shows a dialog on the owner's screen (VICTOR, 15 Nov). A stand-in is a script, never a .app.
	// And never the owner's own browser: refused here, before anything starts, whatever found it (FXCHROMEPOPUPR1).
	if why := browserRefusal(chrome); why != "" {
		t.Fatalf("refused before it started: %s %s. A test's browser is a chrome-headless-shell named in SMOLCORP_TEST_CHROME, "+
			"or a stand-in it made in its temp folder, a plain program (FXCHROMEPOPUPR1)", chrome, why)
	}
	dir := t.TempDir()
	b := &headlessBrowser{profile: filepath.Join(dir, "profile"), ended: make(chan struct{}), waited: make(chan error, 1)}
	args := []string{"--headless=new", "--use-mock-keychain", "--password-store=basic", "--disable-features=DialMediaRouteProvider",
		"--no-first-run", "--no-default-browser-check", "--user-data-dir=" + b.profile}
	if runtime.GOOS == "linux" {
		// A CI runner may not give Chrome its sandbox; the pages are the tests' own, on 127.0.0.1.
		args = append(args, "--no-sandbox")
	}
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	if !until.IsZero() {
		ctx, cancel = context.WithDeadline(ctx, until)
	}
	if err := b.start(ctx, chrome, append(args, words...), append(os.Environ(), "HOME="+dir)); err != nil {
		cancel()
		t.Fatalf("starting %s: %v", chrome, err)
	}
	// Registered after t.TempDir's, so it runs first: the profile is let go before its folder is removed.
	t.Cleanup(func() {
		defer cancel()
		b.group.kill()
		killProfile(b.profile) // a helper that left the group, by its pid (FXCHROMEKILL1R1 B1)
		b.group.release()
		select {
		case <-b.waited:
		case <-time.After(10 * time.Second):
			t.Errorf("the browser (pid %d) hadn't ended 10 s after it was killed: %s", b.cmd.Process.Pid, b.said())
		}
		if left := browserLeftovers(b.profile, 5*time.Second); left != "" {
			t.Errorf("processes with this test's profile are still running 5 s after they were killed, by group and by pid (tell VICTOR):\n%s", left)
		}
	})
	return b
}

// end says the browser has ended, and how: once.
func (b *headlessBrowser) end(why string) {
	b.once.Do(func() {
		b.why = why
		close(b.ended)
	})
}

// said is what the browser has printed so far.
func (b *headlessBrowser) said() string { return b.output.String() }

// lockedBuffer is a bytes.Buffer a process writes to while a test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// headlessChrome is the browser a test runs headless: SMOLCORP_TEST_CHROME when it names a chrome-headless-shell, a
// plain program in no bundle (on a Mac, scripts/fetch-headless-shell.sh DIR fetches the pinned one), or "", and the
// test skips. It never looks for one, on the PATH or where browsers are installed: the owner's own Chrome is never a
// test's (FXCHROMEPOPUP).
func headlessChrome() string {
	named := os.Getenv("SMOLCORP_TEST_CHROME")
	if strings.TrimSuffix(filepath.Base(named), ".exe") != "chrome-headless-shell" || bundlePath.MatchString(named) {
		return ""
	}
	return named
}

// browserRefusal is why a test may not start program as its browser, or "" when it may (FXCHROMEPOPUPR1, SIERRA's
// ruling on RT126CHROMEPOPUP B1): only a plain chrome-headless-shell (the opt-in's, SMOLCORP_TEST_CHROME) or a
// stand-in the test made in its temp folder; never a program in a .app bundle or in an Applications folder (where the
// owner's browsers are installed), nor a link to one. It reads the path's words and where a link goes, never runs it,
// so a finder that falls back to the owner's browser (by another variable, a helper of the package's code, a command
// on the PATH) fails its test instead of starting it.
func browserRefusal(program string) string {
	seen := []string{program}
	if target, err := os.Readlink(program); err == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(program), target)
		}
		seen = append(seen, target)
	}
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		seen = append(seen, resolved)
	}
	applications := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		applications = append(applications, filepath.Join(home, "Applications"))
	}
	for _, at := range seen {
		clean := filepath.Clean(at)
		if bundlePath.MatchString(clean) {
			return "is in an app bundle"
		}
		for _, folder := range applications {
			if under(clean, filepath.Clean(folder)) {
				return "is in " + folder + ", where apps are installed"
			}
		}
		if strings.TrimSuffix(filepath.Base(clean), ".exe") != "chrome-headless-shell" && !inTempFolder(clean) {
			return "is neither a chrome-headless-shell nor in this test's temp folder"
		}
	}
	return ""
}

// under is whether path is folder or in it, any case (a Mac's disk doesn't tell /applications from /Applications).
func under(path, folder string) bool {
	path, folder = strings.ToLower(path), strings.ToLower(folder)
	return path == folder || strings.HasPrefix(path, strings.TrimSuffix(folder, string(filepath.Separator))+string(filepath.Separator))
}

// inTempFolder is whether path is in the temp folder (t.TempDir's), as named or with its links followed.
func inTempFolder(path string) bool {
	temps := []string{os.TempDir()}
	if resolved, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		temps = append(temps, resolved)
	}
	for _, temp := range temps {
		if under(path, filepath.Clean(temp)) {
			return true
		}
	}
	return false
}
