package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

func init() {
	// abstractprobe <name>: connect to the abstract unix socket name, and say how it went ("connected", or the errno).
	moreStandIns["abstractprobe"] = func(args []string) int {
		c, err := net.Dial("unix", args[0])
		if err != nil {
			fmt.Println(errnoWord(err))
			return 0
		}
		c.Close()
		fmt.Println("connected")
		return 0
	}
}

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
	at, p, err := realSystem{}.startForwarder(socket, "127.0.0.1:0", "")
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
	if _, _, err := (realSystem{}).startForwarder(socket, at, ""); err == nil || !strings.Contains(err.Error(), "no listener") {
		t.Errorf("a second forwarder at the same address: %v", err)
	}
}

// The real forwarder with the app's port (SA-L7): it listens there too, says both addresses, and each connection there
// is a CONNECT to the app's port through the core's proxy, which reaches the app. (On one network, as here, the app's
// own address is the forwarder's: the stand-in proxy's dial goes to a stand-in app instead, so nothing loops.)
func TestTheForwarderForwardsTheAppsPortThroughTheProxy(t *testing.T) {
	t.Setenv(enterTestVar, "1")
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "the app: "+r.URL.Path)
	}))
	t.Cleanup(app.Close)
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	appAt := free.Addr().String()
	free.Close()
	appPort := free.Addr().(*net.TCPAddr).Port
	d := &dialLog{}
	socket := serveProxy(t, &Proxy{AppPort: appPort, own: ownAddrs, dial: d.dialTo(strings.TrimPrefix(app.URL, "http://"))})
	at, p, err := realSystem{}.startForwarder(socket, "127.0.0.1:0", appAt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Kill(); p.Wait() })
	if err := wayOutHolds(at); err != nil {
		t.Errorf("the way out: %v", err)
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	resp, err := client.Get("http://" + appAt + "/hook")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "the app: /hook" {
		t.Errorf("the app's port in the view: %q", body)
	}
	if dials := d.all(); len(dials) != 1 || dials[0] != netip.MustParseAddrPort(appAt) {
		t.Errorf("the proxy dialled %v, not the app's port", dials)
	}
}

// An abstract unix socket of the computer's (X11's @/tmp/.X11-unix/X0, an old D-Bus address) is never reached through
// the way out: `sandbox connect` doesn't ask for it, and the proxy dials TCP alone. From a view with a network of its
// own (bwrap's --unshare-net, as every desk's view has: bwrap.go), it isn't there at all; outside, the stand-in answers.
func TestAnAbstractSocketIsNeverReached(t *testing.T) {
	word := make([]byte, 6)
	rand.Read(word)
	name := "@smolcorp-standin-" + hex.EncodeToString(word)
	l, err := net.Listen("unix", name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	var got atomic.Int64
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			got.Add(1)
			c.Close()
		}
	}()
	d := &dialLog{}
	socket := serveProxy(t, &Proxy{own: ownAddrs, dial: d.dialTo("127.0.0.1:1"), Log: t.Logf,
		resolve: func(context.Context, string) ([]netip.Addr, error) { return nil, fmt.Errorf("no such stand-in name") }})
	if code := connectThrough(forwarder(t, socket), []string{name, "22"}, strings.NewReader(""), io.Discard, io.Discard); code != 2 {
		t.Errorf("sandbox connect %s: exit %d", name, code)
	}
	for _, head := range []string{"CONNECT " + name + ":22 HTTP/1.1\r\nHost: x\r\n\r\n", "GET http://" + name + ":22/ HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT unix:" + name + " HTTP/1.1\r\nHost: x\r\n\r\n"} {
		if status, _, _ := ask(t, socket, head); status == http.StatusOK {
			t.Errorf("%q: 200", head)
		}
	}
	if dials := d.all(); len(dials) != 0 {
		t.Errorf("the proxy dialled %v", dials)
	}
	if n := got.Load(); n != 0 {
		t.Fatalf("the abstract stand-in was reached %d times through the way out", n)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	probe := func(words ...string) string {
		cmd := disclaim.Cmd(exec.Command(words[0], words[1:]...))
		cmd.Env = append(os.Environ(), enterTestVar+"=1")
		out, err := cmd.CombinedOutput()
		t.Logf("%q: %v %s", words, err, out)
		return strings.TrimSpace(string(out))
	}
	if said := probe(self, "abstractprobe", name); said != "connected" || got.Load() != 1 {
		t.Fatalf("outside a view, the stand-in didn't answer (the probe can't tell): %q", said)
	}
	if _, err := os.Stat(BwrapPath); err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	said := probe(BwrapPath, "--unshare-user", "--unshare-net", "--ro-bind", "/", "/", "--dev", "/dev", "--", self, "abstractprobe", name)
	if said != "ECONNREFUSED" || got.Load() != 1 {
		t.Errorf("from a view with its own network, the abstract stand-in: %q (reached %d times in all)", said, got.Load())
	}
}
