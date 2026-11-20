package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXPOLICYFOLDF1 (RT126POLICYFOLDR2 F1), on this Mac with stand-ins only: a place one of the owner's links leads to, or
// a folder on its way, whose name has a code point Unicode 17.0 doesn't assign (U+E0080) stops the start: New says
// ErrUnknownCodePoint, naming the place and the code point. Decide denies such a path by a row of its own that no
// profile has, so followLinks must not read that deny as "held already": on a later macOS whose volume takes the name,
// Seatbelt would leave the owner's `.zshrc` target open. This volume refuses the name (EILSEQ), so each is only a
// link's text, which a link takes whole, leading to a place not there. A known letter (é) is the control: New makes
// the Policy, and a row of its own holds the target. A link the walk finds below a held place (`~/.ssh`'s
// `authorized_keys*`) is the same.
func TestOnAMacALinkToANameTheFoldDoesntKnowStopsTheStart(t *testing.T) {
	const unknown = "\U000E0080"
	for _, c := range []struct {
		what, link, target string
		refused            bool
	}{
		{"known letter (control)", ".zshrc", "dotfiles/zshérc", false},
		{"U+E0080 in the target's name", ".zshrc", "dotfiles/zsh" + unknown + "rc", true},
		{"U+E0080 in a folder on the way", ".zshrc", "dot" + unknown + "/zshrc", true},
		{"known letter, a link the walk finds (control)", ".ssh/authorized_keys2", "../dotfiles/aké", false},
		{"U+E0080, a link the walk finds", ".ssh/authorized_keys2", "../dotfiles/ak" + unknown, true},
	} {
		base, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		home := filepath.Join(base, "h")
		support := filepath.Join(home, "L", "AD")
		root := filepath.Join(home, "P", "app")
		for _, d := range []string{support, root, filepath.Join(home, "dotfiles"), filepath.Join(home, ".ssh")} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		link := filepath.Join(home, c.link)
		if err := os.Symlink(c.target, link); err != nil {
			t.Fatal(err)
		}
		real := filepath.Join(filepath.Dir(link), c.target)
		p, err := New(Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root})
		if c.refused {
			if !errors.Is(err, ErrUnknownCodePoint) {
				t.Errorf("%s: New says %v, want %v", c.what, err, ErrUnknownCodePoint)
			} else if !strings.Contains(err.Error(), "(U+E0080)") || !strings.Contains(err.Error(), filepath.Join(home, "dot")) {
				t.Errorf("%s: New says %q, want the place and U+E0080 named", c.what, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: New says %v", c.what, err)
			continue
		}
		if v, by := p.Decide(real, Write); v != Deny || by.Row == rowUnknownCodePoint {
			t.Errorf("%s: write at the target %+q: %v by %q, want deny by a row", c.what, real, v, by.Row)
		}
		if !hasRowAt(p, real, Write) {
			t.Errorf("%s: no row of its own denies write at the target %+q", c.what, real)
		}
	}
}

// hasRowAt is whether a row of p's, by its own place, denies op at path or a folder above it.
func hasRowAt(p *Policy, path string, op Op) bool {
	k := key(p.goos, path)
	for _, r := range p.rules {
		if r.Path != "" && r.verdict(op) == Deny && (r.key == k || r.Below && under(k, r.key)) {
			return true
		}
	}
	return false
}
