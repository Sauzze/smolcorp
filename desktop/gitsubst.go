package desktop

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Git's substitutions already in a repo (FXGITPLANTSWARN; SIERRA's ruling #5 on FXREPLACEREFS §6, 17 Nov): what
// makes the owner's git read a branch as something it doesn't hold while the branch never moves. The core's own reads
// leave each one out (gitReadEnv), and on a Mac no desk may write one now, but one written before, or by a desk with
// no sandbox, still fools the owner's own git. So a desk's start looks in the repo's shared .git in the same pass as
// for the old git plants (gitPlantsAt), and lists each one in Setup with "Remove", which runs the command shown, and
// "Keep", which is remembered for that repo. The desk doesn't start while one is listed that the owner hasn't
// answered. Smolcorp removes nothing by itself.
//
//   - A replace ref (refs/replace/, loose or packed) and a grafts file (info/grafts) are always listed: nothing git
//     does by itself writes one. A replace ref can be a symbolic one (RT126GITPLANTSWARN B1): it follows another ref,
//     a branch a desk moves maybe, so what it substitutes can change at any time. That one is listed with what it
//     follows, also while that isn't there yet (git takes it once it is), and can't be kept; its Remove deletes the
//     replace ref itself, never the ref it follows (--no-deref).
//   - A shallow list (shallow) is normal in a shallow clone, whose history really ends at the commits it names: their
//     parents aren't in the repo. It's listed only when a commit it names has a parent that is: then it hides history
//     the repo holds. Raw commits are read for it (cat-file), which no shallow list, graft or replace ref changes.
//   - A multi-pack index (objects/pack/multi-pack-index, or its chain in multi-pack-index.d) is normal under git's
//     maintenance. It's listed only when `git multi-pack-index verify` fails: it places an object elsewhere than its
//     pack's own index does, or names a pack that isn't there.
//
// When a check can't be made (git fails, or there's too much to look at), what it would have checked is listed, in
// calmer words.

const (
	whyReplaceRef       = "A replace ref: wherever your git reads the object it names, it reads another one, so a branch can show what it doesn't hold."
	whyReplaceFollows   = "A replace ref that follows another ref: wherever your git reads the object it names, it reads what that ref holds now, which can move. It can't be kept."
	whyReplaceFollowsTo = "A replace ref that follows a ref that isn't there yet: once something makes that ref, wherever your git reads the object it names, it reads what the ref holds. It can't be kept."
	whyReplaceUnread    = "Smolcorp couldn't list the replace refs here, so what your git reads in place of a commit isn't known."
	whyGrafts           = "A grafts file: your git reads the commits it names with other parents, so a branch's history isn't what it holds."
	whyShallowCut       = "A shallow list that hides history this repository holds: your git stops at commits whose parents are here."
	whyShallowUnchecked = "A shallow list Smolcorp couldn't check. A shallow clone has one, but one that names other commits hides history from your git."
	whyMidxForged       = "A multi-pack index that doesn't match your packs (git multi-pack-index verify fails): your git can read the wrong bytes for a file."
	whyMidxUnchecked    = "A multi-pack index Smolcorp couldn't check. Git's maintenance writes one, but one that doesn't match your packs makes your git read the wrong bytes for a file."
	// gitShallowMost is the longest shallow list read: a longer one is listed as one Smolcorp couldn't check.
	gitShallowMost = 4 << 20
)

// gitSubst is what a substitution's Remove needs, as it was listed: its .git; a replace ref, the object it named and
// the ref it follows ("" for one that isn't symbolic); or the files it is, by their sum, and the shallow list's lines
// to take out (nil: the files go).
type gitSubst struct {
	gitDir, ref, target string
	follows             string
	files               []string
	sum                 string
	drop                []string
}

// gitPlantsAt is what a desk's start lists in gitDir, the repo's shared .git (makeDeskSandbox): its old git plants and
// git's substitutions there that the owner hasn't kept, with those substitutions on their own, which refuse the start.
func gitPlantsAt(gitDir string) (listed, held []oldGitPlant) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	listed = findOldGitPlants(ctx, gitDir)
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), gitTimeout)
	held = notKeptGitSubsts(gitDir, findGitSubsts(ctx, gitDir))
	cancel()
	return append(listed, held...), held
}

// gitSubstRefusal is a start refused while held lists a substitution the owner hasn't answered: the words for them.
func gitSubstRefusal(held []oldGitPlant) string {
	more := ""
	if n := len(held) - 1; n > 0 {
		more = " and " + map[bool]string{true: "1 more", false: strconv.Itoa(n) + " more"}[n == 1]
	}
	keep := ", or keep it if it's yours,"
	if !gitSubstKeepable(held[0]) {
		keep = ","
	}
	return "Smolcorp didn't start the desk: your repository's .git holds " + held[0].File + more + ". " + held[0].Why +
		" Remove it in Hires and roles, under Sandbox" + keep + " then start the desk again."
}

// gitSubstKeepable is whether the owner can keep p: one of git's substitutions that follows no other ref. One that
// does is listed and held on every start until it's removed, as what it substitutes can change with no start between.
func gitSubstKeepable(p oldGitPlant) bool { return p.Held && p.Follows == "" }

// findGitSubsts lists git's substitutions in gitDir (above). It only reads.
func findGitSubsts(ctx context.Context, gitDir string) []oldGitPlant {
	look := gitCall{global: gitReadArgs, config: gitHere().owner}
	found := replaceRefs(ctx, gitDir, look)
	if grafts := filepath.Join(gitDir, "info", "grafts"); !nothingAt(grafts) {
		found = append(found, gitSubstFiles(gitDir, whyGrafts, grafts))
	}
	if p, ok := shallowSubst(ctx, gitDir, look); ok {
		found = append(found, p)
	}
	if p, ok := midxSubst(ctx, gitDir, look); ok {
		found = append(found, p)
	}
	return found
}

// nothingAt is whether nothing is at at: a name that can't be looked at counts as there.
func nothingAt(at string) bool {
	_, err := os.Lstat(at)
	return errors.Is(err, fs.ErrNotExist)
}

// replaceRefs lists each replace ref, as git reads them (loose or packed), with the ref a symbolic one follows, and
// each loose symbolic one git doesn't list as what it follows isn't there yet (replaceRefsToCome). Its Remove deletes
// the replace ref only while it still names what it named and follows what it followed, and never through to the ref
// it follows (--no-deref, in the command shown too).
func replaceRefs(ctx context.Context, gitDir string, look gitCall) []oldGitPlant {
	out, status, err := readGitOut(ctx, gitDir, look, "for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/replace/")
	if err != nil || status != 0 {
		return []oldGitPlant{gitPlant(oldGitPlant{File: filepath.Join(gitDir, "refs", "replace"), Why: whyReplaceUnread, Held: true})}
	}
	var found []oldGitPlant
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || len(f) > 3 {
			continue
		}
		ref, target, follows, why := f[0], f[1], "", whyReplaceRef
		if len(f) == 3 {
			follows, why = f[2], whyReplaceFollows
		}
		seen[ref] = true
		found = append(found, gitPlant(oldGitPlant{File: filepath.Join(gitDir, filepath.FromSlash(ref)), Value: target, Follows: follows,
			Why: why, Held: true, Command: "git --git-dir=" + shellQuoted(gitDir) + " update-ref --no-deref -d " + shellQuoted(ref) + " " + target,
			subst: gitSubst{gitDir: gitDir, ref: ref, target: target, follows: follows}}))
	}
	return append(found, replaceRefsToCome(gitDir, seen)...)
}

// replaceRefsToCome is each loose replace ref in gitDir not in listed that is symbolic: git lists none whose ref isn't
// there, and takes it once that ref is made, by a desk too. Its Remove takes the file away while it's as listed. With
// too many files to look through, the replace refs are listed as ones Smolcorp couldn't list.
func replaceRefsToCome(gitDir string, listed map[string]bool) []oldGitPlant {
	var found []oldGitPlant
	n, root := 0, filepath.Join(gitDir, "refs", "replace")
	err := filepath.WalkDir(root, func(at string, d fs.DirEntry, err error) error {
		if n++; err != nil || n > gitPlantMax {
			return cmp.Or(err, errors.New("too many files"))
		}
		rel, _ := filepath.Rel(root, at)
		// A link is read as git reads it, through to a file only (one to a FIFO would hang the start). Its Remove takes
		// the link away, not what it leads to.
		if info, err := os.Stat(at); d.IsDir() || err != nil || !info.Mode().IsRegular() || listed["refs/replace/"+filepath.ToSlash(rel)] {
			return nil
		}
		data, err := readMost(at, 4096)
		if follows, ok := strings.CutPrefix(string(data), "ref:"); err == nil && ok {
			p := gitSubstFiles(gitDir, whyReplaceFollowsTo, at)
			p.Follows = strings.TrimSpace(follows)
			found = append(found, gitPlant(p))
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return []oldGitPlant{gitPlant(oldGitPlant{File: root, Why: whyReplaceUnread, Held: true})}
	}
	return found
}

// gitSubstFiles is the substitution why, made of files in gitDir, by their sum: its Remove takes them away while
// they're as listed. One that can't be read has no Remove.
func gitSubstFiles(gitDir, why string, files ...string) oldGitPlant {
	p := oldGitPlant{File: files[0], Why: why, Held: true, subst: gitSubst{gitDir: gitDir, files: files}}
	if sum, err := gitSubstSum(files); err == nil {
		p.Value, p.subst.sum = sum, sum
		words := []string{"rm"}
		if slices.ContainsFunc(files, func(f string) bool { info, err := os.Lstat(f); return err == nil && info.IsDir() }) {
			words = append(words, "-r")
		}
		for _, f := range files {
			words = append(words, shellQuoted(f))
		}
		p.Command = strings.Join(words, " ")
	}
	return gitPlant(p)
}

// gitSubstSum is what files are now: names, kinds, bytes and where links lead.
func gitSubstSum(files []string) (string, error) {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00", f)
		if err := sumClaudeTree(h, f); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// shallowSubst is the shallow list, if it hides history the repo holds (shallowCuts). When only some of its commits
// do, Remove takes those lines out and keeps the rest, as a shallow clone needs them; one it couldn't check has no
// Remove, as it may be a shallow clone's own.
func shallowSubst(ctx context.Context, gitDir string, look gitCall) (oldGitPlant, bool) {
	at := filepath.Join(gitDir, "shallow")
	data, err := readMost(at, gitShallowMost)
	if errors.Is(err, fs.ErrNotExist) {
		return oldGitPlant{}, false
	}
	lines := strings.Fields(string(data))
	var cut []string
	if err == nil {
		cut, err = shallowCuts(ctx, gitDir, look, lines)
	}
	if err != nil {
		p := gitSubstFiles(gitDir, whyShallowUnchecked, at)
		p.Command = ""
		return p, true
	}
	if len(cut) == 0 {
		return oldGitPlant{}, false
	}
	p := gitSubstFiles(gitDir, whyShallowCut, at)
	if p.Command != "" && len(cut) < len(slices.Compact(slices.Sorted(slices.Values(lines)))) {
		p.subst.drop = cut
		words := []string{"grep -v -x -F"}
		for _, c := range cut {
			words = append(words, "-e "+c)
		}
		lock := shellQuoted(at + ".lock")
		p.Command = strings.Join(words, " ") + " " + shellQuoted(at) + " > " + lock + " && mv " + lock + " " + shellQuoted(at)
	}
	return p, true
}

// shallowCuts are the commits of a shallow list's lines with a parent in the repo, once each, in order.
func shallowCuts(ctx context.Context, gitDir string, look gitCall, lines []string) ([]string, error) {
	if len(lines) > gitPlantMax || slices.ContainsFunc(lines, func(l string) bool { return !gitObjectID(l) }) {
		return nil, errors.New("not a shallow list Smolcorp can check")
	}
	if len(lines) == 0 {
		return nil, nil
	}
	out, err := readGitIn(ctx, gitDir, look, lines, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	parents := map[string][]string{}
	for len(out) > 0 {
		header, rest, ok := bytes.Cut(out, []byte("\n"))
		f := strings.Fields(string(header))
		if ok && len(f) == 2 && f[1] == "missing" {
			out = rest
			continue
		}
		size := -1
		if ok && len(f) == 3 {
			size, _ = strconv.Atoi(f[2])
		}
		if size < 0 || size+1 > len(rest) {
			return nil, errors.New("git cat-file answered what Smolcorp can't read")
		}
		for line := range strings.Lines(string(rest[:size])) {
			if line == "\n" {
				break // the end of a commit's header
			}
			if parent, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "parent "); ok && f[1] == "commit" {
				parents[f[0]] = append(parents[f[0]], parent)
			}
		}
		out = rest[size+1:]
	}
	var all []string
	for _, ps := range parents {
		all = append(all, ps...)
	}
	if len(all) == 0 {
		return nil, nil
	}
	out, err = readGitIn(ctx, gitDir, look, all, "cat-file", "--batch-check=%(objectname) %(objecttype)")
	if err != nil {
		return nil, err
	}
	here := map[string]bool{}
	for line := range strings.Lines(string(out)) {
		if f := strings.Fields(line); len(f) == 2 && f[1] != "missing" {
			here[f[0]] = true
		}
	}
	var cut []string
	for _, c := range lines {
		if !slices.Contains(cut, c) && slices.ContainsFunc(parents[c], func(p string) bool { return here[p] }) {
			cut = append(cut, c)
		}
	}
	return cut, nil
}

// readGitIn is readGitOut with lines, one each, as git's standard input; a git that didn't end well is an error.
func readGitIn(ctx context.Context, dir string, call gitCall, lines []string, args ...string) ([]byte, error) {
	out, status, err := execGitIn(ctx, append(gitReadEnv(os.Environ(), call.config), call.env...),
		strings.NewReader(strings.Join(lines, "\n")+"\n"), append(append([]string{"-C", dir}, call.global...), args...)...)
	if err == nil && status != 0 {
		err = fmt.Errorf("git %s exited %d", args[0], status)
	}
	return out, err
}

// midxSubst is the multi-pack index, if `git multi-pack-index verify` doesn't pass it. Remove takes it away, which is
// always safe: git reads each pack by its own index without one, and its maintenance writes a new one.
func midxSubst(ctx context.Context, gitDir string, look gitCall) (oldGitPlant, bool) {
	pack := filepath.Join(gitDir, "objects", "pack")
	var files []string
	for _, name := range []string{"multi-pack-index", "multi-pack-index.d"} {
		if at := filepath.Join(pack, name); !nothingAt(at) {
			files = append(files, at)
		}
	}
	if len(files) == 0 {
		return oldGitPlant{}, false
	}
	_, status, err := readGitOut(ctx, gitDir, look, "multi-pack-index", "verify", "--no-progress")
	switch {
	case err == nil && status == 0:
		return oldGitPlant{}, false
	case err == nil && status == 1: // what verify answers when it finds the index wrong
		return gitSubstFiles(gitDir, whyMidxForged, files...), true
	}
	return gitSubstFiles(gitDir, whyMidxUnchecked, files...), true
}

// removeGitSubst is the owner's "Remove" for substitution p, only while it's as listed: a replace ref is deleted by
// git, and only while it names what it named; files go while their sum is the same; a shallow list's cut lines are
// taken out and the rest kept, written through shallow.lock as git writes it. One with no Remove is left alone.
func removeGitSubst(ctx context.Context, p oldGitPlant) error {
	s := p.subst
	switch {
	case p.Command == "":
		return errors.New("Smolcorp can't remove this: look at it yourself")
	case s.ref != "":
		look := gitCall{global: gitReadArgs, config: gitHere().owner}
		out, status, err := readGitOut(ctx, s.gitDir, look, "for-each-ref", "--format=%(objectname) %(symref)", s.ref)
		if err != nil || status != 0 || strings.TrimSuffix(string(out), "\n") != s.target+" "+s.follows {
			return errGitPlantGone
		}
		// --no-deref: a symbolic replace ref is deleted itself, not the ref it follows (RT126GITPLANTSWARN B1).
		if _, status, err = readGitOut(ctx, s.gitDir, look, "update-ref", "--no-deref", "-d", s.ref, s.target); err == nil && status != 0 {
			err = fmt.Errorf("git update-ref exited %d", status)
		}
		return err
	}
	if sum, err := gitSubstSum(s.files); err != nil || sum != s.sum {
		return errGitPlantGone
	}
	if s.drop == nil {
		for _, f := range s.files {
			if err := os.RemoveAll(f); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := readMost(s.files[0], gitShallowMost)
	if err != nil {
		return err
	}
	var kept strings.Builder
	for _, line := range strings.Fields(string(data)) {
		if !slices.Contains(s.drop, line) {
			kept.WriteString(line + "\n")
		}
	}
	lock := s.files[0] + ".lock"
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) // git's own lock: one it holds refuses this
	if err != nil {
		return err
	}
	_, err = f.WriteString(kept.String())
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(lock, s.files[0])
	}
	if err != nil {
		os.Remove(lock)
	}
	return err
}

// notKeptGitSubsts is found but what the owner kept in gitDir. What was kept there and isn't found any more is
// forgotten, so one that comes back is listed again.
func notKeptGitSubsts(gitDir string, found []oldGitPlant) []oldGitPlant {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	kept, still := claudeHeld.gitKept[gitDir], map[string]bool{}
	var held []oldGitPlant
	for _, p := range found {
		if kept[p.ID] && gitSubstKeepable(p) {
			still[p.ID] = true
		} else {
			held = append(held, p)
		}
	}
	if len(still) != len(kept) {
		claudeHeld.gitKept[gitDir] = still
		if len(still) == 0 {
			delete(claudeHeld.gitKept, gitDir)
		}
		saveClaudeHeldLocked(true) // fewer kept is more held
	}
	return held
}

// keepGitSubst is the owner's "Keep" for substitution id in gitDir: it's theirs, so it isn't listed there again, nor
// refuses a start, unless it changes. Kept on record, as the Claude plants they keep are (claudeheld.go).
func keepGitSubst(gitDir, id string) {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	if claudeHeld.gitKept[gitDir] == nil {
		claudeHeld.gitKept[gitDir] = map[string]bool{}
	}
	claudeHeld.gitKept[gitDir][id] = true
	saveClaudeHeldLocked(false)
}
