package desktop

import (
	"net"
	"net/http"
	"strings"
)

// Which program asked for a key (FXWINTOOLASK; RT126SWEEP2 P4 F4). Any program on this computer can send a terminal's
// ask, with any folder it likes, and print the code in a window you watch. On a Mac a hire runs in its sandbox; on
// Windows there's none, so a hire can ask as plainly as you can. So on Windows, Smolcorp's window says which program
// asked, as Windows reports it, not in the asker's own words:
//   - the program: the one holding the asking connection in Windows' TCP table (the connection is open while the ask
//     is taken), by its full path, read with a handle on that process, so its number can't pass to another one
//     meanwhile (askprogram_windows.go);
//   - what started it: the programs above it in Windows' list of processes, by name, nearest first, each started
//     before the one below it (a number Windows gave again, to a later program, ends the line).
//
// It's a hint for the owner, not proof, and nothing is refused on it: a program chooses its own name, it can choose
// the parent Windows reports for it (any process of the same user), and it can simply run the real smolcorp.exe
// status. So a hire's `smolcorp status` usually shows the agent that runs it (claude.exe, say) in that line, and yours
// your terminal, but a hire that means to can look like yours. The window says so ("One of your hires can ask too…").
// What can't be read is said too: "Windows couldn't say which program asked". On Linux nothing is read (it takes no
// ask), and the window shows the folder alone.
//
// On a Mac a hire in its sandbox can still reach Smolcorp's port, so it could ask too (RT126SWEEP3 P4 F1). There the
// asking program is found by its connection, as Windows' is: every process of yours holding the socket from the
// ask's address to Smolcorp's (askprogram_darwin.go, with libproc, as the fire's reap lists processes). macOS then says
// of each, by the audit token of what it runs now, whether it's in a sandbox (sandbox_check, with no operation: a hire
// is in its desk's; your terminal is in none). One in a sandbox: the ask is refused, in words that say why. None
// found holding it: refused too, and nothing is listed (RT126SWEEP3FIXP4 N2), even if some process's files couldn't
// be read (RT126SWEEP3FIXP4R1 B1). Its asker let go at once, or handed the socket on and it wasn't taken yet; no
// terminal of yours does either, as yours holds its socket till the answer and its files are read. A process that has
// ended, whether it waits for its parent (a zombie, N1) or is gone, holds no files, so it's passed over; so is one
// inside exit(), which never runs its own code again even while closing its files holds it there (RT126SWEEP3FIXP4R2
// B2), and one with no files open (B1).
// Where macOS can't say of one found (a process it won't answer for, or beside it a running process, or one being
// made, whose files it won't read), or couldn't look (a build without cgo), the ask is taken, and the window says it
// may be a hire's, with the folder shown as before.

// askAsker is what the system says of an ask's program.
type askAsker struct {
	tried   bool     // this system was asked (Windows)
	program string   // its full path; "" if it couldn't be read
	from    []string // what started it, by name, nearest first
	sandbox askSandbox
}

// askSandbox is what macOS says of the program that asked: sandboxNotAsked off a Mac.
type askSandbox int

const (
	sandboxNotAsked askSandbox = iota
	sandboxOutside             // every process holding the asking connection is in no sandbox, as your terminal is
	sandboxInside              // one of them is in a sandbox: a hire's, or another sandboxed program's
	sandboxUnknown             // macOS couldn't say: one found wasn't answered for, or beside one a process wasn't read, or none was looked at
	sandboxNoAsker             // no process found holds the connection: the asker let it go, so it can't read a ticket either
)

// askFromASandbox is what a program in a sandbox is told when it asks, on a Mac.
const askFromASandbox = "This program runs in a sandbox, as Smolcorp's hires do, so Smolcorp doesn't take its ask. " +
	"Run it in your own terminal."

// askNoAsker is what an ask is told on a Mac when no program holds its connection (sandboxNoAsker).
const askNoAsker = "Smolcorp couldn't see which program asked, so it didn't take the ask. " +
	"Run the command again in your own terminal."

// askHolderLook is how askSandboxOf finds the processes holding a connection and asks about each: list is every
// process of yours; holds says whether pid holds a socket from one address to another (1 it does, 0 not, -1 can't
// be read; a process that has ended, or is inside exit(), holds none, so 0); in whether pid is in a sandbox (1 in one,
// 0 in none, -1 not answered). Stand-ins in a test.
type askHolderLook struct {
	list  func() ([]int, error)
	holds func(pid int, from, to *net.TCPAddr) int
	in    func(pid int) int
}

// askSandboxOf is what macOS says of the processes holding the connection from remote (the asker's end) to local
// (Smolcorp's), both on this Mac's loopback. A socket passed on, or kept across a fork, has more than one. Each is
// asked whether it's in a sandbox, then must still hold the socket, so the answer is that holder's and not a later
// process's given the same number; one that no longer does wasn't a holder. Any one in a sandbox: sandboxInside. None
// found holding it: sandboxNoAsker, refused, even if some process couldn't be read (B1: a desk can always keep one
// such, so "unread" alone would take its closed asks and flag every one of yours). A holder found, but one not
// answered, or beside it a process whose files couldn't be read (that one may hold the socket too, and be in a
// sandbox): sandboxUnknown. Only when every holder is in none, and every process was read: sandboxOutside.
func askSandboxOf(remote, local string, look askHolderLook) askSandbox {
	from, fromErr := net.ResolveTCPAddr("tcp", remote)
	to, toErr := net.ResolveTCPAddr("tcp", local)
	if fromErr != nil || toErr != nil || !from.IP.IsLoopback() || !to.IP.IsLoopback() {
		return sandboxUnknown
	}
	pids, err := look.list()
	if err != nil {
		return sandboxUnknown
	}
	verdict, held, unread := sandboxOutside, false, false
	for _, pid := range pids {
		switch look.holds(pid, from, to) {
		case 1:
		case 0:
			continue
		default:
			unread = true
			continue
		}
		in := look.in(pid)
		switch look.holds(pid, from, to) {
		case 1:
		case 0:
			continue // let go meanwhile: not a holder (its answer may be a later process's)
		default:
			unread = true
			continue
		}
		held = true
		switch in {
		case 1:
			return sandboxInside
		case 0:
		default:
			verdict = sandboxUnknown
		}
	}
	switch {
	case !held:
		return sandboxNoAsker
	case unread:
		return sandboxUnknown
	}
	return verdict
}

// askFromMost is how many programs above the asker are named.
const askFromMost = 4

// askingProgram is which program sent r, as this system says: a variable so a test can stand one in.
var askingProgram = askingProgramHere

// shown adds what is known of the asker to an ask as the page shows it: nothing where nothing was read (a Mac, Linux),
// else the program ("" when Windows couldn't say) and what started it.
func (a askAsker) shown(ask map[string]any) {
	// A Mac that couldn't tell the asker: the window says it may be a hire's (askprogram_darwin.go).
	if a.sandbox == sandboxUnknown {
		ask["mayBeAHire"] = true
	}
	if !a.tried {
		return
	}
	ask["program"] = askFolder(a.program) // printable, one line, cut short, as the folder is
	from := []string{}
	for _, name := range a.from {
		from = append(from, askFolder(name))
	}
	ask["startedFrom"] = from
}

// listedProc is one process in a list of them, as Windows' list says: its program's name, when it started, and the
// process that started it.
type listedProc struct {
	name    string
	created int64 // Unix nanoseconds
	parent  uint32
}

// startedFrom is what started pid, by name, nearest first, at most askFromMost: each one in the list and started before
// the one it started. A parent not in the list, or one started after (its number given again), ends the line, as
// does a loop.
func startedFrom(pid uint32, list map[uint32]listedProc) []string {
	var names []string
	seen := map[uint32]bool{pid: true}
	child, ok := list[pid]
	for ok && len(names) < askFromMost {
		parent, listed := list[child.parent]
		if !listed || seen[child.parent] || parent.created <= 0 || parent.created >= child.created || strings.TrimSpace(parent.name) == "" {
			break
		}
		seen[child.parent] = true
		names = append(names, parent.name)
		child = parent
	}
	return names
}

// askerRead is the asker of r on a system that reads one: the program, and what started it, from read; "" for a
// request with no connection behind it or none Windows can name.
func askerRead(r *http.Request, read func(from, to string) (string, []string)) askAsker {
	local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if local == nil || r.RemoteAddr == "" {
		return askAsker{tried: true}
	}
	program, from := read(r.RemoteAddr, local.String())
	if program == "" {
		return askAsker{tried: true}
	}
	return askAsker{tried: true, program: program, from: from}
}
