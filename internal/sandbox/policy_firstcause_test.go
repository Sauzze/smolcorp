package sandbox

import (
	"errors"
	"strings"
	"testing"
)

// FXPOLICYFOLDF1 (RT126POLICYFOLDR2 F2): the Policy's error is its first cause, and a Mac's place with a code point
// the fold doesn't know is named in it with that code point, so the desk's log says which place stopped the start.
func TestAPolicysErrorIsItsFirstCauseAndNamesThePlace(t *testing.T) {
	b := &builder{goos: "darwin"}
	b.add(Rule{Row: "one", Path: "/Users/o/notes-\U000E0080/x", Write: Deny})
	b.add(Rule{Row: "two", Path: "not/whole", Write: Deny})
	b.add(Rule{Row: "three", Path: "/Users/o", Tail: "a͸*", Write: Deny})
	if !errors.Is(b.err, ErrUnknownCodePoint) {
		t.Fatalf("error %v, want %v first", b.err, ErrUnknownCodePoint)
	}
	if want := "/Users/o/notes-\U000E0080/x (U+E0080)"; !strings.HasSuffix(b.err.Error(), want) {
		t.Errorf("error %q, want it to end %q", b.err, want)
	}

	b = &builder{goos: "darwin"}
	b.add(Rule{Row: "one", Path: "not/whole", Write: Deny})
	b.add(Rule{Row: "two", Path: "/Users/o", Tail: "a͸*", Write: Deny})
	if !errors.Is(b.err, ErrNotWhole) || errors.Is(b.err, ErrUnknownCodePoint) {
		t.Errorf("error %v, want %v, the first", b.err, ErrNotWhole)
	}

	b = &builder{goos: "darwin"}
	b.add(Rule{Row: "one", Path: "/Users/o", Tail: "a͸*", Write: Deny})
	if want := "/Users/o/a͸* (U+0378)"; !errors.Is(b.err, ErrUnknownCodePoint) || !strings.HasSuffix(b.err.Error(), want) {
		t.Errorf("error %q, want %v ending %q", b.err, ErrUnknownCodePoint, want)
	}

	b = &builder{goos: "linux"}
	b.add(Rule{Row: "one", Path: "/home/o/notes-\U000E0080", Write: Deny})
	if b.err != nil {
		t.Errorf("Linux: error %v, want none", b.err)
	}
}
