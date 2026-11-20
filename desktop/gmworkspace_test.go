//go:build !windows

package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGMROOT (RT126SWEEP2 P1 B1), the engine's side, with the stand-in start-only runner of desksandbox_test.go: the GM
// hired into a folder that isn't a repo works in a workspace of its own in its sandbox (made 0700 at its hire, taken
// away at its fire), starts in that folder and reads it, is told so in its start words with the workspace's real
// path, and its settings make the workspace a working folder of Claude Code's; every other desk is kept out of the
// workspace alone. A GM in a repo is as before. Under the real profile: gmroot_darwin_test.go.

// gmProjectsFolder is a folder for hires that isn't a repo, ~/Projects as Setup and Start here hire the GM into.
func gmProjectsFolder(t *testing.T) string {
	t.Helper()
	folder := filepath.Join(homeDir, "Projects")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	return folder
}

func TestTheGMInTheProjectsFolderWorksInAWorkspaceOfItsOwn(t *testing.T) {
	r := newSandboxRig(t)
	windowShows(r.engineRig, pane(t, "idle-empty"), true)
	request := r.request(gmRole)
	request.Folder = gmProjectsFolder(t)
	var started string
	request.StartTyped = func(message string) { started = message }
	gm, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	workspace := sandbox.Workspace(supportDir, gm.ID)
	pl := r.desks[0].places
	if pl.Subject != sandbox.GM || pl.Root != physical(workspace) || pl.GMProjects != physical(request.Folder) ||
		pl.GMPlans != filepath.Join(physical(request.Folder), ".smolcorp", "plans", gm.ID) || pl.GitDir != "" || pl.Checkout != "" {
		t.Errorf("its places: %+v", pl)
	}
	if info, err := os.Lstat(workspace); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("its workspace: %v %v", info, err)
	}
	// It still starts in the folder it was hired into.
	if row := registered(t)[0]; row.Folder.storedOnly() != request.Folder || row.storedStart() != request.Folder {
		t.Errorf("its row: %+v", row)
	}
	// Its start words say where it writes, its workspace and its plans folder, with the real paths, and what to tell
	// the owner (the owner's ruling on D1: the owner knows where plans live).
	plans := filepath.Join(request.Folder, ".smolcorp", "plans", gm.ID)
	for _, want := range []string{"You read the projects in " + request.Folder, "Your workspace is " + workspace + ":",
		"except your plan files, which Claude Code keeps in " + plans + ".",
		`"I read your projects; hires change the code; my own notes go in ` + workspace + `, and my plans in ` + plans + `."`} {
		if !strings.Contains(started, want) {
			t.Errorf("its start words lack %q:\n%s", want, started)
		}
	}
	// Claude Code is told the workspace is a working folder of its own.
	if dirs := settingsDirectories(t, gm.ID); !slices.Equal(dirs, []string{workspace}) {
		t.Errorf("its settings' additionalDirectories: %q", dirs)
	}

	// A hire under it is kept out of the workspace alone, not out of the projects folder its worktree is in.
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.desks[1].places; got.GMFolder != physical(workspace) || got.Root != physical(hire.Worktree.storedOnly()) {
		t.Errorf("the hire's places: %+v", got)
	}
	if dirs := settingsDirectories(t, hire.ID); dirs != nil {
		t.Errorf("a hire's settings name working folders: %q", dirs)
	}

	// Kept across a start made again from its row (a stop's listing, a core restarted): the same workspace, as it is.
	if err := os.WriteFile(filepath.Join(workspace, "notes.md"), []byte("the GM's notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	again := deskAtOf(registryRow(t, gm.ID), deskSocketPath(gm.ID, 0), nil, r.o)
	if again.Root != workspace || again.Projects != request.Folder {
		t.Errorf("made again from its row: %+v", again)
	}
	if err := makeGMWorkspace(again.Root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(workspace, "notes.md")); string(data) != "the GM's notes" {
		t.Error("its workspace wasn't kept as it was")
	}

	// Its fire takes it away with its other files.
	if _, err := fireEngineHire(registryRow(t, gm.ID), 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(workspace); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a fired GM's workspace is still there: %v", err)
	}
}

// (v): the GM in a repo, Setup's "a worktree of its own", is a desk in its worktree as before: no workspace, no
// projects folder, nothing more in its words or settings.
func TestTheGMInARepoIsAsBefore(t *testing.T) {
	r := newSandboxRig(t)
	request := r.request(gmRole)
	var started string
	request.StartTyped = func(message string) { started = message }
	gm, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	pl := r.desks[0].places
	if pl.Subject != sandbox.GM || pl.Root != physical(gm.Worktree.storedOnly()) || pl.Checkout != physical(r.repo) || pl.GitDir == "" ||
		pl.GMProjects != "" || pl.GMPlans != "" {
		t.Errorf("its places: %+v", pl)
	}
	if _, err := os.Lstat(sandbox.Workspace(supportDir, gm.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a GM in a repo got a workspace: %v", err)
	}
	if strings.Contains(started, "my own notes go in") || settingsDirectories(t, gm.ID) != nil {
		t.Errorf("a GM in a repo is told of a workspace:\n%s", started)
	}
	if d := deskAtOf(registryRow(t, gm.ID), "", nil, r.o); d.Root != gm.Worktree.storedOnly() || d.Projects != "" {
		t.Errorf("made again from its row: %+v", d)
	}
}

// Without its sandbox (the owner's switch off, or a computer with none yet), the GM in the projects folder starts as
// before: nothing it can't do is said, and its settings name no workspace.
func TestTheGMOutsideItsSandboxIsToldOfNoWorkspace(t *testing.T) {
	r := newSandboxRig(t)
	r.o.SandboxOff = true
	request := r.request(gmRole)
	request.Folder = gmProjectsFolder(t)
	var started string
	request.StartTyped = func(message string) { started = message }
	gm, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.desks) != 0 || strings.Contains(started, "my own notes go in") || settingsDirectories(t, gm.ID) != nil {
		t.Errorf("sandboxes %d, words:\n%s", len(r.desks), started)
	}
	// A hire in its sandbox under that GM: kept out of the GM's workspace, which isn't there, spelled from the support
	// folder resolved (the test's goes through /var, a link), and not out of the projects folder.
	r.o.SandboxOff = false
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if pl := r.desks[0].places; pl.GMFolder != sandbox.Workspace(physical(supportDir), gm.ID) || pl.Root != physical(hire.Worktree.storedOnly()) {
		t.Errorf("the hire's places: %+v", pl)
	}
}

func TestOnlyAGMOutsideARepoHasAWorkspace(t *testing.T) {
	isolate(t)
	for _, c := range []struct {
		gm                 bool
		id, repo, worktree string
		want               bool
	}{
		{true, "gm1", "", "", true}, {false, "h1", "", "", false}, {true, "gm1", "/p/app", "", false},
		{true, "gm1", "/p/app", "/p/app-hires/gm1", false}, {true, "gm1", "", "/p/app-hires/gm1", false},
		{true, "..", "", "", false}, {true, "a/b", "", "", false}, {true, "", "", "", false},
	} {
		got := gmWorkspace(c.gm, c.id, c.repo, c.worktree)
		if want := filepath.Join(supportDir, "hires", c.id, "work"); c.want && got != want || !c.want && got != "" {
			t.Errorf("%+v: %q", c, got)
		}
	}
}

func TestTheGMsStartWordsSayWhereItWrites(t *testing.T) {
	const work, plans = "/Users/ada/Library/Application Support/Smolcorp/hires/gm1/work", "/Users/ada/Projects/.smolcorp/plans/gm1"
	intro := gmIntroduction("/Users/ada/Projects", work, plans, false)
	for _, want := range []string{"Your folder is /Users/ada/Projects.",
		"You read the projects in /Users/ada/Projects and don't change them: your hires change the code. ",
		"Your workspace is " + work + ": your notes, your briefs, your playbook and your other files go there. ",
		"Your sandbox refuses every other write in /Users/ada/Projects, except your plan files, which Claude Code keeps in " + plans + ". ",
		`"I read your projects; hires change the code; my own notes go in ` + work + `, and my plans in ` + plans + `."`} {
		if !strings.Contains(intro, want) {
			t.Errorf("%q lacks %q", intro, want)
		}
	}
	// O1: plain English, and nothing that isn't always true (a hire may work at the project, not in a worktree).
	for _, gone := range []string{"but your plans'", "in worktrees of their own"} {
		if strings.Contains(intro, gone) {
			t.Errorf("%q still says %q", intro, gone)
		}
	}
	if strings.Contains(gmIntroduction("/Users/ada/Projects", "", "", false), "notes go in") {
		t.Error("a GM with no workspace is told of one")
	}
}

// makeGMWorkspace refuses anything but a folder at its name, and never writes through a link.
func TestAGMsWorkspaceIsAFolderOrTheStartIsRefused(t *testing.T) {
	base := t.TempDir()
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.Mkdir(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	link, file := filepath.Join(base, "link"), filepath.Join(base, "file")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, at := range []string{link, file} {
		if makeGMWorkspace(at) == nil {
			t.Errorf("%s: made", at)
		}
	}
	if err := makeGMWorkspace(filepath.Join(base, "work")); err != nil {
		t.Fatal(err)
	}
}

// settingsDirectories are the working folders desk id's settings give Claude Code, nil for none.
func settingsDirectories(t *testing.T, id string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hireFolder(supportDir, id), hireSettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Permissions struct {
			AdditionalDirectories []string `json:"additionalDirectories"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	return settings.Permissions.AdditionalDirectories
}

// FXGMROOTR1 N1: the link check passes over a GM's workspace in its core's support folder or another one, and nothing
// else: not the GM's own folder, its Claude folder, a folder inside the workspace walked from elsewhere, a folder
// named work one level off, nor one whose desk's name isn't one name.
func TestTheLinkCheckPassesOverAGMsWorkspaceAlone(t *testing.T) {
	base := t.TempDir()
	support, other := filepath.Join(base, "support"), filepath.Join(base, "test-copy")
	for _, c := range []struct {
		folder string
		want   bool
	}{
		{sandbox.Workspace(support, "gm1"), true}, {sandbox.Workspace(other, "gm1"), true}, {sandbox.Workspace(support, "h2"), true},
		{filepath.Join(support, "hires", "gm1"), false}, {filepath.Join(support, "hires", "gm1", "claude"), false},
		{filepath.Join(sandbox.Workspace(support, "gm1"), "node_modules"), false},
		{filepath.Join(support, "hires", "gm1", "x", "work"), false}, {filepath.Join(support, "work"), false},
		{filepath.Join(support, "hires", "..", "work"), false}, {filepath.Join(base, "elsewhere", "hires", "gm1", "work"), false},
	} {
		if got := gmWorkspaceSkipped([]string{support, other, ""}, c.folder); got != c.want {
			t.Errorf("%s: %v, want %v", c.folder, got, c.want)
		}
	}
}
