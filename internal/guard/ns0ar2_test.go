package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126NS0aR2's probes, each a row: what moved main or ran the stand-in in real git (N1–N4) is refused for the hire
// and the GM, and the ordinary work its over-refusal list names runs again.

// stand is the harmless stand-in the review ran in place of a command; push is the one the guard's groups know.
const (
	stand = `"touch ran; false"`
	push  = `"git push origin x"`
)

// refusedOrNot is Check's verdict on line at cwd under groups: "" when it runs, or the refusal's group.
func refusedOrNot(line, cwd string, groups []Group) string {
	if r := Check(line, At(groups, PlaceOf(cwd))); r != nil {
		return r.Group
	}
	return ""
}

// N4, pinned (VICTOR, with SIERRA's OK): each of these is refused in every group it touches, the push group alone
// among them, so none can regress.
func TestGitRunsTextFromTheLineInEveryGroup(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	only := func(name string) []Group {
		for _, g := range WithGates(DefaultGroups()) {
			if g.Name == name {
				return []Group{g}
			}
		}
		t.Fatalf("no %s group", name)
		return nil
	}
	sets := map[string][]Group{
		"push alone": only("push"), "deploy alone": only("deploy"), "merge alone": only(MergeGroup),
		"the defaults": DefaultGroups(), "with the gates": WithGates(DefaultGroups()), "the GM's": ForGM(WithGates(DefaultGroups())),
		"none": nil,
	}
	pinned := map[string]string{
		"in-line core.fsmonitor":   `git -c core.fsmonitor=` + push + ` status`,
		"GIT_EDITOR":               `GIT_EDITOR="git push origin x; true" git commit -e`,
		"grep -O":                  `git grep -O` + push + ` b`,
		"ls-remote --upload-pack":  `git ls-remote --upload-pack=` + push + ` .`,
		"difftool -x":              `git difftool -y -x ` + push + ` HEAD~1`,
		"in-line, a merge inside":  `git -c core.fsmonitor="git merge feature" status`,
		"in-line, a deploy inside": `git -c core.fsmonitor="vercel deploy" status`,
	}
	for row, line := range pinned {
		for name, groups := range sets {
			t.Run(row+", "+name, func(t *testing.T) {
				if got := refusedOrNot(line, onFeature, groups); got != "guard" {
					t.Errorf("%q: %q", line, got)
				}
			})
		}
	}
	// A program's plain name is read as a command: one a group refuses is refused by it.
	for line, group := range map[string]string{"git difftool -x vercel HEAD~1": "deploy", "git grep -Overcel b": "deploy",
		"git bisect run git push origin x": "push", "git submodule foreach 'git push'": "push", "git rebase -x 'git push' HEAD~2": "push",
		"git filter-branch --tree-filter 'git push' HEAD": "push", "git ls-remote --upload-pack=vercel .": "deploy"} {
		if got := refusedOrNot(line, onFeature, DefaultGroups()); got != group {
			t.Errorf("%q: %q, want %s", line, got, group)
		}
	}
}

// N4: every probe that ran the stand-in, and the forms around it.
func TestGitRunsNoTextTheGuardDoesntRead(t *testing.T) {
	onMain, onFeature := testRepo(t, "main", "main"), testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	refused := []string{
		// in-line config on an allowed subcommand
		`git -c core.fsmonitor=` + stand + ` status`, `FSM=` + stand + ` git --config-env=core.fsmonitor=FSM status`,
		`git --config-env core.fsmonitor=FSM status`, `git -ccore.fsmonitor=x status`,
		`GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.fsmonitor GIT_CONFIG_VALUE_0=` + stand + ` git status`,
		`export GIT_CONFIG_KEY_0=core.pager; git log`, `env GIT_CONFIG_PARAMETERS="'core.pager=x'" git log`,
		`GIT_CONFIG_GLOBAL=./evil git status`, `read GIT_CONFIG_KEY_0 < f; git status`,
		`git -c core.editor=` + stand + ` commit --allow-empty -e`, `git -c core.sshCommand=` + stand + ` ls-remote ssh://example.invalid/x`,
		`git -c protocol.ext.allow=always ls-remote "ext::sh -c touch% ran"`, `git -c core.hooksPath=h commit -m x`,
		`git -c pager.log=` + stand + ` log`, `git -c diff.x.textconv=t diff`, `git -c "core.editor=$(echo x)" commit`,
		`git -c Core.FSMonitor=x status`, `git clone -c core.fsmonitor=x . other/c`, `git clone --template=t . other/c`,
		// variables git runs
		`GIT_EDITOR=` + stand + ` git commit -e`, `GIT_EDITOR=` + stand + ` git branch --edit-description`,
		`EDITOR=` + stand + ` git tag -a v1`, `GIT_SSH_COMMAND=` + stand + ` git ls-remote ssh://example.invalid/x`,
		`GIT_EXTERNAL_DIFF=` + stand + ` git diff HEAD~1`, `GIT_SEQUENCE_EDITOR=` + stand + ` git rebase -i HEAD~1`,
		`export GIT_EDITOR=vim; git commit`, `env VISUAL=x git commit`, `PAGER=less git log`, `GIT_ASKPASS=x git fetch`,
		`SSH_ASKPASS=x git fetch`, `typeset -n r=GIT_EDITOR`, `read GIT_EDITOR <<< x`, `echo ${GIT_EDITOR:=x}`,
		`GIT_ALLOW_PROTOCOL=ext git ls-remote "ext::x"`, `LESSOPEN="|x %s" git log`,
		`powershell -c '$env:GIT_EDITOR="x"; git commit'`, `powershell -c 'Set-Item env:GIT_EDITOR x'`,
		// options that take a command
		`git grep -O` + stand + ` b`, `git grep -nO` + stand + ` b`, `git grep --open-files-in-pager=` + stand + ` b`,
		`git ls-remote --upload-pack=` + stand + ` .`, `git ls-remote -u ` + stand + ` .`, `git clone -u ` + stand + ` . other/c`,
		`git fetch --upload-pack=` + stand + ` .`, `git archive --remote=. --exec=` + stand + ` HEAD`,
		`git push --receive-pack=` + stand + ` . x`, `git difftool -y -x ` + stand + ` HEAD~1`, `git difftool --extcmd ` + stand,
		`git mergetool -x ` + stand, `git send-email --sendmail-cmd=` + stand + ` x.patch`,
		// a git config that stores a command for later in the line
		`git config core.fsmonitor ` + push + ` && git status`, `git config set core.pager x`, `git config alias.p push`,
		`git config --add filter.x.clean x`,
	}
	allowed := []string{
		"git -c user.name=x commit -m y", "git -c color.ui=never log", "git -c core.editor=true rebase --continue",
		"git -c core.quotepath=off status", "git -c advice.detachedHead=false log", "git -c core.fsmonitor=false status",
		"GIT_EDITOR=true git commit --amend", "GIT_EDITOR=: git rebase --continue", "GIT_PAGER=cat git log", "PAGER=cat git log",
		"GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null git status", "GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=x git log",
		"git grep -O b", "git grep -n foo", "git grep -e ORIGIN", "git ls-remote origin", "git clone https://example.invalid/x.git",
		"git difftool -y HEAD~1", "git difftool -x meld HEAD~1", "git bisect run go test ./...", "git submodule foreach git status",
		"git config user.email x@example.invalid", "git config pull.rebase true",
		"git config core.editor true", "grep -rn GIT_EDITOR docs/", "echo $EDITOR", "printenv PAGER", "unset GIT_EDITOR",
	}
	for _, gm := range []bool{false, true} {
		groups := WithGates(DefaultGroups())
		if gm {
			groups = ForGM(groups)
		}
		for _, cwd := range []string{onMain, onFeature} {
			for _, line := range refused {
				// The core refuses each, for every group; the merge group may refuse a rebase -i first.
				if got := refusedOrNot(line, cwd, groups); got != "guard" && (got != MergeGroup || !strings.Contains(line, "rebase")) {
					t.Errorf("gm %v, %s: %q: %q", gm, filepath.Base(cwd), line, got)
				}
			}
		}
		for _, line := range allowed {
			if r := Check(line, At(groups, PlaceOf(onFeature))); r != nil {
				t.Errorf("gm %v: %q: %v", gm, line, r)
			}
		}
	}
	// rebase --exec's line is read by the core; the merge group refuses --exec itself (RT126NS0a).
	if r := Check("git rebase -x 'go test ./...' HEAD~2", DefaultGroups()); r != nil {
		t.Errorf("merge off: %v", r)
	}
}

// N1: a mirror remote, and a remote's fetch refspec, mirror or push URL written in the line; and a fetch that would
// use one already stored.
func TestARemoteCantWriteABaseBranch(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	mirrored := testRepo(t, "feature", "main", "config", "[remote \"x\"]\n\turl = ../src.git\n\tfetch = +refs/*:refs/*\n\tmirror = true\n")
	intoHeads := testRepo(t, "feature", "main", "config", "[remote \"x\"]\n\turl = ../src.git\n\tfetch = +refs/heads/main:refs/heads/main\n")
	bareMirror := testRepo(t, "feature", "main", "config", "[remote \"x\"]\n\tmirror\n")
	included := testRepo(t, "feature", "main", "config", "[include]\n\tpath = other\n")
	usual := testRepo(t, "feature", "main", "config", "[remote \"origin\"]\n\turl = x\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n")
	refused := map[string][]string{
		onFeature: {"git remote add -f --mirror=fetch x other/src.git", "git remote add --mirror=fetch x other/src.git && git fetch -q x",
			"git remote add x other/src.git && git config remote.x.fetch +refs/heads/main:refs/heads/main && git fetch -q x",
			"git remote add --mirror x y", "git remote add --mirror=push x y", "git remote add --mir x y", "git remote -v add --mirror x y",
			"git config --add remote.x.fetch +refs/heads/*:refs/heads/*", "git config remote.x.mirror true",
			"git config remote.origin.pushurl .", "git config set remote.x.pushurl .", "git -c remote.x.fetch=x fetch x",
			"git remote set-url --push origin .", "git remote set-head origin feature", "git remote set-head origin -d",
			"git remote rename origin x", "git remote remove origin", "git remote rm origin", "git remote frob"},
		mirrored:   {"git fetch -q x", "git pull -q x", "git fetch", "git fetch x main", "git remote update", "git pull --rebase"},
		intoHeads:  {"git fetch -q x", "git fetch --all", "git pull"},
		bareMirror: {"git fetch x"},
		included:   {"git fetch"},
	}
	allowed := map[string][]string{
		onFeature: {"git remote -v", "git remote", "git remote add upstream https://example.invalid/x.git",
			"git remote add -f upstream https://example.invalid/x.git", "git remote set-head origin -a", "git remote set-head origin --auto",
			"git remote show origin", "git remote get-url origin", "git remote set-url origin https://example.invalid/y.git",
			"git remote prune origin", "git remote update", "git fetch", "git fetch origin", "git pull"},
		usual: {"git fetch", "git fetch origin main", "git pull", "git remote update"},
	}
	checkRows(t, refused, allowed)
}

// checkRows asks Check for each line at its folder, for the hire and the GM: refused ones must be refused (by any
// group or the core), allowed ones must run.
func checkRows(t *testing.T, refused, allowed map[string][]string) {
	t.Helper()
	for _, gm := range []bool{false, true} {
		for cwd, lines := range refused {
			for _, line := range lines {
				if got := mergeRefusal(line, cwd, gm); got == "" {
					t.Errorf("gm %v, %s: %q runs", gm, filepath.Base(cwd), line)
				}
			}
		}
		for cwd, lines := range allowed {
			for _, line := range lines {
				if got := mergeRefusal(line, cwd, gm); got != "" {
					t.Errorf("gm %v, %s: %q: %s", gm, filepath.Base(cwd), line, got)
				}
			}
		}
	}
}

// N2: a base branch's remote-tracking ref is a base branch, and a base branch git would make from one (its guess) is
// named, unless the guard reads that the local one is there.
func TestABaseBranchIsntMadeFromARemotes(t *testing.T) {
	noLocal := testRepo(t, "feature", "main", "refs/remotes/origin/release/9", "abc\n")
	withLocal := testRepo(t, "feature", "main", "refs/heads/main", "abc\n", "refs/heads/release/9", "abc\n")
	packed := testRepo(t, "feature", "main", "packed-refs", "# pack-refs with: peeled\nabc refs/heads/release/9\nabc refs/heads/main\n")
	refused := map[string][]string{
		noLocal: {"git update-ref refs/remotes/origin/release/9 feature", "git update-ref remotes/origin/main x",
			"git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/x", "git fetch . feature:refs/remotes/origin/release/9",
			"git update-ref refs/remotes/origin/HEAD x",
			"git switch release/9", "git checkout -q release/9", "git worktree add -q other/w release/9", "git switch main",
			"git checkout main", "git checkout main -- f", "git switch --track origin/release/9", "git checkout -t origin/main",
			"git checkout $b", "cd x && git switch main", "git -C ../x checkout release/1"},
	}
	allowed := map[string][]string{
		noLocal: {"git switch feature", "git switch --detach release/9", "git checkout --detach origin/release/9",
			"git switch --no-guess release/9", "git checkout -- f", "git checkout -", "git switch -", "git checkout HEAD -- f",
			"git checkout -b feature/y origin/release/9", "git worktree add --detach other/w release/9", "git update-ref refs/remotes/origin/x y",
			"cd x && git switch feature/y", "git checkout origin/main"},
		withLocal: {"git switch release/9", "git checkout release/9", "git switch main", "git worktree add ../w release/9"},
		packed:    {"git switch release/9", "git checkout main"},
	}
	checkRows(t, refused, allowed)
	if got := mergeRefusal("git switch release/9", noLocal, false); !strings.Contains(got, "makes release/9 from a remote's branch") {
		t.Errorf("why: %q", got)
	}
}

// N3: writing a ref file moves main with no git command at all; only readers may name a path in a git folder.
func TestAGitFolderIsWrittenOnlyByGit(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n")
	// A bare repository, and a worktree whose gitdir: names a folder not called .git (git init --separate-git-dir).
	bare := filepath.Join(t.TempDir(), "src.git")
	for _, f := range []string{"objects", "refs/heads"} {
		if err := os.MkdirAll(filepath.Join(bare, f), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bare, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "store")
	if err := os.Rename(filepath.Join(testRepo(t, "feature", "", "refs/heads/main", "x\n"), ".git"), store); err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, ".git"), []byte("gitdir: "+store+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := map[string][]string{
		onFeature: {"cp .git/refs/heads/feature .git/refs/heads/main", "git rev-parse feature > .git/refs/heads/main",
			`echo "ref: refs/heads/feature" > .git/refs/heads/main`,
			"mv .git/refs/heads/feature .git/refs/heads/main && git switch -q -c feature main",
			"cp x .git/packed-refs", "echo x >> .git/packed-refs", "printf 'ref: refs/heads/main' > .git/HEAD",
			"cat x >> .git/config", "cp hook .git/hooks/pre-commit", "tee .git/refs/heads/main < x", "ln -s x .git/refs/heads/main",
			"dd if=x of=.git/refs/heads/main", "rm .git/refs/heads/main", "cd .git && cp x refs/heads/main", "d=.git; cp x $d/refs/heads/main",
			"cp x .GIT/refs/heads/main", "cp x .git./refs/heads/main", `cp x .git\refs\heads\main`, "cp x GIT~1/refs/heads/main",
			"cp x .g*/refs/heads/main", "cp x ./.gi?/refs/heads/main", "cp x .*/refs/heads/main", "cp x sub/.?it/refs/heads/main", "cp x sub/../.git/refs/heads/main",
			"cp x " + filepath.Join(onFeature, ".git", "refs", "heads", "main"), "git config -f .git/config x.y z",
			"git --git-dir=.git hash-object -w x", "less -o .git/refs/heads/main x", "sed -i s/a/b/ .git/packed-refs",
			"find .git -name main -delete"},
		bare: {"cp x refs/heads/main", "cp x " + filepath.Join(bare, "refs", "heads", "main"), "echo x > packed-refs"},
		tree: {"cp x " + filepath.Join(store, "refs", "heads", "main"), "cp x " + filepath.ToSlash(rel(t, tree, store)) + "/HEAD"},
	}
	allowed := map[string][]string{
		onFeature: {"cat .git/HEAD", "ls .git/refs/heads", "grep -r url .git/config", "head -1 .git/packed-refs",
			"diff .git/HEAD x", "wc -l .git/packed-refs", "stat .git/HEAD", "cat < .git/HEAD", "git status",
			"cp x .gitignore", "cp x .github/workflows/ci.yml", "echo x >> .gitignore", "git clone https://example.invalid/a/b.git",
			"cp x src.git/y", "less .git/config", "for i in {1..100}; do touch f$i; done", "cp a b{1..3}",
			"make check TASK='fixed in split(), see (#3)'", "cp * ../backup/",
			`git commit -m "Ignore .git/ in the linter"`, "git commit -qam 'keep .git/hooks out'"},
		tree: {"cp a b", "git status"},
	}
	checkRows(t, refused, allowed)
	// Only while the merge group is on.
	if r := Check("cp x .git/refs/heads/main", At(DefaultGroups(), PlaceOf(onFeature))); r != nil {
		t.Errorf("merge off: %v", r)
	}
	if got := mergeRefusal("cp x .git/refs/heads/main", onFeature, false); !strings.Contains(got, "git folder") {
		t.Errorf("why: %q", got)
	}
}

// The should-fix items: a replace ref, and the default branch's own ref.
func TestAReplaceRefOrADefaultCantBeSet(t *testing.T) {
	onFeature := testRepo(t, "feature", "trunk", "refs/heads/main", "abc\n")
	checkRows(t, map[string][]string{onFeature: {
		"git replace " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40), "git replace -f a b", "git replace --edit a",
		"git remote set-head origin x", "git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/x",
		"git update-ref refs/remotes/origin/trunk x",
	}}, map[string][]string{onFeature: {"git replace -l", "git replace", "git replace --list"}})
}

// RT126NS0aR2's over-refusal list, checked on a feature branch in a clone whose origin/HEAD is main: these run.
func TestOrdinaryWorkOnAFeatureBranchRuns(t *testing.T) {
	onFeature := testRepo(t, "feature", "main", "refs/heads/main", "abc\n",
		"config", "[remote \"origin\"]\n\turl = https://example.invalid/x.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n")
	runs := []string{
		// "still run"
		"git commit -qam x", "git -c user.name=x commit -m y", "git rebase origin/main", "git rebase origin/main 2>&1 | tail -3",
		"git rebase -i origin/main", "git rebase -i HEAD~2", "git rebase --continue", "git pull", "git pull --rebase origin main",
		"git merge main", "git merge main && echo ok", "git branch feature/x", "git switch -c feature/x origin/main",
		"git checkout -b feature/y main", "git switch main", "git stash && git switch main && git stash pop",
		"git worktree add ../x -b feature/z", "git cherry-pick main", "git lfs pull", "git submodule update --init",
		// "wrongly refused", now running
		"git rebase main", "git rebase -i main", "git fetch origin && git rebase origin/main",
		"git fetch origin main && git merge origin/main", "git status && git rebase origin/main",
		"GIT_EDITOR=true git rebase --continue", "git -c core.editor=true rebase --continue", `git lfs track "*.psd"`,
		"git -C . merge main", "git log --oneline -3 && git merge main",
	}
	checkRows(t, nil, map[string][]string{onFeature: runs})
	// Still refused, on purpose: a command that may move HEAD before the merge (go test runs code), and on main.
	onMain := testRepo(t, "main", "main")
	checkRows(t, map[string][]string{onFeature: {"git merge origin/main && go test ./...", "make x && git rebase origin/main",
		"git checkout main && git merge feature", "git lfs migrate import --everything"},
		onMain: {"git fetch origin && git rebase origin/main", "GIT_EDITOR=true git rebase --continue", "git -C . merge feature"}}, nil)
}

// rel is target from base, as a relative path.
func rel(t *testing.T, base, target string) string {
	t.Helper()
	r, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
