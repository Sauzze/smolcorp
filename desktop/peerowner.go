package desktop

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Only the real `tailscale serve` counts (1.26, design B, SN1). `tailscale serve` hands a phone's request on with a
// proxy's headers and the ts.net name the phone asked for. Any program here can send the same headers and the same
// Host, so neither is ever enough on its own. Since SN1 serve hands the deck on to serve.sock, a unix socket in the
// support folder (on Windows, the serve port), and only there: a request counts as Tailscale's only when it came
// through serve.sock and the process at the other end was identified positively, as it was accepted, as Tailscale's
// own. Anything not identified is closed unanswered. Over this computer's TCP loopback to the deck's own port nothing
// counts as `tailscale serve` any more, on any system.
//
//   - On a Mac, the kernel says who connected (peerowner_darwin.go): connected as root, running as root, and
//     Tailscale's network extension by its signature. So only Tailscale's standalone build counts: the Mac App Store's
//     runs as the owner.
//   - On Linux (OLD-SERVE1), the kernel says who connected too (peerowner_linux.go): connected as root (SO_PEERCRED),
//     and that process still there and running as root now (/proc). Linux has no signature to check, so any process
//     running as root passes this rule, not only tailscaled: one that can already do anything on this computer.
//   - On Windows (OLD-SERVE2) `tailscale serve` takes no unix: target, so it hands the deck on to a port of its own on
//     this computer only (servePort, peerowner_windows.go). Windows' own tables say who holds the other end: its row
//     in the TCP table must be established and held by the Tailscale service's process, as the service manager names
//     it (by its name, its program tailscaled.exe), and that process must have started before the accept
//     (judgeWindowsPeer). A program the owner runs can't pass for it. One running as administrator could, as it can
//     do anything on this computer: say so wherever this is explained.
//
// Then the device `tailscale serve` names in X-Forwarded-For (it sets that header itself, from the real source) must
// be in the tailnet and not this computer, so a program here that asks for its own ts.net name through serve is
// refused too. A ts.net Host with no proxy's headers (a TCP forward) is let in as a phone on the Wi-Fi is, never to
// the rows over Tailscale only, the typing rows or the owner's tap: nobody says whom it is for.

// serveSocketName is serve.sock's name in the support folder.
const serveSocketName = "serve.sock"

// serveSocketPath is where `tailscale serve` hands the deck on: serve.sock in the support folder.
func serveSocketPath() string { return filepath.Join(supportDir, serveSocketName) }

// tailscaleUID is the user Tailscale's own process runs as: root, for the Mac's standalone extension and for
// tailscaled as Linux's packages run it. A variable so a test's stand-in peer, which runs as this test's user, can be
// taken for it.
var tailscaleUID uint32 = 0

// identifyServePeer says whether the other end of a connection to serve.sock is Tailscale's own process, as this
// system can tell. Anything it can't tell is a no. A variable so a test can stand a peer in.
var identifyServePeer = identifyServePeerHere

// serveListener is serve.sock's listener: each connection is identified as it's accepted, and one whose other end
// isn't Tailscale's is closed before anything of it is read.
type serveListener struct{ net.Listener }

func (l *serveListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if identifyServePeer(conn) {
			return servedConn{conn}, nil
		}
		conn.Close()
	}
}

// servedConn is a connection to serve.sock whose other end is Tailscale's own process.
type servedConn struct{ net.Conn }

// connRecord is what the server keeps for one connection: whether it is `tailscale serve`'s, through serve.sock.
type connRecord struct{ tailscale bool }

type connRecordKey struct{}

// recordConn is the server's ConnContext: every connection gets its record as it's accepted.
func recordConn(ctx context.Context, conn net.Conn) context.Context {
	if slot, ok := conn.(*slotConn); ok {
		conn = slot.Conn
	}
	_, served := conn.(servedConn)
	return context.WithValue(ctx, connRecordKey{}, &connRecord{tailscale: served})
}

// servedUnrecorded answers for a request with no connection behind it (a test asks the router itself): no in the
// app, where every connection is recorded. A variable so a test can stand a peer in.
var servedUnrecorded = func(*http.Request) bool { return false }

// servedByTailscale says whether r came through serve.sock from Tailscale's own process: never over TCP.
func servedByTailscale(r *http.Request) bool {
	if record, ok := r.Context().Value(connRecordKey{}).(*connRecord); ok {
		return record.tailscale
	}
	return servedUnrecorded(r)
}

// forwardedForTailnet says whether X-Forwarded-For is what `tailscale serve` sets: one header, one address, a device
// in the tailnet (100.64/10, or Tailscale's own IPv6 range), and not this computer itself.
func forwardedForTailnet(header http.Header) bool {
	values := header.Values("X-Forwarded-For")
	if len(values) != 1 {
		return false
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(values[0]))
	if err != nil || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	inTailnet := ip.Is4() && tailnet.Contains(net.IP(ip.AsSlice())) || ip.Is6() && tailnet6.Contains(net.IP(ip.AsSlice()))
	return inTailnet && !thisComputersAddress(net.IP(ip.AsSlice()))
}

// thisComputersAddress says whether ip is one of this computer's own: one Tailscale gave it, or any adapter's. A
// request from one of them is a program here, whatever way it came (the own-address door).
func thisComputersAddress(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if slices.ContainsFunc(tailscaleAddresses(), func(own string) bool { return ip.Equal(net.ParseIP(own)) }) {
		return true
	}
	addresses, _ := net.InterfaceAddrs()
	return slices.ContainsFunc(addresses, func(address net.Addr) bool {
		network, ok := address.(*net.IPNet)
		return ok && network.IP.Equal(ip)
	})
}

// credPeer is what Linux says of the other end of a connection to serve.sock (peerowner_linux.go). Here, not in that
// file, so its judge is tested on every system.
type credPeer struct {
	// connectUID is the user the other end connected as (SO_PEERCRED): set by the kernel at connect, kept across exec,
	// and kept after the process ends.
	connectUID uint32
	// pid is the process that connected, as SO_PEERCRED gives it: 0 if it is in a pid namespace this one can't see.
	pid int32
	// now is that process as /proc says it is now: zero if it has gone, or /proc can't be read.
	now procUsers
}

// procUsers is a process as /proc/<pid>/status says it is now: its real and effective user, and whether it still
// runs (not a zombie, not dead).
type procUsers struct {
	realUID, effectiveUID uint32
	running               bool
}

// judgeCredPeer says whether the other end is tailscaled, as far as Linux can tell: it connected as root, and the
// process that connected is still there and runs as root now (its real and its effective user). Who connected is what
// exec can't change (RT126SN1 B1): a program of the owner's that execs a setuid-root program keeps the user it
// connected as, and is refused. A client that sent its request and ended before the accept is no process now, so it
// is refused too (RSBXPLANf F2). Root alone is the rule: tailscaled isn't signed, so any process running as root
// passes it, as it could do anything on this computer anyway. A tailscaled run as another user can't reach serve.sock
// (0600, the owner's), and one run as the owner can't be told from any program of theirs: both are refused.
func judgeCredPeer(peer credPeer) bool {
	return peer.connectUID == tailscaleUID && peer.pid > 0 && peer.now.running &&
		peer.now.realUID == tailscaleUID && peer.now.effectiveUID == tailscaleUID
}

// procUsersOf reads the text of /proc/<pid>/status: its State and Uid lines (real, effective, saved and filesystem
// users). Either missing, said twice or not as Linux writes it: not read.
func procUsersOf(status string) (procUsers, bool) {
	var users procUsers
	var state, uids bool
	for _, line := range strings.Split(status, "\n") {
		name, value, _ := strings.Cut(line, ":")
		fields := strings.Fields(value)
		switch name {
		case "State":
			if state || len(fields) == 0 {
				return procUsers{}, false
			}
			state = true
			users.running = !slices.Contains([]string{"Z", "X", "x"}, fields[0])
		case "Uid":
			if uids || len(fields) != 4 {
				return procUsers{}, false
			}
			real, realErr := strconv.ParseUint(fields[0], 10, 32)
			effective, effectiveErr := strconv.ParseUint(fields[1], 10, 32)
			if realErr != nil || effectiveErr != nil {
				return procUsers{}, false
			}
			uids = true
			users.realUID, users.effectiveUID = uint32(real), uint32(effective)
		}
	}
	if !state || !uids {
		return procUsers{}, false
	}
	return users, true
}

// serviceProcess is the Tailscale service as Windows says it is at one moment (peerowner_windows.go): its process,
// by the service manager, and that process in Windows' list of processes. Here, not in that file, so its judge is
// tested on every system.
type serviceProcess struct {
	// pid is the service's process, as the service manager says by the service's name: 0 if there is no such
	// service, or it isn't running.
	pid uint32
	// binary is the program the service manager runs for it, as the service's configuration says (quoted, maybe
	// with its arguments).
	binary string
	// image is that pid's program by name, and created when it started (Unix nanoseconds), as the process list says:
	// both zero if the list has no such process.
	image   string
	created int64
}

// tcpRow is a connection's row in Windows' TCP table (GetExtendedTcpTable): its state and the process that holds
// it. found is false when the table has no such row.
type tcpRow struct {
	found bool
	state uint32
	pid   uint32
}

// tcpEstablished is an established connection's state in Windows' TCP table (MIB_TCP_STATE_ESTAB). A client that
// has half-closed (FIN_WAIT_2 on its side) or ended (gone, or TIME_WAIT with no process) isn't.
const tcpEstablished = 5

// windowsPeer is what is read of the other end of a connection to the serve port, as it's judged: the Tailscale
// service before and after its row in the TCP table is read, so the pid the row names is the service's process at
// that moment, not one that took its pid after it ended.
type windowsPeer struct {
	accepted      int64 // when the connection was accepted (Unix nanoseconds)
	before, after serviceProcess
	row           tcpRow
}

// tailscaleService and tailscaledImage are Tailscale's service, by the name its installer gives it, and its program.
// Variables so a test can stand a process of its own in.
var (
	tailscaleService = "Tailscale"
	tailscaledImage  = "tailscaled.exe"
)

// judgeWindowsPeer says whether the other end of a connection to the serve port is the Tailscale service's own
// process, as far as Windows can tell. The service manager names the service's process, and its program is
// tailscaled.exe (an absolute path) in the service's configuration and in the process list. The client's row in the
// TCP table is established and held by that process. That process started before the accept, and is the same one
// after the row was read: a pid Windows gave again, to a process started later, is refused. Only an administrator
// can register or change a service, so a program the owner runs can't pass for it; one running as administrator could.
func judgeWindowsPeer(peer windowsPeer) bool {
	service := peer.before
	return service.pid > 0 && peer.after == service &&
		strings.EqualFold(service.image, tailscaledImage) && strings.EqualFold(windowsProgramName(service.binary), tailscaledImage) &&
		service.created > 0 && service.created < peer.accepted &&
		peer.row.found && peer.row.state == tcpEstablished && peer.row.pid == service.pid
}

// windowsAbsolute is a path on a drive, as Windows writes one: C:\...
var windowsAbsolute = regexp.MustCompile(`^[A-Za-z]:\\`)

// windowsProgramName is the name of the program a service's configured command line runs: the quoted path (its quote
// closed), or the line up to its first space, if it is an absolute path; "" otherwise.
func windowsProgramName(line string) string {
	program := ""
	if rest, quoted := strings.CutPrefix(line, `"`); quoted {
		if quotedProgram, _, closed := strings.Cut(rest, `"`); closed {
			program = quotedProgram
		}
	} else if fields := strings.Fields(line); len(fields) > 0 {
		program = fields[0]
	}
	if !windowsAbsolute.MatchString(program) || strings.HasSuffix(program, `\`) {
		return ""
	}
	return program[strings.LastIndex(program, `\`)+1:]
}
