package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// GITCFG1 (SIERRA, 14 Nov): what the owner's git config names for git to run or read as config, read at a desk's
// start. The reader runs a real git on stand-ins only: a repo, a worktree and a fake home in the test's temp folder,
// no system config but a stand-in's, and a `standin.who` value as the only thing ever read back. No git the tests run
// runs a hook or an fsmonitor: each has `core.hooksPath=/dev/null` and `core.fsmonitor=false`, and the reader runs
// only `git config --list`.

// gitForStandins is a git to run on the stand-ins with, or "". On a Mac /usr/bin/git is Apple's shim: with no
// developer tools it asks to install them, a system dialog, so it is used only when xcode-select names a developer
// folder.
func gitForStandins(t *testing.T) string {
	found, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	if runtime.GOOS == "darwin" && found == "/usr/bin/git" {
		if err := disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run(); err != nil {
			t.Logf("no developer tools (%v): /usr/bin/git would ask to install them", err)
			return ""
		}
	}
	return found
}

// gitStandin is a stand-in repo with the hire's worktree, in a fake home in the test's temp folder.
type gitStandin struct {
	t                                 *testing.T
	git, folder, home, repo, root, gd string
	env                               []string // for the test's own git runs: no system config
}

func newGitStandin(t *testing.T) *gitStandin {
	t.Helper()
	gitPath := gitForStandins(t)
	if gitPath == "" || runtime.GOOS == "windows" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" && runtime.GOOS != "windows" {
			t.Fatal("no git here")
		}
		t.Skip("no git here to run on stand-ins (or Windows, whose git spells paths its own way)")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	s := &gitStandin{t: t, git: gitPath, folder: folder, home: home, repo: filepath.Join(home, "Projects", "app"),
		root: filepath.Join(home, "Projects", "app-hires", "h1")}
	s.gd = filepath.Join(s.repo, ".git")
	s.env = []string{"HOME=" + home, "PATH=" + filepath.Dir(gitPath) + ":/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1",
		"XDG_CONFIG_HOME=" + filepath.Join(folder, "xdg"), "GIT_CEILING_DIRECTORIES=" + folder, "PYTHONDONTWRITEBYTECODE=1"}
	for _, dir := range []string{s.repo, filepath.Dir(s.root)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	s.ok(s.repo, "init", "-q")
	s.ok(s.repo, "commit", "-q", "--allow-empty", "-m", "standin")
	s.ok(s.repo, "worktree", "add", "-q", "-b", "hire/h1", s.root)
	return s
}

func (s *gitStandin) args(dir string, args ...string) []string {
	return append([]string{s.git, "-C", dir, "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main", "-c", "user.name=standin",
		"-c", "user.email=standin@example.invalid"}, args...)
}

func (s *gitStandin) run(args []string) (string, error) {
	cmd := disclaim.Cmd(exec.Command(args[0], args[1:]...))
	cmd.Dir, cmd.Env = s.folder, s.env
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (s *gitStandin) ok(dir string, args ...string) string {
	s.t.Helper()
	out, err := s.run(s.args(dir, args...))
	if err != nil {
		s.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return out
}

func (s *gitStandin) write(at, text string) {
	s.t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
		s.t.Fatal(err)
	}
}

// places is the hire's Places in the stand-in.
func (s *gitStandin) places() Places {
	return Places{Home: s.home, Support: filepath.Join(s.home, "Library", "Application Support", "AgentDeck"), OwnID: "h1",
		Root: s.root, Checkout: s.repo, GitDir: s.gd, OwnWorktree: "h1", HiresFolder: filepath.Dir(s.root)}
}

// Every place the owner's config names, in every file git reads: the global one (its includes, an includeIf whose
// condition is false here, an include of an include, a cycle behind a false condition, a link to a file that isn't
// there, a hooks folder holding links), the XDG one, a stand-in system config (GitConfigs), the repo's (husky's relative
// hooks folder, an fsmonitor), the desk's registration's config.worktree, and the command line's (GIT_CONFIG_*). A file
// that isn't there is named, and not read. Husky's `.husky` above its `_` is a hooks folder, and its init.sh named
// (RT126GITCFG1 B1); a link in a hooks folder has its target named, a folder as a hooks folder, at any depth (B2).
func TestTheOwnersGitConfigNamesItsHooksIncludesAndCommands(t *testing.T) {
	s := newGitStandin(t)
	home, folder := s.home, s.folder
	s.ok(s.repo, "config", "core.hooksPath", ".husky/_")
	s.ok(s.repo, "config", "core.fsmonitor", "tools/fsmon.sh")
	s.ok(s.repo, "config", "extensions.worktreeConfig", "true")
	s.ok(s.root, "config", "--worktree", "include.path", "wt-inc")
	s.write(filepath.Join(home, ".gitconfig"), "[include]\n\tpath = inc/one\n\tpath = ~/linked\n"+
		"[includeIf \"gitdir:/nowhere/\"]\n\tpath = ~/cond\n"+
		"[core]\n\tsshCommand = ssh -F ~/ssh/config -o UserKnownHostsFile=/dev/null -o \\\"ProxyCommand=/opt/standin/nc %h %p\\\"\n"+
		"\thooksPath = ~/.githooks\n")
	// stow-style links in the global hooks folder: a hook to a file, a folder to one holding a link to a missing file.
	dotfiles := filepath.Join(folder, "dotfiles")
	s.write(filepath.Join(dotfiles, "githooks", "post-checkout"), "#!/bin/sh\n")
	s.write(filepath.Join(dotfiles, "hooklib", "README"), "standin\n")
	for link, to := range map[string]string{
		filepath.Join(home, ".githooks", "post-checkout"): filepath.Join(dotfiles, "githooks", "post-checkout"),
		filepath.Join(home, ".githooks", "lib"):           filepath.Join("..", "..", "dotfiles", "hooklib"),
		filepath.Join(dotfiles, "hooklib", "deeper"):      filepath.Join(dotfiles, "not-yet"),
	} {
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(to, link); err != nil {
			t.Fatal(err)
		}
	}
	s.write(filepath.Join(home, "inc", "one"), "[include]\n\tpath = ../missing\n[standin]\n\twho = one\n")
	// git never reads cond, its condition being false here, so its cycle (it names itself) is the reader's only.
	s.write(filepath.Join(home, "cond"), "[includeIf \"onbranch:nope\"]\n\tpath = "+filepath.Join(folder, "two")+"\n\tpath = cond\n")
	s.write(filepath.Join(folder, "xdg", "git", "config"), "[includeIf \"gitdir:~/Projects/\"]\n\tpath = ~/work.gitconfig\n")
	system := filepath.Join(folder, "prefix", "etc", "gitconfig")
	s.write(system, "[core]\n\thooksPath = /dev/null\n\tfsmonitor = true\n")
	if err := os.Symlink(filepath.Join(folder, "dotfiles", "real"), filepath.Join(home, "linked")); err != nil {
		t.Fatal(err)
	}
	if got := s.ok(s.repo, "config", "standin.who"); got != "one" {
		t.Fatalf("the stand-in's config reads %q, want one", got)
	}

	pl := s.places()
	pl.GitConfigs = []string{system}
	env := append(slices.Clip(s.env), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=include.path", "GIT_CONFIG_VALUE_0=~/from-env")
	hooks, named, err := ReadGitConfig(context.Background(), s.git, env, pl)
	if err != nil {
		t.Fatalf("ReadGitConfig: %v", err)
	}
	wantHooks := []string{filepath.Join(s.root, ".husky", "_"), filepath.Join(s.repo, ".husky", "_"), filepath.Join(s.root, ".husky"),
		filepath.Join(s.repo, ".husky"), filepath.Join(home, ".githooks"), filepath.Join(dotfiles, "hooklib")}
	wantNamed := []string{
		filepath.Join(home, "inc", "one"), filepath.Join(home, "missing"), filepath.Join(home, "linked"),
		filepath.Join(folder, "dotfiles", "real"), filepath.Join(home, "cond"), filepath.Join(folder, "two"),
		filepath.Join(home, "ssh", "config"), "/opt/standin/nc", filepath.Join(home, "work.gitconfig"),
		filepath.Join(s.gd, "worktrees", "h1", "wt-inc"), filepath.Join(home, "from-env"),
		filepath.Join(s.root, "tools", "fsmon.sh"), filepath.Join(s.repo, "tools", "fsmon.sh"),
		filepath.Join(folder, "xdg", "husky", "init.sh"), filepath.Join(home, ".huskyrc"),
		filepath.Join(dotfiles, "githooks", "post-checkout"), filepath.Join(dotfiles, "not-yet"),
	}
	slices.Sort(wantHooks)
	slices.Sort(wantNamed)
	if !slices.Equal(hooks, wantHooks) {
		t.Errorf("hooks:\n%s\nwant\n%s", strings.Join(hooks, "\n"), strings.Join(wantHooks, "\n"))
	}
	if !slices.Equal(named, wantNamed) {
		t.Errorf("named:\n%s\nwant\n%s", strings.Join(named, "\n"), strings.Join(wantNamed, "\n"))
	}

	// The Policy denies writing each, and what's in each hooks folder (husky's hooks beside its `_`, the owner's and new
	// ones), and nothing beside them.
	pl.GitHooks, pl.GitNamed = hooks, named
	p := mustNew(t, pl)
	for _, at := range append(slices.Clone(named), filepath.Join(s.root, ".husky", "_", "pre-commit"), filepath.Join(s.repo, ".husky", "_", "h"),
		filepath.Join(s.root, ".husky", "pre-commit"), filepath.Join(s.root, ".husky", "post-checkout"),
		filepath.Join(s.repo, ".husky", "pre-commit"), filepath.Join(dotfiles, "hooklib", "README"), filepath.Join(home, ".githooks", "pre-push")) {
		if v, by := p.Decide(at, Write); v != Deny {
			t.Errorf("write %s: %v by %q, want deny", at, v, by.Row)
		}
	}
	for _, at := range []string{filepath.Join(s.root, "main.go"), filepath.Join(s.root, "husky.md"), filepath.Join(dotfiles, "notes"),
		filepath.Join(home, "inc", "two"), filepath.Join(s.root, "tools", "build.sh"), filepath.Join(folder, "xdg", "husky", "notes")} {
		if v, by := p.Decide(at, Write); v != Allow {
			t.Errorf("write %s: %v by %q, want allow", at, v, by.Row)
		}
	}
}

// A value whose place can't be told for sure, a config file git can't read, or a git that doesn't run, fails the
// start: the Policy is never made from a partial read.
func TestAGitConfigTheReaderCantBeSureOfFailsTheStart(t *testing.T) {
	s := newGitStandin(t)
	global := filepath.Join(s.home, ".gitconfig")
	ours := map[string]string{
		"another user's home":        "[includeIf \"gitdir:/nowhere/\"]\n\tpath = ~other/x\n", // git's own startup refuses it unconditioned
		"git's prefix":               "[include]\n\tpath = %(prefix)/etc/x\n",
		"an expansion in ssh":        "[core]\n\tsshCommand = ssh -F $HOME/x\n",
		"ssh's own %d":               "[core]\n\tsshCommand = ssh -i %d/.ssh/k\n",
		"an operator in fsmonitor":   "[core]\n\tfsmonitor = \"tools/a;tools/b\"\n",
		"unbalanced quotes":          "[core]\n\tfsmonitor = \"tools/fsmon 'x\"\n",
		"a glob":                     "[core]\n\tsshCommand = ssh -F ~/ssh/*.conf\n",
		"a subshell in double quote": "[core]\n\tsshCommand = \"ssh -F \\\"`x`\\\"\"\n",
	}
	for name, text := range ours {
		s.write(global, text)
		if _, _, err := ReadGitConfig(context.Background(), s.git, s.env, s.places()); !errors.Is(err, ErrGitConfig) {
			t.Errorf("%s: %v, want ErrGitConfig", name, err)
		}
	}
	// More files than git itself would include: behind false conditions, which git doesn't follow (it fails on an include
	// it can't expand, a cycle or too deep a chain, before it lists anything).
	for i := range maxGitConfigFiles + 2 {
		s.write(filepath.Join(s.home, "chain", fmt.Sprint(i)), fmt.Sprintf("[includeIf \"gitdir:/nowhere/\"]\n\tpath = %d\n", i+1))
	}
	s.write(global, "[includeIf \"gitdir:/nowhere/\"]\n\tpath = chain/0\n")
	if _, _, err := ReadGitConfig(context.Background(), s.git, s.env, s.places()); !errors.Is(err, ErrGitConfig) {
		t.Errorf("a long chain of includes: %v, want ErrGitConfig", err)
	}
	// git's own refusals.
	s.write(global, "[include]\n\tpath = ~/broken\n")
	s.write(filepath.Join(s.home, "broken"), "[broken\n")
	if _, _, err := ReadGitConfig(context.Background(), s.git, s.env, s.places()); err == nil {
		t.Error("an include git can't read: no error")
	}
	s.write(global, "")
	env := append(slices.Clip(s.env), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=include.path", "GIT_CONFIG_VALUE_0=relative")
	if _, _, err := ReadGitConfig(context.Background(), s.git, env, s.places()); err == nil {
		t.Error("a relative include on the command line: no error")
	}
	if _, _, err := ReadGitConfig(context.Background(), filepath.Join(s.folder, "no-git"), s.env, s.places()); err == nil {
		t.Error("a git that isn't there: no error")
	}
	// The control: the same stand-in reads with no error once the config is plain.
	s.write(global, "[core]\n\tsshCommand = ssh -o \"ProxyCommand=nc %h %p\"\n")
	if _, named, err := ReadGitConfig(context.Background(), s.git, s.env, s.places()); err != nil || len(named) != 2 {
		t.Errorf("a plain config: %q %v, want the two nc's in each root", named, err)
	}
}

// A command's words are the shell's, its quotes removed; what only a shell could tell is refused.
func TestACommandsWordsAreTheShells(t *testing.T) {
	for in, want := range map[string][]string{
		`ssh -i ~/.ssh/k -F "/x y/cfg"`:    {"ssh", "-i", "~/.ssh/k", "-F", "/x y/cfg"},
		`'a b'c\ d  e`:                     {"a bc d", "e"},
		`"a\"b\\c\d"`:                      {`a"b\c\d`},
		`a#b -o "ProxyCommand=nc %h %p"`:   {"a#b", "-o", "ProxyCommand=nc %h %p"},
		"tools/fsmon.sh\t--flag\n":         {"tools/fsmon.sh", "--flag"},
		``:                                 nil,
		`ssh -o UserKnownHostsFile=/x/y z`: {"ssh", "-o", "UserKnownHostsFile=/x/y", "z"},
	} {
		if got, err := commandWords(in); err != nil || !slices.Equal(got, want) {
			t.Errorf("%q: %q %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{`ssh $X`, "ssh `x`", `a;b`, `a | b`, `a && b`, `"open`, `'open`, `a*`, `a?`, `a[b]`, `x "$y"`,
		"x \"`y`\"", `# c`, `a\`, `a > b`, `(a)`, `{a,b}`, `!a`} {
		if got, err := commandWords(in); !errors.Is(err, ErrGitConfig) {
			t.Errorf("%q: %q %v, want ErrGitConfig", in, got, err)
		}
	}
	for word, want := range map[string][]string{"ssh": nil, "/opt/x/ssh": {"/opt/x/ssh"}, "~/bin/ssh": {"~/bin/ssh"}} {
		if got := commandFiles(0, word); !slices.Equal(got, want) {
			t.Errorf("the program %q: %q, want %q", word, got, want)
		}
	}
	for word, want := range map[string][]string{"-F": nil, "-v": nil, "cfg": {"cfg"}, "-oIdentityFile=/k": {"/k"},
		"ProxyCommand=/opt/nc %h": {"/opt/nc", "%h"}, "--long": nil,
		// RT126GITCFG1 B3: a value attached to its letter, ssh_config's `Key Value`, a command inside a value.
		"-F/x/cfg": {"/x/cfg"}, "-i~/k": {"~/k"}, "ProxyCommand /x/nc": {"ProxyCommand /x/nc", "ProxyCommand", "/x/nc"},
		"ProxyCommand /x/nc %h": {"ProxyCommand", "/x/nc", "%h"}, "-oInclude /x/inc": {"Include /x/inc", "Include", "/x/inc"},
		"ProxyCommand=ssh -F/x/j.conf j": {"ssh -F/x/j.conf j", "ssh", "/x/j.conf", "j"}, "/a dir/cfg": {"/a dir/cfg", "/a", "dir/cfg"},
		"--opt=/x/long": {"/x/long"}} {
		if got := commandFiles(1, word); !slices.Equal(got, want) {
			t.Errorf("the word %q: %q, want %q", word, got, want)
		}
	}
	for value, want := range map[string]bool{"true": true, "False": true, "": true, "1": true, "0": true, "-1": true, "on": true,
		"tools/fsmon.sh": false, "/usr/local/bin/watchman-hook": false, "1x": false} {
		if gitBool(value) != want {
			t.Errorf("gitBool(%q) = %v", value, !want)
		}
	}
}

// RT126GITCFG1 B3: an ssh option's file is named whatever its spelling: a value attached to its letter (`-F/x`, ssh's
// getopt takes it), ssh_config's `Key Value` form in a -o, and the `Key=Value` form as the control. The words around
// it (`ProxyCommand`, `Include`) are over-denied, as other option values are.
func TestAnSSHOptionNamesItsFileInEverySpelling(t *testing.T) {
	s := newGitStandin(t)
	global := filepath.Join(s.home, ".gitconfig")
	bin := filepath.Join(s.home, "bin", "nc")
	for command, want := range map[string]string{
		"ssh -F" + filepath.Join(s.home, "work", "ssh_config"):     filepath.Join(s.home, "work", "ssh_config"),
		"ssh -F~/work/attached":                                    filepath.Join(s.home, "work", "attached"),
		`ssh -o "ProxyCommand ` + bin + ` %h %p"`:                  bin,
		"ssh -o ProxyCommand=" + bin:                               bin,
		"ssh -i" + filepath.Join(s.home, "keys", "id_standin"):     filepath.Join(s.home, "keys", "id_standin"),
		`ssh -o "Include /opt/standin/inc"`:                        "/opt/standin/inc",
		`ssh -o "KnownHostsCommand /opt/standin/khc %H"`:           "/opt/standin/khc",
		`ssh -o "LocalCommand /opt/standin/local"`:                 "/opt/standin/local",
		`ssh "-oProxyCommand /opt/standin/attached"`:               "/opt/standin/attached",
		`ssh -o "ProxyCommand=ssh -F/opt/standin/jump.conf jump"`:  "/opt/standin/jump.conf",
		`ssh -F "` + filepath.Join(s.home, "a dir", "cfg") + `"`:   filepath.Join(s.home, "a dir", "cfg"),
		`ssh --standin=` + filepath.Join(s.home, "long", "option"): filepath.Join(s.home, "long", "option"),
	} {
		s.write(global, "[core]\n\tsshCommand = "+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(command)+"\n")
		_, named, err := ReadGitConfig(context.Background(), s.git, s.env, s.places())
		if err != nil || !slices.Contains(named, want) {
			t.Errorf("sshCommand = %s: %v, %q doesn't name %s", command, err, named, want)
		}
	}
	// What can't be told still fails the start in the new spellings: ssh's %d in an attached value or a -o's value.
	for _, command := range []string{"ssh -i%d/.ssh/k", `ssh -o "IdentityFile %d/.ssh/k"`} {
		s.write(global, "[core]\n\tsshCommand = "+strings.ReplaceAll(command, `"`, `\"`)+"\n")
		if _, _, err := ReadGitConfig(context.Background(), s.git, s.env, s.places()); !errors.Is(err, ErrGitConfig) {
			t.Errorf("sshCommand = %s: %v, want ErrGitConfig", command, err)
		}
	}
}

// RT126GITCFG1 N1: a place under a file is missing, as git takes it, not an error that stops every desk: git's own
// fsmonitor-watchman docs suggest `.git/hooks/fsmonitor-watchman`, and in a hire's worktree .git is a file. The place is
// still named.
func TestAPlaceUnderAFileIsMissingAndStillNamed(t *testing.T) {
	s := newGitStandin(t)
	s.ok(s.repo, "config", "core.fsmonitor", ".git/hooks/fsmonitor-watchman")
	s.write(filepath.Join(s.home, ".gitconfig"), "[includeIf \"gitdir:/nowhere/\"]\n\tpath = ~/.gitconfig/x\n")
	_, named, err := ReadGitConfig(context.Background(), s.git, s.env, s.places())
	if err != nil {
		t.Fatalf("ReadGitConfig: %v", err)
	}
	for _, want := range []string{filepath.Join(s.root, ".git", "hooks", "fsmonitor-watchman"),
		filepath.Join(s.repo, ".git", "hooks", "fsmonitor-watchman"), filepath.Join(s.home, ".gitconfig", "x")} {
		if !slices.Contains(named, want) {
			t.Errorf("%s isn't named: %q", want, named)
		}
	}
}

// RT126GITCFG1 N2: another desk's registration's config.worktree is read, as the owner's git reads it in that worktree:
// its absolute include is named for this desk too, whether or not Places lists the registration.
func TestEveryRegistrationsConfigWorktreeIsRead(t *testing.T) {
	s := newGitStandin(t)
	other := filepath.Join(filepath.Dir(s.root), "h2")
	s.ok(s.repo, "worktree", "add", "-q", "-b", "hire/h2", other)
	s.ok(s.repo, "config", "extensions.worktreeConfig", "true")
	include := filepath.Join(s.home, ".gitconfig-h2")
	s.ok(other, "config", "--worktree", "include.path", include)
	for _, listed := range [][]string{nil, {"h1", "h2"}} {
		pl := s.places()
		pl.OtherWorktrees = listed
		_, named, err := ReadGitConfig(context.Background(), s.git, s.env, pl)
		if err != nil || !slices.Contains(named, include) {
			t.Errorf("other worktrees %q: %v, %q doesn't name %s", listed, err, named, include)
		}
	}
}
