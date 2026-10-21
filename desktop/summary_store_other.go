//go:build !darwin

package desktop

import "github.com/zalando/go-keyring"

// keyringStore keeps Summary's key in Windows Credential Manager, or in the
// Secret Service on Linux (GNOME Keyring, KWallet), never in a file.
var keyringStore = secretStore{
	get:    func() (string, error) { return keyring.Get(summaryKeyService, summaryKeyAccount) },
	set:    func(key string) error { return keyring.Set(summaryKeyService, summaryKeyAccount, key) },
	delete: func() error { return keyring.Delete(summaryKeyService, summaryKeyAccount) },
}
