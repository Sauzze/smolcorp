package desktop

import (
	"encoding/json"
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
		_ = json.Unmarshal(serve(t, d, call{method: "POST", target: "/pair", host: deckOnWiFi, from: aPhone, body: `{"code": "000000"}`}).Body.Bytes(), &refused)
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
