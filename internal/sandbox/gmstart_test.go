package sandbox

import "testing"

// FXHIREPROJECTSR1 (RT126FIXREVIEW-GMROOTR1 N1): what the GM's Claude Code loads from the folder it starts in, and the
// git folder at home git takes for any folder below it in no repo. As data, on the made-up Mac of policy_test.go;
// desktop's hireprojectsr1_darwin_test.go runs it under the real profile.

const (
	gmHome      = home + "/gm-home"
	gmHomePlans = gmHome + "/.smolcorp/plans/gm1"
	gmFolderRow = "other desks' places: the GM's folder"
)

// ownFolderHirePlaces is a hire under a GM with a workspace that started in a folder of its own, outside the projects
// folder, as deskPlaces gives it: the folder holds none of the hire's places.
func ownFolderHirePlaces() Places {
	pl := inProjectsPlaces()
	pl.GMFolder, pl.GMStart, pl.GMStartPlans = gmWorkspace, gmHome, gmHomePlans
	return pl
}

// gmLoads are what the GM's Claude Code loads from where it starts, besides the agent files N2 holds, and what git
// takes for a repo there: its playbook and briefs, a .claude's contents, a .git, a bare repo's HEAD, and a new file.
func gmLoads(start string) []string {
	return []string{start + "/PLAYBOOK.md", start + "/brief-h1.md", start + "/.claude/agents/x.md", start + "/.claude/commands/x.md",
		start + "/.claude/CLAUDE.md", start + "/.claude/skills", start + "/.git",
		start + "/.git/HEAD", start + "/.git/objects", start + "/.git/refs", start + "/.git/config", start + "/HEAD", start + "/notes.md"}
}

// loadsRow is the row that denies at, one of gmLoads(start): a .git or a HEAD there is held by its own, narrower row
// since FXHIREPROJECTSR2, the folder's row the rest.
func loadsRow(start, at, row string) string {
	if at == start+"/HEAD" || under(at, start+"/.git") {
		return rowAboveGit
	}
	return row
}

// N1 (a): the GM in a folder of its own. A hire is kept out of all of it, as the base kept it out of the GM's folder,
// and reads it still. Its own worktree stays its own.
func TestAHireIsKeptOutOfAGMsOwnStartFolderWhole(t *testing.T) {
	p := mustNew(t, ownFolderHirePlaces())
	var cases []projectsCase
	for _, at := range gmLoads(gmHome) {
		cases = append(cases, projectsCase{at, Write, Deny, loadsRow(gmHome, at, gmFolderRow)})
	}
	cases = append(cases,
		projectsCase{gmHome, Write, Deny, gmFolderRow}, projectsCase{gmHome, Move, Deny, ""}, projectsCase{gmHome, Link, Deny, ""},
		projectsCase{gmHome + "/CLAUDE.md", Write, Deny, ""}, projectsCase{gmHomePlans + "/p.md", Write, Deny, ""},
		projectsCase{gmHome + "/PLAYBOOK.md", Read, Allow, ""}, projectsCase{gmHome + "/.claude/agents/x.md", Read, Allow, ""},
		projectsCase{hires + "/h1/own.txt", Write, Allow, "its own place"}, projectsCase{hires + "/h1/.claude/agents/x.md", Write, Allow, ""},
		projectsCase{gmWorkspace + "/notes.md", Write, Deny, ""})
	decideEach(t, "own folder", p, cases)
	// It holds what the base held for a hire there: no Hold is asked for it, nor for any folder in it.
	for _, folder := range p.Hold() {
		if under(key("darwin", folder), key("darwin", gmHome)) {
			t.Errorf("Hold asks for %s, in the GM's start folder", folder)
		}
	}
}

// …and only there: a start folder that holds any of the desk's own places gets no such row, so the desk keeps them.
func TestAGMsStartFolderThatHoldsADesksPlacesIsNotDeniedWhole(t *testing.T) {
	for _, c := range []struct {
		name  string
		start string
		at    string // a place of the desk's in it, which stays its own ("" for none to write)
	}{
		{"its worktree", home + "/Projects", hires + "/h1/main.go"},
		{"its checkout and shared .git", repo, gitDir + "/objects/ab/cd"},
		{"its support folder", home + "/Library", ""},
		{"its socket folder", "/private/tmp", sockets + "/h1/x"},
	} {
		pl := hirePlaces(realSupport)
		pl.GMFolder, pl.GMStart, pl.GMStartPlans = gmWorkspace, c.start, ""
		p := mustNew(t, pl)
		for _, r := range p.Rules() {
			if r.Row == gmFolderRow && key("darwin", r.Path) == key("darwin", c.start) {
				t.Errorf("%s: a start folder at %s, which holds it, is denied whole", c.name, c.start)
			}
		}
		if got, by := p.Decide(c.at, Write); c.at != "" && got != Allow {
			t.Errorf("%s: write %s: %v by %q", c.name, c.at, got, by.Row)
		}
	}
}

// N1 (b): the GM in the projects folder (the default). A hire held to its own worktree there is refused everything the
// GM loads at its top, an owner's existing .claude included, and a .git or a bare repo's HEAD there.
func TestAHireUnderTheGMInTheProjectsFolderMakesNothingThereTheGMLoads(t *testing.T) {
	pl := inProjectsPlaces()
	pl.GMFolder, pl.GMStart, pl.GMStartPlans = gmWorkspace, projects, gmPlans
	p := mustNew(t, pl)
	var cases []projectsCase
	for _, at := range gmLoads(projects) {
		cases = append(cases, projectsCase{at, Write, Deny, loadsRow(projects, at, rowProjects)})
	}
	cases = append(cases, projectsCase{projects + "/.claude", Write, Deny, ""}, projectsCase{projects + "/.claude", Move, Deny, ""},
		projectsCase{projects + "/.claude/commands/x.md", Read, Allow, ""}, projectsCase{hires + "/h1/own.txt", Write, Allow, "its own place"})
	decideEach(t, "projects", p, cases)
}

// "Seen outside" in RT126FIXREVIEW-GMROOTR1: no desk on a Mac makes ~/.git, a link or a file there, nor a HEAD in ~
// that makes ~ a bare repo, nor writes an owner's ~/.git; it reads one, and the rest of ~ is normal work as before.
func TestNoDeskMakesAGitFolderAtHome(t *testing.T) {
	all := append(setups(), setup{"GM in the projects folder", projectsGMPlaces()}, setup{"hire under it", projectsHirePlaces()},
		setup{"hire in the projects folder", inProjectsPlaces()}, setup{"hire under a GM in its own folder", ownFolderHirePlaces()})
	for _, s := range all {
		p := mustNew(t, s.pl)
		decideEach(t, s.name, p, []projectsCase{
			{home + "/.git", Write, Deny, rowHomeGit}, {home + "/.git", Move, Deny, ""}, {home + "/.git", Link, Deny, ""},
			{home + "/.git/HEAD", Write, Deny, rowHomeGit}, {home + "/.git/config", Write, Deny, rowHomeGit},
			{home + "/.git/objects/ab/cd", Write, Deny, rowHomeGit}, {home + "/.GIT/hooks/post-checkout", Write, Deny, rowHomeGit},
			{home + "/HEAD", Write, Deny, rowHomeGit}, {home + "/head", Write, Deny, rowHomeGit},
			{home + "/.git/config", Read, Allow, ""},
			{home + "/notes.md", Write, Allow, ""}, {home + "/.gitignore_global", Write, Allow, ""}, {home + "/config", Write, Allow, ""},
		})
	}
	// A desk whose repo it is (a desk at an owner's home repo) keeps its shared .git's own rows; Linux's view is as before.
	at := hirePlaces(realSupport)
	at.Root, at.Checkout, at.GitDir, at.OwnWorktree, at.OtherWorktrees, at.HiresFolder = home+"/notes", "", home+"/.git", "", nil, ""
	decideEach(t, "a desk at the home repo", mustNew(t, at), []projectsCase{
		{home + "/.git/objects/ab/cd", Write, Allow, "the rest of the shared .git"}, {home + "/.git/config", Write, Deny, "the shared git places"},
	})
	linux := hirePlaces(realSupport)
	linux.GOOS = "linux"
	for _, r := range mustNew(t, linux).Rules() {
		if r.Row == rowHomeGit {
			t.Errorf("Linux's Policy has %q at %s", r.Row, r.Path)
		}
	}
}
