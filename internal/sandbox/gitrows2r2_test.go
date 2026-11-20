//go:build unix

package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// FXGITROWS2R2 (RT126FIXREVIEW-GITROWS2R1's B1) in the Policy, on a stand-in .git in a temp folder: a Mac desk's
// Policy with the owner's checkout on topic/x and the default branch team/main. Its real-profile rows are in
// gitrows2r2_darwin_test.go and desktop/deskgitrows_darwin_test.go.

// refRows is a Mac desk's Places around a stand-in shared .git, made with refs/heads and logs/refs/heads, and folder
// outside it, empty: where a link leads.
func refRows(t *testing.T) (pl Places, git, outside string) {
	t.Helper()
	base := shortTemp(t)
	h := filepath.Join(base, "h")
	pl = Places{GOOS: "darwin", Subject: Hire, Home: h, Support: filepath.Join(h, "AD"), OwnID: "h1",
		Root: filepath.Join(h, "app-hires", "h1"), Checkout: filepath.Join(h, "app"), GitDir: filepath.Join(h, "app", ".git"),
		OwnWorktree: "h1", HiresFolder: filepath.Join(h, "app-hires"), CheckoutBranch: "topic/x", DefaultBranch: "team/main"}
	outside = filepath.Join(base, "out")
	for _, folder := range []string{"refs/heads", "logs/refs/heads", "refs/tags"} {
		if err := os.MkdirAll(filepath.Join(pl.GitDir, folder), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	return pl, pl.GitDir, outside
}

// No keeper or held folder in refs/, logs/ or on the way to a base branch's name is made through a link, nor is one
// that is a link taken as made: each is a *RefPlaceError naming it, and nothing is made where it leads. A file on the
// way (a reftable repo's refs/heads) leaves the rest unmade, as MakeHold does. With no link, every keeper is made, a
// folder in the .git.
func TestTheKeepersAreNeverMadeThroughALink(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("a Mac desk's Policy spells /tmp as /private/tmp, which only a Mac has: off a Mac it makes nothing in this temp folder")
	}
	pl, git, _ := refRows(t)
	p := mustNew(t, pl)
	if err := p.makeHold(inGit(p.Hold(), git)); err != nil {
		t.Fatal(err)
	}
	if err := p.makeHold(p.Keep()); err != nil {
		t.Fatal(err)
	}
	for _, keeper := range p.Keep() {
		for at := keeper; at != git; at = filepath.Dir(at) {
			if info, err := os.Lstat(at); err != nil || !info.IsDir() {
				t.Errorf("%s: %v, want a folder", at, err)
			}
		}
	}
	for _, c := range []struct{ name, link, folders string }{
		{"a branch's folder", "refs/heads/hire", "keep"}, // the owner's topic needs no keeper since FXGITROWS2R4
		{"a reflog's folder", "logs/refs/heads/hire", "keep"},
		{"a keeper", "refs/heads/hire/.smolcorp-keep", "keep"},
		// No desk writes the rest of the .git since FXGITROWS2R6 (rowGitTop), so team is no Hold: nothing is made there.
		{"a base branch's name beside the .git", "team", "none"},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			link := filepath.Join(git, filepath.FromSlash(c.link))
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			p := mustNew(t, pl)
			folders := p.Keep()
			if c.folders == "hold" {
				folders = inGit(p.Hold(), git)
			}
			var at *RefPlaceError
			if c.folders == "none" {
				if folders = inGit(p.Hold(), git); slices.Contains(folders, link) {
					t.Errorf("%s is a Hold", link)
				}
				if err := p.makeHold(folders); err != nil {
					t.Errorf("the Hold: %v", err)
				}
			} else if err := p.makeHold(folders); !errors.As(err, &at) || at.At != link || !at.Link {
				t.Errorf("made through the link at %s: %v", link, err)
			}
			if names := namesOf(t, outside); len(names) != 0 {
				t.Errorf("made where the link leads: %q", names)
			}
		})
	}
	t.Run("a file on the way", func(t *testing.T) {
		pl, git, _ := refRows(t)
		heads := filepath.Join(git, "refs", "heads")
		if err := os.Remove(heads); err != nil {
			t.Fatal(err)
		}
		if err := fileAt(heads, ""); err != nil {
			t.Fatal(err)
		}
		p := mustNew(t, pl)
		if err := p.makeHold(p.Keep()); err != nil {
			t.Errorf("below a file: %v", err)
		}
		if info, err := os.Lstat(heads); err != nil || !info.Mode().IsRegular() {
			t.Errorf("refs/heads: %v", err)
		}
	})
}

// On a Mac, a start looks at the whole of the shared .git's refs/ and logs/, and at each folder on the way to a base
// branch's name beside them: a link there (at a folder or a file, held now or not, a symbolic ref git wrote as a link
// included), a file with a second name, or anything but a folder or a file refuses it (FXGITROWS2R3, from
// RT126FIXREVIEW-GITROWS2R2's B1). A file on the way, a place not there yet, a symbolic ref git wrote as a file, a
// missing logs/, and the owner's linked hooks folder are passed over.
func TestOnAMacTheStartLooksAtEveryFolderOnTheWayToAHeldRef(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of the system it runs on: a Mac's here")
	}
	pl, _, _ := refRows(t)
	if err := mustNew(t, pl).CheckRefPlaces(); err != nil {
		t.Errorf("a clean .git: %v", err)
	}
	inFolder := func(make func(at, outside string) error) func(at, outside string) error {
		return func(at, outside string) error {
			if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
				return err
			}
			return make(at, outside)
		}
	}
	hardLink := func(at, outside string) error {
		f := filepath.Join(outside, "f")
		if err := os.WriteFile(f, []byte("0123456789012345678901234567890123456789\n"), 0o600); err != nil {
			return err
		}
		return os.Link(f, at)
	}
	for _, c := range []struct {
		name, at string // what is made at at, below the .git
		make     func(at, outside string) error
		refused  string // "link", "symref", "hard" or "other" for what refuses the start; "" when it starts
	}{
		{"a link at the checkout branch's folder", "refs/heads/topic", symlinkTo, "link"},
		{"a link at its reflog's folder", "logs/refs/heads/topic", symlinkTo, "link"},
		{"a link at the default branch's folder", "refs/heads/team", symlinkTo, "link"},
		{"a link at a tag's folder of its name", "refs/tags/team", symlinkTo, "link"},
		{"a link at its name beside the .git", "team", symlinkTo, "link"},
		{"a link at refs/heads/hire", "refs/heads/hire", symlinkTo, "link"},
		{"a link at refs itself", "refs", symlinkTo, "link"},
		{"a link at a folder nothing held is below", "refs/heads/other", symlinkTo, "link"},
		{"a link at the checkout branch's own file", "refs/heads/topic/x", inFolder(symlinkTo), "link"},
		{"a link at its reflog's own file", "logs/refs/heads/topic/x", inFolder(symlinkTo), "link"},
		{"a link at a branch nothing holds yet", "refs/heads/rev/x", inFolder(symlinkTo), "link"},
		{"a link deep in logs/", "logs/refs/remotes/origin/up/x", inFolder(symlinkTo), "link"},
		{"a symbolic ref git wrote as a link", "refs/heads/topic/x", inFolder(func(at, _ string) error {
			return os.Symlink("refs/heads/main", at)
		}), "symref"},
		{"a hard link at a branch nothing holds yet", "refs/heads/rev/x", inFolder(hardLink), "hard"},
		{"a pipe at the default branch's folder", "refs/heads/team", func(at, _ string) error { return syscall.Mkfifo(at, 0o600) }, "other"},
		{"a pipe deep in refs/", "refs/tags/v1", func(at, _ string) error { return syscall.Mkfifo(at, 0o600) }, "other"},
		{"a file at refs/heads", "refs/heads", fileAt, ""},
		{"a symbolic ref git wrote as a file", "refs/heads/topic/x", inFolder(func(at, _ string) error {
			return os.WriteFile(at, []byte("ref: refs/heads/main\n"), 0o600)
		}), ""},
		{"no logs/", "logs", func(string, string) error { return nil }, ""},
		// FXGITROWS2R8 (B1): a folder the desk can write would refuse it now; this one is held, as git's config names it.
		{"the owner's linked hooks folder", "hooks", symlinkTo, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			if c.at == "hooks" {
				pl.GitHooks = []string{outside} // as ReadGitConfig gives core.hooksPath's folder
			}
			at := filepath.Join(git, filepath.FromSlash(c.at))
			if err := os.RemoveAll(at); err != nil {
				t.Fatal(err)
			}
			if err := c.make(at, outside); err != nil {
				t.Fatal(err)
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			if c.refused == "" {
				if err != nil {
					t.Errorf("refused: %v", err)
				}
				return
			}
			kind := "other"
			switch {
			case !errors.As(err, &e):
			case e.Symref != "":
				kind = "symref"
			case e.Link:
				kind = "link"
			case e.Hard:
				kind = "hard"
			}
			if e == nil || e.At != at || e.Ref != c.at || kind != c.refused {
				t.Errorf("%v, want it refused for %s, a %s", err, at, c.refused)
			}
			if c.refused == "symref" && e.Symref != "refs/heads/main" {
				t.Errorf("the symbolic ref's text: %q", e.Symref)
			}
		})
	}
}

func symlinkTo(at, outside string) error { return os.Symlink(outside, at) }

func fileAt(at, _ string) error {
	return os.WriteFile(at, []byte("this repository uses the reftable format\n"), 0o600)
}

// inGit is those of folders in the shared .git git: the others of a made-up Mac's Hold (/usr/lib/cron) may be a
// system's own, which no test makes.
func inGit(folders []string, git string) []string {
	return slices.DeleteFunc(slices.Clone(folders), func(f string) bool { return !strings.HasPrefix(f, git+"/") })
}

// namesOf is the names in folder, sorted; none when it isn't there.
func namesOf(t *testing.T, folder string) []string {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}
