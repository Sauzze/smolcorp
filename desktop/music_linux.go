package desktop

import (
	"os/exec"
	"strings"
)

// musicKey plays or pauses whatever player is playing, or skips a song
// either way, through MPRIS, as playerctl reaches it: music apps, and
// browsers playing a video. key is one of musicKeys.
func musicKey(key string) error {
	command := map[string]string{"playpause": "play-pause", "previous": "previous", "next": "next"}[key]
	return exec.Command("playerctl", command).Run()
}

// musicPlaying is whether a player is playing, or nil without playerctl or
// any player to ask.
func musicPlaying() *bool {
	out, err := exec.Command("playerctl", "status").Output()
	if err != nil {
		return nil
	}
	playing := strings.TrimSpace(string(out)) == "Playing"
	return &playing
}
