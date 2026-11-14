package desktop

import (
	"crypto/hmac"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The GM's token's verifier is kept between runs of the core, so a hired GM, in a tmux session of its own that
// outlives the app, isn't refused after a restart (RM7 B3); the token itself is never kept (gmVerifierPrefix).
// Under the Mac app it is the app's own Keychain item (GMKeychain: smolcorp-gm, gm-token), through the bridge's
// gm-token, which the app reads only if no program changed it since it wrote it (KeychainGate.swift). In a test copy the app keeps that
// item in a file of its own, never the Keychain. Elsewhere it is this computer's secret store (Windows Credential
// Manager, or the Secret Service on Linux), as the Summary key's, else a file in the support folder,
// gm-token-kept: readable by you alone on Linux (0600), and on Windows as private as that folder is
// (gmtoken_store_other.go). Never the gm-token file, which `smolcorp gm`
// reads: that one is only for a GM started by hand, under the Mac app (gmbyhand.go).
//
// It is read once, as the core starts, before anything is served. Every change is written after it, in order and
// without d.mu (gmTokenKeeper), so the deck never waits on the Keychain; a write that fails is tried again, and a
// quit waits a little for the last one. A new token from setup is written at once, still without d.mu, as the
// Swift engine wrote it (GMIdentity.make): one the store refused isn't given (gmbyhand.go).

// gmTokenItem is the GM's token among the Mac app's own items (bridgeSecrets).
const gmTokenItem = "gm-token"

// gmTokenStore is where the GM's token is kept between runs: read gives "" with no error when none is kept;
// write with "" takes it away.
type gmTokenStore struct {
	read  func() (string, error)
	write func(token string) error
}

// noGMTokenStore keeps nothing: a Mac without the app, where this program only ever runs as tests, and every
// test, which never reaches a real store.
var noGMTokenStore = gmTokenStore{read: func() (string, error) { return "", nil }, write: func(string) error { return nil }}

// gmStoreFor is the store under the Mac app h, or this system's own without it.
func gmStoreFor(h hostBridge) gmTokenStore {
	if h == nil {
		return systemGMTokenStore
	}
	return gmTokenStore{
		read: func() (string, error) {
			answer, err := ask(h, bridgeRequest{Action: "secret-read", Item: gmTokenItem})
			if err == nil && answer.Failed {
				// There is an item the Keychain couldn't read (locked, say): not "none".
				err = errGMTokenUnreadable
			}
			if err != nil || !answer.Found {
				return "", err
			}
			return answer.Value, nil
		},
		write: func(token string) error { return hostWriteSecret(h, gmTokenItem, token) },
	}
}

// errGMTokenUnreadable: the Mac app has the GM's token in its Keychain item, but couldn't read it.
var errGMTokenUnreadable = errors.New("the GM's token couldn't be read from the Keychain")

// keptGMTokenName is the file the token is kept in on a computer whose secret store can't keep it, written whole to
// keptGMTokenTemp first.
const (
	keptGMTokenName = "gm-token-kept"
	keptGMTokenTemp = keptGMTokenName + ".tmp"
)

func keptGMTokenPath() string { return filepath.Join(supportDir, keptGMTokenName) }

// storeOrFile keeps the token in store, else, while store can't, in the file at path (0600, writeKeptToken). The file is there only
// while the last token written couldn't go in the store, so it is read first: a store that came back after it may
// still hold an older one.
func storeOrFile(store secretStore, path func() string) gmTokenStore {
	// None kept, or no store: either way there's nothing to read.
	return storeOrFileWhere(store, path, func(error) bool { return true })
}

// storeOrFileWhere is storeOrFile where only a store's error that none says is "none kept" reads as none: any other
// is the read's error, so nothing new is made and written over what the store may still hold (the push key,
// pushkey.go's pushKeyNone).
func storeOrFileWhere(store secretStore, path func() string, none func(error) bool) gmTokenStore {
	return gmTokenStore{
		read: func() (string, error) {
			if data, err := os.ReadFile(path()); err == nil {
				return strings.TrimSpace(string(data)), nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			token, err := store.get()
			if err != nil {
				if none(err) {
					return "", nil
				}
				return "", err
			}
			return token, nil
		},
		write: func(token string) error {
			if token == "" {
				_ = store.delete()
				if err := os.Remove(path()); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				return nil
			}
			if store.set(token) == nil {
				if err := os.Remove(path()); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				return nil
			}
			return writeKeptToken(path(), []byte(token+"\n"))
		},
	}
}

// writeKeptToken writes data to path through a new file beside it, path's name with ".tmp": made by this call alone,
// only its owner can read it, and a link put there is never followed. One left behind is taken away first.
func writeKeptToken(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|noFollow, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closed := f.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		_ = os.Remove(temp)
	}
	return err
}

// gmTokenRetry is how long the keeper waits before it tries a write that failed again, doubled after each failure
// up to gmTokenRetryMost; a change asked meanwhile, or a quit, tries again at once.
const (
	gmTokenRetry     = time.Second
	gmTokenRetryMost = time.Minute
	// gmTokenFlushWait is how long a quit waits for the token's last change to be kept.
	gmTokenFlushWait = 3 * time.Second
)

// gmTokenKeeper writes the token's changes to its store one after the other, the last one last, on a goroutine of
// its own that ends once the store holds the token asked. A write that fails is tried again until one succeeds.
// Nil keeps nothing.
type gmTokenKeeper struct {
	store gmTokenStore
	retry time.Duration
	// writing is held through each write to the store, the goroutine's and writeNow's, so two never cross.
	writing sync.Mutex
	mu      sync.Mutex
	// wanted is the token the store is to hold; latest, the one it holds, read at the start or last written, while
	// known (not after a read or a write that failed); unreadable, that the read at the start failed and nothing has
	// been written since. running: the goroutine is on; wake cuts its wait before a retry short; idle is closed
	// for each flush waiting once it ends.
	wanted, latest             string
	known, unreadable, running bool
	wake                       chan struct{}
	idle                       []chan struct{}
}

func newGMTokenKeeper(store gmTokenStore) *gmTokenKeeper {
	return &gmTokenKeeper{store: store, retry: gmTokenRetry, wake: make(chan struct{}, 1)}
}

// What is kept between runs is never the GM's token, only its verifier (SANDBOX-PLAN §3.1, SK1): "v1:" and the
// token's SHA-256 in hex. A program that reads the Keychain item, the secret store or gm-token-kept finds nothing
// that opens anything. A token kept whole, by 1.25 or before, is dropped at the first start of 1.26 (B1): the GM
// gets a new one when it is hired again.
//
// On a Mac the verifier is sealed under the record key too (§3.1, RT126SK1 B2): ":" and its seal follow it in the
// Keychain item, written again under each new key (records.go's rotate). A verifier another program put in the
// item, or one from a store whose key failed its check, never counts. Off a Mac nothing can seal it (§3.8).
const gmVerifierPrefix = "v1:"

// gmVerifierRecord is the GM's verifier among the sealed records.
const gmVerifierRecord = "gm-verifier"

// sealGMVerifier is kept, a verifier's form, as the store keeps it: sealed under the record key when there is one.
func sealGMVerifier(kept string) string {
	key := recordKey()
	if kept == "" || key == nil {
		return kept
	}
	return kept + ":" + recordMAC(key, gmVerifierRecord, "", []byte(kept))
}

// gmVerifierForm is a verifier as it's kept, "" for none.
func gmVerifierForm(verifier string) string {
	if verifier == "" {
		return ""
	}
	return gmVerifierPrefix + verifier
}

// gmVerifierFrom is the verifier kept, or "" if what's kept isn't one: with a record key, only one whose seal fits.
func gmVerifierFrom(saved string) string {
	rest, ok := strings.CutPrefix(strings.TrimSpace(saved), gmVerifierPrefix)
	verifier, _, _ := strings.Cut(rest, ":")
	if !ok || !isKeyHash(verifier) {
		return ""
	}
	// At the first sealed start (the update to 1.26) what's kept counts as found, as the records do; from then on,
	// only with its seal.
	if recordKey() != nil && !recordsFirst() && !hmac.Equal([]byte(strings.TrimSpace(saved)), []byte(sealGMVerifier(gmVerifierForm(verifier)))) {
		return ""
	}
	return verifier
}

// errRecordsUnchecked: the records' key failed its check, so a verifier kept counts for nothing, and nothing is
// written over it until the owner resets (records.go).
var errRecordsUnchecked = errors.New("the key that seals Smolcorp's records failed its check")

// restore is the verifier kept from the last run, as the core starts: "" for none, or what isn't one (a token
// kept whole by 1.25, which is taken away). With no GM to give it to (none hired, and none to start by hand), a
// kept verifier is taken away rather than kept working: a GM that's gone never comes back with its token.
func (k *gmTokenKeeper) restore(gmWanted bool) string {
	if k == nil {
		return ""
	}
	saved, err := k.store.read()
	verifier := gmVerifierFrom(saved)
	if err == nil && strings.TrimSpace(saved) != "" && recordsFailed() {
		err, verifier = errRecordsUnchecked, ""
	}
	kept := gmVerifierForm(verifier)
	k.mu.Lock()
	k.wanted, k.latest = kept, kept
	k.known, k.unreadable = err == nil && sealGMVerifier(kept) == strings.TrimSpace(saved), err != nil
	k.mu.Unlock()
	if err == nil && verifier != "" && !gmWanted {
		k.keep("")
		return ""
	}
	return verifier
}

// readFailed is whether the store couldn't be read as the core started, and nothing has been written to it since:
// as the Swift engine did, no token is made in place of the one that couldn't be read, and none written over it.
func (k *gmTokenKeeper) readFailed() bool {
	if k == nil {
		return false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.unreadable
}

// keep has token written to the store, after any change before it; the token the store is known to hold is nothing
// to do. It never waits on the store: the caller may hold d.mu. While the records' key can't be read
// (recordsHeldOnly) it is held in memory alone: written then, the verifier would carry no seal, and the GM's token
// from before would stop working at the restart after the owner unlocks the keychain (RT126SK1R4 follow-up 3).
func (k *gmTokenKeeper) keep(token string) {
	if k == nil {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.wanted = token
	if recordsHeldOnly() {
		return
	}
	k.startLocked()
}

// startLocked starts the goroutine while the store isn't known to hold the token wanted, or has it try again at once
// if it is waiting to. The caller holds k.mu.
func (k *gmTokenKeeper) startLocked() {
	if k.running {
		select {
		case k.wake <- struct{}{}:
		default:
		}
		return
	}
	if k.known && k.wanted == k.latest {
		return
	}
	k.running = true
	go k.write()
}

func (k *gmTokenKeeper) write() {
	wait := k.retry
	for {
		k.writing.Lock()
		k.mu.Lock()
		token := k.wanted
		if k.known && token == k.latest {
			k.running = false
			for _, done := range k.idle {
				close(done)
			}
			k.idle = nil
			k.mu.Unlock()
			k.writing.Unlock()
			return
		}
		k.mu.Unlock()
		err := k.store.write(sealGMVerifier(token))
		k.mu.Lock()
		k.wroteLocked(token, err)
		k.mu.Unlock()
		k.writing.Unlock()
		if err == nil {
			wait = k.retry
			continue
		}
		// The store is left as it was, or half changed (the Mac's Keychain item is taken away, then added): tried
		// again, with whatever is wanted by then, so the next run reads what this one has.
		select {
		case <-k.wake:
		case <-time.After(wait):
		}
		wait = min(2*wait, gmTokenRetryMost)
	}
}

// wroteLocked notes what a write did: the store holds token, or isn't known to hold anything. The caller holds k.mu.
func (k *gmTokenKeeper) wroteLocked(token string, err error) {
	if err != nil {
		k.known = false
		return
	}
	k.latest, k.known, k.unreadable = token, true, false
}

// writeNow writes token to the store at once, after any write under way, and says whether it was kept; one that
// failed is followed by the token wanted until now, tried again by the goroutine. It waits on the store: never call
// it with d.mu held. While the records' key can't be read (recordsHeldOnly) it writes nothing and says
// errRecordsHeld, so a new token is refused with Setup's reason, and the one from before goes on working.
func (k *gmTokenKeeper) writeNow(token string) error {
	if k == nil {
		return nil
	}
	if recordsHeldOnly() {
		return errRecordsHeld
	}
	k.writing.Lock()
	k.mu.Lock()
	asked := k.wanted
	k.mu.Unlock()
	err := k.store.write(sealGMVerifier(token))
	k.mu.Lock()
	k.wroteLocked(token, err)
	if err == nil && k.wanted == asked {
		// Nothing else was asked meanwhile: the caller puts token in memory next.
		k.wanted = token
	}
	k.mu.Unlock()
	k.writing.Unlock()
	if err != nil {
		k.mu.Lock()
		// Not over an item that couldn't be read at the start: nothing was asked for it, so nothing is put back.
		if !k.unreadable {
			k.startLocked()
		}
		k.mu.Unlock()
	}
	return err
}

// reseal writes what the store is to hold again, sealed under the record key of the moment: a step of each
// rotation (records.go), so the verifier still fits at the next start. Nothing over an item that couldn't be read.
func (k *gmTokenKeeper) reseal() error {
	if k == nil {
		return nil
	}
	k.writing.Lock()
	defer k.writing.Unlock()
	k.mu.Lock()
	token, skip := k.wanted, k.unreadable
	k.mu.Unlock()
	if skip || token == "" {
		return nil
	}
	err := k.store.write(sealGMVerifier(token))
	k.mu.Lock()
	k.wroteLocked(token, err)
	k.mu.Unlock()
	return err
}

// flush waits up to within until the store holds the last token asked, trying a failed write again at once, and
// says whether it does. It also says true when the keeper isn't trying to write anything: over an item that couldn't
// be read at the start, a refused write isn't tried again on purpose. A quit calls it, so a change just made isn't lost.
func (k *gmTokenKeeper) flush(within time.Duration) bool {
	if k == nil {
		return true
	}
	k.mu.Lock()
	if !k.running {
		k.mu.Unlock()
		return true
	}
	done := make(chan struct{})
	k.idle = append(k.idle, done)
	k.startLocked()
	k.mu.Unlock()
	select {
	case <-done:
		return true
	case <-time.After(within):
		return false
	}
}

// keepGMToken has the token's verifier as it is now kept for the next run. The caller holds d.mu.
func (d *Deck) keepGMToken() { d.gmKept.keep(gmVerifierForm(d.gmCheck())) }
