//go:build !linux && !windows

package desktop

import "errors"

// The jukebox on a Mac is the Mac app's: under it, its keys, whether music is
// playing and the volume are asked of the app over the bridge, by the Swift
// engine's own code (Music.swift), as its jukebox does them. Without the app
// there's no music control here.

func musicKey(key string) error {
	if host == nil {
		return errors.New("no music control here")
	}
	return hostMusicKey(host, key)
}

func musicPlaying() *bool {
	if host == nil {
		return nil
	}
	playing, _, _ := hostMusicState(host)
	return playing
}

func getVolume() *musicVolume {
	if host == nil {
		return nil
	}
	_, volume, _ := hostMusicState(host)
	return volume
}

func setVolume(level int) error {
	if host == nil {
		return errors.New("no volume control here")
	}
	return hostSetVolume(host, level)
}
