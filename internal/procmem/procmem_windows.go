package procmem

import "golang.org/x/sys/windows"

// accessList is the process's: the owner keeps SYNCHRONIZE (wait for it to end), PROCESS_TERMINATE (stop it, as
// Task Manager does) and PROCESS_QUERY_LIMITED_INFORMATION (whether it runs, its name: desktop/process_windows.go),
// granted to OWNER RIGHTS, which also takes away the owner's own right to rewrite the list; the system keeps all.
// Protected, so nothing is inherited into it.
const accessList = "D:P(A;;0x00101001;;;OW)(A;;GA;;;SY)"

// DenyMemoryAccess sets this process's access list (see the package's comment).
func DenyMemoryAccess() error {
	descriptor, err := windows.SecurityDescriptorFromString(accessList)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(windows.CurrentProcess(), windows.SE_KERNEL_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// NotDumpable is Linux's only: here, DenyMemoryAccess does it.
func NotDumpable() error { return nil }
