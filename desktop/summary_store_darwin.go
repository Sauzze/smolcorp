package desktop

// keyringStore, on a Mac, is a deck made without the Mac app, which only tests
// make: no store, so they never touch the Keychain. Under the Mac app a pasted
// Summary key is its Keychain item, through the bridge (summaryStore in
// hostshell.go).
var keyringStore = secretStore{
	get:    func() (string, error) { return "", errNoSecretStore },
	set:    func(string) error { return errNoSecretStore },
	delete: func() error { return nil },
}
