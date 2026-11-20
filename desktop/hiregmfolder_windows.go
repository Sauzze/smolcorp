//go:build windows

package desktop

// gmOwnsFolderNow: no desk starts in a sandbox on Windows yet (deskSandboxed), so a GM there can change whatever the
// owner can, and the rule (hiregmfolder.go) never applies.
func gmOwnsFolderNow(string) bool { return false }
