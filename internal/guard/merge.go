package guard

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"
)

// The merge and spend groups (1.26's FS3, the owner's answer to Q10): on for hires, the General Manager, an outside
// app's MCP key and the night shift; the owner's own terminal has no guard. They came after the guard file had its
// six groups, so they aren't among DefaultGroups: the profiles (Profiled, Held) add each one a guard file doesn't
// name (WithGates). A file that names one keeps what it says, so the owner switches one off by naming it off.
//
// Merge refuses work landing on a base branch: main, master, release/* and the repository's default (what its
// remotes' HEAD names). It fails closed (RT126NS0a): it lists what it lets through, not what it refuses.
//   - A subcommand that never moves a branch, or moves the one it is on only by a commit made there (commit,
//     cherry-pick, revert, reset, am: the owner's decision 6), runs anywhere (gitKeeps).
//   - merge, pull, rebase and subtree move the branch they are on; a rebase with two words, or --root and one, the
//     branch it names last instead, as git reads it (RT126NS0aR2). update-ref, branch, checkout, switch, worktree
//     add, fetch, pull, stash branch and symbolic-ref make, move, rename or delete the branches they name, a base
//     branch's remote-tracking ref (refs/remotes/origin/main) among them; and checkout, switch and worktree add make a
//     missing local branch from a remote's (git's guess), unless the guard reads that it is here. Each is read with its
//     options as git 2.55's --help-all lists them (gitOptions): a long one by any start of its name that fits no other,
//     as git takes it, and one the guard doesn't know is refused.
//   - fast-import, filter-branch, replay, receive-pack, replace, update-ref --stdin, fetch --stdin, rebase
//     --update-refs and --exec move what the guard can't see: refused anywhere. So are a mirror remote, a remote's
//     push URL and a new default (gitRemote), a git config that writes an alias, autocorrect, include or a remote's
//     fetch refspec, mirror or push URL, and a fetch or pull in a repository whose config fetches into local branches
//     (storedFetch).
//   - Any other subcommand (an extension, an alias, a typo git's autocorrect runs) is taken to move the branch it is
//     on and every branch a word of it names; git lfs, but for migrate, moves none.
//   - A command that names a path in a git folder (cp x .git/refs/heads/main) is refused unless it only reads (N3).
//   - Config set in the line itself, and the editors, pagers and options git runs, are the guard core's (gitruns.go),
//     for every group: what gets past it runs nothing and moves no branch.
//
// The branch it is on is read from the repository at the call's folder, found as git finds it (PlaceOf), and only for
// a line that is that one git command, run where the call runs: after a cd, with -C or GIT_DIR, inside a wrapper, or
// beside another command that could move HEAD (anything but a reader, or a git command that leaves HEAD where it is),
// the guard can't tell, and refuses. A detached HEAD, a reftable repository and a git folder it can't read are taken
// as a base branch, and so is a name it can't read (a $x, a $( ), a pattern).
//
// Spend is a short list of commands that spend the owner's money. Its real gate is the owner's OK: the gate service
// asks for one (desktop/gates.go); the hook, which can't ask yet, refuses.

// The gate groups' names.
const (
	MergeGroup = "merge"
	SpendGroup = "spend"
)

// baseRule marks a rule that refuses only what lands on a base branch.
const baseRule = "@base"

// isBaseRule is whether rule is marked baseRule. mergePlace asks it of every rule for every command and redirection,
// so the text is looked at before the cached words (FXGUARDWIN).
func isBaseRule(rule Rule) bool {
	return strings.Contains(string(rule), baseRule) && slices.Contains(ruleWords(rule), baseRule)
}

// GateGroups are the merge and spend groups, both on.
func GateGroups() []Group {
	return []Group{
		{Name: MergeGroup, On: true, Commands: []Rule{
			// what lands on the branch it is on (a rebase: or one it names)
			"git merge|pull|rebase|subtree " + baseRule,
			// what moves a branch it names, or what the guard can't read
			"git update-ref|branch|checkout|switch|worktree|fetch|stash|symbolic-ref|config|fast-import " + baseRule,
			// any other git command: only what the guard reads as leaving a base branch alone
			"git " + baseRule,
		}},
		{Name: SpendGroup, On: true, Commands: []Rule{
			// Stripe's live mode moves real money; a server made at a host is billed. None of these is a deny rule
			// (an option, ... or @ keeps a rule the guard's alone, hiresettings.go): the gate service must see them to ask.
			"stripe --live|--api-key", "doctl ... create", "hcloud ... create", "linode-cli ... create", "vultr-cli ... create",
			"scw ... create", "openai ... api",
		}},
	}
}

// WithGates is groups with each gate group they don't name added, as GateGroups has it.
func WithGates(groups []Group) []Group {
	out := slices.Clone(groups)
	for _, gate := range GateGroups() {
		if !slices.ContainsFunc(groups, func(g Group) bool { return g.Name == gate.Name }) {
			out = append(out, gate)
		}
	}
	return out
}

// Place is where a line runs, as the merge group reads it: whether the repository there was found and read, the
// branch its HEAD is on (refs/heads/…; during a rebase, the branch being rebased; "" when it isn't one the guard can
// read for sure, a detached HEAD among them), and the branches its remotes name as their default.
type Place struct {
	Known    bool
	Branch   string
	Defaults []string

	// Set by check for one line: alone, the line is one command besides readers, git commands that leave HEAD where
	// it is and redirections that stay out of .git, in every reading; moved, it goes to another folder or repository
	// (cd, GIT_DIR).
	alone, moved bool

	// Set by PlaceOf: the call's folder, the git folder found and the common folder it shares (absolute and clean),
	// for the paths in a git folder (gitFolderPath) and the local branches (hasLocal); and whether the repository's
	// config fetches into local branches (storedFetch).
	cwd, gitDir, common string
	fetchesHeads        bool

	// Set by placeIn for a place unknown with a reason of its own to give: past 32 remotes (manyRemotesWhy).
	unknown string

	// Set by At: the folders above as gitFolderPath reads them, read once for a line.
	folders *placeFolders
}

// At is groups as they are checked for a line run at place. Groups never placed are checked as at a place unknown.
func At(groups []Group, place Place) []Group {
	place.folders = readFolders(place)
	out := slices.Clone(groups)
	for i := range out {
		p := place
		out[i].place = &p
	}
	return out
}

// PlaceOf is the repository at folder cwd (an absolute path), found as git finds it (setup.c): going up, a .git
// folder that is a git folder, a .git file's gitdir, or a folder that is a git folder itself; a .git folder that isn't
// one is passed by, as git passes it. It is read with ReadFile. Anything the guard can't find or read for sure is a
// place unknown.
func PlaceOf(cwd string) Place {
	if cwd == "" || !filepath.IsAbs(cwd) {
		return Place{}
	}
	p := placeOf(filepath.Clean(cwd))
	if p.Known {
		p.cwd = filepath.Clean(cwd)
	}
	return p
}

func placeOf(cwd string) Place {
	for dir, i := filepath.Clean(cwd), 0; i < 128; i++ {
		dotGit := filepath.Join(dir, ".git")
		info, err := os.Lstat(dotGit)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return Place{}
		case info.IsDir():
			switch gitFolder(dotGit) {
			case isGitFolder:
				return placeIn(dotGit)
			case unsureGitFolder:
				return Place{}
			}
		case info.Mode().IsRegular():
			data, err := ReadFile(dotGit)
			line := strings.TrimSpace(string(data))
			if err != nil || !strings.HasPrefix(line, "gitdir: ") {
				return Place{}
			}
			gitdir := strings.TrimPrefix(line, "gitdir: ")
			if !filepath.IsAbs(gitdir) {
				gitdir = filepath.Join(dir, gitdir)
			}
			if gitFolder(gitdir) != isGitFolder {
				return Place{} // git stops here too
			}
			return placeIn(gitdir)
		default:
			return Place{} // a link or anything else
		}
		switch gitFolder(dir) { // a bare repository, or a folder inside .git
		case isGitFolder:
			return placeIn(dir)
		case unsureGitFolder:
			return Place{}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return Place{}
}

const (
	notGitFolder = iota
	isGitFolder
	unsureGitFolder
)

// gitFolder is whether git takes dir as a git folder (is_git_directory in git's setup.c): a HEAD that names a ref
// under refs/ or a commit, and objects/ and refs/ folders in the common folder a worktree's commondir names. What
// the guard can't read for sure (a HEAD that is a link, a folder it can't open) is unsure.
func gitFolder(dir string) int {
	if info, err := os.Lstat(filepath.Join(dir, "HEAD")); errors.Is(err, fs.ErrNotExist) || err == nil && info.IsDir() {
		return notGitFolder // git reads no ref from a folder either
	}
	head, err := ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		return unsureGitFolder
	}
	if ref, ok := strings.CutPrefix(string(head), "ref:"); ok {
		if !strings.HasPrefix(strings.TrimLeft(ref, " \t\n\v\f\r"), "refs/") {
			return notGitFolder
		}
	} else if !hexStart(string(head), 64) && !hexStart(string(head), 40) {
		return notGitFolder
	}
	common, ok := commonFolder(dir)
	if !ok {
		return unsureGitFolder
	}
	for _, name := range []string{"objects", "refs"} {
		info, err := os.Stat(filepath.Join(common, name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return notGitFolder
		case err != nil || !info.IsDir():
			return unsureGitFolder
		}
	}
	return isGitFolder
}

// hexStart is whether text starts with n hex digits.
func hexStart(text string, n int) bool {
	if len(text) < n {
		return false
	}
	for _, c := range []byte(text[:n]) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// commonFolder is the folder that holds a git folder's refs, objects and config: the one its commondir names (a
// worktree's), or itself; ok is false when commondir can't be read.
func commonFolder(gitdir string) (string, bool) {
	data, err := ReadFile(filepath.Join(gitdir, "commondir"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return gitdir, true
	case err != nil:
		return "", false
	}
	common := strings.TrimSpace(string(data))
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitdir, common)
	}
	return common, true
}

// placeIn reads the git folder gitdir: its HEAD, a rebase in progress, and its remotes' HEADs (in the common folder
// a worktree shares). A reftable repository (extensions.refStorage) keeps a stand-in HEAD and no ref files, and past
// 32 remotes a default may go unread: both are a place unknown.
func placeIn(gitdir string) Place {
	common, ok := commonFolder(gitdir)
	if !ok {
		return Place{}
	}
	config, err := ReadFile(filepath.Join(common, "config"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) || strings.Contains(Fold(string(config)), "refstorage") {
		return Place{}
	}
	head, err := ReadFile(filepath.Join(gitdir, "HEAD"))
	if err != nil {
		return Place{}
	}
	worktreeConfig, err := ReadFile(filepath.Join(gitdir, "config.worktree"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Place{}
	}
	p := Place{Known: true, gitDir: filepath.Clean(gitdir), common: filepath.Clean(common),
		fetchesHeads: storedFetch(string(config)) || storedFetch(string(worktreeConfig))}
	if ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref:"); ok {
		p.Branch = branchRef(ref)
	}
	for _, name := range []string{"rebase-merge/head-name", "rebase-apply/head-name"} {
		data, err := ReadFile(filepath.Join(gitdir, name))
		switch {
		case err == nil:
			p.Branch = branchRef(string(data)) // "detached HEAD" when a detached HEAD is rebased
		case !errors.Is(err, fs.ErrNotExist):
			return Place{}
		}
	}
	remotes := filepath.Join(common, "refs", "remotes")
	entries, err := os.ReadDir(remotes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Place{}
	}
	if len(entries) > 32 {
		return Place{unknown: manyRemotesWhy}
	}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			return Place{} // git follows it; the guard reads no link
		}
		if !e.IsDir() {
			continue
		}
		data, err := ReadFile(filepath.Join(remotes, e.Name(), "HEAD"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Place{}
		}
		if ref, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: refs/remotes/"+e.Name()+"/"); ok {
			p.Defaults = append(p.Defaults, ref)
		}
	}
	return p
}

// storedFetch is whether a repository's config (its text) makes a fetch write anything but remote-tracking branches
// and tags (N1): a remote's fetch refspec into refs/heads (or anywhere else), a mirror, or an include the guard
// doesn't follow. git also uses a remote's refspecs for a fetch that names its own (the opportunistic update), so
// then every fetch and pull there is refused.
func storedFetch(config string) bool {
	for _, line := range strings.Split(config, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		key, value = Fold(strings.TrimSpace(key)), Fold(strings.Trim(strings.TrimSpace(value), `"`))
		switch {
		case strings.HasPrefix(key, "[include"), key == "path" && ok:
			return true
		case key == "mirror" && value != "false" && value != "no" && value != "off" && value != "0":
			return true
		case key == "fetch" && ok:
			dst := value[strings.LastIndexByte(value, ':')+1:]
			if !strings.HasPrefix(dst, "refs/remotes/") && !strings.HasPrefix(dst, "refs/tags/") || strings.Contains(dst, "..") {
				return true
			}
		}
	}
	return false
}

// branchRef is ref (refs/heads/name) when it names a branch, or "".
func branchRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok && name != "" && name != ".invalid" {
		return ref
	}
	return ""
}

// lineReaders are commands that only read and print: beside one of them, a line is still that git command alone.
var lineReaders = map[string]bool{"cat": true, "head": true, "tail": true, "grep": true, "wc": true, "cut": true, "tr": true,
	"echo": true, "printf": true, "true": true}

// editorVars may stand before the git command of a line that is alone (GIT_EDITOR=true git rebase --continue): the
// guard core lets them through only as true, : or cat (gitruns.go), which run nothing and move no HEAD.
var editorVars = map[string]bool{"GIT_EDITOR": true, "GIT_SEQUENCE_EDITOR": true, "EDITOR": true, "VISUAL": true,
	"GIT_PAGER": true, "PAGER": true}

// headStays is whether words are a git command that leaves HEAD on the branch it is on (RT126NS0aR2's over-refusal
// 2): one of gitKeeps, or fetch, whose refspecs' targets the group reads as it reads any fetch. Beside one, a merge,
// pull or rebase still lands where the guard read.
func headStays(words []string) bool {
	if commandName(words[0]) != "git" {
		return false
	}
	sub, _ := gitSubcommand(words[1:])
	return gitKeeps[sub] || sub == "fetch"
}

// placeLine is p for one line, read as readings: whether it is one command alone, and whether it moves.
func placeLine(readings [][]simple, p Place) Place {
	p.alone, p.moved = true, false
	for _, cmds := range readings {
		others := 0
		for _, c := range cmds {
			words := unmark(stripSubs(c))
			if len(words) == 0 {
				continue
			}
			for _, w := range words {
				low := Fold(w)
				if strings.Contains(low, "git_dir") || strings.Contains(low, "git_work_tree") ||
					strings.Contains(low, "git_common_dir") || strings.Contains(low, "git_namespace") {
					p.moved = true
				}
			}
			switch {
			case words[0] == redirMark:
				if strings.Contains(Fold(strings.Join(words[1:], " ")), ".git") {
					p.alone = false
				}
				continue
			case words[0] == redirInMark:
				continue
			}
			for len(words) > 1 && isAssignment(words[0]) {
				if name, _, _ := strings.Cut(words[0], "="); !editorVars[name] {
					p.alone = false
				}
				words = words[1:]
			}
			name := commandName(words[0])
			if cdFamily[name] || name == "popd" || name == "pop-location" {
				p.moved = true
			}
			if !lineReaders[name] && !headStays(words) {
				others++
			}
		}
		if others != 1 {
			p.alone = false
		}
	}
	return p
}

// onBase is why the merge group's @base rule refuses git's args at p (nil: a place unknown), or "" when what the
// command moves isn't a base branch. depth above 0 is a command inside another's, run who knows where.
func onBase(p *Place, args []string, depth int) string {
	if p == nil {
		p = &Place{}
	}
	known := p.Known && !p.moved && depth == 0 && plainGitOptions(args)
	sub, rest := gitSubcommand(args)
	if sub == "" || gitKeeps[sub] {
		return ""
	}
	m := gitMoves(sub, rest)
	if m.why != "" {
		return m.why
	}
	for _, name := range m.named {
		if isHead(name) {
			m.current = true
		} else if isBase(name, p, known) {
			if !known && !isBase(name, &Place{}, true) {
				// Not main, master or release/*: only the default the guard couldn't read makes it one (FXGUARDWIN126 6e).
				if p.unknown != "" && !p.moved && depth == 0 && plainGitOptions(args) {
					return p.unknown // read where it runs, the place has a reason of its own (past 32 remotes)
				}
				// Run alone from the folder it runs in, the guard read that place and couldn't read a repository there:
				// telling it to run there would say nothing (RT126GUARDWIN126 N4).
				how := defaultHowWhy
				if !p.moved && depth == 0 && plainGitOptions(args) {
					how = defaultUnreadWhy
				}
				return "The guard can't tell whether " + shownBranch(name) + " is the repository's default branch, so it takes it as a base branch. " + how
			}
			return "This moves " + shownBranch(name) + ", " + baseWhy
		}
	}
	// A branch git makes from a remote's when there is none here (switch release/9: N2). Where the place is unknown,
	// only the fixed names count: a default the guard can't read is made from the remote-tracking branch, which it
	// refuses to move anywhere by that name.
	for _, name := range m.guessed {
		if isBase(name, p, true) && !(known && p.hasLocal(name)) {
			return "This makes " + shownBranch(name) + " from a remote's branch, " + baseWhy
		}
	}
	if m.stored && (!known || p.fetchesHeads) {
		return storedFetchWhy
	}
	if m.current && (!known || !p.alone) {
		if p.unknown != "" && !p.moved && depth == 0 && plainGitOptions(args) {
			return p.unknown // the place it runs in, read where it runs, has a reason of its own
		}
		return cantTellBranch
	}
	if m.current {
		if p.Branch == "" {
			return noBranchWhy
		}
		if isBase(p.Branch, p, true) {
			return "This lands on " + shownBranch(p.Branch) + ", " + baseWhy
		}
	}
	return ""
}

const (
	baseWhy        = "a base branch (main, master, release/* and the repository's default): what lands there is the owner's. Leave the work on its own branch for review."
	cantTellBranch = "The guard can't tell which branch this lands on, so it takes it as a base branch: run it alone, in the repository's folder, with no cd, -C or GIT_DIR."
	// manyRemotesWhy: the guard reads each remote's HEAD for the repository's defaults, and no more than 32 of them.
	manyRemotesWhy = "This repository has more than 32 remotes, more than the guard reads to learn each one's default branch, so it can't tell whether this lands on a base branch and takes it as one. Leave it to the owner."
	noBranchWhy    = "The guard can't read the branch this lands on (a detached HEAD, or a HEAD that names no branch), so it takes it as a base branch. Check out the work's own branch first."
	cantReadWhy    = "The guard can't read this git command in full (an option it doesn't know, or one that moves branches from input or rules it can't see), so it may move a base branch. Spell its options out as git lists them, or leave it to the owner."
	// configWhy: an alias, autocorrect or include in git's config runs what the guard never sees; a remote's fetch
	// refspec or mirror moves local branches on a later fetch, and its push URL makes a push land here.
	configWhy      = "An alias, autocorrect or include in git's config runs commands the guard never sees, and a remote's fetch refspec, mirror or push URL moves branches out of its sight. Leave it to the owner."
	storedFetchWhy = "This fetch would write local branches by refspecs stored in the repository's config (a mirror, or a fetch into refs/heads), or it runs where the guard can't read that config (after a cd, with -C or GIT_DIR, inside a wrapper). Run it alone in the repository's folder, or leave it to the owner."
	remoteWhy      = "This changes which branches a remote writes or which one the guard reads as the default (a mirror, a push URL, set-head, rename, remove). Leave it to the owner."
	// defaultHowWhy: the guard reads the repository's default only where the call runs, for a git command alone there.
	defaultHowWhy = "It reads the default only for a git command run alone from the repository's own folder: run it there, without -C, cd, GIT_DIR or a wrapper."
	// defaultUnreadWhy: the same, run there already, where the guard couldn't read the repository (none it can read, a
	// link in its git folder, a read error).
	defaultUnreadWhy = "It reads the default where the command runs, and couldn't read this repository there (it isn't one, or the guard can't read all of its git folder). Leave it to the owner."
)

func shownBranch(name string) string {
	if name == "" {
		return "a branch named at run time"
	}
	return strings.TrimPrefix(name, "refs/heads/")
}

// isHead is whether a name is HEAD, the branch a command is on.
func isHead(name string) bool {
	return name == "@" || strings.EqualFold(name, "HEAD")
}

// isBase is whether a branch's name (or a ref, refs/heads/main) is a base branch at p. A name the guard can't read
// (a pattern, -, @{-1}) is; where the repository isn't known, every name is. A remote-tracking ref of one
// (refs/remotes/origin/main, remotes/origin/release/9) and a remote's HEAD count too (N2): git makes a missing local
// branch from it, and reads the default from it.
func isBase(name string, p *Place, known bool) bool {
	n := Fold(strings.TrimPrefix(name, "+"))
	if n == "" || n == "-" || strings.Contains(n, "@{") || strings.ContainsAny(n, "$`*?[\\\x01\x04") || !known {
		return true
	}
	for strings.Contains(n, "//") {
		n = strings.ReplaceAll(n, "//", "/")
	}
	n = strings.TrimPrefix(n, "refs/")
	if tracked, ok := strings.CutPrefix(n, "remotes/"); ok {
		if _, branch, ok := strings.Cut(tracked, "/"); ok {
			if branch == "head" {
				return true
			}
			n = branch
		}
	}
	n = strings.TrimPrefix(n, "heads/")
	if n == "main" || n == "master" || strings.HasPrefix(n, "release/") {
		return true
	}
	return slices.ContainsFunc(p.Defaults, func(d string) bool { return Fold(d) == n })
}

// plainGitOptions is whether git's options before its subcommand leave it in the repository at the call's folder:
// none, or only ones that change no repository: -C . (the same folder), and -c and --config-env, which the guard core
// lets through only for keys that run nothing or as true, : or cat (gitruns.go). -C elsewhere, --git-dir, --work-tree
// and the rest may.
func plainGitOptions(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			return true
		case a == "-c" || a == "--config-env" || a == "-C" && i+1 < len(args) && (args[i+1] == "." || args[i+1] == "./"):
			i++
		case strings.HasPrefix(a, "--config-env="):
		case !oneOf("--no-pager|-P|-p|--paginate|--no-optional-locks|--no-advice", a):
			return false
		}
	}
	return true
}

// gitKeeps are the subcommands that never move a branch, or move the one they are on only by a commit made there
// (commit, cherry-pick, revert, reset, am: the owner's decision 6). They are git's own (builtins), which neither an
// alias nor a program on the exec path can stand in for.
var gitKeeps = map[string]bool{}

func init() {
	for _, sub := range strings.Fields(`status log show diff blame annotate grep shortlog describe name-rev rev-parse rev-list
		ls-files ls-tree ls-remote cat-file show-ref for-each-ref show-branch merge-base cherry range-diff whatchanged
		format-patch archive count-objects fsck verify-commit verify-tag verify-pack diff-tree diff-index diff-files
		check-ignore check-attr check-mailmap check-ref-format var version help reflog add rm mv restore apply clean notes
		tag gc prune prune-packed repack pack-refs maintenance commit-graph multi-pack-index hash-object mktree mktag
		write-tree read-tree commit-tree update-index checkout-index unpack-objects pack-objects index-pack merge-file
		merge-tree sparse-checkout rerere bundle init clone credential column bugreport diagnose interpret-trailers
		mailinfo mailsplit stripspace fetch-pack upload-pack upload-archive show-index patch-id get-tar-commit-id
		fmt-merge-msg refs commit cherry-pick revert reset am`) {
		gitKeeps[sub] = true
	}
}

// gitMove is what a git command moves: the branches it names, the branch it makes from a remote's when there is no
// local one of that name (guessed), whether the branch it is on, or why the guard can't tell.
type gitMove struct {
	named, guessed  []string
	current, stored bool // stored: a fetch, which uses the remotes' refspecs in config (storedFetch)
	why             string
}

// gitMoves is what git's subcommand sub moves, given the words after it. A subcommand not known here moves the branch
// it is on, and every branch a word of it names.
func gitMoves(sub string, rest []string) gitMove {
	if sub == "config" {
		// Read as git reads it, its legacy form taking options only before the key (readConfig, RT126NS0aR4 N1).
		c, ok := readConfig(rest)
		if !ok {
			return gitMove{why: cantReadWhy}
		}
		return gitConfig(c)
	}
	var o gitArgs
	if set, ok := gitOptions[sub]; ok {
		if sub == "worktree" {
			if len(rest) == 0 || !strings.EqualFold(rest[0], "add") {
				if len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
					return gitMove{why: cantReadWhy}
				}
				return gitMove{} // list, lock, move, prune, remove, repair, unlock
			}
			rest = rest[1:]
		}
		if o, ok = set.read(rest); !ok {
			return gitMove{why: cantReadWhy}
		}
	}
	switch sub {
	case "merge":
		return gitMove{current: !o.set["abort"] && !o.set["quit"]}
	case "pull":
		return gitMove{named: append(refspecTargets(after(o.pos, 1)), refspecTargets(o.values["refmap"])...), current: true, stored: true}
	case "fetch":
		if o.set["stdin"] {
			return gitMove{why: cantReadWhy}
		}
		return gitMove{named: append(refspecTargets(after(o.pos, 1)), refspecTargets(o.values["refmap"])...), stored: true}
	case "rebase":
		switch {
		case o.set["update-refs"]:
			return gitMove{why: "This rebase moves every branch that points into what it rebases, a base branch among them maybe. " +
				"Rebase without --update-refs."}
		case o.set["exec"]:
			return gitMove{why: "This rebase runs commands the guard doesn't read (--exec). Run them yourself, between commits."}
		case o.set["abort"] || o.set["quit"] || o.set["show-current-patch"]:
			return gitMove{}
		}
		// Read as git reads it (RT126NS0aR2's over-refusal 1): with two words, the second is the branch it checks out
		// and moves; with --root, its one word; otherwise the branch it is on, and the word is only the upstream.
		// More words than git takes is refused.
		switch {
		case len(o.pos) > 2 || len(o.pos) == 2 && o.set["root"]:
			return gitMove{why: cantReadWhy}
		case len(o.pos) == 2 || len(o.pos) == 1 && o.set["root"]:
			return gitMove{named: o.pos[len(o.pos)-1:]}
		}
		return gitMove{current: true}
	case "subtree":
		return gitMove{named: o.values["branch"], current: true}
	case "update-ref":
		switch {
		case o.set["stdin"]:
			return gitMove{why: cantReadWhy}
		case len(o.pos) == 0:
			return gitMove{}
		case isHead(o.pos[0]) && o.set["no-deref"] && !o.set["deref"]:
			return gitMove{} // HEAD itself, detached
		}
		return gitMove{named: o.pos[:1]}
	case "branch":
		switch {
		case o.set["delete"] || o.set["-D"] || o.set["copy"] || o.set["-C"]:
			return gitMove{named: o.pos}
		case o.set["move"] || o.set["-M"]:
			return gitMove{named: o.pos, current: len(o.pos) < 2} // with one name, the branch it is on is renamed
		case slices.ContainsFunc(branchLists, func(name string) bool { return o.set[name] }), len(o.pos) == 0:
			return gitMove{}
		}
		return gitMove{named: o.pos[:1]} // made, or with -f reset
	case "checkout":
		made := slices.Concat(o.values["-b"], o.values["-B"], o.values["orphan"])
		return gitMove{named: made, guessed: guessed(o, len(made) > 0, o.pos[:o.before])}
	case "switch":
		made := slices.Concat(o.values["create"], o.values["force-create"], o.values["orphan"])
		return gitMove{named: made, guessed: guessed(o, len(made) > 0, o.pos)}
	case "worktree":
		named := slices.Concat(o.values["-b"], o.values["-B"])
		if len(named) == 0 && len(o.pos) == 1 && !o.set["detach"] {
			// with no branch named, git makes one named as the new folder when there isn't one
			named = []string{filepath.Base(filepath.ToSlash(o.pos[0]))}
		}
		return gitMove{named: named, guessed: guessed(o, len(named) > 0 || o.set["orphan"], after(o.pos, 1))}
	case "symbolic-ref":
		switch {
		case o.set["delete"]:
			return gitMove{named: o.pos}
		case len(o.pos) >= 2:
			return gitMove{named: o.pos[:2]}
		}
		return gitMove{}
	case "stash":
		if len(rest) > 0 && strings.EqualFold(rest[0], "branch") {
			if len(rest) < 2 || strings.HasPrefix(rest[1], "-") {
				return gitMove{why: cantReadWhy}
			}
			return gitMove{named: rest[1:2]}
		}
		return gitMove{}
	case "push":
		// The push group's, which the owner may switch off: a push to a remote is then theirs to allow. One to a folder
		// (git push . x:main) moves a branch here, or in a repository the guard doesn't read.
		if slices.ContainsFunc(rest, func(w string) bool {
			return w == "." || w == ".." || strings.HasPrefix(w, "~") || strings.HasPrefix(w, "file:") ||
				slices.ContainsFunc([]string{"/", "./", "../", `\`, `.\`, `..\`}, func(p string) bool { return strings.HasPrefix(w, p) })
		}) {
			return gitMove{named: namesIn(rest)}
		}
		return gitMove{}
	case "remote":
		return gitRemote(rest)
	case "replace":
		// A replace ref makes a base branch read, check out and build as another commit, though it doesn't move
		// (RT126NS0aR2's should-fix 1): only listing runs.
		if len(rest) == 0 || len(rest) == 1 && oneOf("-l|--list", rest[0]) {
			return gitMove{}
		}
		return gitMove{why: cantReadWhy}
	case "lfs":
		// git-lfs moves branches only by migrate, which rewrites them; its other commands (track, pull, install…)
		// change files, hooks and its own store (over-refusal 4: a pattern given to track is no branch).
		if pos := positional("git", rest); len(pos) == 0 || !strings.EqualFold(pos[0], "migrate") {
			return gitMove{}
		}
	case "fast-import", "filter-branch", "replay", "receive-pack":
		return gitMove{why: cantReadWhy}
	}
	return gitMove{named: namesIn(rest), current: true}
}

// guessed is the word a checkout, switch or worktree add may make a local branch of, from a remote-tracking branch of
// that name, when none is here (git's guess, N2): the first of words, unless it makes a branch by name (made), detaches,
// or doesn't guess. With --track and no name, the branch is named after the remote's (origin/release/9 makes
// release/9).
func guessed(o gitArgs, made bool, words []string) []string {
	if made || o.set["detach"] || len(words) == 0 {
		return nil
	}
	if o.set["track"] {
		name := strings.TrimPrefix(strings.TrimPrefix(words[0], "refs/"), "remotes/")
		if _, branch, ok := strings.Cut(name, "/"); ok {
			return []string{branch}
		}
	}
	if guess, given := o.set["guess"]; given && !guess || words[0] == "-" || strings.Contains(words[0], "@{") || isHead(words[0]) {
		return nil // the branch before (-, @{-1}) is one that was here
	}
	return words[:1]
}

// gitRemote is what git remote moves (N1, and RT126NS0aR2's should-fix 3). A mirror writes every ref a fetch brings
// (refs/heads/main among them), and a push URL makes a later push land where it names; set-head (but by asking the
// remote, -a), rename and remove change which branch the guard reads as the default. Listing, show, get-url,
// set-url, set-branches, prune, update and add without a mirror move nothing here: a fetch writes their branches
// under refs/remotes, and a branch made from one is read as switch's (guessed).
func gitRemote(rest []string) gitMove {
	pos := positional("git", rest)
	if len(pos) == 0 {
		return gitMove{}
	}
	args := rest[slices.Index(rest, pos[0])+1:]
	has := func(long string, short string) bool {
		return slices.ContainsFunc(args, func(a string) bool {
			given, _, _ := strings.Cut(strings.TrimPrefix(a, "--"), "=")
			return short != "" && a == short || strings.HasPrefix(a, "--") && len(given) >= 2 && strings.HasPrefix(long, given)
		})
	}
	switch Fold(pos[0]) {
	case "add":
		if has("mirror", "") {
			return gitMove{why: remoteWhy}
		}
	case "set-url":
		if has("push", "") {
			return gitMove{why: remoteWhy}
		}
	case "set-head":
		if !has("auto", "-a") || has("delete", "-d") {
			return gitMove{why: remoteWhy}
		}
	case "rename", "remove", "rm":
		return gitMove{why: remoteWhy}
	case "update":
		return gitMove{stored: true}
	case "show", "get-url", "set-branches", "prune":
	default:
		return gitMove{why: cantReadWhy}
	}
	return gitMove{}
}

// branchLists are git branch's options that make it list, set config or print, never make a branch.
var branchLists = []string{"list", "show-current", "edit-description", "set-upstream-to", "set-upstream", "unset-upstream",
	"contains", "no-contains", "with", "without", "merged", "no-merged", "points-at"}

// gitConfig is what git config moves: nothing, unless it writes an alias, autocorrect or include, or opens an editor.
func gitConfig(c configCall) gitMove {
	if c.edit {
		return gitMove{why: configWhy}
	}
	keys := append(slices.Clone(c.unsets), c.sections...)
	for _, kv := range c.writes {
		keys = append(keys, kv[0])
	}
	for _, key := range keys {
		k := Fold(key)
		section := k[:strings.IndexByte(k+".", '.')]
		if k == "" || strings.ContainsAny(k, "$`*?[\\\x01\x04") || oneOf("alias|help|include|includeif", section) ||
			section == "remote" && oneOf("fetch|mirror|pushurl", k[strings.LastIndexByte(k, '.')+1:]) {
			return gitMove{why: configWhy}
		}
	}
	return gitMove{}
}

// after is words from i on.
func after(words []string, i int) []string {
	if i >= len(words) {
		return nil
	}
	return words[i:]
}

// refspecTargets are the local branches refspecs (src:dst) write.
func refspecTargets(refspecs []string) []string {
	var out []string
	for _, w := range refspecs {
		if colon := strings.LastIndexByte(w, ':'); colon >= 0 && colon < len(w)-1 {
			out = append(out, w[colon+1:])
		}
	}
	return out
}

// namesIn are the words of a command the guard doesn't know that may name a branch: each word, an option's value and a
// refspec's destination. An option that may mean every branch names one the guard can't read ("").
func namesIn(rest []string) []string {
	var out []string
	for _, a := range rest {
		if strings.HasPrefix(a, "-") {
			name, value, ok := strings.Cut(a, "=")
			if oneOf("--all|--branches|--everything|--mirror", name) {
				out = append(out, "")
			}
			if !ok {
				continue
			}
			a = value
		}
		out = append(out, a)
		if colon := strings.LastIndexByte(a, ':'); colon >= 0 {
			out = append(out, a[colon+1:])
		}
	}
	return out
}

// gitArgs are a subcommand's words, read with its options: its other words, the options given (by full name; false
// when the last was --no-), and their values.
type gitArgs struct {
	pos    []string
	before int // how many of pos came before a --
	set    map[string]bool
	values map[string][]string
}

// gitOptionSet is a subcommand's options: each one's full name (a one-letter one with no long name: -x), with what
// it takes (0 nothing, '=' a value, the next word or after =, '?' a value only after = or attached), and its letter.
type gitOptionSet struct {
	takes  map[string]byte
	letter map[byte]string
}

// gitOptionSpec reads a list of options: "name", "x,name" or "-x" for a letter alone, each ending in = or ? when it
// takes a value. -h and --help come with every subcommand.
func gitOptionSpec(spec string) gitOptionSet {
	s := gitOptionSet{takes: map[string]byte{}, letter: map[byte]string{}}
	for _, f := range strings.Fields(spec + " h,help help-all") {
		var takes byte
		if end := f[len(f)-1]; end == '=' || end == '?' {
			takes, f = end, f[:len(f)-1]
		}
		switch {
		case f[0] == '-':
			s.letter[f[1]] = f
		case len(f) > 2 && f[1] == ',':
			s.letter[f[0]], f = f[2:], f[2:]
		}
		s.takes[f] = takes
	}
	return s
}

// read is rest read as git's parse-options reads it: options anywhere until --, a long one by any start of its name
// (or of its --no- form) that fits no other, letters run together (-fB main, -Bmain). ok is false when a word is an
// option the guard doesn't know, or a start that fits more than one.
func (s gitOptionSet) read(rest []string) (g gitArgs, ok bool) { return s.parse(rest, false) }

// readHead is rest read as parse-options' STOP_AT_NON_OPTION reads it (git config's legacy form, N1 of RT126NS0aR4):
// options only up to the first other word, and every word from there on is one, - or not.
func (s gitOptionSet) readHead(rest []string) (g gitArgs, ok bool) { return s.parse(rest, true) }

// parse is read's and readHead's reading; head stops taking options at the first other word.
func (s gitOptionSet) parse(rest []string, head bool) (g gitArgs, ok bool) {
	g = gitArgs{set: map[string]bool{}, values: map[string][]string{}}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--" || a == "--end-of-options":
			g.before = len(g.pos)
			g.pos = append(g.pos, rest[i+1:]...)
			return g, true
		case strings.HasPrefix(a, "--"):
			given, value, hasValue := strings.Cut(a[2:], "=")
			name, negated, known := s.long(given)
			if !known {
				return g, false
			}
			g.set[name] = !negated
			if takes := s.takes[name]; !negated && (takes == '=' || takes == '?' && hasValue) {
				if !hasValue && i+1 < len(rest) {
					i++
					value = rest[i]
				}
				g.values[name] = append(g.values[name], value)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for k := 1; k < len(a); k++ {
				name, known := s.letter[a[k]]
				if !known {
					return g, false
				}
				g.set[name] = true
				takes := s.takes[name]
				if takes == 0 {
					continue
				}
				if value := a[k+1:]; value != "" || takes == '=' {
					if value == "" && i+1 < len(rest) {
						i++
						value = rest[i]
					}
					g.values[name] = append(g.values[name], value)
				}
				break
			}
		case head:
			g.pos = append(g.pos, rest[i:]...)
			g.before = len(g.pos)
			return g, true
		default:
			g.pos = append(g.pos, a)
		}
	}
	g.before = len(g.pos)
	return g, true
}

// long is the option a --given names: its full name, whether it is its --no- form, and whether exactly one fits.
func (s gitOptionSet) long(given string) (name string, negated, ok bool) {
	plain := func(n string) bool { _, ok := s.takes[n]; return ok && n != "" && n[0] != '-' }
	if plain(given) {
		return given, false, true
	}
	cut, isNo := strings.CutPrefix(given, "no-")
	if isNo && plain(cut) {
		return cut, true, true
	}
	if given == "" || strings.HasPrefix("no-", given) {
		return "", false, false // --n, --no, --no-: the start of every --no- form
	}
	fits := 0
	for n := range s.takes {
		if !plain(n) {
			continue
		}
		if strings.HasPrefix(n, given) {
			fits, name, negated = fits+1, n, false
		}
		if isNo && strings.HasPrefix(n, cut) {
			fits, name, negated = fits+1, n, true
		}
	}
	return name, negated, fits == 1
}

// gitOptions are the options of the subcommands the merge group reads in full, as git 2.55's `--help-all` lists them.
var gitOptions = map[string]gitOptionSet{
	"merge": gitOptionSpec(`-n stat summary compact-summary log? squash commit e,edit cleanup= ff ff-only rerere-autoupdate
		verify-signatures s,strategy= X,strategy-option= m,message= F,file= into-name= v,verbose q,quiet abort quit continue
		allow-unrelated-histories progress S,gpg-sign? autostash overwrite-ignore signoff no-verify verify`),
	"pull": gitOptionSpec(`v,verbose q,quiet progress recurse-submodules? r,rebase? -n stat summary compact-summary log? signoff?
		squash commit edit cleanup= ff ff-only verify verify-signatures autostash s,strategy= X,strategy-option= S,gpg-sign?
		allow-unrelated-histories all a,append upload-pack= f,force t,tags p,prune j,jobs? dry-run k,keep depth= shallow-since=
		shallow-exclude= deepen= unshallow update-shallow refmap= o,server-option= 4,ipv4 6,ipv6 negotiation-restrict=
		negotiation-tip= negotiation-include= show-forced-updates set-upstream`),
	"fetch": gitOptionSpec(`v,verbose q,quiet all set-upstream a,append atomic upload-pack= f,force m,multiple t,tags -n j,jobs=
		prefetch p,prune P,prune-tags recurse-submodules? dry-run porcelain write-fetch-head k,keep u,update-head-ok progress
		depth= shallow-since= shallow-exclude= deepen= unshallow refetch update-shallow refmap= o,server-option= 4,ipv4 6,ipv6
		negotiation-restrict= negotiation-tip= negotiation-include= negotiate-only filter= auto-maintenance auto-gc
		show-forced-updates write-commit-graph stdin submodule-prefix= recurse-submodules-default=`),
	"rebase": gitOptionSpec(`onto= keep-base verify no-verify q,quiet v,verbose n,no-stat stat trailer= signoff
		committer-date-is-author-date ignore-date reset-author-date -C= ignore-whitespace whitespace= f,force-rebase no-ff ff
		continue skip abort quit edit-todo show-current-patch apply m,merge i,interactive p,preserve-merges k,keep-empty
		rerere-autoupdate allow-empty-message empty= autosquash update-refs S,gpg-sign? autostash x,exec= r,rebase-merges?
		fork-point s,strategy= X,strategy-option= root reschedule-failed-exec reapply-cherry-picks`),
	"subtree": gitOptionSpec(`q,quiet d,debug P,prefix= annotate= b,branch= ignore-joins onto= rejoin squash m,message=
		S,gpg-sign?`),
	"update-ref": gitOptionSpec(`-m= -d no-deref deref -z stdin create-reflog 0,batch-updates`),
	"branch": gitOptionSpec(`v,verbose q,quiet t,track? u,set-upstream-to= set-upstream unset-upstream color? r,remotes contains=
		no-contains= with= without= abbrev? a,all d,delete -D m,move -M omit-empty c,copy -C l,list show-current create-reflog
		edit-description f,force merged= no-merged= column? sort= points-at= i,ignore-case recurse-submodules format=`),
	"checkout": gitOptionSpec(`-b= -B= -l guess overlay auto-advance q,quiet recurse-submodules? progress m,merge conflict=
		d,detach t,track? f,force orphan= overwrite-ignore ignore-other-worktrees 2,ours 3,theirs p,patch U,unified=
		inter-hunk-context= ignore-skip-worktree-bits pathspec-from-file= pathspec-file-nul`),
	"switch": gitOptionSpec(`c,create= C,force-create= guess discard-changes q,quiet recurse-submodules? progress m,merge
		conflict= d,detach t,track? f,force orphan= overwrite-ignore ignore-other-worktrees`),
	"worktree": gitOptionSpec(`f,force -b= -B= orphan d,detach checkout lock reason= q,quiet track guess-remote
		relative-paths`), // worktree add's
	"symbolic-ref": gitOptionSpec(`q,quiet d,delete short recurse -m=`),
	"config": gitOptionSpec(`global system local worktree f,file= blob= get get-all get-regexp get-urlmatch replace-all add
		unset unset-all rename-section remove-section l,list e,edit get-color get-colorbool z,null name-only show-origin
		show-scope show-names t,type= bool int bool-or-int bool-or-str path expiry-date default= comment= fixed-value includes
		all regexp url= value= append`),
}

// hasLocal is whether the place's repository has the local branch name (main, or refs/heads/main): a ref file, or a
// line of packed-refs. A name git wouldn't take as a ref, or anything the guard can't read, is none.
func (p *Place) hasLocal(name string) bool {
	if p.common == "" {
		return false
	}
	n := strings.TrimPrefix(strings.TrimPrefix(name, "refs/"), "heads/")
	if n == "" || strings.Contains(n, "..") || strings.ContainsAny(n, "\\:?*[~^ \t") || strings.HasPrefix(n, "/") ||
		strings.HasSuffix(n, "/") || strings.Contains(n, "//") || n == "-" {
		return false
	}
	if info, err := os.Lstat(filepath.Join(p.common, "refs", "heads", filepath.FromSlash(n))); err == nil {
		return info.Mode().IsRegular()
	}
	packed, err := ReadFile(filepath.Join(p.common, "packed-refs"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(packed), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == "refs/heads/"+n {
			return true
		}
	}
	return false
}

// N3 (RT126NS0aR2): a ref is a file. A command that writes in a git folder (cp, mv, a redirection, tee, ln, git
// itself given the path) moves a base branch with no git command the group reads, and reaches packed-refs, HEAD,
// config and hooks/ too. So, failing closed as guardReaders does, a command that names a path in a git folder, by a
// word or by an output redirection, is refused unless it is one of gitFolderReaders. A path is in a git folder when a
// part of it is .git (any case, as Windows also spells it: .git., GIT~1, .git::$DATA), when a pattern could fill one
// in, or when it is in the place's own git folder or the common folder it shares, by its absolute path or from the
// call's folder (a bare repository, a worktree's gitdir: target, a --separate-git-dir).
//
// Edit and Write, and a script the hire writes and runs, don't go through this hook (guard.go's known list).

// gitFolderRule names the refusal.
const (
	gitFolderRule = "a path in a git folder"
	gitFolderWhy  = "Writing in a git folder (.git: its refs, packed-refs, HEAD, config, hooks) moves branches and sets commands out of the guard's sight. Use git's own commands for it; only cat, head, tail, less, grep, ls, stat, wc, diff, cmp, echo and printf may name a path there."
)

// gitFolderReaders only read or print the paths they're given.
var gitFolderReaders = map[string]bool{"cat": true, "head": true, "tail": true, "less": true, "grep": true, "egrep": true,
	"fgrep": true, "ls": true, "stat": true, "wc": true, "diff": true, "cmp": true, "echo": true, "printf": true}

// mergePlace is the place of the merge group in groups when it is on (nil, and false, when it isn't).
func mergePlace(groups []Group) (*Place, bool) {
	for _, g := range groups {
		if g.On && slices.ContainsFunc(g.Commands, isBaseRule) {
			if g.place == nil {
				return &Place{}, true
			}
			return g.place, true
		}
	}
	return nil, false
}

// gitFolderRefusal is the merge group's refusal of a command (name and its words; "" for an output redirection's
// target) that names a path in a git folder, or nil.
func gitFolderRefusal(groups []Group, name string, words []string) *Refusal {
	p, on := mergePlace(groups)
	if !on || name != "" && gitFolderReaders[name] && !(name == "less" && lessWrites(words[1:])) {
		return nil
	}
	var data, code []bool // a git message's text names no path (dataArgs); an interpreter's code is no path glob (codeArgs)
	if name != "" && name != "=" && len(words) > 1 {
		data = append([]bool{false}, dataArgs(name, unmark(words[1:]))...)
		code = append([]bool{false}, codeArgs(name, unmark(words[1:]))...)
	}
	for i, w := range words {
		if i < len(data) && data[i] {
			continue
		}
		if i < len(code) && code[i] && !strings.HasPrefix(w, unreadable) {
			if codeNamesGitFolder(w, p) {
				return &Refusal{Group: MergeGroup, Rule: gitFolderRule, Command: strings.Join(unmark(words), " "), Why: gitFolderWhy}
			}
			continue
		}
		if gitFolderPath(strings.TrimPrefix(w, unreadable), p) {
			return &Refusal{Group: MergeGroup, Rule: gitFolderRule, Command: strings.Join(unmark(words), " "), Why: gitFolderWhy}
		}
	}
	return nil
}

// codeArgs says which of an interpreter's arguments is the code it is given on the line: python's -c (and py's,
// pypy's), node's and bun's -e, -p, --eval and --print. Code is text in a language of its own, not a path, so its ( ) [ ]
// * = aren't read as a pattern that could fill in .git (FXGUARDWIN126 6b: …; print(t) read as …/.gi and t). A path it
// names written out still is (codeNamesGitFolder). An option the guard doesn't know before the code leaves the code
// unmarked: it is read as any word.
func codeArgs(name string, args []string) []bool {
	code := make([]bool, len(args))
	switch {
	case name == "python" || name == "python2" || name == "python3" || name == "py" || name == "pypy" || name == "pypy3" ||
		strings.HasPrefix(name, "python3.") || strings.HasPrefix(name, "python2."):
		for i := 0; i < len(args); i++ {
			a := args[i]
			if a == "-" || a == "--" || !strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--") {
				return code // the script, standard input, or a long option it reads on its own
			}
			for k := 1; k < len(a); k++ {
				switch c := a[k]; {
				case c == 'c':
					if k < len(a)-1 {
						code[i] = true // -cCODE
					} else if i+1 < len(args) {
						code[i+1] = true
					}
					return code
				case c == 'W' || c == 'X':
					// A warning filter or an -X option: the rest of the word, or the next one.
					if k == len(a)-1 {
						i++
					}
					k = len(a)
				case strings.IndexByte("bBdEhiIOPqRsSuvx", c) < 0:
					return code // -m runs a module; anything else isn't read
				}
			}
		}
	case name == "node" || name == "nodejs" || name == "bun":
		for i := 0; i < len(args); i++ {
			switch a := args[i]; {
			case a == "-e" || a == "-p" || a == "-pe" || a == "-ep" || a == "--eval" || a == "--print":
				if i+1 < len(args) {
					code[i+1] = true
				}
				return code
			case strings.HasPrefix(a, "--eval=") || strings.HasPrefix(a, "--print="):
				code[i] = true
				return code
			case a == "-r" || a == "--require" || a == "--import" || a == "-C" || a == "--conditions" || a == "--input-type":
				i++
			case !strings.HasPrefix(a, "-") || a == "--":
				return code // the script
			}
		}
	}
	return code
}

// codeNamesGitFolder is whether an interpreter's code names a path in a git folder, written out: each run of the
// letters a path is written with (letters, digits, . _ - ~ $ : / \) is read as a path, its patterns not filled in.
// open(".git/HEAD", "w") names .git/HEAD.
func codeNamesGitFolder(code string, p *Place) bool {
	for _, token := range strings.FieldsFunc(code, func(r rune) bool {
		return r < utf8.RuneSelf && !isAlnum(r) && strings.IndexRune("._-~$:/\\", r) < 0
	}) {
		if gitFolderNamed(token, p, false) {
			return true
		}
	}
	return false
}

// lessWrites is whether less's options write a file: -o and -O (--log-file, --LOG-FILE).
func lessWrites(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "oO") ||
			strings.HasPrefix(Fold(a), "--log-file")
	})
}

// gitFolderPath is whether a word (or what follows an = in it: of=…, --output=…) names a path in a git folder at p.
func gitFolderPath(word string, p *Place) bool { return gitFolderNamed(word, p, true) }

// gitFolderNamed is gitFolderPath, its patterns read (globs) or not.
func gitFolderNamed(word string, p *Place, globs bool) bool {
	low := Fold(strings.ReplaceAll(word, `\`, "/"))
	var folders *placeFolders
	var wide []string
	candidates := []string{low}
	for i := 0; i < len(low); i++ {
		if low[i] == '=' {
			candidates = append(candidates, low[i+1:])
		}
	}
	for _, c := range candidates {
		for _, part := range strings.Split(c, "/") {
			for _, s := range strings.Split(part, ":") {
				if s = strings.TrimRight(s, ". "); s == ".git" || gitShortName.MatchString(s) {
					return true
				}
			}
		}
		if globs && isWild(c, nil) {
			sk := skeletonOf(c, nil)
			for _, tg := range gitFolderTargets {
				if couldFill(&sk, &tg) {
					return true
				}
			}
		}
		if p == nil || p.gitDir == "" || c == "" || strings.HasPrefix(c, "-") || strings.HasPrefix(c, "~") {
			continue
		}
		if folders == nil {
			if folders = p.folders; folders == nil {
				folders = readFolders(*p)
			}
			if windowsPaths {
				wide = wideNames(word, nil)
			}
		}
		if inFolders(c, folders, windowsPaths, wide) {
			return true
		}
	}
	return false
}

// windowsPaths says the computer's paths are Windows' (inFolders). Unlike onWindows, which is how a line is read and
// which tests set, it is the computer the guard runs on.
var windowsPaths = runtime.GOOS == "windows"

// placeFolders is a place's folders as gitFolderPath compares a path with them, read once for a line (At) rather
// than for every word (FXGUARDWINR1, P1): the call's folder and the git folders, folded, and as Windows reads them.
type placeFolders struct {
	cwd  string   // folded; "" when it isn't known
	dirs []string // the git folder and the common folder, folded

	here   *winAbs  // the call's folder as Windows reads it (nil when it isn't known)
	win    []winAbs // dirs as Windows reads them
	winBad bool     // a folder Windows' reading can't place: every path counts as in it
	wide   []string // the names of the folders' parts that hold a letter outside ASCII (wideNames)
}

// readFolders reads p's folders (placeFolders).
func readFolders(p Place) *placeFolders {
	f := &placeFolders{cwd: Fold(p.cwd), dirs: []string{Fold(p.gitDir), Fold(p.common)}}
	if here, ok := winPath(f.cwd, nil); ok && !here.anyOnDrive && !here.anywhere {
		f.here = &here
	}
	for _, d := range f.dirs {
		dir, ok := winPath(d, nil)
		if !ok || dir.anyOnDrive || dir.anywhere {
			f.winBad = true
			continue
		}
		f.win = append(f.win, dir)
	}
	f.wide = wideNames(p.common, wideNames(p.gitDir, wideNames(p.cwd, nil)))
	return f
}

// wideNames appends to out the names of raw's parts (\, / or : between them) that hold a letter outside ASCII,
// folded and without their trailing dots and spaces, as winPath reads a part. They are read from the raw path because
// Fold makes some such letters ASCII (ſ is s, the Kelvin sign k, ß ss, ﬁ fi), and Windows makes a short name from the
// raw letters: Fold("ſtore") is store, whose short name winShortFor reads as STORE~1, but Windows may write TORE~1
// (FXGUARDWINF1).
func wideNames(raw string, out []string) []string {
	if !hasWide(raw) {
		return out
	}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\\' || r == '/' || r == ':' }) {
		if hasWide(part) {
			out = append(out, strings.TrimRight(Fold(part), ". "))
		}
	}
	return out
}

// hasWide is whether text holds a byte outside ASCII.
func hasWide(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return true
		}
	}
	return false
}

// inFolders is whether a path (folded, / between its parts) is one of f's git folders or below one, read from the
// call's folder (when it isn't known, a relative path is in none). On a Mac or Linux it is read as it was. On
// Windows (windows) the place's folders are Windows paths, with \ between their parts: before FXGUARDWIN, a path
// cleaned there (C:\x\store\HEAD) was compared with store+"/" and never matched, so cp x ../003/store/HEAD wrote a
// worktree's git folder. There a path is read as Windows reads it (winPath): a drive, a drive alone, a / alone,
// \\?\, \\.\ and \??\, UNC, Git Bash's /c/, a part's trailing dots and spaces and its :stream; and a short name
// (STORE~1, RUNNER~1) may be any one part, on either side. What Windows' reading can't place (a device namespace
// such as \\?\GLOBALROOT\, or a git folder it can't read) counts as in the folder. wide are the names of the path's
// parts that hold a letter outside ASCII (wideNames), for its short names.
func inFolders(p string, f *placeFolders, windows bool, wide []string) bool {
	if !windows {
		if !path.IsAbs(p) {
			if f.cwd == "" {
				return false
			}
			p = path.Join(f.cwd, p)
		}
		p = path.Clean(p)
		for _, d := range f.dirs {
			if p == d || strings.HasPrefix(p, d+"/") {
				return true
			}
		}
		return false
	}
	if f.winBad {
		return true
	}
	if len(f.wide) > 0 {
		wide = slices.Concat(wide, f.wide)
	}
	var readings [2]winAbs
	for _, abs := range winReadings(p, f.here, readings[:0]) {
		if abs.anywhere {
			return true
		}
		for _, dir := range f.win {
			if abs.anyOnDrive && abs.vol == dir.vol || !abs.anyOnDrive && winBelow(abs, dir, wide) {
				return true
			}
		}
	}
	return false
}

// winAbs is a Windows path read to its volume (c: or //server/share, folded) and its parts, cleaned. anyOnDrive is a
// drive alone (d:x) on a drive the call's folder isn't on: that drive's own folder isn't known, so it may be anywhere
// on it. anywhere is a device namespace the guard can't place (\\?\GLOBALROOT\…, \\?\Volume{…}\…): it may be any
// folder at all.
type winAbs struct {
	vol        string
	parts      []string
	anyOnDrive bool
	anywhere   bool
}

// winReadings appends to out each way Windows may read a path from the call's folder here: Git Bash's /c/x is C:\x,
// and to cmd and PowerShell \c\x on the call's drive, so both are taken.
func winReadings(p string, here *winAbs, out []winAbs) []winAbs {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) >= 3 && p[0] == '/' && isDriveLetter(p[1]) && p[2] == '/' || len(p) == 2 && p[0] == '/' && isDriveLetter(p[1]) {
		if a, ok := winPath(p[1:2]+":/"+strings.TrimPrefix(p[2:], "/"), here); ok {
			out = append(out, a)
		}
	}
	if a, ok := winPath(p, here); ok {
		out = append(out, a)
	}
	return out
}

// winDevices are Win32's device spellings: \\?\ and \\.\, and \??\, the NT one Win32 passes through as it is.
var winDevices = []string{"//?/", "//./", "/??/"}

// winPath reads one Windows path from the call's folder here (winAbs; nil when it isn't known); ok is false when it
// can't be read (relative, with no folder to read it from).
func winPath(p string, here *winAbs) (winAbs, bool) {
	p = strings.ReplaceAll(p, `\`, "/")
	// \\?\C:\x, \\.\C:\x and \??\C:\x are C:\x, and \\?\UNC\server\share is \\server\share. Anything else after
	// one (GLOBALROOT, Volume{…}, HarddiskVolume3, GLOBAL??) is a namespace the guard can't place: it fails closed.
	for _, device := range winDevices {
		if rest, ok := strings.CutPrefix(p, device); ok {
			switch {
			case strings.HasPrefix(rest, "unc/"):
				p = "//" + rest[len("unc/"):]
			case len(rest) >= 3 && isDriveLetter(rest[0]) && rest[1] == ':' && rest[2] == '/':
				p = rest
			default:
				return winAbs{anywhere: true}, true
			}
			break
		}
	}
	var vol, rest string
	var base []string
	switch {
	case len(p) >= 2 && isDriveLetter(p[0]) && p[1] == ':':
		vol, rest = p[:2], p[2:]
		if !strings.HasPrefix(rest, "/") {
			// A drive alone: from that drive's own folder, the call's when it is on that drive.
			if here == nil || here.vol != vol {
				return winAbs{vol: vol, anyOnDrive: true}, true
			}
			base = here.parts
		}
	case strings.HasPrefix(p, "//"):
		f := strings.SplitN(p[2:], "/", 3)
		if len(f) < 2 {
			return winAbs{}, false
		}
		vol = "//" + f[0] + "/" + f[1]
		if len(f) == 3 {
			rest = f[2]
		}
	case here != nil:
		vol, rest = here.vol, p
		if !strings.HasPrefix(p, "/") {
			base = here.parts
		}
	default:
		return winAbs{}, false
	}
	parts := slices.Clone(base)
	for _, part := range strings.Split(rest, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		// Windows drops a part's :stream (store::$INDEX_ALLOCATION) and its trailing dots and spaces (store.).
		part, _, _ = strings.Cut(part, ":")
		if part = strings.TrimRight(part, ". "); part != "" {
			parts = append(parts, part)
		}
	}
	return winAbs{vol: vol, parts: parts}, true
}

// winBelow is whether a is dir or below it: the same volume, and each of dir's parts the same, or one the other's
// short name (winShortOf, with the names of either path's parts that hold a letter outside ASCII, wide), on either
// side.
func winBelow(a, dir winAbs, wide []string) bool {
	if a.vol != dir.vol || len(a.parts) < len(dir.parts) {
		return false
	}
	for i, d := range dir.parts {
		if a.parts[i] != d && !winShortOf(a.parts[i], d, wide) && !winShortOf(d, a.parts[i], wide) {
			return false
		}
	}
	return true
}

// winShortName is a Windows short name (8.3), folded: up to six letters, ~ and a number, and up to three letters
// of an extension (store~1, src~1.git, runner~1; and Windows' hashed form, two letters and four hex digits: st1a2b~1).
// Letters, not bytes: a short name may hold a letter outside ASCII (hé1a2b~1).
var winShortName = regexp.MustCompile(`^([^.~]{1,6})~[0-9]{1,6}(?:\.([^.]{1,3}))?$`)

// winShortFor is whether short could be long's short name, read as Windows makes one: long's stem (before its last
// dot, its dots and spaces dropped) starts with short's letters, or with their first two when they are six (the
// hashed form), and its extension (its spaces dropped) with short's. A letter Windows can't keep in a short name it
// writes as _, which is taken as any letter. HEAD~1 is no short name of .git (its stem, git, doesn't start with head).
// It never says no when it can't tell: a long name with a letter outside ASCII (Łukasz, Hélène) takes any short name,
// since Windows writes such a letter as _, drops it or writes one like it (Ł as L) by the computer's code page,
// which the guard can't know (FXGUARDWINR1, B2).
func winShortFor(short, long string) bool {
	if long == short || !strings.Contains(short, "~") {
		return false
	}
	m := winShortName.FindStringSubmatch(short)
	if m == nil {
		return false
	}
	for i := 0; i < len(long); i++ {
		if long[i] >= utf8.RuneSelf {
			return true
		}
	}
	stem, ext := strings.TrimLeft(long, "."), ""
	if i := strings.LastIndexByte(stem, '.'); i > 0 {
		stem, ext = stem[:i], stem[i+1:]
	}
	stem, ext = strings.NewReplacer(".", "", " ", "").Replace(stem), strings.ReplaceAll(ext, " ", "")
	starts := func(text, start string) bool {
		if len(text) < len(start) {
			return false
		}
		for i := 0; i < len(start); i++ {
			if start[i] != '_' && start[i] != text[i] {
				return false
			}
		}
		return true
	}
	hashed := ""
	if letters := []rune(m[1]); len(letters) == 6 {
		hashed = string(letters[:2])
	}
	return (starts(stem, m[1]) || hashed != "" && starts(stem, hashed)) && starts(ext, m[2])
}

// winShortOf is winShortFor, with long read from its raw name too: a long name that is one of wide (its raw name held a
// letter outside ASCII, which Fold may have made ASCII: wideNames) takes any short name, as such a name does in
// winShortFor (FXGUARDWINF1).
func winShortOf(short, long string, wide []string) bool {
	return winShortFor(short, long) || long != short && slices.Contains(wide, long) && winShortName.MatchString(short)
}

func isDriveLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// gitShortName is .git's 8.3 name on Windows (GIT~1), folded.
var gitShortName = regexp.MustCompile(`^git~[0-9]+$`)

// gitFolderTargets are a .git part anywhere in a path, for a pattern to be compared with: the whole path, its start,
// its end, or its middle. One of the part's own characters must be written in the pattern (.g*, .*, *it/refs), as
// the guard reads its other kept names: a group or a brace with nothing of it written (split(), {1..9}) is no .git.
var gitFolderTargets = func() []skTarget {
	dotGit := piece{text: ".git", counted: true}
	slash := piece{text: "/"}
	return []skTarget{
		target(1, dotGit),
		target(1, dotGit, slash, piece{any: true}),
		target(1, piece{any: true}, slash, dotGit),
		target(1, piece{any: true}, slash, dotGit, slash, piece{any: true}),
	}
}()
