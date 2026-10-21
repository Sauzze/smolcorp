package desktop

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	getLastInputInfo = user32.NewProc("GetLastInputInfo")
	getTickCount     = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetTickCount")
)

// presence is how long the keyboard and mouse have gone unused. Whether the
// screen is locked isn't asked: a locked screen gets no input either.
func presence() (idle time.Duration, locked, ok bool) {
	info := struct {
		size uint32
		time uint32
	}{size: 8}
	if result, _, _ := getLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); result == 0 {
		return 0, false, false
	}
	now, _, _ := getTickCount.Call()
	return time.Duration(uint32(now)-info.time) * time.Millisecond, false, true
}
