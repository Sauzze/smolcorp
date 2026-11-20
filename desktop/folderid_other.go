//go:build !darwin

package desktop

// folderIdentity: off a Mac no desk runs in a sandbox yet, and a hire's folder is checked by none of its accessors
// (checkedFolder), so no identity is kept.
func folderIdentity(string) string { return "" }
