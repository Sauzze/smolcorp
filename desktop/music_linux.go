package desktop

import (
	"os/exec"
	"strconv"
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

// getVolume is the default output's volume, through PipeWire's wpctl, or
// PulseAudio's pactl where there's no wpctl; nil with neither.
func getVolume() *musicVolume {
	if out, err := exec.Command("wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@").Output(); err == nil {
		return parseWpctlVolume(string(out))
	}
	volume, err := exec.Command("pactl", "get-sink-volume", "@DEFAULT_SINK@").Output()
	if err != nil {
		return nil
	}
	mute, _ := exec.Command("pactl", "get-sink-mute", "@DEFAULT_SINK@").Output()
	return parsePactlVolume(string(volume), string(mute))
}

// setVolume sets the default output's volume, 0 to 100. A level above 0
// unmutes, as the keyboard's volume keys do; 0 is simply 0.
func setVolume(level int) error {
	if _, err := exec.LookPath("wpctl"); err == nil {
		if err := exec.Command("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", strconv.FormatFloat(float64(level)/100, 'f', 2, 64)).Run(); err != nil {
			return err
		}
		if level > 0 {
			return exec.Command("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "0").Run()
		}
		return nil
	}
	if err := exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", strconv.Itoa(level)+"%").Run(); err != nil {
		return err
	}
	if level > 0 {
		return exec.Command("pactl", "set-sink-mute", "@DEFAULT_SINK@", "0").Run()
	}
	return nil
}
