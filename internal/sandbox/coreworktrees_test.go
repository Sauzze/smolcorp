package sandbox

import (
	"errors"
	"testing"
)

// FXCOREWORKTREES126: the core's own folder for hires' worktrees (Places.Worktrees). Every hire writes only its own
// worktree there; a GM writes none of it and reads all of it; no desk moves, removes or makes the folders above it; and
// no GM's own place is in it. As data, on the made-up Mac of policy_test.go.

const coreWorktrees = home + "/.smolcorp/worktrees"

// coreHirePlaces is a hire made in the core's own folder for worktrees, under the GM in the projects folder.
func coreHirePlaces() Places {
	pl := projectsHirePlaces()
	pl.Root = coreWorktrees + "/app-0123abcd/h1"
	pl.GitHooks = nil // policy_test.go's hooks are in the hire's old place
	return pl
}

func TestAGMReadsTheCoresFolderForWorktreesAndWritesNoneOfIt(t *testing.T) {
	p := mustNew(t, projectsGMPlaces())
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
		row  string
	}{
		{coreWorktrees + "/app-0123abcd/h1/main.go", Read, Allow, ""},
		{coreWorktrees + "/app-0123abcd/h1/main.go", Write, Deny, rowOtherDesks},
		{coreWorktrees + "/app-0123abcd/h1", Write, Deny, rowOtherDesks},  // a hire's tree swapped for a link: no
		{coreWorktrees + "/app-0123abcd", Write, Deny, rowOtherDesks},     // the repo's folder turned: no
		{coreWorktrees + "/app-0123abcd/new", Write, Deny, rowOtherDesks}, // a tree made ahead of the core's: no
		{coreWorktrees, Write, Deny, rowOtherDesks},                       // the folder itself, moved aside: no
		{home + "/.smolcorp", Write, Deny, rowWorktreesAbove},             // the folder above it, moved and made again: no
		{home + "/.smolcorp/other", Write, Allow, ""},                     // anything else there is as before
		{gmWorkspace + "/notes.md", Write, Allow, ""},                     // its own workspace, as before
	} {
		got, by := p.Decide(c.at, c.op)
		if got != c.want || c.row != "" && by.Row != c.row {
			t.Errorf("%v %s: %v by %q, want %v by %q", c.op, c.at, got, by.Row, c.want, c.row)
		}
	}
}

func TestAHireWritesOnlyItsOwnWorktreeInTheCoresFolder(t *testing.T) {
	p := mustNew(t, coreHirePlaces())
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
	}{
		{coreWorktrees + "/app-0123abcd/h1/main.go", Write, Allow},
		{coreWorktrees + "/app-0123abcd/h1/main.go", Read, Allow},
		{coreWorktrees + "/app-0123abcd/h2/main.go", Write, Deny}, // another hire's tree
		{coreWorktrees + "/app-0123abcd/h2/main.go", Read, Allow}, // read, as another desk's place is
		{coreWorktrees + "/other-4567cdef/h3", Write, Deny},       // another repo's hire
		{coreWorktrees + "/app-0123abcd/h1", Write, Deny},         // its own tree, moved or removed: no
		{coreWorktrees + "/app-0123abcd", Write, Deny},            // the repo's folder
		{home + "/.smolcorp", Write, Deny},                        // the folder above
	} {
		if got, by := p.Decide(c.at, c.op); got != c.want {
			t.Errorf("%v %s: %v by %q, want %v", c.op, c.at, got, by.Row, c.want)
		}
	}
}

func TestTheCoresFolderForWorktreesIsRefusedWhereItWouldHoldTooMuch(t *testing.T) {
	for _, c := range []struct {
		name string
		pl   func() Places
		want error
	}{
		{"a GM's workspace in it", func() Places {
			pl := projectsGMPlaces()
			pl.Worktrees = []string{realSupport + "/hires"}
			return pl
		}, ErrWorktrees},
		{"a GM's place in it", func() Places {
			pl := setups()[1].pl
			pl.Worktrees = []string{gmDir}
			return pl
		}, ErrWorktrees},
		{"a GM's projects in it", func() Places {
			pl := projectsGMPlaces()
			pl.Worktrees = []string{home + "/Projects"}
			return pl
		}, ErrWorktrees},
		{"the home folder", func() Places { pl := coreHirePlaces(); pl.Worktrees = []string{home}; return pl }, ErrWorktrees},
		{"the projects folder", func() Places { pl := inProjectsPlaces(); pl.Worktrees = []string{projects}; return pl }, ErrWorktrees},
		{"in the support folder", func() Places { pl := coreHirePlaces(); pl.Worktrees = []string{realSupport + "/w"}; return pl }, ErrWorktrees},
		{"not whole", func() Places { pl := coreHirePlaces(); pl.Worktrees = []string{"w"}; return pl }, ErrWorktrees},
		{"a hire's place holding it", func() Places {
			pl := coreHirePlaces()
			pl.Root = home + "/.smolcorp"
			return pl
		}, ErrRootHasDesks},
	} {
		if _, err := New(c.pl()); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}
