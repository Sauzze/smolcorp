//go:build !linux && !windows

package desktop

import "errors"

// The jukebox is Windows' and Linux's here; the Mac has its own app.

func musicKey(string) error { return errors.New("no music control here") }

func musicPlaying() *bool { return nil }
