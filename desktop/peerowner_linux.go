//go:build linux

package desktop

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// On Linux, as on a Mac, `tailscale serve` hands a phone's request on to serve.sock, a unix socket in the support
// folder (peerowner.go), and the kernel says who is at the other end of each connection to it (OLD-SERVE1). It must
// have connected as root (SO_PEERCRED: the user it connected as, set at connect and never changed by exec), and the
// process that connected must still be there, running as root now (/proc/<pid>/status), as tailscaled does as Linux's
// packages run it (judgeCredPeer). There is no signature to check on Linux, so any process running as root passes:
// say so wherever this is explained. A program of the owner's is refused, whatever it runs or sends.
//
// Over this computer's TCP loopback nothing counts as `tailscale serve` (servedByTailscale), on Linux as on a Mac.

// sunPathMost is the longest path a unix socket takes on Linux: sun_path is 108 bytes, its closing NUL among them.
const sunPathMost = 107

// listenServeSocket makes serve.sock again, as the app starts: under the support folder's instance lock, held by
// this process, so no second Smolcorp takes it over.
func listenServeSocket() (net.Listener, error) {
	if !serveHandOnHere() {
		return nil, nil
	}
	if held == nil {
		return nil, errors.New("the support folder's instance lock isn't held")
	}
	return openServeSocket(serveSocketPath())
}

// readServePeer reads the other end from the kernel: SO_PEERCRED, then that pid's /proc/<pid>/status as it is now. A
// variable so a test can change what was read.
var readServePeer = func(conn net.Conn) (credPeer, bool) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return credPeer{}, false
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return credPeer{}, false
	}
	var peer credPeer
	read := false
	err = raw.Control(func(fd uintptr) {
		credential, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			return
		}
		peer, read = credPeer{connectUID: credential.Uid, pid: credential.Pid}, true
	})
	if err != nil || !read {
		return credPeer{}, false
	}
	if peer.pid > 0 {
		if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", peer.pid)); err == nil {
			peer.now, _ = procUsersOf(string(status))
		}
	}
	return peer, true
}

// identifyServePeerHere reads the other end of a connection to serve.sock and judges it (peerowner.go).
func identifyServePeerHere(conn net.Conn) bool {
	peer, ok := readServePeer(conn)
	return ok && judgeCredPeer(peer)
}
