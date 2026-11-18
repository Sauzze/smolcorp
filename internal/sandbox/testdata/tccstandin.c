// The TCC stand-in for SA-M5's runner rows (escape2_darwin_test.go, TestOnAMacAStandInWithAccessibilityCantUseItInADesk).
// It tries what a program holding Accessibility could do with it, and prints what each gave as one JSON line:
//   trusted  AXIsProcessTrusted(): whether it holds the grant
//   ax       AXUIElementCopyAttributeValue of the focused app, through the system-wide element: 0 when it answered
//   tap      whether a listen-only event tap on key presses could be made; it is invalidated at once, never added to a
//            run loop, so it sees nothing
//   hid      IOServiceOpen of the HID system's parameter connection, the one IOHIDPostEvent posts through: 0 when it
//            opened; it is closed at once, and nothing is posted
// With one argument, it writes the line to that file instead (as an app LaunchServices opened would).
//
// Only on the Mac Studio runner, never on a Mac someone uses. Build it once, at a path that stays, and have a person
// grant it Accessibility in System Settings (a new build needs a new grant):
//   cc -o <folder>/tccstandin internal/sandbox/testdata/tccstandin.c -framework ApplicationServices -framework IOKit
// For ST1S's row (it also needs SMOLCORP_STUDIO_ROWS=1: no app is made, opened or run on a Mac someone uses, _rules.md),
// wrap it in an app whose program has the app's name, and grant the app:
//   mkdir -p <folder>/TCCStandIn.app/Contents/MacOS && cp <folder>/tccstandin <folder>/TCCStandIn.app/Contents/MacOS/TCCStandIn
//   (plus a Contents/Info.plist with CFBundleExecutable TCCStandIn, a CFBundleIdentifier of the runner's, LSUIElement 1)
// Then: SMOLCORP_STUDIO_ROWS=1 SMOLCORP_TEST_TCC_STANDIN=<folder>/tccstandin SMOLCORP_TEST_TCC_APP=<folder>/TCCStandIn.app \
//   go test -run TCC ./internal/sandbox/

#include <ApplicationServices/ApplicationServices.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/hidsystem/IOHIDLib.h>
#include <IOKit/hidsystem/IOHIDShared.h>
#include <stdio.h>

static CGEventRef seen(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *info) {
	(void)proxy;
	(void)type;
	(void)info;
	return event;
}

int main(int argc, char **argv) {
	int trusted = AXIsProcessTrusted() ? 1 : 0;

	AXUIElementRef everything = AXUIElementCreateSystemWide();
	CFTypeRef focused = NULL;
	int ax = (int)AXUIElementCopyAttributeValue(everything, kAXFocusedApplicationAttribute, &focused);
	if (focused != NULL) {
		CFRelease(focused);
	}
	CFRelease(everything);

	int tap = 0;
	CFMachPortRef port = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly,
		CGEventMaskBit(kCGEventKeyDown), seen, NULL);
	if (port != NULL) {
		tap = 1;
		CFMachPortInvalidate(port);
		CFRelease(port);
	}

	int hid = -1;
	io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching(kIOHIDSystemClass));
	if (service != IO_OBJECT_NULL) {
		io_connect_t connection = IO_OBJECT_NULL;
		hid = (int)IOServiceOpen(service, mach_task_self(), kIOHIDParamConnectType, &connection);
		if (hid == KERN_SUCCESS) {
			IOServiceClose(connection);
		}
		IOObjectRelease(service);
	}

	FILE *out = stdout;
	if (argc > 1 && (out = fopen(argv[1], "w")) == NULL) {
		return 2;
	}
	fprintf(out, "{\"trusted\":%s,\"ax\":%d,\"tap\":%s,\"hid\":%d}\n", trusted ? "true" : "false", ax, tap ? "true" : "false", hid);
	return out == stdout ? 0 : fclose(out);
}
