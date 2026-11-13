//go:build !darwin

package desktop

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// The Macs joined, in this computer's secret store, under the names the Mac's Keychain item has.
const (
	peerKeysService = "smolcorp-peers"
	peerKeysAccount = "peer-keys"
)

// systemPeerKeyStore keeps the computers joined, keys and all, in Windows Credential Manager, or in the Secret
// Service on Linux (GNOME Keyring, KWallet), never in a file: with neither, joining is refused. A test binary never
// reaches either: no store.
var systemPeerKeyStore = func() peerKeyStore {
	if testing.Testing() {
		return noPeerKeyStore
	}
	return peerKeyStore{
		read: func() (string, error) {
			if sessionBusMissing() {
				return "", errNoSessionBus
			}
			value, err := keyring.Get(peerKeysService, peerKeysAccount)
			if errors.Is(err, keyring.ErrNotFound) {
				return "", nil
			}
			return value, err
		},
		write: func(value string) error {
			if sessionBusMissing() {
				return errNoSessionBus
			}
			return keyring.Set(peerKeysService, peerKeysAccount, value)
		},
	}
}()
