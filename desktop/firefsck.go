package desktop

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// The fire's look at the repo's stored objects (FXFIREFSCK, SIERRA's ruling #1c; in 1.26 the only guard against
// objects rewritten in place, FXOBJSTORE being 1.26.1's). Git trusts its object store: it never hashes an object again
// when it reads it. A desk that may write the shared objects/ can so give a name other bytes (a loose object's file
// rewritten, two offsets swapped in a pack's .idx, FXREPLACEREFS §5), and main then reads as the desk wants without
// moving: a file's bytes, or a commit whose parent is the desk's, which makes the desk's branch look merged.
// `git fsck --connectivity-only` hashes nothing, and `rev-list --verify-objects` says a mismatch but still exits 0
// (FXFIREFSCK's probe).
//
// So before the fire's removal of a hire's own worktree and branch (its merged look: the branch goes only when it's
// merged), the core hashes again every object the repo's HEAD, its base branches (storeBaseRefs) and the hire's branch
// reach, as git names them (`rev-list --objects`) and as git reads them (`cat-file --batch`), through gitReadEnv and
// gitReadArgs: no replace ref, grafts, commit-graph or multi-pack index hides anything, and nothing the repo names
// runs. Then every other stored copy of those names (firefsckcopies.go). It fails closed: an object that doesn't hold
// what its name says, one git can't read, and any look that can't be made in full keep the worktree and the branch,
// and the owner is told which object, and which path, commit and branch reach it.

// fireStoreTimeout is how long the fire's look may take (about 30 GB of history, inflated, on an M-series Mac).
const fireStoreTimeout = 2 * time.Minute

// storeLook hashes again every object tips reach in repo (storeHoldsWhatItNames is the real one): how many it hashed,
// the first that doesn't hold its name in the owner's words ("" for none), and err when the look couldn't be made.
type storeLook func(ctx context.Context, repo string, tips []string) (objects int, cause string, err error)

// fireStoreKept is "" when every object the repo's HEAD, its base branches and h's branch reach holds what its name
// says. Else it is the owner's words: the worktree and the branch were kept, why, and how to look. look nil is the
// real one. The words never say to run git gc or repack: those are what would serve a changed copy (B1).
func fireStoreKept(h Hire, look storeLook) string {
	if h.Worktree == "" {
		return "" // no worktree: nothing of the desk's is taken for merged
	}
	if look == nil {
		look = storeHoldsWhatItNames
	}
	var cause string
	var err error
	if h.Repo == "" || h.Branch == "" {
		err = errors.New("its row names no repo or no branch")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), fireStoreTimeout)
		_, cause, err = look(ctx, h.Repo, []string{"HEAD", "refs/heads/" + h.Branch})
		cancel()
	}
	kept := "Smolcorp kept " + calledHire(h) + "'s worktree " + h.Worktree + " and its branch " + h.Branch
	to := ". To look, run git fsck --full in " + h.Repo + "."
	switch {
	case err != nil:
		return kept + ": it couldn't check the repo's stored objects (" + err.Error() + "), so it didn't look at whether the branch is merged" + to
	case cause != "":
		return kept + ": the repo's stored objects were changed under it (" + cause + "), so what git shows for " + storeShownFor(h.Repo, h.Branch) + " can't be trusted, and it didn't look at whether the branch is merged" + to
	}
	return ""
}

// storeShownFor names, for the owner, what the look covered in repo: the branch checked out there, its base branches
// and the hire's branch ("feat, main and the branch"). It is read again here, for the words only.
func storeShownFor(repo, branch string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	read := storeReader(ctx, repo, gitHere().owner)
	var names []string
	add := func(name string) {
		if name != branch && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if head, status, err := read("symbolic-ref", "--short", "-q", "HEAD"); err == nil && status == 0 && head != "" {
		add(head)
	} else {
		add("the commit checked out there")
	}
	if bases, err := storeBaseRefs(read); err == nil {
		for _, b := range bases {
			add(strings.TrimPrefix(b[1], "refs/heads/"))
		}
	} else {
		add("its base branches")
	}
	return strings.Join(names, ", ") + " and the branch"
}

// storeReader reads repo with the core's read env and settings, in ctx: git's trimmed output and exit code.
func storeReader(ctx context.Context, repo string, owner [][2]string) func(args ...string) (string, int, error) {
	return func(args ...string) (string, int, error) {
		out, status, err := readGitOut(ctx, repo, gitCall{global: gitReadArgs, config: owner}, args...)
		return strings.TrimSpace(string(out)), status, err
	}
}

// storeBaseRefs are the repo's local base branches, as {id, ref}: main, master, the default branch its origin/HEAD
// names, and release and every branch under it. The same list a Mac desk's Policy keeps from it (FXREPLACEREFS #2),
// since the owner's checkout may be on another branch when the fire comes (RT126FIREFSCK's B2). The default branches
// are the repo's pin, read from origin/HEAD the first time (FXGITROWS2, RT126SWEEP2's B2), and what origin/HEAD names
// now when that is another (FXGITROWS2R1, N1: deskgitrows.go).
func storeBaseRefs(read func(args ...string) (string, int, error)) ([][2]string, error) {
	patterns := []string{"refs/heads/main", "refs/heads/master", "refs/heads/release"}
	common, status, err := read("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || status != 0 || !filepath.IsAbs(common) {
		return nil, errors.New("git couldn't say where its shared .git is")
	}
	var unsaid error
	pinned, now := pinnedDefaultBranches(common, func() string {
		origin, status, err := read("symbolic-ref", "-q", "refs/remotes/origin/HEAD")
		if err != nil {
			unsaid = errors.New("git couldn't say its default branch")
		}
		if def, ok := strings.CutPrefix(origin, "refs/remotes/origin/"); err == nil && status == 0 && ok {
			return def
		}
		return ""
	})
	if unsaid != nil {
		return nil, unsaid
	}
	for _, def := range []string{pinned, now} {
		if def != "" {
			patterns = append(patterns, "refs/heads/"+def)
		}
	}
	said, status, err := read(append([]string{"for-each-ref", "--format=%(objectname) %(refname)"}, patterns...)...)
	if err != nil || status != 0 {
		return nil, errors.New("git couldn't list its base branches")
	}
	var refs [][2]string
	for _, line := range strings.Split(said, "\n") {
		if id, ref, ok := strings.Cut(line, " "); ok && gitObjectID(id) {
			refs = append(refs, [2]string{id, ref})
		} else if line != "" {
			return nil, errors.New("git listed its base branches in words Smolcorp can't read: " + strconv.Quote(line))
		}
	}
	return refs, nil
}

// storeHoldsWhatItNames is the real storeLook: tips, and the repo's base branches next to HEAD. err is a look that
// couldn't be made (git missing or too old, a name it can't find, an object git can't read, a timeout, git's answer
// short of what it listed); cause, an object or a stored copy of it that doesn't hold its name, or is missing.
func storeHoldsWhatItNames(ctx context.Context, repo string, tips []string) (objects int, cause string, err error) {
	here := gitHere()
	if !gitVersionAtLeast(here.version, 2, 32) {
		return 0, "", errors.New("the git on this computer is missing, or older than 2.32")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return 0, "", err
	}
	read := storeReader(ctx, repo, here.owner)
	bases, err := storeBaseRefs(read)
	if err != nil {
		return 0, "", err
	}
	// Each tip's name is read first, so a name git can't find says so, and the walk is from exactly these. Only the
	// name: its object is read (and hashed) by the walk, as every other one. The base branches come right after HEAD
	// (or last), with the ids git listed them at.
	var walked, ids []string
	for _, tip := range tips {
		id, status, err := read("rev-parse", "--verify", "-q", "--end-of-options", tip)
		if err != nil || status != 0 || !gitObjectID(id) {
			return 0, "", errors.New("git couldn't find " + tip + " in it")
		}
		walked, ids = append(walked, tip), append(ids, id)
		if tip == "HEAD" {
			for _, b := range bases {
				walked, ids = append(walked, b[1]), append(ids, b[0])
			}
			bases = nil
		}
	}
	for _, b := range bases {
		walked, ids = append(walked, b[1]), append(ids, b[0])
	}
	tips = walked
	// Every stored copy, listed before the walk: the names with more than one are hashed again after it.
	folders, err := storeObjectFolders(read, repo)
	if err != nil {
		return 0, "", err
	}
	twice, err := storeSecondCopies(ctx, folders, len(ids[0])/2)
	if err != nil {
		return 0, "", err
	}
	defer closeStoreCopies(twice)
	env := gitReadEnv(os.Environ(), here.owner)
	command := func(args ...string) *exec.Cmd {
		cmd := disclaim.Cmd(exec.CommandContext(ctx, git, append(append([]string{"-C", repo}, gitReadArgs...), args...)...))
		cmd.Env, cmd.Dir = env, repo
		cmd.WaitDelay = 2 * time.Second
		stopGitGently(cmd)
		hideGitWindow(cmd)
		return cmd
	}
	// In commit order, each tree and file comes right after the first commit that names it, with its path: what
	// changed is said with the commit and path that reach it.
	list := command(append([]string{"rev-list", "--objects", "--in-commit-order", "--end-of-options"}, append(ids, "--")...)...)
	cat := command("cat-file", "--batch")
	var listSaid, catSaid gitOutput
	list.Stderr, cat.Stderr = &listSaid, &catSaid
	names, err1 := list.StdoutPipe()
	ask, err2 := cat.StdinPipe()
	out, err3 := cat.StdoutPipe()
	if err := errors.Join(err1, err2, err3); err != nil {
		return 0, "", err
	}
	if err := list.Start(); err != nil {
		return 0, "", err
	}
	if err := cat.Start(); err != nil {
		_ = list.Process.Kill()
		_ = list.Wait()
		return 0, "", err
	}
	// rev-list's names go to cat-file one by one, and where each was found to the hashing below, in the same order.
	found := make(chan storePlace, 4096)
	type passed struct {
		n     int
		whole bool // every name rev-list gave went on to cat-file
	}
	stop, listed := make(chan struct{}), make(chan passed, 1)
	go func() {
		n, whole := 0, false
		lines := bufio.NewReaderSize(names, 64<<10)
		defer func() { listed <- passed{n, whole} }()
		defer close(found)
		// On any way out, rev-list is read to its end, so it never waits on a pipe nobody reads (once cat-file has
		// ended, or both were stopped).
		defer func() { _, _ = io.Copy(io.Discard, lines) }()
		defer ask.Close()
		for {
			line, err := lines.ReadString('\n')
			if line = strings.TrimSuffix(line, "\n"); line != "" {
				name, path, named := strings.Cut(line, " ")
				select {
				case found <- storePlace{name: name, path: path, named: named}:
				case <-stop:
					return
				}
				if _, err := io.WriteString(ask, name+"\n"); err != nil {
					return
				}
				n++
			}
			if err != nil {
				whole = err == io.EOF
				return
			}
		}
	}()
	var again []storeFault // the names the walk hashed that have other stored copies
	objects, fault := hashBatch(bufio.NewReaderSize(out, 64<<10), found, func(f storeFault) {
		if name, err := hex.DecodeString(f.name); err == nil && len(twice) > 0 && twice[string(name)] != nil {
			again = append(again, f)
		}
	})
	close(stop)
	if fault.what != "" {
		// The first object that doesn't hold its name is enough: both are stopped.
		_ = cat.Process.Kill()
		_ = list.Process.Kill()
	}
	_, _ = io.Copy(io.Discard, out)
	sent := <-listed
	catErr, listErr := cat.Wait(), list.Wait()
	switch {
	case ctx.Err() != nil:
		return objects, "", fmt.Errorf("it took over %d minutes", int(fireStoreTimeout.Minutes()))
	case fault.what != "":
		return objects, fault.words(storeBranches(read, tips, ids, fault)), nil
	case listErr != nil || catErr != nil:
		// Not taken for changed (N1): a partial clone's or a gone object fails closed with git's own words.
		said := firstLine(listSaid.String())
		if said == "" {
			said = firstLine(catSaid.String())
		}
		if said == "" {
			said = errors.Join(listErr, catErr).Error()
		}
		return objects, "", errors.New("git couldn't read every object they reach: " + said)
	case objects == 0:
		return 0, "", errors.New("git listed no object")
	case !sent.whole:
		return objects, "", errors.New("git's list of them was cut short")
	case objects != sent.n:
		return objects, "", fmt.Errorf("git answered for %d of the %d objects it listed", objects, sent.n)
	}
	// Each other stored copy of a name the walk hashed: all of them, whichever one git reads now.
	for _, f := range again {
		name, _ := hex.DecodeString(f.name)
		for _, c := range twice[string(name)] {
			sound, err := c.holds(ctx, name)
			switch {
			case ctx.Err() != nil:
				return objects, "", fmt.Errorf("it took over %d minutes", int(fireStoreTimeout.Minutes()))
			case err != nil:
				f.what = "has a stored copy, in " + storeCopyPlace(repo, c) + ", that can't be read (" + err.Error() + ")"
			case !sound:
				f.what = "has a stored copy, in " + storeCopyPlace(repo, c) + ", that holds other bytes than its name says"
			default:
				continue
			}
			return objects, f.words(storeBranches(read, tips, ids, f)), nil
		}
	}
	return objects, "", nil
}

// storePlace is where rev-list found a name: the name as it listed it, and the path it gave a tree or file (named;
// "" for a commit's top folder).
type storePlace struct {
	name, path string
	named      bool
}

// storeFault is the first object that doesn't hold its name: what's wrong with it (what), its name and kind as git
// answered, where it was found, and the commit the walk was at (the object itself, for a commit).
type storeFault struct {
	what, name, kind, commit string
	at                       storePlace
}

// words says f for the owner, with the branches that reach its commit: "the object <id> holds other bytes than its
// name says: it is the file a/b in commit <id>, on main and hire/x".
func (f storeFault) words(branches []string) string {
	said := "the object " + f.name + " " + f.what
	switch {
	case !f.at.named && f.commit == f.name:
		said += ": it is the commit " + f.name
	case f.kind == "tag" && f.at.named:
		return said + ": it is the tag " + f.at.path // a tip's own: no commit or branch says more
	case f.commit == "":
		return said
	case !f.at.named:
		said += ": it is the " + f.kind + " " + f.name + ", named by commit " + f.commit
	case f.at.path == "":
		said += ": it is the top folder of commit " + f.commit
	case f.kind == "tree":
		said += ": it is the folder " + f.at.path + " in commit " + f.commit
	case f.kind == "blob":
		said += ": it is the file " + f.at.path + " in commit " + f.commit
	default: // git didn't say its kind
		said += ": it is the file or folder " + f.at.path + " in commit " + f.commit
	}
	if len(branches) > 0 {
		said += ", on " + strings.Join(branches, " and ")
	}
	return said
}

// storeBranches are the tips, by the names the owner knows (HEAD as its branch), whose history holds f's commit, as
// git reads it. A tip it can't tell about is left out: the fault is said either way.
func storeBranches(read func(args ...string) (string, int, error), tips, ids []string, f storeFault) []string {
	if f.commit == "" {
		return nil
	}
	var on []string
	for i, tip := range tips {
		if _, status, err := read("merge-base", "--is-ancestor", f.commit, ids[i]); err != nil || status != 0 {
			continue
		}
		name := strings.TrimPrefix(tip, "refs/heads/")
		if tip == "HEAD" {
			if branch, status, err := read("symbolic-ref", "--short", "-q", "HEAD"); err == nil && status == 0 && branch != "" {
				name = branch
			}
		}
		if !slices.Contains(on, name) { // HEAD is often a base branch too
			on = append(on, name)
		}
	}
	return on
}

// hashBatch reads `cat-file --batch`'s answer and hashes each object again, as git names it: "<type> <size>\0" and
// its bytes, under SHA-1 or SHA-256 by the name's length, and checks it against the name rev-list listed too (N4).
// found gives where each was listed, in the same order; sound, if not nil, is told each object that held its name.
// How many held their names, and the first that didn't (its what is "" when none).
func hashBatch(r *bufio.Reader, found <-chan storePlace, sound func(storeFault)) (objects int, fault storeFault) {
	commit := ""
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF && line == "" {
			return objects, storeFault{}
		}
		at, listed := <-found
		fields := strings.Fields(line)
		if listed && len(fields) > 0 && !at.named {
			commit = fields[0]
		}
		fault := storeFault{name: "", commit: commit, at: at}
		if len(fields) > 0 {
			fault.name = fields[0]
		}
		switch {
		case err != nil:
			fault.what = "broke off git's answer"
			return objects, fault
		case !listed:
			fault.what = "was answered for, though git listed no more"
			return objects, fault
		case len(fields) == 2 && fields[1] == "missing":
			fault.what = "is missing, or git can't read it" // a loose file that isn't one is "missing" to cat-file
			return objects, fault
		case len(fields) != 3:
			fault.what = "came with an answer git doesn't give: " + strconv.Quote(firstLine(line))
			return objects, fault
		}
		fault.kind = fields[1]
		size, err := strconv.ParseInt(fields[2], 10, 64)
		var sum hash.Hash
		switch len(fault.name) {
		case sha1.Size * 2:
			sum = sha1.New()
		case sha256.Size * 2:
			sum = sha256.New()
		}
		if err != nil || size < 0 || sum == nil {
			fault.what = "came with an answer git doesn't give: " + strconv.Quote(firstLine(line))
			return objects, fault
		}
		fmt.Fprintf(sum, "%s %d\x00", fault.kind, size)
		if n, err := io.CopyN(sum, r, size); err != nil || n != size {
			fault.what = "broke off git's answer"
			return objects, fault
		}
		if end, err := r.ReadByte(); err != nil || end != '\n' {
			fault.what = "broke off git's answer"
			return objects, fault
		}
		if hex.EncodeToString(sum.Sum(nil)) != fault.name {
			fault.what = "holds other bytes than its name says"
			return objects, fault
		}
		if fault.name != at.name {
			fault.what = "was answered for, though git listed " + at.name
			return objects, fault
		}
		if sound != nil {
			sound(fault)
		}
		objects++
	}
}
