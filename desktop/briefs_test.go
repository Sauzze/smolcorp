package desktop

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentdeck "github.com/Sauzze/smolcorp"
)

// What the shared fixture (briefs_fixture_test.go) can't show: briefs written
// to disk under names nobody has, git's branch read from its files, the
// library you can edit, the routes as setup asks them, and a later part
// added to a brief. The words are the Mac's (Tests/AgentDeckTests/BriefTests.swift).

func TestABriefIsWrittenUnderTheFirstNamesNobodyHas(t *testing.T) {
	isolate(t)
	role := Role{ID: "tester", Name: "Tester", Agent: "claude", Brief: &RoleBrief{Kind: "library", ID: "tester"}}
	library := briefLibrary{files: bundledBriefs()}
	project := t.TempDir()
	date := time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC)
	written := func(managed string, pinned bool) *briefMade {
		t.Helper()
		made, err := makeBrief(briefRequest{role: role, library: library, folder: project, pinned: pinned, managed: managed, date: date,
			kept: briefsKeptDir(), write: true})
		if err != nil || made == nil {
			t.Fatalf("not made: %v", err)
		}
		return made
	}
	// At a pinned desk, in the project: BRIEF.md, then BRIEF-tester.md, then BRIEF-tester-2.md; an owner's STATUS.md counts as taken.
	if err := os.WriteFile(filepath.Join(project, "STATUS.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BRIEF-tester.md", "BRIEF-tester-2.md"} {
		made := written("", true)
		if filepath.Base(made.names.brief) != want || !strings.Contains(made.firstMessage, want) {
			t.Errorf("brief %s, message %q; want %s", made.names.brief, made.firstMessage, want)
		}
		data, _ := os.ReadFile(made.names.brief)
		if string(data) != made.markdown {
			t.Errorf("%s holds something else than its brief", want)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(project, "STATUS.md")); string(data) != "mine" {
		t.Errorf("the owner's STATUS.md was written over: %q", data)
	}
	// A managed hire's, in its own folder: BRIEF.md then BRIEF-2.md, the folder for you alone.
	managed := hireFolder(supportDir, "0123abcd4567")
	first, second := written(managed, false), written(managed, false)
	if first.names.brief != filepath.Join(managed, "BRIEF.md") || second.names.brief != filepath.Join(managed, "BRIEF-2.md") {
		t.Errorf("managed briefs: %s, %s", first.names.brief, second.names.brief)
	}
	if info, err := os.Stat(managed); err != nil || !modeIs(info, 0o700) {
		t.Errorf("the hire's folder: %v %v", info, err)
	}
	// Elsewhere, in Smolcorp's briefs folder, named for the desk, the role and the minute.
	kept := written("", false)
	if kept.names.brief != filepath.Join(briefsKeptDir(), "desk-tester-20270115-0800.md") {
		t.Errorf("kept brief: %s", kept.names.brief)
	}
	// A link where the brief would go is never written through.
	if runtime.GOOS != "windows" {
		target := filepath.Join(t.TempDir(), "elsewhere.md")
		next := filepath.Join(briefsKeptDir(), "desk-tester-20270115-0800-2.md")
		if err := os.Symlink(target, next); err != nil {
			t.Fatal(err)
		}
		if made := written("", false); made.names.brief != filepath.Join(briefsKeptDir(), "desk-tester-20270115-0800-3.md") {
			t.Errorf("past the link: %s", made.names.brief)
		}
		if _, err := os.Stat(target); err == nil {
			t.Error("the brief was written through a link")
		}
	}
	// A role on its own instructions has no brief.
	plain := role
	plain.Brief = &RoleBrief{Kind: "text", Text: "Do it."}
	if made, err := makeBrief(briefRequest{role: plain, library: library, folder: project, date: date, kept: briefsKeptDir(), write: true}); made != nil || err != nil {
		t.Errorf("a role on its instructions: %v %v", made, err)
	}
	// A library brief this copy hasn't is said so.
	if _, err := makeBrief(briefRequest{role: role, library: briefLibrary{files: os.DirFS(t.TempDir())}, folder: project, date: date}); err == nil ||
		err.Error() != "The brief tester isn't in this copy of Smolcorp." {
		t.Errorf("no library: %v", err)
	}
}

func TestTheBranchIsReadFromGitsOwnFiles(t *testing.T) {
	repo := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/x\n")
	write(filepath.Join(repo, "sub", "deeper", "a.txt"), "")
	if got := briefBranch(filepath.Join(repo, "sub", "deeper")); got != "feature/x" {
		t.Errorf("from a folder inside: %q", got)
	}
	// A worktree's .git is a file naming its folder.
	worktree := t.TempDir()
	write(filepath.Join(worktree, ".git"), "gitdir: "+filepath.Join(repo, ".git", "worktrees", "w")+"\n")
	write(filepath.Join(repo, ".git", "worktrees", "w", "HEAD"), "ref: refs/heads/hire/w\n")
	if got := briefBranch(worktree); got != "hire/w" {
		t.Errorf("a worktree: %q", got)
	}
	// None checked out, or no git at all.
	write(filepath.Join(repo, ".git", "HEAD"), "0123456789abcdef0123456789abcdef01234567\n")
	if got := briefBranch(repo); got != "" {
		t.Errorf("detached: %q", got)
	}
	if got := briefBranch(t.TempDir()); got != "" {
		t.Errorf("no git: %q", got)
	}
}

func TestTheLibraryYouEditIsCopiedOnceAndPutBack(t *testing.T) {
	isolate(t)
	mine := briefTemplatesDir()
	library := editableBriefs(bundledBriefs(), mine)
	if len(library.entries()) != len(briefLibraryIDs) {
		t.Fatalf("entries: %d", len(library.entries()))
	}
	if info, err := os.Stat(mine); err != nil || !modeIs(info, 0o700) {
		t.Errorf("the folder: %v %v", info, err)
	}
	if info, err := os.Stat(filepath.Join(mine, "tester.md")); err != nil || !modeIs(info, 0o600) {
		t.Errorf("a brief: %v %v", info, err)
	}
	// An edit is kept, and used.
	base := filepath.Join(mine, "_base.md")
	if err := os.WriteFile(base, []byte("Edited base {{> about}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(mine, "notes.txt")
	_ = os.WriteFile(other, []byte("mine"), 0o600)
	library = editableBriefs(bundledBriefs(), mine)
	if template, _ := library.template("tester"); !strings.Contains(template, "Edited base") {
		t.Errorf("the edit wasn't used: %q", template)
	}
	// Restore defaults puts the app's own back, and leaves anything else.
	answer := briefsRestore(bundledBriefs(), mine)
	if answer.status != http.StatusOK || answer.json.(map[string]any)["restored"] != len(briefLibraryFiles()) {
		t.Errorf("restore: %+v", answer)
	}
	if data, _ := os.ReadFile(base); strings.Contains(string(data), "Edited") {
		t.Error("the edit is still there")
	}
	if data, _ := os.ReadFile(other); string(data) != "mine" {
		t.Error("a file of yours was touched")
	}
	// Without the app's own files, nothing is put back.
	if answer := briefsRestore(os.DirFS(t.TempDir()), mine); answer.status != http.StatusInternalServerError {
		t.Errorf("nothing to restore: %+v", answer)
	}
}

func TestSetupAsksForTheBriefsTheFoldersAndTheGuard(t *testing.T) {
	d := newTestDeck(t)
	ask := func(c call) (int, map[string]any, string) {
		t.Helper()
		c.host, c.from = "localhost:7710", thisComputer
		if c.method == "POST" {
			c.headers = map[string]string{setupHeader: "1"}
		}
		w := serve(t, d, c)
		var answer map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &answer)
		return w.Code, answer, w.Body.String()
	}
	code, answer, _ := ask(call{method: "GET", target: "/setup/briefs"})
	if briefs, _ := answer["briefs"].([]any); code != http.StatusOK || len(briefs) != len(briefLibraryIDs) {
		t.Fatalf("briefs: %d %v", code, answer)
	}
	if code, answer, _ = ask(call{method: "GET", target: "/setup/briefs/tester"}); code != http.StatusOK ||
		!strings.HasPrefix(answer["markdown"].(string), "---\nname: Tester\n") {
		t.Errorf("one brief: %d %v", code, answer)
	}
	if code, _, text := ask(call{method: "GET", target: "/setup/briefs/nobody"}); code != http.StatusNotFound || text != "There's no brief nobody." {
		t.Errorf("no such brief: %d %q", code, text)
	}
	// A tool may read them, as on the Mac; only the page restores them.
	if code, _, _ := ask(call{method: "GET", target: "/setup/briefs", asTool: true}); code != http.StatusOK {
		t.Errorf("a tool reading the briefs: %d", code)
	}
	if code, _, _ := ask(call{method: "POST", target: "/setup/briefs/restore", asTool: true, body: "{}"}); code != http.StatusForbidden {
		t.Errorf("a tool restoring the briefs: %d", code)
	}
	if code, answer, _ := ask(call{method: "POST", target: "/setup/briefs/restore", body: "{}"}); code != http.StatusOK || answer["ok"] != true {
		t.Errorf("the page restoring the briefs: %d %v", code, answer)
	}
	if code, answer, _ := ask(call{method: "POST", target: "/setup/briefs/preview", body: `{"role":"tester","folder":"~/Projects/app"}`}); code != http.StatusOK ||
		answer["kind"] != "library" || !strings.Contains(answer["markdown"].(string), "# Brief: Tester") {
		t.Errorf("a preview: %d %v", code, answer)
	}
	// The folders: the one set up, and those hired into lately that are still there.
	there := t.TempDir()
	d.mu.Lock()
	d.config.HireFolder = there
	d.config.RecentHireFolders = []string{there, filepath.Join(there, "gone")}
	d.mu.Unlock()
	code, answer, _ = ask(call{method: "GET", target: "/setup/hire/folders"})
	if recent, _ := answer["recent"].([]any); code != http.StatusOK || answer["default"] != there || len(recent) != 1 || recent[0] != there {
		t.Errorf("hire folders: %d %v", code, answer)
	}
	// The guard: every group on with no file, and a tool may read it.
	code, answer, _ = ask(call{method: "GET", target: "/setup/guard", asTool: true})
	if groups, _ := answer["groups"].([]any); code != http.StatusOK || len(groups) != 6 || answer["limits"] == "" || answer["problem"] != nil {
		t.Errorf("guard: %d %v", code, answer)
	}
	// Setup's own hire sheet is served here now (POST /setup/hire, setuphire.go): the state no longer turns it off.
	code, answer, _ = ask(call{method: "GET", target: "/setup/state"})
	if hiring, _ := answer["hiring"].(map[string]any); code != http.StatusOK || hiring == nil || hiring["fromSetup"] != nil {
		t.Errorf("setup state's hiring: %d %v", code, answer["hiring"])
	}
	page, err := fs.ReadFile(agentdeck.Files, "Resources/web/setup.html")
	if err != nil || !strings.Contains(string(page), "if (canHireHere) return openHireSheet(role);") {
		t.Errorf("the page doesn't open its hire sheet where the app can hire (%v)", err)
	}
	// Nothing here answers anyone but this computer's setup.
	if code := serve(t, d, call{method: "GET", target: "/setup/briefs", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}).Code; code != http.StatusUnauthorized {
		t.Errorf("without the key: %d", code)
	}
}

func TestRecentHireFoldersAreKeptAsTheMacKeepsThem(t *testing.T) {
	isolate(t)
	if err := os.WriteFile(configFile, []byte(`{"recentHireFolders":["/a","relative","/b","/c","/d","/e","/f","/g","/h","/i"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadConfig().RecentHireFolders
	if runtime.GOOS != "windows" && strings.Join(got, ",") != "/a,/b,/c,/d,/e,/f,/g,/h" {
		t.Errorf("kept %v", got)
	}
}

func TestALaterPartIsAddedToTheHiresOwnBriefOnly(t *testing.T) {
	isolate(t)
	hires := filepath.Join(supportDir, "hires")
	date := time.Date(2027, 1, 15, 8, 30, 0, 0, time.UTC)
	// The path written down at hire.
	written := filepath.Join(t.TempDir(), "BRIEF-tester.md")
	_ = os.WriteFile(written, []byte("# Brief\n"), 0o644)
	if got := briefFileOf(Hire{ID: "a", Brief: written}, hires); got != written {
		t.Errorf("the written brief: %q", got)
	}
	if err := appendBriefPart(written, "  Also the login page.\n", date); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(written); string(data) != "# Brief\n\n\n## New part, 2027-01-15 08:30\n\nAlso the login page.\n" {
		t.Errorf("after the part: %q", data)
	}
	// A managed hire from before it was written down: the first BRIEF*.md in its own folder.
	folder := filepath.Join(hires, "b")
	_ = os.MkdirAll(folder, 0o700)
	_ = os.WriteFile(filepath.Join(folder, "BRIEF-2.md"), []byte("two"), 0o644)
	_ = os.WriteFile(filepath.Join(folder, "BRIEF.md"), []byte("one"), 0o644)
	if got := briefFileOf(Hire{ID: "b", Worktree: "/w"}, hires); got != filepath.Join(folder, "BRIEF-2.md") {
		t.Errorf("a managed hire's: %q", got)
	}
	// Not managed, or the file gone or a link: none.
	if got := briefFileOf(Hire{ID: "b"}, hires); got != "" {
		t.Errorf("not managed: %q", got)
	}
	if got := briefFileOf(Hire{ID: "c", Brief: filepath.Join(folder, "gone.md")}, hires); got != "" {
		t.Errorf("gone: %q", got)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(folder, "BRIEF-link.md")
		_ = os.Symlink(written, link)
		if got := briefFileOf(Hire{ID: "c", Brief: link}, hires); got != "" {
			t.Errorf("a link: %q", got)
		}
		if err := appendBriefPart(link, "x", date); err == nil {
			t.Error("added through a link")
		}
	}
	// Another file put in its place between the check and the open (a link, on Windows, where the open
	// can't refuse one): nothing is added to either.
	other := filepath.Join(t.TempDir(), "other.md")
	_ = os.WriteFile(other, []byte("other"), 0o644)
	before, _ := os.ReadFile(written)
	kept := openBriefFile
	t.Cleanup(func() { openBriefFile = kept })
	openBriefFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return os.OpenFile(other, flag, perm)
	}
	if err := appendBriefPart(written, "x", date); err == nil {
		t.Error("added to a file swapped in after the check")
	}
	if data, _ := os.ReadFile(other); string(data) != "other" {
		t.Errorf("the swapped-in file reads %q", data)
	}
	if data, _ := os.ReadFile(written); string(data) != string(before) {
		t.Errorf("the brief reads %q", data)
	}
}
