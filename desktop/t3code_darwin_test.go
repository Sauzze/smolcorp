package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

// C4 A1: Open in T3 Code on the Mac's Go engine, as the Swift engine does it: the app opened by the Mac app (by
// its bundle id), then `t3 app <folder>`, which this engine runs itself. Nothing real is opened or run: the Mac
// app is a stand-in, and t3 is a file that's never started (the machine's Run is the test's).
func TestT3OnAMacOpenOpensTheAppThroughTheMacAppThenRunsT3App(t *testing.T) {
	mac := withT3Mac(t, true)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(home, "repo-hires", "ada")
	bin := filepath.Join(home, ".local", "bin")
	for _, dir := range []string{folder, bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "t3"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var runs [][]string
	m := defaultT3Machine()
	m.Home, m.Path, m.Pause, m.Now = home, "", func(time.Duration) {}, time.Now
	m.Run = func(program string, args []string, _ time.Duration) t3Ran {
		runs = append(runs, args)
		if args[0] == "thread" {
			return t3Ran{Ended: true, Output: "[]"}
		}
		return t3Ran{Ended: true}
	}
	status, said := t3Open(Hire{ID: "abcdef12", Name: "Ada", Worktree: folder}, t3Status{State: t3Found, Version: "v0.1.8"}, m)
	if status != http.StatusOK || !reflect.DeepEqual(said, map[string]any{"ok": true, "opened": "thread"}) {
		t.Fatalf("%d %v", status, said)
	}
	if got := mac.actions(); !slices.Equal(got, []string{"t3-app ", "t3-open-app "}) {
		t.Errorf("the Mac app was asked %q", got)
	}
	if !reflect.DeepEqual(runs, [][]string{{"thread", "list", "--project", folder, "--json"}, {"app", folder}}) {
		t.Errorf("t3 ran %q", runs)
	}
}

// C4 A1: a hire in T3 Code opens on its own thread, by T3 Code's link, handed to the Mac app's opener; t3 isn't run.
func TestT3OnAMacAHireInT3CodeOpensItsThreadThroughTheMacApp(t *testing.T) {
	mac := withT3Mac(t, true)
	m := defaultT3Machine()
	m.Run = func(string, []string, time.Duration) t3Ran { t.Error("t3 ran"); return t3Ran{} }
	h := Hire{ID: "abcdef12", Name: "Ada", Engine: t3Engine, T3Thread: "mcp:abc-1", T3Link: "https://app.t3.codes/threads/local/mcp:abc-1"}
	status, said := t3Open(h, t3Status{State: t3Found, Version: "v0.1.8"}, m)
	if status != http.StatusOK {
		t.Fatalf("%d %v", status, said)
	}
	if got := mac.actions(); !slices.Equal(got, []string{"t3-link t3code://threads/local/mcp%3Aabc-1"}) {
		t.Errorf("the Mac app was asked %q", got)
	}
	// Without the app, a bare core's machine opens nothing of T3 Code's: there's none found to open.
	saved := host
	host = nil
	defer func() { host = saved }()
	if got := m.App(); got != "" {
		t.Errorf("a bare core found %q", got)
	}
}
