package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
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
	if err := copyFrom(temp, at); err != nil {
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

// copyFrom copies the file at at into w, and closes it before it returns.
func copyFrom(w io.Writer, at string) error {
	src, err := os.Open(at)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	if closeErr := src.Close(); err == nil {
		err = closeErr
	}
	return err
}

// oldGitPlant is one thing in the shared .git that runs a program for the owner's git, from before the sandbox: a
// setting in the repo's common config or a submodule's, or a hook that isn't one of git's samples. Setup lists it
// with a "Remove" button; ID is what the button sends back.
type oldGitPlant struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Key   string `json:"key,omitempty"` // a setting's name; "" for a hook
	Value string `json:"value,omitempty"`
	Why   string `json:"why"`

	info fs.FileInfo // a hook's file as it was listed
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
// --fixed-value, which needs git 2.30 or later). Nothing that wasn't listed can be removed, and a plant that can't be
// read isn't removed: the owner looks at it.
func removeOldGitPlant(ctx context.Context, listed []oldGitPlant, id string) error {
	i := slices.IndexFunc(listed, func(p oldGitPlant) bool { return p.ID == id })
	if i < 0 {
		return errGitPlantGone
	}
	p := listed[i]
	switch {
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
