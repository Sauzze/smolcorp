package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// The pull request badge (1.26 R1): for a desk whose folder is on a branch with a pull request on github.com, its
// number, state, review decision and address. It only reads: the one command is
// `gh pr view <branch> --repo owner/name --json number,state,reviewDecision,url`, so no `gh` write is ever run here
// (the guard's publish group refuses those to hires and the GM, and is left as it is).
//
// The branch and the remote come from git's safe read (gitstate.go), never from a plain `git`. A hire writes the
// repos read here, so what gh is told is checked (the repo's owner and name, the branch's spelling), where gh runs
// is a folder of no repo, what it sees of the environment is a short list, and what it answers is checked again
// (its address must be the asked repo's, its states from a closed set) before any of it is shown. Anything that
// doesn't hold is no badge, and no error to the desk.
//
// A lookup never makes the desk wait: it hands back what is kept, and a run in the background (one at a time per
// repo and branch, a few at once in all) refreshes it when it is older than prEvery.

const (
	prEvery      = time.Minute     // an answer, or the lack of one, is kept this long per (repo, branch)
	prStaleFor   = 5 * time.Minute // an older answer than this is not shown while its refresh runs
	prTimeout    = 8 * time.Second // a gh run that takes longer is stopped
	prOutputCap  = 16 << 10        // bytes of gh's answer; more is no badge
	prErrorCap   = 4 << 10         // bytes of gh's error text kept, to tell "signed out" from the rest
	prAtOnce     = 3               // gh runs at the same time, whatever the number of desks
	prKept       = 128             // (repo, branch) pairs remembered
	prMaxNumber  = 100_000_000     // a number above this isn't one
	ghAuthStatus = 4               // gh's exit code for "sign in first"
	ghHintNone   = ""              // gh answered, or nothing was asked
	ghHintAbsent = "missing"       // gh isn't installed where it is looked for
	ghHintOut    = "signedout"     // gh is installed and isn't signed in to github.com
)

// PRBadge is what the desk is shown of its branch's pull request.
type PRBadge struct {
	Number         int    `json:"number"`
	State          string `json:"state"`                    // OPEN, CLOSED or MERGED
	ReviewDecision string `json:"reviewDecision,omitempty"` // APPROVED, CHANGES_REQUESTED or REVIEW_REQUIRED; "" when none is asked
	URL            string `json:"url"`
}

// prAnswer is a lookup's result: the badge, if any, and a hint for Setup when gh is why there is none.
type prAnswer struct {
	badge *PRBadge
	hint  string
}

// What GitHub allows in an owner and in a repo name, and what an scp-style remote looks like.
var (
	ghOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	ghNamePattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	ghScpPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]+@github\.com:([^/:]+)/([^/:]+?)/?$`)
)

// githubRepoOf is "owner/name" for a remote on github.com (https, ssh or git, or scp's `git@github.com:o/n.git`),
// and "" for any other host, a port, or a remote that can't be read. A user name before the host is dropped, and
// never kept. The spelling is checked, since it goes to a command.
func githubRepoOf(remote string) string {
	remote = strings.TrimSpace(remote)
	var path string
	if m := ghScpPattern.FindStringSubmatch(remote); m != nil {
		path = m[1] + "/" + m[2]
	} else if u, err := url.Parse(remote); err == nil && u.Host != "" {
		if (u.Scheme != "https" && u.Scheme != "ssh" && u.Scheme != "git") || !strings.EqualFold(u.Host, "github.com") ||
			u.RawQuery != "" || u.Fragment != "" {
			return ""
		}
		path = strings.Trim(u.Path, "/")
	} else {
		return ""
	}
	owner, name, found := strings.Cut(path, "/")
	name = strings.TrimSuffix(name, ".git")
	if !found || !ghOwnerPattern.MatchString(owner) || !ghNamePattern.MatchString(name) || name == "." || name == ".." {
		return ""
	}
	return owner + "/" + name
}

// prBranchOK is whether branch is safe to hand gh as its argument: gh reads a bare number as a pull request, "x:y" as
// someone's fork's branch and anything starting with "-" as an option, and none of those is this repo's branch.
func prBranchOK(branch string) bool {
	if branch == "" || len(branch) > 250 || branch[0] == '-' || strings.ContainsAny(branch, ":\\ ") {
		return false
	}
	if _, err := strconv.Atoi(branch); err == nil {
		return false
	}
	for _, r := range branch {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// gitRemoteOf is the url of the remote a branch tracks (origin if it has none), out of `config --list -z`, or "".
// With `url.<base>.insteadOf` rewrites in the config the address git would use isn't the one written, so there is none.
func gitRemoteOf(list []byte, branch string) string {
	if branch == "" {
		return ""
	}
	remotes := map[string]string{}
	remoteName := ""
	for _, record := range strings.Split(string(list), "\x00") {
		key, value, _ := strings.Cut(record, "\n")
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		if first < 0 || first == last {
			continue
		}
		section, sub, name := strings.ToLower(key[:first]), key[first+1:last], strings.ToLower(key[last+1:])
		switch {
		case section == "url" && (name == "insteadof" || name == "pushinsteadof"):
			return ""
		case section == "branch" && name == "remote" && sub == branch:
			remoteName = value
		case section == "remote" && name == "url":
			remotes[sub] = value // the last one stands, as git's own `config --get`
		}
	}
	if remoteName == "" {
		remoteName = "origin"
	}
	return remotes[remoteName] // a remote named "." (this repo) has no url here
}

// ghEnvKept are the variables a gh run is given, by name (upper case), and nothing else of the app's environment
// goes: where the account's home is (so its config and, on Windows and Linux, its credentials are found), the
// temporary folder and the system folder Windows needs, the locale, the session bus and runtime folder Linux's
// keyring is reached by, the proxy and certificate settings a network may need. None starts with GH_ or GITHUB_ (no
// host, repo, token, config folder, pager, browser or editor), and nothing here names a program.
var ghEnvKept = map[string]bool{
	"HOME": true, "USERPROFILE": true, "APPDATA": true, "LOCALAPPDATA": true, "SYSTEMROOT": true, "WINDIR": true,
	"TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "LANGUAGE": true, "USER": true, "USERNAME": true,
	"DBUS_SESSION_BUS_ADDRESS": true, "XDG_RUNTIME_DIR": true,
	"HTTPS_PROXY": true, "HTTP_PROXY": true, "NO_PROXY": true, "ALL_PROXY": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
}

// ghRunEnv is env cut down to ghEnvKept (and LC_*), with a PATH of the system's own folders (gh calls nothing it
// needs a hire's PATH for), and gh told to ask nothing, check for no update, show no colour or spinner and page nothing.
func ghRunEnv(env []string) []string {
	out := make([]string, 0, 24)
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		if ghEnvKept[name] || strings.HasPrefix(name, "LC_") {
			out = append(out, kv)
		}
	}
	path := "/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		path = filepath.Join(os.Getenv("SystemRoot"), "System32")
	}
	return append(out, "PATH="+path, "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "GH_NO_EXTENSION_UPDATE_NOTIFIER=1",
		"GH_SPINNER_DISABLED=1", "GH_PAGER=cat", "PAGER=cat", "NO_COLOR=1", "CLICOLOR=0", "GH_TELEMETRY=false", "DO_NOT_TRACK=1")
}

// ghPlaces are where gh may be besides the PATH: Homebrew's, the system's, Linuxbrew's and snap's, and the
// installer's and scoop's on Windows. A variable, for the tests.
var ghPlaces = func() []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(os.Getenv("ProgramFiles"), "GitHub CLI"), filepath.Join(os.Getenv("ProgramFiles(x86)"), "GitHub CLI"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "GitHub CLI"), filepath.Join(os.Getenv("USERPROFILE"), "scoop", "shims")}
	}
	return []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin", "/usr/bin", "/home/linuxbrew/.linuxbrew/bin", "/snap/bin"}
}()

// findGH is the gh program, or "": in the usual places first, then on the PATH your login shell sets (as findTmux does),
// and last on this program's own PATH, and only as a full path (never one relative to a folder). The PATH of a hire's
// tab is never looked at: it isn't this program's.
func findGH() string {
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.exe"
	}
	runnable := func(folder string) string {
		if folder == "" || !filepath.IsAbs(folder) {
			return ""
		}
		path := filepath.Join(folder, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0) {
			return path
		}
		return ""
	}
	for _, folder := range ghPlaces {
		if path := runnable(folder); path != "" {
			return path
		}
	}
	for _, folder := range tmuxLoginPath() {
		if path := runnable(folder); path != "" {
			return path
		}
	}
	for _, folder := range filepath.SplitList(os.Getenv("PATH")) {
		if path := runnable(folder); path != "" {
			return path
		}
	}
	return ""
}

// ghRunner runs gh for a repo ("owner/name") and branch, and returns its standard output (capped), the text of its
// errors (capped) and its exit code, or an error when it could not run, took too long or answered over the cap.
type ghRunner func(ctx context.Context, repo, branch string) (out, errText []byte, status int, err error)

// capped keeps what it is written up to its cap and refuses the rest, which stops the copy. It holds its buffer as a
// field, not embedded: an embedded one would give it bytes.Buffer's ReadFrom, which io.Copy prefers to Write and which
// has no cap.
type capped struct {
	buf  bytes.Buffer
	cap  int
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > c.cap {
		c.over = true
		return 0, errGitTooBig
	}
	return c.buf.Write(p)
}

func (c *capped) Bytes() []byte { return c.buf.Bytes() }

var errGHMissing = errors.New("gh isn't installed")

// runGH is the real ghRunner: gh as findGH found it, run from a folder that is no repo (so no repo config is read),
// with ghRunEnv, no shell, nothing on its standard input and a deadline from ctx.
func runGH(ctx context.Context, repo, branch string) (out, errText []byte, status int, err error) {
	program := findGH()
	if program == "" {
		return nil, nil, 0, errGHMissing
	}
	cmd := disclaim.Cmd(exec.CommandContext(ctx, program, "pr", "view", branch, "--repo", repo, "--json", "number,state,reviewDecision,url"))
	cmd.Dir = os.TempDir()
	cmd.Env = ghRunEnv(os.Environ())
	cmd.WaitDelay = 2 * time.Second
	hideGitWindow(cmd)
	stdout, stderr := &capped{cap: prOutputCap}, &capped{cap: prErrorCap}
	cmd.Stdout, cmd.Stderr = stdout, io.Writer(stderr)
	runErr := cmd.Run()
	switch {
	case stdout.over:
		return nil, nil, 0, errGitTooBig
	case ctx.Err() != nil:
		return nil, nil, 0, ctx.Err()
	case runErr == nil:
		return stdout.Bytes(), stderr.Bytes(), 0, nil
	}
	if exit, isExit := runErr.(*exec.ExitError); isExit {
		return nil, stderr.Bytes(), exit.ExitCode(), nil
	}
	return nil, nil, 0, runErr
}

var (
	prStates   = map[string]bool{"OPEN": true, "CLOSED": true, "MERGED": true}
	prDecision = map[string]bool{"": true, "APPROVED": true, "CHANGES_REQUESTED": true, "REVIEW_REQUIRED": true}
)

// prFromGH is the badge in gh's answer, or nil when it isn't what was asked for: one JSON object of exactly the four
// fields, a number that is a number, a state and a decision from the closed sets, and an address that is
// https://github.com/<owner>/<name>/pull/<number> of the asked repo (GitHub may spell the owner or name in another case).
func prFromGH(out []byte, repo string) *PRBadge {
	var got struct {
		Number         int    `json:"number"`
		State          string `json:"state"`
		ReviewDecision string `json:"reviewDecision"`
		URL            string `json:"url"`
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.DisallowUnknownFields()
	if dec.Decode(&got) != nil || dec.More() {
		return nil
	}
	want := "https://github.com/" + repo + "/pull/" + strconv.Itoa(got.Number)
	switch {
	case got.Number < 1 || got.Number > prMaxNumber, !prStates[got.State], !prDecision[got.ReviewDecision]:
		return nil
	case !strings.EqualFold(got.URL, want) || !ghURLPattern.MatchString(got.URL):
		return nil
	}
	return &PRBadge{Number: got.Number, State: got.State, ReviewDecision: got.ReviewDecision, URL: want}
}

var ghURLPattern = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9-]+/[A-Za-z0-9._-]+/pull/[0-9]+$`)

// prEntry is what is kept for one (repo, branch).
type prEntry struct {
	answer  prAnswer
	has     bool
	at      time.Time
	running bool
}

// prBadges keeps the pull request of each (repo, branch) for prEvery.
type prBadges struct {
	mu    sync.Mutex
	by    map[string]*prEntry
	run   ghRunner
	now   func() time.Time
	slots chan struct{}
	wg    sync.WaitGroup // the runs under way, for the tests to wait on
}

func newPRBadges(run ghRunner) *prBadges {
	return &prBadges{by: map[string]*prEntry{}, run: run, now: time.Now, slots: make(chan struct{}, prAtOnce)}
}

// prs is the one the app reads through.
var prs = newPRBadges(runGH)

// For is the badge of the branch state is on, as far as is kept, and a hint when gh is why there is none. It never
// waits: an answer not yet kept is none, and this starts the run that will keep it. A folder in no repo, a detached
// head, a remote that isn't github.com's and a branch gh couldn't safely be given are no badge and no run.
func (p *prBadges) For(state GitState) (*PRBadge, string) {
	repo := githubRepoOf(state.remote)
	if !state.Repo || state.Skipped || state.Detached || repo == "" || !prBranchOK(state.Branch) {
		return nil, ghHintNone
	}
	key := strings.ToLower(repo) + "\x00" + state.Branch
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.by[key]
	if e == nil {
		e = &prEntry{}
		p.by[key] = e
		p.trim(key)
	}
	age := p.now().Sub(e.at)
	if !e.running && (!e.has || age >= prEvery) {
		e.running = true
		p.wg.Add(1)
		go p.refresh(key, e, repo, state.Branch)
	}
	if !e.has || age >= prStaleFor {
		return nil, ghHintNone
	}
	return e.answer.badge, e.answer.hint
}

// refresh runs gh once for key (a few at a time in all) and keeps what it said, whatever that was.
func (p *prBadges) refresh(key string, e *prEntry, repo, branch string) {
	defer p.wg.Done()
	p.slots <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), prTimeout)
	answer := p.ask(ctx, repo, branch)
	cancel()
	<-p.slots
	p.mu.Lock()
	e.answer, e.has, e.at, e.running = answer, true, p.now(), false
	p.mu.Unlock()
}

// ask is one gh run, read: a badge, or none with a hint if gh is missing or signed out.
func (p *prBadges) ask(ctx context.Context, repo, branch string) prAnswer {
	out, errText, status, err := p.run(ctx, repo, branch)
	switch {
	case errors.Is(err, errGHMissing):
		return prAnswer{hint: ghHintAbsent}
	case err != nil:
		return prAnswer{}
	case status == ghAuthStatus || (status != 0 && strings.Contains(string(errText), "gh auth login")):
		return prAnswer{hint: ghHintOut}
	case status != 0: // no pull request for the branch, no network, no access: no badge
		return prAnswer{}
	}
	return prAnswer{badge: prFromGH(out, repo)}
}

// trim forgets the oldest pairs past prKept, never keep's and never one being run. Lock held.
func (p *prBadges) trim(keep string) {
	for len(p.by) > prKept {
		oldest, found := "", false
		for key, e := range p.by {
			if key != keep && !e.running && (!found || e.at.Before(p.by[oldest].at)) {
				oldest, found = key, true
			}
		}
		if !found {
			return
		}
		delete(p.by, oldest)
	}
}

// deskGitAnswer is what the window and the GM are told of a folder: its git state, and beside it, when there is
// one, its branch's pull request, or, when there is none because gh is missing or signed out, a hint (gh) for Setup.
type deskGitAnswer struct {
	GitState
	PR *PRBadge `json:"pr,omitempty"`
	GH string   `json:"gh,omitempty"`
}

// withPR adds the pull request kept for the state's branch (and starts a run to keep it).
func withPR(state GitState) deskGitAnswer {
	badge, hint := prs.For(state)
	return deskGitAnswer{GitState: state, PR: badge, GH: hint}
}
