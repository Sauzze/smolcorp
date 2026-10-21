package desktop

import (
	"regexp"
	"strconv"
	"strings"
)

// musicVolume is this computer's output volume, for the jukebox's volume
// bar: a level from 0 to 100, and whether the output is muted.
type musicVolume struct {
	level int
	muted bool
}

// volumeLevel reads a level from the office: a whole number from 0 to 100,
// or false for anything else.
func volumeLevel(text string) (int, bool) {
	level, err := strconv.Atoi(text)
	if err != nil || level < 0 || level > 100 || strconv.Itoa(level) != text {
		return 0, false
	}
	return level, true
}

// clampVolume keeps a level read from the system, which can go past 100%,
// within the bar's 0 to 100.
func clampVolume(level int) int {
	return min(max(level, 0), 100)
}

// parseWpctlVolume reads `wpctl get-volume`, as "Volume: 0.45" or
// "Volume: 0.45 [MUTED]".
func parseWpctlVolume(out string) *musicVolume {
	fields := strings.Fields(out)
	if len(fields) < 2 || fields[0] != "Volume:" {
		return nil
	}
	scalar, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return nil
	}
	return &musicVolume{level: clampVolume(int(scalar*100 + 0.5)), muted: strings.Contains(out, "[MUTED]")}
}

// pactlPercent finds the first channel's percentage in `pactl
// get-sink-volume`, as in "Volume: front-left: 29491 /  45% / -20.81 dB, …".
var pactlPercent = regexp.MustCompile(`(\d+)%`)

// parsePactlVolume reads `pactl get-sink-volume` and `pactl get-sink-mute`
// ("Mute: yes").
func parsePactlVolume(volume, mute string) *musicVolume {
	match := pactlPercent.FindStringSubmatch(volume)
	if match == nil {
		return nil
	}
	level, _ := strconv.Atoi(match[1])
	return &musicVolume{level: clampVolume(level), muted: strings.TrimSpace(mute) == "Mute: yes"}
}
