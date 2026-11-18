package tmuxfake

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The spike row on Linux (SA-L5), beside the Mac's LOCAL_PEERPID one: the guard reads SO_PEERCRED right after
// connecting, before anything is sent, and stops unless its pid is the server pid it was given. The same stand-in
// servers listen, then (a) exec into another program that accepts on the same socket, or (b) hand the socket to a
// child that accepts while the parent keeps running.
//
// What it shows: SO_PEERCRED names the process that called listen(), as it was then (unix(7)). Across an exec that's
// the same pid, so a hire's checked server that execs a fake server still passes the pid check: T5's worst case, as
// on a Mac, and why the guard's refusals, not the pid, are what make a client safe. A socket handed to another process
// still names the one that listened, unlike a Mac's LOCAL_PEERPID: a real tmux server listens itself, so the pid
// Smolcorp records is the right one, and a server that handed its socket on and ended leaves a pid that may be
// recycled (M7's recycling row, RT126SG2R2 N1). Both are inside the same sandbox.

// peerCred connects and reads SO_PEERCRED before sending anything, as the guard does.
func peerCred(t *testing.T, socket string) int {
	t.Helper()
	conn, err := net.DialTimeout("unix", socket, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := conn.(*net.UnixConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var cred *unix.Ucred
	var sockErr error
	raw.Control(func(fd uintptr) { cred, sockErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED) })
	if sockErr != nil {
		t.Fatal(sockErr)
	}
	if int(cred.Uid) != unix.Geteuid() {
		t.Fatalf("SO_PEERCRED says uid %d, not this user's %d", cred.Uid, unix.Geteuid())
	}
	return int(cred.Pid)
}

func TestSpikeSoPeercredAcrossExecAndAHandedOnSocket(t *testing.T) {
	dir := shortDir(t)

	socket := filepath.Join(dir, "exec")
	listened, accepting := startPeerPidHelper(t, "exec", socket)
	got := peerCred(t, socket)
	t.Logf("exec: listened in %d, accepts in %d (after exec), SO_PEERCRED pid %d", listened, accepting, got)
	if listened != accepting || got != listened {
		t.Errorf("across an exec the pid should stay the same and be the one reported: listened %d, accepts %d, SO_PEERCRED %d", listened, accepting, got)
	}

	socket = filepath.Join(dir, "child")
	listened, accepting = startPeerPidHelper(t, "child", socket)
	first := peerCred(t, socket)
	time.Sleep(100 * time.Millisecond)
	got = peerCred(t, socket)
	t.Logf("handed on: listened in %d, accepts in child %d, SO_PEERCRED pid %d first, then %d", listened, accepting, first, got)
	if listened == accepting || first != listened || got != listened {
		t.Errorf("a socket handed to a child should still name the process that listened: listened %d, child %d, SO_PEERCRED %d then %d", listened, accepting, first, got)
	}
}
