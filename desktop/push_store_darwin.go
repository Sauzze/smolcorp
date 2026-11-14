package desktop

// systemPushKeyStore, on a Mac, where the Smolcorp in use is the Mac app (its own item, through the bridge:
// pushKeyStoreFor) and this one without it only ever runs as tests: none, so they never touch the Keychain.
var systemPushKeyStore = noPushKeyStore
