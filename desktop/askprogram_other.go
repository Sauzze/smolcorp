//go:build !windows

package desktop

import "net/http"

// askingProgramHere: off Windows nothing is read (askprogram.go). On a Mac a hire asks from inside its sandbox, and
// Linux takes no ask; the window shows the ask's folder alone, as before.
func askingProgramHere(*http.Request) askAsker { return askAsker{} }
