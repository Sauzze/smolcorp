//go:build darwin && cgo

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// FXFIREREAPR2's rows (RT126FIXREVIEW-FIREREAPR1 N1): an ended process whose parent never reads its end (a zombie) is
// listed among the owner's processes for as long as its parent runs. One of another app's mustn't make every reap
// refuse; one in a desk's sandbox, its parent there too, mustn't let the desk be called empty while that parent runs.

// zombieParent forks a child that ends at once, writes its own pid and its child's, and never reads the child's end.
const zombieParent = `$| = 1;
my $child = fork;
exit 1 unless defined $child;
exit 0 if $child == 0;
print "$$ $child\n";
sleep 600;`

// isZombie is whether process pid has ended, and waits for its parent to read its end.
func isZombie(pid int) bool {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && int(k.Proc.P_pid) == pid && k.Proc.P_stat == 5 // SZOMB, <sys/proc.h>
}

// standInZombie starts zombieParent, run by sandboxExec first when it's given (under a desk's profile), and waits for
// its child to be a zombie. Both are registered for the reap; the parent is ended at the end, and launchd reads its
// child's end then.
func standInZombie(t *testing.T, sandboxExec ...string) (parent, zombie int) {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "pids"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	args := append(sandboxExec, "/usr/bin/perl", "-e", zombieParent)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	for i := 0; ; i++ {
		text, _ := os.ReadFile(out.Name())
		if pids := strings.Fields(string(text)); strings.HasSuffix(string(text), "\n") && len(pids) == 2 {
			parent, _ = strconv.Atoi(pids[0])
			zombie, _ = strconv.Atoi(pids[1])
			if isZombie(zombie) {
				break
			}
		}
		if i == 250 {
			t.Fatalf("no zombie: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if parent != cmd.Process.Pid {
		t.Fatalf("another process wrote the pids: %d, not %d", parent, cmd.Process.Pid)
	}
	reapLooksAt(t, parent, zombie)
	everyone, err := reapList()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(everyone, zombie) {
		t.Fatalf("the zombie %d isn't listed: no row of it to make", zombie)
	}
	return parent, zombie
}

// A zombie of another app's (an unsandboxed parent of this test's that never reads its child's end), beside an empty
// desk with its real profile: the desk's reap says none is left, at once, and leaves both as they are.
func TestAnotherAppsZombieDoesntStopAnEmptyDesksReap(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	s := standInDesk(t)
	socket := deskSocketPath(s.at.ID, 0)
	deskProfileFor(t, s, socket)
	parent, zombie := standInZombie(t)
	h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket}
	began := time.Now()
	why := deskReap(h)
	took := time.Since(began)
	t.Logf("the reap of an empty desk beside the zombie %d: %q after %v", zombie, why, took)
	if why != "" {
		t.Errorf("the reap of an empty desk refused: %q", why)
	}
	if took >= deskReapGrace {
		t.Errorf("the reap took %v: not at once", took)
	}
	if err := syscall.Kill(parent, 0); err != nil {
		t.Errorf("the zombie's parent %d was ended: %v", parent, err)
	}
	if !isZombie(zombie) {
		t.Errorf("the zombie %d isn't one any more", zombie)
	}
}

// zombieForker waits for a line, then forks a child that stays (setsid), writes its pid, and ends, its own end left for
// its parent to read.
const zombieForker = `$| = 1;
<STDIN>;
my $child = fork;
exit 1 unless defined $child;
if ($child == 0) { POSIX::setsid(); sleep 600; exit 0 }
print "$child\n";
exit 0;`

// A zombie in a desk's sandbox whose parent runs there too: the desk isn't known stopped while the parent runs, and
// its reap finds the parent, ends it, and says none is left once the zombie's end has been read.
//
// Then R2's rule itself (RT126FIXREVIEW-FIREREAPR2 O3): a desk's zombie that's new at a pass, its parent outside the
// desk (the core, here this test), isn't answered for. A desk's process started after a pass that found none forks a
// child that stays after the next pass's list, and ends before its look: that pass finds only the zombie, and
// mustn't agree with the one before. Taken as answered (any zombie answered, or a zombie taken as running whole
// outside), the reap would say none is left while the child runs.
func TestADesksZombieDoesntLetItBeCalledEmptyWhileItsParentRuns(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	s := standInDesk(t)
	socket := deskSocketPath(s.at.ID, 0)
	profile := deskProfileFor(t, s, socket)
	parent, zombie := standInZombie(t, "/usr/bin/sandbox-exec", "-f", profile)
	if !inDesk(t, parent, profile) {
		t.Fatalf("the zombie's parent %d isn't in the desk's sandbox", parent)
	}
	h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket, TmuxServer: 99999999} // no process has that pid
	if deskStoppedNow(h) {
		t.Error("known stopped while the zombie's parent runs in its sandbox")
	}
	if !inDesk(t, parent, profile) || !isZombie(zombie) {
		t.Fatal("the look ended the zombie's parent")
	}
	if why := deskReap(h); why != "" {
		t.Fatalf("the reap refused: %q", why)
	}
	// The parent's end is read by this test, the zombie's by launchd once its parent has ended.
	for i := 0; ; i++ {
		everyone, err := reapList()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(everyone, parent) && !slices.Contains(everyone, zombie) {
			break
		}
		if i == 100 {
			t.Fatalf("after the reap, the zombie's parent %d (ended: %t) or the zombie %d is still listed", parent,
				isZombie(parent), zombie)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !deskStoppedNow(h) {
		t.Error("not known stopped once nothing runs in its sandbox")
	}

	// R2's rule: a desk's process whose parent never reads its end, started (as the list has it) after a pass that
	// found none.
	sandboxExec := "/usr/bin/sandbox-exec"
	forker := exec.Command(sandboxExec, "-f", profile, "/usr/bin/perl", "-MPOSIX", "-e", zombieForker)
	out, err := os.Create(filepath.Join(t.TempDir(), "child"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	forker.Stdout = out
	tell, err := forker.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := forker.Start(); err != nil {
		t.Fatal(err)
	}
	child := 0
	t.Cleanup(func() {
		forker.Process.Kill() // its end not read yet, so its pid is still its own; before it reads a line, it forks nothing
		for _, pid := range []int{forker.Process.Pid, child} {
			if pid == 0 {
				continue
			}
			if run, verdict := reapLook(pid, reapAtOf(profile)); verdict == reapIn {
				reapSignal(run, syscall.SIGKILL)
			}
		}
		tell.Close()
		forker.Wait() // its end, read only now
	})
	for i := 0; !inDesk(t, forker.Process.Pid, profile); i++ { // once sandbox-exec has applied the profile
		if i == 250 {
			t.Fatalf("the forker %d isn't in the desk's sandbox", forker.Process.Pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	list := deskReapSystem.list
	t.Cleanup(func() { deskReapSystem.list = list })
	passes := 0
	deskReapSystem.list = func() ([]int, error) {
		if passes++; passes != 2 {
			return list()
		}
		// The second pass: the forker is in its list, a zombie by its look; its child, forked after the list, isn't.
		if _, err := tell.Write([]byte("fork\n")); err != nil {
			return nil, err
		}
		for i := 0; ; i++ {
			text, _ := os.ReadFile(out.Name())
			if pid, err := strconv.Atoi(strings.TrimSpace(string(text))); err == nil && strings.HasSuffix(string(text), "\n") &&
				isZombie(forker.Process.Pid) {
				child = pid
				break
			}
			if i == 250 {
				return nil, errors.New("the forker didn't fork")
			}
			time.Sleep(20 * time.Millisecond)
		}
		reapLooksAt(t, forker.Process.Pid)
		pids, err := list()
		reapLooksAt(t, child)
		return pids, err
	}
	why := deskReap(h)
	deskReapSystem.list = list
	if child == 0 {
		t.Fatalf("the reap made %d passes, and the forker never forked: %q", passes, why)
	}
	if why != "" {
		t.Errorf("the reap refused: %q", why)
	}
	notRunning(t, "after the reap said none was left of a desk whose zombie, new at a pass, had made it", child)
	if !isZombie(forker.Process.Pid) {
		t.Errorf("the forker %d isn't a zombie any more: its end was read by someone else", forker.Process.Pid)
	}
}
