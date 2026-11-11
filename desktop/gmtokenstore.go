package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The GM's token is kept between runs of the core, so a hired GM, in a tmux session of its own that outlives the
// app, isn't refused after a restart or an update (RM7 B3). Under the Mac app it is the app's own Keychain item,
// the one the Swift engine kept it in (GMKeychain: smolcorp-gm, gm-token), through the bridge's gm-token: the
// token of a GM hired on 1.24 is read on the first launch and goes on working. In a test copy the app keeps that
// item in a file of its own, never the Keychain. Elsewhere it is this computer's secret store (Windows Credential
// Manager, or the Secret Service on Linux), as the Summary key's, else a file readable by you alone,
// gm-token-kept, in the support folder (gmtoken_store_other.go). Never the gm-token file, which `smolcorp gm`
// reads: that one is only for a GM started by hand, under the Mac app (gmbyhand.go).
//
// It is read once, as the core starts, before anything is served. Every change is written after it, in order and
// without d.mu (gmTokenKeeper), so the deck never waits on the Keychain.

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
			value, found, err := hostReadSecret(h, gmTokenItem)
			if err != nil || !found {
				return "", err
			}
			return value, nil
		},
		write: func(token string) error { return hostWriteSecret(h, gmTokenItem, token) },
	}
}

// keptGMTokenName is the file the token is kept in on a computer whose secret store can't keep it.
const keptGMTokenName = "gm-token-kept"

func keptGMTokenPath() string { return filepath.Join(supportDir, keptGMTokenName) }

// storeOrFile keeps the token in store, else, while store can't, in the file at path (0600). The file is there only
// while the last token written couldn't go in the store, so it is read first: a store that came back after it may
// still hold an older one.
func storeOrFile(store secretStore, path func() string) gmTokenStore {
	return gmTokenStore{
		read: func() (string, error) {
			if data, err := os.ReadFile(path()); err == nil {
				return strings.TrimSpace(string(data)), nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			token, err := store.get()
			if err != nil {
				// None kept, or no store: either way there's nothing to read.
				return "", nil
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
			return writePrivate(path(), []byte(token+"\n"))
		},
	}
}

// gmTokenKeeper writes the token's changes to its store one after the other, the last one last, on a goroutine of
// its own that ends once nothing is waiting. Nil keeps nothing.
type gmTokenKeeper struct {
	store gmTokenStore
	mu    sync.Mutex
	done  *sync.Cond
	// latest is the token the store is to hold; pending, that it isn't written yet; running, that the goroutine is on.
	latest           string
	pending, running bool
}

func newGMTokenKeeper(store gmTokenStore) *gmTokenKeeper {
	k := &gmTokenKeeper{store: store}
	k.done = sync.NewCond(&k.mu)
	return k
}

// restore is the token kept from the last run, as the core starts: "" for none, or one that isn't a token. With no
// GM to give it to (none hired, and none to start by hand), a kept token is taken away rather than kept working: a
// GM that's gone never comes back with it.
func (k *gmTokenKeeper) restore(gmWanted bool) string {
	if k == nil {
		return ""
	}
	saved, err := k.store.read()
	token := validGMToken(saved)
	k.mu.Lock()
	k.latest = token
	k.mu.Unlock()
	if err == nil && token != "" && !gmWanted {
		k.keep("")
		return ""
	}
	return token
}

// keep has token written to the store, after any change before it; the same token as the last one asked is nothing
// to do. It never waits on the store: the caller may hold d.mu.
func (k *gmTokenKeeper) keep(token string) {
	if k == nil {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if token == k.latest {
		return
	}
	k.latest, k.pending = token, true
	if !k.running {
		k.running = true
		go k.write()
	}
}

func (k *gmTokenKeeper) write() {
	k.mu.Lock()
	for k.pending {
		token := k.latest
		k.pending = false
		k.mu.Unlock()
		// One that fails is left as it was: the token works on in memory, until the next change or restart.
		_ = k.store.write(token)
		k.mu.Lock()
	}
	k.running = false
	k.done.Broadcast()
	k.mu.Unlock()
}

// flush waits until every change asked is written.
func (k *gmTokenKeeper) flush() {
	if k == nil {
		return
	}
	k.mu.Lock()
	for k.running {
		k.done.Wait()
	}
	k.mu.Unlock()
}

// keepGMToken has the token as it is now kept for the next run. The caller holds d.mu.
func (d *Deck) keepGMToken() { d.gmKept.keep(d.gmToken) }
