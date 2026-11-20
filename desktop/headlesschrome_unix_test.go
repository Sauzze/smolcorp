//go:build !windows

package desktop

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// browserWatchdog leads the browser's process group (Setpgid): it starts the browser ("$@", after the profile, $1) in
// it, says on fd 3 when the browser has ended, and stays, holding the test program's pipe (its stdin, kept as fd 4),
// until that pipe ends: the test's cleanup closes it, and the system closes it when the test program ends, killed or
// timed out with no cleanup run. Then it kills, by pid, each process outside its group with the profile in its words
// (a helper of the browser's that left the group, into a session or a group of its own, as Chrome's crashpad handler
// can), reading each one's words again right before; then its whole group, itself included: only ever its own, as -$$
// names a group only while it leads one. Neither the browser nor the pipe's reader holds fd 3, so it ends once the
// browser has.
const browserWatchdog = `profile=$1
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
browser=$!
(read -r _ <&4; case $profile in /?*/profile) sweep; sweep; sweep ;; esac; kill -s KILL -- -$$) 3>&- &
exec 4<&-
wait "$browser"
echo "status $?" >&3
exec 3>&-
wait
`

// browserGroup is the browser's process group, led by its watchdog, and the pipe the watchdog holds.
type browserGroup struct {
	group processGroup
	watch io.Closer
}

func (g browserGroup) kill() {
	if g.group.id > 0 {
		g.group.kill()
	}
}

// release closes the watchdog's pipe: it kills the group too, whatever is left of it.
func (g browserGroup) release() {
	if g.watch != nil {
		g.watch.Close()
	}
}

// start starts the browser under its watchdog. Past ctx's deadline, the whole group is killed, not only the watchdog.
func (b *headlessBrowser) start(ctx context.Context, program string, args, env []string) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", browserWatchdog, "sh", b.profile, program}, args...)...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &b.output, &b.output
	ownGroup(cmd)
	cmd.Cancel = func() error {
		err := processGroup{cmd.Process.Pid}.kill()
		killProfile(b.profile) // and its helpers outside the group
		return err
	}
	cmd.WaitDelay = 5 * time.Second // what a helper of the browser's printed mustn't hold Wait
	watch, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	status, report, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.ExtraFiles = []*os.File{report}
	err = cmd.Start()
	report.Close()
	if err != nil {
		status.Close()
		return err
	}
	b.cmd, b.group = cmd, browserGroup{group: processGroup{cmd.Process.Pid}, watch: watch}
	go func() {
		line, _ := bufio.NewReader(status).ReadString('\n')
		status.Close()
		if line = strings.TrimSpace(line); line == "" {
			line = "killed"
		}
		b.end(line)
	}()
	go func() { b.waited <- cmd.Wait() }()
	return nil
}

// processesWith gives the processes running with marker in their words (ps, read in full): their pids and words. Never
// this test program, nor one that has ended and is only waiting to be reaped. A marker is a folder of a test's own: a
// short or relative one gives none.
func processesWith(marker string) (map[int]string, error) {
	if !filepath.IsAbs(marker) || len(marker) < 8 {
		return nil, nil
	}
	out, err := exec.Command("ps", "-A", "-ww", "-o", "pid=", "-o", "stat=", "-o", "command=").Output()
	if err != nil {
		return nil, err
	}
	found := map[int]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[1], "Z") {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if words := strings.Join(fields[2:], " "); err == nil && pid != os.Getpid() && strings.Contains(words, marker) {
			found[pid] = words
		}
	}
	return found, nil
}

// processWords is what pid runs with now (ps), or "" once it has ended.
func processWords(pid int) string {
	out, _ := exec.Command("ps", "-ww", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	return strings.TrimSpace(string(out))
}

// killProfile kills, by pid, each process with this test's own profile in its words: only this test starts those, as
// the profile is a folder of its own test's (t.TempDir). That reaches a helper of the browser's outside its group,
// in a session or a group of its own. Never by name: each pid's words are read again right before it's killed, so one
// that has ended since, and whose pid was given to another program, is let be. Again until none is left (2 s at most),
// as a helper may start another while it's killed. Left: a helper that leaves the session AND drops the profile from
// its words.
func killProfile(profile string) {
	for until := time.Now().Add(2 * time.Second); time.Now().Before(until); time.Sleep(50 * time.Millisecond) {
		found, err := processesWith(profile)
		if err != nil || len(found) == 0 {
			return
		}
		for pid := range found {
			if strings.Contains(processWords(pid), profile) {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
}

// browserLeftovers is what's still running with marker in its words, as "pid words" lines, once nothing is or for at
// most wait.
func browserLeftovers(marker string, wait time.Duration) string {
	for until := time.Now().Add(wait); ; time.Sleep(100 * time.Millisecond) {
		found, err := processesWith(marker)
		if err != nil {
			return "ps: " + err.Error()
		}
		if len(found) == 0 {
			return ""
		}
		if time.Now().After(until) {
			var lines []string
			for pid, words := range found {
				lines = append(lines, strconv.Itoa(pid)+" "+words)
			}
			slices.Sort(lines)
			return strings.Join(lines, "\n")
		}
	}
}

// standInKinds are the stand-in browsers the rows hold: a plain one; a hung one, that ignores SIGTERM; and two that
// start a helper, with the browser's words, that leaves the browser's group: into a session of its own (setsid, as
// Chrome's crashpad handler does with its double fork), or a group of its own (setpgid).
var standInKinds = []string{"plain", "ignores TERM", "setsid helper", "setpgid helper"}

// writeStandInBrowser writes a stand-in browser of a kind into dir: a shell script that writes its pid next to itself,
// then stays until it's killed (for two minutes at most, so a failing row leaves nothing for long). A helper, a perl
// script, forks, leaves the group, writes "pid group" next to the stand-in, and stays as long. Never a real browser.
func writeStandInBrowser(t *testing.T, dir, kind string) string {
	t.Helper()
	path := filepath.Join(dir, "standin-browser")
	script := "#!/bin/sh\n# A stand-in browser for a row (headlesschrome_unix_test.go): " + kind + ".\necho $$ > \"$0.pid\"\n"
	switch kind {
	case "ignores TERM":
		script += "trap '' TERM HUP INT\n"
	case "setsid helper", "setpgid helper":
		perl, err := exec.LookPath("perl")
		if err != nil {
			t.Skip("no perl on this computer, for the stand-in's helper")
		}
		leave := "POSIX::setsid()"
		if kind == "setpgid helper" {
			leave = "setpgrp(0, 0)"
		}
		script += perl + ` -MPOSIX -e 'exit if fork; ` + leave + ` or die; open(my $f, ">", "$ARGV[0].helper.new") or die; ` +
			`print $f "$$ " . getpgrp() . "\n"; close $f; rename("$ARGV[0].helper.new", "$ARGV[0].helper"); sleep 1 for 1 .. 120' -- "$0" "$@"` + "\n"
	}
	script += "i=0\nwhile [ $i -lt 120 ]; do sleep 1; i=$((i+1)); done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// standInPids are the pids of the stand-in at path and, for a kind with a helper, its helper's, once each has written
// it (within 20 s) and runs with marker in its words. A helper must have left the browser's group.
func standInPids(t *testing.T, path, kind, marker string) []int {
	t.Helper()
	pids := []int{standInPid(t, path)}
	if strings.HasSuffix(kind, " helper") {
		var group int
		for until := time.Now().Add(20 * time.Second); len(pids) == 1 && time.Now().Before(until); time.Sleep(50 * time.Millisecond) {
			data, _ := os.ReadFile(path + ".helper")
			if fields := strings.Fields(string(data)); len(fields) == 2 {
				pid, _ := strconv.Atoi(fields[0])
				group, _ = strconv.Atoi(fields[1])
				pids = append(pids, pid)
			}
		}
		if len(pids) == 1 {
			t.Fatalf("the stand-in's helper never started: %s", path)
		}
		if group != pids[1] {
			t.Fatalf("the stand-in's helper (pid %d) is in group %d: it was to lead one of its own", pids[1], group)
		}
	}
	running, _ := processesWith(marker)
	for _, pid := range pids {
		if _, ok := running[pid]; !ok {
			t.Fatalf("pid %d of the stand-in isn't running with %s in its words", pid, marker)
		}
	}
	return pids
}

// standInPid is the pid the stand-in at path wrote, once it has, within 20 s.
func standInPid(t *testing.T, path string) int {
	t.Helper()
	for until := time.Now().Add(20 * time.Second); time.Now().Before(until); time.Sleep(50 * time.Millisecond) {
		data, _ := os.ReadFile(path + ".pid")
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			return pid
		}
	}
	t.Fatalf("the stand-in browser %s never started", path)
	return 0
}

// killStrays kills what this row started, if it's still running after the row failed: only those pids, and each only
// while its words still hold the row's marker.
func killStrays(t *testing.T, marker string, pids []int) {
	t.Helper()
	for _, pid := range pids {
		if pid > 0 && strings.Contains(processWords(pid), marker) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Logf("killed the row's own stand-in, pid %d", pid)
		}
	}
}

// After the test, normal end: the stand-in browser, its watchdog and any helper of its are gone, killed by the
// cleanup: by group, and by pid for a helper that left the group (FXCHROMEKILL1R1 B1).
func TestAStandInBrowserIsGoneOnceItsTestEnds(t *testing.T) {
	for _, kind := range standInKinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			standIn := writeStandInBrowser(t, t.TempDir(), kind)
			var profile string
			var pids []int
			t.Run("holds a browser", func(t *testing.T) {
				b := startHeadlessChrome(t, standIn, "--window-size=1100,700", "http://127.0.0.1:9/")
				profile = b.profile
				pids = standInPids(t, standIn, kind, b.profile)
			})
			if left := browserLeftovers(profile, 5*time.Second); left != "" {
				t.Errorf("the test has ended, and its browser hasn't:\n%s", left)
				killStrays(t, profile, pids)
			}
		})
	}
}

// Its lifetime is bounded: past its deadline, the browser, all its group and any helper of its are killed, with the
// test still running.
func TestAStandInBrowserIsKilledAtItsDeadline(t *testing.T) {
	for _, kind := range standInKinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			standIn := writeStandInBrowser(t, t.TempDir(), kind)
			b := startBrowserUntil(t, standIn, time.Now().Add(2*time.Second), nil)
			pids := standInPids(t, standIn, kind, b.profile)
			select {
			case <-b.ended:
			case <-time.After(15 * time.Second):
				t.Fatalf("the stand-in (pids %v) still runs 13 s past its deadline", pids)
			}
			if left := browserLeftovers(b.profile, 5*time.Second); left != "" {
				t.Errorf("past its deadline, the browser ended (%s) but its group or its helper didn't:\n%s", b.why, left)
				killStrays(t, b.profile, pids)
			}
			if b.why != "killed" {
				t.Errorf("killed with its watchdog, the browser ended as %q", b.why)
			}
		})
	}
	// The margin: a quarter of what's left, never more than 30 s.
	for left, want := range map[time.Duration]time.Duration{10 * time.Minute: 30 * time.Second, 40 * time.Second: 10 * time.Second, 0: 0} {
		if got := browserMargin(left); got != want {
			t.Errorf("with %v left, the margin is %v, want %v", left, got, want)
		}
	}
}

// A browser that ends by itself says so, with its status, while its watchdog stays for the test's cleanup.
func TestAStandInBrowserThatEndsByItselfSaysSo(t *testing.T) {
	quits := filepath.Join(t.TempDir(), "quits")
	if err := os.WriteFile(quits, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	b := startHeadlessChrome(t, quits)
	select {
	case <-b.ended:
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in that quits at once hadn't ended 10 s later")
	}
	if b.why != "status 3" {
		t.Errorf("it ended as %q, want status 3", b.why)
	}
	select {
	case err := <-b.waited:
		t.Fatalf("the watchdog ended with the browser (%v): it must stay until the test ends", err)
	default:
	}
}

// browserHolderVariable: set (mode:stand-in's path), this test program is a row's child: it starts the stand-in
// through startHeadlessChrome, then is killed with SIGKILL by the row ("killed"), panics ("panic"), or overruns its
// -test.timeout ("timeout").
const browserHolderVariable = "SMOLCORP_TEST_BROWSER_HOLDER"

func TestAStandInBrowserHeldForARow(t *testing.T) {
	mode, standIn, _ := strings.Cut(os.Getenv(browserHolderVariable), ":")
	if standIn == "" {
		t.Skip("only as a row's child (TestAStandInBrowserDiesWithTheTestProgramThatStartedIt)")
	}
	startHeadlessChrome(t, standIn)
	standInPid(t, standIn)
	if mode == "panic" {
		// Once the row has seen the stand-in running: the cleanup a panic runs kills it at once.
		for until := time.Now().Add(20 * time.Second); time.Now().Before(until); time.Sleep(50 * time.Millisecond) {
			if _, err := os.Stat(standIn + ".seen"); err == nil {
				break
			}
		}
		panic("a row's child panics here, on purpose")
	}
	time.Sleep(time.Hour)
}

// The watchdog: the test program that started the browser is killed with SIGKILL (no cleanup, no defer), panics, or
// times out (no cleanup either), and within a few seconds the stand-in browser and any helper of its are gone. The
// child is this test program again, holding a stand-in of each kind: a sleep script with the row's own folder in its
// words.
func TestAStandInBrowserDiesWithTheTestProgramThatStartedIt(t *testing.T) {
	for _, mode := range []string{"killed", "panic", "timeout"} {
		for _, kind := range standInKinds {
			t.Run(mode+", "+kind, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				standIn := writeStandInBrowser(t, dir, kind)
				timeout := "2m"
				if mode == "timeout" {
					timeout = "6s"
				}
				child := exec.Command(os.Args[0], "-test.run=^TestAStandInBrowserHeldForARow$", "-test.count=1", "-test.timeout="+timeout)
				// Its temp folders, the browser's profile among them, in the row's: the row's folder marks all it starts.
				child.Env = append(os.Environ(), browserHolderVariable+"="+mode+":"+standIn, "TMPDIR="+dir)
				var said lockedBuffer
				child.Stdout, child.Stderr = &said, &said
				if err := child.Start(); err != nil {
					t.Fatal(err)
				}
				ended := make(chan error, 1)
				go func() { ended <- child.Wait() }()
				pids := standInPids(t, standIn, kind, dir)
				switch mode {
				case "killed":
					child.Process.Kill() // SIGKILL
				case "panic":
					if err := os.WriteFile(standIn+".seen", nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				select {
				case <-ended:
				case <-time.After(30 * time.Second):
					child.Process.Kill()
					<-ended
					t.Fatalf("the child never ended: %s", said.String())
				}
				if left := browserLeftovers(dir, 5*time.Second); left != "" {
					t.Errorf("the test program that started it has ended (%s), and the stand-in browser or its helper hasn't:\n%s\nthe child said: %s",
						child.ProcessState, left, said.String())
					killStrays(t, dir, pids)
				}
				if mode == "timeout" && !strings.Contains(said.String(), "test timed out") {
					t.Errorf("the child didn't time out: %s", said.String())
				}
				if mode == "panic" && !strings.Contains(said.String(), "on purpose") {
					t.Errorf("the child didn't panic: %s", said.String())
				}
			})
		}
	}
}
