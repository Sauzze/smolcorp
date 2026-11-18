//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// SA-M9: a tmux pane's focus and close, asked of the Mac app, carry its desk as the core recorded it: the guard the
// core's own calls on that socket are told (a checked server's pid, or "unchecked"), the pid and start of the process
// its start made in the pane, and its folder. Never for a pane that isn't an open desk's, nor one whose server the core
// never checked: the app then runs nothing on a desk's socket (TmuxTests, HostBridgeTests).
func TestATmuxPanesFocusAndCloseCarryItsDeskAsTheCoreRecordedIt(t *testing.T) {
	program, _, sandboxed, old := tmuxGateRig(t)
	sandboxed.TmuxPaneStarted, sandboxed.Worktree, sandboxed.Folder = 1794000000000000, "/work/repo-hires/ab12", "/work/repo"
	old.Folder = "/work/old"
	fired := Hire{ID: "gh78", State: "fired", TmuxSocket: deskSocketPath("gh78", 0), TmuxProgram: program, TmuxServer: 6161, TmuxPane: 4343}
	unchecked := Hire{ID: "ij90", State: "working", TmuxSocket: deskSocketPath("ij90", 0), TmuxProgram: program, TmuxPane: 4444}
	writeRegistryHires(t, sandboxed, old, fired, unchecked)
	own := filepath.Join(t.TempDir(), "default")

	asked := func(tab macTab, close bool) *bridgeDesk {
		t.Helper()
		fake := &fixtureHost{answer: bridgeAnswer{OK: true, Fired: "fired"}}
		tab.host = fake
		if close {
			_ = tab.Close(Claude)
		} else {
			_ = tab.Focus()
		}
		if fake.asked == nil {
			t.Fatalf("%s: nothing asked", tab.name)
		}
		return fake.asked.Desk
	}
	pane := func(socket string) macTab {
		return macTab{name: "tmux:%3@" + socket, pane: &commandTab{kind: "tmux", id: "%3", socket: socket}}
	}
	for _, close := range []bool{false, true} {
		if got, want := asked(pane(sandboxed.TmuxSocket), close), (&bridgeDesk{ID: "ab12", Server: "5151", Pane: 4242,
			PaneStarted: 1794000000000000, Folder: "/work/repo-hires/ab12"}); !reflect.DeepEqual(got, want) {
			t.Errorf("a desk in its sandbox (close %v): %+v, want %+v", close, got, want)
		}
		if got, want := asked(pane(old.TmuxSocket), close), (&bridgeDesk{ID: "cd34", Server: tmuxUnchecked, Pane: 4242,
			Folder: "/work/old"}); !reflect.DeepEqual(got, want) {
			t.Errorf("a desk from before the sandbox (close %v): %+v, want %+v", close, got, want)
		}
		for name, socket := range map[string]string{"a fired desk's": fired.TmuxSocket, "a desk whose server was never checked": unchecked.TmuxSocket,
			"a tmux of your own": own, "no desk's at all": deskSocketPath("kl12", 0)} {
			if got := asked(pane(socket), close); got != nil {
				t.Errorf("%s (close %v): %+v", name, close, got)
			}
		}
		// A Terminal tab has no pane, and so no desk.
		if got := asked(macTab{name: "tty:/dev/ttys003"}, close); got != nil {
			t.Errorf("a Terminal tab (close %v): %+v", close, got)
		}
	}
}

// RT126SAM9's condition on SA-M10 (MSYSTACK4): a plain hire's own server (smolcorp-plain-…) has no registry row, so the
// core sends the app its own record of the server it made: told "unchecked", with the pid its start made in the pane,
// when that started, and its folder. Without it the app is sent no desk, runs nothing on a socket named as a hire's,
// and focus and close do nothing on a Mac. Only a socket the core made: one merely named so gets nothing, and neither
// does one the core forgot.
func TestAPlainHiresTmuxPaneCarriesTheCoresOwnRecordOfIt(t *testing.T) {
	d, h, folder := hiringDeck(t, "/usr/bin/tmux")
	d.config.HireInTmux = true
	savedStarted := paneStartedAt
	t.Cleanup(func() { paneStartedAt = savedStarted })
	paneStartedAt = func(pid int) int64 {
		if pid == 4242 {
			return 1794000000000000
		}
		return 0
	}
	t.Cleanup(func() {
		for _, socket := range h.sockets {
			forgetPlainTmux(socket)
		}
	})
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.sockets) != 1 {
		t.Fatalf("hire in tmux: %d %s, sockets %q", status, message, h.sockets)
	}
	socket := h.sockets[0]
	id := strings.TrimPrefix(filepath.Base(socket), hireSocketPrefix)
	if !strings.HasPrefix(id, "plain-") {
		t.Fatalf("the plain hire's socket %s", socket)
	}

	asked := func(socket string, close bool) *bridgeDesk {
		t.Helper()
		fake := &fixtureHost{answer: bridgeAnswer{OK: true, Fired: "fired"}}
		tab := macTab{name: "tmux:%7@" + socket, host: fake, pane: &commandTab{kind: "tmux", id: "%7", socket: socket}}
		if close {
			_ = tab.Close(Claude)
		} else {
			_ = tab.Focus()
		}
		if fake.asked == nil {
			t.Fatalf("%s: nothing asked", tab.name)
		}
		return fake.asked.Desk
	}
	want := &bridgeDesk{ID: id, Server: tmuxUnchecked, Pane: 4242, PaneStarted: 1794000000000000, Folder: folder}
	for _, close := range []bool{false, true} {
		if got := asked(socket, close); !reflect.DeepEqual(got, want) {
			t.Errorf("a plain hire's own server (close %v): %+v, want %+v", close, got, want)
		}
		// Named as a plain hire's, but never made by the core: nothing.
		if got := asked(filepath.Join(filepath.Dir(socket), hireSocketPrefix+"plain-000000000000"), close); got != nil {
			t.Errorf("a socket the core never made (close %v): %+v", close, got)
		}
	}
	forgetPlainTmux(socket)
	if got := asked(socket, false); got != nil {
		t.Errorf("a server the core forgot: %+v", got)
	}
}
