//go:build !linux && !windows

package procmem

// NotDumpable is Linux's only.
func NotDumpable() error { return nil }

// DenyMemoryAccess is Windows' only.
func DenyMemoryAccess() error { return nil }
