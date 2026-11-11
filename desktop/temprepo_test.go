package desktop

import (
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
	if !inTempFolder(repo) {
		t.Fatalf("stopSigning is for repos in the temp folder only: %s", repo)
	}
	for _, key := range []string{"commit.gpgsign", "tag.gpgsign"} {
		if out, err := exec.Command("git", "-C", repo, "config", "--local", key, "false").CombinedOutput(); err != nil {
			t.Fatalf("git config --local %s: %v %s", key, err, out)
		}
	}
}

// inTempFolder says repo sits inside the temp folder (symlinks resolved: on a Mac, /var is /private/var).
func inTempFolder(repo string) bool {
	temp := physical(os.TempDir())
	here := physical(repo)
	return here != temp && strings.HasPrefix(here, temp+string(filepath.Separator))
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
	if err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "x").Run(); err == nil {
		t.Fatal("the commit went through, so the stand-in signer isn't in play")
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

// A folder outside the temp folder (the temp folder itself, its parent) is refused; one inside is taken.
func TestStopSigningOnlyTakesReposInTheTempFolder(t *testing.T) {
	if inTempFolder(os.TempDir()) || inTempFolder(filepath.Dir(filepath.Clean(os.TempDir()))) {
		t.Error("a folder outside the temp folder passed")
	}
	if !inTempFolder(t.TempDir()) {
		t.Error("a folder in the temp folder was refused")
	}
}
