package sandbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The network through the core, I (SA-L6, SANDBOX-PLAN §A3.2 "The network through the core", N4, AQ15): a Linux desk's
// view has a network namespace of its own (--unshare-net, bwrap.go), whose only network is its own loopback. That
// keeps every port of the computer's (`ssh localhost`, Docker's TCP, a debug port, a database) and every abstract unix
// socket (X11's, an old D-Bus address) out of reach. The desk reaches out through the core instead:
//   - the core listens, for each desk, as an HTTP(S) proxy (Proxy) on a unix socket in the desk's own folder
//     (ProxySocket), which its view brings back, connect only, by its own Policy row ("its way out"), so the socket
//     mask (mask.go) lists it. No other desk's view has it: the support folder is covered, and so is /tmp;
//   - inside the view, a small forwarder (forward_linux.go) listens on 127.0.0.1:ForwardPort, the view's own loopback,
//     and hands each connection on, byte for byte, to that socket (Forward). `enter` starts it, checks that the way out
//     answers and refuses the computer's own address (the proxy row), and sets HTTP(S)_PROXY and NO_PROXY for the
//     desk (ProxyEnv);
//   - the proxy takes CONNECT (HTTPS, and anything else tunnelled) and plain http:// requests, looks the host up
//     itself, and dials only TCP, only to an address that isn't the computer's own (ownAddress: loopback, any address
//     of the computer's interfaces, its tailnet address among them, the unspecified and multicast ones, every
//     address on a container's or a VM's bridge, where a local stack's database answers, and its own services at
//     addresses no interface holds: link-local, a cloud VM's metadata service, Tailscale's 100.100.100.100), save
//     the app's own port on loopback (Proxy.AppPort, for hooks and the GM's API). It never dials a unix socket, so
//     neither a session bus nor any other local service is reached through it. The address checked is the address
//     dialled: a name that resolves to a refused address is refused, however often its answer changes.
//
// Left to SA-L7: SSH through CONNECT (`smolcorp sandbox connect`, GIT_SSH_COMMAND) and the app's port forwarded inside
// on its own number. The core's side of each desk's start (the proxy listening before the view is made, closed when the
// desk ends) is the start wiring's, SA-L4.

// ProxySocketName is the proxy's socket in the desk's own folder.
const ProxySocketName = "proxy.sock"

// ForwardPort is where the forwarder listens in the view's own network namespace: a port no common tool takes, so a
// desk's own servers keep theirs.
const ForwardPort = 47128

// maxUnixPath is the longest path a unix socket is bound to on every system the core runs on (a Mac's sun_path is the
// shortest, 104 bytes with its end).
const maxUnixPath = 103

// ErrProxyPath is a desk's proxy socket that can't be bound: its path is too long for a unix socket, or not whole.
var ErrProxyPath = errors.New("sandbox: the desk's proxy socket can't be bound at that path")

// ProxySocket is where the core listens as the proxy for the desk id: proxy.sock in its own folder in support, or, when
// that is too long for a unix socket, in its own socket folder ownSockets (which its view brings back too, and the mask
// lists). Neither fitting is ErrProxyPath.
func ProxySocket(support, id, ownSockets string) (string, error) {
	for _, at := range []string{filepath.Join(support, "hires", id, ProxySocketName), filepath.Join(ownSockets, ProxySocketName)} {
		if filepath.IsAbs(at) && len(at) <= maxUnixPath && id != "" && !strings.ContainsAny(id, `/\`) {
			return at, nil
		}
	}
	return "", ErrProxyPath
}

// ListenProxy makes the proxy's socket at, new: something already there is refused, not replaced. The socket is the
// owner's alone (0600), and closing the listener removes it.
func ListenProxy(at string) (net.Listener, error) {
	if !filepath.IsAbs(at) || len(at) > maxUnixPath {
		return nil, fmt.Errorf("%w: %s", ErrProxyPath, at)
	}
	if _, err := os.Lstat(at); !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("sandbox: the proxy socket %s can't be made: something is there (%v)", at, err)
	}
	l, err := net.Listen("unix", at)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(at, 0o600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

// Proxy is one desk's way out (above). The zero value refuses the app's port too; its stand-ins are for the tests.
type Proxy struct {
	// AppPort is the app's own port, the one place on loopback the desk may reach; 0 is none.
	AppPort int
	// Log is where each refusal is said; nil is the core's log.
	Log func(format string, args ...any)

	// resolve looks a host up; own lists the computer's own places (ownPlaces); dial makes a TCP connection.
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
	own     func() ([]netip.Prefix, error)
	dial    func(ctx context.Context, to netip.AddrPort) (net.Conn, error)

	// said counts the refusals said in the core's log since saidSince, and unsaid the ones over refusalLogMost.
	saidMu    sync.Mutex
	saidSince time.Time
	said      int
	unsaid    int
}

// refusalLogMost is the most refusals one desk's proxy says in the core's log in a minute. The rest are counted, and
// said as one line at the next refusal after that minute, so a desk can't fill the core's log with them.
const refusalLogMost = 20

// proxyMost is the most connections one desk's proxy holds at once: more are refused, so a desk can't use up the
// core's descriptors.
const proxyMost = 256

// proxyHeadMost is the most of a request's head the proxy reads, and proxyHeadWait how long it waits for it.
const (
	proxyHeadMost = 64 << 10
	proxyHeadWait = 30 * time.Second
	proxyDialWait = 15 * time.Second
)

// Serve answers each connection l accepts until l is closed: then it gives the listener's error.
func (p *Proxy) Serve(l net.Listener) error {
	held := make(chan struct{}, proxyMost)
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		select {
		case held <- struct{}{}:
			go func() {
				defer func() { <-held }()
				p.handle(conn)
			}()
		default:
			p.answer(conn, http.StatusServiceUnavailable, "Smolcorp's proxy for this hire has too many connections open.")
			conn.Close()
		}
	}
}

// handle reads one request's head from conn and answers it: a tunnel for CONNECT, one request and its response for a
// plain http:// one. Anything else is refused, in words.
func (p *Proxy) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(proxyHeadWait))
	head := &io.LimitedReader{R: conn, N: proxyHeadMost}
	in := bufio.NewReader(head)
	req, err := http.ReadRequest(in)
	if err != nil {
		p.answer(conn, http.StatusBadRequest, "Smolcorp's proxy couldn't read that request.")
		return
	}
	conn.SetReadDeadline(time.Time{})
	head.N = 1 << 62         // the head is read: what follows is the request's body, or the tunnel's bytes
	hostPort := req.URL.Host // CONNECT's authority; a CONNECT to a path has none, and is refused below
	if req.Method != http.MethodConnect {
		if req.URL.Scheme != "http" || req.URL.Host == "" {
			p.answer(conn, http.StatusBadRequest, "Smolcorp's proxy takes CONNECT, or a request for an http:// address.")
			return
		}
		hostPort = req.URL.Host
		if req.URL.Port() == "" {
			hostPort = net.JoinHostPort(req.URL.Hostname(), "80")
		}
	}
	host, port, ok := splitTarget(hostPort)
	if !ok {
		p.answer(conn, http.StatusBadRequest, "Smolcorp's proxy reaches a host and a port over TCP, nothing else.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), proxyDialWait)
	far, err := p.reach(ctx, host, port)
	cancel()
	var refused *refusedAddress
	switch {
	case errors.As(err, &refused):
		p.logRefusal(hostPort, err)
		p.answer(conn, http.StatusForbidden, "Smolcorp's sandbox: a hire can't reach this computer's own addresses through its proxy, only the app's own port ("+err.Error()+").")
		return
	case err != nil:
		p.answer(conn, http.StatusBadGateway, "Smolcorp's proxy couldn't reach "+hostPort+": "+err.Error())
		return
	}
	defer far.Close()
	if req.Method == http.MethodConnect {
		if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
			return
		}
		splice(conn, in, far) // in first: bytes the client sent after its head, a TLS hello, are already there
		return
	}
	// A plain request goes on in the form a server reads, without the proxy's own headers, and is the only one on
	// this connection: its response, then the end.
	for _, h := range []string{"Proxy-Connection", "Proxy-Authorization", "Connection", "Keep-Alive", "Te", "Trailer", "Upgrade"} {
		req.Header.Del(h)
	}
	req.RequestURI, req.Close = "", true
	if err := req.Write(far); err != nil {
		p.answer(conn, http.StatusBadGateway, "Smolcorp's proxy couldn't send the request: "+err.Error())
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(far), req)
	if err != nil {
		p.answer(conn, http.StatusBadGateway, "Smolcorp's proxy got no response: "+err.Error())
		return
	}
	defer resp.Body.Close()
	resp.Close = true
	resp.Write(conn)
}

// answer writes a short response with words, and says the connection ends.
func (p *Proxy) answer(conn net.Conn, status int, words string) {
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s\n",
		status, http.StatusText(status), len(words)+1, words)
}

// logRefusal says a refusal in the core's log, refusalLogMost a minute at most (above).
func (p *Proxy) logRefusal(hostPort string, err error) {
	p.saidMu.Lock()
	now := time.Now()
	if now.Sub(p.saidSince) >= time.Minute {
		if p.unsaid > 0 {
			p.logf("sandbox: a hire's proxy refused %d more requests in the last minute, not each said", p.unsaid)
		}
		p.saidSince, p.said, p.unsaid = now, 0, 0
	}
	sayIt := p.said < refusalLogMost
	if sayIt {
		p.said++
	} else {
		p.unsaid++
	}
	p.saidMu.Unlock()
	if sayIt {
		p.logf("sandbox: a hire's proxy refused %s: %v", hostPort, err)
	}
}

func (p *Proxy) logf(format string, args ...any) {
	if p.Log != nil {
		p.Log(format, args...)
		return
	}
	log.Printf(format, args...)
}

// refusedAddress is a place the desk may not reach: the computer's own address, or one that isn't anywhere.
type refusedAddress struct{ to, why string }

func (e *refusedAddress) Error() string { return e.to + " is " + e.why }

// splitTarget is a request's host and port: a name or an address, and a port from 1 to 65535. A path, a unix: address
// or anything else isn't one.
func splitTarget(hostPort string) (string, uint16, bool) {
	host, portWord, err := net.SplitHostPort(hostPort)
	port, perr := strconv.ParseUint(portWord, 10, 16)
	if err != nil || perr != nil || port == 0 || host == "" || strings.ContainsAny(host, "/ ") {
		return "", 0, false
	}
	return host, uint16(port), true
}

// reach looks host up and dials, over TCP, the first of its addresses the desk may reach that answers. A host with
// none it may reach is a *refusedAddress.
func (p *Proxy) reach(ctx context.Context, host string, port uint16) (net.Conn, error) {
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{a}
	} else if addrs, err = p.lookup(ctx, host); err != nil {
		return nil, err
	}
	own, err := p.ownPlaces()
	if err != nil {
		return nil, fmt.Errorf("this computer's own addresses can't be listed: %w", err)
	}
	var refused, failed error
	for _, a := range addrs {
		to := netip.AddrPortFrom(a, port)
		if why := ownAddress(to, own, p.AppPort); why != "" {
			refused = &refusedAddress{to.String(), why}
			continue
		}
		conn, err := p.dialAt(ctx, to)
		if err == nil {
			return conn, nil
		}
		failed = err
	}
	if failed != nil {
		return nil, failed
	}
	if refused == nil {
		refused = &refusedAddress{host, "a name with no address"}
	}
	return nil, refused
}

// offInterface is one of this computer's own places that no interface of its holds (offInterfaces).
type offInterface struct {
	at  netip.Prefix
	why string
}

// offInterfaces are this computer's own services that sit at an address none of its interfaces holds, so ownPlaces
// can't find them, and that only this computer reaches (RT126SAL6 B1). They are refused whatever the port, on every
// computer, whether the service is there or not:
//   - the link-local networks, host-local by nature: a cloud VM's metadata service (169.254.169.254 on every cloud,
//     which hands out the machine's credentials), its DNS and time services, and a link peer's address that only
//     this computer can reach;
//   - Tailscale's own address on each device ("quad100"), where the local tailscaled answers: its DNS, and the
//     device's own Tailscale web interface;
//   - the cloud services at a fixed address of their own outside the link-local networks: AWS's on IPv6 (the
//     metadata service at fd00:ec2::254, its DNS and time service beside it), Alibaba Cloud's metadata service, and
//     Azure's platform address, which only that VM reaches.
var offInterfaces = []offInterface{
	{netip.MustParsePrefix("169.254.0.0/16"), "link-local: a cloud VM's metadata service, or a link peer only this computer reaches"},
	{netip.MustParsePrefix("fe80::/10"), "link-local: a link peer only this computer reaches"},
	{netip.MustParsePrefix("100.100.100.100/32"), "Tailscale's own address on this computer, answered by its tailscaled"},
	{netip.MustParsePrefix("fd7a:115c:a1e0::53/128"), "Tailscale's own address on this computer, answered by its tailscaled"},
	{netip.MustParsePrefix("fd00:ec2::/32"), "a cloud VM's metadata service (AWS's, on IPv6)"},
	{netip.MustParsePrefix("100.100.100.200/32"), "a cloud VM's metadata service (Alibaba Cloud's)"},
	{netip.MustParsePrefix("168.63.129.16/32"), "a cloud VM's platform address (Azure's)"},
}

// nat64 is the well-known prefix (RFC 6052) under which an IPv6 address carries an IPv4 one for a NAT64 to reach: on
// a computer that runs its own NAT64, 64:ff9b::a9fe:a9fe is 169.254.169.254.
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// ownAddress is why the desk may not reach to, or "" when it may: loopback, but for the app's port; the unspecified
// address and 0.0.0.0/8, which reach this computer; multicast and the broadcast address; this computer's own services
// off its interfaces (offInterfaces); and every place in own (ownPlaces), whatever the port. An IPv4 address written
// as IPv6, or carried under NAT64's prefix, is the IPv4 one.
func ownAddress(to netip.AddrPort, own []netip.Prefix, appPort int) string {
	a := to.Addr().Unmap().WithZone("")
	if nat64.Contains(a) {
		a = netip.AddrFrom4([4]byte(a.AsSlice()[12:]))
	}
	switch {
	case a.IsLoopback() && appPort != 0 && int(to.Port()) == appPort && to.Addr().Unmap().WithZone("") == a:
		return "" // the app's port on loopback itself, not through a NAT64
	case a.IsLoopback():
		return "this computer's loopback"
	case a.IsUnspecified() || a.Is4() && a.As4()[0] == 0:
		return "this computer (an unspecified address)"
	case a.IsMulticast() || a == netip.AddrFrom4([4]byte{255, 255, 255, 255}):
		return "a multicast or broadcast address"
	}
	for _, o := range offInterfaces {
		if o.at.Contains(a) {
			return "this computer's own service off its interfaces (" + o.why + ")"
		}
	}
	if slices.ContainsFunc(own, func(p netip.Prefix) bool { return p.Contains(a) }) {
		return "this computer's own address, or a container's or a VM's on it"
	}
	return ""
}

func (p *Proxy) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	if p.resolve != nil {
		return p.resolve(ctx, host)
	}
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// ownPlaces are the computer's own places, asked at each request, so an interface that comes up (a VPN, a container's
// bridge) counts at once: each address of each of its interfaces, and the whole network of a container's or a VM's
// bridge (bridgeInterface), whose containers are the owner's local services too (Docker's, Podman's, a local
// Supabase's database). The other networks it is on, the LAN's machines among them, aren't its own.
func (p *Proxy) ownPlaces() ([]netip.Prefix, error) {
	if p.own != nil {
		return p.own()
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var own []netip.Prefix
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, n := range addrs {
			ipNet, ok := n.(*net.IPNet)
			if !ok {
				continue
			}
			a, ok := netip.AddrFromSlice(ipNet.IP)
			if !ok {
				continue
			}
			a = a.Unmap()
			own = append(own, netip.PrefixFrom(a, a.BitLen()))
			if bits, _ := ipNet.Mask.Size(); bridgeInterface(iface.Name) && bits > 0 {
				if whole, err := a.Prefix(bits - (len(ipNet.Mask)*8 - a.BitLen())); err == nil {
					own = append(own, whole)
				}
			}
		}
	}
	return own, nil
}

// bridgeInterface is whether the interface named name is a container's or a VM's bridge, or one end of a container's
// link: Docker's (docker0, br-<id>, veth…), Podman's, nerdctl's and CNI's, libvirt's, LXC's, LXD's and Incus's,
// Multipass's, Kubernetes' network plugins', VirtualBox's and VMware's host networks.
func bridgeInterface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth", "podman", "cni", "virbr", "lxcbr", "lxdbr", "incusbr", "mpqemubr", "nerdctl",
		"flannel", "cali", "cilium_", "kube-bridge", "weave", "vboxnet", "vmnet"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (p *Proxy) dialAt(ctx context.Context, to netip.AddrPort) (net.Conn, error) {
	if p.dial != nil {
		return p.dial(ctx, to)
	}
	var d net.Dialer
	return d.DialContext(ctx, "tcp", to.String())
}

// splice copies a's bytes (read from aIn, which reads a) to b and b's to a, until both ends have said they're done,
// passing each end's close of its writing on to the other.
func splice(a net.Conn, aIn io.Reader, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(b, aIn)
		closeWrite(b)
	}()
	io.Copy(a, b)
	closeWrite(a)
	wg.Wait()
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
		return
	}
	c.Close()
}

// Forward is the forwarder's work inside the view: each connection l accepts goes on, byte for byte, to the proxy's
// socket at socket, until l is closed. A connection the socket doesn't take is closed.
func Forward(l net.Listener, socket string) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()
			far, err := net.DialTimeout("unix", socket, 5*time.Second)
			if err != nil {
				return
			}
			defer far.Close()
			splice(conn, conn, far)
		}()
	}
}

// proxyVars are the names ProxyEnv sets, and takes out of the desk's environment first, in either case: an owner's
// own proxy, on the computer's loopback, isn't the desk's.
var proxyVars = []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy", "NODE_USE_ENV_PROXY"}

// ProxyEnv is env with the desk's way out: HTTP_PROXY and HTTPS_PROXY (and their lower-case spellings, which curl and
// git read) at the forwarder on 127.0.0.1:port; NO_PROXY for the view's own loopback, so a desk's own servers are
// reached directly; and Node's NODE_USE_ENV_PROXY, so Node 24's built-in fetch takes the proxy (§A4.3). Every
// earlier value of these is taken out, ALL_PROXY's too.
func ProxyEnv(env []string, port int) []string {
	env = slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(proxyVars, name)
	})
	at := "http://127.0.0.1:" + strconv.Itoa(port)
	return append(env, "HTTP_PROXY="+at, "HTTPS_PROXY="+at, "http_proxy="+at, "https_proxy="+at,
		"NO_PROXY=localhost,127.0.0.1,::1", "no_proxy=localhost,127.0.0.1,::1", "NODE_USE_ENV_PROXY=1")
}

// wayOutHolds is the proxy row's check, from inside the view (enter_unix.go): the forwarder at addr answers, as the
// core's proxy, and refuses the computer's own address (a CONNECT to its loopback's port 1, never the app's). Anything
// else, a tunnel opened above all, is why not.
func wayOutHolds(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("the forwarder doesn't answer: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(conn, "CONNECT 127.0.0.1:1 HTTP/1.1\r\nHost: 127.0.0.1:1\r\n\r\n"); err != nil {
		return fmt.Errorf("the forwarder took nothing: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		return fmt.Errorf("the proxy didn't answer: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		return fmt.Errorf("the proxy answered %q to this computer's own address, not 403", resp.Status)
	}
	return nil
}
