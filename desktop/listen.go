package desktop

import (
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Where the server listens.
//
//   - "all": every interface, as the Mac app does and as this server always did.
//     Anything that can reach the computer's address can reach the port.
//   - "tailscale": this computer itself (127.0.0.1 and ::1: the setup page, the
//     hooks, the launcher and the office window are all local) and the address
//     Tailscale gave this computer. The home network, the Wi-Fi of a cafe, the
//     other side of the box: none of them can open the port at all.
//
// The default is "tailscale" on Windows and "all" elsewhere. The setting is
// "listen" in config.json ("all" or "tailscale"), read at start.
const (
	listenAll       = "all"
	listenTailscale = "tailscale"
)

// listenScope is the scope in force, set when the server starts, and read by
// phoneHost to give the phone an address that works.
var listenScope = defaultListenScope()

func defaultListenScope() string {
	if runtime.GOOS == "windows" {
		return listenTailscale
	}
	return listenAll
}

// scopeOf is the scope a config asks for: its own if valid, else the default.
func scopeOf(c *Config) string {
	switch strings.ToLower(strings.TrimSpace(c.Listen)) {
	case listenAll:
		return listenAll
	case listenTailscale:
		return listenTailscale
	}
	return defaultListenScope()
}

// How often the server looks again for Tailscale's address, so one that starts
// after Smolcorp is joined without a restart, and one that goes is let go.
var tailscaleRecheck = 15 * time.Second

// tailscaleAddresses is the IPv4 addresses Tailscale gave this computer. A
// variable so a test can put an address of its own in its place.
var tailscaleAddresses = func() []string {
	adapters, _ := net.Interfaces()
	var found []string
	for _, adapter := range adapters {
		addresses, _ := adapter.Addrs()
		var ips []net.IP
		for _, address := range addresses {
			if network, ok := address.(*net.IPNet); ok {
				ips = append(ips, network.IP)
			}
		}
		found = append(found, tailscaleFrom(adapter.Name, adapter.Flags&net.FlagUp != 0, ips)...)
	}
	return found
}

// tailnet is Tailscale's range, 100.64.0.0/10 (the carrier-grade NAT block).
var tailnet = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// tailscaleFrom picks the Tailscale addresses out of one adapter: it must be
// up, named Tailscale (tailscale0 on Linux), and the address IPv4 in the
// tailnet range. An address in that range on another adapter is not Tailscale's:
// some providers use it for their own customers.
func tailscaleFrom(name string, up bool, ips []net.IP) []string {
	if !up || !strings.HasPrefix(strings.ToLower(name), "tailscale") {
		return nil
	}
	var found []string
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil && tailnet.Contains(v4) {
			found = append(found, v4.String())
		}
	}
	return found
}

// phoneHost is the host a phone opens to reach this computer. With the server
// on Tailscale only, that is the Tailscale address, since nothing else answers;
// otherwise the address on the local network, as before, and Tailscale's only if
// there is no other.
func phoneHost() string {
	tailscale := tailscaleAddresses()
	if listenScope == listenTailscale {
		if len(tailscale) > 0 {
			return tailscale[0]
		}
		return "localhost"
	}
	if lan := localAddress(); lan != "localhost" || len(tailscale) == 0 {
		return lan
	}
	return tailscale[0]
}

// listenerFor opens what the scope asks for on port.
func listenerFor(scope string, port int) (net.Listener, error) {
	if scope != listenTailscale {
		return net.Listen("tcp", fmt.Sprintf(":%d", port))
	}
	set := newListenerSet(port)
	if err := set.add("127.0.0.1"); err != nil {
		return nil, err
	}
	// Where there's no IPv6 on loopback, 127.0.0.1 is all there is.
	if err := set.add("::1"); err != nil {
		log.Printf("smolcorp: no listening on ::1: %v", err)
	}
	// Tailscale's address only once the firewall's rules are set: opening it
	// first would have Windows ask, and make rules that let anyone in.
	set.follow(func() []string {
		if !firewallReady() {
			return nil
		}
		return tailscaleAddresses()
	}, tailscaleRecheck)
	return set, nil
}

// listenerSet is several listeners on one port, one per address, that serve as
// one: Accept gives the connections of all. Addresses can be added and taken
// away while it runs.
type listenerSet struct {
	port int

	mu      sync.Mutex
	open    map[string]net.Listener
	managed map[string]bool // the ones follow added, which it also removes
	closed  bool

	conns chan net.Conn
	done  chan struct{}
}

func newListenerSet(port int) *listenerSet {
	return &listenerSet{port: port, open: map[string]net.Listener{}, managed: map[string]bool{},
		conns: make(chan net.Conn), done: make(chan struct{})}
}

func (s *listenerSet) hosts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var hosts []string
	for host := range s.open {
		hosts = append(hosts, host)
	}
	return hosts
}

func (s *listenerSet) add(host string) error {
	return s.addListener(host, false)
}

func (s *listenerSet) addListener(host string, managed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return net.ErrClosed
	}
	if _, ok := s.open[host]; ok {
		return nil
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(s.port)))
	if err != nil {
		return err
	}
	s.open[host] = listener
	s.managed[host] = managed
	go s.pump(host, listener)
	return nil
}

// pump hands a listener's connections to Accept until it is closed.
func (s *listenerSet) pump(host string, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// Anything else is worth a pause, not a spin; and the end of it if the
			// listener has been let go meanwhile.
			s.mu.Lock()
			current := s.open[host] == listener
			s.mu.Unlock()
			if !current {
				return
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		select {
		case s.conns <- conn:
		case <-s.done:
			conn.Close()
			return
		}
	}
}

// follow keeps the managed addresses equal to what wanted() says, now and
// every interval after, until the set is closed.
func (s *listenerSet) follow(wanted func() []string, interval time.Duration) {
	s.sync(wanted())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sync(wanted())
			case <-s.done:
				return
			}
		}
	}()
}

// sync listens on the wanted addresses it doesn't yet, and stops listening on
// the managed ones that are no longer wanted. An address that can't be opened
// yet (the adapter is still coming up) is tried again next time.
func (s *listenerSet) sync(wanted []string) {
	want := map[string]bool{}
	for _, host := range wanted {
		want[host] = true
		s.mu.Lock()
		_, there := s.open[host]
		s.mu.Unlock()
		if there {
			continue
		}
		if err := s.addListener(host, true); err != nil {
			log.Printf("smolcorp: not yet listening on %s: %v", host, err)
		} else {
			log.Printf("smolcorp: listening on %s", host)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for host, listener := range s.open {
		if s.managed[host] && !want[host] {
			delete(s.open, host)
			delete(s.managed, host)
			listener.Close()
			log.Printf("smolcorp: no longer listening on %s", host)
		}
	}
}

func (s *listenerSet) Accept() (net.Conn, error) {
	select {
	case conn := <-s.conns:
		return conn, nil
	case <-s.done:
		return nil, net.ErrClosed
	}
}

func (s *listenerSet) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	for host, listener := range s.open {
		listener.Close()
		delete(s.open, host)
	}
	return nil
}

func (s *listenerSet) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: s.port}
}
