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
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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
	// SetupKey is the setup page's, new at every start: the tray opens the
	// page with it, and it's kept in a file only you can read, for tools of
	// your own on this computer.
	SetupKey string
	// Tokens the setup links carry, each good once, until it runs out. The
	// key itself is never in a link.
	setupTokens map[string]time.Time
	// The setup page's sessions: random ids, kept in memory only, each opened
	// by a token. A cookie holds one of these, never the setup key.
	setupSessions []string
	// WindowKey is the key the Mac app's office window reads the office with,
	// handed to the app alone (core_darwin.go's office op): new at each start,
	// never saved, taken only from this computer, and never in a browser's
	// address. Under the Mac app it is the one window whose tap is the owner's.
	WindowKey string
	// BrowserKey is the key the office in a browser on this computer reads
	// the office with: the tray's Show Office carries a token of officeTokens,
	// which the deck swaps for it. It reads the office as the window does, but
	// a browser keeps it in its history, a file any program here can read, so
	// it never answers the owner's asks (RT126FS2b N1).
	BrowserKey   string
	officeTokens map[string]time.Time

	code        string
	codeExpiry  time.Time
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
	return &Access{Phones: loadPhones(), HookKey: loadHookKey(), SetupKey: newSetupKey(), WindowKey: randomKey(),
		BrowserKey: randomKey(), misses: map[string]int{}}
}

func (a *Access) IsHookKey(candidate string) bool {
	return candidate != "" && constantTimeEqual(candidate, a.HookKey)
}

func (a *Access) IsSetupKey(candidate string) bool {
	return candidate != "" && constantTimeEqual(candidate, a.SetupKey)
}

// How long a setup link's token works: long enough to open it from a terminal.
const setupTokenLifetime = 5 * time.Minute

// How long an office link's token works: long enough for a browser to start.
const officeTokenLifetime = 2 * time.Minute

// The most setup sessions kept; a new one makes the oldest make way.
const maxSetupSessions = 20

// NewSetupToken is a token for one link to the setup page.
func (a *Access) NewSetupToken() string { return newToken(&a.setupTokens, setupTokenLifetime) }

// NewSetupSession opens a session for the setup page, for a cookie to hold.
func (a *Access) NewSetupSession() string {
	id := randomKey()
	a.setupSessions = append(a.setupSessions, id)
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
	for _, id := range a.setupSessions {
		if constantTimeEqual(candidate, id) {
			return true
		}
	}
	return false
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

// Redeem returns a key for a newly paired phone if attempt is the active
// code and address isn't locked out. The code is spent on success. A right
// code with an empty key is a phone this computer couldn't keep (Pair).
func (a *Access) Redeem(attempt, address, userAgent string) (string, bool) {
	active := a.ActiveCode()
	if active == "" || a.misses[address] >= maxAttemptsPerAddress {
		return "", false
	}
	if constantTimeEqual(attempt, active) {
		a.code = ""
		return a.Phones.Pair(phoneName(userAgent)), true
	}
	a.misses[address]++
	a.totalMisses++
	if a.totalMisses >= maxAttempts {
		a.code = ""
	}
	return "", false
}

func newSetupKey() string {
	key := randomKey()
	_ = writePrivate(setupKeyFile, []byte(key+"\n"))
	return key
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
	// SHA-256 of the phone's key, in hex. The key itself is never stored.
	KeyHash string `json:"keyHash"`
	// Seal is this app's own mark on the phone (sealOf): an entry it didn't
	// write itself has none that fits, and is never trusted.
	Seal string `json:"seal,omitempty"`
}

// Phones is every paired phone. Each has its own key, so one can be revoked
// and the others keep working. This computer keeps only a hash of each key,
// so the file can't be used to pose as a phone.
//
// Nor to add one (RT126FS2b N2): a phone's tap can answer the owner's asks
// (grants.go), and phones.json holds hashes alone, so anyone who can write
// the file could list a key of their own. Each entry is sealed with an HMAC
// of its id, name and key's hash, made with a key of 32 random bytes kept
// beside the other secrets, in phones-seal (0600; the guard and a hire's
// fence name both files). At start, an entry whose seal doesn't fit is left
// out and said, so only a phone paired through the owner's own pairing flow
// (Pair), or one he confirmed on the setup page (Confirm), is ever let in.
// An entry with no seal at all is never sealed on trust, not even as an
// update's (RT126FS2c B1 b): it waits, left out, for the owner. And each
// unpairing makes a new seal's key, so a copy of an entry from before it no
// longer fits (B1 c).
type Phones struct {
	All []PairedPhone
	// Refused is what phones.json lists that this app can't vouch for, as it
	// started: never let in. An entry with no seal (from before the seal, or
	// written by another program) waits for the owner (waiting): the setup
	// page lists it, to confirm or revoke, and it stays in the file till then.
	// One whose seal doesn't fit, that shares its id or key with another
	// entry, or whose name this app never gives can't be confirmed (refusal):
	// it is dropped at the next save, and the phone is paired again.
	Refused []PairedPhone
	sealKey []byte
}

// phoneSealName is the file the seal's key is kept in, beside phones.json.
const phoneSealName = "phones-seal"

func phoneSealPath() string { return filepath.Join(filepath.Dir(phonesFile), phoneSealName) }

// loadPhoneSealKey is the seal's key, as kept; if none is kept, or what is
// kept isn't one, a new one. Nil if none could be kept.
func loadPhoneSealKey() []byte {
	if saved, err := os.ReadFile(phoneSealPath()); err == nil {
		if key, err := hex.DecodeString(strings.TrimSpace(string(saved))); err == nil && len(key) == 32 {
			return key
		}
	}
	return newPhoneSealKey()
}

// newPhoneSealKey makes a seal's key and keeps it, in place of any before it:
// nil if it couldn't be kept.
func newPhoneSealKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil
	}
	if writePrivate(phoneSealPath(), []byte(hex.EncodeToString(key)+"\n")) != nil {
		return nil
	}
	return key
}

// sealOf is the seal a phone carries when this app paired it: the HMAC of its
// id, name and key's hash. The name is in it, so the Windows window's device
// (window.go), whose key is in a file here, can't be renamed into a phone.
func sealOf(key []byte, phone PairedPhone) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(phone.ID + "\x00" + phone.Name + "\x00" + phone.KeyHash))
	return hex.EncodeToString(mac.Sum(nil))
}

// How out of date LastSeen may get before a request updates it. Saving on
// every request would rewrite the file many times a second.
const seenResolution = time.Minute

// loadPhones is the phones this app paired, as phones.json lists them: an
// entry it didn't seal is left out (Refused) and said. One with no seal (a
// list from before the seal, 1.25 or earlier, is all such) waits for the
// owner to confirm it on the setup page: nothing here can tell his phone
// from one another program wrote in.
func loadPhones() *Phones {
	phones := &Phones{}
	var listed []PairedPhone
	if data, err := os.ReadFile(phonesFile); err == nil {
		_ = json.Unmarshal(data, &listed)
	}
	key := loadPhoneSealKey()
	phones.sealKey = key
	for _, phone := range listed {
		if key != nil && phone.Seal != "" && hmac.Equal([]byte(phone.Seal), []byte(sealOf(key, phone))) {
			phones.All = append(phones.All, phone)
		} else {
			phones.Refused = append(phones.Refused, phone)
		}
	}
	if text := phones.refusedText(); text != "" {
		sayPhones(text)
	}
	return phones
}

// refusedText says what loadPhones left out, and why, or "". A name is said
// escaped and cut short: it is whatever was written into the list.
func (p *Phones) refusedText() string {
	byWhy := map[string][]string{}
	for i, phone := range p.Refused {
		why := p.refusal(i)
		byWhy[why] = append(byWhy[why], saidName(phone.Name))
	}
	var said []string
	if names := byWhy[refusedWaits]; len(names) > 0 {
		said = append(said, fmt.Sprintf("phones.json lists %d device(s) this app can't vouch for (%s): they carry no seal of this app's, "+
			"as a list from before 1.26 doesn't, or one another program wrote in. None of them is let in until you confirm it on the setup page, "+
			"under Paired phones. Confirm only a phone you know you paired; if you're not sure, revoke it and pair the phone again.",
			len(names), strings.Join(names, ", ")))
	}
	if names := byWhy[refusedSeal]; len(names) > 0 {
		said = append(said, fmt.Sprintf("phones.json lists %d device(s) this app didn't pair (%s): their seal doesn't fit, "+
			"as after an unpairing or when the seal's key is lost. None of them is let in. Pair a phone again from setup if it was yours.",
			len(names), strings.Join(names, ", ")))
	}
	if names := byWhy[refusedTwin]; len(names) > 0 {
		said = append(said, fmt.Sprintf("phones.json lists %d device(s) that share an id or a key with another of its entries (%s): "+
			"a tap on the setup page couldn't say which one it lets in, so none of them is let in or offered to you. "+
			"Pair a phone again from setup if it was yours.",
			len(names), strings.Join(names, ", ")))
	}
	if names := byWhy[refusedForm]; len(names) > 0 {
		said = append(said, fmt.Sprintf("phones.json lists %d device(s) under a name, or in a form, this app never gives a phone (%s): "+
			"such an entry was written by another program, maybe to mislead you, so none of them is let in or offered to you. "+
			"Pair a phone again from setup if it was yours.",
			len(names), strings.Join(names, ", ")))
	}
	return strings.Join(said, " ")
}

// saidName is a name from phones.json as the log says it: quoted, anything
// but plain ASCII escaped, and cut at 40 bytes.
func saidName(name string) string {
	quoted := strconv.QuoteToASCII(name)
	if len(quoted) > 40 {
		return quoted[:37] + "…\""
	}
	return quoted
}

// Why an entry phones.json lists is refused (refusal).
const (
	refusedWaits = ""     // it waits for the owner to confirm it
	refusedSeal  = "seal" // a seal that doesn't fit, or the window's own device
	refusedTwin  = "twin" // its id or key's hash is another listed entry's too
	refusedForm  = "form" // a name or key's hash this app never writes
)

// waitingNames is every name a build of this app ever gave a phone (phoneName,
// and the Mac's own engine before 1.25, which also said "iPhone app"). Only
// an entry under one of these may wait for the owner, so the setup page never
// shows him words another program chose (RT126FS2R4 B2): no markup, no
// look-alike, no "Smolcorp checked this phone".
var waitingNames = []string{"iPhone", "iPad", "Android phone", "Phone", "Agent Office", "Screen saver", "iPhone app"}

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

// refusal is why the refused entry at i isn't let in: refusedWaits if it waits
// for the owner. One waits only if it has no seal at all, isn't the window's
// own device (which pairs again by itself), holds an id and a key's hash that
// no other listed entry holds, sealed or not (RT126FS2R4 B1: the owner's tap
// names one entry, never one of two), and has a name and a hash as this app
// writes them (B2).
func (p *Phones) refusal(i int) string {
	phone := p.Refused[i]
	if phone.Seal != "" || phone.isWindowDevice() {
		return refusedSeal
	}
	same := func(other PairedPhone) bool { return other.ID == phone.ID || other.KeyHash == phone.KeyHash }
	if slices.ContainsFunc(p.All, same) {
		return refusedTwin
	}
	for j, other := range p.Refused {
		if j != i && same(other) {
			return refusedTwin
		}
	}
	if !slices.Contains(waitingNames, phone.Name) || !isKeyHash(phone.KeyHash) {
		return refusedForm
	}
	return refusedWaits
}

// Waiting is every refused entry that waits for the owner, for the setup page.
func (p *Phones) Waiting() []PairedPhone {
	var waiting []PairedPhone
	for i, phone := range p.Refused {
		if p.refusal(i) == refusedWaits {
			waiting = append(waiting, phone)
		}
	}
	return waiting
}

// EntryDigest names one entry of phones.json whole: every field, as listed.
// The setup page confirms an entry by its id and this, so its tap lets in
// exactly the row it showed, or nothing (RT126FS2R4 B1).
func EntryDigest(phone PairedPhone) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{phone.ID, phone.Name,
		strconv.FormatInt(phone.PairedAt.UnixNano(), 10), strconv.FormatInt(phone.LastSeen.UnixNano(), 10),
		phone.KeyHash, phone.Seal}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// KeyPrint is the short fingerprint of a waiting entry's key, as the setup
// page shows it: the first 8 digits of its hash, which isKeyHash has checked.
// It tells two rows apart; the phone itself shows nothing to match it with.
func KeyPrint(phone PairedPhone) string {
	if !isKeyHash(phone.KeyHash) {
		return ""
	}
	return phone.KeyHash[:8]
}

// sayPhones says what loadPhones left out: in the log, or to a test.
var sayPhones = func(text string) { log.Printf("smolcorp: %s", text) }

// seal is the seal's key, read or made at the first use: nil if none can be kept.
func (p *Phones) seal() []byte {
	if p.sealKey == nil {
		p.sealKey = loadPhoneSealKey()
	}
	return p.sealKey
}

// Pair adds a phone and returns its key: the only time the key exists here.
// It is sealed as this app's own. With no seal's key it can't be kept, so it
// isn't paired, and its key is "".
func (p *Phones) Pair(name string) string {
	sealKey := p.seal()
	if sealKey == nil {
		return ""
	}
	key := randomKey()
	now := time.Now().UTC().Truncate(time.Second)
	phone := PairedPhone{ID: newID(), Name: name, PairedAt: now, LastSeen: now, KeyHash: hashKey(key)}
	phone.Seal = sealOf(sealKey, phone)
	p.All = append(p.All, phone)
	p.save()
	return key
}

// Confirm lets in an entry that waits for the owner, sealed now as this app's
// own. Only the setup page's own tap reaches it (POST /setup/phones/confirm),
// naming the entry it showed by its id and its EntryDigest: the one entry
// that holds both, or none (RT126FS2R4 B1). False if no entry waits with that
// id and digest, if the id is listed more than once, or if it can't be sealed.
func (p *Phones) Confirm(id, digest string) bool {
	at := -1
	for i, phone := range p.Refused {
		if phone.ID != id {
			continue
		}
		if at >= 0 {
			return false
		}
		at = i
	}
	if at < 0 || p.refusal(at) != refusedWaits || !hmac.Equal([]byte(EntryDigest(p.Refused[at])), []byte(digest)) {
		return false
	}
	sealKey := p.seal()
	if sealKey == nil {
		return false
	}
	phone := p.Refused[at]
	phone.Seal = sealOf(sealKey, phone)
	p.All = append(p.All, phone)
	p.Refused = slices.Delete(p.Refused, at, at+1)
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

func (p *Phones) MarkSeen(phone *PairedPhone) {
	if time.Since(phone.LastSeen) < seenResolution {
		return
	}
	phone.LastSeen = time.Now().UTC().Truncate(time.Second)
	p.save()
}

// Remove forgets a phone, or an entry that waits for the owner, and seals the
// rest anew (reseal).
func (p *Phones) Remove(id string) {
	p.All = slices.DeleteFunc(p.All, func(phone PairedPhone) bool { return phone.ID == id })
	p.Refused = slices.DeleteFunc(p.Refused, func(phone PairedPhone) bool { return phone.ID == id })
	p.reseal()
	p.save()
}

// RemoveAll forgets every phone and every entry that waits, and makes a new
// seal's key.
func (p *Phones) RemoveAll() {
	p.All, p.Refused = nil, nil
	p.reseal()
	p.save()
}

// reseal makes a new seal's key and seals the phones kept with it, so a copy
// of phones.json from before (a phone unpaired, put back) no longer fits
// (RT126FS2c B1 c). The key is written before the list: a stop between the
// two leaves the phones kept refused, to pair again, never an old one let in.
// If the new key can't be kept, the old one is deleted all the same, and
// every entry refused at the next start.
func (p *Phones) reseal() {
	key := newPhoneSealKey()
	if key == nil {
		_ = os.Remove(phoneSealPath())
		p.sealKey = nil
		return
	}
	p.sealKey = key
	for i := range p.All {
		p.All[i].Seal = sealOf(key, p.All[i])
	}
}

// save writes the phones let in, and the entries that wait for the owner, as
// they were listed: with no seal, still not let in at the next start.
func (p *Phones) save() {
	data, err := json.MarshalIndent(append(slices.Clone(p.orEmpty()), p.Waiting()...), "", "  ")
	if err == nil {
		_ = writePrivate(phonesFile, data)
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
