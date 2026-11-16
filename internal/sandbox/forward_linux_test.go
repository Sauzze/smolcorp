package sandbox

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// The real forwarder (SA-L6), as `enter` starts it: this test program becomes `smolcorp sandbox forward` (TestMain),
// listens on this computer's loopback (in a view, the view's own), says where, and hands what it gets on to the core's
// proxy, which refuses the computer's own address: the proxy row's check passes through it. It holds none of the
// starter's descriptors. Killed when the test ends.
func TestTheForwarderStartsAndHandsOnToTheProxy(t *testing.T) {
	t.Setenv(enterTestVar, "1")
	socket := serveProxy(t, &Proxy{own: ownAddrs})
	// A descriptor the starter holds, not marked to close at an exec: the forwarder must not get it.
	leak, err := os.Open(filepath.Dir(socket))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { leak.Close() })
	if _, err := unix.FcntlInt(leak.Fd(), unix.F_SETFD, 0); err != nil {
		t.Fatal(err)
	}
	at, p, err := realSystem{}.startForwarder(socket, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Kill(); p.Wait() })
	if host, _, err := net.SplitHostPort(at); err != nil || host != "127.0.0.1" {
		t.Fatalf("it listens at %q (%v)", at, err)
	}
	if err := wayOutHolds(at); err != nil {
		t.Errorf("the way out through the real forwarder: %v", err)
	}
	fds, _ := os.ReadDir("/proc/" + strconv.Itoa(p.Pid) + "/fd")
	for _, fd := range fds {
		if to, _ := os.Readlink("/proc/" + strconv.Itoa(p.Pid) + "/fd/" + fd.Name()); to == filepath.Dir(socket) {
			t.Errorf("the forwarder holds the starter's descriptor %s (%s)", fd.Name(), to)
		}
	}
	// One that can't listen says why, and isn't left running.
	if _, _, err := (realSystem{}).startForwarder(socket, at); err == nil || !strings.Contains(err.Error(), "no listener") {
		t.Errorf("a second forwarder at the same address: %v", err)
	}
}
