package sandbox

import "testing"

// FXSTACKG G22, RT126FIXREVIEW-GITROWS2R9's N1: anyDesks takes the whole projects folder as some desk's (a hire's
// Projects, or a GM's GMProjects), outside this repo's own .git, which it judges as before (gitDesksPlaces).
func TestTheWholeProjectsFolderIsSomeDesksOutsideThisReposOwnGit(t *testing.T) {
	const projects = "/Users/ada/Projects"
	for _, c := range []struct {
		row string
		pl  Places
		git string
	}{
		{"a hire's", Places{Root: projects + "/app-hires/h1", Checkout: projects + "/app", Projects: projects}, projects + "/app/.git"},
		{"a GM's", Places{Root: "/Users/ada/AD/gm-workspace", GMProjects: projects}, ""},
	} {
		p := &Policy{goos: "darwin", git: c.git, desks: desksPlaces("darwin", c.pl)}
		for at, want := range map[string]bool{
			projects + "/other/x":        true,
			projects + "/lib/.git/hooks": true,
			projects + "/Clients/acme":   true,
			projects:                     true,
			"/Users/ada/elsewhere":       false,
			"/Users/ada/projects-old/x":  false,
		} {
			if got := p.anyDesks(at); got != want {
				t.Errorf("%s: anyDesks(%s) = %v, want %v", c.row, at, got, want)
			}
		}
	}
	// This repo's own .git, in the projects folder: only gitDesksPlaces there.
	p := &Policy{goos: "darwin", git: projects + "/app/.git", desks: desksPlaces("darwin", Places{Projects: projects})}
	for at, want := range map[string]bool{
		projects + "/app/.git/rr-real":                 false,
		projects + "/app/.git/worktrees/h2":            true,
		projects + "/app/.git/refs/heads/hire/h2/work": true,
	} {
		if got := p.anyDesks(at); got != want {
			t.Errorf("this repo's .git: anyDesks(%s) = %v, want %v", at, got, want)
		}
	}
}
