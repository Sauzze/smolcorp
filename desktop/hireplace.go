package desktop

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The hire's place (Phase 6 T602, part A), as the Mac's HireDesk.prepare: a git
// worktree and branch of its own, the worktree trusted in the hire's Claude
// account, and a range of ports. It gets the place ready, or refuses with
// nothing left behind. Nothing calls it yet: part B (tmux) and part C (wiring)
// follow. Nothing here types into a window or starts an agent.

const (
	firstHirePort, lastHirePort, portsPerHire = 4100, 4999, 4
	worktreeTimeout                           = 30 * time.Second
	gitTimeout                                = 10 * time.Second
)

var (
	errHireTrust = &hireRefusal{Status: 409, Message: "Trust this folder once in Claude Code, then hire again: Smolcorp doesn't know how this account keeps its trusted folders."}
	// A fresh account has no .claude.json until Claude Code has run on it once.
	// Smolcorp never makes one: it only edits what Claude Code wrote, so it refuses.
	errHireNoConfig = &hireRefusal{Status: 409, Message: "Start Claude Code once on this account, so it has a .claude.json, then hire again: Smolcorp only edits that file, it never creates it."}
	errHireNoKey    = &hireRefusal{Status: 409, Message: "Smolcorp couldn't read your Anthropic API key from the secret store. Save it again under Summaries, or hire on a Claude account."}
)

// gitRunner runs `git -C dir args`: its status and output (errors included),
// ok false if it couldn't be run or was stopped for taking too long.
type gitRunner func(dir string, args ...string) (status int, output string, ok bool)

// runGit is the real gitRunner. A dir in a hire's own worktree (and the fire's removal of one) is a desk's tree:
// git runs there as deskRun runs it, named and hardened, in the hire's sandbox when it has one (deskrun.go). Any other
// folder may be a repo a desk can write: git runs there only on config the core wrote, or not at all, and in the
// owner's repo SA-X7's checks come first (ownergit.go).
//
// A call whose answer its caller compares exactly (gitComparedExactly) is answered by runGitOut: what git said on
// standard output, its standard error in the log. A clean tree's status still counts every line git said on standard
// error (FXWINWORKTREER2): git says it couldn't look at a folder (a path too long, one the owner can't read) only
// there, with exit 0, so the answer is that line too, and the tree is kept, unless the line is one harmlessGitWarnings
// names.
func runGit(dir string, args ...string) (int, string, bool) {
	if gitComparedExactly(args) {
		status, out, errs, ok := runGitOut(dir, args...)
		if errs != "" {
			log.Printf("smolcorp: git %s in %s said on its standard error (exit %d): %s", strings.Join(args, " "), dir, status, gitWords(errs, 500))
		}
		if held := gitWarningsHeld(errs); args[0] == "status" && held != "" {
			out = strings.TrimSpace(out + "\n" + held)
		}
		return status, out, ok
	}
	if run, mine := deskGitFor(dir, args); mine {
		return run()
	}
	return ownerGit(dir, args)
}

// runGitOut is runGit with git's two streams apart (FXWINWORKTREER1): out is its standard output alone, errs its
// standard error. Only the reads split (a status, a branch's name); any other call answers as runGit's, errs "".
func runGitOut(dir string, args ...string) (status int, out, errs string, ok bool) {
	if run, mine := deskGitForAs(dir, args); mine {
		return run(true)
	}
	return ownerGitAs(dir, args, true)
}

// gitComparedExactly is whether args are a call whose answer its caller compares exactly (FXWINWORKTREER1): a clean
// tree's empty status (removeWorkspace) and the branch a tree has checked out (hiresOwnWorktree, hirePushRemote). The
// branch's name is git's standard output alone, so a warning beside it doesn't change it. The status's standard error
// still counts, but for the lines harmlessGitWarnings names (FXWINWORKTREER2). It stays fail-closed: a nonzero exit,
// or any answer where none is expected, still counts.
func gitComparedExactly(args []string) bool {
	return slices.Equal(args, []string{"status", "--porcelain", "--ignore-submodules=all"}) ||
		slices.Equal(args, []string{"symbolic-ref", "--short", "-q", "HEAD"})
}

// harmlessGitWarnings is each line git may print on standard error beside a status that still looked at the whole
// tree, by a narrow form of its own, matched against the whole line (FXWINWORKTREER2). Any other line keeps the tree:
// git says it couldn't open a folder only there, with exit 0, and the work in that folder would go with the tree. A
// form goes in only from words git was seen to print (Git for Windows' on windows-latest, in the log's "said on its
// standard error" lines), each with a row of its own (TestOnlyTheHarmlessWarningLetsACleanTreeGo):
//   - Git for Windows 2.56.0.windows.4, on a clean tree (FXWINWORKTREER3): it couldn't open the grafts file the core's
//     reads named with an empty name. That was the core's own setting, and reads name the null device there now
//     (gitNoGrafts), but the line stays: it names no folder of the tree, and a Git for Windows that prints it anyway
//     still lets a clean tree go. Only the empty name: the same words with a path in them are held.
var harmlessGitWarnings = []*regexp.Regexp{
	regexp.MustCompile("^" + regexp.QuoteMeta("warning: unable to access '': Invalid argument") + "$"),
}

// gitWarningsHeld is the lines of errs, git's standard error, that harmlessGitWarnings doesn't name: "" for none.
func gitWarningsHeld(errs string) string {
	var held []string
	for _, line := range strings.Split(errs, "\n") {
		line = strings.TrimSpace(line)
		harmless := func(form *regexp.Regexp) bool {
			at := form.FindStringIndex(line)
			return at != nil && at[0] == 0 && at[1] == len(line)
		}
		if line != "" && !slices.ContainsFunc(harmlessGitWarnings, harmless) {
			held = append(held, line)
		}
	}
	return strings.Join(held, "\n")
}

// gitWords is what git said, on one line, each run of spaces and line ends one space (as gitSaid has it), cut to n
// runes: for the log, where a name git quotes with a line end in it would otherwise start a line of its own.
func gitWords(text string, n int) string {
	return firstRunes(strings.Join(strings.Fields(text), " "), n)
}

// hireRequest is what the place is made for.
type hireRequest struct {
	HireID    string
	Name      string // its worktree and branch are named for it: the nickname, else the role
	Folder    string
	Claude    bool   // only Claude Code has trust, an account and an apiKeyHelper
	ConfigDir string // the account's config folder; "" is ~/.claude's
	APIKey    bool   // the API-key account was picked
	Worktree  bool   // a repo gets a worktree for it
	// Held is the scope of a GM held to one project, whose hire this is: Folder is the folder it checked, links
	// followed, and the place is made only while it is still that folder, held, and its checkout's top. nil for none.
	Held *gmScope
	// Check, when set, is asked about the place once it's known, before it is trusted: its error refuses the hire,
	// and the worktree made for it is taken away (a desk in its sandbox at the projects folder: FXHIREPROJECTSR4).
	Check func(hirePlace) error
}

// hirePlaceOptions are what prepareHire needs from outside, each with a test stand-in.
type hirePlaceOptions struct {
	Worktrees string   // the folder set for worktrees, "" for beside the repo
	Taken     []string // the port ranges of hires still working
	Git       gitRunner
	PortFree  func(int) bool
	Key       func() string // the API key, read from the secret store
}

// hirePlace is where the hire works.
type hirePlace struct {
	Folder   string // the folder the agent starts in
	Branch   string
	Worktree string
	Repo     string // the repo's top folder, when there is one
	Ports    string
	Note     string // something to tell whoever hired
}

const notARepo = "This folder isn't a git repository, so the hire works in the folder itself."

// prepareHire gets the hire's place ready, or refuses with nothing left behind.
func prepareHire(r hireRequest, o hirePlaceOptions) (hirePlace, error) {
	git, free := o.Git, o.PortFree
	if git == nil {
		git = runGit
	}
	if free == nil {
		free = portIsFree
	}
	var repo string
	if status, out, ok := git(r.Folder, "rev-parse", "--show-toplevel"); ok && status == 0 && out != "" {
		repo = physical(out)
	}
	given := physical(r.Folder)
	// A scoped GM's hire (FXGMSCOPEPIN): resolved here, before anything is read or made there, the folder is still
	// the one its scope checked, and that scope still holds it; what follows acts on that folder alone.
	if r.Held != nil {
		if real, held := r.Held.place(r.Folder); !held || real != r.Folder || given != r.Folder || repo != r.Folder {
			return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
		}
	}
	configDir := r.ConfigDir
	file := claudeJSON(configDir)
	var tail *string
	// Before anything is made: the settings that would change who pays, and a config we can't edit.
	if r.Claude {
		if configDir == "" {
			configDir = filepath.Join(homeDir, ".claude")
		}
		if !r.APIKey {
			if helper := apiKeyHelper(r.Folder, repo, configDir); helper != "" {
				return hirePlace{}, &hireRefusal{Status: 409, Message: helper + " sets an apiKeyHelper, which would bill an API key instead of the Claude account. " +
					"Take it out, or hire on the API-key account."}
			}
		}
		resolved := file
		if link, err := filepath.EvalSymlinks(file); err == nil {
			resolved = link
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return hirePlace{}, errHireNoConfig
		}
		if _, err := trusting("/", data); err != nil {
			return hirePlace{}, errHireTrust
		}
		// The API-key account: Claude Code asks "use this API key?" (default No) unless the key's tail is approved.
		if r.APIKey {
			var key string
			if o.Key != nil {
				key = o.Key()
			}
			if tail = keyApproval(key); *tail == "" {
				return hirePlace{}, errHireNoKey
			}
			if _, err := approving(*tail, data); err != nil {
				return hirePlace{}, errHireTrust
			}
		}
	}
	ports := allocatePorts(o.Taken, free)
	if ports == "" {
		return hirePlace{}, &hireRefusal{Status: 409, Message: "No free ports for a hire: fire someone, or free ports " +
			strconv.Itoa(firstHirePort) + " to " + strconv.Itoa(lastHirePort) + "."}
	}
	place := hirePlace{Folder: r.Folder, Ports: ports}
	if repo != "" && r.Worktree {
		name := hireWorktreeName(r.Name, r.HireID)
		parent := hiresFolder(repo, o.Worktrees)
		path, branch := filepath.Join(parent, name), hireBranch(r.HireID, name)
		// A scoped GM's hire (FXGMSCOPEPINR1): git is never driven by the folder's path again once it was checked,
		// as the GM can swap it for a link to a repo of its own. The worktree is made from the project, whose path
		// the GM can't change, on the commit the project's own registration of that worktree names.
		from, add := repo, []string{"worktree", "add", "-b", branch, path}
		if r.Held != nil {
			from = r.Held.root
			if repo != from {
				commit := r.Held.commitOf(repo)
				if commit == "" {
					return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
				}
				add = append(add, commit)
			}
		}
		_ = os.MkdirAll(parent, 0o755)
		status, out, ok := git(from, add...)
		if !ok || status != 0 {
			// No answer: git was stopped for taking too long, and may have left a half-made worktree.
			if !ok {
				discardHalfMade(from, parent, path, branch, git)
				if r.Held != nil {
					r.Held.discardMade(branch, git)
				}
				if out == "" { // else the words of the core's refusal (ownerGit)
					out = "git didn't answer"
				}
			}
			return hirePlace{}, &hireRefusal{Status: 409, Message: "Couldn't make the hire's worktree: " + firstRunes(out, 200)}
		}
		// Its upstream, so a plain `git push` works from its sandbox: the worktree is taken away if it can't be set.
		if !presetUpstream(repo, branch, git) {
			removeWorkspace(repo, physical(path), branch, git)
			return hirePlace{}, &hireRefusal{Status: 409, Message: "Couldn't set where the hire's branch pushes to in your repo's git config, so no worktree was made."}
		}
		place.Repo, place.Branch, place.Worktree = repo, branch, physical(path)
		// A scoped GM's hire: the worktree git made is the project's, and the folder asked for is still the one checked
		// (a hire's row keeps it); else the worktree is taken away again, from the project, and nothing starts in it.
		if r.Held != nil {
			if real, held := r.Held.place(r.Folder); !held || real != r.Folder || !r.Held.holds(place.Worktree) {
				removeWorkspace(from, place.Worktree, place.Branch, git)
				r.Held.discardMade(place.Branch, git)
				return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
			}
		}
		place.Folder = place.Worktree
		// By this system's separator: on Windows, given is C:\…\repo\sub.
		if rest, found := strings.CutPrefix(given, repo+string(filepath.Separator)); found {
			place.Folder = filepath.Join(place.Worktree, rest)
		}
	} else if repo == "" {
		place.Note = notARepo
	}
	if r.Check != nil {
		if err := r.Check(place); err != nil {
			if place.Worktree != "" {
				removeWorkspace(place.Repo, place.Worktree, place.Branch, git)
			}
			return hirePlace{}, err
		}
	}
	if r.Claude {
		// With symlinks resolved: how Claude Code knows a folder.
		trusted := place.Folder
		if place.Worktree != "" {
			trusted = place.Worktree
		}
		if err := preTrust(physical(trusted), file, tail); err != nil {
			if place.Worktree != "" {
				removeWorkspace(place.Repo, place.Worktree, place.Branch, git)
			}
			if errors.Is(err, claudeJSONRefusal) {
				err = errHireTrust // as the up-front check, and the Mac
			} else if _, refused := err.(*hireRefusal); !refused {
				err = &hireRefusal{Status: 500, Message: err.Error()}
			}
			return hirePlace{}, err
		}
	}
	// Its submodules, on a Mac, where a desk can't initialise one (FXHIREPROJECTSR9): last, as a worktree with one
	// checked out is kept by every removal above (removeDeskTree).
	if place.Worktree != "" && runtime.GOOS == "darwin" {
		initHireSubmodules(place.Repo, place.Worktree, git)
	}
	return place, nil
}

// hireBranch is the branch a hire works on in its worktree, hire/<its id>/<name>: in a folder of its own, the one a
// Linux desk's view lets it write (sandbox.HireBranch, SIERRA's ruling on RT126SAL1R2 N1). The ID is a slug here too,
// as in the worktree's name; the core's IDs are 12 lower-case hex digits, which a slug keeps as they are (a desk's
// Places.OwnID must be the same, or its view lets it write no branch).
func hireBranch(id, name string) string { return sandbox.HireBranch(hireSlug(id), name) }

// hireWorktreeName is the name of hire id's worktree, named for name (a role's id, or its agent's), and the last part
// of its branch (hireBranch). Both parts are slugs: the ID is never trusted to be a safe path part. A plain desk's id
// is told by what follows its "plain-" (plainDeskIDNew), so two plain hires of one repo don't get one name
// (FXHIREPROJECTSR8). A plain hire's launcher checks the branch it is told by it (parseRun).
func hireWorktreeName(name, id string) string {
	return hireSlug(name) + "-" + firstRunes(hireSlug(strings.TrimPrefix(id, "plain-")), 6)
}

// hiresFolder is where the worktrees of a repo's hires are made: beside the
// repo, or under worktrees when that is set.
func hiresFolder(repo, worktrees string) string {
	if worktrees != "" {
		return filepath.Join(worktrees, filepath.Base(repo))
	}
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-hires")
}

// discardHalfMade clears what a `worktree add` that was stopped may have left:
// its registration, its folder (new, named for this hire alone) and its branch.
// It never deletes a folder that isn't strictly inside parent.
func discardHalfMade(repo, parent, worktree, branch string, git gitRunner) {
	if !strictlyInside(parent, worktree) {
		return
	}
	git(repo, "worktree", "remove", "--force", worktree)
	os.RemoveAll(worktree)
	git(repo, "worktree", "prune")
	git(repo, "branch", "-D", branch)
}

// discardMade takes away what is still in the project of a scoped GM's hire refused once its worktree was made
// (FXRELPATHS, RT126GMSCOPEPINR2 F2), after the worktree's own removal (removeWorkspace, discardHalfMade). That one
// goes by the folder, and is refused when the folder is no longer the hire's tree (the GM can swap it for a link), so
// this goes by the project's own registration: the one whose HEAD is on the hire's branch, which the core carried
// moments ago (the branch is new, and no desk writes a registration). It is removed through a root that leads no
// link out of the project's .git, and the folder it names is never followed or touched: what is there now may not be
// the hire's. Then the branch, whatever it holds, as a stopped hire's: the hire never started.
func (s gmScope) discardMade(branch string, git gitRunner) {
	if root, err := os.OpenRoot(filepath.Join(s.root, ".git")); err == nil {
		if folder, err := root.Open("worktrees"); err == nil {
			entries, _ := folder.ReadDir(-1)
			folder.Close()
			for _, entry := range entries {
				if head, ok := gitRegistrationFile(root, entry.Name(), "HEAD"); entry.IsDir() && ok && head == "ref: refs/heads/"+branch {
					_ = root.RemoveAll(filepath.ToSlash(filepath.Join("worktrees", entry.Name())))
				}
			}
		}
		root.Close()
	}
	git(s.root, "branch", "-D", branch)
}

// strictlyInside is whether path, resolved and without "..", is below parent
// and is not parent itself.
func strictlyInside(parent, path string) bool {
	base := filepath.Base(path)
	if base == "." || base == ".." || base == string(filepath.Separator) || strings.Contains(path, "..") {
		return false
	}
	resolved := filepath.Join(physical(filepath.Dir(path)), base)
	if info, err := os.Lstat(resolved); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	rel, err := filepath.Rel(physical(parent), resolved)
	return err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// removeWorkspace removes a worktree made for a hire, and its branch, when it
// is clean: nothing the hire did is lost. False when it was left, as it holds
// changes (or git couldn't say). The clean check is this status, on the tree:
// the removal is forced, as git's own check would run a status on whatever the
// worktree's .git file names, a file the hire writes. For a hire's own tree the
// removal is the core's, never git's: moved aside, checked again, deleted with
// no link followed (deskrun.go, removeDeskTree).
func removeWorkspace(repo, worktree, branch string, git gitRunner) bool {
	if repo == "" || worktree == "" {
		return false
	}
	// A tree kept is never a quiet answer (FXWINWORKTREER1): one line says which tree and why.
	if status, out, ok := git(worktree, "status", "--porcelain", "--ignore-submodules=all"); !ok || status != 0 || out != "" {
		why := "git's status there couldn't be run, or was refused"
		switch {
		case ok && status != 0:
			why = "git's status there ended with " + strconv.Itoa(status)
		case ok:
			why = "it holds changes, or git warned while it looked (" + strconv.Itoa(len(strings.Split(out, "\n"))) + " lines in git's status)"
		}
		log.Printf("smolcorp: the worktree %s is kept: %s", worktree, why)
		return false
	}
	if status, out, ok := git(repo, "worktree", "remove", "--force", worktree); !ok || status != 0 {
		why := "its removal couldn't be run, or was refused"
		if ok {
			why = "its removal ended with " + strconv.Itoa(status)
			if out != "" {
				why += ": " + gitWords(out, 200)
			}
		}
		log.Printf("smolcorp: the worktree %s is kept: %s", worktree, why)
		return false
	}
	// Only a branch your checkout's HEAD holds is deleted, as before it had an upstream: `branch -d` alone would also
	// take one its upstream holds (pushed, but not merged here), and keep one HEAD holds whose upstream is behind
	// (pushed, then more merged here). With HEAD's look passed, -D deletes just what -d did before the upstream. The
	// branch is named in full: by its short name, a tag of that name would be looked at instead (RT126SAX7R2's F2).
	if status, _, ok := git(repo, "merge-base", "--is-ancestor", "refs/heads/"+branch, "HEAD"); ok && status == 0 {
		git(repo, "branch", "-D", branch)
	}
	return true
}

// allocatePorts is the first range of portsPerHire ports that no taken range
// touches and that are all free, written "4100-4103"; "" for none.
func allocatePorts(taken []string, free func(int) bool) string {
	type span struct{ from, to int }
	var used []span
	for _, text := range taken {
		from, to, found := strings.Cut(text, "-")
		a, errA := strconv.Atoi(from)
		b, errB := strconv.Atoi(to)
		if found && errA == nil && errB == nil && a <= b && b >= firstHirePort && a <= lastHirePort {
			used = append(used, span{max(a, firstHirePort), min(b, lastHirePort)})
		}
	}
next:
	for start := firstHirePort; start+portsPerHire-1 <= lastHirePort; {
		end := start + portsPerHire - 1
		for _, u := range used {
			if u.from <= end && start <= u.to {
				start = u.to + 1
				continue next
			}
		}
		for port := start; port <= end; port++ {
			if !free(port) {
				start = port + 1
				continue next
			}
		}
		return strconv.Itoa(start) + "-" + strconv.Itoa(end)
	}
	return ""
}

// portIsFree is whether nothing is listening on port on this computer.
func portIsFree(port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// hireSlug is lower case letters, digits and dashes: a folder or branch name's
// part. "hire" for nothing usable.
func hireSlug(text string) string {
	var out strings.Builder
	for _, c := range strings.ToLower(text) {
		if c < 128 && (c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			out.WriteRune(c)
		} else {
			out.WriteByte('-')
		}
	}
	joined := strings.Join(strings.FieldsFunc(out.String(), func(r rune) bool { return r == '-' }), "-")
	if joined == "" {
		return "hire"
	}
	return firstRunes(joined, 30)
}

func firstRunes(text string, n int) string {
	if runes := []rune(text); len(runes) > n {
		return string(runes[:n])
	}
	return text
}

// apiKeyHelper is the first settings file among those of places (folders with a
// .claude/ inside, or an account's own folder) that sets an apiKeyHelper, which
// would bill a key rather than the account; "" if none.
func apiKeyHelper(places ...string) string {
	for _, place := range places {
		if place == "" {
			continue
		}
		for _, name := range []string{".claude/settings.json", ".claude/settings.local.json", "settings.json", "settings.local.json"} {
			file := filepath.Join(place, name)
			data, err := os.ReadFile(file)
			var root map[string]json.RawMessage
			if err == nil && json.Unmarshal(data, &root) == nil {
				if _, has := root["apiKeyHelper"]; has {
					return file
				}
			}
		}
	}
	return ""
}
