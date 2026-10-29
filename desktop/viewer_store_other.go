//go:build !darwin

package desktop

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// viewerStore keeps the viewer's id, key and write secret in Windows
// Credential Manager, or in the Secret Service on Linux (GNOME Keyring,
// KWallet), never in a file. A variable, for the tests.
var viewerStore = viewerSecrets{
	get:      func(account string) (string, error) { return keyring.Get(viewerService, account) },
	set:      func(account, value string) error { return keyring.Set(viewerService, account, value) },
	delete:   func(account string) { _ = keyring.Delete(viewerService, account) },
	notFound: func(err error) bool { return errors.Is(err, keyring.ErrNotFound) },
}
