package desktop

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// FXFIREFSCK: before the fire's merged look, the core hashes again every object HEAD and the hire's branch reach
// (firefsck.go). Throwaway repos only (aSubstRepo: main is A then B with f = "base"; the desk's D, on hire/x, has
// f = "desk"). Each tampering case first shows the owner's plain git taking the desk's bytes, so it isn't empty.

// looseObject is where git keeps id as a loose object in repo.
func looseObject(repo, id string) string {
	return filepath.Join(repo, ".git", "objects", id[:2], id[2:])
}

// overwrite puts data in file, as a desk that may write objects/ could (git leaves its objects read-only).
func overwrite(t *testing.T, file string, data []byte) {
	t.Helper()
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// madeMergedByAForgedCommit rewrites commit's loose file in repo with a commit of the same tree and message whose
// parent is parent: no ref moves, but git now takes parent as merged into whatever reaches commit.
func madeMergedByAForgedCommit(t *testing.T, repo, commit, parent string) {
	t.Helper()
	tree := gitRepoCmd(t, repo, "rev-parse", commit+"^{tree}")
	text := fmt.Sprintf("tree %s\nparent %s\nauthor t <t@example.com> 1 +0000\ncommitter t <t@example.com> 1 +0000\n\nforged\n", tree, parent)
	cmd := exec.Command("git", "-C", repo, "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	forged, err := os.ReadFile(looseObject(repo, strings.TrimSpace(string(out))))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(looseObject(repo, commit)); err != nil {
		t.Fatalf("%s isn't a loose object here: %v", commit, err)
	}
	overwrite(t, looseObject(repo, commit), forged)
}

// swappedInThePackIndex repacks repo, then swaps a's and b's offsets in the pack's own .idx and seals it again.
func swappedInThePackIndex(t *testing.T, repo, a, b string) {
	t.Helper()
	gitRepoCmd(t, repo, "repack", "-adq")
	gitRepoCmd(t, repo, "prune")
	indexes, _ := filepath.Glob(filepath.Join(repo, ".git", "objects", "pack", "*.idx"))
	if len(indexes) != 1 {
		t.Fatalf("pack indexes %q", indexes)
	}
	rewriteSealed(t, indexes[0], func(data []byte) {
		n := int(binary.BigEndian.Uint32(data[8+255*4:]))
		names, at := 8+256*4, map[string]int{}
		for i := 0; i < n; i++ {
			at[string(data[names+20*i:names+20*(i+1)])] = i
		}
		ia, okA := at[string(hexID(t, a))]
		ib, okB := at[string(hexID(t, b))]
		if !okA || !okB {
			t.Fatalf("%s or %s isn't in the pack", a, b)
		}
		offsets := names + 24*n // the names, then a CRC each, then the offsets
		pa, pb := offsets+4*ia, offsets+4*ib
		var keep [4]byte
		copy(keep[:], data[pa:pa+4])
		copy(data[pa:pa+4], data[pb:pb+4])
		copy(data[pb:pb+4], keep[:])
	})
}

// changedFile is the cause for f's object id rewritten, as commit on branch has it.
func changedFile(id, commit, branch string) string {
	return "the object " + id + " holds other bytes than its name says: it is the file f in commit " + commit + ", on " + branch
}

func TestTheFiresLookFindsASoundStoreSound(t *testing.T) {
	r := aSubstRepo(t)
	listed, _ := r.plain(t, "rev-list", "--objects", "main", "hire/x")
	objects, cause, err := storeHoldsWhatItNames(context.Background(), r.dir, []string{"HEAD", "refs/heads/hire/x"})
	if err != nil || cause != "" || objects != len(strings.Split(listed, "\n")) {
		t.Fatalf("objects %d (want %d), cause %q, err %v", objects, len(strings.Split(listed, "\n")), cause, err)
	}
	if said := fireStoreKept(Hire{Name: "Rex", Repo: r.dir, Worktree: filepath.Join(r.dir, "elsewhere"), Branch: "hire/x"}, nil); said != "" {
		t.Errorf("a sound store kept the worktree: %q", said)
	}
	if said := fireStoreKept(Hire{Name: "Rex", Repo: r.dir, Branch: "hire/x"}, nil); said != "" {
		t.Errorf("a desk with no worktree was looked at: %q", said)
	}
}

func TestTheFiresLookCatchesAnObjectRewrittenInPlace(t *testing.T) {
	cases := []struct {
		name   string
		tamper func(t *testing.T, r substRepo, base, desk string)
		taken  func(t *testing.T, r substRepo) bool          // the owner's plain git takes the desk's version
		named  func(r substRepo, base, desk string) []string // the causes it may give, in full
	}{
		{"a loose object rewritten",
			func(t *testing.T, r substRepo, base, desk string) {
				data, err := os.ReadFile(looseObject(r.dir, desk))
				if err != nil {
					t.Fatal(err)
				}
				overwrite(t, looseObject(r.dir, base), data)
			},
			func(t *testing.T, r substRepo) bool { out, _ := r.plain(t, "show", "main:f"); return out == "desk" },
			func(r substRepo, base, desk string) []string { return []string{changedFile(base, r.b, "main")} }},
		{"an object only the desk's branch reaches, rewritten",
			func(t *testing.T, r substRepo, base, desk string) {
				data, err := os.ReadFile(looseObject(r.dir, base))
				if err != nil {
					t.Fatal(err)
				}
				overwrite(t, looseObject(r.dir, desk), data)
			},
			func(t *testing.T, r substRepo) bool { out, _ := r.plain(t, "show", "hire/x:f"); return out == "base" },
			func(r substRepo, base, desk string) []string { return []string{changedFile(desk, r.d, "hire/x")} }},
		{"two offsets swapped in a pack's .idx",
			func(t *testing.T, r substRepo, base, desk string) { swappedInThePackIndex(t, r.dir, base, desk) },
			func(t *testing.T, r substRepo) bool { out, _ := r.plain(t, "show", "main:f"); return out == "desk" },
			func(r substRepo, base, desk string) []string {
				return []string{changedFile(base, r.b, "main"), changedFile(desk, r.d, "hire/x")}
			}},
		{"a commit rewritten so the desk's branch looks merged",
			func(t *testing.T, r substRepo, base, desk string) { madeMergedByAForgedCommit(t, r.dir, r.b, r.d) },
			func(t *testing.T, r substRepo) bool {
				_, plain := r.plain(t, "merge-base", "--is-ancestor", "refs/heads/hire/x", "HEAD")
				_, core := r.core(t, "merge-base", "--is-ancestor", "refs/heads/hire/x", "HEAD") // gitReadEnv alone doesn't help
				return plain == 0 && core == 0
			},
			func(r substRepo, base, desk string) []string {
				return []string{"the object " + r.b + " holds other bytes than its name says: it is the commit " + r.b + ", on main"}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := aSubstRepo(t)
			base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
			c.tamper(t, r, base, desk)
			if !c.taken(t, r) {
				t.Fatal("the owner's git doesn't take the desk's version, so this case proves nothing")
			}
			_, cause, err := storeHoldsWhatItNames(context.Background(), r.dir, []string{"HEAD", "refs/heads/hire/x"})
			if err != nil {
				t.Fatalf("the look couldn't be made: %v", err)
			}
			if !slices.Contains(c.named(r, base, desk), cause) {
				t.Fatalf("cause %q, want one of %q", cause, c.named(r, base, desk))
			}
			h := Hire{Name: "Janitor", Nickname: "Rex", Repo: r.dir, Worktree: filepath.Join(r.dir, "w"), Branch: "hire/x"}
			said := fireStoreKept(h, nil)
			for _, want := range []string{"Smolcorp kept Rex's worktree " + h.Worktree + " and its branch hire/x",
				"the repo's stored objects were changed under it (" + cause + ")", "didn't look at whether the branch is merged",
				"run git fsck --full in " + r.dir} {
				if !strings.Contains(said, want) {
					t.Errorf("the owner is told %q, without %q", said, want)
				}
			}
		})
	}
}

// The look runs on the core's read env (gitReadEnv): grafts that cut main's history at B don't hide an object only
// A reaches, rewritten in place.
func TestTheFiresLookIsntCutShortByGrafts(t *testing.T) {
	r := aSubstRepo(t)
	old, desk := gitRepoCmd(t, r.dir, "rev-parse", r.a+":f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
	data, err := os.ReadFile(looseObject(r.dir, desk))
	if err != nil {
		t.Fatal(err)
	}
	overwrite(t, looseObject(r.dir, old), data)
	writeRepoFile(t, r.dir, filepath.Join(".git", "info", "grafts"), r.b+"\n")
	if out, _ := r.plain(t, "rev-list", "--count", "main"); out != "1" {
		t.Fatalf("the owner's git counts %q commits in main, want the grafts' 1, so this case proves nothing", out)
	}
	if _, cause, err := storeHoldsWhatItNames(context.Background(), r.dir, []string{"HEAD"}); err != nil ||
		cause != changedFile(old, r.a, "main") {
		t.Fatalf("cause %q, err %v", cause, err)
	}
}

// A look that can't be made keeps the worktree and the branch too, and says it couldn't check.
func TestTheFiresLookThatCantBeMadeKeepsThemAndSaysSo(t *testing.T) {
	r := aSubstRepo(t)
	if _, _, err := storeHoldsWhatItNames(context.Background(), r.dir, []string{"HEAD", "refs/heads/hire/gone"}); err == nil ||
		err.Error() != "git couldn't find refs/heads/hire/gone in it" {
		t.Fatalf("err %v", err)
	}
	said := fireStoreKept(Hire{Name: "Rex", Repo: r.dir, Worktree: filepath.Join(r.dir, "w"), Branch: "hire/gone"}, nil)
	if !strings.HasPrefix(said, "Smolcorp kept Rex's worktree ") ||
		!strings.Contains(said, ": it couldn't check the repo's stored objects (git couldn't find refs/heads/hire/gone in it)") {
		t.Errorf("the owner is told %q", said)
	}
	// A row with a worktree but no repo or branch is never taken as sound.
	if said := fireStoreKept(Hire{Name: "Rex", Worktree: filepath.Join(r.dir, "w")}, nil); !strings.Contains(said,
		": it couldn't check the repo's stored objects (its row names no repo or no branch)") {
		t.Errorf("no repo: the owner is told %q", said)
	}
	// An object git can't read: one of the history's own is gone.
	gone := aSubstRepo(t)
	if err := os.Remove(looseObject(gone.dir, gone.a)); err != nil {
		t.Fatal(err)
	}
	// N1: that isn't taken for changed objects; it is a look that couldn't be made, and it fails closed the same.
	if _, cause, err := storeHoldsWhatItNames(context.Background(), gone.dir, []string{"HEAD"}); err == nil || cause != "" ||
		!strings.HasPrefix(err.Error(), "git couldn't read every object they reach: ") {
		t.Errorf("a missing object: cause %q, err %v", cause, err)
	}
}

// hashBatch reads cat-file's answer: SHA-1 and SHA-256 names, a missing object, an answer that breaks off or says
// more than git listed, or that names another object than rev-list listed (N4); each fault with where it was found.
func TestTheFiresLookReadsGitsAnswerAsGitWritesIt(t *testing.T) {
	blob := func(name, body string) string { return name + " blob " + strconv.Itoa(len(body)) + "\n" + body + "\n" }
	const one = "d670460b4b4aece5915caf5c68d12f560a9fe3e4"                         // "test content\n", SHA-1
	const two = "ab59ce5b2ec1a5e8a6aa83e9b2d2a8e2b3c55a8b8c7e2c5a0f5d68a3c8e8e1a1" // not its SHA-256
	const commit = "c0ffee0000000000000000000000000000000001"
	for _, c := range []struct {
		name, answer string
		objects      int
		what, words  string
		listed       string // the name rev-list gave; the answer's own when ""
	}{
		{"sound", blob(one, "test content\n"), 1, "", "", ""},
		{"other bytes", blob(one, "test kontent\n"), 0, "holds other bytes than its name says",
			"the object " + one + " holds other bytes than its name says: it is the file a/f in commit " + commit + ", on main", ""},
		{"a SHA-256 name", blob(two, "test content\n"), 0, "holds other bytes than its name says", "", ""},
		{"missing", one + " missing\n", 0, "is missing, or git can't read it",
			"the object " + one + " is missing, or git can't read it: it is the file or folder a/f in commit " + commit + ", on main", ""},
		{"broken off", one + " blob 13\ntest", 0, "broke off git's answer", "", ""},
		{"not git's words", "what\n", 0, `came with an answer git doesn't give: "what"`, "", ""},
		{"more than git listed", blob(one, "test content\n") + blob(one, "test content\n"), 1, "was answered for, though git listed no more", "", ""},
		{"another name than listed", blob(one, "test content\n"), 0, "was answered for, though git listed " + commit, "", commit},
	} {
		listed := c.listed
		if listed == "" {
			listed = strings.Fields(c.answer)[0]
		}
		found := make(chan storePlace, 1) // git listed one name, a file at a/f
		found <- storePlace{name: listed, path: "a/f", named: true}
		close(found)
		objects, fault := hashBatch(bufio.NewReader(strings.NewReader(c.answer)), found, nil)
		if objects != c.objects || fault.what != c.what {
			t.Errorf("%s: %d %q, want %d %q", c.name, objects, fault.what, c.objects, c.what)
		}
		if c.words != "" {
			fault.commit = commit // the commit the walk was at
			if got := fault.words([]string{"main"}); got != c.words {
				t.Errorf("%s: said %q, want %q", c.name, got, c.words)
			}
		}
	}
}
