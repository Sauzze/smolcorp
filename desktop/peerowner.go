package desktop

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
)

// Only the real `tailscale serve` counts (1.26, design B, SN1). `tailscale serve` hands a phone's request on with a
// proxy's headers and the ts.net name the phone asked for. Any program here can send the same headers and the same
// Host, so neither is ever enough on its own. Since SN1 serve hands the deck on to serve.sock, a unix socket in the
// support folder, and only there: a request counts as Tailscale's only when it came through serve.sock and the
// process at the other end was identified positively, as it was accepted, as Tailscale's own. Anything not identified
// is closed unanswered. Over this computer's TCP loopback nothing counts as `tailscale serve` any more.
//
//   - On a Mac, the kernel says who connected (peerowner_darwin.go): connected as root, running as root, and
//     Tailscale's network extension by its signature. So only Tailscale's standalone build counts: the Mac App Store's
//     runs as the owner.
//   - On Linux and Windows there is no serve.sock yet, and every hand-on is refused (serveHandOnHere is off). Turning
//     it on is its own piece, after SN1.
//
// Then the device `tailscale serve` names in X-Forwarded-For (it sets that header itself, from the real source) must
// be in the tailnet and not this computer, so a program here that asks for its own ts.net name through serve is
// refused too. A ts.net Host with no proxy's headers (a TCP forward) is let in as a phone on the Wi-Fi is, never to
// the rows over Tailscale only, the typing rows or the owner's tap: nobody says whom it is for.

// serveSocketName is serve.sock's name in the support folder.
const serveSocketName = "serve.sock"

// serveSocketPath is where `tailscale serve` hands the deck on: serve.sock in the support folder.
func serveSocketPath() string { return filepath.Join(supportDir, serveSocketName) }

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
