package desktop

// keyringStore, on a Mac, where the Smolcorp in use is the Mac app and this
// one only ever runs as tests: no store, so they never touch the Keychain.
var keyringStore = secretStore{
	get:    func() (string, error) { return "", errNoSecretStore },
	set:    func(string) error { return errNoSecretStore },
	delete: func() error { return nil },
}
