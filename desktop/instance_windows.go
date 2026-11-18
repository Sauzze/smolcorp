//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// instanceName is the mutex's name. "Local\" is this Windows session's own
// namespace: another user on the same PC (fast user switching) can have their
// own Smolcorp, which the port would then refuse anyway.
var instanceName = `Local\Smolcorp-single-instance`

// acquireInstance takes the lock for as long as the process lives (the system
// releases a mutex when its owner dies, however it dies). running is true when
// another Smolcorp holds it.
func acquireInstance() (running bool, err error) {
	name, err := windows.UTF16PtrFromString(instanceName)
	if err != nil {
		return false, err
	}
	// The handle is deliberately never closed: closing it is how the lock ends.
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		// Another's: this handle let go, or the mutex would outlive its owner
		// for as long as this one waits for it (see waitForInstance).
		windows.CloseHandle(handle)
		return true, nil
	}
	return false, err
}

// makeWayName is the event an update sets for the Smolcorp it was installed
// over: instanceName's, in the same session's own namespace.
func makeWayName() string { return instanceName + "-make-way" }

// listenForMakeWay has this copy, holding the instance lock, leave when an
// update installed over it asks (runningToMakeWay), as a restart leaves:
// the new copy waits for the lock it lets go of. Any program of yours could
// end this one anyway; this only asks it to.
func listenForMakeWay() {
	name, err := windows.UTF16PtrFromString(makeWayName())
	if err != nil {
		return
	}
	// Reset by the wait that sees it; kept open, as the mutex is, for as long as this copy lives.
	event, err := windows.CreateEvent(nil, 0, 0, name)
	if event == 0 {
		fmt.Fprintf(os.Stderr, "smolcorp: an update won't be able to ask this copy to make way (%v)\n", err)
		return
	}
	go func() {
		if got, _ := windows.WaitForSingleObject(event, windows.INFINITE); got == windows.WAIT_OBJECT_0 {
			fmt.Fprintln(os.Stderr, "smolcorp: an update was installed over this copy: it starts in its place")
			leave()
		}
	}()
}

// runningToMakeWay finds the Smolcorp running, if one listens, to ask later
// to make way for the copy just installed: ask is true when one was asked, and
// done lets go of it, asked or not. Found before the installed copy starts
// (startThenMakeWay): only the copy holding the instance lock makes the event,
// so one opened now is the old copy's, never the new one's.
func runningToMakeWay() (ask func() bool, done func()) {
	none := func() bool { return false }
	name, err := windows.UTF16PtrFromString(makeWayName())
	if err != nil {
		return none, func() {}
	}
	event, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return none, func() {}
	}
	return func() bool { return windows.SetEvent(event) == nil }, func() { windows.CloseHandle(event) }
}

// notifyUser: Smolcorp.exe has no console, so a message that matters is shown.
func notifyUser(text string) {
	title, _ := windows.UTF16PtrFromString("Smolcorp")
	body, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
}
