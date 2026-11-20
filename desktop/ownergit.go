package desktop

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// The owner's git on a repo a desk may write (FXGITSTATECFGR1, SIERRA's ruling on RT126GITSTATECFG): every git the
// core runs outside a sandbox, on a folder that isn't a sandboxed desk's own tree, runs on config the core wrote, or
// isn't run at all. That is runGit's and scopeGit's every other folder (the repo a hire is made from, a worktree before
// its row is written or once the registry is lost, the demo's project) and deskGit for a desk with no sandbox (the
// sandbox off, a desk from before the update, every desk off a Mac).
//
// Any repo may be one a desk can write: policy.go has no row for repos outside a desk's own places, and a registry
// that can't be read, or has no row for a tree, says nothing either way. So every one is taken for a desk's. git reads
// a repo's config at each run, and a desk can change it between any look and the run (a filter, an fsmonitor, a hook
// in a folder it names, a config-based hook, hook.<name>.command, which core.hooksPath doesn't switch off). So:
//   - A command that reads refs or config only (gitReadsRefsOnly: rev-parse, for-each-ref, symbolic-ref's read, one
//     config key or one pattern's keys, the remotes' names, merge-base --is-ancestor) runs with gitstate's guards
//     (gitReadEnv, gitReadArgs). It reads no working file,
//     writes nothing and fires no hook event, so no key of any name runs a program (RT126GITSTATECFG's P1 ran these
//     after every class of key was flipped in).
//   - A command that reads the working files (status, ls-files, check-ignore, diff) runs through a git folder of the
//     core's own (gitstatefolder.go), as the git state's read does: the repo's config is never opened by it.
//   - A command that writes (a hire's worktree and branch, add, commit, a branch's deletion) runs through a git folder
//     of the core's own too: git writes the objects and the index, which are data, and the refs and a new worktree's
//     registration only in that folder. The core then carries what changed into the repo itself: the one branch it
//     changed, written or deleted as a file through a handle that leads no link out of the repo's git folder (a
//     branch git keeps packed, a branch that moved meanwhile, or refs that aren't files: refused), and the new
//     worktree's registration, copied in.
//   - A worktree's removal is the core's own (removeDeskTree), as a hire's tree's is.
//   - A hire's new branch gets its upstream (presetUpstream, SA-X7), the one config write the core asks of git in
//     the repo itself: `config --local` of that branch's remote and merge, and nothing else (gitPresetsUpstream). A
//     config write reads no working file and fires no hook event, and runs with the refs-only reads' guards. A branch
//     the core deletes loses its section too, as git's own `branch -D` drops it.
//   - Anything else is refused, and the log says why. Nothing falls back to plain git.
//
// SA-X7 (SANDBOX-PLAN §A3.5.1, "New (NB2)") adds, for the owner's repo (runGit's every folder that isn't a hire's own
// tree): before a git that reads or writes its files (a worktree made, add, commit, a status or diff; not a branch's
// deletion, which touches none), the repo's config is judged as SA-X7 judged it (ownerRepoConfig), on the same list
// the core's folder is made from, and an include, a key no read knows or an address that runs a program refuses it,
// in words the owner hears (prepareHire's "Couldn't make the hire's worktree: …"). Through the core's folder none of
// them would be read; the owner is told why rather than given a hire on a config Smolcorp doesn't understand. A
// config-based hook (hook.*) is the one exception for a write: the core's folder leaves it out like every program
// setting, and FXGITSTATECFGR1 lets a hire be made on a repo that has one (a read still refuses it). A git older than
// 2.32 is refused, in words too, for every command that reads or writes files; rev-parse still finds the repo, so the
// owner hears why rather than "not a repository".
//
// What that costs the hire's checkout: a file git-lfs would fetch is checked out as its pointer (the hire runs
// `git lfs pull` in its own sandbox); a partial clone that lacks a file's content can't be checked out (no
// transport); line ends follow the repo's own config and .gitattributes, and the owner's own core.autocrlf, core.eol
// and core.safecrlf, carried by value (gitOwnerLineEnds, RT126CRLFRULING); and git older than 2.32 makes no worktree.

// gitReadsFilesThrough are the commands that read working files, run through a git folder of the core's own.
var gitReadsFilesThrough = map[string]bool{"status": true, "ls-files": true, "check-ignore": true, "diff": true}

// ownerGitNo is a refusal the owner hears (SA-X7): ownerGit gives its words as the output, with ok false, so a
// caller's own message can say them.
type ownerGitNo string

func (no ownerGitNo) Error() string { return "Smolcorp didn't run git in your repo: " + string(no) }

// gitCarryNo is a new worktree the core refused to carry into its tree (carryWorktree), in words for whoever hired:
// git checked it out in a folder of the core's own, and the carry took back what it had put in the repo, so nothing
// is left to clear, and this is no "git didn't answer" (FXGMHIRENOGIT126R3, N1).
type gitCarryNo string

func (no gitCarryNo) Error() string { return string(no) }

// ownerGit is runGit's and scopeGit's git for a folder that isn't a hire's own tree by the registry: ok false when it
// couldn't be run, or was refused (with the refusal's words as the output when the owner is to hear them).
func ownerGit(dir string, args []string) (int, string, bool) {
	status, out, _, ok := ownerGitAs(dir, args, false)
	return status, out, ok
}

// ownerGitAs is ownerGit, with git's standard output apart from its standard error when split (runGitOut): a read's
// answer is then its standard output alone, and errs its standard error. Without split, errs is "" and the output is
// both, as ownerGit gives it.
func ownerGitAs(dir string, args []string, split bool) (status int, out, errs string, ok bool) {
	limit := gitTimeout
	if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
		limit = worktreeTimeout // a checkout of a big repo can take a while
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	status, out, errs, err := ownerGitRunAs(ctx, dir, args, split)
	if err != nil {
		log.Printf("smolcorp: git %s in %s wasn't run as the owner: %v", strings.Join(args, " "), dir, err)
		var no ownerGitNo
		if errors.As(err, &no) {
			return 0, no.Error(), "", false
		}
		var carry gitCarryNo
		if errors.As(err, &carry) {
			return 0, carry.Error(), "", false
		}
		return 0, "", "", false
	}
	return status, out, errs, true
}

func ownerGitRun(ctx context.Context, dir string, args []string) (int, string, error) {
	status, out, _, err := ownerGitRunAs(ctx, dir, args, false)
	return status, out, err
}

// ownerGitRunAs is ownerGitRun, split as ownerGitAs says. Only the reads split (refs only, or the working files): every
// other form answers as ownerGitRun does, with errs "".
func ownerGitRunAs(ctx context.Context, dir string, args []string, split bool) (int, string, string, error) {
	if len(args) == 0 {
		return 0, "", "", errors.New("no command")
	}
	if (gitReadsFilesThrough[args[0]] || gitWritesThrough(args)) && !gitVersionAtLeast(gitHere().version, 2, 32) {
		return 0, "", "", ownerGitNo("the git on this computer is missing, or older than 2.32, which can't be told to leave your git config out.")
	}
	switch {
	case gitPresetsUpstream(args):
		status, out, err := gitPresetIntoCarried(ctx, args)
		return status, out, "", err
	case gitReadsRefsOnly(args):
		call := gitCall{global: gitReadArgs, config: gitHere().owner}
		if split {
			return gitRunSplit(ctx, dir, call, args)
		}
		status, out, err := gitRun(ctx, dir, call, args)
		return status, out, "", err
	case gitReadsFilesThrough[args[0]]:
		return gitFilesThroughFolderAs(ctx, dir, "", "", ownerRepoJudged, args, split)
	}
	status, out, err := ownerGitWrites(ctx, dir, args)
	return status, out, "", err
}

// ownerGitWrites is ownerGitRun's every form but the reads: what writes, and the refusal of any other.
func ownerGitWrites(ctx context.Context, dir string, args []string) (int, string, error) {
	switch {
	case gitWritesThrough(args) && args[0] == "branch":
		// A branch's deletion reads and writes no file: its config isn't judged, as before SA-X7's. A hire's branch
		// takes its upstream with it, through the root the deletion was carried through (gitDropUpstream).
		return gitWriteThroughFolder(ctx, dir, "", "", "", nil, args)
	case gitWritesThrough(args):
		return gitWriteThroughFolder(ctx, dir, "", "", "", ownerRepoJudged, args)
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && args[2] == "--force":
		// The worktree of a hire with no row (yet, or any more): removed as the fire removes a hire's own, where git
		// would run a status on it (and follow its .git file) and walk it by path.
		// Its folder resolved, its own name as given: the removal's checks refuse a link there (checkedTree).
		tree := filepath.Join(physical(filepath.Dir(args[3])), filepath.Base(args[3]))
		status, out, ok := removeDeskTree(Hire{Name: "a worktree with no row", Repo: physical(dir), Worktree: keptPath(tree)})
		if !ok {
			return 0, "", errors.New("its removal was refused")
		}
		return status, out, nil
	}
	return 0, "", errors.New("it has no form on config of the core's own, and a repo a desk may write never gets plain git")
}

// gitReadsRefsOnly is whether args read refs or config only: rev-parse, for-each-ref, symbolic-ref with one name (a
// second one would write it), one config key or the keys one pattern matches (hirePushRemote's remotes), the
// remotes' names, and whether one commit holds another (the fire's merged look, SA-X7). And `ls-tree -r -z
// [<gitlinkFormat>] <tree>`, which reads that tree's objects only, as rev-parse reads a path's object, never a working
// file (the gitlinks of the owner's HEAD, gitlinkWords, and of a new worktree's, initHireSubmodules).
func gitReadsRefsOnly(args []string) bool {
	switch args[0] {
	case "rev-parse", "for-each-ref":
		return true
	case "ls-tree":
		if len(args) == 5 && args[3] == gitlinkFormat {
			args = slices.Delete(slices.Clone(args), 3, 4)
		}
		return len(args) == 4 && args[1] == "-r" && args[2] == "-z" && args[3] != "" && !strings.HasPrefix(args[3], "-")
	case "merge-base":
		return len(args) == 4 && args[1] == "--is-ancestor" && !strings.HasPrefix(args[2], "-") && !strings.HasPrefix(args[3], "-")
	case "symbolic-ref":
		names := 0
		for _, arg := range args[1:] {
			switch {
			case arg == "-d" || arg == "--delete" || arg == "-m":
				return false
			case !strings.HasPrefix(arg, "-"):
				names++
			}
		}
		return names == 1
	case "config":
		return len(args) == 2 && !strings.HasPrefix(args[1], "-") || len(args) == 3 && args[1] == "--get" ||
			len(args) == 4 && args[1] == "-z" && args[2] == "--get-regexp" && !strings.HasPrefix(args[3], "-")
	case "remote":
		return len(args) == 1
	}
	return false
}

// gitPresetsUpstream is whether args are presetUpstream's write, and only it: `config --local
// branch.<hire's branch>.remote <name>`, or `.merge refs/heads/<that branch>`.
func gitPresetsUpstream(args []string) bool {
	if len(args) != 4 || args[0] != "config" || args[1] != "--local" {
		return false
	}
	rest, found := strings.CutPrefix(args[2], "branch.")
	if !found {
		return false
	}
	if branch, found := strings.CutSuffix(rest, ".merge"); found {
		return strings.HasPrefix(branch, "hire/") && gitBranchFile("refs/heads/"+branch) && args[3] == "refs/heads/"+branch
	}
	branch, found := strings.CutSuffix(rest, ".remote")
	return found && strings.HasPrefix(branch, "hire/") && gitBranchFile("refs/heads/"+branch) &&
		args[3] != "" && !strings.HasPrefix(args[3], "-") && !strings.ContainsAny(args[3], " \t\n\r\x00\\")
}

// gitWritesThrough is whether args write in a form the core carries into the repo (gitWriteThroughFolder): add,
// commit, a branch's deletion, a worktree made (in its one form, gitWorktreeAdded, else refused there).
func gitWritesThrough(args []string) bool {
	switch args[0] {
	case "add", "commit":
		return true
	case "branch":
		return len(args) == 3 && (args[1] == "-d" || args[1] == "-D")
	case "worktree":
		return len(args) >= 2 && args[1] == "add"
	}
	return false
}

// gitWorktreeAdded is the new worktree's path in `worktree add [-q] -b <branch> <path> [<commit>]`: a commit only as
// its whole object name (a scoped GM's hire from one of the project's worktrees, made from the project, FXGMSCOPEPINR1).
func gitWorktreeAdded(args []string) (string, bool) {
	rest := args[2:]
	if len(rest) > 0 && rest[0] == "-q" {
		rest = rest[1:]
	}
	if len(rest) == 4 && gitObjectID(rest[3]) {
		rest = rest[:3]
	}
	if len(rest) != 3 || rest[0] != "-b" || strings.HasPrefix(rest[1], "-") || !filepath.IsAbs(rest[2]) {
		return "", false
	}
	return rest[2], true
}

// gitRun runs git with args and call in dir, outside any sandbox: its exit code and its output, standard error with
// it (runGit's callers show it).
func gitRun(ctx context.Context, dir string, call gitCall, args []string) (int, string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return 0, "", err
	}
	env := append(gitReadEnv(os.Environ(), call.config), call.env...)
	var out gitOutput
	err = deskRun(ctx, "", dir, env, &out, &out, git, append(append([]string{"-C", dir}, call.global...), args...)...)
	var exit *exec.ExitError
	switch {
	case out.over:
		return 0, "", errGitTooBig
	case errors.As(err, &exit) && ctx.Err() == nil:
		return exit.ExitCode(), strings.TrimSpace(out.String()), nil
	case err != nil:
		return 0, "", err
	}
	return 0, strings.TrimSpace(out.String()), nil
}

// gitRunSplit is gitRun with git's two streams apart (FXWINWORKTREER1): out is its standard output alone and errs its
// standard error, so an answer compared exactly (runGitOut) is what git answered, never a warning it printed beside
// it. The exit code is git's, as gitRun's. With call.stdout, git's standard output goes there as it comes, and out is
// empty.
func gitRunSplit(ctx context.Context, dir string, call gitCall, args []string) (status int, out, errs string, err error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return 0, "", "", err
	}
	env := append(gitReadEnv(os.Environ(), call.config), call.env...)
	var stdout, stderr gitOutput
	var to io.Writer = &stdout
	if call.stdout != nil {
		to = call.stdout
	}
	err = deskRun(ctx, "", dir, env, to, &stderr, git, append(append([]string{"-C", dir}, call.global...), args...)...)
	var exit *exec.ExitError
	switch {
	case stdout.over || stderr.over:
		return 0, "", "", errGitTooBig
	case errors.As(err, &exit) && ctx.Err() == nil:
		return exit.ExitCode(), strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), nil
	case err != nil:
		return 0, "", "", err
	}
	return 0, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), nil
}

// gitFilesThroughFolder runs args, which read the working files, on the repo dir is in, through a git folder of the
// core's own: gitDir and top name the repo when the caller knows them (a desk's tree, by its registration), "" asks
// git. A config with a key the read doesn't know refuses it (gitRepoConfig), as the git state's read; a status or a
// diff never goes into a submodule. judge, when not nil, is asked about that same config first (ownerRepoJudged).
func gitFilesThroughFolder(ctx context.Context, dir, gitDir, top string, judge func([]byte) error, args []string) (int, string, error) {
	status, out, _, err := gitFilesThroughFolderAs(ctx, dir, gitDir, top, judge, args, false)
	return status, out, err
}

// gitFilesThroughFolderAs is gitFilesThroughFolder, its last run gitRunSplit's when split (errs "" when not).
func gitFilesThroughFolderAs(ctx context.Context, dir, gitDir, top string, judge func([]byte) error, args []string, split bool) (int, string, string, error) {
	return gitFilesThroughFolderTo(ctx, dir, gitDir, top, judge, args, split, nil)
}

// gitFilesThroughFolderTo is gitFilesThroughFolderAs with git's standard output handed to stdout as it comes, when
// not nil (gitCall's stdout, FXWINWORKTREER4): its last run is then gitRunSplit's, and out is empty.
func gitFilesThroughFolderTo(ctx context.Context, dir, gitDir, top string, judge func([]byte) error, args []string, split bool, stdout io.Writer) (int, string, string, error) {
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return 0, "", "", errors.New("the git on this computer is missing, or older than 2.32")
	}
	look := gitCall{global: gitReadArgs, config: here.owner}
	if gitDir != "" {
		look.env = []string{"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + top}
	} else {
		out, status, err := readGitOut(ctx, dir, look, "rev-parse", "--show-toplevel", "--absolute-git-dir")
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if err != nil || status != 0 || len(lines) != 2 {
			return 0, "", "", errors.Join(err, errors.New("it isn't a repo git can read here"))
		}
		top, gitDir = filepath.FromSlash(lines[0]), filepath.FromSlash(lines[1])
	}
	list, status, err := readGitOut(ctx, dir, look, "config", "--list", "-z")
	if err != nil || status != 0 {
		return 0, "", "", errors.Join(err, errors.New("its config couldn't be read"))
	}
	if judge != nil {
		if err := judge(list); err != nil {
			return 0, "", "", err
		}
	}
	blanks, refused := gitRepoConfig(list)
	if refused != "" {
		return 0, "", "", errors.New("its config has a setting a read doesn't know, or an address that runs a program (" + refused + ")")
	}
	folder, err := makeGitReadFolder(ctx, dir, top, gitDir, readGitOut, look, list)
	if err != nil {
		return 0, "", "", err
	}
	defer folder.remove()
	call := folder.call
	call.config = append(call.config, blanks...)
	if gitVersionAtLeast(here.version, 2, 40) {
		call.global = append(call.global, "--attr-source="+gitEmptyTrees[folder.format])
	}
	switch args[0] {
	case "status":
		args = append([]string{"status", "--ignore-submodules=all"}, slices.DeleteFunc(slices.Clone(args[1:]), func(arg string) bool {
			return strings.HasPrefix(arg, "--ignore-submodules")
		})...)
	case "diff":
		args = append([]string{"diff", "--no-ext-diff", "--no-textconv", "--ignore-submodules=all"}, args[1:]...)
	}
	if split || stdout != nil {
		call.stdout = stdout
		return gitRunSplit(ctx, dir, call, args)
	}
	status, out, err := gitRun(ctx, dir, call, args)
	return status, out, "", err
}

// gitFolderLeadsBack is why the git folder git named for top (gitDir, sharing common) isn't top's own, "" when it is.
// The core writes branches and registrations through it, so it must lead back to top (FXSWEEP3P3R1 B1); a .git file
// naming another repo's .git, or one of its registrations, would have it write into a repo nobody hired into. The
// folder the core was asked about is already in top (inTop, FXSWEEP3P3R2 B1), and git takes a git folder's
// core.worktree as the top, so a git folder's own work tree is top unless it is one of these:
//   - a .git folder at the top: it is the git folder (or a link to it);
//   - a .git file naming the git folder, which is either a registration in <common>/worktrees whose own gitdir names
//     top's .git back, as git wrote it, or the shared one;
//   - the shared one, named .git (in any spelling), is its parent's: that parent must be top, or no work tree of it
//     (gitAnotherWorkTreesDotGit). A .git, or a .git file, leading to another work tree's .git is that repo's (as ruled
//     for FXSWEEP3P3R2). One with another name (a submodule's, or one made by --separate-git-dir) has no work tree but
//     the one git took: top.
//
// The way there is judged too, before git resolved it (FXSWEEP3P3R3 N3): a link on the way to another work tree's
// .git is that repo's, even when that .git is itself a link to a folder of another name (gitDotGitHops).
//
// The .git file and the registration's gitdir are read as data, through the core's reader (hirefile.go): never a link.
func gitFolderLeadsBack(ctx context.Context, top, gitDir, common string) string {
	for _, hop := range gitDotGitHops(top)[1:] {
		if why := gitAnotherWorkTreesDotGit(ctx, top, hop, common); why != "" {
			return why
		}
	}
	dotGit := filepath.Join(top, ".git")
	info, err := os.Stat(dotGit)
	switch {
	case err != nil:
		return "it has no .git at its top"
	case info.IsDir():
		if physical(dotGit) != physical(gitDir) || physical(gitDir) != physical(common) {
			return "its .git folder isn't the git folder git named"
		}
	default:
		named, found := gitdirLine(top, dotGit, "gitdir: ")
		if !found || physical(named) != physical(gitDir) {
			return "its .git file doesn't name the git folder git named"
		}
		if physical(gitDir) != physical(common) {
			back, found := gitdirLine(gitDir, filepath.Join(gitDir, "gitdir"), "")
			if !found || physical(back) != physical(dotGit) {
				return "it is another worktree's registration, whose gitdir doesn't name this .git"
			}
			return ""
		}
	}
	return gitAnotherWorkTreesDotGit(ctx, top, physical(gitDir), common)
}

// gitDotGitHops is each place the way from top's .git to its git folder names, as given, before git resolves it
// (FXSWEEP3P3R3 N3): the .git itself, each link it leads through, and, when that ends at a file, the folder its gitdir
// line names and each link on from there. A gitdir line is read once, as git reads it (a second .git file isn't
// followed). At most 40 hops, as the system follows links.
//
// Each place is where the system and git reach it (FXSWEEP3P3R4 N1): its parent resolved on the disk, links first,
// then its own name (gitHopPlace). A ".." is never taken off by name: after a link it leads out of the link's target.
func gitDotGitHops(top string) []string {
	at := filepath.Join(top, ".git")
	hops, read := []string{at}, false
	for len(hops) < 40 {
		info, err := os.Lstat(at)
		switch {
		case err != nil:
			return hops
		case info.Mode()&fs.ModeSymlink != 0:
			to, err := os.Readlink(at)
			if err != nil {
				return hops
			}
			if !filepath.IsAbs(to) {
				to = filepath.Dir(at) + string(filepath.Separator) + to
			}
			at = gitHopPlace(to)
		case info.Mode().IsRegular() && !read:
			named, found := gitdirNamed(top, at, "gitdir: ")
			if !found {
				return hops
			}
			at, read = gitHopPlace(named), true
		default:
			return hops
		}
		hops = append(hops, at)
	}
	return hops
}

// gitHopPlace is raw, a path as a link or a gitdir line gives it (absolute, never cleaned), as the disk resolves it:
// the folder it is in with every link on the way resolved, each before the name after it (so a "lnk/.." leaves the
// link's target, not lnk), and its own name kept as given, so a link there is the next hop. A raw that ends in "." or
// "..", or whose folder isn't there, is resolved or cleaned whole.
func gitHopPlace(raw string) string {
	volume := filepath.VolumeName(raw)
	rest := raw[len(volume):]
	for len(rest) > 1 && os.IsPathSeparator(rest[len(rest)-1]) {
		rest = rest[:len(rest)-1]
	}
	cut := len(rest) - 1
	for cut >= 0 && !os.IsPathSeparator(rest[cut]) {
		cut--
	}
	name := rest[cut+1:]
	if cut < 0 || name == "" || name == "." || name == ".." {
		if resolved, err := filepath.EvalSymlinks(raw); err == nil {
			return resolved
		}
		return filepath.Clean(raw)
	}
	folder := volume + rest[:cut]
	if rest[:cut] == "" {
		folder = volume + rest[:1]
	}
	if resolved, err := filepath.EvalSymlinks(folder); err == nil {
		return filepath.Join(resolved, name)
	}
	return filepath.Clean(raw)
}

// gitAnotherWorkTreesDotGit is why folder, a git folder or a place on the way to top's (gitDotGitHops), is another work
// tree's .git, "" when it isn't: it is named .git (in any spelling), its parent isn't top, and that parent is a work
// tree of the repo whose shared git folder is common (gitWorkTreeOf, FXSWEEP3P3R3 N4). A .git-named folder whose parent
// isn't such a work tree is judged as a folder of any other name.
func gitAnotherWorkTreesDotGit(ctx context.Context, top, folder, common string) string {
	parent := filepath.Dir(folder)
	if !namedDotGit(folder) || oneFolderOnDisk(parent, top) || !gitWorkTreeOf(ctx, parent, common) {
		return ""
	}
	return "it is the .git of another work tree, " + physical(parent)
}

// gitWorkTreeOf is whether folder is a work tree of the repo whose shared git folder is common (FXSWEEP3P3R3 N4): git
// run there takes folder as its top, with common as its shared git folder, and folder holds something besides its .git.
// A folder that holds its .git alone (a git folder kept elsewhere under the name .git, as `clone --separate-git-dir
// <store>/x/.git` makes one) isn't one: git would take it as a work tree whose every file is deleted, and nothing on
// the disk tells the two apart. True when git couldn't be asked: a look that can't tell refuses.
func gitWorkTreeOf(ctx context.Context, folder, common string) bool {
	out, status, err := readGitOut(ctx, folder, gitCall{global: gitReadArgs, config: gitHere().owner}, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	if err != nil {
		return true
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if status != 0 || len(lines) != 2 || !oneFolderOnDisk(filepath.FromSlash(lines[0]), folder) || !oneFolderOnDisk(filepath.FromSlash(lines[1]), common) {
		return false
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), ".git") {
			return true
		}
	}
	return false
}

// namedDotGit is whether gitDir is a folder named .git: by its name in any case, or as the very .git of its parent.
func namedDotGit(gitDir string) bool {
	if strings.EqualFold(filepath.Base(gitDir), ".git") {
		return true
	}
	return oneFolderOnDisk(filepath.Join(filepath.Dir(gitDir), ".git"), gitDir)
}

// oneFolderOnDisk is whether a and b are one folder on the disk, in whatever spelling each is given.
func oneFolderOnDisk(a, b string) bool {
	one, err1 := os.Stat(a)
	two, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && one.IsDir() && os.SameFile(one, two)
}

// inTop is whether folder is top or inside it, compared physically (FXSWEEP3P3R2 B1): folder with its links
// resolved, then it or one of its parents is top's very folder. That takes every spelling the disk takes for top (a
// Mac's case and Unicode folds, as the Policy folds them, Windows' case, short names and junctions) and only those:
// a name that merely folds to top's is another folder on a volume that tells case apart.
func inTop(top, folder string) bool {
	folder = physical(folder)
	if top == "" || folder == "" {
		return false
	}
	for at := folder; ; at = filepath.Dir(at) {
		if oneFolderOnDisk(at, top) {
			return true
		}
		if filepath.Dir(at) == at {
			return false
		}
	}
}

// gitdirLine is the folder a file of git's names on its first line, after prefix ("gitdir: " in a .git file, nothing
// in a registration's gitdir), made absolute from base when git wrote it relative (worktree.useRelativePaths); false
// when the file names none, or isn't a plain file.
func gitdirLine(base, path, prefix string) (string, bool) {
	named, found := gitdirNamed(base, path, prefix)
	if !found {
		return "", false
	}
	return filepath.Clean(named), true
}

// gitdirNamed is gitdirLine's folder as the file gives it: joined to base when relative, never cleaned, so a ".." in
// it is left for the disk to resolve (gitHopPlace).
func gitdirNamed(base, path, prefix string) (string, bool) {
	data, err := readHireFile(base, path, 4096)
	if err != nil {
		return "", false
	}
	line, _, _ := strings.Cut(string(data), "\n")
	named, found := strings.CutPrefix(strings.TrimRight(line, "\r"), prefix)
	named = filepath.FromSlash(strings.TrimSpace(named))
	if !found || named == "" {
		return "", false
	}
	if !filepath.IsAbs(named) {
		named = base + string(filepath.Separator) + named
	}
	return named, true
}

// gitWriteThroughFolder runs args, which write (gitWritesThrough), on the repo dir is in, through a git folder of the
// core's own, and carries the one branch they changed into the repo; for a new worktree (made from the repo's HEAD, or
// from the commit named), its registration too. Nothing is carried when git says no. gitDir, top and common name the
// repo when the caller knows them (a desk's tree, by the registry), "" asks git. judge, when not nil, is asked about
// the repo's config before anything is made (ownerRepoJudged).
//
// When git is asked, its answer is looked at once, and that repo is the one written (FXSWEEP3P3R3 B3): the common git
// folder the checks judged is opened then as the root the carry writes through, and must be that very folder; every
// git after the look names that git folder and top (GIT_DIR, GIT_WORK_TREE), never finding the repo from dir again.
func gitWriteThroughFolder(ctx context.Context, dir, gitDir, top, common string, judge func([]byte) error, args []string) (int, string, error) {
	tree := ""
	if args[0] == "worktree" {
		var ok bool
		if tree, ok = gitWorktreeAdded(args); !ok {
			return 0, "", errors.New("a worktree is made only as `worktree add [-q] -b <branch> <path>`")
		}
	}
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return 0, "", errors.New("the git on this computer is missing, or older than 2.32")
	}
	look := gitCall{global: gitReadArgs, config: here.owner}
	var judged os.FileInfo
	if gitDir == "" {
		out, status, err := readGitOut(ctx, dir, look, "rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir")
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if err != nil || status != 0 || len(lines) != 3 {
			return 0, "", errors.Join(err, errors.New("it isn't a repo git can write here"))
		}
		top, gitDir, common = filepath.FromSlash(lines[0]), filepath.FromSlash(lines[1]), filepath.FromSlash(lines[2])
		// The folder asked about must be in the top git gave: a git folder's core.worktree moves the top wherever it
		// names, into another repo (FXSWEEP3P3R2 B1). Asked again here, so a .git swapped in since the caller's own look
		// is caught too. Each refusal is the owner's to hear, in words naming the folder (ownerGitNo).
		if !inTop(top, dir) {
			return 0, "", ownerGitNo(fmt.Sprintf("git takes %s as the top of %s, a folder that isn't in it, so its .git leads into another repo.", top, dir))
		}
		// The shared git folder must be the repo's own: a git folder that isn't it is one of its registrations, in its
		// worktrees folder. One anywhere else (a .git file naming a folder whose commondir names another repo's .git)
		// would have the core write a branch and a worktree into a repo nobody hired into (FXSWEEP3P3 F-8).
		if physical(gitDir) != physical(common) && physical(filepath.Dir(gitDir)) != physical(filepath.Join(common, "worktrees")) {
			return 0, "", ownerGitNo(fmt.Sprintf("the git folder of %s, %s, names a shared one that isn't its own (commondir).", top, gitDir))
		}
		// The folder the checks judge, by its identity: the root the carry writes through must be it (B3).
		physicalCommon := physical(common)
		if judged, err = os.Stat(physicalCommon); err != nil || !judged.IsDir() {
			return 0, "", errors.Join(err, errors.New("its shared git folder isn't a folder"))
		}
		if why := gitFolderLeadsBack(ctx, top, gitDir, common); why != "" {
			return 0, "", ownerGitNo(fmt.Sprintf("its git folder, %s, isn't %s's own: %s.", gitDir, top, why))
		}
		gitDir, common = physical(gitDir), physicalCommon
	}
	look.env = []string{"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + top}
	root, err := os.OpenRoot(common)
	if err != nil {
		return 0, "", err
	}
	defer root.Close()
	rooted, err := root.Stat(".")
	if err != nil {
		return 0, "", err
	}
	if judged != nil && !os.SameFile(rooted, judged) {
		return 0, "", ownerGitNo(fmt.Sprintf("the shared git folder of %s, %s, was swapped for another as it was looked at.", top, common))
	}
	// Its config is read through the root, never by git finding it by the path (FXSWEEP3P3R4 N2): a git folder is
	// the shared one, or one of its registrations.
	registration := "."
	if !oneFolderOnDisk(gitDir, common) {
		registration = path.Join("worktrees", filepath.Base(gitDir))
	}
	list, err := gitConfigListThrough(ctx, root, registration, look)
	if err != nil {
		return 0, "", errors.Join(err, errors.New("its config couldn't be read"))
	}
	if judge != nil {
		if err := judge(list); err != nil {
			return 0, "", err
		}
	}
	if storage := gitConfigValue(list, "extensions.refstorage"); storage != "" && !strings.EqualFold(storage, "files") {
		return 0, "", errors.New("its branches aren't kept as files (" + storage + "), and the core writes them only as files")
	}
	// Only settings a read knows are data are handed over: a key it doesn't know is left out, never read.
	folder, err := makeGitReadFolder(ctx, dir, top, gitDir, readGitOut, look, gitDataOnly(list))
	if err != nil {
		return 0, "", err
	}
	defer folder.remove()
	call := folder.call
	extra := ""
	if tree != "" {
		// Made from the core's folder as from a bare repo: git names the new worktree's own git folder and work tree
		// for the gits it starts, and no index but the new worktree's is written.
		call.env = slices.DeleteFunc(slices.Clone(call.env), func(kv string) bool {
			return strings.HasPrefix(kv, "GIT_WORK_TREE=") || strings.HasPrefix(kv, "GIT_INDEX_FILE=")
		})
		extra = "[core]\n\tbare = true\n"
	}
	if args[0] == "commit" {
		// No global config is read here: a commit is made by the repo's own name and address, else the owner's, as
		// their own git would make it (never one git makes up from the computer's name).
		name, email := ownerGitIdentity(ctx)
		for _, key := range [][2]string{{"name", name}, {"email", email}} {
			if key[1] != "" && gitConfigValue(list, "user."+key[0]) == "" {
				extra += "[user]\n\t" + key[0] + " = " + gitConfigQuoted(key[1]) + "\n"
			}
		}
	}
	if extra != "" {
		file, err := os.OpenFile(filepath.Join(folder.dir, "config"), os.O_WRONLY|os.O_APPEND, 0)
		if err == nil {
			_, err = file.WriteString(extra)
			err = errors.Join(err, file.Close())
		}
		if err != nil {
			return 0, "", err
		}
	}
	before, err := gitBranchesIn(ctx, dir, call)
	if err != nil {
		return 0, "", err
	}
	run, stage := args, ""
	if tree != "" && worktreeCheckedOutInCore {
		// git checks a new worktree out by its path, and the folder for hires it is made in is one whoever owns the
		// repo can write, and so swap the tree for a link while git writes (FXGMHIRENOGIT126R2, B1). So git makes it
		// in a folder of the core's own, under the same name, and the core carries it into the real tree through a
		// root (carryWorktree): git never writes in the tree by its path.
		staging, err := os.MkdirTemp(gitReadsFolder(), "tree-")
		if err != nil {
			return 0, "", err
		}
		defer os.RemoveAll(staging)
		stage = filepath.Join(staging, filepath.Base(tree))
		run = slices.Clone(args)
		run[slices.Index(run, tree)] = stage
	}
	status, said, err := gitRun(ctx, dir, call, run)
	if err != nil || status != 0 {
		return status, said, err
	}
	after, err := gitBranchesIn(ctx, dir, call)
	if err != nil {
		return 0, "", err
	}
	var changed []string
	for name := range mergedKeys(before, after) {
		if before[name] != after[name] {
			changed = append(changed, name)
		}
	}
	if len(changed) > 1 {
		return 0, "", errors.New("git changed more than one branch, and the core carries one")
	}
	// What is carried names the repo's git folder by its path (a worktree's .git file), so that path must still lead to
	// the root it is written through.
	if now, err := os.Stat(root.Name()); err != nil || !os.SameFile(now, rooted) {
		return 0, "", fmt.Errorf("the shared git folder of %s, %s, was moved while git ran, so nothing was carried into it", top, root.Name())
	}
	if tree != "" {
		if len(changed) != 1 || before[changed[0]] != "" {
			return 0, "", errors.New("git made no new branch for the worktree")
		}
		name, treeRoot, carried, err := carryWorktree(root, folder.dir, tree, stage, folder.format, changed[0], after[changed[0]])
		if err != nil {
			if stage != "" {
				return 0, "", gitCarryNo("Smolcorp didn't put it at " + shownPath(tree) + ": " + strings.ReplaceAll(err.Error(), "\n", "; ") + ". Nothing was left there.")
			}
			return 0, "", err
		}
		if err := rememberCarried(root, changed[0], name, treeRoot, carried); err != nil {
			return 0, "", err
		}
		return status, said, nil
	}
	for _, name := range changed {
		if after[name] == "" && gitBranchCheckedOut(root, name) {
			return 0, "", fmt.Errorf("the branch %s is checked out, and git deletes no branch a worktree is on", name)
		}
		if err := carryBranch(root, name, before[name], after[name]); err != nil {
			return 0, "", err
		}
		if args[0] == "branch" && after[name] == "" && name == "refs/heads/"+args[2] && strings.HasPrefix(args[2], "hire/") {
			gitDropUpstream(ctx, root, args[2])
		}
	}
	return status, said, nil
}

// gitDropUpstream removes a hire's deleted branch's upstream (presetUpstream's two keys) from the config of the repo
// whose shared git folder is root, through root (FXSWEEP3P3R4 N3), never in a repo git would find from a folder again.
// git's own `branch -D` drops the branch's section with it; the deletion the core carried left it in the repo's
// config. Only a hire's branch, as every caller's is, and only the two keys presetUpstream wrote, each by its whole
// name: a section's removal (git's own -D too) matches the section's name only up to its first "]", so it would take
// another branch's named this one and a "]" after it (RT126SAX7STACK F2, F4). None there is fine, and a config that
// can't be read or written is left as it is, as before.
func gitDropUpstream(ctx context.Context, root *os.Root, branch string) {
	var edits [][]string
	for _, key := range []string{"remote", "merge"} {
		edits = append(edits, []string{"--unset-all", "branch." + branch + "." + key})
	}
	if err := gitConfigThroughRoot(ctx, root, "config", edits); err != nil {
		log.Printf("smolcorp: the upstream of the deleted branch %s was left in the repo's config: %v", branch, err)
	}
}

func mergedKeys(a, b map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

// gitBranchesIn is each branch the git folder call names has, and its commit. The listing grows with the repo's
// branches, so it is read line by line as it comes (gitRecords, FXWINWORKTREER4), up to gitRefsMost bytes, where
// gitOutput's cap would refuse a repo of some 45,000 branches every write through the core's folder.
func gitBranchesIn(ctx context.Context, dir string, call gitCall) (map[string]string, error) {
	branches := map[string]string{}
	lines := &gitRecords{sep: '\n', most: gitRefsMost, each: func(line []byte) error {
		if id, name, found := strings.Cut(string(line), " "); found {
			branches[name] = id
		}
		return nil
	}}
	call.stdout = lines
	_, status, err := readGitOut(ctx, dir, call, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/")
	if err := lines.end(); err != nil {
		return nil, errors.Join(err, errors.New("git couldn't list the branches"))
	}
	if err != nil || status != 0 {
		return nil, errors.Join(err, errors.New("git couldn't list the branches"))
	}
	return branches, nil
}

// carryWorktree carries a worktree git made at tree, on branch (at id), from the core's folder made into the repo
// whose common git folder is root: the branch, then the registration (copied, under the name git gave it, which must
// be free), then the worktree's .git file, naming it. The registration's gitdir and the .git file are written as the
// repo's own git would write them there (gitWorktreeLinks), never as git wrote them in the core's folder. A step that
// fails undoes the ones before. The tree is opened as a root for the .git file's write, and that root is kept, for
// whatever the hire's flow writes in the tree after (its submodules, FXGMHIRENOGIT126R1's B6 fix): the folder git made,
// never a link put in its place. With a stage, the tree's name as the carry made it is given too (its folder resolved,
// as git's links name it), for the hire's row (FXGMHIRENOGIT126R3, B2); "" without one.
//
// When git checked the worktree out at stage, a folder of the core's own (worktreeCheckedOutInCore,
// FXGMHIRENOGIT126R2, B1), the core makes the tree's folder itself, in the folder for hires opened as a root, opens it
// as a root, and carries the checked-out files in through it (carryWorkTree), each with its times; the registration's
// index then gets the carried files' own stat keys (refreshCarriedIndex), so git finds them as it left them. Nothing
// is written in the tree by its path. format is the repo's object format.
func carryWorktree(root *os.Root, made, tree, stage, format, branch, id string) (string, *os.Root, string, error) {
	entries, err := os.ReadDir(filepath.Join(made, "worktrees"))
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return "", nil, "", errors.Join(err, errors.New("git made no worktree registration the core can carry"))
	}
	name := entries[0].Name()
	var parent *os.Root
	if stage != "" {
		// The folders above the tree are made when they aren't there, as git makes them for a worktree it adds; the
		// tree's own folder is then made, and everything in it written, through the last one opened as a root.
		_ = os.MkdirAll(filepath.Dir(tree), hireTreesMode(filepath.Dir(tree)))
		if parent, err = openFolderRoot(filepath.Dir(tree)); err != nil {
			return "", nil, "", errors.Join(err, errors.New("the folder for the hire's worktree isn't a folder the core can open"))
		}
		defer parent.Close()
	}
	realTree := "" // the tree as git's links name it: its folder resolved
	if parent != nil {
		realTree = filepath.Join(physical(parent.Name()), filepath.Base(tree))
	}
	gitdir, dotGit, err := gitWorktreeLinks(made, name, filepath.Join(root.Name(), "worktrees", name), stage, realTree)
	if err != nil {
		return "", nil, "", err
	}
	if err := carryBranch(root, branch, "", id); err != nil {
		return "", nil, "", err
	}
	registration := path.Join("worktrees", name)
	if err := root.MkdirAll("worktrees", 0o755); err != nil {
		_ = carryBranch(root, branch, id, "")
		return "", nil, "", err
	}
	if err := root.Mkdir(registration, 0o755); err != nil {
		_ = carryBranch(root, branch, id, "")
		return "", nil, "", fmt.Errorf("a worktree named %s is registered already: %w", name, err)
	}
	var treeRoot *os.Root
	madeTree := false
	undo := func() {
		if treeRoot != nil && parent != nil {
			// Through the tree's root only: what is in it is what the carry put there.
			if entries, err := readDirIn(treeRoot, "."); err == nil {
				for _, e := range entries {
					_ = treeRoot.RemoveAll(e.Name())
				}
			}
		}
		if treeRoot != nil {
			treeRoot.Close()
		}
		if madeTree {
			_ = parent.Remove(filepath.Base(tree)) // only while it's an empty folder
		}
		_ = root.RemoveAll(registration)
		_ = carryBranch(root, branch, id, "")
	}
	var seen map[string]os.FileInfo
	if parent != nil {
		if treeRoot, madeTree, err = makeTreeFolder(parent, filepath.Base(tree)); err == nil {
			seen = map[string]os.FileInfo{}
			err = carryWorkTreeSeen(treeRoot, stage, ".", seen)
		}
		if err != nil {
			undo()
			return "", nil, "", err
		}
	}
	from := filepath.Join(made, "worktrees", name)
	err = filepath.WalkDir(from, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || at == from {
			return err
		}
		rel, err := filepath.Rel(from, at)
		if err != nil {
			return err
		}
		to := path.Join(registration, filepath.ToSlash(rel))
		switch {
		case entry.IsDir():
			return root.Mkdir(to, 0o755)
		case entry.Type().IsRegular():
			data, err := cpReadData(at, gitCopiesMost)
			if err != nil {
				return err
			}
			if rel == "index" && seen != nil {
				data = refreshCarriedIndex(data, seen, gitHashSize(format))
			}
			return root.WriteFile(to, data, 0o644)
		}
		return nil // git makes no other kind of file here
	})
	if err == nil {
		err = root.WriteFile(path.Join(registration, "gitdir"), gitdir, 0o644)
	}
	if err == nil && treeRoot == nil {
		treeRoot, err = openFolderRoot(tree)
	}
	if err == nil {
		var at *os.File
		if at, err = treeRoot.OpenFile(".git", os.O_WRONLY|os.O_CREATE|os.O_TRUNC|excludeIf(parent != nil), 0o644); err == nil {
			_, err = at.Write(dotGit)
			err = errors.Join(err, at.Close())
		}
	}
	if err != nil {
		undo()
		return "", nil, "", err
	}
	return name, treeRoot, realTree, nil
}

// excludeIf is O_EXCL when exclusive: a .git the core writes in a tree folder it made itself is new.
func excludeIf(exclusive bool) int {
	if exclusive {
		return os.O_EXCL
	}
	return 0
}

// makeTreeFolder makes the worktree's folder name in parent, the folder for hires opened as a root, and opens it as a
// root: a new folder, or an empty one already there (as git takes one), never a link, and the very folder parent
// holds under that name as it is opened. made is whether it was made here.
func makeTreeFolder(parent *os.Root, name string) (tree *os.Root, made bool, err error) {
	if err = parent.Mkdir(name, 0o755); err == nil {
		made = true
	} else if !errors.Is(err, fs.ErrExist) {
		return nil, false, err
	}
	seen, err := parent.Lstat(name)
	if err != nil || !seen.IsDir() {
		return nil, false, errors.Join(err, errors.New("the hire's worktree folder isn't a folder"))
	}
	if tree, err = parent.OpenRoot(name); err != nil {
		return nil, false, err
	}
	if opened, err := tree.Stat("."); err != nil || !os.SameFile(seen, opened) {
		tree.Close()
		return nil, false, errors.Join(err, errors.New("the hire's worktree folder was moved while it was opened"))
	}
	if !made && !emptyFolderIn(tree, ".") {
		tree.Close()
		return nil, false, errors.New("the hire's worktree folder is there already, and isn't empty")
	}
	return tree, made, nil
}

// openFolderRoot opens folder as a root, refusing a link at its name, or a folder swapped in between the look and the
// open: the root is the folder Lstat saw there.
func openFolderRoot(folder string) (*os.Root, error) {
	seen, err := os.Lstat(folder)
	if err != nil {
		return nil, err
	}
	if !seen.IsDir() {
		return nil, errors.New(folder + " isn't a folder")
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil, err
	}
	if opened, err := root.Stat("."); err != nil || !os.SameFile(seen, opened) {
		root.Close()
		return nil, errors.Join(err, errors.New(folder+" was moved while it was opened"))
	}
	return root, nil
}

// hireCarried is where the core carried a hire's new worktree (gitWriteThroughFolder): the repo's shared git folder,
// by the path and the identity of the root it was written through, and the registration it made there, by its name
// and identity (FXSWEEP3P3R3 B3). What the hire's flow writes after the carry goes there and nowhere else, never into
// a repo found again from a folder: the branch's upstream (gitPresetIntoCarried) and, on a Mac, its submodules' git
// folders (initHireSubmodules). Kept by the branch's short name (hire/…) until prepareHire is done (forgetCarried), or,
// for a caller that asks (hireRequest.KeepCarried), until its hire ends: its undos go through it (FXGMHIRENOGIT126R5).
//
// Both folders are kept open as roots, the ones the carry wrote (FXSWEEP3P3R4 B4, B5): every later write goes through
// them, never by a path that crosses the repo's folders, which whoever owns the repo can rename or link elsewhere
// while a write waits. So is the worktree's own folder, tree, the one the carry wrote its .git file in
// (FXGMHIRENOGIT126R1, round 4's B6): whoever can write the folder for hires can swap the tree's path for a link.
//
// treeName is the tree's name as the carry made it, its folder resolved (carryWorktree), "" when git checked it out
// where it is: what the hire's row keeps, once prepareHire found it still names that folder (carriedTreeAt).
type hireCarried struct {
	common, registration     string
	treeName                 string
	commonInfo, registryInfo os.FileInfo
	root, registry, tree     *os.Root
}

var carriedHires = struct {
	sync.Mutex
	by map[string]hireCarried
}{by: map[string]hireCarried{}}

// rememberCarried keeps where the core carried branch's worktree, registered under name in root: root itself and the
// registration, each opened again through root (never by its path), and the worktree's folder as the carry opened it
// (tree, which the record now holds, and closes on a failure here), for as long as the record is kept.
func rememberCarried(root *os.Root, branch, name string, tree *os.Root, treeName string) error {
	common, err := root.OpenRoot(".")
	if err != nil {
		tree.Close()
		return err
	}
	registry, err := root.OpenRoot(path.Join("worktrees", name))
	if err != nil {
		common.Close()
		tree.Close()
		return err
	}
	commonInfo, err1 := common.Stat(".")
	registryInfo, err2 := registry.Stat(".")
	if err := errors.Join(err1, err2); err != nil {
		common.Close()
		registry.Close()
		tree.Close()
		return err
	}
	carriedHires.Lock()
	defer carriedHires.Unlock()
	short := strings.TrimPrefix(branch, "refs/heads/")
	carriedHires.by[short].close()
	carriedHires.by[short] = hireCarried{common: root.Name(), registration: name, treeName: treeName, commonInfo: commonInfo, registryInfo: registryInfo, root: common, registry: registry, tree: tree}
	return nil
}

// carriedFor is where the core carried branch's worktree (its short name), when it did and both folders are still
// there by their paths: the very ones it wrote. What it holds open stays the record's (forgetCarried closes it).
func carriedFor(branch string) (hireCarried, bool) {
	carriedHires.Lock()
	c, found := carriedHires.by[branch]
	carriedHires.Unlock()
	if !found {
		return hireCarried{}, false
	}
	common, err1 := os.Stat(c.common)
	registration, err2 := os.Stat(c.registrationPath())
	return c, err1 == nil && err2 == nil && os.SameFile(common, c.commonInfo) && os.SameFile(registration, c.registryInfo)
}

// registrationPath is the carried registration's path.
func (c hireCarried) registrationPath() string {
	return filepath.Join(c.common, "worktrees", c.registration)
}

// close closes what the record holds open.
func (c hireCarried) close() {
	if c.root != nil {
		c.root.Close()
	}
	if c.registry != nil {
		c.registry.Close()
	}
	if c.tree != nil {
		c.tree.Close()
	}
}

// carriedTreeAt is, for branch's worktree the core carried into a tree folder it made (a stage: on a Mac), that tree's
// name as the carry made it, and whether path (the tree as prepareHire named it) and that name both still are, by
// Lstat, the very folder the carry wrote through (FXGMHIRENOGIT126R3, B2): no link put at either since, nor another
// folder. carried is false for a branch with no such record (git checked the tree out where it is, or a stand-in git
// ran): the caller goes on as before.
func carriedTreeAt(branch, path string) (name string, carried, same bool) {
	carriedHires.Lock()
	c, found := carriedHires.by[branch]
	carriedHires.Unlock()
	if !found || c.tree == nil || c.treeName == "" {
		return "", false, false
	}
	opened, err := c.tree.Stat(".")
	if err != nil {
		return c.treeName, true, false
	}
	for _, at := range []string{path, c.treeName} {
		if seen, err := os.Lstat(at); err != nil || !seen.IsDir() || !os.SameFile(seen, opened) {
			return c.treeName, true, false
		}
	}
	return c.treeName, true, true
}

// discardCarriedTree takes back what the core carried for branch's worktree, through the roots it kept and nothing
// else (FXGMHIRENOGIT126R3, B2): everything in the tree's folder, then its registration in the repo's shared git
// folder. The tree's folder itself is left, empty, wherever it is now: its name may lead elsewhere. The branch is
// the caller's to delete. It says whether there was a record to take back.
func discardCarriedTree(branch string) bool {
	carriedHires.Lock()
	c, found := carriedHires.by[branch]
	carriedHires.Unlock()
	if !found || c.tree == nil || c.root == nil {
		return false
	}
	if entries, err := readDirIn(c.tree, "."); err == nil {
		for _, e := range entries {
			_ = c.tree.RemoveAll(e.Name())
		}
	}
	_ = c.root.RemoveAll(path.Join("worktrees", c.registration))
	return true
}

// removeCarriedTreeFolder removes branch's tree folder itself, once discardCarriedTree emptied it, through the folder
// it is in (no link at that one's name), and only while its name there is still, by Lstat, the very folder the carry
// made (FXGMHIRENOGIT126R4, R12). A name turned since is left as it is: it may lead elsewhere. Off a Mac, git checked
// the tree out where it is, and the carry kept no name for it (carryWorktree): the tree is then worktree, the path the
// caller made it at, and goes the same way, only while that name is still the folder the carry opened (FXCI53).
func removeCarriedTreeFolder(branch, worktree string) {
	carriedHires.Lock()
	c, found := carriedHires.by[branch]
	carriedHires.Unlock()
	at := c.treeName
	if at == "" {
		at = worktree
	}
	if !found || c.tree == nil || at == "" {
		return
	}
	opened, err := c.tree.Stat(".")
	if err != nil {
		return
	}
	parent, err := openFolderRoot(filepath.Dir(at))
	if err != nil {
		return
	}
	defer parent.Close()
	name := filepath.Base(at)
	if seen, err := parent.Lstat(name); err == nil && seen.IsDir() && os.SameFile(seen, opened) {
		_ = parent.Remove(name)
	}
}

// carriedTreeRoot opens sub, a folder inside branch's worktree ("." for the tree itself), through the tree's root the
// carry kept (FXGMHIRENOGIT126R5): the very folder carried, whatever its path leads to now. found is false for a branch
// with no such record (git checked the tree out where it is, or a stand-in git ran).
func carriedTreeRoot(branch, sub string) (root *os.Root, found bool, err error) {
	carriedHires.Lock()
	defer carriedHires.Unlock()
	c, found := carriedHires.by[branch]
	if !found || c.tree == nil {
		return nil, false, nil
	}
	root, err = c.tree.OpenRoot(sub)
	return root, true, err
}

// forgetCarried drops branch's record, once its hire's place is made or refused, or its caller's hire ended.
func forgetCarried(branch string) {
	carriedHires.Lock()
	defer carriedHires.Unlock()
	carriedHires.by[branch].close()
	delete(carriedHires.by, branch)
}

// gitPresetIntoCarried writes presetUpstream's key (args, gitPresetsUpstream) into the config of the repo the core
// carried the hire's worktree into, and only there (FXSWEEP3P3R3 B3): never the repo git would find from the folder now,
// which a .git rewritten since the carry names. Its shared git folder must still be the very folder the carry wrote,
// and the key is written through the root the carry kept (gitConfigThroughRoot, FXSWEEP3P3R4 B4): git edits a copy in
// a folder of the core's own and finds nothing in the repo. A branch the core carried no worktree for gets no key.
func gitPresetIntoCarried(ctx context.Context, args []string) (int, string, error) {
	branch, found := strings.CutSuffix(strings.TrimPrefix(args[2], "branch."), ".remote")
	if !found {
		branch = strings.TrimSuffix(branch, ".merge")
	}
	c, found := carriedFor(branch)
	if !found {
		return 0, "", errors.New("the core carried no worktree for " + branch + " into a repo it still finds")
	}
	if err := gitConfigThroughRoot(ctx, c.root, "config", [][]string{{args[2], args[3]}}); err != nil {
		return 0, "", err
	}
	return 0, "", nil
}

// gitConfigLock is how a config file's lock is opened in a git folder: made new, as git makes it.
const gitConfigLock = os.O_WRONLY | os.O_CREATE | os.O_EXCL

// gitConfigThroughRoot makes edits (each the arguments after `git config --file <file>`: a key and its value, or
// --unset-all and a key) to the config file name in the git folder root, as git edits it, and through root only
// (FXSWEEP3P3R4 B4):
//   - its lock (name.lock) is made in root first, as git's own, so no other git changes it meanwhile;
//   - the file is opened in root without waiting (a fifo can't hold the core) and read only as a regular file;
//   - git edits a copy of it in a folder of the core's own (`config --file <copy>`, GIT_DIR there), finding nothing
//     in the repo; an --unset-all of a key that isn't there (git's 5) changes nothing;
//   - what git made is written into the lock, which is renamed onto the file inside root, keeping the file's mode.
//
// Nothing is written when nothing changed.
func gitConfigThroughRoot(ctx context.Context, root *os.Root, name string, edits [][]string) error {
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return errors.New("the git on this computer is missing, or older than 2.32")
	}
	lock := name + ".lock"
	held, err := root.OpenFile(lock, gitConfigLock, 0o644)
	if err != nil {
		return fmt.Errorf("its %s is being changed: %w", name, err)
	}
	locked := true
	defer func() {
		if locked {
			held.Close()
			_ = root.Remove(lock)
		}
	}()
	data, mode, err := gitRootFile(root, name)
	if err != nil {
		return err
	}
	if err := sweepGitReads(); err != nil {
		return err
	}
	made, err := os.MkdirTemp(gitReadsFolder(), "config-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(made)
	copied := filepath.Join(made, "edited")
	if err := os.WriteFile(copied, data, 0o600); err != nil {
		return err
	}
	call := gitCall{global: gitReadArgs, config: here.owner, env: []string{"GIT_DIR=" + made}}
	for _, edit := range edits {
		status, said, err := gitRun(ctx, made, call, append([]string{"config", "--file", copied}, edit...))
		switch {
		case err != nil:
			return err
		case status == 5 && edit[0] == "--unset-all":
		case status != 0:
			return fmt.Errorf("git config: %s", firstRunes(said, 200))
		}
	}
	edited, err := os.ReadFile(copied)
	if err != nil {
		return err
	}
	if bytes.Equal(edited, data) {
		return nil
	}
	_, err = held.Write(edited)
	if err = errors.Join(err, held.Close()); err == nil {
		err = root.Chmod(lock, mode)
	}
	if err == nil {
		err = root.Rename(lock, name)
	}
	if err != nil {
		_ = root.Remove(lock)
		locked = false
		return err
	}
	locked = false
	return nil
}

// gitRootFile is the file name in the git folder root, opened there without waiting and read only as a regular file
// (gitRootOpen), up to gitCopiesMost, and its mode: none there reads as empty, with git's own mode for a new one.
func gitRootFile(root *os.Root, name string) ([]byte, fs.FileMode, error) {
	file, err := gitRootOpen(root, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	data, err := cpReadOpen(file, gitCopiesMost)
	if err != nil {
		return nil, 0, errors.Join(err, fmt.Errorf("the repo's %s is bigger than the core reads", name))
	}
	return data, info.Mode().Perm(), nil
}

// gitConfigListThrough is `config --list -z` (as call lists it) of the repo whose shared git folder is root, and
// whose git folder is registration in it ("." for the shared one itself), read through root (FXSWEEP3P3R4 N2): its
// config, and the git folder's config.worktree, each opened without waiting and only as a regular file (gitRootFile),
// copied into a folder of the core's own that git is pointed at (GIT_DIR). git finds nothing in the repo itself.
//
// git lists an include's own key, as before; what a relative include names isn't there in the copy, so it isn't
// listed (a hire's write refuses any include, ownerRepoJudged; a branch's deletion reads no setting).
func gitConfigListThrough(ctx context.Context, root *os.Root, registration string, call gitCall) ([]byte, error) {
	config, _, err := gitRootFile(root, "config")
	if err != nil {
		return nil, err
	}
	worktree, _, err := gitRootFile(root, path.Join(registration, "config.worktree"))
	if err != nil {
		return nil, err
	}
	if err := sweepGitReads(); err != nil {
		return nil, err
	}
	made, err := os.MkdirTemp(gitReadsFolder(), "config-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(made)
	files := map[string][]byte{"HEAD": []byte("ref: refs/heads/main\n"), "config": config}
	if worktree != nil {
		files["config.worktree"] = worktree
	}
	for _, folder := range []string{"objects", "refs"} {
		if err == nil {
			err = os.Mkdir(filepath.Join(made, folder), 0o700)
		}
	}
	for name, data := range files {
		if err == nil {
			err = os.WriteFile(filepath.Join(made, name), data, 0o600)
		}
	}
	if err != nil {
		return nil, err
	}
	// A work tree of the copy's own too: a core.worktree in the repo's config (a submodule's, relative to its git
	// folder) would have git look for it from the copy, and stop.
	call.env = []string{"GIT_DIR=" + made, "GIT_WORK_TREE=" + made}
	list, status, err := readGitOut(ctx, made, call, "config", "--list", "-z")
	if err != nil || status != 0 {
		return nil, errors.Join(err, fmt.Errorf("git config ended with %d", status))
	}
	return list, nil
}

// gitWorktreeLinks are the two files that tie a worktree git made from the core's folder made (registration name) to
// its registration carried to reg in the repo: the registration's gitdir, naming the tree's .git, and the tree's .git
// file, naming the registration. git made them for the core's folder; here they are as the repo's own git writes them
// (FXRELPATHS, SIERRA's ruling on RT126GMSCOPEPINR2 F2): between the real paths, relative to each other when git
// was asked for relative paths (the repo's worktree.useRelativePaths, handed over with its other data keys), else
// whole. The form is the one git chose, read from the gitdir it wrote: git read the setting itself, so the owner's
// choice is kept as git takes it, and git's real path of the tree is in that file, so nothing in the tree's folder
// is looked up again. reg is as the core names the repo's common folder (git's own path for it).
//
// When git made the worktree at stage, a folder of the core's own (FXGMHIRENOGIT126R2, B1), the gitdir it wrote must
// lead there, and the files name tree instead, the real tree as the core resolved it.
func gitWorktreeLinks(made, name, reg, stage, tree string) (gitdir, dotGit []byte, err error) {
	from := filepath.Join(made, "worktrees", name)
	data, err := cpReadData(filepath.Join(from, "gitdir"), 4096)
	written := strings.TrimSuffix(string(data), "\n")
	if err != nil || written == "" || strings.ContainsAny(written, "\n\r\x00") {
		return nil, nil, errors.Join(err, errors.New("git wrote no worktree place the core can carry"))
	}
	whole := filepath.IsAbs(filepath.FromSlash(written))
	if whole && stage == "" {
		// Whole: git's own gitdir names the tree's real place, wherever the core's folder is; the .git file names
		// the registration where it was carried.
		return []byte(written + "\n"), []byte("gitdir: " + reg + "\n"), nil
	}
	// Relative: git wrote it from the real path of the registration in the core's folder (a folder of the core's
	// own, in the support folder), so it leads to the tree's real .git from there.
	dot := filepath.FromSlash(written)
	if !whole {
		real, err := filepath.EvalSymlinks(from)
		if err != nil {
			return nil, nil, err
		}
		dot = filepath.Clean(filepath.Join(real, dot))
	}
	if filepath.Base(dot) != ".git" {
		return nil, nil, errors.New("git wrote a worktree place that isn't a .git")
	}
	if stage != "" {
		if !oneFolderOnDisk(filepath.Dir(dot), stage) {
			return nil, nil, errors.New("git made the worktree somewhere other than the core's folder")
		}
		if whole {
			return []byte(filepath.ToSlash(filepath.Join(tree, ".git")) + "\n"), []byte("gitdir: " + reg + "\n"), nil
		}
	} else {
		tree = filepath.Dir(dot)
	}
	toTree, errTree := filepath.Rel(reg, tree)
	toReg, errReg := filepath.Rel(tree, reg)
	if errTree != nil || errReg != nil {
		// No relative path between them (two drives on Windows): git writes the whole path then.
		toTree, toReg = tree, reg
	}
	return []byte(filepath.ToSlash(toTree) + "/.git\n"), []byte("gitdir: " + filepath.ToSlash(toReg) + "\n"), nil
}

// carryBranch changes branch name (refs/heads/…) in the repo whose common git folder is root from old to now ("" for
// none: made, or deleted), as git does: under its lock file, as a loose file. The branch must be at old when the lock
// is held (else it moved meanwhile), and a branch git keeps packed isn't deleted (the core doesn't rewrite that file).
func carryBranch(root *os.Root, name, old, now string) error {
	if !gitBranchFile(name) {
		return fmt.Errorf("%q isn't a branch the core writes", name)
	}
	if now != "" {
		if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return err
		}
	}
	lock := name + ".lock"
	file, err := root.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("the branch %s is being changed: %w", name, err)
	}
	held := true
	defer func() {
		if held {
			_ = root.Remove(lock)
		}
	}()
	packed, err := gitPackedBranches(root)
	if err != nil {
		file.Close()
		return err
	}
	current, err := gitLooseBranch(root, name)
	if err != nil {
		file.Close()
		return err
	}
	if current == "" {
		current = packed[name]
	}
	switch {
	case current != old:
		file.Close()
		return fmt.Errorf("the branch %s moved meanwhile", name)
	case now == "" && packed[name] != "":
		file.Close()
		return fmt.Errorf("git keeps the branch %s packed, in a file the core doesn't rewrite", name)
	case now == "":
		file.Close()
		if err := root.Remove(name); err != nil {
			return err
		}
		_ = root.Remove(path.Join("logs", name))
		_ = root.Remove(lock)
		held = false
		for dir := path.Dir(name); dir != "refs/heads" && root.Remove(dir) == nil; dir = path.Dir(dir) {
		}
		return nil
	}
	_, err = file.WriteString(now + "\n")
	if err = errors.Join(err, file.Close()); err == nil {
		err = root.Rename(lock, name)
	}
	if err != nil {
		return err
	}
	held = false
	return nil
}

// gitBranchCheckedOut is whether a worktree of the repo whose common git folder is root is on branch name, by its HEAD:
// the main one's, or a registration's. A HEAD that can't be read counts as on it.
func gitBranchCheckedOut(root *os.Root, name string) bool {
	heads := []string{"HEAD"}
	if folder, err := root.OpenFile("worktrees", cpDataFlags, 0); err == nil { // never waiting on a fifo there
		entries, err := folder.ReadDir(-1)
		folder.Close()
		if err != nil {
			return true
		}
		for _, entry := range entries {
			heads = append(heads, path.Join("worktrees", entry.Name(), "HEAD"))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return true
	}
	for _, head := range heads {
		file, err := gitRootOpen(root, head)
		if errors.Is(err, fs.ErrNotExist) && head != "HEAD" {
			continue
		}
		if err != nil {
			return true
		}
		data, err := io.ReadAll(io.LimitReader(file, 4096))
		file.Close()
		if err != nil || strings.TrimSpace(string(data)) == "ref: "+name {
			return true
		}
	}
	return false
}

// gitRootOpen opens the file name in the git folder root for reading, as every file of the repo's own the core reads
// (FXSWEEP3P3R4): without waiting (a fifo can't hold the core) and only a regular file, judged as opened.
func gitRootOpen(root *os.Root, name string) (*os.File, error) {
	file, err := root.OpenFile(name, cpDataFlags, 0)
	if err != nil {
		return nil, err
	}
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.Join(err, fmt.Errorf("the repo's %s isn't a plain file the core reads", name))
	}
	return file, nil
}

// gitBranchFile is whether name is a branch's file under the common git folder, as git names one: refs/heads/…, no
// part that is empty, dot-led or a lock file.
func gitBranchFile(name string) bool {
	rest, found := strings.CutPrefix(name, "refs/heads/")
	if !found || rest == "" || strings.ContainsAny(name, "\\ ~^:?*[\x00\x7f") {
		return false
	}
	for _, part := range strings.Split(rest, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") || strings.Contains(part, "..") {
			return false
		}
	}
	return true
}

// gitLooseBranch is the commit branch name's own file holds ("" for no file).
func gitLooseBranch(root *os.Root, name string) (string, error) {
	file, err := gitRootOpen(root, name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4096))
	value := strings.TrimSpace(string(data))
	if err != nil || value == "" || strings.HasPrefix(value, "ref:") {
		return "", errors.Join(err, fmt.Errorf("the branch %s isn't a commit git can name", name))
	}
	return value, nil
}

// gitPackedBranches is each branch the repo's packed-refs holds, and its commit.
func gitPackedBranches(root *os.Root) (map[string]string, error) {
	packed := map[string]string{}
	file, err := gitRootOpen(root, "packed-refs")
	if errors.Is(err, fs.ErrNotExist) {
		return packed, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for lines.Scan() {
		line := lines.Text()
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		if id, name, found := strings.Cut(line, " "); found && strings.HasPrefix(name, "refs/heads/") {
			packed[name] = id
		}
	}
	return packed, lines.Err()
}

// gitConfigQuoted is value as a config file holds it: in quotes, with git's escapes, so that nothing in it ends the
// line or the value.
func gitConfigQuoted(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\b", `\b`).Replace(value) + `"`
}

// gitConfigValue is key's last value in list (`config --list -z`), "" for none.
func gitConfigValue(list []byte, key string) string {
	value := ""
	for _, record := range strings.Split(string(list), "\x00") {
		if k, v, _ := strings.Cut(record, "\n"); strings.EqualFold(k, key) {
			value = v
		}
	}
	return value
}

// ownerRepoConfig is the rule of what in the owner's repo's config (list, `config --list -z`) refuses a git that
// reads or writes its files there (SA-X7), "" for none: an include (include.path, includeIf.*.path), else what
// refuses a read (gitRepoConfig: a key no read knows, an address that runs a program). A config-based hook (hook.*)
// doesn't refuse it here: the core's folder leaves it out (gitDataOnly), and FXGITSTATECFGR1 makes a hire on a repo
// that has one. A read is still refused for it, by gitRepoConfig in gitFilesThroughFolder's own look.
func ownerRepoConfig(list []byte) string {
	var judged []string
	for _, record := range strings.Split(string(list), "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		rule, section := gitRuleOf(key)
		if gitIncludeRule(rule) {
			return rule
		}
		if section != "hook" {
			judged = append(judged, record)
		}
	}
	_, refused := gitRepoConfig([]byte(strings.Join(judged, "\x00")))
	return refused
}

// ownerRepoJudged is ownerRepoConfig's answer as the refusal the owner hears, nil when it has none.
func ownerRepoJudged(list []byte) error {
	if refused := ownerRepoConfig(list); refused != "" {
		return ownerGitNo("its git config has " + refused + ", which could run a program, and git there runs with nothing of the kind.")
	}
	return nil
}

// gitIncludeRule is whether rule brings in another file's config.
func gitIncludeRule(rule string) bool { return rule == "include.path" || rule == "includeif.*.path" }

// gitHandOverRefused are keys that are data to a read, but not to the owner's own git, which takes them from a
// worktree's config.worktree when the repo turns worktree config on (RT126SAX7's B1). core.worktree and core.bare say
// where the work tree's top is: a read names its own (GIT_WORK_TREE), the owner's git doesn't, and git runs hooks
// from the top, so a relative hooks folder would be one Take over never looked at. pull.twohead and pull.octopus name
// a merge strategy, and git runs git-merge-<name> for one that isn't its own; so can branch.<name>.mergeOptions, with
// its -s (RT126SAX7R2's F1).
var gitHandOverRefused = gitKeySet(`core.worktree core.bare pull.twohead pull.octopus branch.*.mergeoptions`)

// gitHandOverConfig is the rule of the first setting in list (`config --list -z` of a file a desk could have
// written) that runs a program, or could, for a git that blanks nothing: the owner's own (Take over, C6). "" when
// every setting is data. Unlike a read's (gitRepoConfig), nothing is blanked: a filter, a text converter, an alias,
// gitReadArgs' own keys (an fsmonitor, a hooks folder), an include and gitHandOverRefused are each refused, as is a
// key no read knows and an address that runs a program.
func gitHandOverConfig(list []byte) string {
	for _, record := range strings.Split(string(list), "\x00") {
		key, value, _ := strings.Cut(record, "\n")
		if key == "" {
			continue
		}
		rule, section := gitRuleOf(key)
		switch _, blanked := gitBlankedKeys[rule]; {
		case rule == "":
			return "a key with no section"
		case gitIncludeRule(rule), blanked, gitBlankedSections[section], gitFixedKeys[rule], gitHandOverRefused[rule]:
			return rule
		case gitSettingAddressRuns(key, value):
			return rule + gitRefusedAddress
		case !gitDataKeys[rule] && !gitDataSections[section]:
			return rule
		}
	}
	return ""
}

// presetUpstream gives the hire's new branch an upstream (SANDBOX-PLAN §A3.5.1, AQ2): the remote it pushes to, and
// the same name there. A hire's sandbox denies it the repo's shared config, so its own `git push -u` can't write
// one; with this, a plain `git push` works. The remote is origin, else the one the checkout's branch tracks, else the
// repo's only one; with none, there is nowhere to push and nothing is set. False when the setting couldn't be
// written. The fire's branch deletion removes it with the branch (ownerGitRun).
func presetUpstream(repo, branch string, git gitRunner) bool {
	remote := hirePushRemote(repo, git)
	if remote == "" {
		return true
	}
	for _, setting := range [][2]string{{"remote", remote}, {"merge", "refs/heads/" + branch}} {
		if status, _, ok := git(repo, "config", "--local", "branch."+branch+"."+setting[0], setting[1]); !ok || status != 0 {
			return false
		}
	}
	return true
}

// hirePushRemote is the remote a hire's branch in repo pushes to (presetUpstream), "" for none.
func hirePushRemote(repo string, git gitRunner) string {
	status, out, ok := git(repo, "config", "-z", "--get-regexp", `^remote\..+\.url$`)
	if !ok || status != 0 {
		return "" // 1: no remote
	}
	var names []string
	for _, record := range strings.Split(out, "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		if name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url"); name != "" && name != key {
			names = append(names, name)
		}
	}
	if slices.Contains(names, "origin") {
		return "origin"
	}
	if status, current, ok := git(repo, "symbolic-ref", "--short", "-q", "HEAD"); ok && status == 0 && current != "" {
		if status, tracked, ok := git(repo, "config", "--get", "branch."+current+".remote"); ok && status == 0 && slices.Contains(names, tracked) {
			return tracked
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return ""
}
