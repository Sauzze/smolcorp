package desktop

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The phone's sentences in the Mac's words under the Mac app, as the Swift engine says them; elsewhere "computer".
func TestUnderTheMacAppThePhoneHearsTheMacsWords(t *testing.T) {
	for _, mac := range []bool{false, true} {
		saved := host
		host = nil
		if mac {
			withMac(t)
		}
		d := newTestDeck(t)
		key := pair(t, d)
		said := func(target string) string {
			return serve(t, d, call{method: "POST", target: target + "&t=" + key, host: deckOnWiFi, from: aPhone}).Body.String()
		}
		var refused struct{ Error string }
		_ = json.Unmarshal(serve(t, d, call{method: "POST", target: "/pair", host: "localhost:7710", from: thisComputer, body: `{"code": "000000"}`}).Body.Bytes(), &refused)
		got := []string{refused.Error, said("/act?action=settings"), said("/act?action=music"), said("/act?action=hire&desk=0"),
			said("/peek?session=nobody")}
		want := []string{pairCodeRefusal, settingsElsewhere, musicOff, hiringOff, peekTailnet}
		if mac {
			want = []string{"That code isn't valid. Get a new one on your Mac.", "Settings open on your Mac.",
				"The jukebox can't play your music: turn it on in setup, on your Mac.",
				"Hiring and firing are off. Turn them on in setup, on your Mac.",
				"The input line shows only over Tailscale's HTTPS address."}
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("mac %v: %q, want %q", mac, got[i], want[i])
			}
		}
		// Each of them, and nothing else: the summaries' say "computer" on the Mac too.
		for words, inMac := range macWords {
			if got := inMacWords(words); mac && got != inMac || !mac && got != words {
				t.Errorf("mac %v: %q said %q", mac, words, got)
			}
		}
		if got := inMacWords(errSummaryOff.Message); got != errSummaryOff.Message {
			t.Errorf("mac %v: %q", mac, got)
		}
		host = saved
	}
}

// Setup's and the self-test's sentences that name this computer say "Mac" under the Mac app, as its Swift engine
// said them (Hooks.swift, DeckRouter.swift, HireFolder.swift); "computer" elsewhere.
func TestUnderTheMacAppSetupsRefusalsNameTheMac(t *testing.T) {
	for _, mac := range []bool{false, true} {
		saved := host
		host = nil
		if mac {
			withMac(t)
		}
		word := map[bool]string{false: "computer", true: "Mac"}[mac]
		home := t.TempDir()
		_, _, background := setChoices(&Config{}, map[string][]string{"background": {"1"}}, false)
		got := []string{noAgentsFound().Error(), background, hireFolderRefusal(filepath.Join(home, "Library", "x"), home)}
		want := []string{"No coding agent found on this " + word + ". Install one, open it once, then connect again.",
			"Running hires in the background needs tmux, which isn't on this " + word + ".",
			"That folder holds your " + word + "'s or your apps' own files. Choose a project folder, " + hireFolderExample + "."}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("mac %v: %q, want %q", mac, got[i], want[i])
			}
		}
		host = saved
	}
}
