package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The two plain choices (T796), as on the Mac: saved and loaded, a new install
// asked and an older file left as it was, the T3 Code gate, and switching to
// the background, which only detaches.

func TestTheChoicesAreSavedAndLoaded(t *testing.T) {
	isolate(t)
	config := loadConfig()
	if config.ChoicesAsked || config.T3CodeEnabled || config.HiresInBackground {
		t.Fatalf("a new install: asked %v, T3 Code %v, background %v; want all off", config.ChoicesAsked, config.T3CodeEnabled, config.HiresInBackground)
	}
	config.T3CodeEnabled, config.HiresInBackground, config.ChoicesAsked = true, true, true
	config.save()
	again := loadConfig()
	if !again.T3CodeEnabled || !again.HiresInBackground || !again.ChoicesAsked {
		t.Fatalf("read back: %+v", again)
	}
}

func TestAnOlderConfigKeepsTodaysBehaviourUnasked(t *testing.T) {
	isolate(t)
	if err := os.WriteFile(configFile, []byte(`{"hireFolder": "/work", "hireInTmux": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config := loadConfig()
	if !config.ChoicesAsked || config.T3CodeEnabled || config.HiresInBackground || !config.HireInTmux {
		t.Fatalf("an older file: asked %v, T3 Code %v, background %v, tmux %v", config.ChoicesAsked, config.T3CodeEnabled, config.HiresInBackground, config.HireInTmux)
	}
}

func TestTheT3GateHidesEverythingWhenOff(t *testing.T) {
	config := &Config{ChoicesAsked: true}
	if t3CodeAllowed(config) {
		t.Fatal("the gate is open while off")
	}
	state := choicesState(config, true, func() bool { t.Fatal("T3 Code was looked for while off"); return true })
	if _, said := state["t3Installed"]; said || state["t3Code"] != false {
		t.Fatalf("off: %v", state)
	}
	looked := 0
	config.ChoicesAsked = false
	if state := choicesState(config, true, func() bool { looked++; return true }); state["t3Installed"] != true {
		t.Fatalf("first setup looks, to know whether to ask: %v", state)
	}
	config.ChoicesAsked, config.T3CodeEnabled = true, true
	if !t3CodeAllowed(config) || choicesState(config, true, func() bool { looked++; return false })["t3Installed"] != false || looked != 2 {
		t.Fatalf("on: looked %d times", looked)
	}
}

func TestT3IsFoundAsAFileAndNeverRun(t *testing.T) {
	home := t.TempDir()
	// A full path on the PATH as this system writes one: /usr/bin has no drive, so Windows doesn't take it as one.
	system := "/usr/bin"
	if runtime.GOOS == "windows" {
		system = `C:\Windows\System32`
	}
	folders := t3LookFolders(home, strings.Join([]string{system, "relative"}, string(os.PathListSeparator)))
	if folders[0] != filepath.Join(home, ".local", "bin") || folders[1] != filepath.Join(home, "bin") || folders[2] != filepath.Join(home, ".t3", "bin") ||
		folders[len(folders)-1] != system || slices.Contains(folders, "relative") {
		t.Fatalf("folders %v", folders)
	}
	if t3Installed(home, "", isRegularFile) {
		t.Fatal("found in an empty home")
	}
	if runtime.GOOS == "windows" {
		return
	}
	bin := filepath.Join(home, ".t3", "bin")
	ran := filepath.Join(home, "ran")
	if os.MkdirAll(bin, 0o700) != nil || os.WriteFile(filepath.Join(bin, "t3"), []byte("#!/bin/sh\ntouch '"+ran+"'\n"), 0o700) != nil {
		t.Fatal("couldn't make the fake t3")
	}
	if !t3Installed(home, "", isRegularFile) {
		t.Fatal("not found")
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("t3 was run")
	}
}

func TestBackgroundNeedsTmux(t *testing.T) {
	config := &Config{}
	if _, status, refusal := setChoices(config, url.Values{"background": {"1"}}, false); status != http.StatusBadRequest ||
		!strings.Contains(refusal, "needs tmux") || config.HiresInBackground {
		t.Fatalf("without tmux: %d %q", status, refusal)
	}
	switched, _, _ := setChoices(config, url.Values{"background": {"1"}, "t3": {"1"}, "asked": {"1"}}, true)
	if !switched || !config.HiresInBackground || !config.T3CodeEnabled || !config.ChoicesAsked {
		t.Fatalf("with tmux: switched %v, %+v", switched, config)
	}
	if again, _, _ := setChoices(config, url.Values{"background": {"1"}}, true); again {
		t.Fatal("already in the background: nothing to switch")
	}
	if backgroundHires(config, false) {
		t.Fatal("background without tmux")
	}
}

func TestSwitchingToTheBackgroundOnlyDetaches(t *testing.T) {
	hires := []Hire{
		{ID: "a", Name: "Ada", TmuxSocket: "/s/smolcorp-a", State: "working"},
		{ID: "b", Name: "Bo", Nickname: "Bobby", TmuxSocket: "/s/smolcorp-b", State: "idle"},
		{ID: "c", Name: "Cy", TmuxSocket: "/s/smolcorp-c", State: "fired"},
		{ID: "d", Name: "Di", State: "working"},
		{ID: "e", Name: "Ed", TmuxSocket: "/s/smolcorp-e", State: "working"},
		{ID: "f", Name: "Flo", TmuxSocket: "/elsewhere/default", State: "working"},
		{ID: "g", Name: "Gus", TmuxSocket: "/s/smolcorp-g", State: "working"},
	}
	asked := []string{}
	said := hiresToBackground(hires, func(h Hire) detached {
		asked = append(asked, h.ID)
		switch h.ID {
		case "a":
			return detached{own: true, asked: true, viewers: 1, ok: true}
		case "b":
			return detached{own: true, asked: true, ok: true}
		case "f":
			return detached{}
		case "g":
			return detached{own: true}
		default:
			return detached{own: true, asked: true, viewers: 1}
		}
	})
	if !slices.Equal(asked, []string{"a", "b", "e", "f", "g"}) {
		t.Fatalf("asked %v: a fired hire, or one with no tmux socket, isn't detached", asked)
	}
	want := []string{
		// The window this computer opened is the attach itself, so it ends with it.
		"Ada: detached. A window this computer opened for it closes with it; any other is left open.",
		"Di isn't on a tmux server of its own, so its window is left.",
		"Ed's window couldn't be detached, so it's left.",
		"Flo isn't on a tmux server of its own, so its window is left.",
		"Gus's tmux couldn't be asked who's watching it, so its window is left.",
	}
	if !slices.Equal(said, want) {
		t.Fatalf("said %q", said)
	}
}
