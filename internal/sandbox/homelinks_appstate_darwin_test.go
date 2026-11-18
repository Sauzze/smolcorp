package sandbox

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// FXPLANTSTERMR4 (the R3 review's N1): the link walk at each start looks into the folders that hold an owner's app or
// agent state whole only two levels deep, on a budget of each folder's own, and never into the agents' bulk stores, so
// a long-used agent folder no longer stops every hire. Every link those two levels hold is still followed and held
// where it leads: a top-level settings file kept as a link into a dotfiles folder above all. A link deeper in, or in a
// bulk store, isn't (the known limit, pinned here so a change to it is seen). A folder still too big says where it
// stopped. Stand-in home folders in the test's own folder, the Policy alone (Decide).
func TestOnAMacAnAppsOwnStateIsLookedIntoTwoLevelsDeepOnItsOwnBudget(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	limits(t, 40, maxHomeEntries)
	stand := func(name string, files map[string]int, links ...[2]string) Places {
		t.Helper()
		home := filepath.Join(base, name)
		for _, folder := range []string{"dotfiles/codex", "dotfiles/skills/s", "dotfiles/bin", "elsewhere/snap", ".codex/a/b",
			".agents/skills", ".gemini/tmp", ".grok", "Library/Preferences", ".local/share/opencode", ".config/fish"} {
			if err := os.MkdirAll(filepath.Join(home, folder), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for folder, n := range files {
			if err := os.MkdirAll(filepath.Join(home, folder), 0o700); err != nil {
				t.Fatal(err)
			}
			for i := range n {
				if err := os.WriteFile(filepath.Join(home, folder, fmt.Sprintf("f%d", i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, l := range links {
			if err := os.Symlink(l[1], filepath.Join(home, l[0])); err != nil {
				t.Fatal(err)
			}
		}
		return Places{GOOS: "darwin", Home: home, Support: filepath.Join(home, "L", "AD"), OwnID: "h1", Root: filepath.Join(home, "P", "app")}
	}

	// Past each budget in the bulk stores and below the top two levels, under it everywhere else.
	pl := stand("h", map[string]int{
		".cache/opencode/node_modules": 300, ".local/share/opencode/snapshot/p/objects/00": 300, ".codex/sessions/2026/11/18": 300,
		".gemini/history/p": 300, ".gemini/tmp/p/chats": 300, ".codex/a/b/c": 300, "Library/Preferences/ByHost": 30,
	},
		// At the top: a settings file kept as a link, the case that matters.
		[2]string{".codex/config.toml", "../dotfiles/codex/config.toml"},
		[2]string{"Library/Preferences/com.googlecode.iterm2.plist", "../../dotfiles/iterm2.plist"},
		// Two levels in: a skill kept as a link to a folder, and a link at the top of that folder.
		[2]string{".agents/skills/s", "../../dotfiles/skills/s"}, [2]string{"dotfiles/skills/s/run.sh", "../../bin/run.sh"},
		// A bulk store that is itself a link: held where it leads, not looked into there.
		[2]string{".local/share/opencode/storage", "../../../elsewhere/snap"}, [2]string{"elsewhere/snap/x.json", "../../dotfiles/in-snap"},
		// The known limit: a link three levels in, and one inside a bulk store, aren't followed.
		[2]string{".codex/a/b/deep.toml", "../../../dotfiles/deep.toml"}, [2]string{".gemini/tmp/p/plan.md", "../../../dotfiles/in-tmp"},
	)
	p := mustNew(t, pl)
	at := func(rel string) string { return filepath.Join(pl.Home, rel) }
	for _, c := range []struct {
		rel  string
		op   Op
		want Verdict
	}{
		{"dotfiles/codex/config.toml", Write, Deny}, {"dotfiles/iterm2.plist", Write, Deny},
		{"dotfiles/skills/s/SKILL.md", Write, Deny}, {"dotfiles/bin/run.sh", Write, Deny}, {"dotfiles/skills/s/run.sh", Move, Deny},
		{"elsewhere/snap/new.json", Write, Deny},
		// The rest of the dotfiles folder is the desk's, as before; and the known limit.
		{"dotfiles/notes", Write, Allow}, {"dotfiles/bin/notes", Write, Allow},
		{"dotfiles/deep.toml", Write, Allow}, {"dotfiles/in-tmp", Write, Allow}, {"dotfiles/in-snap", Write, Allow},
	} {
		if v, r := p.Decide(at(c.rel), c.op); v != c.want {
			t.Errorf("%s %s: %v (%s), want %v", c.op, c.rel, v, r.Row, c.want)
		}
	}

	// Too big at the top two levels of an agent's folder: refused, saying where it stopped.
	big := stand("big", map[string]int{".grok/big": 50})
	_, err = New(big)
	var tooBig *HomeTooBigError
	if !errors.Is(err, ErrHomeLinks) || !errors.As(err, &tooBig) {
		t.Fatalf("an agent's folder too big at its top two levels: %v, want a %T", err, tooBig)
	}
	if want := (HomeTooBigError{filepath.Join(big.Home, ".grok"), filepath.Join(big.Home, ".grok", "big"), 40}); *tooBig != want {
		t.Errorf("it stopped at %+v, want %+v", *tooBig, want)
	}
	// …and one of the other folders held whole, on the walk's own budget, which the apps' state doesn't take from. The
	// least budget the stand-in home's other looks need (the home folder's own names, for the rows of names beside a
	// file), found as the least that New takes, and a hundred more: a fish folder of 61 names fits, one of 151 doesn't,
	// and three agent folders of over 30 names each, over 100 together, fit on their own budgets.
	bare := stand("bare", nil)
	least := sort.Search(1<<20, func(n int) bool {
		maxHomeEntries = n
		_, err := New(bare)
		return err == nil
	})
	if least == 1<<20 {
		t.Fatal("the bare stand-in home doesn't fit any budget")
	}
	maxHomeEntries = least + 100
	agents := map[string]int{".codex/x": 30, ".gemini/y": 30, ".grok/z": 30}
	more := map[string]int{".config/fish/functions": 150}
	maps.Copy(more, agents)
	other := stand("other", more)
	if _, err = New(other); !errors.As(err, &tooBig) || tooBig.Inside != filepath.Join(other.Home, ".config", "fish", "functions") ||
		tooBig.Most != least+100 {
		t.Errorf("fish's folder, too big: %v", err)
	}
	agents[".config/fish/functions"] = 60
	if _, err = New(stand("fits", agents)); err != nil {
		t.Errorf("each folder under its budget: %v", err)
	}
}

// limits sets the walk's budgets for the test: an app state folder's own, and the rest's together.
func limits(t *testing.T, appState, rest int) {
	t.Helper()
	savedState, savedRest := maxAppStateEntries, maxHomeEntries
	maxAppStateEntries, maxHomeEntries = appState, rest
	t.Cleanup(func() { maxAppStateEntries, maxHomeEntries = savedState, savedRest })
}

// The R3 review's repro of N1 (its TestR3ReviewBigAgentFolder): a long-used OpenCode's snapshot gits, 200,100 files
// stood in for in the stand-in home, stopped every desk's start at its Policy. Now the Policy is made (a Claude Code
// hire starts), and under its real profile the owner's Codex settings, kept as a link into a dotfiles folder at the
// top of ~/.codex, are still held where the link leads; the snapshots stay the owner's.
func TestOnAMacABigAgentFolderDoesntStopAHireAndATopLinkIsStillHeld(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	snapshot := filepath.Join(d.home, ".local", "share", "opencode", "snapshot")
	for i := range 2001 {
		dir := filepath.Join(snapshot, "p", "objects", fmt.Sprintf("%02x", i%256), fmt.Sprintf("d%d", i))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for j := range 100 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("o%d", j)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	settings := filepath.Join(d.home, "dotfiles", "codex", "config.toml")
	mustWrite(t, settings, "# the owner's")
	if err := os.MkdirAll(filepath.Join(d.home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/codex/config.toml", filepath.Join(d.home, ".codex", "config.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(d.pl); err != nil {
		t.Fatalf("a big agent folder stops the desk's start: %v", err)
	}
	d.prepare(t)
	d.check(t, []escapeRow{
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		{"control: the rest of the dotfiles folder", attempt{"create", filepath.Join(d.home, "dotfiles", "notes"), ""}, "ok"},
		{"control: Codex's settings read", attempt{"read", settings, ""}, "ok"},
		{"Codex's settings, through its link", attempt{"write", filepath.Join(d.home, ".codex", "config.toml"), ""}, "EPERM"},
		{"Codex's settings, where the link leads", attempt{"write", settings, ""}, "EPERM"},
		{"Codex's settings replaced where the link leads", attempt{"rename", filepath.Join(d.root, "new.go"), settings}, "EPERM"},
		{"OpenCode's snapshots", attempt{"create", filepath.Join(snapshot, "p", "objects", "planted"), ""}, "EPERM"},
	})
	if got, err := os.ReadFile(settings); err != nil || string(got) != "# the owner's" {
		t.Errorf("Codex's settings changed: %q (%v)", got, err)
	}
}
