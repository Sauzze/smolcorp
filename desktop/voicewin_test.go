package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"golang.org/x/term"
)

// VOICEWIN1: the words dictated on the phone (POST /dictate) or sent from the office's window (POST /talk/say)
// reach an agent in a desk on Windows, Return included, with the Mac's refusals. On Linux and Windows a desk is a
// launcher's window: the real `smolcorp run`, in a process of its own, starts its agent in a terminal of its own
// (Windows' pseudo-console, ConPTY, there; a pseudo-terminal elsewhere), and the agent here is a stand-in that writes
// down every byte it is given, so what it gets is what an agent would get. Fake words only; no microphone, no speech
// service and no real agent. The other tests of the words (phonetext_test.go, talksay_test.go) stop at the launcher's
// command line, with a fake launcher: this one goes through the launcher and its terminal too, and CI runs it by name
// on Windows (ci.yml).

// voiceAgentArgument makes this test program the stand-in agent (TestMain): its next argument is the file it writes
// what it's given to.
const voiceAgentArgument = "smolcorp-voice-agent-standin"

// The launcher stand-in's wiring (wireLauncherStandIn): the app's hook key, which a launcher reads from the support
// folder, and whether to say its console has a size (a desk's window always has one; the stand-in has no console).
const (
	launcherStandInKeyVariable  = "SMOLCORP_LAUNCHER_STANDIN_KEY"
	launcherStandInSizeVariable = "SMOLCORP_LAUNCHER_STANDIN_SIZE"
)

// wireLauncherStandIn readies the launcher stand-in, in its own temp support folder, before it runs: the hook key it
// connects to the test's deck with, and a console size, so it reads its agent's screen as a desk's launcher does.
func wireLauncherStandIn() bool {
	if key := os.Getenv(launcherStandInKeyVariable); key != "" {
		if writePrivate(hookKeyFile, []byte(key+"\n")) != nil {
			return false
		}
	}
	if os.Getenv(launcherStandInSizeVariable) != "" {
		consoleSize = func() (int, int, bool) { return 24, 100, true }
	}
	return true
}

// voiceAgentStandIn is a desk's agent, as far as typing goes: its terminal in raw mode, bracketed paste asked for and
// a prompt drawn, as Claude Code does, then every byte it's given added to record. record+".ready" says it's
// listening. It never outlives two minutes, whatever happens to the launcher that started it.
func voiceAgentStandIn(record string) int {
	time.AfterFunc(2*time.Minute, func() { os.Exit(7) })
	in := int(os.Stdin.Fd())
	saved, err := term.MakeRaw(in)
	if err != nil {
		return 4
	}
	defer term.Restore(in, saved)
	prepareConsole()
	fmt.Print(pasteOn + "> ")
	file, err := os.OpenFile(record, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 5
	}
	defer file.Close()
	if os.WriteFile(record+".ready", nil, 0o600) != nil {
		return 6
	}
	buffer := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buffer)
		if n > 0 {
			_, _ = file.Write(buffer[:n])
		}
		if err != nil {
			return 0
		}
	}
}

// pasteOn turns bracketed paste on (mode 2004).
const pasteOn = "\x1b[?2004h"

// terminalReports are what a terminal may add to an agent's input by itself: focus in and out, and the cursor's
// place. Never the words.
var terminalReports = regexp.MustCompile(`\x1b\[[IO]|\x1b\[\d+;\d+R`)

// voiceDesk is the stand-in's desk: its launcher's id, the file the agent writes what it's given to, and how the
// launcher's process ended (closed once it has).
type voiceDesk struct {
	id, record string
	ended      chan struct{}
}

// typed is what the agent was given, without the terminal's own reports.
func (desk voiceDesk) typed(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(desk.record)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return terminalReports.ReplaceAllString(string(data), "")
}

// waitTyped waits for the agent to have been given want, all told, as read, and says what it was given if it isn't.
func (desk voiceDesk) waitTyped(t *testing.T, want string, read func(string) string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		got := read(desk.typed(t))
		if got == want {
			return
		}
		if len(got) > len(want) || time.Now().After(deadline) {
			t.Fatalf("the agent was given %q, want %q", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// startVoiceDesk starts the real launcher, in a process of its own, with the stand-in agent, connected to d's own
// server on a port of its own, and waits for both: the launcher connected and the agent listening. Ended at cleanup
// as the office fires a launcher's agent (Stop), and the launcher must then end by itself, which it does only once its
// agent has: an agent left behind is a failure.
func startVoiceDesk(t *testing.T, d *Deck) voiceDesk {
	t.Helper()
	d.mu.Lock()
	key := d.access.HookKey
	d.mu.Unlock()
	server := newLocalServer(t, d)
	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	desk := voiceDesk{record: filepath.Join(t.TempDir(), "typed"), ended: make(chan struct{})}
	args, _ := json.Marshal([]string{self, voiceAgentArgument, desk.record})
	cmd := disclaim.Cmd(exec.Command(self, "-test.run=^$"))
	cmd.Env = append(os.Environ(), launcherStandInVariable+"="+string(args), launcherStandInKeyVariable+"="+key,
		launcherStandInSizeVariable+"=1", "SMOLCORP_PORT="+address.Port())
	cmd.Dir = t.TempDir()
	// What the launcher shows is its agent's screen: not kept. What it says of itself is.
	var said voiceLockedBuffer
	cmd.Stdout, cmd.Stderr = io.Discard, &said
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = cmd.Wait()
		close(desk.ended)
	}()
	t.Cleanup(func() {
		d.mu.Lock()
		tab := d.launchers[desk.id]
		d.mu.Unlock()
		if tab != nil {
			_ = tab.Close(Claude)
		}
		select {
		case <-desk.ended:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-desk.ended
			t.Errorf("the launcher didn't end once its agent was fired: its agent may be left (it ends itself within two minutes)")
		}
	})
	deadline := time.Now().Add(30 * time.Second)
	for desk.id == "" {
		d.mu.Lock()
		for id := range d.launchers {
			desk.id = id
		}
		d.mu.Unlock()
		select {
		case <-desk.ended:
			t.Fatalf("the launcher ended before it connected: %s", said.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the launcher didn't connect: %s", said.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	for {
		if _, err := os.Stat(desk.record + ".ready"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stand-in agent didn't start: %s", said.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	return desk
}

// voiceLockedBuffer is a buffer the launcher's output is copied into while the test reads it.
type voiceLockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *voiceLockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *voiceLockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// pasteLineBreaks reads each line break inside a bracketed paste as a line feed, as the terminal may write a return
// or both for one. Outside a paste nothing changes: there, a return is Return.
func pasteLineBreaks(typed string) string {
	var read strings.Builder
	for {
		before, rest, found := strings.Cut(typed, pasteStart)
		read.WriteString(before)
		if !found {
			return read.String()
		}
		inside, after, closed := strings.Cut(rest, pasteEnd)
		read.WriteString(pasteStart + strings.ReplaceAll(strings.ReplaceAll(inside, "\r\n", "\n"), "\r", "\n"))
		if !closed {
			return read.String()
		}
		read.WriteString(pasteEnd)
		typed = after
	}
}

// aLine is a line of words as an agent can be given it: as typed, or as one bracketed paste where the launcher saw
// its agent ask for that. Either is the words in the box; the paste's marks, if they are there, wrap them whole.
func aLine(got, words string) bool {
	return got == words || got == pasted(words)
}

// The words of a phone and of the office's window, typed into a stand-in agent through the real launcher and its
// terminal: on Windows, ConPTY.
func TestDictatedWordsReachAnAgentThroughTheRealLauncher(t *testing.T) {
	saved, wait := host, launcherPasteWait
	host, launcherPasteWait = nil, 15*time.Second // a CI runner can be slow: the words' own answer, not its timing, is what's checked
	t.Cleanup(func() { host, launcherPasteWait = saved, wait })
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	window := d.access.WindowKey
	d.mu.Unlock()
	phone := pair(t, d)
	desk := startVoiceDesk(t, d)
	tab := "launcher:" + desk.id
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", tab, "", 20)

	post := func(route, key, session, body string) (int, string) {
		got := serve(t, d, call{method: "POST", target: route + "?session=" + url.QueryEscape(session) + "&t=" + key,
			host: "localhost:7710", from: thisComputer, body: body})
		return got.Code, got.Body.String()
	}
	dictate := func(session, body string) (int, string) { return post("/dictate", phone, session, body) }
	say := func(session, body string) (int, string) { return post("/talk/say", window, session, body) }
	words := func(text string, send ...bool) string {
		fields := map[string]any{"text": text}
		if len(send) > 0 {
			fields["send"] = send[0]
		}
		body, _ := json.Marshal(fields)
		return string(body)
	}
	// Another Return may go at once: each case is its own.
	returnGoes := func() {
		d.mu.Lock()
		delete(d.lastReturn, "s1")
		d.mu.Unlock()
	}

	t.Run("refusals type nothing", func(t *testing.T) {
		hire, found := d.t3HireAtDesk(t3DeskID(t3DeskTestID))
		if !found {
			t.Fatal("no T3 Code desk")
		}
		asksAt(d, tab, 30)
		for _, c := range []struct {
			row, session, body string
			phone, window      int
			phoneSays, windows string
		}{
			{"a desk that asks", "s1", words("fix the stand-in login", true), http.StatusConflict, http.StatusConflict, answerFirst, windowAnswerFirst},
			{"a desk in T3 Code", t3DeskID(t3DeskTestID), words("fix the stand-in login", true), http.StatusConflict, http.StatusConflict,
				t3NoControls(calledHire(hire)), t3NoControls(calledHire(hire))},
		} {
			if status, says := dictate(c.session, c.body); status != c.phone || says != c.phoneSays {
				t.Errorf("%s, from the phone: %d %q", c.row, status, says)
			}
			if status, says := say(c.session, c.body); status != c.window || says != c.windows {
				t.Errorf("%s, from the window: %d %q", c.row, status, says)
			}
		}
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", tab, "", 40)
		for _, c := range []struct{ row, body, says string }{
			{"a shell command first", words("!rm -rf stand-in", true), noBangPrompt},
			{"too long", words(strings.Repeat("a", phoneTextLongest+1), true), tooLong},
			{"nothing", words(" \n ", true), "Nothing to type"},
		} {
			if status, says := dictate("s1", c.body); status != http.StatusBadRequest || says != c.says {
				t.Errorf("%s, from the phone: %d %q", c.row, status, says)
			}
			if status, says := say("s1", c.body); status != http.StatusBadRequest || says != c.says {
				t.Errorf("%s, from the window: %d %q", c.row, status, says)
			}
		}
		time.Sleep(500 * time.Millisecond)
		if got := desk.typed(t); got != "" {
			t.Fatalf("a refusal typed %q", got)
		}
	})

	before := ""
	// The words, then Return on its own: the line must be in the box whole, and Return the last thing given.
	sentLine := func(t *testing.T, line string) {
		t.Helper()
		desk.waitTypedEnding(t, before, line)
		before = desk.typed(t)
	}

	t.Run("the phone's words and Return", func(t *testing.T) {
		if status, says := dictate("s1", words("fix the stand-in login", true)); status != http.StatusNoContent {
			t.Fatalf("%d %q", status, says)
		}
		sentLine(t, "fix the stand-in login")
		// Whether the launcher saw its agent ask for bracketed paste through its terminal, and the paste's marks
		// reached the agent: Windows' ConPTY may not pass them on. Said, not required: one line goes either way.
		t.Logf("one line reached the agent inside a bracketed paste: %v", strings.HasPrefix(before, pasteStart))
	})

	t.Run("the window's words and Return", func(t *testing.T) {
		returnGoes()
		// The window sends unless it says not.
		if status, says := say("s1", words("deploy the stand-in when ready")); status != http.StatusNoContent {
			t.Fatalf("%d %q", status, says)
		}
		sentLine(t, "deploy the stand-in when ready")
	})

	t.Run("type only, no Return", func(t *testing.T) {
		returnGoes()
		if status, says := dictate("s1", words("stand-in draft only", false)); status != http.StatusNoContent {
			t.Fatalf("%d %q", status, says)
		}
		deadline := time.Now().Add(20 * time.Second)
		for !aLine(strings.TrimPrefix(desk.typed(t), before), "stand-in draft only") && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(phoneTextSettle + 500*time.Millisecond)
		if got := strings.TrimPrefix(desk.typed(t), before); !aLine(got, "stand-in draft only") {
			t.Fatalf("the agent was given %q after what it had", got)
		}
		before = desk.typed(t)
	})

	t.Run("several lines go as one paste or not at all", func(t *testing.T) {
		returnGoes()
		lines := "first stand-in line\nsecond stand-in line"
		status, says := say("s1", words(lines))
		switch {
		case status == http.StatusConflict && says == noLines:
			t.Log("several lines refused: the launcher saw no bracketed paste asked for")
			time.Sleep(500 * time.Millisecond)
			if got := desk.typed(t); got != before {
				t.Fatalf("refused, and the agent was given %q after what it had", strings.TrimPrefix(got, before))
			}
		case status == http.StatusNoContent:
			// Inside the paste, a line break may come as the terminal writes one; never outside it.
			desk.waitTyped(t, before+pasted(lines)+keyBytes[Enter], pasteLineBreaks)
			t.Log("several lines went as one bracketed paste")
		default:
			t.Fatalf("%d %q", status, says)
		}
	})
}

// waitTypedEnding waits for the agent to have been given line after before, then Return, and nothing more.
func (desk voiceDesk) waitTypedEnding(t *testing.T, before, line string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, had := strings.CutPrefix(desk.typed(t), before)
		if !had {
			t.Fatalf("what the agent had was changed: %q", desk.typed(t))
		}
		if body, sent := strings.CutSuffix(got, keyBytes[Enter]); sent && aLine(body, line) {
			return
		}
		if len(got) > len(pasted(line))+1 || time.Now().After(deadline) {
			t.Fatalf("the agent was given %q after what it had, want %q and Return", got, line)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
