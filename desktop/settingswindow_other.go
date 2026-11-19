//go:build !windows

package desktop

import "errors"

// Off Windows there's no Settings window of the Go app's own (settingswindow.go): the Mac app has its own, and Linux
// opens Setup in the browser (FXOWNSETTINGS' scope: no webview Smolcorp can ship there yet without cgo).

func settingsWebViewHere() bool { return false }

var errNoSettingsWindow = errors.New("no Settings window on this computer")

func startSettingsWindowHere(*Deck, settingsRun) error { return errNoSettingsWindow }
