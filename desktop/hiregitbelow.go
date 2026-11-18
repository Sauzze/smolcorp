package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The git folders in a desk's place, on a Mac with its sandbox (FXHIREPROJECTSR5, FXHIREPROJECTSR6; the ruling of
// 18 Nov: a hire writes only its own project folder, one hire per repo). Another repo's git folder below a desk's
// place would give the desk that repo's hooks and settings, which the owner's own git runs outside any sandbox.
//   - A `.git` strictly below any place (an owner's nested clone, `clients/acme` under a place in a repo) is held by
//     the desk's Policy itself: nothing in it is written, nor one made there (the sandbox package's rowGitBelow). The
//     walk lists the ones there at the start, so the Policy holds the folders on the way to each by name too.
//   - A bare repo (no `.git` name: `git init --bare mirror`) no pattern can tell: the walk looks for one in every
//     place, and refuses the start, or the hire, where it finds one.
//   - A place in no repo that holds a repo (`~/Projects/clients`) is still refused at its hire and at each start, as
//     before: the office's picker lists the repos in it instead (hireProjectsIn).
//   - A repo moved in after a start (FXHIREPROJECTSR7, RT126FIXREVIEW-HIREPROJECTSR6 B1 (a)): no pattern follows a
//     folder renamed in, so each start keeps the list of .git folders it found (gitsListed), and the next refuses one
//     that wasn't there that the desk could have moved in (gitsSinceWords).
//   - A link below the place that leads to a repo a desk can change, or to a folder that holds one, or into a package
//     manager's folder that holds one (gitLinkWords), refuses the start too (FXHIREPROJECTSR8, B1).
//   - A package manager's own clones and mirrors (sandbox.ToolsGits: .build/repositories, …) don't stop a place, as a
//     hire's own `swift build` makes them; the ones there at a start are listed apart and held (N1).
//   - A gitlink in a hire's own worktree, staged or on its branch, at a path where git finds a repo makes the owner's
//     plain `git status` there run git in that repo, with its settings: it refuses the start (gitlinkWords, B2), the
//     path looked at as git spells it (FXHIREPROJECTSR9, B1).
//
// The walk is bounded and fails closed: past gitBelowMost folders, or at a folder it can't read, the hire or start is
// refused with words, and so is the walk of the links' targets. Left open: a link made and used within one session, a
// repo moved in and used within one, and a gitlink made and run over within one.

// gitBelowMost is how many folders gitBelow looks in, root among them, before it gives up: enough for an ordinary
// project, an installed node_modules, a Python venv or an unpacked SDK in it included (each a few thousand to a few
// tens of thousands of folders; the largest checkout measured for FXHIREPROJECTSR6 had about 32,000). A variable, for
// the tests.
var gitBelowMost = 50000

// gitBelowMostWords is gitBelowMost as the owner reads it, with a comma for each thousand.
func gitBelowMostWords() string {
	digits := strconv.Itoa(gitBelowMost)
	var s strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			s.WriteByte(',')
		}
		s.WriteRune(c)
	}
	return s.String()
}

// gitBelowError is a walk gitBelow couldn't finish: at, a folder it couldn't read, or "" for more than gitBelowMost.
type gitBelowError struct{ at string }

func (e *gitBelowError) Error() string {
	if e.at == "" {
		return "more than " + gitBelowMostWords() + " folders"
	}
	return "couldn't read " + e.at
}

// gitFound is what gitBelow found in a place.
type gitFound struct {
	// repo is the first folder strictly below the place, nearest first, that has a .git of its own; "" for none.
	repo string
	// gits are the .git entries strictly below the place (a folder, a file or a link), in the order met.
	gits []string
	// bare is the first folder at or below the place that is a bare repo; "" for none. The walk ends there.
	bare string
	// links are the links strictly below the place, outside a .git, in the order met (a .git that is a link too).
	links []string
	// tools are the git folders in a package manager's own folders below the place (sandbox.ToolsGits), in the order
	// met: each .git there (a folder, a file or a link), and each bare repo there, which the walk doesn't go into
	// (FXHIREPROJECTSR8, RT126FIXREVIEW-HIREPROJECTSR7 N1). They neither stop the place nor end the walk: the start's
	// Policy holds them (Places.ToolsGitBelow).
	tools []string
}

// toolsRepos are the repos of found.tools: the folder each .git is in, and each bare repo itself.
func (found gitFound) toolsRepos() []string {
	var repos []string
	for _, git := range found.tools {
		if strings.EqualFold(filepath.Base(git), ".git") {
			git = filepath.Dir(git)
		}
		repos = append(repos, git)
	}
	return repos
}

// gitBelow walks root, folder by folder, nearest first, for git's own folders: a .git (a folder, a file or a link) in
// a folder below root, and a bare repo at or below it, a folder with a HEAD (anything but a folder) and either a
// commondir or both objects and refs, as git itself tells a git folder. Names are compared in any case, as a Mac's
// disks match them. A .git is never looked in (its own HEAD, objects and refs are its repo's), nor is root's own: the
// desk's shared .git or its worktree's file, or, at a place that isn't its repo's top, one its Policy holds. A package
// manager's own folder (sandbox.ToolsGits) is looked in too, but what it finds there is listed apart (tools), and ends
// nothing. Links are listed, never followed. The walk ends at a bare repo, and with first at the first .git too, both
// outside a package manager's folders; at most gitBelowMost folders are looked in, and any it can't read ends the walk
// with an error.
func gitBelow(root string, first bool) (gitFound, error) {
	var found gitFound
	type folderIn struct {
		at    string
		tools bool // in a package manager's own folder
	}
	queue := []folderIn{{at: root}}
	for looked := 0; len(queue) > 0; looked++ {
		if looked == gitBelowMost {
			return found, &gitBelowError{}
		}
		folder, inTools := queue[0].at, queue[0].tools
		queue = queue[1:]
		entries, err := os.ReadDir(folder)
		if err != nil {
			return found, &gitBelowError{at: folder}
		}
		var head, commondir, objects, refs bool
		for _, e := range entries {
			switch name := e.Name(); {
			case strings.EqualFold(name, "HEAD"):
				head = !e.IsDir()
			case strings.EqualFold(name, "commondir"):
				commondir = true
			case strings.EqualFold(name, "objects"):
				objects = true
			case strings.EqualFold(name, "refs"):
				refs = true
			}
		}
		if head && (commondir || objects && refs) {
			if inTools {
				found.tools = append(found.tools, folder) // a mirror: not gone into
				continue
			}
			found.bare = folder
			return found, nil
		}
		for _, e := range entries {
			name := e.Name()
			if e.Type()&fs.ModeSymlink != 0 {
				found.links = append(found.links, filepath.Join(folder, name))
			}
			if strings.EqualFold(name, ".git") {
				switch {
				case inTools:
					found.tools = append(found.tools, filepath.Join(folder, name))
				case folder != root:
					if found.repo == "" {
						found.repo = folder
					}
					found.gits = append(found.gits, filepath.Join(folder, name))
					if first {
						return found, nil
					}
				}
				continue
			}
			if at := filepath.Join(folder, name); e.IsDir() { // a link is never a folder here
				queue = append(queue, folderIn{at: at, tools: inTools || toolsGits(root, at)})
			}
		}
	}
	return found, nil
}

// toolsGits is whether folder, below root, is a package manager's own folder of clones and mirrors, or in one
// (sandbox.ToolsGits): only a folder whose name ends one of their patterns is matched against them.
func toolsGits(root, folder string) bool {
	switch strings.ToLower(filepath.Base(folder)) {
	case "repositories", "checkouts", "git", "src":
		return sandbox.InToolsGits(runtime.GOOS, root, folder)
	}
	return false
}

// gitPlaceWords say why a desk whose place is root can't work there in its sandbox on a Mac, or "" when it can, with
// what the walk found: the .git entries strictly below root for its Policy to hold (Places.GitBelow), and the links
// for gitLinkWords. Any place is refused for a bare repo at or below it, or a walk that can't finish; a place in no
// repo (inRepo false) for any repo below it too. At its hire it is told what to do instead; at a start (started), a
// desk hired so before, to be fired and hired again, but a bare repo found at a later start (startedBefore: one this
// desk started at, with the list kept, gitsListed) wasn't there at the last, so it's to be removed (FXHIREPROJECTSR7,
// O2), and a folder the walk can't read is to be made readable or moved out (N2).
func gitPlaceWords(root string, inRepo, started, startedBefore bool) (string, gitFound) {
	if runtime.GOOS != "darwin" {
		return "", gitFound{}
	}
	root = physical(root)
	shown := shownPath(root)
	found, err := gitBelow(root, !inRepo)
	switch {
	case err == nil && found.bare != "":
		what := shown + " holds a bare git repository, " + shownPath(found.bare)
		if found.bare == root {
			what = shown + " is a bare git repository"
		}
		if started && startedBefore && found.bare != root {
			return "This hire was set to work in " + shown + ", but " + what + ", that wasn't there at its last start, and your own git runs its hooks, so it didn't start. Remove " + shownPath(found.bare) + ", or move it out of " + shown + ", then start it again.", found
		}
		if started {
			return "This hire was set to work in " + shown + ", but " + what + ", whose hooks your own git runs, and on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into a folder that holds none.", found
		}
		return "On this Mac, each hire works in one project of its own, and " + what + ", whose hooks your own git runs. Hire it into a folder that holds none.", found
	case err == nil && found.repo != "" && !inRepo:
		if started {
			return "This hire was set to work in " + shown + ", which holds a git repository, " + shownPath(found.repo) + ", but on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into that repository itself.", found
		}
		return "On this Mac, each hire works in one project of its own, and " + shown + " isn't one: it holds a git repository, " + shownPath(found.repo) + ". Hire it into that repository itself.", found
	case err == nil:
		return "", found
	}
	what := "holds no git repository"
	if inRepo {
		what = "holds no bare git repository"
	}
	if e, _ := err.(*gitBelowError); e != nil && e.at != "" {
		unread := shownPath(e.at)
		if started {
			return "This hire was set to work in " + shown + ", but Smolcorp can't read " + unread + ", so it couldn't make sure " + shown + " " + what + ", and it didn't start. Make " + unread + " readable, or move it out of " + shown + ", then start it again.", found
		}
		return "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure " + shown + " " + what + ": it can't read " + unread + ". Make " + unread + " readable, or move it out of " + shown + ", then hire it again.", found
	}
	why := shown + " has more than " + gitBelowMostWords() + " folders, more than Smolcorp looks through"
	instead := "the repository you want it to work on, or into a smaller folder that holds none"
	if inRepo {
		instead = "a project with fewer folders"
	}
	if started {
		return "This hire was set to work in " + shown + ", but Smolcorp couldn't make sure it " + what + " (" + why + "), and on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into " + instead + ".", found
	}
	return "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure " + shown + " " + what + ": " + why + ". Hire it into " + instead + ".", found
}

// gitsListedName is the file, in a desk's own folder in the support folder, that keeps the .git folders its last
// start found below its place (FXHIREPROJECTSR7): only the core writes there (the support folder's row).
const gitsListedName = "gits-below.json"

// gitsList is what gitsListedName holds: the place the walk looked in, as it was resolved, and the .git entries it
// found below it.
type gitsList struct {
	Root string   `json:"root"`
	Gits []string `json:"gits"`
}

// gitsListed is the list desk id's last start kept for root, and whether there is one: none before this desk's first
// start since the list was kept (a desk hired before starts once without it), for another place, or one that can't be
// read (logged; only the core writes it).
func gitsListed(id, root string) ([]string, bool) {
	if !oneName(id) {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(hireFolder(supportDir, id), gitsListedName))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("sandbox: desk %s's list of git folders: %v", id, err)
		}
		return nil, false
	}
	var list gitsList
	if err := json.Unmarshal(data, &list); err != nil {
		log.Printf("sandbox: desk %s's list of git folders: %v", id, err)
		return nil, false
	}
	return list.Gits, list.Root == root
}

// listGits keeps gits, the .git entries a start of desk id found below root, for its next start: written whole beside
// it, then renamed onto it.
func listGits(id, root string, gits []string) error {
	if !oneName(id) {
		return errors.New("a desk's id isn't one name")
	}
	folder := hireFolder(supportDir, id)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(gitsList{Root: root, Gits: append([]string{}, gits...)})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(folder, "."+gitsListedName+".*")
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(file.Name(), filepath.Join(folder, gitsListedName))
	}
	if err != nil {
		_ = os.Remove(file.Name())
	}
	return err
}

// gitsSinceWords say why a desk whose place is root doesn't start, or "": a .git below root that its last start didn't
// list (listed) and that the desk could have moved in under the Policy it ran under then (before, made with listed
// held; an error making it counts as could). It could where any folder on the way from the repo to root, the repo's
// own folder included, is one it could rename (Move): a folder held by name (on the way to a .git listed then) it
// couldn't. So a repo the desk made outside and renamed in (RT126FIXREVIEW-HIREPROJECTSR6 B1 (a)) stops its next start,
// and so does a repo the owner added during its session (B1 (b)), which the desk could have moved out, changed and
// moved back: the core can't tell who made it. It can't catch a repo moved in and used within one session.
func gitsSinceWords(before *sandbox.Policy, beforeErr error, root string, listed, gits []string) string {
	for _, git := range gits {
		if slices.ContainsFunc(listed, func(l string) bool {
			return sandbox.Inside(runtime.GOOS, git, l) && sandbox.Inside(runtime.GOOS, l, git)
		}) {
			continue
		}
		could := beforeErr != nil || before == nil
		for folder := filepath.Dir(git); !could && folder != root && sandbox.Inside(runtime.GOOS, folder, root); folder = filepath.Dir(folder) {
			if v, _ := before.Decide(folder, sandbox.Move); v != sandbox.Deny {
				could = true
			}
		}
		if could {
			repo, shown := shownPath(filepath.Dir(git)), shownPath(root)
			return "This hire was set to work in " + shown + ", but a git repository turned up there since its last start, " + repo + ", and the hire could have put it there with hooks your own git runs, so it didn't start. Remove " + repo + ", or move it out of " + shown + ", then start it again. If you added it yourself and want it there, check its hooks first, then fire this hire and hire it again."
		}
	}
	return ""
}

// gitLinkWords say why a desk with Policy pol at places pl doesn't start, or "": a link below its place (found.links,
// as the walk found them) by which your git reaches a repo a desk could change (FXHIREPROJECTSR7,
// RT126FIXREVIEW-HIREPROJECTSR6 B1 (a)'s link; FXHIREPROJECTSR8, RT126FIXREVIEW-HIREPROJECTSR7 B1): your git, run in the
// link or in a folder below it, works on that repo, with its hooks and settings. Each link is followed whole, then:
//   - one that leads outside the place: the repo git finds from where it leads, looking up, as git does (repoFrom),
//     and every repo below there, a .git or a bare repo, walked as gitBelow walks, links in it followed the same way;
//   - one that leads into the place: the place's walk saw what's there, but a repo in a package manager's folder
//     (found.tools) at or below where it leads, or holding it, refuses the start, as any desk could have made it.
//
// A repo outside refuses the start by the compare's rule (gitsSinceWords): when this desk can write its git folders
// (sandbox's DeskWrites, the "leads where?" test of the git rows' links), or could have moved it there (any folder on
// the way from the repo to where the link leads, the repo's own included, is one it could rename), or when it is in
// any other desk's place (the registry's open hires). A link that leads nowhere now, to a file, to a folder that holds
// no repo, or to the desk's own repo, whose shared .git its rows hold, passes. The walk of the links' targets is
// bounded, all of them together, by gitBelowMost folders, and fails closed: past that, or at a folder it can't read, the
// start is refused too. Left open: a link that leads to a folder a desk then makes a repo in, within one session (the
// next start looks again).
func gitLinkWords(pol *sandbox.Policy, pl sandbox.Places, found gitFound, id string) string {
	if pol == nil || len(found.links) == 0 {
		return ""
	}
	var places []string
	for _, h := range readRegistry(registryPath(), false).hires {
		dir := hireDir(h)
		if h.Role == gmRole && h.Repo == "" && h.Worktree == "" && oneName(h.ID) {
			dir = sandbox.Workspace(supportDir, h.ID) // a GM in a folder of projects works in its workspace (gmWorkspace)
		}
		if h.ID != id && isOpen(h) && dir != "" {
			places = append(places, physical(dir))
		}
	}
	inside := func(path, base string) bool { return sandbox.Inside(runtime.GOOS, path, base) }
	// changes is whether a desk could change the repo at top (its git folders, gits), met from where a link leads.
	changes := func(top string, gits []string, from string) bool {
		if pl.GitDir != "" && slices.ContainsFunc(gits, func(g string) bool { return inside(g, pl.GitDir) }) {
			return false // the desk's own repo: its shared .git's rows hold it
		}
		if slices.ContainsFunc(places, func(place string) bool { return inside(top, place) || inside(from, place) }) {
			return true
		}
		for _, git := range gits {
			if writes, err := pol.DeskWrites(filepath.ToSlash(git)); err != nil || writes {
				return true
			}
		}
		for folder := top; ; folder = filepath.Dir(folder) {
			if v, _ := pol.Decide(folder, sandbox.Move); v != sandbox.Deny {
				return true
			}
			if folder == from || !inside(folder, from) || filepath.Dir(folder) == folder {
				return false
			}
		}
	}
	root, tools := pl.Root, found.toolsRepos()
	type leading struct{ link, at string } // a link in the place, as the owner meets it, and a link on the way
	queue := make([]leading, 0, len(found.links))
	for _, link := range found.links {
		queue = append(queue, leading{link, link})
	}
	walked := map[string]bool{}
	left := gitBelowMost
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		// A link that leads nowhere (nothing there, or a file on the way) leads git nowhere either. Any other answer
		// (a folder on the way it can't read, a loop) refuses the start: a desk could make one for a start to pass,
		// then undo it (FXHIREPROJECTSR9, RT126FIXREVIEW-HIREPROJECTSR8 O2).
		leads, err := filepath.EvalSymlinks(next.at)
		var info fs.FileInfo
		if err == nil {
			info, err = os.Stat(leads)
		}
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if err != nil {
			return "This hire was set to work in " + shownPath(pl.Root) + ", but Smolcorp can't follow the link " + shownPath(next.at) + " to where it leads, so it couldn't make sure no git repository a hire can change is there, and it didn't start. Make where it leads readable, or remove the link, then start it again."
		}
		if !info.IsDir() {
			continue
		}
		if inside(leads, root) {
			if i := slices.IndexFunc(tools, func(repo string) bool { return inside(repo, leads) || inside(leads, repo) }); i >= 0 {
				return linkWords(pl.Root, next.link, tools[i], !inside(leads, tools[i]))
			}
			continue
		}
		if top, gits := repoFrom(leads); top != "" && changes(top, gits, leads) {
			return linkWords(pl.Root, next.link, top, false)
		}
		folders := []string{leads}
		for len(folders) > 0 {
			folder := folders[0]
			folders = folders[1:]
			if walked[folder] || inside(folder, root) {
				continue // looked in already, or the place itself, whose own walk is the start's
			}
			walked[folder] = true
			if left == 0 {
				return "This hire was set to work in " + shownPath(pl.Root) + ", but its links lead to more than " + gitBelowMostWords() + " folders, more than Smolcorp looks through to make sure none holds a git repository a hire can change, so it didn't start. Remove the link " + shownPath(next.link) + ", or the links you don't need there, then start it again."
			}
			left--
			entries, err := os.ReadDir(folder)
			if err != nil {
				unread := shownPath(folder)
				return "This hire was set to work in " + shownPath(pl.Root) + ", but Smolcorp can't read " + unread + ", where the link " + shownPath(next.link) + " leads, so it couldn't make sure no git repository a hire can change is there, and it didn't start. Make " + unread + " readable, or remove the link, then start it again."
			}
			if folder != leads && bareRepo(folder) {
				if changes(folder, gitFoldersOf(folder), leads) {
					return linkWords(pl.Root, next.link, folder, true)
				}
				continue // a bare repo's own folders are its own
			}
			for _, e := range entries {
				at := filepath.Join(folder, e.Name())
				switch {
				case strings.EqualFold(e.Name(), ".git"):
					if folder != leads && changes(folder, gitFoldersOf(at), leads) {
						return linkWords(pl.Root, next.link, folder, true)
					}
				case e.Type()&fs.ModeSymlink != 0:
					queue = append(queue, leading{next.link, at})
				case e.IsDir():
					folders = append(folders, at)
				}
			}
		}
	}
	return ""
}

// linkWords say that a desk at root didn't start for link, which leads to repo, a git repository a hire can change,
// or to a folder that holds it (holds).
func linkWords(root, link, repo string, holds bool) string {
	shown := shownPath(link)
	if holds {
		return "This hire was set to work in " + shownPath(root) + ", but " + shown + " is a link to a folder that holds a git repository a hire can change, " + shownPath(repo) + ", and your own git runs that repository's hooks when it works there through the link, so it didn't start. Remove the link " + shown + ", then start it again."
	}
	return "This hire was set to work in " + shownPath(root) + ", but " + shown + " is a link to a git repository a hire can change, " + shownPath(repo) + ", and your own git runs that repository's hooks when it works in " + shown + ", so it didn't start. Remove the link " + shown + ", then start it again."
}

// gitlinkWords say why a desk working in a worktree of its own at root, a worktree of the owner's checkout, doesn't
// start, or "": a gitlink (a submodule's entry, mode 160000) staged in its worktree, or committed on its branch, at a
// path where git finds a repo (FXHIREPROJECTSR8, RT126FIXREVIEW-HIREPROJECTSR7 B2). With one there, the owner's plain
// `git status` or `git diff` in the worktree runs git in that repo too, with that repo's settings (its core.fsmonitor),
// and a repo in a package manager's folder is any desk's to make.
//
// It works from git's side, not the walk's (FXHIREPROJECTSR9, RT126FIXREVIEW-HIREPROJECTSR8 B1): git takes a gitlink's
// path as the index spells it, and a Mac's disk finds a folder by it in any case and either Unicode form (`EVIL` over
// `evil`, NFD over NFC). So every gitlink is listed, with no pathspec: the index's (`ls-files`) and HEAD's (`ls-tree -r
// HEAD`). Each path is then looked at as git looks at it: by its own spelling, one folder at a time,
// following no link (git doesn't go through one: a link on the way leads where the links' check looked, gitLinkWords),
// for a .git in it or a bare repo at it. The walk says only whether there is any repo below root at all; with none,
// no gitlink can lead to one, and git isn't asked.
//
// A repo there passes only as the project's own submodule: a gitlink at that same path, byte for byte, in the owner's
// checkout's HEAD (the commit prepareHire made the worktree from), whose .git is a file naming a git folder in the
// worktree's own registration's modules (registration, its worktrees/<id>), which no desk writes (the sandbox
// package's "a worktree's submodules" row) and which the core makes at the hire (initHireSubmodules). What git can't
// answer refuses the start (fails closed). git runs as the core runs it in a desk's tree (runGit): neither listing
// goes into a submodule or reads a working file. Left open: a gitlink made in a session and run over by the owner's git
// within it (the next start looks).
func gitlinkWords(git gitRunner, root, checkout, registration string, found gitFound) string {
	if checkout == "" || len(found.gits) == 0 && len(found.tools) == 0 {
		return ""
	}
	if git == nil {
		git = runGit
	}
	cant := "This hire was set to work in " + shownPath(root) + ", which holds a git repository of its own, and Smolcorp couldn't ask git whether the hire made it a submodule of your project, so it didn't start. Try again in a moment; if it keeps happening, delete the git repositories in " + shownPath(root) + " that your project doesn't have, then start it again."
	status, out, ok := git(root, "rev-parse", "--show-toplevel")
	if !ok || status != 0 || !filepath.IsAbs(strings.TrimSpace(out)) {
		return cant
	}
	top := physical(strings.TrimSpace(out))
	staged, ok := gitlinksListed(git, top, gitlinksOfIndex()...)
	if !ok {
		return cant
	}
	committed, ok := gitlinksListed(git, top, gitlinksOfTree("HEAD")...)
	if !ok {
		return cant
	}
	paths := slices.Clone(staged)
	for _, rel := range committed {
		if !slices.Contains(paths, rel) {
			paths = append(paths, rel)
		}
	}
	var real []string // the owner's checkout's gitlinks, asked once there is a repo to look at
	looked := false
	for _, rel := range paths {
		if !gitFindsRepoAt(top, rel) {
			continue
		}
		if !looked {
			if real, ok = gitlinksListed(git, physical(checkout), gitlinksOfTree("HEAD")...); !ok {
				return cant
			}
			looked = true
		}
		if slices.Contains(real, rel) && submoduleOfRegistration(filepath.Join(top, filepath.FromSlash(rel)), registration) {
			continue // a submodule the project has, its git folder the core's
		}
		shown, tree := shownPath(filepath.Join(top, filepath.FromSlash(rel))), shownPath(top)
		did, then := "staged "+shown+" as a submodule", "To unstage the submodule as well, run `git rm --cached "+rel+"` in "+tree+" afterwards."
		if slices.Contains(committed, rel) {
			did, then = "committed "+shown+" on its branch as a submodule", "To take the submodule off its branch as well, run `git rm --cached "+rel+"` in "+tree+" afterwards, and commit."
		}
		return "This hire was set to work in " + shownPath(root) + ", but it " + did + ", and " + shown + " holds a git repository your project doesn't have: your own git, run in " + tree + ", would run in that repository too, with its settings. So it didn't start. Delete the folder " + shown + ", then start it again. " + then
	}
	return ""
}

// gitlinksListed is the gitlinks' paths (mode 160000) in what git lists in the repo at top with args (gitlinksOfIndex,
// gitlinksOfTree: each entry "<mode>[ <object> <stage or type>]\t<path>\0"), as git spells them, in its order; ok false
// when git couldn't list them (an answer past gitOutputCap among them: it fails closed). ls-tree's --format writes a
// path with a byte past ASCII, a quote or a backslash C-quoted, even with -z ("e\314\201vil"): it's unquoted back
// to git's own bytes, and one that doesn't unquote fails the listing.
func gitlinksListed(git gitRunner, top string, args ...string) ([]string, bool) {
	status, out, ok := git(top, args...)
	if !ok || status != 0 {
		return nil, false
	}
	quoted := args[0] == "ls-tree" && slices.Contains(args, gitlinkFormat)
	var paths []string
	for _, entry := range strings.Split(out, "\x00") {
		info, path, found := strings.Cut(entry, "\t")
		if fields := strings.Fields(info); !found || path == "" || len(fields) == 0 || fields[0] != "160000" {
			continue
		}
		if quoted && strings.HasPrefix(path, `"`) {
			unquoted, err := strconv.Unquote(path)
			if err != nil {
				return nil, false
			}
			path = unquoted
		}
		paths = append(paths, path)
	}
	return paths, true
}

// gitlinkFormat is each entry as gitlinksListed reads it, its mode and path alone (git 2.38 and later), so a project's
// whole list of files fits in git's answer (gitOutputCap) about twice over what `ls-files -s` or a plain ls-tree takes.
const gitlinkFormat = "--format=%(objectmode)%x09%(path)"

// gitlinksOfIndex is the listing of every entry in a worktree's index, with no pathspec.
func gitlinksOfIndex() []string {
	if gitVersionAtLeast(gitHere().version, 2, 38) {
		return []string{"ls-files", "-z", gitlinkFormat}
	}
	return []string{"ls-files", "-s", "-z"}
}

// gitlinksOfTree is the listing of every entry in tree, with no pathspec.
func gitlinksOfTree(tree string) []string {
	if gitVersionAtLeast(gitHere().version, 2, 38) {
		return []string{"ls-tree", "-r", "-z", gitlinkFormat, tree}
	}
	return []string{"ls-tree", "-r", "-z", tree}
}

// gitFindsRepoAt is whether git, at the top of a checkout, finds a repo at a gitlink's path rel (with /), as git looks:
// each folder by rel's own spelling, from top down, with no link followed on the way or at it (git takes a path
// through a link for one that isn't there), then a .git in it (a folder, a file or a link), or the folder a bare repo.
// A name the disk doesn't have, or that isn't a folder, is no repo.
func gitFindsRepoAt(top, rel string) bool {
	at := top
	for _, name := range strings.Split(rel, "/") {
		if name == "" || name == "." || name == ".." {
			return false // git takes no such path into its index
		}
		at = filepath.Join(at, name)
		info, err := os.Lstat(at)
		if err != nil || !info.IsDir() {
			return false
		}
	}
	if _, err := os.Lstat(filepath.Join(at, ".git")); err == nil {
		return true
	}
	return bareRepo(at)
}

// submoduleOfRegistration is whether the submodule checked out at folder is the one the core made in a worktree's
// registration (worktrees/<id>, links resolved): its .git a file whose gitdir names a folder in the registration's
// modules, and no commondir there naming another.
func submoduleOfRegistration(folder, registration string) bool {
	info, err := os.Lstat(filepath.Join(folder, ".git"))
	if registration == "" || err != nil || !info.Mode().IsRegular() {
		return false
	}
	gits := gitFoldersOf(filepath.Join(folder, ".git"))
	return len(gits) == 2 && sandbox.Inside(runtime.GOOS, gits[1], filepath.Join(registration, "modules")) &&
		!sandbox.Inside(runtime.GOOS, filepath.Join(registration, "modules"), gits[1])
}

// repoFrom is the repo git finds from folder, a resolved path, looking up as git does: the first folder, folder
// itself included, with a .git in it, or that is a bare repo (or a .git folder: git finds its repo inside it). It is
// that folder, and the git folders it reads hooks and settings from, resolved: its .git, and for a .git file, or a
// folder with a commondir, the folders they name. "" for none.
func repoFrom(folder string) (string, []string) {
	for at := folder; ; at = filepath.Dir(at) {
		var gits []string
		if _, err := os.Lstat(filepath.Join(at, ".git")); err == nil {
			gits = gitFoldersOf(filepath.Join(at, ".git"))
		} else if bareRepo(at) {
			gits = gitFoldersOf(at)
		}
		if gits != nil {
			return at, gits
		}
		if filepath.Dir(at) == at {
			return "", nil
		}
	}
}

// bareRepo is whether folder is a git folder by git's own test (gitBelow's): a HEAD that isn't a folder, and either a
// commondir or both objects and refs.
func bareRepo(folder string) bool {
	head, err := os.Lstat(filepath.Join(folder, "HEAD"))
	if err != nil || head.IsDir() {
		return false
	}
	there := func(name string) bool { _, err := os.Lstat(filepath.Join(folder, name)); return err == nil }
	return there("commondir") || there("objects") && there("refs")
}

// gitFoldersOf is git, a .git or a git folder, resolved, with what it names: for a .git file, the folder its
// `gitdir:` line names; for a git folder with a commondir, the folder that names. Each is followed as far as it is
// there; what can't be followed is left out (git can't read it either).
func gitFoldersOf(git string) []string {
	real, err := filepath.EvalSymlinks(git)
	if err != nil {
		return []string{git}
	}
	out := []string{real}
	named := func(file, prefix string) string {
		data, err := os.ReadFile(file)
		if err != nil {
			return ""
		}
		text, ok := strings.CutPrefix(strings.TrimSpace(string(data)), prefix)
		if !ok || text == "" {
			return ""
		}
		text = strings.TrimSpace(text)
		if !filepath.IsAbs(text) {
			text = filepath.Join(filepath.Dir(file), text)
		}
		if resolved, err := filepath.EvalSymlinks(text); err == nil {
			return resolved
		}
		return ""
	}
	if info, err := os.Stat(real); err == nil && !info.IsDir() {
		if dir := named(real, "gitdir:"); dir != "" {
			out = append(out, dir)
			real = dir
		}
	}
	if common := named(filepath.Join(real, "commondir"), ""); common != "" {
		out = append(out, common)
	}
	return out
}

// shownPath is path as the owner's words name it: ~/… inside the home folder, however that is spelled (the walk's
// paths are resolved: /private/var/… for /var/…), else whole.
func shownPath(path string) string {
	if home := physical(homeDir); home != "" && home != "/" {
		if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
			return "~" + string(filepath.Separator) + rest
		}
	}
	return tildePath(path)
}
