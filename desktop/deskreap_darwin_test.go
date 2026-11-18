//go:build darwin && cgo

package desktop

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXFIREREAP's rows (RT126SWEEP2 P3 B1): a fire, and Take over's stop, end every program a sandboxed desk started,
// setsid and nohup ones too, and see them ended before anything is checked or merged; an unsandboxed bystander and
// another desk's processes are never touched; a desk that can't be shown empty refuses the fire, and nothing is
// checked, merged or removed. The desks are stand-ins: real profiles from their Policy (sandbox.PrepareProfile) under
// the real sandbox-exec, whose processes this test starts.

// reapRegistered is every process a test of this program registered: the only ones its reap ever looks at. The real
// list (every process of the owner's account) is narrowed to them, so no test looks at a process it didn't start.
// files are where a test's stand-ins that keep moving to new pids write each pid they take, before the one before it
// ends: read after the real list, so a pid it holds is registered by then (pidsWrittenWhole).
var reapRegistered = struct {
	sync.Mutex
	pids  map[int]bool
	files map[string]bool
	given map[string]int // the fork in each file whose child's pid was waited for in vain
}{pids: map[int]bool{}, files: map[string]bool{}, given: map[string]int{}}

func init() {
	everyone := deskReapSystem.list
	deskReapSystem.list = func() ([]int, error) {
		all, err := everyone()
		if err != nil {
			return nil, err
		}
		reapRegistered.Lock()
		defer reapRegistered.Unlock()
		for file := range reapRegistered.files {
			for _, pid := range pidsWrittenWhole(file) {
				reapRegistered.pids[pid] = true
			}
		}
		return slices.DeleteFunc(all, func(pid int) bool { return !reapRegistered.pids[pid] }), nil
	}
}

// pidsWrittenWhole is pidsWrittenIn(file) once no fork written there is still without its child's pid. A stand-in that
// forks writes "fork" first, then its child's pid as fork gives it, and the child its own: so a child running at a list
// taken before this is read is in it, even when its parent was ended before writing it. A fork whose child's pid
// doesn't come within 200 ms (its parent ended before it forked) isn't waited for again. With reapRegistered held.
func pidsWrittenWhole(file string) []int {
	for deadline := time.Now().Add(200 * time.Millisecond); ; time.Sleep(time.Millisecond) {
		text, _ := os.ReadFile(file)
		lines := strings.Split(string(text), "\n")
		lines = lines[:len(lines)-1] // whole lines only
		fork := -1
		for i, line := range lines {
			if line == "fork" {
				fork = i
			}
		}
		pending := fork >= 0 && !slices.ContainsFunc(lines[fork+1:], func(line string) bool { _, err := strconv.Atoi(line); return err == nil })
		if !pending || reapRegistered.given[file] == fork {
			return pidsWrittenIn(file)
		}
		if time.Now().After(deadline) {
			reapRegistered.given[file] = fork
			return pidsWrittenIn(file)
		}
	}
}

// pidsWrittenIn is every pid written whole in file, one a line.
func pidsWrittenIn(file string) []int {
	text, _ := os.ReadFile(file)
	var pids []int
	for _, line := range strings.SplitAfter(string(text), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSuffix(line, "\n")); err == nil && strings.HasSuffix(line, "\n") && pid > 1 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// reapLooksAt registers pids for the reap until the test ends.
func reapLooksAt(t *testing.T, pids ...int) {
	t.Helper()
	reapRegistered.Lock()
	for _, pid := range pids {
		reapRegistered.pids[pid] = true
	}
	reapRegistered.Unlock()
	t.Cleanup(func() {
		reapRegistered.Lock()
		for _, pid := range pids {
			delete(reapRegistered.pids, pid)
		}
		reapRegistered.Unlock()
	})
}

// shortReap makes the reap's waits short for the test.
func shortReap(t *testing.T) {
	t.Helper()
	grace, within, every := deskReapGrace, deskReapWithin, deskReapEvery
	deskReapGrace, deskReapWithin, deskReapEvery = 300*time.Millisecond, 1500*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { deskReapGrace, deskReapWithin, deskReapEvery = grace, within, every })
}

// standInProcs are a stand-in desk's processes: its pane, and the two it left running in the background.
type standInProcs struct {
	id, profile         string
	pane, setsid, nohup int
}

// all is every pid of them.
func (p standInProcs) all() []int { return []int{p.pane, p.setsid, p.nohup} }

// deskProfileFor writes desk s's real profile where its start writes it, <support>/hires/<id>/sandbox.sb.
func deskProfileFor(t *testing.T, s *deskSandbox, socket string) string {
	t.Helper()
	profile := filepath.Join(hireFolder(supportDir, s.at.ID), "sandbox.sb")
	if err := os.MkdirAll(filepath.Dir(profile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareProfile(profile, deskServices(s.places, socket)); err != nil {
		t.Fatal(err)
	}
	return profile
}

// startStandInDesk starts desk s's stand-in processes in its sandbox, under its profile: a pane, which leaves a setsid
// child and a nohup child running in the background as an agent's dev server would. Each writes its pid in the desk's
// own folder (its .md files are the desk's to write). All three are registered for the reap; whatever of them is still
// in the desk's sandbox at the end is ended then, by its run, never by a pid alone.
func startStandInDesk(t *testing.T, s *deskSandbox, profile string) standInProcs {
	t.Helper()
	own := physical(hireFolder(supportDir, s.at.ID))
	script := `echo $$ > "$1/pane.md"
/usr/bin/perl -MPOSIX -e 'exit if fork; POSIX::setsid(); open(F, ">", $ARGV[0]) or exit 1; print F "$$\n"; close F; sleep 600' "$1/setsid.md" &
nohup /bin/sh -c 'echo $$ > "$1"; exec /bin/sleep 600' sh "$1/nohup.md" >/dev/null 2>&1 &
exec /bin/sleep 600`
	cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", script, "sh", own)
	cmd.Dir = s.places.Root
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	p := standInProcs{id: s.at.ID, profile: profile, pane: cmd.Process.Pid}
	for name, pid := range map[string]*int{"setsid": &p.setsid, "nohup": &p.nohup} {
		*pid = pidWritten(t, filepath.Join(own, name+".md"))
	}
	if got := pidWritten(t, filepath.Join(own, "pane.md")); got != p.pane {
		t.Fatalf("the pane wrote %d, not %d", got, p.pane)
	}
	reapLooksAt(t, p.all()...)
	// A stop that took its profile away (with its folder) leaves the kernel's check nothing to answer by: it's put back
	// first, where it was, so what's still in its sandbox is found, even after a failing row.
	text, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := os.Lstat(profile); errors.Is(err, os.ErrNotExist) && os.MkdirAll(filepath.Dir(profile), 0o700) == nil {
			_ = os.WriteFile(profile, text, 0o600)
		}
		endWhatsStillIn(p)
	})
	return p
}

// pidWritten waits for a stand-in to write its pid in file.
func pidWritten(t *testing.T, file string) int {
	t.Helper()
	for range 250 {
		if text, err := os.ReadFile(file); err == nil && strings.HasSuffix(string(text), "\n") {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(text))); err == nil && pid > 1 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no pid in %s", file)
	return 0
}

// endWhatsStillIn ends, by its run, every process of p still in its desk's sandbox: a pid another process has taken
// since is never signalled.
func endWhatsStillIn(p standInProcs) {
	for _, pid := range p.all() {
		if run, verdict := reapLook(pid, reapAtOf(p.profile)); verdict == reapIn {
			reapSignal(run, syscall.SIGKILL)
		}
	}
}

// reapAtOf is what a look asks about the desk whose profile is given: its hires folder, and its mark when it has one.
func reapAtOf(profile string) reapAt {
	folder := filepath.Dir(physical(profile))
	at := reapAt{profile: physical(profile), hires: filepath.Dir(folder)}
	if mark, err := reapMarkIn(folder); err == nil {
		at.mark = filepath.Join(folder, sandbox.ReapMarkFolder, mark)
	}
	return at
}

// inDesk is whether process pid is still among the owner's processes, in the sandbox whose profile is given.
func inDesk(t *testing.T, pid int, profile string) bool {
	t.Helper()
	pids, err := reapList()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pids, pid) {
		return false // ended, its end read by its parent
	}
	_, verdict := reapLook(pid, reapAtOf(profile))
	// Ended (a zombie, whose end its parent hasn't read yet, is still listed), or not in it, otherwise.
	return verdict == reapIn || verdict == reapInUnreached || verdict == reapUnknown
}

// otherDeskAndBystander starts another desk's stand-in processes (desk zz99, its own sandbox) and an unsandboxed
// bystander, all registered for the reap: none may be touched.
func otherDeskAndBystander(t *testing.T) (other standInProcs, bystander int) {
	t.Helper()
	if err := os.MkdirAll(hireFolder(supportDir, "zz99"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := makeDeskSandbox(deskAt{ID: "zz99", Root: t.TempDir(), Socket: deskSocketPath("zz99", 0), Git: runGit})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keptSandboxes.Lock(); delete(keptSandboxes.of, "zz99"); keptSandboxes.Unlock() })
	other = startStandInDesk(t, s, deskProfileFor(t, s, deskSocketPath("zz99", 0)))
	cmd := exec.Command("/bin/sleep", "600")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	reapLooksAt(t, cmd.Process.Pid)
	return other, cmd.Process.Pid
}

// sandboxedDeskRunning is a sandboxed engine hire (sandboxedForHandBack), its window idle, with its real profile and
// its stand-in processes running in its sandbox; another desk's and a bystander beside it.
func sandboxedDeskRunning(t *testing.T) (d *Deck, r *engineRig, hire Hire, desk, other standInProcs, bystander int) {
	t.Helper()
	needSandboxExec(t)
	shortReap(t)
	d, r, hire, _ = sandboxedForHandBack(t)
	// With its profile there, the core's git on its worktree runs in its sandbox (SA-X6): this test program is its
	// `smolcorp sandbox enter`, as sandboxedHire has it.
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	was := deskRunSmolcorp
	deskRunSmolcorp = func() string { return physical(program) }
	t.Cleanup(func() { deskRunSmolcorp = was })
	t.Setenv(deskRunStandInVariable, "1")
	t.Setenv("HOME", homeDir)
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux
	s, ok := deskSandboxOf(hire.ID)
	if !ok {
		t.Fatal("no sandbox kept")
	}
	desk = startStandInDesk(t, s, deskProfileFor(t, s, hire.TmuxSocket))
	other, bystander = otherDeskAndBystander(t)
	// The owner's account's list finds every one of them, the setsid child too: the reap looks at no fewer.
	everyone, err := reapList()
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range append(append(desk.all(), other.all()...), bystander) {
		if !slices.Contains(everyone, pid) {
			t.Fatalf("process %d isn't among the owner's processes: %v", pid, everyone)
		}
	}
	for _, pid := range desk.all() {
		if !inDesk(t, pid, desk.profile) || inDesk(t, pid, other.profile) {
			t.Fatalf("process %d isn't taken for its own desk's alone", pid)
		}
	}
	return d, r, hire, desk, other, bystander
}

// untouched fails t unless the other desk's processes still run in their sandbox, and the bystander still runs.
func untouched(t *testing.T, other standInProcs, bystander int) {
	t.Helper()
	for _, pid := range other.all() {
		if !inDesk(t, pid, other.profile) {
			t.Errorf("another desk's process %d was ended", pid)
		}
	}
	if err := syscall.Kill(bystander, 0); err != nil {
		t.Errorf("the unsandboxed bystander %d was ended: %v", bystander, err)
	}
}

// Rows (i) and (ii): the fire ends the desk's setsid and nohup children, and its pane, before it checks the repo's
// objects; another desk's processes and an unsandboxed bystander are left running.
func TestAFireEndsEveryProgramItsDeskStartedBeforeItChecksTheObjects(t *testing.T) {
	_, r, hire, desk, other, bystander := sandboxedDeskRunning(t)
	looked := false
	r.o.StoreLook = func(context.Context, string, []string) (int, string, error) {
		looked = true
		for _, pid := range desk.all() {
			if inDesk(t, pid, desk.profile) {
				t.Errorf("the desk's process %d still runs in its sandbox as the fire checks the objects", pid)
			}
		}
		return 1, "", nil // stood in as sound
	}
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if !looked {
		t.Fatal("the fire didn't check the objects")
	}
	if row := registered(t)[0]; row.State != "fired" {
		t.Errorf("row %+v, want fired", row)
	}
	untouched(t, other, bystander)
}

// Take over's stop (and so the restart's, which stops a desk the same way) ends them too, and leaves the others.
func TestTakeOversStopEndsEveryProgramItsDeskStarted(t *testing.T) {
	d, _, hire, desk, other, bystander := sandboxedDeskRunning(t)
	if err := d.t3StopInTmux(registered(t)[0]); err != nil {
		t.Fatal(err)
	}
	for _, pid := range desk.all() {
		if inDesk(t, pid, desk.profile) {
			t.Errorf("the desk's process %d still runs in its sandbox after Take over's stop", pid)
		}
	}
	if _, ok := deskSandboxOf(hire.ID); ok {
		t.Error("a stopped desk's sandbox is still kept")
	}
	untouched(t, other, bystander)
}

// stuckChild makes the desk's setsid child one that can't be ended: no signal reaches it.
func stuckChild(t *testing.T, desk standInProcs) {
	t.Helper()
	signal := deskReapSystem.signal
	t.Cleanup(func() { deskReapSystem.signal = signal })
	deskReapSystem.signal = func(run reapRun, sig syscall.Signal) error {
		if run.pid == desk.setsid {
			return nil // stood in as sent, and not
		}
		return signal(run, sig)
	}
}

// Row (iii): a desk process that can't be ended refuses the fire, in plain words: the objects aren't checked, nothing
// is merged, the worktree and branch stay, and the row isn't marked fired.
func TestAFireWhoseDeskCantBeShownEmptyRefusesAndChecksMergesAndRemovesNothing(t *testing.T) {
	_, r, hire, desk, other, bystander := sandboxedDeskRunning(t)
	stuckChild(t, desk)
	r.o.StoreLook = func(context.Context, string, []string) (int, string, error) {
		t.Error("the fire checked the objects of a desk it couldn't show empty")
		return 1, "", nil
	}
	_, err := fireEngineHire(hire, 0, r.o, false)
	var refused *hireRefusal
	program := reapName(desk.setsid)
	if program == "" {
		t.Fatalf("macOS didn't name the desk's process %d", desk.setsid)
	}
	want := "A program " + calledHire(hire) + " started is still running in its sandbox and couldn't be ended (" + program +
		", process " + strconv.Itoa(desk.setsid) + "), so nothing was checked, merged or removed. Fire it again, and if this " +
		"keeps happening, restart your Mac and then fire it."
	if !errors.As(err, &refused) || refused.Status != http.StatusConflict || refused.Message != want {
		t.Fatalf("%v, want %q", err, want)
	}
	if _, err := os.Stat(hire.Worktree); err != nil || !strings.Contains(branches(t, r.repo), hire.Branch) {
		t.Errorf("the worktree (%v) or the branch (%q) went", err, branches(t, r.repo))
	}
	if row := registered(t)[0]; row.State == "fired" {
		t.Errorf("row %+v marked fired", row)
	}
	if !inDesk(t, desk.setsid, desk.profile) {
		t.Error("the stand-in for a process that can't be ended was ended")
	}
	for _, pid := range []int{desk.pane, desk.nohup} {
		if inDesk(t, pid, desk.profile) {
			t.Errorf("the desk's process %d, which could be ended, still runs", pid)
		}
	}
	untouched(t, other, bystander)
}

// The kill code reaches only the run it looked at: once that process has run another program, its old run names
// nothing, and the process is left as it is. A bystander is never in a desk's sandbox, so no run of it is ever taken.
func TestTheReapSignalsOnlyTheRunItLookedAt(t *testing.T) {
	needSandboxExec(t)
	s := standInDesk(t)
	profile := deskProfileFor(t, s, deskSocketPath(s.at.ID, 0))
	own := physical(hireFolder(supportDir, s.at.ID))
	ready := filepath.Join(s.places.Root, "go")
	cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c",
		`echo $$ > "$2"; while [ ! -e "$1" ]; do sleep 0.02; done; exec /bin/sleep 600`, "sh", ready, filepath.Join(own, "pane.md"))
	cmd.Dir = s.places.Root
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	// Once it says so, it runs in its sandbox (sandbox-exec has entered it and run the shell).
	if pidWritten(t, filepath.Join(own, "pane.md")) != cmd.Process.Pid {
		t.Fatal("another process wrote the pane's pid")
	}
	run, verdict := reapLook(cmd.Process.Pid, reapAtOf(profile))
	if verdict != reapIn {
		t.Fatalf("the desk's process isn't found in its sandbox: %v", verdict)
	}
	if err := os.WriteFile(ready, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// It runs sleep now: the same pid, another run.
	for i := 0; ; i++ {
		if now, _ := reapLook(cmd.Process.Pid, reapAtOf(profile)); now.token[7] != run.token[7] {
			break
		}
		if i == 250 {
			t.Fatal("the process didn't run its next program")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := reapSignal(run, syscall.SIGKILL); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("a signal to a run that's gone: %v, want ESRCH", err)
	}
	select {
	case <-waited:
		t.Fatal("the process was ended by a run it no longer runs")
	case <-time.After(300 * time.Millisecond):
	}
	// An unsandboxed process (this test) is never taken for a desk's.
	if _, verdict := reapLook(os.Getpid(), reapAtOf(profile)); verdict != reapNotIn {
		t.Errorf("an unsandboxed process was taken for the desk's: %v", verdict)
	}
}
