package sandbox

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// FXRC3FIX (RT126RC3's B1): the folders whose OpenCode names a desk's Policy holds are each folder above the support
// folders, up to and with home, and on a Mac those above Smolcorp's own support folder there; none outside home.
func TestOpenCodesConfigIsHeldInEachFolderAboveTheSupportFolder(t *testing.T) {
	home := "/Users/ada"
	for _, c := range []struct {
		goos     string
		supports []string
		want     []string
	}{
		{"darwin", []string{home + "/Library/Application Support/AgentDeck"},
			[]string{home + "/Library/Application Support", home + "/Library", home}},
		{"darwin", []string{home + "/AD", home + "/x/y/AD", "/private/tmp/AD", home},
			[]string{home, home + "/x/y", home + "/x", home + "/Library/Application Support", home + "/Library"}},
		{"linux", []string{"/home/ada/.local/share/smolcorp"}, []string{"/home/ada/.local/share", "/home/ada/.local", "/home/ada"}},
	} {
		h := home
		if c.goos == "linux" {
			h = "/home/ada"
		}
		if got := openCodeConfigFolders(c.goos, h, c.supports); !slices.Equal(got, c.want) {
			t.Errorf("%s %v: %v, not %v", c.goos, c.supports, got, c.want)
		}
	}

	// Each one, in a Mac hire's Policy: each name a write deny, the files with the owner's names beside them, the
	// folders with their links row.
	s := profileSetups()[0]
	p := mustNew(t, s.pl)
	for _, folder := range []string{"Library/Application Support", "Library", ""} {
		for _, name := range openCodeConfigNames {
			at := strings.TrimSuffix(s.pl.Home+"/"+folder, "/") + "/" + name
			if !slices.ContainsFunc(p.rules, func(r Rule) bool { return r.Path == at && r.Row == rowAgents && r.Write == Deny }) {
				t.Errorf("%s: no write deny at %s", s.name, at)
			}
			if strings.HasPrefix(name, "opencode.") {
				if !slices.ContainsFunc(p.rules, func(r Rule) bool { return r.Path == at+".lock" && r.Write == Deny }) {
					t.Errorf("%s: no write deny beside %s", s.name, at)
				}
			} else if !slices.ContainsFunc(p.rules, func(r Rule) bool { return r.Path == at && r.Row == rowAgentLinks && r.Link == Deny }) {
				t.Errorf("%s: no links row at %s", s.name, at)
			}
		}
	}
}

// FXRC3FIXR1 (RT126RC3FIX's N1): in each of those folders below home, a Mac desk's Policy holds .git and .hg whole, as
// a folder or a file, and HEAD beside them; at home, .hg (its .git and HEAD are rowHomeGit's). A .git holding the
// desk's own repo keeps that repo's rows: a test copy's hire on the checkout its support folder is in.
func TestOpenCodesProjectMarksAreHeldInEachFolderAboveTheSupportFolder(t *testing.T) {
	held := func(p *Policy, at string, below bool) bool {
		return slices.ContainsFunc(p.rules, func(r Rule) bool {
			return r.Path == at && r.Row == rowAboveGit && r.Write == Deny && r.Below == below
		})
	}
	for _, s := range profileSetups() {
		p := mustNew(t, s.pl)
		for _, folder := range []string{home + "/Library/Application Support", home + "/Library"} {
			for _, name := range []string{".git", ".hg"} {
				if !held(p, folder+"/"+name, true) {
					t.Errorf("%s: %s/%s isn't held whole", s.name, folder, name)
				}
			}
			if !held(p, folder+"/HEAD", false) {
				t.Errorf("%s: %s/HEAD isn't held", s.name, folder)
			}
		}
		if !held(p, home+"/.hg", true) {
			t.Errorf("%s: ~/.hg isn't held whole", s.name)
		}
		if held(p, home+"/.git", true) {
			t.Errorf("%s: ~/.git is held here too, not only as rowHomeGit", s.name)
		}
		var twice []string
		for i, r := range p.rules {
			if r.Row == rowAboveGit && slices.ContainsFunc(p.rules[:i], func(o Rule) bool { return o.Row == rowAboveGit && o.Path == r.Path }) {
				twice = append(twice, r.Path)
			}
		}
		if len(twice) > 0 {
			t.Errorf("%s: held twice: %v", s.name, twice)
		}
	}

	// A test copy's hire whose shared .git is the one of the checkout its support folder is in (a worktree of that
	// repo; the checkout itself is never a desk's place, ErrRootHasSupport): that .git and HEAD beside it are its own
	// repo's, its .hg is held; the folders between are held whole.
	smolcorp := home + "/Projects/smolcorp"
	pl := hirePlaces(smolcorp + "/.dev/support")
	pl.OtherSupports = []string{realSupport}
	pl.Checkout, pl.GitDir, pl.Root = "", smolcorp+"/.git", smolcorp+"-hires/h1"
	pl.HiresFolder, pl.GMFolder = smolcorp+"-hires", smolcorp+"-gm"
	pl.GitHooks = nil
	p := mustNew(t, pl)
	if held(p, smolcorp+"/.git", true) || held(p, smolcorp+"/HEAD", false) {
		t.Errorf("a test copy's hire on its own checkout: that checkout's .git or HEAD is held as a plant")
	}
	for _, at := range []string{smolcorp + "/.hg", smolcorp + "/.dev/.git", smolcorp + "/.dev/.hg"} {
		if !held(p, at, true) {
			t.Errorf("a test copy's hire: %s isn't held whole", at)
		}
	}
	if v, _ := p.Decide(smolcorp+"/.git/objects/ab/cd", Write); v == Deny {
		t.Errorf("a test copy's hire: its own repo's objects are denied")
	}
}

// FXRC3FIXR1 (the Studio's run of FXRC3FIX): the OpenCode files above the support folder are held with their own
// writers' names only. A writer's random name in those folders (install(1)'s INS@ and six, perl -i's XX and six) stays
// a desk's own, in ~/Library, ~/Library/Application Support and a test copy's folders up to ~/Projects: those
// folders hold the projects folder and ~, where such a name is any file's. Names that are the file's own are still
// held. And a folder above the support folder that is a place the desk works in, the projects folder here, gets none
// of these rows.
func TestOpenCodesFilesAboveTheSupportFolderHoldOnlyTheirOwnWritersNames(t *testing.T) {
	lib, as := home+"/Library", home+"/Library/Application Support"
	for _, s := range setups() {
		p := mustNew(t, s.pl)
		for _, c := range []struct {
			at string
			v  Verdict
		}{
			{lib + "/INS@q4Ontc", Allow}, {lib + "/XXNu6Xnm", Allow}, {as + "/INS@q4Ontc", Allow}, {as + "/XXNu6Xnm", Allow},
			{home + "/XXNu6Xnm", Allow}, {home + "/Projects/INS@q4Ontc", Allow}, {home + "/Projects/XXNu6Xnm", Allow},
			{home + "/Projects/smolcorp/INS@q4Ontc", Allow}, {home + "/Projects/smolcorp/.dev/XXNu6Xnm", Allow},
			{lib + "/opencode.json", Deny}, {lib + "/opencode.json.lock", Deny}, {lib + "/.!12!opencode.json", Deny},
			{as + "/.opencode.jsonc.tmp.1.2", Deny}, {as + "/opencode.json.tmp1", Deny}, {home + "/opencode.jsonc.lock", Deny},
		} {
			if s.name != "test copy's hire" && strings.HasPrefix(c.at, home+"/Projects/smolcorp") {
				continue
			}
			if v, r := p.Decide(c.at, Write); v != c.v {
				t.Errorf("%s: write %s: %v by %q, want %v", s.name, c.at, v, r.Row, c.v)
			}
		}
	}

	// A test copy's hire with ~/Projects as its projects folder: no OpenCode or project-mark row of this hold there.
	s := setups()[2]
	s.pl.Projects = home + "/Projects"
	for _, r := range mustNew(t, s.pl).rules {
		// The projects folder's own .git and HEAD are its rows' (rowAboveGit, FXHIREPROJECTSR2): no .hg is.
		if path.Dir(r.Path) == home+"/Projects" && (r.Row == rowAboveGit && path.Base(r.Path) == ".hg" ||
			r.Row == rowAgents && slices.ContainsFunc(openCodeConfigNames, func(n string) bool { return strings.Contains(path.Base(r.Path), n) })) {
			t.Errorf("a row in the projects folder: %+v", r)
		}
	}
}
