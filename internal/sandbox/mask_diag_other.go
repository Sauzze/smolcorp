//go:build !linux

package sandbox

import "errors"

// readUnixDiag is Linux's alone: a start elsewhere fails (ErrNoSocketList), though PrepareBwrap refuses it first.
func readUnixDiag() ([]byte, error) { return nil, errors.ErrUnsupported }
