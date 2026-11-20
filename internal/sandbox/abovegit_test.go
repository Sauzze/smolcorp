package sandbox

import (
	"strings"
	"testing"
)

// FXHIREPROJECTSR2 (FXHIREPROJECTSR1's Open 1, 3 and 4): no desk makes a .git, or a HEAD that makes a bare repo, in a
// folder above its place, the folder for hires or a GM's start folder, which the owner's git (or the GM's) takes for
// every folder below it in no repo; and a GM's start folder that holds a desk's places is held as the folder for hires
// is. As data, on the made-up Mac of policy_test.go; desktop's hireprojectsr2_darwin_test.go runs it under the real
// profile.

const (
	code        = home + "/Code"
	clients     = code + "/clients"
	clientsApp  = clients + "/app"
	clientsHire = clients + "/app-hires/h1"
	work        = home + "/Work"
)

// gitAbove is what no desk writes in folder, which isn't its own place: a .git there (a folder, a gitfile, a link, and
// what is in an owner's), and a HEAD, in any spelling.
func gitAbove(folder string) []projectsCase {
	upper := folder + "/.GIT"
	return []projectsCase{
		{folder + "/.git", Write, Deny, rowAboveGit}, {folder + "/.git/HEAD", Write, Deny, rowAboveGit},
		{folder + "/.git/config", Write, Deny, rowAboveGit}, {folder + "/.git/objects/ab/cd", Write, Deny, rowAboveGit},
		{upper + "/hooks/post-checkout", Write, Deny, rowAboveGit}, {folder + "/HEAD", Write, Deny, rowAboveGit},
		{folder + "/head", Write, Deny, rowAboveGit}, {folder + "/.git", Move, Deny, ""}, {folder + "/.git", Link, Deny, ""},
		{folder + "/.git/config", Read, Allow, ""},
	}
}

// clientsPlaces is a hire in a worktree of ~/Code/clients/app, the folder for hires ~/Code/clients (Open 3).
func clientsPlaces() Places {
	pl := hirePlaces(realSupport)
	pl.Root, pl.Checkout, pl.GitDir, pl.HiresFolder, pl.Projects = clientsHire, clientsApp, clientsApp+"/.git", clients+"/app-hires", clients
	pl.GMFolder, pl.GitHooks = "", nil
	return pl
}

// Open 3: the folders between ~ and the hire's place, ~/Code above the folder for hires among them. The rest of them is
// normal work as before. A .git the hire makes below its own place is refused since FXHIREPROJECTSR6 (gitbelow_test.go);
// a HEAD there is the core's walk's, at its next start.
func TestNoDeskMakesAGitFolderBetweenHomeAndItsPlace(t *testing.T) {
	p := mustNew(t, clientsPlaces())
	cases := append(append(append(gitAbove(code), gitAbove(clients)...), gitAbove(clients+"/app-hires")...),
		projectsCase{code + "/notes.md", Write, Allow, ""}, projectsCase{code + "/config", Write, Allow, ""},
		projectsCase{code + "/objects/x", Write, Allow, ""}, projectsCase{code + "/refs/x", Write, Allow, ""},
		projectsCase{code + "/.gitignore", Write, Allow, ""}, projectsCase{code + "/other/.git/config", Write, Allow, ""},
		projectsCase{clientsHire + "/main.go", Write, Allow, "its own place"}, projectsCase{clientsHire + "/sub/.git/config", Write, Deny, rowGitBelow},
		projectsCase{clientsHire + "/sub/HEAD", Write, Allow, ""}, projectsCase{clientsApp + "/.git/objects/ab/cd", Write, Allow, ""},
		projectsCase{clientsApp + "/.git/refs/heads/hire/h1", Write, Allow, ""})
	decideEach(t, "a hire in ~/Code/clients", p, cases)
	// Its worktree elsewhere: the folders above the owner's checkout and the folder for hires are held all the same.
	pl := clientsPlaces()
	pl.Root, pl.HiresFolder = worktrees+"/app-hires/h1", worktrees+"/app-hires"
	decideEach(t, "its worktree elsewhere", mustNew(t, pl), append(append(gitAbove(code), gitAbove(clients)...), gitAbove(worktrees)...))
	// Nothing is asked of Hold for them: a .git and a HEAD above are denied, and the folders are held already.
	for _, folder := range p.Hold() {
		if strings.HasSuffix(folder, "/.git") || strings.HasSuffix(folder, "/HEAD") {
			t.Errorf("Hold asks for %s", folder)
		}
	}
}

// A desk in a repo above its place keeps its shared .git's own rows: the .git above it isn't denied whole. (R2's Open 4,
// a plain hire whose place is the folder for hires, doesn't start on a Mac since FXHIREPROJECTSR4: projects_test.go.)
func TestADeskInARepoAboveItsPlaceKeepsItsSharedGit(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.Root, pl.Checkout, pl.GitDir, pl.OwnWorktree, pl.OtherWorktrees, pl.HiresFolder, pl.GMFolder, pl.GitHooks =
		code+"/notes", "", code+"/.git", "", nil, "", "", nil
	decideEach(t, "a desk in a repo above its place", mustNew(t, pl), []projectsCase{
		{code + "/.git/objects/ab/cd", Write, Allow, "the rest of the shared .git"}, {code + "/.git/config", Write, Deny, "the shared git places"},
	})
}

// workHirePlaces is a hire under a GM with a workspace hired into ~/Work, a folder of repos that holds the hire's
// worktree and isn't the folder for hires (Open 1).
func workHirePlaces() Places {
	pl := hirePlaces(realSupport)
	pl.Root, pl.Checkout, pl.GitDir, pl.HiresFolder, pl.Projects = work+"/app-hires/h1", work+"/app", work+"/app/.git", work+"/app-hires", projects
	pl.GMFolder, pl.GMStart, pl.GMStartPlans, pl.GitHooks = gmWorkspace, work, work+"/.smolcorp/plans/gm1", nil
	return pl
}

// Open 1: there, the hire writes only its own worktree and its repo's shared .git, as in the folder for hires, and
// nothing the GM loads at the top: its playbook and briefs, a .claude's contents, a .git and a HEAD.
func TestAHireUnderAGMInAFolderOfReposMakesNothingThereTheGMLoads(t *testing.T) {
	p := mustNew(t, workHirePlaces())
	var cases []projectsCase
	for _, at := range gmLoads(work) {
		cases = append(cases, projectsCase{at, Write, Deny, loadsRow(work, at, rowProjects)})
	}
	cases = append(cases,
		projectsCase{work + "/.claude", Write, Deny, ""}, projectsCase{work + "/.claude", Move, Deny, ""},
		projectsCase{work + "/lib/main.go", Write, Deny, rowProjects}, projectsCase{work + "/lib/.git/hooks/post-checkout", Write, Deny, rowProjects},
		projectsCase{work + "/CLAUDE.md", Write, Deny, ""}, projectsCase{work + "/.smolcorp/plans/gm1/p.md", Write, Deny, ""},
		projectsCase{work, Write, Deny, ""}, projectsCase{work, Move, Deny, ""},
		projectsCase{work + "/PLAYBOOK.md", Read, Allow, ""}, projectsCase{work + "/.claude/agents/a.md", Read, Allow, ""},
		projectsCase{work + "/app-hires/h1/main.go", Write, Allow, "its own place"},
		projectsCase{work + "/app/.git/objects/ab/cd", Write, Allow, "the rest of the shared .git"},
		projectsCase{work + "/app/.git/config", Write, Deny, "the shared git places"},
		projectsCase{realSupport + "/hires/h1/STATUS.md", Write, Allow, ""}, projectsCase{home + "/Library/Caches/x", Write, Allow, ""})
	decideEach(t, "a hire under a GM in ~/Work", p, cases)
	// Hold asks for nothing there but in its repo's shared .git, as in the folder for hires.
	for _, folder := range p.Hold() {
		if k := key("darwin", folder); under(k, key("darwin", work)) && !under(k, key("darwin", work+"/app/.git")) {
			t.Errorf("Hold asks for %s, in the GM's start folder", folder)
		}
	}
}

// …but not where the row would take what a desk's work needs that no exception of its opens: a GM started at home, or
// in the folder that holds the support folder. Nor where the folder for hires holds it already, nor for the desk whose
// place it is.
func TestAGMsStartFolderIsHeldAsTheProjectsFolderOnlyWhereItCanBe(t *testing.T) {
	for _, c := range []struct {
		name  string
		start string
		root  string // "" for the hire's in ~/Work
	}{
		{"home", home, ""}, {"the support folder's", home + "/Library", ""},
		{"inside the folder for hires", projects + "/app", hires + "/h1"}, {"the desk's own place", work + "/app-hires/h1", ""},
	} {
		pl := workHirePlaces()
		pl.GMStart, pl.GMStartPlans = c.start, ""
		if c.root != "" {
			pl.Root, pl.Checkout, pl.GitDir, pl.HiresFolder = c.root, repo, gitDir, hires
		}
		for _, r := range mustNew(t, pl).Rules() {
			if r.Row == rowProjects && key("darwin", r.Path) == key("darwin", c.start) {
				t.Errorf("%s: the GM's start folder %s is held as the folder for hires", c.name, c.start)
			}
		}
	}
	p := mustNew(t, func() Places { pl := workHirePlaces(); pl.GMStart, pl.GMStartPlans = home+"/Library", ""; return pl }())
	decideEach(t, "a GM in ~/Library", p, []projectsCase{{home + "/Library/Caches/x", Write, Allow, ""}})
}

// Linux's view isn't changed here, as ~'s isn't.
func TestOnLinuxNoRowHoldsAGitFolderAbove(t *testing.T) {
	pl := clientsPlaces()
	pl.GOOS = "linux"
	for _, r := range mustNew(t, pl).Rules() {
		if r.Row == rowAboveGit {
			t.Errorf("Linux's Policy has %q at %s", r.Row, r.Path)
		}
	}
}
