package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXSTACKG G22, RT126FIXREVIEW-GITROWS2R9's N1 (MUST at the stack): with FXHIREPROJECTS' projects folder held from a
// desk, another project there is held from it too, but it is that project's desks' place, and their starts never look
// at this .git. So a link at one of git's own names that leads into the projects folder, outside this repo's own .git,
// refuses this repo's next start, as a link into any other desk's place does (anyDesks).
//   - XPROJ: $G/rr-cache → ~/Projects/other/x, held from h1 (it can't write there), refuses app's next start, naming
//     $G/rr-cache; once the owner removes it, the desk starts.
//   - XPROJ, app in the projects folder (RT126STACKP3 O2): the same with app itself at ~/Projects/app, the shape
//     GITROWS2R9's N1 named. Its own .git is then in the projects folder too, but stays judged by gitDesksPlaces
//     alone: the owner's own link there into that .git ($G/rr-cache → $G/rr-real) starts, and the link into another
//     project still refuses.
func TestOnAMacALinkAtGitsOwnNameIntoAnotherProjectStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	t.Run("XPROJ", func(t *testing.T) {
		d, _ := newRowsDesk(t, git)
		d.pl.Projects = filepath.Join(d.home, "Projects")
		to := filepath.Join(d.pl.Projects, "other", "x")
		mustWrite(t, filepath.Join(to, "notes"), "")
		if err := d.prepared(t); err != nil {
			t.Fatalf("the first start, with the projects folder: %v", err)
		}
		d.probe(t, python, [][]string{{"write", filepath.Join(to, "standin")}}, []string{"PermissionError"})
		top := filepath.Join(d.git, "rr-cache")
		if err := os.RemoveAll(top); err != nil {
			t.Fatal(err)
		}
		mustLink(t, os.Symlink, to, top)
		if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), top) {
			t.Errorf("the next start: %v, want it refused, naming %s", err, top)
		}
		if err := os.Remove(top); err != nil {
			t.Fatal(err)
		}
		if err := d.prepared(t); err != nil {
			t.Errorf("the start once $G/rr-cache was removed: %v", err)
		}
	})
	t.Run("XPROJ, app in the projects folder", func(t *testing.T) {
		d, owner := newRowsDesk(t, git)
		d.pl.Projects = filepath.Join(d.home, "Projects")
		app := filepath.Join(d.pl.Projects, "app")
		if err := os.MkdirAll(d.pl.Projects, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(d.checkout, app); err != nil {
			t.Fatal(err)
		}
		owner(app, "worktree", "repair")
		d.checkout, d.git = app, filepath.Join(app, ".git")
		d.pl.Checkout, d.pl.GitDir = d.checkout, d.git
		if common := strings.TrimSpace(owner(d.root, "rev-parse", "--path-format=absolute", "--git-common-dir")); common != d.git {
			t.Fatalf("h1's worktree after the move: its git folder %q, want %q", common, d.git)
		}
		to := filepath.Join(d.pl.Projects, "other", "x")
		mustWrite(t, filepath.Join(to, "notes"), "")
		if err := d.prepared(t); err != nil {
			t.Fatalf("the first start, app in the projects folder: %v", err)
		}
		d.probe(t, python, [][]string{{"write", filepath.Join(to, "standin")}}, []string{"PermissionError"})
		top, own := filepath.Join(d.git, "rr-cache"), filepath.Join(d.git, "rr-real")
		if err := os.RemoveAll(top); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(own, 0o700); err != nil {
			t.Fatal(err)
		}
		mustLink(t, os.Symlink, own, top)
		if err := d.prepared(t); err != nil {
			t.Errorf("the start with $G/rr-cache → $G/rr-real, app's own .git: %v, want it started", err)
		}
		if err := os.Remove(top); err != nil {
			t.Fatal(err)
		}
		mustLink(t, os.Symlink, to, top)
		if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), top) {
			t.Errorf("the next start: %v, want it refused, naming %s", err, top)
		}
		if err := os.Remove(top); err != nil {
			t.Fatal(err)
		}
		if err := d.prepared(t); err != nil {
			t.Errorf("the start once $G/rr-cache was removed: %v", err)
		}
	})
}
