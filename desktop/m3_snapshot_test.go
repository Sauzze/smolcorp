package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// The Mac always sends these keys, false or empty too: Go must not leave them out.
func TestFloorRuleAndBotAlwaysSendTheirKeys(t *testing.T) {
	rule, _ := json.Marshal(FloorRule{Folder: "/x", Floor: 1})
	var got map[string]any
	json.Unmarshal(rule, &got)
	if v, ok := got["exact"]; !ok || v != false {
		t.Errorf("a rule's exact is sent when false: %s", rule)
	}
	bot, _ := json.Marshal(CloudBot{ID: "b1", Name: "B"})
	got = nil
	json.Unmarshal(bot, &got)
	for key, want := range map[string]any{"discovered": false, "pinned": false, "mailbox": ""} {
		if v, ok := got[key]; !ok || v != want {
			t.Errorf("a bot's %s is sent when false or empty: %s", key, bot)
		}
	}
}

// POST /act?action=language off the Mac app: a known language is 204 and nothing else (on Linux and Windows
// nothing is stored and nothing dictates: dictation is the Mac's, via the bridge); an unknown or missing one is
// the Mac's 400.
func TestActLanguageAnswersAsTheMacDoes(t *testing.T) {
	isolate(t)
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	before, _ := json.Marshal(d.config)
	for _, lang := range []string{"fr_FR", "en_US"} {
		if code, text := d.Perform("language", "", url.Values{"lang": {lang}}); code != http.StatusNoContent || text != "" {
			t.Errorf("lang=%s: %d %q, want 204", lang, code, text)
		}
	}
	if after, _ := json.Marshal(d.config); string(after) != string(before) {
		t.Errorf("a known language changed the config:\n%s\n%s", before, after)
	}
	for _, q := range []url.Values{{"lang": {"xx"}}, {"lang": {""}}, {}} {
		if code, text := d.Perform("language", "", q); code != http.StatusBadRequest || text != "Unknown language" {
			t.Errorf("query %v: %d %q, want 400 Unknown language", q, code, text)
		}
	}
}

// Under the Mac app (engine=go), POST /act?action=language is dictation's language, as the Mac's Deck has it:
// kept in config.json, saved, and sent to every phone in the office's snapshot. It is checked against the
// languages that Mac has, so one the Mac doesn't have is the Mac's 400 and changes nothing.
func TestActLanguageUnderTheMacAppIsKeptAndSent(t *testing.T) {
	mac := withMac(t)
	// This Mac has no en_US, which the Linux and Windows list has, and suggests de_DE until one is chosen.
	mac.facts.Dictation.Languages, mac.facts.Dictation.Suggested = []string{"fr_FR", "de_DE"}, "de_DE"
	d := newTestDeck(t)
	d.macFactsNow() // the languages this Mac has, as the deck learns them at its start
	for _, q := range []url.Values{{"lang": {"en_US"}}, {"lang": {"xx"}}, {}} {
		if code, text := d.Perform("language", "", q); code != http.StatusBadRequest || text != "Unknown language" {
			t.Errorf("query %v: %d %q, want 400 Unknown language", q, code, text)
		}
	}
	if kept := loadConfig().DictationLanguage; kept != "" {
		t.Errorf("a refused language was kept: %q", kept)
	}

	told := make(chan struct{}, 64)
	d.mu.Lock()
	d.OnChange = func() { told <- struct{}{} }
	d.mu.Unlock()
	if code, text := d.Perform("language", "", url.Values{"lang": {"fr_FR"}}); code != http.StatusNoContent || text != "" {
		t.Fatalf("fr_FR: %d %q, want 204", code, text)
	}
	if kept := loadConfig().DictationLanguage; kept != "fr_FR" {
		t.Errorf("config.json's dictationLanguage: %q, want fr_FR", kept)
	}
	select {
	case <-told:
	case <-time.After(2 * time.Second):
		t.Error("the phones weren't told")
	}
	d.mu.Lock()
	snapshot := d.snapshot()
	d.mu.Unlock()
	var office struct {
		Dictation map[string]any `json:"dictation"`
	}
	if err := json.Unmarshal(snapshot, &office); err != nil {
		t.Fatal(err)
	}
	if dictation := office.Dictation; dictation["language"] != "fr_FR" {
		t.Errorf("the office's dictation: %v", dictation)
	}
}
