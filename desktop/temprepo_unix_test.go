//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// homeRepo (setup's hire tests, not built on Windows) leaves "false" in its repo's own config too,
// and commits where the global config signs everything.
func TestHomeRepoDoesNotSign(t *testing.T) {
	isolate(t)
	signsEverything(t)
	repo := homeRepo(t)
	for _, key := range []string{"commit.gpgsign", "tag.gpgsign"} {
		if got := localConfig(repo, key); got != "false" {
			t.Errorf("local %s = %q, want false", key, got)
		}
	}
	if err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "again").Run(); err != nil {
		t.Errorf("a second commit failed: %v", err)
	}
}

// `link/..` where link (inside the temp folder) points into a repo outside it: the OS goes through
// the link and then up, to the outside repo. Cleaning the text first would turn it into the temp
// folder's own subfolder, so the path must reach git as given.
func TestStopSigningRefusesALinkThenDotDotToARepoOutside(t *testing.T) {
	r := newGuardRig(t)
	if err := os.MkdirAll(filepath.Join(r.outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.temp, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(r.outside, "sub"), filepath.Join(r.temp, "x", "link")); err != nil {
		t.Fatal(err)
	}
	r.refused(r.temp+"/x/link/..", "link/..") // string concatenation: filepath.Join would clean the ".." away
	if err := os.Symlink(r.outside, filepath.Join(r.temp, "direct")); err != nil {
		t.Fatal(err)
	}
	r.refused(filepath.Join(r.temp, "direct"), "a link straight to the repo")
	r.refused(r.temp+"/../outside", "..")
}
