//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <stdlib.h>
#include <Security/Security.h>
#include <sys/socket.h>
#include <sys/un.h>

// Tailscale's team, and the one program of theirs that counts: the standalone build's network extension, which runs
// as root (peerowner_darwin.go). Pinned to its identifier, so Tailscale's command line, its app or anything else the
// team signs isn't taken for it. The Mac App Store build's extension (another identifier, run as the owner) is
// refused.
#define SMOLCORP_TAILSCALE_TEAM "W5364U7YZB"
#define SMOLCORP_TAILSCALE_EXTENSION "io.tailscale.ipn.macsys.network-extension"
#define SMOLCORP_TAILSCALE_REQUIREMENT "anchor apple generic and identifier \"" SMOLCORP_TAILSCALE_EXTENSION \
	"\" and certificate leaf[subject.OU] = \"" SMOLCORP_TAILSCALE_TEAM "\""

static const char *smolcorpTailscaleRequirementText(void) { return SMOLCORP_TAILSCALE_REQUIREMENT; }
static const char *smolcorpTailscaleTeam(void) { return SMOLCORP_TAILSCALE_TEAM; }

static SecRequirementRef smolcorpRequirement(const char *text) {
	CFStringRef string = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
	if (string == NULL) {
		return NULL;
	}
	SecRequirementRef requirement = NULL;
	OSStatus status = SecRequirementCreateWithString(string, kSecCSDefaultFlags, &requirement);
	CFRelease(string);
	return status == errSecSuccess ? requirement : NULL;
}

static int smolcorpRequirementParses(const char *text) {
	SecRequirementRef requirement = smolcorpRequirement(text);
	if (requirement == NULL) {
		return 0;
	}
	CFRelease(requirement);
	return 1;
}

// The other end's audit token, as the kernel has it now (LOCAL_PEERTOKEN): 0 when it can't be read, as when the
// process that connected has ended.
static int smolcorpPeerToken(int fd, audit_token_t *token) {
	socklen_t size = sizeof(*token);
	return getsockopt(fd, SOL_LOCAL, LOCAL_PEERTOKEN, token, &size) == 0 && size == sizeof(*token);
}

// The process the audit token names, checked by that token (kSecGuestAttributeAudit): valid, meeting the
// requirement, and, when team isn't empty, of that team. A token names one run of one program: once its process
// has run another (exec), or ended, the token names nothing and the check fails.
static int smolcorpTokenMeets(const audit_token_t *token, const char *text, const char *team) {
	int meets = 0;
	CFDataRef data = CFDataCreate(NULL, (const UInt8 *)token, sizeof(*token));
	if (data == NULL) {
		return 0;
	}
	const void *keys[] = {kSecGuestAttributeAudit};
	const void *values[] = {data};
	CFDictionaryRef attributes = CFDictionaryCreate(NULL, keys, values, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	SecCodeRef code = NULL;
	SecRequirementRef requirement = smolcorpRequirement(text);
	CFDictionaryRef information = NULL;
	if (attributes != NULL && requirement != NULL
		&& SecCodeCopyGuestWithAttributes(NULL, attributes, kSecCSDefaultFlags, &code) == errSecSuccess
		&& SecCodeCheckValidity(code, kSecCSDefaultFlags, requirement) == errSecSuccess) {
		if (team[0] == 0) {
			meets = 1;
		} else if (SecCodeCopySigningInformation((SecStaticCodeRef)code, kSecCSSigningInformation, &information) == errSecSuccess) {
			CFStringRef wanted = CFStringCreateWithCString(NULL, team, kCFStringEncodingUTF8);
			CFTypeRef found = CFDictionaryGetValue(information, kSecCodeInfoTeamIdentifier);
			meets = wanted != NULL && found != NULL && CFGetTypeID(found) == CFStringGetTypeID()
				&& CFStringCompare((CFStringRef)found, wanted, 0) == kCFCompareEqualTo;
			if (wanted != NULL) CFRelease(wanted);
		}
	}
	if (information != NULL) CFRelease(information);
	if (code != NULL) CFRelease(code);
	if (requirement != NULL) CFRelease(requirement);
	if (attributes != NULL) CFRelease(attributes);
	CFRelease(data);
	return meets;
}
*/
import "C"

import "unsafe"

// peerTokenHere reads the other end's audit token from a unix socket: as the kernel has it now, not as it was at
// connect (a process that execs gets a new one), so the judge never takes it for who connected.
func peerTokenHere(fd int) (auditToken, bool) {
	var token C.audit_token_t
	if C.smolcorpPeerToken(C.int(fd), &token) != 1 {
		return auditToken{}, false
	}
	return *(*auditToken)(unsafe.Pointer(&token)), true
}

// tokenMeets says whether the process an audit token names meets a code requirement, checked by the token, and is
// of team ("" for any). Never a prompt; a process that can't be checked is a no.
func tokenMeets(token auditToken, requirement, team string) bool {
	text, wanted := C.CString(requirement), C.CString(team)
	defer C.free(unsafe.Pointer(text))
	defer C.free(unsafe.Pointer(wanted))
	return C.smolcorpTokenMeets((*C.audit_token_t)(unsafe.Pointer(&token)), text, wanted) == 1
}

// tailscaleSignedHere says whether the process an audit token names is Tailscale's network extension (its
// identifier, team W5364U7YZB).
func tailscaleSignedHere(token auditToken) bool {
	return tokenMeets(token, C.GoString(C.smolcorpTailscaleRequirementText()), C.GoString(C.smolcorpTailscaleTeam()))
}

// tailscaleRequirementParses says whether macOS takes the requirement above: a test's check that a no from
// tailscaleSignedHere isn't only a requirement it couldn't read.
func tailscaleRequirementParses() bool {
	text := C.CString(C.GoString(C.smolcorpTailscaleRequirementText()))
	defer C.free(unsafe.Pointer(text))
	return C.smolcorpRequirementParses(text) == 1
}
