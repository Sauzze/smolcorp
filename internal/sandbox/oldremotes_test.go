package sandbox

import (
	"cmp"
	"slices"
	"strings"
	"testing"
)

// No desk writes git's old remote files (RT126GITCFG2's note 3; SIERRA, 14 Nov): `branches/<name>` and
// `remotes/<name>` in the shared .git, and in every worktree's registration, its own included, a desk at the project
// as well, in Decide and in the profile read as Seatbelt reads it. It still reads them, and its other work in its
// registration stays its own.
func TestNoDeskWritesTheOldRemoteFiles(t *testing.T) {
	probes := []struct {
		at   string
		op   Op
		want Verdict
	}{
		{"/branches", Write, D}, {"/branches/up1", Write, D}, {"/remotes", Write, D}, {"/remotes/up2", Write, D},
		{"/remotes/a/b", Write, D}, {"/REMOTES/up2", Write, D}, {"/refs/../remotes/up2", Write, D},
		{"/branches", Move, D}, {"/remotes", Move, D}, {"/remotes/up2", Link, D},
		{"/worktrees/$WT/branches/up1", Write, D}, {"/worktrees/$WT/remotes/up2", Write, D}, {"/worktrees/$WT/remotes", Move, D},
		{"/worktrees/added-later/remotes/up2", Write, D},
		{"/remotes/up2", Read, A}, {"/branches/up1", Read, A}, {"/worktrees/$WT/remotes/up2", Read, A},
	}
	atProject := setup{"a desk at the project", hirePlaces(realSupport)}
	atProject.pl.Root, atProject.pl.Checkout, atProject.pl.OwnWorktree = repo, "", ""
	for _, s := range append(setups(), atProject) {
		p := mustNew(t, s.pl)
		profile := profileOf(p, p.Exception)
		for _, c := range probes {
			at := strings.Replace(gitDir+c.at, "$WT", cmp.Or(s.pl.OwnWorktree, "h1"), 1)
			if got, by := p.Decide(at, c.op); got != c.want {
				t.Errorf("%s: %v %s: %v by %q, want %v", s.name, c.op, at, got, by.Row, c.want)
			}
			if got := profile.says(at, c.op); got != c.want {
				t.Errorf("%s: %v %s: the profile says %v, want %v", s.name, c.op, at, got, c.want)
			}
		}
		// What the desk's own registration holds for its own work stays its own.
		own := gitDir + "/worktrees/" + cmp.Or(s.pl.OwnWorktree, "h1")
		if s.pl.Root != repo {
			for _, at := range []string{own + "/HEAD", own + "/index", own + "/logs/HEAD", own + "/rebase-merge/git-rebase-todo"} {
				if got, by := p.Decide(at, Write); got != A {
					t.Errorf("%s: write %s: %v by %q, want it still the desk's", s.name, at, got, by.Row)
				}
			}
		}
	}
}

// The old remote files' rows only add final denies (and their rename guards) to each profile, in the section nothing
// opens: no allow moves, no other line changes or goes (SIERRA's seatbelt order rule). Both profiles have the golden
// column's Services, so SA-M2's sections, before the file rules and after them, are in both and must not change either.
func TestTheOldRemoteFilesRowsOnlyAddFinalDenies(t *testing.T) {
	mine := func(r Rule) bool { return r.Row == rowOldRemotes }
	for _, s := range profileSetups() {
		p, svc := mustNew(t, s.pl), goldenServices(s.pl)
		profile, err := p.Profile(svc)
		if err != nil {
			t.Fatal(err)
		}
		without := profileWithout(t, p, svc, mine)
		// SA-M2's layout: the opening lines, then 7 sections, the 4 of the file rules (profileSections) among them.
		whole, wholeBefore := strings.Split(profile, "\n;; "), strings.Split(without, "\n;; ")
		if len(whole) != 8 || len(wholeBefore) != 8 {
			t.Fatalf("%s: %d and %d parts, want the opening lines and 7 sections", s.name, len(whole), len(wholeBefore))
		}
		for i, part := range whole {
			if heading, _, _ := strings.Cut(part, "\n"); !slices.Contains(fileHeadings, heading) && part != wholeBefore[i] {
				t.Errorf("%s: the old remote files' rows change the part headed %q", s.name, heading)
			}
		}
		after, before := profileSections(profile), profileSections(without)
		if len(after) != 4 || len(before) != 4 {
			t.Fatalf("%s: %d and %d sections, want 4", s.name, len(after), len(before))
		}
		for i := range 3 {
			if !slices.Equal(after[i], before[i]) {
				t.Errorf("%s: the old remote files' rows change section %d", s.name, i)
			}
		}
		added := slices.Clone(after[3])
		for _, line := range before[3] {
			at := slices.Index(added, line)
			if at < 0 {
				t.Errorf("%s: the old remote files' rows take out %s", s.name, line)
				continue
			}
			added = slices.Delete(added, at, at+1)
		}
		for _, line := range added {
			if !strings.HasPrefix(line, "(deny ") || !strings.HasSuffix(line, " ; "+rowOldRemotes) && !strings.HasSuffix(line, " ; "+rowAbove) {
				t.Errorf("%s: the old remote files' rows add %s", s.name, line)
			}
		}
		for _, folder := range []string{"branches", "remotes"} {
			if want := `(deny file-write* file-link (subpath "` + gitDir + "/" + folder + `")) ; ` + rowOldRemotes; !slices.Contains(added, want) {
				t.Errorf("%s: no %s", s.name, want)
			}
		}
	}
}
