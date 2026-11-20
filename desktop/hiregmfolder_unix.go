//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// gmOwnsFolderNow is the rule (hiregmfolder.go), asked of the registry as it is now: folder, resolved, or a folder on
// git's way up from it to the top git would find, or a git folder git would read from that top, is in the own places
// of a General Manager the registry holds open. Nothing is run and nothing is waited on: the registry, one Lstat of
// each folder's .git on the way, then the top's .git and commondir followed (gitFoldersFromTop). A GM whose
// places can't be known counts as owning every folder, so the hire runs no git rather than git where a GM may wait
// for it. With no GM open, or where no desk starts in its sandbox, false.
func gmOwnsFolderNow(folder string) bool {
	if !deskSandboxed || folder == "" {
		return false
	}
	var places []string
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Role != gmRole || !isOpen(h) {
			continue
		}
		own, known := gmOwnPlaces(h)
		if !known {
			return true
		}
		places = append(places, own...)
	}
	if len(places) == 0 {
		return false
	}
	owned := func(at string) bool {
		return slices.ContainsFunc(places, func(place string) bool { return sandbox.Inside(runtime.GOOS, at, place) })
	}
	for dir := physicalBelow(folder); ; dir = filepath.Dir(dir) {
		if owned(dir) {
			return true
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			// The top git would find: and the git folders it would read from there (FXGMHIRENOGIT126R2, O1), so a .git
			// outside every GM's place that names a git folder in one is judged by where git would go.
			return slices.ContainsFunc(gitFoldersFromTop(dir), func(at string) bool { return owned(at) || owned(physicalBelow(at)) })
		}
		if filepath.Dir(dir) == dir {
			return false // the top of the disk
		}
	}
}

// gitFoldersFromTop are the places git goes through from top's .git to the git folders it reads, as the rule looks at
// them, with nothing run and nothing waited on: each place on the way from the .git (gitDotGitHops: each link, and a
// .git file's gitdir line, read without waiting), and the shared git folder that git folder's commondir names, read so
// too. Each as the disk resolves its parent, its own name as given (gitHopPlace).
func gitFoldersFromTop(top string) []string {
	hops := gitDotGitHops(top)
	gitDir := hops[len(hops)-1]
	if common, found := gitdirNamed(gitDir, filepath.Join(gitDir, "commondir"), ""); found {
		hops = append(hops, gitHopPlace(common))
	}
	return hops
}

// gmOwnPlaces are GM h's own places, resolved, as its Places spell them (deskPlaces): a GM hired into a folder of
// projects works in its workspace and writes its plans folder there; a GM in a repo works in its worktree, or in the
// checkout. Not known for a GM whose places can't be spelled from its row.
//
// The GM's row is read as stored (storedOnly): these are places the core keeps its git out of, compared only.
func gmOwnPlaces(h Hire) ([]string, bool) {
	folder, tree := h.Folder.storedOnly(), h.Worktree.storedOnly()
	if workspace := gmWorkspace(true, h.ID, h.Repo, tree); workspace != "" {
		if folder == "" {
			return nil, false
		}
		return []string{sandbox.Workspace(physical(supportDir), h.ID),
			filepath.Join(physicalBelow(folder), filepath.FromSlash(deskPlansFor(h.ID)))}, true
	}
	switch {
	case tree != "":
		return []string{physicalBelow(tree)}, true
	case h.Repo != "" && folder != "":
		return []string{physicalBelow(folder)}, true
	}
	return nil, false
}
