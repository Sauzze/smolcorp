package desktop

import (
	"os/exec"
	"strings"
)

// playPause plays or pauses whatever player is playing, through MPRIS, as
// playerctl reaches it: music apps, and browsers playing a video.
func playPause() error {
	return exec.Command("playerctl", "play-pause").Run()
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
