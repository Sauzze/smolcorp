//go:build !windows

package desktop

// longPathHere is the path as it is: only Windows has short names.
func longPathHere(name string) (string, error) { return name, nil }
