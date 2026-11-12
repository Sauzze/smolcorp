package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The stand-in agent is this test binary run again with standInEnv set: a
// small ACP agent in Go, so no real agent ever runs.
const standInEnv = "SMOLCORP_ACP_STANDIN"

// childPidEnv names the file where the "child" stand-in writes its child's pid.
const childPidEnv = "SMOLCORP_ACP_CHILDPID"

// TestMain runs the stand-in when asked to; otherwise it points the home and
// config folders at a fresh temporary folder, as the other packages do.
func TestMain(m *testing.M) {
	if mode := os.Getenv(standInEnv); mode != "" {
		standIn(mode)
		os.Exit(0)
	}
	root, err := os.MkdirTemp("", "smolcorp-acp-test-")
	if err != nil {
		println("can't make a temp folder:", err.Error())
		os.Exit(1)
	}
	os.Setenv("HOME", root)
	os.Setenv("USERPROFILE", root)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	os.Setenv("APPDATA", filepath.Join(root, "AppData"))
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

// standIn answers by the prompt's text: hello, tool (asks permission), crash,
// big (a 1 MiB line), ask-unknown, read (asks for a file), wait (until
// cancelled). Mode "old" answers initialize with version 2; "stubborn"
// doesn't exit when its stdin closes; "stringid" answers with string ids;
// "child" starts a `sleep 30` child first and writes its pid to the file named
// by childPidEnv (then it exits on EOF, or on "crash" like the others).
func standIn(mode string) {
	var mu sync.Mutex
	out := bufio.NewWriter(os.Stdout)
	send := func(v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		out.Write(append(b, '\n'))
		out.Flush()
		mu.Unlock()
	}
	reply := func(id json.RawMessage, result any) {
		if mode == "stringid" {
			id = json.RawMessage(strconv.Quote(string(id)))
		}
		send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	update := func(u map[string]any) {
		send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "s1", "update": u}})
	}
	chunk := func(s string) {
		update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": s}})
	}
	answers := map[string]chan inMsg{}
	nextID := 100
	ask := func(method string, params any) inMsg {
		mu.Lock()
		nextID++
		id := fmt.Sprint(nextID)
		ch := make(chan inMsg, 1)
		answers[id] = ch
		mu.Unlock()
		send(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "method": method, "params": params})
		return <-ch
	}
	cancelled := make(chan struct{})
	var cancelOnce sync.Once

	prompt := func(id json.RawMessage, text string) {
		switch text {
		case "hello":
			chunk("hi there")
		case "tool":
			update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "rm -rf build", "kind": "execute", "status": "pending"})
			a := ask("session/request_permission", map[string]any{"sessionId": "s1",
				"toolCall": map[string]any{"toolCallId": "t1", "title": "rm -rf build", "kind": "execute"},
				"options":  []map[string]any{{"optionId": "allow", "name": "Allow", "kind": "allow_once"}, {"optionId": "reject", "name": "Reject", "kind": "reject_once"}}})
			var r permissionResponse
			json.Unmarshal(a.Result, &r)
			if r.Outcome.Outcome == "selected" && r.Outcome.OptionID == "allow" {
				chunk("ran")
			} else {
				update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": "failed"})
				chunk("not run: " + r.Outcome.Outcome + "/" + r.Outcome.OptionID)
			}
			if r.Outcome.Outcome == "cancelled" {
				reply(id, map[string]any{"stopReason": "cancelled"})
				return
			}
		case "crash":
			chunk("working")
			os.Exit(3)
		case "big":
			chunk(strings.Repeat("x", 1<<20))
		case "ask-unknown":
			a, b := ask("x/frobnicate", map[string]any{}), ask("fs/read_text_file", map[string]any{"sessionId": "s1", "path": "/etc/hosts"})
			chunk(fmt.Sprint(a.Error.Code, " ", b.Error.Code))
		case "read":
			a := ask("fs/read_text_file", map[string]any{"sessionId": "s1", "path": "/work/notes.txt"})
			var r ReadTextFileResponse
			json.Unmarshal(a.Result, &r)
			chunk("read: " + r.Content)
		case "wait":
			<-cancelled
			reply(id, map[string]any{"stopReason": "cancelled"})
			return
		}
		reply(id, map[string]any{"stopReason": "end_turn"})
	}

	if mode == "child" {
		child := exec.Command("sleep", "30")
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		os.WriteFile(os.Getenv(childPidEnv), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(nil, 64<<20)
	for in.Scan() {
		var m inMsg
		if json.Unmarshal(in.Bytes(), &m) != nil {
			continue
		}
		switch m.Method {
		case "":
			mu.Lock()
			ch := answers[string(m.ID)]
			mu.Unlock()
			if ch != nil {
				ch <- m
			}
		case "session/cancel":
			cancelOnce.Do(func() { close(cancelled) })
		case "initialize":
			var p initializeRequest
			json.Unmarshal(m.Params, &p)
			version := ProtocolVersion
			if mode == "old" {
				version = 2
			}
			caps := fmt.Sprintf("fs=%v terminal=%v", p.ClientCapabilities.FS.ReadTextFile, p.ClientCapabilities.Terminal)
			reply(m.ID, map[string]any{"protocolVersion": version, "agentCapabilities": map[string]any{"loadSession": true},
				"agentInfo": map[string]any{"name": "stand-in", "version": caps}, "somethingNew": 1})
		case "session/new":
			reply(m.ID, map[string]any{"sessionId": "s1"})
		case "session/load":
			update(map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]any{"type": "text", "text": "earlier"}})
			chunk("replayed")
			reply(m.ID, nil)
		case "session/prompt":
			var p promptRequest
			json.Unmarshal(m.Params, &p)
			go prompt(m.ID, p.Prompt[0].Text)
		default:
			send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": CodeMethodNotFound, "message": "method not found"}})
		}
	}
	if mode == "stubborn" {
		time.Sleep(time.Hour)
	}
}

// recorder is the caller: it keeps the updates and permission asks, and
// answers asks with pick (nil: refuse by error).
type recorder struct {
	mu      sync.Mutex
	updates []SessionNotification
	asked   []PermissionRequest
	pick    func(context.Context, PermissionRequest) (PermissionOutcome, error)
}

func (r *recorder) Update(n SessionNotification) {
	r.mu.Lock()
	r.updates = append(r.updates, n)
	r.mu.Unlock()
}

func (r *recorder) RequestPermission(ctx context.Context, req PermissionRequest) (PermissionOutcome, error) {
	r.mu.Lock()
	r.asked = append(r.asked, req)
	pick := r.pick
	r.mu.Unlock()
	if pick == nil {
		return PermissionOutcome{}, errors.New("no")
	}
	return pick(ctx, req)
}

// said is the agent's message chunks so far, joined by "|".
func (r *recorder) said() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var s []string
	for _, n := range r.updates {
		if n.Update.Content != nil && strings.HasSuffix(n.Update.Kind, "_chunk") {
			s = append(s, n.Update.Content.Text)
		}
	}
	return strings.Join(s, "|")
}

type fileRecorder struct{ recorder }

func (f *fileRecorder) ReadTextFile(_ context.Context, req ReadTextFileRequest) (ReadTextFileResponse, error) {
	return ReadTextFileResponse{Content: "contents of " + req.Path}, nil
}

func (f *fileRecorder) WriteTextFile(context.Context, WriteTextFileRequest) error {
	return &RPCError{Code: -32000, Message: "refused"}
}

func startStandIn(t *testing.T, mode string, h Handler, opt Options) (*Client, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if opt.CallTimeout == 0 {
		opt.CallTimeout = 10 * time.Second
	}
	c, err := Start(standInCommand(exe, mode), h, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	sid, err := c.NewSession(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c, sid
}

func TestHappyPath(t *testing.T) {
	r := &recorder{}
	exe, _ := os.Executable()
	c, err := Start(standInCommand(exe, "plain"), r, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := c.NewSession(ctx, t.TempDir()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("a session before initialize: %v", err)
	}
	got, err := c.Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProtocolVersion != 1 || !got.AgentCapabilities.LoadSession || got.AgentInfo == nil || got.AgentInfo.Name != "stand-in" {
		t.Fatalf("initialize: %+v", got)
	}
	if got.AgentInfo.Version != "fs=false terminal=false" {
		t.Fatalf("a client without file or terminal handlers offered them: %s", got.AgentInfo.Version)
	}
	if _, err := c.NewSession(ctx, "relative/folder"); err == nil {
		t.Fatal("a relative folder was accepted")
	}
	sid, err := c.NewSession(ctx, t.TempDir())
	if err != nil || sid != "s1" {
		t.Fatalf("session/new: %q %v", sid, err)
	}
	stop, err := c.Prompt(ctx, sid, []ContentBlock{Text("hello")})
	if err != nil || stop != "end_turn" {
		t.Fatalf("prompt: %q %v", stop, err)
	}
	if got := r.said(); got != "hi there" { // delivered before Prompt returned
		t.Fatalf("updates: %q", got)
	}
	if err := c.LoadSession(ctx, sid, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := r.said(); got != "hi there|earlier|replayed" {
		t.Fatalf("replay: %q", got)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !errors.Is(c.Err(), ErrClosed) {
		t.Fatalf("after close: %v", c.Err())
	}
	if _, err := c.Prompt(ctx, sid, []ContentBlock{Text("hello")}); !errors.Is(err, ErrClosed) {
		t.Fatalf("prompt after close: %v", err)
	}
}

func TestCrashMidSession(t *testing.T) {
	r := &recorder{}
	c, sid := startStandIn(t, "plain", r, Options{})
	_, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("crash")})
	if !errors.Is(err, ErrAgentExited) || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("crash: %v", err)
	}
	if r.said() != "working" {
		t.Fatalf("the update sent before the crash was lost: %q", r.said())
	}
	select {
	case <-c.Done():
	default:
		t.Fatal("Done isn't closed after a crash")
	}
	start := time.Now()
	if _, err := c.NewSession(context.Background(), t.TempDir()); !errors.Is(err, ErrAgentExited) || time.Since(start) > time.Second {
		t.Fatalf("a call after the crash: %v after %s", err, time.Since(start))
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close after a crash: %v", err)
	}
}

func TestOversizeMessage(t *testing.T) {
	r := &recorder{}
	c, sid := startStandIn(t, "plain", r, Options{MaxMessage: 64 << 10})
	ctx := context.Background()
	if _, err := c.Prompt(ctx, sid, []ContentBlock{Text(strings.Repeat("y", 100<<10))}); !errors.Is(err, ErrTooBig) {
		t.Fatalf("an oversize prompt: %v", err)
	}
	if _, err := c.Prompt(ctx, sid, []ContentBlock{Text("hello")}); err != nil {
		t.Fatalf("an oversize prompt wasn't sent, so the session goes on: %v", err)
	}
	_, err := c.Prompt(ctx, sid, []ContentBlock{Text("big")})
	if !errors.Is(err, ErrTooBig) {
		t.Fatalf("an oversize message from the agent: %v", err)
	}
	if strings.Contains(r.said(), "xxx") {
		t.Fatal("part of the oversize message was delivered")
	}
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the connection didn't end")
	}
}

func TestUnknownMethod(t *testing.T) {
	r := &recorder{}
	c, sid := startStandIn(t, "plain", r, Options{})
	ctx := context.Background()
	var re *RPCError
	if err := c.call(ctx, "x/unknown", map[string]any{}, nil, time.Second); !errors.As(err, &re) || re.Code != CodeMethodNotFound {
		t.Fatalf("an unknown method at the agent: %v", err)
	}
	// The agent asks the client for a method it doesn't know, and for files
	// with no file handler: both "method not found".
	if _, err := c.Prompt(ctx, sid, []ContentBlock{Text("ask-unknown")}); err != nil {
		t.Fatal(err)
	}
	if got := r.said(); got != "-32601 -32601" {
		t.Fatalf("codes: %q", got)
	}
}

func TestPermissionGoesToTheCaller(t *testing.T) {
	refuse := func(context.Context, PermissionRequest) (PermissionOutcome, error) {
		return PermissionOutcome{OptionID: "reject"}, nil
	}
	notOffered := func(context.Context, PermissionRequest) (PermissionOutcome, error) {
		return PermissionOutcome{OptionID: "allow_always"}, nil
	}
	cases := []struct {
		name string
		pick func(context.Context, PermissionRequest) (PermissionOutcome, error)
		said string
		stop StopReason
	}{
		{"caller refuses", refuse, "not run: selected/reject", "end_turn"},
		{"caller fails", nil, "not run: cancelled/", StopCancelled},
		{"caller picks an option not offered", notOffered, "not run: cancelled/", StopCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{pick: tc.pick}
			c, sid := startStandIn(t, "plain", r, Options{})
			stop, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("tool")})
			if err != nil || stop != tc.stop {
				t.Fatalf("prompt: %q %v", stop, err)
			}
			if got := r.said(); got != tc.said {
				t.Fatalf("agent said %q, want %q", got, tc.said)
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.asked) != 1 || r.asked[0].ToolCall.Title != "rm -rf build" || len(r.asked[0].Options) != 2 {
				t.Fatalf("the caller wasn't asked with the request: %+v", r.asked)
			}
		})
	}

	t.Run("cancel while asking", func(t *testing.T) {
		asking := make(chan struct{})
		r := &recorder{pick: func(ctx context.Context, _ PermissionRequest) (PermissionOutcome, error) {
			close(asking)
			<-ctx.Done() // the owner never taps; the cancel ends the ask
			return PermissionOutcome{OptionID: "allow"}, nil
		}}
		c, sid := startStandIn(t, "plain", r, Options{})
		go func() {
			<-asking
			c.Cancel(context.Background(), sid)
		}()
		stop, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("tool")})
		if err != nil || stop != StopCancelled || r.said() != "not run: cancelled/" {
			t.Fatalf("cancelled ask: %q %v %q", stop, err, r.said())
		}
	})
}

func TestFilesGoToTheCaller(t *testing.T) {
	f := &fileRecorder{}
	exe, _ := os.Executable()
	c, err := Start(standInCommand(exe, "plain"), f, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got, err := c.Initialize(context.Background())
	if err != nil || got.AgentInfo.Version != "fs=true terminal=false" {
		t.Fatalf("capabilities: %+v %v", got, err)
	}
	sid, _ := c.NewSession(context.Background(), t.TempDir())
	if _, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("read")}); err != nil {
		t.Fatal(err)
	}
	if got := f.said(); got != "read: contents of /work/notes.txt" {
		t.Fatalf("read: %q", got)
	}
}

func TestTurnTimeoutCancels(t *testing.T) {
	r := &recorder{}
	c, sid := startStandIn(t, "plain", r, Options{TurnTimeout: 300 * time.Millisecond})
	if _, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("wait")}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("a turn past its timeout: %v", err)
	}
	// The stand-in only answers "wait" once cancelled; the client goes on.
	if stop, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("hello")}); err != nil || stop != "end_turn" {
		t.Fatalf("after a timeout: %q %v", stop, err)
	}
}

func TestVersionRefused(t *testing.T) {
	exe, _ := os.Executable()
	c, err := Start(standInCommand(exe, "old"), &recorder{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var ve *VersionError
	if _, err := c.Initialize(context.Background()); !errors.As(err, &ve) || ve.Agent != 2 {
		t.Fatalf("another version: %v", err)
	}
}

func TestCloseKillsAStubbornAgent(t *testing.T) {
	c, _ := startStandIn(t, "stubborn", &recorder{}, Options{ShutdownGrace: 200 * time.Millisecond})
	start := time.Now()
	err := c.Close()
	if err == nil || !strings.Contains(err.Error(), "killed") || time.Since(start) > 3*time.Second {
		t.Fatalf("close: %v after %s", err, time.Since(start))
	}
	if c.cmd.ProcessState == nil {
		t.Fatal("the agent wasn't reaped")
	}
}

func TestNoAgentPath(t *testing.T) {
	if _, err := Start(Command{}, &recorder{}, Options{}); err == nil {
		t.Fatal("started nothing")
	}
}

func TestStartNeedsTheHiresEnvironment(t *testing.T) {
	exe, _ := os.Executable()
	cmd := standInCommand(exe, "plain")
	cmd.Env = nil
	c, err := Start(cmd, &recorder{}, Options{})
	if c != nil || !errors.Is(err, errNoEnv) || !strings.Contains(err.Error(), "hireEnvironment()") {
		t.Fatalf("a nil Env: %v %v", c, err)
	}
	cmd.Env = []string{standInEnv + "=plain", "GORACE=atexit_sleep_ms=0"} // any list is the caller's choice
	c, err = Start(cmd, &recorder{}, Options{})
	if err != nil {
		t.Fatalf("a given Env: %v", err)
	}
	c.Close()
}

func TestStartNeedsAnAbsoluteFolder(t *testing.T) {
	exe, _ := os.Executable()
	for _, dir := range []string{"", "relative/folder", "."} {
		cmd := standInCommand(exe, "plain")
		cmd.Dir = dir
		if c, err := Start(cmd, &recorder{}, Options{}); c != nil || !errors.Is(err, errBadDir) {
			t.Fatalf("Dir %q: %v %v", dir, c, err)
		}
	}
}

// isAlive says whether the process is there now.
func isAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

// stillAlive polls for a process to be gone (init reaps it a moment after the
// kill) and says whether it still is after a couple of seconds.
func stillAlive(pid int) bool {
	for i := 0; i < 100; i++ {
		if !isAlive(pid) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// startWithChild runs the "child" stand-in and returns the pid of the
// `sleep 30` it started, killed at the end of the test whatever happens.
func startWithChild(t *testing.T) (*Client, string, int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("what the agent started is only reached on Mac and Linux for now")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	exe, _ := os.Executable()
	cmd := standInCommand(exe, "child")
	cmd.Env = append(cmd.Env, childPidEnv+"="+pidFile)
	c, err := Start(cmd, &recorder{}, Options{CallTimeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	sid, err := c.NewSession(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p, err := os.FindProcess(pid); err == nil {
			p.Kill()
		}
	})
	if !isAlive(pid) {
		t.Fatal("the stand-in's child wasn't running")
	}
	return c, sid, pid
}

func TestCloseStopsWhatTheAgentStarted(t *testing.T) {
	c, _, pid := startWithChild(t)
	if err := c.Close(); err != nil { // the agent leaves on its own when stdin closes
		t.Fatalf("close: %v", err)
	}
	if stillAlive(pid) {
		t.Fatal("the agent's child outlived Close")
	}
}

func TestCrashStopsWhatTheAgentStarted(t *testing.T) {
	c, sid, pid := startWithChild(t)
	if _, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("crash")}); !errors.Is(err, ErrAgentExited) {
		t.Fatalf("crash: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close after a crash: %v", err)
	}
	if stillAlive(pid) {
		t.Fatal("the agent's child outlived the crash and Close")
	}
}

func TestStringIDsAreAnswers(t *testing.T) {
	c, sid := startStandIn(t, "stringid", &recorder{}, Options{CallTimeout: 3 * time.Second})
	if stop, err := c.Prompt(context.Background(), sid, []ContentBlock{Text("hello")}); err != nil || stop != "end_turn" {
		t.Fatalf("an answer with a string id: %q %v", stop, err)
	}
}

// The reading goroutine never waits on its own replies: with the queue to the
// agent full, a garbage line and a request over the limit return at once.
func TestReaderRepliesNeverBlock(t *testing.T) {
	c := &Client{
		opt:   Options{CallTimeout: 5 * time.Second}.withDefaults(),
		out:   make(chan []byte, 1),
		inbox: make(chan struct{}, 1),
		dead:  make(chan struct{}),
	}
	c.out <- []byte("full\n")
	c.inbox <- struct{}{}
	start := time.Now()
	c.dispatch([]byte("this is not json"))
	c.dispatch([]byte(`{"jsonrpc":"2.0","id":7,"method":"fs/read_text_file","params":{}}`))
	if d := time.Since(start); d > time.Second {
		t.Fatalf("the reader waited %s on its own replies", d)
	}
	if len(c.out) != 1 {
		t.Fatalf("a reply was queued past the full queue: %d", len(c.out))
	}
}

// standInCommand runs this test binary as the stand-in. Under -race the
// child would wait a second at exit (the race runtime's default): not here.
func standInCommand(exe, mode string) Command {
	wd, _ := os.Getwd() // Start wants an absolute folder
	return Command{Path: exe, Dir: wd, Env: append(os.Environ(), standInEnv+"="+mode, "GORACE=atexit_sleep_ms=0")}
}
