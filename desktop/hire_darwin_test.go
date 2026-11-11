//go:build darwin

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// scriptedHost stands in for the Mac app, answering each action its own way.
// It never tells a terminal anything.
type scriptedHost struct {
	asked   []bridgeRequest
	answers map[string]func(bridgeRequest) (bridgeAnswer, error)
}

func (s *scriptedHost) call(r bridgeRequest) (bridgeAnswer, error) {
	s.asked = append(s.asked, r)
	if answer, ok := s.answers[r.Action]; ok {
		return answer(r)
	}
	return bridgeAnswer{OK: false, Error: "not scripted"}, nil
}

func (s *scriptedHost) actions() []string {
	var names []string
	for _, r := range s.asked {
		names = append(names, r.Action)
	}
	return names
}

// withMacOpeners puts the Mac's openers in place, on a stand-in app, for one test.
func withMacOpeners(t *testing.T, installed []string) *scriptedHost {
	t.Helper()
	saved := installedOpeners
	t.Cleanup(func() { installedOpeners = saved })
	app := &scriptedHost{answers: map[string]func(bridgeRequest) (bridgeAnswer, error){
		"terminals": func(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{OK: true, Terminals: installed}, nil },
		"open": func(r bridgeRequest) (bridgeAnswer, error) {
			return bridgeAnswer{OK: true, Tab: "tty:/dev/ttys004", Window: "tty:/dev/ttys004#w1234"}, nil
		},
	}}
	useMacOpeners(app)
	return app
}

// The terminals offered are the app's, in setup's order, and the list is asked once in a while, not at every look.
func TestTheMacsTerminalsAreTheAppsInstalledOnes(t *testing.T) {
	isolate(t)
	app := withMacOpeners(t, []string{"ghostty", "terminal", "xterm"})
	for range 3 {
		openers := installedOpeners()
		var ids []string
		for _, o := range openers {
			ids = append(ids, o.id+"="+o.name)
		}
		if strings.Join(ids, " ") != "terminal=Terminal ghostty=Ghostty" {
			t.Fatalf("openers %v", ids)
		}
	}
	if got := app.actions(); len(got) != 1 {
		t.Errorf("the app was asked %v", got)
	}
	// The app can't say: no terminal, so a hire goes to tmux alone.
	failing := &scriptedHost{answers: map[string]func(bridgeRequest) (bridgeAnswer, error){
		"terminals": func(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{}, errors.New("gone") },
	}}
	useMacOpeners(failing)
	if openers := installedOpeners(); len(openers) != 0 {
		t.Errorf("openers without the app: %v", openers)
	}
}

// A hire's window: a script only you can read or run, in the hires folder,
// that removes itself and runs the command quoted word by word; the app opens
// the chosen terminal on it, and its window's record comes back. AppleScript
// is the app's alone: nothing here names it.
func TestAMacWindowOpensOnAScriptThroughTheApp(t *testing.T) {
	isolate(t)
	app := withMacOpeners(t, []string{"terminal", "iterm"})
	folder := t.TempDir()
	var iterm hireOpener
	for _, o := range installedOpeners() {
		if o.id == "iterm" {
			iterm = o
		}
	}
	if iterm.open == nil {
		t.Fatal("no iTerm2 opener")
	}
	var script string
	app.answers["open"] = func(r bridgeRequest) (bridgeAnswer, error) {
		script = r.Script
		data, err := os.ReadFile(r.Script)
		info, _ := os.Stat(r.Script)
		if err != nil || info.Mode().Perm() != 0o700 || filepath.Dir(r.Script) != filepath.Join(supportDir, "hires") ||
			r.Terminal != "iterm" || r.Folder != folder {
			t.Errorf("opened %+v (%v)", r, err)
		}
		want := "#!/bin/sh\nrm -f -- \"$0\"\nexec '/usr/bin/tmux' '-S' '/s/its socket' 'attach' '-t' '=hire'\n"
		if testCopyPort() != 0 {
			want = ""
		}
		if want != "" && string(data) != want {
			t.Errorf("the script:\n%s", data)
		}
		return bridgeAnswer{OK: true, Tab: "iterm:AB12", Window: "iterm:AB12"}, nil
	}
	window, err := showWindow(iterm, folder, []string{"/usr/bin/tmux", "-S", "/s/its socket", "attach", "-t", "=hire"})
	if err != nil || window != "iterm:AB12" {
		t.Fatalf("%q %v", window, err)
	}
	if !slices.Contains(app.actions(), "open") || script == "" {
		t.Errorf("the app was asked %v", app.actions())
	}
	// The window didn't open: its script goes, as nothing ran it.
	app.answers["open"] = func(r bridgeRequest) (bridgeAnswer, error) {
		script = r.Script
		return bridgeAnswer{OK: false, Error: "The window didn't open."}, nil
	}
	if _, err := showWindow(iterm, folder, []string{"true"}); err == nil {
		t.Fatal("a window that didn't open was taken as open")
	}
	if _, err := os.Stat(script); err == nil {
		t.Error("the script of a window that didn't open was left")
	}
}

// A test copy's window points the smolcorp command at the copy: the terminal's
// environment is its own, not the engine's.
func TestATestCopysWindowTalksToTheCopy(t *testing.T) {
	lines, err := windowScriptLines([]string{"smolcorp", "run", "--hire", "AB"}, 7779, "/Users/me/test support")
	if err != nil {
		t.Fatal(err)
	}
	want := "#!/bin/sh\nrm -f -- \"$0\"\nexport SMOLCORP_PORT='7779'\nexport SMOLCORP_TEST_SUPPORT_DIR='/Users/me/test support'\nexec 'smolcorp' 'run' '--hire' 'AB'\n"
	if lines != want {
		t.Errorf("got\n%s\nwant\n%s", lines, want)
	}
	if _, err := windowScriptLines(nil, 0, ""); err == nil {
		t.Error("an empty command was written")
	}
}

// An engine hire's window, opened through the app, is recorded for the hire,
// as the Swift engine's Deck.hireWindows; and so is "Open a window" from Setup.
func TestAnEngineHiresMacWindowIsRecorded(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	app := withMacOpeners(t, []string{"terminal"})
	d.mu.Lock()
	d.config.HireTerminal = "terminal"
	d.mu.Unlock()
	if status, message := d.Perform("hire", "", map[string][]string{"role": {"janitor"}}); status != 204 {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 {
		t.Fatalf("registry %+v", got)
	}
	if windows := windowsOfHire(got[0].ID); len(windows) != 1 || windows[0] != "tty:/dev/ttys004#w1234" {
		t.Errorf("windows %v (app asked %v)", windows, app.actions())
	}
	forgetHireWindows(got[0].ID, []string{"tty:/dev/ttys004#w1234"})
	if windows := windowsOfHire(got[0].ID); len(windows) != 0 {
		t.Errorf("forgotten, yet %v", windows)
	}
}
