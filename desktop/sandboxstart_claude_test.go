package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// CLAUDEPLANT1's rows: what a desk makes or changes in a .claude, or an .mcp.json, where the sandbox can't refuse one
// (its place, a home folder it may write) is listed and held, on record across a restart, and your own Claude Code
// isn't started where one is held. Stand-ins only: every settings or .mcp.json file holds `{"hooks":"standin"}`,
// every hook `#!/bin/sh` and nothing else.

// freshClaudeHeld gives the test an empty hold, on record in the support folder of the moment, and puts back what
// was there after it.
func freshClaudeHeld(t *testing.T) {
	t.Helper()
	claudeHeld.Lock()
	file, plants, baselines, kept, ended := claudeHeld.file, claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept, claudeHeld.ended
	claudeHeld.file = filepath.Join(supportDir, claudeHeldName)
	claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept = map[string]claudePlant{}, map[string]map[string]claudePlant{}, map[string]claudePlant{}
	claudeHeld.ended = map[string]bool{}
	claudeHeld.Unlock()
	t.Cleanup(func() {
		claudeHeld.Lock()
		claudeHeld.file, claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept, claudeHeld.ended = file, plants, baselines, kept, ended
		claudeHeld.Unlock()
	})
}

// claudeRestart forgets what the core holds in memory, as a new start of the core has it.
func claudeRestart() {
	claudeHeld.Lock()
	claudeHeld.file, claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept, claudeHeld.ended = "", nil, nil, nil, nil
	claudeHeld.Unlock()
}

// claudeLink makes a link at at, leading to to, or skips the test where this account can't.
func claudeLink(t *testing.T, to, at string) {
	t.Helper()
	if err := os.Symlink(to, at); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("this account can't make a symbolic link: %v", err)
		}
		t.Fatal(err)
	}
}

// claudeRegistry writes hires as the registry, which the core then reads as it would at its start.
func claudeRegistry(t *testing.T, hires ...Hire) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": hires})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	forgetHeldRegistries()
}

// claudeTree writes files under root (a name ending in / is a folder) and gives root.
func claudeTree(t *testing.T, root string, files ...string) string {
	t.Helper()
	for _, rel := range files {
		at := filepath.Join(root, filepath.FromSlash(rel))
		if strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(at, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		text := `{"hooks":"standin"}`
		if strings.HasSuffix(rel, ".sh") {
			text = "#!/bin/sh\n"
		}
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// listedAs is what was listed, by its path from root, with slashes, and why.
func listedAs(t *testing.T, root string, listed []claudePlant) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, p := range listed {
		rel, err := filepath.Rel(root, p.File)
		if err != nil {
			t.Fatal(err)
		}
		got[filepath.ToSlash(rel)] = p.Why
	}
	return got
}

func placeLook(root string) []claudeLook { return []claudeLook{{path: root, depth: -1}} }

func claudeTempDir(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return base
}

func sameListing(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	for rel, why := range want {
		if got[rel] != why {
			t.Errorf("%s: %s listed as %q, want %q", what, rel, got[rel], why)
		}
	}
	for rel, why := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("%s: %s listed (%q), want it not listed", what, rel, why)
		}
	}
}

// Each kind, at any depth in a desk's place, its own top .claude and .mcp.json too: the whole .claude tree (settings,
// hooks, commands, agents, skills, output styles, anything else) and an .mcp.json, any case. A CLAUDE.md and what's
// in a .git aren't.
func TestEachKindOfClaudePlantIsListedAndHeld(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"),
		".claude/settings.json", ".claude/hooks/pre.sh", ".mcp.json", "CLAUDE.md", "a/CLAUDE.md",
		"a/.claude/settings.json", "a/.claude/settings.local.json", "a/.claude/Settings-Extra.JSON", "a/.claude/CLAUDE.md",
		"a/.claude/commands/x.md", "a/.claude/agents/y.md", "a/.claude/skills/s/SKILL.md", "a/.claude/output-styles/o.md",
		"a/.claude/notes.txt", "a/.claude/hooks/pre.sh", "b/.claude/CLAUDE.md",
		"c/d/e/f/g/h/.claude/settings.json", "c/d/e/f/g/h/.claude/hooks/", "c/d/e/f/g/h/.mcp.json", "i/.CLAUDE/settings.json",
		"k/.MCP.JSON", ".git/x/.claude/settings.json", "j/.git/.claude/settings.json", ".git/.mcp.json")
	listed := holdClaudePlants("h1", placeLook(place), "")
	sameListing(t, "place", listedAs(t, place, listed), map[string]string{
		".claude": whyClaudeFolder, ".claude/settings.json": whyClaudeSettings, ".claude/hooks": whyClaudeHooks,
		".claude/hooks/pre.sh": whyClaudeHooks, ".mcp.json": whyClaudeMCP,
		"a/.claude": whyClaudeFolder, "a/.claude/settings.json": whyClaudeSettings, "a/.claude/settings.local.json": whyClaudeSettings,
		"a/.claude/Settings-Extra.JSON": whyClaudeSettings, "a/.claude/hooks": whyClaudeHooks, "a/.claude/hooks/pre.sh": whyClaudeHooks,
		"a/.claude/commands": whyClaudeTools, "a/.claude/commands/x.md": whyClaudeTools, "a/.claude/agents": whyClaudeTools,
		"a/.claude/agents/y.md": whyClaudeTools, "a/.claude/skills": whyClaudeTools, "a/.claude/skills/s": whyClaudeTools,
		"a/.claude/skills/s/SKILL.md": whyClaudeTools, "a/.claude/output-styles": whyClaudeTools,
		"a/.claude/output-styles/o.md": whyClaudeTools, "a/.claude/notes.txt": whyClaudeOther, "b/.claude": whyClaudeFolder,
		"c/d/e/f/g/h/.claude": whyClaudeFolder, "c/d/e/f/g/h/.claude/settings.json": whyClaudeSettings,
		"c/d/e/f/g/h/.claude/hooks": whyClaudeHooks, "c/d/e/f/g/h/.mcp.json": whyClaudeMCP,
		"i/.CLAUDE": whyClaudeFolder, "i/.CLAUDE/settings.json": whyClaudeSettings, "k/.MCP.JSON": whyClaudeMCP,
	})
	for _, p := range listed {
		if p.Desk != "h1" || p.ID == "" {
			t.Errorf("%s: desk %q, id %q", p.File, p.Desk, p.ID)
		}
		rel, _ := filepath.Rel(place, p.File)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		want := place // the folder above its .claude, or its .mcp.json's
		for i, part := range parts {
			if strings.EqualFold(part, ".claude") || strings.EqualFold(part, ".mcp.json") {
				want = filepath.Join(append([]string{place}, parts[:i]...)...)
				break
			}
		}
		if p.In != want {
			t.Errorf("%s: read by Claude Code started in %s, want %s", p.File, p.In, want)
		}
	}
	claudeHeld.Lock()
	held := len(claudeHeld.plants)
	claudeHeld.Unlock()
	if held != len(listed) {
		t.Errorf("%d held, %d listed", held, len(listed))
	}
}

// Your Claude Code isn't started in a folder where one is held, nor above or below it; elsewhere it is, and once it's
// gone, or you've kept it, it is there too.
func TestYourClaudeCodeIsntStartedWhereOneIsHeld(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	place := claudeTree(t, filepath.Join(base, "place"), "a/b/.claude/settings.json", "a/c/.claude/hooks/pre.sh", "z/")
	listed := holdClaudePlants("h1", placeLook(place), "")
	if len(listed) != 5 {
		t.Fatalf("listed %v", listedAs(t, place, listed))
	}
	for _, folder := range []string{"a/b", "a/b/deeper", "a", "", "a/c"} {
		at := filepath.Join(place, filepath.FromSlash(folder))
		if why := claudeHeldRefusal(at); !strings.Contains(why, "Smolcorp won't start your Claude Code in "+at) {
			t.Errorf("in %q: %q, want a refusal", folder, why)
		}
	}
	// Through a link to the same place, too.
	if err := os.Symlink(place, filepath.Join(base, "alias")); err == nil {
		if why := claudeHeldRefusal(filepath.Join(base, "alias", "a", "b")); why == "" {
			t.Error("started through a link to the folder")
		}
	}
	for _, folder := range []string{filepath.Join(place, "z"), filepath.Join(base, "elsewhere")} {
		if why := claudeHeldRefusal(folder); why != "" {
			t.Errorf("in %s: %q", folder, why)
		}
	}
	// Cleared: a/b's by Remove (the folder whole), a/c's by Keep (the folder, with what's held in it).
	for _, p := range listed {
		if strings.HasSuffix(p.File, filepath.Join("a", "b", ".claude")) {
			if err := removeClaudePlant(p.ID, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if exists(filepath.Join(place, "a", "b", ".claude")) {
		t.Error("Remove left it")
	}
	if why := claudeHeldRefusal(filepath.Join(place, "a", "b")); why != "" {
		t.Errorf("after Remove: %q", why)
	}
	if why := claudeHeldRefusal(filepath.Join(place, "a", "c")); why == "" {
		t.Error("a/c's isn't held any more, but it wasn't cleared")
	}
	for _, p := range listed {
		if p.File == filepath.Join(place, "a", "c", ".claude") {
			if err := removeClaudePlant(p.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	if why := claudeHeldRefusal(place); why != "" {
		t.Errorf("after Keep: %q", why)
	}
	if !exists(filepath.Join(place, "a", "c", ".claude", "hooks", "pre.sh")) {
		t.Error("Keep took it away")
	}
	// Kept, it isn't listed again, for this desk nor another at the same place; changed, it is.
	if again := holdClaudePlants("h1", placeLook(place), ""); len(again) != 0 {
		t.Errorf("listed again: %v", listedAs(t, place, again))
	}
	if again := holdClaudePlants("h2", placeLook(place), ""); len(again) != 0 {
		t.Errorf("listed again for another desk: %v", listedAs(t, place, again))
	}
	if err := os.WriteFile(filepath.Join(place, "a", "c", ".claude", "hooks", "pre.sh"), []byte("#!/bin/sh\necho standin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sameListing(t, "changed after Keep", listedAs(t, place, holdClaudePlants("h1", placeLook(place), "")),
		map[string]string{"a/c/.claude/hooks/pre.sh": whyClaudeHooks})
	if why := claudeHeldRefusal(filepath.Join(place, "a", "c")); why == "" {
		t.Error("changed after Keep, it isn't held")
	}
}

// A link named .claude is listed as itself, never followed: neither what it leads to nor a folder reached through a
// link is looked in. Remove takes the link away, not what it leads to.
func TestALinkNamedClaudeIsListedAndNeverFollowed(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	elsewhere := claudeTree(t, filepath.Join(base, "elsewhere"), "settings.json", "hooks/pre.sh", "inner/.claude/settings.json")
	place := claudeTree(t, filepath.Join(base, "place"), "a/", "b/", "c/")
	for link, to := range map[string]string{"a/.claude": elsewhere, "b/.claude": filepath.Join(base, "gone"), "c/through": elsewhere} {
		if err := os.Symlink(to, filepath.Join(place, filepath.FromSlash(link))); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("this account can't make a symbolic link: %v", err)
			}
			t.Fatal(err)
		}
	}
	claudeTree(t, place, "d/.claude") // a file named .claude
	listed := holdClaudePlants("h1", placeLook(place), "")
	sameListing(t, "links", listedAs(t, place, listed), map[string]string{
		"a/.claude": whyClaudeLink, "b/.claude": whyClaudeLink, "d/.claude": whyClaudeLink})
	if why := claudeHeldRefusal(filepath.Join(place, "a")); why == "" {
		t.Error("a link named .claude isn't held")
	}
	for _, p := range listed {
		if p.File == filepath.Join(place, "a", ".claude") {
			if err := removeClaudePlant(p.ID, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := os.Lstat(filepath.Join(place, "a", ".claude")); !errors.Is(err, os.ErrNotExist) || !exists(filepath.Join(elsewhere, "settings.json")) {
		t.Errorf("Remove: the link %v, what it leads to there %v", err, exists(filepath.Join(elsewhere, "settings.json")))
	}
	// A link put in its place after the listing isn't removed as if it were the folder.
	claudeTree(t, place, "e/.claude/settings.json")
	listed = holdClaudePlants("h1", placeLook(place), "")
	for _, p := range listed {
		if p.File == filepath.Join(place, "e", ".claude") {
			swapped := filepath.Join(place, "e", ".claude")
			if err := os.RemoveAll(swapped); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, swapped); err != nil {
				t.Fatal(err)
			}
			if err := removeClaudePlant(p.ID, false); !errors.Is(err, errGitPlantGone) {
				t.Errorf("Remove of a folder swapped for a link: %v", err)
			}
			if !exists(filepath.Join(elsewhere, "settings.json")) {
				t.Error("Remove went through the link")
			}
		}
	}
}

// Remove takes out only what was listed, as it was; anything else is refused and left.
func TestRemoveTakesOutOnlyTheClaudePlantAsItWasListed(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "a/.claude/settings.json")
	listed := holdClaudePlants("h1", placeLook(place), "")
	if err := removeClaudePlant("0123456789abcdef", false); !errors.Is(err, errGitPlantGone) {
		t.Errorf("an ID never listed: %v", err)
	}
	settings := filepath.Join(place, "a", ".claude", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"hooks":"standin 2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range listed {
		if err := removeClaudePlant(p.ID, false); !errors.Is(err, errGitPlantGone) {
			t.Errorf("%s, changed since: %v", p.File, err)
		}
	}
	if !exists(settings) {
		t.Fatal("a changed file was removed")
	}
	if why := claudeHeldRefusal(filepath.Join(place, "a")); why == "" {
		t.Error("refused Removes cleared it")
	}
}

// The first listing, or one after a restart, knows no baseline: a worktree's .claude the same as its checkout's at
// the same place is the project's, and isn't listed; one that differs, or one at a desk at the project, is.
func TestWithoutABaselineOnlyWhatDiffersFromTheCheckoutIsListed(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	both := []string{"pkg/.claude/settings.json", "pkg/.claude/hooks/pre.sh", "tools/.claude/CLAUDE.md"}
	repo := claudeTree(t, filepath.Join(base, "repo"), both...)
	wt := claudeTree(t, filepath.Join(base, "wt"), append(both, "new/.claude/settings.local.json")...)
	if err := os.WriteFile(filepath.Join(wt, "tools", ".claude", "settings.json"), []byte(`{"hooks":"standin"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sameListing(t, "worktree", listedAs(t, wt, holdClaudePlants("h1", placeLook(wt), repo)), map[string]string{
		"new/.claude": whyClaudeFolder, "new/.claude/settings.local.json": whyClaudeSettings,
		"tools/.claude/settings.json": whyClaudeSettings})
	// At the project, the checkout is the place: nothing to tell it by, so all is listed.
	sameListing(t, "project", listedAs(t, repo, holdClaudePlants("h2", placeLook(repo), repo)), map[string]string{
		"pkg/.claude": whyClaudeFolder, "pkg/.claude/settings.json": whyClaudeSettings, "pkg/.claude/hooks": whyClaudeHooks,
		"pkg/.claude/hooks/pre.sh": whyClaudeHooks, "tools/.claude": whyClaudeFolder})
	// The next listing goes by the baseline: what's as it was isn't listed again, a hook made since is.
	claudeTree(t, wt, "pkg/.claude/hooks/post.sh")
	sameListing(t, "later", listedAs(t, wt, holdClaudePlants("h1", placeLook(wt), repo)), map[string]string{
		"pkg/.claude/hooks/post.sh": whyClaudeHooks})
}

// At a desk's start (M6) the looks are its place and the home folders its Policy lets it write, claudeHomeDepth down:
// never your account folders, your checkout, other desks' places nor Smolcorp's own folder, and its own place only as
// its place (its top .claude the same as the checkout's: not listed).
func TestTheHomeFoldersADeskMayWriteAreLookedIn(t *testing.T) {
	freshClaudeHeld(t)
	saved := claudeHomeWalked
	claudeHomeWalked = true
	t.Cleanup(func() { claudeHomeWalked = saved })
	tr := newLinkTree(t)
	claudeTree(t, tr.home, "notes/.claude/settings.json", "notes/a/.claude/hooks/pre.sh", "notes/a/b/.claude/settings.local.json",
		"notes/a/b/c/.claude/settings.json", ".local/share/x/.claude/settings.json", "P/other/.claude/settings.json",
		"P/app-hires/h1/src/.claude/settings.json", "P/app-hires/h2/.claude/settings.json", "P/app/lib/.claude/settings.json",
		"L/AD/hires/h1/claude/.claude/settings.json", ".config/fish/.claude/settings.json")
	listed := holdClaudePlants("h1", claudeLooks(tr.policy(t), tr.places()), tr.repo)
	sameListing(t, "home", listedAs(t, tr.home, listed), map[string]string{
		"notes/.claude": whyClaudeFolder, "notes/.claude/settings.json": whyClaudeSettings,
		"notes/a/.claude": whyClaudeFolder, "notes/a/.claude/hooks": whyClaudeHooks, "notes/a/.claude/hooks/pre.sh": whyClaudeHooks,
		"notes/a/b/.claude": whyClaudeFolder, "notes/a/b/.claude/settings.local.json": whyClaudeSettings,
		".local/share/x/.claude": whyClaudeFolder, ".local/share/x/.claude/settings.json": whyClaudeSettings,
		"P/other/.claude": whyClaudeFolder, "P/other/.claude/settings.json": whyClaudeSettings,
		"P/app-hires/h1/src/.claude": whyClaudeFolder, "P/app-hires/h1/src/.claude/settings.json": whyClaudeSettings,
	})
	// Your Claude Code started in a home folder where one is held: refused.
	if why := claudeHeldRefusal(filepath.Join(tr.home, "notes", "a", "b")); why == "" {
		t.Error("started where one is held in a home folder")
	}
	// A Mac's listing is its place alone.
	claudeHomeWalked = false
	if looks := claudeLooks(tr.policy(t), tr.places()); len(looks) != 1 || looks[0].path != tr.root || looks[0].depth != -1 {
		t.Errorf("on a Mac: %+v", looks)
	}
}

// A fire lists and holds what the hire left in its place.
func TestAFireHoldsWhatTheHireLeftInItsPlace(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "a/.claude/settings.json")
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": []Hire{{ID: "abcdef123456", Name: "Ada", Folder: keptPath(place), State: "working"}}})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := markFired("abcdef123456", ""); err != nil {
		t.Fatal(err)
	}
	if why := claudeHeldRefusal(filepath.Join(place, "a")); why == "" {
		t.Error("what a fired hire left isn't held")
	}
}

// Take over hands the session to your own Claude Code: with one held in the worktree, it's handed back, and T3 Code
// is never told of the folder. One the same as the checkout's is the project's, and Take over goes ahead.
func TestTakeOverIsHandedBackWhileItsWorktreeHasOneHeld(t *testing.T) {
	freshClaudeHeld(t)
	k := newT3TakeOverRig(t)
	claudeTree(t, k.hire.Repo, "pkg/.claude/settings.json")
	claudeTree(t, k.hire.Worktree.storedOnly(), "pkg/.claude/settings.json", "web/.claude/hooks/pre.sh")
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	refused := refusalOf(t, err)
	if refused.Status != http.StatusConflict || !strings.Contains(refused.Message, "Smolcorp won't start your Claude Code in "+k.hire.Worktree.storedOnly()) ||
		!strings.Contains(refused.Message, filepath.Join("web", ".claude")) || !strings.Contains(refused.Message, "Ada was resumed") {
		t.Errorf("%+v", refused)
	}
	if !slices.Contains(k.log, "resume "+t3TakeOverSession) || slices.Contains(k.log, "call agentSessions.import") {
		t.Errorf("%q", k.log)
	}
	if slices.ContainsFunc(k.runs(), func(run []string) bool { return len(run) > 1 && run[0] == "project" && run[1] == "add" }) {
		t.Errorf("T3 Code was told: %q", k.runs())
	}
	// Removed, Take over goes ahead: the project's own .claude isn't held.
	if err := os.RemoveAll(filepath.Join(k.hire.Worktree.storedOnly(), "web")); err != nil {
		t.Fatal(err)
	}
	if thread, err := t3TakeOver(k.hire, k.session, k.ways()); err != nil || thread == "" {
		t.Errorf("once removed: %q %v", thread, err)
	}
}

// A plain hire is your own Claude Code in a terminal of yours: not started where one is held, listed at the gate.
// Another agent is.
func TestAPlainHireOfClaudeCodeIsntStartedWhereOneIsHeld(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	freshClaudeHeld(t)
	// Nothing listed it yet: a desk working in that folder, not fired yet (B4a). The gate lists it.
	claudeRegistry(t, Hire{ID: "abcdef999999", Name: "Ada", Folder: keptPath(folder), State: "working"})
	claudeTree(t, folder, "sub/.claude/settings.json")
	d.config.HireAgent = "claude"
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusConflict || !strings.Contains(message, "won't start your Claude Code") {
		t.Errorf("hire: %d %q", status, message)
	}
	if len(h.windows) != 0 {
		t.Errorf("a window opened: %q", h.windows)
	}
	d.config.HireAgent = "codex"
	hiresOfEveryAgentHere(d)
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.windows) != 1 {
		t.Errorf("codex: %d %q, %d windows", status, message, len(h.windows))
	}
}

// RT126CLAUDEPLANT1 B1 (probe 2): the place's own top .claude is listed by what changed in it. A hook the project's
// settings run, changed in the worktree, a hook and a skill added: each listed and held. What's as in the checkout
// isn't, and neither is the folder for your untracked settings.local.json there.
func TestTheTopClaudeIsListedByWhatChangedInIt(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	committed := []string{".claude/settings.json", ".claude/hooks/check.sh", ".mcp.json"}
	repo := claudeTree(t, filepath.Join(base, "repo"), append(committed, ".claude/settings.local.json")...)
	clean := claudeTree(t, filepath.Join(base, "clean"), committed...)
	if listed := holdClaudePlants("h2", placeLook(clean), repo); len(listed) != 0 {
		t.Errorf("a worktree as the checkout: %v", listedAs(t, clean, listed))
	}
	wt := claudeTree(t, filepath.Join(base, "wt"), append(committed, ".claude/hooks/new.sh", ".claude/skills/s/SKILL.md")...)
	if err := os.WriteFile(filepath.Join(wt, ".claude", "hooks", "check.sh"), []byte("#!/bin/sh\necho standin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sameListing(t, "top", listedAs(t, wt, holdClaudePlants("h1", placeLook(wt), repo)), map[string]string{
		".claude/hooks/check.sh": whyClaudeHooks, ".claude/hooks/new.sh": whyClaudeHooks, ".claude/skills": whyClaudeTools,
		".claude/skills/s": whyClaudeTools, ".claude/skills/s/SKILL.md": whyClaudeTools})
	if why := claudeHeldRefusal(wt); !strings.Contains(why, filepath.Join(".claude", "hooks", "check.sh")) {
		t.Errorf("in the worktree: %q", why)
	}
	if why := claudeHeldRefusal(clean); why != "" {
		t.Errorf("in the clean worktree: %q", why)
	}
}

// RT126CLAUDEPLANT1 B2 (probe 5): an .mcp.json names programs Claude Code starts. Made or changed anywhere in the place,
// any case, it's listed and held; one as in the checkout isn't.
func TestAnMCPFileIsListedAndHeld(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	repo := claudeTree(t, filepath.Join(base, "repo"), ".mcp.json", "same/.mcp.json")
	wt := claudeTree(t, filepath.Join(base, "wt"), ".mcp.json", "same/.mcp.json", "sub/.mcp.json", "deep/a/b/.Mcp.Json")
	if err := os.WriteFile(filepath.Join(wt, ".mcp.json"), []byte(`{"hooks":"standin 2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sameListing(t, "mcp", listedAs(t, wt, holdClaudePlants("h1", placeLook(wt), repo)), map[string]string{
		".mcp.json": whyClaudeMCP, "sub/.mcp.json": whyClaudeMCP, "deep/a/b/.Mcp.Json": whyClaudeMCP})
	for _, folder := range []string{"", "sub", "deep/a/b"} {
		if why := claudeHeldRefusal(filepath.Join(wt, filepath.FromSlash(folder))); why == "" {
			t.Errorf("in %q: not refused", folder)
		}
	}
	if why := claudeHeldRefusal(filepath.Join(wt, "same")); !strings.Contains(why, filepath.Join(wt, ".mcp.json")) {
		t.Errorf("in same, below the changed top one: %q", why)
	}
}

// RT126CLAUDEPLANT1 finding 1 (probe 4): a .claude link the checkout has too doesn't hide a change made through it in
// the place; one that leads out of the place is never cleared by the checkout.
func TestALinkTheCheckoutHasTooDoesntHideAChangeThroughIt(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	outside := claudeTree(t, filepath.Join(base, "outside"), "settings.json")
	var places []string
	for _, name := range []string{"repo", "same", "changed", "out"} {
		at := claudeTree(t, filepath.Join(base, name), "cfg/settings.json", "sub/")
		claudeLink(t, filepath.Join("..", "cfg"), filepath.Join(at, "sub", ".claude"))
		claudeLink(t, outside, filepath.Join(at, "far"))
		places = append(places, at)
	}
	repo := places[0]
	for _, at := range []string{repo, places[3]} {
		if err := os.MkdirAll(filepath.Join(at, "o"), 0o700); err != nil {
			t.Fatal(err)
		}
		claudeLink(t, outside, filepath.Join(at, "o", ".claude"))
	}
	if listed := holdClaudePlants("h1", placeLook(places[1]), repo); len(listed) != 0 {
		t.Errorf("the same link, the same behind it: %v", listedAs(t, places[1], listed))
	}
	claudeTree(t, places[2], "cfg/hooks/new.sh")
	if err := os.WriteFile(filepath.Join(places[2], "cfg", "settings.json"), []byte(`{"hooks":"standin 2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sameListing(t, "changed through it", listedAs(t, places[2], holdClaudePlants("h2", placeLook(places[2]), repo)),
		map[string]string{"sub/.claude": whyClaudeLink})
	sameListing(t, "leading out", listedAs(t, places[3], holdClaudePlants("h3", placeLook(places[3]), repo)),
		map[string]string{"o/.claude": whyClaudeLink})
}

// RT126CLAUDEPLANT1 B3 (probe 3): what's held, the baselines and what you kept are on record, so a restart forgets
// none of it; sealed, a record another program changed counts for nothing, and the gate then lists again.
func TestWhatsHeldOutlivesARestart(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "a/.claude/settings.json", "b/.mcp.json")
	listed := holdClaudePlants("h1", placeLook(place), "")
	if len(listed) != 3 {
		t.Fatalf("listed %v", listedAs(t, place, listed))
	}
	claudeRestart()
	if why := claudeHeldRefusal(filepath.Join(place, "a")); why == "" {
		t.Fatal("after a restart, it isn't held")
	}
	// Kept before a restart, it stays yours after it.
	for _, p := range listed {
		if p.File == filepath.Join(place, "b", ".mcp.json") {
			if err := removeClaudePlant(p.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	claudeRestart()
	if again := holdClaudePlants("h2", placeLook(place), ""); slices.ContainsFunc(again, func(p claudePlant) bool { return p.Why == whyClaudeMCP }) {
		t.Errorf("kept, then listed again after a restart: %v", listedAs(t, place, again))
	}
	if why := claudeHeldRefusal(filepath.Join(place, "b")); why != "" {
		t.Errorf("kept, then held after a restart: %q", why)
	}

	// Sealed: under a record key, the record fits and counts after a restart.
	records.mu.Lock()
	key, state, owed, writers := records.key, records.state, records.owed, maps.Clone(records.writers)
	records.key, records.state = []byte(strings.Repeat("k", 32)), recordsOK
	records.mu.Unlock()
	t.Cleanup(func() {
		records.mu.Lock()
		records.key, records.state, records.owed, records.writers = key, state, owed, writers
		records.mu.Unlock()
	})
	gone := Hire{ID: "abcdef777777", Name: "Ada", Folder: keptPath(filepath.Join(place, "c")), State: "fired"}
	claudeTree(t, gone.Folder.storedOnly(), ".claude/hooks/pre.sh")
	listClaudePlantsOf(gone)
	data, err := os.ReadFile(filepath.Join(supportDir, claudeHeldName))
	if seal, sealed := sealOfFile(claudeHeldName, data); err != nil || !sealed || !seal.fits(recordKey()) {
		t.Fatalf("not sealed: %v %v", err, sealed)
	}
	claudeRestart()
	if why := claudeHeldRefusal(gone.Folder.storedOnly()); why == "" {
		t.Fatal("sealed, after a restart, it isn't held")
	}
	// Another program writes the record: nothing held, and the gone hire on record as listed. It doesn't count.
	forged, _ := json.Marshal(claudeHeldRecord{Baselines: map[string][]claudePlant{gone.ID: nil}, Ended: []string{gone.ID}})
	if err := os.WriteFile(filepath.Join(supportDir, claudeHeldName), forged, 0o600); err != nil {
		t.Fatal(err)
	}
	claudeRestart()
	if claudeListed(gone.ID) {
		t.Error("a record another program wrote counted")
	}
	if why := claudeGate(gone.Folder.storedOnly(), []Hire{gone}); why == "" {
		t.Error("the record didn't count, and the gate didn't list again")
	}
}

// RT126CLAUDEPLANT1 B4a: the gate lists before it answers: a desk not fired yet, the GM, a gone hire never listed,
// and what was made after the last listing. A folder no hire is in or around isn't looked in.
func TestTheGateListsWhatAHireMadeSinceItsLastListing(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	desk := Hire{ID: "abcdef111111", Name: "Ada", Folder: keptPath(claudeTree(t, filepath.Join(base, "project"), "sub/.claude/settings.json")), State: "working"}
	gm := Hire{ID: "abcdef222222", Name: "Gem", Role: gmRole, Folder: keptPath(claudeTree(t, filepath.Join(base, "gm"), ".mcp.json")), State: "idle"}
	old := Hire{ID: "abcdef333333", Name: "Old", Folder: keptPath(claudeTree(t, filepath.Join(base, "old"), "x/.claude/hooks/pre.sh")), State: "fired"}
	claudeRegistry(t, desk, gm, old)
	for _, h := range []Hire{desk, gm, old} {
		if why := claudeGate(h.Folder.storedOnly(), officeHires()); why == "" {
			t.Errorf("%s's folder: not refused", h.Name)
		}
	}
	if why := claudeGate(filepath.Join(base, "elsewhere"), officeHires()); why != "" {
		t.Errorf("elsewhere: %q", why)
	}
	// Kept, it goes ahead; then one made after the listing is listed at the next gate.
	claudeHeld.Lock()
	ids := slices.Collect(maps.Keys(claudeHeld.plants))
	claudeHeld.Unlock()
	for _, id := range ids {
		_ = removeClaudePlant(id, true)
	}
	if why := claudeGate(desk.Folder.storedOnly(), officeHires()); why != "" {
		t.Fatalf("kept: %q", why)
	}
	claudeTree(t, desk.Folder.storedOnly(), "later/.claude/settings.json")
	if why := claudeGate(desk.Folder.storedOnly(), officeHires()); !strings.Contains(why, filepath.Join("later", ".claude")) {
		t.Errorf("made after the listing: %q", why)
	}
	// A gone hire whose session isn't known to have ended (from before this record) is listed again at each gate.
	if claudeListed(old.ID) {
		t.Error("a gone hire the gate listed is on record")
	}
	claudeTree(t, old.Folder.storedOnly(), "y/.claude/settings.json")
	if why := claudeGate(old.Folder.storedOnly(), officeHires()); !strings.Contains(why, filepath.Join("y", ".claude")) {
		t.Errorf("a gone hire not known to have ended, not listed again: %q", why)
	}
	// One whose session is known to have ended is on record, even one that left nothing, and isn't listed again: what
	// you change there later is yours.
	quiet := Hire{ID: "abcdef444444", Name: "Quiet", Folder: keptPath(claudeTree(t, filepath.Join(base, "quiet"), "src/")), State: "working"}
	claudeRegistry(t, quiet)
	if err := markFired(quiet.ID, ""); err != nil {
		t.Fatal(err)
	}
	claudeHireEnded(quiet.ID)
	claudeTree(t, quiet.Folder.storedOnly(), "src/.claude/settings.json")
	if why := claudeGate(quiet.Folder.storedOnly(), officeHires()); why != "" {
		t.Errorf("a gone hire that left nothing, listed again: %q", why)
	}
}

// RT126CLAUDEPLANT1 B4b: every way a hire is gone lists its place, the GM's edit of its state too: one listing in the
// registry's save. An edit that leaves it open lists nothing.
func TestEveryWayAHireIsGoneListsItsPlace(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	var hires []Hire
	for i, name := range []string{"edited-fired", "edited-failed", "failed", "fired", "renamed"} {
		hires = append(hires, Hire{ID: fmt.Sprintf("abcdef00000%d", i), Name: name, State: "working",
			Folder: keptPath(claudeTree(t, filepath.Join(base, name), "a/.claude/settings.json"))})
	}
	claudeRegistry(t, hires...)
	for i, state := range []string{"fired", "failed"} {
		if _, refused := editHire(registryPath(), []byte(`{"id":"`+hires[i].ID+`","state":"`+state+`"}`), noDeskSession); refused != nil {
			t.Fatal(refused.message)
		}
	}
	failHire(hires[2].ID, "standin")
	if err := markFired(hires[3].ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"`+hires[4].ID+`","nickname":"Bea"}`), noDeskSession); refused != nil {
		t.Fatal(refused.message)
	}
	for i, h := range hires {
		why := claudeHeldRefusal(filepath.Join(h.Folder.storedOnly(), "a"))
		if gone := i < 4; gone != (why != "") {
			t.Errorf("%s: %q", h.Name, why)
		}
		// None of these ways says its session ended (only claudeHireEnded does): none is on record.
		if claudeListed(h.ID) {
			t.Errorf("%s: on record as listed", h.Name)
		}
	}
}

// RT126CLAUDEPLANT1R2 B4b (R2's probe): the GM's edit makes a hire fired or failed and nothing else, so its desk may
// still be at work, and what it plants after the edit is listed at the next gate: the edit doesn't put it on record
// (option 1), with or without a worktree, even when its session isn't seen up.
func TestWhatADeskPlantsAfterTheGMsEditMadeItGoneIsListedAtTheGate(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	upWas := claudeSessionUp
	claudeSessionUp = func(Hire) bool { return false } // a job it started outlives what tmux shows
	t.Cleanup(func() { claudeSessionUp = upWas })
	base := claudeTempDir(t)
	repo := claudeTree(t, filepath.Join(base, "repo"), "src/")
	folder := Hire{ID: "abcdef510000", Name: "Ada", Folder: keptPath(claudeTree(t, filepath.Join(base, "project"), "src/", "a/.claude/settings.json")), State: "working"}
	tree := Hire{ID: "abcdef520000", Name: "Bea", Folder: keptPath(repo), Repo: repo, Worktree: keptPath(claudeTree(t, filepath.Join(base, "wt"), "src/")), State: "working"}
	claudeRegistry(t, folder, tree)
	for _, c := range []struct {
		h     Hire
		state string
	}{{folder, "fired"}, {tree, "failed"}} {
		if _, refused := editHire(registryPath(), []byte(`{"id":"`+c.h.ID+`","state":"`+c.state+`"}`), noDeskSession); refused != nil {
			t.Fatal(refused.message)
		}
		if claudeListed(c.h.ID) {
			t.Errorf("%s: the GM's edit put it on record as listed", c.h.Name)
		}
		// What was there at the edit is listed then (Ada's a/.claude, a baseline for it): kept, it's yours.
		claudeHeld.Lock()
		ids := slices.Collect(maps.Keys(claudeHeld.plants))
		claudeHeld.Unlock()
		if c.h.ID == folder.ID && len(ids) == 0 {
			t.Errorf("%s: what was there at the edit isn't held", c.h.Name)
		}
		for _, id := range ids {
			_ = removeClaudePlant(id, true)
		}
		if why := claudeGate(c.h.storedPlace(), officeHires()); why != "" {
			t.Errorf("%s: nothing planted yet, refused: %q", c.h.Name, why)
		}
		claudeTree(t, c.h.storedPlace(), ".claude/hooks/x.sh", "src/.mcp.json")
		if why := claudeGate(c.h.storedPlace(), officeHires()); !strings.Contains(why, filepath.Join(c.h.storedPlace(), ".claude")) {
			t.Errorf("%s: planted after the edit, the gate let it through: %q", c.h.Name, why)
		}
		claudeHeld.Lock()
		for _, rel := range []string{".claude/hooks/x.sh", "src/.mcp.json"} {
			at := filepath.Join(c.h.storedPlace(), filepath.FromSlash(rel))
			if !slices.ContainsFunc(slices.Collect(maps.Values(claudeHeld.plants)), func(p claudePlant) bool { return p.File == at && p.Desk == c.h.ID }) {
				t.Errorf("%s: %s isn't held against it", c.h.Name, rel)
			}
		}
		claudeHeld.Unlock()
		if why := claudeGate(filepath.Join(c.h.storedPlace(), "src"), officeHires()); !strings.Contains(why, "won't start") {
			t.Errorf("%s: below its place: %q", c.h.Name, why)
		}
	}
}

// RT126CLAUDEPLANT1R2 B4b, option 2: at the gate, a gone hire whose own tmux session is still up counts as open,
// even one on record as listed, so what its desk plants is listed. Once its session is down, what changes there is
// yours (decision 5).
func TestAGoneHireWhoseSessionIsStillUpCountsAsOpenAtTheGate(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	up := map[string]bool{}
	upWas := claudeSessionUp
	claudeSessionUp = func(h Hire) bool { return up[h.ID] }
	t.Cleanup(func() { claudeSessionUp = upWas })
	h := Hire{ID: "abcdef530000", Name: "Cy", Folder: keptPath(claudeTree(t, filepath.Join(claudeTempDir(t), "project"), "src/")), State: "working"}
	claudeRegistry(t, h)
	if err := markFired(h.ID, ""); err != nil {
		t.Fatal(err)
	}
	claudeHireEnded(h.ID)
	if !claudeListed(h.ID) {
		t.Fatal("not on record after its session ended")
	}
	up[h.ID] = true
	claudeTree(t, h.Folder.storedOnly(), "src/.claude/settings.json")
	if why := claudeGate(h.Folder.storedOnly(), officeHires()); !strings.Contains(why, filepath.Join("src", ".claude")) {
		t.Errorf("its session up, the gate let its plant through: %q", why)
	}
	claudeHeld.Lock()
	ids := slices.Collect(maps.Keys(claudeHeld.plants))
	claudeHeld.Unlock()
	for _, id := range ids {
		_ = removeClaudePlant(id, true)
	}
	up[h.ID] = false
	claudeTree(t, h.Folder.storedOnly(), "mine/.claude/settings.json")
	if why := claudeGate(h.Folder.storedOnly(), officeHires()); why != "" {
		t.Errorf("its session down and on record, yours held: %q", why)
	}
}

// RT126CLAUDEPLANT1R2 follow-up 1: a .claude or an .mcp.json named with a trailing dot or space (which Windows reads
// without it) is listed, everywhere.
func TestATrailingDotOrSpaceDoesntHideAClaudePlant(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "l/.claude./settings.json", "m/.mcp.json ")
	got := map[string]bool{}
	for _, p := range holdClaudePlants("h1", placeLook(place), "") {
		rel, _ := filepath.Rel(place, p.File)
		got[strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]+" "+p.Why] = true
	}
	for _, want := range []string{"l " + whyClaudeSettings, "m " + whyClaudeMCP} {
		if !got[want] {
			t.Errorf("not listed: %s (listed %v)", want, got)
		}
	}
}

// RT126CLAUDEPLANT1R2's own tries, as rows: a link inside a committed .claude is listed by what it leads to, the same
// link with the same thing behind it isn't.
func TestALinkInsideACommittedClaudeIsListedByWhatItLeadsTo(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	for _, c := range []struct{ name, change, want string }{
		{"unchanged", "", ""}, {"hooks", "scripts/check.sh", ".claude/hooks"}, {"settings", "conf/s.json", ".claude/settings.json"},
	} {
		checkout := claudeTree(t, filepath.Join(base, c.name, "checkout"), "scripts/check.sh", "conf/s.json", ".claude/")
		tree := claudeTree(t, filepath.Join(base, c.name, "wt"), "scripts/check.sh", "conf/s.json", ".claude/")
		for _, root := range []string{checkout, tree} {
			claudeLink(t, filepath.Join("..", "scripts"), filepath.Join(root, ".claude", "hooks"))
			claudeLink(t, filepath.Join("..", "conf", "s.json"), filepath.Join(root, ".claude", "settings.json"))
		}
		if c.change != "" {
			if err := os.WriteFile(filepath.Join(tree, filepath.FromSlash(c.change)), []byte("#!/bin/sh\n# changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		got := listedAs(t, tree, holdClaudePlants("abcdef6"+c.name, placeLook(tree), checkout))
		want := map[string]string{}
		if c.want != "" {
			want[c.want] = claudeWhy(strings.TrimPrefix(c.want, ".claude/"))
		}
		sameListing(t, c.name, got, want)
	}
}

// RT126CLAUDEPLANT1R2's own tries: a .claude in another case is listed by what's in it, and one renamed in after a
// listing (a file renamed .MCP.JSON, a folder renamed .claude) is refused at the next gate.
func TestACaseOrARenameDoesntHideAClaudePlant(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	repo := claudeTree(t, filepath.Join(base, "repo"), ".claude/settings.json")
	h := Hire{ID: "abcdef710000", Name: "Ada", Folder: keptPath(repo), Repo: repo, Worktree: keptPath(claudeTree(t, filepath.Join(base, "wt"), "notes/draft.txt", "x/cfg/hooks/pre.sh")),
		State: "working"}
	claudeRegistry(t, h)
	if why := claudeGate(h.Worktree.storedOnly(), officeHires()); why != "" {
		t.Fatalf("clean: %q", why)
	}
	claudeTree(t, h.Worktree.storedOnly(), ".Claude/Hooks/x.sh")
	listed := listedAs(t, h.Worktree.storedOnly(), listClaudePlantsOf(h))
	if listed[".Claude/Hooks/x.sh"] != whyClaudeHooks {
		t.Errorf("another case: %v", listed)
	}
	claudeHeld.Lock()
	ids := slices.Collect(maps.Keys(claudeHeld.plants))
	claudeHeld.Unlock()
	for _, id := range ids {
		_ = removeClaudePlant(id, true)
	}
	for _, c := range []struct{ from, to string }{{"notes/draft.txt", "notes/.MCP.JSON"}, {"x/cfg", "x/.claude"}} {
		if why := claudeGate(h.Worktree.storedOnly(), officeHires()); why != "" {
			t.Fatalf("before %s: %q", c.to, why)
		}
		if err := os.Rename(filepath.Join(h.Worktree.storedOnly(), filepath.FromSlash(c.from)), filepath.Join(h.Worktree.storedOnly(), filepath.FromSlash(c.to))); err != nil {
			t.Fatal(err)
		}
		if why := claudeGate(h.Worktree.storedOnly(), officeHires()); !strings.Contains(why, filepath.Join(h.Worktree.storedOnly(), filepath.FromSlash(c.to))) {
			t.Errorf("renamed to %s: %q", c.to, why)
		}
		claudeHeld.Lock()
		ids := slices.Collect(maps.Keys(claudeHeld.plants))
		claudeHeld.Unlock()
		for _, id := range ids {
			_ = removeClaudePlant(id, true)
		}
	}
}

// RT126CLAUDEPLANT1R2's own tries: the record erased fails closed. What was held isn't, by the record alone, but the
// gate lists the gone hire again (it's no longer on record) and refuses; and a desk's Policy denies the record, a
// file next to it, and moving the support folder.
func TestTheRecordErasedFailsClosedAndADeskCantReachIt(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	h := Hire{ID: "abcdef720000", Name: "Ada", Folder: keptPath(claudeTree(t, filepath.Join(claudeTempDir(t), "project"), "src/")), State: "working"}
	claudeRegistry(t, h)
	claudeTree(t, h.Folder.storedOnly(), "src/.claude/hooks/x.sh")
	if err := markFired(h.ID, ""); err != nil {
		t.Fatal(err)
	}
	claudeHireEnded(h.ID)
	if !claudeListed(h.ID) || claudeHeldRefusal(h.Folder.storedOnly()) == "" {
		t.Fatal("fired: not on record, or nothing held")
	}
	claudeRestart()
	if !claudeListed(h.ID) {
		t.Error("on record, then not after a restart")
	}
	if err := os.Remove(filepath.Join(supportDir, claudeHeldName)); err != nil {
		t.Fatal(err)
	}
	claudeRestart()
	if claudeListed(h.ID) {
		t.Error("erased, still on record")
	}
	if why := claudeGate(h.Folder.storedOnly(), officeHires()); !strings.Contains(why, filepath.Join("src", ".claude")) {
		t.Errorf("erased, the gate let it through: %q", why)
	}

	tr := newLinkTree(t)
	pol := tr.policy(t)
	for _, c := range []struct {
		at string
		op sandbox.Op
	}{{filepath.Join(tr.support, claudeHeldName), sandbox.Write}, {filepath.Join(tr.support, claudeHeldName+".tmp"), sandbox.Write},
		{filepath.Join(tr.support, claudeHeldName), sandbox.Read}, {tr.support, sandbox.Move}} {
		if v, why := pol.Decide(c.at, c.op); v != sandbox.Deny {
			t.Errorf("%s %v: %v (%+v)", c.at, c.op, v, why)
		}
	}
}
