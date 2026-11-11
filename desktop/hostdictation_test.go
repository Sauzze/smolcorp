package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// dictationDeck is a deck under the stand-in Mac app, which can dictate (its facts learned, as the deck learns
// them), with dictation on, and a Codex desk in a Terminal tab whose terminal types text: the office's mic
// dictates there. Never a real microphone or speech engine: the stand-in answers for them.
func dictationDeck(t *testing.T) (*Deck, *macStandIn, string) {
	t.Helper()
	mac := withMac(t)
	mac.facts.Dictation.Available, mac.facts.Dictation.Allowed = true, true
	d := newTestDeck(t)
	d.macFactsNow()
	const tab = "tty:/dev/ttys003"
	d.mu.Lock()
	d.config.DictationEnabled = true
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "codex-1", "cwd": "/work/smolcorp"}, Codex, tab, "", 100)
	// Learned for this tab, so the office doesn't ask again (hostoffice.go): a test changes them by hand.
	d.mac.abilities, d.mac.abilitiesFor = map[string]tabAbilities{tab: {Keys: true, Text: true, Fire: true}}, tab
	d.mu.Unlock()
	return d, mac, tab
}

// sessionSaid is session id's entry in the snapshot's keys, and the snapshot's dictation.
func sessionSaid(t *testing.T, d *Deck, id string) (map[string]any, map[string]any) {
	t.Helper()
	d.mu.Lock()
	var state struct {
		Dictation map[string]any
		Keys      []struct{ Session map[string]any }
	}
	_ = json.Unmarshal(d.snapshot(), &state)
	d.mu.Unlock()
	for _, key := range state.Keys {
		if key.Session["id"] == id {
			return key.Session, state.Dictation
		}
	}
	t.Fatalf("no desk for %s", id)
	return nil, nil
}

// askedFor is the stand-in's requests of one action, in order.
func askedFor(mac *macStandIn, action string) []bridgeRequest {
	mac.mu.Lock()
	defer mac.mu.Unlock()
	var found []bridgeRequest
	for _, request := range mac.asked {
		if request.Action == action {
			found = append(found, request)
		}
	}
	return found
}

// Holding the mic on a desk it dictates for has the Mac app record for that tab, in dictation's language; let go,
// what it heard, cleaned up, goes into the tab without Return, and a note over the desk says how it went, for a
// moment: as the Swift engine's mic (Deck.perform, insertDictation, noteVoice).
func TestTheMicDictatesThroughTheMacApp(t *testing.T) {
	saved := voiceNoteLife
	voiceNoteLife = 200 * time.Millisecond
	t.Cleanup(func() { voiceNoteLife = saved })
	d, mac, tab := dictationDeck(t)
	mac.heard = "Fix the Smolcorp build"
	d.mu.Lock()
	d.config.DictationCleanup = true
	website := "/work/website"
	d.config.Pins[0] = &website
	d.mu.Unlock()

	entry, dictation := sessionSaid(t, d, "codex-1")
	if dictation["on"] != true || entry["dictates"] != true || entry["mic"] != true {
		t.Fatalf("dictation %v, the desk's dictates %v and mic %v", dictation, entry["dictates"], entry["mic"])
	}
	if entry["voiceNote"] != nil {
		t.Errorf("a voice note before any dictation: %v", entry["voiceNote"])
	}
	if status, text := d.PerformFrom("phone-1", "mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusNoContent {
		t.Fatalf("the mic held: %d %q", status, text)
	}
	started := askedFor(mac, "dictation-start")
	if len(started) != 1 || started[0].Tab != tab || started[0].Language != "en_US" {
		t.Fatalf("the app was asked %+v", started)
	}
	// A second desk's mic while this one records: one Mac, one microphone.
	if status, text := d.PerformFrom("phone-2", "mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusConflict || text != dictationCouldntStart {
		t.Errorf("a second recording: %d %q", status, text)
	}
	if status, _ := d.Perform("mic", "", url.Values{"phase": {"up"}}); status != http.StatusNoContent {
		t.Fatalf("the mic let go: %d", status)
	}
	eventually(t, "typed what it heard", func() bool { return len(askedFor(mac, "insert")) == 1 })
	clean := askedFor(mac, "dictation-clean")
	if len(askedFor(mac, "dictation-stop")) != 1 || len(clean) != 1 || clean[0].Text != "Fix the Smolcorp build" || clean[0].On == nil || !*clean[0].On ||
		clean[0].Glossary != nil || !slices.Equal(clean[0].Desks, []string{"smolcorp", "website"}) {
		t.Errorf("the clean-up asked %+v", clean)
	}
	insert := askedFor(mac, "insert")[0]
	if insert.Tab != tab || insert.Agent != Codex || insert.Text != "Fix the Smolcorp build" {
		t.Errorf("typed %+v", insert)
	}
	if slices.Contains(mac.actions(), "press") {
		t.Error("a key was pressed: dictation goes in without Return")
	}
	entry, _ = sessionSaid(t, d, "codex-1")
	note, _ := entry["voiceNote"].(map[string]any)
	if note["text"] != "cleaned in 0.9 s" || note["cleaned"] != true || note["id"] == nil || note["at"] == nil {
		t.Errorf("the voice note: %v", entry["voiceNote"])
	}
	eventually(t, "the note forgotten", func() bool {
		entry, _ := sessionSaid(t, d, "codex-1")
		return entry["voiceNote"] == nil
	})
	// Let go again with nothing recording: nothing asked, and no Space for Claude's own voice mode.
	if status, _ := d.Perform("mic", "", url.Values{}); status != http.StatusNoContent || len(askedFor(mac, "dictation-stop")) != 1 {
		t.Errorf("a release with nothing recording: %d, %d stops", status, len(askedFor(mac, "dictation-stop")))
	}
}

// The clean-up off, the text goes in as heard, with no note: as before it existed. Nothing heard, nothing typed.
func TestDictationWithoutTheCleanUpHasNoNoteAndNothingHeardTypesNothing(t *testing.T) {
	d, mac, _ := dictationDeck(t)
	glossary := []string{"Tailscale"}
	d.mu.Lock()
	d.config.DictationGlossary = &glossary
	d.mu.Unlock()
	mac.heard = "  hello\nthere\t "
	d.PerformFrom("phone-1", "mic", "codex-1", url.Values{"phase": {"down"}})
	d.Perform("mic", "", url.Values{})
	eventually(t, "typed", func() bool { return len(askedFor(mac, "insert")) == 1 })
	if text := askedFor(mac, "insert")[0].Text; text != "hello there" {
		t.Errorf("typed %q: one line, no control character", text)
	}
	if clean := askedFor(mac, "dictation-clean")[0]; *clean.On || clean.Glossary == nil || !slices.Equal(*clean.Glossary, glossary) || clean.Text != "hello there" {
		t.Errorf("the clean-up asked %+v", clean)
	}
	if entry, _ := sessionSaid(t, d, "codex-1"); entry["voiceNote"] != nil {
		t.Errorf("a note with the clean-up off: %v", entry["voiceNote"])
	}
	mac.heard = ""
	d.PerformFrom("phone-1", "mic", "codex-1", url.Values{"phase": {"down"}})
	d.Perform("mic", "", url.Values{})
	eventually(t, "stopped", func() bool { return len(askedFor(mac, "dictation-stop")) == 2 })
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac, "insert")) != 1 || len(askedFor(mac, "dictation-clean")) != 1 {
		t.Error("nothing heard was cleaned up or typed")
	}
}

// A recording stops by itself at its cap, and when the phone holding the mic goes away: what it heard goes in.
func TestDictationStopsAtItsCapAndWhenItsPhoneLeaves(t *testing.T) {
	saved := dictationLongest
	dictationLongest = 100 * time.Millisecond
	t.Cleanup(func() { dictationLongest = saved })
	d, mac, _ := dictationDeck(t)
	mac.heard = "capped"
	d.PerformFrom("phone-1", "mic", "codex-1", url.Values{"phase": {"down"}})
	eventually(t, "stopped at its cap", func() bool { return len(askedFor(mac, "insert")) == 1 })

	dictationLongest = time.Hour
	stream := d.attach("phone-1")
	other := d.attach("phone-2")
	d.PerformFrom("phone-1", "mic", "codex-1", url.Values{"phase": {"down"}})
	d.detach(other)
	time.Sleep(50 * time.Millisecond)
	if len(askedFor(mac, "dictation-stop")) != 1 {
		t.Fatal("another phone leaving stopped it")
	}
	d.detach(stream)
	eventually(t, "stopped as its phone left", func() bool { return len(askedFor(mac, "insert")) == 2 })
}

// The mic's refusals, as the Swift engine's: a terminal that types no text, Accessibility missing where its keys
// need it, and a recording that couldn't start, which leaves nothing recording.
func TestTheMicsRefusalsAreTheMacs(t *testing.T) {
	d, mac, tab := dictationDeck(t)
	mac.notStarting = true
	if status, text := d.PerformFrom("", "mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusConflict || text != dictationCouldntStart {
		t.Errorf("not started: %d %q", status, text)
	}
	mac.notStarting = false
	mac.refusing(map[string]string{"dictation-start": macNoKeyboard})
	if status, text := d.PerformFrom("", "mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusForbidden || text != macNoKeyboard {
		t.Errorf("no Accessibility: %d %q", status, text)
	}
	mac.refusing(nil)
	d.mu.Lock()
	d.mac.abilities[tab] = tabAbilities{Keys: true}
	d.mu.Unlock()
	if status, text := d.PerformFrom("", "mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusConflict || text != notHere {
		t.Errorf("no text in that terminal: %d %q", status, text)
	}
	if entry, _ := sessionSaid(t, d, "codex-1"); entry["mic"] != false || entry["dictates"] != false {
		t.Errorf("the desk offers the mic: %v %v", entry["mic"], entry["dictates"])
	}
	d.mu.Lock()
	recording := d.dictating != nil
	d.mu.Unlock()
	if recording {
		t.Error("a refused start left something recording")
	}
}

// A Claude Code desk keeps its own voice mode (Space) unless setup says Claude Code too, or the clean-up is on;
// and dictation off, or a Mac that can't, every desk does as before.
func TestClaudeCodeKeepsItsVoiceModeUnlessSetupSaysOtherwise(t *testing.T) {
	d, _, _ := dictationDeck(t)
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range []struct {
		enabled, canDictate, forClaude, cleanup bool
		claude, codex                           bool
	}{
		{true, true, false, false, false, true},
		{true, true, true, false, true, true},
		{true, true, false, true, true, true},
		{false, true, true, true, false, false},
		{true, false, true, true, false, false},
	} {
		d.config.DictationEnabled, d.mac.canDictate = c.enabled, c.canDictate
		d.config.DictationForClaude, d.config.DictationCleanup = c.forClaude, c.cleanup
		if d.dictatesFor(Claude) != c.claude || d.dictatesFor(Codex) != c.codex {
			t.Errorf("%+v: Claude %v, Codex %v", c, d.dictatesFor(Claude), d.dictatesFor(Codex))
		}
	}
}

// The mic's clean-up switch: kept under the Mac app, for every phone; elsewhere only said; anything but 0 or 1 refused.
func TestTheCleanUpSwitch(t *testing.T) {
	d, _, _ := dictationDeck(t)
	if status, text := d.Perform("cleanup", "", url.Values{"on": {"yes"}}); status != http.StatusBadRequest || text != "Say on=0 or on=1" {
		t.Errorf("on=yes: %d %q", status, text)
	}
	if status, _ := d.Perform("cleanup", "", url.Values{"on": {"1"}}); status != http.StatusNoContent || !loadConfig().DictationCleanup {
		t.Errorf("on: %d, kept %v", status, loadConfig().DictationCleanup)
	}
	if _, dictation := sessionSaid(t, d, "codex-1"); dictation["cleanup"] != true {
		t.Errorf("the office's dictation: %v", dictation)
	}

	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	plain := newTestDeck(t)
	if status, _ := plain.Perform("cleanup", "", url.Values{"on": {"0"}}); status != http.StatusNoContent {
		t.Errorf("without the Mac app: %d", status)
	}
}

// Without the Mac app nothing dictates: the mic presses the agent's own voice key as before, and a desk's voice
// note is always null.
func TestWithoutTheMacAppTheMicDoesntDictate(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.DictationEnabled, d.config.DictationForClaude = true, true
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "codex-1", "cwd": "/work/smolcorp"}, Codex, "tmux:%1@/tmp/tmux", "", 100)
	dictates := d.dictatesFor(Codex)
	d.mu.Unlock()
	if dictates {
		t.Error("dictates without the Mac app")
	}
	if status, text := d.Perform("mic", "codex-1", url.Values{"phase": {"down"}}); status != http.StatusBadRequest || !strings.Contains(text, "no voice input") {
		t.Errorf("Codex's mic: %d %q", status, text)
	}
	entry, _ := sessionSaid(t, d, "codex-1")
	if voiceNote, ok := entry["voiceNote"]; !ok || voiceNote != nil {
		t.Errorf("the voice note: %v (there %v)", voiceNote, ok)
	}
}

// What's typed is one line, with no control character, and no longer than a tab takes at once.
func TestDictationsTextIsOneLine(t *testing.T) {
	for heard, want := range map[string]string{
		"":                       "",
		"  bonjour ":             "bonjour",
		"one\ntwo\r\nthree\x1b[": "one two  three [",
	} {
		if got := dictationText(heard); got != want {
			t.Errorf("%q: %q, want %q", heard, got, want)
		}
	}
	if got := dictationText(strings.Repeat("é", dictationTextMost+10)); len([]rune(got)) != dictationTextMost {
		t.Errorf("a long one kept %d characters", len([]rune(got)))
	}
}
