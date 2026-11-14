package desktop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// The app's VAPID key (RFC 8292), which signs every notification it sends: made the first time one is sent, then
// kept between runs in the secret store, as the GM's token is (gmtokenstore.go), since every phone's subscription is
// made for its public half and stops working with another. Under the Mac app it is one of the app's own items,
// push-key, through the bridge; elsewhere this computer's secret store (Windows Credential Manager, or the Secret
// Service on Linux), else a file in the support folder, push-key-kept, readable by you alone. Only the public half
// is ever served; the private half never leaves the store and this program's memory, and is never logged.

// pushKeyItem is the VAPID key among the Mac app's own items.
const pushKeyItem = "push-key"

// keptPushKeyName is the file the key is kept in on a computer whose secret store can't keep it.
const keptPushKeyName = "push-key-kept"

func keptPushKeyPath() string { return filepath.Join(supportDir, keptPushKeyName) }

// errNoPushKeyStore: there's nowhere to keep the key, so none is made (a Mac without the app, where this program
// only ever runs as tests, and every test).
var errNoPushKeyStore = errors.New("nowhere to keep the push key")

// pushKeyNone is whether a secret store's error says it keeps no push key: its "not found" alone (RT126W2 N3). Any
// other (the store locked, its service not running, no session bus this time) may hide the key every phone
// subscribed with, and a new one kept in its place would break each of those subscriptions for good, unseen: the
// push service only refuses what is signed with another key. So the key's read fails, and none is made, until the
// store answers. A computer with no store at all, and no push-key-kept yet, gets no key, and sends nothing.
func pushKeyNone(err error) bool { return errors.Is(err, keyring.ErrNotFound) }

// noPushKeyStore keeps nothing, and makes no key: one that isn't kept would break every subscription at the next run.
var noPushKeyStore = gmTokenStore{
	read:  func() (string, error) { return "", nil },
	write: func(string) error { return errNoPushKeyStore },
}

// pushKeyStoreFor is where the key is kept under the Mac app h, or this system's own without it.
func pushKeyStoreFor(h hostBridge) gmTokenStore {
	if h == nil {
		return systemPushKeyStore
	}
	return gmTokenStore{
		read: func() (string, error) {
			answer, err := ask(h, bridgeRequest{Action: "secret-read", Item: pushKeyItem})
			if err == nil && answer.Failed {
				// There is an item the Keychain couldn't read: not "none", so no new key goes over it.
				err = errPushKeyUnreadable
			}
			if err != nil || !answer.Found {
				return "", err
			}
			return answer.Value, nil
		},
		write: func(key string) error { return hostWriteSecret(h, pushKeyItem, key) },
	}
}

var errPushKeyUnreadable = errors.New("the push key couldn't be read from the Keychain")

// pushKeys is the app's VAPID key, read from its store or made and kept there the first time it's asked for.
type pushKeys struct {
	store gmTokenStore
	mu    sync.Mutex
	kept  *ecdsa.PrivateKey
}

func newPushKeys(store gmTokenStore) *pushKeys { return &pushKeys{store: store} }

// key is the VAPID key: the one kept, or a new one once the store holds it. A store that can't be read gives an
// error and no new key, so a key there is never written over; one that can't keep a new key gives an error too, and
// the key isn't used. It waits on the store: never call it with d.mu held.
func (k *pushKeys) key() (*ecdsa.PrivateKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.kept != nil {
		return k.kept, nil
	}
	saved, err := k.store.read()
	if err != nil {
		return nil, fmt.Errorf("the push key couldn't be read: %w", err)
	}
	if saved != "" {
		if raw, err := b64.DecodeString(saved); err == nil {
			if key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw); err == nil {
				k.kept = key
				return key, nil
			}
		}
		// What's kept isn't a key, so nothing could have been subscribed with it: a new one goes in its place.
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	raw, err := key.Bytes()
	if err != nil {
		return nil, err
	}
	if err := k.store.write(b64.EncodeToString(raw)); err != nil {
		return nil, fmt.Errorf("the push key couldn't be kept: %w", err)
	}
	k.kept = key
	return key, nil
}

// publicKey is the key's public half, as a page subscribes with it (applicationServerKey): the uncompressed P-256
// point, base64url. The only part of the key ever served.
func (k *pushKeys) publicKey() (string, error) {
	key, err := k.key()
	if err != nil {
		return "", err
	}
	public, err := key.PublicKey.Bytes()
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(public), nil
}
