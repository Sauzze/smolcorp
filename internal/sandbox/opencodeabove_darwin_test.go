package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

// FXRC3FIX (RT126RC3's B1), on this Mac with stand-ins only: a summary on OpenCode runs in a folder made in the support
// folder, as the owner and outside any sandbox, and OpenCode loads its config files, its own folder's plugins and the
// skills folder from that folder and every folder above it. Under the real profile, neither a hire nor a GM writes,
// makes or renames onto any of them in ~/Library/Application Support, ~/Library or ~; each still reads them. Text
// files only: nothing here runs OpenCode or what the files would name.

// openCodeAboveRows are the rows for a desk d: the review's writes, the ones beside them, and the controls.
func openCodeAboveRows(t *testing.T, d *escapeDesk, own string) []escapeRow {
	t.Helper()
	lib := filepath.Join(d.home, "Library")
	as := filepath.Join(lib, "Application Support")
	owners := map[string]string{
		filepath.Join(as, "opencode.json"):                       "{}",
		filepath.Join(as, ".opencode", "plugins", "owner.js"):    "// the owner's",
		filepath.Join(lib, ".agents", "skills", "s", "SKILL.md"): "the owner's",
		filepath.Join(d.home, "opencode.jsonc"):                  "{}",
		filepath.Join(own, "draft.json"):                         "{}",
	}
	for file, text := range owners {
		mustWrite(t, file, text)
	}
	return []escapeRow{
		// The positive controls: the desk's own work, a neighbour's name, and the owner's OpenCode config read.
		{"control: its own folder writes", attempt{"create", filepath.Join(own, "new.txt"), ""}, "ok"},
		{"control: a neighbour in ~/Library", attempt{"create", filepath.Join(lib, "opencode.json-notes"), ""}, "ok"},
		{"control: ~/Library/Application Support/opencode.json read", attempt{"read", filepath.Join(as, "opencode.json"), ""}, "ok"},
		{"control: a plugin there read", attempt{"read", filepath.Join(as, ".opencode", "plugins", "owner.js"), ""}, "ok"},
		{"control: ~/opencode.jsonc read", attempt{"read", filepath.Join(d.home, "opencode.jsonc"), ""}, "ok"},
		{"control: a skill in ~/Library/.agents read", attempt{"read", filepath.Join(lib, ".agents", "skills", "s", "SKILL.md"), ""}, "ok"},

		// The review's writes (TestZZRC3…WritesTheSummaryFoldersAncestors).
		{"~/Library/Application Support/opencode.json", attempt{"write", filepath.Join(as, "opencode.json"), ""}, "EPERM"},
		{"~/Library/Application Support/.opencode, made", attempt{"mkdir", filepath.Join(as, ".opencode", "plugin"), ""}, "EPERM"},
		{"~/Library/Application Support/.agents, made", attempt{"mkdir", filepath.Join(as, ".agents"), ""}, "EPERM"},
		{"~/Library/opencode.json, made", attempt{"create", filepath.Join(lib, "opencode.json"), ""}, "EPERM"},
		{"~/Library/.opencode, made", attempt{"mkdir", filepath.Join(lib, ".opencode"), ""}, "EPERM"},
		{"~/opencode.json, made", attempt{"create", filepath.Join(d.home, "opencode.json"), ""}, "EPERM"},
		{"~/opencode.jsonc", attempt{"write", filepath.Join(d.home, "opencode.jsonc"), ""}, "EPERM"},
		// Beside them: the other names in each folder, a plugin in a folder there, a rename onto, a link, a copy.
		{"~/Library/Application Support/opencode.jsonc, made", attempt{"create", filepath.Join(as, "opencode.jsonc"), ""}, "EPERM"},
		{"a plugin in ~/Library/Application Support/.opencode", attempt{"create", filepath.Join(as, ".opencode", "plugins", "x.js"), ""}, "EPERM"},
		{"~/Library/opencode.jsonc, made", attempt{"create", filepath.Join(lib, "opencode.jsonc"), ""}, "EPERM"},
		{"a skill in ~/Library/.agents", attempt{"create", filepath.Join(lib, ".agents", "skills", "s", "x.md"), ""}, "EPERM"},
		{"~/.opencode, made", attempt{"mkdir", filepath.Join(d.home, ".opencode"), ""}, "EPERM"},
		{"~/opencode.json, by a rename", attempt{"rename", filepath.Join(own, "draft.json"), filepath.Join(d.home, "opencode.json")}, "EPERM"},
		{"~/Library/opencode.json, by a rename", attempt{"rename", filepath.Join(own, "draft.json"), filepath.Join(lib, "opencode.json")}, "EPERM"},
		{"~/Library/Application Support/.opencode, a link", attempt{"symlink", filepath.Join(as, ".opencode", "plugins", "l.js"), filepath.Join(own, "draft.json")}, "EPERM"},
		{"~/Library/.opencode, a link to the desk's", attempt{"symlink", filepath.Join(lib, ".opencode"), own}, "EPERM"},
		{"a copy to rename onto ~/opencode.json", attempt{"create", filepath.Join(d.home, "opencode.json.tmp.1.2"), ""}, "EPERM"},
		{"~/Library/Application Support/.opencode moved aside", attempt{"rename", filepath.Join(as, ".opencode"), filepath.Join(as, "old-opencode")}, "EPERM"},
	}
}

func TestOnAMacAHireCantWriteOpenCodesConfigAboveTheSupportFolder(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	rows := openCodeAboveRows(t, d, d.root)
	d.prepare(t)
	d.check(t, rows)
}

func TestOnAMacAGMCantWriteOpenCodesConfigAboveTheSupportFolder(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	if err := os.MkdirAll(filepath.Join(d.home, "Projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.pl.Subject, d.pl.GMFolder, d.pl.Projects = GM, "", filepath.Join(d.home, "Projects")
	d.pl.Worktrees = []string{filepath.Join(d.home, ".smolcorp", "worktrees")}
	rows := openCodeAboveRows(t, d, d.root)
	d.prepare(t)
	d.check(t, rows)
}
