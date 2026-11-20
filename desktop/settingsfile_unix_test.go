//go:build !windows

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// SETTINGSFILE1: the tmux engine's ticket checks the desk's settings file at the last moment before its start. A file
// changed after it was written opens nothing, and the desk's watch is told why in words; the file as written starts,
// named by its path alone.
func TestATicketsStartIsRefusedWhenItsSettingsFileChanged(t *testing.T) {
	shortSupport(t)
	noLoginShell(t)
	standInDeskProgram(t, "/opt/stand-in/claude")
	standInPeer(t)
	r := newHireSettingsRig(t)
	pane := os.Getpid()
	for _, changed := range []bool{false, true} {
		written, err := r.writeFile("sf1", nil)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := hireStart{Role: testHireRole(), Message: "go", DefaultAccount: true, Settings: written}.plan()
		if err != nil {
			t.Fatal(err)
		}
		command, err := deskStarts.issue(standInHelper, "sf1", paneTmux(&pane), plan, nil)
		if err != nil {
			t.Fatal(err)
		}
		deskStarts.pin("sf1", pane)
		if changed { // a second name for it, made after it was written
			must0(t, os.Link(written.Path, filepath.Join(filepath.Dir(written.Path), "STATUS.md")))
		}
		launch, err := takeDeskStart(command[2:])
		why := deskStarts.forget("sf1")
		if !changed && (err != nil || launch.Error != "" || !slices.Contains(launch.Args, "--settings="+written.Path)) {
			t.Errorf("untouched: %+v %v", launch, err)
		}
		if changed && (launch.Program != "" || !strings.Contains(launch.Error, "settings file isn't the one Smolcorp wrote") || why != launch.Error) {
			t.Errorf("changed: %+v, the watch told %q", launch, why)
		}
	}
}

// A hire whose settings file can't be written isn't started: nothing of it is left, and the refusal says why.
func TestAnEngineHireWhoseSettingsFileCantBeWrittenIsntStarted(t *testing.T) {
	r := newEngineRig(t)
	must0(t, os.MkdirAll(supportDir, 0o700))
	must0(t, os.WriteFile(filepath.Join(supportDir, "hires"), nil, 0o600)) // the hires folder's place taken by a file
	_, err := runManagedHire(r.request("janitor"), r.o)
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "couldn't write this desk's settings file") {
		t.Fatalf("err = %v", err)
	}
	if branches(t, r.repo) != "" || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Errorf("something was made for a hire whose settings weren't written: %q %q %v", branches(t, r.repo), r.tmux.asked, registered(t))
	}
}
