package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's B1, O2 and N2) under the real profile, with newRowsDesk's stand-ins: the
// hire h1 on hire/h1/work, another desk's worktree h2 on hire/h2/work, the owner's checkout on feature. These rows name
// no API of this round's, so they run on the tree before it too.

// B1 and O2: a name at the top of the shared .git made outside the sandbox before this build (by a desk under an older
// profile, say) that the owner's git takes for a branch, and that a desk could change or that already holds a desk's
// commit, refuses the desk's next start, naming it; once the owner removes it, the desk starts.
//   - LINKABS, LINKREL, DIRLINK: a link, to a file in the desk's worktree, to git-lfs's store (open to a desk), or a
//     folder linked into the desk's worktree; git reads a link whose text isn't refs/… as a file, through it.
//   - HARDTOP: a file with a second name in the desk's worktree.
//   - REFLFS, REFOBJ: a symbolic ref to a name outside refs/ the desk can write.
//   - STATIC: a commit's id, which the owner's git takes for feature (O2).
func TestOnAMacANamePlantedAtTheTopOfTheSharedGitStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	for _, c := range []struct {
		name, ask, top string // the case, the name the owner's git is asked for, and what the owner removes ($G/<top>)
		plant          func(t *testing.T, d *escapeDesk, mine string)
	}{
		{"LINKABS", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.root, "f"), mine+"\n")
			mustLink(t, os.Symlink, filepath.Join(d.root, "f"), filepath.Join(d.git, "feature"))
		}},
		{"LINKREL", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.git, "lfs", "x"), mine+"\n")
			mustLink(t, os.Symlink, "lfs/x", filepath.Join(d.git, "feature"))
		}},
		{"DIRLINK", "hire/h2/work", "hire", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.root, "d", "h2", "work"), mine+"\n")
			mustLink(t, os.Symlink, filepath.Join(d.root, "d"), filepath.Join(d.git, "hire"))
		}},
		{"HARDTOP", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.root, "f"), mine+"\n")
			mustLink(t, os.Link, filepath.Join(d.root, "f"), filepath.Join(d.git, "feature"))
		}},
		{"REFLFS", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.git, "lfs", "x"), mine+"\n")
			mustWrite(t, filepath.Join(d.git, "feature"), "ref: lfs/x\n")
		}},
		{"REFOBJ", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.git, "objects", "x"), mine+"\n")
			mustWrite(t, filepath.Join(d.git, "feature"), "ref: objects/x\n")
		}},
		{"STATIC", "feature", "feature", func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.git, "feature"), mine+"\n")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
				t.Fatalf("the desk's commit on its own branch: %+v", r)
			}
			mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
			c.plant(t, d, mine)
			if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", c.ask); !strings.Contains(got, mine) {
				t.Fatalf("the stand-in: the owner's rev-parse %s is %q, not the desk's %s", c.ask, got, mine)
			}
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

// B1, PROJFETCH and PROJORIG: for a desk at the project, which writes the checkout's FETCH_HEAD and ORIG_HEAD as its
// own (FXBASEHEADS' ruling #3), `$G/feature` made outside as a symbolic ref to either refuses its next start; removed,
// it starts.
func TestOnAMacASymbolicRefToTheProjectDesksOwnStateStopsItsNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	for _, name := range []string{"FETCH_HEAD", "ORIG_HEAD"} {
		t.Run(name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			atTheProject(d, owner)
			if err := d.prepared(t); err != nil {
				t.Fatalf("the desk at the project's first start: %v", err)
			}
			mine := strings.TrimSpace(owner(d.checkout, "commit-tree", "HEAD^{tree}", "-m", "the desk's"))
			text := mine + "\n"
			if name == "FETCH_HEAD" {
				text = mine + "\t\tbranch 'main' of ../up\n"
			}
			mustWrite(t, filepath.Join(d.git, name), text)
			feature := filepath.Join(d.git, "feature")
			mustWrite(t, feature, "ref: "+name+"\n")
			if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "feature"); !strings.Contains(got, mine) {
				t.Fatalf("the stand-in: the owner's rev-parse feature is %q, not the desk's %s", got, mine)
			}
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), feature) {
				t.Errorf("the next start, $G/feature to %s: %v, want it refused, naming %s", name, err, feature)
			}
			if err := os.Remove(feature); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once $G/feature was removed: %v", err)
			}
		})
	}
}

// The controls: what a repo's own git leaves at the top of its .git still starts a desk. The owner's merge in progress
// (MERGE_HEAD and ORIG_HEAD, commits' ids; a one-line MERGE_MSG, then one that starts "ref: #12 …"), a fetch's
// FETCH_HEAD, packed-refs, and what `git init` makes (config, description, info/, hooks/).
func TestOnAMacTheOwnersMergeAndARepositorysOwnTopLevelStillStartADesk(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	up := filepath.Join(d.home, "up.git")
	owner(d.home, "clone", "-q", "--bare", d.checkout, up)
	owner(d.checkout, "remote", "add", "origin", up)
	owner(d.checkout, "fetch", "-q", "origin")
	owner(d.checkout, "pack-refs", "--all")
	owner(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's")
	owner(filepath.Join(d.home, "app-hires", "h2"), "commit", "-q", "--allow-empty", "-m", "another desk's")
	owner(d.checkout, "merge", "-q", "--no-ff", "--no-commit", "hire/h2/work")
	for _, name := range []string{"MERGE_HEAD", "ORIG_HEAD", "MERGE_MSG", "FETCH_HEAD", "packed-refs", "config", "description", "info", "hooks"} {
		if _, err := os.Lstat(filepath.Join(d.git, name)); err != nil {
			t.Fatalf("the stand-in: $G/%s: %v", name, err)
		}
	}
	msg, err := os.ReadFile(filepath.Join(d.git, "MERGE_MSG"))
	if err != nil || strings.Count(string(msg), "\n") != 1 {
		t.Fatalf("the stand-in: MERGE_MSG %q (%v), want one line", msg, err)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the start during the owner's merge: %v", err)
	}
	mustWrite(t, filepath.Join(d.git, "MERGE_MSG"), "ref: #12 the owner's fix\n")
	if err := d.prepared(t); err != nil {
		t.Errorf("the start with a MERGE_MSG that starts \"ref: #12\": %v", err)
	}
	owner(d.checkout, "merge", "--abort")
	if err := d.prepared(t); err != nil {
		t.Errorf("the start once the owner's merge was given up: %v", err)
	}
}

// N2: the checkout's merge state is a desk's own only at the project's main checkout. A Mac plain hire (no Checkout,
// as newLauncherSandbox gives it) in a linked worktree can't write the main checkout's MERGE_HEAD, ORIG_HEAD or
// FETCH_HEAD, which would make the owner's next commit a merge of its own; a desk at the project still writes them.
func TestOnAMacOnlyADeskAtTheProjectWritesTheCheckoutsMergeState(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	g := d.git
	d.pl.Checkout, d.pl.CheckoutBranch = "", ""
	d.prepare(t)
	steps := [][]string{{"write", g + "/MERGE_HEAD"}, {"write", g + "/ORIG_HEAD"}, {"write", g + "/FETCH_HEAD"}}
	d.probe(t, python, steps, times(len(steps), "PermissionError"))
	for _, name := range []string{"MERGE_HEAD", "ORIG_HEAD"} {
		if _, err := os.Lstat(filepath.Join(g, name)); err == nil {
			t.Errorf("$G/%s was made by a plain hire in a linked worktree", name)
		}
	}
	atTheProject(d, owner)
	d.prepare(t)
	steps = append(steps, []string{"unlink", g + "/MERGE_HEAD"})
	d.probe(t, python, steps, times(len(steps), "ok"))
}

// atTheProject makes newRowsDesk's desk one at the project: its worktree goes (left on hire/h1/work, which a desk at
// the project writes, its HEAD would refuse the start, FXGITROWS2R5), and it works in the owner's checkout.
func atTheProject(d *escapeDesk, owner func(string, ...string) string) {
	owner(d.checkout, "worktree", "remove", "--force", d.root)
	d.root = d.checkout
	d.pl.Root, d.pl.Checkout, d.pl.OwnWorktree, d.pl.CheckoutBranch = d.checkout, "", "", ""
}

// mustLink makes a link (os.Symlink or os.Link) at at to to.
func mustLink(t *testing.T, link func(string, string) error, to, at string) {
	t.Helper()
	if err := link(to, at); err != nil {
		t.Fatal(err)
	}
}
