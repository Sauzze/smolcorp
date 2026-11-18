package desktop

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// deskRun (SA-X6, SANDBOX-PLAN §A3.5.1): what the core runs for a desk, on the tree the desk writes, runs in that
// desk's sandbox. That is git on a hire's own worktree, wherever the core runs it (fire, clean-up, undo, Take over,
// the demo fence, the GM's scope: every caller of runGit and scopeGit), and R1's `gh pr view` for a hire's branch.
//
//   - A hire's own worktree is told by the registry, never by the tree: runGit looks up whether the folder it is given
//     is in a hire's worktree, and if so runs git with the worktree named. GIT_DIR is its registration in the repo's
//     common git folder, found from the owner's checkout; GIT_WORK_TREE is its top. The worktree's .git file, which
//     the hire writes, is never read: one rewritten to point at a git folder the hire made, with an fsmonitor in its
//     config, runs nothing (RSBXPLAN P3).
//   - Each such git runs with gitstate's hardened environment and arguments (gitReadEnv, gitReadArgs): no hook, no
//     fsmonitor, no pager, no owner or system config, no transport, no prompt. Before a command that reads the files,
//     the repo's config is read: every filter it names is blanked, and a key a read doesn't know refuses the call
//     (gitRepoConfig). So a setting the hire put in the shared config (core.fsmonitor, core.hooksPath) runs nothing
//     (RSBXPLANb R4), and git older than 2.32, which can't be given those settings, isn't run on a desk's tree.
//   - It runs in the hire's sandbox when the hire has one: on a Mac, a desk whose start wrote its profile
//     (hires/<id>/sandbox.sb, SA-M6), through its start's own runner (deskSandboxCommand). A profile that is there but
//     whose sandbox doesn't hold refuses the call: it never runs outside instead. A desk with no profile (the sandbox
//     off, AQ1; a desk from before the update; Linux until SA-L2) gets the hardened git alone.
//   - The fire's removal of the tree (removeDeskTree) never lets git or a path walk delete it: removing a desk's
//     place is just what its sandbox denies it, and git's own removal walks the tree by path, so a link a desk process
//     that outlived its session swaps in for a folder on the way leads the deletion out of the tree (RT126SAX6 B1). The
//     core moves the tree aside in its parent, where no desk can write, so nothing a desk runs can change it any more;
//     checks it again there in the desk's sandbox, so work added after the fire's first look is kept (N7); then
//     deletes it, and its registration, through a handle on each folder that never follows a link (os.Root). The
//     branch's `branch -d` runs in the checkout, named and hardened. A worktree with submodules (or a locked one) is
//     kept, as git keeps it.
//   - A desk's status never goes into a submodule (--ignore-submodules=all, policy.go's ask): a submodule's config,
//     in the desk's registration, is the desk's to write off a Mac (a Mac's Policy holds it, FXHIREPROJECTSR9).
//
// Every other git call (in the owner's checkout, or a folder no hire works in) runs as before: SA-X7 hardens those in
// the owner's repo. gitstate's frequent read-only pollers keep their hardened environment, run outside (§A3.5.1). The
// checkpoint store runs no git that writes in a desk's repo (CK1's pin): cpRepoReads holds it to that.

// deskRunHandshake is how long a run in a desk's sandbox may take to report that its sandbox holds.
const deskRunHandshake = 5 * time.Second

// errNoRegistration: the registry's worktree isn't one the owner's repo has, by the repo's own records.
var errNoRegistration = errors.New("the hire's worktree has no registration in its repo's common git folder")

// gitReadsNoFiles are the commands run on a desk's tree that read no file's content, so no filter can run: they run
// without the config read first. ls-tree reads a tree's objects only, never a working file (the start's gitlink look,
// gitlinkWords).
var gitReadsNoFiles = map[string]bool{"rev-parse": true, "symbolic-ref": true, "ls-tree": true}

// deskTree is a hire's own worktree, as the core names it to git.
type deskTree struct {
	top    string // the worktree's top, links resolved
	repo   string // the owner's checkout it is a worktree of, links resolved
	common string // the repo's common git folder
	gitDir string // the worktree's registration in it
}

// deskGitFor is the run of git with args in dir when dir is a hire's own tree (or the fire's removal of one in its
// checkout), by the registry: mine false for any other folder.
func deskGitFor(dir string, args []string) (run func() (int, string, bool), mine bool) {
	runAs, mine := deskGitForAs(dir, args)
	if !mine {
		return nil, false
	}
	return func() (int, string, bool) {
		status, out, _, ok := runAs(false)
		return status, out, ok
	}, true
}

// deskGitForAs is deskGitFor, its run splitting git's standard output from its standard error when asked (runGitOut);
// else errs is "" and the output is both, as runGit gives it.
func deskGitForAs(dir string, args []string) (run func(split bool) (status int, out, errs string, ok bool), mine bool) {
	if dir == "" || len(args) == 0 {
		return nil, false
	}
	real := physical(dir)
	hires := readRegistry(registryPath(), false).hires
	if h, ok := hireTreeHolding(hires, real); ok {
		return func(split bool) (int, string, string, bool) { return gitOnDeskTree(h, real, false, split, args) }, true
	}
	for _, h := range hires {
		if !ownWorktree(h) || physical(h.Repo) != real {
			continue
		}
		switch removalOf(h, args) {
		case removeTree:
			return func(bool) (int, string, string, bool) {
				status, out, ok := removeDeskTree(h)
				return status, out, "", ok
			}, true
		case removeBranch:
			return func(split bool) (int, string, string, bool) { return gitOnDeskTree(h, real, true, split, args) }, true
		}
	}
	return nil, false
}

// ownWorktree is whether h was hired with a worktree of its own: anywhere but the owner's checkout itself or a folder
// that holds it, inside the checkout included (a configured hires folder there, RT126SAX6 B3). A call in the
// checkout is never taken for one on the hire's tree: a folder the tree doesn't hold isn't in it.
func ownWorktree(h Hire) bool {
	if !filepath.IsAbs(h.Worktree) || !filepath.IsAbs(h.Repo) {
		return false
	}
	return !within(physical(h.Worktree), physical(h.Repo))
}

// hireTreeHolding is the hire whose own worktree holds folder (resolved), an open one first. A registry row whose
// tree holds its checkout is no worktree of its own, and a folder in it but not in the checkout is held by it too:
// deskGitFor refuses that one's git rather than run it raw.
func hireTreeHolding(hires []Hire, folder string) (Hire, bool) {
	var found Hire
	ok := false
	for _, h := range hires {
		held := filepath.IsAbs(h.Worktree) && within(physical(h.Worktree), folder)
		if held && !ownWorktree(h) && filepath.IsAbs(h.Repo) && within(physical(h.Repo), folder) {
			held = false // the owner's checkout, inside it: not the hire's tree
		}
		if held && (!ok || !isOpen(found) && isOpen(h)) {
			found, ok = h, true
		}
	}
	return found, ok
}

const (
	notRemoval = iota
	removeTree
	removeBranch
)

// removalOf is which of the fire's calls in h's checkout args are: its worktree removed, its branch deleted
// (removeWorkspace).
func removalOf(h Hire, args []string) int {
	switch {
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && args[2] == "--force" && physical(args[3]) == physical(h.Worktree):
		return removeTree
	case len(args) == 3 && args[0] == "branch" && args[1] == "-d" && args[2] == h.Branch:
		return removeBranch
	}
	return notRemoval
}

// scopeGit is the GM's scope's and the demo fence's git: a hire's own tree as runGit runs it, and any other folder
// (one a desk made with git itself, unknown to the registry) as runGit runs it there too (ownerGit).
func scopeGit(dir string, args ...string) (int, string, bool) {
	return runGit(dir, args...)
}

// gitOnDeskTree runs git with args in dir for hire h: on its tree, in its sandbox when it has one, or, for the
// branch's deletion at the fire (inCheckout), in its checkout outside any sandbox. ok false: it couldn't be run, or was
// refused. split as deskGitFor's run says (deskGitAs).
func gitOnDeskTree(h Hire, dir string, inCheckout, split bool, args []string) (int, string, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	refused := func(why string) (int, string, string, bool) {
		status, out, ok := deskRefused(h, args, why)
		return status, out, "", ok
	}
	if !ownWorktree(h) {
		return refused("its worktree holds the owner's checkout")
	}
	// A folder of the tree that isn't there (the fire removed it): git couldn't go into it either, so nothing is run or
	// followed, and the answer is git's own, as it was before deskRun. SA-M6's stop after a core restart, which asks
	// where the repo is once the fire removed the tree, reads it as no repo and still lists the desk (MSYSAX6).
	if _, err := os.Lstat(dir); !inCheckout && errors.Is(err, fs.ErrNotExist) {
		words := "fatal: cannot change to '" + dir + "': No such file or directory"
		if split {
			return 128, "", words, true
		}
		return 128, words, "", true
	}
	// The branch's deletion comes after the worktree's removal, which took its registration: it needs none.
	t, err := deskTreeOf(ctx, h, !inCheckout)
	if err != nil {
		return refused(err.Error())
	}
	if inCheckout {
		// In the checkout, outside any sandbox, as every git the core runs there: the branch is deleted in a git
		// folder of the core's own and carried into the repo (ownergit.go), so no hook a config names fires.
		status, out, err := gitWriteThroughFolder(ctx, t.repo, "", "", "", nil, args)
		if err != nil {
			return refused(err.Error())
		}
		return status, out, "", true
	}
	return deskGitAs(ctx, h, h.ID, t.gitDir, t.top, dir, args, split)
}

// deskRefused is the answer for a git on h's tree the core couldn't take, and its line in the log: one that wasn't
// run, or failed, was refused; one that ran and answered past the cap was too big (FXWINWORKTREER4, the R3 review's O1).
func deskRefused(h Hire, args []string, why string) (int, string, bool) {
	what := "was refused"
	if why == errGitTooBig.Error() {
		what = "answered more than the core takes"
	}
	log.Printf("smolcorp: git on %s %s (%s): %s", treeOf(h), what, strings.Join(args, " "), why)
	return 0, "", false
}

// deskGit is git with args in dir, the repo named (gitDir, top), hardened, in desk's sandbox (desk "" for none). A
// status never goes into a submodule: --ignore-submodules=all, whatever the caller asked.
func deskGit(ctx context.Context, h Hire, desk, gitDir, top, dir string, args []string) (int, string, bool) {
	status, out, _, ok := deskGitAs(ctx, h, desk, gitDir, top, dir, args, false)
	return status, out, ok
}

// deskGitAs is deskGit with git's standard output apart from its standard error when split (FXWINWORKTREER1): out is
// then what git answered alone, and errs what it said on standard error. Without split, errs is "" and out is both, as
// deskGit gives it. The config's look before it reads both, as ever: a word there that isn't config refuses the run.
func deskGitAs(ctx context.Context, h Hire, desk, gitDir, top, dir string, args []string, split bool) (int, string, string, bool) {
	return deskGitTo(ctx, h, desk, gitDir, top, dir, args, split, nil)
}

// deskGitTo is deskGitAs with git's standard output handed to stdout as it comes, when not nil (gitCall's stdout,
// FXWINWORKTREER4): out is then empty, its standard error apart as split's, and whether the copy went whole is
// stdout's to say. Only a read that reads files takes it (the fire's listing of the index): a write keeps its answer.
func deskGitTo(ctx context.Context, h Hire, desk, gitDir, top, dir string, args []string, split bool, stdout io.Writer) (int, string, string, bool) {
	refused := func(why string) (int, string, string, bool) {
		status, out, ok := deskRefused(h, args, why)
		return status, out, "", ok
	}
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return refused("the git on this computer is missing, or older than 2.32")
	}
	git, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(git) {
		return refused("no git by a whole path")
	}
	if args[0] == "status" {
		args = append([]string{"status", "--ignore-submodules=all"}, slices.DeleteFunc(slices.Clone(args[1:]), func(arg string) bool {
			return strings.HasPrefix(arg, "--ignore-submodules")
		})...)
	}
	// Outside a sandbox (FXGITSTATECFGR1, RT126GITSTATECFG B3-1), git never runs on the repo's config where it could
	// run a program: the config read below and the git after it are two runs, and a desk can change the config in
	// between. A command that reads the working files, or writes in a form the core carries, runs through a git
	// folder of the core's own (ownergit.go), with the repo named as the registry names it (the registration is in
	// worktrees/ of the common git folder: deskTreeOf); one with no such form isn't run.
	if !gitReadsNoFiles[args[0]] && !deskRunsSandboxed(desk) {
		var status int
		var out, errs string
		var err error
		switch {
		case gitReadsFilesThrough[args[0]]:
			status, out, errs, err = gitFilesThroughFolderTo(ctx, dir, gitDir, top, nil, args, split, stdout)
		case gitWritesThrough(args):
			status, out, err = gitWriteThroughFolder(ctx, dir, gitDir, top, filepath.Dir(filepath.Dir(gitDir)), nil, args)
		default:
			return refused("this desk has no sandbox, and the command has no form on config of the core's own")
		}
		if err != nil {
			return refused(err.Error())
		}
		return status, out, errs, true
	}
	config := slices.Clip(here.owner)
	// run is git with args, its standard error in errs when errs isn't nil, else with its output; its standard output
	// to to as it comes when to isn't nil.
	run := func(errs *gitOutput, to io.Writer, args ...string) ([]byte, int, error) {
		env := append(gitReadEnv(os.Environ(), config), "GIT_DIR="+gitDir, "GIT_WORK_TREE="+top)
		var out gitOutput
		stderr := &out
		if errs != nil {
			stderr = errs
		}
		if to == nil {
			to = &out
		}
		err := deskRun(ctx, desk, dir, env, to, stderr, git, append(slices.Clone(gitReadArgs), args...)...)
		var exit *exec.ExitError
		switch {
		case out.over || stderr.over:
			return nil, 0, errGitTooBig
		case errors.As(err, &exit) && ctx.Err() == nil:
			return out.Bytes(), exit.ExitCode(), nil
		case err != nil:
			return nil, 0, err
		}
		return out.Bytes(), 0, nil
	}
	if !gitReadsNoFiles[args[0]] {
		list, status, err := run(nil, nil, "config", "--list", "-z")
		if err != nil || status != 0 {
			return refused("its config couldn't be read: " + errorWords(err))
		}
		blanks, held := gitRepoConfig(list)
		if held != "" {
			return refused("its config has " + held)
		}
		config = append(config, blanks...)
		// The owner's line ends, as their git has them around the repo's own (RT126CRLFRULING), so the fire's look in
		// the desk's sandbox agrees with the owner's git on a file the hire's git wrote CRLF. Read after the list, so
		// they're never taken for the repo's.
		config = append(config, gitLineEndsAround(ctx, list)...)
	}
	var errs *gitOutput
	if split || stdout != nil {
		errs = &gitOutput{}
	}
	out, status, err := run(errs, stdout, args...)
	if err != nil {
		return refused(err.Error())
	}
	said := ""
	if errs != nil {
		said = strings.TrimSpace(errs.String())
	}
	return status, strings.TrimSpace(string(out)), said, true
}

// removeDeskTree is the fire's removal of h's own worktree, once removeWorkspace found it clean (RT126SAX6 B1, N7):
//  1. The registration is the repo's own (deskTreeOf). One with submodules, or locked, is kept, as git keeps it.
//  2. The tree is moved aside, by its name in its parent, which no desk can write: from here no path a desk's sandbox
//     allows leads into it, so a desk process that outlived its session can't add work to it or swap a link into it.
//  3. It is checked again there, in the desk's sandbox: anything added since the fire's first look (or a checkout of a
//     submodule's) keeps it, put back where it was; so does a .git file that doesn't name its registration.
//  4. It is deleted through a handle on each folder, opened with no link followed (os.Root's RemoveAll: openat with
//     O_NOFOLLOW, unlinkat), so nothing outside it can be reached, whatever is swapped in on the way; then its
//     registration, the same way (git's own removal walks both by path).
//
// Status 0 is removed; 1 with words is kept; ok false is refused (couldn't tell).
func removeDeskTree(h Hire) (int, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	args := []string{"worktree", "remove", h.Worktree}
	refuse := func(why string) (int, string, bool) { return deskRefused(h, args, why) }
	t, err := deskTreeOf(ctx, h, true)
	if err != nil {
		return refuse(err.Error())
	}
	// keep is the answer for a tree kept, with why, and its line in the log (FXWINWORKTREER1): a kept tree is never
	// a quiet answer.
	keep := func(why, seen string) (int, string, bool) {
		log.Printf("smolcorp: %s %s is kept: %s", treeOf(h), t.top, seen)
		return 1, why, true
	}
	for _, kept := range []string{"modules", "locked"} {
		if _, err := os.Lstat(filepath.Join(t.gitDir, kept)); !errors.Is(err, fs.ErrNotExist) {
			return keep("the worktree has submodules, or is locked, so it is kept", "its registration has "+kept+", so it is kept as git keeps it")
		}
	}
	parent, err := os.OpenRoot(filepath.Dir(t.top))
	if err != nil {
		return refuse(err.Error())
	}
	defer parent.Close()
	name := filepath.Base(t.top)
	aside := "." + name + ".fired-" + strings.ToLower(rand.Text()[:10])
	if info, err := parent.Lstat(name); err != nil || !info.IsDir() {
		return refuse("the worktree isn't a folder")
	}
	if err := parent.Rename(name, aside); err != nil {
		return refuse("it couldn't be moved aside: " + err.Error())
	}
	moved := filepath.Join(filepath.Dir(t.top), aside)
	why := deskDotGitNamesIt(parent, aside, t)
	if why == "" {
		why = deskTreeHolds(ctx, h, t.gitDir, moved)
	}
	if why != "" {
		if err := parent.Rename(aside, name); err != nil {
			log.Printf("smolcorp: %s is kept at %s: it couldn't be put back (%v)", treeOf(h), moved, err)
		}
		return keep(why, why)
	}
	if err := parent.RemoveAll(aside); err != nil {
		return refuse("it couldn't be removed whole, and what is left is at " + moved + ": " + err.Error())
	}
	worktrees, err := os.OpenRoot(filepath.Dir(t.gitDir))
	if err == nil {
		err = worktrees.RemoveAll(filepath.Base(t.gitDir))
		worktrees.Close()
	}
	if err != nil {
		return refuse("its registration couldn't be removed: " + err.Error())
	}
	return 0, "", true
}

// treeOf is how the log names h's worktree: "<hire>'s worktree", or "the worktree" for a tree with no hire's row
// (ownergit.go's stand-in, which has no ID), FXWINWORKTREER2's O2.
func treeOf(h Hire) string {
	if h.ID == "" {
		return "the worktree"
	}
	return calledHire(h) + "'s worktree"
}

// deskDotGitNamesIt is why the tree moved aside (aside, in parent) must be kept for its .git file ("" for none): one
// that isn't a file naming its registration is kept, as git keeps it. It is read as data only, through parent, and git
// never follows it (P3).
func deskDotGitNamesIt(parent *os.Root, aside string, t deskTree) string {
	data, err := cpRootData(parent, filepath.Join(aside, ".git"), 4096)
	named, found := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if err != nil || !found {
		return "its .git isn't a file that names its registration"
	}
	if !filepath.IsAbs(named) {
		named = filepath.Join(t.top, named) // worktree.useRelativePaths
	}
	if physical(named) != physical(t.gitDir) {
		return "its .git names another git folder than its registration"
	}
	return ""
}

// deskTreeHolds is why h's tree, moved aside to top, must be kept ("" for none): a change git sees, or a submodule
// checked out in it (status no longer looks in one). In the desk's sandbox, as every git on its tree.
func deskTreeHolds(ctx context.Context, h Hire, gitDir, top string) string {
	// What git answered, and what it said on standard error apart (FXWINWORKTREER1): a nonzero exit, any answer, or a
	// line on standard error harmlessGitWarnings doesn't name keeps the tree, with git's own words (FXWINWORKTREER2).
	// git says it couldn't open a folder only there, with exit 0, and the work in that folder would go with the tree.
	status, out, errs, ok := deskGitAs(ctx, h, h.ID, gitDir, top, top, []string{"status", "--porcelain"}, true)
	switch {
	case !ok:
		return "it holds changes, or git couldn't say"
	case status != 0:
		return "it holds changes, or git couldn't say" + gitSaid(status, errs)
	case out != "":
		return "it holds changes"
	case gitWarningsHeld(errs) != "":
		return "git warned while it looked, so it may not have seen everything" + gitSaid(status, errs)
	}
	// The index's records are git's standard output alone: a warning beside them is no record. Only its submodules
	// are kept, as the listing comes (deskGitlinks): a clean tree's index is as big as the repo.
	links := &deskGitlinks{}
	records := &gitRecords{sep: 0, each: links.take}
	status, _, errs, ok = deskGitTo(ctx, h, h.ID, gitDir, top, top, []string{"ls-files", "-z", "--stage"}, true, records)
	if err := records.end(); err != nil {
		return "git couldn't list it (" + err.Error() + ")"
	}
	if !ok || status != 0 {
		return "git couldn't list it" + gitSaid(status, errs)
	}
	for _, path := range links.paths {
		if _, err := os.Lstat(filepath.Join(top, filepath.FromSlash(path), ".git")); !errors.Is(err, fs.ErrNotExist) {
			return "a submodule is checked out in it"
		}
	}
	return ""
}

// deskGitlinksMost is the most submodules (gitlinks) a fired tree's index may hold: more and the tree is kept.
const deskGitlinksMost = 10000

// deskGitlinks are the submodules of an index git lists (`ls-files -z --stage`) record by record (FXWINWORKTREER4):
// each record is read and only a gitlink's path is kept, so what is held is bounded by the submodules, not the repo.
type deskGitlinks struct {
	paths   []string
	records int
}

// take reads one record, "<mode> <object> <stage>\t<path>": one that isn't, or a gitlink past deskGitlinksMost,
// stops the listing, and the tree is kept.
func (l *deskGitlinks) take(record []byte) error {
	meta, path, found := strings.Cut(string(record), "\t")
	fields := strings.Fields(meta)
	if !found || path == "" || len(fields) != 3 || len(fields[0]) != 6 || strings.Trim(fields[0], "01234567") != "" {
		return fmt.Errorf("record %d of git's listing isn't one it gives: %q", l.records+1, gitWords(string(record), 100))
	}
	l.records++
	if fields[0] != "160000" {
		return nil
	}
	if len(l.paths) == deskGitlinksMost {
		return fmt.Errorf("it has more than %d submodules", deskGitlinksMost)
	}
	l.paths = append(l.paths, path)
	return nil
}

// gitSaid is git's exit and its words on standard error, for a kept tree's reason: " (git ended with 128: fatal: …)".
func gitSaid(status int, errs string) string {
	if status == 0 && errs == "" {
		return ""
	}
	said := " (git ended with " + strconv.Itoa(status)
	if errs != "" {
		said += ": " + gitWords(errs, 200)
	}
	return said + ")"
}

func errorWords(err error) string {
	if err == nil {
		return "git said no"
	}
	return err.Error()
}

// deskTreeOf names h's worktree as the owner's repo records it: the common git folder from the checkout (its .git
// folder, or git's answer there when the checkout is itself a linked worktree), then the registration whose gitdir
// file names the worktree's .git (when registration is asked).
func deskTreeOf(ctx context.Context, h Hire, registration bool) (deskTree, error) {
	t := deskTree{top: physical(h.Worktree), repo: physical(h.Repo)}
	if info, err := os.Lstat(filepath.Join(t.repo, ".git")); err == nil && info.IsDir() {
		t.common = filepath.Join(t.repo, ".git")
	} else {
		out, status, err := readGitOut(ctx, t.repo, gitCall{global: gitReadArgs, config: gitHere().owner},
			"rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil || status != 0 {
			return t, errors.New("the owner's checkout isn't a repo git can read")
		}
		t.common = physical(strings.TrimSpace(string(out)))
	}
	if !registration {
		return t, nil
	}
	worktrees := filepath.Join(t.common, "worktrees")
	entries, err := os.ReadDir(worktrees)
	if err != nil {
		return t, errNoRegistration
	}
	for _, e := range entries {
		reg := filepath.Join(worktrees, e.Name())
		if info, err := os.Lstat(reg); err != nil || !info.IsDir() {
			continue
		}
		data, err := cpReadData(filepath.Join(reg, "gitdir"), 4096)
		named := strings.TrimSpace(string(data))
		if err != nil || named == "" {
			continue
		}
		if !filepath.IsAbs(named) {
			named = filepath.Join(reg, named) // worktree.useRelativePaths
		}
		if filepath.Base(named) == ".git" && physical(filepath.Dir(named)) == t.top {
			t.gitDir = reg
			return t, nil
		}
	}
	return t, errNoRegistration
}

// deskRun runs program with args and the whole environment env in dir, its output to stdout and stderr, for desk id
// ("" for none): in the desk's sandbox when it has one (deskSandboxCommand), else as it is. A desk's run is given env
// without what hireEnvironment keeps from a desk's start (the session, the GM's token, the keys). A sandbox that doesn't
// hold is an error, and the program never runs. It ends with ctx. The error is the run's as exec gives it (an
// *exec.ExitError for an exit code).
func deskRun(ctx context.Context, id, dir string, env []string, stdout, stderr io.Writer, program string, args ...string) error {
	var cmd *exec.Cmd
	var held deskSandboxHeld
	if id != "" {
		env = hireEnvironmentOf(env) // what a desk's start is given, and no more (RT126SAX6 N2)
		var err error
		if cmd, held, err = deskSandboxCommand(id, env, program, args...); err != nil {
			return err
		}
	}
	if cmd == nil {
		cmd = disclaim.Cmd(exec.Command(program, args...))
		cmd.Env = env
	}
	defer held.close()
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stdout, stderr
	cmd.WaitDelay = 2 * time.Second
	hideGitWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()
	stop := func(err error) error {
		_ = cmd.Process.Kill()
		<-ended
		return err
	}
	if err := held.wait(deskRunHandshake); err != nil {
		return stop(err)
	}
	select {
	case err := <-ended:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case <-ctx.Done():
		return stop(ctx.Err())
	}
}

// ghDesk is the context key of the desk a gh run is for (runGH): its id.
type ghDesk struct{}

// withGHDesk is ctx for a gh run on the branch of the folder top: in the sandbox of the hire whose own worktree it is.
func withGHDesk(ctx context.Context, top string) context.Context {
	if top == "" {
		return ctx
	}
	if h, ok := hireTreeHolding(readRegistry(registryPath(), false).hires, physical(top)); ok {
		return context.WithValue(ctx, ghDesk{}, h.ID)
	}
	return ctx
}
