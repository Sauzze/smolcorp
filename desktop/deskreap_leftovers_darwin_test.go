//go:build darwin && cgo

package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// FXFIREREAPR3's rows (RT126FIXREVIEW-FIREREAPR1 O1–O3, still open after R2): what a plain hire's launcher that ended
// without taking its sandbox away left running is ended at the next plain hire's start; a stop's words that were
// dropped reach the owner; and the restart's words after a colon are in lower case. The same stand-ins as
// deskreap_darwin_test.go's.

// startedAt is process pid's start time, to the microsecond, as the kernel's process table has it.
func startedAt(t *testing.T, pid int) uint64 {
	t.Helper()
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || int(k.Proc.P_pid) != pid {
		t.Fatalf("process %d's start time: %v", pid, err)
	}
	return uint64(k.Proc.P_starttime.Sec)*1_000_000 + uint64(k.Proc.P_starttime.Usec)
}

// O1: a plain hire whose launcher ended without its takeAway (a crash, a SIGKILL) left its agent's programs running in
// its sandbox, with no row for a fire to find. The next plain hire's start ends them, and takes its folders away, even
// a start refused at once; that of a launcher whose pid another process has taken since too. A plain hire whose
// launcher still runs, another desk and an unsandboxed bystander are left as they are.
func TestTheNextPlainHiresStartEndsWhatAnEndedLaunchersAgentLeftRunning(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	type plainHire struct {
		own, socket string
		agent       standInProcs
	}
	// plain makes plain hire id's folders, its profile and its record of its launcher (pid, started), as its launcher
	// would, and starts its agent's stand-in processes in its sandbox.
	plain := func(id string, pid int, started uint64) plainHire {
		h := plainHire{own: hireFolder(supportDir, id), socket: deskSocketPath(id, 0)}
		if err := os.MkdirAll(h.own, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := makeDeskSocketFolder(h.socket); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { removeDeskSocketFolder(h.socket) })
		if err := os.WriteFile(filepath.Join(h.own, "launcher"), []byte(fmt.Sprintf("%d %d\n", pid, started)), 0o600); err != nil {
			t.Fatal(err)
		}
		ds, err := makeDeskSandbox(deskAt{ID: id, Root: t.TempDir(), Socket: h.socket, Git: runGit})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { keptSandboxes.Lock(); delete(keptSandboxes.of, id); keptSandboxes.Unlock() })
		h.agent = startStandInDesk(t, ds, deskProfileFor(t, ds, h.socket))
		return h
	}
	// A launcher that has ended: a process of this test's, its start time read, then ended and its end read.
	launcher := exec.Command("/bin/sleep", "600")
	if err := launcher.Start(); err != nil {
		t.Fatal(err)
	}
	endedPid, endedStart := launcher.Process.Pid, startedAt(t, launcher.Process.Pid)
	launcher.Process.Kill()
	launcher.Wait()
	self := startedAt(t, os.Getpid())
	crashed := plain("plain-00000000c0a1", endedPid, endedStart)
	reused := plain("plain-00000000c0a2", os.Getpid(), self-1) // its pid is this test's now
	running := plain("plain-00000000c0a3", os.Getpid(), self)
	other, bystander := otherDeskAndBystander(t)

	// The next plain hire's start, refused before it makes anything of its own.
	if _, err := newLauncherSandbox("not/a/whole/path", false, "tab-next", Claude, "/bin/sh"); err == nil {
		t.Fatal("a plain hire in a folder that isn't a whole path wasn't refused")
	}
	for name, h := range map[string]plainHire{"ended": crashed, "ended, its pid another's now": reused} {
		notRunning(t, "after the next plain hire's start, from one whose launcher "+name, h.agent.all()...)
		if _, err := os.Lstat(h.own); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the own folder of a plain hire whose launcher %s wasn't taken away: %v", name, err)
		}
		if _, err := os.Lstat(filepath.Dir(h.socket)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the socket folder of a plain hire whose launcher %s wasn't taken away: %v", name, err)
		}
	}
	for _, pid := range running.agent.all() {
		if !inDesk(t, pid, running.agent.profile) {
			t.Errorf("process %d of a plain hire whose launcher still runs was ended", pid)
		}
	}
	if _, err := os.Lstat(filepath.Join(running.own, "sandbox.sb")); err != nil {
		t.Errorf("the profile of a plain hire whose launcher still runs went: %v", err)
	}
	untouched(t, other, bystander)
}

// O2: a fire that went through, whose last stop found a program it couldn't end (none was left at the fire's reap, but
// one couldn't be shown gone by its stop's), says so in its answer, and the desk's sandbox stays kept. None was named:
// the hire is fired, its row gone from Setup, so the answer says to restart the Mac to be sure, never to fire it again
// (RT126FIXREVIEW-FIREREAPR4 N1). A fire of it sent all the same does look again: once the processes can be listed, it
// finds the sandbox empty and says the hire is already gone.
func TestAFiresLastStopSaysWhatItCouldntEnd(t *testing.T) {
	_, r, hire, _, other, bystander := sandboxedDeskRunning(t)
	r.o.StoreLook = func(context.Context, string, []string) (int, string, error) { return 1, "", nil } // stood in as sound
	list := deskReapSystem.list
	t.Cleanup(func() { deskReapSystem.list = list })
	deskReapSystem.list = func() ([]int, error) {
		if row, ok := registryHire(hire.ID); ok && row.State == "fired" {
			return nil, errors.New("stood in: the processes can't be listed once it's marked fired")
		}
		return list()
	}
	_, said, err := fireEngineHireSaid(hire, 0, r.o, false)
	deskReapSystem.list = list
	if err != nil {
		t.Fatal(err)
	}
	want := "Smolcorp couldn't make sure every program " + calledHire(hire) + " started has ended. Restart your Mac to be sure."
	if !strings.Contains(said, want) || strings.Contains(strings.ToLower(said), "fire it") {
		t.Errorf("the fire said %q, want it to say %q, and not to fire it again", said, want)
	}
	if row := registered(t)[0]; row.State != "fired" {
		t.Errorf("row %+v, want fired", row)
	}
	if _, ok := deskSandboxOf(hire.ID); !ok {
		t.Error("the sandbox of a desk whose stop couldn't show it empty wasn't kept")
	}
	again := hire
	again.State = "fired"
	_, _, err = fireEngineHireSaid(again, 0, r.o, false)
	var refused *hireRefusal
	if !errors.As(err, &refused) || !strings.Contains(refused.Message, calledHire(hire)+" is already gone.") {
		t.Errorf("firing it again said %v, want it to look again and find it already gone", err)
	}
	untouched(t, other, bystander)
}

// O2 (FXFIREREAPR4): where only a restart of the Mac is left to suggest (a fire that went through, a plain hire's
// window), its words fit what was left: "it" for one program, "them" for more, and, with none named, a restart "to be
// sure", for a plain hire after saying Smolcorp looks again at the next one's start (RT126FIXREVIEW-FIREREAPR4 O2).
func TestTheRestartWordsFitWhatTheStopLeft(t *testing.T) {
	h := Hire{Name: "Janitor"}
	for _, c := range []struct {
		left        []int
		fire, plain string
	}{
		{nil, ". Restart your Mac to be sure.",
			". Smolcorp looks again when you next start a plain hire; if this keeps happening, restart your Mac to be sure."},
		{[]int{os.Getpid()}, ". Restart your Mac to end it.", ". Restart your Mac to end it."},
		{[]int{os.Getpid(), os.Getppid()}, ". Restart your Mac to end them.", ". Restart your Mac to end them."},
		{[]int{os.Getpid(), os.Getppid(), 1}, ". Restart your Mac to end them.", ". Restart your Mac to end them."},
	} {
		why := reapLeftWords(h, c.left)
		if got := firedStopNote(why); !strings.HasSuffix(got, c.fire) || strings.Count(got, "Restart your Mac")+strings.Count(got, "restart your Mac") != 1 {
			t.Errorf("%d left: the fire says %q, want it to end %q", len(c.left), got, c.fire)
		}
		if got := launcherKeptWords(why); got != "smolcorp: "+why+c.plain {
			t.Errorf("%d left: a plain hire's window says %q, want %q", len(c.left), got, "smolcorp: "+why+c.plain)
		}
	}
}

// N1 (RT126FIXREVIEW-FIREREAPR4): a fire that went through is marked fired, and its row leaves Setup's team list, so
// what its last stop says never asks the owner to fire it again, whatever was left; a refused fire keeps its row, and
// its words still do.
func TestAFireThatWentThroughNeverAsksToBeFiredAgain(t *testing.T) {
	h := Hire{Name: "Janitor"}
	for _, left := range [][]int{nil, {os.Getpid()}, {os.Getpid(), os.Getppid()}} {
		why := reapLeftWords(h, left)
		if note := firedStopNote(why); strings.Contains(strings.ToLower(note), "fire it") || !strings.Contains(note, "Restart your Mac") {
			t.Errorf("%d left: a fire that went through says %q, want a restart of the Mac and no fire again", len(left), note)
		}
		if refusal := why + firedReapRefusal; !strings.Contains(refusal, "Fire it again") {
			t.Errorf("%d left: a refused fire says %q, want it to say to fire it again", len(left), refusal)
		}
	}
}

// O2 and O3: the restart, stopped and refused at its start, whose stop couldn't end what that start left running in
// its sandbox, says so, after its colon in lower case.
func TestARestartsStopThatCouldntEndWhatItsStartLeftSaysSo(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	d, _, old, _ := oldDeskRig(t)
	coreGitInDeskSandbox(t)
	var desk standInProcs
	signal := deskReapSystem.signal
	t.Cleanup(func() { deskReapSystem.signal = signal })
	deskReapSystem.signal = func(run reapRun, sig syscall.Signal) error {
		if run.pid == desk.setsid {
			return nil // stood in as sent, and not: it can't be ended
		}
		return signal(run, sig)
	}
	starter := d.engine.Sandbox
	d.engine.Sandbox = func(s *deskSandbox) tmuxStarter {
		starter(s)
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			desk = startStandInDesk(t, s, deskProfileFor(t, s, args[1]))
			return "", 0, errors.New(takenPlace)
		}
	}
	status, why := d.restartInSandbox(old.ID)
	if desk.setsid == 0 {
		t.Fatalf("the stand-in desk didn't start: %d %v", status, why)
	}
	program := reapName(desk.setsid)
	if program == "" {
		t.Fatalf("macOS didn't name the desk's process %d", desk.setsid)
	}
	want := "Janitor was stopped, and couldn't start again: a program Janitor started is still running in its sandbox " +
		"and couldn't be ended (" + program + ", process " + strconv.Itoa(desk.setsid) + "). Its worktree and branch are as it left them"
	if words, _ := why.(string); status != http.StatusInternalServerError || !strings.Contains(words, want) {
		t.Errorf("%d %v, want %q", status, why, want)
	}
}
