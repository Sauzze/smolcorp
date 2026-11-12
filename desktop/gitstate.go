package desktop

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Git state of a folder (1.26 FG1): the repo's top, its branch and upstream, how far ahead and behind, the
// changed files and the lines added and removed against HEAD. It only reads: no command here writes a repo (no
// optional locks, so even `status` leaves the index alone). It also runs nothing the repo names: hires write the
// repos read here, and a repo's own config can point at programs (filters, text converters, an external diff, an
// fsmonitor, hooks, a pager, an ssh or credential helper), so each of those is blanked, and the owner's own git
// config is left out too (see gitReadArgs and gitReadEnv). The PR badge and the diffs read it, and it never goes in
// the /events snapshot, so phones see nothing.
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
}

// gitReader runs one read-only git command in dir and returns its standard output, its exit code, and an error
// when it could not run, took too long (ctx) or answered more than the cap. global is the options that go before
// the command's own name (the guards of gitReadArgs and what readGitState adds to them).
type gitReader func(ctx context.Context, dir string, global []string, args ...string) (out []byte, status int, err error)

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

// gitReadArgs go before every command: nothing paged, no lock taken for a refresh, and every program a repo's
// config could name blanked (fsmonitor, hooks, pager, attributes file, ssh and credential helpers, the external
// diff). The commands add --no-ext-diff and --no-textconv where they diff; the filters and text converters a repo
// names are blanked by readGitState (gitBlanks), which has to read the repo's config to know their names.
var gitReadArgs = []string{"--no-pager", "--no-optional-locks",
	"-c", "core.fsmonitor=false", "-c", "core.pager=cat", "-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.attributesFile=" + os.DevNull, "-c", "core.sshCommand=", "-c", "credential.helper=", "-c", "diff.external="}

// gitConfigNames are the environment variables that put git config (or a program) in git's hands from outside:
// they are dropped, whole or by prefix, from a read's environment.
var gitConfigNames = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true, "GIT_OBJECT_DIRECTORY": true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_PAGER": true, "GIT_TERMINAL_PROMPT": true, "GIT_OPTIONAL_LOCKS": true,
	"GIT_CONFIG": true, "GIT_CONFIG_COUNT": true, "GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_SYSTEM": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_EXTERNAL_DIFF": true, "GIT_ATTR_SOURCE": true,
	"GIT_SSH": true, "GIT_SSH_COMMAND": true, "GIT_ASKPASS": true, "SSH_ASKPASS": true, "GIT_PROXY_COMMAND": true,
}

// gitReadEnv is the environment of a read: the app's, without what would point git at another repo or hand it
// config or a program, and with nothing to ask the owner. The system and user git config are switched off, so
// they can't run anything either: the cost is that the owner's own settings don't apply to what is read here (their
// global ignore file, so files it hides show as untracked, and their safe.directory list, so a repo owned by
// another account is refused and reads as "not a repo"). Git older than 2.32 doesn't know GIT_CONFIG_GLOBAL; the
// programs its config names are blanked by name all the same.
func gitReadEnv(env []string) []string {
	out := make([]string, 0, len(env)+5)
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		if gitConfigNames[name] || strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
}

// readGitOut is the real gitReader: `git -C dir …` with no shell, in ctx, with only its standard output kept.
func readGitOut(ctx context.Context, dir string, global []string, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, "git", append(append([]string{"-C", dir}, global...), args...)...)
	cmd.Env = gitReadEnv(os.Environ())
	cmd.WaitDelay = 2 * time.Second
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

// gitHasAttrSource says whether the git here knows --attr-source (2.40 and later), asked once. With an empty tree
// as the source no .gitattributes in the working folder is read, so none can name a filter. It does not cover the
// repo's .git/info/attributes (git reads that whatever the source), so the filters are blanked by name as well.
var gitHasAttrSource = sync.OnceValue(func() bool {
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	out, status, err := readGitOut(ctx, os.TempDir(), nil, "--version")
	return err == nil && status == 0 && gitVersionAtLeast(string(out), 2, 40)
})

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

// The empty tree, by the repo's hash: no attributes in it.
var gitEmptyTrees = map[string]string{
	"sha1":   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
	"sha256": "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
}

// gitBlanks reads `config --list -z` (config with its includes followed) and says, for every filter and text
// converter the repo's config names, the -c options that blank it. Reading config runs nothing.
func gitBlanks(list []byte) []string {
	var blanks []string
	seen := map[string]bool{}
	blank := func(key, value string) {
		if !seen[key] {
			seen[key] = true
			blanks = append(blanks, "-c", key+"="+value)
		}
	}
	for _, record := range strings.Split(string(list), "\x00") {
		key, _, _ := strings.Cut(record, "\n")
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		if first < 0 || first == last { // a variable with no subsection names no program of this kind
			continue
		}
		section, sub, name := strings.ToLower(key[:first]), key[first+1:last], strings.ToLower(key[last+1:])
		switch {
		case section == "filter" && (name == "clean" || name == "smudge" || name == "process"):
			blank("filter."+sub+"."+name, "")
			blank("filter."+sub+".required", "false")
		case section == "diff" && (name == "textconv" || name == "command"):
			blank("diff."+sub+"."+name, "")
		}
	}
	return blanks
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
	attrSource := gitHasAttrSource()
	revParse := []string{"rev-parse", "--show-toplevel", "--absolute-git-dir"}
	if attrSource {
		revParse = append(revParse, "--show-object-format")
	}
	out, status, err := read(ctx, dir, gitReadArgs, revParse...)
	if err != nil {
		return failed(err), ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if status != 0 || len(lines) < 2 || len(lines) > 3 { // a folder in no repo, a bare repo, or the inside of a .git folder
		return state, ""
	}
	state.Repo, state.Top, gitDir = true, filepath.FromSlash(lines[0]), filepath.FromSlash(lines[1])
	// From here git reads files of the repo, which a filter its config names would run a program on: so the
	// filters and text converters it names are blanked, and, where git allows, no attributes are read from the
	// working folder. If the config can't be read, nothing else is.
	list, listStatus, err := read(ctx, dir, gitReadArgs, "config", "--list", "-z")
	if err != nil || listStatus != 0 {
		return failed(err), gitDir
	}
	global := append(append([]string{}, gitReadArgs...), gitBlanks(list)...)
	if attrSource {
		format := "sha1"
		if len(lines) == 3 {
			format = lines[2]
		}
		tree, known := gitEmptyTrees[format]
		if !known {
			return failed(nil), gitDir
		}
		global = append(global, "--attr-source="+tree)
	}
	// Untracked files are listed as folders, not file by file, so a build folder is one line.
	out, status, err = read(ctx, dir, global, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=normal", "--ignore-submodules=dirty")
	if err != nil || status != 0 {
		return failed(err), gitDir
	}
	unborn := parseGitStatus(out, &state)
	if unborn { // no HEAD to count against
		return state, gitDir
	}
	out, status, err = read(ctx, dir, global, "diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=dirty", "HEAD", "--")
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
	return http.StatusOK, gits.get(folder, event)
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
		writeJSON(w, http.StatusOK, gits.get(startedIn(h), 0))
	case query.Get("session") != "":
		writeAnswer(w)(rt.deck.deskGit(query.Get("session")))
	default:
		writeStatus(w)(http.StatusBadRequest, "Send ?hire=<id> or ?session=<id>.")
	}
}
