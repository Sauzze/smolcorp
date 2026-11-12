package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Mail typed into a hire's window through its launcher (the Windows way; the Mac and Linux
// type through tmux, see mail_tmux.go). Beta. The launcher keeps a text screen of its agent
// (launcherscreen.go) and the app asks it how the window reads (launcherlook.go); mail goes in
// as a `paste` command, which writes only the text, and Return is the existing `press`.
// Same order as mailType: look, commit, look, paste, settle, look, Return. Hard rule 2: only an
// idle window is typed into, and any doubt (an unknown screen, a launcher that doesn't answer or
// that isn't the only one in the hire's folder) types nothing. Builds on every OS so it is tested
// on every OS; only the hand-over in mail_tmux_windows.go is Windows's.

const (
	// mailSettle is a moment between the paste and Return, so the agent has taken the paste in (MailDelivery.settle).
	mailSettle = 400 * time.Millisecond
)

// mailLaunchLookWait is how long a look at a launcher waits for its answer (a variable so a test can wait less).
var mailLaunchLookWait = 2 * time.Second

// mailOneLine is the lines of text joined with " / ", empty lines dropped, tabs made spaces
// (a Tab would arrive as a key) (MailDelivery.oneLine).
func mailOneLine(text string) string {
	var lines []string
	for _, line := range strings.FieldsFunc(strings.ReplaceAll(text, "\t", " "), func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == '\u0085' || r == ' ' || r == ' '
	}) {
		if line = strings.TrimFunc(line, func(r rune) bool { return r == ' ' || unicode.Is(unicode.Zs, r) }); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " / ")
}

// launcherMailPort is what the launcher typer needs of one launcher; the Deck gives it, tests fake it.
type launcherMailPort interface {
	// look is how the window reads and whether the agent has bracketed paste on.
	look(wait time.Duration) (PaneState, bool, error)
	// paste writes text into the box, no Return.
	paste(text string) error
	// enter presses Return.
	enter() error
}

// launcherMailWindow is one hire's launcher as a mailTyper (and mailStateTyper), for one try.
type launcherMailWindow struct {
	port  launcherMailPort
	pause func(time.Duration)
	// bracketed is as of the last look.
	bracketed bool
	note      string
}

// State looks at the launcher. A launcher that is gone or doesn't answer is an error (trouble);
// any other doubt comes back as PaneUnknown, which waits.
func (w *launcherMailWindow) State() (PaneState, error) {
	state, bracketed, err := w.port.look(mailLaunchLookWait)
	if err != nil {
		return PaneUnknown, err
	}
	w.bracketed = bracketed
	return state, nil
}

// Capture is not used: the launcher sends the window's state, never its text.
func (w *launcherMailWindow) Capture() (string, error) {
	return "", errors.New("a launcher's window can't be captured, only read")
}

// Paste puts the text in the box and waits mailSettle for the agent to take it in. Multi-line text
// goes as one line when the agent didn't ask for bracketed paste: a line feed would be read as Return.
func (w *launcherMailWindow) Paste(text string) error {
	body, note := text, ""
	if !w.bracketed {
		if strings.Contains(text, "\n") {
			body = mailOneLine(text)
			note = "Sent as one line (new lines shown as /): its window wasn't taking pastes as one piece."
		} else if strings.Contains(text, "\t") {
			body = mailOneLine(text)
		}
	}
	if !mailPastable(body) || body == "" {
		return errors.New("nothing pasteable")
	}
	if err := w.port.paste(body); err != nil {
		return err
	}
	w.note = note
	w.pause(mailSettle)
	return nil
}

// Submit presses Return.
func (w *launcherMailWindow) Submit() error { return w.port.enter() }

// Note is how the message went in when it was not quite as written (the Mac's deliveredAs).
func (w *launcherMailWindow) Note() string { return w.note }

// deckLauncherPort is one connected launcher, by its id, as a launcherMailPort.
type deckLauncherPort struct {
	d  *Deck
	id string
}

func (p deckLauncherPort) look(wait time.Duration) (PaneState, bool, error) {
	return p.d.lookAtLauncherPaste(p.id, wait)
}

func (p deckLauncherPort) tab() (*launcherTab, error) {
	p.d.mu.Lock()
	defer p.d.mu.Unlock()
	if tab, ok := p.d.launchers[p.id]; ok {
		return tab, nil
	}
	return nil, errLauncherGone
}

func (p deckLauncherPort) paste(text string) error {
	tab, err := p.tab()
	if err != nil {
		return err
	}
	return tab.Paste(text)
}

func (p deckLauncherPort) enter() error {
	tab, err := p.tab()
	if err != nil {
		return err
	}
	return tab.Press(press(Enter))
}

// hireLauncherID is the id of the one connected launcher that is the hire's window, and never
// the GM's. A hire the launcher engine made (ViaLauncher) is matched by its id exactly: its
// launcher carries the id from the hire's one-time token, so two hires in one folder are told
// apart, and without such a launcher there is none. Any other hire is matched as before: started
// in its worktree (or, with none, its folder), started by the office (with a hire token), and
// carrying no hire's id. A plain `smolcorp run` in the same folder is the owner's own window and
// is never matched. ok is false when there is none, more than one, or (by folder) another open
// hire works in the same place (its window could be the one that is open; in doubt nothing is typed).
func (d *Deck) hireLauncherID(h Hire) (id string, ok bool) {
	if h.Role == gmRole {
		return "", false
	}
	type candidate struct{ id, folder string }
	var all []candidate
	var byID []string
	d.mu.Lock()
	for id, tab := range d.launchers {
		switch {
		case tab.role == gmRole || !tab.office:
		case tab.hireID == h.ID && h.ID != "":
			byID = append(byID, id)
		case tab.hireID == "":
			all = append(all, candidate{id, tab.folder})
		}
	}
	d.mu.Unlock()
	if len(byID) > 0 || h.ViaLauncher {
		if len(byID) != 1 {
			return "", false
		}
		return byID[0], true
	}
	// An older hire's launcher has no hire id, only the folder it was started in: match the
	// hire's own place (its worktree if it has one, never also its folder), and give up when any
	// other open hire works there too, as its window would match just as well. Each by its own place: a
	// registry's folder is the project a hire was hired into, not where one with a worktree works.
	ownPlace := func(o Hire) Hire {
		if o.Worktree != "" {
			return Hire{Worktree: o.Worktree}
		}
		return Hire{Folder: o.Folder}
	}
	own := ownPlace(h)
	var others []Hire
	for _, o := range readRegistry(registryPath(), false).hires {
		if o.ID != h.ID && isOpen(o) {
			others = append(others, ownPlace(o))
		}
	}
	// Compared outside the lock: it resolves links on disk.
	var found []string
	for _, c := range all {
		at := liveSession{Folder: c.folder}
		if !works(at, own) {
			continue
		}
		for _, o := range others {
			if works(at, o) {
				return "", false
			}
		}
		found = append(found, c.id)
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// mailTypeIntoLauncher types text into a hire's window through its launcher, in mailType's order
// (the caller has made sure it isn't the GM). No launcher connected is trouble, as tmux's session
// that isn't running.
func (d *Deck) mailTypeIntoLauncher(h Hire, text string, commit func() bool, pause func(time.Duration)) mailResult {
	name := mailHireName(h)
	id, ok := d.hireLauncherID(h)
	if !ok {
		return mailResult{Outcome: mailTrouble, Note: fmt.Sprint(name, "'s window isn't open through Smolcorp's launcher.")}
	}
	return mailTypeViaLauncher(deckLauncherPort{d: d, id: id}, name, text, func() bool { return d.mailDeckAsking(h) }, commit, pause)
}

// mailTypeViaLauncher is the order of looking and typing over one launcher.
func mailTypeViaLauncher(port launcherMailPort, name, text string, deckAsking func() bool, commit func() bool, pause func(time.Duration)) mailResult {
	return mailType(mailTarget{Name: name}, text, &launcherMailWindow{port: port, pause: pause}, deckAsking, commit)
}
