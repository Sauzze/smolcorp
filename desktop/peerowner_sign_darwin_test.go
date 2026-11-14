//go:build darwin && cgo

package desktop

import (
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// thisTestsCode is a code requirement this test's binary meets, and a stand-in peer run from it: the Go linker signs
// it ad hoc, named a.out. /bin/sleep (com.apple.sleep) doesn't meet it.
const thisTestsCode = `identifier "a.out"`

// The real signature check, by audit token: macOS takes its requirement, and refuses this test (signed ad hoc),
// an Apple program that isn't Tailscale's (/bin/sleep, its socket kept across exec) and a token that names no
// process. The same token check does see /bin/sleep for Apple's, so a no isn't only a check that can't run.
func TestTheRealCheckTakesOnlyTailscalesSignature(t *testing.T) {
	if !tailscaleRequirementParses() {
		t.Fatal("macOS doesn't take the requirement: every process would be refused for that alone")
	}
	// This test's own process, at the other end of its own connection.
	path, accepted := acceptOne(t)
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	self, ok := readServePeer(<-accepted)
	if !ok || self.token.pid() != int32(os.Getpid()) || self.connectUID != uint32(os.Getuid()) || self.token.realUID() != uint32(os.Getuid()) {
		t.Fatalf("this test's end: %+v %v", self, ok)
	}
	if !tokenMeets(self.token, thisTestsCode, "") {
		t.Fatal("the token check doesn't see this test as itself")
	}
	if tailscaleSignedHere(self.token) {
		t.Error("this test was taken for Tailscale's")
	}
	// /bin/sleep, run by a stand-in that connected first.
	path, accepted = acceptOne(t)
	sleeper := startStandIn(t, "unix", path, "sleep", "")
	if got := sleeper.said(t); got != "connected" {
		t.Fatalf("the stand-in said %q", got)
	}
	conn := <-accepted
	if !runsNow(sleeper.cmd.Process.Pid, "sleep", 10*time.Second) {
		t.Fatal("the stand-in never ran /bin/sleep")
	}
	sleep, ok := readServePeer(conn)
	if !ok || sleep.token.pid() != int32(sleeper.cmd.Process.Pid) {
		t.Fatalf("/bin/sleep's end: %+v %v", sleep, ok)
	}
	if !tokenMeets(sleep.token, "anchor apple", "") || tokenMeets(sleep.token, thisTestsCode, "") {
		t.Error("the token check doesn't see /bin/sleep for Apple's")
	}
	if tailscaleSignedHere(sleep.token) {
		t.Error("/bin/sleep, signed by Apple, was taken for Tailscale's")
	}
	if tailscaleSignedHere(auditToken{}) || tokenMeets(auditToken{}, "anchor apple", "") {
		t.Error("a token of nothing was taken for a process")
	}
}

// A token names one run of one program (RT126SN1 B1): a stand-in that connects and then runs this same program
// again, its socket kept, is still met by its own token as read now, but the token read before it ran it again names
// nothing, and the user it connected as is kept. So a process that execs between the read and the signature check is
// refused, and the token is never taken for who connected.
func TestATokenIsTheProcessAsItRunsNow(t *testing.T) {
	path, accepted := acceptOne(t)
	peer := startStandIn(t, "unix", path, "hold", "")
	if got := peer.said(t); got != "connected" {
		t.Fatalf("the stand-in said %q", got)
	}
	conn := <-accepted
	before, ok := readServePeer(conn)
	if !ok || before.token.pid() != int32(peer.cmd.Process.Pid) || !tokenMeets(before.token, thisTestsCode, "") {
		t.Fatalf("before: %+v %v", before, ok)
	}
	if _, err := io.WriteString(peer.stdin, "exec\n"); err != nil {
		t.Fatal(err)
	}
	if got := peer.said(t); got != "execed" {
		t.Fatalf("the stand-in said %q", got)
	}
	if tokenMeets(before.token, thisTestsCode, "") {
		t.Error("the token read before the exec still names a process")
	}
	after, ok := readServePeer(conn)
	if !ok || after.token.pid() != before.token.pid() || after.token == before.token {
		t.Fatalf("after: %+v %v (before %+v)", after, ok, before)
	}
	if !tokenMeets(after.token, thisTestsCode, "") {
		t.Error("the token read after the exec doesn't name the program it runs now")
	}
	if after.connectUID != before.connectUID {
		t.Errorf("the user it connected as changed: %d, then %d", before.connectUID, after.connectUID)
	}
}

// SN1 end to end on this Mac, through the deck's own server, a real serve.sock and the kernel's real answers, with
// stand-in peers (serveSockRig): only the stand-in the check takes for Tailscale's is (this test's binary, by its
// signature, at that pid), with this test's user standing in for root. Its hand-on through serve.sock is let in, and
// the rows a Mac and Linux share refuse the rest (sharedRows). With root wanted, as in the app: a client that sends
// and then execs the "signed" image is refused for the user it connected as alone (RT126SN1 B1), and so are the
// stand-in serve itself (run as the owner, like the Mac App Store build) and a client that half-closes (ownRows).
func TestOnlyTheRealServeIsLetInThroughServeSock(t *testing.T) {
	rig := newServeSockRig(t)
	savedSigned, savedUID, savedRead := tailscaleSigned, tailscaleUID, readServePeer
	t.Cleanup(func() { tailscaleSigned, tailscaleUID, readServePeer = savedSigned, savedUID, savedRead })
	macTailscale = true
	// Root wanted, as in the app. The stand-ins are set once, before any server runs, and steered by standing: the
	// stand-in peers run as this test's user, so it stands in for root (everyUser), until the rows that want root
	// itself; there, only the stand-in's token is stood in as root's (itsToken), then nothing (asItIs).
	const everyUser, itsToken, asItIs = 0, 1, 2
	var standing atomic.Int32
	own := uint32(os.Getuid())
	tailscaleUID = 0
	readServePeer = func(conn net.Conn) (servePeer, bool) {
		peer, ok := savedRead(conn)
		asRoot := func(uid *uint32) {
			if *uid == own {
				*uid = 0
			}
		}
		switch standing.Load() {
		case everyUser:
			asRoot(&peer.connectUID)
			asRoot(&peer.token[1])
			asRoot(&peer.token[3])
		case itsToken:
			if peer.token.pid() == rig.standIn.Load() {
				asRoot(&peer.token[1])
				asRoot(&peer.token[3])
			}
		}
		return peer, ok
	}
	tailscaleSigned = func(token auditToken) bool {
		return token.pid() == rig.standIn.Load() && tokenMeets(token, thisTestsCode, "")
	}
	rig.sharedRows(t)
	rig.ownRows(t, func() { standing.Store(everyUser) }, func() { standing.Store(itsToken) }, func() { standing.Store(asItIs) })
}

// runsNow waits until the process pid runs the program name, and says whether it did in time.
func runsNow(pid int, name string, wait time.Duration) bool {
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
		if err == nil && unix.ByteSliceToString(info.Proc.P_comm[:]) == name {
			return true
		}
	}
	return false
}
