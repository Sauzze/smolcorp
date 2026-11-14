//go:build !linux && !darwin && !windows

package desktop

import "net"

// deskListeners can't look on this system: a desk's preview is the declared one alone.
func deskListeners(previewRoot, []int) ([]tcpListener, bool) { return nil, false }

// connectionOwner can't look on this system either.
func connectionOwner(*net.TCPAddr, *net.TCPAddr) int { return 0 }
