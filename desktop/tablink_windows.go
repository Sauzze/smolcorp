//go:build windows

package desktop

// inDeskSocketFolder: no desk has a socket folder here (tablink.go).
func inDeskSocketFolder(string) bool { return false }
