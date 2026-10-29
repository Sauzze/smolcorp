package desktop

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

func fakePicker(t *testing.T, pick func(start string) (string, bool, error)) {
	t.Helper()
	savedHas, savedPick := hasFolderPicker, pickFolder
	t.Cleanup(func() { hasFolderPicker, pickFolder = savedHas, savedPick })
	hasFolderPicker = func() bool { return true }
	pickFolder = pick
}

func TestSetupChoosesTheHiresFolderWithThePicker(t *testing.T) {
	d := newTestDeck(t)
	chosen := t.TempDir()
	var started string
	fakePicker(t, func(start string) (string, bool, error) {
		started = start
		return chosen, true, nil
	})
	choose := func() int {
		return serve(t, d, call{method: "POST", target: "/setup/hiring/choose", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	if code := choose(); code != 200 || d.config.HireFolder != chosen || started != homeDir {
		t.Fatalf("choosing: %d, folder %q, the picker started in %q", code, d.config.HireFolder, started)
	}
	// It starts in the folder chosen, and closed without a choice changes nothing.
	fakePicker(t, func(start string) (string, bool, error) {
		started = start
		return "", false, nil
	})
	if code := choose(); code != 200 || d.config.HireFolder != chosen || started != chosen {
		t.Errorf("closed without a choice: %d, folder %q, started in %q", code, d.config.HireFolder, started)
	}
	// What isn't a folder here isn't kept.
	fakePicker(t, func(string) (string, bool, error) { return "relative/path", true, nil })
	if choose(); d.config.HireFolder != chosen {
		t.Errorf("a picker's answer that isn't a folder was kept: %q", d.config.HireFolder)
	}
	fakePicker(t, func(string) (string, bool, error) { return "", false, errors.New("no display") })
	if code := choose(); code != 500 {
		t.Errorf("a picker that didn't open: %d", code)
	}
	hasFolderPicker = func() bool { return false }
	if code := choose(); code != 404 {
		t.Errorf("with no picker: %d", code)
	}
}

func TestAPickerOpenAlreadyIsntOpenedAgain(t *testing.T) {
	d := newTestDeck(t)
	opened := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	fakePicker(t, func(string) (string, bool, error) {
		calls++
		close(opened)
		<-release
		return "", false, nil
	})
	done := make(chan error)
	go func() { done <- d.chooseHireFolder() }()
	<-opened
	if err := d.chooseHireFolder(); err != nil || calls != 1 {
		t.Errorf("asked again while open: %v, %d pickers", err, calls)
	}
	close(release)
	<-done
}

func TestAPickerClosedWithoutAChoiceSaysSo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows' picker prints nothing instead")
	}
	cancelled := exec.Command("sh", "-c", "exit 1")
	if folder, chosen, err := pickedFolder(cancelled.Output()); chosen || err != nil || folder != "" {
		t.Errorf("a cancelled picker gave %q, %v, %v", folder, chosen, err)
	}
	picked := exec.Command("sh", "-c", "echo /home/someone/Projects")
	if folder, chosen, err := pickedFolder(picked.Output()); !chosen || err != nil || folder != "/home/someone/Projects" {
		t.Errorf("a picked folder gave %q, %v, %v", folder, chosen, err)
	}
}
