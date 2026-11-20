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

// Linux's judge (OLD-SERVE1), with stand-in peers: only one that connected as root and is still there, running as root
// (its real and its effective user), is tailscaled. Who connected is what exec can't change (RT126SN1 B1): a program
// of the owner's that execs a setuid-root program keeps the user it connected as. A client that sent and ended before
// the accept is no process now (RSBXPLANf F2). Any process running as root passes: Linux has no signature to check.
func TestOnlyWhatConnectedAndRunsAsRootIsTailscaledOnLinux(t *testing.T) {
	saved := tailscaleUID
	t.Cleanup(func() { tailscaleUID = saved })
	tailscaleUID = 0
	const owner, other = 1000, 998
	asRoot := procUsers{realUID: 0, effectiveUID: 0, running: true}
	for _, c := range []struct {
		name string
		peer credPeer
		want bool
	}{
		{"tailscaled, as root", credPeer{0, 812, asRoot}, true},
		{"any other process running as root (the rule can't tell)", credPeer{0, 4242, asRoot}, true},
		{"connected as the owner, now root (exec into a setuid-root program)", credPeer{owner, 4242, asRoot}, false},
		{"connected as the owner, and runs as the owner (a program here)", credPeer{owner, 4242, procUsers{owner, owner, true}}, false},
		{"a tailscaled run as its own user", credPeer{other, 812, procUsers{other, other, true}}, false},
		{"root at connect, its real user the owner now", credPeer{0, 4242, procUsers{owner, 0, true}}, false},
		{"root at connect, its effective user the owner now (dropped)", credPeer{0, 4242, procUsers{0, owner, true}}, false},
		{"root at connect, sent and ended before the accept (gone, or /proc unreadable)", credPeer{0, 4242, procUsers{}}, false},
		{"root at connect, a zombie now", credPeer{0, 4242, procUsers{0, 0, false}}, false},
		{"no pid (another pid namespace)", credPeer{0, 0, asRoot}, false},
		{"a pid that can't be one", credPeer{0, -1, asRoot}, false},
	} {
		if got := judgeCredPeer(c.peer); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// /proc/<pid>/status is read for its State and Uid lines alone, as Linux writes them: anything missing, said twice or
// otherwise is no process.
func TestProcStatusIsReadAsLinuxWritesIt(t *testing.T) {
	status := func(name, state, uid string) string {
		return "Name:\t" + name + "\nUmask:\t0022\nState:\t" + state + "\nTgid:\t812\nPid:\t812\nPPid:\t1\nUid:\t" + uid + "\nGid:\t0\t0\t0\t0\n"
	}
	for _, c := range []struct {
		name, text string
		want       procUsers
		ok         bool
	}{
		{"tailscaled, sleeping", status("tailscaled", "S (sleeping)", "0\t0\t0\t0"), procUsers{0, 0, true}, true},
		{"running", status("tailscaled", "R (running)", "0\t0\t0\t0"), procUsers{0, 0, true}, true},
		{"the owner's", status("bash", "S (sleeping)", "1000\t1000\t1000\t1000"), procUsers{1000, 1000, true}, true},
		{"setuid root (real the owner)", status("su", "S (sleeping)", "1000\t0\t0\t0"), procUsers{1000, 0, true}, true},
		{"a zombie", status("x", "Z (zombie)", "0\t0\t0\t0"), procUsers{0, 0, false}, true},
		{"dead", status("x", "X (dead)", "0\t0\t0\t0"), procUsers{0, 0, false}, true},
		{"no State", "Name:\tx\nUid:\t0\t0\t0\t0\n", procUsers{}, false},
		{"no Uid", "Name:\tx\nState:\tS (sleeping)\n", procUsers{}, false},
		{"Uid of three", status("x", "S (sleeping)", "0\t0\t0"), procUsers{}, false},
		{"Uid not a number", status("x", "S (sleeping)", "root\t0\t0\t0"), procUsers{}, false},
		{"Uid too big", status("x", "S (sleeping)", "4294967296\t0\t0\t0"), procUsers{}, false},
		{"State empty", status("x", "", "0\t0\t0\t0"), procUsers{}, false},
		{"a name that says Uid again", status("x\nUid:\t0\t0\t0\t0", "S (sleeping)", "1000\t1000\t1000\t1000"), procUsers{}, false},
		{"a name that says State again", status("x\nState:\tR (running)", "Z (zombie)", "0\t0\t0\t0"), procUsers{}, false},
		{"empty", "", procUsers{}, false},
	} {
		got, ok := procUsersOf(c.text)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %+v %v, want %+v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// On Windows (OLD-SERVE2) only the Tailscale service's own process is Tailscale's: named by the service manager,
// running tailscaled.exe by its configuration and by the process list, started before the accept, the same process
// after the TCP table was read, and holding the client's end of this connection, established. Anything else, a
// program the owner runs among them, is refused, as is a client that has half-closed or ended.
func TestOnlyTheTailscaleServiceIsTailscalesOnWindows(t *testing.T) {
	const accepted = int64(2_000_000_000_000_000_000)
	service := serviceProcess{pid: 4120, binary: `"C:\Program Files\Tailscale\tailscaled.exe"`, image: "tailscaled.exe", created: accepted - 3_600_000_000_000}
	peer := windowsPeer{accepted: accepted, before: service, after: service, row: tcpRow{found: true, state: tcpEstablished, pid: 4120}}
	if !judgeWindowsPeer(peer) {
		t.Fatal("the Tailscale service's own connection, established, wasn't judged Tailscale's: every row below proves nothing")
	}
	changed := func(change func(*windowsPeer)) windowsPeer {
		p := peer
		change(&p)
		return p
	}
	for _, c := range []struct {
		name string
		peer windowsPeer
		want bool
	}{
		{"its program unquoted, as an older installer wrote it", changed(func(p *windowsPeer) {
			p.before.binary, p.after.binary = `C:\Tailscale\tailscaled.exe`, `C:\Tailscale\tailscaled.exe`
		}), true},
		{"its program quoted, with arguments", changed(func(p *windowsPeer) {
			p.before.binary = `"C:\Program Files\Tailscale\tailscaled.exe" -port 41641`
			p.after.binary = p.before.binary
		}), true},
		{"names in another case", changed(func(p *windowsPeer) {
			p.before.image, p.after.image = "TailscaleD.EXE", "TailscaleD.EXE"
		}), true},
		{"a program here holds the row", changed(func(p *windowsPeer) { p.row.pid = 7000 }), false},
		{"no Tailscale service", changed(func(p *windowsPeer) { p.before, p.after = serviceProcess{}, serviceProcess{} }), false},
		{"the service gone after the table was read", changed(func(p *windowsPeer) { p.after = serviceProcess{} }), false},
		{"its pid given again, to a process started later", changed(func(p *windowsPeer) { p.after.created = accepted + 1 }), false},
		{"another service process after the table was read", changed(func(p *windowsPeer) { p.after.pid = 7000 }), false},
		{"started after the accept", changed(func(p *windowsPeer) { p.before.created, p.after.created = accepted+1, accepted+1 }), false},
		{"started at the accept", changed(func(p *windowsPeer) { p.before.created, p.after.created = accepted, accepted }), false},
		{"no start time", changed(func(p *windowsPeer) { p.before.created, p.after.created = 0, 0 }), false},
		{"another program by the process list", changed(func(p *windowsPeer) { p.before.image, p.after.image = "svchost.exe", "svchost.exe" }), false},
		{"not in the process list", changed(func(p *windowsPeer) { p.before.image, p.after.image = "", "" }), false},
		{"another program by its configuration", changed(func(p *windowsPeer) {
			p.before.binary, p.after.binary = `C:\Windows\System32\svchost.exe -k netsvcs`, `C:\Windows\System32\svchost.exe -k netsvcs`
		}), false},
		{"a relative program", changed(func(p *windowsPeer) { p.before.binary, p.after.binary = "tailscaled.exe", "tailscaled.exe" }), false},
		{"a program in a folder named tailscaled.exe", changed(func(p *windowsPeer) {
			p.before.binary, p.after.binary = `"C:\tailscaled.exe\other.exe"`, `"C:\tailscaled.exe\other.exe"`
		}), false},
		{"no row for the connection (its client ended)", changed(func(p *windowsPeer) { p.row = tcpRow{} }), false},
		{"half-closed (FIN_WAIT_2)", changed(func(p *windowsPeer) { p.row.state = 7 }), false},
		{"closing (FIN_WAIT_1)", changed(func(p *windowsPeer) { p.row.state = 6 }), false},
		{"ended (TIME_WAIT, no process)", changed(func(p *windowsPeer) { p.row = tcpRow{found: true, state: 11} }), false},
		{"no pid at all", changed(func(p *windowsPeer) {
			p.before.pid, p.after.pid, p.row.pid = 0, 0, 0
		}), false},
	} {
		if got := judgeWindowsPeer(c.peer); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// A service's configured command line names its program as Windows writes one: quoted, or up to its first space, on
// a drive.
func TestAServicesProgramIsReadAsWindowsWritesIt(t *testing.T) {
	for line, want := range map[string]string{
		`"C:\Program Files\Tailscale\tailscaled.exe"`:          "tailscaled.exe",
		`"C:\Program Files\Tailscale\tailscaled.exe" -port 41`: "tailscaled.exe",
		`C:\Tailscale\tailscaled.exe`:                          "tailscaled.exe",
		`C:\Program Files\Tailscale\tailscaled.exe`:            "Program",
		`c:\x\y.exe -k`: "y.exe",
		`"C:\Program Files\Tailscale\tailscaled.exe`: "",
		`tailscaled.exe`:                "",
		`\\server\share\tailscaled.exe`: "",
		`"C:\folder\"`:                  "",
		`""`:                            "",
		``:                              "",
		`/usr/sbin/tailscaled`:          "",
	} {
		if got := windowsProgramName(line); got != want {
			t.Errorf("%s: %q, want %q", line, got, want)
		}
	}
}
