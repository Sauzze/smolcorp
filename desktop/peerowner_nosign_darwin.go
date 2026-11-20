//go:build darwin && !cgo

package desktop

// Without cgo nothing can ask macOS (the app's own build has it: scripts/build-app.sh): no token is read and no
// process is Tailscale's, so every connection to serve.sock is refused.
func peerTokenHere(int) (auditToken, bool) { return auditToken{}, false }

func tailscaleSignedHere(auditToken) bool { return false }
