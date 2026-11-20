package desktop

import (
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// F1 (RT126SWEEP3 P4): on a Mac a terminal's ask is judged by the processes holding its connection
// (askprogram.go's askSandboxOf), with stand-ins for macOS here; the real one is row B (sweep3p4_darwin_test.go).
func TestTheAskersSandboxIsEveryHoldersAnswer(t *testing.T) {
	const asker, ours = "127.0.0.1:50123", "127.0.0.1:7710"
	look := func(holders map[int]int, unreadable ...int) askHolderLook {
		return askHolderLook{
			list: func() ([]int, error) { return []int{1, 2, 3, 4, 5}, nil },
			holds: func(pid int, from, to *net.TCPAddr) int {
				if from.String() != asker || to.String() != ours {
					t.Errorf("asked about %s to %s", from, to)
				}
				if slices.Contains(unreadable, pid) {
					return -1
				}
				if _, ok := holders[pid]; ok {
					return 1
				}
				return 0
			},
			in: func(pid int) int { return holders[pid] },
		}
	}
	for _, c := range []struct {
		name        string
		remote      string
		holders     map[int]int // pid: 1 in a sandbox, 0 in none, -1 not answered
		unreadable  []int
		want        askSandbox
		listFailure bool
	}{
		{name: "your terminal", remote: asker, holders: map[int]int{3: 0}, want: sandboxOutside},
		{name: "a hire", remote: asker, holders: map[int]int{3: 1}, want: sandboxInside},
		{name: "a hire and a program it passed the socket to", remote: asker, holders: map[int]int{2: 0, 4: 1}, want: sandboxInside},
		{name: "one not answered", remote: asker, holders: map[int]int{2: 0, 4: -1}, want: sandboxUnknown},
		{name: "none found: the asker closed its end (N2)", remote: asker, holders: map[int]int{}, want: sandboxNoAsker},
		{name: "none found, but one unread: refused all the same (B1)", remote: asker, holders: map[int]int{}, unreadable: []int{5}, want: sandboxNoAsker},
		{name: "a process whose files can't be read may hold it too", remote: asker, holders: map[int]int{3: 0}, unreadable: []int{5}, want: sandboxUnknown},
		{name: "one in a sandbox wins over one unread", remote: asker, holders: map[int]int{3: 1}, unreadable: []int{5}, want: sandboxInside},
		{name: "not loopback", remote: "192.168.1.20:50123", holders: map[int]int{3: 0}, want: sandboxUnknown},
		{name: "no address", remote: "", holders: map[int]int{3: 0}, want: sandboxUnknown},
		{name: "no list", remote: asker, holders: map[int]int{3: 0}, listFailure: true, want: sandboxUnknown},
	} {
		l := look(c.holders, c.unreadable...)
		if c.listFailure {
			l.list = func() ([]int, error) { return nil, errors.New("macOS didn't list the processes") }
		}
		if got := askSandboxOf(c.remote, ours, l); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// A holder that ends while it's asked about (its number given again, or none) isn't taken as outside: it wasn't a
	// holder (N2), so with no other the ask has none; beside one outside, that one's answer stands; one that can't be
	// read at its re-check is unread, which counts only beside a holder found (B1): alone, the ask has none.
	goneAt := func(holders map[int]int, again int) askHolderLook {
		l := look(holders)
		asked, holds := 0, l.holds
		l.holds = func(pid int, from, to *net.TCPAddr) int {
			if pid == 3 {
				if asked++; asked > 1 {
					return again
				}
			}
			return holds(pid, from, to)
		}
		return l
	}
	for _, c := range []struct {
		name    string
		holders map[int]int
		again   int
		want    askSandbox
	}{
		{"the only holder, gone before its answer was checked", map[int]int{3: 0}, 0, sandboxNoAsker},
		{"one in a sandbox, gone before its answer was checked", map[int]int{3: 1}, 0, sandboxNoAsker},
		{"one gone beside one outside", map[int]int{2: 0, 3: 0}, 0, sandboxOutside},
		{"the only holder, unread at its re-check", map[int]int{3: 0}, -1, sandboxNoAsker},
		{"one unread at its re-check beside one outside", map[int]int{2: 0, 3: 0}, -1, sandboxUnknown},
	} {
		if got := askSandboxOf(asker, ours, goneAt(c.holders, c.again)); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// F1: an ask from a program macOS says is in a sandbox is refused in plain words and never listed; one macOS can't
// tell is listed with mayBeAHire, its folder shown as ever; one from outside any sandbox is listed as before.
func TestOnAMacAnAskIsJudgedByItsProgramsSandbox(t *testing.T) {
	d := newMacTestDeck(t)
	var said askSandbox
	saved := askingProgram
	askingProgram = func(*http.Request) askAsker { return askAsker{sandbox: said} }
	t.Cleanup(func() { askingProgram = saved })
	said = sandboxInside
	if code, answer := askFromTerminal(t, d, askTool, "/work"); code != http.StatusForbidden || answer != nil {
		t.Errorf("from a sandbox: %d %v", code, answer)
	}
	if shown := shownAsks(t, d); len(shown) != 0 {
		t.Fatalf("from a sandbox, the window lists %v", shown)
	}
	said = sandboxUnknown
	if code, _ := askFromTerminal(t, d, askTool, "/work/app"); code != http.StatusOK {
		t.Fatalf("untold: %d", code)
	}
	said = sandboxOutside
	if code, _ := askFromTerminal(t, d, askTool, "/work/mine"); code != http.StatusOK {
		t.Fatalf("outside: %d", code)
	}
	shown := shownAsks(t, d)
	if len(shown) != 2 || shown[0]["folder"] != "/work/app" || shown[0]["mayBeAHire"] != true ||
		shown[1]["folder"] != "/work/mine" || shown[1]["mayBeAHire"] != nil {
		t.Errorf("the window lists %v", shown)
	}
	// An ask no program holds (N2) is refused too, and listed nowhere.
	said = sandboxNoAsker
	refused := serve(t, d, call{method: "POST", target: terminalAskPath + "?kind=tool&folder=/work/gone",
		host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if refused.Code != http.StatusForbidden || strings.TrimSpace(refused.Body.String()) != askNoAsker {
		t.Errorf("no asker: %d %q", refused.Code, refused.Body.String())
	}
	if shown := shownAsks(t, d); len(shown) != 2 {
		t.Errorf("after an ask no program holds, the window lists %v", shown)
	}
	// The words the fail-first rows check are these.
	for got, want := range map[string]string{askFromASandbox: sweepAskWords, hireFolderPageOnly: sweepFolderPageOnly,
		hireFolderOwnersOnly: sweepFolderOwnersOnly, hiringOffOnly: sweepHiringOffOnly, officeWindowOnly: sweepOfficeWindow,
		askNoAsker: sweepNoAskerWords} {
		if got != want {
			t.Errorf("%q, the rows check %q", got, want)
		}
	}
}

// sweepAskWords is sweep3p4_darwin_test.go's sweepAskFromASandbox, which only a Mac builds.
const sweepAskWords = "This program runs in a sandbox, as Smolcorp's hires do, so Smolcorp doesn't take its ask. " +
	"Run it in your own terminal."
