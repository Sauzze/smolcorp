//go:build windows

package desktop

import (
	"errors"

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

// notifyUser: Smolcorp.exe has no console, so a message that matters is shown.
func notifyUser(text string) {
	title, _ := windows.UTF16PtrFromString("Smolcorp")
	body, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND)
}
