package sandbox

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// FXGITROWS2 (RT126SWEEP2 part 1: B2, B3 and F1), on a Mac, for every desk in a repo: what else in the shared .git
// changes what the owner's git, or the core, later takes from it. Linux's view isn't changed here (as FXREPLACEREFS):
// its report lists these for the VM.
const (
	// rowDefaultName is origin/HEAD, which names the repo's default branch (B2). The core reads it for the next
	// start's base branches and for the fire's look; a desk that pointed it at another branch took the default branch
	// out of both. The core now pins the default branch it first read (desktop/deskgitrows.go): this row is the
	// other side, so no desk changes what a pin, or the owner's own `git remote set-head`, reads; the core also holds
	// the branch origin/HEAD names at each start, the owner's on a Mac (FXGITROWS2R1, N1: Places.DefaultBranchNow).
	rowDefaultName = "the default branch: origin/HEAD, which names it"
	// rowRerere is rerere's records (B3): once `rr-cache` is there, git turns rerere on by itself, and the owner's
	// merge takes a resolution a desk recorded, a line in neither side and not in the hire's diff. MERGE_RR is the
	// list of conflicts a merge in progress has, which the next `git commit` or `git rerere` in that worktree records
	// into rr-cache as the resolution: the shared .git's (the owner's checkout's) and each worktree's, the desk's own
	// included, since the owner and the core run git in its worktree too. A desk loses only rerere.
	rowRerere = "the owner's merges: rerere's recorded resolutions"
	// rowOtherBranches is every desk's branches but its own, and their reflogs (F1): `hire/<id>/…` is a desk's,
	// `hire/<name>` one's from before branches had a folder of their own.
	rowOtherBranches = "other desks' branches and their reflogs"
	// rowCheckoutBranch is the branch the owner's checkout is on at the desk's start, and its reflog (F1).
	rowCheckoutBranch = "the owner's checkout: the branch it is on, and its reflog"
	// rowRefFolders is a held folder in refs/ or logs/refs/ that git removes once it is empty (FXGITROWS2R1, from
	// RT126FIXREVIEW-GITROWS2's B1): refs/heads/hire, logs/refs/heads/hire, refs/remotes/origin, and the folders of a
	// held branch whose name has a slash (the owner checkout's topic/x, a default branch team/main). A desk can't
	// make it, by mkdir, a rename or a link, and a keeper (refKeeper) keeps it there: see keepRefFolders.
	rowRefFolders = "the ref folders git removes when empty, above a held ref: kept, never made by a desk"
	// rowRefLinks is every link a desk would make in refs/ or logs/ (RT126FIXREVIEW-GITROWS2R1's B1): see gitRows2.
	rowRefLinks = "no links in refs/ or logs/: git makes none there"
	// rowNotOwnBranches is F1's own rule as an allowlist (FXGITROWS2R4): in refs/heads and logs/refs/heads a desk writes
	// only its own branches, `hire/<OwnID>` and Places.OwnBranches, with their locks. rowOtherBranches and
	// rowCheckoutBranch name the branches they hold; this holds every other, the owner's branches no row named, one
	// the owner makes later, and every new name: see gitRows2.
	rowNotOwnBranches = "every branch but the desk's own, and their reflogs: no desk makes, moves or removes one"
	// rowTags is every tag and its reflog (FXGITROWS2R4): no desk makes, moves or removes one.
	rowTags = "tags and their reflogs: no desk makes, moves or removes one"
	// rowNotOwnRefs is the allowlist over the whole of refs/ and logs/refs/ (FXGITROWS2R5, RT126FIXREVIEW-GITROWS2R4's
	// N1): beside its own branches a desk writes only the remote-tracking ones (origin/HEAD apart). rowNotOwnBranches,
	// rowTags and the rows that name a ref hold theirs; this holds the rest: refs/<name> and refs/hire/…, which the
	// owner's git reads before refs/heads/<name>, refs/stash, refs/notes, refs/replace and any new folder.
	rowNotOwnRefs = "every other ref and its reflog (stash, notes, a name read before a branch's): no desk makes, moves or removes one"
	// rowOtherWorktrees is every worktree's registration but the desk's own (FXGITROWS2R5, RT126FIXREVIEW-GITROWS2R4's
	// N2): its HEAD, index and logs, for one registered after the desk's start too.
	rowOtherWorktrees = "every other worktree's registration: no desk writes its HEAD, index or logs"
	// rowGitTop is the allowlist over the rest of the shared .git (FXGITROWS2R6, RT126FIXREVIEW-GITROWS2R5's N2): the
	// owner's git reads `$G/<name>` before refs/<name> and refs/heads/<name>, so a name there is what the owner's `git
	// log feature` takes. A desk writes only the folders other rows hold and what its own work needs: see gitRows2.
	rowGitTop = "the rest of the shared .git: no name a desk's own work doesn't write there (one the owner's git reads before a branch)"
)

// perWorktreeRefs are the refs git reads from each worktree's own registration, `$G/worktrees/<name>/refs/…`, and the
// owner checkout's from the shared .git (git 2.55, is_per_worktree_ref). What such a name leads to depends on which
// checkout's git reads it, so no one look at the start can follow it for every checkout: symrefWay refuses it.
var perWorktreeRefs = []string{"refs/worktree/", "refs/bisect/", "refs/rewritten/"}

// refKeeper is the folder the core keeps, empty, in each of rowRefFolders' folders, so none is ever empty. Git reads no
// name with a leading dot as a ref or a reflog (for-each-ref, pack-refs, gc, reflog expire skip it), and `git fsck` and
// `git refs verify` pass over a folder: a file there, even `<name>.lock`, is a bad ref to them (git 2.55, and Apple's
// 2.54 for a .lock's content). Its own name has a leading dot, which no branch the Policy holds has (branchName).
const refKeeper = ".smolcorp-keep"

// gitRows2 adds FXGITROWS2's rows for the shared .git git, on a Mac.
//
// packed-refs after them (the owner's D1 (b) ruling): packed-refs and packed-refs.new are the base branches' rows, so
// no desk rewrites packed-refs, and so moves no packed ref, a branch of another desk's, the owner's or its own; its own
// commits write loose refs. packed-refs.lock stays the desk's: its own commit, rebase and branch deletion take it, and
// while a desk holds it the owner's `git pack-refs`, `gc` and branch deletions wait, then fail (a stop, never a move).
// Git never renames packed-refs.lock onto anything (it renames packed-refs.new), so what a desk writes in it is never
// read. Removing another's loose ref would bring back its older packed value: rowNotOwnBranches and rowTags deny that
// for every branch and tag but the desk's own (FXGITROWS2R4).
// A loose ref is read before a packed one, so each branch held here is held by its loose name, and its folders by
// keepRefFolders: refs/heads/hire holds against a move (denyAbove), no desk makes it while it is missing, and the core
// keeps it from ever being empty, so git never removes it (FXGITROWS2R1). A name is held by its path, which Seatbelt
// checks where its links lead: so neither it nor a folder on the way to it may be a link. No desk makes a link in
// refs/ or logs/ (rowRefLinks), but one can rename in a folder that holds one, and a program outside any sandbox can
// make one: each start looks at the whole of refs/ and logs/ and is refused for any link there, and no keeper or held
// folder is made through one (CheckRefPlaces, makeRefFolder: RT126FIXREVIEW-GITROWS2R1's and R2's B1).
func (b *builder) gitRows2(pl Places, git string) {
	if b.goos != "darwin" {
		return
	}
	at := func(names ...string) string { return path.Join(append([]string{spell(b.goos, git)}, names...)...) }
	b.beside(Rule{Row: rowDefaultName, Path: at("refs/remotes/origin/HEAD"), Write: Deny}, gitLock)

	// No desk makes a link in refs/ or logs/, symbolic or hard. A desk's link at a folder no row holds yet
	// (refs/heads/topic, to a folder in its worktree) would put the owner's later `git switch -c topic/x` in the
	// desk's worktree, and the next start's row for topic/x, a path, would hold the link's name only. Git makes none
	// there (its docs, and git 2.55 and Apple's 2.54 run here): a symbolic ref is a file, `ref: refs/…`, unless
	// core.preferSymlinkRefs is on, which git 3.0 drops, and git's own hard links are in objects/. Below a held place
	// a link was a write already; this holds the folders no row holds yet. It doesn't stop a desk renaming in a folder
	// it made that holds a link (Seatbelt can't tell a folder renamed in from one made): the start's look at the whole
	// of refs/ and logs/ refuses that one (CheckRefPlaces), so it's a stop, never a move.
	for _, folder := range []string{"refs", "logs"} {
		b.add(Rule{Row: rowRefLinks, Path: at(folder), Below: true, Link: Deny})
	}

	// rr-cache is denied whole, the folder itself too, so no desk makes it (and Hold never does). `git rerere`,
	// rerere-train (contrib/rerere-train.sh, a `git rerere` in a checkout of the history) and a merge with
	// rerere.enabled all write only these. The other merge inputs a desk might write are held already: info/attributes
	// (the shared git places' info, whole), the config (a merge driver, rerere.enabled, rerere.autoUpdate) and each
	// worktree's config.worktree.
	b.add(Rule{Row: rowRerere, Path: at("rr-cache"), Below: true, Write: Deny})
	b.beside(Rule{Row: rowRerere, Path: at("MERGE_RR"), Write: Deny}, gitLock)
	b.beside(Rule{Row: rowRerere, Path: at("worktrees"), Tail: "*/MERGE_RR", Write: Deny}, gitLock)

	// Every desk's branch folder but its own: `hire/<OwnID>`, and the others the core names (Places.OwnBranches),
	// each with what is below it and its lock. A desk hired after this one starts is held too: no folder is named.
	own := append([]string{"hire/" + pl.OwnID}, pl.OwnBranches...)
	for _, folder := range []string{"refs/heads", "logs/refs/heads"} {
		var except []string
		for _, name := range own {
			except = append(except, at(folder, name), at(folder, name+gitLock))
		}
		b.add(Rule{Row: rowOtherBranches, Path: at(folder, "hire"), Tail: "*", Below: true, Write: Deny, Except: except})
	}
	// Every other branch, as an allowlist (FXGITROWS2R4, after RT126FIXREVIEW-GITROWS2R3's B1): refs/heads and
	// logs/refs/heads whole, the folders themselves too, but the desk's own branches above, each with everything below
	// it (its folder, its refs and their `.lock`s, a reflog's `.lock` from `reflog expire`) and its own `.lock`. A
	// branch held by its path (the owner checkout's, R1 to R3) left every branch no row named yet the desk's to write,
	// and each could become held later: a folder renamed in, a link, a symbolic ref (`git symbolic-ref
	// refs/heads/topic/y refs/heads/hire/h1/work`, then the owner's `git switch topic/y` puts the desk's commits on the
	// owner's HEAD). Now no desk makes, writes, renames in, removes or links anything there but its own, so no later
	// checkout finds a branch a desk made. A desk loses `git branch`, `switch -c` and `checkout -b` of any other name,
	// as a Linux desk's view does (bwrap.go). Git makes the folders on the way (refs/heads/hire) only when missing: the
	// core keeps them (keepRefFolders). Linux's view isn't changed here: it lets a desk write its own branch folder only.
	for _, folder := range []string{"refs/heads", "logs/refs/heads"} {
		var except []string
		for _, name := range own {
			except = append(except, at(folder, name), at(folder, name+gitLock))
		}
		b.add(Rule{Row: rowNotOwnBranches, Path: at(folder), Below: true, Write: Deny, Except: except})
	}
	// Tags, whole (FXGITROWS2R4): a desk's `git tag -f v1.0` moved the owner's release tag, which the owner's fetch
	// doesn't put back, and a tag named for a branch (`refs/tags/feature`) is what the owner's `git log feature` and
	// `git merge feature` take, with only a warning. A desk's work needs none: its fetch from origin follows no tags
	// (DeskGitConfig), and a tag is the owner's to make.
	for _, folder := range []string{"refs/tags", "logs/refs/tags"} {
		b.add(Rule{Row: rowTags, Path: at(folder), Below: true, Write: Deny})
	}
	// Every other ref, as an allowlist over refs/ and logs/refs/ whole (FXGITROWS2R5, RT126FIXREVIEW-GITROWS2R4's N1 and
	// "Seen outside"): the desk's own branches as above, and the remote-tracking branches, which its fetch writes
	// (origin/HEAD is rowDefaultName's, a deny of its own). The rest is the owner's:
	//   - refs/<name> and refs/hire/h2/work, which the owner's `git log` and `git merge` of feature or hire/h2/work take
	//     before refs/heads/<name>, with only an "ambiguous" warning;
	//   - refs/stash, which every worktree shares: the owner's `git stash pop` would apply a desk's stash;
	//   - refs/replace, what every commit reads as for the owner's git (rowSubstitutions names it too);
	//   - refs/notes, what the owner's `git log` shows beside a commit, refs/prefetch, refs/original, any new folder.
	// A desk in its own worktree keeps bisect, `rebase -r` and refs/worktree: git keeps those in its registration. It
	// loses `git stash`, `git notes`, and a `rebase --autostash` whose stash doesn't apply again: it saves work in
	// progress as a commit on its own branch, as its start words say.
	for _, folder := range []string{"refs", "logs/refs"} {
		var except []string
		for _, name := range own {
			except = append(except, at(folder, "heads", name), at(folder, "heads", name+gitLock))
		}
		except = append(except, at(folder, "remotes"))
		b.add(Rule{Row: rowNotOwnRefs, Path: at(folder), Below: true, Write: Deny, Except: except})
	}
	// Every worktree's registration but the desk's own, whole (FXGITROWS2R5, RT126FIXREVIEW-GITROWS2R4's N2): one made
	// after the desk's start (a later hire's, the owner's `git worktree add`) is held as well, which OtherWorktrees,
	// named at the start, didn't hold. Its HEAD was the desk's to point at the desk's own branch, whose commits then
	// moved that worktree's HEAD. In its own, the rows that name a file there still hold (commondir, gitdir, MERGE_RR).
	var ownWorktree []string
	if pl.OwnWorktree != "" {
		ownWorktree = []string{at("worktrees", pl.OwnWorktree)}
	}
	b.add(Rule{Row: rowOtherWorktrees, Path: at("worktrees"), Below: true, Write: Deny, Except: ownWorktree})
	// The rest of the shared .git, as an allowlist too (FXGITROWS2R6, RT126FIXREVIEW-GITROWS2R5's N2): the owner's git
	// reads `$G/feature` and `$G/hire/h2/work` before the branches of those names, and takes them with only an
	// "ambiguous" warning, so a desk's file there was what the owner's `git log feature` and `git merge hire/h2/work`
	// took; `$G/FEATURE_HEAD` and any other new name the same. Open are the folders whose rows hold what is inside them
	// (objects, refs and logs/refs, worktrees), and only what a desk's own work writes beside them, found by tracing
	// that work under this row (git 2.55: commit, amend, rebase, fetch, merge, pull --rebase, a signed commit, a push,
	// gc --auto, bisect, git-lfs):
	//   - packed-refs.lock, which its commit, rebase and branch deletion take (the D1 (b) ruling, above);
	//   - lfs, git-lfs's own store, where its clean filter puts each file a commit takes; only a branch named lfs/… could
	//     be read there;
	//   - for a desk at the project, FETCH_HEAD, which its fetch writes there (a desk in its own worktree writes its own
	//     in its registration), and the checkout's merge, pick and rebase state, which it keeps as its own (FXBASEHEADS,
	//     ruling #3: ownersStateNames), each with its lock. "At the project" is a desk whose place is the project's main
	//     checkout: git gave it no registration of its own in worktrees/ (OwnWorktree), and the core didn't put it in a
	//     worktree of a checkout (Checkout). A Mac plain hire has no Checkout wherever it is, so OwnWorktree is what says
	//     it (RT126FIXREVIEW-GITROWS2R6's N2): in an owner's linked worktree, the main checkout's MERGE_HEAD would make
	//     the owner's next commit a merge of the hire's.
	// A desk in its own worktree keeps the rest of its work's files in its registration (FETCH_HEAD, ORIG_HEAD, gc.log,
	// its rebase, merge and bisect state). gc.pid stays the owner's: a desk's `git gc` that gets that far stops at
	// packed-refs.new all the same (the base branches' row).
	top := []string{at("objects"), at("refs"), at("logs", "refs"), at("worktrees"), at("packed-refs" + gitLock), at("lfs")}
	if pl.OwnWorktree == "" && (pl.Checkout == "" || key(b.goos, pl.Checkout) == key(b.goos, pl.Root)) {
		for _, name := range append([]string{"FETCH_HEAD"}, ownersStateNames...) {
			top = append(top, at(name), at(name+gitLock))
		}
	}
	b.add(Rule{Row: rowGitTop, Path: at(), Below: true, Write: Deny, Except: top})
	// git-lfs's store, the folder itself (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's O2): what is in it stays the desk's,
	// but no desk removes it, renames it away or makes a link at its name, which would stop every later start (ownLink)
	// until the owner removed it. git-lfs makes the folder when it is missing and never removes or replaces it (traced
	// under this row: install, track, a commit, checkout, pull, fetch, push, fsck, prune, dedup and migrate import), so a
	// desk loses nothing. Its hold (denyAbove) has the core make it when missing (Hold), so none is renamed in either.
	b.add(Rule{Row: rowGitTop, Path: at("lfs"), Move: Deny, Link: Deny})
	// The owner checkout's branch, for a desk in its own worktree: a desk at the project works on it.
	if pl.CheckoutBranch != "" && pl.Checkout != "" && key(b.goos, pl.Checkout) != key(b.goos, pl.Root) {
		for _, folder := range []string{"refs/heads", "logs/refs/heads"} {
			b.beside(Rule{Row: rowCheckoutBranch, Path: at(folder, pl.CheckoutBranch), Write: Deny}, gitLock)
		}
	}
}

// keepRefFolders holds, on a Mac, each folder in the shared .git's refs/ or logs/refs/ that denyAbove holds, that
// the desk may write itself, and that git removes once it is empty: any below refs/<kind>/, which git's own pruning
// spares (refs/heads, refs/tags, refs/remotes). Such a folder goes missing in normal use: every ref packed (the
// owner's `git gc` or `pack-refs`, an auto gc), then a branch below it deleted outside the sandbox (the fire's `git
// branch -D`, the owner's), and git removes the empty folder. A desk could then make it again, a folder it prepared
// renamed in with another desk's branch inside, which shadows the packed one (RT126FIXREVIEW-GITROWS2's B1).
//   - The folder itself is denied writing, so no desk makes it while it is missing, by mkdir, a rename or a link. What
//     is below it stays as its other rows say: the desk's own branch's folder and its files are still its work.
//   - The core makes it at the start when missing, with refKeeper in it, which no desk writes, so git never finds it
//     empty and a desk's own commit always has it (Keep, PrepareProfile). Should the owner remove the keeper too, a
//     desk's commit there stops until its next start: a stop, never a move.
//
// A folder the desk can't write is passed, unless a deny around it leaves a place below it the desk's (an Except):
// since rowNotOwnBranches, refs/heads/hire is no desk's to make, and the desk's own branch is below it
// (FXGITROWS2R4). The owner's folders (refs/heads/topic) need no keeper any more: no desk makes them.
func (b *builder) keepRefFolders(git string) {
	if b.goos != "darwin" || git == "" {
		return
	}
	// The folders denyAbove will hold, as it will: on a copy of the rows so far, which are all but these.
	trial := &builder{goos: b.goos, rules: slices.Clone(b.rules)}
	trial.denyAbove()
	g := key(b.goos, git)
	seen := &Policy{goos: b.goos, rules: trial.rules}
	for _, r := range trial.rules {
		if r.Row != rowAbove || r.Tail != "" || !under(r.key, g) || r.key == g {
			continue
		}
		names := strings.Split(strings.TrimPrefix(r.key, strings.TrimSuffix(g, "/")+"/"), "/")
		if names[0] == "logs" {
			names = names[1:]
		}
		if len(names) < 3 || names[0] != "refs" {
			continue
		}
		if v, _ := seen.Decide(r.Path, Write); v == Deny && !opensBelow(trial.rules, r.key) {
			continue // denied whole (refs/heads/release, the owner's refs/heads/topic): nothing is renamed onto it
		}
		b.add(Rule{Row: rowRefFolders, Path: r.Path, Write: Deny})
		b.add(Rule{Row: rowRefFolders, Path: path.Join(r.Path, refKeeper), Below: true, Write: Deny})
	}
}

// opensBelow is whether one of rules leaves a place strictly below folder k, a key, the desk's (an Except).
func opensBelow(rules []Rule, k string) bool {
	return slices.ContainsFunc(rules, func(r Rule) bool {
		return slices.ContainsFunc(r.except, func(e string) bool { return e != k && under(e, k) })
	})
}

// Keep is the keepers of the Policy's ref folders (keepRefFolders), each a folder the core makes, empty and
// owner-only, with the folder it is in, before the desk starts (PrepareProfile, through makeHold: never by a link).
func (p *Policy) Keep() []string {
	var keepers []string
	for _, r := range p.rules {
		if r.Row == rowRefFolders && r.Below {
			keepers = append(keepers, r.Path)
		}
	}
	slices.Sort(keepers)
	return keepers
}

// RefPlaceError is a start refused for what is at At, Ref below the shared .git (CheckRefPlaces): in its refs/ or
// logs/, or on the way to a base branch's name beside them, a link (Link), a file with another name (Hard), or
// something that is neither a folder nor a file. Symref is a link's text when the link is in refs/ and its text
// starts "refs/": a symbolic ref git wrote as a link, with core.preferSymlinkRefs on, most likely, but refused all the
// same.
//
// Or, with Link false and Symref set (FXGITROWS2R4), At is a ref p holds, or the owner checkout's HEAD (Ref "HEAD"),
// that is a symbolic ref git reads as Symref, and Moves is the first ref on its way a desk can move ("" when the way
// leaves refs/ or doesn't end): whatever moves Moves moves it too.
//
// Or, with Commit (FXGITROWS2R7, RT126FIXREVIEW-GITROWS2R6's O2), At is a file at the top of the shared .git, not one of
// git's own, that holds a commit's id at a name git reads as a ref: the owner's git takes that commit for the branch of
// that name.
//
// Or, with Link and Leads (FXGITROWS2R8, RT126FIXREVIEW-GITROWS2R7's B1), At is a link at one of git's own names at the
// top of the shared .git (hooks, config, rr-cache, FETCH_HEAD…), and Leads where it leads, a place the desk can write:
// what git reads or runs there would be the desk's. With Unfollowed too, it can't be followed (it leads nowhere, or
// to a place that can't be read), and Leads is its text.
//
// Or, with Link and Via (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B1), At is such a link whose way goes through Via, a
// link a desk can change (At itself, or one on the way): where At leads is then the desk's to choose, whatever it is
// at the start. Leads is where it leads now, "" when it can't be followed.
//
// Own is whether At is one of git's own files at the top of the shared .git (gitsOwnFiles): with Hard, a file of
// git's own state, or of the repository's, that has another name (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B2).
type RefPlaceError struct {
	At, Ref    string
	Link       bool
	Hard       bool
	Symref     string
	Moves      string
	Commit     bool
	Leads      string
	Unfollowed bool
	Via        string
	Own        bool
}

func (e *RefPlaceError) Error() string {
	switch {
	case e.Link && e.Via == e.At:
		return e.At + " is a link a desk can change"
	case e.Link && e.Via != "":
		return e.At + " is a link by way of " + e.Via + ", a link a desk can change"
	case e.Link && e.Unfollowed:
		return e.At + " is a link to " + e.Leads + ", which can't be followed to a place no desk can write"
	case e.Link && e.Leads != "":
		return e.At + " is a link to " + e.Leads + ", where a desk can write"
	case e.Link:
		return e.At + " is a link, where the sandbox holds refs by their paths"
	case e.Hard:
		return e.At + " has another name (a hard link), where the sandbox holds refs by their paths"
	case e.Symref != "" && e.Moves != "":
		return e.At + " is a symbolic ref to " + e.Symref + ", by way of " + e.Moves + ", which a desk can move"
	case e.Symref != "":
		return e.At + " is a symbolic ref to " + e.Symref + ", which can't be followed to a ref no desk can move"
	case e.Commit:
		return e.At + " holds a commit's id, which git takes for the branch " + e.Ref
	}
	return e.At + " is neither a folder nor a file, where the sandbox holds refs by their paths"
}

// CheckRefPlaces looks at the whole of the shared .git's refs/ and logs/, never through a link, and from the .git
// down at each folder on the way to a base branch's name beside them ($G/team/main, rowBaseNames) and at that name:
// a link, a file with a second name, or anything but a folder or a file there is a *RefPlaceError, and the desk
// mustn't start. Every ref and reflog p holds is held by its path, and Seatbelt checks where a path's links lead: a
// link at a folder on the way (refs/heads/topic, RT126FIXREVIEW-GITROWS2R1's B1) or at the held file itself
// (refs/heads/topic/y → a file in the desk's worktree, RT126FIXREVIEW-GITROWS2R2's B1) leaves the owner's branch a
// file the desk writes, and the row holds only the link's name. No desk makes a link there (rowRefLinks), but one can
// rename in a folder that holds one, and a program outside any sandbox can make one: so every link there refuses the
// start, not only those on the way to a place held now, since what the owner's git makes later (switch -c topic/y,
// switch to a branch a desk made) is held from the next start on. Git makes none there: a symbolic ref is a file,
// "ref: refs/…", which passes, unless core.preferSymlinkRefs is on, which git 3.0 drops; such a link is refused too
// (RefPlaceError.Symref), since what reads it as a ref and not a path is git's own refname check. A place that isn't
// there, or is below a file (a reftable repo's refs/heads), is passed: nothing can be there. A folder it can't look at
// fails the start. Only the Policy of this system looks.
//
// Symbolic refs (FXGITROWS2R4, RT126FIXREVIEW-GITROWS2R3's B1 (a) and (b)): a ref p holds, whatever row holds it, that
// is a symbolic ref whose way leads to a ref a desk can move refuses the start, since the desk's ordinary commit there
// moves the held one too (`refs/heads/topic/y` → `refs/heads/hire/h1/work`, then the owner's `git switch topic/y`). So
// does the owner checkout's HEAD, for a desk in its own worktree, when its way leads to one (HEAD read as a file or as
// a link). refs/remotes/origin/HEAD is passed: it is a symbolic ref to a remote-tracking branch by its nature, and no
// one commits on it. A symbolic ref whose whole way stays in refs no desk can write (`master` → `main`) is passed.
// A desk's own symbolic ref in its own space (`hire/h2` → `refs/remotes/origin/main`) passes its own start and stops
// every other desk's (RT126FIXREVIEW-GITROWS2R4's O1), and that is right: to them it is a held ref that the remote-tracking
// branch moves, so whichever desk moves that branch would move hire/h2 too; a stop, never a move.
//
// Every other worktree's HEAD too, on a Mac (FXGITROWS2R5, RT126FIXREVIEW-GITROWS2R4's N2): each registration in
// worktrees/ but the desk's own, those made after an earlier start included, is read and followed as the owner
// checkout's HEAD is, so a HEAD made outside the sandbox (or by a desk before rowOtherWorktrees) that points at a branch
// the desk can move refuses the start. A registration that is a link refuses it as well: git makes none, and the rows
// that hold it would hold only the link's name.
//
// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's B1): a way through a ref git reads per worktree (perWorktreeRefs) refuses
// the start, wherever it is met: git never makes a symbolic ref to one (switch and checkout point HEAD at refs/heads/
// only, bisect and rebase leave it detached), and what it names depends on which checkout's git reads it, the desk's
// own registration included. And a HEAD the desk can't write, the shared .git's and each other registration's, that is
// a file with a second name (a hard link) refuses it, as one in refs/ does: through its other name a desk would
// rewrite it in place. So does, on a Mac, a symbolic ref at the rest of the shared .git (topNames) whose way leads to a
// ref a desk can move: rowGitTop holds those names from this build on, and one made before it is a stop, never a move.
// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's B1 and O2): there, outside git's own names, a link, a file with a second
// name, a symbolic ref out of refs/ and a commit's id refuse it as well (topName).
// FXGITROWS2R8 (RT126FIXREVIEW-GITROWS2R7's B1 and O1): a link at one of git's own names there refuses it when it leads
// where the desk can write, or can't be followed (ownLink); and a symbolic ref there whose way leaves refs/ refuses it
// only where that way ends at a name the desk can write, or can't be followed (topWay).
// FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's B1, B2 and N1): such a link is followed one link at a time, and one on the
// way that a desk could change refuses it too (linkWay); one that leads under a place Smolcorp gives any desk, not only
// this one, refuses it (anyDesks); and a file with a second name at one of git's own names refuses it as well.
func (p *Policy) CheckRefPlaces() error {
	if p.git == "" || p.goos != runtime.GOOS {
		return nil
	}
	for _, top := range []string{"refs", "logs"} {
		if err := p.scanRefs(top); err != nil {
			return err
		}
	}
	head := path.Join(p.git, "HEAD")
	to, err := headText(head, "HEAD")
	if err != nil {
		return err
	}
	if to != "" && p.ownerHEAD {
		if err := p.symrefWay(head, "HEAD", to); err != nil {
			return err
		}
	}
	if err := p.worktreeHEADs(); err != nil {
		return err
	}
	if err := p.topNames(); err != nil {
		return err
	}
	for _, r := range p.rules {
		names, ok := p.refPlace(r.Path)
		if !ok || names[0] == "refs" || names[0] == "logs" {
			continue // scanned whole
		}
		at := p.git
		for _, name := range names {
			at = path.Join(at, name)
			info, err := os.Lstat(filepath.FromSlash(at))
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
				break
			} else if err != nil {
				return err
			}
			if err := p.refEntry(at, info.Mode().Type(), info); err != nil {
				return err
			} else if !info.IsDir() {
				break
			}
		}
	}
	return nil
}

// scanRefs is CheckRefPlaces's look at the whole of the shared .git's folder top, refs or logs: each name in it,
// never following a link, a link at top itself too. What git removes meanwhile (a lock, a folder emptied) is passed.
// On a stand-in with 10,000 loose branches, each with its reflog (20,000 files, 208 folders), CheckRefPlaces takes
// about 36ms on a MacBook, warm (FXGITROWS2R3), and about 175ms once each loose ref outside refs/remotes is read for
// a symbolic ref (heldSymref, FXGITROWS2R4: some 14µs a ref); a start runs it twice. A clone's refs are packed, and
// those aren't read.
func (p *Policy) scanRefs(top string) error {
	root := filepath.FromSlash(path.Join(p.git, top))
	return filepath.WalkDir(root, func(at string, e fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		var info fs.FileInfo
		if e.Type().IsRegular() {
			if info, err = e.Info(); errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
		}
		if err := p.refEntry(filepath.ToSlash(at), e.Type(), info); err != nil || top != "refs" || !e.Type().IsRegular() {
			return err
		}
		return p.heldSymref(filepath.ToSlash(at))
	})
}

// heldSymref is CheckRefPlaces's look at at, a file in refs/: when p holds it and it is a symbolic ref, where its way
// leads (symrefWay). A ref no desk can be refused at isn't read: a remote-tracking branch, which no row holds but
// origin/HEAD, and a lock, which git writes whole and renames.
func (p *Policy) heldSymref(at string) error {
	ref := strings.TrimPrefix(at, p.git+"/")
	if strings.HasPrefix(ref, "refs/remotes/") || strings.HasSuffix(ref, gitLock) {
		return nil
	}
	to, err := symrefText(at)
	if err != nil || to == "" {
		return err
	}
	if v, _ := p.Decide(at, Write); v != Deny {
		return nil // the desk's own (refs/heads/hire/h1/x): no row holds it
	}
	return p.symrefWay(at, ref, to)
}

// worktreeHEADs is CheckRefPlaces's look, on a Mac, at the HEAD of each registration in the shared .git's worktrees/
// that the desk can't write (all but its own): a symbolic ref there is followed as the owner checkout's HEAD is. A name
// there that isn't a folder git never reads as a registration, and is passed; a link is refused.
func (p *Policy) worktreeHEADs() error {
	if p.goos != "darwin" {
		return nil // rowOtherWorktrees is a Mac's row
	}
	folder := path.Join(p.git, "worktrees")
	entries, err := os.ReadDir(filepath.FromSlash(folder))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	} else if err != nil {
		return err
	}
	for _, e := range entries {
		at, ref := path.Join(folder, e.Name()), "worktrees/"+e.Name()
		switch {
		case e.Type()&fs.ModeSymlink != 0:
			return &RefPlaceError{At: at, Ref: ref, Link: true}
		case !e.IsDir():
			continue
		}
		head := path.Join(at, "HEAD")
		if v, _ := p.Decide(head, Write); v != Deny {
			continue // the desk's own worktree
		}
		to, err := headText(head, ref+"/HEAD")
		if err != nil {
			return err
		}
		if to != "" {
			if err := p.symrefWay(head, ref+"/HEAD", to); err != nil {
				return err
			}
		}
	}
	return nil
}

// headText is symrefText of a checkout's HEAD at at, ref below the shared .git, but a regular file there with a second
// name is a *RefPlaceError (Hard): a desk that can write the other name (a file in its worktree) rewrites HEAD in place,
// where the rows hold HEAD by its path (RT126FIXREVIEW-GITROWS2R5's B1, HARDWT and OWNERHARD).
func headText(at, ref string) (string, error) {
	info, err := os.Lstat(filepath.FromSlash(at))
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return "", nil
	case err != nil:
		return "", err
	case info.Mode().IsRegular() && linkCount(info) > 1:
		return "", &RefPlaceError{At: at, Ref: ref, Hard: true}
	}
	return symrefText(at)
}

// gitsOwnFolders are the folders git keeps at the top of a .git, which topNames passes over: refs/ and logs/ are scanned
// whole, worktrees/ by worktreeHEADs, and the rest hold no ref git reads by its name, but files whose text may start
// "ref:" all the same (a rerere preimage, a submodule's HEAD). A link at one of these names is followed (ownLink): an
// owner may keep hooks elsewhere, but not where a desk writes.
var gitsOwnFolders = []string{"objects", "refs", "logs", "worktrees", "modules", "lfs", "hooks", "info", "rr-cache",
	"rebase-merge", "rebase-apply", "sequencer", "reftable", "branches", "remotes", "svn", "lost-found", "annex"}

// gitsOwnFiles are the files git keeps at the top of a .git (git 2.55, gitrepository-layout and the commands that write
// them), which topName doesn't refuse for being a link or a commit's id: git writes some of them as one (ORIG_HEAD,
// MERGE_HEAD, a shallow clone's shallow), and an owner may keep config elsewhere (a link is ownLink's to follow).
// Their names are git's, not a branch's. A symbolic ref in one is still followed, and a file with a second name is
// refused (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B2): git makes no hard link there, and through its other name a
// desk would rewrite it in place (FETCH_HEAD, MERGE_AUTOSTASH, planted under an older profile).
var gitsOwnFiles = []string{"config", "config.worktree", "description", "index", "packed-refs", "shallow", "FETCH_HEAD",
	"ORIG_HEAD", "MERGE_HEAD", "MERGE_MSG", "MERGE_MODE", "MERGE_RR", "MERGE_AUTOSTASH", "AUTO_MERGE", "CHERRY_PICK_HEAD",
	"REVERT_HEAD", "REBASE_HEAD", "BISECT_HEAD", "BISECT_EXPECTED_REV", "BISECT_START", "BISECT_LOG", "BISECT_NAMES",
	"BISECT_TERMS", "BISECT_RUN", "BISECT_FIRST_PARENT", "BISECT_ANCESTORS_OK", "COMMIT_EDITMSG", "SQUASH_MSG",
	"TAG_EDITMSG", "NOTES_EDITMSG", "NOTES_MERGE_PARTIAL", "NOTES_MERGE_REF", "EDIT_DESCRIPTION", "gc.pid", "gc.log",
	"gitk.cache"}

// topNames is CheckRefPlaces's look, on a Mac, at the rest of the shared .git (rowGitTop): each name there but HEAD and
// git's own folders, and each name below a folder there that isn't one of them, never through a link, that the desk
// can't write and that isn't a lock, is looked at by topName. The owner's git reads `$G/feature` and `$G/hire/h2/work`
// before the branches of those names, so one made before rowGitTop (by a desk under an older profile, or a program
// outside any sandbox) is what the owner's `git log feature` takes. A link at one of git's own names there is looked at
// by ownLink first.
func (p *Policy) topNames() error {
	if p.goos != "darwin" {
		return nil // rowGitTop is a Mac's row
	}
	entries, err := os.ReadDir(filepath.FromSlash(p.git))
	if err != nil {
		return err
	}
	for _, e := range entries {
		own := slices.Contains(gitsOwnFolders, e.Name()) || slices.Contains(gitsOwnFiles, e.Name())
		if e.Type()&fs.ModeSymlink != 0 && own {
			if err := p.ownLink(path.Join(p.git, e.Name()), e.Name()); err != nil {
				return err
			}
		}
		if e.Name() == "HEAD" || slices.Contains(gitsOwnFolders, e.Name()) && (e.IsDir() || e.Type()&fs.ModeSymlink != 0) {
			continue
		}
		err := filepath.WalkDir(filepath.Join(filepath.FromSlash(p.git), e.Name()), func(at string, e fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil || e.IsDir() {
				return err
			}
			at = filepath.ToSlash(at)
			if v, _ := p.Decide(at, Write); v != Deny || strings.HasSuffix(at, gitLock) {
				return nil // the desk's own (packed-refs.lock, a desk at the project's FETCH_HEAD), or a lock
			}
			return p.topName(at)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// topName is topNames' look at at, a name below the shared .git the desk can't write: a *RefPlaceError when the owner's
// git could take a desk's commit for the branch of that name through it (RT126FIXREVIEW-GITROWS2R6's B1 and O2).
//   - A link, a file's or a folder's, unless at one of git's own files: git reads a link whose text isn't refs/… as a
//     file, through it, so a link into the desk's worktree or git-lfs's store is a file the desk writes; git makes none.
//   - A file with a second name, git's own too (FXGITROWS2R9): the desk could rewrite it in place through the other.
//   - A symbolic ref to any name git takes for a ref (no space in it: an owner's message "ref: #12 fix" in
//     COMMIT_EDITMSG or MERGE_MSG is no ref to git, and passes), followed as git follows it (topWay): a way that ends
//     where the desk writes, `ref: lfs/x`, `ref: objects/x`, a desk at the project's `ref: FETCH_HEAD`, refuses it.
//   - A commit's id, unless one of git's own, at a name git reads as a ref (refNameGitReads): it doesn't move, but the
//     owner's git takes the commit for that branch, with at most an "ambiguous" warning (O2).
func (p *Policy) topName(at string) error {
	ref := strings.TrimPrefix(at, p.git+"/")
	own := slices.Contains(gitsOwnFiles, ref)
	info, err := os.Lstat(filepath.FromSlash(at))
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return nil
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0 && !own:
		return &RefPlaceError{At: at, Ref: ref, Link: true}
	case info.Mode().IsRegular() && linkCount(info) > 1:
		return &RefPlaceError{At: at, Ref: ref, Hard: true, Own: own}
	}
	to, err := symrefText(at)
	switch {
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		if strings.HasPrefix(to, "refs/") {
			return p.symrefWay(at, ref, to) // one of git's own, as git writes it with core.preferSymlinkRefs
		}
		return nil // one of git's own, read through: an owner's config kept elsewhere
	case to != "" && !strings.ContainsFunc(to, func(r rune) bool { return r <= ' ' || r == 0x7f }):
		return p.topWay(at, ref, to)
	case own || !info.Mode().IsRegular() || !refNameGitReads(ref):
		return nil
	}
	text, err := fileStart(at)
	if err != nil || !commitID(text) {
		return err
	}
	return &RefPlaceError{At: at, Ref: ref, Commit: true}
}

// refNameGitReads is whether git takes name, a path below the .git, for a ref's name (check_refname_format with
// REFNAME_ALLOW_ONELEVEL, git 2.55): parts that aren't empty, don't start with "." and don't end ".lock"; no "..",
// "@{", control character, space or any of ~^:?*[\; not "@", and not ending "/" or ".".
func refNameGitReads(name string) bool {
	if name == "" || name == "@" || strings.HasSuffix(name, ".") || strings.Contains(name, "..") || strings.Contains(name, "@{") ||
		strings.ContainsFunc(name, func(r rune) bool { return r <= ' ' || r == 0x7f || strings.ContainsRune(`~^:?*[\`, r) }) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, gitLock) {
			return false
		}
	}
	return true
}

// commitID is whether text, the start of a file, is what git reads there as a commit's id: 40 hex digits (sha1) or 64
// (sha256), then the end or a space (FETCH_HEAD's way: git reads what follows a space as nothing).
func commitID(text []byte) bool {
	n := 0
	for n < len(text) && strings.IndexByte("0123456789abcdefABCDEF", text[n]) >= 0 {
		n++
	}
	return (n == 40 || n == 64) && (n == len(text) || strings.IndexByte(" \t\n\v\f\r", text[n]) >= 0)
}

// ownLink is topNames' look at at, a link at name, one of git's own names at the top of the shared .git (FXGITROWS2R8,
// RT126FIXREVIEW-GITROWS2R7's B1 and its "seen outside" hooks link): git reads, writes and runs what is there through
// the link, and the rows hold only the link's name, so a *RefPlaceError when it leads where the desk can write
// (deskWrites), or can't be followed. A desk under an older profile could plant rr-cache, MERGE_AUTOSTASH or FETCH_HEAD
// as a link into its worktree, and the owner's merge, commit or log would take what it writes there; an owner's hooks
// kept in the checkout (hooks → ../.githooks) are a desk at the project's to write.
//
// FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's B1 and N1): the way is followed one link at a time (linkWay), and a link on it
// that a desk could change refuses the start (Via), wherever the way leads now: a desk that points it elsewhere after
// the start chooses where git reads. And where it leads must be no desk's, not only not this one's (anyDesks): a place
// held from this desk may be another's, another hire's or another project's, whose start never looks at this .git.
func (p *Policy) ownLink(at, name string) error {
	end, via, err := p.linkWay(at, false)
	if via != "" {
		return &RefPlaceError{At: at, Ref: name, Link: true, Leads: end, Via: via}
	}
	if err == nil {
		var writes bool
		if writes, err = p.deskWrites(end); err == nil && !writes && !p.anyDesks(end) {
			return nil
		} else if err == nil {
			return &RefPlaceError{At: at, Ref: name, Link: true, Leads: end}
		}
	}
	text, _ := os.Readlink(filepath.FromSlash(at))
	return &RefPlaceError{At: at, Ref: name, Link: true, Leads: text, Unfollowed: true}
}

// linkHopsMax is how many links linkWay follows on one way before it gives up, as the kernel does (MAXSYMLINKS, 32 on
// a Mac).
const linkHopsMax = 32

// linkWay is where place, a path, leads, followed one name at a time as the kernel follows it (FXGITROWS2R9,
// RT126FIXREVIEW-GITROWS2R8's B1): each link on the way, at place itself or at a folder on the way, is read and its text
// followed in turn, from the link's folder or from the top, up to linkHopsMax links. The shared .git, as the rows spell
// it, is taken as it is: the core gives it resolved. via is the first link on the way a desk could change
// (linkChangeable), "" for none. A name that isn't there ends the way, the rest as written, when missing allows it and
// no link's text named it; else that, a link that can't be read, a file on the way or more than linkHopsMax links is
// an error.
func (p *Policy) linkWay(place string, missing bool) (end, via string, err error) {
	type step struct {
		name   string
		linked bool // from a link's text
	}
	steps := func(s string, linked bool) []step {
		var out []step
		for _, name := range strings.Split(s, "/") {
			if name != "" && name != "." {
				out = append(out, step{name, linked})
			}
		}
		return out
	}
	done, rest := "/", place
	if after, ok := strings.CutPrefix(place, p.git+"/"); ok && p.git != "" {
		done, rest = p.git, after
	}
	todo, hops := steps(rest, false), 0
	for len(todo) > 0 {
		s := todo[0]
		todo = todo[1:]
		if s.name == ".." {
			done = path.Dir(done)
			continue
		}
		at := path.Join(done, s.name)
		info, err := os.Lstat(filepath.FromSlash(at))
		switch {
		case missing && !s.linked && errors.Is(err, fs.ErrNotExist):
			for _, t := range todo {
				at = path.Join(at, t.name)
			}
			return at, via, nil
		case err != nil:
			return "", via, err
		case info.Mode()&fs.ModeSymlink == 0:
			if len(todo) > 0 && !info.IsDir() {
				return "", via, syscall.ENOTDIR
			}
			done = at
			continue
		}
		if hops++; hops > linkHopsMax {
			return "", via, syscall.ELOOP
		}
		if via == "" && p.linkChangeable(at) {
			via = at
		}
		text, err := os.Readlink(filepath.FromSlash(at))
		if err != nil {
			return "", via, err
		}
		if text = filepath.ToSlash(text); path.IsAbs(text) {
			done = "/"
		}
		todo = append(steps(text, true), todo...)
	}
	return done, via, nil
}

// linkChangeable is whether a desk could change the link at at, or move it away with a folder it is in, so that a way
// through it leads elsewhere (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's B1): it is under a place Smolcorp gives some desk
// (anyDesks); or the desk's Policy lets it write the link or its folder, or move a folder above them, and the folder
// that holds that name lets this user write in it (userWrites: where the owner can't, at / or in a folder of root's,
// no desk can either). A link whose Move and Link the Policy denies (git-lfs's store, `$G/lfs`) is no desk's to
// change, though what is below it is the desk's to write: no desk removes it, renames it away or makes a link at its
// name (RT126FIXREVIEW-GITROWS2R9's O1).
func (p *Policy) linkChangeable(at string) bool {
	if p.anyDesks(at) {
		return true
	}
	move, _ := p.Decide(at, Move)
	link, _ := p.Decide(at, Link)
	if v, _ := p.Decide(at, Write); v != Deny && (move != Deny || link != Deny) && userWrites(path.Dir(at)) {
		return true
	}
	for folder, op := path.Dir(at), Write; folder != path.Dir(folder); folder, op = path.Dir(folder), Move {
		if v, _ := p.Decide(folder, op); v != Deny && userWrites(path.Dir(folder)) {
			return true
		}
	}
	return false
}

// gitDesksPlaces are the places in the shared .git that some desk writes for itself and that other desks' Policies
// deny (anyDesks): every worktree's registration, and every desk's branches and their reflogs.
var gitDesksPlaces = []string{"worktrees", "refs/heads/hire", "logs/refs/heads/hire"}

// desksPlaces is the keys of the places Smolcorp gives some desk, as pl names them for goos (Policy.desks): the support
// folders, each desk's hires/<id> in them, the socket folder, tmux's folder, the GM's folder, the hires folder with
// every hire's worktree, the owner's checkout (a desk at the project, a Mac plain hire at the main checkout), the
// desk's own place, and the whole projects folder (Projects, or a GM's GMProjects: FXHIREPROJECTS holds it from this
// desk, but every other project in it is its own desks' place, whose starts never look at this .git; FXSTACKG G22,
// RT126FIXREVIEW-GITROWS2R9's N1). This repo's own .git, where it is in the projects folder, is anyDesks' to judge
// by gitDesksPlaces, as before.
func desksPlaces(goos string, pl Places) []string {
	var keys []string
	places := append([]string{pl.Support, pl.SocketParent, pl.TmuxFolder, pl.GMFolder, pl.HiresFolder, pl.Checkout, pl.Root, pl.Projects, pl.GMProjects}, pl.OtherSupports...)
	for _, place := range append(places, pl.Worktrees...) { // and the core's folders for every hire's worktree (FXCOREWORKTREES126)
		if k := key(goos, place); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// anyDesks is whether place, a resolved path, is under a place Smolcorp gives some desk, this one or another
// (FXGITROWS2R9, RT126FIXREVIEW-GITROWS2R8's N1): one of desksPlaces, or in the shared .git one of gitDesksPlaces. A
// desk's Policy denies another desk's place, but that desk writes it, and its own start doesn't look at this .git:
// "held from this desk" is enough only where every desk is held. A place that isn't whole is taken as one.
func (p *Policy) anyDesks(place string) bool {
	k := key(p.goos, place)
	if k == "" {
		return true
	}
	if g := key(p.goos, p.git); g != "" && under(k, g) {
		return slices.ContainsFunc(gitDesksPlaces, func(name string) bool { return under(k, path.Join(g, name)) })
	}
	return slices.ContainsFunc(p.desks, func(d string) bool { return under(k, d) })
}

// deskWrites is whether the desk can write at place, a resolved path, or, when it is a folder, at a name in it: a deny
// of the folder alone, or a row that leaves a place below it the desk's (an allow there, or an Except), says so. A
// desk's Policy denies only what it holds: an owner's folder no row holds (~/dotfiles) is one the desk writes.
func (p *Policy) deskWrites(place string) (bool, error) {
	if v, _ := p.Decide(place, Write); v != Deny {
		return true, nil
	}
	info, err := os.Stat(filepath.FromSlash(place))
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return false, nil // not there: the desk can't make it
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, nil
	}
	if v, _ := p.Decide(path.Join(place, "x"), Write); v != Deny {
		return true, nil
	}
	k := key(p.goos, place)
	return opensBelow(p.rules, k) || slices.ContainsFunc(p.rules, func(r Rule) bool {
		return r.Write == Allow && r.key != "" && r.key != k && under(r.key, k)
	}), nil
}

// rootRef is whether git reads name, at the top of a .git, per worktree (git 2.55, is_root_ref_syntax: capitals, "_"
// and "-" only; FETCH_HEAD, ORIG_HEAD, COMMIT_EDITMSG): a linked worktree's git reads it in its own registration, never
// in the shared .git.
func rootRef(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(r rune) bool { return (r < 'A' || r > 'Z') && r != '_' && r != '-' })
}

// symrefMax is how many symbolic refs git follows, one to the next, before it gives up (SYMREF_MAXDEPTH, git 2.55).
const symrefMax = 5

// symrefWay follows the symbolic ref at, ref below the shared .git, from to, its text, one ref to the next as git does:
// a *RefPlaceError when a ref on the way is one the desk can write (Moves), or when the way leaves refs/, goes through
// a ref git reads per worktree (perWorktreeRefs), or doesn't end within symrefMax refs. It ends at a ref that isn't a
// symbolic ref, or isn't there as a file (packed, or not made yet: a desk can't make one it can't write).
func (p *Policy) symrefWay(at, ref, to string) error { return p.way(at, ref, to, false) }

// topWay is symrefWay for a name at the top of the shared .git, or in a folder there (topName), but a way that leaves
// refs/ is followed on, as git follows it (FXGITROWS2R8, RT126FIXREVIEW-GITROWS2R7's O1): it refuses the start only
// where it goes through or ends at a name the desk can write, by that name or where a link on the way leads (`ref:
// lfs/x`, `ref: objects/x`, a desk at the project's `ref: FETCH_HEAD`), or through a link on the way a desk could
// change, or where it leads under a place Smolcorp gives any desk (linkWay, anyDesks: FXGITROWS2R9), or can't be followed: a link there, a way out
// of the .git, to another worktree's own (worktrees/…, main-worktree/…), or one that leads nowhere. An all-capitals
// name (rootRef) on the way is read per worktree: from a name only the main checkout's git reads (COMMIT_EDITMSG), it
// is the shared .git's; from one every worktree's git reads (feature), it is each one's, the desk's own included, and
// refused. So an owner's message `ref: cleanup` in COMMIT_EDITMSG, which git follows to $G/cleanup, a name no desk
// writes, passes.
func (p *Policy) topWay(at, ref, to string) error { return p.way(at, ref, to, true) }

func (p *Policy) way(at, ref, to string, top bool) error {
	refuse := func(moves string) error { return &RefPlaceError{At: at, Ref: ref, Symref: to, Moves: moves} }
	mainOnly := !strings.Contains(ref, "/") && rootRef(ref)
	next := to
	for range symrefMax {
		place := path.Join(p.git, next)
		switch {
		case slices.ContainsFunc(perWorktreeRefs, func(w string) bool { return strings.HasPrefix(place, p.git+"/"+w) }):
			return refuse("")
		case strings.HasPrefix(next, "refs/") && strings.HasPrefix(place, p.git+"/refs/"):
			if v, _ := p.Decide(place, Write); v != Deny {
				return refuse(next)
			}
			// A held branch whose way goes into another desk's branches (RT126FIXREVIEW-GITROWS2R9's O3, SYMX): that
			// desk would move it with its next commit, and this desk's start is the first to see it. A worktree's HEAD
			// on its own desk's branch, and a desk's own symbolic ref among its own branches, are that desk's.
			if strings.HasPrefix(ref, "refs/") && !p.anyDesks(at) && p.anyDesks(place) {
				return refuse(next)
			}
		case !top || !strings.HasPrefix(place, p.git+"/") || strings.HasPrefix(next, "worktrees/") ||
			strings.HasPrefix(next, "main-worktree/") || rootRef(next) && !mainOnly:
			return refuse("")
		default:
			if v, _ := p.Decide(place, Write); v != Deny {
				return refuse(next)
			}
			if info, err := os.Lstat(filepath.FromSlash(place)); err == nil && info.Mode()&fs.ModeSymlink != 0 {
				return refuse("") // read through: looked at in its own turn, not followed here
			}
			real, via, err := p.linkWay(place, true)
			switch {
			case via != "":
				return refuse(next) // a link on the way a desk can change (FXGITROWS2R9)
			case err != nil:
				return refuse("")
			}
			if writes, err := p.deskWrites(real); err != nil {
				return refuse("")
			} else if writes || p.anyDesks(real) {
				return refuse(next)
			}
		}
		n, err := symrefText(place)
		if err != nil || n == "" {
			return err
		}
		next = n
	}
	return refuse("")
}

// symrefText is what the ref at at points at when git reads it as a symbolic ref: the text after "ref:" in a file, or
// a link's text, as git reads one (core.preferSymlinkRefs); "" for anything else, a commit's id, or nothing there.
func symrefText(at string) (string, error) {
	at = filepath.FromSlash(at)
	info, err := os.Lstat(at)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return "", nil
	case err != nil:
		return "", err
	case info.Mode()&fs.ModeSymlink != 0:
		return os.Readlink(at)
	case !info.Mode().IsRegular():
		return "", nil
	}
	b, err := fileStart(filepath.ToSlash(at))
	if err != nil {
		return "", err
	}
	text, ok := strings.CutPrefix(string(b), "ref:")
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(text), nil
}

// fileStart is the first 1024 bytes of the file at at, none when it isn't there. Only a plain file is read (openPlain):
// one swapped since its look for a FIFO is refused, never waited on.
func fileStart(at string) ([]byte, error) {
	f, err := openPlain(filepath.FromSlash(at))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, 1024)
	n, err := io.ReadFull(f, b)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return b[:n], nil
}

// openPlain opens the file at at to read it, without waiting on a pipe (keptNoWait), and gives it only if what was
// opened is a regular file: a FIFO there, put at any moment, is refused at once and never read (RT126SWEEP4 part 3's
// B1, as the core's cpReadData). A link is followed, as git follows one.
func openPlain(at string) (*os.File, error) {
	f, err := os.OpenFile(at, os.O_RDONLY|keptNoWait, 0)
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, &os.PathError{Op: "open", Path: at, Err: errors.New("not a plain file")}
	}
	return f, nil
}

// refEntry is what CheckRefPlaces makes of at, of type mode (info for a regular file): nil for a folder or a file of
// one name.
func (p *Policy) refEntry(at string, mode fs.FileMode, info fs.FileInfo) error {
	e := &RefPlaceError{At: at, Ref: strings.TrimPrefix(at, p.git+"/")}
	switch {
	case mode&fs.ModeSymlink != 0:
		e.Link = true
		if to, err := os.Readlink(filepath.FromSlash(at)); err == nil && strings.HasPrefix(e.Ref, "refs/") && strings.HasPrefix(to, "refs/") {
			e.Symref = to
		}
	case mode.IsRegular():
		if linkCount(info) <= 1 {
			return nil
		}
		e.Hard = true
	case mode.IsDir():
		return nil
	}
	return e
}

// refPlace is the names from the shared .git down to at, a place as p spells it, and whether CheckRefPlaces looks
// there: in refs/ or logs/, or on the way to a base branch's name beside them.
func (p *Policy) refPlace(at string) ([]string, bool) {
	rel, ok := strings.CutPrefix(at, p.git+"/")
	if p.git == "" || !ok {
		return nil, false
	}
	names := strings.Split(rel, "/")
	if names[0] == "refs" || names[0] == "logs" {
		return names, true
	}
	return names, slices.ContainsFunc(p.rules, func(r Rule) bool {
		return r.Row == rowBaseNames && (r.Path == at || strings.HasPrefix(r.Path, at+"/"))
	})
}

// makeHold is MakeHold of folders, but each where CheckRefPlaces looks is made by makeRefFolder, never through a link:
// a keeper made through a link at refs/heads/topic would be in the desk's worktree (RT126FIXREVIEW-GITROWS2R1's B1).
// Every other one the owner can't make either doesn't fail the start (makeHeld): unmade are the first missing folders
// of those, each once, for the profile to deny (unmadeHeld). The shared .git is the owner's to write: a ref folder
// that can't be made there still fails the start.
func (p *Policy) makeHold(folders []string) (unmade []string, err error) {
	for _, folder := range folders {
		if names, ok := p.refPlace(folder); ok {
			if err := makeRefFolder(p.git, names); err != nil {
				return nil, err
			}
			continue
		}
		m, err := makeHeld(folder)
		if err != nil {
			return nil, err
		}
		if m != "" && !slices.Contains(unmade, m) {
			unmade = append(unmade, m)
		}
	}
	return unmade, nil
}

// makeRefFolder makes the folder names below the shared .git git, as MakeHold does, from the .git down, one folder at
// a time: each missing one made, empty and owner-only, and each opened only if it is still the folder it was found to
// be. A link on the way is a *RefPlaceError, never followed, and so is anything but a folder or a file; a file on the
// way leaves the rest unmade (a reftable repo's refs/heads), as MakeHold does.
func makeRefFolder(git string, names []string) error {
	dir, err := os.OpenRoot(filepath.FromSlash(git) + "/.")
	if err != nil {
		return err
	}
	defer func() { dir.Close() }()
	at := git
	for _, name := range names {
		at = path.Join(at, name)
		info, err := dir.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			if err = dir.Mkdir(name, 0o700); err == nil || errors.Is(err, fs.ErrExist) {
				info, err = dir.Lstat(name)
			}
		}
		switch {
		case err != nil:
			return err
		case info.Mode()&fs.ModeSymlink != 0:
			return &RefPlaceError{At: at, Ref: strings.TrimPrefix(at, git+"/"), Link: true}
		case info.Mode().IsRegular():
			return nil
		case !info.IsDir():
			return &RefPlaceError{At: at, Ref: strings.TrimPrefix(at, git+"/")}
		}
		// os.Root follows a link that stays inside it: one swapped in since the Lstat is refused by its own fd's file.
		sub, err := dir.OpenRoot(name + "/.")
		if err != nil {
			return err
		}
		if now, err := sub.Stat("."); err != nil || !os.SameFile(info, now) {
			sub.Close()
			return fmt.Errorf("%s was replaced while it was made", at)
		}
		dir.Close()
		dir = sub
	}
	return nil
}

// BranchName is whether name is a branch's name a Policy takes (Places.DefaultBranch, CheckoutBranch): one git would
// make, with nothing a path or a pattern reads. The core keeps no other (desktop/deskgitrows.go).
func BranchName(name string) bool { return branchName(name) }

// DeskGitConfig is env, a desk's environment on a Mac, with the settings every git the desk runs needs beside these
// rows, as command-line config in the environment (GIT_CONFIG_COUNT, git 2.31 and later), after any env has, which no
// repo's config overrides. A desk that takes them out only stops its own git; an env whose count git can't read is
// left as it is (git refuses to run with it anyway).
//   - rerere.enabled=false (B3): git runs rerere after each commit, merge and rebase once the repo's config turns it
//     on or rr-cache is there, and with MERGE_RR denied each would end "Unable to create MERGE_RR.lock" (exit 128,
//     the commit made all the same).
//   - remote.origin.followRemoteHEAD=never (RT126FIXREVIEW-GITROWS2's O1): from git 2.48 a fetch makes a missing
//     origin/HEAD (`create`, the default), and with origin/HEAD denied each fetch in a repo with none would say
//     "error: cannot lock ref 'refs/remotes/origin/HEAD'" (exit 0), which an agent may take for a failure to fix.
//     An older git doesn't know the setting and ignores it.
//   - remote.origin.tagOpt=--no-tags (FXGITROWS2R4): a fetch from origin takes the tags that point into what it
//     fetched as well, and with every tag denied (rowTags) each new one would end "cannot lock ref 'refs/tags/…'",
//     and the fetch with an error. The desk's fetch from another remote, or with --tags, still says so.
func DeskGitConfig(env []string) []string {
	n, at := 0, -1
	for i, kv := range env {
		if value, ok := strings.CutPrefix(kv, "GIT_CONFIG_COUNT="); ok {
			count, err := strconv.Atoi(value)
			if err != nil || count < 0 {
				return env
			}
			n, at = count, i
		}
	}
	out := slices.Clone(env)
	if at >= 0 {
		out = slices.Delete(out, at, at+1)
	}
	settings := [][2]string{{"rerere.enabled", "false"}, {"remote.origin.followRemoteHEAD", "never"}, {"remote.origin.tagOpt", "--no-tags"}}
	out = append(out, "GIT_CONFIG_COUNT="+strconv.Itoa(n+len(settings)))
	for i, kv := range settings {
		out = append(out, "GIT_CONFIG_KEY_"+strconv.Itoa(n+i)+"="+kv[0], "GIT_CONFIG_VALUE_"+strconv.Itoa(n+i)+"="+kv[1])
	}
	return out
}
