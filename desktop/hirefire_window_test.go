//go:build !windows

package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// A hire in a window of its own, not in tmux (a 1.24 Swift-made hire in a Terminal tab): the engine's hire, its row
// then as the Swift engine wrote it, with no tmux socket. tab is its session's, in its worktree ("" for none); the
// Mac app is a stand-in answering closed with fired.
func ownWindowHire(t *testing.T, tab, closed string) (*Deck, *gmRig, *fakeHost) {
	t.Helper()
	g := newGMRig(t, "Rex")
	started := float64(time.Now().Add(-time.Minute).UnixMilli()) // hired a minute ago; its session started since
	g.changeRow(func(h *Hire) { h.TmuxSocket, h.State, h.Started = "", "working", started })
	g.hire.TmuxSocket, g.hire.Started = "", started
	fake := withHost(t, bridgeAnswer{OK: true, Fired: closed})
	if tab != "" {
		g.d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "rex-1",
			"cwd": g.hire.Worktree}, Claude, tab, "", nowMillis())
	}
	return g.d, g, fake
}

func closedTabs(fake *fakeHost) []bridgeRequest {
	var closes []bridgeRequest
	for _, asked := range fake.asked {
		if asked.Action == "close" {
			closes = append(closes, asked)
		}
	}
	return closes
}

func TestTheGMFiresAHireInAWindowOfItsOwnByClosingItsTab(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if w := g.fire(t, true, nil); w.Code != http.StatusNoContent {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if closes := closedTabs(fake); len(closes) != 1 || closes[0].Tab != "tty:/dev/ttys009" || closes[0].Agent != Claude || !closes[0].Strict {
		t.Errorf("the app was asked %+v, want one close of its tab", fake.asked)
	}
	if got := registered(t)[0]; got.State != "fired" || got.Notes != "" {
		t.Errorf("row %+v, want fired with no note", got)
	}
	if _, err := os.Stat(filepath.Join(supportDir, "recaps", g.hire.ID+".md")); err != nil {
		t.Errorf("no recap: %v", err)
	}
	if _, err := os.Stat(g.hire.Worktree); err != nil {
		t.Errorf("the worktree went though nobody asked: %v", err)
	}
	if d.sessions.Get("rex-1") != nil {
		t.Error("its session is still on a desk")
	}
	if len(g.asked()) != 0 {
		t.Errorf("tmux was asked %q", g.asked())
	}
}

func TestAHireInAWindowOfItsOwnKeepsTheRefusalsThatNeedNoWindow(t *testing.T) {
	t.Run("uncommitted", func(t *testing.T) {
		_, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
		os.WriteFile(filepath.Join(g.hire.Worktree, "draft.txt"), []byte("half"), 0o600)
		w := g.fire(t, true, nil)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Rex has changes that aren't committed: ?? draft.txt") {
			t.Fatalf("fire: %d %q", w.Code, w.Body)
		}
		if len(closedTabs(fake)) != 0 || registered(t)[0].State != "working" {
			t.Errorf("closed %+v, row %+v", fake.asked, registered(t)[0])
		}
	})
	t.Run("not stopped", func(t *testing.T) {
		_, g, _ := ownWindowHire(t, "tty:/dev/ttys009", "not-stopped")
		if w := g.fire(t, true, map[string]any{"force": true}); w.Code != http.StatusConflict || w.Body.String() != "Couldn't stop the agent, so its tab is left open." {
			t.Fatalf("fire: %d %q", w.Code, w.Body)
		}
		if registered(t)[0].State != "working" {
			t.Errorf("row %+v, want it still working", registered(t)[0])
		}
	})
}

const ownWindowNotFoundRex = "Rex runs in a window of its own, not in tmux, and Smolcorp can't find that window now, so nothing was changed. " +
	"If it's still open, close it there. Then fire again with force to mark Rex fired."

// refusedUnchanged checks a fire was refused with want, closed nothing and left the row working.
func refusedUnchanged(t *testing.T, w *httptest.ResponseRecorder, fake *fakeHost, status int, want string) {
	t.Helper()
	if w.Code != status || w.Body.String() != want {
		t.Fatalf("fire: %d %q, want %d %q", w.Code, w.Body, status, want)
	}
	if fake != nil && len(closedTabs(fake)) != 0 {
		t.Errorf("closed %+v", fake.asked)
	}
	if registered(t)[0].State != "working" {
		t.Errorf("row %+v, want it still working", registered(t)[0])
	}
}

// With no tab to close (none in its worktree, the tab has gone, or only its shell is left there), nothing changes
// and it says so. With force (RHIRE B2) it's marked fired, but the answer, a 200, says its window wasn't closed, and
// its worktree stays even though its removal was asked: the hire may still be at work in it.
func TestAHireInAWindowOfItsOwnWithNoTabIsMarkedFiredOnlyWithForceAndSaysSo(t *testing.T) {
	for _, c := range []struct{ name, tab, closed string }{{"no session", "", ""}, {"tab gone", "tty:/dev/ttys009", "gone"}} {
		t.Run(c.name, func(t *testing.T) {
			_, g, fake := ownWindowHire(t, c.tab, c.closed)
			refusedUnchanged(t, g.fire(t, true, nil), nil, http.StatusConflict, ownWindowNotFoundRex)
			w := g.fire(t, false, map[string]any{"force": true, "removeWorktree": true})
			var said map[string]any
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &said) != nil {
				t.Fatalf("fire with force: %d %q", w.Code, w.Body)
			}
			if said["note"] != "Rex is marked fired, but Smolcorp couldn't find its window, so it wasn't closed. If it's still open, close it there." ||
				said["kept"] != "Its window wasn't closed, so it may still be at work in it." || said["worktree"] != g.hire.Worktree {
				t.Errorf("said %v", said)
			}
			if registered(t)[0].State != "fired" {
				t.Errorf("row %+v, want fired", registered(t)[0])
			}
			if _, err := os.Stat(g.hire.Worktree); err != nil {
				t.Errorf("the clean worktree of a hire maybe still at work went: %v", err)
			}
			if c.tab == "" && len(closedTabs(fake)) != 0 {
				t.Errorf("closed %+v with no tab", fake.asked)
			}
		})
	}
}

// RHIRE B1 case 1: a session left on a tab whose agent has gone (the hire's tab closed by hand, its tty given to a
// new tab of the owner's, a shell at its prompt) is closed only strictly, which the Mac app answers gone.
func TestAStaleSessionOnATabWithOnlyAShellIsNeverClosed(t *testing.T) {
	_, g, fake := ownWindowHire(t, "tty:/dev/ttys005", "gone")
	refusedUnchanged(t, g.fire(t, true, nil), nil, http.StatusConflict, ownWindowNotFoundRex)
	if len(fake.asked) != 1 || fake.asked[0].Action != "close" || !fake.asked[0].Strict {
		t.Errorf("the app was asked %+v, want one strict close", fake.asked)
	}
}

// RHIRE B1 case 2: the owner's own session alone in the worktree is never taken for the hire's by the GM or Setup:
// one that started before the hire, or one after the hire's own session there ended. The office's fire of that desk,
// where the owner points at it, still closes it.
func TestTheOwnersSessionAloneInTheWorktreeIsNeverClosedByTheGM(t *testing.T) {
	t.Run("started before the hire", func(t *testing.T) {
		d, g, fake := ownWindowHire(t, "", "fired")
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "owner-1",
			"cwd": g.hire.Worktree}, Claude, "tty:/dev/ttys004", "", g.hire.Started-60_000)
		refusedUnchanged(t, g.fire(t, true, nil), fake, http.StatusConflict, ownWindowNotFoundRex)
		refusedUnchanged(t, g.fire(t, false, nil), fake, http.StatusConflict, ownWindowNotFoundRex)
		if status, message := d.Perform("fire", "owner-1", url.Values{}); status != http.StatusNoContent || len(closedTabs(fake)) != 1 {
			t.Errorf("the office's fire of its desk: %d %q, closed %+v", status, message, fake.asked)
		}
	})
	t.Run("after the hire's own session ended", func(t *testing.T) {
		d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionEnd", "session_id": "rex-1"}, Claude, "tty:/dev/ttys009", "", nowMillis())
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "owner-1",
			"cwd": g.hire.Worktree}, Claude, "tty:/dev/ttys004", "", nowMillis()+1)
		refusedUnchanged(t, g.fire(t, true, nil), fake, http.StatusConflict, ownWindowNotFoundRex)
	})
}

// RHIRE N3: the Mac app not answering isn't a window that has gone: nothing is changed, even with force.
func TestABridgeFailureIsNotTakenForAWindowThatHasGone(t *testing.T) {
	_, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	fake.err = errors.New("the Mac app stopped answering part way")
	w := g.fire(t, true, map[string]any{"force": true})
	if w.Code != http.StatusBadGateway || w.Body.String() != "The Mac app didn't close Rex's window, so nothing was changed. Try again in a moment." {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if registered(t)[0].State != "working" {
		t.Errorf("row %+v, want it still working", registered(t)[0])
	}
}

// RHIRE B3: a 1.24 row left "starting" (it never reported) is fired like any other, and with force when its tab
// can't be found; it's never refused for good.
func TestAHireInAWindowOfItsOwnStillStartingCanBeFired(t *testing.T) {
	_, g, _ := ownWindowHire(t, "", "")
	g.changeRow(func(h *Hire) { h.State = "starting" })
	if w := g.fire(t, true, nil); w.Code != http.StatusConflict || w.Body.String() != ownWindowNotFoundRex || registered(t)[0].State != "starting" {
		t.Fatalf("fire: %d %q, row %+v", w.Code, w.Body, registered(t)[0])
	}
	if w := g.fire(t, true, map[string]any{"force": true}); w.Code != http.StatusOK || registered(t)[0].State != "fired" {
		t.Fatalf("fire with force: %d %q, row %+v", w.Code, w.Body, registered(t)[0])
	}
}

// RHIRE N8 (testCopyKeepsTab once FXWTC is merged): a test copy never closes such a hire's tab, the owner's own,
// even with force; nothing is changed.
func TestATestCopyNeverClosesTheTabOfAHireInAWindowOfItsOwn(t *testing.T) {
	_, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	saved := Port
	t.Cleanup(func() { Port = saved })
	Port = 7799
	if testCopyPort() == 0 {
		t.Fatal("not a test copy's port")
	}
	refusedUnchanged(t, g.fire(t, true, map[string]any{"force": true}), fake, http.StatusConflict, ownWindowTestCopy)
	if len(fake.asked) != 0 {
		t.Errorf("the app was asked %+v", fake.asked)
	}
}

// RHIRE B1 b and N1: a Terminal tab whose agent has left it, seen twice in a row by the Mac app, loses its desk, as
// the Swift engine's Deck.pruneClosedPanes did; once, or unknown, isn't enough.
func TestATabWhoseAgentHasLeftItIsDroppedAtTheSecondLook(t *testing.T) {
	d := newTestDeck(t)
	fake := withHost(t, bridgeAnswer{OK: true, Ended: []bool{true}})
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s-1", "cwd": t.TempDir()},
		Claude, "tty:/dev/ttys005", "", nowMillis())
	d.pruneClosedTabs()
	if d.sessions.Get("s-1") == nil {
		t.Fatal("dropped at the first look")
	}
	fake.answer = bridgeAnswer{OK: true, Ended: []bool{false}}
	d.pruneClosedTabs()
	fake.answer = bridgeAnswer{OK: true, Ended: []bool{true}}
	d.pruneClosedTabs()
	fake.err = errors.New("no answer")
	d.pruneClosedTabs() // unknown is not gone, and not a second look
	fake.err = nil
	if d.sessions.Get("s-1") == nil {
		t.Fatal("dropped though its agent was back between two looks")
	}
	d.pruneClosedTabs()
	if d.sessions.Get("s-1") != nil {
		t.Error("its desk stays though its agent left its tab twice in a row")
	}
	for _, asked := range fake.asked {
		if asked.Action != "agents-ended" || !slices.Equal(asked.Tabs, []string{"tty:/dev/ttys005"}) || !slices.Equal(asked.Agents, []Agent{Claude}) {
			t.Errorf("asked %+v", asked)
		}
	}
}

func TestRemovingTheWorktreeOfAHireInAWindowOfItsOwnGoesAsTheEnginesDoes(t *testing.T) {
	_, g, _ := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if w := g.fire(t, true, map[string]any{"removeWorktree": true}); w.Code != http.StatusNoContent {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if _, err := os.Stat(g.hire.Worktree); !os.IsNotExist(err) {
		t.Errorf("the clean worktree is still there: %v", err)
	}
}

// The office's fire of its desk: the same, the worktree kept, as the Mac's office keeps it.
func TestTheOfficeFiresAHireInAWindowOfItsOwnFromItsDesk(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	if status, message := d.Perform("fire", "rex-1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("fire: %d %q", status, message)
	}
	if len(closedTabs(fake)) != 1 || registered(t)[0].State != "fired" {
		t.Errorf("closed %+v, row %+v", fake.asked, registered(t)[0])
	}
	if _, err := os.Stat(g.hire.Worktree); err != nil {
		t.Errorf("the worktree went: %v", err)
	}
}

// Two sessions in one worktree: neither tab is closed on a guess.
func TestTwoSessionsInAWorktreeAreNeitherTakenForTheHiresTab(t *testing.T) {
	d, g, fake := ownWindowHire(t, "tty:/dev/ttys009", "fired")
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "rex-2",
		"cwd": g.hire.Worktree}, Claude, "tty:/dev/ttys010", "", nowMillis())
	if w := g.fire(t, true, nil); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "can't find that window") {
		t.Fatalf("fire: %d %q", w.Code, w.Body)
	}
	if len(closedTabs(fake)) != 0 {
		t.Errorf("closed %+v", fake.asked)
	}
}

// RHIRE B2: Setup says what a fire's 200 says (no window closed, and why a worktree stayed) under the team, where it
// outlives the fired hire's row; run in node on setup.html's own firedSaid. Without node the test is skipped.
func TestSetupSaysWhenAFireClosedNoWindow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: setup.html's firedSaid can't be run")
	}
	data, err := os.ReadFile(filepath.Join("..", "Resources", "web", "setup.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	for _, want := range []string{"tell('team-problem', await firedSaid(res)); load(); return;", "tell('gm-problem', await firedSaid(res)); load();"} {
		if !strings.Contains(page, want) {
			t.Errorf("setup.html doesn't say a fire's answer: no %q", want)
		}
	}
	start := strings.Index(page, "  async function firedSaid(res) {")
	end := strings.Index(page[max(start, 0):], "\n  }\n")
	if start < 0 || end < 0 {
		t.Fatal("setup.html has no firedSaid")
	}
	script := page[start:start+end+4] + `
const res = (status, body) => ({ status, json: async () => body });
(async () => console.log(JSON.stringify([
  await firedSaid(res(204)),
  await firedSaid(res(200, { ok: true, note: "Rex is marked fired, but Smolcorp couldn't find its window, so it wasn't closed. If it's still open, close it there." })),
  await firedSaid(res(200, { ok: true, note: "Rex is marked fired, but…", kept: "Its window wasn't closed, so it may still be at work in it.", worktree: "/w/rex" })),
])))();`
	out, err := exec.Command(node, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := `["","Rex is marked fired, but Smolcorp couldn't find its window, so it wasn't closed. If it's still open, close it there.",` +
		`"Rex is marked fired, but… Its worktree (/w/rex) was kept: its window wasn't closed, so it may still be at work in it."]`
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
