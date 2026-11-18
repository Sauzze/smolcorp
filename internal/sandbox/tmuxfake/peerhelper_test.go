//go:build darwin || linux

package tmuxfake

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// The stand-in servers of the pid spikes (peerpid_darwin_test.go, peercred_linux_test.go): copies of this test
// program that listen on a socket and then (a) exec into another program that accepts on the same socket, or (b) hand
// the socket to a child that accepts while the parent keeps running.

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
