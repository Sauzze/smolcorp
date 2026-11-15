//go:build unix

package sandbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-X3's rows (PL126A §2.1): the canary fresh per start, written new then renamed; the positive controls; every
// descriptor above 2 closed at the exec; the handshake carries the nonce, and the core takes only a full, passing
// report. Each check faked is refused. The real Mac sandbox runs in enter_darwin_test.go.

// enterTestVar makes this test program `smolcorp sandbox enter` (argument "sandbox") or the desk's stand-in program
// (argument "standin"), as the core would start them.
const enterTestVar = "SMOLCORP_SANDBOX_ENTER_TEST"

func TestMain(m *testing.M) {
	if os.Getenv(enterTestVar) == "1" && len(os.Args) > 1 {
		switch os.Args[1] {
		case "sandbox":
			os.Exit(Run(os.Args[2:], os.Stderr))
		case "standin":
			standIn(os.Args[2])
			os.Exit(0)
		case "serve":
			standInServer(os.Args[2])
		}
	}
	os.Exit(m.Run())
}

// standIn is the desk's program: it writes which descriptors above 2 it holds, and what it was given of the nonce and
// the tmux guard's variable, to the file at out. It touches nothing else.
func standIn(out string) {
	var fds []int
	for fd := 3; fd < 256; fd++ {
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil {
			fds = append(fds, fd)
		}
	}
	_, nonce := os.LookupEnv(nonceVar)
	data, _ := json.Marshal(map[string]any{"fds": fds, "nonce": nonce, "guard": os.Getenv(tmuxGuardVar)})
	// Its own descriptor for the file is opened after the list is made.
	os.WriteFile(out, data, 0o600)
}

// standInServer is a desk's stand-in server (SA-M3): it listens at the socket path, says so on stdout, and takes
// connections and keeps them open, as tmux keeps its clients', until it's killed, or for a minute at most. (A server
// that hangs up at once has no audit token left to read, and the server check refuses it.)
func standInServer(socket string) {
	l, err := net.Listen("unix", socket)
	if err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		os.Exit(3)
	}
	fmt.Println("listening")
	go func() {
		time.Sleep(time.Minute)
		os.Exit(4)
	}()
	var open []net.Conn
	for {
		c, err := l.Accept()
		if err != nil {
			os.Exit(5)
		}
		open = append(open, c)
	}
}

// shortTemp is a temp folder short enough for a unix socket's path (104 bytes on a Mac), links resolved.
func shortTemp(t *testing.T) string {
	t.Helper()
	made, err := os.MkdirTemp("", "sx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	return base
}

// standInDesk is a stand-in desk's files, all in a temp folder: the support folder with the guard file and the
// desk's brief, its worktree with a .claude/settings.json. Its Checks are the ones the core would make.
type standInDesk struct {
	base, home, support, own, root string
	checks                         Checks
}

func newStandInDesk(t *testing.T) standInDesk {
	t.Helper()
	base := shortTemp(t)
	d := standInDesk{base: base, home: filepath.Join(base, "h")}
	d.support = filepath.Join(d.home, "AD")
	d.own = filepath.Join(d.support, "hires", "h1")
	d.root = filepath.Join(d.home, "app-hires", "h1")
	for at, text := range map[string]string{
		filepath.Join(d.support, "hire-guard.json"): `{"fence":1}`, filepath.Join(d.own, "BRIEF.md"): "the brief",
		filepath.Join(d.root, ".claude", "settings.json"): "{}", filepath.Join(d.root, "main.go"): "package main",
	} {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d.checks = Checks{Support: d.support,
		NoWrite: []string{filepath.Join(d.support, "hire-guard.json"), filepath.Join(d.own, "BRIEF.md"), filepath.Join(d.root, ".claude", "settings.json")},
		Other:   filepath.Join(d.support, "o"), Read: []string{filepath.Join(d.own, "BRIEF.md")}, Write: []string{filepath.Join(d.own, "STATUS.md")}}
	return d
}

// startsMade numbers each test start's other server, which must be new.
var startsMade atomic.Int64

func mustStart(t *testing.T, c Checks) *Start {
	t.Helper()
	c.Other = filepath.Join(filepath.Dir(c.Other), fmt.Sprintf("o%d", startsMade.Add(1)))
	s, err := NewStart(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(); s.Canary.Remove() })
	return s
}

func TestTheCanaryIsFreshForEachStartWrittenNewThenRenamed(t *testing.T) {
	support := shortTemp(t)
	a, err := NewCanary(support)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewCanary(support)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path || a.Ino == b.Ino {
		t.Errorf("two starts share a canary: %+v %+v", a, b)
	}
	for _, c := range []Canary{a, b} {
		if row, ok := SupportRow(filepath.Base(c.Path)); !ok || !strings.Contains(row, "canary") {
			t.Errorf("%s has no canary row in the Policy: %q", c.Path, row)
		}
		var st syscall.Stat_t
		if err := syscall.Lstat(c.Path, &st); err != nil || uint64(st.Dev) != c.Dev || st.Ino != c.Ino || st.Nlink != 1 ||
			st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Mode&0o777 != 0o600 {
			t.Errorf("the canary %s: %+v %v", c.Path, st, err)
		}
		if data, _ := os.ReadFile(c.Path); string(data) != canaryText {
			t.Errorf("the canary holds %q", data)
		}
	}
	if names, _ := os.ReadDir(support); len(names) != 2 {
		t.Errorf("a new file was left beside the canaries: %v", names)
	}

	// Something already at the name, a file or a link: no canary is made, and it is left as it is.
	planted := filepath.Join(support, "sandbox-canary-planted")
	os.WriteFile(planted, []byte("planted"), 0o600)
	if _, err := makeCanary(planted); err == nil {
		t.Error("a canary was made over a file already there")
	}
	linked := filepath.Join(support, "sandbox-canary-linked")
	os.Symlink(filepath.Join(support, "elsewhere"), linked+".new")
	if _, err := makeCanary(linked); err == nil {
		t.Error("a canary was written through a link at its new name")
	}
	if _, err := os.Lstat(filepath.Join(support, "elsewhere")); err == nil {
		t.Error("the link at the new name was followed")
	}
	if data, _ := os.ReadFile(planted); string(data) != "planted" {
		t.Errorf("the file already there was changed: %q", data)
	}

	// Remove takes only the canary the core made: one put in its place stays.
	if err := a.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(a.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the canary wasn't removed")
	}
	os.Remove(b.Path)
	os.WriteFile(b.Path, []byte("another"), 0o600)
	if err := b.Remove(); err == nil {
		t.Error("a file put in the canary's place was removed as the canary")
	}
	if a.Remove() != nil {
		t.Error("removing a canary already gone failed")
	}
}

// give writes data on the handshake as `enter` would, from its own copy of the pipe, and closes that copy.
func give(t *testing.T, s *Start, data []byte) {
	t.Helper()
	fd, err := syscall.Dup(int(s.write.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	f := os.NewFile(uintptr(fd), "enter's end")
	go func() { f.Write(data); f.Close() }()
}

// passingResults are every row's "ok".
func passingResults(n int) []string {
	results := make([]string, n)
	for i := range results {
		results[i] = passed
	}
	return results
}

func TestTheCoreTakesOnlyAFullPassingReportForThisStart(t *testing.T) {
	d := newStandInDesk(t)
	if err := mustStart(t, d.checks).Wait(time.Second); err == nil {
		t.Fatal("a start with no report at all passed")
	}
	s := mustStart(t, d.checks)
	give(t, s, report(s.nonce, s.rows, passingResults(len(s.rows))))
	if err := s.Wait(5 * time.Second); err != nil {
		t.Fatalf("a full, passing report was refused: %v", err)
	}

	type faked struct {
		name   string
		report func(s *Start) []byte
	}
	cases := []faked{
		{"another start's nonce", func(s *Start) []byte {
			return report(strings.Repeat("0", 32), s.rows, passingResults(len(s.rows)))
		}},
		{"no nonce", func(s *Start) []byte { return report("", s.rows, passingResults(len(s.rows))) }},
		{"a row left out", func(s *Start) []byte { return report(s.nonce, s.rows[1:], passingResults(len(s.rows)-1)) }},
		{"the rows in another order", func(s *Start) []byte {
			rows := append([]checkRow{s.rows[1], s.rows[0]}, s.rows[2:]...)
			return report(s.nonce, rows, passingResults(len(rows)))
		}},
		{"a row added", func(s *Start) []byte {
			rows := append(append([]checkRow(nil), s.rows...), checkRow{rowRead, "/etc/hosts"})
			return report(s.nonce, rows, passingResults(len(rows)))
		}},
		{"a row on another path", func(s *Start) []byte {
			rows := append([]checkRow(nil), s.rows...)
			rows[1].path = "/tmp/elsewhere"
			return report(s.nonce, rows, passingResults(len(rows)))
		}},
		{"no end", func(s *Start) []byte {
			full := report(s.nonce, s.rows, passingResults(len(s.rows)))
			return bytes.TrimSuffix(full, []byte(reportEnd+"\n"))
		}},
		{"more after the end", func(s *Start) []byte {
			return append(report(s.nonce, s.rows, passingResults(len(s.rows))), "canary \"\" ok\n"...)
		}},
		{"too long", func(s *Start) []byte {
			return append(report(s.nonce, s.rows, passingResults(len(s.rows))), bytes.Repeat([]byte("x"), reportMost)...)
		}},
		{"half a report", func(s *Start) []byte { return report(s.nonce, s.rows, passingResults(len(s.rows)))[:40] }},
	}
	for i := range len(s.rows) { // a row failed, each in turn
		cases = append(cases, faked{fmt.Sprintf("row %d (%s) failed", i, s.rows[i].kind), func(s *Start) []byte {
			results := passingResults(len(s.rows))
			results[i] = `failed "faked"`
			return report(s.nonce, s.rows, results)
		}})
	}
	for _, c := range cases {
		s := mustStart(t, d.checks)
		give(t, s, c.report(s))
		var refused *Refused
		if err := s.Wait(5 * time.Second); !errors.As(err, &refused) {
			t.Errorf("%s: %v, want refused", c.name, err)
		} else {
			t.Logf("%s: %v", c.name, err)
		}
	}

	// `enter` still running, saying nothing: refused when the wait is over.
	s = mustStart(t, d.checks)
	fd, err := syscall.Dup(int(s.write.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	if err := s.Wait(200 * time.Millisecond); err == nil {
		t.Error("a start whose `enter` never reported passed")
	}
}

func TestAStartChecksNothingLessThanItsRows(t *testing.T) {
	d := newStandInDesk(t)
	for name, change := range map[string]func(c *Checks){
		"no fence": func(c *Checks) { c.NoWrite = nil }, "no other server": func(c *Checks) { c.Other = "" },
		"nothing to read": func(c *Checks) { c.Read = nil }, "nothing to write": func(c *Checks) { c.Write = nil },
		"a fence that isn't there": func(c *Checks) { c.NoWrite = []string{filepath.Join(d.root, "missing")} },
	} {
		c := d.checks
		change(&c)
		if s, err := NewStart(c); err == nil {
			s.Close()
			s.Canary.Remove()
			t.Errorf("%s: the start was made", name)
		}
	}
	// `enter`'s own words: each row's kind needed, the canary first, the program by its whole path.
	s := mustStart(t, d.checks)
	good := s.Words("/bin/echo", "hi")
	if e, err := parseEnter(good[2:]); err != nil || len(e.rows) != len(s.rows) || e.program != "/bin/echo" ||
		strings.Join(e.args, " ") != "/bin/echo hi" || e.handshake != 3 || e.ino != s.Canary.Ino {
		t.Fatalf("the core's words don't read back: %+v %v", e, err)
	}
	without := func(flag string) []string {
		var out []string
		for i := 2; i < len(good); i++ {
			if good[i] == flag {
				i++
				continue
			}
			out = append(out, good[i])
		}
		return out
	}
	for _, flag := range []string{"--canary", "--no-write", "--no-connect", "--read", "--write", "--handshake", "--canary-id"} {
		if _, err := parseEnter(without(flag)); err == nil {
			t.Errorf("enter took its words without %s", flag)
		}
	}
	reordered := append([]string(nil), good[2:]...)
	at := slicesIndex(reordered, "--canary")
	moved := append(append(append([]string(nil), reordered[:at]...), reordered[at+2:len(reordered)-3]...), "--canary", reordered[at+1], "--", "/bin/echo", "hi")
	for name, args := range map[string][]string{
		"the canary not first":   moved,
		"a program on PATH":      append(append([]string(nil), good[2:len(good)-2]...), "echo", "hi"),
		"no program":             good[2 : len(good)-2],
		"a word it doesn't take": append([]string{"--also", "x"}, good[2:]...),
		"a relative path":        append([]string{"--read", "BRIEF.md"}, good[2:]...),
		"the handshake on 2":     append([]string{"--handshake", "2"}, good[2:]...),
	} {
		if _, err := parseEnter(args); err == nil {
			t.Errorf("%s: enter took it", name)
		}
	}
}

func slicesIndex(list []string, word string) int {
	for i, w := range list {
		if w == word {
			return i
		}
	}
	return -1
}

// fakeSystem answers each row as the system says, through its funcs; nil funcs answer as a sandbox that holds.
type fakeSystem struct {
	lstatFn        func(path string) (fs.FileInfo, error)
	openFn         func(path string, flag int) error
	removeFn       func(path string) error
	connectFn      func(path string) error
	fsTypeFn       func(path string) string
	descriptorsFn  func() ([]int, error)
	closeAtExecFn  func(fd int) error
	closesAtExecFn func(fd int) (bool, error)
}

func (f fakeSystem) lstat(path string) (fs.FileInfo, error) { return f.lstatFn(path) }
func (f fakeSystem) open(path string, flag int) error       { return f.openFn(path, flag) }
func (f fakeSystem) remove(path string) error               { return f.removeFn(path) }
func (f fakeSystem) connect(path string) error              { return f.connectFn(path) }
func (f fakeSystem) fsType(path string) string              { return f.fsTypeFn(path) }
func (f fakeSystem) descriptors() ([]int, error)            { return f.descriptorsFn() }
func (f fakeSystem) closeAtExec(fd int) error               { return f.closeAtExecFn(fd) }
func (f fakeSystem) closesAtExec(fd int) (bool, error)      { return f.closesAtExecFn(fd) }

// otherDevice is info as seen on another device: the private view's tmpfs.
type otherDevice struct {
	fs.FileInfo
	st *syscall.Stat_t
}

func (o otherDevice) Sys() any { return o.st }

func onAnotherDevice(info fs.FileInfo) fs.FileInfo {
	st := *info.Sys().(*syscall.Stat_t)
	st.Dev++
	return otherDevice{info, &st}
}

// holding is the system inside a sandbox that holds on goos, for the stand-in desk's start s.
func holding(goos string, s *Start, d standInDesk) fakeSystem {
	denied := deniedWith(goos)
	return fakeSystem{
		lstatFn: func(path string) (fs.FileInfo, error) {
			switch {
			case goos == "linux" && path == s.Canary.Path:
				return nil, syscall.ENOENT
			case goos == "linux" && path == d.support:
				info, err := os.Lstat(path)
				return onAnotherDevice(info), err
			}
			return os.Lstat(path)
		},
		openFn: func(path string, flag int) error {
			switch {
			case path == s.Canary.Path:
				return syscall.EPERM
			case flag&os.O_WRONLY != 0 && slicesIndex(d.checks.NoWrite, path) >= 0:
				return denied
			}
			return nil
		},
		removeFn: func(string) error { return nil },
		connectFn: func(string) error {
			if goos == "darwin" {
				return syscall.EPERM
			}
			return syscall.ENOENT
		},
		fsTypeFn:       func(string) string { return "tmpfs" },
		descriptorsFn:  func() ([]int, error) { return []int{0, 1, 2, 3, 7}, nil },
		closeAtExecFn:  func(int) error { return nil },
		closesAtExecFn: func(int) (bool, error) { return true, nil },
	}
}

// Each of `enter`'s checks, faked to answer as a sandbox that doesn't hold would, fails its row, and the core refuses
// the start. The same rows answered as a holding sandbox pass, on a Mac and on Linux.
func TestEachCheckFakedIsRefused(t *testing.T) {
	d := newStandInDesk(t)
	type fake struct {
		name   string
		row    string
		change func(f *fakeSystem, s *Start)
	}
	for _, goos := range []string{"darwin", "linux"} {
		cases := []fake{
			{"a fence opened for writing", rowNoWrite, func(f *fakeSystem, s *Start) {
				open := f.openFn
				f.openFn = func(p string, fl int) error {
					if p == d.checks.NoWrite[1] {
						return nil
					}
					return open(p, fl)
				}
			}},
			{"a fence denied with another error", rowNoWrite, func(f *fakeSystem, s *Start) {
				f.openFn = func(p string, fl int) error {
					if p == s.Canary.Path {
						return syscall.EPERM
					}
					if fl&os.O_WRONLY != 0 && slicesIndex(d.checks.NoWrite, p) >= 0 {
						return syscall.EACCES
					}
					return nil
				}
			}},
			{"a fence not there", rowNoWrite, func(f *fakeSystem, s *Start) {
				lstat := f.lstatFn
				f.lstatFn = func(p string) (fs.FileInfo, error) {
					if p == d.checks.NoWrite[2] {
						return nil, syscall.ENOENT
					}
					return lstat(p)
				}
			}},
			{"the other server reached", rowNoConnect, func(f *fakeSystem, s *Start) { f.connectFn = func(string) error { return nil } }},
			{"the brief not read", rowRead, func(f *fakeSystem, s *Start) {
				open := f.openFn
				f.openFn = func(p string, fl int) error {
					if p == d.checks.Read[0] && fl&os.O_WRONLY == 0 {
						return syscall.EPERM
					}
					return open(p, fl)
				}
			}},
			{"the status not written", rowWrite, func(f *fakeSystem, s *Start) {
				open := f.openFn
				f.openFn = func(p string, fl int) error {
					if p == d.checks.Write[0] {
						return syscall.EPERM
					}
					return open(p, fl)
				}
			}},
			{"the status made, then not removed", rowWrite, func(f *fakeSystem, s *Start) { f.removeFn = func(string) error { return syscall.EPERM } }},
			{"a descriptor kept past the exec", rowFDs, func(f *fakeSystem, s *Start) {
				f.closeAtExecFn = func(fd int) error {
					if fd == 7 {
						return syscall.EPERM
					}
					return nil
				}
			}},
			{"a descriptor not marked", rowFDs, func(f *fakeSystem, s *Start) {
				f.closesAtExecFn = func(fd int) (bool, error) { return fd != 7, nil }
			}},
			{"the descriptors not listed", rowFDs, func(f *fakeSystem, s *Start) {
				f.descriptorsFn = func() ([]int, error) { return nil, syscall.ENOENT }
			}},
		}
		if goos == "darwin" {
			cases = append(cases,
				fake{"the canary read", rowCanary, func(f *fakeSystem, s *Start) {
					open := f.openFn
					f.openFn = func(p string, fl int) error {
						if p == s.Canary.Path {
							return nil
						}
						return open(p, fl)
					}
				}},
				fake{"the canary absent (ENOENT, not EPERM)", rowCanary, func(f *fakeSystem, s *Start) {
					open := f.openFn
					f.openFn = func(p string, fl int) error {
						if p == s.Canary.Path {
							return syscall.ENOENT
						}
						return open(p, fl)
					}
				}},
				fake{"another file than this start's canary", rowCanary, func(f *fakeSystem, s *Start) {
					lstat := f.lstatFn
					f.lstatFn = func(p string) (fs.FileInfo, error) {
						if p == s.Canary.Path {
							return os.Lstat(d.checks.NoWrite[0])
						}
						return lstat(p)
					}
				}},
				fake{"the other server not there", rowNoConnect, func(f *fakeSystem, s *Start) {
					f.connectFn = func(string) error { return syscall.ENOENT }
				}})
		} else {
			cases = append(cases,
				fake{"the canary seen", rowCanary, func(f *fakeSystem, s *Start) {
					lstat := f.lstatFn
					f.lstatFn = func(p string) (fs.FileInfo, error) {
						if p == s.Canary.Path {
							return os.Lstat(p)
						}
						return lstat(p)
					}
				}},
				fake{"the core's own support folder", rowCanary, func(f *fakeSystem, s *Start) {
					lstat := f.lstatFn
					f.lstatFn = func(p string) (fs.FileInfo, error) {
						if p == d.support {
							return os.Lstat(p)
						}
						return lstat(p)
					}
				}},
				fake{"a support folder that isn't a tmpfs", rowCanary, func(f *fakeSystem, s *Start) {
					f.fsTypeFn = func(string) string { return "" }
				}})
		}

		// A holding sandbox passes, and the core takes its report.
		s := mustStart(t, d.checks)
		e, err := parseEnter(s.Words("/bin/echo")[2:])
		if err != nil {
			t.Fatal(err)
		}
		results, held := runRows(goos, holding(goos, s, d), e)
		if !held {
			t.Fatalf("%s: a holding sandbox failed: %q", goos, results)
		}
		give(t, s, report(s.nonce, e.rows, results))
		if err := s.Wait(5 * time.Second); err != nil {
			t.Fatalf("%s: a holding sandbox's report was refused: %v", goos, err)
		}
		// No sandbox known for this system: the canary row fails.
		if _, held := runRows("freebsd", holding("darwin", s, d), e); held {
			t.Errorf("a system with no sandbox passed")
		}

		for _, c := range cases {
			s := mustStart(t, d.checks)
			e, _ := parseEnter(s.Words("/bin/echo")[2:])
			sys := holding(goos, s, d)
			c.change(&sys, s)
			results, held := runRows(goos, sys, e)
			failedRow := ""
			for i, result := range results {
				if result != passed && failedRow == "" {
					failedRow = e.rows[i].kind
				}
			}
			if held || failedRow != c.row {
				t.Errorf("%s, %s: held %v, first failed row %q, want %q (%q)", goos, c.name, held, failedRow, c.row, results)
				continue
			}
			give(t, s, report(s.nonce, e.rows, results))
			var refused *Refused
			if err := s.Wait(5 * time.Second); !errors.As(err, &refused) {
				t.Errorf("%s, %s: the core took it: %v", goos, c.name, err)
			}
		}
	}
}

// runEnter starts this test program as `smolcorp sandbox enter` for s, through wrap (sandbox-exec on a Mac, or
// nothing), with extra descriptors the core might leak. It gives Wait's answer, enter's stderr, and where the stand-in
// program writes what it was given.
func runEnter(t *testing.T, s *Start, d standInDesk, wrap ...string) (error, string, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(d.root, "given.json")
	words := append(append(wrap, self), s.Words(self, "standin", out)...)
	cmd := disclaim.Cmd(exec.Command(words[0], words[1:]...))
	cmd.Dir = d.root
	cmd.Env = []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", enterTestVar + "=1", s.Env(), tmuxGuardVar + "=4242"}
	leaked, err := os.Open(filepath.Join(d.root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	defer leaked.Close()
	cmd.ExtraFiles = append(s.Files(), leaked, leaked)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := s.Wait(30 * time.Second)
	cmd.Wait()
	return waited, stderr.String(), out
}

// With no sandbox around it, the real `enter` fails its canary row: the core refuses the start, the owner's sentence
// is said, and the desk's program never runs.
func TestEnterWithNoSandboxIsRefusedAndRunsNothing(t *testing.T) {
	d := newStandInDesk(t)
	s := mustStart(t, d.checks)
	waited, stderr, out := runEnter(t, s, d)
	var refused *Refused
	why := map[string]string{"darwin": "read: opened, not EPERM", "linux": "the support folder seen is the core's"}[runtime.GOOS]
	if !errors.As(waited, &refused) || !strings.Contains(refused.Why, "canary") || !strings.Contains(refused.Why, why) {
		t.Errorf("the core's answer: %v", waited)
	}
	if !strings.Contains(stderr, Refusal(runtime.GOOS)) {
		t.Errorf("enter said %q", stderr)
	}
	if _, err := os.Lstat(out); err == nil {
		t.Error("the desk's program ran")
	}
	if data, _ := os.ReadFile(d.checks.NoWrite[0]); string(data) != `{"fence":1}` {
		t.Errorf("the guard file is %q", data)
	}
	if _, err := os.Lstat(d.checks.Write[0]); err == nil {
		t.Error("a status file made by the check was left behind")
	}
	if Run([]string{"connect"}, &bytes.Buffer{}) != 2 || Run(nil, &bytes.Buffer{}) != 2 {
		t.Error("smolcorp sandbox took a word it doesn't have")
	}
}
