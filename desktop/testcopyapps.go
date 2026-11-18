package desktop

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// What a test copy never does to another app (RTCH N1), or to the owner's music and microphone (RWTC N2), as the
// Mac's TestCopy.OtherApp has it, word for word: a route that would bring a terminal app, T3 Code, the Grok app,
// System Settings or a system prompt in front of the owner's work answers with one of these instead. A copy on a
// port of its own counts, marked or not (testCopyPort): refusing is the way it fails closed. The office's own: nothing.
const (
	testCopyKeepsTerminal      = "This is a test copy of Smolcorp: it never brings a terminal app forward, opens a window in one or touches its tabs."
	testCopyKeepsWindows       = "This is a test copy of Smolcorp: it never moves or tiles the windows of other apps."
	testCopyKeepsGrok          = "This is a test copy of Smolcorp: it never opens the Grok app."
	testCopyKeepsT3Code        = "This is a test copy of Smolcorp: it never opens T3 Code."
	testCopyKeepsAccessibility = "This is a test copy of Smolcorp: it never asks macOS for Accessibility or opens System Settings."
	testCopyKeepsFixes         = "This is a test copy of Smolcorp: it never opens another app, a download page or System Settings to fix something."
	testCopyKeepsMusic         = "This is a test copy of Smolcorp: it never plays or skips your music, or changes the volume."
	testCopyKeepsMicrophone    = "This is a test copy of Smolcorp: it never asks for the microphone or turns dictation on."
)

// What a test copy on Linux or Windows says of the self-test's login and install fixes (RWTC N3), as the Mac's
// SelfTest says them (testCopyLogin, FixError.testCopy), in this computer's words: opening at login is the
// Smolcorp in use's, and a copy never installs itself over it.
const (
	testCopyKeepsLogin   = "A test copy leaves opening at login to the Smolcorp you use."
	testCopyKeepsInstall = "A test copy stays where it was built."
)

// testCopyComputerWords are the sentences above that name the Mac's own parts, as they're said off the Mac
// (RWTC N5). Under the Mac app they stay the Mac's.
var testCopyComputerWords = map[string]string{
	testCopyKeepsFixes: "This is a test copy of Smolcorp: it never opens another app, a download page or this computer's settings to fix something.",
}

// testCopyNever is how every sentence of a marked test copy starts but login's and install's.
const testCopyNever = "This is a test copy of Smolcorp: it never "

// testCopyMarked is whether this is a test copy by its own word: a port other than the office's, and
// SMOLCORP_TEST_SUPPORT_DIR beside it, as the Mac's TestCopy asks for both. Under the Mac app the shell
// always gives the core both. On another system a port alone is a copy that fails closed (testCopyPort:
// it listens on this computer only and writes no agent's settings) but isn't called a test copy.
func testCopyMarked() bool {
	return testCopyPort() != 0 && os.Getenv(testSupportVariable) != ""
}

// testCopyKeepsAway is the sentence a test copy answers when asked to do what why says, or "" for the office's own:
// in this computer's words off the Mac, and, on a copy that is only on another port, in words that don't call it a
// test copy (RWTC N4).
func testCopyKeepsAway(why string) string {
	if testCopyPort() == 0 {
		return ""
	}
	if computer, ok := testCopyComputerWords[why]; ok && host == nil {
		why = computer
	}
	if testCopyMarked() {
		return why
	}
	return onAPortOfItsOwn(why)
}

// onAPortOfItsOwn is why, as a copy on another port that isn't marked a test copy says it (a real install on
// SMOLCORP_PORT, say): "Smolcorp on port 7799 never opens the Grok app: only the one on port 7710 does that."
func onAPortOfItsOwn(why string) string {
	office := portFromEnvironment("")
	switch why {
	case testCopyKeepsLogin:
		return fmt.Sprintf("Smolcorp on port %d leaves opening at login to the one on port %d.", Port, office)
	case testCopyKeepsInstall:
		return fmt.Sprintf("Smolcorp on port %d stays where it was built: only the one on port %d installs itself.", Port, office)
	}
	rest, ok := strings.CutPrefix(why, testCopyNever)
	if !ok {
		return why
	}
	return fmt.Sprintf("Smolcorp on port %d never %s: only the one on port %d does that.", Port, strings.TrimSuffix(rest, "."), office)
}

// testCopyKeepsTab is a test copy's answer to anything done in the tab named name (RWTC N1): keys, text, a scroll,
// a look at its screen, a close. Only a tmux pane on one of the copy's own hire sockets is its own; any other tab
// is in one of the owner's terminal apps, a launcher of theirs, or on a tmux server of theirs. "" for the office's own.
func testCopyKeepsTab(name string) string {
	testPort := testCopyPort()
	if testPort == 0 {
		return ""
	}
	if kind, _, socket := splitTab(name); kind == "tmux" && socket != "" && ownsHireSocket(socket, testPort) {
		return ""
	}
	return testCopyKeepsAway(testCopyKeepsTerminal)
}

// refusedInTestCopy writes a test copy's 409 for why, and says whether it did.
func refusedInTestCopy(w http.ResponseWriter, why string) bool {
	if why = testCopyKeepsAway(why); why == "" {
		return false
	}
	writeStatus(w)(http.StatusConflict, why)
	return true
}
