package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Smolcorp's own bubblewrap (L1APPARMOR1, SIERRA's ruling after VM run 1). Ubuntu, from 23.10, lets a program make a
// user namespace only when an AppArmor profile naming its file allows it (kernel.apparmor_restrict_unprivileged_userns),
// and it ships none for /usr/bin/bwrap: every view's start failed there with "setting up uid map: Permission denied",
// and a network namespace's loopback with "Operation not permitted", both for the capabilities the kernel takes away
// in a namespace no profile allows. So the core runs its own copy of the packaged bwrap, at BwrapPath, in folders root
// has alone, and the profile smolcorp-bwrap lets that one file make user namespaces, with its capabilities in them.
// Never a profile for /usr/bin/bwrap, and never a wildcard.
//
// What that grants, exactly (RT126L1APPARMOR1 B1 and RT126L1APPARMOR1R2 B3; SIERRA: both of its options 1 and 2):
//   - Who can run the copy: root, and the owner's primary group alone (the command installs it 0750 with that group,
//     and the start checks both, and that it has no setuid, setgid or sticky bit). No account outside that group can.
//   - What the copy starts: every program runs stacked under a second profile, smolcorp-bwrap-child, that denies
//     every capability and any further user namespace. So a hire, what it starts, and the copy run again from inside,
//     hold no capability and can make no user namespace: what Ubuntu's restriction leaves a program with no profile.
//     Hires lose nothing: bwrap already drops every capability before it starts them.
//   - What the copy itself can do: any program of the owner's can run it, and code it loads into the copy (with
//     LD_PRELOAD or LD_AUDIT, for one: the copy isn't setuid, and AppArmor's attachment from an unconfined program is
//     no secure exec, so ld.so honours them) runs under smolcorp-bwrap, with its capabilities in the user namespaces
//     it makes, without starting anything. That is the class Qualys published in 2025 against this restriction:
//     code injected into a program with a permissive profile. It reaches only programs already running as the owner,
//     which can read and change everything the owner can; SIERRA accepted it (FXL1APPARMOR1R2), and SandboxWhy says
//     it. Installing the copy setgid (2750) doesn't close it: the kernel marks an exec secure only when it changes
//     the effective gid from the real one (commoncap's __is_setgid), and the owner's own group is already their real
//     gid. A group of its own would make it secure, but needs a new group and a new login, and bwrap's unprivileged
//     gid map writes the real gid, which the kernel then refuses (it allows only the writer's effective gid).
//
// The profile's rules, and why each, for AppArmor 4.0's parser:
//   - smolcorp-bwrap is an enforcing profile with every rule it needs written out, not flags=(unconfined): the kernel
//     doesn't look at an unconfined-mode profile's exec rules, so the child profile would never be stacked.
//   - It allows capability and userns (bwrap's own steps: the uid map, the mounts, the loopback), mount, umount and
//     pivot_root (the view), and network, unix, signal, ptrace, mqueue, io_uring, dbus and files (rwlkm), because in a
//     stack what the hire does must be allowed by both profiles.
//   - It runs every program as pix -> &smolcorp-bwrap//&smolcorp-bwrap-child: stacked on its own label, not
//     replaced. The kernel (x_to_label) first looks for a profile attaching to the program, as px does, and stacks
//     the target on what it finds; with none, which is every program a hire runs, only pix's ix fallback finds
//     smolcorp-bwrap to stack on. A bare px -> & finds nothing there and refuses the exec (RT126L1APPARMOR1R2 B2).
//     The target names smolcorp-bwrap too, as Ubuntu's own bwrap profile does, so a program with a profile of its
//     own runs under that one, smolcorp-bwrap and the child, and not without smolcorp-bwrap: bwrap sets no_new_privs
//     before it starts the hire, and then the kernel lets an exec change label only to one that keeps the old one
//     in its stack. A plain px to the child would be refused. Lowercase p: no environment scrubbing, so hires keep
//     their environment.
//   - smolcorp-bwrap-child has no attachment (it's only stacked), the same rules but files with ix (what the hire
//     starts stays under both) and no mount rules, and audit deny for capability and userns. A deny can't be undone
//     by any allow, and the stack holds at every exec after: bwrap's pix gives the same two labels again.
//   - attach_disconnected and mediate_deleted, on both: bwrap's pivot_root leaves paths outside the new root, and
//     files deleted while open, which are otherwise refused.
//
// The owner puts the copy and the profile in place once, with sudo, through SandboxCommand: the Linux installer runs
// it, and Setup shows it. Nothing changes a kernel setting. The copy must be the packaged bwrap byte for byte, so the
// bubblewrap package's updates aren't left behind: a copy that differs fails the start until the command is run
// again. Without the copy as it must be, or with the restriction on and both profiles not loaded and enforcing, or
// what the copy runs not under both, the start is refused (CheckBwrap): a sandboxed hire never starts without its
// sandbox.

// PackagedBwrap is the system's bubblewrap package, which the copy is made from.
const PackagedBwrap = "/usr/bin/bwrap"

// AppArmorName is bwrap's profile's name, AppArmorChild the profile stacked on what it runs, and AppArmorFile where
// the command writes both.
const (
	AppArmorName  = "smolcorp-bwrap"
	AppArmorChild = AppArmorName + "-child"
	AppArmorFile  = "/etc/apparmor.d/" + AppArmorName
)

// AppArmorProfile is the two profiles, word for word (testdata/linux-apparmor.profile; the rules' whys are above).
// It holds no quote, $, ` or \, so SandboxCommand can carry it as it is.
const AppArmorProfile = `# Smolcorp: lets its own copy of bubblewrap make the user namespaces that sandboxed hires need. Every program
# that copy starts runs under ` + AppArmorChild + ` too, which denies it every capability and any user namespace.
# Put here by the Smolcorp installer, or by the one command Smolcorp Setup shows.
abi <abi/4.0>,
include <tunables/global>

profile ` + AppArmorName + ` ` + BwrapPath + ` flags=(attach_disconnected,mediate_deleted) {
  capability,
  userns,
  mount,
  umount,
  pivot_root,
  network,
  unix,
  signal,
  ptrace,
  mqueue,
  io_uring,
  dbus,
  /{**,} rwlkm,
  /** pix -> &` + AppArmorName + `//&` + AppArmorChild + `,
}

profile ` + AppArmorChild + ` flags=(attach_disconnected,mediate_deleted) {
  audit deny capability,
  audit deny userns,
  network,
  unix,
  signal,
  ptrace,
  mqueue,
  io_uring,
  dbus,
  /{**,} rwlkmix,
}
`

// Where the kernel says what it allows, as paths below "/" (fs.FS's form). The restriction reads 1 where it's on.
// Each loaded profile has a folder of its own in the policy folder, with its name, the path it attaches to and its
// mode, which any user may read. The path isn't always there: Ubuntu 24.04's kernel and parser give "<unknown>" for
// bwrap's profile (VM run 2), so what the copy runs is asked its label too (copysLabel). The abi file is the one
// AppArmor 4's userland installs, which the profile needs.
const (
	restrictFile = "proc/sys/kernel/apparmor_restrict_unprivileged_userns"
	policyFolder = "sys/kernel/security/apparmor/policy/profiles"
	abiFile      = "etc/apparmor.d/abi/4.0"
)

// unknownAttach is what the kernel's attach file reads for a profile whose attachment it keeps only compiled.
const unknownAttach = "<unknown>"

// AppArmorStack is the label of every program the copy runs, exactly, as the kernel says it: both profiles stacked,
// enforcing. VM run 2 read it inside the copy on Ubuntu 24.04.
const AppArmorStack = AppArmorName + "//&" + AppArmorChild + " (enforce)"

// labelArgs are bwrap's words for the no-op whose label the check reads: gone with bwrap if bwrap is stopped, a user
// namespace of its own (the step the restriction refuses a copy without its profile), "/" read-only, and sh printing
// the label of the cat it runs from AppArmor's own file, or, on a kernel without that one, the shared one. Nothing is
// written, and nothing stays.
var labelArgs = []string{"--die-with-parent", "--unshare-user", "--ro-bind", "/", "/", "--", "/bin/sh", "-c",
	"cat /proc/self/attr/apparmor/current 2>/dev/null || cat /proc/self/attr/current"}

// copysLabel is the AppArmor label of a program Smolcorp's copy of bubblewrap runs; the tests change it.
var copysLabel = func() (string, error) { return labelOf(BwrapPath) }

// labelOf runs bwrap with labelArgs, as the owner, with no environment but PATH, and gives what the no-op printed,
// without the kernel's line end. An exec or a bwrap that fails gives its error, with what bwrap said. One that hangs
// is stopped after 10 s, and a second later the check stops waiting for anything it started that still holds its
// output open (RT126L1APPARMOR1VM2 F1).
func labelOf(bwrap string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bwrap, labelArgs...)
	cmd.Dir, cmd.Env = "/", []string{"PATH=/usr/bin:/bin"}
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(bytes.TrimSpace(exit.Stderr)) > 0 {
		return "", fmt.Errorf("%v: %s", err, bytes.TrimSpace(exit.Stderr))
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\x00\n"), nil
}

// Errors CheckBwrap gives besides ErrNoBwrap: each is refused with RefusalFor's words.
var (
	ErrNoOwnBwrap = errors.New("sandbox: Smolcorp's copy of bubblewrap is missing, isn't root's alone and your group's to run, or isn't the packaged one")
	ErrNoProfile  = errors.New("sandbox: AppArmor limits user namespaces here, and Smolcorp's profiles aren't loaded for its bubblewrap")

	// The ones with words of their own: a bubblewrap update since the copy (RT126L1APPARMOR1 N1), a folder above the
	// copy the command can't fix (N3), and an AppArmor too old to load the profile (N2).
	errStaleBwrap  = fmt.Errorf("%w: it isn't the same file as %s (updated since?)", ErrNoOwnBwrap, PackagedBwrap)
	errBwrapFolder = fmt.Errorf("%w: a folder above it isn't root's alone", ErrNoOwnBwrap)
	errOldAppArmor = fmt.Errorf("%w: this computer's AppArmor is older than 4.0 (no /%s)", ErrNoProfile, abiFile)
)

// ownerGroup is the owner's primary group, which may run the copy: the group Smolcorp, and the command it prints,
// run as.
var ownerGroup = os.Getgid

// SandboxCommand is the one command that puts Smolcorp's bubblewrap in place: the packaged bwrap copied to BwrapPath,
// root's, 0750, in the owner's group, and, where AppArmor 4 runs (the version with the restriction), the profiles
// written and loaded.
func SandboxCommand() string { return sandboxCommand(ownerGroup()) }

func sandboxCommand(group int) string {
	lines := strings.Split(strings.TrimSuffix(AppArmorProfile, "\n"), "\n")
	return "sudo sh -c 'umask 022 && install -D -m 0750 -g " + strconv.Itoa(group) + " " + PackagedBwrap + " " + BwrapPath +
		" && if [ -e /" + abiFile + " ] && [ -d /sys/kernel/security/apparmor ]; then printf \"%s\\n\" \"" +
		strings.Join(lines, "\" \"") + "\" > " + AppArmorFile + " && apparmor_parser -r " + AppArmorFile + "; fi'"
}

// SandboxWhy says what the command does and grants, for the owner who runs it.
func SandboxWhy() string {
	return "Hires on this computer start in a sandbox that Smolcorp's own copy of bubblewrap makes. This copies your " +
		"bubblewrap to " + BwrapPath + ", where only root can change it, and only root and your account's group (gid " +
		strconv.Itoa(ownerGroup()) + ") can run it. Where AppArmor limits user namespaces (Ubuntu 23.10 and later), it " +
		"also loads a profile that lets that copy make them, and every program the copy starts runs under a second " +
		"profile that gives it no capabilities and lets it make no user namespaces of its own, so hires can't use the " +
		"copy to get past that limit, and no other account can run it. A program of yours can: by loading its own code " +
		"into the copy (with LD_PRELOAD, for one), it can use the copy's capabilities in the namespaces it makes, which " +
		"the limit otherwise refuses any program without a profile. It changes no other setting."
}

// SandboxRefusal is what the owner sees when Smolcorp's bubblewrap or its profile fails the check: why, and the one
// command, run once in a terminal.
func SandboxRefusal() string {
	return "Hires can't start on this computer until Smolcorp's own bubblewrap is set up. " + SandboxWhy() +
		" Run this once in a terminal (it asks for your password): " + SandboxCommand()
}

// RefusalFor is the owner's words for CheckBwrap's error.
func RefusalFor(err error) string {
	var place *notRootsAlone
	switch {
	case errors.Is(err, ErrNoBwrap):
		return BwrapRefusal
	case errors.Is(err, errOldAppArmor):
		return "Hires can't start on this computer: it limits user namespaces " +
			"(kernel.apparmor_restrict_unprivileged_userns), and its AppArmor is older than 4.0, which can't load the " +
			"profile Smolcorp's bubblewrap needs. Smolcorp changes no kernel setting. Once AppArmor 4 is installed, run " +
			"this once in a terminal (it asks for your password): " + SandboxCommand()
	case errors.Is(err, errBwrapFolder) && errors.As(err, &place):
		return "Hires can't start on this computer: " + place.at + ", a folder above Smolcorp's copy of bubblewrap, " +
			"isn't root's alone (owned by root, and writable by root only), and Smolcorp's command doesn't change a " +
			"folder that's already there. Once it is root's alone, run this once in a terminal (it asks for your " +
			"password): " + SandboxCommand()
	case errors.Is(err, errStaleBwrap):
		return "Hires can't start on this computer until Smolcorp's copy of bubblewrap is updated: your bubblewrap was " +
			"updated since it was made. " + SandboxWhy() + " Run this again in a terminal (it asks for your password): " +
			SandboxCommand()
	}
	return SandboxRefusal()
}

// checkOwnBwrap is the check of Smolcorp's copy (on sys, as checkBwrap looks) and of what the kernel allows (in root,
// the computer's "/").
func checkOwnBwrap(sys viewFS, root fs.FS) error {
	if err := rootsAlone(sys, BwrapPath); err != nil {
		var place *notRootsAlone
		if errors.As(err, &place) && place.at != BwrapPath {
			return fmt.Errorf("%w: %w", errBwrapFolder, err)
		}
		return fmt.Errorf("%w: %v", ErrNoOwnBwrap, err)
	}
	// No setuid, setgid or sticky bit either (RT126L1APPARMOR1R2 F3): Perm() leaves them out, and the words say 0750.
	if st, err := sys.stat(BwrapPath, false); err != nil || st.mode.Perm() != 0o750 ||
		st.mode&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 || st.gid != int64(ownerGroup()) {
		return fmt.Errorf("%w: %v, group %d, want %v, group %d (%v)", ErrNoOwnBwrap, st.mode, st.gid,
			fs.FileMode(0o750), ownerGroup(), err)
	}
	own, err := fs.ReadFile(root, BwrapPath[1:])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoOwnBwrap, err)
	}
	if packaged, err := fs.ReadFile(root, PackagedBwrap[1:]); err != nil || !bytes.Equal(own, packaged) {
		return errStaleBwrap
	}
	return checkAppArmor(root)
}

// checkAppArmor: where the kernel limits user namespaces (the restriction there and not 0), profiles named
// AppArmorName and AppArmorChild are loaded and both enforce; AppArmorName's attach file reads BwrapPath or, where the
// kernel keeps no text of it, "<unknown>" (never another path or a pattern); and a no-op the copy runs is labelled
// AppArmorStack exactly (FXL1APPARMOR1VM2 B1): that is what says the profile is the copy's, whatever attach reads.
// A restriction that can't be read, a policy folder that can't be listed, a no-op that can't run or any other label
// is refused like a missing profile. With the restriction off, or no AppArmor at all, nothing runs.
func checkAppArmor(root fs.FS) error {
	on, err := fs.ReadFile(root, restrictFile)
	if errors.Is(err, fs.ErrNotExist) || err == nil && strings.TrimSpace(string(on)) == "0" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: the restriction can't be read: %v", ErrNoProfile, err)
	}
	profiles, err := fs.ReadDir(root, policyFolder)
	if err != nil {
		return fmt.Errorf("%w: AppArmor's loaded profiles can't be listed: %v", ErrNoProfile, err)
	}
	read := func(folder, what string) string {
		data, _ := fs.ReadFile(root, policyFolder+"/"+folder+"/"+what)
		return strings.TrimSpace(string(data))
	}
	found := map[string]bool{}
	for _, p := range profiles {
		name := read(p.Name(), "name")
		if name != AppArmorName && name != AppArmorChild {
			continue
		}
		if mode := read(p.Name(), "mode"); mode != "enforce" {
			return fmt.Errorf("%w: %s is loaded in mode %q, not enforce", ErrNoProfile, name, mode)
		}
		if got := read(p.Name(), "attach"); name == AppArmorName && got != BwrapPath && got != unknownAttach {
			return fmt.Errorf("%w: %s is loaded for %q, not %s", ErrNoProfile, name, got, BwrapPath)
		}
		found[name] = true
	}
	for _, name := range []string{AppArmorName, AppArmorChild} {
		if found[name] {
			continue
		}
		if _, err := fs.Stat(root, abiFile); errors.Is(err, fs.ErrNotExist) {
			return errOldAppArmor
		}
		return fmt.Errorf("%w: %s isn't loaded", ErrNoProfile, name)
	}
	label, err := copysLabel()
	if err != nil {
		return fmt.Errorf("%w: Smolcorp's bubblewrap can't run a program to read its label: %v", ErrNoProfile, err)
	}
	if label != AppArmorStack {
		return fmt.Errorf("%w: what Smolcorp's bubblewrap runs is labelled %q, not %q", ErrNoProfile, label, AppArmorStack)
	}
	return nil
}

// runCheck is `smolcorp sandbox check` and `smolcorp sandbox command` (the command alone): the installer's and CI's
// way to the same check and command as the start's and Setup's.
func runCheck(what string, stdout, stderr io.Writer) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(stderr, "smolcorp sandbox "+what+": for Linux only")
		return 2
	}
	if what == "command" {
		fmt.Fprintln(stdout, SandboxCommand())
		return 0
	}
	return checkExit(CheckBwrap(), stdout, stderr)
}

// checkExit is what `smolcorp sandbox check` says for err: nothing, 0, when hires can start in their sandbox; the
// owner's words, 3, when the bubblewrap package is missing (the command can't help); the owner's words, which give
// the command, 1, otherwise. The specific error goes to stderr.
func checkExit(err error, stdout, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(stdout, RefusalFor(err))
	fmt.Fprintln(stderr, err)
	if errors.Is(err, ErrNoBwrap) {
		return 3
	}
	return 1
}

// osRoot is the computer's "/" as fs.FS.
var osRoot = os.DirFS("/")
