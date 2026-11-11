package desktop

import (
	"errors"
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

// dictationRun is a recording the Mac app is making for the office: the tab it's for, its agent, and the
// phone that holds the mic. Each is its own, so a cap or a phone gone ends this one and never the next.
type dictationRun struct {
	tab   string
	agent Agent
	phone string
	// Being stopped: the app has been asked, and hasn't answered yet. With the deck's lock.
	stopping bool
}

// voiceNote is how a desk's last dictation went, for a moment (Deck.voiceNotes).
type voiceNote struct {
	id      int
	text    string
	cleaned bool
	at      time.Time
}

// dictationOn says whether the office's mic can dictate: under the Mac app, turned on in setup, and the
// Mac able to (its speech engine, and the microphone allowed), as Dictation.isOn. With the lock held.
func (d *Deck) dictationOn() bool {
	return d.mac.host != nil && d.config.DictationEnabled && d.mac.canDictate
}

// dictatesFor says whether the mic dictates for agent rather than pressing its own voice key: for every
// agent without one, and for Claude Code too if setup says so, or while Claude's clean-up is on
// (Deck.dictates). With the lock held.
func (d *Deck) dictatesFor(agent Agent) bool {
	return d.dictationOn() && (!agent.HasVoice() || d.config.DictationForClaude || d.config.DictationCleanup)
}

// startDictation is the mic held on a desk whose agent it dictates for: the Mac app records, for tab. Only
// where its terminal types text, as the Swift engine's; one recording at a time. Called without the lock.
func (d *Deck) startDictation(tab string, agent Agent, phone string) (int, string) {
	d.mu.Lock()
	h, language := d.mac.host, d.dictationLanguage()
	can := d.mac.abilities[tab]
	if kind, _, _ := splitTab(tab); kind == "launcher" {
		can.Text = true
	}
	if !can.Text {
		d.mu.Unlock()
		return http.StatusConflict, notHere
	}
	// Already recording, or getting ready to: Dictation.start's no.
	if d.dictating != nil {
		d.mu.Unlock()
		return http.StatusConflict, dictationCouldntStart
	}
	run := &dictationRun{tab: tab, agent: agent, phone: phone}
	d.dictating = run
	d.mu.Unlock()

	started, err := hostDictationStart(h, tab, language)
	if err == nil && started {
		// Held longer than a recording runs: it stops, and its text goes in.
		d.rounds.after(dictationLongest, func() { d.endDictation(run) })
		return http.StatusNoContent, ""
	}
	d.mu.Lock()
	if d.dictating == run {
		d.dictating = nil
	}
	d.mu.Unlock()
	var refusal appRefusal
	switch {
	case errors.Is(err, errUnsupported):
		return http.StatusConflict, notHere
	case errors.As(err, &refusal):
		// The app's no, before anything started: Accessibility, as the deck's keys; or its own sentence.
		if refusal.Error() == macNoKeyboard {
			return http.StatusForbidden, macNoKeyboard
		}
		return http.StatusConflict, dictationCouldntStart
	case err != nil:
		// Not answered in time, it may still come up: it's stopped, and what it heard is dropped.
		d.rounds.run(func() { _, _ = hostDictationStop(h) })
	}
	return http.StatusConflict, dictationCouldntStart
}

// releaseDictation is the mic let go while recording, or getting ready to: the text goes into the tab it
// was for, without Return, once the app has it; the phone isn't kept waiting. False when nothing is
// recording. Called without the lock.
func (d *Deck) releaseDictation() bool {
	d.mu.Lock()
	run := d.dictating
	d.mu.Unlock()
	if run == nil {
		return false
	}
	d.rounds.run(func() { d.endDictation(run) })
	return true
}

// dictationPhoneLeft is a phone's last event stream gone, asleep or off the Wi-Fi: the recording it
// started stops, and what it said goes in (Dictation.phoneLeft). With the lock held.
func (d *Deck) dictationPhoneLeft(phone string) {
	if run := d.dictating; run != nil && run.phone == phone && !d.isConnected(phone) {
		d.rounds.run(func() { d.endDictation(run) })
	}
}

// endDictation stops run, if it's still the one recording, and types what the app heard, cleaned up as
// setup says, with its note over the desk (Deck.insertDictation). The recording is the deck's until the app
// has stopped it, so a new one never starts before; the clean-up comes after, as the next one may record.
// Called without the lock.
func (d *Deck) endDictation(run *dictationRun) {
	d.mu.Lock()
	if d.dictating != run || run.stopping {
		d.mu.Unlock()
		return
	}
	run.stopping = true
	h := d.mac.host
	d.mu.Unlock()
	heard, err := hostDictationStop(h)
	d.mu.Lock()
	if d.dictating == run {
		d.dictating = nil
	}
	clean := dictationClean{cleanup: d.config.DictationCleanup, glossary: d.config.DictationGlossary, desks: d.deskNames()}
	d.mu.Unlock()
	text := dictationText(heard)
	if err != nil || text == "" {
		return
	}
	// Whatever goes wrong with the clean-up, the text goes in as heard: what was said is never lost.
	if cleaned, err := hostDictationClean(h, text, clean); err == nil && dictationText(cleaned.text) != "" {
		text = dictationText(cleaned.text)
		if cleaned.note != "" {
			d.noteVoice(run.tab, cleaned.note, cleaned.cleaned)
		}
	}
	d.typeDictation(run, text)
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

// typeDictation types text into run's tab, without Return, only while its agent is in front there, as the
// deck's keys go (Deck.typeDictation). A terminal that didn't take it says so over the desk. Called without the lock.
func (d *Deck) typeDictation(run *dictationRun, text string) {
	var failed error
	_, _ = d.toAgent(run.tab, run.agent, func(t Tab) error {
		switch tab := t.(type) {
		case macTab:
			if tab.pane == nil {
				answer, err := ask(tab.host, bridgeRequest{Action: "insert", Tab: tab.name, Agent: run.agent, Text: text})
				if answer.Place == "" {
					failed = err
				}
				return placeError(answer, err)
			}
		}
		result, err := d.phoneText(run.tab, t, run.agent, text, false, func() SessionState { return "" })
		if err == nil && result != pasteDone {
			err = errPasteFailed
		}
		failed = err
		return err
	})
	// The Mac app's sentence for a terminal that stopped answering, said over the desk; an agent that
	// has ended or isn't in front is left as the deck leaves it.
	var refusal appRefusal
	if errors.As(failed, &refusal) {
		d.noteVoice(run.tab, refusal.Error(), false)
	}
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
	if d.mac.host != nil {
		d.config.DictationCleanup = on == "1"
		d.config.save()
		d.changed()
	}
	return http.StatusNoContent, ""
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
// tab's terminal types text, and has Accessibility where its keys need it (macNoKeyboard).
func hostDictationStart(h hostBridge, tab, language string) (bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "dictation-start", Tab: tab, Language: language})
	return answer.Started, err
}

// hostDictationStop has the Mac app stop recording and give what it heard, "" for nothing: at once, as
// Dictation.stop, or what a recording that stopped by itself left.
func hostDictationStop(h hostBridge) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "dictation-stop"})
	return answer.Text, err
}

// hostDictationClean has the Mac app clean text up with Claude and your key when clean says so
// (DictationCleanup.clean: as heard if that fails or is slow), and say how it went.
func hostDictationClean(h hostBridge, text string, clean dictationClean) (dictationCleaned, error) {
	on := clean.cleanup
	answer, err := ask(h, bridgeRequest{Action: "dictation-clean", Text: text, On: &on, Glossary: clean.glossary, Desks: clean.desks})
	return dictationCleaned{text: answer.Text, note: answer.Note, cleaned: answer.Cleaned}, err
}
