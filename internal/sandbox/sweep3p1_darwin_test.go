package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// FXSWEEP3P1 (RT126SWEEP3 P1's B1, B2, F1 and F3, P2's B1, B2 (b) and F2), on this Mac with stand-ins only, under the
// real profile: what zsh, bash and Terminal read or run at a new or restored window, the VM tools' SSH keys, a git
// folder in the GM's own plans, mounting, debugging and packet capture. Nothing here opens Terminal or an app, touches
// the owner's real home, shells or files, or debugs a process that isn't the row's own stand-in.

// standInSleep is the test program as a stand-in that waits: an unhardened program of the owner's, or of a desk's. It
// says standInSleeping first, so a row knows it's the stand-in and not a nested run of every test here
// (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B2): one started without enterTestVar is refused at once (TestMain).
const (
	standInSleep    = "sleep3p1"
	standInSleeping = "sleep3p1: a stand-in, waiting"
)

func init() {
	moreStandIns[standInSleep] = func([]string) int {
		fmt.Println(standInSleeping)
		time.Sleep(time.Minute)
		return 0
	}
}

// standInEnv is a stand-in's whole environment: this test program as a stand-in, never a nested run of its tests.
func standInEnv() []string {
	return []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", enterTestVar + "=1"}
}

// startSleeping starts program (the test program, or sandbox-exec around it) as standInSleep outside the desk, with
// standInEnv, and waits until it says standInSleeping: a stand-in that doesn't fails the row, at once when TestMain
// refused it (standInRefused), with what it said.
func startSleeping(t *testing.T, base, program string, args ...string) *exec.Cmd {
	t.Helper()
	cmd, stdout, stderr := startOutside(t, base, standInEnv(), program, args...)
	for end := time.Now().Add(20 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		said, _ := os.ReadFile(stdout)
		if strings.Contains(string(said), standInSleeping) {
			return cmd
		}
		if errs, _ := os.ReadFile(stderr); strings.Contains(string(errs), standInRefused) || time.Now().After(end) {
			t.Fatalf("%s %q isn't the waiting stand-in: %q %q", program, args, tail(string(said)), tail(string(errs)))
		}
	}
}

// FXSWEEP3P1R1 (RT126SWEEP3FIXP1's B2): the test program started as a stand-in without enterTestVar ends at once,
// saying so, and runs no test: never a nested run of every test here. The guard the rows' stand-ins lean on.
func TestAStandInStartedWithoutItsVariableEndsAtOnce(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	for _, name := range []string{standInSleep, "standin", "serve"} {
		began := time.Now()
		cmd, stdout, stderr := startOutside(t, base, []string{"PATH=/usr/bin:/bin"}, self, name, filepath.Join(base, "x"))
		ended := make(chan error, 1)
		go func() { ended <- cmd.Wait() }()
		var err error
		select {
		case err = <-ended:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			err = <-ended
		}
		took := time.Since(began)
		said, _ := os.ReadFile(stdout)
		errs, _ := os.ReadFile(stderr)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != standInRefusedStatus || took > 5*time.Second ||
			!strings.Contains(string(errs), standInRefused) || len(said) != 0 {
			t.Errorf("%s without %s: %v after %s, said %q %q", name, enterTestVar, err, took, tail(string(said)), tail(string(errs)))
		}
	}
}

// B1, B2 and the start-file list: refused, and the desk's own shell work and the owner's reads, not.
func TestOnAMacAHireCantPlantWhatZshBashAndTerminalReadAtAWindowsStart(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	zdot := at("zdot") // ZDOTDIR in the core's environment
	d.pl.ShellHomes = []string{zdot}
	d.pl.RemoteLogin = false // so ~/.ssh is open, and the Podman key's own row is what refuses it
	d.prepare(t)
	const id = "A1B2C3D4-0000-4000-8000-000000000001"
	owners := map[string]string{
		at(".zshrc"):                                                           "# the owner's",
		at(".zsh_sessions", id+".history"):                                     "ls\n",
		at(".zkbd", "xterm-apple"):                                             "# the owner's keys",
		at(".config", "zsh", ".zshrc"):                                         "# the owner's",
		filepath.Join(zdot, ".zshenv"):                                         "# the owner's",
		at(".lima", "_config", "user"):                                         escapeKey,
		at(".lima", "default", "lima.yaml"):                                    "# the owner's VM",
		at(".colima", "_lima", "_config", "user"):                              escapeKey,
		at(".orbstack", "ssh", "id_ed25519"):                                   escapeKey,
		at(".local", "share", "containers", "podman", "machine", "machine"):    escapeKey,
		at(".ssh", "podman-machine-default"):                                   escapeKey,
		at(".ssh", "known_hosts"):                                              "stand-in",
		at(".oh-my-zsh", "oh-my-zsh.sh"):                                       "# the owner's oh-my-zsh",
		at(".p10k.zsh"):                                                        "# the owner's prompt",
		at(".cache", "notes"):                                                  "# the owner's",
		at(".oh-my-zsh", "cache", "completions", "_brew"):                      "# the owner's",
		at(".oh-my-zsh", "custom", "plugins", "example", "example.plugin.zsh"): "# the owner's",
		at("Library", "Application Support", "rancher-desktop", "lima", "_config", "user"): escapeKey,
		filepath.Join(zdot, "ohmyzsh", "oh-my-zsh.sh"):                                     "# the owner's oh-my-zsh, beside ZDOTDIR's files",
	}
	for file, text := range owners {
		mustWrite(t, file, text)
	}
	staged := filepath.Join(d.root, "staged")
	mustWrite(t, filepath.Join(staged, id+".session"), "echo a desk's")
	// One file of the desk's for each rename, so each row's answer is its own.
	draft := func(name string) string {
		at := filepath.Join(d.root, "drafts", name)
		mustWrite(t, at, "a desk's")
		return at
	}
	savedState := at("Library", "Saved Application State")
	if err := os.MkdirAll(savedState, 0o700); err != nil {
		t.Fatal(err)
	}

	d.check(t, []escapeRow{
		// The controls: what a desk's own shell writes, its neighbours, and the owner's files read.
		{"control: zsh's history", attempt{"create", at(".zsh_history"), ""}, "ok"},
		{"control: a neighbour of the compiled ~/.zshrc", attempt{"create", at(".zshrc.zwc-notes"), ""}, "ok"},
		{"control: a compiled file of the desk's own", attempt{"create", filepath.Join(d.root, "x.zwc"), ""}, "ok"},
		{"control: a neighbour of ~/.inputrc", attempt{"create", at(".inputrc-notes"), ""}, "ok"},
		{"control: another app's saved state", attempt{"mkdir", filepath.Join(savedState, "com.example.savedState"), ""}, "ok"},
		{"control: a window's history read", attempt{"read", at(".zsh_sessions", id+".history"), ""}, "ok"},
		{"control: Lima's other files read", attempt{"read", at(".lima", "default", "lima.yaml"), ""}, "ok"},
		{"control: ~/.ssh's other files read", attempt{"read", at(".ssh", "known_hosts"), ""}, "ok"},
		{"control: oh-my-zsh read", attempt{"read", at(".oh-my-zsh", "oh-my-zsh.sh"), ""}, "ok"},
		{"control: Powerlevel10k's instant prompt in ~/.cache (a known limit)", attempt{"create", at(".cache", "p10k-instant-prompt-ada.zsh"), ""}, "ok"},

		// B1: zsh's compiled start files, each run in its file's place, in the home folder.
		{"~/.zshrc.zwc", attempt{"create", at(".zshrc.zwc"), ""}, "EPERM"},
		{"~/.zshenv.zwc", attempt{"create", at(".zshenv.zwc"), ""}, "EPERM"},
		{"~/.zprofile.zwc", attempt{"create", at(".zprofile.zwc"), ""}, "EPERM"},
		{"~/.zlogin.zwc", attempt{"create", at(".zlogin.zwc"), ""}, "EPERM"},
		{"~/.zlogout.zwc", attempt{"create", at(".zlogout.zwc"), ""}, "EPERM"},
		{"~/.zshrc.zwc, by a rename", attempt{"rename", draft("zwc"), at(".zshrc.zwc")}, "EPERM"},
		// …the completion dump compinit sources, oh-my-zsh's name for it, its compiled copy and compdump's own copy.
		{"~/.zcompdump", attempt{"create", at(".zcompdump"), ""}, "EPERM"},
		{"oh-my-zsh's ~/.zcompdump-<host>-<version>", attempt{"create", at(".zcompdump-mac-5.9"), ""}, "EPERM"},
		{"~/.zcompdump.zwc", attempt{"create", at(".zcompdump.zwc"), ""}, "EPERM"},
		{"compdump's copy beside it", attempt{"create", at(".zcompdump.mac.4242"), ""}, "EPERM"},
		// …the key codes /etc/zshrc sources.
		{"~/.zkbd's file", attempt{"write", at(".zkbd", "xterm-apple"), ""}, "EPERM"},
		{"a file in ~/.zkbd", attempt{"create", at(".zkbd", "xterm-256color-apple"), ""}, "EPERM"},
		{"~/.zkbd moved aside", attempt{"rename", at(".zkbd"), at("old-zkbd")}, "EPERM"},
		// …and where ZDOTDIR puts them: as given by the environment, and zsh's folder in ~/.config.
		{"$ZDOTDIR/.zshenv", attempt{"write", filepath.Join(zdot, ".zshenv"), ""}, "EPERM"},
		{"$ZDOTDIR/.zshrc", attempt{"create", filepath.Join(zdot, ".zshrc"), ""}, "EPERM"},
		{"$ZDOTDIR/.zshrc.zwc", attempt{"create", filepath.Join(zdot, ".zshrc.zwc"), ""}, "EPERM"},
		{"git's lock beside $ZDOTDIR/.zprofile", attempt{"create", filepath.Join(zdot, ".zprofile.lock"), ""}, "EPERM"},
		{"$ZDOTDIR/.zcompdump", attempt{"create", filepath.Join(zdot, ".zcompdump"), ""}, "EPERM"},
		{"$ZDOTDIR/.zsh_sessions", attempt{"mkdir", filepath.Join(zdot, ".zsh_sessions"), ""}, "EPERM"},
		{"~/.config/zsh/.zshrc", attempt{"write", at(".config", "zsh", ".zshrc"), ""}, "EPERM"},
		{"~/.config/zsh/.zshenv.zwc", attempt{"create", at(".config", "zsh", ".zshenv.zwc"), ""}, "EPERM"},

		// B2: Terminal's restored sessions, zsh's and bash's, whole.
		{"a window's restore file", attempt{"create", at(".zsh_sessions", id+".session"), ""}, "EPERM"},
		{"a window's restore file, by a rename", attempt{"rename", draft("session"), at(".zsh_sessions", id+".session")}, "EPERM"},
		{"a window's history", attempt{"write", at(".zsh_sessions", id+".history"), ""}, "EPERM"},
		{"~/.zsh_sessions moved aside", attempt{"rename", at(".zsh_sessions"), at("old-sessions")}, "EPERM"},
		{"~/.bash_sessions", attempt{"mkdir", at(".bash_sessions"), ""}, "EPERM"},
		{"~/.bash_sessions, brought in whole", attempt{"rename", staged, at(".bash_sessions")}, "EPERM"},

		// The list's others: readline's bindings, and Terminal's saved state.
		{"~/.inputrc", attempt{"create", at(".inputrc"), ""}, "EPERM"},
		{"git's lock beside ~/.inputrc", attempt{"create", at(".inputrc.lock"), ""}, "EPERM"},
		{"Terminal's saved state", attempt{"mkdir", filepath.Join(savedState, "com.apple.Terminal.savedState"), ""}, "EPERM"},

		// What most owners' ~/.zshrc sources (FXSWEEP3P1R1, recommendation (b)): oh-my-zsh, whole, and Powerlevel10k's
		// settings with their writers' names.
		{"oh-my-zsh's script", attempt{"write", at(".oh-my-zsh", "oh-my-zsh.sh"), ""}, "EPERM"},
		{"a function in oh-my-zsh's completion cache", attempt{"create", at(".oh-my-zsh", "cache", "completions", "_git"), ""}, "EPERM"},
		{"a custom plugin", attempt{"mkdir", at(".oh-my-zsh", "custom", "plugins", "x"), ""}, "EPERM"},
		{"~/.oh-my-zsh moved aside", attempt{"rename", at(".oh-my-zsh"), at("old-omz")}, "EPERM"},
		{"~/.p10k.zsh", attempt{"write", at(".p10k.zsh"), ""}, "EPERM"},
		{"~/.p10k.zsh, by a rename", attempt{"rename", draft("p10k"), at(".p10k.zsh")}, "EPERM"},
		{"git's lock beside ~/.p10k.zsh", attempt{"create", at(".p10k.zsh.lock"), ""}, "EPERM"},
		{"$ZDOTDIR/.p10k.zsh", attempt{"create", filepath.Join(zdot, ".p10k.zsh"), ""}, "EPERM"},
		// …and oh-my-zsh where its installer puts it when ZDOTDIR is set (FXSWEEP3P1R2, RT126SWEEP3FIXP1R1's N3).
		{"$ZDOTDIR/ohmyzsh's script", attempt{"write", filepath.Join(zdot, "ohmyzsh", "oh-my-zsh.sh"), ""}, "EPERM"},
		{"a custom folder in $ZDOTDIR/ohmyzsh", attempt{"mkdir", filepath.Join(zdot, "ohmyzsh", "custom"), ""}, "EPERM"},
		{"$ZDOTDIR/ohmyzsh moved aside", attempt{"rename", filepath.Join(zdot, "ohmyzsh"), filepath.Join(zdot, "old-omz")}, "EPERM"},
		{"control: $ZDOTDIR/ohmyzsh read", attempt{"read", filepath.Join(zdot, "ohmyzsh", "oh-my-zsh.sh"), ""}, "ok"},

		// P2's B2 (b): the VM tools' SSH keys, neither read nor written.
		{"Lima's key", attempt{"read", at(".lima", "_config", "user"), ""}, "EPERM"},
		{"a key of Lima's made", attempt{"create", at(".lima", "_config", "user2"), ""}, "EPERM"},
		{"Colima's key", attempt{"read", at(".colima", "_lima", "_config", "user"), ""}, "EPERM"},
		{"OrbStack's key", attempt{"read", at(".orbstack", "ssh", "id_ed25519"), ""}, "EPERM"},
		{"Podman's machine key", attempt{"read", at(".local", "share", "containers", "podman", "machine", "machine"), ""}, "EPERM"},
		{"Podman's key in ~/.ssh", attempt{"read", at(".ssh", "podman-machine-default"), ""}, "EPERM"},
		{"Podman's key in ~/.ssh, replaced", attempt{"rename", draft("podman"), at(".ssh", "podman-machine-default")}, "EPERM"},
		{"Rancher Desktop's Lima key (N2)", attempt{"read", at("Library", "Application Support", "rancher-desktop", "lima", "_config", "user"), ""}, "EPERM"},
	})
	for file, text := range owners {
		if data, err := os.ReadFile(file); err != nil || string(data) != text {
			t.Errorf("%s changed: %q (%v)", file, data, err)
		}
	}
}

// B1 and B2 end to end, as the reviewer showed them: inside, the desk compiles a plant and copies it to ~/.zshrc.zwc,
// and writes a restore file for the owner's open window; outside, the owner's next zsh, a restored Terminal window's
// (stand-in environment, stand-in home), runs neither.
func TestOnAMacTheOwnersNextZshRunsNothingADeskPlanted(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	const id = "A1B2C3D4-0000-4000-8000-000000000001"
	mustWrite(t, filepath.Join(d.home, ".zsh_sessions", id+".history"), "ls\n")
	r := d.work(t, "", d.root, 30*time.Second, []string{"PATH=/usr/bin:/bin"}, "/bin/zsh", "-f", "-c", `
		print -r -- "print -r -- STAND-IN-PLANT-RAN" > $PWD/plant.zsh && zcompile $PWD/plant.zsh &&
		cp $PWD/plant.zsh.zwc $HOME/.zshrc.zwc; print -r -- zwc=$?
		for h in $HOME/.zsh_sessions/*.history; do
			print -r -- "print -r -- STAND-IN-SESSION-RAN" > ${h%.history}.session; print -r -- session=$?
		done`)
	if !strings.Contains(r.stdout, "zwc=1") || !strings.Contains(r.stdout, "session=1") {
		t.Errorf("the desk's plants weren't refused: %+v", r)
	}
	for _, file := range []string{filepath.Join(d.home, ".zshrc.zwc"), filepath.Join(d.home, ".zsh_sessions", id+".session")} {
		if _, err := os.Lstat(file); err == nil {
			t.Errorf("%s was made", file)
		}
	}
	// The owner's next window, restored: Terminal's environment, the stand-in home. It runs no plant.
	said := outside(t, d.base, []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", "TERM=dumb", "TERM_PROGRAM=Apple_Terminal",
		"TERM_SESSION_ID=" + id}, "/bin/zsh", "-i", "-c", "print -r -- owner-shell-ran")
	if !strings.Contains(said, "owner-shell-ran") || strings.Contains(said, "STAND-IN") {
		t.Errorf("the owner's zsh: %q", said)
	}
}

// FXSWEEP3P1R1 (RT126SWEEP3FIXP1's recommendation (b)): with oh-my-zsh and ~/.p10k.zsh held, a desk's own interactive
// zsh, as Claude Code's Bash tool starts one (stdin from /dev/null, no controlling terminal), still starts: it ends
// well, says nothing, and doesn't hang. The owner's oh-my-zsh is a stand-in shaped as the review's (its start's
// writes, from its published code): check_for_upgrade.sh's gate (`-w` and `-O` on $ZSH, then a lock folder),
// compinit's dump with `-d`, its `tee -a` and the background zrecompile, and a plugin's completion cache written in
// the background. Each would say or write something with oh-my-zsh open to the desk; held, its update check is
// skipped and its cache writes fail silently, and the owner's oh-my-zsh is as it was.
func TestOnAMacADesksOwnZshStartsQuietlyWithOhMyZshHeld(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	d.noTerminal = true // as the Bash tool's shell, whatever the runner's terminal (FXZSHROW126)
	omz := filepath.Join(d.home, ".oh-my-zsh")
	mustWrite(t, filepath.Join(omz, "oh-my-zsh.sh"), `
ZSH_CACHE_DIR="$ZSH/cache"
mkdir -p "$ZSH_CACHE_DIR/completions"
fpath=("$ZSH_CACHE_DIR/completions" $fpath)
if [[ -w "$ZSH" && -O "$ZSH" ]] && command mkdir "$ZSH/log/update.lock" 2>/dev/null; then
  print -u2 "[oh-my-zsh] Would you like to update? [Y/n]"; command rm -rf "$ZSH/log/update.lock"
fi
ZSH_COMPDUMP="${ZDOTDIR:-$HOME}/.zcompdump-mac-${ZSH_VERSION}"
autoload -U compaudit compinit zrecompile
compinit -i -d "$ZSH_COMPDUMP"
tee -a "$ZSH_COMPDUMP" &>/dev/null <<DUMP
#omz revision
DUMP
{ zrecompile -q -p "$ZSH_COMPDUMP" } &!
command date 2>/dev/null >| "$ZSH_CACHE_DIR/completions/_standin" &|
`)
	for _, folder := range []string{filepath.Join(omz, "cache", "completions"), filepath.Join(omz, "log")} {
		if err := os.MkdirAll(folder, 0o700); err != nil { // made by the owner's own first window
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(d.home, ".p10k.zsh"), "typeset -g POWERLEVEL9K_MODE=compatible\n")
	zshrc := filepath.Join(d.home, ".zshrc")
	mustWrite(t, zshrc, "PS1= RPS1=\n")
	control := d.work(t, "", d.root, 30*time.Second, []string{"PATH=/usr/bin:/bin"}, "/bin/zsh", "-i")
	if control.status != 0 {
		t.Fatalf("control: a desk's zsh -i with no oh-my-zsh: %+v", control)
	}
	mustWrite(t, zshrc,
		"export ZSH=$HOME/.oh-my-zsh\nsource $ZSH/oh-my-zsh.sh\n[[ -f ~/.p10k.zsh ]] && source ~/.p10k.zsh\nPS1= RPS1=\n")
	began := time.Now()
	r := d.work(t, "", d.root, 30*time.Second, []string{"PATH=/usr/bin:/bin"}, "/bin/zsh", "-i")
	took := time.Since(began)
	t.Logf("a desk's zsh -i </dev/null: status %d after %s", r.status, took.Round(time.Millisecond))
	// Nothing but what zsh says at any interactive start (its mark for a line left open, the control's).
	if r.status != 0 || r.stdout != control.stdout || r.stderr != control.stderr || took > 10*time.Second {
		t.Errorf("a desk's zsh -i: status %d after %s, said %q %q (zsh alone: %q %q)", r.status, took, tail(r.stdout), tail(r.stderr),
			control.stdout, control.stderr)
	}
	time.Sleep(500 * time.Millisecond) // its background writes, had they been let through
	for _, made := range []string{filepath.Join(omz, "cache", "completions", "_standin"), filepath.Join(omz, "log", "update.lock"),
		filepath.Join(d.home, ".zcompdump-mac-5.9")} {
		if _, err := os.Lstat(made); err == nil {
			t.Errorf("a desk's zsh wrote %s", made)
		}
	}
}

// F3: the GM in the projects folder makes no git folder in its own plans, at any depth, nor a HEAD at their top;
// its plans themselves it still writes.
func TestOnAMacTheGMMakesNoGitFolderInItsOwnPlans(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	projects := filepath.Join(d.home, "Projects")
	plans := filepath.Join(projects, ".smolcorp", "plans", "h1")
	workspace := Workspace(d.support, "h1")
	for _, folder := range []string{filepath.Join(plans, "sub"), workspace} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(workspace, "staged-git", "HEAD"), "ref: refs/heads/main\n")
	d.pl.Subject, d.pl.Root = GM, workspace
	d.pl.Checkout, d.pl.GitDir, d.pl.OwnWorktree, d.pl.OtherWorktrees, d.pl.DefaultBranch, d.pl.HiresFolder, d.pl.GMFolder =
		"", "", "", nil, "", "", ""
	d.pl.GMProjects, d.pl.GMPlans = projects, plans
	d.prepare(t)
	staged := filepath.Join(workspace, "staged-git")
	d.root = workspace
	d.check(t, []escapeRow{
		{"control: a plan", attempt{"create", filepath.Join(plans, "PLAN.md"), ""}, "ok"},
		{"control: a folder in its plans", attempt{"mkdir", filepath.Join(plans, "drafts"), ""}, "ok"},
		{"control: .gitignore-like names are its own", attempt{"create", filepath.Join(plans, ".github"), ""}, "ok"},
		{"control: its workspace", attempt{"create", filepath.Join(workspace, "notes.md"), ""}, "ok"},
		{"a .git file at its plans, in another case", attempt{"create", filepath.Join(plans, ".GIT"), ""}, "EPERM"},
		{"a .git at its plans", attempt{"mkdir", filepath.Join(plans, ".git"), ""}, "EPERM"},
		{"a .git below its plans", attempt{"mkdir", filepath.Join(plans, "sub", ".git"), ""}, "EPERM"},
		{"a .git brought in whole", attempt{"rename", staged, filepath.Join(plans, "sub", ".git")}, "EPERM"},
		{"a bare repo's HEAD at its plans", attempt{"create", filepath.Join(plans, "HEAD"), ""}, "EPERM"},
	})
}

// F1: a desk mounts nothing, over a folder it may not write or anywhere else, and unmounts nothing of the owner's.
// The owner's own stand-in mount, made outside, shows a mount works here. Every mount is taken away at the end.
func TestOnAMacADeskCantMountOrUnmount(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	hooks := filepath.Join(d.git, "hooks")
	own := filepath.Join(d.root, "mnt")
	owners := filepath.Join(d.base, "owners-mount")
	for _, folder := range []string{own, owners} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mountOf := func(at string) (Mount, bool) {
		mounts, err := OwnersMounts()
		if err != nil {
			t.Fatal(err)
		}
		real, _ := filepath.EvalSymlinks(at)
		for _, m := range mounts {
			if m.At == at || m.At == real {
				return m, true
			}
		}
		return Mount{}, false
	}
	mounted := func(at string) bool { _, ok := mountOf(at); return ok }
	unmount := func(at string) {
		if mounted(at) {
			outside(t, d.base, nil, "/sbin/umount", at)
		}
	}
	t.Cleanup(func() { unmount(hooks); unmount(own); unmount(owners) })

	for _, folder := range []string{hooks, own} {
		r := d.work(t, "", d.root, 20*time.Second, []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}, "/sbin/mount", "-t", "devfs", "devfs", folder)
		if r.status == 0 || mounted(folder) {
			t.Errorf("a desk mounted devfs at %s: %+v", folder, r)
		}
	}
	outside(t, d.base, nil, "/sbin/mount", "-t", "devfs", "devfs", owners)
	if m, ok := mountOf(owners); !ok {
		t.Fatal("control: the owner's mount isn't in the table: the rows prove nothing")
	} else if m.Type != "devfs" {
		t.Errorf("the owner's devfs mount is in the table as %q (FXSWEEP3P1R1: the check counts it by its type)", m.Type)
	}
	r := d.work(t, "", d.root, 20*time.Second, []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}, "/sbin/umount", owners)
	if r.status == 0 || !mounted(owners) {
		t.Errorf("a desk unmounted the owner's mount at %s: %+v", owners, r)
	}
}

// P2's B1: a desk's debugger attaches to what the desk started itself, never to an unhardened program of the owner's
// outside every sandbox, nor to one in another desk's sandbox. lldb, as the reviewer ran it, from inside, only ever at
// this row's own stand-in. Needs Developer mode and the developer tools (else lldb attaches to nothing: skipped).
func TestOnAMacADeskDebugsOnlyItsOwnPrograms(t *testing.T) {
	sandboxExecOrSkip(t)
	if !strings.Contains(outside(t, "/", nil, "/usr/sbin/DevToolsSecurity", "-status"), "currently enabled") {
		t.Skip("Developer mode is off: lldb attaches to nothing without asking")
	}
	// The developer tools /usr/bin/lldb runs, found without running it or xcode-select: with none, it would ask to
	// install them.
	tools, err := os.Readlink("/var/db/xcode_select_link")
	if err != nil {
		tools = "/Library/Developer/CommandLineTools"
	}
	if _, err := os.Stat(filepath.Join(tools, "usr", "bin", "lldb")); err != nil {
		t.Skipf("no lldb in the developer tools (%s): /usr/bin/lldb would ask to install them", tools)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := newEscapeDesk(t)
	env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	attach := func(pid int) string {
		t.Helper()
		r := d.work(t, "", d.root, 2*time.Minute, env, "/usr/bin/lldb", "-p", strconv.Itoa(pid), "--batch", "-o", "process detach")
		return r.stdout + r.stderr
	}
	refusal := func(said string) string {
		for _, line := range strings.Split(said, "\n") {
			if strings.Contains(line, "attach failed") {
				return strings.TrimSpace(line)
			}
		}
		return ""
	}
	attached := func(said string, pid int) bool {
		return strings.Contains(said, "Process "+strconv.Itoa(pid)+" stopped") && !strings.Contains(said, "attach failed")
	}

	// The owner's: an unhardened stand-in outside every sandbox (built by go test, ad hoc signed, as the owner's own
	// builds and Homebrew's are).
	owners := startSleeping(t, d.base, self, standInSleep)
	if said := attach(owners.Process.Pid); attached(said, owners.Process.Pid) || !strings.Contains(said, "attach failed") {
		t.Errorf("a desk's lldb attached to the owner's program outside the sandbox:\n%s", tail(said))
	} else {
		t.Logf("the owner's: %s", refusal(said))
	}
	// Another desk's: the same stand-in in another sandbox (another desk's profile, its own instance).
	other := d.appended(t, "other-desk.sb", ";; another desk")
	others := startSleeping(t, d.base, sandboxExecOrSkip(t), "-f", other, self, standInSleep)
	if said := attach(others.Process.Pid); attached(said, others.Process.Pid) || !strings.Contains(said, "attach failed") {
		t.Errorf("a desk's lldb attached to another desk's program:\n%s", tail(said))
	} else {
		t.Logf("another desk's: %s", refusal(said))
	}
	// Its own: started by the desk itself, in its sandbox, then debugged and detached there.
	r := d.work(t, "", d.root, 2*time.Minute, standInEnv(), "/bin/sh", "-c",
		`"$0" `+standInSleep+` & p=$!; echo "own=$p"; sleep 1; /usr/bin/lldb -p "$p" --batch -o "process detach"; kill "$p"`, self)
	if !strings.Contains(r.stdout, standInSleeping) {
		t.Fatalf("its own isn't the waiting stand-in:\n%s\n%s", tail(r.stdout), tail(r.stderr))
	}
	own := 0
	for _, line := range strings.Split(r.stdout, "\n") {
		if v, ok := strings.CutPrefix(line, "own="); ok {
			own, _ = strconv.Atoi(v)
		}
	}
	if own == 0 || !attached(r.stdout+r.stderr, own) {
		t.Errorf("a desk's lldb didn't attach to its own program:\n%s\n%s", tail(r.stdout), tail(r.stderr))
	} else {
		t.Logf("its own: Process %d stopped, then detached", own)
	}
}

// P2's F2: no packet capture. Asked of macOS for a stand-in in the desk's sandbox (sandbox_check, no device opened):
// /dev/bpf0 and the clone device are denied, the desk's own files not.
func TestOnAMacADeskCantCapturePackets(t *testing.T) {
	sandboxExecOrSkip(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := newEscapeDesk(t)
	desk := startSleeping(t, d.base, sandboxExecOrSkip(t), "-f", d.profile, self, standInSleep)
	// In its sandbox once the setup key, which no desk reads, is denied to it.
	for end := time.Now().Add(10 * time.Second); childDeniesCanary(desk.Process.Pid, d.key) != 1; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the stand-in never ran in the desk's sandbox")
		}
	}
	if got := childDeniesCanary(desk.Process.Pid, filepath.Join(d.root, "main.go")); got != 0 {
		t.Fatalf("control: macOS says %d for the desk's own file (0 allowed): the rows prove nothing", got)
	}
	for _, dev := range []string{"/dev/bpf0", "/dev/bpf1", "/dev/bpf"} {
		if got := childDeniesCanary(desk.Process.Pid, dev); got != 1 {
			t.Errorf("%s: macOS says %d to the desk's read (1 denied)", dev, got)
		}
	}
}
