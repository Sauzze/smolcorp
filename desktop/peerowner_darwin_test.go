package desktop

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The judge, with stand-in peers: only one that connected as root, runs as root (its real and its effective user)
// and is Tailscale's extension by its signature is Tailscale's. Who connected is what exec can't fake (RT126SN1 B1):
// a program of the owner's that execs a signed image keeps the user it connected as. A peer of the owner's is never
// even checked for a signature.
func TestOnlyRootsSignedExtensionIsTailscales(t *testing.T) {
	savedSigned, savedUID := tailscaleSigned, tailscaleUID
	t.Cleanup(func() { tailscaleSigned, tailscaleUID = savedSigned, savedUID })
	tailscaleUID = 0
	const owner = 501
	const extension, other = 100, 200
	checked := 0
	tailscaleSigned = func(token auditToken) bool {
		checked++
		return token.pid() == extension
	}
	token := func(ruid, euid uint32, pid int32) auditToken {
		var t auditToken
		t[1], t[3], t[5] = euid, ruid, uint32(pid)
		return t
	}
	for _, c := range []struct {
		name    string
		peer    servePeer
		want    bool
		checked bool
	}{
		{"Tailscale's extension, as root", servePeer{0, token(0, 0, extension)}, true, true},
		{"root, but not Tailscale's", servePeer{0, token(0, 0, other)}, false, true},
		{"connected as the owner, now the extension as root (exec into a setuid image)", servePeer{owner, token(0, 0, extension)}, false, false},
		{"connected as the owner, now the extension (exec)", servePeer{owner, token(owner, owner, extension)}, false, false},
		{"the extension, run as the owner (the Mac App Store build)", servePeer{owner, token(owner, owner, extension)}, false, false},
		{"root at connect, its real user the owner (setuid root)", servePeer{0, token(owner, 0, extension)}, false, false},
		{"root at connect, its effective user the owner (dropped)", servePeer{0, token(0, owner, extension)}, false, false},
		{"no process", servePeer{0, token(0, 0, 0)}, false, false},
	} {
		checked = 0
		if got := judgeServePeer(c.peer); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		if (checked > 0) != c.checked {
			t.Errorf("%s: the signature was checked %d times", c.name, checked)
		}
	}
}

// serve.sock is made only under the support folder's instance lock, held by the Mac app that started this engine:
// without it, there is none. Off the Mac there is none at all.
func TestServeSockIsMadeOnlyUnderTheInstanceLock(t *testing.T) {
	savedInstance, savedHeld, savedMac := instanceFile, held, macTailscale
	t.Cleanup(func() { instanceFile, held, macTailscale = savedInstance, savedHeld, savedMac })
	dir := shortTempDir(t)
	lock := filepath.Join(dir, "instance.lock")
	instanceFile, held, macTailscale = func() string { return lock }, nil, true
	if listener, err := listenServeSocket(); err == nil || listener != nil {
		t.Fatalf("no instance lock: %v %v", listener, err)
	}
	if _, err := os.Lstat(serveSocketPath()); err == nil {
		t.Fatal("serve.sock was made without the instance lock")
	}
	// The app's lock: another open of the file, held.
	app, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := syscall.Flock(int(app.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	listener, err := listenServeSocket()
	if err != nil || listener == nil {
		t.Fatalf("under the app's lock: %v", err)
	}
	listener.Close()
	macTailscale = false
	if listener, err := listenServeSocket(); err != nil || listener != nil {
		t.Errorf("off the Mac: %v %v", listener, err)
	}
}
