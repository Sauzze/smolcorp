package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// SA-L1's CI row (PL126A §4): the view starts on the system's bwrap, with a desk's places made of stand-ins in the
// test's own folder, and from inside this test program, started again as the desk, sees what SA-X3's enter checks:
// the support folder a tmpfs on another device, the canary absent, the fences refused with EROFS, its own brief read
// and status written. And: a stand-in key and X11's key refused, its .claude can't be renamed, its desk-start socket
// answers, /tmp is private, the pid namespace is its own. FXSAL1R1: a name the Policy denies that isn't there (a plant,
// the shared .git's commondir, a new brief, a base branch) can't be made, nor a .git above the checkout, nor the
// checkout's .git's mode changed (B1, B2), while git's own new names (a branch's lock, an object's folder) and a new
// file in its place still are. RT126SAL1R2 N1: a branch's lock is made only in its own branch folder. The folder made private is the test's own, not /tmp, where `go test` keeps this
// program; on CI the stand-ins go under $RUNNER_TEMP, so the folders above them, which take no new name in the view,
// aren't /tmp's. With SMOLCORP_LINUXCAPS_REQUIRED=1 (the linuxcaps CI job) a missing bwrap fails.

const viewStandIn = "SMOLCORP_VIEW_STANDIN" // the places to try, as JSON: set only in the stand-in

// viewWait names a folder the stand-in writes "ready" in, then waits for "go" in before its rows: the test changes
// files outside meanwhile.
const viewWait = "SMOLCORP_VIEW_WAIT"

// viewTry is what the stand-in tries: each of its rows, by name, on a path.
type viewTry struct {
	SupportDev uint64            `json:"supportDev"`
	Support    string            `json:"support"`
	Rows       map[string]string `json:"rows"` // row name → path; the name says what is done
}

// The stand-in: only runs inside the view.
func TestTheLinuxViewStandIn(t *testing.T) {
	var try viewTry
	if err := json.Unmarshal([]byte(os.Getenv(viewStandIn)), &try); err != nil || try.Support == "" {
		t.Skip("runs only as the view's stand-in")
	}
	got := map[string]string{}
	var fsStat unix.Statfs_t
	var st unix.Stat_t
	switch {
	case unix.Statfs(try.Support, &fsStat) != nil:
		got["support: a tmpfs"] = "can't be looked at"
	case fsStat.Type != unix.TMPFS_MAGIC:
		got["support: a tmpfs"] = fmt.Sprintf("%#x", fsStat.Type)
	default:
		got["support: a tmpfs"] = "ok"
	}
	if err := unix.Stat(try.Support, &st); err != nil || uint64(st.Dev) == try.SupportDev {
		got["support: another device"] = fmt.Sprint("the same device, or ", err)
	} else {
		got["support: another device"] = "ok"
	}
	if folder := os.Getenv(viewWait); folder != "" {
		if err := os.WriteFile(folder+"/ready", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 300; i++ {
			if _, err := os.Stat(folder + "/go"); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	for name, at := range try.Rows {
		var err error
		switch {
		case strings.HasPrefix(name, "readtext "):
			var data []byte
			if data, err = os.ReadFile(at); err == nil {
				got[name] = "text:" + string(data)
				continue
			}
		case strings.HasPrefix(name, "read "):
			var data []byte
			if data, err = os.ReadFile(at); err == nil && len(data) == 0 {
				got[name] = "empty"
				continue
			}
		case strings.HasPrefix(name, "write "):
			var f *os.File
			if f, err = os.OpenFile(at, os.O_WRONLY|os.O_APPEND, 0); err == nil {
				_, err = f.WriteString("stand-in line\n")
				f.Close()
			}
		case strings.HasPrefix(name, "rename "):
			err = os.Rename(at, at+".moved")
		case strings.HasPrefix(name, "renameover "), strings.HasPrefix(name, "rmdir "): // the kernel's own answer
			_, err = folderRow(name, at)
		case strings.HasPrefix(name, "symlink "):
			err = os.Symlink("/", at)
		case strings.HasPrefix(name, "create "):
			var f *os.File
			if f, err = os.OpenFile(at, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
				f.Close()
			}
		case strings.HasPrefix(name, "mkdir "):
			err = os.Mkdir(at, 0o700)
		case strings.HasPrefix(name, "chmod "):
			if err = os.Chmod(at, 0); err == nil {
				os.Chmod(at, 0o700) // the view let it through: put it back, so the test's folder can be removed
			}
		case strings.HasPrefix(name, "connect "):
			var c net.Conn
			if c, err = net.Dial("unix", at); err == nil {
				c.Close()
			}
		case strings.HasPrefix(name, "git "): // at is the folder, then git's words, a line each
			words := strings.Split(at, "\n")
			out, gitErr := exec.Command("git", append([]string{"-C", words[0]}, words[1:]...)...).CombinedOutput()
			t.Logf("%s: %v\n%s", name, gitErr, out)
			if gitErr == nil {
				got[name] = "ok:" + strings.TrimSpace(string(out))
			} else {
				got[name] = "fail"
			}
			continue
		// SIERRA (FXL1APPARMOR1R2), a hire stays blocked: what runs in the view runs a plain program (the positive
		// control: with RT126L1APPARMOR1R2 B2's px, every exec here was refused), holds no capability, has
		// no_new_privs, and can make no user namespace, by itself or through Smolcorp's bwrap again.
		case strings.HasPrefix(name, "exec:"):
			err = exec.Command(at).Run()
		case strings.HasPrefix(name, "userns: a nested one"):
			cmd := exec.Command(at)
			cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUSER}
			err = cmd.Run()
		case strings.HasPrefix(name, "userns: Smolcorp's bwrap again"):
			out, bwrapErr := exec.Command(at, "--unshare-user", "--ro-bind", "/", "/", "true").CombinedOutput()
			t.Logf("%s: %v\n%s", name, bwrapErr, out)
			got[name] = map[bool]string{true: "ok", false: "fail"}[bwrapErr == nil]
			continue
		case strings.HasPrefix(name, "status:"): // at is a line of /proc/self/status, each such line as it must read
			var data []byte
			if data, err = os.ReadFile("/proc/self/status"); err == nil {
				for _, want := range strings.Split(at, "\n") {
					key, _, _ := strings.Cut(want, ":")
					for _, line := range strings.Split(string(data), "\n") {
						if strings.HasPrefix(line, key+":") && strings.Join(strings.Fields(line), " ") != want {
							err = errors.New(line)
						}
					}
				}
			}
		case strings.HasPrefix(name, "pid namespace"):
			var link string
			if link, err = os.Readlink("/proc/self/ns/pid"); err == nil && link == at {
				err = errors.New("the same as outside: " + link)
			}
		}
		got[name] = errName(err)
	}
	line, _ := json.Marshal(got)
	fmt.Println("view-rows: " + string(line))
}

// errName is "ok", an errno's name, or the error.
func errName(err error) string {
	var errno syscall.Errno
	switch {
	case err == nil:
		return "ok"
	case errors.As(err, &errno):
		return unix.ErrnoName(errno)
	}
	return err.Error()
}

// L1APPARMOR1's row for VM run 2: Smolcorp's own bwrap, put in place by its one command, makes a user, pid, mount and
// network namespace in one start: the uid map and the loopback that VM run 1 refused. And RT126L1APPARMOR1 B1's two
// facts, what any program the copy starts gets: where the restriction is on, what runs inside is under both
// profiles, stacked (smolcorp-bwrap//&smolcorp-bwrap-child), and wherever it is, it holds no capability and can't
// make a user namespace of its own (unshare -Ur fails). With SMOLCORP_LINUXCAPS_REQUIRED=1 a bwrap or profile not in
// place fails.
func TestSmolcorpsBwrapMakesEveryNamespace(t *testing.T) {
	required := os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1"
	if err := CheckBwrap(); err != nil {
		if required {
			t.Fatalf("Smolcorp's bwrap isn't in place (run: smolcorp sandbox command): %v", err)
		}
		t.Skip("Smolcorp's bwrap isn't in place here: ", err)
	}
	inside := func(command ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		words := append([]string{"--unshare-user", "--unshare-pid", "--unshare-net", "--ro-bind", "/", "/", "--dev", "/dev",
			"--proc", "/proc", "--"}, command...)
		out, err := exec.CommandContext(ctx, BwrapPath, words...).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	label, err := inside("cat", "/proc/self/attr/current")
	if err != nil {
		t.Fatalf("Smolcorp's bwrap didn't make the namespaces: %v\n%s", err, label)
	}
	t.Logf("inside, AppArmor's label: %s", label)
	stacked := stackedUnderBoth(label)
	if on, _ := os.ReadFile("/" + restrictFile); strings.TrimSpace(string(on)) == "1" && !stacked {
		t.Errorf("the restriction is on, and what runs inside isn't under %s and %s, enforcing: %q", AppArmorName, AppArmorChild, label)
	}
	status, err := inside("grep", "^CapEff:", "/proc/self/status")
	if err != nil || strings.Trim(strings.TrimPrefix(status, "CapEff:"), " \t0") != "" {
		t.Errorf("what runs inside holds capabilities: %v %q", err, status)
	}
	unshare, err := exec.LookPath("unshare")
	if err != nil {
		if required {
			t.Fatal("no unshare here to try a nested user namespace: ", err)
		}
		t.Skip("no unshare here: ", err)
	}
	out, err := inside(unshare, "-Ur", "true")
	t.Logf("a nested user namespace from inside: %v %s", err, out)
	if stacked && err == nil {
		t.Errorf("what runs inside, under %s, made a user namespace of its own", AppArmorChild)
	}
}

func TestTheLinuxViewStartsOnBwrap(t *testing.T) {
	if _, err := os.Stat(BwrapPath); err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	if err := CheckBwrap(); err != nil {
		t.Fatalf("Smolcorp's bwrap fails the start's check (smolcorp sandbox command puts it in place): %v", err)
	}
	base := t.TempDir()
	if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
		var err error
		if base, err = os.MkdirTemp(runner, "smolcorp-view-"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	repo := filepath.Join(home, "Projects", "app")
	hires := filepath.Join(home, "Projects", "app-hires")
	root := filepath.Join(hires, "h1")
	reg := repo + "/.git/worktrees/h1"
	private := filepath.Join(base, "tmp")
	files := map[string]string{
		support + "/phones.json": "stand-in secret", support + "/hire-guard.json": "{}", support + "/sandbox-canary-1": "canary",
		support + "/hires/h1/BRIEF.md": "brief", support + "/hires/h1/STATUS.md": "status",
		root + "/.claude/settings.json": "{}", root + "/src/main.go": "package main", repo + "/.git/HEAD": "ref: refs/heads/main\n",
		root + "/.git": "gitdir: " + repo + "/.git/worktrees/h1\n", repo + "/.git/worktrees/h1/commondir": "../..\n",
		repo + "/.git/worktrees/h1/gitdir": root + "/.git\n", repo + "/.git/refs/heads/main": "0000000000000000000000000000000000000000\n",
		repo + "/.git/config": "", home + "/.Xauthority": "stand-in", home + "/.ssh/id_ed25519": "stand-in key",
		private + "/outside.txt": "made outside",
	}
	for at, text := range files {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, folder := range []string{repo + "/.git/objects", repo + "/.git/refs/heads"} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	deskStart := filepath.Join(support, "desk-start")
	listener, err := net.Listen("unix", deskStart)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: deskStart, Root: root,
		Checkout: repo, GitDir: repo + "/.git", OwnWorktree: "h1", HiresFolder: hires, RemoteLogin: true}
	args, err := prepareBwrap(pl, View{}, osView{}, private)
	if err != nil {
		t.Fatal(err)
	}
	var st unix.Stat_t
	if err := unix.Stat(support, &st); err != nil {
		t.Fatal(err)
	}
	pidNS, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"support: a tmpfs": "ok", "support: another device": "ok",
		"read the canary": "ENOENT", "read phones.json": "ENOENT",
		"read the guard file": "ok", "write the guard file": "EROFS",
		"write its place's .claude/settings.json": "EROFS", "rename its place's .claude": "EBUSY",
		"read its brief": "ok", "write its brief": "EROFS", "write its status": "ok",
		"connect desk-start": "ok", "read X11's key": "ENOENT", "read a stand-in ssh key": "ENOENT",
		"read a file in the private /tmp, made outside": "ENOENT", "pid namespace: its own": "ok",
		"write its place": "ok", "write the owner's checkout's .git/config": "EROFS",
		"create a plant the owner doesn't have": "EROFS", "create the shared .git's commondir": "EROFS",
		"create a second brief in its own folder": "EROFS", "create a base branch the repo doesn't have": "EROFS",
		"mkdir a .git above the checkout": "EROFS", "chmod the checkout's .git": "EROFS",
		"create a branch's lock file in its own folder": "ok", "create a branch's lock file outside it": "EROFS",
		"mkdir an object's folder": "ok", "create a new file in its place": "ok",
		// X12LINK1's old remote folders (MSYSAL1): its registration's, held by the start's placeholders, and the shared
		// .git's, in its read-only folder. FXMSYSAL1R1: "renameover" and "rmdir" ask the kernel (folderRow), which
		// answers EBUSY only on a mount point; on a plain folder both go through (TestTheFolderRowsGoThroughOnAPlainFolder).
		"create a lock file in its registration": "ok", "create in its registration's branches": "EROFS",
		"create in its registration's remotes": "EROFS", "symlink in its registration's remotes": "EROFS",
		"rename its registration's branches": "EBUSY", "renameover its registration's remotes": "EBUSY",
		"rmdir its registration's branches": "EBUSY", "chmod its registration's branches": "EROFS",
		"mkdir the shared .git's branches": "EROFS", "mkdir the shared .git's remotes": "EROFS",
		"exec: a plain program": "ok", "status: no capability held": "ok", "status: no_new_privs": "ok",
	}
	// Where the restriction is on, the hire is under smolcorp-bwrap-child, which denies any user namespace.
	if on, _ := os.ReadFile("/" + restrictFile); strings.TrimSpace(string(on)) == "1" {
		want["userns: a nested one"] = "EACCES or EPERM"
		want["userns: Smolcorp's bwrap again"] = "fail"
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{
		"read the canary": support + "/sandbox-canary-1", "read phones.json": support + "/phones.json",
		"read the guard file": support + "/hire-guard.json", "write the guard file": support + "/hire-guard.json",
		"write its place's .claude/settings.json": root + "/.claude/settings.json", "rename its place's .claude": root + "/.claude",
		"read its brief": support + "/hires/h1/BRIEF.md", "write its brief": support + "/hires/h1/BRIEF.md",
		"write its status": support + "/hires/h1/STATUS.md", "connect desk-start": deskStart,
		"read X11's key": home + "/.Xauthority", "read a stand-in ssh key": home + "/.ssh/id_ed25519",
		"read a file in the private /tmp, made outside": private + "/outside.txt", "pid namespace: its own": pidNS,
		"write its place": root + "/src/main.go", "write the owner's checkout's .git/config": repo + "/.git/config",
		"create a plant the owner doesn't have": home + "/.zshenv", "create the shared .git's commondir": repo + "/.git/commondir",
		"create a second brief in its own folder":    support + "/hires/h1/BRIEF-2.md",
		"create a base branch the repo doesn't have": repo + "/.git/refs/heads/master",
		"mkdir a .git above the checkout":            home + "/Projects/.git", "chmod the checkout's .git": repo + "/.git",
		"create a branch's lock file in its own folder": repo + "/.git/refs/heads/hire/h1/work.lock",
		"create a branch's lock file outside it":        repo + "/.git/refs/heads/h1.lock", "mkdir an object's folder": repo + "/.git/objects/cd",
		"create a new file in its place":         root + "/src/new.go",
		"create a lock file in its registration": reg + "/index.lock", "create in its registration's branches": reg + "/branches/origin",
		"create in its registration's remotes": reg + "/remotes/origin", "symlink in its registration's remotes": reg + "/remotes/up",
		"rename its registration's branches": reg + "/branches", "renameover its registration's remotes": reg + "/remotes",
		"rmdir its registration's branches": reg + "/branches", "chmod its registration's branches": reg + "/branches",
		"mkdir the shared .git's branches": repo + "/.git/branches", "mkdir the shared .git's remotes": repo + "/.git/remotes",
		"exec: a plain program": truePath, "userns: a nested one": truePath, "userns: Smolcorp's bwrap again": BwrapPath,
		"status: no capability held": "CapPrm: 0000000000000000\nCapEff: 0000000000000000\nCapAmb: 0000000000000000",
		"status: no_new_privs":       "NoNewPrivs: 1",
	}
	try, err := json.Marshal(viewTry{SupportDev: uint64(st.Dev), Support: support, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, BwrapPath, append(args, self, "-test.run=^TestTheLinuxViewStandIn$", "-test.v")...)
	cmd.Env = append(os.Environ(), viewStandIn+"="+string(try))
	out, err := cmd.CombinedOutput()
	t.Logf("bwrap %s\n%s", strings.Join(args, " "), out)
	if err != nil {
		t.Fatalf("the view didn't start, or the stand-in failed: %v", err)
	}
	_, line, ok := strings.Cut(string(out), "view-rows: ")
	if !ok {
		t.Fatal("the stand-in reported nothing")
	}
	line, _, _ = strings.Cut(line, "\n")
	var got map[string]string
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		if !slices.Contains(strings.Split(w, " or "), got[name]) {
			t.Errorf("%s: got %s, want %s", name, got[name], w)
		}
	}
	if data, err := os.ReadFile(support + "/hires/h1/STATUS.md"); err != nil || !strings.Contains(string(data), "stand-in line") {
		t.Errorf("its status, written inside, isn't on the disk: %q, %v", data, err)
	}
	if data, _ := os.ReadFile(root + "/src/main.go"); !strings.Contains(string(data), "stand-in line") {
		t.Errorf("its place, written inside, isn't on the disk: %q", data)
	}
}

// SIERRA's call 2 (FXSAL1R1): since Linux 3.18, a program in the owner's namespace that replaces a file by a rename,
// or removes it and makes it again, takes away every mount on that file in other namespaces. A stand-in desk waits
// inside; outside, a stand-in X11 key is replaced by a rename and a stand-in sign-in is removed and made again; then
// the desk reads both. With the old per-file masks (/dev/null over each, the control) it reads the new files: the
// kernel does let go. In the view, their folders are a tmpfs with only what the Policy allows brought back: ENOENT.
func TestTheViewHidesAKeyReplacedOutside(t *testing.T) {
	if _, err := os.Stat(BwrapPath); err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	if err := CheckBwrap(); err != nil {
		t.Fatalf("Smolcorp's bwrap fails the start's check (smolcorp sandbox command puts it in place): %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const keyRow, signInRow = "readtext X11's key, replaced by a rename outside", "readtext a sign-in, removed and made again outside"
	for _, perFileMasks := range []bool{true, false} {
		base := t.TempDir()
		if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
			if base, err = os.MkdirTemp(runner, "smolcorp-view-"); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(base) })
		}
		home := filepath.Join(base, "home")
		support := filepath.Join(home, ".config", "sauzzedeck-pro")
		root := filepath.Join(home, "Projects", "h1")
		private, sync := filepath.Join(base, "tmp"), filepath.Join(root, "sync")
		key, signIn := home+"/.Xauthority", home+"/.codex/auth.json"
		for at, text := range map[string]string{key: "stand-in key 1", signIn: "stand-in sign-in 1", support + "/hire-guard.json": "{}",
			support + "/hires/h1/BRIEF.md": "brief", root + "/src/main.go": "package main", private + "/x": ""} {
			if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(sync, 0o700); err != nil {
			t.Fatal(err)
		}
		pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", Root: root}
		args, err := prepareBwrap(pl, View{}, osView{}, private)
		if err != nil {
			t.Fatal(err)
		}
		if perFileMasks {
			args = []string{"--unshare-pid", "--bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--tmpfs", private,
				"--ro-bind", "/dev/null", key, "--ro-bind", "/dev/null", signIn, "--chdir", root, "--"}
		}
		try, err := json.Marshal(viewTry{Support: support, Rows: map[string]string{keyRow: key, signInRow: signIn}})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, BwrapPath, append(args, self, "-test.run=^TestTheLinuxViewStandIn$", "-test.v")...)
		cmd.Env = append(os.Environ(), viewStandIn+"="+string(try), viewWait+"="+sync)
		var out strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		ready := false
		for i := 0; i < 300 && !ready; i++ {
			_, err := os.Stat(sync + "/ready")
			if ready = err == nil; !ready {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if !ready {
			cmd.Wait()
			t.Fatalf("masks %v: the stand-in never got ready:\n%s", perFileMasks, out.String())
		}
		// Outside, as a program of the owner's does it: a new key written beside and renamed over; the sign-in removed and
		// made again.
		if err := os.WriteFile(key+".new", []byte("stand-in key 2"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(key+".new", key); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(signIn); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(signIn, []byte("stand-in sign-in 2"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sync+"/go", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		err = cmd.Wait()
		t.Logf("masks %v: bwrap %s\n%s", perFileMasks, strings.Join(args, " "), out.String())
		if err != nil {
			t.Fatalf("masks %v: the view didn't start, or the stand-in failed: %v", perFileMasks, err)
		}
		_, line, ok := strings.Cut(out.String(), "view-rows: ")
		if !ok {
			t.Fatalf("masks %v: the stand-in reported nothing", perFileMasks)
		}
		line, _, _ = strings.Cut(line, "\n")
		var got map[string]string
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{keyRow: "ENOENT", signInRow: "ENOENT"}
		if perFileMasks {
			want = map[string]string{keyRow: "text:stand-in key 2", signInRow: "text:stand-in sign-in 2"}
		}
		for row, w := range want {
			switch {
			case got[row] == w:
			case perFileMasks:
				t.Errorf("the control: %s: got %s, want %s (the kernel kept the mask: the premise of call 2 doesn't hold here)", row, got[row], w)
			default:
				t.Errorf("the view: %s: got %s, want %s", row, got[row], w)
			}
		}
	}
}

// RT126SAL1R2 N1 and N2, with SIERRA's rulings, on the system's bwrap and git, with stand-ins in the test's own folder:
// a hire's worktree on hire/h1/work, and a stand-in owner's global config that signs (its key a stand-in path, which
// no ssh-keygen can load) and names its email in an included file. While the stand-in desk waits inside, the owner's
// programs, outside: replace main by a rename, make master, rewrite packed-refs, and empty ~/.gitconfig and its
// include by renames. Inside, a write to each branch fails, and so do a desk's pack-refs and gc; its own branch
// folder and reflog folder still take new names; its git still has the owner's email and signing, and its commit
// refuses rather than go out unsigned. Outside, nothing the desk tried has changed.
func TestTheViewHoldsBranchesAndGitConfigReplacedOutside(t *testing.T) {
	if _, err := os.Stat(BwrapPath); err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	if err := CheckBwrap(); err != nil {
		t.Fatalf("Smolcorp's bwrap fails the start's check (smolcorp sandbox command puts it in place): %v", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("no git on this runner: %v", err)
	}
	base := t.TempDir()
	if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
		if base, err = os.MkdirTemp(runner, "smolcorp-view-"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	repo := filepath.Join(home, "Projects", "app")
	hires := filepath.Join(home, "Projects", "app-hires")
	root, git := filepath.Join(hires, "h1"), filepath.Join(repo, ".git")
	private, sync := filepath.Join(base, "tmp"), filepath.Join(base, "sync")
	env := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home + "/.config", "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	for at, text := range map[string]string{
		home + "/.gitconfig": "[user]\n\tname = Stand In\n\tsigningkey = " + home + "/.ssh/stand-in-missing.pub\n[commit]\n\tgpgsign = true\n" +
			"[gpg]\n\tformat = ssh\n[include]\n\tpath = ~/.gitconfig-work\n",
		home + "/.gitconfig-work": "[user]\n\temail = stand-in@example.invalid\n", support + "/hire-guard.json": "{}",
		support + "/hires/h1/BRIEF.md": "brief", private + "/x": "", sync + "/x": "",
	} {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The owner's git, outside, on its throwaway repo: commits there aren't signed (the owner's OK for test repos).
	owner := func(dir string, args ...string) string {
		cmd := exec.Command(gitPath, append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	owner(repo, "init", "-q", "-b", "main")
	owner(repo, "commit", "-q", "--allow-empty", "-m", "one")
	owner(repo, "worktree", "add", "-q", "-b", HireBranch("h1", "work"), root)
	tip := owner(repo, "rev-parse", "main")

	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", DefaultBranch: "main", HiresFolder: hires, GitNamed: []string{home + "/.gitconfig-work"}}
	args, err := prepareBwrap(pl, View{}, osView{}, private)
	if err != nil {
		t.Fatal(err)
	}
	gitRow := func(words ...string) string { return strings.Join(append([]string{root}, words...), "\n") }
	rows := map[string]string{
		"write main, replaced outside by a rename": git + "/refs/heads/main", "create main's lock": git + "/refs/heads/main.lock",
		"write master, made outside": git + "/refs/heads/master", "write packed-refs, rewritten outside": git + "/packed-refs",
		"create packed-refs' lock": git + "/packed-refs.lock", "mkdir another desk's branch folder": git + "/refs/heads/hire/h2",
		"write main's reflog": git + "/logs/refs/heads/main", "create a lock in its own branch folder": git + "/refs/heads/hire/h1/other.lock",
		"create a reflog in its own folder": git + "/logs/refs/heads/hire/h1/other",
		"git user.email":                    gitRow("config", "--get", "user.email"), "git commit.gpgsign": gitRow("config", "--get", "commit.gpgsign"),
		"git user.useConfigOnly": gitRow("config", "--get", "user.useConfigOnly"), "git commit": gitRow("commit", "--allow-empty", "-m", "unsigned?"),
		"git pack-refs": gitRow("pack-refs", "--all"), "git gc": gitRow("gc"),
	}
	want := map[string]string{
		"write main, replaced outside by a rename": "EROFS", "create main's lock": "EROFS", "write master, made outside": "EROFS",
		"write packed-refs, rewritten outside": "EROFS", "create packed-refs' lock": "EROFS", "mkdir another desk's branch folder": "EROFS",
		"write main's reflog": "EROFS", "create a lock in its own branch folder": "ok", "create a reflog in its own folder": "ok",
		"git user.email": "ok:stand-in@example.invalid", "git commit.gpgsign": "ok:true", "git user.useConfigOnly": "ok:true",
		"git commit": "fail", "git pack-refs": "fail", "git gc": "fail",
	}
	try, err := json.Marshal(viewTry{Support: support, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, BwrapPath, append(args, self, "-test.run=^TestTheLinuxViewStandIn$", "-test.v")...)
	cmd.Env = append(append(os.Environ(), env...), viewStandIn+"="+string(try), viewWait+"="+sync)
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready := false
	for i := 0; i < 300 && !ready; i++ {
		_, err := os.Stat(sync + "/ready")
		if ready = err == nil; !ready {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !ready {
		cmd.Wait()
		t.Fatalf("the stand-in never got ready:\n%s", out.String())
	}
	// Outside, as the owner's programs do it: each a new file renamed over the old, and a new branch.
	packed := "# pack-refs with: peeled fully-peeled sorted \n" + tip + " refs/heads/release\n"
	for at, text := range map[string]string{git + "/refs/heads/main": tip + "\n", git + "/packed-refs": packed,
		home + "/.gitconfig": "", home + "/.gitconfig-work": ""} {
		if err := os.WriteFile(at+".new", []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(at+".new", at); err != nil {
			t.Fatal(err)
		}
	}
	owner(repo, "branch", "master")
	if err := os.WriteFile(sync+"/go", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	t.Logf("bwrap %s\n%s", strings.Join(args, " "), out.String())
	if err != nil {
		t.Fatalf("the view didn't start, or the stand-in failed: %v", err)
	}
	_, line, ok := strings.Cut(out.String(), "view-rows: ")
	if !ok {
		t.Fatal("the stand-in reported nothing")
	}
	line, _, _ = strings.Cut(line, "\n")
	var got map[string]string
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %s, want %s", name, got[name], w)
		}
	}
	for at, text := range map[string]string{git + "/refs/heads/main": tip + "\n", git + "/packed-refs": packed, git + "/refs/heads/master": tip + "\n"} {
		if data, err := os.ReadFile(at); err != nil || string(data) != text {
			t.Errorf("%s changed: %q, %v", at, data, err)
		}
	}
	if now := owner(repo, "rev-parse", HireBranch("h1", "work")); now != tip {
		t.Errorf("a commit went onto the desk's branch while its signing config was emptied outside: %s", owner(repo, "log", "--format=%H %G? %ae", "-1", now))
	}
}
