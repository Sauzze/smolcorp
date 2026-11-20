//go:build darwin

package desktop

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGMHIRENOGIT126 (round 1, the narrowed rule): a hire into a General Manager's own places (its workspace, its
// plans, the worktree or checkout it works in) runs no git there, whoever asks for it, nor in the repo git would find
// from there; everywhere else, the owner's repos included, the full flow. The GM in these rows is a row in the test's
// own registry. FIFOs and links are the test's own, in its temp folders.

// gmOfTheRig registers an open General Manager hired into projects, a folder of projects: it works in a workspace of
// its own and reads projects, never writes it.
func gmOfTheRig(t *testing.T, projects string) Hire {
	t.Helper()
	keepRegistry(t)
	if err := os.MkdirAll(projects, 0o700); err != nil {
		t.Fatal(err)
	}
	gm := Hire{ID: "gm0123456789", Name: "General Manager", Role: gmRole, Folder: keptPath(physical(projects)), State: "working"}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, gm) }); err != nil {
		t.Fatal(err)
	}
	return gm
}

// gitCalls puts a stand-in git first on PATH that notes each call (the folder it runs in, resolved, and its words)
// and runs the real one. The returned func lists the calls that ran in, or named, any of folders.
func gitCalls(t *testing.T) func(folders ...string) []string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	bin := t.TempDir()
	notes := filepath.Join(bin, "calls")
	script := "#!/bin/sh\nprintf '%s\\t%s\\n' \"$(pwd -P)\" \"$*\" >> '" + notes + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func(folders ...string) []string {
		data, _ := os.ReadFile(notes)
		var named []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line != "" && slices.ContainsFunc(folders, func(f string) bool {
				return strings.Contains(line, f) || strings.Contains(line, physical(f))
			}) {
				named = append(named, line)
			}
		}
		return named
	}
}

// heldFifo is a FIFO the test put where a file was (or where none was): opened says whether a reader ever opened it,
// which the test then holds by keeping a write end open without writing, as a GM would.
type heldFifo struct {
	path   string
	was    []byte
	mu     sync.Mutex
	opened bool
	writer *os.File
	stop   chan struct{}
	done   chan struct{}
}

func plantFifo(t *testing.T, path string) *heldFifo {
	t.Helper()
	f := &heldFifo{path: path, stop: make(chan struct{}), done: make(chan struct{})}
	if data, err := os.ReadFile(path); err == nil {
		f.was = data
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(f.done)
		for {
			select {
			case <-f.stop:
				return
			default:
			}
			if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				f.mu.Lock()
				f.opened, f.writer = true, w // held open, never written: the reader waits on its read
				f.mu.Unlock()
				<-f.stop
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	t.Cleanup(f.release)
	return f
}

func (f *heldFifo) wasOpened() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opened
}

// release lets every reader go (end of file), then puts back what was there.
func (f *heldFifo) release() {
	select {
	case <-f.stop:
		return
	default:
	}
	close(f.stop)
	<-f.done
	f.mu.Lock()
	if f.writer != nil {
		f.writer.Close()
	}
	f.mu.Unlock()
	for i := 0; i < 100; i++ {
		if w, err := os.OpenFile(f.path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
		time.Sleep(5 * time.Millisecond)
	}
	if info, err := os.Lstat(f.path); err == nil && info.Mode()&fs.ModeNamedPipe != 0 {
		_ = os.Remove(f.path)
		if f.was != nil {
			_ = os.WriteFile(f.path, f.was, 0o644)
		}
	}
}

// hireWithin is prepareHire for folder (a worktree asked for), given at most within: past that the row fails, and
// the FIFOs are let go so the hire comes back.
func hireWithin(t *testing.T, within time.Duration, r hireRequest, o hirePlaceOptions, fifos ...*heldFifo) (hirePlace, error, time.Duration) {
	t.Helper()
	type answer struct {
		place hirePlace
		err   error
	}
	done := make(chan answer, 1)
	start := time.Now()
	go func() {
		place, err := prepareHire(r, o)
		done <- answer{place, err}
	}()
	select {
	case a := <-done:
		return a.place, a.err, time.Since(start)
	case <-time.After(within):
		t.Errorf("the hire was still being placed after %v", within)
		for _, f := range fifos {
			f.release()
		}
		a := <-done
		return a.place, a.err, time.Since(start)
	}
}

// inPlace says what a hire placed in a GM's folder got wrong: it must work in folder itself, with no repo, worktree
// or branch, and be told why.
func inPlace(t *testing.T, place hirePlace, err error, folder string) {
	t.Helper()
	if err != nil {
		t.Fatalf("the hire was refused: %v", err)
	}
	if place.Folder != folder || place.Worktree != "" || place.Repo != "" || place.Branch != "" || place.Note != gmFolderLine {
		t.Errorf("its place: %+v, want %s itself with the GM folder's line", place, folder)
	}
}

// gmWorkingIn registers an open General Manager in a repo, working in place (its worktree): place is its own.
func gmWorkingIn(t *testing.T, place string) Hire {
	t.Helper()
	keepRegistry(t)
	gm := Hire{ID: "gm0123456789", Name: "General Manager", Role: gmRole, Folder: keptPath(physical(place)), Repo: physical(place),
		Worktree: keptPath(physical(place)), State: "working"}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, gm) }); err != nil {
		t.Fatal(err)
	}
	return gm
}

// gmPlansOf is a GM's plans folder in the folder of projects it was hired into.
func gmPlansOf(h Hire) string {
	return filepath.Join(h.Folder.storedOnly(), filepath.FromSlash(deskPlansFor(h.ID)))
}

// gmPolicyOf is GM h's Policy, made from its row as its stop makes it (rowDeskSandbox), with worktrees the configured
// folder for hires' worktrees.
func gmPolicyOf(t *testing.T, h Hire, worktrees string) *sandbox.Policy {
	t.Helper()
	pl, err := deskPlaces(deskAtOf(h, deskSocketPath(h.ID, testCopyPort()), nil, engineOptions{Git: runGit, Worktrees: worktrees}))
	if err != nil {
		t.Fatalf("the GM's places: %v", err)
	}
	pol, err := sandbox.New(pl)
	if err != nil {
		t.Fatalf("the GM's Policy: %v", err)
	}
	return pol
}

// The round-2 to round-4 reviewers' probes (B4's FIFO at presetUpstream, B5's FIFO and undo, B6's link in the folder
// for its hires, the round-2 swap of x/.git), each in a GM's own place (the rig's folder is the worktree a GM works
// in), by prepareHire, which every route takes: the hire works in the folder as it is, no git process runs in or
// names the rig's folders, and nothing is written into or removed from Q.
func TestAHireIntoAGMsOwnFolderRunsNoGitThere(t *testing.T) {
	cases := []struct {
		name string
		rig  func(t *testing.T) (r *ownerRig, x string, fifos []*heldFifo)
	}{
		{"B4: a FIFO at x-git/config, where presetUpstream writes", func(t *testing.T) (*ownerRig, string, []*heldFifo) {
			r, x, xGit, _ := r3SeparateClone(t)
			return r, x, []*heldFifo{plantFifo(t, filepath.Join(xGit, "config"))}
		}},
		{"B5: a FIFO at x-git/modules/mod/commondir and HEAD, where the submodule step reads", func(t *testing.T) (*ownerRig, string, []*heldFifo) {
			r, x, xGit, _ := r4WithSubmodule(t)
			return r, x, []*heldFifo{plantFifo(t, filepath.Join(xGit, "modules", "mod", "commondir")), plantFifo(t, filepath.Join(xGit, "modules", "mod", "HEAD"))}
		}},
		{"B6: the hire's tree in the folder for its hires is already a link to Q's work tree", func(t *testing.T) (*ownerRig, string, []*heldFifo) {
			r, x, _, _ := r4WithSubmodule(t)
			tree := filepath.Join(r.base, "hires", "x", hireWorktreeName("Ada", "abcdef123456"))
			if err := os.MkdirAll(filepath.Dir(tree), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(r.repo, tree); err != nil {
				t.Fatal(err)
			}
			return r, x, nil
		}},
		{"round 2: x/.git a link to Q's .git", func(t *testing.T) (*ownerRig, string, []*heldFifo) {
			r := newOwnerRig(t)
			x := filepath.Join(r.base, "x")
			if err := os.MkdirAll(x, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(r.repo, ".git"), filepath.Join(x, ".git")); err != nil {
				t.Fatal(err)
			}
			return r, x, nil
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolate(t) // a registry of its own: the GM below is the only one
			r, x, fifos := c.rig(t)
			qGit := filepath.Join(r.repo, ".git")
			before, qConfig := r2Registrations(qGit), readOr(filepath.Join(qGit, "config"))
			qMod := readOr(filepath.Join(r.repo, "mod", ".git"))
			gmWorkingIn(t, r.base)
			calls := gitCalls(t)
			place, err, took := hireWithin(t, 20*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true},
				hirePlaceOptions{Worktrees: filepath.Join(r.base, "hires"), PortFree: func(int) bool { return true }}, fifos...)
			t.Logf("placed in %v: %+v, %v", took.Round(time.Millisecond), place, err)
			inPlace(t, place, err, x)
			if got := calls(r.base); len(got) > 0 {
				t.Errorf("git ran in, or named, the rig's folders: %q", got)
			}
			for _, f := range fifos {
				if f.wasOpened() {
					t.Errorf("something opened %s", f.path)
				}
			}
			r3Landed(t, qGit, before)
			if now := readOr(filepath.Join(qGit, "config")); now != qConfig {
				t.Errorf("Q's config changed:\n%s\nwas\n%s", now, qConfig)
			}
			if now := readOr(filepath.Join(r.repo, "mod", ".git")); now != qMod {
				t.Errorf("Q's submodule .git changed: %q, was %q", now, qMod)
			}
			if qMod != "" {
				if _, err := os.Stat(filepath.Join(r.repo, "mod", "s.txt")); err != nil {
					t.Errorf("Q's submodule lost its checked-out file: %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(r.base, "x-hires")); err == nil {
				t.Error("a folder for x's hires was made beside it")
			}
		})
	}
}

// What the rule judges: a GM's own places and everything below them, and a folder whose way up to the top git finds
// passes through one; never the projects it reads, nor a folder it may write outside them (O1: the owner's own repos,
// with a GM not held to one project open, keep the full flow). With no GM, or the GM fired, nothing is.
func TestTheRuleJudgesOnlyAGMsOwnPlaces(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	keepRegistry(t)
	base := physical(t.TempDir())
	projects, code := filepath.Join(base, "Projects"), filepath.Join(base, "code")
	app, lib := filepath.Join(code, "app"), filepath.Join(projects, "lib")
	if gmOwnsFolder(app) {
		t.Fatal("with no GM, a folder counts as a GM's")
	}
	gm := gmOfTheRig(t, projects)
	plans := gmPlansOf(gm)
	workspace := sandbox.Workspace(physical(supportDir), gm.ID)
	ownRepo := filepath.Join(plans, "lib2")
	for _, folder := range []string{app, lib, filepath.Join(lib, ".claude"), workspace, filepath.Join(ownRepo, ".claude", "x"), filepath.Join(ownRepo, "src")} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{app, lib, ownRepo} {
		if err := os.Mkdir(filepath.Join(repo, ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		folder string
		owned  bool
	}{
		{app, false},                           // an owner's repo outside the projects folder, which the GM may write
		{lib, false},                           // an owner's repo in its projects, which it reads
		{filepath.Join(lib, ".claude"), false}, // a held-entry folder of one
		{projects, false},
		{workspace, true},
		{plans, true},
		{ownRepo, true},
		{filepath.Join(ownRepo, ".claude"), true}, // B1: a held-entry folder of the GM's own repo
		{filepath.Join(ownRepo, ".claude", "x"), true},
		{filepath.Join(ownRepo, "src"), true},
	} {
		if got := gmOwnsFolder(c.folder); got != c.owned {
			t.Errorf("gmOwnsFolder(%s) = %v, want %v", c.folder, got, c.owned)
		}
	}
	// A GM whose places can't be spelled from its row: every folder counts as its.
	if err := updateRegistry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "../x", Role: gmRole, Folder: keptPath(projects), State: "working"})
	}); err != nil {
		t.Fatal(err)
	}
	if !gmOwnsFolder(app) {
		t.Error("a GM whose places can't be known doesn't fail closed")
	}
	// Fired, it owns nothing any more.
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			f.hires[i].State = "fired"
		}
	}); err != nil {
		t.Fatal(err)
	}
	if gmOwnsFolder(ownRepo) {
		t.Error("a fired GM's folder still counts")
	}
	place, err := r2HireInto(t, func() string { r := newOwnerRig(t); return r.repo }(), "", nil)
	if err != nil || place.Worktree == "" || place.Note != "" {
		t.Errorf("a hire with only a fired GM: %+v, %v; want the full flow", place, err)
	}
}

// A hire placed in a GM's own folder is told so, and its start's Places have no repo: its Policy holds the .git at its
// place as any git folder not its own, whether its row says so or, its line lost, the rule says so again at its start
// (O2), with no git run.
func TestAHireInAGMsFolderIsToldAndItsSandboxHoldsTheGitThere(t *testing.T) {
	message, _, err := engineBrief(engineRequest{Role: Role{ID: "janitor", Name: "Janitor", Agent: "grok"}}, hirePlace{Folder: "/x", Note: gmFolderLine}, "abcdef123456", time.Now())
	if err != nil || !strings.Contains(message, gmFolderHireLine) {
		t.Errorf("its first message: %q, %v", message, err)
	}
	if message, _, _ := engineBrief(engineRequest{Role: Role{ID: "janitor", Name: "Janitor", Agent: "grok"}}, hirePlace{Folder: "/x", Note: notARepo}, "abcdef123456", time.Now()); strings.Contains(message, gmFolderHireLine) {
		t.Errorf("a hire in no repo is told it's a GM's folder: %q", message)
	}
	shortSupport(t)
	deskSockets(t)
	r := newOwnerRig(t)
	keepRegistry(t)
	at := func(notes string) sandbox.Places {
		t.Helper()
		pl, err := deskPlaces(deskAtOf(Hire{ID: "abcdef123456", Role: "janitor", Folder: keptPath(r.repo), Notes: notes}, deskSocketPath("abcdef123456", 0), nil, engineOptions{Git: runGit}))
		if err != nil {
			t.Fatal(err)
		}
		return pl
	}
	if pl := at(""); pl.GitDir == "" {
		t.Fatal("the control: no git places for a repo")
	}
	for _, c := range []struct {
		name  string
		notes string
		gm    bool
	}{{"its row says so", "Hired. " + gmFolderLine, false}, {"its line lost: the rule says so again", "", true}} {
		t.Run(c.name, func(t *testing.T) {
			if c.gm {
				gmWorkingIn(t, r.base)
			}
			calls := gitCalls(t)
			pl := at(c.notes)
			if pl.GitDir != "" || pl.OwnWorktree != "" {
				t.Errorf("its places have a repo: %q", pl.GitDir)
			}
			if got := calls(r.base); len(got) > 0 {
				t.Errorf("git ran in its folder: %q", got)
			}
			pol, err := sandbox.New(pl)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{".git/config", ".git/hooks/pre-commit", ".git/HEAD"} {
				if v, _ := pol.Decide(filepath.Join(r.repo, file), sandbox.Write); v != sandbox.Deny {
					t.Errorf("its sandbox lets it write %s", file)
				}
			}
			// Its own files: with no GM, its own; in a GM's own place, held from it as that GM's folder is from every
			// other desk.
			if v, rule := pol.Decide(filepath.Join(r.repo, "a.txt"), sandbox.Write); !c.gm && v == sandbox.Deny || c.gm && v != sandbox.Deny {
				t.Errorf("its sandbox on the folder's own files: %v by %q", v, rule.Row)
			}
		})
	}
}

// D5: a full-flow hire's worktree keeps its git places at its start, even were its folder judged a GM's: it was given
// the full flow, and commits there.
func TestAFullFlowHiresWorktreeKeepsItsGitPlacesWhereverItIs(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	r := newOwnerRig(t)
	keepRegistry(t)
	place, err := r2HireInto(t, r.repo, "", nil)
	if err != nil || place.Worktree == "" {
		t.Fatalf("the hire: %+v, %v; want the full flow", place, err)
	}
	saved := gmOwnsFolder
	gmOwnsFolder = func(folder string) bool {
		return sandbox.Inside(runtime.GOOS, physical(folder), physical(hiresFolder(r.repo, "")))
	}
	t.Cleanup(func() { gmOwnsFolder = saved })
	at := deskAtOf(Hire{ID: "abcdef123456", Role: "janitor", Folder: keptPath(r.repo), Repo: r.repo, Worktree: keptPath(place.Worktree), Notes: place.Note},
		deskSocketPath("abcdef123456", 0), nil, engineOptions{Git: runGit})
	pl, err := deskPlaces(at)
	if err != nil || pl.GitDir == "" || pl.OwnWorktree == "" {
		t.Errorf("its start's places: git %q, own worktree %q, %v", pl.GitDir, pl.OwnWorktree, err)
	}
}

// B2: a GM held to one project, hired into the project itself (the demo's layout): its hire into the project keeps the
// full flow, and the project's folder for hires, where that hire's worktree is, is read only to the GM (its Policy
// denies writing there, and lets it read), as is a configured folder for hires' worktrees. Its own plans, in the
// project, are its own: a hire there is refused (by the scope's look, which takes only the project or the top of one of
// its worktrees, or else by the rule), with nothing made and no git of the hire's own.
func TestAHeldGMReadsItsHiresWorktreesAndWritesNone(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	hire := func(folder string) (int, string) {
		body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": folder})
		return scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
	}
	if code, text := hire(project); code != http.StatusOK {
		t.Fatalf("into the project: %d %q", code, text)
	}
	hires := registered(t)
	gm, tree := hires[0], hires[len(hires)-1].Worktree
	if tree.none() || !sandbox.Inside(runtime.GOOS, tree.storedOnly(), physical(hiresFolder(project, ""))) {
		t.Fatalf("the hire's worktree %q isn't in the project's folder for hires", tree)
	}
	if gmOwnsFolder(tree.storedOnly()) || gmOwnsFolder(project) {
		t.Error("the project or its hire's worktree counts as the GM's own")
	}
	worktrees := filepath.Join(filepath.Dir(project), "worktrees")
	pol := gmPolicyOf(t, gm, worktrees)
	for _, c := range []struct {
		at        string
		op        sandbox.Op
		want      sandbox.Verdict
		wantRowIs string
	}{
		{filepath.Join(tree.storedOnly(), "a.txt"), sandbox.Write, sandbox.Deny, "other desks' places"},
		{filepath.Join(tree.storedOnly(), "README"), sandbox.Read, sandbox.Allow, ""},
		{filepath.Join(physical(hiresFolder(project, "")), "new"), sandbox.Write, sandbox.Deny, "other desks' places"},
		{filepath.Join(physical(worktrees), "app", "w", "a.txt"), sandbox.Write, sandbox.Deny, "other desks' places"},
		{filepath.Join(gmPlansOf(gm), "plan.md"), sandbox.Write, sandbox.Allow, ""},
	} {
		v, rule := pol.Decide(c.at, c.op)
		if v != c.want || c.wantRowIs != "" && rule.Row != c.wantRowIs {
			t.Errorf("the GM's %v of %s: %v (%s), want %v (%s)", c.op, c.at, v, rule.Row, c.want, c.wantRowIs)
		}
	}
	plans := gmPlansOf(gm)
	if err := os.MkdirAll(plans, 0o700); err != nil {
		t.Fatal(err)
	}
	calls := gitCalls(t)
	spend(d)
	code, text := hire(plans)
	if code != http.StatusForbidden && (code != http.StatusConflict || !strings.Contains(text, "is a General Manager's own folder, so Smolcorp doesn't run git there")) {
		t.Errorf("into its plans: %d %q", code, text)
	}
	if got := registered(t); len(got) != len(hires) {
		t.Errorf("a hire was made: %+v", got[len(got)-1])
	}
	// The scope's own look at the folder asked for (gmScope.place, at the route, before the hire and outside d.hiring)
	// is the one git left there (N1): a read. The hire itself runs none.
	for _, call := range calls(plans) {
		if !strings.HasSuffix(call, "rev-parse --path-format=absolute --show-toplevel --git-common-dir") {
			t.Errorf("git ran in, or named, its plans: %q", call)
		}
	}
}

// B2, the reviewer's probe: a GM hired into the project itself, a hire into the project with the full flow, and the
// round's stand-in git doing what a GM that won the race would: after the submodule's checkout, the hire's tree is
// swapped for a link to the project, and the step fails, or doesn't. The project's own submodule is untouched either
// way, and the GM's Policy denies it the folder for hires.
func TestAHeldGMsTreeSwapNeverReachesTheProjectsSubmodule(t *testing.T) {
	for _, mode := range []string{"control", "standin-fail", "standin-ok"} {
		t.Run(mode, func(t *testing.T) {
			shortSupport(t)
			deskSockets(t)
			r, _, _, _ := r4WithSubmodule(t)
			gm := gmOfTheRig(t, r.repo)
			hires := physical(hiresFolder(r.repo, ""))
			tree := filepath.Join(hires, hireWorktreeName("Ada", "abcdef123456"))
			if v, rule := gmPolicyOf(t, gm, "").Decide(tree, sandbox.Write); v != sandbox.Deny {
				t.Errorf("the GM may write its hire's tree: %s", rule.Row)
			}
			qSub := filepath.Join(r.repo, "mod")
			was := readOr(filepath.Join(qSub, ".git"))
			sh := "mv '" + tree + "' '" + tree + ".real' && ln -s '" + r.repo + "' '" + tree + "'"
			switch mode {
			case "standin-fail":
				r4StandInGit(t, r.base, " checkout -q --detach ", "", sh, "1")
			case "standin-ok":
				r4StandInGit(t, r.base, " checkout -q --detach ", "", sh, "")
			}
			place, err := r2HireInto(t, r.repo, "", nil)
			t.Logf("prepareHire: worktree=%q note=%q err=%v", place.Worktree, place.Note, err)
			if err != nil || place.Worktree == "" {
				t.Fatalf("the hire into the project: %v; want the full flow", err)
			}
			if now := readOr(filepath.Join(qSub, ".git")); now != was {
				t.Errorf("the project's mod/.git: %q, was %q", now, was)
			}
			if _, err := os.Stat(filepath.Join(qSub, "s.txt")); err != nil {
				t.Errorf("the project's mod/s.txt: %v", err)
			}
			if mode == "control" && !r4CheckedOut(place) {
				t.Error("the control's submodule isn't checked out")
			}
		})
	}
}

// feedFifo puts a FIFO where file was and, at the first reader's open, runs swap, then writes payload to that reader
// and every later one until the test ends (or file is no longer a FIFO).
func feedFifo(t *testing.T, file string, swap func(), payload []byte) {
	t.Helper()
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(file, 0o644); err != nil {
		t.Fatal(err)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(stop); <-done })
	go func() {
		defer close(done)
		first := true
		for {
			select {
			case <-stop:
				return
			default:
			}
			if info, err := os.Lstat(file); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
				return
			}
			f, err := os.OpenFile(file, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				time.Sleep(2 * time.Millisecond)
				continue
			}
			if first {
				first = false
				swap()
			}
			_ = syscall.SetNonblock(int(f.Fd()), false)
			_, _ = f.Write(payload)
			f.Close()
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

// Round 4's B6, the tree side: the hire's tree, in a folder for hires its owner (here the test) can write, is swapped
// for a link to Q's work tree, or to a folder whose mod is a whole repo, while the submodule step's checkout waits on a
// FIFO (a loose object of x's own submodule git folder, read through the alternates the core writes), or after it,
// with the step failing. The step works through the root the carry kept: nothing is written or removed outside the
// tree, and the control is checked out and clean.
func TestAHiresSubmoduleStepNeverLeavesItsTree(t *testing.T) {
	for _, mode := range []string{"control", "standin-fail", "fifo-garbage", "fifo-good", "fifo-good-into-a-repo"} {
		t.Run(mode, func(t *testing.T) {
			r, x, xGit, _ := r4WithSubmodule(t)
			hires := filepath.Join(r.base, "hires")
			tree := filepath.Join(physical(r.base), "hires", "x", hireWorktreeName("Ada", "abcdef123456"))
			qSub := filepath.Join(r.repo, "mod")
			qDotGit := readOr(filepath.Join(qSub, ".git"))
			target, other := r.repo, filepath.Join(r.base, "other")
			if mode == "fifo-good-into-a-repo" {
				if err := os.MkdirAll(other, 0o755); err != nil {
					t.Fatal(err)
				}
				gitRepoCmd(t, other, "clone", "-q", filepath.Join(r.base, "subsource"), "mod")
				target = other
			}
			swap := func() {
				if err := os.Rename(tree, tree+".real"); err != nil {
					t.Errorf("swap: %v", err)
					return
				}
				if err := os.Symlink(target, tree); err != nil {
					t.Errorf("swap: %v", err)
				}
			}
			once := ""
			switch mode {
			case "standin-fail":
				sh := "mv '" + tree + "' '" + tree + ".real' && ln -s '" + r.repo + "' '" + tree + "'"
				once = r4StandInGit(t, r.base, " checkout -q --detach ", "", sh, "1")
			case "fifo-garbage", "fifo-good", "fifo-good-into-a-repo":
				blob := strings.TrimSpace(gitRepoCmd(t, filepath.Join(x, "mod"), "rev-parse", "HEAD:s.txt"))
				loose := filepath.Join(xGit, "modules", "mod", "objects", blob[:2], blob[2:])
				data, err := os.ReadFile(loose)
				if err != nil {
					t.Fatalf("the blob isn't loose in x's submodule git folder: %v", err)
				}
				payload := []byte("not an object")
				if mode != "fifo-garbage" {
					payload = data
				}
				feedFifo(t, loose, swap, payload)
			}
			place, err := r2HireInto(t, x, hires, nil)
			if once != "" {
				if _, statErr := os.Stat(once); statErr == nil {
					t.Fatal("the stand-in never swapped")
				}
			}
			t.Logf("prepareHire: worktree=%q err=%v", place.Worktree, err)
			if mode == "fifo-good-into-a-repo" {
				if _, err := os.Stat(filepath.Join(other, "mod", ".git", "HEAD")); err != nil {
					t.Errorf("other/mod's own .git folder is gone: %v", err)
				}
				if _, err := os.Stat(filepath.Join(other, "mod", "s.txt")); err != nil {
					t.Errorf("other/mod's checked-out file is gone: %v", err)
				}
				return
			}
			if now := readOr(filepath.Join(qSub, ".git")); now != qDotGit {
				t.Errorf("Q's submodule .git file was removed or rewritten: %q, was %q", now, qDotGit)
			}
			if _, err := os.Stat(filepath.Join(qSub, "s.txt")); err != nil {
				t.Errorf("Q's submodule's checked-out file: %v", err)
			}
			if mode == "control" {
				if !r4CheckedOut(place) {
					t.Fatal("the control's submodule isn't checked out")
				}
				if out := gitRepoCmd(t, filepath.Join(place.Worktree, "mod"), "status", "--porcelain"); strings.TrimSpace(out) != "" {
					t.Errorf("the control's submodule isn't clean: %q", out)
				}
				if out := gitRepoCmd(t, filepath.Join(place.Worktree, "mod"), "diff-files", "--name-only"); strings.TrimSpace(out) != "" {
					t.Errorf("the control's submodule's files aren't as its index has them: %q", out)
				}
			}
		})
	}
}

// gmFolderDeck is the office's deck with a GM in the registry, hired into the folder for hires (~/Projects, which it
// reads), and lib2, a repo it made in its own plans folder: FIFOs where git reads its config and HEAD, and where a
// hire's settings are read.
func gmFolderDeck(t *testing.T) (*Deck, gmRootLayout, string, []*heldFifo) {
	t.Helper()
	d, l, _ := officeDeck(t)
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, l.gm) }); err != nil {
		t.Fatal(err)
	}
	lib2 := filepath.Join(gmPlansOf(l.gm), "lib2")
	if err := os.MkdirAll(filepath.Join(lib2, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, lib2, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(lib2, "README"), "the GM's\n")
	writeFile(t, filepath.Join(lib2, "src", "a.txt"), "a\n")
	standInOwnersGit(t, lib2, "add", "README", "src/a.txt")
	standInOwnersGit(t, lib2, "commit", "-q", "-m", "first")
	return d, l, lib2, []*heldFifo{plantFifo(t, filepath.Join(lib2, ".git", "config")), plantFifo(t, filepath.Join(lib2, ".git", "HEAD")),
		plantFifo(t, filepath.Join(lib2, ".claude", "settings.json")), plantFifo(t, filepath.Join(lib2, "settings.json"))}
}

// gmRouteHire is the GM's own hire, by its route with its token, into folder: its answer when it comes.
func gmRouteHire(d *Deck, folder string) chan routeAnswer {
	done := make(chan routeAnswer, 1)
	start := time.Now()
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/setup/gm/hire", strings.NewReader(`{"role":"janitor","folder":`+jsonText(folder)+`}`))
		d.gmHire(w, &request{Request: req, caller: callerGM})
		done <- routeAnswer{w.Code, w.Body.String(), time.Since(start)}
	}()
	return done
}

type routeAnswer struct {
	code int
	body string
	took time.Duration
}

// B1 and B7: the GM's hire, by its route with its token, into its own repo (lib2, in its plans folder), spelled as
// the repo, as a held-entry folder of it (.claude, which a GM's Policy holds by name while it writes inside), as a
// plain folder of it, and into its plans folder made a repo itself: each comes back at once, placed in the folder and
// told why; no git names its plans and no FIFO there is opened (a plans folder that holds lib2 is refused as holding
// a repo, so the last spelling is a folder of it, whose top is the plans); then the owner's office hire and Setup's hire go
// ahead, each into a repo of the owner's with the full flow, as do the GM's into an owner's repo and its .claude.
func TestAGMsHireIntoItsOwnRepoRunsNoGitAndHoldsNoOtherHire(t *testing.T) {
	for _, spelling := range []string{"lib2", "lib2/.claude", "lib2/src", "notes, in its plans, a repo"} {
		t.Run(spelling, func(t *testing.T) {
			d, l, lib2, fifos := gmFolderDeck(t)
			plans := gmPlansOf(l.gm)
			folder := filepath.Join(filepath.Dir(lib2), filepath.FromSlash(spelling))
			if strings.HasPrefix(spelling, "notes") {
				standInOwnersGit(t, plans, "init", "-q", "-b", "main")
				folder = filepath.Join(plans, "notes")
				if err := os.Mkdir(folder, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			calls := gitCalls(t)
			var a routeAnswer
			select {
			case a = <-gmRouteHire(d, folder):
			case <-time.After(5 * time.Second):
				t.Errorf("the GM's hire into its own folder was still going after 5s (opened: %q)", fifosOpened(fifos))
				spend(d)
				s1, w1 := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"app"}})
				t.Logf("meanwhile, the office's hire into app: %d %q", s1, w1)
				for _, f := range fifos {
					f.release()
				}
				t.FailNow()
			}
			t.Logf("the GM's hire into %s: %d %s in %v", folder, a.code, a.body, a.took.Round(time.Millisecond))
			var said map[string]any
			_ = json.Unmarshal([]byte(a.body), &said)
			if a.code != http.StatusOK || said["note"] != gmFolderLine || said["worksIn"] != folder || said["branch"] != nil {
				t.Errorf("the GM's hire: %d %s", a.code, a.body)
			}
			d.watching.Wait()
			if got := registered(t); got[len(got)-1].Folder.storedOnly() != folder || !got[len(got)-1].Worktree.none() || !placedInGMFolder(got[len(got)-1].Notes) {
				t.Errorf("its row: %+v", got[len(got)-1])
			}
			if got := fifosOpened(fifos); len(got) > 0 {
				t.Errorf("something opened %q", got)
			}
			if got := calls(plans); len(got) > 0 {
				t.Errorf("git ran in, or named, the GM's plans: %q", got)
			}
			if _, err := os.Stat(hiresFolder(lib2, "")); err == nil {
				t.Error("a folder for lib2's hires was made")
			}
			// The owner's office hire and Setup's hire, into the owner's own repos: the full flow, refused by nothing.
			spend(d)
			pickedHire(t, d, "app")
			spend(d)
			if status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: l.lib, Answers: []string{}, Worktree: true}); status != http.StatusOK || answer == nil || answer.(map[string]any)["note"] != nil {
				t.Errorf("Setup's hire into lib: %d %v %q", status, answer, why)
			}
			d.watching.Wait()
			if got := registered(t); got[len(got)-1].Folder.storedOnly() != l.lib || filepath.Dir(got[len(got)-1].Worktree.storedOnly()) != hiresFolder(l.lib, "") {
				t.Errorf("Setup's hire into lib: %+v", got[len(got)-1])
			}
		})
	}
	t.Run("the control: an owner's repo's .claude, outside every GM's own places", func(t *testing.T) {
		d, l, _, _ := gmFolderDeck(t)
		claude := filepath.Join(l.app, ".claude")
		if err := os.MkdirAll(claude, 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(claude, "notes.md"), "x\n")
		standInOwnersGit(t, l.app, "add", ".claude/notes.md")
		standInOwnersGit(t, l.app, "commit", "-q", "-m", "notes")
		a := <-gmRouteHire(d, claude)
		var said map[string]any
		_ = json.Unmarshal([]byte(a.body), &said)
		if a.code != http.StatusOK || said["note"] != nil || said["branch"] == nil {
			t.Errorf("the GM's hire into app/.claude: %d %s", a.code, a.body)
		}
	})
}

// The owner's office hire and Setup's hire into a GM's own repo, FIFOs there: each runs no git there, comes back at
// once, works in the folder and says why, in the answer and on its row.
func TestTheOwnersHireIntoAGMsFolderRunsNoGitAndSaysSo(t *testing.T) {
	d, l, lib2, fifos := gmFolderDeck(t)
	calls := gitCalls(t)
	d.mu.Lock()
	d.config.HireFolder = filepath.Dir(lib2) // the folder for hires is the GM's plans: the office's hire picks lib2 there
	d.mu.Unlock()
	spend(d)
	var status int
	var said string
	took := guarded(t, fifos, func() { status, said = d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"lib2"}}) })
	t.Logf("the office's hire into lib2: %d %q in %v", status, said, took.Round(time.Millisecond))
	if status != http.StatusNoContent {
		t.Fatalf("the office's hire: %d %s", status, said)
	}
	d.watching.Wait()
	if got := registered(t); got[len(got)-1].Folder.storedOnly() != lib2 || !got[len(got)-1].Worktree.none() || !placedInGMFolder(got[len(got)-1].Notes) {
		t.Errorf("the office's hire's row: %+v", got[len(got)-1])
	}
	spend(d)
	var answer any
	var why string
	took = guarded(t, fifos, func() {
		status, answer, why = d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: lib2, Answers: []string{}, Worktree: true})
	})
	t.Logf("Setup's hire into lib2: %d %v %q in %v", status, answer, why, took.Round(time.Millisecond))
	if body, _ := answer.(map[string]any); status != http.StatusOK || body["note"] != gmFolderLine {
		t.Errorf("Setup's hire: %d %v %q", status, answer, why)
	}
	d.watching.Wait()
	if got := fifosOpened(fifos); len(got) > 0 {
		t.Errorf("something opened %q", got)
	}
	if got := calls(gmPlansOf(l.gm)); len(got) > 0 {
		t.Errorf("git ran in, or named, the GM's plans: %q", got)
	}
}

// O1's regression must not happen: with a GM not held to one project open (which may write the owner's folders outside
// its projects), the owner's hire into ~/code/app, outside every GM's own places, keeps the full flow, by Setup and by
// the GM's route alike.
func TestTheOwnersRepoOutsideTheGMsPlacesKeepsTheFullFlowWithAGMOpen(t *testing.T) {
	d, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	app := filepath.Join(homeDir, "code", "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, app, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(app, "README"), "the owner's\n")
	standInOwnersGit(t, app, "add", "README")
	standInOwnersGit(t, app, "commit", "-q", "-m", "first")
	if gmOwnsFolder(app) {
		t.Fatal("~/code/app counts as the GM's own")
	}
	spend(d)
	status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: app, Answers: []string{}, Worktree: true})
	if body, _ := answer.(map[string]any); status != http.StatusOK || body["note"] != nil {
		t.Errorf("Setup's hire into ~/code/app: %d %v %q", status, answer, why)
	}
	d.watching.Wait()
	if got := registered(t); got[len(got)-1].Folder.storedOnly() != app || filepath.Dir(got[len(got)-1].Worktree.storedOnly()) != hiresFolder(app, "") || got[len(got)-1].Branch == "" {
		t.Errorf("Setup's hire into ~/code/app: %+v", got[len(got)-1])
	}
	spend(d)
	a := <-gmRouteHire(d, app)
	var said map[string]any
	_ = json.Unmarshal([]byte(a.body), &said)
	if a.code != http.StatusOK || said["note"] != nil || said["branch"] == nil {
		t.Errorf("the GM's hire into ~/code/app: %d %s", a.code, a.body)
	}
}

// The known limit, in the release notes and the team's guide.
func TestTheGMFolderLimitIsInTheDocs(t *testing.T) {
	const line = "A General Manager can change files across your home folder outside a few protected places, so a folder it prepared on purpose could affect a hire there. Look over a hire's changes before you merge them. Inside a General Manager's own folders, Smolcorp doesn't run git, and a hire there can read the files but not change them."
	for _, doc := range []string{"../RELEASE-NOTES.md", "../docs/team.md"} {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(strings.Fields(string(data)), " "), line) {
			t.Errorf("%s doesn't say: %s", doc, line)
		}
	}
}

// readOr is a file's text, "" when it can't be read as a plain file.
func readOr(path string) string {
	data, _ := cpReadData(path, 1<<20)
	return string(data)
}

// The settings files apiKeyHelper reads, for every Claude hire, the owner's too and with no GM at all: a FIFO there,
// or a link to one, is no settings file and isn't waited on; a plain one, or a link to one, that sets apiKeyHelper
// still refuses the hire, and one too big to judge refuses it too.
func TestTheSettingsFilesAHireChecksAreNeverWaitedOn(t *testing.T) {
	isolate(t)
	r := newOwnerRig(t)
	keepRegistry(t)
	account := filepath.Join(r.base, "stand-in-claude")
	if err := os.MkdirAll(account, 0o700); err != nil {
		t.Fatal(err)
	}
	fifos := []*heldFifo{plantFifo(t, filepath.Join(r.repo, "settings.json")), plantFifo(t, filepath.Join(r.repo, ".claude", "settings.json")),
		plantFifo(t, filepath.Join(r.base, "elsewhere-fifo"))}
	if err := os.Symlink(filepath.Join(r.base, "elsewhere-fifo"), filepath.Join(r.repo, ".claude", "settings.local.json")); err != nil {
		t.Fatal(err)
	}
	_, err, took := hireWithin(t, 5*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: r.repo, Worktree: true, Claude: true, ConfigDir: account},
		hirePlaceOptions{Worktrees: filepath.Join(r.base, "hires"), PortFree: func(int) bool { return true }}, fifos...)
	t.Logf("came back in %v: %v", took.Round(time.Millisecond), err)
	for _, f := range fifos {
		if f.wasOpened() {
			t.Errorf("%s was opened", f.path)
		}
	}
	if err == nil || strings.Contains(err.Error(), "apiKeyHelper") {
		t.Errorf("the hire: %v, want the account's own refusal (no .claude.json here), not an apiKeyHelper", err)
	}
	// The controls.
	helper := filepath.Join(r.base, "helper.json")
	writeFile(t, helper, `{"apiKeyHelper": "echo x"}`)
	big := filepath.Join(r.base, "big.json")
	writeFile(t, big, "{"+strings.Repeat(" ", settingsMost)+"}")
	for _, c := range []struct {
		name, file, want string
		unread           bool
	}{
		{"a plain file", helper, "settings.json", false},
		{"a link to a plain file", "", "settings.json", false},
		{"a file too big to judge", big, "settings.json", true},
	} {
		place := t.TempDir()
		at := filepath.Join(place, c.want)
		if c.file == "" {
			if err := os.Symlink(helper, at); err != nil {
				t.Fatal(err)
			}
		} else if data, err := os.ReadFile(c.file); err != nil {
			t.Fatal(err)
		} else {
			writeFile(t, at, string(data))
		}
		got, unread := apiKeyHelper(place)
		if c.unread && unread != at || !c.unread && got != at {
			t.Errorf("%s: apiKeyHelper = %q, unread %q", c.name, got, unread)
		}
	}
}

// The brief's branch is read from git's own files without waiting: a HEAD that is a FIFO is no branch.
func TestTheBriefsBranchIsNeverWaitedOn(t *testing.T) {
	r := newOwnerRig(t)
	if got := briefBranch(r.repo); got != "main" {
		t.Fatalf("the control: %q, want main", got)
	}
	head := plantFifo(t, filepath.Join(r.repo, ".git", "HEAD"))
	done := make(chan string, 1)
	go func() { done <- briefBranch(r.repo) }()
	select {
	case got := <-done:
		if got != "" || head.wasOpened() {
			t.Errorf("a FIFO HEAD: %q, opened %v", got, head.wasOpened())
		}
	case <-time.After(5 * time.Second):
		t.Error("the brief's branch waited on a FIFO HEAD")
		head.release()
		<-done
	}
}

// fifosOpened lists the FIFOs something opened.
func fifosOpened(fifos []*heldFifo) []string {
	var paths []string
	for _, f := range fifos {
		if f.wasOpened() {
			paths = append(paths, f.path)
		}
	}
	return paths
}

// spend lets the next hire go at once: the cooldown spent, so only d.hiring could refuse it.
func spend(d *Deck) {
	d.mu.Lock()
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	d.mu.Unlock()
}

// guarded runs hire, given at most 10s: past that the row fails, and the FIFOs are let go so it comes back. How long
// it took.
func guarded(t *testing.T, fifos []*heldFifo, hire func()) time.Duration {
	t.Helper()
	done := make(chan struct{})
	start := time.Now()
	go func() { defer close(done); hire() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Errorf("the hire was still going after 10s (opened: %q)", fifosOpened(fifos))
		for _, f := range fifos {
			f.release()
		}
		<-done
	}
	return time.Since(start)
}
