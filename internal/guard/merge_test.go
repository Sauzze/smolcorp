package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testRepo makes a repository's git folder by hand in a temp folder (no git runs): HEAD on head (a branch, or a
// commit when it has no /), objects/ and refs/ as git wants them, origin's HEAD naming defaultBranch, and any more
// files given as path, text.
func testRepo(t *testing.T, head, defaultBranch string, more ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, folder := range []string{"objects", "refs/heads"} {
		if err := os.MkdirAll(filepath.Join(dir, ".git", folder), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{"HEAD", "ref: refs/heads/" + head + "\n"}
	if !strings.Contains(head, "/") && len(head) == 40 {
		files[1] = head + "\n"
	}
	if defaultBranch != "" {
		files = append(files, "refs/remotes/origin/HEAD", "ref: refs/remotes/origin/"+defaultBranch+"\n")
	}
	files = append(files, more...)
	for i := 0; i < len(files); i += 2 {
		path := filepath.Join(dir, ".git", files[i])
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[i+1]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// mergeRefusal is the merge group's refusal of line run in cwd, with the gate groups added to the defaults, or "".
func mergeRefusal(line, cwd string, gm bool) string {
	groups := WithGates(DefaultGroups())
	if gm {
		groups = ForGM(groups)
	}
	r := Check(line, At(groups, PlaceOf(cwd)))
	if r == nil {
		return ""
	}
	return r.Group + ": " + r.Why
}

// Q10: work lands on a base branch (main, master, release/*, the repository's default) only by the owner's hand. A
// merge, pull or rebase is refused on one, and a branch named onto one is refused wherever the hire is; everything
// else a hire does with branches still runs.
func TestAMergeOntoABaseBranchIsRefused(t *testing.T) {
	onMain, onFeature := testRepo(t, "main", "develop"), testRepo(t, "feature/x", "develop", "refs/heads/main", "abc\n")
	onRelease, onDefault := testRepo(t, "release/2.0", ""), testRepo(t, "develop", "develop")
	detached := testRepo(t, strings.Repeat("a", 40), "develop")
	rebasing := testRepo(t, strings.Repeat("b", 40), "develop", "rebase-merge/head-name", "refs/heads/master\n")
	noRepo := t.TempDir()
	refused := map[string][]string{
		onMain: {"git merge feature/x", "git pull", "git pull --rebase origin main", "git rebase feature/x", "git merge --continue",
			"git --no-pager merge x", "git update-ref HEAD abc", "git commit -m x && git merge feature/x"},
		onRelease: {"git merge feature/x", "git pull"},
		onDefault: {"git merge feature/x"},
		rebasing:  {"git rebase --continue", "git rebase --skip"},
		// RT126NS0a B6: a HEAD that names no branch is taken as a base branch.
		detached: {"git merge x", "git pull", "git rebase main", "git rebase --continue"},
		onFeature: {
			"git rebase main develop", "git rebase main release/2.0", "git rebase --root main", "git rebase --onto x y master",
			"git rebase --update-refs main",
			"git update-ref refs/heads/main abc", "git update-ref -m why main abc", "git update-ref -d refs/heads/develop",
			"git update-ref --stdin", "git branch -f main abc", "git branch --force master", "git branch -M main",
			"git branch -m -f old release/3", "git branch -C x develop", "git branch -f MAIN x", "git branch -f $b x",
			"git branch -f `echo x` y", "git branch -f ma* y", "git checkout -B main", "git checkout -fB master origin/x",
			"git checkout -Bmain", "git switch -C master", "git switch --force-create=develop", "git switch --force-create main",
			"git worktree add -B main ../x", "git fetch origin main:main", "git fetch . feature/x:release/1.0",
			"git fetch origin +refs/heads/*:refs/heads/*", "git fetch origin x:refs/heads/main", "git pull origin x:main",
			// What the guard can't place: another command that may move HEAD, another folder or repository, a wrapper.
			"git checkout main && git merge feature/x", "cd ../other && git merge main", "git -C ../other merge main",
			"sh -c 'git merge main'", "GIT_DIR=x git merge main", "export GIT_DIR=../o/.git; git branch -f y z",
			"git merge main > .git/HEAD", "cd x && git branch -f y z", "git --git-dir=../o/.git branch -f y z",
			"nice git merge main", "git merge main | tee log",
			// RT126NS0aR2 N2: a base branch's remote-tracking ref, and N1: a fetch where the guard can't read the config.
			"git update-ref refs/remotes/origin/main abc", "git fetch origin main:refs/remotes/origin/main", "cd x && git fetch",
		},
		noRepo: {"git merge x", "git pull", "git rebase main", "git branch -f y z"},
	}
	allowed := map[string][]string{
		onMain: {"git merge --abort", "git rebase --abort", "git fetch origin", "git status", "git checkout -b feature/y",
			"git log --oneline main", "git diff main", "git checkout feature/x", "git branch -f feature/y abc", "git switch -c z"},
		onFeature: {"git merge main", "git pull", "git pull origin main", "git rebase origin/main", "git rebase -i main^", "git rebase main",
			"git rebase --onto main old feature/x", "git rebase origin/main feature/x", "git update-ref refs/heads/feature/x abc",
			"git update-ref HEAD abc", "git update-ref refs/remotes/origin/feature/x abc", "git branch -f feature/y main",
			"git branch -m old new", "git branch -D old", "git checkout -B y main", "git checkout main", "git switch -C y",
			"git worktree add ../x main", "git worktree add -B y ../x main", "git fetch origin main",
			"git fetch origin x:refs/remotes/origin/x", "git fetch origin x:y", "git merge main 2>&1 | tail -5",
			"git merge main > out.txt", "git merge main; echo done", "git fetch origin x:y && git log",
			"git branch -f y z && git log"},
		detached: {"git status", "git checkout feature/x", "git log main"},
		rebasing: {"git rebase --abort", "git status"},
		noRepo:   {"git status", "git checkout x", "ls"},
	}
	for _, gm := range []bool{false, true} {
		for cwd, lines := range refused {
			for _, line := range lines {
				if got := mergeRefusal(line, cwd, gm); !strings.HasPrefix(got, MergeGroup+": ") {
					t.Errorf("gm %v, %q in %s: not refused by merge (%q)", gm, line, filepath.Base(cwd), got)
				}
			}
		}
		for cwd, lines := range allowed {
			for _, line := range lines {
				if got := mergeRefusal(line, cwd, gm); got != "" {
					t.Errorf("gm %v, %q in %s: refused: %s", gm, line, filepath.Base(cwd), got)
				}
			}
		}
	}
	// The refusal says which branch, or that it can't tell.
	if got := mergeRefusal("git merge x", onMain, false); !strings.Contains(got, "This lands on main, a base branch") {
		t.Errorf("on main: %q", got)
	}
	if got := mergeRefusal("git branch -f develop x", onFeature, false); !strings.Contains(got, "This moves develop, a base branch") {
		t.Errorf("the default: %q", got)
	}
	if got := mergeRefusal("cd x && git merge y", onFeature, false); !strings.Contains(got, "can't tell which branch") {
		t.Errorf("after a cd: %q", got)
	}
	// Switched off, the group refuses nothing; groups never placed are a place unknown.
	off := At(WithGates(DefaultGroups()), PlaceOf(onMain))
	for i := range off {
		off[i].On = off[i].Name != MergeGroup
	}
	if r := Check("git merge x", off); r != nil {
		t.Errorf("merge off: %v", r)
	}
	if r := Check("git merge main", WithGates(DefaultGroups())); r == nil || r.Group != MergeGroup {
		t.Errorf("unplaced: %v", r)
	}
}

// A worktree's .git is a file naming its own git folder, whose commondir holds the remotes: both are followed.
func TestAWorktreesBranchAndDefaultAreRead(t *testing.T) {
	main := testRepo(t, "feature/a", "trunk", "worktrees/w/HEAD", "ref: refs/heads/master\n", "worktrees/w/commondir", "../..\n")
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, ".git"), []byte("gitdir: "+filepath.Join(main, ".git", "worktrees", "w")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tree, "src", "deep")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	p := PlaceOf(sub)
	if !p.Known || p.Branch != "refs/heads/master" || !slices.Equal(p.Defaults, []string{"trunk"}) {
		t.Fatalf("%+v", p)
	}
	if mergeRefusal("git merge x", sub, false) == "" || mergeRefusal("git branch -f trunk x", sub, false) == "" {
		t.Error("the worktree's branch or its default wasn't read")
	}
	if got := mergeRefusal("git merge x", main, false); got != "" {
		t.Errorf("the main checkout is on a feature branch: %s", got)
	}
	// A .git that is a link, or a file that names no git folder, is a place unknown.
	odd := t.TempDir()
	if err := os.WriteFile(filepath.Join(odd, ".git"), []byte("not a gitdir\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirs := []string{odd, "relative/path", ""}
	// Windows makes a link only with the right to: tried where it can.
	if linked := t.TempDir(); os.Symlink(filepath.Join(main, ".git"), filepath.Join(linked, ".git")) == nil {
		dirs = append(dirs, linked)
	}
	for _, dir := range dirs {
		if PlaceOf(dir).Known {
			t.Errorf("%q: known", dir)
		}
	}
}

// RT126NS0a: the merge group fails closed. Each blocker's spellings, which moved main in real git 2.55, are refused
// for the hire and the GM; what a hire does on its own branch still runs.
func TestTheMergeGroupFailsClosed(t *testing.T) {
	onMain, onFeature := testRepo(t, "main", "develop"), testRepo(t, "feature", "develop")
	type probe struct {
		cwd     string
		refused []string
		allowed []string
	}
	blockers := map[string][]probe{
		// B1: rebase's options that take a value, and every word a rebase names.
		"B1": {{onMain, []string{"git rebase --empty drop feature", "git rebase --whitespace fix feature",
			"git rebase --trailer x feature", "git rebase --empty=drop feature", "git rebase -C 3 feature"}, nil},
			{onFeature, []string{"git rebase --empty drop up main", "git rebase --whitespace fix up main", "git rebase up main",
				"git rebase --trailer x up master", "git rebase -x 'make test' up", "git rebase --exec=true up",
				"git rebase up @{-1}", "git rebase a b main", "git rebase --root a main"},
				[]string{"git rebase --empty drop origin/main", "git rebase --whitespace fix up feature", "git rebase --onto main up feature",
					"git rebase -i up feature", "git rebase --root feature", "git rebase --continue", "git rebase main",
					"git rebase - up", "GIT_SEQUENCE_EDITOR=true git rebase -i up feature"}}},
		// B2: a long option by any start of its name, and one the guard doesn't know.
		"B2": {{onFeature, []string{"git branch --forc main feature", "git branch --fo main feature", "git switch --force-c main",
			"git switch --force-cr=main", "git rebase --roo --onto feature main", "echo x | git update-ref --std",
			"git update-ref --stdi", "git rebase --update-ref up", "git rebase --upd up", "git fetch --std origin",
			"git merge --frobnicate x", "git branch --frobnicate main", "git rebase -Q up", "git checkout --orph main",
			"git worktree add --frob ../x"},
			[]string{"git merge --no-ff main", "git merge --ff-o main", "git branch --forc feature/y main", "git fetch --prune-t origin",
				"git rebase --autosq origin/main", "git switch --crea z", "git merge --abo"}}},
		// B3: deleting a base branch, and making or renaming one.
		"B3": {{onFeature, []string{"git branch -D main", "git branch -d master", "git branch --delete release/1",
			"git branch -rd origin/x main", "git branch main feature", "git branch develop", "git branch -m feature main",
			"git branch -m main", "git branch -M x release/2", "git branch -c x main", "git branch -C master", "git switch -c main",
			"git switch -C main", "git switch --orphan master", "git checkout -b main", "git checkout -bmain", "git checkout --orphan main",
			"git worktree add -b main ../x", "git worktree add -Bmaster ../x feature", "git worktree add ../main",
			"git stash branch main", "git symbolic-ref refs/heads/x refs/heads/main", "git symbolic-ref refs/heads/main refs/heads/x"},
			[]string{"git branch -D old", "git branch -m old new", "git branch new", "git branch", "git branch -a", "git branch --list 'ma*'",
				"git branch --contains main", "git branch -u origin/main", "git checkout -b feature/z main", "git switch -c z origin/main",
				"git worktree add ../x feature", "git worktree add -b y ../x", "git worktree list", "git worktree remove ../x",
				"git stash", "git stash pop", "git stash branch fix", "git symbolic-ref --short HEAD", "git symbolic-ref HEAD"}},
			{onMain, []string{"git branch -m new"}, []string{"git checkout -b feature/z", "git branch -c copy"}}},
		// B4: what moves a branch from input the guard can't read, and subtree's merges.
		"B4": {{onFeature, []string{"printf 'reset refs/heads/main\\nfrom refs/heads/feature\\n\\n' | git fast-import",
			"git fast-import < s", "git filter-branch -- main", "git replay --advance main up..x", "git receive-pack .",
			"git subtree split -P lib -b main"}, []string{"git subtree merge -P lib main", "git subtree pull --prefix=lib ../x main"}},
			{onMain, []string{"git subtree merge --prefix=lib feature", "git subtree pull --prefix lib ../x feature",
				"git subtree add -P lib x"}, nil}},
		// B5: config set in the line, and an alias, autocorrect or include written to config.
		"B5": {{onFeature, []string{"git -c alias.m=merge m feature", "git -c alias.b='branch -f main' b",
			"git -c help.autocorrect=1 brnch -f main x", "git --config-env=alias.m=M m x", "git --config-env alias.m=M m x",
			"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=alias.m GIT_CONFIG_VALUE_0=merge git m feature",
			"env GIT_CONFIG_PARAMETERS=\"'alias.m=merge'\" git m x", "git -c remote.origin.fetch=+refs/heads/x:refs/heads/main fetch",
			"git -c rebase.updateRefs=true rebase up", "git config alias.m merge", "git config --global alias.m 'branch -f main'",
			"git config set alias.m merge", "git config --add include.path ../x", "git config help.autocorrect 1",
			"git config includeIf.onbranch:x.path y", "git config -e", "git config edit", "git config --rename-section x alias",
			"git config $k merge", "git config --frob alias.m merge"},
			[]string{"git config user.name x", "git config --get alias.m", "git config get alias.m", "git config --list",
				"git config alias.m", "git config list --show-origin", "git -c color.ui=never log",
				"GIT_CONFIG_NOSYSTEM=1 git status", "git -c user.name=x commit -m y"}},
			{onMain, []string{"git -c alias.m=merge m feature", "git m feature", "git -c alias.x=y status"}, []string{"git lfs pull"}}},
	}
	for _, gm := range []bool{false, true} {
		for blocker, probes := range blockers {
			for _, pr := range probes {
				for _, line := range pr.refused {
					// In-line config is the guard core's since RT126NS0aR2 (gitruns.go), for every group.
					if got := mergeRefusal(line, pr.cwd, gm); !strings.HasPrefix(got, MergeGroup+": ") && !strings.HasPrefix(got, "guard: ") {
						t.Errorf("%s, gm %v, %q in %s: not refused by merge (%q)", blocker, gm, line, filepath.Base(pr.cwd), got)
					}
				}
				for _, line := range pr.allowed {
					if got := mergeRefusal(line, pr.cwd, gm); got != "" {
						t.Errorf("%s, gm %v, %q in %s: refused: %s", blocker, gm, line, filepath.Base(pr.cwd), got)
					}
				}
			}
		}
	}
	// With the push group switched off, a push to a remote is the owner's to allow; one to a folder moves a branch here.
	pushOff := At(WithGates(DefaultGroups()), PlaceOf(onFeature))
	for i := range pushOff {
		pushOff[i].On = pushOff[i].Name != "push"
	}
	for line, refused := range map[string]bool{"git push origin main": false, "git push -f origin feature:main": false,
		"git push . feature:main": true, "git push ../other x:master": true, "git push . feature:feature/y": false} {
		if r := Check(line, pushOff); (r != nil) != refused || r != nil && r.Group != MergeGroup {
			t.Errorf("push off, %q: %v", line, r)
		}
	}
	// The refusal says why.
	for line, why := range map[string]string{"git rebase up main": "This moves main", "git branch -D main": "This moves main",
		"git -c alias.m=merge m x": "git would run text given here", "git config alias.m merge": "git would run text given here",
		"git config help.autocorrect 1": "An alias, autocorrect or include",
		"git merge --frobnicate x":      "can't read this git command", "git fast-import": "can't read this git command"} {
		if got := mergeRefusal(line, onFeature, false); !strings.Contains(got, why) {
			t.Errorf("%q: %q", line, got)
		}
	}
}

// RT126NS0a B6: the branch is read only as git reads it, and what can't be read for sure is a base branch.
func TestABranchTheGuardCantReadIsABaseBranch(t *testing.T) {
	write := func(dir string, files ...string) {
		t.Helper()
		for i := 0; i < len(files); i += 2 {
			path := filepath.Join(dir, files[i])
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if files[i+1] != "/" {
				if err := os.WriteFile(path, []byte(files[i+1]), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A reftable repository: HEAD is a stand-in, and the refs are no files (git init --ref-format=reftable).
	reftable := t.TempDir()
	write(reftable, ".git/HEAD", "ref: refs/heads/.invalid\n", ".git/config", "[core]\n\trepositoryformatversion = 1\n"+
		"[extensions]\n\trefStorage = reftable\n", ".git/refs/heads", "this repository uses the reftable format\n",
		".git/objects", "/", ".git/reftable/tables.list", "x\n")
	// A .git folder that holds only a HEAD isn't a repository to git, which goes on to the one above it, on main.
	onMain := testRepo(t, "main", "")
	fake := filepath.Join(onMain, "sub")
	write(fake, ".git/HEAD", "ref: refs/heads/feature\n")
	// Past 32 remotes, a default may go unread.
	var remotes []string
	for i := range 33 {
		remotes = append(remotes, "refs/remotes/r"+strings.Repeat("x", i)+"/HEAD", "ref: refs/remotes/r/trunk\n")
	}
	many := testRepo(t, "feature", "", remotes...)
	detachedRebase := testRepo(t, strings.Repeat("c", 40), "", "rebase-merge/head-name", "detached HEAD\n")
	onTag := testRepo(t, "feature", "")
	write(onTag, ".git/HEAD", "ref: refs/tags/v1\n")
	for name, c := range map[string]struct {
		cwd, line string
		known     bool
	}{
		"reftable": {reftable, "git merge x", false}, "reftable, a branch named": {reftable, "git branch -f feature/y x", false},
		"a fake .git": {fake, "git merge x", true}, "past 32 remotes": {many, "git merge x", false},
		"a detached HEAD rebased": {detachedRebase, "git rebase --continue", true}, "HEAD on a tag": {onTag, "git merge x", true},
		"inside .git": {filepath.Join(onMain, ".git", "refs"), "git update-ref HEAD x", true},
	} {
		if got := mergeRefusal(c.line, c.cwd, false); !strings.HasPrefix(got, MergeGroup+": ") {
			t.Errorf("%s: %q not refused (%q)", name, c.line, got)
		}
		if PlaceOf(c.cwd).Known != c.known {
			t.Errorf("%s: %+v", name, PlaceOf(c.cwd))
		}
	}
	if p := PlaceOf(fake); p.Branch != "refs/heads/main" {
		t.Errorf("the fake .git was read: %+v", p)
	}
	// Under 32 remotes, on a feature branch, the hire still merges.
	if got := mergeRefusal("git merge main", testRepo(t, "feature", "", remotes[2:]...), false); got != "" {
		t.Errorf("32 remotes: %s", got)
	}
}

// The hook reads the call's folder (Claude Code's cwd), and a desk whose environment names another git folder
// can't be placed.
func TestTheHookPlacesTheCallInItsFolder(t *testing.T) {
	onMain, onFeature := testRepo(t, "main", ""), testRepo(t, "feature", "")
	says := func(cwd string, args ...string) bool {
		t.Helper()
		call, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "git merge x"}, "cwd": cwd})
		var out strings.Builder
		Run(append(args, "--rules", filepath.Join(t.TempDir(), fileName)), strings.NewReader(string(call)), &out, &out)
		return strings.Contains(out.String(), "(merge: ")
	}
	held := GroupsArgument(DefaultGroups()) // a core's copy from before FS3: the hook adds the groups
	for _, args := range [][]string{nil, {"--profile", ProfileGM}, {"--groups", held}, {"--profile", ProfileGM, "--groups", held}} {
		if !says(onMain, args...) || says(onFeature, args...) || !says("", args...) {
			t.Errorf("%v: on main, on a feature, nowhere: %v %v %v", args, says(onMain, args...), says(onFeature, args...), says("", args...))
		}
	}
	t.Setenv("GIT_DIR", filepath.Join(onMain, ".git"))
	if !says(onFeature) {
		t.Error("GIT_DIR in the desk's environment was followed")
	}
}

// The gate groups come with every profile where the file (or the core's copy) doesn't name them; one it names keeps
// what it says. The six groups of the guard file, and so every row of theirs, are as they were.
func TestTheGateGroupsComeWithEveryProfile(t *testing.T) {
	names := func(groups []Group) (out []string) {
		for _, g := range groups {
			if g.On {
				out = append(out, g.Name)
			}
		}
		return out
	}
	six := []string{"push", "deploy", "publish", "cloud", "remote", "local"}
	if !slices.Equal(names(DefaultGroups()), six) {
		t.Fatalf("the default groups changed: %v", names(DefaultGroups()))
	}
	all := append(slices.Clone(six), MergeGroup, SpendGroup)
	dir := t.TempDir()
	before := filepath.Join(dir, "before.json") // what Settings wrote before FS3
	if err := os.WriteFile(before, DefaultFile(), 0o600); err != nil {
		t.Fatal(err)
	}
	mergeOff := filepath.Join(dir, "off.json")
	if err := os.WriteFile(mergeOff, []byte(`{"groups":[{"name":"push","on":true,"commands":["git push"]},{"name":"merge","on":false,"commands":[]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{ProfileHire, ProfileGM} {
		for _, path := range []string{"", before, filepath.Join(dir, "none.json")} {
			if groups, problem := Profiled(path, profile); problem != "" || !slices.Equal(names(groups), all) {
				t.Errorf("%s %q: %v %q", profile, path, names(groups), problem)
			}
			if groups, problem := Held(GroupsArgument(DefaultGroups()), path, profile); problem != "" || !slices.Equal(names(groups), all) {
				t.Errorf("held, %s %q: %v %q", profile, path, names(groups), problem)
			}
		}
		if groups, _ := Profiled(mergeOff, profile); !slices.Equal(names(groups), []string{"push", SpendGroup}) {
			t.Errorf("%s, merge named off: %v", profile, names(groups))
		}
		// The core's copy says off; a hire's file can't put that back on or take it away.
		core := GroupsArgument(append(DefaultGroups(), Group{Name: MergeGroup, On: false}))
		if groups, _ := Held(core, mergeOff, profile); slices.Contains(names(groups), MergeGroup) {
			t.Errorf("%s: the core's merge off came back on", profile)
		}
		if groups, _ := Held(GroupsArgument(DefaultGroups()), mergeOff, profile); !slices.Contains(names(groups), MergeGroup) {
			t.Errorf("%s: a hire's file switched merge off", profile)
		}
	}
}

// Spend is a short list, and none of it is a deny rule: the gate service must see the line to ask the owner.
func TestTheSpendGroupIsAShortListTheGuardAloneReads(t *testing.T) {
	groups := At(WithGates(DefaultGroups()), Place{})
	for _, line := range []string{"stripe --live charges create --amount 100", "stripe payment_intents create --api-key sk_x",
		"doctl compute droplet create web", "hcloud server create --name x", "linode-cli linodes create", "vultr-cli instance create",
		"scw instance server create", "openai api chat.completions.create -m x", "npx stripe --live customers list"} {
		if r := Check(line, groups); r == nil || r.Group != SpendGroup {
			t.Errorf("%q: %v", line, r)
		}
	}
	for _, line := range []string{"stripe listen", "stripe trigger payment_intent.succeeded", "doctl compute droplet list",
		"hcloud server list", "openai --help", "pip install openai"} {
		if r := Check(line, groups); r != nil {
			t.Errorf("%q: %v", line, r)
		}
	}
	for _, g := range GateGroups() {
		for _, rule := range g.Commands {
			if !slices.ContainsFunc(ruleWords(rule), func(w string) bool { return w == "..." || w[0] == '-' || w[0] == '@' }) {
				t.Errorf("%s's %q would be a deny rule", g.Name, rule)
			}
		}
	}
}
