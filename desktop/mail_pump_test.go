package desktop

import (
	"strings"
	"testing"
	"time"
)

// pumpRig is the pump with a fake typer, a fake clock and a registry of hires given by the test.
type pumpRig struct {
	t     *testing.T
	o     *mailOutbox
	clock *mailClock
	pump  *mailPump
	hires []Hire
	typed []string // "<hire id> <text>", in the order tried
	bots  []string // ids of the messages handed to the bot step
	looks []time.Time
	// answer is what the fake typer comes to; commit says whether it counts the try first.
	answer func(h Hire, text string) mailResult
	commit bool
}

func newPumpRig(t *testing.T) *pumpRig {
	o, clock, _ := testOutbox(t)
	r := &pumpRig{t: t, o: o, clock: clock, commit: true}
	r.hires = []Hire{{ID: "abcdef123456", Name: "Rev", State: "working"}, {ID: "bbbbbb222222", Name: "Ann", State: "working"}}
	r.answer = func(Hire, string) mailResult { return mailResult{Outcome: mailSent, Typed: true} }
	r.pump = &mailPump{
		outbox: o, now: clock.now,
		hires: func() []Hire { return r.hires },
		bot:   func(m mailMessage) { r.bots = append(r.bots, m.ID) },
		look:  func(at time.Time) { r.looks = append(r.looks, at) },
		typeIn: func(h Hire, text string, commit func() bool) mailResult {
			r.typed = append(r.typed, h.ID+" "+text)
			if r.commit && !commit() {
				return mailResult{Outcome: mailTrouble, Note: "no save"}
			}
			return r.answer(h, text)
		},
	}
	return r
}

func (r *pumpRig) msg(id string) mailMessage {
	r.t.Helper()
	m, ok := r.o.get(id)
	if !ok {
		r.t.Fatalf("no message %s", id)
	}
	return m
}

func toHire(hireID, name, text, id string) mailRequest {
	return mailRequest{From: mailFromGM, To: name, Text: text, ID: id, IDGiven: true,
		Resolve: func() (mailDest, *mailError) { return mailDest{ID: hireID, Name: name}, nil }}
}

func TestThePumpTriesDueMessagesInOutboxOrderOnce(t *testing.T) {
	r := newPumpRig(t)
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "first", "m1"))
	mustAdd(t, r.o, toHire("bbbbbb222222", "Ann", "second", "m2"))
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "third", "m3"))
	r.pump.round()
	if len(r.typed) != 3 || !strings.HasSuffix(r.typed[0], "first") || !strings.HasSuffix(r.typed[1], "second") || !strings.HasSuffix(r.typed[2], "third") {
		t.Fatalf("typed %q", r.typed)
	}
	for _, id := range []string{"m1", "m2", "m3"} {
		if m := r.msg(id); m.State != mailDelivered || m.Attempts != 1 {
			t.Errorf("%s: %+v", id, m)
		}
	}
	// Delivered mail is never tried again.
	r.pump.round()
	if len(r.typed) != 3 {
		t.Errorf("a delivered message was tried again: %q", r.typed)
	}
	if len(r.looks) != 2 || !r.looks[0].Equal(r.clock.now()) {
		t.Errorf("the mailboxes were looked at %v", r.looks)
	}
}

func TestAMessageWithATryOutIsLeftAlone(t *testing.T) {
	r := newPumpRig(t)
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "busy", "m1"))
	out := r.o.start("m1") // a try is out
	if out == nil {
		t.Fatal("no attempt")
	}
	r.pump.round()
	if len(r.typed) != 0 {
		t.Errorf("a second try was started: %q", r.typed)
	}
	// Not even when its hire is gone: what the try comes to is recorded then.
	r.hires = nil
	r.pump.round()
	if m := r.msg("m1"); m.State != mailQueued {
		t.Errorf("failed under a try: %+v", m)
	}
	out.Finish(mailResult{Outcome: mailSent})
	if m := r.msg("m1"); m.State != mailDelivered {
		t.Errorf("after: %+v", m)
	}
}

func TestMailForAHireThatIsGoneFailsAtOnce(t *testing.T) {
	r := newPumpRig(t)
	r.hires = []Hire{{ID: "abcdef123456", Name: "Rev", State: "fired"}, {ID: "bbbbbb222222", Name: "Ann", State: "failed"}}
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "x", "m1"))
	mustAdd(t, r.o, toHire("bbbbbb222222", "Ann", "y", "m2"))
	mustAdd(t, r.o, toHire("cccccc333333", "Zed", "z", "m3")) // not in the registry at all
	r.pump.round()
	if len(r.typed) != 0 {
		t.Errorf("typed into a gone hire: %q", r.typed)
	}
	for id, who := range map[string]string{"m1": "Rev", "m2": "Ann", "m3": "Zed"} {
		if m := r.msg(id); m.State != mailFailed || m.Note != who+" is gone." || m.Attempts != 0 {
			t.Errorf("%s: %+v", id, m)
		}
	}
}

func TestWaitingIsNotAnAttemptAndIsLookedAtOnEveryRound(t *testing.T) {
	r := newPumpRig(t)
	r.commit = false // nothing was typed, so nothing was counted
	r.answer = func(Hire, string) mailResult { return mailResult{Outcome: mailWaiting, Note: "waiting: at work"} }
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "x", "m1"))
	for i := 0; i < 10; i++ {
		r.clock.advance(5 * time.Second)
		r.pump.round()
	}
	if len(r.typed) != 10 {
		t.Errorf("looked at %d times, want every round", len(r.typed))
	}
	if m := r.msg("m1"); m.State != mailQueued || m.Attempts != 0 || m.Note != "waiting: at work" {
		t.Errorf("after waiting: %+v", m)
	}
	// Past the wait it is left, with the Mac's words.
	r.clock.advance(24 * time.Hour)
	r.pump.round()
	if m := r.msg("m1"); m.State != mailLeft || !strings.HasPrefix(m.Note, "Left after waiting 24 hours for Rev to be free, not sent again: ") {
		t.Errorf("after the wait: %+v", m)
	}
}

func TestTroubleIsTriedAgainAfter30SecondsAndLeftAfterSixTries(t *testing.T) {
	r := newPumpRig(t)
	r.answer = func(Hire, string) mailResult {
		return mailResult{Outcome: mailTrouble, Note: "tmux couldn't be asked."}
	}
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "x", "m1"))
	r.pump.round()
	if m := r.msg("m1"); m.Attempts != 1 || m.State != mailQueued || m.NextAt != float64(r.clock.now().Add(30*time.Second).UnixMilli()) {
		t.Fatalf("after one try: %+v", m)
	}
	r.clock.advance(29 * time.Second)
	r.pump.round()
	if len(r.typed) != 1 {
		t.Fatalf("tried again after 29 s: %q", r.typed)
	}
	r.clock.advance(time.Second)
	r.pump.round()
	if len(r.typed) != 2 {
		t.Fatalf("not tried again after 30 s: %q", r.typed)
	}
	for i := 0; i < 10; i++ {
		r.clock.advance(30 * time.Second)
		r.pump.round()
	}
	if m := r.msg("m1"); m.State != mailLeft || m.Attempts != 6 || !strings.HasPrefix(m.Note, "Left after 6 tries, not sent again: ") {
		t.Errorf("after six: %+v", m)
	}
	if len(r.typed) != 6 {
		t.Errorf("tried %d times, want 6", len(r.typed))
	}
}

func TestAMessageForABotGoesToTheBotStepAndNeverTheWindow(t *testing.T) {
	r := newPumpRig(t)
	mustAdd(t, r.o, mailRequest{From: mailFromGM, To: "grok-1", Text: "hi", ID: "mb", IDGiven: true,
		Resolve: func() (mailDest, *mailError) { return mailDest{ID: mailBotPrefix + "grok-1", Name: "Grokky"}, nil }})
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "hello", "mh"))
	r.pump.round()
	if len(r.bots) != 1 || r.bots[0] != "mb" || len(r.typed) != 1 || !strings.HasPrefix(r.typed[0], "abcdef123456 ") {
		t.Errorf("bots %q typed %q", r.bots, r.typed)
	}
}

func TestTheGMsWindowIsNeverTypedInto(t *testing.T) {
	r := newPumpRig(t)
	r.hires = append(r.hires, Hire{ID: "dddddd444444", Name: "Boss", Role: gmRole, State: "working"})
	mustAdd(t, r.o, toHire("dddddd444444", "Boss", "to the gm", "mg"))
	r.pump.round()
	if len(r.typed) != 0 {
		t.Errorf("typed into the GM: %q", r.typed)
	}
	if m := r.msg("mg"); m.State != mailFailed || !strings.HasPrefix(m.Note, "Mail is never put into the GM's own session.") {
		t.Errorf("the GM's mail: %+v", m)
	}
}

func TestThePumpEndsATryThatPanics(t *testing.T) {
	r := newPumpRig(t)
	r.pump.typeIn = func(Hire, string, func() bool) mailResult { panic("tmux output") }
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "x", "m1"))
	r.pump.round()
	if m := r.msg("m1"); m.Attempts != 1 || m.State != mailQueued || !strings.Contains(m.Note, "Delivery stopped") {
		t.Errorf("after a panic: %+v", m)
	}
	if r.o.start("m1") == nil {
		t.Error("the message is still marked as out")
	}
}

func TestTheDeckPumpStopsAndMayBeStoppedTwice(t *testing.T) {
	d := newTestDeck(t)
	stop := d.startMailPump()
	done := make(chan struct{})
	go func() { stop(); stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the pump did not stop")
	}
}

// A hire at a question, as the hooks tell the app, is asking; one working elsewhere is not.
func TestTheDeckKnowsWhenAHiresSessionIsAsking(t *testing.T) {
	d := newTestDeck(t)
	folder := t.TempDir()
	other := t.TempDir()
	h := Hire{ID: "abcdef123456", Name: "Rev", Folder: folder}
	d.mu.Lock()
	d.sessions.byID["s1"] = &Session{ID: "s1", Folder: other, State: Asking}
	d.sessions.byID["s2"] = &Session{ID: "s2", Folder: folder, State: Thinking}
	d.mu.Unlock()
	if d.mailDeckAsking(h) {
		t.Error("asking, but not where the hire works, or not asking")
	}
	d.mu.Lock()
	d.sessions.byID["s2"].State = Asking
	d.mu.Unlock()
	if !d.mailDeckAsking(h) {
		t.Error("a session of the hire at a question was missed")
	}
}
