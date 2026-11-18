package desktop

import (
	"bufio"
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
	case gitReadsRefsOnly(args), gitPresetsUpstream(args):
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
		// A branch's deletion reads and writes no file: its config isn't judged, as before SA-X7's.
		status, out, err := gitWriteThroughFolder(ctx, dir, "", "", "", nil, args)
		if err == nil && status == 0 && strings.HasPrefix(args[2], "hire/") && gitBranchFile("refs/heads/"+args[2]) {
			// git's own `branch -D` drops the branch's section (a hire's upstream, presetUpstream) with it; the
			// deletion the core carried left it in the repo's config. Only a hire's branch, as every caller's is, and
			// only the two keys presetUpstream wrote, each by its whole name: a section's removal (git's own -D too)
			// matches the section's name only up to its first "]", so it would take another branch's named this one
			// and a "]" after it (RT126SAX7STACK F2, F4). None there (git's 5 for no such key) is fine.
			for _, key := range []string{"remote", "merge"} {
				gitRun(ctx, dir, gitCall{global: gitReadArgs, config: gitHere().owner}, []string{"config", "--local", "--unset-all", "branch." + args[2] + "." + key})
			}
		}
		return status, out, err
	case gitWritesThrough(args):
		return gitWriteThroughFolder(ctx, dir, "", "", "", ownerRepoJudged, args)
	case len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && args[2] == "--force":
		// The worktree of a hire with no row (yet, or any more): removed as the fire removes a hire's own, where git
		// would run a status on it (and follow its .git file) and walk it by path.
		status, out, ok := removeDeskTree(Hire{Name: "a worktree with no row", Repo: physical(dir), Worktree: args[3]})
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

// gitWriteThroughFolder runs args, which write (gitWritesThrough), on the repo dir is in, through a git folder of the
// core's own, and carries the one branch they changed into the repo; for a new worktree (made from the repo's HEAD, or
// from the commit named), its registration too. Nothing is carried when git says no. gitDir, top and common name the
// repo when the caller knows them (a desk's tree, by the registry), "" asks git. judge, when not nil, is asked about
// the repo's config before anything is made (ownerRepoJudged).
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
	if gitDir != "" {
		look.env = []string{"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + top}
	} else {
		out, status, err := readGitOut(ctx, dir, look, "rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir")
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if err != nil || status != 0 || len(lines) != 3 {
			return 0, "", errors.Join(err, errors.New("it isn't a repo git can write here"))
		}
		top, gitDir, common = filepath.FromSlash(lines[0]), filepath.FromSlash(lines[1]), filepath.FromSlash(lines[2])
	}
	list, status, err := readGitOut(ctx, dir, look, "config", "--list", "-z")
	if err != nil || status != 0 {
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
	status, said, err := gitRun(ctx, dir, call, args)
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
	root, err := os.OpenRoot(common)
	if err != nil {
		return 0, "", err
	}
	defer root.Close()
	if tree != "" {
		if len(changed) != 1 || before[changed[0]] != "" {
			return 0, "", errors.New("git made no new branch for the worktree")
		}
		return status, said, carryWorktree(root, folder.dir, tree, changed[0], after[changed[0]])
	}
	for _, name := range changed {
		if after[name] == "" && gitBranchCheckedOut(root, name) {
			return 0, "", fmt.Errorf("the branch %s is checked out, and git deletes no branch a worktree is on", name)
		}
		if err := carryBranch(root, name, before[name], after[name]); err != nil {
			return 0, "", err
		}
	}
	return status, said, nil
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
// fails undoes the ones before.
func carryWorktree(root *os.Root, made, tree, branch, id string) error {
	entries, err := os.ReadDir(filepath.Join(made, "worktrees"))
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return errors.Join(err, errors.New("git made no worktree registration the core can carry"))
	}
	name := entries[0].Name()
	gitdir, dotGit, err := gitWorktreeLinks(made, name, filepath.Join(root.Name(), "worktrees", name))
	if err != nil {
		return err
	}
	if err := carryBranch(root, branch, "", id); err != nil {
		return err
	}
	registration := path.Join("worktrees", name)
	if err := root.MkdirAll("worktrees", 0o755); err != nil {
		_ = carryBranch(root, branch, id, "")
		return err
	}
	if err := root.Mkdir(registration, 0o755); err != nil {
		_ = carryBranch(root, branch, id, "")
		return fmt.Errorf("a worktree named %s is registered already: %w", name, err)
	}
	undo := func() {
		_ = root.RemoveAll(registration)
		_ = carryBranch(root, branch, id, "")
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
			return root.WriteFile(to, data, 0o644)
		}
		return nil // git makes no other kind of file here
	})
	if err == nil {
		err = root.WriteFile(path.Join(registration, "gitdir"), gitdir, 0o644)
	}
	if err == nil {
		var treeRoot *os.Root
		if treeRoot, err = os.OpenRoot(tree); err == nil {
			err = treeRoot.WriteFile(".git", dotGit, 0o644)
			treeRoot.Close()
		}
	}
	if err != nil {
		undo()
		return err
	}
	return nil
}

// gitWorktreeLinks are the two files that tie a worktree git made from the core's folder made (registration name) to
// its registration carried to reg in the repo: the registration's gitdir, naming the tree's .git, and the tree's .git
// file, naming the registration. git made them for the core's folder; here they are as the repo's own git writes them
// (FXRELPATHS, SIERRA's ruling on RT126GMSCOPEPINR2 F2): between the real paths, relative to each other when git
// was asked for relative paths (the repo's worktree.useRelativePaths, handed over with its other data keys), else
// whole. The form is the one git chose, read from the gitdir it wrote: git read the setting itself, so the owner's
// choice is kept as git takes it, and git's real path of the tree is in that file, so nothing in the tree's folder
// is looked up again. reg is as the core names the repo's common folder (git's own path for it).
func gitWorktreeLinks(made, name, reg string) (gitdir, dotGit []byte, err error) {
	from := filepath.Join(made, "worktrees", name)
	data, err := cpReadData(filepath.Join(from, "gitdir"), 4096)
	written := strings.TrimSuffix(string(data), "\n")
	if err != nil || written == "" || strings.ContainsAny(written, "\n\r\x00") {
		return nil, nil, errors.Join(err, errors.New("git wrote no worktree place the core can carry"))
	}
	if filepath.IsAbs(filepath.FromSlash(written)) {
		// Whole: git's own gitdir names the tree's real place, wherever the core's folder is; the .git file names
		// the registration where it was carried.
		return []byte(written + "\n"), []byte("gitdir: " + reg + "\n"), nil
	}
	// Relative: git wrote it from the real path of the registration in the core's folder (a folder of the core's
	// own, in the support folder), so it leads to the tree's real .git from there.
	real, err := filepath.EvalSymlinks(from)
	if err != nil {
		return nil, nil, err
	}
	dot := filepath.Clean(filepath.Join(real, filepath.FromSlash(written)))
	if filepath.Base(dot) != ".git" {
		return nil, nil, errors.New("git wrote a worktree place that isn't a .git")
	}
	tree := filepath.Dir(dot)
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
	if folder, err := root.Open("worktrees"); err == nil {
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
		file, err := root.Open(head)
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
	file, err := root.Open(name)
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
	file, err := root.Open("packed-refs")
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
