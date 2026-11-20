package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXSWEEP3P1 (RT126SWEEP3 P1's B1 and F1), with stand-ins: the core holds zsh's files where ZDOTDIR puts them, and a
// desk's start and stop refuse while something a desk could have mounted is at or inside a narrow place its sandbox
// holds (FXSWEEP3P1R1).

// B1: ZDOTDIR in the core's environment is where a desk's Policy holds zsh's start files, resolved and as spelled;
// none when it's the home folder, not a whole path, or off a Mac.
func TestTheCoreHoldsZshsFilesWhereZDOTDIRPutsThem(t *testing.T) {
	homeDir = t.TempDir()
	env := map[string]string{}
	getenv := func(v string) string { return env[v] }
	if got := deskShellHomes("darwin", getenv); got != nil {
		t.Errorf("no ZDOTDIR: %q", got)
	}
	zdot := filepath.Join(homeDir, ".config", "zsh")
	env["ZDOTDIR"] = zdot
	if got := deskShellHomes("darwin", getenv); !slices.Contains(got, physicalBelow(zdot)) {
		t.Errorf("ZDOTDIR=%s: %q", zdot, got)
	}
	for _, value := range []string{homeDir, homeDir + "/", "zsh", ""} {
		env["ZDOTDIR"] = value
		if got := deskShellHomes("darwin", getenv); got != nil {
			t.Errorf("ZDOTDIR=%q: %q", value, got)
		}
	}
	env["ZDOTDIR"] = zdot
	if got := deskShellHomes("linux", getenv); got != nil {
		t.Errorf("off a Mac: %q", got)
	}
	// …and a desk's start reads it: its Policy holds the start files there.
	t.Setenv("ZDOTDIR", zdot)
	s := standInDesk(t)
	if !slices.Contains(s.places.ShellHomes, physicalBelow(zdot)) {
		t.Fatalf("the start's ShellHomes: %q", s.places.ShellHomes)
	}
	for _, name := range []string{".zshrc", ".zshenv.zwc", ".zcompdump", ".zsh_sessions/A1.session"} {
		if v, r := s.policy.Decide(filepath.Join(physicalBelow(zdot), name), sandbox.Write); v != sandbox.Deny {
			t.Errorf("%s in ZDOTDIR: %v (%q)", name, v, r.Row)
		}
	}
}

// F1, narrowed (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B1), on a desk in a stand-in repo in the home folder: the owner's
// ordinary mounts, the reviewer's cases, let every desk start and stop (so a fire goes on), and a type a desk can't make
// over a narrow place (an image at the hooks: the accepted residual) does too. devfs over the repo's hooks and over
// LaunchAgents refuses the start, and stops the stop there, its sandbox kept, in words that say what is mounted where
// and how to take it away; with it gone, both go on. So does a share at ~/Library or at zsh's folder in ~/.config, which
// hold plants themselves, while devfs there, and any mount at the home folder, the projects folder or the checkout, lets
// both go on (FXSWEEP3P1R2). A mount table that can't be read stops them too.
func TestADesksStartAndStopRefuseWhileAMountHidesAHeldPlace(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	saved := deskMounts
	t.Cleanup(func() { deskMounts = saved })
	var mounts []sandbox.Mount
	var failed error
	deskMounts = func() ([]sandbox.Mount, error) { return mounts, failed }
	freshGitPlants(t)
	shortSupport(t)
	freshGitKept(t)
	homeDir = t.TempDir()
	deskSockets(t)
	t.Setenv("PATH", filepath.Dir(git)+":"+os.Getenv("PATH"))
	home := physical(homeDir)
	repo := filepath.Join(home, "Work", "app")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, repo, "init", "-q", "-b", "main")
	writeRepoFile(t, repo, "f", "one\n")
	gitRepoCmd(t, repo, "add", "f")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "A")
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := func() error {
		_, err := makeDeskSandbox(deskAt{ID: "ab12", Root: repo, Socket: deskSocketPath("ab12", 0), Git: runGit})
		return err
	}
	h := Hire{ID: "ab12"}
	hooks := filepath.Join(repo, ".git", "hooks")
	agents := filepath.Join(home, "Library", "LaunchAgents")

	// The owner's own mounts: each desk starts, and stops with nothing kept.
	for _, c := range []struct {
		what string
		m    sandbox.Mount
	}{
		{"a FUSE mount in the projects folder", sandbox.Mount{At: filepath.Join(home, "Projects", "data"), Type: "macfuse"}},
		{"the same through FUSE-T, which mounts NFS", sandbox.Mount{At: filepath.Join(home, "Projects", "data"), Type: "nfs"}},
		{"the projects folder, an image", sandbox.Mount{At: filepath.Join(home, "Projects"), Type: "apfs"}},
		{"a network home", sandbox.Mount{At: home, Type: "smbfs"}},
		{"a network home over NFS", sandbox.Mount{At: home, Type: "nfs"}},
		{"a volume in the home folder holding the repo", sandbox.Mount{At: filepath.Join(home, "Work"), Type: "apfs"}},
		{"a share in the home folder holding the repo", sandbox.Mount{At: filepath.Join(home, "Work"), Type: "smbfs"}},
		{"an image at the repo's hooks: a type no desk can mount, the accepted residual", sandbox.Mount{At: hooks, Type: "apfs"}},
		// FXSWEEP3P1R2 (RT126SWEEP3FIXP1R1's N1): devfs above a plant only hides it, and a share at the home folder, the
		// projects folder or the checkout is the owner's.
		{"devfs at ~/Library, above LaunchAgents", sandbox.Mount{At: filepath.Join(home, "Library"), Type: "devfs"}},
		{"devfs at the home folder", sandbox.Mount{At: home, Type: "devfs"}},
		{"the projects folder, a share", sandbox.Mount{At: filepath.Join(home, "Projects"), Type: "nfs"}},
		{"the checkout, a share", sandbox.Mount{At: repo, Type: "nfs"}},
		{"the checkout, devfs", sandbox.Mount{At: repo, Type: "devfs"}},
		{"the folder holding the checkout, devfs", sandbox.Mount{At: filepath.Join(home, "Work"), Type: "devfs"}},
	} {
		mounts = []sandbox.Mount{c.m}
		if err := start(); err != nil {
			t.Errorf("%s: the start: %v", c.what, err)
			continue
		}
		if why := deskStopped(h, engineOptions{}); why != "" {
			t.Errorf("%s: the stop: %q", c.what, why)
		}
		if _, kept := deskSandboxOf("ab12"); kept {
			t.Errorf("%s: the stop kept the desk's sandbox", c.what)
		}
	}

	// devfs over a narrow place: the repo's hooks, and LaunchAgents.
	for _, at := range []string{hooks, agents} {
		mounts = []sandbox.Mount{{At: at, Type: "devfs"}}
		begins := mountedBegins + "is mounted at " + at + " (devfs), where no desk may change anything (unmount it with umount " +
			shellQuoted(at) + " in Terminal)"
		// The start.
		var refused *errSandboxNotMade
		if err := start(); !errors.As(err, &refused) || refused.Error() != begins+", so no desk starts while it's there." {
			t.Errorf("%s: the start: %v", at, err)
		}
		// The stop: kept, then let go once it's gone.
		mounts = nil
		if err := start(); err != nil {
			t.Fatalf("%s: with nothing mounted, the start: %v", at, err)
		}
		mounts = []sandbox.Mount{{At: at, Type: "devfs"}}
		why := deskStopped(h, engineOptions{})
		if why != begins || strings.Contains(why, "yourself") {
			t.Errorf("%s: the stop's words: %q", at, why)
		}
		if _, kept := deskSandboxOf("ab12"); !kept {
			t.Errorf("%s: the stop let the desk's sandbox go", at)
		}
		if fired := firedRefusal(h, why); fired != why+firedReapRefusal {
			t.Errorf("%s: the fire's words: %q", at, fired)
		}
		mounts = nil
		if why := deskStopped(h, engineOptions{}); why != "" {
			t.Errorf("%s: with the mount gone, the stop: %q", at, why)
		}
		if _, kept := deskSandboxOf("ab12"); kept {
			t.Errorf("%s: with the mount gone, the stop kept the desk's sandbox", at)
		}
	}

	// FXSWEEP3P1R2 (RT126SWEEP3FIXP1R1's N1 and O1): a share at a folder that holds a plant itself, ~/Library and zsh's
	// folder in ~/.config, refuses the start and stops the stop, in words that say what it's over; devfs inside the
	// repo's hooks says "inside".
	for _, m := range []sandbox.Mount{
		{At: filepath.Join(home, "Library"), Type: "nfs"},
		{At: filepath.Join(home, ".config", "zsh"), Type: "nfs"},
		{At: filepath.Join(hooks, "x"), Type: "devfs"},
	} {
		mounts = nil
		if err := start(); err != nil {
			t.Fatalf("%s: with nothing mounted, the start: %v", m.At, err)
		}
		s, _ := deskSandboxOf("ab12")
		_, place := s.policy.MountedOver(home, []sandbox.Mount{m})
		where := ", over " + place
		if m.Type != "nfs" {
			where = ", inside " + place
			if !strings.HasPrefix(m.At, place+"/") {
				t.Errorf("%s: inside %q", m.At, place)
			}
		} else if filepath.Dir(place) != m.At {
			t.Errorf("%s: over %q", m.At, place)
		}
		begins := mountedBegins + "is mounted at " + m.At + " (" + m.Type + ")" + where +
			", where no desk may change anything (unmount it with umount " + shellQuoted(m.At) + " in Terminal)"
		mounts = []sandbox.Mount{m}
		if why := deskStopped(h, engineOptions{}); why != begins {
			t.Errorf("%s: the stop's words: %q, want %q", m.At, why, begins)
		}
		if _, kept := deskSandboxOf("ab12"); !kept {
			t.Errorf("%s: the stop let the desk's sandbox go", m.At)
		}
		var refused *errSandboxNotMade
		if err := start(); !errors.As(err, &refused) || refused.Error() != begins+", so no desk starts while it's there." {
			t.Errorf("%s: the start: %v", m.At, err)
		}
		mounts = nil
		if why := deskStopped(h, engineOptions{}); why != "" {
			t.Errorf("%s: with the mount gone, the stop: %q", m.At, why)
		}
	}

	// A mount table that can't be read.
	if err := start(); err != nil {
		t.Fatal(err)
	}
	failed = errors.New("stand-in")
	if why := deskStopped(h, engineOptions{}); !strings.HasPrefix(why, mountedBegins+"may be mounted") {
		t.Errorf("a mount table that can't be read: %q", why)
	}
	failed = nil
	if why := deskStopped(h, engineOptions{}); why != "" {
		t.Errorf("read again, the stop: %q", why)
	}
}
