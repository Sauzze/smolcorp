package gm

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// `smolcorp gm login` against a stand-in for the app: a fake token, and a stand-in for the GM's start, so nothing is
// ever started.

const fakeLoginToken = "fakeLoginToken-0123456789-abcdefghijklmnopqr"

// started is what login would have started: the program, its words and its environment; and what it checked.
type started struct {
	program     string
	args, env   []string
	calledTimes int
	checked     [][2]string // each check's word and program
	refuse      func(n int) error
}

// String is what a failing row prints: never the environment, which is this test's own, its secrets included.
func (s *started) String() string {
	return fmt.Sprintf("{program:%s args:%q started:%d checked:%q}", s.program, s.args, s.calledTimes, s.checked)
}

// stubStart puts stand-ins for the GM's start and for the program's check (desktop's, which cmd/smolcorp puts in
// CheckProgram): the check passes unless s.refuse says otherwise, for the n-th check (from 1).
func stubStart(t *testing.T) *started {
	t.Helper()
	s := &started{}
	saved, savedCheck := startGM, CheckProgram
	startGM = func(program string, args, env []string) error {
		s.program, s.args, s.env = program, args, env
		s.calledTimes++
		return nil
	}
	CheckProgram = func(word, program string) error {
		s.checked = append(s.checked, [2]string{word, program})
		if s.refuse != nil {
			return s.refuse(len(s.checked))
		}
		return nil
	}
	t.Cleanup(func() { startGM, CheckProgram = saved, savedCheck })
	return s
}

// The owner's Allow: the GM starts with its token in its own environment, and the token is never printed.
func TestLoginStartsTheGMWithItsTokenOnceAllowed(t *testing.T) {
	start := stubStart(t)
	app := newFakeApp(t, allowing(fakeLoginToken, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	self, _ := os.Executable()
	code, out, errOut := runGM(app.env(t, nil), "login", "--", self, "--model", "opus")
	if code != 0 {
		t.Fatalf("exit %d %q", code, errOut)
	}
	if !strings.Contains(out, "Check that it shows this code: "+fakeAskCode) || !strings.Contains(out, "Smolcorp's Settings") {
		t.Errorf("the code and where to look aren't said: %q", out)
	}
	if strings.Contains(out+errOut, fakeLoginToken) {
		t.Errorf("the token was printed: %q %q", out, errOut)
	}
	if start.calledTimes != 1 || start.program != self || !slices.Equal(start.args, []string{self, "--model", "opus"}) {
		t.Fatalf("started %+v", start)
	}
	var given []string
	for _, entry := range start.env {
		if strings.HasPrefix(entry, tokenEnv+"=") {
			given = append(given, entry)
		}
	}
	if !slices.Equal(given, []string{tokenEnv + "=" + fakeLoginToken}) {
		t.Errorf("the GM's environment has %v", given)
	}
	// The ask: a GM's, its folder, with the header no web page can send, and no key of any kind.
	asks := app.callsTo(http.MethodPost, terminalAskPath)
	if len(asks) != 1 {
		t.Fatalf("%d asks", len(asks))
	}
	query, _ := url.ParseQuery(asks[0].RawQuery)
	here, _ := os.Getwd()
	if query.Get("kind") != AskGM || query.Get("folder") != here || asks[0].Header.Get(setupHeader) != "1" {
		t.Errorf("the ask: %v %v", query, asks[0].Header)
	}
	for _, r := range app.reqs {
		if r.Header.Get(tokenHeader) != "" || r.Header.Get(setupKeyHeader) != "" || r.Header.Get(hireHeader) != "" {
			t.Errorf("%s %s carried a key", r.Method, r.Path)
		}
	}
}

// A no in the window, the app refusing the ask (the switch off), a hire, and a GM's own environment: nothing starts.
func TestLoginStartsNothingWithoutTheOwnersAllow(t *testing.T) {
	start := stubStart(t)
	self, _ := os.Executable()
	denied := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"ticket":"fake-ticket","code":"` + fakeAskCode + `","where":"Smolcorp's Settings"}`))
			return
		}
		w.Write([]byte(`{"state":"denied"}`))
	})
	if code, _, errOut := runGM(denied.env(t, nil), "login", "--", self); code != exitApp || !strings.Contains(errOut, "you said no") {
		t.Errorf("denied: %d %q", code, errOut)
	}
	off := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Turn on Allow a General Manager started by hand in Smolcorp's Settings, Hires and roles, then run smolcorp gm login again."))
	})
	if code, _, errOut := runGM(off.env(t, nil), "login", "--", self); code != exitApp || !strings.Contains(errOut, "Turn on Allow a General Manager started by hand") {
		t.Errorf("switch off: %d %q", code, errOut)
	}
	gone := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"ticket":"fake-ticket","code":"` + fakeAskCode + `"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if code, _, errOut := runGM(gone.env(t, nil), "login", "--", self); code != exitApp || !strings.Contains(errOut, "ran out") {
		t.Errorf("run out: %d %q", code, errOut)
	}
	none := newFakeApp(t, allowing(fakeLoginToken, nil))
	for name, vars := range map[string]map[string]string{"a hire": {hireKeyEnv: "fake-hire-key-89"}, "a GM": withToken()} {
		if code, _, errOut := runGM(none.env(t, vars), "login", "--", self); code == exitOK || errOut == "" {
			t.Errorf("%s: %d %q", name, code, errOut)
		}
	}
	if len(none.reqs) != 0 {
		t.Errorf("a hire's or a GM's login asked the app: %d requests", len(none.reqs))
	}
	if code, _, errOut := runGM(none.env(t, nil), "login", "--model"); code != exitUsage || !strings.Contains(errOut, "after --") {
		t.Errorf("an option: %d %q", code, errOut)
	}
	if code, _, errOut := runGM(none.env(t, nil), "login", "--", "no-such-program-smolcorp-test"); code != exitUsage || !strings.Contains(errOut, "can't find") {
		t.Errorf("no program: %d %q", code, errOut)
	}
	if start.calledTimes != 0 {
		t.Errorf("something started: %+v", start)
	}
}

// RMSY126k nit 6: the program login would start as the GM goes through the check every agent Smolcorp starts goes
// through (SG2), before the ask and again just before the start. A refused one never starts, and you're told why in
// plain words. The check here is a stand-in that says no: nothing real is checked or started.
func TestLoginStartsNoProgramTheCheckRefuses(t *testing.T) {
	notClaude := errors.New("Smolcorp didn't start Claude Code: the program it found isn't signed by Anthropic.")
	self, _ := os.Executable()

	// Refused at once: nothing is asked, nothing starts.
	start := stubStart(t)
	start.refuse = func(int) error { return notClaude }
	none := newFakeApp(t, allowing(fakeLoginToken, nil))
	code, _, errOut := runGM(none.env(t, nil), "login", "--", self)
	if code != exitApp || !strings.Contains(errOut, notClaude.Error()+" Your General Manager wasn't started.") {
		t.Errorf("refused: %d %q", code, errOut)
	}
	if len(none.reqs) != 0 || start.calledTimes != 0 {
		t.Errorf("a refused program was asked for or started: %d requests, %+v", len(none.reqs), start)
	}

	// Changed while you answered: the second check, after your Allow, refuses it, and nothing starts.
	start = stubStart(t)
	start.refuse = func(n int) error {
		if n == 2 {
			return notClaude
		}
		return nil
	}
	allowed := newFakeApp(t, allowing(fakeLoginToken, nil))
	code, out, errOut := runGM(allowed.env(t, nil), "login", "--", self)
	if code != exitApp || !strings.Contains(errOut, "wasn't started") || start.calledTimes != 0 || len(start.checked) != 2 {
		t.Errorf("changed while asking: %d %q, %+v", code, errOut, start)
	}
	if strings.Contains(out+errOut, fakeLoginToken) {
		t.Errorf("the token was printed: %q %q", out, errOut)
	}

	// The check is given the word as typed and the program found for it: claude, found on PATH, by default.
	start = stubStart(t)
	bin := t.TempDir()
	standIn := filepath.Join(bin, "claude")
	if err := os.WriteFile(standIn, []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("PATH's claude is claude.exe or claude.cmd on Windows")
	}
	t.Setenv("PATH", bin)
	if code, _, errOut := runGM(allowed.env(t, nil), "login"); code != exitOK {
		t.Fatalf("the default: %d %q", code, errOut)
	}
	if want := [2]string{"claude", standIn}; len(start.checked) != 2 || start.checked[0] != want || start.checked[1] != want {
		t.Errorf("checked %v, want %v twice", start.checked, want)
	}
	if start.calledTimes != 1 || start.program != standIn {
		t.Errorf("started %+v", start)
	}

	// Without a check (no cmd/smolcorp to give one), nothing is asked or started.
	start = stubStart(t)
	CheckProgram = nil
	unchecked := newFakeApp(t, allowing(fakeLoginToken, nil))
	if code, _, errOut := runGM(unchecked.env(t, nil), "login", "--", self); code != exitApp || !strings.Contains(errOut, "can't check") {
		t.Errorf("no check: %d %q", code, errOut)
	}
	if len(unchecked.reqs) != 0 || start.calledTimes != 0 {
		t.Errorf("unchecked: %d requests, %+v", len(unchecked.reqs), start)
	}
}
