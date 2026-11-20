//go:build !windows && !darwin

package desktop

import "net/http"

// askingProgramHere: on Linux nothing is read (askprogram.go), as it takes no ask; the window would show the ask's
// folder alone.
func askingProgramHere(*http.Request) askAsker { return askAsker{} }
