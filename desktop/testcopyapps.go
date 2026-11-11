package desktop

import (
	"net/http"
	"os"
)

// What a test copy never does to another app (RTCH N1), as the Mac's TestCopy.OtherApp has it, word for
// word: a route that would bring a terminal app, T3 Code, the Grok app, System Settings or a system
// prompt in front of the owner's work answers with one of these instead. A copy on a port of its own
// counts, marked or not (testCopyPort): refusing is the way it fails closed. The office's own: nothing.
const (
	testCopyKeepsTerminal      = "This is a test copy of Smolcorp: it never brings a terminal app forward or opens a window in one."
	testCopyKeepsWindows       = "This is a test copy of Smolcorp: it never moves or tiles the windows of other apps."
	testCopyKeepsGrok          = "This is a test copy of Smolcorp: it never opens the Grok app."
	testCopyKeepsT3Code        = "This is a test copy of Smolcorp: it never opens T3 Code."
	testCopyKeepsAccessibility = "This is a test copy of Smolcorp: it never asks macOS for Accessibility or opens System Settings."
	testCopyKeepsFixes         = "This is a test copy of Smolcorp: it never opens another app, a download page or System Settings to fix something."
)

// testCopyMarked is whether this is a test copy by its own word: a port other than the office's, and
// SMOLCORP_TEST_SUPPORT_DIR beside it, as the Mac's TestCopy asks for both. Under the Mac app the shell
// always gives the core both. On another system a port alone is a copy that fails closed (testCopyPort:
// it listens on this computer only and writes no agent's settings) but isn't called a test copy.
func testCopyMarked() bool {
	return testCopyPort() != 0 && os.Getenv(testSupportVariable) != ""
}

// testCopyKeepsAway is the sentence a test copy answers when asked to do what why says, or "" for the office's own.
func testCopyKeepsAway(why string) string {
	if testCopyPort() == 0 {
		return ""
	}
	return why
}

// refusedInTestCopy writes a test copy's 409 for why, and says whether it did.
func refusedInTestCopy(w http.ResponseWriter, why string) bool {
	if testCopyKeepsAway(why) == "" {
		return false
	}
	writeStatus(w)(http.StatusConflict, why)
	return true
}
