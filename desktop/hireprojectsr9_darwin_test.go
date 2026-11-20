package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// FXHIREPROJECTSR9 (RT126FIXREVIEW-HIREPROJECTSR8 B1, B2, O2, O4) under the real profile, with stand-ins only.
//   - B1: a gitlink a hire commits over a repo it made in its worktree, spelled in another case or Unicode form than
//     the folder, refuses its next start: the path is looked at as git spells it.
//   - B2: the git folders of a hire's submodules, in its worktree's registration, are held from it whole; the core
//     checks out the project's own submodules as it makes the worktree, and the owner's plain `git status` there runs
//     nothing the hire tried to plant. In its registration, what the owner's git runs a program from is held.
//   - O2: a link whose way can't be followed refuses the start.
//   - O4: a hard-linked mirror in a package manager's folder stops the start with words that never point at its other
//     name, the owner's repo it was cloned from.

// r9Plant is the hire's tries in its worktree: a repo at the folder $3 with a core.fsmonitor ($1, which leaves $2), a
// gitlink to it spelled $4, and a commit.
const r9Plant = r6Tries + `c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
t mkdir -p "$3"
t sh -c 'cd "$0" && /usr/bin/git init -q' "$3"
t sh -c 'printf "#!/bin/sh\necho planted > %s\n" "$1" > "$0" && chmod 755 "$0"' "$1" "$2"
t /usr/bin/git -C "$3" config core.fsmonitor "$1"
t sh -c '/usr/bin/git -c core.precomposeunicode=false update-index --add --cacheinfo 160000,$(/usr/bin/git rev-parse HEAD),"$0"' "$4"
t sh -c '/usr/bin/git -c core.precomposeunicode=false '"$c"' commit -q -m evil'`

// B1: the reviewer's three spellings. Each folder is made as the disk keeps it, each gitlink spelled otherwise: in
// another case, with its folders in another case, and in NFD over an NFC folder. Every try passes, the next start is
// refused with the gitlink's own spelling, and the core's git ran nothing. The owner's plain `git status` in the
// worktree then runs the planted fsmonitor: the stand-in is live, and the start was refused for a real repo.
func TestAGitlinkInAnotherSpellingOverARepoStopsItsNextStart(t *testing.T) {
	for _, c := range []struct{ name, folder, gitlink string }{
		{"case", ".build/checkouts/evil", ".build/checkouts/EVIL"},
		{"folder case", ".build/checkouts/evil", ".BUILD/Checkouts/evil"},
		{"NFD over NFC", ".build/checkouts/évil", ".build/checkouts/évil"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(deskStandInVariable, "1") // its next start's git in its sandbox
			l := newHireProjectsLayout(t)
			profile, _ := l.profile(t, l.hire)
			fsm := filepath.Join(l.base, "fsm.sh")
			if got := sandboxedSh(t, profile, l.hireTree, r9Plant, fsm, l.planted, c.folder, c.gitlink); strings.Join(got, " ") != "ok ok ok ok ok ok" {
				t.Fatalf("the hire's tries: %q", got)
			}
			tree, shown := tilde(l.hireTree), tilde(filepath.Join(l.hireTree, filepath.FromSlash(c.gitlink)))
			want := "This hire was set to work in " + tree + ", but it committed " + shown + " on its branch as a submodule, and " + shown + " holds a git repository your project doesn't have: your own git, run in " + tree + ", would run in that repository too, with its settings. So it didn't start. Delete the folder " + shown + ", then start it again. To take the submodule off its branch as well, run `git rm --cached " + c.gitlink + "` in " + tree + " afterwards, and commit."
			if got := r7Start(t, l.hire); got != want {
				t.Errorf("its next start:\n got %q\nwant %q", got, want)
			}
			if _, err := os.Lstat(l.planted); err == nil {
				t.Fatalf("the core's git ran the hire's fsmonitor")
			}
			standInOwnersGit(t, l.hireTree, "status", "--porcelain")
			if _, err := os.Lstat(l.planted); err != nil {
				t.Errorf("the owner's git status ran nothing: the stand-in isn't live (%v)", err)
			}
		})
	}
}

// r9SubmoduleProject gives app a real submodule, sub, as the owner adds one: committed, and initialised in the
// owner's checkout.
func r9SubmoduleProject(t *testing.T, l gmRootLayout) {
	t.Helper()
	sub := filepath.Join(l.base, "subsrc")
	r7Repo(t, sub)
	standInOwnersGit(t, l.app, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	standInOwnersGit(t, l.app, "commit", "-q", "-m", "a submodule")
}

// r9Hire is a worktree hire of app made as the office makes one (prepareHire), with its row and its own folder.
func r9Hire(t *testing.T, l gmRootLayout, id string) Hire {
	t.Helper()
	began := time.Now()
	place, err := prepareHire(hireRequest{HireID: id, Name: "Janitor", Folder: l.app, Worktree: true},
		hirePlaceOptions{Git: runGit, PortFree: func(int) bool { return true }})
	if err != nil {
		t.Fatalf("the hire: %v", err)
	}
	t.Logf("prepareHire took %v", time.Since(began))
	h := Hire{ID: id, Name: "Janitor", Role: "janitor", Folder: place.Folder, Repo: place.Repo, Worktree: place.Worktree, Branch: place.Branch, State: "working"}
	writeRegistryHires(t, l.hire, h)
	if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	return h
}

// r9HeldTries are a hire's tries at its submodule's git folder ($1), with $2 a stand-in fsmonitor: its config, a hook,
// its info, a folder made in it, the config through another spelling of modules, and git's own config write.
const r9HeldTries = r6Tries + `t sh -c 'printf "[core]\n\tfsmonitor = %s\n" "$1" >> "$0/config"' "$1" "$2"
t sh -c 'mkdir -p "$0/hooks" && printf "#!/bin/sh\n" > "$0/hooks/post-checkout"' "$1"
t sh -c 'mkdir -p "$0/info" && printf "x\n" > "$0/info/exclude"' "$1"
t mkdir "$1/modules"
t sh -c 'printf "[core]\n\tfsmonitor = %s\n" "$1" >> "$(dirname "$(dirname "$0")")/MODULES/sub/config"' "$1" "$2"
t /usr/bin/git -C sub config core.fsmonitor "$2"`

// B2, the reviewer's (d) and the controls. A project with a real submodule: the core checks it out as it makes the
// hire's worktree, from the owner's own git folder of it; the hire starts and sees it, edits a file in it, commits
// the project's other files, and can't commit in it. Its submodule's git folder, in its registration, is held: its
// config, hooks, info, a folder made there, another spelling, git's own config write, and its commondir. The owner's plain
// `git status` in the worktree runs nothing, and its next start passes. "made before" is a worktree whose submodule the
// owner's own git checked out there (as before this round): its git folder is held from the hire just the same.
func TestTheSubmodulesInAHiresWorktreeAreHeldFromIt(t *testing.T) {
	for _, made := range []string{"by the core", "before"} {
		t.Run(made, func(t *testing.T) {
			t.Setenv(deskStandInVariable, "1")
			l := newHireProjectsLayout(t)
			r9SubmoduleProject(t, l)
			h := l.hire
			if made == "by the core" {
				h = r9Hire(t, l, "a1b2c3d4e5f6")
			} else {
				standInOwnersGit(t, l.hireTree, "merge", "-q", "--ff-only", "main")
				standInOwnersGit(t, l.hireTree, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init")
			}
			tree := h.Worktree
			registration := strings.TrimSpace(standInOwnersGit(t, tree, "rev-parse", "--path-format=absolute", "--git-dir"))
			gitFolder := filepath.Join(registration, "modules", "sub")
			if data, err := os.ReadFile(filepath.Join(tree, "sub", "README")); err != nil || string(data) != "the owner's\n" {
				t.Fatalf("the submodule isn't checked out in the hire's worktree: %q %v", data, err)
			}
			if got := strings.TrimSpace(standInOwnersGit(t, filepath.Join(tree, "sub"), "rev-parse", "--path-format=absolute", "--git-dir")); got != gitFolder {
				t.Fatalf("the submodule's git folder is %s, want %s", got, gitFolder)
			}
			profile, _ := l.profile(t, h)
			fsm := filepath.Join(l.base, "fsm.sh") // the stand-in a planted core.fsmonitor would run
			if err := os.WriteFile(fsm, []byte("#!/bin/sh\necho planted > '"+l.planted+"'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			got := sandboxedSh(t, profile, tree, r9HeldTries, gitFolder, fsm)
			if want := "refused refused refused refused refused refused"; strings.Join(got, " ") != want {
				t.Errorf("config, hook, info, a folder, MODULES/sub/config, git config:\n got %q\nwant %q", got, want)
			}
			got = sandboxedSh(t, profile, tree, r6Tries+`c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
t sh -c 'echo edited >> sub/README'
t sh -c '/usr/bin/git -C sub '"$c"' commit -q -a -m sub'
t sh -c 'echo edited >> README && /usr/bin/git add README && /usr/bin/git '"$c"' commit -q -m readme'`)
			if want := "ok refused ok"; strings.Join(got, " ") != want {
				t.Errorf("edit a file in the submodule, commit in it, commit the project's file:\n got %q\nwant %q", got, want)
			}
			standInOwnersGit(t, tree, "status", "--porcelain")
			if _, err := os.Lstat(l.planted); err == nil {
				t.Errorf("the owner's git status in the hire's worktree ran what it planted")
			}
			// Its commondir, which would name another folder for the submodule's config, last: on the tree before, the
			// write passed and broke the folder for the look above.
			got = sandboxedSh(t, profile, tree, r6Tries+`t sh -c 'printf "%s\n" "$1" > "$0/commondir"' "$1" "$2"`, gitFolder, filepath.Join(l.lib, ".git"))
			if strings.Join(got, " ") != "refused" {
				t.Errorf("its submodule's commondir: %q", got)
			}
			if words := r7Start(t, h); words != "" {
				t.Errorf("its next start: %s", words)
			}
		})
	}
}

// B2's measure of the hire's own registration, worktrees/<id>: the files there the owner's git reads that can make it
// run a program (commondir names where its config is; config.worktree is its config once the owner turns
// worktreeConfig on; gitdir, which `git worktree` reads, with them; and the submodules' git folders) are held, each
// with its lock; HEAD's neighbours that name no program (logs, ORIG_HEAD, its index's own lock) stay its own.
func TestAHiresRegistrationHoldsWhatTheOwnersGitRunsFrom(t *testing.T) {
	l := newHireProjectsLayout(t)
	registration := strings.TrimSpace(standInOwnersGit(t, l.hireTree, "rev-parse", "--path-format=absolute", "--git-dir"))
	profile, _ := l.profile(t, l.hire)
	got := sandboxedSh(t, profile, l.hireTree, r6Tries+`for f in commondir commondir.lock gitdir gitdir.lock config.worktree config.worktree.lock modules modules/sub/config; do
  t sh -c 'mkdir -p "$(dirname "$0/$1")" 2>/dev/null; printf "x\n" > "$0/$1"' "$1" "$f"
done
for f in logs/standin ORIG_HEAD index.lock; do
  t sh -c 'printf "x\n" > "$0/$1"' "$1" "$f"
done`, registration)
	want := strings.TrimSpace(strings.Repeat("refused ", 8) + "ok ok ok")
	if strings.Join(got, " ") != want {
		t.Errorf("commondir, gitdir, config.worktree (each with its lock), modules, modules/sub/config; logs, ORIG_HEAD, index.lock:\n got %q\nwant %q", got, want)
	}
}

// O2: a link below the place whose way can't be followed (a folder on it the core can't search) refuses the start; a
// desk could make one so a start passes, then undo it. Readable again, it starts (the control), as does a link that
// leads nowhere.
func TestALinkWhoseWayCantBeFollowedStopsItsStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	h := Hire{ID: "r9o", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	r6Desk(t, l, h.ID, l.lib)
	locked := filepath.Join(homeDir, "stage", "locked")
	if err := os.MkdirAll(filepath.Join(locked, "box"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(l.lib, "box")
	if err := os.Symlink(filepath.Join(locked, "box"), link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(homeDir, "stage", "nowhere"), filepath.Join(l.lib, "gone")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	want := "This hire was set to work in " + tilde(l.lib) + ", but Smolcorp can't follow the link " + tilde(link) + " to where it leads, so it couldn't make sure no git repository a hire can change is there, and it didn't start. Make where it leads readable, or remove the link, then start it again."
	if got := r7Start(t, h); got != want {
		t.Errorf("its start with the way unreadable:\n got %q\nwant %q", got, want)
	}
	if err := os.Chmod(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := r7Start(t, h); got != "" {
		t.Errorf("its start with the way readable again: %s", got)
	}
}

// O4: the owner's mirror of a dependency given by a local path, git's local clone with hard-linked objects, in lib's
// .build: the start is refused with words that say to delete .build and build again, never to remove the other
// name, which is an object file of the owner's dependency repo.
func TestAHardLinkedMirrorInAPackageManagersFolderSaysToBuildAgain(t *testing.T) {
	l := newHireProjectsLayout(t)
	dep := filepath.Join(l.projects, "dep")
	r7Repo(t, dep)
	mirror := filepath.Join(l.lib, ".build", "repositories", "dep-1a2b3c4d")
	standInOwnersGit(t, l.lib, "clone", "-q", "--mirror", dep, mirror)
	h := Hire{ID: "r9m", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	writeRegistryHires(t, l.hire, h)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	got := r7Start(t, h)
	tail := " has a second name on this " + machineHere() + " (a hard link), so a desk could read or change it through that name, and the desk didn't start. It's in " + filepath.Join(l.lib, ".build") + ", a package manager's own folder: delete that folder and build again, then start it again."
	if !strings.HasPrefix(got, mirror+string(filepath.Separator)) || !strings.HasSuffix(got, tail) || strings.Contains(got, "samefile") || strings.Contains(got, dep+string(filepath.Separator)) {
		t.Errorf("its start:\n got %q\nwant <a file in %s>%q", got, mirror, tail)
	}
}

// B2's other control: a submodule the owner's checkout hasn't initialised (deinit) stays so in a hire's worktree, with
// no git folder made for it, and the hire starts.
func TestASubmoduleTheOwnerHasntInitialisedStaysSoInAHiresWorktree(t *testing.T) {
	l := newHireProjectsLayout(t)
	r9SubmoduleProject(t, l)
	standInOwnersGit(t, l.app, "submodule", "deinit", "-q", "-f", "sub")
	h := r9Hire(t, l, "a1b2c3d4e5f6")
	if entries, err := os.ReadDir(filepath.Join(h.Worktree, "sub")); err != nil || len(entries) != 0 {
		t.Errorf("the hire's sub: %v %v, want an empty folder", entries, err)
	}
	registration := strings.TrimSpace(standInOwnersGit(t, h.Worktree, "rev-parse", "--path-format=absolute", "--git-dir"))
	if _, err := os.Lstat(filepath.Join(registration, "modules")); err == nil {
		t.Errorf("a git folder was made for a submodule the owner hasn't initialised")
	}
	if words := r7Start(t, h); words != "" {
		t.Errorf("its start: %s", words)
	}
}

// N1: a recap whose worktree was kept says not to run git in it, and how to look at its work and remove it from the
// owner's own checkout; a removed one says nothing of it.
func TestAKeptWorktreesRecapSaysNotToRunGitInIt(t *testing.T) {
	shortSupport(t)
	h := Hire{ID: "abc123", Name: "Janitor", Folder: t.TempDir(), Worktree: "/p/app-hires/janitor-abc123", Branch: "hire/abc123/janitor"}
	read := func(removed bool) string {
		data, err := os.ReadFile(writeRecap(h, "", removed, time.Unix(0, 0)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	want := "- Worktree: /p/app-hires/janitor-abc123 (kept). Don't run git in it: from your own checkout, `git log hire/abc123/janitor` shows its work and `git worktree remove --force /p/app-hires/janitor-abc123` removes it, deleting what it left uncommitted: open or copy those files first, without git.\n"
	if kept := read(false); !strings.Contains(kept, want) {
		t.Errorf("kept recap:\n%s\nwant the line %q", kept, want)
	}
	if gone := read(true); !strings.Contains(gone, "- Worktree: /p/app-hires/janitor-abc123 (removed)\n") || strings.Contains(gone, "git worktree remove") {
		t.Errorf("removed recap:\n%s", gone)
	}
}
