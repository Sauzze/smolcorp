package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// The checkpoint store (1.26 C1): a snapshot of a desk's working folder, kept inside its repo, so a later piece can
// say what a turn changed and put it back ("undo this turn", C2 to C4). This file is the store only: no route, no
// screen, and nothing calls it yet.
//
// A snapshot is the working tree as it is on disk, tracked and untracked files alike, with .gitignore (the repo's, its
// .git/info/exclude and the owner's own ignore file) respected. It is made without touching what the owner has:
//
//	a private git folder (cpPrivate, made new for each snapshot in the app's support folder, with its own index and
//	object store, reading the repo's objects as an alternate) → read-tree HEAD → add -A → write-tree → commit-tree → the new objects put
//	in the repo's own store, through its git folder opened once (importObjects) → update-ref
//	refs/smolcorp/checkpoints/<session>/<n> in the repo
//
// (read-tree HEAD first so a file the repo tracks but its .gitignore also names is still in the snapshot.) The owner's
// index, HEAD, branches, tags, stash, reflogs and config are never written: the only things written are new objects in
// the object store (which git's own gc sweeps once nothing points at them) and the checkpoint refs. The commit has
// no parent and a fixed author, and says its label and the commit HEAD was at, on one line.
//
// It runs nothing the repo names, by the rules of the reads of FG1 (gitstate.go), which it builds on:
//   - every command runs in gitReadEnv: no owner or system config, no transport, no prompt, no other repo named;
//   - and with gitReadArgs: core.hooksPath is the null device (so no hook runs: reference-transaction on update-ref,
//     post-index-change on add), core.fsmonitor is blank, no attributes file, no pager, no external diff;
//   - the repo's config is read first (config --list -z) and the repo is refused whole at a key the store doesn't know
//     (gitRepoConfig), and every filter, text converter and merge driver it names is blanked by name (clean, smudge,
//     process; required is turned off) for the commands that run in the repo (none of which reads a file's content);
//   - add, the one command that reads the files, never opens the repo's config or attributes at all, so what they say
//     when it runs (a filter written in since the look, say) can't matter: it runs in a private git folder the store
//     wrote, by construction (cpPrivate); git 2.40 and later is also given the empty tree as the attribute source, so
//     no .gitattributes of the working folder is read either;
//   - on top, for what a write needs: commit.gpgsign=false (a snapshot is no signed commit; gpg.program is blanked, so
//     a signature could only fail), core.logAllRefUpdates=false (no reflog is started for a checkpoint ref), gc.auto=0
//     (nothing is collected under a snapshot) and core.autocrlf=false and core.safecrlf=false (a file's line endings
//     are kept as on disk), core.splitIndex=false (the private index is written whole: a split one would leave a
//     sharedindex.* file in the owner's git folder for every snapshot). The identity comes from the environment, so
//     neither user.* nor the owner's own config is asked;
//   - a submodule is never shown to add, whatever is on disk: add would ask it whether it is dirty, by running git in it,
//     with its own config and attributes, which the store never judges. Each submodule's commit is recorded by hand;
//   - every command names its repo and working folder (--git-dir, --work-tree, links resolved once) after the first
//     look, the refs are read and written in the common git folder named by itself (so not even a linked worktree's
//     commondir file is read for them), and a repo whose working folder isn't the desk's (core.worktree, a .git file
//     naming a git folder of somewhere else) is refused (settle);
//   - no git ever writes in the repo (T126CK1). The snapshot's objects are written in its private store; the store
//     itself puts them in the repo's, and writes and drops its refs there, by hand (createRef, deleteRef: git's own
//     way for a loose ref, its lock file made new and renamed in). Each write goes through the common git folder opened
//     (os.Root) and checked to be the very one settle pinned, by what the open folder says it is (its device and inode,
//     and those of its objects folder), never through a path: a .git swapped for another repo, even for a moment while
//     a write runs, can't send it there. (A git given the path, update-ref, looks the path up again for each file it
//     touches, so a swap in the middle of its run wrote a ref, or the folders for one, in the other repo.) A .git that
//     isn't the one pinned, before a write or right after a ref is put in, is refused: nothing is kept, and it is said
//     in words and in the log. A reftable repo, whose refs the store can't write by hand, is refused in words. In a repo
//     shared by core.sharedRepository, each folder and ref the store makes gets the mode git would give it (cpSharedMode);
//     a value of that setting the store can't follow exactly as git does is refused in words.
//
// Never blocking the desk: one call runs at most checkpointTimeout, and at most checkpointJobs snapshots run at once
// (one waits for its turn inside that time, and a second for the same desk waits for the first). A write of a ref runs
// no git, so it needs no deadline and leaves no lock file of its own behind. A lock file found in its way is said in
// words, never taken for the repo being wedged. A repo with more
// files or bytes than the caps, a git that is missing or too old, a repo config with a key the store doesn't know, any
// step that fails: each is skipped, and the answer says why in words for the owner. Nothing is half made: a failed step
// leaves no ref (a snapshot that missed a file would make a later undo wrong, so none is kept). C2 should still call
// snapshot off the hook's own goroutine, so even those seconds are not the turn's.

const (
	checkpointRefs     = "refs/smolcorp/checkpoints/"
	checkpointJobs     = 2   // snapshots run at once, over every desk
	checkpointLabelMax = 200 // bytes of a label kept
	checkpointSubject  = "smolcorp-checkpoint v1 "
	checkpointName     = "Smolcorp checkpoint"
	checkpointEmail    = "checkpoint@smolcorp.invalid"
)

// The limits of the store; variables so a test can lower them.
var (
	checkpointTimeout  = 45 * time.Second // one snapshot, its wait for a free slot included
	checkpointDeskWait = 50 * time.Second // how long fire and prune wait for a snapshot of the desk that is running (its cap and a little)
	checkpointsKept    = 40               // per desk; a turn takes two (its start and its end), so about twenty turns
	checkpointMaxFiles = 20000            // files a snapshot would read; more and it is skipped
	checkpointMaxBytes = int64(300 << 20) // and the sum of their sizes
)

var checkpointID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// checkpointDesk is the desk a snapshot is of: its session's id and the folder it works in (a hire's own worktree).
type checkpointDesk struct {
	Session string
	Folder  string
}

// Checkpoint is one snapshot. ID is the commit's id, what between takes. N counts a desk's snapshots up from 1 and is
// what the ref is named with. Head is the commit HEAD was at ("" before the first commit): two snapshots with
// different heads mean a commit was made between them, which C4 refuses to undo.
type Checkpoint struct {
	ID    string `json:"id"`
	N     int    `json:"n"`
	Label string `json:"label"`
	Head  string `json:"head,omitempty"`
	At    int64  `json:"at"` // milliseconds (git keeps whole seconds)
}

// CheckpointChange is one path that differs between two snapshots: Status is A (added), M (modified), D (deleted)
// or T (its kind changed, a file that became a link).
type CheckpointChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// checkpointStore makes, lists, compares and forgets desks' snapshots.
type checkpointStore struct {
	jobs chan struct{} // a slot to run a snapshot in

	mu    sync.Mutex
	desks map[string]chan struct{} // one call at a time per session, so a number is never handed out twice
	where map[string]string        // a session's repo git folder as last seen, for forget once its folder is gone

	swept sync.Once // the private folders a crash left behind are removed before the first snapshot
}

func newCheckpoints() *checkpointStore {
	return &checkpointStore{jobs: make(chan struct{}, checkpointJobs), desks: map[string]chan struct{}{}, where: map[string]string{}}
}

// checkpoints is the one the app uses.
var checkpoints = newCheckpoints()

var safeSessionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$`)

// sessionRefName is the folder of a desk's refs. An id that is plain (letters, digits, dot, dash, underscore, no ".."
// and not ending in ".lock" or ".", and with no capital letter: a disk that ignores case would give "Abc" and "abc" one
// folder) is used as it is; any other is a hash of it, behind a "_" no plain id starts with.
func sessionRefName(session string) string {
	if safeSessionID.MatchString(session) && !strings.Contains(session, "..") && !strings.HasSuffix(session, ".lock") && !strings.HasSuffix(session, ".") && session == strings.ToLower(session) {
		return session
	}
	sum := sha256.Sum256([]byte(session))
	return "_" + hex.EncodeToString(sum[:8])
}

// enter takes the desk's turn, or gives up when ctx ends. The caller calls the returned function when done.
func (s *checkpointStore) enter(ctx context.Context, session string) (leave func(), ok bool) {
	s.mu.Lock()
	gate := s.desks[session]
	if gate == nil {
		gate = make(chan struct{}, 1)
		s.desks[session] = gate
	}
	s.mu.Unlock()
	if ctx.Err() != nil {
		return nil, false
	}
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, true
	case <-ctx.Done():
		return nil, false
	}
}

// cpSkip is why the store did nothing, in words for the owner.
type cpSkip string

func (e cpSkip) Error() string { return string(e) }

// cpWhy turns what a git run came back with into a skip's words.
func cpWhy(err error, fallback string) error {
	var said cpSkip
	switch {
	case errors.As(err, &said): // words already
		return said
	case errors.Is(err, errGitTooBig):
		return cpSkip("Git's answer was too big to take a checkpoint of this folder.")
	case errors.Is(err, exec.ErrNotFound):
		return cpSkip("Git isn't installed on this computer, so no checkpoint was taken.")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return cpSkip("Taking a checkpoint took too long here, so it was skipped.")
	}
	return cpSkip(fallback)
}

// cpRepo is a repo found from a desk's folder, with what every command of it is run with.
type cpRepo struct {
	dir    string   // where commands run: the repo's top, or a scratch folder when only its git folder is known
	top    string   // the working tree's top, links resolved; "" when only the git folder is known (forget after the folder went)
	gitDir string   // the repo's git folder (a linked worktree's own), links resolved, a real folder
	common string   // the repo's common git folder, where the refs and the objects are, links resolved
	bind   []string // --git-dir (and --work-tree) of every command: the repo is named, never found again from a folder
	refs   []string // --git-dir of the commands that read or write the checkpoint refs: the common folder itself
	call   gitCall  // the global options and the config for every command

	dotGit     string      // the .git that made the desk's folder a repo, and what it was when looked at (B6)
	dotGitWas  os.FileInfo // (nil when only the git folder is known)
	commonWas  os.FileInfo // the common git folder as an open folder saw it in open: the one every write goes in (nil: none)
	objectsWas os.FileInfo // and its objects folder, then: an inode used again by a new folder isn't the repo pinned
	format     string      // the repo's hash: sha1 or sha256
	compat     string      // extensions.compatobjectformat, if the repo has one
	keep       [][2]string // the repo's settings that shape what a snapshot holds (cpKeptKeys), carried into its private git folder
	shared     int         // its core.sharedRepository as git reads it (cpSharedPerm): 0 when the umask decides the modes
}

// git runs a command with the repo's settings; extra is environment for that one command.
func (r *cpRepo) git(ctx context.Context, extra []string, args ...string) ([]byte, int, error) {
	return r.gitIn(ctx, r.dir, r.bind, extra, args...)
}

// refGit runs a command on the refs, in the common git folder by its resolved path. (Not the worktree's git folder with
// GIT_COMMON_DIR: git's ref store would still follow the worktree's commondir file, and the objects the variable.)
func (r *cpRepo) refGit(ctx context.Context, args ...string) ([]byte, int, error) {
	return r.gitIn(ctx, r.dir, r.refs, nil, args...)
}

// gitIn is git with the folder and the repo named by the caller: after the first look, so that a folder's own .git, or
// a core.worktree in the config, never decides which repo a command runs in.
func (r *cpRepo) gitIn(ctx context.Context, dir string, bind, extra []string, args ...string) ([]byte, int, error) {
	env := append(gitReadEnv(os.Environ(), r.call.config), extra...)
	argv := append([]string{"-C", dir}, bind...)
	argv = append(append(argv, r.call.global...), args...)
	return execGit(ctx, env, argv...)
}

var (
	errDotGitMoved   = cpSkip("This desk's .git changed while its checkpoints were being kept, so nothing was written.")
	errReftable      = cpSkip("This repo keeps its refs in a reftable, which the checkpoints can't write safely, so none was taken.")
	errPackedLocked  = cpSkip(lockWords("packed-refs.lock"))
	errRefsNotFolder = cpSkip("This repo's .git folder has a link or a file where a folder of the checkpoints' refs goes, so none was written there.")
)

// cpSay puts a line in the app's log; a variable so a test can read it.
var cpSay = func(text string) { log.Printf("smolcorp: checkpoints: %s", text) }

// cpBeforeRefWrite, when a test sets it, runs between the look at the folder pinned and a write of a ref through it:
// the worst moment for a swap, and a stand-in for a slow disk.
var cpBeforeRefWrite func()

// moved says the desk's .git is no longer the one the store looked at in open (gone, or another file or folder).
func (r *cpRepo) moved() bool {
	if r.dotGitWas == nil {
		return false
	}
	now, err := os.Lstat(r.dotGit)
	return err != nil || !os.SameFile(now, r.dotGitWas)
}

// pinFolder is what a common git folder is, as an open folder of it says (not a path looked up a second time): the
// folder itself and its objects folder.
func pinFolder(path string) (folder, objects os.FileInfo, err error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	if folder, err = root.Stat("."); err == nil {
		objects, err = root.Lstat("objects")
	}
	return folder, objects, err
}

// pinned opens the repo's common git folder for a write, and gives it back only if it is the one pinned in open (the
// open folder and its objects folder are the very ones) and the desk's .git is still the one looked at. Anything else
// is errDotGitMoved, said in the log too. Every write goes through the folder given back, never through a path.
func (r *cpRepo) pinned() (*os.Root, error) {
	if r.commonWas == nil || r.objectsWas == nil {
		return nil, errors.New("no pinned git folder")
	}
	if root, err := os.OpenRoot(r.common); err == nil {
		here, errHere := root.Stat(".")
		objects, errObjects := root.Lstat("objects")
		if errHere == nil && errObjects == nil && os.SameFile(here, r.commonWas) && os.SameFile(objects, r.objectsWas) && !r.moved() {
			return root, nil
		}
		root.Close()
	}
	r.sayMoved()
	return nil, errDotGitMoved
}

func (r *cpRepo) sayMoved() {
	cpSay("the .git of " + cmpOr(r.top, r.common) + " is no longer the repo this desk's checkpoints were pinned to, so nothing was written in it")
}

// refRoot is pinned for a write of a ref, which a reftable repo can't take by hand (errReftable).
func (r *cpRepo) refRoot() (*os.Root, error) {
	root, err := r.pinned()
	if err != nil {
		return nil, err
	}
	if _, err := root.Lstat("reftable"); err == nil {
		root.Close()
		return nil, errReftable
	}
	if cpBeforeRefWrite != nil {
		cpBeforeRefWrite()
	}
	return root, nil
}

// writable is a look at the folder pinned in the middle of a snapshot, before any work is written: a swap still there
// is said now, and a reftable repo is refused before the work.
func (r *cpRepo) writable() error {
	root, err := r.pinned()
	if err == nil {
		defer root.Close()
		if _, there := root.Lstat("reftable"); there == nil {
			err = errReftable
		}
	}
	return err
}

// createRef puts a new ref of the desk's, prefix<n> → oid, in the folder pinned, at one past the highest number there
// (a ref, loose or packed, or a lock file a git left at a number, which is skipped). Never over a ref that is there. A
// .git that changed by the time the ref is in takes it out again: nothing is kept, and it is said (errDotGitMoved).
func (r *cpRepo) createRef(prefix, oid string) (int, error) {
	root, err := r.refRoot()
	if err != nil {
		return 0, err
	}
	defer root.Close()
	folder := filepath.FromSlash(strings.TrimSuffix(prefix, "/"))
	if err := cpMakeFolders(root, folder, r.shared); err != nil {
		return 0, err
	}
	packed, err := cpPacked(root)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, line := range packed {
		if num, ok := strings.CutPrefix(line[1], prefix); ok {
			n = max(n, cpRefNumber(num))
		}
	}
	dir, err := root.Open(folder)
	if err != nil {
		return 0, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		n = max(n, cpRefNumber(strings.TrimSuffix(entry.Name(), ".lock")))
	}
	n++
	ref := prefix + strconv.Itoa(n)
	if err := cpPlaceRef(root, ref, oid, r.shared); err != nil {
		return 0, err
	}
	if r.moved() {
		_ = cpDropRef(root, ref, oid, r.shared)
		r.sayMoved()
		return 0, errDotGitMoved
	}
	return n, nil
}

func cpRefNumber(name string) int {
	n, err := strconv.Atoi(name)
	if err != nil || n < 1 || strconv.Itoa(n) != name {
		return 0
	}
	return n
}

// deleteRef drops a ref of the desk's from the folder pinned, only while it still holds oid (loose, packed, or both).
func (r *cpRepo) deleteRef(ref, oid string) error {
	root, err := r.refRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	return cpDropRef(root, ref, oid, r.shared)
}

// cpSharedPerm reads core.sharedRepository as git does (git_config_perm), for the values the store follows exactly:
// 0 when the umask decides the modes (unset, umask, false, 0), 0660 for a repo shared with its group (group, true, 1),
// 0664 for one shared with everybody (all, world, everybody, 2), and -mode for a mode every file is set to (an octal
// one such as 0640; git refuses one that leaves the owner without read and write). ok is false for any other value
// (git reads some of them, "1k" or "08" as true, say): the repo is refused rather than given modes that might not be git's.
func cpSharedPerm(value string, set bool) (perm int, ok bool) {
	if !set {
		return 0, true
	}
	switch value {
	case "umask", "": // git reads "" as the octal 0
		return 0, true
	case "group":
		return 0o660, true
	case "all", "world", "everybody":
		return 0o664, true
	}
	if cpSharedOctal.MatchString(value) {
		mode, _ := strconv.ParseInt(value, 8, 32)
		switch {
		case mode == 0:
			return 0, true
		case mode == 1:
			return 0o660, true
		case mode == 2:
			return 0o664, true
		case mode&0o600 != 0o600:
			return 0, false
		}
		return -int(mode & 0o666), true
	}
	switch strings.ToLower(value) {
	case "true", "yes", "on":
		return 0o660, true
	case "false", "no", "off":
		return 0, true
	}
	return 0, false
}

var cpSharedOctal = regexp.MustCompile(`^[0-7]{1,8}$`)

// cpModeBits are the bits of a mode that chmod sets.
const cpModeBits = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// cpSharedMode is the mode git gives a file or a folder it just made with mode, in a repo shared as shared says
// (cpSharedPerm, not 0): its calc_shared_perm, and for a folder adjust_shared_perm's extra (each read bit gives the
// matching search bit, and setgid when the group gets anything, so what is made in it stays the group's).
func cpSharedMode(shared int, mode fs.FileMode, folder bool) fs.FileMode {
	perm, tweak := int(mode.Perm()), max(shared, -shared)
	if perm&0o200 == 0 {
		tweak &^= 0o222
	}
	if perm&0o100 != 0 {
		tweak |= (tweak & 0o444) >> 2
	}
	if shared < 0 {
		perm = tweak
	} else {
		perm |= tweak
	}
	if folder {
		perm |= (perm & 0o444) >> 2
	}
	result := fs.FileMode(perm) | mode&(cpModeBits&^fs.ModePerm)
	if folder && perm&0o060 != 0 {
		result |= fs.ModeSetgid
	}
	return result
}

// cpMakeFolders makes the folders of folder (below the git folder) that aren't there yet, as git makes a ref's: in a
// repo that isn't shared, with the mode of its refs folder (and the umask); in a shared one, 0777 and the umask, then
// set past the umask to the mode git gives it (cpSharedMode). One that is a link, or a file, is refused (errRefsNotFolder):
// even a link that stays in the git folder would put the ref somewhere else, a branch say.
func cpMakeFolders(root *os.Root, folder string, shared int) error {
	refs, err := root.Stat("refs")
	if err != nil {
		return err
	}
	mode := refs.Mode().Perm()
	if shared != 0 {
		mode = 0o777
	}
	path := ""
	for _, part := range strings.Split(filepath.ToSlash(folder), "/") {
		path = filepath.Join(path, part)
		err := root.Mkdir(path, mode)
		if errors.Is(err, fs.ErrExist) {
			if info, err := root.Lstat(path); err != nil {
				return err
			} else if !info.IsDir() {
				return errRefsNotFolder
			}
			continue
		}
		if err == nil && shared != 0 {
			var info os.FileInfo
			if info, err = root.Lstat(path); err == nil {
				if want := cpSharedMode(shared, info.Mode(), true); want != info.Mode()&cpModeBits {
					err = root.Chmod(path, want)
				}
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// cpLock takes a lock file of git's ("<name>.lock", made new): a git of the owner's that holds it is waited on by no
// one, and is said in words. In a shared repo it is set at once to the mode git gives a lock file (cpSharedMode), so
// the file it is renamed to has it.
func cpLock(root *os.Root, name string, shared int) (*os.File, error) {
	file, err := cpCreate(root, name+".lock", shared)
	if errors.Is(err, fs.ErrExist) {
		return nil, cpSkip(lockWords(filepath.ToSlash(name) + ".lock"))
	}
	return file, err
}

// cpCreate makes a new file of git's (a lock, packed-refs.new) as git's tempfiles are made: 0666 and the umask, and in
// a shared repo set at once to the mode git gives it (cpSharedMode). Never over a file that is there (fs.ErrExist).
func cpCreate(root *os.Root, name string, shared int) (*os.File, error) {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil || shared == 0 {
		return file, err
	}
	info, err := file.Stat()
	if err == nil {
		if want := cpSharedMode(shared, info.Mode(), false); want != info.Mode()&cpModeBits {
			err = file.Chmod(want)
		}
	}
	if err != nil {
		file.Close()
		_ = root.Remove(name)
		return nil, err
	}
	return file, nil
}

// cpPlaceRef writes a new loose ref as git does: its lock made new, the id written in it, renamed to the ref. A ref of
// that name already there, loose or packed, is never overwritten.
func cpPlaceRef(root *os.Root, ref, oid string, shared int) error {
	name := filepath.FromSlash(ref)
	file, err := cpLock(root, name, shared)
	if err != nil {
		return err
	}
	placed := false
	defer func() {
		if !placed {
			_ = root.Remove(name + ".lock")
		}
	}()
	_, err = file.WriteString(oid + "\n")
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	packed, err := cpPacked(root)
	if err != nil {
		return err
	}
	if _, err := root.Lstat(name); !errors.Is(err, fs.ErrNotExist) || slices.ContainsFunc(packed, func(line [2]string) bool { return line[1] == ref }) {
		return errors.New("the ref is there already")
	}
	if err := root.Rename(name+".lock", name); err != nil {
		return err
	}
	placed = true
	return nil
}

// cpDropRef deletes a ref as git does: its own lock taken, then packed-refs' lock, then its reflog, packed-refs without
// its line and its loose file go, and its folder if left empty. Only while it holds oid; one already gone is fine.
func cpDropRef(root *os.Root, ref, oid string, shared int) error {
	name := filepath.FromSlash(ref)
	if err := cpMakeFolders(root, filepath.Dir(name), shared); err != nil { // a packed ref's folder may be gone
		return err
	}
	lock, err := cpLock(root, name, shared)
	if err != nil {
		return err
	}
	lock.Close()
	err = cpDropLocked(root, ref, oid, shared)
	_ = root.Remove(name + ".lock")
	if err == nil {
		_ = root.Remove(filepath.Dir(name)) // the desk's own folder of refs, once empty (no other desk's)
	}
	return err
}

func cpDropLocked(root *os.Root, ref, oid string, shared int) error {
	name := filepath.FromSlash(ref)
	loose, err := cpRootData(root, name, 1<<10)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case strings.TrimSpace(string(loose)) != oid:
		return errAnotherCommit
	}
	// packed-refs under its lock, always, as git's own delete does, and held until the loose file is gone: a pack-refs
	// of the owner's running meanwhile would otherwise pack the ref again as its loose file goes.
	lock, err := cpLock(root, "packed-refs", shared)
	if err != nil {
		return err
	}
	lock.Close()
	defer root.Remove("packed-refs.lock")
	data, err := cpRootData(root, "packed-refs", cpPackedMax)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = nil, nil
	}
	var kept []string
	packed, dropping := "", false
	for _, line := range strings.SplitAfter(string(data), "\n") {
		fields := strings.Fields(line)
		peeled := strings.HasPrefix(line, "^")
		if dropping && peeled {
			continue
		}
		if dropping = !peeled && len(fields) == 2 && fields[1] == ref; dropping {
			packed = fields[0]
		} else {
			kept = append(kept, line)
		}
	}
	switch {
	case err != nil:
		return err
	case loose == nil && packed != oid && packed != "":
		return errAnotherCommit
	}
	// As git's own delete: the reflog first (a ref left with no reflog is better than the other way round), then
	// packed-refs without the ref, then the loose file; the lock goes last. Gone already is fine.
	_ = root.Remove(filepath.Join("logs", name))
	if packed != "" {
		if err := cpWritePacked(root, strings.Join(kept, ""), shared); err != nil {
			return err
		}
	}
	if loose != nil {
		return root.Remove(name)
	}
	return nil
}

// cpWritePacked puts text in packed-refs as git does, while its lock is held: written to packed-refs.new (made new, at
// git's mode for it in a shared repo, else at packed-refs' own mode), then renamed in.
func cpWritePacked(root *os.Root, text string, shared int) error {
	info, err := root.Stat("packed-refs")
	if err != nil {
		return err
	}
	file, err := cpCreate(root, "packed-refs.new", shared)
	if err != nil {
		return err
	}
	if shared == 0 {
		err = file.Chmod(info.Mode().Perm())
	}
	if err == nil {
		_, err = file.WriteString(text)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = root.Rename("packed-refs.new", "packed-refs")
	}
	if err != nil {
		_ = root.Remove("packed-refs.new")
	}
	return err
}

var errAnotherCommit = errors.New("the ref holds another commit now")

const cpPackedMax = 64 << 20 // bytes of packed-refs read; a bigger one fails the write in words

// cpPacked is the refs of packed-refs, [id, name] each (none when there is no such file).
func cpPacked(root *os.Root) ([][2]string, error) {
	data, err := cpRootData(root, "packed-refs", cpPackedMax)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var refs [][2]string
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "^") {
			refs = append(refs, [2]string{fields[0], fields[1]})
		}
	}
	return refs, nil
}

// lockWords says a lock file is in the way, for the owner: it fails the snapshot or the drop with a word, and is not
// taken for the repo being broken. (Git takes it again, so it can only be deleted by hand, with no git running.)
func lockWords(lock string) string {
	return "A lock file (" + filepath.ToSlash(lock) + ") is in the way in this repo's .git folder, so the checkpoints can't be kept up to date. " +
		"If no git command is running, delete that file."
}

// open finds the repo of desk's folder and reads its settings, by the rules at the top of the file. With wantTree
// false and the folder gone, the repo the session was last seen in is used by its git folder (a fired hire's worktree
// is deleted, its refs are not).
func (s *checkpointStore) open(ctx context.Context, desk checkpointDesk, wantTree bool) (*cpRepo, error) {
	here := gitHere()
	switch {
	case here.version == "":
		return nil, cpSkip("Git isn't installed on this computer, so no checkpoint was taken.")
	case !gitVersionAtLeast(here.version, 2, 32):
		return nil, cpSkip("The git on this computer is older than 2.32, so no checkpoint was taken: update git to get them.")
	case desk.Folder == "":
		return nil, cpSkip("This desk has no folder, so no checkpoint was taken.")
	}
	attrSource := gitVersionAtLeast(here.version, 2, 40)
	repo := &cpRepo{dir: desk.Folder, call: gitCall{global: slices.Clip(gitReadArgs), config: slices.Clip(here.owner)}}
	query := []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir", "--show-object-format"}
	byGitDir := false
	if _, err := os.Stat(desk.Folder); err != nil {
		s.mu.Lock()
		known := s.where[desk.Session]
		s.mu.Unlock()
		if wantTree || known == "" {
			return nil, cpSkip("This desk's folder is gone, so no checkpoint was taken.")
		}
		byGitDir = true
		repo.dir, repo.bind = os.TempDir(), []string{"--git-dir=" + known}
		repo.refs = repo.bind
		query = []string{"rev-parse", "--path-format=absolute", "--git-common-dir", "--show-object-format"}
	}
	out, status, err := repo.git(ctx, nil, query...)
	if err != nil {
		return nil, cpWhy(err, "Git couldn't be read here, so no checkpoint was taken.")
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if status != 0 || len(lines) != len(query)-2 {
		return nil, cpSkip("This folder isn't in a git repo that can be read here, so there is no checkpoint to take.")
	}
	repo.format = lines[len(lines)-1]
	if byGitDir {
		// The folder is gone, so no .git can be swapped in it: the common git folder is pinned as it is now.
		common, err := filepath.EvalSymlinks(filepath.FromSlash(lines[0]))
		if err != nil {
			return nil, cpSkip("This desk's folder is gone, so no checkpoint was taken.")
		}
		repo.common = common
		if repo.commonWas, repo.objectsWas, err = pinFolder(common); err != nil {
			return nil, cpSkip("This desk's folder is gone, so no checkpoint was taken.")
		}
	} else if err := repo.settle(ctx, desk.Folder, lines[0], lines[1], lines[2]); err != nil {
		return nil, err
	}
	// The config first: a key the store doesn't know, and the repo isn't used; every program it names is blanked.
	list, status, err := repo.git(ctx, nil, "config", "--list", "-z")
	if err != nil || status != 0 {
		return nil, cpWhy(err, "Git couldn't be read here, so no checkpoint was taken.")
	}
	blanks, refused := gitRepoConfig(list)
	if refused != "" {
		return nil, cpSkip("No checkpoint was taken: this repo's git config has a setting the store doesn't know (" + refused + ").")
	}
	repo.call.config = append(repo.call.config, blanks...)
	repo.keep, repo.compat = cpKept(list)
	shared, set := "", false
	for _, kept := range repo.keep {
		if kept[0] == "core.sharedrepository" {
			shared, set = kept[1], true
		}
	}
	if repo.shared, set = cpSharedPerm(shared, set); !set {
		return nil, cpSkip("No checkpoint was taken: this repo's git config sets core.sharedRepository to a value the store can't follow as git does " +
			"(it follows umask, group, all, world, everybody, true, false, 0, 1, 2 and a mode such as 0640).")
	}
	_, knownFormat := gitEmptyTrees[repo.format]
	if _, knownCompat := gitEmptyTrees[repo.compat]; !knownFormat || (repo.compat != "" && !knownCompat) {
		return nil, cpSkip("This repo's kind of hash isn't one the store knows, so no checkpoint was taken.")
	}
	if attrSource {
		repo.call.global = append(repo.call.global, "--attr-source="+gitEmptyTrees[repo.format])
	}
	// What a write needs on top, handed last so it wins over what the repo says.
	repo.call.config = append(repo.call.config, cpWriteSettings...)
	return repo, nil
}

// cpWriteSettings go last in every command of a snapshot, in the repo and in its private git folder.
var cpWriteSettings = [][2]string{{"commit.gpgsign", "false"}, {"core.logAllRefUpdates", "false"}, {"gc.auto", "0"},
	{"core.autocrlf", "false"}, {"core.safecrlf", "false"}, {"core.splitIndex", "false"}}

// settle takes the repo that git found from folder (its top, git folder and common folder, as rev-parse said them) as
// the desk's for good, or refuses it in words. Every path is resolved once, here, and named to every later command
// (B6): nothing found again from a folder decides where the store works. Refused:
//   - a top that is a folder of another repo (a core.worktree that points there);
//   - a desk's folder that isn't the top or inside it (B4): a .git file, or a core.worktree, that sends git to a working
//     folder elsewhere (a submodule's git folder names its own, so a one-line gitfile would do it);
//   - a .git that names another folder's own git folder ("<other>/.git"), or a linked worktree's git folder that
//     doesn't point back at this .git: the git folder has to be this folder's own.
func (r *cpRepo) settle(ctx context.Context, folder, top, gitDir, common string) error {
	unreadable := cpSkip("This folder isn't in a git repo that can be read here, so there is no checkpoint to take.")
	resolved := map[string]string{}
	for _, path := range []string{folder, top, gitDir, common} {
		real, err := filepath.EvalSymlinks(filepath.FromSlash(path))
		if err != nil {
			return unreadable
		}
		resolved[path] = real
	}
	r.top, r.gitDir, r.common = resolved[top], resolved[gitDir], resolved[common]
	for _, dir := range []string{r.gitDir, r.common} {
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
			return unreadable
		}
	}
	// The common git folder, pinned: what an open folder of it (and of its objects folder) says it is. Every write only
	// ever goes into that folder (pinned).
	was, objects, err := pinFolder(r.common)
	if err != nil {
		return unreadable
	}
	r.commonWas, r.objectsWas = was, objects
	// A core.worktree that points at another repo: the top it names has to be a folder of this same repo, or the store
	// would snapshot the other one.
	out, status, err := r.gitIn(ctx, r.top, nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return cpWhy(err, "Git couldn't be read here, so no checkpoint was taken.")
	}
	if status != 0 || !sameCheckpointFolder(strings.TrimSpace(string(out)), r.common) {
		return cpSkip("This repo's config (core.worktree) puts its working folder somewhere else, in another repo, so no checkpoint was taken.")
	}
	elsewhere := cpSkip("This folder's .git sends git to a working folder somewhere else, so no checkpoint was taken.")
	topInfo, err := os.Stat(r.top)
	if err != nil {
		return unreadable
	}
	// The desk's folder is the top or inside it; on the way up, the first .git is the one git found (its own search).
	inside := false
	for dir := resolved[folder]; ; dir = filepath.Dir(dir) {
		if r.dotGitWas == nil {
			if info, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
				r.dotGit, r.dotGitWas = filepath.Join(dir, ".git"), info
			}
		}
		if info, err := os.Stat(dir); err == nil && os.SameFile(info, topInfo) {
			inside = true
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if !inside {
		return elsewhere
	}
	if r.dotGitWas == nil {
		return unreadable
	}
	notOwn := cpSkip("This folder's .git points at a git folder that isn't this folder's own, so no checkpoint was taken.")
	switch mode := r.dotGitWas.Mode(); {
	case mode.IsDir():
		// The repo's own git folder, right here: the folder pinned has to be this very one (so not one that was swapped
		// in while git or the store looked), and the repo's common one (not a linked worktree's folder moved here).
		if r.gitDir != r.common || !os.SameFile(r.dotGitWas, r.commonWas) {
			return notOwn
		}
	case !mode.IsRegular():
		// A link (or a junction, or anything else that isn't a file): the git folder would be wherever it points at
		// each moment, which no look can pin.
		return notOwn
	case r.gitDir != r.common:
		// A linked worktree: its git folder's gitdir file names the .git of the worktree it is for.
		back, err := cpReadData(filepath.Join(r.gitDir, "gitdir"), 1<<12)
		if err != nil {
			return notOwn
		}
		path := strings.TrimSpace(string(back))
		if !filepath.IsAbs(path) {
			path = filepath.Join(r.gitDir, path)
		}
		if !sameCheckpointFolder(path, r.dotGit) {
			return notOwn
		}
	case filepath.Base(r.gitDir) == ".git" && !sameCheckpointFolder(filepath.Dir(r.gitDir), filepath.Dir(r.dotGit)):
		return notOwn // "<other>/.git", the git folder of another folder's own repo
	}
	r.dir = r.top
	r.bind = []string{"--git-dir=" + r.gitDir, "--work-tree=" + r.top}
	r.refs = []string{"--git-dir=" + r.common}
	return nil
}

// cpKeptKeys are the repo's settings that shape what add, write-tree and commit-tree make of the files (the ignore
// file, the letter case and the Unicode form of names, links and the exec bit, a sparse checkout) or how they write
// objects (its sharing mode, its fsync; not core.bigFileThreshold, so a file is never written in a pack, which
// importObjects doesn't carry: git's own threshold is above checkpointMaxBytes). All data: none names a program. They
// are the only ones of the repo's carried into a snapshot's private git folder, at the value the repo has (the last in
// config --list).
var cpKeptKeys = gitKeySet(`core.excludesfile core.ignorecase core.precomposeunicode core.symlinks core.filemode
	core.sparsecheckout core.sparsecheckoutcone core.protecthfs core.protectntfs core.longpaths core.sharedrepository
	core.fsync core.fsyncmethod core.fsyncobjectfiles core.createobject core.compression core.loosecompression`)

// cpKept reads config --list -z for the settings above, and the repo's second hash, if any.
func cpKept(list []byte) (keep [][2]string, compat string) {
	last := map[string]string{}
	var order []string
	for _, record := range strings.Split(string(list), "\x00") {
		key, value, found := strings.Cut(record, "\n")
		if !found {
			value = "true" // a key with no "=" is a boolean that is on
		}
		switch {
		case key == "extensions.compatobjectformat":
			compat = value
		case cpKeptKeys[key]:
			if _, seen := last[key]; !seen {
				order = append(order, key)
			}
			last[key] = value
		}
	}
	for _, key := range order {
		keep = append(keep, [2]string{key, last[key]})
	}
	return keep, compat
}

// cpReadData reads a small file of a git folder that is data (an ignore list, a path): a regular file of at most max
// bytes, links followed as git follows them. Nothing else is read: a fifo or a device would never end (B9). What is
// judged is the file that was opened, never the path looked at a second time: it is opened first, in a way that never
// waits (cpOpenData: a fifo with no writer opens at once), then asked what it is, and read only if it is a regular
// file, from that same open file. A fifo swapped in at any moment is refused, and never blocks the desk.
func cpReadData(path string, max int64) ([]byte, error) {
	file, err := cpOpenData(path)
	if err != nil {
		return nil, err
	}
	return cpReadOpen(file, max)
}

// cpRootData is cpReadData through an open git folder: name is looked up in it, and never leads out of it.
func cpRootData(root *os.Root, name string, max int64) ([]byte, error) {
	file, err := root.OpenFile(name, cpDataFlags, 0)
	if err != nil {
		return nil, err
	}
	return cpReadOpen(file, max)
}

func cpReadOpen(file *os.File, max int64) ([]byte, error) {
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > max {
		return nil, errors.New("not a small file")
	}
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err == nil && int64(len(data)) > max {
		err = errors.New("not a small file")
	}
	return data, err
}

// cpPrivate is the git folder a snapshot is made in (B5, B8): read-tree, ls-files (for the caps), add, write-tree and
// commit-tree run there, never in the repo's own git folder, so what git reads while it adds the files is what the
// store wrote, and nothing of the repo's: a config that holds the repo's hash and the cpKeptKeys (handed in the
// environment) and no other setting, so no filter, fsmonitor, hook or include of the repo's; an empty
// info/attributes; and the repo's info/exclude and info/sparse-checkout, copied (ignore and sparse rules are data).
// The repo's own config and attributes can change at any moment: add never opens them. Objects are written in the
// folder's own store; the repo's is only read, as an alternate (an object is named by its hash, so whichever store
// answers, it is the same object). The files are the desk's top (--work-tree); the index is the folder's own.
//
// "What git reads is what the store wrote" only holds if nobody else can write the folder, so it is made in the app's
// support folder (cpFolder), which a hire can't write (the sandbox's design denies it the whole support folder), and
// not in $TMPDIR, which every program of the owner's account can list and write, hires included. It is made new for
// each snapshot, 0700, under a name never used before, and refused if that name is already there (cpMakeFolder).
type cpPrivate struct {
	repo    *cpRepo
	dir     string // the private git folder
	objects string // its own object store, where every object a snapshot makes is written
	call    gitCall
}

// git runs a command in the private git folder, on the desk's files; extra is environment for that one command.
func (p *cpPrivate) git(ctx context.Context, extra []string, args ...string) ([]byte, int, error) {
	env := gitReadEnv(os.Environ(), p.call.config)
	argv := append([]string{"-C", p.repo.top, "--git-dir=" + p.dir, "--work-tree=" + p.repo.top}, p.call.global...)
	return execGit(ctx, append(env, extra...), append(argv, args...)...)
}

func (r *cpRepo) private(scratch string) (*cpPrivate, error) {
	dir := filepath.Join(scratch, "git")
	config := "[core]\n\trepositoryformatversion = 1\n\tbare = false\n[extensions]\n\tobjectformat = " + r.format + "\n"
	if r.compat != "" {
		config += "\tcompatobjectformat = " + r.compat + "\n"
	}
	files := map[string]string{"HEAD": "ref: refs/heads/checkpoint\n", "config": config, "info/attributes": ""}
	// A store that is a link (moved elsewhere and linked back) is one the import below can't follow out of the git
	// folder (os.Root): said in words before any work, though plain git is fine with it.
	if info, err := os.Lstat(filepath.Join(r.common, "objects")); err == nil && !info.IsDir() {
		return nil, cpSkip("This repo's .git/objects folder is a link to another place, which the checkpoints don't follow, so none was taken.")
	}
	for name, from := range map[string]string{"info/exclude": r.common, "info/sparse-checkout": r.gitDir} {
		data, err := cpReadData(filepath.Join(from, filepath.FromSlash(name)), 8<<20)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, cpSkip("This repo's " + name + " isn't a file that can be read, so no checkpoint was taken.")
		default:
			files[name] = string(data)
		}
	}
	// The repo's objects, read as an alternate: by their resolved path, which only ever serves reads.
	store, err := filepath.EvalSymlinks(filepath.Join(r.common, "objects"))
	if err != nil {
		return nil, err
	}
	files["objects/info/alternates"] = store + "\n"
	for _, sub := range []string{"refs", "info", "objects/info"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(sub)), 0o700); err != nil {
			return nil, err
		}
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(text), 0o600); err != nil {
			return nil, err
		}
	}
	call := gitCall{global: r.call.global, config: slices.Concat(slices.Clip(gitHere().owner), r.keep, cpWriteSettings)}
	return &cpPrivate{repo: r, dir: dir, call: call, objects: filepath.Join(dir, "objects")}, nil
}

// cpFolder is where the snapshots' private git folders are made: in the app's support folder (a test's own in tests).
func cpFolder() string { return filepath.Join(supportDir, "checkpoints") }

// cpFolderName names a snapshot's private folder: random, so never one used before; a variable so a test can name it.
var cpFolderName = rand.Text

// cpMakeFolder makes a snapshot's private folder: the checkpoints folder (made if need be, 0700, and refused unless it
// is a real folder, not a link), then a folder in it under a new name, 0700, made by this call or refused. A folder of
// that name already there is never used, and never removed: the caller only removes what it made.
func cpMakeFolder() (string, error) {
	parent := cpFolder()
	if err := os.Mkdir(parent, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() {
		return "", errors.New("the checkpoints folder isn't a folder")
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return "", err
	}
	dir := filepath.Join(parent, cpFolderName())
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// cpSweep removes what is in the checkpoints folder: private folders a crash left behind. Once per store, before its
// first snapshot, when none of its own can be running.
func cpSweep() {
	entries, _ := os.ReadDir(cpFolder())
	for _, entry := range entries {
		_ = os.RemoveAll(filepath.Join(cpFolder(), entry.Name()))
	}
}

var hexName = regexp.MustCompile(`^[0-9a-f]+$`)

// importObjects puts the objects a snapshot wrote in its private store (from) into the repo's own (B6). The repo's
// common git folder is opened once, and refused unless it is the very folder settle pinned (errDotGitMoved): from then
// on every step goes through that open folder (os.Root), never through a path, so a .git swapped for another, even
// one put back right after, can't send an object anywhere else, and no link in the store leads out of it. An object
// already there is left as it is; a new one is written beside it and renamed in, as git does, readable by whoever can
// read the store's folder (a repo shared with its group stays so), and a new fan-out folder takes the store's mode.
// Only loose objects are carried: a pack, or anything else in the private store, fails the snapshot.
func (r *cpRepo) importObjects(from string) error {
	common, err := r.pinned()
	if err != nil {
		return err
	}
	defer common.Close()
	store, err := common.OpenRoot("objects")
	if err != nil {
		return err
	}
	defer store.Close()
	info, err := store.Stat(".")
	if err != nil {
		return err
	}
	folderMode := info.Mode() & (fs.ModePerm | fs.ModeSetgid)
	fileMode := 0o444 & info.Mode().Perm()
	fans, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, fan := range fans {
		name := fan.Name()
		if name == "info" || name == "pack" {
			if inside, err := os.ReadDir(filepath.Join(from, name)); err != nil || (name == "pack" && len(inside) > 0) {
				return errors.New("a pack in the private store")
			}
			continue
		}
		if !fan.IsDir() || len(name) != 2 || !hexName.MatchString(name) {
			return errors.New("not an object folder: " + name)
		}
		objects, err := os.ReadDir(filepath.Join(from, name))
		if err != nil {
			return err
		}
		for _, object := range objects {
			if !object.Type().IsRegular() || !hexName.MatchString(object.Name()) {
				return errors.New("not an object: " + object.Name())
			}
			if err := importObject(store, filepath.Join(from, name, object.Name()), name, object.Name(), folderMode, fileMode); err != nil {
				return err
			}
		}
	}
	return nil
}

// importObject writes one loose object into store (an open object folder), at fan/name, unless it is there already.
func importObject(store *os.Root, from, fan, name string, folderMode, fileMode fs.FileMode) error {
	target := filepath.Join(fan, name)
	if _, err := store.Lstat(target); err == nil {
		return nil
	}
	if err := store.Mkdir(fan, folderMode.Perm()); err == nil {
		_ = store.Chmod(fan, folderMode) // past the umask, as git's own fan-out folders are
	} else if !errors.Is(err, fs.ErrExist) {
		return err
	}
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	temp := filepath.Join(fan, "tmp_obj_smolcorp_"+rand.Text()) // git's prune sweeps a tmp_obj_ left behind
	file, err := store.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, source)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = store.Chmod(temp, fileMode)
	}
	if err == nil {
		err = store.Rename(temp, target)
	}
	if err != nil {
		_ = store.Remove(temp)
		if _, there := store.Lstat(target); there == nil {
			return nil // a git of the owner's wrote the same object meanwhile
		}
	}
	return err
}

// sameCheckpointFolder says two paths are the one folder (links and the disk's letter case taken into account).
func sameCheckpointFolder(a, b string) bool {
	x, errA := os.Stat(filepath.FromSlash(a))
	y, errB := os.Stat(filepath.FromSlash(b))
	return errA == nil && errB == nil && os.SameFile(x, y)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A label is one line: control characters and line breaks become spaces, runs of spaces one, and it is cut to
// checkpointLabelMax bytes.
func cleanLabel(label string) string {
	label = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, label)), " ")
	if len(label) > checkpointLabelMax {
		label = strings.ToValidUTF8(label[:checkpointLabelMax], "")
	}
	if label == "" {
		return "checkpoint"
	}
	return label
}

// snapshot takes a checkpoint of desk's working tree. A zero Checkpoint and a note say it was skipped, and why.
func (s *checkpointStore) snapshot(desk checkpointDesk, label string) (Checkpoint, string) {
	ctx, cancel := context.WithTimeout(context.Background(), checkpointTimeout)
	defer cancel()
	leave, ok := s.enter(ctx, desk.Session)
	if !ok {
		return Checkpoint{}, "Taking a checkpoint took too long here, so it was skipped."
	}
	defer leave()
	select {
	case s.jobs <- struct{}{}:
		defer func() { <-s.jobs }()
	case <-ctx.Done():
		return Checkpoint{}, "Taking a checkpoint took too long here, so it was skipped."
	}
	if ctx.Err() != nil {
		return Checkpoint{}, "Taking a checkpoint took too long here, so it was skipped."
	}
	cp, repo, err := s.take(ctx, desk, cleanLabel(label))
	if err != nil {
		return Checkpoint{}, err.Error()
	}
	// A failure to drop an old one is no failure of this one. It reads on a deadline of its own: the snapshot's may be spent.
	pruneCtx, pruneCancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer pruneCancel()
	s.pruneIn(pruneCtx, repo, desk, checkpointsKept)
	return cp, ""
}

// cpGitErr is err, or a plain error for a git that ran and said no (cpWhy then gives the generic words).
func cpGitErr(err error) error {
	if err == nil {
		return errors.New("git said no")
	}
	return err
}

// A gitlink of the private index: a submodule's folder and the commit it is at.
type cpGitlink struct{ path, oid string }

// Where a gitlink's folder stands on disk, as the store sees it: no link of any kind is followed.
const (
	linkGone   = iota // nothing is there (or a file where a parent folder should be)
	linkFound         // a real folder that holds a .git: a checked-out submodule
	linkOther         // anything else: a link, a file, a folder with no .git
	linkBehind        // a link stands in the way, on a folder above it: add refuses a pathspec beyond a link
)

// gitlinkState looks at where path (a gitlink's, slash-separated) is below top, one step at a time with Lstat, so no
// symlink on the way is followed. For linkFound it also gives the .git to read the submodule's HEAD from.
func gitlinkState(top, path string) (state int, dotGit string) {
	parts := strings.Split(path, "/")
	here := top
	for i, part := range parts {
		here = filepath.Join(here, part)
		info, err := os.Lstat(here)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return linkGone, ""
		case err != nil:
			return linkOther, ""
		case info.Mode()&os.ModeSymlink != 0:
			if i < len(parts)-1 {
				return linkBehind, ""
			}
			return linkOther, ""
		case !info.IsDir():
			if i < len(parts)-1 {
				return linkGone, ""
			}
			return linkOther, ""
		}
	}
	dotGit = filepath.Join(here, ".git")
	if _, err := os.Lstat(dotGit); err != nil {
		return linkOther, ""
	}
	return linkFound, dotGit
}

// stage is add -A into the private index, with every submodule left out of what add sees. add asks every gitlink of the
// index whether its folder is dirty, by running git in the submodule, with the submodule's own config and attributes
// (a clean filter of its own, say), which nothing here has judged; and it decides that by what is on disk at that
// moment, which a hire can change between the store's look and add's. So no gitlink is ever shown to add, whatever is on
// disk: each is excluded by a pathspec (one behind a symlink, which add won't take as a pathspec, is taken out of the
// index), and then settled by hand, in a way that runs nothing:
//   - its folder is gone: the link is removed from the index (plain git's add -A does the same);
//   - its folder is a real one with a .git: the link is set to the commit that submodule's HEAD is at, read with
//     --git-dir (a rev-parse, which starts nothing; it never finds a repo above the submodule if the .git has gone);
//   - anything else (a link or a file in its place, a folder with no .git, a HEAD that can't be read, as before the
//     first commit): the commit HEAD of the repo had for it is kept. Plain git would record the file or the files, and
//     the store doesn't: the difference is rare, and the checkpoints don't restore a submodule.
func (s *checkpointStore) stage(ctx context.Context, repo *cpRepo, work *cpPrivate, scratch string) error {
	out, status, err := work.git(ctx, nil, "ls-files", "--stage", "-z")
	if err != nil || status != 0 {
		return cpGitErr(err)
	}
	var links, behind []cpGitlink
	for _, record := range strings.Split(string(out), "\x00") {
		meta, path, found := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 3 || fields[0] != "160000" || !checkpointID.MatchString(fields[1]) {
			continue
		}
		link := cpGitlink{path: path, oid: fields[1]}
		if state, _ := gitlinkState(repo.top, path); state == linkBehind {
			behind = append(behind, link)
			continue
		}
		links = append(links, link)
	}
	// Behind a link: the folder a submodule was in is a link now, so the submodule is gone (plain git's add -A takes it
	// for deleted too, and the link itself, a file of its own, couldn't sit beside it in the tree). It is taken out
	// before add, which has no pathspec for what is beyond a link.
	for _, link := range behind {
		if _, status, err := work.git(ctx, nil, "update-index", "--force-remove", "--", link.path); err != nil || status != 0 {
			return cpGitErr(err)
		}
	}
	args := []string{"add", "-A"}
	if len(links) > 0 {
		// The pathspecs go in a file (NUL between them, so no name is quoted), and not on the command line, which a repo
		// with many submodules would overrun. "." is everything, the excludes take the submodules out of it.
		spec := "."
		for _, link := range links {
			spec += "\x00:(exclude,literal)" + link.path
		}
		file := filepath.Join(scratch, "pathspec")
		if err := os.WriteFile(file, []byte(spec), 0o600); err != nil {
			return err
		}
		args = append(args, "--pathspec-from-file="+file, "--pathspec-file-nul")
	}
	if _, status, err := work.git(ctx, nil, args...); err != nil || status != 0 {
		return cpGitErr(err)
	}
	for _, link := range links {
		state, dotGit := gitlinkState(repo.top, link.path)
		var update []string
		switch state {
		case linkGone:
			update = []string{"update-index", "--force-remove", "--", link.path}
		case linkFound:
			// The submodule is asked as a repo by itself (--git-dir after the repo's own, no work tree), never by
			// its folder: a folder whose .git went would be found in the repo above it.
			head, status, err := repo.gitIn(ctx, repo.dir, []string{"--git-dir=" + dotGit}, nil, "rev-parse", "--verify", "-q", "HEAD")
			if err != nil {
				return err
			}
			oid := strings.TrimSpace(string(head))
			if status != 0 || len(oid) != len(link.oid) || !checkpointID.MatchString(oid) || oid == link.oid {
				continue
			}
			update = []string{"update-index", "--add", "--cacheinfo", "160000," + oid + "," + link.path}
		default: // a link or a file or a folder with no .git, or a link above it that came after the look: as it was
			continue
		}
		if _, status, err := work.git(ctx, nil, update...); err != nil || status != 0 {
			return cpGitErr(err)
		}
	}
	return nil
}

func (s *checkpointStore) take(ctx context.Context, desk checkpointDesk, label string) (Checkpoint, *cpRepo, error) {
	repo, err := s.open(ctx, desk, true)
	if err != nil {
		return Checkpoint{}, nil, err
	}
	failed := func(err error) (Checkpoint, *cpRepo, error) {
		return Checkpoint{}, nil, cpWhy(err, "Git couldn't take the checkpoint, so none was kept.")
	}
	// HEAD now; none before the first commit.
	head := ""
	if out, status, err := repo.git(ctx, nil, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return failed(err)
	} else if status == 0 {
		head = strings.TrimSpace(string(out))
		if !checkpointID.MatchString(head) {
			return failed(nil)
		}
	}
	// The repo is still the one pinned, before any work is written (a swap still there is said now).
	if err := repo.writable(); err != nil {
		return failed(err)
	}
	// The private git folder, its index and its store: a new folder in the app's support folder, which only the app
	// writes (B8), gone when the snapshot is.
	s.swept.Do(cpSweep)
	scratch, err := cpMakeFolder()
	if err != nil {
		return Checkpoint{}, nil, cpSkip("The checkpoint's private folder couldn't be made in the app's own folder, so none was kept.")
	}
	defer os.RemoveAll(scratch)
	work, err := repo.private(scratch)
	if err != nil {
		return failed(err)
	}
	if head != "" {
		if _, status, err := work.git(ctx, nil, "read-tree", head); err != nil || status != 0 {
			return failed(err)
		}
	}
	// The caps, before a byte is hashed: the files add would read, counted and measured, as the private index (HEAD's
	// files) and the folder show them.
	out, status, err := work.git(ctx, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	if err != nil || status != 0 {
		return failed(err)
	}
	files, total := 0, int64(0)
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		if files++; files > checkpointMaxFiles {
			return Checkpoint{}, nil, cpSkip("This folder has more than " + strconv.Itoa(checkpointMaxFiles) + " files, so no checkpoint was taken.")
		}
		if info, err := os.Lstat(filepath.Join(repo.top, filepath.FromSlash(name))); err == nil && info.Mode().IsRegular() {
			if total += info.Size(); total > checkpointMaxBytes {
				return Checkpoint{}, nil, cpSkip("The files in this folder add up to more than " + strconv.FormatInt(checkpointMaxBytes>>20, 10) + " MB, so no checkpoint was taken.")
			}
		}
	}
	if err := s.stage(ctx, repo, work, scratch); err != nil {
		return failed(err)
	}
	out, status, err = work.git(ctx, nil, "write-tree")
	tree := strings.TrimSpace(string(out))
	if err != nil || status != 0 || !checkpointID.MatchString(tree) {
		return failed(err)
	}
	now := time.Now()
	subject := checkpointSubject + "head=" + cmpOr(head, "-") + " label=" + label
	identity := append([]string{}, "GIT_AUTHOR_NAME="+checkpointName, "GIT_AUTHOR_EMAIL="+checkpointEmail,
		"GIT_COMMITTER_NAME="+checkpointName, "GIT_COMMITTER_EMAIL="+checkpointEmail,
		"GIT_AUTHOR_DATE="+strconv.FormatInt(now.Unix(), 10)+" +0000", "GIT_COMMITTER_DATE="+strconv.FormatInt(now.Unix(), 10)+" +0000")
	out, status, err = work.git(ctx, identity, "commit-tree", tree, "-m", subject)
	commit := strings.TrimSpace(string(out))
	if err != nil || status != 0 || !checkpointID.MatchString(commit) {
		return failed(err)
	}
	// Into the repo's own store, through the folder pinned: from here the commit is in the desk's repo, and only there.
	if err := repo.importObjects(work.objects); err != nil {
		return failed(err)
	}
	// The ref, at the next number of the desk's, as the folder pinned holds them (not as a read by path said them: a
	// swap during it could have shown another repo's).
	n, err := repo.createRef(checkpointRefs+sessionRefName(desk.Session)+"/", commit)
	if err != nil { // no git ran: the store's own write failed (a link out of the git folder, say)
		return Checkpoint{}, nil, cpWhy(err, "The checkpoint couldn't be written in this repo's .git folder, so none was kept.")
	}
	s.mu.Lock()
	s.where[desk.Session] = repo.common
	s.mu.Unlock()
	return Checkpoint{ID: commit, N: n, Label: label, Head: head, At: now.Unix() * 1000}, repo, nil
}

// read lists a desk's checkpoints from the repo, oldest first.
func (s *checkpointStore) read(ctx context.Context, repo *cpRepo, desk checkpointDesk) ([]Checkpoint, string) {
	prefix := checkpointRefs + sessionRefName(desk.Session) + "/"
	out, status, err := repo.refGit(ctx, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(committerdate:unix)%00%(contents:subject)", prefix)
	if err != nil || status != 0 {
		return nil, cpWhy(err, "Git couldn't list the checkpoints here.").Error()
	}
	var list []Checkpoint
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) != 4 || !strings.HasPrefix(parts[0], prefix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(parts[0], prefix))
		if err != nil || n < 1 || !checkpointID.MatchString(parts[1]) {
			continue
		}
		cp := Checkpoint{ID: parts[1], N: n}
		cp.At, _ = strconv.ParseInt(parts[2], 10, 64)
		cp.At *= 1000
		if rest, ok := strings.CutPrefix(parts[3], checkpointSubject+"head="); ok {
			head, label, _ := strings.Cut(rest, " label=")
			if head != "-" && checkpointID.MatchString(head) {
				cp.Head = head
			}
			cp.Label = label
		}
		list = append(list, cp)
	}
	slices.SortFunc(list, func(a, b Checkpoint) int { return a.N - b.N })
	return list, ""
}

// list is desk's checkpoints, oldest first. A note says why none could be read.
func (s *checkpointStore) list(desk checkpointDesk) ([]Checkpoint, string) {
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	repo, err := s.open(ctx, desk, false)
	if err != nil {
		return nil, err.Error()
	}
	list, note := s.read(ctx, repo, desk)
	if list == nil {
		list = []Checkpoint{}
	}
	return list, note
}

// between is the paths that differ between two of desk's checkpoints (ids as list gave them), sorted as git does.
// A note says why it could not be answered, an id that isn't one of the desk's (or was pruned) included.
func (s *checkpointStore) between(desk checkpointDesk, a, b string) ([]CheckpointChange, string) {
	if !checkpointID.MatchString(a) || !checkpointID.MatchString(b) {
		return nil, "That is not a checkpoint of this desk."
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	repo, err := s.open(ctx, desk, false)
	if err != nil {
		return nil, err.Error()
	}
	list, note := s.read(ctx, repo, desk)
	if note != "" {
		return nil, note
	}
	has := func(id string) bool {
		return slices.ContainsFunc(list, func(cp Checkpoint) bool { return cp.ID == id })
	}
	if !has(a) || !has(b) {
		return nil, "That checkpoint is no longer kept."
	}
	out, status, err := repo.git(ctx, nil, "diff-tree", "-r", "-z", "--name-status", "--no-renames", "--no-ext-diff", "--no-textconv", a, b, "--")
	if err != nil || status != 0 {
		note := "The changed files couldn't be listed."
		if errors.Is(err, errGitTooBig) {
			note = "There are too many changed files to list."
		}
		return nil, note
	}
	changes := []CheckpointChange{}
	tokens := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(tokens); i += 2 {
		if tokens[i] == "" {
			break
		}
		changes = append(changes, CheckpointChange{Status: tokens[i][:1], Path: filepath.FromSlash(tokens[i+1])})
	}
	return changes, ""
}

// prune keeps desk's newest `keep` checkpoints and drops the others' refs (their objects go with git's own gc).
func (s *checkpointStore) prune(ctx context.Context, desk checkpointDesk, keep int) string {
	repo, err := s.open(ctx, desk, false)
	if err != nil {
		return err.Error()
	}
	return s.pruneIn(ctx, repo, desk, keep)
}

const dropTooLong = "Dropping old checkpoints took too long here."

func (s *checkpointStore) pruneIn(ctx context.Context, repo *cpRepo, desk checkpointDesk, keep int) string {
	list, note := s.read(ctx, repo, desk)
	if note != "" || len(list) <= keep {
		return note
	}
	prefix := checkpointRefs + sessionRefName(desk.Session) + "/"
	first := ""
	for _, cp := range list[:len(list)-max(keep, 0)] { // each ref only while it still holds the commit read
		if ctx.Err() != nil { // the drop as a whole is bounded, as each delete is on its own
			if first == "" {
				first = dropTooLong
			}
			break
		}
		err := repo.deleteRef(prefix+strconv.Itoa(cp.N), cp.ID)
		if err == nil {
			continue
		}
		if first == "" {
			first = cpWhy(err, "An old checkpoint couldn't be dropped from this repo's .git folder.").Error()
		}
		// One ref that can't go (a lock file of its own left on it) leaves the others to go, so a desk is never stuck
		// with more and more. A .git that isn't the one pinned (nothing more is written in a repo that may not be the
		// desk's now), a reftable, the lock of packed-refs, or a link where the desk's folder of refs goes stops the
		// whole drop: the rest would fail the same.
		if errors.Is(err, errDotGitMoved) || errors.Is(err, errReftable) || errors.Is(err, errPackedLocked) || errors.Is(err, errRefsNotFolder) {
			break
		}
	}
	return first
}

// keepOnly is prune with its place in the desk's line: what the app calls to cap a desk. It waits for a snapshot of
// the desk that is running (up to the snapshot's own cap), then reads on its own time.
func (s *checkpointStore) keepOnly(desk checkpointDesk, keep int) string {
	leave, ok := s.waitForDesk(desk)
	if !ok {
		return dropTooLong
	}
	defer leave()
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	return s.prune(ctx, desk, keep)
}

// forget drops every checkpoint of desk: the desk was fired. Call it while the folder is still there; if it is
// gone, the repo the desk last took a checkpoint in this run is used. A snapshot of the desk that is running is waited
// for, up to its cap (a fire usually comes right after a turn, when one is), so the ref it adds is not left behind.
func (s *checkpointStore) forget(desk checkpointDesk) string {
	leave, ok := s.waitForDesk(desk)
	if !ok {
		return "Dropping the checkpoints took too long here."
	}
	defer leave()
	ctx, cancel := context.WithTimeout(context.Background(), gitStateTimeout)
	defer cancel()
	note := s.prune(ctx, desk, 0)
	if note == "" {
		s.mu.Lock()
		delete(s.where, desk.Session)
		s.mu.Unlock()
	}
	return note
}

// waitForDesk takes the desk's turn, waiting as long as a snapshot of it can run.
func (s *checkpointStore) waitForDesk(desk checkpointDesk) (leave func(), ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), checkpointDeskWait)
	defer cancel()
	return s.enter(ctx, desk.Session)
}
