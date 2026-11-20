package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R8 (RT126FIXREVIEW-GITROWS2R7's B1 and its "seen outside" hooks link) under the real profile, with
// newRowsDesk's stand-ins: the hire h1 on hire/h1/work in its worktree $W1, the owner's checkout. Each plant is made
// outside, as a desk under the signed base's profile could make it, or the owner. These rows name no API of this
// round's, so they run on the tree before it too.

// A link at one of git's own names at the top of the shared .git that leads where the desk writes refuses its next
// start, naming it; once the owner removes it, the desk starts.
//   - UPRR2: rr-cache → a folder in $W1: the owner's merge would apply a resolution the desk recorded there.
//   - UPAUTO: MERGE_AUTOSTASH → a file in $W1: the owner's next commit would apply the desk's stash to the owner's files.
//   - UPFETCH: FETCH_HEAD → a file in $W1: the owner's git log FETCH_HEAD takes the desk's commit.
//   - HOOKS: hooks → ../.githooks, in the checkout, for a desk at the project (or a Mac plain hire at the main
//     checkout): the owner's next commit would run a hook the desk wrote there.
//   - HOOKSHOME: hooks → ~/dotfiles/hooks, a folder no row holds: a desk can write there (checked first, as the desk).
func TestOnAMacALinkAtGitsOwnNameThatLeadsWhereADeskWritesStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	for _, c := range []struct {
		name, top string
		atProject bool
		plant     func(t *testing.T, d *escapeDesk, owner func(string, ...string) string, mine string)
	}{
		{"UPRR2", "rr-cache", false, func(t *testing.T, d *escapeDesk, _ func(string, ...string) string, mine string) {
			mustWrite(t, filepath.Join(d.root, "rr", mine, "preimage"), "c\n")
			mustLink(t, os.Symlink, filepath.Join(d.root, "rr"), filepath.Join(d.git, "rr-cache"))
		}},
		{"UPAUTO", "MERGE_AUTOSTASH", false, func(t *testing.T, d *escapeDesk, _ func(string, ...string) string, mine string) {
			mustWrite(t, filepath.Join(d.root, "as"), mine+"\n")
			mustLink(t, os.Symlink, filepath.Join(d.root, "as"), filepath.Join(d.git, "MERGE_AUTOSTASH"))
		}},
		{"UPFETCH", "FETCH_HEAD", false, func(t *testing.T, d *escapeDesk, _ func(string, ...string) string, mine string) {
			mustWrite(t, filepath.Join(d.root, "fh"), mine+"\t\tbranch 'main' of ../up\n")
			mustLink(t, os.Symlink, filepath.Join(d.root, "fh"), filepath.Join(d.git, "FETCH_HEAD"))
			if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "FETCH_HEAD"); !strings.Contains(got, mine) {
				t.Fatalf("the stand-in: the owner's rev-parse FETCH_HEAD is %q, not the desk's %s", got, mine)
			}
		}},
		{"HOOKS", "hooks", true, func(t *testing.T, d *escapeDesk, _ func(string, ...string) string, _ string) {
			mustWrite(t, filepath.Join(d.checkout, ".githooks", "pre-commit.sample"), "")
			if err := os.RemoveAll(filepath.Join(d.git, "hooks")); err != nil {
				t.Fatal(err)
			}
			mustLink(t, os.Symlink, "../.githooks", filepath.Join(d.git, "hooks"))
		}},
		{"HOOKSHOME", "hooks", false, func(t *testing.T, d *escapeDesk, _ func(string, ...string) string, _ string) {
			folder := filepath.Join(d.home, "dotfiles", "hooks")
			mustWrite(t, filepath.Join(folder, "pre-commit.sample"), "")
			// Under the first start's profile: no row holds the folder, so the desk writes a hook there.
			d.probe(t, python, [][]string{{"write", filepath.Join(folder, "post-commit")}}, []string{"ok"})
			if err := os.RemoveAll(filepath.Join(d.git, "hooks")); err != nil {
				t.Fatal(err)
			}
			mustLink(t, os.Symlink, folder, filepath.Join(d.git, "hooks"))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
				t.Fatalf("the desk's commit on its own branch: %+v", r)
			}
			mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
			if c.atProject {
				atTheProject(d, owner)
			}
			if err := d.prepared(t); err != nil {
				t.Fatalf("the first start: %v", err)
			}
			c.plant(t, d, owner, mine)
			top := filepath.Join(d.git, c.top)
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), top) {
				t.Errorf("the next start, %s: %v, want it refused, naming %s", c.name, err, top)
			}
			if err := os.Remove(top); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once $G/%s was removed: %v", c.top, err)
			}
		})
	}
}

// The controls: a link at hooks to a folder the owner's git config names for hooks (which no desk writes), a link at
// config to a file in ~/.config/git (which no desk writes), and a real rr-cache folder with an entry each start a
// desk, and the desk can't write where the links lead. (The hooks link into the checkout for a desk in its own
// worktree was a control here until FXGITROWS2R9: a desk at the project writes there, so it now refuses the start,
// TestOnAMacALinkAtGitsOwnNameIntoAPlaceAnyDeskIsGivenStopsTheNextStart's CHECKOUT.)
func TestOnAMacALinkAtGitsOwnNameToWhereNoDeskWritesStillStartsADesk(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	hooks := filepath.Join(d.home, "dotfiles", "hooks")
	mustWrite(t, filepath.Join(hooks, "pre-commit.sample"), "")
	if err := os.RemoveAll(filepath.Join(d.git, "hooks")); err != nil {
		t.Fatal(err)
	}
	mustLink(t, os.Symlink, hooks, filepath.Join(d.git, "hooks"))
	d.pl.GitHooks = []string{hooks} // as ReadGitConfig gives core.hooksPath's folder
	config := filepath.Join(d.home, ".config", "git", "app-config")
	data, err := os.ReadFile(filepath.Join(d.git, "config"))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, config, string(data))
	if err := os.Remove(filepath.Join(d.git, "config")); err != nil {
		t.Fatal(err)
	}
	mustLink(t, os.Symlink, config, filepath.Join(d.git, "config"))
	mustWrite(t, filepath.Join(d.git, "rr-cache", strings.Repeat("1", 40), "preimage"), "c\n")
	if got := owner(d.checkout, "config", "commit.gpgsign"); strings.TrimSpace(got) != "false" {
		t.Fatalf("the stand-in: the owner's git reads its config through the link as %q", got)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the start with hooks and config linked where no desk writes, and a real rr-cache: %v", err)
	}
	d.prepare(t)
	d.probe(t, python, [][]string{{"write", filepath.Join(hooks, "post-commit")}, {"write", config}},
		[]string{"PermissionError", "PermissionError"})
}
