package desktop

import "golang.org/x/sys/windows"

// The keyboard's media keys, which Windows sends to whatever is playing: a
// music app, a video in a browser.
var keybdEvent = windows.NewLazySystemDLL("user32.dll").NewProc("keybd_event")

const (
	vkMediaNextTrack  = 0xB0
	vkMediaPrevTrack  = 0xB1
	vkMediaPlayPause  = 0xB3
	keyeventfExtended = 0x1
	keyeventfKeyUp    = 0x2
)

// musicKey presses play/pause, previous or next, as the keyboard's media
// keys do. key is one of musicKeys.
func musicKey(key string) error {
	if err := keybdEvent.Find(); err != nil {
		return err
	}
	code := map[string]uintptr{"playpause": vkMediaPlayPause, "previous": vkMediaPrevTrack, "next": vkMediaNextTrack}[key]
	keybdEvent.Call(code, 0, keyeventfExtended, 0)
	keybdEvent.Call(code, 0, keyeventfExtended|keyeventfKeyUp, 0)
	return nil
}

// musicPlaying is whether music is playing, which Windows doesn't say here.
func musicPlaying() *bool { return nil }
