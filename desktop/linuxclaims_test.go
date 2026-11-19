package desktop

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXLINUXCLAIMS (RT126SWEEP2 C1): no Linux desk starts in a sandbox in 1.26.0 (deskSandboxed), so nothing tells a
// Linux owner its hires are in one, or asks for root to set one up. One switch, desksSandboxedHere, gates both
// Smolcorp's bubblewrap's line in Setup's checklist and the installer's step with sudo (`smolcorp sandbox used`);
// the day Linux desks start in their sandbox (SA-L4), both come back with it.

// installerGate is the installer's line that asks Smolcorp whether this computer's desks start in a sandbox.
const installerGate = `if "$BIN/smolcorp" sandbox used 2>/dev/null; then`

// On Linux, with no desk in a sandbox: Setup's checklist has the one no-sandbox line, never the bubblewrap fix, and
// doesn't run the bubblewrap check at all; `smolcorp sandbox used` says no. With the switch on, both come back.
func TestOnLinuxWithNoDeskInASandboxSetupNeverAsksForBubblewrap(t *testing.T) {
	saved := setupSystem
	t.Cleanup(func() { setupSystem = saved })
	setupSystem = "linux"
	playSandboxedHere(t, false)
	asked := 0
	checkSandbox = func() error { asked++; return fmt.Errorf("%w: smolcorp-bwrap isn't loaded", sandbox.ErrNoProfile) }
	d := newTestDeck(t)
	lines := 0
	for _, c := range d.selfTest() {
		said := c.Title + " " + c.Detail + " " + c.Fix
		if words := regexp.MustCompile(`(?i)bubblewrap|bwrap|apparmor|their own sandbox|start in a sandbox`).FindAllString(said, -1); words != nil {
			t.Errorf("the %s line says %q: %+v", c.ID, words, c)
		}
		if c.ID != "sandbox" {
			continue
		}
		lines++
		if c.Status != checkInfo || c.Title != "No sandbox on this computer yet" || c.Detail != noSandboxHere ||
			regexp.MustCompile(`(?i)sudo|password`).MatchString(said) {
			t.Errorf("Linux's sandbox line: %+v", c)
		}
	}
	if lines != 1 {
		t.Errorf("%d sandbox lines, want the one", lines)
	}
	if asked != 0 {
		t.Errorf("the checklist ran Smolcorp's bubblewrap check %d times, with no desk to start in it", asked)
	}
	if code := RunSandboxUsed(); code != 1 {
		t.Errorf("smolcorp sandbox used: %d, want 1 (no), so the installer asks for no password", code)
	}

	// The day Linux desks start in their sandbox: the same switch brings the check, its fix and the installer's step back.
	playSandboxedHere(t, true)
	checkSandbox = func() error { asked++; return fmt.Errorf("%w: smolcorp-bwrap isn't loaded", sandbox.ErrNoProfile) }
	var line *check
	checks := d.selfTest()
	for i := range checks {
		if checks[i].ID == "sandbox" {
			line = &checks[i]
		}
	}
	if line == nil || line.Status != checkFix || asked == 0 ||
		!strings.Contains(line.Detail, "Run this once in a terminal (it asks for your password): "+sandbox.SandboxCommand()) {
		t.Errorf("with desks in their sandbox, the bubblewrap fix (asked %d): %+v", asked, line)
	}
	if code := RunSandboxUsed(); code != 0 {
		t.Errorf("smolcorp sandbox used, desks in their sandbox: %d, want 0", code)
	}
}

// The Linux installer, read as text (no Linux here to run it): outside its one gated step, no line, comment or
// command, says sandbox, bubblewrap or AppArmor, or asks for sudo or a password; the step (the check, the command run
// with sudo, their words) is all inside the gate, which asks Smolcorp's own switch (RunSandboxUsed, from main.go).
func TestTheLinuxInstallerAsksForNoSandboxUntilDesksStartInOne(t *testing.T) {
	src, err := os.ReadFile("../packaging/linux/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(src), "\n")
	gate, end := -1, -1
	for i, line := range lines {
		if line == installerGate {
			if gate >= 0 {
				t.Fatalf("two gates, at lines %d and %d", gate+1, i+1)
			}
			gate = i
		} else if gate >= 0 && end < 0 && line == "fi" {
			end = i
		}
	}
	if gate < 0 || end < 0 {
		t.Fatalf("install.sh has no gated sandbox step (%q ... fi)", installerGate)
	}
	claims := regexp.MustCompile(`(?i)sandbox|bubblewrap|bwrap|apparmor|sudo|password`)
	for i, line := range lines {
		if (i < gate || i > end) && claims.MatchString(line) {
			t.Errorf("install.sh:%d, outside the gate: %s", i+1, line)
		}
	}
	step := strings.Join(lines[gate+1:end], "\n")
	for _, want := range []string{`"$BIN/smolcorp" sandbox check`, `"$BIN/smolcorp" sandbox command`, `sh -c "$SANDBOX"`, "LD_PRELOAD"} {
		if !strings.Contains(step, want) {
			t.Errorf("the gated step has no %s", want)
		}
	}
	main, err := os.ReadFile("../cmd/smolcorp/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`args\[1\] == "used" \{\s+os\.Exit\(desktop\.RunSandboxUsed\(\)\)`).Match(main) {
		t.Error(`main.go doesn't answer "smolcorp sandbox used" with desktop.RunSandboxUsed`)
	}
}
