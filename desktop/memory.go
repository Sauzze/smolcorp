package desktop

import (
	"fmt"
	"os"

	"github.com/Sauzze/smolcorp/internal/procmem"
)

// protectMemory keeps this process's memory from your other programs, a hire among them, as far as the system lets
// a process do that for itself (internal/procmem): non-dumpable on Linux, the process's access list on Windows. It
// is the first step of RunApp and RunServer, the two ways this program becomes the core, before any key exists
// here, Setup's key and the office window's among them. A variable, for the tests.
var protectMemory = func() error {
	if err := procmem.NotDumpable(); err != nil {
		return err
	}
	return procmem.DenyMemoryAccess()
}

// unprotectedMessage is what the owner is told when protectMemory fails: nothing has started, and why.
const unprotectedMessage = "Smolcorp didn't start: it couldn't keep its memory from your other programs (%v), and its keys would be in that memory. " +
	"No key was made or read. Open Smolcorp again. If this message comes back, restart your computer, then open it."

// warnOwner shows a message that matters with no terminal to read it in (instance_windows.go): a variable, for the
// tests, which never put a dialog on the screen.
var warnOwner = notifyUser

// refuseUnprotected fails closed: said in the terminal, and in a dialog on Windows, whose app has none.
func refuseUnprotected(err error) int {
	message := fmt.Sprintf(unprotectedMessage, err)
	fmt.Fprintln(os.Stderr, "smolcorp: "+message)
	warnOwner(message)
	return 1
}
