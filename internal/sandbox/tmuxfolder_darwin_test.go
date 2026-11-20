package sandbox

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXTMUXDIR on this Mac, with stand-ins only: tmux's own folder (/private/tmp/tmux-<uid>) holds the plain hires'
// servers, the team grid's, the desks' started outside the sandbox and the owner's own `default`, and the core, the Mac
// app and the owner's tmux connect to them by their paths. The real profile, made by PrepareProfile for a stand-in desk
// whose places are all in the test's temp folder, its tmux folder a stand-in there too (never the owner's real one, nor
// its servers), run by /usr/bin/sandbox-exec. A stand-in desk (python) inside tries what the Part 1 probe found it
// could do there: write, make a folder, unlink, rename in, out and the folder itself, a hard link, a symlink, bind a
// socket, and the flip of ruling (7) (a plain hire's server removed and a link to another desk's socket put in its
// place). With the folder's place, every one is refused, and the desk-start socket in it still connects. Without it
// (the Policy before FXTMUXDIR), each is done, so the row fails if the folder's row goes. Each check is "ok" or the
// error's name.

// tmuxFolderProbe does each step of its JSON argument, a list of [what, path, (path)], and prints a JSON list of "ok"
// or the error's name, one per step.
const tmuxFolderProbe = `import json, os, socket, sys
said = []
for step in json.loads(sys.argv[1]):
    what, at = step[0], step[1]
    try:
        if what == "write":
            with open(at, "w") as f:
                f.write("standin")
        elif what == "mkdir":
            os.mkdir(at)
        elif what == "unlink":
            os.unlink(at)
        elif what == "rename":
            os.rename(at, step[2])
        elif what == "link":
            os.link(at, step[2])
        elif what == "symlink":
            os.symlink(step[2], at)
        elif what == "bind":
            s = socket.socket(socket.AF_UNIX)
            s.bind(at)
            s.close()
        elif what == "connect":
            s = socket.socket(socket.AF_UNIX)
            s.connect(at)
            s.close()
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// tmuxFolderStep is one try of the stand-in desk's: what it does, where, and what it should get with the folder's
// place and without it.
type tmuxFolderStep struct {
	what, at, to  string
	with, without string
}

// tmuxFolderRun makes a stand-in desk d1 in a temp folder of its own, its tmux folder a stand-in with a plain hire's
// server, a `default` and the desk-start socket listening in it, writes its real profile (with tmux folder's place when
// withPlace), runs steps inside it, and gives what each said, the folder's place, and the temp folder.
func tmuxFolderRun(t *testing.T, python string, withPlace bool, steps func(base, tmux, sockets string) []tmuxFolderStep) ([]tmuxFolderStep, []string, string) {
	t.Helper()
	// A short folder: a unix socket's path must fit in 104 bytes.
	made, err := os.MkdirTemp("", "tmd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "h")
	support := filepath.Join(home, "L", "AD")
	own := filepath.Join(support, "hires", "d1")
	root := filepath.Join(home, "P", "d1")
	sockets := filepath.Join(base, "k")
	tmux := filepath.Join(base, "tmux-"+strconv.Itoa(os.Getuid()))
	for _, dir := range []string{own, root, filepath.Join(sockets, "d1"), filepath.Join(sockets, "cd34"), tmux, filepath.Join(base, "elsewhere")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(tmux, "plainfile"), filepath.Join(tmux, "renameme"), filepath.Join(tmux, "linkme"), filepath.Join(base, "elsewhere", "planted")} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The stand-in servers: a plain hire's and the owner's `default` in the tmux folder, another desk's in the socket
	// folder, and the desk-start socket in the tmux folder, as the core puts it when its path is too long elsewhere.
	deskStart := filepath.Join(tmux, "smolcorp.desk-start.1")
	for _, at := range []string{filepath.Join(tmux, "smolcorp-plain"), filepath.Join(tmux, "default"), filepath.Join(sockets, "cd34", "s"), deskStart} {
		l, err := net.Listen("unix", at)
		if err != nil {
			t.Fatal(err)
		}
		l.(*net.UnixListener).SetUnlinkOnClose(false)
		t.Cleanup(func() { l.Close() })
	}
	pl := Places{GOOS: "darwin", Home: home, Support: support, OwnID: "d1", Root: root, DeskStart: deskStart,
		SocketParent: sockets, OwnSockets: filepath.Join(sockets, "d1")}
	if withPlace {
		pl.TmuxFolder = tmux
	}
	p := mustNew(t, pl)
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file, Services{OwnSocket: filepath.Join(sockets, "d1", "s")}); err != nil {
		t.Fatal(err)
	}
	list := steps(base, tmux, sockets)
	var args [][]string
	for _, s := range list {
		arg := []string{s.what, s.at}
		if s.to != "" {
			arg = append(arg, s.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", file, python, "-I", "-c", tmuxFolderProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(list) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	return list, said, base
}

func TestOnAMacADeskCantWriteLinkOrBindInTmuxsOwnFolder(t *testing.T) {
	python := python3(t)
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	const ok, no = "ok", "PermissionError"
	steps := func(base, tmux, sockets string) []tmuxFolderStep {
		return []tmuxFolderStep{
			{"write", tmux + "/newfile", "", no, ok},
			{"mkdir", tmux + "/newdir", "", no, ok},
			{"unlink", tmux + "/plainfile", "", no, ok},
			{"rename", tmux + "/renameme", tmux + "/renamed", no, ok},
			{"rename", base + "/elsewhere/planted", tmux + "/planted", no, ok},
			{"link", tmux + "/linkme", tmux + "/hardlink", no, ok},
			{"symlink", tmux + "/symlink", sockets + "/cd34/s", no, ok},
			{"bind", tmux + "/fake-server", "", no, ok},
			// Ruling (7)'s flip: a plain hire's server taken away and a link to another desk's socket put at its name.
			{"unlink", tmux + "/smolcorp-plain", "", no, ok},
			{"symlink", tmux + "/smolcorp-plain", sockets + "/cd34/s", no, ok},
			// The owner's `default` taken over by a server of the desk's: with the folder's place it is still there, so
			// the name is taken (EADDRINUSE, python's plain OSError), as a fresh name's bind above is refused.
			{"unlink", tmux + "/default", "", no, ok},
			{"bind", tmux + "/default", "", "OSError", ok},
			// The desk-start socket in the folder still connects; nothing else there does, as before.
			{"connect", tmux + "/smolcorp.desk-start.1", "", ok, ok},
			{"connect", tmux + "/smolcorp-plain", "", no, no},
			// Beside it, normal work.
			{"write", base + "/elsewhere/notes", "", ok, ok},
			{"rename", tmux, base + "/elsewhere/tmux-moved", no, ok},
		}
	}
	for _, withPlace := range []bool{true, false} {
		list, said, base := tmuxFolderRun(t, python, withPlace, steps)
		label := "with tmux's folder's place"
		if !withPlace {
			label = "without it (before FXTMUXDIR)"
		}
		for i, s := range list {
			want := s.with
			if !withPlace {
				want = s.without
			}
			at, to := strings.TrimPrefix(s.at, base+"/"), strings.TrimPrefix(s.to, base+"/")
			t.Logf("%s: %-7s %s %s: %s", label, s.what, at, to, said[i])
			if said[i] != want {
				t.Errorf("%s: %s %s %s: %s, want %s", label, s.what, at, to, said[i], want)
			}
		}
		if withPlace {
			for _, name := range []string{"smolcorp-plain", "default"} {
				if info, err := os.Lstat(filepath.Join(base, "tmux-"+strconv.Itoa(os.Getuid()), name)); err != nil || info.Mode().Type() != os.ModeSocket {
					t.Errorf("the stand-in %s isn't its server's socket any more: %v %v", name, info, err)
				}
			}
		}
	}
}
