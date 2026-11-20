package desktop

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXHIREPROJECTS, the engine's side, with the stand-in start-only runner of desksandbox_test.go: every desk in its
// sandbox gets the folder for hires Setup has (Places.Projects) but the GM with a workspace, whose own row holds it,
// and is told plainly, in its start words, that only its own worktree there is its to change.

func TestADeskIsHeldToItsOwnWorktreeInTheProjectsFolderAndToldSo(t *testing.T) {
	for _, c := range []struct {
		name      string
		worktrees bool // Setup's folder for worktrees, outside the projects folder
		want      string
	}{
		{"its worktree in the projects folder", false, "Your projects folder is %s. You can change files only in your own worktree here; other projects are read-only."},
		{"its worktree elsewhere", true, "Your projects folder, %s, is read-only to you, apart from your repository's git data."},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newSandboxRig(t)
			if !c.worktrees {
				r.o.Worktrees = ""
			}
			projects := filepath.Dir(r.repo)
			setHireFolder(t, projects)
			var started string
			request := r.request("janitor")
			request.StartTyped = func(message string) { started = message }
			hire, err := runManagedHire(request, r.o)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.desks[0].places; got.Projects != physical(projects) || got.Root != physical(hire.Worktree) {
				t.Errorf("its places: Projects %q, Root %q", got.Projects, got.Root)
			}
			if want := strings.Replace(c.want, "%s", physical(projects), 1); !strings.HasSuffix(started, "\n\n"+want) {
				t.Errorf("its start words don't end %q:\n%s", want, started)
			}
			// Made again from its row (a stop's listing, a restart): the same.
			again, err := deskPlaces(deskAtOf(registryRow(t, hire.ID), deskSocketPath(hire.ID, 0), nil, r.o.withDefaults()))
			if err != nil || again.Projects != physical(projects) {
				t.Errorf("made again from its row: %q %v", again.Projects, err)
			}
		})
	}
}

// The GM with a workspace has its own row and words (FXGMROOT); with no folder for hires set, nothing changes.
func TestNoProjectsWordsForTheGMWithAWorkspaceNorWithNoFolderForHires(t *testing.T) {
	r := newSandboxRig(t)
	projects := gmProjectsFolder(t)
	setHireFolder(t, projects)
	request := r.request(gmRole)
	request.Folder = projects
	var started string
	request.StartTyped = func(message string) { started = message }
	if _, err := runManagedHire(request, r.o); err != nil {
		t.Fatal(err)
	}
	if got := r.desks[0].places; got.Projects != "" || got.GMProjects != physical(projects) {
		t.Errorf("the GM's places: Projects %q, GMProjects %q", got.Projects, got.GMProjects)
	}
	if strings.Contains(started, "other projects are read-only") {
		t.Errorf("the GM with a workspace is told a hire's words:\n%s", started)
	}

	setHireFolder(t, "")
	started = ""
	hireRequest := r.request("janitor")
	hireRequest.StartTyped = func(message string) { started = message }
	if _, err := runManagedHire(hireRequest, r.o); err != nil {
		t.Fatal(err)
	}
	if got := r.desks[1].places; got.Projects != "" {
		t.Errorf("with no folder for hires: Projects %q", got.Projects)
	}
	if strings.Contains(started, "projects folder") {
		t.Errorf("with no folder for hires, a hire is told of one:\n%s", started)
	}
}

// A desk whose place is the projects folder, or holds it, is told nothing of it: on a Mac it doesn't start
// (wholeProjectsWords, FXHIREPROJECTSR4), and elsewhere its place is all of it.
func TestADeskWhosePlaceIsTheProjectsFolderIsToldNothingOfIt(t *testing.T) {
	isolate(t)
	projects := physical(t.TempDir())
	setHireFolder(t, projects)
	for root, want := range map[string]string{
		projects:                                "",
		filepath.Dir(projects):                  "",
		filepath.Join(projects, "app"):          "the folder you work in here; other projects are read-only.",
		filepath.Join(projects, "app-hires/h1"): "your own worktree here; other projects are read-only.",
	} {
		got := deskProjectsWords(root, strings.HasSuffix(root, "h1"))
		if want == "" && got != "" || want != "" && !strings.HasSuffix(got, want) {
			t.Errorf("at %s: %q, want …%q", root, got, want)
		}
	}
}

// FXHIREPROJECTSR4: the office's picker, as the core says it and the page shows it. It lists the projects in the
// folder for hires by name (a folder of its own), never a hidden folder, a file, the folder a repo's hires' worktrees
// go in beside it, or a link, wherever it leads (FXHIREPROJECTSR5, O3); with none there, it says so and what to do.
// Every word is the core's.
func TestThePickerAsksWhichProjectAndSaysWhatToDo(t *testing.T) {
	isolate(t)
	homeDir = physical(homeDir)
	projects := filepath.Join(homeDir, "Projects")
	elsewhere := filepath.Join(homeDir, "Code", "site")
	for _, folder := range []string{projects + "/app", projects + "/app-hires/h1", projects + "/lib", projects + "/.smolcorp", projects + "/orphan-hires", elsewhere} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(projects+"/notes.md", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for link, to := range map[string]string{projects + "/site": elsewhere, projects + "/gone": homeDir + "/nowhere"} {
		if err := os.Symlink(to, link); err != nil {
			t.Fatal(err)
		}
	}
	want := `{"pick":{"title":"Which project should this hire work on?","folder":"~/Projects","projects":["app","lib","orphan-hires"],` +
		`"note":"On this Mac, each hire works in one project of its own. These are the projects in ~/Projects.","cancel":"Cancel"}}`
	if got := pickFor(projects); got != want {
		t.Errorf("the picker:\n got %s\nwant %s", got, want)
	}
	empty := filepath.Join(homeDir, "Empty")
	if err := os.MkdirAll(empty+"/.hidden", 0o700); err != nil {
		t.Fatal(err)
	}
	want = `{"pick":{"title":"Which project should this hire work on?","folder":"~/Empty","projects":[],"note":"",` +
		`"none":"There's no project in your projects folder, ~/Empty, yet. Put one there (a folder of its own, or a git clone), then hire again.","cancel":"Cancel"}}`
	if got := pickFor(empty); got != want {
		t.Errorf("the picker with no project:\n got %s\nwant %s", got, want)
	}
	for name, want := range map[string]string{"lib": projects + "/lib", "site": "", "app-hires": "", ".smolcorp": "", "notes.md": "", "..": "", "lib/..": "", "": ""} {
		got, why := pickedProject(projects, name)
		if got != want || (want == "") != (why != "") {
			t.Errorf("picked %q: %q %q, want %q", name, got, why, want)
		}
	}
	if _, why := pickedProject(projects, "gone"); why != "There's no project called gone in your projects folder, ~/Projects, now. Hire again to pick another." {
		t.Errorf("a project gone: %q", why)
	}
	// The page shows the core's words alone, and sends the hire again with the name picked; Cancel sends nothing.
	page, err := os.ReadFile(filepath.Join("..", "Resources", "web", "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`send('/act', params, undefined, (text) => pickProject(params, text));`, `send('/act', { ...params, project: name });`,
		`$('pick-title').textContent = pick.title || '';`, `const note = pick.projects.length ? pick.note : pick.none;`,
		`$('pick-cancel').addEventListener('click', () => { tick(); $('pick').hidden = true; });`} {
		if !strings.Contains(string(page), part) {
			t.Errorf("deck.html has no %s", part)
		}
	}
}

// The words a desk at the projects folder, or a folder that holds it or home, gets at its hire and at a start; none
// for a project's own folder or a worktree.
func TestADeskAtTheProjectsFolderIsToldToHireIntoAProject(t *testing.T) {
	isolate(t)
	homeDir = physical(homeDir)
	projects := filepath.Join(homeDir, "Projects")
	if err := os.MkdirAll(projects+"/app", 0o700); err != nil {
		t.Fatal(err)
	}
	setHireFolder(t, projects)
	for root, where := range map[string]string{
		projects:                           "your whole projects folder, " + projects,
		filepath.Join(homeDir, "projects"): "your whole projects folder, " + projects,
		homeDir:                            homeDir + ", which holds your projects folder, " + projects,
		filepath.Dir(homeDir):              filepath.Dir(homeDir) + ", which holds your projects folder, " + projects,
		projects + "/app":                  "",
		projects + "/app-hires/h1":         "",
		filepath.Join(homeDir, "Code"):     "",
	} {
		hire, start := wholeProjectsWords(root, false), wholeProjectsWords(root, true)
		if where == "" {
			if hire != "" || start != "" {
				t.Errorf("at %s: %q, %q", root, hire, start)
			}
			continue
		}
		if want := projectsWholeAtHire(where); hire != want {
			t.Errorf("at %s, at its hire: %q, want %q", root, hire, want)
		}
		if want := projectsWholeAtStart(where); start != want {
			t.Errorf("at %s, at its start: %q, want %q", root, start, want)
		}
	}
	// With no folder for hires: the home folder, and a folder that holds it.
	setHireFolder(t, "")
	if got := wholeProjectsWords(homeDir, false); got != projectsWholeAtHire("your whole home folder") {
		t.Errorf("at home, with no folder for hires: %q", got)
	}
	if got := wholeProjectsWords(projects, false); got != "" {
		t.Errorf("at ~/Projects, with no folder for hires: %q", got)
	}
}

// standInInfo is a file's mode and owner, as Lstat gives them, for a folder or link no test can make (another user's).
type standInInfo struct {
	mode fs.FileMode
	uid  int
}

func (i standInInfo) Name() string       { return "x" }
func (i standInInfo) Size() int64        { return 0 }
func (i standInInfo) Mode() fs.FileMode  { return i.mode }
func (i standInInfo) ModTime() time.Time { return time.Time{} }
func (i standInInfo) IsDir() bool        { return i.mode.IsDir() }
func (i standInInfo) Sys() any           { return &syscall.Stat_t{Uid: uint32(i.uid)} }

// O7a (RT126FIXREVIEW-HIREPROJECTSR3): whether a desk can change a link on the way to the folder for hires is its
// folder's to say. A root-owned link in a folder of the owner's (sudo ln -s ~/Projects) is held; one in a folder only
// another user writes in (/var in /), or another user's in a sticky folder of theirs, isn't.
func TestALinkOnTheWayToTheProjectsFolderIsHeldUnlessItsFolderKeepsDesksOut(t *testing.T) {
	me, root := os.Getuid(), 0
	if me == root {
		t.Skip("run as root: every folder is the runner's")
	}
	link := func(uid int) standInInfo { return standInInfo{fs.ModeSymlink | 0o755, uid} }
	for name, c := range map[string]struct {
		folder, link standInInfo
		fixed        bool
	}{
		"a root-owned link in the owner's folder":     {standInInfo{fs.ModeDir | 0o755, me}, link(root), false},
		"the owner's link in the owner's folder":      {standInInfo{fs.ModeDir | 0o700, me}, link(me), false},
		"a root-owned link in root's folder":          {standInInfo{fs.ModeDir | 0o755, root}, link(root), true},
		"a link in root's folder anyone writes in":    {standInInfo{fs.ModeDir | 0o777, root}, link(root), false},
		"a link in root's folder its group writes in": {standInInfo{fs.ModeDir | 0o775, root}, link(root), false},
		"the owner's link in root's sticky folder":    {standInInfo{fs.ModeDir | fs.ModeSticky | 0o777, root}, link(me), false},
		"a root-owned link in root's sticky folder":   {standInInfo{fs.ModeDir | fs.ModeSticky | 0o777, root}, link(root), true},
	} {
		if got := linkFixedInFolder(c.folder, c.link); got != c.fixed {
			t.Errorf("%s: %v, want %v", name, got, c.fixed)
		}
	}
	// The real thing: /var, a link in /.
	top, errTop := os.Lstat("/")
	v, errVar := os.Lstat("/var")
	if errTop != nil || errVar != nil || v.Mode()&fs.ModeSymlink == 0 {
		t.Skipf("no /var link here: %v %v", errTop, errVar)
	}
	if !linkFixedInFolder(top, v) {
		t.Error("/var, in /, is held as a link a desk could change")
	}
}

// O7b (RT126FIXREVIEW-HIREPROJECTSR3): a repo with more loose objects than the start looks at is said as its own git
// objects, with what packs them; within the budget, a repo of its own starts.
func TestARepoWithTooManyLooseObjectsIsToldToPackThem(t *testing.T) {
	repo := t.TempDir()
	for i := 0; i < 8; i++ {
		folder := filepath.Join(repo, ".git", "objects", "a"+string(rune('0'+i)))
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "object"), []byte("x"), 0o400); err != nil {
			t.Fatal(err)
		}
	}
	most := linkCheckMost
	t.Cleanup(func() { linkCheckMost = most })
	linkCheckMost = 5
	err := checkSharedObjects(filepath.Join(repo, ".git"))
	want := repo + "'s git objects are more than 5 files, more than Smolcorp looks at before a desk starts, so the desk didn't start. Run git repack -a -d && git prune in " + repo + " to pack them, then start it again."
	if err == nil || err.Error() != want {
		t.Errorf("too many loose objects: %v, want %q", err, want)
	}
	linkCheckMost = 100
	if err := checkSharedObjects(filepath.Join(repo, ".git")); err != nil {
		t.Errorf("within the budget: %v", err)
	}
}
