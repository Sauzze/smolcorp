package desktop

// systemGMTokenStore, on a Mac, where the Smolcorp in use is the Mac app (its Keychain item, through the bridge:
// gmStoreFor) and this one without it only ever runs as tests: no store, so they never touch the Keychain.
var systemGMTokenStore = noGMTokenStore
