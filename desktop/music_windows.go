package desktop

import "golang.org/x/sys/windows"

// The keyboard's play/pause media key, which Windows sends to whatever is
// playing: a music app, a video in a browser.
var keybdEvent = windows.NewLazySystemDLL("user32.dll").NewProc("keybd_event")

const (
	vkMediaPlayPause  = 0xB3
	keyeventfExtended = 0x1
	keyeventfKeyUp    = 0x2
)

// playPause presses play/pause, as the keyboard's media key does.
func playPause() error {
	if err := keybdEvent.Find(); err != nil {
		return err
	}
	keybdEvent.Call(vkMediaPlayPause, 0, keyeventfExtended, 0)
	keybdEvent.Call(vkMediaPlayPause, 0, keyeventfExtended|keyeventfKeyUp, 0)
	return nil
}

// musicPlaying is whether music is playing, which Windows doesn't say here.
func musicPlaying() *bool { return nil }
