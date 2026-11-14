package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The other side of floors (peers_mac.go): this Mac joining another's office with the code it shows, and setup's
// routes for both sides, the page's alone (goOnlyRoutes). The key this Mac is given is kept in its secret store,
// with the Macs it joined: under the Mac app its Keychain item (peer-keys, through the bridge), elsewhere Windows
// Credential Manager or the Secret Service. Never a file: with no store, joining is refused before the code is sent.

// peerKeysItem is the Macs joined, keys and all, among the Mac app's own items (bridgeSecrets).
const peerKeysItem = "peer-keys"

// joinedMac is a Mac this one joined, as its secret store keeps it, its key with it.
type joinedMac struct {
	Address  string    `json:"address"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joinedAt"`
	Key      string    `json:"key"`
}

// peerKeyStore is where the Macs joined are kept, as one item: read gives "" with no error when there is none.
type peerKeyStore struct {
	read  func() (string, error)
	write func(string) error
}

// noPeerKeyStore keeps nothing, and says so: a Mac without the app, where this program runs only as tests, and
// every test, which never reaches a real store.
var noPeerKeyStore = peerKeyStore{
	read:  func() (string, error) { return "", errNoSecretStore },
	write: func(string) error { return errNoSecretStore },
}

// peerKeyStoreFor is the store under the Mac app h, or this system's own without it.
func peerKeyStoreFor(h hostBridge) peerKeyStore {
	if h == nil {
		return systemPeerKeyStore
	}
	return peerKeyStore{
		read: func() (string, error) {
			value, _, err := hostReadSecret(h, peerKeysItem)
			return value, err
		},
		write: func(value string) error { return hostWriteSecret(h, peerKeysItem, value) },
	}
}

// readJoined is the Macs joined, from the store; an error when it can't be read.
func readJoined(store peerKeyStore) ([]joinedMac, error) {
	saved, err := store.read()
	if err != nil {
		return nil, err
	}
	var joined []joinedMac
	if strings.TrimSpace(saved) != "" {
		if err := json.Unmarshal([]byte(saved), &joined); err != nil {
			return nil, err
		}
	}
	return joined, nil
}

// peerAddressOf is the other Mac's address as typed, made host:port, if it's its Tailscale address: IPv4, in
// 100.64/10. Anything else is "": a Wi-Fi address above all, and also a ts.net name or an IPv6 address, which the
// other one refuses (fromAnotherMac), so the code is never sent where it can only fail.
func peerAddressOf(typed string) string {
	text := strings.TrimSpace(typed)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "http://"), "https://")
	text = strings.TrimSuffix(text, "/")
	host, port, err := net.SplitHostPort(text)
	if err != nil {
		host, port = strings.Trim(text, "[]"), strconv.Itoa(Port)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return ""
	}
	ip := net.ParseIP(host).To4()
	if ip == nil || !tailnet.Contains(ip) {
		return ""
	}
	return net.JoinHostPort(ip.String(), port)
}

// onTailnet is whether ip is one of Tailscale's.
func onTailnet(ip net.IP) bool {
	if four := ip.To4(); four != nil {
		return tailnet.Contains(four)
	}
	return tailnet6.Contains(ip)
}

var errNotTailnet = errors.New("not a Tailscale address")

// dialPeer connects to the other Mac at address, host:port, only where its name leads to a Tailscale address:
// WireGuard carries it, never the Wi-Fi. A variable, so a test can lead it to a deck of its own.
var dialPeer = func(ctx context.Context, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if onTailnet(ip) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
		}
	}
	return nil, errNotTailnet
}

// peerClient asks the other Mac: through dialPeer, through no proxy, following no redirect (a key goes nowhere
// else), and for a short while.
var peerClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
			return dialPeer(ctx, address)
		},
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// The most an answer from the other Mac may hold.
const peerAnswerMostBytes = 64 << 10

// joinPeer sends the code to the Mac at address and keeps the key it gives in store, with the Macs already joined.
// It never holds d.mu: the other Mac and the store may both take a while.
func joinPeer(store peerKeyStore, address, code, name string) (joinedMac, int, string) {
	joined, err := readJoined(store)
	if err != nil {
		return joinedMac{}, http.StatusServiceUnavailable, "This computer's secret store can't keep the key, so nothing was sent."
	}
	body, _ := json.Marshal(map[string]string{"code": code, "name": name})
	request, err := http.NewRequest(http.MethodPost, (&url.URL{Scheme: "http", Host: address, Path: peerPairPath}).String(), bytes.NewReader(body))
	if err != nil {
		return joinedMac{}, http.StatusBadRequest, "That isn't an address."
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := peerClient.Do(request)
	if err != nil {
		return joinedMac{}, http.StatusBadGateway, "The other one couldn't be reached over Tailscale."
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, peerAnswerMostBytes))
	if response.StatusCode != http.StatusOK {
		said := strings.TrimSpace(string(data))
		if said == "" || len(said) > 200 {
			said = "The other one said no."
		}
		return joinedMac{}, response.StatusCode, said
	}
	var answer struct{ Key, Name string }
	if json.Unmarshal(data, &answer) != nil || answer.Key == "" {
		return joinedMac{}, http.StatusBadGateway, "The other one's answer had no key."
	}
	mac := joinedMac{Address: address, Name: peerName(answer.Name), JoinedAt: time.Now().UTC().Truncate(time.Second), Key: answer.Key}
	kept := []joinedMac{mac}
	for _, other := range joined {
		if other.Address != address {
			kept = append(kept, other)
		}
	}
	saved, _ := json.Marshal(kept)
	if err := store.write(string(saved)); err != nil {
		return joinedMac{}, http.StatusServiceUnavailable, "Paired, but the key couldn't be kept here: remove this one from the other one's list, and try again."
	}
	return mac, http.StatusOK, ""
}

// peersState is what setup's Floors from other Macs shows: the setting, the code while there is one, the Macs
// paired here and those this one joined. Never a key.
func (d *Deck) peersState() map[string]any {
	d.mu.Lock()
	on := d.config.FloorsFromOtherMacs
	code := d.macPeers.code.active()
	expiresIn := 0
	if code != "" {
		expiresIn = int(time.Until(d.macPeers.code.expiry).Seconds())
	}
	peers := []map[string]any{}
	for _, peer := range d.macPeers.All {
		peers = append(peers, map[string]any{"id": peer.ID, "name": peer.Name, "pairedAt": peer.PairedAt, "lastSeen": peer.LastSeen})
	}
	store := d.peerKeys
	d.mu.Unlock()
	joinedList, err := readJoined(store)
	joined := []map[string]any{}
	for _, mac := range joinedList {
		joined = append(joined, map[string]any{"address": mac.Address, "name": mac.Name, "joinedAt": mac.JoinedAt})
	}
	return map[string]any{"on": on, "code": code, "expiresIn": expiresIn, "peers": peers, "joined": joined,
		"storeReadable": err == nil}
}

// peerSetupRoutes are setup's routes for floors from other Macs, the page's alone (goOnlyRoutes): the setting, a
// code, removing a Mac paired here, and joining another. It says whether it answered.
func (rt *router) peerSetupRoutes(w http.ResponseWriter, r *request) bool {
	d := rt.deck
	if r.URL.Path != "/setup/peers" && !strings.HasPrefix(r.URL.Path, "/setup/peers/") {
		return false
	}
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/setup/peers":
	case r.Method == http.MethodPost && r.URL.Path == "/setup/peers":
		on := query.Get("enabled") == "1"
		if !on && query.Get("enabled") != "0" {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		if d.refusedConfigWhileHeld(w, func(c *Config) { c.FloorsFromOtherMacs = on }) {
			return true
		}
		d.mu.Lock()
		d.config.FloorsFromOtherMacs = on
		d.config.save()
		if !on {
			d.macPeers.code.withdraw()
		}
		d.mu.Unlock()
	case r.Method == http.MethodPost && r.URL.Path == "/setup/peers/code":
		// No code while the records can't take a Mac (records.go): it would be kept unsealed, and dropped at the next
		// start with every Mac paired before (RT126SK1R3 N3), as a phone isn't paired then either.
		refused := recordsRefusal()
		d.mu.Lock()
		on := d.config.FloorsFromOtherMacs
		if on && refused == "" {
			d.macPeers.code.renew()
		}
		d.mu.Unlock()
		if !on {
			writeStatus(w)(http.StatusConflict, d.inWords(peersOffRefusal))
			return true
		}
		if refused != "" {
			writeStatus(w)(http.StatusConflict, refused)
			return true
		}
	case r.Method == http.MethodPost && r.URL.Path == "/setup/peers/revoke":
		if refusedWhileHeld(w) {
			return true
		}
		d.mu.Lock()
		gone := d.macPeers.remove(query.Get("peer"))
		d.mu.Unlock()
		if !gone {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
	case r.Method == http.MethodPost && r.URL.Path == "/setup/peers/join":
		var asked struct{ Address, Code, Name string }
		if json.NewDecoder(io.LimitReader(r.Body, peerPairMostBytes)).Decode(&asked) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		d.mu.Lock()
		on, store := d.config.FloorsFromOtherMacs, d.peerKeys
		d.mu.Unlock()
		if !on {
			writeStatus(w)(http.StatusConflict, d.inWords(peersOffRefusal))
			return true
		}
		address := peerAddressOf(asked.Address)
		if address == "" {
			writeStatus(w)(http.StatusBadRequest, d.inWords(peerAddressRefusal))
			return true
		}
		code := strings.Map(func(c rune) rune {
			if c >= '0' && c <= '9' {
				return c
			}
			return -1
		}, asked.Code)
		if len(code) != 6 {
			writeStatus(w)(http.StatusBadRequest, d.inWords(peerCodeRefusal))
			return true
		}
		name := asked.Name
		if strings.TrimSpace(name) == "" {
			name = peerMachineName()
		}
		d.joinMu.Lock()
		_, status, refusal := joinPeer(store, address, code, peerName(name))
		d.joinMu.Unlock()
		if status != http.StatusOK {
			writeStatus(w)(status, refusal)
			return true
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	writeJSON(w, http.StatusOK, d.peersState())
	return true
}
