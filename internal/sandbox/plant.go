package sandbox

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Plants at a desk at the project (GITPLANT1, SIERRA's decision on FXSAX1R4's Decision 3, 14 Nov). A desk that works
// at the project itself writes the owner's working tree, and the Policy can't deny it a `.git` in a subfolder without
// holding every folder above it, which would take ordinary work (Decision 3 (i)). A `.git` planted there, a file
// naming a git folder of the desk's or a folder with a config and hooks of its own, is what the owner's git takes for
// a repo later: any git run in that folder, or `git submodule update --init` once a merged branch names the path. A
// link is the other way in: one into a .git, or out of the project, sends a later write by the owner's tools there.
//
// So the core looks for them, the way it holds the guard and the records: what a desk wrote counts for nothing until
// the owner says so. Snap, at the desk's start, notes every .git, every such link and every folder it can't look
// inside that the working tree already has. Plants, at the desk's stop, lists every one that is new, or isn't the
// same as at the start, and HoldPlants moves each out of the tree for the owner, who puts it back (Restore) or drops
// it. M1, the desk's start and stop on a Mac, calls them:
//   - Snap before the desk's first process, at its Root, for a desk whose Checkout is "" (a desk at the project);
//   - Plants and then HoldPlants once every process of the desk has ended, and before anything else runs in the
//     tree: the core's own git (the recap's status, a merge), a hook, the next desk's start;
//   - and when Snap fails, the desk doesn't start; when Plants or HoldPlants fails, or a plant couldn't be held, the
//     whole tree is held: nothing runs in it until the owner has looked.
//
// It fails closed. A Snapshot is kept in memory only: lost (the core stopped in between), every .git and every link
// out counts as new. A .git file or a link that can't be read, a link that can't be followed to its end, and a folder
// that can't be listed are each listed. The repo's own `.git`, at the root, is never listed: the Policy denies it to
// every desk (RT126SAX1R5 B3).

// PlantKind is what a plant is.
type PlantKind int

const (
	// GitFolder is a folder named .git: a git folder of its own, with its config and hooks.
	GitFolder PlantKind = iota
	// GitFile is a file named .git: a gitfile, which names the git folder git then reads.
	GitFile
	// GitOther is anything else named .git: a link, a pipe.
	GitOther
	// LinkIntoGit is a link that ends in a .git, the repo's own or another.
	LinkIntoGit
	// LinkOutside is a link that ends outside the project.
	LinkOutside
	// LinkUnresolved is a link Smolcorp can't follow to its end: a loop, or one it can't read.
	LinkUnresolved
	// Unlistable is a folder Smolcorp can't look inside, so what was planted in it can't be told.
	Unlistable
)

func (k PlantKind) String() string {
	return [...]string{"a new or changed .git folder", "a new or changed .git file", "a new or changed .git that is neither a file nor a folder",
		"a new or changed link into a .git", "a new or changed link out of the project", "a new or changed link Smolcorp can't follow to its end",
		"a folder Smolcorp can't look inside"}[k]
}

// Plant is one thing a desk left in the working tree that the owner decides on.
type Plant struct {
	// Path is where it is, from the project's root, with slashes.
	Path string
	Kind PlantKind
	// Target is a link's target as it is written, "" for anything else.
	Target string
}

func (p Plant) String() string {
	if p.Target != "" {
		return p.Path + ": " + p.Kind.String() + " (to " + p.Target + ")"
	}
	return p.Path + ": " + p.Kind.String()
}

// Snapshot is the working tree as Snap found it at a desk's start: every entry that would be a plant then.
type Snapshot struct {
	root  string
	info  fs.FileInfo
	found map[string]seen
}

// seen is one entry as the scan found it: what it is, which file it is, when it last changed, and what it says (a
// gitfile's bytes, a link's target), so that one swapped in, rewritten, or made again in its place doesn't pass for
// it.
type seen struct {
	Plant
	info fs.FileInfo
	// changed is when the entry itself last changed, which no program can set back (ctime; off a Mac and Linux, the
	// time it was written). A .git folder's moves with every git run in it, so its birth time stands in (Mac; none
	// elsewhere): a folder made again in its place is still a new file (os.SameFile).
	changed time.Time
	sum     [sha256.Size]byte
	// unknown is an entry whose content couldn't be read: never the same as anything.
	unknown bool
	// open is a folder that can't be listed, but in which a name can be looked up: what is in it can still be
	// reached, so it is listed at every stop.
	open bool
}

func (a seen) same(b seen) bool {
	return a.Kind == b.Kind && a.Target == b.Target && !a.unknown && !b.unknown && a.info != nil && b.info != nil &&
		os.SameFile(a.info, b.info) && a.info.Mode() == b.info.Mode() && (a.info.IsDir() || a.info.Size() == b.info.Size()) &&
		a.changed.Equal(b.changed) && a.sum == b.sum
}

// Snap notes the working tree at root, at the start of a desk at the project: the start Plants compares with. An
// error is a root it can't look at; the desk mustn't start.
func Snap(root string) (*Snapshot, error) {
	root, info, err := realRoot(root)
	if err != nil {
		return nil, err
	}
	found, err := scan(root)
	if err != nil {
		return nil, err
	}
	return &Snapshot{root: root, info: info, found: found}, nil
}

// Plants is every plant in the working tree at root at a desk's stop, by path: each .git, link into a .git or out of
// the project, and folder that can't be listed, that start didn't find or found otherwise. A start that is nil, or
// was taken at another root, counts as none: everything is listed. An error is a tree it couldn't look at.
func Plants(root string, start *Snapshot) ([]Plant, error) {
	root, info, err := realRoot(root)
	if err != nil {
		return nil, err
	}
	found, err := scan(root)
	if err != nil {
		return nil, err
	}
	if start != nil && (start.root != root || !os.SameFile(start.info, info)) {
		start = nil
	}
	var plants []Plant
	for rel, now := range found {
		if start != nil && !now.open {
			if old, ok := start.found[rel]; ok && old.same(now) {
				continue
			}
		}
		plants = append(plants, now.Plant)
	}
	slices.SortFunc(plants, func(a, b Plant) int { return strings.Compare(a.Path, b.Path) })
	return plants, nil
}

// realRoot is root with every link in it followed, and the folder it is.
func realRoot(root string) (string, fs.FileInfo, error) {
	root, err := filepath.Abs(root)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return "", nil, err
	}
	info, err := os.Lstat(root)
	if err == nil && !info.IsDir() {
		err = fmt.Errorf("%s isn't a folder", root)
	}
	return root, info, err
}

// scan is every entry in the tree at root that would be a plant, by its path from root. It doesn't go into a .git, nor
// follow a link.
func scan(root string) (map[string]seen, error) {
	found := map[string]seen{}
	err := filepath.WalkDir(root, func(at string, d fs.DirEntry, err error) error {
		if at == root {
			return err // the root itself, or a root that can't be listed: nothing is known
		}
		rel, relErr := filepath.Rel(root, at)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if err != nil {
			// WalkDir calls again with the error of a folder it couldn't list, then goes on with what it listed.
			found[rel] = unlistable(rel, at)
			return nil
		}
		if rel == ".git" { // the repo's own, denied to every desk
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if gitName(d.Name()) {
			found[rel] = gitEntry(rel, at)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if linkish(d.Type()) {
			if s, ok := linkEntry(root, rel, at); ok {
				found[rel] = s
			}
		}
		return nil
	})
	return found, err
}

// gitName is whether name is one git takes for `.git` on some system: in any case (a Mac's and Windows' volumes), with
// the characters HFS+ ignores in it, with dots and spaces at its end (Windows drops them), or as Windows' short name
// `git~1`. git's own checks (is_hfs_dotgit, is_ntfs_dotgit) are the model; matching more only lists more.
func gitName(name string) bool {
	name = strings.Map(func(r rune) rune {
		if r >= 0x200c && r <= 0x200f || r >= 0x202a && r <= 0x202e || r >= 0x206a && r <= 0x206f || r == 0xfeff {
			return -1
		}
		return r
	}, name)
	name = strings.ToLower(strings.TrimRight(name, ". "))
	return name == ".git" || name == "git~1"
}

// linkish is whether an entry of this type points somewhere else: a link, or (Windows) a junction.
func linkish(mode fs.FileMode) bool { return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// seenAt is the entry at at as it is now, kind given.
func seenAt(rel, at string, kind PlantKind) (seen, fs.FileInfo, bool) {
	s := seen{Plant: Plant{Path: rel, Kind: kind}}
	info, err := os.Lstat(at)
	if err != nil {
		s.unknown = true
		return s, nil, false
	}
	s.info, s.changed = info, changeTime(info)
	return s, info, true
}

// gitEntry is the .git at at: a gitfile's bytes are read (a pipe isn't waited on, and a file swapped in under the
// read is unknown).
func gitEntry(rel, at string) seen {
	s, info, ok := seenAt(rel, at, GitOther)
	switch {
	case !ok:
	case info.IsDir():
		s.Kind, s.changed = GitFolder, birthTime(info)
	case info.Mode().IsRegular():
		s.Kind = GitFile
		s.sum, s.unknown = fileSum(at, info)
	case linkish(info.Mode()):
		target, err := os.Readlink(at)
		s.Target, s.unknown = target, err != nil
	}
	return s
}

// fileSum is the SHA-256 of the regular file at at, as long as it is still info's file, and whether it couldn't be
// read so.
func fileSum(at string, info fs.FileInfo) ([sha256.Size]byte, bool) {
	f, err := os.OpenFile(at, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return [sha256.Size]byte{}, true
	}
	defer f.Close()
	if now, err := f.Stat(); err != nil || !os.SameFile(now, info) || !now.Mode().IsRegular() {
		return [sha256.Size]byte{}, true
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, 1<<20)); err != nil { // the size is compared too
		return [sha256.Size]byte{}, true
	}
	return [sha256.Size]byte(h.Sum(nil)), false
}

// unlistable is the folder at at, which WalkDir couldn't list. One in which a name can still be looked up is open.
func unlistable(rel, at string) seen {
	s, _, _ := seenAt(rel, at, Unlistable)
	_, err := os.Lstat(filepath.Join(at, ".git"))
	s.open = !errors.Is(err, fs.ErrPermission)
	return s
}

// linkEntry is the link at at, and whether it is a plant: one that ends in a .git or outside the project at root, as
// it is written or as it resolves, or one that can't be read or followed to its end.
func linkEntry(root, rel, at string) (seen, bool) {
	s, _, ok := seenAt(rel, at, LinkUnresolved)
	if !ok {
		return s, true
	}
	target, err := os.Readlink(at)
	if err != nil {
		s.unknown = true
		return s, true
	}
	s.Target = target
	written := target
	if !filepath.IsAbs(written) {
		written = filepath.Join(filepath.Dir(at), target)
	}
	end, err := resolve(at)
	if err != nil {
		return s, true
	}
	for _, p := range []string{written, end} {
		if kind, plant := where(root, p); plant {
			s.Kind = kind
			return s, true
		}
	}
	return s, false
}

// where says what a link that ends at p is a plant as, and false when p is in the project at root and in no .git.
func where(root, p string) (PlantKind, bool) {
	r, k := key(runtime.GOOS, root), key(runtime.GOOS, p)
	if r == "" || k == "" || !under(k, r) {
		return LinkOutside, true
	}
	for _, name := range strings.Split(strings.TrimPrefix(k, strings.TrimSuffix(r, "/")), "/") {
		if gitName(name) {
			return LinkIntoGit, true
		}
	}
	return 0, false
}

var errLinkLoop = errors.New("too many links")

// resolve is where the absolute path p ends, as the system walks it: name by name, each link followed where it is met
// and `..` taken after it, as far as the path exists; the rest is joined as written. A loop, or more than 255 links,
// is an error.
func resolve(p string) (string, error) {
	budget := 255
	return walk(p, &budget)
}

func walk(p string, budget *int) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%s isn't whole", p)
	}
	vol := filepath.VolumeName(p)
	at := vol + string(filepath.Separator)
	parts := strings.FieldsFunc(p[len(vol):], func(r rune) bool { return r < 0x80 && os.IsPathSeparator(uint8(r)) })
	for i, part := range parts {
		switch part {
		case ".":
			continue
		case "..":
			at = filepath.Dir(at)
			continue
		}
		next := filepath.Join(at, part)
		info, err := os.Lstat(next)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return filepath.Join(append([]string{next}, parts[i+1:]...)...), nil
		}
		if err != nil {
			return "", err
		}
		if !linkish(info.Mode()) {
			at = next
			continue
		}
		if *budget--; *budget < 0 {
			return "", errLinkLoop
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = at + string(filepath.Separator) + target // not Join: its `..` are walked, after the links before them
		}
		if at, err = walk(target, budget); err != nil {
			return "", err
		}
	}
	return at, nil
}

// Held is a plant HoldPlants moved out of the working tree: At is where it is now.
type Held struct {
	Plant
	At string
}

// HoldPlants moves each plant out of the working tree at root, for the owner, into heldDir: a folder outside the tree,
// on the same volume, that doesn't exist yet. It is made owner-only, and each plant goes in a folder of its own in it,
// under its own name. A plant whose way from root has a link on it now isn't moved (the desk must have stopped: it
// would move what the link names), nor one that can't be moved; each is in the error, and the others are still
// moved. Held is what was moved.
func HoldPlants(root, heldDir string, plants []Plant) ([]Held, error) {
	root, _, err := realRoot(root)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(heldDir) {
		return nil, fmt.Errorf("%s isn't whole", heldDir)
	}
	spellings := []string{heldDir}
	if outer, err := filepath.EvalSymlinks(filepath.Dir(heldDir)); err == nil {
		spellings = append(spellings, filepath.Join(outer, filepath.Base(heldDir)))
	}
	for _, at := range spellings {
		if under(key(runtime.GOOS, at), key(runtime.GOOS, root)) {
			return nil, fmt.Errorf("%s is in the working tree at %s", heldDir, root)
		}
	}
	if err := os.MkdirAll(filepath.Dir(heldDir), 0o700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(heldDir, 0o700); err != nil {
		return nil, err
	}
	var held []Held
	var problems []error
	for i, plant := range plants {
		from, err := inTree(root, plant.Path)
		if err == nil {
			to := filepath.Join(heldDir, strconv.Itoa(i+1), filepath.Base(from))
			if err = os.Mkdir(filepath.Dir(to), 0o700); err == nil {
				if err = os.Rename(from, to); err == nil {
					held = append(held, Held{Plant: plant, At: to})
					continue
				}
			}
		}
		problems = append(problems, fmt.Errorf("%s isn't held: %w", plant.Path, err))
	}
	return held, errors.Join(problems...)
}

// Restore puts a held plant back where it was in the working tree at root, on the owner's word: only while nothing is
// there, and no link is on its way from root.
func Restore(root string, h Held) error {
	root, _, err := realRoot(root)
	if err != nil {
		return err
	}
	to, err := inTree(root, h.Path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(to); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: something is there now (%v)", h.Path, err)
	}
	if err := os.Rename(h.At, to); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(h.At)) // its own folder, empty now
	return nil
}

// inTree is rel's place in the working tree at root, checked: rel stays in the tree, and every folder on the way from
// root to it is a folder, not a link.
func inTree(root, rel string) (string, error) {
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) || filepath.Clean(local) != local {
		return "", fmt.Errorf("%s isn't a path in the tree", rel)
	}
	at := root
	parts := strings.Split(local, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		at = filepath.Join(at, part)
		info, err := os.Lstat(at)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || linkish(info.Mode()) {
			return "", fmt.Errorf("%s: %s is a link or a file now", rel, at)
		}
	}
	return filepath.Join(root, local), nil
}
