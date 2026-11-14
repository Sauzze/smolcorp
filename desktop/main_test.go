package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// realAskTailnet is Tailscale's look as the app does it, before the tests put a stand-in for it: for a test that
// points it at a stand-in `tailscale` of its own.
var realAskTailnet = askTailnet

// TestMain points every path the package keeps its state in at a fresh
// temporary folder before any test runs, so a test that forgets isolate(t)
// still can't touch the real app's files (on a Mac that folder is the Mac
// app's own, Application Support/AgentDeck). It removes the folders after.
func TestMain(m *testing.M) {
	// A row's stand-in for Smolcorp: this test program again, run as far as RunApp's first step (memory_test.go).
	if part := os.Getenv(memoryStandInVariable); part != "" {
		if part == "silent" { // one that never answers, as a window that never opens, and makes no folder to leave
			time.Sleep(time.Hour)
			os.Exit(1)
		}
		os.Exit(runWithTempSupport(func() int { return memoryStandIn(part) }))
	}
	os.Exit(runWithTempSupport(m.Run))
}

func runWithTempSupport(run func() int) int {
	realSupport := supportDir // already a temp folder: makeSupportDir sees a test binary
	root, err := os.MkdirTemp("", "smolcorp-desktop-main-")
	if err != nil {
		println("can't make a temp folder:", err.Error())
		return 1
	}
	defer os.RemoveAll(root)
	defer os.RemoveAll(realSupport)

	supportDir = filepath.Join(root, "support")
	homeDir = filepath.Join(root, "home")
	for _, dir := range []string{supportDir, homeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			println("can't make a temp folder:", err.Error())
			return 1
		}
	}
	phonesFile = filepath.Join(supportDir, "phones.json")
	configFile = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	windowKeyFile = filepath.Join(supportDir, "window-key")

	// No test inherits a test copy's folder from whatever ran it (a hire's start script sets this): a test
	// that wants one sets its own.
	os.Unsetenv(testSupportVariable)
	// Anything that asks the system for the home or config folder lands here too.
	os.Setenv("HOME", homeDir)
	os.Setenv("USERPROFILE", homeDir)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	os.Setenv("APPDATA", filepath.Join(homeDir, "AppData"))
	// Tailscale's command line is never run by a test: no Tailscale here, unless a test puts in its own.
	// The tmux that came with Smolcorp is looked for beside the real program: a test binary has none, and
	// a test that wants one gives its own (hirebundledtmux_test.go).
	bundledTmux = func() string { return "" }
	askTailnet = func() (tailnetAnswer, bool) { return tailnetAnswer{}, false }
	// Nor is one looked for along PATH, or in the places it is usually put: a test that wants one points
	// tailnetCandidates at its own stand-in (withStandInTailscale).
	tailnetLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	tailnetCandidates = nil
	// No test runs the claude installed here to learn its version: a stand-in new enough (claudeversion.go).
	claudeVersion = func() (string, bool) { return "2.1.323 (Claude Code)", true }
	// Nor is a stand-in claude refused for its signature (agentcheck.go): the rows that check the refusal put the
	// real check back, on stand-ins of their own.
	agentSignatureCheck = func(string) error { return nil }
	return run()
}
