package desktop

import (
	"errors"
	"os"
	"strings"
)

// A hire into a General Manager's own places (FXGMHIRENOGIT126, the narrowed rule for 1.26.0): whoever asks for
// it, the GM's token, the office, Setup or the page, the core runs no git there, nor in the repo git would find from
// there. No discovery, no config read, no worktree, branch or registration, no upstream, no submodule, no folder for
// its hires: the hire works in the folder as it is, as in a folder that isn't a repo, and its fire runs no git there
// either. A GM writes there as its own work, so it can plant what holds git or swaps a folder under it (a FIFO, a
// link), and the core's git would then wait on it, or write where the GM chose, while every other hire waits
// (d.hiring).
//
// A GM's own places are where it works and writes as its own: its workspace and its plans folder (a GM hired into a
// folder of projects), or the worktree or checkout it works in (a GM in a repo), and everything below them. The
// projects it reads aren't: a hire there, the owner's repos and a held GM's project included, keeps the full flow,
// and its folder for hires is held from the GM (deskPlaces' GMHires). The folder named for the hire is judged, and
// so is every folder on git's way up from it to the top git would find (the first that holds a .git entry, looked at
// with Lstat alone), and the git folders git would read from there: where that .git leads, its links and a .git
// file's gitdir followed, and the shared folder its commondir names, each read without waiting. Only on a Mac, where desks start in their sandbox: elsewhere a GM runs in none yet, and every
// folder is as much its as the owner's (D2).

// gmFolderLine is said to whoever hired (the hire's card, Setup's and the GM route's answers) whenever the rule
// applies; the hire is told gmFolderHireLine, in its first message.
const gmFolderLine = "This folder is your General Manager's, so Smolcorp doesn't run git here: the hire gets no worktree or branch of its own. For those, hire into one of your own repositories."

// gmFolderHireLine is gmFolderLine as the hire is told it. Its sandbox holds every GM's own places from it as from
// any other desk, so it can read there but not change anything (FXGMHIRENOGIT126R1's R6).
const gmFolderHireLine = "This folder is the General Manager's, so Smolcorp doesn't run git here, and you can read it but not change it."

// gmOwnsFolder is whether folder, or a folder on git's way up from it, is in an open General Manager's own places:
// the rule above. A variable, for the tests' stand-ins; gmOwnsFolderNow is the real one.
var gmOwnsFolder = gmOwnsFolderNow

// placedInGMFolder is whether a hire's notes say it was placed under the rule (its row keeps the line), so its later
// starts and its stop run no git there either, whatever the GMs are by then. A row whose line was lost is judged by
// the rule again at each start (deskPlaces).
func placedInGMFolder(notes string) bool { return strings.Contains(notes, gmFolderLine) }

// plainFileData is a small plain file's bytes, read without waiting: a FIFO, socket or device at path (or at the end
// of a link there) is never opened, and one swapped in after the look is refused by cpReadData's own check.
func plainFileData(path string, max int64) ([]byte, error) {
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, errors.New("not a plain file")
	}
	return cpReadData(path, max)
}
