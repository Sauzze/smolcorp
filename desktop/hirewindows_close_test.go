//go:build !windows

package desktop

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// closerStandIn stands in for the Mac app's terminals: what each recorded window shows, and what closing it comes to.
type closerStandIn struct {
	shown  map[string][2]string // window → state, tty
	left   map[string]string    // window → why it's left open; closed if not here
	looked []string
	closed []string
}

func (c *closerStandIn) look(window string) (string, string) {
	c.looked = append(c.looked, window)
	if s, ok := c.shown[window]; ok {
		return s[0], s[1]
	}
	return "unknown", ""
}

func (c *closerStandIn) close(window, tty string) (bool, string) {
	c.closed = append(c.closed, window+" "+tty)
	if why, ok := c.left[window]; ok {
		return false, why
	}
	return true, ""
}

// M4b: Agent windows switched to the background under the Mac app close the windows it opened for each hire, as
// its Swift engine's HireWindows.toBackground: each hire's viewers detached on its own socket and session (never
// ended), then each recorded window that was among them closed, by its record. A window showing something else,
// one its terminal couldn't be asked about, and one that wasn't opened by Smolcorp are left and said; a window
// closed or gone is forgotten; nothing is asked of a hire without tmux of its own.
func TestUnderTheMacAppSwitchingToTheBackgroundClosesTheWindowsItOpened(t *testing.T) {
	shortSupport(t)
	port := testCopyPort()
	own := func(id, name string) Hire {
		socket := hireOwnSocket(id, port)
		listenAt(t, socket)
		return Hire{ID: id, Name: name, State: "working", TmuxSocket: socket}
	}
	ann, bob := own("ab12cd34ef01", "Ann"), own("ab12cd34ef02", "Bob")
	bob.Nickname = "Rex"
	stray := Hire{ID: "ab12cd34ef03", Name: "Cy", State: "working", TmuxSocket: filepath.Join(supportDir, "default")}
	plain := Hire{ID: "ab12cd34ef04", Name: "Dee", State: "working"}
	fired := own("ab12cd34ef05", "Eve")
	fired.State = "fired"
	for id, windows := range map[string][]string{
		ann.ID:   {"tty:/dev/ttys004#w12", "iterm:AB", "ghostty-id:G1", "tty:/dev/ttys008#w13"},
		bob.ID:   {"tty:/dev/ttys009#w20"},
		stray.ID: {"iterm:CD"},
		fired.ID: {"iterm:EF"},
	} {
		for _, w := range windows {
			recordHireWindow(id, w)
		}
		t.Cleanup(func() { forgetHireWindows(id, windowsOfHire(id)) })
	}
	closer := &closerStandIn{
		shown: map[string][2]string{
			"tty:/dev/ttys004#w12": {"open", "/dev/ttys004"}, "iterm:AB": {"open", "/dev/ttys050"},
			"tty:/dev/ttys008#w13": {"gone", ""}, "tty:/dev/ttys009#w20": {"open", "/dev/ttys009"},
		},
		left: map[string]string{"tty:/dev/ttys009#w20": "something still runs in it"},
	}
	tmux := newRecorder()
	tmux.output = "/dev/ttys004\n/dev/ttys009\n/dev/ttys010\n"
	said := hiresToBackgroundClosing([]Hire{ann, bob, stray, plain, fired}, closer, tmux.run)
	want := []string{
		"Ann: detached; a window Smolcorp opened for it shows something else now, so it's left; " +
			"Smolcorp couldn't ask Ghostty about the window it opened for it, so it's left; " +
			"its window wasn't opened by Smolcorp while it ran, so it's left open.",
		"Rex: detached; its window is left open: something still runs in it; " +
			"2 windows weren't opened by Smolcorp while it ran, so they're left open.",
		"Cy's tmux couldn't be reached, so its window is left.",
		"Dee runs in its window without tmux, so it stays there.",
	}
	if !reflect.DeepEqual(said, want) {
		t.Errorf("said:\n%q\nwant:\n%q", said, want)
	}
	if want := []string{"tty:/dev/ttys004#w12", "iterm:AB", "ghostty-id:G1", "tty:/dev/ttys008#w13", "tty:/dev/ttys009#w20"}; !reflect.DeepEqual(closer.looked, want) {
		t.Errorf("looked at %q, want only the hires' own records", closer.looked)
	}
	if want := []string{"tty:/dev/ttys004#w12 /dev/ttys004", "tty:/dev/ttys009#w20 /dev/ttys009"}; !reflect.DeepEqual(closer.closed, want) {
		t.Errorf("closed %q, want only windows among the viewers", closer.closed)
	}
	// Closed and gone are forgotten; the rest are kept, and so are those of hires not switched.
	if got := windowsOfHire(ann.ID); !reflect.DeepEqual(got, []string{"iterm:AB", "ghostty-id:G1"}) {
		t.Errorf("Ann's records left: %q", got)
	}
	if got := windowsOfHire(bob.ID); len(got) != 1 || len(windowsOfHire(stray.ID)) != 1 || len(windowsOfHire(fired.ID)) != 1 {
		t.Errorf("records left: Bob %q, Cy %q, Eve %q", got, windowsOfHire(stray.ID), windowsOfHire(fired.ID))
	}
	// Only listed and detached, on each hire's own socket and session: never ended.
	for _, call := range tmux.asked {
		if verb := call[2]; verb != "list-clients" && verb != "detach-client" || !slices.Contains(call, "=hire") ||
			call[1] != ann.TmuxSocket && call[1] != bob.TmuxSocket {
			t.Errorf("tmux was asked %q", call)
		}
	}
	if len(tmux.asked) != 4 {
		t.Errorf("tmux calls %q, want a list and a detach for each of Ann and Bob", tmux.asked)
	}

	// A tmux that can't say who's watching: nothing detached, nothing closed.
	quiet, closer2 := newRecorder(), &closerStandIn{}
	quiet.status = 1
	if said := hiresToBackgroundClosing([]Hire{bob}, closer2, quiet.run); !reflect.DeepEqual(said,
		[]string{"Rex's tmux couldn't be asked who's watching it, so its window is left."}) || len(quiet.asked) != 1 || len(closer2.looked) != 0 {
		t.Errorf("tmux couldn't say: %q, calls %q, looked %q", said, quiet.asked, closer2.looked)
	}
}

// The bridge's answers, read as the Mac's Closer gives them: a tty that isn't one, an app that said no and an
// answer it doesn't know are "unknown" (nothing closed); a no to a close is "the terminal didn't close it".
func TestTheMacAppsWindowAnswersAreReadSafely(t *testing.T) {
	for _, c := range []struct {
		answer     bridgeAnswer
		err        error
		shown, tty string
	}{
		{bridgeAnswer{OK: true, Shown: "open", TTY: "/dev/ttys004"}, nil, "open", "/dev/ttys004"},
		{bridgeAnswer{OK: true, Shown: "gone"}, nil, "gone", ""},
		{bridgeAnswer{OK: true, Shown: "open", TTY: "/dev/../etc"}, nil, "unknown", ""},
		{bridgeAnswer{OK: true, Shown: "maybe"}, nil, "unknown", ""},
		{bridgeAnswer{OK: false, Error: "Not a window Smolcorp opened for the engine."}, nil, "unknown", ""},
		{bridgeAnswer{}, errors.New("the app didn't answer"), "unknown", ""},
	} {
		fake := &fakeHost{answer: c.answer, err: c.err}
		if shown, tty := hostLookWindow(fake, "iterm:AB"); shown != c.shown || tty != c.tty {
			t.Errorf("%+v: %q %q", c.answer, shown, tty)
		}
		if len(fake.asked) != 1 || fake.asked[0].Action != "window-look" || fake.asked[0].Window != "iterm:AB" {
			t.Errorf("asked %+v", fake.asked)
		}
	}
	for _, c := range []struct {
		answer bridgeAnswer
		closed bool
		left   string
	}{
		{bridgeAnswer{OK: true, Closed: true}, true, ""},
		{bridgeAnswer{OK: true, Left: "its window has other tabs"}, false, "its window has other tabs"},
		{bridgeAnswer{OK: true}, false, "the terminal didn't close it"},
		{bridgeAnswer{OK: false, Error: "The terminal didn't answer in time."}, false, "the terminal didn't close it"},
	} {
		fake := &fakeHost{answer: c.answer}
		if closed, left := hostCloseWindow(fake, "iterm:AB", "/dev/ttys004"); closed != c.closed || left != c.left {
			t.Errorf("%+v: %v %q", c.answer, closed, left)
		}
		if len(fake.asked) != 1 || fake.asked[0].Action != "window-close" || fake.asked[0].TTY != "/dev/ttys004" {
			t.Errorf("asked %+v", fake.asked)
		}
	}
}
