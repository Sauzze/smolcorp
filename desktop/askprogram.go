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
// What can't be read is said too: "Windows couldn't say which program asked". On a Mac and on Linux nothing is read,
// and the window shows the folder alone, as before.

// askAsker is what the system says of an ask's program.
type askAsker struct {
	tried   bool     // this system was asked (Windows)
	program string   // its full path; "" if it couldn't be read
	from    []string // what started it, by name, nearest first
}

// askFromMost is how many programs above the asker are named.
const askFromMost = 4

// askingProgram is which program sent r, as this system says: a variable so a test can stand one in.
var askingProgram = askingProgramHere

// shown adds what is known of the asker to an ask as the page shows it: nothing where nothing was read (a Mac, Linux),
// else the program ("" when Windows couldn't say) and what started it.
func (a askAsker) shown(ask map[string]any) {
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
