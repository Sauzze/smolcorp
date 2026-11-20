package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

// FXRC2FIXB (RT126SWEEP4-P1 F1), on this Mac with stand-ins only, under the real profile: the GM in the projects folder
// makes no .git and no HEAD at ~/.smolcorp, above the core's folder for hires' worktrees, nor above a configured folder
// for them; and neither does a hire.
func TestOnAMacNoDeskMakesAGitFolderAboveTheFoldersForHiresWorktrees(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	smolcorp := filepath.Join(d.home, ".smolcorp")
	worktrees, dev := filepath.Join(smolcorp, "worktrees"), filepath.Join(d.home, "dev")
	configured := filepath.Join(dev, "wt")
	rows := func(desk string) []escapeRow {
		return []escapeRow{
			{desk + ", control: another folder in ~/.smolcorp", attempt{"mkdir", filepath.Join(smolcorp, "other-"+desk), ""}, "ok"},
			{desk + ", control: a file beside the configured folder", attempt{"create", filepath.Join(dev, "notes-"+desk+".md"), ""}, "ok"},
			{desk + ": a .git at ~/.smolcorp", attempt{"mkdir", filepath.Join(smolcorp, ".git"), ""}, "EPERM"},
			{desk + ": a .git file there, in another case", attempt{"create", filepath.Join(smolcorp, ".GIT"), ""}, "EPERM"},
			{desk + ": a bare repo's HEAD at ~/.smolcorp", attempt{"create", filepath.Join(smolcorp, "HEAD"), ""}, "EPERM"},
			{desk + ": a .git in the core's folder", attempt{"mkdir", filepath.Join(worktrees, ".git"), ""}, "EPERM"},
			{desk + ": a .git above the configured folder", attempt{"mkdir", filepath.Join(dev, ".git"), ""}, "EPERM"},
			{desk + ": a HEAD above the configured folder", attempt{"create", filepath.Join(dev, "HEAD"), ""}, "EPERM"},
		}
	}
	for _, folder := range []string{worktrees, configured} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	hire, hireRoot := d.pl, d.root

	// The GM in the projects folder, its Root its workspace in the support folder.
	projects := filepath.Join(d.home, "Projects")
	plans := filepath.Join(projects, ".smolcorp", "plans", "h1")
	workspace := Workspace(d.support, "h1")
	for _, folder := range []string{plans, workspace} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	d.pl.Subject, d.pl.Root = GM, workspace
	d.pl.Checkout, d.pl.GitDir, d.pl.OwnWorktree, d.pl.OtherWorktrees, d.pl.DefaultBranch, d.pl.HiresFolder, d.pl.GMFolder =
		"", "", "", nil, "", "", ""
	d.pl.GMProjects, d.pl.GMPlans = projects, plans
	d.pl.Worktrees, d.pl.GMHires = []string{worktrees}, []string{configured}
	d.prepare(t)
	d.root = workspace
	d.check(t, rows("the GM"))

	// A hire made before the core's folder, in its repo's folder for hires.
	d.pl, d.root = hire, hireRoot
	d.pl.Worktrees = []string{worktrees, configured}
	d.prepare(t)
	d.check(t, rows("a hire"))
}
