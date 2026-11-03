//go:build !darwin

package desktop

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// errNoSessionBus is the secret store's answer with no session bus to reach
// it on, as over SSH: as a store that is down, with no dbus-launch started.
var errNoSessionBus = errors.New("no session bus, so no secret store")

// keyringStore keeps Summary's key in Windows Credential Manager, or in the
// Secret Service on Linux (GNOME Keyring, KWallet), never in a file.
var keyringStore = secretStore{
	get: func() (string, error) {
		if sessionBusMissing() {
			return "", errNoSessionBus
		}
		return keyring.Get(summaryKeyService, summaryKeyAccount)
	},
	set: func(key string) error {
		if sessionBusMissing() {
			return errNoSessionBus
		}
		return keyring.Set(summaryKeyService, summaryKeyAccount, key)
	},
	delete: func() error {
		if sessionBusMissing() {
			return errNoSessionBus
		}
		return keyring.Delete(summaryKeyService, summaryKeyAccount)
	},
}
