package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// What both systems' runners (run_darwin.go, SA-M3; run_linux.go, SA-L2) refuse with, and the part of Linux's runner
// that decides without asking the kernel: written here, so it is tested on every system.

// NotOurServerRefusal is what the owner sees (§A5.2's table) when something was at the desk's socket already, or the
// server there isn't the one started.
const NotOurServerRefusal = "Something else was already using this hire's place, so Smolcorp didn't start it. Quit other copies of Smolcorp and try again."

// LinuxSandboxRefusal is what the owner sees when a Linux desk's view can't be made, or its start fails before its
// check for any other reason (§A5.2's first row, as Linux says it).
const LinuxSandboxRefusal = "Smolcorp couldn't put this hire in its sandbox on this computer, so it didn't start it. Update Smolcorp."

// KeptChangedRefusal is what the owner sees when a Linux desk's start put back a place its last session was stopped
// for (ErrKeptChanged, L1GUARD1): the start fails that once, and the next one goes on. What was put back follows it.
const KeptChangedRefusal = "Smolcorp didn't start this hire this time: its last session changed a place it may not change and was stopped for it, so Smolcorp put that place back as it was. Start the hire again to go on."

// Errors the runners give.
var (
	ErrStartNotWhole = errors.New("sandbox: a desk's start needs whole paths (the profile or the view's words, Smolcorp and the program) and every row its system checks")
	ErrNotOurServer  = errors.New("sandbox: the server at the desk's socket isn't in this start's sandbox")
)

// LinuxStartRefusal is the owner's words, on the card and in Setup's Sandbox row, for what failed a Linux desk's start,
// from whichever step said it: a place put back (ErrKeptChanged: what, in KeptWords' words); Smolcorp's bubblewrap or
// its profiles not fit (RefusalFor); the server not the one started (ErrNotOurServer); a check row that failed, or no
// handshake (*Refused); anything else, LinuxSandboxRefusal.
func LinuxStartRefusal(err error) string {
	var refused *Refused
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrKeptChanged):
		// SaveKept's error is ErrKeptChanged, then what was put back, in the words the owner's card uses.
		if put, ok := strings.CutPrefix(err.Error(), ErrKeptChanged.Error()+": "); ok && put != "" {
			return KeptChangedRefusal + "\n" + put
		}
		return KeptChangedRefusal
	case errors.Is(err, ErrNoBwrap), errors.Is(err, ErrNoOwnBwrap), errors.Is(err, ErrNoProfile):
		return RefusalFor(err)
	case errors.Is(err, ErrNotOurServer):
		return NotOurServerRefusal
	case errors.As(err, &refused):
		return Refusal("linux")
	}
	return LinuxSandboxRefusal
}

// NS is one namespace, as stat names it: its file system's device and its inode.
type NS struct{ Dev, Ino uint64 }

// Namespaces are the pid and mount namespaces of one desk's view, as the core recorded them when bwrap made it
// (Start.Namespaces). The server check (ServerInView) holds the server's pid to them.
type Namespaces struct{ Pid, Mnt NS }

// infoMost is the most of bwrap's --info-fd the core reads: its child's pid and a few namespaces' numbers.
const infoMost = 64 << 10

// readInfo reads what bwrap writes to its --info-fd: one JSON object, of which the core takes its child's pid, the
// first process in the view. It reads that object alone, not to the end: bwrap's own process in the view may hold the
// descriptor as long as the view lasts.
func readInfo(r io.Reader) (int, error) {
	var info struct {
		ChildPid *int `json:"child-pid"`
	}
	if err := json.NewDecoder(io.LimitReader(r, infoMost)).Decode(&info); err != nil {
		return 0, fmt.Errorf("bwrap said nothing the core can read: %w", err)
	}
	if info.ChildPid == nil || *info.ChildPid <= 1 {
		return 0, errors.New("bwrap named no child the core can see")
	}
	return *info.ChildPid, nil
}

// peerInView is nil when the process listening at a desk's socket, by the user and pid the kernel gave (SO_PEERCRED)
// and the namespaces that pid is in (got), is in the view whose namespaces the core recorded (view). Else why not.
func peerInView(uid uint32, pid int, got, view Namespaces) error {
	switch {
	case view.Pid.Ino == 0 || view.Mnt.Ino == 0:
		return errors.New("no view recorded to check it against")
	case uid != uint32(os.Getuid()):
		return fmt.Errorf("it listens as user %d", uid)
	case pid <= 1:
		return fmt.Errorf("the kernel names no process for it (pid %d)", pid)
	case got.Pid != view.Pid:
		return fmt.Errorf("pid %d isn't in the view's pid namespace", pid)
	case got.Mnt != view.Mnt:
		return fmt.Errorf("pid %d isn't in the view's mount namespace", pid)
	}
	return nil
}

// landlockScopeABI is the first Landlock ABI with the abstract-socket scope (Linux 6.12).
const landlockScopeABI = 6

// landlockFrom says, from the kernel's answer to which Landlock ABI it has, whether a start has the Landlock row, and
// why not when it hasn't (for the start's log, never the owner's: a kernel without it still has the view, the filter
// and --unshare-net, which already keeps the computer's abstract sockets out of reach).
func landlockFrom(abi int, err error) (bool, string) {
	switch {
	case err != nil:
		return false, "Landlock isn't on in this kernel (" + err.Error() + "), so a desk's start has no Landlock scope"
	case abi < landlockScopeABI:
		return false, fmt.Sprintf("this kernel's Landlock is ABI %d; its abstract-socket scope needs ABI %d, so a desk's start has none", abi, landlockScopeABI)
	}
	return true, ""
}
