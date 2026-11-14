package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
)

// One dictation service (1.26, FD1). A recording has a source, where the words come from, and a target, where
// they go.
//
// The source today is the Mac's speech engine through the bridge (hostdictation.go): it records from the Mac's own
// microphone and gives back what it heard when it stops. Whatever else hears words later (the phone's Web Speech,
// another backend) reports them through dictationPartial and the stop's text, and the targets below don't change.
//
// The target is one of two:
//   - dictateToTab: a desk's terminal tab directly, without Return: what the office's mic has always done on the Mac
//     (the phone's mic, the office's mic, the cap, a phone gone), kept exactly as it was.
//   - dictateToBox: a desk's input box in the office's own window (POST /talk/listen). The words wait there for
//     Send, which is POST /talk/say (typeText): nothing goes into the tab before, so a launcher's tab, which the
//     tab target leaves out, may have one. Its partial words and its final text go to the window's event stream
//     alone, as a note, never in the state, so never to a phone, the viewer, a push or the events.
//
// There is one microphone: one recording at a time, whatever its target. Where the Mac app isn't there (Linux,
// Windows) there is no desktop microphone, and the route says so. The GM as a target comes later (11).

// dictationTarget is where a recording's words go.
type dictationTarget int

const (
	// dictateToTab is a desk's tab, typed without Return: today's behaviour.
	dictateToTab dictationTarget = iota
	// dictateToBox is a desk's input box in the window: the words wait for Send.
	dictateToBox
)

// dictationRun is a recording the Mac app is making for the office: where its words go, the tab it's for, its agent,
// and the phone that holds the mic (the office's window for its own). Each is its own, so a cap or a phone gone
// ends this one and never the next.
type dictationRun struct {
	target dictationTarget
	// The desk's input box in the window it's for (dictateToBox), by session; "" for a tab.
	session string
	tab     string
	agent   Agent
	phone   string
	// Starting: the app has been asked to record, and hasn't answered yet. Being stopped: let go, and the app
	// asked to stop, or to be once it has answered the start. With the deck's lock.
	starting, stopping bool
}

// The window's notes of a dictation, for the desk's box: what phase it's in, and the words.
const (
	// listeningNote: the app is recording for the box.
	listeningNote = "listening"
	// partialNote: what has been heard so far, which the next one replaces.
	partialNote = "partial"
	// finalNote: what was heard in the end, "" when nothing was or it didn't work: the box stops listening.
	finalNote = "final"
)

// dictationNote is what the window's event stream says of a dictation: the event "dictation".
type dictationNote struct {
	Session string `json:"session"`
	Phase   string `json:"phase"`
	Text    string `json:"text,omitempty"`
}

// dictatesFor says whether the mic dictates for agent, in tab, rather than pressing its own voice key: for
// every agent without one, and for Claude Code too if setup says so, or while Claude's clean-up is on
// (Deck.dictates). Never in a launcher's window: the Swift engine had none, and the Mac app only records for
// a terminal's tab it knows (RGAPA B2); there the mic is the agent's voice key, as before. With the lock held.
// (A launcher's desk may still dictate into its input box in the window: dictateToBox.)
func (d *Deck) dictatesFor(tab string, agent Agent) bool {
	if kind, _, _ := splitTab(tab); kind == "launcher" {
		return false
	}
	return d.dictationOn() && (!agent.HasVoice() || d.config.DictationForClaude || d.config.DictationCleanup)
}

// startDictation is the mic held on a desk whose agent it dictates for: the Mac app records, for tab. Only
// where its terminal types text, as the Swift engine's; one recording at a time. Called without the lock.
func (d *Deck) startDictation(tab string, agent Agent, phone string) (int, string) {
	return d.beginDictation(&dictationRun{target: dictateToTab, tab: tab, agent: agent, phone: phone, starting: true})
}

// beginDictation has the Mac app record for run, whatever its target. Only the tab's target needs its terminal to
// type text now; the box's words are typed, if they are, when Send is pressed, and checked then. Called without
// the lock.
func (d *Deck) beginDictation(run *dictationRun) (int, string) {
	tab := run.tab
	d.mu.Lock()
	h, language := d.mac.host, d.dictationLanguage()
	if run.target == dictateToTab && !d.mac.abilities[tab].Text {
		d.mu.Unlock()
		return http.StatusConflict, notHere
	}
	// Already recording, or getting ready to: the app still checks the terminal and Accessibility first, and
	// then says no without recording, so the refusals come in the Swift engine's order (RGAPA N2).
	busy := d.dictating != nil
	if !busy {
		d.dictating = run
	}
	d.mu.Unlock()

	started, err := hostDictationStart(h, tab, language, busy)
	if busy {
		started = false
	} else {
		d.mu.Lock()
		run.starting = false
		released := run.stopping
		if !started && d.dictating == run {
			d.dictating = nil
		}
		d.mu.Unlock()
		if err == nil && started {
			if run.target == dictateToBox {
				d.tellWindow(dictationNote{Session: run.session, Phase: listeningNote})
			}
			if released {
				// Let go while it was starting (RGAPA N1): stopped now the app has begun, never before, so the
				// stop can't reach it first and leave the Mac recording with nothing here holding it.
				d.rounds.run(func() { d.stopDictation(run) })
			} else {
				// Held longer than a recording runs: it stops, and its text goes in.
				d.rounds.after(dictationLongest, func() { d.endDictation(run) })
			}
			return http.StatusNoContent, ""
		}
	}
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
	case err != nil && !busy:
		// Not answered in time, it may still come up: it's stopped, and what it heard is dropped.
		d.rounds.run(func() { _, _ = hostDictationStop(h) })
	}
	return http.StatusConflict, dictationCouldntStart
}

// releaseDictation is the mic let go while recording, or getting ready to: the text goes into the tab it
// was for, without Return, once the app has it; the phone isn't kept waiting. False when nothing is
// recording. A recording for the window's box isn't this one to end: the mic of a phone or the deck says
// nothing of it, and the window's own stop does (stopListening). Called without the lock.
func (d *Deck) releaseDictation() bool {
	d.mu.Lock()
	run := d.dictating
	d.mu.Unlock()
	if run == nil || run.target != dictateToTab {
		return false
	}
	d.rounds.run(func() { d.endDictation(run) })
	return true
}

// dictationPhoneLeft is a phone's last event stream gone, asleep or off the Wi-Fi: the recording it
// started stops, and what it said goes in (Dictation.phoneLeft). The office's window is one of these: its box's
// recording stops when its last stream goes, and nobody is told. With the lock held.
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
	// Still starting: the start, once answered, stops it (RGAPA N1).
	starting := run.starting
	d.mu.Unlock()
	if !starting {
		d.stopDictation(run)
	}
}

// stopDictation has the app stop run, which is being stopped, and types what it heard, as endDictation says: into
// the tab, or, for the window's box, to the window to wait for Send. Called without the lock.
func (d *Deck) stopDictation(run *dictationRun) {
	d.mu.Lock()
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
	if run.target == dictateToBox {
		// What was said, as heard: the clean-up for the box is FD2's. Said even when nothing was, so the box
		// stops listening; the words are never typed anywhere from here.
		if err != nil {
			text = ""
		}
		d.tellWindow(dictationNote{Session: run.session, Phase: finalNote, Text: text})
		return
	}
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

// typeDictation types text into run's tab, without Return, only while its agent is in front there, as the
// deck's keys go (Deck.typeDictation), and never while it asks you something, as the phone's words and the
// window's go (typeTarget): checked right before the text, on the key worker. A terminal that stopped
// answering, or a desk that began asking, says so over the desk. Called without the lock.
func (d *Deck) typeDictation(run *dictationRun, text string) {
	// The core types it, maybe as Claude tidied it: never kept as the owner's words (ownerwords.go, RT126MEM3 N4).
	d.coreTypes(run.tab, text)
	var failed error
	var asked bool
	_, _ = d.toAgent(run.tab, run.agent, func(t Tab) error {
		if d.tabAsking(run.tab) {
			asked = true
			return nil
		}
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
	if asked {
		d.noteVoice(run.tab, dictationAsking, false)
		return
	}
	// The Mac app's sentence for a terminal that stopped answering, said over the desk: only that, the Swift
	// engine's delivery's 503 (RGAPA N3). Any other no, and an agent that has ended or isn't in front, is
	// left as the deck leaves it.
	var refusal appRefusal
	if errors.As(failed, &refusal) && terminalNotAnswering(refusal.Error()) {
		d.noteVoice(run.tab, refusal.Error(), false)
	}
}

// dictationPartial is words a source has heard so far, for the recording that's on for the window's box: they go
// to the window's event stream, one line as dictationText makes it, and replace the last partial there. Nothing
// else hears them. Words for a recording that isn't the box's, or is being stopped, or that's empty, go nowhere.
// Called without the lock.
func (d *Deck) dictationPartial(heard string) {
	d.mu.Lock()
	run := d.dictating
	live := run != nil && run.target == dictateToBox && !run.stopping
	d.mu.Unlock()
	if text := dictationText(heard); live && text != "" {
		d.tellWindow(dictationNote{Session: run.session, Phase: partialNote, Text: text})
	}
}

// tellWindow says note to the office's own window, on its event stream alone. Not the state: no phone, no viewer,
// no push, no webhook and no event hears it, and it's kept nowhere. Called without the lock.
func (d *Deck) tellWindow(note dictationNote) {
	text, err := json.Marshal(note)
	if err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for stream, phone := range d.streams {
		if phone == officeWindowID {
			stream.note(text)
		}
	}
}

// listenOff is what a computer with no desktop microphone says: Linux and Windows have none in 1.26. Said in the
// Mac's words where it is a Mac (writeStatus, inMacWords), though a Mac app has one.
const listenOff = "Dictation isn't available on this computer."

// ListenStart is POST /talk/listen?session=&phase=start: the Mac records from its own microphone for session id's
// input box in the window. The words wait in the box for Send; nothing is typed in the desk's tab. Called without
// the lock.
func (d *Deck) ListenStart(id string) (int, string) {
	d.mu.Lock()
	if d.mac.host == nil {
		d.mu.Unlock()
		return http.StatusConflict, listenOff
	}
	// Off in setup, or the Mac unable to (or a test copy): what the mic says on the desk's keys.
	if !d.dictationOn() {
		d.mu.Unlock()
		return http.StatusConflict, dictationCouldntStart
	}
	// A hire in T3 Code's desk takes no words from here, as typeTarget says; a headless run has no input to send to.
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return http.StatusConflict, t3NoControls(calledHire(hire))
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "Session not found"
	}
	run := &dictationRun{target: dictateToBox, session: id, tab: s.Tab, agent: s.Agent, phone: officeWindowID, starting: true}
	headless := s.Headless()
	d.mu.Unlock()
	if headless {
		return http.StatusConflict, headlessNoControls
	}
	return d.beginDictation(run)
}

// ListenStop is POST /talk/listen?session=&phase=stop: let go of the box's mic. What was heard goes to the window,
// as its final note, once the app has it. A recording for another desk, a tab's or none is left alone and nothing
// is said, as the mic let go with nothing recording. Called without the lock.
func (d *Deck) ListenStop(id string) (int, string) {
	d.mu.Lock()
	run := d.dictating
	d.mu.Unlock()
	if run != nil && run.target == dictateToBox && run.session == id {
		d.rounds.run(func() { d.endDictation(run) })
	}
	return http.StatusNoContent, ""
}

// POST /talk/listen?session=&phase=start|stop: the mic in a desk's input box, from the office's own window on
// this computer, never a phone's. In the table of routes by class (phoneroutes.go), so a phone's key is refused
// in the window's sentence before anything else.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/talk/listen", access: accessWindow, refusal: talkRefusal,
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			switch r.URL.Query().Get("phase") {
			case "start":
				writeStatus(w)(d.ListenStart(session))
			case "stop":
				writeStatus(w)(d.ListenStop(session))
			default:
				writeStatus(w)(http.StatusBadRequest, "Say phase=start or phase=stop")
			}
		}})
}
