package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunCore is Smolcorp.app's engine, behind its `engine` switch: the deck's
// server, as Linux runs it, started by the Mac app as its child and never by
// hand. It has no tray and no window: the app shows its pages, and does what
// only macOS can for it over the host bridge (hostbridge.go).
//
//	smolcorp core --port N --support DIR --bridge DIR/SOCKET
//
// The bridge's token is the first line on its standard input, which only the
// app holds: never on the command line or in the environment, which other
// processes of yours can read (ps -E -ww). It is never exported, so nothing
// this engine starts has it. It is sent only to the app that started this
// engine, its parent, checked by the socket's pid at each request (see
// socketBridge); the app serves this engine's pid alone, token or not.
// Without the app's socket and token it doesn't start: it is never a second
// app of its own.
//
// The app talks to it on its standard input and output, which nothing else
// can reach: a line of JSON each way. The first line out says it's ready;
// the app then asks for the office window's key and for setup sessions. When
// the app goes, its input ends, and so does the engine.
func RunCore(args []string, in io.Reader, out, errs io.Writer) int {
	input := bufio.NewReaderSize(in, 4096)
	token, err := readBridgeToken(input, coreTokenWait)
	if err != nil {
		fmt.Fprintf(errs, "%s (%v)\n", coreRefusal, err)
		return 2
	}
	options, err := parseCoreOptions(args, token)
	if err != nil {
		fmt.Fprintf(errs, "%s (%v)\n", coreRefusal, err)
		return 2
	}
	// Only ever the app that started it: the token goes to no other process.
	bridge := &socketBridge{path: options.bridge, token: options.token, parent: os.Getppid}
	if _, err := ask(bridge, bridgeRequest{Action: "hello"}); err != nil {
		fmt.Fprintf(errs, "%s (the app's bridge: %v)\n", coreRefusal, err)
		return 2
	}

	useSupportDir(options.support)
	Port = options.port
	if running, err := acquireInstance(); running {
		fmt.Fprintln(errs, "smolcorp core: another Smolcorp is using this support folder.")
		return 1
	} else if err != nil {
		fmt.Fprintf(errs, "smolcorp: no instance lock (%v), going on without\n", err)
	}
	recordPid()
	defer forgetPid()
	host = bridge
	// Hires' windows open in Terminal, iTerm2 or Ghostty, through the app.
	useMacOpeners(bridge)
	deck := NewDeck()
	failed := make(chan error, 1)
	go func() { failed <- Serve(deck) }()
	if err := waitUntilServing(Port, failed, coreStartWait); err != nil {
		fmt.Fprintf(errs, "smolcorp core: can't serve on port %d: %v\n", Port, err)
		return 1
	}
	// Stopped by the app (SIGTERM) or smolcorp quit: the portable office stops, as on quit.
	stopPortableOnSignal(deck)
	lines := make(chan []byte)
	go func() {
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			lines <- append([]byte(nil), scanner.Bytes()...)
		}
		close(lines)
	}()
	return serveShell(deck, lines, out, failed)
}

// coreTokenWait is how long the app has to give the token, so a core started
// by hand in a terminal says what it is rather than waiting on its input.
var coreTokenWait = 5 * time.Second

// bridgeTokenMost is the longest token line read: the app's is 64 characters.
const bridgeTokenMost = 256

// readBridgeToken reads the token, the first line the app writes on the
// engine's input, before anything else is read from it.
func readBridgeToken(input *bufio.Reader, wait time.Duration) (string, error) {
	type read struct {
		line string
		err  error
	}
	got := make(chan read, 1)
	go func() {
		var line []byte
		for {
			b, err := input.ReadByte()
			if err != nil {
				got <- read{err: err}
				return
			}
			if b == '\n' {
				got <- read{line: string(line)}
				return
			}
			if line = append(line, b); len(line) > bridgeTokenMost {
				got <- read{err: errors.New("too long")}
				return
			}
		}
	}()
	select {
	case r := <-got:
		if r.err != nil {
			return "", errors.New("no bridge token")
		}
		return strings.TrimSuffix(r.line, "\r"), nil
	case <-time.After(wait):
		return "", errors.New("no bridge token")
	}
}

// coreRefusal is what the engine says to anyone but the app.
const coreRefusal = "smolcorp core is Smolcorp.app's engine, started by the app itself: open Smolcorp.app instead"

// coreStartWait is how long the server has to start listening.
var coreStartWait = 10 * time.Second

type coreOptions struct {
	port            int
	support, bridge string
	token           string
}

// parseCoreOptions checks what the app gave: a port of its own, a private
// support folder, its socket inside it, and a token long enough to guess at.
func parseCoreOptions(args []string, token string) (coreOptions, error) {
	flags := flag.NewFlagSet("core", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var o coreOptions
	flags.IntVar(&o.port, "port", 0, "")
	flags.StringVar(&o.support, "support", "", "")
	flags.StringVar(&o.bridge, "bridge", "", "")
	if err := flags.Parse(args); err != nil {
		return o, err
	}
	if flags.NArg() > 0 {
		return o, fmt.Errorf("unexpected %q", flags.Arg(0))
	}
	if len(token) < 32 {
		return o, errors.New("no bridge token")
	}
	o.token = token
	if o.port < 1024 || o.port > 65535 {
		return o, errors.New("no port")
	}
	if !filepath.IsAbs(o.support) || !filepath.IsAbs(o.bridge) {
		return o, errors.New("the support folder and the bridge must be full paths")
	}
	o.support, o.bridge = filepath.Clean(o.support), filepath.Clean(o.bridge)
	if filepath.Dir(o.bridge) != o.support {
		return o, errors.New("the bridge must be in the support folder")
	}
	folder, err := os.Stat(o.support)
	if err != nil || !folder.IsDir() {
		return o, errors.New("no support folder")
	}
	if folder.Mode().Perm()&0o077 != 0 {
		return o, errors.New("the support folder must be yours alone (0700)")
	}
	socket, err := os.Lstat(o.bridge)
	if err != nil || socket.Mode()&os.ModeSocket == 0 {
		return o, errors.New("no bridge socket")
	}
	if socket.Mode().Perm()&0o077 != 0 {
		return o, errors.New("the bridge socket must be yours alone (0600)")
	}
	return o, nil
}

// waitUntilServing waits for the port to take connections, or the server to fail.
func waitUntilServing(port int, failed <-chan error, wait time.Duration) error {
	address := net.JoinHostPort("127.0.0.1", fmt.Sprint(port))
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); {
		select {
		case err := <-failed:
			return err
		default:
		}
		if conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond); err == nil {
			conn.Close()
			// A server that failed to listen says so at once: the port may be another's.
			select {
			case err := <-failed:
				return err
			case <-time.After(100 * time.Millisecond):
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("it didn't start in time")
}

// shellRequest is one line from the app, and shellAnswer the line back.
type shellRequest struct {
	ID   int    `json:"id"`
	Op   string `json:"op"`
	Then string `json:"then,omitempty"`
}

type shellAnswer struct {
	ID    int    `json:"id,omitempty"`
	Ready bool   `json:"ready,omitempty"`
	Port  int    `json:"port,omitempty"`
	Error string `json:"error,omitempty"`
	// office: the office window's own key, as the browser's office reads it (Access.WindowKey).
	Key string `json:"key,omitempty"`
	// setup-session: a page session for the Settings window, and its cookie's name (said when ready too).
	Session string `json:"session,omitempty"`
	Cookie  string `json:"cookie,omitempty"`
	// setup-link: a link to setup that works once, for the browser.
	Link string `json:"link,omitempty"`
}

// serveShell says it's ready, then answers the app until its input ends or the server fails.
func serveShell(deck *Deck, lines <-chan []byte, out io.Writer, failed <-chan error) int {
	encoder := json.NewEncoder(out)
	if encoder.Encode(shellAnswer{Ready: true, Port: Port, Cookie: setupCookie}) != nil {
		return 1
	}
	for {
		select {
		case err := <-failed:
			fmt.Fprintf(os.Stderr, "smolcorp core: the server stopped: %v\n", err)
			return 1
		case line, open := <-lines:
			if !open {
				// The app has gone: the engine goes with it, as on quit.
				deck.portable.Stop(viewerQuitWait)
				return 0
			}
			if encoder.Encode(answerShell(deck, line)) != nil {
				return 1
			}
		}
	}
}

func answerShell(deck *Deck, line []byte) shellAnswer {
	var request shellRequest
	if err := json.Unmarshal(line, &request); err != nil {
		return shellAnswer{Error: "not a request"}
	}
	answer := shellAnswer{ID: request.ID}
	switch request.Op {
	case "office":
		deck.mu.Lock()
		answer.Key = deck.access.WindowKey
		deck.mu.Unlock()
	case "setup-session":
		answer.Session, answer.Cookie = deck.newSetupSession(), setupCookie
	case "setup-link":
		// Only what the app's menus open setup at.
		switch request.Then {
		case "", "pair", "unpair", "viewer", "guide":
			answer.Link = deck.setupLink(request.Then)
		default:
			answer.Error = "unknown page"
		}
	default:
		answer.Error = "unknown op"
	}
	return answer
}
