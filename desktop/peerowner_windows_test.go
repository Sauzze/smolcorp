//go:build windows

package desktop

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// OLD-SERVE2's proofs on Windows itself (CI's Windows runner): Windows' own tables, read as the serve port reads
// them, and the serve port end to end through the deck's own server, with this test program run again as a stand-in
// for the Tailscale service's process (only the service manager's answer is stood in: the process list and the TCP
// table are Windows' own). Nothing here starts or asks the real Tailscale.

// TestStandInPeerForOLDSERVE2 is a stand-in for the other end of a connection to the serve port, run as its own
// process by the tests below; it does nothing in a normal run. It waits for the word, connects, sends its request
// and, by its mode:
//   - stay: prints "sent", then reads the answer and prints its status, or "no answer";
//   - half: the same, half-closing after it sends, before it prints "sent";
//   - exit: ends at once (RSBXPLANf F2).
func TestStandInPeerForOLDSERVE2(t *testing.T) {
	address, mode := os.Getenv("SMOLCORP_TEST_OS2_PEER"), os.Getenv("SMOLCORP_TEST_OS2_MODE")
	if address == "" {
		return
	}
	if line, _ := bufio.NewReader(os.Stdin).ReadString('\n'); line != "go\n" {
		os.Exit(3)
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		os.Exit(4)
	}
	if _, err := io.WriteString(conn, os.Getenv("SMOLCORP_TEST_OS2_REQUEST")); err != nil {
		os.Exit(5)
	}
	switch mode {
	case "exit":
		os.Exit(0)
	case "half":
		if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
			os.Exit(10)
		}
	}
	fmt.Println("sent")
	printAnswer(conn)
}

// thisProgramFile is this test program's own file: absolute, as a service's configuration names its program.
func thisProgramFile(t *testing.T) string {
	t.Helper()
	file, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// The tables the serve port reads, read for real: this test's own connection is established and held by this
// process, which the process list names by its program and has started already; a running service's process and
// program come from the service manager, without a handle on that process; a service that isn't there is none.
// Once the client half-closes, its row is no longer established. IPv4 and IPv6 alike.
func TestWindowsSaysWhoHoldsAConnectionsOtherEnd(t *testing.T) {
	own := uint32(os.Getpid())
	image, created := listedProcess(own)
	if !strings.EqualFold(image, filepath.Base(thisProgramFile(t))) || created <= 0 || created > time.Now().UnixNano() {
		t.Errorf("this process in the list: %q started %d", image, created)
	}
	if image, created := listedProcess(0xFFFFFFF0); image != "" || created != 0 {
		t.Errorf("no such process: %q %d", image, created)
	}
	// The event log runs in a shared svchost as LocalService: its process is read without a handle, as tailscaled's
	// (SYSTEM) is.
	pid, binary := serviceManagerSays("EventLog")
	if pid == 0 || !strings.EqualFold(windowsProgramName(binary), "svchost.exe") {
		t.Errorf("the event log's service: %d %q", pid, binary)
	} else if image, created := listedProcess(pid); !strings.EqualFold(image, "svchost.exe") || created <= 0 {
		t.Errorf("the event log's process in the list: %q %d", image, created)
	}
	if pid, binary := serviceManagerSays("SmolcorpNoSuchService"); pid != 0 || binary != "" {
		t.Errorf("no such service: %d %q", pid, binary)
	}
	for _, host := range []string{"127.0.0.1", "::1"} {
		listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			if host == "::1" {
				t.Logf("no IPv6 on loopback here: %v", err)
				continue
			}
			t.Fatal(err)
		}
		defer listener.Close()
		client, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		server, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer server.Close()
		from, to := server.RemoteAddr().(*net.TCPAddr), server.LocalAddr().(*net.TCPAddr)
		if row := clientRow(from, to); row != (tcpRow{found: true, state: tcpEstablished, pid: own}) {
			t.Errorf("%s: the client's row: %+v", host, row)
		}
		// Asked for a connection nobody made, there is no row.
		if row := clientRow(&net.TCPAddr{IP: from.IP, Port: to.Port}, &net.TCPAddr{IP: to.IP, Port: 1}); row.found {
			t.Errorf("%s: a row for no connection: %+v", host, row)
		}
		if err := client.(*net.TCPConn).CloseWrite(); err != nil {
			t.Fatal(err)
		}
		row := clientRow(from, to)
		for deadline := time.Now().Add(5 * time.Second); row.state == tcpEstablished && time.Now().Before(deadline); {
			time.Sleep(20 * time.Millisecond)
			row = clientRow(from, to)
		}
		if !row.found || row.state == tcpEstablished {
			t.Errorf("%s: half-closed, the client's row: %+v", host, row)
		}
	}
}

// The serve port is held exclusively: while it's open, no other socket can bind it, not even one asking to share
// it (SO_REUSEADDR), which Windows otherwise allows a program of the same user.
func TestTheServePortIsHeldExclusively(t *testing.T) {
	sharing := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var err error
		if controlErr := raw.Control(func(fd uintptr) {
			err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
		}); controlErr != nil {
			return controlErr
		}
		return err
	}}
	held, err := listenExclusive("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if second, err := sharing.Listen(context.Background(), "tcp", held.Addr().String()); err == nil {
		second.Close()
		t.Error("a second socket asking to share it bound the serve port")
	}
	if second, err := net.Listen("tcp", held.Addr().String()); err == nil {
		second.Close()
		t.Error("a second socket bound the serve port")
	}
	// Without it, as the deck's own port is opened: said, so a reader sees what the option changes here.
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	second, err := sharing.Listen(context.Background(), "tcp", plain.Addr().String())
	if err == nil {
		second.Close()
	}
	t.Logf("a plain listener, asked to share by the same user: %v", err)
}

// The serve port is opened only where the hand-on is let in, at the port after the deck's, on this computer only,
// held exclusively, each connection stamped as it's accepted and judged before it's handed on: a connection from
// this test (no Tailscale service at all here) is closed unanswered.
func TestTheServePortIsMadeOnlyWhereTheHandOnIs(t *testing.T) {
	savedPort, savedWindows, savedService := Port, windowsTailscale, tailscaleService
	t.Cleanup(func() { Port, windowsTailscale, tailscaleService = savedPort, savedWindows, savedService })
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	Port = free.Addr().(*net.TCPAddr).Port - 1
	free.Close()
	tailscaleService = "SmolcorpNoSuchService"
	windowsTailscale = false
	if listener, err := listenServeSocket(); err != nil || listener != nil {
		t.Fatalf("the hand-on off: %v %v", listener, err)
	}
	windowsTailscale = true
	listener, err := listenServeSocket()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	served, ok := listener.(*serveListener)
	set, isSet := served.Listener.(*listenerSet)
	if !ok || !isSet || !set.stamp || set.listen == nil || set.port != servePort() {
		t.Fatalf("the serve port's listener: %#v", listener)
	}
	for _, host := range set.hosts() {
		if host != "127.0.0.1" && host != "::1" {
			t.Errorf("listening on %s", host)
		}
	}
	if second, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", servePort())); err == nil {
		second.Close()
		t.Error("the serve port bound twice")
	}
	accepted := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
		accepted <- err
	}()
	client, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", servePort()))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Error("a program here was answered on the serve port")
	}
	select {
	case err := <-accepted:
		t.Errorf("a program here was handed on: %v", err)
	default:
	}
	// A connection that wasn't stamped as it was accepted is never judged Tailscale's.
	if identifyServePeerHere(client) {
		t.Error("an unstamped connection was judged Tailscale's")
	}
}

// windowsServeRig is the serve port end to end: the deck's own server on a stamped, exclusive listener set as
// openServePort makes it, each connection judged by the real identifyServePeerHere (the real process list and TCP
// table) as it's taken, and every answer kept by its "who". Only the service manager is stood in: it names the
// stand-in peer's process, or whatever a row says.
type windowsServeRig struct {
	d        *Deck
	phoneKey string
	standIn  atomic.Uint32
	// says is what the stood-in service manager answers, by the time it's asked (0 first): the stand-in's pid by
	// default.
	says     func(asked int) uint32
	asked    atomic.Int32
	judged   chan bool
	mu       sync.Mutex
	answered map[string]int
}

func newWindowsServeRig(t *testing.T) *windowsServeRig {
	t.Helper()
	d, phoneKey := probeDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	servedUnrecorded = func(*http.Request) bool { return false }
	macTailscale, linuxTailscale, windowsTailscale = false, false, true
	rig := &windowsServeRig{d: d, phoneKey: phoneKey, judged: make(chan bool, 64), answered: map[string]int{}}
	program := thisProgramFile(t)
	savedIdentify, savedSays, savedImage := identifyServePeer, serviceManagerSays, tailscaledImage
	t.Cleanup(func() { identifyServePeer, serviceManagerSays, tailscaledImage = savedIdentify, savedSays, savedImage })
	tailscaledImage = filepath.Base(program)
	serviceManagerSays = func(name string) (uint32, string) {
		if name != tailscaleService {
			return 0, ""
		}
		asked := int(rig.asked.Add(1) - 1)
		pid := rig.standIn.Load()
		if rig.says != nil {
			pid = rig.says(asked)
		}
		return pid, `"` + program + `" -stand-in`
	}
	identifyServePeer = func(conn net.Conn) bool {
		ok := identifyServePeerHere(conn)
		rig.judged <- ok
		return ok
	}
	return rig
}

// serveHeld runs the deck's server on the serve port's listener at host (port 0), accepting nothing until let go.
func (rig *windowsServeRig) serveHeld(t *testing.T, host string) (string, chan struct{}) {
	t.Helper()
	set := newListenerSet(0)
	set.listen, set.stamp = listenExclusive, true
	if err := set.add(host); err != nil {
		set.Close()
		return "", nil
	}
	set.mu.Lock()
	address := set.open[host].Addr().String()
	set.mu.Unlock()
	letGo := make(chan struct{})
	server := newServer(rig.d)
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kept := &statusKept{ResponseWriter: w}
		inner.ServeHTTP(kept, r)
		rig.mu.Lock()
		rig.answered[r.URL.Query().Get("who")] = kept.status
		rig.mu.Unlock()
	})
	listener := &serveListener{Listener: heldListener{Listener: set, letGo: letGo}}
	go func() {
		_ = server.Serve(&cappedListener{Listener: listener, byAddress: map[string]int{}, thisComputer: true})
	}()
	t.Cleanup(func() { _ = server.Close(); _ = set.Close() })
	return address, letGo
}

// request is a phone's GET /approvals through `tailscale serve`, by its "who", to its ts.net name, for another device.
func (rig *windowsServeRig) request(who string) string {
	return strings.Join([]string{fmt.Sprintf("GET /approvals?session=nobody&who=%s&t=%s HTTP/1.1", who, rig.phoneKey),
		"Host: my-mac.tail1234.ts.net", "X-Forwarded-For: 100.101.102.103", "Connection: close", "", ""}, "\r\n")
}

// standInPeer runs a stand-in peer of a mode to address, the server held until it has sent (stay; half, its sending
// side closed too) or sent and ended (exit), and says what it printed last ("" for exit, whose judgement is checked
// here).
func (rig *windowsServeRig) standInPeer(t *testing.T, address string, letGo chan struct{}, mode, who string) string {
	t.Helper()
	peer := startStandInRun(t, "TestStandInPeerForOLDSERVE2", "SMOLCORP_TEST_OS2_PEER="+address,
		"SMOLCORP_TEST_OS2_MODE="+mode, "SMOLCORP_TEST_OS2_REQUEST="+rig.request(who))
	rig.standIn.Store(uint32(peer.cmd.Process.Pid))
	rig.asked.Store(0)
	if _, err := io.WriteString(peer.stdin, "go\n"); err != nil {
		t.Fatal(err)
	}
	if mode == "exit" {
		if err := peer.cmd.Wait(); err != nil {
			t.Fatal(err)
		}
		for len(rig.judged) > 0 {
			<-rig.judged
		}
		close(letGo)
		select {
		case ok := <-rig.judged:
			if ok {
				t.Errorf("%s: a client that ended before the accept was judged Tailscale's", who)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("%s: never judged", who)
		}
		return ""
	}
	if got := peer.said(t); got != "sent" {
		t.Fatalf("%s: the stand-in said %q", who, got)
	}
	close(letGo)
	return peer.said(t)
}

func (rig *windowsServeRig) saidBy(who string) (int, bool) {
	rig.mu.Lock()
	defer rig.mu.Unlock()
	status, ok := rig.answered[who]
	return status, ok
}

// OLD-SERVE2 end to end on Windows. The stand-in, named by the stood-in service manager and running this test's own
// program (taken for tailscaled.exe), is let in through the serve port, over IPv4 and IPv6. Refused, closed
// unanswered: the same stand-in while the service manager names another process (this test's own, a program here
// running the same program), or names it before the TCP table is read and another after (a pid given again); a
// client that sent and ended before the accept (RSBXPLANf F2), or half-closed; anyone with no Tailscale service at
// all. Over the deck's own port the same stand-in, with the same request, is refused at the door: only the serve
// port counts.
func TestOnlyTheTailscaleServiceIsLetInThroughTheServePort(t *testing.T) {
	rig := newWindowsServeRig(t)
	address, letGo := rig.serveHeld(t, "127.0.0.1")
	if got := rig.standInPeer(t, address, letGo, "stay", "serve"); got != "200" {
		t.Fatalf("tailscale serve's hand-on through the serve port, from the service's own process: %s, want 200", got)
	}
	if address, letGo := rig.serveHeld(t, "::1"); address != "" {
		if got := rig.standInPeer(t, address, letGo, "stay", "serve6"); got != "200" {
			t.Errorf("the same over IPv6: %s, want 200", got)
		}
	} else {
		t.Log("no IPv6 on loopback here")
	}
	own := uint32(os.Getpid())
	// first is the stand-in when the service manager is first asked, then another.
	first := func(then uint32) func(int) uint32 {
		return func(asked int) uint32 {
			if asked == 0 {
				return rig.standIn.Load()
			}
			return then
		}
	}
	for who, says := range map[string]func(int) uint32{
		"a-program-here":     func(int) uint32 { return own },
		"pid-given-again":    first(own),
		"no-service":         func(int) uint32 { return 0 },
		"service-gone-after": first(0),
	} {
		rig.says = says
		address, letGo := rig.serveHeld(t, "127.0.0.1")
		if got := rig.standInPeer(t, address, letGo, "stay", who); got != "no answer" {
			t.Errorf("%s: %s, want no answer", who, got)
		}
	}
	rig.says = nil
	address, letGo = rig.serveHeld(t, "127.0.0.1")
	rig.standInPeer(t, address, letGo, "exit", "exited")
	address, letGo = rig.serveHeld(t, "127.0.0.1")
	if got := rig.standInPeer(t, address, letGo, "half", "half"); got != "no answer" {
		t.Errorf("a client that half-closed after sending: %s, want no answer", got)
	}
	for _, who := range []string{"a-program-here", "pid-given-again", "no-service", "service-gone-after", "exited", "half"} {
		if status, ok := rig.saidBy(who); ok {
			t.Errorf("%s was answered: %d", who, status)
		}
	}
	// The deck's own port, as the deck listens there: nothing over it is `tailscale serve`'s.
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	letGo = make(chan struct{})
	server := newServer(rig.d)
	go func() {
		_ = server.Serve(&cappedListener{Listener: heldListener{Listener: tcp, letGo: letGo}, byAddress: map[string]int{}})
	}()
	t.Cleanup(func() { _ = server.Close() })
	if got := rig.standInPeer(t, tcp.Addr().String(), letGo, "stay", "deck-port"); got != "403" {
		t.Errorf("the same over the deck's own port: %s, want 403", got)
	}
}
