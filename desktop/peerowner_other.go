//go:build !darwin && !linux

package desktop

import "net"

// listenServeSocket makes no serve.sock on Windows: the hand-on stays off there (serveHandOnHere), so nothing is
// handed on (peerowner.go). The piece that turns it on there (OLD-SERVE2) identifies the other end by the owning pid
// (the Tailscale service's), over TCP: Windows' `tailscale serve` has no unix: target.
func listenServeSocket() (net.Listener, error) { return nil, nil }

// identifyServePeerHere identifies nothing on Windows yet.
func identifyServePeerHere(net.Conn) bool { return false }
