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
	"sync"
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

// gmHireDeadline is how long a hire the General Manager asked for may go on starting git (FXCOREWORKTREES126, item 5):
// once it has passed, no git of that hire starts, so a FIFO the GM planted in a repo it can write holds its own
// request for at most this and one git timeout (gitTimeout; worktreeTimeout for a worktree's add), and never another
// hire: the GM's hires have their own lock (Deck.gmAskedHiring). A variable, for the tests.
var gmHireDeadline = 45 * time.Second

// hirePlaceDeadline is how long the place of a hire the General Manager didn't ask for (the owner's, Setup's, the
// office's) may go on starting git, across its check and its making (hirePlaceWords, prepareHire; RT126SWEEP4 part 3's
// B1). What the check reads of a repo's .git as files is read without waiting, so a FIFO whoever can write the repo
// put there holds the deck's hires (Deck.hiring) for at most this and one git timeout. Well past what a place takes,
// its worktree's add and its submodules (worktreeTimeout each) included. A variable, for the tests.
var hirePlaceDeadline = 2 * time.Minute

// gitUntil is git (runGit when nil) that starts no call once deadline has passed: such a call answers as one that
// couldn't be run (ok false), and the hire refuses and undoes as for any git that didn't answer. A zero deadline is
// git as it is.
func gitUntil(git gitRunner, deadline time.Time) gitRunner {
	if git == nil {
		git = runGit
	}
	if deadline.IsZero() {
		return git
	}
	return func(dir string, args ...string) (int, string, bool) {
		if !time.Now().Before(deadline) {
			log.Printf("smolcorp: a hire passed its deadline: git %s in %s wasn't run", strings.Join(args, " "), dir)
			return 0, "", false
		}
		return git(dir, args...)
	}
}

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
	// GM: the hire is a General Manager, whose own worktree is made beside the repo (gmTreeFolder), never in a folder
	// hires' worktrees are made in (FXCOREWORKTREES126).
	GM bool
	// Held is the scope of a GM held to one project, whose hire this is: Folder is the folder it checked, links
	// followed, and the place is made only while it is still that folder, held, and its checkout's top. nil for none.
	Held *gmScope
	// Check, when set, is asked about the place once it's known, before it is trusted: its error refuses the hire,
	// and the worktree made for it is taken away (a desk in its sandbox at the projects folder: FXHIREPROJECTSR4).
	// judged is the folder as the rule judged it, its links resolved once (prepareHire's given): a check that looks
	// at the folder itself looks there, never at Folder's links resolved again (FXGMHIRENOGIT126R3, B1).
	Check func(place hirePlace, judged string) error
	// KeepCarried leaves the carry's record (carriedHires, the roots it kept) to the caller once the place is made: its
	// undos after prepareHire go through those roots (takeBackTree), never by the tree's path, which whoever can write
	// the folder for hires can turn into a link meanwhile. The caller forgets it (forgetCarried) when its hire ends,
	// made or refused (FXGMHIRENOGIT126R5, R14). A refused hire's record is forgotten here, as before.
	KeepCarried bool
}

// hirePlaceOptions are what prepareHire needs from outside, each with a test stand-in.
type hirePlaceOptions struct {
	Worktrees string   // the folder set for worktrees, "" for beside the repo
	Taken     []string // the port ranges of hires still working
	Git       gitRunner
	PortFree  func(int) bool
	Key       func() string // the API key, read from the secret store
	// Hold: the ports given are held for this hire until its caller lets go (allocateHirePorts, releaseHirePorts), so a
	// hire made at the same time doesn't take them. Every hire's own way sets it.
	Hold bool
}

// hirePlace is where the hire works.
type hirePlace struct {
	Folder   string // the folder the agent starts in
	Branch   string
	Worktree string
	Repo     string // the repo's top folder, when there is one
	Ports    string
	Note     string // something to tell whoever hired
	// Resolved is, on a Mac, the folder asked for as the rule judged it, its links resolved once (given): what the
	// row of a hire with no worktree keeps, and every start checks (Hire.FolderResolved). "" when its links couldn't
	// be resolved, and elsewhere.
	Resolved string
	// ResolvedID is Resolved's identity, taken with it (folderIdentity): what the row keeps beside it (Hire.FolderID).
	ResolvedID string
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
	// The folder as the rule judges it, links resolved once: every git and every read below runs on this spelling,
	// never on r.Folder's, which the kernel would resolve again at each start (FXGMHIRENOGIT126R2, B2). A link on the
	// way that led out of a GM's own places when judged, and into one by git's start, would have the core's git run
	// there. A folder whose links couldn't be resolved (not there, or a link turned as it was read) gets no git.
	given, resolved := resolvedFolder(r.Folder)
	// What the core carried for the hire's branch is kept until prepareHire is done, or, when the caller asked to keep
	// it (KeepCarried), until the caller's hire ends: made is set once the place is made.
	made := false
	// A General Manager's own folder (hiregmfolder.go): no git at all, not even this first look. The hire works in it
	// as it is, as in a folder that isn't a repo.
	inGMFolder := gmOwnsFolder(given)
	// A General Manager held to one project hires only with a worktree of its own, made from the project (FXGMSCOPEPIN):
	// in a GM's own folder there is none, so it is refused, before anything is read or made.
	if inGMFolder && r.Held != nil {
		return hirePlace{}, &hireRefusal{Status: http.StatusConflict, Message: r.Folder + " is a General Manager's own folder, so Smolcorp doesn't run git there, " +
			"and a General Manager held to one project hires only with a worktree of its own: hire into " + r.Held.root + " itself. Nothing was made."}
	}
	var repo string
	if !inGMFolder && resolved {
		if status, out, ok := git(given, "rev-parse", "--show-toplevel"); ok && status == 0 && out != "" {
			repo = physical(out)
		}
	}
	// A scoped GM's hire (FXGMSCOPEPIN): resolved here, before anything is read or made there, the folder is still
	// the one its scope checked, and that scope still holds it; what follows acts on that folder alone.
	if r.Held != nil {
		if given != r.Folder {
			return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
		}
		if real, held := r.Held.place(given); !held || real != r.Folder || repo != r.Folder {
			return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
		}
	}
	// The folder named for the hire must be in the top git gave for it, compared physically: a .git there naming a
	// git folder whose core.worktree is elsewhere (another repo's submodule, say) moves the top into that repo, and
	// everything below is made there (FXSWEEP3P3R2 B1). Before anything is read or made.
	if repo != "" && !inTop(repo, given) {
		return hirePlace{}, &hireRefusal{Status: 409, Message: "Smolcorp didn't hire into " + r.Folder + ": git takes " + repo +
			" as its repo's top, a folder it isn't in, so its .git leads into another repo. Nothing was made."}
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
			helper, unread := apiKeyHelper(given, repo, configDir)
			if unread != "" {
				return hirePlace{}, &hireRefusal{Status: 409, Message: "Smolcorp couldn't check " + unread + " for an apiKeyHelper, which would bill an API key instead of the Claude account: " +
					"it's far bigger than a settings file. Make it smaller, or hire on the API-key account."}
			}
			if helper != "" {
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
	if o.Hold {
		ports = allocateHirePorts(o.Taken, free)
	}
	if ports == "" {
		return hirePlace{}, &hireRefusal{Status: 409, Message: "No free ports for a hire: fire someone, or free ports " +
			strconv.Itoa(firstHirePort) + " to " + strconv.Itoa(lastHirePort) + "."}
	}
	if o.Hold {
		// A place refused from here lets its ports go at once: its caller is given no place, so no range to let go of
		// (releaseHirePorts), and the range would stay held from the next hire for hirePortsHold (FXHIREORDERFLAKE).
		defer func() {
			if !made {
				releaseHirePorts(ports)
			}
		}()
	}
	place := hirePlace{Folder: startFolder(r.Folder, given, runtime.GOOS), Ports: ports}
	if resolved && runtime.GOOS == "darwin" {
		place.Resolved, place.ResolvedID = given, folderIdentity(given)
	}
	if repo != "" && r.Worktree {
		name := hireWorktreeName(r.Name, r.HireID)
		parent := hiresFolder(repo, o.Worktrees)
		if r.GM {
			parent = gmTreeFolder(repo)
		} else if r.Held != nil && r.Held.demo != "" {
			// A demo's hires stay in the demo folder, beside its project (smolcorp demo's promise: everything is in the
			// demo folder), which its GM's scope holds them to and its Policy denies it (GMHires).
			parent = legacyHiresFolder(repo)
		} else if words := configuredWorktreesWords(o.Worktrees); words != "" {
			// The configured folder is honoured only outside every GM's write reach (FXCOREWORKTREES126): else refused, in
			// plain words, before anything is made.
			return hirePlace{}, &hireRefusal{Status: http.StatusConflict, Message: words + " Nothing was made."}
		} else if o.Worktrees == "" && coreWorktreesOn() {
			// The core's own folder, made (0700) and checked before the worktree is made in it.
			if err := makeCoreWorktrees(); err != nil {
				return hirePlace{}, &hireRefusal{Status: http.StatusConflict, Message: err.Error()}
			}
		}
		path, branch := filepath.Join(parent, name), hireBranch(r.HireID, name)
		// A scoped GM's hire (FXGMSCOPEPINR1): git is never driven by the folder's path again once it was checked,
		// as the GM can swap it for a link to a repo of its own. The worktree is made from the project, whose path
		// the GM can't change, on the commit the project's own registration of that worktree names.
		from, add := repo, []string{"worktree", "add", "-b", branch, path}
		// What the core carries for this branch is where everything after the worktree add writes (FXSWEEP3P3R3 B3):
		// kept for this hire's place alone.
		defer func() {
			if !made || !r.KeepCarried {
				forgetCarried(branch)
			}
		}()
		if r.Held == nil {
			// git must find, at the top it gave, the repo whose top that is: a git folder whose core.worktree names one
			// of the folder's parents gives a top where git finds no repo, or another's (FXSWEEP3P3R3 O2).
			if status, out, ok := git(repo, "rev-parse", "--show-toplevel"); !ok || status != 0 || out == "" || !oneFolderOnDisk(physical(out), repo) {
				return hirePlace{}, &hireRefusal{Status: 409, Message: "Smolcorp didn't hire into " + r.Folder + ": git takes " + repo +
					" as the top, and finds no repo there. Nothing was made."}
			}
		}
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
		_ = os.MkdirAll(parent, hireTreesMode(parent))
		if mode := hireTreesMode(parent); mode == 0o700 {
			// In the core's own folder, the owner's alone even if it was made before (FXCOREWORKTREES126).
			if info, err := os.Lstat(parent); err == nil && info.IsDir() && info.Mode().Perm() != mode {
				_ = os.Chmod(parent, mode)
			}
		}
		status, out, ok := git(from, add...)
		if !ok || status != 0 {
			limit := 200
			switch {
			case !ok && out == "":
				// No answer: git was stopped for taking too long, and may have left a half-made worktree.
				discardHalfMade(from, parent, path, branch, git)
				if r.Held != nil {
					r.Held.discardMade(branch, git)
				}
				out = "git didn't answer"
			case !ok:
				// The core's refusal (ownerGitNo), said before anything was made: nothing to clear, and its words
				// whole, as they name the folder (FXSWEEP3P3R2 N1).
				limit = 1000
			}
			return hirePlace{}, &hireRefusal{Status: 409, Message: "Couldn't make the hire's worktree: " + firstRunes(out, limit)}
		}
		// Its upstream, so a plain `git push` works from its sandbox: the worktree is taken away if it can't be set.
		if !presetUpstream(repo, branch, git) {
			takeBackTree(repo, physical(path), branch, git)
			return hirePlace{}, &hireRefusal{Status: 409, Message: "Couldn't set where the hire's branch pushes to in your repo's git config, so no worktree was made."}
		}
		place.Repo, place.Branch, place.Worktree = repo, branch, physical(path)
		// On a Mac the core carried the tree into a folder it made itself (carryWorktree): the row keeps the carry's own
		// name for it, never path resolved again, and only while path and that name are both still the very folder
		// carried. A tree swapped since, for a link the hire's sandbox would hold as its own place at every start, is
		// taken back through the carry's roots, and the hire refused (FXGMHIRENOGIT126R3, B2).
		if name, carried, same := carriedTreeAt(branch, path); carried {
			if !same {
				discardCarriedTree(branch)
				if r.Held != nil {
					r.Held.discardMade(branch, git)
				} else {
					git(repo, "branch", "-D", branch)
				}
				if r.Held != nil { // a scoped GM's hire is answered as its other refusals once its worktree is made
					return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
				}
				return hirePlace{}, &hireRefusal{Status: 409, Message: "Smolcorp didn't hire: the hire's worktree, " + shownPath(path) +
					", was moved or replaced while it was being made, so what was made there is taken away again. Hire again."}
			}
			place.Worktree = name
		}
		// A scoped GM's hire: the worktree git made is the project's, and the folder asked for is still the one checked
		// (a hire's row keeps it); else the worktree is taken away again, from the project, and nothing starts in it.
		if r.Held != nil {
			if real, held := r.Held.place(given); !held || real != r.Folder || !r.Held.holds(place.Worktree) {
				takeBackTree(from, place.Worktree, place.Branch, git)
				r.Held.discardMade(place.Branch, git)
				return hirePlace{}, &hireRefusal{Status: http.StatusForbidden, Message: "This General Manager " + r.Held.hiresOnly()}
			}
		}
		place.Folder = place.Worktree
		// By this system's separator: on Windows, given is C:\…\repo\sub.
		if rest, found := strings.CutPrefix(given, repo+string(filepath.Separator)); found {
			place.Folder = filepath.Join(place.Worktree, rest)
		}
	} else if inGMFolder {
		place.Note = gmFolderLine
	} else if repo == "" {
		place.Note = notARepo
	}
	if r.Check != nil {
		if err := r.Check(place, given); err != nil {
			if place.Worktree != "" {
				takeBackTree(place.Repo, place.Worktree, place.Branch, git)
			}
			return hirePlace{}, err
		}
	}
	if r.Claude {
		// With symlinks resolved: how Claude Code knows a folder. The very spelling checked above, never the folder's
		// links resolved again, which whoever can write a folder on its way could have turned since: the worktree as
		// carried and checked (carriedTreeAt), or, on a Mac, the folder as the rule judged it (FXGMHIRENOGIT126R5).
		if err := preTrust(trustedSpelling(place), file, tail); err != nil {
			if place.Worktree != "" {
				takeBackTree(place.Repo, place.Worktree, place.Branch, git)
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
		initHireSubmodules(place.Repo, place.Worktree, place.Branch, git)
	}
	made = true
	return place, nil
}

// trustedSpelling is the folder a hire made at place is trusted under, links resolved, as checked at its hire: its
// worktree, which prepareHire gives resolved (the carry's own name on a Mac), or the folder as the rule judged it
// (place.Resolved, on a Mac). Elsewhere, and for a folder whose links couldn't be resolved, physical's, as before.
func trustedSpelling(place hirePlace) string {
	switch {
	case place.Worktree != "":
		return place.Worktree
	case place.Resolved != "":
		return place.Resolved
	}
	return physical(place.Folder)
}

// resolvedFolder is folder as physical gives it, and whether its links were all resolved: physical gives a folder whose
// links it couldn't resolve as it was asked (one not there, or a link turned while it was read, which readlink can
// refuse), and git, started there, would resolve them again itself (FXGMHIRENOGIT126R2, B2). Only on a Mac, where the
// rule it serves is (D2): elsewhere such a folder counts as resolved, as physical gives it, and gets git as before
// (FXGMHIRENOGIT126R3, O1).
func resolvedFolder(folder string) (string, bool) {
	if folder == "" {
		return "", false
	}
	if abs, err := filepath.Abs(folder); err == nil {
		folder = abs
	}
	real, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return folder, runtime.GOOS != "darwin"
	}
	return real, true
}

// startFolder is the folder a hire with no worktree starts in, asked as it was asked for and resolved as physical
// resolves it: on Windows the resolved one, the very spelling it is trusted under in .claude.json (preTrust). A
// working folder keeps the spelling it was given there (a drive letter in lower case, a folder's 8.3 short name), and
// Claude Code looks its trust up by it, so a folder for hires saved as c:\users\ada\projects would never match the
// C:\Users\Ada\Projects that was trusted. A junction isn't resolved: since Go 1.23, EvalSymlinks doesn't follow a
// mount point, so a folder reached through one keeps that spelling, in its start and its trust alike (RT126GMHIRE126
// N2). Elsewhere the kernel gives the resolved folder itself, and the one asked for is kept.
func startFolder(asked, resolved, goos string) string {
	if goos == "windows" && resolved != "" {
		return resolved
	}
	return asked
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

// hiresFolder is where the worktrees of a repo's new hires are made: under worktrees when that is set (and, on a Mac,
// honoured: configuredWorktreesWords); else, on a Mac, in the core's own folder (coreRepoFolder, FXCOREWORKTREES126);
// else beside the repo, as before. A General Manager's own worktree isn't made here (gmTreeFolder).
func hiresFolder(repo, worktrees string) string {
	switch {
	case worktrees != "":
		return filepath.Join(worktrees, filepath.Base(repo))
	case coreWorktreesOn():
		return coreRepoFolder(repo)
	}
	return legacyHiresFolder(repo)
}

// gmTreeFolder is where a General Manager hired into a repo gets its own worktree: beside the repo, as before. No GM's
// own place may be in a folder hires' worktrees are made in, which every GM's Policy denies it whole (gmWorktreesWords).
func gmTreeFolder(repo string) string { return legacyHiresFolder(repo) }

// discardHalfMade clears what a `worktree add` that was stopped may have left:
// its registration, its folder (new, named for this hire alone) and its branch.
// It never deletes a folder that isn't strictly inside parent. On a Mac the folder
// is removed through parent opened as a root (openFolderRoot), never by its path: a
// link put at the folder for hires would lead the removal elsewhere, and is refused
// there, so nothing is removed (FXGMHIRENOGIT126R3, N1).
func discardHalfMade(repo, parent, worktree, branch string, git gitRunner) {
	if !strictlyInside(parent, worktree) {
		return
	}
	git(repo, "worktree", "remove", "--force", worktree)
	if runtime.GOOS != "darwin" {
		os.RemoveAll(worktree)
	} else if rel, err := filepath.Rel(parent, worktree); err == nil && filepath.IsLocal(rel) {
		if root, err := openFolderRoot(parent); err == nil {
			_ = root.RemoveAll(rel)
			root.Close()
		}
	}
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
	deleteHireBranch(repo, branch, git)
	return true
}

// takeBackTree takes away the worktree prepareHire made for a hire, and its branch, once a later step refused the
// hire. On a Mac, where the core carried the tree into a folder of its own (carryWorktree), only through the roots the
// carry kept (discardCarriedTree, removeCarriedTreeFolder), never by the tree's path, which whoever can write the
// folder for hires could have turned into a link since the tree's check: git's status, then its removal, would run
// where that link leads (FXGMHIRENOGIT126R4, R12). The branch goes as removeWorkspace deletes it. Off a Mac the carry
// is on record too, with the tree git checked out where it is: its folder, emptied the same way, goes by worktree, once
// it is still the folder the carry opened (FXCI53). A tree with no carry on record (a stand-in git) is removed as
// before (removeWorkspace).
func takeBackTree(repo, worktree, branch string, git gitRunner) {
	if !discardCarriedTree(branch) {
		removeWorkspace(repo, worktree, branch, git)
		return
	}
	removeCarriedTreeFolder(branch, worktree)
	deleteHireBranch(repo, branch, git)
}

// takeBackWorkedTree takes away the worktree prepareHire made for a hire once its agent may have worked there (the
// engine's hire refused after its start, T3's after its thread was asked for): as removeWorkspace does, keeping a tree
// that holds changes, so nothing the agent did is lost. On a Mac, while the carry is on record, only while the tree's
// path and the carry's name are still the very folder carried (carriedTreeAt): a tree turned since is kept, with its
// branch, and no git runs on its path, which leads elsewhere (FXGMHIRENOGIT126R5, R14). It says whether it was taken.
func takeBackWorkedTree(repo, worktree, branch string, git gitRunner) bool {
	if _, carried, same := carriedTreeAt(branch, worktree); carried && !same {
		log.Printf("smolcorp: the worktree %s is kept: it was moved or replaced since it was made, so no git runs there", worktree)
		return false
	}
	return removeWorkspace(repo, worktree, branch, git)
}

// deleteHireBranch deletes a hire's branch once its worktree is gone, when your checkout's HEAD holds it.
func deleteHireBranch(repo, branch string, git gitRunner) {
	// Only a branch your checkout's HEAD holds is deleted, as before it had an upstream: `branch -d` alone would also
	// take one its upstream holds (pushed, but not merged here), and keep one HEAD holds whose upstream is behind
	// (pushed, then more merged here). With HEAD's look passed, -D deletes just what -d did before the upstream. The
	// branch is named in full: by its short name, a tag of that name would be looked at instead (RT126SAX7R2's F2).
	if status, _, ok := git(repo, "merge-base", "--is-ancestor", "refs/heads/"+branch, "HEAD"); ok && status == 0 {
		git(repo, "branch", "-D", branch)
	}
}

// allocatePorts is the first range of portsPerHire ports that no taken range
// touches and that are all free, written "4100-4103"; "" for none.
// hirePortsHeld are the port ranges hires still under way took (allocateHirePorts), each with when: the owner's hire
// and one the General Manager asked for may now be made at once (FXCOREWORKTREES126, item 5), and neither's row is
// written yet when the other picks its ports. Held until its maker lets go (releaseHirePorts: its row written, or the
// hire refused), or for hirePortsHold at most.
var hirePortsHeld = struct {
	sync.Mutex
	at map[string]time.Time
}{at: map[string]time.Time{}}

const hirePortsHold = 5 * time.Minute

// allocateHirePorts is allocatePorts with the ranges other hires under way took counted as taken, and the range it
// gives held for this hire the same way.
func allocateHirePorts(taken []string, free func(int) bool) string {
	hirePortsHeld.Lock()
	defer hirePortsHeld.Unlock()
	for ports, at := range hirePortsHeld.at {
		if time.Since(at) > hirePortsHold {
			delete(hirePortsHeld.at, ports)
		} else {
			taken = append(slices.Clip(taken), ports)
		}
	}
	ports := allocatePorts(taken, free)
	if ports != "" {
		hirePortsHeld.at[ports] = time.Now()
	}
	return ports
}

// releaseHirePorts lets go of the range allocateHirePorts held for a hire, once its row holds it or it was refused.
func releaseHirePorts(ports string) {
	hirePortsHeld.Lock()
	defer hirePortsHeld.Unlock()
	delete(hirePortsHeld.at, ports)
}

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
// would bill a key rather than the account; "" if none. Each is read without
// waiting (cpReadData, FXGMHIRENOGIT126): a FIFO, socket or device there, or a link
// to one, is no settings file and counts as absent, so nothing holds the hire on it.
// unread is a plain file too big to judge (settingsMost), which refuses the hire.
func apiKeyHelper(places ...string) (helper, unread string) {
	for _, place := range places {
		if place == "" {
			continue
		}
		for _, name := range []string{".claude/settings.json", ".claude/settings.local.json", "settings.json", "settings.local.json"} {
			file := filepath.Join(place, name)
			if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
				continue // not there, or no plain file: never opened
			} else if info.Size() > settingsMost {
				return "", file
			}
			data, err := cpReadData(file, settingsMost)
			var root map[string]json.RawMessage
			if err == nil && json.Unmarshal(data, &root) == nil {
				if _, has := root["apiKeyHelper"]; has {
					return file, ""
				}
			}
		}
	}
	return "", ""
}

// settingsMost is the largest settings file apiKeyHelper reads: far more than any Claude Code settings file holds.
const settingsMost = 4 << 20
