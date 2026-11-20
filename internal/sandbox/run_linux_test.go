package sandbox

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// SA-L2 on a Linux kernel (CI's ubuntu and the VM; none of it runs on a Mac): the runner's command and what it hands
// bwrap, bwrap's info read for the view's namespaces, the server check against the kernel's SO_PEERCRED, and the
// filter and Landlock's scope as `enter`'s rows see them. Only stand-ins: a stand-in bwrap, this test's own listener,
// a filter and a scope on one thread of this test that ends with it. The whole view under Smolcorp's real bwrap is the
// VM's (the report's RUN notes).

// standInBwrapChecked makes LinuxCommand take bwrap as checked for the test.
func standInBwrapChecked(t *testing.T, err error) {
	t.Helper()
	was := checkBwrapNow
	checkBwrapNow = func() error { return err }
	t.Cleanup(func() { checkBwrapNow = was })
}

// The start's words: Smolcorp's bwrap, the filter on 4 and the info on 5, the view as PrepareBwrap gave it, then
// `enter` with its rows and the program. Its descriptors are the handshake, the filter (whole, and nothing more), and
// the info's end. The nonce is this start's alone.
func TestALinuxStartIsBwrapWithTheFilterTheViewAndEnter(t *testing.T) {
	standInBwrapChecked(t, nil)
	d := newStandInDesk(t)
	c := d.checks
	c.Seccomp = true
	s := mustStart(t, c)
	view := []string{"--unshare-pid", "--unshare-net", "--bind", "/", "/", "--chdir", d.root, "--"}
	cmd, err := LinuxCommand(s, view, "/opt/smolcorp/smolcorp", []string{"HOME=" + d.home, nonceVar + "=old"}, "/usr/bin/tmux", "-S", "/s", "new-session")
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]string{BwrapPath, "--seccomp", "4", "--info-fd", "5"}, view...), "/opt/smolcorp/smolcorp")
	want = append(want, s.Words("/usr/bin/tmux", "-S", "/s", "new-session")...)
	if cmd.Path != BwrapPath || !slices.Equal(cmd.Args, want) {
		t.Errorf("the start:\n%q\nwant\n%q", cmd.Args, want)
	}
	if !slices.Contains(cmd.Args, "--seccomp") || cmd.Args[slices.Index(cmd.Args, "enter")+1] != "--handshake" {
		t.Errorf("the start's words: %q", cmd.Args)
	}
	if !slices.Equal(cmd.Env, []string{"HOME=" + d.home, s.Env()}) {
		t.Errorf("the start's environment: %q", cmd.Env)
	}
	if len(cmd.ExtraFiles) != 3 || cmd.ExtraFiles[0] != s.write || cmd.ExtraFiles[1] != s.given[0] || cmd.ExtraFiles[2] != s.given[1] {
		t.Fatalf("the start's descriptors: %v", cmd.ExtraFiles)
	}
	filter, _ := DeskFilter(runtime.GOARCH)
	got, err := io.ReadAll(cmd.ExtraFiles[1])
	if err != nil || !bytes.Equal(got, filter) {
		t.Errorf("descriptor 4 holds %d bytes (%v), want the filter's %d", len(got), err, len(filter))
	}

	// Refused, with nothing given to any bwrap: a start without the filter's row, a view not ended, a program by name,
	// a second command for one start, and a bwrap that isn't fit (its own error, for LinuxStartRefusal).
	plain := mustStart(t, d.checks)
	for name, try := range map[string]struct {
		s                 *Start
		view              []string
		smolcorp, program string
	}{
		"no seccomp row":     {plain, view, "/s/smolcorp", "/usr/bin/tmux"},
		"a view not ended":   {mustStart(t, c), view[:2], "/s/smolcorp", "/usr/bin/tmux"},
		"a program by name":  {mustStart(t, c), view, "/s/smolcorp", "tmux"},
		"smolcorp not whole": {mustStart(t, c), view, "smolcorp", "/usr/bin/tmux"},
		"a second command":   {s, view, "/s/smolcorp", "/usr/bin/tmux"},
	} {
		if _, err := LinuxCommand(try.s, try.view, try.smolcorp, nil, try.program); !errors.Is(err, ErrStartNotWhole) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if plain.info != nil || plain.given != nil {
		t.Error("a refused start was given bwrap's descriptors")
	}
	standInBwrapChecked(t, errStaleBwrap)
	fresh := mustStart(t, c)
	if _, err := LinuxCommand(fresh, view, "/s/smolcorp", nil, "/usr/bin/tmux"); !errors.Is(err, errStaleBwrap) ||
		LinuxStartRefusal(err) != RefusalFor(errStaleBwrap) || fresh.info != nil {
		t.Errorf("an unfit bwrap: %v", err)
	}
}

// standInBwrap is a program in place of bwrap that runs script: the command LinuxCommand made, run by it.
func standInBwrap(t *testing.T, s *Start, d standInDesk, script string) func() {
	t.Helper()
	standInBwrapChecked(t, nil)
	cmd, err := LinuxCommand(s, []string{"--"}, "/s/smolcorp", nil, "/usr/bin/tmux")
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(d.base, "bwrap")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"+script), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd.Path = program
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() { cmd.Process.Kill(); cmd.Wait() }
	t.Cleanup(stop)
	return stop
}

// The view's namespaces are those of the child bwrap's info names, read as soon as it says it (bwrap's process may keep
// the descriptor open). A bwrap that says nothing, or exits, is refused.
func TestTheViewsNamespacesAreThoseOfBwrapsChild(t *testing.T) {
	d := newStandInDesk(t)
	c := d.checks
	c.Seccomp = true
	s := mustStart(t, c)
	standInBwrap(t, s, d, "printf '{\\n    \"child-pid\": %d\\n}\\n' $$ >&5\nexec sleep 30\n")
	got, err := s.Namespaces(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	mine, err := namespacesOf(os.Getpid()) // the stand-in makes none: its namespaces are this test's
	if err != nil || got != mine || got.Pid.Ino == 0 || got.Mnt.Ino == 0 {
		t.Errorf("the view's namespaces %+v, this test's %+v (%v)", got, mine, err)
	}
	for name, script := range map[string]string{"exits": "exit 1\n", "says nothing": "exec sleep 30\n", "names no child": "echo '{}' >&5\nexec sleep 30\n"} {
		s := mustStart(t, c)
		stop := standInBwrap(t, s, d, script)
		began := time.Now()
		var refused *Refused
		if _, err := s.Namespaces(time.Second); !errors.As(err, &refused) || time.Since(began) > 5*time.Second {
			t.Errorf("%s: %v after %v", name, err, time.Since(began))
		}
		stop()
	}
}

// The server check on the kernel: this test's own listener passes against its own namespaces and fails against any
// other; a file that isn't a socket, or no socket, fails before anything connects.
func TestTheServerCheckReadsTheKernelsPeer(t *testing.T) {
	d := newStandInDesk(t)
	socket := filepath.Join(d.base, "s")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
		}
	}()
	mine, err := namespacesOf(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if pid, err := ServerInView(socket, mine); err != nil || pid != os.Getpid() {
		t.Fatalf("this test's server: %d, %v", pid, err)
	}
	another := mine
	another.Pid.Ino++
	notSocket := filepath.Join(d.base, "f")
	os.WriteFile(notSocket, nil, 0o600)
	for name, at := range map[string]struct {
		socket string
		view   Namespaces
	}{
		"another view": {socket, another}, "no view": {socket, Namespaces{}},
		"not a socket": {notSocket, mine}, "no socket": {filepath.Join(d.base, "none"), mine},
	} {
		if _, err := ServerInView(at.socket, at.view); !errors.Is(err, ErrNotOurServer) || LinuxStartRefusal(err) != NotOurServerRefusal {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// onItsOwnThread runs f on a thread of its own that ends with it: what f does to its thread (a filter, a scope) goes
// with it, and nothing else of the test has it.
func onItsOwnThread(f func()) {
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread() // never unlocked: the thread ends when this does
		defer close(done)
		f()
	}()
	<-done
}

// The filter on the kernel: once loaded, `enter`'s seccomp row passes (TIOCSTI and TIOCLINUX get EPERM on no
// descriptor at all, high bits or not, and another request EBADF); without it the row fails.
func TestTheKernelRefusesTheTwoIoctlsThroughTheFilter(t *testing.T) {
	filter, err := DeskFilter(runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	row := checkRow{kind: rowSeccomp}
	if got := check("linux", realSystem{}, entering{}, row); got == passed {
		t.Fatal("the row passed with no filter")
	}
	var got string
	var loadErr error
	onItsOwnThread(func() {
		instructions := make([]unix.SockFilter, len(filter)/8)
		for i := range instructions {
			in := filter[i*8:]
			instructions[i] = unix.SockFilter{Code: binary.NativeEndian.Uint16(in), Jt: in[2], Jf: in[3], K: binary.NativeEndian.Uint32(in[4:])}
		}
		program := unix.SockFprog{Len: uint16(len(instructions)), Filter: &instructions[0]}
		if loadErr = unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); loadErr != nil {
			return
		}
		if _, _, errno := unix.Syscall(unix.SYS_SECCOMP, 1 /* SECCOMP_SET_MODE_FILTER */, 0, uintptr(unsafe.Pointer(&program))); errno != 0 {
			loadErr = errno
			return
		}
		got = check("linux", realSystem{}, entering{}, row)
	})
	if loadErr != nil {
		t.Fatalf("the filter wasn't loaded: %v", loadErr)
	}
	if got != passed {
		t.Errorf("with the filter: %s", got)
	}
}

// Landlock's scope on the kernel, where it has it: once applied, `enter`'s row passes (the abstract socket made before
// it can't be reached); without it, the same socket is reached. A kernel without the scope skips, saying its ABI.
func TestTheKernelsLandlockScopeHoldsWhereItHasIt(t *testing.T) {
	ok, why := LandlockScope()
	if !ok {
		t.Skip(why)
	}
	name := "@smolcorp-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	l, err := net.Listen("unix", name)
	if err != nil {
		t.Fatal(err)
	}
	if err := (realSystem{}).connect(name); err != nil {
		t.Fatalf("without the scope, the socket isn't reached: %v", err)
	}
	l.Close()
	var got string
	onItsOwnThread(func() { got = check("linux", realSystem{}, entering{}, checkRow{kind: rowLandlock}) })
	if got != passed {
		t.Errorf("with the scope: %s", got)
	}
}
