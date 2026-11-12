//go:build darwin

package desktop

import (
	"net/http"
	"net/url"
	"testing"
)

// Under the Mac app the jukebox is the Mac's, as the Swift engine's: its keys only with Accessibility, in the
// app's words; the volume set; whether music plays and the volume, followed.
func TestUnderTheMacAppTheJukeboxIsTheMacs(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.MusicEnabled = true
	d.mu.Unlock()
	if status, message := d.Perform("music", "", url.Values{"do": {"playpause"}}); status != http.StatusForbidden ||
		message != "Keyboard access is off. Turn it on in setup, on your Mac." {
		t.Fatalf("without Accessibility: %d %s", status, message)
	}
	mac.mu.Lock()
	mac.keyboard = true
	mac.mu.Unlock()
	if status, message := d.Perform("music", "", url.Values{"do": {"next"}}); status != http.StatusNoContent {
		t.Fatalf("with Accessibility: %d %s", status, message)
	}
	if status, message := d.Perform("music", "", url.Values{"do": {"volume"}, "level": {"30"}}); status != http.StatusNoContent {
		t.Fatalf("the volume: %d %s", status, message)
	}
	d.followMusic()
	d.mu.Lock()
	state, _ := d.musicState().(map[string]any)
	d.mu.Unlock()
	if playing, _ := state["playing"].(*bool); playing == nil || !*playing || state["volume"] != 30 || state["muted"] != false {
		t.Errorf("the jukebox shows %v", state)
	}
}
