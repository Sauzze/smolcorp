package sandbox

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// The places no mount holds in Linux's view (L1GUARD1, SIERRA's rulings of 15 Nov; RT126SAL1R3 F1). In the folders the
// desk's own work makes new names in (its place, its registration in `worktrees/`, the shared `objects`), the view
// holds each name the Policy denies by a mount on itself, and the kernel lets go of such a mount when a program of the
// owner's replaces the name by a rename. So the start keeps a copy of each (keptPlaces): a file's content, a folder's
// listing with each file's content in it, a link's target, each with its device and inode, in the desk's own folder
// (KeptFolder), which the view brings back read-only: the desk's guard reads it, nothing in the desk writes it.
//
// The guard compares on both sides of every tool call (CheckKept) and, if one changed, only stops the desk: it never
// touches what it found, whoever changed it, and it notes what it stopped the desk for beside the copy (NoteKeptStop),
// the one file there the view lets it write. Only the core puts back, outside the view, and only what the desk was
// stopped for, as it was when it was stopped: at a fire, a Take over or a Hand back (RestoreKept, L1GUARD1FIRE), and at
// the next start (SaveKept). What it finds there is first set aside whole in the desk's found folder, never removed, and the
// owner is told which place, before and after, when, and where it was set aside (KeptWords). Anything else changed
// between two sessions is left as it is, and is the next copy; the owner is told in Setup (KeptNewsFile).
//
// The repo's shared objects/info (its alternates among them) is never put back: the owner's git gc and maintenance
// write it, every desk on the repo shares it, and the repo's objects depend on it. A change stops the desk and is
// shown, and its next start takes it as it is. The desk's own branch folders, `refs/heads/hire/<id>` and
// `logs/refs/heads/hire/<id>`, are watched by device and inode alone, since its commits change what is in them. The
// owner's `git pack-refs`, `maintenance` or `gc` (or git's own in a busy repo) removes or remakes them, and the desk's
// next commit would fail at its lock: the desk is stopped with words that say so, and nothing is put back (the start
// makes them again).

// KeptFolder is where a desk's start keeps its copy: `<support>/hires/<id>/held`.
func KeptFolder(support, id string) string {
	return path.Join(spell("linux", support), "hires", id, keptName)
}

// KeptCopy is the folder that holds a desk's last copy: KeptFolder, or, when a start stopped between its two renames
// (SaveKept), the last copy it set aside beside it, which is still the last copy. "" when the desk keeps none: only a
// Linux desk started in its view does (L1GUARD1FIRE).
func KeptCopy(support, id string) string {
	held := KeptFolder(support, id)
	for _, at := range []string{held, held + keptOld} {
		if _, err := os.Lstat(at); !errors.Is(err, fs.ErrNotExist) {
			return at
		}
	}
	return ""
}

// KeptNewsFile is where a desk's starts list what changed between two of its sessions and was taken as it was, for
// Setup to show the owner: `<support>/hires/<id>/kept-news.txt`. Each start adds to it; Setup empties it once read.
func KeptNewsFile(support, id string) string {
	return path.Join(spell("linux", support), "hires", id, keptNewsName)
}

// ErrKeptChanged fails a start that put back a place its last session was stopped for; the error says which, and
// where what was there is set aside.
var ErrKeptChanged = errors.New("sandbox: the desk's last session was stopped for a place no mount holds, and its saved copy is put back")

const (
	keptName      = "held"
	keptOld       = ".old" // beside held: the last copy, while a start puts its new one in its place
	keptList      = "kept.json"
	keptStopName  = "kept-stop.json" // beside held: the guard's notes of what it stopped the desk for
	keptNewsName  = "kept-news.txt"
	keptFoundName = "found"
	keptFoundTime = "2006-01-02T15-04-05.000000000Z0700"
	keptMost      = 256 << 20 // bytes copied at most: more fails the start
	keptNames     = 10000     // places at most
	keptShown     = 2048      // bytes of a before or after shown
	keptNotesMost = 1 << 20   // bytes of the guard's notes read at most
)

// keptPlace is one place kept: branch, a branch folder, is watched by its device and inode alone; repo, the shared
// objects/info, is compared as any other and never put back.
type keptPlace struct {
	at           string
	branch, repo bool
}

// keptPlaces are the places pl's start keeps: its place's .claude, the shared objects/info, and in its own worktree
// its .git link file, its registration's commondir, gitdir, config.worktree and old remote folders, and its branch
// folders. The same as the view's placeholders (viewMade) and the key test's awaitingSierra.
func keptPlaces(pl Places) []keptPlace {
	root, git := spell("linux", pl.Root), spell("linux", pl.GitDir)
	kept := []keptPlace{{at: path.Join(root, ".claude")}}
	if pl.GitDir == "" {
		return kept
	}
	kept = append(kept, keptPlace{at: git + "/objects/info", repo: true})
	if pl.OwnWorktree == "" {
		return kept
	}
	kept = append(kept, keptPlace{at: path.Join(root, ".git")})
	for _, name := range []string{"commondir", "gitdir", "config.worktree", "branches", "remotes"} {
		kept = append(kept, keptPlace{at: path.Join(git, "worktrees", pl.OwnWorktree, name)})
	}
	return append(kept, keptPlace{at: git + "/refs/heads/hire/" + pl.OwnID, branch: true},
		keptPlace{at: git + "/logs/refs/heads/hire/" + pl.OwnID, branch: true})
}

// keptEntry is a place as it was: kind is f (a file), d (a folder), l (a link), - (missing) or ? (anything else). Sum
// is a file's sha256: its copy is <held>/<sum>. Repo is the shared objects/info, or in it.
type keptEntry struct {
	At, Kind, Sum, To string
	Mode              fs.FileMode
	Dev, Ino          uint64
	Branch, Repo      bool
}

type keptCopy struct {
	Taken   time.Time
	Entries []keptEntry
}

// KeptChange is a place found changed: how, its before and after, and when. Seal is what was found, as the guard notes
// it. Back says its saved copy was made again, Found where what was there was set aside ("" when nothing was), Kept
// that it was left as it is, as the next copy. None of them: only seen (the guard's look).
type KeptChange struct {
	At, What, Before, After string
	Branch, Repo            bool
	When                    time.Time
	Seal, Found             string
	Back, Kept              bool
}

// looker walks places. Saving, into is where a file's content is copied, and seen and copied count against the caps.
// Checking, known is the copy: a place it doesn't have is added, and is neither read nor walked into; shown is what
// the look read of each place not as the copy has it, for the words (a file's text, the names in a folder).
//
// A desk can swap any folder below its .claude while the core looks (FXKEPTFD, RT126RESOLVESWEEP S3b): the look goes
// one name at a time in the folders it opened, each opened only if it is still the folder its Lstat saw, and never
// reads a place again by its path.
type looker struct {
	into         string
	known        map[string]keptEntry
	seen, copied int
	entries      []keptEntry
	shown        map[string]string
}

// lookTop looks at the kept place at from the folder above it, opened by its path as the start resolved it.
func (l *looker) lookTop(at string, branch, repo bool) error {
	dir, err := openKeptRoot(path.Dir(at))
	if err == nil {
		defer dir.Close()
	}
	return l.look(dir, err, at, branch, repo)
}

// show keeps what a checking look read of e, from its own fd, when e isn't as the copy has it.
func (l *looker) show(e keptEntry, text func() string) {
	if l.into == "" && l.known[e.At] != e {
		if l.shown == nil {
			l.shown = map[string]string{}
		}
		l.shown[e.At] = text()
	}
}

// look looks at at, the name it has in dir (when dir couldn't be opened, opened is why, and at is looked at as missing
// when that is because it isn't there).
func (l *looker) look(dir *os.Root, opened error, at string, branch, repo bool) error {
	_, known := l.known[at]
	known = known || l.known == nil
	e := keptEntry{At: at, Kind: "-", Branch: branch, Repo: repo}
	name := path.Base(at)
	st, err := fs.FileInfo(nil), opened
	if err == nil {
		st, err = dir.Lstat(name)
	}
	if l.seen++; l.into != "" && l.seen > keptNames {
		return fmt.Errorf("more than %d places to keep", keptNames)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !branch { // a reftable repo has no branch folder
		return err
	} else if err == nil {
		e.Kind, e.Mode = "?", st.Mode()
		e.Dev, e.Ino, _ = fileID(st)
	}
	switch {
	case err != nil:
	case branch: // the desk may change what is in its branch folder, and its mode: only what it is counts
		e.Kind, e.Mode = map[bool]string{true: "d", false: "?"}[st.IsDir()], 0
	case st.Mode().IsRegular() && (!known || l.into == "" && st.Size() > int64(keptMost-l.copied)):
		e.Kind, e.Sum = "f", "(not read)" // added, or larger than any copy: changed either way
		l.show(e, func() string { return keptText(readKeptIn(dir, name, st, keptShown)) })
	case st.Mode().IsRegular():
		data, err := readKeptIn(dir, name, st, keptMost-l.copied)
		sum := sha256.Sum256(data)
		e.Kind, e.Sum, l.copied = "f", hex.EncodeToString(sum[:]), l.copied+len(data)
		if err != nil && l.into == "" { // checking, one that can't be read is changed
			e.Sum = "(can't be read)"
			l.show(e, func() string { return keptText(nil, err) })
			break
		} else if err == nil && l.into != "" {
			err = os.WriteFile(path.Join(l.into, e.Sum), data, 0o600)
		}
		if err != nil {
			return err
		}
		l.show(e, func() string {
			if len(data) > keptShown {
				return keptText(data[:keptShown+1], fmt.Errorf("%s holds more than %d bytes", at, keptShown))
			}
			return keptText(data, nil)
		})
	case st.Mode()&fs.ModeSymlink != 0:
		e.Kind = "l"
		if e.To, err = dir.Readlink(name); err != nil {
			return err
		}
	case st.IsDir():
		e.Kind = "d"
		l.entries = append(l.entries, e)
		i := len(l.entries) - 1
		sub, err := keptSub(dir, name, st)
		if err == nil {
			defer sub.Close()
		}
		var names []fs.DirEntry
		if known && err == nil {
			names, err = keptNamesIn(sub)
		}
		if known && err != nil { // a folder that can't be listed is changed, never an error that leaves it so
			l.entries[i].Sum = "(can't be listed)"
		}
		l.show(l.entries[i], func() string { return keptListing(sub) })
		if !known {
			return nil
		}
		for _, n := range names {
			if err := l.look(sub, nil, path.Join(at, n.Name()), false, repo); err != nil {
				return err
			}
		}
		return nil
	}
	l.entries = append(l.entries, e)
	return nil
}

// keptSub opens the folder name in dir, and only if it is still the folder st (a look's Lstat of it) says. os.Root
// keeps what it opens inside dir, but follows a link that stays there: one swapped in meanwhile is refused here, by the
// device and inode of its own fd.
//
// A desk can swap a pipe, a socket or a link for any folder below its .claude meanwhile, and opening a pipe to read
// waits for a writer: the desk's to give (RT126KEPTFD B1). Root.OpenRoot(name) passes no O_DIRECTORY, but name/. makes
// name a folder on the way, which Root opens with O_DIRECTORY|O_NOFOLLOW: the kernel refuses anything else at once
// (ENOTDIR), before it is opened, so nothing but a folder is ever opened or waited on.
func keptSub(dir *os.Root, name string, st fs.FileInfo) (*os.Root, error) {
	sub, err := dir.OpenRoot(name + "/.")
	if err != nil {
		return nil, err
	}
	if now, err := sub.Stat("."); err != nil || !os.SameFile(st, now) {
		sub.Close()
		return nil, fmt.Errorf("%s was replaced while it was opened", path.Join(dir.Name(), name))
	}
	return sub, nil
}

// openKeptRoot opens the folder at by its path, and only a folder: at/. makes the kernel refuse anything else at once
// (ENOTDIR), so a pipe, a device or a socket there is never opened or waited on (RT126KEPTFD B1). A link in the path is
// followed, as the start resolved it: no desk can write the folders above a kept place.
func openKeptRoot(at string) (*os.Root, error) { return os.OpenRoot(at + "/.") }

// keptNamesIn are the names in the folder dir, by name.
func keptNamesIn(dir *os.Root) ([]fs.DirEntry, error) {
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.ReadDir(-1)
	slices.SortFunc(names, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return names, err
}

// readKept is the file at's content, at most most bytes (and an error past them), read without following a link or
// waiting on a pipe, and only if it is still the file st says.
func readKept(at string, st fs.FileInfo, most int) ([]byte, error) {
	f, err := openKept(at)
	return readOpened(f, err, at, st, most)
}

// readKeptIn is readKept of the file name in dir.
func readKeptIn(dir *os.Root, name string, st fs.FileInfo, most int) ([]byte, error) {
	f, err := dir.OpenFile(name, os.O_RDONLY|keptNoWait, 0)
	return readOpened(f, err, path.Join(dir.Name(), name), st, most)
}

func readOpened(f *os.File, err error, at string, st fs.FileInfo, most int) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fst, err := f.Stat(); err != nil || !os.SameFile(st, fst) {
		return nil, fmt.Errorf("%s was replaced while it was read", at)
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(most)+1))
	if err == nil && len(data) > most {
		err = fmt.Errorf("%s holds more than %d bytes", at, most)
	}
	return data, err
}

// SaveKept is the start's copy of pl's kept places, in its own folder, in place of the last start's. The last copy is
// first compared: a place the last session was stopped for, still as it was when the guard noted it, is put back
// (what is there set aside whole), and the start then fails once, with ErrKeptChanged and the words. Anything else
// changed is left as it is, taken into the new copy, and listed for Setup (KeptNewsFile); so is a change to the shared
// objects/info, which is never put back. A branch folder the start made again isn't counted.
func SaveKept(pl Places) error {
	held := KeptFolder(pl.Support, pl.OwnID)
	old := held + keptOld
	// A start that stopped between its two renames below left the last copy aside: it is still the last copy.
	if _, err := os.Lstat(held); errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Lstat(old); err == nil {
			if err := os.Rename(old, held); err != nil {
				return fmt.Errorf("sandbox: the last start's copy of the places no mount holds couldn't be found again: %w", err)
			}
		}
	}
	var back, kept []KeptChange
	if _, err := os.Lstat(path.Join(held, keptList)); err == nil {
		changes, c, err := compareKept(held, false)
		if err == nil {
			stops := keptStops(held)
			changes, err = putBackAll(held, changes, c, func(ch KeptChange) bool { return stops[ch.At][ch.Seal] })
		}
		if err != nil {
			words := KeptWords(changes) // what this look put back before, and where what it found is set aside
			if words != "" {
				words = "\n" + words
			}
			return fmt.Errorf("sandbox: the last start's copy of the places no mount holds couldn't be checked, or a place "+
				"its session was stopped for put back (%w). Move that place aside yourself and start the hire again: "+
				"what is there then is kept as its new copy. Removing %q starts it with everything as it is now.%s", err, held, words)
		}
		for _, ch := range changes {
			if ch.Back {
				back = append(back, ch)
			} else {
				ch.Kept = true // objects/info too: the start takes it as it is
				kept = append(kept, ch)
			}
		}
	}
	fresh := held + ".new"
	l := &looker{into: fresh}
	err := errors.Join(os.RemoveAll(fresh), os.RemoveAll(old), os.MkdirAll(fresh, 0o700))
	for _, k := range keptPlaces(pl) {
		if err == nil {
			err = l.lookTop(k.at, k.branch, k.repo)
		}
	}
	if err != nil {
		return fmt.Errorf("sandbox: the places no mount holds can't be kept: %w", err)
	}
	if err = writeList(fresh, keptCopy{Taken: time.Now(), Entries: l.entries}); err == nil {
		// The last copy is set aside before the new one takes its name, and only then removed.
		if err = os.Rename(held, old); errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		if err == nil {
			err = os.Rename(fresh, held)
		}
		err = cmp.Or(err, os.RemoveAll(old))
	}
	// The guard's notes are read: the file is made empty for this session, which the view binds writable.
	if err == nil {
		err = emptyKept(keptStopFile(held))
	}
	if err == nil && len(kept) > 0 {
		err = addNews(KeptNewsFile(pl.Support, pl.OwnID), KeptWords(kept))
	}
	if err == nil && len(back) > 0 {
		err = fmt.Errorf("%w: %s", ErrKeptChanged, KeptWords(back))
	}
	return err
}

func writeList(held string, c keptCopy) error {
	data, err := json.Marshal(c)
	if err == nil {
		err = os.WriteFile(path.Join(held, keptList), data, 0o600)
	}
	return err
}

// addNews adds words, with the time, to the news file at.
func addNews(at, words string) error {
	f, err := os.OpenFile(at, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err == nil {
		_, err = fmt.Fprintf(f, "%s, at a start:\n%s\n\n", time.Now().Format(keptWhen), words)
		err = errors.Join(err, f.Close())
	}
	return err
}

// CheckKept is what changed of the places the copy in held keeps (an error when there is no copy), each with the seal
// of what was found. Nothing is touched: the guard's look, inside the desk's view.
func CheckKept(held string) ([]KeptChange, error) {
	changes, _, err := compareKept(held, true)
	return changes, err
}

// RestoreKept is the core's put back at a fire, a Take over or a Hand back (L1GUARD1FIRE): what changed of the places
// the copy in held keeps (an error when there is no copy). Only a place the desk's guard stopped it for, still as it
// was then, is put back, what is there set aside whole in the desk's found folder first; anything else is Kept, left
// as it is: the owner's own edit, the owner's gc in the shared objects/info, which is never put back, a change made
// again after the stop. An error when one couldn't be put back: the caller then refuses what it was doing.
func RestoreKept(held string) ([]KeptChange, error) {
	changes, c, err := compareKept(held, true)
	if err != nil {
		return nil, err
	}
	stops := keptStops(held)
	return putBackAll(held, changes, c, func(ch KeptChange) bool { return stops[ch.At][ch.Seal] })
}

// putBackAll puts back each of changes back says, but a branch folder and the shared objects/info: what is there is
// set aside in one found folder for this look. The others back refuses, and objects/info, are Kept. Once all is put back, the copy's list takes the
// devices and inodes of what was made again (the core alone writes it: in the desk's view it is read-only), so the
// next look doesn't find the same change again. Content is still compared, so a place changed again is still found.
func putBackAll(held string, changes []KeptChange, c keptCopy, back func(KeptChange) bool) ([]KeptChange, error) {
	found := path.Join(path.Dir(held), keptFoundName, time.Now().Format(keptFoundTime))
	var err error
	ids := map[string]fs.FileInfo{} // what was made again, from its own fd: never the disk again by its path
	for i, ch := range changes {
		switch {
		case ch.Branch:
		case ch.Repo || !back(ch):
			changes[i].Kept = true
		case err == nil:
			var made map[string]fs.FileInfo
			changes[i].Found, made, err = putBack(held, found, ch, c)
			changes[i].Back = err == nil
			maps.Copy(ids, made)
		}
	}
	for i, e := range c.Entries {
		if st, ok := ids[e.At]; ok {
			c.Entries[i].Dev, c.Entries[i].Ino, _ = fileID(st)
		}
	}
	if slices.ContainsFunc(changes, func(ch KeptChange) bool { return ch.Back }) {
		err = errors.Join(err, writeList(held, c))
	}
	return changes, err
}

// compareKept compares the copy in held with the places now: only the topmost change of each place is said.
func compareKept(held string, branches bool) ([]KeptChange, keptCopy, error) {
	var c keptCopy
	data, err := os.ReadFile(path.Join(held, keptList))
	if err != nil || json.Unmarshal(data, &c) != nil {
		return nil, c, cmp.Or(err, errors.New("its list can't be read"))
	}
	saved, now := map[string]keptEntry{}, map[string]keptEntry{}
	for _, e := range c.Entries {
		saved[e.At] = e
	}
	l := &looker{known: saved}
	for _, e := range c.Entries {
		if parent, ok := saved[path.Dir(e.At)]; (branches || !e.Branch) && (!ok || parent.Branch) && err == nil {
			err = l.lookTop(e.At, e.Branch, e.Repo)
		}
	}
	var changes []KeptChange
	for _, n := range l.entries {
		if now[n.At] = n; saved[n.At].Kind == "" {
			changes = append(changes, KeptChange{At: n.At, What: "added", Repo: n.Repo})
		}
	}
	for _, e := range c.Entries {
		if n := now[e.At]; n != e && (branches || !e.Branch) && (n.Kind != "" || e.Kind != "-") {
			changes = append(changes, KeptChange{At: e.At, What: keptWhat(e, n), Branch: e.Branch, Repo: e.Repo})
		}
	}
	if err != nil {
		return nil, c, err
	}
	slices.SortFunc(changes, func(a, b KeptChange) int { return strings.Compare(a.At, b.At) })
	keptBetween("show", "")
	var top []KeptChange
	for _, ch := range changes {
		if len(top) == 0 || !under(ch.At, top[len(top)-1].At) {
			ch.Before, ch.After, ch.When = keptShow(held, saved[ch.At], nil), keptShow("", now[ch.At], l.shown), time.Now()
			ch.Seal = keptSeal(ch.At, saved)
			top = append(top, ch)
		}
	}
	return top, c, nil
}

// keptWhat says how place was changed into now.
func keptWhat(was, now keptEntry) string {
	switch {
	case now.Kind == "-" || now.Kind == "":
		return "removed"
	case was.Branch || was.Kind != now.Kind || was.Dev != now.Dev || was.Ino != now.Ino:
		return "replaced"
	}
	return "changed" // its content, its mode, or what is in it
}

// keptSeal is what is at at now, and in it, by each name's kind, mode, device, inode, size, time and link: the same
// seen from the desk's view and from the core. Any later change to it, by anyone, changes the seal. It is walked as the
// look walks (RT126KEPTFD N1): from the folder above it, opened as the copy saved it (keptOpen), one name at a time,
// never through a link or a folder swapped meanwhile, never waiting on a pipe.
func keptSeal(at string, saved map[string]keptEntry) string {
	dir, err := keptOpen(path.Dir(at), saved)
	if err == nil {
		defer dir.Close()
	}
	return keptSealIn(dir, err, at)
}

// keptSealIn is keptSeal of at, the name it has in dir (when dir couldn't be opened, opened is why).
func keptSealIn(dir *os.Root, opened error, at string) string {
	h, n := sha256.New(), 0
	var walk func(dir *os.Root, opened error, at string)
	walk = func(dir *os.Root, opened error, at string) {
		name := path.Base(at)
		st, err := fs.FileInfo(nil), opened
		if err == nil {
			st, err = dir.Lstat(name)
		}
		if n++; n > keptNames {
			fmt.Fprintf(h, "(more)\n")
			return
		} else if err != nil {
			fmt.Fprintf(h, "%q -\n", at)
			return
		}
		to, _ := dir.Readlink(name)
		dev, ino, _ := fileID(st)
		fmt.Fprintf(h, "%q %v %d %d %d %d %q\n", at, st.Mode(), dev, ino, st.Size(), st.ModTime().UnixNano(), to)
		if st.IsDir() {
			sub, err := keptSub(dir, name, st)
			var names []fs.DirEntry
			if err == nil {
				defer sub.Close()
				names, err = keptNamesIn(sub)
			}
			if err != nil {
				fmt.Fprintf(h, "(can't be listed)\n")
			}
			for _, e := range names {
				walk(sub, nil, path.Join(at, e.Name()))
			}
		}
	}
	walk(dir, opened, at)
	return hex.EncodeToString(h.Sum(nil))
}

// keptShow is e's content as shown: a file's text, a folder's mode, else what it is. Before, a file's text is held's
// copy of it. After (held ""), it is what the look read of e from its own fd (shown: a file's text, the names in a
// folder), never the place read again by its path, which a desk may have swapped meanwhile (FXKEPTFD S3b).
func keptShow(held string, e keptEntry, shown map[string]string) string {
	switch e.Kind {
	case "", "-":
		return "(not there)"
	case "l":
		return "(a link to " + e.To + ")"
	case "?":
		return "(" + e.Mode.String() + ")"
	case "d": // a folder as it is now shows the names in it: an added one is never walked by the look
		return "(" + e.Mode.String() + ")" + shown[e.At]
	}
	if held == "" {
		if text, ok := shown[e.At]; ok {
			return text
		}
		return "(not read)"
	}
	at := path.Join(held, e.Sum)
	st, err := os.Lstat(at)
	var data []byte
	if err == nil {
		data, err = readKept(at, st, keptShown)
	}
	return keptText(data, err)
}

// keptText is a file's content read, as shown.
func keptText(data []byte, err error) string {
	switch {
	case len(data) == 0 && err != nil:
		return "(can't be read: " + err.Error() + ")"
	case bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) && err == nil:
		return "(not text)"
	}
	return string(data)
}

// keptListing is the names in the folder dir (none when it is nil), a line each, a folder's ending in /, at most
// keptShown of them: each folder opened from the one it is in, as the look opens it.
func keptListing(dir *os.Root) string {
	var b strings.Builder
	n := 0
	var walk func(dir *os.Root, shown string)
	walk = func(dir *os.Root, shown string) {
		names, _ := keptNamesIn(dir)
		for _, name := range names {
			if n++; n > keptShown {
				return
			}
			line := shown + name.Name()
			if name.IsDir() {
				line += "/"
			}
			b.WriteString("\n" + line)
			if st, err := dir.Lstat(name.Name()); err == nil && name.IsDir() && st.IsDir() {
				if sub, err := keptSub(dir, name.Name(), st); err == nil {
					walk(sub, line)
					sub.Close()
				}
			}
		}
	}
	if dir != nil {
		walk(dir, "")
	}
	if n > keptShown {
		b.WriteString("\n(and more)")
	}
	return b.String()
}

// keptStopFile is the file beside held the desk's guard notes what it stopped the desk for in.
func keptStopFile(held string) string { return path.Join(path.Dir(held), keptStopName) }

type keptStop struct{ At, Seal string }

// NoteKeptStop notes, for the next start, the places changes stopped the desk for, each with the seal of what was
// found: a line each, added to the file beside held (the start made it, the view binds it writable). A branch folder
// and the shared objects/info aren't noted: neither is ever put back. A place noted already isn't noted again.
func NoteKeptStop(held string, changes []KeptChange) error {
	noted := keptStops(held)
	var lines []byte
	for _, ch := range changes {
		if !ch.Branch && !ch.Repo && !noted[ch.At][ch.Seal] {
			line, _ := json.Marshal(keptStop{ch.At, ch.Seal})
			lines = append(append(lines, line...), '\n')
		}
	}
	if len(lines) == 0 {
		return nil
	}
	f, err := openKeptWrite(keptStopFile(held), os.O_APPEND)
	if err == nil {
		_, err = f.Write(lines)
		err = errors.Join(err, f.Close())
	}
	return err
}

// keptStops are the places the guard noted, each with the seals it noted. The desk can write the file: a line that
// isn't a note is passed over, and a note only ever names what was found at a place, which a later change unseals.
func keptStops(held string) map[string]map[string]bool {
	stops := map[string]map[string]bool{}
	at := keptStopFile(held)
	st, err := os.Lstat(at)
	var data []byte
	if err == nil {
		data, _ = readKept(at, st, keptNotesMost)
	}
	lines := bufio.NewScanner(bytes.NewReader(data))
	lines.Buffer(nil, keptNotesMost)
	for lines.Scan() {
		var s keptStop
		if json.Unmarshal(lines.Bytes(), &s) == nil && s.At != "" && s.Seal != "" {
			if stops[s.At] == nil {
				stops[s.At] = map[string]bool{}
			}
			stops[s.At][s.Seal] = true
		}
	}
	return stops
}

// emptyKept makes the file at empty, or makes it, never through a link.
func emptyKept(at string) error {
	f, err := openKeptWrite(at, os.O_CREATE|os.O_TRUNC)
	if err == nil {
		err = f.Close()
	}
	return err
}

// putBack puts the place ch names back as c has it: what is there now is set aside whole under found, then the copy's
// subtree is made again; it gives where it was set aside, and what it made. A folder above it must still be the folder
// saved, and what is there must be set aside whole, or nothing is touched.
//
// A desk can swap any folder below its .claude while its session is still up, which it is at a fire (FXKEPTFD,
// RT126RESOLVESWEEP S3). So all of it is done in the folder above the place, held open once checked, one name at a
// time: never by a path, never through a link, and what is in it is removed only through its own folders' fds.
func putBack(held, found string, ch KeptChange, c keptCopy) (string, map[string]fs.FileInfo, error) {
	above := map[string]keptEntry{}
	var below []keptEntry
	for _, e := range c.Entries {
		if e.At != ch.At && under(ch.At, e.At) {
			above[e.At] = e
		} else if under(e.At, ch.At) {
			below = append(below, e)
		}
	}
	parent, err := keptOpen(path.Dir(ch.At), above)
	if err != nil {
		return "", nil, err
	}
	defer parent.Close()
	keptBetween("put back", ch.At)
	name, aside := path.Base(ch.At), ""
	if _, err := parent.Lstat(name); err == nil {
		aside = path.Join(found, ch.At)
		if err := setAside(parent, name, aside); err != nil {
			return "", nil, fmt.Errorf("%s couldn't be set aside whole, so it is left as it is: %w", ch.At, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", nil, err
	}
	made, err := makeKept(held, parent, path.Dir(ch.At), below)
	if err != nil {
		aside, err = putBackFailed(held, ch, parent, aside, err)
		return aside, nil, err
	}
	return aside, made, nil
}

// keptOpen opens the folder at: from the first folder above it the copy doesn't save (the one above a kept place),
// opened by its path as the start resolved it, then one name at a time, each a folder and not a link, and the folder
// the copy saved by its own fd's device and inode.
func keptOpen(at string, saved map[string]keptEntry) (*os.Root, error) {
	top := at
	for _, ok := saved[top]; ok; _, ok = saved[top] {
		top = path.Dir(top)
	}
	dir, err := openKeptRoot(top)
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(strings.TrimPrefix(strings.TrimPrefix(at, top), "/"), "/") {
		if name == "" {
			continue
		}
		top = path.Join(top, name)
		st, err := dir.Lstat(name)
		var sub *os.Root
		if err == nil && st.IsDir() {
			sub, err = keptSub(dir, name, st)
		}
		dir.Close()
		if err != nil || !st.IsDir() {
			return nil, fmt.Errorf("%s isn't the folder it was", top)
		}
		if dev, ino, _ := fileID(st); dev != saved[top].Dev || ino != saved[top].Ino {
			sub.Close()
			return nil, fmt.Errorf("%s isn't the folder it was", top)
		}
		dir = sub
	}
	return dir, nil
}

// makeKept makes below again (in the list's order: a folder before what is in it), each in the folder it is in, as
// made and held open, the first in parent, the folder at: a file new, never through a link made meanwhile. It gives
// what it made, as its own fd or the folder it is in saw it.
func makeKept(held string, parent *os.Root, at string, below []keptEntry) (map[string]fs.FileInfo, error) {
	dirs := map[string]*os.Root{at: parent}
	defer func() {
		for made, dir := range dirs {
			if made != at {
				dir.Close()
			}
		}
	}()
	made := map[string]fs.FileInfo{}
	for _, e := range below {
		dir, name := dirs[path.Dir(e.At)], path.Base(e.At)
		var st fs.FileInfo
		var err error
		switch {
		case e.Kind == "-":
			continue
		case dir == nil:
			err = fmt.Errorf("%s has no folder to be made again in", e.At)
		case e.Kind == "d":
			if err = dir.Mkdir(name, e.Mode.Perm()); err == nil {
				var sub *os.Root
				if st, err = dir.Lstat(name); err == nil {
					sub, err = keptSub(dir, name, st)
				}
				if err == nil {
					dirs[e.At] = sub
				}
			}
		case e.Kind == "l":
			if err = dir.Symlink(e.To, name); err == nil {
				st, err = dir.Lstat(name)
			}
		case e.Kind == "f":
			var data []byte
			var f *os.File
			if data, err = os.ReadFile(path.Join(held, e.Sum)); err == nil {
				if f, err = dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, e.Mode.Perm()); err == nil {
					_, err = f.Write(data)
					err = errors.Join(err, f.Chmod(e.Mode.Perm()))
					st, _ = f.Stat()
					err = errors.Join(err, f.Close())
				}
			}
		default:
			err = fmt.Errorf("%s can't be made again (%s)", e.At, e.Mode)
		}
		if err != nil {
			return nil, err
		}
		made[e.At] = st
	}
	return made, nil
}

// keptBetween is a test's stand-in for a desk swapping a folder in the middle between a look and what is done after it.
var keptBetween = func(step, at string) {}

// keptRename is renameKept, for setAside and putBackFailed: a test's stand-in for a found folder on another disk.
var keptRename = renameKept

// putBackFailed is a put back whose saved copy couldn't be made again whole once what was there was set aside (a full
// disk at the write, the copy's content unreadable): what it made is taken away and what was set aside goes back in
// its place, so the place is as it was found, nothing set aside. When it can't go back, the error and the words say
// where what was there is set aside, and that the place is half made or not there (RT126L1GUARD1FIRE B2). Either way
// the place is noted, as the guard notes what it stops the desk for, so the next look (a fire, a Hand back, a start)
// puts its saved copy back again, setting what is there aside whole, and refuses again until it can: it never goes on
// as if nothing had been tried. Both are done in parent, the folder above the place, held open.
func putBackFailed(held string, ch KeptChange, parent *os.Root, aside string, failed error) (string, error) {
	why := fmt.Errorf("%s couldn't be made again as its saved copy has it (%w)", ch.At, failed)
	name := path.Base(ch.At)
	undo := parent.RemoveAll(name)
	switch {
	case undo != nil:
		why = fmt.Errorf("%w, and what it made couldn't be taken away (%v): the place is left half made", why, undo)
	case aside != "":
		var found *os.Root
		if found, undo = openKeptRoot(path.Dir(aside)); undo == nil {
			undo = keptRename(found, path.Base(aside), parent, name)
			found.Close()
		}
		if undo != nil {
			why = fmt.Errorf("%w, and what was there couldn't go back in its place (%v): the place is not there now", why, undo)
		}
	}
	if undo == nil {
		why, aside = fmt.Errorf("%w, so what it made is taken away, and the place is as it was found", why), ""
	} else if aside != "" {
		why = fmt.Errorf("%w, and what was there is set aside whole in %q", why, aside)
	}
	if note := NoteKeptStop(held, []KeptChange{{At: ch.At, Seal: keptSealIn(parent, nil, ch.At)}}); note != nil {
		return aside, fmt.Errorf("%w. It couldn't be noted to be put back again either (%v)", why, note)
	}
	return aside, fmt.Errorf("%w. It is noted, to be put back again before anything goes on", why)
}

// setAside moves name in dir, whole, to aside (in the core's own found folder), which must not be there yet: by a
// rename, or on another disk by a copy, and only once the copy is whole is it removed. What can't be copied (a pipe,
// a socket, past the cap) leaves it as it is.
func setAside(dir *os.Root, name, aside string) error {
	if _, err := os.Lstat(aside); err == nil {
		return fmt.Errorf("%s is there already", aside)
	}
	if err := os.MkdirAll(path.Dir(aside), 0o700); err != nil {
		return err
	}
	to, err := openKeptRoot(path.Dir(aside))
	if err != nil {
		return err
	}
	defer to.Close()
	if keptRename(dir, name, to, path.Base(aside)) == nil {
		return nil
	}
	most := keptMost
	if err := copyAside(dir, name, to, path.Base(aside), &most); err != nil {
		return errors.Join(err, to.RemoveAll(path.Base(aside)))
	}
	return dir.RemoveAll(name)
}

// copyAside copies name in dir to toName in to, a folder with all in it, never following a link, each folder opened
// from the one it is in; most is what may still be copied.
func copyAside(dir *os.Root, name string, to *os.Root, toName string, most *int) error {
	st, err := dir.Lstat(name)
	switch {
	case err != nil:
		return err
	case st.Mode()&fs.ModeSymlink != 0:
		link, err := dir.Readlink(name)
		if err != nil {
			return err
		}
		return to.Symlink(link, toName)
	case st.IsDir():
		sub, err := keptSub(dir, name, st)
		if err != nil {
			return err
		}
		defer sub.Close()
		names, err := keptNamesIn(sub)
		if err = cmp.Or(err, to.Mkdir(toName, 0o700)); err != nil {
			return err
		}
		into, err := to.OpenRoot(toName + "/.") // only a folder, as keptSub
		if err != nil {
			return err
		}
		defer into.Close()
		for _, n := range names {
			if err := copyAside(sub, n.Name(), into, n.Name(), most); err != nil {
				return err
			}
		}
		return nil
	case st.Mode().IsRegular():
		data, err := readKeptIn(dir, name, st, *most)
		if err != nil {
			return err
		}
		*most -= len(data)
		return to.WriteFile(toName, data, st.Mode().Perm()&0o700|0o600)
	}
	return fmt.Errorf("%s can't be set aside (%s)", path.Join(dir.Name(), name), st.Mode())
}

// keptWhen is how a time is said: its zone with it.
const keptWhen = "15:04:05 MST on 2 Jan 2006"

// KeptWords are changes as the desk and the owner read them: each place, when it was seen and how, what was done (set
// aside and put back, kept as it is, or only seen), with the lines taken away (-) and added (+); for a branch folder,
// that the hire must restart.
func KeptWords(changes []KeptChange) string {
	var b strings.Builder
	diff := func(mark string, lines, from []string) {
		from = slices.Clone(from)
		for _, line := range lines {
			if i := slices.Index(from, line); i >= 0 {
				from = slices.Delete(from, i, i+1)
			} else {
				fmt.Fprintf(&b, "\n    %s %q", mark, line)
			}
		}
	}
	for _, c := range changes {
		fmt.Fprintf(&b, "\n- %q, seen at %s: %s.", c.At, c.When.Format(keptWhen), c.What)
		switch {
		case c.Branch:
			b.WriteString(" The owner's git repacked branches: restart this hire to keep committing.")
			continue
		case c.Repo && c.Kept:
			b.WriteString(" It is the repo's own (git's gc and maintenance write it): never put back, and kept as the desk's new copy.")
		case c.Repo:
			b.WriteString(" It is the repo's own (git's gc and maintenance write it), so it is never put back: restart this hire to go on with it as it is.")
		case c.Back && c.Found != "":
			fmt.Fprintf(&b, " What was there is set aside whole in %q, and its saved copy is put back.", c.Found)
		case c.Back:
			b.WriteString(" Its saved copy is put back.")
		case c.Found != "": // a put back that stopped once what was there was set aside (putBackFailed)
			fmt.Fprintf(&b, " Its saved copy couldn't be made again whole: what was there is set aside whole in %q, and "+
				"the place is half made or not there, to be put back again before anything goes on.", c.Found)
		case c.Kept:
			b.WriteString(" The desk's last session wasn't stopped for it as it is now, so it is left as it is, and kept as the desk's new copy.")
		default:
			b.WriteString(" Nothing is undone here: when the hire is fired, handed back or started again, Smolcorp sets " +
				"what is there aside whole and puts its saved copy back.")
		}
		b.WriteString(" Before, then after:")
		was, now := strings.Split(c.Before, "\n"), strings.Split(c.After, "\n")
		diff("-", was, now)
		diff("+", now, was)
	}
	if b.Len() == 0 {
		return ""
	}
	return "A place the desk may not change was changed:" + b.String()
}
