//go:build !darwin

package desktop

import "net"

// listenServeSocket makes no serve.sock on Linux and Windows: the hand-on stays off there (serveHandOnHere), so
// nothing is handed on (peerowner.go). The piece that turns it on there identifies the socket's owner: on Linux by
// SO_PEERCRED (root), on Windows by the owning pid (the Tailscale service's).
func listenServeSocket() (net.Listener, error) { return nil, nil }

// identifyServePeerHere identifies nothing on Linux and Windows yet.
func identifyServePeerHere(net.Conn) bool { return false }
