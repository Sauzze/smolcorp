package desktop

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The parts of SN1's stand-in rig every system shares (peerowner_standin_test.go on a Mac and Linux,
// peerowner_windows_test.go on Windows): a stand-in peer run as its own process, a listener held until let go, and
// the status each request was answered with.

// standInProcess is a stand-in peer, running.
type standInProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
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

// startStandInRun runs this test program again as a stand-in peer, only the test named run, with env added.
func startStandInRun(t *testing.T, run string, env ...string) *standInProcess {
	t.Helper()
	peer := exec.Command(os.Args[0], "-test.run=^"+run+"$")
	peer.Env = append(os.Environ(), env...)
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
