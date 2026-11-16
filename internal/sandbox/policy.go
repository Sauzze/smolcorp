package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
	// Move is removing the folder at the path, or renaming it away: a row's Write says it too, and the rows on the
	// folders above a denied place say only this and Link, so files and folders are still made inside them. On a Mac
	// it is `file-write-unlink` on a folder (policy_darwin_test.go checks `mv`, perl's rename and python's os.rename).
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

	key, tail string // folded
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
	// DefaultBranch is the repo's default branch (DefaultBranch(GitDir)), "" when it can't be told.
	DefaultBranch string
	// HiresFolder is where the hires' worktrees go (<repo>-hires, or the configured folder); GMFolder the GM's.
	HiresFolder, GMFolder string
	// SocketParent is /tmp/smolcorp-<uid>; OwnSockets the desk's own folder in it.
	SocketParent, OwnSockets string
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
	// for git to read as config or run (include and includeIf targets, core.fsmonitor's and core.sshCommand's files),
	// as ReadGitConfig gives them at the desk's start (GITCFG1). M1 must fill both.
	GitHooks, GitNamed []string
	// RemoteLogin is whether port 22 answers on this computer (C5): the ssh keys are then denied.
	RemoteLogin bool
	// XAuthority is $XAUTHORITY on Linux.
	XAuthority string
}

// Policy is the rows for one desk, in order.
type Policy struct {
	goos  string
	rules []Rule
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
// several rows, the narrowest is named. A path that isn't whole is denied.
//
// path must be resolved: Decide reads it as written, folding only the spellings policy_fold.go names. A link, a
// mount, another volume's name for the same disk (/Volumes/Macintosh HD/…), a Windows 8.3 name, a trailing dot or
// space, a `::$DATA` stream, and two combining marks after one letter in an order NFD wouldn't write (RT126SAX1R2 N2:
// e, a dot below, a circumflex compose to `ệ`; e, a circumflex, a dot below are left so) are the caller's to resolve
// first; the kernel does it for a profile.
func (p *Policy) Decide(path string, op Op) (Verdict, Rule) {
	k := key(p.goos, path)
	if k == "" {
		return Deny, Rule{Row: "not a whole path"}
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
)

// The rows exceptions open, by name.
const (
	rowSSH        = "ssh keys while Remote Login answers"
	rowOtherDesks = "other desks' places"
	rowCheckout   = "the owner's checkout"
	rowSockets    = "the socket folder"
	// rowOwnSockets is the desk's own socket folder: its own socket must be in it (sbpl.go's serviceRules).
	rowOwnSockets = "its own socket folder"
	rowSupport    = "the support folder, whole"
	rowAbove      = "the folders above a denied place can't be moved or removed"
)

// rowOldRemotes is the row of git's old remote files, which X12's start-time listing names as well.
const rowOldRemotes = "the shared git places: the old remote files"

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
	b := &builder{goos: goos}
	supports := append([]string{pl.Support}, pl.OtherSupports...)
	for _, place := range append([]string{pl.Home, pl.Root}, supports...) {
		if key(goos, place) == "" {
			return nil, ErrNotWhole
		}
	}
	for _, root := range []string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GMFolder} {
		if root == "" {
			continue
		}
		rk := key(goos, root)
		if rk == "" {
			return nil, ErrNotWhole
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
	if !plainName(pl.OwnID) || pl.OwnWorktree != "" && !plainName(pl.OwnWorktree) || pl.DefaultBranch != "" && !branchName(pl.DefaultBranch) {
		return nil, ErrBadName
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

	// The owner's own places in the home folder.
	if pl.RemoteLogin {
		b.add(Rule{Row: rowSSH, Path: join(home, ".ssh"), Tail: "*", Below: true, Read: Deny, Write: Deny, Wide: true})
		for _, tail := range []string{"*.pub", "known_hosts*", "config"} {
			b.add(Rule{Row: "ssh files that aren't keys", Path: join(home, ".ssh"), Tail: tail, Read: Allow, Write: Allow, Opens: []string{rowSSH}})
		}
	}
	plants := []string{".zshrc", ".zshenv", ".zprofile", ".zlogin", ".zlogout", ".bashrc", ".bash_profile", ".bash_login",
		".bash_logout", ".profile", ".gitconfig", ".tmux.conf", ".ssh/config", ".ssh/rc"}
	for _, name := range plants {
		b.add(Rule{Row: "plants the owner's programs run later", Path: join(home, name), Write: Deny})
	}
	for _, folder := range []string{".config/fish", "Library/LaunchAgents", ".config/autostart", ".config/systemd/user", ".config/git", ".config/tmux"} {
		b.add(Rule{Row: "plants the owner's programs run later", Path: join(home, folder), Below: true, Write: Deny})
	}
	b.add(Rule{Row: "plants the owner's programs run later", Path: join(home, ".ssh"), Tail: "authorized_keys*", Write: Deny})
	for _, file := range pl.GitConfigs {
		b.add(Rule{Row: "plants the owner's programs run later: git's other config", Path: file, Write: Deny})
	}
	// What the owner's git config names for git to run or read as config (GITCFG1). A hooks folder is denied whole: a
	// hook may run what is beside it (husky's hooks source its `h`). ReadGitConfig gives the folders beside which a hook
	// runs more (husky's `.husky` above its `_`) and those a link in one leads to as hooks folders too (RT126GITCFG1).
	for _, folder := range pl.GitHooks {
		b.add(Rule{Row: "plants the owner's programs run later: git's hooks folder", Path: folder, Below: true, Write: Deny})
	}
	for _, file := range pl.GitNamed {
		b.add(Rule{Row: "plants the owner's programs run later: what git's config names", Path: file, Write: Deny})
	}
	if goos == "darwin" {
		for _, folder := range []string{"/usr/lib/cron/tabs", "/private/var/at"} {
			b.add(Rule{Row: "plants the owner's programs run later", Path: folder, Below: true, Write: Deny})
		}
	}
	for _, account := range append([]string{join(home, ".claude")}, pl.Accounts...) {
		b.add(Rule{Row: "the account folders", Path: account, Below: true, Read: Allow, Write: Deny})
		for _, secret := range []string{"ide", ".credentials.json", "history.jsonl", "projects", "sessions"} {
			b.add(Rule{Row: "the account folders' tokens and history", Path: join(account, secret), Below: true, Read: Deny, Write: Deny})
		}
		b.add(Rule{Row: "the accounts' .claude.json", Path: account, Tail: ".claude.json*", Read: Deny, Write: Deny})
	}
	b.add(Rule{Row: "the accounts' .claude.json", Path: join(home), Tail: ".claude.json*", Read: Deny, Write: Deny})
	for _, file := range CredentialFiles(home) {
		b.add(Rule{Row: "other agents' sign-ins", Path: file, Read: Deny, Write: Deny})
	}
	if goos == "linux" {
		for _, file := range []string{join(home, ".Xauthority"), pl.XAuthority} {
			if file != "" {
				b.add(Rule{Row: "X11's key", Path: file, Read: Deny, Write: Deny})
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
	if pl.Checkout != "" && key(goos, pl.Checkout) != key(goos, pl.Root) {
		b.add(Rule{Row: rowCheckout, Path: pl.Checkout, Below: true, Write: Deny, Wide: true})
	}
	b.add(Rule{Row: "its own place", Path: pl.Root, Below: true, Read: Allow, Write: Allow, Opens: []string{rowOtherDesks, rowCheckout}})
	if pl.GitDir != "" {
		git := pl.GitDir
		b.add(Rule{Row: "the rest of the shared .git", Path: git, Below: true, Write: Allow, Opens: []string{rowCheckout}})
		// The shared git places: the repo's config, hooks and objects' list of other stores, and every file git reads to
		// find where those are (gitrepository-layout; SIERRA, 14 Nov): `commondir` names the folder git takes them all
		// from, for the owner's checkout as for a worktree, and `gitdir` is denied with it. `info` holds `alternates`
		// too, should a git read it there; `objects/info` holds `alternates` and `http-alternates`.
		for _, name := range []string{"config", "config.worktree", "index", "HEAD", "commondir", "gitdir"} {
			b.add(Rule{Row: "the shared git places", Path: join(git, name), Write: Deny})
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
		// The submodules' git folders, whole, for every desk (RT126SAX1R4 B1‴): any folder in there, brought in whole by
		// a rename with its config and hooks inside, the owner's git could later take for a submodule's, once the desk
		// had broken the git folder above it with one write (its HEAD). A desk in its own worktree loses nothing by it:
		// git keeps that worktree's submodules in its own registration, worktrees/<id>/modules, which stays its own. A
		// desk at the project loses its submodule work (SANDBOX-PLAN §A4.1, the shared git places): it can't init,
		// update, fetch, check out or commit in a submodule, which all write here; it still reads them, and works in
		// the superproject, a submodule's new commit included once the owner has fetched it.
		b.add(Rule{Row: "the shared git places: the submodules' git folders", Path: join(git, "modules"), Below: true, Write: Deny})
		// A worktree's registration is made, moved and removed by the core, never by a desk; in each, the files that
		// say where its shared .git is (`commondir`) and where it is (`gitdir`), and its own config (read when the
		// owner's config turns worktreeConfig on). The desk's own registration included: the core and the owner run git
		// in its worktree too. Its HEAD, index, logs and modules stay its own. The config of its submodules, in
		// worktrees/<id>/modules, is read by any git that recurses into them there, the core's own `git status` at a
		// fire included (RT126SAX1R5): no row can deny it without taking the desk's submodule work, so M1 must give
		// every git the core runs in a desk's place gitReadArgs' settings (desktop/gitstate.go) and
		// --ignore-submodules=all.
		worktrees := join(git, "worktrees")
		b.add(Rule{Row: "the shared git places: a worktree's registration", Path: worktrees, Tail: "*", Write: Deny})
		for _, name := range []string{"commondir", "gitdir", "config.worktree"} {
			b.add(Rule{Row: "the shared git places: where a worktree finds them", Path: worktrees, Tail: "*/" + name, Write: Deny})
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
		b.add(Rule{Row: "the base branches", Path: join(git, "packed-refs"), Write: Deny})
		for _, branch := range []string{"main", "master", pl.DefaultBranch} {
			if branch != "" {
				b.add(Rule{Row: "the base branches", Path: join(git, "refs/heads", branch), Write: Deny})
			}
		}
		b.add(Rule{Row: "the base branches", Path: join(git, "refs/heads/release"), Below: true, Write: Deny})
		// In a reftable repo (`git init --ref-format=reftable`) every ref of the shared .git is in reftable/: no branch
		// can be told from another there, so no desk moves any, its own branch included (GITCFG1). A repo of files has
		// no such folder, and git reads one only when the repo's config, which no desk writes, says so.
		b.add(Rule{Row: "the base branches: a reftable repo's refs", Path: join(git, "reftable"), Below: true, Write: Deny})
		// The old remote files (RT126GITCFG2's note 3; SIERRA, 14 Nov): `branches/<name>` and `remotes/<name>` give a
		// remote config doesn't name an address, which the owner's fetch, pull and push use, and an address can name a
		// program (`ext::`, a `<helper>::`). They aren't config, so gitstate never sees them. Git reads them from the
		// shared .git only; each worktree's registration is denied as well (SIERRA), and the submodules' are in modules/,
		// denied whole. Git hasn't written them for years: a desk loses nothing.
		for _, folder := range []string{"branches", "remotes"} {
			b.add(Rule{Row: rowOldRemotes, Path: join(git, folder), Below: true, Write: Deny})
			b.add(Rule{Row: rowOldRemotes, Path: worktrees, Tail: "*/" + folder, Below: true, Write: Deny})
		}
	}

	// Anywhere: the fences, and the folders a desk's place hangs from.
	b.add(Rule{Row: "a .claude folder can't be made, moved or removed", Tail: "**/.claude", Write: Deny})
	b.add(Rule{Row: "fences and hooks", Tail: "**/.claude/settings.json", Write: Deny})
	b.add(Rule{Row: "fences and hooks", Tail: "**/.claude/settings.local.json", Write: Deny})
	for _, place := range append([]string{pl.Root, pl.Checkout}, supports...) {
		for folder := spell(goos, place); place != ""; folder = parent(folder) {
			b.add(Rule{Row: "a desk's place and the folders above it can't be moved or removed", Path: folder, Write: Deny})
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

	// The support folder, whole, with the desk's own exceptions in its core's. Another support folder has none.
	b.add(Rule{Row: rowSupport, Path: pl.Support, Below: true, Read: Deny, Write: Deny, Connect: Deny, Wide: true})
	for _, s := range pl.OtherSupports {
		b.add(Rule{Row: "another support folder, whole", Path: s, Below: true, Read: Deny, Write: Deny, Connect: Deny})
	}
	support, opens := pl.Support, []string{rowSupport}
	b.add(Rule{Row: "the guard file: the guard reads it", Path: join(support, "hire-guard.json"), Read: Allow, Opens: opens})
	// The night shift's jobs (NS1), kept as the guard file is: a hire or the GM may read them (they hold no secret, and
	// the guard lets its readers name the file), only the core writes them, and an approval a desk wrote would start
	// hires overnight. The copy the core sets aside unread (nightshift.json.bad) has no exception.
	b.add(Rule{Row: "the night shift's jobs: read only, as the guard file", Path: join(support, "nightshift.json"), Read: Allow, Opens: opens})
	own := join(support, "hires", pl.OwnID)
	b.add(Rule{Row: "its status and report", Path: own, Tail: "*.md", Read: Allow, Write: Allow, Opens: opens})
	for _, tail := range []string{"BRIEF*.md", "settings.json", "sandbox.sb"} {
		b.add(Rule{Row: "its brief and its fence", Path: own, Tail: tail, Read: Allow, Write: Deny, Opens: opens})
	}
	claude := join(own, "claude")
	b.add(Rule{Row: "its own Claude folder", Path: claude, Below: true, Read: Allow, Write: Allow, Opens: opens})
	b.add(Rule{Row: "its own Claude folder's fence", Path: claude, Tail: "settings*.json", Read: Allow, Write: Deny, Opens: opens})
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
	// Final denies in every support folder: no exception reaches them.
	for _, s := range supports {
		b.add(Rule{Row: "the owner's tap (serve.sock)", Path: join(s, "serve.sock"), Read: Deny, Write: Deny, Connect: Deny})
		b.add(Rule{Row: "the GM's memory", Path: join(s, "gm-memory"), Below: true, Read: Deny, Write: Deny, Connect: Deny})
		b.add(Rule{Row: "the sandbox's canary", Path: spell(goos, s), Tail: "sandbox-canary-*", Below: true, Read: Deny, Write: Deny, Connect: Deny})
	}
	b.denyAbove()
	if b.err != nil {
		return nil, b.err
	}
	return &Policy{goos: goos, rules: b.rules}, nil
}

type builder struct {
	goos  string
	rules []Rule
	err   error
}

// add puts r last. A place that isn't whole fails the whole Policy: it would otherwise read as anywhere.
func (b *builder) add(r Rule) {
	if r.Path != "" {
		r.Path = spell(b.goos, r.Path)
		if r.Path == "" {
			b.err = ErrNotWhole
		}
	} else if r.Tail == "" {
		b.err = ErrNotWhole
	}
	r.key, r.tail = foldName(b.goos, r.Path), foldName(b.goos, r.Tail)
	b.rules = append(b.rules, r)
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
// desk starts. A path that exists, as anything, is left as it is. The first folder it can't make fails the start.
// It is load-bearing (RT126SAX1R3 follow-up 3): without it a rename onto a missing `~/.gemini`, held by its Link row
// alone, brings `oauth_creds.json` in. The desk's start (M1) must call it before the desk's first process, and test it.
func MakeHold(folders []string) error {
	for _, folder := range folders {
		if _, err := os.Lstat(folder); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return err
		}
	}
	return nil
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
	return []string{filepath.Join(home, ".codex", "auth.json"), filepath.Join(home, ".gemini", "oauth_creds.json")}
}

// DefaultBranch is the repo's default branch, as its origin/HEAD names it in gitDir (the common .git), or "".
// Read as a file: no git runs.
func DefaultBranch(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "refs", "remotes", "origin", "HEAD"))
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
		"denied: the records' and the core's own state":                             {"hire-guard.seal", "keys-changed-1.26", "nightshift.json.bad", "instance.lock", "smolcorp.pid", "claude-install", "voice-set-by-smolcorp", "WebView2"},
		"denied: what desks did, said and were sent":                                {"sessions.json", "events.jsonl", "events.1.jsonl", "grants.jsonl", "saved.json", "hire-queue.json", "outbox.json", "acp-transcripts", "recaps", "briefs", "checkpoints", "photos"},
		"denied, but the desk's own brief, status, report, fence and Claude folder": {"hires"},
		"denied: the hook's key, the owner's sessions' (§5.10)":                     {"hook-key", "hook-header", "report.sh"},
		"denied: a key, or an old key's file removed at each start":                 {"setup-key", "window-key", "selftest-key", "token", "phones-seal", "gm-token", "gm-token-kept", "push-key-kept", "gm-session"},
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
	why, ok := supportFiles[name]
	return why, ok
}
