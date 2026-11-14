//go:build darwin

package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// On a Mac, `tailscale serve` hands a phone's request on to serve.sock, a unix socket in the support folder
// (peerowner.go), and the kernel says who is at the other end of each connection to it. It must have connected as
// root (LOCAL_PEERCRED: the user it connected as, set at connect and never changed by exec), run as root now (its real
// and effective user, from its audit token, LOCAL_PEERTOKEN), and be Tailscale's network extension, its signature
// checked by that same token (tailscaleSigned). A client that sent its request and ended has no token any more:
// refused.
//
// Root counts because what a process runs can change and who it is can't (RT126SN1 B1). The token is read as the
// process is now, not as it was at connect: a program that connects, sends and then execs a program Tailscale signed
// gets a new token, which passes the signature check. But it connected as the owner, and the kernel keeps that, so it
// is refused. A token also names one run of one program, so a process that execs between the read and the signature
// check names nothing and is refused too. Tailscale's standalone extension runs as root. The Mac App Store version's
// runs as the owner, like any program of theirs, so it is refused: Smolcorp can't tell it apart from one (setup and the
// release notes say so).

// sunPathMost is the longest path a unix socket takes on a Mac: sun_path is 104 bytes, its closing NUL among them.
const sunPathMost = 103

// listenServeSocket makes serve.sock again, as the app starts: under the support folder's instance lock (held by
// this process, or by the Mac app that started this engine), so no second Smolcorp takes it over.
func listenServeSocket() (net.Listener, error) {
	if !serveHandOnHere() {
		return nil, nil
	}
	if held == nil {
		if ok, err := appHoldsInstance(); !ok {
			return nil, fmt.Errorf("the support folder's instance lock isn't held: %v", err)
		}
	}
	return openServeSocket(serveSocketPath())
}

// openServeSocket removes whatever is at path and listens there, for the owner alone (0600), with each connection
// identified as it's accepted (serveListener). A path too long for a socket is refused rather than cut short.
func openServeSocket(path string) (net.Listener, error) {
	if len(path) > sunPathMost {
		return nil, fmt.Errorf("%s is too long for a socket (%d bytes, at most %d)", path, len(path), sunPathMost)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return &serveListener{Listener: listener}, nil
}

// auditToken is a process's audit token as the kernel gives it (bsm's audit_token_t): eight words.
type auditToken [8]uint32

func (t auditToken) effectiveUID() uint32 { return t[1] }
func (t auditToken) realUID() uint32      { return t[3] }
func (t auditToken) pid() int32           { return int32(t[5]) }

// servePeer is what is read of the other end of a connection to serve.sock, as it's accepted.
type servePeer struct {
	// connectUID is the user the other end connected as (LOCAL_PEERCRED's effective user), kept across exec.
	connectUID uint32
	// token is the other end's audit token as it is now: who it runs as, and the run of the program it is.
	token auditToken
}

// xucredVersion is the only layout of LOCAL_PEERCRED's answer (sys/ucred.h's XUCRED_VERSION).
const xucredVersion = 0

// readServePeer reads both from the kernel. A variable so a test can change what was read.
var readServePeer = func(conn net.Conn) (servePeer, bool) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return servePeer{}, false
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return servePeer{}, false
	}
	var peer servePeer
	read := false
	err = raw.Control(func(fd uintptr) {
		credential, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil || credential.Version != xucredVersion {
			return
		}
		token, ok := peerTokenHere(int(fd))
		if !ok {
			return
		}
		peer, read = servePeer{connectUID: credential.Uid, token: token}, true
	})
	return peer, err == nil && read
}

// tailscaleUID is the user Tailscale's extension runs as: root. A variable so a test's stand-in peer, which runs as
// this test's user, can be taken for it.
var tailscaleUID uint32 = 0

// tailscaleSigned says whether the process an audit token names is Tailscale's network extension
// (tailscaleSignedHere). A variable so a test can stand a peer in.
var tailscaleSigned = tailscaleSignedHere

// judgeServePeer says whether the other end is Tailscale's: connected as root, running as root (its real and its
// effective user), and Tailscale's extension by its signature. The users come first: they cost nothing, and a
// process of the owner's needs no signature check.
func judgeServePeer(peer servePeer) bool {
	if peer.connectUID != tailscaleUID || peer.token.realUID() != tailscaleUID || peer.token.effectiveUID() != tailscaleUID || peer.token.pid() <= 0 {
		return false
	}
	return tailscaleSigned(peer.token)
}

// identifyServePeerHere reads the other end of a connection to serve.sock and judges it (peerowner.go).
func identifyServePeerHere(conn net.Conn) bool {
	peer, ok := readServePeer(conn)
	return ok && judgeServePeer(peer)
}
