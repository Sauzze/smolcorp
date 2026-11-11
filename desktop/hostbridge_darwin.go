package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// socketBridge reaches the Mac app on its socket in the support folder, with
// the token the app gave this engine as it started it: one connection per
// request, a line of JSON each way.
//
// The socket's path alone says nothing of who listens there: another process
// of yours could move the app's socket aside and listen in its place. So each
// connection's other end is checked first, by its pid (LOCAL_PEERPID, which
// the system sets as the socket is listened on, and no process can fake): it
// must be the app that started this engine. Only then is the token sent. The
// app checks this end's pid in turn (HostBridge.serve), so the token alone
// opens nothing either.
type socketBridge struct {
	path, token string
	// parent says which process the app is: os.Getppid, the app that started
	// this engine. Asked at each request, so an engine whose app has gone (its
	// parent is then launchd) sends nothing to anyone.
	parent func() int
	// How long one request may take: a terminal's AppleScript can take a few
	// seconds. 20 s when zero; the app starts a tab action only within 5 s of
	// taking the connection (Terminals.inputWait) and a window within 18 s, and
	// answers by 18 s (HostBridge.actionWait), so it never runs one after this
	// side has given up.
	wait time.Duration
}

// bridgeAnswerMost is the longest answer read: a secret or a tab name is short.
const bridgeAnswerMost = 64 << 10

// errNotTheApp is a socket whose other end isn't the app that started this engine.
var errNotTheApp = errors.New("the bridge's socket isn't the app's that started this engine")

func (b *socketBridge) call(request bridgeRequest) (bridgeAnswer, error) {
	conn, err := net.DialTimeout("unix", b.path, 2*time.Second)
	if err != nil {
		return bridgeAnswer{}, err
	}
	defer conn.Close()
	// Who answers, before anything is written: never the token to anyone else.
	if !b.isTheApp(conn) {
		return bridgeAnswer{}, errNotTheApp
	}
	request.Token = b.token
	wait := b.wait
	if wait <= 0 {
		wait = 20 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(wait))
	line, err := json.Marshal(request)
	if err != nil {
		return bridgeAnswer{}, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return bridgeAnswer{}, err
	}
	reader := bufio.NewReaderSize(conn, 4096)
	var text []byte
	for {
		part, more, err := reader.ReadLine()
		if err != nil {
			return bridgeAnswer{}, err
		}
		text = append(text, part...)
		if len(text) > bridgeAnswerMost {
			return bridgeAnswer{}, errors.New("the Mac app's answer is too long")
		}
		if !more {
			break
		}
	}
	var answer bridgeAnswer
	if err := json.Unmarshal(text, &answer); err != nil {
		return bridgeAnswer{}, err
	}
	return answer, nil
}

// isTheApp says whether conn's other end is the process that listens as the
// app: the app that started this engine, and not launchd (pid 1), which an
// engine whose app has gone has as its parent.
func (b *socketBridge) isTheApp(conn net.Conn) bool {
	if b.parent == nil {
		return false
	}
	app := b.parent()
	peer, err := peerPid(conn)
	return err == nil && app > 1 && peer == app
}

// peerPid is the pid of the process at conn's other end, as the system has it.
func peerPid(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not a Unix socket")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	pid, failed := 0, error(nil)
	if err := raw.Control(func(fd uintptr) {
		pid, failed = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	}); err != nil {
		return 0, err
	}
	return pid, failed
}
