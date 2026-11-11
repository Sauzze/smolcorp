package desktop

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
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
	// Names the app on the 401 of /setup/state, so the locked page tells how to get in.
	appHeader = "X-Smolcorp-App"
	// The setup key, as a tool of your own sends it, read from the setup-key
	// file; or as the page's cookie, from the tray's link.
	setupKeyHeader = "X-Smolcorp-Setup-Key"
	// A tray link's one-time token, which the page swaps for the cookie.
	setupTokenHeader = "X-Smolcorp-Setup-Once"
	// The office in a browser on this computer, as the deck sees it: see
	// Access.WindowKey.
	officeWindowID = "office-window"
)

// setupCookie is the setup page's session cookie, named as the Mac names it (DeckRouter.setupCookie): a
// browser sends localhost's cookies to every port, so a copy on a port of its own has a name of its own, or
// each office's setup page would sign the other's out.
func setupCookie() string {
	if testCopyPort() == 0 {
		return "smolcorp-setup"
	}
	return fmt.Sprintf("smolcorp-setup-%d", Port)
}

// Serve listens on the deck's port until it fails: on every interface, or on
// this computer and Tailscale only, or a test copy on this computer only (see listen.go).
func Serve(deck *Deck) error {
	deck.mu.Lock()
	scope := effectiveScope(deck.config)
	deck.mu.Unlock()
	listenScope = scope
	// Where tmux is, now: if only your login shell's PATH can say, the shell is
	// asked in the background from here, so no request waits for it.
	if hasTmuxHires {
		findTmux()
	}
	listener, err := listenerFor(scope, Port)
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
	isLocal             bool // sent from this computer to a local name, not passed on by a proxy
	isAddressedDirectly bool // to an IP address or a local name, as a phone would
	// On a Mac: handed on by `tailscale serve` on this computer, to a ts.net name or with a proxy's headers.
	isTailnet     bool
	remoteAddress string
	caller        caller // who is asking setup: set only for /setup and /bots
}

// caller is who is asking for setup: the page, a tool of yours, or no one.
type caller int

const (
	callerNobody caller = iota
	// A tool on this computer sending the setup key from its file in a header.
	callerTool
	// The setup page, with a session cookie a one-time link gave it.
	callerPage
	// The General Manager, with its token in a header (gmTokenHeader).
	callerGM
	// A hire the app started, with its own send key in a header (hireKeyHeader).
	callerHire
)

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
	fromLoopback, proxied := ip != nil && ip.IsLoopback(), isProxied(r.Header)
	// On a Mac, as its app reads a request (HTTPServer.swift's Request.addressed): `tailscale serve` hands a
	// phone's request on from this computer's loopback, with whatever Host the phone sent, so a proxy's hand-on
	// is the tailnet's, never addressed directly; let in only with the phone's text on.
	fromProxy := macTailscale && fromLoopback && proxied
	req := &request{
		Request: r,
		// A proxy on this computer passes on someone else's request: never this computer's, for setup, the hook or the window's key.
		isLocal:             fromLoopback && !proxied && (host == "localhost" || host == "127.0.0.1" || host == "[::1]"),
		isAddressedDirectly: !fromProxy && (host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasPrefix(host, "[") || ipv4Address.MatchString(host)),
		isTailnet:           fromProxy || macTailscale && fromLoopback && isTailnetName(host),
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
			w.WriteHeader(http.StatusNotFound) // empty, as the Mac's 404s
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(setupHeader) != "1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// A link carries a token that works once, after #o=. The page hands it
		// over once for a session cookie only this page's requests carry. The
		// setup key never opens a session, and the cookie is never the key.
		session := r.Method == http.MethodPost && r.URL.Path == "/setup/session"
		redeemed := session && d.redeemSetupToken(r.Header.Get(setupTokenHeader))
		r.caller = d.routeCallerOf(r.Request, r.Method, r.URL.Path)
		if r.URL.Path != "/setup" && !redeemed && r.caller == callerNobody {
			// The locked page says which app it came from, so its note names this app's own way in.
			if r.URL.Path == "/setup/state" {
				w.Header().Set(appHeader, d.appNamed())
			}
			if session {
				writeStatus(w)(http.StatusUnauthorized, setupSessionRefusal)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if session {
			if !redeemed {
				writeStatus(w)(http.StatusUnauthorized, setupSessionRefusal)
				return
			}
			d.completeSetup()
			http.SetCookie(w, &http.Cookie{Name: setupCookie(), Value: d.newSetupSession(), Path: "/setup", HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// What each caller may use is the route table's (routes.go): pairing a
		// phone, and what a hire holding the setup-key file could misuse, are
		// the page's. A tool with the key can see whether pairing is open,
		// never the code. /setup itself is the page.
		if r.URL.Path != "/setup" && !routeAllows(r.caller, r.Method, r.URL.Path) {
			writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
			return
		}
		d.mu.Lock()
		managerOn := d.config.ManagerEnabled
		d.mu.Unlock()
		if managerRefuses(r.Method, r.URL.Path, managerOn) {
			writeStatus(w)(http.StatusConflict, managerOffAnswer)
			return
		}
		// A General Manager the owner held to one project goes no further than it (gmscope.go).
		if r.caller == callerGM {
			if status, why := d.gmScopeRefusal(r); status != 0 {
				writeStatus(w)(status, why)
				return
			}
		}
		rt.setup(w, r)
		return
	// A cloud bot's news, from a tool of yours: guarded as setup's actions are.
	case strings.HasPrefix(r.URL.Path, "/bots/"):
		if !r.isLocal {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(setupHeader) != "1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		who := d.routeCallerOf(r.Request, r.Method, r.URL.Path)
		// Nobody known is 401. Anyone known but the table's callers (the page and a tool) is 403, as the Mac's
		// gate says it: the page's cookie may post a bot's news, as a tool's key does.
		if who == callerNobody {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !routeAllows(who, r.Method, r.URL.Path) {
			writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
			return
		}
		writeStatus(w)(d.cloudBotEvent(r.Method, r.URL.Path, r.URL.Query(), decodeJSON(r.Body)))
		return
	case r.URL.Path == "/hook":
		if !r.isLocal || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// A hook header twice is one smuggled in after another, as a line break in a variable the hook was
		// given would make it: not believed, as the Mac's (DeckRouter.hasSmuggledHookHeader).
		if hasSmuggledHookHeader(r.Header) {
			writeStatus(w)(http.StatusBadRequest, "A hook header came twice")
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
		event := decodeJSON(r.Body)
		// ⓘ Info: whether the agent's environment has an API key or a cloud provider, never the key.
		infos.noteBilling(r.Header.Get("X-AgentDeck-Billing"), stringValue(event["session_id"]))
		d.IngestFrom(event, r.Header.Get("X-AgentDeck-Agent"), r.Header.Get("X-AgentDeck-Tab"),
			r.Header.Get("X-AgentDeck-Project"), pid, r.Header.Get("X-AgentDeck-Parent"), at)
		w.WriteHeader(http.StatusNoContent)
		return
	// A second Smolcorp asking this one to show the office instead of staying.
	case r.URL.Path == raisePath:
		if !r.isLocal || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := showOffice(d); err != nil {
			http.Error(w, "The browser could not be started: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case r.URL.Path == "/launcher":
		if !r.isLocal || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rt.launcher(w, r)
		return
	// An engine hire's launcher asks for its start environment, once (hirestartenv.go).
	case r.URL.Path == hireStartPath:
		if !r.isLocal || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		d.startEnvRoute(w, r.URL.Query())
		return
	// A launcher's answer to a look at its window: the state only.
	case r.URL.Path == screenAnswerPath:
		if !r.isLocal || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !d.isHookKey(r.Header.Get(hookKeyHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// The hook key is every agent's; the answer must also carry the
		// secret only that id's launcher was sent.
		if !d.isLauncherSecret(r.URL.Query().Get("id"), r.Header.Get(lookSecretHeader)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var answer screenAnswer
		if json.NewDecoder(r.Body).Decode(&answer) != nil || answer.Look == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !d.launcherScreenAnswered(r.URL.Query().Get("id"), r.Header.Get(lookSecretHeader), answer) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Past setup and the hook: to an IP address or a local name, as a phone asks; or on a Mac, with the phone's
	// text on, handed on by `tailscale serve`, as the Mac app lets it in (DeckRouter.admits).
	d.mu.Lock()
	phoneText := d.config.PhoneMicEnabled
	d.mu.Unlock()
	if !r.isAddressedDirectly && !(r.isTailnet && phoneText) {
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
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "That code isn't valid. Get a new one on your " + d.machine() + "."})
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
	// The first-run guide's arrows, for the setup page: nothing of yours either.
	if r.Method == http.MethodGet && r.URL.Path == "/guide.js" {
		serveFile(w, "Resources/web/guide.js")
		return
	}
	// Its recordings, the keyboards and the talk, in either kind a browser plays.
	if r.Method == http.MethodGet && slices.Contains(sounds, r.URL.Path) {
		serveFile(w, "Resources/web"+r.URL.Path)
		return
	}

	// The tray's Show Office: its token, good once, swapped for the office's
	// own key on this computer, which the page then reads the office with.
	if r.Method == http.MethodGet && r.URL.Path == "/office" && r.isLocal && r.URL.Query().Has("o") {
		d.mu.Lock()
		redeemed, windowKey := d.access.RedeemOfficeToken(r.URL.Query().Get("o")), d.access.WindowKey
		d.mu.Unlock()
		if redeemed {
			http.Redirect(w, r.Request, "/office?t="+url.QueryEscape(windowKey)+"&"+ownWindowMark, http.StatusSeeOther)
			return
		}
	}

	key := r.URL.Query().Get("t")
	if key == "" {
		key = r.Header.Get(hookKeyHeader)
	}
	d.mu.Lock()
	phoneID := ""
	// The office in a browser on this computer reads it as a phone does,
	// from this computer only: a phone that isn't paired, saved or listed.
	if r.isLocal && d.access.IsWindowKey(key) {
		phoneID = officeWindowID
	} else if phone := d.access.Phones.WithKey(key); phone != nil {
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
		d.mu.Lock()
		logo := d.config.Logo
		d.mu.Unlock()
		// The pairing page in the Mac's words under the Mac app, as the Swift engine serves it.
		var worded func([]byte) []byte
		if page == "pair.html" && d.macHost() != nil {
			worded = macPairing
		}
		servePageWithIcon(w, "Resources/web/"+page, "Resources/web/icon-"+logo+".png", worded)
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
	// ⚙︎ in the office's corner, from its own window on this computer: setup opens, as from the tray.
	case r.Method == http.MethodPost && r.URL.Path == "/act" && r.URL.Query().Get("action") == "settings":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "Settings open on your "+d.machine()+".")
			return
		}
		if err := openSetupPage(d); err != nil {
			writeStatus(w)(http.StatusBadGateway, "Couldn't open your browser ("+err.Error()+"). Instead: "+setupAdvice(runtime.GOOS))
			return
		}
		writeStatus(w)(http.StatusNoContent, "")
	// ⧉ window (T796), from the office's own window on this computer, never a phone: whether hires run in
	// the background, so the key shows; then a window attached to the picked hire's own tmux session, as
	// Setup's Open a window (openHireWindow).
	case r.URL.Path == "/hire/window" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, hireWindowElsewhere)
			return
		}
		if r.Method == http.MethodGet {
			_, tmux := hireWays()
			d.mu.Lock()
			background := backgroundHires(d.config, tmux)
			d.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{"background": background})
			return
		}
		hire, found := d.hireInSession(session)
		if !found {
			writeStatus(w)(http.StatusNotFound, "This agent wasn't hired by Smolcorp, so it has no window to open.")
			return
		}
		status, problem := d.openHireWindow(hire)
		writeStatus(w)(status, problem)
	// Open in T3 Code, from the office's own window on this computer, never a phone: what's installed, and
	// whether the picked agent is a hire; then its folder opened, found by its session, never a path that's sent.
	case r.URL.Path == "/t3code" || r.URL.Path == "/t3code/open":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "T3 Code opens on your "+d.machine()+".")
			return
		}
		if !d.t3On() {
			writeStatus(w)(http.StatusNotFound, t3Off)
			return
		}
		hire, found := d.hireInSession(session)
		switch r.Method {
		case http.MethodGet:
			answer := t3JSON(t3StatusNow(r.URL.Query().Get("fresh") == "1", newT3Machine()))
			answer["hire"] = nil
			if found {
				answer["hire"] = t3Title(hire)
			}
			// A hire in T3 Code: its desk (the tag, its status) and branch, for the card.
			if found && hire.Engine == t3Engine {
				lane := t3DeskJSON(hire, d.t3)
				lane["branch"] = hire.Branch
				answer["lane"] = lane
			}
			// Take over in T3 Code, or Hand back, where it's offered (the Go core's only, t3takeover.go).
			if found {
				if offered := t3TakeOverJSON(hire, true); offered != nil {
					answer["takeOver"] = offered
				}
			}
			writeJSON(w, http.StatusOK, answer)
		case http.MethodPost:
			openHireInT3(w, d.t3Latest(hire), found)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	// Take over in T3 Code and Hand back (EXPERIMENTAL, t3takeover.go), from a desk's card in the office's own window
	// on this computer, never a phone: the hire is found by its desk, never by anything sent.
	case r.Method == http.MethodPost && r.URL.Path == "/act" && slices.Contains([]string{"takeover", "handback"}, r.URL.Query().Get("action")):
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "Take over in T3 Code is on your "+t3Here()+".")
			return
		}
		status, answer, ok := d.takeOverAtDesk(r.URL.Query().Get("action"), session)
		if !ok {
			writeStatus(w)(http.StatusNotFound, "Only a hire Smolcorp runs can be taken over in T3 Code, or handed back.")
			return
		}
		if text, isText := answer.(string); isText {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	// Let go of a hire in T3 Code from its desk's card, after its confirm (t3desks.go): the lane's answer, as Setup's.
	case r.Method == http.MethodPost && r.URL.Path == "/act" && r.URL.Query().Get("action") == "letgo":
		status, answer, ok := d.letGoAtDesk(session)
		if !ok {
			writeStatus(w)(d.Perform("letgo", session, r.URL.Query()))
			return
		}
		if text, isText := answer.(string); isText {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	case r.Method == http.MethodPost && r.URL.Path == "/act":
		writeStatus(w)(d.PerformFrom(phoneID, r.URL.Query().Get("action"), session, r.URL.Query()))
	// The phone's text (typed, or heard by its own mic), typed into the session's agent: from a paired phone
	// with its key, over the Wi-Fi as over Tailscale (I34), never from the office's own window. See phonetext.go.
	case r.Method == http.MethodPost && r.URL.Path == "/dictate":
		if phoneID == officeWindowID {
			writeStatus(w)(http.StatusForbidden, phoneTextPhonesOnly)
			return
		}
		body := decodeJSON(r.Body)
		// The confirm box's choice, Send ⏎ or Type only; an older page's says nothing.
		var send *bool
		if value, ok := body["send"].(bool); ok {
			send = &value
		}
		writeStatus(w)(d.TypePhoneText(stringValue(body["text"]), session, send))
	// The last lines of the session's tab, read-only, to see what was typed: over Tailscale only.
	case r.Method == http.MethodPost && r.URL.Path == "/peek":
		if !fromTailnet(r.Request) {
			writeStatus(w)(http.StatusForbidden, peekTailnet)
			return
		}
		status, lines, message := d.Peek(session)
		if status != http.StatusOK {
			writeStatus(w)(status, message)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
	// A typed answer to a menu's "Type something": as /dictate, one line only.
	case r.Method == http.MethodPost && r.URL.Path == "/choose":
		if phoneID == officeWindowID {
			writeStatus(w)(http.StatusForbidden, phoneTextPhonesOnly)
			return
		}
		if r.URL.Query().Get("option") != "other" {
			writeStatus(w)(http.StatusBadRequest, pickWithKeys)
			return
		}
		writeStatus(w)(d.AnswerOther(stringValue(decodeJSON(r.Body)["text"]), r.URL.Query().Get("choice"), session))
	// What a permission prompt is about: over Tailscale only.
	case r.Method == http.MethodGet && r.URL.Path == "/choice":
		if !fromTailnet(r.Request) {
			writeStatus(w)(http.StatusForbidden, choiceTailnet)
			return
		}
		writeStatus(w)(d.ChoiceDetail(session))
	// The approvals list: who waits for you, read only, to a paired phone over Tailscale, never over the Wi-Fi nor
	// to the office's window. Answering is /act and /choose.
	case r.Method == http.MethodGet && r.URL.Path == "/approvals":
		if phoneID == officeWindowID || !fromTailnet(r.Request) {
			writeStatus(w)(http.StatusForbidden, "The approvals show only over Tailscale's HTTPS address.")
			return
		}
		writeJSON(w, http.StatusOK, approvalsAnswer(d.approvalItems(), nowMillis()))
	// ▤ Conversation, in the office's own window on this computer, never a phone: the session's conversation, a page
	// at a time, as it was written (conversation.go).
	case r.Method == http.MethodGet && r.URL.Path == "/conversation":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, d.shownOnlyInTheWindow("The conversation"))
			return
		}
		before, placed := conversationPlace(r.URL.Query(), "before")
		known, sized := conversationPlace(r.URL.Query(), "size")
		if !placed || !sized {
			writeStatus(w)(http.StatusBadRequest, "That isn't a place in the conversation.")
			return
		}
		writeAnswer(w)(d.conversation(session, before, known))
	// ⓘ Info, in the office's own window on this computer, never a phone: what the session runs on, and what it
	// has used (sessioninfo.go).
	case r.Method == http.MethodGet && r.URL.Path == "/session/info":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, d.shownOnlyInTheWindow("The info"))
			return
		}
		id := session
		if r.URL.Query().Has("id") {
			id = r.URL.Query().Get("id")
		}
		writeAnswer(w)(d.sessionInfo(id))
	// ✨ Summary: a few lines on what the session has been doing, or why there are none.
	case r.Method == http.MethodPost && r.URL.Path == "/summary":
		text, err := d.Summarize(session)
		if err != nil {
			writeJSON(w, err.Status, map[string]any{"error": err.Message})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"summary": text})
	// The Mac answers an unknown route with an empty 404.
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// setupSessionRefusal is the Mac's text (DeckRouter.swift) in the 401 of POST /setup/session without a menu link's key.
const setupSessionRefusal = "Only the menu's link opens a setup page."

// callerOf says who is asking, one caller per request, in this order (the same
// as SetupSessions.caller): the page by its session cookie, a hire by its send
// key, a tool by the setup key in its header, the GM by its token. A valid setup
// key wins over the GM's token, so a GM that also holds the owner's key is
// acting as the owner; a wrong key or token never changes what the other one
// gets alone. The cookie is only ever a session id, never the key. A request
// with a valid hire key is never taken as the GM (a wrong one carries no weight,
// as on the Mac), and the setup key never passes for the GM's token, nor for a
// hire's key.
func (d *Deck) callerOf(r *http.Request) caller {
	who, _ := d.callerAndGM(r)
	return who
}

// callerAndGM is callerOf, and whether the request also carries the GM's valid
// token, read under the deck's lock with the rest (the token changes under it).
func (d *Deck) callerAndGM(r *http.Request) (caller, bool) {
	hireKey := r.Header.Get(hireKeyHeader)
	d.mu.Lock()
	page := func() bool { c, err := r.Cookie(setupCookie()); return err == nil && d.access.IsSetupSession(c.Value) }()
	isTool := d.access.IsSetupKey(r.Header.Get(setupKeyHeader))
	isGM := d.isGMToken(r.Header.Get(gmTokenHeader))
	if isGM {
		d.noteGM(r.Header.Get(gmSessionHeader)) // a GM started by hand, and its session (gmbyhand.go)
	}
	d.mu.Unlock()
	if page {
		return callerPage, isGM
	}
	// The registry is read with the deck's lock let go.
	switch {
	case isHireKey(registryPath(), hireKey):
		return callerHire, isGM
	case isTool:
		return callerTool, isGM
	case isGM:
		return callerGM, true
	}
	return callerNobody, false
}

// routeCallerOf is callerOf, with one step more for a route a tool may not use:
// a request with the setup key and the GM's token is the owner's tool for what a
// tool may do (a read-only request with both acts as the owner), but for a route
// the table keeps from a tool, the token makes it the GM, as on the Mac.
func (d *Deck) routeCallerOf(r *http.Request, method, path string) caller {
	who, isGM := d.callerAndGM(r)
	if who == callerTool && isGM && !routeAllows(callerTool, method, path) {
		return callerGM
	}
	return who
}

// hasSmuggledHookHeader is whether any X-AgentDeck-* header came more than once.
func hasSmuggledHookHeader(header http.Header) bool {
	for name, values := range header {
		if len(values) > 1 && strings.HasPrefix(strings.ToLower(name), "x-agentdeck-") {
			return true
		}
	}
	return false
}

func (d *Deck) newSetupSession() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.NewSetupSession()
}

// completeSetup marks setup complete: what a setup page that got in with a
// token or the key does, as the tray's link never reloads /setup with a key.
func (d *Deck) completeSetup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.config.HasCompletedSetup {
		d.config.HasCompletedSetup = true
		d.config.save()
	}
}

func (d *Deck) redeemSetupToken(candidate string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.RedeemSetupToken(candidate)
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
	tab := rt.deck.launcherConnected(id, Agent(query.Get("agent")), query.Get("folder"), query.Get("role"), query.Get("hire"))
	if tab == nil {
		w.WriteHeader(http.StatusConflict) // that id's launcher is still connected
		return
	}
	defer rt.deck.launcherGone(id, tab)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	if encoder.Encode(launcherCommand{Secret: tab.screenSecret}) != nil {
		return
	}
	flusher.Flush()
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

	// The briefs, and the folders a hire from setup can start in (briefroutes.go).
	if d.briefRoute(w, r.Method, path, r.Body) {
		return
	}
	// Setup's own hire sheet (setuphire.go).
	if d.setupHireRoute(w, r.Method, path, r.Body, r.caller) {
		return
	}
	// The demo's page: Setup opened at `smolcorp demo`'s hire (demopage.go).
	if d.demoRoute(w, r) {
		return
	}

	// What only macOS does, under the Mac app (hostsetup.go): not served elsewhere.
	if rt.macSetup(w, r) {
		return
	}
	// What the Mac app does with its own windows, panel and login item, under it (hostshell.go).
	if rt.macShell(w, r) {
		return
	}
	// A GM started by hand and a new GM token, under the Mac app (gmbyhand.go).
	if rt.byHandRoutes(w, r) {
		return
	}
	// The GM's screens and tiling, under the Mac app (gmscreens.go). Elsewhere they're 404, and `smolcorp gm`
	// words it itself.
	if rt.gmScreenRoutes(w, r) {
		return
	}

	switch {
	case r.Method == http.MethodGet && path == "/setup":
		// Only the page or the setup key counts as setup; a GM token or a hire's key
		// doesn't (DeckRouter.swift: completeSetup for .page and .tool).
		keyed := r.caller == callerPage || r.caller == callerTool
		d.mu.Lock()
		if keyed && !config.HasCompletedSetup {
			config.HasCompletedSetup = true
			config.save()
		}
		d.mu.Unlock()
		serveFile(w, "Resources/web/setup.html")

	// The GM's feed: the events after since, at once if there are any, or as soon
	// as one comes, or none after wait seconds (30 at most). Its row in the route
	// table is GET /setup/gm/events (read: gm, page, tool).
	case r.Method == http.MethodGet && path == "/setup/gm/events":
		since, wait, limit := eventQuestion(query)
		found := d.events.wait(r.Context(), since, limit, wait)
		writeJSON(w, http.StatusOK, d.events.answer(found, since))

	case r.Method == http.MethodGet && path == "/setup/state":
		// The Grok Bot app's bots looked for again as setup opens, so a bot made there a moment ago is there.
		d.rounds.run(func() { d.discoverBots(true) })
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

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

	// The Claude accounts Smolcorp follows: a tool may read them, only the page changes them (claudeaccounts.go).
	case (r.Method == http.MethodGet || r.Method == http.MethodPost) && path == "/setup/claude-accounts":
		rt.claudeAccounts(w, r)

	case r.Method == http.MethodPost && path == "/setup/profiles":
		rt.profiles(w, r)

	case r.Method == http.MethodGet && path == "/setup/qr.png":
		d.mu.Lock()
		code := d.access.ActiveCode()
		d.mu.Unlock()
		// Under the Mac app, ?via=tailnet: the same code at this Mac's tailnet address, for the phone mic over
		// HTTPS, with the phone mic on and an address (hostphonemic.go).
		if r.URL.Query().Get("via") == "tailnet" {
			link := ""
			if code != "" && d.macHost() != nil {
				link = d.tailnetPairingLink(code)
			}
			if link == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeQR(w, link)
			return
		}
		// No code, or no address a phone could open: no QR code to scan.
		if code == "" || phoneAddress() == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeQR(w, pairingLink(code))

	// The portable office's viewer (beta). Only this reply carries the link,
	// which holds the key; the write secret is in a reply only once, as it's made.
	case r.Method == http.MethodGet && path == "/setup/viewer":
		// The link holds the key: the page gets it, a tool with the setup-key file doesn't.
		writeJSON(w, http.StatusOK, d.portable.SetupState(r.caller == callerPage))

	case r.Method == http.MethodPost && path == "/setup/viewer":
		var given struct {
			BaseURL        *string `json:"baseURL"`
			HideNames      *bool   `json:"hideNames"`
			StopOnReturn   *bool   `json:"stopOnReturn"`
			StopAfterHours *int    `json:"stopAfterHours"`
		}
		if data, _ := io.ReadAll(r.Body); json.Unmarshal(data, &given) != nil {
			writeStatus(w)(http.StatusBadRequest, "That isn't a viewer setting.")
			return
		}
		d.mu.Lock()
		viewer := config.Viewer
		if given.BaseURL != nil {
			if strings.TrimSpace(*given.BaseURL) == "" {
				viewer.BaseURL = ""
			} else if origin, ok := viewerOrigin(*given.BaseURL); ok {
				viewer.BaseURL = origin
			} else {
				d.mu.Unlock()
				writeStatus(w)(http.StatusBadRequest, viewerAddressProblem)
				return
			}
		}
		if given.HideNames != nil {
			viewer.HideNames = *given.HideNames
		}
		if given.StopOnReturn != nil {
			viewer.StopOnReturn = *given.StopOnReturn
		}
		if given.StopAfterHours != nil && slices.Contains(viewerHourChoices, *given.StopAfterHours) {
			viewer.StopAfterHours = *given.StopAfterHours
		}
		moved := viewer.BaseURL != config.Viewer.BaseURL
		config.Viewer = viewer
		config.save()
		d.mu.Unlock()
		// A new address while it's on: the old viewer is told to forget it, and nothing more goes there.
		if moved {
			d.portable.Stop(0)
		}
		d.portable.setSettings(viewer)
		d.portable.Changed()
		writeJSON(w, http.StatusOK, d.portable.SetupState(false))

	case r.Method == http.MethodPost && path == "/setup/viewer/secret":
		secret, err := d.portable.MakeWriteSecret()
		if err != nil {
			writeStatus(w)(http.StatusInternalServerError, d.portable.secrets.notKept())
			return
		}
		state := d.portable.SetupState(false)
		state["secret"] = secret
		writeJSON(w, http.StatusOK, state)

	case r.Method == http.MethodPost && path == "/setup/viewer/check":
		d.portable.Check()
		writeJSON(w, http.StatusOK, d.portable.SetupState(false))

	case r.Method == http.MethodPost && path == "/setup/viewer/switch":
		if query.Get("on") == "1" {
			d.portable.Start()
		} else {
			d.portable.Stop(0)
		}
		writeJSON(w, http.StatusOK, d.portable.SetupState(false))

	case r.Method == http.MethodPost && path == "/setup/viewer/rotate":
		d.portable.RotateKey()
		writeJSON(w, http.StatusOK, d.portable.SetupState(false))

	case r.Method == http.MethodPost && path == "/setup/viewer/wipe":
		d.portable.Wipe()
		writeJSON(w, http.StatusOK, d.portable.SetupState(false))

	case r.Method == http.MethodGet && path == "/setup/viewer/qr.png":
		link := ""
		if d.portable.IsSetUp() {
			link = d.portable.Link()
		}
		if link == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeQR(w, link)

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/setup/logo/"):
		logo := strings.TrimPrefix(path, "/setup/logo/")
		if !contains(logos, logo) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		serveFile(w, "Resources/web/icon-"+logo+".png")

	case r.Method == http.MethodPost && path == "/setup/pair":
		d.mu.Lock()
		d.access.NewCode()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	case r.Method == http.MethodPost && path == "/setup/unpair":
		id := query.Get("phone")
		if query.Get("all") != "1" && id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		d.unpair(id)
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

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
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

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
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	case r.Method == http.MethodPost && path == "/setup/webhook/test":
		d.mu.Lock()
		webhook := config.Webhook
		d.mu.Unlock()
		d.testWebhook(webhook)
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// ✨ Summary on only once the page's dialog has said what it sends where,
	// and only with what it names working; off at any time.
	case r.Method == http.MethodPost && path == "/setup/summaries":
		provider, _ := validSummaryProvider(query.Get("provider"))
		if query.Get("enabled") == "1" {
			if query.Get("confirmed") != "1" {
				http.Error(w, "Turn Summary on from its dialog, which says what it sends where.", http.StatusBadRequest)
				return
			}
			if _, ok := validSummaryProvider(query.Get("provider")); !ok {
				http.Error(w, "Pick what makes summaries first.", http.StatusBadRequest)
				return
			}
			if refusal := d.summaryRefusal(provider); refusal != "" {
				http.Error(w, refusal, http.StatusBadRequest)
				return
			}
		}
		d.mu.Lock()
		if query.Get("enabled") == "1" {
			config.SummaryProvider = string(provider)
		}
		config.SummaryOptIn = query.Get("enabled") == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		d.summaries.Forget()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// What makes summaries: picked while they're on, only from the dialog,
	// as it changes where they're sent; and only one that works.
	case r.Method == http.MethodPost && path == "/setup/summary/provider":
		provider, ok := validSummaryProvider(query.Get("id"))
		if !ok {
			http.Error(w, "No such way to make summaries.", http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		on := config.SummaryOptIn
		d.mu.Unlock()
		if on && query.Get("confirmed") != "1" {
			http.Error(w, "Change it from its dialog, which says where summaries are sent.", http.StatusBadRequest)
			return
		}
		if refusal := d.summaryRefusal(provider); refusal != "" {
			http.Error(w, refusal, http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		config.SummaryProvider = string(provider)
		config.save()
		d.changed()
		d.mu.Unlock()
		d.summaries.Forget()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The coding agents looked for again, as setup's Check again asks.
	case r.Method == http.MethodPost && path == "/setup/summary/check":
		d.summaryProviders.Refresh(true)
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

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
		// With no key left, summaries made with it are off: nothing to send with, and never sent elsewhere instead.
		if !d.summaryKeys.Present() && config.SummaryOptIn && (config.SummaryProvider == "" || config.SummaryProvider == string(providerAPIKey)) {
			config.SummaryOptIn = false
			config.save()
		}
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The first-run guide, finished or skipped (done=1), or to be shown again (done=0).
	case r.Method == http.MethodPost && path == "/setup/guide":
		d.mu.Lock()
		config.GuideDone = query.Get("done") == "1"
		config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	case r.Method == http.MethodPost && path == "/setup/music":
		// A test copy leaves the owner's music and volume alone (RWTC N2): the jukebox is never turned on there.
		if query.Get("enabled") == "1" && refusedInTestCopy(w, testCopyKeepsMusic) {
			return
		}
		d.mu.Lock()
		config.MusicEnabled = query.Get("enabled") == "1"
		if !config.MusicEnabled {
			d.playing = nil
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The two plain choices (T796), from first setup or Settings: T3 Code on or
	// off, agent windows, and asked=1 once first setup has asked them. Switching
	// to the background detaches each hire's windows on its own tmux server.
	case r.Method == http.MethodPost && path == "/setup/choices":
		_, tmux := hireWays()
		d.mu.Lock()
		switched, status, refusal := setChoices(config, query, tmux)
		if refusal != "" {
			d.mu.Unlock()
			http.Error(w, inMacWords(refusal), status)
			return
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		answer := rt.setupStateFor(r.caller)
		if switched {
			// Under the Mac app, the windows it opened for hires close too, as its Swift engine closes them
			// (hirewindows_close.go); elsewhere a terminal running tmux attach ends as its viewer is detached.
			if host != nil {
				answer["windowsSaid"] = hiresToBackgroundClosing(officeHires(), bridgeCloser{host}, nil)
			} else {
				answer["windowsSaid"] = hiresToBackground(officeHires(), func(h Hire) detached { return detachHireViewers(h, nil) })
			}
		}
		writeJSON(w, http.StatusOK, answer)

	// "Open a window" on a hire's desk in Setup's team list (T796): by the hire's
	// id, from the registry; a window attached to its own tmux server.
	case r.Method == http.MethodPost && path == "/setup/hire/window":
		var given struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&given)
		// Read once: the list may change meanwhile, and i must point into this one.
		hires := officeHires()
		i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == given.ID })
		if i < 0 {
			http.Error(w, "That hire isn't on the team.", http.StatusConflict)
			return
		}
		if status, problem := d.openHireWindow(hires[i]); problem != "" {
			http.Error(w, problem, status)
			return
		}
		w.WriteHeader(http.StatusNoContent)

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
		terminals, tmux := hireWays()
		if query.Has("terminal") && !slices.ContainsFunc(terminals, func(o hireOpener) bool { return o.id == query.Get("terminal") }) {
			http.Error(w, "That terminal isn't on this "+d.machine(), http.StatusBadRequest)
			return
		}
		if query.Get("tmux") == "1" && !tmux {
			http.Error(w, "tmux isn't on this "+d.machine(), http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		enabled, hireFolder := config.HiringEnabled, config.HireFolder
		if query.Has("enabled") {
			enabled = query.Get("enabled") == "1"
		}
		if query.Has("folder") {
			hireFolder = folder
		}
		if enabled && (query.Has("enabled") || query.Has("folder")) && canHireHere() &&
			!hasHireFolder(&Config{HireFolder: hireFolder, FloorRules: config.FloorRules}) {
			d.mu.Unlock()
			http.Error(w, hiringNeedsFolder, http.StatusBadRequest)
			return
		}
		if query.Has("enabled") {
			config.HiringEnabled = query.Get("enabled") == "1"
		}
		if query.Has("folder") {
			config.HireFolder = folder
		}
		if query.Has("agent") {
			config.HireAgent = query.Get("agent")
		}
		if query.Has("terminal") {
			config.HireTerminal = query.Get("terminal")
		}
		if query.Has("tmux") {
			config.HireInTmux = query.Get("tmux") == "1"
		}
		if query.Has("managedWindows") {
			config.HireManagedWindows = query.Get("managedWindows") == "1"
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		// A folder typed while the Mac's folder panel is open closes that panel, as on Start here.
		if folder != "" {
			d.closeMacFolderPanel()
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The system's own folder picker, which a web page can't open.
	case r.Method == http.MethodPost && path == "/setup/hiring/choose":
		if !hasFolderPicker() {
			http.Error(w, "There's no folder picker here: type the folder's path.", http.StatusNotFound)
			return
		}
		if err := d.chooseHireFolder(); err != nil {
			http.Error(w, "The folder picker didn't open.", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Start here's hires folder: the one set, or with none, one suggested; check= checks one as it's typed.
	case r.Method == http.MethodGet && path == "/setup/hire-folder":
		var asked *string
		if query.Has("check") {
			given := query.Get("check")
			asked = &given
		}
		d.mu.Lock()
		saved := config.HireFolder
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, hireFolderState(saved, asked, homeDir))

	// Keeps it, once checked; create=1 makes it first if it isn't there yet. A problem answers 400 with the same JSON, to show with its fix.
	case r.Method == http.MethodPost && path == "/setup/hire-folder":
		folder, refused := keepHireFolder(query.Get("folder"), query.Get("create") == "1", homeDir)
		if refused != nil {
			writeJSON(w, http.StatusBadRequest, hireFolderJSON(*refused, "", homeDir))
			return
		}
		d.mu.Lock()
		config.HireFolder = folder
		config.save()
		d.changed()
		d.mu.Unlock()
		// A folder typed on the page while the Mac's folder panel is open closes that panel: it's chosen now.
		d.closeMacFolderPanel()
		writeJSON(w, http.StatusOK, hireFolderState(folder, nil, homeDir))

	// The hire guard's groups and your own patterns: a tool may read them, only the page changes them (routes.go, guardstate.go).
	case r.Method == http.MethodGet && path == "/setup/guard":
		writeJSON(w, http.StatusOK, guardState(filepath.Join(supportDir, hireGuardFile), runtime.GOOS))

	case r.Method == http.MethodPost && path == "/setup/guard":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		saveGuard(body, filepath.Join(supportDir, hireGuardFile), runtime.GOOS).write(w)

	// The roles as the page has them, all of them: checked as they're kept,
	// since their instructions go to agents.
	case r.Method == http.MethodPost && path == "/setup/roles":
		var roles []Role
		if json.NewDecoder(r.Body).Decode(&roles) != nil {
			http.Error(w, "Those aren't roles this app can use.", http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		config.Roles = checkedRoles(syncedRoles(roles, config.Roles))
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The cloud bots, each at a desk of its own: all of them, as a list, replaced at once.
	case r.Method == http.MethodGet && path == "/setup/bots":
		d.mu.Lock()
		bots := config.CloudBots
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"bots": botsResolved(d.macHost(), bots)})

	case r.Method == http.MethodPost && path == "/setup/bots":
		given, ok := cloudBotsSent(r.Body)
		if !ok {
			writeStatus(w)(http.StatusBadRequest, `Send the bots as a list: {"bots": [{"id": …, "name": …}]}.`)
			return
		}
		d.mu.Lock()
		// The mailbox is the page's to choose (POST /setup/bots/mailbox): what is sent here never sets or changes it.
		bots := keepMailboxes(checkedCloudBots(given), config.CloudBots)
		if d.mac.host != nil {
			// A bot set or changed here is yours: kept whatever the Grok Bot app says.
			bots = pinnedIfChanged(bots, config.CloudBots)
		}
		config.CloudBots = bots
		config.save()
		d.cloud.configure(config.CloudBots)
		d.changed()
		bots = config.CloudBots
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"bots": botsResolved(d.macHost(), bots)})

	// A bot's mailbox folder, page only (routes.json).
	case r.Method == http.MethodPost && path == "/setup/bots/mailbox":
		rt.botMailboxChoose(w, r)

	// One bot pinned or not, or put on a floor, from setup's list; or the floor new bots go to.
	case r.Method == http.MethodPost && path == "/setup/bots/override":
		var given struct {
			ID     string `json:"id"`
			Pinned *bool  `json:"pinned"`
			// A floor that isn't a string changes nothing and isn't refused, as the Mac's does (the page sends a string).
			Floor    any  `json:"floor"`
			Hidden   bool `json:"hidden"`
			BotFloor *int `json:"botFloor"`
			// Under the Mac app: finding the Grok Bot app's bots on or off, and a bot hidden from there shown again.
			AutoDetect *bool  `json:"autoDetect"`
			Show       string `json:"show"`
		}
		h := d.macHost()
		mac := h != nil
		if json.NewDecoder(r.Body).Decode(&given) != nil || given.ID == "" && given.BotFloor == nil && !(mac && (given.AutoDetect != nil || given.Show != "")) {
			writeStatus(w)(http.StatusBadRequest, `Send {"id": …} with "pinned" or "floor", or {"botFloor": …}.`)
			return
		}
		// A bot hidden is kept out by its id in the Grok Bot app, or the app would give it back at the next look.
		var roster []grokRow
		read := false
		if mac && given.Hidden {
			roster, read, _ = hostBotsRoster(h)
		}
		d.mu.Lock()
		if given.BotFloor != nil && *given.BotFloor >= 0 && *given.BotFloor < floorCount {
			config.BotFloor = *given.BotFloor
		}
		if mac && given.AutoDetect != nil {
			config.BotsAutoDetect = given.AutoDetect
		}
		if mac && safeGrokBotID(given.Show) {
			config.HiddenBots = slices.DeleteFunc(config.HiddenBots, func(id string) bool { return strings.EqualFold(id, given.Show) })
		}
		bots := slices.Clone(config.CloudBots)
		found := given.ID == ""
		for i := range bots {
			if bots[i].ID != given.ID {
				continue
			}
			found = true
			if given.Pinned != nil {
				bots[i].Pinned = *given.Pinned
			}
			if floor, isText := given.Floor.(string); isText && slices.Contains(cloudFloors, floor) {
				bots[i].Floor = floor
			}
		}
		if given.Hidden {
			// Taken off; under the Mac app also kept out of the Grok Bot app's bots found. Elsewhere there's no app
			// to find it again in.
			if at := slices.IndexFunc(bots, func(b CloudBot) bool { return b.ID == given.ID }); mac && at >= 0 {
				if !read {
					roster = d.mac.roster
				}
				if app := bots[at].appID(roster); app != "" && !slices.ContainsFunc(config.HiddenBots, func(id string) bool { return strings.EqualFold(id, app) }) {
					config.HiddenBots = append(config.HiddenBots, app)
				}
			}
			bots = slices.DeleteFunc(bots, func(b CloudBot) bool { return b.ID == given.ID })
		}
		if !found {
			d.mu.Unlock()
			writeStatus(w)(http.StatusNotFound, "That bot isn't there any more.")
			return
		}
		config.CloudBots = bots
		config.save()
		d.cloud.configure(config.CloudBots)
		d.changed()
		d.mu.Unlock()
		// Under the Mac app, the Grok Bot app looked in again, as the switches may have changed what's found.
		d.rounds.run(func() { d.discoverBots(false) })
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The six floors' names and styles, all of them, replaced at once and checked as they're kept;
	// and the folders whose sessions sit on each, with "rules", which a setup page from before them leaves out.
	case r.Method == http.MethodPost && path == "/setup/floors":
		body := floorRulesBody(r.Body)
		floors, ok := floorsSent(bytes.NewReader(body))
		rules, haveRules := floorRulesSent(body)
		if !ok && !haveRules {
			writeStatus(w)(http.StatusBadRequest, `Send the floors as a list: {"floors": [{"name": …, "preset": …}]}.`)
			return
		}
		d.mu.Lock()
		if ok {
			config.Floors = floors
		}
		if haveRules {
			config.FloorRules = rules
		}
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Whether the office asks before max effort: the phones are told, as it's theirs to ask.
	case r.Method == http.MethodPost && path == "/setup/maxconfirm":
		d.mu.Lock()
		config.MaxConfirm = query.Get("enabled") == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The office's window kept on top of other windows, where it has one:
	// the tray's Always on Top, and applied at once.
	case r.Method == http.MethodPost && path == "/setup/ontop":
		if !hasNativeWindow {
			http.Error(w, "The office opens in the browser here: there's no window to keep on top.", http.StatusNotFound)
			return
		}
		d.setAlwaysOnTop(query.Get("enabled") == "1")
		d.mu.Lock()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The keys that show and hide the office from anywhere: the Mac app's (hostshell.go under it). Here there's
	// no such key, so they're checked and kept, for the page to show.
	case r.Method == http.MethodPost && path == "/setup/officekey":
		keys, refusal := officeKeyChoice(query.Get("keys"), nil)
		if refusal != "" {
			http.Error(w, refusal, http.StatusBadRequest)
			return
		}
		d.mu.Lock()
		config.OfficeKey = officeKeySaved(keys)
		config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	case r.Method == http.MethodPost && path == "/setup/scroll":
		d.mu.Lock()
		config.ScrollEnabled = query.Get("enabled") == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	case r.Method == http.MethodPost && path == "/setup/hooks":
		var err error
		notice := ""
		if query.Get("remove") == "" {
			notice, err = connectAgents(true)
		} else {
			err = disconnectAgents()
		}
		// A test copy refuses, as the Mac's does (409): the page shows it inline
		if errors.Is(err, errTestCopyAgents) {
			writeStatus(w)(http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			said := err.Error()
			if notice != "" {
				// Connecting partly failed after turning voice on: the notice still reaches the page
				said += "\n\n" + notice
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, said)
			return
		}
		d.setHooksDisconnected(query.Get("remove") != "")
		state := rt.setupStateFor(r.caller)
		if notice != "" {
			state["notice"] = notice
		}
		writeJSON(w, http.StatusOK, state)

	case r.Method == http.MethodPost && path == "/setup/login":
		setOpenAtLogin(query.Get("enabled") == "1")
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The self-test: Start here's checklist, and `smolcorp setup`'s.
	case r.Method == http.MethodGet && path == "/setup/selftest":
		writeJSON(w, http.StatusOK, selfTestReport(d.selfTest()))

	// One of its fixes, by name, and the checklist after it.
	case r.Method == http.MethodPost && path == "/setup/fix":
		// A tool (smolcorp setup) only repairs, and with auto=1: its repair leaves agents you disconnected.
		if !fixAllowed(r.caller, query.Get("fix"), query.Get("auto"), d.macHost() != nil) {
			writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
			return
		}
		if query.Get("fix") == fixRepair && query.Get("auto") == "1" && d.hooksDisconnected() {
			writeStatus(w)(http.StatusConflict, errHooksDisconnected.Error())
			return
		}
		if status, message := d.fix(query.Get("fix")); status >= 400 {
			writeStatus(w)(status, message)
			return
		}
		writeJSON(w, http.StatusOK, selfTestReport(d.selfTest()))

	// The office, from the page: the tray's Show Office.
	case r.Method == http.MethodPost && path == "/setup/office":
		if err := d.openOffice(); err != nil {
			writeStatus(w)(http.StatusBadGateway, "The browser could not be started: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)

	// The front desk: the app's version and features, and the manager switch (see manager.go).
	case r.Method == http.MethodGet && path == "/setup/gm/version":
		d.mu.Lock()
		enabled, underMac := config.ManagerEnabled, d.mac.host != nil
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, versionAnswer(Version, enabled, underMac))

	case r.Method == http.MethodPost && path == "/setup/gm/manager":
		// Routes table: the page only (routes.json lists the GM too, but no GM caller exists here yet).
		if r.caller != callerPage {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		enabled := query.Get("enabled")
		if enabled != "1" && enabled != "0" {
			writeStatus(w)(http.StatusBadRequest, "Say enabled=1 or enabled=0.")
			return
		}
		d.mu.Lock()
		config.ManagerEnabled = enabled == "1"
		config.save()
		d.changed()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"managerEnabled": enabled == "1"})

	// Open in T3 Code (t3code.go): what's installed (again now, as its menu opens: fresh=1), and a hire's
	// folder opened there, by the hire's id: the folder comes from the registry, never from the request.
	// The page alone (routes.go).
	case r.Method == http.MethodGet && path == "/setup/t3code":
		if !d.t3On() {
			http.Error(w, t3Off, http.StatusNotFound)
			return
		}
		d.mu.Lock()
		on := t3CodeAllowed(d.config)
		d.mu.Unlock()
		status := t3StatusNow(query.Get("fresh") == "1", newT3Machine())
		answer := t3JSON(status)
		// Whether a hire may go into T3 Code (t3desk.go), or the one sentence saying why not.
		why := t3LaneRefusal(on, status)
		answer["lane"] = map[string]any{"ok": why == "", "why": why}
		// Whether Take over and Hand back may be used (the Go core's only, t3takeover.go), or why not.
		gate := t3TakeOverGate(on, status)
		answer["takeOver"] = map[string]any{"ok": gate == "", "why": gate, "tag": t3TakeOverTag}
		writeJSON(w, http.StatusOK, answer)
	case r.Method == http.MethodPost && path == "/setup/t3code/open":
		if !d.t3On() {
			http.Error(w, t3Off, http.StatusNotFound)
			return
		}
		id, _ := decodeJSON(r.Body)["id"].(string)
		var hire Hire
		found := false
		for _, h := range readRegistry(registryPath(), false).hires {
			if h.ID == id {
				hire, found = h, true
			}
		}
		openHireInT3(w, d.t3Latest(hire), found)
	// Take over in T3 Code and Hand back (EXPERIMENTAL, t3takeover.go), by the hire's id after the page's confirm:
	// everything else comes from the registry and the hire's own session, never from the request. The page alone.
	case r.Method == http.MethodPost && (path == "/setup/t3code/takeover" || path == "/setup/t3code/handback"):
		id, _ := decodeJSON(r.Body)["id"].(string)
		var status int
		var answer any
		if path == "/setup/t3code/takeover" {
			status, answer = d.takeOverIntoT3(id)
		} else {
			status, answer = d.handBackFromT3(id)
		}
		if text, ok := answer.(string); ok {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	// Let go of a hire in T3 Code (t3deskdeck.go), by its id after the page's confirm: its T3 project removed, its
	// worktree kept. The worktree comes from the registry, never from the request.
	case r.Method == http.MethodPost && path == "/setup/t3code/letgo":
		id, _ := decodeJSON(r.Body)["id"].(string)
		status, answer := d.letGoOfT3(id)
		if text, ok := answer.(string); ok {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)

	// What a first General Manager hire needs (gmpreflight.go), Start here's lines above its
	// Hire button. Outside /setup/gm/, which is closed while the manager is off, as on a first run.
	// An account that isn't there is said, as the hire would say it, rather than checked as the
	// default (DeckRouter.swift): 400, whatever the hire's own status for it.
	case r.Method == http.MethodGet && path == "/setup/hire/gm/preflight":
		account := query.Get("account")
		if strings.TrimSpace(account) != "" {
			d.mu.Lock()
			accounts := claudeAccountList(d.config.ClaudeProfiles)
			d.mu.Unlock()
			if status, why := claudeAccountRefusal(account, accounts); status != 0 {
				writeStatus(w)(http.StatusBadRequest, why)
				return
			}
		}
		writeJSON(w, http.StatusOK, d.gmPreflightOn(account))

	// Start here's Hire your General Manager: the GM role, in the folder for
	// your hires, its questions asked in its terminal. `account=` names the
	// Claude account and `worktree=1` gives it a worktree, as on the Mac.
	case r.Method == http.MethodPost && path == "/setup/hire/gm":
		// One GM per office: the GM's token never hires the gm role. Checked before anything is saved
		// (DeckRouter.swift: the same refusal before the hire).
		if status, text, refused := gmHireRefusal(gmRole, r.caller, readRegistry(registryPath(), false).gmIsHired()); refused {
			writeStatus(w)(status, text)
			return
		}
		// One at a time, as the Mac claims it: a second click while one runs would read the switches
		// the first turned on as its own "before", and could leave them on when both fail.
		if !d.gmHiring.TryLock() {
			writeStatus(w)(http.StatusConflict, gmBeingHired)
			return
		}
		defer d.gmHiring.Unlock()
		// The manager is switched on before the hire (the GM is a managed hire) and back if the hire
		// fails, as the Mac does (DeckRouter.swift). d.mu is not held across the hire. Hire and fire
		// from the office goes on with it too, as the GM hires its team, and back off the same way.
		// In the folder for your hires, never your home folder: Start here's first step sets it.
		d.mu.Lock()
		if _, ok := existingFolder(d.config.HireFolder); !ok {
			d.mu.Unlock()
			writeStatus(w)(http.StatusConflict, chooseHireFolderFirst)
			return
		}
		before, hiring := d.config.ManagerEnabled, d.config.HiringEnabled
		d.config.ManagerEnabled = true
		d.config.HiringEnabled = true
		d.config.save()
		d.changed()
		d.mu.Unlock()
		// The Claude account named (with one account it is left out) and a worktree of its own (worktree=1),
		// passed on as the Mac's (DeckRouter.swift).
		worktree := query.Get("worktree") == "1"
		status, text := d.hireWith(hireSpec{roleID: gmRole, fromSetup: true, account: strings.Trim(query.Get("account"), " \t"), worktree: &worktree})
		if status >= http.StatusBadRequest {
			d.mu.Lock()
			d.config.ManagerEnabled = before
			d.config.HiringEnabled = hiring
			d.config.save()
			d.changed()
			d.mu.Unlock()
		}
		writeStatus(w)(status, text)

	// routes.json: POST /setup/gm/hire is an action for gm and page (see gmhire.go).
	case r.Method == http.MethodPost && path == "/setup/gm/hire":
		d.gmHire(w, r)

	// The front desk. routes.json: GET /setup/gm/team is read for gm, page and tool.
	case r.Method == http.MethodGet && path == "/setup/gm/team":
		rt.team(w)

	// Who waits for you, read only (routes.json: read for gm, page and tool). A prompt
	// is answered by the owner's tap alone: no method but GET is served here.
	case path == "/setup/gm/approvals":
		rt.approvals(w, r)

	// routes.json: POST /setup/gm/registry is an action for gm and page, never a
	// tool. The table has let in only those two by here; this keeps it so.
	case r.Method == http.MethodPost && path == "/setup/gm/registry":
		if r.caller != callerPage && r.caller != callerGM {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		hire, refused := editHire(registryPath(), body)
		if refused != nil {
			writeStatus(w)(refused.status, refused.message)
			return
		}
		// A GM whose row the owner set to failed or fired loses its token, as a fired one does: its scope
		// follows the row (gmscope.go), so a working token with no open row would be unscoped (RX1d N1).
		if hire.Role == gmRole && !isOpen(hire) {
			d.clearFiredGMToken()
		}
		writeJSON(w, http.StatusOK, hire)

	// routes.json: POST /setup/gm/nickname is an action for gm and page, never a tool or a hire.
	case r.Method == http.MethodPost && path == "/setup/gm/nickname":
		if r.caller != callerPage && r.caller != callerGM {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		d.mu.Lock()
		var places []namePlace
		for _, s := range d.sessions.All() {
			places = append(places, namePlace{project: s.Project, folder: s.Folder})
		}
		layout := config.Layout
		layout.Room.Team = maps.Clone(layout.Room.Team)
		project, name, refused := setNickname(&layout, body, places, config.Pins)
		if refused == nil {
			config.Layout = layout
			config.save()
			d.changed()
		}
		d.mu.Unlock()
		if refused != nil {
			writeStatus(w)(refused.status, refused.message)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"project": project, "nickname": name})

	// Mail (routes.json): send is for gm, page and hire; the outbox is read by gm, page and tool, and a tool is not given the texts.
	case r.Method == http.MethodPost && path == "/setup/gm/send":
		rt.mailSend(w, r)

	// Fire and brief a hire found by its registry id (gmfire.go, gmbrief.go).
	case r.Method == http.MethodPost && path == "/setup/gm/fire":
		rt.gmFire(w, r)

	case r.Method == http.MethodPost && path == "/setup/gm/brief":
		rt.gmBrief(w, r)

	case r.Method == http.MethodGet && path == "/setup/gm/outbox":
		rt.mailOutboxList(w, r)

	// Who may write to whom (routes.json: gm and page).
	case r.Method == http.MethodPost && path == "/setup/gm/contacts":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		pairs, refused := setContacts(registryPath(), body)
		if refused != nil {
			writeStatus(w)(refused.status, refused.message)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contacts": pairs})

	// What the GM asks the app to watch: firings come to its inbox, the event log (watchers.go).
	// The table has let in only gm and page for the POSTs; the list is read for tool too.
	case r.Method == http.MethodGet && path == "/setup/gm/watches",
		r.Method == http.MethodPost && path == "/setup/gm/watch",
		(r.Method == http.MethodPost || r.Method == http.MethodDelete) && path == "/setup/gm/unwatch":
		rt.watchRoutes(w, r, path)

	// The Mac answers an unknown route under /setup with an empty 404.
	default:
		w.WriteHeader(http.StatusNotFound)
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
	writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))
}

// Version is the build's version, set by the command before it starts
// anything, and shown on the setup page.
var Version = "dev"

// setupState is the page's view, with the pairing code. It defaults to the page
// on purpose, for the app's own code and the tests: a request handler must pass
// its caller to setupStateFor, or a tool would be handed the code.
func (rt *router) setupState() map[string]any { return rt.setupStateFor(callerPage) }

// setupStateFor is the state a caller may see: a tool sees the pairing
// address and whether pairing is open, never the code.
func (rt *router) setupStateFor(who caller) map[string]any {
	d := rt.deck
	found := foundAgents()
	agentList := make([]map[string]any, len(found))
	for i, agent := range found {
		agentList[i] = map[string]any{"id": agent, "name": agent.Name(), "connected": isConnected(agent)}
	}
	connected := allConnected()
	openAtLogin := d.openAtLogin()
	gmHired := readRegistry(registryPath(), false).gmIsHired()
	// Under the Mac app, what only it can say of this Mac (hostsetup.go), and what Tailscale last said of it,
	// for the phone mic (hostphonemic.go).
	facts := d.macFactsNow()
	var tailnetFound tailnetAnswer
	if d.macHost() != nil {
		tailnetFound = d.tailnetNow()
	}
	// The API-key account for a hire: the Mac app's Keychain, or this computer's secret store (hireapikey.go).
	apiKey := d.apiKeyOffer()
	// The account folders are looked in without the lock: one that doesn't answer must not stop the deck.
	d.mu.Lock()
	savedAccounts := slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	claudeAccounts := claudeAccountsState(savedAccounts)

	d.mu.Lock()
	defer d.mu.Unlock()
	keys, _ := d.layout()
	keyOf := map[string]int{}
	for i, key := range keys {
		if key.session != nil {
			keyOf[key.session.ID] = i
		}
	}
	pairing := map[string]any{"address": phoneAddress(), "tailnet": phoneTailnet(), "open": false, "codeExpiresIn": 0}
	if code := d.access.ActiveCode(); code != "" {
		pairing["open"] = true
		pairing["codeExpiresIn"] = int(d.access.CodeExpiresIn().Seconds())
		if who == callerPage {
			pairing["code"] = code
		}
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
			// What the setup page's outfit maker dresses them by: their agent, model and role
			"agent": s.Agent, "model": nilIfEmpty(s.Model), "role": roleID(d.roleOf(s)),
		})
	}
	_, tmuxHere := hireWays()
	state := map[string]any{
		"version": Version,
		// Which computer this is, so the page shows its own kind of folder path.
		"platform": runtime.GOOS,
		// Mail to a hire is beta on Windows, where it is typed through the launcher.
		"mailBeta": runtime.GOOS == "windows",
		// No accessibility step: nothing here needs that permission.
		"checklist": map[string]any{"hooks": connected, "scroll": d.config.ScrollEnabled, "openAtLogin": openAtLogin},
		"agents":    agentList,
		// Whether a General Manager is hired, whatever the manager switch says: Start here looks no further then.
		"gmHired": gmHired,
		// The Claude accounts besides ~/.claude, for the account pickers and the accounts step.
		"claudeAccounts": claudeAccounts,
		// The manager, as the Mac's setup state has it: under the Mac app with a GM started by hand and its
		// token file's path (gmbyhand.go), elsewhere with neither. Never the token.
		"manager": d.managerState(),
		// Where hires open: the terminals here that can open them, and tmux if it's there.
		"hiring": d.hireState(apiKey),
		// The roles, those that come with the app to go back to, and what a
		// role can be: its agent, whether that agent has an effort and takes a
		// first message, its badge, and how long each part can be.
		"roles": map[string]any{
			"roles": d.config.Roles, "shipped": shippedWithBriefs(), "agents": roleAgents(), "efforts": effortLevels, "badges": roleBadges,
			"limits": map[string]int{"name": roleNameLimit, "model": roleModelLimit, "instructions": roleInstructionsLimit,
				"questions": roleQuestionsMost, "question": roleQuestionLimit, "briefFile": roleBriefFileLimit,
				"briefFileName": roleBriefFileNameLimit, "answer": briefAnswerLimit},
		},
		"music":      map[string]any{"enabled": d.config.MusicEnabled},
		"maxConfirm": map[string]any{"enabled": d.config.MaxConfirm},
		// The first-run guide's arrows: shown until it's finished or skipped (see guide.js).
		"guide": map[string]any{"done": d.config.GuideDone},
		// The two plain choices (T796), and the T3 Code gate as T3 Code's own code reads it.
		"choices":       choicesState(d.config, tmuxHere, t3InstalledRecently),
		"t3CodeEnabled": t3CodeAllowed(d.config),
		// Without a window of its own, setup says why there's no switch; under the Mac app, its own window.
		"officeWindow": d.officeWindowState(),
		// The keys that show and hide it from anywhere: the Mac app's, kept here as set.
		"officeKey": officeKeyState(d.config.OfficeKey, nil, false),
		"summary":   d.summaryState(),
		"webhook": map[string]any{
			"enabled": d.config.Webhook.Enabled, "url": d.config.Webhook.URL, "secret": webhookSecretFor(who, d.config.Webhook.Secret),
			"outside": d.config.Webhook.Outside, "events": d.config.Webhook.Events, "last": d.lastDelivery(),
		},
		"pairing": pairing,
		"phones":  phones,
		"logo":    d.config.Logo,
		"pins":    pins,
		// How many keys the phone shows now.
		"phoneKeys": len(keys),
		"sessions":  sessions,
		"theme":     themeEditor(d.config),
		"layout":    d.config.Layout,
		// The building's floors, the ground floor first, and the styles a floor above it can have.
		"floors": checkedFloors(d.config.Floors),
		// The folders whose sessions sit on a floor of their own.
		"floorRules": checkedFloorRules(d.config.FloorRules),
		// The cloud bots, for setup's list, each pinned or not and on its floor; those at work with no room for
		// them; and the floor new bots go to. Nothing here looks in the Grok Bot app, the Mac's alone, so none are
		// found and none can be hidden from there.
		"cloudBots": map[string]any{"detects": false, "bots": cloudBotsForSetup(d.config.CloudBots), "hidden": []any{},
			"noRoom": d.botsWithNoDesk(), "botFloor": d.config.BotFloor},
		"presets": floorStyles(),
		// Saved looks, besides Default, the one the deck was last switched
		// to, and whether it has changed since.
		// A copy: the reply is written after the lock is let go.
		"profiles":      slices.Clone(d.config.Profiles),
		"activeProfile": d.config.ActiveProfile,
		"profileEdited": d.config.profileEdited(),
	}
	// What to type to start an agent through the launcher. Not under the Mac app, even while it doesn't answer:
	// the page takes "launcher" for a Linux or Windows computer and words everything for one (its Keychain
	// becomes a secret store, a terminal hire a launcher one), and the Swift engine never sent it.
	if d.mac.host == nil {
		state["launcher"] = launchCommand()
	}
	d.addMacSetup(state, facts)
	// Phone microphone, and this Mac's tailnet address if Tailscale says it: the Mac's alone, as its setup has it.
	if d.mac.host != nil {
		state["phoneMic"] = d.phoneMicState(tailnetFound)
	}
	return state
}

// MARK: Helpers

// writeQR answers with a QR code of text.
func writeQR(w http.ResponseWriter, text string) {
	image, err := qr.Encode(text, qr.M)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	image.Scale = 8
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(image.PNG())
}

// serveFile serves a page no other page may show in a frame, where it could
// lay something over it and trick you into pressing a button.
func serveFile(w http.ResponseWriter, name string) {
	serveFramedBy(w, name, "DENY", "'none'")
}

// servePageWithIcon serves a page as serveFile does, with its Home Screen
// icon in it (see withIcon), and in other words with worded, if any.
func servePageWithIcon(w http.ResponseWriter, name, icon string, worded func([]byte) []byte) {
	data, err := fs.ReadFile(agentdeck.Files, name)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if worded != nil {
		data = worded(data)
	}
	png, _ := fs.ReadFile(agentdeck.Files, icon)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	_, _ = w.Write(withIcon(data, png))
}

// withIcon puts the page's Home Screen icon in it, as a data: URL of the
// logo's 180 px PNG. iOS fetches a linked touch icon itself, outside the page
// and without its browser, and where that fetch fails the Home Screen shows a
// letter instead of the logo.
func withIcon(page, png []byte) []byte {
	if len(png) == 0 {
		return page
	}
	return bytes.Replace(page, []byte(`<link rel="apple-touch-icon" href="/icon.png">`),
		[]byte(`<link rel="apple-touch-icon" sizes="180x180" href="data:image/png;base64,`+base64.StdEncoding.EncodeToString(png)+`">`), 1)
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

// openHireWindow is "Open a window" on hire, from Setup's team list or the
// office's ⧉ window, as the Mac's Deck.openHireWindow: one window of the hires'
// terminal attached to the hire's own tmux session, recorded as this app's (on a
// Mac, through the app). 204, or 409 and why not.
func (d *Deck) openHireWindow(hire Hire) (int, string) {
	// A test copy opens no window of the owner's terminal app (RTCH N1).
	if why := testCopyKeepsAway(testCopyKeepsTerminal); why != "" {
		return http.StatusConflict, why
	}
	if hire.State == "fired" || hire.State == "failed" {
		return http.StatusConflict, "That hire isn't on the team."
	}
	if hire.TmuxSocket == "" {
		return http.StatusConflict, "This hire runs in a window of its own, not in tmux."
	}
	terminals, _ := hireWays()
	d.mu.Lock()
	terminal, hasTerminal := hireTerminal(d.config.HireTerminal, terminals)
	d.mu.Unlock()
	if !hasTerminal {
		return http.StatusConflict, "There's no terminal here to open a window in: tmux attach shows it."
	}
	if err := openHireViewer(hire, terminal); err != nil {
		return http.StatusConflict, "Its window didn't open: the hire may have ended."
	}
	return http.StatusNoContent, ""
}

// writeAnswer answers with a status and JSON, or, for text, as writeStatus.
func writeAnswer(w http.ResponseWriter) func(int, any) {
	return func(status int, answer any) {
		if text, isText := answer.(string); isText {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	}
}

// writeStatus answers with a status and, unless it's empty, a message the
// phone shows.
func writeStatus(w http.ResponseWriter) func(int, string) {
	return func(status int, message string) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, inMacWords(message))
	}
}

// macWords are the phone's sentences that name this computer, or say how the phone reaches it, as the Mac
// app's Swift engine says them (Deck.swift, DeckRouter.swift).
var macWords = map[string]string{
	pairCodeRefusal:     "That code isn't valid. Get a new one on your Mac.",
	settingsElsewhere:   "Settings open on your Mac.",
	hireWindowElsewhere: "Hires' windows open on your Mac.",
	backgroundNeedsTmux: "Running hires in the background needs tmux, which isn't on this Mac.",
	t3Elsewhere:         "T3 Code opens on your Mac.",
	hiringOff:           "Hiring and firing are off. Turn them on in setup, on your Mac.",
	scrollingOff:        "Scrolling is off. Turn it on in setup, on your Mac.",
	buttonTypesNothing:  "This button types nothing yet. Set it up in setup, on your Mac.",
	hireFolderFirst:     "Choose a folder for new hires in setup, on your Mac.",
	musicOff:            "The jukebox can't play your music: turn it on in setup, on your Mac.",
	phoneTextOff:        "The phone mic is off. Turn it on in setup, on your Mac.",
	peekTailnet:         "The input line shows only over Tailscale's HTTPS address.",
	choiceTailnet:       "The details show only over Tailscale's HTTPS address.",
}

// inMacWords is message as the Mac app says it, under the Mac app; elsewhere as it is.
func inMacWords(message string) string {
	if mac, ok := macWords[message]; ok && host != nil {
		return mac
	}
	return message
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

// nilIfEmpty is text, or nil for none, as JSON's null.
func nilIfEmpty(text string) any {
	if text == "" {
		return nil
	}
	return text
}

// roleID is the id of a role as the phone is told of it, or nil.
func roleID(role map[string]any) any {
	if role == nil {
		return nil
	}
	return role["id"]
}
