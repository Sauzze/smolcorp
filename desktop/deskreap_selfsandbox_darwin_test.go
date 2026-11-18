//go:build darwin && cgo

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// FXFIREREAPR4's row (RT126FIXREVIEW-FIREREAPR3 B1): a process that sandboxes itself while a reap looks at it (a
// browser's renderer, an App Sandbox app, each at its start), with a profile of its own that denies Smolcorp's support
// folder, is never taken for the desk's, and the reap never signals it.

// selfSandboxingBystanders forks bystanders until the file argv[3] is there (argv[2] of them at most), one every 2 ms.
// Each waits 0–10 ms, sandboxes itself with the profile argv[1], lives 30 ms more and exits 0; 3 when it couldn't
// sandbox itself. Once the last has ended, it writes how each ended in argv[4]: "exit 0", "exit 3", "signal 15"…
const selfSandboxingBystanders = `import ctypes, os, random, sys, time
lib = ctypes.CDLL("/usr/lib/libSystem.B.dylib")
lib.sandbox_init.argtypes = [ctypes.c_char_p, ctypes.c_uint64, ctypes.POINTER(ctypes.c_char_p)]
profile, most, stop, out = sys.argv[1].encode(), int(sys.argv[2]), sys.argv[3], sys.argv[4]
ends, born = {}, 0
def ended(status):
    what = "signal %d" % os.WTERMSIG(status) if os.WIFSIGNALED(status) else "exit %d" % os.WEXITSTATUS(status)
    ends[what] = ends.get(what, 0) + 1
while born < most and not os.path.exists(stop):
    wait = random.random() * 0.01
    if os.fork() == 0:
        time.sleep(wait)
        err = ctypes.c_char_p()
        if lib.sandbox_init(profile, 0, ctypes.byref(err)) != 0:
            os._exit(3)
        time.sleep(0.03)
        os._exit(0)
    born += 1
    time.sleep(0.002)
    while True:
        pid, status = os.waitpid(-1, os.WNOHANG)
        if pid == 0:
            break
        ended(status)
while True:
    try:
        pid, status = os.waitpid(-1, 0)
    except ChildProcessError:
        break
    ended(status)
with open(out + ".part", "w") as f:
    f.write("born %d\n" % born)
    for what, n in sorted(ends.items()):
        f.write("%s %d\n" % (what, n))
os.rename(out + ".part", out)
`

// The reap runs its whole time (a desk process stood in as one that can't be ended), three times over, beside
// bystanders of this test's that sandbox themselves. The list is the kernel's, narrowed to this test's two process
// groups: the desk process's, and the bystanders'. No signal may be sent to any bystander, and every one ends by
// itself. Before FXFIREREAPR4, the allowed read was asked first: a bystander that sandboxed itself after it, and before
// the three denies, was "in", and SIGKILLed.
func TestTheReapNeverSignalsAProcessThatSandboxesItselfWhileItLooks(t *testing.T) {
	needSandboxExec(t)
	python := deskPython(t)
	if python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("no python3 to sandbox the bystanders with")
		}
		t.Skip("no python3 here to sandbox the bystanders with")
	}
	shortReap(t)
	s := standInDesk(t)
	socket := deskSocketPath(s.at.ID, 0)
	profile := deskProfileFor(t, s, socket)
	support := physical(supportDir)

	// The desk's one process: in its sandbox, in a group of its own. It's never signalled (below), so the desk can't be
	// shown empty and the reap runs to its time limit, looking again and again.
	stuck := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sleep", "60")
	stuck.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stuck.Start(); err != nil {
		t.Fatal(err)
	}
	stuckDone := make(chan struct{})
	go func() { stuck.Wait(); close(stuckDone) }()
	t.Cleanup(func() { stuck.Process.Kill(); <-stuckDone }) // this test's child, its end not read yet: its pid is its own
	for i := 0; !inDesk(t, stuck.Process.Pid, profile); i++ {
		if i == 250 {
			t.Fatal("the desk's process isn't in its sandbox")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The bystanders, in a group of their own; their profile has nothing to do with any desk's.
	dir := t.TempDir()
	stop, ends := filepath.Join(dir, "stop"), filepath.Join(dir, "ends")
	own := `(version 1)(allow default)(deny file-read* file-write* (subpath ` + strconv.Quote(support) + `))`
	spawner := exec.Command(python, "-I", "-c", selfSandboxingBystanders, own, "20000", stop, ends)
	spawner.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := spawner.Start(); err != nil {
		t.Fatal(err)
	}
	spawnerDone := make(chan struct{})
	go func() { spawner.Wait(); close(spawnerDone) }()
	t.Cleanup(func() {
		os.WriteFile(stop, nil, 0o600)
		select {
		case <-spawnerDone:
		case <-time.After(10 * time.Second):
			syscall.Kill(-spawner.Process.Pid, syscall.SIGKILL) // this test's own group, its leader's end not read yet
			<-spawnerDone
		}
	})

	list, look, signal := deskReapSystem.list, deskReapSystem.look, deskReapSystem.signal
	t.Cleanup(func() { deskReapSystem.list, deskReapSystem.look, deskReapSystem.signal = list, look, signal })
	groups := []int{stuck.Process.Pid, spawner.Process.Pid}
	deskReapSystem.list = func() ([]int, error) {
		var pids []int
		for _, group := range groups {
			procs, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", group)
			if err != nil {
				return nil, err
			}
			for _, k := range procs {
				pids = append(pids, int(k.Proc.P_pid))
			}
		}
		return pids, nil
	}
	var mu sync.Mutex
	looked := map[reapVerdict]int{}
	var sent []string
	deskReapSystem.look = func(pid int, at reapAt) (reapRun, reapVerdict) {
		run, verdict := look(pid, at)
		if pid != stuck.Process.Pid {
			mu.Lock()
			looked[verdict]++
			mu.Unlock()
		}
		return run, verdict
	}
	deskReapSystem.signal = func(run reapRun, sig syscall.Signal) error {
		if run.pid == stuck.Process.Pid {
			return nil // as if sent, and it went on running
		}
		err := signal(run, sig) // a bystander: this test's own, and that one run only
		mu.Lock()
		sent = append(sent, fmt.Sprintf("%v to process %d (%v)", sig, run.pid, err))
		mu.Unlock()
		return err
	}

	time.Sleep(200 * time.Millisecond) // the bystanders under way
	for round := range 3 {
		h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket}
		if why := deskReap(h); !strings.Contains(why, fmt.Sprintf("process %d", stuck.Process.Pid)) {
			t.Fatalf("reap %d said %q, want it to name the desk's process %d it couldn't end", round, why, stuck.Process.Pid)
		}
	}
	os.WriteFile(stop, nil, 0o600)
	<-spawnerDone
	text, err := os.ReadFile(ends)
	if err != nil {
		t.Fatalf("the bystanders didn't say how they ended: %v", err)
	}
	t.Logf("bystanders' looks: %d not in, %d gone, %d zombies, %d in; their ends: %s", looked[reapNotIn], looked[reapGone],
		looked[reapZombie], looked[reapIn]+looked[reapInUnreached], strings.ReplaceAll(strings.TrimSpace(string(text)), "\n", ", "))
	mu.Lock()
	defer mu.Unlock()
	if len(sent) > 0 {
		t.Errorf("the reap signalled %d bystanders that aren't in the desk's sandbox: %s…", len(sent), strings.Join(sent[:min(len(sent), 3)], "; "))
	}
	if n := looked[reapIn] + looked[reapInUnreached]; n > 0 {
		t.Errorf("%d looks at bystanders found them in the desk's sandbox", n)
	}
	tally := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(text)), "\n") {
		if at := strings.LastIndex(line, " "); at > 0 {
			tally[line[:at]], _ = strconv.Atoi(line[at+1:])
		}
	}
	if tally["exit 0"] != tally["born"] || tally["born"] < 500 {
		t.Errorf("not every bystander sandboxed itself and ended by itself: %v", tally)
	}
	if seen := looked[reapNotIn] + looked[reapGone]; seen < 1000 {
		t.Errorf("the reap looked at bystanders only %d times, too few to show anything", seen)
	}
}
