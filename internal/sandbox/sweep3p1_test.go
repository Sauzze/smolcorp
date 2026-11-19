package sandbox

import (
	"strings"
	"testing"
)

// FXSWEEP3P1's rows on the made-up Mac (RT126SWEEP3 P1's B1, B2, F1 and F3, P2's B2 (b)), whatever computer this runs
// on: Decide's answer for each place the reviewer found open, the places beside them that stay the desk's own, and the
// mount table's check. The real profile's rows are sweep3p1_darwin_test.go's.

// B1, B2, the start-file list and the VM tools' keys, as Decide says them, in the home folder, where ZDOTDIR puts zsh's
// files, and in zsh's folder in ~/.config.
func TestOnAMacZshBashAndTerminalStartFilesAndTheVMKeysAreHeld(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.RemoteLogin = false // ~/.ssh open: the Podman key is held by its own row
	zdot := home + "/.zsh"
	pl.ShellHomes, pl.ConfigHomes = []string{zdot}, []string{home + "/.xdg"}
	p := mustNew(t, pl)
	for _, c := range []struct {
		at   string
		op   Op
		want Verdict
	}{
		{home + "/.zshrc.zwc", Write, Deny}, {home + "/.zshenv.zwc", Write, Deny}, {home + "/.zprofile.zwc", Write, Deny},
		{home + "/.zlogin.zwc", Write, Deny}, {home + "/.zlogout.zwc", Write, Deny}, {home + "/.ZSHRC.ZWC", Write, Deny},
		{home + "/.zcompdump", Write, Deny}, {home + "/.zcompdump-mac-5.9", Write, Deny}, {home + "/.zcompdump.zwc", Write, Deny},
		{home + "/.zcompdump.mac.4242", Write, Deny}, {home + "/.zkbd/xterm-apple", Write, Deny},
		{home + "/.zsh_sessions/A1B2.session", Write, Deny}, {home + "/.zsh_sessions/A1B2.history", Write, Deny},
		{home + "/.bash_sessions/A1B2.session", Write, Deny}, {home + "/.inputrc", Write, Deny}, {home + "/.inputrc.lock", Write, Deny},
		{home + "/Library/Saved Application State/com.apple.Terminal.savedState/windows.plist", Write, Deny},
		// Where ZDOTDIR puts them, and zsh's folder in ~/.config and where XDG_CONFIG_HOME moves that.
		{zdot + "/.zshrc", Write, Deny}, {zdot + "/.zshenv.zwc", Write, Deny}, {zdot + "/.zshrc.lock", Write, Deny},
		{zdot + "/.zshrc-omztemp", Write, Deny}, {zdot + "/.zcompdump", Write, Deny}, {zdot + "/.zsh_sessions/A1B2.session", Write, Deny},
		{zdot + "/.zkbd/xterm-apple", Write, Deny},
		{home + "/.config/zsh/.zshrc", Write, Deny}, {home + "/.config/zsh/.zprofile.zwc", Write, Deny},
		{home + "/.xdg/zsh/.zlogin", Write, Deny},
		// The VM tools' keys.
		{home + "/.lima/_config/user", Read, Deny}, {home + "/.lima/_config/user", Write, Deny},
		{home + "/.colima/_lima/_config/user", Read, Deny}, {home + "/.orbstack/ssh/id_ed25519", Read, Deny},
		{home + "/.local/share/containers/podman/machine/machine", Read, Deny}, {home + "/.ssh/podman-machine-default", Read, Deny},
		{home + "/.ssh/podman-machine-default.pub", Read, Deny},
		{home + "/Library/Application Support/rancher-desktop/lima/_config/user", Read, Deny}, // N2
		// What most owners' ~/.zshrc sources (FXSWEEP3P1R1): oh-my-zsh, whole, and Powerlevel10k's settings.
		{home + "/.oh-my-zsh/oh-my-zsh.sh", Write, Deny}, {home + "/.oh-my-zsh/cache/completions/_git", Write, Deny},
		{home + "/.oh-my-zsh/custom/plugins/x/x.plugin.zsh", Write, Deny}, {home + "/.p10k.zsh", Write, Deny},
		{home + "/.p10k.zsh.lock", Write, Deny}, {zdot + "/.p10k.zsh", Write, Deny}, {home + "/.config/zsh/.p10k.zsh", Write, Deny},
		// …and oh-my-zsh where its installer puts it when ZDOTDIR is set, in each zsh folder but the home folder (N3).
		{zdot + "/ohmyzsh/oh-my-zsh.sh", Write, Deny}, {zdot + "/ohmyzsh/custom/plugins/x", Write, Deny},
		{home + "/.config/zsh/ohmyzsh/oh-my-zsh.sh", Write, Deny}, {home + "/.xdg/zsh/ohmyzsh/cache/completions/_git", Write, Deny},
		// What stays the desk's: its shells' history, a neighbour's name, the rest of zsh's and Lima's folders, a read.
		{home + "/.zsh_history", Write, Allow}, {home + "/.zshrc.zwc-notes", Write, Allow}, {zdot + "/.zsh_history", Write, Allow},
		{home + "/.config/zsh/notes.md", Write, Allow}, {home + "/.lima/default/lima.yaml", Read, Allow},
		{home + "/.ssh/known_hosts", Read, Allow}, {home + "/.zsh_sessions/A1B2.history", Read, Allow},
		{home + "/Library/Saved Application State/com.example.savedState/windows.plist", Write, Allow},
		{home + "/.oh-my-zsh/oh-my-zsh.sh", Read, Allow}, {home + "/.oh-my-zsh-notes", Write, Allow},
		{zdot + "/ohmyzsh/oh-my-zsh.sh", Read, Allow}, {zdot + "/ohmyzsh-notes", Write, Allow}, {home + "/ohmyzsh/notes", Write, Allow},
		{home + "/.cache/p10k-instant-prompt-ada.zsh", Write, Allow}, // a known limit
		{home + "/Library/Application Support/rancher-desktop/settings.json", Read, Allow},
	} {
		if got, r := p.Decide(c.at, c.op); got != c.want {
			t.Errorf("%s, %v: %v (%q), want %v", c.at, c.op, got, r.Row, c.want)
		}
	}
	// Off a Mac, nothing new: Linux's view and Windows' jail come with their sandboxes.
	linux := pl
	linux.GOOS = "linux"
	for _, r := range mustNew(t, linux).Rules() {
		if r.Row == rowShellStart || r.Row == rowShellSessions || r.Row == rowVMKeys || strings.HasSuffix(r.Path, ".zwc") {
			t.Errorf("Linux's Policy has %q at %s", r.Row, r.Path)
		}
	}
}

// F3: the GM's own plans in the projects folder take no git folder, at any depth, in any case, nor a HEAD at their top;
// the plans themselves stay its own.
func TestTheGMsOwnPlansTakeNoGitFolder(t *testing.T) {
	p := mustNew(t, projectsGMPlaces())
	for _, c := range []struct {
		at   string
		want Verdict
	}{
		{gmPlans + "/.git", Deny}, {gmPlans + "/.git/config", Deny}, {gmPlans + "/.GIT/hooks/pre-commit", Deny},
		{gmPlans + "/sub/.git", Deny}, {gmPlans + "/sub/deeper/.git/HEAD", Deny}, {gmPlans + "/HEAD", Deny},
		{gmPlans + "/calm-river.md", Allow}, {gmPlans + "/.github/x.md", Allow}, {gmPlans + "/.gitignore", Allow},
		{gmPlans + "/sub/HEAD.md", Allow},
	} {
		if got, r := p.Decide(c.at, Write); got != c.want {
			t.Errorf("%s: %v (%q), want %v", c.at, got, r.Row, c.want)
		}
	}
}

// F1's check, narrowed (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B1): only a file system a desk could have mounted under an
// earlier profile (DeskMountTypes) counts, and only at or inside a narrow place: git's folders, the plants, the support
// folder, the sockets. The owner's ordinary mounts, the reviewer's cases, are never found: a FUSE or FUSE-T mount in
// the projects folder, the projects folder itself as an image or a network share, a network home, a volume or a share
// in the home folder holding the repo. A type outside the list over a narrow place (an image mounted at .git/hooks) isn't
// found either: that's the residual this accepts, since a desk can't make one (no DiskArbitration, no FUSE helper).
// A network mount (nfs, smbfs, afpfs, webdav, ftp) at a folder in the home folder that holds a plant itself is found
// too, since what it shows a desk chose (FXSWEEP3P1R2, RT126SWEEP3FIXP1R1's N1): `~/Library`, a zsh folder, `~/.ssh`;
// never at `~`, `~/Projects`, a checkout or the folder holding the desks' places, and never devfs or fdesc there.
func TestAMountOverAHeldPlaceIsFound(t *testing.T) {
	p := mustNew(t, hirePlaces(realSupport))
	for _, c := range []struct {
		at, kind string
		found    bool
	}{
		// Found: what a desk could mount, at or inside a narrow place.
		{gitDir + "/hooks", "devfs", true},                  // the reviewer's: the shared .git's hooks
		{gitDir + "/hooks", "nfs", true},                    // any type on the list
		{gitDir, "devfs", true},                             // what makes it a git folder
		{gitDir + "/refs/heads", "fdesc", true},             // the base branches
		{gitDir + "/worktrees/h2", "smbfs", true},           // another desk's registration
		{home + "/.config/fish", "webdav", true},            // a plant folder
		{home + "/Library/LaunchAgents", "devfs", true},     // the plants launchd runs
		{home + "/Library/LaunchAgents/sub", "afpfs", true}, // inside one
		{home + "/.zsh_sessions", "ftp", true},              // Terminal's restored sessions
		{home + "/.zcompdump-mac-5.9", "devfs", true},       // what a pattern names
		{realSupport + "/hires", "devfs", true},             // inside the support folder
		{sockets, "devfs", true},                            // the socket folder
		{home + "/.codex", "devfs", true},                   // an agent's home
		{home + "/.claude", "devfs", true},                  // the account folder
		{hires + "/h1/vendor/lib/.git", "devfs", true},      // a git folder below its place
		{home + "/.ssh", "devfs", true},                     // held whole while Remote Login answers (N2)
		// Found (FXSWEEP3P1R2, RT126SWEEP3FIXP1R1's N1): a share at a folder that holds a plant itself, inside the home
		// folder, whatever files it shows a desk chose.
		{home + "/Library", "nfs", true},                       // LaunchAgents, Preferences, Keychains
		{home + "/.config/zsh", "nfs", true},                   // zsh's start files
		{home + "/.ssh", "smbfs", true},                        // its config
		{home + "/.config", "webdav", true},                    // fish's, git's, tmux's folders
		{home + "/Library/Application Support", "afpfs", true}, // iTerm2's, Ghostty's
		{home + "/dotfiles", "ftp", true},                      // a hooks folder git's config names
		// Not found: the reviewer's cases, as their types and as types on the list.
		{home + "/Projects/data", "macfuse", false},        // sshfs, rclone, Cryptomator in the projects folder
		{home + "/Projects/data", "nfs", false},            // …through FUSE-T, which mounts NFS
		{home + "/Projects", "apfs", false},                // the projects folder, an encrypted or case-sensitive image
		{home + "/Projects", "smbfs", false},               // …or a network share
		{home, "smbfs", false},                             // a network home
		{home, "nfs", false},                               // …over NFS
		{home, "devfs", false},                             // the home folder itself, whatever the type
		{repo, "apfs", false},                              // a volume holding the repo
		{repo, "smbfs", false},                             // …or a share
		{repo + "/vendor/img", "apfs", false},              // an image inside the checkout
		{repo + "/vendor/img", "nfs", false},               // …or a share: the checkout is a broad row
		{hires + "/h2", "nfs", false},                      // another desk's place: a broad row
		{home + "/Library", "devfs", false},                // devfs above a narrow place only hides it: never counted
		{home + "/.config/zsh", "fdesc", false},            // …nor fdesc
		{home + "/Projects", "nfs", false},                 // a share as the projects folder, holding a git folder above the places
		{repo, "nfs", false},                               // a share as the checkout, holding its .git
		{repo, "devfs", false},                             // …or devfs
		{hires, "nfs", false},                              // the folder holding the desks' places
		{home + "/Music", "nfs", false},                    // a share at a folder holding no plant
		{home + "/.ssh/sub", "devfs", true},                // inside ~/.ssh
		{home + "/mnt/nas", "nfs", false},                  // the owner's own, holding nothing held
		{home + "/Music", "devfs", false},                  // nor here
		{"/Volumes/Work", "apfs", false},                   // a volume, outside the home folder
		{"/Volumes/Work/app/.git/hooks", "devfs", false},   // a repo there that isn't the desk's
		{gitDir + "/hooks", "apfs", false},                 // an image at the hooks: the accepted residual
		{home + "/Library/LaunchAgents", "macfuse", false}, // a FUSE mount over a plant: the same
	} {
		mount, place := p.MountedOver(home, []Mount{{At: c.at, Type: c.kind}})
		if (mount != "") != c.found || (mount != "" && (mount != c.at || place == "")) {
			t.Errorf("%s (%s): found %q over %q, want found %v", c.at, c.kind, mount, place, c.found)
		}
	}
	// A share over a plant names the plant, in the folder it's mounted at.
	for _, at := range []string{home + "/Library", home + "/.config/zsh", home + "/.config"} {
		if mount, place := p.MountedOver(home, []Mount{{At: at, Type: "nfs"}}); mount != at || !strings.HasPrefix(place, at+"/") {
			t.Errorf("a share at %s: found %q over %q", at, mount, place)
		}
	}
	if mount, _ := p.MountedOver(home, []Mount{{home + "/mnt/nas", "nfs"}, {gitDir + "/hooks", "devfs"}}); mount != gitDir+"/hooks" {
		t.Errorf("the second of two mounts: %q", mount)
	}
	if mount, _ := p.MountedOver(home, nil); mount != "" {
		t.Errorf("no mounts: %q", mount)
	}
	// The GM in the projects folder: its projects and its own plans are broad, the git folder in its plans narrow.
	gm := mustNew(t, projectsGMPlaces())
	for _, c := range []struct {
		at    string
		found bool
	}{{home + "/Projects", false}, {home + "/Projects/data", false}, {gmPlans, false}, {gmPlans + "/sub/.git", true},
		{home + "/Library", true}} { // a share over LaunchAgents: its workspace in the support folder isn't a work place

		if mount, place := gm.MountedOver(home, []Mount{{At: c.at, Type: "nfs"}}); (mount != "") != c.found {
			t.Errorf("the GM's, %s: found %q over %q, want found %v", c.at, mount, place, c.found)
		}
	}
}

// RT126SWEEP3FIXP1R2 N2: with the projects folder in a cloud folder inside ~/Library (Dropbox's, iCloud Drive's), a
// share at ~/Library is above the places the owner works in, and is still found over the plants it holds directly
// (LaunchAgents, Preferences, Keychains): only the git folders above the places aren't counted there. A share at the
// cloud folders themselves, above the places and holding no other plant, and at or inside the places, isn't found.
// Mutant MA's case stays: a share at ~/Projects, holding ~/Projects/.git above the places, isn't found.
func TestAShareAboveAProjectsFolderInLibraryIsFoundOverItsPlants(t *testing.T) {
	for _, projects := range []string{home + "/Library/CloudStorage/Dropbox/Projects",
		home + "/Library/Mobile Documents/com~apple~CloudDocs/Projects"} {
		move := func(s string) string { return strings.Replace(s, home+"/Projects", projects, 1) }
		pl := hirePlaces(realSupport)
		pl.Root, pl.Checkout, pl.GitDir, pl.HiresFolder, pl.GMFolder = move(pl.Root), move(pl.Checkout), move(pl.GitDir),
			move(pl.HiresFolder), move(pl.GMFolder)
		pl.GitHooks = append([]string(nil), pl.GitHooks...)
		for i := range pl.GitHooks {
			pl.GitHooks[i] = move(pl.GitHooks[i])
		}
		pl.Projects = projects
		p := mustNew(t, pl)
		cloud := parent(projects)
		for _, c := range []struct {
			at, kind string
			found    bool
		}{
			{home + "/Library", "smbfs", true},                   // above the projects folder, holding LaunchAgents
			{home + "/Library", "nfs", true},                     // …through FUSE-T too
			{home + "/Library/Application Support", "nfs", true}, // holding no place, as before
			{parent(cloud), "nfs", false},                        // above the places, holding only a git folder above them
			{cloud, "smbfs", false},                              // the folder holding the projects folder: the same
			{projects, "nfs", false},                             // the projects folder itself
			{move(repo), "nfs", false},                           // the checkout
			{move(hires), "smbfs", false},                        // the folder holding the desks' places
			{home + "/Library", "devfs", false},                  // devfs above a narrow place only hides it
		} {
			mount, place := p.MountedOver(home, []Mount{{At: c.at, Type: c.kind}})
			if (mount != "") != c.found || mount != "" && (!strings.HasPrefix(place, c.at+"/") || strings.HasSuffix(place, "/.git")) {
				t.Errorf("projects in %s: %s (%s): found %q over %q, want found %v, over a plant but a git folder", projects,
					c.at, c.kind, mount, place, c.found)
			}
		}
	}
	// MA's case, with the projects folder where it usually is: ~/Projects/.git is above the places, and a share there
	// is the owner's.
	p := mustNew(t, hirePlaces(realSupport))
	for _, kind := range []string{"nfs", "smbfs"} {
		if mount, place := p.MountedOver(home, []Mount{{At: home + "/Projects", Type: kind}}); mount != "" {
			t.Errorf("a share at ~/Projects (%s): found %q over %q", kind, mount, place)
		}
	}
}
