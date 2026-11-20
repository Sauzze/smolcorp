//go:build darwin && !cgo

package desktop

import "net/http"

// askingProgramHere: without cgo nothing can ask macOS which program asked (the app's own build has it:
// scripts/build-app.sh), so every ask is taken as one that may be a hire's, and the window says so (askprogram.go).
func askingProgramHere(*http.Request) askAsker { return askAsker{sandbox: sandboxUnknown} }
