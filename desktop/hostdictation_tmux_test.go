//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The office's mic on a tmux desk with no window open (1.26, DICT126): the Mac app records from a stand-in, and
// what it heard reaches the pane through this program's own tmux paste. Never a real microphone or speech
// engine, and no real Claude Code: the pane runs cat, in raw mode, which has asked for bracketed paste as Claude
// Code does, under the name claude so the deck finds its agent in front.

// dictationPane is a real tmux server on a socket of its own, and in it a pane whose front process is a `claude`
// that is cat, writing every byte it is given to out, with the bracketed paste asked for. The server is ended by
// its PID, and nothing else.
func dictationPane(t *testing.T) (pane commandTab, out string) {
	t.Helper()
	needTmux(t)
	// Short: a socket's path has a length limit. The system's temporary folder, never /tmp itself.
	dir, err := os.MkdirTemp("", "dkt")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("no cat here")
	}
	claude := filepath.Join(dir, "claude")
	if err := os.Symlink(cat, claude); err != nil {
		t.Fatal(err)
	}
	socket, out := filepath.Join(dir, "s"), filepath.Join(dir, "out")
	tmux := func(args ...string) string {
		output, err := exec.Command("tmux", append([]string{"-S", socket, "-f", "/dev/null"}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(output))
	}
	tmux("new-session", "-d", "-x", "80", "-y", "24", "sh", "-c",
		`stty raw -echo; printf '\033[?2004h'; touch `+out+`.ready; exec `+claude+` > `+out)
	pid, err := strconv.Atoi(tmux("display-message", "-p", "#{pid}"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		os.RemoveAll(dir)
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(out + ".ready"); err == nil {
			break
		}
	}
	time.Sleep(150 * time.Millisecond) // the claude in front
	pane = commandTab{kind: "tmux", id: tmux("display-message", "-p", "#{pane_id}"), socket: socket}
	if running, known := pane.runs(Claude); !running || !known {
		t.Fatalf("the pane's agent in front: running %v, known %v", running, known)
	}
	return pane, out
}

// tmuxDictationDeck is a deck under the stand-in Mac app that can dictate, voice on, and a Claude Code desk, idle
// at its prompt, in tab: nothing has a window open for it.
func tmuxDictationDeck(t *testing.T, tab string) (*Deck, *macStandIn) {
	t.Helper()
	mac := withMac(t)
	mac.facts.Dictation.Available, mac.facts.Dictation.Allowed = true, true
	d := newTestDeck(t)
	d.macFactsNow()
	d.mu.Lock()
	d.config.DictationEnabled, d.config.DictationForClaude = true, true
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "claude-1", "cwd": "/work/smolcorp"}, Claude, tab, "", 100)
	d.sessions.Apply(map[string]any{"hook_event_name": "Stop", "session_id": "claude-1", "cwd": "/work/smolcorp"}, Claude, tab, "", 101)
	// Learned for this tab, so the office doesn't ask again (hostoffice.go).
	d.mac.abilities, d.mac.abilitiesFor = map[string]tabAbilities{tab: {Keys: true, Text: true, Fire: true}}, tab
	d.mu.Unlock()
	return d, mac
}

// dictated holds the mic on id, says what the stand-in heard, lets go, and waits for the Mac app to have been
// asked to stop: after that the text is on its way to the pane, or has been dropped.
func dictated(t *testing.T, d *Deck, mac *macStandIn, id, heard string) {
	t.Helper()
	stops := len(askedFor(mac, "dictation-stop"))
	mac.mu.Lock()
	mac.heard = heard
	mac.mu.Unlock()
	if status, text := d.PerformFrom("phone-1", "mic", id, url.Values{"phase": {"down"}}); status != http.StatusNoContent {
		t.Fatalf("the mic held: %d %q", status, text)
	}
	if status, _ := d.Perform("mic", "", url.Values{"phase": {"up"}}); status != http.StatusNoContent {
		t.Fatalf("the mic let go: %d", status)
	}
	eventually(t, "the recording stopped", func() bool { return len(askedFor(mac, "dictation-stop")) == stops+1 })
	eventually(t, "the deck done with it", func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.dictating == nil
	})
}

// quiet waits a little, and says what out holds then.
func quiet(path string) string {
	time.Sleep(400 * time.Millisecond)
	data, _ := os.ReadFile(path)
	return string(data)
}

// The mic held on a tmux desk with no window records through the Mac app, and, let go, what it heard is pasted
// into that desk's pane, on its own socket, bracketed, as one piece with no Return. It is never pasted while the
// desk asks, as the phone's words and the window's aren't. Text the owner has put in the box already is not a
// reason to refuse it, which a second sentence would then always be (nothing sends the first): it goes after it.
func TestTheMicDictatesIntoATmuxDeskWithNoWindow(t *testing.T) {
	pane, out := dictationPane(t)
	// Another server, with a pane of the same id, that must hear nothing: the paste goes by the desk's socket.
	decoy, decoyOut := dictationPane(t)
	if decoy.id != pane.id || decoy.socket == pane.socket {
		t.Fatalf("a decoy %+v beside %+v", decoy, pane)
	}
	tab := "tmux:" + pane.id + "@" + pane.socket
	d, mac := tmuxDictationDeck(t, tab)

	entry, dictation := sessionSaid(t, d, "claude-1")
	if dictation["on"] != true || entry["dictates"] != true || entry["mic"] != true {
		t.Fatalf("dictation %v, the desk's dictates %v and mic %v", dictation, entry["dictates"], entry["mic"])
	}

	// One sentence, with what a shell would take for something else, and some accents.
	heard := "Fix the build, \"s'il vous plaît\" $(touch canary) `x`; ça marche 👩‍💻"
	dictated(t, d, mac, "claude-1", heard)
	started := askedFor(mac, "dictation-start")
	if len(started) != 1 || started[0].Tab != tab || started[0].Language != "en_US" {
		t.Fatalf("the app was asked %+v", started)
	}
	want := pasteStart + heard + pasteEnd
	if got := readAfter(t, out, len(want)); string(got) != want {
		t.Fatalf("the pane got %q, want %q", got, want)
	}
	// Nothing after it: no Return, no key, nothing through the Mac app's own typing, nothing left in tmux.
	if got := quiet(out); got != want {
		t.Errorf("a moment later the pane has %q", got)
	}
	if strings.ContainsAny(want, "\r") || strings.Contains(strings.TrimSuffix(strings.TrimPrefix(want, pasteStart), pasteEnd), "\n") {
		t.Fatal("the test's own text has a Return in it")
	}
	if len(askedFor(mac, "insert")) != 0 || len(askedFor(mac, "press")) != 0 || len(askedFor(mac, "type")) != 0 {
		t.Errorf("the Mac app typed: %v", mac.actions())
	}
	if buffers, _ := pane.output("list-buffers"); strings.TrimSpace(string(buffers)) != "" {
		t.Errorf("a buffer was left in tmux: %s", buffers)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(out), "canary")); err == nil {
		t.Error("something ran")
	}
	if got := quiet(decoyOut); got != "" {
		t.Errorf("another server's pane with the same id heard %q", got)
	}

	// The desk asks you something: the dictation is recorded and dropped, said over the desk, and nothing is pasted.
	before := quiet(out)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "claude-1", "cwd": "/work/smolcorp", "tool_name": "Bash"}, Claude, tab, "", 200)
	d.mu.Unlock()
	if entry, _ := sessionSaid(t, d, "claude-1"); entry["state"] != string(Asking) {
		t.Fatalf("the desk's state %v", entry["state"])
	}
	dictated(t, d, mac, "claude-1", "yes to all of it")
	eventually(t, "the desk told", func() bool {
		entry, _ := sessionSaid(t, d, "claude-1")
		note, _ := entry["voiceNote"].(map[string]any)
		return note["text"] == dictationAsking
	})
	if got := quiet(out); got != before {
		t.Errorf("pasted while asking: the pane has %q, had %q", got, before)
	}
	if len(askedFor(mac, "dictation-clean")) != 2 {
		t.Errorf("the second dictation was heard and cleaned up all the same: %d clean-ups asked", len(askedFor(mac, "dictation-clean")))
	}

	// Answered, and the owner has begun a message of their own in the box: the dictation goes in after it.
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PostToolUse", "session_id": "claude-1", "cwd": "/work/smolcorp", "tool_name": "Bash"}, Claude, tab, "", 300)
	d.sessions.Apply(map[string]any{"hook_event_name": "Stop", "session_id": "claude-1", "cwd": "/work/smolcorp"}, Claude, tab, "", 301)
	d.mu.Unlock()
	if err := pane.run("send-keys", "-t", pane.id, "-l", "my own words, "); err != nil {
		t.Fatal(err)
	}
	draft := before + "my own words, "
	if got := readAfter(t, out, len(draft)); string(got) != draft {
		t.Fatalf("the owner's text: %q, want %q", got, draft)
	}
	dictated(t, d, mac, "claude-1", "and then this")
	want = draft + pasteStart + "and then this" + pasteEnd
	if got := readAfter(t, out, len(want)); string(got) != want {
		t.Errorf("the pane got %q, want %q", got, want)
	}
	if got := quiet(out); got != want {
		t.Errorf("a moment later the pane has %q", got)
	}
	if got := quiet(decoyOut); got != "" {
		t.Errorf("another server's pane with the same id heard %q", got)
	}
}

// Under the Mac app a tmux desk with no window is offered the mic whatever the setup's switches say, as every
// desk is: dictation for an agent without a voice key of its own, and for Claude Code when setup says so or its
// clean-up is on; else Claude Code's own voice key, which tmux can press. Voice off, or a Mac that can't dictate,
// leaves the mic to the agent's own key, as before. Every combination of the switches, for both kinds of agent.
func TestATmuxDeskIsOfferedTheMicUnderEverySwitch(t *testing.T) {
	const tab = "tmux:%7@/nowhere/dictation-socket"
	d, mac := tmuxDictationDeck(t, tab)
	d.mu.Lock()
	// Codex in a desk of its own, as an agent has its tab to itself.
	const codexTab = "tmux:%8@/nowhere/dictation-socket"
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "codex-1", "cwd": "/work/other"}, Codex, codexTab, "", 102)
	d.mac.abilities, d.mac.abilitiesFor = map[string]tabAbilities{tab: {Keys: true, Text: true, Fire: true}, codexTab: {Keys: true, Text: true, Fire: true}}, tab+"\n"+codexTab
	d.mu.Unlock()
	mac.mu.Lock()
	mac.heard = "" // nothing heard: nothing is typed into a socket that isn't there
	mac.mu.Unlock()

	for _, enabled := range []bool{false, true} {
		for _, canDictate := range []bool{false, true} {
			for _, forClaude := range []bool{false, true} {
				for _, cleanup := range []bool{false, true} {
					d.mu.Lock()
					d.config.DictationEnabled, d.mac.canDictate = enabled, canDictate
					d.config.DictationForClaude, d.config.DictationCleanup = forClaude, cleanup
					claude, codex := d.dictatesFor(tab, Claude), d.dictatesFor(codexTab, Codex)
					d.mu.Unlock()
					on := enabled && canDictate
					// Claude Code has a voice key: dictation only when asked for. Codex has none: always.
					wantClaude, wantCodex := on && (forClaude || cleanup), on
					name := "voice " + strconv.FormatBool(enabled) + ", Mac can " + strconv.FormatBool(canDictate) +
						", for Claude " + strconv.FormatBool(forClaude) + ", clean-up " + strconv.FormatBool(cleanup)
					if claude != wantClaude || codex != wantCodex {
						t.Errorf("%s: dictates for Claude %v (want %v), for Codex %v (want %v)", name, claude, wantClaude, codex, wantCodex)
					}
					// What the office shows on each desk: a mic wherever something answers it.
					for id, want := range map[string]struct{ dictates, mic bool }{
						"claude-1": {wantClaude, true},     // its own voice key, or dictation
						"codex-1":  {wantCodex, wantCodex}, // dictation, or nothing
					} {
						entry, _ := sessionSaid(t, d, id)
						if entry["dictates"] != want.dictates || entry["mic"] != want.mic {
							t.Errorf("%s, %s: the desk shows dictates %v and mic %v, want %v and %v", name, id, entry["dictates"], entry["mic"], want.dictates, want.mic)
						}
					}
					// Held, it starts the recording exactly where it dictates; elsewhere it isn't asked here (a
					// Space for Claude Code's own mode, nothing for Codex).
					for id, dictates := range map[string]bool{"claude-1": wantClaude, "codex-1": wantCodex} {
						if !dictates {
							continue
						}
						starts := len(askedFor(mac, "dictation-start"))
						dictated(t, d, mac, id, "")
						if got := askedFor(mac, "dictation-start"); len(got) != starts+1 || got[starts].Tab != map[string]string{"claude-1": tab, "codex-1": codexTab}[id] {
							t.Errorf("%s, %s: the app was asked %+v", name, id, got)
						}
					}
				}
			}
		}
	}
}
