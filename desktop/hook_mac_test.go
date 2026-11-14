package desktop

import (
	"os"
	"runtime"
	"testing"
)

// A process tree as ps shows it: pid → parent, tty and program.
type fakeTree map[int]struct {
	parent       int
	tty, program string
}

func (f fakeTree) process(pid int) (int, string, string, bool) {
	p, ok := f[pid]
	return p.parent, p.tty, p.program, ok
}

// On a Mac, the hook names Terminal, iTerm2, Ghostty and other: tabs by its tty, as report.sh does.
func TestTheHookNamesAMacTerminalsTabAsReportSh(t *testing.T) {
	// The hook (99) runs under claude (50), in a shell (40) on ttys003, in a login (30), under Terminal (20).
	inTerminal := fakeTree{
		99: {50, "??", "/usr/bin/sh"},
		50: {40, "ttys003", "claude"},
		40: {30, "ttys003", "-zsh"},
		30: {20, "ttys003", "login"},
		20: {1, "??", "/System/Applications/Utilities/Terminal.app/Contents/MacOS/Terminal"},
	}
	elsewhere := fakeTree{99: {50, "??", "sh"}, 50: {40, "ttys007", "claude"}, 40: {1, "??", "/Applications/Other.app/Contents/MacOS/Other"}}
	noTTY := fakeTree{99: {50, "??", "sh"}, 50: {1, "??", "claude"}}
	event := []byte(`{"hook_event_name":"Stop","session_id":"abc-123"}`)
	for _, c := range []struct {
		name  string
		agent Agent
		env   map[string]string
		tree  fakeTree
		tab   string
		send  bool
	}{
		{"Terminal", Claude, map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, inTerminal, "tty:/dev/ttys003", true},
		{"Terminal, its variable dropped", Claude, nil, inTerminal, "tty:/dev/ttys003", true},
		{"iTerm2", Claude, map[string]string{"TERM_PROGRAM": "iTerm.app", "ITERM_SESSION_ID": "w0t1p0:AB12-CD34"}, elsewhere, "iterm:AB12-CD34", true},
		{"iTerm2, a session id of another shape", Claude, map[string]string{"ITERM_SESSION_ID": "w0t1p0:AB\r\nX-Other: 1"}, elsewhere, "other:/dev/ttys007", true},
		{"Ghostty", Gemini, map[string]string{"TERM_PROGRAM": "ghostty"}, elsewhere, "ghostty:/dev/ttys007", true},
		{"another terminal", Claude, map[string]string{"TERM_PROGRAM": "vscode"}, elsewhere, "other:/dev/ttys007", true},
		{"no TERM_PROGRAM, no Terminal up the tree", Claude, nil, elsewhere, "other:/dev/ttys007", true},
		{"tmux inside Terminal, tmux first", Claude, map[string]string{"TERM_PROGRAM": "Apple_Terminal", "TMUX_PANE": "%3", "TMUX": "/tmp/tmux-501/default,123,0"}, inTerminal, "tmux:%3@/tmp/tmux-501/default", true},
		{"tmux with no socket", Claude, map[string]string{"TMUX_PANE": "%3"}, inTerminal, "other:/dev/ttys003", true},
		{"tmux, a pane of another shape", Claude, map[string]string{"TMUX_PANE": "%3\nX", "TMUX": "/tmp/s,1,0"}, inTerminal, "other:/dev/ttys003", true},
		{"WezTerm", Claude, map[string]string{"WEZTERM_PANE": "4", "WEZTERM_UNIX_SOCKET": "/tmp/wez"}, elsewhere, "wezterm:4@/tmp/wez", true},
		{"kitty", Claude, map[string]string{"KITTY_WINDOW_ID": "2", "KITTY_LISTEN_ON": "unix:/tmp/kitty"}, elsewhere, "kitty:2@unix:/tmp/kitty", true},
		{"kitty, a socket with a line break", Claude, map[string]string{"KITTY_WINDOW_ID": "2", "KITTY_LISTEN_ON": "unix:/tmp/k\n"}, elsewhere, "other:/dev/ttys007", true},
		{"the launcher's session, first", Claude, map[string]string{"SMOLCORP_SESSION": "s1", "TERM_PROGRAM": "Apple_Terminal"}, inTerminal, "launcher:s1", true},
		{"no tty: nothing sent", Claude, nil, noTTY, "", false},
		{"no tty, headless", Claude, map[string]string{"SMOLCORP_HEADLESS": "1"}, noTTY, "headless:abc-123", true},
		{"no tty, Gemini says headless: nothing sent", Gemini, map[string]string{"SMOLCORP_HEADLESS": "1"}, noTTY, "", false},
		{"no tty, OpenCode: found by its folder", OpenCode, nil, noTTY, "", true},
		{"Codex: found by its folder", Codex, map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, inTerminal, "", true},
		{"Codex with a stale SMOLCORP_SESSION: found by its folder", Codex, map[string]string{"SMOLCORP_SESSION": "other-desk"}, inTerminal, "", true},
	} {
		env := func(name string) string { return c.env[name] }
		tab, _, send := macHookTab(c.agent, event, env, c.tree.process, 99)
		if tab != c.tab || send != c.send {
			t.Errorf("%s: tab %q, send %v; want %q, %v", c.name, tab, send, c.tab, c.send)
		}
	}
	if _, tty, _ := macHookTab(Claude, event, func(string) string { return "" }, inTerminal.process, 99); tty != "/dev/ttys003" {
		t.Errorf("the tty is %q", tty)
	}
}

// ps itself, as the hook reads this test's own process: its parent, its tty (?? for none) and its program.
func TestPsReadsAProcess(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the Mac's ps")
	}
	parent, tty, program, ok := psProcess(os.Getpid())
	if !ok || parent != os.Getppid() || tty == "" || program == "" {
		t.Fatalf("ps read %d %q %q %v", parent, tty, program, ok)
	}
}
