package sandbox

import (
	"errors"
	"testing"
)

// FXGMROOT (RT126SWEEP2 P1 B1): the GM hired into the folder for hires, ~/Projects by default, which isn't a repo and
// holds the repos, their hires' worktrees and the owner's checkouts. Its Root is a workspace of its own in its folder
// in the support folder; the projects folder is its to read and never to write, but for its own plans; and a hire
// under it is kept out of the workspace alone, not out of the whole projects folder its own worktree is in. As data,
// on the made-up Mac of policy_test.go; desktop's gmroot_darwin_test.go runs it under the real profile.

const (
	projects    = home + "/Projects"
	gmWorkspace = realSupport + "/hires/gm1/work"
	gmPlans     = projects + "/.smolcorp/plans/gm1"
)

// projectsGMPlaces is the GM hired into the projects folder, as desktop's deskPlaces gives it.
func projectsGMPlaces() Places {
	pl := hirePlaces(realSupport)
	pl.Subject, pl.OwnID, pl.OwnSockets, pl.Root = GM, "gm1", sockets+"/gm1", gmWorkspace
	pl.Checkout, pl.GitDir, pl.OwnWorktree, pl.OtherWorktrees, pl.DefaultBranch, pl.HiresFolder, pl.GMFolder = "", "", "", nil, "", "", ""
	pl.GMProjects, pl.GMPlans = projects, gmPlans
	return pl
}

// projectsHirePlaces is a hire under that GM, as deskPlaces gives it (FXGMROOTR1 N2): kept out of the GM's workspace,
// and out of what the GM reads from the folder it starts in.
func projectsHirePlaces() Places {
	pl := hirePlaces(realSupport)
	pl.GMFolder, pl.GMStart, pl.GMStartPlans = gmWorkspace, projects, gmPlans
	return pl
}

func TestTheGMInTheProjectsFolderReadsThemAndWritesOnlyItsWorkspaceAndPlans(t *testing.T) {
	p := mustNew(t, projectsGMPlaces())
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
		row  string
	}{
		// Every repo's shared git places, its base branches, a hire's worktree, the owner's checkout files.
		{gitDir + "/hooks/post-checkout", Write, Deny, rowGMProjects}, {gitDir + "/config", Write, Deny, rowGMProjects},
		{gitDir + "/refs/heads/develop", Write, Deny, rowGMProjects}, {gitDir + "/refs/heads/main", Write, Deny, rowGMProjects},
		{gitDir + "/refs/heads/hire/h1", Write, Deny, rowGMProjects}, {projects + "/lib/.git/hooks/pre-commit", Write, Deny, rowGMProjects},
		{hires + "/h1/main.go", Write, Deny, rowGMProjects}, {repo + "/main.go", Write, Deny, rowGMProjects},
		{repo + "/.claude/commands/x.md", Write, Deny, ""}, {projects + "/CLAUDE.md", Write, Deny, rowGMProjects},
		{projects + "/.mcp.json", Write, Deny, rowGMProjects},
		// No new folder there, nor the folder itself moved, removed, linked or changed.
		{projects + "/new-project", Write, Deny, rowGMProjects}, {projects, Write, Deny, ""}, {projects, Move, Deny, ""},
		{projects, Link, Deny, ""}, {home, Move, Deny, ""},
		// Core's folder there: only its own plans, not the folder they hang from, nor another desk's.
		{projects + "/.smolcorp", Write, Deny, rowGMProjects}, {projects + "/.smolcorp/.gitignore", Write, Deny, rowGMProjects},
		{projects + "/.smolcorp/plans/h1/plan.md", Write, Deny, rowGMProjects},
		{gmPlans + "/calm-river.md", Write, Allow, rowGMProjects + ": its own plans"}, {gmPlans + "/calm-river.md", Read, Allow, ""},
		{gmPlans + "/../h1/x.md", Write, Deny, rowGMProjects},
		// Reads stay as the profile has them.
		{repo + "/main.go", Read, Allow, ""}, {gitDir + "/config", Read, Allow, ""}, {hires + "/h1/main.go", Read, Allow, ""},
		// Its workspace is its own; the folder itself stays in place.
		{gmWorkspace + "/notes.md", Write, Allow, ""}, {gmWorkspace + "/notes.md", Read, Allow, ""},
		{gmWorkspace + "/team/h1.md", Write, Allow, ""}, {gmWorkspace, Move, Deny, ""}, {gmWorkspace, Write, Deny, ""},
		// The rest of its folder, and the support folder, as before.
		{realSupport + "/hires/gm1/settings.json", Write, Deny, ""}, {realSupport + "/hires/gm1/STATUS.md", Write, Allow, ""},
		{realSupport + "/hires/gm1/claude/settings.json", Write, Deny, ""}, {realSupport + "/hires/gm1/claude/todos/x.json", Write, Allow, ""},
		{realSupport + "/hires/gm1/x.json", Write, Deny, ""}, {realSupport + "/hires/h1/work/x", Write, Deny, ""},
		{realSupport + "/setup-key", Read, Deny, ""}, {realSupport + "/gm-memory/x", Read, Deny, ""},
		// Normal work, and the owner's plants.
		{home + "/Library/Caches/x", Write, Allow, ""}, {"/private/tmp/x", Write, Allow, ""}, {home + "/.zshrc", Write, Deny, ""},
	} {
		got, by := p.Decide(c.at, c.op)
		if got != c.want || c.row != "" && by.Row != c.row {
			t.Errorf("%v %s: %v by %q, want %v by %q", c.op, c.at, got, by.Row, c.want, c.row)
		}
	}
}

func TestAHireUnderTheGMWritesItsWorktreeAndNotTheGMsWorkspace(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.GMFolder = gmWorkspace
	p := mustNew(t, pl)
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
	}{
		{hires + "/h1/main.go", Write, Allow}, {hires + "/h1/pkg/new.go", Write, Allow}, {hires + "/h2/main.go", Write, Deny},
		{gmWorkspace + "/notes.md", Write, Deny}, {gmWorkspace + "/notes.md", Read, Deny}, {gmWorkspace, Move, Deny},
		{repo + "/main.go", Write, Deny}, {gitDir + "/hooks/post-checkout", Write, Deny},
	} {
		if got, by := p.Decide(c.at, c.op); got != c.want {
			t.Errorf("%v %s: %v by %q, want %v", c.op, c.at, got, by.Row, c.want)
		}
	}
}

// The workspace is the one place in a support folder a desk works in: the GM's Root with its projects folder, and
// another desk's GMFolder. Anything else there, or a projects folder given otherwise, is refused.
func TestOnlyTheGMsOwnWorkspaceIsAPlaceInTheSupportFolder(t *testing.T) {
	for _, c := range []struct {
		name string
		pl   func() Places
		want error
	}{
		{"the GM in the projects folder", projectsGMPlaces, nil},
		{"no plans folder", func() Places { pl := projectsGMPlaces(); pl.GMPlans = ""; return pl }, nil},
		{"a hire with the GM's workspace for its GMFolder", func() Places { pl := hirePlaces(realSupport); pl.GMFolder = gmWorkspace; return pl }, nil},
		{"spelled otherwise", func() Places {
			pl := projectsGMPlaces()
			pl.Root = "/SYSTEM/Volumes/Data/users/ADA/Library/Application Support/agentdeck/HIRES/gm1/Work"
			return pl
		}, nil},
		{"a hire with projects", func() Places { pl := projectsGMPlaces(); pl.Subject = Hire; return pl }, ErrGMProjects},
		{"another desk's workspace for its Root", func() Places { pl := projectsGMPlaces(); pl.Root = realSupport + "/hires/h1/work"; return pl }, ErrGMProjects},
		{"its own folder for its Root", func() Places { pl := projectsGMPlaces(); pl.Root = realSupport + "/hires/gm1"; return pl }, ErrGMProjects},
		{"the projects folder for its Root", func() Places { pl := projectsGMPlaces(); pl.Root = projects; return pl }, ErrGMProjects},
		{"plans outside the projects", func() Places { pl := projectsGMPlaces(); pl.GMPlans = home + "/plans"; return pl }, ErrGMProjects},
		{"plans the projects folder", func() Places { pl := projectsGMPlaces(); pl.GMPlans = projects; return pl }, ErrGMProjects},
		{"plans with no projects", func() Places { pl := hirePlaces(realSupport); pl.GMPlans = gmPlans; return pl }, ErrGMProjects},
		{"projects holding the support folder", func() Places { pl := projectsGMPlaces(); pl.GMProjects, pl.GMPlans = home, ""; return pl }, ErrRootHasSupport},
		{"projects in the support folder", func() Places {
			pl := projectsGMPlaces()
			pl.GMProjects, pl.GMPlans = realSupport+"/p", ""
			return pl
		}, ErrRootInSupport},
		{"a workspace for a hire's Root", func() Places { pl := hirePlaces(realSupport); pl.Root = realSupport + "/hires/h1/work"; return pl }, ErrRootInSupport},
		{"the GM's workspace for the GM's GMFolder", func() Places { pl := projectsGMPlaces(); pl.GMFolder = gmWorkspace; return pl }, ErrRootInSupport},
		{"a GMFolder in the support folder that isn't a workspace", func() Places {
			pl := hirePlaces(realSupport)
			pl.GMFolder = realSupport + "/hires/gm1"
			return pl
		}, ErrRootInSupport},
		{"a GMFolder that is a workspace one folder down", func() Places {
			pl := hirePlaces(realSupport)
			pl.GMFolder = realSupport + "/hires/gm1/x/work"
			return pl
		}, ErrRootInSupport},
		{"a GMFolder that is another support folder's workspace", func() Places {
			pl := hirePlaces(copyDir)
			pl.OtherSupports, pl.GMFolder = []string{realSupport}, realSupport+"/hires/gm1/work"
			return pl
		}, ErrRootInSupport},
	} {
		if _, err := New(c.pl()); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

// The folders above the projects folder are held as a desk's place's are, also where they aren't above its Root.
func TestTheFoldersAboveTheGMsProjectsAreHeld(t *testing.T) {
	pl := projectsGMPlaces()
	pl.GMProjects, pl.GMPlans = "/Volumes/Work/projects", "/Volumes/Work/projects/.smolcorp/plans/gm1"
	p := mustNew(t, pl)
	for _, at := range []string{"/Volumes/Work/projects", "/Volumes/Work", "/Volumes"} {
		for _, op := range []Op{Write, Move, Link} {
			if got, by := p.Decide(at, op); got != Deny {
				t.Errorf("%v %s: %v by %q", op, at, got, by.Row)
			}
		}
	}
	if got, _ := p.Decide("/Volumes/Work/other/x", Write); got != Allow {
		t.Error("a folder beside the projects folder isn't normal work")
	}
}

// FXGMROOTR1 N2: a hire under that GM writes none of what the GM reads from the folder it starts in: its plans, and
// the agent files at the top there with their writers' names. The rest of the projects folder is as before.
func TestAHireUnderTheGMWritesNothingTheGMLoadsWhereItStarts(t *testing.T) {
	p := mustNew(t, projectsHirePlaces())
	const plansRow, filesRow = rowGMStart + ": its plans", rowGMStart + ": the agent files there"
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
		row  string
	}{
		{gmPlans + "/calm-river.md", Write, Deny, plansRow}, {gmPlans + "/sub/x.md", Write, Deny, plansRow}, {gmPlans, Write, Deny, plansRow},
		{gmPlans, Move, Deny, ""}, {projects + "/.smolcorp/plans", Move, Deny, ""}, {projects + "/.smolcorp", Move, Deny, ""},
		{gmPlans + "/calm-river.md", Read, Allow, ""},
		{projects + "/CLAUDE.md", Write, Deny, filesRow}, {projects + "/CLAUDE.local.md", Write, Deny, filesRow},
		{projects + "/.mcp.json", Write, Deny, filesRow}, {projects + "/AGENTS.md", Write, Deny, filesRow},
		{projects + "/claude.md", Write, Deny, filesRow}, {projects + "/CLAUDE.md", Read, Allow, ""},
		// Their writers' names (ownersBeside).
		{projects + "/CLAUDE.md.lock", Write, Deny, filesRow}, {projects + "/.mcp.json.tmp.501.1", Write, Deny, filesRow},
		{projects + "/AGENTS.md.x1y2.tmp", Write, Deny, filesRow}, {projects + "/.!501!CLAUDE.local.md", Write, Deny, filesRow},
		{projects + "/.CLAUDE.md.tmp.501.1", Write, Deny, filesRow}, {projects + "/INS@abc123", Write, Deny, filesRow},
		// Its own worktree stays its own, those names in it too.
		{hires + "/h1/CLAUDE.md", Write, Allow, ""}, {hires + "/h1/.mcp.json", Write, Allow, ""}, {hires + "/h1/main.go", Write, Allow, ""},
		{gmWorkspace + "/notes.md", Write, Deny, ""},
	} {
		got, by := p.Decide(c.at, c.op)
		if got != c.want || c.row != "" && by.Row != c.row {
			t.Errorf("%v %s: %v by %q, want %v by %q", c.op, c.at, got, by.Row, c.want, c.row)
		}
	}
	// A desk under a GM with no workspace has none of these rows: its GMFolder holds both.
	if got, by := mustNew(t, hirePlaces(realSupport)).Decide(projects+"/CLAUDE.md", Write); by.Row == filesRow {
		t.Errorf("a hire under a GM in its own folder: %v by %q", got, by.Row)
	}
}

// The GM's start folder is another desk's to be kept from, never the GM's own, and its plans folder is inside it.
func TestAGMsStartFolderIsOnlyAnotherDesksToBeKeptFrom(t *testing.T) {
	for _, c := range []struct {
		name string
		pl   func() Places
		want error
	}{
		{"a hire under the GM", projectsHirePlaces, nil},
		{"no plans folder", func() Places { pl := projectsHirePlaces(); pl.GMStartPlans = ""; return pl }, nil},
		{"the GM itself", func() Places { pl := projectsGMPlaces(); pl.GMStart, pl.GMStartPlans = projects, gmPlans; return pl }, ErrGMStart},
		{"plans with no start folder", func() Places { pl := projectsHirePlaces(); pl.GMStart = ""; return pl }, ErrGMStart},
		{"plans outside it", func() Places { pl := projectsHirePlaces(); pl.GMStartPlans = home + "/plans"; return pl }, ErrGMStart},
		{"plans the start folder", func() Places { pl := projectsHirePlaces(); pl.GMStartPlans = projects; return pl }, ErrGMStart},
		{"a start folder that isn't whole", func() Places { pl := projectsHirePlaces(); pl.GMStart, pl.GMStartPlans = "Projects", ""; return pl }, ErrGMStart},
	} {
		if _, err := New(c.pl()); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}
