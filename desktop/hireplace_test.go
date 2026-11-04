package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const placeTestKey = "fake-key-0123456789ABCDEFGHIJ"

// placeRepo makes a git repo with one commit in a temp folder, unsigned, and
// returns its symlink-resolved path.
func placeRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Resolved once it exists: a path that isn't there yet can't be (on a Mac, /var is /private/var).
	repo = physical(repo)
	os.WriteFile(filepath.Join(repo, "sub", "a.txt"), []byte("a"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "one"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return repo
}

// placeOptions are options with free ports, a key, and worktrees in a temp folder.
func placeOptions(t *testing.T) hirePlaceOptions {
	return hirePlaceOptions{Worktrees: t.TempDir(), PortFree: func(int) bool { return true }, Key: func() string { return placeTestKey }}
}

// writeConfig writes the default account's .claude.json in the isolated home.
func writeConfig(t *testing.T, home, text string) string {
	t.Helper()
	file := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func branches(t *testing.T, repo string) string {
	t.Helper()
	out, _ := exec.Command("git", "-C", repo, "branch", "--list", "hire/*").Output()
	return strings.TrimSpace(string(out))
}

func TestAHireGetsAWorktreeTrustedByItsResolvedPathAndPorts(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	file := writeConfig(t, home, "{\n  \"numStartups\": 3\n}\n")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	// Asked for through a symlink to a folder inside the repo.
	place, err := prepareHire(hireRequest{HireID: "0123456789ab", Name: "Ada Lovelace!", Folder: filepath.Join(link, "sub"), Claude: true, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if place.Branch != "hire/ada-lovelace-012345" || place.Repo != repo || place.Ports != "4100-4103" {
		t.Fatalf("place = %+v", place)
	}
	if place.Folder != filepath.Join(place.Worktree, "sub") || place.Worktree != physical(place.Worktree) {
		t.Fatalf("folder %q worktree %q", place.Folder, place.Worktree)
	}
	if _, err := os.Stat(filepath.Join(place.Folder, "a.txt")); err != nil {
		t.Fatal("the worktree has no checkout:", err)
	}
	data, _ := os.ReadFile(file)
	if !isTrusted(place.Worktree, data) || !strings.Contains(string(data), "\"numStartups\": 3") {
		t.Fatalf("worktree not trusted by its resolved path, or other bytes changed: %s", data)
	}
	if info, _ := os.Stat(file); !modeIs(info, 0o600) {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestAnAPIKeyHireAlwaysApprovesItsKeyTail(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	file := writeConfig(t, home, "{}")
	place, err := prepareHire(hireRequest{HireID: "aaaaaaaaaaaa", Name: "x", Folder: repo, Claude: true, APIKey: true, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	if !isApproved(keyTail(placeTestKey), data) || keyTail(placeTestKey) != "0123456789ABCDEFGHIJ" || !isTrusted(place.Worktree, data) {
		t.Fatalf("key tail not approved: %s", data)
	}
	// A subscription hire approves nothing.
	other := writeConfig(t, home, "{}")
	if _, err := prepareHire(hireRequest{HireID: "bbbbbbbbbbbb", Name: "x", Folder: repo, Claude: true, Worktree: true}, placeOptions(t)); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(other); strings.Contains(string(data), "customApiKeyResponses") {
		t.Fatalf("subscription hire approved a key: %s", data)
	}
}

func TestAnAPIKeyHireWithoutAKeyIsRefusedWithNothingMade(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	file := writeConfig(t, home, "{}")
	o := placeOptions(t)
	o.Key = func() string { return " \n" }
	_, err := prepareHire(hireRequest{HireID: "cccccccccccc", Name: "x", Folder: repo, Claude: true, APIKey: true, Worktree: true}, o)
	if err != errHireNoKey {
		t.Fatalf("err = %v", err)
	}
	if data, _ := os.ReadFile(file); string(data) != "{}" || branches(t, repo) != "" {
		t.Fatalf("something was made: %s %q", data, branches(t, repo))
	}
}

func TestAMissingClaudeJSONIsRefusedAndNothingIsMadeOrCreated(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	_, err := prepareHire(hireRequest{HireID: "dddddddddddd", Name: "x", Folder: repo, Claude: true, Worktree: true}, o)
	if err != errHireNoConfig {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude.json")); statErr == nil {
		t.Fatal("a .claude.json was created")
	}
	if entries, _ := os.ReadDir(o.Worktrees); len(entries) != 0 || branches(t, repo) != "" {
		t.Fatal("a worktree or branch was made")
	}
	// A config in a shape we don't know is refused the same way.
	writeConfig(t, home, "[1,2]")
	if _, err := prepareHire(hireRequest{HireID: "dddddddddddd", Name: "x", Folder: repo, Claude: true, Worktree: true}, o); err != errHireTrust {
		t.Fatalf("err = %v", err)
	}
}

func TestIfPreTrustRefusesTheWorktreeAndBranchAreRemoved(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	file := writeConfig(t, home, "{}")
	o := placeOptions(t)
	// The config turns into a shape we can't edit just as the worktree is made.
	o.Git = func(dir string, args ...string) (int, string, bool) {
		status, out, ok := runGit(dir, args...)
		if len(args) > 1 && args[1] == "add" {
			os.WriteFile(file, []byte("[]"), 0o600)
		}
		return status, out, ok
	}
	_, err := prepareHire(hireRequest{HireID: "eeeeeeeeeeee", Name: "x", Folder: repo, Claude: true, Worktree: true}, o)
	// The same 409 as the up-front check, not a 500.
	if refusal, ok := err.(*hireRefusal); !ok || refusal.Status != 409 || refusal != errHireTrust {
		t.Fatalf("err = %v, want errHireTrust (409)", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(o.Worktrees, filepath.Base(repo))); len(entries) != 0 || branches(t, repo) != "" {
		t.Fatalf("worktree or branch left behind: %v %q", entries, branches(t, repo))
	}
}

func TestAConfigThatIsASymlinkIsWrittenThroughAndKept(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	target := filepath.Join(t.TempDir(), "dotfiles-claude.json")
	os.WriteFile(target, []byte("{}"), 0o600)
	if err := os.Symlink(target, filepath.Join(home, ".claude.json")); err != nil {
		t.Fatal(err)
	}
	place, err := prepareHire(hireRequest{HireID: "ffffffffffff", Name: "x", Folder: repo, Claude: true, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(filepath.Join(home, ".claude.json")); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced")
	}
	if data, _ := os.ReadFile(target); !isTrusted(place.Worktree, data) {
		t.Fatalf("target not trusted: %s", data)
	}
}

func TestAFolderThatIsNotARepoIsHiredIntoAsItIs(t *testing.T) {
	home := isolate(t)
	writeConfig(t, home, "{}")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir())) // no repo above a temp folder, wherever it is
	real := physical(t.TempDir())
	folder := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(real, folder); err != nil {
		t.Fatal(err)
	}
	place, err := prepareHire(hireRequest{HireID: "111111111111", Name: "x", Folder: folder, Claude: true, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	// Trusted by its resolved path, how Claude Code knows a folder; the hire starts where it was asked.
	if place.Note != notARepo || place.Worktree != "" || place.Folder != folder || !isTrusted(real, data) || isTrusted(folder, data) {
		t.Fatalf("place = %+v, config %s", place, data)
	}
}

func TestAStrayAPIKeyHelperIsRefusedUnlessTheAPIKeyAccountWasPicked(t *testing.T) {
	home := isolate(t)
	repo := placeRepo(t)
	writeConfig(t, home, "{}")
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	helper := filepath.Join(home, ".claude", "settings.json")
	os.WriteFile(helper, []byte(`{"apiKeyHelper":"x"}`), 0o600)
	_, err := prepareHire(hireRequest{HireID: "222222222222", Name: "x", Folder: repo, Claude: true, Worktree: true}, placeOptions(t))
	if refusal, ok := err.(*hireRefusal); !ok || refusal.Status != 409 || !strings.HasPrefix(refusal.Message, helper+" sets an apiKeyHelper") {
		t.Fatalf("err = %v", err)
	}
	if branches(t, repo) != "" {
		t.Fatal("a branch was made")
	}
	if _, err := prepareHire(hireRequest{HireID: "222222222222", Name: "x", Folder: repo, Claude: true, APIKey: true, Worktree: true}, placeOptions(t)); err != nil {
		t.Fatal(err)
	}
}

func TestPortsAreTheFirstFreeRangeAwayFromTakenOnes(t *testing.T) {
	all := func(int) bool { return true }
	for _, c := range []struct {
		taken []string
		free  func(int) bool
		want  string
	}{
		{nil, all, "4100-4103"},
		{[]string{"4100-4103"}, all, "4104-4107"},
		{[]string{"4102-4105", "junk", "9-1"}, all, "4106-4109"},
		{nil, func(p int) bool { return p != 4101 }, "4102-4105"},
		{[]string{"4100-4995"}, all, "4996-4999"},
		{[]string{"4100-4996"}, all, ""},
		{[]string{"0-99999999999"}, all, ""},
		{[]string{"-5-3", "4100-4103", "5000-6000"}, all, "4104-4107"},
		{nil, func(int) bool { return false }, ""},
	} {
		if got := allocatePorts(c.taken, c.free); got != c.want {
			t.Errorf("allocatePorts(%v) = %q, want %q", c.taken, got, c.want)
		}
	}
}

func TestRemoveWorkspaceKeepsAWorktreeThatHoldsChanges(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	for _, dirty := range []bool{false, true} {
		name := "w-" + strings.Repeat("d", map[bool]int{false: 1, true: 2}[dirty])
		path := filepath.Join(t.TempDir(), name)
		if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "hire/"+name, path).CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		if dirty {
			os.WriteFile(filepath.Join(path, "new.txt"), []byte("work"), 0o644)
		}
		if got := removeWorkspace(repo, path, "hire/"+name, runGit); got == dirty {
			t.Errorf("dirty=%v: removed = %v", dirty, got)
		}
		if _, err := os.Stat(path); (err == nil) != dirty {
			t.Errorf("dirty=%v: worktree present = %v", dirty, err == nil)
		}
	}
}

// gitTimesOutOnAdd is git as if `worktree add` was stopped for taking too long.
func gitTimesOutOnAdd(dir string, args ...string) (int, string, bool) {
	if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
		return 0, "", false
	}
	return runGit(dir, args...)
}

func TestAHireIDThatIsAPathNeverTouchesAnythingOutsideTheHiresFolder(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	o.Git = gitTimesOutOnAdd
	sentinel := filepath.Join(o.Worktrees, "sentinel")
	os.WriteFile(sentinel, []byte("keep"), 0o644)
	beside := filepath.Join(filepath.Dir(repo), "beside")
	os.WriteFile(beside, []byte("keep"), 0o644)
	for _, id := range []string{"/../..", "../../..", "..", "/", ""} {
		_, err := prepareHire(hireRequest{HireID: id, Name: "x", Folder: repo, Worktree: true}, o)
		if refusal, ok := err.(*hireRefusal); !ok || refusal.Status != 409 {
			t.Fatalf("id %q: err = %v", id, err)
		}
		for _, file := range []string{sentinel, beside, repo} {
			if _, err := os.Stat(file); err != nil {
				t.Fatalf("id %q: %s was removed", id, file)
			}
		}
	}
}

func TestAHireIDIsOnlyEverASlugInThePathAndBranch(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	place, err := prepareHire(hireRequest{HireID: "/../..", Name: "x", Folder: repo, Worktree: true}, o)
	if err != nil {
		t.Fatal(err)
	}
	if place.Worktree != filepath.Join(physical(o.Worktrees), filepath.Base(repo), "x-hire") || place.Branch != "hire/x-hire" {
		t.Fatalf("place = %+v", place)
	}
}

func TestDiscardHalfMadeRefusesAnythingNotStrictlyInsideItsParent(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	inside := filepath.Join(parent, "x-abc")
	outside := filepath.Join(root, "outside")
	for _, dir := range []string{inside, outside} {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "f"), []byte("keep"), 0o644)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{parent, root, outside, filepath.Join(parent, "x", "..", ".."), link, filepath.Join(parent, "..")} {
		discardHalfMade(repo, parent, path, "hire/none", runGit)
		if _, err := os.Stat(filepath.Join(outside, "f")); err != nil {
			t.Fatalf("%s: outside removed", path)
		}
		if _, err := os.Stat(parent); err != nil {
			t.Fatalf("%s: parent removed", path)
		}
	}
	discardHalfMade(repo, parent, inside, "hire/none", runGit)
	if _, err := os.Stat(inside); err == nil {
		t.Fatal("the hire's own folder was left")
	}
}

func TestNoFreePortsIsA409ThroughPrepareHireWithNothingMade(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	o.PortFree = func(int) bool { return false }
	_, err := prepareHire(hireRequest{HireID: "333333333333", Name: "x", Folder: repo, Worktree: true}, o)
	if refusal, ok := err.(*hireRefusal); !ok || refusal.Status != 409 || !strings.Contains(refusal.Message, "No free ports") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(o.Worktrees); len(entries) != 0 || branches(t, repo) != "" {
		t.Fatalf("something was made: %v %q", entries, branches(t, repo))
	}
}

func TestWithoutWorktreeAHireInARepoWorksInItsFolder(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	folder := filepath.Join(repo, "sub")
	place, err := prepareHire(hireRequest{HireID: "444444444444", Name: "x", Folder: folder, Worktree: false}, o)
	if err != nil {
		t.Fatal(err)
	}
	if place.Folder != folder || place.Worktree != "" || place.Branch != "" || place.Note != "" || branches(t, repo) != "" {
		t.Fatalf("place = %+v", place)
	}
	if entries, _ := os.ReadDir(o.Worktrees); len(entries) != 0 {
		t.Fatalf("a folder was made: %v", entries)
	}
}

func TestHireSlugKeepsLettersDigitsAndDashesAndAtMost30(t *testing.T) {
	for text, want := range map[string]string{
		"":                      "hire",
		"///":                   "hire",
		"Ünï":                   "n",
		" Ada  Lovelace! ":      "ada-lovelace",
		strings.Repeat("a", 45): strings.Repeat("a", 30),
		strings.Repeat("a", 30): strings.Repeat("a", 30),
	} {
		if got := hireSlug(text); got != want {
			t.Errorf("hireSlug(%q) = %q, want %q", text, got, want)
		}
	}
}
