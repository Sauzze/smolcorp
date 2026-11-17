package desktop

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXREPLACEREFS: the core reads a base branch as it is. A desk could write, in the shared .git, what makes git read
// main as another tree or history while main never moves (a replace ref, grafts, a commit-graph, a multi-pack index);
// every git the core runs goes through gitReadEnv, which leaves each of them out. Throwaway repos only: main is A then
// B (f = "base"), and the desk's commit D, on a branch of its own with no parent, has f = "desk". Each case shows the
// owner's plain git reading the desk's version first, so the case isn't empty.

type substRepo struct{ dir, a, b, d string }

func aSubstRepo(t *testing.T) substRepo {
	t.Helper()
	r := substRepo{dir: physical(t.TempDir())}
	gitRepoCmd(t, r.dir, "init", "-q", "-b", "main")
	gitRepoCmd(t, r.dir, "config", "commit.gpgsign", "false")
	gitRepoCmd(t, r.dir, "config", "gc.auto", "0")
	writeRepoFile(t, r.dir, "f", "one\n")
	gitRepoCmd(t, r.dir, "add", "f")
	gitRepoCmd(t, r.dir, "commit", "-q", "-m", "A")
	r.a = gitRepoCmd(t, r.dir, "rev-parse", "HEAD")
	writeRepoFile(t, r.dir, "f", "base\n")
	gitRepoCmd(t, r.dir, "commit", "-q", "-am", "B")
	r.b = gitRepoCmd(t, r.dir, "rev-parse", "HEAD")
	gitRepoCmd(t, r.dir, "checkout", "-q", "--orphan", "hire/x")
	writeRepoFile(t, r.dir, "f", "desk\n")
	gitRepoCmd(t, r.dir, "add", "f")
	gitRepoCmd(t, r.dir, "commit", "-q", "-m", "D")
	r.d = gitRepoCmd(t, r.dir, "rev-parse", "HEAD")
	gitRepoCmd(t, r.dir, "checkout", "-q", "main")
	return r
}

// plain is the owner's own git in the repo, with no system or global config: its output and exit code.
func (r substRepo) plain(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "GIT_") }),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return strings.TrimSpace(string(out)), exit.ExitCode()
	} else if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out)), 0
}

// core is the core's own read of the repo (readGitOut, gitReadArgs): its output and exit code.
func (r substRepo) core(t *testing.T, args ...string) (string, int) {
	t.Helper()
	out, status, err := readGitOut(context.Background(), r.dir, gitCall{global: gitReadArgs}, args...)
	if err != nil {
		t.Fatalf("the core's git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out)), status
}

// gitChunks is a commit-graph's or multi-pack index's chunk offsets by id: the table starts at start, after its header.
func gitChunks(t *testing.T, data []byte, count, start int) map[string]int {
	t.Helper()
	chunks := map[string]int{}
	for i := 0; i <= count; i++ {
		at := start + 12*i
		chunks[string(data[at:at+4])] = int(binary.BigEndian.Uint64(data[at+4 : at+12]))
	}
	return chunks
}

// gitObjectIndex is where id is in a chunk file's sorted object names (OIDF, OIDL).
func gitObjectIndex(t *testing.T, data []byte, chunks map[string]int, id string) int {
	t.Helper()
	n := int(binary.BigEndian.Uint32(data[chunks["OIDF"]+4*255:]))
	for i := 0; i < n; i++ {
		at := chunks["OIDL"] + 20*i
		if string(data[at:at+20]) == string(hexID(t, id)) {
			return i
		}
	}
	t.Fatalf("%s isn't in the file", id)
	return 0
}

func hexID(t *testing.T, id string) []byte {
	t.Helper()
	var b [20]byte
	if len(id) != 40 {
		t.Fatalf("not a SHA-1 name: %q", id)
	}
	for i := range b {
		var v byte
		for _, c := range id[2*i : 2*i+2] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= byte(c - '0')
			default:
				v |= byte(c-'a') + 10
			}
		}
		b[i] = v
	}
	return b[:]
}

// rewriteSealed changes a git data file and puts its trailing SHA-1 right, as a desk with write access could.
func rewriteSealed(t *testing.T, file string, change func(data []byte)) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	change(data)
	sum := sha1.Sum(data[:len(data)-20])
	copy(data[len(data)-20:], sum[:])
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheCoresReadsLeaveGitsSubstitutionsOut(t *testing.T) {
	app := []string{"PATH=/bin", "GIT_NO_REPLACE_OBJECTS=0", "git_graft_file=/elsewhere/grafts", "GIT_REPLACE_REF_BASE=refs/heads/"}
	env := gitReadEnv(app, [][2]string{{"core.commitGraph", "true"}, {"core.multiPackIndex", "true"}})
	for _, want := range []string{"GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="} {
		if n := len(slices.DeleteFunc(slices.Clone(env), func(kv string) bool { return kv != want })); n != 1 {
			t.Errorf("%s is there %d times: %q", want, n, env)
		}
	}
	for _, kv := range env {
		name, _, _ := strings.Cut(strings.ToUpper(kv), "=")
		if name == "GIT_REPLACE_REF_BASE" || name == "GIT_NO_REPLACE_OBJECTS" && kv != "GIT_NO_REPLACE_OBJECTS=1" || name == "GIT_GRAFT_FILE" && kv != "GIT_GRAFT_FILE=" {
			t.Errorf("the app's %s is handed on", kv)
		}
	}
	// The caller's values first, gitReadAround's last: git takes the last.
	if got := strings.Join(env, " "); !strings.HasSuffix(got, "GIT_CONFIG_COUNT=4 GIT_CONFIG_KEY_0=core.commitGraph GIT_CONFIG_VALUE_0=true "+
		"GIT_CONFIG_KEY_1=core.multiPackIndex GIT_CONFIG_VALUE_1=true GIT_CONFIG_KEY_2=core.commitGraph GIT_CONFIG_VALUE_2=false "+
		"GIT_CONFIG_KEY_3=core.multiPackIndex GIT_CONFIG_VALUE_3=false") {
		t.Errorf("the settings around a read aren't last: %s", got)
	}
}

func TestTheCoresGitReadsABaseBranchAsItIs(t *testing.T) {
	t.Run("a replace ref", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "replace", r.b, r.d)
		if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
			t.Fatalf("the owner's git reads main:f as %q: the replace ref isn't taken", got)
		}
		if got, _ := r.core(t, "cat-file", "-p", "main:f"); got != "base" {
			t.Errorf("the core's read of main:f: %q, want base", got)
		}
		blob, _ := r.plain(t, "--no-replace-objects", "rev-parse", r.b+":f")
		if status, out, ok := ownerGit(r.dir, []string{"rev-parse", "main:f"}); !ok || status != 0 || out != blob {
			t.Errorf("the core's rev-parse main:f (ownerGit): %d %q %v, want base's blob %s", status, out, ok, blob)
		}
	})
	t.Run("grafts", func(t *testing.T) {
		r := aSubstRepo(t)
		writeRepoFile(t, r.dir, ".git/info/grafts", r.b+" "+r.d+"\n")
		if _, status := r.plain(t, "merge-base", "--is-ancestor", r.d, "main"); status != 0 {
			t.Fatalf("the owner's git: D is no ancestor of main (%d): the grafts aren't taken", status)
		}
		if out, status := r.core(t, "merge-base", "--is-ancestor", r.d, "main"); status != 1 || out != "" {
			t.Errorf("the core's read: D is an ancestor of main (%d, %q)", status, out)
		}
	})
	t.Run("a commit-graph", func(t *testing.T) {
		r := aSubstRepo(t)
		gitRepoCmd(t, r.dir, "commit-graph", "write", "--reachable")
		rewriteSealed(t, filepath.Join(r.dir, ".git", "objects", "info", "commit-graph"), func(data []byte) {
			chunks := gitChunks(t, data, int(data[6]), 8)
			at := chunks["CDAT"] + 36*gitObjectIndex(t, data, chunks, r.b) + 20 // B's first parent
			binary.BigEndian.PutUint32(data[at:], uint32(gitObjectIndex(t, data, chunks, r.d)))
		})
		if _, status := r.plain(t, "merge-base", "--is-ancestor", r.d, "main"); status != 0 {
			t.Fatalf("the owner's git: D is no ancestor of main (%d): the commit-graph isn't taken", status)
		}
		if _, status := r.core(t, "merge-base", "--is-ancestor", r.d, "main"); status != 1 {
			t.Errorf("the core's read: D is an ancestor of main (%d)", status)
		}
		if _, status := r.core(t, "merge-base", "--is-ancestor", r.a, "main"); status != 0 {
			t.Errorf("the core's read: A is no ancestor of main (%d)", status)
		}
	})
	t.Run("a multi-pack index", func(t *testing.T) {
		r := aMidxRepo(t)
		if got, _ := r.plain(t, "show", "main:f"); got != "desk" {
			t.Fatalf("the owner's git reads main:f as %q: the multi-pack index isn't taken", got)
		}
		if got, _ := r.core(t, "cat-file", "-p", "main:f"); got != "base" {
			t.Errorf("the core's read of main:f: %q, want base", got)
		}
	})
}

// aMidxRepo is aSubstRepo packed, with a multi-pack index whose places for main's f and the desk's f are swapped.
func aMidxRepo(t *testing.T) substRepo {
	t.Helper()
	r := aSubstRepo(t)
	gitRepoCmd(t, r.dir, "repack", "-q", "-a", "-d")
	gitRepoCmd(t, r.dir, "multi-pack-index", "write")
	base, _ := r.plain(t, "rev-parse", r.b+":f")
	desk, _ := r.plain(t, "rev-parse", r.d+":f")
	rewriteSealed(t, filepath.Join(r.dir, ".git", "objects", "pack", "multi-pack-index"), func(data []byte) {
		chunks := gitChunks(t, data, int(data[6]), 12)
		i, j := chunks["OOFF"]+8*gitObjectIndex(t, data, chunks, base), chunks["OOFF"]+8*gitObjectIndex(t, data, chunks, desk)
		a, b := bytes.Clone(data[i:i+8]), bytes.Clone(data[j:j+8])
		copy(data[i:], b)
		copy(data[j:], a)
	})
	return r
}

// prepareHire's worktree is made through a git folder of the core's own, on the repo's objects: a multi-pack index a
// desk wrote there would give the new hire main's files with the desk's bytes.
func TestAHiresWorktreeIsMadeFromTheBaseAsItIs(t *testing.T) {
	isolate(t)
	r := aMidxRepo(t)
	owners := filepath.Join(t.TempDir(), "owners")
	if _, status := r.plain(t, "worktree", "add", "-q", "--detach", owners, "main"); status != 0 {
		t.Fatalf("the owner's worktree add: %d", status)
	}
	if data, _ := os.ReadFile(filepath.Join(owners, "f")); string(data) != "desk\n" {
		t.Fatalf("the owner's worktree of main has f = %q: nothing is substituted", data)
	}
	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "x", Folder: r.dir, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(place.Worktree, "f")); string(data) != "base\n" {
		t.Errorf("the hire's worktree has f = %q (%v), want main's own", data, err)
	}
}
