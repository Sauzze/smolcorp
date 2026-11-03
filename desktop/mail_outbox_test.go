package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// sendFixture is Tests/fixtures/frontdesk/send.json, the file the Mac app's tests also read.
func sendFixture(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "send.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type mailClock struct{ t time.Time }

func (c *mailClock) now() time.Time          { return c.t }
func (c *mailClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func testOutbox(t *testing.T) (*mailOutbox, *mailClock, string) {
	t.Helper()
	dir := t.TempDir()
	clock := &mailClock{t: time.UnixMilli(1_800_000_000_000)}
	file := filepath.Join(dir, "outbox.json")
	o := newMailOutbox(file, func() mailLimits {
		return mailLimits{MaxBytes: mailDefaultMaxBytes, PerHour: mailDefaultPerHour, WaitHours: mailDefaultWaitHours}
	}, clock.now)
	return o, clock, file
}

func toRev(text, id string) mailRequest {
	return mailRequest{From: mailFromGM, To: "Rev", Text: text, ID: id, IDGiven: id != "",
		Resolve: func() (mailDest, *mailError) { return mailDest{ID: "abcdef123456", Name: "Rev"}, nil }}
}

func fromHire(hireID, name, to, text, id string, gm bool) mailRequest {
	return mailRequest{From: mailFromHire, SignerID: hireID, SignerName: name, To: to, Text: text, ID: id, IDGiven: id != "",
		Resolve: func() (mailDest, *mailError) { return mailDest{ID: to, Name: to, GM: gm}, nil }}
}

func mustAdd(t *testing.T, o *mailOutbox, req mailRequest) mailAdded {
	t.Helper()
	added, err := o.add(req)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	return added
}

func wantRefusal(t *testing.T, o *mailOutbox, req mailRequest, status int) *mailError {
	t.Helper()
	_, err := o.add(req)
	if err == nil || err.Status != status {
		t.Fatalf("got %v, want a %d", err, status)
	}
	return err
}

func jsonShape(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}

func TestAMessageIsAnsweredAsTheFixtureAndKeptInTheMacsFormat(t *testing.T) {
	o, _, file := testOutbox(t)
	spec := sendFixture(t)
	req := spec["request"].(map[string]any)
	added := mustAdd(t, o, toRev(req["text"].(string), req["id"].(string)))
	if !reflect.DeepEqual(jsonShape(t, added.Answer), jsonShape(t, func() any {
		// The fixture's answer says delivered: the answer here is what the store knows before any typing, queued.
		a := map[string]any{}
		for k, v := range spec["answer"].(map[string]any) {
			a[k] = v
		}
		a["state"] = "queued"
		return a
	}())) {
		t.Fatalf("answer %v, fixture %v", added.Answer, spec["answer"])
	}
	if !added.Deliver || added.ToGM || added.Duplicate {
		t.Fatalf("flags %+v: a new queued message is to be tried", added)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk struct {
		Version  int
		Messages []map[string]any
	}
	if err := json.Unmarshal(data, &onDisk); err != nil || onDisk.Version != 1 || len(onDisk.Messages) != 1 {
		t.Fatalf("file %s: %v", data, err)
	}
	var keys []string
	for k := range onDisk.Messages[0] {
		keys = append(keys, k)
	}
	for _, name := range []string{"id", "from", "to", "toName", "text", "at", "state", "size", "attempts", "nextAt", "note"} {
		if _, ok := onDisk.Messages[0][name]; !ok {
			t.Errorf("the message has no %q (has %v)", name, keys)
		}
	}
	if _, ok := onDisk.Messages[0]["fromID"]; ok {
		t.Error("a message from the GM has a fromID")
	}
	if onDisk.Messages[0]["at"] != float64(1_800_000_000_000) || onDisk.Messages[0]["state"] != "queued" || onDisk.Messages[0]["size"] != float64(38) {
		t.Errorf("message %v", onDisk.Messages[0])
	}
	// Keys come out sorted, like the Mac's: attempts is before id, which is before to.
	text := string(data)
	if !(strings.Index(text, `"attempts"`) < strings.Index(text, `"id"`) && strings.Index(text, `"id"`) < strings.Index(text, `"to"`) && strings.Index(text, `"messages"`) < strings.Index(text, `"version"`)) {
		t.Errorf("keys aren't sorted:\n%s", text)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
			t.Errorf("outbox.json is %v, want 0600", info.Mode().Perm())
		}
	}
	if _, err := os.Stat(file + ".tmp"); err == nil {
		t.Error("a temporary file was left behind")
	}
}

func TestAnOutboxIsReadBackAfterARestart(t *testing.T) {
	o, clock, file := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	again := newMailOutbox(file, o.limits, clock.now)
	if got, ok := again.get("a"); !ok || got.Text != "Hi" || got.State != mailQueued {
		t.Fatalf("read back %+v", got)
	}
}

func TestAnUnreadableOutboxIsSetAsideAndANewOneStarted(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "outbox.json")
	if err := os.WriteFile(file, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := newMailOutbox(file, func() mailLimits { return mailLimits{1000, 10, 24} }, nil)
	kept, err := os.ReadFile(filepath.Join(dir, "outbox.broken.json"))
	if err != nil || string(kept) != "{not json" {
		t.Fatalf("the broken file wasn't kept: %v %q", err, kept)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("the broken file is still outbox.json")
	}
	mustAdd(t, o, toRev("Hi", "a"))
	if len(o.list(nil, true)["messages"].([]map[string]any)) != 1 {
		t.Error("a new outbox wasn't started")
	}
}

func TestBlankToOrTextIsABadRequest(t *testing.T) {
	o, _, _ := testOutbox(t)
	for _, req := range []mailRequest{
		{From: mailFromGM, To: "", Text: "x"},
		{From: mailFromGM, To: "  ", Text: "x"},
		{From: mailFromGM, To: "Rev", Text: ""},
		{From: mailFromGM, To: "Rev", Text: " \n\t "},
	} {
		wantRefusal(t, o, req, 400)
	}
	if len(o.messages) != 0 {
		t.Error("a refused message was kept")
	}
}

func TestAHireWithoutAKeyIsRefused(t *testing.T) {
	o, _, _ := testOutbox(t)
	wantRefusal(t, o, mailRequest{From: mailFromHire, To: "gm", Text: "x"}, 403)
}

func TestTheIdUsesPlainCharactersAndIsMadeWhenMissing(t *testing.T) {
	o, _, _ := testOutbox(t)
	for _, bad := range []string{"a b", "a/b", "é", strings.Repeat("a", 65)} {
		req := toRev("x", bad)
		req.IDGiven = true
		wantRefusal(t, o, req, 400)
	}
	empty := toRev("x", "")
	empty.IDGiven = true
	wantRefusal(t, o, empty, 400)
	mustAdd(t, o, toRev("x", strings.Repeat("a", 63)+"."))
	mustAdd(t, o, toRev("x", "A-b_c.9"))
	made := mustAdd(t, o, toRev("x", "")).Answer["id"].(string)
	if !regexp.MustCompile(`^m-[0-9a-f]{12}$`).MatchString(made) {
		t.Fatalf("made id %q", made)
	}
	if other := mustAdd(t, o, toRev("x", "")).Answer["id"].(string); other == made {
		t.Fatal("two made ids are the same")
	}
}

func TestTheSameIdIsAnsweredAgainAndNeverAddedTwice(t *testing.T) {
	o, _, _ := testOutbox(t)
	spec := sendFixture(t)
	mustAdd(t, o, toRev("Hi", "one"))
	// Whatever became of it: even a left message is only answered again.
	attempt := o.start("one")
	attempt.Commit()
	attempt.Finish(mailResult{Outcome: mailCannot, Note: "no"})
	again := mustAdd(t, o, toRev("Something else", "one"))
	if !again.Duplicate || again.Deliver || again.ToGM {
		t.Fatalf("flags %+v", again)
	}
	if again.Answer["duplicate"] != true || again.Answer["state"] != "failed" || again.Answer["note"] != "no" || again.Message.Text != "Hi" {
		t.Fatalf("answer %v", again.Answer)
	}
	if len(o.messages) != 1 {
		t.Fatalf("%d messages kept", len(o.messages))
	}
	// The fixture's shape for the same id again: the answer plus duplicate:true.
	d := spec["same-id-again"].(map[string]any)
	if d["duplicate"] != true || len(d) != len(spec["answer"].(map[string]any))+1 {
		t.Fatalf("fixture %v", d)
	}
	o2, _, _ := testOutbox(t)
	mustAdd(t, o2, toRev(spec["request"].(map[string]any)["text"].(string), "gm-2026-11-01-001"))
	done := o2.start("gm-2026-11-01-001")
	done.Commit()
	done.Finish(mailResult{Outcome: mailSent})
	got := mustAdd(t, o2, toRev(spec["request"].(map[string]any)["text"].(string), "gm-2026-11-01-001")).Answer
	if !reflect.DeepEqual(jsonShape(t, got), jsonShape(t, d)) {
		t.Fatalf("answer %v, fixture %v", got, d)
	}
}

func TestAHireCannotReuseAnIdThatIsNotItsOwn(t *testing.T) {
	o, _, _ := testOutbox(t)
	mustAdd(t, o, toRev("From the GM", "shared"))
	mustAdd(t, o, fromHire("h1", "Rev", "gm", "mine", "h-1", true))
	err := wantRefusal(t, o, fromHire("h2", "Dev", "gm", "x", "shared", true), 409)
	if err.Message != "That id is already used. Pick another." {
		t.Errorf("message %q", err.Message)
	}
	wantRefusal(t, o, fromHire("h2", "Dev", "gm", "x", "h-1", true), 409)
	// Its own id again is only answered again.
	if again := mustAdd(t, o, fromHire("h1", "Rev", "gm", "mine", "h-1", true)); !again.Duplicate || again.ToGM {
		t.Fatalf("%+v", again)
	}
	// The GM may ask about any id.
	if again := mustAdd(t, o, toRev("x", "h-1")); !again.Duplicate {
		t.Fatalf("%+v", again)
	}
}

func TestAMessageOverTheLimitIsTooLarge(t *testing.T) {
	o, _, _ := testOutbox(t)
	max := sendFixture(t)["limits"].(map[string]any)["maxBytes"].(map[string]any)
	if max["setting"] != "mailMaxBytes" || int(max["default"].(float64)) != mailDefaultMaxBytes || int(max["status"].(float64)) != 413 {
		t.Fatalf("fixture %v", max)
	}
	mustAdd(t, o, toRev(strings.Repeat("a", mailDefaultMaxBytes), "fits"))
	err := wantRefusal(t, o, toRev(strings.Repeat("a", mailDefaultMaxBytes+1), "big"), 413)
	if !strings.Contains(err.Message, "16385 bytes") || !strings.Contains(err.Message, "16384") || !strings.Contains(err.Message, "mailMaxBytes") {
		t.Errorf("message %q", err.Message)
	}
	// Bytes, not letters.
	wantRefusal(t, o, toRev(strings.Repeat("é", mailDefaultMaxBytes/2+1), "bytes"), 413)
}

func TestASenderIsLimitedPerHourAndEachHireOnItsOwn(t *testing.T) {
	o, clock, _ := testOutbox(t)
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 3, WaitHours: 24} }
	per := sendFixture(t)["limits"].(map[string]any)["perHour"].(map[string]any)
	if per["setting"] != "mailPerHour" || int(per["default"].(float64)) != mailDefaultPerHour || int(per["status"].(float64)) != 429 {
		t.Fatalf("fixture %v", per)
	}
	for i := 0; i < 3; i++ {
		mustAdd(t, o, toRev("x", fmt.Sprintf("g%d", i)))
	}
	err := wantRefusal(t, o, toRev("x", "g3"), 429)
	if !strings.Contains(err.Message, "the GM") || !strings.Contains(err.Message, "mailPerHour") || !strings.Contains(err.Message, "3 messages") {
		t.Errorf("message %q", err.Message)
	}
	// The owner is another sender; a hire is its own.
	owner := toRev("x", "o1")
	owner.From = mailFromOwner
	mustAdd(t, o, owner)
	for i := 0; i < 3; i++ {
		mustAdd(t, o, fromHire("h1", "Rev", "gm", "x", fmt.Sprintf("a%d", i), true))
	}
	err = wantRefusal(t, o, fromHire("h1", "Rev", "gm", "x", "a3", true), 429)
	if !strings.Contains(err.Message, "hire Rev") {
		t.Errorf("message %q", err.Message)
	}
	mustAdd(t, o, fromHire("h2", "Dev", "gm", "x", "b0", true))
	// An hour later the oldest don't count.
	clock.advance(61 * time.Minute)
	mustAdd(t, o, toRev("x", "g3"))
	mustAdd(t, o, fromHire("h1", "Rev", "gm", "x", "a3", true))
}

func TestAMessageThatCannotBeWrittenIsNotKeptAsSent(t *testing.T) {
	clock := &mailClock{t: time.UnixMilli(1_800_000_000_000)}
	// The folder isn't there, so the file can't be written.
	o := newMailOutbox(filepath.Join(t.TempDir(), "missing", "outbox.json"), func() mailLimits { return mailLimits{1000, 10, 24} }, clock.now)
	err := wantRefusal(t, o, toRev("Hi", "a"), 500)
	if !strings.Contains(err.Message, "nothing was sent") {
		t.Errorf("message %q", err.Message)
	}
	if len(o.messages) != 0 {
		t.Fatal("it was kept in memory")
	}
	if len(o.due()) != 0 {
		t.Fatal("it is due for delivery")
	}
	wantRefusal(t, o, fromHire("h1", "Rev", "gm", "x", "g", true), 500)
	if len(o.messages) != 0 {
		t.Fatal("a message to the GM was kept in memory")
	}
}

func TestAHiresMessageToTheGMIsKeptDeliveredWithANote(t *testing.T) {
	o, _, _ := testOutbox(t)
	spec := sendFixture(t)["from-a-hire"].(map[string]any)["to-the-gm"].(map[string]any)
	req := spec["request"].(map[string]any)
	added := mustAdd(t, o, fromHire("abcdef123456", "Rev", req["to"].(string), req["text"].(string), req["id"].(string), true))
	if !reflect.DeepEqual(jsonShape(t, added.Answer), spec["answer"]) {
		t.Fatalf("answer %v, fixture %v", added.Answer, spec["answer"])
	}
	if !added.ToGM || added.Deliver || added.Duplicate || added.Message.FromID != "abcdef123456" || added.Message.FromName != "Rev" {
		t.Fatalf("%+v", added)
	}
	if len(o.due()) != 0 {
		t.Fatal("a message to the GM is due for delivery")
	}
	if o.start(req["id"].(string)) != nil {
		t.Fatal("a delivered message could be started")
	}
}

func TestTheTargetIsFoundAfterTheChecksAndItsRefusalPassesThrough(t *testing.T) {
	o, _, _ := testOutbox(t)
	called := false
	req := toRev("x", "a")
	req.Resolve = func() (mailDest, *mailError) { called = true; return mailDest{}, &mailError{404, "no such hire"} }
	wantRefusal(t, o, req, 404)
	if !called || len(o.messages) != 0 {
		t.Fatal("not resolved, or kept anyway")
	}
	// A bad id never gets as far as the target.
	called = false
	bad := req
	bad.ID, bad.IDGiven = "a b", true
	wantRefusal(t, o, bad, 400)
	if called {
		t.Fatal("resolved before the id was checked")
	}
}

func TestTheWireTextAndTheSenderLine(t *testing.T) {
	spec := sendFixture(t)
	gm := mailMessage{ID: "gm-2026-11-01-001", From: mailFromGM, Text: "Please look at the login change first."}
	if got := mailWire(gm); got != spec["text-the-hire-gets"] {
		t.Fatalf("wire %q, fixture %q", got, spec["text-the-hire-gets"])
	}
	owner := mailMessage{ID: "<id>", From: mailFromOwner, Text: "<text>"}
	if got := mailWire(owner); !strings.HasPrefix(got, spec["text-from-the-page"].(string)) || got != "— from the owner (Smolcorp), message <id>\n\n<text>" {
		t.Fatalf("wire %q", got)
	}
	hire := mailMessage{ID: "<id>", From: mailFromHire, FromID: "abcdef123456", FromName: "Rev", Text: "<text>"}
	if got := mailWire(hire); got != spec["from-a-hire"].(map[string]any)["text-the-other-hire-gets"] {
		t.Fatalf("wire %q", got)
	}
	// A hire called "the GM" still reads as a hire; one with no name or id shows question marks.
	if got := (mailMessage{From: mailFromHire, FromName: "the GM", FromID: "x1"}).fromLine(); got != "hire the GM [x1]" {
		t.Errorf("line %q", got)
	}
	if got := (mailMessage{From: mailFromHire}).fromLine(); got != "hire ? [?]" {
		t.Errorf("line %q", got)
	}
}

func TestSettlingADelivery(t *testing.T) {
	at := float64(1_800_000_000_000)
	fresh := func() mailMessage {
		return mailMessage{ID: "a", ToName: "Rev", At: at, State: mailQueued, Note: "old", NextAt: 5}
	}
	m := fresh()
	settleMail(&m, mailResult{Outcome: mailSent}, at, 24, true)
	if m.State != mailDelivered || m.NextAt != 0 || m.Note != "" {
		t.Errorf("delivered: %+v", m)
	}
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailSent, Note: "sent as one line"}, at, 24, true)
	if m.State != mailDelivered || m.Note != "sent as one line" {
		t.Errorf("deliveredAs: %+v", m)
	}
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailCannot, Note: "never"}, at, 24, false)
	if m.State != mailFailed || m.Note != "never" || m.NextAt != 0 || m.Attempts != 0 {
		t.Errorf("failed: %+v", m)
	}
	// Waiting is not an attempt, stays queued, and is looked at again at once.
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailWaiting, Note: "waiting: at work"}, at+1000, 24, false)
	if m.State != mailQueued || m.Attempts != 0 || m.NextAt != 0 || m.Note != "waiting: at work" {
		t.Errorf("waiting: %+v", m)
	}
	// A day after it was sent: left, with the words, and never again.
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailWaiting, Note: "waiting: at work"}, at+24*3_600_000-1, 24, false)
	if m.State != mailQueued {
		t.Errorf("one millisecond short of the wait: %+v", m)
	}
	settleMail(&m, mailResult{Outcome: mailWaiting, Note: "waiting: at work"}, at+24*3_600_000, 24, false)
	if m.State != mailLeft || m.Attempts != 0 || m.NextAt != 0 ||
		m.Note != "Left after waiting 24 hours for Rev to be free, not sent again: waiting: at work" {
		t.Errorf("waited a day: %+v", m)
	}
	// The wait is a setting.
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailWaiting, Note: "w"}, at+2*3_600_000, 2, false)
	if m.State != mailLeft || !strings.Contains(m.Note, "waiting 2 hours") {
		t.Errorf("two hours: %+v", m)
	}
	// Trouble counts, and is tried again in 30 seconds.
	m = fresh()
	settleMail(&m, mailResult{Outcome: mailTrouble, Note: "tmux down"}, at, 24, false)
	if m.State != mailQueued || m.Attempts != 1 || m.NextAt != at+30_000 || m.Note != "tmux down" {
		t.Errorf("trouble: %+v", m)
	}
	// Already counted by commit: not counted twice.
	m = fresh()
	m.Attempts = 1
	settleMail(&m, mailResult{Outcome: mailTrouble, Note: "tmux down"}, at, 24, true)
	if m.Attempts != 1 {
		t.Errorf("counted twice: %+v", m)
	}
	// After 6 tries: left.
	m = fresh()
	m.Attempts = 5
	settleMail(&m, mailResult{Outcome: mailTrouble, Note: "tmux down"}, at, 24, false)
	if m.State != mailLeft || m.Attempts != 6 || m.NextAt != 0 || m.Note != "Left after 6 tries, not sent again: tmux down" {
		t.Errorf("sixth: %+v", m)
	}
	m = fresh()
	m.Attempts = 4
	settleMail(&m, mailResult{Outcome: mailTrouble, Note: "x"}, at, 24, false)
	if m.State != mailQueued || m.Attempts != 5 {
		t.Errorf("fifth: %+v", m)
	}
}

func TestTheTryIsCountedAndOnDiskBeforeAnythingIsTyped(t *testing.T) {
	o, clock, file := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	attempt := o.start("a")
	if attempt == nil || attempt.Text != "— from the GM (Smolcorp), message a\n\nHi" {
		t.Fatalf("attempt %+v", attempt)
	}
	// Starting doesn't count; waiting never does.
	if got, _ := o.get("a"); got.Attempts != 0 {
		t.Fatal("counted before commit")
	}
	if !attempt.Commit() || !attempt.Commit() {
		t.Fatal("commit refused")
	}
	// On disk, as a restart would see it, before the try is over.
	onDisk := newMailOutbox(file, o.limits, clock.now)
	got, _ := onDisk.get("a")
	if got.Attempts != 1 || got.NextAt != float64(clock.now().UnixMilli()+30_000) {
		t.Fatalf("on disk %+v", got)
	}
	// Trouble after commit isn't counted again.
	attempt.Finish(mailResult{Outcome: mailTrouble, Note: "paste failed"})
	if got, _ := o.get("a"); got.Attempts != 1 || got.State != mailQueued {
		t.Fatalf("after trouble %+v", got)
	}
	// And a try that never typed (trouble without commit) is counted by finishing.
	next := o.start("a")
	next.Finish(mailResult{Outcome: mailTrouble, Note: "no window"})
	if got, _ := o.get("a"); got.Attempts != 2 {
		t.Fatalf("after a try without commit %+v", got)
	}
}

func TestIfTheCountCannotBeSavedNothingMayBeTyped(t *testing.T) {
	o, _, _ := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	attempt := o.start("a")
	o.file = filepath.Join(filepath.Dir(o.file), "gone", "outbox.json")
	if attempt.Commit() {
		t.Fatal("commit said yes with the file unwritable")
	}
	if got, _ := o.get("a"); got.Attempts != 0 || got.NextAt != 0 {
		t.Fatalf("counted anyway %+v", got)
	}
	// Still refused after the attempt is over.
	attempt.Finish(mailResult{Outcome: mailWaiting, Note: "w"})
	if attempt.Commit() {
		t.Fatal("commit said yes after finish")
	}
}

func TestWaitingIsNeverCountedAndOneIsInFlightPerId(t *testing.T) {
	o, clock, _ := testOutbox(t)
	mustAdd(t, o, toRev("1", "a"))
	mustAdd(t, o, toRev("2", "b"))
	mustAdd(t, o, toRev("3", "c"))
	ids := func() string {
		var out []string
		for _, m := range o.due() {
			out = append(out, m.ID)
		}
		return strings.Join(out, ",")
	}
	if ids() != "a,b,c" {
		t.Fatalf("due %s, want the outbox order", ids())
	}
	first := o.start("a")
	if o.start("a") != nil {
		t.Fatal("two tries out for one id")
	}
	if ids() != "b,c" {
		t.Fatalf("due %s, want a left out while it is in flight", ids())
	}
	first.Finish(mailResult{Outcome: mailWaiting, Note: "waiting: at work"})
	// Waiting: due again at once, on every look, and no attempt.
	if ids() != "a,b,c" {
		t.Fatalf("due %s", ids())
	}
	if got, _ := o.get("a"); got.Attempts != 0 || got.State != mailQueued {
		t.Fatalf("%+v", got)
	}
	// Trouble: not due for 30 seconds.
	t2 := o.start("b")
	t2.Commit()
	t2.Finish(mailResult{Outcome: mailTrouble, Note: "x"})
	if ids() != "a,c" {
		t.Fatalf("due %s, want b held back", ids())
	}
	clock.advance(29 * time.Second)
	if ids() != "a,c" {
		t.Fatalf("due %s at 29 s", ids())
	}
	clock.advance(time.Second)
	if ids() != "a,b,c" {
		t.Fatalf("due %s at 30 s", ids())
	}
	// Delivered, left and failed are never due, and never sent again by themselves.
	s := o.start("a")
	s.Commit()
	s.Finish(mailResult{Outcome: mailSent})
	s = o.start("b")
	s.Finish(mailResult{Outcome: mailCannot, Note: "no"})
	if ids() != "c" {
		t.Fatalf("due %s", ids())
	}
	if o.start("a") != nil || o.start("b") != nil || o.start("nope") != nil {
		t.Fatal("a settled message could be started")
	}
	clock.advance(48 * time.Hour)
	if ids() != "c" {
		t.Fatalf("due %s a day later", ids())
	}
}

func TestSixTroublesLeaveAMessageAndItIsNotTriedAgain(t *testing.T) {
	o, clock, _ := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	for i := 1; i <= 6; i++ {
		due := o.due()
		if len(due) != 1 {
			t.Fatalf("try %d: %d due", i, len(due))
		}
		attempt := o.start("a")
		attempt.Commit()
		attempt.Finish(mailResult{Outcome: mailTrouble, Note: "tmux didn't answer"})
		clock.advance(31 * time.Second)
	}
	got, _ := o.get("a")
	if got.State != mailLeft || got.Attempts != 6 || !strings.Contains(got.Note, "not sent again") {
		t.Fatalf("%+v", got)
	}
	if len(o.due()) != 0 {
		t.Fatal("a left message is due")
	}
}

func TestAMessageThatWaitsADayIsLeftOnTheNextLook(t *testing.T) {
	o, clock, _ := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	clock.advance(24*time.Hour - time.Second)
	o.start("a").Finish(mailResult{Outcome: mailWaiting, Note: "waiting: a draft is in its box"})
	if got, _ := o.get("a"); got.State != mailQueued {
		t.Fatalf("%+v", got)
	}
	clock.advance(time.Second)
	o.start("a").Finish(mailResult{Outcome: mailWaiting, Note: "waiting: a draft is in its box"})
	if got, _ := o.get("a"); got.State != mailLeft || got.Attempts != 0 {
		t.Fatalf("%+v", got)
	}
	// The wait is the setting.
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 100, WaitHours: 2} }
	mustAdd(t, o, toRev("Hi", "b"))
	clock.advance(2 * time.Hour)
	o.start("b").Finish(mailResult{Outcome: mailWaiting, Note: "w"})
	if got, _ := o.get("b"); got.State != mailLeft {
		t.Fatalf("%+v", got)
	}
}

func TestFailingAQueuedMessageForAHireThatIsGone(t *testing.T) {
	o, _, _ := testOutbox(t)
	mustAdd(t, o, toRev("Hi", "a"))
	mustAdd(t, o, toRev("Hi", "b"))
	inFlight := o.start("b")
	if !o.fail("a", "Rev is gone.") || o.fail("b", "Rev is gone.") {
		t.Fatal("fail: a queued one fails, one in flight is left to finish")
	}
	inFlight.Finish(mailResult{Outcome: mailSent})
	if o.fail("b", "late") {
		t.Fatal("a delivered message was failed")
	}
	if got, _ := o.get("a"); got.State != mailFailed || got.Note != "Rev is gone." || got.NextAt != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestTheListIsOldestFirstWithoutTheTextForATool(t *testing.T) {
	o, _, _ := testOutbox(t)
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 100000, WaitHours: 24} }
	for i := 0; i < 130; i++ {
		mustAdd(t, o, toRev(fmt.Sprintf("text %d", i), fmt.Sprintf("m%03d", i)))
	}
	mustAdd(t, o, fromHire("h1", "Rev", "gm", "to you", "hx", true))
	rows := func(limit int, text bool) []map[string]any {
		return o.list(&limit, text)["messages"].([]map[string]any)
	}
	// No limit given: 100. A limit of 0 or less is 1, as the Mac's outboxJSON(limit:) has it.
	if none := o.list(nil, true)["messages"].([]map[string]any); len(none) != 100 {
		t.Fatalf("no limit: %d rows", len(none))
	}
	for _, zero := range []int{0, -5} {
		if one := rows(zero, true); len(one) != 1 || one[0]["id"] != "hx" {
			t.Fatalf("limit %d: %v", zero, one)
		}
	}
	all := rows(100, true)
	if len(all) != 100 || all[0]["id"] != "m031" || all[99]["id"] != "hx" {
		t.Fatalf("default list: %d rows from %v to %v", len(all), all[0]["id"], all[len(all)-1]["id"])
	}
	few := rows(3, false)
	if len(few) != 3 || few[0]["id"] != "m128" || few[2]["id"] != "hx" {
		t.Fatalf("limit 3: %v", few)
	}
	if _, has := few[0]["text"]; has {
		t.Error("the tool's list has the text")
	}
	if rows(3, true)[0]["text"] != "text 128" {
		t.Error("the GM's list lacks the text")
	}
	row := few[2]
	if row["from"] != "hire" || row["fromID"] != "h1" || row["fromName"] != "Rev" || row["note"] != mailGMNote || row["state"] != "delivered" ||
		row["to"] != "gm" || row["toName"] != "the GM" || row["attempts"] != 0 || row["size"] != 6 || row["at"] != int64(1_800_000_000_000) {
		t.Fatalf("row %v", row)
	}
	if _, has := few[0]["note"]; has {
		t.Error("an empty note is listed")
	}
	if _, has := few[0]["fromID"]; has {
		t.Error("a GM message has a fromID")
	}
	if len(rows(5000, false)) != 131 {
		t.Error("a large limit lost messages")
	}
}

func TestTheOutboxKeepsTheLatest2000(t *testing.T) {
	o, _, file := testOutbox(t)
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 100000, WaitHours: 24} }
	for i := 0; i < mailKeepMost+3; i++ {
		o.messages = append(o.messages, mailMessage{ID: fmt.Sprintf("old%d", i), From: mailFromGM, State: mailDelivered})
	}
	mustAdd(t, o, toRev("new", "new"))
	if len(o.messages) != mailKeepMost || o.messages[0].ID != "old4" || o.messages[mailKeepMost-1].ID != "new" {
		t.Fatalf("%d kept, first %s", len(o.messages), o.messages[0].ID)
	}
	if again := newMailOutbox(file, o.limits, nil); len(again.messages) != mailKeepMost {
		t.Fatalf("%d on disk", len(again.messages))
	}
}

func TestTheOutboxIsSafeToUseFromRoutesAndAPump(t *testing.T) {
	o, _, _ := testOutbox(t)
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 100000, WaitHours: 24} }
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if _, err := o.add(toRev("x", fmt.Sprintf("w%d-%d", w, i))); err != nil {
					t.Error(err)
				}
				o.list(intp(10), false)
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			for _, m := range o.due() {
				if a := o.start(m.ID); a != nil {
					a.Commit()
					a.Finish(mailResult{Outcome: mailSent})
				}
			}
		}
	}()
	wg.Wait()
	if len(o.messages) != 80 {
		t.Fatalf("%d messages", len(o.messages))
	}
}

func TestTheMailSettingsHaveTheMacsDefaultsAndAtLeastOne(t *testing.T) {
	isolate(t)
	c := loadConfig()
	if c.MailMaxBytes != 16384 || c.MailPerHour != 200 || c.MailWaitHours != 24 {
		t.Fatalf("defaults %d %d %g", c.MailMaxBytes, c.MailPerHour, c.MailWaitHours)
	}
	if got := mailLimitsOf(c); got != (mailLimits{16384, 200, 24}) {
		t.Fatalf("limits %+v", got)
	}
	if err := os.WriteFile(configFile, []byte(`{"mailMaxBytes": 0, "mailPerHour": -4, "mailWaitHours": 0.5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c = loadConfig()
	if c.MailMaxBytes != 1 || c.MailPerHour != 1 || c.MailWaitHours != 1 {
		t.Fatalf("clamped %d %d %g", c.MailMaxBytes, c.MailPerHour, c.MailWaitHours)
	}
	if err := os.WriteFile(configFile, []byte(`{"mailMaxBytes": 500, "mailPerHour": 7, "mailWaitHours": 6}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c = loadConfig()
	if got := mailLimitsOf(c); got != (mailLimits{500, 7, 6}) {
		t.Fatalf("set %+v", got)
	}
	// Saved back under the Mac's names, and not kept as unknown.
	c.save()
	data, _ := os.ReadFile(configFile)
	var saved map[string]any
	_ = json.Unmarshal(data, &saved)
	if saved["mailMaxBytes"] != float64(500) || saved["mailPerHour"] != float64(7) || saved["mailWaitHours"] != float64(6) {
		t.Fatalf("saved %s", data)
	}
	// A Config that was never loaded still gives sensible limits.
	if got := mailLimitsOf(&Config{}); got != (mailLimits{16384, 200, 24}) {
		t.Fatalf("zero config %+v", got)
	}
}

func TestTheFixtureTellsWhatWaitingDoes(t *testing.T) {
	waiting := sendFixture(t)["waiting"].(string)
	if !strings.Contains(waiting, "mailWaitHours") || !strings.Contains(waiting, "not a try") {
		t.Fatalf("the fixture says %q", waiting)
	}
	if strings.Contains(waiting, "is tried every 30 seconds, and is left after 6 tries") {
		t.Fatal("the fixture still says a waiting message is left after 6 tries")
	}
}

func intp(n int) *int { return &n }

func TestTheTargetIsFoundWithTheOutboxUnlocked(t *testing.T) {
	o, _, _ := testOutbox(t)
	req := toRev("x", "a")
	req.Resolve = func() (mailDest, *mailError) {
		// A Resolve that asks the outbox something (as one that takes the Deck's lock may end up doing).
		o.list(intp(1), false)
		return mailDest{ID: "abcdef123456", Name: "Rev"}, nil
	}
	done := make(chan struct{})
	go func() { defer close(done); mustAdd(t, o, req) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("add hangs while its Resolve calls the outbox")
	}
}

func TestTwoSendsWithTheSameIdStoreOneAndAnswerTheOtherAsDuplicate(t *testing.T) {
	o, _, _ := testOutbox(t)
	// Both sends are inside Resolve at the same moment, so both passed the first look.
	var inside sync.WaitGroup
	inside.Add(2)
	req := toRev("same", "twin")
	req.Resolve = func() (mailDest, *mailError) {
		inside.Done()
		inside.Wait()
		return mailDest{ID: "abcdef123456", Name: "Rev"}, nil
	}
	results := make(chan mailAdded, 2)
	for i := 0; i < 2; i++ {
		go func() {
			added, err := o.add(req)
			if err != nil {
				t.Errorf("add: %v", err)
			}
			results <- added
		}()
	}
	first, second := <-results, <-results
	if first.Duplicate == second.Duplicate {
		t.Fatalf("duplicate flags %v and %v: want exactly one", first.Duplicate, second.Duplicate)
	}
	if len(o.messages) != 1 {
		t.Fatalf("%d messages stored", len(o.messages))
	}
	if first.Deliver == second.Deliver {
		t.Error("both or neither would be delivered")
	}
}

func TestTheRateIsCheckedAgainAfterTheTargetIsFound(t *testing.T) {
	o, _, _ := testOutbox(t)
	o.limits = func() mailLimits { return mailLimits{MaxBytes: 1000, PerHour: 1, WaitHours: 24} }
	req := toRev("one", "r1")
	req.Resolve = func() (mailDest, *mailError) {
		// While this one looks for its target, another send takes the hour's only place.
		other := toRev("two", "r2")
		if _, err := o.add(other); err != nil {
			t.Errorf("inner add: %v", err)
		}
		return mailDest{ID: "abcdef123456", Name: "Rev"}, nil
	}
	wantRefusal(t, o, req, 429)
	if len(o.messages) != 1 {
		t.Fatalf("%d messages stored", len(o.messages))
	}
}

func TestTheToIsTrimmedOfSpacesAndTabsOnlyAsTheMacDoes(t *testing.T) {
	o, _, _ := testOutbox(t)
	blank := toRev("x", "a1")
	blank.To = " \t "
	wantRefusal(t, o, blank, 400)
	// A newline is not trimmed from "to" (the Mac's .whitespace): it goes on to the target, whose 404 the Mac gives.
	newline := toRev("x", "a2")
	newline.To = "\n"
	newline.Resolve = func() (mailDest, *mailError) { return mailDest{}, &mailError{404, "no such hire"} }
	wantRefusal(t, o, newline, 404)
	// The text is trimmed of newlines too (.whitespacesAndNewlines).
	wantRefusal(t, o, toRev("\n \t", "a3"), 400)
}
