package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// The Policy (SA-X1, SANDBOX-PLAN §A4.1): which files and places a desk Smolcorp starts may read, write and
// connect to, as data every system's sandbox is made from (the Mac's profile, Linux's view, Windows' ACLs). It holds
// no enforcement itself: it says, for one path, what the desk's sandbox must do, and which row says so.
//
// Deny wins, always, whatever order the rows are in (RT126SAX1 B2): a path any matching row denies is denied. The
// one way in is an exception: an allow that names a Wide deny (the support folder, the hires folder, the owner's
// checkout, the ssh folder, the socket folder) opens it, but only inside that deny's own place, and never past any
// other deny. A path no rule names is normal work: allowed. Paths are compared by their folded key (policy_fold.go),
// so `..`, case, a decomposed name and a firmlink spelling all land on the same row.
//
// A deny also holds the folders above its place (B1): each can't be removed, renamed away or made a link, up to the
// top, so a desk can't move a parent aside, write under its new name and move it back. One that doesn't exist yet is
// made, empty, before the desk starts (Hold, RT126SAX1R2 B1′), so a desk can't bring it in whole either, by a rename
// or as a link, with a denied file already inside. The shared .git's modules folder is denied whole to every desk
// (RT126SAX1R4 B1‴), and so is every file git reads to find the repo's config, hooks and objects (SIERRA, 14 Nov), and
// the entries that make the shared .git a git folder at all (RT126SAX1R5 B3), and what the owner's git config names
// for git to run or read as config (GITCFG1, ReadGitConfig).
//
// An MCP server a desk's agent starts (`smolcorp mcp`) runs inside that desk's sandbox: its column is its desk's, a
// hire's or the GM's, and it is given nothing more. In particular no subject is given the GM's memory (PL126M E1).

// Subject is whom a Policy is for.
type Subject int

const (
	// Hire is a desk Smolcorp started for a hire: in its own worktree, or at a project.
	Hire Subject = iota
	// GM is the General Manager Smolcorp started, in its own folder (§A5.1).
	GM
)

// Op is what a desk asks to do to a path.
type Op int

const (
	// Read is opening a file to read it, or listing a folder.
	Read Op = iota
	// Write is making, changing, removing, renaming in or out, and linking in or out (§A4).
	Write
	// Connect is connecting to the unix socket at the path.
	Connect
	// Move is removing the folder at the path, or renaming it away, and changing its mode, flags, ACL or owner (chmod,
	// chflags, `chmod +a`, chgrp: RT126SWEEP2's F4, a `chmod 000 .git/refs/heads` stops the owner's git): a row's Write
	// says it too, and the rows on the folders above a denied place say only this and Link, so files and folders are
	// still made inside them. On a Mac it is `file-write-unlink` on a folder (policy_darwin_test.go checks `mv`, perl's
	// rename and python's os.rename) and file-write-mode, -flags, -acl and -owner (gitrows2_darwin_test.go).
	Move
	// Link is making a hard or symbolic link at the path: a row's Write says it too, the socket folder's row says only
	// this (§A4.1: no links anywhere in it, the desk's own folder included), and the folders above a denied place say
	// it with Move (B1′).
	Link
)

func (op Op) String() string {
	return [...]string{"read", "write", "connect", "move", "link"}[op]
}

// Verdict is a rule's answer for one Op: Unset when the rule says nothing about it.
type Verdict int8

const (
	Unset Verdict = iota
	Allow
	Deny
)

func (v Verdict) String() string {
	return [...]string{"unset", "allow", "deny"}[v]
}

// Rule is one row of the Policy, for one place.
type Rule struct {
	// Row is the §A4.1 row it comes from, in a few words.
	Row string
	// Path is the place as the system spells it (cleaned; on a Mac the firmlinks put one way): what a profile writes.
	// "" is anywhere, for a Tail that holds wherever it's found.
	Path string
	// Tail is "" for Path itself, else a pattern of the names below Path, one per folder, `**` for any number of
	// folders (path.Match for each name).
	Tail string
	// Below is whether everything below what matches is held too.
	Below bool
	// What it says about each Op. Write also says Move and Link: a deny of either wins.
	Read, Write, Connect, Move, Link Verdict
	// Wide is a deny an exception can open inside it (see Opens). A deny that isn't Wide is final: nothing opens it.
	Wide bool
	// Opens are the Rows of the Wide denies this allow is an exception to. It opens one only where its own place is
	// inside that deny's (or, at the same place, a narrower Tail), and never another deny.
	Opens []string
	// Except are places below Path the row doesn't hold, each with everything below it (FXGITROWS2): a deny of every
	// desk's branch folder but the desk's own. On a Mac each is a `require-not` subpath, which folds as the volume does.
	Except []string
	// ExceptTails are patterns of names below Path, as Tail is, that the row doesn't hold, each with everything below
	// what matches (FXHIREPROJECTSR7: a package manager's own folders, ToolsGits). On a Mac each is a `require-not`
	// regex in the row's own filter, so the row stays final.
	ExceptTails []string

	key, tail   string   // folded
	except      []string // folded
	exceptTails []string // folded
}

func (r Rule) verdict(op Op) Verdict {
	switch op {
	case Read:
		return r.Read
	case Write:
		return r.Write
	case Connect:
		return r.Connect
	case Move:
		return either(r.Write, r.Move)
	}
	return either(r.Write, r.Link)
}

// either is a deny when one of a and b is, else an allow when one is.
func either(a, b Verdict) Verdict {
	if a == Deny || b == Deny {
		return Deny
	}
	return max(a, b)
}

// denies is whether r denies anything.
func (r Rule) denies() bool {
	return r.Read == Deny || r.Write == Deny || r.Connect == Deny || r.Move == Deny || r.Link == Deny
}

// opens is whether exception e opens Wide deny w: e names w, and e's place is inside w's.
func (e Rule) opens(w Rule) bool {
	return slices.Contains(e.Opens, w.Row) && under(e.key, w.key) && (e.key != w.key || e.tail != "" && w.tail != e.tail)
}

// narrower orders rows from the most to the least particular, so the row Decide names is the same in any order.
func narrower(a, b Rule) bool {
	if len(a.key) != len(b.key) {
		return len(a.key) > len(b.key)
	}
	if len(a.tail) != len(b.tail) {
		return len(a.tail) > len(b.tail)
	}
	if a.Below != b.Below {
		return !a.Below
	}
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	if a.key != b.key {
		return a.key < b.key
	}
	return a.tail < b.tail
}

func narrowest(rules []Rule) Rule {
	best := rules[0]
	for _, r := range rules[1:] {
		if narrower(r, best) {
			best = r
		}
	}
	return best
}

// matches is whether the rule holds folded path k.
func (r Rule) matches(k string) bool {
	if slices.ContainsFunc(r.except, func(e string) bool { return under(k, e) }) {
		return false
	}
	var rel string
	switch {
	case r.key == "":
		_, rel, _ = strings.Cut(k, "/")
	case under(k, r.key):
		rel = strings.TrimPrefix(strings.TrimPrefix(k, r.key), "/")
	default:
		return false
	}
	var names []string
	if rel != "" {
		names = strings.Split(rel, "/")
	}
	if slices.ContainsFunc(r.exceptTails, func(tail string) bool { return matchNames(strings.Split(tail, "/"), names, true) }) {
		return false
	}
	if r.tail == "" {
		return len(names) == 0 || r.Below
	}
	return matchNames(strings.Split(r.tail, "/"), names, r.Below)
}

func matchNames(pattern, names []string, below bool) bool {
	if len(pattern) == 0 {
		return len(names) == 0 || below
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(names); i++ {
			if matchNames(pattern[1:], names[i:], below) {
				return true
			}
		}
		return false
	}
	if len(names) == 0 {
		return false
	}
	ok, _ := path.Match(pattern[0], names[0])
	return ok && matchNames(pattern[1:], names[1:], below)
}

// Places is what a desk's start knows about where things are. Every path is whole; "" leaves a place out.
type Places struct {
	// GOOS is the system the Policy is for; "" is this one.
	GOOS    string
	Subject Subject
	Home    string
	// Support is the support folder of the core that starts the desk: the only one whose exceptions apply.
	Support string
	// OtherSupports are Smolcorp's other support folders on this computer, denied with no exception: a test copy's
	// core names the owner's real one here.
	OtherSupports []string
	// OwnID is the desk's id: its folder is <Support>/hires/<OwnID>.
	OwnID string
	// ReapMark is the name of the desk's mark on a Mac (NewReapMark), its file <Support>/hires/<OwnID>/reap.noindex/
	// <ReapMark>: the one place its sandbox alone may read, which tells its processes from every other's at its stop
	// (desktop/deskreap.go). "" for a desk made before marks, which has none.
	ReapMark string
	// DeskStart is the `desk-start` socket the desk's first process connects to: <Support>/desk-start, or tmux's
	// folder's smolcorp.desk-start.<core pid> when that path is too long (SG2).
	DeskStart string
	// Root is where the desk works: the hire's worktree (or project), the GM's folder.
	Root string
	// Checkout is the owner's checkout Root is a worktree of, "" when the desk works at the project itself.
	Checkout string
	// GitDir is the repo's shared .git (git's common dir); OwnWorktree its registration in worktrees/, by name.
	GitDir, OwnWorktree string
	// OtherWorktrees are the other registrations in GitDir/worktrees/ at the start (other desks'), by name.
	OtherWorktrees []string
	// DefaultBranch is the repo's default branch, "" when it can't be told: the one the core pinned for the repo the
	// first time it read DefaultBranch(GitDir) (FXGITROWS2, B2), never read again from a file a desk could change.
	// DefaultBranchNow is the one origin/HEAD names at the start, when it isn't that one (FXGITROWS2R1, N1): the owner's
	// `git remote set-head` is followed, and the pinned one still held. On a Mac no desk writes origin/HEAD, so it is
	// the owner's; elsewhere a desk that changed it only adds a branch to the base branches. Both are base branches.
	DefaultBranch, DefaultBranchNow string
	// CheckoutBranch is the branch the owner's checkout is on at the desk's start, "" for none (detached, or a name
	// with a pattern's characters); OwnBranches are the desk's own branches besides `hire/<OwnID>`, each `hire/<name>`:
	// the folder its branches are named under (`hire/<slug>`) and a branch from before branches had one (FXGITROWS2).
	CheckoutBranch string
	OwnBranches    []string
	// HiresFolder is where the hires' worktrees go (<repo>-hires, or the configured folder); GMFolder the GM's: its
	// Root, its workspace (Workspace) when it has one.
	HiresFolder, GMFolder string
	// GMProjects is, for a GM hired into a folder that isn't a repo checkout or worktree (the folder for hires,
	// ~/Projects by default), that folder: where it starts and what it reads, never writes (FXGMROOT). Its Root is then
	// its workspace, Workspace(Support, OwnID). GMPlans is its own plans folder in GMProjects, the one place there it
	// writes: Claude Code takes a plans folder only inside the folder it starts in. "" for any other desk.
	GMProjects, GMPlans string
	// GMHires are, for a GM, the folders its hires' worktrees go in that its other rows leave open to it
	// (FXGMHIRENOGIT126R1): the folder for hires beside a projects folder that is in a repo (<top>-hires, the demo's and
	// a held GM's layout) and the configured folder for hires' worktrees. It reads and reviews its hires' work there
	// and writes none of it, as rowOtherDesks keeps that folder from other desks; its own worktree there, a GM's in a
	// repo, stays its own place. "" for none, and for any other desk.
	GMHires []string
	// Worktrees are the folders the core makes hires' worktrees in that it keeps for itself (FXCOREWORKTREES126): on a
	// Mac, its own ~/.smolcorp/worktrees, and the configured folder for worktrees when it's honoured. Every desk writes
	// there only its own place (its worktree): no other desk's tree, and no folder there, is its to make, move, swap
	// or remove. A GM writes none of it, reads all of it, and has no place of its own there (ErrWorktrees). The folders
	// above each, up to the home folder, can't be moved, removed or made by any desk either, so none is put elsewhere
	// and another made in its place.
	Worktrees []string
	// GMStart and GMStartPlans are, for every other desk, where an open GM with a workspace starts (its GMProjects)
	// and its plans folder there (its GMPlans): Claude Code loads the agent files at the top of the one and the GM's
	// plan mode reads the other, so no other desk writes them (FXGMROOTR1 N2). "" for the GM itself and for a desk
	// under a GM with no workspace, whose GMFolder holds both.
	GMStart, GMStartPlans string
	// Projects is the folder for hires (Setup's, ~/Projects by default), which holds the owner's repos, their hires'
	// worktrees and the owner's checkouts: a desk writes there only its own place and its repo's shared .git
	// (FXHIREPROJECTS, the owner's ruling, option (a)). "" for none set, and for a GM with GMProjects, which holds it
	// already. On a Mac no desk's place is the projects folder, or holds it (ErrProjectsWhole, FXHIREPROJECTSR4);
	// elsewhere such a desk gets no row: its place is all of it.
	Projects string
	// ProjectsLinks are the links on the way to Projects as Setup spells it, each where the kernel meets it
	// (FXHIREPROJECTSR3, RT126FIXREVIEW-HIREPROJECTSR2 N2): no desk removes, replaces or renames one, so none points a
	// later desk's start at another folder. None when Setup spells the folder as it resolves.
	ProjectsLinks []string
	// GitBelow are the git folders the core found strictly below Root at the desk's start, each a `.git` (a folder, a
	// file or a link) as the walk met it (FXHIREPROJECTSR6): the owner's nested clones and the repos in a folder of
	// repos. The folders on the way to each are held by name, so none is moved out of the place and back.
	GitBelow []string
	// ToolsGitBelow are the git folders the core found at the desk's start in a package manager's own folders below
	// Root (ToolsGits), each a `.git` (a folder, a file or a link) or a bare repo's folder (FXHIREPROJECTSR8,
	// RT126FIXREVIEW-HIREPROJECTSR7 N1): the owner's own clones and mirrors there, which the owner's `swift`, `bundle`
	// or `pip` runs git in. The .git rule leaves those folders out, so each found is held by a row of its own, the
	// folders on the way to it by name: only a clone made during the session is the desk's.
	ToolsGitBelow []string
	// SocketParent is /tmp/smolcorp-<uid>; OwnSockets the desk's own folder in it.
	SocketParent, OwnSockets string
	// TmuxFolder is tmux's own folder for this user, as the core works it out ($TMUX_TMPDIR or /tmp, then tmux-<uid>),
	// resolved: the plain hires' servers, the team grid's, the desks' started outside the sandbox, the owner's own
	// `default`, and the desk-start socket when its path is too long elsewhere. On a Mac a desk can't write, link or
	// bind anything below it (FXTMUXDIR); a Linux desk's view has a /tmp of its own.
	TmuxFolder string
	// Accounts are the confirmed Claude account folders besides ~/.claude.
	Accounts []string
	// InstallRoots are Smolcorp's install and the whole install roots of the programs the core and the app resolved.
	InstallRoots []string
	// GitConfigs are the config files git reads for every repo besides the home folder's, as the core finds them for
	// each git it resolved: its system config (`git config --system --show-origin`; Homebrew's is
	// /opt/homebrew/etc/gitconfig, which the owner can write), and $XDG_CONFIG_HOME/git/config when that isn't
	// ~/.config (SIERRA, 14 Nov).
	GitConfigs []string
	// GitHooks are the folders the owner's git config names for hooks (core.hooksPath), and GitNamed the files it names
	// for git to read as config or run (include and includeIf targets, core.fsmonitor's and core.sshCommand's files)
	// or as the owner's ignore file (core.excludesFile, FXAGENTHOMEMACR1), as ReadGitConfig gives them at the desk's
	// start (GITCFG1). M1 must fill both.
	GitHooks, GitNamed []string
	// RemoteLogin is whether port 22 answers on this computer (C5): the ssh keys are then denied.
	RemoteLogin bool
	// XAuthority is $XAUTHORITY on Linux.
	XAuthority string
	// AgentHomes are the hired agents' own folders, and the files they take settings from, where the owner's
	// environment moves them (AgentPlaces: CODEX_HOME and the like), as the core reads it at the desk's start, and
	// AgentSignIns the sign-ins there: each held on a Mac, as agentFolders and macCredentialNames are
	// (FXPLANTSTERMR3).
	AgentHomes, AgentSignIns []string
	// ConfigHomes are where the owner's environment moves ~/.config (XDG_CONFIG_HOME), as the core reads it at the
	// desk's start, when that isn't ~/.config: the places the owner's programs read in it are held there as they are
	// in ~/.config (FXAGENTHOMEMACR1, RT126AGENTHOMEMAC O3). OpenCode's is AgentHomes'.
	ConfigHomes []string
	// ShellHomes are where the owner's environment moves zsh's own files (ZDOTDIR), as the core reads it at the desk's
	// start, when that isn't the home folder: zsh's start files, their compiled copies, its completion dump, Terminal's
	// restored sessions and the key codes /etc/zshrc sources are held there as they are in the home folder (FXSWEEP3P1,
	// RT126SWEEP3 P1's B1 and B2). A Mac's rows.
	ShellHomes []string
}

// Policy is the rows for one desk, in order.
type Policy struct {
	goos  string
	rules []Rule
	// git is the shared .git as the rows spell it, "" for none: where CheckRefPlaces looks.
	git string
	// ownerHEAD is whether the shared .git's HEAD is the owner checkout's and the desk works in a worktree of its own:
	// CheckRefPlaces then follows it (FXGITROWS2R4).
	ownerHEAD bool
	// desks is the keys of the places Smolcorp gives some desk, this one or another (anyDesks): where a link at git's
	// own names may neither lead nor go through (FXGITROWS2R9).
	desks []string
	// work is the keys of the places the owner and the desks work in (workPlaces): where MountedOver never counts a
	// mount over a narrow place.
	work []string
}

// Rules are the Policy's rows. Their order says nothing: a profile made from them must let every deny win but a Wide
// one an exception opens. On a Mac, where the last row that matches decides, that is each row's allows and denies in
// this order: the plain allows, then the Wide denies, then the allows of the exceptions (Exception), then the final
// denies. A row that names a Wide deny in Opens but doesn't really open one is a plain allow (RT126SAX1R2 B2′): after
// the Wide denies, an allow that holds the hires folder would open it whole.
func (p *Policy) Rules() []Rule { return append([]Rule(nil), p.rules...) }

// Exception is whether r, one of p's Rules, really opens one of p's Wide denies: its place is inside the deny's, and
// it names it. A profile writes its allows after the Wide denies; any other allow goes before them.
func (p *Policy) Exception(r Rule) bool {
	return slices.ContainsFunc(p.rules, func(w Rule) bool { return w.Wide && r.opens(w) })
}

// Decide is what the desk's sandbox does when the desk asks op of path, and the row that says so, the same in any
// order of the rows. A final deny wins; then a Wide deny no exception opens; then an allow; else normal work. Of
// several rows, the narrowest is named. A path that isn't whole is denied, and so, on a Mac, is a path with a code
// point the fold's Unicode doesn't assign (FXPOLICYFOLDR1): a later macOS's volume may fold it as the key can't. (New
// refuses such a place. Unicode keeps the case folding, classes and compositions of the code points it assigns, so
// two names of known code points the fold keeps apart stay apart under a later Unicode.)
//
// path must be resolved: Decide reads it as written, folding only the spellings policy_fold.go names. A link, a
// mount, another volume's name for the same disk (/Volumes/Macintosh HD/…), a Windows 8.3 name, a trailing dot or
// space and a `::$DATA` stream are the caller's to resolve first; the kernel does it for a profile. Marks after a
// letter in any order are one name, as NFD puts them in order (RT126SAX1R2 N2's e, a circumflex, a dot below is `ệ`).
func (p *Policy) Decide(path string, op Op) (Verdict, Rule) {
	k := key(p.goos, path)
	if k == "" {
		return Deny, Rule{Row: "not a whole path"}
	}
	if p.goos == "darwin" && !known(path) {
		return Deny, Rule{Row: rowUnknownCodePoint}
	}
	var final, wide, allows []Rule
	for _, r := range p.rules {
		switch v := r.verdict(op); {
		case v == Unset || !r.matches(k):
		case v == Allow:
			allows = append(allows, r)
		case r.Wide:
			wide = append(wide, r)
		default:
			final = append(final, r)
		}
	}
	if len(final) > 0 {
		return Deny, narrowest(final)
	}
	var shut []Rule
	for _, w := range wide {
		if !slices.ContainsFunc(allows, func(e Rule) bool { return e.opens(w) }) {
			shut = append(shut, w)
		}
	}
	if len(shut) > 0 {
		return Deny, narrowest(shut)
	}
	if len(allows) > 0 {
		return Allow, narrowest(allows)
	}
	return Allow, Rule{Row: "normal work"}
}

// Errors New gives.
var (
	ErrNotWhole       = errors.New("sandbox: a place isn't a whole path")
	ErrRootInSupport  = errors.New("sandbox: a desk's place is inside Smolcorp's support folder")
	ErrRootHasSupport = errors.New("sandbox: a desk's place contains Smolcorp's support folder")
	ErrRootHasDesks   = errors.New("sandbox: a desk's place is or contains the hires folder, or contains the owner's checkout")
	ErrBadName        = errors.New("sandbox: a desk's id, a worktree's or the default branch's name isn't plain")
	ErrDeskStart      = errors.New("sandbox: the desk-start socket is in a support folder, and isn't its core's")
	ErrOwnSockets     = errors.New("sandbox: a desk's own socket folder isn't a folder inside the socket folder")
	ErrHomeLinks      = errors.New("sandbox: a home place's links can't be told whole")
	// ErrUnknownCodePoint is a Mac's place with a code point the name fold's Unicode doesn't assign (FXPOLICYFOLDR1).
	ErrUnknownCodePoint = errors.New("sandbox: a place's name has a code point the Mac's name fold doesn't know")
	// ErrGMProjects is a GM's projects folder given without its workspace as its Root, to another desk, or with a
	// plans folder that isn't inside it (FXGMROOT).
	ErrGMProjects = errors.New("sandbox: a GM's projects folder comes only with its workspace, and its plans folder inside it")
	// ErrGMHires is a GM's hires' folders given to another desk, or one that holds the home folder, the GM's projects
	// or its plans, or is in a support folder or holds one (FXGMHIRENOGIT126R1): its row would hold what the GM's work
	// needs, or the support folder's own rows.
	ErrGMHires = errors.New("sandbox: a GM's hires' folders come only to a GM, and never hold the home folder, its projects or a support folder, nor sit in one")
	// ErrWorktrees is a folder for hires' worktrees (Places.Worktrees) that isn't whole, holds the home folder, the
	// projects folder or a support folder, or is in a support folder; or a GM with a place of its own in one
	// (FXCOREWORKTREES126): the GM's row would deny it its own place, or an exception would open the folder to it.
	ErrWorktrees = errors.New("sandbox: a folder for hires' worktrees holds the home folder, the projects folder or a support folder, sits in a support folder, or holds a GM's own place")
	// ErrGMStart is a GM's start folder given to the GM itself, or with a plans folder that isn't inside it
	// (FXGMROOTR1).
	ErrGMStart = errors.New("sandbox: a GM's start folder is another desk's to be kept from, with the GM's plans folder inside it")
	// ErrProjects is a projects folder given to a GM with its own (GMProjects), inside a support folder, or holding
	// the home folder (FXHIREPROJECTS); or links on the way to it given without one (FXHIREPROJECTSR3).
	ErrProjects = errors.New("sandbox: the projects folder is in a support folder, holds the home folder, is given to a GM with projects of its own, or its links come without it")
	// ErrProjectsWhole is, on a Mac, a desk whose place is the projects folder or holds it, or is the home folder or
	// holds it (FXHIREPROJECTSR4, the owner's ruling of 18 Nov: a hire writes only its own project folder). Its place
	// would be every repo of the owner's, which no row can keep it to one of: it doesn't start in its sandbox.
	ErrProjectsWhole = errors.New("sandbox: a desk's place is the projects folder or the home folder, or holds one")
	// ErrGitBelow is a git folder given in GitBelow that isn't a .git strictly below the desk's place (FXHIREPROJECTSR6).
	ErrGitBelow = errors.New("sandbox: a git folder below a desk's place isn't a .git strictly below it")
)

// gmStartFiles are the agent files at the top of the folder a GM starts in that Claude Code (and other agents) load
// into its session: no other desk writes them, nor their writers' names beside them (FXGMROOTR1 N2).
var gmStartFiles = []string{"CLAUDE.md", "CLAUDE.local.md", ".mcp.json", "AGENTS.md"}

// workspaceName is the GM's workspace in its own folder in the support folder (FXGMROOT).
const workspaceName = "work"

// Workspace is where a GM hired into a folder that isn't a repo checkout or worktree works in its sandbox, its Root:
// <support>/hires/<id>/work, in its own folder there, which the core makes at its hire and takes away at its fire.
func Workspace(support, id string) string {
	return filepath.Join(support, "hires", id, workspaceName)
}

// The GM's rows when it was hired into a folder that isn't a repo checkout or worktree (FXGMROOT).
const (
	rowGMProjects  = "the GM's projects: read only"
	rowGMWorkspace = "its own workspace"
	// rowGMStart is, for another desk, where a GM with a workspace starts (FXGMROOTR1).
	rowGMStart = "other desks' places: where the GM starts"
	// rowProjects is the folder for hires, for a desk whose place is inside it or elsewhere (FXHIREPROJECTS).
	rowProjects = "the projects folder: only its own worktree"
	// rowHomeGit is a git folder at the home folder, which git takes for every folder below it in no repo
	// (FXHIREPROJECTSR1).
	rowHomeGit = "plants the owner's programs run later: a git folder at home"
	// rowAboveGit is a git folder in a folder above a desk's place, the folder for hires or a GM's start folder, which
	// git takes for every folder below it in no repo (FXHIREPROJECTSR2).
	rowAboveGit = "plants the owner's programs run later: a git folder above the places"
	// rowProjectsLinks is, for every desk, the links on the way to the projects folder (FXHIREPROJECTSR3,
	// RT126FIXREVIEW-HIREPROJECTSR2 N2).
	rowProjectsLinks = "the projects folder: the links on the way to it"
	// rowGitBelow is a git folder below a desk's place that isn't its own, or one at its place where its own isn't
	// (FXHIREPROJECTSR6): an owner's nested clone, a repo in a folder of repos, one a desk would make.
	rowGitBelow = "plants the owner's programs run later: a git folder below its place"
)

// ToolsGits are the folders package managers keep their own git clones and mirrors in, as Tails below a desk's place
// (FXHIREPROJECTSR7, RT126FIXREVIEW-HIREPROJECTSR6 N1): SwiftPM's bare mirrors (.build/repositories) and the clones it
// checks dependencies out to (.build/checkouts), Bundler's bare mirrors of a Gemfile's git sources
// (vendor/bundle/ruby/<version>/cache/bundler/git), and pip's clones for `-e git+…` in a virtual environment
// (<venv>/src: .venv and venv, the names Python's own docs and tools give one; pip's src in the folder it runs in is a
// project's own source too often to name). A hire's own `swift build`, `bundle install` or `pip install -e git+…`
// makes them, so the static .git rule doesn't hold them. The core's walk looks in them (desktop's gitBelow), and
// each start holds the clones and mirrors there then, one by one (Places.ToolsGitBelow, FXHIREPROJECTSR8): one made
// during a session is the desk's until its next start.
var ToolsGits = []string{"**/.build/repositories", "**/.build/checkouts", "**/vendor/bundle/**/cache/bundler/git", "**/.venv/src", "**/venv/src"}

// Inside is whether path is base or inside it, both as the system spells them, compared as goos compares names.
func Inside(goos, path, base string) bool {
	k, b := key(goos, path), key(goos, base)
	return k != "" && b != "" && under(k, b)
}

// InToolsGits is whether folder, below root (both as the system spells them), is in one of the ToolsGits folders, or
// is one, on goos.
func InToolsGits(goos, root, folder string) bool {
	k, r := key(goos, folder), key(goos, root)
	if k == "" || r == "" || k == r || !under(k, r) {
		return false
	}
	names := strings.Split(strings.TrimPrefix(strings.TrimPrefix(k, r), "/"), "/")
	return slices.ContainsFunc(ToolsGits, func(tail string) bool {
		return matchNames(strings.Split(foldName(goos, tail), "/"), names, true)
	})
}

// The rows exceptions open, by name.
const (
	rowSSH        = "ssh keys while Remote Login answers"
	rowOtherDesks = "other desks' places"
	// rowWorktreesAbove is the folders above a folder the core makes hires' worktrees in (FXCOREWORKTREES126), which no
	// desk moves, removes or makes.
	rowWorktreesAbove = "other desks' places: the folders above the folder for hires' worktrees"
	rowCheckout       = "the owner's checkout"
	rowSockets        = "the socket folder"
	rowTmuxFolder     = "tmux's own folder"
	// rowOwnSockets is the desk's own socket folder: its own socket must be in it (sbpl.go's serviceRules).
	rowOwnSockets = "its own socket folder"
	rowSupport    = "the support folder, whole"
	rowAbove      = "the folders above a denied place can't be moved or removed"
	// rowPlaceAbove is a desk's place and each folder above it, each held itself, not what's in it.
	rowPlaceAbove = "a desk's place and the folders above it can't be moved or removed"
)

// rowSupportOtherName is a support folder by its other name, whole (supportSiblings).
const rowSupportOtherName = "the support folder by its other name, whole"

// rowUnknownCodePoint is Decide's on a Mac for a path with a code point the fold's Unicode (foldUnicode) doesn't assign.
const rowUnknownCodePoint = "a name with a code point Unicode " + foldUnicode + " doesn't assign"

// rowOldRemotes is the row of git's old remote files, which X12's start-time listing names as well.
const rowOldRemotes = "the shared git places: the old remote files"

// rowWorktreeModules is the Mac's row of each worktree's submodules' git folders (FXHIREPROJECTSR9).
const rowWorktreeModules = "the shared git places: a worktree's submodules' git folders"

// rowSubstitutions and rowBaseReflogs are the Mac's rows of what makes the owner's git read a base branch as another
// tree or history while the branch never moves (FXREPLACEREFS).
const (
	rowSubstitutions = "the base branches: git's substitutions"
	rowBaseReflogs   = "the base branches: their reflogs"
)

// rowBaseNames is the Mac's row of a base branch's name in the places the owner's git looks before refs/heads, and
// rowOwnersState of the owner checkout's merge, pick or rebase in progress, for a desk in its own worktree
// (FXBASEHEADS, SIERRA's rulings #2 and #3).
const (
	rowBaseNames   = "the base branches: their names elsewhere"
	rowOwnersState = "the owner's checkout: its merge, pick or rebase in progress"
)

// ownersStateNames are what rowOwnersState holds in the shared .git: a desk at the project keeps them, its own
// (rowGitTop leaves them its own too).
var ownersStateNames = []string{"MERGE_HEAD", "ORIG_HEAD", "logs/HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "REBASE_HEAD", "AUTO_MERGE"}

// rowTerminals and rowAgents are the Mac's rows of what the owner's terminals and agents run from their own settings
// (FXPLANTSTERM); rowAgentLinks is the agents' folders' own guard against links (FXPLANTSTERMR2).
const (
	rowTerminals  = "plants the owner's programs run later: what a terminal reads at a window's start"
	rowAgents     = "plants the owner's programs run later: a hired agent's own settings"
	rowAgentLinks = "no links in a hired agent's folder"
	rowKeychains  = "plants the owner's programs run later: the keychains"
)

// rowShellStart and rowShellSessions are the Mac's rows of what zsh, bash and Terminal read or run at a window's start
// besides the plants, and rowVMKeys of the VM tools' SSH keys (FXSWEEP3P1).
const (
	rowShellStart    = "plants the owner's programs run later: what a shell reads at its start"
	rowShellSessions = "plants the owner's programs run later: a terminal window's restored session"
	rowVMKeys        = "the VM tools' SSH keys: neither read nor written"
)

// zshStartFiles are zsh's start files in ${ZDOTDIR:-~} (zshall, STARTUP/SHUTDOWN FILES), and zshCompiled what zsh
// runs in a file's place when it's there and not older, or when the file isn't there at all (this Mac's zsh 5.9).
var zshStartFiles = []string{".zshenv", ".zprofile", ".zshrc", ".zlogin", ".zlogout"}

const zshCompiled = ".zwc"

// vmKeyFolders are, in the home folder, where Lima, Colima, OrbStack and Podman keep the SSH keys their machines take,
// and Rancher Desktop, which runs Lima with a LIMA_HOME of its own (FXSWEEP3P1R1, RT126SWEEP3FIXP1's N2).
var vmKeyFolders = []string{".lima/_config", ".colima/_lima/_config", ".orbstack/ssh", ".local/share/containers/podman/machine",
	"Library/Application Support/rancher-desktop/lima/_config"}

// ohMyZsh is where oh-my-zsh is kept, and p10kConfig Powerlevel10k's settings, which ~/.zshrc sources at every
// interactive zsh (FXSWEEP3P1R1, RT126SWEEP3FIXP1's recommendation (b)). oh-my-zsh is held whole: it puts its cache's
// completions first on $fpath, so a function planted there runs at the next window as a plant in its scripts would.
// A desk's own zsh still starts with them held: oh-my-zsh skips its update check where it can't write, and its
// cache writes fail without a word. ohMyZshBeside is where oh-my-zsh's installer puts it when ZDOTDIR is set and isn't
// the home folder ($ZDOTDIR/ohmyzsh), held whole in each zsh folder but the home folder (FXSWEEP3P1R2,
// RT126SWEEP3FIXP1R1's N3).
const (
	ohMyZsh       = ".oh-my-zsh"
	ohMyZshBeside = "ohmyzsh"
	p10kConfig    = ".p10k.zsh"
)

// rowAccounts is the row of the account folders, `~/.claude` and the other accounts': read, never written.
const rowAccounts = "the account folders"

// terminalPlaces are, in the home folder, what the terminals Smolcorp opens (Terminal, iTerm2, Ghostty:
// TerminalDrivers.swift's Openers) and drives (WezTerm, kitty) read when a window starts: the preferences, where
// Terminal's and iTerm2's profiles are (each one's command and whether it runs as the shell), Ghostty's and kitty's
// Mac folders, and every app's named preferences, which a desk can't reach through cfprefsd (lists.go) but could write
// here as a file; iTerm2's folder, with its dynamic profiles and its AutoLaunch scripts, and its link to it in
// ~/.config; Ghostty's two config places; WezTerm's and kitty's config folders; and Terminal's saved state, which a
// restored window is made from (FXSWEEP3P1: nothing of a desk's writes there). WezTerm's ~/.wezterm.lua is a file,
// with the plants.
var terminalPlaces = []string{"Library/Preferences", "Library/Application Support/iTerm2", ".config/iterm2",
	"Library/Application Support/com.mitchellh.ghostty", ".config/ghostty", ".config/wezterm", ".config/kitty",
	"Library/Saved Application State/com.apple.Terminal.savedState"}

// agentFolders are, in the home folder, the own folders of the agents Smolcorp hires besides Claude Code, whose next
// session outside a desk runs or loads what they hold: their settings folders (desktop/hooks.go's agentFolder), with
// the MCP servers, hooks and notify command they name; Gemini CLI's own folder when it runs in its own sandbox;
// OpenCode's data, cache and state, with the programs and plugins it downloads and runs from there, and ~/.opencode,
// a settings folder of its too (FXPLANTSTERMR3); and ~/.agents, the skills the owner's Codex, Gemini CLI, OpenCode and
// Grok load. ~/.claude and the accounts' folders have their own row.
var agentFolders = []string{".codex", ".gemini", ".cache/.gemini", ".config/opencode", ".local/share/opencode",
	".cache/opencode", ".local/state/opencode", ".opencode", ".grok", ".agents"}

// configPlants are the places in ~/.config the plants' row holds, as names below a config folder: held where the
// owner's environment moves it too (Places.ConfigHomes).
var configPlants = []string{"fish", "autostart", "systemd/user", "git", "tmux"}

// agentHomeVariables are the variables that move an agent's own folder, or a file it runs what it names from, out of
// the home folder's places (FXPLANTSTERMR3; which agent reads each, and where in its source: the report): the place
// is the variable's value with Under joined to it, held as agentFolders are, and SignIns are the sign-ins in it, held
// as credentialNames are.
var agentHomeVariables = []struct {
	Variable, Under string
	SignIns         []string
}{
	{"CODEX_HOME", "", codexSignIns},
	{"CODEX_SQLITE_HOME", "", nil}, // its databases
	{"GEMINI_CLI_HOME", ".gemini", geminiSignIns},
	{"GEMINI_CLI_HOME", ".agents", nil},
	{"GEMINI_CLI_TRUSTED_FOLDERS_PATH", "", nil}, // a file: the folders whose hooks run
	{"GEMINI_CLI_SYSTEM_SETTINGS_PATH", "", nil}, // a file: settings over the owner's own
	{"GEMINI_CLI_SYSTEM_DEFAULTS_PATH", "", nil}, // a file: settings under them
	{"GROK_HOME", "", grokSignIns},
	{"GROK_CONFIG_PATH", "", nil}, // a file: settings over the owner's own
	{"XDG_CONFIG_HOME", "opencode", nil},
	{"XDG_DATA_HOME", "opencode", openCodeSignIns},
	{"XDG_CACHE_HOME", "opencode", nil},
	{"XDG_STATE_HOME", "opencode", nil},
	{"OPENCODE_CONFIG_DIR", "", nil},
	{"OPENCODE_CONFIG", "", nil}, // a file: settings over the owner's own
}

// agentSignInVariables each name a sign-in file of an agent's own, wherever it is: Grok's sign-in moved, the
// credentials Grok uploads its traces with, and OpenCode's database moved (its sign-ins are in it).
var agentSignInVariables = []string{"GROK_AUTH_PATH", "GROK_TRACE_UPLOAD_CREDENTIALS_FILE", "OPENCODE_DB"}

// The sign-ins each agent keeps in its own folder (FXPLANTSTERMR3, from each one's source: the report): a desk reads
// none of them. A name with no dot is a folder, held whole.
var (
	// Codex: its sign-in; its MCP servers' tokens, when the keychain isn't there; the keys it reads at start; and its
	// encrypted secrets, whose passphrase is in a keychain item a desk could ask the Security server for.
	codexSignIns = []string{"auth.json", ".credentials.json", ".env", "secrets"}
	// Gemini CLI: its Google sign-in; its MCP servers' and A2A agents' tokens; its encrypted file, whose key is made
	// from the computer's and the user's names; the keys it reads at start.
	geminiSignIns = []string{"oauth_creds.json", "mcp-oauth-tokens.json", "a2a-oauth-tokens.json", "gemini-credentials.json", ".env"}
	// Grok (xAI's grok-build): its sign-in, with its API key, and its MCP servers' tokens. The community grok-cli keeps
	// its API key in user-settings.json and a wallet's private key in wallet.json, always in ~/.grok.
	grokSignIns = []string{"auth.json", "mcp_credentials.json", "user-settings.json", "wallet.json"}
	// OpenCode: its providers' keys and sign-ins, its MCP servers' tokens, and its database, whose tables hold its
	// account's tokens and its providers' credentials.
	openCodeSignIns = []string{"auth.json", "mcp-auth.json", "opencode.db"}
)

// shellPlaceVariables are the variables that move the owner's shell's own files out of the home folder: zsh's
// ZDOTDIR, which the core reads for Places.ShellHomes (FXSWEEP3P1, RT126SWEEP3 P1's B1).
var shellPlaceVariables = []string{"ZDOTDIR"}

// AgentPlaceVariables are the names of agentHomeVariables, each once, then agentSignInVariables, then
// shellPlaceVariables, in that order: every variable AgentPlaces reads, and ZDOTDIR, and the list the Mac app passes
// the core (GoCore.agentPlaces).
func AgentPlaceVariables() []string {
	var names []string
	for _, v := range agentHomeVariables {
		if !slices.Contains(names, v.Variable) {
			names = append(names, v.Variable)
		}
	}
	return slices.Concat(names, agentSignInVariables, shellPlaceVariables)
}

// AgentPlaces are the agents' own places and sign-in files the environment getenv gives moves out of the home
// folder's (agentHomeVariables, agentSignInVariables), for Places.AgentHomes and AgentSignIns, as given. A value that
// isn't a whole path is passed over: an agent reads it from the folder it starts in, which no start can know.
func AgentPlaces(getenv func(string) string) (homes, signIns []string) {
	for _, v := range agentHomeVariables {
		value := getenv(v.Variable)
		if !filepath.IsAbs(value) {
			continue
		}
		place := filepath.Join(value, v.Under)
		homes = append(homes, place)
		for _, name := range v.SignIns {
			signIns = append(signIns, filepath.Join(place, name))
		}
	}
	for _, variable := range agentSignInVariables {
		if value := getenv(variable); filepath.IsAbs(value) {
			signIns = append(signIns, filepath.Clean(value))
		}
	}
	return homes, signIns
}

// AgentPlaceRefused is the first of agentHomeVariables and agentSignInVariables, "" for none, whose place getenv
// gives is one no desk's profile on a Mac can hold, as given or as resolve spells it (FXAGENTHOMEMACR1): the top of the
// disk, which an agent's own folder or sign-in can't be, or a place with a character a profile can't be given exactly
// (a line break or another control character). Such a start is refused in words that name the variable. A variable's
// own value is checked as well as the place below it an agent reads (FXAGENTHOMEMACR2, RT126AGENTHOMEMACR1 N1):
// XDG_CONFIG_HOME's is the config folder itself (Places.ConfigHomes), and `/` there held the whole disk's links. So is
// a value with a `.` or `..` in it, or a doubled `/` (AgentPlaceUnclean, FXAGENTHOMEMACR3).
func AgentPlaceRefused(getenv func(string) string, resolve func(string) string) string {
	unfit := func(place string) bool {
		for _, p := range []string{filepath.Clean(place), resolve(place)} {
			if filepath.Dir(p) == p || !sbSpellable(p) {
				return true
			}
		}
		return false
	}
	for _, v := range agentHomeVariables {
		if value := getenv(v.Variable); filepath.IsAbs(value) &&
			(AgentPlaceUnclean(value) || unfit(value) || unfit(filepath.Join(value, v.Under))) {
			return v.Variable
		}
	}
	for _, variable := range slices.Concat(agentSignInVariables, shellPlaceVariables) {
		if value := getenv(variable); filepath.IsAbs(value) && (AgentPlaceUnclean(value) || unfit(value)) {
			return variable
		}
	}
	return ""
}

// AgentPlaceUnclean is whether value, a whole path one of the agent place variables gives, is spelled with a `.` or
// `..` in it, or a doubled `/` (one at its end aside): a place no desk's profile can hold (FXAGENTHOMEMACR3,
// RT126AGENTHOMEMACR2 B1). A profile holds a place as cleaned, `$HOME/dots/../cfg` as `$HOME/cfg`, while the kernel
// takes the `..` after the link: through a link a desk made at ~/dots, the owner's git and agents, and the core's own
// read of the owner's git config, would read a config the desk wrote. The Mac app passes none such (GoCore.environment);
// the core unsets one and refuses each desk's start in the variable's words.
func AgentPlaceUnclean(value string) bool {
	trimmed := strings.TrimRight(value, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	return filepath.IsAbs(value) && filepath.Clean(value) != trimmed
}

// AgentPlaceOf is the first of agentHomeVariables and agentSignInVariables, "" for none, whose whole-path value getenv
// gives, as given or as resolve spells it, is place or holds it: the variable a place a start couldn't look through
// came from (FXAGENTHOMEMACR2, RT126AGENTHOMEMACR1 N1), so the words for it can name the variable. A value that is
// home, or holds it (`/Users`), names no place in home, which is held whatever the variable says, and no value names
// a place in the agents' own folders in home or in ~/.config, held as well without it.
func AgentPlaceOf(getenv func(string) string, resolve func(string) string, home, place string) string {
	goos := runtime.GOOS
	homeKey := key(goos, home)
	var anyway []string
	for _, folder := range append(slices.Clone(agentFolders), ".config") {
		anyway = append(anyway, key(goos, filepath.Join(home, folder)))
	}
	places := []string{filepath.Clean(place), resolve(place)}
	inAny := func(k string) bool {
		return k != "" && slices.ContainsFunc(places, func(at string) bool { return under(key(goos, at), k) })
	}
	if slices.ContainsFunc(anyway, inAny) {
		return ""
	}
	inHome := inAny(homeKey)
	for _, name := range AgentPlaceVariables() {
		value := getenv(name)
		if !filepath.IsAbs(value) {
			continue
		}
		for _, from := range []string{filepath.Clean(value), resolve(value)} {
			k := key(goos, from)
			if inHome && homeKey != "" && under(homeKey, k) {
				continue
			}
			if inAny(k) {
				return name
			}
		}
	}
	return ""
}

// New makes the Policy for a desk at pl. A desk whose root (or checkout, hires folder or GM folder) is inside a
// support folder, or contains one, is refused: no exception could then keep the folder whole. So is one whose root is
// or contains the hires folder, or contains the owner's checkout (RT126SAX1R2 B2′): it would work in other desks'
// places, and only the deny-wins order of a profile would keep them out.
func New(pl Places) (*Policy, error) {
	p, err := build(pl)
	if err != nil {
		return nil, err
	}
	root := key(p.goos, pl.Root)
	if hires := key(p.goos, pl.HiresFolder); hires != "" && under(hires, root) {
		return nil, ErrRootHasDesks
	}
	if slices.ContainsFunc(pl.Worktrees, func(folder string) bool { return under(key(p.goos, folder), root) }) {
		return nil, ErrRootHasDesks
	}
	if checkout := key(p.goos, pl.Checkout); checkout != "" && checkout != root && under(checkout, root) {
		return nil, ErrRootHasDesks
	}
	return p, nil
}

// build is New's Policy, but for the roots New refuses for holding desks: the tests check the rows hold for them too.
func build(pl Places) (*Policy, error) {
	goos := pl.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	b := &builder{goos: goos, home: key(goos, pl.Home)}
	for _, place := range []string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GMFolder, pl.GMProjects} {
		if k := key(goos, place); k != "" {
			b.work = append(b.work, k)
		}
	}
	supports := append([]string{pl.Support}, pl.OtherSupports...)
	for _, place := range append([]string{pl.Home, pl.Root}, supports...) {
		if key(goos, place) == "" {
			return nil, ErrNotWhole
		}
	}
	// A GM's workspace is the one place a desk works in that is inside a support folder (FXGMROOT): in its core's, in
	// the GM's own folder there, <Support>/hires/<id>/work. It is the GM's Root when the GM has GMProjects, and another
	// desk's GMFolder. Nothing else in a support folder is a place.
	ownHires := key(goos, path.Join(spell(goos, pl.Support), "hires"))
	workspace := func(k string) bool { return path.Base(k) == workspaceName && path.Dir(path.Dir(k)) == ownHires }
	if pl.GMProjects != "" || pl.GMPlans != "" {
		projects, plans := key(goos, pl.GMProjects), key(goos, pl.GMPlans)
		if pl.Subject != GM || !plainName(pl.OwnID) || key(goos, pl.Root) != path.Join(ownHires, foldName(goos, pl.OwnID), workspaceName) ||
			projects == "" || pl.GMPlans != "" && (plans == "" || plans == projects || !under(plans, projects)) {
			return nil, ErrGMProjects
		}
	}
	if pl.GMStart != "" || pl.GMStartPlans != "" {
		start, plans := key(goos, pl.GMStart), key(goos, pl.GMStartPlans)
		if pl.Subject == GM || start == "" || pl.GMStartPlans != "" && (plans == "" || plans == start || !under(plans, start)) {
			return nil, ErrGMStart
		}
	}
	for _, hires := range pl.GMHires {
		hk := key(goos, hires)
		if pl.Subject != GM || hk == "" || under(key(goos, pl.Home), hk) ||
			pl.GMProjects != "" && under(key(goos, pl.GMProjects), hk) || pl.GMPlans != "" && under(key(goos, pl.GMPlans), hk) ||
			slices.ContainsFunc(supports, func(s string) bool { return under(hk, key(goos, s)) || under(key(goos, s), hk) }) {
			return nil, ErrGMHires
		}
	}
	for _, folder := range pl.Worktrees {
		wk := key(goos, folder)
		if wk == "" || under(key(goos, pl.Home), wk) || pl.Projects != "" && under(key(goos, pl.Projects), wk) ||
			slices.ContainsFunc(supports, func(s string) bool { return under(wk, key(goos, s)) || under(key(goos, s), wk) }) {
			return nil, ErrWorktrees
		}
		// A GM's own places are never in it: its row there is final for the GM, so its place would be denied it, and an
		// exception that opened it would open every hire's tree to the GM.
		if pl.Subject == GM && slices.ContainsFunc([]string{pl.Root, pl.Checkout, pl.GMProjects, pl.GMPlans}, func(place string) bool {
			return place != "" && under(key(goos, place), wk)
		}) {
			return nil, ErrWorktrees
		}
	}
	// The projects folder (FXHIREPROJECTS): never in a support folder, never the home folder or a folder above it, and
	// never a GM's with projects of its own, whose row holds them already. It may hold a support folder (a test copy's
	// in the source checkout): the desk's exceptions there open its row as they open the support folder's.
	projects := key(goos, pl.Projects)
	if pl.Projects != "" {
		if projects == "" {
			return nil, ErrNotWhole
		}
		if pl.GMProjects != "" || under(key(goos, pl.Home), projects) ||
			slices.ContainsFunc(supports, func(s string) bool { return under(projects, key(goos, s)) }) {
			return nil, ErrProjects
		}
	}
	// The links on the way to it (FXHIREPROJECTSR3) come only with it.
	if len(pl.ProjectsLinks) > 0 {
		if pl.Projects == "" {
			return nil, ErrProjects
		}
		for _, link := range pl.ProjectsLinks {
			if key(goos, link) == "" {
				return nil, ErrNotWhole
			}
		}
	}
	const rootAt, gmFolderAt = 0, 3
	for i, root := range []string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GMFolder, pl.GMProjects} {
		if root == "" {
			continue
		}
		rk := key(goos, root)
		if rk == "" {
			return nil, ErrNotWhole
		}
		if workspace(rk) && (i == rootAt && pl.GMProjects != "" || i == gmFolderAt && pl.Subject != GM) {
			continue
		}
		for _, s := range supports {
			switch sk := key(goos, s); {
			case under(rk, sk):
				return nil, ErrRootInSupport
			case under(sk, rk):
				return nil, ErrRootHasSupport
			}
		}
	}
	// On a Mac, no desk's place is the projects folder or the home folder, or holds one (FXHIREPROJECTSR4, the owner's
	// ruling of 18 Nov: a hire writes only its own project folder, one hire per repo). Its place would be every repo
	// there, their git folders and their nested and later repos, which no row can keep it to one of: the core refuses
	// such a hire at its hire, and this is the backstop, whatever route made it (a place that holds a support folder,
	// home with the core's own in it, is refused above, as before). Elsewhere it gets no row, as before.
	if root := key(goos, pl.Root); goos == "darwin" && (projects != "" && under(projects, root) || under(key(goos, pl.Home), root)) {
		return nil, ErrProjectsWhole
	}
	if !plainName(pl.OwnID) || pl.OwnWorktree != "" && !plainName(pl.OwnWorktree) || pl.DefaultBranch != "" && !branchName(pl.DefaultBranch) ||
		pl.ReapMark != "" && !ReapMarkValid(pl.ReapMark) ||
		pl.DefaultBranchNow != "" && !branchName(pl.DefaultBranchNow) ||
		pl.CheckoutBranch != "" && !branchName(pl.CheckoutBranch) {
		return nil, ErrBadName
	}
	for _, name := range pl.OwnBranches {
		if own, ok := strings.CutPrefix(name, "hire/"); !ok || !plainName(own) {
			return nil, ErrBadName
		}
	}
	for _, name := range pl.OtherWorktrees {
		if !plainName(name) {
			return nil, ErrBadName
		}
	}
	if pl.OwnSockets != "" {
		own, parent := key(goos, pl.OwnSockets), key(goos, pl.SocketParent)
		switch {
		case own == "":
			return nil, ErrNotWhole
		case parent == "" || own == parent || !under(own, parent):
			return nil, ErrOwnSockets
		}
	}
	join := func(base string, names ...string) string {
		return path.Join(append([]string{spell(goos, base)}, names...)...)
	}
	home := pl.Home

	// The owner's own places in the home folder, held where their links lead too (followLinks, FXHOMELINKS).
	owners := len(b.rules)
	if pl.RemoteLogin {
		b.add(Rule{Row: rowSSH, Path: join(home, ".ssh"), Tail: "*", Below: true, Read: Deny, Write: Deny, Wide: true})
		for _, tail := range []string{"*.pub", "known_hosts*", "config"} {
			b.add(Rule{Row: "ssh files that aren't keys", Path: join(home, ".ssh"), Tail: tail, Read: Allow, Write: Allow, Opens: []string{rowSSH}})
		}
	}
	plants := []string{".zshrc", ".zshenv", ".zprofile", ".zlogin", ".zlogout", ".bashrc", ".bash_profile", ".bash_login",
		".bash_logout", ".profile", ".gitconfig", ".tmux.conf", ".ssh/config", ".ssh/rc"}
	for _, name := range plants {
		var more []string
		if name == ".zshrc" {
			more = omzNames
		}
		if slices.Contains(zshStartFiles, name) {
			more = append(slices.Clone(more), zshCompiled) // zsh runs <file>.zwc in its place (B1)
		}
		b.ownersBeside(Rule{Row: "plants the owner's programs run later", Path: join(home, name), Write: Deny}, more...)
	}
	// What zsh, bash and Terminal read or run at a new window or a restored one besides the plants above (FXSWEEP3P1,
	// RT126SWEEP3 P1's B1 and B2; the report lists each file they read, held or not, and why): zsh's compiled start
	// files (each plant's <file>.zwc, above), its completion dump, which compinit sources (`.zcompdump`, oh-my-zsh's
	// `.zcompdump-<host>-<version>`, their compiled copies and compdump's `.<host>.<pid>` beside), the key codes
	// /etc/zshrc sources (`.zkbd`), and Terminal's restored sessions, which /etc/zshrc_Apple_Terminal and
	// /etc/bashrc_Apple_Terminal source at a window's start (`<id>.session`): each folder whole, so no window's file is
	// made there. zsh reads its own in ${ZDOTDIR:-~}: so in the home folder, in each place the owner's environment
	// moves ZDOTDIR to (ShellHomes), and in zsh's folder in ~/.config and where the environment moves that, where most
	// set it from ~/.zshenv (which the core can't read). bash's sessions are always in the home folder, and readline
	// reads ~/.inputrc, whose bindings type commands. Then what most owners' ~/.zshrc sources (FXSWEEP3P1R1):
	// oh-my-zsh, whole, in the home folder and where its installer puts it beside ZDOTDIR's files, and Powerlevel10k's
	// settings in each zsh folder, where `p10k configure` writes them. On a Mac. Here, before the plant folders: the
	// start's link walk counts these small places first, so a plant folder too big to look through is still the one its
	// words name.
	if goos == "darwin" {
		zdots := []string{home, join(home, ".config", "zsh")}
		for _, config := range pl.ConfigHomes {
			zdots = append(zdots, join(config, "zsh"))
		}
		zdots = append(zdots, pl.ShellHomes...)
		seen := map[string]bool{}
		for _, zdot := range zdots {
			k := key(goos, zdot)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			if k != key(goos, home) {
				for _, name := range zshStartFiles {
					more := []string{zshCompiled}
					if name == ".zshrc" {
						more = append(slices.Clone(omzNames), zshCompiled)
					}
					b.ownersBeside(Rule{Row: "plants the owner's programs run later", Path: join(zdot, name), Write: Deny}, more...)
				}
				b.add(Rule{Row: rowShellStart, Path: join(zdot, ohMyZshBeside), Below: true, Write: Deny})
			}
			b.ownersBeside(Rule{Row: rowShellStart, Path: join(zdot, p10kConfig), Write: Deny})
			b.add(Rule{Row: rowShellStart, Path: zdot, Tail: ".zcompdump*", Write: Deny})
			b.add(Rule{Row: rowShellStart, Path: join(zdot, ".zkbd"), Below: true, Write: Deny})
			b.add(Rule{Row: rowShellSessions, Path: join(zdot, ".zsh_sessions"), Below: true, Write: Deny})
		}
		b.add(Rule{Row: rowShellSessions, Path: join(home, ".bash_sessions"), Below: true, Write: Deny})
		b.ownersBeside(Rule{Row: rowShellStart, Path: join(home, ".inputrc"), Write: Deny})
		b.add(Rule{Row: rowShellStart, Path: join(home, ohMyZsh), Below: true, Write: Deny})
		// The VM tools' SSH keys (RT126SWEEP3 P2's B2, part (b)): Lima, Colima, OrbStack, Podman and Rancher Desktop
		// machines answer SSH on a local port with these keys, and Colima, OrbStack, Podman and Rancher Desktop mount the
		// home folder writable in the VM, so a desk that read one would write the owner's files from outside its sandbox.
		// Neither read nor written. Other local servers that run commands for whoever connects are a known limit.
		for _, folder := range vmKeyFolders {
			b.add(Rule{Row: rowVMKeys, Path: join(home, folder), Below: true, Read: Deny, Write: Deny})
		}
		b.add(Rule{Row: rowVMKeys, Path: join(home, ".ssh"), Tail: "podman-machine-default*", Read: Deny, Write: Deny})
	}
	for _, folder := range []string{".config/fish", "Library/LaunchAgents", ".config/autostart", ".config/systemd/user", ".config/git", ".config/tmux"} {
		b.add(Rule{Row: "plants the owner's programs run later", Path: join(home, folder), Below: true, Write: Deny})
	}
	// …and the same places where the owner's environment moves ~/.config (O3): fish, tmux, git (its ignore and
	// attributes files as much as its config), autostart and systemd read them there instead.
	for _, config := range pl.ConfigHomes {
		for _, folder := range configPlants {
			b.add(Rule{Row: "plants the owner's programs run later", Path: join(config, folder), Below: true, Write: Deny})
		}
	}
	b.add(Rule{Row: "plants the owner's programs run later", Path: join(home, ".ssh"), Tail: "authorized_keys*", Write: Deny})
	// A git folder at the home folder (FXHIREPROJECTSR1, RT126FIXREVIEW-GMROOTR1's "seen outside"): git run in any
	// folder below home that is in no repo (~, ~/Projects, a GM's start folder) walks up and takes ~/.git, or ~ itself
	// as a bare repo once it has a HEAD, and runs what its config names: the owner's git status runs its fsmonitor, a
	// git-aware prompt too, and the owner's git branch on a terminal its pager. So no desk makes either, nor writes an
	// owner's ~/.git that is there (a dotfiles repo); a desk whose repo it is keeps its shared .git's own rows. On a
	// Mac; Linux's view isn't changed here.
	if goos == "darwin" && !under(key(goos, pl.GitDir), key(goos, join(home, ".git"))) {
		b.add(Rule{Row: rowHomeGit, Path: join(home, ".git"), Below: true, Write: Deny})
		b.add(Rule{Row: rowHomeGit, Path: join(home, "HEAD"), Write: Deny})
	}
	// …and in every folder above a place the owner's or a GM's git runs in, between it and ~ (FXHIREPROJECTSR2,
	// FXHIREPROJECTSR1's Open 1, 3 and 4): git in a folder in no repo walks up past ~/Code/clients to ~/Code/.git as it
	// does to ~/.git. The places are the desk's own, the owner's checkout, the folder for hires, where a GM with a
	// workspace starts and a GM's projects; the folder for hires and the GM's start folder hold no repo of their own,
	// so they are held themselves too, the desk's place there included. Below its own place, a .git is the desk's own
	// work. A folder whose .git holds a place of the desk's (its shared .git: an owner's repo above its place) keeps
	// that .git's own rows, as ~'s does. A place outside ~ is held up to the top. On a Mac, as ~'s.
	// The core's folders for hires' worktrees and a GM's hires' folders are such places too (RT126SWEEP4-P1 F1): the
	// owner goes there to look at a hire's work, and git in ~/.smolcorp/worktrees, or in a repo's folder there, walks up
	// to ~/.smolcorp/.git, which a GM's own place, never below it, held from no GM. They hold no repo of their own
	// either, so they are held themselves, as the folder for hires is.
	if goos == "darwin" {
		homeKey := key(goos, home)
		own := []string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GitDir, pl.Support, pl.OwnSockets, pl.DeskStart, pl.TmuxFolder, pl.GMPlans}
		seen := map[string]bool{homeKey: true} // ~ is rowHomeGit's
		places := append([]string{pl.Root, pl.Checkout, pl.Projects, pl.GMStart, pl.GMProjects}, pl.Worktrees...)
		for i, place := range append(places, pl.GMHires...) {
			if place == "" {
				continue
			}
			folder := spell(goos, place)
			if i < 2 {
				folder = parent(folder) // the desk's own place, the owner's repo
			}
			for ; parent(folder) != folder && key(goos, folder) != homeKey; folder = parent(folder) {
				k, git := key(goos, folder), join(folder, ".git")
				if seen[k] {
					continue
				}
				seen[k] = true
				if slices.ContainsFunc(own, func(p string) bool { return p != "" && under(key(goos, p), key(goos, git)) }) {
					continue
				}
				b.add(Rule{Row: rowAboveGit, Path: git, Below: true, Write: Deny})
				b.add(Rule{Row: rowAboveGit, Path: join(folder, "HEAD"), Write: Deny})
			}
		}
	}
	for _, file := range pl.GitConfigs {
		b.ownersBeside(Rule{Row: "plants the owner's programs run later: git's other config", Path: file, Write: Deny})
	}
	// What the owner's git config names for git to run or read as config (GITCFG1). A hooks folder is denied whole: a
	// hook may run what is beside it (husky's hooks source its `h`). ReadGitConfig gives the folders beside which a hook
	// runs more (husky's `.husky` above its `_`) and those a link in one leads to as hooks folders too (RT126GITCFG1).
	for _, folder := range pl.GitHooks {
		b.add(Rule{Row: "plants the owner's programs run later: git's hooks folder", Path: folder, Below: true, Write: Deny})
	}
	for _, file := range pl.GitNamed {
		b.ownersBeside(Rule{Row: "plants the owner's programs run later: what git's config names", Path: file, Write: Deny})
	}
	if goos == "darwin" {
		for _, folder := range []string{"/usr/lib/cron/tabs", "/private/var/at"} {
			b.add(Rule{Row: "plants the owner's programs run later", Path: folder, Below: true, Write: Deny})
		}
		// The terminals' and the hired agents' own settings (FXPLANTSTERM; RT126SWEEP2 P2's F1 and P1's B4, the owner's
		// ruling): Smolcorp opens a window of the owner's terminal itself, outside any sandbox, for each hire and at
		// every reattach, and that window starts with what its settings say; an agent's next session outside a desk
		// runs what its settings name. Read, never written: a desk still reads them. Each agent folder is denied whole
		// (FXPLANTSTERMR2): opening its state to a desk (FXPLANTSTERMR1) let in what the owner's own sessions load
		// (Gemini's project memory), what a folder renamed in carries (Grok's grants, a checkpoints' git), and a link
		// the owner's agent writes through (Codex's installation_id). OpenCode's data, cache and state, and the skills
		// in ~/.agents, are held the same way (FXPLANTSTERMR3). So Codex, Gemini CLI, OpenCode and Grok can't write
		// their own state in a desk: Codex, Gemini CLI and OpenCode need it to start, and Grok would lose its sessions
		// and sign-in refresh. In 1.26.0 their hires are refused on a Mac, in words, before a window opens (the owner's
		// rulings; desktop's notonamac.go), and no row here opens a place for them. A home of their own in each desk
		// comes in 1.26.1. Linux's view and Windows' jail come with their sandboxes.
		for _, folder := range terminalPlaces {
			b.add(Rule{Row: rowTerminals, Path: join(home, folder), Below: true, Write: Deny})
			// Those in ~/.config where the owner's environment moves it (O3): Ghostty, kitty and WezTerm read them there.
			if in, ok := strings.CutPrefix(folder, ".config/"); ok {
				for _, config := range pl.ConfigHomes {
					b.add(Rule{Row: rowTerminals, Path: join(config, in), Below: true, Write: Deny})
				}
			}
		}
		b.ownersBeside(Rule{Row: rowTerminals, Path: join(home, ".wezterm.lua"), Write: Deny})
		folders := []string{}
		for _, folder := range agentFolders {
			folders = append(folders, join(home, folder))
		}
		// …and where the owner's environment moves them (FXPLANTSTERMR3): the owner's agent works there instead.
		for _, folder := range append(folders, pl.AgentHomes...) {
			b.add(Rule{Row: rowAgents, Path: folder, Below: true, Write: Deny})
			// No link made in it, symbolic or hard, nor one renamed in (FXPLANTSTERMR2, R1's B3): the owner's agent
			// opens its files there following links (Codex sets installation_id's mode and rewrites it at every
			// start), so a link there would have it change any file the owner can write. The whole write deny above
			// says it too; this row says it on its own, a final deny no exception opens, should a later row open part
			// of the folder. It doesn't stop a folder holding a link from being renamed in (Seatbelt checks a rename
			// only at its two ends): only a folder no desk writes under does.
			b.add(Rule{Row: rowAgentLinks, Path: folder, Below: true, Link: Deny})
		}
		// The keychains' folder (RT126SWEEP2 P4's F1): a desk that replaced the login keychain file with one whose
		// password it knows would read every item Smolcorp and the owner's apps add there next, offline. Its files are
		// read, never written: a desk still reads an item through the Security server, as the owner's programs do
		// (§A4.4), and can't add or change one in a keychain there, nor make a keychain of its own there.
		b.add(Rule{Row: rowKeychains, Path: join(home, "Library/Keychains"), Below: true, Write: Deny})
	}
	for _, account := range append([]string{join(home, ".claude")}, pl.Accounts...) {
		b.add(Rule{Row: rowAccounts, Path: account, Below: true, Read: Allow, Write: Deny})
		for _, secret := range []string{"ide", ".credentials.json", "history.jsonl", "projects", "sessions"} {
			r := Rule{Row: "the account folders' tokens and history", Path: join(account, secret), Below: true, Read: Deny, Write: Deny}
			if strings.Contains(secret, ".") {
				b.beside(r, anyAfter) // a file: read in its temporary copy as much as in place
			} else {
				b.add(r)
			}
		}
		b.add(Rule{Row: "the accounts' .claude.json", Path: account, Tail: ".claude.json*", Read: Deny, Write: Deny})
	}
	b.add(Rule{Row: "the accounts' .claude.json", Path: join(home), Tail: ".claude.json*", Read: Deny, Write: Deny})
	// Joined as the Policy's system spells them, not this computer's: a Mac's Policy made on Windows stays in /.
	signIns := []string{}
	for _, names := range credentialNames {
		signIns = append(signIns, join(home, names...))
	}
	if goos == "darwin" {
		for _, names := range macCredentialNames {
			signIns = append(signIns, join(home, names...))
		}
		signIns = append(signIns, pl.AgentSignIns...)
	}
	for _, at := range signIns {
		r := Rule{Row: "other agents' sign-ins", Path: at, Read: Deny, Write: Deny}
		if strings.Contains(path.Base(spell(goos, at)), ".") {
			b.beside(r, anyAfter) // a file: read in its temporary copy as much as in place
		} else {
			r.Below = true // a folder (Codex's secrets), whole
			b.add(r)
		}
	}
	if goos == "linux" {
		for _, file := range []string{join(home, ".Xauthority"), pl.XAuthority} {
			if file != "" {
				b.add(Rule{Row: "X11's key", Path: file, Read: Deny, Write: Deny})
			}
		}
	}
	b.followLinks(owners, home, slices.Concat(pl.Accounts, pl.AgentHomes, pl.ConfigHomes, pl.ShellHomes))
	// …and below a desk's place (FXHIREPROJECTSR6, RT126FIXREVIEW-HIREPROJECTSR5 B1 and N2): a git
	// folder there that isn't the desk's own is another repo's, whose hooks and config the owner's git runs outside any
	// sandbox: a nested clone in a checkout (lib/vendor/x), a repo in a folder of repos (clients/acme, under a projects
	// folder or a home that is itself a repo). So a desk writes, links and renames nothing in any `.git` strictly below
	// its place, a folder or a file, in any case, nor makes, replaces or renames one in there. The match is a whole
	// name, `.git`: `.github` and `.gitignore` stay its work. Its own `.git` is at its place, never strictly below:
	// the shared .git of a desk at a checkout, the file of a worktree's. A place that isn't its repo's top (in no repo,
	// or a folder inside one) gets no `.git` of its own either, nor a HEAD there, which git takes for a bare repo's.
	// The rows are final; the pattern leaves out a package manager's own folders (ToolsGits). They come after the
	// owner's places are followed through their links (followLinks), which would walk the whole place for the pattern:
	// a link in the place is judged where it leads, and a .git it leads to below the place matches there. A pattern doesn't follow a folder moved out of the place and back,
	// so the folders on the way to each git folder found at the start (GitBelow) are held by name. Left open: a folder
	// made outside the place with a .git in it, renamed in after the start (the next start of a place in no repo refuses
	// it; in a repo the core's list of the git folders each start found refuses it at the next start: desktop's
	// gitsSince), and a bare repo made there (the next start refuses it). A package manager's own folders (ToolsGits)
	// aren't held by the pattern: the clones and mirrors there at the start are, one by one (ToolsGitBelow).
	if goos == "darwin" {
		root := key(goos, pl.Root)
		b.add(Rule{Row: rowGitBelow, Path: pl.Root, Tail: "*/**/.git", Below: true, Write: Deny, ExceptTails: ToolsGits})
		if gitAt := join(pl.Root, ".git"); key(goos, gitAt) != key(goos, pl.GitDir) && pl.OwnWorktree == "" {
			b.add(Rule{Row: rowGitBelow, Path: gitAt, Below: true, Write: Deny})
			b.add(Rule{Row: rowGitBelow, Path: join(pl.Root, "HEAD"), Write: Deny})
		}
		for _, git := range pl.GitBelow {
			k := key(goos, git)
			if k == "" || !under(k, root) || parent(k) == root || path.Base(k) != foldName(goos, ".git") {
				return nil, ErrGitBelow
			}
			for folder := parent(spell(goos, git)); key(goos, folder) != root; folder = parent(folder) {
				b.add(Rule{Row: rowGitBelow + ": the folders on the way to one", Path: folder, Move: Deny, Link: Deny})
			}
		}
		// A package manager's own clones and mirrors there at the start (FXHIREPROJECTSR8, N1): each held whole, as the
		// .git rule holds every other, and the folders on the way to it by name, so none is moved out and back.
		for _, git := range pl.ToolsGitBelow {
			k := key(goos, git)
			if k == "" || !under(k, root) || k == root || !InToolsGits(goos, pl.Root, git) {
				return nil, ErrGitBelow
			}
			b.add(Rule{Row: rowGitBelow + ": a package manager's own, there at the start", Path: git, Below: true, Write: Deny})
			for folder := parent(spell(goos, git)); key(goos, folder) != root; folder = parent(folder) {
				b.add(Rule{Row: rowGitBelow + ": the folders on the way to one", Path: folder, Move: Deny, Link: Deny})
			}
		}
	}
	for _, root := range pl.InstallRoots {
		b.add(Rule{Row: "programs that run outside", Path: root, Below: true, Write: Deny})
	}

	// Other desks' places, then the desk's own, then the shared git places inside it. The desk's own place opens the
	// hires folder and the owner's checkout only inside them: a place that holds them opens nothing (B2).
	if pl.HiresFolder != "" {
		b.add(Rule{Row: rowOtherDesks, Path: pl.HiresFolder, Below: true, Write: Deny, Wide: true})
	}
	if pl.GMFolder != "" && pl.Subject != GM {
		b.add(Rule{Row: "other desks' places: the GM's folder", Path: pl.GMFolder, Below: true, Write: Deny})
	}
	// A GM's hires' folders (FXGMHIRENOGIT126R1, ruling (2)): read, never written, so the trees the core makes there are
	// never the GM's to swap or plant in while the core's git works on them.
	for _, hires := range pl.GMHires {
		b.add(Rule{Row: rowOtherDesks, Path: hires, Below: true, Write: Deny, Wide: true})
	}
	// The core's folders for hires' worktrees (FXCOREWORKTREES126): every desk's own tree is its place there, which opens
	// this row for it alone; a GM has none there (ErrWorktrees), so for a GM the whole folder is read only. And the
	// folders above each, up to the home folder: no desk moves, removes or makes one, so none is put aside and another
	// made in its name for the core's next tree.
	for _, folder := range pl.Worktrees {
		if pl.Subject == GM {
			b.add(Rule{Row: rowOtherDesks, Path: folder, Below: true, Write: Deny}) // final for a GM: nothing opens it
		} else {
			b.add(Rule{Row: rowOtherDesks, Path: folder, Below: true, Write: Deny, Wide: true})
		}
		home := key(goos, pl.Home)
		for above := parent(spell(goos, folder)); ; above = parent(above) {
			k := key(goos, above)
			if k == "" || k == home || under(home, k) {
				break
			}
			b.add(Rule{Row: rowWorktreesAbove, Path: above, Write: Deny})
			if parent(above) == above {
				break
			}
		}
	}
	// A GM with a workspace (FXGMROOTR1 N2): what it reads from the folder it starts in, which its workspace doesn't
	// hold. Its plans, the one place it writes there, which its plan mode shows the owner and then follows; and the
	// files Claude Code loads into its session from there, as the owner's plants are held: with their writers' names.
	gmStartWhole := false
	if pl.GMStart != "" && pl.Subject != GM {
		// The GM in a folder of its own (FXHIREPROJECTSR1, RT126FIXREVIEW-GMROOTR1 N1 (a)), one that holds none of this
		// desk's places: all of it, as the base held the GM's folder, with what Claude Code loads into the GM's session
		// from there (its playbook and briefs, a .claude's agents, commands and skills, a .git whose config git runs).
		// A start folder that holds this desk's places, the folder for hires by default, is held by the rows below and
		// by "the projects folder: only its own worktree".
		start := key(goos, pl.GMStart)
		if gmStartWhole = !slices.ContainsFunc([]string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GitDir, pl.Support, pl.OwnSockets, pl.DeskStart, pl.TmuxFolder},
			func(place string) bool { return place != "" && under(key(goos, place), start) }); gmStartWhole {
			b.add(Rule{Row: "other desks' places: the GM's folder", Path: pl.GMStart, Below: true, Write: Deny})
		}
		if pl.GMStartPlans != "" {
			b.add(Rule{Row: rowGMStart + ": its plans", Path: pl.GMStartPlans, Below: true, Write: Deny})
		}
		for _, name := range gmStartFiles {
			b.ownersBeside(Rule{Row: rowGMStart + ": the agent files there", Path: join(pl.GMStart, name), Write: Deny})
		}
	}
	if pl.Checkout != "" && key(goos, pl.Checkout) != key(goos, pl.Root) {
		b.add(Rule{Row: rowCheckout, Path: pl.Checkout, Below: true, Write: Deny, Wide: true})
	}
	// The folder for hires (FXHIREPROJECTS, RT126FIXREVIEW-GMROOT's sibling repo; the owner's ruling, option (a)):
	// ~/Projects by default, which holds every repo of the owner's, their shared .git (hooks, config) and other desks'
	// worktrees, and any repo made there later. A desk writes there only its own place and its repo's shared .git, whose
	// own rows still hold its shared places, as before; it reads all of it, as before. The folders above it are held as a
	// desk's place's are: not moved, removed or linked (denyAbove), nor their mode, flags or ACL changed. A desk whose
	// place is the projects folder, or holds it, gets none of this: its place is all of it (off a Mac; a Mac refuses it
	// above).
	root := key(goos, pl.Root)
	projectsRows := func(folder string) {
		b.add(Rule{Row: rowProjects, Path: folder, Below: true, Write: Deny, Wide: true})
		for folder := parent(spell(goos, folder)); !under(root, key(goos, folder)); folder = parent(folder) {
			b.add(Rule{Row: rowProjects + ": the folders above it", Path: folder, Write: Deny}) // those above its Root are held already
		}
	}
	if projects != "" && !under(projects, root) {
		projectsRows(pl.Projects)
	}
	// A GM with a workspace started in a folder that holds this desk's places but isn't the folder for hires
	// (FXHIREPROJECTSR2, FXHIREPROJECTSR1's Open 1): Setup's other folder of repos, the GM's projects. It is held as the
	// folder for hires is, by the same row, so the desk writes there only its own place and its repo's shared .git, and
	// nothing the GM's Claude Code loads at its top (its playbook, the owner's briefs, a .claude's contents). Not where
	// the row would hold what a desk's work needs that no exception of its opens (home, the support folder, its sockets
	// and tmux's), nor where the folder for hires holds it already; a desk whose place is it, or holds it, gets none.
	if start := key(goos, pl.GMStart); pl.GMStart != "" && pl.Subject != GM && !gmStartWhole && !under(start, root) &&
		!(projects != "" && !under(projects, root) && under(start, projects)) &&
		!slices.ContainsFunc([]string{home, pl.Support, pl.OwnSockets, pl.DeskStart, pl.TmuxFolder},
			func(place string) bool { return place != "" && under(key(goos, place), start) }) {
		projectsRows(pl.GMStart)
	}
	// The links on the way to the folder for hires as Setup spells it (FXHIREPROJECTSR3, N2): a desk that pointed one
	// elsewhere would move every later desk's projects folder, the row above held where the link no longer leads.
	for _, link := range pl.ProjectsLinks {
		b.add(Rule{Row: rowProjectsLinks, Path: link, Write: Deny})
	}
	b.add(Rule{Row: "its own place", Path: pl.Root, Below: true, Read: Allow, Write: Allow, Opens: []string{rowOtherDesks, rowCheckout, rowProjects}})
	if pl.GitDir != "" {
		git := pl.GitDir
		b.add(Rule{Row: "the rest of the shared .git", Path: git, Below: true, Write: Allow, Opens: []string{rowCheckout, rowProjects}})
		// The shared git places: the repo's config, hooks and objects' list of other stores, and every file git reads to
		// find where those are (gitrepository-layout; SIERRA, 14 Nov): `commondir` names the folder git takes them all
		// from, for the owner's checkout as for a worktree, and `gitdir` is denied with it. `info` holds `alternates`
		// too, should a git read it there; `objects/info` holds `alternates` and `http-alternates`.
		for _, name := range []string{"config", "config.worktree", "index", "HEAD", "commondir", "gitdir"} {
			b.beside(Rule{Row: "the shared git places", Path: join(git, name), Write: Deny}, gitLock)
		}
		for _, folder := range []string{"hooks", "info", "objects/info"} {
			b.add(Rule{Row: "the shared git places", Path: join(git, folder), Below: true, Write: Deny})
		}
		// What makes the shared .git a git folder at all (RT126SAX1R5 B3): a valid HEAD, denied above, and an objects
		// and a refs folder git can search. A .git folder that fails this isn't an error to git: it walks up and takes
		// the first git folder above the checkout, a `~/.git` or a bare repo in `~/Projects` any desk may make, and the
		// owner's git in the owner's checkout then runs with that folder's config and hooks. So no desk changes these
		// entries themselves (their mode, ACL, flags or name); what git makes inside them is still the desk's work. A
		// desk at the project has the .git denied already, as the .git at its place.
		for _, entry := range []string{"", "objects", "refs"} {
			b.add(Rule{Row: "the shared git places: what makes it a git folder", Path: join(git, entry), Write: Deny})
		}
		// The owner's rebase or am in the checkout (RT126SAX1R5): their todo lists hold `exec` lines, which the owner's
		// `git rebase --continue` runs. A desk in its own worktree keeps its own in its registration and loses nothing; a
		// desk at the project can't rebase or am there, nor pick or revert a range of commits.
		for _, folder := range []string{"rebase-merge", "rebase-apply", "sequencer"} {
			b.add(Rule{Row: "the shared git places: the owner's rebase", Path: join(git, folder), Below: true, Write: Deny})
		}
		// …and, on a Mac, what else the owner checkout's git keeps of a merge, pick, revert or rebase in progress, for a
		// desk in its own worktree (FXBASEHEADS, SIERRA's ruling #3): a MERGE_HEAD of a desk's commit makes the owner's
		// next `git commit` on main a merge of it, real history then, which no read-around undoes; the others are what
		// the owner's `reset --hard ORIG_HEAD`, `--continue` and `--abort` take. A desk in its own worktree loses
		// nothing: git keeps its own in its registration, worktrees/<id>/. A desk at the project keeps them, its own.
		// Each is written through its lock (MERGE_HEAD in place, its lock denied all the same).
		if goos == "darwin" && pl.Checkout != "" && key(goos, pl.Checkout) != key(goos, pl.Root) {
			// The checkout's reflog too (RT126SWEEP2's F2): the owner's `reset --hard HEAD@{1}` and `checkout -` take it.
			for _, name := range ownersStateNames {
				b.beside(Rule{Row: rowOwnersState, Path: join(git, name), Write: Deny}, gitLock)
			}
		}
		// The submodules' git folders, whole, for every desk (RT126SAX1R4 B1‴): any folder in there, brought in whole by
		// a rename with its config and hooks inside, the owner's git could later take for a submodule's, once the desk
		// had broken the git folder above it with one write (its HEAD). A desk in its own worktree loses nothing by it:
		// git keeps that worktree's submodules in its own registration, worktrees/<id>/modules (held on a Mac too, below).
		// A desk at the project loses its submodule work (SANDBOX-PLAN §A4.1, the shared git places): it can't init,
		// update, fetch, check out or commit in a submodule, which all write here; it still reads them, and works in
		// the superproject, a submodule's new commit included once the owner has fetched it.
		b.add(Rule{Row: "the shared git places: the submodules' git folders", Path: join(git, "modules"), Below: true, Write: Deny})
		// A worktree's registration is made, moved and removed by the core, never by a desk; in each, the files that
		// say where its shared .git is (`commondir`) and where it is (`gitdir`), and its own config (read when the
		// owner's config turns worktreeConfig on). The desk's own registration included: the core and the owner run git
		// in its worktree too. Its HEAD, index, logs and its rebase's and merge's state stay its own: none of them names a
		// program for the owner's plain `git status` there (FXHIREPROJECTSR9's measure), and the index's gitlinks are
		// looked at at each start (desktop's gitlinkWords). The config of its submodules, in worktrees/<id>/modules, is
		// read by any git that recurses into them there, the core's own `git status` at a fire included (RT126SAX1R5), so
		// every git the core runs in a desk's place has gitReadArgs' settings (desktop/gitstate.go) and
		// --ignore-submodules=all.
		worktrees := join(git, "worktrees")
		b.add(Rule{Row: "the shared git places: a worktree's registration", Path: worktrees, Tail: "*", Write: Deny})
		for _, name := range []string{"commondir", "gitdir", "config.worktree"} {
			b.beside(Rule{Row: "the shared git places: where a worktree finds them", Path: worktrees, Tail: "*/" + name, Write: Deny}, gitLock)
		}
		// …and, on a Mac, a worktree's submodules' git folders, whole (FXHIREPROJECTSR9, RT126FIXREVIEW-HIREPROJECTSR8
		// B2): the owner's plain `git status` in a hire's worktree goes into each submodule checked out there, with that
		// submodule's config, hooks and info from worktrees/<id>/modules/<name>, and a git folder's config can be
		// redirected (its commondir, an include.path, a nested modules), so no list of held files would do. The core
		// makes each one at the hire, for each submodule the owner's checkout has (desktop's initHireSubmodules). A
		// desk in its own worktree still edits a submodule's files there, but can't commit in a submodule, initialise
		// one, or move its checkout to another commit.
		if goos == "darwin" {
			b.add(Rule{Row: rowWorktreeModules, Path: worktrees, Tail: "*/modules", Below: true, Write: Deny})
		}
		// The .git at the desk's place: in a worktree, the file that names its registration (and so the shared .git);
		// at the project, the shared .git itself.
		b.add(Rule{Row: "the shared git places: the .git at its place", Path: join(pl.Root, ".git"), Write: Deny})
		for _, name := range pl.OtherWorktrees {
			if name != pl.OwnWorktree {
				b.add(Rule{Row: "other desks' places: their worktree's registration", Path: join(git, "worktrees", name), Below: true, Write: Deny})
			}
		}
		// The base branches stay the owner's to move (SIERRA, 14 Nov): a hire's commits go on its own branch.
		// packed-refs' lock is only held, never renamed onto it (git 2.55 renames packed-refs.new), and a desk's own
		// commit, rebase and branch deletion take it, to drop a ref from it: it stays the desk's (FXLOCKSIBLINGS).
		b.beside(Rule{Row: "the base branches", Path: join(git, "packed-refs"), Write: Deny}, gitNew)
		for _, branch := range []string{"main", "master", pl.DefaultBranch, pl.DefaultBranchNow} {
			if branch != "" {
				b.beside(Rule{Row: "the base branches", Path: join(git, "refs/heads", branch), Write: Deny}, gitLock)
			}
		}
		b.add(Rule{Row: "the base branches", Path: join(git, "refs/heads/release"), Below: true, Write: Deny})
		// In a reftable repo (`git init --ref-format=reftable`) every ref of the shared .git is in reftable/: no branch
		// can be told from another there, so no desk moves any, its own branch included (GITCFG1). A repo of files has
		// no such folder, and git reads one only when the repo's config, which no desk writes, says so.
		b.add(Rule{Row: "the base branches: a reftable repo's refs", Path: join(git, "reftable"), Below: true, Write: Deny})
		// git's substitutions (FXREPLACEREFS), on a Mac: the owner's git takes each of these over what a base branch
		// names, while the branch itself never moves. A replace ref (`git replace <main's commit> <a desk's>`) makes the
		// owner's show, log, merge and worktree add of main read the desk's commit; the shallow list cuts main's history
		// where it says (log, merge-base); a multi-pack index tells git where an object's bytes are, so it can give
		// main's file a desk's bytes, to the owner's checkout as to the core's; a base branch's reflog is what the
		// owner's plain `git rebase` takes for its upstream's own commits (--fork-point), and drops from their branch.
		// The grafts (`info/grafts`), the other stores' lists and the commit-graph (`objects/info`) are the shared git
		// places' already. A desk loses no work: none of these is written by its commits, merges or gc (a gc writes a
		// pack, its index and reverse index, and tries the commit-graph), only by `git replace`, a shallow fetch,
		// `multi-pack-index write` and a base branch's move; its own branch's reflog is in logs/refs/heads/hire/<id>.
		// Linux's view isn't changed here (FXREPLACEREFS' report lists it for the VM).
		if goos == "darwin" {
			b.add(Rule{Row: rowSubstitutions, Path: join(git, "refs/replace"), Below: true, Write: Deny})
			b.beside(Rule{Row: rowSubstitutions, Path: join(git, "shallow"), Write: Deny}, gitLock)
			b.add(Rule{Row: rowSubstitutions, Path: join(git, "objects/pack"), Tail: "multi-pack-index*", Below: true, Write: Deny})
			for _, branch := range []string{"main", "master", pl.DefaultBranch, pl.DefaultBranchNow} {
				if branch != "" {
					b.beside(Rule{Row: rowBaseReflogs, Path: join(git, "logs/refs/heads", branch), Write: Deny}, gitLock)
				}
			}
			b.add(Rule{Row: rowBaseReflogs, Path: join(git, "logs/refs/heads/release"), Below: true, Write: Deny})
			// A base branch's name elsewhere (FXBASEHEADS, SIERRA's ruling #2): the owner's git looks for `main` as
			// `$G/main`, `refs/main` and `refs/tags/main` before `refs/heads/main`, and takes the first it finds with only a
			// warning, in its show, merge, log and worktree add of main; `release/1.26` the same. So none of them is a
			// desk's, nor their locks (`git tag main` writes refs/tags/main.lock), and refs/tags holds against a move
			// (denyAbove), or a desk would move it aside, write, and move it back. A desk still tags any other name.
			for _, branch := range []string{"main", "master", pl.DefaultBranch, pl.DefaultBranchNow} {
				if branch != "" {
					for _, at := range []string{"", "refs", "refs/tags"} {
						b.beside(Rule{Row: rowBaseNames, Path: join(git, at, branch), Write: Deny}, gitLock)
					}
				}
			}
			for _, at := range []string{"", "refs", "refs/tags"} {
				b.add(Rule{Row: rowBaseNames, Path: join(git, at, "release"), Below: true, Write: Deny})
			}
		}
		// The old remote files (RT126GITCFG2's note 3; SIERRA, 14 Nov): `branches/<name>` and `remotes/<name>` give a
		// remote config doesn't name an address, which the owner's fetch, pull and push use, and an address can name a
		// program (`ext::`, a `<helper>::`). They aren't config, so gitstate never sees them. Git reads them from the
		// shared .git only; each worktree's registration is denied as well (SIERRA), and the submodules' are in modules/,
		// denied whole. Git hasn't written them for years: a desk loses nothing.
		for _, folder := range []string{"branches", "remotes"} {
			b.add(Rule{Row: rowOldRemotes, Path: join(git, folder), Below: true, Write: Deny})
			b.add(Rule{Row: rowOldRemotes, Path: worktrees, Tail: "*/" + folder, Below: true, Write: Deny})
		}
		b.gitRows2(pl, git) // FXGITROWS2: the default branch's name, rerere, other desks' branches (policy_gitrows2.go)
	}

	// Anywhere: the fences, and the folders a desk's place hangs from.
	b.add(Rule{Row: "a .claude folder can't be made, moved or removed", Tail: "**/.claude", Write: Deny})
	b.beside(Rule{Row: "fences and hooks", Tail: "**/.claude/settings.json", Write: Deny}, anyAfter)
	b.beside(Rule{Row: "fences and hooks", Tail: "**/.claude/settings.local.json", Write: Deny}, anyAfter)
	// Each of these folders is denied writing as a whole, the desk's own root included, so its own mode, flags, ACL,
	// owner and xattrs are refused too: a tool's `chmod` of the worktree root itself fails, while everything inside it is
	// the desk's as before. That was so before FXGITROWS2's F4, which adds nothing here (RT126FIXREVIEW-GITROWS2 O6). It
	// keeps the root as the core made it, readable and removable by the fire's look and `git worktree remove`.
	for _, place := range append([]string{pl.Root, pl.Checkout}, supports...) {
		for folder := spell(goos, place); place != ""; folder = parent(folder) {
			b.add(Rule{Row: rowPlaceAbove, Path: folder, Write: Deny})
			if parent(folder) == folder {
				break
			}
		}
	}
	if pl.SocketParent != "" {
		b.add(Rule{Row: rowSockets, Path: pl.SocketParent, Below: true, Write: Deny, Wide: true})
		b.add(Rule{Row: "no links anywhere in the socket folder", Path: pl.SocketParent, Below: true, Link: Deny})
		if pl.OwnSockets != "" {
			b.add(Rule{Row: rowOwnSockets, Path: pl.OwnSockets, Below: true, Read: Allow, Write: Allow, Opens: []string{rowSockets}})
		}
	}
	// tmux's own folder (FXTMUXDIR), whole, as the socket folder is but with no exception: no socket made, removed,
	// renamed or bound there, and no link, so no server of a desk's takes a name the core, the Mac app or the owner's
	// own tmux connects to by its path, and no path there is made to lead to a desk's socket. The desk-start socket
	// may be in it: connecting is another operation, which its own row allows.
	if goos == "darwin" && pl.TmuxFolder != "" {
		b.add(Rule{Row: rowTmuxFolder, Path: pl.TmuxFolder, Below: true, Write: Deny})
	}

	// The support folder, whole, with the desk's own exceptions in its core's. Another support folder has none.
	b.add(Rule{Row: rowSupport, Path: pl.Support, Below: true, Read: Deny, Write: Deny, Connect: Deny, Wide: true})
	for _, s := range pl.OtherSupports {
		b.add(Rule{Row: "another support folder, whole", Path: s, Below: true, Read: Deny, Write: Deny, Connect: Deny})
	}
	// Each support folder by its other name, whole, with no exception (FXRENAME126A): Smolcorp's own beside the old
	// one, and the old one beside a Smolcorp one, there or not. The core uses one of them (appdirs.Resolve), and a desk
	// reaches neither.
	for _, s := range supportSiblings(goos, supports) {
		b.add(Rule{Row: rowSupportOtherName, Path: s, Below: true, Read: Deny, Write: Deny, Connect: Deny})
	}
	// A support folder in the projects folder (a test copy's, in the source checkout) is the desk's to use as any other:
	// its exceptions open the projects folder's row too.
	support, opens := pl.Support, []string{rowSupport, rowProjects}
	b.add(Rule{Row: "the guard file: the guard reads it", Path: join(support, "hire-guard.json"), Read: Allow, Opens: opens})
	// The night shift's jobs (NS1), kept as the guard file is: a hire or the GM may read them (they hold no secret, and
	// the guard lets its readers name the file), only the core writes them, and an approval a desk wrote would start
	// hires overnight. The copy the core sets aside unread (nightshift.json.bad) has no exception.
	// Its row names no feature (FXPOLISHA, RT126FIXREVIEW-DORMANTOFF's O4): a desk reads it in its profile, and the night
	// shift is off in this version.
	b.add(Rule{Row: "a file only the core writes: read only, as the guard file", Path: join(support, "nightshift.json"), Read: Allow, Opens: opens})
	own := join(support, "hires", pl.OwnID)
	b.add(Rule{Row: "its status and report", Path: own, Tail: "*.md", Read: Allow, Write: Allow, Opens: opens})
	for _, tail := range []string{"BRIEF*.md", "settings.json", "sandbox.sb"} {
		b.add(Rule{Row: "its brief and its fence", Path: own, Tail: tail, Read: Allow, Write: Deny, Opens: opens})
	}
	// Its mark (FXFIREREAPR5): read only, and the only place in the support folder no other desk's sandbox may read. Its
	// folder stays denied, as the rest of its own folder is, so a process given a read extension for a folder that holds
	// the mark reads that folder too, and isn't taken for the desk's.
	if goos == "darwin" && pl.ReapMark != "" {
		b.add(Rule{Row: "its mark: only its sandbox reads it", Path: join(own, ReapMarkFolder, pl.ReapMark), Read: Allow, Opens: opens})
	}
	claude := join(own, "claude")
	b.add(Rule{Row: "its own Claude folder", Path: claude, Below: true, Read: Allow, Write: Allow, Opens: opens})
	b.beside(Rule{Row: "its own Claude folder's fence", Path: claude, Tail: "settings*.json", Read: Allow, Write: Deny, Opens: opens}, anyAfter)
	// The core's own copy of it, made beside it and renamed over it at a start when it has a second name (freshCopy,
	// desktop/sandboxstart.go): a desk's process still running from the last start could open it there. The core's
	// other names of this kind (`.<name>.<id>`, `.sandbox-*.sb`) are in its own folder, which the support folder's row
	// holds whole: none ends in .md.
	if goos == "darwin" {
		b.add(Rule{Row: "its own Claude folder's fence", Path: claude, Tail: ".settings*.json.fresh-*", Read: Allow, Write: Deny, Opens: opens})
	}
	for _, link := range []string{"CLAUDE.md", "commands", "agents", "skills", "plugins", "output-styles"} {
		b.add(Rule{Row: "the links the core put in its Claude folder", Path: join(claude, link), Below: true, Read: Allow, Write: Deny, Opens: opens})
	}
	// Linux's copy of the places no mount holds (L1GUARD1, kept.go): its guard compares them with it. It holds copies
	// of places the desk can read anyway (its place's .claude, its registration's files, the shared objects/info), and
	// a place is never home nor holds the support folder (ErrRootHasSupport), so reading it opens nothing new. Beside
	// it, the file the guard notes what it stopped the desk for in: whatever the desk writes there, the next start puts
	// back only a place still as it was found (keptSeal), and a note lost leaves the change as it is, listed in Setup.
	if goos == "linux" {
		b.add(Rule{Row: "its kept copy: its guard reads it", Path: join(own, keptName), Below: true, Read: Allow, Write: Deny, Opens: opens})
		b.add(Rule{Row: "its guard's notes of what stopped it", Path: join(own, keptStopName), Read: Allow, Write: Allow, Opens: opens})
		// Its way out (SA-L6, proxy.go): the core's proxy for this desk alone, connect only, as the desk-start socket.
		// The view brings it back, and the socket mask lists it, by this row; other desks' are under the cover.
		b.add(Rule{Row: "its way out: the core's proxy", Path: join(own, ProxySocketName), Connect: Allow, Opens: opens})
	}
	if pl.DeskStart != "" {
		at := key(goos, pl.DeskStart)
		for _, s := range supports {
			if under(at, key(goos, s)) && at != key(goos, join(support, "desk-start")) {
				return nil, ErrDeskStart
			}
		}
		b.add(Rule{Row: "the desk-start socket", Path: pl.DeskStart, Connect: Allow, Opens: opens})
	}
	// The GM hired into a folder that isn't a repo checkout or worktree (FXGMROOT, RT126SWEEP2 P1 B1): ~/Projects by
	// default, which holds the repos, their hires' worktrees and the owner's checkouts, none of them a repo the rows
	// above know. The GM starts there and reads it, and writes none of it: hires change the code. Its place is its
	// workspace in its own folder here (its Root), and the one place it writes there is its own plans folder (Claude
	// Code takes none outside the folder it starts in). The projects folder and every folder above it are held as a
	// desk's place is: not moved, removed, linked, nor their mode, flags or ACL changed.
	if pl.GMProjects != "" {
		b.add(Rule{Row: rowGMProjects, Path: pl.GMProjects, Below: true, Write: Deny, Wide: true})
		root := key(goos, pl.Root)
		for folder := parent(spell(goos, pl.GMProjects)); !under(root, key(goos, folder)); folder = parent(folder) {
			b.add(Rule{Row: rowGMProjects + ": the folders above it", Path: folder, Write: Deny}) // those above its Root are held already
		}
		if pl.GMPlans != "" {
			b.add(Rule{Row: rowGMProjects + ": its own plans", Path: pl.GMPlans, Below: true, Read: Allow, Write: Allow, Opens: []string{rowGMProjects}})
			// …with a hire's rule for a git folder below its place (FXSWEEP3P1, RT126SWEEP3 P1's F3): the plans folder is
			// in no repo of its own, so no `.git` at it or below it, a folder or a file, and no HEAD at it, which git
			// takes for a bare repo's. Final: the allow above doesn't open it. On a Mac, as a hire's. But not its backstop
			// (RT126SWEEP3FIXP1's N1): a hire's place is looked at for git folders at its next start (gitsSinceWords), the
			// plans folder isn't, so a folder holding a .git made in the GM's workspace and moved in, or a bare repo's
			// folders below it, stay until the owner removes them. docs/manager/README.md says not to run git there.
			if goos == "darwin" {
				b.add(Rule{Row: rowGitBelow + ": in the GM's own plans", Path: pl.GMPlans, Tail: "**/.git", Below: true, Write: Deny})
				b.add(Rule{Row: rowGitBelow + ": in the GM's own plans", Path: join(pl.GMPlans, "HEAD"), Write: Deny})
			}
		}
		b.add(Rule{Row: rowGMWorkspace, Path: pl.Root, Below: true, Read: Allow, Write: Allow, Opens: opens})
	}
	// Final denies in every support folder: no exception reaches them.
	for _, s := range supports {
		b.add(Rule{Row: "the owner's tap (serve.sock)", Path: join(s, "serve.sock"), Read: Deny, Write: Deny, Connect: Deny})
		b.add(Rule{Row: "the GM's memory", Path: join(s, "gm-memory"), Below: true, Read: Deny, Write: Deny, Connect: Deny})
		b.add(Rule{Row: "the sandbox's canary", Path: spell(goos, s), Tail: "sandbox-canary-*", Below: true, Read: Deny, Write: Deny, Connect: Deny})
	}
	b.keepRefFolders(pl.GitDir) // FXGITROWS2R1: the held ref folders git removes when empty (policy_gitrows2.go)
	b.denyAbove()
	if b.err != nil {
		return nil, b.err
	}
	p := &Policy{goos: goos, rules: b.rules, work: workPlaces(goos, pl)}
	if pl.GitDir != "" {
		p.git = spell(goos, pl.GitDir)
		p.ownerHEAD = pl.Checkout != "" && key(goos, pl.Checkout) != key(goos, pl.Root)
		p.desks = desksPlaces(goos, pl)
	}
	return p, nil
}

type builder struct {
	goos  string
	rules []Rule
	err   error
	// inFolder is the writers' names of their own each folder has a row for already (inFolder).
	inFolder map[[2]string]bool
	// work is the keys of the places a desk works in (Root, Checkout, the hires and GM folders, and a GM's projects,
	// denied whole to it), and home the home folder's: where ownName adds no name (FXLOCKSIBLINGSR2).
	work []string
	home string
}

// add puts r last. A place that isn't whole fails the whole Policy: it would otherwise read as anywhere. So, on a Mac,
// does a place with a code point the fold's Unicode doesn't assign (FXPOLICYFOLDR1): a later Unicode may make it a
// known letter's case pair (U+A7D2 became U+A7D3's in 17.0), and the place's key would miss that letter's spelling.
// The error names the place and the code point (RT126POLICYFOLDR2 F2). The first cause is the one New gives.
func (b *builder) add(r Rule) {
	if r.Path != "" {
		r.Path = spell(b.goos, r.Path)
		if r.Path == "" {
			b.fail(ErrNotWhole)
		}
	} else if r.Tail == "" {
		b.fail(ErrNotWhole)
	}
	if b.goos == "darwin" {
		for _, s := range []string{r.Path, r.Tail} {
			if c, ok := unknownIn(s); ok {
				b.fail(fmt.Errorf("%w: %s (U+%04X)", ErrUnknownCodePoint, path.Join(r.Path, r.Tail), c))
				break
			}
		}
	}
	r.key, r.tail = foldName(b.goos, r.Path), foldName(b.goos, r.Tail)
	r.Except, r.except = slices.Clone(r.Except), nil
	for i, e := range r.Except {
		if r.Except[i] = spell(b.goos, e); r.Except[i] == "" || r.Path == "" {
			b.fail(ErrNotWhole)
		}
		if c, ok := unknownIn(r.Except[i]); ok && b.goos == "darwin" {
			b.fail(fmt.Errorf("%w: %s (U+%04X)", ErrUnknownCodePoint, r.Except[i], c))
		}
		r.except = append(r.except, foldName(b.goos, r.Except[i]))
	}
	r.ExceptTails, r.exceptTails = slices.Clone(r.ExceptTails), nil
	for _, tail := range r.ExceptTails {
		if tail == "" || r.Path == "" {
			b.fail(ErrNotWhole)
		}
		if c, ok := unknownIn(tail); ok && b.goos == "darwin" {
			b.fail(fmt.Errorf("%w: %s (U+%04X)", ErrUnknownCodePoint, path.Join(r.Path, tail), c))
		}
		r.exceptTails = append(r.exceptTails, foldName(b.goos, tail))
	}
	b.rules = append(b.rules, r)
}

// What a denied file's writers write first, then rename over it (FXLOCKSIBLINGS), as beside takes them.
const (
	// gitLock is git's lock file, `<name>.lock`: every file git rewrites (a ref, HEAD, the index, a config, shallow, a
	// reflog it expires) is written there whole, then renamed onto its place.
	gitLock = ".lock"
	// gitNew is packed-refs' new copy, `packed-refs.new`, which git writes and renames onto packed-refs while it
	// holds packed-refs.lock (git 2.55).
	gitNew = ".new"
	// anyAfter is every name that starts with the file's: beside a sign-in or a fence, where no name is a desk's.
	anyAfter = "*"
)

// ownersNames are the names the owner's programs write beside a plant or a git config file they keep whole, then
// rename onto it: git's lock (`git config --global`, `--system`, `--file`), a `.new` copy, `<name>.tmp` (the core's
// writePrivate) and `<name>.tmp.<pid>.<time>` (Claude Code's, Node's), `<name>.<random>.tmp` (the core's own
// os.CreateTemp: writeClaudeJSON, writeSettings, accounthooks.go) and JetBrains' safe write, `<name>___jb_tmp___`. A
// neighbour's name stays the desk's (`<name>.bak`, `<name>-notes`), and so does an editor's backup (`<name>~`), the old
// copy, which nothing renames back. npm's write-file-atomic names its copy `<name>.<hash>`, which no Tail can tell from
// `<name>.bak` (a Tail has no `[0-9a-f]`): the report of FXLOCKSIBLINGS lists it.
var ownersNames = []string{gitLock, gitNew, ".tmp*", ".*.tmp", "___jb_tmp___"}

// ownersAround are the owner's programs' names that start before a plant's or a git config's own (FXLOCKSIBLINGSR1),
// each a name's start and its end: macOS's sed -i (`/usr/bin/sed` with an empty suffix, the usual way to edit a
// dotfile, in setup and uninstall scripts) writes `.!<pid>!<name>`, and mise's dotfiles (`mise bootstrap dotfiles
// add`, `apply`) write `.<name>.tmp.<pid>.<n>`, each then renamed onto the file (both seen on this Mac:
// RT126LOCKSIBLINGS' B1, FXLOCKSIBLINGSR1's report).
var ownersAround = [][2]string{{".!*!", ""}, {".", ".tmp.*"}}

// ownersInFolder are names of their own the owner's programs write in a plant's or a git config's folder, whatever
// its name, then rename onto it (FXLOCKSIBLINGSR1, seen on this Mac): install(1)'s `INS@` and six random characters
// (`install -m 644 zshrc ~/.zshrc`, with or without -S), and perl -i's `XX` and six (perl 5.34). Python's tempfile, a
// Rust tempfile and the like have a random name with no fixed part: the report lists them (R1).
var ownersInFolder = []string{"INS@??????", perlTemp}

// perlTemp is perl -i's name. perl makes it in the edited file's folder whatever the file, and when it can't, only
// warns, leaves the file as it was and exits 0: a desk's edit is lost with no error (RT126LOCKSIBLINGSR2's B2).
const perlTemp = "XX??????"

// ownersNotInHome are the names of ownersInFolder ownName leaves out of the home folder itself (SIERRA's ruling on
// RT126LOCKSIBLINGSR2's B2): a desk's perl -pi of its own file in ~ must work. perl -i's race on a plant there is
// listed with Python's and Rust's tempfile (R1). install(1) copies in place inside a desk, so its name costs nothing.
var ownersNotInHome = []string{perlTemp}

// omzNames are oh-my-zsh's names beside ~/.zshrc (RT126LOCKSIBLINGS' B1): its installer writes `.zshrc-omztemp` and
// moves it onto ~/.zshrc, and its uninstaller moves the old one, kept as `.zshrc.pre-oh-my-zsh`, back onto ~/.zshrc.
var omzNames = []string{"-omztemp", ".pre-oh-my-zsh"}

// ownersBeside adds r, a plant or a git config, and on a Mac r again at each name the owner's programs write and then
// rename onto it: ownersNames and more after its name, ownersAround around it, and ownersInFolder in its folder.
func (b *builder) ownersBeside(r Rule, more ...string) {
	b.beside(r, append(slices.Clone(ownersNames), more...)...)
	for _, a := range ownersAround {
		b.around(r, a[0], a[1])
	}
	for _, name := range ownersInFolder {
		b.ownName(r, name)
	}
}

// beside adds r, and on a Mac r again for each of after, its writers' names beside its place (above): the same row,
// so each is denied as the file is (FXLOCKSIBLINGS). Seatbelt checks a write when a file is opened, not when it is
// renamed onto: a desk that opens such a name while the owner's git or another program of the owner's has it there
// keeps a handle on it, and once the owner's program renames it into place the desk writes the denied file through
// that handle (`main` moved, the shared config changed: RT126REPLACEREFS), or reads a denied file's new copy as it is
// written. A name is put after a file's Path, or after its Tail; a pattern after a Path whose last name a Tail can't
// hold (`[`, `]` or `\`) is left out, its plain names kept. A writer whose temporary file has a name of its own (a
// random one in the same folder, or in another folder the desk can write) can't be denied by name: its folder must be
// held, and the report of FXLOCKSIBLINGS lists those. Linux's view and Windows' jail are made from other rows: the
// report lists them for the VM.
func (b *builder) beside(r Rule, after ...string) {
	b.add(r)
	if b.goos != "darwin" {
		return
	}
	for _, a := range after {
		b.around(r, "", a)
	}
}

// around adds, on a Mac, r again at before, r's own name, then after, beside its place: the name's end after its Path,
// or the whole name in place of its Tail's last one. A pattern around a Path whose last name a Tail can't hold (`[`,
// `]` or `\`) is left out; a plain name is kept.
func (b *builder) around(r Rule, before, after string) {
	if b.goos != "darwin" {
		return
	}
	s := r
	switch name := path.Base(r.Path); {
	case r.Tail != "":
		folders, last := "", r.Tail
		if i := strings.LastIndex(r.Tail, "/"); i >= 0 {
			folders, last = r.Tail[:i+1], r.Tail[i+1:]
		}
		s.Tail = folders + before + last + after
	case !strings.ContainsAny(before+after, "*?"):
		s.Path = path.Join(path.Dir(r.Path), before+name+after)
	case strings.ContainsAny(name, `[]\`):
		return
	default:
		s.Path, s.Tail = path.Dir(r.Path), before+name+after
	}
	b.add(s)
}

// ownName adds, on a Mac, r again at name in r's folder: a writer's name of its own, whatever the file's. The first
// row to hold it in a folder keeps it. It adds none at or under a place the desk works in (Root, Checkout, the hires
// and GM folders), where the name would be every file's, the desk's own work too (FXLOCKSIBLINGSR2: perl -pi of a
// worktree's package.json did nothing once the owner's git config named a file beside it); with a Tail, r's Path is
// the place it goes by. Nor one of ownersNotInHome in the home folder itself.
func (b *builder) ownName(r Rule, name string) {
	if b.goos != "darwin" {
		return
	}
	s := r
	if r.Tail != "" {
		s.Tail = path.Join(path.Dir(r.Tail), name)
	} else {
		s.Path, s.Tail = path.Dir(r.Path), name
	}
	folder := key(b.goos, s.Path)
	if b.leavesOut(folder, s.Tail) {
		return
	}
	k := [2]string{folder, foldName(b.goos, s.Tail)}
	if b.inFolder == nil {
		b.inFolder = map[[2]string]bool{}
	}
	if !b.inFolder[k] {
		b.inFolder[k] = true
		b.add(s)
	}
}

// leavesOut is whether a row of a writer's own name in a folder (ownersInFolder), its Tail tail, is left out of folder,
// a key: at or under a place the desk works in, or one of ownersNotInHome directly in the home folder. ownName asks it
// of the folder as spelled, and followLinks of the folder it leads to (RT126LOCKSIBLINGSR3's F1: a git config value
// spelled through `~/Projects`, a link to the folder that holds the desk's root, put perl -i's name back in the root).
func (b *builder) leavesOut(folder, tail string) bool {
	for _, w := range b.work {
		if under(folder, w) {
			return true
		}
	}
	return folder == b.home && slices.Contains(ownersNotInHome, tail)
}

// fail keeps err as the Policy's error, unless an earlier one is kept already.
func (b *builder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

// denyAbove holds every denied place's parents (RT126SAX1 B1): for each row that denies anything, the folders its
// Tail goes through, its own folder when it has a Tail, and every folder above that, up to the top, can't be removed,
// renamed away or made a link. A rename is checked only against its two paths, never what is below them, so without
// these a desk could move a parent aside (`.git/refs`, `~/.ssh`, `~/.codex`), write under its new name and move it
// back. Files and folders are still made inside them: these rows say Move and Link only. A row for anywhere (no Path)
// needs none: its pattern matches wherever the folder is moved to. One missing at the start is made then (Hold).
func (b *builder) denyAbove() {
	seen := map[[2]string]bool{}
	hold := func(folder, tail string) {
		r := Rule{Row: rowAbove, Path: folder, Tail: tail, Move: Deny, Link: Deny}
		if k := [2]string{key(b.goos, folder), foldName(b.goos, tail)}; !seen[k] {
			seen[k] = true
			b.add(r)
		}
	}
	for _, r := range slices.Clone(b.rules) {
		if r.Path == "" || !r.denies() {
			continue
		}
		var names []string
		if r.Tail != "" {
			names = strings.Split(r.Tail, "/")
		}
		for i := len(names) - 1; i > 0; i-- {
			if slices.Contains(names, "**") && strings.ContainsAny(strings.Join(names[:i], "/"), `*?[\`) {
				// A pattern that reaches any depth (a git folder below its place, FXHIREPROJECTSR6): held by pattern, its
				// folders would be every folder below its Path, and nothing could be renamed there. Its rows hold the
				// folders on the way by name instead.
				continue
			}
			if through := strings.Join(names[:i], "/"); strings.ContainsAny(through, `*?[\`) {
				hold(r.Path, through) // a pattern's folders: on a Mac, a folder-only filter
			} else {
				hold(path.Join(r.Path, through), "")
			}
		}
		hold(r.Path, "")
		for folder := r.Path; parent(folder) != folder; {
			folder = parent(folder)
			hold(folder, "")
		}
	}
}

// Hold is the folders a desk could bring in whole while they don't exist (RT126SAX1R2 B1′): each one the Policy holds
// above a denied place whose own path it lets the desk write, so a folder made elsewhere, a denied file inside, could
// be renamed there, or a link made there (`~/.ssh`, `~/.config`, `~/.config/systemd`, `~/.codex`, `~/.gemini`). The
// core makes each missing one, empty and owner-only, before the desk starts (MakeHold), and the row's Move and Link
// denies then hold it like any other. A place the Policy denies writing needs none: a rename onto it, or a link at it,
// is a write there. That is how `.git/modules` and every folder in it are held (RT126SAX1R4 B1‴), and each worktree's
// registration in `.git/worktrees`.
func (p *Policy) Hold() []string {
	var folders []string
	for _, r := range p.rules {
		if r.Row != rowAbove || r.Tail != "" {
			continue
		}
		if v, _ := p.Decide(r.Path, Write); v != Deny {
			folders = append(folders, r.Path)
		}
	}
	slices.Sort(folders)
	return folders
}

// MakeHold makes each of folders (a Policy's Hold) that doesn't exist, empty and owner-only, for the core before a
// desk starts. A path that exists, as anything, is left as it is, and so is one under a file (ENOTDIR): nothing can be
// made there, and the file is itself a folder above a denied place, which no desk removes or replaces (a reftable
// repo's .git/refs/heads, under which FXGITROWS2 holds refs/heads/hire). The first folder it can't make fails the start.
// It is load-bearing (RT126SAX1R3 follow-up 3): without it a rename onto a missing `~/.gemini`, held by its Link row
// alone, brings `oauth_creds.json` in. The desk's start (M1) must call it before the desk's first process, and test it.
func MakeHold(folders []string) error {
	for _, folder := range folders {
		if _, err := os.Lstat(folder); err == nil || errors.Is(err, syscall.ENOTDIR) {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(folder, 0o700); err != nil && !errors.Is(err, syscall.ENOTDIR) {
			return err
		}
	}
	return nil
}

// makeHeld is MakeHold of one folder, for a desk's start on a Mac (FXAGENTHOMEMACR1, RT126AGENTHOMEMAC B1): one the
// owner can't make either doesn't fail the start. That is one on a volume that isn't there, under a folder of root's,
// one the owner can't write in or a read-only one, or under something that isn't a folder (EACCES, EPERM, EROFS,
// ENOENT, ENOTDIR). unmade is then the first folder on its way that isn't there, which the desk's profile denies
// writing at and below (unmadeHeld): the desk can't make it, link it or rename one there either, should it become
// makeable later (a volume mounted, a folder opened). Any other failure fails the start, as MakeHold's does.
func makeHeld(folder string) (unmade string, err error) {
	err = MakeHold([]string{folder})
	if err != nil && !ownerCantMake(err) {
		return "", err
	}
	if m := firstMissing(folder); m != "" {
		return m, nil
	}
	return "", err
}

// ownerCantMake is whether err, a folder's making's, says the owner can't make it.
func ownerCantMake(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EROFS) || errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, syscall.ENOTDIR)
}

// rowUnmade is the row of a held folder the core couldn't make before the desk's start (makeHeld).
const rowUnmade = "a held folder no one could make at the start"

// unmadeHeld is p with each of folders, the first missing folder of a held one the core couldn't make (makeHeld),
// denied writing at and below: a copy, so a Policy kept for the desk's stop stays as New made it.
func (p *Policy) unmadeHeld(folders []string) (*Policy, error) {
	if len(folders) == 0 {
		return p, nil
	}
	b := &builder{goos: p.goos}
	for _, folder := range folders {
		b.add(Rule{Row: rowUnmade, Path: folder, Below: true, Write: Deny})
	}
	if b.err != nil {
		return nil, b.err
	}
	q := *p
	q.rules = append(slices.Clone(p.rules), b.rules...)
	return &q, nil
}

// parent is a spelled folder's parent: the folder itself at the top.
func parent(folder string) string {
	if i := strings.LastIndex(folder, "/"); i > 0 && !(len(folder) > 2 && folder[1] == ':' && i == 2) {
		return folder[:i]
	} else if i >= 0 {
		return folder[:i+1]
	}
	return folder
}

// plainName is whether name is one folder's name: not empty, no separator, not . or .., and nothing a pattern reads.
func plainName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\*?[]`)
}

// ReapMarkFolder is the folder in a desk's own folder that holds its mark (Places.ReapMark), and nothing else. Spotlight
// doesn't index a folder whose name ends in .noindex.
const ReapMarkFolder = "reap.noindex"

// NewReapMark is a new desk's mark's name: "reap-" and 32 hex digits, at random. No two desks' are the same, and no
// app of the owner's is ever handed a file of that name.
func NewReapMark() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "reap-" + hex.EncodeToString(b[:]), nil
}

// ReapMarkValid is whether name is a mark's name as NewReapMark makes them.
func ReapMarkValid(name string) bool {
	digits, ok := strings.CutPrefix(name, "reap-")
	if !ok || len(digits) != 32 {
		return false
	}
	for _, c := range digits {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// branchName is whether name is a branch's name git would make, with nothing a path or a pattern reads.
func branchName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if !plainName(part) || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// CredentialFiles are the sign-ins of the other agents Smolcorp starts, in home: denied whole to every desk, and the
// list SB15's sign-in runner (S4) takes.
func CredentialFiles(home string) []string {
	var files []string
	for _, names := range append(slices.Clone(credentialNames), macCredentialNames...) {
		files = append(files, filepath.Join(append([]string{home}, names...)...))
	}
	return files
}

// credentialNames are where in the home folder each of CredentialFiles is, on every system; macCredentialNames the
// rest of each agent's sign-ins (FXPLANTSTERMR3), on a Mac, whose Policy holds the agents' folders: Linux's view and
// Windows' jail take them with their sandboxes, as they take the folders.
var (
	credentialNames    = [][]string{{".codex", "auth.json"}, {".gemini", "oauth_creds.json"}}
	macCredentialNames = func() [][]string {
		var names [][]string
		for _, in := range []struct {
			folder  []string
			signIns []string
		}{
			{[]string{".codex"}, codexSignIns}, {[]string{".gemini"}, geminiSignIns}, {[]string{".cache", ".gemini"}, geminiSignIns},
			{[]string{".grok"}, grokSignIns}, {[]string{".local", "share", "opencode"}, openCodeSignIns},
		} {
			for _, name := range in.signIns {
				if folder := append(slices.Clone(in.folder), name); !slices.ContainsFunc(credentialNames, func(n []string) bool {
					return slices.Equal(n, folder)
				}) {
					names = append(names, folder)
				}
			}
		}
		return names
	}()
)

// DefaultBranch is the repo's default branch, as its origin/HEAD names it in gitDir (the common .git), or "".
// Read as a file: no git runs. Only a small plain file is read, opened without waiting (openPlain): a FIFO whoever can
// write the .git puts there is refused at once, never waited on (RT126SWEEP4 part 3's B1).
func DefaultBranch(gitDir string) string {
	f, err := openPlain(filepath.Join(gitDir, "refs", "remotes", "origin", "HEAD"))
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return ""
	}
	name, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref: refs/remotes/origin/")
	if !ok || !branchName(name) {
		return ""
	}
	return name
}

// supportFiles is every name Smolcorp keeps at the top of its support folder, and what the Policy does with it for a
// desk. The folder is denied whole, so a name missing here is still denied; the list is so that each new file is
// thought about once. SK1's class map (desktop/records_test.go) fails for a file the core makes with no row here.
var supportFiles = func() map[string]string {
	rows := map[string]string{}
	for why, names := range map[string][]string{
		"denied: a sealed record":                                                   {"phones.json", "mac-peers.json", "registry.json", "config.json", "claude-held.json"},
		"read only: the guard reads it, nothing in a desk writes":                   {"hire-guard.json"},
		"read only: the night shift's jobs, which only the core writes (NS1)":       {"nightshift.json"},
		"denied: the records' and the core's own state":                             {"hire-guard.seal", "keys-changed-1.26", "nightshift.json.bad", "instance.lock", "smolcorp.pid", "claude-install", "voice-set-by-smolcorp", "WebView2", "paste-hold", "paste-hold-seen", "default-branches.json"},
		"denied: what desks did, said and were sent":                                {"sessions.json", "events.jsonl", "events.1.jsonl", "grants.jsonl", "saved.json", "hire-queue.json", "outbox.json", "acp-transcripts", "recaps", "briefs", "checkpoints", "photos"},
		"denied, but the desk's own brief, status, report, fence and Claude folder": {"hires"},
		"denied: the hook's key, the owner's sessions' (§5.10)":                     {"hook-key", "hook-header", "report.sh"},
		"denied: a key, or an old key's file removed at each start":                 {"setup-key", "window-key", "selftest-key", "hold-key", "token", "phones-seal", "gm-token", "gm-token-kept", "push-key-kept", "gm-session"},
		"connect only: the desk-start socket (SG2)":                                 {"desk-start"},
		"denied, with no exception: the owner's tap (SN1)":                          {"serve.sock"},
		"denied, with no exception: the GM's memory (PL126M)":                       {"gm-memory"},
	} {
		for _, name := range names {
			rows[name] = why
		}
	}
	return rows
}()

// SupportRow is what the Policy does with name, a file or folder at the top of the support folder, for a desk; false
// when it has no row for it.
func SupportRow(name string) (string, bool) {
	if strings.HasPrefix(name, "sandbox-canary-") {
		return "denied, with no exception: the sandbox's canary (§A5.2)", true
	}
	if strings.HasPrefix(name, "signin-") {
		return "denied, with no exception: a Claude folder Setup asks hires' sign-in in, taken away after", true
	}
	if strings.HasPrefix(name, "summary-") {
		return "denied, with no exception: the folder a summary's agent runs in, taken away after (RT126SWEEP4 part 3's F1)", true
	}
	why, ok := supportFiles[name]
	return why, ok
}

// supportSiblings are the support folders by their other name (appdirs), each beside one of supports whose own name is
// one of the two on goos: Application Support/Smolcorp beside Application Support/AgentDeck on a Mac, ~/.config/smolcorp
// beside ~/.config/sauzzedeck-pro on Linux, %APPDATA%\Smolcorp beside %APPDATA%\SauzzeDeck Pro on Windows, and the old
// name beside a Smolcorp one. Only the whole path counts, never the name alone: ~/Projects/smolcorp is no support
// folder. One that is a support folder already, or not whole, is left out.
func supportSiblings(goos string, supports []string) []string {
	old, new := foldName(goos, appdirs.Name(goos)), foldName(goos, appdirs.NewName(goos))
	have := map[string]bool{}
	for _, s := range supports {
		have[key(goos, s)] = true
	}
	var out []string
	for _, s := range supports {
		spelled := spell(goos, s)
		if spelled == "" {
			continue
		}
		var other string
		switch foldName(goos, path.Base(spelled)) {
		case old:
			other = appdirs.NewName(goos)
		case new:
			other = appdirs.Name(goos)
		default:
			continue
		}
		sibling := path.Join(path.Dir(spelled), other)
		if k := key(goos, sibling); k != "" && !have[k] {
			have[k] = true
			out = append(out, sibling)
		}
	}
	return out
}
