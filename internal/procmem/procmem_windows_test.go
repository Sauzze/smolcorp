package procmem

import (
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SK3's Windows spike row (SANDBOX-PLAN §3.3, RSBXPLANe N4): another process of the same user can wait for a
// protected stand-in, stop it and ask whether it runs, and can't read or write its memory, start a thread in it,
// copy a handle out of it or rewrite its access list, elevated or not. Then the row switches SeDebugPrivilege on
// where this account has it (an administrator does), and says whether that opens it: §3.3 expects it to, and the
// release notes say so.
func TestTheProcessAccessListKeepsMemoryFromTheUsersOtherProcesses(t *testing.T) {
	elevated := windows.GetCurrentProcessToken().IsElevated()
	t.Logf("this test runs elevated: %v", elevated)
	open := startStandIn(t, false)
	if handle, err := windows.OpenProcess(windows.PROCESS_VM_READ, false, uint32(open.Process.Pid)); err != nil {
		t.Fatalf("an unprotected stand-in should open for reading (the row means nothing otherwise): %v", err)
	} else {
		windows.CloseHandle(handle)
	}

	pid := uint32(startStandIn(t, true).Process.Pid)
	kept := uint32(windows.SYNCHRONIZE | windows.PROCESS_TERMINATE | windows.PROCESS_QUERY_LIMITED_INFORMATION)
	if handle, err := windows.OpenProcess(kept, false, pid); err != nil {
		t.Errorf("waiting for it, stopping it and asking whether it runs should still work: %v", err)
	} else {
		windows.CloseHandle(handle)
	}
	for name, access := range map[string]uint32{
		"read its memory": windows.PROCESS_VM_READ, "write its memory": windows.PROCESS_VM_WRITE | windows.PROCESS_VM_OPERATION,
		"start a thread in it": windows.PROCESS_CREATE_THREAD, "copy a handle out of it": windows.PROCESS_DUP_HANDLE,
		"rewrite its access list": windows.WRITE_DAC, "take it over": windows.WRITE_OWNER,
	} {
		handle, err := windows.OpenProcess(access, false, pid)
		if err == nil {
			windows.CloseHandle(handle)
			t.Errorf("another process could %s", name)
		} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Errorf("%s: refused, but not as denied: %v", name, err)
		}
	}

	// The spike: SeDebugPrivilege, where this account holds it.
	if !enableDebugPrivilege(t) {
		t.Log("SeDebugPrivilege couldn't be switched on here: that half of the row didn't run")
		return
	}
	if handle, err := windows.OpenProcess(windows.PROCESS_VM_READ, false, pid); err == nil {
		windows.CloseHandle(handle)
		t.Log("with SeDebugPrivilege on, the protected stand-in opens for reading: as §3.3 says, an administrator gets in")
	} else {
		t.Logf("with SeDebugPrivilege on, it still refused reading: %v", err)
	}
}

// enableDebugPrivilege switches SeDebugPrivilege on in this process's token: whether it then holds it.
func enableDebugPrivilege(t *testing.T) bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		t.Logf("its token: %v", err)
		return false
	}
	defer token.Close()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeDebugPrivilege"), &luid); err != nil {
		t.Logf("SeDebugPrivilege: %v", err)
		return false
	}
	privileges := windows.Tokenprivileges{PrivilegeCount: 1}
	privileges.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	if err := windows.AdjustTokenPrivileges(token, false, &privileges, 0, nil, nil); err != nil {
		t.Logf("switching it on: %v", err)
		return false
	}
	// AdjustTokenPrivileges succeeds without granting what the token doesn't hold: the token says what it holds now.
	buffer := make([]byte, 4096)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], uint32(len(buffer)), &size); err != nil {
		t.Logf("its privileges: %v", err)
		return false
	}
	held := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	for _, privilege := range unsafe.Slice(&held.Privileges[0], held.PrivilegeCount) {
		if privilege.Luid == luid {
			return privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0
		}
	}
	return false
}
