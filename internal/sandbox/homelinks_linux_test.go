package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// FXHOMELINKS on Linux (skipped on a Mac by its file name; run it on the Linux VM or CI's ubuntu with
// `go test -count=1 -race -timeout 8m -run 'TestOnLinuxAHomePlaceKeptAsALinkIsHeldWhereItLeads|TestTheOwnersPlacesAreHeldWhereTheirLinksLead' ./internal/sandbox`),
// with stand-ins in the test's own folder: the view is made from the Policy, so a home place kept as a link is held
// where it leads by the same rows as on a Mac. Each view is played as bwrap mounts it (playView), and what it lets a
// desk do at a place is the deepest mount over it (says).
//   - A place the view must hold that is itself a link (a linked `.zshrc` or `.claude`) fails the start (ErrHeldLink):
//     a mount there would land where the link leads. So did it before the fix, and so it stays.
//   - A linked ssh key while Remote Login answers fails the start the same way (RT126HOMELINKS B2): the view covers
//     `.ssh`, and a link in it the Policy denies is ErrHeldLink. That too was so before the fix, so its target was
//     never readable on Linux; now the Policy denies reading where it leads as well.
//   - A link inside a folder a row holds whole (`.config/git/config`) wasn't named by any row before: written through,
//     it reached its target, which the home folder's bind left writable. Now the target's folder is read-only (the
//     view shuts it, and the file needs no mount of its own below it), and the target still reads.
//   - An extra account folder kept as a link, given as deskAccounts gives it on Linux (resolved only: RT126HOMELINKS
//     B3), starts: the link is made again in the covered home, and where it leads is covered whole.
func TestOnLinuxAHomePlaceKeptAsALinkIsHeldWhereItLeads(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stand := func(home string, files map[string]string, links [][2]string) Places {
		t.Helper()
		for at, text := range files {
			at = filepath.Join(home, at)
			if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, l := range links {
			at := filepath.Join(home, l[0])
			if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(l[1], at); err != nil {
				t.Fatal(err)
			}
		}
		for _, at := range []string{"P/app/main.go", ".config/sauzzedeck-pro/hires/h1/BRIEF.md", "tmp/x"} {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(home, at)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, at), []byte("stand-in"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return Places{GOOS: "linux", Home: home, Support: filepath.Join(home, ".config", "sauzzedeck-pro"), OwnID: "h1",
			Root: filepath.Join(home, "P", "app"), RemoteLogin: true}
	}

	// A linked .zshrc and .claude: the start fails, as before.
	for _, name := range []string{".zshrc", ".claude"} {
		home := filepath.Join(base, "spelled"+name)
		pl := stand(home, map[string]string{"dotfiles/target/x": "stand-in"}, [][2]string{{name, "dotfiles/target"}})
		if _, err := prepareBwrap(pl, View{}, osView{}, filepath.Join(home, "tmp")); !errors.Is(err, ErrHeldLink) {
			t.Errorf("a linked %s: the view %v, want %v", name, err, ErrHeldLink)
		}
	}

	// A linked key while Remote Login answers: the start fails, as before; the Policy denies reading where it leads.
	home := filepath.Join(base, "key")
	key := filepath.Join(home, "dotfiles", "keys", "id_x")
	pl := stand(home, map[string]string{"dotfiles/keys/id_x": "stand-in key"}, [][2]string{{".ssh/id_x", "../dotfiles/keys/id_x"}})
	if v, r := mustNew(t, pl).Decide(key, Read); v != Deny {
		t.Errorf("reading the target of .ssh/id_x: %v (%s), want deny", v, r.Row)
	}
	if _, err := prepareBwrap(pl, View{}, osView{}, filepath.Join(home, "tmp")); !errors.Is(err, ErrHeldLink) {
		t.Errorf("a linked .ssh/id_x: the view %v, want %v", err, ErrHeldLink)
	}

	// A link in a held folder: its target is read-only in the view, and still reads.
	home = filepath.Join(base, "gitconfig")
	gitcfg := filepath.Join(home, "dotfiles", "git-config")
	pl = stand(home, map[string]string{"dotfiles/git-config": "[core]\n", "dotfiles/notes": "notes"},
		[][2]string{{".config/git/config", "../../dotfiles/git-config"}})
	if v, r := mustNew(t, pl).Decide(gitcfg, Write); v != Deny {
		t.Errorf("writing the target of .config/git/config: %v (%s), want deny", v, r.Row)
	}
	args, err := prepareBwrap(pl, View{}, osView{}, filepath.Join(home, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	mounts, _ := playView(t, args)
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
	}{{gitcfg, Write, Deny}, {gitcfg, Read, Allow}, {filepath.Join(home, ".config", "git", "config"), Write, Deny}} {
		if v := says(mounts, c.at, c.op); v != c.want {
			t.Errorf("the view: %s %s: %v, want %v (%q)", c.op, c.at, v, c.want, args)
		}
	}

	// An extra account folder kept as a link, given resolved only (deskAccounts on Linux): the start works.
	home = filepath.Join(base, "account")
	account, spelled := filepath.Join(home, "dotfiles", "claude-x"), filepath.Join(home, ".claude-x")
	pl = stand(home, map[string]string{"dotfiles/claude-x/settings.json": "{}", "dotfiles/claude-x/.credentials.json": "stand-in"},
		[][2]string{{".claude-x", "dotfiles/claude-x"}})
	pl.Accounts = []string{account}
	args, err = prepareBwrap(pl, View{}, osView{}, filepath.Join(home, "tmp"))
	if err != nil {
		t.Fatalf("an extra account folder kept as a link, given resolved: %v", err)
	}
	mounts, _ = playView(t, args)
	if !slices.Contains(mounts, mounted{"symlink", spelled}) || !containsPair(args, account, spelled) {
		t.Errorf("the link %s isn't made again to %s: %q", spelled, account, args)
	}
	for _, at := range []string{filepath.Join(account, "settings.json"), filepath.Join(account, ".credentials.json")} {
		for _, op := range []Op{Read, Write} {
			if v := says(mounts, at, op); v != Deny {
				t.Errorf("the view: %s %s: %v, want deny: the account folder is covered whole (%q)", op, at, v, args)
			}
		}
	}
}

// containsPair is whether b follows a in args.
func containsPair(args []string, a, b string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == a && args[i+1] == b {
			return true
		}
	}
	return false
}
