//go:build !linux

package sandbox

import (
	"fmt"
	"io"
)

// runForward: the forwarder is Linux's alone (forward_linux.go), whose desks have a network of their own.
func runForward(_ []string, stderr io.Writer) int {
	fmt.Fprintln(stderr, "smolcorp sandbox forward: Smolcorp runs this itself, inside a Linux desk's sandbox")
	return 2
}
