package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// GITPLANT1 on stand-ins: a project made by hand in the test's temp folder, no git run. Each test plays a desk at the
// project between Snap (its start) and Plants (its stop), checked only by what is listed and where a file is.

// tree is a stand-in project at <folder>/app, with a folder outside it at <folder>/outside.
type tree struct {
	t                     *testing.T
	folder, root, outside string
}

func newTree(t *testing.T) tree {
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := tree{t, folder, filepath.Join(folder, "app"), filepath.Join(folder, "outside")}
	tr.file(".git/HEAD", "ref: refs/heads/main\n")
	tr.file(".git/config", "[standin]\n\twho = owner\n")
	tr.file("src/main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(tr.outside, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	return tr
}

func (tr tree) at(rel string) string { return filepath.Join(tr.root, filepath.FromSlash(rel)) }

func (tr tree) file(rel, text string) {
	tr.t.Helper()
	if err := os.MkdirAll(filepath.Dir(tr.at(rel)), 0o700); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(tr.at(rel), []byte(text), 0o600); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) dir(rel string) {
	tr.t.Helper()
	if err := os.MkdirAll(tr.at(rel), 0o700); err != nil {
		tr.t.Fatal(err)
	}
}

// link makes a link at rel to target. Off a Mac and Linux, a test that needs links is skipped when the system won't
// make one (Windows, without developer mode).
func (tr tree) link(rel, target string) {
	tr.t.Helper()
	if err := os.MkdirAll(filepath.Dir(tr.at(rel)), 0o700); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.Symlink(target, tr.at(rel)); err != nil {
		if runtime.GOOS == "windows" {
			tr.t.Skipf("no links here: %v", err)
		}
		tr.t.Fatal(err)
	}
}

func (tr tree) snap() *Snapshot {
	tr.t.Helper()
	s, err := Snap(tr.root)
	if err != nil {
		tr.t.Fatal(err)
	}
	return s
}

// listed is Plants' list as "path kind" lines.
func (tr tree) listed(start *Snapshot) []string {
	tr.t.Helper()
	plants, err := Plants(tr.root, start)
	if err != nil {
		tr.t.Fatal(err)
	}
	var lines []string
	for _, p := range plants {
		lines = append(lines, fmt.Sprintf("%s %d", p.Path, p.Kind))
	}
	return lines
}

func line(rel string, kind PlantKind) string { return fmt.Sprintf("%s %d", rel, kind) }

func sameLines(t *testing.T, how string, got, want []string) {
	t.Helper()
	slices.Sort(want)
	if !slices.Equal(got, want) {
		for _, g := range got {
			if !slices.Contains(want, g) {
				t.Errorf("%s: listed %q, not wanted", how, g)
			}
		}
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("%s: %q not listed", how, w)
			}
		}
	}
}

// The owner's tree at a desk's start: a submodule's gitfile, a nested repo of the owner's, a link out of the project
// (docs, to a folder beside it), a link out through which a `..` comes back in name only (deep), and links inside.
func (tr tree) ownersTree() {
	tr.file("vendor/lib/.git", "gitdir: ../../.git/modules/lib\n")
	tr.file("vendor/old/.git/config", "[standin]\n\twho = old\n")
	tr.link("docs", filepath.Join("..", "outside"))
	tr.link("deep", filepath.Join(tr.outside, "a", "b"))
	tr.link("cur", "src")
	tr.link("node_modules/x", filepath.Join("..", "src"))
}

// SIERRA's GITPLANT1 (14 Nov): at a desk at the project's stop, every new .git file or folder in the working tree, and
// every new link into a .git or out of the project, is listed; what the owner had at the start, and what the desk did
// that is ordinary work, isn't.
func TestAtAProjectDesksStopEveryNewGitAndLinkOutIsListed(t *testing.T) {
	tr := newTree(t)
	tr.ownersTree()
	start := tr.snap()
	if got := tr.listed(start); len(got) != 0 {
		t.Fatalf("nothing done, and listed: %q", got)
	}

	// Ordinary work: none of it is listed.
	tr.file("src/new.go", "package main\n")
	tr.file(".git/standin", "the repo's own, denied to every desk\n")
	tr.file("vendor/old/.git/index", "the owner's nested repo, used\n")
	tr.link("l-in", filepath.Join("src", "main.go"))
	tr.link("l-dangling-in", filepath.Join("src", "missing.go"))

	want := []string{}
	plant := func(rel string, kind PlantKind) { want = append(want, line(rel, kind)) }
	// New .git files and folders, in every spelling git takes for one somewhere.
	tr.file("third/.git/config", "[standin]\n\twho = planted\n")
	plant("third/.git", GitFolder)
	tr.file("a/b/.git", "gitdir: "+tr.outside+"\n")
	plant("a/b/.git", GitFile)
	tr.dir("x/.Git")
	plant("x/.Git", GitFolder)
	tr.file("z/.g\u200cit", "gitdir: ../third/.git\n")
	plant("z/.g\u200cit", GitFile)
	tr.dir("w/git~1")
	plant("w/git~1", GitFolder)
	if runtime.GOOS != "windows" {
		tr.file("y/.git.", "gitdir: ../third/.git\n")
		plant("y/.git.", GitFile)
	}
	tr.link("s/.git", tr.outside)
	plant("s/.git", GitOther)
	// The owner's gitfile, rewritten where it is (the same file), and the owner's link out, made again elsewhere.
	if err := os.WriteFile(tr.at("vendor/lib/.git"), []byte("gitdir: "+tr.outside+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plant("vendor/lib/.git", GitFile)
	if err := os.Remove(tr.at("docs")); err != nil {
		t.Fatal(err)
	}
	tr.link("docs", filepath.Join("..", "outside", "a"))
	plant("docs", LinkOutside)
	// New links into a .git, or out, as written or as they resolve.
	tr.link("l-out", filepath.Join("..", "outside"))
	plant("l-out", LinkOutside)
	tr.link("l-abs", tr.outside)
	plant("l-abs", LinkOutside)
	tr.link("l-git", filepath.Join("vendor", "lib", ".git"))
	plant("l-git", LinkIntoGit)
	tr.link("l-root-git", filepath.Join(".git", "config"))
	plant("l-root-git", LinkIntoGit)
	tr.link("l-chain", "docs") // written inside; docs ends outside
	plant("l-chain", LinkOutside)
	sep := string(filepath.Separator)
	tr.link("l-dotdot", "deep"+sep+".."+sep+"src") // written, app/src; walked, deep/.. is outside/a
	plant("l-dotdot", LinkOutside)
	tr.link("l-dangling-out", filepath.Join("..", "nowhere", "x"))
	plant("l-dangling-out", LinkOutside)
	tr.link("l-loop1", "l-loop2")
	tr.link("l-loop2", "l-loop1")
	plant("l-loop1", LinkUnresolved)
	plant("l-loop2", LinkUnresolved)
	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		tr.file("locked/.git", "gitdir: ../third/.git\n")
		if err := os.Chmod(tr.at("locked"), 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(tr.at("locked"), 0o700) })
		plant("locked", Unlistable)
	}
	sameLines(t, "at the stop", tr.listed(start), want)

	plants, err := Plants(tr.root, start)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plants {
		if p.Path == "l-chain" && (p.Target != "docs" || !strings.Contains(p.String(), "out of the project (to docs)")) {
			t.Errorf("l-chain says %q, target %q", p, p.Target)
		}
	}
}

// A desk at the project's start that wasn't noted (the core stopped in between) or was noted at another root is no
// start: every .git and every link out is listed, the owner's too. The repo's own .git never is.
func TestWithNoStartEveryGitAndLinkOutIsListed(t *testing.T) {
	tr := newTree(t)
	tr.ownersTree()
	all := []string{line("deep", LinkOutside), line("docs", LinkOutside), line("vendor/lib/.git", GitFile), line("vendor/old/.git", GitFolder)}
	sameLines(t, "no start", tr.listed(nil), all)

	other := newTree(t)
	other.ownersTree()
	sameLines(t, "a start at another root", tr.listed(other.snap()), all)

	// The root's own .git as a file (the owner's checkout a worktree itself) is the repo's, too.
	if err := os.RemoveAll(tr.at(".git")); err != nil {
		t.Fatal(err)
	}
	tr.file(".git", "gitdir: "+tr.outside+"\n")
	sameLines(t, "a gitfile at the root", tr.listed(nil), all)
}

// A folder Smolcorp can't list at the stop is listed, unless it was the same folder, unchanged, at the start, and no
// name in it can be looked up: then nothing in it is reachable, and a desk that opened it to plant something changed
// its ctime, which no program sets back.
func TestAFolderThatCantBeListedIsListedUnlessItIsShutAndUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("no folder this user can't list here")
	}
	tr := newTree(t)
	tr.file("shut/x", "the owner's\n")
	tr.file("half/x", "the owner's\n")
	for rel, mode := range map[string]os.FileMode{"shut": 0, "half": 0o311} {
		if err := os.Chmod(tr.at(rel), mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(tr.at(rel), 0o700) })
	}
	start := tr.snap()
	sameLines(t, "nothing done", tr.listed(start), []string{line("half", Unlistable)})

	time.Sleep(20 * time.Millisecond) // past a coarse ctime's tick (Linux)
	if err := os.Chmod(tr.at("shut"), 0o700); err != nil {
		t.Fatal(err)
	}
	tr.file("shut/.git", "gitdir: ../elsewhere\n")
	if err := os.Chmod(tr.at("shut"), 0); err != nil {
		t.Fatal(err)
	}
	sameLines(t, "opened, planted and shut again", tr.listed(start), []string{line("half", Unlistable), line("shut", Unlistable)})
}

func TestGitNameIsEverySpellingGitTakesForDotGit(t *testing.T) {
	for name, want := range map[string]bool{
		".git": true, ".GIT": true, ".Git": true, ".git.": true, ".git ": true, ".git. .": true, "git~1": true, "GIT~1": true,
		".g\u200cit": true, "\u200e.git": true, ".gi\ufefft": true, ".git\u206a": true,
		"git": false, ".gitignore": false, ".gitmodules": false, "..git": false, "git~2": false, ".git~1": false, "x.git": false, "": false,
	} {
		if got := gitName(name); got != want {
			t.Errorf("gitName(%q) = %v, want %v", name, got, want)
		}
	}
}

// What is listed is held: moved out of the tree for the owner, before anything else runs there, and put back only on
// the owner's word, to a place that is free. Nothing is moved through a link, into the tree, or into a folder that
// was there already.
func TestAHeldPlantLeavesTheTreeAndComesBackOnlyWhereItWas(t *testing.T) {
	tr := newTree(t)
	tr.ownersTree()
	start := tr.snap()
	tr.file("third/.git/config", "[standin]\n\twho = planted\n")
	tr.file("a/b/.git", "gitdir: ../../third/.git\n")
	tr.link("l-out", filepath.Join("..", "outside"))
	plants, err := Plants(tr.root, start)
	if err != nil || len(plants) != 3 {
		t.Fatalf("plants %v, %v", plants, err)
	}

	if _, err := HoldPlants(tr.root, tr.at("held"), plants); err == nil {
		t.Error("held in the working tree")
	}
	if _, err := HoldPlants(tr.root, filepath.Join(tr.root, "src", "..", "held"), plants); err == nil {
		t.Error("held in the working tree, spelled with ..")
	}
	if _, err := HoldPlants(tr.root, "held", plants); err == nil {
		t.Error("held at a path that isn't whole")
	}
	there := filepath.Join(tr.folder, "there")
	if err := os.Mkdir(there, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := HoldPlants(tr.root, there, plants); err == nil {
		t.Error("held in a folder that was there already")
	}
	if got := tr.listed(start); len(got) != 3 {
		t.Fatalf("a refused hold moved something: %q", got)
	}

	heldDir := filepath.Join(tr.folder, "support", "held", "1")
	held, err := HoldPlants(tr.root, heldDir, plants)
	if err != nil || len(held) != 3 {
		t.Fatalf("held %v, %v", held, err)
	}
	if got := tr.listed(start); len(got) != 0 {
		t.Errorf("after the hold, still listed: %q", got)
	}
	for _, h := range held {
		if _, err := os.Lstat(tr.at(h.Path)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is still in the tree (%v)", h.Path, err)
		}
		if _, err := os.Lstat(h.At); err != nil || !under(key(runtime.GOOS, h.At), key(runtime.GOOS, heldDir)) {
			t.Errorf("%s isn't held at %s: %v", h.Path, h.At, err)
		}
	}
	if info, err := os.Stat(heldDir); err != nil || info.Mode().Perm() != 0o700 && runtime.GOOS != "windows" {
		t.Errorf("the held folder: %v %v", info, err)
	}

	gitfile := held[slices.IndexFunc(held, func(h Held) bool { return h.Path == "a/b/.git" })]
	tr.file("a/b/.git", "made since\n")
	if err := Restore(tr.root, gitfile); err == nil {
		t.Error("put back over something")
	}
	if err := os.Remove(tr.at("a/b/.git")); err != nil {
		t.Fatal(err)
	}
	if err := Restore(tr.root, gitfile); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(tr.at("a/b/.git")); string(data) != "gitdir: ../../third/.git\n" {
		t.Errorf("put back: %q", data)
	}
	sameLines(t, "put back, at the next stop", tr.listed(start), []string{line("a/b/.git", GitFile)})
	sameLines(t, "put back, then a new start", tr.listed(tr.snap()), nil)
}

// A plant whose folder was swapped for a link after it was listed isn't moved: the hold would move what the link
// names, outside the tree. The others still are, and the error names it.
func TestAPlantBehindALinkIsntMoved(t *testing.T) {
	tr := newTree(t)
	start := tr.snap()
	tr.file("p/.git", "gitdir: elsewhere\n")
	tr.file("q/.git", "gitdir: elsewhere\n")
	plants, err := Plants(tr.root, start)
	if err != nil || len(plants) != 2 {
		t.Fatalf("plants %v, %v", plants, err)
	}
	owners := filepath.Join(tr.outside, "kept")
	if err := os.MkdirAll(owners, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(owners, ".git"), []byte("the owner's\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tr.at("p"), tr.at("p-aside")); err != nil {
		t.Fatal(err)
	}
	tr.link("p", owners)
	held, err := HoldPlants(tr.root, filepath.Join(tr.folder, "held"), plants)
	if err == nil || !strings.Contains(err.Error(), "p/.git isn't held") {
		t.Errorf("no error for the plant behind a link: %v", err)
	}
	if len(held) != 1 || held[0].Path != "q/.git" {
		t.Errorf("held %v", held)
	}
	if data, _ := os.ReadFile(filepath.Join(owners, ".git")); string(data) != "the owner's\n" {
		t.Errorf("the owner's file outside the tree: %q", data)
	}
	if err := Restore(tr.root, Held{Plant: Plant{Path: "p/.git"}, At: filepath.Join(tr.folder, "held", "1", ".git")}); err == nil {
		t.Error("put back through a link")
	}
	for _, rel := range []string{"../x/.git", "/x/.git", "a/../../x", "a//b", ""} {
		if _, err := inTree(tr.root, rel); err == nil {
			t.Errorf("inTree(%q) took it", rel)
		}
	}
}
