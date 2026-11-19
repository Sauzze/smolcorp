//go:build darwin && cgo

package sandbox

/*
#include <stdlib.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <mach/message.h>

// libsystem_sandbox's checks, as WebKit and Chromium declare them (no public header has them). The filter type takes
// SANDBOX_CHECK_NO_REPORT, so a denied answer isn't logged as a violation.
enum sandbox_filter_type { SANDBOX_FILTER_NONE, SANDBOX_FILTER_PATH };
extern const enum sandbox_filter_type SANDBOX_CHECK_NO_REPORT;
int sandbox_check_by_audit_token(audit_token_t, const char *operation, enum sandbox_filter_type, ...);
int sandbox_check(pid_t, const char *operation, enum sandbox_filter_type, ...);

// The pid the peer's audit token names now (LOCAL_PEERTOKEN), and whether that process may read path: 1 denied,
// 0 allowed, -1 macOS didn't say, -2 no token.
static int smolcorpDeniesRead(int fd, const char *path, int *tokenPID) {
	audit_token_t token;
	socklen_t size = sizeof(token);
	*tokenPID = 0;
	if (getsockopt(fd, SOL_LOCAL, LOCAL_PEERTOKEN, &token, &size) != 0 || size != sizeof(token)) {
		return -2;
	}
	*tokenPID = (int)token.val[5]; // the pid, as libbsm's audit_token_to_pid reads it
	int denied = sandbox_check_by_audit_token(token, "file-read-data", SANDBOX_FILTER_PATH | SANDBOX_CHECK_NO_REPORT, path);
	return denied == 0 || denied == 1 ? denied : -1;
}

// Whether the process pid may read path: 1 denied, 0 allowed, -1 macOS didn't say. Asked by pid: only for the
// caller's own child it hasn't waited for, whose pid can't be another process's.
static int smolcorpChildDeniesRead(int pid, const char *path) {
	int denied = sandbox_check((pid_t)pid, "file-read-data", SANDBOX_FILTER_PATH | SANDBOX_CHECK_NO_REPORT, path);
	return denied == 0 || denied == 1 ? denied : -1;
}
*/
import "C"

import "unsafe"

// deniesCanary is the pid the peer on fd's audit token names, and macOS's answer, asked by that token, to whether it
// may read canary: 1 denied, 0 allowed, below 0 not answered.
func deniesCanary(fd int, canary string) (tokenPID, denied int) {
	path := C.CString(canary)
	defer C.free(unsafe.Pointer(path))
	var pid C.int
	denied = int(C.smolcorpDeniesRead(C.int(fd), path, &pid))
	return int(pid), denied
}

// childDeniesCanary is macOS's answer to whether the process pid, the core's own child not yet waited for, may read
// canary: 1 denied, 0 allowed, below 0 not answered.
func childDeniesCanary(pid int, canary string) int {
	path := C.CString(canary)
	defer C.free(unsafe.Pointer(path))
	return int(C.smolcorpChildDeniesRead(C.int(pid), path))
}
