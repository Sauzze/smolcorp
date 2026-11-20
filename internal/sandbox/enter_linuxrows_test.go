//go:build unix

package sandbox

import (
	"errors"
	"io"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SA-L2's rows in `enter` (§A5.2 step 6): on Linux the filter refuses TIOCSTI and TIOCLINUX, and Landlock's
// abstract-socket scope holds. Faked here, each way a view without them would answer; the kernel's own answers are
// seccomp_linux_test.go's and run_linux_test.go's.

// A Linux start with both rows: the core's words carry them as flags, `enter` reads them back in the core's order,
// a holding view passes them and the core takes its report. Each faked the way a view without it would answer fails
// its own row, and the core refuses the start.
func TestTheLinuxRowsAreCheckedAndEachFakedIsRefused(t *testing.T) {
	d := newStandInDesk(t)
	c := d.checks
	c.Seccomp, c.Landlock = true, true
	s := mustStart(t, c)
	words := s.Words("/bin/echo")
	if !slices.Contains(words, "--seccomp") || !slices.Contains(words, "--landlock") ||
		slices.Index(words, "--seccomp") > slices.Index(words, "--landlock") || slices.Index(words, "--landlock") > slices.Index(words, "--") {
		t.Fatalf("the core's words: %q", words)
	}
	e, err := parseEnter(words[2:])
	if err != nil {
		t.Fatal(err)
	}
	kinds := func(rows []checkRow) []string {
		var out []string
		for _, r := range rows {
			out = append(out, r.kind)
		}
		return out
	}
	if !slices.Equal(kinds(e.rows), kinds(s.rows)) || !slices.Equal(kinds(e.rows)[len(e.rows)-3:], []string{rowSeccomp, rowLandlock, rowFDs}) {
		t.Fatalf("enter's rows %q, the core's %q", kinds(e.rows), kinds(s.rows))
	}
	results, held := runRows("linux", holding("linux", s, d), e)
	if !held {
		t.Fatalf("a holding view failed: %q", results)
	}
	give(t, s, report(s.nonce, e.rows, results))
	if err := s.Wait(5 * time.Second); err != nil {
		t.Fatalf("a holding view's report: %v", err)
	}

	var landlocked, listened, closed bool
	for _, f := range []struct {
		name, row string
		change    func(f *fakeSystem)
	}{
		{"no filter (EBADF for every request)", rowSeccomp, func(f *fakeSystem) {
			f.ioctlFn = func(int, uint64) error { return syscall.EBADF }
		}},
		{"a filter that reads the request's high bits", rowSeccomp, func(f *fakeSystem) {
			f.ioctlFn = func(_ int, request uint64) error {
				if request == requestTIOCSTI || request == requestTIOCLINUX {
					return syscall.EPERM
				}
				return syscall.EBADF
			}
		}},
		{"TIOCLINUX let through", rowSeccomp, func(f *fakeSystem) {
			f.ioctlFn = func(_ int, request uint64) error {
				if uint32(request) == requestTIOCSTI {
					return syscall.EPERM
				}
				return syscall.EBADF
			}
		}},
		{"a filter that refuses every ioctl", rowSeccomp, func(f *fakeSystem) {
			f.ioctlFn = func(int, uint64) error { return syscall.EPERM }
		}},
		{"a kernel with no seccomp row's call (ENOSYS)", rowSeccomp, func(f *fakeSystem) {
			f.ioctlFn = func(int, uint64) error { return syscall.ENOSYS }
		}},
		{"the scope refused by the kernel", rowLandlock, func(f *fakeSystem) {
			f.landlockFn = func() error { return syscall.EOPNOTSUPP }
		}},
		{"the scope said applied, and the socket outside it reached", rowLandlock, func(f *fakeSystem) {
			f.connectFn = func(path string) error {
				if strings.HasPrefix(path, "@") {
					return nil
				}
				return syscall.ENOENT
			}
		}},
		{"the socket outside it refused another way", rowLandlock, func(f *fakeSystem) {
			f.connectFn = func(path string) error {
				if strings.HasPrefix(path, "@") {
					return syscall.ECONNREFUSED
				}
				return syscall.ENOENT
			}
		}},
		{"no socket to try it on", rowLandlock, func(f *fakeSystem) {
			f.listenFn = func(string) (io.Closer, error) { return nil, syscall.EADDRINUSE }
		}},
	} {
		s := mustStart(t, c)
		e, _ := parseEnter(s.Words("/bin/echo")[2:])
		sys := holding("linux", s, d)
		f.change(&sys)
		results, held := runRows("linux", sys, e)
		failedRow := ""
		for i, result := range results {
			if result != passed && failedRow == "" {
				failedRow = e.rows[i].kind
			}
		}
		if held || failedRow != f.row {
			t.Errorf("%s: held %v, first failed row %q, want %q (%q)", f.name, held, failedRow, f.row, results)
			continue
		}
		give(t, s, report(s.nonce, e.rows, results))
		var refused *Refused
		if err := s.Wait(5 * time.Second); !errors.As(err, &refused) {
			t.Errorf("%s: the core took it: %v", f.name, err)
		}
	}

	// The Landlock row tries an abstract name of its own, made before the scope and connected to after it, then closed.
	s = mustStart(t, c)
	e, _ = parseEnter(s.Words("/bin/echo")[2:])
	sys := holding("linux", s, d)
	var name string
	sys.listenFn = func(at string) (io.Closer, error) {
		listened, name = !landlocked, at
		return closer(func() { closed = true }), nil
	}
	sys.landlockFn = func() error { landlocked = true; return nil }
	sys.connectFn = func(at string) error {
		switch {
		case !strings.HasPrefix(at, "@"):
			return syscall.ENOENT // the other server, out of the view
		case at != name || !landlocked:
			return nil // reached: fails the row
		}
		return syscall.EPERM
	}
	if results, held := runRows("linux", sys, e); !held || !listened || !closed || !strings.HasPrefix(name, "@smolcorp-enter-") {
		t.Errorf("the Landlock row: held %v, listened before the scope %v, closed %v, at %q (%q)", held, listened, closed, name, results)
	}
	// Off Linux the real system fails both rows: no start there has them.
	if runtime.GOOS != "linux" {
		if check(runtime.GOOS, realSystem{}, e, checkRow{kind: rowSeccomp}) == passed || (realSystem{}).landlock() == nil {
			t.Errorf("%s passed a Linux row", runtime.GOOS)
		}
	}
}

type closer func()

func (c closer) Close() error { c(); return nil }

// `enter`'s words for the rows: a flag alone, after the canary, once each; a start may have neither.
func TestTheLinuxRowsAreFlagsInPlaceAndOnce(t *testing.T) {
	d := newStandInDesk(t)
	c := d.checks
	c.Seccomp = true
	good := mustStart(t, c).Words("/bin/echo")[2:]
	at := slices.Index(good, "--seccomp")
	for name, args := range map[string][]string{
		"before the canary": append([]string{"--seccomp"}, slices.Delete(slices.Clone(good), at, at+1)...),
		"twice":             slices.Insert(slices.Clone(good), at, "--seccomp"),
	} {
		if _, err := parseEnter(args); err == nil {
			t.Errorf("%s: enter took it", name)
		}
	}
	plain := mustStart(t, d.checks)
	if e, err := parseEnter(plain.Words("/bin/echo")[2:]); err != nil || slices.ContainsFunc(e.rows, func(r checkRow) bool { return bareRow(r.kind) }) {
		t.Errorf("a start with neither row: %v", err)
	}
	if line := (checkRow{kind: rowSeccomp}).line(passed); line != `seccomp "" ok` {
		t.Errorf("the report's line: %q", line)
	}
}
