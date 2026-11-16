package sandbox

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// SA-L1: Linux's view, from the Policy, on a made-up Linux computer whose files are a map: no bwrap runs here (the
// real run is bwrap_linux_test.go's, on CI). The words are the golden ones; and the view, played as the kernel mounts
// it, says what Decide says for every place on that computer and a new file in each folder, but for the differences
// listed below, each with its reason.

const (
	lHome    = "/home/ada"
	lSupport = lHome + "/.config/sauzzedeck-pro"
	lCopy    = lHome + "/Projects/smolcorp/.dev/support"
	lRepo    = lHome + "/Projects/app"
	lGit     = lRepo + "/.git"
	lHires   = lHome + "/Projects/app-hires"
	lGM      = lHome + "/Projects/app-gm"
	lTmux    = "/tmp/tmux-1000"
	lRun     = "/run/user/1000"
)

// fakeEntry is a place on the made-up computer: 'd' a folder, 'f' a file, 's' a socket, 'l' a link to to.
type fakeEntry struct {
	kind byte
	to   string
	uid  int64
	perm fs.FileMode
	gid  int64
	ino  uint64 // 0: one of its path's own (fakeIno)
}

type fakeFS map[string]fakeEntry

func (f fakeFS) stat(p string, follow bool) (viewStat, error) {
	if err := f.refused(p); err != nil {
		return viewStat{}, err
	}
	e, ok := f[p]
	for hops := 0; ok && follow && e.kind == 'l' && hops < 8; hops++ {
		p = e.to
		if err := f.refused(p); err != nil {
			return viewStat{}, err
		}
		e, ok = f[p]
	}
	if !ok {
		return viewStat{}, fs.ErrNotExist
	}
	perm := cmpOr(e.perm, 0o755)
	mode := map[byte]fs.FileMode{'d': fs.ModeDir, 'f': 0, 's': fs.ModeSocket, 'l': fs.ModeSymlink}[e.kind] | perm
	return viewStat{mode: mode, uid: e.uid, gid: e.gid, ino: f.fakeIno(p)}, nil
}

// refused is the refusal the owner's account (uid 1000, in no group that matters) gets for looking at p on the made-up
// computer: a folder above p it can't go through, as a root-only 0700 folder (Docker's /run/docker) refuses it (VM run
// 3 B1, B2). The kernel spells it EACCES, the error the view tells it by.
func (f fakeFS) refused(p string) error {
	for dir := p; dir != path.Dir(dir); { // up to "/", or "." for a name that isn't whole
		dir = path.Dir(dir)
		if e, ok := f[dir]; ok && e.kind == 'd' && !e.lets(0o1) {
			return &fs.PathError{Op: "lstat", Path: p, Err: syscall.EACCES}
		}
	}
	return nil
}

// lets is whether the owner's account may go through (0o1) or list (0o4) the folder e: by the owner's bits in a
// folder of its own, the others' in anyone else's.
func (e fakeEntry) lets(bit fs.FileMode) bool {
	if e.uid == 1000 {
		bit <<= 6
	}
	return cmpOr(e.perm, 0o755)&bit != 0
}

// fakeIno is the inode of the file at p: its own, or one made of its path.
func (f fakeFS) fakeIno(p string) uint64 {
	if e := f[p]; e.ino != 0 {
		return e.ino
	}
	return uint64(crc32.ChecksumIEEE([]byte(p)))
}

func (f fakeFS) link(p string) (string, error) {
	if err := f.refused(p); err != nil {
		return "", err
	}
	if e, ok := f[p]; ok && e.kind == 'l' {
		return e.to, nil
	}
	return "", fs.ErrInvalid
}

// environ is the core's environment on the made-up computer: the session bus's address, and another of D-Bus's.
func (f fakeFS) environ() []string {
	return []string{"PATH=/usr/bin", "DBUS_SESSION_BUS_ADDRESS=unix:path=" + lRun + "/bus", "DBUS_FATAL_WARNINGS=0", "HOME=" + lHome}
}

func cmpOr(a, b fs.FileMode) fs.FileMode {
	if a != 0 {
		return a
	}
	return b
}

func (f fakeFS) names(folder string) ([]string, error) {
	if err := f.refused(folder); err != nil {
		return nil, err
	}
	if e, ok := f[folder]; !ok || e.kind != 'd' {
		return nil, fs.ErrNotExist
	} else if !e.lets(0o4) {
		return nil, &fs.PathError{Op: "open", Path: folder, Err: syscall.EACCES}
	}
	var names []string
	for p := range f {
		if p != "/" && path.Dir(p) == folder {
			names = append(names, path.Base(p))
		}
	}
	slices.Sort(names)
	return names, nil
}

func (f fakeFS) real(p string) (string, error) {
	for hops := 0; hops < 8; hops++ {
		if err := f.refused(p); err != nil {
			return "", err
		}
		e, ok := f[p]
		if !ok {
			return "", fs.ErrNotExist
		}
		if e.kind != 'l' {
			return p, nil
		}
		p = e.to
	}
	return "", errors.New("too many links")
}

// sockets are the made-up computer's sockets, in order: the i-th one's inode is 1000+i.
func (f fakeFS) sockets() []string {
	var sockets []string
	for p, e := range f {
		if e.kind == 's' {
			sockets = append(sockets, p)
		}
	}
	slices.Sort(sockets)
	return sockets
}

// unixSockets is /proc/net/unix as the made-up computer's kernel would write it: every socket on it listens.
func (f fakeFS) unixSockets() ([]byte, error) {
	text := netUnixHeader + "\n"
	for i, p := range f.sockets() {
		text += fmt.Sprintf("0000000000000000: 00000002 00000000 00010000 0001 01 %5d %s\n", 1000+i, p)
	}
	return []byte(text), nil
}

// unixDiag is the kernel's dump of the same sockets: each bound to the file at its name.
func (f fakeFS) unixDiag() ([]byte, error) { return diagDump(f.diagEntries()), nil }

// diagEntries are the dump's entries for the made-up computer's sockets.
func (f fakeFS) diagEntries() []diagEntry {
	var entries []diagEntry
	for i, p := range f.sockets() {
		entries = append(entries, diagEntry{ino: uint32(1000 + i), name: p + "\x00", vfs: uint32(f.fakeIno(p)), file: true})
	}
	return entries
}

// diagEntry is a socket in the kernel's dump: its inode, its name as bound (a path's ends with its NUL, an abstract
// one starts with one), and its file's inode when it has one.
type diagEntry struct {
	ino  uint32
	name string
	vfs  uint32
	file bool
}

// diagDump is the kernel's dump of entries as NETLINK_SOCK_DIAG gives it, byte for byte, and its end.
func diagDump(entries []diagEntry) []byte {
	ne := binary.NativeEndian
	var out []byte
	pad := func(b []byte) []byte {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		return b
	}
	message := func(kind uint16, body []byte) {
		h := make([]byte, 16)
		ne.PutUint32(h, uint32(16+len(body)))
		ne.PutUint16(h[4:], kind)
		out = pad(append(append(out, h...), body...))
	}
	for _, e := range entries {
		body := make([]byte, 16)
		body[0] = 1 // AF_UNIX
		ne.PutUint32(body[4:], e.ino)
		attr := func(kind uint16, value []byte) {
			a := make([]byte, 4)
			ne.PutUint16(a, uint16(4+len(value)))
			ne.PutUint16(a[2:], kind)
			body = pad(append(append(body, a...), value...))
		}
		attr(unixDiagName, []byte(e.name))
		if e.file {
			v := make([]byte, 8)
			ne.PutUint32(v, e.vfs)
			attr(unixDiagVFS, v)
		}
		message(sockDiagByFamily, body)
	}
	message(nlmsgDone, make([]byte, 4))
	return out
}

// add puts each place, and the folders above it, on the computer: a name ending in / is a folder, @ a socket.
func (f fakeFS) add(places ...string) fakeFS {
	for _, p := range places {
		kind := byte('f')
		if strings.HasSuffix(p, "/") {
			kind, p = 'd', strings.TrimSuffix(p, "/")
		} else if strings.HasSuffix(p, "@") {
			kind, p = 's', strings.TrimSuffix(p, "@")
		}
		f[p] = fakeEntry{kind: kind}
		for dir := path.Dir(p); ; dir = path.Dir(dir) {
			if _, ok := f[dir]; !ok {
				f[dir] = fakeEntry{kind: 'd'}
			}
			if dir == "/" {
				break
			}
		}
	}
	return f
}

// linuxComputer is the made-up computer: the owner's files, Smolcorp's support folders, a repo with its desks, the
// agents, and the folders a Policy holds, made as MakeHold would.
func linuxComputer() fakeFS {
	f := fakeFS{"/": {kind: 'd'}}
	f.add(
		// The support folder: records, keys, the guard file, the night shift, the canary, the sockets, the desks.
		lSupport+"/phones.json", lSupport+"/hook-key", lSupport+"/hire-guard.json", lSupport+"/nightshift.json",
		lSupport+"/nightshift.json.bad", lSupport+"/sandbox-canary-7", lSupport+"/serve.sock@", lSupport+"/desk-start@",
		lSupport+"/gm-memory/notes.json", lSupport+"/briefs/", lSupport+"/checkpoints/",
		lSupport+"/hires/h1/BRIEF.md", lSupport+"/hires/h1/STATUS.md", lSupport+"/hires/h1/REPORT.md",
		lSupport+"/hires/h1/settings.json", lSupport+"/hires/h1/hire-1.sh", lSupport+"/hires/h1/notes.txt",
		lSupport+"/hires/h1/claude/settings.json", lSupport+"/hires/h1/claude/.claude.json",
		lSupport+"/hires/h1/claude/CLAUDE.md", lSupport+"/hires/h1/claude/projects/",
		lSupport+"/hires/h2/BRIEF.md", lSupport+"/hires/gm1/BRIEF.md", lSupport+"/hires/gm1/STATUS.md",
		// SA-L6: each desk's way out, the core's proxy listening in its own folder: its own comes back, connect only.
		lSupport+"/hires/h1/proxy.sock@", lSupport+"/hires/h2/proxy.sock@", lSupport+"/hires/gm1/proxy.sock@",
		// A test copy's support folder.
		lCopy+"/phones.json", lCopy+"/hire-guard.json", lCopy+"/desk-start@", lCopy+"/hires/h1/BRIEF.md", lCopy+"/hires/h1/STATUS.md",
		lCopy+"/hires/h1/proxy.sock@",
		// The owner's plants, sign-ins and agents.
		lHome+"/.bashrc", lHome+"/.profile", lHome+"/.gitconfig", lHome+"/.gitconfig-work", lHome+"/.tmux.conf",
		lHome+"/.Xauthority", lHome+"/.claude.json", lHome+"/.claude.json.backup", lHome+"/.githooks/pre-commit",
		lHome+"/.claude/settings.json", lHome+"/.claude/CLAUDE.md", lHome+"/.claude/.credentials.json",
		lHome+"/.claude/history.jsonl", lHome+"/.claude/projects/", lHome+"/.claude/sessions/", lHome+"/.claude/ide/",
		lHome+"/.claude/commands/", lHome+"/.claude-work/.credentials.json", lHome+"/.claude-work/.claude.json",
		lHome+"/.ssh/id_ed25519", lHome+"/.ssh/id_ed25519.pub", lHome+"/.ssh/known_hosts", lHome+"/.ssh/config",
		lHome+"/.ssh/authorized_keys", lHome+"/.codex/auth.json", lHome+"/.codex/config.toml", lHome+"/.gemini/oauth_creds.json",
		lHome+"/.config/git/config", lHome+"/.config/systemd/user/", lHome+"/.config/fish/config.fish",
		lHome+"/.local/share/claude/versions/2.1", lHome+"/.1password/agent.sock@", lHome+"/notes/todo.txt",
		// The repo, its desks, the GM's folder.
		lRepo+"/src/main.go", lRepo+"/.claude/settings.json", lGit+"/config", lGit+"/HEAD", lGit+"/index",
		lGit+"/hooks/pre-commit", lGit+"/info/exclude", lGit+"/objects/info/", lGit+"/objects/ab/cdef",
		lGit+"/refs/heads/main", lGit+"/refs/heads/develop", lGit+"/refs/heads/h1", lGit+"/refs/heads/release/1.0",
		lGit+"/refs/heads/hire/h1/work", lGit+"/refs/heads/hire/h2/work", lGit+"/refs/heads/hire/gm1/plan",
		lGit+"/packed-refs", lGit+"/logs/HEAD", lGit+"/logs/refs/heads/main", lGit+"/logs/refs/heads/hire/h1/work",
		lGit+"/modules/lib/config",
		// The old remote folders X12LINK1 denies (MSYSAL1): older gits made an empty branches/ at `git init`.
		lGit+"/branches/", lGit+"/remotes/",
		lGit+"/worktrees/h1/HEAD", lGit+"/worktrees/h1/commondir", lGit+"/worktrees/h1/gitdir", lGit+"/worktrees/h1/index",
		lGit+"/worktrees/h2/HEAD", lGit+"/worktrees/gm/HEAD", lGit+"/worktrees/gm/commondir", lGit+"/worktrees/gm/gitdir",
		lHires+"/h1/.git", lHires+"/h1/src/main.go", lHires+"/h1/.claude/settings.json", lHires+"/h2/src/main.go",
		lGM+"/.git", lGM+"/.claude/settings.json", lGM+"/plan.md",
		// Programs, /tmp, the runtime folder and its agents, /dev's log.
		"/opt/smolcorp/smolcorp", "/etc/gitconfig", "/usr/bin/git", "/tmp/build/out.o",
		lTmux+"/smolcorp-h1.d/s@", lTmux+"/smolcorp-h2.d/s@", lTmux+"/smolcorp-gm1.d/s@", lTmux+"/default@",
		lRun+"/bus@", lRun+"/keyring/ssh@", lRun+"/gnupg/S.gpg-agent@", lRun+"/gdm/Xauthority", "/run/screen/S-ada/1.pts@",
		"/run/systemd/journal/dev-log@",
		// SA-L3's sockets off the list: the system D-Bus, Docker's, a system service's, one in the home folder, and one
		// in a folder of the owner's the view brings back writable.
		"/run/dbus/system_bus_socket@", "/run/dbus/pid", "/run/docker.sock@", "/run/avahi-daemon/socket@",
		lHome+"/stand-in.sock@", lHome+"/.local/state/app/ipc.sock@",
		// SIERRA's class ruling (FXSAL3R1): systemd --user's own socket, and another user's runtime folder and bus.
		lRun+"/systemd/private@", "/run/user/1001/bus@",
	)
	f.add("/dev/")
	f["/dev/log"] = fakeEntry{kind: 'l', to: "/run/systemd/journal/dev-log"}
	f["/var/run"] = fakeEntry{kind: 'l', to: "/run"}
	f.owners()
	return f
}

// owners gives the home folder and all in it to the owner (uid 1000): the rest is root's alone, as on a computer.
func (f fakeFS) owners() fakeFS {
	for p, e := range f {
		if under(p, "/home/ada") && e.uid == 0 {
			e.uid = 1000
			f[p] = e
		}
	}
	return f
}

// makeAll makes on f each place the start makes that is missing, in a folder that is there, as makeEmpty does.
func (f fakeFS) makeAll(made []string) {
	for _, at := range made {
		kind := byte('f')
		if strings.HasSuffix(at, "/") {
			kind, at = 'd', strings.TrimSuffix(at, "/")
		}
		if _, ok := f[at]; ok || f[path.Dir(at)].kind != 'd' {
			continue
		}
		f[at] = fakeEntry{kind: kind, uid: 1000}
	}
}

// linuxPlaces is a hire's places on the made-up computer, with its core's support folder.
func linuxPlaces(support string) Places {
	return Places{GOOS: "linux", Subject: Hire, Home: lHome, Support: support, OwnID: "h1", DeskStart: support + "/desk-start",
		Root: lHires + "/h1", Checkout: lRepo, GitDir: lGit, OwnWorktree: "h1", OtherWorktrees: []string{"h1", "h2", "gm"},
		DefaultBranch: "develop", HiresFolder: lHires, GMFolder: lGM, SocketParent: lTmux, OwnSockets: lTmux + "/smolcorp-h1.d",
		Accounts: []string{lHome + "/.claude-work"}, InstallRoots: []string{"/opt/smolcorp", lHome + "/.local/share/claude"},
		GitConfigs: []string{"/etc/gitconfig"}, GitHooks: []string{lHome + "/.githooks"}, GitNamed: []string{lHome + "/.gitconfig-work"},
		RemoteLogin: true, XAuthority: lRun + "/gdm/Xauthority"}
}

// linuxSetups are the golden columns: a hire, the GM, a test copy's hire, and a hire with Remote Login off.
func linuxSetups() []setup {
	hire := linuxPlaces(lSupport)
	gm := linuxPlaces(lSupport)
	gm.Subject, gm.OwnID, gm.Root, gm.OwnWorktree, gm.OwnSockets = GM, "gm1", lGM, "gm", lTmux+"/smolcorp-gm1.d"
	testCopy := linuxPlaces(lCopy)
	testCopy.OtherSupports = []string{lSupport}
	noSSH := linuxPlaces(lSupport)
	noSSH.RemoteLogin = false
	return []setup{{"linux hire", hire}, {"linux GM", gm}, {"linux test copy's hire", testCopy}, {"linux hire, no Remote Login", noSSH}}
}

var linuxView = View{
	Covered: []string{lRun, "/run/screen", "/run/nothing-here"},
	Agents:  []string{lRun + "/keyring/ssh", lHome + "/.1password/agent.sock", lRun + "/gnupg/S.gpg-agent", "/dev/log", lSupport + "/serve.sock"},
}

// viewOf is a setup's Policy and its view's words on f, after its holds and what the view makes are made there, as
// PrepareBwrap makes them.
func viewOf(t *testing.T, s setup, f fakeFS) (*Policy, []string) {
	t.Helper()
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, linuxView, f)
	args, err := bwrapArgs(p, s.pl, linuxView, f, "/tmp")
	if err != nil {
		t.Fatalf("%s: %v", s.name, err)
	}
	return p, args
}

// prepare makes on f what PrepareBwrap makes on the disk: the Policy's holds, then what the view makes.
func prepare(t *testing.T, p *Policy, pl Places, v View, f fakeFS) {
	t.Helper()
	for _, folder := range p.Hold() {
		if _, ok := f[folder]; !ok {
			f.add(folder + "/")
		}
	}
	made, err := viewMade(p, pl, v)
	if err != nil {
		t.Fatal(err)
	}
	f.owners().makeAll(made)
	// The start's copy of the places no mount holds, and its guard's notes beside it (SaveKept, L1GUARD1).
	f.add(KeptFolder(pl.Support, pl.OwnID)+"/"+keptList, keptStopFile(KeptFolder(pl.Support, pl.OwnID))).owners()
}

// argvText is the words, a line per option, a word with a space or a quote quoted.
func argvText(args []string) string {
	var s strings.Builder
	for i, w := range args {
		if strings.ContainsAny(w, " \"\\\n\t") {
			w = strconv.Quote(w)
		}
		switch {
		case i == 0:
		case strings.HasPrefix(w, "--"):
			s.WriteByte('\n')
		default:
			s.WriteByte(' ')
		}
		s.WriteString(w)
	}
	return s.String() + "\n"
}

func bwrapGoldenName(name string) string {
	return filepath.Join("testdata", strings.NewReplacer(" ", "-", "'", "", ",", "").Replace(name)+".bwrap")
}

// The view is the golden words, byte for byte. To write them again after a change to the Policy or the view, run the
// test with SMOLCORP_BWRAP_GOLDEN=write, and read the diff.
func TestTheLinuxViewIsTheGoldenWords(t *testing.T) {
	for _, s := range linuxSetups() {
		_, args := viewOf(t, s, linuxComputer())
		text, file := argvText(args), bwrapGoldenName(s.name)
		if os.Getenv("SMOLCORP_BWRAP_GOLDEN") == "write" {
			if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		golden, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%s: %v (SMOLCORP_BWRAP_GOLDEN=write makes it)", s.name, err)
		}
		if text != string(golden) {
			got, want := strings.Split(text, "\n"), strings.Split(string(golden), "\n")
			for i := 0; i < max(len(got), len(want)); i++ {
				if i >= len(got) || i >= len(want) || got[i] != want[i] {
					t.Fatalf("%s: line %d differs from %s:\n got %q\nwant %q", s.name, i+1, file,
						strings.Join(got[i:min(i+1, len(got))], ""), strings.Join(want[i:min(i+1, len(want))], ""))
				}
			}
		}
	}
}

// mounted is one mount of a view, as the kernel keeps it.
type mounted struct{ kind, at string }

// playView mounts a view's words in order, as bwrap does: each mount hides those below its place made before it.
func playView(t *testing.T, args []string) (mounts []mounted, chdir string) {
	t.Helper()
	for i := 0; i < len(args); i++ {
		var m mounted
		switch w := args[i]; w {
		case "--unshare-pid", "--unshare-net":
			continue
		case "--unsetenv":
			i++
			continue
		case "--":
			if i != len(args)-1 {
				t.Fatalf("words after --: %q", args[i+1:])
			}
			return mounts, chdir
		case "--chdir":
			chdir, i = args[i+1], i+1
			continue
		case "--tmpfs", "--dev", "--proc":
			m, i = mounted{w, args[i+1]}, i+1
		case "--remount-ro":
			at := args[i+1]
			i++
			j := slices.IndexFunc(mounts, func(m mounted) bool { return m.at == at })
			if j < 0 || mounts[j].kind != "--tmpfs" || slices.ContainsFunc(args[i+1:], func(w string) bool {
				return w != "--remount-ro" && strings.HasPrefix(w, "--") && w != "--chdir" && w != "--"
			}) {
				t.Fatalf("--remount-ro %s: not a tmpfs, or not after every mount", at)
			}
			mounts[j].kind = "cover"
			continue
		case "--symlink":
			m, i = mounted{"symlink", args[i+2]}, i+2
		case "--bind", "--ro-bind", "--bind-try", "--ro-bind-try":
			src, at := args[i+1], args[i+2]
			i += 2
			w = strings.TrimSuffix(w, "-try")
			switch {
			case w == "--ro-bind" && src == "/dev/null":
				m = mounted{"hide", at}
			case w == "--ro-bind" && strings.HasPrefix(path.Base(src), "git-copy-") && src == gitCopy(path.Dir(src), at):
				m = mounted{"copy", at} // RT126SAL1R2 N2: a place bound from elsewhere, a git config file's copy
			case w == "--ro-bind" && src != at && path.IsAbs(src) && path.Clean(src) == src:
				m = mounted{"agent", at} // RT126SAL3R2 F2b: and an agent's socket, from where its way was checked to lead
			case src != at:
				t.Fatalf("%s %s %s: a place is only ever bound onto itself, hidden, a git config file's copy or an agent's socket", w, src, at)
			default:
				m = mounted{w, at}
			}
		default:
			t.Fatalf("a word the view doesn't write: %q", w)
		}
		for _, old := range mounts {
			if old.at == m.at || under(old.at, m.at) {
				t.Fatalf("%s %s hides %s %s, put in before it", m.kind, m.at, old.kind, old.at)
			}
		}
		mounts = append(mounts, m)
	}
	t.Fatal("the words don't end with --")
	return nil, ""
}

// says is what the view lets a desk do at p: the deepest mount over it decides. A cover reaches nothing of the
// disk; /dev/null over a file refuses it; a read-only bind reads and connects; a bind does all but move its own
// place, which is a mount point.
func says(mounts []mounted, p string, op Op) Verdict {
	m := mounted{"--bind", "/"}
	for _, mm := range mounts {
		if under(p, mm.at) && len(mm.at) >= len(m.at) {
			m = mm
		}
	}
	switch m.kind {
	case "cover": // a folder's tmpfs, made read-only, with what the Policy allows brought back: it lists, nothing else
		if p == m.at && op == Read {
			return Allow
		}
		return Deny
	case "--tmpfs", "--dev", "--proc", "hide", "symlink":
		return Deny
	case "--ro-bind", "copy", "agent":
		if op == Read || op == Connect {
			return Allow
		}
		return Deny
	}
	if (op == Move || op == Link) && p == m.at {
		return Deny
	}
	return Allow
}

// A difference between the view and the Policy: where, which way, and why.
type viewDiff struct {
	why   string
	match func(s setup, p string, op Op, policy Verdict) bool
}

func inside(p string, folders ...string) bool {
	return slices.ContainsFunc(folders, func(f string) bool { return under(p, f) })
}

// viewDiffs are every place the view and the Policy part on the made-up computer. Each is one the doc comment in
// bwrap.go names; a new difference fails the test until it's listed here, with its reason.
var viewDiffs = []viewDiff{
	// Looser than the Policy: what Linux can't hold by a mount.
	{"loose: a held folder can't be moved, removed or made a link (a mount point), but its mode can be changed", func(s setup, p string, op Op, v Verdict) bool {
		return v == Deny && op == Write && slices.ContainsFunc([]string{"a desk's place and the folders above it can't be moved or removed",
			"the shared git places: what makes it a git folder", "the shared git places: a worktree's registration"},
			func(row string) bool { return heldBy(s, p, row) })
	}},
	{"loose: its own folder is brought back read-only: it lists, and reads a file the core makes there later", func(s setup, p string, op Op, v Verdict) bool {
		own := s.pl.Support + "/hires/" + s.pl.OwnID
		return v == Deny && op == Read && (p == own || path.Dir(p) == own && !onTheComputer(p))
	}},
	{"loose: a socket brought back has no data to read; it can't be written, moved or removed", func(s setup, p string, op Op, v Verdict) bool {
		// The desk-start socket, and the desk's own way out (SA-L6), each connect only.
		return v == Deny && op == Read && (p == s.pl.DeskStart || p == s.pl.Support+"/hires/"+s.pl.OwnID+"/"+ProxySocketName)
	}},
	{"loose: a link in its own socket folder (no links there: the core's to check, never following one)", func(s setup, p string, op Op, v Verdict) bool {
		return v == Deny && op == Link && path.Dir(p) == s.pl.OwnSockets
	}},
	// SIERRA's call 1 (FXSAL1R1): the "anywhere" rows hold at the known places; deeper, no mount refuses one name in a
	// folder that takes new names, and the core's own check (CLAUDEPLANT1, desktop/) holds them at the start and the
	// hand-back.
	{"Linux: a nested .claude under the desk's place or a writable home subfolder can be made; held at start and hand-back by CLAUDEPLANT1", func(s setup, p string, op Op, v Verdict) bool {
		return v == Deny && under(p, s.pl.Root+"/src") && strings.Contains(p, "/.claude")
	}},
	// Tighter than the Policy: the view's own denials, and read-only folders.
	{"tight: /tmp is private, the runtime folder and /run/screen covered, /dev and /proc the view's own", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && inside(p, "/tmp", lRun, "/run/user/1001", "/run/screen", "/dev", "/proc")
	}},
	{"tight: a place held is a mount point: it can't be moved, removed or made a link where the Policy says only a write", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && (op == Move || op == Link)
	}},
	{"tight: the owner's Claude config folders are covered whole: nothing of them comes back, a sign-in or not (SIERRA, 14 Nov)", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && op == Read && inside(p, append([]string{lHome + "/.claude"}, s.pl.Accounts...)...)
	}},
	{"tight: a folder holding a name the Policy denies reading is a tmpfs with what the Policy allows brought back: a name not there at the start isn't seen, whoever makes it later", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && op == Read && !onTheComputer(p) && slices.ContainsFunc(coveredFolders(s), func(f string) bool { return p != f && under(p, f) })
	}},
	// SIERRA's N1 ruling (RT126SAL1R2): only the desk's own folder of the branches comes back writable.
	// SA-L3 (mask.go): every listening socket off the list is masked, and the folders root has alone of the sockets
	// Smolcorp always masks are covered.
	{"tight: a socket that listens at the start, off the desk's list, is masked: not there, or /dev/null over it", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && slices.Contains(maskedSockets(s), p)
	}},
	{"tight: /run and /run/dbus, root's, hold a socket Smolcorp always masks: covered, with all else brought back; nothing made there later is seen", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && (inside(p, "/run", "/run/dbus") && !onTheComputer(p) || (p == "/run" || p == "/run/dbus") && op != Read)
	}},
	{"tight: refs/heads and logs/refs/heads are read-only but for the desk's own hire/<id>/: it writes no other branch or reflog", func(s setup, p string, op Op, v Verdict) bool {
		own := []string{lGit + "/refs/heads/hire/" + s.pl.OwnID, lGit + "/logs/refs/heads/hire/" + s.pl.OwnID}
		return v == Allow && op == Write && inside(p, lGit+"/refs/heads", lGit+"/logs/refs/heads") && !inside(p, own...)
	}},
	{"tight: a folder holding a name the Policy denies writing takes no new name, and its mode can't be changed; what the Policy lets a desk write there comes back", func(s setup, p string, op Op, v Verdict) bool {
		return v == Allow && op == Write && slices.ContainsFunc(noNewNames(s), func(f string) bool {
			return p == f || path.Dir(p) == f && !onTheComputer(p)
		})
	}},
}

// maskedSockets are the sockets on the made-up computer a setup's view masks (SA-L3): each listens, and isn't an agent,
// its own socket, or its core's desk-start socket. The ones under a folder covered whole are there too, already gone.
func maskedSockets(s setup) []string {
	masked := []string{"/run/dbus/system_bus_socket", "/run/docker.sock", "/run/avahi-daemon/socket", lHome + "/stand-in.sock",
		lHome + "/.local/state/app/ipc.sock", lRun + "/bus", lRun + "/systemd/private", "/run/user/1001/bus"}
	if s.pl.Support != lCopy {
		masked = append(masked, lCopy+"/desk-start", lCopy+"/hires/h1/proxy.sock")
	}
	return masked
}

// noNewNames are the folders of a setup that take no new name, on the made-up computer: each holds a name the Policy
// denies writing (a plant, a sign-in, a fence, a shared git place, a pattern's names), or is above the desk's place,
// the owner's checkout or a support folder, or is a placeholder for a base branch.
func noNewNames(s setup) []string {
	own := s.pl.Support + "/hires/" + s.pl.OwnID
	folders := []string{lHome, lHome + "/.config", lHome + "/.config/systemd", lHome + "/.codex", lHome + "/.gemini", lHome + "/.local/share",
		lHome + "/Library", lHome + "/Projects", lHome + "/.ssh", lGit, lGit + "/worktrees", own, own + "/claude", s.pl.Root + "/.claude"}
	if s.pl.Support == lCopy {
		folders = append(folders, lHome+"/Projects/smolcorp", lHome+"/Projects/smolcorp/.dev")
	}
	return folders
}

// coveredFolders are the folders of a setup that are a tmpfs with what the Policy allows brought back, on the made-up
// computer: each holds a name the Policy denies reading (a sign-in, X11's key, `.claude.json`, a key while Remote
// Login answers, the support folder, a record in its own folder), and a program of the owner's can change it.
func coveredFolders(s setup) []string {
	folders := []string{lHome, lHome + "/.config", lHome + "/.claude", lHome + "/.claude-work", lHome + "/.codex", lHome + "/.gemini",
		s.pl.Support + "/hires/" + s.pl.OwnID}
	if s.pl.RemoteLogin {
		folders = append(folders, lHome+"/.ssh")
	}
	if s.pl.Support == lCopy {
		folders = append(folders, lHome+"/Projects/smolcorp/.dev")
	}
	return folders
}

// onTheComputer is whether p is on the made-up computer as it is before a start: holds and placeholders not made.
func onTheComputer(p string) bool {
	_, ok := linuxComputer()[p]
	return ok
}

// heldBy is whether row is the Policy's narrowest write deny at p.
func heldBy(s setup, p, row string) bool {
	_, r := mustNewQuiet(s.pl).Decide(p, Write)
	return r.Row == row
}

func mustNewQuiet(pl Places) *Policy {
	p, err := New(pl)
	if err != nil {
		panic(err)
	}
	return p
}

// rt126sal1Probes are the reviewer's places (RT126SAL1 B1): missing on the made-up computer, denied writing by the
// Policy, and writable in the view before FXSAL1R1.
var rt126sal1Probes = []string{
	lHome + "/.bash_profile", lHome + "/.zshenv", lHome + "/.bash_login",
	lHome + "/.config/autostart", lHome + "/.config/autostart/stand-in.desktop", lHome + "/.config/tmux/tmux.conf",
	lGit + "/commondir", lGit + "/rebase-merge/git-rebase-todo", lGit + "/sequencer/todo", lGit + "/refs/heads/master",
	lGit + "/config.worktree", lSupport + "/hires/h1/sandbox.sb", lSupport + "/hires/h1/BRIEF-2.md",
	lSupport + "/hires/h1/claude/settings.local.json",
}

// patternStandIn is a name a pattern matches: each `*` as "stand-in", each `?` as "x".
func patternStandIn(pattern string) string {
	return strings.NewReplacer("*", "stand-in", "?", "x", "[", "", "]", "", `\`, "").Replace(pattern)
}

// missingDenied are the places the Policy denies reading or writing that f doesn't have, as its rows name them: each
// row's Path, each Tail's literal names below it, a stand-in name for each pattern in a Tail (besides the names f has
// that it matches), a stand-in name inside each place denied Below, and the rows for anywhere at the desk's place and
// in a folder below it.
func missingDenied(p *Policy, pl Places, f fakeFS) []string {
	var missing []string
	var walk func(at string, names []string, below bool)
	walk = func(at string, names []string, below bool) {
		switch {
		case len(names) == 0:
			if e, ok := f[at]; !ok {
				missing = append(missing, at)
			} else if below && e.kind != 'd' {
				return
			}
			if _, ok := f[at+"/stand-in"]; below && !ok {
				missing = append(missing, at+"/stand-in")
			}
		case names[0] == "**":
			walk(at, names[1:], below)
		case strings.ContainsAny(names[0], `*?[\`):
			for _, name := range f.namesOf(at) {
				if ok, _ := path.Match(names[0], name); ok {
					walk(path.Join(at, name), names[1:], below)
				}
			}
			walk(path.Join(at, patternStandIn(names[0])), names[1:], below)
		default:
			walk(path.Join(at, names[0]), names[1:], below)
		}
	}
	for _, r := range p.Rules() {
		if r.Write != Deny && r.Read != Deny {
			continue
		}
		if r.Path == "" {
			for _, at := range []string{pl.Root, pl.Root + "/src"} {
				walk(at, strings.Split(strings.TrimPrefix(r.Tail, "**/"), "/"), r.Below)
			}
			continue
		}
		var names []string
		if r.Tail != "" {
			names = strings.Split(r.Tail, "/")
		}
		walk(r.Path, names, r.Below)
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}

// namesOf are a folder's entries on f, none when it isn't there.
func (f fakeFS) namesOf(folder string) []string {
	names, _ := f.names(folder)
	return names
}

// The view, played as mounted, says what Decide says, in every column, for every place on the made-up computer, a
// new file in each folder, and every place the Policy denies that the computer doesn't have at the start (B1,
// missingDenied, and the reviewer's probes): reads, writes, moves, links and, for a socket, connects. Where they
// part, a listed difference says why. A deny the view doesn't hold, unlisted, fails.
func TestTheLinuxViewHoldsWhatThePolicySays(t *testing.T) {
	used := map[string]bool{}
	for _, s := range linuxSetups() {
		f := linuxComputer()
		missing := missingDenied(mustNew(t, s.pl), s.pl, f)
		for _, probe := range rt126sal1Probes {
			if _, ok := f[probe]; ok {
				t.Fatalf("%s: the reviewer's probe %s is on the made-up computer", s.name, probe)
			}
			if s.pl.Support == lSupport && s.pl.OwnID == "h1" {
				missing = append(missing, probe)
			}
		}
		if len(missing) < 40 {
			t.Fatalf("%s: only %d missing denied places to try: %q", s.name, len(missing), missing)
		}
		p, args := viewOf(t, s, f)
		mounts, chdir := playView(t, args)
		if chdir != s.pl.Root {
			t.Errorf("%s: starts in %q, want %q", s.name, chdir, s.pl.Root)
		}
		// SIERRA's call 2: a file the Policy denies reading, in a folder a program of the owner's can change, is hidden by
		// its folder's tmpfs, never by /dev/null over it, which a rename outside would take away.
		for _, m := range mounts {
			// SA-L3: a socket the mask masks is masked by /dev/null where its folder is brought back writable (mask.go;
			// §A3.2 step 4 says a socket made again there is reachable). Keyed on those sockets alone (RT126SAL3 nit 1):
			// any other place the Policy denies, a socket too, still fails here.
			if slices.Contains(maskedSockets(s), m.at) {
				continue
			}
			if e := f[path.Dir(m.at)]; m.kind == "hide" && !(e.uid == 0 && cmpOr(e.perm, 0o755)&0o022 == 0) {
				t.Errorf("%s: %s is hidden by a mount on itself, in a folder the owner changes", s.name, m.at)
			}
			// SIERRA (14 Nov, a desk's sign-in): nothing of the owner's own config folders is brought back.
			for _, account := range append([]string{lHome + "/.claude"}, s.pl.Accounts...) {
				if under(m.at, account) && !(m.at == account && m.kind == "--tmpfs") && m.kind != "--tmpfs" {
					t.Errorf("%s: %s %s brings back part of the owner's config folder %s", s.name, m.kind, m.at, account)
				}
			}
		}
		if !slices.Contains(mounts, mounted{"copy", s.pl.Home + "/.gitconfig"}) {
			t.Errorf("%s: ~/.gitconfig isn't bound from the start's copy", s.name)
		}
		whys, needs := heldOnlyByItself(s, f, mounts)
		for _, why := range whys {
			t.Error(why)
		}
		kept := map[string]bool{}
		for _, k := range keptPlaces(s.pl) {
			kept[k.at] = !k.branch
		}
		for at, why := range awaitingSierra(s) {
			if !needs[at] {
				t.Errorf("%s: %s is listed, but no rule needs it (%s)", s.name, at, why)
			}
			if !kept[at] {
				t.Errorf("%s: %s is listed as held by L1GUARD1, but its start doesn't keep it", s.name, at)
			}
		}
		for at, why := range heldBelow(s) {
			if !needs[at+"/"] {
				t.Errorf("%s: %s is listed as holding a denied place below it, but no rule needs it (%s)", s.name, at, why)
			}
		}
		// F1: the desk's own branch folders, which a mount holds writable, are watched by what they are.
		for _, at := range []string{lGit + "/refs/heads/hire/" + s.pl.OwnID, lGit + "/logs/refs/heads/hire/" + s.pl.OwnID} {
			if b, ok := kept[at]; !ok || b {
				t.Errorf("%s: its branch folder %s isn't watched", s.name, at)
			}
		}
		var places []string
		for at, e := range f {
			places = append(places, at)
			if e.kind == 'd' {
				places = append(places, path.Join(at, viewProbe))
			}
		}
		places = append(places, missing...)
		slices.Sort(places)
		places = slices.Compact(places)
		for _, at := range places {
			ops := []Op{Read, Write, Move, Link}
			if f[at].kind == 's' {
				ops = append(ops, Connect)
			}
			for _, op := range ops {
				policy, row := p.Decide(at, op)
				view := says(mounts, at, op)
				if policy == view {
					continue
				}
				i := slices.IndexFunc(viewDiffs, func(d viewDiff) bool { return d.match(s, at, op, policy) })
				if i < 0 {
					t.Errorf("%s: %s %s: the Policy says %v (%s), the view %v", s.name, op, at, policy, row.Row, view)
					continue
				}
				used[viewDiffs[i].why] = true
			}
		}
	}
	for _, d := range viewDiffs {
		if !used[d.why] {
			t.Errorf("a difference listed that no place shows: %s", d.why)
		}
	}
}

// ownerChanges is whether a program of the owner's can change folder on f: it isn't root's alone.
func ownerChanges(f fakeFS, folder string) bool {
	e := f[folder]
	return !(e.uid == 0 && cmpOr(e.perm, 0o755)&0o022 == 0)
}

// gitConfigFiles are the files a setup's git reads as config: the owner's global one, the files the owner's config
// names and the other config files the core found (Places.GitNamed, Places.GitConfigs), the shared .git's and the
// worktrees'.
func gitConfigFiles(s setup) []string {
	files := []string{s.pl.Home + "/.gitconfig", lGit + "/config", lGit + "/config.worktree", lGit + "/worktrees/" + s.pl.OwnWorktree + "/config.worktree"}
	return append(append(files, s.pl.GitNamed...), s.pl.GitConfigs...)
}

// awaitingSierra are the places RT126SAL1R2 N1 names that no mount can hold against a rename outside, each in a
// folder the desk's own work keeps making new names in (its place, its registration, the shared objects). SIERRA's
// ruling (15 Nov): L1GUARD1 holds each, by the start's copy compared on both sides of every tool call, at the next start
// and at a fire or a take over (kept.go); the test checks each is one of keptPlaces. Nothing else may be named here.
// MSYSAL1 added its registration's branches and remotes (X12LINK1's rows): git neither writes nor reads them there,
// only in the shared .git, so only a program of the owner's renaming over one by hand lets it go.
func awaitingSierra(s setup) map[string]string {
	reg := lGit + "/worktrees/" + s.pl.OwnWorktree
	const by = "held by L1GUARD1's kept copy: "
	return map[string]string{
		s.pl.Root + "/.claude":   by + "its place's .claude: its place takes its new files",
		s.pl.Root + "/.git":      by + "its place's .git: its place takes its new files",
		reg + "/commondir":       by + "its registration's commondir: the registration takes git's HEAD.lock and index.lock",
		reg + "/gitdir":          by + "its registration's gitdir: the registration takes git's HEAD.lock and index.lock",
		reg + "/config.worktree": by + "its registration's config: the registration takes git's HEAD.lock and index.lock",
		reg + "/branches":        by + "its registration's old remote folder: the registration takes git's HEAD.lock and index.lock",
		reg + "/remotes":         by + "its registration's old remote folder: the registration takes git's HEAD.lock and index.lock",
		lGit + "/objects/info":   by + "the shared objects/info: objects takes the desk's new objects' folders",
	}
}

// heldBelow are the mounts whose folder holds a place the Policy denies writing, held only by that mount: a rename
// outside of the folder itself lets the place go (RT126SAL1R3 F3). Each says why that is left as it is.
func heldBelow(s setup) map[string]string {
	return map[string]string{
		lGit + "/refs/heads": "the base branches: refs/heads is read-only for new names, and no git renames or removes " +
			"refs/heads itself (pack-refs empties and removes only the folders below it), so only a program of the " +
			"owner's renaming the whole folder by hand lets them go",
		lHome + "/.local/share": "the install root ~/.local/share/claude: only a program of the owner's renaming " +
			"~/.local/share itself, the data folder of every program they run, lets it go",
	}
}

// heldOnlyByItself is RT126SAL1R2's rule for N1 and N2: no place the Policy denies writing is held only by a mount on
// itself whose mount point the owner's programs can replace (one in a folder of the disk they change, not on the
// view's own tmpfs), in a folder a desk can write; and no git config file is bound from the live file in a folder the
// owner changes, but from the start's copy. For each such mount it takes it away, and what is mounted below it, as
// a rename outside does, and asks the view again.
func heldOnlyByItself(s setup, f fakeFS, mounts []mounted) (whys []string, needs map[string]bool) {
	needs = map[string]bool{}
	p := mustNewQuiet(s.pl)
	exempt := awaitingSierra(s)
	for i, m := range mounts {
		above := mounted{"--bind", "/"}
		for _, a := range mounts[:i] {
			if m.at != a.at && under(m.at, a.at) && len(a.at) >= len(above.at) {
				above = a
			}
		}
		if !ownerChanges(f, path.Dir(m.at)) {
			continue // root's alone: no program of the owner's replaces it
		}
		var why []string
		if slices.Contains(gitConfigFiles(s), m.at) && (m.kind == "--ro-bind" || m.kind == "--bind") {
			why = append(why, fmt.Sprintf("%s: the git config file %s is bound from the live file, in a folder the owner changes", s.name, m.at))
		}
		if above.kind == "--tmpfs" || above.kind == "cover" { // its mount point is on the view's own tmpfs
			whys = append(whys, why...)
			continue
		}
		rest := slices.DeleteFunc(slices.Clone(mounts), func(o mounted) bool { return under(o.at, m.at) })
		for _, at := range []string{m.at, m.at + "/" + viewProbe} {
			if v, _ := p.Decide(m.at, Write); v == Deny && says(rest, at, Write) == Allow {
				why = append(why, fmt.Sprintf("%s: once the owner replaces %s by a rename, %s can be written: it is held only by a mount on itself", s.name, m.at, at))
				break
			}
		}
		if len(why) > 0 && exempt[m.at] != "" {
			needs[m.at] = true
		} else {
			whys = append(whys, why...)
		}
		// F3: and each place below it the Policy denies writing, a new name in each folder below it included. A folder
		// L1GUARD1 keeps is kept whole, each file's content in it too.
		if exempt[m.at] != "" {
			continue
		}
		var below []string
		for at, e := range f {
			if at != m.at && under(at, m.at) {
				below = append(below, at)
				if e.kind == 'd' {
					below = append(below, path.Join(at, viewProbe))
				}
			}
		}
		slices.Sort(below)
		for _, at := range below {
			if v, _ := p.Decide(at, Write); v == Deny && says(mounts, at, Write) == Deny && says(rest, at, Write) == Allow {
				if heldBelow(s)[m.at] != "" {
					needs[m.at+"/"] = true
				} else {
					whys = append(whys, fmt.Sprintf("%s: once the owner replaces %s by a rename, %s below it can be written", s.name, m.at, at))
				}
				break
			}
		}
	}
	return whys, needs
}

// /usr/bin/bwrap, which Smolcorp's copy is checked against, counts only when it, and every folder above it, is
// root's alone, and not a link (§A5.2 step 1).
func TestBwrapIsRunOnlyWhenItAndItsFoldersAreRootsAlone(t *testing.T) {
	good := func() fakeFS {
		return fakeFS{"/": {kind: 'd'}, "/usr": {kind: 'd'}, "/usr/bin": {kind: 'd'}, PackagedBwrap: {kind: 'f'}}
	}
	if err := checkBwrap(good()); err != nil {
		t.Fatalf("the packaged bwrap is refused: %v", err)
	}
	for _, c := range []struct {
		name   string
		change func(fakeFS)
	}{
		{"missing", func(f fakeFS) { delete(f, PackagedBwrap) }},
		{"the owner's", func(f fakeFS) { f[PackagedBwrap] = fakeEntry{kind: 'f', uid: 1000} }},
		{"writable by its group", func(f fakeFS) { f[PackagedBwrap] = fakeEntry{kind: 'f', perm: 0o775} }},
		{"a link to a copy", func(f fakeFS) {
			f[PackagedBwrap] = fakeEntry{kind: 'l', to: "/opt/bwrap"}
			f["/opt/bwrap"] = fakeEntry{kind: 'f'}
		}},
		{"a folder", func(f fakeFS) { f[PackagedBwrap] = fakeEntry{kind: 'd'} }},
		{"in a folder the owner has", func(f fakeFS) { f["/usr/bin"] = fakeEntry{kind: 'd', uid: 1000} }},
		{"in a folder anyone writes", func(f fakeFS) { f["/usr"] = fakeEntry{kind: 'd', perm: 0o777} }},
		{"under a root anyone writes", func(f fakeFS) { f["/"] = fakeEntry{kind: 'd', perm: 0o757} }},
		{"of an owner the system doesn't say", func(f fakeFS) { f[PackagedBwrap] = fakeEntry{kind: 'f', uid: -1} }},
	} {
		f := good()
		c.change(f)
		if err := checkBwrap(f); !errors.Is(err, ErrNoBwrap) {
			t.Errorf("bwrap %s: %v, want ErrNoBwrap", c.name, err)
		}
	}
	if BwrapRefusal != "Install bubblewrap (the `bubblewrap` package) to start hires on this computer." {
		t.Errorf("the owner's words aren't §A5.2's: %q", BwrapRefusal)
	}
}

// A place the view must hold that is a link fails the start: the mount would go where it leads, and the link, in a
// folder the desk writes, could be replaced. One the view leaves alone, or holds by its folder, is no matter.
func TestAHeldPlaceThatIsALinkFailsTheStart(t *testing.T) {
	s := linuxSetups()[0]
	for _, c := range []struct {
		name, at, to string
		fails        bool
	}{
		{"a plant linked from a dotfiles repo", lHome + "/.bashrc", lHome + "/dotfiles/bashrc", true},
		{"a folder above a denied place", lHome + "/.config", lHome + "/dotfiles/config", true},
		{"its brief, linked to a file it writes", lSupport + "/hires/h1/BRIEF.md", lHires + "/h1/src/main.go", true},
		{"its status, linked to a record: it leads inside the view, where the record is covered", lSupport + "/hires/h1/STATUS.md", lSupport + "/phones.json", false},
		{"its place's settings", lHires + "/h1/.claude/settings.json", lHome + "/elsewhere.json", true},
		{"a sign-in", lHome + "/.codex/auth.json", lHome + "/dotfiles/auth.json", true},
		{"its registration's old remote folder (X12LINK1's row, MSYSAL1)", lGit + "/worktrees/h1/remotes", lHome + "/dotfiles/config", true},
		{"a file of its own", lHires + "/h1/src/link.go", lHires + "/h1/src/main.go", false},
		{"one in a read-only folder", lHome + "/.claude/commands/x.md", lHome + "/dotfiles/x.md", false},
		{"one in the covered support folder", lSupport + "/briefs/old.md", lHome + "/notes/todo.txt", false},
	} {
		f := linuxComputer().add(lHome+"/dotfiles/bashrc", lHome+"/dotfiles/config/git/config", lHome+"/elsewhere.json",
			lHome+"/dotfiles/auth.json", lHome+"/dotfiles/x.md")
		if c.at == lHome+"/.config" {
			for p := range f {
				if under(p, c.at) && p != c.at {
					delete(f, p)
				}
			}
		}
		f[c.at] = fakeEntry{kind: 'l', to: c.to, uid: 1000}
		p := mustNew(t, s.pl)
		prepare(t, p, s.pl, linuxView, f)
		_, err := bwrapArgs(p, s.pl, linuxView, f, "/tmp")
		if c.fails && !errors.Is(err, ErrHeldLink) || !c.fails && err != nil {
			t.Errorf("%s (%s): %v, want a failure: %v", c.name, c.at, err, c.fails)
		}
	}
}

// The agents come back read-only where a cover hides them, and only there; one that isn't a socket, or that the
// Policy denies where it is or where it leads, stays out.
func TestOnlyAgentsUnderACoverComeBack(t *testing.T) {
	s := linuxSetups()[0]
	f := linuxComputer().add(lRun+"/not-a-socket", lRun+"/pipewire-0@")
	f[lRun+"/tap"] = fakeEntry{kind: 'l', to: lSupport + "/serve.sock"}
	f[lRun+"/ssh-link"] = fakeEntry{kind: 'l', to: lRun + "/keyring/ssh"}
	p := mustNew(t, s.pl)
	prepare(t, p, s.pl, linuxView, f)
	v := linuxView
	v.Agents = append(slices.Clone(v.Agents), lRun+"/not-a-socket", lRun+"/tap", lRun+"/missing", "relative/agent", lRun+"/ssh-link")
	args, err := bwrapArgs(p, s.pl, v, f, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	text := argvText(args)
	// RT126SAL3R2 F2b: each from where its way was checked to lead (/dev/log is a link root has), so a link swapped at
	// its name before bwrap mounts it isn't followed.
	for in, from := range map[string]string{lRun + "/keyring/ssh": lRun + "/keyring/ssh", lRun + "/gnupg/S.gpg-agent": lRun + "/gnupg/S.gpg-agent",
		"/dev/log": "/run/systemd/journal/dev-log", lRun + "/ssh-link": lRun + "/keyring/ssh"} {
		if !strings.Contains(text, "--ro-bind "+from+" "+in+"\n") {
			t.Errorf("%s isn't brought back read-only from %s:\n%s", in, from, text)
		}
	}
	for _, out := range []string{lHome + "/.1password/agent.sock", lRun + "/not-a-socket", lRun + "/tap", lRun + "/pipewire-0", lRun + "/bus", lRun + "/missing", "relative"} {
		if strings.Contains(text, out) {
			t.Errorf("%s is in the view's words:\n%s", out, text)
		}
	}
}

// The view is for a Linux Policy only; a Policy New refuses is refused here too.
func TestTheViewIsForLinuxOnly(t *testing.T) {
	mac := hirePlaces(realSupport)
	if _, err := PrepareBwrap(mac, linuxView); !errors.Is(err, ErrNotLinux) {
		t.Errorf("a Mac's places: %v, want ErrNotLinux", err)
	}
	if _, err := bwrapArgs(mustNew(t, mac), mac, linuxView, linuxComputer(), "/tmp"); !errors.Is(err, ErrNotLinux) {
		t.Errorf("a Mac's Policy: %v, want ErrNotLinux", err)
	}
	if _, err := bwrapArgs(nil, mac, linuxView, linuxComputer(), "/tmp"); !errors.Is(err, ErrNotLinux) {
		t.Errorf("no Policy: %v, want ErrNotLinux", err)
	}
	inSupport := linuxPlaces(lSupport)
	inSupport.Root = lSupport + "/hires/h1/work"
	if _, err := PrepareBwrap(inSupport, linuxView); !errors.Is(err, ErrRootInSupport) {
		t.Errorf("a place inside the support folder: %v, want ErrRootInSupport", err)
	}
}

// B2 (RT126SAL1, RT126SAX1R5 B3): a desk that changes the mode of a held git folder makes the owner's git walk up from
// the checkout, or from the desk's place, to the first git folder above it. Nothing new is made in any folder above
// them, so there is none to walk up to; one already there (the owner's dotfiles in `~/.git`) is read-only; and the
// checkout's .git itself takes no mode change. A folder root has alone is left to the kernel: no desk makes a name there.
func TestNothingNewIsMadeAboveTheCheckout(t *testing.T) {
	for _, dotfiles := range []bool{false, true} {
		for _, s := range linuxSetups() {
			f := linuxComputer()
			if dotfiles {
				f.add(lHome+"/.git/config", lHome+"/.git/HEAD").owners()
			}
			_, args := viewOf(t, s, f)
			mounts, _ := playView(t, args)
			for _, place := range []string{s.pl.Checkout, s.pl.Root} {
				for folder := path.Dir(place); folder != "/"; folder = path.Dir(folder) {
					if e := f[folder]; e.uid == 0 && cmpOr(e.perm, 0o755)&0o022 == 0 {
						continue // root's alone
					}
					for _, name := range []string{".git", ".git/config", "HEAD", "objects", "refs", "config"} {
						if at := path.Join(folder, name); says(mounts, at, Write) != Deny {
							t.Errorf("%s (dotfiles %v): %s can be made or written above %s", s.name, dotfiles, at, place)
						}
					}
				}
			}
			if says(mounts, lGit, Write) != Deny {
				t.Errorf("%s: the checkout's .git takes a mode change", s.name)
			}
			if dotfiles && (strings.Contains(argvText(args), "--bind "+lHome+"/.git ") || says(mounts, lHome+"/.git/config", Write) != Deny) {
				t.Errorf("%s: the owner's ~/.git is brought back writable:\n%s", s.name, argvText(args))
			}
		}
	}
}

// What PrepareBwrap makes on the disk, here in the test's own folder (no bwrap runs): the placeholders for the names
// the Policy denies in the folders the desk's work makes new names in, the shared .git's logs, the desk's own branch
// folders, and View.Writes, each empty and the owner's alone; nothing where the folder isn't one (a reftable repo's
// refs/heads), and nothing over what is there. The words then hold each one read-only, bring back only the desk's
// own branch folders writable, and bind the owner's global git config from the copy the start wrote (N2), which a
// second start writes again in place.
func TestTheStartMakesThePlaceholdersEmpty(t *testing.T) {
	for _, reftable := range []bool{false, true} {
		base, err := filepath.EvalSymlinks(t.TempDir()) // the Policy's places are resolved (a Mac's temp folder is under /var)
		if err != nil {
			t.Fatal(err)
		}
		home := base + "/home"
		support := home + "/.config/sauzzedeck-pro"
		repo := home + "/Projects/app"
		git := repo + "/.git"
		root := home + "/Projects/app-hires/h1"
		for _, folder := range []string{support + "/hires/h1", git + "/objects/ab", git + "/worktrees/h1", root + "/src"} {
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		files := map[string]string{git + "/HEAD": "ref: refs/heads/develop\n", git + "/config": "", git + "/worktrees/h1/commondir": "../..\n",
			git + "/worktrees/h1/gitdir": root + "/.git\n", root + "/.git": "gitdir: " + git + "/worktrees/h1\n", support + "/hires/h1/BRIEF.md": "brief"}
		if reftable {
			files[git+"/refs/heads"] = "this repository uses the reftable format\n"
		} else {
			files[git+"/refs/heads/develop"] = "0000000000000000000000000000000000000000\n"
		}
		for at, text := range files {
			if err := os.MkdirAll(path.Dir(at), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
			OwnWorktree: "h1", DefaultBranch: "develop", HiresFolder: home + "/Projects/app-hires"}
		var logged []string
		v := View{Writes: []string{support + "/hires/h1/STATUS.md", support + "/hires/h1/claude/"},
			Log: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }}
		if err := os.WriteFile(home+"/.gitconfig", []byte("[user]\n\temail = stand-in@example.invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
		args, err := prepareBwrap(pl, v, rootAbove{base}, base+"/tmp")
		if err != nil {
			t.Fatalf("reftable %v: %v", reftable, err)
		}
		text := argvText(args)
		// VM run 3 B2: containerd's folder, which the owner's account can't open, doesn't stop the start. It is covered
		// (a socket Smolcorp always masks is made there) with nothing in it brought back, and the log says why.
		if !strings.Contains(text, "--tmpfs "+closedRun+"\n") || !strings.Contains(text, "--remount-ro "+closedRun+"\n") ||
			strings.Contains(text, closedRun+"/") {
			t.Errorf("reftable %v: %s isn't covered with nothing in it brought back:\n%s", reftable, closedRun, text)
		}
		if len(logged) != 1 || !strings.Contains(logged[0], "can't open "+closedRun+",") {
			t.Errorf("reftable %v: logged %q, want %s named once", reftable, logged, closedRun)
		}
		wantFolders := []string{root + "/.claude", git + "/logs", git + "/objects/info", support + "/hires/h1/claude",
			git + "/logs/refs/heads/hire/h1", git + "/worktrees/h1/branches", git + "/worktrees/h1/remotes"}
		wantFiles := []string{git + "/worktrees/h1/config.worktree", support + "/hires/h1/STATUS.md"}
		if !reftable {
			wantFolders = append(wantFolders, git+"/refs/heads/hire/h1")
		}
		for _, at := range []string{git + "/refs/heads/main", git + "/refs/heads/master", git + "/refs/heads/release"} {
			if _, err := os.Lstat(at); !reftable && err == nil {
				t.Errorf("a base branch's placeholder is made: %s", at)
			}
		}
		// The global config's copy: the owner's, then useConfigOnly; bound at ~/.gitconfig, and written again in place.
		copy := gitCopy(support+"/hires/h1", home+"/.gitconfig")
		before, err := os.Lstat(copy)
		if data, _ := os.ReadFile(copy); err != nil || string(data) != "[user]\n\temail = stand-in@example.invalid"+useConfigOnly || before.Mode().Perm() != 0o600 {
			t.Errorf("reftable %v: the copy of ~/.gitconfig: %q, %v", reftable, data, err)
		}
		if !strings.Contains(text, "--ro-bind "+copy+" "+home+"/.gitconfig\n") {
			t.Errorf("~/.gitconfig isn't bound from its copy:\n%s", text)
		}
		if err := os.WriteFile(home+"/.gitconfig.lock", nil, 0o600); err != nil { // the owner's git config --global, emptying it
			t.Fatal(err)
		}
		if err := os.Rename(home+"/.gitconfig.lock", home+"/.gitconfig"); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareBwrap(pl, v, rootAbove{base}, base+"/tmp"); err != nil {
			t.Fatal(err)
		}
		if after, err := os.Lstat(copy); err != nil || !os.SameFile(before, after) {
			t.Errorf("reftable %v: the copy was replaced, not written in place: %v", reftable, err)
		}
		if data, _ := os.ReadFile(copy); string(data) != useConfigOnly {
			t.Errorf("reftable %v: the copy of an emptied ~/.gitconfig: %q", reftable, data)
		}
		for _, at := range append(wantFolders, wantFiles...) {
			st, err := os.Lstat(at)
			switch {
			case err != nil:
				t.Errorf("reftable %v: %s isn't made: %v", reftable, at, err)
			case st.IsDir() != slices.Contains(wantFolders, at) || st.Mode().Perm()&0o077 != 0:
				t.Errorf("reftable %v: %s is made %v", reftable, at, st.Mode())
			case !st.IsDir() && st.Size() != 0:
				t.Errorf("reftable %v: %s isn't empty", reftable, at)
			}
		}
		for _, at := range []string{git + "/refs/heads", git + "/logs/refs/heads", git + "/worktrees/h1/config.worktree", root + "/.claude",
			git + "/worktrees/h1/branches", git + "/worktrees/h1/remotes"} {
			if !reftable && !strings.Contains(text, "--ro-bind "+at+" "+at+"\n") {
				t.Errorf("%s isn't held read-only:\n%s", at, text)
			}
		}
		for _, at := range []string{git + "/refs/heads/hire/h1", git + "/logs/refs/heads/hire/h1"} {
			if !reftable && !strings.Contains(text, "--bind "+at+" "+at+"\n") {
				t.Errorf("%s isn't brought back writable:\n%s", at, text)
			}
		}
		if data, _ := os.ReadFile(git + "/refs/heads/develop"); !reftable && len(data) == 0 {
			t.Error("the default branch's ref was changed")
		}
		if reftable {
			if st, err := os.Lstat(git + "/refs/heads"); err != nil || st.IsDir() {
				t.Errorf("a reftable repo's refs/heads was changed: %v", err)
			}
		}
	}
}

// What a start is given is checked: a covered folder that isn't a whole path fails the start (SA-X2's raw
// $XDG_RUNTIME_DIR would otherwise go uncovered: RT126SAL1 nit 1), and so does a file to make that isn't one the
// desk may write in its own folder.
func TestTheViewRefusesWhatItCantPlace(t *testing.T) {
	s := linuxSetups()[0]
	p := mustNew(t, s.pl)
	f := linuxComputer()
	prepare(t, p, s.pl, linuxView, f)
	for _, covered := range []string{"$XDG_RUNTIME_DIR", "run/user/1000", ""} {
		v := linuxView
		v.Covered = append(slices.Clone(v.Covered), covered)
		if _, err := bwrapArgs(p, s.pl, v, f, "/tmp"); !errors.Is(err, ErrNotWhole) {
			t.Errorf("covered %q: %v, want ErrNotWhole", covered, err)
		}
	}
	own := lSupport + "/hires/h1"
	for _, w := range []string{own + "/BRIEF-2.md", own + "/settings.json", own + "/claude/settings.local.json", own, lHome + "/.zshenv",
		lSupport + "/hires/h2/STATUS.md", "relative/STATUS.md", own + "/../h2/STATUS.md"} {
		if _, err := viewMade(p, s.pl, View{Writes: []string{w}}); !errors.Is(err, ErrNotOwn) {
			t.Errorf("a file to make %q: %v, want ErrNotOwn", w, err)
		}
	}
	if _, err := viewMade(p, s.pl, View{Writes: []string{own + "/STATUS.md", own + "/REPORT-x.md", own + "/claude/todos/"}}); err != nil {
		t.Errorf("its status, report and Claude folder: %v", err)
	}
	// A name the Policy denies, missing in a folder the desk's work makes new names in, that the start doesn't make,
	// fails the start: the worktree's own registration without the file that says where its .git is.
	delete(f, lGit+"/worktrees/h1/gitdir")
	if _, err := bwrapArgs(p, s.pl, linuxView, f, "/tmp"); !errors.Is(err, ErrUnheld) {
		t.Errorf("its registration without gitdir: %v, want ErrUnheld", err)
	}
}

// rootAbove is the disk, but the folders above the test's own are root's alone, as a computer's folders above a home
// are: the test's temp folder isn't walked (a Mac's holds tens of thousands of other tests' folders). Its /run is a
// stand-in, the same on every computer: root's, holding only containerd's folder, root's alone and closed to the
// owner's account (0700), as a computer with Docker installed has it (VM run 3 B2).
type rootAbove struct{ base string }

// closedRun is the stand-in /run's containerd folder.
const closedRun = "/run/containerd"

func (r rootAbove) stat(p string, follow bool) (viewStat, error) {
	switch {
	case p == "/run":
		return viewStat{mode: fs.ModeDir | 0o755}, nil
	case p == closedRun:
		return viewStat{mode: fs.ModeDir | 0o700}, nil
	case under(p, closedRun):
		return viewStat{}, &fs.PathError{Op: "lstat", Path: p, Err: syscall.EACCES}
	case under(p, "/run"):
		return viewStat{}, fs.ErrNotExist
	case p != r.base && under(r.base, p):
		return viewStat{mode: fs.ModeDir | 0o755}, nil
	}
	return osView{}.stat(p, follow)
}

func (r rootAbove) names(folder string) ([]string, error) {
	switch {
	case folder == "/run":
		return []string{path.Base(closedRun)}, nil
	case under(folder, closedRun):
		return nil, &fs.PathError{Op: "open", Path: folder, Err: syscall.EACCES}
	case under(folder, "/run"):
		return nil, fs.ErrNotExist
	}
	return osView{}.names(folder)
}

func (r rootAbove) real(p string) (string, error) {
	if under(p, "/run") {
		_, err := r.stat(p, false)
		return p, err
	}
	return osView{}.real(p)
}

func (r rootAbove) link(p string) (string, error) {
	if under(p, "/run") {
		return "", fs.ErrInvalid
	}
	return osView{}.link(p)
}

func (r rootAbove) environ() []string { return nil }

// unixSockets is a list with one stand-in socket, gone from a folder root has alone: nothing to mask, and the start
// goes on.
func (r rootAbove) unixSockets() ([]byte, error) {
	return []byte(netUnixHeader + "\n0000000000000000: 00000002 00000000 00010000 0001 01  4242 /run/smolcorp-gone.sock\n"), nil
}

func (r rootAbove) unixDiag() ([]byte, error) {
	return diagDump([]diagEntry{{ino: 4242, name: "/run/smolcorp-gone.sock\x00", vfs: 7, file: true}}), nil
}

// A folder whose only name the Policy denies is a pattern's takes no new name: no name the pattern matches can be made
// there, whatever it is. Today's Policy has no such folder alone (its own folder, its Claude folder, .ssh and the
// worktrees also hold a name it denies by itself), so a one-row Policy shows it.
func TestAPatternsFolderTakesNoNewName(t *testing.T) {
	b := &builder{goos: "linux"}
	b.add(Rule{Row: "a stand-in pattern", Path: lHome + "/box", Tail: "plant*", Write: Deny})
	b.add(Rule{Row: "a stand-in secret's pattern", Path: lHome + "/vault", Tail: "secret*", Read: Deny})
	p := &Policy{goos: "linux", rules: b.rules}
	f := fakeFS{"/": {kind: 'd'}}
	// The system log's socket: a computer's list of sockets is never empty (mask.go).
	f.add(lHome+"/box/kept.txt", lHome+"/notes/todo.txt", lHome+"/vault/kept.txt", "/run/systemd/journal/dev-log@").owners()
	pl := Places{GOOS: "linux", Home: lHome, Support: lSupport, OwnID: "h1", Root: lHome + "/notes"}
	args, err := bwrapArgs(p, pl, View{}, f, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	mounts, _ := playView(t, args)
	for _, at := range []string{lHome + "/box/plant", lHome + "/box/plant-stand-in", lHome + "/box/plants/x"} {
		if says(mounts, at, Write) != Deny {
			t.Errorf("%s can be made:\n%s", at, argvText(args))
		}
	}
	if says(mounts, lHome+"/box/kept.txt", Write) != Allow {
		t.Errorf("the file there, which the pattern doesn't match, isn't brought back writable:\n%s", argvText(args))
	}
	// A pattern the Policy denies reading: its folder is covered, so a secret the owner makes there later isn't seen.
	for _, at := range []string{lHome + "/vault/secret", lHome + "/vault/secret-stand-in"} {
		if says(mounts, at, Read) != Deny {
			t.Errorf("%s, made later, can be read:\n%s", at, argvText(args))
		}
	}
	if says(mounts, lHome+"/vault/kept.txt", Read) != Allow {
		t.Errorf("the file there, which the pattern doesn't match, isn't brought back:\n%s", argvText(args))
	}
}

// VM run 3 B2 (SIERRA's ruling): every place the start looks at a folder, a folder the owner's account can't open,
// root's alone, where no desk can change a name in it or above it (/srv/closed, 0700), is nothing a desk reaches: a
// Policy row in it decides nothing, and the start goes on and says so. Each row reaches it by another way: a name it
// denies reading (consider, then cover), one it denies writing (consider, then shutNewNames), a pattern it denies
// reading (expand's listing), and every new name in it denied writing (bringBack's listing). The same rows in a folder
// of the owner's it can't open (chmod 600), or in a root 0700 folder a desk could rename into place (in the home
// folder), still fail the start: fail-closed for anything a desk can change, as before.
func TestAPlaceTheOwnersAccountCantOpenDecidesNothing(t *testing.T) {
	for _, folder := range []struct {
		at    string
		entry fakeEntry
		ok    bool
	}{
		{"/srv/closed", fakeEntry{kind: 'd', perm: 0o700}, true},
		{lHome + "/locked", fakeEntry{kind: 'd', uid: 1000, perm: 0o600}, false},
		{lHome + "/rootdir", fakeEntry{kind: 'd', perm: 0o700}, false},
	} {
		for _, r := range []struct {
			name string
			rule Rule
		}{
			{"a name it denies reading", Rule{Path: folder.at + "/sub/secret", Read: Deny}},
			{"a name it denies writing", Rule{Path: folder.at + "/sub/plant", Write: Deny}},
			{"a pattern it denies reading", Rule{Path: folder.at, Tail: "secret*", Read: Deny}},
			{"every new name denied writing", Rule{Path: folder.at, Tail: "*", Write: Deny}},
		} {
			b := &builder{goos: "linux"}
			r.rule.Row = "a stand-in row"
			b.add(r.rule)
			p := &Policy{goos: "linux", rules: b.rules}
			f := fakeFS{"/": {kind: 'd'}}
			f.add(folder.at+"/sub/secret", folder.at+"/kept.txt", lHome+"/notes/todo.txt", "/run/systemd/journal/dev-log@").owners()
			f[folder.at] = folder.entry
			pl := Places{GOOS: "linux", Home: lHome, Support: lSupport, OwnID: "h1", Root: lHome + "/notes"}
			var logged []string
			v := View{Log: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }}
			args, err := bwrapArgs(p, pl, v, f, "/tmp")
			switch {
			case folder.ok && err != nil:
				t.Errorf("%s, %s: %v, want the start to go on", folder.at, r.name, err)
			case folder.ok && strings.Contains(argvText(args), folder.at+"/"):
				t.Errorf("%s, %s: the view decides something in it:\n%s", folder.at, r.name, argvText(args))
			case folder.ok && (len(logged) != 1 || !strings.Contains(logged[0], "can't open "+folder.at+",")):
				t.Errorf("%s, %s: logged %q, want %s named once", folder.at, r.name, logged, folder.at)
			case !folder.ok && !errors.Is(err, fs.ErrPermission):
				t.Errorf("%s, %s: %v, want the refusal to fail the start", folder.at, r.name, err)
			}
		}
	}
}

// A covered folder keeps its links the Policy doesn't hold: each is made again in the tmpfs, to where it led (the
// view then decides what is there). A link to nothing is left out, and a link the Policy holds still fails the start.
func TestACoveredFolderKeepsItsLinks(t *testing.T) {
	s := linuxSetups()[0]
	f := linuxComputer()
	f[lHome+"/bin"] = fakeEntry{kind: 'l', to: lHome + "/.local/share", uid: 1000}
	f[lHome+"/old"] = fakeEntry{kind: 'l', to: lHome + "/gone", uid: 1000}
	_, args := viewOf(t, s, f)
	text := argvText(args)
	if !strings.Contains(text, "--symlink "+lHome+"/.local/share "+lHome+"/bin\n") {
		t.Errorf("~/bin isn't made again in the covered home folder:\n%s", text)
	}
	if strings.Contains(text, lHome+"/old") {
		t.Errorf("a link to nothing is in the words:\n%s", text)
	}
	mounts, _ := playView(t, args)
	if says(mounts, lHome+"/bin", Write) != Deny {
		t.Error("~/bin can be replaced")
	}
}

// The seam for an account's "hires" config folder (SIERRA, 14 Nov): View.Owned binds a whole folder back writable for
// the desk, wherever it sits, a covered home folder included, so a refresh by a rename works in it. One the desk
// doesn't own whole by the Policy is refused: the owner's own config folder, its own Claude folder (which holds its
// fence), one in the support folder the Policy gives no desk, a link, a file, a missing or relative one.
func TestTheViewBindsAnOwnedFolderWhole(t *testing.T) {
	s := linuxSetups()[0]
	p := mustNew(t, s.pl)
	hires := lHome + "/.local/share/smolcorp-hires/work"
	f := linuxComputer().add(hires+"/.credentials.json", lHome+"/plain.txt", lSupport+"/accounts/work/").owners()
	f[lHome+"/hires-link"] = fakeEntry{kind: 'l', to: hires, uid: 1000}
	prepare(t, p, s.pl, linuxView, f)
	v := linuxView
	v.Owned = []string{hires}
	args, err := bwrapArgs(p, s.pl, v, f, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(argvText(args), "--bind "+hires+" "+hires+"\n") {
		t.Errorf("the owned folder isn't bound whole:\n%s", argvText(args))
	}
	mounts, _ := playView(t, args)
	for _, at := range []string{hires + "/.credentials.json", hires + "/.credentials.json.tmp", hires} {
		if says(mounts, at, Write) != Allow {
			t.Errorf("%s can't be written in the owned folder", at)
		}
	}
	for _, bad := range []string{lHome + "/.claude", lHome + "/.claude-work", lSupport + "/hires/h1/claude", lSupport + "/accounts/work",
		lHome + "/hires-link", lHome + "/plain.txt", lHome + "/missing", "relative/hires"} {
		v.Owned = []string{bad}
		if _, err := bwrapArgs(p, s.pl, v, f, "/tmp"); !errors.Is(err, ErrNotOwn) {
			t.Errorf("owned %s: %v, want ErrNotOwn", bad, err)
		}
	}
}

// SIERRA's binding check (RT126SAL1R2 N2), with the system's git and stand-ins in the test's own folder: the desk
// reads ~/.gitconfig from the start's copy (here a hard link to it, which reads what a bind of it reads). Once the
// owner's `git config --global` empties the live file by a rename, the desk's git still has the owner's identity and
// signing; with no identity in the owner's config, its commit refuses rather than go out as someone git makes up.
func TestTheGitConfigCopyKeepsTheOwnersIdentityOrRefuses(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	for _, c := range []struct {
		name, config string
		want         map[string]string // `git config --get <key>` → its answer
		commit       string            // what the commit's output must hold; "" when it isn't tried
	}{
		{"the owner signs", "[user]\n\tname = Stand In\n\temail = stand-in@example.invalid\n[commit]\n\tgpgsign = true\n[gpg]\n\tformat = ssh\n",
			map[string]string{"user.email": "stand-in@example.invalid", "commit.gpgsign": "true", "user.useConfigOnly": "true"}, ""},
		{"no identity", "[core]\n\tautocrlf = false\n", map[string]string{"user.useConfigOnly": "true"}, "auto-detection is disabled"},
		{"no global config", "", map[string]string{"user.useConfigOnly": "true"}, "auto-detection is disabled"},
	} {
		base := t.TempDir()
		owner, desk, own, repo := base+"/owner", base+"/desk", base+"/own", base+"/repo"
		for _, folder := range []string{owner, desk, own, repo} {
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		global := owner + "/.gitconfig"
		if c.config != "" {
			if err := os.WriteFile(global, []byte(c.config), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		copy := gitCopy(own, global)
		if err := writeGitCopy(copy, global, true); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(copy, desk+"/.gitconfig"); err != nil {
			t.Fatal(err)
		}
		if c.config != "" { // the owner's git config --global, outside: a new, empty file renamed over the live one
			if err := os.WriteFile(global+".lock", nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(global+".lock", global); err != nil {
				t.Fatal(err)
			}
		}
		git := func(dir string, args ...string) (string, error) {
			cmd := exec.Command(gitPath, append([]string{"-C", dir}, args...)...)
			cmd.Env = []string{"HOME=" + desk, "XDG_CONFIG_HOME=" + desk + "/.config", "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
			out, err := cmd.CombinedOutput()
			return strings.TrimSpace(string(out)), err
		}
		for key, want := range c.want {
			if got, err := git(base, "config", "--global", "--get", key); got != want {
				t.Errorf("%s: %s is %q (%v), want %q", c.name, key, got, err, want)
			}
		}
		if c.commit == "" {
			continue
		}
		if out, err := git(repo, "init", "-q"); err != nil {
			t.Fatalf("%s: git init: %v: %s", c.name, err, out)
		}
		if out, err := git(repo, "commit", "-q", "--allow-empty", "-m", "stand-in"); err == nil || !strings.Contains(out, c.commit) {
			t.Errorf("%s: the commit went through, or refused for another reason: %v: %s", c.name, err, out)
		}
	}
}

// SIERRA's ruling on RT126SAL1R2 N1: of the shared repo's branches and their reflogs, a desk in a worktree of its own
// writes only its own folder, hire/<id>/, which it can't move; every other branch, its reflog, a new branch beside
// them and another desk's folder are read-only, whatever the Policy lets it write there, and so is packed-refs. A
// desk with no worktree of its own writes no branch. logs/refs can't be moved away with the read-only folder in it.
func TestOnlyItsOwnBranchFolderIsWritable(t *testing.T) {
	for _, s := range linuxSetups() {
		for _, worktree := range []bool{true, false} {
			if !worktree {
				s.pl.OwnWorktree, s.pl.Checkout, s.pl.Root = "", "", lRepo
			}
			_, args := viewOf(t, s, linuxComputer())
			mounts, _ := playView(t, args)
			own := []string{lGit + "/refs/heads/hire/" + s.pl.OwnID, lGit + "/logs/refs/heads/hire/" + s.pl.OwnID}
			for _, at := range []string{own[0] + "/work", own[0] + "/new.lock", own[1] + "/work", own[1] + "/new"} {
				if (says(mounts, at, Write) == Allow) != worktree {
					t.Errorf("%s (worktree %v): its own %s: %v", s.name, worktree, at, says(mounts, at, Write))
				}
			}
			for _, at := range append(own, lGit+"/logs/refs") {
				if worktree && says(mounts, at, Move) != Deny {
					t.Errorf("%s: %s can be moved", s.name, at)
				}
			}
			for _, at := range []string{lGit + "/refs/heads/h1", lGit + "/refs/heads/h1.lock", lGit + "/refs/heads/feature", lGit + "/refs/heads/main",
				lGit + "/refs/heads/hire/h2/work", lGit + "/refs/heads/hire/h2/new", lGit + "/refs/heads/hire/stand-in", lGit + "/refs/heads/hire",
				lGit + "/logs/refs/heads/main", lGit + "/logs/refs/heads/feature", lGit + "/logs/refs/heads/hire/h2", lGit + "/packed-refs",
				lGit + "/packed-refs.lock", lGit + "/refs/heads", lGit + "/logs/refs/heads"} {
				if inside(at, own...) {
					continue
				}
				if says(mounts, at, Write) != Deny {
					t.Errorf("%s (worktree %v): %s can be written", s.name, worktree, at)
				}
			}
		}
	}
}

// An owner with no ~/.gitconfig still gets the copy at that place (RT126SAL1R2 N2): it holds only useConfigOnly, so a
// desk's commit refuses rather than go out as someone git makes up, whatever the owner makes there later.
func TestTheGlobalGitConfigIsACopyEvenWhenMissing(t *testing.T) {
	for _, s := range linuxSetups() {
		f := linuxComputer()
		delete(f, lHome+"/.gitconfig")
		_, args := viewOf(t, s, f)
		mounts, _ := playView(t, args)
		if !slices.Contains(mounts, mounted{"copy", lHome + "/.gitconfig"}) {
			t.Errorf("%s: no copy is bound at ~/.gitconfig:\n%s", s.name, argvText(args))
		}
	}
}

// The start copies only a plain file: a git config file that is a link (to a stand-in key, here) or a folder fails
// the start, and nothing of it is copied where the desk reads it.
func TestAGitConfigCopyIsOnlyOfAPlainFile(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(base+"/stand-in-key", []byte("stand-in secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(base+"/stand-in-key", base+"/link"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(base+"/folder", 0o700); err != nil {
		t.Fatal(err)
	}
	for _, at := range []string{base + "/link", base + "/folder"} {
		copy := gitCopy(base, at)
		if err := writeGitCopy(copy, at, true); err == nil {
			t.Errorf("%s was copied", at)
		}
		if _, err := os.Lstat(copy); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("a copy of %s is made: %v", at, err)
		}
	}
}
