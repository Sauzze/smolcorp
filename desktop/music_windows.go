package desktop

import (
	"errors"
	"math"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The keyboard's media keys, which Windows sends to whatever is playing: a
// music app, a video in a browser.
var keybdEvent = windows.NewLazySystemDLL("user32.dll").NewProc("keybd_event")

const (
	vkMediaNextTrack  = 0xB0
	vkMediaPrevTrack  = 0xB1
	vkMediaPlayPause  = 0xB3
	keyeventfExtended = 0x1
	keyeventfKeyUp    = 0x2
)

// musicKey presses play/pause, previous or next, as the keyboard's media
// keys do. key is one of musicKeys.
func musicKey(key string) error {
	if err := keybdEvent.Find(); err != nil {
		return err
	}
	code := map[string]uintptr{"playpause": vkMediaPlayPause, "previous": vkMediaPrevTrack, "next": vkMediaNextTrack}[key]
	keybdEvent.Call(code, 0, keyeventfExtended, 0)
	keybdEvent.Call(code, 0, keyeventfExtended|keyeventfKeyUp, 0)
	return nil
}

// musicPlaying is whether music is playing, which Windows doesn't say here.
func musicPlaying() *bool { return nil }

// The output volume, through Core Audio's IAudioEndpointVolume on the
// default output, reached as plain COM calls: no wrapper library needed.
var (
	coCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioEndpointVolume = windows.GUID{Data1: 0x5CDF2C82, Data2: 0x841E, Data3: 0x4546, Data4: [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
)

// Where each method sits in its interface's table, as the Windows SDK's
// headers declare them.
const (
	comRelease                           = 2
	enumeratorGetDefaultEndpoint         = 4
	deviceActivate                       = 3
	endpointSetMasterScalar              = 7
	endpointGetMasterScalar              = 9
	endpointSetMute                      = 14
	endpointGetMute                      = 15
	eRender, eConsole                    = 0, 0
	clsctxAll                            = 0x17
	rpcEChangedMode              uintptr = 0x80010106
	sFalse                       uintptr = 1
)

// comCall calls method n of a COM object's table.
func comCall(object unsafe.Pointer, n int, args ...uintptr) uintptr {
	table := *(**[16]uintptr)(object)
	hr, _, _ := syscall.SyscallN(table[n], append([]uintptr{uintptr(object)}, args...)...)
	return hr
}

// withEndpointVolume hands the default output's IAudioEndpointVolume to
// use, on a thread with COM started, or returns false where there's none.
func withEndpointVolume(use func(endpoint unsafe.Pointer)) bool {
	// Go's calls pass a float in the register Windows expects it only on
	// x86: on ARM the setter would get nonsense, so there's no bar there.
	if runtime.GOARCH == "arm64" || coCreateInstance.Find() != nil {
		return false
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// S_FALSE: COM was already started on this thread, and still wants its uninitialize.
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err == nil {
		defer windows.CoUninitialize()
	} else if errno, ok := err.(syscall.Errno); ok && uintptr(errno) == sFalse {
		defer windows.CoUninitialize()
	} else if !ok || uintptr(errno) != rpcEChangedMode {
		return false
	}
	var enumerator, device, endpoint unsafe.Pointer
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator))); hr != 0 || enumerator == nil {
		return false
	}
	defer comCall(enumerator, comRelease)
	if hr := comCall(enumerator, enumeratorGetDefaultEndpoint, eRender, eConsole, uintptr(unsafe.Pointer(&device))); hr != 0 || device == nil {
		return false
	}
	defer comCall(device, comRelease)
	if hr := comCall(device, deviceActivate, uintptr(unsafe.Pointer(&iidIAudioEndpointVolume)), clsctxAll, 0,
		uintptr(unsafe.Pointer(&endpoint))); hr != 0 || endpoint == nil {
		return false
	}
	defer comCall(endpoint, comRelease)
	use(endpoint)
	return true
}

// getVolume is the default output's volume, or nil where it has none.
func getVolume() *musicVolume {
	var volume *musicVolume
	withEndpointVolume(func(endpoint unsafe.Pointer) {
		var scalar float32
		var muted int32
		if comCall(endpoint, endpointGetMasterScalar, uintptr(unsafe.Pointer(&scalar))) != 0 {
			return
		}
		comCall(endpoint, endpointGetMute, uintptr(unsafe.Pointer(&muted)))
		volume = &musicVolume{level: clampVolume(int(scalar*100 + 0.5)), muted: muted != 0}
	})
	return volume
}

// setVolume sets the default output's volume, 0 to 100. A level above 0
// unmutes, as the keyboard's volume keys do; 0 is simply 0.
func setVolume(level int) error {
	failed := errors.New("couldn't set the volume")
	ok := withEndpointVolume(func(endpoint unsafe.Pointer) {
		if comCall(endpoint, endpointSetMasterScalar, uintptr(math.Float32bits(float32(level)/100)), 0) != 0 {
			return
		}
		if level > 0 {
			comCall(endpoint, endpointSetMute, 0, 0)
		}
		failed = nil
	})
	if !ok {
		return errors.New("no volume control here")
	}
	return failed
}
