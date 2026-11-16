package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// The Mac's runner (SA-M3, SANDBOX-PLAN §A3.1 and §A5.2 steps 1 and 9): how the core starts a desk's first process in
// its sandbox, and how it then checks that the server at the desk's socket is the one it started.
//
// The start is `smolcorp-spawn /usr/bin/sandbox-exec -f <profile> <smolcorp> sandbox enter … -- <program> …`
// (MacCommand). disclaim.Cmd puts smolcorp-spawn first, so the desk holds none of Smolcorp's grants (ST1); it runs
// before sandbox-exec, outside the profile. sandbox-exec applies the profile and becomes `smolcorp sandbox enter`, which
// checks the sandbox holds and becomes the program: each becomes the next in place, so the pid the core started is the
// program's. Responsibility stays disclaimed across sandbox-exec (run_darwin_test.go proves it on stand-ins).
//
// sandbox-exec is run only from SandboxExecPath, never PATH, and only while it and every folder above it are root's,
// not links, and not writable by the group or others (CheckSandboxExec): a program a desk could replace would be the
// wall itself.
//
// The server check (ServerIsOurs, T10): the core connects to the desk's socket and asks macOS whether the process
// listening there may read this start's canary. Only a process in our profile is refused it: one in no sandbox, or in
// another, looser one, may, and the start is refused. The listener is named twice, as SN1U's finding asks (a token
// follows exec): its pid as the kernel took it when it listened (LOCAL_PEERPID, with LOCAL_PEERCRED's user), and its
// audit token as it is now (LOCAL_PEERTOKEN), which must name that same pid; macOS is asked by the token, which names
// one run of one process, so a pid used again by another process since is never the one asked about.

// SandboxExecPath is the only sandbox-exec the core runs: Apple's, on the system volume.
const SandboxExecPath = "/usr/bin/sandbox-exec"

// What the owner sees, on the card and in Setup's Sandbox row (§A5.2's table).
const (
	// SandboxExecRefusal: sandbox-exec is missing or not Apple's alone, or the profile can't be used.
	SandboxExecRefusal = "Smolcorp couldn't put this hire in its sandbox on this Mac, so it didn't start it. Update Smolcorp, or turn the sandbox off in Settings."
	// NotOurServerRefusal: something was at the desk's socket already, or the server there isn't the one started.
	NotOurServerRefusal = "Something else was already using this hire's place, so Smolcorp didn't start it. Quit other copies of Smolcorp and try again."
)

// Errors the runner gives.
var (
	ErrNoSandboxExec = errors.New("sandbox: /usr/bin/sandbox-exec is missing, or it or a folder above it isn't root's alone")
	ErrStartNotWhole = errors.New("sandbox: a desk's start needs whole paths: the profile, Smolcorp and the program")
	ErrNotOurServer  = errors.New("sandbox: the server at the desk's socket isn't in this start's sandbox")
)

// pinnedStat is what CheckSandboxExec asks of a path, never following a link: a stand-in in the tests.
var pinnedStat = func(path string) (fs.FileMode, uint32, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, 0, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	return info.Mode(), st.Uid, nil
}

// CheckSandboxExec is §A5.2 step 1 on a Mac: SandboxExecPath is a program, and it and every folder above it are
// root's, not links, and not writable by the group or others. Anything else is ErrNoSandboxExec, and the start is
// refused with SandboxExecRefusal.
func CheckSandboxExec() error {
	for at := SandboxExecPath; ; at = filepath.Dir(at) {
		mode, uid, err := pinnedStat(at)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrNoSandboxExec, at, err)
		}
		program := at == SandboxExecPath
		if mode&fs.ModeSymlink != 0 || uid != 0 || mode.Perm()&0o022 != 0 ||
			program && (!mode.IsRegular() || mode.Perm()&0o111 == 0) || !program && !mode.IsDir() {
			return fmt.Errorf("%w: %s (%v, owner %d)", ErrNoSandboxExec, at, mode, uid)
		}
		if at == "/" {
			return nil
		}
	}
}

// MacCommand is a desk's start on a Mac (§A3.1): s's `enter` under the profile at profile, from Smolcorp's own
// program smolcorp, becoming program with args once its checks pass. profile is the one PrepareProfile wrote for the
// desk; smolcorp, the app's resolved bin/smolcorp; program and smolcorp are run by their whole paths. env is the whole
// environment `enter` gets, with the start's nonce added (none of the core's own unless it is in env); a nonce
// already in env is dropped. The command's ExtraFiles are s's handshake: the caller adds none, sets its folder, starts
// it, then calls s.Wait.
//
// It goes through disclaim.Cmd, so in Smolcorp.app smolcorp-spawn runs first, and with none there nothing starts
// (cmd.Err). Before that, sandbox-exec must pass CheckSandboxExec and the profile must be a file, not a link: else
// ErrNoSandboxExec, and nothing is made.
func MacCommand(s *Start, profile, smolcorp string, env []string, program string, args ...string) (*exec.Cmd, error) {
	if s == nil || !filepath.IsAbs(profile) || !filepath.IsAbs(smolcorp) || !filepath.IsAbs(program) {
		return nil, ErrStartNotWhole
	}
	if err := CheckSandboxExec(); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(profile); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: the profile %s isn't a file (%v)", ErrNoSandboxExec, profile, err)
	}
	words := append([]string{"-f", profile, smolcorp}, s.Words(program, args...)...)
	cmd := disclaim.Cmd(exec.Command(SandboxExecPath, words...))
	cmd.Env = append(slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		return strings.HasPrefix(kv, nonceVar+"=")
	}), s.Env())
	cmd.ExtraFiles = s.Files()
	return cmd, nil
}

// OtherServerIn is a fresh place for a start's stand-in other server (Checks.Other) in folder: a random name, so a
// socket a desk planted where it can write can't refuse every start by being there first (RT126SAX3 nit 3). folder
// should be one no desk can write.
func OtherServerIn(folder string) string {
	word := make([]byte, 8)
	rand.Read(word)
	return filepath.Join(folder, "other-"+hex.EncodeToString(word))
}

// serverPeer is what the core reads of the process listening at a desk's socket.
type serverPeer struct {
	uid uint32 // the user it listened as (LOCAL_PEERCRED)
	pid int    // its pid when it listened (LOCAL_PEERPID)
	// tokenPID is the pid its audit token names now (LOCAL_PEERTOKEN), and denied macOS's answer, asked by that
	// token, to whether it may read the canary: 1 denied, 0 allowed, below 0 not answered.
	tokenPID, denied int
}

// readServerPeer reads the peer of a connection to a desk's socket and asks macOS about it. A stand-in in the tests.
var readServerPeer = func(fd int, canary string) (serverPeer, error) {
	cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return serverPeer{}, err
	}
	if cred.Version != 0 {
		return serverPeer{}, fmt.Errorf("LOCAL_PEERCRED's answer has version %d", cred.Version)
	}
	pid, err := unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	if err != nil {
		return serverPeer{}, err
	}
	tokenPID, denied := deniesCanary(fd, canary)
	return serverPeer{uid: cred.Uid, pid: pid, tokenPID: tokenPID, denied: denied}, nil
}

// ServerIsOurs is §A5.2 step 9 on a Mac: the server listening at socket, the desk's own after its start, is in the
// sandbox of the start whose canary is c (T10). It gives the server's pid, for the desk's record and the guarded
// clients, or a *Refused (the owner sees NotOurServerRefusal):
//   - socket must be a socket, the owner's, not a link;
//   - the server must answer within a few seconds;
//   - it listened as this user, and its audit token names the pid it listened with;
//   - asked by that token, macOS must say it may not read c (file-read-data): a server in no sandbox, or in another
//     sandbox that lets the canary be read, is refused.
func ServerIsOurs(socket string, c Canary) (int, error) {
	refuse := func(why string) (int, error) {
		return 0, &Refused{Why: "the server at " + socket + ": " + why}
	}
	if !filepath.IsAbs(c.Path) {
		return refuse("no canary to check it against")
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
	var peer serverPeer
	var readErr error
	if err := raw.Control(func(fd uintptr) { peer, readErr = readServerPeer(int(fd), c.Path) }); err != nil || readErr != nil {
		return refuse(fmt.Sprintf("who listens can't be read: %v", errors.Join(err, readErr)))
	}
	switch {
	case peer.uid != uint32(os.Getuid()):
		return refuse(fmt.Sprintf("it listens as user %d", peer.uid))
	case peer.pid <= 0 || peer.tokenPID != peer.pid:
		return refuse(fmt.Sprintf("it listened as pid %d, and its token names pid %d", peer.pid, peer.tokenPID))
	case peer.denied < 0:
		return refuse("macOS didn't say whether it is in a sandbox")
	case peer.denied == 0:
		return refuse(fmt.Sprintf("pid %d may read the canary: it is in no sandbox, or in one that isn't this start's", peer.pid))
	}
	return peer.pid, nil
}

// ChildIsOurs is ServerIsOurs for a desk with no server, whose first process is the core's own child: an ACP desk's
// agent, or a terminal its agent asked for (SA-M11). Once that child's handshake has passed, macOS, asked by its pid,
// must say it may not read c; anything else is a *Refused. pid must be the core's child, started and not yet waited
// for: until it is waited for, its pid can't be given to another process.
func ChildIsOurs(pid int, c Canary) error {
	if pid <= 1 || !filepath.IsAbs(c.Path) {
		return &Refused{Why: fmt.Sprintf("pid %d can't be checked against canary %q", pid, c.Path)}
	}
	switch childDeniesCanary(pid, c.Path) {
	case 1:
		return nil
	case 0:
		return &Refused{Why: fmt.Sprintf("pid %d may read the canary: it is in no sandbox, or in one that isn't this start's", pid)}
	}
	return &Refused{Why: fmt.Sprintf("macOS didn't say whether pid %d is in a sandbox", pid)}
}
