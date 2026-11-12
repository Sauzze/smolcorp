package desktop

// viewerStore, on a Mac, is a deck made without the Mac app, which only tests
// make: no store, so they never touch the Keychain. Under the Mac app the
// portable office's keys are its Keychain items, through the bridge
// (viewerStoreFor in hostshell.go).
var viewerStore = viewerSecrets{
	get:      func(string) (string, error) { return "", errNoSecretStore },
	set:      func(string, string) error { return errNoSecretStore },
	delete:   func(string) {},
	notFound: func(error) bool { return false },
}
