package sandbox

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

// The socket mask (SA-L3, SANDBOX-PLAN §A3.2 "Unix sockets denied by default", NB5): a read-only bind doesn't stop a
// connect, so at each start the view reads the computer's list of unix sockets (/proc/net/unix) and masks every
// pathname socket that listens there and isn't on the desk's list:
//   - in a folder the view covers whole ($XDG_RUNTIME_DIR, every /run/user/<uid>, /run/screen, the private /tmp, the
//     support folder), it needs nothing more: it isn't there;
//   - in a folder the view covers with what the Policy allows brought back (the home folder, a folder holding a name
//     the Policy denies reading), it isn't brought back;
//   - anywhere else, /dev/null goes over it, read-only: a connect is refused, and the desk can't remove or replace it.
//
// The list is read strictly (FXSAL3R1, RT126SAL3 B1, B2): the kernel's header first, then each line exactly as
// unix_seq_show writes it. A list that can't be read, has no header or no socket, or holds a line that doesn't read
// so, fails the start (ErrNoSocketList). The kernel writes a name byte for byte, a newline too, so each socket is
// checked against the kernel's own dump (NETLINK_SOCK_DIAG, with each name's length and the inode of the file it is
// bound to): a name that differs between the two fails the start, and so does a socket whose name no longer leads to
// its file (moved, removed or replaced: B4) where the desk could have changed the way to it (ErrSocketMoved). A
// socket closed between the two reads is gone.
//
// The sockets Smolcorp always masks (SA-X2's LinuxMasked: the system D-Bus, Docker's, containerd's, Podman's) are
// masked whether or not they listen at the start, whatever the list says, and the folders they are in, or would be
// in, if no one but a system account has them, are covered with all else in them brought back (/run, for
// /run/docker.sock; /run/dbus, root's or messagebus's): one made there after the start (Docker started, or started
// again) isn't seen.
//
// The list is the agents brought back (View.Agents: trusted), the desk's own socket folder, and what the Policy allows
// connecting to by a row of its own (the desk-start socket). An agent is trusted only when each link on its way sits
// where the desk can't change it (B3: a link in a folder the view brings back writable is the desk's own words), and
// no place on its way is a bus (busPlace) or a socket Smolcorp always masks; one that another agent's untrusted link
// leads to isn't trusted either. A socket that listens by a relative name (bound from inside its folder) can't be
// found by its path: each one is logged (View.Log), one starting with @ too, and left as it is. An abstract one (@…,
// D-Bus's @/tmp/dbus-*, X11's; the kernel's dump tells it from a relative @…) isn't reached at all: the view has its
// own network namespace, where abstract names live. The way to a socket's name counts as the list's only where the
// desk can't change it (RT126SAL3R2 B4-R2): through a link it planted, a socket is on no list.
//
// A desk runs as the owner's account, with nothing more (VM run 3 B1, B2; SIERRA's ruling): a place behind a folder
// that account can't open (root's, writable by no group nor other, where no desk can change a name in it or above it:
// closedAbove) is nothing a desk reaches, nor can have moved. A socket listening there (Docker's
// /var/run/docker/libnetwork/<id>.sock, by root's link /var/run) decides nothing: no refusal, no mask, a line in the
// log. A folder like that the view covers (containerd's /run/containerd) is covered with nothing brought back. Anything
// else the account can't open, a folder of the owner's, or one a desk could rename into place, still fails the start.
//
// Said plainly (§A3.2 step 4, §A4.4): a socket made after the list is read (between the read and bwrap's start, or
// while the desk runs), or made again by its program where /dev/null went over the old one (the kernel lets go of a
// mount on a name removed or renamed over outside), is reachable until the desk's next start, unless a cover holds
// its folder.

// Errors the mask gives.
var (
	// ErrNoSocketList is a start that can't read the computer's list of unix sockets as the kernel writes it: the view
	// would mask none of them, or not all.
	ErrNoSocketList = errors.New("sandbox: the computer's unix sockets (" + netUnix + ") can't be read as the kernel writes them")
	// ErrSocketMoved is a socket that listens, whose name no longer leads to it, where the desk could have moved it.
	ErrSocketMoved = errors.New("sandbox: a socket that listens isn't where it was made, and a desk could have moved it (restart the program that listens there)")
)

// netUnix is where Linux lists the unix sockets of the core's network namespace, every user's.
const netUnix = "/proc/net/unix"

// netUnixHeader is the first line the kernel writes there.
const netUnixHeader = "Num       RefCount Protocol Flags    Type St Inode Path"

// acceptCon is the flag netUnix gives a socket that listens (__SO_ACCEPTCON).
const acceptCon = 1 << 16

// sockDgram is netUnix's type for a datagram socket: one bound to a path takes what anyone sends it, without listening.
const sockDgram = 2

func (osView) unixSockets() ([]byte, error) { return os.ReadFile(netUnix) }
func (osView) unixDiag() ([]byte, error)    { return readUnixDiag() }
func (osView) environ() []string            { return os.Environ() }

// netSocket is a socket netUnix lists as reachable by a name: its inode and its name as bound.
type netSocket struct {
	ino  uint64
	name string
}

// parseNetUnix is what the text of netUnix lists as reachable by a name: the sockets that listen (stream or seqpacket)
// and the datagram sockets bound to a name, a whole path's, a relative one or an abstract one (the text writes an
// abstract name and a relative one starting with @ alike: the kernel's dump tells them apart). A socket bound to none
// is left out. Text that isn't the kernel's (no header, no socket, a line it doesn't write) is an error.
func parseNetUnix(text []byte) ([]netSocket, error) {
	lines := strings.Split(string(text), "\n")
	if len(lines) < 3 || lines[0] != netUnixHeader || lines[len(lines)-1] != "" {
		return nil, errors.New("not the kernel's list: no header, no socket, or no end to its last line")
	}
	var out []netSocket
	for i, line := range lines[1 : len(lines)-1] {
		s, reached, ok := netUnixLine(line)
		if !ok {
			return nil, fmt.Errorf("line %d isn't one the kernel writes: %q", i+2, line)
		}
		if reached {
			out = append(out, s)
		}
	}
	return out, nil
}

// netUnixLine reads one line as unix_seq_show writes it, "%pK: %08X %08X %08X %04X %02X %5lu", then a space and the
// name for a bound socket; reached is a socket that listens, or a bound datagram one.
func netUnixLine(line string) (s netSocket, reached, ok bool) {
	num, rest, ok := strings.Cut(line, ": ")
	if !ok || len(num) != 8 && len(num) != 16 || !isHex(num) {
		return s, false, false
	}
	var words [5]string // RefCount, Protocol, Flags, Type, St
	for i, width := range []int{8, 8, 8, 4, 2} {
		if len(rest) <= width || rest[width] != ' ' || !isHex(rest[:width]) {
			return s, false, false
		}
		words[i], rest = rest[:width], rest[width+1:]
	}
	digits := strings.TrimLeft(rest, " ")
	pad, n := len(rest)-len(digits), len(digits)-len(strings.TrimLeft(digits, "0123456789"))
	if n == 0 || pad+n < 5 || pad > 0 && pad+n != 5 {
		return s, false, false
	}
	ino, err := strconv.ParseUint(digits[:n], 10, 64)
	if err != nil {
		return s, false, false
	}
	if rest = digits[n:]; rest != "" {
		if s.name, ok = strings.CutPrefix(rest, " "); !ok || s.name == "" {
			return s, false, false
		}
	}
	s.ino = ino
	flags, _ := strconv.ParseUint(words[2], 16, 32)
	kind, _ := strconv.ParseUint(words[3], 16, 16)
	return s, s.name != "" && (flags&acceptCon != 0 || kind == sockDgram), true
}

func isHex(s string) bool {
	return s != "" && strings.Trim(s, "0123456789abcdefABCDEF") == ""
}

// The kernel's dump of its unix sockets (linux/sock_diag.h, linux/unix_diag.h).
const (
	sockDiag          = 4  // NETLINK_SOCK_DIAG
	sockDiagByFamily  = 20 // SOCK_DIAG_BY_FAMILY
	nlmsgError        = 2  // NLMSG_ERROR
	nlmsgDone         = 3  // NLMSG_DONE
	unixDiagName      = 0  // UNIX_DIAG_NAME: the name, as many bytes as it was bound with
	unixDiagVFS       = 1  // UNIX_DIAG_VFS: the inode and device of the file it is bound to
	udiagShowNameVFS  = 1 | 2
	nlmRequestAndDump = 1 | 0x300 // NLM_F_REQUEST | NLM_F_DUMP
)

// unixDiagRequest asks for every unix socket, in every state, with its name and its file.
func unixDiagRequest() []byte {
	req := make([]byte, 16+24)
	ne := binary.NativeEndian
	ne.PutUint32(req[0:], uint32(len(req)))
	ne.PutUint16(req[4:], sockDiagByFamily)
	ne.PutUint16(req[6:], nlmRequestAndDump)
	req[16] = 1                        // AF_UNIX
	ne.PutUint32(req[20:], 0xffffffff) // every state; no one inode (req[24:]), so every socket
	ne.PutUint32(req[28:], udiagShowNameVFS)
	return req
}

// diagSocket is a socket in the kernel's dump: its name as netUnix writes it, and its file's inode.
type diagSocket struct {
	name     string
	vfs      uint32
	file     bool // bound to a file: vfs is its inode
	abstract bool // an abstract name (its first byte a NUL): netUnix writes it with an @, as it would a relative @…
}

// parseUnixDiag reads the kernel's dump, by socket inode. Anything that doesn't read as the kernel writes it, an
// error the kernel gives, or a dump that doesn't end, is an error.
func parseUnixDiag(dump []byte) (map[uint64]diagSocket, error) {
	out := map[uint64]diagSocket{}
	done, err := diagMessages(dump, func(body []byte) error {
		if len(body) < 16 {
			return errors.New("a socket's message too short")
		}
		ino := uint64(binary.NativeEndian.Uint32(body[4:]))
		var d diagSocket
		for attrs := body[16:]; len(attrs) > 0; {
			if len(attrs) < 4 {
				return errors.New("a socket's attribute too short")
			}
			size, kind := int(binary.NativeEndian.Uint16(attrs)), binary.NativeEndian.Uint16(attrs[2:])&0x3fff
			if size < 4 || size > len(attrs) {
				return errors.New("a socket's attribute of the wrong length")
			}
			value := attrs[4:size]
			switch kind {
			case unixDiagName:
				d.name, d.abstract = netUnixName(value), len(value) > 0 && value[0] == 0
			case unixDiagVFS:
				if len(value) < 8 {
					return errors.New("a socket's file too short")
				}
				d.vfs, d.file = binary.NativeEndian.Uint32(value), true
			}
			attrs = attrs[min(align4(size), len(attrs)):]
		}
		out[ino] = d
		return nil
	})
	if err == nil && !done {
		err = errors.New("the dump doesn't end")
	}
	return out, err
}

// diagEnded is whether one read of the dump holds its end, or the kernel's error.
func diagEnded(chunk []byte) (bool, error) {
	return diagMessages(chunk, func([]byte) error { return nil })
}

// diagMessages gives each socket's message in b to each, up to the dump's end.
func diagMessages(b []byte, each func(body []byte) error) (done bool, err error) {
	for len(b) > 0 {
		if len(b) < 16 {
			return false, errors.New("a message too short")
		}
		size, kind := int(binary.NativeEndian.Uint32(b)), binary.NativeEndian.Uint16(b[4:])
		if size < 16 || size > len(b) {
			return false, errors.New("a message of the wrong length")
		}
		body := b[16:size]
		switch kind {
		case nlmsgDone:
			return true, nil
		case nlmsgError:
			if len(body) < 4 {
				return false, errors.New("the kernel's error too short")
			}
			return false, fmt.Errorf("the kernel refused the dump: errno %d", -int32(binary.NativeEndian.Uint32(body)))
		case sockDiagByFamily:
			if err := each(body); err != nil {
				return false, err
			}
		default:
			return false, fmt.Errorf("a message of type %d", kind)
		}
		b = b[min(align4(size), len(b)):]
	}
	return false, nil
}

func align4(n int) int { return (n + 3) &^ 3 }

// netUnixName is a name from the dump as netUnix writes it: a path's last byte (its NUL) left out, each NUL an @.
func netUnixName(name []byte) string {
	if len(name) > 0 && name[0] != 0 {
		name = name[:len(name)-1]
	}
	return strings.ReplaceAll(string(name), "\x00", "@")
}

// agentAt is an agent the view trusts: as spelled, and the socket its way leads to.
type agentAt struct{ at, real string }

// trusted are the agents whose way the desk can't change and that go through no place never names (B3, and SIERRA's
// class ruling: no bus, no systemd --user, no socket Smolcorp always masks, at any hop). One that another agent's
// untrusted way leads to (a link the desk planted, followed by whoever listed it) is left out too.
func (b *viewBuilder) trusted(agents []string) []agentAt {
	var out []agentAt
	planted := map[string]bool{}
	for _, a := range agents {
		at := spell("linux", a)
		if at == "" {
			continue
		}
		if real, ok := way(b.sys, at, b.fixed, b.never); ok {
			out = append(out, agentAt{at, real})
		} else if real, _ := way(b.sys, at, func(string) bool { return true }, func(string) bool { return false }); real != "" {
			planted[real] = true
		}
	}
	return slices.DeleteFunc(out, func(a agentAt) bool { return planted[a.at] || planted[a.real] })
}

// never is a place no agent's way goes through: a bus or systemd --user's own (busPlace), or a socket Smolcorp always
// masks.
func (b *viewBuilder) never(at string) bool {
	return busPlace(at, b.runtimes) || slices.ContainsFunc(LinuxMasked(), func(m Place) bool { return spell("linux", m.At) == at })
}

// fixed is whether the desk can change no name in folder, nor in any folder above it: none is the disk's in its view
// and writable by it.
func (b *viewBuilder) fixed(folder string) bool {
	for ; ; folder = path.Dir(folder) {
		if b.changeable(folder) {
			return false
		}
		if folder == "/" {
			return true
		}
	}
}

// changeable is whether the desk can make, remove or rename a name in folder on the disk: its view binds it from the
// disk, writable (no cover, nothing read-only, nearer), and it isn't root's alone. A folder that isn't there is
// changeable where the folder it would be in is.
func (b *viewBuilder) changeable(folder string) bool {
	for at := folder; ; at = path.Dir(at) {
		if o, ok := b.ops[at]; ok {
			if o.kind != vBind {
				return false
			}
			break
		}
		if at == "/" {
			break
		}
	}
	st, err := b.sys.stat(folder, false)
	if notThere(err) {
		return folder != "/" && b.changeable(path.Dir(folder))
	}
	return err != nil || !(st.uid == 0 && st.mode.Perm()&0o022 == 0)
}

// closedAbove is the folder the owner's account can't open that err, a refusal (EACCES) to look at at, comes from: the
// nearest folder from at up that can be looked at. It is closed when it is root's, a folder, writable by no group nor
// other, and fixed: no desk can change a name in it or above it (it can't be renamed into place, nor replaced). A desk
// runs as the owner's account with nothing more, so what that account can't reach there, no desk reaches, nor can
// have moved (VM run 3 B1, B2; SIERRA's ruling). Any other error, or a refusal from a folder of the owner's or one a
// desk could change, isn't closed: the caller fails closed on it, as before.
func (b *viewBuilder) closedAbove(at string, err error) (string, bool) {
	if !errors.Is(err, fs.ErrPermission) {
		return "", false
	}
	for folder := at; ; folder = path.Dir(folder) {
		st, err := b.sys.stat(folder, false)
		switch {
		case err == nil:
			return folder, st.mode.IsDir() && st.uid == 0 && st.mode.Perm()&0o022 == 0 && b.fixed(folder)
		case !errors.Is(err, fs.ErrPermission) || folder == "/":
			return "", false
		}
	}
}

// unreached is whether err, from looking at at, comes from a folder closed to the owner's account (closedAbove): then
// at is nothing a desk reaches, and the start says so, once for each such folder.
func (b *viewBuilder) unreached(at string, err error) bool {
	folder, closed := b.closedAbove(at, err)
	if closed && !b.closedLogged[folder] {
		b.closedLogged[folder] = true
		b.logf("sandbox: this account can't open %s, so no desk reaches %s or anything else in it: nothing in it is masked, brought back or refused", folder, at)
	}
	return closed
}

// way follows at link by link, as the kernel does, to where it leads: ok only when every link on the way sits in a
// folder fixed holds, and no place on the way is one never names. A name that isn't there ends the way as written.
func way(sys viewFS, at string, fixed func(folder string) bool, never func(string) bool) (string, bool) {
	done, rest, trusted := "/", strings.Split(strings.TrimPrefix(at, "/"), "/"), true
	for hops := 0; len(rest) > 0; {
		name := rest[0]
		rest = rest[1:]
		switch name {
		case "", ".":
			continue
		case "..":
			done = path.Dir(done)
			continue
		}
		next := path.Join(done, name)
		if never(next) {
			return next, false
		}
		st, err := sys.stat(next, false)
		if err != nil {
			end := path.Join(append([]string{next}, rest...)...)
			return end, trusted && !never(end)
		}
		if st.mode&fs.ModeSymlink == 0 {
			done = next
			continue
		}
		to, err := sys.link(next)
		if hops++; err != nil || hops > 40 {
			return "", false
		}
		trusted = trusted && fixed(done)
		if strings.HasPrefix(to, "/") {
			done = "/"
		}
		rest = append(strings.Split(to, "/"), rest...)
	}
	return done, trusted && !never(done)
}

// busPlace is whether at is a session bus or systemd --user's own (SIERRA's class ruling): `bus`, or anything under
// `systemd/`, in a runtime folder (each /run/user/<uid>, and each of runtimes).
func busPlace(at string, runtimes []string) bool {
	if rest, ok := strings.CutPrefix(at, "/run/user/"); ok {
		uid, _, _ := strings.Cut(rest, "/")
		runtimes = append([]string{"/run/user/" + uid}, runtimes...)
	}
	return slices.ContainsFunc(runtimes, func(r string) bool {
		return r != "" && r != "/" && (at == path.Join(r, "bus") || under(at, path.Join(r, "systemd")) && at != path.Join(r, "systemd"))
	})
}

// busEnv are the names the desk's environment never holds (SIERRA's class ruling): D-Bus's own, besides any other
// DBUS_* the core has, the accessibility bus's address (a D-Bus bus) and systemd's notify socket (RT126SAL3R2 F4).
var busEnv = []string{"AT_SPI_BUS_ADDRESS", "DBUS_SESSION_BUS_ADDRESS", "DBUS_STARTER_ADDRESS", "DBUS_STARTER_BUS_TYPE",
	"DBUS_SYSTEM_BUS_ADDRESS", "NOTIFY_SOCKET"}

// unsetEnv are the names bwrap takes out of the desk's environment: busEnv, and each DBUS_* in env.
func unsetEnv(env []string) []string {
	names := slices.Clone(busEnv)
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "DBUS_") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// maskSockets masks each socket netUnix lists that isn't on the desk's list, and the ones Smolcorp always masks.
func (b *viewBuilder) maskSockets(pl Places, v View, agents []agentAt) error {
	text, err := b.sys.unixSockets()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoSocketList, err)
	}
	listening, err := parseNetUnix(text)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoSocketList, err)
	}
	dump, err := b.sys.unixDiag()
	if err != nil {
		return fmt.Errorf("%w: the kernel's dump: %v", ErrNoSocketList, err)
	}
	files, err := parseUnixDiag(dump)
	if err != nil {
		return fmt.Errorf("%w: the kernel's dump: %v", ErrNoSocketList, err)
	}
	listed := map[string]bool{}
	for _, a := range agents {
		listed[a.at], listed[a.real] = true, true
	}
	own := spell("linux", pl.OwnSockets)
	logged := map[string]bool{}
	for _, s := range listening {
		d, ok := files[s.ino]
		switch {
		case !ok: // closed between the two reads
		case d.name != s.name:
			return fmt.Errorf("%w: socket %d is %q on the list, %q in the kernel's dump", ErrNoSocketList, s.ino, s.name, d.name)
		case d.abstract: // in the core's network namespace: the view has its own
		case !strings.HasPrefix(s.name, "/"): // RT126SAL3R2 F6: a relative name starting with @ too
			if !logged[s.name] {
				logged[s.name] = true
				b.logf("sandbox: a unix socket listens by the relative name %q: the view can't find it to mask it", s.name)
			}
		default:
			if err := b.mask(s.name, d, listed, own); err != nil {
				return err
			}
		}
	}
	for _, m := range LinuxMasked() {
		b.maskAlways(m.At)
	}
	return nil
}

// mask masks the socket that listens at, where its folder's links lead, unless it is on the list. When the name no
// longer leads to it (d's file), it may be anywhere: where the desk could have changed the way to it, the start fails.
func (b *viewBuilder) mask(at string, d diagSocket, listed map[string]bool, own string) error {
	if at = spell("linux", at); at == "" || at == "/" {
		return nil
	}
	folder := b.realFolder(path.Dir(at))
	real := path.Join(folder, path.Base(at))
	// RT126SAL3R2 B4-R2: where the way to real passes through a link the desk can change, real is the desk's word, not
	// the socket's: it is on no list (a link at a moved socket's old folder, into its own socket folder, an agent's or
	// the support folder, would make it so), and a socket not at its file there fails the start.
	_, trustedWay := way(b.sys, at, b.fixed, func(string) bool { return false })
	if v, row := b.p.Decide(real, Connect); trustedWay && (listed[real] || own != "" && under(real, own) || v == Allow && row.Row != "normal work") {
		return nil
	}
	st, err := b.sys.stat(real, false)
	// VM run 3 B1: behind a folder the owner's account can't open, by a way no desk changes, the socket is nothing a desk
	// reaches, nor can have moved. Through a desk's link, it is the moved check's, as before.
	if trustedWay && b.unreached(real, err) {
		return nil
	}
	socket := err == nil && st.mode&fs.ModeSocket != 0
	if !socket || !d.file || uint32(st.ino) != d.vfs {
		if !trustedWay || !b.fixed(folder) {
			return fmt.Errorf("%w: %s", ErrSocketMoved, at)
		}
	}
	if socket {
		b.hideSocket(real)
	}
	return nil
}

// maskAlways masks a socket Smolcorp always masks, there or not, whatever the list says, and covers the folder it is
// or would be in (coverRoots).
func (b *viewBuilder) maskAlways(at string) {
	at = spell("linux", at)
	folder := b.realFolder(path.Dir(at))
	real := path.Join(folder, path.Base(at))
	if st, err := b.sys.stat(real, false); err == nil && st.mode&fs.ModeSocket != 0 {
		b.hideSocket(real)
	}
	b.coverRoots(folder)
}

// hideSocket puts /dev/null over the socket at, or leaves it out of the cover it is in.
func (b *viewBuilder) hideSocket(at string) {
	o, ok := b.ops[at]
	if !ok {
		o = viewOp{at: at}
	}
	o.kind = vHide
	b.ops[at] = o
}

// coverRoots covers the nearest folder there is from folder up (but /), when only a system account has it: no
// group or other may write it, and the owner doesn't own it (root's /run, or a messagebus's /run/dbus). All else in it
// is brought back.
func (b *viewBuilder) coverRoots(folder string) {
	for ; folder != "/"; folder = path.Dir(folder) {
		st, err := b.sys.stat(folder, false)
		if notThere(err) {
			continue
		}
		if err == nil && st.mode.IsDir() && st.uid != b.ownerUID && st.uid >= 0 && st.mode.Perm()&0o022 == 0 && !b.rootCovers[folder] {
			b.rootCovers[folder] = true
			b.masks = append(b.masks, folder)
		}
		return
	}
}

// realFolder is folder with every link on it resolved, as far as it is there.
func (b *viewBuilder) realFolder(folder string) string {
	if real, err := b.sys.real(folder); err == nil {
		return real
	} else if folder == "/" {
		return folder
	}
	return path.Join(b.realFolder(path.Dir(folder)), path.Base(folder))
}
