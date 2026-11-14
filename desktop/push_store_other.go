//go:build !darwin

package desktop

import (
	"testing"

	"github.com/zalando/go-keyring"
)

// The VAPID key in this computer's secret store.
const (
	pushKeyService = "smolcorp-push"
	pushKeyAccount = "vapid-key"
)

// systemPushKeyStore keeps the VAPID key in Windows Credential Manager, or in the Secret Service on Linux (GNOME
// Keyring, KWallet), else in push-key-kept (pushkey.go) when the store answers that it has none but won't keep one.
// Only the store's "not found" reads as no key (pushKeyNone): a store that is locked, or can't be reached now, gives
// an error, and no key is made over the one it may hold; with no store at all (no session bus) there's no key, and
// no notifications on that computer. A test binary never reaches either: no store.
var systemPushKeyStore = func() gmTokenStore {
	if testing.Testing() {
		return noPushKeyStore
	}
	return storeOrFileWhere(secretStore{
		get: func() (string, error) {
			if sessionBusMissing() {
				return "", errNoSessionBus
			}
			return keyring.Get(pushKeyService, pushKeyAccount)
		},
		set: func(key string) error {
			if sessionBusMissing() {
				return errNoSessionBus
			}
			return keyring.Set(pushKeyService, pushKeyAccount, key)
		},
		delete: func() error {
			if sessionBusMissing() {
				return errNoSessionBus
			}
			return keyring.Delete(pushKeyService, pushKeyAccount)
		},
	}, keptPushKeyPath, pushKeyNone)
}()
