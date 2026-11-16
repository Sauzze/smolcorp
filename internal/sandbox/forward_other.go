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

// RunConnect: a desk's ssh goes through the core only on Linux (forward_linux.go), where a desk has a network of its own.
func RunConnect(_ []string, _ io.Reader, _, stderr io.Writer) int {
	fmt.Fprintln(stderr, "smolcorp sandbox connect: Smolcorp gives this to ssh itself, inside a Linux desk's sandbox")
	return 2
}
