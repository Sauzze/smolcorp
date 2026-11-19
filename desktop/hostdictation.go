package desktop

import (
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The office's mic dictating under the Mac app (engine=go), as the Swift
// engine did it (Deck.swift, Dictation.swift): for an agent without a voice
// mode of its own, and for Claude Code too when setup says so or Claude's
// clean-up is on, holding the mic has the Mac record from its own microphone
// and turn what you say into text with Apple's speech engine; let go, and the
// text, cleaned up by Claude when that's on, is typed into the tab it was
// for, without Return. The Mac app records and cleans up, with its own code,
// through the bridge (dictation-start, dictation-stop); this program decides,
// types the text as the deck types, and keeps the voice notes: a few seconds'
// note over the desk of how its dictation went. On Windows and Linux there is
// no dictation: the mic presses the agent's own voice key, as before, and no
// desk has a voice note.

// dictationCouldntStart is the Swift engine's answer when recording doesn't start (Deck.perform's mic).
const dictationCouldntStart = "Dictation couldn't start. Look at it in setup, on your Mac."

// macNoKeyboard is the Mac app's no when a terminal's keys need Accessibility and it isn't given (Deck.noKeyboard).
const macNoKeyboard = "Keyboard access is off. Turn it on in setup, on your Mac."

// dictationLongest is the longest a recording runs (Dictation.longest): held longer, it stops and its text goes in.
var dictationLongest = 60 * time.Second

// voiceNoteLife is how long a voice note stays known (Deck.voiceNoteLife): the office fades it sooner.
var voiceNoteLife = 6 * time.Second

// dictationTextMost is the most text typed at once (HostBridge.textMost): past what a minute of speech makes.
const dictationTextMost = 4000

// voiceNote is how a desk's last dictation went, for a moment (Deck.voiceNotes).
type voiceNote struct {
	id      int
	text    string
	cleaned bool
	at      time.Time
}

// dictationOn says whether the office's mic can dictate: under the Mac app, turned on in setup, and the
// Mac able to (its speech engine, and the microphone allowed), as Dictation.isOn. Never in a test copy,
// whatever its settings say: it doesn't record from the Mac's microphone (RGAPA B1). With the lock held.
func (d *Deck) dictationOn() bool {
	return d.mac.host != nil && d.config.DictationEnabled && d.mac.canDictate && testCopyPort() == 0
}

// deskNames are the desks' project folders, the sessions' and the pinned ones', for the clean-up's
// glossary (Deck.insertDictation). With the lock held.
func (d *Deck) deskNames() []string {
	var names []string
	for _, s := range d.sessions.All() {
		names = append(names, s.ProjectName())
	}
	for _, pin := range d.config.Pins {
		if pin != nil {
			names = append(names, lastPathComponent(*pin))
		}
	}
	return names
}

// dictationText is what the app heard as it's typed: one line, no control character, and no longer than
// a tab takes at once.
func dictationText(heard string) string {
	text := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, heard))
	if utf8.RuneCountInString(text) > dictationTextMost {
		text = strings.TrimSpace(string([]rune(text)[:dictationTextMost]))
	}
	return text
}

// dictationAsking is the note over a desk whose dictation was dropped because it began asking you something
// while you spoke: its menu would take the words.
const dictationAsking = "It's asking you something, so what you said wasn't typed. Answer it first."

// tabAsking says whether an agent in tab waits on you, a permission prompt or a question. Called without the lock.
func (d *Deck) tabAsking(tab string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.sessions.All() {
		if s.Tab == tab && s.State == Asking {
			return true
		}
	}
	return false
}

// terminalNotAnswering says whether the Mac app's no is its sentence for a terminal that didn't answer in
// time, at all or part way (Terminals.notAnswering).
func terminalNotAnswering(why string) bool {
	return strings.HasSuffix(why, " isn't answering, so nothing was typed. Try again in a moment.") ||
		strings.HasSuffix(why, " stopped answering part way. Look at the desk before you try again.")
}

// noteVoice puts a few seconds' note over tab's desk, of how its dictation went, and forgets it after
// voiceNoteLife (Deck.noteVoice). Called without the lock.
func (d *Deck) noteVoice(tab, text string, cleaned bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var id string
	for _, s := range d.sessions.All() {
		if s.Tab == tab {
			id = s.ID
			break
		}
	}
	if id == "" {
		return
	}
	d.voiceNoteCount++
	count := d.voiceNoteCount
	d.voiceNotes[id] = voiceNote{id: count, text: text, cleaned: cleaned, at: time.Now()}
	d.changed()
	// Once, not polling: the note goes from memory; the office has faded it already.
	d.rounds.after(voiceNoteLife, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.voiceNotes[id].id == count {
			delete(d.voiceNotes, id)
		}
	})
}

// voiceNoteOf is session id's voice note for the snapshot, or nil. With the lock held.
func (d *Deck) voiceNoteOf(id string) any {
	note, ok := d.voiceNotes[id]
	if !ok {
		return nil
	}
	return map[string]any{"id": note.id, "text": note.text, "cleaned": note.cleaned, "at": note.at.UnixMilli()}
}

// setCleanup is the mic's other quick switch, Claude's clean-up on or off, for every phone: the setup page's
// checkbox is the same setting (Deck.perform's cleanup). Kept only under the Mac app; elsewhere there is no
// dictation to clean up, so a well-formed one only says so, as the language's switch.
func (d *Deck) setCleanup(on string) (int, string) {
	if on != "0" && on != "1" {
		return http.StatusBadRequest, "Say on=0 or on=1"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// Off while the records' key can't be read, it would be on again at the restart, the heard text sent to Claude
	// again (records_heldconfig.go, RT126SK1F B1).
	if refused := d.config.heldRefusal(func(c *Config) { c.DictationCleanup = on == "1" }); d.mac.host != nil && refused != "" {
		return http.StatusConflict, refused
	}
	if d.mac.host != nil {
		d.config.DictationCleanup = on == "1"
		d.config.save()
		d.changed()
		if on == "0" {
			d.cleanupOff()
		}
	}
	return http.StatusNoContent, ""
}

// cleanupOff is Claude's clean-up switched off, by the mic's switch or setup's checkbox: its last problem goes
// with it, as Deck.setCleanup (RGAPA N4), so switched on again setup doesn't show an old one. Not waited for.
// With the lock held.
func (d *Deck) cleanupOff() {
	if h := d.mac.host; h != nil {
		d.rounds.run(func() { _ = hostDictationCleanupOff(h) })
	}
}

// dictationClean is what the app is told as it cleans up what it heard: whether to, the glossary as setup has
// it (nil: the app's own), and the desks' names.
type dictationClean struct {
	cleanup  bool
	glossary *[]string
	desks    []string
}

// dictationCleaned is the text cleaned up, or as heard, and how that went for the desk's note ("" for none:
// the clean-up off), and whether it was cleaned.
type dictationCleaned struct {
	text, note string
	cleaned    bool
}

// hostDictationStart has the Mac app record for tab, in language: false when it couldn't start, as
// Dictation.start says it (already recording, or the microphone or the model not there). The app checks
// tab's terminal types text, and has Accessibility where its keys need it (macNoKeyboard). Busy, this
// program is recording already: the app checks the same, then says false without recording.
func hostDictationStart(h hostBridge, tab, language string, busy bool) (bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "dictation-start", Tab: tab, Language: language, Busy: busy})
	return answer.Started, err
}

// hostDictationStop has the Mac app stop recording and give what it heard, "" for nothing: at once, as
// Dictation.stop, or what a recording that stopped by itself left.
func hostDictationStop(h hostBridge) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "dictation-stop"})
	return answer.Text, err
}

// hostDictationCleanupOff has the Mac app forget why the clean-up last didn't work.
func hostDictationCleanupOff(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "dictation-cleanup-off"})
	return err
}

// hostDictationClean has the Mac app clean text up with Claude and your key when clean says so
// (DictationCleanup.clean: as heard if that fails or is slow), and say how it went.
func hostDictationClean(h hostBridge, text string, clean dictationClean) (dictationCleaned, error) {
	on := clean.cleanup
	answer, err := ask(h, bridgeRequest{Action: "dictation-clean", Text: text, On: &on, Glossary: clean.glossary, Desks: clean.desks})
	return dictationCleaned{text: answer.Text, note: answer.Note, cleaned: answer.Cleaned}, err
}
