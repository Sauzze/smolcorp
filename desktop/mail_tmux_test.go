//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A fake "claude" for a real tmux pane: it draws the shapes of the pane fixtures (an input box between two
// rules, a footer; a menu with a numbered list), reads what is typed one character at a time and notes it,
// so a test can tell what was typed, what was sent with Return, and what was left in the box. MODE is idle,
// busy, menu, draft (starts with a half-typed note) or askafter (idle; a menu 0.2 s after a paste ends, the box showing the paste until then).
// BRACKET=1 asks for bracketed paste. Files in DIR: got (every character as hex), submits (one line per Return).
const fakeClaudeScript = `#!/bin/bash
MODE=__MODE__; DIR=__DIR__; BRACKET=__BRACKET__
RULE="────────────────────────────────────────────────────────────────────────────────"
stty -echo -icanon -icrnl 2>/dev/null
[ "$BRACKET" = 1 ] && printf '\033[?2004h'
buf=""; inpaste=0
[ "$MODE" = draft ] && buf="half typed note"
: > "$DIR/got"; : > "$DIR/submits"
draw() {
  local out=" ▐▛███▛█   Claude Code v2.1.316"$'\n'"▝▜██████▀  Haiku 5.5 · Claude Ultra"$'\n'" ▝▝   ▝▝   /tmp/capdir"$'\n\n'
  if [ "$MODE" = menu ]; then
    out="${out} Bash command"$'\n'" Do you want to proceed?"$'\n'" ❯ 1. Yes"$'\n'"   2. No"$'\n'" Esc to cancel"
  else
    local first=1 line shown="" foot="  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents"
    [ "$MODE" = busy ] && foot="  ⏵⏵ auto mode on (shift+tab to cycle) · esc to interrupt"
    out="${out}${RULE}"$'\n'
    if [ -z "$buf" ]; then
      shown='❯ Try "refactor <filepath>"'; [ "$MODE" = busy ] && shown='❯ '
      out="${out}${shown}"$'\n'
    else
      while IFS= read -r line; do
        if [ $first = 1 ]; then out="${out}❯ ${line}"$'\n'; first=0; else out="${out}  ${line}"$'\n'; fi
      done <<< "$buf"
    fi
    out="${out}${RULE}"$'\n'"$foot"
  fi
  printf '\033[H\033[J%s' "$out"
}
draw
while IFS= read -rsn1 c; do
  if [ -z "$c" ]; then printf '0a ' >> "$DIR/got"; else printf '%02x ' "'$c" >> "$DIR/got"; fi
  if [ "$c" = $'\033' ]; then
    seq=""
    while IFS= read -rsn1 d; do seq="$seq$d"; printf '%02x ' "'$d" >> "$DIR/got"; [ "$d" = "~" ] && break; done
    case "$seq" in
      "[200~") inpaste=1 ;;
      "[201~") inpaste=0; if [ "$MODE" = askafter ]; then draw; sleep 0.2; MODE=menu; fi ;;
    esac
  elif [ "$MODE" = busy ] || [ "$MODE" = menu ]; then
    :
  elif [ "$c" = $'\r' ] || { [ -z "$c" ] && [ $inpaste = 0 ]; }; then
    printf '%s\n' "${buf//$'\n'/|}" >> "$DIR/submits"; buf=""
  elif [ -z "$c" ]; then
    buf="$buf"$'\n'
  else
    buf="$buf$c"
  fi
  [ $inpaste = 0 ] && draw
done
`

type fakeClaude struct {
	t   *testing.T
	h   *hireTmux
	dir string
}

// startFakeClaude runs the fake agent in a real tmux on a socket of the test's own, and ends it at the end.
func startFakeClaude(t *testing.T, id, mode string, bracketed bool) *fakeClaude {
	t.Helper()
	path, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux isn't installed")
	}
	shortSupport(t)
	saved := tmuxPath
	tmuxPath = func() string { return path }
	t.Cleanup(func() { tmuxPath = saved })
	h := newHireTmux(id, 0, nil)
	if h == nil {
		t.Fatal("no hire tmux")
	}
	dir := t.TempDir()
	bracket := "0"
	if bracketed {
		bracket = "1"
	}
	script := strings.NewReplacer("__MODE__", mode, "__DIR__", "'"+dir+"'", "__BRACKET__", bracket).Replace(fakeClaudeScript)
	agent := filepath.Join(dir, "claude.sh")
	if err := os.WriteFile(agent, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := h.start(dir, "/bin/sh", agent); !ok {
		t.Fatal("start failed")
	}
	// Ends only its own session, on its own socket.
	t.Cleanup(func() { h.end() })
	f := &fakeClaude{t: t, h: h, dir: dir}
	f.waitFor(func(screen string) bool {
		return strings.Contains(screen, "Claude Code") && strings.Contains(screen, "auto mode") || strings.Contains(screen, "Do you want")
	})
	return f
}

func (f *fakeClaude) waitFor(ok func(screen string) bool) {
	f.t.Helper()
	var screen string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		screen, _ = f.h.capture(40)
		if ok(screen) {
			return
		}
	}
	f.t.Fatalf("the pane shows %q", screen)
}

func (f *fakeClaude) read(name string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

// typed is every character the agent was sent, as hex; submits is one line per Return it was sent.
func (f *fakeClaude) typed() string   { return strings.TrimSpace(f.read("got")) }
func (f *fakeClaude) submits() string { return f.read("submits") }

// try is one delivery of text, with the asking answer given and the commits counted.
func (f *fakeClaude) try(text string, asking bool, commits *int, saves bool) mailResult {
	result := mailTypeInto(f.h, "Rev", text, func() bool { return asking }, func() bool { *commits++; return saves }, time.Sleep)
	time.Sleep(300 * time.Millisecond) // the agent reads what was typed a moment later
	return result
}

func TestAnIdleBoxTakesTheMailAndReturnIsPressedOnce(t *testing.T) {
	f := startFakeClaude(t, "mt-idle", "idle", true)
	var commits int
	got := f.try("Read notes/review-1.md and do it.", false, &commits, true)
	if got.Outcome != mailSent || got.Note != "" || !got.Typed || commits != 1 {
		t.Fatalf("result %+v, commits %d", got, commits)
	}
	if want := "Read notes/review-1.md and do it.\n"; f.submits() != want {
		t.Errorf("sent %q, want %q", f.submits(), want)
	}
	// The paste went in as a bracketed paste (200~ ... 201~), and the one Return came after it (the agent's
	// terminal shows it as 0a).
	typed := f.typed()
	if !strings.HasPrefix(typed, "1b 5b 32 30 30 7e ") || !strings.HasSuffix(typed, "1b 5b 32 30 31 7e 0a") || strings.Count(typed, "0a") != 1 {
		t.Errorf("typed %q", typed)
	}
}

func TestABusyWindowAMenuAndADraftAreNeverTypedInto(t *testing.T) {
	for _, c := range []struct{ mode, want string }{
		{"busy", "waiting: at work"},
		{"menu", "waiting: at a question"},
		{"draft", "waiting: a draft is in its box"},
	} {
		f := startFakeClaude(t, "mt-"+c.mode, c.mode, true)
		var commits int
		got := f.try("hello", false, &commits, true)
		if got.Outcome != mailWaiting || got.Note != c.want || got.Typed || commits != 0 {
			t.Errorf("%s: %+v, commits %d", c.mode, got, commits)
		}
		if f.typed() != "" || f.submits() != "" {
			t.Errorf("%s: something was typed: %q %q", c.mode, f.typed(), f.submits())
		}
	}
}

func TestTheAppKnowingItIsAskingStopsTheMailOverAnIdleBox(t *testing.T) {
	f := startFakeClaude(t, "mt-asking", "idle", true)
	var commits int
	if got := f.try("hello", true, &commits, true); got.Outcome != mailWaiting || got.Note != "waiting: at a question" || commits != 0 {
		t.Errorf("result %+v, commits %d", got, commits)
	}
	if f.typed() != "" {
		t.Errorf("typed %q", f.typed())
	}
}

func TestAWindowInCopyModeWaitsAndNothingIsTyped(t *testing.T) {
	f := startFakeClaude(t, "mt-copy", "idle", true)
	if !f.h.ok("copy-mode", "-t", hirePane) {
		t.Fatal("couldn't enter copy mode")
	}
	var commits int
	got := f.try("hello", false, &commits, true)
	if got.Outcome != mailWaiting || got.Note != "waiting: its window is scrolled back or in copy mode" || got.Typed || commits != 0 {
		t.Errorf("result %+v, commits %d", got, commits)
	}
	if f.typed() != "" {
		t.Errorf("typed %q", f.typed())
	}
}

func TestIfTheWindowBeginsAskingAfterThePasteReturnIsNotPressed(t *testing.T) {
	f := startFakeClaude(t, "mt-after", "askafter", true)
	var commits int
	got := f.try("hello", false, &commits, true)
	if got.Outcome != mailCannot || !got.Typed || commits != 1 || !strings.HasPrefix(got.Note, "The text was put in its box but not sent, because ") ||
		!strings.Contains(got.Note, "has to be cleared by hand") {
		t.Fatalf("result %+v, commits %d", got, commits)
	}
	if f.submits() != "" || strings.Contains(f.typed(), "0a") {
		t.Errorf("Return was pressed: %q %q", f.submits(), f.typed())
	}
	if !strings.Contains(f.typed(), "68 65 6c 6c 6f") { // "hello" was pasted
		t.Errorf("typed %q", f.typed())
	}
}

func TestAMultiLineMessageIsPastedAsItIsWhenTheAgentTakesBracketedPaste(t *testing.T) {
	f := startFakeClaude(t, "mt-multi", "idle", true)
	var commits int
	got := f.try("line one\nline two", false, &commits, true)
	if got.Outcome != mailSent || got.Note != "" {
		t.Fatalf("result %+v", got)
	}
	// Two lines in the box (the line feed inside the paste), then one Return: one submit.
	if f.submits() != "line one|line two\n" || strings.Count(f.typed(), "0a") != 2 || !strings.HasSuffix(f.typed(), "7e 0a") {
		t.Errorf("submits %q typed %q", f.submits(), f.typed())
	}
}

func TestAMultiLineMessageGoesAsOneLineWhenTheAgentDidNotAskForBracketedPaste(t *testing.T) {
	f := startFakeClaude(t, "mt-single", "idle", false)
	var commits int
	got := f.try("line one\n\n\tline two  \nthree", false, &commits, true)
	want := "Sent as one line (new lines shown as /): its window wasn't taking pastes as one piece."
	if got.Outcome != mailSent || got.Note != want {
		t.Fatalf("result %+v", got)
	}
	if f.submits() != "line one / line two / three\n" {
		t.Errorf("sent %q", f.submits())
	}
	// No line feed reached the agent inside the paste: it would have been read as Return. Only the Return itself.
	if strings.Count(f.typed(), "0a") != 1 || !strings.HasSuffix(f.typed(), " 0a") {
		t.Errorf("a line feed was typed: %q", f.typed())
	}
}

func TestAOneLineMessageHasNoNoteWithoutBracketedPaste(t *testing.T) {
	f := startFakeClaude(t, "mt-plain", "idle", false)
	var commits int
	if got := f.try("just one line", false, &commits, true); got.Outcome != mailSent || got.Note != "" {
		t.Fatalf("result %+v", got)
	}
	if f.submits() != "just one line\n" {
		t.Errorf("sent %q", f.submits())
	}
}

func TestIfTheTryCannotBeSavedNothingIsTyped(t *testing.T) {
	f := startFakeClaude(t, "mt-nosave", "idle", true)
	var commits int
	got := f.try("hello", false, &commits, false)
	if got.Outcome != mailTrouble || got.Typed || commits != 1 || got.Note != "The outbox couldn't be written, so nothing was typed." {
		t.Errorf("result %+v, commits %d", got, commits)
	}
	if f.typed() != "" {
		t.Errorf("typed %q", f.typed())
	}
}

func TestAControlCharacterIsRefusedBeforeAnythingIsTyped(t *testing.T) {
	f := startFakeClaude(t, "mt-ctrl", "idle", true)
	var commits int
	got := f.try("hello\x1b[201~\rrm -rf", false, &commits, true)
	if got.Outcome != mailCannot || got.Typed || commits != 0 || got.Note != "The message holds a control character that can't be pasted safely." {
		t.Errorf("result %+v, commits %d", got, commits)
	}
	if f.typed() != "" {
		t.Errorf("typed %q", f.typed())
	}
}

func TestASessionThatIsNotRunningIsTroubleAndNothingIsTyped(t *testing.T) {
	f := startFakeClaude(t, "mt-gone", "idle", true)
	f.h.end()
	var commits int
	got := f.try("hello", false, &commits, true)
	if got.Outcome != mailTrouble || got.Note != "Rev's tmux session isn't running." || got.Typed || commits != 0 {
		t.Errorf("result %+v, commits %d", got, commits)
	}
}

// The pasting itself, with a recorder: the exact tmux calls, on our socket only.
func TestThePasteIsOneBracketedPasteThroughANamedBufferAndReturnIsSeparate(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	if !h.paste("a\nb") || !h.submit() {
		t.Fatal("paste or submit failed")
	}
	want := [][]string{
		{"-S", socket, "set-buffer", "-b", "smolcorp-paste", "--", "a\nb"},
		{"-S", socket, "paste-buffer", "-p", "-r", "-d", "-b", "smolcorp-paste", "-t", "=hire:"},
		{"-S", socket, "send-keys", "-t", "=hire:", "Enter"},
	}
	if !reflect.DeepEqual(r.asked, want) {
		t.Errorf("asked %q", r.asked)
	}
	r.asked = nil
	if h.paste("bad\x1b") || len(r.asked) != 0 {
		t.Errorf("a control character was pasted: %q", r.asked)
	}
}

func TestPasteStateReadsTheTwoFlags(t *testing.T) {
	r := newRecorder()
	h, socket := fakeHireTmux(t, r, 0)
	for out, want := range map[string]pasteMode{"0 1": {false, true}, "1 0": {true, false}, "0 0": {false, false}} {
		r.output = out
		if got, ok := h.pasteState(); !ok || got != want {
			t.Errorf("%q gave %+v %v", out, got, ok)
		}
	}
	if want := []string{"-S", socket, "display-message", "-p", "-t", "=hire:", "#{pane_in_mode} #{bracket_paste_flag}"}; !reflect.DeepEqual(r.asked[0], want) {
		t.Errorf("asked %q", r.asked[0])
	}
	for _, bad := range []string{"", "1", "0 1 2", "a b", "0 x"} {
		r.output = bad
		if _, ok := h.pasteState(); ok {
			t.Errorf("%q was read", bad)
		}
	}
	r.output, r.status = "0 1", 1
	if _, ok := h.pasteState(); ok {
		t.Error("a failed call was read")
	}
}

func TestMailOneLineIsTheMacsOneLine(t *testing.T) {
	for in, want := range map[string]string{
		"a\nb":            "a / b",
		"a\n\n  b \t\n c": "a / b / c",
		"\tx\ty":          "x y",
		"one":             "one",
		"a b":             "a / b",
	} {
		if got := mailOneLine(in); got != want {
			t.Errorf("%q gave %q, want %q", in, got, want)
		}
	}
}

func TestMailTypesOnlyOnTheHiresOwnExactSocket(t *testing.T) {
	shortSupport(t)
	tmuxPath = func() string { return "/usr/bin/tmux" }
	own := filepath.Join(supportDir, "smolcorp-ab12-cd34")
	listenAt(t, own)
	if h := mailHireTmux(Hire{ID: "ab12-cd34", TmuxSocket: own}); h == nil || h.socket != own {
		t.Errorf("the hire's own socket: %+v", h)
	}
	other := filepath.Join(supportDir, "smolcorp-zz")
	listenAt(t, other)
	plain := filepath.Join(supportDir, "smolcorp-ab12-cd35")
	os.WriteFile(plain, nil, 0o600)
	link := filepath.Join(supportDir, "smolcorp-ab12-cd36")
	os.Symlink(own, link)
	elsewhere := filepath.Join(t.TempDir(), "smolcorp-ab12-cd34")
	for name, h := range map[string]Hire{
		"no socket recorded (once derived from the id)": {ID: "ab12-cd34"},
		"another hire's socket":                         {ID: "ab12-cd34", TmuxSocket: other},
		"the user's own tmux":                           {ID: "ab12-cd34", TmuxSocket: "/tmp/tmux-501/default"},
		"a plain file named as the hire's":              {ID: "ab12-cd35", TmuxSocket: plain},
		"a link named as the hire's":                    {ID: "ab12-cd36", TmuxSocket: link},
		"the right name in another folder":              {ID: "ab12-cd34", TmuxSocket: elsewhere},
		"a test copy's socket name on a real install":   {ID: "ab12-cd34", TmuxSocket: filepath.Join(supportDir, "smolcorp-7799-ab12-cd34")},
		"a bad id": {ID: "../x", TmuxSocket: own},
	} {
		if got := mailHireTmux(h); got != nil {
			t.Errorf("%s: typed on %+v", name, got)
		}
	}
}
