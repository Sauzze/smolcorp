//go:build darwin && cgo

package desktop

import (
	"errors"
	"io/fs"
	"net/http"
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

// FXFIREREAPR1's rows. N1: the reap says none is left only from two passes that agree, so a process that moves to a new
// pid before each look is never missed, and a fork storm is ended or the stop refuses, in bounded time. N2: every stop
// of a sandboxed Mac desk ends what it started in its sandbox (a plain hire's agent, an ACP desk, a refused start, a
// start whose row turned gone, the engine's watch that gives up), and a desk isn't known stopped while anything runs
// there. O1: another sandbox that reads the desk's profile is never taken for the desk's. Each with an unsandboxed
// bystander and another desk's processes that must be left running. The same stand-ins as deskreap_darwin_test.go's.

// notRunning fails t for each of pids still among the owner's processes after what. Each of them is ended once its end
// is read: by this test (the pane), or by launchd (a child whose parent has ended). Until then it's a zombie, and listed.
func notRunning(t *testing.T, what string, pids ...int) {
	t.Helper()
	everyone, err := reapList()
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range pids {
		if slices.Contains(everyone, pid) && !isZombie(pid) {
			t.Errorf("the desk's process %d still runs %s", pid, what)
		}
	}
}

// FXFIREREAPR2: a zombie found by a pass that found none of the desk's is answered for only as the same process the pass
// before saw ended, or saw running whole outside the sandbox. FXFIREREAPR3: a pid gone at its look too, and "the same
// process" is its pid and start time, never its pid alone (RT126FIXREVIEW-FIREREAPR2 O1): once pids have gone round, a
// pid the pass before answered for can be a desk's process by this pass's look. Any other could have been a desk's
// process that made a child after the pass before listed every process. A start time the look couldn't read (0) names
// no process.
func TestAZombieIsAnsweredForOnlyAsTheProcessThePassBeforeSawEnded(t *testing.T) {
	type seen = map[int]uint64
	pass := func(answered, gone, zombies seen) *reapPass {
		return &reapPass{whole: true, answered: answered, gone: gone, zombies: zombies}
	}
	for _, row := range []struct {
		name        string
		before, now *reapPass
		agrees      bool
	}{
		{"the same zombie", pass(nil, nil, seen{4242: 7}), pass(nil, nil, seen{4242: 7}), true},
		{"the same process running whole outside before", pass(seen{4242: 7}, nil, nil), pass(nil, nil, seen{4242: 7}), true},
		{"another process on its pid before, ended", pass(nil, nil, seen{4242: 6}), pass(nil, nil, seen{4242: 7}), false},
		{"another process on its pid before, running outside", pass(seen{4242: 6}, nil, nil), pass(nil, nil, seen{4242: 7}), false},
		{"unseen before", pass(nil, nil, nil), pass(nil, nil, seen{4242: 7}), false},
		{"no pass before", nil, pass(nil, nil, seen{4242: 7}), false},
		{"gone: the same process running whole outside before", pass(seen{4242: 7}, nil, nil), pass(nil, seen{4242: 7}, nil), true},
		{"gone: another process running outside on its pid before", pass(seen{4242: 6}, nil, nil), pass(nil, seen{4242: 7}, nil), false},
		{"gone before its start time was read", pass(seen{4242: 7}, nil, nil), pass(nil, seen{4242: 0}, nil), false},
		{"gone, neither start time read", pass(seen{4242: 0}, nil, nil), pass(nil, seen{4242: 0}, nil), false},
		{"gone: a zombie before, the same process", pass(nil, nil, seen{4242: 7}), pass(nil, seen{4242: 7}, nil), true},
		{"gone: unseen before", pass(nil, nil, nil), pass(nil, seen{4242: 7}, nil), false},
		{"a zombie before, gone now unread", pass(nil, nil, seen{4242: 7}), pass(nil, seen{4242: 0}, nil), false},
		{"something else running outside before", pass(seen{4343: 7}, nil, nil), pass(nil, seen{4242: 7}, nil), false},
	} {
		if got := row.now.agrees(row.before); got != row.agrees {
			t.Errorf("%s: agrees %t, want %t", row.name, got, row.agrees)
		}
	}
}

// coreGitInDeskSandbox has the core's git on a desk's worktree run in its sandbox, as sandboxedDeskRunning has it: with
// its profile there, this test program is its `smolcorp sandbox enter` (SA-X6).
func coreGitInDeskSandbox(t *testing.T) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	was := deskRunSmolcorp
	deskRunSmolcorp = func() string { return physical(program) }
	t.Cleanup(func() { deskRunSmolcorp = was })
	t.Setenv(deskRunStandInVariable, "1")
	t.Setenv("HOME", homeDir)
}

// forkStorm keeps forking under setsid: each process writes "fork", forks, writes its child's pid, and ends once the
// child has written its own, so one of them always runs and is written down (pidsWrittenWhole). It ignores SIGTERM,
// and ends by itself after its limit.
const forkStorm = `my ($file, $limit) = @ARGV;
$SIG{TERM} = "IGNORE";
sub note { open(my $f, ">>", $file) or exit 1; print $f "$_[0]\n"; close $f }
note($$);
my $end = time() + $limit;
while (time() < $end) {
	pipe(my $r, my $w) or exit 1;
	note("fork");
	my $pid = fork;
	exit 1 unless defined $pid;
	if ($pid) { note($pid); close $w; my $x = <$r>; exit 0 }
	close $r; POSIX::setsid(); note($$); close $w;
}`

// mover moves to a new pid each time it gets SIGUSR1, as forkStorm does, and is otherwise still.
const mover = `my ($file, $limit) = @ARGV;
my $moved = 0;
$SIG{USR1} = sub { $moved = 1 };
$SIG{TERM} = "IGNORE";
sub note { open(my $f, ">>", $file) or exit 1; print $f "$$\n"; close $f }
note();
my $end = time() + $limit;
while (time() < $end) {
	select(undef, undef, undef, 0.01);
	next unless $moved;
	$moved = 0;
	pipe(my $r, my $w) or exit 1;
	my $pid = fork;
	exit 1 unless defined $pid;
	if ($pid) { close $w; my $x = <$r>; exit 0 }
	close $r; POSIX::setsid(); note(); close $w;
}`

// startMoving starts script (forkStorm or mover) in desk s's sandbox, under its profile, writing its pids in the desk's
// own name.md: that file is registered for the reap. Whatever of it is still in the desk's sandbox at the end is ended
// then, by its run.
func startMoving(t *testing.T, s *deskSandbox, profile, name, script string) (file string) {
	t.Helper()
	file = filepath.Join(physical(hireFolder(supportDir, s.at.ID)), name+".md")
	cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/usr/bin/perl", "-MPOSIX", "-e", script, file, "30")
	cmd.Dir = s.places.Root
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	reapRegistered.Lock()
	reapRegistered.files[file] = true
	reapRegistered.Unlock()
	t.Cleanup(func() {
		endMoving(file, profile)
		reapRegistered.Lock()
		delete(reapRegistered.files, file)
		for _, pid := range pidsWrittenIn(file) {
			delete(reapRegistered.pids, pid)
		}
		reapRegistered.Unlock()
	})
	for i := 0; len(pidsWrittenIn(file)) == 0; i++ {
		if i == 250 {
			t.Fatalf("no pid in %s", file)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pidsWrittenIn(file)[0] != cmd.Process.Pid {
		t.Fatalf("another process wrote %s", file)
	}
	return file
}

// endMoving ends, by their runs, the processes written in file that are still in the sandbox whose profile is given,
// until two looks in a row find none (or 5 s).
func endMoving(file, profile string) {
	quiet := 0
	for deadline := time.Now().Add(5 * time.Second); quiet < 2 && time.Now().Before(deadline); {
		found := 0
		everyone, _ := reapList()
		for _, pid := range pidsWrittenIn(file) {
			if !slices.Contains(everyone, pid) {
				continue
			}
			if run, verdict := reapLook(pid, reapAtOf(profile)); verdict == reapIn {
				found++
				reapSignal(run, syscall.SIGKILL)
			}
		}
		if quiet++; found > 0 {
			quiet = 0
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// stillIn is the processes written in file still among the owner's, in the sandbox whose profile is given.
func stillIn(t *testing.T, file, profile string) []int {
	t.Helper()
	everyone, err := reapList()
	if err != nil {
		t.Fatal(err)
	}
	var in []int
	for _, pid := range pidsWrittenIn(file) {
		if slices.Contains(everyone, pid) && inDesk(t, pid, profile) {
			in = append(in, pid)
		}
	}
	return in
}

// N1: a desk's process that forks to a new pid and ends between the list and its look, at every look (a stand-in for
// the fastest fork chain: the test moves it just before the reap's look). One pass then finds none of the desk's; the
// reap must not say none is left. It refuses within its time, and says it couldn't make sure.
func TestTheReapNeverSaysNoneLeftOfAProcessThatMovesBeforeEachLook(t *testing.T) {
	_, _, hire, desk, other, bystander := sandboxedDeskRunning(t)
	s, _ := deskSandboxOf(hire.ID)
	file := startMoving(t, s, desk.profile, "mover", mover)
	look := deskReapSystem.look
	t.Cleanup(func() { deskReapSystem.look = look })
	moves := 0
	deskReapSystem.look = func(pid int, at reapAt) (reapRun, reapVerdict) {
		if pids := pidsWrittenIn(file); len(pids) > 0 && pids[len(pids)-1] == pid {
			if run, verdict := look(pid, at); verdict == reapIn && reapSignal(run, syscall.SIGUSR1) == nil {
				for range 300 { // its child written down, and itself gone
					everyone, _ := reapList()
					if now := pidsWrittenIn(file); now[len(now)-1] != pid && !slices.Contains(everyone, pid) {
						moves++
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
		return look(pid, at)
	}
	began := time.Now()
	why := deskReap(hire)
	took := time.Since(began)
	deskReapSystem.look = look
	want := "Smolcorp couldn't make sure every program " + calledHire(hire) + " started has ended"
	if why != want {
		t.Errorf("the reap said %q, want %q: the desk's process %v still runs in its sandbox", why, want,
			stillIn(t, file, desk.profile))
	}
	if limit := deskReapGrace + deskReapWithin + 2*time.Second; took > limit {
		t.Errorf("the reap took %v, more than %v", took, limit)
	}
	t.Logf("the reap: %q after %v; the stand-in moved %d times", why, took, moves)
	notRunning(t, "after the reap", desk.all()...)
	untouched(t, other, bystander)
}

// N1: a fork storm in the desk's sandbox, keeping on forking under setsid and ignoring SIGTERM, with the reap's real
// looks. The reap ends it and says none is left, or refuses, in bounded time: it never says none is left while any of
// it runs, or before it has stopped taking new pids.
func TestAForkStormInADesksSandboxIsEndedOrItsStopRefuses(t *testing.T) {
	_, _, hire, desk, other, bystander := sandboxedDeskRunning(t)
	s, _ := deskSandboxOf(hire.ID)
	file := startMoving(t, s, desk.profile, "storm", forkStorm)
	for i := 0; len(pidsWrittenIn(file)) < 20; i++ {
		if i == 250 {
			t.Fatal("the fork storm didn't start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	began := time.Now()
	why := deskReap(hire)
	took := time.Since(began)
	left := stillIn(t, file, desk.profile)
	t.Logf("the reap: %q after %v; the storm took %d pids; still in its sandbox: %v", why, took,
		len(pidsWrittenIn(file)), left)
	// Ended, not hidden: it writes no new pid afterwards.
	written := len(pidsWrittenIn(file))
	time.Sleep(300 * time.Millisecond)
	if more := pidsWrittenIn(file)[written:]; len(more) > 0 {
		t.Logf("the fork storm took %d pids more after the reap", len(more))
		left = append(left, more...)
	}
	if why == "" && len(left) > 0 {
		t.Errorf("the reap said none was left while %v still run in its sandbox", left)
	}
	if why != "" && !strings.Contains(why, calledHire(hire)) {
		t.Errorf("a refusal not in the owner's words: %q", why)
	}
	if why != "" {
		t.Logf("the reap refused (fail closed); %d of the storm still in its sandbox", len(left))
	}
	if limit := deskReapGrace + deskReapWithin + 2*time.Second; took > limit {
		t.Errorf("the reap took %v, more than %v", took, limit)
	}
	notRunning(t, "after the reap", desk.all()...)
	untouched(t, other, bystander)
}

// N2: a plain hire's agent in its sandbox (the launcher's): its stop ends every program the agent started there, the
// setsid and nohup ones too, before its own folder (and its profile with it) goes.
func TestAPlainHiresStopEndsEveryProgramItsAgentStarted(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	const id = "plain-ab12cd34ef56"
	own := hireFolder(supportDir, id)
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	ds, err := makeDeskSandbox(deskAt{ID: id, Root: t.TempDir(), Socket: deskSocketPath(id, 0), Git: runGit})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keptSandboxes.Lock(); delete(keptSandboxes.of, id); keptSandboxes.Unlock() })
	profile := deskProfileFor(t, ds, deskSocketPath(id, 0))
	agent := startStandInDesk(t, ds, profile)
	other, bystander := otherDeskAndBystander(t)
	s := &launcherSandbox{desk: ds, own: own, profile: profile}
	s.close()
	notRunning(t, "after the plain hire's stop", agent.all()...)
	untouched(t, other, bystander)
	if _, err := os.Stat(own); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("its own folder wasn't taken away: %v", err)
	}
}

// N2: an ACP desk's stop ends every program it started in its sandbox, not only its terminals' groups.
func TestAnACPDesksStopEndsEveryProgramItStartedInItsSandbox(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	s := standInDesk(t)
	desk := startStandInDesk(t, s, deskProfileFor(t, s, "")) // an ACP desk's: no socket of its own
	other, bystander := otherDeskAndBystander(t)
	sb := newACPSandbox(s, filepath.Join(t.TempDir(), "smolcorp"))
	sb.close()
	notRunning(t, "after the ACP desk's stop", desk.all()...)
	untouched(t, other, bystander)
}

// sandboxedStart is an engine rig whose desks start in their sandbox through a stand-in start-only runner, as
// sandboxedForHandBack's, which starts the desk's stand-in processes under its real profile, calls meanwhile, and
// answers with said(socket). desk is filled in by the start.
func sandboxedStart(t *testing.T, said func(socket string) string, meanwhile func(s *deskSandbox)) (r *engineRig, desk *standInProcs) {
	t.Helper()
	needSandboxExec(t)
	shortReap(t)
	_, r, _ = engineDeck(t)
	deskSockets(t)
	coreGitInDeskSandbox(t)
	desk = &standInProcs{}
	r.o.Sandbox = func(s *deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, errors.New(takenPlace)
			}
			staleSocketAt(t, args[1])
			r.tmux.up = true
			*desk = startStandInDesk(t, s, deskProfileFor(t, s, args[1]))
			if meanwhile != nil {
				meanwhile(s)
			}
			return said(args[1]), 5151, nil
		}
	}
	return r, desk
}

// N2: a start the core refuses once its server is up (hiretmux.go's start: what tmux said isn't whole) ends every
// program it left in its sandbox before its row, worktree and folders go.
func TestARefusedStartEndsEveryProgramItLeftInItsSandbox(t *testing.T) {
	r, desk := sandboxedStart(t, func(string) string { return "%1 4242" }, nil) // no socket: refused
	other, bystander := otherDeskAndBystander(t)
	if _, err := runManagedHire(r.request("janitor"), r.o); err == nil {
		t.Fatal("a start with no socket said wasn't refused")
	}
	if desk.pane == 0 {
		t.Fatal("the stand-in desk didn't start")
	}
	notRunning(t, "after its refused start", desk.all()...)
	untouched(t, other, bystander)
	if rows := registered(t); len(rows) != 0 {
		t.Errorf("the refused start's row stays: %+v", rows)
	}
}

// N2: a start whose row turned gone while its server came up (hireengine.go: ended rather than left running) ends every
// program it started in its sandbox, before its worktree and folders go.
func TestAStartWhoseRowTurnedGoneEndsEveryProgramItStarted(t *testing.T) {
	r, desk := sandboxedStart(t, func(socket string) string { return "%1 4242 " + socket }, func(s *deskSandbox) {
		_ = updateRegistry(func(f *registryFile) {
			if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == s.at.ID }); i >= 0 {
				f.hires[i].State = "fired"
			}
		})
	})
	other, bystander := otherDeskAndBystander(t)
	_, err := runManagedHire(r.request("janitor"), r.o)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != http.StatusConflict || !strings.HasSuffix(refused.Message, "so its tmux session was ended.") {
		t.Fatalf("%v, want the start's words for a desk marked gone", err)
	}
	notRunning(t, "after its start was ended", desk.all()...)
	untouched(t, other, bystander)
}

// N2: the engine's watch that gives up on a desk that never reported ends every program it started in its sandbox,
// before its worktree goes.
func TestTheWatchThatGivesUpEndsEveryProgramItsDeskStarted(t *testing.T) {
	_, r, hire, desk, other, bystander := sandboxedDeskRunning(t)
	o := r.o
	o.Within, o.Every = 20*time.Millisecond, 5*time.Millisecond
	o.HeardFrom = func(Hire) bool { return false }
	watchManagedHire(hire, 0, o)
	notRunning(t, "after its watch gave up", desk.all()...)
	untouched(t, other, bystander)
	if row := registered(t)[0]; row.State != "failed" {
		t.Errorf("row %+v, want failed", row)
	}
	if _, err := os.Stat(hire.Worktree.storedOnly()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("its worktree wasn't taken away: %v", err)
	}
}

// N2: CLAUDEPLANT1's "remove only while stopped": a desk whose server is gone and whose socket is silent isn't known
// stopped while a program it started still runs in its sandbox. The look ends nothing; once they're ended, it is.
func TestADeskIsntKnownStoppedWhileAProgramItStartedRunsInItsSandbox(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	s := standInDesk(t)
	socket := deskSocketPath(s.at.ID, 0)
	desk := startStandInDesk(t, s, deskProfileFor(t, s, socket))
	other, bystander := otherDeskAndBystander(t)
	h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket, TmuxServer: 99999999} // no process has that pid
	if deskStoppedNow(h) {
		t.Error("known stopped while its setsid and nohup children still run in its sandbox")
	}
	for _, pid := range desk.all() {
		if !inDesk(t, pid, desk.profile) {
			t.Errorf("the look ended the desk's process %d", pid)
		}
	}
	if why := deskReap(h); why != "" {
		t.Fatal(why)
	}
	if !deskStoppedNow(h) {
		t.Error("not known stopped once nothing runs in its sandbox")
	}
	untouched(t, other, bystander)
}

// O1: another sandbox (not a desk's) that may read the desk's profile and its mark is never taken for the desk's, and
// is left running by its stop. Each pins one check of the fingerprint: reading everything; reading all but the hires
// folder (the support folder's check); all but the support folder (the hires folder's); neither folder, but writing the
// profile (the write check); neither folder nor writing the profile, but reading the mark's folder (its check:
// FXFIREREAPR5, a read extension for a folder that holds the mark).
func TestAnotherSandboxThatReadsTheDesksProfileIsntTakenForTheDesks(t *testing.T) {
	_, _, hire, desk, other, bystander := sandboxedDeskRunning(t)
	support := physical(supportDir)
	hires := filepath.Join(support, "hires")
	noWrites := "(deny file-write* (subpath " + strconv.Quote(support) + "))"
	sandboxes := []struct{ name, rules string }{
		{"reads everything", noWrites},
		{"reads the profile, not the hires folder", noWrites + "\n(deny file-read-data (literal " + strconv.Quote(hires) + "))"},
		{"reads the profile and the hires folder, not the support folder", noWrites + "\n(deny file-read-data (literal " + strconv.Quote(support) + "))"},
		{"reads neither folder, and may write the profile", "(deny file-read-data (literal " + strconv.Quote(hires) + ") (literal " + strconv.Quote(support) + "))"},
		{"reads neither folder nor writes the profile, but reads the mark's folder", noWrites + "\n(deny file-read-data (literal " + strconv.Quote(hires) + ") (literal " + strconv.Quote(support) + "))"},
	}
	pids := make([]int, len(sandboxes))
	for i, sb := range sandboxes {
		dir := t.TempDir()
		profile, ready := filepath.Join(dir, "other.sb"), filepath.Join(dir, "ready")
		if err := os.WriteFile(profile, []byte("(version 1)\n(allow default)\n"+sb.rules+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", `echo $$ > "$1"; exec /bin/sleep 600`, "sh", ready)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		waited := make(chan struct{})
		go func() { cmd.Wait(); close(waited) }()
		t.Cleanup(func() { cmd.Process.Kill(); <-waited })
		if pidWritten(t, ready) != cmd.Process.Pid { // in its sandbox by then
			t.Fatalf("%s: another process wrote its pid", sb.name)
		}
		reapLooksAt(t, cmd.Process.Pid)
		pids[i] = cmd.Process.Pid
		if _, verdict := reapLook(cmd.Process.Pid, reapAtOf(desk.profile)); verdict != reapNotIn {
			t.Errorf("a sandbox that %s is taken for the desk's: %v", sb.name, verdict)
		}
	}
	if why := deskReap(hire); why != "" {
		t.Fatal(why)
	}
	for i, sb := range sandboxes {
		if err := syscall.Kill(pids[i], 0); err != nil {
			t.Errorf("a sandbox that %s was ended by the desk's stop: %v", sb.name, err)
		}
	}
	notRunning(t, "after its stop", desk.all()...)
	untouched(t, other, bystander)
}
