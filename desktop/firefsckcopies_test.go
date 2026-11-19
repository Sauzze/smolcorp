package desktop

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// FXFIREFSCKR1: the look hashes every stored copy of each name it reached (RT126FIREFSCK's B1), walks every local
// base branch next to HEAD (B2), and the reviewer's other probes, as rows. Throwaway repos only (aSubstRepo: main is A
// then B with f = "base"; the desk's D, on hire/x, has f = "desk"). Each tampering case first shows what the owner's
// plain git does with it, so no case is empty.

// gitFed runs git in dir with stdin, as gitRepoCmd does.
func gitFed(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeLoose stores body as a loose object of kind in the object folder objects, as git would (and a desk could).
func writeLoose(t *testing.T, objects, id, kind, body string) {
	t.Helper()
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	fmt.Fprintf(w, "%s %d\x00%s", kind, len(body), body)
	w.Close()
	file := filepath.Join(objects, id[:2], id[2:])
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, z.Bytes(), 0o444); err != nil {
		t.Fatal(err)
	}
}

// packedEntry is an object's entry in a pack, as git's verify-pack says it: where it starts, how long it is, and
// whether it is a delta.
type packedEntry struct {
	at, length int
	delta      bool
}

// entriesOf are the entries of idx's pack, by name.
func entriesOf(t *testing.T, repo, idx string) map[string]packedEntry {
	t.Helper()
	entries := map[string]packedEntry{}
	for _, line := range strings.Split(gitRepoCmd(t, repo, "verify-pack", "-v", idx), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !gitObjectID(f[0]) {
			continue
		}
		length, _ := strconv.Atoi(f[3])
		at, _ := strconv.Atoi(f[4])
		entries[f[0]] = packedEntry{at: at, length: length, delta: len(f) == 7}
	}
	return entries
}

// onePack repacks repo uncompressed (same-size bodies make same-size entries) and is its one pack's .idx.
func onePack(t *testing.T, repo string) string {
	t.Helper()
	gitRepoCmd(t, repo, "config", "pack.compression", "0")
	gitRepoCmd(t, repo, "repack", "-adq")
	indexes, _ := filepath.Glob(filepath.Join(repo, ".git", "objects", "pack", "*.idx"))
	if len(indexes) != 1 {
		t.Fatalf("pack indexes %q", indexes)
	}
	return indexes[0]
}

// packedOver puts from's entry over to's in idx's pack: two whole entries of the same size, so nothing else moves, and
// git's next read of to gives from's bytes. crc: to's CRC in the .idx is put right too, and the .idx sealed again, so
// the owner's repack reuses the entry as it is (RT126FIREFSCK's "CRC-fixed form").
func packedOver(t *testing.T, repo, idx, to, from string, crc bool) {
	t.Helper()
	entries := entriesOf(t, repo, idx)
	a, b := entries[to], entries[from]
	if a.length == 0 || a.length != b.length || a.delta || b.delta {
		t.Fatalf("entries %+v and %+v aren't two whole ones of the same size", a, b)
	}
	pack := strings.TrimSuffix(idx, ".idx") + ".pack"
	data, err := os.ReadFile(pack)
	if err != nil {
		t.Fatal(err)
	}
	copy(data[a.at:a.at+a.length], data[b.at:b.at+b.length])
	overwrite(t, pack, data)
	if crc {
		sum := crc32.ChecksumIEEE(data[a.at : a.at+a.length])
		rewriteSealed(t, idx, func(x []byte) {
			n := int(binary.BigEndian.Uint32(x[8+255*4:]))
			names := 8 + 256*4
			for i := 0; i < n; i++ {
				if bytes.Equal(x[names+20*i:names+20*(i+1)], hexID(t, to)) {
					binary.BigEndian.PutUint32(x[names+20*n+4*i:], sum)
					return
				}
			}
			t.Fatalf("%s isn't in %s", to, idx)
		})
	}
}

// aStoredCopy is the words for the copy of f's object id in file that doesn't hold its name, as commit on main has it.
func aStoredCopy(id, file, commit string) string {
	return "the object " + id + " has a stored copy, in " + file + ", that holds other bytes than its name says: it is the file f in commit " + commit + ", on main"
}

// packOf is a pack's file inside repo, as the owner is told it.
func packOf(repo, idx string) string {
	rel, _ := filepath.Rel(repo, strings.TrimSuffix(idx, ".idx")+".pack")
	return filepath.ToSlash(rel)
}

// B1: a changed copy behind the one git reads now is caught, whichever store or pack it is in; harmless second copies
// pass.
func TestTheFiresLookHashesEveryStoredCopyOfEachName(t *testing.T) {
	look := func(t *testing.T, repo string) (string, error) {
		t.Helper()
		_, cause, err := storeHoldsWhatItNames(context.Background(), repo, []string{"HEAD"})
		return cause, err
	}
	// A good loose copy in front of a packed one that holds the desk's bytes; after the look, the owner's ordinary
	// maintenance (then) drops the loose copy, and main reads the desk's bytes: what the look keeps them from.
	for _, c := range []struct {
		name string
		crc  bool
		then []string
	}{
		{"a good loose copy before a changed packed one, its CRC put right", true, []string{"gc", "-q"}},
		{"a good loose copy before a changed packed one, its CRC left", false, []string{"prune-packed"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := aSubstRepo(t)
			base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
			idx := onePack(t, r.dir)
			packedOver(t, r.dir, idx, base, desk, c.crc)
			writeLoose(t, filepath.Join(r.dir, ".git", "objects"), base, "blob", "base\n")
			if out, _ := r.plain(t, "show", "main:f"); out != "base" {
				t.Fatalf("the owner's git shows %q for main:f, want the good loose copy's", out)
			}
			cause, err := look(t, r.dir)
			if err != nil || cause != aStoredCopy(base, packOf(r.dir, idx), r.b) {
				t.Fatalf("cause %q, err %v; want %q", cause, err, aStoredCopy(base, packOf(r.dir, idx), r.b))
			}
			h := Hire{Name: "Rex", Repo: r.dir, Worktree: keptPath(filepath.Join(r.dir, "w")), Branch: "hire/x"}
			said := fireStoreKept(h, nil)
			if !strings.Contains(said, "the repo's stored objects were changed under it ("+cause+")") ||
				strings.Contains(said, " gc") || strings.Contains(said, "repack") || !strings.Contains(said, "git fsck --full in "+r.dir) {
				t.Errorf("the owner is told %q", said)
			}
			r.plain(t, c.then...)
			if out, _ := r.plain(t, "show", "main:f"); out != "desk" {
				t.Fatalf("after the owner's git %v, main:f is %q; this case proves nothing", c.then, out)
			}
		})
	}
	t.Run("a name held in two packs, the older one changed", func(t *testing.T) {
		r := aSubstRepo(t)
		base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
		idx := onePack(t, r.dir)
		// A second pack, good, of all main reaches: git, which reads the newest pack first and then the last one it
		// found something in, takes main's file from it.
		gitFed(t, r.dir, "main\n", "pack-objects", "-q", "--revs", filepath.Join(".git", "objects", "pack", "pack"))
		packedOver(t, r.dir, idx, base, desk, true)
		hour := time.Now().Add(-time.Hour) // git reads the newest pack first: the changed one is made the older
		for _, file := range []string{idx, strings.TrimSuffix(idx, ".idx") + ".pack"} {
			if err := os.Chtimes(file, hour, hour); err != nil {
				t.Fatal(err)
			}
		}
		if out, _ := r.plain(t, "show", "main:f"); out != "base" {
			t.Fatalf("the owner's git shows %q for main:f, want the newer pack's", out)
		}
		if cause, err := look(t, r.dir); err != nil || cause != aStoredCopy(base, packOf(r.dir, idx), r.b) {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("in another store the repo reads", func(t *testing.T) {
		r := aSubstRepo(t)
		base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
		idx := onePack(t, r.dir)
		other := filepath.Join(physical(t.TempDir()), "other")
		gitRepoCmd(t, r.dir, "clone", "-q", "--shared", r.dir, other)
		packedOver(t, r.dir, idx, base, desk, true)
		writeLoose(t, filepath.Join(r.dir, ".git", "objects"), base, "blob", "base\n")
		if out, _ := (substRepo{dir: other}).plain(t, "show", "main:f"); out != "base" {
			t.Fatalf("the owner's git shows %q for main:f, want the good loose copy's", out)
		}
		pack := strings.TrimSuffix(idx, ".idx") + ".pack"
		if cause, err := look(t, other); err != nil || cause != aStoredCopy(base, pack, gitRepoCmd(t, other, "rev-parse", "main")) {
			t.Fatalf("cause %q, err %v; want %q", cause, err, aStoredCopy(base, pack, r.b))
		}
	})
	// Harmless second copies (a repack in progress, a fetch of what was here) pass: each is read and hashed, deltas
	// by offset (repack's) and by name (pack-objects' own) put together as git does.
	t.Run("good second copies pass", func(t *testing.T) {
		r := aSubstRepo(t)
		var head, tail strings.Builder // a line put in the middle: each delta copies from inside its base too
		for i := 0; i < 400; i++ {
			fmt.Fprintf(map[bool]*strings.Builder{true: &head, false: &tail}[i < 200], "line %d of a file big enough to be packed as a delta\n", i)
		}
		small, bigger := head.String()+tail.String(), head.String()+"a line put in the middle\n"+tail.String()
		writeRepoFile(t, r.dir, "big", small)
		gitRepoCmd(t, r.dir, "add", "big")
		gitRepoCmd(t, r.dir, "commit", "-q", "-m", "big")
		writeRepoFile(t, r.dir, "big", bigger)
		gitRepoCmd(t, r.dir, "commit", "-q", "-am", "bigger")
		old, now := gitRepoCmd(t, r.dir, "rev-parse", "main~1:big"), gitRepoCmd(t, r.dir, "rev-parse", "main:big")
		idx := onePack(t, r.dir)
		gitFed(t, r.dir, old+"\n"+now+"\n", "pack-objects", "-q", filepath.Join(".git", "objects", "pack", "pack"))
		writeLoose(t, filepath.Join(r.dir, ".git", "objects"), old, "blob", small)
		writeLoose(t, filepath.Join(r.dir, ".git", "objects"), now, "blob", bigger)
		deltas := 0
		indexes, _ := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "pack", "*.idx"))
		for _, x := range indexes {
			for _, name := range []string{old, now} {
				if entriesOf(t, r.dir, x)[name].delta {
					deltas++
				}
			}
		}
		if len(indexes) != 2 || deltas != 2 || idx == "" {
			t.Fatalf("packs %q with %d deltas of big, want 2 and 2", indexes, deltas)
		}
		if cause, err := look(t, r.dir); err != nil || cause != "" {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
}

// aBranchOfItsOwn makes branch from main with f = text, and rewrites that file's object with the desk's bytes: the
// commit and the object. The owner's checkout stays where it was.
func aBranchOfItsOwn(t *testing.T, r substRepo, branch, text string) (string, string) {
	t.Helper()
	was := gitRepoCmd(t, r.dir, "symbolic-ref", "--short", "HEAD")
	gitRepoCmd(t, r.dir, "checkout", "-q", "-b", branch, "main")
	writeRepoFile(t, r.dir, "f", text)
	gitRepoCmd(t, r.dir, "commit", "-q", "-am", branch)
	commit, blob := gitRepoCmd(t, r.dir, "rev-parse", "HEAD"), gitRepoCmd(t, r.dir, "rev-parse", "HEAD:f")
	gitRepoCmd(t, r.dir, "checkout", "-q", was)
	data, err := os.ReadFile(looseObject(r.dir, gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")))
	if err != nil {
		t.Fatal(err)
	}
	overwrite(t, looseObject(r.dir, blob), data)
	if out, _ := r.plain(t, "show", branch+":f"); out != "desk" {
		t.Fatalf("the owner's git shows %q for %s:f; this case proves nothing", out, branch)
	}
	return commit, blob
}

// B2: every local base branch is walked, wherever the owner's checkout is, and the owner is told the real names.
func TestTheFiresLookWalksEveryBaseBranch(t *testing.T) {
	for _, c := range []struct {
		name, branch string
		setup        func(t *testing.T, r substRepo)
		shown        string // what the words say git shows
	}{
		{"main, with the owner on another branch", "main",
			func(t *testing.T, r substRepo) { gitRepoCmd(t, r.dir, "checkout", "-q", "-b", "feat", r.a) },
			"feat, main and the branch"},
		{"a release branch", "release/1.0", nil, "main, release/1.0 and the branch"},
		{"master", "master", nil, "main, master and the branch"},
		{"the default branch origin/HEAD names", "trunk",
			func(t *testing.T, r substRepo) {
				gitRepoCmd(t, r.dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
			},
			"main, trunk and the branch"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := aSubstRepo(t)
			var commit, blob string
			if c.branch == "main" {
				blob = gitRepoCmd(t, r.dir, "rev-parse", "main:f")
				commit = r.b
				data, err := os.ReadFile(looseObject(r.dir, gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")))
				if err != nil {
					t.Fatal(err)
				}
				overwrite(t, looseObject(r.dir, blob), data)
			} else {
				commit, blob = aBranchOfItsOwn(t, r, c.branch, "own\n")
			}
			if c.setup != nil {
				c.setup(t, r)
			}
			if out, _ := r.plain(t, "show", c.branch+":f"); out != "desk" {
				t.Fatalf("the owner's git shows %q for %s:f; this case proves nothing", out, c.branch)
			}
			h := Hire{Name: "Rex", Repo: r.dir, Worktree: keptPath(filepath.Join(r.dir, "w")), Branch: "hire/x"}
			cause := "the object " + blob + " holds other bytes than its name says: it is the file f in commit " + commit + ", on " + c.branch
			want := "the repo's stored objects were changed under it (" + cause + "), so what git shows for " + c.shown + " can't be trusted"
			if said := fireStoreKept(h, nil); !strings.Contains(said, want) {
				t.Fatalf("the owner is told %q, want %q", said, want)
			}
		})
	}
}

// The reviewer's other probes (RT126FIREFSCK's tables), each a row.
func TestTheFiresLookCatchesTheReviewersOtherForms(t *testing.T) {
	look := func(t *testing.T, repo string) (string, error) {
		t.Helper()
		_, cause, err := storeHoldsWhatItNames(context.Background(), repo, []string{"HEAD"})
		return cause, err
	}
	deskBytes := func(t *testing.T, r substRepo) []byte {
		data, err := os.ReadFile(looseObject(r.dir, gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	t.Run("a packed object changed, as its only copy", func(t *testing.T) {
		r := aSubstRepo(t)
		base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
		packedOver(t, r.dir, onePack(t, r.dir), base, desk, false)
		if out, _ := r.plain(t, "show", "main:f"); out != "desk" {
			t.Fatalf("main:f is %q; this case proves nothing", out)
		}
		if cause, err := look(t, r.dir); err != nil || cause != changedFile(base, r.b, "main") {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("a replace ref over a changed object", func(t *testing.T) {
		r := aSubstRepo(t)
		base := gitRepoCmd(t, r.dir, "rev-parse", "main:f")
		overwrite(t, looseObject(r.dir, base), deskBytes(t, r))
		gitRepoCmd(t, r.dir, "replace", base, gitFed(t, r.dir, "Base\n", "hash-object", "-w", "--stdin"))
		if out, _ := r.plain(t, "show", "main:f"); out != "Base" {
			t.Fatalf("main:f is %q; the replace ref isn't read", out)
		}
		if cause, err := look(t, r.dir); err != nil || cause != changedFile(base, r.b, "main") {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("an object only another store holds, changed there", func(t *testing.T) {
		r := aSubstRepo(t)
		other := filepath.Join(physical(t.TempDir()), "other")
		gitRepoCmd(t, r.dir, "clone", "-q", "--shared", r.dir, other)
		base := gitRepoCmd(t, r.dir, "rev-parse", "main:f")
		overwrite(t, looseObject(r.dir, base), deskBytes(t, r))
		if out, _ := (substRepo{dir: other}).plain(t, "show", "main:f"); out != "desk" {
			t.Fatalf("main:f is %q; this case proves nothing", out)
		}
		if cause, err := look(t, other); err != nil || cause != changedFile(base, r.b, "main") {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("a SHA-256 repo", func(t *testing.T) {
		dir := physical(t.TempDir())
		gitRepoCmd(t, dir, "init", "-q", "--object-format=sha256", "-b", "main")
		writeRepoFile(t, dir, "f", "one\n")
		gitRepoCmd(t, dir, "add", "f")
		gitRepoCmd(t, dir, "commit", "-q", "-m", "A")
		one := gitRepoCmd(t, dir, "rev-parse", "main:f")
		writeRepoFile(t, dir, "f", "two\n")
		gitRepoCmd(t, dir, "commit", "-q", "-am", "B")
		two, commit := gitRepoCmd(t, dir, "rev-parse", "main:f"), gitRepoCmd(t, dir, "rev-parse", "main")
		data, err := os.ReadFile(looseObject(dir, one))
		if err != nil {
			t.Fatal(err)
		}
		overwrite(t, looseObject(dir, two), data)
		if out, _ := (substRepo{dir: dir}).plain(t, "show", "main:f"); out != "one" || len(two) != 64 {
			t.Fatalf("main:f is %q, its name %q; this case proves nothing", out, two)
		}
		if cause, err := look(t, dir); err != nil || cause != changedFile(two, commit, "main") {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("a tip's tag changed", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "tag", "-a", "v1", "-m", "first", "main")
		gitRepoCmd(t, r.dir, "tag", "-a", "v2", "-m", "other", "main")
		v1, v2 := gitRepoCmd(t, r.dir, "rev-parse", "v1"), gitRepoCmd(t, r.dir, "rev-parse", "v2")
		writeRepoFile(t, r.dir, filepath.Join(".git", "refs", "heads", "release", "1"), v1+"\n") // a base branch at a tag
		data, err := os.ReadFile(looseObject(r.dir, v2))
		if err != nil {
			t.Fatal(err)
		}
		overwrite(t, looseObject(r.dir, v1), data)
		if out, _ := r.plain(t, "cat-file", "-p", "release/1"); !strings.Contains(out, "other") {
			t.Fatalf("release/1 reads %q; this case proves nothing", out)
		}
		if cause, err := look(t, r.dir); err != nil || cause != "the object "+v1+" holds other bytes than its name says: it is the tag v2" {
			t.Fatalf("cause %q, err %v", cause, err)
		}
	})
	t.Run("a loose file that isn't one", func(t *testing.T) {
		r := aSubstRepo(t)
		base := gitRepoCmd(t, r.dir, "rev-parse", "main:f")
		overwrite(t, looseObject(r.dir, base), []byte("not an object"))
		if _, status := r.plain(t, "show", "main:f"); status == 0 {
			t.Fatal("the owner's git read it; this case proves nothing")
		}
		want := "the object " + base + " is missing, or git can't read it: it is the file or folder f in commit " + r.b + ", on main"
		if cause, err := look(t, r.dir); err != nil || cause != want {
			t.Fatalf("cause %q, err %v; want %q", cause, err, want)
		}
	})
	t.Run("a partial clone fails closed and fetches nothing", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "config", "uploadpack.allowFilter", "true")
		clone := filepath.Join(physical(t.TempDir()), "partial")
		gitRepoCmd(t, r.dir, "clone", "-q", "--no-local", "--filter=blob:none", r.dir, clone)
		before := gitRepoCmd(t, clone, "count-objects", "-v")
		cause, err := look(t, clone)
		if cause != "" || err == nil || !strings.HasPrefix(err.Error(), "git couldn't read every object they reach: ") {
			t.Fatalf("cause %q, err %v", cause, err)
		}
		if after := gitRepoCmd(t, clone, "count-objects", "-v"); after != before {
			t.Fatalf("the look fetched: %q, then %q", before, after)
		}
		said := fireStoreKept(Hire{Name: "Rex", Repo: clone, Worktree: keptPath(filepath.Join(clone, "w")), Branch: "main"}, nil)
		if !strings.Contains(said, ": it couldn't check the repo's stored objects (git couldn't read every object they reach: ") {
			t.Errorf("the owner is told %q", said)
		}
	})
}

// B1′ (RT126FIREFSCKR2): a link anywhere the copy listing walks fails the look closed, with words that name it. Git
// never makes one there, yet reads through each. Each case first shows the owner's git reading through the link.
func TestTheFiresLookFailsClosedOnALinkInTheObjectStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need a privilege on Windows")
	}
	// linked moves path to a folder of the desk's own and leaves a link to it in its place: where it went.
	linked := func(t *testing.T, path string) string {
		t.Helper()
		to := filepath.Join(physical(t.TempDir()), filepath.Base(path))
		if err := os.Rename(path, to); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(to, path); err != nil {
			t.Fatal(err)
		}
		return to
	}
	// failsClosed: the look of repo isn't made, for want's reason, and the owner is told it, never to run gc.
	failsClosed := func(t *testing.T, repo, want string) {
		t.Helper()
		_, cause, err := storeHoldsWhatItNames(context.Background(), repo, []string{"HEAD"})
		if cause != "" || err == nil || err.Error() != want {
			t.Fatalf("cause %q, err %v; want the error %q", cause, err, want)
		}
		said := fireStoreKept(Hire{Name: "Rex", Repo: repo, Worktree: keptPath(filepath.Join(repo, "w")), Branch: "main"}, nil)
		words := strings.ReplaceAll(said, filepath.Dir(physical(t.TempDir())), "<temp>")
		if !strings.Contains(said, ": it couldn't check the repo's stored objects ("+want+"), so it didn't look at whether the branch is merged") ||
			strings.Contains(words, " gc") || strings.Contains(words, "repack") || strings.Contains(words, "prune") {
			t.Errorf("the owner is told %q", said)
		}
	}
	shows := func(t *testing.T, repo, want string) {
		t.Helper()
		if out, _ := (substRepo{dir: repo}).plain(t, "show", "main:f"); out != want {
			t.Fatalf("the owner's git shows %q for main:f, want %q", out, want)
		}
	}
	// The reviewer's case: B1's, with the good loose copy behind a linked fan-out folder. The owner's maintenance
	// afterwards would have served the desk's bytes on main.
	for _, then := range [][]string{{"prune-packed"}, {"gc", "-q"}} {
		t.Run("a good loose copy behind a linked fan-out folder, then git "+then[0], func(t *testing.T) {
			r := aSubstRepo(t)
			base, desk := gitRepoCmd(t, r.dir, "rev-parse", "main:f"), gitRepoCmd(t, r.dir, "rev-parse", "hire/x:f")
			packedOver(t, r.dir, onePack(t, r.dir), base, desk, true)
			fanout := filepath.Join(r.dir, ".git", "objects", base[:2])
			if err := os.MkdirAll(fanout, 0o755); err != nil {
				t.Fatal(err)
			}
			to := linked(t, fanout)
			writeLoose(t, filepath.Dir(to), base, "blob", "base\n")
			shows(t, r.dir, "base")
			failsClosed(t, r.dir, "the object folder "+fanout+" is a link to "+to+", which git never makes")
			r.plain(t, then...)
			shows(t, r.dir, "desk")
		})
	}
	t.Run("the pack folder", func(t *testing.T) {
		r := aSubstRepo(t)
		onePack(t, r.dir)
		pack := filepath.Join(r.dir, ".git", "objects", "pack")
		to := linked(t, pack)
		shows(t, r.dir, "base")
		failsClosed(t, r.dir, "the pack folder "+pack+" is a link to "+to+", which git never makes")
	})
	for _, c := range []struct{ name, what, ext string }{{"a pack's index", "the pack index", ".idx"}, {"a pack", "the pack", ".pack"}} {
		t.Run(c.name, func(t *testing.T) {
			r := aSubstRepo(t)
			file := strings.TrimSuffix(onePack(t, r.dir), ".idx") + c.ext
			to := linked(t, file)
			shows(t, r.dir, "base")
			failsClosed(t, r.dir, c.what+" "+file+" is a link to "+to+", which git never makes")
		})
	}
	t.Run("a loose object", func(t *testing.T) {
		r := aSubstRepo(t)
		file := looseObject(r.dir, gitRepoCmd(t, r.dir, "rev-parse", "main:f"))
		to := linked(t, file)
		shows(t, r.dir, "base")
		failsClosed(t, r.dir, "the loose object "+file+" is a link to "+to+", which git never makes")
	})
	t.Run("another store named through a link, and one store further down", func(t *testing.T) {
		r := aSubstRepo(t)
		temp := physical(t.TempDir())
		other, below := filepath.Join(temp, "other"), filepath.Join(temp, "below")
		gitRepoCmd(t, r.dir, "clone", "-q", "--shared", r.dir, other)
		store, objects := filepath.Join(temp, "store"), filepath.Join(r.dir, ".git", "objects")
		if err := os.Symlink(objects, store); err != nil {
			t.Fatal(err)
		}
		alternates := filepath.Join(other, ".git", "objects", "info", "alternates")
		writeRepoFile(t, filepath.Dir(alternates), "alternates", store+"\n")
		gitRepoCmd(t, other, "clone", "-q", "--shared", other, below)
		if said := gitRepoCmd(t, below, "count-objects", "-v"); !strings.Contains(said+"\n", "alternate: "+objects+"\n") {
			t.Fatalf("git names the stores %q; this case proves nothing", said)
		}
		want := "the other store " + store + ", named in " + alternates + ", is a link to " + objects + ", which git never makes"
		for _, repo := range []string{other, below} {
			shows(t, repo, "base")
			failsClosed(t, repo, want)
		}
	})
}

// anIndex is a version 2 .idx naming names (each 20 bytes, in the order given) at made-up offsets.
func anIndex(names ...[]byte) []byte {
	var x bytes.Buffer
	x.Write([]byte{0xff, 't', 'O', 'c', 0, 0, 0, 2})
	for i := 0; i < 256; i++ {
		n := 0
		for _, name := range names {
			if int(name[0]) <= i {
				n++
			}
		}
		binary.Write(&x, binary.BigEndian, uint32(n))
	}
	for _, name := range names {
		x.Write(name)
	}
	x.Write(make([]byte, 4*len(names))) // their CRCs
	for i := range names {
		binary.Write(&x, binary.BigEndian, uint32(12+20*i))
	}
	x.Write(make([]byte, 40)) // the pack's checksum and its own
	return x.Bytes()
}

// F1 (RT126FIREFSCKR2): an .idx that names an object twice isn't read: git could take either entry, and the look
// another.
func TestTheFiresLookRefusesAPackIndexThatNamesAnObjectTwice(t *testing.T) {
	a := bytes.Repeat([]byte{0xab}, 20)
	b := append(bytes.Repeat([]byte{0xab}, 19), 0xac)
	dir := t.TempDir()
	for _, c := range []struct {
		name  string
		names [][]byte
		err   string
	}{
		{"two names", [][]byte{a, b}, ""},
		{"one name twice", [][]byte{a, a}, "its names aren't in git's order, or one is there twice"},
		{"two names out of order", [][]byte{b, a}, "its names aren't in git's order, or one is there twice"},
	} {
		file := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".idx")
		if err := os.WriteFile(file, anIndex(c.names...), 0o644); err != nil {
			t.Fatal(err)
		}
		x, err := readStoreIndex(file, 20)
		if got := fmt.Sprint(err); c.err == "" && (err != nil || x.n() != 2) || c.err != "" && got != c.err {
			t.Errorf("%s: %d names, err %v; want %q", c.name, x.n(), err, c.err)
		}
	}
}

// F2 (RT126FIREFSCKR2): a loose copy past the look's limit is said to be over 1 GB, as a packed one is, not to hold
// other bytes.
func TestTheFiresLookSaysALooseCopyPastItsLimitIsOver1GB(t *testing.T) {
	dir := t.TempDir()
	loose := func(object string) storeCopy {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		w.Write([]byte(object))
		w.Close()
		file := filepath.Join(dir, fmt.Sprintf("%x", sha1.Sum([]byte(object))))
		if err := os.WriteFile(file, z.Bytes(), 0o444); err != nil {
			t.Fatal(err)
		}
		return storeCopy{file: file}
	}
	good := "blob 5\x00base\n"
	name := sha1.Sum([]byte(good))
	if sound, err := loose(good).holds(context.Background(), name[:]); !sound || err != nil {
		t.Fatalf("a good loose copy: sound %v, err %v", sound, err)
	}
	big := fmt.Sprintf("blob %d\x00base\n", storeObjectLimit+1)
	if sound, err := loose(big).holds(context.Background(), name[:]); sound || fmt.Sprint(err) != "it is over 1 GB" {
		t.Fatalf("a loose copy past the limit: sound %v, err %v; want \"it is over 1 GB\"", sound, err)
	}
}
