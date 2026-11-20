package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// A canary is made only where `enter` has a sandbox to check (RT126SAX3 B1): on a Mac or Linux, one file under its
// whole random name and nothing beside it; off unix, ErrNoCanary and nothing at all, so the support folder's class
// map (desktop) passes there too. This runs on every system CI runs; the off-unix half on this Mac, as a stand-in,
// is FXSAX3R1's mutation M2.
func TestACanaryIsMadeOnlyWhereEnterHasOne(t *testing.T) {
	support := t.TempDir()
	c, err := NewCanary(support)
	names, _ := os.ReadDir(support)
	if !HasCanary {
		if !errors.Is(err, ErrNoCanary) || len(names) != 0 || c != (Canary{}) {
			t.Fatalf("with no sandbox here: %+v, %v, and %d files left", c, err, len(names))
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || filepath.Join(support, names[0].Name()) != c.Path ||
		!regexp.MustCompile(`^sandbox-canary-[0-9a-f]{32}$`).MatchString(names[0].Name()) {
		t.Errorf("the support folder holds %v, not only the canary %s", names, c.Path)
	}
	if err := c.Remove(); err != nil {
		t.Fatal(err)
	}
	if names, _ := os.ReadDir(support); len(names) != 0 {
		t.Errorf("the canary removed, the support folder still holds %v", names)
	}
}
