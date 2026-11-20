package sandbox

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// FXCHRONYSOCK's Linux rows (CI linuxcaps, and the VM): chrony's case on this computer's kernel. No bwrap runs: only
// the start's words are made, as TestAMovedSocketFailsTheStartOnTheKernel does.

// standInSudo is sudo for the row's stand-in folders, which only root can give to another account: CI's runners and
// the VM's account have it without a password. Where it asks for one, or the row runs as root (then the owner's
// account would be root, and no folder another's), the row skips.
func standInSudo(t *testing.T) func(words ...string) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("the row runs as root: a stand-in folder can't be another account's than the owner's")
	}
	if out, err := exec.Command("sudo", "-n", "true").CombinedOutput(); err != nil {
		t.Skipf("no sudo without a password here (CI's runners and the VM's account have it): %v %s", err, out)
	}
	return func(words ...string) string {
		t.Helper()
		out, err := exec.Command("sudo", append([]string{"-n"}, words...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("sudo %q: %v %s", words, err, out)
		}
		return strings.TrimSpace(string(out))
	}
}

// kernelStart makes a desk's start's words on this computer, for a desk with its home and support folders in a temp
// folder: its error and what the start logged.
func kernelStart(t *testing.T) func() ([]string, error, []string) {
	t.Helper()
	base := t.TempDir()
	if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
		var err error
		if base, err = os.MkdirTemp(runner, "smolcorp-sysfolder-"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	own := filepath.Join(base, "sockets", "smolcorp-h1.d")
	for _, folder := range []string{support + "/hires/h1", root, own, base + "/tmp"} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: support + "/desk-start",
		Root: root, SocketParent: filepath.Dir(own), OwnSockets: own}
	return func() ([]string, error, []string) {
		var logged []string
		args, err := prepareBwrap(pl, View{Log: func(format string, a ...any) {
			logged = append(logged, fmt.Sprintf(format, a...))
			t.Logf(format, a...)
		}}, osView{}, base+"/tmp")
		return args, err, logged
	}
}

// A stand-in for chrony's /run/chrony, made with sudo: a folder in a folder of root's in /run, a datagram socket bound
// in it (as chronyd's is), then the folder given to the system account daemon (uid 1), 0750, so the owner's account
// can't look into it, as on CI's runners. The start goes on, and the log names the socket and the folder's owner. Its
// socket renamed there by that account (sudo), the start still goes on: no desk can have moved it. The same moved
// socket in the folder given back to the owner's account (0700, then 0755), or the system account's folder made
// writable by others (0777) or its group (0770), fails the start (ErrSocketMoved), and the error says whose the
// folder is. The real chrony, where there is one (CI's runners), is TestChronysOwnSocketDoesntStopADesk's.
func TestASocketInAFolderASystemAccountHasStartsADeskOnTheKernel(t *testing.T) {
	sudo := standInSudo(t)
	start := kernelStart(t)
	top := sudo("mktemp", "-d", "/run/smolcorp-standin-XXXXXX")
	t.Cleanup(func() { exec.Command("sudo", "-n", "rm", "-rf", "--", top).Run() })
	sudo("chmod", "0755", top)
	me, group := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	folder := top + "/chrony"
	sudo("install", "-d", "-o", me, "-g", group, "-m", "0700", folder)
	sock := folder + "/standin.sock"
	l, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: sock, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	give := func(owner, mode string) {
		t.Helper()
		sudo("chown", owner+":"+owner, folder)
		sudo("chmod", mode, folder)
	}
	give("1", "0750")
	if _, err := os.Lstat(sock); !errors.Is(err, syscall.EACCES) {
		t.Fatalf("the stand-in's socket, in daemon's 0750 folder: %v, want the owner's account refused (EACCES), as on CI", err)
	}
	check := func(name string, want error, says string) {
		t.Helper()
		args, err, logged := start()
		switch {
		case want != nil && !errors.Is(err, want):
			t.Errorf("%s: %v, want %v", name, err, want)
		case want != nil && !strings.Contains(err.Error(), says):
			t.Errorf("%s: %q doesn't say %q", name, err, says)
		case want == nil && err != nil:
			t.Errorf("%s: %v, want the start to go on", name, err)
		case want == nil && !slices.ContainsFunc(logged, func(l string) bool { return strings.Contains(l, sock) && strings.Contains(l, says) }):
			t.Errorf("%s: logged %q, want %s and %q", name, logged, sock, says)
		case want == nil && strings.Contains(strings.Join(args, "\n"), folder+"/"):
			t.Errorf("%s: something is masked or brought back in %s:\n%s", name, folder, strings.Join(args, " "))
		}
	}
	check("daemon's 0750 folder, where chronyd's would be", nil, folder+" is uid 1's")
	sudo("mv", sock, sock+".moved")
	check("daemon's 0750 folder, the socket renamed by daemon", nil, folder+" is uid 1's")
	give("1", "0777")
	check("daemon's folder, writable by others, the socket renamed", ErrSocketMoved, folder+" is uid 1's")
	give("1", "0770")
	check("daemon's folder, writable by its group, the socket renamed", ErrSocketMoved, folder+" is uid 1's")
	give(me, "0700")
	check("the owner's 0700 folder, the socket renamed", ErrSocketMoved, folder+" is uid "+me+"'s")
	give(me, "0755")
	check("the owner's 0755 folder, the socket renamed", ErrSocketMoved, folder+" is uid "+me+"'s")
}

// The real chrony, where this computer has it (CI's ubuntu runners; the VM after `apt install chrony`): its socket
// listens in /run/chrony, _chrony's. A desk's start goes on; where the owner's account can't look in, the log names
// the socket and the folder's owner, and nothing there is masked or brought back.
func TestChronysOwnSocketDoesntStopADesk(t *testing.T) {
	const folder, sock = "/run/chrony", "/run/chrony/chronyd.sock"
	list, err := os.ReadFile(netUnix)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(list), " "+sock+"\n") {
		t.Skipf("no chronyd listens at %s here (`apt install chrony` makes one)", sock)
	}
	st, err := os.Lstat(folder)
	if err != nil {
		t.Fatal(err)
	}
	if uid := fileOwner(st); uid == int64(os.Getuid()) || uid == 0 {
		t.Skipf("%s is uid %d's here, not a system account's other than root", folder, uid)
	}
	args, err, logged := kernelStart(t)()
	if err != nil {
		t.Fatalf("a desk's start with chrony here: %v", err)
	}
	if _, err := os.Lstat(sock); errors.Is(err, syscall.EACCES) {
		if !slices.ContainsFunc(logged, func(l string) bool { return strings.Contains(l, sock) && strings.Contains(l, folder+" is uid ") }) {
			t.Errorf("logged %q, want %s and its folder's owner", logged, sock)
		}
		if strings.Contains(strings.Join(args, "\n"), folder+"/") {
			t.Errorf("something is masked or brought back in %s, which the account can't open:\n%s", folder, strings.Join(args, " "))
		}
	}
}
