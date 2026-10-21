package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	agentdeck "github.com/Sauzze/smolcorp"
	"rsc.io/qr"
)

// The HTTP side, as the Mac app's HTTPServer.swift and DeckRouter.swift:
//
//   - The phone's pages and API need a paired phone's key, sent as ?t= (the
//     page's own URL, so a home-screen bookmark keeps it) or as
//     X-AgentDeck-Token. They must also be addressed to an IP address or a
//     local name, which keeps out web pages using DNS rebinding.
//   - /hook and /launcher need the hook's own key, and answer only to this
//     computer.
//   - Pairing is open, but useless without a code you asked for here.
//   - The setup page answers only to this computer. The page itself holds
//     nothing; what it reads and changes needs the setup key, new at every
//     start, as the cookie the page gets when the tray opens it or as
//     X-Smolcorp-Setup-Key from a tool of your own. Its actions also need a
//     header that other sites can't send without asking first.

const (
	// Far more than a few phones, launchers and hooks ever open at once.
	// Beyond it, new connections are refused, so a flood can't exhaust the app.
	maxConnections = 64
	// No one other address may hold more than this many, and a few are kept
	// for this computer, so a device on the network can't take them all and
	// lock out the phones, or the hook.
	mostPerAddress      = 16
	keptForThisComputer = 8
	// How long a connection kept open between requests is kept.
	idleTimeout = 30 * time.Second
	// Longer than any real request takes to arrive.
	requestTimeout = 10 * time.Second
	maxRequestSize = 1 << 20
	// Sent by the setup page with every action. A custom header makes the
	// browser ask permission first for any cross-site request, which this
	// server never grants.
	setupHeader = "X-AgentDeck-Setup"
	// The setup key, as a tool of your own sends it, read from the setup-key
	// file; or as the page's cookie, from the tray's link.
	setupKeyHeader = "X-Smolcorp-Setup-Key"
	setupCookie    = "smolcorp-setup"
	// A tray link's one-time token, which the page swaps for the cookie.
	setupTokenHeader = "X-Smolcorp-Setup-Once"
)

// Serve listens on the deck's port until it fails.
func Serve(deck *Deck) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		return err
	}
	// No ReadTimeout: it would end the event streams too. Each request's body
	// gets its own deadline instead, in ServeHTTP.
	server := &http.Server{
		Handler:           &router{deck: deck},
		ReadHeaderTimeout: requestTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxRequestSize,
	}
	return server.Serve(&cappedListener{Listener: listener, byAddress: map[string]int{}})
}

// cappedListener refuses connections beyond its limits rather than queueing
// them: maxConnections in all, mostPerAddress from any one address but this
// computer's, and keptForThisComputer that only this computer can have.
type cappedListener struct {
	net.Listener
	mu        sync.Mutex
	open      int
	byAddress map[string]int
}

func (l *cappedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		address, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if release, ok := l.take(address); ok {
			return &slotConn{Conn: conn, release: release}, nil
		}
		conn.Close()
	}
}

func (l *cappedListener) take(address string) (release func(), ok bool) {
	ip := net.ParseIP(address)
	local := ip != nil && ip.IsLoopback()
	l.mu.Lock()
	defer l.mu.Unlock()
	limit := maxConnections
	if !local {
		limit -= keptForThisComputer
	}
	if l.open >= limit || !local && l.byAddress[address] >= mostPerAddress {
		return nil, false
	}
	l.open++
	l.byAddress[address]++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.open--
		if l.byAddress[address]--; l.byAddress[address] == 0 {
			delete(l.byAddress, address)
		}
	}, true
}

type slotConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *slotConn) Close() error {
	c.once.Do(c.release)
	return c.Conn.Close()
}

type router struct {
	deck *Deck
}

// request is what the router needs to know about one.
type request struct {
	*http.Request
	isLocal             bool // sent from this computer to a local name
	isAddressedDirectly bool // to an IP address or a local name, as a phone would
	remoteAddress       string
}

var (
	portSuffix  = regexp.MustCompile(`:\d+$`)
	ipv4Address = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)
	// The office's recordings, in Resources/web/sounds, served as office.js is.
	sounds = []string{"/sounds/keys.m4a", "/sounds/keys.webm", "/sounds/talk.m4a", "/sounds/talk.webm"}
)

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	// A body sent a byte at a time would hold its connection for as long as
	// it liked. The event streams and launchers stay open, and send nothing.
	if r.URL.Path != "/events" && r.URL.Path != "/launcher" {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(requestTimeout))
	}
	host := portSuffix.ReplaceAllString(r.Host, "")
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(remote)
	req := &request{
		Request:             r,
		isLocal:             ip != nil && ip.IsLoopback() && (host == "localhost" || host == "127.0.0.1" || host == "[::1]"),
		isAddressedDirectly: host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasPrefix(host, "[") || ipv4Address.MatchString(host),
		remoteAddress:       remote,
	}
	w.Header().Set("Cache-Control", "no-store")
	rt.handle(w, req)
}

func (rt *router) handle(w http.ResponseWriter, r *request) {
	d := rt.deck
	switch {
	// Fetched by the phone itself for the home-screen icon, with no key to send.
	case r.URL.Path == "/icon.png":
		d.mu.Lock()
		logo := d.config.Logo
		d.mu.Unlock()
		serveFile(w, "Resources/web/icon-"+logo+".png")
		return
	// A browser tab's icon, drawn for its size: the hire's head and shoulders, or the six keys
	case r.URL.Path == "/favicon-16.png" || r.URL.Path == "/favicon-32.png":
		d.mu.Lock()
		logo := d.config.Logo
		d.mu.Unlock()
		if logo == "smolcorp" {
			serveFile(w, "Resources/web"+r.URL.Path)
		} else {
			serveFile(w, "Resources/web/icon-"+logo+".png")
		}
		return
	case r.URL.Path == "/setup" || strings.HasPrefix(r.URL.Path, "/setup/"):
		if !r.isLocal {
			http.NotFound(w, r.Request)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(setupHeader) != "1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// The tray's link carries a token that works once, after #o=, and the
		// link serve prints the key, after #k=. The page hands either over once
		// for a cookie only this page's requests carry.
		session := r.Method == http.MethodPost && r.URL.Path == "/setup/session"
		redeemed := session && d.redeemSetupToken(r.Header.Get(setupTokenHeader))
		if r.URL.Path != "/setup" && !redeemed && !d.isSetupKey(setupKeyOf(r.Request)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if session {
			http.SetCookie(w, &http.Cookie{Name: setupCookie, Value: d.access.SetupKey, Path: "/setup", HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		rt.setup(w, r)
		return
	case r.URL.Path == "/hook":
		if !r.isLocal || r.Method != http.MethodPost {
			http.NotFound(w, r.Request)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		at, err := strconv.ParseFloat(r.Header.Get("X-AgentDeck-At"), 64)
		if err != nil {
			at = nowMillis()
		}
		// The agent's pid, sent for a headless run: digits only, else ignored.
		pid := 0
		if text := r.Header.Get("X-AgentDeck-PID"); text != "" && strings.Trim(text, "0123456789") == "" && len(text) < 10 {
			pid, _ = strconv.Atoi(text)
		}
		d.IngestFrom(decodeJSON(r.Body), r.Header.Get("X-AgentDeck-Agent"), r.Header.Get("X-AgentDeck-Tab"),
			r.Header.Get("X-AgentDeck-Project"), pid, r.Header.Get("X-AgentDeck-Parent"), at)
		w.WriteHeader(http.StatusNoContent)
		return
	case r.URL.Path == "/launcher":
		if !r.isLocal || r.Method != http.MethodGet {
			http.NotFound(w, r.Request)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rt.launcher(w, r)
		return
	}

	if !r.isAddressedDirectly {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/pair" {
		code := strings.Map(func(c rune) rune {
			if c >= '0' && c <= '9' {
				return c
			}
			return -1
		}, stringValue(decodeJSON(r.Body)["code"]))
		d.mu.Lock()
		key, ok := d.access.Redeem(code, r.remoteAddress, r.UserAgent())
		d.mu.Unlock()
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "That code isn't valid. Get a new one on your computer."})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"key": key})
		return
	}

	// The office's drawing, shared by the deck and the pages that show it. It
	// holds nothing of yours, so it needs no key.
	if r.Method == http.MethodGet && r.URL.Path == "/office.js" {
		serveFile(w, "Resources/web/office.js")
		return
	}
	// Its recordings, the keyboards and the talk, in either kind a browser plays.
	if r.Method == http.MethodGet && slices.Contains(sounds, r.URL.Path) {
		serveFile(w, "Resources/web"+r.URL.Path)
		return
	}

	key := r.URL.Query().Get("t")
	if key == "" {
		key = r.Header.Get(hookKeyHeader)
	}
	d.mu.Lock()
	phone := d.access.Phones.WithKey(key)
	phoneID := ""
	if phone != nil {
		d.access.Phones.MarkSeen(phone)
		phoneID = phone.ID
	}
	d.mu.Unlock()

	// The deck, or the office on a screen of its own: the same page, and the
	// same pairing first.
	if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/office") {
		page := "deck.html"
		if phoneID == "" {
			page = "pair.html"
		}
		serveFile(w, "Resources/web/"+page)
		return
	}
	if phoneID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	session := r.URL.Query().Get("session")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/events":
		rt.events(w, r, phoneID)
	case r.Method == http.MethodPost && r.URL.Path == "/focus":
		writeStatus(w)(d.Focus(session))
	case r.Method == http.MethodPost && r.URL.Path == "/act":
		writeStatus(w)(d.Perform(r.URL.Query().Get("action"), session, r.URL.Query()))
	// ✨ Summary: a few lines on what the session has been doing, or why there are none.
	case r.Method == http.MethodPost && r.URL.Path == "/summary":
		text, err := d.Summarize(session)
		if err != nil {
			writeJSON(w, err.Status, map[string]any{"error": err.Message})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"summary": text})
	default:
		http.NotFound(w, r.Request)
	}
}

func setupKeyOf(r *http.Request) string {
	if key := r.Header.Get(setupKeyHeader); key != "" {
		return key
	}
	if cookie, err := r.Cookie(setupCookie); err == nil {
		return cookie.Value
	}
	return ""
}

func (d *Deck) redeemSetupToken(candidate string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.RedeemSetupToken(candidate)
}

func (d *Deck) isSetupKey(candidate string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.IsSetupKey(candidate)
}

func (d *Deck) isHookKey(candidate string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.IsHookKey(candidate)
}

// MARK: Server-Sent Events

// eventStream pushes the deck's state to one phone as it changes.
type eventStream struct {
	messages chan []byte // nil is a ping
	closed   chan struct{}
	once     sync.Once
}

func newEventStream() *eventStream {
	return &eventStream{messages: make(chan []byte, 8), closed: make(chan struct{})}
}

// send never waits. For a phone that isn't keeping up, the oldest message
// waiting makes way for the newest: each carries the whole state, so the
// newest is the one that matters. A ping (nil) with no room is just dropped.
func (s *eventStream) send(state []byte) {
	for {
		select {
		case s.messages <- state:
			return
		default:
		}
		if state == nil {
			return
		}
		select {
		case <-s.messages:
		default:
		}
	}
}

// close hangs up. The phone's next attempt to reconnect is answered afresh.
func (s *eventStream) close() { s.once.Do(func() { close(s.closed) }) }

func (rt *router) events(w http.ResponseWriter, r *request, phone string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	stream := rt.deck.attach(phone)
	defer rt.deck.detach(stream)
	w.Header().Set("Content-Type", "text/event-stream")
	// Hanging up ends the connection, not just this response, as on the Mac.
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)
	for {
		select {
		case state := <-stream.messages:
			if state == nil {
				// A ping, so Safari and anything in between keep an idle
				// stream open, and the page, hearing it, knows the stream is
				// still alive. Without data, the browser wouldn't pass it on.
				_, _ = io.WriteString(w, "event: ping\ndata: 1\n\n")
			} else {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", state)
			}
			flusher.Flush()
		case <-stream.closed:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// launcher keeps a launcher's connection open and sends it the deck's keys.
func (rt *router) launcher(w http.ResponseWriter, r *request) {
	flusher, ok := w.(http.Flusher)
	query := r.URL.Query()
	id := query.Get("id")
	if !ok || id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tab := rt.deck.launcherConnected(id, Agent(query.Get("agent")), query.Get("folder"), query.Get("role"))
	defer rt.deck.launcherGone(id, tab)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	encoder := json.NewEncoder(w)
	for {
		select {
		case command := <-tab.commands:
			if encoder.Encode(command) != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// MARK: Setup page

func (rt *router) setup(w http.ResponseWriter, r *request) {
	d := rt.deck
	query := r.URL.Query()
	path := r.URL.Path
	d.mu.Lock()
	config := d.config
	d.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && path == "/setup":
		d.mu.Lock()
		if !config.HasCompletedSetup {
			config.HasCompletedSetup = true
			config.save()
		}
		d.mu.Unlock()
		serveFile(w, "Resources/web/setup.html")

	case r.Method == http.MethodGet && path == "/setup/state":
		writeJSON(w, http.StatusOK, rt.setupState())

	// The deck, for the preview beside the layout editor: the one page a
	// frame may show, and only one of this deck's own.
	case r.Method == http.MethodGet && path == "/setup/preview":
		serveFramedBy(w, "Resources/web/deck.html", "SAMEORIGIN", "'self'")

	case r.Method == http.MethodPost && path == "/setup/layout":
		data, _ := io.ReadAll(r.Body)
		layout, err := parseLayout(data)
		if err != nil {
			writeStatus(w)(http.StatusBadRequest, err.Error())
			return
		}
		d.mu.Lock()
		config.Layout = layout
		config.save()
		d.changed()
		d.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPost && path == "/setup/profiles":
		rt.profiles(w, r)

	case r.Method == http.MethodGet && path == "/setup/qr.png":
		d.mu.Lock()
		code := d.access.ActiveCode()
		d.mu.Unlock()
		if code == "" {
			http.NotFound(w, r.Request)
			return
		}
		image, err := qr.Encode(pairingLink(code), qr.M)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		image.Scale = 8
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(image.PNG())

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/setup/logo/"):
		logo := strings.TrimPrefix(path, "/setup/logo/")
		if !contains(logos, logo) {
			http.NotFound(w, r.Request)
			return
		}
		serveFile(w, "Resources/web/icon-"+logo+".png")

	case r.Method == http.MethodPost && path == "/setup/pair":
		d.mu.Lock()
		d.access.NewCode()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/unpair":
		id := query.Get("phone")
		if query.Get("all") != "1" && id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		d.unpair(id)
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && (path == "/setup/pin" || path == "/setup/unpin"):
		index, err := strconv.Atoi(query.Get("key"))
		if err != nil || index < 0 || index >= mostKeys {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		if path == "/setup/unpin" {
			config.Unpin(index)
		} else if folder := query.Get("folder"); folder != "" {
			config.Pin(folder, index)
		} else {
			d.mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.changed()
		d.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPost && path == "/setup/theme":
		d.mu.Lock()
		config.Colours = customisations(decodeJSON(r.Body))
		config.save()
		d.changed()
		d.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPost && path == "/setup/logo":
		logo := query.Get("logo")
		if !contains(logos, logo) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		config.Logo = logo
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	// Every part is checked before any is kept, so a wrong one changes
	// nothing. In the body, not the address, as it carries the secret.
	case r.Method == http.MethodPost && path == "/setup/webhook":
		var given struct {
			Enabled *bool     `json:"enabled"`
			URL     *string   `json:"url"`
			Secret  *string   `json:"secret"`
			Outside *bool     `json:"outside"`
			Events  *[]string `json:"events"`
		}
		if data, _ := io.ReadAll(r.Body); json.Unmarshal(data, &given) != nil {
			writeStatus(w)(http.StatusBadRequest, "That isn't a webhook this deck can use.")
			return
		}
		d.mu.Lock()
		webhook := config.Webhook
		if given.Enabled != nil {
			webhook.Enabled = *given.Enabled
		}
		if given.URL != nil {
			webhook.URL = strings.TrimSpace(*given.URL)
		}
		if given.Secret != nil {
			webhook.Secret = *given.Secret
		}
		if given.Outside != nil {
			webhook.Outside = *given.Outside
		}
		if given.Events != nil {
			webhook.Events = knownEvents(*given.Events)
		}
		if webhook.Enabled && webhook.URL != "" {
			if problem := webhookProblem(webhook.URL, webhook.Outside); problem != "" {
				d.mu.Unlock()
				writeStatus(w)(http.StatusBadRequest, problem)
				return
			}
		}
		config.Webhook = webhook
		config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/webhook/test":
		d.mu.Lock()
		webhook := config.Webhook
		d.mu.Unlock()
		d.testWebhook(webhook)
		writeJSON(w, http.StatusOK, rt.setupState())

	// ✨ Summary on only once the page's dialog has said what it sends where,
	// and only with a key; off at any time.
	case r.Method == http.MethodPost && path == "/setup/summaries":
		if query.Get("enabled") == "1" {
			if query.Get("confirmed") != "1" {
				http.Error(w, "Turn Summary on from its dialog, which says what it sends where.", http.StatusBadRequest)
				return
			}
			if !d.summaryKeys.Present() {
				http.Error(w, "Add your Anthropic API key first.", http.StatusBadRequest)
				return
			}
		}
		d.mu.Lock()
		config.SummaryOptIn = query.Get("enabled") == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		d.summaries.Forget()
		writeJSON(w, http.StatusOK, rt.setupState())

	// The key Summary uses: kept in the system's secret store, and answered only as its hint.
	case r.Method == http.MethodPost && path == "/setup/summary/key":
		if query.Get("remove") == "1" {
			d.summaryKeys.Remove()
		} else {
			var given struct {
				Key string `json:"key"`
			}
			_ = json.NewDecoder(r.Body).Decode(&given)
			if err := d.summaryKeys.Save(given.Key); err != nil {
				http.Error(w, err.Message, err.Status)
				return
			}
		}
		d.summaries.Forget()
		d.mu.Lock()
		// With no key left, Summary is off: no key, nothing to send with.
		if !d.summaryKeys.Present() && config.SummaryOptIn {
			config.SummaryOptIn = false
			config.save()
		}
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/music":
		d.mu.Lock()
		config.MusicEnabled = query.Get("enabled") == "1"
		if !config.MusicEnabled {
			d.playing = nil
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	// Every part is checked before any is kept, so a wrong one changes
	// nothing. An empty folder clears it.
	case r.Method == http.MethodPost && path == "/setup/hiring":
		folder := strings.TrimSpace(query.Get("folder"))
		if folder != "" {
			checked, ok := existingFolder(folder)
			if !ok {
				http.Error(w, "There's no folder at "+query.Get("folder")+".", http.StatusBadRequest)
				return
			}
			folder = checked
		}
		if query.Has("agent") && !slices.Contains(agents, Agent(query.Get("agent"))) {
			http.Error(w, "Unknown agent", http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		if query.Has("enabled") {
			config.HiringEnabled = query.Get("enabled") == "1"
		}
		if query.Has("folder") {
			config.HireFolder = folder
		}
		if query.Has("agent") {
			config.HireAgent = query.Get("agent")
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	// The roles as the page has them, all of them: checked as they're kept,
	// since their instructions go to agents.
	case r.Method == http.MethodPost && path == "/setup/roles":
		var roles []Role
		if json.NewDecoder(r.Body).Decode(&roles) != nil {
			http.Error(w, "Those aren't roles this app can use.", http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		config.Roles = checkedRoles(roles)
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/scroll":
		d.mu.Lock()
		config.ScrollEnabled = query.Get("enabled") == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/hooks":
		var err error
		if query.Get("remove") == "" {
			err = connectAgents()
		} else {
			err = disconnectAgents()
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rt.setupState())

	case r.Method == http.MethodPost && path == "/setup/login":
		setOpenAtLogin(query.Get("enabled") == "1")
		writeJSON(w, http.StatusOK, rt.setupState())

	default:
		http.NotFound(w, r.Request)
	}
}

// profiles switches to, saves, renames, deletes or imports a profile, and
// answers with the setup state.
func (rt *router) profiles(w http.ResponseWriter, r *request) {
	d := rt.deck
	query := r.URL.Query()
	name := query.Get("name")
	// What a file's profile would be, without adding it.
	if query.Get("action") == "check" {
		data, _ := io.ReadAll(r.Body)
		profile, err := checkedProfile(data)
		// No room is said up front, not after the warning.
		d.mu.Lock()
		if len(d.config.Profiles) >= mostProfiles {
			err = errTooMany
		}
		d.mu.Unlock()
		if err != nil {
			writeStatus(w)(http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, profile)
		return
	}
	d.mu.Lock()
	var err error
	switch query.Get("action") {
	case "switch":
		err = d.config.SwitchProfile(name)
	case "save":
		err = d.config.SaveProfile(name)
	case "rename":
		err = d.config.RenameProfile(name, query.Get("to"))
	case "delete":
		d.config.DeleteProfile(name)
	case "import":
		data, _ := io.ReadAll(r.Body)
		err = d.config.ImportProfile(data)
	default:
		err = errors.New("Unknown action.")
	}
	if err == nil {
		d.changed()
	}
	d.mu.Unlock()
	if err != nil {
		writeStatus(w)(http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rt.setupState())
}

func (rt *router) setupState() map[string]any {
	d := rt.deck
	found := foundAgents()
	agentList := make([]map[string]any, len(found))
	for i, agent := range found {
		agentList[i] = map[string]any{"id": agent, "name": agent.Name(), "connected": isConnected(agent)}
	}
	connected := allConnected()
	openAtLogin := isOpenAtLogin()

	d.mu.Lock()
	defer d.mu.Unlock()
	keys, _ := d.layout()
	keyOf := map[string]int{}
	for i, key := range keys {
		if key.session != nil {
			keyOf[key.session.ID] = i
		}
	}
	pairing := map[string]any{"address": phoneAddress(), "codeExpiresIn": 0}
	if code := d.access.ActiveCode(); code != "" {
		pairing["code"] = code
		pairing["codeExpiresIn"] = int(d.access.CodeExpiresIn().Seconds())
	}
	phones := make([]map[string]any, len(d.access.Phones.All))
	for i, phone := range d.access.Phones.All {
		phones[i] = map[string]any{
			"id": phone.ID, "name": phone.Name,
			"pairedAt": phone.PairedAt.UnixMilli(), "lastSeen": phone.LastSeen.UnixMilli(),
			"connected": d.isConnected(phone.ID),
		}
	}
	// Every key a project can be pinned to, even those the phone doesn't
	// show: with Auto, a pin there adds keys up to it; with a fixed number,
	// a pin left beyond them can still be undone.
	pins := make([]any, mostKeys)
	for i, pin := range d.config.Pins[:mostKeys] {
		if pin != nil {
			pins[i] = map[string]any{"folder": *pin, "name": lastPathComponent(*pin)}
		}
	}
	sessions := []map[string]any{}
	for _, s := range d.sessions.All() {
		var key any
		if index, ok := keyOf[s.ID]; ok {
			key = index
		}
		sessions = append(sessions, map[string]any{
			"id": s.ID, "folder": s.Project, "name": s.ProjectName(), "agentName": s.Agent.Name(),
			"state": s.State, "key": key,
		})
	}
	return map[string]any{
		// No accessibility step: nothing here needs that permission.
		"checklist": map[string]any{"hooks": connected, "scroll": d.config.ScrollEnabled, "openAtLogin": openAtLogin},
		"agents":    agentList,
		// Firing works here; hiring, which opens a terminal, is the Mac app's for now.
		"hiring": map[string]any{
			"enabled": d.config.HiringEnabled, "folder": d.config.HireFolder, "agent": d.config.HireAgent,
			"canHire": false, "canChoose": false,
		},
		// The roles, those that come with the app to go back to, and what a
		// role can be: its agent, whether that agent has an effort and takes a
		// first message, its badge, and how long each part can be.
		"roles": map[string]any{
			"roles": d.config.Roles, "shipped": shippedRoles, "agents": roleAgents(), "efforts": effortLevels, "badges": roleBadges,
			"limits": map[string]int{"name": roleNameLimit, "model": roleModelLimit, "instructions": roleInstructionsLimit, "own": ownRolesMost,
				"questions": roleQuestionsMost, "question": roleQuestionLimit},
		},
		"music":   map[string]any{"enabled": d.config.MusicEnabled},
		"summary": d.summaryState(),
		"webhook": map[string]any{
			"enabled": d.config.Webhook.Enabled, "url": d.config.Webhook.URL, "secret": d.config.Webhook.Secret,
			"outside": d.config.Webhook.Outside, "events": d.config.Webhook.Events, "last": d.lastDelivery(),
		},
		// What to type to start an agent through the launcher.
		"launcher": launchCommand(),
		"pairing":  pairing,
		"phones":   phones,
		"logo":     d.config.Logo,
		"pins":     pins,
		// How many keys the phone shows now.
		"phoneKeys": len(keys),
		"sessions":  sessions,
		"theme":     themeEditor(d.config),
		"layout":    d.config.Layout,
		// Saved looks, besides Default, the one the deck was last switched
		// to, and whether it has changed since.
		// A copy: the reply is written after the lock is let go.
		"profiles":      slices.Clone(d.config.Profiles),
		"activeProfile": d.config.ActiveProfile,
		"profileEdited": d.config.profileEdited(),
	}
}

// MARK: Helpers

// serveFile serves a page no other page may show in a frame, where it could
// lay something over it and trick you into pressing a button.
func serveFile(w http.ResponseWriter, name string) {
	serveFramedBy(w, name, "DENY", "'none'")
}

// serveFramedBy serves a page with who may frame it, as X-Frame-Options and
// as the Content-Security-Policy's frame-ancestors.
func serveFramedBy(w http.ResponseWriter, name, frameOptions, frameAncestors string) {
	data, err := fs.ReadFile(agentdeck.Files, name)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(name, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".m4a"):
		w.Header().Set("Content-Type", "audio/mp4")
	case strings.HasSuffix(name, ".webm"):
		w.Header().Set("Content-Type", "audio/webm")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Frame-Options", frameOptions)
		w.Header().Set("Content-Security-Policy", "frame-ancestors "+frameAncestors)
	}
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// writeStatus answers with a status and, unless it's empty, a message the
// phone shows.
func writeStatus(w http.ResponseWriter) func(int, string) {
	return func(status int, message string) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, message)
	}
}

func decodeJSON(body io.Reader) map[string]any {
	value := map[string]any{}
	_ = json.NewDecoder(body).Decode(&value)
	return value
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
