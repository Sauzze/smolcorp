package desktop

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"golang.org/x/sys/unix"
)

// Round 3 on RT126SWEEP3 part 4's F1 (FXSWEEP3P4R3, the round-2 review's B2): a desk's process held inside exit()
// never runs its own code again, so it's read as holding nothing. It neither flags the owner's asks "may be a hire"
// nor gets a closed ask taken.

// inExitPeerVariable makes this test program the stand-in desk process of deskHeldInExit, through
// TestADeskProcessHeldInExitForARow: the address it connects to.
const inExitPeerVariable = "SMOLCORP_TEST_INEXIT_PEER"

// TestADeskProcessHeldInExitForARow is no row of its own: run as deskHeldInExit's child, it connects to the row's
// peer, which doesn't read, fills its side until nothing more can be sent, asks for a blocking close that waits up to
// 25 seconds for the rest to go, and exits. Closing its files in exit() then holds it there.
func TestADeskProcessHeldInExitForARow(t *testing.T) {
	peer := os.Getenv(inExitPeerVariable)
	if peer == "" {
		return
	}
	conn, err := net.Dial("tcp", peer)
	if err != nil {
		os.Exit(3)
	}
	tcp := conn.(*net.TCPConn)
	_ = tcp.SetWriteBuffer(64 << 10)
	_ = tcp.SetWriteDeadline(time.Now().Add(1500 * time.Millisecond))
	for chunk := make([]byte, 1<<20); ; {
		if _, err := tcp.Write(chunk); err != nil {
			break
		}
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		os.Exit(3)
	}
	_ = raw.Control(func(fd uintptr) {
		_ = unix.SetsockoptLinger(int(fd), unix.SOL_SOCKET, unix.SO_LINGER_SEC, &unix.Linger{Onoff: 1, Linger: 25})
		_ = unix.SetNonblock(int(fd), false)
	})
	os.Exit(0)
}

// inExit is whether the kernel's process table has pid inside exit() and not yet a zombie (P_WEXIT, <sys/proc.h>).
func inExit(pid int) bool {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && int(k.Proc.P_pid) == pid && k.Proc.P_flag&0x2000 != 0 && k.Proc.P_stat != 5 // SZOMB
}

// deskHeldInExit starts a stand-in desk process under the hire's real profile, in its own group, and waits until it's
// held inside exit(): its pid. When the test ends, its peer is reset (which lets the exit end) and its group killed,
// and ps must then find nothing.
func deskHeldInExit(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		if conn, err := listener.Accept(); err == nil {
			_ = conn.(*net.TCPConn).SetReadBuffer(64 << 10)
			accepted <- conn
		}
	}()
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, os.Args[0], "-test.run=^TestADeskProcessHeldInExitForARow$", "-test.count=1"))
	cmd.Dir, cmd.Env = l.hireTree, append(gmRootEnv(), inExitPeerVariable+"="+listener.Addr().String())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	var peer net.Conn
	t.Cleanup(func() {
		if peer != nil {
			_ = peer.(*net.TCPConn).SetLinger(0)
			peer.Close()
		}
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = cmd.Wait()
		if left, _ := exec.Command("/bin/ps", "-o", "pid=,stat=,comm=", "-p", strconv.Itoa(pid)).Output(); strings.TrimSpace(string(left)) != "" {
			t.Errorf("the stand-in desk is still there: %q", left)
		}
	})
	select {
	case peer = <-accepted:
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in desk never connected")
	}
	for deadline := time.Now().Add(10 * time.Second); !inExit(pid); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			state, _ := exec.Command("/bin/ps", "-o", "stat=,comm=", "-p", strconv.Itoa(pid)).Output()
			t.Fatalf("the stand-in desk %d isn't held in exit(): %q", pid, state)
		}
	}
	return pid
}

// B2: beside a desk's process held inside exit(), each of the owner's own asks is still read as outside any sandbox:
// taken, and listed with no "may be a hire", 10 of 10. Asks closed at once are still refused, in plain words.
func TestOnAMacADeskProcessHeldInExitLeavesTheOwnersAsksOutside(t *testing.T) {
	pid := deskHeldInExit(t)
	rig := newAskRig(t)
	for i := range 10 {
		code, said := rig.ownersAsk(t, "/work/mine")
		if code != http.StatusOK || said["ticket"] == nil {
			t.Fatalf("the owner's ask %d: %d %v", i, code, said)
		}
		waiting := rig.d.asks.waiting()
		if len(waiting) != 1 || waiting[0]["folder"] != "/work/mine" || waiting[0]["mayBeAHire"] != nil {
			t.Errorf("the owner's ask %d, beside a desk's process in exit(): the window lists %v", i, waiting)
		}
		// Answered no, and the answer taken, so the next ask is the only one waiting.
		for _, shown := range waiting {
			id, _ := shown["id"].(string)
			shownCode, _ := shown["code"].(string)
			if ask := rig.d.asks.claim(id, shownCode); ask != nil {
				rig.d.asks.settle(ask, "deny", "")
			}
		}
		ticket, _ := said["ticket"].(string)
		rig.d.asks.take(ticket)
	}
	for i := range 10 {
		if answer := rig.closedAsk(t, "/work/flood"+strconv.Itoa(i)); answer.Code != http.StatusForbidden || strings.TrimSpace(answer.Body.String()) != sweepNoAskerWords {
			t.Errorf("closed ask %d: %d %q", i, answer.Code, answer.Body.String())
		}
	}
	if waiting := rig.d.asks.waiting(); len(waiting) != 0 {
		t.Errorf("after the closed asks, the window lists %v", waiting)
	}
	if !inExit(pid) {
		t.Errorf("the stand-in desk %d left exit() before the asks were done", pid)
	}
}
