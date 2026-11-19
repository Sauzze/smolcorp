package desktop

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXMAKEPROJECT under the real profile, in FXGMROOT's default layout (newGMRootLayout): the core makes the GM's new
// project folder and a clone at its ask, and the GM's sandbox still writes neither, nor makes one of its own; its
// next start is made as before.
func TestTheGMsSandboxStillWritesNoProjectTheCoreMadeForIt(t *testing.T) {
	var l gmRootLayout
	d := newTestDeckAfter(t, func() { l = newGMRootLayout(t) })
	managerOn(d)
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	gm := d.newGMToken()
	address := httpsGitStandIn(t) + "/site.git"
	for _, body := range []map[string]any{{"name": "fresh"}, {"name": "site", "clone": address}} {
		if code, text := askProject(t, d, callerGM, gm, "", body); code != http.StatusOK {
			t.Fatalf("%v: %d %s", body, code, text)
		}
	}
	fresh, site := filepath.Join(l.projects, "fresh"), filepath.Join(l.projects, "site")
	profile, _ := l.profile(t, l.gm) // the GM's next start, with both there
	const no = "PermissionError"
	l.run(t, profile, l.projects, []probeStep{
		{"write", fresh + "/notes.md", no}, {"mkdir", fresh + "/src", no}, {"chmod", fresh, no}, {"rename", fresh, no},
		{"write", site + "/README", no}, {"write", site + "/.git/config", no}, {"mkdir", site + "/.git/hooks", no}, // the clone has no hooks folder: it got no templates
		{"write", site + "/.git/refs/heads/main", no}, {"read", site + "/README", "ok"},
		{"mkdir", l.projects + "/its-own", no},
	})
	if data, _ := os.ReadFile(site + "/README"); string(data) != "a stand-in project\n" {
		t.Errorf("the clone's README is now %q", data)
	}
	if entries, _ := os.ReadDir(fresh); len(entries) != 0 {
		t.Errorf("the new folder holds %v", entries)
	}
}

// projectRouteOn is a test deck in FXGMROOT's default layout, with the manager and hiring on: the layout, the deck
// and the GM's token.
func projectRouteOn(t *testing.T) (gmRootLayout, *Deck, string) {
	t.Helper()
	var l gmRootLayout
	d := newTestDeckAfter(t, func() { l = newGMRootLayout(t) })
	managerOn(d)
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	return l, d, d.newGMToken()
}

// askProjectLater is askProject in the background: its answer, code then text, on the channel.
func askProjectLater(t *testing.T, d *Deck, gm string, body map[string]any) <-chan [2]any {
	done := make(chan [2]any, 1)
	go func() {
		code, text := askProject(t, d, callerGM, gm, "", body)
		done <- [2]any{code, text}
	}()
	return done
}

// foldersIn are root and the folders below it, at most depth levels down, as the owner sees them now.
func foldersIn(root string, depth int) []string {
	var found []string
	_ = filepath.WalkDir(root, func(at string, e fs.DirEntry, err error) error {
		if err == nil && e.IsDir() {
			found = append(found, at)
			if rel, _ := filepath.Rel(root, at); rel != "." && strings.Count(rel, "/") >= depth-1 {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return found
}

// FXMAKEPROJECTR1 B1, under the GM's real profile: while the GM's clone waits on the server, the GM plants a
// post-checkout hook in its core's TMPDIR and in every folder there (a place it may write), and in every folder of
// the clone's staging folder (which it may not), with a config there too. The core's git runs no hook it planted.
func TestNoHookTheGMPlantsWhileItsCloneRunsIsRun(t *testing.T) {
	l, d, gm := projectRouteOn(t)
	base, held, release := heldGitStandIn(t, true)
	profile, _ := l.profile(t, l.gm)
	tmp := filepath.Join(l.base, "tmp") // the core's TMPDIR, one the GM may write, as the owner's per-user one is
	if err := os.Mkdir(tmp, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	done := askProjectLater(t, d, gm, map[string]any{"name": "site", "clone": base + "/site.git"})
	select {
	case <-held:
	case <-time.After(30 * time.Second):
		t.Fatal("the clone never reached the stand-in")
	}
	var steps []probeStep
	for _, folder := range foldersIn(tmp, 3) {
		steps = append(steps, probeStep{"hook", folder + "/post-checkout", "ok"})
	}
	staging := foldersIn(filepath.Join(supportDir, gmProjectStagingName), 4)
	for _, folder := range staging {
		steps = append(steps, probeStep{"hook", folder + "/post-checkout", "refused"}, probeStep{"write", folder + "/config", "refused"})
	}
	l.run(t, profile, l.projects, steps)
	release()
	if a := <-done; a[0] != http.StatusOK {
		t.Errorf("the clone: %v %v", a[0], a[1])
	}
	if _, err := os.Lstat(l.planted); !errors.Is(err, os.ErrNotExist) {
		t.Error("the core's git ran a hook the GM planted, as the owner")
	}
	if len(staging) < 4 { // the staging folder, the clone's own, its checkout and its .git
		t.Errorf("the GM found only %q to plant in while the clone ran", staging)
	}
	if data, _ := os.ReadFile(filepath.Join(l.projects, "site", "README")); string(data) != "a stand-in project\n" {
		t.Errorf("the clone's README: %q", data)
	}
}

// FXMAKEPROJECTR1 B2, under a hire's real profile: while the clone waits on the server, the hire tries to move away
// whatever is at the new project's name, put a link there to a folder it is refused, and move the clone's staging
// folders. Since FXHIREPROJECTS the projects folder is held from every desk, so the hire can do none of it (FXSTACKG
// G22); the same link is then made outside its sandbox, as a desk under an older profile or another program could.
// The clone never lands through the link: the route refuses with words, the link and the folder it points at stay as
// they were, and staging is emptied.
func TestAHireCantSteerTheProjectsCloneThroughALink(t *testing.T) {
	l, d, gm := projectRouteOn(t)
	base, held, release := heldGitStandIn(t, true)
	profile, _ := l.profile(t, l.hire)
	target := filepath.Join(hireFolder(supportDir, "gm1"), "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(l.projects, "site")
	done := askProjectLater(t, d, gm, map[string]any{"name": "site", "clone": base + "/site.git"})
	select {
	case <-held:
	case <-time.After(30 * time.Second):
		t.Fatal("the clone never reached the stand-in")
	}
	steps := []probeStep{{"write", target + "/direct", "refused"}}
	_, err := os.Lstat(folder)
	there := err == nil // the clone's folder made at the name, as before the fix
	if there {
		steps = append(steps, probeStep{"rename", folder, "refused"})
	}
	steps = append(steps, probeStep{"symlink " + target, folder, "refused"})
	for _, f := range foldersIn(filepath.Join(supportDir, gmProjectStagingName), 2) {
		steps = append(steps, probeStep{"rename", f, "refused"})
	}
	l.run(t, profile, l.hireTree, steps)
	// The same plant, made outside the hire's sandbox.
	if there {
		if err := os.Rename(folder, folder+".moved"); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(target, folder); err != nil {
		t.Fatal(err)
	}
	release()
	a := <-done
	want := "Something called site appeared in " + l.projects + " while Git was cloning, so Smolcorp left it as it is and kept nothing of the clone: pick another name."
	if a[0] != http.StatusConflict || a[1] != want {
		t.Errorf("the clone: %v %q, want 409 %q", a[0], a[1], want)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("the clone landed through the link, in %v", entries)
	}
	if to, _ := os.Readlink(folder); to != target {
		t.Errorf("the link at the name now points at %q", to)
	}
	if entries, _ := os.ReadDir(filepath.Join(supportDir, gmProjectStagingName)); len(entries) != 0 {
		t.Errorf("staging still holds %v", entries)
	}
	if lines := projectLines(d); len(lines) != 0 {
		t.Errorf("the office's lines: %+v", lines)
	}
}

// The same, untimed (RT126FIXREVIEW's race): a sandboxed hire spins on the projects folder and, the moment a folder
// shows at the name, moves it away and puts a link there to a folder it is refused. The clone never lands there.
func TestAHireRacingTheProjectRouteNeverGetsTheClone(t *testing.T) {
	l, d, gm := projectRouteOn(t)
	address := httpsGitStandIn(t) + "/site.git"
	profile, _ := l.profile(t, l.hire)
	const racer = `import os, sys, time
folder, target = sys.argv[1], sys.argv[2]
end = time.time() + 20
while time.time() < end:
    if os.path.isdir(folder) and not os.path.islink(folder):
        try:
            os.rename(folder, folder + ".moved")
            os.symlink(target, folder)
            print("swapped")
        except OSError as e:
            print(type(e).__name__)
        break
else:
    print("timeout")`
	for _, name := range []string{"race1", "race2", "race3"} {
		target := filepath.Join(hireFolder(supportDir, "gm1"), "elsewhere-"+name)
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, deskPython(t), "-I", "-c", racer, filepath.Join(l.projects, name), target))
		cmd.Dir, cmd.Env = l.hireTree, gmRootEnv()
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond) // the racer is spinning
		code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": name, "clone": address})
		_ = cmd.Wait()
		if entries, _ := os.ReadDir(target); len(entries) != 0 || code != http.StatusOK && code != http.StatusConflict {
			t.Errorf("%s: racer %q; route %d %s; the refused folder holds %v", name, strings.TrimSpace(out.String()), code, text, entries)
		}
	}
}
