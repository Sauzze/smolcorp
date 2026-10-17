package desktop

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAJukeboxPressesTheKeyItWasAskedFor(t *testing.T) {
	d := newTestDeck(t)
	pressed := []string{}
	saved := pressMusicKey
	pressMusicKey = func(key string) error { pressed = append(pressed, key); return nil }
	t.Cleanup(func() { pressMusicKey = saved })

	if status, _ := d.Perform("music", "", url.Values{}); status != http.StatusForbidden || len(pressed) != 0 {
		t.Fatalf("with the jukebox off: %d, pressed %v", status, pressed)
	}
	d.config.MusicEnabled = true
	for _, key := range []string{"", "previous", "next", "playpause"} {
		if status, message := d.Perform("music", "", url.Values{"do": {key}}); status != http.StatusNoContent {
			t.Fatalf("%q: %d %s", key, status, message)
		}
	}
	if want := []string{"playpause", "previous", "next", "playpause"}; len(pressed) != len(want) ||
		pressed[0] != want[0] || pressed[1] != want[1] || pressed[2] != want[2] || pressed[3] != want[3] {
		t.Fatalf("pressed %v, want %v", pressed, want)
	}
	if status, _ := d.Perform("music", "", url.Values{"do": {"eject"}}); status != http.StatusBadRequest || len(pressed) != 4 {
		t.Fatalf("an unknown key: %d, pressed %v", status, pressed)
	}
}
