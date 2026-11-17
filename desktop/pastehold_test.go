package desktop

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// The paste hold (pastehold.go, PASTEHOLD1): while tests run on this Mac, what the office types into desks waits, in
// order for each desk, and goes once the hold ends, by `smolcorp hold off` or at its deadline. Only a hold sealed with
// the running copy's hold key holds. Never a real clipboard, terminal or app: the stand-in Mac app and the fake
// launcher answer, and the hold's files are in the test's own folder.

// testHoldKey is the hold key a deck started without the Mac app is given here, as the Mac app's start makes one.
const testHoldKey = "fake-hold-key"

// holdOn sets a hold of id, ending at until, as `smolcorp hold on` writes it, and has the deck take it now. The
// deck's own rounds are stopped first, so the hold is looked at only when the test says.
func holdOn(t *testing.T, d *Deck, id string, until time.Time) {
	t.Helper()
	d.rounds.stop()
	if d.hold.key == "" {
		d.hold.key = testHoldKey
	}
	if err := writePrivate(holdFile(), holdRecord(d.hold.key, id, until)); err != nil {
		t.Fatal(err)
	}
	d.hold.look(time.Now())
	if on, _, _, _, _ := d.hold.state(time.Now()); !on {
		t.Fatal("the hold isn't on")
	}
}

// holdOff ends the hold, as `smolcorp hold off`, and has the deck take it now.
func holdOff(t *testing.T, d *Deck) {
	t.Helper()
	if err := os.Remove(holdFile()); err != nil {
		t.Fatal(err)
	}
	d.hold.look(time.Now())
}

func waitingOn(h *pasteHold) int {
	_, _, _, waiting, _ := h.state(time.Now())
	return waiting
}

func holdIsOn(h *pasteHold, now time.Time) bool {
	on, _, _, _, _ := h.state(now)
	return on
}

// The phone's and the window's words during a hold: each told it waits (202), nothing typed; once the hold ends,
// every one is typed, in the order it came, with its own Return or not. Nothing lost.
func TestWordsDuringAHoldWaitAndGoInOrderOnceItEnds(t *testing.T) {
	d, f := windowDeck(t)
	holdOn(t, d, "run-1", time.Now().Add(10*time.Minute))
	no := false
	for _, w := range []struct {
		say  func() (int, string)
		what string
	}{
		{func() (int, string) { return d.TypeWindowText("first, from the window", "s1", nil) }, "window"},
		{func() (int, string) { return d.TypePhoneText("second", "s1", nil) }, "phone"},
		{func() (int, string) { return d.TypeWindowText("third:\n two lines", "s1", &no) }, "window, no Return"},
	} {
		if status, says := w.say(); status != http.StatusAccepted || says != heldWords {
			t.Fatalf("%s: %d %q", w.what, status, says)
		}
	}
	if got := offBy(f); len(got) != 0 {
		t.Fatalf("typed during the hold: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 3 {
		t.Fatalf("%d waiting", waiting)
	}
	holdOff(t, d)
	want := []string{pasted("first, from the window"), keyBytes[Enter], pasted("second"), pasted("third:\n two lines")}
	if got := f.writtenAfter(len(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("after the hold: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 0 {
		t.Errorf("%d still waiting", waiting)
	}
	// Ended, the words go as they come again.
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	if status, says := d.TypeWindowText("fourth", "s1", nil); status != http.StatusNoContent {
		t.Errorf("after: %d %q", status, says)
	}
}

// What's refused at once is still refused at once during a hold (nothing that could never go waits); a desk that
// asks once the hold ends gets its words when it's done asking, however long that takes (nothing dropped), still
// before what came after for it.
func TestAfterAHoldADeskThatAsksGetsItsWordsOnceItsDone(t *testing.T) {
	d, f := windowDeck(t)
	holdOn(t, d, "run-2", time.Now().Add(10*time.Minute))
	if status, says := d.TypeWindowText("!rm", "s1", nil); status != http.StatusBadRequest || says != noBangPrompt {
		t.Errorf("a refusal waited: %d %q", status, says)
	}
	if status, _ := d.TypeWindowText("one", "s1", nil); status != http.StatusAccepted {
		t.Fatal(status)
	}
	asksAt(d, "launcher:L1", 30)
	holdOff(t, d)
	// Still asking ten minutes on: the words still wait (the build before dropped them after five).
	d.hold.look(time.Now().Add(10 * time.Minute))
	if got := offBy(f); len(got) != 0 {
		t.Fatalf("typed into a desk that asks: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 1 {
		t.Fatalf("%d waiting: the words were dropped", waiting)
	}
	if _, _, _, _, notes := d.hold.state(time.Now()); len(notes) != 0 {
		t.Errorf("told as not typed while they wait: %+v", notes)
	}
	// Words that come while one waits go behind it, and are told so.
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 40)
	if status, says := d.TypeWindowText("two", "s1", nil); status != http.StatusAccepted || says != heldBehind {
		t.Fatalf("came after, went first: %d %q", status, says)
	}
	d.hold.look(time.Now())
	// "two" comes right after a Return, so it goes without one (claimReturn), as it would have without a hold.
	if got := f.writtenAfter(3); !reflect.DeepEqual(got, []string{pasted("one"), keyBytes[Enter], pasted("two")}) {
		t.Errorf("after the question: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 0 {
		t.Errorf("%d still waiting", waiting)
	}
}

// RT126PASTEHOLD1 N4: one desk asking you something once the hold ends holds up its own words only: another desk's
// go at once.
func TestAfterAHoldOneDeskAskingHoldsUpNoOther(t *testing.T) {
	d, api := windowDeck(t)
	web := newFakeLauncherAs(t, d, "L2", "/work/web", "", "", "idle-empty", true)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s2", "cwd": "/work/web"}, "claude", "launcher:L2", "", 21)
	holdOn(t, d, "run-n4", time.Now().Add(10*time.Minute))
	for _, w := range [][2]string{{"for api", "s1"}, {"for web", "s2"}} {
		if status, says := d.TypeWindowText(w[0], w[1], nil); status != http.StatusAccepted {
			t.Fatalf("%s: %d %q", w[0], status, says)
		}
	}
	asksAt(d, "launcher:L1", 30)
	holdOff(t, d)
	if got := web.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("for web"), keyBytes[Enter]}) {
		t.Errorf("the other desk waited behind the one asking: %q", got)
	}
	if got := offBy(api); len(got) != 0 {
		t.Errorf("typed into the desk that asks: %q", got)
	}
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 40)
	d.hold.look(time.Now())
	if got := api.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("for api"), keyBytes[Enter]}) {
		t.Errorf("once it's done asking: %q", got)
	}
}

// RT126PASTEHOLD1 B2: words told "typed when the tests end" that a desk refuses for good once they end (here, the
// phone's words switched off meanwhile) aren't dropped without a word: Setup says which desk and why, never the words.
// A photo's path refused so has its photo removed at once (N5), as when it's refused as it comes.
func TestHeldWordsADeskRefusesForGoodAreToldInSetup(t *testing.T) {
	d, f := windowDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	holdOn(t, d, "run-b2", time.Now().Add(10*time.Minute))
	const secret = "the words themselves"
	if status, says := d.TypePhoneText(secret, "s1", nil); status != http.StatusAccepted || says != heldWords {
		t.Fatalf("%d %q", status, says)
	}
	if status, says := sendTestPhoto(d, textFromPhone, "image/png", bytes.NewReader(testPNG(t))); status != http.StatusAccepted {
		t.Fatalf("the photo: %d %q", status, says)
	}
	if kept := photosOf(t, d, "s1"); len(kept) != 1 {
		t.Fatalf("the held photo isn't kept: %v", kept)
	}
	d.mu.Lock()
	d.config.PhoneMicEnabled = false
	d.mu.Unlock()
	holdOff(t, d)
	if got := offBy(f); len(got) != 0 {
		t.Fatalf("typed: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 0 {
		t.Errorf("%d still waiting", waiting)
	}
	if kept := photosOf(t, d, "s1"); len(kept) != 0 {
		t.Errorf("a photo never typed was kept: %v", kept)
	}
	i := slices.IndexFunc(d.selfTest(), func(c check) bool { return c.ID == "pastehold-missed" })
	if i < 0 {
		t.Fatal("Setup doesn't say held words weren't typed")
	}
	line := d.selfTest()[i]
	if !strings.Contains(line.Detail, "api, ") || !strings.Contains(line.Detail, phoneTextOff) || strings.Count(line.Detail, phoneTextOff) != 2 {
		t.Errorf("Setup's line doesn't name the desk and why, for each: %q", line.Detail)
	}
	if strings.Contains(line.Detail, secret) {
		t.Errorf("Setup's line has the words in it: %q", line.Detail)
	}
	// It goes after an hour.
	if _, _, _, _, notes := d.hold.state(time.Now().Add(holdNoteLasts + time.Second)); len(notes) != 0 {
		t.Errorf("still said after an hour: %+v", notes)
	}
}

// The hold's deadline: it ends on its own then, and never later than 15 minutes after the core began to hold,
// whatever its file says, even rewritten later. A crashed runner can't hold for ever, and nobody can hold again and
// again: an ended hold isn't kept again, and a new one only a minute after the last ended.
func TestTheHoldEndsOnItsOwn(t *testing.T) {
	isolate(t)
	now := time.Now()
	hold := pasteHold{key: testHoldKey}
	set := func(id string, until time.Time) {
		t.Helper()
		if err := writePrivate(holdFile(), holdRecord(testHoldKey, id, until)); err != nil {
			t.Fatal(err)
		}
	}
	set("run-3", now.Add(2*time.Minute))
	if id, until, ok := readHold(holdFile(), testHoldKey); !ok || id != "run-3" || until.Unix() != now.Add(2*time.Minute).Unix() {
		t.Fatalf("read back: %v %q %v", ok, id, until)
	}
	went := false
	hold.look(now)
	if hold.held(&heldTyping{desk: "d", try: func() (bool, string) { went = true; return false, "" }}) == "" {
		t.Fatal("not held")
	}
	for _, row := range []struct {
		at   time.Duration
		want bool
	}{{0, true}, {2*time.Minute - time.Second, true}, {2*time.Minute + time.Second, false}} {
		hold.look(now.Add(row.at))
		if got := holdIsOn(&hold, now); got != row.want || went == row.want {
			t.Errorf("at %v: on %v, typed %v", row.at, got, went)
		}
	}
	// The same hold, its file still there, isn't kept again; nor is another within a minute of the last.
	ended := now.Add(2*time.Minute + time.Second)
	set("run-3", ended.Add(5*time.Minute))
	hold.look(ended.Add(2 * holdRest))
	if holdIsOn(&hold, now) {
		t.Error("an ended hold was kept again")
	}
	set("run-4", ended.Add(24*time.Hour))
	hold.look(ended.Add(holdRest - time.Second))
	if holdIsOn(&hold, now) {
		t.Error("a hold kept less than a minute after the last")
	}
	// A minute on it is, and a day's deadline ends 15 minutes after that look, rewritten or not.
	began := ended.Add(holdRest)
	hold.look(began)
	if !holdIsOn(&hold, now) {
		t.Fatal("a new hold a minute on wasn't kept")
	}
	hold.look(began.Add(10 * time.Minute))
	set("run-4", began.Add(24*time.Hour))
	hold.look(began.Add(holdLongest - time.Second))
	if !holdIsOn(&hold, now) {
		t.Error("ended early")
	}
	hold.look(began.Add(holdLongest))
	if holdIsOn(&hold, now) {
		t.Error("still on 15 minutes after the core began to hold")
	}
	// A file the hold's deadline can't be read from, a link, anything but a plain file: none.
	for _, content := range []string{"not a time x " + holdSeal(testHoldKey, "paste-hold not a time"), ""} {
		if err := writePrivate(holdFile(), []byte(content)); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := readHold(holdFile(), testHoldKey); ok {
			t.Errorf("%q holds", content)
		}
	}
	set("run-5", now.Add(time.Hour))
	link := filepath.Join(supportDir, "link")
	if err := os.Symlink(holdFile(), link); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readHold(link, testHoldKey); ok {
		t.Error("a link holds")
	}
}

// RT126PASTEHOLD1 B1, each of its probes as a row: a hold no one sealed with the running copy's hold key holds
// nothing, whoever wrote it; the key is made, readable by the owner alone, only under the Mac app; off a Mac there is
// no key, so nothing holds; and `smolcorp hold` there refuses, writing nothing.
func TestAHoldNotSealedWithTheKeyHoldsNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	d.rounds.stop()
	if d.hold.key == "" || readHoldKey() != d.hold.key {
		t.Fatalf("under the Mac app, the hold key in its file %q isn't the core's %q", readHoldKey(), d.hold.key)
	}
	if info, err := os.Stat(holdKeyFile()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the hold key's file: %v %v", info, err)
	}
	until := time.Now().Add(10 * time.Minute)
	for _, row := range []struct {
		what    string
		content []byte
	}{
		// The probe's own plain write, no command at all.
		{"a plain write", []byte("9999999999 x\n")},
		{"a deadline and an id, no seal", fmt.Appendf(nil, "%d run-x\n", until.Unix())},
		{"sealed with another key (an earlier start's)", holdRecord("an-earlier-start", "run-x", until)},
		{"sealed, then its deadline changed", bytes.Replace(holdRecord(d.hold.key, "run-x", until), fmt.Appendf(nil, "%d", until.Unix()),
			fmt.Appendf(nil, "%d", until.Add(time.Hour).Unix()), 1)},
		{"sealed, then its id changed", bytes.Replace(holdRecord(d.hold.key, "run-x", until), []byte("run-x"), []byte("run-y"), 1)},
		{"sealed as said seen, not as a hold", seenRecord(d.hold.key, fmt.Sprintf("%d run-x", until.Unix()))},
	} {
		if err := writePrivate(holdFile(), row.content); err != nil {
			t.Fatal(err)
		}
		d.hold.look(time.Now())
		if holdIsOn(&d.hold, time.Now()) {
			t.Errorf("%s: holds", row.what)
		}
		if status, says := d.TypePhoneText("words", "s1", nil); status != http.StatusNoContent {
			t.Errorf("%s: the phone's words: %d %q", row.what, status, says)
		}
	}
	if got := len(mac.actions()); got < 6 {
		t.Errorf("the Mac app was asked %d times for 6 messages", got)
	}
	// The core's word that it holds can't be forged either: `smolcorp hold on` takes only its seal.
	for _, forged := range [][]byte{[]byte("run-x\n"), []byte("run-x " + strings.Repeat("0", 64) + "\n"), seenRecord("an-earlier-start", "run-x")} {
		if err := writePrivate(holdSeen(), forged); err != nil {
			t.Fatal(err)
		}
		if got := readSeen(holdSeen(), d.hold.key); got != "" {
			t.Errorf("a forged seen %q: %q", forged, got)
		}
	}
	// What the owner's `smolcorp hold` says of a file it didn't seal.
	saved := holdSystem
	t.Cleanup(func() { holdSystem = saved })
	holdSystem = "darwin"
	if err := writePrivate(holdFile(), []byte("9999999999 x\n")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := runHold(nil, &out, time.Now); code != 0 || !strings.HasPrefix(out.String(), "No hold") {
		t.Errorf("status of an unsealed hold: %d %s", code, out.String())
	}

	// Off a Mac: no key made, and a hold sealed as the Mac's would be holds nothing there.
	w, f := windowDeck(t)
	w.rounds.stop()
	if w.hold.key != "" {
		t.Fatalf("a key off the Mac app: %q", w.hold.key)
	}
	if err := writePrivate(holdFile(), holdRecord("", "run-z", until)); err != nil {
		t.Fatal(err)
	}
	w.hold.look(time.Now())
	if status, says := w.TypeWindowText("typed", "s1", nil); status != http.StatusNoContent || holdIsOn(&w.hold, time.Now()) {
		t.Errorf("off a Mac: %d %q", status, says)
	}
	if got := f.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("typed"), keyBytes[Enter]}) {
		t.Errorf("off a Mac: %q", got)
	}
	_ = os.Remove(holdFile())
	for _, system := range []string{"linux", "windows"} {
		holdSystem = system
		for _, args := range [][]string{{"on"}, {"on", "5"}, {"off"}, nil} {
			out.Reset()
			if code := runHold(args, &out, time.Now); code != 2 || strings.TrimSpace(out.String()) != holdNotHere {
				t.Errorf("%s %q: %d %s", system, args, code, out.String())
			}
		}
		if _, err := os.Stat(holdFile()); err == nil {
			t.Errorf("%s: a hold was written", system)
		}
	}
}

// The CLIP1 guard's side: during a hold the Mac app is never asked to paste (its paste goes through the Mac's own
// clipboard), the dictation's words included. Once the hold ends, they go through it, in order.
func TestDuringAHoldTheMacAppIsNeverAskedToPaste(t *testing.T) {
	d, mac := phoneDeck(t)
	holdOn(t, d, "run-6", time.Now().Add(10*time.Minute))
	if status, says := d.TypePhoneText("two lines:\nfrom the phone", "s1", nil); status != http.StatusAccepted {
		t.Fatalf("%d %q", status, says)
	}
	d.typeDictation(&dictationRun{tab: "tty:/dev/ttys003", agent: Claude}, "dictated")
	typed := func() []string {
		mac.mu.Lock()
		defer mac.mu.Unlock()
		var asked []string
		for _, r := range mac.asked {
			if slices.Contains([]string{"phone-text", "insert", "type", "press"}, r.Action) {
				asked = append(asked, r.Action+" "+r.Text)
			}
		}
		return asked
	}
	if got := typed(); len(got) != 0 {
		t.Fatalf("the Mac app was asked during the hold: %q", got)
	}
	holdOff(t, d)
	if got, want := typed(), []string{"phone-text two lines:\nfrom the phone", "insert dictated"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after the hold: %q", got)
	}
	if waiting := waitingOn(&d.hold); waiting != 0 {
		t.Errorf("%d still waiting", waiting)
	}
}

// Mail to a hire waits in its outbox during a hold, untried (no attempt counted), and goes, in outbox order, at the
// first round after it. A bot's mail is a file, not typing: it goes.
func TestMailWaitsInItsOutboxDuringAHold(t *testing.T) {
	r := newPumpRig(t)
	var hold pasteHold
	r.pump.enter, r.pump.leave = hold.enter, hold.end
	hold.on = true
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "first", "m1"))
	mustAdd(t, r.o, toHire("bbbbbb222222", "Ann", "second", "m2"))
	r.pump.round()
	if len(r.typed) != 0 {
		t.Fatalf("typed during the hold: %q", r.typed)
	}
	for _, id := range []string{"m1", "m2"} {
		if m := r.msg(id); m.State != mailQueued || m.Attempts != 0 {
			t.Errorf("%s during the hold: %+v", id, m)
		}
	}
	hold.on = false
	r.pump.round()
	if len(r.typed) != 2 || !strings.HasSuffix(r.typed[0], "first") || !strings.HasSuffix(r.typed[1], "second") {
		t.Fatalf("after: %q", r.typed)
	}
	if hold.typing != 0 {
		t.Errorf("typing left under way: %d", hold.typing)
	}
}

// The hold is said seen (what `smolcorp hold on` waits for) only once typing let through before it has ended, and
// a paste through the Mac's clipboard has had time to put back what you had copied. Its word is sealed.
func TestTheHoldIsSeenOnlyOnceNothingIsTyping(t *testing.T) {
	isolate(t)
	seen := func() string { return readSeen(holdSeen(), testHoldKey) }
	set := func(id string) time.Time {
		t.Helper()
		if err := writePrivate(holdFile(), holdRecord(testHoldKey, id, time.Now().Add(time.Minute))); err != nil {
			t.Fatal(err)
		}
		return time.Now()
	}
	// Typing let through before the hold.
	hold := pasteHold{key: testHoldKey}
	if !hold.enter() {
		t.Fatal("held with no hold")
	}
	now := set("run-7")
	hold.look(now)
	if hold.enter() || hold.held(&heldTyping{desk: "d", try: func() (bool, string) { return false, "" }}) == "" {
		t.Error("let through during the hold")
	}
	if got := seen(); got != "" {
		t.Fatalf("seen while typing: %q", got)
	}
	hold.end()
	hold.look(now)
	if got := seen(); got != "run-7" {
		t.Fatalf("not seen once the typing ended: %q", got)
	}
	// A paste through the Mac's clipboard that answered just now: it may still put back what you had copied.
	_ = os.Remove(holdSeen())
	mac := pasteHold{key: testHoldKey}
	mac.pastedOnMac()
	now = set("run-8")
	mac.look(now)
	if got := seen(); got != "" {
		t.Fatalf("seen before a Mac paste could put back: %q", got)
	}
	mac.look(now.Add(holdMacSettle))
	if got := seen(); got != "run-8" {
		t.Fatalf("not seen: %q", got)
	}
}

// Setup shows a line while a hold is on, and none without one.
func TestSetupSaysWhileAHoldIsOn(t *testing.T) {
	d, _ := windowDeck(t)
	hasLine := func() bool {
		return slices.ContainsFunc(d.selfTest(), func(c check) bool { return c.ID == "pastehold" && c.Status == checkInfo })
	}
	if hasLine() {
		t.Fatal("a line with no hold")
	}
	holdOn(t, d, "run-8", time.Now().Add(10*time.Minute))
	if !hasLine() {
		t.Error("no line during the hold")
	}
	holdOff(t, d)
	if hasLine() {
		t.Error("a line after the hold")
	}
}

// `smolcorp hold`: on waits until the running copy says it holds; off ends it; at most 15 minutes; never from a hire;
// nothing to hold with no copy running.
func TestTheHoldCommand(t *testing.T) {
	isolate(t)
	running, wait, poll, system := holdRunning, holdWait, holdPoll, holdSystem
	t.Cleanup(func() { holdRunning, holdWait, holdPoll, holdSystem = running, wait, poll, system })
	holdRunning, holdWait, holdPoll, holdSystem = func() bool { return true }, 2*time.Second, 5*time.Millisecond, "darwin"
	var out bytes.Buffer
	run := func(args ...string) int { out.Reset(); return runHold(args, &out, time.Now) }
	for _, args := range [][]string{{"on", "16"}, {"on", "0"}, {"on", "x"}, {"maybe"}, {"off", "now"}} {
		if code := run(args...); code != 2 {
			t.Errorf("%q: %d %s", args, code, out.String())
		}
	}
	// No running copy's key: nothing to hold, and nothing written.
	if code := run("on"); code != 0 || !strings.Contains(out.String(), "isn't running") {
		t.Errorf("no key: %d %s", code, out.String())
	}
	if _, err := os.Stat(holdFile()); err == nil {
		t.Fatal("a hold was set with no copy to hold")
	}
	// The running copy: a stand-in that takes the hold as the core does, with its key.
	key := newHoldKey()
	hold := pasteHold{key: key}
	stop := make(chan struct{})
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				hold.look(time.Now())
			}
		}
	}()
	if code := run("on", "5"); code != 0 || !strings.Contains(out.String(), "on hold until") {
		t.Errorf("on: %d %s", code, out.String())
	}
	id, until, ok := readHold(holdFile(), key)
	if !ok || until.After(time.Now().Add(5*time.Minute)) || id == "" {
		t.Errorf("the hold: %v %v %q", ok, until, id)
	}
	if info, err := os.Stat(holdFile()); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the hold's file: %v %v", info, err)
	}
	if code := run(); code != 0 || !strings.Contains(out.String(), "on hold until") {
		t.Errorf("status: %d %s", code, out.String())
	}
	if code := run("off"); code != 0 {
		t.Errorf("off: %d %s", code, out.String())
	}
	// Right after, a new hold isn't kept for a minute: on says so, and fails.
	if code := run("on"); code != 1 || !strings.Contains(out.String(), "didn't say it holds") {
		t.Errorf("within a minute of the last: %d %s", code, out.String())
	}
	close(stop)
	<-ended
	if holdIsOn(&hold, time.Now()) {
		t.Error("held within a minute of the last")
	}
	_ = os.Remove(holdFile())
	// The self-test, for the runner's install check: stand-ins only, in its own folder, which it leaves as it found.
	if code := run("selftest"); code != 0 || strings.Contains(out.String(), "FAIL") || !strings.Contains(out.String(), "self-test passed") {
		t.Errorf("selftest: %d %s", code, out.String())
	}
	if _, err := os.Stat(holdFile()); err == nil {
		t.Error("the self-test touched the real hold")
	}
	t.Setenv("SMOLCORP_HIRE_KEY", "FAKE")
	if code := run("on"); code != 2 {
		t.Errorf("from a hire: %d", code)
	}
	if _, err := os.Stat(holdFile()); err == nil {
		t.Error("a hire set a hold")
	}
}

// Every way the office types words into a desk passes the hold: the one function that types them into a tab
// (phoneText, the launcher's PhoneText and the Mac app's clipboard paste among its ways) is called only by
// typeTextOr and typeDictationNow, each held, and the Mac app's paste only by phoneText; a hire's mail is typed only
// past the pump's enter. On the Mac app's side, its clipboard paste is reached only from the core's phone-text ask,
// and reads no hold file of its own (one a desk could write).
func TestEveryWayOfTypingIntoADeskPassesTheHold(t *testing.T) {
	set := token.NewFileSet()
	callers := map[string][]string{}
	watched := []string{"phoneText", "hostPhoneText", "pasteIntoLauncher", "deliverToHire", "typeTextOr", "typeDictationNow"}
	files, _ := filepath.Glob("*.go")
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			selected := map[*ast.Ident]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// A watched function named as a value, not called (a func handed on), is counted too: once.
				var named string
				switch f := n.(type) {
				case *ast.SelectorExpr:
					named, selected[f.Sel] = f.Sel.Name, true
				case *ast.Ident:
					// Not a local variable of the same name (the router's phoneText switch).
					if !selected[f] && (f.Obj == nil || f.Obj.Kind != ast.Var) {
						named = f.Name
					}
				}
				if slices.Contains(watched, named) {
					callers[named] = append(callers[named], fn.Name.Name)
				}
				return true
			})
		}
	}
	want := map[string][]string{
		// A Tab's own phoneText (a pane's, a command's, an ACP desk's) is called from Deck.phoneText alone.
		"phoneText":         {"phoneText", "phoneText", "phoneText", "typeDictationNow", "typeTextOr"},
		"hostPhoneText":     {"phoneText"},
		"pasteIntoLauncher": {"phoneText"},
		"deliverToHire":     {"round", "round"},
		// Ungated only as what waited: from within its own gate's retry.
		"typeTextOr":       {"sendPhoto", "typeText", "typeTextOr"},
		"typeDictationNow": {"typeDictation", "typeDictation"},
	}
	for _, name := range watched {
		got := callers[name]
		slices.Sort(got)
		if !reflect.DeepEqual(got, want[name]) {
			t.Errorf("%s is called from %q, not only from %q: does the new way pass the hold?", name, got, want[name])
		}
	}
	// The Mac app's side: the clipboard paste only from a terminal's insertLines, insertLines only from
	// typePhoneWords, and that only from the bridge's ask, which the core makes only in hostPhoneText (above). No
	// Swift file reads the hold.
	sources, _ := filepath.Glob(filepath.Join("..", "Sources", "AgentDeck", "*.swift"))
	if len(sources) == 0 {
		t.Fatal("no Swift sources")
	}
	count := map[string]int{}
	for _, name := range sources {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "paste-hold") || strings.Contains(text, "hold-key") {
			t.Errorf("%s reads the hold's files: a desk-written file must hold nothing", filepath.Base(name))
		}
		for _, call := range []string{"Deck.typePhoneWords(", ".insertLines("} {
			count[call] += strings.Count(text, call)
		}
		for at := strings.Index(text, "ClipboardPaste.paste("); at >= 0; at = indexAfter(text, "ClipboardPaste.paste(", at) {
			if in := text[strings.LastIndex(text[:at], "func "):]; !strings.HasPrefix(in, "func insertLines(") {
				t.Errorf("%s: a clipboard paste outside a terminal's insertLines: %.40q", filepath.Base(name), in)
			}
		}
	}
	if want := map[string]int{"Deck.typePhoneWords(": 1, ".insertLines(": 1}; !reflect.DeepEqual(count, want) {
		t.Errorf("the Mac app's paste is reached in %v places, not %v: does the new way pass the core's hold?", count, want)
	}
}

// indexAfter is where word is next found in text after at, or -1.
func indexAfter(text, word string, at int) int {
	if i := strings.Index(text[at+1:], word); i >= 0 {
		return at + 1 + i
	}
	return -1
}
