package sandbox

import (
	"errors"
	"slices"
	"testing"
)

// FXHIREPROJECTS (RT126FIXREVIEW-GMROOT's sibling repo; SIERRA's option (a)): in the folder for hires (~/Projects by
// default), a desk writes only its own place and its repo's shared .git. Every other repo there, its .git (hooks,
// config) and its sources, and a repo made there later, are its to read and never to write. As data, on the made-up
// Mac of policy_test.go; desktop's hireprojects_darwin_test.go runs it under the real profile.

const (
	lib       = projects + "/lib"
	worktrees = home + "/Worktrees"
)

// inProjectsPlaces is a hire in its worktree in <repo>-hires, inside the folder for hires, as deskPlaces gives it.
func inProjectsPlaces() Places {
	pl := hirePlaces(realSupport)
	pl.Projects = projects
	return pl
}

// projectsCase is one path a desk asks op of, what its Policy says, and the row that says so ("" for any).
type projectsCase struct {
	at   string
	op   Op
	want Verdict
	row  string
}

func decideEach(t *testing.T, name string, p *Policy, cases []projectsCase) {
	t.Helper()
	for _, c := range cases {
		got, by := p.Decide(c.at, c.op)
		if got != c.want || c.row != "" && by.Row != c.row {
			t.Errorf("%s: %v %s: %v by %q, want %v by %q", name, c.op, c.at, got, by.Row, c.want, c.row)
		}
	}
}

// siblingCases are what every desk held by the row is refused, and still reads, in another repo there: its shared
// git places, its sources, a repo made after the start, a new folder, and the projects folder itself.
var siblingCases = []projectsCase{
	{lib + "/.git/hooks/post-checkout", Write, Deny, rowProjects}, {lib + "/.git/hooks/pre-commit", Write, Deny, rowProjects},
	{lib + "/.git/config", Write, Deny, rowProjects}, {lib + "/.git/refs/heads/main", Write, Deny, rowProjects},
	{lib + "/main.go", Write, Deny, rowProjects}, {lib + "/.git", Write, Deny, rowProjects},
	{projects + "/later/.git/hooks/post-checkout", Write, Deny, rowProjects}, {projects + "/later/main.go", Write, Deny, rowProjects},
	{projects + "/new-project", Write, Deny, rowProjects}, {projects + "/notes.md", Write, Deny, rowProjects},
	{projects, Write, Deny, ""}, {projects, Move, Deny, ""}, {projects, Link, Deny, ""}, {lib, Move, Deny, ""},
	{lib + "/main.go", Read, Allow, ""}, {lib + "/.git/config", Read, Allow, ""}, {projects + "/notes.md", Read, Allow, ""},
}

func TestAHireInTheProjectsFolderWritesOnlyItsOwnWorktreeThere(t *testing.T) {
	cases := append([]projectsCase{
		// Its own worktree, and its repo's shared .git as before: its commits, fetches and rebases.
		{hires + "/h1/main.go", Write, Allow, "its own place"}, {hires + "/h1/pkg/new.go", Write, Allow, "its own place"},
		{gitDir + "/objects/ab/cdef", Write, Allow, "the rest of the shared .git"},
		{gitDir + "/refs/heads/hire/h1/x", Write, Allow, "the rest of the shared .git"},
		// Its fetch's FETCH_HEAD is its registration's; the shared one is held from a desk in its own worktree (FXGITROWS2R6,
		// rowGitTop; FXSTACKG G22).
		{gitDir + "/worktrees/h1/FETCH_HEAD", Write, Allow, ""}, {gitDir + "/FETCH_HEAD", Write, Deny, rowGitTop},
		{gitDir + "/worktrees/h1/rebase-merge/done", Write, Allow, ""}, {gitDir + "/worktrees/h1/index.lock", Write, Allow, ""},
		// Its repo's shared places stay held by their own rows; other desks' worktrees and the owner's checkout too.
		{gitDir + "/hooks/post-checkout", Write, Deny, "the shared git places"}, {gitDir + "/config", Write, Deny, "the shared git places"},
		{gitDir + "/refs/heads/main", Write, Deny, "the base branches"}, {hires + "/h2/main.go", Write, Deny, ""},
		{repo + "/main.go", Write, Deny, ""},
		// Its own files in the support folder, as before; the rest of it, as before.
		{realSupport + "/hires/h1/STATUS.md", Write, Allow, ""}, {realSupport + "/hires/h1/claude/todos/x.json", Write, Allow, ""},
		{realSupport + "/setup-key", Read, Deny, ""},
		// Normal work outside the projects folder.
		{home + "/Library/Caches/x", Write, Allow, ""}, {"/private/tmp/x", Write, Allow, ""},
	}, siblingCases...)
	decideEach(t, "hire in the projects folder", mustNew(t, inProjectsPlaces()), cases)
	// Without a projects folder, as before: the sibling repo is normal work.
	if got, by := mustNew(t, hirePlaces(realSupport)).Decide(lib+"/.git/hooks/post-checkout", Write); got != Allow {
		t.Errorf("with no projects folder: %v by %q, want Allow", got, by.Row)
	}
}

// FXHIREPROJECTS' cases (its brief's item 3), each as deskPlaces gives it.
func TestEachLayoutOfTheProjectsFolderKeepsADeskToItsOwnPlaceThere(t *testing.T) {
	ownPlace := func(root string) []projectsCase {
		return []projectsCase{{root + "/main.go", Write, Allow, "its own place"}, {root + "/pkg/new.go", Write, Allow, "its own place"}}
	}
	for _, c := range []struct {
		name  string
		pl    func() Places
		cases []projectsCase
		// sameKey: the projects folder is also the owner's checkout, whose row of the same place Decide names first.
		sameKey bool
	}{
		{"the hires folder inside the projects folder", inProjectsPlaces, ownPlace(hires + "/h1"), false},
		{"its worktree outside the projects folder (Setup's folder for worktrees)", func() Places {
			pl := inProjectsPlaces()
			pl.HiresFolder, pl.Root, pl.GitHooks = worktrees+"/app", worktrees+"/app/h1", nil
			return pl
		}, append(ownPlace(worktrees+"/app/h1"), projectsCase{gitDir + "/objects/ab/cdef", Write, Allow, "the rest of the shared .git"},
			projectsCase{repo + "/main.go", Write, Deny, ""}, projectsCase{hires + "/h1/main.go", Write, Deny, rowProjects},
			projectsCase{worktrees + "/app/h2/main.go", Write, Deny, rowOtherDesks}), false},
		{"the projects folder is a repo, and the hire's repo", func() Places {
			pl := inProjectsPlaces()
			pl.Checkout, pl.GitDir, pl.HiresFolder, pl.Root, pl.GitHooks = projects, projects+"/.git", projects+"-hires", projects+"-hires/h1", nil
			return pl
		}, append(ownPlace(projects+"-hires/h1"), projectsCase{projects + "/.git/objects/ab/cdef", Write, Allow, "the rest of the shared .git"},
			projectsCase{projects + "/.git/hooks/post-checkout", Write, Deny, "the shared git places"},
			projectsCase{projects + "/README", Write, Deny, ""}), true},
		{"the projects folder is a repo, the hire's in one below it", func() Places {
			pl := inProjectsPlaces()
			pl.GitHooks = nil
			return pl
		}, append(ownPlace(hires+"/h1"), projectsCase{projects + "/.git/hooks/post-checkout", Write, Deny, rowAboveGit},
			projectsCase{projects + "/.git/config", Write, Deny, rowAboveGit}), false},
		{"the GM hired into a repo worktree", func() Places {
			pl := inProjectsPlaces()
			pl.Subject, pl.OwnID, pl.Root, pl.OwnWorktree, pl.OwnSockets, pl.GMFolder = GM, "gm1", gmDir, "gm", sockets+"/gm1", ""
			pl.GitHooks = nil
			return pl
		}, append(ownPlace(gmDir), projectsCase{gitDir + "/objects/ab/cdef", Write, Allow, ""}, projectsCase{hires + "/h1/main.go", Write, Deny, ""}), false},
		{"a desk at a project in the projects folder", func() Places {
			pl := inProjectsPlaces()
			pl.Root, pl.Checkout, pl.OwnWorktree = repo, "", ""
			return pl
		}, append(ownPlace(repo), projectsCase{gitDir + "/objects/ab/cdef", Write, Allow, ""}, projectsCase{gitDir + "/hooks/x", Write, Deny, ""}), false},
		{"a test copy's hire, its support folder in the source checkout there", func() Places {
			pl := hirePlaces(copyDir)
			pl.OtherSupports, pl.Projects = []string{realSupport}, projects
			return pl
		}, append(ownPlace(hires+"/h1"), projectsCase{copyDir + "/hires/h1/STATUS.md", Write, Allow, ""},
			projectsCase{copyDir + "/hires/h1/claude/todos/x.json", Write, Allow, ""}, projectsCase{copyDir + "/hires/h1/BRIEF.md", Write, Deny, ""},
			projectsCase{copyDir + "/hires/h2/STATUS.md", Write, Deny, ""}, projectsCase{copyDir + "/setup-key", Read, Deny, ""},
			projectsCase{projects + "/smolcorp/desktop/x.go", Write, Deny, rowProjects}, projectsCase{realSupport + "/hires/h1/STATUS.md", Write, Deny, ""}), false},
	} {
		siblings := siblingCases
		if c.sameKey {
			siblings = nil
			for _, s := range siblingCases {
				s.row = ""
				siblings = append(siblings, s)
			}
		}
		decideEach(t, c.name, mustNew(t, c.pl()), append(slices.Clone(c.cases), siblings...))
	}
}

// A desk whose place is the projects folder, or holds it, or is the home folder or holds it, doesn't start in its
// sandbox on a Mac (FXHIREPROJECTSR4, SIERRA's ruling of 18 Nov: a hire writes only its own project folder): its place
// would be every repo there. The Policy refuses it whatever route made it, the projects folder itself a repo and the
// desk at its checkout too. Elsewhere it gets no row, its place all of it, as before.
func TestADeskWhosePlaceIsTheProjectsFolderOrHoldsItIsRefusedOnAMac(t *testing.T) {
	// A support folder on another disk, so a place at home or above doesn't hold it: there, as before, a place that
	// holds a support folder is refused for that first (ErrRootHasSupport).
	const otherDisk = "/Volumes/Data/AgentDeck"
	for _, c := range []struct {
		name, root, projects, checkout, support string
		want                                    error
	}{
		{"the projects folder", projects, projects, "", realSupport, ErrProjectsWhole},
		{"the projects folder, in another case", home + "/PROJECTS", projects, "", realSupport, ErrProjectsWhole},
		{"a folder that holds it", projects, projects + "/clients", "", realSupport, ErrProjectsWhole},
		{"the projects folder's own checkout", projects, projects, projects, realSupport, ErrProjectsWhole},
		{"the home folder", home, projects, "", otherDisk, ErrProjectsWhole},
		{"the home folder, with no projects folder", home, "", "", otherDisk, ErrProjectsWhole},
		{"the folder that holds home", "/Users", projects, "", otherDisk, ErrProjectsWhole},
		{"the home folder, with the support folder in it", home, projects, "", realSupport, ErrRootHasSupport},
		{"the top of the disk", "/", projects, "", otherDisk, ErrRootHasSupport},
	} {
		pl := hirePlaces(c.support)
		pl.Projects, pl.Root, pl.Checkout, pl.OwnWorktree, pl.OtherWorktrees, pl.HiresFolder, pl.GMFolder = c.projects, c.root, c.checkout, "", nil, "", ""
		pl.GitDir, pl.GitHooks = "", nil
		if c.checkout != "" {
			pl.GitDir = c.checkout + "/.git"
		}
		if _, err := build(pl); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if _, err := New(pl); !errors.Is(err, c.want) {
			t.Errorf("%s, by New: %v, want %v", c.name, err, c.want)
		}
	}
	// Its own project, or a worktree, in the projects folder, a folder beside it, and a GM's workspace are places.
	for _, root := range []string{lib, projects + "-hires/h1", home + "/Code/notes"} {
		pl := inProjectsPlaces()
		pl.Root, pl.Checkout, pl.GitDir, pl.OwnWorktree, pl.OtherWorktrees, pl.HiresFolder, pl.GMFolder, pl.GitHooks = root, "", "", "", nil, "", "", nil
		if _, err := New(pl); err != nil {
			t.Errorf("a desk at %s: %v", root, err)
		}
	}
	// On Linux, as before: no row, its place all of it.
	pl := inProjectsPlaces()
	pl.GOOS, pl.Root, pl.Checkout, pl.GitDir, pl.OwnWorktree, pl.OtherWorktrees, pl.HiresFolder, pl.GMFolder, pl.GitHooks =
		"linux", projects, "", "", "", nil, "", "", nil
	p, err := build(pl)
	if err != nil {
		t.Fatalf("on Linux: %v", err)
	}
	for _, r := range p.Rules() {
		if r.Row == rowProjects || r.Row == rowProjects+": the folders above it" {
			t.Errorf("on Linux: a row %q at %s", r.Row, r.Path)
		}
	}
	if got, by := p.Decide(lib+"/main.go", Write); got != Allow || by.Row != "its own place" {
		t.Errorf("on Linux: %v by %q, want its own place", got, by.Row)
	}
}

// The folders above the projects folder that aren't above the desk's place are held too: its worktree elsewhere.
func TestTheFoldersAboveTheProjectsFolderAreHeld(t *testing.T) {
	pl := inProjectsPlaces()
	pl.Projects = home + "/Code/clients/Projects"
	p := mustNew(t, pl)
	for _, folder := range []string{home + "/Code/clients", home + "/Code"} {
		for _, op := range []Op{Write, Move, Link} {
			if got, by := p.Decide(folder, op); got != Deny {
				t.Errorf("%v %s: %v by %q, want Deny", op, folder, got, by.Row)
			}
		}
	}
	if got, by := p.Decide(home+"/Code/other/x", Write); got != Allow {
		t.Errorf("beside them: %v by %q, want Allow", got, by.Row)
	}
}

func TestAProjectsFolderIsRefusedWhereItCantBeOne(t *testing.T) {
	for _, c := range []struct {
		name string
		pl   func() Places
		want error
	}{
		{"a hire in it", inProjectsPlaces, nil},
		{"the GM in a repo, in it", func() Places { pl := inProjectsPlaces(); pl.Subject, pl.OwnID, pl.Root = GM, "gm1", gmDir; return pl }, nil},
		{"the GM with projects of its own", func() Places { pl := projectsGMPlaces(); pl.Projects = projects; return pl }, ErrProjects},
		{"the home folder", func() Places { pl := inProjectsPlaces(); pl.Projects = home; return pl }, ErrProjects},
		{"above the home folder", func() Places { pl := inProjectsPlaces(); pl.Projects = "/Users"; return pl }, ErrProjects},
		{"in the support folder", func() Places { pl := inProjectsPlaces(); pl.Projects = realSupport + "/P"; return pl }, ErrProjects},
		{"in another support folder", func() Places {
			pl := inProjectsPlaces()
			pl.OtherSupports, pl.Projects = []string{copyDir}, copyDir+"/P"
			return pl
		}, ErrProjects},
		{"not whole", func() Places { pl := inProjectsPlaces(); pl.Projects = "Projects"; return pl }, ErrNotWhole},
	} {
		if _, err := New(c.pl()); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}
