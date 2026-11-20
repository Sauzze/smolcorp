package desktop

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
)

// A screen that breaks says what broke it, by the sequence's own bytes or a kind of character, never the
// screen's text; the answer carries it, and the app logs it once per launcher and reason.
func TestABrokenScreenSaysWhatBrokeItOnceAndNeverItsText(t *testing.T) {
	for _, c := range []struct{ out, want string }{
		{"\x1b[3b", "CSI 3b"}, {"\x1b[?7l", "CSI ?7l"}, {"\x1bH", "ESC H"}, {"\x0e", "control byte 0x0e"},
		{"\xff", "bad UTF-8"}, {"‍", "a character of unsure width"}, {"\x1b(0", "ESC ( 0"},
		{"\x1b]0;" + strings.Repeat("x", 5000), "a string over 4096 bytes"},
		{"\x1bPq#0~\x1b\\", "DCS q"}, {"\x1b_Ga=T;AAAA\x1b\\", "APC G"}, {"\x1b[?045h", "CSI ?045h"}, {"\x1b[!p", "CSI !p"},
		{"x\x1b[3b\x1b7\x1b[2J\x1b[H\x1b8", "a cursor restored that was saved while the screen couldn't be read"},
	} {
		s := drawn(t, 10, 40, "secret plans\r\n"+c.out+"\x1b[3b")
		if got := s.Why(); got != c.want {
			t.Errorf("%q: why %q, want %q", c.out, got, c.want)
		}
	}
	s := drawn(t, 10, 40, "a")
	s.Resize(10, 30)
	if got := s.Why(); got != "a narrower window" {
		t.Errorf("narrower: %q", got)
	}
	if s.Write([]byte("\x1b[2J\x1b[H")); s.Why() != "" {
		t.Errorf("redrawn, still says %q", s.Why())
	}
	if got := newLauncherScreen(24, 80, false, westernCP).screen.Why(); got != "a console whose size couldn't be read" {
		t.Errorf("no size: %q", got)
	}

	// The answer: broke only for an unknown screen.
	var seen []string
	client := &http.Client{Transport: &captureTransport{seen: &seen}}
	broken := teedScreen(t, 60, 200, "secret plans\x1b[3b", 4096)
	postScreenAnswer(client, "http://127.0.0.1:1", "fake-key", "fake-proof", "L1", "3", broken)
	if len(seen) != 1 || seen[0] != `{"look":"3","state":"unknown","unknown":true,"bracketed":false,"broke":"CSI 3b"}` {
		t.Errorf("the answer was %q", seen)
	}

	// The app logs it once per launcher and reason, whatever the order.
	var logged bytes.Buffer
	was, flags := log.Writer(), log.Flags()
	log.SetOutput(&logged)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(was); log.SetFlags(flags) })
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	for _, why := range []string{"CSI 3b", "CSI 3b", "", "CSI ?7l", "CSI 3b", "CSI ?7l"} {
		d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: "1", State: "unknown", Unknown: true, Broke: why})
	}
	d.launcherScreenAnswered("L1", "wrong", screenAnswer{Look: "1", State: "unknown", Unknown: true, Broke: "forged"})
	want := "smolcorp: launcher L1's screen can't be read since \"CSI 3b\"\n" +
		"smolcorp: launcher L1's screen can't be read since \"CSI ?7l\"\n"
	if logged.String() != want {
		t.Errorf("logged %q", logged.String())
	}
	// Up to 32 reasons a launcher.
	logged.Reset()
	for i := range 40 {
		d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: "1", State: "unknown", Unknown: true, Broke: fmt.Sprintf("CSI %db", i)})
	}
	if n := strings.Count(logged.String(), "\n"); n != 30 {
		t.Errorf("logged %d more lines, want 30 (32 in all)", n)
	}
}
