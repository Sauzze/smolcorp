//go:build !windows

package desktop

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Mail typed into a hire's tmux window (MailDelivery.deliver step 2, MailDelivery.swift ~23-114, and
// HireTmux.paste, pasteState and submit, HireTmux.swift ~103-127), without Claude Code's inbox socket
// (that is for 1.24.x). tmuxMailWindow is the mailTyper over hireTmux; mailType does the order of looking,
// counting the try and typing.

const (
	// mailCaptureLines is how many lines of the window are looked at (MailDelivery.captureLines).
	mailCaptureLines = 40
	// mailPasteBuffer is the tmux buffer a paste goes through (HireTmux.buffer).
	mailPasteBuffer = "smolcorp-paste"
)

// pasteMode is how a pane takes a paste.
type pasteMode struct {
	// inMode: the pane is in a mode of tmux's own (copy mode, scrolled back): a paste would go to the mode.
	inMode bool
	// bracketed: the agent asked for bracketed paste.
	bracketed bool
}

// pasteState says how the pane takes a paste; ok is false when tmux couldn't say (HireTmux.pasteState).
// A tmux before 3.7 (Ubuntu 24.04 has 3.4) has no bracket_paste_flag and leaves it empty: the pane is then
// taken as not asking for bracketed paste, the safe way, so a multi-line message goes as one line.
func (h *hireTmux) pasteState() (pasteMode, bool) {
	out, status, ran := h.run(h.command("display-message", "-p", "-t", hirePane, "#{pane_in_mode} #{bracket_paste_flag}"))
	if !ran || status != 0 {
		return pasteMode{}, false
	}
	parts := strings.Split(out, " ")
	if len(parts) != 2 || parts[0] == "" {
		return pasteMode{}, false
	}
	for _, part := range parts {
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return pasteMode{}, false
			}
		}
	}
	return pasteMode{inMode: parts[0] != "0", bracketed: parts[1] == "1"}, true
}

// paste puts text into the pane as one bracketed paste, never followed by Return. -r keeps tmux from
// turning line feeds into Return. A control character other than a line feed or tab is refused
// (Escape could end the bracket early).
func (h *hireTmux) paste(text string) bool {
	if !mailPastable(text) {
		return false
	}
	if !h.ok("set-buffer", "-b", mailPasteBuffer, "--", tmuxLiteral(text)) {
		return false
	}
	if h.ok("paste-buffer", "-p", "-r", "-d", "-b", mailPasteBuffer, "-t", hirePane) {
		return true
	}
	h.ok("delete-buffer", "-b", mailPasteBuffer) // -d only deletes after a paste
	return false
}

// submit presses Return, once: the send of a message that was pasted into an empty box, never an answer.
func (h *hireTmux) submit() bool { return h.ok("send-keys", "-t", hirePane, "Enter") }

// tmuxMailWindow is one hire's window as a mailTyper, for one try.
type tmuxMailWindow struct {
	tmux  *hireTmux
	pause func(time.Duration)
	note  string
}

// Capture is the last mailCaptureLines lines of the window. A window in copy mode, or one tmux can't say
// how it takes a paste, is not looked at as idle: the first waits, the second is trouble.
func (w *tmuxMailWindow) Capture() (string, error) {
	screen, ok := w.tmux.capture(mailCaptureLines)
	if !ok {
		return "", fmt.Errorf("tmux couldn't capture the window")
	}
	mode, ok := w.tmux.pasteState()
	if !ok {
		return "", fmt.Errorf("couldn't tell how its window takes a paste")
	}
	if mode.inMode {
		return "", &mailWaitError{Why: "waiting: its window is scrolled back or in copy mode"}
	}
	return screen, nil
}

// Paste puts the text in the box and waits mailSettle for the agent to take it in. Multi-line text goes
// as one line when the agent didn't ask for bracketed paste: a line feed would be read as Return.
func (w *tmuxMailWindow) Paste(text string) error {
	mode, ok := w.tmux.pasteState()
	if !ok || mode.inMode {
		return fmt.Errorf("its window isn't taking a paste now")
	}
	body, note := text, ""
	if strings.Contains(text, "\n") && !mode.bracketed {
		body = mailOneLine(text)
		note = "Sent as one line (new lines shown as /): its window wasn't taking pastes as one piece."
	}
	if !w.tmux.paste(body) {
		return fmt.Errorf("tmux didn't take the paste")
	}
	w.note = note
	w.pause(mailSettle)
	return nil
}

// Submit presses Return.
func (w *tmuxMailWindow) Submit() error {
	if !w.tmux.submit() {
		return fmt.Errorf("Return couldn't be pressed")
	}
	return nil
}

// Note is how the message went in when it was not quite as written (the Mac's deliveredAs).
func (w *tmuxMailWindow) Note() string { return w.note }

// mailHireTmux is the tmux server a hire's window is on, or nil: only the hire's own, exactly the
// socket the engine recorded when it made the hire (hireOwnTmux: its id's and this copy's port's name in
// the support folder, a real socket, not a link). A hire with no recorded socket, one recorded elsewhere or
// for another id or port, is never typed into, so the user's own tmux and another hire's are never reached.
func mailHireTmux(h Hire) *hireTmux {
	return hireOwnTmux(h, testCopyPort(), nil)
}

// mailTypeInto types text into a hire's window on server, in mailType's order (the caller has made
// sure it isn't the GM). A tmux that can't be asked, or a session that isn't running, is trouble.
func mailTypeInto(server *hireTmux, name, text string, deckAsking func() bool, commit func() bool, pause func(time.Duration)) mailResult {
	up, known := server.alive()
	if !known {
		return mailResult{Outcome: mailTrouble, Note: "tmux couldn't be asked."}
	}
	if !up {
		return mailResult{Outcome: mailTrouble, Note: name + "'s tmux session isn't running."}
	}
	return mailType(mailTarget{Name: name}, text, &tmuxMailWindow{tmux: server, pause: pause}, deckAsking, commit)
}

// mailTypeIntoHire is the pump's way of typing a hire's message: on that hire's own server, never the
// GM's window. A hire not in tmux here can't be reached.
func (d *Deck) mailTypeIntoHire(h Hire, text string, commit func() bool) mailResult {
	server := mailHireTmux(h)
	name := mailHireName(h)
	if server == nil && h.State == "starting" {
		// Its row is written before its tmux server is started: the message waits.
		return mailResult{Outcome: mailWaiting, Note: name + " is still starting."}
	}
	if server == nil {
		return mailResult{Outcome: mailCannot, Note: name + " doesn't run in tmux here, so mail can't reach it."}
	}
	return mailTypeInto(server, name, text, func() bool { return d.mailDeckAsking(h) }, commit, time.Sleep)
}
