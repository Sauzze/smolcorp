package sandbox

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// SA-L3: the socket mask (mask.go), on the made-up computer of bwrap_test.go; the real run is mask_linux_test.go's.

// A /proc/net/unix as a kernel writes it: the header, then a line per socket, the Inode padded on the left, the Path
// as bound (an abstract name's NULs as @).
const netUnixText = `Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000002 00000000 00010000 0001 01 23456 /run/stand-in/listens.sock
0000000000000000: 00000003 00000000 00000000 0001 03 23460 /run/stand-in/listens.sock
0000000000000000: 00000002 00000000 00000000 0001 01 23461 /run/stand-in/bound-not-listening.sock
0000000000000000: 00000002 00000000 00000000 0002 01 23462 /run/stand-in/datagram.sock
0000000000000000: 00000002 00000000 00010000 0005 01 23463 /run/stand-in/seqpacket.sock
0000000000000000: 00000002 00000000 00010000 0001 01   999 /home/ada/a folder/with spaces.sock
0000000000000000: 00000002 00000000 00010000 0001 01 23464 @/tmp/.X11-unix/X0
0000000000000000: 00000002 00000000 00000000 0002 01 23465 @stand-in@abstract
0000000000000000: 00000002 00000000 00010000 0001 01 23466 relative.sock
0000000000000000: 00000002 00000000 00010000 0001 01 23467 ./work/dev.sock
0000000000000000: 00000003 00000000 00000000 0001 03 23468
0000000000000000: 00000002 00000000 00010000 0001 01 23469 /run/stand-in/listens.sock
00000000: 00000002 00000000 00010000 0001 01 1234567 /run/stand-in/32-bit.sock
`

// What /proc/net/unix lists as reachable by a name: each listening (stream or seqpacket) or bound datagram socket, its
// name as bound, an abstract one's too (RT126SAL3R2 F6: the text can't tell it from a relative @…, the dump can);
// bound-but-not-listening ones left out. RT126SAL3 B1 (P1, P4): text that isn't the
// kernel's fails, a line it doesn't write fails, never a line skipped. B2 (P2): a newline in a name leaves a tail that
// doesn't read as a line.
func TestTheListOfUnixSocketsIsReadAsTheKernelWritesIt(t *testing.T) {
	got, err := parseNetUnix([]byte(netUnixText))
	if err != nil {
		t.Fatal(err)
	}
	want := []netSocket{{23456, "/run/stand-in/listens.sock"}, {23462, "/run/stand-in/datagram.sock"},
		{23463, "/run/stand-in/seqpacket.sock"}, {999, "/home/ada/a folder/with spaces.sock"}, {23464, "@/tmp/.X11-unix/X0"},
		{23465, "@stand-in@abstract"}, {23466, "relative.sock"},
		{23467, "./work/dev.sock"}, {23469, "/run/stand-in/listens.sock"}, {1234567, "/run/stand-in/32-bit.sock"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	line := "0000000000000000: 00000002 00000000 00010000 0001 01 23456 /run/stand-in/x.sock\n"
	for name, text := range map[string]string{
		"P1 empty":                "",
		"P1 header only":          netUnixHeader + "\n",
		"P1 garbage":              "hello world\nnot the kernel's\n",
		"no header":               line,
		"another header":          "Num RefCount Protocol Flags Type St Inode Path\n" + line,
		"no end to the last line": netUnixHeader + "\n" + strings.TrimSuffix(line, "\n"),
		"P4 a flags word not hex": netUnixHeader + "\n" + strings.Replace(line, "00010000", "0001000Z", 1),
		"a missing colon":         netUnixHeader + "\n" + strings.Replace(line, ": ", " ", 1),
		"a Num of 15":             netUnixHeader + "\n" + line[1:],
		"a word too short":        netUnixHeader + "\n" + strings.Replace(line, " 0001 01", " 001 01", 1),
		"an inode padded wrong":   netUnixHeader + "\n" + strings.Replace(line, " 23456 ", "  23456 ", 1),
		"an inode not a number":   netUnixHeader + "\n" + strings.Replace(line, "23456", "2345x", 1),
		"a space and no name":     netUnixHeader + "\n" + strings.Replace(line, "23456 /run/stand-in/x.sock", "23456 ", 1),
		"P2 a newline in a name":  netUnixHeader + "\n" + strings.Replace(line, "x.sock", "x\ny.sock", 1),
	} {
		if got, err := parseNetUnix([]byte(text)); err == nil {
			t.Errorf("%s: read as %v, want an error", name, got)
		}
	}
}

// The kernel's dump (NETLINK_SOCK_DIAG) is read byte for byte: each socket's name as /proc/net/unix writes it (a
// path's last NUL left out, an abstract one's NULs as @), and its file's inode. A dump that doesn't read so, an error
// the kernel gives, or one without its end, fails.
func TestTheKernelsDumpOfUnixSocketsIsReadByteForByte(t *testing.T) {
	good := diagDump([]diagEntry{{ino: 7, name: "/run/a\nb.sock\x00", vfs: 99, file: true}, {ino: 8, name: "\x00stand-in\x00x"},
		{ino: 9, name: "rel.sock\x00", vfs: 5, file: true}})
	got, err := parseUnixDiag(good)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint64]diagSocket{7: {"/run/a\nb.sock", 99, true, false}, 8: {"@stand-in@x", 0, false, true}, 9: {"rel.sock", 5, true, false}}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for ino, w := range want {
		if got[ino] != w {
			t.Errorf("socket %d: %+v, want %+v", ino, got[ino], w)
		}
	}
	if done, err := diagEnded(good); !done || err != nil {
		t.Errorf("the dump's end: %v, %v", done, err)
	}
	refused := make([]byte, 20)
	binary.NativeEndian.PutUint32(refused, 20)
	binary.NativeEndian.PutUint16(refused[4:], nlmsgError)
	binary.NativeEndian.PutUint32(refused[16:], uint32(0xffffffea)) // -EINVAL
	attrTooLong := slices.Clone(good)
	binary.NativeEndian.PutUint16(attrTooLong[32:], 200)
	for name, dump := range map[string][]byte{
		"empty": nil, "no end": good[:len(good)-20], "cut short": good[:len(good)-3], "the kernel's error": refused,
		"an attribute too long": attrTooLong,
	} {
		if got, err := parseUnixDiag(dump); err == nil {
			t.Errorf("%s: read as %v, want an error", name, got)
		}
	}
	if req := unixDiagRequest(); len(req) != 40 || binary.NativeEndian.Uint32(req) != 40 || req[16] != 1 ||
		binary.NativeEndian.Uint32(req[28:]) != 3 {
		t.Errorf("the request: % x", req)
	}
}

// netFS is the made-up computer with its own /proc/net/unix, and the kernel's dump: dump as given, or each line's
// socket bound to the file its name leads to on the computer, when there is one.
type netFS struct {
	fakeFS
	text string
	err  error
	dump []byte
}

func (n netFS) unixSockets() ([]byte, error) { return []byte(n.text), n.err }

func (n netFS) unixDiag() ([]byte, error) {
	if n.dump != nil {
		return n.dump, nil
	}
	var entries []diagEntry
	for _, line := range strings.Split(n.text, "\n") {
		s, _, ok := netUnixLine(line)
		if !ok || s.name == "" {
			continue
		}
		e := diagEntry{ino: uint32(s.ino), name: s.name + "\x00"}
		if strings.HasPrefix(s.name, "@") { // abstract: a relative @… needs a dump of its own
			e.name = strings.ReplaceAll(s.name, "@", "\x00")
		}
		if !path.IsAbs(s.name) {
			entries = append(entries, e)
			continue
		}
		b := &viewBuilder{sys: n.fakeFS}
		if at := path.Join(b.realFolder(path.Dir(s.name)), path.Base(s.name)); n.fakeFS[at].kind == 's' {
			e.vfs, e.file = uint32(n.fakeIno(at)), true
		}
		entries = append(entries, e)
	}
	return diagDump(entries), nil
}

// startOn is the view's words for the first setup on f, with v, reading the sockets from sys (f's own when nil).
func startOn(t *testing.T, f fakeFS, v View, sys viewFS) ([]string, error) {
	t.Helper()
	s := linuxSetups()[0]
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, v, f)
	if sys == nil {
		sys = f
	}
	return bwrapArgs(p, s.pl, v, sys, "/tmp")
}

// Every socket that listens at the start and isn't on the desk's list is masked: not brought back where its folder is
// covered with what the Policy allows (the home folder), /dev/null over it where its folder is brought back writable
// (~/.local/state/app), and, in /run, which holds Docker's socket, not brought back. /run and /run/dbus, root's alone,
// are covered with all else brought back: a socket Docker, containerd or Podman makes there later isn't seen. The
// agents, its own socket folder and its desk-start socket stay reachable.
func TestEveryListeningSocketOffTheListIsMasked(t *testing.T) {
	for _, s := range linuxSetups() {
		f := linuxComputer()
		_, args := viewOf(t, s, f)
		mounts, _ := playView(t, args)
		hidden := map[string]bool{}
		for _, m := range mounts {
			if m.kind == "hide" {
				hidden[m.at] = true
			}
		}
		for _, at := range maskedSockets(s) {
			if says(mounts, at, Connect) != Deny {
				t.Errorf("%s: %s, off the list, can be connected to", s.name, at)
			}
		}
		// How: /dev/null only where its folder is brought back writable; elsewhere it isn't there.
		for at, want := range map[string]bool{lHome + "/.local/state/app/ipc.sock": true, "/run/avahi-daemon/socket": true,
			lHome + "/stand-in.sock": false, "/run/docker.sock": false, "/run/dbus/system_bus_socket": false} {
			if hidden[at] != want {
				t.Errorf("%s: %s: /dev/null over it %v, want %v", s.name, at, hidden[at], want)
			}
		}
		if s.pl.Support != lCopy && !hidden[lCopy+"/desk-start"] {
			t.Errorf("%s: a test copy's desk-start socket, another core's, isn't masked", s.name)
		}
		listed := []string{s.pl.DeskStart, s.pl.OwnSockets + "/s", lRun + "/keyring/ssh", lRun + "/gnupg/S.gpg-agent", "/dev/log",
			lHome + "/.1password/agent.sock"}
		for _, at := range listed {
			if says(mounts, at, Connect) != Allow {
				t.Errorf("%s: %s, on the list, can't be connected to", s.name, at)
			}
		}
		for _, folder := range []string{"/run", "/run/dbus"} {
			if !slices.Contains(mounts, mounted{"cover", folder}) {
				t.Errorf("%s: %s, root's, holding a socket Smolcorp always masks, isn't covered", s.name, folder)
			}
		}
		// Made after the start, in a covered folder: not seen. Brought back: /run's folders, and its files.
		for _, at := range []string{"/run/docker.sock.new", "/run/containerd/containerd.sock", "/run/podman/podman.sock", "/run/dbus/stand-in"} {
			if says(mounts, at, Connect) != Deny {
				t.Errorf("%s: %s, made after the start, can be connected to", s.name, at)
			}
		}
		for _, at := range []string{"/run/systemd/journal/dev-log", "/run/dbus/pid", "/run/user"} {
			if says(mounts, at, Read) != Allow {
				t.Errorf("%s: %s isn't brought back into the covered /run", s.name, at)
			}
		}
	}
}

// A socket is found where its folder's links lead (/var/run is /run), and one on the list is never masked, however
// its folder is spelled. A socket in a folder covered whole needs no word of its own.
func TestTheMaskFollowsTheFoldersLinks(t *testing.T) {
	s := linuxSetups()[0]
	f := linuxComputer()
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, linuxView, f)
	text := "Num       RefCount Protocol Flags    Type St Inode Path\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01     1 /var/run/avahi-daemon/socket\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01     2 /var/run/../run/systemd/journal/dev-log\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01     3 " + lRun + "/bus\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01     4 " + lHome + "/.local/state/app/../app/ipc.sock\n"
	args, err := bwrapArgs(p, s.pl, linuxView, netFS{fakeFS: f, text: text}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	words := argvText(args)
	for _, in := range []string{"--ro-bind /dev/null /run/avahi-daemon/socket\n", "--ro-bind /dev/null " + lHome + "/.local/state/app/ipc.sock\n"} {
		if !strings.Contains(words, in) {
			t.Errorf("no %q in:\n%s", in, words)
		}
	}
	for _, out := range []string{"/var/run", "/dev/null /run/systemd", "/dev/null " + lRun} {
		if strings.Contains(words, out) {
			t.Errorf("%q in:\n%s", out, words)
		}
	}
}

// A socket that listens by a relative name can't be found to be masked: the start logs it, and goes on. A list that
// can't be read fails the start: the view would mask nothing.
func TestARelativeSocketIsLoggedAndAMissingListFailsTheStart(t *testing.T) {
	s := linuxSetups()[0]
	f := linuxComputer()
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, linuxView, f)
	plain, err := bwrapArgs(p, s.pl, linuxView, f, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	full, _ := f.unixSockets()
	var logged []string
	v := linuxView
	v.Log = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	args, err := bwrapArgs(p, s.pl, v, netFS{fakeFS: f, text: string(full) + "0000000000000000: 00000002 00000000 00010000 0001 01     9 dev.sock\n"}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], `"dev.sock"`) {
		t.Errorf("logged %q, want the relative name dev.sock", logged)
	}
	if !slices.Equal(args, plain) {
		t.Errorf("a relative name changed the view")
	}
	// RT126SAL3R2 F6: a relative name starting with @ is logged too; an abstract one isn't (the dump tells them apart).
	logged = nil
	text := string(full) + "0000000000000000: 00000002 00000000 00010000 0001 01     9 @dev.sock\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01    10 @stand-in@abstract\n"
	dump := diagDump(append(f.diagEntries(), diagEntry{ino: 9, name: "@dev.sock\x00", vfs: 5, file: true},
		diagEntry{ino: 10, name: "\x00stand-in\x00abstract"}))
	args, err = bwrapArgs(p, s.pl, v, netFS{fakeFS: f, text: text, dump: dump}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], `"@dev.sock"`) {
		t.Errorf("logged %q, want the relative name @dev.sock alone", logged)
	}
	if !slices.Equal(args, plain) {
		t.Errorf("a relative @ name or an abstract one changed the view")
	}
	for _, bad := range []netFS{{fakeFS: f, err: errors.New("no /proc here")}} {
		if _, err := bwrapArgs(p, s.pl, linuxView, bad, "/tmp"); !errors.Is(err, ErrNoSocketList) {
			t.Errorf("an unreadable list: %v, want ErrNoSocketList", err)
		}
	}
}

// RT126SAL3's P1, P2 and P4 on the start: a list that isn't the kernel's fails the start (ErrNoSocketList), and so
// does a newline in a socket's name, in a folder brought back writable or in the home folder, even with a tail written
// to read as a line (the kernel's dump names the socket whole), and a dump that doesn't read. P3: spaces are fine.
func TestAListOrDumpThatIsntTheKernelsFailsTheStart(t *testing.T) {
	ipc := lHome + "/.local/state/app/ipc.sock"
	full, _ := linuxComputer().unixSockets()
	var ipcLine string
	for _, line := range strings.Split(string(full), "\n") {
		if strings.HasSuffix(line, " "+ipc) {
			ipcLine = line
		}
	}
	for name, text := range map[string]string{"P1 empty": "", "P1 header only": netUnixHeader + "\n",
		"P1 garbage":                     "hello world\nnot the kernel's\n",
		"P4 that socket's flags not hex": strings.Replace(string(full), ipcLine, strings.Replace(ipcLine, "00010000", "0001000Z", 1), 1),
	} {
		f := linuxComputer()
		if _, err := startOn(t, f, linuxView, netFS{fakeFS: f, text: text}); !errors.Is(err, ErrNoSocketList) {
			t.Errorf("%s: %v, want ErrNoSocketList", name, err)
		}
	}
	tail := "0000000000000000: 00000002 00000000 00010000 0001 01 77777 /run/stand-in/decoy.sock"
	for _, at := range []string{lHome + "/.local/state/app/x\ny.sock", lHome + "/x\ny.sock", lHome + "/.local/state/app/x\n" + tail} {
		f := linuxComputer()
		f[at] = fakeEntry{kind: 's', uid: 1000}
		if strings.Contains(at, tail) {
			if text, _ := f.unixSockets(); func() error { _, err := parseNetUnix(text); return err }() != nil {
				t.Fatalf("the tail written as a line doesn't read as one")
			}
		}
		if _, err := startOn(t, f, linuxView, nil); !errors.Is(err, ErrNoSocketList) {
			t.Errorf("P2 %q: %v, want ErrNoSocketList", at, err)
		}
	}
	f := linuxComputer()
	refused := make([]byte, 20)
	binary.NativeEndian.PutUint32(refused, 20)
	binary.NativeEndian.PutUint16(refused[4:], nlmsgError)
	for name, dump := range map[string][]byte{"refused by the kernel": refused, "not a dump": {1, 2, 3}} {
		text, _ := f.unixSockets()
		if _, err := startOn(t, f, linuxView, netFS{fakeFS: f, text: string(text), dump: dump}); !errors.Is(err, ErrNoSocketList) {
			t.Errorf("a dump %s: %v, want ErrNoSocketList", name, err)
		}
	}
	spaced := lHome + "/.local/state/app/a b  c.sock"
	f[spaced] = fakeEntry{kind: 's', uid: 1000}
	args, err := startOn(t, f, linuxView, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mounts, _ := playView(t, args); says(mounts, spaced, Connect) != Deny {
		t.Errorf("P3 %q can be connected to", spaced)
	}
}

// RT126SAL3 B3 (P5): a link a desk plants at an agent's path, in a folder its view brings back writable
// (~/.1password), lists nothing for its next start, whether the link alone is on the agents' list or where it leads
// too: Docker's socket, the system D-Bus and an owner's socket stay masked. A socket Smolcorp always masks is masked
// and its folder covered even when it is on the agents' list itself, and /run/dbus is covered when messagebus has it
// (nit 3). A link where the desk can't change it still works: in the runtime folder, the agent comes back.
func TestALinkTheDeskPlantsListsNothing(t *testing.T) {
	for _, c := range []struct {
		name, to string
		listTo   bool
	}{
		{"P5 the link alone", "/run/docker.sock", false}, {"P5 the link and where it leads", "/run/docker.sock", true},
		{"P5 the system D-Bus", "/run/dbus/system_bus_socket", true}, {"an owner's socket", lHome + "/.local/state/app/ipc.sock", true},
	} {
		f := linuxComputer()
		f[lHome+"/.1password/agent.sock"] = fakeEntry{kind: 'l', to: c.to, uid: 1000}
		v := linuxView
		if c.listTo {
			v.Agents = append(slices.Clone(v.Agents), c.to)
		}
		args, err := startOn(t, f, v, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		mounts, _ := playView(t, args)
		if says(mounts, c.to, Connect) != Deny {
			t.Errorf("%s: %s can be connected to:\n%s", c.name, c.to, argvText(args))
		}
	}
	f := linuxComputer()
	e := f["/run/dbus"]
	e.uid = 103 // messagebus
	f["/run/dbus"] = e
	v := linuxView
	v.Agents = append(slices.Clone(v.Agents), "/run/docker.sock", "/var/run/dbus/system_bus_socket", lRun+"/ssh-link")
	f[lRun+"/ssh-link"] = fakeEntry{kind: 'l', to: lRun + "/keyring/ssh"}
	args, err := startOn(t, f, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	mounts, _ := playView(t, args)
	for _, at := range []string{"/run/docker.sock", "/run/dbus/system_bus_socket"} {
		if says(mounts, at, Connect) != Deny {
			t.Errorf("%s, on the agents' list, can be connected to", at)
		}
	}
	for _, folder := range []string{"/run", "/run/dbus"} {
		if !slices.Contains(mounts, mounted{"cover", folder}) {
			t.Errorf("%s isn't covered", folder)
		}
	}
	if says(mounts, lRun+"/ssh-link", Connect) != Allow {
		t.Errorf("an agent's link in the runtime folder, which no desk changes, isn't brought back:\n%s", argvText(args))
	}
}

// SIERRA's class ruling (FXSAL3R1), and RT126SAL3's P5b: no session bus and no systemd --user from inside the view, by
// any path. The bus and systemd's private socket in the runtime folder, and another user's runtime folder's bus, are
// refused: named as agents themselves, through a link the desk planted (P5b), through a link root has at a later hop,
// and through a folder that is a link. SocketPaths lists none of them either, nor where a link the owner's account can
// change leads. The desk's environment holds no DBUS_*, and the view has a network namespace of its own, where an
// abstract @/tmp/dbus-* is (its Linux row: TestTheMaskStartsOnBwrap).
func TestNoSessionBusNorSystemdUserByAnyPath(t *testing.T) {
	for _, place := range []string{lRun + "/bus", lRun + "/systemd/private", "/run/user/1001/bus"} {
		for _, c := range []struct {
			name  string
			plant func(f fakeFS) []string
		}{
			{"named", func(fakeFS) []string { return []string{place} }},
			{"P5b a link the desk planted, and where it leads", func(f fakeFS) []string {
				f[lHome+"/.1password/agent.sock"] = fakeEntry{kind: 'l', to: place, uid: 1000}
				return []string{lHome + "/.1password/agent.sock", place}
			}},
			{"a link root has, at its second hop", func(f fakeFS) []string {
				f["/etc/stand-in-agent"] = fakeEntry{kind: 'l', to: "/etc/stand-in-hop"}
				f["/etc/stand-in-hop"] = fakeEntry{kind: 'l', to: place}
				return []string{"/etc/stand-in-agent"}
			}},
			{"a folder that is a link", func(f fakeFS) []string {
				f["/etc/stand-in-run"] = fakeEntry{kind: 'l', to: path.Dir(place)}
				return []string{"/etc/stand-in-run/" + path.Base(place)}
			}},
		} {
			f := linuxComputer()
			v := linuxView
			v.Agents = append(slices.Clone(v.Agents), c.plant(f)...)
			args, err := startOn(t, f, v, nil)
			if err != nil {
				t.Fatalf("%s, %s: %v", place, c.name, err)
			}
			mounts, _ := playView(t, args)
			if says(mounts, place, Connect) != Deny {
				t.Errorf("%s, %s: it can be connected to", place, c.name)
			}
			for _, at := range v.Agents[len(linuxView.Agents):] {
				if strings.Contains(argvText(args), " "+at+"\n") {
					t.Errorf("%s, %s: %s is in the view:\n%s", place, c.name, at, argvText(args))
				}
			}
		}
	}
	f := linuxComputer().add("/dev/")
	f[lHome+"/.1password/agent.sock"] = fakeEntry{kind: 'l', to: lRun + "/bus", uid: 1000}
	f[lHome+"/.ssh-agent"] = fakeEntry{kind: 'l', to: lRun + "/keyring/ssh", uid: 1000}
	env := map[string]string{"SSH_AUTH_SOCK": lHome + "/.ssh-agent", "XDG_RUNTIME_DIR": lRun}
	got := socketPaths(Sockets("linux"), lHome, func(name string) string { return env[name] }, true, false, f)
	if want := []string{"/dev/log", "/run/systemd/journal/dev-log", lHome + "/.ssh-agent", lRun + "/gnupg/S.gpg-agent",
		lHome + "/.gnupg/S.gpg-agent"}; !slices.Equal(got, want) {
		t.Errorf("SocketPaths: %q, want %q", got, want)
	}
	for _, at := range []string{lRun + "/bus", "/run/user/1001/bus", lRun + "/systemd/private", lRun + "/systemd/notify",
		"/var/run/user/1000/bus", "/run/user/1000/bus", "/srv/runtime/bus", "/srv/runtime/systemd/private"} {
		if !busPlace(path.Clean(strings.Replace(at, "/var/run", "/run", 1)), []string{"/srv/runtime"}) {
			t.Errorf("%s isn't a bus place", at)
		}
	}
	for _, at := range []string{lRun + "/keyring/ssh", lRun + "/systemd", lRun + "/busy", "/run/user", "/srv/bus"} {
		if busPlace(at, []string{"/srv/runtime"}) {
			t.Errorf("%s is a bus place", at)
		}
	}
	args, err := startOn(t, linuxComputer(), linuxView, nil)
	if err != nil {
		t.Fatal(err)
	}
	// RT126SAL3R2 F4: the accessibility bus's address and systemd's notify socket too.
	want := []string{"--unshare-pid", "--unshare-net", "--unsetenv", "AT_SPI_BUS_ADDRESS", "--unsetenv", "DBUS_FATAL_WARNINGS",
		"--unsetenv", "DBUS_SESSION_BUS_ADDRESS", "--unsetenv", "DBUS_STARTER_ADDRESS", "--unsetenv", "DBUS_STARTER_BUS_TYPE",
		"--unsetenv", "DBUS_SYSTEM_BUS_ADDRESS", "--unsetenv", "NOTIFY_SOCKET"}
	if !slices.Equal(args[:min(len(want), len(args))], want) {
		t.Errorf("the view starts %q, want %q", args[:min(len(want), len(args))], want)
	}
}

// RT126SAL3 B4 (P6): a socket that listens by a name that no longer leads to it (the kernel's dump says which file it
// is bound to), where the desk could have changed the way to it: its folder renamed (P6, and the reviewer's own
// stand-in), renamed with a decoy socket made at the old name, or the socket renamed beside a decoy. The start fails (ErrSocketMoved): it doesn't leave the socket unmasked. One gone from a folder no desk
// changes (root's, the runtime folder covered whole, the private /tmp) is nothing to mask: the start goes on. Its old
// folder made a link to where it went leads the mask there: /dev/null goes over it.
func TestASocketMovedWhereTheDeskWritesFailsTheStart(t *testing.T) {
	old, moved := lHome+"/notes/sub/dev.sock", lHome+"/notes/sub2/dev.sock"
	ipc := lHome + "/.local/state/app/ipc.sock"
	list := func(at string) string {
		return netUnixHeader + "\n0000000000000000: 00000002 00000000 00010000 0001 01     1 " + at + "\n"
	}
	boundTo := func(at, file string) []byte {
		return diagDump([]diagEntry{{ino: 1, name: at + "\x00", vfs: uint32(fakeFS{}.fakeIno(file)), file: true}})
	}
	computer := func(places ...string) fakeFS {
		f := linuxComputer().add(places...)
		f.owners()
		return f
	}
	// RT126SAL3R2 B4-R2 (P8a, P8b, P8c): its folder renamed, then a link at the old name into the desk's own socket
	// folder, a folder holding a listed agent of the same name, or the support folder (a socket named desk-start, which
	// a row of its own allows): the way to its name passes through the desk's link, so the name is on no list.
	oldA, movedA := lHome+"/notes/sub/agent.sock", lHome+"/notes/sub2/agent.sock"
	oldD, movedD := lHome+"/notes/sub/desk-start", lHome+"/notes/sub2/desk-start"
	for _, c := range []struct {
		name string
		f    fakeFS
		at   string
		dump []byte
		want error
		link string // where a link at ~/notes/sub, the desk's, leads
	}{
		{"P8a a link at the old folder into its own socket folder", computer(moved + "@"), old, boundTo(old, moved), ErrSocketMoved, lTmux + "/smolcorp-h1.d"},
		{"P8b a link at the old folder into a listed agent's", computer(movedA + "@"), oldA, boundTo(oldA, movedA), ErrSocketMoved, lHome + "/.1password"},
		{"P8c a link at the old folder into the support folder", computer(movedD + "@"), oldD, boundTo(oldD, movedD), ErrSocketMoved, lSupport},
		{"P6 its folder renamed", computer(moved + "@"), old, boundTo(old, moved), ErrSocketMoved, ""},
		{"P6, the reviewer's stand-in", computer(moved + "@"), old, nil, ErrSocketMoved, ""},
		{"its folder renamed, a decoy made at the old name", computer(moved+"@", old+"@"), old, boundTo(old, moved), ErrSocketMoved, ""},
		{"renamed beside a decoy", computer(ipc + ".moved@"), ipc, boundTo(ipc, ipc+".moved"), ErrSocketMoved, ""},
		{"gone from a folder root has alone", computer(), "/run/stand-in-gone.sock", boundTo("/run/stand-in-gone.sock", "/x"), nil, ""},
		{"gone from the runtime folder", computer(), lRun + "/gone.sock", boundTo(lRun+"/gone.sock", "/x"), nil, ""},
		{"gone from the private /tmp", computer(), "/tmp/gone.sock", boundTo("/tmp/gone.sock", "/x"), nil, ""},
		{"where it was made", computer(), ipc, boundTo(ipc, ipc), nil, ""},
	} {
		if c.link != "" {
			c.f[lHome+"/notes/sub"] = fakeEntry{kind: 'l', to: c.link, uid: 1000}
		}
		_, err := startOn(t, c.f, linuxView, netFS{fakeFS: c.f, text: list(c.at), dump: c.dump})
		if !errors.Is(err, c.want) || c.want == nil && err != nil {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	f := computer(moved + "@")
	f[lHome+"/notes/sub"] = fakeEntry{kind: 'l', to: lHome + "/notes/sub2", uid: 1000}
	args, err := startOn(t, f, linuxView, netFS{fakeFS: f, text: list(old), dump: boundTo(old, moved)})
	if err != nil {
		t.Fatalf("its folder made a link to where it went: %v", err)
	}
	if mounts, _ := playView(t, args); says(mounts, moved, Connect) != Deny {
		t.Errorf("its folder made a link to where it went (P8d): %s, where the link leads, can be connected to", moved)
	}
}

// RT126SAL3R2, the row at every hop: an agent named /var/run/docker.sock, where /run/docker.sock is a link root has to
// a socket elsewhere (as Docker Desktop and rootless setups make it). Followed to its end, the way lands where never
// names nothing; only the check at each hop sees it pass through Docker's socket, which Smolcorp always masks. It is
// refused: not in the view, and where it leads stays masked. Named itself, where it leads is an agent like any other.
func TestAnAgentThroughDockersSocketLinkIsRefusedAtEveryHop(t *testing.T) {
	const elsewhere = "/opt/stand-in-docker/docker.sock"
	for _, c := range []struct {
		agent string
		want  Verdict
	}{{"/var/run/docker.sock", Deny}, {"/run/docker.sock", Deny}, {elsewhere, Allow}} {
		f := linuxComputer().add(elsewhere+"@", "/var/") // /var, so the way walks through its link /var/run
		f["/run/docker.sock"] = fakeEntry{kind: 'l', to: elsewhere}
		v := linuxView
		v.Agents = append(slices.Clone(v.Agents), c.agent)
		args, err := startOn(t, f, v, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.agent, err)
		}
		words := argvText(args)
		mounts, _ := playView(t, args)
		if got := says(mounts, elsewhere, Connect); got != c.want {
			t.Errorf("agent %s: %s, where it leads: %v, want %v:\n%s", c.agent, elsewhere, got, c.want, words)
		}
		// Refused: not bound, from where it leads or by its name, and /dev/null over where it leads (root's link itself
		// comes back into the covered /run, as /run's other links do).
		if c.want == Deny && (strings.Contains(words, "--ro-bind "+elsewhere+" ") || strings.Contains(words, "--ro-bind "+c.agent+" ") ||
			!strings.Contains(words, "--ro-bind /dev/null "+elsewhere+"\n")) {
			t.Errorf("agent %s is in the view, or where it leads isn't masked:\n%s", c.agent, words)
		}
	}
}

// busFailures are the only ways each of busCommands (mask_linux_test.go) may fail inside the view (RT126SAL3R2, VM run 3
// point 1): the stand-in's line, its exit status, then all it wrote, in the C locale. Any other failure (a command
// missing, a connection refused, a permission denied) fails the row: it wouldn't show that there is no bus.
var busFailures = map[string]*regexp.Regexp{
	// systemd ≥ 254, then ≤ 253 (VM run 3 interim 2: Ubuntu 24.04's systemd 255 printed the second as well).
	"run systemd-run --user true": regexp.MustCompile(`^exit status 1: Failed to connect to (user scope bus via local transport|bus): No such file or directory$`),
	"run busctl --user list":      regexp.MustCompile(`^exit status 1: Failed to connect to (user scope bus via local transport|bus): No such file or directory$`),
	// GLib's autolaunch: DISPLAY unset; DISPLAY set, without dbus-x11, as GLib 2.80 says it (Ubuntu 24.04, VM run 3
	// interim 2) and as older GLibs do; DISPLAY set, with dbus-x11 (it can't reach X).
	"run gdbus call --session": regexp.MustCompile(`^exit status 1: Error connecting: (Cannot autolaunch D-Bus without X11 \$DISPLAY|` +
		`Failed to execute child process “dbus-launch” \(No such file or directory\)|` +
		`Error spawning command line “dbus-launch --autolaunch=[0-9a-f]+ --binary-syntax --close-stderr”: ` +
		`(Failed to execute child process “dbus-launch” \(No such file or directory\)|Child process exited with code 1))$`),
}

// Each wording busFailures accepts, as a systemd or a GLib writes it with no bus, is accepted whole, and nothing near
// it: another error, another exit status, a missing command, a line before or after (VICTOR's ruling on VM run 3
// interim 2's gdbus line).
func TestTheBusRowAcceptsOnlyTheNoBusWordings(t *testing.T) {
	const launch = "Error spawning command line “dbus-launch --autolaunch=5f3c9a0e1b2d4c6f8a7e9d0c1b2a3f4e --binary-syntax --close-stderr”: "
	for _, c := range []struct {
		command, got string
		want         bool
	}{
		{"run systemd-run --user true", "exit status 1: Failed to connect to user scope bus via local transport: No such file or directory", true},
		{"run systemd-run --user true", "exit status 1: Failed to connect to bus: No such file or directory", true},
		{"run busctl --user list", "exit status 1: Failed to connect to user scope bus via local transport: No such file or directory", true},
		{"run busctl --user list", "exit status 1: Failed to connect to bus: No such file or directory", true},
		{"run gdbus call --session", "exit status 1: Error connecting: Cannot autolaunch D-Bus without X11 $DISPLAY", true},
		{"run gdbus call --session", "exit status 1: Error connecting: Failed to execute child process “dbus-launch” (No such file or directory)", true},
		{"run gdbus call --session", "exit status 1: Error connecting: " + launch + "Failed to execute child process “dbus-launch” (No such file or directory)", true},
		{"run gdbus call --session", "exit status 1: Error connecting: " + launch + "Child process exited with code 1", true},
		// Near misses: each must still fail the row.
		{"run gdbus call --session", "exit status 1: Error connecting: Failed to execute child process “dbus-launch” (Permission denied)", false},
		{"run gdbus call --session", "exit status 1: Error connecting: Failed to execute child process “dbus-launch” (No such file or directory)\nmore", false},
		{"run gdbus call --session", "exit status 1: Error connecting: Child process exited with code 1", false},
		{"run gdbus call --session", "exit status 2: Error connecting: Failed to execute child process “dbus-launch” (No such file or directory)", false},
		{"run gdbus call --session", "exit status 1: Error connecting: Could not connect: Connection refused", false},
		{"run gdbus call --session", "exit status 127: bwrap: execvp gdbus: No such file or directory", false},
		{"run systemd-run --user true", "exit status 1: Failed to connect to bus: Connection refused", false},
		{"run systemd-run --user true", "exit status 1: Failed to connect to bus: Permission denied", false},
		{"run busctl --user list", "exit status 1: Failed to connect to bus: No such file or directory.", false},
		{"run busctl --user list", "exit status 127: bwrap: execvp busctl: No such file or directory", false},
	} {
		if got := busFailures[c.command].MatchString(c.got); got != c.want {
			t.Errorf("%s, %q: accepted %v, want %v", c.command, c.got, got, c.want)
		}
	}
}

// VM run 3 B1 (SIERRA's ruling): a desk runs as the owner's account, with nothing more. A socket behind a folder that
// account can't open, root's alone, where no desk can change a name in it or above it, is nothing a desk reaches, nor
// can have moved: Docker's /var/run/docker/libnetwork/<id>.sock (its /run/docker root's, 0700), by root's link /var/run
// or by /run, and containerd's in its 0700 folder. The start goes on, nothing is masked there, and the log names the
// folder. Both sides in one family: the same refusal from a root 0700 folder a desk could rename into place (in the
// owner's ~/notes), through a link the desk planted at the moved socket's old folder (P8a's shape, into /run/docker),
// or from a folder of the owner's it can't open (chmod 600) still fails the start, as B4 and P8a do.
func TestASocketTheOwnersAccountCantReachDecidesNothing(t *testing.T) {
	const docker, containerd = "/run/docker/libnetwork/37a131b6e41b.sock", "/run/containerd/containerd.sock"
	old, moved := lHome+"/notes/sub/dev.sock", lHome+"/notes/sub2/dev.sock"
	list := func(at string) string {
		return netUnixHeader + "\n0000000000000000: 00000002 00000000 00010000 0001 01     1 " + at + "\n"
	}
	boundTo := func(at, file string) []byte {
		return diagDump([]diagEntry{{ino: 1, name: at + "\x00", vfs: uint32(fakeFS{}.fakeIno(file)), file: true}})
	}
	for _, c := range []struct {
		name, at string
		dump     []byte
		want     error
		closed   string         // the folder the log names, when the start goes on
		change   func(f fakeFS) // what a desk, or the owner, did at ~/notes/sub
	}{
		{"B1 Docker's libnetwork socket, by root's link /var/run", "/var" + docker, boundTo("/var"+docker, docker), nil, "/run/docker", nil},
		{"B1 Docker's libnetwork socket, by /run", docker, boundTo(docker, docker), nil, "/run/docker", nil},
		{"B2 containerd's socket, in its folder root has alone", containerd, boundTo(containerd, containerd), nil, "/run/containerd", nil},
		{"a root 0700 folder a desk could rename into place, in the owner's ~/notes", old, boundTo(old, moved), ErrSocketMoved, "", func(f fakeFS) {
			f[lHome+"/notes/sub"] = fakeEntry{kind: 'd', perm: 0o700}
		}},
		{"P8a's shape: a link the desk planted at the old folder, to root's closed /run/docker", old, boundTo(old, moved), ErrSocketMoved, "", func(f fakeFS) {
			f[lHome+"/notes/sub"] = fakeEntry{kind: 'l', to: "/run/docker", uid: 1000}
		}},
		{"a folder of the owner's it can't open (chmod 600)", old, boundTo(old, old), ErrSocketMoved, "", func(f fakeFS) {
			f.add(old + "@").owners()
			f[lHome+"/notes/sub"] = fakeEntry{kind: 'd', uid: 1000, perm: 0o600}
		}},
		{"B4 its folder renamed", old, boundTo(old, moved), ErrSocketMoved, "", nil},
		{"P8a a link at the old folder into its own socket folder", old, boundTo(old, moved), ErrSocketMoved, "", func(f fakeFS) {
			f[lHome+"/notes/sub"] = fakeEntry{kind: 'l', to: lTmux + "/smolcorp-h1.d", uid: 1000}
		}},
	} {
		f := linuxComputer().add("/var/", docker+"@", containerd+"@", moved+"@") // /var, so the way walks root's link /var/run
		f.owners()
		f["/run/docker"] = fakeEntry{kind: 'd', perm: 0o700}
		f["/run/containerd"] = fakeEntry{kind: 'd', perm: 0o700}
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
			continue
		}
		words := argvText(args)
		if strings.Contains(words, "/dev/null /run/docker/") || strings.Contains(words, "/dev/null /var/run/") ||
			strings.Contains(words, "/run/containerd/") {
			t.Errorf("%s: something is masked or brought back behind a folder the account can't open:\n%s", c.name, words)
		}
		if !slices.ContainsFunc(logged, func(l string) bool { return strings.Contains(l, "can't open "+c.closed+",") }) {
			t.Errorf("%s: logged %q, want %s named", c.name, logged, c.closed)
		}
		// containerd's folder, which holds a socket Smolcorp always masks, is covered with nothing in it brought back: one
		// made there later isn't reached either.
		mounts, _ := playView(t, args)
		if !slices.Contains(mounts, mounted{"cover", "/run/containerd"}) || says(mounts, containerd, Connect) != Deny {
			t.Errorf("%s: /run/containerd isn't covered:\n%s", c.name, words)
		}
	}
}
