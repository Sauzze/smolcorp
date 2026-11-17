package desktop

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// The git folder a read's status and diff go through (FXGITSTATECFG, RT126RESOLVESWEEP S1). The repo read here is
// one a desk may write: its config and its .git/info/attributes are the desk's. Looking at the config first and then
// running git, which reads it again, leaves a gap a desk can change it in, and `-c` can only blank a filter by the
// name it was seen under, never "every filter". So the git that reads the files never reads the repo's own git folder
// for settings at all. It is given one the core makes for that read alone, in the support folder (which no desk can
// read or write, policy.go's rowSupport), and that holds only:
//   - config: the repo's format and nothing else;
//   - HEAD and packed-refs: the branch and the commit, and the branches and remote branches status counts ahead and
//     behind against, as git listed them;
//   - info/exclude, shallow and info/sparse-checkout: copies of the repo's (data, read as plain small files), and no
//     info/attributes;
//   - hooks: an empty folder, which core.hooksPath names (and core.askPass is blanked).
//
// The repo's objects and index are named in git's environment (GIT_OBJECT_DIRECTORY, GIT_INDEX_FILE), and its
// working folder by GIT_WORK_TREE: git reads them as data. The settings the repo's config held when it was judged
// (gitRepoConfig) are handed over in the environment, less every key that names a program (gitForwarded). The
// system and user config are off (gitReadEnv): of the owner's, only their line ends are handed over, by value
// (gitOwnerLineEnds). So the only config the git that reads the files sees is what this file wrote: a filter, text
// converter or fsmonitor a desk names later, or names in an include, or under a name the check never saw, is in no
// file git reads. An attribute naming a filter git has no setting for runs nothing.

// gitReadsFolder is where each read's own git folder is made: in the support folder, never the temp folder, which a
// desk may write.
func gitReadsFolder() string { return filepath.Join(supportDir, "git-reads") }

// gitReadsSwept is each git-reads folder whose leftovers this run has removed (sweepGitReads).
var gitReadsSwept sync.Map

// sweepGitReads makes the git-reads folder, and the first time in a run removes what a core that stopped mid-read (a
// crash) left there: every folder in it an hour old or more. A read takes seconds, so none of this run's is.
func sweepGitReads() error {
	folder := gitReadsFolder()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	if _, swept := gitReadsSwept.LoadOrStore(folder, true); swept {
		return nil
	}
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) >= time.Hour {
			_ = os.RemoveAll(filepath.Join(folder, entry.Name()))
		}
	}
	return nil
}

// gitCopiesMost is the most of one of the repo's data files a read copies.
const gitCopiesMost = 4 << 20

// gitReadFolder is one read's git folder, and how status and diff are run through it.
type gitReadFolder struct {
	dir    string
	format string // the repo's object format (sha1, sha256)
	call   gitCall
}

func (f gitReadFolder) remove() {
	if f.dir != "" {
		_ = os.RemoveAll(f.dir)
	}
}

// makeGitReadFolder makes the git folder for reading the repo whose top is top and git folder gitDir, through call
// (the read's guards, with the owner's settings), with list its config as judged. Each git it runs here reads no file
// of the working folder (rev-parse, symbolic-ref, for-each-ref), so no filter can run.
func makeGitReadFolder(ctx context.Context, dir, top, gitDir string, read gitReader, call gitCall, list []byte) (gitReadFolder, error) {
	out, status, err := read(ctx, dir, call, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-object-format")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if err != nil || status != 0 || len(lines) != 2 {
		return gitReadFolder{}, errors.Join(err, errors.New("git didn't say where the repo's common folder is"))
	}
	common, format := filepath.FromSlash(lines[0]), lines[1]
	if _, known := gitEmptyTrees[format]; !known {
		return gitReadFolder{}, errors.New("an object format this read doesn't know")
	}
	head, status, err := read(ctx, dir, call, "symbolic-ref", "-q", "HEAD")
	switch {
	case err != nil:
		return gitReadFolder{}, err
	case status == 0:
		head = []byte("ref: " + strings.TrimSpace(string(head)) + "\n")
	case status == 1: // detached
		id, status, err := read(ctx, dir, call, "rev-parse", "-q", "--verify", "HEAD")
		if err != nil || status != 0 {
			return gitReadFolder{}, errors.Join(err, errors.New("a detached HEAD git can't name"))
		}
		head = []byte(strings.TrimSpace(string(id)) + "\n")
	default:
		return gitReadFolder{}, errors.New("git couldn't say what HEAD is")
	}
	refs, status, err := read(ctx, dir, call, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/", "refs/remotes/")
	if err != nil || status != 0 {
		return gitReadFolder{}, errors.Join(err, errors.New("git couldn't list the branches"))
	}

	if err := sweepGitReads(); err != nil {
		return gitReadFolder{}, err
	}
	made, err := os.MkdirTemp(gitReadsFolder(), "read-")
	if err != nil {
		return gitReadFolder{}, err
	}
	f := gitReadFolder{dir: made, format: format}
	files := map[string][]byte{
		"config":      []byte("[core]\n\trepositoryformatversion = 1\n[extensions]\n\tobjectformat = " + format + "\n"),
		"HEAD":        head,
		"packed-refs": refs,
	}
	for name, from := range map[string]string{"info/exclude": common, "shallow": common, "info/sparse-checkout": gitDir} {
		data, err := cpReadData(filepath.Join(from, filepath.FromSlash(name)), gitCopiesMost)
		switch {
		case err == nil:
			files[name] = data
		case !errors.Is(err, fs.ErrNotExist):
			f.remove()
			return gitReadFolder{}, errors.Join(err, errors.New("the repo's "+name+" isn't a plain small file"))
		}
	}
	for _, folder := range []string{"refs/heads", "refs/tags", "info", "hooks"} {
		if err == nil {
			err = os.MkdirAll(filepath.Join(made, filepath.FromSlash(folder)), 0o700)
		}
	}
	for name, data := range files {
		if err == nil {
			err = os.WriteFile(filepath.Join(made, filepath.FromSlash(name)), data, 0o600)
		}
	}
	if err != nil {
		f.remove()
		return gitReadFolder{}, err
	}
	// The owner's line ends go around the repo's own settings as git orders them (gitOwnerLineEnds): their system and
	// global values before the repo's, so a repo's own core.autocrlf wins as it does for the owner's git, and the
	// core's environment's after. All in the environment list, where a later entry wins; never `-c`, which would put
	// the owner's global value above the repo's. They're read here, for this git alone, and never in call (the look
	// that lists the repo's config), where they would come back in list as the repo's.
	owner, command := gitOwnerLineEnds(ctx)
	f.call = gitCall{
		// core.askPass too, though a repo that sets it isn't read (gitRepoConfig) and no transport is allowed.
		global: append(append([]string{}, call.global...), "-c", "core.hooksPath="+filepath.Join(made, "hooks"), "-c", "core.askPass="),
		config: slices.Concat(call.config, owner, gitForwarded(list), command),
		env: []string{"GIT_DIR=" + made, "GIT_WORK_TREE=" + top, "GIT_INDEX_FILE=" + filepath.Join(gitDir, "index"),
			"GIT_OBJECT_DIRECTORY=" + filepath.Join(common, "objects")},
	}
	return f, nil
}

// gitNotForwarded are the keys of a repo's config never handed to the git that reads its files, beyond every key
// that names a program: where the repo is and its format (the read's own folder says those), and includes (git
// listed what they hold already, and an include handed over would be read again).
var gitNotForwarded = gitKeySet(`core.worktree core.bare core.repositoryformatversion include.path includeif.*.path`)

// gitRuleOf is key's rule ("section.name", or "section.*.name" for a key with a subsection) and its section, as
// gitRepoConfig reads it: "" for a key with no section.
func gitRuleOf(key string) (rule, section string) {
	first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
	if key == "" || first < 0 {
		return "", ""
	}
	section, name := strings.ToLower(key[:first]), strings.ToLower(key[last+1:])
	if first != last {
		return section + ".*." + name, section
	}
	return section + "." + name, section
}

// gitDataOnly is list (`config --list -z`) with only the settings a read knows to be data (gitDataKeys,
// gitDataSections), for a git that writes through a folder of the core's own (ownergit.go): a key the read doesn't
// know is left out, never handed to git, rather than refuse the repo.
func gitDataOnly(list []byte) []byte {
	var kept []string
	for _, record := range strings.Split(string(list), "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		if rule, section := gitRuleOf(key); rule != "" && (gitDataKeys[rule] || gitDataSections[section]) {
			kept = append(kept, record)
		}
	}
	return []byte(strings.Join(kept, "\x00"))
}

// gitForwarded is the repo's config as judged (list, `config --list -z`), for the git that reads its files: every
// setting but those that name a program, whatever their name (gitBlankedKeys, gitBlankedSections, the keys gitReadArgs
// set) and those gitNotForwarded names, and no extension (the read's folder has its own format). A key a read doesn't
// know never gets here: gitRepoConfig refused the repo.
func gitForwarded(list []byte) [][2]string {
	var kept [][2]string
	for _, record := range strings.Split(string(list), "\x00") {
		key, value, _ := strings.Cut(record, "\n")
		rule, section := gitRuleOf(key)
		if rule == "" {
			continue
		}
		_, blanked := gitBlankedKeys[rule]
		if blanked || gitBlankedSections[section] || gitFixedKeys[rule] || gitNotForwarded[rule] || section == "extensions" {
			continue
		}
		kept = append(kept, [2]string{key, value})
	}
	return kept
}

// The owner's line ends (RT126CRLFRULING). The git the core runs through its own folder reads no system or user
// config (gitReadEnv), so without these a hire's worktree would be checked out, and its files read, as no git of the
// owner's would: LF where their git writes CRLF, and a file the hire's own git restored would read as changed to the
// core (the fire's look, the window's counts) while the owner's git calls it clean. So the owner's choice is carried
// ("a real user's git settings stay theirs", WIN125b), and only it: core.autocrlf, core.eol and core.safecrlf. They are
// data (gitDataKeys), already forwarded from a repo's own config, which a desk can write: carrying the owner's adds
// nothing a desk couldn't do. None names a program or a path, and each value must be one of these, in any case, and
// is handed over as written here; any other value is dropped.
var gitLineEndValues = map[string]map[string]string{
	"core.autocrlf": {"true": "true", "yes": "true", "on": "true", "1": "true", "false": "false", "no": "false", "off": "false", "0": "false", "": "false", "input": "input"},
	"core.eol":      {"lf": "lf", "crlf": "crlf", "native": "native"},
	"core.safecrlf": {"true": "true", "yes": "true", "on": "true", "1": "true", "false": "false", "no": "false", "off": "false", "0": "false", "": "false", "warn": "warn"},
}

// gitLineEndsReadDropped are the names left out of the environment of gitOwnerLineEnds' read: what would point git at
// a repo, and GIT_CONFIG, which only `git config` itself reads (in place of the system and user files), never the git
// that checks files out, so it isn't how the owner's git sees its line ends.
var gitLineEndsReadDropped = map[string]bool{"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
	"GIT_INDEX_FILE": true, "GIT_CEILING_DIRECTORIES": true, "GIT_CONFIG": true}

// gitOwnerLineEnds is the owner's line-end settings, as their own git reads them: owner from the system and global
// scopes (both global files, ~/.gitconfig and the XDG one: a `config --global` read sees only the first when it
// exists), command from the core's own environment (GIT_CONFIG_COUNT, GIT_CONFIG_PARAMETERS). It's one read of config,
// which runs nothing, in a fresh empty folder in the support folder (which desks are denied) as the repo, so no repo's
// config, local or of a worktree, is in it, and with no includes followed, so no file an include names (one a desk
// may have written) is either. The owner's GIT_CONFIG_SYSTEM, GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM and
// GIT_CONFIG_COUNT/KEY/VALUE are kept: they're how the owner's git reads its config. It's read for each git, not once
// per run, so a setting the owner changes applies at the next read. Nothing set, or any error: nothing is carried,
// which leaves the line ends to the repo's own settings and runs nothing.
func gitOwnerLineEnds(ctx context.Context) (owner, command [][2]string) {
	if err := sweepGitReads(); err != nil {
		return nil, nil
	}
	empty, err := os.MkdirTemp(gitReadsFolder(), "config-")
	if err != nil {
		return nil, nil
	}
	defer os.RemoveAll(empty)
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); !gitLineEndsReadDropped[strings.ToUpper(name)] {
			env = append(env, kv)
		}
	}
	env = append(env, "GIT_DIR="+empty, "GIT_TERMINAL_PROMPT=0")
	out, status, err := execGit(ctx, env, "-C", empty, "--no-pager", "config", "--no-includes", "--show-scope", "-z",
		"--get-regexp", `^core\.(autocrlf|eol|safecrlf)$`)
	if err != nil || status != 0 {
		return nil, nil
	}
	// Each setting is "<scope>\0<key>\n<value>\0", or "<scope>\0<key>\0" for a key with no value (true, to git).
	records := strings.Split(string(out), "\x00")
	if len(records)%2 != 1 || records[len(records)-1] != "" {
		return nil, nil
	}
	for i := 0; i+1 < len(records); i += 2 {
		key, value, given := strings.Cut(records[i+1], "\n")
		key = strings.ToLower(key)
		if !given {
			value = "true"
		}
		normal, known := gitLineEndValues[key][strings.ToLower(value)]
		if !known {
			continue
		}
		switch records[i] {
		case "system", "global":
			owner = append(owner, [2]string{key, normal})
		case "command":
			command = append(command, [2]string{key, normal})
		}
	}
	return owner, command
}

// gitLineEndsAround is the owner's line ends (gitOwnerLineEnds) for a git that reads the repo's own config (deskGit in
// a desk's sandbox), whose config list was read as list: there the repo's settings are git's local scope, below
// anything in the environment, so an owner's system or global value is carried only for a key the repo doesn't set,
// which is what the owner's git does. The core's environment's values are carried whatever the repo says, as git does.
func gitLineEndsAround(ctx context.Context, list []byte) [][2]string {
	owner, command := gitOwnerLineEnds(ctx)
	set := map[string]bool{}
	for _, record := range strings.Split(string(list), "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		set[strings.ToLower(key)] = true
	}
	return append(slices.DeleteFunc(owner, func(kv [2]string) bool { return set[kv[0]] }), command...)
}
