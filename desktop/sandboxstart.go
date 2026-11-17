package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// Start-time checks for a desk in the sandbox (SA-X12, SANDBOX-PLAN §A5.3): what's left of SB10 once B had changed
// every key. Two checks, both run by the core before the desk's first process, after the Policy is made:
//
//   - Links. A sandbox checks a write by its path, and a hard link is the same file under another path: one made in a
//     place a desk may write, to a file the Policy keeps from it, hands the desk that file. A sandboxed desk can't make
//     one (the profile denies it), but a process outside any sandbox could have, before the update or while a desk ran
//     unsandboxed. So every file the Policy denies a desk is looked at, and one with more than one name is replaced or
//     the desk isn't started (checkLinks).
//   - Old git plants. The shared .git's config and hooks are write-denied from the first sandboxed start, but one a
//     desk wrote before is still there, and the owner's git runs it. The repo's common config and its submodules'
//     are read with gitstate's key classifier, and its hooks that aren't git's samples are listed (findOldGitPlants),
//     for Setup to show the owner with a "Remove" button (removeOldGitPlant). Smolcorp removes nothing by itself.

// linkCheckMost is how many entries the link check looks at, at most, in one start. Smolcorp's own folders and the
// owner's small ones it looks in hold a few thousand; past this the start is refused, as a check left half done.
var linkCheckMost = 100_000

// linkCheckSkipped are the Policy's rows over whole folders the link check doesn't walk, and why: each is a code tree
// or an install, too big to look through at every start. A link into one needed a process outside any sandbox, which
// could have written the file itself. TestEveryDenyRowIsLookedAtOrSkippedWithAReason holds every other row to a look.
// A file the check would replace that has a name in one (or anywhere else it doesn't look) refuses the start: the
// name it can't see may be a file desks can't read (X12LINK1).
var linkCheckSkipped = map[string]string{
	"other desks' places":                  "other desks' worktrees: code trees, too big to walk at each start",
	"the owner's checkout":                 "the owner's working tree: a code tree, too big to walk at each start",
	"other desks' places: the GM's folder": "the GM's working folder: a code tree, too big to walk at each start",
	"programs that run outside":            "whole install roots (Homebrew's, npm's, Claude Code's): too big to walk at each start",
}

// accountFoldersSkipped are the folders in an account folder the link check passes over: its transcripts, its
// plugins and its file history, each of which can hold tens of thousands of files. A submodule's objects are passed
// over too: the repo's own are the desk's to write anyway. A transcript is read-denied, so a file the check would
// replace with a second name here refuses the start, as one with a name in a linkCheckSkipped place does.
var accountFoldersSkipped = []string{"projects", "plugins", "file-history"}

// linkCheckError is a start refused by the link check: the words for the owner.
type linkCheckError struct{ words string }

func (e *linkCheckError) Error() string { return e.words }

// checkLinks is the link check for a desk at pl, whose Policy is pol. Every regular file pol denies the desk reading
// or writing, in the places its rows name (but those linkCheckSkipped names), is looked at; one with more than one
// name is:
//   - replaced by a fresh copy, put in its place by a rename, when it is Smolcorp's own and the desk may read it but
//     not write it (the guard file, the night shift's jobs, the desk's brief, fence, profile and Claude settings): the
//     other name keeps a dead copy, and nothing of the owner's changes;
//   - else the reason the desk isn't started: a key's other name still reads the key, and the owner's own file (a
//     plant, a fence, the shared .git's config or hooks) is the owner's to make one again.
//
// It looks at everything before it changes anything: a start it refuses replaces nothing. A file the desk can't read
// that has a second name refuses the start wherever the walk meets it, and so does a file it would replace that is
// one of those under another name (a readable name for a key is a leak a fresh copy can't mend: the copy would hold
// the key's bytes). A file it would replace is replaced only when the walk met every one of its names (X12LINK1): a
// name it didn't meet (a transcript, another desk's worktree, the desk's own place) may be a file desks can't read,
// so a link count that isn't the names met refuses the start. It fails closed: a file or folder it can't look at (but
// one only another user can list), or more than linkCheckMost entries, refuses the start. It says which files it
// replaced.
func checkLinks(pol *sandbox.Policy, pl sandbox.Places) (replaced []string, err error) {
	looks, skip := linkLooks(pol, pl)
	budget := linkCheckMost
	var toReplace, unreadable, met []linked
	var refusal error
	for _, look := range looks {
		err := walkLook(look.path, look.depth, skip, &budget, func(at string, info fs.FileInfo) error {
			// Every name the walk meets, denied or not, to count a file's names by. Elsewhere one name only, but
			// Windows tells a file's links only of an open file.
			if runtime.GOOS == "windows" || fileLinks(nil, info) > 1 {
				met = append(met, linked{at: at, info: info})
			}
			readV, _ := pol.Decide(at, sandbox.Read)
			writeV, _ := pol.Decide(at, sandbox.Write)
			if readV != sandbox.Deny && writeV != sandbox.Deny {
				return nil
			}
			links, err := linksOf(at, info)
			if err != nil {
				return couldntCheck(at, err)
			}
			switch {
			case links <= 1:
			case readV == sandbox.Deny:
				unreadable = append(unreadable, linked{at, info, links})
			case writeV == sandbox.Deny && inside(pl.Support, at):
				toReplace = append(toReplace, linked{at, info, links})
			case refusal == nil:
				refusal = linkedRefusal(at)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for _, r := range toReplace {
		for _, u := range unreadable {
			if os.SameFile(r.info, u.info) {
				return nil, &linkCheckError{fmt.Sprintf("%s is a second name of %s, which desks can't read, so a desk could read it through that name, and the desk didn't start. Remove the second name and put the file back as it was, then start it again.", r.at, u.at)}
			}
		}
	}
	if len(unreadable) > 0 {
		return nil, linkedRefusal(unreadable[0].at)
	}
	if refusal != nil {
		return nil, refusal
	}
	for _, r := range toReplace {
		names, err := namesMet(r, met)
		if err != nil {
			return nil, couldntCheck(r.at, err)
		}
		if names != r.links {
			return nil, &linkCheckError{fmt.Sprintf("%s has a second name in a place Smolcorp doesn't look in at a start (%d names, %d of them where it looks), which may be a file desks can't read, so the desk didn't start. Remove the other name (%s), then start it again.", r.at, r.links, names, otherNames(r.at))}
		}
		if most := freshCopyMost(pl, r.at); r.info.Size() > most {
			return nil, &linkCheckError{fmt.Sprintf("%s has a second name and is bigger than Smolcorp ever makes it (%d bytes, at most %d), so Smolcorp didn't copy it, and the desk didn't start. Remove the other name (%s) and put the file back as it was, then start it again.", r.at, r.info.Size(), most, otherNames(r.at))}
		}
	}
	for i, r := range toReplace {
		// One already replaced was one of this file's names: it has one fewer now.
		links := r.links
		for _, done := range toReplace[:i] {
			if os.SameFile(done.info, r.info) {
				links--
			}
		}
		if err := freshCopy(r.at, r.info, links); err != nil {
			return replaced, couldntCheck(r.at, err)
		}
		replaced = append(replaced, r.at)
	}
	return replaced, nil
}

// linked is a regular file the link check met, and how many names it has (0 when it wasn't asked).
type linked struct {
	at    string
	info  fs.FileInfo
	links uint64
}

// namesMet is how many of r's names are among met, each counted once: the same folder looked at under two spellings
// (through a link above it, or in another case) is one folder, so a name met twice can't stand in for one not met.
// It errs toward fewer: a name it can't place refuses the start. Size and time are compared first, so Windows opens
// only the files that may be r.
func namesMet(r linked, met []linked) (uint64, error) {
	type name struct {
		folder fs.FileInfo
		base   string
	}
	var names []name
	for _, m := range met {
		if m.info.Size() != r.info.Size() || !m.info.ModTime().Equal(r.info.ModTime()) || !os.SameFile(m.info, r.info) {
			continue
		}
		folder, err := os.Stat(filepath.Dir(m.at))
		if err != nil {
			return 0, err
		}
		base := filepath.Base(m.at)
		if !slices.ContainsFunc(names, func(n name) bool { return strings.EqualFold(n.base, base) && os.SameFile(n.folder, folder) }) {
			names = append(names, name{folder, base})
		}
	}
	return uint64(len(names)), nil
}

// linkLook is one place the link check looks in: a file, or a folder to depth levels below it (-1, all of them).
type linkLook struct {
	path  string
	depth int
}

// linkLooks are the places pol's rows name, each folder once, and the folders a walk passes over.
func linkLooks(pol *sandbox.Policy, pl sandbox.Places) (looks []linkLook, skip func(string) bool) {
	for _, r := range pol.Rules() {
		if r.Read != sandbox.Deny && r.Write != sandbox.Deny {
			continue // an allow, or a row on a folder above a denied place (Move and Link only)
		}
		if _, skipped := linkCheckSkipped[r.Row]; skipped {
			continue
		}
		tail := 0
		if r.Tail != "" {
			tail = strings.Count(r.Tail, "/") + 1
		}
		switch {
		case r.Path == "": // a row for anywhere: the .claude folders where the desk and the owner work
			for _, root := range []string{pl.Root, pl.Checkout} {
				if root != "" {
					looks = append(looks, linkLook{filepath.Join(root, ".claude"), 1})
				}
			}
		case r.Below || strings.Contains(r.Tail, "**"):
			looks = append(looks, linkLook{filepath.FromSlash(r.Path), -1})
		default:
			looks = append(looks, linkLook{filepath.FromSlash(r.Path), tail})
		}
	}
	// Each place once, to its deepest look; a folder walked whole covers every look inside it.
	deepest := map[string]int{}
	for _, l := range looks {
		if d, ok := deepest[l.path]; !ok || d >= 0 && (l.depth < 0 || l.depth > d) {
			deepest[l.path] = l.depth
		}
	}
	places := slices.Sorted(maps.Keys(deepest))
	var kept []linkLook
	for _, at := range places {
		if !slices.ContainsFunc(places, func(folder string) bool { return folder != at && deepest[folder] < 0 && inside(folder, at) }) {
			kept = append(kept, linkLook{at, deepest[at]})
		}
	}
	var passed []string
	for _, account := range append([]string{filepath.Join(pl.Home, ".claude")}, pl.Accounts...) {
		for _, name := range accountFoldersSkipped {
			passed = append(passed, filepath.Join(account, name))
		}
	}
	modules := ""
	if pl.GitDir != "" {
		modules = filepath.Join(pl.GitDir, "modules")
	}
	return kept, func(folder string) bool {
		return slices.Contains(passed, folder) || modules != "" && filepath.Base(folder) == "objects" && inside(modules, folder)
	}
}

// inside is whether at is folder or below it, as spelled.
func inside(folder, at string) bool {
	if folder == "" {
		return false
	}
	rel, err := filepath.Rel(folder, at)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// walkLook calls each for every regular file at at, or below it to depth levels, never following a link. A folder
// skip names, and one only another user can list, are passed over; a place that isn't there is nothing.
func walkLook(at string, depth int, skip func(string) bool, budget *int, each func(string, fs.FileInfo) error) error {
	*budget--
	if *budget < 0 {
		return &linkCheckError{fmt.Sprintf("Smolcorp couldn't look at every file it keeps from desks (more than %d, at %s), so the desk didn't start.", linkCheckMost, at)}
	}
	info, err := os.Lstat(at)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return couldntCheck(at, err)
	case info.Mode().IsRegular():
		return each(at, info)
	case !info.IsDir() || depth == 0 || skip(at):
		return nil
	}
	entries, err := os.ReadDir(at)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) && ownedByAnother(info) {
			return nil // root's cron folders: the owner's user can't make a link inside either
		}
		return couldntCheck(at, err)
	}
	for _, e := range entries {
		if err := walkLook(filepath.Join(at, e.Name()), depth-1, skip, budget, each); err != nil {
			return err
		}
	}
	return nil
}

func couldntCheck(at string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return &linkCheckError{fmt.Sprintf("Smolcorp couldn't check %s, which it keeps from desks (%v), so the desk didn't start.", at, err)}
}

func linkedRefusal(at string) error {
	return &linkCheckError{fmt.Sprintf("%s has a second name on this %s (a hard link), so a desk could read or change it through that name, and the desk didn't start. Remove the other name (%s), then start it again.", at, machineHere(), otherNames(at))}
}

// otherNames is how the owner finds the other names of the file at at.
func otherNames(at string) string {
	if runtime.GOOS == "windows" {
		return `fsutil hardlink list "` + at + `" lists it`
	}
	return "find ~ -samefile '" + at + "' lists it"
}

// linksOf is how many names the regular file at at has. Windows tells it only of an open file.
func linksOf(at string, info fs.FileInfo) (uint64, error) {
	if runtime.GOOS != "windows" {
		return fileLinks(nil, info), nil
	}
	f, err := os.Open(at)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if now, err := f.Stat(); err != nil || !os.SameFile(now, info) {
		return 0, fmt.Errorf("it changed while it was checked: %w", err)
	}
	return fileLinks(f, info), nil
}

// freshCopy puts a copy of the file at at in its place by a rename, with its mode: a write-new-then-rename, so the
// file's other names keep the old, dead copy. The guard's saves wait for it. A file that changes while it is copied,
// or whose names are no longer the links the check met, isn't replaced: the start is refused, and the next one looks
// again. The file is closed before the rename: Windows can't put a file in place of one still open (Go opens without
// FILE_SHARE_DELETE).
func freshCopy(at string, info fs.FileInfo, links uint64) error {
	guardSaving.Lock()
	defer guardSaving.Unlock()
	temp, err := os.CreateTemp(filepath.Dir(at), "."+filepath.Base(at)+".fresh-*")
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			temp.Close()
			os.Remove(temp.Name())
		}
	}()
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := copyFrom(temp, at, info); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	now, err := os.Lstat(at)
	if err != nil || !os.SameFile(now, info) || now.Size() != info.Size() || !now.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("it changed while it was copied: %w", err)
	}
	if n, err := linksOf(at, now); err != nil {
		return err
	} else if n != links {
		return fmt.Errorf("its names changed while it was copied (%d, not %d)", n, links)
	}
	if err := os.Rename(temp.Name(), at); err != nil {
		return err
	}
	done = true
	return nil
}

// copyFrom copies into w the plain file at at that the check looked at as info, and closes it before it returns. As
// copyLooked does: it's opened through no link and without waiting on a pipe (openClaudeIn), only while it's still
// the file looked at, and read no further than its size then, which checkLinks holds to freshCopyMost. So a desk with
// another name for the file can't make the copy long by growing it through that name (FXFRESHCOPYCAP): a copy that
// isn't the size looked at fails.
func copyFrom(w io.Writer, at string, info fs.FileInfo) error {
	src, err := openClaudeIn(nil, at, info)
	if err != nil {
		return err
	}
	defer src.Close()
	if now, err := src.Stat(); err != nil || !now.Mode().IsRegular() || !os.SameFile(now, info) {
		return errors.New("it changed while it was looked at")
	}
	n, err := io.Copy(w, io.LimitReader(src, info.Size()+1))
	if err == nil && n != info.Size() {
		err = errors.New("it changed while it was copied")
	}
	if closeErr := src.Close(); err == nil {
		err = closeErr
	}
	return err
}

// freshCopySmallMost is the most bytes freshCopy copies of a file Smolcorp writes for a desk: the guard file, the night
// shift's jobs, a desk's brief, its fence, its profile and its Claude settings. The real ones are a few tens of KiB
// (on 17 Nov, the owner's 32 briefs at most 14 KiB, 30 fences at most 19 KiB); the night shift reads no more than
// nightFileMost of its jobs, and a brief or the guard's patterns come in a request of at most maxRequestSize.
const freshCopySmallMost = 4 << 20

// freshCopyMost is the most bytes freshCopy copies of the file at at, in the desk at pl's start: the largest a real
// one can be. What the core copies into a Linux desk's own folder at each start, its Claude folder's copies and its
// kept copy, may be as big as a start copies into the Claude folder at most (deskClaudeCopyBytesMost); Smolcorp's own
// files, freshCopySmallMost. A bigger file with a second name was grown through that name: the start is refused.
func freshCopyMost(pl sandbox.Places, at string) int64 {
	copies := []string{filepath.FromSlash(sandbox.KeptFolder(pl.Support, pl.OwnID))}
	for _, name := range deskClaudeCopied {
		copies = append(copies, filepath.Join(pl.Support, "hires", pl.OwnID, deskClaudeName, name))
	}
	for _, folder := range copies {
		if inside(folder, at) {
			return deskClaudeCopyBytesMost
		}
	}
	return freshCopySmallMost
}

// oldGitPlant is one thing in the shared .git that runs a program for the owner's git, from before the sandbox: a
// setting in the repo's common config or a submodule's, or a hook that isn't one of git's samples. Setup lists it
// with a "Remove" button; ID is what the button sends back. Held is one of git's substitutions (gitsubst.go): the
// desk doesn't start until it's removed or kept, and Command is what its Remove runs ("" when it has none). Follows
// is the ref a symbolic replace ref follows, which can't be kept (gitSubstKeepable).
type oldGitPlant struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Key     string `json:"key,omitempty"` // a setting's name; "" for a hook
	Value   string `json:"value,omitempty"`
	Why     string `json:"why"`
	Held    bool   `json:"held,omitempty"`
	Command string `json:"command,omitempty"`
	Follows string `json:"follows,omitempty"`

	info  fs.FileInfo // a hook's file as it was listed
	subst gitSubst    // a substitution's, as it was listed
}

const (
	whyProgram   = "Git runs a program it names."
	whyHooks     = "Git runs its hooks from the folder it names."
	whyInclude   = "Git reads more settings from the file it names."
	whyTransport = "Lets git use a transport that runs a program (ext:: runs the one a remote's address names)."
	whyUnknown   = "A setting Smolcorp doesn't know, which may run a program."
	whyHook      = "A hook: git runs it for the owner."
	whyRemote    = "An old remote file: it gives a remote an address, which can name a program git runs (ext::, a helper)."
	whyUnread    = "Smolcorp couldn't read it, so what it runs isn't known."
	whyTooMany   = "Too many folders to look through; the rest weren't checked."
	gitPlantMax  = 10_000
)

// findOldGitPlants lists what in gitDir, the repo's shared .git, runs a program for the owner's git: each setting of
// its config, config.worktree, every worktree's config.worktree and every submodule's config that runs one (gitstate's
// classifier: a program it names, a hooks folder, an include, a key it doesn't know), and each file or link in its
// hooks folder and the submodules' that isn't git's sample, and every file or link in their old remote folders
// (remoteFilePlants). It only reads; a file it can't read is listed as such.
func findOldGitPlants(ctx context.Context, gitDir string) []oldGitPlant {
	var plants []oldGitPlant
	configs := []string{filepath.Join(gitDir, "config"), filepath.Join(gitDir, "config.worktree")}
	if entries, err := os.ReadDir(filepath.Join(gitDir, "worktrees")); err == nil {
		for _, e := range entries {
			configs = append(configs, filepath.Join(gitDir, "worktrees", e.Name(), "config.worktree"))
		}
	}
	hooks, gitDirs := []string{filepath.Join(gitDir, "hooks")}, []string{gitDir}
	modules, unread, tooMany := gitModuleDirs(filepath.Join(gitDir, "modules"))
	for _, folder := range unread {
		plants = append(plants, gitPlant(oldGitPlant{File: folder, Why: whyUnread}))
	}
	for _, m := range modules {
		configs = append(configs, filepath.Join(m, "config"), filepath.Join(m, "config.worktree"))
		hooks = append(hooks, filepath.Join(m, "hooks"))
		gitDirs = append(gitDirs, m)
	}
	if tooMany {
		plants = append(plants, gitPlant(oldGitPlant{File: filepath.Join(gitDir, "modules"), Why: whyTooMany}))
	}
	for _, file := range configs {
		plants = append(plants, configPlants(ctx, file)...)
	}
	for _, folder := range hooks {
		entries, err := os.ReadDir(folder)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			plants = append(plants, gitPlant(oldGitPlant{File: folder, Why: whyUnread}))
			continue
		}
		for _, e := range entries {
			at := filepath.Join(folder, e.Name())
			info, err := os.Lstat(at)
			if err != nil || info.IsDir() || strings.HasSuffix(e.Name(), ".sample") {
				continue // git runs neither a folder nor a sample
			}
			plants = append(plants, gitPlant(oldGitPlant{File: at, Why: whyHook, info: info}))
		}
	}
	for _, dir := range gitDirs {
		for _, folder := range []string{"branches", "remotes"} {
			plants = append(plants, remoteFilePlants(filepath.Join(dir, folder))...)
		}
	}
	return plants
}

// remoteFilePlants lists every file or link in folder, a git folder's branches/ or remotes/, at any depth (a remote's
// name may hold a slash). Each gives a remote config doesn't name an address, which the owner's fetch, pull and push
// use, and an address can name a program (ext::, a <helper>::). They aren't config, so gitstate never reads them, and
// what one says isn't judged here: each is listed (RT126GITCFG2's note 3; SIERRA, 14 Nov). A folder that can't be
// listed is listed as such, and past gitPlantMax entries the rest aren't looked at.
func remoteFilePlants(folder string) []oldGitPlant {
	var plants []oldGitPlant
	seen := 0
	filepath.WalkDir(folder, func(at string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			plants = append(plants, gitPlant(oldGitPlant{File: at, Why: whyUnread}))
			return nil
		case seen >= gitPlantMax:
			plants = append(plants, gitPlant(oldGitPlant{File: folder, Why: whyTooMany}))
			return fs.SkipAll
		}
		seen++
		if d.IsDir() {
			return nil
		}
		info, err := d.Info() // the entry itself: a link is listed, and removed, as a link
		if err != nil {
			plants = append(plants, gitPlant(oldGitPlant{File: at, Why: whyUnread}))
			return nil
		}
		plants = append(plants, gitPlant(oldGitPlant{File: at, Why: whyRemote, info: info}))
		return nil
	})
	return plants
}

// gitModuleDirs are the submodules' git folders under modules, nested ones included: a folder with a HEAD is one, and
// any other folder is part of a submodule's name. A folder it can't list is in unread. It stops past gitPlantMax
// folders.
func gitModuleDirs(modules string) (dirs, unread []string, tooMany bool) {
	seen := 0
	var walk func(folder string)
	walk = func(folder string) {
		entries, err := os.ReadDir(folder)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				unread = append(unread, folder)
			}
			return
		}
		for _, e := range entries {
			if !e.IsDir() || tooMany {
				continue
			}
			if seen++; seen > gitPlantMax {
				tooMany = true
				return
			}
			at := filepath.Join(folder, e.Name())
			if _, err := os.Lstat(filepath.Join(at, "HEAD")); err == nil {
				dirs = append(dirs, at)
				walk(filepath.Join(at, "modules"))
			} else {
				walk(at)
			}
		}
	}
	walk(modules)
	return dirs, unread, tooMany
}

// configPlants are the settings of one config file that run a program, read as a file with no include followed
// (an include is listed itself). A file that isn't there has none.
func configPlants(ctx context.Context, file string) []oldGitPlant {
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	settings, err := readConfigFile(ctx, file)
	if err != nil {
		return []oldGitPlant{gitPlant(oldGitPlant{File: file, Why: whyUnread})}
	}
	var plants []oldGitPlant
	for _, kv := range settings {
		if why := gitSettingRuns(kv[0], kv[1]); why != "" {
			plants = append(plants, gitPlant(oldGitPlant{File: file, Key: kv[0], Value: kv[1], Why: why}))
		}
	}
	return plants
}

// readConfigFile is file's settings, as `git config --file … --list -z --no-includes` gives them, run as gitstate's
// reads are (no system or global config, nothing from the environment) at the top of the volume, in no repo.
func readConfigFile(ctx context.Context, file string) ([][2]string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitStateTimeout)
	defer cancel()
	out, status, err := execGit(ctx, gitReadEnv(os.Environ(), nil), "-C", filepath.VolumeName(file)+string(filepath.Separator),
		"--no-pager", "config", "--file", file, "--no-includes", "--list", "-z")
	if err == nil && status != 0 {
		err = fmt.Errorf("git config exited %d", status)
	}
	if err != nil {
		return nil, err
	}
	var settings [][2]string
	for _, record := range strings.Split(string(out), "\x00") {
		if record != "" {
			key, value, _ := strings.Cut(record, "\n")
			settings = append(settings, [2]string{key, value})
		}
	}
	return settings, nil
}

// gitSettingRuns is why the setting key = value runs a program for the owner's git, "" when it doesn't: gitstate's
// classifier (gitRepoConfig) for a key with a program under a name of its own, or one it doesn't know; the keys its
// reads blank whatever a repo says (gitFixedKeys), which are programs and a hooks folder in a repo's own config; and
// includes, which bring in more settings.
func gitSettingRuns(key, value string) string {
	lower := strings.ToLower(key)
	section, name := lower, ""
	if first, last := strings.Index(lower, "."), strings.LastIndex(lower, "."); first >= 0 {
		section, name = lower[:first], lower[last+1:]
		if first != last {
			name = "*." + name
		}
	}
	rule := section + "." + name
	switch {
	case rule == "include.path" || section == "includeif" && strings.HasSuffix(rule, ".path"):
		return whyInclude
	case rule == "protocol.allow" || rule == "protocol.*.allow":
		// Which transports git may use: ext:: runs the program a remote's address names, and a transport git
		// doesn't know runs a git-remote-<name> program. Only "never" lets none through for certain; gitRepoConfig
		// blanks every other value to it.
		if value == "never" {
			return ""
		}
		return whyTransport
	case rule == "core.attributesfile":
		return "" // a file of attributes: no program (a filter is a setting of its own)
	case gitFixedKeys[rule]:
		switch {
		case value == "", rule == "core.fsmonitor" && gitBoolWord(value): // git's own fsmonitor, or none
			return ""
		case rule == "core.hookspath":
			if value == os.DevNull {
				return ""
			}
			return whyHooks
		}
		return whyProgram
	}
	blanks, refused := gitRepoConfig([]byte(key + "\n" + value + "\x00"))
	switch {
	case refused != "":
		return whyUnknown
	case len(blanks) == 0, value == "", value == blanks[0][1]:
		return ""
	case rule == "filter.*.required":
		return "" // whether a filter must run: no program
	case rule == "submodule.*.update" && !strings.HasPrefix(value, "!"):
		return "" // checkout, rebase, merge or none
	}
	return whyProgram
}

// gitBoolWord is whether value is one of git's words for true or false.
func gitBoolWord(value string) bool {
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "on", "off", "1", "0":
		return true
	}
	return false
}

// gitPlant is p with its ID: the same plant always gets the same one.
func gitPlant(p oldGitPlant) oldGitPlant {
	sum := sha256.Sum256([]byte(p.File + "\x00" + p.Key + "\x00" + p.Value + "\x00" + p.Why))
	p.ID = hex.EncodeToString(sum[:8])
	return p
}

// errGitPlantGone is a Remove for something that isn't listed, or isn't as it was when listed.
var errGitPlantGone = errors.New("it isn't there as it was listed: look again")

// removeOldGitPlant is the owner's "Remove" for the plant whose ID is id in listed, the list Setup showed: a hook or
// an old remote file is removed (a link, not what it leads to) if it is still the file that was listed; a setting is
// taken out of its file, that value only, if it is still there (matched as a whole by a quoted pattern, not
// --fixed-value, which needs git 2.30 or later); one of git's substitutions as removeGitSubst says. Nothing that
// wasn't listed can be removed, and a plant that can't be read isn't removed: the owner looks at it.
func removeOldGitPlant(ctx context.Context, listed []oldGitPlant, id string) error {
	i := slices.IndexFunc(listed, func(p oldGitPlant) bool { return p.ID == id })
	if i < 0 {
		return errGitPlantGone
	}
	p := listed[i]
	switch {
	case p.Held:
		return removeGitSubst(ctx, p)
	case p.Why == whyHook || p.Why == whyRemote:
		now, err := os.Lstat(p.File)
		if err != nil || p.info == nil || !os.SameFile(now, p.info) || now.Size() != p.info.Size() || !now.ModTime().Equal(p.info.ModTime()) {
			return errGitPlantGone
		}
		return os.Remove(p.File)
	case p.Key == "":
		return errors.New("Smolcorp can't remove this: look at it yourself")
	}
	settings, err := readConfigFile(ctx, p.File)
	if err != nil || !slices.Contains(settings, [2]string{p.Key, p.Value}) {
		return errGitPlantGone
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, status, err := execGit(ctx, gitReadEnv(os.Environ(), nil), "-C", filepath.VolumeName(p.File)+string(filepath.Separator),
		"--no-pager", "config", "--file", p.File, "--unset-all", p.Key, "^"+regexp.QuoteMeta(p.Value)+"$")
	if err == nil && status != 0 {
		err = fmt.Errorf("git config exited %d", status)
	}
	return err
}

// Claude plants (CLAUDEPLANT1, SIERRA's ruling on FXSAL1R1's call 1, 14 Nov, widened 15 Nov): what your own Claude
// Code reads and runs when it starts in a folder. That is everything in a `.claude` there (settings*.json, hooks,
// commands, agents, skills, output styles, whatever else), and a `.mcp.json`, whose MCP servers are programs it
// starts. A desk can make or change them where its sandbox can't refuse one name in a folder that takes others: in
// its place (a Mac's `**/.claude` rows hold the folder itself, not what's in it), or in a home folder it may write.
// So the core lists them, for the desk that made them, at its start and when it's gone (a fire, a failure, the GM's
// edit: every registry write that marks a hire gone, registryFile.save), at Take over, and at the gate itself (again
// for a gone hire until its session is known to have ended, claudeHireEnded), and
// HOLDS each one listed: Smolcorp doesn't start your Claude Code in its folder, or above or below it, until it's gone
// ("Remove") or you've said it's yours ("Keep"). A CLAUDE.md is only words: it isn't listed. What's held, each
// desk's baseline and what you kept are a sealed record (claudeheld.go), so a restart forgets none of it.
//
// What's listed is what isn't as it was at the desk's last listing (its baseline), nor as you kept it: the first
// time, everything that differs from your checkout at the same place, so a project's own `.claude` doesn't count. A
// folder counts as itself only (what's in it has rows of its own), so your checkout's untracked settings.local.json
// doesn't make a worktree's differ; a file by its bytes, a link by where it leads and what it leads to in the place.
// A desk's place is looked at whole; a home folder the desk may write, claudeHomeDepth folders down, and not on a
// Mac, where the pattern holds and macOS asks you before an app looks in several of them.

const (
	whyClaudeFolder   = "A .claude folder: Claude Code started here reads what it holds."
	whyClaudeSettings = "Claude Code's settings: they can run programs (hooks) and change what it may do."
	whyClaudeHooks    = "Hooks in .claude: programs Claude Code's settings can run."
	whyClaudeTools    = "Claude Code's commands, agents, skills or output styles: it follows what they say, and they can run programs."
	whyClaudeOther    = "In a .claude folder: Claude Code started here may read it."
	whyClaudeLink     = "A link or a file named .claude: Claude Code reads where it leads."
	whyClaudeMCP      = "An .mcp.json: Claude Code started here can run the MCP servers it names."
	// claudeHomeDepth is how far down a home folder the desk may write is looked in: ~/a/.claude to ~/a/b/c/.claude.
	claudeHomeDepth = 3
)

// claudeTools are the folders in .claude whose files Claude Code follows (whyClaudeTools).
var claudeTools = []string{"commands", "agents", "skills", "output-styles"}

// claudeHomeWalked is whether the home folders are looked in: not on a Mac (above). Tests set it.
var claudeHomeWalked = runtime.GOOS != "darwin"

// claudePlant is one thing listed and held: In is the folder Claude Code started in would read it (or, for a
// folder that couldn't be read, that folder). Sum is what it is ("" when it can't be read: never the same as
// anything), Dir whether it's a folder, Within the place it was looked for in.
type claudePlant struct {
	ID   string `json:"id"`
	Desk string `json:"desk"`
	File string `json:"file"`
	Why  string `json:"why"`
	In   string `json:"in"`
	Sum  string `json:"sum"`
	Dir  bool   `json:"dir,omitempty"`

	Within string `json:"within,omitempty"`

	info    fs.FileInfo
	outside bool // a link that leads out of where it was looked for, or nowhere: your checkout never clears it
}

// same is whether p is as was: a folder by being one (what's in it has rows of its own), anything else by its sum.
func (p claudePlant) same(was claudePlant) bool {
	return p.Sum != "" && was.Sum != "" && p.Why == was.Why && p.Dir == was.Dir && (p.Dir || p.Sum == was.Sum)
}

// claudeLook is a place the listing walks, depth folders down (-1, all), passing over the folders skip names.
type claudeLook struct {
	path  string
	depth int
	skip  func(string) bool
}

// claudeLooks are the places a desk at pl may make a .claude in: its place, and the home folders pol lets it write
// (but its place, walked as such). M6 lists them at the desk's start: holdClaudePlants(pl.OwnID, these, pl.Checkout).
func claudeLooks(pol *sandbox.Policy, pl sandbox.Places) []claudeLook {
	looks := []claudeLook{{path: pl.Root, depth: -1}}
	if !claudeHomeWalked {
		return looks
	}
	skip := func(at string) bool { // a folder the desk can't make a name in (a folder's own row only holds it in place)
		v, _ := pol.Decide(filepath.Join(at, "plant"), sandbox.Write)
		return v == sandbox.Deny || inside(pl.Root, at)
	}
	entries, err := os.ReadDir(pl.Home)
	if err != nil {
		return append(looks, claudeLook{path: pl.Home, depth: 1, skip: func(string) bool { return true }})
	}
	for _, e := range entries {
		if at := filepath.Join(pl.Home, e.Name()); e.IsDir() && !skip(at) {
			looks = append(looks, claudeLook{path: at, depth: claudeHomeDepth, skip: skip})
		}
	}
	return looks
}

// listClaudePlantsOf lists and holds what hire h left in its place, its worktree or else its folder, against its
// checkout: the one listing every fire, failure, Take over and gate calls. It never puts a gone hire on record as
// listed: only claudeHireEnded does, once its session is known to have ended.
func listClaudePlantsOf(h Hire) []claudePlant {
	place := h.place()
	if place == "" {
		return nil
	}
	return holdClaudePlants(h.ID, []claudeLook{{path: place, depth: -1}}, h.Repo)
}

// claudeHireEnded is said once hire id's session is known to have ended: a fire that saw its session or window gone
// (hirefire.go, hirefire_window.go, hirelauncher.go), a start that failed after its session was ended
// (hireengine.go). Gone by then, its place is listed once more and it's put on record as listed, even with nothing
// found: the gate doesn't list it again, so what you change there from now on is yours. A hire made gone any other
// way (the GM's edit, T3 Code's let go, a fire whose window nobody closed) stays off the record: its desk may still be
// at work, so every gate near it lists it again (RT126CLAUDEPLANT1R2 B4b). The listing is the worker's
// (claudePending), asked for now and waited for, outside any lock: the fire goes on once it's done, as it did when
// it was listed here.
func claudeHireEnded(id string) {
	h, ok := registryHire(id)
	if !ok || isOpen(h) {
		return
	}
	if asked := askClaudeListings([]Hire{h}, true, false); len(asked) > 0 {
		waitClaudeListed(asked)
	} else {
		markClaudeListed(h.ID) // no place: nothing to list
	}
}

// claudeSessionUp is whether gone hire h's own tmux session is still there, or can't be told (hireSessionUp): a desk
// the GM's edit made gone keeps running. Tests set it.
var claudeSessionUp = func(h Hire) bool { return hireSessionUp(h, testCopyPort(), engineOptions{}) }

// claudeOpening is told the name of each file the plant walk looked at, just before it opens it to sum it. Tests set
// it, to change what's there in between, as a desk still at work could.
var claudeOpening = func(name string) {}

// holdClaudePlants lists, for desk, every plant in looks that isn't as it was in its baseline nor as you kept it,
// holds each one and makes what it found the desk's baseline, on record (none while it found none: the record, a
// file in the support folder, is written only when what it holds changes). With no baseline, a plant the same as
// the one at its place in reference (the owner's checkout, a worktree's twin; not the place itself) isn't listed.
// It says what it listed. It's one listing (claudeWalk): past what that may look at and read, each look it didn't
// finish is listed and held as too much to check.
func holdClaudePlants(desk string, looks []claudeLook, reference string) []claudePlant {
	found := walkClaudePlants(looks, reference)
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	listed, changed := recordClaudePlantsLocked(desk, found)
	// More held is less your Claude Code may do: written under a new record key, so a copy from before doesn't fit.
	if changed || len(listed) > 0 {
		saveClaudeHeldLocked(len(listed) > 0)
	}
	return listed
}

// claudeFound is what one listing found, by path, and which of it is the same as in your checkout.
type claudeFound struct {
	plants    map[string]claudePlant
	checkouts map[string]bool
}

// walkClaudePlants is holdClaudePlants's listing, without claudeHeld: one claudeWalk of looks, and of reference.
func walkClaudePlants(looks []claudeLook, reference string) claudeFound {
	w := newClaudeWalk()
	found := map[string]claudePlant{}
	for _, l := range looks {
		if !w.over {
			w.walkClaudeLook(l, found)
		}
		if w.over {
			found[l.path] = claudePlant{File: l.path, Why: whyClaudeTooMuch, In: l.path, Within: l.path}
		}
	}
	checkouts := map[string]bool{} // the plants the same as your checkout's
	if len(looks) > 0 && reference != "" && physical(reference) != physical(looks[0].path) {
		refs := map[string]claudePlant{} // your checkout's, each summed once: a top's walk notes what's below it
		for _, at := range slices.Sorted(maps.Keys(found)) {
			p := found[at]
			if rel, err := filepath.Rel(looks[0].path, at); err == nil && filepath.IsLocal(rel) && !p.outside && p.Sum != "" {
				ref := filepath.Join(reference, rel)
				was, ok := refs[ref]
				if !ok {
					was = w.entry(ref, p.Why, "", reference, refs)
				}
				was.Why = p.Why
				checkouts[at] = p.same(was) && !was.outside
			}
		}
	}
	return claudeFound{plants: found, checkouts: checkouts}
}

// recordClaudePlantsLocked holds, for desk, what a listing found that isn't as in its baseline nor as you kept it,
// and makes what it found the desk's baseline (holdClaudePlants). It says what it listed, and whether the record
// changed. The caller holds claudeHeld, and writes the record.
func recordClaudePlantsLocked(desk string, f claudeFound) ([]claudePlant, bool) {
	found, checkouts := f.plants, f.checkouts
	baseline, known := claudeHeld.baselines[desk]
	changed := len(found) != len(baseline)
	for at, p := range found {
		was, ok := baseline[at]
		changed = changed || !ok || was.Sum != p.Sum || was.Why != p.Why || was.Dir != p.Dir
	}
	if known || len(found) > 0 {
		claudeHeld.baselines[desk] = found
	}
	var listed []claudePlant
	for at, p := range found {
		if was, ok := baseline[at]; ok && p.same(was) || p.same(claudeHeld.kept[at]) || !known && checkouts[at] {
			continue
		}
		p.Desk = desk
		sum := sha256.Sum256([]byte(desk + "\x00" + p.File + "\x00" + p.Why + "\x00" + p.Sum))
		p.ID = hex.EncodeToString(sum[:8])
		listed = append(listed, p)
	}
	slices.SortFunc(listed, func(a, b claudePlant) int { return strings.Compare(a.File, b.File) })
	for _, p := range listed {
		claudeHeld.plants[p.ID] = p
	}
	return listed, changed
}

// A desk decides what the plant walk finds (RT126CLAUDESUMOPEN B1, B2), and your Claude Code's start, a fire, and the
// gate wait for it (claudePending: no longer under frontDeskMu or the Deck's lock). So one listing looks at no more
// than:
//   - claudeLookMost entries, each name in a folder it lists, and each name looked up to find where a link leads
//     (resolve). A folder hire's place is its project folder, worktrees and all: a big one holds 233,000 to 464,000
//     entries. Listed by name and kind (listKinds), a name costs a busy MacBook about 6 µs in a 78,000-entry worktree
//     and 7 µs in a 464,000-entry folder (past what the Mac keeps in memory, kern.maxvnodes), where a look at each
//     name cost 13 and 44 µs at the same moment (FXCLAUDESUMOPENR3);
//   - claudeTreeMost of them in a .claude, or in what a link there leads to, and each .claude or .mcp.json in the
//     place: each is opened, read into every sum above it and noted on record (about 20 µs each, so 0.2 s; a .claude
//     holds tens to hundreds);
//   - claudeSumBytesMost bytes summed, each byte counted once for each sum it goes into, an entry's own and each
//     folder's above it in its .claude (about 3 GB/s, so 0.1 s);
//   - nothing at claudePathMost bytes or longer, and claudeLinkHops links for each link.
//
// Past any, the listing stops:
// what it was summing can't be summed, and each look it didn't finish is held as too much to check. Each folder below
// a look's own is opened through the folder it was listed in, only as a folder and through no link (in the place,
// openClaudeFolderIn; in a .claude, only while it's the one listed, openClaudeIn): one swapped for a link isn't
// followed, and can't be summed. A file is read once a listing, into every sum it's part of. Tests set them.
var (
	claudeLookMost     = 1_000_000
	claudeTreeMost     = 10_000
	claudeSumBytesMost = int64(256 << 20)
)

// claudePathMost is the longest path, in bytes, the plant walk goes to: no entry at a path this long or longer is
// opened, read or looked into, and it can't be summed (a folder past it in the place is noted as unread). The walk
// before this one went by path, and the system stopped it there (a Mac's MAXPATHLEN, PATH_MAX elsewhere), so the sums
// on record stay as they were. It also bounds how deep the walk goes, and so the fds it keeps open (one a folder
// down) and the paths it makes (RT126CLAUDESUMOPENR2 N1).
var claudePathMost = map[bool]int{true: 1024, false: 4096}[runtime.GOOS == "darwin"]

// claudeLinkHops is the most links the walk goes through to find where one link leads (Linux goes through 40, a Mac
// 32): past them, as past what the walk may still look at, the link can't be summed (RT126CLAUDESUMOPENR2 N2).
const claudeLinkHops = 40

// claudeSumMost is the most bytes of one file the plant walk reads. A bigger one can't be summed, so it never matches
// and stays held: a fire and your Claude Code's start wait for the walk, and a desk mustn't make it read for long.
const claudeSumMost = 32 << 20

const whyClaudeTooMuch = "Too much in it for Smolcorp to check (files, or bytes to read); what wasn't checked isn't known."

// errClaudeTooMuch is a sum the walk stopped making: the listing went past what it may look at or read.
var errClaudeTooMuch = errors.New("too much to check")

// claudeWalk is one listing: the entries it may still look at (tree of them in a .claude), the bytes it may still
// sum, and whether it went past any (over).
type claudeWalk struct {
	entries, tree int
	bytes         int64
	over          bool
	places        map[string]string // each place looked for within, with its links resolved (physical)
}

func newClaudeWalk() *claudeWalk {
	return &claudeWalk{entries: claudeLookMost, tree: claudeTreeMost, bytes: claudeSumBytesMost}
}

// spend takes entries (tree of them in a .claude) and bytes from what's left, and says whether the walk may go on.
func (w *claudeWalk) spend(entries, tree int, bytes int64) bool {
	if !w.over {
		w.entries -= entries
		w.tree -= tree
		w.bytes -= bytes
		w.over = w.entries < 0 || w.tree < 0 || w.bytes < 0
	}
	return !w.over
}

// physical is the place within with its links resolved, once a walk.
func (w *claudeWalk) physical(within string) string {
	if w.places == nil {
		w.places = map[string]string{}
	}
	p, ok := w.places[within]
	if !ok {
		p = physical(within)
		w.places[within] = p
	}
	return p
}

// resolve is where the link at leads, every link on the way resolved, as filepath.EvalSymlinks says (its walk,
// path/filepath/symlink.go, Go 1.27), but spent from the walk: each name looked up costs one entry, so a lookup of a
// path costs as many as it has names, and it goes through claudeLinkHops links at most. Past either, it's
// errClaudeTooMuch. A link elsewhere names where it leads as the system has it (claudeLinkSpelled).
func (w *claudeWalk) resolve(at string) (string, error) {
	path, sep := at, string(os.PathSeparator)
	volLen := len(filepath.VolumeName(path))
	if volLen < len(path) && os.IsPathSeparator(path[volLen]) {
		volLen++
	}
	vol := path[:volLen]
	dest, hops := vol, 0
	look := func() error { // dest is looked up: a name for each folder in it, and its own
		if !w.spend(1+strings.Count(dest, sep), 0, 0) {
			return errClaudeTooMuch
		}
		return nil
	}
	for start, end := volLen, volLen; start < len(path); start = end {
		for start < len(path) && os.IsPathSeparator(path[start]) {
			start++
		}
		end = start
		for end < len(path) && !os.IsPathSeparator(path[end]) {
			end++
		}
		windowsDot := runtime.GOOS == "windows" && path[len(filepath.VolumeName(path)):] == "."
		if end == start {
			break
		} else if path[start:end] == "." && !windowsDot {
			continue
		} else if path[start:end] == ".." {
			r := len(dest) - 1
			for ; r >= volLen && !os.IsPathSeparator(dest[r]); r-- {
			}
			if r < volLen || dest[r+1:] == ".." {
				if len(dest) > volLen {
					dest += sep
				}
				dest += ".."
			} else {
				dest = dest[:r]
			}
			continue
		}
		if len(dest) > len(filepath.VolumeName(dest)) && !os.IsPathSeparator(dest[len(dest)-1]) {
			dest += sep
		}
		dest += path[start:end]
		if err := look(); err != nil {
			return "", err
		}
		fi, err := os.Lstat(dest)
		if err != nil {
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			if !fi.Mode().IsDir() && end < len(path) {
				return "", syscall.ENOTDIR
			}
			continue
		}
		if hops++; hops > claudeLinkHops {
			return "", fmt.Errorf("%s goes through more than %d links: %w", at, claudeLinkHops, errClaudeTooMuch)
		}
		if err := look(); err != nil {
			return "", err
		}
		link, err := os.Readlink(dest)
		if err != nil {
			return "", err
		}
		if windowsDot && !filepath.IsAbs(link) {
			break
		}
		path = link + path[end:]
		if v := len(filepath.VolumeName(link)); v > 0 {
			if v < len(link) && os.IsPathSeparator(link[v]) {
				v++
			}
			vol, dest, end = link[:v], link[:v], v
		} else if len(link) > 0 && os.IsPathSeparator(link[0]) {
			vol, dest, end, volLen = link[:1], link[:1], 1, 1
		} else {
			r := len(dest) - 1
			for ; r >= volLen && !os.IsPathSeparator(dest[r]); r-- {
			}
			if r < volLen {
				dest = vol
			} else {
				dest = dest[:r]
			}
			end = 0
		}
	}
	return claudeLinkSpelled(w, filepath.Clean(dest))
}

// claudeSum is a sum the walk is making, of an entry and everything below it, depth names down from the top of the
// walk; err is why it can't be made.
type claudeSum struct {
	w     io.Writer
	depth int
	err   error
}

// failSums fails each sum not failed yet with err, if any.
func failSums(sums []*claudeSum, err error) {
	for _, s := range sums {
		if err != nil && s.err == nil {
			s.err = err
		}
	}
}

// write writes into each sum not failed what text gives for it, spending its bytes: past what the walk may sum, all
// fail.
func (w *claudeWalk) write(sums []*claudeSum, text func(*claudeSum) string) {
	texts, total := make([]string, len(sums)), 0
	for i, s := range sums {
		if s.err == nil {
			texts[i] = text(s)
			total += len(texts[i])
		}
	}
	if !w.spend(0, 0, int64(total)) {
		failSums(sums, errClaudeTooMuch)
		return
	}
	for i, s := range sums {
		if s.err == nil {
			io.WriteString(s.w, texts[i])
		}
	}
}

// claudeLooked is an entry the walk summed: at, rel names down from the top, looked at as info; sum is "" when it
// can't be summed, unread a folder that couldn't be opened or listed, outside a link that leads out of where it was
// looked for (or nowhere).
type claudeLooked struct {
	at      string
	rel     []string
	info    fs.FileInfo
	sum     string
	unread  bool
	outside bool
}

// plant is e as a plant, why given, read by Claude Code started in in, looked for within.
func (e claudeLooked) plant(why, in, within string) claudePlant {
	return claudePlant{File: e.at, Why: why, In: in, Within: within, Sum: e.sum, Dir: e.info.IsDir(), info: e.info,
		outside: e.outside}
}

// walkClaudeLook notes every plant in look l (walkClaude), from its folder, opened by its path as given.
func (w *claudeWalk) walkClaudeLook(l claudeLook, found map[string]claudePlant) {
	dir, err := openClaudeLook(l.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		found[l.path] = claudePlant{File: l.path, Why: whyUnread, In: l.path}
	default:
		w.walkClaude(dir, l.path, l, l.depth, found)
		dir.Close()
	}
}

// list is what's in the open folder dir (in a .claude, tree), by name, each looked at through dir: no more than the
// walk may still look at (past that it's over, and lists nothing).
func (w *claudeWalk) list(dir *os.File, tree bool) ([]fs.FileInfo, error) {
	var all []fs.FileInfo
	for {
		infos, err := dir.Readdir(1024)
		inTree := 0
		if tree {
			inTree = len(infos)
		}
		if !w.spend(len(infos), inTree, 0) {
			return nil, nil
		}
		all = append(all, infos...)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	dir.Seek(0, io.SeekStart) // lets go of what the listing holds (on a Mac, a second fd): one fd a folder down
	slices.SortFunc(all, func(a, b fs.FileInfo) int { return strings.Compare(a.Name(), b.Name()) })
	return all, nil
}

// listKinds is what's in the open folder dir, by name and kind only: the kind its listing gives (getdirentries, with
// no look at each name; where it gives none, a look through dir, fstatat), no more than the walk may still look at
// (past that it's over, and lists nothing). The place walk lists so (FXCLAUDESUMOPENR3 A): a look at each name was
// most of a listing's time.
func (w *claudeWalk) listKinds(dir *os.File) ([]fs.DirEntry, error) {
	var all []fs.DirEntry
	for {
		entries, err := dir.ReadDir(1024)
		if !w.spend(len(entries), 0, 0) {
			return nil, nil
		}
		all = append(all, entries...)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	dir.Seek(0, io.SeekStart) // as list
	slices.SortFunc(all, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return all, nil
}

// claudePlantName is whether name is a .claude or an .mcp.json, any case, trailing dots and spaces dropped (Windows).
func claudePlantName(name string) bool {
	name = strings.TrimRight(name, ". ")
	return strings.EqualFold(name, ".claude") || strings.EqualFold(name, ".mcp.json")
}

// walkClaude notes every plant in folder, open as dir, depth folders down, never following a link nor going into a
// .git: each .claude, with everything in it, and each .mcp.json (claudePlantName). The folder is listed by name and
// kind only (listKinds), and each folder in it opened through it as a folder only (openClaudeFolderIn). Where there's
// a .claude or an .mcp.json, the folder is listed again with a look at each name through dir (list), so each is looked
// at in full, as before. A folder it can't open, or list, is noted as unread.
func (w *claudeWalk) walkClaude(dir *os.File, folder string, l claudeLook, depth int, found map[string]claudePlant) {
	entries, err := w.listKinds(dir)
	if err != nil {
		found[folder] = claudePlant{File: folder, Why: whyUnread, In: folder}
		return
	}
	if slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return claudePlantName(e.Name()) }) {
		infos, err := w.list(dir, false)
		if err != nil {
			found[folder] = claudePlant{File: folder, Why: whyUnread, In: folder}
			return
		}
		for _, info := range infos {
			if w.over {
				return
			}
			if !claudePlantName(info.Name()) {
				continue
			}
			if !w.spend(0, 1, 0) { // each is opened, summed and noted on record, as an entry in a .claude is
				return
			}
			at := filepath.Join(folder, info.Name())
			if strings.EqualFold(strings.TrimRight(info.Name(), ". "), ".claude") {
				w.addClaudeTree(dir, at, info, l.path, found)
			} else {
				w.sumTree(dir, at, info, l.path, false, func(e claudeLooked) { found[at] = e.plant(whyClaudeMCP, folder, l.path) })
			}
		}
	}
	for _, e := range entries {
		if w.over {
			return
		}
		at := filepath.Join(folder, e.Name())
		switch {
		case claudePlantName(e.Name()) || !e.IsDir() || strings.EqualFold(e.Name(), ".git") || depth == 1 || l.skip != nil && l.skip(at):
		case len(at) >= claudePathMost:
			found[at] = claudePlant{File: at, Why: whyUnread, In: at}
		default:
			sub, err := openClaudeFolderIn(dir, e)
			switch {
			case errors.Is(err, fs.ErrNotExist):
			case err != nil:
				found[at] = claudePlant{File: at, Why: whyUnread, In: at}
			default:
				w.walkClaude(sub, at, l, depth-1, found)
				sub.Close()
			}
		}
	}
}

// addClaudeTree notes the .claude at at, in the open folder dir, looked at as looked, and, in a folder, everything in
// it but a CLAUDE.md, each a row of its own, never following a link.
func (w *claudeWalk) addClaudeTree(dir *os.File, at string, looked fs.FileInfo, within string, found map[string]claudePlant) {
	in := filepath.Dir(at)
	if !looked.IsDir() {
		w.sumTree(dir, at, looked, within, false, func(e claudeLooked) { found[at] = e.plant(whyClaudeLink, in, within) })
		return
	}
	w.sumTree(dir, at, looked, within, true, func(e claudeLooked) {
		switch {
		case e.unread:
			found[e.at] = claudePlant{File: e.at, Why: whyUnread, In: in}
		case len(e.rel) == 0:
			found[at] = e.plant(whyClaudeFolder, in, within)
		case !strings.EqualFold(e.info.Name(), "CLAUDE.md") || !e.info.Mode().IsRegular():
			found[e.at] = e.plant(claudeWhy(filepath.Join(e.rel...)), in, within)
		}
	})
}

// claudeWhy is why the entry rel, in a .claude, is listed.
func claudeWhy(rel string) string {
	first, deeper, _ := strings.Cut(strings.ToLower(filepath.ToSlash(rel)), "/")
	if settings, _ := path.Match("settings*.json", first); settings && deeper == "" {
		return whyClaudeSettings
	}
	switch {
	case first == "hooks":
		return whyClaudeHooks
	case slices.Contains(claudeTools, first):
		return whyClaudeTools
	}
	return whyClaudeOther
}

// claudeEntry is the plant at at, why given, read by Claude Code started in in, with what it is now: a file by its
// bytes, a folder by everything in it, a link by where it leads and, when that's within the place it was looked for
// in, by what it leads to (a link the checkout has too can't hide a change made through it). One that can't be read
// never matches. It's a listing of its own (claudeWalk).
func claudeEntry(at, why, in, within string) claudePlant {
	return newClaudeWalk().entry(at, why, in, within, nil)
}

// entry is claudeEntry in walk w, at looked at by its path. Given record, every entry below at is noted there too,
// by its path, summed in the same walk.
func (w *claudeWalk) entry(at, why, in, within string, record map[string]claudePlant) claudePlant {
	p := claudePlant{File: at, Why: why, In: in, Within: within}
	looked, err := os.Lstat(at)
	if err != nil || w.over {
		return p
	}
	w.sumTree(nil, at, looked, within, record != nil, func(e claudeLooked) {
		if record != nil {
			record[e.at] = e.plant(why, in, within)
		}
		if len(e.rel) == 0 {
			p = e.plant(why, in, within)
		}
	})
	return p
}

// sumClaudeTree writes into h everything at at and below it, never following a link: names, kinds, bytes and where
// links lead.
func sumClaudeTree(h io.Writer, at string) error {
	return newClaudeWalk().sumPath(h, at)
}

// sumPath is sumClaudeTree in walk w.
func (w *claudeWalk) sumPath(h io.Writer, at string) error {
	looked, err := os.Lstat(at)
	if err != nil {
		return err
	}
	s := &claudeSum{w: h}
	w.sumEntry(nil, at, at, looked, nil, []*claudeSum{s}, "", false, nil)
	return s.err
}

// sumTree sums the entry at at, in the open folder dir (or by its path, dir nil), looked at as looked, and everything
// below it, in one walk, and tells each what the top is, and, with all, every entry below it, each before the folder
// it's in. A link's own sum takes in what it leads to when that's within.
func (w *claudeWalk) sumTree(dir *os.File, at string, looked fs.FileInfo, within string, all bool, each func(claudeLooked)) {
	name := at
	if dir != nil {
		name = looked.Name()
	}
	w.sumEntry(dir, name, at, looked, nil, nil, within, all, each)
}

// sumEntry writes the entry name in dir (or at the path name, dir nil), at at, looked at as looked, rel names down
// from the top, into each of sums, and its own sum when each is told of it: its name from each sum's top, its kind,
// then where a link leads, a file's bytes, or each entry in a folder, by name (as filepath.WalkDir and a plain read
// did: sums on record still match). What fails, or the walk going past what it may do, fails every sum it was in.
func (w *claudeWalk) sumEntry(dir *os.File, name, at string, looked fs.FileInfo, rel []string, sums []*claudeSum,
	within string, all bool, each func(claudeLooked)) {
	var own *claudeSum
	var ownSum hash.Hash
	if each != nil && (all || len(rel) == 0) {
		ownSum = sha256.New()
		own = &claudeSum{w: ownSum, depth: len(rel)}
		sums = append(sums[:len(sums):len(sums)], own)
	}
	kind := looked.Mode().Type()
	w.write(sums, func(s *claudeSum) string {
		from := "."
		if len(rel) > s.depth {
			from = filepath.Join(rel[s.depth:]...)
		}
		return from + "\x00" + kind.String() + "\x00"
	})
	e := claudeLooked{at: at, rel: rel, info: looked}
	switch {
	case len(at) >= claudePathMost:
		e.unread = kind.IsDir()
		failSums(sums, fmt.Errorf("%s is past the longest path the walk goes to", at))
	case kind&fs.ModeSymlink != 0:
		target, err := readlinkIn(dir, name)
		w.write(sums, func(*claudeSum) string { return target + "\x00" })
		failSums(sums, err)
	case kind.IsRegular():
		failSums(sums, w.copyLooked(dir, name, at, looked, sums))
	case kind.IsDir():
		err := w.sumFolder(dir, name, at, looked, rel, sums, within, all, each)
		e.unread = err != nil
		failSums(sums, err)
	}
	if w.over {
		failSums(sums, errClaudeTooMuch)
	}
	if own == nil {
		return
	}
	if own.err == nil && kind&fs.ModeSymlink != 0 {
		switch to, err := w.resolve(at); {
		case errors.Is(err, errClaudeTooMuch):
			failSums([]*claudeSum{own}, err)
		case err == nil && within != "" && inside(w.physical(within), to):
			failSums([]*claudeSum{own}, w.sumPath(ownSum, to))
		default:
			e.outside = true
		}
	}
	if own.err == nil {
		e.sum = hex.EncodeToString(ownSum.Sum(nil))
	}
	each(e)
}

// sumFolder writes what's in the folder name in dir, looked at as looked, into sums (sumEntry): opened through dir,
// as the folder listed (openClaudeIn), and listed through itself. A folder it can't open or list is an error.
func (w *claudeWalk) sumFolder(dir *os.File, name, at string, looked fs.FileInfo, rel []string, sums []*claudeSum,
	within string, all bool, each func(claudeLooked)) error {
	f, err := openClaudeIn(dir, name, looked)
	if err != nil {
		return err
	}
	defer f.Close()
	infos, err := w.list(f, true)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if w.over {
			break
		}
		w.sumEntry(f, info.Name(), filepath.Join(at, info.Name()), info, append(rel[:len(rel):len(rel)], info.Name()),
			sums, within, all, each)
	}
	return nil
}

// copyLooked writes into each of sums not failed the plain file name in dir (or at the path name, dir nil), at at,
// looked at as looked, so that a desk still at work can't make the walk wait or read without end (RT126RESOLVESWEEP
// S6): it's opened through no link and without waiting on a pipe (openClaudeIn), only while it's still the plain file
// the walk looked at, and read once, no further than its size then, at most claudeSumMost bytes. A file changed in
// between can't be summed, nor one past what the walk may still sum.
func (w *claudeWalk) copyLooked(dir *os.File, name, at string, looked fs.FileInfo, sums []*claudeSum) error {
	if !looked.Mode().IsRegular() {
		return fmt.Errorf("%s changed while it was looked at", at)
	}
	if looked.Size() > claudeSumMost {
		return fmt.Errorf("%s is too big to read (%d bytes)", at, looked.Size())
	}
	var to []io.Writer
	for _, s := range sums {
		if s.err == nil {
			to = append(to, s.w)
		}
	}
	if len(to) == 0 {
		return nil // every sum it's in has failed already
	}
	if !w.spend(0, 0, looked.Size()*int64(len(to))) {
		return errClaudeTooMuch
	}
	claudeOpening(at)
	f, err := openClaudeIn(dir, name, looked)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(io.MultiWriter(to...), io.LimitReader(f, looked.Size()+1))
	if err == nil && n != looked.Size() {
		err = fmt.Errorf("%s changed while it was read", at)
	}
	return err
}

// claudeNear is whether a and b are the same folder, or one is in the other: as written and with links resolved,
// case-blind off Linux.
func claudeNear(a, b string) bool {
	for _, x := range []string{a, physical(a)} {
		for _, y := range []string{b, physical(b)} {
			if runtime.GOOS != "linux" {
				x, y = strings.ToLower(x), strings.ToLower(y)
			}
			if inside(x, y) || inside(y, x) {
				return true
			}
		}
	}
	return false
}

// claudeGate is why Smolcorp won't start your Claude Code in folder, or "", once it has listed each place of hires
// in or around it: every open hire's, since it may have made one since its last listing (a desk not fired yet, the
// GM); a gone one's not on record as listed (its session not known to have ended: the GM's edit, from before this
// record, or after it was lost); and a gone one's whose tmux session is still up, which counts as open. The first
// of hires counts for its ID. The listings are the worker's (claudePending): it waits up to claudeCheckWait for them,
// holding no lock, and past that says Smolcorp is still checking.
func claudeGate(folder string, hires []Hire) string {
	why, _ := claudeGateWithin(folder, hires, claudeCheckWait)
	return why
}

// claudeGateWithin is claudeGate, waiting up to wait, and whether all it says is that Smolcorp is still checking.
func claudeGateWithin(folder string, hires []Hire, wait time.Duration) (string, bool) {
	seen, ask := map[string]bool{}, []Hire(nil)
	for _, h := range hires {
		if seen[h.ID] || h.place() == "" || !claudeNear(folder, h.place()) {
			continue
		}
		seen[h.ID] = true
		if isOpen(h) || !claudeListedNow(h.ID) || claudeSessionUp(h) {
			ask = append(ask, h)
		}
	}
	askClaudeListings(ask, false, false)
	return claudeHeldRefusalWithin(folder, wait)
}

// claudeHeldRefusal is why Smolcorp won't start your Claude Code in folder, or "": a plant held for you is still
// there, in it, above it or below it, or a listing asked for there isn't done after claudeCheckWait.
func claudeHeldRefusal(folder string) string {
	why, _ := claudeHeldRefusalWithin(folder, claudeCheckWait)
	return why
}

// claudeHeldRefusalWithin is claudeHeldRefusal, waiting up to wait for what's asked for in or around folder, and
// whether all it says is that Smolcorp is still checking: a plant held is said first, as it won't clear by itself.
func claudeHeldRefusalWithin(folder string, wait time.Duration) (string, bool) {
	near := func(p claudePending) bool { return claudeNear(folder, p.Place) }
	waitClaudeListings(near, wait)
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	var held []claudePlant
	for _, p := range claudeHeld.plants {
		if _, err := os.Lstat(p.File); errors.Is(err, fs.ErrNotExist) {
			delete(claudeHeld.plants, p.ID) // cleared
			continue
		}
		if claudeNear(folder, p.In) {
			held = append(held, p)
		}
	}
	if len(held) > 0 {
		slices.SortFunc(held, func(a, b claudePlant) int { return strings.Compare(a.File, b.File) })
		return "Smolcorp won't start your Claude Code in " + folder + ": a hire left " + held[0].File + " (" + held[0].Why +
			") and it's held for you. Remove it in Setup, or keep it if it's yours, then try again.", false
	}
	var checking []string
	for _, p := range claudePendingLocked() {
		if near(p) {
			checking = append(checking, p.Place)
		}
	}
	if len(checking) == 0 {
		return "", false
	}
	slices.Sort(checking)
	return claudeStillChecking(folder, checking[0]), true
}

// claudeStillChecking is what a gate says while a listing of place, in or around folder, isn't done.
func claudeStillChecking(folder, place string) string {
	return "Smolcorp is still checking what a hire left in " + place + ", so it won't start your Claude Code in " + folder +
		" yet. That takes a few seconds and clears by itself."
}

// removeClaudePlant is your "Remove" for the plant held as id: taken away (a link, not what it leads to) if it's
// still what was listed (the same file, while the core runs). keep is your "Keep": it's yours, so neither it nor
// anything held in it is held any more, nor listed again unless it changes. Anything else is errGitPlantGone.
func removeClaudePlant(id string, keep bool) error {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	p, ok := claudeHeld.plants[id]
	if !ok {
		return errGitPlantGone
	}
	if !keep {
		now := claudeEntry(p.File, p.Why, p.In, p.Within)
		if p.Sum == "" || now.Sum != p.Sum || now.Dir != p.Dir || p.info != nil && (now.info == nil || !os.SameFile(now.info, p.info)) {
			return errGitPlantGone
		}
		if err := os.RemoveAll(p.File); err != nil {
			return err
		}
		delete(claudeHeld.plants, id)
	} else {
		for _, q := range claudeHeld.plants {
			if q.Desk == p.Desk && inside(p.File, q.File) {
				claudeHeld.kept[q.File] = q
				delete(claudeHeld.plants, q.ID)
			}
		}
	}
	saveClaudeHeldLocked(false)
	return nil
}
