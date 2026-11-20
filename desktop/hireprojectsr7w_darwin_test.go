package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXHIREPROJECTSR7, ruling (W): on a sandboxed Mac, a plain hire (a hire with no role, or a Claude Code role with the
// manager off; Codex, Gemini CLI, OpenCode and Grok are refused there by FXPLANTSTERM) works in a worktree of its own,
// on its own branch hire/<desk>/…, made as the engine's hire's is (prepareHire), and its launcher's sandbox is that
// desk's (--desk). So no plain hire works in the owner's checkout, and RT126FIXREVIEW-HIREPROJECTSR6's B1 (b), an
// owner's repo added below a desk's place in the owner's checkout, can't arise from one. Where the launcher starts no
// sandbox (Linux and Windows, whose launcherSandboxes is false; FXSWITCHPLAIN's switch off at the stack), a plain hire
// goes into the folder as before.

// r7wPlainHire is the office's plain hire's launcher as tmux was asked to start it: its folder, and its --desk.
func r7wPlainHire(t *testing.T, h *hiring) (folder, desk string) {
	t.Helper()
	if len(h.tmux) != 1 || h.tmux[0][0] != "new-session" {
		t.Fatalf("tmux was asked %q, want one new session", h.tmux)
	}
	args := h.tmux[0]
	if i := slices.Index(args, "--desk"); i >= 0 && i+1 < len(args) {
		desk = args[i+1]
	}
	for i, a := range args {
		if a == "-c" && i+1 < len(args) {
			folder = args[i+1]
		}
	}
	return folder, desk
}

// r7wDeck is the office with the plain hire's launcher starting its agent in a sandbox (a Mac's), hiring Claude Code
// with no role.
func r7wDeck(t *testing.T, sandboxed bool) (*Deck, gmRootLayout, *hiring) {
	t.Helper()
	d, l, _ := officeDeck(t)
	h := fakeHiring(t, "/usr/bin/tmux")
	saved := plainHiresSandboxed
	t.Cleanup(func() { plainHiresSandboxed = saved })
	plainHiresSandboxed = sandboxed
	return d, l, h
}

// The office's plain hire on a Mac is asked which project first, says nothing of committing, and lands in its own
// worktree of the project picked, on its own branch; its launcher's sandbox is made there, for that desk, with the
// owner's checkout held, and in it the hire commits on its own branch and can't change the checkout.
func TestTheOfficesPlainHireWorksInAWorktreeOfItsOwn(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	status, said := d.Perform("hire", "", url.Values{})
	if status != http.StatusOK || !strings.Contains(said, `"projects":["app","lib"]`) || strings.Contains(said, "commit") {
		t.Fatalf("the plain hire's picker: %d %s", status, said)
	}
	d.mu.Lock()
	d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"project": {"lib"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into lib: %d %s", status, said)
	}
	folder, desk := r7wPlainHire(t, h)
	if !plainDeskID(desk) || filepath.Dir(folder) != hiresFolder(l.lib, "") || folder == l.lib {
		t.Fatalf("its launcher: in %q for desk %q, want a worktree in %s", folder, desk, hiresFolder(l.lib, ""))
	}
	branch := strings.TrimSpace(standInOwnersGit(t, folder, "rev-parse", "--abbrev-ref", "HEAD"))
	if !strings.HasPrefix(branch, "hire/"+desk+"/") {
		t.Errorf("its branch: %q, want hire/%s/…", branch, desk)
	}
	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	s, err := newLauncherSandboxAt(desk, "", folder, false, "L1", Claude, program)
	if err != nil {
		t.Fatalf("its sandbox in its worktree: %v", err)
	}
	t.Cleanup(s.close)
	if pl := s.desk.places; pl.Root != folder || pl.Checkout != l.lib || pl.OwnWorktree == "" {
		t.Errorf("its sandbox's places: root %q, checkout %q, own worktree %q", pl.Root, pl.Checkout, pl.OwnWorktree)
	}
	got := sandboxedSh(t, s.profile, folder, r6Tries+`c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
t sh -c 'echo own > own.txt && /usr/bin/git '"$c"' add own.txt && /usr/bin/git '"$c"' commit -q -m own'
t sh -c 'echo x > "$0/README"' "$1"
t sh -c 'echo x > "$0/.git/hooks/post-checkout"' "$1"`, l.lib)
	if want := "ok refused refused"; strings.Join(got, " ") != want {
		t.Errorf("in its sandbox: %q, want %q (its commit, the checkout's file, the checkout's hook)", got, want)
	}
	if subject := strings.TrimSpace(standInOwnersGit(t, l.lib, "log", "--format=%s", "-1", branch)); subject != "own" {
		t.Errorf("its branch's last commit: %q", subject)
	}
}

// (b) can't arise: Setup's plain hire into the owner's checkout, by its own route, also lands in a worktree of its
// own. With the owner's switch off it starts outside its sandbox in the checkout, as before (FXSWITCHPLAIN; one
// plainHireSandboxed, FXSTACKG G22), told so (--sandbox-off), with no desk; if the switch is on again by its
// launcher's start, the launcher refuses rather than sandbox it at the checkout (sandboxOnAgainWords).
func TestNoRouteStartsASandboxedPlainHireAtTheOwnersCheckout(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch on", true: "switch off"}[off], func(t *testing.T) {
			d, l, h := r7wDeck(t, true)
			d.mu.Lock()
			d.config.SandboxOff = off
			d.mu.Unlock()
			if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
				t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
			}
			folder, desk := r7wPlainHire(t, h)
			if !off {
				if folder == l.lib || !plainDeskID(desk) || filepath.Dir(folder) != hiresFolder(l.lib, "") {
					t.Errorf("its launcher in %q for desk %q, want its own worktree", folder, desk)
				}
				return
			}
			if folder != l.lib || desk != "" || !slices.Contains(h.tmux[0], "--sandbox-off") {
				t.Fatalf("its launcher in %q for desk %q, asked %q: want lib itself, no desk, --sandbox-off", folder, desk, h.tmux[0])
			}
			run, err := parseRun(h.tmux[0][slices.Index(h.tmux[0], "--hire"):])
			if err != nil {
				t.Fatalf("its launcher's arguments: %v", err)
			}
			if why := run.sandboxOnAgainWords(true, l.lib); why != plainHireSandboxOnAgain {
				t.Errorf("the switch on again by its start: %q, want the refusal", why)
			}
		})
	}
}

// Where the launcher starts no sandbox (Linux and Windows; at the stack, FXSWITCHPLAIN's switch off), a plain hire goes
// into the folder as before: no worktree, no branch, no --desk.
func TestAPlainHireWithNoSandboxGoesIntoTheFolderAsBefore(t *testing.T) {
	d, l, h := r7wDeck(t, false)
	if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
		t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
	}
	if folder, desk := r7wPlainHire(t, h); folder != l.lib || desk != "" {
		t.Errorf("its launcher in %q for desk %q, want lib itself and none", folder, desk)
	}
	if _, err := os.Stat(hiresFolder(l.lib, "")); err == nil {
		t.Errorf("a worktree folder was made: %s", hiresFolder(l.lib, ""))
	}
	if branches := standInOwnersGit(t, l.lib, "branch", "--list", "hire/*"); strings.TrimSpace(branches) != "" {
		t.Errorf("branches made: %q", branches)
	}
}

// The launcher takes the desk the office named only in its shape, and only for a plain hire, never an engine hire's.
func TestTheLauncherTakesAPlainHiresDesk(t *testing.T) {
	const token = "0123456789ABCDEF0123456789ABCDEF"
	run, err := parseRun([]string{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "claude"})
	if err != nil || run.desk != "plain-0a1b2c3d4e5f" || run.hire != token || strings.Join(run.command, " ") != "claude" {
		t.Errorf("a plain hire's desk: %+v %v", run, err)
	}
	for _, args := range [][]string{
		{"--hire", token, "--desk", "../h1", "claude"},
		{"--hire", token, "--desk", "plain-0A1B2C3D4E5F", "claude"},
		{"--hire", token, "--desk", "h1", "claude"},
		{"--hire", token, "--hire-id", "h1", "--desk", "plain-0a1b2c3d4e5f", "--role", "janitor"},
	} {
		if run, err := parseRun(args); err == nil {
			t.Errorf("%q: %+v", args, run)
		}
	}
}
