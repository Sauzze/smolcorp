package sandbox

import (
	"errors"
	"testing"
)

// FXHIREPROJECTSR3's N2 (RT126FIXREVIEW-HIREPROJECTSR2), as data on policy_test.go's made-up Mac; desktop's
// hireprojectsr3_darwin_test.go runs it under the real profile: no desk changes a link on the way to the folder for
// hires. (Its B1 rows, for a desk whose place was the folder for hires, went with FXHIREPROJECTSR4: no such desk starts
// on a Mac, projects_test.go.)

// N2: the links on the way to the folder for hires, as Setup spells it, are held for every desk that has it.
func TestNoDeskChangesALinkOnTheWayToTheProjectsFolder(t *testing.T) {
	pl := inProjectsPlaces()
	pl.ProjectsLinks = []string{home + "/P", home + "/Volumes-link/Code"}
	decideEach(t, "a hire in its worktree", mustNew(t, pl), []projectsCase{
		{home + "/P", Write, Deny, rowProjectsLinks}, {home + "/P", Move, Deny, ""}, {home + "/P", Link, Deny, ""},
		{home + "/Volumes-link/Code", Write, Deny, rowProjectsLinks}, {home + "/Volumes-link", Move, Deny, ""},
		{home + "/P2", Write, Allow, ""}, {home + "/P", Read, Allow, ""},
	})
}

func TestTheProjectsFoldersLinksComeOnlyWithIt(t *testing.T) {
	for name, c := range map[string]struct {
		pl   func(*Places)
		want error
	}{
		"links with no projects folder": {func(pl *Places) { pl.Projects, pl.ProjectsLinks = "", []string{home + "/P"} }, ErrProjects},
		"a link that isn't whole":       {func(pl *Places) { pl.ProjectsLinks = []string{"P"} }, ErrNotWhole},
	} {
		pl := inProjectsPlaces()
		c.pl(&pl)
		if _, err := New(pl); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}
