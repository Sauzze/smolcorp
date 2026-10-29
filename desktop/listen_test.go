package desktop

import (
	"net"
	"runtime"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnlyTailscalesAdaptersAddressesCount(t *testing.T) {
	ip := net.ParseIP
	for _, c := range []struct {
		name string
		up   bool
		ips  []net.IP
		want []string
	}{
		{"Tailscale", true, []net.IP{ip("100.101.102.103"), ip("fd7a:115c:a1e0::1")}, []string{"100.101.102.103"}},
		{"tailscale0", true, []net.IP{ip("100.64.0.1")}, []string{"100.64.0.1"}},
		{"Tailscale", false, []net.IP{ip("100.101.102.103")}, nil},
		// The same range on another adapter is a provider's, not Tailscale's
		{"Wi-Fi", true, []net.IP{ip("100.101.102.103")}, nil},
		{"Tailscale", true, []net.IP{ip("192.168.1.20")}, nil},
		{"Tailscale", true, []net.IP{ip("100.63.255.255"), ip("100.128.0.0")}, nil},
		{"Tailscale", true, []net.IP{ip("100.127.255.254")}, []string{"100.127.255.254"}},
	} {
		if got := tailscaleFrom(c.name, c.up, c.ips); !slices.Equal(got, c.want) {
			t.Errorf("%s up=%v %v: %v, want %v", c.name, c.up, c.ips, got, c.want)
		}
	}
}

func TestTheScopeFollowsTheConfigThenThePlatform(t *testing.T) {
	want := listenAll
	if runtime.GOOS == "windows" {
		want = listenTailscale
	}
	if got := scopeOf(&Config{}); got != want {
		t.Errorf("default: %q, want %q", got, want)
	}
	if got := scopeOf(&Config{Listen: "ALL"}); got != listenAll {
		t.Errorf("all: %q", got)
	}
	if got := scopeOf(&Config{Listen: " tailscale "}); got != listenTailscale {
		t.Errorf("tailscale: %q", got)
	}
	if got := scopeOf(&Config{Listen: "everything"}); got != want {
		t.Errorf("nonsense: %q, want the default %q", got, want)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// reaches is whether something accepts a connection at host:port.
func reaches(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// 127.0.0.2 stands in for Tailscale's address: it is this computer's, and no other
// device can reach it, but it is not the 127.0.0.1 the server always listens on.
const simulatedTailscale = "127.0.0.2"

// needsSimulatedTailscale skips a test that listens on simulatedTailscale
// where this computer doesn't answer on it: macOS gives loopback 127.0.0.1
// alone, where Windows and Linux give it the whole of 127.0.0.0/8.
func needsSimulatedTailscale(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", simulatedTailscale+":0")
	if err != nil {
		t.Skipf("this computer can't listen on %s: %v", simulatedTailscale, err)
	}
	listener.Close()
}

func withTailscale(t *testing.T, addresses *[]string) {
	t.Helper()
	savedAddresses, savedRecheck, savedReady := tailscaleAddresses, tailscaleRecheck, firewallReady
	tailscaleAddresses = func() []string { return slices.Clone(*addresses) }
	tailscaleRecheck = 50 * time.Millisecond
	// As once setup has set Windows Firewall's rules: see TestTailscaleWaitsForTheFirewallsRules.
	firewallReady = func() bool { return true }
	t.Cleanup(func() { tailscaleAddresses, tailscaleRecheck, firewallReady = savedAddresses, savedRecheck, savedReady })
}

func TestTheServerListensOnThisComputerAndTailscaleOnly(t *testing.T) {
	needsSimulatedTailscale(t)
	addresses := []string{simulatedTailscale}
	withTailscale(t, &addresses)
	port := freePort(t)
	listener, err := listenerFor(listenTailscale, port)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() { // what the server does: take the connections and let go
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	if !reaches("127.0.0.1", port) || !reaches(simulatedTailscale, port) {
		t.Fatal("the server isn't there for this computer and for Tailscale")
	}
	// An address of this computer that isn't either: the local network's, on a real machine.
	for _, host := range lanAddresses() {
		if reaches(host, port) {
			t.Errorf("the server answers on %s, which is neither this computer nor Tailscale", host)
		}
	}
	if reaches("127.0.0.3", port) {
		t.Error("the server answers on another loopback address")
	}
}

// lanAddresses: this computer's private IPv4 addresses on adapters that are up.
func lanAddresses() []string {
	var found []string
	adapters, _ := net.Interfaces()
	for _, adapter := range adapters {
		if adapter.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, _ := adapter.Addrs()
		for _, address := range addresses {
			// The addresses of a network the computer is on (not a link-local one of an unplugged cable)
			if network, ok := address.(*net.IPNet); ok && network.IP.To4() != nil && network.IP.IsPrivate() {
				found = append(found, network.IP.String())
			}
		}
	}
	return found
}

func TestTailscaleStartingAfterTheServerIsJoinedAndLeavingIsLetGo(t *testing.T) {
	needsSimulatedTailscale(t)
	var addresses []string
	withTailscale(t, &addresses)
	port := freePort(t)
	listener, err := listenerFor(listenTailscale, port)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	if reaches(simulatedTailscale, port) {
		t.Fatal("answering on Tailscale's address before Tailscale is there")
	}
	// Tailscale starts (its address appears)
	addresses = []string{simulatedTailscale}
	eventually(t, "joined Tailscale's address without a restart", func() bool { return reaches(simulatedTailscale, port) })
	if !reaches("127.0.0.1", port) {
		t.Fatal("this computer lost its server")
	}
	// Tailscale stops
	addresses = nil
	eventually(t, "let go of Tailscale's address", func() bool { return !reaches(simulatedTailscale, port) })
	if !reaches("127.0.0.1", port) {
		t.Fatal("this computer lost its server")
	}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("never %s", what)
}

// Until setup has set Windows Firewall's rules, Tailscale's address isn't
// opened, so Windows has nothing to ask about; once they're set, it is.
func TestTailscaleWaitsForTheFirewallsRules(t *testing.T) {
	needsSimulatedTailscale(t)
	addresses := []string{simulatedTailscale}
	withTailscale(t, &addresses)
	var set atomic.Bool
	firewallReady = set.Load
	port := freePort(t)
	listener, err := listenerFor(listenTailscale, port)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	if reaches(simulatedTailscale, port) {
		t.Fatal("Tailscale's address was opened before the firewall's rules were set")
	}
	if !reaches("127.0.0.1", port) {
		t.Fatal("this computer can't reach the server while it waits")
	}
	set.Store(true)
	eventually(t, "opened Tailscale's address once the rules were set", func() bool { return reaches(simulatedTailscale, port) })
}

func TestAllInterfacesRemainsAvailable(t *testing.T) {
	port := freePort(t)
	listener, err := listenerFor(listenAll, port)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	if !reaches("127.0.0.1", port) {
		t.Fatal("not on this computer")
	}
	for _, host := range lanAddresses() {
		if !reaches(host, port) {
			t.Errorf("not on %s, though all interfaces were asked for", host)
		}
	}
}

func TestAPortInUseIsAnError(t *testing.T) {
	var none []string
	withTailscale(t, &none)
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	if l, err := listenerFor(listenTailscale, taken.Addr().(*net.TCPAddr).Port); err == nil {
		l.Close()
		t.Fatal("no error for a port that's taken")
	}
}

func TestPhonesAreGivenTailscalesAddressWhenThatIsTheWayIn(t *testing.T) {
	addresses := []string{"100.101.102.103"}
	withTailscale(t, &addresses)
	saved := listenScope
	t.Cleanup(func() { listenScope = saved })

	listenScope = listenTailscale
	if got := phoneHost(); got != "100.101.102.103" {
		t.Errorf("with the server on Tailscale: %s", got)
	}
	if got := phoneAddress(); got != "http://100.101.102.103:"+strconv.Itoa(Port) {
		t.Errorf("phone address %s", got)
	}
	if got := pairingLink("123456"); got != phoneAddress()+"/?pair=123456" {
		t.Errorf("pairing link %s", got)
	}
	// No Tailscale: no address that can work, so none that looks like it would.
	addresses = nil
	if got := phoneHost(); got != "localhost" {
		t.Errorf("with Tailscale off: %s", got)
	}
	// On all interfaces the local network's address is kept, as it was.
	listenScope = listenAll
	addresses = []string{"100.101.102.103"}
	if got, lan := phoneHost(), localAddress(); lan != "localhost" && got != lan {
		t.Errorf("on all interfaces: %s, want the local network's %s", got, lan)
	}
}
