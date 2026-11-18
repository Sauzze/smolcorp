package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

// FXCHRONYSOCK (CI run 44079051196, linuxcaps on ubuntu-22.04, 24.04 and 24.04-arm): chrony's socket listens in
// /run/chrony, _chrony's folder, 0750, which the owner's account can't look into. The moved-socket check called it a
// socket a desk could have moved, and no sandboxed desk could start on the computer.

// The decision, on stand-in stat results: a folder only a system account can change is a system account's (uid ≤ 999),
// not the owner's, and writable by no group nor other. Never one the owner's uid can write, nor any non-root folder as
// such (the mutant "trust any folder not root's").
func TestOnlyASystemAccountsFolderNoOneElseWritesCountsAsHeld(t *testing.T) {
	const owner = 1000
	dir := func(uid int64, perm fs.FileMode) viewStat {
		return viewStat{mode: fs.ModeDir | perm, uid: uid, gid: uid}
	}
	for _, c := range []struct {
		name  string
		st    viewStat
		owner int64
		want  bool
	}{
		{"chrony's /run/chrony: _chrony's, 0750", dir(112, 0o750), owner, true},
		{"a system account's, 0755", dir(105, 0o755), owner, true},
		{"root's, 0755", dir(0, 0o755), owner, true},
		{"a system account's, 0700", dir(999, 0o700), owner, true},
		{"the owner's, 0700", dir(owner, 0o700), owner, false},
		{"the owner's, 0750", dir(owner, 0o750), owner, false},
		{"the owner's, though the owner is a system account (uid 500)", dir(500, 0o755), 500, false},
		{"a system account's, writable by others", dir(112, 0o777), owner, false},
		{"a system account's, writable by its group (or an ACL's mask)", dir(112, 0o770), owner, false},
		{"a system account's, sticky and writable by all (/tmp's shape)", dir(112, 0o777|fs.ModeSticky), owner, false},
		{"another person's account (uid 1001), 0755", dir(1001, 0o755), owner, false},
		{"nobody's (uid 65534), 0755", dir(65534, 0o755), owner, false},
		{"the owner unknown", dir(112, 0o750), -1, false},
		{"its own uid unknown", dir(-1, 0o750), owner, false},
		{"not a folder", viewStat{mode: 0o640, uid: 112}, owner, false},
		{"a link", viewStat{mode: fs.ModeSymlink | 0o777, uid: 112}, owner, false},
	} {
		if got := systemAccountFolder(c.st, c.owner); got != c.want {
			t.Errorf("%s (%v, uid %d, owner %d): held %v, want %v", c.name, c.st.mode, c.st.uid, c.owner, got, c.want)
		}
	}
}

// On the made-up computer, as CI's runners have it: chrony's socket listens in /run/chrony (_chrony's, uid 112, 0750,
// in root's /run), which the owner's account can't look into. The start goes on, nothing is masked there, and the log
// names the folder and its owner; and so it does when the socket isn't at its name any more there (0755), or is
// another. The same socket where a desk could have moved it still fails the start (ErrSocketMoved), and the error says
// which check failed and whose the folder is: a folder of the owner's, one writable by others or by its group, another
// person's account's, a system account's in a folder the desk writes (it could be renamed into place), and the way
// through a link the desk planted at another folder, to /run/chrony.
func TestASocketInAFolderASystemAccountHasStartsTheDesk(t *testing.T) {
	const sock = "/run/chrony/chronyd.sock"
	old := lHome + "/notes/chrony/chronyd.sock"
	list := func(at string) string {
		return netUnixHeader + "\n0000000000000000: 00000002 00000000 00000000 0002 01     1 " + at + "\n" // a bound datagram one, as chronyd's
	}
	boundTo := func(at, file string) []byte {
		return diagDump([]diagEntry{{ino: 1, name: at + "\x00", vfs: uint32(fakeFS{}.fakeIno(file)), file: true}})
	}
	for _, c := range []struct {
		name   string
		at     string
		dump   []byte
		folder fakeEntry // /run/chrony
		change func(f fakeFS)
		want   error
		why    string // in the error or the log
	}{
		{"CI's chrony: _chrony's 0750, the account can't look in", sock, boundTo(sock, sock), fakeEntry{kind: 'd', uid: 112, perm: 0o750}, nil, nil,
			"permission denied"},
		{"_chrony's 0755, the socket gone from its name", sock, boundTo(sock, "/run/chrony/elsewhere.sock"), fakeEntry{kind: 'd', uid: 112, perm: 0o755}, func(f fakeFS) {
			delete(f, sock)
		}, nil, "can't be looked at"},
		{"_chrony's 0755, another socket at its name", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 112, perm: 0o755}, nil, nil,
			"the kernel's dump binds it to"},
		{"_chrony's 0755, a file at its name", sock, boundTo(sock, sock), fakeEntry{kind: 'd', uid: 112, perm: 0o755}, func(f fakeFS) {
			f[sock] = fakeEntry{kind: 'f', uid: 112}
		}, nil, "is no socket"},
		{"the owner's folder in /run, the socket gone", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 1000, perm: 0o755}, nil, ErrSocketMoved,
			"is uid 1000's"},
		{"the owner's 0700 folder in /run, the socket gone", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 1000, perm: 0o700}, nil, ErrSocketMoved,
			"is uid 1000's"},
		{"_chrony's, writable by others", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 112, perm: 0o777}, nil, ErrSocketMoved,
			"is uid 112's"},
		{"_chrony's, writable by its group", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 112, perm: 0o770}, nil, ErrSocketMoved,
			"is uid 112's"},
		{"another person's (uid 1001), 0755", sock, boundTo(sock, "/x"), fakeEntry{kind: 'd', uid: 1001, perm: 0o755}, nil, ErrSocketMoved,
			"is uid 1001's"},
		{"_chrony's 0750, in a folder the desk writes", old, boundTo(old, old), fakeEntry{kind: 'd', uid: 112, perm: 0o750}, func(f fakeFS) {
			f.add(old + "@").owners()
			f[lHome+"/notes/chrony"] = fakeEntry{kind: 'd', uid: 112, perm: 0o750}
		}, ErrSocketMoved, "is uid 112's"},
		{"a link the desk planted at its old folder, to /run/chrony", old, boundTo(old, "/x"), fakeEntry{kind: 'd', uid: 112, perm: 0o750}, func(f fakeFS) {
			f.add(lHome + "/notes/").owners()
			f[lHome+"/notes/chrony"] = fakeEntry{kind: 'l', to: "/run/chrony", uid: 1000}
		}, ErrSocketMoved, "is uid 112's"},
	} {
		f := linuxComputer().add(sock + "@")
		f.owners()
		f["/run/chrony"] = c.folder
		f[sock] = fakeEntry{kind: 's', uid: 112}
		if c.change != nil {
			c.change(f)
		}
		var logged []string
		v := linuxView
		v.Log = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
		args, err := startOn(t, f, v, netFS{fakeFS: f, text: list(c.at), dump: c.dump})
		if !errors.Is(err, c.want) || c.want == nil && err != nil {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
			continue
		}
		if c.want != nil {
			if !strings.Contains(err.Error(), c.why) || !strings.Contains(err.Error(), "the owner is uid 1000") {
				t.Errorf("%s: %q doesn't say %q, nor whose the folder is", c.name, err, c.why)
			}
			continue
		}
		if !slices.ContainsFunc(logged, func(l string) bool {
			return strings.Contains(l, sock) && strings.Contains(l, c.why) && strings.Contains(l, "/run/chrony is uid 112's")
		}) {
			t.Errorf("%s: logged %q, want %s, %q and its folder's owner named", c.name, logged, sock, c.why)
		}
		if words := argvText(args); strings.Contains(words, "/run/chrony/") && c.folder.perm == 0o750 {
			t.Errorf("%s: something is masked or brought back in a folder the account can't open:\n%s", c.name, words)
		}
	}
}

// RT126CHRONYSOCK's F1 (its probe R1): a system account's folder counts as held only when no folder above it, up to /,
// is one a desk can change a name in, not just the one right above. Here _chrony's 0755 folder is in root's 0755
// folder, which no desk can change a name in itself, but that one is in ~/notes, which a desk writes: the desk can
// rename root's folder away and put another in its place. The socket isn't at its name any more there, so the start
// fails (ErrSocketMoved) and says whose the folder is. Root's folder is set after prepare: prepare's owners() makes
// every uid-0 entry in the home folder the owner's, which is why startOn's table above can't hold this case.
func TestASystemAccountsFolderUnderRootsFolderADeskCanRenameFailsTheStart(t *testing.T) {
	old := lHome + "/notes/rootbox/chrony/chronyd.sock"
	list := netUnixHeader + "\n0000000000000000: 00000002 00000000 00000000 0002 01     1 " + old + "\n"
	dump := diagDump([]diagEntry{{ino: 1, name: old + "\x00", vfs: uint32(fakeFS{}.fakeIno("/x")), file: true}})
	f := linuxComputer().add(old + "@")
	f.owners()
	f[lHome+"/notes/rootbox/chrony"] = fakeEntry{kind: 'd', uid: 112, perm: 0o755}
	v := linuxView
	v.Log = t.Logf
	s := linuxSetups()[0]
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, v, f)
	f[lHome+"/notes/rootbox"] = fakeEntry{kind: 'd', uid: 0, perm: 0o755}
	_, err := bwrapArgs(p, s.pl, v, netFS{fakeFS: f, text: list, dump: dump}, "/tmp")
	if !errors.Is(err, ErrSocketMoved) {
		t.Fatalf("_chrony's 0755 folder in root's 0755 folder in ~/notes, the socket gone: %v, want %v", err, ErrSocketMoved)
	}
	for _, want := range []string{"the kernel's dump binds it to", lHome + "/notes/rootbox/chrony is uid 112's", "the owner is uid 1000"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q doesn't say %q", err, want)
		}
	}
}
