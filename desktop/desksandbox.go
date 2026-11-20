//go:build !windows

package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A desk's sandbox at its start (SA-M6, SANDBOX-PLAN §A3.1 and §A5.2): what the core knows of where the desk works,
// as the sandbox's Places; its Policy, made from them; X12's start-time checks on that Policy; and the checks its
// `smolcorp sandbox enter` runs. The start itself is each system's start-only runner (desksandbox_darwin.go on a
// Mac), which the desk's hireTmux runs for its start call alone (hiretmux.go).
//
// On a Mac every desk the engine starts is started in its sandbox: hires, the GM, and a desk Hand back resumes. One
// that can't be is refused, with §A5.2's words; none falls back to a start outside it (AQ1). Linux's start is SA-L2
// and SA-L4's: until they land, a desk there starts as before.

// deskSandboxed is whether the engine starts its desks in their sandbox on this system.
const deskSandboxed = runtime.GOOS == "darwin"

// deskAt is what a desk's start knows of it, for its Places.
type deskAt struct {
	ID string
	GM bool
	// Root is where it works: its worktree, or the folder it was hired into. Checkout is the owner's checkout Root is
	// a worktree of, "" for none; Hires where the hires' worktrees go, "" for none.
	Root, Checkout, Hires string
	// Projects is, for a GM hired into a folder that isn't a repo checkout or worktree, that folder: where it starts
	// and what it reads, never writes. Root is then its workspace (gmWorkspace). "" for any other desk.
	Projects string
	// Socket is its own tmux socket (deskSocketPath).
	Socket string
	// Accounts are the Claude account folders its fence denies besides ~/.claude.
	Accounts []claudeProfile
	// Brief is its brief, "" for none: its check reads it.
	Brief string
	Git   gitRunner
	// Programs are the programs its start runs besides those the core finds (deskFinding): a plain hire's agent, as
	// its launcher resolved it (SA-M10), so its install root is the desk's to read and never to write.
	Programs []sandbox.Program
	// DatabaseOptIn is AQ21's opt-in as the owner set it (sandboxchecks.go): its start checks the databases only then.
	DatabaseOptIn bool
}

// startedInSandbox is whether tmux's start was a desk's in its sandbox, with the server the core checked at it
// (§A5.2 step 9): what a desk's row tags it with (Hire.Sandboxed, SA-X13).
func startedInSandbox(tmux *hireTmux) bool { return tmux.sandboxed && tmux.server > 1 }

// deskSandbox is one desk's sandbox, made before its start: its Places and its Policy, and what its start checks.
type deskSandbox struct {
	at     deskAt
	places sandbox.Places
	policy *sandbox.Policy
}

// errSandboxNotMade is a start refused before anything of it ran: its Places, its Policy or X12's checks.
type errSandboxNotMade struct{ words string }

func (e *errSandboxNotMade) Error() string { return e.words }

// makeDeskSandbox makes desk d's Places and Policy, and runs X12's start-time checks on them: no link in the shared
// .git's refs/ and logs/ or on the way to a base branch's name, and no held symbolic ref a desk could move through
// (CheckRefPlaces), and the link check (checkLinks), whose words refuse the start, and the old git
// plants, kept for Setup (findOldGitPlants), with git's substitutions, whose words refuse the start until the owner
// removes or keeps each one (gitPlantsAt). Nothing of the desk runs yet. The sandbox is kept for the desk's stop
// (deskSandboxOf).
func makeDeskSandbox(d deskAt) (*deskSandbox, error) {
	if d.Projects != "" {
		// A GM's workspace, its Root (FXGMROOT): made at its hire, there at every start after.
		if err := makeGMWorkspace(d.Root); err != nil {
			log.Printf("sandbox: desk %s's workspace: %v", d.ID, err)
			return nil, &errSandboxNotMade{sandboxNotMade}
		}
	}
	// An agent's place no desk's profile can hold (the top of the disk, a control character, a `..`), in words that name
	// its variable (FXAGENTHOMEMACR1): the owner's to fix, not the sandbox's. One spelled with a `..` was unset at the
	// core's start, and only its name kept (agentPlacesDropped, FXAGENTHOMEMACR3).
	variable := sandbox.AgentPlaceRefused(os.Getenv, physicalBelow)
	if len(agentPlacesDropped) > 0 {
		variable = agentPlacesDropped[0]
	}
	if runtime.GOOS == "darwin" && variable != "" {
		log.Printf("sandbox: desk %s's places: %s names a place no profile can hold", d.ID, variable)
		return nil, &errSandboxNotMade{agentPlaceWords(variable)}
	}
	pl, err := deskPlaces(d)
	if err != nil {
		log.Printf("sandbox: desk %s's places: %v", d.ID, err)
		// An ignore file spelled from another account's home or git's own folder, in words that name the setting
		// (FXAGENTHOMEMACR2, RT126AGENTHOMEMACR1 O1).
		var excludes *sandbox.ExcludesFileError
		if errors.As(err, &excludes) {
			return nil, &errSandboxNotMade{excludesFileWords(excludes.Value)}
		}
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	// SA-M12's checks, at each start: whether Remote Login answers (its SSH keys denied then), and, with AQ21's
	// opt-in on, whether every local database refuses a password-less login (deskServices reads it). Setup's Sandbox
	// row shows both (SA-X14).
	pl.RemoteLogin = deskStartChecks(d.ID, d.DatabaseOptIn).RemoteLogin
	// Its mark on a Mac, made at its first start, which its stop's reap tells its processes by (FXFIREREAPR5,
	// deskreap.go).
	if pl.ReapMark, err = deskReapMark(hireFolder(supportDir, d.ID)); err != nil {
		log.Printf("sandbox: desk %s's mark: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	pol, err := sandbox.New(pl)
	// A desk at the projects folder, or one that holds it or home, whose support folder too (FXHIREPROJECTSR4).
	if errors.Is(err, sandbox.ErrProjectsWhole) || errors.Is(err, sandbox.ErrRootHasSupport) {
		if words := wholeProjectsWords(pl.Root, true); words != "" {
			return nil, &errSandboxNotMade{words}
		}
	}
	if err != nil {
		log.Printf("sandbox: desk %s's policy: %v", d.ID, err)
		var big *sandbox.HomeTooBigError
		if errors.As(err, &big) {
			return nil, &errSandboxNotMade{homeTooBig(big)}
		}
		// A place one of the agent place variables names that couldn't be looked through (a folder of root's it
		// can't read, `XDG_CONFIG_HOME=/private`), in words that name the variable (FXAGENTHOMEMACR2, N1).
		var links *sandbox.HomeLinksError
		if errors.As(err, &links) && runtime.GOOS == "darwin" {
			if variable := sandbox.AgentPlaceOf(os.Getenv, physicalBelow, pl.Home, links.At); variable != "" {
				return nil, &errSandboxNotMade{agentPlaceWords(variable)}
			}
		}
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	// A link in refs/ or logs/, before the link check, whose walk would follow one on the way and pass one at a
	// held file (RT126FIXREVIEW-GITROWS2R1's and R2's B1).
	if err := pol.CheckRefPlaces(); err != nil {
		var at *sandbox.RefPlaceError
		if errors.As(err, &at) {
			return nil, &errSandboxNotMade{refPlaceRefusal(at)}
		}
		log.Printf("sandbox: desk %s's ref folders: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	// The git folders in its place, on a Mac, whatever made its row (the office, Setup, the GM, a pinned or recent
	// folder, a plain hire's launcher): a bare repo there, a repo in a place in no repo (FXHIREPROJECTSR5, B1) or a walk
	// that can't finish refuses the start; every other .git below it its Policy holds, the folders on the way to each
	// by name (FXHIREPROJECTSR6). A .git there that the last start didn't list and the desk could have moved in, or a
	// link there to a repo a desk can change, refuses the start too; the list is kept for the next (FXHIREPROJECTSR7).
	// The clones and mirrors in a package manager's folders are held one by one; a link to a folder that holds a repo a
	// desk can change, or into a package manager's folder that holds one, and a gitlink staged or committed in a hire's
	// own worktree at a path where git finds a repo, refuse the start (FXHIREPROJECTSR8; the path as git spells it,
	// FXHIREPROJECTSR9). A GM's workspace is its own.
	if d.Projects == "" && runtime.GOOS == "darwin" {
		listed, before := gitsListed(d.ID, pl.Root)
		words, found := gitPlaceWords(pl.Root, pl.GitDir != "", true, before)
		if words != "" {
			return nil, &errSandboxNotMade{words}
		}
		if before {
			then := pl
			then.GitBelow = listed
			was, err := sandbox.New(then)
			if words := gitsSinceWords(was, err, pl.Root, listed, found.gits); words != "" {
				return nil, &errSandboxNotMade{words}
			}
		}
		if len(found.gits) > 0 || len(found.tools) > 0 {
			pl.GitBelow, pl.ToolsGitBelow = found.gits, found.tools
			if pol, err = sandbox.New(pl); err != nil {
				log.Printf("sandbox: desk %s's policy with the git folders below its place: %v", d.ID, err)
				return nil, &errSandboxNotMade{sandboxNotMade}
			}
		}
		if words := gitLinkWords(pol, pl, found, d.ID); words != "" {
			return nil, &errSandboxNotMade{words}
		}
		if pl.OwnWorktree != "" && pl.Checkout != "" {
			if words := gitlinkWords(d.Git, pl.Root, pl.Checkout, filepath.Join(pl.GitDir, "worktrees", pl.OwnWorktree), found); words != "" {
				return nil, &errSandboxNotMade{words}
			}
		}
		if err := listGits(d.ID, pl.Root, found.gits); err != nil {
			log.Printf("sandbox: desk %s's list of git folders: %v", d.ID, err)
			return nil, &errSandboxNotMade{sandboxNotMade}
		}
	}
	// Something a desk could have mounted before its profile denied mounting, at or inside a narrow place its Policy
	// holds, or a share over a plant (FXSWEEP3P1, RT126SWEEP3 P1's F1; FXSWEEP3P1R1, FXSWEEP3P1R2): no desk starts
	// while it hides what's there.
	if words := mountedWords(pol, pl.Home); words != "" {
		log.Printf("sandbox: desk %s's start: %s", d.ID, words)
		return nil, &errSandboxNotMade{words + ", so no desk starts while it's there."}
	}
	if _, err := checkLinks(pol, pl); err != nil {
		// Too many files to look at in a place one of the agent place variables names (`OPENCODE_DB=/Users`), in words
		// that name the variable (FXAGENTHOMEMACR3, RT126AGENTHOMEMACR2 O1).
		var many *tooManyToLook
		if errors.As(err, &many) && runtime.GOOS == "darwin" {
			if variable := sandbox.AgentPlaceOf(os.Getenv, physicalBelow, pl.Home, many.at); variable != "" {
				return nil, &errSandboxNotMade{agentPlaceWords(variable)}
			}
		}
		var words *linkCheckError
		if errors.As(err, &words) {
			return nil, &errSandboxNotMade{words.words}
		}
		log.Printf("sandbox: desk %s's link check: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	if err := checkSharedObjects(pl.GitDir); err != nil { // a local clone's objects (FXHIREPROJECTSR3)
		var words *linkCheckError
		if errors.As(err, &words) {
			return nil, &errSandboxNotMade{words.words}
		}
		log.Printf("sandbox: desk %s's objects check: %v", d.ID, err)
		return nil, &errSandboxNotMade{sandboxNotMade}
	}
	if pl.GitDir != "" {
		// With git's substitutions there that the owner hasn't answered, the desk doesn't start (gitsubst.go).
		listed, held := gitPlantsAt(pl.GitDir)
		keepOldGitPlants(d.ID, pl.GitDir, listed)
		if len(held) > 0 {
			return nil, &errSandboxNotMade{gitSubstRefusal(held)}
		}
	}
	s := &deskSandbox{at: d, places: pl, policy: pol}
	deskStartListing(s)
	keptSandboxes.Lock()
	keptSandboxes.of[d.ID] = s
	keptSandboxes.Unlock()
	return s, nil
}

// refPlaceRefusal is the owner's words for a start CheckRefPlaces refused: what is at e.At, in the shared .git, lets a
// desk move a branch Smolcorp keeps from it. The start is refused, not listed for Setup as an old git plant is: while
// it is there, the rows that hold the branches at it or below it hold nothing.
func refPlaceRefusal(e *sandbox.RefPlaceError) string {
	where := "where git keeps this repository's branches and their history"
	worktree, inWorktrees := strings.CutPrefix(e.Ref, "worktrees/")
	atTop := !inWorktrees && e.Ref != "HEAD" && !strings.HasPrefix(e.Ref, "refs/") && !strings.HasPrefix(e.Ref, "logs/")
	switch {
	case inWorktrees:
		where = "where git keeps what it knows of a worktree"
	case !strings.HasPrefix(e.Ref, "refs/") && !strings.HasPrefix(e.Ref, "logs/"):
		where = "where git looks for a branch's name before its own branches"
	}
	// A symbolic ref's fix never writes the same text back: a desk may have chosen it (RT126FIXREVIEW-GITROWS2R3's B1).
	ownRef := fmt.Sprintf("check what it holds now (git log %s), then make %s a ref of its own at the commit you want (git update-ref --no-deref %s <commit>), or remove it (git update-ref -d --no-deref %s)", e.Symref, e.Ref, e.Ref, e.Ref)
	switch {
	case e.Symref != "" && e.Link && e.Ref == "refs/remotes/origin/HEAD":
		return fmt.Sprintf("%s is a link, %s, which git reads as a symbolic ref to %s. Git writes one like this only when core.preferSymlinkRefs is on, which git 3.0 drops, and Smolcorp can't tell it from a link a desk left, so the desk didn't start. Turn that setting off, run git remote set-head origin --auto in the repository, which asks origin again and writes it as a plain file, then start the desk again.", e.At, where, e.Symref)
	case e.Symref != "" && e.Link:
		return fmt.Sprintf("%s is a link, %s, which git reads as a symbolic ref to %s. Git writes one like this only when core.preferSymlinkRefs is on, which git 3.0 drops, and Smolcorp can't tell it from a link a desk left to move a branch it keeps, so the desk didn't start. Turn that setting off, %s, then start the desk again.", e.At, where, e.Symref, ownRef)
	case e.Symref != "":
		to := e.Symref
		if e.Moves != "" && e.Moves != e.Symref {
			to += ", and on to " + e.Moves
		}
		// "could" where the way can't be followed: Smolcorp doesn't know that a desk can move it (RT126FIXREVIEW-GITROWS2R4's O3).
		moved, would := "which a desk can move", "would"
		if e.Moves == "" {
			moved, would = "which Smolcorp can't follow to a ref no desk can move", "could"
		}
		if e.Ref == "HEAD" {
			return fmt.Sprintf("Your checkout's HEAD (%s) points at %s, %s, so a desk's work there %s move your checkout too, and the desk didn't start. Check what it holds (git log %s), then switch your checkout to a branch of your own, then start the desk again.", e.At, to, moved, would, e.Symref)
		}
		if name, ok := strings.CutSuffix(worktree, "/HEAD"); inWorktrees && ok {
			return fmt.Sprintf("The HEAD of the worktree git knows as %s (%s) points at %s, %s, so a desk's work there %s move that worktree too, and the desk didn't start. Find it with git worktree list, check what it holds (git log %s), then switch that worktree to a branch of its own, or remove it if you don't need it (git worktree remove), then start the desk again.", name, e.At, to, moved, would, e.Symref)
		}
		// A name at the top of the .git (FXGITROWS2R6): git reads it before the branch of that name, and update-ref
		// doesn't take it, so the fix is to remove the file.
		if !strings.HasPrefix(e.Ref, "refs/") && !inWorktrees {
			return fmt.Sprintf("%s, in the repository's .git folder, isn't a branch: git reads it before any branch called %s, and it points at %s, %s, so a desk's work there %s change what your git takes for %s, and the desk didn't start. Check what it holds (git log %s), then remove that file (rm '%s'), then start the desk again.", e.Ref, e.Ref, to, moved, would, e.Ref, e.Symref, e.At)
		}
		return fmt.Sprintf("%s isn't a ref of its own: it points at %s, %s, so a desk's work there %s move %s too, and the desk didn't start. In the repository, %s, then start the desk again.", strings.TrimPrefix(e.Ref, "refs/heads/"), to, moved, would, e.Ref, ownRef)
	// A link at one of git's own names at the top of the .git that leads where a desk writes, or can't be followed
	// (FXGITROWS2R8), or whose way goes through a link a desk can change (FXGITROWS2R9): what git reads or runs through it
	// would be a desk's.
	case e.Link && (e.Leads != "" || e.Unfollowed || e.Via != ""):
		return ownLinkRefusal(e)
	// A name at the top of the .git that holds a commit's id, is a link or has a second name (FXGITROWS2R7): git reads
	// it before the branch of that name; it is no branch, so the fix is to remove it.
	case e.Commit:
		return fmt.Sprintf("%s, in the repository's .git folder, isn't a branch: it holds a commit's id, and git reads it before any branch called %s, so your git takes that commit for %s, with at most an \"ambiguous\" warning, and the desk didn't start. Check which commit it is (cat '%s'), then remove that file (rm '%s'), then start the desk again.", e.Ref, e.Ref, e.Ref, e.At, e.At)
	case e.Link && atTop:
		return fmt.Sprintf("%s is a link, %s: git reads what it leads to before a branch called %s, or one whose name starts %s/, so a desk could change what your git takes for that branch by writing where it leads, and the desk didn't start. Git doesn't make links there. See where it leads with ls -l '%s' and check what's in it, then remove the link (rm '%s'), then start the desk again.", e.At, where, e.Ref, e.Ref, e.At, e.At)
	// One of git's own files at the top with a second name (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B2).
	case e.Hard && atTop && e.Own:
		return ownHardRefusal(e)
	case e.Hard && atTop:
		return fmt.Sprintf("%s, in the repository's .git folder, isn't a branch, and it has another name elsewhere (a hard link): git reads it before any branch called %s, so a desk could change what your git takes for %s by writing that other name, and the desk didn't start. Git doesn't make hard links there. Check what it holds (cat '%s'), then remove that file (rm '%s'), then start the desk again.", e.Ref, e.Ref, e.Ref, e.At, e.At)
	case e.Link && inWorktrees:
		return fmt.Sprintf("%s is a link, %s, so a desk could move that worktree's HEAD through it, and the desk didn't start. Git doesn't make links there. See where it leads with ls -l '%s', and once you're sure it's yours, put it in the link's place, or remove the link and run git worktree prune, then start the desk again.", e.At, where, e.At)
	case e.Link:
		return fmt.Sprintf("%s is a link, %s, so a desk could move a branch Smolcorp keeps through it, and the desk didn't start. Git doesn't make links there. See where it leads with ls -l '%s' and check what's in it: once you're sure it's yours, put it in the link's place, then start the desk again.", e.At, where, e.At)
	case e.Hard && e.Ref == "HEAD":
		return fmt.Sprintf("Your checkout's HEAD (%s) has another name elsewhere (a hard link), so a desk could point your checkout at its own branch by writing that other name, and the desk didn't start. Git doesn't make hard links there. Check which branch it names (git symbolic-ref HEAD), give it a copy of its own (cp it to a new name, then mv that over it), then start the desk again.", e.At)
	case e.Hard && inWorktrees && strings.HasSuffix(worktree, "/HEAD"):
		return fmt.Sprintf("The HEAD of the worktree git knows as %s (%s) has another name elsewhere (a hard link), so a desk could point that worktree at its own branch by writing that other name, and the desk didn't start. Git doesn't make hard links there. Find it with git worktree list, give that HEAD a copy of its own (cp it to a new name, then mv that over it), then start the desk again.", strings.TrimSuffix(worktree, "/HEAD"), e.At)
	case e.Hard:
		return fmt.Sprintf("%s has another name elsewhere (a hard link), %s, so a desk could move a branch Smolcorp keeps by writing that other name, and the desk didn't start. Git doesn't make hard links there. Check what it holds, give it a copy of its own (cp it to a new name, then mv that over it), then start the desk again.", e.At, where)
	}
	return fmt.Sprintf("%s isn't a folder or a file, %s, so the desk didn't start. Remove it, then start the desk again.", e.At, where)
}

// gitReadsThrough is what git reads or runs through a link at one of its own names at the top of the .git, in the
// owner's words, as a sentence's start (ownLinkRefusal; RT126FIXREVIEW-GITROWS2R8's O1 for hooks, lfs and the rest).
var gitReadsThrough = map[string]string{
	"hooks":           "Git runs the hooks it finds there",
	"config":          "Git reads this repository's settings there, and they can name programs for it to run",
	"config.worktree": "Git reads this repository's settings there, and they can name programs for it to run",
	"info":            "Git reads this repository's own ignore and attributes rules there",
	"rr-cache":        "Git reads the conflict resolutions it reuses in a merge (rerere) there",
	"MERGE_AUTOSTASH": "When a merge ends, git puts the changes it keeps there back into your files",
	"FETCH_HEAD":      "Git reads the result of your last fetch there, and git merge FETCH_HEAD merges it",
	"lfs":             "Git reads the large files git-lfs keeps there",
}

// gitReads is gitReadsThrough's sentence for name, or a plain one for a name it doesn't list.
func gitReads(name string) string {
	if what, ok := gitReadsThrough[name]; ok {
		return what
	}
	return "Git reads its own files there"
}

// ownLinkRefusal is the owner's words for a link at one of git's own names at the top of the .git that leads where a
// desk can write, or can't be followed (FXGITROWS2R8, RT126FIXREVIEW-GITROWS2R7's B1), or whose way goes through a link a
// desk can change (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B1). A desk's sandbox holds only what Smolcorp knows to
// hold, so a folder outside the projects isn't enough: hooks are kept from desks once git's config names them
// (core.hooksPath), and the rest is the repository's own, so a copy goes in the link's place.
func ownLinkRefusal(e *sandbox.RefPlaceError) string {
	what := gitReads(e.Ref)
	leads := e.Leads
	if leads == "" {
		leads = "a place Smolcorp can't read"
	}
	fix := fmt.Sprintf("Check where it leads (ls -l '%s'), then remove the link (rm '%s'): git makes its own %s when it needs one.", e.At, e.At, e.Ref)
	switch e.Ref {
	case "hooks":
		fix = fmt.Sprintf("Remove the link (rm '%s'). To keep using the hooks in that folder, name it with git config core.hooksPath instead: Smolcorp keeps every desk out of the folder your git config names for hooks.", e.At)
	case "lfs":
		fix = fmt.Sprintf("Remove the link (rm '%s'). To keep git-lfs's files where it leads, name that folder with git config lfs.storage instead.", e.At)
	case "config", "config.worktree", "info", "description":
		fix = fmt.Sprintf("Check what it leads to, then put a copy of it in the link's place (rm '%s', then cp -R '%s' '%s').", e.At, leads, e.At)
		if e.Leads == "" {
			fix = fmt.Sprintf("Check where it leads (ls -l '%s'), then put a copy of what it leads to in the link's place.", e.At)
		}
	}
	switch {
	case e.Via == e.At:
		return fmt.Sprintf("%s, in the repository's .git folder, is a link a desk can change or remove, so where it leads is the desk's to choose. %s, so a desk could change what your git runs or applies there, and the desk didn't start. Git doesn't make links there. %s Then start the desk again.", e.At, what, fix)
	case e.Via != "":
		return fmt.Sprintf("%s, in the repository's .git folder, is a link whose way goes through another link, %s, which a desk can change, so where it leads is the desk's to choose, wherever it leads now. %s, so a desk could change what your git runs or applies there, and the desk didn't start. %s Then start the desk again.", e.At, e.Via, what, fix)
	case e.Unfollowed:
		return fmt.Sprintf("%s, in the repository's .git folder, is a link to %s, which isn't there or can't be read, so Smolcorp can't tell whether a desk could write there. %s, so a desk could change what your git runs or applies there, and the desk didn't start. See where it leads with ls -l '%s', then remove the link (rm '%s') or put back what it leads to, then start the desk again.", e.At, leads, what, e.At, e.At)
	}
	return fmt.Sprintf("%s, in the repository's .git folder, is a link to %s, where a desk can write. %s, so a desk could change what your git runs or applies there, and the desk didn't start. %s Then start the desk again.", e.At, leads, what, fix)
}

// gitsOwnKeptFiles are git's own files at the top of the .git whose loss loses something of the owner's: the
// repository's settings, description, index, packed branches and shallow history. The rest is git's state, which it
// makes again when it needs it (ownHardRefusal).
var gitsOwnKeptFiles = []string{"config", "config.worktree", "description", "index", "packed-refs", "shallow"}

// ownHardRefusal is the owner's words for one of git's own files at the top of the .git that has another name, a hard
// link (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B2): through it a desk would rewrite the file in place. The fix is a
// hard link at the top's: remove it, but where its loss would lose the owner's settings or branches, a copy of its own.
func ownHardRefusal(e *sandbox.RefPlaceError) string {
	what := "Git reads it as one of its own files"
	if w, ok := gitReadsThrough[e.Ref]; ok {
		what = w
	}
	fix := fmt.Sprintf("Check what it holds (cat '%s'), then remove that file (rm '%s'): git makes it again when it needs it.", e.At, e.At)
	if slices.Contains(gitsOwnKeptFiles, e.Ref) {
		fix = fmt.Sprintf("Give it a copy of its own (cp '%s' '%s.own', then mv '%s.own' '%s').", e.At, e.At, e.At, e.At)
	}
	return fmt.Sprintf("%s, in the repository's .git folder, has another name elsewhere (a hard link). %s, so a desk could change what your git runs or applies there by writing that other name, and the desk didn't start. Git doesn't make hard links there. %s Then start the desk again.", e.At, what, fix)
}

// homeTooBig is the owner's words for a start refused because a folder of theirs that desks can't change was too big
// to look through for links (sandbox.HomeTooBigError, FXPLANTSTERMR4): where the look stopped, and what to do there.
// Updating Smolcorp wouldn't help. The count is the folder's own, said to be, when the look stopped in a folder inside
// it (RT126POLISHAR2's O2), or, for the folders looked through on one budget together, all of theirs (InAll): the folder
// named may hold far fewer. What to do fits the folder: old sessions, caches and logs fill a store, but in Claude
// Code's skills, commands, agents and hooks (Loaded), it is mostly what a skill brings with it, looked through wherever
// it is kept if a link leads there, so moving it out and linking it back wouldn't help (RT126POLISHAR1's and
// RT126POLISHAR2's O1).
func homeTooBig(e *sandbox.HomeTooBigError) string {
	count := "after " + groupedCount(e.Most) + " files and folders"
	switch {
	case e.InAll:
		count = "after looking through " + groupedCount(e.Most) + " files and folders in all"
	case e.Inside != e.Folder:
		count += " in " + tildePath(e.Folder)
	}
	todo := "Delete or move out what you no longer need there (old sessions, caches, logs)"
	if e.Loaded {
		todo = "It looks through all of your skills, commands, agents and hooks, and wherever a link in them leads, a " +
			"skill's own dependencies included (a node_modules folder, a Python venv). Remove what you don't need, or keep " +
			"it somewhere no link in " + tildePath(e.Account) + " leads"
	}
	return "Smolcorp didn't start this hire. At each start it looks for links in the folders of yours that hires can't " +
		"change, and it stopped in " + tildePath(e.Inside) + " " + count + ", the most it " +
		"looks through. " + todo + ", then start the hire again."
}

// agentPlacesDropped are the names of the agent place variables the core was started with a value of no desk's profile
// can hold, spelled with a `.` or `..` in it or a doubled `/` (sandbox.AgentPlaceUnclean): unset at the core's start on
// a Mac, by it (unsetAgentPlacesNotWhole) or by the Mac app (agentPlacesDroppedVariable), so nothing of the core reads
// them, and kept here so every desk's start is refused in the first one's words (FXAGENTHOMEMACR3, RT126AGENTHOMEMACR2
// B1). Set once, before the core serves.
var agentPlacesDropped []string

// agentPlacesDroppedVariable is how the Mac app tells its core the names, never the values, of the agent place
// variables it dropped for such a spelling (Engine.swift, GoCore.environment). Comma-separated.
const agentPlacesDroppedVariable = "SMOLCORP_AGENT_PLACES_DROPPED"

// agentPlaceWords is the owner's words for a start refused because variable, one the Mac app passes the core
// (sandbox.AgentPlaces), puts an agent's folder or sign-in, or the config folder, where no desk's sandbox can hold it
// (AgentPlaceRefused, agentPlacesDropped), or somewhere the start couldn't look through (AgentPlaceOf, FXAGENTHOMEMACR2):
// it names the variable and says to fix or unset it. Turning the sandbox off isn't the fix, and updating doesn't help.
func agentPlaceWords(variable string) string {
	return "Smolcorp didn't start this hire: " + variable + " puts a folder of yours where a hire's sandbox can't keep " +
		"it (the top of your Mac's disk, a folder Smolcorp can't look all through, a path with . or .. or // in it, or " +
		"one with a line break or another control character in it). Fix " + variable + " or unset it (launchctl " +
		"unsetenv " + variable + " too, if you set it there), then quit and reopen Smolcorp and start the hire again."
}

// excludesFileWords is the owner's words for a start refused because their git config's ignore file,
// core.excludesFile, is value, spelled from another account's home folder (`~name/…`) or from where git is installed
// (`%(prefix)/…`): Smolcorp can't tell which file that is, so it can't keep a hire from writing it (FXAGENTHOMEMACR2,
// RT126AGENTHOMEMACR1 O1). They say how to spell it instead, and to quit and reopen Smolcorp: the core asks for the
// ignore file once for its life and carries it into its reads (gitHere), so the value it carries is the one refused
// until then (FXAGENTHOMEMACR3, RT126AGENTHOMEMACR2 N1).
func excludesFileWords(value string) string {
	form := "from another account's home folder (~name/…)"
	if strings.HasPrefix(value, "%(") {
		form = "from the folder git is installed in (%(prefix)/…)"
	}
	return "Smolcorp didn't start this hire: your git config's core.excludesFile is " + strconv.Quote(value) + ", a path " +
		form + ", and Smolcorp can't tell which file that is, so it can't keep a hire from changing what your git " +
		"ignores. Spell it as a whole path (starting with /) or from your own home folder (~/…), with git config " +
		"--global core.excludesFile, then quit and reopen Smolcorp and start the hire again."
}

// groupedCount is n with a comma between each group of three digits, as English writes a count.
func groupedCount(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// keptSandboxes are the sandboxes of the desks this core started, by id, for their stop. The old git plants each
// one's start found (X12) are kept for Setup's Sandbox row (oldGitPlants, sandboxrow.go). Gone when the core is.
var keptSandboxes = struct {
	sync.Mutex
	of map[string]*deskSandbox
}{of: map[string]*deskSandbox{}}

// deskSandboxOf is desk id's sandbox as its start made it, kept by this core until the desk's stop. It doesn't say
// whether the desk runs (a desk started before the core's restart has none kept): deskStoppedNow does.
func deskSandboxOf(id string) (*deskSandbox, bool) {
	keptSandboxes.Lock()
	defer keptSandboxes.Unlock()
	s, ok := keptSandboxes.of[id]
	return s, ok
}

// deskStartListing is where a desk's start lists what it may plant before it runs: CLAUDEPLANT1's
// holdClaudePlants(s.places.OwnID, claudeLooks(s.policy, s.places), s.places.Checkout) goes here when it merges.
func deskStartListing(s *deskSandbox) {}

// deskStopped is said once desk h's session is known to have ended: a start that failed, a watch that gave up, an
// Attach failure, Take over's stop, and the engine's fire once it saw the session gone (hirefire.go, beside
// CLAUDEPLANT1's claudeHireEnded(h.ID)). What it may have planted is listed at its stop (deskStopListing), with its
// sandbox as its start made it: the one kept, or, for a desk this core didn't start (one from before the core's
// restart), one made again from its row and not kept. Then its sandbox is let go, and its socket folder taken away
// once nothing answers there: nothing of a stopped desk stays kept.
//
// First, on a Mac, every program it started still running in its sandbox is ended, and seen ended (deskReap,
// FXFIREREAPR1): its tmux server's end doesn't end what its panes started in the background. One that can't be stops
// it there, with nothing let go, and its words are said, for the caller to keep what it would have taken away (its
// row, its worktree, its folders) and say so: "" otherwise.
//
// Then, with every program ended, something a desk could have mounted at or inside a narrow place its sandbox holds,
// or a share over a plant, stops it there too (mountedWords, FXSWEEP3P1, FXSWEEP3P1R1, FXSWEEP3P1R2): the listing would read what the mount shows, not what's
// there, and a fire would go on to look at the repo through it.
func deskStopped(h Hire, o engineOptions) string {
	if why := deskReap(h); why != "" {
		return why
	}
	keptSandboxes.Lock()
	s, kept := keptSandboxes.of[h.ID]
	keptSandboxes.Unlock()
	if !kept {
		s = rowDeskSandbox(h.ID, o)
	}
	if s != nil {
		if why := mountedWords(s.policy, s.places.Home); why != "" {
			log.Printf("sandbox: desk %s at its stop: %s", h.ID, why)
			return why
		}
	}
	keptSandboxes.Lock()
	delete(keptSandboxes.of, h.ID)
	keptSandboxes.Unlock()
	if s != nil {
		deskStopListing(s)
	}
	removeDeskSocketFolder(h.TmuxSocket)
	deskChecksForgotten(h.ID)
	return ""
}

// deskMounts are the owner's own file systems, where and of which type, read at each desk's start and stop
// (sandbox.OwnersMounts). Tests set it.
var deskMounts = sandbox.OwnersMounts

// mountedBegins begins mountedWords' words, whichever they are.
const mountedBegins = "Something "

// mountedWords is "" when nothing a desk could have mounted is at or inside a narrow place pol holds, or over a plant
// (MountedOver), and otherwise the owner's words for it, to be followed by what was or wasn't done (", so …"): what is
// mounted where, inside or over what (FXSWEEP3P1R2), and how to take it away (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B1:
// the owner's own mounts aren't counted, so the words don't ask whether it's theirs). A mount table that can't be read
// is said too. On a Mac.
func mountedWords(pol *sandbox.Policy, home string) string {
	if runtime.GOOS != "darwin" || pol == nil {
		return ""
	}
	mounts, err := deskMounts()
	if err != nil {
		log.Printf("sandbox: the mount table: %v", err)
		return mountedBegins + "may be mounted over a place no desk may change: Smolcorp couldn't read what is mounted on this Mac"
	}
	at, place := pol.MountedOver(home, mounts)
	if at == "" {
		return ""
	}
	kind := ""
	for _, m := range mounts {
		if m.At == at && m.Type != "" {
			kind = " (" + m.Type + ")"
			break
		}
	}
	where := ""
	if at, place := filepath.Clean(at), filepath.Clean(place); strings.HasPrefix(place, at+"/") {
		where = ", over " + place // a share at a folder holding a plant
	} else if at != place {
		where = ", inside " + place
	}
	return mountedBegins + "is mounted at " + at + kind + where + ", where no desk may change anything (unmount it with umount " +
		shellQuoted(at) + " in Terminal)"
}

// deskStopListing is where a desk's stop lists what it may have planted, from its sandbox: CLAUDEPLANT1's merge puts
// holdClaudePlants(s.places.OwnID, claudeLooks(s.policy, s.places), s.places.Checkout) here, the home folders the
// Policy let it write as well as its place (its claudeHireEnded lists the place alone). Tests set it.
var deskStopListing = func(s *deskSandbox) {}

// rowDeskSandbox is desk id's sandbox made again from its row, as its start made it, without keeping it: nil for a
// desk whose row records no sandboxed start the core checked (a start refused before its server, or not a sandboxed
// desk), or whose sandbox can't be made now.
func rowDeskSandbox(id string, o engineOptions) *deskSandbox {
	h, ok := registryHire(id)
	if !ok || deskSocketID(h.TmuxSocket, testCopyPort()) != id || h.TmuxServer <= 1 {
		return nil
	}
	if o.Git == nil {
		o.Git = runGit
	}
	d := deskAtOf(h, deskSocketPath(id, testCopyPort()), claudeDenyProfiles(savedConfig().ClaudeProfiles), o)
	pl, err := deskPlaces(d)
	if err != nil {
		log.Printf("sandbox: desk %s's places at its stop: %v", id, err)
		return nil
	}
	// As its start found it. Not known (Smolcorp started again since): off, which denies nothing more, so the stop's
	// listing of what it could write looks at no less (RT126SAX14 nit 5: "on" would narrow the listing).
	pl.RemoteLogin = deskCheckedOf(id).RemoteLogin
	pol, err := sandbox.New(pl)
	if err != nil {
		log.Printf("sandbox: desk %s's policy at its stop: %v", id, err)
		return nil
	}
	return &deskSandbox{at: d, places: pl, policy: pol}
}

// deskAtOf is what desk h's start knows of it, from its row, as the engine's start had it: its worktree (else its
// folder), the checkout that is a worktree of, the hires' folder, its socket, the Claude accounts its fence denies.
func deskAtOf(h Hire, socket string, accounts []claudeProfile, o engineOptions) deskAt {
	d := deskAt{ID: h.ID, GM: h.Role == gmRole, Root: h.Folder, Socket: socket, Accounts: accounts, Brief: h.Brief, Git: o.Git,
		DatabaseOptIn: o.DatabaseOptIn}
	if h.Worktree != "" && h.Repo != "" {
		d.Root, d.Checkout = h.Worktree, h.Repo
	}
	if h.Repo != "" {
		d.Hires = hiresFolder(h.Repo, o.Worktrees)
	}
	if workspace := gmWorkspace(d.GM, h.ID, h.Repo, h.Worktree); workspace != "" {
		d.Root, d.Projects = workspace, h.Folder
	}
	return d
}

// gmWorkspace is where a GM works in its sandbox when it was hired into a folder that isn't a repo checkout or
// worktree (FXGMROOT, RT126SWEEP2 P1 B1): a workspace of its own in its own folder in the support folder,
// hires/<id>/work, which the core makes at its hire (makeGMWorkspace) and takes away at its fire. It still starts in
// the folder it was hired into, which it reads and never writes. "" for a GM in a repo (its own worktree, Setup's
// "a worktree of its own", or the checkout: the repo's rows hold it as any desk's) and for any other desk.
func gmWorkspace(gm bool, id, repo, worktree string) string {
	if !gm || repo != "" || worktree != "" || !oneName(id) {
		return ""
	}
	return sandbox.Workspace(supportDir, id)
}

// makeGMWorkspace makes a GM's workspace, its owner's alone, if it isn't there: kept across the core's restarts, it
// is made new only once gone. Anything else at its name (a link, a file) refuses the start.
func makeGMWorkspace(workspace string) error {
	if err := os.Mkdir(workspace, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if info, err := os.Lstat(workspace); err != nil || !info.IsDir() {
		return errors.New("the GM's workspace isn't a folder")
	}
	return nil
}

// removeGMWorkspace takes GM id's workspace away with its other files, at its fire or a start that failed: what it
// wrote there is its own notes. Links inside are removed as links, never followed.
func removeGMWorkspace(id string) {
	if oneName(id) {
		_ = os.RemoveAll(sandbox.Workspace(supportDir, id))
	}
}

// deskStoppedNow is whether desk h is known to be stopped now, from its row alone, whatever this core kept (for
// CLAUDEPLANT1's "remove only while the desk is stopped"): a desk in its sandbox whose server, as the core checked it
// at its start, is gone, whose socket doesn't answer, and, on a Mac, with no process left in its sandbox, as two
// passes in a row of the reap's look find (FXFIREREAPR1: what its panes started in the background outlives its
// server). Nothing is run, and nothing is ended. Anything else isn't known stopped: another kind of desk, a row with no
// checked server, a pid that's there (its own or one used again), a socket that answers, a process in its sandbox, a
// look that couldn't be made whole.
func deskStoppedNow(h Hire) bool {
	testPort := testCopyPort()
	if deskSocketID(h.TmuxSocket, testPort) != h.ID || h.TmuxServer <= 1 {
		return false
	}
	if err := syscall.Kill(h.TmuxServer, 0); !errors.Is(err, syscall.ESRCH) {
		return false
	}
	conn, err := net.DialTimeout("unix", deskSocketPath(h.ID, testPort), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return false
	}
	if deskSandboxed {
		left, sure := reapSandboxAt(hireFolder(supportDir, h.ID), false)
		return len(left) == 0 && sure
	}
	return true
}

// deskPlaces is what the core knows of where desk d works, as the sandbox's Places: every path resolved, as the
// kernel checks it.
func deskPlaces(d deskAt) (sandbox.Places, error) {
	own, err := deskOwnID(runtime.GOOS, d.ID)
	if err != nil {
		return sandbox.Places{}, err
	}
	pl := sandbox.Places{Subject: sandbox.Hire, Home: physical(homeDir), Support: physical(supportDir), OwnID: own,
		Root: physical(d.Root), SocketParent: physical(deskSocketParent()), OwnSockets: physical(filepath.Dir(d.Socket))}
	if d.GM {
		pl.Subject = sandbox.GM
	}
	start := deskStartSocket()
	pl.DeskStart = filepath.Join(physical(filepath.Dir(start)), filepath.Base(start))
	// tmux's own folder (FXTMUXDIR), spelled as the kernel checks it even before tmux has made it: its parent resolved.
	tmux := tmuxSocketFolder()
	pl.TmuxFolder = filepath.Join(physical(filepath.Dir(tmux)), filepath.Base(tmux))
	if testCopyPort() != 0 {
		// A test copy's desks are denied the owner's own folder with no exception.
		if real := realSupportDir(); real != "" {
			pl.OtherSupports = []string{physical(real)}
		}
	}
	if d.Checkout != "" {
		pl.Checkout = physical(d.Checkout)
	}
	if d.Hires != "" {
		pl.HiresFolder = physical(d.Hires)
	}
	// The GM's place, for every other desk: its workspace when it has one (FXGMROOT), never the projects folder it
	// reads, which holds every hire's worktree. A workspace is spelled from the support folder resolved, whether it is
	// there yet or not (a GM that started outside its sandbox has none).
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Role == gmRole && h.ID != d.ID && isOpen(h) {
			pl.GMFolder, pl.GMStart, pl.GMStartPlans = physical(startedIn(h)), "", ""
			if gmWorkspace(true, h.ID, h.Repo, h.Worktree) != "" {
				// …and where it starts, which its workspace doesn't hold: its plans and the agent files Claude Code loads
				// there, spelled as its own Places spell them (FXGMROOTR1 N2).
				pl.GMFolder = sandbox.Workspace(pl.Support, h.ID)
				pl.GMStart = physical(h.Folder)
				pl.GMStartPlans = filepath.Join(pl.GMStart, filepath.FromSlash(deskPlansFor(h.ID)))
			}
		}
	}
	pl.Accounts = deskAccounts(runtime.GOOS, d.Accounts)
	pl.AgentHomes, pl.AgentSignIns = deskAgentHomes(runtime.GOOS, os.Getenv)
	pl.ConfigHomes = deskConfigHomes(runtime.GOOS, os.Getenv)
	pl.ShellHomes = deskShellHomes(runtime.GOOS, os.Getenv)
	if d.Projects != "" {
		// Its workspace, spelled so too, and its own plans folder where it starts (deskPlansFor), as the folder it
		// starts in spells it to Claude Code.
		pl.Root = sandbox.Workspace(pl.Support, own)
		pl.GMProjects = physical(d.Projects)
		pl.GMPlans = filepath.Join(pl.GMProjects, filepath.FromSlash(deskPlansFor(d.ID)))
	} else {
		// Every other desk writes only its own place in the folder for hires (FXHIREPROJECTS): on a Mac the Policy
		// refuses a desk whose place is that folder, or holds it (FXHIREPROJECTSR4); elsewhere it gives such a desk no row.
		pl.Projects = hireProjectsFolder()
		if err := deskGitPlaces(d, &pl); err != nil { // a GM's workspace is in no repo: no git places
			return pl, err
		}
		if pl.Projects != "" && runtime.GOOS == "darwin" {
			// …and is held at the links on the way to it (FXHIREPROJECTSR3). A Mac's rows.
			pl.ProjectsLinks = hireProjectsLinks()
		}
	}
	finding := deskFinding()
	finding.Also = append(finding.Also, d.Programs...)
	programs, err := sandbox.ResolvePrograms(finding)
	if err != nil {
		return pl, err
	}
	pl.InstallRoots = programs.InstallRoots()
	git := programs.Path("git")
	if git == "" {
		return pl, nil // no git: nothing reads a git config
	}
	pl.GitConfigs = gitConfigFiles(git)
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	pl.GitHooks, pl.GitNamed, err = sandbox.ReadGitConfig(ctx, git, nil, pl)
	if err != nil {
		return pl, err
	}
	// …and the ignore file the core's reads carry (gitHere: asked once for the core's life), which the owner may have
	// changed since: a desk that wrote it would hide its files from them (FXAGENTHOMEMACR2, RT126AGENTHOMEMACR1 N2).
	for _, value := range carriedExcludesFiles() {
		carried, err := sandbox.CarriedIgnorePlaces(value, []string{pl.Home, os.Getenv("HOME")}, []string{pl.Root, pl.Checkout})
		if err != nil {
			return pl, err
		}
		pl.GitNamed = append(pl.GitNamed, carried...)
	}
	slices.Sort(pl.GitNamed)
	pl.GitNamed = slices.Compact(pl.GitNamed)
	return pl, nil
}

// deskAgentHomes are the hired agents' places and sign-ins the core's own environment, getenv, moves out of their
// places in the home folder (sandbox.AgentPlaces: CODEX_HOME, GEMINI_CLI_HOME, GROK_HOME, OpenCode's XDG folders and
// the like), read at each desk's start, for Places.AgentHomes and AgentSignIns on goos: each resolved, the folders
// above a part not there yet too (a sign-in not made yet, in a linked folder), and on a Mac kept as spelled too, as an
// account folder is (deskAccounts), so the Policy holds a link there itself as well as where it leads.
func deskAgentHomes(goos string, getenv func(string) string) (homes, signIns []string) {
	homes, signIns = sandbox.AgentPlaces(getenv)
	return resolvedAndSpelled(goos, homes), resolvedAndSpelled(goos, signIns)
}

// deskConfigHomes is where the core's own environment, getenv, moves ~/.config (XDG_CONFIG_HOME), read at each desk's
// start, for Places.ConfigHomes on a Mac, where a desk's sandbox runs (FXAGENTHOMEMACR1, O3): resolved and as spelled,
// as deskAgentHomes gives its places. None when it isn't a whole path, or is ~/.config, which has its own rows.
func deskConfigHomes(goos string, getenv func(string) string) []string {
	xdg := getenv("XDG_CONFIG_HOME")
	if goos != "darwin" || !filepath.IsAbs(xdg) || physicalBelow(xdg) == physicalBelow(filepath.Join(homeDir, ".config")) {
		return nil
	}
	return resolvedAndSpelled(goos, []string{xdg})
}

// deskShellHomes is where the core's own environment, getenv, moves zsh's own files (ZDOTDIR), read at each desk's
// start, for Places.ShellHomes on a Mac (FXSWEEP3P1, RT126SWEEP3 P1's B1): resolved and as spelled, as deskConfigHomes
// gives its places. None when it isn't a whole path, or is the home folder, whose zsh files have their own rows. The
// Mac app passes it (GoCore.agentPlaces); set only in ~/.zshenv, it never reaches the core.
func deskShellHomes(goos string, getenv func(string) string) []string {
	zdot := getenv("ZDOTDIR")
	if goos != "darwin" || !filepath.IsAbs(zdot) || physicalBelow(zdot) == physicalBelow(homeDir) {
		return nil
	}
	return resolvedAndSpelled(goos, []string{zdot})
}

// resolvedAndSpelled is each of places resolved, the folders above a part not there yet too, and on a Mac as spelled
// too when that differs, as an account folder is (deskAccounts), so the Policy holds a link there itself as well as
// where it leads.
func resolvedAndSpelled(goos string, places []string) []string {
	var out []string
	for _, place := range places {
		out = append(out, physicalBelow(place))
		if goos == "darwin" && filepath.Clean(place) != physicalBelow(place) {
			out = append(out, filepath.Clean(place))
		}
	}
	return out
}

// physicalBelow is p as physical makes it, with the folders above a part of it that isn't there yet resolved too.
func physicalBelow(p string) string {
	p = physical(p)
	if _, err := os.Lstat(p); err == nil {
		return p
	}
	if dir := filepath.Dir(p); dir != p {
		return filepath.Join(physicalBelow(dir), filepath.Base(p))
	}
	return p
}

// hireProjectsFolder is the folder for hires as the core resolves it (Setup's, Config.HireFolder: existingFolder),
// as the kernel checks it, for a desk's Places.Projects (FXHIREPROJECTS); "" when none is set or it isn't a folder.
// It is read from the saved config, as each start, restart and stop finds it: a test copy's is its own.
func hireProjectsFolder() string {
	folder, ok := existingFolder(savedConfig().HireFolder)
	if !ok {
		return ""
	}
	return physical(folder)
}

// hireProjectsLinks are the links on the way to the folder for hires as Setup spells it (FXHIREPROJECTSR3,
// RT126FIXREVIEW-HIREPROJECTSR2 N2), for Places.ProjectsLinks: each where the kernel meets it, the folders before it
// resolved, so the Policy holds the link itself, as deskAccounts keeps an account folder's (RT126RESOLVESWEEP S2b). A
// desk could otherwise point `~/Projects` at another folder, and every later start would hold that one instead. None
// when the folder is spelled as it is, and none in a folder where no desk can change it (`/var`, `/tmp`, in `/`:
// linkFixedInFolder).
func hireProjectsLinks() []string {
	folder, ok := existingFolder(savedConfig().HireFolder)
	if !ok || physical(folder) == folder {
		return nil
	}
	var links []string
	at := string(filepath.Separator)
	for _, name := range strings.Split(strings.TrimPrefix(folder, at), at) {
		next := filepath.Join(at, name)
		info, err := os.Lstat(next)
		if err != nil {
			break // gone since: the Policy holds the folder as it resolves now
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if in, err := os.Lstat(at); err != nil || !linkFixedInFolder(in, info) {
				links = append(links, next)
			}
			next = physical(next)
		}
		at = next
	}
	return links
}

// linkFixedInFolder is whether no desk can remove, replace or rename link, a link in folder (FXHIREPROJECTSR4,
// RT126FIXREVIEW-HIREPROJECTSR3 O7a): that is the folder's to say, not the link's owner's. A desk runs as the owner,
// so it can't when folder is another user's and no one else may write in it (`/`, which holds `/var` and `/tmp`), or
// when folder is sticky (`/private/tmp`) and both it and the link are another user's. A root-owned link in a folder
// of the owner's (`sudo ln -s` in ~) is the owner's to change, so a desk's too.
func linkFixedInFolder(folder, link fs.FileInfo) bool {
	if !ownedByAnother(folder) {
		return false
	}
	if folder.Mode()&os.ModeSticky != 0 {
		return ownedByAnother(link)
	}
	return folder.Mode().Perm()&0o022 == 0
}

// deskProjectsWords are a desk's start words on the folder for hires (FXHIREPROJECTS), for a desk at root in its
// sandbox: "" when none is set, or root is it or holds it (on a Mac such a desk doesn't start: wholeProjectsWords). A
// desk in a worktree of its own in it is told of its worktree; a desk at a project there, of the folder it works in;
// a desk elsewhere, that the folder is read-only to it but its repo's git data (FXHIREPROJECTSR3, N3).
func deskProjectsWords(root string, worktree bool) string {
	projects, root := hireProjectsFolder(), physical(root)
	if projects == "" || inside(root, projects) {
		return ""
	}
	shown := shownInWords(projects) // on one line, whatever its name (FXPICKEDPROJECTR2, B1)
	// Its worktree is elsewhere (Setup's folder for worktrees).
	if !inside(projects, root) {
		return "Your projects folder, " + shown + ", is read-only to you, apart from your repository's git data."
	}
	own := "your own worktree"
	if !worktree {
		own = "the folder you work in"
	}
	return "Your projects folder is " + shown + ". You can change files only in " + own + " here; other projects are read-only."
}

// wholeProjectsWords say why a desk whose place is root can't work there in its sandbox on a Mac, or "" when it can
// (FXHIREPROJECTSR4, the owner's ruling of 18 Nov: a hire writes only its own project folder): root is the folder for
// hires, or holds it, or is the home folder, or holds it. At its hire (by Setup or the GM: the office asks which project
// first) it is told to give a project's folder; at a start (started), a desk hired so before, to be fired and hired
// again into one, by whichever way (FXHIREPROJECTSR5, N2).
func wholeProjectsWords(root string, started bool) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	projects, home, root := hireProjectsFolder(), physical(homeDir), physical(root)
	// As the Mac's disks name folders: one name in any case (the Policy's key folds it the same way).
	is, holds := func(a, b string) bool { return strings.EqualFold(a, b) }, func(a, b string) bool { return inside(strings.ToLower(a), strings.ToLower(b)) }
	var where string
	switch {
	case projects != "" && is(root, projects):
		where = "your whole projects folder, " + projects
	case projects != "" && holds(root, projects):
		where = root + ", which holds your projects folder, " + projects
	case is(root, home):
		where = "your whole home folder"
	case holds(root, home):
		where = root + ", which holds your home folder"
	default:
		return ""
	}
	if started {
		return "This hire was set to work in " + where + ", but on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into one of your projects: the office asks which one; in Setup, choose or type that project's own folder; or ask the General Manager to name it."
	}
	return "On this Mac, each hire works in one project of its own, not in " + where + ". Give it the folder of one of your projects instead."
}

// hirePlaceWords say why a desk in its sandbox on a Mac can't work at root, at its hire, or "" when it can: root is
// the projects folder or home, or holds one (wholeProjectsWords), or holds a bare repo, or is in no repo and holds one,
// or can't be looked all through (gitPlaceWords). Whether root is in a repo is asked as its start asks it
// (deskGitPlaces), with git; when git can't say, the start refuses anyway, so nothing is said here.
func hirePlaceWords(root string, git gitRunner) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	if words := wholeProjectsWords(root, false); words != "" {
		return words
	}
	pl := sandbox.Places{Root: physical(root)}
	if err := deskGitPlaces(deskAt{Root: root, Git: git}, &pl); err != nil {
		return ""
	}
	words, _ := gitPlaceWords(root, pl.GitDir != "", false, false)
	return words
}

// deskAccounts are the Claude account folders besides ~/.claude, for Places.Accounts on goos: each resolved, and on a
// Mac one kept as a link as it is spelled too (RT126RESOLVESWEEP S2b), so the Policy holds the link itself, which no
// desk can then point elsewhere for the owner's later sessions, as well as where it leads. On Linux the view already
// makes such a link again, read-only, in the covered home folder and covers where it leads whole; given as spelled, it
// would be a place the view must hold that is a link, and every start would fail (RT126HOMELINKS B3).
func deskAccounts(goos string, accounts []claudeProfile) []string {
	var dirs []string
	for _, account := range accounts {
		if account.Dir == "" || physical(account.Dir) == physical(filepath.Join(homeDir, ".claude")) {
			continue
		}
		dirs = append(dirs, physical(account.Dir))
		if spelled, err := filepath.Abs(account.Dir); goos == "darwin" && err == nil && spelled != physical(account.Dir) {
			dirs = append(dirs, spelled)
		}
	}
	return dirs
}

// deskOwnID is the id desk id's sandbox knows it by (Places.OwnID): its own folder hires/<id>, and on Linux its kept
// copy and its branch folder refs/heads/hire/<id>, the only one its view lets it write. There it is the slug hireBranch
// names its branch by, the GM's as well as a hire's (RMSYLCHAIN F4), or the desk commits nowhere. The core's ids are
// their own slug; a desk whose id isn't has no sandbox on Linux, rather than one whose folders don't match.
func deskOwnID(goos, id string) (string, error) {
	if goos != "linux" || hireSlug(id) == id {
		return id, nil
	}
	return "", errors.New("desk " + strconv.Quote(id) + "'s id isn't the slug its branch is named by (" + hireSlug(id) + ")")
}

// deskKeptFolder is the kept copy a desk's guard is told of (--held), for a desk the engine starts in its sandbox on
// goos (sandboxed): a Linux desk's start keeps a copy of the places no mount holds in its view (sandbox.SaveKept, from
// PrepareBwrap), and its guard checks them against it before and after each call (L1GUARD1, the release gate's item
// 4b). "" for a desk that keeps none: on a Mac, or a desk not started in its sandbox.
func deskKeptFolder(goos string, sandboxed bool, id string) string {
	if goos != "linux" || !sandboxed {
		return ""
	}
	return sandbox.KeptFolder(physical(supportDir), hireSlug(id))
}

// deskGitPlaces fills the repo d works in: its shared .git, its own registration in it and the others', and the
// default branch. A folder that isn't in a repo has none.
func deskGitPlaces(d deskAt, pl *sandbox.Places) error {
	status, out, ran := d.Git(pl.Root, "rev-parse", "--path-format=absolute", "--git-common-dir", "--git-dir")
	if !ran {
		return errors.New("git couldn't be asked where the repo is")
	}
	lines := strings.Split(out, "\n")
	if status != 0 || len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return nil
	}
	pl.GitDir = physical(lines[0])
	own := physical(lines[1])
	worktrees := filepath.Join(pl.GitDir, "worktrees")
	if filepath.Dir(own) == worktrees {
		pl.OwnWorktree = filepath.Base(own)
	}
	entries, err := os.ReadDir(worktrees)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if e.Name() != pl.OwnWorktree {
			pl.OtherWorktrees = append(pl.OtherWorktrees, e.Name())
		}
	}
	// Pinned the first time (FXGITROWS2), and what origin/HEAD names now, the owner's (FXGITROWS2R1, N1: deskgitrows.go).
	pl.DefaultBranch, pl.DefaultBranchNow = deskDefaultBranches(pl.GitDir)
	pl.CheckoutBranch, pl.OwnBranches = deskBranches(d, pl.GitDir)
	return nil
}

// deskFinding is where the core finds the programs it runs, for their install roots (SA-X2): its PATH, then the one
// your login shell sets, itself, and every hire's recorded tmux.
func deskFinding() sandbox.Finding {
	f := sandbox.Finding{Home: physical(homeDir), Search: append(filepath.SplitList(os.Getenv("PATH")), tmuxLoginPath()...),
		DeveloperDir: os.Getenv("DEVELOPER_DIR"), XcrunDB: os.Getenv("xcrun_db")}
	if program, err := executable(); err == nil {
		f.Executable = program
	}
	if temp := os.Getenv("TMPDIR"); filepath.IsAbs(temp) {
		f.UserTemp = temp // the TMPDIR launchd gives the app: confstr's _CS_DARWIN_USER_TEMP_DIR
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if filepath.IsAbs(h.TmuxProgram) && !slices.ContainsFunc(f.Also, func(p sandbox.Program) bool { return p.Path == h.TmuxProgram }) {
			f.Also = append(f.Also, sandbox.Program{Name: "tmux", Path: h.TmuxProgram})
		}
	}
	return f
}

// systemConfigNamed is the file git names when it lists, or can't read, its system config.
var systemConfigNamed = regexp.MustCompile(`(?m)^file:([^\t\n]+)\t|unable to read config file '([^'\n]+)'`)

// gitConfigFiles are the config files git reads for every repo besides the home folder's (Places.GitConfigs): its
// system config, named whether it is there or not (git names a missing one in its error), and
// $XDG_CONFIG_HOME/git/config when that isn't ~/.config's.
func gitConfigFiles(git string) []string {
	var files []string
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, git, "config", "--system", "--list", "--show-origin"))
	cmd.Dir = "/"
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	_ = cmd.Run()
	if m := systemConfigNamed.FindSubmatch(out.Bytes()); m != nil {
		if named := string(append(m[1], m[2]...)); filepath.IsAbs(named) {
			files = append(files, named)
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) && physical(xdg) != physical(filepath.Join(homeDir, ".config")) {
		files = append(files, filepath.Join(xdg, "git", "config"))
	}
	return files
}

// sandboxStartWait is how long a sandboxed start may take: its handshake, then tmux's start call.
var sandboxStartWait = 10 * time.Second
