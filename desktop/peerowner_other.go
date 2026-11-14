//go:build !darwin && !linux && !windows

package desktop

import "net"

// listenServeSocket makes no serve.sock on a system Smolcorp can't tell Tailscale's process on (not a Mac, Linux or
// Windows): nothing is handed on there (peerowner.go).
func listenServeSocket() (net.Listener, error) { return nil, nil }

// identifyServePeerHere identifies nothing there.
func identifyServePeerHere(net.Conn) bool { return false }
