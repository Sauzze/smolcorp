//go:build !darwin

package desktop

import (
	"testing"

	"github.com/zalando/go-keyring"
)

// The GM's token in this computer's secret store, under the names the Mac's Keychain item has.
const (
	gmTokenService = "smolcorp-gm"
	gmTokenAccount = "gm-token"
)

// systemGMTokenStore keeps the GM's token in Windows Credential Manager, or in the Secret Service on Linux (GNOME
// Keyring, KWallet), else in gm-token-kept (gmtokenstore.go). A test binary never reaches either: no store.
var systemGMTokenStore = func() gmTokenStore {
	if testing.Testing() {
		return noGMTokenStore
	}
	return storeOrFile(secretStore{
		get: func() (string, error) {
			if sessionBusMissing() {
				return "", errNoSessionBus
			}
			return keyring.Get(gmTokenService, gmTokenAccount)
		},
		set: func(token string) error {
			if sessionBusMissing() {
				return errNoSessionBus
			}
			return keyring.Set(gmTokenService, gmTokenAccount, token)
		},
		delete: func() error {
			if sessionBusMissing() {
				return errNoSessionBus
			}
			return keyring.Delete(gmTokenService, gmTokenAccount)
		},
	}, keptGMTokenPath)
}()
