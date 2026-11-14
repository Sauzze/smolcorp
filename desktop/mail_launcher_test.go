package desktop

import (
	"encoding/json"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// A fake launcher: the real follow() and launcherScreen, fed by the commands the deck sends down the
// launcher's stream, answering looks through the real route on an httptest server. What the agent would
// do with the keys is the test's: react is called with each write and draws on the screen.
type fakeLauncher struct {
	t      *testing.T
	d      *Deck
	id     string
	tab    *launcherTab
	screen *launcherScreen

	mu     sync.Mutex
	writes []string
	// react is what the fake agent does with a write.
	react func(f *fakeLauncher, write string)
}

func (f *fakeLauncher) Write(p []byte) (int, error) {
	f.mu.Lock()
	f.writes = append(f.writes, string(p))
	react := f.react
	f.mu.Unlock()
	if react != nil {
		react(f, string(p))
	}
	return len(p), nil
}

func (f *fakeLauncher) written() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

// writtenAfter is what was written once n writes have come (the launcher writes on its own goroutine),
// or what there is after a moment.
func (f *fakeLauncher) writtenAfter(n int) []string {
	for i := 0; i < 200 && len(f.written()) < n; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	return f.written()
}

// show redraws the whole screen as the capture, after what the agent sent first.
func (f *fakeLauncher) show(before, capture string) {
	f.screen.Write([]byte(before + "\x1b[2J\x1b[H" + ptyOutput(capture)))
}

// newFakeLauncher connects a launcher in folder showing the fixture, with bracketed paste on or off.
func newFakeLauncher(t *testing.T, d *Deck, id, folder, role, shows string, bracketed bool) *fakeLauncher {
	t.Helper()
	return newFakeLauncherAs(t, d, id, folder, role, "office-hire-token", shows, bracketed)
}

// newFakeLauncherAs is newFakeLauncher with the hire token it was started with ("" is a plain launcher).
func newFakeLauncherAs(t *testing.T, d *Deck, id, folder, role, hire, shows string, bracketed bool) *fakeLauncher {
	t.Helper()
	tab := d.launcherConnected(id, Claude, folder, role, hire)
	if tab == nil {
		t.Fatal("launcher not connected")
	}
	f := &fakeLauncher{t: t, d: d, id: id, tab: tab, screen: newLauncherScreen(60, 200, true)}
	mode := ""
	if bracketed {
		mode = "\x1b[?2004h"
	}
	f.show(mode, pane(t, shows))
	server := newLocalServer(t, d)
	pipe, feed := io.Pipe()
	t.Cleanup(func() { feed.Close() })
	go func() {
		encoder := json.NewEncoder(feed)
		encoder.Encode(launcherCommand{Secret: tab.screenSecret})
		for {
			select {
			case command := <-tab.commands:
				if encoder.Encode(command) != nil {
					return
				}
			case <-tab.gone:
				return
			}
		}
	}()
	cursor := &cursorMode{bracketed: f.screen.Bracketed}
	go followWith(pipe, f, cursor, func(proof, number string) {
		postScreenAnswer(server.Client(), server.URL, d.access.HookKey, proof, id, number, f.screen)
	}, func(proof, number, outcome string) {
		postPasteAnswer(server.Client(), server.URL, d.access.HookKey, proof, id, number, outcome, f.screen)
	}, func() {})
	return f
}

// takesTheDraft is an agent that shows what is pasted as a draft, and a Return as an empty box.
func takesTheDraft(t *testing.T) func(*fakeLauncher, string) {
	draft, idle := pane(t, "draft"), pane(t, "idle-empty")
	return func(f *fakeLauncher, write string) {
		if write == keyBytes[Enter] {
			f.show("", idle)
		} else {
			f.show("", draft)
		}
	}
}

func launcherHire() Hire {
	return Hire{ID: "abcdef123456", Name: "Rev", Role: "reviewer", Folder: "/work/api", State: "working"}
}

func noPause(time.Duration) {}

func counting(n *int) func() bool { return func() bool { *n++; return true } }

func TestIdleWindowIsPastedThenEnterAndNothingElse(t *testing.T) {
	for _, bracketed := range []bool{true, false} {
		d := newTestDeck(t)
		f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", bracketed)
		f.react = takesTheDraft(t)
		commits := 0
		r := d.mailTypeIntoLauncher(launcherHire(), "Please review T7", counting(&commits), noPause)
		if r.Outcome != mailSent || !r.Typed || r.Note != "" || commits != 1 {
			t.Fatalf("bracketed %v: %+v, %d commits", bracketed, r, commits)
		}
		want := "Please review T7"
		if bracketed {
			want = pasteStart + want + pasteEnd
		}
		if got := f.writtenAfter(2); len(got) != 2 || got[0] != want || got[1] != keyBytes[Enter] {
			t.Errorf("bracketed %v: written %q, want the paste %q then Enter", bracketed, got, want)
		}
	}
}

func TestMultiLineGoesWholeInABracketAndAsOneLineWithoutOne(t *testing.T) {
	text := "Line one\n\nline\ttwo\nthree"
	d := newTestDeck(t)
	on := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	on.react = takesTheDraft(t)
	if r := d.mailTypeIntoLauncher(launcherHire(), text, yes, noPause); r.Outcome != mailSent || r.Note != "" {
		t.Fatalf("bracketed: %+v", r)
	}
	if got := on.written(); got[0] != pasteStart+text+pasteEnd {
		t.Errorf("bracketed paste wrote %q", got[0])
	}

	d = newTestDeck(t)
	off := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", false)
	off.react = takesTheDraft(t)
	r := d.mailTypeIntoLauncher(launcherHire(), text, yes, noPause)
	if r.Outcome != mailSent || !strings.Contains(r.Note, "Sent as one line") {
		t.Fatalf("not bracketed: %+v", r)
	}
	if got := off.writtenAfter(2); len(got) != 2 || got[0] != "Line one / line two / three" || got[1] != keyBytes[Enter] {
		t.Errorf("one-line fallback wrote %q", got)
	}
}

func TestOnlyAnIdleWindowIsTypedInto(t *testing.T) {
	for name, shows := range map[string]string{"busy": "busy-1", "menu": "permission-menu", "trust menu": "trust-menu", "draft": "draft"} {
		d := newTestDeck(t)
		f := newFakeLauncher(t, d, "L1", "/work/api", "", shows, true)
		commits := 0
		r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause)
		if r.Outcome != mailWaiting || r.Typed || commits != 0 || len(f.written()) != 0 {
			t.Errorf("%s: %+v, %d commits, wrote %q", name, r, commits, f.written())
		}
	}
	// A screen the launcher can't read (the alternate screen) is unknown, which waits too.
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	f.screen.Write([]byte("\x1b[?1049h"))
	commits := 0
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailWaiting || commits != 0 || len(f.written()) != 0 {
		t.Errorf("unknown: %+v, %d commits, wrote %q", r, commits, f.written())
	}
}

func TestTheDecksOwnQuestionWaitsOverAnIdleBox(t *testing.T) {
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	r := mailTypeViaLauncher(deckLauncherPort{d: d, id: "L1"}, "Rev", "hi", yes, yes, noPause)
	if r.Outcome != mailWaiting || len(f.written()) != 0 {
		t.Errorf("asking: %+v, wrote %q", r, f.written())
	}
}

func TestIfTheBoxDoesNotHoldADraftAfterThePasteNoEnterIsSent(t *testing.T) {
	for name, react := range map[string]func(*fakeLauncher, string){
		"nothing shows": nil, // still the idle box
		"a menu opens": func(f *fakeLauncher, _ string) {
			f.show("", pane(t, "permission-menu"))
		},
		"busy": func(f *fakeLauncher, _ string) { f.show("", pane(t, "busy-1")) },
	} {
		d := newTestDeck(t)
		f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
		f.react = react
		r := d.mailTypeIntoLauncher(launcherHire(), "hi", yes, noPause)
		if r.Outcome != mailCannot || !r.Typed || !strings.Contains(r.Note, "put in its box but not sent") {
			t.Errorf("%s: %+v", name, r)
		}
		for _, w := range f.written() {
			if w == keyBytes[Enter] {
				t.Errorf("%s: Enter was pressed", name)
			}
		}
	}
	// The deck beginning to ask during the settle blocks Return as well.
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	asked := false
	port := deckLauncherPort{d: d, id: "L1"}
	pasted := func() bool { return asked }
	f.react = func(f *fakeLauncher, w string) { asked = true; takesTheDraft(t)(f, w) }
	if r := mailTypeViaLauncher(port, "Rev", "hi", pasted, yes, noPause); r.Outcome != mailCannot || !r.Typed {
		t.Errorf("asking after the paste: %+v", r)
	}
	for _, w := range f.written() {
		if w == keyBytes[Enter] {
			t.Error("Enter was pressed at a question")
		}
	}
}

func TestACommitThatFailsTypesNothing(t *testing.T) {
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	r := d.mailTypeIntoLauncher(launcherHire(), "hi", func() bool { return false }, noPause)
	if r.Outcome != mailTrouble || r.Typed || len(f.written()) != 0 {
		t.Errorf("%+v, wrote %q", r, f.written())
	}
}

func TestNoLauncherOrNoSureOneIsTroubleAndTypesNothing(t *testing.T) {
	d := newTestDeck(t)
	commits := 0
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || r.Typed || commits != 0 {
		t.Errorf("no launcher: %+v", r)
	}
	// Another folder's launcher is not the hire's.
	other := newFakeLauncher(t, d, "L2", "/work/web", "", "idle-empty", true)
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || commits != 0 {
		t.Errorf("another folder's launcher: %+v", r)
	}
	// The GM's launcher in the hire's folder is never the hire's.
	gm := newFakeLauncher(t, d, "L3", "/work/api", gmRole, "idle-empty", true)
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || commits != 0 {
		t.Errorf("the GM's launcher: %+v", r)
	}
	// Two launchers in the folder: which one is the hire's can't be told.
	a := newFakeLauncher(t, d, "L4", "/work/api", "", "idle-empty", true)
	b := newFakeLauncher(t, d, "L5", "/work/api", "", "idle-empty", true)
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || commits != 0 {
		t.Errorf("two launchers: %+v", r)
	}
	for _, f := range []*fakeLauncher{other, gm, a, b} {
		if len(f.written()) != 0 {
			t.Errorf("%s was written to: %q", f.id, f.written())
		}
	}
}

func TestAPlainLauncherInTheHiresFolderIsNotTheHiresWindow(t *testing.T) {
	d := newTestDeck(t)
	owner := newFakeLauncherAs(t, d, "L1", "/work/api", "", "", "idle-empty", true)
	commits := 0
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || r.Typed || commits != 0 {
		t.Errorf("the owner's own launcher: %+v", r)
	}
	if len(owner.written()) != 0 {
		t.Errorf("the owner's launcher was written to: %q", owner.written())
	}
}

func TestALauncherThatIsGoneOrSilentIsTroubleNotIdle(t *testing.T) {
	old := mailLaunchLookWait
	mailLaunchLookWait = 150 * time.Millisecond
	t.Cleanup(func() { mailLaunchLookWait = old })

	// Connected but never answering (an old launcher that ignores look).
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "office-hire-token")
	commits := 0
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || commits != 0 {
		t.Errorf("silent: %+v", r)
	}
	// Gone while the look waits.
	go func() { time.Sleep(20 * time.Millisecond); d.launcherGone("L1", tab) }()
	if r := d.mailTypeIntoLauncher(launcherHire(), "hi", counting(&commits), noPause); r.Outcome != mailTrouble || commits != 0 {
		t.Errorf("gone: %+v", r)
	}
}

func TestTheLauncherRefusesWhatIsNotSafeToPaste(t *testing.T) {
	// The command is the last gate: it writes nothing for text that could end a bracket early, or a line
	// feed where the agent isn't taking bracketed paste, and never an Enter.
	for _, c := range []struct {
		name      string
		bracketed bool
		text      string
		want      string
	}{
		{"plain", false, "hello", "hello"},
		{"bracketed", true, "a\nb", pasteStart + "a\nb" + pasteEnd},
		{"newline, no bracket", false, "a\nb", ""},
		{"tab, no bracket", false, "a\tb", ""},
		{"escape", true, "a\x1b[201~b", ""},
		{"carriage return", true, "a\rb", ""},
		{"C1 control", true, "a\u009bb", ""},
		{"empty", true, "", ""},
	} {
		var typed strings.Builder
		cursor := &cursorMode{bracketed: func() bool { return c.bracketed }}
		line, _ := json.Marshal(launcherCommand{Paste: c.text})
		follow(strings.NewReader(string(line)+"\n"), &typed, cursor, nil, func() {})
		if typed.String() != c.want {
			t.Errorf("%s: wrote %q, want %q", c.name, typed.String(), c.want)
		}
	}
	// No screen to ask: not bracketed.
	var typed strings.Builder
	follow(strings.NewReader(`{"paste":"a\nb"}`+"\n"), &typed, &cursorMode{}, nil, func() {})
	if typed.String() != "" {
		t.Errorf("no screen: wrote %q", typed.String())
	}
}

func TestThePhonesOwnTypeAndPressAreAsTheyWere(t *testing.T) {
	var typed strings.Builder
	follow(strings.NewReader(`{"type":"hello"}`+"\n"+`{"press":{"key":"enter","times":1}}`+"\n"), &typed, &cursorMode{bracketed: func() bool { return true }}, nil, func() {})
	if typed.String() != "hello"+keyBytes[Enter]+keyBytes[Enter] {
		t.Errorf("wrote %q", typed.String())
	}
}

func TestTheLookAnswersWhetherBracketedPasteIsOn(t *testing.T) {
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	if state, bracketed, err := d.lookAtLauncherPaste("L1", 5*time.Second); err != nil || state != PaneIdle || !bracketed {
		t.Errorf("on: %v %v %v", state, bracketed, err)
	}
	f.screen.Write([]byte("\x1b[?2004l"))
	if state, bracketed, err := d.lookAtLauncherPaste("L1", 5*time.Second); err != nil || state != PaneIdle || bracketed {
		t.Errorf("off: %v %v %v", state, bracketed, err)
	}
	// An unknown screen says nothing about it.
	f.screen.Write([]byte("\x1b[?2004h\x1b[?1049h"))
	if state, bracketed, err := d.lookAtLauncherPaste("L1", 5*time.Second); err != nil || state != PaneUnknown || bracketed {
		t.Errorf("unknown: %v %v %v", state, bracketed, err)
	}
	// The old look still answers the state alone.
	if state, err := d.lookAtLauncher("L1", 5*time.Second); err != nil || state != PaneUnknown {
		t.Errorf("lookAtLauncher: %v %v", state, err)
	}
}

func TestTheScreenTracksBracketedPasteWithoutChangingWhatItReads(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	plain := drawn(t, 60, 200, idle)
	if plain.Bracketed() {
		t.Error("bracketed without being asked")
	}
	for name, c := range map[string]struct {
		out  string
		want bool
	}{
		"on":                {"\x1b[?2004h" + idle, true},
		"on then off":       {"\x1b[?2004h\x1b[?2004l" + idle, false},
		"off then on":       {"\x1b[?2004l\x1b[?2004h" + idle, true},
		"among others":      {"\x1b[?25;2004h" + idle, true},
		"between the lines": {"\x1b[?2004h" + idle + "\x1b[?25l", true},
	} {
		s := drawn(t, 60, 200, c.out)
		if s.Bracketed() != c.want {
			t.Errorf("%s: bracketed %v, want %v", name, s.Bracketed(), c.want)
		}
		if s.Text() != plain.Text() || s.Unknown() || s.PaneState() != plain.PaneState() {
			t.Errorf("%s: the mode changed what the screen reads (%v, unknown %v)", name, s.PaneState(), s.Unknown())
		}
	}
	// Split across two writes, and cleared with the screen's reset.
	s := newTextScreen(60, 200)
	s.Write([]byte("\x1b[?20"))
	s.Write([]byte("04h"))
	if !s.Bracketed() {
		t.Error("a switch split across writes was missed")
	}
	s.Reset(60, 200)
	if s.Bracketed() {
		t.Error("Reset kept the mode")
	}
}

func TestThePumpDeliversThroughALauncherFromStartToEnd(t *testing.T) {
	r := newPumpRig(t)
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	f.react = takesTheDraft(t)
	r.hires = []Hire{launcherHire()}
	r.pump.typeIn = func(h Hire, text string, commit func() bool) mailResult {
		return d.mailTypeIntoLauncher(h, text, commit, noPause)
	}
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "Please review T7", "m1"))
	r.pump.round()
	if m := r.msg("m1"); m.State != mailDelivered || m.Attempts != 1 {
		t.Fatalf("after one round: %+v", m)
	}
	if got := f.writtenAfter(2); len(got) != 2 || !strings.HasPrefix(got[0], pasteStart+"— from the GM") || !strings.HasSuffix(got[0], "Please review T7"+pasteEnd) || got[1] != keyBytes[Enter] {
		t.Errorf("wrote %q", got)
	}

	// At a menu it waits, with nothing typed and no try counted; when the menu is gone, the next round sends it.
	before := len(f.written())
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "second", "m2"))
	f.show("", pane(t, "permission-menu"))
	r.pump.round()
	if m := r.msg("m2"); m.State == mailDelivered || m.Attempts != 0 || len(f.written()) != before {
		t.Errorf("at a menu: %+v, wrote %q", m, f.written()[before:])
	}
	f.show("", pane(t, "idle-empty"))
	r.clock.advance(mailPumpEvery)
	r.pump.round()
	if m := r.msg("m2"); m.State != mailDelivered || m.Attempts != 1 {
		t.Errorf("after the menu: %+v", m)
	}
}

func TestSetupStateSaysMailIsBetaOnWindowsOnly(t *testing.T) {
	d := newTestDeck(t)
	state := (&router{deck: d}).setupStateFor(callerPage)
	beta, ok := state["mailBeta"].(bool)
	if !ok || beta != (runtime.GOOS == "windows") {
		t.Errorf("mailBeta %v (%T) on %s", state["mailBeta"], state["mailBeta"], runtime.GOOS)
	}
}

func TestAnAnswerNotSureOfItsScreenNeverSaysBracketed(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	go func() {
		command := <-tab.commands
		d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: command.Look, State: "idle", Unknown: true, Bracketed: true})
	}()
	if state, bracketed, err := d.lookAtLauncherPaste("L1", 5*time.Second); err != nil || state != PaneUnknown || bracketed {
		t.Errorf("%v %v %v", state, bracketed, err)
	}
}

// Two hires in one folder: the open office window may be the other one's, so neither gets mail
// typed into it. A hire with a worktree is matched by its worktree only, never by its folder.
func TestAWindowAnotherHireMayOwnIsNotTyped(t *testing.T) {
	d := newTestDeck(t)
	a, b := launcherHire(), launcherHire()
	b.ID, b.Name = "bbbbbb123456", "Bo"
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": []Hire{a, b}})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	theirs := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	commits := 0
	if r := d.mailTypeIntoLauncher(a, "hi", counting(&commits), noPause); r.Outcome != mailTrouble || r.Typed || commits != 0 {
		t.Errorf("a window two hires share: %+v", r)
	}
	if len(theirs.written()) != 0 {
		t.Errorf("another hire's window was written to: %q", theirs.written())
	}
	// Once B is fired, the window can only be A's.
	b.State = "fired"
	data, _ = json.Marshal(map[string]any{"version": 1, "hires": []Hire{a, b}})
	writeRegistryFile(registryPath(), data, 0o600)
	if id, ok := d.hireLauncherID(a); !ok || id != "L1" {
		t.Errorf("with the other hire fired: %q %v", id, ok)
	}
	// A hire with a worktree isn't matched by the folder it was hired from.
	a.Worktree = "/work/api-hires/abcdef123456"
	if _, ok := d.hireLauncherID(a); ok {
		t.Error("a hire with a worktree was matched by its folder")
	}
}
