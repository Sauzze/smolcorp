package desktop

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// Git state of a folder (1.26 FG1): the repo's top, its branch and upstream, how far ahead and behind, the
// changed files and the lines added and removed against HEAD. It only reads: no command here writes a repo (no
// optional locks, so even `status` leaves the index alone). It also runs nothing the repo names: hires write the
// repos read here, and a repo's own config can point at programs (filters, text converters, an external diff, an
// fsmonitor, hooks, a pager, an ssh or credential helper, a fetch's upload-pack), so each of those is blanked or
// refused: a repo whose config has a key a read doesn't know isn't read at all (gitRepoConfig), no transport is
// allowed, and the owner's own git config is left out too (see gitReadArgs and gitReadEnv). Git older than 2.32 can't
// be given those settings safely, so there only where the repo is gets read. The PR badge and the diffs read it, and
// it never goes in the /events snapshot, so phones see nothing.
//
// Each read runs off the main loop with no shell, a timeout, no pager and no prompts. Answers are kept per folder:
// a folder is read again at most every gitStateEvery, then only when its index or HEAD moved or its session's
// last event did (or after gitStateMaxAge, for what neither shows).

const (
	gitStateEvery   = 5 * time.Second
	gitStateMaxAge  = time.Minute
	gitStateTimeout = 10 * time.Second
	gitOutputCap    = 4 << 20 // bytes of one answer from git; more and that part is skipped
	gitFilesShown   = 200
	gitStatesKept   = 64
)

var errGitTooBig = errors.New("git's answer is over the size cap")

// GitFile is one changed file: its place in the repo, git's two-letter state ("M.", ".M", "A.", "R.", "UU", "??")
// and its lines added and removed against HEAD (an untracked file has none counted).
type GitFile struct {
	Path    string `json:"path"`
	State   string `json:"state"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// GitState is what the office and the GM are told. Repo false is a folder in no repo. Skipped says some of the
// numbers are missing, and Note says why, in words for the owner.
type GitState struct {
	Repo     bool      `json:"repo"`
	Top      string    `json:"top,omitempty"`
	Branch   string    `json:"branch,omitempty"` // "" when detached or before the first commit
	Detached bool      `json:"detached,omitempty"`
	Head     string    `json:"head,omitempty"` // the commit's short id
	Upstream string    `json:"upstream,omitempty"`
	Ahead    int       `json:"ahead"`
	Behind   int       `json:"behind"`
	Changed  int       `json:"changed"`
	Added    int       `json:"added"`
	Removed  int       `json:"removed"`
	Files    []GitFile `json:"files"`
	More     int       `json:"more,omitempty"` // changed files beyond those listed
	Skipped  bool      `json:"skipped,omitempty"`
	Note     string    `json:"note,omitempty"`
	At       int64     `json:"at"` // when it was read, in milliseconds
	// remote is the address of the remote the branch tracks, as written in the repo's config (it may hold a user
	// name). Never sent: the pull request badge reads the owner and name off it (ghpr.go).
	remote string
}

// gitCall is what goes with a command: global is the options before the command's own name (the guards of
// gitReadArgs and what readGitState adds to them), and config is settings handed to git in its environment
// (GIT_CONFIG_COUNT, KEY_n and VALUE_n), where a key is never split as `-c` splits it, at its first "=": a filter
// named "x=y" is blanked there as "filter.x=y.clean", where `-c` would set "filter.x" and leave it.
type gitCall struct {
	global []string
	config [][2]string
}

// gitReader runs one read-only git command in dir and returns its standard output, its exit code, and an error
// when it could not run, took too long (ctx) or answered more than the cap.
type gitReader func(ctx context.Context, dir string, call gitCall, args ...string) (out []byte, status int, err error)

// gitOutput takes git's output and stops the copy, so git ends on a broken pipe, past the cap.
type gitOutput struct {
	bytes.Buffer
	over bool
}

func (b *gitOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > gitOutputCap {
		b.over = true
		return 0, errGitTooBig
	}
	return b.Buffer.Write(p)
}

// gitReadArgs go before every command: nothing paged, no lock taken for a refresh, no transport (so no fetch, not
// even the lazy one a partial clone starts for an object it lacks; gitReadEnv refuses it twice more), and every
// program a repo's config can name with a key of its own blanked (fsmonitor, hooks, pager, attributes file, editors,
// ssh, credential and gpg helpers, the external diff). The commands add --no-ext-diff and --no-textconv where they
// diff. Programs a repo names under a name of its own (a filter, a text converter) are blanked by readGitState,
// which reads the config first to know them (gitRepoConfig). None of these keys holds an "=", so `-c` is safe here.
var gitReadArgs = []string{"--no-pager", "--no-optional-locks", "-c", "protocol.allow=never",
	"-c", "core.fsmonitor=", "-c", "core.pager=cat", "-c", "core.hooksPath=" + os.DevNull, "-c", "core.attributesFile=" + os.DevNull,
	"-c", "core.sshCommand=", "-c", "core.editor=", "-c", "sequence.editor=", "-c", "gpg.program=", "-c", "credential.helper=",
	"-c", "diff.external="}

// gitConfigNames are the environment variables that put git config (or a program, or a transport) in git's hands
// from outside (and git's own test switches, GIT_TEST_*, which change what it writes, and the switches that change
// what every pathspec means, GIT_*_PATHSPECS): they are dropped, whole or by prefix, from a read's environment. GIT_TEST_ASSUME_DIFFERENT_OWNER is kept: it only makes git refuse a repo as
// another account's, never write more, and it is how the tests stand in for another account.
var gitConfigNames = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true, "GIT_OBJECT_DIRECTORY": true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_PAGER": true, "GIT_TERMINAL_PROMPT": true, "GIT_OPTIONAL_LOCKS": true,
	"GIT_CONFIG": true, "GIT_CONFIG_COUNT": true, "GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_SYSTEM": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_EXTERNAL_DIFF": true, "GIT_ATTR_SOURCE": true,
	"GIT_SSH": true, "GIT_SSH_COMMAND": true, "GIT_ASKPASS": true, "SSH_ASKPASS": true, "GIT_PROXY_COMMAND": true,
	"GIT_ALLOW_PROTOCOL": true, "GIT_PROTOCOL_FROM_USER": true, "GIT_NO_LAZY_FETCH": true, "GIT_EXEC_PATH": true,
	"GIT_LITERAL_PATHSPECS": true, "GIT_GLOB_PATHSPECS": true, "GIT_NOGLOB_PATHSPECS": true, "GIT_ICASE_PATHSPECS": true,
}

// gitReadEnv is the environment of a read: the app's, without what would point git at another repo or hand it
// config, a program or a transport, with nothing to ask the owner, and with config, if any, handed over (gitCall).
// No transport: GIT_ALLOW_PROTOCOL empty allows none and, unlike protocol.allow, beats a repo's own
// protocol.<name>.allow; GIT_NO_LAZY_FETCH (git 2.44 and later; older ones ignore it) starts no fetch at all. The
// system and user git config are switched off, so they can't run anything either: the owner's ignore file and
// trusted folders are handed back by name (gitHere), and nothing else of theirs applies to what is read here.
func gitReadEnv(env []string, config [][2]string) []string {
	out := make([]string, 0, len(env)+8+2*len(config))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		if gitConfigNames[name] || strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") || (strings.HasPrefix(name, "GIT_TEST_") && name != "GIT_TEST_ASSUME_DIFFERENT_OWNER") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "GIT_ALLOW_PROTOCOL=", "GIT_NO_LAZY_FETCH=1",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if len(config) > 0 {
		out = append(out, "GIT_CONFIG_COUNT="+strconv.Itoa(len(config)))
		for i, kv := range config {
			n := strconv.Itoa(i)
			out = append(out, "GIT_CONFIG_KEY_"+n+"="+kv[0], "GIT_CONFIG_VALUE_"+n+"="+kv[1])
		}
	}
	return out
}

// readGitOut is the real gitReader: `git -C dir …` with no shell, in ctx, with only its standard output kept.
func readGitOut(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
	return execGit(ctx, gitReadEnv(os.Environ(), call.config), append(append([]string{"-C", dir}, call.global...), args...)...)
}

// execGit runs git with args in env, keeping its standard output up to the cap.
func execGit(ctx context.Context, env []string, args ...string) ([]byte, int, error) {
	cmd := disclaim.Cmd(exec.CommandContext(ctx, "git", args...))
	cmd.Env = env
	cmd.WaitDelay = 2 * time.Second // after the stop below, git has this long to clean up before it is killed
	stopGitGently(cmd)
	hideGitWindow(cmd)
	var out gitOutput
	cmd.Stdout = &out // standard input and error go to the null device
	err := cmd.Run()
	switch {
	case out.over:
		return nil, 0, errGitTooBig
	case ctx.Err() != nil:
		return nil, 0, ctx.Err()
	case err == nil:
		return out.Bytes(), 0, nil
	}
	if exit, isExit := err.(*exec.ExitError); isExit {
		return out.Bytes(), exit.ExitCode(), nil
	}
	return nil, 0, err
}

// gitHereInfo is what is asked once of the git on this computer: its version (`git --version`), and the owner's own
// ignore file and trusted folders (core.excludesFile, safe.directory) from their git config, which the reads leave
// out: paths, which run nothing, handed back so files they hide stay hidden and a repo they trust is read.
type gitHereInfo struct {
	version string
	owner   [][2]string
}

// gitHere asks once git answers: until then (git missing, say) it asks again at each read, so a git installed while
// the app runs is found.
var gitHere = func() func() gitHereInfo {
	var mu sync.Mutex
	var known gitHereInfo
	return func() gitHereInfo {
		mu.Lock()
		defer mu.Unlock()
		if known.version == "" {
			known = askGitHere()
		}
		return known
	}
}()

func askGitHere() gitHereInfo {
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	out, status, err := execGit(ctx, gitReadEnv(os.Environ(), nil), "-C", os.TempDir(), "--version")
	if err != nil || status != 0 {
		return gitHereInfo{}
	}
	here := gitHereInfo{version: string(out)}
	// The owner's own config, read with their global config left on (reading config runs nothing; --global follows
	// no includes). Only these two keys are kept.
	env := slices.DeleteFunc(gitReadEnv(os.Environ(), nil), func(kv string) bool { return strings.HasPrefix(kv, "GIT_CONFIG_GLOBAL=") })
	list, _, _ := execGit(ctx, env, "-C", os.TempDir(), "--no-pager", "config", "--global", "-z", "--get-regexp", `^(core\.excludesfile|safe\.directory)$`)
	for _, record := range strings.Split(string(list), "\x00") {
		if key, value, found := strings.Cut(record, "\n"); found {
			here.owner = append(here.owner, [2]string{key, value})
		}
	}
	return here
}

// gitVersionAtLeast reads "git version 2.43.0.windows.1" or "git version 2.39.5 (Apple Git-154)".
func gitVersionAtLeast(text string, major, minor int) bool {
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return false
	}
	nums := strings.SplitN(fields[2], ".", 3)
	if len(nums) < 2 {
		return false
	}
	a, errA := strconv.Atoi(nums[0])
	b, errB := strconv.Atoi(nums[1])
	return errA == nil && errB == nil && (a > major || (a == major && b >= minor))
}

// The empty tree, by the repo's hash: no attributes in it. With it as the attribute source (--attr-source, git 2.40
// and later) no .gitattributes in the working folder is read, so none can name a filter. It does not cover the
// repo's .git/info/attributes (git reads that whatever the source), so the filters are blanked by name as well.
var gitEmptyTrees = map[string]string{
	"sha1":   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
	"sha256": "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
}

// The config keys a read understands, as "section.name", or "section.*.name" for a key with a subsection of any
// name. A repo whose config has any other key isn't read (gitRepoConfig): fail closed, so a key a later git (or
// git-lfs, or an editor) gives a program to run is refused, not run. Every key whose value is a program is either
// blanked or not here.
var (
	// gitDataKeys hold data that rev-parse, config, status and diff run nothing on.
	gitDataKeys = gitKeySet(`core.repositoryformatversion core.filemode core.bare core.logallrefupdates core.ignorecase
		core.precomposeunicode core.symlinks core.autocrlf core.eol core.safecrlf core.checkroundtripencoding core.worktree
		core.excludesfile core.quotepath core.untrackedcache core.checkstat core.trustctime core.splitindex core.compression
		core.loosecompression core.packedgitwindowsize core.packedgitlimit core.deltabasecachelimit core.bigfilethreshold
		core.whitespace core.abbrev core.commentchar core.commentstring core.sparsecheckout core.sparsecheckoutcone
		core.multipackindex core.commitgraph core.preloadindex core.notesref core.hidedotfiles core.longpaths core.fscache
		core.filesreflocktimeout core.packedrefstimeout core.warnambiguousrefs core.restrictinheritedhandles core.fsync
		core.fsyncmethod core.fsyncobjectfiles core.createobject core.sharedrepository core.protecthfs core.protectntfs
		core.fsmonitorhookversion core.usereplacerefs
		diff.algorithm diff.autorefreshindex diff.colormoved diff.colormovedws diff.context diff.dirstat diff.ignoresubmodules
		diff.indentheuristic diff.interhunkcontext diff.mnemonicprefix diff.noprefix diff.orderfile diff.relative
		diff.renamelimit diff.renames diff.statgraphwidth diff.statnamewidth diff.submodule diff.suppressblankempty
		diff.wserrorhighlight diff.srcprefix diff.dstprefix diff.tool diff.guitool
		diff.*.binary diff.*.xfuncname diff.*.funcname diff.*.wordregex diff.*.cachetextconv
		merge.ff merge.conflictstyle merge.log merge.renames merge.renamelimit merge.tool merge.guitool merge.verbosity
		merge.autostash merge.defaulttoupstream merge.stat merge.directoryrenames merge.branchdesc merge.*.name merge.*.recursive
		remote.pushdefault remote.*.url remote.*.pushurl remote.*.fetch remote.*.push remote.*.promisor
		remote.*.partialclonefilter remote.*.tagopt remote.*.mirror remote.*.prune remote.*.prunetags remote.*.skipdefaultupdate
		remote.*.skipfetchall remote.*.followremotehead remote.*.proxy remote.*.proxyauthmethod remote.*.serveroption
		remote.*.gh-resolved
		submodule.recurse submodule.fetchjobs submodule.propagatebranches submodule.alternatelocation
		submodule.alternateerrorstrategy submodule.*.url submodule.*.path submodule.*.branch submodule.*.ignore
		submodule.active submodule.*.active submodule.*.fetchrecursesubmodules submodule.*.shallow
		extensions.objectformat extensions.compatobjectformat extensions.worktreeconfig extensions.partialclone
		extensions.preciousobjects extensions.refstorage extensions.noop extensions.noop-v1 extensions.relativeworktrees
		credential.username credential.usehttppath credential.interactive credential.*.username credential.*.usehttppath
		gpg.format protocol.version include.path includeif.*.path gui.wmstate gui.geometry
		worktree.guessremote worktree.userelativepaths safe.directory safe.barerepository
		gpg.*.allowedsignersfile gpg.*.revocationfile credential.provider credential.*.provider credential.validate
		am.keepcr stash.showpatch stash.showstat help.autocorrect versionsort.suffix versionsort.prereleasesuffix
		format.signoff interactive.singlekey notes.rewriteref mergetool.keepbackup mergetool.keeptemporaries
		mergetool.prompt mergetool.writetotemp commitgraph.changedpaths commitgraph.generationversion gui.gcwarning
		receive.autogc trailer.separators trailer.where trailer.ifexists trailer.ifmissing trailer.*.key trailer.*.where
		trailer.*.ifexists trailer.*.ifmissing`)
	// gitDataSections hold nothing but data in any of their keys. lfs's keys are read by git-lfs alone, which runs
	// only as a filter, and filters are blanked.
	gitDataSections = gitKeySet(`user author committer init color advice i18n gc pack index rerere pull push fetch
		transfer status lfs branch maintenance commit tag log column feature checkout clone blame grep rebase url http`)
	// gitBlankedKeys are programs (or what allows one) under a name a repo picks, and what they are blanked to.
	// A filter's "required" is turned off, so a blanked filter is no error. A submodule's update can be "!program".
	gitBlankedKeys = map[string]string{"filter.*.clean": "", "filter.*.smudge": "", "filter.*.process": "", "filter.*.required": "false",
		"diff.*.textconv": "", "diff.*.command": "", "merge.*.driver": "", "credential.*.helper": "", "gpg.*.program": "",
		"protocol.*.allow": "never", "submodule.*.update": "checkout"}
	// gitBlankedSections have programs in any key (an alias "!program", a pager per command): every key is blanked.
	// Neither is used by the commands run here (an alias can't stand for one of git's own commands; nothing is paged).
	gitBlankedSections = map[string]bool{"alias": true, "pager": true}
	// gitFixedKeys are those gitReadArgs blank whatever the repo says.
	gitFixedKeys = func() map[string]bool {
		keys := map[string]bool{}
		for i := 1; i < len(gitReadArgs); i++ {
			if gitReadArgs[i-1] == "-c" {
				key, _, _ := strings.Cut(gitReadArgs[i], "=")
				keys[strings.ToLower(key)] = true
			}
		}
		return keys
	}()
)

func gitKeySet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

// gitRepoConfig reads `config --list -z` (config with its includes followed: the repo's, its worktree's, and what
// the read hands git) and says the settings that blank every program it names under a name of its own, or, for a
// key a read doesn't understand, that key's rule ("remote.*.uploadpack"), and the repo isn't read. Reading config
// runs nothing. A subsection is kept as it is, "=" and all; sections and names are lowercase in git's list.
func gitRepoConfig(list []byte) (blanks [][2]string, refused string) {
	seen := map[string]bool{}
	for _, record := range strings.Split(string(list), "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		if key == "" {
			continue
		}
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		if first < 0 {
			return nil, "a key with no section"
		}
		section, name := strings.ToLower(key[:first]), strings.ToLower(key[last+1:])
		rule := section + "." + name
		if first != last {
			rule = section + ".*." + name
		}
		value, blanked := gitBlankedKeys[rule]
		switch {
		case blanked || gitBlankedSections[section]:
			if !seen[key] {
				seen[key] = true
				blanks = append(blanks, [2]string{key, value})
			}
		case gitDataKeys[rule] || gitDataSections[section] || gitFixedKeys[rule]:
		default:
			return nil, rule
		}
	}
	return blanks, ""
}

// gitStamp is where a repo's index and HEAD were when it was read.
type gitStamp struct{ index, head string }

func stampOf(gitDir string) gitStamp {
	one := func(name string) string {
		if info, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
			return strconv.FormatInt(info.ModTime().UnixNano(), 10) + ":" + strconv.FormatInt(info.Size(), 10)
		}
		return ""
	}
	return gitStamp{one("index"), one("HEAD")}
}

// readGitState reads dir's state with read, within one timeout. gitDir is the repo's own git folder ("" when
// dir is in no repo), for the cache to watch.
func readGitState(dir string, read gitReader, now time.Time) (state GitState, gitDir string) {
	state = GitState{Files: []GitFile{}, At: now.UnixMilli()}
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	failed := func(err error) GitState {
		state.Skipped = true
		switch {
		case errors.Is(err, errGitTooBig):
			state.Note = "There are too many changes to list, so the numbers are left out."
		case errors.Is(err, exec.ErrNotFound):
			state.Note = "Git isn't installed on this computer."
		case errors.Is(err, context.DeadlineExceeded):
			state.Note = "Git took too long to answer here."
		default:
			state.Note = "Git couldn't be read here."
		}
		return state
	}
	here := gitHere()
	attrSource := gitVersionAtLeast(here.version, 2, 40)
	call := gitCall{global: gitReadArgs, config: here.owner}
	revParse := []string{"rev-parse", "--show-toplevel", "--absolute-git-dir"}
	if attrSource {
		revParse = append(revParse, "--show-object-format")
	}
	out, status, err := read(ctx, dir, call, revParse...)
	if err != nil {
		return failed(err), ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if status != 0 || len(lines) < 2 || len(lines) > 3 { // a folder in no repo, a bare repo, or the inside of a .git folder
		// Git refuses a repo owned by another account (one the owner hasn't trusted) as if there were none: asked
		// again with every folder trusted, which rev-parse can be (it runs nothing), so the owner is told why.
		if status != 0 {
			trusting := gitCall{global: gitReadArgs, config: append(slices.Clip(here.owner), [2]string{"safe.directory", "*"})}
			if out, status, err := read(ctx, dir, trusting, "rev-parse", "--show-toplevel"); err == nil && status == 0 {
				state.Repo, state.Top, state.Skipped = true, filepath.FromSlash(strings.TrimSpace(string(out))), true
				state.Note = "This repo belongs to another account on this computer, so git won't read it here."
			}
		}
		return state, ""
	}
	state.Repo, state.Top, gitDir = true, filepath.FromSlash(lines[0]), filepath.FromSlash(lines[1])
	// Before 2.31 git can't be given a key in its environment, where an "=" in a filter's name can't split it, and
	// before 2.32 the owner's own config can't be left out: so nothing more is read.
	if !gitVersionAtLeast(here.version, 2, 32) {
		state.Skipped = true
		state.Note = "The git on this computer is older than 2.32, so the changes aren't read: update git to see them."
		return state, gitDir
	}
	// From here git reads files of the repo, which a filter its config names would run a program on: so the
	// config is read first, and the read stops at a key it doesn't know; the filters and text converters it names
	// are blanked; and, where git allows, no attributes are read from the working folder. If the config can't be
	// read, nothing else is. A key added between this and the reads below (a few milliseconds) isn't seen: a hire
	// could, but a hire already runs commands as the owner, so that is an accepted risk.
	list, listStatus, err := read(ctx, dir, call, "config", "--list", "-z")
	if err != nil || listStatus != 0 {
		return failed(err), gitDir
	}
	blanks, refused := gitRepoConfig(list)
	if refused != "" {
		state.Skipped = true
		state.Note = "The changes aren't read: this repo's git config has a setting this read doesn't know (" + refused + ")."
		return state, gitDir
	}
	call = gitCall{global: slices.Clip(gitReadArgs), config: append(slices.Clip(here.owner), blanks...)}
	if attrSource {
		format := "sha1"
		if len(lines) == 3 {
			format = lines[2]
		}
		tree, known := gitEmptyTrees[format]
		if !known {
			return failed(nil), gitDir
		}
		call.global = append(call.global, "--attr-source="+tree)
	}
	// Untracked files are listed as folders, not file by file, so a build folder is one line.
	out, status, err = read(ctx, dir, call, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=normal", "--ignore-submodules=dirty")
	if err != nil || status != 0 {
		return failed(err), gitDir
	}
	unborn := parseGitStatus(out, &state)
	state.remote = gitRemoteOf(list, state.Branch)
	if unborn { // no HEAD to count against
		return state, gitDir
	}
	out, status, err = read(ctx, dir, call, "diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=dirty", "HEAD", "--")
	if err != nil || status != 0 {
		state.Skipped = true
		state.Note = "The lines added and removed couldn't be counted."
		if errors.Is(err, errGitTooBig) {
			state.Note = "There are too many changes to count their lines."
		}
		return state, gitDir
	}
	parseGitNumstat(out, &state)
	return state, gitDir
}

// parseGitStatus fills state from `status --porcelain=v2 --branch -z`; it says whether the repo has no commit yet.
func parseGitStatus(out []byte, state *GitState) (unborn bool) {
	tokens := strings.Split(string(out), "\x00")
	add := func(path, code string) {
		state.Changed++
		if len(state.Files) < gitFilesShown {
			state.Files = append(state.Files, GitFile{Path: filepath.FromSlash(path), State: code})
		} else {
			state.More++
		}
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		fields := strings.Fields(token)
		switch {
		case strings.HasPrefix(token, "# branch.oid "):
			if id := strings.TrimPrefix(token, "# branch.oid "); id == "(initial)" {
				unborn = true
			} else {
				state.Head = id[:min(len(id), 8)]
			}
		case strings.HasPrefix(token, "# branch.head "):
			if name := strings.TrimPrefix(token, "# branch.head "); name == "(detached)" {
				state.Detached = true
			} else {
				state.Branch = name
			}
		case strings.HasPrefix(token, "# branch.upstream "):
			state.Upstream = strings.TrimPrefix(token, "# branch.upstream ")
		case strings.HasPrefix(token, "# branch.ab ") && len(fields) == 4:
			state.Ahead, _ = strconv.Atoi(strings.TrimPrefix(fields[2], "+"))
			state.Behind, _ = strconv.Atoi(strings.TrimPrefix(fields[3], "-"))
		case strings.HasPrefix(token, "1 "):
			if parts := strings.SplitN(token, " ", 9); len(parts) == 9 {
				add(parts[8], parts[1])
			}
		case strings.HasPrefix(token, "2 "): // a rename or copy: the path it came from is the next token
			if parts := strings.SplitN(token, " ", 10); len(parts) == 10 {
				add(parts[9], parts[1])
			}
			i++
		case strings.HasPrefix(token, "u "):
			if parts := strings.SplitN(token, " ", 11); len(parts) == 11 {
				add(parts[10], parts[1])
			}
		case strings.HasPrefix(token, "? "):
			add(token[2:], "??")
		}
	}
	return unborn
}

// parseGitNumstat adds `diff --numstat -z --no-renames` to state: the totals over every file, and each listed
// file's own. A binary file counts none ("-").
func parseGitNumstat(out []byte, state *GitState) {
	at := map[string]int{}
	for i, f := range state.Files {
		at[filepath.ToSlash(f.Path)] = i
	}
	for _, record := range strings.Split(string(out), "\x00") {
		parts := strings.SplitN(record, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		state.Added += added
		state.Removed += removed
		if i, listed := at[parts[2]]; listed {
			state.Files[i].Added, state.Files[i].Removed = added, removed
		}
	}
}

// gitEntry is what is kept of one folder.
type gitEntry struct {
	state   GitState
	has     bool
	at      time.Time
	event   float64 // the session's last event when it was read
	gitDir  string
	stamp   gitStamp
	running chan struct{} // closed when the read under way ends
}

// gitStates keeps the state of each folder, so a badge and a diff asking together cost one read.
type gitStates struct {
	mu    sync.Mutex
	byDir map[string]*gitEntry
	read  gitReader
	now   func() time.Time
}

func newGitStates() *gitStates {
	return &gitStates{byDir: map[string]*gitEntry{}, read: readGitOut, now: time.Now}
}

// gits is the one the app reads through.
var gits = newGitStates()

// fresh is whether e still stands for the folder. event is the session's last event now (0 for a caller with no
// session). Called with no lock held: it looks at files.
func (g *gitStates) fresh(e gitEntry, event float64) bool {
	age := g.now().Sub(e.at)
	switch {
	case !e.has:
		return false
	case age < gitStateEvery:
		return true
	case age >= gitStateMaxAge || event != e.event:
		return false
	}
	return e.gitDir == "" || stampOf(e.gitDir) == e.stamp
}

// get is folder's state: kept while fresh, read again otherwise, one read at a time per folder (others wait for it).
func (g *gitStates) get(folder string, event float64) GitState {
	if folder == "" {
		return GitState{Files: []GitFile{}, At: g.now().UnixMilli()}
	}
	key := physical(folder)
	for {
		g.mu.Lock()
		e := g.byDir[key]
		if e == nil {
			e = &gitEntry{}
			g.byDir[key] = e
			g.trim(key)
		}
		if wait := e.running; wait != nil {
			g.mu.Unlock()
			<-wait
			continue
		}
		seen := *e
		g.mu.Unlock()
		if g.fresh(seen, event) {
			return seen.state
		}
		g.mu.Lock()
		if e.running != nil { // another asker began a read first
			g.mu.Unlock()
			continue
		}
		done := make(chan struct{})
		e.running = done
		g.mu.Unlock()

		var state GitState
		var gitDir string
		var stamp gitStamp
		func() {
			// The answer is stored before the others are let go, so none of them reads again.
			stored := false
			defer func() {
				g.mu.Lock()
				if !stored {
					e.running = nil
				}
				g.mu.Unlock()
				close(done)
			}()
			state, gitDir = readGitState(key, g.read, g.now())
			stamp = stampOf(gitDir)
			g.mu.Lock()
			e.state, e.has, e.at, e.event, e.gitDir, e.stamp, e.running = state, true, g.now(), event, gitDir, stamp, nil
			stored = true
			g.mu.Unlock()
		}()
		return state
	}
}

// trim forgets the oldest folders past gitStatesKept, never keep's and never one being read. Lock held.
func (g *gitStates) trim(keep string) {
	for len(g.byDir) > gitStatesKept {
		oldest, found := "", false
		for key, e := range g.byDir {
			if key != keep && e.running == nil && (!found || e.at.Before(g.byDir[oldest].at)) {
				oldest, found = key, true
			}
		}
		if !found {
			return
		}
		delete(g.byDir, oldest)
	}
}

// deskGit answers GET /desk/git: the state of the folder session id works in: its project, which for a hire is
// its own worktree (the place startedIn gave it), not the repo it was hired from.
func (d *Deck) deskGit(id string) (int, any) {
	d.mu.Lock()
	s := d.sessions.Get(id)
	var folder string
	var event float64
	if s != nil {
		folder, event = cmpOr(s.Project, s.Folder), s.LastEventAt
	}
	d.mu.Unlock()
	if s == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	return http.StatusOK, withPR(gits.get(folder, event))
}

// gmGit answers GET /setup/gm/git for the GM (and the page and tools): ?hire= is a hire's registry id, read where it
// works (startedIn), and ?session= is a desk's id, as the window asks.
func (rt *router) gmGit(w http.ResponseWriter, r *request) {
	query := r.URL.Query()
	switch {
	case query.Get("hire") != "":
		h, found := hireByID(query.Get("hire"))
		if !found {
			writeStatus(w)(http.StatusNotFound, "There is no hire with that id.")
			return
		}
		writeJSON(w, http.StatusOK, withPR(gits.get(startedIn(h), 0)))
	case query.Get("session") != "":
		writeAnswer(w)(rt.deck.deskGit(query.Get("session")))
	default:
		writeStatus(w)(http.StatusBadRequest, "Send ?hire=<id> or ?session=<id>.")
	}
}

// Git state of the desk's folder (phoneroutes.go), in the office's own window on this computer, never a phone.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/desk/git", access: accessWindow,
		refusal: func(d *Deck) string { return d.shownOnlyInTheWindow("The git state") },
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeAnswer(w)(d.deskGit(session))
		}})
}
