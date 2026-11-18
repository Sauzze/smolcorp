//go:build unix

package tmuxfake

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const repo = "../../.."

// pristineClient is tmux 3.8's own client.c, from the pinned tarball scripts/build-tmux.sh downloaded into
// build/third-party, or "" when it hasn't (CI's Linux runners, a fresh checkout).
func pristineClient(t *testing.T) string {
	t.Helper()
	tarball := filepath.Join(repo, "build", "third-party", "tmux-3.8.tar.gz")
	if _, err := os.Stat(tarball); err != nil {
		return ""
	}
	dir := t.TempDir()
	if out, err := exec.Command("tar", "-xzf", tarball, "-C", dir, "tmux-3.8/client.c").CombinedOutput(); err != nil {
		t.Fatalf("tar: %v %s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tmux-3.8", "client.c"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// applyGuard runs `build-tmux.sh apply-guard` from root (the repo, or a copy of its scripts and third_party) on a
// source folder holding client.c.
func applyGuard(t *testing.T, root, client string) result {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "tmux-3.8")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(tree, "client.c"), client, 0o644)
	script, _ := filepath.Abs(filepath.Join(root, "scripts", "build-tmux.sh"))
	return runProgram(t, []string{"PATH=/usr/bin:/bin", "HOME=" + tree, "LANG=C"}, "", "/bin/bash", script, "apply-guard", tree)
}

// The build stops when the guard didn't apply: a patch that isn't the pinned one, a client.c it doesn't fit, or one
// it fits only by fuzz or offset, so that the result isn't the pinned guarded file. The pinned file itself passes.
func TestTheBuildStopsWhenTheGuardDidntApply(t *testing.T) {
	if r := applyGuard(t, repo, "int main(void) { return 0; }\n"); r.status != 1 || !strings.Contains(r.stderr, "guard.patch didn't apply") {
		t.Errorf("a client.c that isn't tmux's: %+v", r)
	}

	// A copy of the repo's scripts and pins, with one byte of the patch's comment changed.
	copyRoot := t.TempDir()
	for _, name := range []string{"scripts/build-tmux.sh", "third_party/tmux/SHA256SUMS", "third_party/tmux/guard.patch"} {
		data, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "third_party/tmux/guard.patch" {
			data = []byte(strings.Replace(string(data), "Smolcorp's guard.", "Smolcorp's guard!", 1))
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(copyRoot, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(copyRoot, name), string(data), 0o755)
	}
	if r := applyGuard(t, copyRoot, "int main(void) { return 0; }\n"); r.status != 1 || !strings.Contains(r.stderr, "guard.patch doesn't match its line in SHA256SUMS") {
		t.Errorf("a changed patch: %+v", r)
	}

	pristine := pristineClient(t)
	if pristine == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("SMOLCORP_SANDBOX_REQUIRED=1 and no build/third-party/tmux-3.8.tar.gz: run scripts/build-tmux.sh first")
		}
		t.Skip("SKIPPED the rest, no build/third-party/tmux-3.8.tar.gz here (scripts/build-tmux.sh downloads it)")
	}
	if r := applyGuard(t, repo, pristine); r.status != 0 || !strings.Contains(r.stdout, "the guard is applied") {
		t.Errorf("tmux 3.8's own client.c: %+v", r)
	}
	// The patch still applies (with an offset), but the result isn't the pinned file.
	if r := applyGuard(t, repo, "/* a line more */\n"+pristine); r.status != 1 || !strings.Contains(r.stderr, "isn't the guarded file SHA256SUMS pins") {
		t.Errorf("a client.c with a line more: %+v", r)
	}
}

// scripts/check-runtime.sh fails a hardened tmux that isn't guarded, and passes the guarded build once it's signed
// with the runtime (as build-app.sh signs it).
func TestCheckRuntimeFailsATmuxWithoutTheGuard(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("check-runtime.sh is the Mac's")
	}
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("SKIPPED, no cc here for the stand-in")
	}
	script, _ := filepath.Abs(filepath.Join(repo, "scripts", "check-runtime.sh"))
	dir := t.TempDir()
	stock := filepath.Join(dir, "stock", "tmux")
	os.MkdirAll(filepath.Dir(stock), 0o755)
	writeFile(t, filepath.Join(dir, "stock.c"), "#include <stdio.h>\nint main(int c, char **v) { fprintf(stderr, \"no server running on %s\\n\", v[2]); return 1; }\n", 0o644)
	sign := func(path string) {
		if out, err := exec.Command("codesign", "--force", "--sign", "-", "--options", "runtime", path).CombinedOutput(); err != nil {
			t.Fatalf("codesign: %v %s", err, out)
		}
	}
	if out, err := exec.Command("cc", "-o", stock, filepath.Join(dir, "stock.c")).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v %s", err, out)
	}
	sign(stock)
	env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + dir}
	if r := runProgram(t, env, "", script, stock); r.status != 1 || !strings.Contains(r.stdout, "isn't the guarded tmux") {
		t.Errorf("a hardened stock stand-in should fail: %+v", r)
	}
	guarded := clients()[1]
	guarded.need(t)
	copied := filepath.Join(dir, "guarded", "tmux")
	os.MkdirAll(filepath.Dir(copied), 0o755)
	data, err := os.ReadFile(guarded.path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, copied, string(data), 0o755)
	sign(copied)
	if r := runProgram(t, env, "", script, copied); r.status != 0 {
		t.Errorf("the guarded build, hardened, should pass: %+v", r)
	}
}
