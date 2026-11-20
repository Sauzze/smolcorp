package desktop

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
)

// hirePath is a path a hire's row keeps: its worktree (Hire.Worktree) or the folder it was hired into (Hire.Folder).
// Its value can't be read as a string, so nothing acts at it by mistake (FXCOREWORKTREES126, item 10): whoever can
// write a folder on its way (a GM, for the owner's folders) could have turned it into a link since the hire, and git, a
// window, T3 Code or a removal there would run where the link leads. Every act goes through the hire's checked
// accessors (Hire.checkedTree, Hire.checkedFolder, Hire.checkedPlace, Hire.checkedStart), which give the path only
// while it still resolves to itself; a comparison, a look-up or the words for the owner use storedOnly, which says
// so by its name. In the registry file it is the plain string it always was.
type hirePath struct{ stored string }

// keptPath is path as a hire's row keeps it.
func keptPath(path string) hirePath { return hirePath{stored: path} }

// storedOnly is the path as the row keeps it, unchecked: for comparisons, look-ups and the owner's words only, never
// to act at (run git or another program there, open, read, write or remove anything there, or hand it to an app).
func (p hirePath) storedOnly() string { return p.stored }

// none is whether the row keeps no path here.
func (p hirePath) none() bool { return p.stored == "" }

func (p hirePath) MarshalJSON() ([]byte, error) { return json.Marshal(p.stored) }

func (p *hirePath) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	p.stored = ""
	if s != nil {
		p.stored = *s
	}
	return nil
}

// hireMoved is an act at a hire's worktree or folder refused because it no longer resolves to the path its row keeps:
// its words say so plainly, for the owner.
type hireMoved struct{ words string }

func (e *hireMoved) Error() string { return e.words }

// movedTreeWords and movedFolderWords are hireMoved's words for a hire's worktree and for the folder a hire with no
// worktree works in: what Smolcorp did (nothing there) and what to do.
func movedTreeWords(tree string) string {
	return "This hire's worktree, " + shownPath(tree) + ", was moved or replaced since it was hired, so Smolcorp left it alone: " +
		"nothing ran or opened there. Fire it, then hire it again."
}

func movedFolderWords(folder string) string {
	return "This hire's folder, " + shownPath(folder) + ", was moved or replaced since it was hired, so Smolcorp left it alone: " +
		"nothing ran or opened there. Fire it, then hire it again."
}

// checkedTree is h's worktree as its row keeps it, only while it still resolves to itself (resolvesToItself): "" with
// no error for a hire with no worktree. A turned one is a hireMoved with its words.
func (h Hire) checkedTree() (string, error) {
	tree := h.Worktree.stored
	if tree == "" {
		return "", nil
	}
	if !resolvesToItself(tree) {
		return "", &hireMoved{movedTreeWords(tree)}
	}
	return tree, nil
}

// checkedFolder is, for a hire with no worktree, the folder it works in, only while it still resolves where the rule
// judged it at its hire (Hire.FolderResolved, on a Mac), and then in that very spelling: what is done there goes where
// the hire was placed, never where a link put on the folder's way since leads. A row from before 1.26.0 keeps no
// resolved folder: its folder is given as its row keeps it (D21, as ruled; its first start keeps one). For a hire with
// a worktree, the folder it was hired into (the owner's project, which the hire doesn't work in) is a look-up only:
// storedOnly.
func (h Hire) checkedFolder() (string, error) {
	folder := h.Folder.stored
	if folder == "" || runtime.GOOS != "darwin" || h.FolderResolved == "" {
		return folder, nil
	}
	if real, ok := resolvedFolder(folder); !ok || real != h.FolderResolved {
		return "", &hireMoved{movedFolderWords(folder)}
	}
	return h.FolderResolved, nil
}

// checkedPlace is where h works, checked: its worktree when it has one (checkedTree), else its folder (checkedFolder).
func (h Hire) checkedPlace() (string, error) {
	if !h.Worktree.none() {
		return h.checkedTree()
	}
	return h.checkedFolder()
}

// checkedStart is the folder h starts in, checked: in its worktree, the folder below it that matches the one it was
// hired into in its repo (a hire into <repo>/sub starts in <tree>/sub); else its folder (checkedFolder).
func (h Hire) checkedStart() (string, error) {
	if h.Worktree.none() || h.Repo == "" {
		return h.checkedFolder()
	}
	tree, err := h.checkedTree()
	if err != nil {
		return "", err
	}
	return startBelow(tree, h.Folder.stored, h.Repo), nil
}

// startBelow is the folder a hire with worktree tree starts in: tree, or the folder in it that folder (the one it was
// hired into) is in repo. folder is only compared here.
func startBelow(tree, folder, repo string) string {
	if rest, inside := strings.CutPrefix(physical(folder), physical(repo)+string(filepath.Separator)); inside {
		return filepath.Join(tree, rest)
	}
	return tree
}

// storedStart is where h starts as its row keeps it, unchecked (startedIn's old answer): for comparisons and words
// only, never to act at.
func (h Hire) storedStart() string {
	if h.Worktree.none() || h.Repo == "" {
		return h.Folder.stored
	}
	return startBelow(h.Worktree.stored, h.Folder.stored, h.Repo)
}

// storedPlace is where h works as its row keeps it (its worktree, else its folder), unchecked: for comparisons and
// words only, never to act at.
func (h Hire) storedPlace() string {
	if !h.Worktree.none() {
		return h.Worktree.stored
	}
	return h.Folder.stored
}

// removeHirePlans takes away hire h's plans folder where it starts (a hire with no worktree, a GM), at its fire or a
// failed start: only while that folder still resolves where it was placed (checkedStart), and then through a root
// opened with no link on its way when its row keeps it resolved, never through a link put on the folder since
// (FXGMHIRENOGIT126R8's found 5). A folder moved or replaced keeps them: they're Smolcorp's own, and harmless.
func removeHirePlans(h Hire) {
	folder, err := h.checkedStart()
	if err != nil || folder == "" {
		return
	}
	if h.FolderResolved != "" && folder == h.FolderResolved {
		if root, err := openResolvedRoot(folder); err == nil {
			_ = removeDeskPlansIn(root, h.ID)
			root.Close()
		}
		return
	}
	_ = removeDeskPlans(folder, h.ID)
}
