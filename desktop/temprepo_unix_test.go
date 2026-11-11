//go:build !windows

package desktop

import (
	"os/exec"
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
