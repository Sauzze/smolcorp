//go:build darwin && !cgo

package sandbox

// deniesCanary: without cgo nothing can ask macOS (the app's own build has it: scripts/build-app.sh), so every server
// check fails and no desk starts in a sandbox.
func deniesCanary(int, string) (tokenPID, denied int) { return 0, -1 }
