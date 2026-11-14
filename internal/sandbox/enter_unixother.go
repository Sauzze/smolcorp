//go:build unix && !linux

package sandbox

// fsType is only asked on Linux, where the desk's support folder is a tmpfs of its own.
func (realSystem) fsType(string) string { return "" }
