package desktop

// viewerStore, on a Mac, where the Smolcorp in use is the Mac app and this
// one only ever runs as tests: no store, so they never touch the Keychain.
var viewerStore = viewerSecrets{
	get:      func(string) (string, error) { return "", errNoSecretStore },
	set:      func(string, string) error { return errNoSecretStore },
	delete:   func(string) {},
	notFound: func(error) bool { return false },
}
