package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"
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
	// Windows and Linux: this computer and Tailscale, until you let the home network in. The Mac: everyone, as its app.
	for system, scope := range map[string]string{"windows": listenTailscale, "linux": listenTailscale, "darwin": listenAll} {
		if got := defaultScopeOn(system); got != scope {
			t.Errorf("the default on %s: %q, want %q", system, got, scope)
		}
	}
	want := defaultScopeOn(runtime.GOOS)
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

func TestALinuxInstallFromBeforeKeepsItsWiFi(t *testing.T) {
	for _, c := range []struct {
		system, listen string
		phones         int
		want           string
		changed        bool
	}{
		// From 1.22.1, which listened everywhere, with a phone paired: most likely on the Wi-Fi, so it stays.
		{"linux", "", 1, listenAll, true},
		// A new install, or one with no phone yet: the default, written down so a phone paired later changes nothing.
		{"linux", "", 0, listenTailscale, true},
		// Chosen already, either way: left alone.
		{"linux", listenTailscale, 2, listenTailscale, false},
		{"linux", listenAll, 0, listenAll, false},
		{"linux", " ALL ", 0, " ALL ", false},
		// Neither, as before: the default was every interface then, so it's as if unset.
		{"linux", "lan", 1, listenAll, true},
		{"linux", "lan", 0, listenTailscale, true},
		// Windows has had this default all along, and the Mac never had it.
		{"windows", "", 1, "", false},
		{"darwin", "", 1, "", false},
	} {
		config := &Config{Listen: c.listen}
		if changed := settleListenScope(config, c.phones, c.system); changed != c.changed || config.Listen != c.want {
			t.Errorf("%s, listen %q, %d phones: %q (changed %v), want %q (changed %v)", c.system, c.listen, c.phones, config.Listen, changed, c.want, c.changed)
		}
	}
}

func TestNoQRCodeWithNoAddressAPhoneCanOpen(t *testing.T) {
	d := newTestDeck(t)
	savedScope, savedAddresses := listenScope, tailscaleAddresses
	t.Cleanup(func() { listenScope, tailscaleAddresses = savedScope, savedAddresses })
	listenScope = listenTailscale
	tailscaleAddresses = func() []string { return nil }
	serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if got := serve(t, d, call{method: "GET", target: "/setup/qr.png", host: "localhost:7710", from: thisComputer}).Code; got != http.StatusNotFound {
		t.Errorf("a QR code with no address a phone can open: %d, want %d", got, http.StatusNotFound)
	}
	tailscaleAddresses = func() []string { return []string{"100.101.102.103"} }
	if got := serve(t, d, call{method: "GET", target: "/setup/qr.png", host: "localhost:7710", from: thisComputer}).Code; got != http.StatusOK {
		t.Errorf("the QR code with Tailscale's address: %d, want %d", got, http.StatusOK)
	}
}

// Pair a phone says the Tailscale address beside the Wi-Fi one, only where it is another way in.
func TestPairingSaysTheTailscaleAddressOnlyWhereItIsAnotherWayIn(t *testing.T) {
	savedScope, savedAddresses := listenScope, tailscaleAddresses
	t.Cleanup(func() { listenScope, tailscaleAddresses = savedScope, savedAddresses })
	listenScope = listenAll
	tailscaleAddresses = func() []string { return nil }
	if got := phoneTailnet(); got != "" {
		t.Errorf("no Tailscale here: %q, want none", got)
	}
	tailscaleAddresses = func() []string { return []string{"100.101.102.103"} }
	want := fmt.Sprintf("http://100.101.102.103:%d", Port)
	if localAddress() == "localhost" {
		want = "" // no Wi-Fi here: the phone's address is Tailscale's already
	}
	if got := phoneTailnet(); got != want {
		t.Errorf("on the Wi-Fi and Tailscale: %q, want %q", got, want)
	}
	listenScope = listenTailscale
	if got := phoneTailnet(); got != "" {
		t.Errorf("on Tailscale only, where the phone's address is Tailscale's: %q, want none", got)
	}
	d := newTestDeck(t)
	pairing, _ := (&router{deck: d}).setupStateFor(callerPage)["pairing"].(map[string]any)
	if got, ok := pairing["tailnet"].(string); !ok || got != "" {
		t.Errorf("the setup state's pairing tailnet: %#v, want \"\"", pairing["tailnet"])
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
	// The server rechecks from its own goroutine while the test changes the
	// answer, so the answer is guarded.
	var mu sync.Mutex
	var addresses []string
	setAddresses := func(a []string) { mu.Lock(); addresses = a; mu.Unlock() }
	withTailscale(t, &addresses)
	tailscaleAddresses = func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(addresses) }
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
	setAddresses([]string{simulatedTailscale})
	eventually(t, "joined Tailscale's address without a restart", func() bool { return reaches(simulatedTailscale, port) })
	if !reaches("127.0.0.1", port) {
		t.Fatal("this computer lost its server")
	}
	// Tailscale stops
	setAddresses(nil)
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
	// On a Mac, a test binary listening on every interface can have the firewall ask
	// the owner about it: only when the LAN is asked for, as for a test copy.
	if runtime.GOOS == "darwin" && os.Getenv(testListenVariable) != "lan" {
		t.Skip("every interface on a Mac only with " + testListenVariable + "=lan")
	}
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
	if got := pairingLink("a-key"); got != phoneAddress()+"/#pair=a-key" {
		t.Errorf("pairing link %s", got)
	}
	// No Tailscale: no address that can work, so none that looks like it would.
	addresses = nil
	if got := phoneHost(); got != "" || phoneAddress() != "" {
		t.Errorf("with Tailscale off: %q, %q; want no address, never localhost, which a phone can't open", got, phoneAddress())
	}
	// On all interfaces the local network's address is kept, as it was.
	listenScope = listenAll
	addresses = []string{"100.101.102.103"}
	if got, lan := phoneHost(), localAddress(); lan != "localhost" && got != lan {
		t.Errorf("on all interfaces: %s, want the local network's %s", got, lan)
	}
}

func TestTailscaleAdapterEdges(t *testing.T) {
	ip := net.ParseIP
	for _, c := range []struct {
		name string
		up   bool
		ips  []net.IP
		want []string
	}{
		// Tailscale's range is 100.64.0.0/10, both ends in; the adapter's name isn't case-sensitive
		{"TAILSCALE", true, []net.IP{ip("100.64.0.0")}, []string{"100.64.0.0"}},
		{"Tailscale 2", true, []net.IP{ip("100.100.100.100")}, []string{"100.100.100.100"}},
		// An IPv4 address written the IPv6 way is still the same address
		{"tailscale0", true, []net.IP{ip("::ffff:100.101.102.103")}, []string{"100.101.102.103"}},
		// Nothing a phone could use: IPv6 alone, no address, an unnamed adapter, a name that only contains it
		{"tailscale0", true, []net.IP{ip("fd7a:115c:a1e0::1")}, nil},
		{"tailscale0", true, nil, nil},
		{"", true, []net.IP{ip("100.101.102.103")}, nil},
		{"not-tailscale0", true, []net.IP{ip("100.101.102.103")}, nil},
		// More than one: all of them, in order
		{"tailscale0", true, []net.IP{ip("100.101.102.103"), ip("100.101.102.104")}, []string{"100.101.102.103", "100.101.102.104"}},
	} {
		if got := tailscaleFrom(c.name, c.up, c.ips); !slices.Equal(got, c.want) {
			t.Errorf("%q up=%v %v: %v, want %v", c.name, c.up, c.ips, got, c.want)
		}
	}
}

// What Linux settles on at the first start of this version, and what the server then asks for.
func TestAMigratedLinuxConfigAsksForTheScopeItWasSettledOn(t *testing.T) {
	for _, c := range []struct {
		listen string
		phones int
		want   string
	}{
		{"", 1, listenAll},
		{"", 0, listenTailscale},
		{"lan", 3, listenAll},
		{"lan", 0, listenTailscale},
		{" Tailscale ", 1, listenTailscale},
		{"ALL", 0, listenAll},
	} {
		config := &Config{Listen: c.listen}
		settleListenScope(config, c.phones, "linux")
		if got := scopeOf(config); got != c.want {
			t.Errorf("listen %q, %d phones: the server asks for %q, want %q", c.listen, c.phones, got, c.want)
		}
		// Settling twice is settling once
		before := config.Listen
		if settleListenScope(config, c.phones+1, "linux") || config.Listen != before {
			t.Errorf("listen %q, %d phones: settled again, to %q", c.listen, c.phones, config.Listen)
		}
	}
}

// writePhones puts paired phones in this test's phones file, as an earlier version left them.
func writePhones(t *testing.T, names ...string) {
	t.Helper()
	phones := &Phones{}
	for _, name := range names {
		phones.All = append(phones.All, PairedPhone{ID: name, Name: name, KeyHash: "00"})
	}
	phones.save()
}

// savedListen is what config.json says "listen" is, "" if it doesn't say.
func savedListen(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(configFile)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	} else if err != nil {
		t.Fatal(err)
	}
	var fields struct{ Listen string }
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields.Listen
}

func TestStartingOnLinuxWritesTheScopeDownBySettlingOnThePhonesPaired(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the migration is for Linux, and NewDeck asks which system this is")
	}
	for _, c := range []struct {
		name   string
		phones []string
		config string
		want   string
	}{
		{"a phone paired", []string{"Pixel"}, "", listenAll},
		{"no phone", nil, "", listenTailscale},
		// The office's own window is a paired device, not a phone on the Wi-Fi
		{"only the window", []string{windowDeviceName, windowDeviceNameBefore}, "", listenTailscale},
		{"the window and a phone", []string{windowDeviceName, "Pixel"}, "", listenAll},
		{"chosen already", []string{"Pixel"}, `{"listen": "tailscale"}`, listenTailscale},
		{"neither, as before", []string{"Pixel"}, `{"listen": "lan"}`, listenAll},
	} {
		t.Run(c.name, func(t *testing.T) {
			isolate(t)
			writePhones(t, c.phones...)
			if c.config != "" {
				if err := os.WriteFile(configFile, []byte(c.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			d := startDeck(t)
			if d.config.Listen != c.want || scopeOf(d.config) != c.want {
				t.Errorf("listen %q, scope %q, want %q", d.config.Listen, scopeOf(d.config), c.want)
			}
			if got := savedListen(t); got != c.want {
				t.Errorf("config.json says %q, want %q written down", got, c.want)
			}
		})
	}
}

func TestAPhonePairedAfterTheMigrationChangesNothingOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the migration is for Linux, and NewDeck asks which system this is")
	}
	isolate(t)
	if d := startDeck(t); d.config.Listen != listenTailscale {
		t.Fatalf("a new install settled on %q", d.config.Listen)
	}
	writePhones(t, "Pixel")
	if d := startDeck(t); d.config.Listen != listenTailscale || scopeOf(d.config) != listenTailscale {
		t.Errorf("the next start, with a phone: %q, want it to stay %q", d.config.Listen, listenTailscale)
	}
	if got := savedListen(t); got != listenTailscale {
		t.Errorf("config.json says %q", got)
	}
}

func TestAConfigWithAPhoneKeepsItsOtherSettingsWhenTheScopeIsWritten(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the migration is for Linux, and NewDeck asks which system this is")
	}
	isolate(t)
	writePhones(t, "Pixel")
	if err := os.WriteFile(configFile, []byte(`{"alwaysOnTop": true, "somethingNewer": 7}`), 0o600); err != nil {
		t.Fatal(err)
	}
	startDeck(t)
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["listen"] != listenAll || fields["alwaysOnTop"] != true || fields["somethingNewer"] != float64(7) {
		t.Errorf("config.json after settling: %s", data)
	}
	if filepath.Dir(configFile) != supportDir {
		t.Errorf("config at %s, outside the test's folder", configFile)
	}
}

// openSet is a listener set on a free port, with the connections taken and let go.
func openSet(t *testing.T) (*listenerSet, int) {
	t.Helper()
	port := freePort(t)
	set := newListenerSet(port)
	t.Cleanup(func() { set.Close() })
	go func() {
		for {
			conn, err := set.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return set, port
}

func TestSyncTakesOnlyTheAddressesItAddedAway(t *testing.T) {
	needsSimulatedTailscale(t)
	set, port := openSet(t)
	if err := set.add("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	set.sync(nil)
	if !reaches("127.0.0.1", port) {
		t.Fatal("sync with nothing wanted took this computer's own address away")
	}
	set.sync([]string{simulatedTailscale})
	if !reaches(simulatedTailscale, port) {
		t.Fatal("not listening on the address that was wanted")
	}
	set.sync([]string{simulatedTailscale}) // the same again: nothing changes, nothing breaks
	if got := len(set.hosts()); got != 2 {
		t.Errorf("%d addresses after wanting the same one twice: %v", got, set.hosts())
	}
	set.sync(nil)
	if reaches(simulatedTailscale, port) || !reaches("127.0.0.1", port) {
		t.Error("with Tailscale gone: its address should be let go and this computer's kept")
	}
}

func TestAnAddressThatCantBeOpenedYetIsLeftForNextTime(t *testing.T) {
	needsSimulatedTailscale(t)
	set, port := openSet(t)
	if err := set.add("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	// 192.0.2.1 is reserved for documentation: never an address of this computer
	set.sync([]string{"192.0.2.1", simulatedTailscale})
	if slices.Contains(set.hosts(), "192.0.2.1") {
		t.Error("listening on an address this computer doesn't have")
	}
	if !reaches(simulatedTailscale, port) || !reaches("127.0.0.1", port) {
		t.Error("one address that failed took the others with it")
	}
}

func TestAnAddressAlreadyAddedIsNotOpenedTwice(t *testing.T) {
	set, port := openSet(t)
	for range 2 {
		if err := set.add("127.0.0.1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := set.hosts(); !slices.Equal(got, []string{"127.0.0.1"}) {
		t.Errorf("hosts %v", got)
	}
	if !reaches("127.0.0.1", port) {
		t.Error("not there")
	}
}

func TestClosingTheSetLetsGoOfEveryAddress(t *testing.T) {
	needsSimulatedTailscale(t)
	set, port := openSet(t)
	if err := set.add("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	set.sync([]string{simulatedTailscale})
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if err := set.Close(); err != nil {
		t.Errorf("closing twice: %v", err)
	}
	if _, err := set.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Errorf("accepting once closed: %v", err)
	}
	if err := set.add("127.0.0.1"); !errors.Is(err, net.ErrClosed) {
		t.Errorf("adding once closed: %v", err)
	}
	set.sync([]string{simulatedTailscale}) // the follower's last tick may land after the close
	if reaches("127.0.0.1", port) || reaches(simulatedTailscale, port) {
		t.Error("still answering after the close")
	}
}

func TestFollowingStopsWhenTheSetIsClosed(t *testing.T) {
	needsSimulatedTailscale(t)
	var calls atomic.Int32
	set, port := openSet(t)
	set.follow(func() []string { calls.Add(1); return []string{simulatedTailscale} }, 10*time.Millisecond)
	eventually(t, "listened on the address it follows", func() bool { return reaches(simulatedTailscale, port) })
	set.Close()
	time.Sleep(50 * time.Millisecond) // for a tick already on its way
	seen := calls.Load()
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != seen {
		t.Error("still asking where to listen after the close")
	}
	if reaches(simulatedTailscale, port) {
		t.Error("answering after the close")
	}
}

func TestTheServerAsksForTailscaleAgainWhenTheAddressChanges(t *testing.T) {
	needsSimulatedTailscale(t)
	var mu sync.Mutex
	var addresses []string
	withTailscale(t, &addresses)
	tailscaleAddresses = func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(addresses) }
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
	// Tailscale on, then its address given up for the same one another time: still served
	for _, a := range [][]string{{simulatedTailscale}, nil, {simulatedTailscale}} {
		mu.Lock()
		addresses = a
		mu.Unlock()
		want := len(a) > 0
		eventually(t, "followed Tailscale's address", func() bool { return reaches(simulatedTailscale, port) == want })
		if !reaches("127.0.0.1", port) {
			t.Fatal("this computer lost its server")
		}
	}
}

func TestPhoneHostWithSeveralTailscaleAddressesAndOnTheWiFi(t *testing.T) {
	addresses := []string{"100.101.102.103", "100.101.102.104"}
	withTailscale(t, &addresses)
	saved := listenScope
	t.Cleanup(func() { listenScope = saved })

	// Tailscale only: the first of them, whatever the local network has
	listenScope = listenTailscale
	if got := phoneHost(); got != "100.101.102.103" {
		t.Errorf("tailscale scope, two addresses: %q", got)
	}
	if lan := localAddress(); lan != "localhost" && phoneHost() == lan {
		t.Errorf("tailscale scope gave the local network's %s, which nothing answers on", lan)
	}
	// Everyone: the local network's address, and Tailscale's only when there is no other
	listenScope = listenAll
	lan := localAddress()
	want := lan
	if lan == "localhost" {
		want = "100.101.102.103"
	}
	if got := phoneHost(); got != want {
		t.Errorf("all scope with Tailscale: %q, want %q", got, want)
	}
	addresses = nil
	if got := phoneHost(); got != lan {
		t.Errorf("all scope without Tailscale: %q, want %q", got, lan)
	}
}

// A test copy listens on this computer only, whatever its file says, unless the
// LAN is asked for; the office's own keeps its scope.
func TestATestCopyListensOnThisComputerOnly(t *testing.T) {
	for _, c := range []struct {
		scope    string
		testPort int
		lan      string
		want     string
	}{
		{listenAll, 0, "", listenAll},
		{listenTailscale, 0, "lan", listenTailscale},
		{listenAll, 7795, "", listenThisComputer},
		{listenTailscale, 7795, "", listenThisComputer},
		{listenTailscale, 7795, "lan", listenAll},
		{listenTailscale, 7795, " LAN ", listenAll},
		{listenAll, 7795, "all", listenThisComputer},
	} {
		if got := testCopyScope(c.scope, c.testPort, c.lan); got != c.want {
			t.Errorf("testCopyScope(%q, %d, %q) = %q, want %q", c.scope, c.testPort, c.lan, got, c.want)
		}
	}
	// "this-computer" is never a config value: a file asking for it gets the default.
	if got := scopeOf(&Config{Listen: listenThisComputer}); got != defaultListenScope() {
		t.Errorf("a config asking for %q: %q", listenThisComputer, got)
	}
}

// At the socket: a test copy's listeners are bound to 127.0.0.1 and ::1 only,
// and never to Tailscale's address, even with one there.
func TestATestCopysServerIsBoundToLoopbackOnly(t *testing.T) {
	addresses := []string{simulatedTailscale}
	withTailscale(t, &addresses)
	port := freePort(t)
	listener, err := listenerFor(listenThisComputer, port)
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
	set, ok := listener.(*listenerSet)
	if !ok {
		t.Fatalf("a %T, want a set of loopback listeners", listener)
	}
	time.Sleep(3 * tailscaleRecheck) // long enough for a follow to have added Tailscale, were there one
	set.mu.Lock()
	var bound []string
	for _, l := range set.open {
		address := l.Addr().(*net.TCPAddr)
		bound = append(bound, address.String())
		if !address.IP.IsLoopback() || !(address.IP.Equal(net.IPv4(127, 0, 0, 1)) || address.IP.Equal(net.IPv6loopback)) {
			t.Errorf("bound to %s, want 127.0.0.1 or ::1 only", address)
		}
	}
	set.mu.Unlock()
	if !slices.Contains(bound, net.JoinHostPort("127.0.0.1", strconv.Itoa(port))) {
		t.Errorf("bound to %v, not 127.0.0.1", bound)
	}
	if !reaches("127.0.0.1", port) {
		t.Fatal("not there for this computer")
	}
	if reaches(simulatedTailscale, port) {
		t.Error("a test copy answers on Tailscale's address")
	}
	for _, host := range lanAddresses() {
		if reaches(host, port) {
			t.Errorf("a test copy answers on %s", host)
		}
	}
}

// A test copy on this computer only gives a phone no address: none it could open.
func TestATestCopyOnThisComputerGivesAPhoneNoAddress(t *testing.T) {
	addresses := []string{"100.101.102.103"}
	withTailscale(t, &addresses)
	saved := listenScope
	t.Cleanup(func() { listenScope = saved })
	listenScope = listenThisComputer
	if got := phoneHost(); got != "" || phoneAddress() != "" {
		t.Errorf("phone host %q, address %q: want none", got, phoneAddress())
	}
}
