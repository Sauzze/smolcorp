package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stopSigning is for a throwaway repo a test made in its own temp folder, called right after
// `git init`: its commits and tags must never ask the owner's signing key for a signature (1Password
// pops up, or a locked one fails the commit). It writes that repo's OWN config only: never the
// global one, never a repo the test didn't make.
func stopSigning(t *testing.T, repo string) {
	t.Helper()
	if err := stopSigningIn(repo, os.TempDir()); err != nil {
		t.Fatal(err)
	}
}

// stopSigningIn does the work for a given temp folder and says why it refused, if it did.
func stopSigningIn(repo, temp string) error {
	dir, err := ownGitDir(repo, temp)
	if err != nil {
		return err
	}
	for _, key := range []string{"commit.gpgsign", "tag.gpgsign"} {
		// --git-dir names the folder git itself says holds the config, so nothing else can be written.
		if out, err := exec.Command("git", "--git-dir="+dir, "config", "--local", key, "false").CombinedOutput(); err != nil {
			return fmt.Errorf("git config --local %s: %v %s", key, err, out)
		}
	}
	return nil
}

// ownGitDir is where git keeps the config repo really uses (a worktree shares its main repo's),
// resolved on disk; an error unless that folder sits strictly inside the temp folder. git is asked
// with the path exactly as given (never cleaned as text), so `link/..` is followed the way the OS
// does. A folder that isn't a repo, or only sits inside another repo, answers with nothing or with
// that repo's folder: both refused.
func ownGitDir(repo, temp string) (string, error) {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("stopSigning is for a repo made in the temp folder, and %s isn't a repo: %v", repo, err)
	}
	dir := physical(strings.TrimSpace(string(out)))
	if !underFolder(dir, physical(temp)) {
		return "", fmt.Errorf("stopSigning is for repos in the temp folder only: %s keeps its config in %s", repo, dir)
	}
	return dir, nil
}

// underFolder says path is under folder by whole path parts (/a/tmp2/x is not under /a/tmp),
// and is not folder itself.
func underFolder(path, folder string) bool {
	rel, err := filepath.Rel(folder, path)
	if err != nil || rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// localConfig is what `git config --local key` says in repo ("" if unset).
func localConfig(repo, key string) string {
	out, _ := exec.Command("git", "-C", repo, "config", "--local", key).Output()
	return strings.TrimSpace(string(out))
}

// globalConfig is what `git config --global key` says, as this test process sees it.
func globalConfig(key string) string {
	out, _ := exec.Command("git", "config", "--global", key).Output()
	return strings.TrimSpace(string(out))
}

// signsEverything points git's global config at a stand-in that signs every commit and tag through a
// signer that always fails, as the owner's locked 1Password does. The real global config is never read.
func signsEverything(t *testing.T) {
	t.Helper()
	stand := filepath.Join(t.TempDir(), "signing.gitconfig")
	text := "[commit]\n\tgpgsign = true\n[tag]\n\tgpgsign = true\n[gpg]\n\tprogram = smolcorp-no-such-signer\n"
	if err := os.WriteFile(stand, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", stand)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// The stand-in really breaks a signed commit, so the tests below prove something.
func TestTheStandInSignerBreaksSignedCommits(t *testing.T) {
	signsEverything(t)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	out, err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "x").CombinedOutput()
	if err == nil {
		t.Fatal("the commit went through, so the stand-in signer isn't in play")
	}
	if !strings.Contains(string(out), "sign") {
		t.Fatalf("it failed, but not for the signer: %s", out)
	}
}

// Every test helper that makes a repo and commits leaves "false" in that repo's own config, and a
// commit goes through even where the global config signs everything. The global config stays as it was.
func TestTempRepoHelpersDoNotSign(t *testing.T) {
	isolate(t)
	signsEverything(t)
	before := globalConfig("commit.gpgsign")
	if before != "true" {
		t.Fatalf("the stand-in global config says commit.gpgsign=%q", before)
	}
	rig := newT3Rig(t)
	repos := map[string]string{"placeRepo": placeRepo(t), "t3Rig.repo": rig.repo()}
	for name, repo := range repos {
		for _, key := range []string{"commit.gpgsign", "tag.gpgsign"} {
			if got := localConfig(repo, key); got != "false" {
				t.Errorf("%s: local %s = %q, want false", name, key, got)
			}
		}
		if err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"commit", "-q", "--allow-empty", "-m", "again").Run(); err != nil {
			t.Errorf("%s: a second commit failed: %v", name, err)
		}
	}
	if after := globalConfig("commit.gpgsign"); after != before {
		t.Errorf("global commit.gpgsign went from %q to %q", before, after)
	}
}

// Whole path parts, not string prefixes; the temp folder itself is not inside itself.
func TestStopSigningComparesWholePathParts(t *testing.T) {
	sep := string(filepath.Separator)
	at := func(parts ...string) string { return sep + strings.Join(parts, sep) }
	if underFolder(at("a", "tmp2", "repo", ".git"), at("a", "tmp")) {
		t.Error("tmp2 passed as inside tmp")
	}
	if underFolder(at("a", "tmp"), at("a", "tmp")) || underFolder(at("a"), at("a", "tmp")) {
		t.Error("the temp folder itself, or its parent, passed")
	}
	if !underFolder(at("a", "tmp", "repo", ".git"), at("a", "tmp")) {
		t.Error("a folder in the temp folder was refused")
	}
}

// guardRig is a pretend temp folder ("tmp") next to a repo outside it ("outside"), standing in for
// the owner's real repo. Every refusal below must leave that repo's own config as it was.
type guardRig struct {
	t       *testing.T
	root    string
	temp    string
	outside string
}

func newGuardRig(t *testing.T) *guardRig {
	t.Helper()
	isolate(t)
	signsEverything(t) // a stand-in global config: the owner's real one is never read
	root := physical(t.TempDir())
	r := &guardRig{t: t, root: root, temp: filepath.Join(root, "tmp"), outside: filepath.Join(root, "outside")}
	if err := os.MkdirAll(r.temp, 0o755); err != nil {
		t.Fatal(err)
	}
	r.git(root, "init", "-q", "-b", "main", r.outside)
	r.git(r.outside, "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", "x")
	return r
}

func (r *guardRig) git(dir string, args ...string) {
	r.t.Helper()
	all := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)
	if out, err := exec.Command("git", all...).CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// refused: the guard says no, in its own words, and the outside repo's config has no signing line.
func (r *guardRig) refused(repo, why string) {
	r.t.Helper()
	if dir, err := ownGitDir(repo, r.temp); err == nil {
		r.t.Errorf("%s: the guard let %s through, to %s", why, repo, dir)
	}
	if err := stopSigningIn(repo, r.temp); err == nil {
		r.t.Errorf("%s: stopSigning took %s", why, repo)
	}
	for _, key := range []string{"commit.gpgsign", "tag.gpgsign"} {
		if got := localConfig(r.outside, key); got != "" {
			r.t.Errorf("%s: the outside repo's local %s is %q, it was not touched before", why, key, got)
		}
	}
}

// A worktree of a repo outside the temp folder sits in it but shares that repo's config.
func TestStopSigningRefusesAWorktreeOfARepoOutside(t *testing.T) {
	r := newGuardRig(t)
	tree := filepath.Join(r.temp, "wt")
	r.git(r.outside, "worktree", "add", "-q", "-b", "side", tree)
	r.refused(tree, "worktree of an outside repo")
}

// A folder that isn't a repo: git either finds nothing, or walks up to a repo around the temp folder.
func TestStopSigningRefusesAFolderThatIsNotARepo(t *testing.T) {
	r := newGuardRig(t)
	plain := filepath.Join(r.temp, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	r.refused(plain, "a plain folder")
	r.refused(filepath.Join(r.temp, "missing"), "a folder that isn't there")
	// A temp folder that itself sits inside the outside repo: the plain folder's repo is the outside one.
	inner := filepath.Join(r.outside, "tmp")
	if err := os.MkdirAll(filepath.Join(inner, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.temp = inner
	r.refused(filepath.Join(inner, "plain"), "a plain folder, temp folder inside a repo")
}

// A temp path that is only a string prefix of another ("tmp" and "tmp2") is a different folder.
func TestStopSigningRefusesARepoInASiblingWithTheSamePrefix(t *testing.T) {
	r := newGuardRig(t)
	sibling := filepath.Join(r.root, "tmp2", "repo")
	r.git(r.root, "init", "-q", "-b", "main", sibling)
	if err := stopSigningIn(sibling, r.temp); err == nil {
		t.Error("tmp2/repo was taken as inside tmp")
	}
	if got := localConfig(sibling, "commit.gpgsign"); got != "" {
		t.Errorf("the sibling repo's local commit.gpgsign is %q", got)
	}
}

// A worktree of a repo that IS in the temp folder is fine: it shares that repo's own config.
func TestStopSigningTakesAWorktreeOfATempRepo(t *testing.T) {
	r := newGuardRig(t)
	repo, tree := filepath.Join(r.temp, "repo"), filepath.Join(r.temp, "wt")
	r.git(r.root, "init", "-q", "-b", "main", repo)
	r.git(repo, "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", "x")
	r.git(repo, "worktree", "add", "-q", "-b", "side", tree)
	if err := stopSigningIn(tree, r.temp); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(repo, "commit.gpgsign"); got != "false" {
		t.Errorf("the temp repo's local commit.gpgsign is %q, want false", got)
	}
}
