package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// The forwarder (SA-L6, proxy.go): `smolcorp sandbox forward <socket> <address>`, which `enter` starts inside the
// desk's view for its proxy row, before it becomes the desk's program. It listens at address, on the view's own
// loopback, says so on descriptor 3 ("ok <address>", or why not), closes it, and hands each connection on to the
// core's proxy at socket (Forward) for as long as the view lasts: bwrap's first process ends the view, and the kernel
// every process left in its pid namespace, when the desk's program ends. With a third word, the app's address
// (`… <socket> <address> 127.0.0.1:<the app's port>`, SA-L7), it listens there too, in the view, and each connection
// there goes to the app's port through the proxy (ForwardTo); it says "ok <address> <app's address>" once both listen.

// forwardReadyFD is the forwarder's descriptor for its one line to `enter`.
const forwardReadyFD = 3

// forwardWait is how long `enter` waits for the forwarder to listen.
const forwardWait = 5 * time.Second

// forward starts the forwarder for the proxy's socket at socket, listening at addr (and at app, for the app's port,
// when app isn't ""), and gives where it listens at addr once it listens at both. It is a process of its own, in a
// session of its own, holding nothing of `enter`'s but its line back: every descriptor above 2 is marked to close at
// the exec first, the handshake's among them.
func (s realSystem) forward(socket, addr, app string) (string, error) {
	at, p, err := s.startForwarder(socket, addr, app)
	if err == nil {
		p.Release() // the view's: it ends with it
	}
	return at, err
}

// startForwarder is forward, giving the forwarder's process too.
func (s realSystem) startForwarder(socket, addr, app string) (string, *os.Process, error) {
	fds, err := s.descriptors()
	if err != nil {
		return "", nil, err
	}
	for _, fd := range fds {
		if fd > 2 {
			if err := s.closeAtExec(fd); err != nil && !errors.Is(err, syscall.EBADF) {
				return "", nil, err
			}
		}
	}
	read, write, err := os.Pipe()
	if err != nil {
		return "", nil, err
	}
	defer read.Close()
	// Smolcorp's own program, as the kernel has it: the one `enter` runs from.
	words := []string{"sandbox", "forward", socket, addr}
	if app != "" {
		words = append(words, app)
	}
	cmd := disclaim.Cmd(exec.Command("/proc/self/exe", words...))
	cmd.ExtraFiles = []*os.File{write}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = cmd.Start()
	write.Close()
	if err != nil {
		return "", nil, err
	}
	read.SetReadDeadline(time.Now().Add(forwardWait))
	line, err := bufio.NewReader(io.LimitReader(read, 512)).ReadString('\n')
	if said, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "ok "); err == nil && ok && len(strings.Fields(said)) == len(words)-3 {
		return strings.Fields(said)[0], cmd.Process, nil
	}
	cmd.Process.Kill()
	cmd.Wait()
	if err == nil {
		err = errors.New(strings.TrimSpace(line))
	}
	return "", nil, fmt.Errorf("the forwarder: %w", err)
}

// runForward is `smolcorp sandbox forward <socket> <address>` (above).
func runForward(args []string, stderr io.Writer) int {
	ready := os.NewFile(forwardReadyFD, "ready")
	if _, err := unix.FcntlInt(forwardReadyFD, unix.F_GETFD, 0); err != nil || len(args) < 2 || len(args) > 3 {
		fmt.Fprintln(stderr, "smolcorp sandbox forward: Smolcorp runs this itself, inside a desk's sandbox")
		return 2
	}
	var ls []net.Listener
	for _, addr := range args[1:] {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(ready, "no listener at %s: %v\n", addr, err)
			ready.Close()
			return 1
		}
		ls = append(ls, l)
	}
	said := "ok"
	for _, l := range ls {
		said += " " + l.Addr().String()
	}
	fmt.Fprintln(ready, said)
	ready.Close()
	if len(ls) == 2 {
		go ForwardTo(ls[1], args[0], args[2]) // the app's port, by its own number
	}
	Forward(ls[0], args[0])
	return 1
}

// RunConnect is `smolcorp sandbox connect <host> <port>`, ssh's ProxyCommand in a Linux desk (ProxyEnv's
// GIT_SSH_COMMAND): a CONNECT to host:port through the forwarder on the view's own loopback, and ssh's bytes both ways
// on stdin and stdout. The computer's own addresses (`ssh localhost`, Docker's TCP) are refused by the core's proxy,
// in its words on stderr.
func RunConnect(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return connectThrough("127.0.0.1:"+strconv.Itoa(ForwardPort), args, stdin, stdout, stderr)
}
