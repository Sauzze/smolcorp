//go:build unix

package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// HasCanary: `enter` checks a sandbox here, so a start has a canary (NewCanary).
const HasCanary = true

// fileID is a file's device and inode, as `enter` sees them inside.
func fileID(info fs.FileInfo) (dev, ino uint64, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(st.Dev), uint64(st.Ino), true
}

// system is what `enter`'s rows ask of the system: the real one inside a desk's sandbox, a stand-in in the tests,
// which fake each check's outcome.
type system interface {
	lstat(path string) (fs.FileInfo, error)
	// open opens path with flag, reads a byte when it's for reading, and closes it.
	open(path string, flag int) error
	remove(path string) error
	connect(path string) error
	// fsType is the file system path is on ("tmpfs"), "" when it can't be told.
	fsType(path string) string
	// descriptors are the process's open descriptors; closesAtExec, whether fd is marked to close at an exec.
	descriptors() ([]int, error)
	closeAtExec(fd int) error
	closesAtExec(fd int) (bool, error)
	// ioctl makes an ioctl call with request on fd and gives its error (Linux; ENOSYS elsewhere).
	ioctl(fd int, request uint64) error
	// listen listens on a unix socket at name ("@…": an abstract one); landlock applies Landlock's abstract-socket
	// scope to the calling thread (Linux; ENOSYS elsewhere).
	listen(name string) (io.Closer, error)
	landlock() error
	// forward starts the forwarder to the proxy's socket, listening at addr, and gives where it listens (Linux;
	// ENOSYS elsewhere).
	forward(socket, addr string) (string, error)
}

type realSystem struct{}

func (realSystem) lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }
func (realSystem) remove(path string) error               { return os.Remove(path) }

func (realSystem) open(path string, flag int) error {
	f, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		if _, err := f.Read(make([]byte, 1)); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

func (realSystem) listen(name string) (io.Closer, error) { return net.Listen("unix", name) }

func (realSystem) connect(path string) error {
	c, err := net.Dial("unix", path)
	if err == nil {
		c.Close()
	}
	return err
}

func (realSystem) descriptors() ([]int, error) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return nil, err
	}
	var fds []int
	for _, entry := range entries {
		if fd, err := strconv.Atoi(entry.Name()); err == nil {
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

func (realSystem) closeAtExec(fd int) error {
	_, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.FD_CLOEXEC)
	return err
}

func (realSystem) closesAtExec(fd int) (bool, error) {
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	return flags&unix.FD_CLOEXEC != 0, err
}

// errnoWord is an error's errno name ("EPERM"), or the error's words.
func errnoWord(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if name := unix.ErrnoName(errno); name != "" {
			return name
		}
	}
	return err.Error()
}

// deniedWith is the error a sandbox gives a denied open on goos: EPERM from a Mac's profile (S8), EROFS from Linux's
// read-only view.
func deniedWith(goos string) syscall.Errno {
	if goos == "darwin" {
		return syscall.EPERM
	}
	return syscall.EROFS
}

// check runs one row on sys, as the sandbox on goos must answer it, and gives its result: passed, or why not.
func check(goos string, sys system, e entering, r checkRow) string {
	failed := func(why string) string { return "failed " + strconv.Quote(why) }
	switch r.kind {
	case rowCanary:
		switch goos {
		case "darwin":
			// Its metadata stays readable: it must be this start's canary, and opening it must give EPERM, not ENOENT.
			info, err := sys.lstat(r.path)
			if err != nil {
				return failed("not seen: " + errnoWord(err))
			}
			if dev, ino, ok := fileID(info); !ok || dev != e.dev || ino != e.ino {
				return failed("another file than this start's canary")
			}
			if err := sys.open(r.path, os.O_RDONLY); !errors.Is(err, syscall.EPERM) {
				return failed("read: " + said(err) + ", not EPERM")
			}
		case "linux":
			// The support folder seen inside is a tmpfs on another device than the core's, and the canary isn't there.
			info, err := sys.lstat(filepath.Dir(r.path))
			if err != nil {
				return failed("the support folder isn't seen: " + errnoWord(err))
			}
			if dev, _, ok := fileID(info); !ok || dev == e.dev || sys.fsType(filepath.Dir(r.path)) != "tmpfs" {
				return failed("the support folder seen is the core's, not a tmpfs of its own")
			}
			if _, err := sys.lstat(r.path); !errors.Is(err, syscall.ENOENT) {
				return failed("seen: " + said(err) + ", not ENOENT")
			}
		default:
			return failed("no sandbox on " + goos)
		}
	case rowNoWrite:
		if _, err := sys.lstat(r.path); err != nil {
			return failed("not there to check: " + errnoWord(err))
		}
		if want := deniedWith(goos); !errors.Is(sys.open(r.path, os.O_WRONLY), want) {
			return failed("opened for writing, or not with " + unix.ErrnoName(want))
		}
	case rowNoConnect:
		if err := sys.connect(r.path); err == nil {
			return failed("connected")
		} else if goos == "darwin" && !errors.Is(err, syscall.EPERM) {
			// On a Mac the socket is there for the desk to see: only the profile may stop the connect.
			return failed("connect: " + errnoWord(err) + ", not EPERM")
		}
	case rowRead:
		if err := sys.open(r.path, os.O_RDONLY|syscall.O_NOFOLLOW); err != nil {
			return failed("not read: " + errnoWord(err))
		}
	case rowWrite:
		// Its status: opened to add to it, nothing written. One that wasn't there is removed again.
		_, err := sys.lstat(r.path)
		absent := errors.Is(err, fs.ErrNotExist)
		if err := sys.open(r.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|syscall.O_NOFOLLOW); err != nil {
			return failed("not written: " + errnoWord(err))
		}
		if absent {
			if err := sys.remove(r.path); err != nil {
				return failed("made, then not removed: " + errnoWord(err))
			}
		}
	case rowSeccomp:
		// The filter bwrap loaded refuses both requests before the kernel looks at the descriptor: on none at all (-1)
		// each gives EPERM, not EBADF, high bits or not. Another request still gives EBADF: it refuses no more.
		for _, request := range []uint64{requestTIOCSTI, requestTIOCLINUX, 1<<32 | requestTIOCSTI} {
			if err := sys.ioctl(-1, request); !errors.Is(err, syscall.EPERM) {
				return failed("ioctl 0x" + strconv.FormatUint(request, 16) + ": " + gave(err) + ", not EPERM")
			}
		}
		if err := sys.ioctl(-1, requestTCGETS); !errors.Is(err, syscall.EBADF) {
			return failed("another ioctl: " + gave(err) + ", not EBADF")
		}
	case rowLandlock:
		// A socket made before the scope, so outside it, at a fresh abstract name: once the scope is on, connecting to
		// it gives EPERM. Without the scope it would connect. Either way it is closed again.
		word := make([]byte, 8)
		rand.Read(word)
		name := "@smolcorp-enter-" + hex.EncodeToString(word)
		l, err := sys.listen(name)
		if err != nil {
			return failed("no socket to try it on: " + errnoWord(err))
		}
		defer l.Close()
		if err := sys.landlock(); err != nil {
			return failed("not applied: " + errnoWord(err))
		}
		if err := sys.connect(name); !errors.Is(err, syscall.EPERM) {
			return failed("an abstract socket outside it: " + gave(err) + ", not EPERM")
		}
	case rowProxy:
		// The way out: the forwarder listens on the view's own loopback, and what answers through it is the core's
		// proxy, which refuses the computer's own address.
		at, err := sys.forward(r.path, "127.0.0.1:"+strconv.Itoa(ForwardPort))
		if err != nil {
			return failed("the forwarder didn't start: " + errnoWord(err))
		}
		if err := wayOutHolds(at); err != nil {
			return failed(err.Error())
		}
	case rowFDs:
		// Every descriptor above 2 closes at the exec. The handshake is closed before it, after the report.
		fds, err := sys.descriptors()
		if err != nil {
			return failed("the descriptors can't be listed: " + errnoWord(err))
		}
		for _, fd := range fds {
			if fd > 2 {
				if err := sys.closeAtExec(fd); err != nil && !errors.Is(err, syscall.EBADF) {
					return failed("descriptor " + strconv.Itoa(fd) + ": " + errnoWord(err))
				}
			}
		}
		if fds, err = sys.descriptors(); err != nil {
			return failed("the descriptors can't be listed again: " + errnoWord(err))
		}
		for _, fd := range fds {
			if closes, err := sys.closesAtExec(fd); fd > 2 && !closes && !errors.Is(err, syscall.EBADF) {
				return failed("descriptor " + strconv.Itoa(fd) + " would reach the program")
			}
		}
	default:
		return failed("a row it doesn't know")
	}
	return passed
}

// gave is a call's outcome in a word: "no error", or the errno's name.
func gave(err error) string {
	if err == nil {
		return "no error"
	}
	return errnoWord(err)
}

// said is an open's outcome in a word: "opened", or the errno's name.
func said(err error) string {
	if err == nil {
		return "opened"
	}
	return errnoWord(err)
}

// runRows runs e's rows in order and gives the results, and whether every one passed.
func runRows(goos string, sys system, e entering) ([]string, bool) {
	results, held := make([]string, len(e.rows)), true
	for i, r := range e.rows {
		results[i] = check(goos, sys, e, r)
		held = held && results[i] == passed
	}
	return results, held
}

// enter is `smolcorp sandbox enter` (above, in enter.go): the rows, the report, then the exec.
func enter(args []string, stderr io.Writer) int {
	e, err := parseEnter(args)
	if err != nil {
		return refuseHere(stderr, err.Error())
	}
	// Landlock's scope (its row) is the calling thread's: the rows and the exec run on this one alone, never unlocked.
	runtime.LockOSThread()
	nonce := os.Getenv(nonceVar)
	os.Unsetenv(nonceVar)
	// The handshake first closes at the exec, whatever happens next.
	if _, err := unix.FcntlInt(uintptr(e.handshake), unix.F_SETFD, unix.FD_CLOEXEC); err != nil {
		return refuseHere(stderr, "no handshake: "+errnoWord(err))
	}
	handshake := os.NewFile(uintptr(e.handshake), "handshake")
	if nonce == "" || strings.ContainsAny(nonce, " \n") {
		handshake.Close()
		return refuseHere(stderr, "no nonce for this start")
	}
	results, held := runRows(runtime.GOOS, realSystem{}, e)
	_, err = handshake.Write(report(nonce, e.rows, results))
	if closeErr := handshake.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return refuseHere(stderr, "the report wasn't given: "+err.Error())
	}
	if !held {
		for i, result := range results {
			if result != passed {
				return refuseHere(stderr, e.rows[i].line(result))
			}
		}
	}
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, nonceVar+"=") || strings.HasPrefix(kv, tmuxGuardVar+"=")
	})
	// A start's tmux starts the desk's server, its guard off for that call alone; an attach's client is told the server
	// the core checked, and talks to no other (§A3.1).
	guard := "off"
	if e.attach > 1 {
		guard = strconv.Itoa(e.attach)
	}
	env = append(env, tmuxGuardVar+"="+guard)
	if slices.ContainsFunc(e.rows, func(r checkRow) bool { return r.kind == rowProxy }) {
		env = ProxyEnv(env, ForwardPort) // its way out, checked by its row
	}
	err = syscall.Exec(e.program, e.args, env)
	// The core read a passing report; with no server after it, its server check refuses the start (§A5.2 step 9).
	return refuseHere(stderr, e.program+" didn't start: "+err.Error())
}
