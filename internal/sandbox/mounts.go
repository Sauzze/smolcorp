package sandbox

import (
	"slices"
	"strings"
)

// Mount is a file system of the owner's own, where it's mounted and its type as the kernel names it (statfs's
// f_fstypename): what OwnersMounts gives.
type Mount struct {
	At, Type string
}

// DeskMountTypes are the file system types a desk running as the owner could have mounted under an earlier profile,
// which didn't deny mounting: those a user mounts with mount(2) alone, with no root, no DiskArbitration and no FUSE
// helper (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B1). Read from this Mac (macOS 26: /sbin's mount_* programs, none setuid,
// and /System/Library/Filesystems), and tried there with mount(2) on an empty folder of a test's:
//   - devfs: mounted by a user at once, and by a desk under the earlier profile (RT126SWEEP3 P1's F1);
//   - fdesc: made of the process's own descriptors, from no device, as devfs is (mount_fdesc; the kernel here has no
//     fdesc, older ones do);
//   - nfs: mount(2) takes a user's (it goes on to read the arguments, where tmpfs and nullfs refuse a user), and
//     mount_nfs isn't setuid;
//   - smbfs, afpfs, webdav and ftp: a share or a server's folder, mounted by the user's own mount_smbfs, mount_afp,
//     mount_webdav and mount_ftp, as Finder's "Connect to Server" does.
//
// Not counted: apfs, hfs, msdos, exfat, ntfs, cd9660, udf, cddafs and acfs (a disk or a disk image: DiskArbitration
// or hdiutil, which a desk can't reach), lifs (FSKit's, through fskitd), tmpfs, nullfs and autofs (root's: mount(2)
// refuses a user's tmpfs and nullfs), 9p and virtiofs (a VM's own devices), and macFUSE's (its helper). So the owner's
// own volumes, images and macFUSE mounts are never refused, wherever they are. FUSE-T mounts as nfs, so one of its
// mounts is counted as an NFS mount is: refused at or inside a narrow place, or at a folder holding a plant (MountedOver),
// and let through anywhere else (FXSWEEP3P1R2, RT126SWEEP3FIXP1R1's O1).
var DeskMountTypes = []string{"devfs", "fdesc", "nfs", "smbfs", "afpfs", "webdav", "ftp"}

// netMountTypes are those of DeskMountTypes that show a server's or a share's files, which whoever mounts it chooses:
// one above a narrow place can put a file there (FXSWEEP3P1R2, RT126SWEEP3FIXP1R1's N1). devfs and fdesc show only the
// system's devices and the process's own descriptors, read only and root's: above a narrow place, they only hide it.
var netMountTypes = []string{"nfs", "smbfs", "afpfs", "webdav", "ftp"}

// rowPlants begins the row of every plant: what the owner's programs run later, held from every desk.
const rowPlants = "plants the owner's programs run later"

// broadRows are the rows MountedOver never counts (FXSWEEP3P1R1, RT126SWEEP3FIXP1's B1): whole folders the owner works
// in, which a desk only reads, where the owner may well keep a mount of their own: the projects folder, the GM's
// projects, the owner's checkout, and other desks' places and the GM's plans there; and the rows that hold a folder
// above them, or above a desk's place, only against being moved (`~`, `~/Library`, `~/Projects`), for a mount there
// is above a narrow place, not at it.
var broadRows = []string{rowProjects, rowGMProjects, rowCheckout, rowOtherDesks, rowOtherDesks + ": the GM's folder",
	rowGMStart + ": its plans", rowPlaceAbove, rowProjects + ": the folders above it", rowGMProjects + ": the folders above it"}

// workPlaces are the keys of the places the owner and the desks work in: a desk's place, the owner's checkout, the
// hires' folder, the GM's folder, the projects folder and the GM's projects, each but those in Smolcorp's own support
// folders (a GM's workspace). MountedOver counts no mount over a narrow place at, above or inside one of these: a volume
// or a share holding a repo, or a projects folder that is one, is the owner's (FXSWEEP3P1R2).
func workPlaces(goos string, pl Places) []string {
	var supports []string
	for _, s := range append([]string{pl.Support}, pl.OtherSupports...) {
		if k := key(goos, s); k != "" {
			supports = append(supports, k)
		}
	}
	var keys []string
	for _, place := range []string{pl.Root, pl.Checkout, pl.HiresFolder, pl.GMFolder, pl.Projects, pl.GMProjects} {
		k := key(goos, place)
		if k != "" && !slices.ContainsFunc(supports, func(s string) bool { return under(k, s) }) {
			keys = append(keys, k)
		}
	}
	return keys
}

// MountedOver is the first of mounts, the owner's own (OwnersMounts), that a desk could have made under an earlier
// profile (DeskMountTypes) and that sits at or inside a narrow place p holds, or over one that is a plant, with that
// place as p spells it: ("", "") when none does (FXSWEEP3P1, RT126SWEEP3 P1's F1; narrowed by FXSWEEP3P1R1,
// RT126SWEEP3FIXP1's B1; FXSWEEP3P1R2). `mount -t devfs devfs .git/hooks` by a desk running as the owner hid the
// owner's hooks, and outlived the desk's processes. A desk's profile denies mounting now; a start and a stop refuse
// while one made before is there, in words that name it.
//
// A narrow place is one a row denies reads or writes of that isn't one of broadRows: git's folders (hooks, refs,
// config, objects, worktrees, modules), the plants (the shells', the terminals', LaunchAgents, the agents' homes),
// the support folder and the sockets. A row with a Tail is held at the folders its Tail names before its first
// pattern, whole; one whose Tail begins with a pattern (`~/.zcompdump*`, `<place>/*/**/.git`) only at what it matches,
// but for one that holds everything in its folder (a Tail of `*` or `**` alone, and Below: `~/.ssh` while Remote Login
// answers), which is held at its folder too (RT126SWEEP3FIXP1R1's N2).
//
// A mount above a narrow place is counted only when it's a network one (netMountTypes), at the folder that holds a
// plant itself, inside the home folder: `~/Library` (LaunchAgents, Preferences, Keychains), each zsh folder
// (`~/.config/zsh`, ZDOTDIR's), `~/.ssh` (its config), `~/.config` (fish's, git's, tmux's folders), `~/.cache`,
// `~/.local/share` and `~/.local/state` (the agents' settings), `~/Library/Application Support` (iTerm2's, Ghostty's),
// `~/Library/Saved Application State`, and the folders holding a file git's config names or a hooks folder: whatever
// files such a mount shows, a desk chose (RT126SWEEP3FIXP1R1's N1). Only the folder that directly holds the plant: a
// share two or more levels above one (`~/.local`, over `share/opencode`; the folder above one holding a hooks folder)
// isn't counted. That's a residual this accepts: a desk's profile denies mounting now, and the restart of the Mac the
// release notes ask for after the update unmounts any share a desk made before. Never at or above the home folder, nor
// at or inside a place the owner or a desk works in (workPlaces: `~/Projects`, a checkout); above one (the folder
// holding a checkout, `~/Library` holding a projects folder in `~/Library/CloudStorage`), every plant but the git
// folders above the places is counted (RT126SWEEP3FIXP1R2 N2). devfs and fdesc above a narrow place aren't counted:
// they only hide what's under them, and loudly.
//
// Never counted: the home folder itself and the folders above it, any other mount above a narrow place (a volume
// holding a repo, a projects folder that is a volume, a network home: it shows the owner's own files), and a file
// system of another type (an image, a macFUSE or FSKit mount, even over a narrow place: the residual this accepts).
func (p *Policy) MountedOver(home string, mounts []Mount) (mount, place string) {
	homeKey := key(p.goos, home)
	for _, m := range mounts {
		mk := key(p.goos, m.At)
		if mk == "" || homeKey == "" || under(homeKey, mk) || !slices.Contains(DeskMountTypes, m.Type) {
			continue
		}
		over := slices.Contains(netMountTypes, m.Type) && under(mk, homeKey) &&
			!slices.ContainsFunc(p.work, func(w string) bool { return under(mk, w) })
		// Above a work place, the git folders above the places (`~/Projects/.git`) aren't counted: a share holding a
		// projects folder always holds one. Every other plant is, so with the projects folder in
		// `~/Library/CloudStorage/…`, a share at `~/Library` is still found (RT126SWEEP3FIXP1R2 N2).
		aboveWork := slices.ContainsFunc(p.work, func(w string) bool { return under(w, mk) })
		overPlace := ""
		for _, r := range p.rules {
			if r.Read != Deny && r.Write != Deny || slices.Contains(broadRows, r.Row) {
				continue
			}
			held := spell(p.goos, r.Path)
			if r.Tail != "" {
				names := strings.Split(r.Tail, "/")
				if strings.ContainsAny(names[0], "*?[") {
					whole := r.Below && (r.Tail == "*" || r.Tail == "**") && !under(homeKey, r.key)
					if r.key != "" && (r.matches(mk) || whole && mk == r.key) { // at what its pattern names, or inside it
						return m.At, spell(p.goos, m.At)
					}
					continue
				}
				for _, name := range names {
					if strings.ContainsAny(name, "*?[") {
						break
					}
					held += "/" + name
				}
			}
			hk := key(p.goos, held)
			if hk == "" || under(homeKey, hk) {
				continue
			}
			if mk == hk || (r.Below || r.Tail != "") && under(mk, hk) { // at it, or inside it
				return m.At, held
			}
			aboveGit := r.Row == rowAboveGit || r.Row == rowHomeGit
			if over && overPlace == "" && strings.HasPrefix(r.Row, rowPlants) && !(aboveWork && aboveGit) &&
				key(p.goos, parent(held)) == mk {
				overPlace = held // the folder that holds it: counted once no row has it at or inside
			}
		}
		if overPlace != "" {
			return m.At, overPlace
		}
	}
	return "", ""
}
