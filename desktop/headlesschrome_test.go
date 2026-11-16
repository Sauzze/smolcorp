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

// startHeadlessChrome starts chrome (a Chrome or Chromium, from headlessChrome) headless, with words after its own (a
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
	if !installedOrNoBundle(chrome) {
		t.Fatalf("%s is in a bundle that isn't an installed app's: a stand-in browser is a plain program, never an app bundle", chrome)
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

// headlessChrome is a Chrome or Chromium to run headless: SMOLCORP_TEST_CHROME, or one where it's usually installed.
func headlessChrome() string {
	if named := os.Getenv("SMOLCORP_TEST_CHROME"); named != "" {
		return named
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}
	paths := []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	for _, folder := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		if at := os.Getenv(folder); at != "" {
			paths = append(paths, filepath.Join(at, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// installedOrNoBundle is whether program is in no .app bundle, or in an installed app's (in /Applications, or in the
// Applications of this account's home): one a test made, in a temp folder or anywhere else, is neither.
func installedOrNoBundle(program string) bool {
	if !bundlePath.MatchString(program) {
		return true
	}
	roots := []string{"/Applications/"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Join(home, "Applications")+"/")
	}
	clean := filepath.Clean(program)
	if made, err := filepath.Rel(os.TempDir(), clean); err == nil && !strings.HasPrefix(made, "..") {
		return false
	}
	for _, root := range roots {
		if strings.HasPrefix(clean, root) {
			return true
		}
	}
	return false
}
