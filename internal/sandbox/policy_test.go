package sandbox

import (
	"cmp"
	"errors"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// SA-X1: the Policy, row by row (SANDBOX-PLAN §A4.1, SIERRA's base-branch row, PL126M's memory row), for a hire, the
// GM and a hire of a test copy, on a made-up Mac whose paths exist nowhere: the Policy is data and touches no file.

const (
	home        = "/Users/ada"
	realSupport = home + "/Library/Application Support/AgentDeck"
	copyDir     = home + "/Projects/smolcorp/.dev/support"
	repo        = home + "/Projects/app"
	gitDir      = repo + "/.git"
	hires       = home + "/Projects/app-hires"
	gmDir       = home + "/Projects/app-gm"
	sockets     = "/private/tmp/smolcorp-501"
	tmuxFolder  = "/private/tmp/tmux-501"
)

// setup is one column: a desk's places, and what its placeholders stand for.
type setup struct {
	name string
	pl   Places
}

func (s setup) path(p string) string {
	return strings.NewReplacer("$ROOT", s.pl.Root, "$S", s.pl.Support, "$R", realSupport, "$ID", s.pl.OwnID, "$WT", s.pl.OwnWorktree,
		"$H", home, "$G", gitDir).Replace(p)
}

func hirePlaces(support string) Places {
	return Places{GOOS: "darwin", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: support + "/desk-start",
		Root: hires + "/h1", Checkout: repo, GitDir: gitDir, OwnWorktree: "h1", OtherWorktrees: []string{"h1", "h2", "gm"},
		DefaultBranch: "develop", HiresFolder: hires, GMFolder: gmDir, SocketParent: sockets, OwnSockets: sockets + "/h1", TmuxFolder: tmuxFolder,
		Accounts: []string{home + "/.claude-work"}, InstallRoots: []string{"/Applications/Smolcorp.app", "/opt/homebrew/Cellar/tmux"},
		GitConfigs: []string{"/opt/homebrew/etc/gitconfig", home + "/.xdg/git/config"}, RemoteLogin: true,
		GitHooks: []string{hires + "/h1/.husky/_", hires + "/h1/.husky", home + "/.githooks", home + "/dotfiles/hooklib"},
		GitNamed: []string{home + "/.gitconfig-work", home + "/.config/standin/ssh_config", "/opt/standin/nc",
			home + "/dotfiles/githooks/post-checkout", home + "/.config/husky/init.sh"}}
}

func setups() []setup {
	hire := hirePlaces(realSupport)
	gm := hirePlaces(realSupport)
	gm.Subject, gm.OwnID, gm.Root, gm.OwnWorktree, gm.OwnSockets = GM, "gm1", gmDir, "gm", sockets+"/gm1"
	testCopy := hirePlaces(copyDir)
	testCopy.OtherSupports = []string{realSupport}
	return []setup{{"hire", hire}, {"GM", gm}, {"test copy's hire", testCopy}}
}

func mustNew(t *testing.T, pl Places) *Policy {
	t.Helper()
	p, err := New(pl)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// mustBuild is New's Policy for a root New refuses for holding desks (RT126SAX1R2 B2′): the rows must hold there too.
func mustBuild(t *testing.T, pl Places) *Policy {
	t.Helper()
	p, err := build(pl)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return p
}

const (
	A = Allow
	D = Deny
)

// row is one §A4.1 row's case: a path (with placeholders), what is asked, and the answer in each column.
type row struct {
	path               string
	op                 Op
	hire, gm, testCopy Verdict
}

// all is the same answer in every column.
func all(path string, op Op, v Verdict) row { return row{path, op, v, v, v} }

var rows = []row{
	// The support folder, whole: every file in it, files added later, its sockets.
	all("$S/phones.json", Read, D), all("$S/phones.json", Write, D), all("$S/hook-key", Read, D),
	all("$S/a-secret-added-later", Read, D), all("$S/a-secret-added-later", Write, D), all("$S", Read, D),
	all("$S/hires", Read, D), all("$S/hires/$ID", Read, D), all("$S/hires/$ID", Write, D), all("$S/selftest-key", Read, D),
	// A test copy's hire reaches nothing of the owner's realSupport folder: no exception holds there.
	{"$R/phones.json", Read, D, D, D}, {"$R/hire-guard.json", Read, A, A, D}, {"$R/hires/$ID/BRIEF.md", Read, A, A, D},
	{"$R/hires/$ID/STATUS.md", Write, A, A, D}, {"$R/desk-start", Connect, A, A, D},
	// …except the guard file, read only.
	all("$S/hire-guard.json", Read, A), all("$S/hire-guard.json", Write, D),
	// …and the night shift's jobs, read only as the guard file (NS1, STUDIO27): no write, by any spelling, nor a link
	// or a rename onto it; the copy set aside unread and a temp file beside it are denied whole.
	all("$S/nightshift.json", Read, A), all("$S/nightshift.json", Write, D), all("$S/nightshift.json", Connect, D),
	all("$S/nightshift.json", Link, D), all("$S/nightshift.json", Move, D), all("$S/NightShift.JSON", Write, D),
	all("$S/hires/$ID/../../nightshift.json", Write, D), all("$S/nightshift.json.bad", Read, D),
	all("$S/nightshift.json.tmp", Write, D), all("$S/.nightshift.json.tmp-write", Write, D), all("$S/nightshift.json/x", Read, D),
	{"$R/nightshift.json", Read, A, A, D}, {"$R/nightshift.json", Write, D, D, D},
	// …its fence and brief: read only.
	all("$S/hires/$ID/settings.json", Read, A), all("$S/hires/$ID/settings.json", Write, D),
	all("$S/hires/$ID/sandbox.sb", Read, A), all("$S/hires/$ID/sandbox.sb", Write, D),
	all("$S/hires/$ID/BRIEF.md", Read, A), all("$S/hires/$ID/BRIEF.md", Write, D), all("$S/hires/$ID/BRIEF-2.md", Write, D),
	// …its status and report: its work.
	all("$S/hires/$ID/STATUS.md", Write, A), all("$S/hires/$ID/STATUS-2.md", Write, A), all("$S/hires/$ID/REPORT.md", Write, A),
	all("$S/hires/$ID/STATUS.md", Read, A),
	// The start script's row is gone (SG2: no script): nothing else in its folder is open.
	all("$S/hires/$ID/hire-1.sh", Read, D), all("$S/hires/$ID/hire-1.sh", Write, D), all("$S/hires/$ID/notes.txt", Read, D),
	all("$S/hires/$ID/sub/STATUS.md", Write, D),
	// …its own Claude folder, but its settings and the links the core put there, and a fence made in it (RT126SAX1 N3).
	all("$S/hires/$ID/claude/.claude/settings.json", Write, D), all("$S/hires/$ID/claude/.claude", Write, D),
	all("$S/hires/$ID/claude/projects/p/s.jsonl", Write, A), all("$S/hires/$ID/claude/.claude.json", Write, A),
	all("$S/hires/$ID/claude/todos/t.json", Read, A), all("$S/hires/$ID/claude/settings.json", Read, A),
	all("$S/hires/$ID/claude/settings.json", Write, D), all("$S/hires/$ID/claude/settings.local.json", Write, D),
	all("$S/hires/$ID/claude/CLAUDE.md", Write, D), all("$S/hires/$ID/claude/commands/x.md", Write, D),
	all("$S/hires/$ID/claude/skills", Write, D), all("$S/hires/$ID/claude/commands/x.md", Read, A),
	// Another desk's folder in the support folder.
	all("$S/hires/h2/STATUS.md", Read, D), all("$S/hires/h2/STATUS.md", Write, D), all("$S/hires/h2/claude/x", Read, D),
	// The desk-start socket: connect only. The owner's tap: never.
	all("$S/desk-start", Connect, A), all("$S/desk-start", Read, D), all("$S/desk-start", Write, D),
	all("$S/serve.sock", Connect, D), all("$S/serve.sock", Read, D), {"$R/serve.sock", Connect, D, D, D},
	all("$S/other.sock", Connect, D),
	// The GM's memory: no column holds an exception (PL126M; an MCP server's column is its desk's, so these too).
	all("$S/gm-memory/manifest.json", Read, D), all("$S/gm-memory/x.md", Write, D), all("$S/gm-memory", Read, D),
	{"$R/gm-memory/x.md", Read, D, D, D},
	// The canary.
	all("$S/sandbox-canary-7", Read, D), all("$S/sandbox-canary-7", Write, D),
	// The fences, anywhere.
	all("$ROOT/.claude/settings.json", Write, D), all("$ROOT/.claude/settings.json", Read, A),
	all("$ROOT/sub/pkg/.claude/settings.local.json", Write, D), all("$ROOT/.claude/commands/x.md", Write, A),
	all("$ROOT/new/.claude", Write, D), all("$ROOT/.claude", Write, D),
	// ssh while Remote Login answers.
	all("$H/.ssh/id_ed25519", Read, D), all("$H/.ssh/id_ed25519", Write, D), all("$H/.ssh/id_ed25519.pub", Read, A),
	all("$H/.ssh/known_hosts", Write, A), all("$H/.ssh/config", Read, A), all("$H/.ssh/config", Write, D),
	all("$H/.ssh/authorized_keys", Read, D), all("$H/.ssh/authorized_keys2", Write, D), all("$H/.ssh/rc", Write, D),
	// The account folders: readable but their tokens and history; never writable.
	all("$H/.claude/CLAUDE.md", Read, A), all("$H/.claude/CLAUDE.md", Write, D), all("$H/.claude/skills/s/SKILL.md", Write, D),
	all("$H/.claude/ide/1234.lock", Read, D), all("$H/.claude/.credentials.json", Read, D), all("$H/.claude/history.jsonl", Read, D),
	all("$H/.claude/projects/p/s.jsonl", Read, D), all("$H/.claude/sessions/s.json", Read, D),
	all("$H/.claude-work/agents/a.md", Write, D), all("$H/.claude-work/.credentials.json", Read, D), all("$H/.claude-work/.claude.json", Read, D),
	all("$H/.claude.json", Read, D), all("$H/.claude.json", Write, D), all("$H/.claude.json.backup", Read, D),
	// The other agents' sign-ins.
	all("$H/.codex/auth.json", Read, D), all("$H/.gemini/oauth_creds.json", Read, D), all("$H/.codex/config.toml", Read, A),
	// The places desks hang from can't be moved or removed.
	all("$S", Write, D), all("$ROOT", Write, D), all("$H/Projects", Write, D), all("$H", Write, D), all(repo, Write, D),
	all(home+"/Library/Application Support", Write, D), {"$R", Write, D, D, D},
	// Smolcorp's install and the programs it resolved.
	all("/Applications/Smolcorp.app/Contents/MacOS/AgentDeck", Write, D), all("/Applications/Smolcorp.app/Contents/MacOS/AgentDeck", Read, A),
	all("/opt/homebrew/Cellar/tmux/3.5/bin/tmux", Write, D),
	// Plants.
	all("$H/.zshrc", Write, D), all("$H/.zshrc", Read, A), all("$H/.bash_profile", Write, D), all("$H/.profile", Write, D),
	all("$H/Library/LaunchAgents/x.plist", Write, D), all("$H/.gitconfig", Write, D), all("$H/.config/git/config", Write, D),
	all("$H/.tmux.conf", Write, D), all("$H/.config/tmux/tmux.conf", Write, D), all("$H/.config/fish/config.fish", Write, D),
	all("$H/.config/systemd/user/x.service", Write, D), all("$H/.config/autostart/x.desktop", Write, D),
	all("/usr/lib/cron/tabs/ada", Write, D), all("/private/var/at/jobs/x", Write, D),
	all("/opt/homebrew/etc/gitconfig", Write, D), all("/opt/homebrew/etc/gitconfig", Read, A), all("$H/.xdg/git/config", Write, D),
	all("/opt/homebrew/etc/gitattributes", Write, A), all("/opt/homebrew/etc", Move, D),
	// The shared git places.
	all("$G/config", Write, D), all("$G/config", Read, A), all("$G/config.worktree", Write, D), all("$G/hooks/pre-commit", Write, D),
	all("$G/info/exclude", Write, D), all("$G/objects/info/alternates", Write, D), all("$G/modules/sub/config", Write, D),
	all("$G/modules/a/b/hooks/post-checkout", Write, D), all("$G/index", Write, D), all("$G/HEAD", Write, D),
	all("$G/worktrees/h2/HEAD", Write, D), all("$G/worktrees/$WT/HEAD", Write, A), all("$G/worktrees/$WT/index", Write, A),
	all("$G/worktrees/added-later/HEAD", Write, A),
	// …and every file git reads to find the repo's config, hooks and objects (SIERRA, 14 Nov): the shared .git's
	// commondir and gitdir, the other stores' lists, each worktree's registration, its commondir, gitdir and config, the
	// desk's own included, and the .git at the desk's place. Its own registration's HEAD, logs and submodules stay its.
	all("$G/commondir", Write, D), all("$G/commondir", Read, A), all("$G/gitdir", Write, D), all("$G/info/alternates", Write, D),
	all("$G/objects/info/http-alternates", Write, D), all("$G/worktrees/$WT", Write, D), all("$G/worktrees/new", Write, D),
	all("$G/worktrees/$WT/commondir", Write, D), all("$G/worktrees/$WT/gitdir", Write, D), all("$G/worktrees/$WT/config.worktree", Write, D),
	all("$G/worktrees/added-later/commondir", Write, D), all("$G/worktrees/added-later/gitdir", Write, D),
	all("$G/worktrees/$WT/commondir", Read, A), all("$G/worktrees/$WT/logs/HEAD", Write, A),
	all("$G/worktrees/$WT/modules/lib/config", Write, A), all("$G/worktrees/$WT/modules/lib/objects/x", Write, A),
	all("$G/worktrees/h2/modules/lib/objects/x", Write, D), all("$ROOT/.git", Write, D), all("$ROOT/.git", Read, A),
	all("$ROOT/.git", Move, D), all("$ROOT/.git", Link, D), all("$ROOT/lib/.git", Write, A),
	// …and the entries that make the shared .git a git folder (RT126SAX1R5 B3): their mode, ACL or name, so git never
	// walks past it to one a desk made above the checkout, which it may. What git makes inside them is still its work.
	all("$G", Write, D), all("$G/objects", Write, D), all("$G/refs", Write, D), all("$G", Read, A), all("$G/refs", Read, A),
	all("$G/ORIG_HEAD", Write, A), all("$G/objects/ab", Write, A), all("$G/refs/tags/v1", Write, A),
	all("$H/.git/config", Write, A), all("$H/Projects/config", Write, A), all("$H/Projects/HEAD", Write, A),
	// …and the owner's rebase state, whose todo lists hold exec lines; the desk's own is in its registration.
	all("$G/rebase-merge", Write, D), all("$G/rebase-merge/git-rebase-todo", Write, D), all("$G/rebase-apply/next", Write, D),
	all("$G/sequencer/todo", Write, D), all("$G/rebase-merge", Move, D), all("$G/worktrees/$WT/rebase-merge/git-rebase-todo", Write, A),
	// The rest of the shared .git.
	all("$G/objects/ab/cdef", Write, A), all("$G/refs/heads/feature", Write, A), all("$G/refs/heads/hire/h1", Write, A),
	all("$G/logs/HEAD", Write, A), all("$G/refs/remotes/origin/main", Write, A),
	// The base branches (SIERRA, 14 Nov): main, master, release/*, the default; packed-refs; the config.
	all("$G/refs/heads/main", Write, D), all("$G/refs/heads/master", Write, D), all("$G/refs/heads/release/1.26", Write, D),
	all("$G/refs/heads/release", Write, D), all("$G/refs/heads/develop", Write, D), all("$G/packed-refs", Write, D),
	all("$G/refs/heads/main", Read, A), all("$G/packed-refs", Read, A), all("$G/refs/heads/mainline", Write, A),
	// …and in a reftable repo, every ref of the shared .git (GITCFG1); the desk's own registration's stay its.
	all("$G/reftable", Write, D), all("$G/reftable/tables.list", Write, D), all("$G/reftable/0x01-0x02-abcd.ref", Write, D),
	all("$G/reftable", Move, D), all("$G/reftable/tables.list", Read, A), all("$G/worktrees/$WT/reftable/tables.list", Write, A),
	// What the owner's git config names (GITCFG1): its hooks folders, whole, and the files it includes or runs, and the
	// folders above them hold. What is beside them stays normal work, or the desk's own.
	all(hires+"/h1/.husky/_", Write, D), all(hires+"/h1/.husky/_/pre-commit", Write, D), all(hires+"/h1/.husky/_/h", Write, D),
	all(hires+"/h1/.husky/_/pre-commit", Read, A), all(hires+"/h1/.husky", Move, D), all(hires+"/h1/.husky", Link, D),
	// Husky's hooks beside its `_`, the owner's and new ones, and its init.sh (RT126GITCFG1 B1); what a link in a hooks
	// folder leads to, a file or a folder (B2).
	all(hires+"/h1/.husky/pre-commit", Write, D), all(hires+"/h1/.husky/post-checkout", Write, D), all(hires+"/h1/.husky", Move, D),
	{hires + "/h1/husky.md", Write, A, D, A}, all("$H/.config/husky/init.sh", Write, D), all("$H/.config/husky/notes", Write, A),
	all("$H/dotfiles/githooks/post-checkout", Write, D), all("$H/dotfiles/githooks/README", Write, A),
	all("$H/dotfiles/hooklib/run.sh", Write, D), all("$H/dotfiles/hooklib", Move, D), all("$H/dotfiles/notes", Write, A),
	all("$H/.githooks/post-checkout", Write, D), all("$H/.githooks", Link, D),
	all("$H/.gitconfig-work", Write, D), all("$H/.gitconfig-work", Read, A), all("$H/.gitconfig-work", Move, D),
	all("$H/.gitconfig-work.bak", Write, A), all("$H/.config/standin/ssh_config", Write, D), all("$H/.config/standin", Move, D),
	all("$H/.config/standin/notes", Write, A), all("/opt/standin/nc", Write, D), all("/opt/standin/notes", Write, A),
	// Other desks' places.
	all(hires+"/h2/main.go", Write, D), all(hires+"/h2/main.go", Read, A), {gmDir + "/notes.md", Write, D, A, D},
	all(repo+"/main.go", Write, D), all(repo+"/main.go", Read, A),
	// The socket folder, and no links anywhere in it, the desk's own folder too (§A4.1, RT126SAX1 B3).
	all(sockets+"/h2/s", Write, D), all(sockets+"/$ID/s", Write, A), all(sockets+"/link", Write, D),
	all(sockets+"/link", Link, D), all(sockets+"/$ID/a-link", Link, D), all(sockets+"/$ID/s", Link, D), all(sockets+"/h2/a-link", Link, D),
	all("$ROOT/a-link", Link, A), all("$G/hooks/a-link", Link, D), all("$S/hires/$ID/claude/a-link", Link, A),
	// tmux's own folder (FXTMUXDIR), whole and with no exception: nothing made, removed, renamed or linked there, the
	// folder itself included; read as normal work. Beside it, normal work.
	all(tmuxFolder+"/default", Write, D), all(tmuxFolder+"/smolcorp-h2", Write, D), all(tmuxFolder+"/smolcorp-h2", Link, D),
	all(tmuxFolder+"/smolcorp-h2", Move, D), all(tmuxFolder+"/new/s", Write, D), all(tmuxFolder, Write, D), all(tmuxFolder, Move, D),
	all(tmuxFolder, Link, D), all(tmuxFolder+"/default", Read, A), all("/private/tmp/tmux-5010/default", Write, A),
	all("/private/tmp/tmux-501-x", Link, A),
	// The folders above a denied place can't be removed or renamed away (RT126SAX1 B1): the shared .git, its refs,
	// a submodule's folders, ~/.ssh, the other agents' and the config folders, the account folders, the desk's own
	// Claude folder. Files and folders are still made inside them.
	all("$G", Move, D), all("$G/refs", Move, D), all("$G/refs/heads", Move, D), all("$G/refs/heads/release", Move, D),
	all("$G/modules", Move, D), all("$G/modules/sub", Move, D), all("$G/modules/a/b", Move, D), all("$G/worktrees", Move, D),
	all("$G/objects", Move, D), all("$H/.ssh", Move, D), all("$H/.codex", Move, D), all("$H/.gemini", Move, D),
	all("$H/.config", Move, D), all("$H/.config/systemd", Move, D), all("$H/.config/git", Move, D), all("$H/.claude", Move, D),
	all("$H/Library", Move, D), all("$S/hires/$ID/claude", Move, D), all("$S/hires", Move, D), all(hires, Move, D),
	all("/opt/homebrew/Cellar", Move, D), all("/private/tmp", Move, D), all(sockets, Move, D),
	all("$G/refs/heads", Write, A), all("$G/refs/heads/team/x", Write, A),
	all("$H/.config/nvim/init.lua", Write, A),
	// …nor made a link while they don't exist (RT126SAX1R2 B1′), and a submodule's folder isn't brought in whole. The
	// core makes the rest before the desk starts (Hold); a desk may still make them.
	all("$G", Link, D), all("$G/modules", Link, D), all("$G/modules/sub", Link, D), all("$H/.ssh", Link, D), all("$H/.codex", Link, D),
	all("$H/.gemini", Link, D), all("$H/.config", Link, D), all("$H/.config/systemd", Link, D), all("$S/hires/$ID/claude", Link, D),
	all("$G/modules/new", Write, D), all("$G/modules/a/modules/b", Write, D),
	all("$H/.codex", Write, A), all("$H/.config/systemd", Write, A), all("$H/.config/nvim", Link, A),
	// …and .git/modules is denied whole (RT126SAX1R4 B1‴): a submodule's git folder as much as any other folder.
	all("$G/modules", Write, D), all("$G/modules/sub/modules", Write, D), all("$G/modules/sub/modules/x/config", Write, D),
	all("$G/modules/third_party", Write, D), all("$G/modules/third_party/evil", Write, D),
	all("$G/modules/third_party/evil/config", Write, D), all("$G/modules/third_party/bar", Write, D),
	all("$G/modules/new/objects/ab/cdef", Write, D), all("$G/modules/third_party/bar/objects/ab/cdef", Write, D),
	all("$G/modules/sub/logs/HEAD", Write, D), all("$G/modules/third_party/bar/config", Write, D),
	all("$G/modules/sub/objects/x", Write, D), all("$G/modules/sub/objects", Write, D), all("$G/modules/sub/HEAD", Write, D),
	all("$G/modules/sub/objects/ab", Move, D), all("$G/modules/sub/objects/ab/cdef", Read, A),
	// …and a folder above nothing denied moves as before.
	all("$G/refs/tags", Move, A), all("$G/refs/heads/topic", Move, A), all("$G/worktrees/$WT/logs", Move, A),
	all("$H/.config/nvim", Move, A), all("$ROOT/pkg", Move, A), all(sockets+"/$ID/sub", Move, A),
	// Its own place, and normal work.
	all("$ROOT/main.go", Write, A), all("$ROOT/main.go", Read, A), all("$H/Library/Caches/x", Write, A),
	all("/private/tmp/x", Write, A), all("$H/go/pkg/mod/x", Write, A), all("/opt/homebrew/bin/node", Read, A),
}

func TestEveryRowOfThePolicyForAHireTheGMAndATestCopy(t *testing.T) {
	for i, s := range setups() {
		p := mustNew(t, s.pl)
		for _, r := range rows {
			want := [...]Verdict{r.hire, r.gm, r.testCopy}[i]
			at := s.path(r.path)
			if got, by := p.Decide(at, r.op); got != want {
				t.Errorf("%s: %v %s: %v by %q, want %v", s.name, r.op, at, got, by.Row, want)
			}
		}
	}
}

// Every spelling of one file is the same rule: case, a decomposed name, the firmlinks, the old links and `..`.
func TestEverySpellingOfAPathIsTheSameRule(t *testing.T) {
	const jose = "/Users/Jos\u00e9" // composed, as the places are given
	pl := hirePlaces(jose + "/Library/Application Support/AgentDeck")
	pl.Home, pl.Root, pl.HiresFolder, pl.Checkout, pl.GitDir, pl.GMFolder = jose, jose+"/p-hires/h1", jose+"/p-hires", jose+"/p", jose+"/p/.git", ""
	pl.Accounts, pl.SocketParent, pl.OwnSockets = nil, "/tmp/smolcorp-501", "/tmp/smolcorp-501/h1"
	p := mustNew(t, pl)
	for _, c := range []struct {
		paths []string
		op    Op
		want  Verdict
	}{
		{[]string{
			jose + "/Library/Application Support/AgentDeck/hook-key",
			"/Users/Jose\u0301/Library/Application Support/AgentDeck/hook-key", // decomposed
			"/USERS/JOSÉ/LIBRARY/APPLICATION SUPPORT/AGENTDECK/HOOK-KEY",
			"/Users/JOSE\u0301/library/application support/agentdeck/Hook-Key",
			"/System/Volumes/Data" + jose + "/Library/Application Support/AgentDeck/hook-key",
			"/system/volumes/DATA" + jose + "/Library/Application Support/AgentDeck/hook-key",
			jose + "/p-hires/h1/../../Library/./Application Support//AgentDeck/hook-key",
			jose + "/Library/Application Support/AgentDeck/hires/h1/../../hook-key",
		}, Read, Deny},
		{[]string{
			jose + "/Library/Application Support/AgentDeck/hires/h1/BRIEF.md",
			"/Users/Jose\u0301/Library/Application Support/AgentDeck/hires/H1/brief.md",
		}, Write, Deny},
		{[]string{
			jose + "/p/.git/refs/heads/main",
			"/Users/Jose\u0301/P/.GIT/refs/heads/MAIN",
			jose + "/p-hires/h1/../../p/.git/refs/heads/main",
			"/System/Volumes/Data/Users/jos\u00c9/p/.git/refs/heads/main",
		}, Write, Deny},
		{[]string{"/tmp/smolcorp-501/h2/s", "/private/tmp/smolcorp-501/h2/s", "/TMP/smolcorp-501/h2/s", "/var/../tmp/smolcorp-501/h2/s",
			"/System/Volumes/Data/private/tmp/smolcorp-501/h2/s"}, Write, Deny},
		{[]string{"/tmp/smolcorp-501/h1/s", "/private/tmp/smolcorp-501/h1/s"}, Write, Allow},
	} {
		for _, at := range c.paths {
			if got, by := p.Decide(at, c.op); got != c.want {
				t.Errorf("%v %q: %v by %q, want %v", c.op, at, got, by.Row, c.want)
			}
		}
	}
	// Places given in another spelling hold the same.
	decomposed := pl
	decomposed.Support = "/System/Volumes/Data/Users/Jose\u0301/Library/Application Support/AgentDeck"
	if got, _ := mustNew(t, decomposed).Decide(jose+"/Library/Application Support/AgentDeck/phones.json", Read); got != Deny {
		t.Error("a support folder given decomposed, through the firmlink, didn't hold its composed spelling")
	}
	for _, at := range []string{"relative/hook-key", "", "~/Library/Application Support/AgentDeck/hook-key"} {
		if got, _ := p.Decide(at, Read); got != Deny {
			t.Errorf("%q, not a whole path, was %v", at, got)
		}
	}
}

func TestARootInsideOrContainingTheSupportFolderIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(*Places)
		want error
	}{
		{"root inside", func(p *Places) { p.Root = realSupport + "/hires/h1" }, ErrRootInSupport},
		{"root is it", func(p *Places) { p.Root = realSupport }, ErrRootInSupport},
		{"root is home", func(p *Places) { p.Root = home }, ErrRootHasSupport},
		{"root contains", func(p *Places) { p.Root = home + "/Library" }, ErrRootHasSupport},
		{"root is the hires folder", func(p *Places) { p.Root = hires }, ErrRootHasDesks},
		{"root is the hires folder, spelled otherwise", func(p *Places) { p.Root = "/users/ADA/projects/APP-hires/h1/.." }, ErrRootHasDesks},
		{"root contains the hires folder and the checkout", func(p *Places) { p.Root = home + "/Projects" }, ErrRootHasDesks},
		{"root contains the checkout", func(p *Places) { p.Root, p.HiresFolder = home+"/Projects", "/Volumes/Work/app-hires" }, ErrRootHasDesks},
		{"root contains the hires folder", func(p *Places) { p.Root, p.Checkout, p.GitDir = home+"/Projects", "/Volumes/Work/app", "" }, ErrRootHasDesks},
		{"a desk at the project itself", func(p *Places) { p.Root = repo }, nil},
		{"root inside, spelled otherwise", func(p *Places) { p.Root = "/SYSTEM/Volumes/Data/users/ADA/Library/Application Support/agentdeck/x" }, ErrRootInSupport},
		{"root inside by ..", func(p *Places) { p.Root = hires + "/../../Library/Application Support/AgentDeck/x" }, ErrRootInSupport},
		{"checkout contains", func(p *Places) { p.Checkout = home }, ErrRootHasSupport},
		{"hires folder inside", func(p *Places) { p.HiresFolder = realSupport + "/w" }, ErrRootInSupport},
		{"GM folder contains", func(p *Places) { p.GMFolder = "/" }, ErrRootHasSupport},
		{"a test copy's root inside the realSupport folder", func(p *Places) {
			p.Support, p.OtherSupports, p.DeskStart, p.Root = copyDir, []string{realSupport}, "", realSupport+"/x"
		}, ErrRootInSupport},
		{"a test copy's root containing its own", func(p *Places) {
			p.Support, p.OtherSupports, p.DeskStart, p.Root = copyDir, []string{realSupport}, "", home+"/Projects/smolcorp"
		}, ErrRootHasSupport},
		{"root not whole", func(p *Places) { p.Root = "Projects/app" }, ErrNotWhole},
		{"no home", func(p *Places) { p.Home = "" }, ErrNotWhole},
		{"an account not whole", func(p *Places) { p.Accounts = []string{""} }, ErrNotWhole},
		{"an install root not whole", func(p *Places) { p.InstallRoots = []string{"Applications"} }, ErrNotWhole},
		{"a git config not whole", func(p *Places) { p.GitConfigs = []string{"etc/gitconfig"} }, ErrNotWhole},
		{"own sockets not whole", func(p *Places) { p.OwnSockets = "smolcorp-501/h1" }, ErrNotWhole},
		{"tmux's own folder not whole", func(p *Places) { p.TmuxFolder = "tmux-501" }, ErrNotWhole},
		{"own sockets the socket folder", func(p *Places) { p.OwnSockets = sockets }, ErrOwnSockets},
		{"own sockets outside it", func(p *Places) { p.OwnSockets = home + "/.ssh" }, ErrOwnSockets},
		{"own sockets above it", func(p *Places) { p.OwnSockets = "/private/tmp" }, ErrOwnSockets},
		{"own sockets with no socket folder", func(p *Places) { p.SocketParent = "" }, ErrOwnSockets},
		{"own id ..", func(p *Places) { p.OwnID = ".." }, ErrBadName},
		{"own id a path", func(p *Places) { p.OwnID = "h1/../h2" }, ErrBadName},
		{"no own id", func(p *Places) { p.OwnID = "" }, ErrBadName},
		{"a worktree a pattern", func(p *Places) { p.OtherWorktrees = []string{"*"} }, ErrBadName},
		{"a default branch out of refs", func(p *Places) { p.DefaultBranch = "../../config" }, ErrBadName},
		{"desk-start in another support folder", func(p *Places) {
			p.Support, p.OtherSupports, p.DeskStart = copyDir, []string{realSupport}, realSupport+"/desk-start"
		}, ErrDeskStart},
		{"desk-start, another file", func(p *Places) { p.DeskStart = realSupport + "/serve.sock" }, ErrDeskStart},
	} {
		pl := hirePlaces(realSupport)
		c.edit(&pl)
		if _, err := New(pl); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	pl := hirePlaces(realSupport)
	pl.DeskStart = sockets + "/smolcorp.desk-start.4242"
	p := mustNew(t, pl)
	if got, _ := p.Decide(pl.DeskStart, Connect); got != Allow {
		t.Error("the desk-start socket in tmux's folder can't be reached")
	}
	if got, _ := p.Decide(realSupport+"/desk-start", Connect); got != Deny {
		t.Error("a desk-start in the support folder that isn't the desk's is open")
	}
}

// Without Remote Login, the ssh keys are normal files; the plants in ~/.ssh stay write-denied.
func TestTheSSHKeysAreDeniedOnlyWhileRemoteLoginAnswers(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.RemoteLogin = false
	p := mustNew(t, pl)
	for at, want := range map[string]Verdict{home + "/.ssh/id_ed25519": Allow, home + "/.ssh/authorized_keys": Allow} {
		if got, _ := p.Decide(at, Read); got != want {
			t.Errorf("read %s: %v", at, got)
		}
	}
	if got, _ := p.Decide(home+"/.ssh/authorized_keys", Write); got != Deny {
		t.Error("authorized_keys is writable")
	}
}

// Every name the core keeps in the support folder has a row, and only the listed ones open anything.
func TestEverySupportFileHasARowAndOnlyTheListedOnesOpen(t *testing.T) {
	p := mustNew(t, hirePlaces(realSupport))
	for name := range supportFiles {
		read, _ := p.Decide(realSupport+"/"+name, Read)
		write, _ := p.Decide(realSupport+"/"+name, Write)
		connect, _ := p.Decide(realSupport+"/"+name, Connect)
		want := [3]Verdict{Deny, Deny, Deny}
		switch name {
		case "hire-guard.json", "nightshift.json":
			want[0] = Allow
		case "desk-start":
			want[2] = Allow
		}
		if got := [3]Verdict{read, write, connect}; got != want {
			t.Errorf("%s: read, write, connect %v, want %v", name, got, want)
		}
	}
	for _, name := range []string{"sandbox-canary-1", "serve.sock", "gm-memory", "hires", "nightshift.json", "nightshift.json.bad"} {
		if why, ok := SupportRow(name); !ok || why == "" {
			t.Errorf("%s has no row", name)
		}
	}
	if _, ok := SupportRow("a-file-added-later"); ok {
		t.Error("a name the Policy never named has a row")
	}
}

// Off a Mac: Linux compares bytes, Windows folds case and its spellings.
func TestOffAMacThePathsAreTheSystems(t *testing.T) {
	linux := Places{GOOS: "linux", Home: "/home/ada", Support: "/home/ada/.config/sauzzedeck-pro", OwnID: "h1", Root: "/home/ada/p-hires/h1",
		XAuthority: "/run/user/1000/xauth_x"}
	p := mustNew(t, linux)
	for at, want := range map[string]Verdict{
		"/home/ada/.config/sauzzedeck-pro/phones.json":             Deny,
		"/home/ada/p-hires/h1/../../.config/sauzzedeck-pro/x":      Deny,
		"/home/ada/.config/SAUZZEDECK-PRO/phones.json":             Allow, // another folder, on Linux
		"/home/ada/.Xauthority":                                    Deny,
		"/run/user/1000/xauth_x":                                   Deny,
		"/home/ada/.config/sauzzedeck-pro/hires/h1/STATUS.md":      Allow,
		"/home/ada/.config/sauzzedeck-pro/hires/h1/../h2/BRIEF.md": Deny,
	} {
		if got, by := p.Decide(at, Read); got != want {
			t.Errorf("linux: read %s: %v by %q, want %v", at, got, by.Row, want)
		}
	}
	windows := Places{GOOS: "windows", Home: `C:\Users\Ada`, Support: `C:\Users\Ada\AppData\Roaming\SauzzeDeck Pro`, OwnID: "h1", Root: `C:\Users\Ada\p-hires\h1`}
	w := mustNew(t, windows)
	for _, at := range []string{`C:\Users\Ada\AppData\Roaming\SauzzeDeck Pro\phones.json`, `c:/users/ada/appdata/roaming/sauzzedeck pro/PHONES.JSON`,
		`\\?\C:\Users\Ada\AppData\Roaming\SauzzeDeck Pro\hook-key`, `C:\..\..\Users\Ada\AppData\Roaming\SauzzeDeck Pro\x`,
		`C:\Users\Ada\p-hires\h1\..\..\AppData\Roaming\SauzzeDeck Pro\x`, `C:\Users\Ada\.codex\auth.json`} {
		if got, by := w.Decide(at, Read); got != Deny {
			t.Errorf("windows: read %s: %v by %q", at, got, by.Row)
		}
	}
	if got, _ := w.Decide(`C:\Users\Ada\p-hires\h1\.claude\settings.json`, Write); got != Deny {
		t.Error("windows: a fence is writable")
	}
	windows.Root = `C:\Users`
	if _, err := New(windows); !errors.Is(err, ErrRootHasSupport) {
		t.Errorf("windows: a root containing the support folder: %v", err)
	}
}

func TestDecomposedNamesCompose(t *testing.T) {
	for in, want := range map[string]string{"e\u0301": "\u00e9", "A\u030a": "\u00c5", "\u304b\u3099": "\u304c", "\u1100\u1161\u11a8": "\uac01",
		"\u1100\u1161": "\uac00", "e\u0323\u0302": "\u1ec7", "e\u0302\u0323": "\u1ec7", "\u212b": "\u00c5", "\uf900": "\u8c48",
		"plain": "plain", "\u00e9": "\u00e9"} {
		if got := composed(in); got != want {
			t.Errorf("composed(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTheDefaultBranchIsReadFromOriginsHEAD(t *testing.T) {
	git := t.TempDir()
	if DefaultBranch(git) != "" {
		t.Error("a default branch with no origin/HEAD")
	}
	file := filepath.Join(git, "refs", "remotes", "origin", "HEAD")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]string{"ref: refs/remotes/origin/develop\n": "develop", "ref: refs/remotes/origin/team/trunk": "team/trunk",
		"ref: refs/remotes/origin/../../config\n": "", "0123abcd\n": "", "ref: refs/remotes/origin/*": ""} {
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := DefaultBranch(git); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}

// B2 (RT126SAX1): deny wins everywhere. A desk whose place holds a secret, a plant or another desk's place gets none
// of them from its own place's allow; nor does one at the hires folder itself.
var holdingCases = []struct {
	root, at string
	op       Op
}{
	{home + "/.claude", home + "/.claude/.credentials.json", Read},
	{home + "/.claude", home + "/.claude/history.jsonl", Read},
	{home + "/.claude", home + "/.claude/settings.json", Write},
	{home + "/.config", home + "/.config/git/config", Write},
	{home + "/.config", home + "/.config/fish/config.fish", Write},
	{home + "/.ssh", home + "/.ssh/id_ed25519", Read},
	{home + "/.ssh", home + "/.ssh/authorized_keys", Write},
	{home + "/.codex", home + "/.codex/auth.json", Read},
	{home + "/Projects", gmDir + "/notes.md", Write},
	{home + "/Projects", hires + "/h2/main.go", Write},
	{home + "/Projects", repo + "/main.go", Write},
	{home + "/Projects", gitDir + "/hooks/pre-commit", Write},
	{home + "/Projects", gitDir + "/refs/heads/main", Write},
	{home + "/Projects", gitDir + "/config", Write},
	{home + "/Projects", gitDir + "/worktrees/h2/HEAD", Write},
	{home + "/Projects", gitDir + "/refs", Move},
	{hires, hires + "/h2/main.go", Write},
	{sockets + "/h1/../..", sockets + "/h2/s", Write},
}

// New refuses the roots that hold the hires folder or the checkout (B2′); build makes theirs, which must hold anyway.
func TestAPlaceHoldingADeniedOneOpensNothing(t *testing.T) {
	for _, c := range holdingCases {
		pl := hirePlaces(realSupport)
		pl.Root = c.root
		p := mustBuild(t, pl)
		if got, by := p.Decide(c.at, c.op); got != Deny {
			t.Errorf("root %s: %v %s: %v by %q, want deny", c.root, c.op, c.at, got, by.Row)
		}
	}
	// Its own work there still goes on.
	pl := hirePlaces(realSupport)
	pl.Root = home + "/Projects"
	if got, by := mustBuild(t, pl).Decide(home+"/Projects/notes/today.md", Write); got != Allow {
		t.Errorf("root ~/Projects: its own file: %v by %q", got, by.Row)
	}
}

// B2: the rows' order says nothing. Every case of the table, in every column, and the B2 places, give the same
// answer and name the same row with the rows reversed and shuffled.
func TestTheRowsOrderDoesntMatter(t *testing.T) {
	var policies []*Policy
	var paths []string
	for _, s := range setups() {
		policies = append(policies, mustNew(t, s.pl))
		for _, r := range rows {
			paths = append(paths, s.path(r.path))
		}
	}
	for _, root := range []string{home + "/.claude", home + "/.config", home + "/Projects", hires} {
		pl := hirePlaces(realSupport)
		pl.Root = root
		policies = append(policies, mustBuild(t, pl))
	}
	paths = append(paths, home+"/.claude/.credentials.json", home+"/.config/git/config", gitDir+"/hooks/pre-commit", gmDir+"/x")
	rng := rand.New(rand.NewPCG(126, 1))
	for i, p := range policies {
		for round := 0; round < 20; round++ {
			rules := p.Rules()
			if round == 0 {
				slices.Reverse(rules)
			} else {
				rng.Shuffle(len(rules), func(a, b int) { rules[a], rules[b] = rules[b], rules[a] })
			}
			q := &Policy{goos: p.goos, rules: rules}
			for _, at := range paths {
				for op := Read; op <= Link; op++ {
					want, wantBy := p.Decide(at, op)
					if got, by := q.Decide(at, op); got != want || by.Row != wantBy.Row {
						t.Fatalf("policy %d, order %d: %v %s: %v by %q, in the made order %v by %q", i, round, op, at, got, by.Row, want, wantBy.Row)
					}
				}
			}
		}
	}
}

// B1: for every row that denies anything, each folder above a place it holds can't be moved, nor made a link (B1′), up
// to the top: a pattern's own folders included (a sample name stands for each pattern).
func TestEveryFolderAboveADeniedPlaceCantBeMoved(t *testing.T) {
	sample := strings.NewReplacer("**", "a/b", "*", "x", "?", "x")
	for _, s := range setups() {
		p := mustNew(t, s.pl)
		checked := 0
		for _, r := range p.Rules() {
			if r.Path == "" || !r.denies() {
				continue
			}
			at := r.Path
			if r.Tail != "" {
				at += "/" + sample.Replace(r.Tail)
			}
			for folder := parent(at); ; folder = parent(folder) {
				checked++
				for _, op := range []Op{Move, Link} {
					if got, by := p.Decide(folder, op); got != Deny {
						t.Errorf("%s: %q (%s) holds %s, but %s: %v %s (by %q)", s.name, r.Row, r.Path, at, op, folder, got, by.Row)
					}
				}
				if parent(folder) == folder {
					break
				}
			}
		}
		if checked < 100 {
			t.Errorf("%s: only %d folders checked", s.name, checked)
		}
	}
}

// B1′ (RT126SAX1R2): a folder above a denied place that doesn't exist yet could be brought in whole, a denied file
// inside, by a rename or as a link. Hold names each one the Policy lets a desk write, and MakeHold makes the missing
// ones, empty and owner-only, so the Move and Link denies hold them. A place denied itself, or a file, isn't in it.
func TestTheMissingFoldersAboveADeniedPlaceAreMadeFirst(t *testing.T) {
	for _, s := range setups() {
		p := mustNew(t, s.pl)
		held := p.Hold()
		for _, want := range []string{home + "/.ssh", home + "/.config", home + "/.config/systemd", home + "/.codex",
			home + "/.gemini", gitDir + "/refs/heads", s.pl.Support + "/hires/" + s.pl.OwnID + "/claude", home + "/.config/standin"} {
			if !slices.Contains(held, want) {
				t.Errorf("%s: %s isn't in Hold", s.name, want)
			}
		}
		for _, not := range []string{home + "/.zshrc", home + "/.config/git", home + "/.ssh/rc", home + "/.codex/auth.json", home,
			s.pl.Support, realSupport, hires, repo, gitDir, gitDir + "/refs", gitDir + "/objects", gitDir + "/hooks", gitDir + "/modules", gitDir + "/modules/sub",
			gitDir + "/modules/third_party", gitDir + "/modules/third_party/bar", "/", gitDir + "/reftable", home + "/.githooks",
			hires + "/h1/.husky/_", hires + "/h1/.husky", home + "/dotfiles/hooklib"} {
			if slices.Contains(held, not) {
				t.Errorf("%s: %s is in Hold", s.name, not)
			}
		}
		for _, folder := range held {
			if v, by := p.Decide(folder, Write); v == Deny {
				t.Errorf("%s: %s is in Hold, but no desk may write it (%q)", s.name, folder, by.Row)
			}
			for _, op := range []Op{Move, Link} {
				if v, by := p.Decide(folder, op); v != Deny {
					t.Errorf("%s: %s is in Hold, but %v is %v (by %q)", s.name, folder, op, v, by.Row)
				}
			}
		}
	}

	// MakeHold makes the missing ones, owner-only and empty, and leaves what exists alone, a file or a link included.
	dir := t.TempDir()
	fake := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(fake, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, ".codex"), []byte("standin"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(fake, ".gemini")); err != nil {
		t.Fatal(err)
	}
	folders := []string{filepath.Join(fake, ".config"), filepath.Join(fake, ".config", "systemd"), filepath.Join(fake, ".ssh"),
		filepath.Join(fake, ".codex"), filepath.Join(fake, ".gemini")}
	if err := MakeHold(folders); err != nil {
		t.Fatal(err)
	}
	for _, folder := range folders[1:3] {
		info, err := os.Lstat(folder)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v, want a folder, owner-only", folder, info, err)
		}
		if names, _ := os.ReadDir(folder); len(names) != 0 {
			t.Errorf("%s isn't empty", folder)
		}
	}
	if info, _ := os.Lstat(folders[0]); info.Mode().Perm() != 0o755 {
		t.Errorf("an existing folder was changed: %v", info.Mode())
	}
	if data, _ := os.ReadFile(folders[3]); string(data) != "standin" {
		t.Error("a file at a hold folder was changed")
	}
	if info, _ := os.Lstat(folders[4]); info.Mode()&os.ModeSymlink == 0 {
		t.Error("a link at a hold folder was changed")
	}
	if _, err := os.Lstat(filepath.Join(dir, "elsewhere")); err == nil {
		t.Error("MakeHold followed a link")
	}
	if err := MakeHold([]string{filepath.Join(folders[3], "below-a-file")}); err == nil {
		t.Error("a folder that can't be made didn't fail")
	}
}

// seatbelt is a profile written in the order Rules() documents, read as Seatbelt reads one: everything allowed, then
// each row in turn, the last that matches deciding. exception says which allows go after the Wide denies.
type seatbelt struct {
	p    *Policy
	rows [Link + 1][]Rule
}

func profileOf(p *Policy, exception func(Rule) bool) seatbelt {
	s := seatbelt{p: p}
	rules := p.Rules()
	isException := make([]bool, len(rules))
	for i, r := range rules {
		isException[i] = exception(r)
	}
	for op := Read; op <= Link; op++ {
		var plain, wide, exceptions, final []Rule
		for i, r := range rules {
			switch v := r.verdict(op); {
			case v == Unset:
			case v == Allow && isException[i]:
				exceptions = append(exceptions, r)
			case v == Allow:
				plain = append(plain, r)
			case r.Wide:
				wide = append(wide, r)
			default:
				final = append(final, r)
			}
		}
		s.rows[op] = slices.Concat(plain, wide, exceptions, final)
	}
	return s
}

func (s seatbelt) says(at string, op Op) Verdict {
	k := key(s.p.goos, at)
	said := Allow
	for _, r := range s.rows[op] {
		if r.matches(k) {
			said = r.verdict(op)
		}
	}
	return said
}

// B2′ (RT126SAX1R2): the documented profile order, played as Seatbelt reads it, says what Decide says, for every case
// of the table in every column, the B2 places and their roots, the roots New refuses for holding desks included, a desk
// at the project. A row that only names a Wide deny it doesn't open is a plain allow: put after the Wide denies, it
// opens them whole.
func TestAProfileInTheDocumentedOrderSaysWhatDecideSays(t *testing.T) {
	type policy struct {
		name string
		p    *Policy
	}
	var policies []policy
	var paths []string
	for _, s := range setups() {
		policies = append(policies, policy{s.name, mustNew(t, s.pl)})
		for _, r := range rows {
			paths = append(paths, s.path(r.path))
		}
	}
	atProject := hirePlaces(realSupport)
	atProject.Root = repo
	policies = append(policies, policy{"a desk at the project", mustNew(t, atProject)})
	for _, c := range holdingCases {
		paths = append(paths, c.at)
		pl := hirePlaces(realSupport)
		pl.Root = c.root
		if !slices.ContainsFunc(policies, func(p policy) bool { return p.name == "root "+c.root }) {
			policies = append(policies, policy{"root " + c.root, mustBuild(t, pl)})
		}
	}
	checked := 0
	for _, p := range policies {
		profile := profileOf(p.p, p.p.Exception)
		for _, at := range paths {
			for op := Read; op <= Link; op++ {
				checked++
				want, by := p.p.Decide(at, op)
				if got := profile.says(at, op); got != want {
					t.Errorf("%s: %v %s: the profile says %v, Decide %v by %q", p.name, op, at, got, want, by.Row)
				}
			}
		}
	}
	if checked < 10000 {
		t.Errorf("only %d cases checked", checked)
	}
	// The order RT126SAX1R2 found: every row naming a Wide deny goes after them. A root that holds the hires folder or
	// the checkout, or is the hires folder, then writes in them.
	named := func(r Rule) bool { return len(r.Opens) > 0 }
	for _, c := range []struct{ root, at string }{{home + "/Projects", hires + "/h2/main.go"}, {home + "/Projects", repo + "/main.go"},
		{hires, hires + "/h2/main.go"}} {
		pl := hirePlaces(realSupport)
		pl.Root = c.root
		p := mustBuild(t, pl)
		if _, err := New(pl); !errors.Is(err, ErrRootHasDesks) {
			t.Errorf("root %s: New gave %v, want ErrRootHasDesks", c.root, err)
		}
		if got := profileOf(p, named).says(c.at, Write); got != Allow {
			t.Errorf("root %s: the old order gives %v for %s: the check can't tell it from the fixed one", c.root, got, c.at)
		}
		if got := profileOf(p, p.Exception).says(c.at, Write); got != Deny {
			t.Errorf("root %s: write %s: the profile says %v", c.root, c.at, got)
		}
	}
}

// B1‴ (RT126SAX1R4): no desk writes anything in .git/modules, a submodule's git folder included. The reviewer's probes
// (a folder brought into a git folder, at any depth, and the git folder's HEAD, whose one write let git take it), R3's,
// and their neighbours, in Decide and in the profile read as Seatbelt reads it: in every column, a desk in its own
// worktree, and for a desk at the project, where the shared .git's allow is a plain one. A worktree's own submodules
// live in its registration, worktrees/<id>/modules, and stay its own; another desk's don't.
func TestInGitModulesNoDeskWritesAnything(t *testing.T) {
	probes := []struct {
		at                  string
		inWorktree, project Verdict
	}{
		{"/modules", D, D}, {"/modules/sub", D, D}, {"/modules/sub/config", D, D}, {"/modules/sub/hooks/x", D, D},
		{"/modules/sub/evil", D, D}, {"/modules/sub/evil/config", D, D}, {"/modules/sub/evil/hooks/post-checkout", D, D},
		{"/modules/sub/refs/heads/x", D, D}, {"/modules/sub/refs/heads/x/config", D, D}, {"/modules/sub/objects/zz", D, D},
		{"/modules/third_party/bar/evil", D, D}, {"/modules/third_party/bar/evil/config", D, D},
		{"/modules/sub/HEAD", D, D}, {"/modules/sub/commondir", D, D}, {"/modules/sub/objects", D, D},
		{"/modules/sub/objects/x", D, D}, {"/modules/sub/refs/heads/topic", D, D}, {"/modules/sub/index", D, D},
		{"/modules/sub/modules", D, D}, {"/modules/sub/modules/x", D, D}, {"/modules/sub/modules/x/config", D, D},
		{"/modules/third_party", D, D}, {"/modules/third_party/evil", D, D}, {"/modules/third_party/evil/config", D, D},
		{"/modules/new", D, D}, {"/modules/new/objects/x", D, D}, {"/modules/third_party/bar", D, D},
		{"/modules/third_party/bar/objects/x", D, D}, {"/modules/MODULES/SUB/HEAD", D, D}, {"/modules/sub/../third_party/evil", D, D},
		{"/worktrees/$WT/modules/sub/objects/x", A, D}, {"/worktrees/$WT/modules/sub/config", A, D},
		{"/worktrees/$WT/modules/sub/HEAD", A, D}, {"/worktrees/h2/modules/sub/objects/x", D, D},
	}
	// A desk at the project has no registration of its own: h1's is another desk's.
	atProject := setup{"a desk at the project", hirePlaces(realSupport)}
	atProject.pl.Root, atProject.pl.Checkout, atProject.pl.OwnWorktree = repo, "", ""
	for _, s := range append(setups(), atProject) {
		p := mustNew(t, s.pl)
		profile := profileOf(p, p.Exception)
		for _, c := range probes {
			at, want := s.path(strings.Replace(gitDir+c.at, "$WT", cmp.Or(s.pl.OwnWorktree, "h1"), 1)), c.inWorktree
			if s.pl.Root == repo {
				want = c.project
			}
			if got, by := p.Decide(at, Write); got != want {
				t.Errorf("%s: write %s: %v by %q, want %v", s.name, at, got, by.Row, want)
			}
			if got := profile.says(at, Write); got != want {
				t.Errorf("%s: write %s: the profile says %v, want %v", s.name, at, got, want)
			}
		}
	}
}

// B3 (RT126SAX1R5): no desk changes the entries that make the shared .git a git folder, a desk at the project included,
// in Decide and in the profile read as Seatbelt reads it, nor the owner's rebase state. What git makes inside them is
// still the desk's.
func TestNoDeskMakesTheSharedGitStopBeingAGitFolder(t *testing.T) {
	probes := []struct {
		at   string
		want Verdict
	}{
		{"", D}, {"/objects", D}, {"/refs", D}, {"/HEAD", D}, {"/OBJECTS", D}, {"/refs/../objects", D}, {"/./refs", D},
		{"/rebase-merge", D}, {"/rebase-merge/git-rebase-todo", D}, {"/rebase-apply/next", D}, {"/sequencer/todo", D},
		{"/objects/ab", A}, {"/objects/ab/cdef", A}, {"/refs/heads/topic", A}, {"/refs/tags/v1", A}, {"/ORIG_HEAD", A},
	}
	atProject := setup{"a desk at the project", hirePlaces(realSupport)}
	atProject.pl.Root, atProject.pl.Checkout, atProject.pl.OwnWorktree = repo, "", ""
	for _, s := range append(setups(), atProject) {
		p := mustNew(t, s.pl)
		profile := profileOf(p, p.Exception)
		for _, c := range probes {
			at := gitDir + c.at
			if got, by := p.Decide(at, Write); got != c.want {
				t.Errorf("%s: write %s: %v by %q, want %v", s.name, at, got, by.Row, c.want)
			}
			if got := profile.says(at, Write); got != c.want {
				t.Errorf("%s: write %s: the profile says %v, want %v", s.name, at, got, c.want)
			}
		}
	}
}

// B1″ (RT126SAX1R3): every folder above a denied pattern place, with several names for each `**` and `*`, is in Hold
// or denied writing, so a desk can't bring it in whole with the denied place already inside. The `**` names are the
// reviewers' (B1″, B1‴), for a row with a `**` that holds a place; `*` takes `h1` and `added-later` in worktrees/.
func TestEveryFolderAboveADeniedPatternPlaceIsHeldOrDenied(t *testing.T) {
	var samples func(names []string) []string
	samples = func(names []string) []string {
		if len(names) == 0 {
			return []string{""}
		}
		var heads []string
		switch {
		case names[0] == "**":
			heads = []string{"", "a", "sub", "sub/modules/x", "third_party/evil", "third_party/bar", "sub/evil", "third_party/bar/evil"}
		case strings.ContainsAny(names[0], "*?"):
			for _, name := range []string{"x", "sub", "evil", "h1", "added-later"} {
				heads = append(heads, strings.NewReplacer("*", name, "?", "x").Replace(names[0]))
			}
		default:
			heads = []string{names[0]}
		}
		var out []string
		for _, head := range heads {
			for _, rest := range samples(names[1:]) {
				out = append(out, strings.Trim(head+"/"+rest, "/"))
			}
		}
		return out
	}
	for _, s := range setups() {
		p := mustNew(t, s.pl)
		held := p.Hold()
		checked := 0
		for _, r := range p.Rules() {
			if r.Path == "" || r.Tail == "" || !r.denies() || r.Row == rowAbove {
				continue
			}
			for _, sample := range samples(strings.Split(r.Tail, "/")) {
				at := strings.TrimSuffix(r.Path+"/"+sample, "/")
				for folder := parent(at); ; folder = parent(folder) {
					checked++
					if v, by := p.Decide(folder, Write); v != Deny && !slices.Contains(held, folder) {
						t.Errorf("%s: %q holds %s, but %s is neither in Hold nor denied writing (%v by %q)", s.name, r.Row, at, folder, v, by.Row)
					}
					if parent(folder) == folder {
						break
					}
				}
			}
		}
		if checked < 200 {
			t.Errorf("%s: only %d folders checked", s.name, checked)
		}
	}
}

// FXTMUXDIR: tmux's own folder is a Mac row. A Linux desk's view has a /tmp of its own (bwrap.go's tmpfs), so the
// owner's tmux folder isn't in it, and the Policy gives no row there that the view would bind in.
func TestTmuxsOwnFolderIsAMacRow(t *testing.T) {
	mac := hirePlaces(realSupport)
	if v, r := mustNew(t, mac).Decide(tmuxFolder+"/smolcorp-h2", Write); v != Deny || r.Row != rowTmuxFolder {
		t.Errorf("on a Mac, a write in tmux's own folder: %v by %q", v, r.Row)
	}
	mac.TmuxFolder = ""
	if v, _ := mustNew(t, mac).Decide(tmuxFolder+"/smolcorp-h2", Write); v != Allow {
		t.Errorf("with no tmux folder given, a write there: %v", v)
	}
	linux := linuxPlaces(lHome + "/.local/share/smolcorp")
	linux.TmuxFolder = "/tmp/tmux-1001"
	for _, r := range mustNew(t, linux).Rules() {
		if r.Row == rowTmuxFolder {
			t.Errorf("a Linux Policy has a row in tmux's own folder: %+v", r)
		}
	}
}

// FXREPLACEREFS: on a Mac no desk writes what makes the owner's git read a base branch as another tree or history
// while the branch never moves: a replace ref, the shallow list, a multi-pack index, a base branch's reflog; and the
// grafts, the other stores' lists and the commit-graph stay the shared git places'. What a desk's commits, merges and
// gc write is still its work: objects, packs and their indexes, its own branch and reflog, tags, notes.
func TestGitsSubstitutionsOfABaseBranchAreNoDesksOnAMac(t *testing.T) {
	cases := []struct {
		path string
		v    Verdict
		row  string
	}{
		{"$G/refs/replace", D, rowSubstitutions}, {"$G/refs/replace/0123456789abcdef0123456789abcdef01234567", D, rowSubstitutions},
		{"$G/refs/replace/sub/x", D, rowSubstitutions}, {"$G/Refs/Replace/x", D, rowSubstitutions},
		{"$G/shallow", D, rowSubstitutions}, {"$G/objects/pack/multi-pack-index", D, rowSubstitutions},
		{"$G/objects/pack/multi-pack-index.lock", D, rowSubstitutions}, {"$G/objects/pack/multi-pack-index-ab12.bitmap", D, rowSubstitutions},
		{"$G/objects/pack/multi-pack-index-ab12.rev", D, rowSubstitutions}, {"$G/objects/pack/multi-pack-index.d/multi-pack-index-ab12.midx", D, rowSubstitutions},
		{"$G/logs/refs/heads/main", D, rowBaseReflogs}, {"$G/logs/refs/heads/master", D, rowBaseReflogs},
		{"$G/logs/refs/heads/develop", D, rowBaseReflogs}, {"$G/logs/refs/heads/release/1.26", D, rowBaseReflogs},
		{"$G/logs/refs/heads/release", D, rowBaseReflogs},
		// The shared git places' already: the grafts, the other stores' lists, the commit-graph.
		{"$G/info/grafts", D, "the shared git places"}, {"$G/objects/info/commit-graph", D, "the shared git places"},
		{"$G/objects/info/commit-graphs/commit-graph-chain", D, "the shared git places"},
		{"$G/objects/info/commit-graphs/graph-ab12.graph", D, "the shared git places"},
		// A desk's own work.
		{"$G/objects/ab/cdef", A, ""}, {"$G/objects/pack/pack-ab12.pack", A, ""}, {"$G/objects/pack/pack-ab12.idx", A, ""},
		{"$G/objects/pack/pack-ab12.rev", A, ""}, {"$G/objects/pack/pack-ab12.bitmap", A, ""}, {"$G/objects/pack/tmp_pack_x", A, ""},
		{"$G/refs/heads/hire/h1/work", A, ""}, {"$G/logs/refs/heads/hire/h1/work", A, ""}, {"$G/logs/refs/heads/mainline", A, ""},
		{"$G/refs/tags/v1", A, ""}, {"$G/refs/notes/commits", A, ""}, {"$G/refs/replaced", A, ""},
		// The shallow list's lock, which git renames onto it: denied as it is (FXLOCKSIBLINGS flips this pin).
		{"$G/shallow.lock", D, rowSubstitutions},
	}
	for _, s := range setups() {
		p := mustNew(t, s.pl)
		for _, c := range cases {
			v, r := p.Decide(s.path(c.path), Write)
			if v != c.v || c.row != "" && r.Row != c.row {
				t.Errorf("%s: write %s: %v by %q, want %v by %q", s.name, c.path, v, r.Row, c.v, c.row)
			}
		}
		// The folders above them hold: a desk can't move one aside, write under its new name and move it back.
		for _, folder := range []string{"$G/objects/pack", "$G/logs/refs/heads", "$G/logs/refs", "$G/logs", "$G/refs/replace"} {
			if v, r := p.Decide(s.path(folder), Move); v != Deny {
				t.Errorf("%s: move %s: %v by %q", s.name, folder, v, r.Row)
			}
		}
	}
	// Linux's view and Windows' jail are made from other rows (FXREPLACEREFS' report lists them for the VM).
	for _, pl := range []Places{linuxPlaces(lHome + "/.local/share/smolcorp"), {GOOS: "windows", Subject: Hire, Home: `C:\Users\ada`,
		Support: `C:\Users\ada\AppData\Roaming\Smolcorp`, OwnID: "h1", Root: `C:\Users\ada\Projects\app-hires\h1`,
		Checkout: `C:\Users\ada\Projects\app`, GitDir: `C:\Users\ada\Projects\app\.git`, OwnWorktree: "h1"}} {
		for _, r := range mustNew(t, pl).Rules() {
			if r.Row == rowSubstitutions || r.Row == rowBaseReflogs {
				t.Errorf("a %s Policy has a row of git's substitutions: %+v", pl.GOOS, r)
			}
		}
	}
}

// FXLOCKSIBLINGS (RT126REPLACEREFS' finding): on a Mac, each name a denied file's writers write first and then rename
// onto it is denied as the file is, by the file's own row. Seatbelt checks a write when a file is opened, so a desk
// that opened one of them while the owner's program had it there would write the file through it once renamed. One
// class per subtest: git's lock beside the shared git places, beside the base branches (and packed-refs' new copy),
// beside git's substitutions; the owner's programs' names beside a plant and git's other config; beside a fence; a
// sign-in's copy, which isn't read either; and the core's own names in the support folder, which its row holds whole
// already. A desk's own work beside them stays its own.
func TestADeniedFilesWritersNamesBesideItAreDeniedAsItIsOnAMac(t *testing.T) {
	type c struct {
		path string
		op   Op
		v    Verdict
		row  string
	}
	const plants, fences, ownFence = "plants the owner's programs run later", "fences and hooks", "its own Claude folder's fence"
	classes := []struct {
		name  string
		cases []c
	}{
		{"git's lock beside the shared git places", []c{
			{"$G/config.lock", Write, D, "the shared git places"}, {"$G/config.worktree.lock", Write, D, "the shared git places"},
			{"$G/index.lock", Write, D, "the shared git places"}, {"$G/HEAD.lock", Write, D, "the shared git places"},
			{"$G/commondir.lock", Write, D, "the shared git places"}, {"$G/gitdir.lock", Write, D, "the shared git places"},
			{"$G/Config.LOCK", Write, D, "the shared git places"},
			{"$G/worktrees/$WT/config.worktree.lock", Write, D, "the shared git places: where a worktree finds them"},
			{"$G/worktrees/$WT/commondir.lock", Write, D, "the shared git places: where a worktree finds them"},
			{"$G/worktrees/$WT/gitdir.lock", Write, D, "the shared git places: where a worktree finds them"},
		}},
		{"git's lock and new copy beside the base branches", []c{
			{"$G/packed-refs.new", Write, D, "the base branches"},
			{"$G/refs/heads/main.lock", Write, D, "the base branches"}, {"$G/refs/heads/master.lock", Write, D, "the base branches"},
			{"$G/refs/heads/develop.lock", Write, D, "the base branches"}, {"$G/refs/heads/release/1.26.lock", Write, D, "the base branches"},
		}},
		{"git's lock beside git's substitutions", []c{
			{"$G/shallow.lock", Write, D, rowSubstitutions}, {"$G/refs/replace/0123.lock", Write, D, rowSubstitutions},
			{"$G/objects/pack/multi-pack-index.lock", Write, D, rowSubstitutions},
			{"$G/logs/refs/heads/main.lock", Write, D, rowBaseReflogs}, {"$G/logs/refs/heads/master.lock", Write, D, rowBaseReflogs},
			{"$G/logs/refs/heads/develop.lock", Write, D, rowBaseReflogs},
		}},
		{"the owner's programs' names beside a plant", []c{
			{"$H/.gitconfig.lock", Write, D, plants}, {"$H/.zshrc.tmp.4242.1763400000000", Write, D, plants},
			{"$H/.zshenv___jb_tmp___", Write, D, plants}, {"$H/.bash_profile.new", Write, D, plants}, {"$H/.profile.1a2b3c4d.tmp", Write, D, plants},
			{"$H/.tmux.conf.new", Write, D, plants}, {"$H/.ssh/config.lock", Write, D, plants}, {"$H/.ssh/rc.tmp", Write, D, plants},
			{"/opt/homebrew/etc/gitconfig.lock", Write, D, plants + ": git's other config"},
			{"$H/.xdg/git/config.lock", Write, D, plants + ": git's other config"},
			{"$H/.gitconfig-work.lock", Write, D, plants + ": what git's config names"},
			{"/opt/standin/nc.tmp", Write, D, plants + ": what git's config names"},
		}},
		{"the writers' names beside a fence", []c{
			{"$ROOT/.claude/settings.local.json.tmp", Write, D, fences}, {"$ROOT/.claude/settings.json.tmp.4242.1763400000000", Write, D, fences},
			{"$ROOT/sub/.claude/settings.local.json.lock", Write, D, fences},
			{"$S/hires/$ID/claude/settings.json.tmp.4242.1763400000000", Write, D, ownFence},
			{"$S/hires/$ID/claude/.settings.json.fresh-12345", Write, D, ownFence},
			{"$S/hires/$ID/claude/settings.local.json.lock", Write, D, ownFence},
		}},
		{"a sign-in's copy isn't read or written", []c{
			{"$H/.claude/.credentials.json.tmp.4242.1763400000000", Read, D, "the account folders' tokens and history"},
			{"$H/.claude-work/.credentials.json.lock", Read, D, "the account folders' tokens and history"},
			{"$H/.claude/history.jsonl.tmp", Read, D, "the account folders' tokens and history"},
			{"$H/.codex/auth.json.tmp", Read, D, "other agents' sign-ins"}, {"$H/.codex/auth.json.tmp", Write, D, "other agents' sign-ins"},
			{"$H/.gemini/oauth_creds.json.tmp.1", Read, D, "other agents' sign-ins"},
		}},
		// The support folder's row holds these whole already: none of the core's names ends in .md.
		{"the core's own names in the support folder", []c{
			{"$S/hires/$ID/.BRIEF.md.0123abcd", Write, D, rowSupport}, {"$S/hires/$ID/.BRIEF.md.fresh-12345", Write, D, rowSupport},
			{"$S/hires/$ID/.settings.json.0123abcd", Write, D, rowSupport}, {"$S/hires/$ID/.sandbox-12345.sb", Write, D, rowSupport},
			{"$S/hires/$ID/.BRIEF.md.0123abcd", Read, D, rowSupport}, {"$S/.hire-guard.json.0123abcd", Write, D, rowSupport},
			{"$S/hire-guard.json.tmp", Read, D, rowSupport}, {"$S/config.json.tmp", Write, D, rowSupport},
			{"$S/nightshift.json.tmp", Write, D, rowSupport},
		}},
		{"a desk's own work beside them stays its own", []c{
			{"$G/refs/heads/hire/h1/work.lock", Write, A, ""}, {"$G/logs/refs/heads/hire/h1/work.lock", Write, A, ""},
			// Held, never renamed onto packed-refs; a desk's commit, rebase and branch deletion take it.
			{"$G/packed-refs.lock", Write, A, ""},
			{"$G/worktrees/$WT/index.lock", Write, A, ""}, {"$G/worktrees/$WT/HEAD.lock", Write, A, ""},
			{"$G/worktrees/$WT/ORIG_HEAD.lock", Write, A, ""}, {"$G/refs/tags/v2.lock", Write, A, ""},
			{"$G/refs/remotes/origin/main.lock", Write, A, ""}, {"$G/refs/heads/mainline.lock", Write, A, ""},
			{"$G/objects/pack/tmp_pack_x", Write, A, ""}, {"$G/objects/ab/tmp_obj_x", Write, A, ""},
			{"$ROOT/.claude/notes.md", Write, A, ""}, {"$ROOT/settings.json.tmp", Write, A, ""},
			{"$S/hires/$ID/claude/.claude.json.tmp.4242.1763400000000", Write, A, ""}, {"$S/hires/$ID/STATUS.md", Write, A, ""},
			{"$H/.zsh_history", Write, A, ""}, {"$H/.bash_history", Write, A, ""}, {"$H/.claude/CLAUDE.md", Read, A, ""},
			// A plant's neighbour, and an editor's backup of it: the old copy, which nothing renames back.
			{"$H/.zshrc-notes", Write, A, ""}, {"$H/.gitconfig-work.bak", Write, A, ""}, {"$H/.zshrc~", Write, A, ""},
		}},
		// FXLOCKSIBLINGSR1 (RT126LOCKSIBLINGS' B1, widened by SIERRA): macOS's sed -i's `.!<pid>!<name>` and mise's
		// `.<name>.tmp.<pid>.<n>` beside each plant and git config, install(1)'s `INS@` and perl -i's `XX` names in
		// their folders, and oh-my-zsh's two names beside ~/.zshrc.
		{"the owner's tools' other names beside a plant", []c{
			{"$H/.!40818!.zshrc", Write, D, plants}, {"$H/.!1!.gitconfig", Write, D, plants}, {"$H/.!7!.bash_profile", Write, D, plants},
			{"$H/.ssh/.!7!config", Write, D, plants}, {"$H/.!7!.ZSHRC", Write, D, plants},
			{"/opt/homebrew/etc/.!9!gitconfig", Write, D, plants + ": git's other config"},
			{"$H/.xdg/git/.!9!config", Write, D, plants + ": git's other config"},
			{"$H/.!9!.gitconfig-work", Write, D, plants + ": what git's config names"}, {"/opt/standin/.!9!nc", Write, D, plants + ": what git's config names"},
			{"$H/..zshrc.tmp.42164.1", Write, D, plants}, {"$H/..profile.tmp.1.0", Write, D, plants}, {"$H/.ssh/.rc.tmp.1.0", Write, D, plants},
			{"/opt/homebrew/etc/.gitconfig.tmp.1.0", Write, D, plants + ": git's other config"},
			{"$H/..gitconfig-work.tmp.1.0", Write, D, plants + ": what git's config names"},
			{"$H/INS@q4Ontc", Write, D, plants}, {"$H/ins@Q4ONTC", Write, D, plants}, {"$H/.ssh/XXNu6Xnm", Write, D, plants},
			{"/opt/homebrew/etc/INS@q4Ontc", Write, D, plants + ": git's other config"}, {"$H/.xdg/git/XXNu6Xnm", Write, D, plants + ": git's other config"},
			{"/opt/standin/XXNu6Xnm", Write, D, plants + ": what git's config names"},
			{"$H/.config/standin/INS@q4Ontc", Write, D, plants + ": what git's config names"},
			{"$H/.zshrc-omztemp", Write, D, plants}, {"$H/.zshrc.pre-oh-my-zsh", Write, D, plants},
		}},
		{"a desk's own names beside the owner's tools' stay its own", []c{
			{"$H/.!x.zshrc", Write, A, ""}, {"$H/.zshrc.!1!", Write, A, ""},
			{"$H/XXshort", Write, A, ""}, {"$H/XXtoolong1", Write, A, ""}, {"$H/INS@abc", Write, A, ""}, {"$H/Projects/INS@q4Ontc", Write, A, ""},
			{"$H/Projects/XXNu6Xnm", Write, A, ""}, {"$ROOT/XXNu6Xnm", Write, A, ""}, {"$ROOT/INS@q4Ontc", Write, A, ""},
			{"$ROOT/.!1!main.go", Write, A, ""}, {"$H/.bashrc-omztemp", Write, A, ""}, {"$H/..zsh_history.tmp.1.0", Write, A, ""},
			// perl -i's directly in ~ (SIERRA's ruling on RT126LOCKSIBLINGSR2's B2; the report lists its race, R1).
			{"$H/XXNu6Xnm", Write, A, ""}, {"$H/xxnu6xnm", Write, A, ""},
		}},
	}
	for _, class := range classes {
		t.Run(class.name, func(t *testing.T) {
			for _, s := range setups() {
				p := mustNew(t, s.pl)
				for _, c := range class.cases {
					v, r := p.Decide(s.path(c.path), c.op)
					if v != c.v || c.row != "" && r.Row != c.row {
						t.Errorf("%s: %v %s: %v by %q, want %v by %q", s.name, c.op, c.path, v, r.Row, c.v, c.row)
					}
				}
			}
		})
	}
	// Linux's view and Windows' jail are made from other rows (the report lists them for the VM).
	for _, pl := range []Places{linuxPlaces(lHome + "/.local/share/smolcorp"), {GOOS: "windows", Subject: Hire, Home: `C:\Users\ada`,
		Support: `C:\Users\ada\AppData\Roaming\Smolcorp`, OwnID: "h1", Root: `C:\Users\ada\Projects\app-hires\h1`,
		Checkout: `C:\Users\ada\Projects\app`, GitDir: `C:\Users\ada\Projects\app\.git`, OwnWorktree: "h1"}} {
		for _, r := range mustNew(t, pl).Rules() {
			if strings.HasSuffix(r.Path+r.Tail, gitLock) || strings.HasSuffix(r.Path+r.Tail, gitNew) || strings.HasSuffix(r.Tail, ".json*") && r.Row != "the accounts' .claude.json" {
				t.Errorf("a %s Policy has a row of a writer's name beside a file: %+v", pl.GOOS, r)
			}
			for _, name := range append(append([]string{".!*!", ".tmp.*"}, ownersInFolder...), omzNames...) {
				if strings.Contains(r.Path+"/"+r.Tail, name) {
					t.Errorf("a %s Policy has a row of an owner's tool's name beside a plant: %+v", pl.GOOS, r)
				}
			}
		}
	}
}

// FXLOCKSIBLINGSR2 (RT126LOCKSIBLINGSR2's B2): install(1)'s and perl -i's names in a folder are no row at or under a
// place a desk works in (its root, the owner's checkout, the hires and GM folders), even when the owner's git config
// names files there and in a folder below, so a desk's own install and perl -pi there still work. What git's config
// names there is still denied, and so are the names beside it. Away from those places they stay: in ~ install(1)'s
// (perl -i's isn't, SIERRA's ruling), in ~/.ssh both, and beside what git's config names elsewhere.
func TestTheOwnersToolsNamesInAFolderAreNoRowWhereADeskWorksOnAMac(t *testing.T) {
	const plants = "plants the owner's programs run later"
	const named = plants + ": what git's config names"
	for _, s := range profileSetups() {
		pl := s.pl
		root := pl.Root
		pl.GitConfigs = append(slices.Clone(pl.GitConfigs), root+"/etc/gitconfig")
		pl.GitNamed = append(slices.Clone(pl.GitNamed), root+"/.gitconfig", root+"/scripts/pre-commit.sh", repo+"/.gitconfig",
			repo+"/scripts/pre-commit.sh", hires+"/h2/scripts/pre-commit.sh", gmDir+"/.gitconfig")
		p := mustNew(t, pl)
		work := []string{key("darwin", root), key("darwin", repo), key("darwin", hires), key("darwin", gmDir)}
		for _, r := range p.Rules() {
			if !slices.Contains(ownersInFolder, path.Base(r.Tail)) {
				continue
			}
			for _, w := range work {
				if under(key("darwin", r.Path), w) {
					t.Errorf("%s: a row of an owner's tool's name in a place a desk works in: %+v", s.name, r)
				}
			}
		}
		for _, c := range []struct {
			path string
			v    Verdict
			row  string
		}{
			{root + "/XXNu6Xnm", A, ""}, {root + "/INS@q4Ontc", A, ""}, {root + "/scripts/XXNu6Xnm", A, ""},
			{root + "/scripts/INS@q4Ontc", A, ""}, {root + "/etc/XXNu6Xnm", A, ""}, {root + "/etc/INS@q4Ontc", A, ""},
			{root + "/.gitconfig", D, named}, {root + "/.!9!.gitconfig", D, named}, {root + "/.gitconfig.lock", D, named},
			{root + "/scripts/pre-commit.sh", D, named}, {root + "/scripts/.!9!pre-commit.sh", D, named},
			{root + "/scripts/.pre-commit.sh.tmp.1.0", D, named},
			{root + "/etc/gitconfig", D, plants + ": git's other config"}, {root + "/etc/.!9!gitconfig", D, plants + ": git's other config"},
			{home + "/XXNu6Xnm", A, ""}, {home + "/INS@q4Ontc", D, plants}, {home + "/.ssh/XXNu6Xnm", D, plants},
			{home + "/.ssh/INS@q4Ontc", D, plants}, {"/opt/standin/XXNu6Xnm", D, named}, {home + "/.config/standin/XXNu6Xnm", D, named},
			{"/opt/homebrew/etc/XXNu6Xnm", D, plants + ": git's other config"},
		} {
			if v, r := p.Decide(c.path, Write); v != c.v || c.row != "" && r.Row != c.row {
				t.Errorf("%s: write %s: %v by %q, want %v by %q", s.name, c.path, v, r.Row, c.v, c.row)
			}
		}
	}
}

// FXPOLICYFOLD: the Mac's volume takes a name in Unicode's full case folding, so each of these is the support
// folder's key, its ssh keys, a plant or the default branch, as the volume takes them: by ſ (the long s), ß and ẞ (ss),
// ﬆ and ﬅ (st), ﬁ (fi), the Kelvin sign (k), the Ohm and Ångström signs (ω, å), İ (i and a dot above), and marks in
// another order. Linux takes a name as written; Windows is left as it was (its volume's upcase table isn't measured).
func TestEveryFullyFoldedSpellingOfAPathIsTheSameRule(t *testing.T) {
	const asa = "/Users/\u212bsa\u2126" // the Ångström and Ohm signs: Åsaω
	pl := hirePlaces(asa + "/Library/Application Support/AgentDeck")
	pl.Home, pl.Root, pl.HiresFolder, pl.Checkout, pl.GitDir, pl.GMFolder = asa, asa+"/p-hires/h1", asa+"/p-hires", asa+"/p", asa+"/p/.git", ""
	pl.Accounts, pl.SocketParent, pl.OwnSockets, pl.DefaultBranch, pl.RemoteLogin = nil, "/tmp/smolcorp-501", "/tmp/smolcorp-501/h1", "stable", true
	p := mustNew(t, pl)
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
	}{
		{"/Users/\u00e5sa\u03c9/Library/Application \u017fupport/AgentDeck/hook-key", Read, Deny},
		{"/Users/A\u030asa\u03a9/Library/Application Support/AgentDeck/hook-\u212aey", Read, Deny},
		{asa + "/Library/Application \u017fupport/AgentDeck", Move, Deny},
		{asa + "/Library/Application Support/AgentDec\u212a/setup-key", Read, Deny},
		{asa + "/.\u00dfh/id_ed25519", Read, Deny},
		{asa + "/.\u1e9eH/id_ed25519", Read, Deny},
		{asa + "/.\u017f\u017fh/id_ed25519", Read, Deny},
		{asa + "/.z\u017fhrc", Write, Deny},
		{asa + "/.pro\ufb01le", Write, Deny},
		{asa + "/.gitcon\ufb01g", Write, Deny},
		{asa + "/.ssh/authorized_\u212aeys", Write, Deny},
		{asa + "/p/.git/refs/heads/\ufb06able", Write, Deny},
		{asa + "/p/.git/refs/heads/\ufb05able", Write, Deny},
		{asa + "/p/.git/con\ufb01g", Write, Deny},
		{asa + "/p-hire\u017f/h2/main.go", Write, Deny},
		{asa + "/.claude/.credential\u017f.json", Read, Deny},
		{"/private/tmp/\u017fmolcorp-501/h2/s", Write, Deny},
		{"/\u017fystem/Volume\u017f/Data/private/tmp/smolcorp-501/h2/s", Write, Deny},
		{"/Sy\ufb06em/Volumes/Data/private/tmp/smolcorp-501/h2/s", Write, Deny},
		{"/tmp/smolcorp-501/h2/s", Write, Deny},
		// What isn't the same name stays its own: the dotless ı, a ligature not in the folding, an accent.
		{asa + "/.\u017f\u017fh-notes", Read, Allow},
		{asa + "/p/.git/refs/heads/\u0131stable", Write, Allow},
		{asa + "/p/.git/refs/heads/stabl\u00e9", Write, Allow},
		{"/tmp/smolcorp-501/h1/s", Write, Allow},
		{"/private/tmp/smolcorp-501/H1/\u017f", Write, Allow},
	} {
		if got, by := p.Decide(c.at, c.op); got != c.want {
			t.Errorf("%v %+q: %v by %q, want %v", c.op, c.at, got, by.Row, c.want)
		}
	}
	for in, want := range map[string]string{"\u0130": "i\u0307", "\u1e9e": "ss", "\u00df": "ss", "\ufb06": "st", "\u212a": "k",
		"\u03b1\u0345\u0301": "\u03ac\u03b9", "\u03b1\u0301\u0345": "\u03ac\u03b9", "\u1fb4": "\u03ac\u03b9", "\u1f88": "\u1f00\u03b9",
		"e\u0302\u0323": "\u1ec7", "\u03c2": "\u03c3", "\u13a0": "\u13a0", "\uab70": "\u13a0", "plain": "plain"} {
		if got := foldName("darwin", in); got != want {
			t.Errorf("darwin: %+q folds to %+q, want %+q", in, got, want)
		}
	}
	if got := foldName("linux", "\u017f"); got != "\u017f" {
		t.Errorf("linux: \u017f folds to %+q", got)
	}
}

// RT126POLICYFOLD's probe, made a row (FXPOLICYFOLDR1): a stand-in owner whose home, checkout and hires folder are
// named with letters this Mac's volume (Unicode 17.0) takes for one and Unicode 16 didn't know: three Latin pairs, the
// first and last Beria Erfe pairs, and a dot below with a new mark of class 230 (U+1AD0) or 220 (U+1ADD, against an
// acute) in either order. The volume takes each pair for one name (the review measured them on stand-in files); a
// denied place spelled the other way must be denied too.
func TestReviewUnicode17SpellingsOfADeniedPlace(t *testing.T) {
	for _, c := range []struct{ name, homeName, other string }{
		{"U+A7CF home, U+A7CE spelling", "/Users/o꟏", "/Users/o꟎"},
		{"U+A7D3 home, U+A7D2 spelling", "/Users/oꟓ", "/Users/o꟒"},
		{"U+A7D5 home, U+A7D4 spelling", "/Users/oꟕ", "/Users/o꟔"},
		{"U+16EBB home, U+16EA0 spelling", "/Users/o\U00016ebb", "/Users/o\U00016ea0"},
		{"U+16ED3 home, U+16EB8 spelling", "/Users/o\U00016ed3", "/Users/o\U00016eb8"},
		{"dot below then U+1AD0, other order", "/Users/oạ᫐", "/Users/oa᫐̣"},
		{"acute then U+1ADD, other order", "/Users/oá᫝", "/Users/oa᫝́"},
	} {
		pl := hirePlaces(c.homeName + "/Library/Application Support/AgentDeck")
		pl.Home, pl.Root, pl.HiresFolder, pl.Checkout, pl.GitDir, pl.GMFolder = c.homeName, c.homeName+"/p-hires/h1", c.homeName+"/p-hires", c.homeName+"/p", c.homeName+"/p/.git", ""
		pl.Accounts, pl.GitHooks, pl.GitNamed, pl.GitConfigs = nil, nil, nil, nil
		p := mustNew(t, pl)
		for _, at := range []struct {
			rest string
			op   Op
		}{{"/.ssh/id_ed25519", Read}, {"/.zshrc", Write}, {"/Library/Application Support/AgentDeck/setup-key", Read}, {"/p-hires/h2/main.go", Write}} {
			plain, _ := p.Decide(c.homeName+at.rest, at.op)
			other, row := p.Decide(c.other+at.rest, at.op)
			t.Logf("%s: %v %s: plainly %v, other spelling %v (%q)", c.name, at.op, at.rest, plain, other, row.Row)
			if plain != Deny || other != Deny {
				t.Errorf("%s: %v %s: plainly %v, other spelling %v by %q; want both deny", c.name, at.op, at.rest, plain, other, row.Row)
			}
		}
		// And the desk's own place, in the other spelling, is still its work: the letters are known, not refused.
		if v, row := p.Decide(c.other+"/p-hires/h1/main.go", Write); v != Allow {
			t.Errorf("%s: its own place in the other spelling: %v by %q, want allow", c.name, v, row.Row)
		}
	}
}

// FXPOLICYFOLDR1 (RT126POLICYFOLD's fix 3): on a Mac, a path with a code point the fold's Unicode (17.0) doesn't assign
// is denied, whatever the rows say, and a Policy with such a place isn't made: a later macOS's volume may know it and
// fold it as the key can't, and this one refuses it (EILSEQ). Unicode 17.0's new letters and marks are known. Off a
// Mac nothing is folded, so nothing is refused.
func TestOnAMacAPathWithACodePointTheFoldDoesntKnowIsDenied(t *testing.T) {
	mac := mustNew(t, hirePlaces(realSupport))
	lin := hirePlaces(realSupport)
	lin.GOOS = "linux"
	linux := mustNew(t, lin)
	for _, r := range []rune{0x0378, 0x1aec, 0xfffe, 0x2fffd, 0x3347a, 0x50000, 0xe0080, 0xe01f0, 0x10ffff} {
		for _, at := range []string{hires + "/h1/a" + string(r), hires + "/h1/" + string(r) + "/main.go", home + "/notes-" + string(r)} {
			if v, row := mac.Decide(at, Write); v != Deny {
				t.Errorf("Mac: write %+q: %v by %q, want deny", at, v, row.Row)
			}
			if v, row := linux.Decide(at, Write); v != Allow {
				t.Errorf("Linux: write %+q: %v by %q, want allow", at, v, row.Row)
			}
		}
		for what, change := range map[string]func(*Places){
			"home":       func(pl *Places) { pl.Home = "/Users/o" + string(r) },
			"checkout":   func(pl *Places) { pl.Checkout, pl.GitDir = repo+string(r), repo+string(r)+"/.git" },
			"a git hook": func(pl *Places) { pl.GitHooks = []string{home + "/.githooks" + string(r)} },
		} {
			pl := hirePlaces(realSupport)
			change(&pl)
			if _, err := New(pl); !errors.Is(err, ErrUnknownCodePoint) {
				t.Errorf("Mac: a %s with U+%04X: New says %v, want %v", what, r, err, ErrUnknownCodePoint)
			}
			pl.GOOS = "linux"
			if _, err := New(pl); err != nil {
				t.Errorf("Linux: a %s with U+%04X: New says %v", what, r, err)
			}
		}
	}
	for _, r := range []rune{0xa7ce, 0xa7cf, 0x16ea0, 0x16ed3, 0x1acf, 0x1ad0, 0x1aeb, 0x10efa, 0x1e6f5, 0x3347a - 1, 0xe01ef, 0xe000, 0x10fffd} {
		at := hires + "/h1/a" + string(r)
		if v, row := mac.Decide(at, Write); v != Allow {
			t.Errorf("Mac: write %+q: %v by %q, want allow (Unicode 17.0 assigns U+%04X)", at, v, row.Row, r)
		}
	}
}

// FXPOLICYFOLDR1: every Go simple-fold orbit (Go's unicode, 17.0 in Go 1.27) is one key on a Mac, or one of the
// two letters is one the fold doesn't know, so that a path with it is denied and a place with it refused: the volume
// joins a letter and its case pairs, so the key must. (RT126POLICYFOLD's new17 probe, without the volume;
// TestTheMacsFoldIsUnicodesCanonicalCaselessMatch tries them on it.) When Go's Unicode is the fold's, the two say
// the same code points are assigned.
func TestEveryGoSimpleFoldOrbitIsOneKeyOrRefused(t *testing.T) {
	p := mustNew(t, hirePlaces(realSupport))
	refused := func(r rune) bool {
		v, _ := p.Decide(hires+"/h1/a"+string(r), Write)
		pl := hirePlaces(realSupport)
		pl.Home = "/Users/o" + string(r)
		_, err := New(pl)
		return v == Deny && errors.Is(err, ErrUnknownCodePoint)
	}
	orbits, apart := 0, 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) == r {
			continue
		}
		orbits++
		for o := unicode.SimpleFold(r); o != r; o = unicode.SimpleFold(o) {
			if foldName("darwin", string(o)) == foldName("darwin", string(r)) {
				continue
			}
			apart++
			if !refused(r) && !refused(o) {
				t.Errorf("U+%04X and U+%04X are one Go fold orbit, but keys %+q and %+q, and neither is refused",
					r, o, foldName("darwin", string(r)), foldName("darwin", string(o)))
			}
		}
	}
	t.Logf("%d code points with a Go fold orbit (Go's Unicode %s), %d pairs kept apart by the key, so one refused", orbits, unicode.Version, apart)
	if orbits < 2800 {
		t.Errorf("only %d code points with a fold orbit", orbits)
	}
	if unicode.Version != foldUnicode {
		return
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		if goKnows := !unicode.Is(unicode.Cn, r); goKnows != known(string(r)) {
			t.Errorf("U+%04X: Go's Unicode %s assigns it: %v; the fold's: %v", r, unicode.Version, goKnows, known(string(r)))
		}
	}
}

// FXPOLICYFOLDR1 (RT126POLICYFOLD's cheap follow-up): an ASCII path's firmlink folders are compared without folding
// it, so a plain path costs nothing more than it did; any case of them still counts.
func TestAnASCIIPathsFirmlinkFoldersCostNothing(t *testing.T) {
	for _, c := range []struct {
		p, folder, rest string
		ok              bool
	}{
		{"/System/Volumes/Data/Users/ada", "/System/Volumes/Data", "/Users/ada", true},
		{"/SYSTEM/volumes/DATA", "/System/Volumes/Data", "", true},
		{"/System/Volumes/Datax/Users", "/System/Volumes/Data", "", false},
		{"/System/Volumes", "/System/Volumes/Data", "", false},
		{"/TMP/x", "/tmp", "/x", true},
		{"/Users/ada/.ssh", "/var", "", false},
		{"/ſystem/Volumes/Data/x", "/System/Volumes/Data", "/x", true},
	} {
		if rest, ok := cutFolderFold(c.p, c.folder); rest != c.rest || ok != c.ok {
			t.Errorf("cutFolderFold(%+q, %q) = %q, %v; want %q, %v", c.p, c.folder, rest, ok, c.rest, c.ok)
		}
	}
	for _, p := range []string{"/Users/ada/Projects/app-hires/h1/main.go", "/System/Volumes/Data/Users/Ada/.SSH/id_ed25519", "/Private/Var/Folders/x"} {
		for _, folder := range []string{"/System/Volumes/Data", "/var", "/tmp", "/etc"} {
			if n := testing.AllocsPerRun(20, func() { cutFolderFold(p, folder) }); n != 0 {
				t.Errorf("cutFolderFold(%q, %q): %v allocations, want 0", p, folder, n)
			}
		}
	}
}
