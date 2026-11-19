package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FXCHROMEPOPUPR1 (SIERRA's ruling on RT126CHROMEPOPUP B1): startBrowserUntil, the way every test's browser starts,
// refuses the owner's own browser at run time, before anything starts, whatever found it: a program in a .app bundle
// or an Applications folder, a command outside the test's temp folder, or a link to one. Every path here is a stand-in:
// missing, or in the row's own temp folder; nothing here is ever run.

// standInBundle is a missing program in a missing app's bundle in /Applications: the shape the owner's browser has on
// a Mac (the review's MA and MC), refused by the rule, never by "not found".
const standInBundle = "/Applications/ZZ Stand-in.app/Contents/MacOS/x"

// standInCommand is a missing command outside any temp folder: the shape the owner's browser has on Linux (the
// review's MB, /usr/bin/google-chrome).
const standInCommand = "/usr/bin/zz-stand-in"

func TestABrowserIsRefusedUnlessItsAPlainShellOrAStandIn(t *testing.T) {
	dir := t.TempDir()
	home, _ := os.UserHomeDir()
	rows := []struct {
		program string
		said    string // what the refusal says; "" when the program may start
	}{
		{standInBundle, "is in an app bundle"},
		{"/Applications/zz-stand-in", "where apps are installed"},
		{"/applications/zz-stand-in", "where apps are installed"},
		{filepath.Join(home, "Applications", "zz-stand-in"), "where apps are installed"},
		{standInCommand, "is neither"},
		{filepath.Join(dir, "ZZ Stand-in.app", "Contents", "MacOS", "x"), "is in an app bundle"},
		{filepath.Join(dir, "standin-browser"), ""},
		// The opt-in's own (a plain chrome-headless-shell, which still starts) isn't a row here: its name, in a test
		// outside the helper, is a browser's path to TestEveryBrowserATestStartsGoesThroughTheHelper.
	}
	for _, row := range rows {
		if got := browserRefusal(row.program); row.said == "" && got != "" || row.said != "" && !strings.Contains(got, row.said) {
			t.Errorf("browserRefusal(%q) = %q, want %q", row.program, got, row.said)
		}
	}
	// A link in the temp folder to the owner's browser is refused for where it goes (the links' targets are missing).
	for _, target := range []string{standInBundle, standInCommand} {
		link := filepath.Join(t.TempDir(), "standin-browser")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("no links here (%v)", err)
		}
		if got := browserRefusal(link); got == "" {
			t.Errorf("browserRefusal of a link to %s gave it leave to start", target)
		}
	}
}

// browserRefusedVariable: set (to a program's path), this test program is a row's child: it hands the program to
// startBrowserUntil, which must refuse it before it starts anything.
const browserRefusedVariable = "SMOLCORP_TEST_BROWSER_REFUSED"

func TestABrowserHandedToTheHelperForARow(t *testing.T) {
	program := os.Getenv(browserRefusedVariable)
	if program == "" {
		t.Skip("only as a row's child (TestTheOwnersBrowserIsRefusedBeforeItStarts)")
	}
	startBrowserUntil(t, program, time.Now().Add(5*time.Second), nil)
	t.Log("NOT REFUSED: the helper went on to start it")
}

// The helper itself refuses, before it starts anything: the child is this test program again, handing it a stand-in
// of the owner's browser, a missing path, so that were the refusal gone, nothing could start ("not found").
func TestTheOwnersBrowserIsRefusedBeforeItStarts(t *testing.T) {
	for _, program := range []string{standInBundle, standInCommand} {
		dir := t.TempDir()
		child := exec.Command(os.Args[0], "-test.run=^TestABrowserHandedToTheHelperForARow$", "-test.count=1", "-test.v", "-test.timeout=1m")
		child.Env = append(os.Environ(), browserRefusedVariable+"="+program, "TMPDIR="+dir)
		said, err := child.CombinedOutput()
		if err == nil || !strings.Contains(string(said), "refused before it started: "+program) || strings.Contains(string(said), "NOT REFUSED") {
			t.Errorf("the helper handed %s: %v, want it refused before it started:\n%s", program, err, said)
		}
		if left := browserLeftovers(dir, time.Second); left != "" {
			t.Errorf("something of the row's is running:\n%s", left)
		}
	}
}
