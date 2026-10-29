package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func swapped(d *Deck, from, to string) (int, string) {
	return d.Perform("swap", "", url.Values{"from": {from}, "to": {to}})
}

func TestSwappingDesksOfTheSameProjectChangesNothing(t *testing.T) {
	d := newTestDeck(t)
	for i, name := range []string{"tmux:%1@/tmp/tmux", "tmux:%2@/tmp/tmux"} {
		e := map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": name, "cwd": "/work/api"}
		d.sessions.Apply(e, Claude, name, "", float64(100+i))
	}
	if status, _ := swapped(d, "0", "1"); status != http.StatusConflict {
		t.Fatalf("swapping two desks of one project: %d, want %d", status, http.StatusConflict)
	}
	for i, pin := range d.config.Pins {
		if pin != nil {
			t.Fatalf("key %d was pinned to %s by a swap that was refused", i+1, *pin)
		}
	}
}

func TestSwappingTwoEmptyDesksChangesNothing(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	if status, _ := swapped(d, "0", "2"); status != http.StatusNoContent {
		t.Fatalf("swapping two empty desks: %d, want %d", status, http.StatusNoContent)
	}
	for i, pin := range d.config.Pins {
		if pin != nil {
			t.Fatalf("key %d was pinned to %s by swapping two empty desks", i+1, *pin)
		}
	}
	for _, desks := range [][2]string{{"0", "0"}, {"0", "3"}, {"-1", "1"}, {"one", "2"}} {
		if status, _ := swapped(d, desks[0], desks[1]); status != http.StatusBadRequest {
			t.Errorf("swapping desks %s and %s: %d, want %d", desks[0], desks[1], status, http.StatusBadRequest)
		}
	}
}

// A project pinned to a key beyond the three shown sits at a free desk; swapped,
// it's pinned where it went, and its old pin goes.
func TestASwappedProjectIsPinnedWhereItWent(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Keys = 3
	d.config.Pin("/work/p0", 7)
	running(d, 1)
	if keys, _ := d.layout(); keys[0].session == nil || keys[0].session.Project != "/work/p0" {
		t.Fatal("the project's session isn't at the first desk")
	}
	if status, _ := swapped(d, "0", "1"); status != http.StatusNoContent {
		t.Fatalf("swapping a project to an empty desk: %d, want %d", status, http.StatusNoContent)
	}
	if p := d.config.Pins; p[7] != nil || p[0] != nil || p[1] == nil || *p[1] != "/work/p0" {
		t.Fatalf("pins %v, want the project on key 2 only", p)
	}
}

func TestHiringAndFiringAreOffUntilTurnedOn(t *testing.T) {
	d, tab := madeButtonDeck(t)
	for _, action := range []string{"fire", "hire"} {
		if status, _ := d.Perform(action, "s1", url.Values{}); status != http.StatusForbidden {
			t.Errorf("%s with hiring off: %d, want %d", action, status, http.StatusForbidden)
		}
	}
	select {
	case command := <-tab.commands:
		t.Fatalf("firing while it's off sent %+v", command)
	default:
	}

	d.config.HiringEnabled = true
	// With Auto and one session, desk 1 is the one after the last, which a hire adds. No folder
	// for new hires is chosen, so none is hired.
	for desk, want := range map[string]int{"": http.StatusConflict, "0": http.StatusConflict, "1": http.StatusConflict,
		"2": http.StatusBadRequest, "8": http.StatusBadRequest, "x": http.StatusBadRequest} {
		query := url.Values{}
		if desk != "" {
			query.Set("desk", desk)
		}
		if status, _ := d.Perform("hire", "", query); status != want {
			t.Errorf("hire at desk %q: %d, want %d", desk, status, want)
		}
	}
	if status, _ := d.Perform("fire", "s1", url.Values{}); status != http.StatusNoContent || !sent(t, tab).Stop {
		t.Fatalf("fire: %d, and the launcher wasn't told to stop the agent", status)
	}
}

func TestFiringAnAgentWhoseTabHasGone(t *testing.T) {
	d, _ := madeButtonDeck(t)
	d.config.HiringEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s2", "cwd": "/work/api"},
		Claude, "launcher:L2", "", 200)
	if status, message := d.Perform("fire", "s2", url.Values{}); status != http.StatusNotFound || message != "That agent's tab has gone." {
		t.Fatalf("firing an agent with no tab: %d %q", status, message)
	}
}

func TestTheSnapshotCarriesTheComputersClock(t *testing.T) {
	d := newTestDeck(t)
	var snapshot struct{ Now int64 }
	if err := json.Unmarshal(d.snapshot(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if ago := time.Since(time.UnixMilli(snapshot.Now)); ago < 0 || ago > time.Minute {
		t.Fatalf("the snapshot's clock says %d, %v from now", snapshot.Now, ago)
	}
}

func TestAStyleFromThePhoneIsKeptAndAnUnknownOneIsNot(t *testing.T) {
	d := newTestDeck(t)
	d.config.Layout.Room.Corner = "dragon"
	if status, _ := d.Perform("style", "", url.Values{"preset": {"campsite"}}); status != http.StatusNoContent {
		t.Fatalf("picking a style: %d, want %d", status, http.StatusNoContent)
	}
	if room := d.config.Layout.Room; room.Preset != "campsite" || room.Corner != "dragon" {
		t.Fatalf("after picking Campsite the room is in %q with %q in the corner, want campsite over yours", room.Preset, room.Corner)
	}
	if status, _ := d.Perform("style", "", url.Values{"preset": {"disco"}}); status != http.StatusBadRequest {
		t.Fatalf("picking an unknown style: %d, want %d", status, http.StatusBadRequest)
	}
	if d.config.Layout.Room.Preset != "campsite" {
		t.Fatalf("an unknown style changed the room to %q", d.config.Layout.Room.Preset)
	}
}

func TestATapThatLandsTwiceSendsReturnOnce(t *testing.T) {
	d, tab := madeButtonDeck(t)
	if status, _ := d.Perform("accept", "s1", url.Values{}); status != http.StatusNoContent || *sent(t, tab).Press != press(Enter) {
		t.Fatal("accept didn't press Return")
	}
	if status, _ := d.Perform("accept", "s1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("a second accept gave %d", status)
	}
	if status, _ := d.Perform("send", "s1", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("a send straight after gave %d", status)
	}
	select {
	case command := <-tab.commands:
		t.Fatalf("a second Return reached the session: %+v", command)
	case <-time.After(100 * time.Millisecond):
	}
	// Down, down is two moves through a list, as the phone means it.
	for range 2 {
		if status, _ := d.Perform("down", "s1", url.Values{}); status != http.StatusNoContent || *sent(t, tab).Press != press(Down) {
			t.Fatal("down twice didn't press Down twice")
		}
	}
	time.Sleep(returnRepeat)
	if status, _ := d.Perform("accept", "s1", url.Values{}); status != http.StatusNoContent || *sent(t, tab).Press != press(Enter) {
		t.Fatal("accept a moment later didn't press Return")
	}
}

func TestAReturnThatDidntGoDoesntHoldBackTheNext(t *testing.T) {
	d, tab := madeButtonDeck(t)
	d.mu.Lock()
	delete(d.launchers, "L1") // its tab gone, for a moment
	d.mu.Unlock()
	if status, _ := d.Perform("accept", "s1", url.Values{}); status != http.StatusNotFound {
		t.Fatalf("accept with the tab gone gave %d", status)
	}
	d.mu.Lock()
	d.launchers["L1"] = tab
	d.mu.Unlock()
	if status, _ := d.Perform("accept", "s1", url.Values{}); status != http.StatusNoContent || *sent(t, tab).Press != press(Enter) {
		t.Fatal("the accept straight after one that failed didn't press Return")
	}
}

func TestFocusingWhileHooksArriveReadsTheTabSafely(t *testing.T) {
	d, tab := madeButtonDeck(t)
	go func() {
		for range tab.commands {
		}
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 200 {
			d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": fmt.Sprintf("/work/api/%d", i)},
				"claude", "launcher:L1", "", float64(1000+i))
		}
	}()
	for range 200 {
		d.Focus("s1")
	}
	<-done
}

// headless starts count headless runs, oldest first.
func headless(d *Deck, count int) {
	for i := range count {
		id := fmt.Sprintf("h%d", i)
		e := map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/api"}
		d.sessions.Apply(e, Claude, "headless:"+id, "", float64(1000+i))
	}
}

func TestHeadlessRunsSitUpstairsAwayFromTheKeys(t *testing.T) {
	d := newTestDeck(t)
	running(d, 2)
	headless(d, 14)
	keys, overflow := d.layout()
	for i, key := range keys {
		if key.session != nil && key.session.Headless() {
			t.Fatalf("key %d holds a headless run", i+1)
		}
	}
	if len(keys) != 2 || len(overflow) != 0 {
		t.Fatalf("%d keys and %d over, want 2 and none", len(keys), len(overflow))
	}
	var snapshot struct {
		Overflow int
		Upstairs struct {
			Desks    []map[string]any
			Overflow int
		}
	}
	if err := json.Unmarshal(d.snapshot(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Overflow != 0 || len(snapshot.Upstairs.Desks) != 12 || snapshot.Upstairs.Overflow != 2 {
		t.Fatalf("downstairs over %d, upstairs %d desks and %d over; want 0, 12 and 2",
			snapshot.Overflow, len(snapshot.Upstairs.Desks), snapshot.Upstairs.Overflow)
	}
	first := snapshot.Upstairs.Desks[0]
	if first["id"] != "h0" || first["headless"] != true || first["role"] != nil {
		t.Fatalf("the first desk upstairs is %v", first)
	}
	for _, control := range []string{"keys", "fire", "mic", "dictates", "summary"} {
		if first[control] != false {
			t.Errorf("a headless desk has %s %v", control, first[control])
		}
	}
	if status, message := d.Perform("accept", "h0", url.Values{}); status != http.StatusConflict || message != headlessNoControls {
		t.Errorf("accepting on a headless run gave %d %q", status, message)
	}
	if status, _ := d.Focus("h0"); status != http.StatusConflict {
		t.Errorf("focusing a headless run gave %d", status)
	}
}

func TestNobodyUpstairs(t *testing.T) {
	d := newTestDeck(t)
	var snapshot struct{ Upstairs json.RawMessage }
	if err := json.Unmarshal(d.snapshot(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if string(snapshot.Upstairs) != `{"desks":[],"overflow":0}` {
		t.Fatalf("upstairs with nobody there is %s", snapshot.Upstairs)
	}
}

// A desk's name tag counts from when its session started, which /clear
// doesn't move, so the phone is told it beside when the turn started.
func TestADeskSaysWhenItsSessionStartedThroughClear(t *testing.T) {
	d := newTestDeck(t)
	start := func(id, source string, at float64) {
		d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": source, "session_id": id, "cwd": "/work/api"},
			Claude, tab, "/work/api", at)
	}
	prompt := func(id string, at float64) {
		d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": "/work/api"}, Claude, tab, "/work/api", at)
	}
	start("s1", "startup", 1_000)
	prompt("s1", 5_000)
	start("s2", "clear", 9_000)
	prompt("s2", 9_500)
	var snapshot struct {
		Keys []struct{ Session map[string]any }
	}
	if err := json.Unmarshal(d.snapshot(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Keys) == 0 || snapshot.Keys[0].Session == nil {
		t.Fatalf("no session at the first desk: %v", snapshot.Keys)
	}
	desk := snapshot.Keys[0].Session
	if desk["id"] != "s2" || desk["startedAt"] != float64(1_000) || desk["turnStartedAt"] != float64(9_500) {
		t.Fatalf("the desk after /clear is %v %v started, turn %v; want s2, 1000 and 9500", desk["id"], desk["startedAt"], desk["turnStartedAt"])
	}
}
