package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// Linux's runner (SA-L2, SANDBOX-PLAN §A3.2 and §A5.2 steps 1, 6 and 9): how the core starts a desk's first process in
// its view, and how it then checks that the server at the desk's socket is the one it started.
//
// The start is `<BwrapPath> --seccomp 4 --info-fd 5 <the view's words, up to and with --> <smolcorp> sandbox enter …
// -- <program> …` (LinuxCommand), through disclaim.Cmd (which off a Mac starts it as it is). Smolcorp's own bwrap,
// checked first (CheckBwrap: root's alone, the packaged one, under its AppArmor profiles), makes the view whose words
// PrepareBwrap gave, loads the filter (DeskFilter) from descriptor 4, writes its child's pid to descriptor 5, and runs
// `enter` in the view, which checks the view holds (the support folder a tmpfs of its own with no canary in it, the
// fences EROFS, the filter, Landlock's scope where the kernel has it) and becomes the program. bwrap stays while
// anything in the view runs: its first process is the view's own.
//
// The server check (ServerInView, step 9): as bwrap starts, before anything of the desk's runs, the core records the
// pid and mount namespaces of bwrap's child (Start.Namespaces), from /proc as the core sees it. After the start it
// connects to the desk's socket and takes the listener's pid as the kernel gives it (SO_PEERCRED). The start passes
// only when that pid's pid and mount namespaces are the recorded ones: a server in no view, or in another desk's, is
// refused. Wiring it into a desk's start (the socket folder made new, tmux's start, the pid recorded) is SA-L4's.

// checkBwrapNow is CheckBwrap: a stand-in in the tests.
var checkBwrapNow = CheckBwrap

// LinuxCommand is a desk's start on Linux (§A3.2): s's `enter` in the view whose words are view (PrepareBwrap's, up to
// and with "--"), from Smolcorp's own program smolcorp, becoming program with args once its checks pass. s must have
// the seccomp row (Checks.Seccomp): the filter is always loaded, and always checked. env is the whole environment
// `enter` gets, with the start's nonce added (a nonce already in env is dropped); the view's words take out what the
// desk mustn't have. The command's ExtraFiles are s's: the caller adds none, starts it, then calls s.Namespaces and
// s.Wait, and closes s when the start is over.
//
// Nothing is made, and nothing starts, unless CheckBwrap passes (its error, which LinuxStartRefusal words) and this
// processor has a filter.
func LinuxCommand(s *Start, view []string, smolcorp string, env []string, program string, args ...string) (*exec.Cmd, error) {
	if s == nil || len(view) == 0 || view[len(view)-1] != "--" || !filepath.IsAbs(smolcorp) || !filepath.IsAbs(program) ||
		!slices.ContainsFunc(s.rows, func(r checkRow) bool { return r.kind == rowSeccomp }) || s.info != nil {
		return nil, ErrStartNotWhole
	}
	if err := checkBwrapNow(); err != nil {
		return nil, err
	}
	filter, err := DeskFilter(runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	filterRead, filterWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	_, err = filterWrite.Write(filter) // a few hundred bytes: the pipe holds them until bwrap reads
	if closeErr := filterWrite.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		filterRead.Close()
		return nil, err
	}
	infoRead, infoWrite, err := os.Pipe()
	if err != nil {
		filterRead.Close()
		return nil, err
	}
	s.given, s.info = []*os.File{filterRead, infoWrite}, infoRead
	words := append([]string{"--seccomp", strconv.Itoa(handshakeFD + 1), "--info-fd", strconv.Itoa(handshakeFD + 2)}, view...)
	words = append(append(words, smolcorp), s.Words(program, args...)...)
	cmd := disclaim.Cmd(exec.Command(BwrapPath, words...))
	cmd.Env = append(slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		return strings.HasPrefix(kv, nonceVar+"=")
	}), s.Env())
	cmd.ExtraFiles = append(s.Files(), s.given...)
	return cmd, nil
}

// Namespaces are the pid and mount namespaces of the view bwrap made for s, as its info names its child: read once
// the command has started, before the handshake, for up to within. The core keeps them for ServerInView. Anything
// else is a *Refused.
func (s *Start) Namespaces(within time.Duration) (Namespaces, error) {
	if s.info == nil {
		return Namespaces{}, &Refused{Why: "no bwrap info for this start"}
	}
	s.closeGiven() // bwrap holds its own now: what it says, and its end, are its own
	if err := s.info.SetReadDeadline(time.Now().Add(within)); err != nil {
		return Namespaces{}, &Refused{Why: "bwrap's info can't be waited for: " + err.Error()}
	}
	pid, err := readInfo(s.info)
	if err != nil {
		return Namespaces{}, &Refused{Why: "bwrap's info: " + err.Error()}
	}
	ns, err := namespacesOf(pid)
	if err != nil {
		return Namespaces{}, &Refused{Why: "the view's namespaces can't be read: " + err.Error()}
	}
	return ns, nil
}

// namespacesOf are the pid and mount namespaces process pid is in, as /proc shows them: a stand-in in the tests.
var namespacesOf = func(pid int) (Namespaces, error) {
	var n Namespaces
	for _, ns := range []struct {
		name string
		to   *NS
	}{{"pid", &n.Pid}, {"mnt", &n.Mnt}} {
		var st unix.Stat_t
		if err := unix.Stat("/proc/"+strconv.Itoa(pid)+"/ns/"+ns.name, &st); err != nil {
			return Namespaces{}, err
		}
		*ns.to = NS{Dev: uint64(st.Dev), Ino: st.Ino}
	}
	return n, nil
}

// peerCred is the user and pid of the process listening at the far end of a unix socket on fd, as the kernel took them
// when it listened (SO_PEERCRED): a stand-in in the tests.
var peerCred = func(fd int) (uid uint32, pid int, err error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, 0, err
	}
	return cred.Uid, int(cred.Pid), nil
}

// ServerInView is §A5.2 step 9 on Linux: the server listening at socket, the desk's own after its start, is in the
// view whose namespaces are view (Start.Namespaces). It gives the server's pid, for the desk's record and the guarded
// clients, or an error that is ErrNotOurServer (the owner sees NotOurServerRefusal):
//   - socket must be a socket, the owner's, not a link;
//   - the server must answer within a few seconds;
//   - it listened as this user, with a pid the kernel names;
//   - that pid's pid and mount namespaces must be the view's.
func ServerInView(socket string, view Namespaces) (int, error) {
	refuse := func(why string) (int, error) {
		return 0, fmt.Errorf("%w: the server at %s: %s", ErrNotOurServer, socket, why)
	}
	info, err := os.Lstat(socket)
	if err != nil {
		return refuse("not there: " + errnoWord(err))
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode().Type() != fs.ModeSocket || !ok || st.Uid != uint32(os.Getuid()) {
		return refuse(fmt.Sprintf("not a socket of this user's (%v)", info.Mode()))
	}
	conn, err := net.DialTimeout("unix", socket, 5*time.Second)
	if err != nil {
		return refuse("no answer: " + errnoWord(err))
	}
	defer conn.Close()
	raw, err := conn.(*net.UnixConn).SyscallConn()
	if err != nil {
		return refuse(err.Error())
	}
	var uid uint32
	var pid int
	var readErr error
	if err := raw.Control(func(fd uintptr) { uid, pid, readErr = peerCred(int(fd)) }); err != nil || readErr != nil {
		return refuse(fmt.Sprintf("who listens can't be read: %v", errors.Join(err, readErr)))
	}
	got, err := namespacesOf(pid)
	if err != nil {
		return refuse(fmt.Sprintf("pid %d's namespaces can't be read: %v", pid, err))
	}
	if err := peerInView(uid, pid, got, view); err != nil {
		return refuse(err.Error())
	}
	return pid, nil
}

// LandlockScope is whether a start on this kernel has the Landlock row (Checks.Landlock: Landlock's abstract-socket
// scope, ABI 6 and later), and why not when it hasn't, for the start's log (§A5.2 step 6, "where present"). The core
// asks the kernel itself, outside any view: no desk changes the answer.
func LandlockScope() (bool, string) { return landlockFrom(landlockABI()) }

// landlockABI is the kernel's Landlock ABI, or why it has none: a stand-in in the tests.
var landlockABI = func() (int, error) {
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0, errno
	}
	return int(abi), nil
}
