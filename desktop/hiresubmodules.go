package desktop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A hire's submodules, on a Mac (FXHIREPROJECTSR9, RT126FIXREVIEW-HIREPROJECTSR8 B2). git keeps a worktree's
// submodules' git folders in that worktree's registration, worktrees/<id>/modules/<name>, and the owner's plain
// `git status` in the worktree goes into each one checked out there, with its config. A desk's Policy holds that
// folder whole (the sandbox package's "a worktree's submodules' git folders"), so the hire can't initialise a
// submodule itself: the core does it as it makes the worktree, for each submodule the owner's own checkout has
// initialised. One the owner hasn't stays as it is, uninitialised.
//
// It's `git submodule update --init -- <path>` with `--reference` to the owner's own git folder of that submodule, made
// on config the core wrote (SA-X7's rule for a repo a desk may write): the core runs no git in the project's repo but
// two reads of refs and objects (rev-parse, ls-tree), and none that reads the project's config or .gitmodules. For each
// gitlink in the worktree's HEAD whose path in the owner's checkout has a .git naming a git folder (the owner's
// <common>/modules/<name>, or an older submodule's own .git folder):
//   - a new git folder at worktrees/<id>/modules/<name>, made by `git init --separate-git-dir` in the submodule's
//     empty folder in the worktree (no template, no config but git's own defaults, and the .git file that names it);
//   - the owner's objects as its alternates (what --reference writes), so nothing is fetched or copied;
//   - `checkout --detach` of the commit the gitlink names, run with gitReadArgs (no hook, no fsmonitor, no
//     transport, so an object the owner's folder lacks fails it rather than fetching) and the owner's line ends.
//
// A step that fails leaves that submodule uninitialised, as before, and the log says why. Nested submodules aren't
// initialised (as `update --init` without --recursive). All of it is bounded by worktreeTimeout.

// initHireSubmodules checks out, in the hire's new worktree (tree, of the owner's checkout repo), each submodule the
// owner's checkout has initialised, as the package comment says. git is the hire's gitRunner for the reads in the
// repo; the submodules' own git folders, the core's, are run by gitRun. It says how many it made.
func initHireSubmodules(repo, tree string, git gitRunner) (made int) {
	if repo == "" || tree == "" {
		return 0
	}
	status, out, ok := git(tree, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !ok || status != 0 || len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return 0
	}
	registration, common := physical(lines[0]), physical(lines[1])
	if filepath.Dir(registration) != filepath.Join(common, "worktrees") {
		return 0 // not a worktree's own registration
	}
	gitlinks, ok := gitlinksListed(git, tree, gitlinksOfTree("HEAD")...)
	if !ok {
		log.Printf("smolcorp: the hire's worktree %s: git couldn't list its submodules, so none is checked out", tree)
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), worktreeTimeout)
	defer cancel()
	for _, rel := range gitlinks {
		status, out, ok := git(tree, "rev-parse", "-q", "--verify", "HEAD:"+rel)
		commit := strings.TrimSpace(out)
		if !ok || status != 0 || len(commit) < 40 || strings.Trim(commit, "0123456789abcdef") != "" {
			log.Printf("smolcorp: the submodule %s wasn't checked out in the hire's worktree %s: git couldn't name its commit", rel, tree)
			continue
		}
		if err := initHireSubmodule(ctx, repo, tree, registration, common, rel, commit); err != nil {
			log.Printf("smolcorp: the submodule %s wasn't checked out in the hire's worktree %s: %v", rel, tree, err)
			continue
		}
		made++
	}
	return made
}

// errNotInitialised is a submodule the owner's checkout hasn't initialised: left as it is.
var errNotInitialised = errors.New("your checkout hasn't initialised it")

// initHireSubmodule makes the submodule at rel (as git spells it) in tree, on commit, from the owner's git folder of it.
func initHireSubmodule(ctx context.Context, repo, tree, registration, common, rel, commit string) error {
	owners, name, err := ownersSubmodule(repo, common, rel)
	if err != nil {
		return err
	}
	// Each part of the way is looked at by itself, as gitFindsRepoAt looks (RT126FIXREVIEW-HIREPROJECTSR9 O4): a folder,
	// never a link, so the owner's submodule is never written outside the worktree through a link on the way.
	folder, whole := tree, true
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			whole = false
			break
		}
		folder = filepath.Join(folder, part)
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() {
			whole = false
			break
		}
	}
	if !whole || !emptyFolder(folder) {
		return errors.New("its folder in the worktree isn't the empty one git made")
	}
	own := filepath.Join(registration, "modules", filepath.FromSlash(name))
	if _, err := os.Lstat(own); !errors.Is(err, os.ErrNotExist) {
		return errors.New("its git folder is there already")
	}
	if err := os.MkdirAll(filepath.Dir(own), 0o755); err != nil {
		return err
	}
	undo := func(why error) error {
		_ = os.RemoveAll(own)
		_ = os.Remove(filepath.Join(folder, ".git"))
		if entries, err := os.ReadDir(folder); err == nil {
			for _, e := range entries {
				_ = os.RemoveAll(filepath.Join(folder, e.Name()))
			}
		}
		return why
	}
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return errors.New("the git on this computer is missing, or older than 2.32")
	}
	call := gitCall{global: gitReadArgs, config: here.owner}
	if status, said, err := gitRun(ctx, folder, call, []string{"init", "-q", "--template=", "--separate-git-dir", own}); err != nil || status != 0 {
		return undo(errors.Join(err, fmt.Errorf("git init: %s", firstRunes(said, 200))))
	}
	objects := gitFoldersOf(owners)
	if err := os.WriteFile(filepath.Join(own, "objects", "info", "alternates"), []byte(filepath.Join(objects[len(objects)-1], "objects")+"\n"), 0o644); err != nil {
		return undo(err)
	}
	owner, command := gitOwnerLineEnds(ctx)
	call.config = append(append(call.config, owner...), command...)
	call.env = []string{"GIT_DIR=" + own, "GIT_WORK_TREE=" + folder}
	if status, said, err := gitRun(ctx, folder, call, []string{"checkout", "-q", "--detach", commit}); err != nil || status != 0 {
		return undo(errors.Join(err, fmt.Errorf("git checkout: %s", firstRunes(said, 200))))
	}
	return nil
}

// ownersSubmodule is the owner's git folder of the submodule at rel in the checkout repo, links resolved, and the name
// git keeps it by: the folder the submodule's .git file names in the repo's common folder's modules (its name, the
// path in there), or an older submodule's own .git folder (named by its path). Each folder on the way is looked at by
// its own name, with no link followed, as git looks at a gitlink's path.
func ownersSubmodule(repo, common, rel string) (string, string, error) {
	if !gitFindsRepoAt(repo, rel) {
		return "", "", errNotInitialised
	}
	dotGit := filepath.Join(repo, filepath.FromSlash(rel), ".git")
	info, err := os.Lstat(dotGit)
	switch {
	case err != nil:
		return "", "", errNotInitialised
	case info.IsDir():
		return physical(dotGit), rel, nil
	case !info.Mode().IsRegular():
		return "", "", errors.New("its .git in your checkout is a link")
	}
	gits := gitFoldersOf(dotGit)
	modules := filepath.Join(common, "modules")
	if len(gits) < 2 || !sandbox.Inside(runtime.GOOS, gits[1], modules) || sandbox.Inside(runtime.GOOS, modules, gits[1]) {
		return "", "", errors.New("its .git in your checkout names a git folder outside your repository's modules")
	}
	name, err := filepath.Rel(physical(modules), gits[1])
	if err != nil || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
		return "", "", errors.New("its git folder's name couldn't be told")
	}
	return gits[1], filepath.ToSlash(name), nil
}

// emptyFolder is whether folder holds nothing.
func emptyFolder(folder string) bool {
	entries, err := os.ReadDir(folder)
	return err == nil && len(entries) == 0
}
