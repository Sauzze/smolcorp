package desktop

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// The ACP Tab (ACP2): how the deck's words and controls reach an ACP desk (acpdesk.go), which has no terminal. It is
// the desk's Tab, found by its acp: name (Deck.tab), so every way in that takes a desk's tab reaches it through the
// checks it already makes; what reaches the agent goes over the protocol, never as keys:
//
//   - Words go only through typeText's rules (the phone's /dictate, the window's /talk/say): its phoneText puts them
//     in the desk's draft, as a terminal's input line, and the Return typeText presses after them (or the phone's ⏎,
//     with the same Return checks /act makes) sends the draft as session/prompt. Type, which the core's own lines
//     use (a made button's text, /effort, a new conversation's /clear), types nothing here.
//   - Nothing is typed or sent while the agent asks something, or while a turn is out: the desk refuses, and the
//     deck's own Asking checks come first. An ask only comes while a turn is out, so a prompt can never answer one.
//   - Esc and Reject are session/cancel, which also ends any ask as "cancelled" (the client's Cancel), never a pick.
//     New conversation is a new session/new on the same agent, with a new transcript, while the desk is free.
//   - No other key does anything, and there's no screen to read, focus or scroll: errUnsupported.

// acpTab is an ACP desk's Tab. Made afresh by each Deck.tab: what lasts is the desk's.
type acpTab struct{ desk *acpDesk }

var _ Tab = acpTab{}

// acpRefusal is an ACP desk's no, in the owner's words: a 409 with them, wherever the desk is reached from.
type acpRefusal string

func (r acpRefusal) Error() string { return string(r) }

const (
	// acpDraftMost is the most the draft holds, in bytes: several of the phone's longest texts.
	acpDraftMost = 64 << 10
	// acpCallLimit is how long a control may wait on the agent's client: a cancel sent, a new session answered.
	acpCallLimit = 30 * time.Second
)

var errACPDraftFull = errors.New("the ACP desk's draft is full")

// acpTab is the Tab of the desk held under name, or nil. With the lock held.
func (d *Deck) acpTab(name string) Tab {
	if desk := d.acpDesks[name]; desk != nil {
		return acpTab{desk: desk}
	}
	return nil
}

// holdACPDesk puts desk, its session open, where the deck finds its Tab: by the desk's own tab name. With the lock held.
func (d *Deck) holdACPDesk(desk *acpDesk) { d.acpDesks[desk.tab] = desk }

// dropACPDesk lets go of desk, if it is still the one held under its name. Called without the lock.
func (d *Deck) dropACPDesk(desk *acpDesk) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.acpDesks[desk.tab] == desk {
		delete(d.acpDesks, desk.tab)
	}
}

// Press takes Return alone: it sends the draft, the words typeText put there. Esc and Reject aren't keys here
// (Deck.acpAct), and no other key means anything to an agent with no terminal.
func (t acpTab) Press(k Keystroke) error {
	if k.Key != Enter {
		return errUnsupported
	}
	return t.desk.refusal(t.desk.sendDraft())
}

// Type types nothing: a prompt comes only from words that passed typeText's rules (see above).
func (t acpTab) Type(string) error { return errUnsupported }

func (t acpTab) Focus() error          { return errUnsupported }
func (t acpTab) Scroll(int) error      { return errUnsupported }
func (t acpTab) ScrollToBottom() error { return errUnsupported }

// Close ends the desk's agent and its session, and lets go of the desk: the office's firing. How the agent's own
// children are stopped is its client's (acp.Client.Close).
func (t acpTab) Close(Agent) error {
	t.desk.close()
	t.desk.deck.dropACPDesk(t.desk)
	return nil
}

// phoneText puts words in the draft, as a terminal takes them into its input line without Return: several lines as
// one, as a prompt takes them. On the key worker, after typeText's checks.
func (t acpTab) phoneText(words string) (pasteResult, error) {
	if err := t.desk.refusal(t.desk.typeDraft(words)); err != nil {
		return pasteFailed, err
	}
	return pasteDone, nil
}

// typeDraft adds words to the draft, while a prompt could go: never while the agent asks or a turn is out, where
// the Return after them couldn't send them.
func (desk *acpDesk) typeDraft(words string) error {
	desk.mu.Lock()
	defer desk.mu.Unlock()
	if err := desk.mayPrompt(); err != nil {
		return err
	}
	if len(desk.draft)+len(words) > acpDraftMost {
		return errACPDraftFull
	}
	desk.draft += words
	return nil
}

// sendDraft sends the draft as a turn, and empties it; with nothing in it, nothing goes. Refused, as typeDraft is,
// while the agent asks or a turn is out, so a Return never answers an ask; the draft is kept then. The turn runs on
// its own: its end comes as the desk's events.
func (desk *acpDesk) sendDraft() error {
	desk.mu.Lock()
	if err := desk.mayPrompt(); err != nil || desk.draft == "" {
		desk.mu.Unlock()
		return err
	}
	text := desk.draft
	session, err := desk.begin(text)
	if err != nil {
		desk.mu.Unlock()
		return err
	}
	desk.draft = ""
	desk.mu.Unlock()
	go func() { _, _ = desk.run(context.Background(), session, text) }()
	return nil
}

// interrupt is Esc and Reject: a turn out is cancelled at the agent (session/cancel), which ends any ask of the
// turn as "cancelled"; with no turn out, the draft goes, as Esc clears a terminal's line.
func (desk *acpDesk) interrupt(ctx context.Context) error {
	desk.mu.Lock()
	if desk.session == "" || desk.ended {
		desk.mu.Unlock()
		return errACPNotOpen
	}
	if !desk.turn {
		desk.draft = ""
		desk.mu.Unlock()
		return nil
	}
	desk.mu.Unlock()
	return desk.cancel(ctx)
}

// renew is New conversation: a new session/new on the same agent, in the desk's folder, with a transcript of its
// own; the deck's session stays the desk's, and starts again idle. Only while the desk is free; the draft goes.
func (desk *acpDesk) renew(ctx context.Context) error {
	desk.mu.Lock()
	if err := desk.mayPrompt(); err != nil {
		desk.mu.Unlock()
		return err
	}
	desk.renewing = true
	desk.mu.Unlock()
	session, err := desk.conn.NewSession(ctx, desk.folder)
	var transcript *acpTranscript
	if err == nil {
		transcript, err = newACPTranscript(supportDir, session)
	}
	desk.mu.Lock()
	defer desk.mu.Unlock()
	desk.renewing = false
	switch {
	case err != nil:
		return err
	case desk.ended:
		transcript.close(time.Now())
		return errACPNotOpen
	}
	desk.session, desk.draft = session, ""
	desk.push(acpItem{kind: "session/new", opened: transcript})
	return nil
}

// refusal is err as the owner is told it: the desk's no in words, an ended desk as an agent gone. Any other error
// is the client's, as it came.
func (desk *acpDesk) refusal(err error) error {
	name := desk.agent.Name()
	switch {
	case errors.Is(err, errACPNotOpen):
		return errAgentGone
	case errors.Is(err, errACPAsking):
		return acpRefusal(name + " is asking you something. Answer it first.")
	case errors.Is(err, errACPBusy):
		return acpRefusal(name + " is still working. Send this once it's done, or stop it with Esc.")
	case errors.Is(err, errACPDraftFull):
		return acpRefusal(tooLong)
	}
	return err
}

// acpAct answers the /act actions an ACP desk is told through its protocol rather than by keys: Esc and Reject
// (session/cancel) and New conversation (session/new). ok is false for any other action, which goes on as for every
// desk, to its Tab. Called without the lock, past PerformFrom's checks (the session, a test copy's tabs).
func (d *Deck) acpAct(action, name string, agent Agent, busy bool) (status int, message string, ok bool) {
	var op func(*acpDesk, context.Context) error
	switch action {
	case "reject", "esc":
		op = (*acpDesk).interrupt
	case "clear":
		if busy {
			return http.StatusConflict, "Busy — clear it when " + agent.Name() + " is done", true
		}
		op = (*acpDesk).renew
	default:
		return 0, "", false
	}
	d.mu.Lock()
	desk := d.acpDesks[name]
	d.mu.Unlock()
	if desk == nil {
		return http.StatusNotFound, "Tab not found", true
	}
	ctx, cancel := context.WithTimeout(context.Background(), acpCallLimit)
	defer cancel()
	var refusal acpRefusal
	switch err := desk.refusal(op(desk, ctx)); {
	case err == nil:
	case errors.Is(err, errAgentGone):
		return http.StatusGone, "That agent has ended.", true
	case errors.As(err, &refusal):
		return http.StatusConflict, string(refusal), true
	default:
		return http.StatusBadGateway, agent.Name() + " didn't answer. Try again in a moment.", true
	}
	d.mu.Lock()
	if d.sessions.MarkSeen(name) {
		d.changed()
	}
	d.mu.Unlock()
	return http.StatusNoContent, "", true
}
