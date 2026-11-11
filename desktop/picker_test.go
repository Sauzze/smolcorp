package desktop

import (
	"errors"
	"net/http"
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

// The folder picker is the page's alone: a GM, a tool and a hire are refused the way every closed route refuses, and
// the stand-in picker is never asked, with pick=1 or for=floor as well. The page's call reaches the picker.
func TestTheFolderPickerIsOpenedByThePageAlone(t *testing.T) {
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	opened := 0
	fakePicker(t, func(string) (string, bool, error) {
		opened++
		return "", false, nil
	})
	for _, query := range []string{"", "?pick=1", "?pick=1&for=floor"} {
		for _, who := range []caller{callerGM, callerTool, callerHire, callerNobody} {
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from = "POST", "/setup/hiring/choose"+query, "localhost:7710", thisComputer
			if c.headers == nil {
				c.headers = map[string]string{}
			}
			c.headers[setupHeader] = "1"
			code := serve(t, d, c).Code
			if who == callerNobody {
				if code != http.StatusUnauthorized {
					t.Errorf("nobody on %q: %d, want 401", query, code)
				}
			} else if code != http.StatusForbidden {
				t.Errorf("caller %s on %q: %d, want 403", who.name(), query, code)
			}
		}
	}
	if opened != 0 {
		t.Fatalf("a refused caller opened the picker %d times", opened)
	}
	page := call{method: "POST", target: "/setup/hiring/choose", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}}
	if code := serve(t, d, page).Code; code != http.StatusOK || opened != 1 {
		t.Errorf("the page: %d, the picker opened %d times", code, opened)
	}
	// The GM is still given a folder through the hiring settings and the hire-folder route.
	for _, target := range []string{"/setup/hiring", "/setup/hire-folder"} {
		c := asCaller(callerGM, gm, hireKey)
		c.method, c.target, c.host, c.from = "POST", target, "localhost:7710", thisComputer
		c.headers[setupHeader] = "1"
		if code := serve(t, d, c).Code; code == http.StatusForbidden || code == http.StatusUnauthorized {
			t.Errorf("the GM on %s: %d, refused", target, code)
		}
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
