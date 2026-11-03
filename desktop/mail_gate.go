package desktop

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// The mail delivery gate: a port of the gate in the Mac's MailRouter and of
// MailDelivery's typing order (Sources/AgentDeck/MailRouter.swift,
// MailDelivery.swift, HireTmux.canPaste). A library only: no routes, no tmux,
// nothing calls it yet. The outbox (send-id, waiting is not an attempt) comes
// in the next change and wraps mailType; the tmux engine will implement
// mailTyper over `tmux -S <socket>`.
//
// Hard rule 2: mail never types into a window at a question, a menu, a draft
// or a busy session, and never into the GM's own window. It looks again just
// before typing. When in doubt, nothing is typed.

// mailVerdict is what the gate decides for one look at a window.
type mailVerdict int

const (
	// mailWait: not now; the message stays queued and the window is looked
	// at again later. Not an attempt.
	mailWait mailVerdict = iota
	// mailDeliver: the window is an idle, empty input box.
	mailDeliver
	// mailRefuse: never, whatever the window shows (the GM's own window).
	mailRefuse
)

// mailTarget is what the gate needs to know of where a message is going.
type mailTarget struct {
	// GM: the GM's own window. Mail is never typed into it.
	GM bool
	// Name is who it is for, in words.
	Name string
}

// mailGate decides from one pane capture. Only an idle, empty input box is
// typed into (ReadPaneState reads the window's structure, not its words);
// anything else, an unreadable or empty capture included, waits. deckAsking
// is whether the app itself knows the session is at a question, read just
// then: it keeps the message waiting even over an idle-looking box.
func mailGate(t mailTarget, capture string, deckAsking bool) (mailVerdict, string) {
	return mailGateState(t, ReadPaneState(capture), deckAsking)
}

// mailGateState is mailGate for a window already read: the Windows launcher
// sends the state of its screen, never its text.
func mailGateState(t mailTarget, state PaneState, deckAsking bool) (mailVerdict, string) {
	if t.GM {
		return mailRefuse, "Mail is never put into the GM's own session. The GM reads its mail with smolcorp gm inbox."
	}
	if deckAsking {
		return mailWait, "waiting: at a question"
	}
	switch state {
	case PaneIdle:
		return mailDeliver, ""
	case PaneMenu:
		return mailWait, "waiting: at a question"
	case PaneBusy:
		return mailWait, "waiting: at work"
	case PaneDraft:
		return mailWait, "waiting: a draft is in its box"
	}
	return mailWait, "waiting: its input box couldn't be told apart"
}

// mailTyper is the window mail goes into. The tmux engine will implement it
// (capture-pane, a bracketed paste, then Return); tests use a fake. Paste must
// be a bracketed paste, so the text lands in the box and is never read as
// keys. Copy mode and the one-line fallback (when a multi-line paste isn't
// safe) are the engine's job, not the gate's.
type mailTyper interface {
	// Capture is the window's screen as it is now.
	Capture() (string, error)
	// Paste puts text into the input box, without sending it.
	Paste(text string) error
	// Submit presses Return, the send of what Paste put there.
	Submit() error
}

// mailStateTyper is a typer that can only say how the window reads, not show it
// (the Windows launcher). mailType asks it in place of Capture.
type mailStateTyper interface {
	// State is how the window reads now; PaneUnknown on any doubt.
	State() (PaneState, error)
}

// mailWaitError is how a typer says "not now" from Capture without it being
// trouble: its window is in a mode of its own (tmux's copy mode), where a paste
// would go to the mode and not the agent. mailType makes it a wait.
type mailWaitError struct{ Why string }

func (e *mailWaitError) Error() string { return e.Why }

// mailNoter is a typer that can say how the message went in when it was not quite as written
// (the Mac's deliveredAs: new lines shown as /). mailType asks it after Return.
type mailNoter interface{ Note() string }

// mailOutcome is what one try at typing a message came to.
type mailOutcome int

const (
	// mailSent: pasted and sent with one Return.
	mailSent mailOutcome = iota
	// mailWaiting: the window wasn't ready. Nothing was typed; not an attempt.
	mailWaiting
	// mailTrouble: a failure that may pass (the window couldn't be looked at
	// or taken the paste). Nothing was sent.
	mailTrouble
	// mailCannot: it can never work (the GM's window, a control character),
	// or text was put in the box and not sent, which the owner clears by hand.
	mailCannot
)

// mailResult is what one try at delivery came to.
type mailResult struct {
	Outcome mailOutcome
	Note    string
	// Typed: something went into the window.
	Typed bool
}

// mailPastable is the Mac's HireTmux.canPaste: no control character (Unicode
// category Cc, so Escape, Return and U+0085 too) other than a line feed or tab.
// One deliberate difference: text that is not valid UTF-8 is refused too (a
// terminal can read the byte 0x9b as the 8-bit control sequence introducer;
// Swift strings can't hold such bytes, so the Mac never meets them).
func mailPastable(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if unicode.Is(unicode.Cc, r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

// mailType puts text into the window, in this order: look (the gate), count
// the try with commit, look again, type. The second look is the one just
// before typing: a screen that changed between the two looks blocks the typing
// and the message waits. commit is called once, before the first thing that
// types, and answers false when the try couldn't be written down; then nothing
// is typed. After the paste the box must hold the message as a draft, with no
// question showing, before Return.
func mailType(t mailTarget, text string, w mailTyper, deckAsking func() bool, commit func() bool) mailResult {
	if t.GM {
		_, why := mailGate(t, "", false)
		return mailResult{Outcome: mailCannot, Note: why}
	}
	if !mailPastable(text) {
		return mailResult{Outcome: mailCannot, Note: "The message holds a control character that can't be pasted safely."}
	}
	look := func() (bool, string, error) {
		if s, ok := w.(mailStateTyper); ok {
			state, err := s.State()
			if err != nil {
				return false, "", err
			}
			verdict, why := mailGateState(t, state, deckAsking())
			return verdict == mailDeliver, why, nil
		}
		capture, err := w.Capture()
		var wait *mailWaitError
		if errors.As(err, &wait) {
			return false, wait.Why, nil
		}
		if err != nil {
			return false, "", err
		}
		verdict, why := mailGate(t, capture, deckAsking())
		return verdict == mailDeliver, why, nil
	}
	trouble := func(why string) mailResult { return mailResult{Outcome: mailTrouble, Note: why} }
	if ok, why, err := look(); err != nil {
		return trouble("Its window couldn't be looked at.")
	} else if !ok {
		return mailResult{Outcome: mailWaiting, Note: why}
	}
	if !commit() {
		return trouble("The outbox couldn't be written, so nothing was typed.")
	}
	if ok, why, err := look(); err != nil {
		return trouble("Its window couldn't be looked at.")
	} else if !ok {
		return mailResult{Outcome: mailWaiting, Note: why}
	}
	if err := w.Paste(text); err != nil {
		return trouble("Its window didn't take the paste.")
	}
	stuck := "The text was put in its box but not sent, because %s. What is in the box has to be cleared by hand; later mail waits until then."
	var after PaneState
	var err error
	if s, ok := w.(mailStateTyper); ok {
		after, err = s.State()
	} else {
		var capture string
		if capture, err = w.Capture(); err == nil {
			after = ReadPaneState(capture)
		}
	}
	if err != nil {
		return mailResult{Outcome: mailCannot, Typed: true, Note: fmt.Sprintf(stuck, "its window couldn't be looked at")}
	}
	if after != PaneDraft || deckAsking() {
		return mailResult{Outcome: mailCannot, Typed: true, Note: fmt.Sprintf(stuck, "its window no longer looked like a box holding the message, or began waiting at a question")}
	}
	if err := w.Submit(); err != nil {
		return mailResult{Outcome: mailCannot, Typed: true, Note: "The text was put in its box but Return couldn't be pressed."}
	}
	sent := mailResult{Outcome: mailSent, Typed: true}
	if n, ok := w.(mailNoter); ok {
		sent.Note = n.Note()
	}
	return sent
}
