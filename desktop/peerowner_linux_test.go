package desktop

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
)

// The kernel says who connected to serve.sock, and keeps it (OLD-SERVE1): this test's own end is this process and
// this user, running. A stand-in that connects and then runs the program again, its socket kept, is still the same
// pid, connected as the same user. Once it has ended, SO_PEERCRED still names who connected (the kernel keeps it after
// the process ends, so the user alone can't refuse an ended client), but /proc says it is no process now, and the
// judge refuses it for that.
func TestTheKernelKeepsWhoConnectedToServeSock(t *testing.T) {
	path, accepted := acceptOne(t)
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	own := uint32(os.Getuid())
	self, ok := readServePeer(<-accepted)
	if !ok || self.pid != int32(os.Getpid()) || self.connectUID != own || self.now != (procUsers{own, own, true}) {
		t.Fatalf("this test's end: %+v %v", self, ok)
	}
	path, accepted = acceptOne(t)
	peer := startStandIn(t, "unix", path, "hold", "")
	if got := peer.said(t); got != "connected" {
		t.Fatalf("the stand-in said %q", got)
	}
	conn := <-accepted
	before, ok := readServePeer(conn)
	if !ok || before.pid != int32(peer.cmd.Process.Pid) || before.connectUID != own || !before.now.running {
		t.Fatalf("before: %+v %v", before, ok)
	}
	if _, err := io.WriteString(peer.stdin, "exec\n"); err != nil {
		t.Fatal(err)
	}
	if got := peer.said(t); got != "execed" {
		t.Fatalf("the stand-in said %q", got)
	}
	after, ok := readServePeer(conn)
	if !ok || after != before {
		t.Fatalf("after the exec: %+v %v (before %+v)", after, ok, before)
	}
	// Its input ends, so it ends; reaped, it is no process.
	_ = peer.stdin.Close()
	_ = peer.cmd.Wait()
	ended, ok := readServePeer(conn)
	if !ok || ended.pid != before.pid || ended.connectUID != own || ended.now.running {
		t.Fatalf("ended: %+v %v", ended, ok)
	}
	saved := tailscaleUID
	t.Cleanup(func() { tailscaleUID = saved })
	tailscaleUID = own
	if judgeCredPeer(ended) {
		t.Error("an ended client was judged tailscaled, with this user standing in for root")
	}
	if !judgeCredPeer(self) {
		t.Error("this test's own end wasn't judged tailscaled, with this user standing in for root: the row proves nothing")
	}
}

// OLD-SERVE1 end to end on Linux, through the deck's own server, a real serve.sock and the kernel's real answers
// (SO_PEERCRED, /proc), with SN1's stand-in peers (serveSockRig). Only the stand-in's pid is read as root (this user
// standing in for root there alone), so this test's own process stays the owner's: the stand-in's hand-on through
// serve.sock is let in, and the rows a Mac and Linux share refuse the rest, the TCP loopback and this computer's own
// address among them (sharedRows). Then: a client that sends and then execs is refused for the user it connected as
// alone, once only its process as it runs now is read as root; the stand-in as it really is (the owner's) is refused,
// whole or half-closed (ownRows).
func TestOnlyTheRealServeIsLetInThroughServeSockOnLinux(t *testing.T) {
	rig := newServeSockRig(t)
	savedUID, savedRead := tailscaleUID, readServePeer
	t.Cleanup(func() { tailscaleUID, readServePeer = savedUID, savedRead })
	linuxTailscale = true
	const standInIsRoot, itsProcess, asItIs = 0, 1, 2
	var standing atomic.Int32
	own := uint32(os.Getuid())
	tailscaleUID = 0
	readServePeer = func(conn net.Conn) (credPeer, bool) {
		peer, ok := savedRead(conn)
		asRoot := func(uid *uint32) {
			if *uid == own {
				*uid = 0
			}
		}
		if peer.pid != rig.standIn.Load() {
			return peer, ok
		}
		switch standing.Load() {
		case standInIsRoot:
			asRoot(&peer.connectUID)
			asRoot(&peer.now.realUID)
			asRoot(&peer.now.effectiveUID)
		case itsProcess:
			asRoot(&peer.now.realUID)
			asRoot(&peer.now.effectiveUID)
		}
		return peer, ok
	}
	rig.sharedRows(t)
	rig.ownRows(t, func() { standing.Store(standInIsRoot) }, func() { standing.Store(itsProcess) }, func() { standing.Store(asItIs) })
}

// On Linux serve.sock is made only under the support folder's instance lock, held by this process: without it there
// is none. With the hand-on off (as on Windows), there is none at all.
func TestServeSockIsMadeOnlyUnderTheInstanceLockOnLinux(t *testing.T) {
	savedInstance, savedHeld, savedLinux := instanceFile, held, linuxTailscale
	t.Cleanup(func() { instanceFile, held, linuxTailscale = savedInstance, savedHeld, savedLinux })
	dir := shortTempDir(t)
	lock := filepath.Join(dir, "instance.lock")
	instanceFile, held, linuxTailscale = func() string { return lock }, nil, true
	if listener, err := listenServeSocket(); err == nil || listener != nil {
		t.Fatalf("no instance lock: %v %v", listener, err)
	}
	if _, err := os.Lstat(serveSocketPath()); err == nil {
		t.Fatal("serve.sock was made without the instance lock")
	}
	if running, err := acquireInstance(); running || err != nil || held == nil {
		t.Fatalf("the lock: %v %v", running, err)
	}
	defer func() { _ = syscall.Flock(int(held.Fd()), syscall.LOCK_UN); held.Close() }()
	listener, err := listenServeSocket()
	if err != nil || listener == nil {
		t.Fatalf("under the lock: %v", err)
	}
	info, err := os.Lstat(serveSocketPath())
	if err != nil || info.Mode().Type() != os.ModeSocket || info.Mode().Perm() != 0o600 {
		t.Errorf("serve.sock: %v %v, want a socket, 0600", info, err)
	}
	listener.Close()
	linuxTailscale = false
	if listener, err := listenServeSocket(); err != nil || listener != nil {
		t.Errorf("the hand-on off: %v %v", listener, err)
	}
}
