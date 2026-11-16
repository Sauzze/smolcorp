//go:build unix

package sandbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// SA-L6 (proxy.go): a Linux desk's way out through the core. The proxy never reaches the computer's own addresses
// (but the app's port on loopback) nor any unix socket, dials the address it checked, and carries CONNECT and plain
// http:// requests; the forwarder hands the view's loopback on to it; the proxy row fails when the way out isn't the
// core's, and the desk's environment points at it. Everything here is on this computer's loopback and in a temp
// folder: names outside it are stand-ins (192.0.2.0/24, TEST-NET-1), dialled to local stand-in servers.

// Stand-in addresses: the computer's own (as its interfaces would list them), and one "out in the world".
var (
	ownV4   = netip.MustParseAddr("192.0.2.99")
	ownV6   = netip.MustParseAddr("fe80::99")
	worldV4 = netip.MustParseAddr("192.0.2.10")
	// The computer's own places, as ownPlaces would give them: its two addresses, a container bridge's network, and
	// tailscale0's two addresses (a /32 and a /128, as tailscaled sets them).
	ownAddrs = func() ([]netip.Prefix, error) {
		return []netip.Prefix{netip.PrefixFrom(ownV4, 32), netip.PrefixFrom(ownV6, 128), netip.MustParsePrefix("172.18.0.0/16"),
			netip.MustParsePrefix("100.101.102.103/32"), netip.MustParsePrefix("fd7a:115c:a1e0::1234/128")}, nil
	}
)

// countingServer is a stand-in local service of the owner's, on this computer's loopback: it counts each connection
// it gets, and answers none.
func countingServer(t *testing.T) (port int, got *atomic.Int64) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	got = &atomic.Int64{}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			got.Add(1)
			c.Close()
		}
	}()
	return l.Addr().(*net.TCPAddr).Port, got
}

// echoServer is a stand-in server out in the world: it sends back what it is sent.
func echoServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { io.Copy(c, c); c.Close() }()
		}
	}()
	return l.Addr().String()
}

// serveProxy listens as p on a new socket in a temp folder, until the test ends, and gives the socket.
func serveProxy(t *testing.T, p *Proxy) string {
	t.Helper()
	at := filepath.Join(shortTemp(t), ProxySocketName)
	l, err := ListenProxy(at)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go p.Serve(l)
	return at
}

// forwarder hands a loopback port on to the proxy at socket, as the view's forwarder does, until the test ends.
func forwarder(t *testing.T, socket string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go Forward(l, socket)
	return l.Addr().String()
}

// ask sends head (and more) to the proxy at socket, and gives the status code of its answer and the connection, open.
func ask(t *testing.T, socket, head string) (int, *bufio.Reader, net.Conn) {
	t.Helper()
	c, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := io.WriteString(c, head); err != nil {
		t.Fatal(err)
	}
	in := bufio.NewReader(c)
	method := http.MethodGet
	if strings.HasPrefix(head, "CONNECT ") {
		method = http.MethodConnect
	}
	resp, err := http.ReadResponse(in, &http.Request{Method: method})
	if err != nil {
		t.Fatalf("%q: no answer: %v", head, err)
	}
	if method != http.MethodConnect || resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return resp.StatusCode, in, c
}

// dialLog records each address a stand-in proxy dials, and dials to.
type dialLog struct {
	mu    sync.Mutex
	dials []netip.AddrPort
}

func (d *dialLog) dialTo(to string) func(context.Context, netip.AddrPort) (net.Conn, error) {
	return func(ctx context.Context, at netip.AddrPort) (net.Conn, error) {
		d.mu.Lock()
		d.dials = append(d.dials, at)
		d.mu.Unlock()
		var dialer net.Dialer
		return dialer.DialContext(ctx, "tcp", to)
	}
}

func (d *dialLog) all() []netip.AddrPort {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.dials)
}

// The proxy refuses every one of the computer's own addresses, by any spelling and through any name that leads to
// one, whatever the port but the app's: 403, nothing dialled, the owner's service never reached. With the stand-in
// dial, each refused request would otherwise reach that service; with the real one too, for loopback.
func TestTheProxyRefusesTheComputersOwnAddresses(t *testing.T) {
	ownerPort, got := countingServer(t)
	owner := strconv.Itoa(ownerPort)
	appPort, _ := countingServer(t) // the app's port is another: the owner's service isn't it
	targets := []string{
		"127.0.0.1:" + owner, "127.0.0.2:" + owner, "localhost:" + owner, "[::1]:" + owner, "0.0.0.0:" + owner,
		"0.1.2.3:" + owner, "[::]:" + owner, "[::ffff:127.0.0.1]:" + owner, "[::ffff:192.0.2.99]:" + owner,
		"rebind.test:" + owner, "192.0.2.99:" + owner, "own.test:443", "[fe80::99%25eth0]:" + owner, "[fe80::99]:22",
		"224.0.0.1:" + owner, "255.255.255.255:" + owner, "nothing.test:443", "127.0.0.1:2375", "127.0.0.1:22",
		"172.18.0.3:5432", "supabase-db.test:5432", // a local stack's database, in a container on the computer's bridge
		"100.101.102.103:" + owner, "[fd7a:115c:a1e0::1234]:" + owner, // its own tailnet addresses
	}
	// This computer's own services off its interfaces (RT126SAL6 B1): at addresses no interface holds, answered by
	// this computer alone. Tailscale's quad100 (its web interface, its DNS), a cloud VM's metadata service by address
	// and by name, AWS's on IPv6, Alibaba Cloud's and Azure's, a link peer, and each of them by another spelling
	// (IPv4 as IPv6, under NAT64's prefix). With the stand-in dial only: these must never reach the real network.
	offInterfaces := []string{
		"100.100.100.100:80", "100.100.100.100:53", "[fd7a:115c:a1e0::53]:53", "169.254.169.254:80", "metadata.test:80",
		"169.254.170.2:80", "169.254.1.1:" + owner, "[fd00:ec2::254]:80", "[fd00:ec2::253]:53", "100.100.100.200:80",
		"168.63.129.16:80", "[fe80::1%25eth0]:22", "[fe80::1]:" + owner, "[::ffff:169.254.169.254]:80",
		"[::ffff:100.100.100.100]:80", "[64:ff9b::a9fe:a9fe]:80", "[64:ff9b::6464:6464]:53",
		"[64:ff9b::7f00:1]:" + strconv.Itoa(appPort),   // loopback under NAT64's prefix, even at the app's port
		"[64:ff9b::c000:263]:" + owner,                 // the computer's own LAN address (192.0.2.99) under NAT64's prefix
		"[fd20:ce::254]:80", "[fd00:c1::a9fe:a9fe]:80", // Google Cloud's and Oracle Cloud's metadata services on IPv6
	}
	resolve := func(_ context.Context, host string) ([]netip.Addr, error) {
		switch host {
		case "localhost":
			return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, nil
		case "rebind.test": // a name whose answer is this computer's loopback
			return []netip.Addr{netip.MustParseAddr("127.0.0.53")}, nil
		case "own.test":
			return []netip.Addr{ownV4}, nil
		case "supabase-db.test":
			return []netip.Addr{netip.MustParseAddr("172.18.0.3")}, nil
		case "metadata.test": // a cloud's metadata name (metadata.google.internal, instance-data.ec2.internal)
			return []netip.Addr{netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("fd00:ec2::254")}, nil
		case "nothing.test":
			return nil, nil
		}
		return nil, errors.New("no such stand-in name")
	}
	for _, real := range []bool{false, true} {
		d := &dialLog{}
		p := &Proxy{AppPort: appPort, own: ownAddrs, resolve: resolve, dial: d.dialTo("127.0.0.1:" + owner), Log: t.Logf}
		if real {
			p.dial = nil
		}
		socket := serveProxy(t, p)
		rows := targets
		if !real {
			rows = slices.Concat(targets, offInterfaces)
		}
		for _, to := range rows { // a zone in an address is written %25 in a request's line (RFC 6874)
			words := "the computer's own address"
			if slices.Contains(offInterfaces, to) {
				words = "this computer's own service off its interfaces"
			}
			for _, head := range []string{
				"CONNECT " + to + " HTTP/1.1\r\nHost: " + to + "\r\n\r\n",
				"GET http://" + to + "/ HTTP/1.1\r\nHost: " + to + "\r\n\r\n",
			} {
				if status, _, _ := ask(t, socket, head); status != http.StatusForbidden {
					t.Errorf("%s, real dial %v, %q: %d, not 403", words, real, head, status)
				}
			}
		}
		if dials := d.all(); len(dials) != 0 {
			t.Errorf("the proxy dialled the computer's own addresses: %v", dials)
		}
	}
	if n := got.Load(); n != 0 {
		t.Errorf("the owner's local service was reached %d times through the proxy", n)
	}
}

// This computer's own services off its interfaces are refused by name, and no more than they: the addresses beside
// each one (a tailnet peer, a machine out in the world, an address outside the link-local network, the world under
// NAT64's prefix) stay reachable, with no place of the computer's own listed at all.
func TestTheComputersOwnServicesOffItsInterfacesAreRefusedAndNoMore(t *testing.T) {
	for _, to := range []string{
		"100.100.100.100:80", "[fd7a:115c:a1e0::53]:53", "169.254.169.254:80", "169.254.0.1:22", "169.254.255.255:22",
		"[fe80::1]:22", "[febf:ffff::1]:22", "[fd00:ec2::254]:80", "[fd00:ec2::123]:123", "100.100.100.200:80",
		"168.63.129.16:80", "[::ffff:a9fe:a9fe]:80", "[64:ff9b::a9fe:a9fe]:80", "[fd20:ce::254]:80", "[fd00:c1::a9fe:a9fe]:80",
	} {
		if why := ownAddress(netip.MustParseAddrPort(to), nil, 0); !strings.Contains(why, "own service off its interfaces") {
			t.Errorf("%s: %q, not this computer's own service off its interfaces", to, why)
		}
	}
	for _, to := range []string{
		"100.100.100.99:80", "100.100.100.101:80", "100.100.100.199:80", "100.101.102.104:443", "[fd7a:115c:a1e0::54]:53",
		"[fd7a:115c:a1e0::1235]:443", "169.253.255.255:80", "169.255.0.1:80", "[fec0::1]:22", "[fe7f:ffff::1]:22",
		"[fd00:ec3::254]:80", "168.63.129.17:80", "192.0.2.10:443", "[64:ff9b::c000:20a]:443", "[2001:db8::1]:443",
		"[fd20:ce::253]:80", "[fd20:ce:0:1::254]:80", "[fd00:c1::a9fe:a9ff]:80", "[fd00:c1:0:1::a9fe:a9fe]:80", // a VPC's own, beside them
	} {
		if why := ownAddress(netip.MustParseAddrPort(to), nil, 0); why != "" {
			t.Errorf("%s: refused (%s), but it isn't this computer's", to, why)
		}
	}
	// The app's port is on loopback itself only: under NAT64's prefix it is loopback, refused.
	if ownAddress(netip.MustParseAddrPort("127.0.0.1:7710"), nil, 7710) != "" ||
		ownAddress(netip.MustParseAddrPort("[64:ff9b::7f00:1]:7710"), nil, 7710) == "" {
		t.Error("the app's port: allowed on loopback only")
	}
}

// A desk that asks for refused places over and over can't fill the core's log: refusalLogMost lines a minute, then
// one line saying how many more there were.
func TestTheProxysRefusalsFillNoLog(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	p := &Proxy{own: ownAddrs, dial: (&dialLog{}).dialTo("127.0.0.1:1"), Log: func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(format, args...))
	}}
	socket := serveProxy(t, p)
	for i := range refusalLogMost + 10 {
		to := "169.254.169.254:" + strconv.Itoa(1+i)
		if status, _, _ := ask(t, socket, "CONNECT "+to+" HTTP/1.1\r\nHost: "+to+"\r\n\r\n"); status != http.StatusForbidden {
			t.Fatalf("%s: %d, not 403", to, status)
		}
	}
	mu.Lock()
	if len(lines) != refusalLogMost {
		t.Errorf("%d refusals said in a minute, not %d: %q", len(lines), refusalLogMost, lines)
	}
	mu.Unlock()
	p.saidMu.Lock()
	p.saidSince = p.saidSince.Add(-time.Minute) // the minute is over
	p.saidMu.Unlock()
	ask(t, socket, "CONNECT 127.0.0.1:1 HTTP/1.1\r\nHost: x\r\n\r\n")
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != refusalLogMost+2 || !strings.Contains(lines[refusalLogMost], "refused 10 more requests") ||
		!strings.Contains(lines[refusalLogMost+1], "127.0.0.1:1") {
		t.Errorf("after the minute: %q", lines[min(len(lines), refusalLogMost):])
	}
}

// Nothing smuggled through the forwarder reaches the owner's service on loopback (RT126SAL6's probe, kept): a
// CONNECT pipelined behind a plain request or carried in a body, a Host header or userinfo naming loopback, numeric
// names that resolve to it, an abstract name, the session bus as a unix: address, HTTP/1.0 and lower-case CONNECTs,
// OPTIONS * and an origin-form request. The stand-in dial sends every loopback address to the owner's service, so a
// check that let one through would move its count.
func TestNothingSmuggledThroughTheForwarderReachesTheOwnersService(t *testing.T) {
	ownerPort, owner := countingServer(t)
	op := strconv.Itoa(ownerPort)
	world, _ := countingServer(t)
	resolve := func(_ context.Context, host string) ([]netip.Addr, error) {
		switch host {
		case "example.test":
			return []netip.Addr{worldV4}, nil
		case "2130706433", "0x7f000001", "127.1": // what getaddrinfo makes of these
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		}
		return nil, errors.New("no such stand-in name")
	}
	dial := func(ctx context.Context, at netip.AddrPort) (net.Conn, error) {
		var d net.Dialer
		if at.Addr().Unmap().IsLoopback() {
			return d.DialContext(ctx, "tcp", "127.0.0.1:"+op)
		}
		return d.DialContext(ctx, "tcp", "127.0.0.1:"+strconv.Itoa(world))
	}
	via := forwarder(t, serveProxy(t, &Proxy{own: ownAddrs, resolve: resolve, dial: dial, Log: t.Logf}))
	for _, head := range []string{
		"GET http://example.test/ HTTP/1.1\r\nHost: example.test\r\n\r\nCONNECT 127.0.0.1:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"POST http://example.test/ HTTP/1.1\r\nHost: example.test\r\nContent-Length: 44\r\n\r\nCONNECT 127.0.0.1:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"POST http://example.test/ HTTP/1.1\r\nHost: example.test\r\nTransfer-Encoding: chunked\r\nContent-Length: 4\r\n\r\n0\r\n\r\nCONNECT 127.0.0.1:" + op + " HTTP/1.1\r\n\r\n",
		"GET http://example.test/ HTTP/1.1\r\nHost: 127.0.0.1:" + op + "\r\n\r\n",
		"GET http://example.test@127.0.0.1:" + op + "/ HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"CONNECT 2130706433:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT 0x7f000001:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT 127.1:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT @/tmp/.X11-unix/X0:1 HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT unix:/run/user/1000/bus HTTP/1.1\r\nHost: x\r\n\r\n",
		"GET http://unix:/run/user/1000/bus:/ HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT [::ffff:7f00:1]:" + op + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT 127.0.0.1:" + op + "\r\n\r\n",
		"CONNECT 127.0.0.1:" + op + " HTTP/1.0\r\n\r\n",
		"connect 127.0.0.1:" + op + " HTTP/1.1\r\n\r\n",
		"OPTIONS * HTTP/1.1\r\nHost: 127.0.0.1:" + op + "\r\n\r\n",
		"GET /x HTTP/1.1\r\nHost: 127.0.0.1:" + op + "\r\n\r\n",
	} {
		c, err := net.Dial("tcp", via)
		if err != nil {
			t.Fatal(err)
		}
		c.SetDeadline(time.Now().Add(5 * time.Second))
		io.WriteString(c, head)
		reply, _ := io.ReadAll(c)
		c.Close()
		if !strings.HasPrefix(string(reply), "HTTP/1.1 4") && !strings.HasPrefix(string(reply), "HTTP/1.1 5") {
			t.Errorf("%q: %.40q, not a refusal", head, reply)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if n := owner.Load(); n != 0 {
		t.Errorf("the owner's loopback service was reached %d times through the forwarder", n)
	}
}

// The app's own port, on loopback, is the one place there the desk reaches: its hooks and the GM's API, through
// CONNECT or a plain request.
func TestTheProxyReachesTheAppsPortOnLoopback(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "the app: "+r.URL.Path)
	}))
	t.Cleanup(app.Close)
	appURL, _ := url.Parse(app.URL)
	appPort, _ := strconv.Atoi(appURL.Port())
	socket := serveProxy(t, &Proxy{AppPort: appPort, own: ownAddrs})
	status, _, c := ask(t, socket, "GET http://127.0.0.1:"+appURL.Port()+"/hook HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
	if status != http.StatusOK {
		t.Fatalf("a plain request to the app's port: %d", status)
	}
	c.Close()
	status, in, c := ask(t, socket, "CONNECT 127.0.0.1:"+appURL.Port()+" HTTP/1.1\r\nHost: x\r\n\r\n")
	if status != http.StatusOK {
		t.Fatalf("CONNECT to the app's port: %d", status)
	}
	io.WriteString(c, "GET /gm HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n")
	resp, err := http.ReadResponse(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "the app: /gm" {
		t.Errorf("through the tunnel the app said %q", body)
	}
	// The same port on the computer's other addresses is still its own address.
	if status, _, _ := ask(t, socket, "CONNECT 192.0.2.99:"+appURL.Port()+" HTTP/1.1\r\nHost: x\r\n\r\n"); status != http.StatusForbidden {
		t.Errorf("the app's port on the computer's own LAN address: %d, not 403", status)
	}
}

// Out in the world: a name is looked up by the core, and the address checked is the address dialled. A plain request
// arrives as a server reads it (no proxy headers, the path alone), a tunnel carries bytes both ways, and the bytes a
// client sends right behind its CONNECT (a TLS hello) aren't lost. Through the forwarder, a real HTTP client with
// HTTP_PROXY's address reaches it as a desk's tools do.
func TestTheProxyCarriesRequestsAndTunnelsOut(t *testing.T) {
	var seen atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.RequestURI + " " + r.Header.Get("Proxy-Authorization") + "|" + r.Host)
		io.WriteString(w, "hello from the world")
	}))
	t.Cleanup(upstream.Close)
	upURL, _ := url.Parse(upstream.URL)
	echo := echoServer(t)
	resolve := func(_ context.Context, host string) ([]netip.Addr, error) {
		if host == "example.test" {
			return []netip.Addr{worldV4}, nil
		}
		return nil, errors.New("no such stand-in name")
	}
	web, tunnel := &dialLog{}, &dialLog{}
	socket := serveProxy(t, &Proxy{own: ownAddrs, resolve: resolve, dial: web.dialTo(upURL.Host)})
	tunnelSocket := serveProxy(t, &Proxy{own: ownAddrs, resolve: resolve, dial: tunnel.dialTo(echo)})

	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: forwarder(t, socket)})}}
	req, _ := http.NewRequest(http.MethodGet, "http://example.test/path?q=1", nil)
	req.Header.Set("Proxy-Authorization", "Basic ZmFrZQ==")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "hello from the world" {
		t.Errorf("through the forwarder: %d %q", resp.StatusCode, body)
	}
	if got := seen.Load(); got != "/path?q=1 |example.test" {
		t.Errorf("the server was sent %q", got)
	}
	if dials := web.all(); !slices.Equal(dials, []netip.AddrPort{netip.AddrPortFrom(worldV4, 80)}) {
		t.Errorf("dialled %v, not the address the name was checked as", dials)
	}

	status, in, c := ask(t, tunnelSocket, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\n\r\nhello-right-behind")
	if status != http.StatusOK {
		t.Fatalf("CONNECT: %d", status)
	}
	io.WriteString(c, "|and-after")
	c.(*net.UnixConn).CloseWrite()
	back, _ := io.ReadAll(in)
	if string(back) != "hello-right-behind|and-after" {
		t.Errorf("through the tunnel came back %q", back)
	}
	if dials := tunnel.all(); !slices.Equal(dials, []netip.AddrPort{netip.AddrPortFrom(worldV4, 443)}) {
		t.Errorf("the tunnel dialled %v", dials)
	}
}

// Nothing but TCP to a host and a port: a path (a session bus, any unix socket), a unix: or https: address in a plain
// request, a request with no address, and a head that isn't HTTP are refused, and nothing is dialled.
func TestTheProxyReachesNoUnixSocketNorAnythingButTCP(t *testing.T) {
	d := &dialLog{}
	bus := filepath.Join(shortTemp(t), "bus")
	busL, err := net.Listen("unix", bus)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { busL.Close() })
	reached := make(chan struct{}, 8)
	go func() {
		for {
			c, err := busL.Accept()
			if err != nil {
				return
			}
			reached <- struct{}{}
			c.Close()
		}
	}()
	socket := serveProxy(t, &Proxy{own: ownAddrs, dial: d.dialTo("127.0.0.1:1")})
	for _, head := range []string{
		"CONNECT " + bus + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"CONNECT unix:" + bus + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"GET unix://" + bus + " HTTP/1.1\r\nHost: x\r\n\r\n",
		"GET https://example.test/ HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"GET / HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"CONNECT example.test HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"CONNECT example.test:0 HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"CONNECT example.test:99999 HTTP/1.1\r\nHost: example.test\r\n\r\n",
		"not http at all\r\n\r\n",
	} {
		if status, _, _ := ask(t, socket, head); status != http.StatusBadRequest {
			t.Errorf("%q: %d, not 400", head, status)
		}
	}
	if dials := d.all(); len(dials) != 0 {
		t.Errorf("dialled %v", dials)
	}
	select {
	case <-reached:
		t.Error("the stand-in bus was reached through the proxy")
	default:
	}
}

// The proxy's socket: made new (something there is refused, and left as it is), the owner's alone, gone when it
// closes; in the desk's own folder, or its own socket folder when that is too long for a unix socket.
func TestTheProxySocketIsNewTheOwnersAndInTheDesksOwnPlace(t *testing.T) {
	dir := shortTemp(t)
	at := filepath.Join(dir, ProxySocketName)
	if err := os.WriteFile(at, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := ListenProxy(at); err == nil {
		l.Close()
		t.Fatal("a proxy socket was made over a file")
	}
	if data, _ := os.ReadFile(at); string(data) != "not a socket" {
		t.Errorf("the file there was changed: %q", data)
	}
	os.Remove(at)
	l, err := ListenProxy(at)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(at); err != nil || info.Mode().Type() != os.ModeSocket || info.Mode().Perm() != 0o600 {
		t.Errorf("the socket: %v %v", info.Mode(), err)
	}
	l.Close()
	if _, err := os.Lstat(at); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the socket outlived its listener: %v", err)
	}
	if _, err := ListenProxy("proxy.sock"); !errors.Is(err, ErrProxyPath) {
		t.Errorf("a socket not by a whole path: %v", err)
	}

	short, err := ProxySocket("/home/ada/.config/smolcorp", "h1", "/tmp/tmux-1000/smolcorp-h1.d")
	if err != nil || short != "/home/ada/.config/smolcorp/hires/h1/proxy.sock" {
		t.Errorf("a short support folder: %q %v", short, err)
	}
	long := "/home/" + strings.Repeat("a", 80) + "/.config/smolcorp"
	if got, err := ProxySocket(long, "h1", "/tmp/tmux-1000/smolcorp-h1.d"); err != nil || got != "/tmp/tmux-1000/smolcorp-h1.d/proxy.sock" {
		t.Errorf("a long support folder: %q %v", got, err)
	}
	for _, c := range [][3]string{{long, "h1", "/tmp/" + strings.Repeat("b", 100)}, {"/s", "../h2", "/t"}, {"/s", "", ""}, {"s", "h1", ""}} {
		if got, err := ProxySocket(c[0], c[1], c[2]); !errors.Is(err, ErrProxyPath) {
			t.Errorf("%q: %q %v, not ErrProxyPath", c, got, err)
		}
	}
}

// A Linux desk may connect to its own proxy socket, by its own row, and not to another desk's; a Mac desk's Policy
// has no such row (its network is its own).
func TestTheLinuxPolicyOpensTheDesksOwnProxySocketAlone(t *testing.T) {
	pl := linuxPlaces(lSupport)
	p := mustNewQuiet(pl)
	own := lSupport + "/hires/h1/" + ProxySocketName
	if v, row := p.Decide(own, Connect); v != Allow || row.Row != "its way out: the core's proxy" {
		t.Errorf("its own proxy socket: %v by %q", v, row.Row)
	}
	for _, at := range []string{lSupport + "/hires/h2/" + ProxySocketName, lSupport + "/hires/gm1/" + ProxySocketName, lSupport + "/" + ProxySocketName} {
		if v, _ := p.Decide(at, Connect); v != Deny {
			t.Errorf("%s: %v", at, v)
		}
	}
	for _, op := range []Op{Read, Write} {
		if v, _ := p.Decide(own, op); v != Deny {
			t.Errorf("its proxy socket, %v: %v (connect only)", op, v)
		}
	}
	mac := pl
	mac.GOOS = "darwin"
	for _, r := range mustNewQuiet(mac).Rules() {
		if strings.Contains(r.Row, "way out") {
			t.Errorf("a Mac desk's Policy has %q", r.Row)
		}
	}
}

// The desk's environment points every proxy variable at the forwarder, and nothing of the owner's own proxy is left:
// it would be on the view's loopback, where nothing listens. git's ssh is Smolcorp's (SA-L7), and the owner's own way
// of running it for git is taken out with it.
func TestTheDesksEnvironmentPointsAtTheForwarder(t *testing.T) {
	env := ProxyEnv([]string{"PATH=/usr/bin", "HTTP_PROXY=http://127.0.0.1:3128", "all_proxy=socks5://127.0.0.1:1080",
		"ALL_PROXY=socks5h://127.0.0.1:1080", "no_proxy=*", "NODE_USE_ENV_PROXY=0", "GIT_SSH_COMMAND=ssh -i ~/.ssh/work",
		"GIT_SSH_VARIANT=plink", "HOME=/home/ada"}, 47128, "/usr/bin/smolcorp")
	want := []string{"PATH=/usr/bin", "HOME=/home/ada", "HTTP_PROXY=http://127.0.0.1:47128", "HTTPS_PROXY=http://127.0.0.1:47128",
		"http_proxy=http://127.0.0.1:47128", "https_proxy=http://127.0.0.1:47128", "NO_PROXY=localhost,127.0.0.1,::1",
		"no_proxy=localhost,127.0.0.1,::1", "NODE_USE_ENV_PROXY=1",
		"GIT_SSH_COMMAND=ssh -o 'ProxyCommand='\\''/usr/bin/smolcorp'\\'' sandbox connect %h %p' -o ControlPath=none"}
	if !slices.Equal(env, want) {
		t.Errorf("the desk's environment:\n got %q\nwant %q", env, want)
	}
}

// The proxy row's check holds for the core's proxy through the forwarder, and fails for a way out that lets the
// computer's own address through (an open proxy), one where nothing answers, and one that answers something else.
func TestTheWayOutCheckFailsUnlessItIsTheCoresProxy(t *testing.T) {
	if err := wayOutHolds(forwarder(t, serveProxy(t, &Proxy{own: ownAddrs}))); err != nil {
		t.Errorf("the core's proxy: %v", err)
	}
	open, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { open.Close() })
	go func() {
		for {
			c, err := open.Accept()
			if err != nil {
				return
			}
			go func() {
				bufio.NewReader(c).ReadString('\n')
				io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")
				c.Close()
			}()
		}
	}()
	if err := wayOutHolds(open.Addr().String()); err == nil || !strings.Contains(err.Error(), "not 403") {
		t.Errorf("an open proxy: %v", err)
	}
	gone, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := gone.Addr().String()
	gone.Close()
	if err := wayOutHolds(addr); err == nil {
		t.Error("nothing listening passed")
	}
	// A forwarder to a socket nobody listens at: the client gets nothing back.
	if err := wayOutHolds(forwarder(t, filepath.Join(shortTemp(t), "none.sock"))); err == nil {
		t.Error("a forwarder to no proxy passed")
	}
}

// The proxy row in `enter`: once at most, by a whole path, a Linux start's choice; it fails when the forwarder doesn't
// start or the way out isn't the core's, passes through the real proxy, and the core takes the report.
func TestTheProxyRow(t *testing.T) {
	d := newStandInDesk(t)
	socket := serveProxy(t, &Proxy{own: ownAddrs})
	checks := d.checks
	checks.Proxy = socket
	s := mustStart(t, checks)
	words := s.Words("/bin/echo")[2:]
	if !slices.Contains(words, "--proxy") {
		t.Fatalf("the words have no proxy row: %q", words)
	}
	e, err := parseEnter(words)
	if err != nil {
		t.Fatal(err)
	}
	if kinds := len(e.rows); e.rows[kinds-2] != (checkRow{rowProxy, socket}) || e.rows[kinds-1].kind != rowFDs {
		t.Fatalf("the rows: %q", e.rows)
	}
	sys := holding("linux", s, d)
	var asked []string
	sys.forwardFn = func(at, addr, app string) (string, error) {
		asked = append(asked, at, addr, app)
		return forwarder(t, at), nil
	}
	results, held := runRows("linux", sys, e)
	if !held || !slices.Equal(asked, []string{socket, "127.0.0.1:" + strconv.Itoa(ForwardPort), ""}) {
		t.Fatalf("through the core's proxy: held %v, %q, forwarder asked %q", held, results, asked)
	}
	give(t, s, report(s.nonce, e.rows, results))
	if err := s.Wait(5 * time.Second); err != nil {
		t.Fatalf("the core refused a passing report with a proxy row: %v", err)
	}

	open := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(open.Close)
	for _, c := range []struct {
		name    string
		forward func(string, string, string) (string, error)
	}{
		{"no forwarder here", nil},
		{"the forwarder didn't listen", func(string, string, string) (string, error) { return "", syscall.EADDRINUSE }},
		{"an open way out", func(string, string, string) (string, error) { return strings.TrimPrefix(open.URL, "http://"), nil }},
	} {
		s := mustStart(t, checks)
		e, _ := parseEnter(s.Words("/bin/echo")[2:])
		sys := holding("linux", s, d)
		sys.forwardFn = c.forward
		results, held := runRows("linux", sys, e)
		if held || results[len(results)-2] == passed {
			t.Errorf("%s: held %v, %q", c.name, held, results)
		}
		give(t, s, report(s.nonce, e.rows, results))
		var refused *Refused
		if err := s.Wait(5 * time.Second); !errors.As(err, &refused) {
			t.Errorf("%s: the core took it: %v", c.name, err)
		}
	}

	// Its words: by a whole path, once at most, after the canary; a start without it has none.
	plain := mustStart(t, d.checks)
	if slices.Contains(plain.Words("/bin/echo"), "--proxy") {
		t.Error("a start with no proxy has a proxy row")
	}
	base := plain.Words("/bin/echo")[2:]
	dash := slices.Index(base, "--")
	for _, extra := range [][]string{{"--proxy", "proxy.sock"}, {"--proxy", socket, "--proxy", socket}} {
		words := slices.Concat(base[:dash], extra, base[dash:])
		if _, err := parseEnter(words); err == nil {
			t.Errorf("%q was taken", extra)
		}
	}
	if _, err := parseEnter(slices.Concat(base[:dash], []string{"--proxy", socket}, base[dash:])); err != nil {
		t.Errorf("one proxy row by a whole path: %v", err)
	}
}

// The computer's own places from its interfaces: each address, exactly, and a container's or a VM's bridge whole;
// a LAN's network isn't.
func TestTheComputersOwnPlacesCoverItsBridgesWhole(t *testing.T) {
	for name, want := range map[string]bool{"docker0": true, "br-1a2b3c": true, "veth9f": true, "podman0": true, "cni0": true,
		"virbr0": true, "lxdbr0": true, "vboxnet0": true, "incusbr0": true, "mpqemubr0": true, "nerdctl0": true,
		"cilium_host": true, "kube-bridge": true, "eth0": false, "wlan0": false, "enp3s0": false, "tailscale0": false, "lo": false} {
		if bridgeInterface(name) != want {
			t.Errorf("%s: a bridge %v", name, !want)
		}
	}
	own, err := (&Proxy{}).ownPlaces()
	if err != nil {
		t.Fatal(err)
	}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, n := range addrs {
			ipNet, ok := n.(*net.IPNet)
			if !ok {
				continue
			}
			a, _ := netip.AddrFromSlice(ipNet.IP)
			if why := ownAddress(netip.AddrPortFrom(a.Unmap(), 443), own, 0); why == "" {
				t.Errorf("%s's address %v isn't the computer's own", iface.Name, a)
			}
		}
	}
	lan := netip.MustParsePrefix("192.0.2.0/24")
	if ownAddress(netip.AddrPortFrom(worldV4, 443), []netip.Prefix{netip.PrefixFrom(ownV4, 32)}, 0) != "" || !lan.Contains(worldV4) {
		t.Error("another machine on the computer's own network is refused")
	}
}
