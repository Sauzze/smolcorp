//go:build darwin

package desktop

import "testing"

// FXWINWORKTREER4: the big repo rows (hireremovebig_test.go) for a hire's tree with its real profile too, where every
// look runs in the hire's sandbox and the branch's deletion goes through the core's folder.
func init() {
	bigTreeKinds = append(bigTreeKinds, bigTreeKind{"a sandboxed hire's tree", func(t *testing.T, grow func(*testing.T, string)) bigTree {
		r := newDeskRunRig(t)
		sandboxedHire(t, r)
		return grownRig(t, r, grow)
	}})
}
