package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Two Macs, in one process: the home, served on a loopback port of 7750–7799 behind a listener that says each
// connection came over Tailscale (or the Wi-Fi), and the one joining it, asked in-process as its setup page asks.
// Neither reaches a real network, Tailscale, the Keychain or a secret store: the joining one's store is a map.

// tailHome is the home Mac's Tailscale address in these tests, as tailscaleAddresses gives it.
const tailHome = "100.64.1.2"

// addrOf is an address as a connection gives it.
func addrOf(text string) net.Addr {
	host, port, _ := net.SplitHostPort(text)
	var n int
	fmt.Sscan(port, &n)
	return &net.TCPAddr{IP: net.ParseIP(host), Port: n}
}

// asIfConn is a loopback connection that gives the addresses a Tailscale (or Wi-Fi) one would, and keeps what
// crosses it.
type asIfConn struct {
	net.Conn
	remote, local net.Addr
	seen          *crossed
}

func (c *asIfConn) RemoteAddr() net.Addr { return c.remote }
func (c *asIfConn) LocalAddr() net.Addr  { return c.local }
func (c *asIfConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.seen.add(&c.seen.in, b[:n])
	return n, err
}
func (c *asIfConn) Write(b []byte) (int, error) {
	c.seen.add(&c.seen.out, b)
	return c.Conn.Write(b)
}

// crossed is every byte the home Mac read (in) and wrote (out).
type crossed struct {
	mu      sync.Mutex
	in, out bytes.Buffer
}

func (c *crossed) add(to *bytes.Buffer, b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	to.Write(b)
}

func (c *crossed) both() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.in.String() + c.out.String()
}

type asIfListener struct {
	net.Listener
	remote, local net.Addr
	seen          *crossed
}

func (l *asIfListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &asIfConn{Conn: conn, remote: l.remote, local: l.local, seen: l.seen}, nil
}

// twoMacs is a home Mac and one to join it, both with floors from other Macs on.
type twoMacs struct {
	home, joiner *Deck
	// The home's address on the tailnet, as the joining one types it, and what the joining one dialled.
	address string
	dialled []string
	seen    *crossed
	// The joining one's secret store.
	kept map[string]string
	mu   sync.Mutex
}

// newTwoMacs starts the two, the home's connections coming from remote to local (a Tailscale pair, or Wi-Fi).
func newTwoMacs(t *testing.T, remote, local string) *twoMacs {
	t.Helper()
	savedHost, savedAddresses, savedDial, savedGap, savedName := host, tailscaleAddresses, dialPeer, peerCodeTryGap, peerMachineName
	t.Cleanup(func() {
		host, tailscaleAddresses, dialPeer, peerCodeTryGap, peerMachineName = savedHost, savedAddresses, savedDial, savedGap, savedName
		peerClient.CloseIdleConnections()
	})
	host = nil
	tailscaleAddresses = func() []string { return []string{tailHome} }
	peerMachineName = func() string { return "Studio" }
	m := &twoMacs{home: newTestDeck(t), seen: &crossed{}, kept: map[string]string{}}
	m.joiner = newTestDeck(t)
	m.joiner.mu.Lock()
	m.joiner.peerKeys = peerKeyStore{
		read: func() (string, error) { m.mu.Lock(); defer m.mu.Unlock(); return m.kept[peerKeysItem], nil },
		write: func(value string) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.kept[peerKeysItem] = value
			return nil
		},
	}
	m.joiner.mu.Unlock()
	var listener net.Listener
	for port := 7750; port <= 7799 && listener == nil; port++ {
		listener, _ = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	}
	if listener == nil {
		t.Fatal("no free port in 7750–7799")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_, localPort, _ := net.SplitHostPort(local)
	m.address = fmt.Sprintf("%s:%d", tailHome, port)
	if localPort == "0" {
		local = net.JoinHostPort(strings.Split(local, ":")[0], fmt.Sprint(port))
	}
	server := &http.Server{Handler: &router{deck: m.home}}
	go server.Serve(&asIfListener{Listener: listener, remote: addrOf(remote), local: addrOf(local), seen: m.seen})
	t.Cleanup(func() { _ = server.Close() })
	dialPeer = func(ctx context.Context, address string) (net.Conn, error) {
		m.mu.Lock()
		m.dialled = append(m.dialled, address)
		m.mu.Unlock()
		if address != m.address {
			return nil, errNotTailnet
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", listener.Addr().String())
	}
	for _, d := range []*Deck{m.home, m.joiner} {
		if got := peerPage(t, d, "POST", "/setup/peers?enabled=1", ""); got.Code != http.StatusOK {
			t.Fatalf("turning floors on: %d %s", got.Code, got.Body)
		}
	}
	return m
}

// peerPage asks a deck's setup as its page does.
func peerPage(t *testing.T, d *Deck, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, body: body,
		headers: map[string]string{setupHeader: "1"}})
}

// code is a new code on the home Mac, from its page.
func (m *twoMacs) code(t *testing.T) string {
	t.Helper()
	asked := peerPage(t, m.home, "POST", "/setup/peers/code", "")
	var state struct{ Code string }
	if err := json.Unmarshal(asked.Body.Bytes(), &state); err != nil || len(state.Code) != 6 {
		t.Fatalf("asking for a code: %d %s", asked.Code, asked.Body)
	}
	return state.Code
}

// join has the joining Mac's page enter code for the home.
func (m *twoMacs) join(t *testing.T, address, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"address": address, "code": code, "name": "Laptop"})
	return peerPage(t, m.joiner, "POST", "/setup/peers/join", string(body))
}

// key is the joining Mac's key for the home, from its store.
func (m *twoMacs) key(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	var joined []joinedMac
	if err := json.Unmarshal([]byte(m.kept[peerKeysItem]), &joined); err != nil || len(joined) != 1 || joined[0].Key == "" {
		t.Fatalf("the joining Mac keeps %q", m.kept[peerKeysItem])
	}
	return joined[0].Key
}

// ask asks the home Mac over the network, as the joining one would, with the key.
func (m *twoMacs) ask(t *testing.T, method, path, key string) (int, string) {
	t.Helper()
	r, _ := http.NewRequest(method, "http://"+m.address+path, strings.NewReader("{}"))
	if key != "" {
		r.Header.Set(peerKeyHeader, key)
	}
	answer, err := peerClient.Do(r)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer answer.Body.Close()
	var body bytes.Buffer
	_, _ = body.ReadFrom(answer.Body)
	return answer.StatusCode, strings.TrimSpace(body.String())
}

// peerAsk asks a deck in-process as another Mac would: from from (host:port), to the deck's Tailscale address
// unless to says otherwise.
func peerAsk(d *Deck, method, target, key, from, to, body string) *httptest.ResponseRecorder {
	if to == "" {
		to = tailHome + ":7710"
	}
	return peerAskVia(d, method, target, key, from, to, to, body, nil)
}

// peerAskVia is peerAsk with the Host sent apart from the address the connection reached (local), and headers.
func peerAskVia(d *Deck, method, target, key, from, host, local, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host, r.RemoteAddr = host, from
	r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, addrOf(local)))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		r.Header.Set(peerKeyHeader, key)
	}
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w
}

const (
	otherMac   = "100.101.102.103:50000"
	wifiMac    = "192.168.1.20:50000"
	homeOnWiFi = "192.168.1.10:7710"
)

func TestTwoMacsPairOverTailscale(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	joined := m.join(t, m.address, m.code(t))
	if joined.Code != http.StatusOK {
		t.Fatalf("joining: %d %s", joined.Code, joined.Body)
	}
	var state struct {
		Joined []map[string]any
	}
	_ = json.Unmarshal(joined.Body.Bytes(), &state)
	if len(state.Joined) != 1 || state.Joined[0]["name"] != "Studio" || state.Joined[0]["address"] != m.address || state.Joined[0]["key"] != nil {
		t.Errorf("the joining Mac lists %v", state.Joined)
	}
	key := m.key(t)
	// The home lists it by the name it sent, never with a key.
	listed := peerPage(t, m.home, "GET", "/setup/peers", "")
	if !strings.Contains(listed.Body.String(), `"name":"Laptop"`) || strings.Contains(listed.Body.String(), key) ||
		strings.Contains(listed.Body.String(), hashKey(key)) {
		t.Errorf("the home lists %s", listed.Body)
	}
	if status, body := m.ask(t, "GET", "/peer/hello", key); status != http.StatusOK || !strings.Contains(body, "Studio") {
		t.Errorf("hello with the key: %d %s", status, body)
	}
	// The code was used up.
	if again := m.join(t, m.address, "000000"); again.Code != http.StatusForbidden {
		t.Errorf("a second join: %d %s", again.Code, again.Body)
	}
	if status, _ := m.ask(t, "GET", "/peer/hello", "not-a-real-key"); status != http.StatusUnauthorized {
		t.Errorf("hello with a wrong key: %d", status)
	}
}

func TestAWrongExpiredOrOverusedCodeIsRefused(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	peerCodeTryGap = 0
	code := m.code(t)
	wrong := fmt.Sprintf("%06d", (atoi(code)+1)%1_000_000)
	if got := m.join(t, m.address, wrong); got.Code != http.StatusForbidden {
		t.Errorf("a wrong code: %d %s", got.Code, got.Body)
	}
	if len(m.kept) != 0 || len(m.home.macPeers.All) != 0 {
		t.Errorf("a wrong code paired: %v %v", m.kept, m.home.macPeers.All)
	}
	// Run out.
	m.home.mu.Lock()
	m.home.macPeers.code.expiry = time.Now().Add(-time.Second)
	m.home.mu.Unlock()
	if got := m.join(t, m.address, code); got.Code != http.StatusForbidden {
		t.Errorf("a code run out: %d %s", got.Code, got.Body)
	}
	// Five wrong tries from one address lock it out, the right code with them.
	code = m.code(t)
	wrong = fmt.Sprintf("%06d", (atoi(code)+1)%1_000_000)
	for range maxAttemptsPerAddress {
		m.join(t, m.address, wrong)
	}
	if got := m.join(t, m.address, code); got.Code != http.StatusForbidden || len(m.home.macPeers.All) != 0 {
		t.Errorf("the right code after five wrong: %d %s", got.Code, got.Body)
	}
	// Twenty from everywhere take the code away.
	code = m.code(t)
	wrong = fmt.Sprintf("%06d", (atoi(code)+1)%1_000_000)
	for i := range maxAttempts {
		peerAsk(m.home, "POST", peerPairPath, "", fmt.Sprintf("100.101.102.%d:50000", i+1), "", `{"code":"`+wrong+`"}`)
	}
	if got := peerAsk(m.home, "POST", peerPairPath, "", "100.101.103.1:50000", "", `{"code":"`+code+`"}`); got.Code != http.StatusForbidden {
		t.Errorf("the right code after twenty wrong: %d %s", got.Code, got.Body)
	}
	// A second try within peerCodeTryGap isn't looked at, nor counted.
	peerCodeTryGap = time.Hour
	code = m.code(t)
	wrong = fmt.Sprintf("%06d", (atoi(code)+1)%1_000_000)
	peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+wrong+`"}`)
	if got := peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+code+`"}`); got.Code != http.StatusTooManyRequests {
		t.Errorf("a try at once: %d %s", got.Code, got.Body)
	}
	m.home.mu.Lock()
	// Counted by IP, the port aside, as remoteAddress gives it.
	otherIP, _, _ := net.SplitHostPort(otherMac)
	misses := m.home.macPeers.code.misses[otherIP]
	m.home.mu.Unlock()
	if misses != 1 {
		t.Errorf("%d misses counted, want 1", misses)
	}
	// A phone's code never pairs a Mac.
	peerCodeTryGap = 0
	m.home.mu.Lock()
	phoneCode := m.home.access.NewCode()
	m.home.macPeers.code.withdraw()
	m.home.mu.Unlock()
	if got := m.join(t, m.address, phoneCode); got.Code != http.StatusForbidden || len(m.home.macPeers.All) != 0 {
		t.Errorf("a phone's code: %d %s", got.Code, got.Body)
	}
}

func atoi(text string) int {
	var n int
	fmt.Sscan(text, &n)
	return n
}

func TestARevokedMacIsRefusedAtOnce(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	if got := m.join(t, m.address, m.code(t)); got.Code != http.StatusOK {
		t.Fatalf("joining: %d %s", got.Code, got.Body)
	}
	key := m.key(t)
	if status, _ := m.ask(t, "GET", "/peer/hello", key); status != http.StatusOK {
		t.Fatalf("hello: %d", status)
	}
	id := m.home.macPeers.All[0].ID
	m.home.mu.Lock()
	m.home.gmToken = "fake-gm-token"
	m.home.mu.Unlock()
	// Removing is the page's alone.
	for _, c := range []call{
		{asTool: true},
		{withoutSetupKey: true, headers: map[string]string{gmTokenHeader: "fake-gm-token"}},
	} {
		c.method, c.target, c.host, c.from = "POST", "/setup/peers/revoke?peer="+id, "localhost:7710", thisComputer
		c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
		if got := serve(t, m.home, c); got.Code == http.StatusOK {
			t.Errorf("removed by someone other than the page: %+v", c)
		}
	}
	if got := peerPage(t, m.home, "POST", "/setup/peers/revoke?peer="+id, ""); got.Code != http.StatusOK {
		t.Fatalf("removing: %d %s", got.Code, got.Body)
	}
	if status, _ := m.ask(t, "GET", "/peer/hello", key); status != http.StatusUnauthorized {
		t.Errorf("hello once removed: %d", status)
	}
	if got := peerPage(t, m.home, "POST", "/setup/peers/revoke?peer="+id, ""); got.Code != http.StatusNotFound {
		t.Errorf("removing again: %d", got.Code)
	}
}

// pairedPeer is a key paired in-process with the home's code, over Tailscale.
func pairedPeer(t *testing.T, m *twoMacs) string {
	t.Helper()
	got := peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+m.code(t)+`","name":"Laptop"}`)
	var answer struct{ Key string }
	if err := json.Unmarshal(got.Body.Bytes(), &answer); err != nil || answer.Key == "" {
		t.Fatalf("pairing: %d %s", got.Code, got.Body)
	}
	return answer.Key
}

func TestAPeerKeyOpensNothingButThePeerRoutes(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	savedOpen := openSetupPage
	t.Cleanup(func() { openSetupPage = savedOpen })
	openSetupPage = func(*Deck) error { return nil }
	key := pairedPeer(t, m)
	// Every route there is, setup's, the window's, the phone's and the hook's, and the grant record's to come.
	targets := []string{"/setup", "/setup/state", "/setup/peers", "/setup/pair", "/setup/unpair", "/setup/gm/team",
		"/setup/gm/approvals", "/setup/session", "/hook", "/launcher", "/raise", "/bots/x", "/grants", "/grants/answer",
		"/office?o=x", "/peer/nowhere"}
	for _, route := range everyPhoneRoute {
		targets = append(targets, route[1])
	}
	for _, target := range targets {
		for _, method := range []string{"GET", "POST"} {
			if method+" "+target == "GET /peer/hello" {
				continue
			}
			for _, to := range []string{"", "localhost:7710"} {
				from := otherMac
				if to != "" {
					from = thisComputer
				}
				got := peerAsk(m.home, method, target, key, from, to, "{}")
				if got.Code != http.StatusForbidden && got.Code != http.StatusNotFound && got.Code != http.StatusUnauthorized {
					t.Errorf("%s %s to %q with a peer key: %d %s", method, target, to, got.Code, got.Body)
				}
			}
		}
	}
	// Nor is it a phone's key, the window's, the setup key or the GM's token, wherever it's put.
	for _, c := range []call{
		{method: "GET", target: "/events?t=" + key, host: deckOnWiFi, from: aPhone},
		{method: "GET", target: "/events?t=" + key, host: "localhost:7710", from: thisComputer},
		{method: "GET", target: "/conversation?session=x&t=" + key, host: "localhost:7710", from: thisComputer},
		{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupKeyHeader: key}},
		{method: "GET", target: "/setup/gm/team", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{gmTokenHeader: key}},
	} {
		if got := serve(t, m.home, c); got.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with the peer key as another's: %d %s", c.method, c.target, got.Code, got.Body)
		}
	}
	if got := peerAsk(m.home, "GET", "/peer/hello", key, otherMac, "", ""); got.Code != http.StatusOK {
		t.Errorf("hello: %d %s", got.Code, got.Body)
	}
	// A phone's key and the window's are not a peer's.
	m.home.mu.Lock()
	phoneKey, windowKey := m.home.access.Phones.Pair("Probe", true), m.home.access.WindowKey
	m.home.mu.Unlock()
	for _, other := range []string{phoneKey, windowKey} {
		if got := peerAsk(m.home, "GET", "/peer/hello", other, otherMac, "", ""); got.Code != http.StatusUnauthorized {
			t.Errorf("hello with a phone's or the window's key: %d", got.Code)
		}
	}
}

func TestPlainWiFiIsRefusedForEveryPeerRoute(t *testing.T) {
	m := newTwoMacs(t, wifiMac, "192.168.1.10:0")
	key := pairedPeer(t, m)
	code := m.code(t)
	for _, target := range []string{"/peer/hello", peerPairPath, "/peer/nowhere"} {
		for _, method := range []string{"GET", "POST"} {
			for _, to := range []string{homeOnWiFi, "my-mac.local:7710", "localhost:7710"} {
				from := wifiMac
				if to == "localhost:7710" {
					// A request handed on by something on this computer from the Wi-Fi, without Tailscale's names.
					from = thisComputer
				}
				got := peerAsk(m.home, method, target, key, from, to, `{"code":"`+code+`"}`)
				if got.Code != http.StatusForbidden {
					t.Errorf("%s %s from the Wi-Fi to %s: %d %s", method, target, to, got.Code, got.Body)
				}
			}
		}
	}
	// Over the network too: the home's connections all come from the Wi-Fi.
	if status, _ := m.ask(t, "GET", "/peer/hello", key); status != http.StatusForbidden {
		t.Errorf("hello over the Wi-Fi: %d", status)
	}
	if got := m.join(t, m.address, code); got.Code != http.StatusForbidden {
		t.Errorf("joining over the Wi-Fi: %d %s", got.Code, got.Body)
	}
	// The joining one never sends a code to a Wi-Fi address.
	m.dialled = nil
	// Nor to a name or an IPv6 address, Tailscale's included: the home takes neither (fromAnotherMac).
	for _, address := range []string{homeOnWiFi, "192.168.1.10", "my-mac.local", "example.com", "[fe80::1]:7710", "100.128.0.1",
		"Studio.tail1234.ts.net", "studio.tail1234.ts.net:7710", "[fd7a:115c:a1e0::1]:7710"} {
		if got := m.join(t, address, code); got.Code != http.StatusBadRequest {
			t.Errorf("joining %s: %d %s", address, got.Code, got.Body)
		}
	}
	if len(m.dialled) != 0 {
		t.Errorf("dialled %v", m.dialled)
	}
	// The code wasn't spent by any of it: over Tailscale it still pairs.
	if got := peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+code+`"}`); got.Code != http.StatusOK {
		t.Errorf("the code over Tailscale after: %d %s", got.Code, got.Body)
	}
	for typed, want := range map[string]string{"100.64.1.2": "100.64.1.2:7710", "http://100.64.1.2:7751/": "100.64.1.2:7751",
		"[::ffff:100.64.1.2]:7710": "100.64.1.2:7710", "Studio.tail1234.ts.net": "", "[fd7a:115c:a1e0::1]:7710": ""} {
		if got := peerAddressOf(typed); got != want {
			t.Errorf("%q: %q, want %q", typed, got, want)
		}
	}
}

// Another Mac comes over WireGuard alone, straight to this one's Tailscale address. Never from loopback, whatever
// handed it on (`tailscale serve`, a proxy forwarding the Wi-Fi, a process here), with a proxy's headers or to a
// name: on a Mac and off it, with a valid key or the live code (RT126M1 B2).
func TestLoopbackOrAProxyIsRefusedEvenWithAKeyOrTheCode(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	key := pairedPeer(t, m)
	code := m.code(t)
	saved := macTailscale
	t.Cleanup(func() { macTailscale = saved })
	proxies := []map[string]string{
		{"X-Forwarded-For": "192.168.1.30"},
		{"Forwarded": "for=192.168.1.30"},
		{"X-Forwarded-Proto": "http"},
		{"X-Forwarded-Host": "studio.tail1234.ts.net"},
		{"Tailscale-User-Login": "someone@example.com"},
	}
	ask := func(what string, from, host, local string, headers map[string]string) {
		t.Helper()
		if got := peerAskVia(m.home, "GET", "/peer/hello", key, from, host, local, "", headers); got.Code != http.StatusForbidden {
			t.Errorf("%s: hello with a valid key: %d %s", what, got.Code, got.Body)
		}
		if got := peerAskVia(m.home, "POST", peerPairPath, "", from, host, local, `{"code":"`+code+`"}`, headers); got.Code != http.StatusForbidden {
			t.Errorf("%s: pair with the live code: %d %s", what, got.Code, got.Body)
		}
	}
	for _, mac := range []bool{true, false} {
		macTailscale = mac
		for _, host := range []string{"localhost:7710", "127.0.0.1:7710", "evil.example", "studio.tail1234.ts.net", "studio.tail1234.ts.net:443"} {
			// From loopback, through a proxy, or with no header at all.
			for _, headers := range append(proxies, nil) {
				ask(fmt.Sprintf("mac=%v from loopback to %s with %v", mac, host, headers), thisComputer, host, "127.0.0.1:7710", headers)
			}
		}
		// From the tailnet, straight to this one's address, but with a proxy's headers.
		for _, headers := range proxies {
			ask(fmt.Sprintf("mac=%v over WireGuard with %v", mac, headers), otherMac, tailHome+":7710", tailHome+":7710", headers)
		}
		// From the tailnet to this one's address, sent to a name: a ts.net one, or one a page made point here.
		for _, host := range []string{"studio.tail1234.ts.net:7710", "studio.tail1234.ts.net", "evil.example:7710", "my-mac.local:7710"} {
			ask(fmt.Sprintf("mac=%v over WireGuard to %s", mac, host), otherMac, host, tailHome+":7710", nil)
		}
		// From the tailnet to an address of this computer's that isn't its Tailscale one.
		ask(fmt.Sprintf("mac=%v over WireGuard to the Wi-Fi address", mac), otherMac, homeOnWiFi, homeOnWiFi, nil)
	}
	// None of it spent the code or a try: straight over WireGuard, it still pairs, and the key still works.
	macTailscale = saved
	if got := peerAsk(m.home, "GET", "/peer/hello", key, otherMac, "", ""); got.Code != http.StatusOK {
		t.Errorf("hello over WireGuard: %d %s", got.Code, got.Body)
	}
	m.home.mu.Lock()
	tries := m.home.macPeers.code.total
	m.home.mu.Unlock()
	if tries != 0 {
		t.Errorf("%d tries counted from refused requests", tries)
	}
	if got := peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+code+`"}`); got.Code != http.StatusOK {
		t.Errorf("the code over WireGuard after: %d %s", got.Code, got.Body)
	}
}

// A web page on a tailnet device can POST text or a form to /peer/pair without a preflight, never JSON: refused
// before the code is looked at, so it spends neither the code nor a try (RT126M1 N3).
func TestPairTakesJSONOnly(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	code := m.code(t)
	for _, kind := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		r := httptest.NewRequest("POST", peerPairPath, strings.NewReader(`{"code":"`+code+`"}`))
		r.Host, r.RemoteAddr = tailHome+":7710", otherMac
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, addrOf(tailHome+":7710")))
		if kind != "" {
			r.Header.Set("Content-Type", kind)
		}
		w := httptest.NewRecorder()
		(&router{deck: m.home}).ServeHTTP(w, r)
		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%q: %d %s", kind, w.Code, w.Body)
		}
	}
	m.home.mu.Lock()
	tries, paired := m.home.macPeers.code.total, len(m.home.macPeers.All)
	m.home.mu.Unlock()
	if tries != 0 || paired != 0 {
		t.Errorf("%d tries counted, %d Macs paired", tries, paired)
	}
	if got := peerAsk(m.home, "POST", peerPairPath, "", otherMac, "", `{"code":"`+code+`"}`); got.Code != http.StatusOK {
		t.Errorf("the code as JSON after: %d %s", got.Code, got.Body)
	}
}

func TestFloorsOffRefusesEverything(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	key := pairedPeer(t, m)
	code := m.code(t)
	if got := peerPage(t, m.home, "POST", "/setup/peers?enabled=0", ""); got.Code != http.StatusOK || strings.Contains(got.Body.String(), code) {
		t.Fatalf("turning it off: %d %s", got.Code, got.Body)
	}
	for _, target := range []string{"/peer/hello", peerPairPath, "/peer/nowhere"} {
		for _, method := range []string{"GET", "POST"} {
			if got := peerAsk(m.home, method, target, key, otherMac, "", `{"code":"`+code+`"}`); got.Code != http.StatusForbidden {
				t.Errorf("%s %s with it off: %d %s", method, target, got.Code, got.Body)
			}
		}
	}
	if got := peerPage(t, m.home, "POST", "/setup/peers/code", ""); got.Code != http.StatusConflict {
		t.Errorf("a code with it off: %d %s", got.Code, got.Body)
	}
	// Off on the joining one: nothing is sent.
	peerPage(t, m.joiner, "POST", "/setup/peers?enabled=0", "")
	m.dialled = nil
	if got := m.join(t, m.address, code); got.Code != http.StatusConflict || len(m.dialled) != 0 {
		t.Errorf("joining with it off: %d %s, dialled %v", got.Code, got.Body, m.dialled)
	}
	// On again, the Mac paired goes on: it was never removed.
	peerPage(t, m.home, "POST", "/setup/peers?enabled=1", "")
	if got := peerAsk(m.home, "GET", "/peer/hello", key, otherMac, "", ""); got.Code != http.StatusOK {
		t.Errorf("hello on again: %d", got.Code)
	}
}

func TestFloorsAreOffUntilThePageTurnsThemOn(t *testing.T) {
	d := newTestDeck(t)
	if got := peerPage(t, d, "GET", "/setup/peers", ""); !strings.Contains(got.Body.String(), `"on":false`) {
		t.Errorf("a new deck: %s", got.Body)
	}
	d.mu.Lock()
	d.gmToken = "fake-gm-token-for-floors"
	d.mu.Unlock()
	for _, target := range []string{"/setup/peers?enabled=1", "/setup/peers/code", "/setup/peers/join", "/setup/peers/revoke?peer=x"} {
		for _, c := range []call{
			{asTool: true},
			{withoutSetupKey: true, headers: map[string]string{gmTokenHeader: "fake-gm-token-for-floors"}},
		} {
			c.method, c.target, c.host, c.from = "POST", target, "localhost:7710", thisComputer
			c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
			if got := serve(t, d, c); got.Code != http.StatusForbidden {
				t.Errorf("%s by a tool or the GM: %d %s", target, got.Code, got.Body)
			}
		}
	}
	if got := serve(t, d, call{method: "GET", target: "/setup/peers", host: "localhost:7710", from: thisComputer, asTool: true}); got.Code != http.StatusForbidden {
		t.Errorf("the list by a tool: %d", got.Code)
	}
	d.mu.Lock()
	on := d.config.FloorsFromOtherMacs
	d.mu.Unlock()
	if on {
		t.Error("turned on by someone other than the page")
	}
	// Nor from a phone or the Wi-Fi: setup is this computer's.
	if got := serve(t, d, call{method: "POST", target: "/setup/peers?enabled=1", host: deckOnWiFi, from: aPhone,
		headers: map[string]string{setupHeader: "1"}}); got.Code != http.StatusNotFound {
		t.Errorf("from the Wi-Fi: %d", got.Code)
	}
}

func TestAJoinWithoutASecretStoreSendsNothing(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	m.joiner.mu.Lock()
	m.joiner.peerKeys = noPeerKeyStore
	m.joiner.mu.Unlock()
	code := m.code(t)
	m.dialled = nil
	if got := m.join(t, m.address, code); got.Code != http.StatusServiceUnavailable || len(m.dialled) != 0 {
		t.Errorf("joining with no store: %d %s, dialled %v", got.Code, got.Body, m.dialled)
	}
	m.home.mu.Lock()
	still := m.home.macPeers.code.active()
	m.home.mu.Unlock()
	if still != code {
		t.Error("the code was spent")
	}
}

func TestThePeerKeyIsKeptHashedAndNoSecretCrosses(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	// Every secret of both Macs: the setup key, the hook's, the window's, the GM's token and a phone's.
	var secrets []string
	for i, d := range []*Deck{m.home, m.joiner} {
		d.mu.Lock()
		d.gmToken = fmt.Sprintf("fake-gm-token-%d", i)
		phone := d.access.Phones.Pair("Probe", true)
		secrets = append(secrets, d.access.SetupKey, d.access.HookKey, d.access.WindowKey, d.gmToken, phone)
		d.mu.Unlock()
	}
	code := m.code(t)
	if got := m.join(t, m.address, code); got.Code != http.StatusOK {
		t.Fatalf("joining: %d %s", got.Code, got.Body)
	}
	key := m.key(t)
	crossed := m.seen.both()
	for _, secret := range secrets {
		if secret == "" || strings.Contains(crossed, secret) {
			t.Errorf("a secret crossed (or is empty): %q", secret)
		}
	}
	// The request held the code and a name, nothing else; the answer, the new key and the home's name.
	m.seen.mu.Lock()
	request, answer := m.seen.in.String(), m.seen.out.String()
	m.seen.mu.Unlock()
	_, body, _ := strings.Cut(request, "\r\n\r\n")
	var sent map[string]any
	if err := json.Unmarshal([]byte(body), &sent); err != nil || len(sent) != 2 || sent["code"] != code || sent["name"] != "Laptop" {
		t.Errorf("sent %q", body)
	}
	for _, header := range []string{peerKeyHeader, hookKeyHeader, setupKeyHeader, gmTokenHeader, "Cookie", "Authorization"} {
		if strings.Contains(strings.ToLower(request), strings.ToLower(header)+":") {
			t.Errorf("the request carried %s", header)
		}
	}
	_, body, _ = strings.Cut(answer, "\r\n\r\n")
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil || len(got) != 2 || got["key"] != key || got["name"] != "Studio" {
		t.Errorf("answered %q", body)
	}
	// Kept hashed at home, as phones are, and nowhere in a file on either Mac.
	data, err := os.ReadFile(m.home.macPeers.file)
	if err != nil || !strings.Contains(string(data), hashKey(key)) {
		t.Errorf("mac-peers.json: %v %s", err, data)
	}
	for _, folder := range []string{filepath.Dir(m.home.macPeers.file), filepath.Dir(m.joiner.macPeers.file)} {
		_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if data, err := os.ReadFile(path); err == nil && bytes.Contains(data, []byte(key)) {
				t.Errorf("the key is in %s", path)
			}
			return nil
		})
	}
	if info, err := os.Stat(m.home.macPeers.file); err == nil && !modeIs(info, 0o600) {
		t.Errorf("mac-peers.json is %v", info.Mode())
	}
}

// A peer row outside /peer/, or another class's inside it, stops the start.
func TestPeerRowsAreUnderPeerOnly(t *testing.T) {
	for _, row := range []phoneRoute{
		{method: "GET", path: "/test-peer-outside", access: accessPeer},
		{method: "GET", path: "/peer/test-phone-inside", access: accessPhone},
	} {
		row.refusal, row.serve = says("no"), func(*Deck, http.ResponseWriter, *request, string) {}
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s %s registered", row.method, row.path)
					delete(phoneRouteTable, row.method+" "+row.path)
				}
			}()
			registerPhoneRoute(row)
		}()
	}
	var peerRows []string
	for key, row := range phoneRouteTable {
		if row.access == accessPeer {
			peerRows = append(peerRows, key)
		}
	}
	if !slices.Equal(peerRows, []string{"GET /peer/hello"}) {
		t.Errorf("peer rows: %v", peerRows)
	}
}

// FL0: under the Mac app, the Macs this one joined and their keys go in its own item, peer-keys, over the bridge, and
// in no file of the support folder. An item there that the app won't read (another program changed it, so its dates
// moved or its seal doesn't fit, or the Keychain can't be read now) is never taken for none: the join is refused
// before the code is sent, and the list kept isn't written over with only the next Mac.
func TestUnderTheMacAppTheMacsJoinedAreTheAppsItem(t *testing.T) {
	m := newTwoMacs(t, otherMac, tailHome+":0")
	mac := &macStandIn{}
	m.joiner.mu.Lock()
	m.joiner.peerKeys = peerKeyStoreFor(mac)
	m.joiner.mu.Unlock()
	if got := m.join(t, m.address, m.code(t)); got.Code != http.StatusOK {
		t.Fatalf("joining: %d %s", got.Code, got.Body)
	}
	mac.mu.Lock()
	kept := mac.secrets[peerKeysItem]
	mac.mu.Unlock()
	var joined []joinedMac
	if err := json.Unmarshal([]byte(kept), &joined); err != nil || len(joined) != 1 || joined[0].Key == "" || joined[0].Address != m.address {
		t.Fatalf("the app's item holds %q", kept)
	}
	_ = filepath.WalkDir(supportDir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(joined[0].Key)) {
				t.Errorf("%s holds the key", path)
			}
		}
		return nil
	})

	mac.mu.Lock()
	mac.unreadable = []string{peerKeysItem}
	mac.mu.Unlock()
	code := m.code(t)
	m.dialled = nil
	if got := m.join(t, m.address, code); got.Code != http.StatusServiceUnavailable || len(m.dialled) != 0 {
		t.Errorf("joining with the item unreadable: %d %s, dialled %v", got.Code, got.Body, m.dialled)
	}
	mac.mu.Lock()
	after := mac.secrets[peerKeysItem]
	mac.mu.Unlock()
	if after != kept {
		t.Errorf("the list was written over: %q", after)
	}
	if state := m.joiner.peersState(); state["storeReadable"] != false || len(state["joined"].([]map[string]any)) != 0 {
		t.Errorf("setup says %v", state)
	}
}
