//go:build unix

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FXHOMELINKS' rows on this system, with a stand-in home folder in the test's own folder: a place a row of the
// owner's names is decided where its links lead, every link on the way is held, a place a link leads to that isn't
// there can't be made, a link inside the folder a link leads to is followed too (two levels in, as with
// `~/.claude/hooks/pre.sh` and `~/.config/git/hooks/pre-commit`: RT126HOMELINKS B1), and a loop or a folder that
// can't be read on the way fails New. A Policy for another system's paths follows no link.
func TestTheOwnersPlacesAreHeldWhereTheirLinksLead(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stand := func(name string, links ...[2]string) Places {
		home := filepath.Join(base, name)
		for _, folder := range []string{"dotfiles/sub", "dotfiles/claude", "chain", ".config",
			"dotfiles/hooks", "dotfiles/githooks", "dotfiles/scripts", "dotfiles/bin", ".config/git"} {
			if err := os.MkdirAll(filepath.Join(home, folder), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, l := range links {
			if err := os.Symlink(l[1], filepath.Join(home, l[0])); err != nil {
				t.Fatal(err)
			}
		}
		return Places{Home: home, Support: filepath.Join(home, "L", "AD"), OwnID: "h1", Root: filepath.Join(home, "P", "app")}
	}
	pl := stand("h", [2]string{".zshrc", "chain/zshrc"}, [2]string{"chain/zshrc", "../dotfiles/zshrc"},
		[2]string{".bashrc", "cfg/bashrc"}, [2]string{"cfg", "dotfiles/sub"}, [2]string{".claude", "dotfiles/claude"},
		[2]string{".zshenv", "dotfiles/gone/zshenv"},
		[2]string{"dotfiles/claude/hooks", "../hooks"}, [2]string{"dotfiles/hooks/pre.sh", "../scripts/pre.sh"},
		[2]string{".config/git/hooks", "../../dotfiles/githooks"}, [2]string{"dotfiles/githooks/pre-commit", "../bin/pc"})
	for _, file := range []string{"dotfiles/scripts/pre.sh", "dotfiles/bin/pc"} {
		if err := os.WriteFile(filepath.Join(pl.Home, file), []byte("stand-in"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := mustNew(t, pl)
	at := func(rel string) string { return filepath.Join(pl.Home, rel) }
	for _, c := range []struct {
		rel  string
		op   Op
		want Verdict
	}{
		{"dotfiles/zshrc", Write, Deny}, {"chain/zshrc", Write, Deny}, {"chain", Move, Deny}, {"dotfiles/sub/bashrc", Write, Deny},
		{"cfg", Write, Deny}, {"dotfiles/sub", Move, Deny}, {"dotfiles/claude/settings.json", Write, Deny},
		{"dotfiles/claude/projects/t.jsonl", Read, Deny}, {"dotfiles/claude/CLAUDE.md", Read, Allow}, {"dotfiles/gone", Write, Deny},
		{"dotfiles/notes", Write, Allow}, {"dotfiles/sub/other", Write, Allow},
		// Two levels in: the hooks' targets and the links to them; the rest of their folders stays the desk's.
		{"dotfiles/scripts/pre.sh", Write, Deny}, {"dotfiles/hooks/pre.sh", Write, Deny}, {"dotfiles/hooks", Move, Deny},
		{"dotfiles/bin/pc", Write, Deny}, {"dotfiles/githooks/pre-commit", Write, Deny},
		{"dotfiles/scripts/notes", Write, Allow}, {"dotfiles/bin/notes", Write, Allow},
	} {
		if v, r := p.Decide(at(c.rel), c.op); v != c.want {
			t.Errorf("%s %s: %v (%s), want %v", c.op, c.rel, v, r.Row, c.want)
		}
	}
	for _, folder := range p.Hold() {
		if folder == at("dotfiles/gone") {
			t.Errorf("the Hold makes %s, where a link leads", folder)
		}
	}

	if _, err := New(stand("loop", [2]string{".profile", "a"}, [2]string{"a", "b"}, [2]string{"b", ".profile"})); !errors.Is(err, ErrHomeLinks) {
		t.Errorf("a link loop at .profile: %v, want %v", err, ErrHomeLinks)
	}
	if os.Geteuid() != 0 {
		shut := stand("shut")
		if err := os.Chmod(filepath.Join(shut.Home, ".config"), 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(filepath.Join(shut.Home, ".config"), 0o700) })
		if _, err := New(shut); !errors.Is(err, ErrHomeLinks) {
			t.Errorf("a folder on the way that can't be read: %v, want %v", err, ErrHomeLinks)
		}
	}
	other := "windows"
	if runtime.GOOS != "linux" {
		other = "linux"
	}
	looped := stand("other", [2]string{".profile", ".profile"})
	looped.GOOS = other
	if other == "windows" {
		looped.Home, looped.Support, looped.Root = `C:\Users\o`, `C:\Users\o\AppData\Roaming\Smolcorp`, `C:\Users\o\P\app`
	}
	if _, err := New(looped); err != nil {
		t.Errorf("a Policy for %s followed this system's links: %v", other, err)
	}
}
