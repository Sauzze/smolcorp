package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXGITPLANTSWARN's rows: git's substitutions already in a repo (a replace ref, grafts, a shallow list or a
// multi-pack index that hides what the repo holds) are listed before a desk starts, refuse the start until the owner
// removes or keeps each one, and a keep is remembered for that repo. A clean repo, a real shallow clone and git's own
// multi-pack index start with nothing asked. Throwaway repos only (aSubstRepo: main is A then B, f = "base"; the
// desk's D, with no parent, has f = "desk").

// freshGitKept starts a test with nothing kept, on record in the support folder of the moment, and puts back what was
// kept after it.
func freshGitKept(t *testing.T) {
	t.Helper()
	freshClaudeHeld(t)
	claudeHeld.Lock()
	saved := claudeHeld.gitKept
	claudeHeld.gitKept = map[string]map[string]bool{}
	claudeHeld.Unlock()
	t.Cleanup(func() { claudeHeld.Lock(); claudeHeld.gitKept = saved; claudeHeld.Unlock() })
}

// gitOf is r's shared .git.
func (r substRepo) gitOf() string { return filepath.Join(r.dir, ".git") }

// heldAt is what a desk's start in gitDir holds, failing the test unless each one is also listed for Setup.
func heldAt(t *testing.T, gitDir string) []oldGitPlant {
	t.Helper()
	listed, held := gitPlantsAt(gitDir)
	for _, p := range held {
		if !p.Held || !slices.ContainsFunc(listed, func(q oldGitPlant) bool { return q.ID == p.ID }) {
			t.Errorf("held but not listed for Setup: %+v", p)
		}
	}
	return held
}

// aShallowClone is a real shallow clone of r (--depth 1): main only, its shallow list B.
func aShallowClone(t *testing.T, r substRepo) substRepo {
	t.Helper()
	c := substRepo{dir: filepath.Join(physical(t.TempDir()), "clone"), a: r.a, b: r.b, d: r.d}
	gitRepoCmd(t, filepath.Dir(c.dir), "clone", "-q", "--depth", "1", "file://"+r.dir, c.dir)
	gitRepoCmd(t, c.dir, "config", "gc.auto", "0")
	return c
}

func TestEachOfGitsSubstitutionsIsListedAndRefusesTheStart(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	words := func(p oldGitPlant) string {
		return "Smolcorp didn't start the desk: your repository's .git holds " + p.File + ". " + p.Why +
			" Remove it in Hires and roles, under Sandbox, or keep it if it's yours, then start the desk again."
	}
	type want struct{ file, why, command string }
	cases := []struct {
		name  string
		plant func(t *testing.T) (substRepo, want)
	}{
		{"a replace ref", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			gitRepoCmd(t, r.dir, "replace", r.b, r.d)
			if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
				t.Fatalf("the owner's git reads main:f as %q: the replace ref isn't taken", got)
			}
			return r, want{filepath.Join(r.gitOf(), "refs", "replace", r.b), whyReplaceRef,
				"git --git-dir='" + r.gitOf() + "' update-ref --no-deref -d 'refs/replace/" + r.b + "' " + r.d}
		}},
		{"a packed replace ref", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			gitRepoCmd(t, r.dir, "replace", r.b, r.d)
			gitRepoCmd(t, r.dir, "pack-refs", "--all")
			if !nothingAt(filepath.Join(r.gitOf(), "refs", "replace", r.b)) {
				t.Fatal("the replace ref is still loose")
			}
			return r, want{filepath.Join(r.gitOf(), "refs", "replace", r.b), whyReplaceRef,
				"git --git-dir='" + r.gitOf() + "' update-ref --no-deref -d 'refs/replace/" + r.b + "' " + r.d}
		}},
		{"grafts", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.d+"\n")
			at := filepath.Join(r.gitOf(), "info", "grafts")
			return r, want{at, whyGrafts, "rm '" + at + "'"}
		}},
		{"a shallow list in a whole repo", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			writeRepoFile(t, r.gitOf(), "shallow", r.b+"\n")
			if _, status := r.plain(t, "merge-base", "--is-ancestor", r.a, "main"); status == 0 {
				t.Fatal("the owner's git still finds A in main's history: the shallow list isn't taken")
			}
			at := filepath.Join(r.gitOf(), "shallow")
			return r, want{at, whyShallowCut, "rm '" + at + "'"}
		}},
		{"a shallow clone's list with a line added", func(t *testing.T) (substRepo, want) {
			c := aShallowClone(t, aSubstRepo(t))
			writeRepoFile(t, c.dir, "f", "next\n")
			gitRepoCmd(t, c.dir, "commit", "-q", "-am", "C")
			next := gitRepoCmd(t, c.dir, "rev-parse", "HEAD")
			at := filepath.Join(c.gitOf(), "shallow")
			data, _ := os.ReadFile(at)
			writeRepoFile(t, c.gitOf(), "shallow", string(data)+next+"\n")
			return c, want{at, whyShallowCut, "grep -v -x -F -e " + next + " '" + at + "' > '" + at + ".lock' && mv '" + at + ".lock' '" + at + "'"}
		}},
		{"a shallow list Smolcorp can't check", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			writeRepoFile(t, r.gitOf(), "shallow", "not a commit\n")
			return r, want{filepath.Join(r.gitOf(), "shallow"), whyShallowUnchecked, ""}
		}},
		{"a multi-pack index that places objects elsewhere", func(t *testing.T) (substRepo, want) {
			r := aMidxRepo(t)
			if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
				t.Fatalf("the owner's git reads main:f as %q: the multi-pack index isn't taken", got)
			}
			at := filepath.Join(r.gitOf(), "objects", "pack", "multi-pack-index")
			return r, want{at, whyMidxForged, "rm '" + at + "'"}
		}},
		{"a multi-pack index naming a pack that isn't there", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			gitRepoCmd(t, r.dir, "repack", "-q", "-d")
			first, _ := filepath.Glob(filepath.Join(r.gitOf(), "objects", "pack", "pack-*"))
			gitRepoCmd(t, r.dir, "repack", "-q", "-a") // a second pack holding everything; the first stays
			gitRepoCmd(t, r.dir, "multi-pack-index", "write")
			for _, f := range first {
				if err := os.Remove(f); err != nil {
					t.Fatal(err)
				}
			}
			at := filepath.Join(r.gitOf(), "objects", "pack", "multi-pack-index")
			return r, want{at, whyMidxForged, "rm '" + at + "'"}
		}},
		{"a multi-pack index chain that places objects elsewhere", func(t *testing.T) (substRepo, want) {
			r := aSubstRepo(t)
			gitRepoCmd(t, r.dir, "repack", "-q", "-a", "-d")
			gitRepoCmd(t, r.dir, "multi-pack-index", "write", "--incremental")
			layers, _ := filepath.Glob(filepath.Join(r.gitOf(), "objects", "pack", "multi-pack-index.d", "*.midx"))
			if len(layers) != 1 {
				t.Skipf("this git writes no incremental multi-pack index: %v", layers)
			}
			rewriteSealed(t, layers[0], func(data []byte) {
				chunks := gitChunks(t, data, int(data[6]), 12)
				i, j := chunks["OOFF"], chunks["OOFF"]+8
				a := slices.Clone(data[i : i+8])
				copy(data[i:], data[j:j+8])
				copy(data[j:], a)
			})
			at := filepath.Join(r.gitOf(), "objects", "pack", "multi-pack-index.d")
			return r, want{at, whyMidxForged, "rm -r '" + at + "'"}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, w := c.plant(t)
			held := heldAt(t, r.gitOf())
			if len(held) != 1 {
				t.Fatalf("held %d, want 1: %+v", len(held), held)
			}
			p := held[0]
			if p.File != w.file || p.Why != w.why || p.Command != w.command {
				t.Errorf("listed as\n  %q\n  %q\n  %q\nwant\n  %q\n  %q\n  %q", p.File, p.Why, p.Command, w.file, w.why, w.command)
			}
			if got := gitSubstRefusal(held); got != words(p) {
				t.Errorf("the start's words: %q", got)
			}
		})
	}
	// More than one: the first is named, with how many more.
	r := aSubstRepo(t)
	gitRepoCmd(t, r.dir, "replace", r.b, r.d)
	writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.d+"\n")
	writeRepoFile(t, r.gitOf(), "shallow", r.b+"\n")
	held := heldAt(t, r.gitOf())
	if got := gitSubstRefusal(held); len(held) != 3 || !strings.Contains(got, held[0].File+" and 2 more. "+held[0].Why) {
		t.Errorf("three held (%d): %q", len(held), got)
	}
}

func TestACleanRepoAShallowCloneAndGitsOwnMultiPackIndexStartWithNothingAsked(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	r := aSubstRepo(t)
	if held := heldAt(t, r.gitOf()); len(held) != 0 {
		t.Errorf("a clean repo: %+v", held)
	}
	c := aShallowClone(t, r)
	if data, err := os.ReadFile(filepath.Join(c.gitOf(), "shallow")); err != nil || strings.TrimSpace(string(data)) != r.b {
		t.Fatalf("not a shallow clone at B: %q %v", data, err)
	}
	if held := heldAt(t, c.gitOf()); len(held) != 0 {
		t.Errorf("a shallow clone: %+v", held)
	}
	gitRepoCmd(t, r.dir, "repack", "-q", "-a", "-d")
	gitRepoCmd(t, r.dir, "multi-pack-index", "write")
	if nothingAt(filepath.Join(r.gitOf(), "objects", "pack", "multi-pack-index")) {
		t.Fatal("no multi-pack index was written")
	}
	if held := heldAt(t, r.gitOf()); len(held) != 0 {
		t.Errorf("git's own multi-pack index: %+v", held)
	}
}

func TestAKeptSubstitutionIsRememberedForItsRepoUntilItChanges(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	r, other := aSubstRepo(t), aSubstRepo(t)
	for _, repo := range []substRepo{r, other} {
		gitRepoCmd(t, repo.dir, "replace", repo.b, repo.d)
	}
	held := heldAt(t, r.gitOf())
	if len(held) != 1 {
		t.Fatalf("held: %+v", held)
	}
	keepGitSubst(r.gitOf(), held[0].ID)
	listed, again := gitPlantsAt(r.gitOf())
	if len(again) != 0 || slices.ContainsFunc(listed, func(p oldGitPlant) bool { return p.Held }) {
		t.Errorf("kept, then held or listed: %+v %+v", again, listed)
	}
	claudeRestart()
	if again := heldAt(t, r.gitOf()); len(again) != 0 {
		t.Errorf("kept, then held after a restart: %+v", again)
	}
	if held := heldAt(t, other.gitOf()); len(held) != 1 {
		t.Errorf("kept in one repo, but not held in another: %+v", held)
	}
	// It changes (the ref names another commit): listed again.
	gitRepoCmd(t, r.dir, "replace", "-f", r.b, r.a)
	if again := heldAt(t, r.gitOf()); len(again) != 1 || again[0].Value != r.a {
		t.Errorf("changed, but not held again: %+v", again)
	}
	// Gone, then back as it was kept: listed again, as what was kept and isn't found any more is forgotten.
	keepGitSubst(r.gitOf(), heldAt(t, r.gitOf())[0].ID)
	gitRepoCmd(t, r.dir, "replace", "-d", r.b)
	if again := heldAt(t, r.gitOf()); len(again) != 0 {
		t.Fatalf("none there: %+v", again)
	}
	gitRepoCmd(t, r.dir, "replace", r.b, r.a)
	if again := heldAt(t, r.gitOf()); len(again) != 1 {
		t.Errorf("back after it was gone, but not held: %+v", again)
	}
}

func TestRemoveClearsEachSubstitutionOnlyAsListed(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	ctx := context.Background()
	remove := func(t *testing.T, gitDir string) {
		t.Helper()
		held := heldAt(t, gitDir)
		if len(held) != 1 {
			t.Fatalf("held: %+v", held)
		}
		if err := removeOldGitPlant(ctx, held, held[0].ID); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if again := heldAt(t, gitDir); len(again) != 0 {
			t.Errorf("still held after Remove: %+v", again)
		}
	}
	t.Run("a replace ref", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "replace", r.b, r.d)
		gitRepoCmd(t, r.dir, "pack-refs", "--all")
		remove(t, r.gitOf())
		if got, _ := r.plain(t, "show", "main:f"); got != "base" {
			t.Errorf("main:f after Remove: %q", got)
		}
	})
	t.Run("grafts", func(t *testing.T) {
		r := aSubstRepo(t)
		writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.d+"\n")
		remove(t, r.gitOf())
		if !nothingAt(filepath.Join(r.gitOf(), "info", "grafts")) {
			t.Error("the grafts file stayed")
		}
	})
	t.Run("a shallow list in a whole repo", func(t *testing.T) {
		r := aSubstRepo(t)
		writeRepoFile(t, r.gitOf(), "shallow", r.b+"\n")
		remove(t, r.gitOf())
		if _, status := r.plain(t, "merge-base", "--is-ancestor", r.a, "main"); status != 0 {
			t.Error("A isn't in main's history after Remove")
		}
	})
	t.Run("a shallow clone's added line", func(t *testing.T) {
		c := aShallowClone(t, aSubstRepo(t))
		writeRepoFile(t, c.dir, "f", "next\n")
		gitRepoCmd(t, c.dir, "commit", "-q", "-am", "C")
		next := gitRepoCmd(t, c.dir, "rev-parse", "HEAD")
		writeRepoFile(t, c.gitOf(), "shallow", c.b+"\n"+next+"\n")
		remove(t, c.gitOf())
		if data, err := os.ReadFile(filepath.Join(c.gitOf(), "shallow")); err != nil || string(data) != c.b+"\n" {
			t.Errorf("the clone's own line isn't all that's left: %q %v", data, err)
		}
		if got, status := c.plain(t, "rev-list", "--count", "main"); got != "2" || status != 0 {
			t.Errorf("the clone's history after Remove: %q %d", got, status)
		}
	})
	t.Run("a multi-pack index", func(t *testing.T) {
		r := aMidxRepo(t)
		remove(t, r.gitOf())
		if got, _ := r.plain(t, "show", "main:f"); got != "base" {
			t.Errorf("main:f after Remove: %q", got)
		}
	})
	t.Run("changed since it was listed", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "replace", r.b, r.d)
		writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.d+"\n")
		held := heldAt(t, r.gitOf())
		gitRepoCmd(t, r.dir, "replace", "-f", r.b, r.a)
		writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.a+"\n")
		for _, p := range held {
			if err := removeOldGitPlant(ctx, held, p.ID); !errors.Is(err, errGitPlantGone) {
				t.Errorf("Remove of %s, changed: %v", p.File, err)
			}
		}
		if got, _ := r.plain(t, "rev-parse", "refs/replace/"+r.b); got != r.a {
			t.Errorf("a changed replace ref was touched: %q", got)
		}
		if data, _ := os.ReadFile(filepath.Join(r.gitOf(), "info", "grafts")); string(data) != r.b+" "+r.a+"\n" {
			t.Errorf("a changed grafts file was touched: %q", data)
		}
	})
	t.Run("one Smolcorp can't check", func(t *testing.T) {
		r := aSubstRepo(t)
		writeRepoFile(t, r.gitOf(), "shallow", "not a commit\n")
		held := heldAt(t, r.gitOf())
		if err := removeOldGitPlant(ctx, held, held[0].ID); err == nil || errors.Is(err, errGitPlantGone) || nothingAt(filepath.Join(r.gitOf(), "shallow")) {
			t.Errorf("Remove of one it couldn't check: %v", err)
		}
	})
}

// Setup's row: a substitution shows its command, a Remove and a Keep; Keep takes it off the list and is on record; an
// old plant can't be kept.
func TestSetupShowsASubstitutionWithItsCommandAndKeepsIt(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	freshGitPlants(t)
	d := newTestDeck(t)
	freshGitKept(t)
	r := aSubstRepo(t)
	gitRepoCmd(t, r.dir, "replace", r.b, r.d)
	writeRepoFile(t, r.gitOf(), "hooks/pre-commit", "#!/bin/sh\n")
	listed, held := gitPlantsAt(r.gitOf())
	keepOldGitPlants("aaaa00000011", r.gitOf(), listed)
	ref, hook := filepath.Join(r.gitOf(), "refs", "replace", r.b)+" ", filepath.Join(r.gitOf(), "hooks", "pre-commit")+" "
	plants := rowPlants(t, d)
	if row := plants[ref]; row == nil || row["held"] != true || row["removable"] != true || row["command"] != held[0].Command || row["why"] != whyReplaceRef ||
		row["keepable"] != true || row["follows"] != nil {
		t.Fatalf("the substitution's row: %v", row)
	}
	if row := plants[hook]; row == nil || row["held"] != nil || row["command"] != nil || row["keepable"] != nil {
		t.Fatalf("the hook's row: %v", row)
	}
	for _, c := range d.selfTest() {
		if c.ID == "sandbox" && (c.Status != checkTodo || !strings.Contains(c.Detail, "1 hook or setting") ||
			!strings.Contains(c.Detail, "A desk didn't start, as a repository's .git holds something that changes what your git reads.") ||
			strings.Contains(c.Detail, r.dir)) {
			t.Errorf("Start here: %+v", c)
		}
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?keep=1&id="+plants[hook]["id"].(string)); code != 409 || body != gitSubstNotKept {
		t.Errorf("Keep of a hook: %d %s", code, body)
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?keep=1&id="+plants[ref]["id"].(string)); code != 200 || strings.Contains(body, "refs/replace") {
		t.Errorf("Keep: %d %s", code, body)
	}
	if got, _ := r.plain(t, "rev-parse", "refs/replace/"+r.b); got != r.d {
		t.Errorf("Keep touched the ref: %q", got)
	}
	claudeRestart()
	if again := heldAt(t, r.gitOf()); len(again) != 0 {
		t.Errorf("kept in Setup, then held after a restart: %+v", again)
	}
	if rowPlants(t, d)[hook] == nil {
		t.Error("the hook left the row with the Keep")
	}
}

// RT126GITPLANTSWARN B1: a replace ref that is a symbolic one follows another ref, an owner's branch or a desk's, so
// what it substitutes changes when that ref moves. It's listed with what it follows, also while that ref isn't there
// yet; it can't be kept, in Setup or on record; and its Remove deletes the replace ref itself, never the branch it
// follows (the reviewer's TestRTGitPlantsSymrefReplaceRemove), in the command shown too.
func TestASymbolicReplaceRefIsListedWithWhatItFollowsAndOnlyRemoved(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	freshGitPlants(t)
	d := newTestDeck(t)
	freshGitKept(t)
	ctx := context.Background()
	noKeep := func(t *testing.T, gitDir string, p oldGitPlant) {
		t.Helper()
		if got, want := gitSubstRefusal([]oldGitPlant{p}), "Smolcorp didn't start the desk: your repository's .git holds "+p.File+". "+p.Why+
			" Remove it in Hires and roles, under Sandbox, then start the desk again."; got != want {
			t.Errorf("the start's words:\n  %q\nwant\n  %q", got, want)
		}
		keepGitSubst(gitDir, p.ID) // on record from before (or by hand): still held
		if again := heldAt(t, gitDir); len(again) != 1 || again[0].ID != p.ID {
			t.Errorf("kept on record, then not held: %+v", again)
		}
		keepOldGitPlants("aaaa00000012", gitDir, []oldGitPlant{p})
		row := rowPlants(t, d)[p.File+" "]
		if row == nil || row["follows"] != p.Follows || row["keepable"] != nil || row["removable"] != true || row["held"] != true {
			t.Errorf("the row: %v", row)
		}
		if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?keep=1&id="+p.ID); code != 409 || body != gitSubstFollowsNotKept {
			t.Errorf("Keep in Setup: %d %s", code, body)
		}
	}
	t.Run("it follows a branch", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "branch", "owner-work", r.d) // an owner's branch, at D
		writeRepoFile(t, r.gitOf(), "refs/replace/"+r.b, "ref: refs/heads/owner-work\n")
		if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
			t.Fatalf("the owner's git reads main:f as %q: the symbolic replace ref isn't taken", got)
		}
		held := heldAt(t, r.gitOf())
		if len(held) != 1 {
			t.Fatalf("held: %+v", held)
		}
		p := held[0]
		if p.File != filepath.Join(r.gitOf(), "refs", "replace", r.b) || p.Follows != "refs/heads/owner-work" || p.Value != r.d ||
			p.Why != whyReplaceFollows || p.Command != "git --git-dir='"+r.gitOf()+"' update-ref --no-deref -d 'refs/replace/"+r.b+"' "+r.d {
			t.Errorf("listed as %+v", p)
		}
		noKeep(t, r.gitOf(), p)
		if err := removeOldGitPlant(ctx, held, p.ID); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if got, status := r.plain(t, "rev-parse", "--verify", "-q", "refs/heads/owner-work"); status != 0 || got != r.d {
			t.Errorf("Remove of the replace ref touched refs/heads/owner-work: %q %d", got, status)
		}
		if got, _ := r.plain(t, "for-each-ref", "refs/replace/"); got != "" || !nothingAt(p.File) {
			t.Errorf("the replace ref is still there after Remove: %q", got)
		}
		if got, _ := r.plain(t, "show", "main:f"); got != "base" {
			t.Errorf("main:f after Remove: %q", got)
		}
		// The command shown does the same, run as plain does: with no system or global config.
		writeRepoFile(t, r.gitOf(), "refs/replace/"+r.b, "ref: refs/heads/owner-work\n")
		shown := exec.Command("/bin/sh", "-c", heldAt(t, r.gitOf())[0].Command)
		shown.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "GIT_") }),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if out, err := shown.CombinedOutput(); err != nil {
			t.Fatalf("the command shown: %v %s", err, out)
		}
		if got, status := r.plain(t, "rev-parse", "--verify", "-q", "refs/heads/owner-work"); status != 0 || got != r.d || !nothingAt(p.File) {
			t.Errorf("the command shown: owner-work %q %d; the replace ref gone: %v", got, status, nothingAt(p.File))
		}
	})
	t.Run("it follows a ref that isn't there yet", func(t *testing.T) {
		r := aSubstRepo(t)
		// Through a link to a file elsewhere, as git reads it.
		elsewhere := filepath.Join(physical(t.TempDir()), "ref")
		writeRepoFile(t, filepath.Dir(elsewhere), "ref", "ref: refs/heads/hire/later\n")
		at := filepath.Join(r.gitOf(), "refs", "replace", r.b)
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, at); err != nil {
			t.Fatal(err)
		}
		if got, _ := r.plain(t, "for-each-ref", "refs/replace/"); got != "" {
			t.Fatalf("git lists it: %q", got)
		}
		held := heldAt(t, r.gitOf())
		if len(held) != 1 {
			t.Fatalf("held: %+v", held)
		}
		p := held[0]
		if p.File != at || p.Follows != "refs/heads/hire/later" || p.Why != whyReplaceFollowsTo || p.Command != "rm '"+at+"'" {
			t.Errorf("listed as %+v", p)
		}
		noKeep(t, r.gitOf(), p)
		// Once the ref is made, git takes it, and it's listed as one that follows a branch.
		gitRepoCmd(t, r.dir, "branch", "hire/later", r.d)
		if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
			t.Fatalf("the owner's git reads main:f as %q once hire/later is made", got)
		}
		if again := heldAt(t, r.gitOf()); len(again) != 1 || again[0].Why != whyReplaceFollows || again[0].Follows != "refs/heads/hire/later" {
			t.Errorf("once hire/later is made: %+v", again)
		}
		gitRepoCmd(t, r.dir, "branch", "-D", "hire/later")
		if err := removeOldGitPlant(ctx, held, p.ID); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if data, err := os.ReadFile(elsewhere); !nothingAt(at) || err != nil || string(data) != "ref: refs/heads/hire/later\n" {
			t.Errorf("Remove: the link gone %v; the file it led to %q %v", nothingAt(at), data, err)
		}
	})
	t.Run("a plain one made symbolic after it was listed", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "replace", r.b, r.d)
		held := heldAt(t, r.gitOf())
		if len(held) != 1 || held[0].Follows != "" || held[0].Why != whyReplaceRef {
			t.Fatalf("held: %+v", held)
		}
		gitRepoCmd(t, r.dir, "branch", "owner-work", r.d) // at D, the object the plain one named
		writeRepoFile(t, r.gitOf(), "refs/replace/"+r.b, "ref: refs/heads/owner-work\n")
		if err := removeOldGitPlant(ctx, held, held[0].ID); !errors.Is(err, errGitPlantGone) {
			t.Errorf("Remove of one that changed: %v", err)
		}
		if got, _ := r.plain(t, "rev-parse", "--verify", "-q", "refs/heads/owner-work"); got != r.d || nothingAt(held[0].File) {
			t.Errorf("Remove of one that changed touched something: owner-work %q", got)
		}
	})
}

// The reviewer's probes (RT126GITPLANTSWARN, brief item 1), as rows: a check that can't run (here, out of time) lists
// each thing it would have checked as held, and a packed replace ref with grafts holds both.
func TestALookThatRunsOutOfTimeListsWhatItWouldHaveChecked(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	r := aSubstRepo(t)
	gitRepoCmd(t, r.dir, "repack", "-q", "-a", "-d")
	gitRepoCmd(t, r.dir, "multi-pack-index", "write")
	writeRepoFile(t, r.gitOf(), "shallow", r.b+"\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	held := map[string]bool{}
	for _, p := range findGitSubsts(ctx, r.gitOf()) {
		held[p.Why] = p.Held
	}
	for _, why := range []string{whyReplaceUnread, whyShallowUnchecked, whyMidxUnchecked} {
		if !held[why] {
			t.Errorf("not listed and held when its check can't run: %q (%v)", why, held)
		}
	}
}

func TestAPackedReplaceRefWithGraftsHoldsBoth(t *testing.T) {
	newTestDeck(t)
	freshGitKept(t)
	r := aSubstRepo(t)
	gitRepoCmd(t, r.dir, "replace", r.b, r.d)
	gitRepoCmd(t, r.dir, "pack-refs", "--all")
	if !nothingAt(filepath.Join(r.gitOf(), "refs", "replace", r.b)) {
		t.Fatal("the replace ref is still loose")
	}
	writeRepoFile(t, r.gitOf(), "info/grafts", r.b+" "+r.a+"\n")
	held := heldAt(t, r.gitOf())
	whys := []string{}
	for _, p := range held {
		whys = append(whys, p.Why)
	}
	slices.Sort(whys)
	if want := []string{whyGrafts, whyReplaceRef}; !slices.Equal(whys, slices.Sorted(slices.Values(want))) {
		t.Errorf("held: %q", whys)
	}
	if got := gitSubstRefusal(held); !strings.Contains(got, held[0].File+" and 1 more. "+held[0].Why) {
		t.Errorf("the start's words: %q", got)
	}
}

// Setup's page offers Keep only where the row says it can be kept, and shows what a replace ref follows.
func TestSetupsPageKeepsOnlyWhatCanBeKept(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	for _, want := range []string{"if (p.keepable) buttons.append(' ', gitKeepButton(p));", "p.follows ? `${p.file} → ${p.follows}`"} {
		if !strings.Contains(page, want) {
			t.Errorf("setup.html lacks %q", want)
		}
	}
	if strings.Contains(page, "if (p.held) buttons.append(' ', gitKeepButton(p))") {
		t.Error("setup.html offers Keep for every held row")
	}
}
