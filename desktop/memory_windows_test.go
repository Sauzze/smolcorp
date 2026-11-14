package desktop

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

func init() { standInWindow = openStandInWindow }

// T126WINMEM (RT126SD1R2 N-B1): the tray app and `smolcorp serve` keep their memory, Setup's key and the office
// window's in it, from the user's other programs, a hire among them, by the same first step. The stand-ins run this
// test program as far as the end of that step, with the real protectMemory; then another process of the same user
// (this one) asks to read their memory, and is refused. They can still be waited for, stopped and asked whether they
// run. The control, with no step, opens for reading. An administrator opens any process whatever its access list, so
// an elevated run, or one whose account holds SeDebugPrivilege at all, skips and says why; with
// SMOLCORP_MEMORY_REQUIRED=1 it fails instead. CI's windows-memory job sets that and runs this as a standard user,
// so only that user's run can say PASS.
func TestTheTrayAppAndServeRefuseTheUsersOtherProcessesTheirMemory(t *testing.T) {
	requireStandardUserForMemory(t)
	pid, answer := startMemoryStandIn(t, "open")
	if answer != "ready" {
		t.Fatalf("the control didn't start: %s", answer)
	}
	if handle, err := windows.OpenProcess(windows.PROCESS_VM_READ, false, uint32(pid)); err != nil {
		t.Fatalf("a stand-in with no access list should open for reading (the row means nothing otherwise): %v", err)
	} else {
		windows.CloseHandle(handle)
	}

	for _, part := range []string{"app", "serve"} {
		pid, answer := startMemoryStandIn(t, part)
		if answer != "ready" {
			t.Errorf("%s: its first step failed: %s", part, answer)
			continue
		}
		if handle, err := windows.OpenProcess(windows.PROCESS_VM_READ, false, uint32(pid)); err == nil {
			windows.CloseHandle(handle)
			t.Errorf("%s: another process of yours could read its memory", part)
		} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Errorf("%s: reading its memory was refused, but not as denied: %v", part, err)
		}
		kept := uint32(windows.SYNCHRONIZE | windows.PROCESS_TERMINATE | windows.PROCESS_QUERY_LIMITED_INFORMATION)
		if handle, err := windows.OpenProcess(kept, false, uint32(pid)); err != nil {
			t.Errorf("%s: waiting for it, stopping it and asking whether it runs should still work: %v", part, err)
		} else {
			windows.CloseHandle(handle)
		}
	}
}

// requireStandardUserForMemory skips an administrator's run, or fails it where the row is required, as the jail's
// rows do (internal/sandbox's requireStandardUser).
func requireStandardUserForMemory(t *testing.T) {
	t.Helper()
	token := windows.GetCurrentProcessToken()
	why := ""
	if token.IsElevated() {
		why = "this test runs elevated"
	} else if holdsPrivilege(t, token, "SeDebugPrivilege") {
		why = "this account holds SeDebugPrivilege"
	}
	if why == "" {
		return
	}
	why += ": an administrator opens any process, so this row proves nothing here. CI's windows-memory job runs it as a standard user"
	if os.Getenv("SMOLCORP_MEMORY_REQUIRED") == "1" {
		t.Fatal(why)
	}
	t.Skip(why)
}

// holdsPrivilege says whether token holds the privilege at all, on or off (a copy of internal/sandbox's).
func holdsPrivilege(t *testing.T, token windows.Token, name string) bool {
	t.Helper()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4096)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], uint32(len(buffer)), &size); err != nil {
		t.Fatalf("the token's privileges can't be read: %v", err)
	}
	for _, privilege := range (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0])).AllPrivileges() {
		if privilege.Luid == luid {
			return true
		}
	}
	return false
}

// The office window is WebView2's, in the app's own process, whose browser runs apart and talks back to it: under
// the access list, it must still open and run its page. The control is the same window with no list. Where that one
// doesn't run either (no WebView2, no desktop to open it on), the row has nothing to compare and skips, saying so.
func TestTheOfficeWindowStillRunsUnderTheAccessList(t *testing.T) {
	if !webview2Installed() {
		t.Skip("WebView2 isn't installed here")
	}
	if _, answer := startMemoryStandIn(t, "open-window"); answer != "ready" {
		t.Skipf("with no access list, the window didn't run its page here either: %s", answer)
	}
	if _, answer := startMemoryStandIn(t, "app-window"); answer != "ready" {
		t.Errorf("under the access list, the window didn't run its page: %s", answer)
	}
}

// openStandInWindow opens a small WebView2 window, as the office's opens (window_windows.go), with a page that calls
// back into this process, and closes it when it has: a minute at most.
func openStandInWindow() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dataDir, err := os.MkdirTemp("", "smolcorp-memory-webview2-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:      dataDir,
		WindowOptions: webview2.WindowOptions{Title: "Smolcorp memory row", Width: 320, Height: 200},
	})
	if w == nil {
		return errors.New("WebView2 didn't respond")
	}
	answered := false
	if err := w.Bind("smolcorpAnswer", func() { answered = true; w.Terminate() }); err != nil {
		return err
	}
	late := time.AfterFunc(time.Minute, func() { w.Dispatch(w.Terminate) })
	w.SetHtml("<script>smolcorpAnswer()</script>")
	w.Run()
	late.Stop()
	w.Destroy()
	if !answered {
		return errors.New("its page didn't call back within a minute")
	}
	return nil
}
