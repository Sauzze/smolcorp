package desktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// appBridge is the engine's bridge to a fake app in this test process: the
// process at the socket's other end, as its parent would be for the engine.
func appBridge(path, token string) *socketBridge {
	return &socketBridge{path: path, token: token, parent: os.Getpid}
}

// listenAndKeep listens at path and keeps every byte any connection sends it, as
// a process in the app's place would see them.
func listenAndKeep(t *testing.T, path string) (heard func() string) {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	var mu sync.Mutex
	var got bytes.Buffer
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				text, _ := io.ReadAll(conn)
				mu.Lock()
				got.Write(text)
				mu.Unlock()
			}()
		}
	}()
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return got.String()
	}
}

func TestTheTokenGoesOnlyToTheAppThatStartedTheEngine(t *testing.T) {
	folder := shortFolder(t)
	path := filepath.Join(folder, "bridge.sock")
	heard := listenAndKeep(t, path)
	for name, parent := range map[string]func() int{
		// The engine's real parent is `go test`; what listens is this test process.
		"not the parent":     os.Getppid,
		"an app gone":        func() int { return 1 },
		"no parent at all":   nil,
		"a parent of no pid": func() int { return 0 },
	} {
		bridge := &socketBridge{path: path, token: fakeBridgeToken, parent: parent}
		if _, err := ask(bridge, bridgeRequest{Action: "secret-read", Item: "gm-token"}); err != errNotTheApp {
			t.Errorf("%s: %v", name, err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if text := heard(); text != "" {
		t.Fatalf("something was written to a process that isn't the app: %q", text)
	}
	// The same socket, with this process as the engine's app: then, and only then, the token.
	_, _ = ask(appBridge(path, fakeBridgeToken), bridgeRequest{Action: "hello"})
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(heard(), fakeBridgeToken) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(heard(), fakeBridgeToken) {
		t.Errorf("the app was sent %q", heard())
	}
}

// TestImpostorForTheBridge stands in for another process of yours: run as
// one, it listens at the path it's given and writes down all it's sent.
func TestImpostorForTheBridge(t *testing.T) {
	socket, record := os.Getenv("SMOLCORP_TEST_IMPOSTOR"), os.Getenv("SMOLCORP_TEST_IMPOSTOR_RECORD")
	if socket == "" || record == "" {
		return
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		os.Exit(3)
	}
	fmt.Println("listening")
	for {
		conn, err := listener.Accept()
		if err != nil {
			os.Exit(0)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		text, _ := io.ReadAll(conn)
		file, _ := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		_, _ = file.Write(append(text, '|'))
		file.Close()
		conn.Close()
	}
}

// The attack the check is for: the app's socket moved aside, another process
// of yours listening at its name. The engine dials the name, finds that
// process there, and sends it nothing, while the app still answers at the new name.
func TestARenamedSocketsImpostorIsSentNothing(t *testing.T) {
	folder := shortFolder(t)
	path := filepath.Join(folder, "bridge.sock")
	fakeApp(t, folder, func(bridgeRequest) string { return `{"ok":true}` })
	kept := filepath.Join(folder, "kept.sock")
	if err := os.Rename(path, kept); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(folder, "heard")
	impostor := exec.Command(os.Args[0], "-test.run=^TestImpostorForTheBridge$")
	impostor.Env = append(os.Environ(), "SMOLCORP_TEST_IMPOSTOR="+path, "SMOLCORP_TEST_IMPOSTOR_RECORD="+record)
	out, err := impostor.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := impostor.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = impostor.Process.Kill(); _ = impostor.Wait() })
	if line, _ := bufio.NewReader(out).ReadString('\n'); line != "listening\n" {
		t.Fatalf("the impostor said %q", line)
	}
	// The engine, its app being this test process.
	bridge := appBridge(path, fakeBridgeToken)
	if _, err := ask(bridge, bridgeRequest{Action: "secret-read", Item: "gm-token"}); err != errNotTheApp {
		t.Errorf("through the impostor: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if text, _ := os.ReadFile(record); strings.Contains(string(text), fakeBridgeToken) || strings.Trim(string(text), "|") != "" {
		t.Fatalf("the impostor heard %q", text)
	}
	// The app itself, at the name it was moved to, is still the app.
	if _, err := ask(appBridge(kept, fakeBridgeToken), bridgeRequest{Action: "hello"}); err != nil {
		t.Errorf("the app: %v", err)
	}
}

func TestTheSocketCarriesTheTokenAndTheAnswer(t *testing.T) {
	folder := shortFolder(t)
	var got bridgeRequest
	path := fakeApp(t, folder, func(r bridgeRequest) string {
		got = r
		return `{"ok":true,"tab":"iterm:AB12"}`
	})
	tab, _, err := hostOpenTerminal(appBridge(path, fakeBridgeToken), "iterm", "/Users/me", filepath.Join(folder, "hires", "a.sh"))
	if err != nil || tab != "iterm:AB12" {
		t.Fatalf("open: %q %v", tab, err)
	}
	if got.Action != "open" || got.Terminal != "iterm" {
		t.Errorf("the app was asked %+v", got)
	}
	if _, err := ask(appBridge(path, "wrong-token"), bridgeRequest{Action: "hello"}); err == nil || err.Error() != "no" {
		t.Errorf("a wrong token is refused, got %v", err)
	}
	if _, err := ask(appBridge(filepath.Join(folder, "none.sock"), fakeBridgeToken), bridgeRequest{Action: "hello"}); err == nil {
		t.Error("no app, no answer")
	}
}

func TestAnAnswerTooLongIsRefused(t *testing.T) {
	folder := shortFolder(t)
	path := fakeApp(t, folder, func(bridgeRequest) string {
		return `{"ok":true,"value":"` + strings.Repeat("x", bridgeAnswerMost) + `"}`
	})
	if _, err := ask(appBridge(path, fakeBridgeToken), bridgeRequest{Action: "secret-read", Item: "gm-token"}); err == nil {
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
	if _, err := ask(&socketBridge{path: path, token: fakeBridgeToken, parent: os.Getpid, wait: 100 * time.Millisecond}, bridgeRequest{Action: "hello"}); err == nil {
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
	var errs bytes.Buffer
	if code := RunCore([]string{"--port", "7779"}, strings.NewReader(""), &bytes.Buffer{}, &errs); code != 2 {
		t.Errorf("code %d", code)
	}
	if !strings.Contains(errs.String(), "open Smolcorp.app instead") || !strings.Contains(errs.String(), "no bridge token") {
		t.Errorf("said %q", errs.String())
	}
	// The right shape, but a socket whose other end isn't the engine's parent
	// (this test process, not `go test`): refused, and nothing sent to it.
	folder := shortFolder(t)
	socket := filepath.Join(folder, "bridge.sock")
	heard := listenAndKeep(t, socket)
	_ = os.Chmod(socket, 0o600)
	token := strings.Repeat("z", 40)
	errs.Reset()
	if code := RunCore([]string{"--port", "7779", "--support", folder, "--bridge", socket}, strings.NewReader(token+"\n"), &bytes.Buffer{}, &errs); code != 2 {
		t.Errorf("code %d", code)
	}
	if !strings.Contains(errs.String(), "the app's bridge: "+errNotTheApp.Error()) {
		t.Errorf("said %q", errs.String())
	}
	time.Sleep(100 * time.Millisecond)
	if text := heard(); text != "" {
		t.Errorf("sent to a process that isn't its app: %q", text)
	}
	for _, variable := range os.Environ() {
		if strings.Contains(variable, token) {
			t.Errorf("the token is in the environment: %s", strings.SplitN(variable, "=", 2)[0])
		}
	}
	if supportDir == folder {
		t.Error("a refused engine never takes the folder")
	}
}

func TestTheTokenIsTheFirstLineOnTheInput(t *testing.T) {
	input := bufio.NewReader(strings.NewReader(fakeBridgeToken + "\n" + `{"id":1,"op":"office"}` + "\n"))
	token, err := readBridgeToken(input, time.Second)
	if err != nil || token != fakeBridgeToken {
		t.Fatalf("token %q, %v", token, err)
	}
	if rest, _ := input.ReadString('\n'); rest != `{"id":1,"op":"office"}`+"\n" {
		t.Errorf("the app's lines after it are left to read: %q", rest)
	}
	for name, text := range map[string]string{"nothing": "", "no line end": fakeBridgeToken, "too long": strings.Repeat("z", bridgeTokenMost+1) + "\n"} {
		if _, err := readBridgeToken(bufio.NewReader(strings.NewReader(text)), time.Second); err == nil {
			t.Errorf("%s: a token", name)
		}
	}
	// Started by hand, with nothing on its input: it doesn't wait for ever.
	silent, _ := io.Pipe()
	start := time.Now()
	if _, err := readBridgeToken(bufio.NewReader(silent), 100*time.Millisecond); err == nil || time.Since(start) > 900*time.Millisecond {
		t.Errorf("a silent input: %v after %v", err, time.Since(start))
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

// The phone's text: the app asks the desk's state, on the same connection, where the Swift engine looks at its
// own sessions, and is answered at once with it ("" for a desk gone). No question to answer is asking. Up to
// bridgeQuestionsMost questions on one request (a paste asks before and right before ⌘V); more is an error.
func TestTheAppAsksTheDesksStateOnItsConnection(t *testing.T) {
	folder := shortFolder(t)
	path := filepath.Join(folder, "bridge.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	_ = os.Chmod(path, 0o600)
	heard := make(chan []string, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				line, err := reader.ReadBytes('\n')
				var request bridgeRequest
				if err != nil || json.Unmarshal(line, &request) != nil || request.Token != fakeBridgeToken {
					return
				}
				// As many questions as the text says, as the app asks before a paste and right before ⌘V.
				asks := strings.Count(request.Text, "?")
				var replies []string
				for range asks {
					_, _ = conn.Write([]byte(`{"ready":true}` + "\n"))
					said, err := reader.ReadBytes('\n')
					if err != nil {
						heard <- replies
						return
					}
					replies = append(replies, strings.TrimSpace(string(said)))
				}
				heard <- replies
				_, _ = conn.Write([]byte(`{"ok":true,"pasted":"pasted"}` + "\n"))
			}()
		}
	}()
	bridge := appBridge(path, fakeBridgeToken)
	for _, c := range []struct {
		text  string
		state func() SessionState
		said  []string
	}{
		{"one?", func() SessionState { return Unread }, []string{`{"state":"unread"}`}},
		{"two??", func() SessionState { return Thinking }, []string{`{"state":"thinking"}`, `{"state":"thinking"}`}},
		{"gone?", func() SessionState { return "" }, []string{`{"state":""}`}},
		{"none?", nil, []string{`{"state":"asking"}`}},
	} {
		result, err := hostPhoneText(bridge, "tty:/dev/ttys003", Claude, c.text, c.state)
		if err != nil || result != pasteDone {
			t.Errorf("%s: %v %v", c.text, result, err)
		}
		if said := <-heard; !slices.Equal(said, c.said) {
			t.Errorf("%s: answered %q, want %q", c.text, said, c.said)
		}
	}
	// One question too many: an error that may have come after the words went, never the app's no.
	result, err := hostPhoneText(bridge, "tty:/dev/ttys003", Claude, "four????", func() SessionState { return Idle })
	if !errors.Is(err, errPasteFailed) || result != pasteFailed {
		t.Errorf("asked four times: %v %v", result, err)
	}
	<-heard
}
