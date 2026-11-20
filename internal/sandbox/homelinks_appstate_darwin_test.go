package sandbox

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if want := (HomeTooBigError{Folder: filepath.Join(big.Home, ".grok"), Inside: filepath.Join(big.Home, ".grok", "big"), Most: 40}); *tooBig != want {
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
		tooBig.Most != least+100 || !tooBig.InAll {
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

// FXPOLISHA (the PLANTSTERMR4 review's O4 and O1): the account folders, ~/.claude and another account's, are looked
// into as an app's state is: two levels deep, on a budget of each one's own, and never into Claude Code's bulk stores
// (its transcripts, file history, to-do lists, shell snapshots, debug logs and hooks' environments), so a long-used
// ~/.claude no longer stops every hire. A settings file at the top kept as a link, a linked skill folder, and a linked
// hooks folder with a link in it are still held where they lead; a link three levels into the plugins, or inside a
// bulk store, isn't (the known limit, pinned so a change to it is seen; the skills, commands, agents and hooks are
// looked into whole since FXPOLISHAR1, in the row below). A bulk store that is itself a link is held where it leads.
// And iTerm2's AutoLaunch scripts, three levels into its folder, are looked at as a folder of their own: a script kept
// there as a link is held where it leads. Stand-in home folders in the test's own folder, the Policy alone (Decide).
//
// Since FXPOLISHAR3 (RT126POLISHAR2's N1, option a), the account folder counts on the shared budget, as when it
// was looked into whole: past the 40 an app's state gets, it still starts.
func TestOnAMacAnAccountFolderIsLookedIntoTwoLevelsDeepOnTheSharedBudget(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	limits(t, 40, maxHomeEntries)
	autoLaunch := "Library/Application Support/iTerm2/Scripts/AutoLaunch"
	stand := func(name string, files map[string]int, links ...[2]string) Places {
		t.Helper()
		home := filepath.Join(base, name)
		for _, folder := range []string{"dotfiles/claude", "dotfiles/hooks", "dotfiles/scripts", "dotfiles/skills/s", "elsewhere/fh",
			".claude/plugins/cache", ".claude/skills", ".claude/todos", ".claude-x", autoLaunch} {
			if err := os.MkdirAll(filepath.Join(home, folder), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, file := range []string{"dotfiles/claude/settings.json", "dotfiles/scripts/pre.sh", "dotfiles/skills/s/SKILL.md",
			"dotfiles/x.md", "dotfiles/autolaunch.py", "dotfiles/deep.sh", "dotfiles/in-todos", "dotfiles/in-fh", "elsewhere/fh/x"} {
			if err := os.WriteFile(filepath.Join(home, file), nil, 0o600); err != nil {
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
		return Places{GOOS: "darwin", Home: home, Support: filepath.Join(home, "L", "AD"), OwnID: "h1", Root: filepath.Join(home, "P", "app"),
			Accounts: []string{filepath.Join(home, ".claude-x")}}
	}

	// The shared budget: the least the bare stand-in home takes, and a hundred more. Each bulk store below holds more
	// than that, and so does the plugins' folder below its top two levels: walked whole on the shared budget, as before,
	// they stop the start.
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
	pl := stand("h", map[string]int{
		".claude/projects/-p-app": 300, ".claude/file-history/s1": 300, ".claude/todos": 300, ".claude/shell-snapshots": 300,
		".claude/debug": 300, ".claude/session-env/s1": 300, ".claude/plugins/cache/m/p/1.0": 300, ".claude-x/todos": 300,
	},
		// At the top: a settings file kept as a link, the case that matters, and another account's notes.
		[2]string{".claude/settings.json", "../dotfiles/claude/settings.json"}, [2]string{".claude-x/CLAUDE.md", "../dotfiles/x.md"},
		// A hooks folder kept as a link, and a link at the top of the folder it leads to (RT126HOMELINKS B1's shape).
		[2]string{".claude/hooks", "../dotfiles/hooks"}, [2]string{"dotfiles/hooks/pre.sh", "../scripts/pre.sh"},
		// Two levels in: a skill kept as a link to a folder.
		[2]string{".claude/skills/s", "../../dotfiles/skills/s"},
		// A bulk store that is itself a link: held where it leads, not looked into there.
		[2]string{".claude-x/file-history", "../elsewhere/fh"}, [2]string{"elsewhere/fh/y", "../../dotfiles/in-fh"},
		// The known limit: a link three levels in, and one inside a bulk store, aren't followed.
		[2]string{".claude/plugins/cache/m/deep.sh", "../../../../dotfiles/deep.sh"}, [2]string{".claude/todos/t.json", "../../dotfiles/in-todos"},
		// iTerm2's AutoLaunch, three levels into its folder: looked at all the same.
		[2]string{autoLaunch + "/a.py", "../../../../../dotfiles/autolaunch.py"},
	)
	p, err := New(pl)
	if err != nil {
		t.Fatalf("a long-used ~/.claude stops the start: %v", err)
	}
	at := func(rel string) string { return filepath.Join(pl.Home, rel) }
	for _, c := range []struct {
		rel  string
		op   Op
		want Verdict
	}{
		{"dotfiles/claude/settings.json", Write, Deny}, {"dotfiles/x.md", Write, Deny},
		{"dotfiles/hooks/new.sh", Write, Deny}, {"dotfiles/scripts/pre.sh", Write, Deny}, {"dotfiles/hooks/pre.sh", Move, Deny},
		{"dotfiles/skills/s/SKILL.md", Write, Deny}, {"elsewhere/fh/new", Write, Deny}, {"dotfiles/autolaunch.py", Write, Deny},
		// The rest of the dotfiles folder is the desk's, as before; and the known limit.
		{"dotfiles/notes", Write, Allow}, {"dotfiles/scripts/notes", Write, Allow},
		{"dotfiles/deep.sh", Write, Allow}, {"dotfiles/in-todos", Write, Allow}, {"dotfiles/in-fh", Write, Allow},
	} {
		if v, r := p.Decide(at(c.rel), c.op); v != c.want {
			t.Errorf("%s %s: %v (%s), want %v", c.op, c.rel, v, r.Row, c.want)
		}
	}

	// Past an app's own 40 at the top two levels of an account folder, or where a link at its top two levels leads (the
	// plugins), each in a stand-in home of its own: the start is made (FXPOLISHAR3).
	if _, err := New(stand("plans", map[string]int{".claude/plans": 20, ".claude/paste-cache": 21})); err != nil {
		t.Errorf("an account folder past an app's own budget: %v", err)
	}
	// (The folder is in dotfiles, a folder every stand-in home has at its top: a new one there would be listed again for
	// each pattern row in the home folder, ~/.p10k.zsh's writers' names among them since FXSWEEP3P1R1, and that cost,
	// not the folder's 41, would decide the row.)
	if _, err := New(stand("plugins", map[string]int{"dotfiles/big2": 41}, [2]string{".claude/plugins/big2", "../../dotfiles/big2"})); err != nil {
		t.Errorf("a folder a link in the plugins leads to, past an app's own budget: %v", err)
	}
	// Too big at the top two levels of an account folder: refused, saying where it stopped, on everything's count.
	big := stand("big", map[string]int{".claude/plans": 150})
	_, err = New(big)
	var tooBig *HomeTooBigError
	if !errors.Is(err, ErrHomeLinks) || !errors.As(err, &tooBig) {
		t.Fatalf("an account folder too big at its top two levels: %v, want a %T", err, tooBig)
	}
	if want := (HomeTooBigError{Folder: filepath.Join(big.Home, ".claude"), Inside: filepath.Join(big.Home, ".claude", "plans"), Most: least + 100,
		InAll: true}); *tooBig != want {
		t.Errorf("it stopped at %+v, want %+v", *tooBig, want)
	}

	// The skills are looked into whole on the shared budget, not the account folder's own (FXPOLISHAR2, RT126POLISHAR1's
	// N1): 60 files there, past the account folder's 40, fit; 150, past the shared budget's hundred spare, stop the
	// start, saying it stopped in what Claude Code loads (Loaded), on everything's count.
	if _, err := New(stand("skills", map[string]int{".claude/skills/s/node_modules": 60})); err != nil {
		t.Errorf("a skills folder past the account folder's own budget: %v", err)
	}
	big = stand("bigskills", map[string]int{".claude/skills/s/node_modules": 150})
	if _, err = New(big); !errors.As(err, &tooBig) {
		t.Fatalf("a skills folder past the shared budget: %v, want a %T", err, tooBig)
	}
	if want := (HomeTooBigError{Folder: filepath.Join(big.Home, ".claude"), Inside: filepath.Join(big.Home, ".claude", "skills"),
		Account: filepath.Join(big.Home, ".claude"), Most: least + 100, InAll: true, Loaded: true}); *tooBig != want {
		t.Errorf("it stopped at %+v, want %+v", *tooBig, want)
	}
}

// FXPOLISHAR1 (the RT126POLISHA review's O1, ruled a fix; O6 for this item): in an account folder, what Claude Code
// loads or runs is looked into whole, not two levels deep: a skill's SKILL.md, a namespaced command, a sub-agent in a
// subfolder and a hook in a subfolder, each kept as a link into a dotfiles folder, are held where they lead, as they
// were before FXPOLISHA. So is a link three levels into the folder a linked agents folder leads to. Everything else in
// the account folder is still looked into two levels deep: a link three levels into its plugins isn't followed (the
// known limit). Under the desk's real profile (sandbox-exec), in a stand-in home.
func TestOnAMacALinkDeepInAClaudeFoldersSkillsCommandsAgentsOrHooksIsHeld(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	dot := func(rel string) string { return filepath.Join(d.home, "dotfiles", rel) }
	for _, rel := range []string{"skill.md", "command.md", "agent.md", "pre.sh", "plugin.sh"} {
		mustWrite(t, dot(rel), "the owner's")
	}
	if err := os.MkdirAll(dot("agents/team/sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(d.home, ".claude")
	for _, l := range [][2]string{
		{"skills/deep/SKILL.md", "../../../dotfiles/skill.md"},
		{"commands/ns/c.md", "../../../dotfiles/command.md"},
		{"hooks/sub/pre.sh", "../../../dotfiles/pre.sh"},
		// An agents folder kept as a link, and a link three levels into the folder it leads to.
		{"agents", "../dotfiles/agents"},
		{"../dotfiles/agents/team/sub/a.md", "../../../agent.md"},
		// The known limit, as before: three levels into the plugins.
		{"plugins/p/deep/x.sh", "../../../../dotfiles/plugin.sh"},
	} {
		at := filepath.Join(claude, l[0])
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(l[1], at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := New(d.pl); err != nil {
		t.Fatalf("the desk's start: %v", err)
	}
	d.prepare(t)
	d.check(t, []escapeRow{
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		{"control: the rest of the dotfiles folder", attempt{"create", dot("notes"), ""}, "ok"},
		{"control: the skill read where its link leads", attempt{"read", dot("skill.md"), ""}, "ok"},
		{"a skill's SKILL.md, where its link leads", attempt{"write", dot("skill.md"), ""}, "EPERM"},
		{"a skill's SKILL.md, replaced where its link leads", attempt{"rename", filepath.Join(d.root, "new.go"), dot("skill.md")}, "EPERM"},
		{"a namespaced command, where its link leads", attempt{"write", dot("command.md"), ""}, "EPERM"},
		{"a hook in a subfolder, where its link leads", attempt{"write", dot("pre.sh"), ""}, "EPERM"},
		{"a sub-agent three levels into the linked agents folder, where its link leads", attempt{"write", dot("agent.md"), ""}, "EPERM"},
		{"a new sub-agent in the linked agents folder", attempt{"create", dot("agents/team/sub/b.md"), ""}, "EPERM"},
		{"the known limit: three levels into the plugins", attempt{"write", dot("plugin.sh"), ""}, "ok"},
	})
}

// FXPOLISHAR2 (RT126POLISHAR1's B1): a folder reached twice, first by a two-level look and then by a whole one, is
// looked into whole, and so is where a link in it leads: a link found by the two-level look is found again by the whole
// one. Layout D: one account folder, its plugins and its skills both keep a link to the same kit folder (the plugins
// sort first), and a link in the kit leads to a folder with a skill's file kept as a link three levels in. Layout E:
// Codex's skills folder and a Claude Code skill both lead to one shared folder (Codex's folder is looked at first).
// Under the desk's real profile (sandbox-exec), in a stand-in home.
func TestOnAMacAFolderReachedFirstByATwoLevelLookIsLookedIntoWholeWhenASkillLeadsThere(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	dot := func(rel string) string { return filepath.Join(d.home, "dotfiles", rel) }
	for _, rel := range []string{"orderleak.md", "xleak.md"} {
		mustWrite(t, dot(rel), "the owner's")
	}
	for _, rel := range []string{"kit", "deep/a/b", "shared", "sdeep/a/b"} {
		if err := os.MkdirAll(dot(rel), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range [][2]string{
		// D: the plugins' look is two levels deep, the skills' whole.
		{".claude/plugins/kit", "../../dotfiles/kit"},
		{".claude/skills/kit", "../../dotfiles/kit"},
		{"dotfiles/kit/sub", "../deep"},
		{"dotfiles/deep/a/b/SKILL.md", "../../../orderleak.md"},
		// E: Codex's folder is looked into two levels deep, Claude Code's skills whole.
		{".codex/skills", "../dotfiles/shared"},
		{".claude/skills/shared", "../../dotfiles/shared"},
		{"dotfiles/shared/s", "../sdeep"},
		{"dotfiles/sdeep/a/b/x.md", "../../../xleak.md"},
	} {
		at := filepath.Join(d.home, l[0])
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(l[1], at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := New(d.pl); err != nil {
		t.Fatalf("the desk's start: %v", err)
	}
	d.prepare(t)
	d.check(t, []escapeRow{
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		{"control: the rest of the dotfiles folder", attempt{"create", dot("notes"), ""}, "ok"},
		{"control: D's skill read where its link leads", attempt{"read", dot("orderleak.md"), ""}, "ok"},
		{"D: a skill's file reached through the plugins first, where its link leads", attempt{"write", dot("orderleak.md"), ""}, "EPERM"},
		{"D: the same, replaced", attempt{"rename", filepath.Join(d.root, "new.go"), dot("orderleak.md")}, "EPERM"},
		{"E: a file in a folder Codex's skills share with Claude Code's, where its link leads", attempt{"write", dot("xleak.md"), ""}, "EPERM"},
	})
}

// FXPOLISHAR3 (RT126POLISHAR2's O3): a link found by a two-level look and then by a whole one in the same round adds
// the row that holds where it leads once, not twice. Layout D, the Policy alone: ~/.claude's plugins and skills both
// keep a link to one kit folder, and a link in the kit leads to a folder with a skill's file kept as a link three
// levels in. One row holds that folder, and the profile names it as often as before FXPOLISHAR2 (three lines).
func TestOnAMacALinkFoundTwiceInOneRoundAddsItsRowOnce(t *testing.T) {
	d := newEscapeDesk(t)
	dot := func(rel string) string { return filepath.Join(d.home, "dotfiles", rel) }
	mustWrite(t, dot("orderleak.md"), "the owner's")
	for _, rel := range []string{"kit", "deep/a/b"} {
		if err := os.MkdirAll(dot(rel), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range [][2]string{
		{".claude/plugins/kit", "../../dotfiles/kit"},
		{".claude/skills/kit", "../../dotfiles/kit"},
		{"dotfiles/kit/sub", "../deep"},
		{"dotfiles/deep/a/b/SKILL.md", "../../../orderleak.md"},
	} {
		at := filepath.Join(d.home, l[0])
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(l[1], at); err != nil {
			t.Fatal(err)
		}
	}
	p, err := New(d.pl)
	if err != nil {
		t.Fatalf("the desk's start: %v", err)
	}
	rows := 0
	for _, r := range p.Rules() {
		if r.Path == dot("deep") && r.Row == rowHomeLink+": where one in the owner's places leads" {
			rows++
		}
	}
	if rows != 1 {
		t.Errorf("%d rows hold where kit/sub leads, want 1", rows)
	}
	profile, err := p.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(profile, dot("deep")+`"`); n != 3 {
		t.Errorf("the profile names %s %d times, want 3", dot("deep"), n)
	}
}
