//go:build (darwin && cgo) || linux

package desktop

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// SN1's stand-in peers and its end-to-end rig, on a Mac (SN1) and on Linux (OLD-SERVE1): the same processes and the
// same rows, judged by each system's own kernel. Only what each system's test stands in for Tailscale differs.

// TestStandInPeerForSN1 is a stand-in for the other end of a connection to the deck, run as its own process by the
// tests below; it does nothing in a normal run. By its mode it:
//   - stay: waits for the word, connects (to serve.sock, or over TCP), sends its request, prints "sent", reads the
//     answer and prints its status, or "no answer";
//   - half: the same, half-closing after it sends, before it prints "sent";
//   - exit: sends, and ends at once (RSBXPLANf F2);
//   - exec: sends, then runs this same program again, its socket kept (RT126SN1 B1, into an image the stand-in check
//     takes for Tailscale's); that run prints "execed", then reads the answer as stay does;
//   - hold: connects, prints "connected", and on the word "exec" runs this same program again, its socket kept; that
//     run prints "execed" and waits for its input to end;
//   - sleep: connects, prints "connected", and runs /bin/sleep, its socket kept.
func TestStandInPeerForSN1(t *testing.T) {
	address, mode := os.Getenv("SMOLCORP_TEST_SN1_PEER"), os.Getenv("SMOLCORP_TEST_SN1_MODE")
	if address == "" {
		return
	}
	input := bufio.NewReader(os.Stdin)
	if mode == "execed" || mode == "held" {
		fd, _ := strconv.Atoi(os.Getenv("SMOLCORP_TEST_SN1_FD"))
		conn, err := net.FileConn(os.NewFile(uintptr(fd), "peer"))
		if err != nil {
			os.Exit(9)
		}
		fmt.Println("execed")
		if mode == "held" {
			_, _ = io.Copy(io.Discard, input)
			os.Exit(0)
		}
		printAnswer(conn)
	}
	network := os.Getenv("SMOLCORP_TEST_SN1_NET")
	if mode == "hold" || mode == "sleep" {
		conn, err := net.Dial(network, address)
		if err != nil {
			os.Exit(4)
		}
		fmt.Println("connected")
		if mode == "hold" {
			if line, _ := input.ReadString('\n'); line != "exec\n" {
				os.Exit(3)
			}
		}
		execKeeping(conn, mode)
	}
	if line, _ := input.ReadString('\n'); line != "go\n" {
		os.Exit(3)
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		os.Exit(4)
	}
	if _, err := io.WriteString(conn, os.Getenv("SMOLCORP_TEST_SN1_REQUEST")); err != nil {
		os.Exit(5)
	}
	switch mode {
	case "exit":
		os.Exit(0)
	case "half":
		if err := conn.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
			os.Exit(10)
		}
	case "exec":
		execKeeping(conn, mode)
	}
	fmt.Println("sent")
	printAnswer(conn)
}

// execKeeping runs another program in this process, conn kept open across it: this same test for hold and exec,
// /bin/sleep for sleep.
func execKeeping(conn net.Conn, mode string) {
	kept, err := conn.(interface{ File() (*os.File, error) }).File()
	if err != nil {
		os.Exit(6)
	}
	if _, err := unix.FcntlInt(kept.Fd(), unix.F_SETFD, 0); err != nil {
		os.Exit(7)
	}
	if mode == "sleep" {
		_ = syscall.Exec("/bin/sleep", []string{"sleep", "30"}, os.Environ())
		os.Exit(8)
	}
	next := map[string]string{"hold": "held", "exec": "execed"}[mode]
	// Its mode replaced, not added: a program takes the first of two.
	var env []string
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "SMOLCORP_TEST_SN1_MODE=") {
			env = append(env, variable)
		}
	}
	env = append(env, "SMOLCORP_TEST_SN1_MODE="+next, fmt.Sprintf("SMOLCORP_TEST_SN1_FD=%d", kept.Fd()))
	_ = syscall.Exec(os.Args[0], []string{os.Args[0], "-test.run=^TestStandInPeerForSN1$"}, env)
	os.Exit(8)
}

// startStandIn runs a stand-in peer of a mode, to address over network, with a request to send.
func startStandIn(t *testing.T, network, address, mode, request string) *standInProcess {
	t.Helper()
	return startStandInRun(t, "TestStandInPeerForSN1", "SMOLCORP_TEST_SN1_PEER="+address, "SMOLCORP_TEST_SN1_NET="+network,
		"SMOLCORP_TEST_SN1_MODE="+mode, "SMOLCORP_TEST_SN1_REQUEST="+request)
}

// acceptOne listens on a unix socket of the test's own and gives the one connection it accepts.
func acceptOne(t *testing.T) (string, <-chan net.Conn) {
	t.Helper()
	path := filepath.Join(shortTempDir(t), "peer.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		if conn, err := listener.Accept(); err == nil {
			t.Cleanup(func() { conn.Close() })
			accepted <- conn
		}
	}()
	return path, accepted
}

// serveSockRig is SN1's end to end: the deck's own server on a real serve.sock (or over TCP), each connection to
// serve.sock judged by this system's real identifyServePeerHere as it's accepted, and every answer kept by its "who".
type serveSockRig struct {
	d        *Deck
	phoneKey string
	// standIn is the running stand-in peer's pid: the one each system's test takes for Tailscale's.
	standIn atomic.Int32
	// judged is each connection to serve.sock as it was judged.
	judged   chan bool
	mu       sync.Mutex
	answered map[string]int
}

// newServeSockRig is a deck with a paired phone and its text on, and serve.sock's judge watched.
func newServeSockRig(t *testing.T) *serveSockRig {
	t.Helper()
	d, phoneKey := probeDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	servedUnrecorded = func(*http.Request) bool { return false }
	rig := &serveSockRig{d: d, phoneKey: phoneKey, judged: make(chan bool, 64), answered: map[string]int{}}
	savedIdentify := identifyServePeer
	t.Cleanup(func() { identifyServePeer = savedIdentify })
	identifyServePeer = func(conn net.Conn) bool {
		ok := identifyServePeerHere(conn)
		rig.judged <- ok
		return ok
	}
	return rig
}

// serveHeld runs the deck's server on serve.sock, or over TCP, accepting nothing until let go.
func (rig *serveSockRig) serveHeld(t *testing.T, network string) (string, chan struct{}) {
	t.Helper()
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
	var listener net.Listener
	var address string
	if network == "unix" {
		address = filepath.Join(shortTempDir(t), serveSocketName)
		socket, err := openServeSocket(address)
		if err != nil {
			t.Fatal(err)
		}
		listener = &serveListener{Listener: heldListener{Listener: socket.(*serveListener).Listener, letGo: letGo}}
	} else {
		tcp, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address, listener = tcp.Addr().String(), heldListener{Listener: tcp, letGo: letGo}
	}
	go func() {
		_ = server.Serve(&cappedListener{Listener: listener, byAddress: map[string]int{}, thisComputer: network == "unix"})
	}()
	t.Cleanup(func() { _ = server.Close() })
	return address, letGo
}

// request is a phone's GET /approvals, by its "who", to host, with headers.
func (rig *serveSockRig) request(who, host string, headers ...string) string {
	lines := []string{fmt.Sprintf("GET /approvals?session=nobody&who=%s&t=%s HTTP/1.1", who, rig.phoneKey), "Host: " + host}
	lines = append(lines, headers...)
	return strings.Join(append(lines, "Connection: close", "", ""), "\r\n")
}

// standInPeer runs a stand-in peer, the one each system's test takes for Tailscale's, and says what it printed last.
// The server is held until the peer has sent (stay; half, its sending side closed too), sent and ended (exit), or run
// itself again (exec). So every row judges a client whose request is in: a server let go first could refuse and close
// before the client sent, and its write would fail, which proves nothing.
func (rig *serveSockRig) standInPeer(t *testing.T, network, mode, who, text string) string {
	t.Helper()
	address, letGo := rig.serveHeld(t, network)
	peer := startStandIn(t, network, address, mode, text)
	rig.standIn.Store(int32(peer.cmd.Process.Pid))
	if _, err := io.WriteString(peer.stdin, "go\n"); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "exit":
		// RSBXPLANf F2: the whole request is in, and its sender has ended, before the server accepts.
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
				t.Errorf("%s: an ended client was judged Tailscale's", who)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("%s: never judged", who)
		}
		return ""
	case "exec":
		// RT126SN1 B1: the request is in, and its sender runs a program again, before the server accepts.
		if got := peer.said(t); got != "execed" {
			t.Fatalf("%s: the stand-in said %q", who, got)
		}
	default:
		if got := peer.said(t); got != "sent" {
			t.Fatalf("%s: the stand-in said %q", who, got)
		}
	}
	close(letGo)
	return peer.said(t)
}

// thisProgram asks as this test's own process: a program here, which each system's test never takes for Tailscale's.
func (rig *serveSockRig) thisProgram(t *testing.T, network, text string) string {
	t.Helper()
	address, letGo := rig.serveHeld(t, network)
	close(letGo)
	conn, err := net.Dial(network, address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, text)
	answer, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "no answer"
	}
	return fmt.Sprint(answer.StatusCode)
}

// saidBy is the status a "who" was answered with, if it was answered at all.
func (rig *serveSockRig) saidBy(who string) (int, bool) {
	rig.mu.Lock()
	defer rig.mu.Unlock()
	status, ok := rig.answered[who]
	return status, ok
}

// sharedRows are the rows a Mac and Linux share, with the stand-in taken for Tailscale's: through serve.sock its
// hand-on is let in; over the TCP loopback the same stand-in, with the same request, is refused at the door (nothing
// counts as Tailscale's there). A program here (this test) is closed unanswered on serve.sock, and refused over TCP.
// Tailscale's own process naming this computer, or its TCP forward with a phone's key, is refused. A client that
// sends its request and ends before the server accepts (RSBXPLANf F2) is never answered.
func (rig *serveSockRig) sharedRows(t *testing.T) {
	t.Helper()
	const tsnet = "my-mac.tail1234.ts.net"
	forwardedFor := "X-Forwarded-For: 100.101.102.103"
	if got := rig.standInPeer(t, "unix", "stay", "serve", rig.request("serve", tsnet, forwardedFor)); got != "200" {
		t.Fatalf("tailscale serve's hand-on through serve.sock, from Tailscale's own process: %s, want 200", got)
	}
	if got := rig.standInPeer(t, "tcp", "stay", "serve-tcp", rig.request("serve-tcp", tsnet, forwardedFor)); got != "403" {
		t.Errorf("the same over the TCP loopback: %s, want 403", got)
	}
	for who, headers := range map[string][]string{
		"program-xff":       {forwardedFor},
		"program-forwarded": {"Forwarded: for=100.101.102.103"},
		"program-proto":     {"X-Forwarded-Proto: https"},
		"program-xfh":       {"X-Forwarded-Host: " + tsnet},
		"program-login":     {"Tailscale-User-Login: owner@example.com"},
		"program-all":       {forwardedFor, "X-Forwarded-Proto: https", "X-Forwarded-Host: " + tsnet, "Tailscale-User-Login: owner@example.com"},
		"program-tsnet":     nil,
	} {
		if got := rig.thisProgram(t, "unix", rig.request(who, tsnet, headers...)); got != "no answer" {
			t.Errorf("%s: a program here, through serve.sock: %s, want no answer", who, got)
		}
		if got := rig.thisProgram(t, "tcp", rig.request(who, tsnet, headers...)); got != "403" {
			t.Errorf("%s: a program here, over the loopback: %s, want 403", who, got)
		}
		if got := rig.thisProgram(t, "tcp", rig.request(who+"-localhost", "localhost:7710", headers...)); got == "200" {
			t.Errorf("%s to localhost: a program here: %s", who, got)
		}
	}
	if got := rig.standInPeer(t, "unix", "stay", "self", rig.request("self", tsnet, "X-Forwarded-For: 100.64.1.2")); got != "403" {
		t.Errorf("Tailscale's own process, for this computer itself: %s, want 403", got)
	}
	if got := rig.standInPeer(t, "unix", "stay", "forward", rig.request("forward", tsnet)); got != "401" {
		t.Errorf("Tailscale's TCP forward, with a phone's key: %s, want 401", got)
	}
	rig.standInPeer(t, "unix", "exit", "exited", rig.request("exited", tsnet, forwardedFor))
	if status, ok := rig.saidBy("exited"); ok {
		t.Errorf("a client that sent its request and ended before the server accepted was answered: %d", status)
	}
}

// ownRows are the rows each system's test runs as its stand-in for Tailscale changes (RT126SN1 B1): exec (a client
// that sends, then runs the program again, its socket kept) is let in while the stand-in is taken for Tailscale's
// whole, and closed unanswered once only the process as it runs now is; asOwner and half (the stand-in as it really
// is, the owner's, whole or half-closed) are closed unanswered.
func (rig *serveSockRig) ownRows(t *testing.T, takenWhole, asItRunsNow, asItIs func()) {
	t.Helper()
	const tsnet = "my-mac.tail1234.ts.net"
	forwardedFor := "X-Forwarded-For: 100.101.102.103"
	// With the stand-in taken for Tailscale's whole, a client that sends, then runs the program again, is let in: same
	// pid, read as it is now. So the row reaches past every other check, and only the user it connected as refuses it
	// below.
	takenWhole()
	if got := rig.standInPeer(t, "unix", "exec", "exec-as-before", rig.request("exec-as-before", tsnet, forwardedFor)); got != "200" {
		t.Fatalf("an exec'd client, taken for Tailscale's: %s, want 200 (the row doesn't reach the user)", got)
	}
	// Root wanted, and the process as it runs now stood in as root's: only the user it connected as, kept by the
	// kernel across exec, is left to refuse it.
	asItRunsNow()
	if got := rig.standInPeer(t, "unix", "exec", "exec", rig.request("exec", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("a client that sent, then exec'd, connected as the owner: %s, want no answer", got)
	}
	// And as it really is: the stand-in serve runs as the owner. Refused, whole or half-closed.
	asItIs()
	if got := rig.standInPeer(t, "unix", "stay", "as-owner", rig.request("as-owner", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("Tailscale's stand-in, running as the owner: %s, want no answer", got)
	}
	if got := rig.standInPeer(t, "unix", "half", "half", rig.request("half", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("a client that half-closed after sending: %s, want no answer", got)
	}
	for _, who := range []string{"exec", "as-owner", "half"} {
		if status, ok := rig.saidBy(who); ok {
			t.Errorf("%s was answered: %d", who, status)
		}
	}
}
