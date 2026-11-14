package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

var (
	ErrClosed       = errors.New("acp: the client is closed")
	ErrAgentExited  = errors.New("acp: the agent stopped")
	ErrTooBig       = errors.New("acp: message over the size cap")
	ErrTimeout      = errors.New("acp: no answer in time")
	ErrNotReady     = errors.New("acp: initialize first")
	ErrCantLoad     = errors.New("acp: the agent can't load sessions")
	errNotAbsolute  = errors.New("acp: the session folder must be an absolute path")
	errNoAgentPath  = errors.New("acp: no agent to start")
	errNoEnv        = errors.New("acp: the agent's environment must be given (the hire's, from hireEnvironment())")
	errBadDir       = errors.New("acp: the agent's folder must be an absolute path")
	errMethodAnswer = &RPCError{Code: CodeMethodNotFound, Message: "method not found"}
)

// maxInbound caps the agent's requests being answered at once; past it the
// agent is told to wait (an error), so a flood can't pile up goroutines.
const maxInbound = 32

// groupGrace is how long what the agent started has to obey SIGTERM, after
// the agent is gone, before it is killed.
const groupGrace = time.Second

// Handler is the caller: it sees every update and answers every permission
// request. Update is called in order, on the client's reading goroutine,
// before the answer to the call that caused it; it must return quickly and
// must not wait on the client. RequestPermission may wait for the owner: its
// context ends when the turn is cancelled, the client closes or TurnTimeout
// passes, and the agent is then told "cancelled".
type Handler interface {
	Update(SessionNotification)
	RequestPermission(ctx context.Context, req PermissionRequest) (PermissionOutcome, error)
}

// FileHandler, if the Handler also is one, answers fs/read_text_file and
// fs/write_text_file (and the agent is told the client can). Otherwise the
// agent gets "method not found". The client itself never touches a file.
type FileHandler interface {
	ReadTextFile(ctx context.Context, req ReadTextFileRequest) (ReadTextFileResponse, error)
	WriteTextFile(ctx context.Context, req WriteTextFileRequest) error
}

// TerminalHandler, if the Handler also is one, answers the terminal/*
// methods. The client itself never runs a command.
type TerminalHandler interface {
	CreateTerminal(ctx context.Context, req CreateTerminalRequest) (CreateTerminalResponse, error)
	TerminalOutput(ctx context.Context, req TerminalRequest) (TerminalOutputResponse, error)
	WaitForTerminalExit(ctx context.Context, req TerminalRequest) (TerminalExitStatus, error)
	KillTerminal(ctx context.Context, req TerminalRequest) error
	ReleaseTerminal(ctx context.Context, req TerminalRequest) error
}

// Command is the agent to start: a program and its arguments, run directly
// (no shell). Start refuses a Command without both Env and Dir, so an agent
// never silently gets Smolcorp's own environment or working folder:
//   - Env is the agent's whole environment. Pass the hire's, built by
//     hireEnvironment() in package desktop (it leaves out Smolcorp's tokens,
//     keys and session variables). An empty, non-nil list is fine; nil is not.
//   - Dir is the folder it runs in, an absolute path.
type Command struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

// Options; zero values take the defaults.
type Options struct {
	CallTimeout   time.Duration // initialize, sessions, file and terminal requests (30 s)
	TurnTimeout   time.Duration // a prompt turn, a permission ask, a terminal wait (30 min)
	ShutdownGrace time.Duration // after stdin closes, before the agent is killed (3 s)
	MaxMessage    int           // bytes in one message, either way (16 MiB)
	Stderr        io.Writer     // the agent's stderr; nil drops it
	ClientInfo    *Implementation
}

func (o Options) withDefaults() Options {
	if o.CallTimeout <= 0 {
		o.CallTimeout = 30 * time.Second
	}
	if o.TurnTimeout <= 0 {
		o.TurnTimeout = 30 * time.Minute
	}
	if o.ShutdownGrace <= 0 {
		o.ShutdownGrace = 3 * time.Second
	}
	if o.MaxMessage <= 0 {
		o.MaxMessage = 16 << 20
	}
	return o
}

// Client is one agent process and the conversation with it. Always Close it.
type Client struct {
	h   Handler
	opt Options
	cmd *exec.Cmd

	stdin  *os.File // our end of the agent's stdin
	stdout *os.File // our end of the agent's stdout
	out    chan []byte
	stop   chan struct{} // the writer flushes, then closes stdin
	inbox  chan struct{} // a slot per request from the agent being answered

	ctx    context.Context // ends with the connection: handlers' asks end too
	cancel context.CancelFunc

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan response
	asks    map[int64]ask
	agent   *InitializeResponse
	err     error         // why the connection ended
	dead    chan struct{} // closed when it ended

	readerDone, waitDone chan struct{}
	stopOnce, closeOnce  sync.Once
	closeErr             error
}

type response struct {
	result json.RawMessage
	err    error
}

type ask struct {
	session string
	cancel  context.CancelFunc
}

// Start runs the agent and begins reading it. Call Initialize next.
func Start(command Command, h Handler, opt Options) (*Client, error) {
	if command.Path == "" {
		return nil, errNoAgentPath
	}
	if command.Env == nil {
		return nil, errNoEnv
	}
	if !filepath.IsAbs(command.Dir) {
		return nil, errBadDir
	}
	opt = opt.withDefaults()
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, err
	}
	cmd := disclaim.Cmd(exec.Command(command.Path, command.Args...))
	cmd.Dir, cmd.Env = command.Dir, command.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, opt.Stderr
	cmd.WaitDelay = opt.ShutdownGrace // a child it left behind can't hold Wait open
	ownGroup(cmd)
	err = cmd.Start()
	inR.Close()
	outW.Close()
	if err != nil {
		inW.Close()
		outR.Close()
		return nil, fmt.Errorf("acp: can't start the agent: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		h: h, opt: opt, cmd: cmd, stdin: inW, stdout: outR,
		out: make(chan []byte, 16), stop: make(chan struct{}), inbox: make(chan struct{}, maxInbound),
		ctx: ctx, cancel: cancel,
		pending: map[int64]chan response{}, asks: map[int64]ask{}, dead: make(chan struct{}),
		readerDone: make(chan struct{}), waitDone: make(chan struct{}),
	}
	go c.writer()
	go c.reader()
	go c.waiter()
	return c, nil
}

// Done is closed when the connection ends; Err then says why.
func (c *Client) Done() <-chan struct{} { return c.dead }

func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Initialize agrees the protocol version and says what this client can do.
func (c *Client) Initialize(ctx context.Context) (*InitializeResponse, error) {
	req := initializeRequest{ProtocolVersion: ProtocolVersion, ClientInfo: c.opt.ClientInfo}
	if _, ok := c.h.(FileHandler); ok {
		req.ClientCapabilities.FS = FileSystemCapability{ReadTextFile: true, WriteTextFile: true}
	}
	_, req.ClientCapabilities.Terminal = c.h.(TerminalHandler)
	var resp InitializeResponse
	if err := c.call(ctx, "initialize", req, &resp, c.opt.CallTimeout); err != nil {
		return nil, err
	}
	if resp.ProtocolVersion != ProtocolVersion {
		return nil, &VersionError{Agent: resp.ProtocolVersion}
	}
	c.mu.Lock()
	c.agent = &resp
	c.mu.Unlock()
	return &resp, nil
}

func (c *Client) ready() (*InitializeResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.agent == nil {
		return nil, ErrNotReady
	}
	return c.agent, nil
}

// NewSession starts a conversation in cwd (an absolute folder).
func (c *Client) NewSession(ctx context.Context, cwd string) (string, error) {
	if _, err := c.ready(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(cwd) {
		return "", errNotAbsolute
	}
	var resp NewSessionResponse
	err := c.call(ctx, "session/new", newSessionRequest{Cwd: cwd, MCPServers: []MCPServer{}}, &resp, c.opt.CallTimeout)
	if err == nil && resp.SessionID == "" {
		err = &RPCError{Code: CodeInternalError, Message: "the agent gave no session id"}
	}
	return resp.SessionID, err
}

// LoadSession resumes an earlier conversation. The agent replays it as
// updates, all delivered before LoadSession returns.
func (c *Client) LoadSession(ctx context.Context, sessionID, cwd string) error {
	agent, err := c.ready()
	if err != nil {
		return err
	}
	if !agent.AgentCapabilities.LoadSession {
		return ErrCantLoad
	}
	if !filepath.IsAbs(cwd) {
		return errNotAbsolute
	}
	return c.call(ctx, "session/load", loadSessionRequest{SessionID: sessionID, Cwd: cwd, MCPServers: []MCPServer{}}, nil, c.opt.TurnTimeout)
}

// Prompt sends one turn and waits for its end; its updates all arrive
// before it returns. If the wait ends first (timeout or ctx), the turn is
// cancelled at the agent, but Prompt doesn't wait for the agent's own
// "cancelled" answer: late updates of that turn can still reach the handler
// after Prompt returned, and can mix with the next turn's. The caller (A3)
// must tolerate that, e.g. by dropping updates while no turn is running.
func (c *Client) Prompt(ctx context.Context, sessionID string, prompt []ContentBlock) (StopReason, error) {
	if _, err := c.ready(); err != nil {
		return "", err
	}
	var resp PromptResponse
	err := c.call(ctx, "session/prompt", promptRequest{SessionID: sessionID, Prompt: prompt}, &resp, c.opt.TurnTimeout)
	if err != nil && (errors.Is(err, ErrTimeout) || errors.Is(err, context.Canceled)) {
		cctx, done := context.WithTimeout(context.Background(), c.opt.CallTimeout)
		c.Cancel(cctx, sessionID)
		done()
	}
	return resp.StopReason, err
}

// Cancel stops the session's turn: its open permission asks end (answered
// "cancelled") and the agent is told.
func (c *Client) Cancel(ctx context.Context, sessionID string) error {
	c.mu.Lock()
	for _, a := range c.asks {
		if a.session == sessionID {
			a.cancel()
		}
	}
	c.mu.Unlock()
	return c.send(ctx, outMsg{JSONRPC: "2.0", Method: "session/cancel", Params: cancelNotification{SessionID: sessionID}})
}

// Close ends the conversation: stdin closes (the agent's cue to exit), and
// after ShutdownGrace the agent and anything it started are killed. Whatever
// the agent started is also stopped once the agent itself is gone, however it
// went (see waiter). It returns nil when the agent stopped on its own.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.fail(ErrClosed)
		c.quit()
		select {
		case <-c.waitDone:
		case <-time.After(c.opt.ShutdownGrace):
			c.kill()
			<-c.waitDone
			c.closeErr = fmt.Errorf("acp: the agent didn't stop within %s and was killed", c.opt.ShutdownGrace)
		}
	})
	return c.closeErr
}

// ---- the wire

type outMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  any             `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type inMsg struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// responseID reads the id of an answer: a number, or (sloppy but legal) a
// string holding the number we sent.
func responseID(raw json.RawMessage) (int64, bool) {
	var id int64
	if json.Unmarshal(raw, &id) == nil {
		return id, true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}

func present(v json.RawMessage) bool { return len(v) > 0 && string(v) != "null" }

func (c *Client) call(ctx context.Context, method string, params, result any, d time.Duration) error {
	ctx, done := context.WithTimeout(ctx, d)
	defer done()
	ch := make(chan response, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return c.err
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()
	forget := func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}
	if err := c.send(ctx, outMsg{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprint(id)), Method: method, Params: params}); err != nil {
		forget()
		return err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if result != nil && present(r.result) { // "null" stays the zero value
			if err := json.Unmarshal(r.result, result); err != nil {
				return fmt.Errorf("acp: %s: unreadable answer: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		forget()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("acp: %s: %w", method, ErrTimeout)
		}
		return ctx.Err()
	}
}

// send queues one message for the writer; it never waits past ctx.
func (c *Client) send(ctx context.Context, m outMsg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > c.opt.MaxMessage {
		return ErrTooBig
	}
	select {
	case <-c.dead: // after the end nothing more is sent
		return c.Err()
	default:
	}
	select {
	case c.out <- append(b, '\n'):
		return nil
	case <-c.dead:
		return c.Err()
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrTimeout
		}
		return ctx.Err()
	}
}

// reply answers one of the agent's requests; an answer over the cap becomes an error.
func (c *Client) reply(id json.RawMessage, result any, rerr *RPCError) {
	m := outMsg{JSONRPC: "2.0", ID: id, Error: rerr}
	if rerr == nil {
		b, err := json.Marshal(result)
		if err != nil {
			b, m.Error = nil, &RPCError{Code: CodeInternalError, Message: "unwritable answer"}
		}
		m.Result = b
	}
	if b, _ := json.Marshal(m); len(b) > c.opt.MaxMessage {
		m = outMsg{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: CodeInternalError, Message: "answer over the size cap"}}
	}
	ctx, done := context.WithTimeout(context.Background(), c.opt.CallTimeout)
	defer done()
	c.send(ctx, m)
}

// tryReply is for the reading goroutine: it answers at once or not at all.
// If the queue to the agent is full (it isn't reading us), the answer is
// dropped rather than holding up everything the agent sends.
func (c *Client) tryReply(id json.RawMessage, rerr *RPCError) {
	b, err := json.Marshal(outMsg{JSONRPC: "2.0", ID: id, Error: rerr})
	if err != nil {
		return
	}
	select {
	case <-c.dead:
	case c.out <- append(b, '\n'):
	default:
	}
}

func (c *Client) writer() {
	defer c.stdin.Close()
	write := func(b []byte) bool { _, err := c.stdin.Write(b); return err == nil }
	for {
		select {
		case b := <-c.out:
			if !write(b) {
				return
			}
		case <-c.stop: // flush what's queued (a last cancel), then close stdin
			for {
				select {
				case b := <-c.out:
					if !write(b) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

func (c *Client) reader() {
	defer close(c.readerDone)
	defer c.stdout.Close()
	sc := bufio.NewScanner(c.stdout)
	sc.Buffer(make([]byte, 0, 64<<10), c.opt.MaxMessage+1)
	for sc.Scan() {
		if line := bytes.TrimSpace(sc.Bytes()); len(line) > 0 {
			c.dispatch(line)
		}
	}
	if errors.Is(sc.Err(), bufio.ErrTooLong) {
		// The stream can't be trusted past a cut message: end it.
		c.fail(fmt.Errorf("%w (over %d bytes from the agent)", ErrTooBig, c.opt.MaxMessage))
		c.quit()
		c.kill()
		return
	}
	select { // stdout ended: the waiter reports the exit, unless the agent lives on
	case <-c.waitDone:
	case <-time.After(c.opt.ShutdownGrace):
		c.fail(fmt.Errorf("%w: it closed its output", ErrAgentExited))
		c.quit()
		c.kill()
	}
}

func (c *Client) waiter() {
	werr := c.cmd.Wait()
	// The agent is gone, however it went (it saw stdin close, or it crashed):
	// what it started must not outlive it. Done before waitDone closes, so
	// Close returns only when they are gone too.
	stopGroup(c.cmd.Process, min(c.opt.ShutdownGrace, groupGrace))
	close(c.waitDone)
	c.quit()
	select {
	case <-c.readerDone:
	case <-time.After(c.opt.ShutdownGrace):
		c.stdout.Close() // something it started still holds its output open
	}
	if werr == nil {
		c.fail(ErrAgentExited)
	} else {
		c.fail(fmt.Errorf("%w: %v", ErrAgentExited, werr))
	}
}

func (c *Client) quit() { c.stopOnce.Do(func() { close(c.stop) }) }

// kill ends the agent and its process group, only while it hasn't been
// reaped (so the group id can't be someone else's yet).
func (c *Client) kill() {
	select {
	case <-c.waitDone:
	default:
		killTree(c.cmd.Process)
	}
}

// fail ends the connection once: every waiting call gets err, every
// handler's ask ends.
func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.err = err
	close(c.dead)
	c.cancel()
	for id, ch := range c.pending {
		ch <- response{err: err}
		delete(c.pending, id)
	}
}

func (c *Client) dispatch(line []byte) {
	var m inMsg
	if err := json.Unmarshal(line, &m); err != nil {
		c.tryReply(json.RawMessage("null"), &RPCError{Code: CodeParseError, Message: "parse error"})
		return
	}
	switch {
	case m.Method != "" && present(m.ID):
		select {
		case c.inbox <- struct{}{}:
			go func() {
				defer func() { <-c.inbox }()
				result, rerr := c.serve(m.Method, m.Params)
				c.reply(m.ID, result, rerr)
			}()
		default:
			c.tryReply(m.ID, &RPCError{Code: CodeInternalError, Message: "too many requests at once"})
		}
	case m.Method == "session/update":
		var n SessionNotification
		if json.Unmarshal(m.Params, &n) == nil && c.h != nil {
			c.h.Update(n)
		}
	case m.Method != "":
		// another notification: nothing to do
	case present(m.ID):
		id, ok := responseID(m.ID)
		if !ok {
			return
		}
		c.mu.Lock()
		ch, ok := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ok {
			r := response{result: m.Result}
			if m.Error != nil {
				r.err = m.Error
			}
			ch <- r
		}
	}
}

// serve answers one request from the agent, always through the caller.
func (c *Client) serve(method string, params json.RawMessage) (any, *RPCError) {
	fh, _ := c.h.(FileHandler)
	th, _ := c.h.(TerminalHandler)
	short, long := c.opt.CallTimeout, c.opt.TurnTimeout
	switch {
	case method == "session/request_permission":
		return c.permission(params)
	case method == "fs/read_text_file" && fh != nil:
		return handle(c, params, short, fh.ReadTextFile)
	case method == "fs/write_text_file" && fh != nil:
		return handle(c, params, short, noResult(fh.WriteTextFile))
	case method == "terminal/create" && th != nil:
		return handle(c, params, short, th.CreateTerminal)
	case method == "terminal/output" && th != nil:
		return handle(c, params, short, th.TerminalOutput)
	case method == "terminal/wait_for_exit" && th != nil:
		return handle(c, params, long, th.WaitForTerminalExit)
	case method == "terminal/kill" && th != nil:
		return handle(c, params, short, noResult(th.KillTerminal))
	case method == "terminal/release" && th != nil:
		return handle(c, params, short, noResult(th.ReleaseTerminal))
	}
	return nil, errMethodAnswer
}

func noResult[Req any](f func(context.Context, Req) error) func(context.Context, Req) (struct{}, error) {
	return func(ctx context.Context, r Req) (struct{}, error) { return struct{}{}, f(ctx, r) }
}

func handle[Req, Resp any](c *Client, params json.RawMessage, d time.Duration, f func(context.Context, Req) (Resp, error)) (any, *RPCError) {
	var req Req
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "invalid params"}
	}
	ctx, done := context.WithTimeout(c.ctx, d)
	defer done()
	resp, err := f(ctx, req)
	if err != nil {
		var re *RPCError
		if errors.As(err, &re) {
			return nil, re
		}
		return nil, &RPCError{Code: CodeInternalError, Message: "request failed"}
	}
	return resp, nil
}

// permission asks the caller and answers the agent. Only an option the agent
// offered and the caller picked, before the turn ended, is sent as picked;
// anything else (no handler, an error, a timeout, a cancel) is "cancelled".
func (c *Client) permission(params json.RawMessage) (any, *RPCError) {
	var req PermissionRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: CodeInvalidParams, Message: "invalid params"}
	}
	answer := permissionResponse{Outcome: outcomeWire{Outcome: "cancelled"}}
	if c.h == nil {
		return answer, nil
	}
	ctx, done := context.WithTimeout(c.ctx, c.opt.TurnTimeout)
	defer done()
	c.mu.Lock()
	c.nextID++
	key := c.nextID
	c.asks[key] = ask{session: req.SessionID, cancel: done}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.asks, key)
		c.mu.Unlock()
	}()
	o, err := c.h.RequestPermission(ctx, req)
	if err != nil || ctx.Err() != nil || o.OptionID == "" {
		return answer, nil
	}
	for _, opt := range req.Options {
		if opt.OptionID == o.OptionID {
			return permissionResponse{Outcome: outcomeWire{Outcome: "selected", OptionID: o.OptionID}}, nil
		}
	}
	return answer, nil
}
