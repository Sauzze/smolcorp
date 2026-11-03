package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeWindow is a window whose screen is a script: each look gives the next
// screen (the last one stays), and it records what was typed.
type fakeWindow struct {
	screens []string
	looks   int
	pasted  []string
	submits int
	failCap error
}

func (f *fakeWindow) Capture() (string, error) {
	if f.failCap != nil {
		return "", f.failCap
	}
	i := min(f.looks, len(f.screens)-1)
	f.looks++
	return f.screens[i], nil
}
func (f *fakeWindow) Paste(text string) error { f.pasted = append(f.pasted, text); return nil }
func (f *fakeWindow) Submit() error           { f.submits++; return nil }

func never() bool { return false }
func yes() bool   { return true }

func hireTarget() mailTarget { return mailTarget{Name: "Rev"} }

func TestTheGateAllowsOnlyAnIdleWindow(t *testing.T) {
	for name, want := range map[string]mailVerdict{
		"idle-empty": mailDeliver, "after-decline": mailDeliver,
		"draft": mailWait, "busy-1": mailWait, "permission-menu": mailWait, "trust-menu": mailWait,
	} {
		if got, _ := mailGate(hireTarget(), pane(t, name), false); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
	for _, capture := range []string{"", "\n\n", "│ > │", paneBox("", "❯ ")} {
		if got, _ := mailGate(hireTarget(), capture, false); got == mailDeliver {
			t.Errorf("%q: delivered into a window that can't be told apart", capture)
		}
	}
	// The app knowing the session is asking beats an idle-looking box.
	if got, _ := mailGate(hireTarget(), pane(t, "idle-empty"), true); got != mailWait {
		t.Errorf("deck asking: %v", got)
	}
	// The GM's own window is refused whatever it shows.
	for _, name := range []string{"idle-empty", "after-decline", "draft", "busy-1"} {
		if got, _ := mailGate(mailTarget{GM: true}, pane(t, name), false); got != mailRefuse {
			t.Errorf("GM window showing %s: %v", name, got)
		}
	}
	w := &fakeWindow{screens: []string{pane(t, "idle-empty")}}
	r := mailType(mailTarget{GM: true}, "hi", w, never, yes)
	if r.Outcome != mailCannot || r.Typed || len(w.pasted) != 0 || w.looks != 0 {
		t.Errorf("GM window: %+v, typed %q", r, w.pasted)
	}
}

func TestTheWindowIsLookedAtAgainJustBeforeTyping(t *testing.T) {
	idle, busy, menu := pane(t, "idle-empty"), pane(t, "busy-1"), pane(t, "permission-menu")
	// Idle both times, then the box holds the message: typed, one Return.
	w := &fakeWindow{screens: []string{idle, idle, pane(t, "draft")}}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailSent || len(w.pasted) != 1 || w.submits != 1 || w.looks != 3 {
		t.Errorf("idle: %+v, pasted %q, %d returns, %d looks", r, w.pasted, w.submits, w.looks)
	}
	// Idle at the first look, a question or work at the second: nothing typed.
	for _, changed := range []string{busy, menu, pane(t, "draft")} {
		w := &fakeWindow{screens: []string{idle, changed}}
		r := mailType(hireTarget(), "hi", w, never, yes)
		if r.Outcome != mailWaiting || r.Typed || len(w.pasted) != 0 || w.submits != 0 || w.looks != 2 {
			t.Errorf("changed screen: %+v, pasted %q, %d looks", r, w.pasted, w.looks)
		}
	}
	// The app learns of a question between the looks.
	asks := 0
	w = &fakeWindow{screens: []string{idle}}
	if r := mailType(hireTarget(), "hi", w, func() bool { asks++; return asks > 1 }, yes); r.Outcome != mailWaiting || len(w.pasted) != 0 {
		t.Errorf("deck asking at the second look: %+v", r)
	}
	// A busy window never reaches the count of a try.
	counted := false
	w = &fakeWindow{screens: []string{busy}}
	mailType(hireTarget(), "hi", w, never, func() bool { counted = true; return true })
	if counted {
		t.Errorf("a waiting window was counted as a try")
	}
	// A window that can't be looked at is trouble, not typed.
	w = &fakeWindow{failCap: errors.New("no tmux")}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailTrouble || r.Typed {
		t.Errorf("no capture: %+v", r)
	}
	// A try that can't be written down types nothing.
	w = &fakeWindow{screens: []string{idle}}
	if r := mailType(hireTarget(), "hi", w, never, never); r.Typed || len(w.pasted) != 0 {
		t.Errorf("uncounted try typed: %+v", r)
	}
	// The app starts asking after the paste, before Return: the box looks
	// like a draft, but no Return (hard rules 1 and 2).
	w = &fakeWindow{screens: []string{idle, idle, pane(t, "draft")}}
	if r := mailType(hireTarget(), "hi", w, func() bool { return len(w.pasted) > 0 }, yes); r.Outcome != mailCannot || !r.Typed || w.submits != 0 {
		t.Errorf("deck asking after the paste: %+v, %d returns", r, w.submits)
	}
	// A question after the paste means no Return.
	w = &fakeWindow{screens: []string{idle, idle, menu}}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailCannot || !r.Typed || w.submits != 0 {
		t.Errorf("menu after the paste: %+v, %d returns", r, w.submits)
	}
}

// The Mac's HireTmuxTests.testControlCharactersAreNotPasted, and the one it
// allows: a line feed and a tab.
func TestControlCharactersAreNeverPastedAndNotCounted(t *testing.T) {
	for _, text := range []string{"a\x1b[201~\rb", "a\x1bb", "a\rb", "a\x9bb", "a\xffb", "a\xc3b", "a\x07b", "a\u0085b", "a\x00b", "a\x7fb"} {
		counted := false
		w := &fakeWindow{screens: []string{pane(t, "idle-empty")}}
		r := mailType(hireTarget(), text, w, never, func() bool { counted = true; return true })
		if r.Outcome != mailCannot || r.Typed || len(w.pasted) != 0 || counted || w.looks != 0 {
			t.Errorf("%q: %+v, pasted %q, counted %v", text, r, w.pasted, counted)
		}
	}
	w := &fakeWindow{screens: []string{pane(t, "idle-empty"), pane(t, "idle-empty"), pane(t, "draft")}}
	if r := mailType(hireTarget(), "a\nb\tc", w, never, yes); r.Outcome != mailSent {
		t.Errorf("line feed and tab: %+v", r)
	}
}

// Every pane fixture, so a new one is covered: only the idle ones may be typed
// into; every other is refused (it waits).
func TestEveryPaneFixtureIsRefusedUnlessIdle(t *testing.T) {
	idle := map[string]bool{"idle-empty": true, "after-decline": true}
	files, err := filepath.Glob(filepath.Join("..", "Tests", "fixtures", "panes", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no pane fixtures found: %v", err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".txt")
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := mailGate(hireTarget(), string(data), false)
		if idle[name] && got != mailDeliver {
			t.Errorf("%s: %v, want deliver", name, got)
		}
		if !idle[name] && got == mailDeliver {
			t.Errorf("%s: delivered into a window that isn't idle", name)
		}
	}
}
