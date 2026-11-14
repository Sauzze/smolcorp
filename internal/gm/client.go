// Package gm is the command line's side of the front desk: `smolcorp gm`
// asks the Smolcorp running on this computer who is on the team and what has
// happened since you last looked, over the app's /setup/gm routes. It is the
// GM's hands (a Claude Code session) and yours from a terminal.
//
// The GM is whoever sends the GM token: the CLI sends it from
// $SMOLCORP_GM_TOKEN, which a GM hired from Setup has, and so does one started
// with `smolcorp gm login` (login.go). No file holds it. A hire (anyone with
// $SMOLCORP_HIRE_KEY set) never acts as the GM: it sends its hire key only.
package gm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// Exit codes.
const (
	exitOK         = 0
	exitApp        = 1 // the app refused, or answered with an error
	exitUsage      = 2
	exitNotRunning = 3 // nothing answers on the app's port
)

// The app's port, token and headers.
const (
	defaultPort = 7710
	minPort     = 1024 // the app never listens below this
	portEnv     = "SMOLCORP_PORT"
	// testSupportEnv is what the Mac app's test copy names its support folder by (TestCopy.support).
	testSupportEnv = "SMOLCORP_TEST_SUPPORT_DIR"
	tokenEnv       = "SMOLCORP_GM_TOKEN"
	tokenHeader    = "x-smolcorp-gm-token"
	hireKeyEnv     = "SMOLCORP_HIRE_KEY"
	hireHeader     = "x-smolcorp-hire-key"
	// setupKeyHeader is what a person's own tool sends (people.go): the setup key, the owner's, kept in a file only
	// the owner reads.
	setupKeyHeader = "X-Smolcorp-Setup-Key"
	// sessionEnv is the id Claude Code gives the session it runs in; sessionHeader sends it so the
	// Mac app can tell which session is a GM started by hand. The Go app ignores it.
	sessionEnv    = "CLAUDE_CODE_SESSION_ID"
	sessionHeader = "x-smolcorp-session"
	setupHeader   = "X-AgentDeck-Setup"
)

// Limits on a request and its answer.
const (
	requestTimeout = 5 * time.Second
	maxBody        = 16 << 20
)

// The app's name.
const appName = "Smolcorp"

// Error is a failure with the exit code the command ends with. Its message
// never holds the token.
type Error struct {
	Code   int
	Status int // the HTTP status, when the app answered
	Msg    string
	Reason string // the app's own words for a refusal, whole and unscrubbed of line ends
	Cause  error
}

func (e *Error) Error() string { return e.Msg }

// Unwrap lets errors.Is find the reason behind a failure, when it has one.
func (e *Error) Unwrap() error { return e.Cause }

// supportDir is where the app keeps its state, as each app names it: the Mac
// app's folder in Application Support; on Linux and Windows the same folder
// as desktop/paths.go. (It is worked out again here, without importing
// desktop, which makes the folder as a side effect.)
func supportDir() (string, error) {
	// A test copy of the Mac app (scripts/test-copy.sh) keeps its state in a folder of its own: its hires read that, never the real app's.
	if dir := strings.TrimSpace(os.Getenv(testSupportEnv)); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("can't find the app's support folder: %v", err)
	}
	return filepath.Join(base, appdirs.Name(runtime.GOOS)), nil
}

// Env is what the CLI reads from outside: the environment and the support
// folder. Tests give their own.
type Env struct {
	Getenv     func(string) string
	SupportDir func() (string, error)
	ReadFile   func(string) ([]byte, error) // nil reads from disk; tests give their own
}

func (e Env) readFile(name string) ([]byte, error) {
	if e.ReadFile != nil {
		return e.ReadFile(name)
	}
	return os.ReadFile(name)
}

func osEnv() Env { return Env{Getenv: os.Getenv, SupportDir: supportDir} }

// port is $SMOLCORP_PORT, else 7710.
func (e Env) port() (int, error) {
	s := strings.TrimSpace(e.Getenv(portEnv))
	if s == "" {
		return defaultPort, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minPort || n > 65535 {
		return 0, fmt.Errorf("$%s: %q is not a port (%d-65535)", portEnv, s, minPort)
	}
	return n, nil
}

// token is $SMOLCORP_GM_TOKEN. No file holds it any more (1.26, SK2): a GM started by hand gets it from
// `smolcorp gm login`.
func (e Env) token() (string, error) {
	if t := strings.TrimSpace(e.Getenv(tokenEnv)); t != "" {
		return validToken(t)
	}
	return "", &Error{Code: exitApp, Cause: errNoToken, Msg: fmt.Sprintf("no GM token: set $%s (a GM hired from %s has it), or start your GM with smolcorp gm login, which asks %s for one in its window.", tokenEnv, appName, appName)}
}

// validToken refuses a token that couldn't go in a header.
func validToken(t string) (string, error) {
	for _, r := range t {
		if r < 0x21 || r > 0x7e {
			return "", &Error{Code: exitApp, Cause: errBadToken, Msg: "the GM token is malformed; ask the owner to rotate it in Smolcorp's setup."}
		}
	}
	return t, nil
}

// Client talks to the front desk. It holds the token and sends it only in its
// header. A hire's client holds its send key instead, sent in its own header:
// never both.
type Client struct {
	BaseURL string
	token   string
	hireKey string
	// A person's tool (people.go) holds the setup key alone: no GM token, no session id, no hire key.
	setupKey string
	session  string // the session id the token is sent from, when the environment names one
	http     *http.Client
}

// NewClient is a client for the app at 127.0.0.1 on the environment's port,
// for the commands only the GM can use. In a hire's environment (a hire key is
// set) it refuses, without reading any token.
func NewClient(e Env) (*Client, error) { return newEnvClient(e, false) }

// NewSendClient is NewClient for the commands a hire may use too: with
// $SMOLCORP_HIRE_KEY set the CLI is a hire and signs in with that key alone
// (a hire writes to the GM, or to a hire the GM allowed). It never reads the
// GM token file and never uses $SMOLCORP_GM_TOKEN, even when set.
func NewSendClient(e Env) (*Client, error) { return newEnvClient(e, true) }

func newEnvClient(e Env, allowHire bool) (*Client, error) {
	port, err := e.port()
	if err != nil {
		return nil, &Error{Code: exitUsage, Msg: err.Error()}
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	if given := strings.TrimSpace(e.Getenv(hireKeyEnv)); given != "" {
		if !allowHire {
			return nil, &Error{Code: exitApp, Msg: "this command is for the GM; a hire can't use it. A hire can only `smolcorp gm send` and `smolcorp gm preview`."}
		}
		key, kerr := validKey(given)
		if kerr != nil {
			return nil, kerr
		}
		c := newClient(base, "")
		c.hireKey = key
		return c, nil
	}
	token, err := e.token()
	if err != nil {
		// No token: the Mac's way to get one ("Allow a GM started by hand") doesn't exist
		// in the Go app, so say what does, if that is the app answering.
		if errors.Is(err, errNoToken) && newClient(base, "").isGoApp() {
			return nil, &Error{Code: exitApp, Cause: errNoToken, Msg: noGMHere}
		}
		if errors.Is(err, errBadToken) && newClient(base, "").isGoApp() {
			return nil, &Error{Code: exitApp, Cause: errBadToken, Msg: "the GM token is malformed; ask the owner to hire the GM again from Setup."}
		}
		return nil, err
	}
	c := newClient(base, token)
	c.session = validSession(e.Getenv(sessionEnv))
	return c, nil
}

// validSession is a session id as it may go in a header: 1 to 64 letters, digits, "-" or "_".
// Anything else (unset, empty, odd) is "", and no header is sent.
func validSession(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return ""
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return ""
		}
	}
	return s
}

// What the Go app (Smolcorp on Linux or Windows) says about the GM's token: the engine
// gives the GM it hires a token, in memory only, so a new one comes with a new hire.
const (
	noGMHere      = "there's no GM token here: the GM gets one when the owner hires it from Setup, in its own environment ($SMOLCORP_GM_TOKEN)."
	noGMServedYet = "this token isn't the one Smolcorp gave its GM (it changes when the GM is hired again, and Smolcorp forgets it when it restarts). Ask the owner to hire the GM again from Setup."
	// macAppName is what the Mac app names itself by in X-Smolcorp-App.
	macAppName = "mac"
)

// errBadToken marks a token that couldn't go in a header.
var errBadToken = errors.New("malformed GM token")

// errNotServedHere marks the answer for a GM route the Go app doesn't serve yet.
var errNotServedHere = errors.New("route not served by the Go app yet")

// goAppUnserved maps the GM routes the Go app still answers 404 to, to the command that uses each.
var goAppUnserved = map[string]string{
	"POST /setup/gm/tile":        "tile",
	"GET /setup/gm/screens":      "screens",
	"POST /setup/gm/screen":      "screen",
	"GET /setup/claude-accounts": "accounts",
}

// notServedHere is whether err is the Go app saying a GM route isn't there yet.
func notServedHere(err error) bool { return errors.Is(err, errNotServedHere) }

// isGoApp is whether the app answering is the Go app, as the app says itself
// (X-Smolcorp-App on the 401 of /setup/state), never as this CLI's own GOOS
// tells: a CLI on Linux may talk to a Mac through a tunnel. Anything unclear,
// or no answer, is false, so the Mac's wording stays.
func (c *Client) isGoApp() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/setup/state", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	app := strings.TrimSpace(resp.Header.Get("X-Smolcorp-App"))
	return resp.StatusCode == http.StatusUnauthorized && app != "" && app != macAppName
}

// errNoToken marks "there is no GM token".
var errNoToken = errors.New("no GM token")

// validKey refuses a hire key that couldn't go in a header.
func validKey(k string) (string, error) {
	for _, r := range k {
		if r < 0x21 || r > 0x7e {
			return "", &Error{Code: exitApp, Msg: "$" + hireKeyEnv + " is malformed."}
		}
	}
	return k, nil
}

func newClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			// Straight to this computer: never through a proxy, never
			// redirected elsewhere with the token.
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Get asks for path and returns the answer's body.
func (c *Client) Get(path string, query url.Values, timeout time.Duration) ([]byte, error) {
	return c.do(http.MethodGet, path, query, nil, timeout)
}

// Post sends body (JSON, or nil) to path, with the setup header the gate asks of every POST.
func (c *Client) Post(path string, query url.Values, body any) ([]byte, error) {
	var data []byte
	if body != nil {
		var err error
		if data, err = json.Marshal(body); err != nil {
			return nil, &Error{Code: exitApp, Msg: "can't encode the request: " + err.Error()}
		}
	}
	return c.do(http.MethodPost, path, query, data, requestTimeout)
}

func (c *Client) do(method, path string, query url.Values, body []byte, timeout time.Duration) ([]byte, error) {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	if timeout <= 0 {
		timeout = requestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, &Error{Code: exitApp, Msg: c.scrub("bad request: " + err.Error())}
	}
	if c.hireKey != "" {
		req.Header.Set(hireHeader, c.hireKey)
	} else {
		if c.token != "" {
			req.Header.Set(tokenHeader, c.token)
			if c.session != "" {
				req.Header.Set(sessionHeader, c.session)
			}
		}
		if c.setupKey != "" {
			req.Header.Set(setupKeyHeader, c.setupKey)
		}
	}
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		req.Header.Set(setupHeader, "1")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.transportError(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, &Error{Code: exitApp, Msg: c.scrub(fmt.Sprintf("%s's answer was cut off: %v", appName, err))}
	}
	// A token the Go app doesn't know gets a bare 401 there: a stale one, or one from another hire.
	if resp.StatusCode == http.StatusUnauthorized && c.token != "" && c.isGoApp() {
		return nil, &Error{Code: exitApp, Status: resp.StatusCode, Msg: noGMServedYet}
	}
	if resp.StatusCode == http.StatusNotFound {
		if cmd, ok := goAppUnserved[method+" "+path]; ok && c.isGoApp() {
			msg := fmt.Sprintf("`gm %s` isn't available in Smolcorp on Linux and Windows yet.", cmd)
			return nil, &Error{Code: exitApp, Status: resp.StatusCode, Cause: errNotServedHere, Msg: msg}
		} else if ok && strings.HasPrefix(strings.TrimSpace(string(data)), "`gm "+cmd+"` isn't available") {
			// The Go engine under the Mac app says it in its own words (an older app's 404 is empty).
			return nil, &Error{Code: exitApp, Status: resp.StatusCode, Cause: errNotServedHere, Msg: c.scrub(oneLine(strings.TrimSpace(string(data))))}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &Error{Code: exitApp, Status: resp.StatusCode,
			Msg:    c.scrub(fmt.Sprintf("%s answered %d: %s", appName, resp.StatusCode, errorText(resp.StatusCode, data))),
			Reason: c.scrub(fullReason(resp.StatusCode, data))}
	}
	return data, nil
}

func (c *Client) transportError(err error) error {
	notRunning := &Error{Code: exitNotRunning, Msg: fmt.Sprintf("%s isn't running: nothing answers at %s.", appName, c.BaseURL)}
	var oe *net.OpError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &Error{Code: exitApp, Msg: fmt.Sprintf("%s didn't answer in time at %s.", appName, c.BaseURL)}
	case errors.Is(err, syscall.ECONNREFUSED):
		return notRunning
	// On Windows a refused connection is WSAECONNREFUSED, which Go doesn't
	// match to ECONNREFUSED: any dial that fails without timing out means
	// nothing is listening.
	case errors.As(err, &oe) && oe.Op == "dial" && !oe.Timeout():
		return notRunning
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return &Error{Code: exitApp, Msg: c.scrub(fmt.Sprintf("can't reach %s at %s: %v", appName, c.BaseURL, err))}
}

// errorText is the reason in an error answer: its {"error": …}, or its plain text, on one line.
func errorText(status int, body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	text := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		text = e.Error
	}
	text = truncate(oneLine(text), 300)
	if text == "" {
		text = http.StatusText(status)
	}
	return text
}

// fullReason is errorText without the cut to 300 characters or the joining of lines.
func fullReason(status int, body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	text := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		text = strings.TrimSpace(e.Error)
	}
	if text == "" {
		text = http.StatusText(status)
	}
	return text
}

// scrub takes the token out of a message, should one ever carry it.
func (c *Client) scrub(s string) string {
	if c.token != "" {
		s = strings.ReplaceAll(s, c.token, "[GM token]")
	}
	if c.hireKey != "" {
		s = strings.ReplaceAll(s, c.hireKey, "[hire key]")
	}
	if c.setupKey != "" {
		s = strings.ReplaceAll(s, c.setupKey, "[setup key]")
	}
	return s
}

// oneLine replaces tabs, newlines and other control characters with spaces,
// so a value can't break a line or a column.
func oneLine(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s))
}

// truncate cuts s to n characters, the last one an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
