//go:build unix

package sandbox

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

// SA-L2: what Linux's runner decides without the kernel, on every system. The kernel's side is run_linux_test.go's.

// ErrKeptChanged reaches the owner in plain words (the release gate's item 4b): a real start that put back a place the
// desk's guard stopped it for says so, and what, in the card's words; not the error's own. Each other step's error
// gets its row of §A5.2's table.
func TestALinuxStartsRefusalIsThePlainWordsOfItsStep(t *testing.T) {
	pl := keptDesk(t)
	at := pl.GitDir + "/worktrees/h1/gitdir"
	replaceByRename(t, at, "/elsewhere/.git\n")
	if changes := guardStops(t, pl); len(changes) != 1 {
		t.Fatalf("the guard: %+v", changes)
	}
	kept := SaveKept(pl)
	if !errors.Is(kept, ErrKeptChanged) {
		t.Fatalf("the next start: %v", kept)
	}
	words := LinuxStartRefusal(kept)
	if !strings.HasPrefix(words, KeptChangedRefusal+"\n") || !strings.Contains(words, at) ||
		!strings.Contains(words, "set aside whole in") || strings.Contains(words, "sandbox: ") || strings.Contains(words, ErrKeptChanged.Error()) {
		t.Errorf("the owner reads %q", words)
	}
	wrapped := fmt.Errorf("desk h1's start: %w", kept)
	if got := LinuxStartRefusal(wrapped); !strings.HasPrefix(got, KeptChangedRefusal) {
		t.Errorf("wrapped, the owner reads %q", got)
	}
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"no bubblewrap", fmt.Errorf("%w: /usr/bin/bwrap: missing", ErrNoBwrap), BwrapRefusal},
		{"Smolcorp's copy stale", errStaleBwrap, RefusalFor(errStaleBwrap)},
		{"no profile", ErrNoProfile, RefusalFor(ErrNoProfile)},
		{"a row failed", &Refused{Why: `seccomp "" failed`}, Refusal("linux")},
		{"not our server", fmt.Errorf("%w: pid 7 isn't in the view's pid namespace", ErrNotOurServer), NotOurServerRefusal},
		{"the view not made", fmt.Errorf("%w: /x", ErrHeldLink), LinuxSandboxRefusal},
		{"no filter here", errors.New("sandbox: no seccomp filter for riscv64"), LinuxSandboxRefusal},
		{"nothing", nil, ""},
	} {
		if got := LinuxStartRefusal(c.err); got != c.want {
			t.Errorf("%s: the owner reads %q, want %q", c.name, got, c.want)
		}
	}
	if strings.Contains(LinuxSandboxRefusal+NotOurServerRefusal+KeptChangedRefusal, "Mac") {
		t.Error("Linux's words say Mac")
	}
}

// bwrap's info: its child's pid, read from one JSON object without waiting for the end (bwrap's first process may
// hold the descriptor while the view lasts). Anything else names no child.
func TestBwrapsInfoNamesItsChild(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close() // kept open: the object is read whole before any end
	fmt.Fprint(w, "{\n    \"child-pid\": 4242,\n    \"mnt-namespace\": 4026532301,\n    \"pid-namespace\": 4026532303\n}\n")
	if pid, err := readInfo(r); err != nil || pid != 4242 {
		t.Errorf("read %d, %v", pid, err)
	}
	for _, info := range []string{"", "{}", `{"child-pid": 1}`, `{"child-pid": 0}`, `{"child-pid": "12"}`, `["child-pid", 12]`, "bwrap: setting up uid map"} {
		if pid, err := readInfo(strings.NewReader(info)); err == nil {
			t.Errorf("%q named child %d", info, pid)
		}
	}
}

// The server check passes only a server of this user's, with a pid, in the view's pid and mount namespaces both.
func TestTheServerMustSitInTheRecordedNamespaces(t *testing.T) {
	view := Namespaces{Pid: NS{Dev: 4, Ino: 4026532303}, Mnt: NS{Dev: 4, Ino: 4026532301}}
	me := uint32(os.Getuid())
	if err := peerInView(me, 4242, view, view); err != nil {
		t.Fatalf("the desk's own server: %v", err)
	}
	other := func(change func(*Namespaces)) Namespaces {
		n := view
		change(&n)
		return n
	}
	for _, c := range []struct {
		name     string
		uid      uint32
		pid      int
		got, rec Namespaces
	}{
		{"in no view (the computer's namespaces)", me, 4242, Namespaces{Pid: NS{4, 4026531836}, Mnt: NS{4, 4026531841}}, view},
		{"in another view's pid namespace", me, 4242, other(func(n *Namespaces) { n.Pid.Ino++ }), view},
		{"in the pid namespace, another mount namespace", me, 4242, other(func(n *Namespaces) { n.Mnt.Ino++ }), view},
		{"another device's inode", me, 4242, other(func(n *Namespaces) { n.Mnt.Dev++ }), view},
		{"another user's", me + 1, 4242, view, view},
		{"no pid", me, 0, view, view},
		{"no view recorded", me, 4242, Namespaces{}, Namespaces{}},
	} {
		if err := peerInView(c.uid, c.pid, c.got, c.rec); err == nil {
			t.Errorf("%s: passed", c.name)
		}
	}
}

// The Landlock row is there where the kernel has the abstract-socket scope (ABI 6), and skipped with why where it
// hasn't.
func TestTheLandlockRowIsThereOnlyWhereTheKernelHasTheScope(t *testing.T) {
	for _, c := range []struct {
		abi  int
		err  error
		want bool
		why  string
	}{
		{6, nil, true, ""}, {7, nil, true, ""},
		{5, nil, false, "ABI 5"}, {1, nil, false, "ABI 1"},
		{0, syscall.EOPNOTSUPP, false, "isn't on"}, {0, syscall.ENOSYS, false, "isn't on"},
	} {
		got, why := landlockFrom(c.abi, c.err)
		if got != c.want || !strings.Contains(why, c.why) || got != (why == "") {
			t.Errorf("ABI %d (%v): %v, %q", c.abi, c.err, got, why)
		}
	}
}
