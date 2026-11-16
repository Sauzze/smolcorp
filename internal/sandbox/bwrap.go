package sandbox

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Linux's view (SA-L1, SANDBOX-PLAN §A3.2, §A4.1 and §A5.2 step 1): a desk's Policy written as the words Smolcorp's
// BwrapPath starts the desk with. bwrap gives the desk a pid namespace with its own /proc, so it sees and signals
// only its own processes, and a mount namespace: the computer as it is (`--bind / /`, the normal work a Mac's profile
// writes as `(allow default)`), its own /dev (no other terminal's pty, no device), a private /tmp, an empty tmpfs over
// the folders the core covers ($XDG_RUNTIME_DIR, /run/screen: View.Covered) with the agents brought back
// (View.Agents), then each place a Policy row names, as it is at the start:
//   - a folder the desk may not read: an empty tmpfs over it (the support folder whole, the account folders' tokens
//     and history), with what the Policy opens in it brought back from the disk: in the support folder, the guard
//     file and the night shift's jobs, the desk's own folder, and the desk-start socket;
//   - the owner's own Claude config folders (~/.claude, each confirmed account): an empty tmpfs, nothing of them
//     brought back, a sign-in or not (SIERRA, 14 Nov). A desk signs in through its account's "hires" config folder,
//     which its desks own: View.Owned binds such a folder back whole, writable;
//   - a folder holding a name the desk may not read (the home folder for X11's key and `.claude.json`, a sign-in's
//     folder, the ssh folder while Remote Login answers, its own folder for the core's files): an
//     empty tmpfs over it, with each place in it the Policy lets the desk read brought back from the disk (a link
//     made again, to where it led), made read-only once all is mounted. A name it denies is then not there at all,
//     whatever the owner does with it later: a mount on the file itself would go when a program of the owner's
//     replaces it by a rename (Linux ≥ 3.18 lets go of mounts on a file renamed over or removed in another
//     namespace), and a file made there later would be read. Only in a folder root has alone does /dev/null go over
//     a file (or socket) itself, read-only and refused to open, since bwrap mounts it without devices;
//   - a folder none of whose files the desk may write (a plant's folder, an install root, the owner's checkout, other
//     desks' places, a shared git place) or in which no new name may be made (one holding a name the Policy denies
//     writing, below): read-only, with what the Policy lets the desk write in it brought back;
//   - a file the desk may not write (a fence, a plant, the guard file): read-only;
//   - a folder that can't be moved, removed or replaced by a link (a folder above a denied place, the desk's place,
//     the shared `objects` and `refs`): bound onto itself, which makes it a mount point the kernel won't rename or
//     remove (EBUSY);
//
// and the desk starts in its place (`--chdir`). A mount hides every earlier one below it, so the places go in from
// the top down, and one that needs nothing more than its folder above gives is left out.
//
// A name the Policy denies writing is refused when it is made, whether or not it is there at the start (FXSAL1R1,
// SIERRA's ruling): its folder is read-only for new names, with each place in it the Policy lets the desk write
// brought back as it is. That holds the plants the owner doesn't have yet (`~/.bash_profile`, `~/.zshenv`,
// `~/.config/autostart/`), the shared .git's `commondir`, `config.worktree`, `rebase-merge/` and `sequencer/`, a new
// `BRIEF*.md` in its own folder and a new `settings*.json` in its Claude folder, and every folder above the desk's
// place, the owner's checkout and the support folders: nothing new is made above the checkout, so a desk that changes
// the mode of a held git folder (`refs`, `objects`) leaves the owner's git nothing to walk up to (RT126SAL1 B2). A
// git folder already above them is read-only too. A folder root has alone needs nothing: no desk makes a name there.
// Three folders take new names for the desk's work: its place (new files), the shared `objects` (new objects), and its
// own registration in `worktrees/` (its index, HEAD and rebase). Each name the Policy denies in them is held by being
// there: the start makes the ones that may be missing, empty (placeholders: `objects/info`, an empty
// `config.worktree`, the registration's old remote folders `branches` and `remotes`, the place's `.claude`), and any
// other one missing fails the start (ErrUnheld). The start also
// makes the shared .git's `logs`, which git can't make once the .git takes no new names, and the files View.Writes
// names in its own folder.
//
// The branches (RT126SAL1R2 N1, SIERRA's ruling): a desk's branch is `hire/<its id>/<name>` (HireBranch), and only
// that folder of `refs/heads` comes back writable, with its reflogs' folder in `logs/refs/heads`; the start makes
// both. The rest of `refs/heads` and `logs/refs/heads` is read-only, whatever is there: a base branch the owner's git
// replaces by a rename, or makes while the desk runs, is in a read-only folder, not held by a mount on itself, which
// the kernel lets go of when the owner renames over it. `packed-refs` is in the read-only .git, so a desk's
// `pack-refs` or `gc` fails at its lock (EROFS) before it writes anything. A desk at the project, or on a branch of
// another name, commits nowhere.
//
// Git's config files (RT126SAL1R2 N2, SIERRA's binding check): `~/.gitconfig`, and each file git reads as config
// that the view brings back by itself into a covered folder, is bound read-only from a copy the start writes in the
// desk's own folder (gitCopy), not from the live file: the owner's `git config --global` replaces that by a rename,
// and the desk would then see an empty file and commit unsigned, as someone git makes up. The copy is written in place,
// never by a rename, and `~/.gitconfig`'s ends with `user.useConfigOnly = true`, so a desk whose owner has no identity
// configured refuses to commit rather than guess one. It is there even when the owner has no `~/.gitconfig`.
//
// The "anywhere" rows (`**/.claude`, `**/.claude/settings*.json`) hold at the desk's place, the one known place; a
// .claude deeper in its place, or in a home folder brought back writable, can be made: no mount refuses one name in
// a folder that takes others (SIERRA's call 1, a listed Linux difference). The core's own check, CLAUDEPLANT1 in
// desktop/, holds them at the start and the hand-back.
//
// A place the view must hold that is a link fails the start (ErrHeldLink): a mount goes where the link leads, and
// what the Policy holds is there. A link it doesn't hold, in a folder made read-only for new names, is left as it is:
// it can't be replaced, and it leads where the view says. The canary, `serve.sock` and the GM's memory are under the support folder's tmpfs:
// nothing brings them back.
//
// Left to the pieces after this one: seccomp (SA-L2), the sockets listening elsewhere (SA-L3's mask), the network
// namespace (SA-L6), and `--new-session` stays off (§A3.2).

// BwrapPath is the only bubblewrap the core runs: Smolcorp's own copy of the system's package, in folders root has
// alone, which its AppArmor profile names (apparmor.go, L1APPARMOR1); never one on PATH.
const BwrapPath = "/usr/local/lib/smolcorp/bwrap"

// BwrapRefusal is what the owner sees when CheckBwrap fails (§A5.2).
const BwrapRefusal = "Install bubblewrap (the `bubblewrap` package) to start hires on this computer."

// Errors the view gives.
var (
	ErrNotLinux = errors.New("sandbox: a bwrap view is for a Linux Policy")
	ErrNoBwrap  = errors.New("sandbox: " + PackagedBwrap + " is missing, or it or a folder above it isn't root's alone")
	ErrHeldLink = errors.New("sandbox: a place the view must hold is a link")
	ErrUnheld   = errors.New("sandbox: a place the Policy denies is missing where the desk's work makes new names")
	ErrNotOwn   = errors.New("sandbox: a place the start makes or binds for the desk isn't one it owns")
)

// HireBranch is the branch a desk with the id id works on, in the shared repo: `hire/<id>/<name>`. Its folder,
// `refs/heads/hire/<id>`, is the only one of the repo's branches its view lets it write (RT126SAL1R2 N1).
func HireBranch(id, name string) string { return "hire/" + id + "/" + name }

// View is what a Linux desk's start knows besides its Places.
type View struct {
	// Covered are the folders an empty tmpfs covers, whatever is made in them later, besides /tmp and /dev:
	// $XDG_RUNTIME_DIR and /run/screen (SA-X2's LinuxCovered), each expanded to a whole path: one that isn't whole
	// fails the start (ErrNotWhole), one that isn't a folder is left out.
	Covered []string
	// Agents are the sockets brought back where a tmpfs covers them: the ssh and gpg agents, the system log (SA-X2's
	// SocketPaths). One that isn't a socket, or that the Policy denies by its path or where it leads, is left out.
	Agents []string
	// Writes are the files in the desk's own folder it is told to write and may not find there (its status and
	// report, as its brief names them; what Claude Code makes at the top of its Claude folder, a folder ending in /):
	// the start makes each one missing, empty, since that folder takes no new names. Each must be a place there the
	// Policy lets the desk write (ErrNotOwn).
	Writes []string
	// Owned are whole folders the desk owns, bound back writable as they are, wherever they sit: the seam for an
	// account's "hires" config folder (SIERRA, 14 Nov: the desks of that account own it, so Claude Code's refresh by
	// a rename works in it). Each must be a folder, not a link, the Policy lets the desk read and write, in it as at
	// it, with no row of the Policy denying anything in it (ErrNotOwn). An account folder of the owner's never is.
	Owned []string
}

// PrepareBwrap is what a Linux desk's start does with its Places before the desk's first process (the runner, SA-L2,
// calls it): it makes the folders the Policy holds that are missing (MakeHold), then what the view makes (the
// placeholders, the shared .git's logs, its own branch folders and View.Writes), then gives the words after BwrapPath,
// up to and with "--", and writes the copies of git's config files they bind (gitCopy): the program and its words go
// after them. Anything that fails fails the start.
func PrepareBwrap(pl Places, v View) ([]string, error) { return prepareBwrap(pl, v, osView{}, "/tmp") }

func prepareBwrap(pl Places, v View, sys viewFS, tmp string) ([]string, error) {
	p, err := New(pl)
	if err != nil {
		return nil, err
	}
	if p.goos != "linux" {
		return nil, ErrNotLinux
	}
	if err := MakeHold(p.Hold()); err != nil {
		return nil, err
	}
	made, err := viewMade(p, pl, v)
	if err != nil {
		return nil, err
	}
	if err := makeEmpty(made); err != nil {
		return nil, err
	}
	args, err := bwrapArgs(p, pl, v, sys, tmp)
	if err != nil {
		return nil, err
	}
	for i := 0; i+2 < len(args); i++ {
		if args[i] == "--ro-bind" && args[i+1] != args[i+2] && args[i+1] != "/dev/null" {
			if err := writeGitCopy(args[i+1], args[i+2], args[i+2] == spell("linux", path.Join(pl.Home, ".gitconfig"))); err != nil {
				return nil, err
			}
		}
	}
	return args, nil
}

// viewMade is what the start makes, empty, before the view when it is missing (a folder ends in /): in each folder
// the desk's work makes new names in, the names the Policy denies there that may be missing (see above), the shared
// .git's logs, the desk's own branch folders, and v.Writes.
func viewMade(p *Policy, pl Places, v View) ([]string, error) {
	made := []string{path.Join(spell("linux", pl.Root), ".claude") + "/"}
	if pl.GitDir != "" {
		git := spell("linux", pl.GitDir)
		made = append(made, git+"/logs/", git+"/objects/info/")
		if pl.OwnWorktree != "" {
			reg := path.Join(git, "worktrees", pl.OwnWorktree)
			// Its registration's old remote folders, which the Policy denies (X12LINK1): held by being there, empty and
			// read-only (MSYSAL1). Git reads them from the shared .git only and never writes them.
			made = append(made, path.Join(reg, "config.worktree"), reg+"/branches/", reg+"/remotes/")
			// Its branch's folder and its reflogs', each folder on the way (in a reftable repo, whose refs/heads is a
			// file, no branch folder is made, and the desk commits nowhere).
			for _, at := range []string{"refs/heads/hire", "refs/heads/hire/" + pl.OwnID, "logs/refs", "logs/refs/heads",
				"logs/refs/heads/hire", "logs/refs/heads/hire/" + pl.OwnID} {
				made = append(made, git+"/"+at+"/")
			}
		}
	}
	own := path.Join(spell("linux", pl.Support), "hires", pl.OwnID)
	for _, w := range v.Writes {
		at := spell("linux", w)
		if allow, _ := p.Decide(at, Write); at == "" || at == own || !under(at, own) || allow != Allow {
			return nil, fmt.Errorf("%w: %s", ErrNotOwn, w)
		}
		if strings.HasSuffix(w, "/") {
			at += "/"
		}
		made = append(made, at)
	}
	return made, nil
}

// makeEmpty makes each of made that is missing, in a folder that is there: an empty folder (one ending in /), else an
// empty file, the owner's alone. One whose folder isn't a folder is left out: nothing is made there (a reftable
// repo's refs/heads is a file).
func makeEmpty(made []string) error {
	for _, at := range made {
		folder := strings.HasSuffix(at, "/")
		at = strings.TrimSuffix(at, "/")
		if st, err := os.Lstat(path.Dir(at)); err != nil || !st.IsDir() {
			continue
		}
		var err error
		if folder {
			err = os.Mkdir(at, 0o700)
		} else {
			var f *os.File
			if f, err = os.OpenFile(at, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
				err = f.Close()
			}
		}
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

// useConfigOnly ends the copy of `~/.gitconfig`: with no identity configured, git refuses to commit rather than make
// one up from the account and the computer's name (RT126SAL1R2 N2).
const useConfigOnly = "\n[user]\n\tuseConfigOnly = true\n"

// maxGitCopy is the most of a git config file the start copies: one larger fails the start.
const maxGitCopy = 1 << 20

// gitCopy is where the start writes the copy of the git config file at, in the desk's own folder own, which the view
// covers: the desk reads it only at at.
func gitCopy(own, at string) string {
	sum := sha256.Sum256([]byte(at))
	return path.Join(own, "git-copy-"+hex.EncodeToString(sum[:8]))
}

// writeGitCopy writes to the file copy what the file at holds now, `useConfigOnly` after it for the global config (a
// global config that is missing is empty). It writes in place, never by a rename, so a view that binds copy keeps it:
// the kernel lets go of a mount whose file is renamed over. copy must be a plain file, or missing.
func writeGitCopy(copy, at string, global bool) error {
	data, err := readPlain(at)
	switch {
	case global && errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return fmt.Errorf("sandbox: git's config file %s can't be copied: %w", at, err)
	}
	if global {
		data = append(data, useConfigOnly...)
	}
	flags := os.O_WRONLY
	was, err := os.Lstat(copy)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		flags |= os.O_CREATE | os.O_EXCL
	case err != nil:
		return err
	case !was.Mode().IsRegular():
		return fmt.Errorf("%w: %s", ErrNotOwn, copy)
	}
	f, err := os.OpenFile(copy, flags, 0o600)
	if err != nil {
		return err
	}
	if st, err := f.Stat(); err != nil || was != nil && !os.SameFile(was, st) {
		f.Close()
		return fmt.Errorf("%w: %s", ErrNotOwn, copy)
	}
	_, err = f.WriteAt(data, 0)
	if err == nil {
		err = f.Truncate(int64(len(data)))
	}
	return errors.Join(err, f.Close())
}

// readPlain is the file at's text: a plain file, not a link, of maxGitCopy at most.
func readPlain(at string) ([]byte, error) {
	st, err := os.Lstat(at)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxGitCopy {
		return nil, fmt.Errorf("not a plain file of %d bytes at most", maxGitCopy)
	}
	f, err := os.Open(at)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fst, err := f.Stat(); err != nil || !os.SameFile(st, fst) {
		return nil, errors.New("replaced while it was read")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxGitCopy+1))
	if err == nil && len(data) > maxGitCopy {
		err = errors.New("grew while it was read")
	}
	return data, err
}

// CheckBwrap is §A5.2 step 1 on Linux: PackagedBwrap and BwrapPath are programs, and each and every folder above it
// are root's, not links, and not writable by the group or others; BwrapPath is 0750 in the owner's group and the
// packaged one byte for byte; and where AppArmor limits user namespaces, its two profiles are loaded and enforcing,
// and what BwrapPath runs is under both (apparmor.go). Anything else refuses the start, with RefusalFor's words:
// there is no start without the sandbox.
func CheckBwrap() error { return checkAll(osView{}, osRoot) }

func checkAll(sys viewFS, root fs.FS) error {
	if err := checkBwrap(sys); err != nil {
		return err
	}
	return checkOwnBwrap(sys, root)
}

func checkBwrap(sys viewFS) error {
	if err := rootsAlone(sys, PackagedBwrap); err != nil {
		return fmt.Errorf("%w: %v", ErrNoBwrap, err)
	}
	return nil
}

// rootsAlone is nil when program is a file, and it and every folder above it are root's, not links, and not writable
// by the group or others. Its error is a *notRootsAlone.
func rootsAlone(sys viewFS, program string) error {
	for at := program; ; at = path.Dir(at) {
		st, err := sys.stat(at, false)
		if err != nil {
			return &notRootsAlone{at, fmt.Sprintf("%s: %v", at, err)}
		}
		if st.mode&fs.ModeSymlink != 0 || st.uid != 0 || st.mode.Perm()&0o022 != 0 || at == program && !st.mode.IsRegular() {
			return &notRootsAlone{at, fmt.Sprintf("%s (%v, owner %d)", at, st.mode, st.uid)}
		}
		if at == "/" {
			return nil
		}
	}
}

// notRootsAlone is the first place, from the program up, that is missing or isn't root's alone. A missing folder
// above it is never first: the program is missing then too.
type notRootsAlone struct {
	at  string
	why string
}

func (e *notRootsAlone) Error() string { return e.why }

// viewFS is how the view looks at the computer: os's in use, a made-up one in the tests.
type viewFS interface {
	// stat is path's mode and its owner's uid and gid (-1 when the system doesn't say): a link's own unless follow.
	stat(path string, follow bool) (viewStat, error)
	// names are a folder's entries.
	names(folder string) ([]string, error)
	// real is path with every link on it resolved.
	real(path string) (string, error)
}

type viewStat struct {
	mode fs.FileMode
	uid  int64
	gid  int64
}

type osView struct{}

func (osView) stat(p string, follow bool) (viewStat, error) {
	stat := os.Lstat
	if follow {
		stat = os.Stat
	}
	fi, err := stat(p)
	if err != nil {
		return viewStat{}, err
	}
	return viewStat{mode: fi.Mode(), uid: fileOwner(fi), gid: fileGroup(fi)}, nil
}

func (osView) names(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, err
}

func (osView) real(p string) (string, error) { return filepath.EvalSymlinks(p) }

// viewKind is how a place goes into the view.
type viewKind int8

const (
	vBind  viewKind = iota // as it is on the disk; as a mount point it can't be moved or removed
	vRO                    // read-only
	vHide                  // /dev/null over it
	vTmpfs                 // an empty tmpfs over it: nothing below it is reached
	vDev                   // the view's own /dev, a tmpfs with the safe devices
	vProc                  // the pid namespace's /proc
	vLink                  // a link made again in a covered folder, to where it led
	vCopy                  // read-only, from the copy the start writes (a git config file in a covered folder)
)

// viewOp is one place in the view.
type viewOp struct {
	kind viewKind
	at   string
	// held is a bind that must be a mount point: the Policy denies moving, removing or linking the place.
	held bool
	// opened is a place brought back into a covered folder: the Policy, or the agents' list, names it as allowed.
	opened bool
	// link is a place that is a link: held, it fails the start.
	link bool
	// agent is an agent's socket: it goes in only where a cover would hide it.
	agent bool
	// denied is a place the Policy denies reading or writing: a link there fails the start wherever the desk reaches
	// it, mounted or held by its folder.
	denied bool
	// masked is a tmpfs over a folder holding a name the Policy denies reading, with all else in it brought back, and
	// made read-only once everything inside is mounted.
	masked bool
	// to is where a vLink leads, and the copy a vCopy is bound from.
	to string
}

// words are o's words; try is a place brought back only because its folder is covered, left out if it is gone by
// the time bwrap mounts it.
func (o viewOp) words(try bool) []string {
	bind, roBind := "--bind", "--ro-bind"
	if try {
		bind, roBind = "--bind-try", "--ro-bind-try"
	}
	switch o.kind {
	case vLink:
		return []string{"--symlink", o.to, o.at}
	case vCopy:
		return []string{"--ro-bind", o.to, o.at}
	case vRO:
		return []string{roBind, o.at, o.at}
	case vHide:
		return []string{"--ro-bind", "/dev/null", o.at}
	case vTmpfs:
		return []string{"--tmpfs", o.at}
	case vDev:
		return []string{"--dev", o.at}
	case vProc:
		return []string{"--proc", o.at}
	}
	return []string{bind, o.at, o.at}
}

// covers is whether o, kept in the view, hides what is below it from the disk.
func (o viewOp) covers() bool { return o.kind == vTmpfs || o.kind == vDev || o.kind == vHide }

// viewProbe is a name no row names, for what a Policy says of a new file in a folder.
const viewProbe = ".smolcorp-view-probe"

type viewBuilder struct {
	p   *Policy
	sys viewFS
	own string // the desk's own folder in the support folder
	ops map[string]viewOp
	// keeps are the folders the desk's work makes new names in: its place, the shared objects, its own registration. A
	// name the Policy denies writing in one must be there (ErrUnheld).
	keeps map[string]bool
	// branches are the shared refs/heads and logs/refs/heads: read-only whole, nothing in them brought back but the
	// desk's own folder (RT126SAL1R2 N1).
	branches map[string]bool
	// gitFiles are the files git reads as config that the view binds from a copy when it brings one back by itself into
	// a covered folder (RT126SAL1R2 N2): ~/.gitconfig, Places.GitNamed and Places.GitConfigs.
	gitFiles map[string]bool
	// shut are the folders that take no new name, to make so; shutDone those done.
	shut     []string
	shutDone map[string]bool
	unheld   []string
	// accounts are the owner's Claude config folders: covered whole, nothing of them brought back (SIERRA, 14 Nov).
	accounts map[string]bool
	// masks are the folders holding a name the Policy denies reading, to cover; maskDone those done.
	masks    []string
	maskDone map[string]bool
}

// bwrapArgs is the view of p, a Linux Policy for pl, as the places are on sys; tmp is the folder made private.
func bwrapArgs(p *Policy, pl Places, v View, sys viewFS, tmp string) ([]string, error) {
	if p == nil || p.goos != "linux" {
		return nil, ErrNotLinux
	}
	root := spell("linux", pl.Root)
	home := spell("linux", pl.Home)
	b := &viewBuilder{p: p, sys: sys, ops: map[string]viewOp{}, own: path.Join(spell("linux", pl.Support), "hires", pl.OwnID),
		keeps: map[string]bool{root: true}, shutDone: map[string]bool{}, maskDone: map[string]bool{}, accounts: map[string]bool{},
		branches: map[string]bool{}, gitFiles: map[string]bool{path.Join(home, ".gitconfig"): true}}
	for _, account := range append([]string{path.Join(home, ".claude")}, pl.Accounts...) {
		b.accounts[spell("linux", account)] = true
	}
	for _, file := range append(slices.Clone(pl.GitNamed), pl.GitConfigs...) {
		b.gitFiles[spell("linux", file)] = true
	}
	if pl.GitDir != "" {
		git := spell("linux", pl.GitDir)
		b.keeps[git+"/objects"] = true
		if pl.OwnWorktree != "" {
			b.keeps[path.Join(git, "worktrees", pl.OwnWorktree)] = true
		}
		b.ownBranches(git, pl.OwnWorktree != "", pl.OwnID)
	}
	for _, o := range []viewOp{{kind: vBind, at: "/"}, {kind: vDev, at: "/dev"}, {kind: vProc, at: "/proc"}, {kind: vTmpfs, at: spell("linux", tmp)}} {
		b.ops[o.at] = o
	}
	for _, c := range v.Covered {
		at := spell("linux", c)
		if at == "" {
			return nil, fmt.Errorf("%w: a covered folder %q", ErrNotWhole, c)
		}
		if st, err := sys.stat(at, true); err == nil && st.mode.IsDir() && !b.has(at) {
			b.ops[at] = viewOp{kind: vTmpfs, at: at}
		}
	}
	for _, o := range v.Owned {
		if err := b.owned(o); err != nil {
			return nil, err
		}
	}
	// A git folder already above the desk's place or the owner's checkout is what the owner's git walks up to when a
	// desk has broken the one below (B2): read-only, and nothing in it brought back.
	for _, place := range []string{root, spell("linux", pl.Checkout)} {
		for folder := path.Dir(place); place != "" && place != "/"; folder = path.Dir(folder) {
			at := path.Join(folder, ".git")
			if st, err := sys.stat(at, false); err == nil && !b.has(at) {
				b.ops[at] = viewOp{kind: vRO, at: at, link: st.mode&fs.ModeSymlink != 0, denied: true}
			}
			if folder == "/" {
				break
			}
		}
	}
	for _, r := range p.rules {
		at, tail := r.Path, r.Tail
		if at == "" { // anywhere: held at the desk's place, the one known place a desk's agent reads them from
			at, tail = root, strings.TrimPrefix(tail, "**/")
		}
		if err := b.consider(at); err != nil {
			return nil, err
		}
		if tail != "" {
			if err := b.expand(at, strings.Split(tail, "/"), r.Write == Deny, r.Read == Deny); err != nil {
				return nil, err
			}
		}
	}
	for len(b.shut) > 0 || len(b.masks) > 0 {
		var err error
		if len(b.masks) > 0 {
			at := b.masks[0]
			b.masks = b.masks[1:]
			err = b.cover(at)
		} else {
			at := b.shut[0]
			b.shut = b.shut[1:]
			err = b.shutNewNames(at)
		}
		if err != nil {
			return nil, err
		}
	}
	// The owner's global git config, even when there is none: from a copy whose identity is the owner's or none.
	if global := path.Join(home, ".gitconfig"); b.ops[home].masked && !b.has(global) {
		b.ops[global] = viewOp{kind: vCopy, at: global, held: true, opened: true, to: gitCopy(b.own, global)}
	}
	if len(b.unheld) > 0 {
		slices.Sort(b.unheld)
		return nil, fmt.Errorf("%w: %s", ErrUnheld, strings.Join(slices.Compact(b.unheld), ", "))
	}
	for _, at := range v.Agents {
		b.agent(at)
	}
	return b.words(root)
}

// owned brings back the folder at whole, writable, for the desk that owns it (View.Owned).
func (b *viewBuilder) owned(o string) error {
	at := spell("linux", o)
	st, err := b.sys.stat(at, false)
	ok := at != "" && err == nil && st.mode.IsDir() && !b.accounts[at]
	for _, place := range []string{at, path.Join(at, viewProbe)} {
		for _, op := range []Op{Read, Write} {
			if v, _ := b.p.Decide(place, op); v != Allow {
				ok = false
			}
		}
	}
	for _, r := range b.p.rules {
		if r.Path != "" && r.denies() && under(key("linux", r.Path), key("linux", at)) {
			ok = false
		}
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotOwn, o)
	}
	b.ops[at] = viewOp{kind: vBind, at: at, held: true, opened: true}
	return nil
}

// ownBranches makes the shared refs/heads and logs/refs/heads read-only, and, for a desk in a worktree of its own,
// brings back writable the folder its branch is in, `hire/<id>` (HireBranch), in each: a mount point it can't move.
// logs/refs is one too, so the desk can't move the read-only folder away and make another in its place. A branch
// folder that is a link fails the start; one that isn't a folder (a reftable repo's refs/heads is a file) is left out.
func (b *viewBuilder) ownBranches(git string, worktree bool, id string) {
	for _, folder := range []string{git + "/refs/heads", git + "/logs/refs/heads"} {
		b.branches[folder] = true
		b.shutDone[folder] = true
		b.shut = append(b.shut, folder)
		at := folder + "/hire/" + id
		if st, err := b.sys.stat(at, false); worktree && err == nil && (st.mode.IsDir() || st.mode&fs.ModeSymlink != 0) {
			b.ops[at] = viewOp{kind: vBind, at: at, held: true, opened: true, link: st.mode&fs.ModeSymlink != 0}
		}
	}
	if st, err := b.sys.stat(git+"/logs/refs", false); err == nil && st.mode.IsDir() {
		b.ops[git+"/logs/refs"] = viewOp{kind: vBind, at: git + "/logs/refs", held: true}
	}
}

// refuse makes the name at, which the Policy denies writing, one the desk can't make: its folder takes no new name,
// but where the desk's work makes new names, and the name must be there.
func (b *viewBuilder) refuse(at string) {
	folder := path.Dir(at)
	switch {
	case at == "/" || folder == "/" || b.shutDone[folder]: // root's alone, never made read-only whole
	case b.keeps[folder]: // held by being there, and taking no new name itself
		if _, err := b.sys.stat(at, false); err != nil {
			if st, err := b.sys.stat(folder, false); err == nil && st.mode.IsDir() { // else no name is made in it
				b.unheld = append(b.unheld, at)
			}
		} else if !b.shutDone[at] {
			b.shutDone[at] = true
			b.shut = append(b.shut, at)
		}
	default:
		b.shutDone[folder] = true
		b.shut = append(b.shut, folder)
	}
}

// hide makes the name at, which the Policy denies reading, one the desk can't reach however the owner changes it: its
// folder is covered (RT126SAL1R1, SIERRA's call 2). A mount on the file itself would go when a program of the owner's
// replaces the file by a rename, or removes it and makes it again: since Linux 3.18 the kernel then lets go of every
// mount on it in other namespaces, and the desk would read the new file.
func (b *viewBuilder) hide(at string) {
	if folder := path.Dir(at); at != "/" && folder != "/" && !b.maskDone[folder] {
		b.maskDone[folder] = true
		b.masks = append(b.masks, folder)
	}
}

// cover makes folder a tmpfs with each place in it the Policy lets the desk read brought back as it says (a link it
// doesn't hold made again, to where it led), read-only once all is mounted: a name it denies reading isn't there,
// whether the owner makes, replaces or removes it, and nothing new is made. A folder that is missing is covered in its
// own folder; one root has alone (no program of the owner's changes it), or covered already, needs nothing.
func (b *viewBuilder) cover(folder string) error {
	st, err := b.sys.stat(folder, false)
	if notThere(err) {
		b.hide(folder)
		return nil
	} else if err != nil {
		return err
	}
	if st.uid == 0 && (st.mode&fs.ModeSymlink != 0 || st.mode.Perm()&0o022 == 0) {
		return nil
	}
	op, ok := b.ops[folder]
	switch {
	case !ok:
		op = viewOp{at: folder, held: true, link: st.mode&fs.ModeSymlink != 0, denied: true}
	case op.kind != vBind && op.kind != vRO:
		return nil
	}
	if op.link || !st.mode.IsDir() {
		op.kind = vRO
		b.ops[folder] = op
		return nil
	}
	op.kind, op.masked = vTmpfs, true
	b.ops[folder] = op
	names, err := b.sys.names(folder)
	if err != nil {
		return err
	}
	for _, name := range names {
		at := path.Join(folder, name)
		if err := b.consider(at); err != nil {
			return err
		}
		if o, ok := b.ops[at]; ok && o.link && !o.denied && !o.held {
			if to, err := b.sys.real(at); err == nil {
				b.ops[at] = viewOp{kind: vLink, at: at, to: to}
			} else {
				delete(b.ops, at) // a link to nothing: left out
			}
		}
	}
	return nil
}

// shutNewNames makes folder take no new name: read-only, with each place in it brought back as the Policy says. A
// folder that is missing is refused in its own folder; one root has alone (no desk makes a name there), or already
// read-only or covered, needs nothing.
func (b *viewBuilder) shutNewNames(folder string) error {
	st, err := b.sys.stat(folder, false)
	if notThere(err) {
		b.refuse(folder)
		return nil
	} else if err != nil {
		return err
	}
	if st.uid == 0 && (st.mode&fs.ModeSymlink != 0 || st.mode.Perm()&0o022 == 0) { // root's: no desk changes it
		return nil
	}
	op, ok := b.ops[folder]
	switch {
	case !ok:
		op = viewOp{at: folder, held: true, link: st.mode&fs.ModeSymlink != 0, denied: true}
	case op.kind != vBind:
		return nil
	}
	op.kind = vRO
	b.ops[folder] = op
	if op.link || !st.mode.IsDir() || b.branches[folder] { // the branches: only the desk's own folder comes back
		return nil
	}
	return b.bringBack(folder)
}

// bringBack considers each place in a folder made read-only, but a link the Policy doesn't deny: the folder holds
// it, and it leads where the view says.
func (b *viewBuilder) bringBack(folder string) error {
	names, err := b.sys.names(folder)
	if err != nil {
		return err
	}
	for _, name := range names {
		at := path.Join(folder, name)
		if st, err := b.sys.stat(at, false); err == nil && st.mode&fs.ModeSymlink != 0 && !b.denies(at) {
			continue
		}
		if err := b.consider(at); err != nil {
			return err
		}
	}
	return nil
}

func (b *viewBuilder) has(at string) bool {
	_, ok := b.ops[at]
	return ok
}

// expand considers each place below folder the names, a Tail's, match: `**` as no folder, since deeper copies aren't
// places the view knows. A pattern in a Tail that denies writing makes its folder take no new name: no name it
// matches can then be made.
func (b *viewBuilder) expand(folder string, names []string, deny, hide bool) error {
	if len(names) == 0 {
		return nil
	}
	if names[0] == "**" {
		return b.expand(folder, names[1:], deny, hide)
	}
	var here []string
	if !strings.ContainsAny(names[0], `*?[\`) {
		here = []string{path.Join(folder, names[0])}
	} else {
		if deny {
			b.refuse(path.Join(folder, names[0]))
		}
		if hide {
			b.hide(path.Join(folder, names[0]))
		}
		entries, err := b.sys.names(folder)
		if err != nil && !notThere(err) {
			return err
		}
		for _, e := range entries {
			if ok, _ := path.Match(names[0], e); ok {
				here = append(here, path.Join(folder, e))
			}
		}
	}
	for _, at := range here {
		if err := b.consider(at); err != nil {
			return err
		}
		if err := b.expand(at, names[1:], deny, hide); err != nil {
			return err
		}
	}
	return nil
}

// consider is how place at goes into the view, by what the Policy says of it and, for a folder, of a new file in it.
// A place the Policy denies writing is refused when it is made, there or not (refuse); one that doesn't exist needs
// nothing more, and one that can't be looked at fails the start.
func (b *viewBuilder) consider(at string) error {
	if v, _ := b.p.Decide(at, Write); v == Deny {
		b.refuse(at)
	}
	if v, _ := b.p.Decide(at, Read); v == Deny {
		b.hide(at)
	}
	if b.has(at) {
		return nil
	}
	st, err := b.sys.stat(at, false)
	if notThere(err) {
		return nil
	} else if err != nil {
		return err
	}
	p := b.p
	read, readRow := p.Decide(at, Read)
	write, writeRow := p.Decide(at, Write)
	move, _ := p.Decide(at, Move)
	link, _ := p.Decide(at, Link)
	op := viewOp{at: at, link: st.mode&fs.ModeSymlink != 0, held: write == Deny || move == Deny || link == Deny, denied: b.denies(at),
		opened: read == Allow && readRow.Row != "normal work" || write == Allow && writeRow.Row != "normal work"}
	children := false
	switch {
	case b.accounts[at] && st.mode.IsDir(): // the owner's config folder: covered whole, nothing of it brought back
		op.kind, op.opened = vTmpfs, false
	case at == b.own: // brought back: it takes no new name (its brief's pattern), what it writes there comes back
		op.kind, op.opened, children = vBind, true, true
	case st.mode&fs.ModeSocket != 0:
		if connect, row := p.Decide(at, Connect); connect == Deny {
			op.kind = vHide
		} else { // read-only: a connect still works, but its mode can't be changed
			op.kind, op.opened = vRO, op.opened || row.Row != "normal work"
		}
	case st.mode.IsDir():
		inside := path.Join(at, viewProbe)
		readIn, _ := p.Decide(inside, Read)
		writeIn, _ := p.Decide(inside, Write)
		switch {
		case read == Deny && readIn == Deny:
			op.kind = vTmpfs
		case writeIn == Deny:
			op.kind = vRO
			children = write != Deny // read-only for its new names: what the Policy lets it write there comes back
		default:
			op.kind = vBind
		}
	case read == Deny:
		op.kind = vHide
	case b.gitFiles[at] && st.mode.IsRegular(): // from a copy where a cover brings it back by itself, else read-only
		op.kind, op.to = vCopy, gitCopy(b.own, at)
	case op.held:
		op.kind = vRO
	default:
		op.kind = vBind
	}
	b.ops[at] = op
	if !children || op.link {
		return nil
	}
	return b.bringBack(at)
}

// agent brings back the socket at, where a tmpfs covers it.
func (b *viewBuilder) agent(at string) {
	if at = spell("linux", at); at == "" || b.has(at) {
		return
	}
	st, err := b.sys.stat(at, true)
	if err != nil || st.mode&fs.ModeSocket == 0 {
		return
	}
	real, err := b.sys.real(at)
	if err != nil {
		return
	}
	for _, place := range []string{at, real} {
		if v, _ := b.p.Decide(place, Connect); v == Deny {
			return
		}
	}
	b.ops[at] = viewOp{kind: vRO, at: at, opened: true, agent: true}
}

// notThere is whether err says a place isn't there: none of that name, or a file where a folder on its path would be.
func notThere(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) || errors.Is(err, syscall.ENOTDIR)
}

// denies is whether the Policy denies reading or writing at.
func (b *viewBuilder) denies(at string) bool {
	read, _ := b.p.Decide(at, Read)
	write, _ := b.p.Decide(at, Write)
	return read == Deny || write == Deny
}

// depth is how many folders down at is: "/" is 0.
func depth(at string) int {
	if at == "/" {
		return 0
	}
	return strings.Count(at, "/")
}

// words are the view's words: the namespaces, then the places from the top down, each one only where the place
// kept nearest above it doesn't already give what it needs, then the desk's place.
func (b *viewBuilder) words(root string) ([]string, error) {
	ops := make([]viewOp, 0, len(b.ops))
	for _, o := range b.ops {
		ops = append(ops, o)
	}
	slices.SortFunc(ops, func(x, y viewOp) int {
		return cmp.Or(cmp.Compare(depth(x.at), depth(y.at)), strings.Compare(x.at, y.at))
	})
	args := []string{"--unshare-pid"}
	var kept []viewOp
	var masked []string
	for _, o := range ops {
		above := nearest(kept, o.at)
		if o.kind == vCopy && !above.masked { // in a folder bound from the disk, the live file is what that shows
			o.kind = vRO
		}
		heldLink := o.link && (o.denied || o.held)
		if o.link && !heldLink && !above.covers() { // a link the Policy doesn't hold needs no mount: it leads where the view says
			continue
		}
		if o.at != "/" && !needed(above, o) {
			if heldLink && (above.masked || !above.covers()) { // held by its folder, but what it holds is where it leads
				return nil, fmt.Errorf("%w: %s", ErrHeldLink, o.at)
			}
			continue
		}
		if o.link {
			return nil, fmt.Errorf("%w: %s", ErrHeldLink, o.at)
		}
		kept = append(kept, o)
		args = append(args, o.words(above.masked && !o.held && !o.opened && !o.denied)...)
		if o.masked {
			masked = append(masked, o.at)
		}
	}
	for _, at := range masked { // once every mount point inside is made
		args = append(args, "--remount-ro", at)
	}
	return append(args, "--chdir", root, "--"), nil
}

// nearest is the place kept nearest above at: "/" at worst.
func nearest(kept []viewOp, at string) viewOp {
	for i := len(kept) - 1; i >= 0; i-- {
		if k := kept[i]; k.at != at && under(at, k.at) {
			return k
		}
	}
	return viewOp{kind: vBind, at: "/"}
}

// needed is whether o must go in below above: under a covered folder all but what it hides, under another cover only
// what is brought back, an agent nowhere else, under a read-only folder anything that isn't read-only too, under a
// bind anything but a bind that needn't be a mount point.
func needed(above, o viewOp) bool {
	switch {
	case above.masked:
		return o.kind != vHide
	case above.covers():
		return o.opened && (o.kind == vBind || o.kind == vRO || o.masked)
	case o.agent:
		return false
	case above.kind == vRO:
		return o.kind != vRO
	}
	return o.kind != vBind || o.held
}
