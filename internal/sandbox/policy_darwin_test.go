package sandbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SIERRA's row (14 Nov), on this Mac with stand-ins only (owner Q5 (a)): a hire's python and its Write tool can't
// move a base branch, rewrite packed-refs or change the shared config, and `mv`, perl's rename and python's os.rename
// can't move a folder above them aside (RT126SAX1 B1). The Policy's write rows for the test's own folder are written
// as a Seatbelt profile here, for the test only (M1 writes the real one), and a stand-in hire runs under
// /usr/bin/sandbox-exec: python3 writing each file, then a stand-in of Claude Code's Write tool, given the tool's
// input as Claude Code gives it, writing whole files through a temporary file and a rename, then each rename of a
// folder, checked only for "not permitted". Nothing here reaches outside the test's temp folder, and the repo is made
// by hand there: no git runs, and nothing is written into a moved folder.

// writeProfile is p's write rows inside folder as a Seatbelt profile, everything else allowed. Seatbelt lets the last
// match decide, so deny wins as in the Policy by the rows' order: the plain allows, the Wide denies, the exceptions
// that really open one (p.Exception, RT126SAX1R2 B2′), then the final denies. A Move row is `file-write-unlink` on a folder; a Link row is `file-link` and
// making a symlink there. A row with a Tail is a regex (tailFilter), as M1 might write it.
func writeProfile(t *testing.T, p *Policy, folder string) string {
	t.Helper()
	var plain, wide, exceptions, final []string
	for _, r := range p.Rules() {
		if r.Path == "" || !strings.HasPrefix(r.Path, folder+"/") {
			continue // the rows for anywhere are M1's; the stand-ins write nothing they hold
		}
		quoted, _ := json.Marshal(r.Path) // a Seatbelt string reads as a JSON one for these paths
		filter := fmt.Sprintf("(literal %s)", quoted)
		if r.Below {
			filter = fmt.Sprintf("(subpath %s)", quoted)
		}
		if r.Tail != "" {
			filter = tailFilter(t, r)
		}
		switch {
		case r.Write == Allow && p.Exception(r):
			exceptions = append(exceptions, "(allow file-write* "+filter+")")
		case r.Write == Allow:
			plain = append(plain, "(allow file-write* "+filter+")")
		case r.Write == Deny && r.Wide:
			wide = append(wide, "(deny file-write* "+filter+")")
		case r.Write == Deny:
			final = append(final, "(deny file-write* "+filter+")")
		}
		if r.Write == Unset && r.Move == Deny {
			final = append(final, "(deny file-write-unlink (require-all "+filter+" (vnode-type DIRECTORY)))")
		}
		if r.Write == Unset && r.Link == Deny {
			final = append(final, "(deny file-link "+filter+")", "(deny file-write-create (require-all "+filter+" (vnode-type SYMLINK)))")
		}
	}
	return "(version 1)\n(allow default)\n" + strings.Join(slices.Concat(plain, wide, exceptions, final), "\n") + "\n"
}

// tailFilter is a row with a Tail as a Seatbelt regex, the way RT126SAX1R3's stand-in wrote the pattern rows: `**`
// is any number of folders, `*` and `?` stay within one name, and Below adds everything under what matches. The
// stand-ins' paths are spelled as the rows are, so the regex needn't fold case.
func tailFilter(t *testing.T, r Rule) string {
	t.Helper()
	re := "^" + regexp.QuoteMeta(r.Path)
	for _, name := range strings.Split(r.Tail, "/") {
		if name == "**" {
			re += "(/[^/]+)*"
			continue
		}
		if strings.ContainsAny(name, `[\`) {
			t.Fatalf("a Tail the stand-in can't write: %q", r.Tail)
		}
		re += "/" + strings.NewReplacer(`\*`, "[^/]*", `\?`, "[^/]").Replace(regexp.QuoteMeta(name))
	}
	if r.Below {
		re += "(/.*)?"
	}
	return `(regex #"` + re + `$")`
}

// python3 is a python to run the stand-ins with, or "". /usr/bin/python3 is Apple's shim: with no developer tools it
// asks to install them, a system dialog, so it is used only when xcode-select names a developer folder.
func python3(t *testing.T) string {
	found, err := exec.LookPath("python3")
	if err != nil {
		return ""
	}
	if found == "/usr/bin/python3" {
		if err := disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run(); err != nil {
			t.Logf("no developer tools (%v): /usr/bin/python3 would ask to install them", err)
			return ""
		}
	}
	return found
}

const (
	// pythonWrite opens each path in its arguments for writing and says, per path, "wrote" or why not.
	pythonWrite = `import sys, json
said = {}
for p in sys.argv[1:]:
    try:
        with open(p, "w") as f:
            f.write("0000000000000000000000000000000000000000\n")
        said[p] = "wrote"
    except OSError as e:
        said[p] = type(e).__name__
print(json.dumps(said))`
	// writeTool is a stand-in of Claude Code's Write tool: the tool's input on stdin, the file written whole through
	// a temporary file beside it and a rename, as editors and git do.
	writeTool = `import sys, json, os
event = json.load(sys.stdin)
path, content = event["tool_input"]["file_path"], event["tool_input"]["content"]
tmp = os.path.join(os.path.dirname(path), "." + os.path.basename(path) + ".tmp-write")
try:
    with open(tmp, "w") as f:
        f.write(content)
    os.replace(tmp, path)
    print("wrote")
except OSError as e:
    try:
        os.remove(tmp)
    except OSError:
        pass
    print(type(e).__name__)`
)

func TestOnAMacAHiresPythonAndWriteToolCantMoveTheBaseBranches(t *testing.T) {
	sandboxExec, python, perl := "/usr/bin/sandbox-exec", python3(t), "/usr/bin/perl"
	_, noPerl := os.Stat(perl)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" || noPerl != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v), python3 (%q) or perl (%v) missing", err, python, noPerl)
		}
		t.Skip("no sandbox-exec, python3 or perl here")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	repo := filepath.Join(home, "Projects", "app")
	git := filepath.Join(repo, ".git")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	support := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	const old = "1111111111111111111111111111111111111111\n"
	files := map[string]string{
		"refs/heads/main": old, "refs/heads/master": old, "refs/heads/develop": old, "refs/heads/release/1.26": old,
		"refs/heads/feature": old, "packed-refs": old, "config": "[core]\n", "HEAD": "ref: refs/heads/main\n",
		"refs/remotes/origin/HEAD": "ref: refs/remotes/origin/develop\n", "refs/tags/v1": old, "hooks/README": "hooks\n",
	}
	for name, text := range files {
		at := filepath.Join(git, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{root, support, filepath.Join(git, "worktrees", "h1")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", DefaultBranch: DefaultBranch(git), HiresFolder: filepath.Dir(root)})
	profile := writeProfile(t, p, folder)

	ref := func(name string) string { return filepath.Join(git, filepath.FromSlash(name)) }
	denied := []string{ref("refs/heads/main"), ref("refs/heads/master"), ref("refs/heads/develop"), ref("refs/heads/release/1.26"),
		ref("refs/heads/release/1.27"), ref("packed-refs"), ref("config")}
	allowed := []string{ref("refs/heads/feature"), ref("refs/heads/hire-h1"), ref("worktrees/h1/HEAD"), filepath.Join(root, "main.go")}
	for _, at := range denied {
		if v, by := p.Decide(at, Write); v != Deny {
			t.Errorf("the Policy lets %s be written (%q)", at, by.Row)
		}
	}
	sandboxed := func(stdin []byte, args ...string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-p", profile}, args...)...))
		cmd.Dir, cmd.Stdin = folder, bytes.NewReader(stdin)
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err := cmd.Run()
		return strings.TrimSpace(out.String() + stderr.String()), err
	}
	run := func(stdin []byte, args ...string) string {
		out, err := sandboxed(stdin, append([]string{python, "-I"}, args...)...)
		if err != nil {
			t.Fatalf("the stand-in didn't run: %v %s", err, out)
		}
		return out
	}

	var said map[string]string
	if err := json.Unmarshal([]byte(run(nil, append([]string{"-c", pythonWrite}, append(denied, allowed...)...)...)), &said); err != nil {
		t.Fatal(err)
	}
	for _, at := range denied {
		if said[at] != "PermissionError" {
			t.Errorf("python: %s: %q, want PermissionError", at, said[at])
		}
	}
	for _, at := range allowed {
		if said[at] != "wrote" {
			t.Errorf("python: %s: %q, want wrote (the stand-in must still work)", at, said[at])
		}
	}
	for _, at := range append(denied, allowed[:1]...) {
		event, _ := json.Marshal(map[string]any{"tool_name": "Write", "tool_input": map[string]string{"file_path": at, "content": "2222\n"}})
		want := "PermissionError"
		if at == allowed[0] {
			want = "wrote"
		}
		if got := run(event, "-c", writeTool); got != want {
			t.Errorf("the Write tool: %s: %q, want %s", at, got, want)
		}
	}
	for name, text := range files {
		if data, _ := os.ReadFile(ref(name)); name != "refs/heads/feature" && string(data) != text {
			t.Errorf("%s changed: %q", name, data)
		}
	}
	if _, err := os.Lstat(ref("refs/heads/release/1.27")); err == nil {
		t.Error("a new release branch was made")
	}

	// B1: no folder above a denied place moves aside, by mv, perl's rename or python's os.rename; a folder above
	// nothing denied does, and is put back, so the stand-ins really run.
	parents := []string{ref("refs/heads"), ref("refs"), git, ref("hooks"), repo}
	for _, at := range parents {
		if v, by := p.Decide(at, Move); v != Deny {
			t.Errorf("the Policy lets %s be moved (%q)", at, by.Row)
		}
	}
	moves := map[string]func(from, to string) []string{
		"mv": func(from, to string) []string { return []string{"/bin/mv", from, to} },
		"perl rename": func(from, to string) []string {
			return []string{perl, "-e", `rename($ARGV[0], $ARGV[1]) or die "$!\n"`, from, to}
		},
		"python3 os.rename": func(from, to string) []string {
			return []string{python, "-I", "-c", "import os, sys; os.rename(sys.argv[1], sys.argv[2])", from, to}
		},
	}
	for how, move := range moves {
		for _, at := range parents {
			out, err := sandboxed(nil, move(at, at+"-aside")...)
			if err == nil || !strings.Contains(out, "not permitted") {
				t.Errorf("%s %s: %v %q, want not permitted", how, at, err, out)
			}
			if _, err := os.Lstat(at + "-aside"); err == nil {
				t.Fatalf("%s moved %s aside", how, at)
			}
		}
		tags := ref("refs/tags")
		if out, err := sandboxed(nil, move(tags, tags+"-aside")...); err != nil {
			t.Errorf("%s refs/tags, a folder above nothing denied: %v %q (the stand-in must still work)", how, err, out)
		} else if out, err := sandboxed(nil, move(tags+"-aside", tags)...); err != nil {
			t.Fatalf("%s didn't put refs/tags back: %v %q", how, err, out)
		}
	}
	for name, text := range files {
		if data, _ := os.ReadFile(ref(name)); name != "refs/heads/feature" && string(data) != text {
			t.Errorf("%s changed after the moves: %q", name, data)
		}
	}
}

// B1′ (RT126SAX1R2), on this Mac with stand-ins only: a folder above a denied place that doesn't exist yet
// (`~/.codex`, `~/.config`, `~/.config/systemd`, `~/.ssh`) is made by the core first (MakeHold), so a folder made
// elsewhere with a `standin` file where a denied one would be can't be renamed onto it, by python's os.rename or perl's
// rename, nor a symlink made there. A hold folder left missing (`~/.gemini`) still takes no link, by the profile's own
// Link row, and a missing `.git/modules` takes neither, denied writing whole (RT126SAX1R4 B1‴). Each check is only that the denied path doesn't exist afterwards. A rename onto a missing folder above nothing
// denied works, so the stand-ins really run.
func TestOnAMacAMissingFolderAboveADeniedPlaceCantBeBroughtInWhole(t *testing.T) {
	sandboxExec, python, perl := "/usr/bin/sandbox-exec", python3(t), "/usr/bin/perl"
	_, noPerl := os.Stat(perl)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" || noPerl != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v), python3 (%q) or perl (%v) missing", err, python, noPerl)
		}
		t.Skip("no sandbox-exec, python3 or perl here")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	repo := filepath.Join(home, "Projects", "app")
	git := filepath.Join(repo, ".git")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	support := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	staging := filepath.Join(root, "staging")
	for _, dir := range []string{filepath.Join(git, "refs", "heads"), root, support, staging} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", HiresFolder: filepath.Dir(root), RemoteLogin: true})
	profile := writeProfile(t, p, folder)
	sandboxed := func(args ...string) {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-p", profile}, args...)...))
		cmd.Dir, cmd.Env = folder, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
		out, err := cmd.CombinedOutput()
		t.Logf("%s: %v %s", filepath.Base(args[0]), err, bytes.TrimSpace(out)) // only the paths below are checked
	}
	rename := map[string]func(from, to string) []string{
		"python3 os.rename": func(from, to string) []string {
			return []string{python, "-I", "-c", "import os, sys; os.rename(sys.argv[1], sys.argv[2])", from, to}
		},
		"perl rename": func(from, to string) []string {
			return []string{perl, "-e", `rename($ARGV[0], $ARGV[1]) or die "$!\n"`, from, to}
		},
	}
	symlink := func(from, to string) []string {
		return []string{python, "-I", "-c", "import os, sys; os.symlink(sys.argv[1], sys.argv[2])", from, to}
	}
	// staged makes a folder in staging with a standin file at inside, and gives the folder.
	n := 0
	staged := func(inside string) string {
		n++
		dir := filepath.Join(staging, fmt.Sprint(n))
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, inside)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, inside), []byte("standin\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	absent := func(how, at string) {
		if _, err := os.Lstat(at); !os.IsNotExist(err) {
			t.Errorf("%s: %s exists afterwards (%v)", how, at, err)
		}
	}

	// The stand-ins run: a missing folder above nothing denied is brought in whole.
	for how, move := range rename {
		at := filepath.Join(home, ".not-held-"+strings.Fields(how)[0])
		sandboxed(move(staged("standin"), at)...)
		if _, err := os.Lstat(filepath.Join(at, "standin")); err != nil {
			t.Fatalf("%s onto %s, a folder above nothing denied: %v (the stand-in must still work)", how, at, err)
		}
	}

	var hold []string
	for _, at := range p.Hold() {
		if strings.HasPrefix(at, folder+"/") && at != filepath.Join(home, ".gemini") {
			hold = append(hold, at)
		}
	}
	codex, config, systemd, ssh, modules := filepath.Join(home, ".codex"), filepath.Join(home, ".config"),
		filepath.Join(home, ".config", "systemd"), filepath.Join(home, ".ssh"), filepath.Join(git, "modules")
	for _, at := range []string{codex, config, systemd, ssh, modules} {
		if inHold := slices.Contains(hold, at); inHold == (at == modules) {
			t.Fatalf("%s: in Hold %v: %q", at, inHold, hold)
		}
		if _, err := os.Lstat(at); !os.IsNotExist(err) {
			t.Fatalf("%s exists before the start: %v", at, err)
		}
	}
	if v, by := p.Decide(modules, Write); v != Deny {
		t.Fatalf("%s is writable (%q)", modules, by.Row)
	}
	if err := MakeHold(hold); err != nil {
		t.Fatal(err)
	}

	// Renamed onto a hold folder the core made, or onto the missing .git/modules: nothing arrives.
	for how, move := range rename {
		for at, inside := range map[string]string{codex: "auth.json", config: "git/config", systemd: "user/x.service", ssh: "rc",
			modules: "sub/config"} {
			sandboxed(move(staged(inside), at)...)
			absent(how+" onto "+at, filepath.Join(at, inside))
		}
	}
	// A symlink there: nothing arrives either.
	for at, inside := range map[string]string{ssh: "authorized_keys", systemd: "user/x.service", modules: "sub/hooks/post-checkout"} {
		sandboxed(symlink(staged(inside), at)...)
		absent("a symlink at "+at, filepath.Join(at, inside))
	}
	// A hold folder left missing takes no link, by the profile's Link row alone.
	gemini := filepath.Join(home, ".gemini")
	sandboxed(symlink(staged("oauth_creds.json"), gemini)...)
	absent("a symlink at the missing "+gemini, gemini)
}

// B1‴ (RT126SAX1R4), on this Mac with stand-ins only: no desk writes in .git/modules, a desk in its own worktree or at
// the project. The owner's side holds the git folders `sub` and `third_party/bar`, each with a HEAD and a config. A
// folder made elsewhere with `standin` files where denied ones would be is renamed, by python's os.rename and perl's
// rename, onto `sub/evil` (inside a git folder: the reviewer's, with config, hooks/post-checkout and HEAD),
// `third_party/bar/evil`, `sub/modules` (a nested submodules' folder), `third_party/evil` and `new`; `mkdir` makes none
// of them; and the write to `sub/HEAD` that let git take `sub/evil` for a submodule's git folder, a write in
// `sub/objects` and one to `sub/config` are refused. The pattern rows are written as regexes. Each check is only a
// file's existence, or that the owner's file is unchanged. The desk in a worktree still writes its own submodules' git
// folders, in its registration, and renames a folder into one; the desk at the project its own files: so the stand-ins
// really run.
func TestOnAMacNothingInGitModulesIsWrittenNorBroughtIn(t *testing.T) {
	sandboxExec, python, perl := "/usr/bin/sandbox-exec", python3(t), "/usr/bin/perl"
	_, noPerl := os.Stat(perl)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" || noPerl != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v), python3 (%q) or perl (%v) missing", err, python, noPerl)
		}
		t.Skip("no sandbox-exec, python3 or perl here")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	repo := filepath.Join(home, "Projects", "app")
	git := filepath.Join(repo, ".git")
	modules := filepath.Join(git, "modules")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	support := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	own := filepath.Join(git, "worktrees", "h1", "modules", "sub")
	const config, head = "[core]\n\tstandin = owner\n", "ref: refs/heads/main\n"
	for _, at := range []string{filepath.Join(modules, "sub"), filepath.Join(modules, "third_party", "bar"), own} {
		if err := os.MkdirAll(filepath.Join(at, "objects"), 0o700); err != nil {
			t.Fatal(err)
		}
		for name, text := range map[string]string{"HEAD": head, "config": config} {
			if err := os.WriteFile(filepath.Join(at, name), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, dir := range []string{filepath.Join(git, "refs", "heads"), root, support, filepath.Join(home, "staging")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	py := func(code string, args ...string) []string { return append([]string{python, "-I", "-c", code}, args...) }
	rename := map[string]func(from, to string) []string{
		"python3 os.rename": func(from, to string) []string {
			return py("import os, sys; os.rename(sys.argv[1], sys.argv[2])", from, to)
		},
		"perl rename": func(from, to string) []string {
			return []string{perl, "-e", `rename($ARGV[0], $ARGV[1]) or die "$!\n"`, from, to}
		},
	}
	write := func(at string) []string { return py("import sys; open(sys.argv[1], 'w').write('standin\\n')", at) }
	mkdir := func(at string) []string { return py("import os, sys; os.mkdir(sys.argv[1])", at) }
	exists := func(how, at string, want bool) {
		if _, err := os.Lstat(at); (err == nil) != want {
			t.Errorf("%s: %s exists %v, want %v (%v)", how, at, err == nil, want, err)
		}
	}
	unchanged := func(how, at, want string) {
		if data, _ := os.ReadFile(at); string(data) != want {
			t.Errorf("%s: the owner's %s is now %q", how, at, data)
		}
	}
	in := func(rel string) string { return filepath.Join(modules, filepath.FromSlash(rel)) }

	for _, desk := range []struct {
		name, root, checkout, worktree, staging string
	}{
		{"a desk in its own worktree", root, repo, "h1", filepath.Join(root, "staging")},
		{"a desk at the project", repo, "", "", filepath.Join(repo, "staging")},
	} {
		p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: desk.root, Checkout: desk.checkout,
			GitDir: git, OwnWorktree: desk.worktree, HiresFolder: filepath.Dir(root)})
		if err := MakeHold(p.Hold()); err != nil {
			t.Fatal(err)
		}
		profile := writeProfile(t, p, folder)
		sandboxed := func(args ...string) {
			cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-p", profile}, args...)...))
			cmd.Dir, cmd.Env = folder, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
			out, err := cmd.CombinedOutput()
			t.Logf("%s: %s: %v %s", desk.name, filepath.Base(args[0]), err, bytes.TrimSpace(out)) // only the paths below are checked
		}
		n := 0
		staged := func(inside ...string) string {
			n++
			dir := filepath.Join(desk.staging, fmt.Sprint(n))
			for _, name := range inside {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("standin\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			return dir
		}

		// The reviewers' probes: the Policy denies each place, and the kernel brings nothing there.
		for _, at := range []string{"sub/evil", "sub/evil/config", "sub/evil/hooks/post-checkout", "third_party/bar/evil", "sub/HEAD",
			"sub/refs/heads/x/config", "sub/objects/standin", "sub/modules", "sub/modules/x/config", "third_party/evil",
			"third_party/evil/config", "new", "sub/config"} {
			if v, by := p.Decide(in(at), Write); v != Deny {
				t.Errorf("%s: write %s: %v by %q", desk.name, in(at), v, by.Row)
			}
		}
		sandboxed(write(in("sub/HEAD"))...)
		unchanged(desk.name+": python write", in("sub/HEAD"), head)
		sandboxed(write(in("sub/config"))...)
		unchanged(desk.name+": python write", in("sub/config"), config)
		sandboxed(write(in("sub/objects/standin"))...)
		exists(desk.name+": python write", in("sub/objects/standin"), false)
		for _, at := range []string{"new", "sub/evil", "sub/modules", "third_party/evil", "third_party/bar/evil"} {
			sandboxed(mkdir(in(at))...)
			exists(desk.name+": mkdir", in(at), false)
		}
		for how, move := range rename {
			how = desk.name + ": " + how
			sandboxed(move(staged("config", "hooks/post-checkout", "HEAD"), in("sub/evil"))...)
			exists(how+" onto sub/evil", in("sub/evil"), false)
			sandboxed(move(staged("config"), in("third_party/bar/evil"))...)
			exists(how+" onto third_party/bar/evil", in("third_party/bar/evil"), false)
			sandboxed(move(staged("x/config"), in("sub/modules"))...)
			exists(how+" onto sub/modules", in("sub/modules"), false)
			sandboxed(move(staged("config"), in("third_party/evil"))...)
			exists(how+" onto third_party/evil", in("third_party/evil"), false)
			sandboxed(move(staged("HEAD"), in("new"))...)
			exists(how+" onto new", in("new"), false)
		}

		// The stand-ins run: the desk in a worktree writes its own submodule's git folder, in its registration, and
		// renames a folder into it; the desk at the project writes its files.
		if desk.worktree == "" {
			sandboxed(write(filepath.Join(repo, "main.go"))...)
			exists(desk.name+": python write", filepath.Join(repo, "main.go"), true)
			continue
		}
		sandboxed(write(filepath.Join(own, "objects", "standin"))...)
		exists(desk.name+": python write", filepath.Join(own, "objects", "standin"), true)
		for how, move := range rename {
			at := filepath.Join(own, "objects", "pack-"+strings.Fields(how)[0])
			sandboxed(move(staged("standin"), at)...)
			exists(desk.name+": "+how+" into its own submodule's objects", filepath.Join(at, "standin"), true)
		}
	}
}

// gitForStandins is a git to run on the stand-ins with, or "". /usr/bin/git is Apple's shim: with no developer tools it
// asks to install them, a system dialog, so it is used only when xcode-select names a developer folder.
func gitForStandins(t *testing.T) string {
	found, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	if found == "/usr/bin/git" {
		if err := disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run(); err != nil {
			t.Logf("no developer tools (%v): /usr/bin/git would ask to install them", err)
			return ""
		}
	}
	return found
}

// SIERRA's sweep (14 Nov), on this Mac with stand-ins only: a hire can't change where git finds the repo's config,
// hooks and objects. A real git makes a stand-in repo, its submodule from a local folder, and the hire's worktree,
// all in the test's temp folder, with a fake home, no system or global config, no hooks, and `standin.who = owner` in
// the repo's config. Under the hire's profile, python writes each file git reads to find them: the shared .git's
// commondir and gitdir, the worktree registration's commondir, gitdir and config.worktree, the worktree's .git file,
// each naming a stand-in git folder whose config says `elsewhere`, the lists of other object stores (alternates,
// http-alternates, and info/alternates should a git read it), a stand-in of git's system config (GitConfigs), and the
// owner's rebase todo lists; and a registration is brought in whole by a rename.
// Each is refused, and git, in the repo and in the worktree, still reads `owner`. The same write to commondir,
// outside the sandbox, on a copy, makes git read `elsewhere`, so the check can tell. Then RT126SAX1R5's B3: with a
// `~/.git` and a bare `~/Projects` planted, no desk's chmod or ACL makes the shared .git stop being a git folder (below).
// Then the cost the plan names: the hire still runs `git submodule update --init`, commits and rebases in its worktree
// (the submodule's git folder goes in its own registration, and .git/modules is left as it was), and a desk at the
// project can't commit in a submodule.
func TestOnAMacAHireCantMoveWhereGitFindsTheSharedGit(t *testing.T) {
	sandboxExec, python, gitPath := "/usr/bin/sandbox-exec", python3(t), gitForStandins(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" || gitPath == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v), python3 (%q) or git (%q) missing", err, python, gitPath)
		}
		t.Skip("no sandbox-exec, python3 or git here")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	projects := filepath.Join(home, "Projects")
	repo, lib, elsewhere := filepath.Join(projects, "app"), filepath.Join(projects, "lib"), filepath.Join(folder, "elsewhere")
	git := filepath.Join(repo, ".git")
	root := filepath.Join(projects, "app-hires", "h1")
	support := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	for _, dir := range []string{support, filepath.Dir(root)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"HOME=" + home, "PATH=" + filepath.Dir(gitPath) + ":/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CEILING_DIRECTORIES=" + folder, "PYTHONDONTWRITEBYTECODE=1"}
	gitArgs := func(dir string, args ...string) []string {
		return append([]string{gitPath, "-C", dir, "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false",
			"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main",
			"-c", "user.name=standin", "-c", "user.email=standin@example.invalid"}, args...)
	}
	run := func(args []string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(args[0], args[1:]...))
		cmd.Dir, cmd.Env = folder, env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	gitOK := func(dir string, args ...string) string {
		t.Helper()
		out, err := run(gitArgs(dir, args...))
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return out
	}
	writeFile := func(at, text string) {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{lib, repo} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		gitOK(dir, "init", "-q")
		writeFile(filepath.Join(dir, "standin.txt"), "standin\n")
		gitOK(dir, "add", "standin.txt")
		gitOK(dir, "commit", "-q", "-m", "standin")
	}
	gitOK(repo, "submodule", "add", "-q", "file://"+lib, "lib")
	gitOK(repo, "commit", "-q", "-m", "standin submodule")
	gitOK(repo, "config", "standin.who", "owner")
	gitOK(repo, "worktree", "add", "-q", "-b", "hire/h1", root)
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	gitOK(elsewhere, "init", "-q", "--bare")
	gitOK(elsewhere, "config", "standin.who", "elsewhere")
	who := func(dir string) string { return gitOK(dir, "config", "standin.who") }
	if got := who(root); got != "owner" {
		t.Fatalf("the worktree's git reads %q before anything", got)
	}

	// The check can tell: on a copy of the shared .git, a commondir naming elsewhere moves its whole config.
	copied := filepath.Join(folder, "copy")
	if out, err := run([]string{"/bin/cp", "-R", repo, copied}); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
	writeFile(filepath.Join(copied, ".git", "commondir"), elsewhere+"\n")
	if got := who(copied); got != "elsewhere" {
		t.Fatalf("a commondir written outside the sandbox: git reads %q, want elsewhere (the check can't tell)", got)
	}

	var others []string
	if entries, err := os.ReadDir(filepath.Join(git, "worktrees")); err == nil {
		for _, entry := range entries {
			others = append(others, entry.Name())
		}
	}
	systemConfig := filepath.Join(folder, "prefix", "etc", "gitconfig") // a stand-in for Homebrew's
	writeFile(systemConfig, "[standin]\n\twho = system\n")
	hire := Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", OtherWorktrees: others, DefaultBranch: DefaultBranch(git), HiresFolder: filepath.Dir(root),
		GitConfigs: []string{systemConfig}}
	p := mustNew(t, hire)
	if err := MakeHold(p.Hold()); err != nil {
		t.Fatal(err)
	}
	profile := writeProfile(t, p, folder)
	sandboxedAs := func(profile string, stdin []byte, args ...string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-p", profile}, args...)...))
		cmd.Dir, cmd.Env, cmd.Stdin = folder, env, bytes.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	sandboxed := func(stdin []byte, args ...string) (string, error) { return sandboxedAs(profile, stdin, args...) }

	// The owner, mid-rebase in the checkout: the todo lists a desk would add an exec line to (RT126SAX1R5).
	ownerRebase := map[string]string{
		filepath.Join(git, "rebase-merge", "git-rebase-todo"): "pick 0000000 standin\n",
		filepath.Join(git, "rebase-apply", "next"):            "1\n",
		filepath.Join(git, "sequencer", "todo"):               "pick 0000000 standin\n",
	}
	for at, text := range ownerRebase {
		writeFile(at, text)
	}
	registration := filepath.Join(git, "worktrees", "h1")
	attempts := map[string]string{
		filepath.Join(git, "commondir"):                          elsewhere + "\n",
		filepath.Join(git, "gitdir"):                             elsewhere + "\n",
		filepath.Join(registration, "commondir"):                 elsewhere + "\n",
		filepath.Join(registration, "gitdir"):                    filepath.Join(elsewhere, ".git") + "\n",
		filepath.Join(registration, "config.worktree"):           "[standin]\n\twho = elsewhere\n",
		filepath.Join(root, ".git"):                              "gitdir: " + elsewhere + "\n",
		systemConfig:                                             "[core]\n\thooksPath = " + elsewhere + "\n",
		filepath.Join(git, "objects", "info", "alternates"):      filepath.Join(elsewhere, "objects") + "\n",
		filepath.Join(git, "objects", "info", "http-alternates"): "file://" + filepath.Join(elsewhere, "objects") + "\n",
		filepath.Join(git, "info", "alternates"):                 filepath.Join(elsewhere, "objects") + "\n",
	}
	for at := range ownerRebase {
		attempts[at] = "exec standin\n"
	}
	before := map[string]string{}
	var paths []string
	for at := range attempts {
		if v, by := p.Decide(at, Write); v != Deny {
			t.Errorf("the Policy lets %s be written (%q)", at, by.Row)
		}
		data, err := os.ReadFile(at)
		if err != nil {
			data = nil
		}
		before[at] = string(data)
		paths = append(paths, at)
	}
	writeEach := `import sys, json
said = {}
for p, text in zip(sys.argv[1::2], sys.argv[2::2]):
    try:
        with open(p, "w") as f:
            f.write(text)
        said[p] = "wrote"
    except OSError as e:
        said[p] = type(e).__name__
print(json.dumps(said))`
	args := []string{python, "-I", "-c", writeEach}
	for _, at := range paths {
		args = append(args, at, attempts[at])
	}
	allowed := filepath.Join(root, "main.go")
	args = append(args, allowed, "standin\n")
	out, err := sandboxed(nil, args...)
	if err != nil {
		t.Fatalf("the stand-in didn't run: %v %s", err, out)
	}
	var said map[string]string
	if err := json.Unmarshal([]byte(out), &said); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, at := range paths {
		if said[at] != "PermissionError" {
			t.Errorf("python: %s: %q, want PermissionError", at, said[at])
		}
		if data, err := os.ReadFile(at); (err == nil) != (before[at] != "") || string(data) != before[at] {
			t.Errorf("%s changed: %q (%v), was %q", at, data, err, before[at])
		}
	}
	if said[allowed] != "wrote" {
		t.Errorf("python: %s: %q, want wrote (the stand-in must still work)", allowed, said[allowed])
	}
	// A registration brought in whole, its commondir inside.
	staged := filepath.Join(root, "staging", "h9")
	writeFile(filepath.Join(staged, "commondir"), elsewhere+"\n")
	writeFile(filepath.Join(staged, "HEAD"), "ref: refs/heads/main\n")
	brought := filepath.Join(git, "worktrees", "h9")
	out, err = sandboxed(nil, python, "-I", "-c", "import os, sys; os.rename(sys.argv[1], sys.argv[2])", staged, brought)
	if _, statErr := os.Lstat(brought); statErr == nil {
		t.Errorf("python os.rename brought %s in whole: %v %s", brought, err, out)
	}
	for _, dir := range []string{repo, root} {
		if got := who(dir); got != "owner" {
			t.Errorf("git in %s reads standin.who %q, want owner", dir, got)
		}
	}
	for at := range ownerRebase {
		if err := os.RemoveAll(filepath.Dir(at)); err != nil {
			t.Fatal(err)
		}
	}

	// B3 (RT126SAX1R5): git takes a folder for a git folder only while its objects and refs can be searched, and walks
	// past one that can't to the first git folder above: `<level>/.git`, or the level itself as a bare repo. A desk may
	// make both, a `~/.git` and a bare repo in `~/Projects` (made here outside the sandbox). Outside the sandbox, a mode
	// on refs makes git read the bare repo's config, and one on the bare repo's refs too then `~/.git`'s, so the check
	// can tell. Under each desk's profile (a hire, a test copy's hire, a desk at the project), python's chmod and an ACL
	// on the shared .git, its objects and its refs are refused, and git still reads owner.
	inHome, bare := filepath.Join(home, ".git"), projects
	for at, says := range map[string]string{inHome: "planted-in-home", bare: "planted-bare-in-projects"} {
		if v, by := p.Decide(filepath.Join(at, "config"), Write); v != Allow {
			t.Errorf("a desk may not make %s (%v by %q): the probe's premise has changed", at, v, by.Row)
		}
		for _, sub := range []string{"objects", "refs"} {
			if err := os.MkdirAll(filepath.Join(at, sub), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(filepath.Join(at, "HEAD"), "ref: refs/heads/main\n")
		writeFile(filepath.Join(at, "config"), "[standin]\n\twho = "+says+"\n")
	}
	entries := []string{git, filepath.Join(git, "objects"), filepath.Join(git, "refs")}
	restore := func() {
		for _, at := range append(entries, filepath.Join(bare, "refs")) {
			_ = os.Chmod(at, 0o755)
			_ = disclaim.Cmd(exec.Command("/bin/chmod", "-N", at)).Run()
		}
	}
	t.Cleanup(restore)
	if err := os.Chmod(filepath.Join(git, "refs"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := who(repo); got != "planted-bare-in-projects" {
		t.Fatalf("outside the sandbox, with .git/refs unsearchable: git reads %q, want the bare repo's (the check can't tell)", got)
	}
	if err := os.Chmod(filepath.Join(bare, "refs"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := who(repo); got != "planted-in-home" {
		t.Fatalf("outside the sandbox, with the bare repo's refs unsearchable too: git reads %q, want ~/.git's", got)
	}
	restore()
	testCopy := filepath.Join(projects, "smolcorp", ".dev", "support")
	desks := map[string]Places{
		"a hire": hire,
		"a test copy's hire": {GOOS: "darwin", Home: home, Support: testCopy, OtherSupports: []string{support}, OwnID: "h1", Root: root,
			Checkout: repo, GitDir: git, OwnWorktree: "h1", OtherWorktrees: others, DefaultBranch: DefaultBranch(git), HiresFolder: filepath.Dir(root)},
		"a desk at the project": {GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: repo, GitDir: git,
			OtherWorktrees: others, DefaultBranch: DefaultBranch(git), HiresFolder: filepath.Dir(root)},
	}
	for name, pl := range desks {
		desk := mustNew(t, pl)
		if err := MakeHold(desk.Hold()); err != nil {
			t.Fatal(err)
		}
		deskProfile := writeProfile(t, desk, folder)
		for _, at := range entries {
			for _, change := range [][]string{{python, "-I", "-c", "import os, sys; os.chmod(sys.argv[1], 0o600)", at},
				{"/bin/chmod", "+a", "everyone deny search", at}} {
				out, err := sandboxedAs(deskProfile, nil, change...)
				if err == nil || !strings.Contains(out, "not permitted") {
					t.Errorf("%s: %s %s: %v %q, want not permitted", name, filepath.Base(change[0]), at, err, out)
				}
				if got := who(repo); got != "owner" {
					t.Errorf("%s: after %s %s, git in the checkout reads standin.who %q, want owner", name, filepath.Base(change[0]), at, got)
				}
				restore()
			}
		}
	}
	for _, at := range []string{inHome, filepath.Join(bare, "HEAD"), filepath.Join(bare, "config"), filepath.Join(bare, "objects"),
		filepath.Join(bare, "refs")} {
		if err := os.RemoveAll(at); err != nil {
			t.Fatal(err)
		}
	}

	// The cost: none for the hire's submodules, which git keeps in its registration, its commits or its rebase...
	modulesBefore, _ := run([]string{"/usr/bin/find", filepath.Join(git, "modules")})
	if out, err := sandboxed(nil, gitArgs(root, "submodule", "update", "--init")...); err != nil {
		t.Errorf("git submodule update --init in the hire's worktree: %v %s", err, out)
	}
	if out, err := sandboxed(nil, gitArgs(root, "commit", "-q", "--allow-empty", "-m", "standin")...); err != nil {
		t.Errorf("git commit in the hire's worktree: %v %s", err, out)
	}
	if out, err := sandboxed(nil, gitArgs(root, "rebase", "-q", "--exec", "true", "HEAD~1")...); err != nil {
		t.Errorf("git rebase in the hire's worktree: %v %s", err, out)
	}
	exists := func(at string) bool { _, err := os.Lstat(at); return err == nil }
	if !exists(filepath.Join(registration, "modules", "lib", "HEAD")) || !exists(filepath.Join(root, "lib", "standin.txt")) {
		t.Errorf("the hire's submodule isn't in its registration nor checked out")
	}
	if modulesAfter, _ := run([]string{"/usr/bin/find", filepath.Join(git, "modules")}); modulesAfter != modulesBefore {
		t.Errorf(".git/modules changed:\n%s\nwas\n%s", modulesAfter, modulesBefore)
	}
	// ...and a desk at the project can't commit in one: that writes .git/modules.
	profile = writeProfile(t, mustNew(t, desks["a desk at the project"]), folder)
	headBefore := gitOK(filepath.Join(repo, "lib"), "rev-parse", "HEAD")
	if out, err := sandboxed(nil, gitArgs(filepath.Join(repo, "lib"), "commit", "-q", "--allow-empty", "-m", "standin")...); err == nil {
		t.Errorf("a desk at the project committed in a submodule: %s", out)
	}
	if got := gitOK(filepath.Join(repo, "lib"), "rev-parse", "HEAD"); got != headBefore {
		t.Errorf("the submodule's HEAD moved: %s, was %s", got, headBefore)
	}
	own := filepath.Join(repo, "main.go")
	if out, err := sandboxed(nil, python, "-I", "-c", "import sys; open(sys.argv[1], 'w').write('standin\\n')", own); err != nil || !exists(own) {
		t.Errorf("a desk at the project can't write its own file: %v %s (the stand-in must still work)", err, out)
	}
}
