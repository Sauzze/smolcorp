package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The core's own folder for hires' worktrees (FXCOREWORKTREES126). On a Mac, every new hire's worktree is made in
// ~/.smolcorp/worktrees/<repo>-<short id>/<hire>, a folder the core makes for itself, its owner's alone (0700), and
// one no General Manager may write: every GM's Policy denies writing anywhere in it, and moving the folders above it
// (Places.Worktrees), and no GM's own place may be in it. So a GM can't swap a hire's tree for a link to a folder of its
// own, which made the core's git, a window or T3 Code run there (F-8): the tree's side is closed by where it is, and
// every act at a hire's tree or folder still goes through its checked accessors (hirepath.go).
//
// Elsewhere (Linux, Windows) no desk starts in a sandbox yet, so no GM is held from any folder: hires' worktrees are
// still made beside the repo, in <repo>-hires, as before. A hire made before this version keeps its tree where it is
// (beside the repo, or in the configured folder) until it's fired; its every start is checked as before (deskTreeWords).

// coreWorktreesOn is whether new hires' worktrees go in the core's own folder: on a Mac.
func coreWorktreesOn() bool { return runtime.GOOS == "darwin" }

// coreWorktreesFolder is the core's own folder for hires' worktrees, spelled from the home folder resolved:
// ~/.smolcorp/worktrees.
func coreWorktreesFolder() string {
	return filepath.Join(physical(homeDir), ".smolcorp", "worktrees")
}

// coreRepoName is the name of repo's folder in the core's folder for worktrees: its own name, then the first 8 hex
// digits of its resolved path's SHA-256, so two repos with one name never share a folder (~/code/app and ~/work/app).
func coreRepoName(repo string) string {
	sum := sha256.Sum256([]byte(physical(repo)))
	return hireSlug(filepath.Base(repo)) + "-" + hex.EncodeToString(sum[:])[:8]
}

// coreRepoFolder is where repo's hires' worktrees are made in the core's own folder.
func coreRepoFolder(repo string) string {
	return filepath.Join(coreWorktreesFolder(), coreRepoName(repo))
}

// legacyHiresFolder is where a repo's hires' worktrees were made before this version, and still are off a Mac and for a
// General Manager's own worktree: <repo>-hires beside the repo.
func legacyHiresFolder(repo string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-hires")
}

// treeFolders are every folder a hire's worktree of repo may be right in: the one a new hire's is made in
// (hiresFolder), the core's own (on a Mac), the configured one (worktrees, when set) and the one beside the repo, for a
// hire made before. Each as hiresFolder spells it.
func treeFolders(repo, worktrees string) []string {
	folders := []string{hiresFolder(repo, worktrees), legacyHiresFolder(repo)}
	if coreWorktreesOn() {
		folders = append(folders, coreRepoFolder(repo))
	}
	if worktrees != "" {
		folders = append(folders, filepath.Join(worktrees, filepath.Base(repo)))
	}
	slices.Sort(folders)
	return slices.Compact(folders)
}

// treeRightIn is whether tree (resolved) is right in a folder for hires' worktrees of repo (treeFolders), or, on a Mac,
// right in any repo's folder in the core's own folder: no desk can write there, so every folder in it is the core's,
// and a repo the owner moved since (its resolved path, so its folder's name, changed) keeps its hires' trees.
func treeRightIn(tree, repo, worktrees string) bool {
	parent := filepath.Dir(tree)
	if slices.ContainsFunc(treeFolders(repo, worktrees), func(folder string) bool { return parent == physical(folder) }) {
		return true
	}
	return coreWorktreesOn() && filepath.Dir(parent) == coreWorktreesFolder()
}

// errCoreWorktrees is the core's folder for worktrees refused: something else is at its name.
var errCoreWorktrees = errors.New("Smolcorp's own folder for hires' worktrees, ~/.smolcorp/worktrees, isn't a folder of its own: " +
	"something else is at its name (a link, a file, or a folder of another user's). Move it away, then hire again.")

// hireTreesMode is the mode a folder for hires' worktrees is made with: the owner's alone (0700) in the core's own
// folder, 0755 elsewhere, as before.
func hireTreesMode(folder string) os.FileMode {
	if coreWorktreesOn() && sandbox.Inside(runtime.GOOS, folder, coreWorktreesFolder()) {
		return 0o700
	}
	return 0o755
}

// hireWorkWords are a hire's first message's words on where its work is (FXCOREWORKTREES126, item 8): its own worktree,
// tree, on branch, and how its owner looks at it, that folder or `git diff main...<branch>` from their own checkout. On
// a Mac every new hire's worktree is in ~/.smolcorp/worktrees, a folder the owner doesn't go to by habit, so the hire
// says so when it introduces itself. "" for a hire with no worktree.
func hireWorkWords(tree, branch string) string {
	if tree == "" || branch == "" {
		return ""
	}
	return "Your work is in your own worktree, " + shownInWords(shownPath(tree)) + ", on the branch " + branch + ". " +
		"When you introduce yourself, tell your owner so: they can open that folder, or run git diff main..." + branch +
		" from their own checkout (with their default branch's name in place of main, if it has another)."
}
