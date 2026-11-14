package desktop

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// standInTailscalePeer has every request the router is asked directly (no connection behind it) that looks as one
// through serve.sock does (asServeSockIs) taken as one from Tailscale's own process, for the rest of the test (a stand-in peer): the real
// `tailscale serve`, as SN1 lets it in. On a Mac too, for the test's length.
func standInTailscalePeer(t *testing.T) {
	t.Helper()
	savedPeer, savedMac := servedUnrecorded, macTailscale
	t.Cleanup(func() { servedUnrecorded, macTailscale = savedPeer, savedMac })
	servedUnrecorded = asServeSockIs
	macTailscale = true
}

// asServeSockIs says whether a request the router is asked directly looks as one through serve.sock does to a
// stand-in: with no address (a unix socket's request has none), or the loopback's, where `tailscale serve` handed it
// on before SN1. Never the Wi-Fi's or the tailnet's.
func asServeSockIs(r *http.Request) bool {
	if r.RemoteAddr == "" {
		return true
	}
	from, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && from.Addr().IsLoopback()
}

// X-Forwarded-For counts only as `tailscale serve` sets it: one header, one address, another device of the tailnet.
// This computer's own addresses (a program here asking its own ts.net name through serve), the Wi-Fi, a list, two
// headers, an address with a zone and anything that isn't an address are each refused.
func TestOnlyAnotherTailnetDeviceIsForwardedFor(t *testing.T) {
	saved := tailscaleAddresses
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	t.Cleanup(func() { tailscaleAddresses = saved })
	for _, c := range []struct {
		values []string
		want   bool
	}{
		{[]string{"100.101.102.103"}, true},
		{[]string{"fd7a:115c:a1e0::1234"}, true},
		{[]string{" 100.101.102.103 "}, true},
		{[]string{"100.64.1.2"}, false},
		{[]string{"127.0.0.1"}, false},
		{[]string{"::1"}, false},
		{[]string{"192.168.1.30"}, false},
		{[]string{"100.128.0.1"}, false},
		{[]string{"fd7a:115c:a1e1::1"}, false},
		{[]string{"fd7a:115c:a1e0::1%en0"}, false},
		{[]string{"192.168.1.30, 100.101.102.103"}, false},
		{[]string{"100.101.102.103", "100.101.102.104"}, false},
		{[]string{""}, false},
		{[]string{"my-phone.tail1234.ts.net"}, false},
		{nil, false},
	} {
		header := http.Header{}
		for _, value := range c.values {
			header.Add("X-Forwarded-For", value)
		}
		if got := forwardedForTailnet(header); got != c.want {
			t.Errorf("X-Forwarded-For %q: %v, want %v", c.values, got, c.want)
		}
	}
}

// A request is Tailscale's only by its connection's record: through serve.sock, its other end identified as it was
// accepted (servedConn), even inside the listener's slot. Any other connection, over the loopback too, never is,
// whatever a stand-in for unrecorded requests would say.
func TestOnlyAConnectionThroughServeSockIsTailscales(t *testing.T) {
	saved := servedUnrecorded
	t.Cleanup(func() { servedUnrecorded = saved })
	servedUnrecorded = func(*http.Request) bool { return true }
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	request := func(ctx context.Context) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/events", nil)
		r.RemoteAddr = "127.0.0.1:50000"
		return r.WithContext(ctx)
	}
	for name, c := range map[string]struct {
		conn net.Conn
		want bool
	}{
		"through serve.sock":            {servedConn{server}, true},
		"through serve.sock, in a slot": {&slotConn{Conn: servedConn{server}, release: func() {}}, true},
		"over TCP":                      {server, false},
		"over TCP, in a slot":           {&slotConn{Conn: server, release: func() {}}, false},
	} {
		if got := servedByTailscale(request(recordConn(context.Background(), c.conn))); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	// No record (the router asked itself, in a test): the stand-in answers; in the app it's no.
	if !servedByTailscale(request(context.Background())) {
		t.Error("the stand-in for an unrecorded request wasn't asked")
	}
	servedUnrecorded = saved
	if servedByTailscale(request(context.Background())) {
		t.Error("an unrecorded request counted as Tailscale's in the app")
	}
}

// serve.sock's listener hands the server only what it identified as Tailscale's: anything else is closed before a
// byte of it is read, and the next connection is looked at.
func TestServeSockClosesWhatIsntTailscales(t *testing.T) {
	saved := identifyServePeer
	t.Cleanup(func() { identifyServePeer = saved })
	identifyServePeer = func(conn net.Conn) bool { return conn.(*pipeEnd).name == "tailscale" }
	refused, tailscale := &pipeEnd{name: "program"}, &pipeEnd{name: "tailscale"}
	listener := &serveListener{Listener: &listOfConns{conns: []net.Conn{refused, tailscale}}}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if served, ok := conn.(servedConn); !ok || served.Conn != tailscale {
		t.Errorf("accepted %#v, want Tailscale's, marked", conn)
	}
	if !refused.closed || tailscale.closed {
		t.Errorf("closed: the program's %v, Tailscale's %v", refused.closed, tailscale.closed)
	}
	if _, err := listener.Accept(); err == nil {
		t.Error("accepted past the end")
	}
}

// pipeEnd is a connection that only remembers whether it was closed.
type pipeEnd struct {
	net.Conn
	name   string
	closed bool
}

func (p *pipeEnd) Close() error { p.closed = true; return nil }

// listOfConns is a listener that accepts its connections in turn, then fails.
type listOfConns struct {
	net.Listener
	conns []net.Conn
}

func (l *listOfConns) Accept() (net.Conn, error) {
	if len(l.conns) == 0 {
		return nil, net.ErrClosed
	}
	conn := l.conns[0]
	l.conns = l.conns[1:]
	return conn, nil
}

// The deck's server records each connection as it's accepted: whether it came through serve.sock, identified.
func TestTheServerRecordsEachConnection(t *testing.T) {
	d := newTestDeck(t)
	if newServer(d).ConnContext == nil {
		t.Fatal("the server records no connection: a request through serve.sock couldn't say so")
	}
}
