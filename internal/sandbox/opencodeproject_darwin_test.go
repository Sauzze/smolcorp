package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

// FXRC3FIXR1 (RT126RC3FIX's N1), on this Mac with stand-ins only: from a summary's folder in the support folder,
// OpenCode climbs to the first .git or .hg and runs git or hg in the folder holding it, as the owner and outside any
// sandbox. Under the real profile, neither a hire nor a GM makes one, as a folder or a file, by a rename or as a link,
// in ~/Library/Application Support, ~/Library or ~, nor HEAD beside it; nor one in the support folder or a summary's
// folder there. The owner's own still read. Nothing here runs git, hg or OpenCode.

// openCodeProjectRows are the rows for a desk d working in own: the writes, the ones beside them, and the controls.
func openCodeProjectRows(t *testing.T, d *escapeDesk, own string) []escapeRow {
	t.Helper()
	lib := filepath.Join(d.home, "Library")
	as := filepath.Join(lib, "Application Support")
	summary := filepath.Join(d.support, "summary-1")
	for file, text := range map[string]string{
		filepath.Join(lib, ".hg", "hgrc"):    "# the owner's",
		filepath.Join(summary, "prompt.txt"): "the summary's",
		filepath.Join(own, "draft"):          "gitdir: elsewhere\n",
		filepath.Join(own, "repo", "HEAD"):   "ref: refs/heads/main\n",
	} {
		mustWrite(t, file, text)
	}
	draft, repo := filepath.Join(own, "draft"), filepath.Join(own, "repo")
	rows := []escapeRow{
		// The positive controls: the desk's own work, the neighbours' names, and the owner's own read.
		{"control: its own folder writes", attempt{"create", filepath.Join(own, "new.txt"), ""}, "ok"},
		{"control: a neighbour of .git in ~/Library", attempt{"create", filepath.Join(lib, ".git-notes"), ""}, "ok"},
		{"control: .github in ~/Library/Application Support", attempt{"mkdir", filepath.Join(as, ".github"), ""}, "ok"},
		{"control: a neighbour of HEAD in ~/Library/Application Support", attempt{"create", filepath.Join(as, "HEADS-UP"), ""}, "ok"},
		{"control: ~/Library/.hg/hgrc read", attempt{"read", filepath.Join(lib, ".hg", "hgrc"), ""}, "ok"},

		// The owner's own, never written.
		{"~/Library/.hg/hgrc", attempt{"write", filepath.Join(lib, ".hg", "hgrc"), ""}, "EPERM"},
		{"~/Library/.hg moved aside", attempt{"rename", filepath.Join(lib, ".hg"), filepath.Join(lib, "old-hg")}, "EPERM"},
		// ~'s own: rowHomeGit's .git and HEAD, and .hg.
		{"~/.git, made", attempt{"mkdir", filepath.Join(d.home, ".git"), ""}, "EPERM"},
		{"~/HEAD, made", attempt{"create", filepath.Join(d.home, "HEAD"), ""}, "EPERM"},
		{"~/.hg, a folder", attempt{"mkdir", filepath.Join(d.home, ".hg"), ""}, "EPERM"},
		{"~/.hg, a file", attempt{"create", filepath.Join(d.home, ".hg"), ""}, "EPERM"},
		{"~/.hg, a link to the desk's", attempt{"symlink", filepath.Join(d.home, ".hg"), filepath.Join(own, "repo")}, "EPERM"},
		// The support folder and a summary's folder there.
		{"the support folder's .git, made", attempt{"mkdir", filepath.Join(d.support, ".git"), ""}, "EPERM"},
		{"the support folder's .hg, made", attempt{"mkdir", filepath.Join(d.support, ".hg"), ""}, "EPERM"},
		{"a summary's .git, made", attempt{"create", filepath.Join(summary, ".git"), ""}, "EPERM"},
		{"a summary's .hg, made", attempt{"mkdir", filepath.Join(summary, ".hg"), ""}, "EPERM"},
		{"a summary's HEAD, made", attempt{"create", filepath.Join(summary, "HEAD"), ""}, "EPERM"},
	}
	// The review's writes and the ones beside them, in each folder above the support folder below ~.
	for _, f := range []struct{ name, at string }{{"~/Library/Application Support", as}, {"~/Library", lib}} {
		for _, name := range []string{".git", ".hg"} {
			if f.at == lib && name == ".hg" {
				continue // the owner's, above
			}
			at := filepath.Join(f.at, name)
			rows = append(rows,
				escapeRow{f.name + "/" + name + ", a folder", attempt{"mkdir", at, ""}, "EPERM"},
				escapeRow{f.name + "/" + name + ", a file", attempt{"create", at, ""}, "EPERM"},
				escapeRow{f.name + "/" + name + ", a folder renamed onto it", attempt{"rename", repo, at}, "EPERM"},
				escapeRow{f.name + "/" + name + ", a file renamed onto it", attempt{"rename", draft, at}, "EPERM"},
				escapeRow{f.name + "/" + name + ", a link to the desk's", attempt{"symlink", at, repo}, "EPERM"},
				escapeRow{f.name + "/" + name + ", a hard link", attempt{"link", draft, at}, "EPERM"},
			)
		}
		rows = append(rows,
			escapeRow{f.name + "/HEAD, made", attempt{"create", filepath.Join(f.at, "HEAD"), ""}, "EPERM"},
			escapeRow{f.name + "/head, made", attempt{"create", filepath.Join(f.at, "head"), ""}, "EPERM"},
			escapeRow{f.name + "/.GIT, made", attempt{"mkdir", filepath.Join(f.at, ".GIT"), ""}, "EPERM"},
			escapeRow{f.name + "/HEAD, by a rename", attempt{"rename", draft, filepath.Join(f.at, "HEAD")}, "EPERM"},
		)
	}
	return rows
}

func TestOnAMacAHireCantPlantARepoWhereOpenCodeLooksForItsProject(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	rows := openCodeProjectRows(t, d, d.root)
	d.prepare(t)
	d.check(t, rows)
}

func TestOnAMacAGMCantPlantARepoWhereOpenCodeLooksForItsProject(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	if err := os.MkdirAll(filepath.Join(d.home, "Projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.pl.Subject, d.pl.GMFolder, d.pl.Projects = GM, "", filepath.Join(d.home, "Projects")
	d.pl.Worktrees = []string{filepath.Join(d.home, ".smolcorp", "worktrees")}
	rows := openCodeProjectRows(t, d, d.root)
	d.prepare(t)
	d.check(t, rows)
}
