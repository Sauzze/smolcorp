package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The default branch's pin (FXGITROWS2, RT126SWEEP2's B2). A repo's default branch is what its origin/HEAD names, a
// file in the shared .git. A desk that pointed it at another branch took the default branch out of the next start's
// Policy and out of the fire's look, and then moved it. So the core reads it once per repo, the first time it needs
// it, and keeps it in the support folder, which no desk reads or writes: every later start, fire and check holds the
// pin, whatever origin/HEAD names then. The profile denies every desk origin/HEAD and its lock as well
// (internal/sandbox/policy_gitrows2.go).
//
// The owner's own change is followed (FXGITROWS2R1, RT126FIXREVIEW-GITROWS2's N1). Each start, fire and check also
// takes the branch origin/HEAD names now, and holds both: the pinned one and that one, as main, master and release
// are. On a Mac no desk writes origin/HEAD, so what it names now is the owner's (`git remote set-head`, a clone's new
// default); where desks have no sandbox, a desk that changed it only adds a branch to the ones held, and the pin is
// still held. The pin stays until the owner takes the repo's line out of default-branches.json in Smolcorp's support
// folder (or the file): then the next read pins origin/HEAD as it is. A repo with no origin/HEAD yet pins nothing, and
// is read again next time.

// defaultBranchesName is the pins' file in the support folder: {"<the repo's shared .git, resolved>": "<branch>"}.
const defaultBranchesName = "default-branches.json"

// defaultBranches serializes the pins' reads and writes within this core.
var defaultBranches sync.Mutex

// pinnedDefaultBranches are the default branches of the repo whose shared .git is gitDir: pinned, its pin, or, the
// first time, what read gives (origin/HEAD, as the caller reads it), pinned then; and now, what read gives when it
// isn't the pin ("" when it is). Each is "" when there is none, or a name the Policy doesn't take
// (sandbox.BranchName): a name it refused would stop every start in the repo. A pin that can't be kept is still used
// for this call, and logged; one that can't be read is never replaced, so a file the owner broke pins nothing a desk
// chose, and only origin/HEAD as it is now is held.
func pinnedDefaultBranches(gitDir string, read func() string) (pinned, now string) {
	if now = read(); !sandbox.BranchName(now) {
		now = ""
	}
	other := func(pin string) string {
		if strings.EqualFold(now, pin) {
			return ""
		}
		return now
	}
	repo := physical(gitDir)
	defaultBranches.Lock()
	defer defaultBranches.Unlock()
	file := filepath.Join(supportDir, defaultBranchesName)
	pins := map[string]string{}
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		log.Printf("sandbox: the default branches' pins couldn't be read: %v", err)
		return "", now
	default:
		if err := json.Unmarshal(data, &pins); err != nil {
			log.Printf("sandbox: the default branches' pins (%s) can't be read: %v", defaultBranchesName, err)
			return "", now
		}
	}
	if pinned, ok := pins[repo]; ok {
		if !sandbox.BranchName(pinned) {
			return "", now // only the owner writes this file: a broken line pins nothing
		}
		return pinned, other(pinned)
	}
	if now == "" {
		return "", ""
	}
	pins[repo] = now
	if data, err := json.MarshalIndent(pins, "", "  "); err != nil {
		log.Printf("sandbox: the default branch of %s couldn't be pinned: %v", repo, err)
	} else if err := writePrivate(file, append(data, '\n')); err != nil {
		log.Printf("sandbox: the default branch of %s couldn't be pinned: %v", repo, err)
	}
	return now, ""
}

// deskDefaultBranches are the default branches of the repo whose shared .git is gitDir, for a desk's Places
// (DefaultBranch, DefaultBranchNow): pinned, and what its origin/HEAD names now when that is another.
func deskDefaultBranches(gitDir string) (pinned, now string) {
	return pinnedDefaultBranches(gitDir, func() string { return sandbox.DefaultBranch(gitDir) })
}

// deskBranchWords are a desk's start words on its branches (FXGITROWS2R5, an item for the owner), for a desk in a
// worktree of its own on branch, on goos: "" for none. Its sandbox refuses every other branch, every tag and git stash
// with git's own "Operation not permitted" alone, so it is told plainly what is its own and what to do instead. Only on
// a Mac, whose rows these are, and only for an agent its sandbox holds (sandboxed): the rows hold no other, and to
// one they don't hold the words would be untrue. Every engine that makes a hire on its own branch asks here
// (FXGITROWS2R6, RT126FIXREVIEW-GITROWS2R5's O1); the launcher's (--hire-id) and T3 Code's ask with sandboxed false,
// as no sandbox holds their agents, so theirs are told nothing. A desk with no worktree of its own can't commit in the
// shared .git. The General Manager (gm) is told to ask the owner: it is the one the others ask (O2).
func deskBranchWords(goos string, sandboxed, gm bool, branch string) string {
	if goos != "darwin" || !sandboxed || branch == "" {
		return ""
	}
	own := "your own branch, " + branch
	if folder := filepath.ToSlash(filepath.Dir(branch)); strings.Count(branch, "/") >= 2 {
		own = "your own branches, " + branch + " and any new one under " + folder + "/"
	}
	ask := "ask the General Manager or the owner"
	if gm {
		ask = "ask the owner"
	}
	return "You can commit only to " + own + ": other branches, tags and git stash aren't open to you here, so save work in progress as a commit on your branch, and " + ask + " when you need another branch or a tag."
}

// deskStartWords are the paragraph a sandboxed desk's start words end with: sentences, each on its place, one after
// the other, so they read as one paragraph; "" when all are "". At the stack with FXHIREPROJECTS they are
// deskStartWords(deskProjectsWords(…), deskBranchWords(…)): where it is, then what it commits to.
func deskStartWords(sentences ...string) string {
	var said []string
	for _, s := range sentences {
		if s != "" {
			said = append(said, s)
		}
	}
	if len(said) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(said, " ")
}
