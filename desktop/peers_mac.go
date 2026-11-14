package desktop

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Floors from other Macs (1.26, M1): another Mac (or computer) of yours joins this one's office, as a phone pairs,
// with a code. Off unless you turn it on, on each of the two; only over Tailscale, never the Wi-Fi's plain http.
//
// This one, the home, shows a 6-digit code (POST /setup/peers/code), with the phone's code's limits: five minutes,
// once, five wrong tries an address, twenty in all, and a try a second an address. The other one enters it (POST
// /setup/peers/join, peers_mac_join.go), which sends it here (POST /peer/pair) and gets a key of its own, a peer key:
// kept here only as its hash, as a phone's is, in mac-peers.json, and there in its secret store. With it, the other
// one may use the routes of the `peer` class (phoneroutes.go), under /peer/, and nothing else: no phone's or the
// window's route, no setup, no hook. This piece serves only what pairing needs (GET /peer/hello); what a floor
// reads is the next one's. Setup lists the Macs paired, and removing one refuses its very next request.
//
// What crosses: the code and a name one way, the new key and this one's name the other. Never the setup key, the
// GM's token, the window's key, the hook's or a phone's.

// peerKeyHeader carries a peer's key, never in the address, so it's in no log of one.
const peerKeyHeader = "X-Smolcorp-Peer"

// peerPairPath is where the other Mac sends the code; the only /peer/ route without a key.
const peerPairPath = "/peer/pair"

// peerCodeTryGap is how soon one address may try a code again: a guess a second at most, besides the counts.
var peerCodeTryGap = time.Second

// The most a code's request may hold: a code and a name.
const peerPairMostBytes = 4 << 10

// MacPeer is another Mac paired here, as mac-peers.json keeps it.
type MacPeer struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PairedAt time.Time `json:"pairedAt"`
	LastSeen time.Time `json:"lastSeen"`
	// SHA-256 of its key, in hex. The key itself is never kept here.
	KeyHash string `json:"keyHash"`
}

// macPeers is the Macs paired here, and the code another may join with. Read and changed under d.mu.
type macPeers struct {
	file string
	All  []MacPeer
	code peerCode
}

// macPeersFile is the list's file in the support folder, taken as the deck starts.
func macPeersFile() string { return filepath.Join(supportDir, "mac-peers.json") }

// loadMacPeers is the Macs paired here. The list is a sealed record (records.go): on a Mac, one whose seal doesn't
// fit is left out whole and said, and at the one-time change of every key (B1) every Mac pairs again.
func loadMacPeers(file string) *macPeers {
	p := &macPeers{file: file}
	if data, err := os.ReadFile(file); err == nil && !recordsRotated() {
		body, counts := openBody(filepath.Base(file), data)
		if counts {
			_ = json.Unmarshal(body, &p.All)
		} else {
			recordDropped(filepath.Base(file), "mac-peers.json was changed by another program, or its seal doesn't fit: no Mac it lists is let in. Pair them again.")
		}
	}
	return p
}

// pair adds a Mac and gives its key: the only time the key exists here. None while the records can't keep it
// (access.go's keepable, RT126SK1R3 N3): then it gives "", and nothing is paired.
func (p *macPeers) pair(name string) string {
	if !keepable() {
		return ""
	}
	key := randomKey()
	now := time.Now().UTC().Truncate(time.Second)
	p.All = append(p.All, MacPeer{ID: newID(), Name: name, PairedAt: now, LastSeen: now, KeyHash: hashKey(key)})
	p.save()
	return key
}

// withKey is the Mac holding key, if any.
func (p *macPeers) withKey(key string) *MacPeer {
	if key == "" {
		return nil
	}
	hash := hashKey(key)
	for i := range p.All {
		if constantTimeEqual(p.All[i].KeyHash, hash) {
			return &p.All[i]
		}
	}
	return nil
}

func (p *macPeers) markSeen(peer *MacPeer) {
	if time.Since(peer.LastSeen) < seenResolution {
		return
	}
	peer.LastSeen = time.Now().UTC().Truncate(time.Second)
	p.save()
}

// remove takes a Mac off the list, and says whether it was on it.
func (p *macPeers) remove(id string) bool {
	kept := p.All[:0]
	for _, peer := range p.All {
		if peer.ID != id {
			kept = append(kept, peer)
		}
	}
	gone := len(kept) < len(p.All)
	p.All = kept
	// Under a new record key when one went: a copy of the list from before no longer fits (records.go).
	_ = keepRecord("mac-peers.json", p.writer(), gone)
	return gone
}

func (p *macPeers) save() { _ = keepRecord("mac-peers.json", p.writer(), false) }

// writer writes the list as it is now (records.go's recordWriter).
func (p *macPeers) writer() recordWriter {
	list, file := slices.Clone(p.All), p.file
	if list == nil {
		list = []MacPeer{}
	}
	return func(key []byte, prev string) error {
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}
		return writePrivate(file, sealBody(key, prev, filepath.Base(file), data))
	}
}

// peerCode is the code another Mac joins with: Access's code (access.go), with its limits, and one of its own, so
// a phone's code never pairs a Mac, nor a Mac's a phone.
type peerCode struct {
	code    string
	expiry  time.Time
	misses  map[string]int
	total   int
	lastTry map[string]time.Time
}

// renew replaces any earlier code and lifts the lockouts.
func (c *peerCode) renew() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	c.code = fmt.Sprintf("%06d", n.Int64())
	c.expiry = time.Now().Add(codeLifetime)
	c.misses, c.total, c.lastTry = map[string]int{}, 0, map[string]time.Time{}
	return c.code
}

// active is the code, or "" when none was asked for, it ran out or was used.
func (c *peerCode) active() string {
	if c.code == "" || time.Now().After(c.expiry) {
		return ""
	}
	return c.code
}

func (c *peerCode) withdraw() { c.code = "" }

// try says whether attempt is the code, from address; the code is spent when it is. tooSoon: the address tried
// less than peerCodeTryGap ago, which isn't counted as a try.
func (c *peerCode) try(attempt, address string) (ok, tooSoon bool) {
	active := c.active()
	if active == "" || c.misses[address] >= maxAttemptsPerAddress {
		return false, false
	}
	now := time.Now()
	if last, tried := c.lastTry[address]; tried && now.Sub(last) < peerCodeTryGap {
		return false, true
	}
	c.lastTry[address] = now
	if constantTimeEqual(attempt, active) {
		c.code = ""
		return true, false
	}
	c.misses[address]++
	c.total++
	if c.total >= maxAttempts {
		c.code = ""
	}
	return false, false
}

// What a peer is told, %s being "Mac" under the Mac app, else "computer" (d.machine).
const (
	peersOffRefusal     = "Floors from other %ss is off here."
	peersTailnetRefusal = "Another %s joins over Tailscale only, never the Wi-Fi."
	peerAddressRefusal  = "Enter the other %s's Tailscale address, as 100.x.y.z: not a name, nor a Wi-Fi address."
	peerRoutesRefusal   = "A %s paired for floors can't use this."
	peerOnlyRefusal     = "Only another %s paired for floors uses this."
	peerCodeRefusal     = "That code isn't valid. Get a new one on the other %s."
	peerCodeTooSoon     = "Too many tries at once. Wait a moment."
)

// inWords is a refusal with this computer's word in it.
func (d *Deck) inWords(refusal string) string { return fmt.Sprintf(refusal, d.machine()) }

func init() {
	// The other Mac checks its new key works, and learns this one's name.
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/peer/hello", access: accessPeer,
		refusal: func(d *Deck) string { return d.inWords(peerOnlyRefusal) },
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			writeJSON(w, http.StatusOK, map[string]any{"name": peerMachineName()})
		}})
}

// servePeer answers anything under /peer/, and any request carrying a peer's key, wherever it's sent: another Mac
// is let in here alone, to the peer class's rows, never as a phone, the window, setup or the hook. It says whether
// it answered. Asked first, before the router's other checks.
func (d *Deck) servePeer(w http.ResponseWriter, r *request) bool {
	key := r.Header.Get(peerKeyHeader)
	if key == "" && !strings.HasPrefix(r.URL.Path, "/peer/") {
		return false
	}
	answer := writeStatus(w)
	row := phoneRouteFor(r.Method, r.URL.Path)
	isPair := r.Method == http.MethodPost && r.URL.Path == peerPairPath
	if key == "" && !isPair {
		if row != nil && row.access == accessPeer {
			answer(http.StatusForbidden, row.refusal(d))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		return true
	}
	d.mu.Lock()
	on := d.config.FloorsFromOtherMacs
	d.mu.Unlock()
	if !on {
		answer(http.StatusForbidden, d.inWords(peersOffRefusal))
		return true
	}
	// Over WireGuard alone, straight to this computer's Tailscale address: never the Wi-Fi, loopback or a proxy.
	if !fromAnotherMac(r.Request) {
		answer(http.StatusForbidden, d.inWords(peersTailnetRefusal))
		return true
	}
	if isPair {
		d.peerPair(w, r)
		return true
	}
	d.mu.Lock()
	peer := d.macPeers.withKey(key)
	if peer != nil {
		d.macPeers.markSeen(peer)
	}
	d.mu.Unlock()
	if peer == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}
	if row == nil || row.access != accessPeer {
		answer(http.StatusForbidden, d.inWords(peerRoutesRefusal))
		return true
	}
	row.serve(d, w, r, r.URL.Query().Get("session"))
	return true
}

// fromAnotherMac says whether a request came the one way another Mac's does: dialled by dialPeer from another
// device's Tailscale address straight to this computer's, so WireGuard carried it, and sent to that address, not a
// name. Never from loopback, whatever handed it on there (`tailscale serve`, a proxy forwarding the Wi-Fi, a hire),
// never with a proxy's headers, never to a ts.net name, never from one of this computer's own addresses (SN1's
// own-address door): on every system, even with a key or the code. Not
// fromTailnet, which lets a phone in through `tailscale serve` as well; a Mac never comes that way.
func fromAnotherMac(r *http.Request) bool {
	if isProxied(r.Header) {
		return false
	}
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	from := net.ParseIP(remote)
	local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if from == nil || from.To4() == nil || !tailnet.Contains(from.To4()) || thisComputersAddress(from) || local == nil {
		return false
	}
	address, _, _ := net.SplitHostPort(local.String())
	here := net.ParseIP(address)
	sentTo, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		sentTo = r.Host
	}
	if to := net.ParseIP(sentTo); here == nil || to == nil || !to.Equal(here) {
		return false
	}
	return slices.ContainsFunc(tailscaleAddresses(), func(own string) bool { return here.Equal(net.ParseIP(own)) })
}

// peerPair is POST /peer/pair: the code, from another Mac, swapped for its key. Let in by servePeer: on, and over
// Tailscale.
func (d *Deck) peerPair(w http.ResponseWriter, r *request) {
	// JSON only, as joinPeer sends it: a web page on a tailnet device can't send that without a preflight, which
	// nothing here answers, so it can't spend the code or its tries with a form's or text's POST.
	if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); media != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	var asked struct{ Code, Name string }
	if json.NewDecoder(io.LimitReader(r.Body, peerPairMostBytes)).Decode(&asked) != nil {
		writeStatus(w)(http.StatusBadRequest, d.inWords(peerCodeRefusal))
		return
	}
	d.mu.Lock()
	ok, tooSoon := d.macPeers.code.try(strings.TrimSpace(asked.Code), r.remoteAddress)
	key := ""
	if ok {
		key = d.macPeers.pair(peerName(asked.Name))
	}
	d.mu.Unlock()
	switch {
	case tooSoon:
		writeStatus(w)(http.StatusTooManyRequests, peerCodeTooSoon)
	case !ok, key == "":
		writeStatus(w)(http.StatusForbidden, d.inWords(peerCodeRefusal))
	default:
		writeJSON(w, http.StatusOK, map[string]any{"key": key, "name": peerMachineName()})
	}
}

// peerName is what setup calls a Mac paired here: the name it sent, one line of 40 letters at most, else "Mac" (or
// "Computer" off the Mac).
func peerName(sent string) string {
	name := strings.TrimSpace(strings.Map(func(c rune) rune {
		if unicode.IsControl(c) || !unicode.IsPrint(c) {
			return -1
		}
		return c
	}, sent))
	if runes := []rune(name); len(runes) > 40 {
		name = strings.TrimSpace(string(runes[:40]))
	}
	if name == "" {
		word := machineHere()
		return strings.ToUpper(word[:1]) + word[1:]
	}
	return name
}

// peerMachineName is this computer's name, as the other Mac lists it: its host name without .local.
var peerMachineName = func() string {
	name, _ := os.Hostname()
	return peerName(strings.TrimSuffix(name, ".local"))
}
