package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FLD: since FXC3 a hire's folder in the registry is the project it was hired into (the Mac's meaning), its
// worktree where it works, and startedIn the folder its session started in. These are the places code written
// before then still read the folder as where the hire works.

// A hire into a folder inside its project, named through a link (an office desk's pin or the folder for new hires
// is kept as written, not resolved), started at the same place in its worktree. startedIn finds that place with
// the links resolved, as the Mac's HireDesk.prepare does (`given = physical(request.folder)`, against the physical
// repo), so Take over still sees the hire isn't at its worktree's top, and Hand back starts it where it was.
func TestAHireIntoALinkedFolderStartedAtTheSamePlaceInItsWorktree(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "app")
	tree := filepath.Join(base, "app-hires", "rex-abcdef")
	for _, folder := range []string{filepath.Join(repo, "web"), tree} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "linked")
	if err := os.Symlink(repo, link); err != nil {
		t.Skip("links can't be made here:", err)
	}
	h := Hire{ID: "abcdef123456", Name: "Rex", Role: "reviewer", Folder: filepath.Join(link, "web"), Repo: physical(repo),
		Worktree: physical(tree), Branch: "hire/rex-abcdef", TmuxSocket: "/tmp/tmux-501/smolcorp-abcdef123456", State: "working"}
	if got, want := startedIn(h), filepath.Join(h.Worktree, "web"); got != want {
		t.Errorf("started in %q, want %q", got, want)
	}
	// The project's top, through the same link, is the worktree's top.
	top := h
	top.Folder = link
	if got := startedIn(top); got != top.Worktree {
		t.Errorf("hired into the project's top, started in %q, want %q", got, top.Worktree)
	}
	// Everything else about it is takeable: its session is on the default account.
	session := "0b7e3c1a-1234-4abc-8def-0123456789ab"
	transcript := filepath.Join(base, ".claude", "projects", "-app-web", session+".jsonl")
	if why := t3TakeOverRefusal(top, t3Session{ID: session, Transcript: transcript}, base, false); why != "" {
		t.Fatalf("Take over of a hire at its worktree's top: %q", why)
	}
	why := t3TakeOverRefusal(h, t3Session{ID: session, Transcript: transcript}, base, false)
	if !strings.Contains(why, "works in a folder inside its worktree") {
		t.Errorf("Take over of a hire started below its worktree's top: %q", why)
	}
}

// A launcher with no hire id is matched to an older hire by the hire's own place, and given up when another open
// hire may own that window. Another hire with a worktree works in its worktree, not in the project its folder now
// names: a row written the Go way before 1.25 (folder = its worktree) is read with the project as its folder, and
// must still not make a window in the project look like that hire's.
func TestAnotherHiresProjectIsNotWhereItWorks(t *testing.T) {
	d := newTestDeck(t)
	a := launcherHire() // no worktree: it works in /work/api
	b := Hire{ID: "bbbbbb123456", Name: "Bo", Role: "tester", Folder: "/work/api-hires/bo-bbbbbb", Worktree: "/work/api-hires/bo-bbbbbb",
		Repo: "/work/api", Branch: "hire/bo-bbbbbb", State: "working"}
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": []Hire{a, b}})
	if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readRegistry(registryPath(), false).hires[1].Folder; got != "/work/api" {
		t.Fatalf("the old row's folder reads as %q, want the project", got)
	}
	newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	if id, ok := d.hireLauncherID(a); !ok || id != "L1" {
		t.Errorf("a window in a's folder, with b in its own worktree: %q %v", id, ok)
	}
	// A window in b's worktree is still never a's.
	d2 := newTestDeck(t)
	if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	newFakeLauncher(t, d2, "L2", "/work/api-hires/bo-bbbbbb", "", "idle-empty", true)
	if id, ok := d2.hireLauncherID(a); ok {
		t.Errorf("b's window was taken for a's: %q", id)
	}
}
