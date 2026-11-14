//go:build darwin && cgo

package desktop

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
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

// thisTestsCode is a code requirement this test's binary meets, and a stand-in peer run from it: the Go linker signs
// it ad hoc, named a.out. /bin/sleep (com.apple.sleep) doesn't meet it.
const thisTestsCode = `identifier "a.out"`

// TestStandInPeerForSN1 is a stand-in for the other end of a connection to the deck, run as its own process by the
// tests below; it does nothing in a normal run. By its mode it:
//   - stay: waits for the word, connects (to serve.sock, or over TCP), sends its request, reads the answer and
//     prints its status, or "no answer";
//   - half: the same, half-closing after it sends;
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
		_ = conn.(interface{ CloseWrite() error }).CloseWrite()
	case "exec":
		execKeeping(conn, mode)
	}
	printAnswer(conn)
}

// printAnswer reads the deck's answer and prints its status, or "no answer" when the deck closed the connection.
func printAnswer(conn net.Conn) {
	answer, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		fmt.Println("no answer")
		os.Exit(0)
	}
	fmt.Println(answer.StatusCode)
	os.Exit(0)
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

// standInProcess is a stand-in peer, running.
type standInProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
}

// startStandIn runs a stand-in peer of a mode, to address over network, with a request to send.
func startStandIn(t *testing.T, network, address, mode, request string) *standInProcess {
	t.Helper()
	peer := exec.Command(os.Args[0], "-test.run=^TestStandInPeerForSN1$")
	peer.Env = append(os.Environ(), "SMOLCORP_TEST_SN1_PEER="+address, "SMOLCORP_TEST_SN1_NET="+network,
		"SMOLCORP_TEST_SN1_MODE="+mode, "SMOLCORP_TEST_SN1_REQUEST="+request)
	stdin, _ := peer.StdinPipe()
	stdout, _ := peer.StdoutPipe()
	if err := peer.Start(); err != nil {
		t.Fatal(err)
	}
	p := &standInProcess{cmd: peer, stdin: stdin, lines: make(chan string, 8)}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.lines <- scanner.Text()
		}
		close(p.lines)
	}()
	t.Cleanup(func() { _ = stdin.Close(); _ = peer.Process.Kill(); _ = peer.Wait() })
	return p
}

// said is the stand-in's next line, or "" if it said nothing more in time.
func (p *standInProcess) said(t *testing.T) string {
	t.Helper()
	select {
	case line := <-p.lines:
		return line
	case <-time.After(20 * time.Second):
		t.Error("the stand-in peer said nothing")
		return ""
	}
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

// heldListener accepts nothing until it is let go: a client can connect, send and end before the server accepts.
type heldListener struct {
	net.Listener
	letGo chan struct{}
}

func (l heldListener) Accept() (net.Conn, error) {
	<-l.letGo
	return l.Listener.Accept()
}

// statusKept keeps the status a request was answered with, by its "who".
type statusKept struct {
	http.ResponseWriter
	status int
}

func (s *statusKept) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusKept) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusKept) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// SN1 end to end on this Mac, through the deck's own server, a real serve.sock and the kernel's real answers, with
// stand-in peers: only the stand-in the check takes for Tailscale's is (this test's binary, by its signature, at
// that pid), with this test's user standing in for root. Through serve.sock its hand-on is let in. Over the TCP
// loopback the same stand-in, with the same request, is refused at the door: nothing counts as Tailscale's there. A
// program here (this test) is closed unanswered on serve.sock; so is a client that sends its request and ends before
// the server accepts (RSBXPLANf F2), even the one the check takes for Tailscale's. Tailscale's own process naming
// this computer, or its TCP forward with a phone's key, is refused. With root wanted, as in the app: a client that
// sends and then execs the "signed" image is refused for the user it connected as alone (RT126SN1 B1), and so are
// the stand-in serve itself (run as the owner, like the Mac App Store build) and a client that half-closes.
func TestOnlyTheRealServeIsLetInThroughServeSock(t *testing.T) {
	d, phoneKey := probeDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	savedSigned, savedUID, savedRead, savedIdentify := tailscaleSigned, tailscaleUID, readServePeer, identifyServePeer
	t.Cleanup(func() {
		tailscaleSigned, tailscaleUID, readServePeer, identifyServePeer = savedSigned, savedUID, savedRead, savedIdentify
	})
	macTailscale, servedUnrecorded = true, func(*http.Request) bool { return false }
	// Root wanted, as in the app. The stand-ins are set once, before any server runs, and steered by standing: the
	// stand-in peers run as this test's user, so it stands in for root (everyUser), until the rows that want root
	// itself; there, only the stand-in's token is stood in as root's (itsToken), then nothing (asItIs).
	const everyUser, itsToken, asItIs = 0, 1, 2
	var standing, standIn atomic.Int32
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
			if peer.token.pid() == standIn.Load() {
				asRoot(&peer.token[1])
				asRoot(&peer.token[3])
			}
		}
		return peer, ok
	}
	tailscaleSigned = func(token auditToken) bool {
		return token.pid() == standIn.Load() && tokenMeets(token, thisTestsCode, "")
	}
	// Each connection to serve.sock, as it's judged.
	judged := make(chan bool, 64)
	identifyServePeer = func(conn net.Conn) bool {
		ok := identifyServePeerHere(conn)
		judged <- ok
		return ok
	}

	var mu sync.Mutex
	answered := map[string]int{}
	// serveHeld runs the deck's server on serve.sock, or over TCP, accepting nothing until let go.
	serveHeld := func(t *testing.T, network string) (string, chan struct{}) {
		letGo := make(chan struct{})
		server := newServer(d)
		inner := server.Handler
		server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			kept := &statusKept{ResponseWriter: w}
			inner.ServeHTTP(kept, r)
			mu.Lock()
			answered[r.URL.Query().Get("who")] = kept.status
			mu.Unlock()
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
	request := func(who, host string, headers ...string) string {
		lines := []string{fmt.Sprintf("GET /approvals?session=nobody&who=%s&t=%s HTTP/1.1", who, phoneKey), "Host: " + host}
		lines = append(lines, headers...)
		return strings.Join(append(lines, "Connection: close", "", ""), "\r\n")
	}
	const tsnet = "my-mac.tail1234.ts.net"
	forwardedFor := "X-Forwarded-For: 100.101.102.103"
	// standInPeer runs a stand-in peer, the one the stand-in check takes for Tailscale's, and says what it printed
	// last. The server is held until the peer has sent and ended (exit), or run itself again (exec).
	standInPeer := func(t *testing.T, network, mode, who, text string) string {
		address, letGo := serveHeld(t, network)
		peer := startStandIn(t, network, address, mode, text)
		standIn.Store(int32(peer.cmd.Process.Pid))
		if _, err := io.WriteString(peer.stdin, "go\n"); err != nil {
			t.Fatal(err)
		}
		switch mode {
		case "exit":
			// RSBXPLANf F2: the whole request is in, and its sender has ended, before the server accepts.
			if err := peer.cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			for len(judged) > 0 {
				<-judged
			}
			close(letGo)
			select {
			case ok := <-judged:
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
		}
		close(letGo)
		return peer.said(t)
	}
	// thisProgram asks as this test's own process: a program here, which the stand-in check never takes for
	// Tailscale's.
	thisProgram := func(t *testing.T, network, text string) string {
		address, letGo := serveHeld(t, network)
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
	saidBy := func(who string) (int, bool) {
		mu.Lock()
		defer mu.Unlock()
		status, ok := answered[who]
		return status, ok
	}

	if got := standInPeer(t, "unix", "stay", "serve", request("serve", tsnet, forwardedFor)); got != "200" {
		t.Fatalf("tailscale serve's hand-on through serve.sock, from Tailscale's own process: %s, want 200", got)
	}
	if got := standInPeer(t, "tcp", "stay", "serve-tcp", request("serve-tcp", tsnet, forwardedFor)); got != "403" {
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
		if got := thisProgram(t, "unix", request(who, tsnet, headers...)); got != "no answer" {
			t.Errorf("%s: a program here, through serve.sock: %s, want no answer", who, got)
		}
		if got := thisProgram(t, "tcp", request(who, tsnet, headers...)); got != "403" {
			t.Errorf("%s: a program here, over the loopback: %s, want 403", who, got)
		}
		if got := thisProgram(t, "tcp", request(who+"-localhost", "localhost:7710", headers...)); got == "200" {
			t.Errorf("%s to localhost: a program here: %s", who, got)
		}
	}
	if got := standInPeer(t, "unix", "stay", "self", request("self", tsnet, "X-Forwarded-For: 100.64.1.2")); got != "403" {
		t.Errorf("Tailscale's own process, for this computer itself: %s, want 403", got)
	}
	if got := standInPeer(t, "unix", "stay", "forward", request("forward", tsnet)); got != "401" {
		t.Errorf("Tailscale's TCP forward, with a phone's key: %s, want 401", got)
	}
	standInPeer(t, "unix", "exit", "exited", request("exited", tsnet, forwardedFor))
	if status, ok := saidBy("exited"); ok {
		t.Errorf("a client that sent its request and ended before the server accepted was answered: %d", status)
	}

	// RT126SN1 B1. With this user standing in for root, a client that sends, then runs the program the stand-in check
	// takes for Tailscale's, is let in: same pid, a "signed" image, read as it is now. So the row reaches past every
	// other check, and only the user it connected as refuses it below.
	if got := standInPeer(t, "unix", "exec", "exec-as-before", request("exec-as-before", tsnet, forwardedFor)); got != "200" {
		t.Fatalf("an exec'd client, with this user standing in for root: %s, want 200 (the row doesn't reach the user)", got)
	}
	// Root wanted, and the process now stood in as root's (its real and effective user in its token): only the user
	// it connected as, kept by the kernel across exec, is left to refuse it.
	standing.Store(itsToken)
	if got := standInPeer(t, "unix", "exec", "exec", request("exec", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("a client that sent, then exec'd a signed program, connected as the owner: %s, want no answer", got)
	}
	// And as it really is: the stand-in serve runs as the owner, like the Mac App Store build's extension. Refused,
	// whole or half-closed.
	standing.Store(asItIs)
	if got := standInPeer(t, "unix", "stay", "as-owner", request("as-owner", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("Tailscale's stand-in, running as the owner (the Mac App Store build): %s, want no answer", got)
	}
	if got := standInPeer(t, "unix", "half", "half", request("half", tsnet, forwardedFor)); got != "no answer" {
		t.Errorf("a client that half-closed after sending: %s, want no answer", got)
	}
	for _, who := range []string{"exec", "as-owner", "half"} {
		if status, ok := saidBy(who); ok {
			t.Errorf("%s was answered: %d", who, status)
		}
	}
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
