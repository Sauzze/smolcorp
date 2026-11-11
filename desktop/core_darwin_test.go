package desktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeBridgeToken = "fake-bridge-token-fake-bridge-token"

// shortFolder is a private folder with a path short enough for a socket (104 bytes on a Mac).
func shortFolder(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeApp listens where the Mac app's bridge would, and answers each request
// with answer when its token is right.
func fakeApp(t *testing.T, folder string, answer func(bridgeRequest) string) string {
	t.Helper()
	path := filepath.Join(folder, "bridge.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	_ = os.Chmod(path, 0o600)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var request bridgeRequest
				if json.Unmarshal(line, &request) != nil || request.Token != fakeBridgeToken {
					_, _ = conn.Write([]byte(`{"ok":false,"error":"no"}` + "\n"))
					return
				}
				_, _ = conn.Write([]byte(answer(request) + "\n"))
			}()
		}
	}()
	return path
}

func TestTheSocketCarriesTheTokenAndTheAnswer(t *testing.T) {
	folder := shortFolder(t)
	var got bridgeRequest
	path := fakeApp(t, folder, func(r bridgeRequest) string {
		got = r
		return `{"ok":true,"tab":"iterm:AB12"}`
	})
	tab, err := hostOpenTerminal(&socketBridge{path: path, token: fakeBridgeToken}, "iterm", "/Users/me", filepath.Join(folder, "hires", "a.sh"))
	if err != nil || tab != "iterm:AB12" {
		t.Fatalf("open: %q %v", tab, err)
	}
	if got.Action != "open" || got.Terminal != "iterm" {
		t.Errorf("the app was asked %+v", got)
	}
	if _, err := ask(&socketBridge{path: path, token: "wrong-token"}, bridgeRequest{Action: "hello"}); err == nil || err.Error() != "no" {
		t.Errorf("a wrong token is refused, got %v", err)
	}
	if _, err := ask(&socketBridge{path: filepath.Join(folder, "none.sock"), token: fakeBridgeToken}, bridgeRequest{Action: "hello"}); err == nil {
		t.Error("no app, no answer")
	}
}

func TestAnAnswerTooLongIsRefused(t *testing.T) {
	folder := shortFolder(t)
	path := fakeApp(t, folder, func(bridgeRequest) string {
		return `{"ok":true,"value":"` + strings.Repeat("x", bridgeAnswerMost) + `"}`
	})
	if _, err := ask(&socketBridge{path: path, token: fakeBridgeToken}, bridgeRequest{Action: "secret-read", Item: "gm-token"}); err == nil {
		t.Error("an answer over the limit is an error")
	}
}

func TestAnAppThatNeverAnswersIsGivenUpOn(t *testing.T) {
	folder := shortFolder(t)
	path := filepath.Join(folder, "bridge.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			time.Sleep(time.Second)
			conn.Close()
		}
	}()
	start := time.Now()
	if _, err := ask(&socketBridge{path: path, token: fakeBridgeToken, wait: 100 * time.Millisecond}, bridgeRequest{Action: "hello"}); err == nil {
		t.Error("no answer is an error")
	}
	if time.Since(start) > 900*time.Millisecond {
		t.Error("the wait wasn't kept to")
	}
}

func TestTheEngineRefusesAnyoneButTheApp(t *testing.T) {
	folder := shortFolder(t)
	socket := fakeApp(t, folder, func(bridgeRequest) string { return `{"ok":true}` })
	good := []string{"--port", "7779", "--support", folder, "--bridge", socket}
	if o, err := parseCoreOptions(good, fakeBridgeToken); err != nil || o.port != 7779 || o.support != folder || o.bridge != socket {
		t.Fatalf("the app's own: %+v %v", o, err)
	}
	open := shortFolder(t)
	_ = os.Chmod(open, 0o755)
	plain := filepath.Join(folder, "plain")
	_ = os.WriteFile(plain, nil, 0o600)
	for name, c := range map[string]struct {
		args  []string
		token string
	}{
		"no token":           {good, ""},
		"a short token":      {good, "short"},
		"no port":            {[]string{"--support", folder, "--bridge", socket}, fakeBridgeToken},
		"a low port":         {[]string{"--port", "80", "--support", folder, "--bridge", socket}, fakeBridgeToken},
		"a relative folder":  {[]string{"--port", "7779", "--support", "support", "--bridge", socket}, fakeBridgeToken},
		"a bridge elsewhere": {[]string{"--port", "7779", "--support", open, "--bridge", socket}, fakeBridgeToken},
		"an open folder":     {[]string{"--port", "7779", "--support", open, "--bridge", filepath.Join(open, "bridge.sock")}, fakeBridgeToken},
		"not a socket":       {[]string{"--port", "7779", "--support", folder, "--bridge", plain}, fakeBridgeToken},
		"something more":     {append(append([]string{}, good...), "serve"), fakeBridgeToken},
	} {
		if _, err := parseCoreOptions(c.args, c.token); err == nil {
			t.Errorf("%s: started", name)
		}
	}
	_ = os.Chmod(socket, 0o666)
	if _, err := parseCoreOptions(good, fakeBridgeToken); err == nil {
		t.Error("a socket others can reach is refused")
	}
}

func TestRunCoreWithoutTheAppSaysWhatItIs(t *testing.T) {
	t.Setenv(bridgeTokenEnvironment, "")
	var errs bytes.Buffer
	if code := RunCore([]string{"--port", "7779"}, strings.NewReader(""), &bytes.Buffer{}, &errs); code != 2 {
		t.Errorf("code %d", code)
	}
	if !strings.Contains(errs.String(), "open Smolcorp.app instead") {
		t.Errorf("said %q", errs.String())
	}
	// The right shape, but an app that doesn't know the token: refused before anything is read.
	folder := shortFolder(t)
	socket := fakeApp(t, folder, func(bridgeRequest) string { return `{"ok":true}` })
	t.Setenv(bridgeTokenEnvironment, strings.Repeat("z", 40))
	errs.Reset()
	if code := RunCore([]string{"--port", "7779", "--support", folder, "--bridge", socket}, strings.NewReader(""), &bytes.Buffer{}, &errs); code != 2 {
		t.Errorf("code %d", code)
	}
	if os.Getenv(bridgeTokenEnvironment) != "" {
		t.Error("the token isn't handed on")
	}
	if supportDir == folder {
		t.Error("a refused engine never takes the folder")
	}
}

func TestTheAppsLines(t *testing.T) {
	d := newTestDeck(t)
	lines := make(chan []byte, 8)
	for _, line := range []string{
		`{"id":1,"op":"office"}`, `{"id":2,"op":"setup-session"}`, `{"id":3,"op":"setup-link","then":"pair"}`,
		`{"id":4,"op":"setup-link","then":"x&o=1"}`, `{"id":5,"op":"shell"}`, `not json`,
	} {
		lines <- []byte(line)
	}
	close(lines)
	var out bytes.Buffer
	if code := serveShell(d, lines, &out, make(chan error)); code != 0 {
		t.Fatalf("code %d", code)
	}
	var answers []shellAnswer
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var a shellAnswer
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			t.Fatal(err)
		}
		answers = append(answers, a)
	}
	if len(answers) != 7 || !answers[0].Ready || answers[0].Port != Port || answers[0].Cookie != setupCookie {
		t.Fatalf("answers %+v", answers)
	}
	if answers[1].ID != 1 || answers[1].Key == "" || !d.access.IsWindowKey(answers[1].Key) {
		t.Errorf("office: %+v", answers[1])
	}
	if answers[2].Cookie != setupCookie || !d.access.IsSetupSession(answers[2].Session) {
		t.Errorf("setup session: %+v", answers[2])
	}
	if !strings.Contains(answers[3].Link, "/setup#o=") || !strings.HasSuffix(answers[3].Link, "&pair") {
		t.Errorf("setup link: %+v", answers[3])
	}
	for _, a := range answers[4:] {
		if a.Error == "" || a.Link != "" {
			t.Errorf("refused: %+v", a)
		}
	}
}
