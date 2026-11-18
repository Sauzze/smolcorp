//go:build darwin && cgo

package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXFIREREAPR5: a desk's mark, the one file its sandbox alone may read, which its stop's reap tells its processes by.
// The core makes it at the desk's first start and keeps it. A desk made before marks gets one at a start that finds
// nothing of it running (FXFIREREAPR6); till then its reap signals nothing.

// deskMade is a desk's sandbox made as its start makes it, in temporary folders (standInDesk's), with id ab12.
func deskMade(t *testing.T) (*deskSandbox, error) {
	t.Helper()
	return makeDeskSandbox(deskAt{ID: "ab12", Root: t.TempDir(), Socket: deskSocketPath("ab12", 0), Git: runGit})
}

// marksIn is the names in desk own's mark folder.
func marksIn(t *testing.T, own string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(own, "reap.noindex"))
	if err != nil {
		t.Fatalf("its mark folder: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// A desk's first start makes its mark: one file of a mark's name, in a folder of its own, that the profile allows its
// sandbox to read; its next start keeps the same one. A mark folder that doesn't hold one mark alone refuses the start,
// and the reap can't say the desk is empty.
func TestADesksMarkIsMadeAtItsFirstStartAndKept(t *testing.T) {
	s := standInDesk(t)
	own := physical(hireFolder(supportDir, s.at.ID))
	marks := marksIn(t, own)
	if len(marks) != 1 || !strings.HasPrefix(marks[0], "reap-") || len(marks[0]) != len("reap-")+32 {
		t.Fatalf("the first start made %q, want one mark", marks)
	}
	profile := deskProfileFor(t, s, deskSocketPath(s.at.ID, 0))
	text, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if line := `(allow file-read-data file-read-xattr (literal "` + filepath.Join(own, "reap.noindex", marks[0]) + `"))`; !strings.Contains(string(text), line) {
		t.Errorf("the profile doesn't allow its sandbox to read its mark: no %s", line)
	}
	if _, err := deskMade(t); err != nil {
		t.Fatal(err)
	}
	if again := marksIn(t, own); !slices.Equal(again, marks) {
		t.Errorf("the next start left %q, want the first start's %q", again, marks)
	}
	// Another file beside it (none of the core's makes one there, and no desk can): no start, and no "empty".
	if err := os.WriteFile(filepath.Join(own, "reap.noindex", "reap-ffffffffffffffffffffffffffffffff"), nil, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := deskMade(t); err == nil {
		t.Error("a start went on with two marks")
	}
	if left, sure := reapSandboxAt(own, false); len(left) != 0 || sure {
		t.Errorf("the reap's look with two marks said %v, %t; want it unsure", left, sure)
	}
}

// deskMadeBeforeMarks is desk ab12's own folder as an earlier version left it: its profile there, and no mark.
func deskMadeBeforeMarks(t *testing.T) string {
	t.Helper()
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	own := physical(hireFolder(supportDir, "ab12")) // resolved once it's there
	if err := os.WriteFile(filepath.Join(own, "sandbox.sb"), []byte("(version 1)\n(allow default)\n"), 0o600); err != nil {
		t.Fatal(err) // an earlier version's profile
	}
	return own
}

// FXFIREREAPR6: a desk made before marks, started with nothing of it running, gets its mark at that start, once the
// start's profile is written and before anything of it runs: the profile allows its sandbox to read it. From then on
// it's a full desk: its next stop ends what it left running in the background.
func TestADeskMadeBeforeMarksGetsOneAtAStartWithNothingOfItRunning(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	own := deskMadeBeforeMarks(t)
	s, err := deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(own, "reap.noindex")); !os.IsNotExist(err) {
		t.Errorf("its mark was made before its new profile (%v)", err)
	}
	socket := deskSocketPath(s.at.ID, 0)
	profile := deskProfileFor(t, s, socket)
	if marks := marksIn(t, own); len(marks) != 1 || marks[0] != s.places.ReapMark {
		t.Fatalf("the start made %q, want its profile's mark %q", marks, s.places.ReapMark)
	}
	text, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if line := `(allow file-read-data file-read-xattr (literal "` + filepath.Join(own, "reap.noindex", s.places.ReapMark) + `"))`; !strings.Contains(string(text), line) {
		t.Errorf("the profile doesn't allow its sandbox to read its mark: no %s", line)
	}

	desk := startStandInDesk(t, s, profile)
	if why := deskReap(Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket}); why != "" {
		t.Errorf("its stop said %q, want its programs ended", why)
	}
	for _, pid := range desk.all() {
		if inDesk(t, pid, profile) {
			t.Errorf("the desk's process %d still runs in its sandbox after its stop", pid)
		}
	}
}

// FXFIREREAPR6: a desk made before marks, started while a process of its earlier sandbox still runs, keeps no mark:
// that process couldn't read one. Its stop's reap looks by its profile, which another sandbox holding a read extension
// for it would answer as the desk's, so it signals nothing: the process is named and left running, and the refusal says
// what ends it, not to try again. Once none is left, the desk is said empty, and its next start gives it its mark.
func TestADeskMadeBeforeMarksStartedWithSomethingOfItRunningKeepsNoneAndRefuses(t *testing.T) {
	needSandboxExec(t)
	shortReap(t)
	own := deskMadeBeforeMarks(t)
	s, err := deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	socket := deskSocketPath(s.at.ID, 0)
	// What an earlier version wrote: this desk's profile without the mark's row.
	earlier := s.places
	earlier.ReapMark = ""
	pol, err := sandbox.New(earlier)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(own, "sandbox.sb")
	if err := sandbox.PrepareProfile(pol, profile, deskServices(earlier, socket)); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(profile); strings.Contains(string(text), "reap.noindex") {
		t.Fatal("the earlier version's profile names a mark")
	}
	cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan struct{})
	go func() { cmd.Wait(); close(ended) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-ended }) // this test's child, its end not read yet: its pid is its own
	pid := cmd.Process.Pid
	reapLooksAt(t, pid)
	for i := 0; ; i++ {
		if left, _ := reapSandboxAt(own, false); slices.Contains(left, pid) {
			break
		}
		if i == 250 {
			t.Fatal("the desk's process isn't found in its sandbox")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Its start, with that process still running.
	s, err = deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	deskProfileFor(t, s, socket)
	if _, err := os.Lstat(filepath.Join(own, "reap.noindex")); !os.IsNotExist(err) {
		t.Errorf("a start with a process of the desk still running gave it a mark (%v)", err)
	}

	signal := deskReapSystem.signal
	t.Cleanup(func() { deskReapSystem.signal = signal })
	var mu sync.Mutex
	var sent []string
	deskReapSystem.signal = func(run reapRun, sig syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, fmt.Sprintf("%v to process %d", sig, run.pid))
		return nil // stood in as sent: nothing reaches it
	}
	h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket}
	why := deskReap(h)
	if !strings.Contains(why, fmt.Sprintf("process %d", pid)) || !strings.Contains(why, "couldn't be ended") {
		t.Errorf("the stop said %q, want it to name the desk's process %d it left running", why, pid)
	}
	mu.Lock()
	if len(sent) > 0 {
		t.Errorf("the reap of a desk made before marks signalled %d times: %s…", len(sent), strings.Join(sent[:min(len(sent), 3)], "; "))
	}
	mu.Unlock()
	select {
	case <-ended:
		t.Error("the desk's process ended")
	default:
	}
	// Its refusals say what ends it, and never to try again as it is.
	fired, stopped := firedRefusal(h, why), stopRefusal(h, why)
	for _, refusal := range []string{fired, stopped} {
		if !strings.Contains(refusal, "Janitor dates from an earlier version of Smolcorp, so Smolcorp can't end Janitor's programs itself: quit it in Activity Monitor or restart your Mac, then ") ||
			strings.Contains(refusal, "Fire it again") || strings.Contains(refusal, "Try again") || strings.Contains(refusal, "keeps happening") {
			t.Errorf("an old desk's refusal said %q", refusal)
		}
	}
	if !strings.HasSuffix(fired, ", so nothing was checked, merged or removed. Janitor dates from an earlier version of Smolcorp, so Smolcorp can't end Janitor's programs itself: quit it in Activity Monitor or restart your Mac, then fire Janitor.") {
		t.Errorf("the fire's refusal said %q", fired)
	}
	if !strings.HasSuffix(stopped, ", so nothing more was done. Janitor dates from an earlier version of Smolcorp, so Smolcorp can't end Janitor's programs itself: quit it in Activity Monitor or restart your Mac, then try again. Once Janitor starts again with nothing of it running, Smolcorp will end such programs itself.") {
		t.Errorf("the stop's refusal said %q", stopped)
	}
	// Nothing named (a look never whole): a retry may find it empty, as for any desk.
	unsure := reapLeftWords(h, nil)
	if firedRefusal(h, unsure) != unsure+firedReapRefusal || stopRefusal(h, unsure) != unsure+stopReapRefusal {
		t.Error("an old desk's refusal with nothing named doesn't ask for a retry")
	}

	cmd.Process.Kill() // this test's own, by its pid: its end isn't read yet
	<-ended
	if why := deskReap(h); why != "" {
		t.Errorf("with nothing left, the stop said %q", why)
	}
	// Its next start, nothing of it running: its mark now, its profile's.
	s, err = deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	deskProfileFor(t, s, socket)
	if marks := marksIn(t, own); len(marks) != 1 || marks[0] != s.places.ReapMark {
		t.Errorf("the next start, nothing running, made %q, want its profile's mark %q", marks, s.places.ReapMark)
	}
	if refusal := firedRefusal(h, "A program Janitor started is still running in its sandbox and couldn't be ended (sleep, process 1)"); !strings.Contains(refusal, "Fire it again") {
		t.Errorf("a marked desk's refusal said %q", refusal)
	}
}

// FXFIREREAPR7 (RT126FIXREVIEW-FIREREAPR6 N2): a desk made before marks whose start's look couldn't be made whole keeps
// no mark: a process macOS didn't answer for, or one no list showed, may be one of its earlier sandbox's, which couldn't
// read a mark. Its start goes on, and nothing is signalled. Its next start with a whole look gives it its mark.
func TestADeskMadeBeforeMarksWhoseLookIsntWholeKeepsNoneAndStarts(t *testing.T) {
	shortReap(t)
	own := deskMadeBeforeMarks(t)
	socket := deskSocketPath("ab12", 0)
	list, look, signal := deskReapSystem.list, deskReapSystem.look, deskReapSystem.signal
	t.Cleanup(func() { deskReapSystem.list, deskReapSystem.look, deskReapSystem.signal = list, look, signal })
	deskReapSystem.signal = func(run reapRun, sig syscall.Signal) error {
		t.Errorf("the start's look sent %v to process %d", sig, run.pid)
		return nil // stood in as sent: nothing reaches it
	}
	unanswered := os.Getppid() // never asked: the look at it is stood in
	for _, c := range []struct {
		name string
		list func() ([]int, error)
		look func(pid int, at reapAt) (reapRun, reapVerdict)
	}{
		{"macOS doesn't answer for one process", func() ([]int, error) { return []int{unanswered}, nil },
			func(pid int, at reapAt) (reapRun, reapVerdict) { return reapRun{pid: pid}, reapUnknown }},
		{"the processes can't be listed", func() ([]int, error) { return nil, errors.New("stood in: no list") }, look},
	} {
		deskReapSystem.list, deskReapSystem.look = c.list, c.look
		s, err := deskMade(t)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		profile := deskProfileFor(t, s, socket) // the start goes on: a profile written, no refusal
		if _, err := os.Lstat(filepath.Join(own, "reap.noindex")); !os.IsNotExist(err) {
			t.Errorf("%s: the start gave the desk a mark (%v), want none", c.name, err)
		}
		if text, _ := os.ReadFile(profile); !strings.Contains(string(text), s.places.ReapMark) {
			t.Errorf("%s: the start didn't write its new profile", c.name)
		}
	}
	deskReapSystem.list, deskReapSystem.look = list, look
	s, err := deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	deskProfileFor(t, s, socket)
	if marks := marksIn(t, own); len(marks) != 1 || marks[0] != s.places.ReapMark {
		t.Errorf("the next start, its look whole, made %q, want its profile's mark %q", marks, s.places.ReapMark)
	}
}

// FXFIREREAPR7 (RT126FIXREVIEW-FIREREAPR6 O1): a desk made before marks gets its mark only once its new profile is
// written. A start refused before that leaves no mark beside its earlier profile, which can't read one, and which the
// core's later deskRun and attach would still start its programs under.
func TestARefusedStartOfADeskMadeBeforeMarksLeavesItNoMark(t *testing.T) {
	shortReap(t)
	own := deskMadeBeforeMarks(t)
	profile := filepath.Join(own, "sandbox.sb")
	earlier, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	s, err := deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	svc := deskServices(s.places, deskSocketPath(s.at.ID, 0))
	svc.Sockets = append(svc.Sockets, "not/whole") // a socket not spelled whole: its profile can't be written
	if err := s.prepareProfile(profile, svc); err == nil {
		t.Fatal("a start went on with a socket that isn't whole")
	}
	if _, err := os.Lstat(filepath.Join(own, "reap.noindex")); !os.IsNotExist(err) {
		t.Errorf("a refused start gave the desk a mark (%v), beside its earlier profile", err)
	}
	if text, _ := os.ReadFile(profile); string(text) != string(earlier) {
		t.Error("a refused start changed the desk's profile")
	}
	deskProfileFor(t, s, deskSocketPath(s.at.ID, 0)) // the control: the same start, its services whole
	if marks := marksIn(t, own); len(marks) != 1 || marks[0] != s.places.ReapMark {
		t.Errorf("the start made %q, want its profile's mark %q", marks, s.places.ReapMark)
	}
}

// FXFIREREAPR7 (RT126FIXREVIEW-FIREREAPR6 O2): a mark folder made meanwhile, holding another mark, while a start of a
// desk made before marks looks: that start's own mark can't be made, and its profile, written already, lets its sandbox
// read a mark that isn't the desk's, so the reap would take its processes for not in. The start is refused.
func TestADeskMadeBeforeMarksWhoseMarkFolderCameMeanwhileIsRefused(t *testing.T) {
	shortReap(t)
	own := deskMadeBeforeMarks(t)
	other := "reap-ffffffffffffffffffffffffffffffff"
	list := deskReapSystem.list
	t.Cleanup(func() { deskReapSystem.list = list })
	planted := false
	deskReapSystem.list = func() ([]int, error) { // the look's first list: the other mark comes then
		if planted {
			return list()
		}
		planted = true
		if err := os.Mkdir(filepath.Join(own, "reap.noindex"), 0o700); err != nil {
			t.Error(err)
		} else if err := os.WriteFile(filepath.Join(own, "reap.noindex", other), nil, 0o400); err != nil {
			t.Error(err)
		}
		return list()
	}
	s, err := deskMade(t)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(own, "sandbox.sb")
	if err := s.prepareProfile(profile, deskServices(s.places, deskSocketPath(s.at.ID, 0))); err == nil {
		t.Error("the start went on with another mark in the desk's mark folder")
	}
	if !planted {
		t.Fatal("the start didn't look") // else the row shows nothing
	}
	if marks := marksIn(t, own); !slices.Equal(marks, []string{other}) {
		t.Errorf("the mark folder holds %q, want the other mark alone", marks)
	}
	if text, _ := os.ReadFile(profile); !strings.Contains(string(text), s.places.ReapMark) {
		t.Fatal("the start didn't write its profile before its look") // else the row shows nothing
	}
}
