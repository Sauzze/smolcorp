package desktop

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Access decides who may use the deck: the phones you paired, and the agents'
// hook. It mirrors the Mac app's Access and Phones.
//
// Each phone has a key of its own, handed over once when it pairs. To pair,
// you ask for a code; the phone opens the deck's address and enters it, or
// scans it, and gets a new key in return. A code exists only after you ask
// for one, lasts five minutes and works once. Five wrong tries lock out the
// address they came from until the next code, and twenty in all withdraw the
// code.
//
// The hook has a separate key, in a file only you can read. It opens /hook,
// from this computer, and nothing else; no phone's key opens /hook.
type Access struct {
	Phones  *Phones
	HookKey string
	// SetupKey is a tool's key to setup, new at every start and kept in this
	// process's memory only: no file holds it (SK2). The core's own demo
	// office and a terminal you allowed (toolKeys) are the only tools.
	SetupKey string
	// SelftestKey is the check-only key, new at every start, in a file only
	// you can read: it opens GET /setup/selftest and nothing else (selftestkey.go).
	SelftestKey string
	// toolKeys are the keys handed to a terminal you allowed (terminalask.go):
	// each opens what the setup key opens, from that process's memory only,
	// until this app stops.
	toolKeys []string
	// Tokens the setup links carry, each good once, until it runs out. The
	// key itself is never in a link.
	setupTokens map[string]time.Time
	// The tokens already used, with the session each opened, until they'd
	// have run out: a second use is a link another program used first
	// (setuplink.go).
	usedSetupTokens map[string]usedSetupToken
	// The setup page's sessions: random ids, kept in memory only, each opened
	// by a token or by Smolcorp's own window. A cookie holds one of these,
	// never the setup key.
	setupSessions []setupSession
	// Sessions ended because their link was used again, and when: their cookie is told why for a while.
	endedSessions map[string]time.Time
	// usedFirst: a link was used first, kept until the owner's own channel clears it (setuplink.go).
	usedFirst *usedFirstReport
	// WindowKey is the key the Mac app's office window reads the office with,
	// handed to the app alone (core_darwin.go's office op): new at each start,
	// never saved, taken only from this computer, and never in a browser's
	// address. Under the Mac app it is the one window whose tap is the owner's.
	WindowKey string
	// windowDeviceKey is the key of the Windows office window's own paired
	// device (window.go's ensureWindowKey): in memory only, new at each start.
	windowDeviceKey string
	// BrowserKey is the key the office in a browser on this computer reads
	// the office with: the tray's Show Office carries a token of officeTokens,
	// which the deck swaps for it. It reads the office as the window does, but
	// a browser keeps it in its history, a file any program here can read, so
	// it never answers the owner's asks (RT126FS2b N1).
	BrowserKey   string
	officeTokens map[string]time.Time

	code       string
	codeExpiry time.Time
	// The key the pairing QR code carries, in its fragment, for the phone that
	// scans it: made with the code, it lasts and works as long as the code, and
	// it never crosses the network (see Claim).
	pairKey string
	// The nonces of signed requests, until their time runs out (signed.go).
	nonces map[string]time.Time
	// When this app started: the nonces before it are gone, so nothing signed earlier is taken (signed.go).
	started     time.Time
	misses      map[string]int // wrong codes since the current one, by address
	totalMisses int
}

const (
	codeLifetime = 5 * time.Minute
	// Wrong codes one address may try before it is locked out.
	maxAttemptsPerAddress = 5
	// Wrong codes from everywhere before the code is withdrawn: the backstop
	// for a device that keeps changing its address.
	maxAttempts = 20
)

func NewAccess() *Access {
	phones := loadPhones()
	// The office window's devices from the run before go at once: only this run's window key opens it (window.go).
	dropWindowDevices(phones)
	return &Access{Phones: phones, HookKey: loadHookKey(), SetupKey: newSetupKey(), SelftestKey: newSelftestKey(),
		WindowKey: randomKey(), BrowserKey: randomKey(), misses: map[string]int{}, started: processStarted}
}

func (a *Access) IsHookKey(candidate string) bool {
	return candidate != "" && constantTimeEqual(candidate, a.HookKey)
}

// IsSetupKey is whether candidate is the setup key, or a key a terminal you allowed holds.
func (a *Access) IsSetupKey(candidate string) bool {
	if candidate == "" {
		return false
	}
	found := constantTimeEqual(candidate, a.SetupKey)
	for _, key := range a.toolKeys {
		found = constantTimeEqual(candidate, key) || found
	}
	return found
}

// How long a setup link's token works. Off a Mac the browser is started with
// the link on its command line, which any program of yours can read: a minute,
// enough for a browser to start, keeps that race short, and setuplink.go makes
// it seen. Under the Mac app, Settings opens with no command line.
var setupTokenLifetime = setupLinkLifetime(runtime.GOOS)

func setupLinkLifetime(goos string) time.Duration {
	if goos == "darwin" {
		return 5 * time.Minute
	}
	return time.Minute
}

// How long an office link's token works: long enough for a browser to start.
const officeTokenLifetime = 2 * time.Minute

// The most setup sessions kept; a new one makes the oldest make way.
const maxSetupSessions = 20

// NewSetupToken is a token for one link to the setup page.
func (a *Access) NewSetupToken() string { return newToken(&a.setupTokens, setupTokenLifetime) }

// NewSetupSession opens a session for the setup page, for a cookie to hold:
// Smolcorp's own window's (core_darwin.go's setup-session).
func (a *Access) NewSetupSession() string { return a.NewSetupSessionFrom("") }

// NewSetupSessionFrom opens a session for a browser that brought a link's
// token: the token is remembered with it (setuplink.go). "" is the window's.
func (a *Access) NewSetupSessionFrom(token string) string {
	id := randomKey()
	a.setupSessions = append(a.setupSessions, setupSession{id: id, token: token, window: token == ""})
	if token != "" {
		a.noteTokenUsed(token, id)
	}
	if len(a.setupSessions) > maxSetupSessions {
		a.setupSessions = a.setupSessions[len(a.setupSessions)-maxSetupSessions:]
	}
	return id
}

// IsSetupSession says whether candidate is an open session. It is never
// compared with the setup key, and the key never opens one.
func (a *Access) IsSetupSession(candidate string) bool {
	if candidate == "" {
		return false
	}
	return a.setupSessionOf(candidate) != nil
}

// RedeemSetupToken says whether candidate is a token not yet used and not
// run out, and uses it up.
func (a *Access) RedeemSetupToken(candidate string) bool {
	return redeemToken(a.setupTokens, candidate)
}

// NewOfficeToken is a token for one link to the office, from the tray.
func (a *Access) NewOfficeToken() string { return newToken(&a.officeTokens, officeTokenLifetime) }

// RedeemOfficeToken is RedeemSetupToken for the office's links.
func (a *Access) RedeemOfficeToken(candidate string) bool {
	return redeemToken(a.officeTokens, candidate)
}

// IsWindowKey is whether candidate is the office window's own key on this computer.
func (a *Access) IsWindowKey(candidate string) bool {
	return candidate != "" && a.WindowKey != "" && constantTimeEqual(candidate, a.WindowKey)
}

// IsBrowserKey is whether candidate is the key the office in a browser on this computer was given.
func (a *Access) IsBrowserKey(candidate string) bool {
	return candidate != "" && a.BrowserKey != "" && constantTimeEqual(candidate, a.BrowserKey)
}

func newToken(tokens *map[string]time.Time, lifetime time.Duration) string {
	now := time.Now()
	if *tokens == nil {
		*tokens = map[string]time.Time{}
	}
	for token, expiry := range *tokens {
		if now.After(expiry) {
			delete(*tokens, token)
		}
	}
	token := randomKey()
	(*tokens)[token] = now.Add(lifetime)
	return token
}

func redeemToken(tokens map[string]time.Time, candidate string) bool {
	for token, expiry := range tokens {
		if candidate != "" && constantTimeEqual(candidate, token) {
			delete(tokens, token)
			return time.Now().Before(expiry)
		}
	}
	return false
}

// NewCode replaces any earlier code and lifts lockouts.
func (a *Access) NewCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	a.code = fmt.Sprintf("%06d", n.Int64())
	a.pairKey = randomKey()
	a.codeExpiry = time.Now().Add(codeLifetime)
	a.misses = map[string]int{}
	a.totalMisses = 0
	return a.code
}

// ActiveCode is the current code, or "" if none was asked for or it ran out.
func (a *Access) ActiveCode() string {
	if a.code == "" || time.Now().After(a.codeExpiry) {
		return ""
	}
	return a.code
}

func (a *Access) CodeExpiresIn() time.Duration { return time.Until(a.codeExpiry) }

// PairingKey is the key the QR code hands over, while there's a code, or "".
func (a *Access) PairingKey() string {
	if a.ActiveCode() == "" {
		return ""
	}
	return a.pairKey
}

// Claim pairs the phone that scanned the QR code: it signs its request with
// the key the code carried, so the key itself never travels. Like a typed
// code, it works once, and wrong tries count towards the lockouts. Refused,
// it says why (signed.go's reasons): a phone whose clock is off may try again.
// The request was read already (readSigned), so the deck isn't held while its
// body comes.
func (a *Access) Claim(parts signedParts, ok bool, part, address, userAgent string, now time.Time) string {
	key := a.PairingKey()
	if key == "" || a.misses[address] >= maxAttemptsPerAddress {
		return signedBad
	}
	if ok && parts.signedBy(hashKey(key)) && len(part) == 43 && isNonce(part) {
		if why := a.fresh(parts, now); why != "" {
			return why
		}
		a.code, a.pairKey = "", ""
		// A phone that can't be kept (PairKey: the records' key failed its check) isn't paired, and is told so.
		if !a.Phones.PairKey(phoneKeyFrom(key, part), phoneName(userAgent), true) {
			return signedBad
		}
		return ""
	}
	a.misses[address]++
	a.totalMisses++
	if a.totalMisses >= maxAttempts {
		a.code = ""
	}
	return signedBad
}

// phoneKeyFrom is the key of a phone that claimed the QR code's key with a part
// of its own (32 random bytes, base64url), as sign.js makes it: a photo of the
// code alone isn't it, though with the claim read off the Wi-Fi (the part
// crosses it unencrypted) it is. No key exchange in 1.25.0.
func phoneKeyFrom(codeKey, part string) string {
	mac := hmac.New(sha256.New, []byte(codeKey))
	mac.Write([]byte(part))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Redeem returns a key for a newly paired phone if attempt is the active
// code and address isn't locked out. The code is spent on success. The key
// signs over the Wi-Fi only if it came over a private way: this computer's
// loopback or Tailscale. A right code with an empty key is a phone this
// computer couldn't keep (Pair).
func (a *Access) Redeem(attempt, address, userAgent string, private bool) (string, bool) {
	active := a.ActiveCode()
	if active == "" || a.misses[address] >= maxAttemptsPerAddress {
		return "", false
	}
	if constantTimeEqual(attempt, active) {
		a.code = ""
		return a.Phones.Pair(phoneName(userAgent), private), true
	}
	a.misses[address]++
	a.totalMisses++
	if a.totalMisses >= maxAttempts {
		a.code = ""
	}
	return "", false
}

// newSetupKey is a new setup key, for this process's memory alone. The files
// earlier versions kept it and the Windows window's key in are taken away (SK2).
func newSetupKey() string {
	_ = os.Remove(setupKeyFile)
	_ = os.Remove(windowKeyFile)
	return randomKey()
}

// loadHookKey is the hook's key, kept across starts. Under the Mac app it is also kept as the Mac's
// hook-header, the line its report.sh sends, which every agent hooked by the Swift engine runs: the key
// already there is kept, so they go on reporting with no new connect, and switching back finds the same.
func loadHookKey() string {
	key := macHookKey()
	if key == "" {
		if saved, err := os.ReadFile(hookKeyFile); err == nil {
			key = strings.TrimSpace(string(saved))
		}
	}
	if key == "" {
		key = randomKey()
	}
	keepPrivate(hookKeyFile, key+"\n")
	if hookHeaderFile != "" {
		keepPrivate(hookHeaderFile, hookKeyHeader+": "+key+"\n")
	}
	return key
}

// macHookKey is the key in the Mac app's hook-header, read as its Access.loadHookKey reads it, or "".
func macHookKey() string {
	if hookHeaderFile == "" {
		return ""
	}
	saved, err := os.ReadFile(hookHeaderFile)
	if err != nil {
		return ""
	}
	key, found := strings.CutPrefix(strings.TrimSpace(string(saved)), hookKeyHeader+": ")
	if !found || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return ""
	}
	return key
}

// keepPrivate writes text to path, readable by its owner only, unless it is so already.
func keepPrivate(path, text string) {
	if saved, err := os.ReadFile(path); err == nil && string(saved) == text {
		if info, err := os.Stat(path); err == nil && (runtime.GOOS == "windows" || info.Mode().Perm() == 0o600) {
			return
		}
	}
	_ = writePrivate(path, []byte(text))
}

// PairedPhone is a phone you paired, as phones.json keeps it.
type PairedPhone struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PairedAt time.Time `json:"pairedAt"`
	LastSeen time.Time `json:"lastSeen"`
	// SHA-256 of the phone's key, in hex, in memory. The key itself is never stored. It is also the secret the
	// phone signs its requests with (signed.go), so it opens what the phone opens: on a Mac it is kept in the file
	// only encrypted (Secret, records.go); off a Mac it is kept as it is, readable by you alone (§3.8).
	KeyHash string `json:"keyHash,omitempty"`
	// Secret is KeyHash encrypted under the record key, as a Mac keeps it in the file: never in memory.
	Secret string `json:"secret,omitempty"`
	// Signs: its key never crossed the Wi-Fi in clear (paired with the QR code,
	// or a code typed on this computer or over Tailscale), so it may sign there.
	// A phone paired before 1.25 sent its key as ?t= over the Wi-Fi, where
	// anyone could have read it: its key works only over this computer's
	// loopback and Tailscale, until it pairs again.
	Signs bool `json:"signs"`
	// Push is the subscription this phone's page posted to have notifications (pushsubs.go), nil without: the phone's
	// own, kept in this sealed record with it, so it goes when the phone is unpaired, and with the whole list when its
	// seal doesn't fit.
	Push *pushSubscription `json:"push,omitempty"`
}

// Phones is every paired phone. Each has its own key, so one can be revoked
// and the others keep working. This computer keeps only a hash of each key:
// the key itself never shows, though the hash signs as the phone does.
//
// Nor can the file add a phone (RT126FS2b N2, RT126FS2c B1): a phone's tap can answer the owner's asks
// (grants.go). phones.json is a sealed record (records.go): on a Mac, a list whose seal doesn't fit the record
// key, in the Keychain only the Mac app reads, is left out whole and said, so only a phone paired through the
// owner's own pairing flow (Pair) is ever let in. Nothing is offered for a tap: a list another program wrote, or
// one from before 1.26, is dropped, and its phones pair again (Q1). Each unpairing makes a new record key, so a
// copy of the list from before it no longer fits. Off a Mac nothing can seal it (Q7): the file counts as the core
// finds it at its start, and is only written after.
type Phones struct {
	All []PairedPhone
}

// How out of date LastSeen may get before a request updates it. Saving on
// every request would rewrite the file many times a second.
const seenResolution = time.Minute

// loadPhones is the phones this app paired, as phones.json lists them, if its seal fits; none at all if it
// doesn't, or at the one-time change of every key (B1), when every phone pairs again.
func loadPhones() *Phones {
	phones := &Phones{}
	data, err := os.ReadFile(phonesFile)
	if err != nil {
		return phones
	}
	if recordsRotated() {
		sayPhones("Smolcorp 1.26 changed every key it keeps, once: the phones paired before were let go. Pair each phone again from Setup.")
		return phones
	}
	body, counts := openBody("phones.json", data)
	if !counts {
		sayPhones("phones.json was changed by another program, or its seal doesn't fit: none of the phones it lists is let in. Pair your phones again from Setup.")
		recordDropped("phones.json", "") // said above, to the phones' own listener
		return phones
	}
	var listed []PairedPhone
	_ = json.Unmarshal(body, &listed)
	key := phoneSecretsKey(recordKey())
	for _, phone := range listed {
		if key != nil {
			phone.KeyHash = decryptSecret(key, phone.ID, phone.Secret)
		}
		phone.Secret = ""
		if phone.Push != nil && !phone.Push.usable() {
			// Not a push service's address this app sends to, or not a browser's keys: nothing is rung with it.
			phone.Push = nil
		}
		if isKeyHash(phone.KeyHash) {
			phones.All = append(phones.All, phone)
		}
	}
	return phones
}

// isKeyHash is whether text has the form of a key's hash as this app writes
// one: 64 lowercase hex digits.
func isKeyHash(text string) bool {
	if len(text) != 64 {
		return false
	}
	for _, c := range text {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// sayPhones says what loadPhones left out: in the log, or to a test.
var sayPhones = func(text string) { log.Printf("smolcorp: %s", text) }

// keepable is whether a phone can be kept now: not on a Mac whose record key failed its check, where a list
// written now could neither be sealed nor hide its secrets (records.go).
func keepable() bool { return recordsState() == recordsOff || recordKey() != nil }

// Pair adds a phone and returns its key: the only time the key exists here.
// signs: whether it may sign over the Wi-Fi (see PairedPhone.Signs). It can't be kept while the records' key
// failed its check: then it isn't paired, and its key is "".
func (p *Phones) Pair(name string, signs bool) string {
	key := randomKey()
	if !p.PairKey(key, name, signs) {
		return ""
	}
	return key
}

// PairKey adds a phone holding key, made already for the QR code: false, and nothing kept, if it can't be kept.
func (p *Phones) PairKey(key, name string, signs bool) bool {
	if !keepable() {
		return false
	}
	now := time.Now().UTC().Truncate(time.Second)
	p.All = append(p.All, PairedPhone{ID: newID(), Name: name, PairedAt: now, LastSeen: now, KeyHash: hashKey(key), Signs: signs})
	p.save()
	return true
}

// WithKey is the phone holding key, if any.
func (p *Phones) WithKey(key string) *PairedPhone {
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

// PairedAgain is whether a phone of the same name that may sign was paired
// after phone, one from before signing: it may be the same phone, paired
// again, whose old entry setup offers to remove.
func (p *Phones) PairedAgain(phone PairedPhone) bool {
	for _, other := range p.All {
		if other.Signs && other.Name == phone.Name && !other.PairedAt.Before(phone.PairedAt) && other.ID != phone.ID {
			return true
		}
	}
	return false
}

func (p *Phones) MarkSeen(phone *PairedPhone) {
	if time.Since(phone.LastSeen) < seenResolution {
		return
	}
	phone.LastSeen = time.Now().UTC().Truncate(time.Second)
	p.save()
}

// Remove forgets a phone, written under a new record key, so a copy of phones.json from before (the phone put
// back) no longer fits (RT126FS2c B1 c).
func (p *Phones) Remove(id string) {
	p.All = slices.DeleteFunc(p.All, func(phone PairedPhone) bool { return phone.ID == id })
	p.keep(true)
}

// RemoveAll forgets every phone, under a new record key too.
func (p *Phones) RemoveAll() {
	p.All = nil
	p.keep(true)
}

// save writes the phones, sealed, their secrets encrypted, under the record key of the moment (records.go): as they
// are, off a Mac; not at all on a Mac whose record key failed its check.
func (p *Phones) save() { p.keep(false) }

// keep is save, under a new record key when a phone went (tighten).
func (p *Phones) keep(tighten bool) {
	if !keepable() {
		return
	}
	_ = keepRecord("phones.json", p.writer(), tighten)
}

// writer writes the phones as they are now (records.go's recordWriter).
func (p *Phones) writer() recordWriter {
	phones, file := slices.Clone(p.All), phonesFile
	return func(key []byte, prev string) error {
		secrets := phoneSecretsKey(key)
		list := []PairedPhone{}
		for _, phone := range phones {
			if secrets != nil {
				phone.Secret, phone.KeyHash = encryptSecret(secrets, phone.ID, phone.KeyHash), ""
			}
			list = append(list, phone)
		}
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}
		return writePrivate(file, sealBody(key, prev, "phones.json", data))
	}
}

func (p *Phones) orEmpty() []PairedPhone {
	if p.All == nil {
		return []PairedPhone{}
	}
	return p.All
}

// phoneName is what the setup page calls a phone. The User-Agent is all
// there is to go on, and a broad name is all it can honestly give.
func phoneName(userAgent string) string {
	switch {
	// Agent Office pairs like a phone. Its app is named with a space, so the
	// system may write it either way.
	case strings.Contains(userAgent, "AgentOffice"), strings.Contains(userAgent, "Agent%20Office"):
		return "Agent Office"
	// So does the Mac's screen saver, from its Options (ScreenSaver/Pairing.swift).
	case strings.Contains(userAgent, "SauzzeDeckScreenSaver"):
		return "Screen saver"
	case strings.Contains(userAgent, "iPhone"):
		return "iPhone"
	case strings.Contains(userAgent, "iPad"):
		return "iPad"
	case strings.Contains(userAgent, "Android"):
		return "Android phone"
	}
	return "Phone"
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// randomKey is 32 random bytes, base64url-encoded so they travel in a URL
// unescaped.
func randomKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("no secure random source available")
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func newID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return strings.ToUpper(hex.EncodeToString(bytes))
}

// constantTimeEqual compares without stopping at the first difference, so
// response timing says nothing about how much of a guess was right.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
