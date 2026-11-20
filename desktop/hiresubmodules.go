package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
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
//   - a new git folder at worktrees/<id>/modules/<name>, made by `git init --separate-git-dir` in a folder of the
//     core's own (no template, no config but git's own defaults and the two stat keys below), checked out there, then
//     carried into the registration through a root (FXSWEEP3P3R4 B5), with the .git file in the submodule's folder
//     naming it;
//   - its files checked out in that folder of the core's own too, then carried into the submodule's folder through
//     the worktree's root the carry kept (FXGMHIRENOGIT126R1, round 4's B6): git never writes in the tree by its path,
//     which whoever can write the folder for hires can swap for a link. Their times are kept, and the git folder told
//     to compare only times and sizes (core.checkStat minimal, core.trustctime false), so git finds them as it left
//     them;
//   - the owner's objects as its alternates (what --reference writes), so nothing is fetched or copied;
//   - `checkout --detach` of the commit the gitlink names, run with gitReadArgs (no hook, no fsmonitor, no
//     transport, so an object the owner's folder lacks fails it rather than fetching) and the owner's line ends.
//
// A step that fails leaves that submodule uninitialised, as before, and the log says why. Nested submodules aren't
// initialised (as `update --init` without --recursive). All of it is bounded by worktreeTimeout.

// initHireSubmodules checks out, in the hire's new worktree (tree, of the owner's checkout repo, on branch), each
// submodule the owner's checkout has initialised, as the package comment says. git is the hire's gitRunner for the
// reads in the repo; the submodules' own git folders, the core's, are run by gitRun. It says how many it made.
//
// Their git folders go into the registration the core carried for the worktree (hireCarried, FXSWEEP3P3R3 B3), still
// the very folder it wrote, and nowhere else: never one the tree's .git names now, which a rewrite since the carry
// would have name another repo's. A tree whose git names another registration gets none.
func initHireSubmodules(repo, tree, branch string, git gitRunner) (made int) {
	if repo == "" || tree == "" {
		return 0
	}
	carried, found := carriedFor(branch)
	if !found {
		log.Printf("smolcorp: the hire's worktree %s: the core finds no registration it carried for it, so no submodule is checked out", tree)
		return 0
	}
	registration, common := carried.registrationPath(), carried.common
	status, out, ok := git(tree, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !ok || status != 0 || len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return 0
	}
	if !oneFolderOnDisk(lines[0], registration) || !oneFolderOnDisk(lines[1], common) {
		log.Printf("smolcorp: the hire's worktree %s: its git names %s, not the registration the core carried, so no submodule is checked out", tree, lines[0])
		return 0
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
		if _, still := carriedFor(branch); !still {
			log.Printf("smolcorp: the hire's worktree %s: the registration the core carried was moved, so no more submodules are checked out", tree)
			break
		}
		if err := initHireSubmodule(ctx, repo, carried.tree, carried.registry, common, rel, commit); err != nil {
			log.Printf("smolcorp: the submodule %s wasn't checked out in the hire's worktree %s: %v", rel, tree, err)
			continue
		}
		made++
	}
	return made
}

// errNotInitialised is a submodule the owner's checkout hasn't initialised: left as it is.
var errNotInitialised = errors.New("your checkout hasn't initialised it")

// initHireSubmodule makes the submodule at rel (as git spells it) in tree, the worktree's folder the core carried,
// opened as a root, on commit, from the owner's git folder of it. Its git folder goes into registry, the worktree's
// registration the core carried, opened as a root.
//
// That registration is in the repo's own git folder, whose folders whoever owns the repo can rename, or swap for a
// link elsewhere, while git runs. So nothing reaches it by its path (FXSWEEP3P3R4 B5): the submodule's git folder is
// made and checked out in a folder of the core's own, then carried in through registry, each folder and file made new
// there; only what was carried is removed again, and only through registry. The submodule's .git names it there, as
// git init would have. The tree is in the folder for hires, which whoever can write it can swap for a link too, so
// the work-tree side goes the same way (round 4's B6): the files are checked out in the core's own folder and carried
// in through tree, the .git file written through it, and the undo removes only through it, never by a path.
func initHireSubmodule(ctx context.Context, repo string, tree, registry *os.Root, common, rel, commit string) error {
	if tree == nil {
		return errors.New("the core kept no root for the worktree's folder")
	}
	owners, name, err := ownersSubmodule(repo, common, rel)
	if err != nil {
		return err
	}
	// Each part of the way is looked at by itself, as gitFindsRepoAt looks (RT126FIXREVIEW-HIREPROJECTSR9 O4): a folder,
	// never a link, so the owner's submodule is never written outside the worktree through a link on the way. Through
	// tree: nothing leads out of it whatever its path leads to now.
	folder, whole := ".", true
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			whole = false
			break
		}
		folder = path.Join(folder, part)
		if info, err := tree.Lstat(folder); err != nil || !info.IsDir() {
			whole = false
			break
		}
	}
	if !whole || !emptyFolderIn(tree, folder) {
		return errors.New("its folder in the worktree isn't the empty one git made")
	}
	own := path.Join("modules", name)
	if _, err := registry.Lstat(own); !errors.Is(err, fs.ErrNotExist) {
		return errors.New("its git folder is there already")
	}
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return errors.New("the git on this computer is missing, or older than 2.32")
	}
	if err := sweepGitReads(); err != nil {
		return err
	}
	made, err := os.MkdirTemp(gitReadsFolder(), "module-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(made)
	gitDir, work := filepath.Join(made, "git"), filepath.Join(made, "work")
	carried := false
	undo := func(why error) error {
		if carried {
			_ = registry.RemoveAll(own)
		}
		// Through tree only: what is in the submodule's folder there now is what this step put there.
		if entries, err := readDirIn(tree, folder); err == nil {
			for _, e := range entries {
				_ = tree.RemoveAll(path.Join(folder, e.Name()))
			}
		}
		return why
	}
	call := gitCall{global: gitReadArgs, config: here.owner}
	if status, said, err := gitRun(ctx, made, call, []string{"init", "-q", "--template=", "--separate-git-dir", gitDir, work}); err != nil || status != 0 {
		return undo(errors.Join(err, fmt.Errorf("git init: %s", firstRunes(said, 200))))
	}
	objects := gitFoldersOf(owners)
	if err := os.WriteFile(filepath.Join(gitDir, "objects", "info", "alternates"), []byte(filepath.Join(objects[len(objects)-1], "objects")+"\n"), 0o644); err != nil {
		return undo(err)
	}
	// The files are carried, so their inode and change time aren't the ones git's index has: git compares their times
	// and sizes only, which the carry keeps, so a status finds them clean without reading each again.
	if err := appendOwnFile(filepath.Join(gitDir, "config"), "[core]\n\tcheckStat = minimal\n\ttrustctime = false\n"); err != nil {
		return undo(err)
	}
	owner, command := gitOwnerLineEnds(ctx)
	call.config = append(append(call.config, owner...), command...)
	call.env = []string{"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + work}
	if status, said, err := gitRun(ctx, work, call, []string{"checkout", "-q", "--detach", commit}); err != nil || status != 0 {
		return undo(errors.Join(err, fmt.Errorf("git checkout: %s", firstRunes(said, 200))))
	}
	if err := carryWorkTree(tree, work, folder); err != nil {
		return undo(err)
	}
	if err := registry.MkdirAll(path.Dir(own), 0o755); err != nil {
		return undo(err)
	}
	if err := registry.Mkdir(own, 0o755); err != nil {
		return undo(fmt.Errorf("its git folder is there already: %w", err))
	}
	carried = true
	if err := carryGitFolder(registry, gitDir, own); err != nil {
		return undo(err)
	}
	dotGit := []byte("gitdir: " + filepath.Join(registry.Name(), filepath.FromSlash(own)) + "\n")
	at, err := tree.OpenFile(path.Join(folder, ".git"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		_, err = at.Write(dotGit)
		err = errors.Join(err, at.Close())
	}
	if err != nil {
		return undo(err)
	}
	return nil
}

// carryWorkTree copies the files git checked out in work, a folder of the core's own, into folder in tree, through
// tree only: each folder, file and link made new there (never one that was there), each file with its mode and its
// times. The .git file git init wrote at work's top isn't carried: the submodule's own is written after.
func carryWorkTree(tree *os.Root, work, folder string) error {
	return carryWorkTreeSeen(tree, work, folder, nil)
}

// carryWorkTreeSeen is carryWorkTree, noting in seen (when not nil) each file it carried, by its path from work (as
// git's index names it), as the file it wrote is once its times are set: what refreshCarriedIndex gives the index.
func carryWorkTreeSeen(tree *os.Root, work, folder string, seen map[string]os.FileInfo) error {
	return filepath.WalkDir(work, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || at == work {
			return err
		}
		rel, err := filepath.Rel(work, at)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return nil
		}
		into := path.Join(folder, filepath.ToSlash(rel))
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return tree.Mkdir(into, 0o755)
		case entry.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(at)
			if err != nil {
				return err
			}
			if err := tree.Symlink(target, into); err != nil || seen == nil {
				return err
			}
			// A link's times can't be set through the root: its own keys are noted instead, as long as the name is
			// still the link written, leading where it was told, from one look to the next.
			first, err1 := tree.Lstat(into)
			now, err2 := tree.Readlink(into)
			again, err3 := tree.Lstat(into)
			if errors.Join(err1, err2, err3) == nil && now == target && os.SameFile(first, again) && again.ModTime().Equal(first.ModTime()) {
				seen[filepath.ToSlash(rel)] = again
			}
			return nil
		case !entry.Type().IsRegular():
			return errors.New("git checked out a file that isn't a plain one: " + rel)
		}
		source, err := os.Open(at)
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := tree.OpenFile(into, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer target.Close()
		if _, err = io.Copy(target, source); err != nil {
			return err
		}
		if err := tree.Chtimes(into, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
		if seen != nil {
			// The file written, by its handle: not whatever its name may lead to by now.
			if carried, err := target.Stat(); err == nil {
				seen[filepath.ToSlash(rel)] = carried
			}
		}
		return target.Close()
	})
}

// appendOwnFile adds text at the end of the file at name, a file of the core's own.
func appendOwnFile(name, text string) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, err = file.WriteString(text)
	return errors.Join(err, file.Close())
}

// carryGitFolder copies the git folder from, the core's own, into to, a new folder in root: each folder and file made
// new there (never one that was there), through root only.
func carryGitFolder(root *os.Root, from, to string) error {
	return filepath.WalkDir(from, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || at == from {
			return err
		}
		rel, err := filepath.Rel(from, at)
		if err != nil {
			return err
		}
		into := path.Join(to, filepath.ToSlash(rel))
		switch {
		case entry.IsDir():
			return root.Mkdir(into, 0o755)
		case !entry.Type().IsRegular():
			return errors.New("git made a file that isn't a plain one: " + rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		source, err := os.Open(at)
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := root.OpenFile(into, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(target, source)
		return errors.Join(err, target.Close())
	})
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

// emptyFolderIn is whether folder, in root, holds nothing.
func emptyFolderIn(root *os.Root, folder string) bool {
	entries, err := readDirIn(root, folder)
	return err == nil && len(entries) == 0
}

// readDirIn is what folder, in root, holds, read through root.
func readDirIn(root *os.Root, folder string) ([]fs.DirEntry, error) {
	dir, err := root.Open(folder)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadDir(-1)
}
