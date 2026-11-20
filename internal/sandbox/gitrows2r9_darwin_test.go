package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's B1, B2, N1 and O2) under the real profile, with newRowsDesk's stand-ins:
// the hire h1 on hire/h1/work in its worktree $W1, the owner's checkout. Each plant is made outside, as a desk under
// the signed base's profile could make it, or the owner. The rows of B1, B2 and N1 name no API of this round's, so they
// run on the tree before it too.

// B1: a link at one of git's own names whose way goes through a link the desk can change stops the next start, naming
// the name, wherever the way leads at the start: the desk would point it elsewhere once started. Once the owner
// removes it, the desk starts.
//   - UPMID (the review's own): rr-cache → $W1/lnk → the GM's folder.
//   - UPMIDHELD: rr-cache → $W1/lnk → a folder git's config names for hooks, held from every desk and no desk's.
//   - UPMIDDIR: rr-cache → $W1/dl/rr, where $W1/dl → that held folder: a link in a folder on the way.
//   - UPMIDAUTO: MERGE_AUTOSTASH → $W1/as → a file in that held folder.
//
// Each first shows, as the desk under the first start's profile, that it can point its link elsewhere.
func TestOnAMacALinkAtGitsOwnNameByWayOfALinkADeskCanChangeStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	for _, c := range []struct {
		name, top string
		plant     func(t *testing.T, d *escapeDesk, held string) (link string)
	}{
		{"UPMID", "rr-cache", func(t *testing.T, d *escapeDesk, _ string) string {
			mustWrite(t, filepath.Join(d.pl.GMFolder, "notes"), "the GM's")
			mustLink(t, os.Symlink, d.pl.GMFolder, filepath.Join(d.root, "lnk"))
			mustLink(t, os.Symlink, filepath.Join(d.root, "lnk"), filepath.Join(d.git, "rr-cache"))
			return filepath.Join(d.root, "lnk")
		}},
		{"UPMIDHELD", "rr-cache", func(t *testing.T, d *escapeDesk, held string) string {
			mustLink(t, os.Symlink, held, filepath.Join(d.root, "lnk"))
			mustLink(t, os.Symlink, filepath.Join(d.root, "lnk"), filepath.Join(d.git, "rr-cache"))
			return filepath.Join(d.root, "lnk")
		}},
		{"UPMIDDIR", "rr-cache", func(t *testing.T, d *escapeDesk, held string) string {
			mustWrite(t, filepath.Join(held, "rr", "notes"), "")
			mustLink(t, os.Symlink, held, filepath.Join(d.root, "dl"))
			mustLink(t, os.Symlink, filepath.Join(d.root, "dl", "rr"), filepath.Join(d.git, "rr-cache"))
			return filepath.Join(d.root, "dl")
		}},
		{"UPMIDAUTO", "MERGE_AUTOSTASH", func(t *testing.T, d *escapeDesk, held string) string {
			mustWrite(t, filepath.Join(held, "as"), strings.Repeat("1", 40)+"\n")
			mustLink(t, os.Symlink, filepath.Join(held, "as"), filepath.Join(d.root, "as"))
			mustLink(t, os.Symlink, filepath.Join(d.root, "as"), filepath.Join(d.git, "MERGE_AUTOSTASH"))
			return filepath.Join(d.root, "as")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, _ := newRowsDesk(t, git)
			held := filepath.Join(d.home, "held-hooks")
			mustWrite(t, filepath.Join(held, "pre-commit.sample"), "")
			d.pl.GitHooks = []string{held} // as ReadGitConfig gives core.hooksPath's folder: no desk writes it
			if err := d.prepared(t); err != nil {
				t.Fatalf("the first start: %v", err)
			}
			link := c.plant(t, d, held)
			top := filepath.Join(d.git, c.top)
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), top) {
				t.Errorf("the next start, %s: %v, want it refused, naming %s", c.name, err, top)
			}
			// Why: as the desk under the first start's profile, the link on the way is its own to point elsewhere.
			d.probe(t, python, [][]string{{"unlink", link}, {"symlink", link, filepath.Join(d.root, "mine")}}, []string{"ok", "ok"})
			if err := os.Remove(top); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once $G/%s was removed: %v", c.top, err)
			}
		})
	}
}

// B2: a hard link at one of git's own files stops the next start, naming it: through its other name, in its own
// worktree, the desk would rewrite it in place. Once the owner removes it, or gives it a copy of its own, the desk
// starts.
//   - UPHFETCH: $G/FETCH_HEAD, a second name of $W1/fh: the owner's rev-parse FETCH_HEAD takes the desk's commit.
//   - UPHAUTO: $G/MERGE_AUTOSTASH, a second name of $W1/as, which holds the desk's commit.
//   - HCONFIG: $G/config, a second name of $W1/cfg: the owner gives it a copy of its own.
func TestOnAMacAHardLinkAtGitsOwnFileStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	for _, c := range []struct {
		name, top string
		copied    bool // the fix is a copy of its own, not removing it
		plant     func(t *testing.T, d *escapeDesk, mine string)
	}{
		{"UPHFETCH", "FETCH_HEAD", false, func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.root, "fh"), mine+"\t\tbranch 'main' of ../up\n")
			if err := os.Remove(filepath.Join(d.git, "FETCH_HEAD")); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			mustLink(t, os.Link, filepath.Join(d.root, "fh"), filepath.Join(d.git, "FETCH_HEAD"))
			if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", "FETCH_HEAD"); !strings.Contains(got, mine) {
				t.Fatalf("the stand-in: the owner's rev-parse FETCH_HEAD is %q, not the desk's %s", got, mine)
			}
		}},
		{"UPHAUTO", "MERGE_AUTOSTASH", false, func(t *testing.T, d *escapeDesk, mine string) {
			mustWrite(t, filepath.Join(d.root, "as"), mine+"\n")
			mustLink(t, os.Link, filepath.Join(d.root, "as"), filepath.Join(d.git, "MERGE_AUTOSTASH"))
		}},
		{"HCONFIG", "config", true, func(t *testing.T, d *escapeDesk, _ string) {
			if err := os.Link(filepath.Join(d.git, "config"), filepath.Join(d.root, "cfg")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
				t.Fatalf("the desk's commit on its own branch: %+v", r)
			}
			mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
			if err := d.prepared(t); err != nil {
				t.Fatalf("the first start: %v", err)
			}
			c.plant(t, d, mine)
			top := filepath.Join(d.git, c.top)
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), top) {
				t.Errorf("the next start, %s: %v, want it refused, naming %s", c.name, err, top)
			}
			if c.copied {
				data, err := os.ReadFile(top)
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, top+".own", string(data))
				if err := os.Rename(top+".own", top); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(top); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once the owner fixed $G/%s: %v", c.top, err)
			}
		})
	}
}

// N1: a link at one of git's own names that leads under a place Smolcorp gives some desk stops the next start, naming
// it, though this desk can't write there (checked first, as the desk): that desk writes it, and its start never looks
// at this repository's .git. Once the owner removes the link, the desk starts.
//   - XPROJ (the review's own): rr-cache → $SUP/hires/o1/claude, the Claude folder of a desk at another project.
//   - XAUTO: MERGE_AUTOSTASH → $SUP/hires/o1/as.
//   - HIRE2: rr-cache → another hire's worktree.
//   - SOCKETS: rr-cache → a folder in the socket folder.
//   - GMFOLDER: rr-cache → a folder in the GM's folder.
//   - TMUX: rr-cache → a folder in tmux's folder.
//   - CHECKOUT: hooks → ../.githooks, for a desk in its own worktree: a desk at the project (a Mac plain hire at the
//     main checkout) writes there.
func TestOnAMacALinkAtGitsOwnNameIntoAPlaceAnyDeskIsGivenStopsTheNextStart(t *testing.T) {
	git, python := rowsTools(t)
	for _, c := range []struct {
		name, top string
		where     func(d *escapeDesk) string // where the link leads, a folder made outside (or a file: "as")
	}{
		{"XPROJ", "rr-cache", func(d *escapeDesk) string { return filepath.Join(d.support, "hires", "o1", "claude") }},
		{"XAUTO", "MERGE_AUTOSTASH", func(d *escapeDesk) string { return filepath.Join(d.support, "hires", "o1", "as") }},
		{"HIRE2", "rr-cache", func(d *escapeDesk) string { return filepath.Join(d.home, "app-hires", "h2", "rr") }},
		{"SOCKETS", "rr-cache", func(d *escapeDesk) string { return filepath.Join(d.sockets, "o1", "rr") }},
		{"GMFOLDER", "rr-cache", func(d *escapeDesk) string { return filepath.Join(d.pl.GMFolder, "rr") }},
		{"TMUX", "rr-cache", func(d *escapeDesk) string { return filepath.Join(d.pl.TmuxFolder, "rr") }},
		{"CHECKOUT", "hooks", func(d *escapeDesk) string { return filepath.Join(d.checkout, ".githooks") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, _ := newRowsDesk(t, git)
			d.pl.TmuxFolder = filepath.Join(d.base, "tmux-501")
			if err := os.MkdirAll(d.pl.TmuxFolder, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Fatalf("the first start: %v", err)
			}
			to := c.where(d)
			inside := filepath.Join(to, "standin")
			if filepath.Base(to) == "as" {
				mustWrite(t, to, strings.Repeat("1", 40)+"\n")
				inside = to
			} else {
				mustWrite(t, filepath.Join(to, "notes"), "")
			}
			d.probe(t, python, [][]string{{"write", inside}}, []string{"PermissionError"})
			top := filepath.Join(d.git, c.top)
			if err := os.RemoveAll(top); err != nil {
				t.Fatal(err)
			}
			mustLink(t, os.Symlink, to, top)
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

// O2: git-lfs's store, the folder $G/lfs itself, is no desk's to remove, rename away, change the mode of or make a link
// at; the core makes it when missing, before the start; what is in it stays the desk's. With git-lfs here, the desk's
// lfs work in its own worktree runs as before, and leaves $G/lfs a folder: track, a commit, ls-files, fsck, a push to
// a remote, checkout, fetch, pull, prune, migrate import on its own branch and dedup.
func TestOnAMacADeskCantReplaceGitLFSsFolderButItsLFSWorkRuns(t *testing.T) {
	git, python := rowsTools(t)
	d, _ := newRowsDesk(t, git)
	lfs := filepath.Join(d.git, "lfs")
	if info, err := os.Lstat(lfs); err != nil || !info.IsDir() {
		t.Fatalf("$G/lfs after the start: %v, want a folder the core made", err)
	}
	mustWrite(t, filepath.Join(d.root, "x", "y"), "")
	d.probe(t, python, [][]string{{"rmdir", lfs}, {"rename", lfs, filepath.Join(d.root, "aside")}, {"chmod", lfs},
		{"mkdir", lfs + "/objects"}, {"write", lfs + "/objects/standin"}, {"unlink", lfs + "/objects/standin"}},
		[]string{"PermissionError", "PermissionError", "PermissionError", "ok", "ok", "ok"})
	// The owner removes it while the desk runs: the desk makes no link there, and may make the folder again.
	if err := os.RemoveAll(lfs); err != nil {
		t.Fatal(err)
	}
	d.probe(t, python, [][]string{{"symlink", lfs, filepath.Join(d.root, "x")}, {"mkdir", lfs}, {"rmdir", lfs}},
		[]string{"PermissionError", "ok", "PermissionError"})
	if info, err := os.Lstat(lfs); err != nil || !info.IsDir() {
		t.Fatalf("$G/lfs after the desk's probes: %v, want a folder", err)
	}

	found, err := exec.LookPath("git-lfs")
	if err != nil {
		t.Skipf("SKIPPED the lfs work: no git-lfs here (%v)", err)
	}
	path := "PATH=" + filepath.Dir(found) + ":" + filepath.Dir(git) + ":/usr/bin:/bin"
	env := append(rowsEnv(d, git), path)
	up := filepath.Join(d.home, "up.git")
	outside(t, d.home, env, git, "init", "-q", "--bare", up)
	outside(t, d.checkout, env, git, "lfs", "install", "--local")
	outside(t, d.checkout, env, git, "remote", "add", "origin", "file://"+up)
	if err := d.prepared(t); err != nil {
		t.Fatalf("the start with git-lfs set up: %v", err)
	}
	mustWrite(t, filepath.Join(d.root, "big.bin"), strings.Repeat("lfs stand-in\n", 64))
	mustWrite(t, filepath.Join(d.root, "more.dat"), strings.Repeat("dat stand-in\n", 64))
	extra := append(DeskGitConfig(gitTestEnv(git)), path)
	work := func(steps ...[]string) {
		t.Helper()
		for _, args := range steps {
			if r := d.inside(t, extra, git, args...); r.status != 0 {
				t.Errorf("the desk's git %s: %+v", strings.Join(args, " "), r)
			}
		}
	}
	work([]string{"lfs", "track", "*.bin"}, []string{"add", ".gitattributes", "big.bin", "more.dat"}, []string{"commit", "-q", "-m", "lfs"},
		[]string{"lfs", "ls-files"}, []string{"lfs", "fsck"}, []string{"push", "-q", "origin", "hire/h1/work"})
	if err := os.Remove(filepath.Join(d.root, "big.bin")); err != nil {
		t.Fatal(err)
	}
	work([]string{"checkout", "--", "big.bin"}, []string{"lfs", "checkout"}, []string{"lfs", "fetch", "origin"},
		[]string{"lfs", "pull", "origin"}, []string{"lfs", "prune"})
	// migrate takes any untracked file for work it would lose: the stand-ins' own go first. It rewrites only the desk's
	// own commits: the first one is develop's too, which no desk moves (rowNotOwnBranches).
	for _, gone := range []string{filepath.Join(d.root, ".claude"), filepath.Join(d.root, "x")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	work(
		[]string{"lfs", "migrate", "import", "--yes", "--include=*.dat", "--include-ref=refs/heads/hire/h1/work",
			"--exclude-ref=refs/remotes/origin/develop"}, []string{"lfs", "fsck"}, []string{"lfs", "dedup"})
	if info, err := os.Lstat(lfs); err != nil || !info.IsDir() {
		t.Errorf("$G/lfs after the desk's lfs work: %v, want a folder", err)
	}
	if got := outside(t, d.root, env, git, "lfs", "ls-files", "-n"); !strings.Contains(got, "big.bin") || !strings.Contains(got, "more.dat") {
		t.Errorf("the owner's git lfs ls-files: %q, want big.bin and more.dat", got)
	}
}
