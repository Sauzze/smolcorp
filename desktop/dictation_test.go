package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The dictation service (dictation.go, FD1): the mic in a desk's input box in the office's own window. The words
// wait in the box for Send; nothing goes into the tab before. The old behaviour, the tab as target, is pinned by
// hostdictation_test.go and hostdictation_tmux_test.go, which this change leaves as they were. Never a real
// microphone, speech engine, clipboard or terminal app: the stand-in Mac app and a fake launcher answer for them.

// boxMac is the stand-in Mac app as the app will answer a recording for a desk's input box (FD2): a launcher's desk
// has no tab the app knows, and the box needs none, so it records all the same. Everything else is the stand-in's.
type boxMac struct{ *macStandIn }

func (b boxMac) call(request bridgeRequest) (bridgeAnswer, error) {
	if kind, _, _ := splitTab(request.Tab); request.Action == "dictation-start" && kind == "launcher" {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.asked = append(b.asked, request)
		return bridgeAnswer{OK: true, Started: !b.notStarting && !request.Busy}, nil
	}
	return b.macStandIn.call(request)
}

// boxDeck is a deck under the stand-in Mac app, which can dictate, with dictation on, and a Claude Code desk s1 in
// a launcher's window, done with its turn, with the fake launcher that takes what is typed there: a desk the
// mic's tab target leaves out, and the window may type into.
func boxDeck(t *testing.T) (*Deck, *macStandIn, *fakeLauncher) {
	t.Helper()
	mac := withMac(t)
	mac.facts.Dictation.Available, mac.facts.Dictation.Allowed = true, true
	host = boxMac{mac}
	settle := phoneTextSettle
	phoneTextSettle = time.Millisecond
	t.Cleanup(func() { phoneTextSettle = settle })
	d := newTestDeck(t)
	d.macFactsNow()
	d.mu.Lock()
	d.config.DictationEnabled = true
	d.mu.Unlock()
	f := newFakeLauncherAs(t, d, "L1", "/work/api", "", "", "idle-empty", true)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
	return d, mac, f
}

// nextNote is the next note on a stream, or a failure when none comes.
func nextNote(t *testing.T, stream *eventStream) dictationNote {
	t.Helper()
	select {
	case text := <-stream.notes:
		var note dictationNote
		if err := json.Unmarshal(text, &note); err != nil {
			t.Fatalf("a note that isn't JSON: %q", text)
		}
		return note
	case <-time.After(3 * time.Second):
		t.Fatal("no note came")
		return dictationNote{}
	}
}

// drained empties a stream's state messages, the snapshot it starts with, and says how many there were.
func drained(stream *eventStream) int {
	n := 0
	for {
		select {
		case <-stream.messages:
			n++
		default:
			return n
		}
	}
}

// typedNothing says whether the Mac app was never asked to type or press, and no terminal was written to.
func typedNothing(mac *macStandIn, f *fakeLauncher) bool {
	for _, action := range mac.actions() {
		switch action {
		case "insert", "press", "phone-text", "dictation-clean":
			return false
		}
	}
	return len(offBy(f)) == 0
}

// Holding the mic in a desk's box has the Mac app record, partial words go to the window's stream, and let go,
// what was heard is the window's final note: one recording, one line as heard (no clean-up: FD2's), kept nowhere.
func TestTheMicInADesksBoxStartsStopsAndTellsTheWindowItsWords(t *testing.T) {
	d, mac, f := boxDeck(t)
	window := d.attach(officeWindowID)
	drained(window)

	if status, says := d.ListenStart("s1"); status != http.StatusNoContent {
		t.Fatalf("start: %d %q", status, says)
	}
	if started := askedFor(mac, "dictation-start"); len(started) != 1 || started[0].Tab != "launcher:L1" || started[0].Language != "en_US" {
		t.Fatalf("the app was asked %+v", started)
	}
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: listeningNote}) {
		t.Errorf("first note %+v", note)
	}
	// Partial words, one line each; the next replaces the last. Nothing for words that say nothing.
	d.dictationPartial("fix the")
	d.dictationPartial("  fix the\nlogin  ")
	d.dictationPartial("   ")
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: partialNote, Text: "fix the"}) {
		t.Errorf("partial %+v", note)
	}
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: partialNote, Text: "fix the login"}) {
		t.Errorf("partial %+v", note)
	}
	select {
	case note := <-window.notes:
		t.Errorf("a note for words that said nothing: %+v", note)
	default:
	}

	// One microphone: a second start is refused in the deck's words and the first goes on.
	if status, says := d.ListenStart("s1"); status != http.StatusConflict || says != dictationCouldntStart {
		t.Errorf("a second start: %d %q", status, says)
	}
	mac.mu.Lock()
	mac.heard = "  fix the\nlogin bug "
	mac.mu.Unlock()
	if status, _ := d.ListenStop("s1"); status != http.StatusNoContent {
		t.Fatalf("stop: %d", status)
	}
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: finalNote, Text: "fix the login bug"}) {
		t.Errorf("final %+v", note)
	}
	if len(askedFor(mac, "dictation-stop")) != 1 {
		t.Errorf("%d stops", len(askedFor(mac, "dictation-stop")))
	}
	d.mu.Lock()
	recording := d.dictating != nil
	d.mu.Unlock()
	if recording {
		t.Error("still recording after the stop")
	}
	// Words that arrive once it is over go nowhere, and a stop with nothing recording is nothing.
	d.dictationPartial("too late")
	if status, _ := d.ListenStop("s1"); status != http.StatusNoContent || len(askedFor(mac, "dictation-stop")) != 1 {
		t.Errorf("a stop with nothing recording: %d, %d stops", status, len(askedFor(mac, "dictation-stop")))
	}
	select {
	case note := <-window.notes:
		t.Errorf("a note after the end: %+v", note)
	case <-time.After(50 * time.Millisecond):
	}

	// Nothing heard, or the app failing to give it, is a final note with no words: the box stops listening.
	mac.mu.Lock()
	mac.heard = ""
	mac.mu.Unlock()
	d.ListenStart("s1")
	nextNote(t, window)
	d.ListenStop("s1")
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: finalNote}) {
		t.Errorf("nothing heard: %+v", note)
	}
	if !typedNothing(mac, f) {
		t.Error("something was typed")
	}
}

// The recording's cap, and the window going away, end a box's recording as they end a tab's: the words reach the
// window on a stream that's still there, and a recording of another desk isn't touched by this desk's stop.
func TestABoxRecordingEndsAtItsCapOrWhenTheWindowLeavesAndIsNeverAnotherDesks(t *testing.T) {
	saved := dictationLongest
	dictationLongest = 100 * time.Millisecond
	t.Cleanup(func() { dictationLongest = saved })
	d, mac, _ := boxDeck(t)
	mac.heard = "capped"
	window := d.attach(officeWindowID)
	drained(window)
	if status, says := d.ListenStart("s1"); status != http.StatusNoContent {
		t.Fatalf("%d %q", status, says)
	}
	nextNote(t, window)
	if note := nextNote(t, window); note != (dictationNote{Session: "s1", Phase: finalNote, Text: "capped"}) {
		t.Errorf("at its cap: %+v", note)
	}

	dictationLongest = time.Hour
	d.ListenStart("s1")
	nextNote(t, window)
	// Another desk's stop leaves it recording.
	d.ListenStop("s2")
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac, "dictation-stop")) != 1 {
		t.Fatal("another desk's stop ended it")
	}
	// A phone leaving is not the window leaving.
	phone := d.attach("phone-1")
	d.detach(phone)
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac, "dictation-stop")) != 1 {
		t.Fatal("a phone leaving ended it")
	}
	d.detach(window)
	eventually(t, "stopped as the window left", func() bool { return len(askedFor(mac, "dictation-stop")) == 2 })
}

// Neither a phone's mic nor the deck's mic let go ends a box's recording, and a tab's recording is not the box's to
// stop: each is its own, and there's one microphone.
func TestABoxAndATabRecordingAreEachOwn(t *testing.T) {
	d, mac, _ := boxDeck(t)
	d.ListenStart("s1")
	if status, _ := d.PerformFrom("phone-1", "mic", "s1", map[string][]string{}); status != http.StatusNoContent {
		t.Errorf("the mic let go: %d", status)
	}
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac, "dictation-stop")) != 0 {
		t.Fatal("the deck's mic let go ended the box's recording")
	}
	d.ListenStop("s1")
	eventually(t, "stopped", func() bool { return len(askedFor(mac, "dictation-stop")) == 1 })

	// The other way: a tab's recording, on a desk the mic dictates for, isn't ended by the window's stop.
	d2, mac2, tab := dictationDeck(t)
	if status, _ := d2.PerformFrom("phone-1", "mic", "codex-1", map[string][]string{"phase": {"down"}}); status != http.StatusNoContent {
		t.Fatalf("the mic held: %d", status)
	}
	if status, says := d2.ListenStart("codex-1"); status != http.StatusConflict || says != dictationCouldntStart {
		t.Errorf("a box start while a tab records: %d %q", status, says)
	}
	d2.ListenStop("codex-1")
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac2, "dictation-stop")) != 0 {
		t.Fatal("the window's stop ended a tab's recording")
	}
	mac2.heard = "to the tab"
	d2.Perform("mic", "", map[string][]string{})
	eventually(t, "typed into its tab", func() bool { return len(askedFor(mac2, "insert")) == 1 })
	if insert := askedFor(mac2, "insert")[0]; insert.Tab != tab || insert.Text != "to the tab" {
		t.Errorf("typed %+v", insert)
	}
}

// What a box hears goes to the window's stream alone: a phone's stream gets no note, the snapshot every one of them
// shares says nothing of the words, and no change is announced for them (no push, no webhook, no viewer).
func TestWhatABoxHearsIsTheWindowsAlone(t *testing.T) {
	d, mac, _ := boxDeck(t)
	window, phone := d.attach(officeWindowID), d.attach("phone-1")
	drained(window)
	drained(phone)
	var changes atomic.Int32
	d.mu.Lock()
	d.OnChange = func() { changes.Add(1) }
	d.mu.Unlock()
	mac.heard = "secret words"
	d.ListenStart("s1")
	d.dictationPartial("secret")
	d.dictationPartial("secret words")
	d.ListenStop("s1")
	for range 4 {
		nextNote(t, window)
	}
	if got := len(phone.notes); got != 0 {
		t.Errorf("%d notes on a phone's stream", got)
	}
	if got := drained(phone); got != 0 {
		t.Errorf("%d state messages on a phone's stream: the words started a change", got)
	}
	d.mu.Lock()
	state := string(d.snapshot())
	d.mu.Unlock()
	if strings.Contains(state, "secret") {
		t.Error("the words are in the shared state")
	}
	time.Sleep(50 * time.Millisecond)
	if got := changes.Load(); got != 0 {
		t.Errorf("%d changes announced", got)
	}
}

// The words never go into the tab until Send: a box on a launcher's desk, which the tab target leaves out, hears
// them and types nothing; Send, the window's /talk/say (typeText), is what puts them in, with Return.
func TestTheInputBoxTargetTypesNothingUntilSend(t *testing.T) {
	d, mac, f := boxDeck(t)
	// The tab target still leaves a launcher's desk out, as the Swift engine had it.
	d.mu.Lock()
	leaves := !d.dictatesFor("launcher:L1", Codex)
	d.mu.Unlock()
	if !leaves {
		t.Fatal("the tab target dictates in a launcher's window")
	}
	window := d.attach(officeWindowID)
	drained(window)
	mac.heard = "fix the login bug"
	if status, says := d.ListenStart("s1"); status != http.StatusNoContent {
		t.Fatalf("a launcher desk's box: %d %q", status, says)
	}
	nextNote(t, window)
	d.ListenStop("s1")
	if note := nextNote(t, window); note.Phase != finalNote || note.Text != "fix the login bug" {
		t.Fatalf("final %+v", note)
	}
	if !typedNothing(mac, f) {
		t.Fatalf("typed before Send: the app was asked %v, the desk got %q", mac.actions(), f.written())
	}
	if status, says := d.TypeWindowText("fix the login bug", "s1", nil); status != http.StatusNoContent {
		t.Fatalf("Send: %d %q", status, says)
	}
	if got := f.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("fix the login bug"), keyBytes[Enter]}) {
		t.Errorf("Send typed %q", got)
	}
}

// Send keeps the never-type-into-a-prompt rule: a desk asking you something takes no words, whatever was dictated,
// and its Return isn't sent either. Listening itself goes on while it asks: the words wait in the box.
func TestSendRefusesWhileTheDeskAsks(t *testing.T) {
	d, mac, f := boxDeck(t)
	window := d.attach(officeWindowID)
	drained(window)
	asksAt(d, "launcher:L1", 30)
	mac.heard = "yes, do that"
	if status, says := d.ListenStart("s1"); status != http.StatusNoContent {
		t.Fatalf("listening while it asks: %d %q", status, says)
	}
	nextNote(t, window)
	d.ListenStop("s1")
	if note := nextNote(t, window); note.Text != "yes, do that" {
		t.Fatalf("final %+v", note)
	}
	if status, says := d.TypeWindowText("yes, do that", "s1", nil); status != http.StatusConflict || says != windowAnswerFirst {
		t.Errorf("Send while asking: %d %q", status, says)
	}
	if !typedNothing(mac, f) {
		t.Error("something was typed while it asks")
	}
	// Answered, it takes them.
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 40)
	if status, says := d.TypeWindowText("yes, do that", "s1", nil); status != http.StatusNoContent {
		t.Errorf("Send once it's answered: %d %q", status, says)
	}
}

// A desk the box can't send to isn't listened for: none there, a hire in T3 Code's, a headless run.
func TestTheBoxIsRefusedWhereSendCouldNotGo(t *testing.T) {
	d, mac, _ := boxDeck(t)
	if status, says := d.ListenStart("nobody"); status != http.StatusNotFound || says != "Session not found" {
		t.Errorf("no such desk: %d %q", status, says)
	}
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "h1", "cwd": "/work/bg"}, "claude", "headless:run-1", "", 50)
	if status, says := d.ListenStart("h1"); status != http.StatusConflict || says != headlessNoControls {
		t.Errorf("headless: %d %q", status, says)
	}
	if len(askedFor(mac, "dictation-start")) != 0 {
		t.Error("the Mac was asked to record for a desk nothing can be sent to")
	}
	// The app's own no is the deck's sentence, and leaves nothing recording.
	mac.notStarting = true
	if status, says := d.ListenStart("s1"); status != http.StatusConflict || says != dictationCouldntStart {
		t.Errorf("not started: %d %q", status, says)
	}
	mac.notStarting = false
	d.mu.Lock()
	recording := d.dictating != nil
	d.mu.Unlock()
	if recording {
		t.Error("a start that failed left a recording")
	}
}

// Dictation off in setup, or a Mac that can't, is the mic's own sentence; a test copy never records either.
func TestTheBoxNeedsDictationOnTheMac(t *testing.T) {
	d, mac, _ := boxDeck(t)
	d.mu.Lock()
	d.config.DictationEnabled = false
	d.mu.Unlock()
	if status, says := d.ListenStart("s1"); status != http.StatusConflict || says != dictationCouldntStart {
		t.Errorf("off in setup: %d %q", status, says)
	}
	if len(askedFor(mac, "dictation-start")) != 0 {
		t.Error("the app was asked to record with dictation off")
	}
}

// listenAsk is POST /talk/listen through the whole router, as a caller with key from where.
func listenAsk(t *testing.T, d *Deck, query, key, host, from string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/talk/listen?" + query
	if key != "" {
		target += "&t=" + key
	}
	return serve(t, d, call{method: "POST", target: target, host: host, from: from})
}

// POST /talk/listen is the window's alone, on this computer with its key: a phone's key gets the window's sentence,
// no key is 401, the window's key from another device or through a proxy too; nothing is recorded for any of them.
// The window's own start and stop answer 204; anything but a phase is a 400.
func TestTalkListenIsTheWindowsAlone(t *testing.T) {
	// The proxied and ts.net asks below come through the real `tailscale serve` (a stand-in peer, SN1): even then, the
	// window's key isn't taken. From a program here they are refused before it (peerowner_test.go).
	standInTailscalePeer(t)
	d, mac, _ := boxDeck(t)
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	phone := pair(t, d)
	start := "session=s1&phase=start"
	if got := listenAsk(t, d, start, "", "localhost:7710", thisComputer); got.Code != http.StatusUnauthorized {
		t.Errorf("no key: %d", got.Code)
	}
	if got := listenAsk(t, d, start, phone, deckOnWiFi, aPhone); got.Code != http.StatusForbidden || got.Body.String() != talkRefusal(d) {
		t.Errorf("a phone's key: %d %q", got.Code, got.Body)
	}
	if got := listenAsk(t, d, start, window, deckOnWiFi, aPhone); got.Code != http.StatusUnauthorized {
		t.Errorf("the window's key from another device: %d", got.Code)
	}
	proxied := serve(t, d, call{method: "POST", target: "/talk/listen?" + start + "&t=" + window, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{"X-Forwarded-For": "100.64.0.9"}})
	if proxied.Code != http.StatusUnauthorized {
		t.Errorf("the window's key through a proxy: %d", proxied.Code)
	}
	if len(askedFor(mac, "dictation-start")) != 0 {
		t.Fatal("the Mac was asked to record for a request that wasn't the window's")
	}
	if got := listenAsk(t, d, "session=s1&phase=sideways", window, "localhost:7710", thisComputer); got.Code != http.StatusBadRequest {
		t.Errorf("a phase that isn't one: %d %q", got.Code, got.Body)
	}
	if got := listenAsk(t, d, start, window, "localhost:7710", thisComputer); got.Code != http.StatusNoContent {
		t.Fatalf("the window's start: %d %q", got.Code, got.Body)
	}
	if got := listenAsk(t, d, "session=s1&phase=stop", window, "localhost:7710", thisComputer); got.Code != http.StatusNoContent {
		t.Errorf("the window's stop: %d %q", got.Code, got.Body)
	}
	eventually(t, "stopped", func() bool { return len(askedFor(mac, "dictation-stop")) == 1 })
}

// Linux and Windows have no desktop microphone (1.26, answer 15): the route says so, plainly, and nothing starts. Those
// builds have no Mac app, and neither has the deck here.
func TestLinuxAndWindowsHaveNoDesktopMicForTheBox(t *testing.T) {
	d, f := windowDeck(t)
	if status, says := d.ListenStart("s1"); status != http.StatusConflict || says != listenOff {
		t.Errorf("no Mac app: %d %q", status, says)
	}
	if status, says := d.ListenStop("s1"); status != http.StatusNoContent || says != "" {
		t.Errorf("a stop with nothing: %d %q", status, says)
	}
	d.mu.Lock()
	window := d.access.WindowKey
	recording := d.dictating != nil
	d.mu.Unlock()
	got := listenAsk(t, d, "session=s1&phase=start", window, "localhost:7710", thisComputer)
	if got.Code != http.StatusConflict || got.Body.String() != listenOff {
		t.Errorf("the route: %d %q", got.Code, got.Body)
	}
	if recording || len(offBy(f)) != 0 {
		t.Errorf("recording %v, typed %q", recording, f.written())
	}
}
