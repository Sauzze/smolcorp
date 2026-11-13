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
// Keyring, KWallet), else in push-key-kept (pushkey.go). A test binary never reaches either: no store.
var systemPushKeyStore = func() gmTokenStore {
	if testing.Testing() {
		return noPushKeyStore
	}
	return storeOrFile(secretStore{
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
	}, keptPushKeyPath)
}()
