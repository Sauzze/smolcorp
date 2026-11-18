package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-X6's sandbox on this Mac, with stand-ins only (owner Q5 (a)): a hire's real profile from its Policy (SA-X1, M1,
// M2), Apple's sandbox-exec, and this test program as `smolcorp sandbox enter` and as a probe. The core's git on the
// hire's tree runs in the hire's sandbox; the probe, run the same way, is refused a stand-in secret in the support
// folder that it reads outside; a profile that doesn't hold refuses the run before anything of it starts; and no
// profile is the hardened run alone.

func init() {
	deskRunStandIn = func(args []string) int {
		switch {
		case args[0] == "sandbox":
			return sandbox.Run(args[1:], os.Stdout, os.Stderr)
		case args[0] == "probe" && len(args) == 3:
			// Leaves a mark where it may write, then says what reading the stand-in secret gave.
			if err := os.WriteFile(args[2], []byte("probe ran\n"), 0o600); err != nil {
				fmt.Println("mark:", err)
			}
			data, err := os.ReadFile(args[1])
			switch {
			case errors.Is(err, syscall.EPERM):
				fmt.Println("denied")
			case err != nil:
				fmt.Println("error:", err)
			default:
				fmt.Println("read:", string(data))
			}
			return 0
		}
		return 2
	}
}

// sandboxedHire gives r's hire a real profile at hires/<id>/sandbox.sb, as its start would (SA-M6), and has the
// core run this test program for `smolcorp`. It skips where this Mac's sandbox-exec can't be used, unless
// SMOLCORP_SANDBOX_REQUIRED=1 (the runner).
func sandboxedHire(t *testing.T, r *deskRunRig) (profile string) {
	t.Helper()
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("this Mac's sandbox-exec: %v", err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	was := deskRunSmolcorp
	deskRunSmolcorp = func() string { return physical(program) }
	t.Cleanup(func() { deskRunSmolcorp = was })
	t.Setenv(deskRunStandInVariable, "1")
	t.Setenv("HOME", homeDir)
	// The run's stand-in other server goes in the sandboxed desks' socket folder, as its start's does (SA-M6): a short
	// one of the test's, never the core's real one.
	sockets, err := os.MkdirTemp("", "dr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	parent, savedParent := filepath.Join(physical(sockets), "p"), deskSocketParent
	deskSocketParent = func() string { return parent }
	t.Cleanup(func() { deskSocketParent = savedParent })
	own := hireFolder(supportDir, r.hire.ID)
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(own) })
	pol, err := sandbox.New(sandbox.Places{GOOS: "darwin", Home: physical(homeDir), Support: physical(supportDir),
		OwnID: r.hire.ID, Root: r.tree, Checkout: r.repo, GitDir: filepath.Join(r.repo, ".git"), OwnWorktree: "ada-abcdef",
		DefaultBranch: "main", HiresFolder: filepath.Dir(r.tree)})
	if err != nil {
		t.Fatal(err)
	}
	var services sandbox.Services
	for _, m := range sandbox.MachServices() {
		services.Mach = append(services.Mach, m.Name)
	}
	profile = filepath.Join(own, "sandbox.sb")
	if err := sandbox.PrepareProfile(pol, profile, services); err != nil {
		t.Fatal(err)
	}
	return profile
}

// probe runs this program as the probe for the hire, as deskRun runs what the core runs for it.
func probe(t *testing.T, id, secret, mark string) (string, error) {
	t.Helper()
	program, _ := os.Executable()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out bytes.Buffer
	err := deskRun(ctx, id, os.TempDir(), os.Environ(), &out, &out, physical(program), "probe", secret, mark)
	return strings.TrimSpace(out.String()), err
}

func TestOnAMacTheCoresGitOnASandboxedHiresTreeRunsInItsSandbox(t *testing.T) {
	r := newDeskRunRig(t)
	sandboxedHire(t, r)
	secret := filepath.Join(physical(supportDir), "stand-in-secret")
	if err := os.WriteFile(secret, []byte("FAKEKEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(secret) })
	mark := filepath.Join(r.tree, "probe-mark")

	// The control: outside any sandbox, the probe reads the secret.
	if said, err := probe(t, "", secret, mark); err != nil || said != "read: FAKEKEY" {
		t.Fatalf("the control: %q %v", said, err)
	}
	os.Remove(mark)
	// In the hire's sandbox it can't, though it writes in the hire's own tree.
	if said, err := probe(t, r.hire.ID, secret, mark); err != nil || said != "denied" {
		t.Errorf("in the hire's sandbox: %q %v, want denied", said, err)
	}
	if _, err := os.Stat(mark); err != nil {
		t.Errorf("the probe didn't run in the sandbox, or couldn't write the hire's tree: %v", err)
	}
	os.Remove(mark)

	// The core's own git, on the tree, through runGit: it works in the sandbox, and the canaries it made are gone.
	if status, out, ok := runGit(r.tree, "status", "--porcelain"); !ok || status != 0 || out != "" {
		t.Errorf("status in the sandbox: %d %q %v", status, out, ok)
	}
	if status, out, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok || status != 0 || out != r.hire.Branch {
		t.Errorf("symbolic-ref in the sandbox: %d %q %v", status, out, ok)
	}
	if left, _ := filepath.Glob(filepath.Join(supportDir, "sandbox-canary-*")); len(left) != 0 {
		t.Errorf("canaries left: %v", left)
	}
	// The fire, whole: its checks in the sandbox, the removal in the checkout.
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's check: %q", message)
	}
	if !removeOwnWorktree(runGit, "", r.hire) {
		t.Error("the clean worktree was kept")
	}
	if out := gitRepoCmd(t, r.repo, "branch", "--list", r.hire.Branch); out != "" {
		t.Errorf("its branch is still there: %q", out)
	}
}

// A profile that is there but whose sandbox doesn't hold (one that allows everything, so the canary reads) refuses
// the run: the program never starts, and nothing runs outside instead. A profile that isn't a file refuses it too.
func TestOnAMacAHiresSandboxThatDoesntHoldRefusesTheCoresRun(t *testing.T) {
	r := newDeskRunRig(t)
	profile := sandboxedHire(t, r)
	secret := filepath.Join(physical(supportDir), "stand-in-secret")
	mark := filepath.Join(r.tree, "probe-mark")
	if err := os.WriteFile(profile, []byte("(version 1)\n(allow default)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var refused *sandbox.Refused
	// `enter` says why on its standard error, as on a desk's terminal; the probe never says anything.
	if said, err := probe(t, r.hire.ID, secret, mark); !errors.As(err, &refused) || !strings.HasPrefix(said, sandbox.Refusal("darwin")) {
		t.Errorf("an open profile: %q %v, want refused before the probe", said, err)
	}
	if _, err := os.Stat(mark); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the probe ran: %v", err)
	}
	if _, _, ok := runGit(r.tree, "status", "--porcelain"); ok {
		t.Error("the core's git ran under an open profile")
	}
	if message := uncommittedRefusal(r.hire, runGit); !strings.Contains(message, "Couldn't tell") {
		t.Errorf("the fire's check: %q", message)
	}

	os.Remove(profile)
	if err := os.Symlink(filepath.Join(r.base, "elsewhere.sb"), profile); err != nil {
		t.Fatal(err)
	}
	if _, err := probe(t, r.hire.ID, secret, mark); err == nil {
		t.Error("a profile that is a link was used")
	}
	if _, err := os.Stat(mark); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the probe ran: %v", err)
	}
}

// A hire with no profile (the sandbox off, a desk from before the update) gets the hardened git alone, outside.
func TestOnAMacAHireWithNoProfileGetsTheHardenedGitAlone(t *testing.T) {
	r := newDeskRunRig(t)
	cmd, held, err := deskSandboxCommand(r.hire.ID, nil, "/usr/bin/true")
	if cmd != nil || held.start != nil || err != nil {
		t.Fatalf("no profile: %v %v %v", cmd, held, err)
	}
	if _, _, err := deskSandboxCommand("../x", nil, "/usr/bin/true"); err == nil {
		t.Error("an id that leads out of hires/ was taken")
	}
	if status, out, ok := runGit(r.tree, "status", "--porcelain"); !ok || status != 0 || out != "" {
		t.Errorf("status with no profile: %d %q %v", status, out, ok)
	}
}

// RT126SAX6 N5: gh's run for a hire's branch goes to deskRun with that hire: a profile that isn't a file refuses it
// before the stand-in gh starts. The control, the same run for no desk, starts it.
func TestOnAMacTheBadgesGhRunForAHiresBranchIsTheHiresRun(t *testing.T) {
	r := newDeskRunRig(t)
	bin := filepath.Join(r.base, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(r.standIn(t, "gh"), filepath.Join(bin, "gh")); err != nil {
		t.Fatal(err)
	}
	was := ghPlaces
	ghPlaces = []string{bin}
	t.Cleanup(func() { ghPlaces = was })
	own := hireFolder(supportDir, r.hire.ID)
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(own) })
	if err := os.Symlink(filepath.Join(r.base, "elsewhere.sb"), filepath.Join(own, "sandbox.sb")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	runGH(ctx, "owner/repo", r.hire.Branch)
	if got := r.ran(t); got != "gh" {
		t.Fatalf("the control: gh for no desk ran %q", got)
	}
	if _, _, _, err := runGH(withGHDesk(ctx, r.tree), "owner/repo", r.hire.Branch); err == nil {
		t.Error("gh for the hire's branch ran though its profile isn't a file")
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the stand-in gh ran: %q", got)
	}
}
