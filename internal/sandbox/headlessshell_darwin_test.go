package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// T126HEADLESSSHELL1: how a hire looks at a screen in a sandboxed Mac desk. Chrome.app's own headless mode doesn't
// run under a desk's profile (RT126SAM5 F1: it looks up the WindowServer's and LaunchServices' Mach services, which
// stay off the list). chrome-headless-shell, Google's headless-only Chrome, is a plain program in a plain folder, with
// no .app bundle: here it reads a page's DOM and takes a screenshot of it, from a server of the test's on 127.0.0.1,
// inside the desk's real profile, as PrepareProfile writes it from the lists. Nothing is added to the profile, and
// the row checks that its bytes didn't change. Two flags:
//   - --no-sandbox, for Chromium's own sandbox, which can't nest in a desk's (AQ9);
//   - --single-process: its helpers (the network service, the renderer, the GPU's) run inside it. Started as
//     programs of their own, each looks up the browser's Mach port rendezvous service,
//     org.chromium.Chromium.MachPortRendezvousServer.<the browser's pid>, which is off the desk's Mach list: the
//     lookup is refused, the helper ends, and the browser waits for it until it's killed (15 Nov, Chrome for
//     Testing 157.0.8103.78). With only `(allow mach-lookup (global-name-prefix
//     "org.chromium.Chromium.MachPortRendezvousServer."))` appended it runs without the flag: that is all it needs,
//     and it isn't on the list (T126HEADLESSSHELL1-report, for SIERRA).
//
// The shell is SMOLCORP_TEST_CHROME, when that names a chrome-headless-shell: scripts/fetch-headless-shell.sh DIR
// fetches the pinned one, checked against third_party/chrome-headless-shell/SHA256SUMS. A Chrome in an .app is never
// run here. With none, the row skips (SMOLCORP_SANDBOX_REQUIRED=1 makes that a failure: CI's Mac sandbox job).
//
// A browser a test starts never outlives it (CHROMEKILL1's rules): each run has a deadline inside the test's own, runs
// in a process group of its own, led by a watchdog that kills the group and every process with the run's profile
// folder in its words when the test program's pipe ends (the test program killed or timed out); the cleanup kills
// the same, by group and by pid, and then pgrep must find nothing with that folder in its words.

// headlessShellPage is the page the shell is shown: its script writes the paragraph, and its image comes from the same
// server, so the load event (when the shell reads the DOM or takes its shot) comes after both.
const headlessShellPage = `<!doctype html><meta charset="utf-8"><title>a desk's page</title>
<style>html,body{margin:0;height:100%;background:#1e90ff}</style>
<p id="said">no script ran</p><img src="/seen.png" alt="">
<script>document.getElementById("said").textContent = "rendered in a desk"</script>
`

// onePixel is a 1x1 PNG, the page's image.
var onePixel, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func TestOnAMacHeadlessShellLooksAtAPageInADesk(t *testing.T) {
	sandboxExecOrSkip(t)
	shell := headlessShell(t)
	w := newWorkDesk(t, IOKitClasses())
	profileBefore := profileSum(t, w.profile)

	var pages, images atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			pages.Add(1)
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(rw, headlessShellPage)
		case "/seen.png":
			images.Add(1)
			rw.Header().Set("Content-Type", "image/png")
			rw.Write(onePixel)
		default:
			http.NotFound(rw, r)
		}
	}))
	defer server.Close()
	if !strings.HasPrefix(server.URL, "http://127.0.0.1:") {
		t.Fatalf("the page's server is at %s, not on 127.0.0.1", server.URL)
	}

	t.Run("the DOM of a page on 127.0.0.1", func(t *testing.T) {
		r := runHeadlessShell(t, w, w.profile, shell, "--dump-dom", server.URL+"/")
		if r.status != 0 || !strings.Contains(r.stdout, `<p id="said">rendered in a desk</p>`) {
			t.Fatalf("chrome-headless-shell in the desk didn't give the page's DOM: status %d\n%s\n%s", r.status, tail(r.stdout), tail(r.stderr))
		}
		if pages.Load() == 0 || images.Load() == 0 {
			t.Errorf("the server saw %d page and %d image requests: the DOM didn't come from it", pages.Load(), images.Load())
		}
	})
	t.Run("a screenshot at 390 by 844", func(t *testing.T) {
		shot := filepath.Join(w.root, "shots", "page-390x844.png")
		if err := os.MkdirAll(filepath.Dir(shot), 0o700); err != nil {
			t.Fatal(err)
		}
		r := runHeadlessShell(t, w, w.profile, shell, "--window-size=390,844", "--hide-scrollbars", "--screenshot="+shot, server.URL+"/")
		if r.status != 0 {
			t.Fatalf("chrome-headless-shell's screenshot in the desk: status %d\n%s\n%s", r.status, tail(r.stdout), tail(r.stderr))
		}
		f, err := os.Open(shot)
		if err != nil {
			t.Fatalf("no screenshot: %v (%s)", err, tail(r.stderr))
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatalf("the screenshot isn't a PNG: %v", err)
		}
		if b := img.Bounds(); b.Dx() != 390 || b.Dy() != 844 {
			t.Errorf("the screenshot is %dx%d, not 390x844", b.Dx(), b.Dy())
		}
		// The page's own colour, drawn: #1e90ff, below the paragraph.
		red, green, blue, _ := img.At(195, 700).RGBA()
		if red>>8 > 0x30 || green>>8 < 0x80 || green>>8 > 0xa0 || blue>>8 < 0xf0 {
			t.Errorf("the screenshot's colour at (195, 700) is #%02x%02x%02x, not the page's #1e90ff", red>>8, green>>8, blue>>8)
		}
	})
	// The control: it ran inside the fence, not around it. The same shell, in the same desk, is shown the setup key
	// as a file:// page, and gets nothing of it, as every program in a desk. First a file of the desk's own as a
	// file:// page, which it does show: so the key's nothing is the profile's refusal, not a shell that reads no
	// file:// page at all (RT126HEADLESSSHELL1, nit 1).
	t.Run("control: the profile still fences it", func(t *testing.T) {
		own := filepath.Join(w.root, "own-page.txt")
		mustWrite(t, own, "a desk's own file\n")
		if r := runHeadlessShell(t, w, w.profile, shell, "--dump-dom", "file://"+own); r.status != 0 || !strings.Contains(r.stdout, "a desk's own file") {
			t.Fatalf("chrome-headless-shell in the desk didn't show a file of the desk's own: status %d\n%s\n%s", r.status, tail(r.stdout), tail(r.stderr))
		}
		r := runHeadlessShell(t, w, w.profile, shell, "--dump-dom", "file://"+w.key)
		if strings.Contains(r.stdout, escapeKey) {
			t.Errorf("chrome-headless-shell read the setup key in a desk: %s", tail(r.stdout))
		}
	})
	if got := profileSum(t, w.profile); got != profileBefore {
		t.Error("the desk's profile changed during the row: it must run as PrepareProfile wrote it")
	}
}

// CHROMEKILL1's rules for a run, on a stand-in browser in a desk (no browser): it starts a helper, with its own words
// and so the run's profile folder, that leaves the run's process group (as Chrome's crashpad handler can) and would
// stay a minute; then it ends. Once the run's cleanup is done, nothing with the folder in its words is left: the
// helper was killed by its pid, as the group kill can't reach it.
func TestOnAMacAHeadlessShellRunLeavesNothingBehind(t *testing.T) {
	sandboxExecOrSkip(t)
	perl := tool(t, "perl")
	w := newWorkDesk(t, IOKitClasses())
	standIn := filepath.Join(w.root, "standin-browser")
	mustWrite(t, standIn, fmt.Sprintf(standInBrowser, perl))
	if err := os.Chmod(standIn, 0o700); err != nil {
		t.Fatal(err)
	}
	var folder string
	var helper int
	t.Run("the run", func(t *testing.T) {
		r := runHeadlessShell(t, w, w.profile, standIn, "--dump-dom", "about:blank")
		var group int
		for _, line := range strings.Split(r.stdout, "\n") {
			if words, ok := strings.CutPrefix(line, "words "); ok {
				for _, word := range strings.Fields(words) {
					if named, ok := strings.CutPrefix(word, "--user-data-dir="); ok {
						folder = named
					}
				}
			}
			fmt.Sscanf(line, "helper %d group %d", &helper, &group)
		}
		if r.status != 0 || helper == 0 || helper != group || !strings.Contains(folder, "shell-profile-") {
			t.Fatalf("the stand-in's helper didn't leave the group with the run's profile folder: %+v", r)
		}
	})
	if helper == 0 {
		return
	}
	if left := pidsWith(folder); len(left) != 0 {
		t.Errorf("after the run's cleanup, pgrep still finds %v with its profile folder in their words", left)
		killWith(folder)
	}
	if words := processWords(helper); strings.Contains(words, folder) {
		t.Errorf("the helper (pid %d) still runs: %s", helper, words)
	}
}

// standInBrowser is the stand-in browser (%s is perl): it says its words; its helper, with the same words, leaves the
// group, says its pid and group, and sleeps a minute; the browser ends a second later.
const standInBrowser = `#!/bin/sh
echo "words $*"
%s -e 'setpgrp(0, 0); print qq(helper $$ group ), getpgrp(), qq(\n); close STDOUT; close STDERR; sleep 60' -- "$@" &
sleep 1
`

// processWords is what pid runs with now (ps), or "" once it has ended. /bin/ps starts plainly, not through
// disclaim.Cmd: it is on disclaim's list of Apple's programs the core starts as they are (RT126HEADLESSSHELL1, nit 6).
func processWords(pid int) string {
	out, _ := exec.Command("/bin/ps", "-ww", "-o", "command=", "-p", fmt.Sprint(pid)).Output()
	return strings.TrimSpace(string(out))
}

// headlessShell is the chrome-headless-shell to run, from SMOLCORP_TEST_CHROME, or the test skips (or fails, where
// required). Never a program in an .app bundle (_rules.md): the shell is a plain one.
func headlessShell(t *testing.T) string {
	t.Helper()
	named := os.Getenv("SMOLCORP_TEST_CHROME")
	if filepath.Base(named) != "chrome-headless-shell" {
		required(t, "SMOLCORP_TEST_CHROME (%q) doesn't name a chrome-headless-shell: scripts/fetch-headless-shell.sh DIR fetches the pinned one", named)
	}
	if strings.Contains(named, ".app/") {
		t.Fatalf("%s is in an .app bundle: the shell is run as a plain program only", named)
	}
	info, err := os.Stat(named)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		required(t, "no chrome-headless-shell to run at %s: %v", named, err)
	}
	return named
}

// profileSum is the SHA-256 of a profile file, as text.
func profileSum(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// shellWatchdog leads the run's process group: it starts the run ("$@", after the profile folder, $1) in it, says on
// fd 3 how the run ended, and holds the test program's pipe (its stdin, kept as fd 4) until that ends: closed by the
// cleanup, or by the system when the test program ends. Then it kills, by pid, each process outside its group with
// the profile folder in its words (read again right before), and its whole group. As CHROMEKILL1's browserWatchdog.
const shellWatchdog = `profile=$1
shift
sweep() {
	ps -A -ww -o pid= -o pgid= -o stat= -o command= | while read -r pid group state words; do
		case $group:$state in "$$":*|*:Z*) continue ;; esac
		case $words in *"$profile"*) ;; *) continue ;; esac
		case $(ps -ww -o command= -p "$pid") in *"$profile"*) kill -s KILL "$pid" ;; esac
	done
}
exec 4<&0 0</dev/null
"$@" 3>&- 4<&- &
run=$!
(read -r _ <&4; case $profile in /?*/shell-profile-*) sweep; sweep ;; esac; kill -s KILL -- -$$) 3>&- &
exec 4<&-
wait "$run"
echo "status $?" >&3
exec 3>&-
wait
`

// runHeadlessShell runs the shell in the desk's sandbox, under profile, with --no-sandbox --single-process, the
// _rules.md flags and args after them, and gives how it ended. Its profile folder is a new one in the desk's TMPDIR.
func runHeadlessShell(t *testing.T, w *workDesk, profile, shell string, args ...string) ran {
	t.Helper()
	sandboxExec := sandboxExecOrSkip(t)
	folder, err := os.MkdirTemp(filepath.Join(w.base, "t"), "shell-profile-")
	if err != nil {
		t.Fatal(err)
	}
	limit := 90 * time.Second
	if deadline, ok := t.Deadline(); ok {
		limit = min(limit, time.Until(deadline)-min(30*time.Second, time.Until(deadline)/4))
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	words := append([]string{"-c", shellWatchdog, "sh", folder, sandboxExec, "-f", profile, shell, "--no-sandbox", "--single-process", "--use-mock-keychain",
		"--password-store=basic", "--disable-features=DialMediaRouteProvider", "--user-data-dir=" + folder, "--no-first-run",
		"--no-default-browser-check", "--disable-background-networking", "--disable-component-update"}, args...)
	cmd := disclaim.Cmd(exec.CommandContext(ctx, "/bin/sh", words...))
	cmd.Dir = w.root
	cmd.Env = []string{"HOME=" + w.home, "LANG=C", "TERM=xterm", "TMPDIR=" + filepath.Join(w.base, "t") + "/", pathOf()}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		killWith(folder)
	}
	cmd.Cancel = func() error { kill(); return nil }
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	watch, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	status, report, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer status.Close()
	cmd.ExtraFiles = []*os.File{report}
	err = cmd.Start()
	report.Close()
	if err != nil {
		t.Fatalf("starting chrome-headless-shell's watchdog: %v", err)
	}
	// Whatever way the run ends, nothing with its profile folder is left: by group, by pid, then pgrep.
	t.Cleanup(func() {
		kill()
		watch.Close()
		if left := leftWith(folder, 5*time.Second); left != "" {
			t.Errorf("processes with this run's profile folder are still running after they were killed (tell VICTOR):\n%s", left)
		}
	})
	line, _ := bufio.NewReader(status).ReadString('\n')
	watch.Close()
	cmd.Wait()
	if ctx.Err() != nil {
		t.Fatalf("chrome-headless-shell %q didn't end within %s, and was killed (%s)", args, limit, tail(stderr.String()))
	}
	r := ran{status: -1, stdout: stdout.String(), stderr: stderr.String()}
	fmt.Sscanf(line, "status %d", &r.status)
	return r
}

// pidsWith is pgrep's pids of the processes with folder in their words, never this test program's.
func pidsWith(folder string) []int {
	out, _ := disclaim.Cmd(exec.Command("/usr/bin/pgrep", "-f", "--", folder)).Output()
	var pids []int
	for _, field := range strings.Fields(string(out)) {
		var pid int
		if _, err := fmt.Sscan(field, &pid); err == nil && pid != os.Getpid() {
			pids = append(pids, pid)
		}
	}
	return pids
}

// killWith kills, by pid, each process with folder (a run's own, made by the test) in its words, until none is left
// or for 2 s at most. Never by name: each pid's words are read again right before, so a pid given since to another
// program is let be.
func killWith(folder string) {
	for until := time.Now().Add(2 * time.Second); time.Now().Before(until); time.Sleep(50 * time.Millisecond) {
		pids := pidsWith(folder)
		if len(pids) == 0 {
			return
		}
		for _, pid := range pids {
			if strings.Contains(processWords(pid), folder) {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
}

// leftWith is what pgrep still finds with folder in its words once nothing is or wait has passed: "" for none.
func leftWith(folder string, wait time.Duration) string {
	for until := time.Now().Add(wait); ; time.Sleep(100 * time.Millisecond) {
		pids := pidsWith(folder)
		if len(pids) == 0 {
			return ""
		}
		if time.Now().After(until) {
			out, _ := disclaim.Cmd(exec.Command("/usr/bin/pgrep", "-l", "-f", "--", folder)).Output()
			return strings.TrimSpace(string(out))
		}
	}
}
