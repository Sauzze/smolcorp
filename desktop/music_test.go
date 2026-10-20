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

func TestAJukeboxSetsTheVolumeItWasAskedFor(t *testing.T) {
	d := newTestDeck(t)
	set := []int{}
	savedSet, savedRead := setMusicVolume, readMusicVolume
	setMusicVolume = func(level int) error { set = append(set, level); return nil }
	readMusicVolume = func() *musicVolume { return nil }
	t.Cleanup(func() { setMusicVolume, readMusicVolume = savedSet, savedRead })

	volume := func(level string) int {
		status, _ := d.Perform("music", "", url.Values{"do": {"volume"}, "level": {level}})
		return status
	}
	if status := volume("40"); status != http.StatusForbidden || len(set) != 0 {
		t.Fatalf("with the jukebox off: %d, set %v", status, set)
	}
	d.config.MusicEnabled = true
	for _, bad := range []string{"", "-1", "101", "4.5", "loud", "07"} {
		if status := volume(bad); status != http.StatusBadRequest {
			t.Fatalf("%q: %d", bad, status)
		}
	}
	if len(set) != 0 {
		t.Fatalf("a bad level reached the setter: %v", set)
	}
	for _, good := range []string{"0", "40", "100"} {
		if status := volume(good); status != http.StatusNoContent {
			t.Fatalf("%q: %d", good, status)
		}
	}
	if len(set) != 3 || set[0] != 0 || set[1] != 40 || set[2] != 100 {
		t.Fatalf("set %v, want [0 40 100]", set)
	}
}

func TestTheVolumeIsReadFromWpctlAndPactl(t *testing.T) {
	if v := parseWpctlVolume("Volume: 0.45\n"); v == nil || *v != (musicVolume{45, false}) {
		t.Fatalf("wpctl: %v", v)
	}
	if v := parseWpctlVolume("Volume: 1.20 [MUTED]\n"); v == nil || *v != (musicVolume{100, true}) {
		t.Fatalf("wpctl, loud and muted: %v", v)
	}
	if v := parseWpctlVolume("nothing"); v != nil {
		t.Fatalf("wpctl, nonsense: %v", v)
	}
	if v := parsePactlVolume("Volume: front-left: 29491 /  45% / -20.81 dB,   front-right: 29491 /  45% / -20.81 dB\n", "Mute: yes\n"); v == nil || *v != (musicVolume{45, true}) {
		t.Fatalf("pactl: %v", v)
	}
	if v := parsePactlVolume("", ""); v != nil {
		t.Fatalf("pactl, nothing: %v", v)
	}
}
