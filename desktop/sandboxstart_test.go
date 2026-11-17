package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// linkTree is a stand-in desk's places in a temp folder: a hire in its worktree of the owner's repo, with Smolcorp's
// support folder, the owner's home and the shared .git. Every file holds a stand-in (FAKEKEY, {"fence":1}).
type linkTree struct {
	base, home, support, own, hires, root, repo, git string
}

func newLinkTree(t *testing.T) linkTree {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, "L", "AD")
	hires := filepath.Join(home, "P", "app-hires")
	repo := filepath.Join(home, "P", "app")
	tr := linkTree{base: base, home: home, support: support, own: filepath.Join(support, "hires", "h1"), hires: hires,
		root: filepath.Join(hires, "h1"), repo: repo, git: filepath.Join(repo, ".git")}
	for rel, text := range map[string]string{
		"home/L/AD/setup-key": "FAKEKEY", "home/L/AD/hire-guard.json": `{"fence":1}`, "home/L/AD/nightshift.json": `{"jobs":[]}`,
		"home/L/AD/events.jsonl": "{}\n", "home/L/AD/hires/h1/BRIEF.md": "brief", "home/L/AD/hires/h1/STATUS.md": "status",
		"home/L/AD/hires/h2/BRIEF.md": "another brief", "home/.zshrc": "# standin", "home/.claude.json": "{}",
		"home/.claude/settings.json": "{}", "home/.claude/projects/p/x.jsonl": "{}\n", "home/.config/git/ignore": "*.o\n",
		"home/P/app/main.go": "package main", "home/P/app/.claude/settings.json": "{}", "home/P/app-hires/h1/main.go": "package main",
		"home/P/app-hires/h1/.claude/settings.json": "{}", "home/P/app/.git/config": "[core]\n", "home/P/app/.git/HEAD": "ref: refs/heads/main\n",
		"home/P/app/.git/refs/heads/main": "1111\n", "home/P/app/.git/hooks/pre-commit": "#!/bin/sh\n",
		"home/P/app/.git/modules/sub/HEAD": "1111\n", "home/P/app/.git/modules/sub/config": "[core]\n",
		"home/P/app/.git/modules/sub/objects/ab/cdef": "standin", "home/P/app/.git/worktrees/h1/HEAD": "1111\n",
		"home/P/app/.git/objects/ab/cdef": "standin",
	} {
		at := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

func (tr linkTree) places() sandbox.Places {
	return sandbox.Places{Home: tr.home, Support: tr.support, OwnID: "h1", Root: tr.root, Checkout: tr.repo, GitDir: tr.git,
		OwnWorktree: "h1", OtherWorktrees: []string{"h1"}, HiresFolder: tr.hires}
}

func (tr linkTree) policy(t *testing.T) *sandbox.Policy {
	t.Helper()
	p, err := sandbox.New(tr.places())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// link gives the file at target a second name in the desk's worktree, where a desk may write, as a process outside any
// sandbox could have before the desk's start.
func (tr linkTree) link(t *testing.T, target string) string {
	t.Helper()
	other := filepath.Join(tr.root, "other-name")
	if err := os.Link(target, other); err != nil {
		t.Fatal(err)
	}
	return other
}

// linkMet makes the desk's status file, which it may write and the check looks at, a second name of the file at target,
// as a process outside any sandbox could have before the desk's start.
func (tr linkTree) linkMet(t *testing.T, target string) string {
	t.Helper()
	other := filepath.Join(tr.own, "STATUS.md")
	if err := os.Remove(other); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, other); err != nil {
		t.Fatal(err)
	}
	return other
}

func TestAFileKeptFromDesksWithASecondNameIsReplacedOrRefusesTheStart(t *testing.T) {
	const replaced, refused, nothing = "replaced", "refused", "nothing"
	for _, c := range []struct {
		rel, want string
		met       bool // the second name is the desk's status file, which the check looks at; else one in its worktree
	}{
		// Smolcorp's own files a desk reads but doesn't write, whose every name the check met: a fresh copy, the other
		// name keeps a dead one.
		{"home/L/AD/hire-guard.json", replaced, true}, {"home/L/AD/nightshift.json", replaced, true}, {"home/L/AD/hires/h1/BRIEF.md", replaced, true},
		// The same with the other name in the desk's worktree, where the check doesn't look: it might be a transcript
		// as well, so the start is refused (X12LINK1).
		{"home/L/AD/hire-guard.json", refused, false}, {"home/L/AD/nightshift.json", refused, false}, {"home/L/AD/hires/h1/BRIEF.md", refused, false},
		// A key, Smolcorp's other files and another desk's: a copy would still read the same; the start is refused.
		{"home/L/AD/setup-key", refused, false}, {"home/L/AD/events.jsonl", refused, false}, {"home/L/AD/hires/h2/BRIEF.md", refused, false},
		{"home/L/AD/setup-key", refused, true},
		// The owner's own files: plants, fences, the account folder, the shared git places, a submodule's.
		{"home/.zshrc", refused, false}, {"home/.config/git/ignore", refused, false}, {"home/.claude.json", refused, false}, {"home/.claude/settings.json", refused, false},
		{"home/P/app/.claude/settings.json", refused, false}, {"home/P/app-hires/h1/.claude/settings.json", refused, false},
		{"home/P/app/.git/config", refused, false}, {"home/P/app/.git/HEAD", refused, false}, {"home/P/app/.git/refs/heads/main", refused, false},
		{"home/P/app/.git/hooks/pre-commit", refused, false}, {"home/P/app/.git/modules/sub/config", refused, false},
		// What a desk may write anyway, and the places the check passes over (linkCheckSkipped, accountFoldersSkipped).
		{"home/L/AD/hires/h1/STATUS.md", nothing, false}, {"home/P/app/.git/objects/ab/cdef", nothing, false}, {"home/P/app/main.go", nothing, false},
		{"home/.claude/projects/p/x.jsonl", nothing, false}, {"home/P/app/.git/modules/sub/objects/ab/cdef", nothing, false},
	} {
		t.Run(fmt.Sprintf("%s/met=%v", c.rel, c.met), func(t *testing.T) {
			tr := newLinkTree(t)
			target := filepath.Join(tr.base, filepath.FromSlash(c.rel))
			before, _ := os.Lstat(target)
			// On Windows, Go reads a FileInfo's file id from its path at the first SameFile, not at Lstat: read it now,
			// or before is whatever file holds the name by then (the fresh copy).
			os.SameFile(before, before)
			text, _ := os.ReadFile(target)
			other := tr.link(t, target)
			if c.met {
				if err := os.Remove(other); err != nil {
					t.Fatal(err)
				}
				other = tr.linkMet(t, target)
			}
			got, err := checkLinks(tr.policy(t), tr.places())
			after, statErr := os.Lstat(target)
			if statErr != nil {
				t.Fatal(statErr)
			}
			otherInfo, _ := os.Lstat(other)
			switch c.want {
			case replaced:
				if err != nil || !slices.Equal(got, []string{target}) {
					t.Fatalf("checkLinks: %v, replaced %v", err, got)
				}
				if os.SameFile(after, before) || os.SameFile(after, otherInfo) || after.Mode() != before.Mode() {
					t.Errorf("not a fresh copy in its place: same as before %v, as the other name %v, mode %v→%v",
						os.SameFile(after, before), os.SameFile(after, otherInfo), before.Mode(), after.Mode())
				}
				if now, _ := os.ReadFile(target); string(now) != string(text) {
					t.Errorf("the copy holds %q, want %q", now, text)
				}
				// A write through the other name now lands in the dead copy.
				if err := os.WriteFile(other, []byte("standin"), 0o600); err != nil {
					t.Fatal(err)
				}
				if now, _ := os.ReadFile(target); string(now) != string(text) {
					t.Errorf("a write through the other name reached the file: %q", now)
				}
				if names, _ := os.ReadDir(filepath.Dir(target)); slices.ContainsFunc(names, func(e fs.DirEntry) bool { return strings.Contains(e.Name(), ".fresh-") }) {
					t.Errorf("a temp file was left beside it: %v", names)
				}
			case refused:
				var words *linkCheckError
				if !errors.As(err, &words) || !strings.Contains(err.Error(), target) || !strings.Contains(err.Error(), "second name") || len(got) != 0 {
					t.Fatalf("checkLinks: %v, replaced %v; want the start refused, naming %s", err, got, target)
				}
				if !os.SameFile(after, before) {
					t.Error("a file the start refused for was changed")
				}
			default:
				if err != nil || len(got) != 0 {
					t.Fatalf("checkLinks: %v, replaced %v; want nothing", err, got)
				}
			}
		})
	}
}

// TestAReadableNameOfAKeyRefusesTheStartWithNothingReplaced: a file desks may read (but not write) made, before the
// start, a second name of one they can't read refuses the start, whichever of the two the walk meets first, and
// nothing is replaced, not even a file that alone would be (RT126SAX12's B1).
func TestAReadableNameOfAKeyRefusesTheStartWithNothingReplaced(t *testing.T) {
	for _, c := range []struct {
		readable, key, alone string
		keyFirst             bool
	}{
		// The reviewer's probe: the guard file, a second name of the setup key, met before it.
		{"home/L/AD/hire-guard.json", "home/L/AD/setup-key", "home/L/AD/nightshift.json", false},
		{"home/L/AD/hires/h1/BRIEF.md", "home/L/AD/setup-key", "home/L/AD/hire-guard.json", false},
		// The key met first: another desk's brief, and the account file in the owner's home.
		{"home/L/AD/nightshift.json", "home/L/AD/hires/h2/BRIEF.md", "home/L/AD/hire-guard.json", true},
		{"home/L/AD/hire-guard.json", "home/.claude.json", "home/L/AD/nightshift.json", true},
	} {
		t.Run(c.readable+" as "+c.key, func(t *testing.T) {
			tr := newLinkTree(t)
			at := func(rel string) string { return filepath.Join(tr.base, filepath.FromSlash(rel)) }
			readable, key, alone := at(c.readable), at(c.key), at(c.alone)
			if (key < readable) != c.keyFirst {
				t.Fatalf("the walk meets %s first, not as the row says", map[bool]string{true: key, false: readable}[key < readable])
			}
			pol := tr.policy(t)
			if v, _ := pol.Decide(key, sandbox.Read); v != sandbox.Deny {
				t.Fatalf("%s: desks may read it, so it is no key here", key)
			}
			if v, _ := pol.Decide(readable, sandbox.Read); v == sandbox.Deny {
				t.Fatalf("%s: desks can't read it", readable)
			}
			// As a process outside any sandbox could have, before the start.
			if err := os.Remove(readable); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(key, readable); err != nil {
				t.Fatal(err)
			}
			aloneOther := tr.linkMet(t, alone)
			sums := treeSums(t, tr.base)
			got, err := checkLinks(pol, tr.places())
			var words *linkCheckError
			if !errors.As(err, &words) || !strings.Contains(err.Error(), readable) || !strings.Contains(err.Error(), key) || len(got) != 0 {
				t.Fatalf("checkLinks: %v, replaced %v; want the start refused, naming %s and %s", err, got, readable, key)
			}
			if now := treeSums(t, tr.base); !slices.Equal(now, sums) {
				t.Error("a refused start changed a file")
			}
			a, _ := os.Lstat(readable)
			b, _ := os.Lstat(key)
			if !os.SameFile(a, b) {
				t.Error("the readable name was replaced")
			}
			// A fresh copy keeps the bytes and the mode, so the sums can't tell: the file that alone would be
			// replaced is still the same file as its other name.
			c, _ := os.Lstat(alone)
			d, _ := os.Lstat(aloneOther)
			if !os.SameFile(c, d) {
				t.Errorf("%s was replaced in a refused start", alone)
			}
		})
	}
}

// TestARefusedStartReplacesNothing: a file that refuses the start, met before or after one that alone would be
// replaced, leaves that one as it was: every decision is made before anything is replaced (RT126SAX12's B1).
func TestARefusedStartReplacesNothing(t *testing.T) {
	for _, refuses := range []string{"home/.zshrc", "home/L/AD/setup-key", "home/P/app/.git/config"} {
		t.Run(refuses, func(t *testing.T) {
			tr := newLinkTree(t)
			guard := filepath.Join(tr.support, "hire-guard.json")
			guardOther := tr.linkMet(t, guard)
			target := filepath.Join(tr.base, filepath.FromSlash(refuses))
			if err := os.Link(target, filepath.Join(tr.root, "another-name")); err != nil {
				t.Fatal(err)
			}
			got, err := checkLinks(tr.policy(t), tr.places())
			if err == nil || !strings.Contains(err.Error(), target) || len(got) != 0 {
				t.Fatalf("checkLinks: %v, replaced %v; want the start refused, naming %s", err, got, target)
			}
			a, _ := os.Lstat(guard)
			b, _ := os.Lstat(guardOther)
			if !os.SameFile(a, b) {
				t.Error("the guard file was replaced in a refused start")
			}
		})
	}
}

func TestWithNoSecondNamesNothingIsReplaced(t *testing.T) {
	tr := newLinkTree(t)
	sums := treeSums(t, tr.base)
	if got, err := checkLinks(tr.policy(t), tr.places()); err != nil || len(got) != 0 {
		t.Fatalf("checkLinks: %v, replaced %v", err, got)
	}
	if now := treeSums(t, tr.base); !slices.Equal(now, sums) {
		t.Error("the check changed a file")
	}
}

func TestTheLinkCheckFailsClosed(t *testing.T) {
	t.Run("too many to look at", func(t *testing.T) {
		tr := newLinkTree(t)
		was := linkCheckMost
		linkCheckMost = 5
		defer func() { linkCheckMost = was }()
		_, err := checkLinks(tr.policy(t), tr.places())
		if err == nil || !strings.Contains(err.Error(), "more than 5") {
			t.Fatalf("checkLinks: %v", err)
		}
	})
	t.Run("a folder of the owner's it can't list", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Getuid() == 0 {
			t.Skip("no folder modes to shut here")
		}
		tr := newLinkTree(t)
		shut := filepath.Join(tr.home, ".config", "git")
		// The Policy is made first: made after, it refuses itself, as it can't tell where links in there lead
		// (FXHOMELINKS, sandbox's TestTheOwnersPlacesAreHeldWhereTheirLinksLead).
		p := tr.policy(t)
		if err := os.Chmod(shut, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(shut, 0o700)
		_, err := checkLinks(p, tr.places())
		if err == nil || !strings.Contains(err.Error(), "couldn't check "+shut) {
			t.Fatalf("checkLinks: %v", err)
		}
	})
}

// TestEveryDenyRowIsLookedAtOrSkippedWithAReason holds the link check to the Policy: every row that denies a desk
// reading or writing a place is looked at, or named in linkCheckSkipped with why, so a row added later is thought
// about once. Every name there is a real row.
func TestEveryDenyRowIsLookedAtOrSkippedWithAReason(t *testing.T) {
	tr := newLinkTree(t)
	pl := tr.places()
	pl.GMFolder, pl.RemoteLogin = filepath.Join(tr.home, "gm"), true
	pl.DeskStart = filepath.Join(tr.support, "desk-start")
	pl.SocketParent, pl.OwnSockets = filepath.Join(tr.base, "k"), filepath.Join(tr.base, "k", "h1")
	pl.Accounts = []string{filepath.Join(tr.home, ".claude-work")}
	pl.InstallRoots = []string{filepath.Join(tr.base, "opt", "homebrew")}
	pl.GitConfigs = []string{filepath.Join(tr.base, "etc", "gitconfig")}
	pl.GitHooks, pl.GitNamed = []string{filepath.Join(tr.repo, ".husky", "_")}, []string{filepath.Join(tr.home, "more.gitconfig")}
	pl.OtherSupports = []string{filepath.Join(tr.base, "test-copy")}
	pl.OtherWorktrees = []string{"h1", "h2"}
	pol, err := sandbox.New(pl)
	if err != nil {
		t.Fatal(err)
	}
	looks, _ := linkLooks(pol, pl)
	looked := func(at string, depth int) bool {
		return slices.ContainsFunc(looks, func(l linkLook) bool {
			return l.path == at && (l.depth < 0 || depth >= 0 && l.depth >= depth) || l.depth < 0 && inside(l.path, at)
		})
	}
	rows := map[string]bool{}
	for _, r := range pol.Rules() {
		rows[r.Row] = true
		if r.Read != sandbox.Deny && r.Write != sandbox.Deny {
			continue
		}
		if _, skipped := linkCheckSkipped[r.Row]; skipped {
			continue
		}
		depth := 0
		switch {
		case r.Below || strings.Contains(r.Tail, "**"):
			depth = -1
		case r.Tail != "":
			depth = strings.Count(r.Tail, "/") + 1
		}
		if r.Path == "" {
			for _, root := range []string{pl.Root, pl.Checkout} {
				if !looked(filepath.Join(root, ".claude"), 1) {
					t.Errorf("row %q (anywhere %s): %s/.claude isn't looked at", r.Row, r.Tail, root)
				}
			}
		} else if !looked(filepath.FromSlash(r.Path), depth) {
			t.Errorf("row %q at %s (%s, below %v) isn't looked at, nor skipped with a reason", r.Row, r.Path, r.Tail, r.Below)
		}
	}
	for row, why := range linkCheckSkipped {
		if !rows[row] || why == "" {
			t.Errorf("linkCheckSkipped names %q, which is no row of the Policy's (or gives no reason)", row)
		}
	}
	// The support folder is walked whole.
	if !looked(tr.support, -1) {
		t.Error("the support folder isn't walked whole")
	}
}

// treeSums is every file below base with its contents' sum, to tell a check changed nothing.
func treeSums(t *testing.T, base string) []string {
	t.Helper()
	var sums []string
	err := filepath.WalkDir(base, func(at string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(at)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		sum := sha256.Sum256(data)
		sums = append(sums, at+" "+info.Mode().String()+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sums
}

// gitPlantTree is a shared .git with plants from before the sandbox, and things that aren't plants.
func gitPlantTree(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git isn't installed")
	}
	git := filepath.Join(t.TempDir(), ".git")
	for rel, text := range map[string]string{
		"config": `[core]
	repositoryformatversion = 0
	fsmonitor = standin-fsmonitor
	hooksPath = .husky/_
[user]
	name = Owner
[remote "origin"]
	url = https://example.invalid/r.git
	uploadpack = standin-upload
[filter "lfs"]
	clean = standin-clean %f
	required = true
[alias]
	st = status
[include]
	path = one.cfg
	path = two.cfg
	path = oneXcfg
[submodule "s"]
	update = !standin-update
	path = s
[submodule "t"]
	update = checkout
[protocol "file"]
	allow = always
[protocol "ext"]
	allow = never
[protocol]
	allow = user
`,
		"modules/sub/HEAD": "1111\n", "modules/sub/config": "[core]\n\tfsmonitor = true\n\tsshCommand = standin-ssh\n",
		"modules/a/b/HEAD": "1111\n", "modules/a/b/config": "[core]\n\tpager = standin-pager\n",
		"modules/a/b/modules/c/HEAD": "1111\n", "modules/a/b/modules/c/config": "[diff]\n\texternal = standin-diff\n",
		"worktrees/w1/config.worktree": "[core]\n\teditor = standin-editor\n",
		"hooks/pre-commit.sample":      "#!/bin/sh\n", "hooks/pre-commit": "#!/bin/sh\necho standin\n", "hooks/sub/x": "",
		"modules/sub/hooks/pre-push": "#!/bin/sh\n", "modules/sub/hooks/pre-push.sample": "#!/bin/sh\n",
	} {
		at := filepath.Join(git, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(git, "hooks", "pre-commit"), filepath.Join(git, "hooks", "post-checkout")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("this Windows account can't make a link")
		}
		t.Fatal(err)
	}
	return git
}

func plantLines(git string, plants []oldGitPlant) []string {
	var lines []string
	for _, p := range plants {
		rel, _ := filepath.Rel(git, p.File)
		lines = append(lines, filepath.ToSlash(rel)+" "+p.Key+"="+p.Value+": "+p.Why)
	}
	slices.Sort(lines)
	return lines
}

func TestOldGitPlantsAreListedAndNothingIsRemoved(t *testing.T) {
	git := gitPlantTree(t)
	sums := treeSums(t, filepath.Dir(git))
	got := plantLines(git, findOldGitPlants(context.Background(), git))
	want := []string{
		"config alias.st=status: " + whyProgram,
		"config core.fsmonitor=standin-fsmonitor: " + whyProgram,
		"config core.hookspath=.husky/_: " + whyHooks,
		"config filter.lfs.clean=standin-clean %f: " + whyProgram,
		"config include.path=one.cfg: " + whyInclude,
		"config include.path=oneXcfg: " + whyInclude,
		"config include.path=two.cfg: " + whyInclude,
		"config protocol.allow=user: " + whyTransport,
		"config protocol.file.allow=always: " + whyTransport,
		"config remote.origin.uploadpack=standin-upload: " + whyUnknown,
		"config submodule.s.update=!standin-update: " + whyProgram,
		"hooks/post-checkout =: " + whyHook,
		"hooks/pre-commit =: " + whyHook,
		"modules/a/b/config core.pager=standin-pager: " + whyProgram,
		"modules/a/b/modules/c/config diff.external=standin-diff: " + whyProgram,
		"modules/sub/config core.sshcommand=standin-ssh: " + whyProgram,
		"modules/sub/hooks/pre-push =: " + whyHook,
		"worktrees/w1/config.worktree core.editor=standin-editor: " + whyProgram,
	}
	if !slices.Equal(got, want) {
		t.Errorf("listed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if now := treeSums(t, filepath.Dir(git)); !slices.Equal(now, sums) {
		t.Error("listing the plants changed a file")
	}
}

// TestASubmoduleFolderThatCantBeListedIsListedAsSuch: a folder under modules/ it can't list is listed as unread, not
// passed over (RT126SAX12's nit 1).
func TestASubmoduleFolderThatCantBeListedIsListedAsSuch(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("no folder modes to shut here")
	}
	git := gitPlantTree(t)
	shut := filepath.Join(git, "modules", "a")
	if err := os.Chmod(shut, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(shut, 0o700)
	plants := findOldGitPlants(context.Background(), git)
	if !slices.ContainsFunc(plants, func(p oldGitPlant) bool { return p.File == shut && p.Why == whyUnread && p.Key == "" }) {
		t.Errorf("listed:\n%s\nwant modules/a as unread", strings.Join(plantLines(git, plants), "\n"))
	}
}

func TestAConfigThatCantBeReadIsListedAsSuch(t *testing.T) {
	git := gitPlantTree(t)
	if err := os.WriteFile(filepath.Join(git, "config"), []byte("[core\n\tbroken"), 0o600); err != nil {
		t.Fatal(err)
	}
	plants := findOldGitPlants(context.Background(), git)
	i := slices.IndexFunc(plants, func(p oldGitPlant) bool { return p.File == filepath.Join(git, "config") })
	if i < 0 || plants[i].Why != whyUnread || plants[i].Key != "" {
		t.Fatalf("the broken config: %+v", plants)
	}
	if err := removeOldGitPlant(context.Background(), plants, plants[i].ID); err == nil {
		t.Error("an unread config was removed from")
	}
}

func TestRemoveTakesOutOnlyWhatWasListedAsItWas(t *testing.T) {
	ctx := context.Background()
	git := gitPlantTree(t)
	plants := findOldGitPlants(ctx, git)
	find := func(file, key, value string) oldGitPlant {
		t.Helper()
		i := slices.IndexFunc(plants, func(p oldGitPlant) bool {
			return p.File == filepath.Join(git, filepath.FromSlash(file)) && p.Key == key && p.Value == value
		})
		if i < 0 {
			t.Fatalf("%s %s=%s isn't listed", file, key, value)
		}
		return plants[i]
	}
	settings := func() []string {
		t.Helper()
		kv, err := readConfigFile(ctx, filepath.Join(git, "config"))
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, s := range kv {
			lines = append(lines, s[0]+"="+s[1])
		}
		return lines
	}

	// A hook: removed once, as listed; a link goes, not what it leads to.
	hook := find("hooks/post-checkout", "", "")
	if err := removeOldGitPlant(ctx, plants, hook.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(hook.File); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the hook is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(git, "hooks", "pre-commit")); err != nil {
		t.Errorf("what the link led to went too: %v", err)
	}
	if err := removeOldGitPlant(ctx, plants, hook.ID); !errors.Is(err, errGitPlantGone) {
		t.Errorf("a second remove: %v", err)
	}
	// A hook changed since it was listed stays.
	changed := find("hooks/pre-commit", "", "")
	if err := os.WriteFile(changed.File, []byte("#!/bin/sh\necho another standin\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := removeOldGitPlant(ctx, plants, changed.ID); !errors.Is(err, errGitPlantGone) {
		t.Errorf("a changed hook: %v", err)
	}
	if _, err := os.Stat(changed.File); err != nil {
		t.Errorf("the changed hook was removed: %v", err)
	}

	// A setting: that value only, the rest of the file as it was ("one.cfg" doesn't take "oneXcfg" with it), with
	// no --fixed-value, which an older git hasn't got.
	before := settings()
	if err := removeOldGitPlant(ctx, plants, find("config", "include.path", "one.cfg").ID); err != nil {
		t.Fatal(err)
	}
	if err := removeOldGitPlant(ctx, plants, find("config", "core.fsmonitor", "standin-fsmonitor").ID); err != nil {
		t.Fatal(err)
	}
	if err := removeOldGitPlant(ctx, plants, find("config", "protocol.file.allow", "always").ID); err != nil {
		t.Fatal(err)
	}
	want := slices.DeleteFunc(slices.Clone(before), func(s string) bool {
		return s == "include.path=one.cfg" || s == "core.fsmonitor=standin-fsmonitor" || s == "protocol.file.allow=always"
	})
	if got := settings(); !slices.Equal(got, want) {
		t.Errorf("after the removes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// A setting changed since it was listed, and an id never listed, stay.
	if err := os.WriteFile(filepath.Join(git, "modules", "a", "b", "config"), []byte("[core]\n\tpager = another-standin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeOldGitPlant(ctx, plants, find("modules/a/b/config", "core.pager", "standin-pager").ID); !errors.Is(err, errGitPlantGone) {
		t.Errorf("a changed setting: %v", err)
	}
	if err := removeOldGitPlant(ctx, plants, "0123456789abcdef"); !errors.Is(err, errGitPlantGone) {
		t.Errorf("an id never listed: %v", err)
	}
}

func TestGitSettingRunsSaysWhichSettingsRunAProgram(t *testing.T) {
	for _, c := range []struct{ key, value, want string }{
		{"core.fsmonitor", "true", ""}, {"core.fsmonitor", "false", ""}, {"core.fsmonitor", "", ""}, {"core.fsmonitor", "./x", whyProgram},
		{"core.hookspath", os.DevNull, ""}, {"core.hookspath", "hooks2", whyHooks}, {"core.attributesfile", "a", ""},
		{"protocol.allow", "always", whyTransport}, {"core.sshcommand", "ssh -i k", whyProgram}, {"credential.helper", "store", whyProgram},
		{"gpg.program", "g", whyProgram}, {"sequence.editor", "e", whyProgram}, {"diff.external", "d", whyProgram},
		{"filter.x.smudge", "s", whyProgram}, {"filter.x.process", "", ""}, {"filter.x.required", "true", ""},
		{"diff.x.textconv", "t", whyProgram}, {"merge.x.driver", "m", whyProgram}, {"credential.https://h.helper", "c", whyProgram},
		{"gpg.ssh.program", "p", whyProgram}, {"pager.log", "less", whyProgram}, {"protocol.ext.allow", "always", whyTransport},
		{"submodule.s.update", "rebase", ""}, {"submodule.s.update", "!x", whyProgram}, {"includeif.gitdir:~/w/.path", "w.cfg", whyInclude},
		{"user.email", "o@example.invalid", ""}, {"remote.origin.receivepack", "r", whyUnknown}, {"core.worktree", "../w", ""},
		{"lfs.url", "u", ""}, {"nosection", "x", whyUnknown},
		// Transports (RT126SAX12's B3): only "never" lets none through.
		{"protocol.allow", "never", ""}, {"protocol.allow", "user", whyTransport}, {"protocol.allow", "", whyTransport},
		{"protocol.ext.allow", "never", ""}, {"protocol.ext.allow", "user", whyTransport}, {"protocol.ext.allow", "", whyTransport},
		{"protocol.file.allow", "always", whyTransport}, {"protocol.ext.allow", "Never", whyTransport},
		// A program in a data section (RT126SAX7's B2): gitstate blanks it, so it is listed.
		{"gc.recentobjectshook", "/stand-in", whyProgram}, {"gc.auto", "1", ""},
	} {
		if got := gitSettingRuns(c.key, c.value); got != c.want {
			t.Errorf("%s = %q: %q, want %q", c.key, c.value, got, c.want)
		}
	}
}

// TestGitSettingRunsIsNoLooserThanGitRepoConfigOnTransports: a transport setting gitstate's reads blank or force
// (gitRepoConfig, gitReadArgs) is listed whatever its value but "never" (RT126SAX12's B3 probe).
func TestGitSettingRunsIsNoLooserThanGitRepoConfigOnTransports(t *testing.T) {
	for _, key := range []string{"protocol.allow", "protocol.ext.allow", "protocol.file.allow", "protocol.foo.allow"} {
		for _, value := range []string{"always", "user", "never", "", "NEVER", "x"} {
			blanks, refused := gitRepoConfig([]byte(key + "\n" + value + "\x00"))
			blanked := refused != "" || len(blanks) > 0 && blanks[0][1] != value || gitFixedKeys[key] && value != "never"
			if got := gitSettingRuns(key, value); blanked && got == "" {
				t.Errorf("%s = %q: gitstate blanks it (%v, %q), but it isn't listed", key, value, blanks, refused)
			}
		}
	}
}
