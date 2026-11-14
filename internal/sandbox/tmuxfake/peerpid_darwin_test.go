package tmuxfake

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The spike row (PL126A §1.4 item 5): LOCAL_PEERTOKEN follows exec (T126SN1U), so does LOCAL_PEERPID, which the
// guard reads right after connecting? Stand-in servers, copies of this test program, listen on a socket and then
// (a) exec into another program that accepts on the same socket, or (b) hand the socket to a child that accepts while
// the parent keeps running. The client reads LOCAL_PEERPID on connecting, before anything is sent, as the guard does.
//
// What it shows: the pid is the one of the process the socket belongs to now, not of the one that made it. Across an
// exec it stays the same pid (an exec keeps the pid), so a hire's checked server that execs a fake server still
// passes the pid check; that's T5's worst case, and why the guard's refusals, not the pid, are what make a client
// safe. A socket handed to another process reports that process once it waits on the socket (from the first
// connection on), so the pid check fails closed for it. Not probed: a parent that touches the socket again after
// handing it on (LOCAL_PEERPID is the socket's last user, so it may name the parent); both would be in the same
// sandbox, which is T5's worst case again.

func init() { helperMain = peerPidHelper }

func peerPidHelper(mode string) {
	switch mode {
	case "exec", "child":
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: os.Getenv("TMUXFAKE_SOCKET"), Net: "unix"})
		if err != nil {
			fmt.Println("error", err)
			return
		}
		file, err := listener.File()
		if err == nil && mode == "exec" { // the dup is close-on-exec: the exec must keep it
			_, err = unix.FcntlInt(file.Fd(), unix.F_SETFD, 0)
		}
		if err != nil {
			fmt.Println("error", err)
			return
		}
		if mode == "exec" {
			fmt.Println("listened", os.Getpid())
			env := helperEnv("TMUXFAKE_HELPER=accept", "TMUXFAKE_FD="+strconv.Itoa(int(file.Fd())))
			err = syscall.Exec(os.Args[0], []string{os.Args[0], "-test.run=^$"}, env)
			fmt.Println("error", err)
			return
		}
		child := exec.Command(os.Args[0], "-test.run=^$")
		child.Env = helperEnv("TMUXFAKE_HELPER=accept", "TMUXFAKE_FD=3")
		child.ExtraFiles = []*os.File{file}
		child.Stdout, child.Stdin = os.Stdout, os.Stdin
		fmt.Println("listened", os.Getpid())
		if err := child.Run(); err != nil {
			fmt.Println("error", err)
		}
	case "accept":
		fd, _ := strconv.Atoi(os.Getenv("TMUXFAKE_FD"))
		listener, err := net.FileListener(os.NewFile(uintptr(fd), "listener"))
		if err != nil {
			fmt.Println("error", err)
			return
		}
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
			}
		}()
		fmt.Println("accepting", os.Getpid())
		io.Copy(io.Discard, os.Stdin) // until the test closes it
	}
}

// helperEnv is this process's environment with the helper's settings replaced: an exec doesn't drop a duplicate.
func helperEnv(settings ...string) []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "TMUXFAKE_HELPER=") && !strings.HasPrefix(entry, "TMUXFAKE_FD=") {
			env = append(env, entry)
		}
	}
	return append(env, settings...)
}

// startPeerPidHelper starts a stand-in server in mode, and returns the pids it printed: the one that listened, the
// one that accepts.
func startPeerPidHelper(t *testing.T, mode, socket string) (int, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = helperEnv("TMUXFAKE_HELPER="+mode, "TMUXFAKE_SOCKET="+socket)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); cmd.Wait() })
	pids := map[string]int{}
	lines := bufio.NewScanner(stdout)
	for len(pids) < 2 && lines.Scan() {
		word, value, _ := strings.Cut(lines.Text(), " ")
		if word == "error" {
			t.Fatal(value)
		}
		pids[word], _ = strconv.Atoi(value)
	}
	if pids["listened"] == 0 || pids["accepting"] == 0 {
		t.Fatalf("the stand-in said %v", pids)
	}
	return pids["listened"], pids["accepting"]
}

// peerPid connects and reads LOCAL_PEERPID before sending anything.
func peerPid(t *testing.T, socket string) int {
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
	pid, sockErr := 0, error(nil)
	raw.Control(func(fd uintptr) { pid, sockErr = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID) })
	if sockErr != nil {
		t.Fatal(sockErr)
	}
	return pid
}

func TestSpikeLocalPeerPidAcrossExecAndAHandedOnSocket(t *testing.T) {
	dir := shortDir(t)

	socket := filepath.Join(dir, "exec")
	listened, accepting := startPeerPidHelper(t, "exec", socket)
	got := peerPid(t, socket)
	t.Logf("exec: listened in %d, accepts in %d (after exec), LOCAL_PEERPID %d", listened, accepting, got)
	if listened != accepting || got != listened {
		t.Errorf("across an exec the pid should stay the same and be the one reported: listened %d, accepts %d, LOCAL_PEERPID %d", listened, accepting, got)
	}

	socket = filepath.Join(dir, "child")
	listened, accepting = startPeerPidHelper(t, "child", socket)
	// The child is waiting in accept when the first connection comes, and the parent never touches the socket again.
	first := peerPid(t, socket)
	time.Sleep(100 * time.Millisecond)
	got = peerPid(t, socket)
	t.Logf("handed on: listened in %d, accepts in child %d, LOCAL_PEERPID %d first, then %d", listened, accepting, first, got)
	if listened == accepting || first != accepting || got != accepting {
		t.Errorf("a socket handed to a child that accepts should report the child: listened %d, child %d, LOCAL_PEERPID %d then %d", listened, accepting, first, got)
	}
}
