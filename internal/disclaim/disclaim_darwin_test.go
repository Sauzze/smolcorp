package disclaim

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These build the real smolcorp-spawn (packaging/macos/smolcorp-spawn.c) and a stand-in program with cc, in the
// test's own folder, and run only those. No permission is asked for and no TCC database is read or changed: who is
// responsible for a process is read with macOS's own call, on stand-ins started here.

// built is a C file compiled into folder, or the test skipped if this Mac can't build C. The developer folder is
// checked first: with none, /usr/bin/cc would offer to install the tools in a window.
func built(t *testing.T, folder, name, source string, flags ...string) string {
	t.Helper()
	if out, err := exec.Command("/usr/bin/xcode-select", "-p").Output(); err != nil {
		t.Skipf("no developer tools here: %v", err)
	} else if _, err := os.Stat(strings.TrimSpace(string(out))); err != nil {
		t.Skipf("the developer tools' folder isn't here: %v", err)
	}
	program := filepath.Join(folder, name)
	args := append([]string{"-Wall", "-Wextra", "-Werror", "-o", program}, flags...)
	if out, err := exec.Command("/usr/bin/cc", append(args, source)...).CombinedOutput(); err != nil {
		t.Skipf("cc can't build %s here: %v\n%s", name, err, out)
	}
	return program
}

// spawnAndStandIn builds smolcorp-spawn, signed ad hoc with the hardened runtime as the app's copy is, and the
// stand-in, and has Cmd use that smolcorp-spawn as an app's would be.
func spawnAndStandIn(t *testing.T) (spawn, standin string) {
	t.Helper()
	folder := t.TempDir()
	spawn = built(t, folder, SpawnName, filepath.Join("..", "..", "packaging", "macos", "smolcorp-spawn.c"))
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--options", "runtime", spawn).CombinedOutput(); err != nil {
		t.Fatalf("signing the stand-in smolcorp-spawn ad hoc: %v\n%s", err, out)
	}
	standin = built(t, folder, "standin", filepath.Join("testdata", "standin.c"))
	standIn(t, true, Place{Spawn: spawn, InApp: true})
	return spawn, standin
}

var saidPid = regexp.MustCompile(`pid=(\d+) responsible=(-?\d+)`)

// pids reads what the stand-in said of itself: its pid and the pid responsible for it.
func pids(t *testing.T, said string) (pid, responsible int) {
	t.Helper()
	m := saidPid.FindStringSubmatch(said)
	if m == nil {
		t.Fatalf("the stand-in didn't say who it is: %q", said)
	}
	pid, _ = strconv.Atoi(m[1])
	responsible, _ = strconv.Atoi(m[2])
	return pid, responsible
}

// The proof (SANDBOX-PLAN §3.5): a program started through smolcorp-spawn is responsible for itself, not for the
// program that started it. The holder stands in for Smolcorp.app: it is started on its own account, as launchd
// starts an app, so what it starts is charged to it unless disclaimed. It then starts the stand-in twice: as it
// would on its own (charged to the holder, which shows the check can see a charge at all), and through Cmd.
func TestAProgramStartedThroughSmolcorpSpawnIsResponsibleForItselfNotForSmolcorp(t *testing.T) {
	spawn, standin := spawnAndStandIn(t)
	holder := exec.Command(spawn, os.Args[0], os.Args[0], "-test.run=^TestHolderProcess$", "-test.count=1")
	holder.Env = append(os.Environ(), "DISCLAIM_HOLDER_SPAWN="+spawn, "DISCLAIM_HOLDER_STANDIN="+standin)
	out, err := holder.CombinedOutput()
	if err != nil {
		t.Fatalf("the holder: %v\n%s", err, out)
	}
	said := string(out)
	holderPid := holder.Process.Pid
	plain, through, found := strings.Cut(said, "--- through smolcorp-spawn\n")
	if !found {
		t.Fatalf("the holder said: %q", said)
	}
	plainPid, plainResponsible := pids(t, plain)
	if plainResponsible != holderPid || plainPid == holderPid {
		t.Fatalf("started as it would be on its own, the stand-in should be charged to the holder %d: pid %d, responsible %d\n%s",
			holderPid, plainPid, plainResponsible, said)
	}
	childPid, childResponsible := pids(t, through)
	if childResponsible != childPid || childResponsible == holderPid {
		t.Fatalf("started through smolcorp-spawn, the stand-in %d should be responsible for itself, not the holder %d: responsible %d\n%s",
			childPid, holderPid, childResponsible, said)
	}
}

// TestHolderProcess is the holder of the proof above, run only from it.
func TestHolderProcess(t *testing.T) {
	spawn, standin := os.Getenv("DISCLAIM_HOLDER_SPAWN"), os.Getenv("DISCLAIM_HOLDER_STANDIN")
	if spawn == "" {
		t.Skip("run by TestAProgramStartedThroughSmolcorpSpawnIsResponsibleForItselfNotForSmolcorp")
	}
	standIn(t, true, Place{Spawn: spawn, InApp: true})
	plain, err := exec.Command(standin, "show").Output()
	if err != nil {
		t.Fatal(err)
	}
	through, err := Cmd(exec.Command(standin, "show")).Output()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s--- through smolcorp-spawn\n%s", plain, through)
}

// smolcorp-spawn stands nowhere between Smolcorp and the program: the same pid, its words as they are, its own exit
// status and signal death, and a deadline that stops the program itself.
func TestSmolcorpSpawnBecomesTheProgram(t *testing.T) {
	_, standin := spawnAndStandIn(t)

	words := []string{"show", "two words", "", "$(touch nope)", "*", "'q'", "-c"}
	cmd := Cmd(exec.Command(standin, words...))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if pid, _ := pids(t, string(out)); pid != cmd.Process.Pid {
		t.Errorf("the program's pid %d isn't the one Smolcorp started, %d", pid, cmd.Process.Pid)
	}
	for i, word := range append([]string{standin}, words...) {
		if line := fmt.Sprintf("arg[%d]=[%s]\n", i, word); !strings.Contains(string(out), line) {
			t.Errorf("word %d isn't as given (%q): %s", i, word, out)
		}
	}

	var exit *exec.ExitError
	if err := Cmd(exec.Command(standin, "exit", "3")).Run(); !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Errorf("exit 3 came back as %v", err)
	}
	err = Cmd(exec.Command(standin, "signal")).Run()
	if !errors.As(err, &exit) {
		t.Fatalf("a signal death came back as %v", err)
	}
	if status := exit.Sys().(syscall.WaitStatus); !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Errorf("a SIGTERM death came back as %v", status)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	waiting := Cmd(exec.CommandContext(ctx, standin, "wait"))
	waiting.WaitDelay = time.Second
	start := time.Now()
	if err := waiting.Run(); err == nil {
		t.Fatal("a program past its deadline ended well")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the deadline took %v to stop it", took)
	}
	if err := syscall.Kill(waiting.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("the program outlived its deadline (pid %d): %v", waiting.Process.Pid, err)
	}
}

// When this Mac has no call that disclaims, smolcorp-spawn starts nothing and says why: never the program charged
// to Smolcorp. A test build names a call that isn't there.
func TestWithNoCallToDisclaimNothingStarts(t *testing.T) {
	_, standin := spawnAndStandIn(t)
	folder := t.TempDir()
	spawn := built(t, folder, SpawnName, filepath.Join("..", "..", "packaging", "macos", "smolcorp-spawn.c"),
		`-DDISCLAIM_CALL="smolcorp_no_such_call"`)
	standIn(t, true, Place{Spawn: spawn, InApp: true})
	marker := filepath.Join(folder, "ran")
	out, err := Cmd(exec.Command(standin, "touch", marker)).CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 126 ||
		!strings.Contains(string(out), "smolcorp_no_such_call), so "+standin+" wasn't started: it would have held Smolcorp's permissions.") {
		t.Fatalf("%v: %s", err, out)
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the program ran: %v", err)
	}
}
