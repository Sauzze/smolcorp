package desktop

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
)

// Take over's look at the worktree (SA-X7; SANDBOX-PLAN A0.1 C6, AQ22). Taken over, a hire's worktree leaves its
// sandbox: T3 Code's agent is your own Claude Code, and the git it runs there (and any you run there yourself) is your
// own, outside any sandbox, with your config and none of the core's hardening. That git reads what the worktree
// names: its .git file says which git folder is its own, that folder's commondir says where the repo's shared one is,
// its config.worktree is config, and the hooks folder your config names runs at each commit. A hire could have
// changed any of these (one without a sandbox, or one from before its sandbox's first day), so before T3 Code hears of
// the folder the core checks, running nothing but git in the hire's own sandbox:
//   - its .git file names its registration in your repo, the one the repo's own records have for that folder
//     (deskTreeOf, deskDotGitNamesIt);
//   - that registration's commondir leads to your repo's shared git folder;
//   - no rebase, merge, cherry-pick or revert is under way in that registration (t3NothingUnderway);
//   - its config.worktree, if there is one, holds data only (gitHandOverConfig): nothing is blanked, as your git
//     blanks nothing;
//   - the hooks folder your config names (core.hooksPath), when it is in the worktree, is the base branch's, file for
//     file, by content: no file changed or added there (ignored ones included), no link in it or on the way to it.
//     Read from the disk and hashed here, never from the index, which the hire writes. The base branch is the repo's
//     default (origin's HEAD), else main, else master: the branches a desk's sandbox can't move. A hooks folder
//     elsewhere isn't the hire's to write (the repo's own .git/hooks is SA-X12's to list);
//   - no repo of its own is below its top (t3NoRepoBelow): your git, run from inside one, would work on that repo, with
//     its config and its hooks, the hire's (RT126SAX7R2's F3).
//
// It runs once the hire is stopped and nothing works in its worktree, after what a Linux desk's guard stopped it for
// was put back (deskKept), just before its fence: what T3 Code's git will read is what is checked. A worktree that
// can't go is handed back at once, resumed in its tmux session.
//
// Not covered, and said in the report: a hook that runs another file of the worktree (a script it calls) runs the
// hire's copy of that file. On Windows, with core.autocrlf on (Git for Windows' default), a hooks folder in the
// worktree is checked out with CRLF line ends, so its files never hash as the base branch's and Take over is always
// refused there: it fails closed. A link in the worktree to a repo outside it isn't walked into (t3NoRepoBelow):
// your git, run in the folder it leads to, works on that repo with its config and hooks, which matters only for a
// place outside the worktree a desk can write (RT126SAX7R3's F2′).

const (
	t3HooksMost     = 200     // files in a hooks folder; more and it isn't handed over
	t3HookFileBytes = 1 << 20 // one hook file's size; more and it isn't handed over
)

// t3TreeCheck is why hire h's worktree can't be handed to T3 Code, in words, "" when it can.
func t3TreeCheck(h Hire) string {
	ctx, cancel := context.WithTimeout(context.Background(), worktreeTimeout)
	defer cancel()
	if !ownWorktree(h) {
		return "its worktree holds your checkout"
	}
	t, err := deskTreeOf(ctx, h, true)
	if err != nil {
		return err.Error()
	}
	defer t.close()
	parent, err := os.OpenRoot(filepath.Dir(t.top))
	if err != nil {
		return "its worktree's folder couldn't be opened"
	}
	defer parent.Close()
	if why := deskDotGitNamesIt(parent, filepath.Base(t.top), t); why != "" {
		return why
	}
	data, err := cpReadData(filepath.Join(t.gitDir, "commondir"), 4096)
	common := strings.TrimSpace(string(data))
	if err != nil || common == "" {
		return "its registration has no commondir"
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(t.gitDir, common)
	}
	if physical(common) != physical(t.common) {
		return "its registration's commondir names another git folder than your repo's"
	}
	if why := t3NothingUnderway(t.gitDir); why != "" {
		return why
	}
	if why := t3WorktreeConfigIsData(ctx, h, t); why != "" {
		return why
	}
	if why := t3HooksAreTheBase(ctx, h, t); why != "" {
		return why
	}
	return t3NoRepoBelow(ctx, t.top)
}

// t3NoRepoBelow is why the worktree at top holds a repo of its own below its top, "" when it holds none. Your git,
// run in a folder, takes the first repo it finds on the way up, and works on it with its config and hooks: below the
// top, those would be the hire's. A repo there is a .git entry (a folder, a file naming one, a link; in any case of
// its letters, as a Mac's disk finds it) other than the top's own, which deskDotGitNamesIt checked, or a folder git
// would take as a repo: a HEAD with objects and refs beside it (a bare repo's look), or a HEAD with a commondir beside
// it, as git takes objects and refs from the folder a commondir names (RT126SAX7R3's F1′). A HEAD alone is a file of
// that name. A submodule the hire set up is one too, and is
// refused. Walked on the disk through a handle that follows no link (a link is never walked into, and your git, in
// the folder a link leads to, starts from there); ctx bounds the walk, and one that can't finish fails closed.
func t3NoRepoBelow(ctx context.Context, top string) string {
	root, err := os.OpenRoot(top)
	if err != nil {
		return "its worktree couldn't be looked through"
	}
	defer root.Close()
	found := ""
	walked := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case ctx.Err() != nil:
			return ctx.Err()
		case path == ".git":
			return nil // the top's own: a file, deskDotGitNamesIt's
		case strings.EqualFold(entry.Name(), ".git"):
			found = path
		case strings.EqualFold(entry.Name(), "HEAD") && path != entry.Name():
			// Not at the top, where your git takes the .git file first.
			dir := filepath.Dir(path)
			_, objects := root.Lstat(filepath.Join(dir, "objects"))
			_, refs := root.Lstat(filepath.Join(dir, "refs"))
			_, common := root.Lstat(filepath.Join(dir, "commondir"))
			if objects == nil && refs == nil || common == nil {
				found = dir
			}
		}
		if found != "" {
			return errT3Differs
		}
		return nil
	})
	switch {
	case found != "":
		return "a repo of its own is in its worktree (" + firstRunes(filepath.ToSlash(found), 200) + "), and your git there would use its config and hooks"
	case walked != nil:
		return "its worktree couldn't be looked through for a repo of its own"
	}
	return ""
}

// t3Underway are the files and folders in a worktree's registration that hold a rebase, merge, cherry-pick or revert
// under way (the sequencer's included): what git carries on with at its --continue. The hire writes its registration
// (its rebase's and merge's state stay its own), and a rebase's to-do list can hold an exec line, which your git, or
// T3 Code's agent, would run as you there (FXSWEEP3P3 F-6).
var t3Underway = []string{"rebase-merge", "rebase-apply", "sequencer", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"}

// t3NothingUnderway is why the registration gitDir holds a rebase, merge, cherry-pick or revert under way, "" when it
// holds none. Anything of those names counts, whatever it is: one that can't be looked at counts too.
func t3NothingUnderway(gitDir string) string {
	for _, name := range t3Underway {
		if _, err := os.Lstat(filepath.Join(gitDir, name)); !errors.Is(err, fs.ErrNotExist) {
			return "its git folder has " + name + ", a rebase, merge or cherry-pick under way that your git would carry on"
		}
	}
	return ""
}

// t3WorktreeConfigIsData is why the registration's config.worktree can't go to your git, "" for none: one that isn't
// there is fine.
func t3WorktreeConfigIsData(ctx context.Context, h Hire, t deskTree) string {
	file := filepath.Join(t.gitDir, "config.worktree")
	info, err := os.Lstat(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ""
	case err != nil || !info.Mode().IsRegular():
		return "its config.worktree isn't a plain file"
	}
	out, status, err := t3TreeGit(ctx, h, t, gitReadEnv(os.Environ(), nil), gitReadArgs, nil, "config", "--file", file, "--list", "-z")
	if err != nil || status != 0 {
		return "its config.worktree couldn't be read"
	}
	if rule := gitHandOverConfig([]byte(out)); rule != "" {
		return "its config.worktree has " + rule + ", which could run a program for your own git there"
	}
	return ""
}

// t3HooksAreTheBase is why the hooks folder your config names, in h's worktree, can't go to your git, "" for none.
func t3HooksAreTheBase(ctx context.Context, h Hire, t deskTree) string {
	// Where your git takes its hooks from there: your own config on (system, user, the repo's, the worktree's), and
	// none of gitReadArgs, which name a hooks folder of their own. Reading config runs nothing.
	env := slices.DeleteFunc(gitOwnerConfigEnv(os.Environ()), func(kv string) bool {
		return strings.HasPrefix(kv, "GIT_CONFIG_NOSYSTEM=")
	})
	out, status, err := t3TreeGit(ctx, h, t, env, []string{"--no-pager"}, nil, "config", "--get", "core.hooksPath")
	switch {
	case err == nil && status == 1:
		return "" // none: the repo's own hooks folder, in its shared git folder
	case err != nil || status != 0:
		return "where your git takes its hooks from there couldn't be read"
	}
	named := strings.TrimSpace(out)
	if named == "" || strings.HasPrefix(named, "%(prefix)") {
		return "" // git's own install: not the worktree
	}
	if strings.HasPrefix(named, "~") {
		// As git expands it: "~/" is your home, "~name/" that user's. One whose home isn't found here isn't let go.
		expanded, ok := t3ExpandHome(named)
		if !ok {
			return "the hooks folder your config names (" + named + ") couldn't be found"
		}
		named = expanded
	}
	if !filepath.IsAbs(named) {
		named = filepath.Join(t.top, named) // as git runs hooks: from the worktree's top
	}
	rel, in := t3Inside(t.top, filepath.Clean(named))
	if !in {
		if rel, in = t3Inside(t.top, physical(named)); !in {
			return ""
		}
	}
	shown := filepath.ToSlash(rel)
	// No link on the way: the folder git runs hooks from is the one walked here.
	at := t.top
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." {
			continue
		}
		at = filepath.Join(at, part)
		info, err := os.Lstat(at)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return "" // no such folder: no hook runs
		case err != nil:
			return "the hooks folder your config names (" + shown + ") couldn't be looked at"
		case info.Mode()&fs.ModeSymlink != 0:
			return "a link is on the way to the hooks folder your config names (" + shown + ")"
		case !info.IsDir():
			return "" // a file where a folder would be: no hook runs
		}
	}
	base := ""
	pinned, now := deskDefaultBranches(t.common)
	for _, branch := range []string{pinned, now, "main", "master"} {
		if branch == "" {
			continue
		}
		if status, _, ok := deskGit(ctx, h, h.ID, t.gitDir, t.top, t.top, []string{"rev-parse", "--verify", "--quiet", "refs/heads/" + branch + "^{commit}"}); ok && status == 0 {
			base = branch
			break
		}
	}
	if base == "" {
		return "your repo has no main, master or default branch to compare the hooks folder your config names (" + shown + ") with"
	}
	kept, why := t3BaseFiles(ctx, h, t, base, shown)
	if why != "" {
		return why
	}
	return t3SameFiles(filepath.Join(t.top, rel), kept, base, shown)
}

// t3ExpandHome is path, which starts with "~", with its home folder in place of "~" or "~name", as git expands a
// path in config; ok false when there is no such home.
func t3ExpandHome(path string) (string, bool) {
	name, rest := path[1:], ""
	if i := strings.IndexAny(name, "/"+string(filepath.Separator)); i >= 0 {
		name, rest = name[:i], name[i+1:]
	}
	home, err := "", error(nil)
	if name == "" {
		home, err = os.UserHomeDir()
	} else {
		var u *user.User
		if u, err = user.Lookup(name); err == nil {
			home = u.HomeDir
		}
	}
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, filepath.FromSlash(rest)), true
}

// t3Inside is path as a part of top, and whether it is in it.
func t3Inside(top, path string) (string, bool) {
	rel, err := filepath.Rel(top, path)
	return rel, err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// t3BaseFile is a file of the base branch's hooks folder: its git object, and whether git keeps it as a link.
type t3BaseFile struct {
	object string
	link   bool
}

// t3BaseFiles is the base branch's hooks folder (shown, from the worktree's top), each file by its path in it.
func t3BaseFiles(ctx context.Context, h Hire, t deskTree, base, shown string) (map[string]t3BaseFile, string) {
	tree := "refs/heads/" + base + ":" + shown
	if shown == "." {
		tree = "refs/heads/" + base + "^{tree}"
	}
	files := map[string]t3BaseFile{}
	status, _, ok := deskGit(ctx, h, h.ID, t.gitDir, t.top, t.top, []string{"rev-parse", "--verify", "--quiet", tree})
	if ok && status == 1 {
		return files, "" // base has no such folder
	}
	// ls-tree reads the base branch's objects, no file of the worktree, so no filter or hook can run: it runs with
	// gitstate's guards, in the hire's sandbox when it has one, as the rev-parse above does. Not through deskGit,
	// whose desk with no sandbox runs only what reads no file at all or goes through the core's folder (FXGITSTATECFGR1).
	// Its records are read as they come (FXPOLISHA, RT126FIXREVIEW-WINWORKTREER4's O2): a folder named at the worktree's
	// top is the whole tree, whose listing passes gitOutput's cap in a big repo. A folder of more than t3HooksMost files
	// is never handed over (t3SameFiles), so the listing stops there, and the words say so.
	records := &gitRecords{sep: 0, each: func(record []byte) error {
		meta, path, found := strings.Cut(string(record), "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 3 {
			return nil
		}
		if len(files) == t3HooksMost {
			return errT3TooMany
		}
		files[path] = t3BaseFile{object: fields[2], link: fields[0] == "120000"}
		return nil
	}}
	_, status, err := t3TreeGit(ctx, h, t, gitReadEnv(os.Environ(), gitHere().owner), gitReadArgs, records, "ls-tree", "-r", "-z", tree)
	switch end := records.end(); {
	case errors.Is(end, errT3TooMany):
		return nil, fmt.Sprintf("the hooks folder your config names (%s) has more than %d files on %s, more than Take over can compare", shown, t3HooksMost, base)
	case end != nil || err != nil || status != 0:
		return nil, "the hooks folder your config names (" + shown + ") couldn't be read from " + base
	}
	return files, ""
}

// errT3TooMany stops t3BaseFiles' listing past t3HooksMost files.
var errT3TooMany = errors.New("more files than a hooks folder handed over holds")

// t3SameFiles is why the folder dir on disk isn't base's files, "" when every file in it is one of them by content:
// read through a handle on the folder that follows no link, so what is hashed is what git would run.
func t3SameFiles(dir string, files map[string]t3BaseFile, base, shown string) string {
	differs := "the hooks folder your config names (" + shown + ") isn't " + base + "'s"
	root, err := os.OpenRoot(dir)
	if err != nil {
		return differs
	}
	defer root.Close()
	seen := 0
	walked := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if seen++; seen > t3HooksMost || !entry.Type().IsRegular() {
			return errT3Differs // a link, a fifo, a device, or too many
		}
		want, found := files[path]
		if !found || want.link {
			return errT3Differs
		}
		got, err := t3BlobID(root, path, len(want.object))
		if err != nil || got != want.object {
			return errT3Differs
		}
		return nil
	})
	if walked != nil {
		return differs
	}
	return ""
}

var errT3Differs = errors.New("not the base branch's")

// t3BlobID is the git object id of the file name in root, as its content would be stored, by the length of the ids
// the repo uses (40: SHA-1, 64: SHA-256).
func t3BlobID(root *os.Root, name string, length int) (string, error) {
	var sum hash.Hash
	switch length {
	case 40:
		sum = sha1.New()
	case 64:
		sum = sha256.New()
	default:
		return "", errors.New("an object id of no known length")
	}
	file, err := root.OpenFile(name, cpDataFlags, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > t3HookFileBytes {
		return "", errT3Differs
	}
	fmt.Fprintf(sum, "blob %d\x00", info.Size())
	if n, err := io.Copy(sum, io.LimitReader(file, info.Size()+1)); err != nil || n != info.Size() {
		return "", errT3Differs
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// t3TreeGit is git with global options and args in h's worktree (t), the repo named, in h's sandbox when it has one,
// with env: its standard output and error, and its status. With to, its standard output goes there as it comes, and
// only its standard error is answered.
func t3TreeGit(ctx context.Context, h Hire, t deskTree, env, global []string, to io.Writer, args ...string) (string, int, error) {
	if !gitVersionAtLeast(gitHere().version, 2, 32) {
		return "", 0, errors.New("the git on this computer is missing, or older than 2.32")
	}
	git, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(git) {
		return "", 0, errors.New("no git by a whole path")
	}
	env = append(slices.Clip(env), "GIT_DIR="+t.gitDir, "GIT_WORK_TREE="+t.top)
	var out gitOutput
	var stdout io.Writer = &out
	if to != nil {
		stdout = to
	}
	err = deskRun(ctx, h.ID, t.top, env, stdout, &out, git, append(slices.Clone(global), args...)...)
	var exit *exec.ExitError
	switch {
	case out.over:
		return "", 0, errGitTooBig
	case errors.As(err, &exit) && ctx.Err() == nil:
		return out.String(), exit.ExitCode(), nil
	case err != nil:
		return "", 0, err
	}
	return out.String(), 0, nil
}
