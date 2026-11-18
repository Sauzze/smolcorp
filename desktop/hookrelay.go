package desktop

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// A plain hire's hook relay (SA-M10). A hire the launcher starts in its sandbox (launchersandbox_darwin.go) can't read
// the hook key in the support folder, and has no key of its own: only the engine's desks, which have a registry row,
// get one (hookkeys.go). Nor is it handed the owner's key, which would let it report as the owner's own sessions
// (AQ3). So its hooks hand their events to its launcher, outside the sandbox, on the hire's own socket, the one unix
// socket its sandbox lets it reach: the launcher sends each one on to the app with the owner's key, as its own tab says
// (launcher:<its id>; none for Codex, whose tab the app finds by folder), for its own agent, and with nothing else the
// hire said about where it runs. So a sandboxed plain hire reports as itself, and can do nothing more with the relay
// than send an event: no other route, no other tab, no key.

// hookRelayEnvironment is where a sandboxed plain hire's hook finds its launcher's relay: the socket's whole path.
const hookRelayEnvironment = "SMOLCORP_HOOK_RELAY"

// hookRelayPath is the relay's socket, as the path the URL names: anything else isn't the relay.
const hookRelayPath = "/hook"

// hookRelayFor is where a hook sends its event when it has no key of its own: the relay its launcher named, a whole
// path, or "".
func hookRelayFor(getenv func(string) string) string {
	if strings.TrimSpace(getenv(hookKeyEnvironment)) != "" {
		return "" // a desk's own key goes straight to the app
	}
	if relay := getenv(hookRelayEnvironment); filepath.IsAbs(relay) && filepath.Clean(relay) == relay {
		return relay
	}
	return ""
}

// hookRelayClient is the hook's client for the relay at socket: every request goes to it, whatever its URL names.
func hookRelayClient(socket string) *http.Client {
	return &http.Client{Timeout: hookTimeout, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}}
}

// hookRelay is a launcher's relay, listening on its hire's own socket.
type hookRelay struct {
	server   *http.Server
	listener net.Listener
}

// startHookRelay listens at socket (its folder made already) and sends each event on (relayHandler).
func startHookRelay(socket string, handler http.Handler) (*hookRelay, error) {
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	r := &hookRelay{listener: listener, server: &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second}}
	go func() { _ = r.server.Serve(listener) }()
	return r, nil
}

// close stops the relay.
func (r *hookRelay) close() {
	if r != nil {
		_ = r.server.Close()
	}
}

// relayHandler sends each event the hire's hook hands it on to the app at base, with the key key gives, as the
// launcher's own: its tab is launcher:<launcher> (none for Codex), its agent is agent, its time the hook's or now. Of
// what the hook said, the event and the folder it was started in alone go on (and how it pays, as the hook says it): no
// tab, tty or pid of the hire's choosing. Only a POST to /hook is taken.
func relayHandler(launcher string, agent Agent, base string, key func() (string, error), client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != hookRelayPath {
			http.NotFound(w, r)
			return
		}
		event, err := io.ReadAll(io.LimitReader(r.Body, mostSent+1))
		if err != nil || len(event) > mostSent {
			http.Error(w, "event too long", http.StatusRequestEntityTooLarge)
			return
		}
		secret, err := key()
		if err != nil || secret == "" {
			http.Error(w, "no key", http.StatusServiceUnavailable)
			return
		}
		out, err := http.NewRequest(http.MethodPost, base+"/hook", bytes.NewReader(event))
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		out.Header.Set("Content-Type", "application/json")
		out.Header.Set(hookKeyHeader, secret)
		out.Header.Set("X-AgentDeck-Agent", string(agent))
		at := r.Header.Get("X-AgentDeck-At")
		if _, err := strconv.ParseInt(at, 10, 64); err != nil {
			at = strconv.FormatInt(time.Now().UnixMilli(), 10)
		}
		out.Header.Set("X-AgentDeck-At", at)
		tab := "launcher:" + launcher
		if agent == Codex {
			tab = "" // as the hook says it: Codex's shared hook service names no tab, and the app finds it by folder
		}
		out.Header.Set("X-AgentDeck-Tab", tab)
		if project := r.Header.Get("X-AgentDeck-Project"); filepath.IsAbs(project) && strings.IndexFunc(project, unicode.IsControl) < 0 {
			out.Header.Set("X-AgentDeck-Project", project)
		}
		if billing := r.Header.Get("X-AgentDeck-Billing"); slices.Contains([]string{billingCloud, billingAPIKey}, billing) {
			out.Header.Set("X-AgentDeck-Billing", billing)
		}
		response, err := client.Do(out)
		if err != nil {
			http.Error(w, "", http.StatusBadGateway)
			return
		}
		response.Body.Close()
		w.WriteHeader(response.StatusCode)
	})
}

// relayClient is the relay's client to the app.
var relayClient = &http.Client{Timeout: hookTimeout}

// launcherHookKey is the key the relay sends with: the launcher's own, as it reaches the app with (serveDeck).
func launcherHookKey() (string, error) {
	key, err := os.ReadFile(hookKeyFile)
	if err != nil {
		return "", err
	}
	if secret := strings.TrimSpace(string(key)); secret != "" {
		return secret, nil
	}
	return "", errors.New("no hook key")
}
