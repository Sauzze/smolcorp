package desktop

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// The portable office (beta), as the Mac app's Viewer.swift: the office,
// seen on a phone from anywhere, view only. While it's on, a redacted
// snapshot of the office is locked with a key only this computer and the
// link know, and put on the viewer you run on your own Vercel, which never
// sees the key. Off at every start; nothing is snapshotted or sent until you
// turn it on. The contract with the viewer page, v1, is in viewer/README.md.

// MARK: Settings

// ViewerSettings are the viewer's settings in config.json: its address and
// the options. No secret is ever kept here: the id, the key and the write
// secret are in the system's secret store.
type ViewerSettings struct {
	// The viewer's address, as Vercel gives it: https://office-viewer-xxxx.vercel.app.
	BaseURL string `json:"baseURL"`
	// Desks shown as "Desk 1", "Desk 2"… rather than by their project's name.
	HideNames bool `json:"hideNames"`
	// Turn it off when you're back at the computer.
	StopOnReturn bool `json:"stopOnReturn"`
	// Turn it off after this many hours; 0 is never.
	StopAfterHours int `json:"stopAfterHours"`
}

var viewerHourChoices = []int{2, 4, 8, 12, 0}

func standardViewerSettings() ViewerSettings {
	return ViewerSettings{StopOnReturn: true, StopAfterHours: 8}
}

// UnmarshalJSON reads the settings field by field, so a file from another
// version still loads.
func (v *ViewerSettings) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	*v = standardViewerSettings()
	json.Unmarshal(raw["baseURL"], &v.BaseURL)
	json.Unmarshal(raw["hideNames"], &v.HideNames)
	json.Unmarshal(raw["stopOnReturn"], &v.StopOnReturn)
	var hours int
	if json.Unmarshal(raw["stopAfterHours"], &hours) == nil && slices.Contains(viewerHourChoices, hours) {
		v.StopAfterHours = hours
	}
	return nil
}

const viewerAddressProblem = "That isn't a viewer address. Paste the https:// address Vercel gives your project."

// viewerOrigin is the viewer's origin, scheme://host[:port], or false if
// text isn't one it can send to: https only, but for http://localhost and
// 127.0.0.1, for trying it out on this computer. No path, query, fragment or
// password.
func viewerOrigin(text string) (string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(text), "/")
	if strings.ContainsAny(trimmed, "?#") {
		return "", false
	}
	parts, err := url.Parse(trimmed)
	if err != nil || parts.User != nil || parts.Path != "" || parts.Opaque != "" {
		return "", false
	}
	scheme, host := strings.ToLower(parts.Scheme), strings.ToLower(parts.Hostname())
	if host == "" || strings.ContainsFunc(host, func(c rune) bool { return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-') }) {
		return "", false
	}
	if scheme != "https" && !(scheme == "http" && (host == "localhost" || host == "127.0.0.1")) {
		return "", false
	}
	origin := scheme + "://" + host
	if port := parts.Port(); port != "" {
		origin += ":" + port
	}
	return origin, true
}

// MARK: Secrets

// The viewer's id, key and write secret, each an item under one service in
// the system's secret store. Never in config.json, logs or the snapshot.
const (
	viewerService          = "smolcorp-viewer"
	viewerIDAccount        = "id"
	viewerKeyAccount       = "key"
	viewerSecretAccount    = "write-secret"
	viewerKeychainProblem  = "Can't read the viewer's key from your computer's secret store"
	viewerNotSetUpProblem  = "Not set up yet"
	viewerNoStoreProblem   = "There's no secret store on this computer to keep the viewer's keys in."
	viewerQuitWait         = 5 * time.Second
	viewerRequestTimeout   = 10 * time.Second
	viewerDebounce         = 5 * time.Second
	viewerHeartbeat        = 60 * time.Second
	viewerReturnGrace      = 2 * time.Minute
	viewerPresenceInterval = 5 * time.Second
	// Away for this long with no keyboard or mouse, and the next use is coming back.
	viewerAwayAfter = 10 * time.Minute
)

var viewerBackoff = []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}

// viewerSecrets is where the viewer's id, key and write secret are kept, by
// account. errNotFound from get is surely none; any other error isn't taken
// for none.
type viewerSecrets struct {
	get    func(account string) (string, error)
	set    func(account, value string) error
	delete func(account string)
	// notFound is whether an error from get says there's surely none.
	notFound func(error) bool
}

// MARK: Crypto

// The body the viewer stores and the phone opens: [0x01] + nonce + ciphertext
// + tag, AES-256-GCM over the padded JSON, bound to the viewer's id.
const (
	viewerVersion byte = 0x01
	// The plaintext is padded with spaces to a multiple of this, so small changes don't show in its size.
	viewerBlock = 1024
	// The most a padded plaintext may be.
	viewerMost = 192 * 1024
)

func viewerAAD(id string) []byte { return []byte("smolcorp-viewer:v1:" + id) }

// viewerPadded is json with trailing spaces to the next multiple of
// viewerBlock, or false over viewerMost.
func viewerPadded(data []byte) ([]byte, bool) {
	size := max(viewerBlock, (len(data)+viewerBlock-1)/viewerBlock*viewerBlock)
	if size > viewerMost {
		return nil, false
	}
	return append(slices.Clone(data), []byte(strings.Repeat(" ", size-len(data)))...), true
}

// viewerPlaintext is {"at":…,"office":…,"v":1}, padded; false when it's too big.
func viewerPlaintext(office map[string]any, at int64) ([]byte, bool) {
	data, err := json.Marshal(map[string]any{"v": 1, "at": at, "office": office})
	if err != nil {
		return nil, false
	}
	return viewerPadded(data)
}

// viewerSeal locks plaintext with key for id, with a fresh random nonce
// unless one is given (the test vector's).
func viewerSeal(plaintext, key []byte, id string, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("the key isn't 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if nonce == nil {
		nonce = make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return nil, err
		}
	}
	return gcm.Seal(append([]byte{viewerVersion}, nonce...), nonce, plaintext, viewerAAD(id)), nil
}

func base64URL(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

// viewerID is the only shape of id: 22 characters of base64url, 16 bytes.
var viewerID = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

// viewerKeyOf is a key as kept, 32 bytes in base64url, or nil.
func viewerKeyOf(text string) []byte {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(text, "="))
	if err != nil || len(key) != 32 {
		return nil
	}
	return key
}

// MARK: Snapshot

// With names hidden, what can still carry your own text goes as a generic
// value: an error kind the office doesn't know (it's an agent hook's raw
// text), a subagent type that isn't one of Claude Code's own (custom agents
// are named by you), and a role you made or renamed.
var (
	viewerErrorKinds = []string{"rate_limit", "overloaded", "billing_error", "authentication_failed", "oauth_org_not_allowed",
		"account_on_hold", "cloud_credential_error"}
	viewerSubagentTypes = []string{"general-purpose", "Explore", "Plan", "statusline-setup", "output-style-setup", "claude-code-guide"}
)

func shownErrorKind(kind string) string {
	if slices.Contains(viewerErrorKinds, kind) {
		return kind
	}
	return "error"
}

func shownSubagentType(kind string) string {
	if slices.Contains(viewerSubagentTypes, kind) {
		return kind
	}
	return "agent"
}

// shownRole is a role with names hidden: the badge and outfit still draw;
// the id and name only as shipped.
func shownRole(role Role) map[string]any {
	shown := map[string]any{"id": "role", "name": "", "badge": role.Badge, "outfit": role.Outfit}
	if i := slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == role.ID }); i >= 0 {
		shown["id"] = role.ID
		if shippedRoles[i].Name == role.Name {
			shown["name"] = role.Name
		}
	}
	return shown
}

// viewerSnapshot is what the viewer draws, built field by field from what's
// allowed (spec §3), never by taking the phone's snapshot and removing what
// isn't: the same shape as snapshot, so office.js draws it unchanged. No
// session ids, paths, activity, models or buttons. With the lock held.
func (d *Deck) viewerSnapshot(hideNames bool, alias func(string) string) map[string]any {
	keys, overflow := d.layout()
	// A desk's name, when names are hidden, by the session's folder (what office.js finds the team by).
	deskOfName := map[string]string{}
	entries := make([]map[string]any, len(keys))
	for i, key := range keys {
		deskName := fmt.Sprintf("Desk %d", i+1)
		entry := map[string]any{"pin": nil}
		if key.pin != nil && !hideNames {
			entry["pin"] = lastPathComponent(*key.pin)
		}
		if s := key.session; s != nil {
			if _, ok := deskOfName[s.Name()]; !ok {
				deskOfName[s.Name()] = map[bool]string{true: deskName, false: s.Name()}[hideNames]
			}
			effort := s.Effort
			if s.PendingEffort != "" {
				effort = s.PendingEffort
			}
			subagents := []map[string]any{}
			for _, subagent := range s.Subagents {
				kind := subagent.Type
				if hideNames {
					kind = shownSubagentType(kind)
				}
				subagents = append(subagents, map[string]any{"id": alias(subagent.ID), "type": kind, "model": nil})
			}
			var errorKind any
			if s.ErrorKind != "" {
				errorKind = s.ErrorKind
				if hideNames {
					errorKind = shownErrorKind(s.ErrorKind)
				}
			}
			var role any
			if found, ok := d.sessionRole(s); ok && hideNames {
				role = shownRole(found)
			} else if ok {
				role = found.forPhone()
			}
			name, project := s.Name(), s.ProjectName()
			if hideNames {
				name, project = deskName, deskName
			}
			entry["session"] = map[string]any{
				"id": alias(s.ID), "name": name, "project": project, "state": s.State, "effort": effort,
				"pending": s.PendingEffort != "", "agent": s.Agent, "agentName": s.Agent.Name(), "compacting": s.Compacting,
				"turnStartedAt": millisOrNull(s.TurnStartedAt), "errorKind": errorKind, "subagents": subagents, "role": role,
				// Nothing to press
				"dial": false, "mic": false, "dictates": false, "keys": false, "fire": false, "summary": false,
			}
		}
		entries[i] = entry
	}

	// The room, with only the team members at a desk now, under the desk's name when names are hidden, and
	// no dress rules: they match model names, which never leave. Of the room, only the team (its projects
	// and nicknames) and the dress rules hold your own text: every other text in it is one of a fixed list
	// of choices, checked as the room is read.
	var layout map[string]any
	data, _ := json.Marshal(d.config.Layout)
	json.Unmarshal(data, &layout)
	room, _ := layout["room"].(map[string]any)
	if room == nil {
		room = map[string]any{}
	}
	rawTeam, _ := room["team"].(map[string]any)
	shownTeam := map[string]any{}
	for name, shown := range deskOfName {
		member, ok := rawTeam[name].(map[string]any)
		if !ok {
			continue
		}
		if hideNames {
			member["name"] = ""
		}
		shownTeam[shown] = member
	}
	room["team"] = shownTeam
	if dress, ok := room["dress"].(map[string]any); ok {
		dress["rules"] = []any{}
	}
	arranged := map[string]any{"room": room, "rows": []any{}}
	for _, field := range []string{"keys", "office", "dial", "stick"} {
		arranged[field] = layout[field]
	}

	var music any
	if d.config.MusicEnabled {
		music = map[string]any{"playing": d.playing}
	}
	return map[string]any{
		"keys": entries, "overflow": len(overflow), "now": int64(nowMillis()), "theme": currentTheme(d.config),
		"scroll": false, "dictation": nil, "hiring": map[string]any{"fire": false, "hire": false, "roles": []any{}},
		"music": music, "layout": arranged, "upstairs": d.viewerUpstairs(keys, hideNames),
	}
}

// viewerUpstairs is the second floor's headless runs, as the viewer may see
// them: each desk's name, under the same rule as the desks downstairs, and
// its state, and the name of whoever hired it, where hiredBy knows: with
// names hidden, the number of the hiring desk, as that desk shows it, or
// "Downstairs". Nothing else of them leaves this computer. With the lock held.
func (d *Deck) viewerUpstairs(keys []deckKey, hideNames bool) map[string]any {
	desks := []map[string]any{}
	overflow := 0
	for _, s := range d.sessions.All() {
		if !s.Headless() {
			continue
		}
		if len(desks) == mostUpstairs {
			overflow++
			continue
		}
		desk := map[string]any{"name": s.Name(), "state": s.State}
		if hideNames {
			desk["name"] = fmt.Sprintf("Upstairs %d", len(desks)+1)
		}
		if parent := d.hiredBy(s); parent != nil {
			name := parent["name"]
			if hideNames {
				name = "Downstairs"
				if id, ok := parent["id"].(string); ok {
					if i := slices.IndexFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == id }); i >= 0 {
						name = fmt.Sprintf("Desk %d", i+1)
					}
				}
			}
			desk["hiredBy"] = map[string]any{"name": name}
		}
		desks = append(desks, desk)
	}
	return map[string]any{"desks": desks, "overflow": overflow}
}

// MARK: The portable office

// portableOffice is turning it on and off, and everything while it's on: the
// uploads (at most one every 5 s after a change, one in flight, a heartbeat
// every 60 s, backing off when the viewer can't be reached), stopping when
// you're back or after some hours, and the setup page's actions.
//
// Its own lock is taken after the deck's, never before: the deck tells it of
// each change with the deck's held, and it takes the deck's for a snapshot
// with its own let go.
type portableOffice struct {
	mu       sync.Mutex
	secrets  viewerSecrets
	settings ViewerSettings
	// The office as the viewer may see it: takes the deck's lock.
	snapshot func(hideNames bool, alias func(string) string) map[string]any
	// Called when it turns on or off or an upload goes or fails, for the tray.
	onChange func()
	client   *http.Client
	presence func() (idle time.Duration, locked, ok bool)

	on                         bool
	since, lastSent, lastTried time.Time
	problem, checked           string
	linkRevision               int
	aliases                    map[string]string
	creds                      *viewerCredentials
	run                        int // changes at each start and stop, so a reply from before is ignored
	inFlight, pending          bool
	failures                   int
	debounce, retry, grace     *time.Timer
	stopTimers                 chan struct{}
	secretKnown                *bool
	putsOut                    map[string]int
	forgotten                  map[string]viewerCredentials
	// Every PUT and DELETE still out, so a quit can wait for them.
	outstanding requestCount
}

// requestCount counts requests still out.
type requestCount struct {
	mu sync.Mutex
	n  int
}

func (c *requestCount) Add(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += n
}

func (c *requestCount) Done() { c.Add(-1) }

// waitAtMost waits for none to be out, for as long as wait at most.
func (c *requestCount) waitAtMost(wait time.Duration) {
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		c.mu.Lock()
		n := c.n
		c.mu.Unlock()
		if n <= 0 {
			return
		}
	}
}

// viewerCredentials are read once as it starts, so the secret store isn't
// asked at every upload.
type viewerCredentials struct {
	base, id string
	key      []byte
	secret   string
}

func newPortableOffice(secrets viewerSecrets, settings ViewerSettings, snapshot func(bool, func(string) string) map[string]any) *portableOffice {
	return &portableOffice{
		secrets: secrets, settings: settings, snapshot: snapshot, onChange: func() {}, presence: presence,
		// A redirect isn't followed: it could take the write secret elsewhere.
		client: &http.Client{Timeout: viewerRequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		aliases: map[string]string{}, putsOut: map[string]int{}, forgotten: map[string]viewerCredentials{},
	}
}

// MARK: State

// setSettings takes the settings as setup keeps them.
func (p *portableOffice) setSettings(settings ViewerSettings) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settings = settings
}

func (p *portableOffice) origin() (string, bool) { return viewerOrigin(p.settings.BaseURL) }

// hasWriteSecret, with the lock held.
func (p *portableOffice) hasWriteSecret() bool {
	if p.secretKnown == nil {
		_, err := p.secrets.get(viewerSecretAccount)
		known := err == nil
		p.secretKnown = &known
	}
	return *p.secretKnown
}

// isSetUp is a viewer address and a write secret: enough to turn it on. With the lock held.
func (p *portableOffice) isSetUp() bool {
	_, ok := p.origin()
	return ok && p.hasWriteSecret()
}

// link is the link to open on the phone, with the key after #k=, which
// browsers never send. With the lock held.
func (p *portableOffice) link() string {
	if c := p.creds; c != nil {
		return c.base + "/v/" + c.id + "#k=" + base64URL(c.key)
	}
	base, ok := p.origin()
	if !ok {
		return ""
	}
	id, errID := p.secrets.get(viewerIDAccount)
	key, errKey := p.secrets.get(viewerKeyAccount)
	if errID != nil || errKey != nil || !viewerID.MatchString(id) || viewerKeyOf(key) == nil {
		return ""
	}
	return base + "/v/" + id + "#k=" + key
}

// Link is the link with its key, for the setup page's QR code.
func (p *portableOffice) Link() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.link()
}

// IsOn and IsSetUp are for the tray.
func (p *portableOffice) IsOn() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.on
}

func (p *portableOffice) IsSetUp() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isSetUp()
}

// Status is the tray's line after "Portable office: ON", and a problem while it's off.
func (p *portableOffice) Status() (status, problem string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.problem != "":
		status = strings.ToLower(p.problem[:1]) + p.problem[1:]
	case !p.lastSent.IsZero():
		status = "last sent " + viewerAgo(p.lastSent)
	case !p.since.IsZero():
		status = "since " + p.since.Format("15:04")
	default:
		status = "starting"
	}
	return status, p.problem
}

func viewerAgo(at time.Time) string {
	seconds := max(0, int(time.Since(at).Seconds()))
	switch {
	case seconds < 60:
		return fmt.Sprintf("%d s ago", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%d min ago", seconds/60)
	}
	return fmt.Sprintf("%d h ago", seconds/3600)
}

// SetupState is for the setup page. The link (which holds the key) is the
// only secret in it, and only in the reply to GET /setup/viewer (spec §7):
// every other reply says whether there is one, and the page asks for it.
func (p *portableOffice) SetupState(withLink bool) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	millis := func(at time.Time) any {
		if at.IsZero() {
			return nil
		}
		return at.UnixMilli()
	}
	current := ""
	if p.isSetUp() {
		current = p.link()
	}
	var link any
	if withLink && current != "" {
		link = current
	}
	return map[string]any{
		"baseURL": p.settings.BaseURL, "hasSecret": p.hasWriteSecret(), "setUp": p.isSetUp(),
		"on": p.on, "since": millis(p.since), "lastSent": millis(p.lastSent),
		"problem": orNull(p.problem), "checked": orNull(p.checked),
		"hasLink": current != "", "link": link, "revision": p.linkRevision,
		"hideNames": p.settings.HideNames, "stopOnReturn": p.settings.StopOnReturn,
		"stopAfterHours": p.settings.StopAfterHours, "hourChoices": viewerHourChoices,
	}
}

// changedNow tells the tray, outside the lock.
func (p *portableOffice) changedNow() { go p.onChange() }

// MARK: On and off

func (p *portableOffice) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.on {
		return
	}
	base, ok := p.origin()
	secret, err := p.secrets.get(viewerSecretAccount)
	if !ok || err != nil {
		p.problem = viewerNotSetUpProblem
		p.changedNow()
		return
	}
	id, key, ok := p.identity()
	if !ok {
		// identity says why when the key is there but unreadable
		if p.problem != viewerKeychainProblem {
			p.problem = "Couldn't keep the viewer's key in your computer's secret store"
		}
		p.changedNow()
		return
	}
	p.creds = &viewerCredentials{base: base, id: id, key: key, secret: secret}
	delete(p.forgotten, id)
	p.run++
	p.on, p.since, p.lastSent, p.problem, p.failures, p.inFlight, p.pending = true, time.Now(), time.Time{}, "", 0, false, false
	p.stopTimers = make(chan struct{})
	go p.beat(p.stopTimers)
	go p.watchForReturn(p.stopTimers, p.since)
	go p.upload()
	p.changedNow()
}

// Stop stops sending and asks the viewer to forget the office. It's off
// even if that fails: the viewer forgets it by itself after 10 minutes. wait
// is how long to wait for the viewer's reply, on quit.
func (p *portableOffice) Stop(wait time.Duration) {
	p.mu.Lock()
	if !p.on {
		p.mu.Unlock()
		return
	}
	c := p.creds
	p.on = false
	p.run++
	p.creds, p.since, p.problem, p.inFlight, p.pending = nil, time.Time{}, "", false, false
	for _, timer := range []*time.Timer{p.debounce, p.retry, p.grace} {
		if timer != nil {
			timer.Stop()
		}
	}
	p.debounce, p.retry, p.grace = nil, nil, nil
	close(p.stopTimers)
	stoppedRun := p.run
	p.forget(c.id, *c, func(status int) {
		if viewerSuccess(status) {
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.run == stoppedRun {
			p.problem = "Stopped, but can't reach the viewer: it forgets the office within 10 minutes"
			p.changedNow()
		}
	})
	p.changedNow()
	p.mu.Unlock()
	// On quit, also wait for an upload still out and the DELETE sent after it.
	if wait > 0 {
		p.outstanding.waitAtMost(wait)
	}
}

// forget asks the viewer to forget id, and again after each PUT to it that's
// still out: a PUT already sent can reach the viewer after a DELETE and
// bring the office back. With the lock held.
func (p *portableOffice) forget(id string, c viewerCredentials, done func(status int)) {
	if p.putsOut[id] > 0 {
		p.forgotten[id] = c
	}
	p.outstanding.Add(1)
	go func() {
		defer p.outstanding.Done()
		done(p.send(http.MethodDelete, c.base+"/api/s/"+id, c.secret, nil))
	}()
}

// putFinished is a PUT to id done, or given up. With the lock held.
func (p *portableOffice) putFinished(id string) {
	left := p.putsOut[id] - 1
	if left > 0 {
		p.putsOut[id] = left
	} else {
		delete(p.putsOut, id)
	}
	again, ok := p.forgotten[id]
	if left <= 0 {
		delete(p.forgotten, id)
	}
	if ok {
		p.outstanding.Add(1)
		go func() {
			defer p.outstanding.Done()
			p.send(http.MethodDelete, again.base+"/api/s/"+id, again.secret, nil)
		}()
	}
}

// Changed is the deck changing: send it within 5 s, the newest each time.
func (p *portableOffice) Changed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.on {
		return
	}
	p.pending = true
	if p.debounce == nil && p.retry == nil && !p.inFlight {
		p.scheduleUpload()
	}
}

// scheduleUpload, with the lock held.
func (p *portableOffice) scheduleUpload() {
	delay := viewerDebounce
	if !p.lastTried.IsZero() {
		delay = viewerDebounce - time.Since(p.lastTried)
	}
	delay = max(time.Second, delay)
	p.debounce = time.AfterFunc(delay, func() {
		p.mu.Lock()
		p.debounce = nil
		p.mu.Unlock()
		p.upload()
	})
}

// beat is the heartbeat, every 60 s while it's on, and the check for the
// hours it may stay on.
func (p *portableOffice) beat(stop <-chan struct{}) {
	ticker := time.NewTicker(viewerHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if p.stopIfOverdue() {
			return
		}
		p.mu.Lock()
		due := p.on && !p.inFlight && p.retry == nil && p.debounce == nil && time.Since(p.lastTried) >= viewerHeartbeat-5*time.Second
		p.mu.Unlock()
		if due {
			p.upload()
		}
	}
}

// stopIfOverdue turns it off after the hours set.
func (p *portableOffice) stopIfOverdue() bool {
	p.mu.Lock()
	hours, since, on := p.settings.StopAfterHours, p.since, p.on
	p.mu.Unlock()
	if !on || hours <= 0 || time.Since(since) < time.Duration(hours)*time.Hour {
		return false
	}
	p.Stop(0)
	return true
}

// alias is each session's and subagent's stand-in id, "v1", "v2"…, for this run.
func (p *portableOffice) alias(id string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if known, ok := p.aliases[id]; ok {
		return known
	}
	made := fmt.Sprintf("v%d", len(p.aliases)+1)
	p.aliases[id] = made
	return made
}

func (p *portableOffice) upload() {
	if p.stopIfOverdue() {
		return
	}
	p.mu.Lock()
	if p.debounce != nil {
		p.debounce.Stop()
		p.debounce = nil
	}
	if !p.on || p.creds == nil {
		p.mu.Unlock()
		return
	}
	if p.inFlight {
		p.pending = true
		p.mu.Unlock()
		return
	}
	p.pending, p.inFlight, p.lastTried = false, true, time.Now()
	c, run, hide := *p.creds, p.run, p.settings.HideNames
	p.mu.Unlock()

	// The deck's lock, with this one let go.
	office := p.snapshot(hide, p.alias)
	plain, ok := viewerPlaintext(office, time.Now().UnixMilli())
	var body []byte
	var problem string
	if !ok {
		problem = "The office is too big to send"
	} else if sealed, err := viewerSeal(plain, c.key, c.id, nil); err != nil {
		problem = "Couldn't lock the snapshot"
	} else {
		body = sealed
	}
	p.mu.Lock()
	if run != p.run || !p.on {
		p.mu.Unlock()
		return
	}
	if problem != "" {
		p.inFlight = false
		p.failed(problem)
		p.mu.Unlock()
		return
	}
	p.putsOut[c.id]++
	p.outstanding.Add(1)
	p.mu.Unlock()

	status := p.send(http.MethodPut, c.base+"/api/s/"+c.id, c.secret, body)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.putFinished(c.id)
	p.outstanding.Done()
	if run != p.run || !p.on {
		return
	}
	p.inFlight = false
	if viewerSuccess(status) {
		p.lastSent, p.problem, p.failures = time.Now(), "", 0
		if p.pending {
			p.scheduleUpload()
		}
		p.changedNow()
	} else {
		p.failed(viewerWords(status))
	}
}

// failed: it stays on, and tries again after 5, 10, 30, then every 60 s.
// With the lock held.
func (p *portableOffice) failed(words string) {
	p.problem = words
	p.failures++
	if p.retry != nil {
		p.retry.Stop()
	}
	p.retry = time.AfterFunc(viewerBackoff[min(p.failures-1, len(viewerBackoff)-1)], func() {
		p.mu.Lock()
		p.retry = nil
		p.mu.Unlock()
		p.upload()
	})
	p.changedNow()
}

// MARK: When you're back

// watchForReturn turns it off when you're back at the computer, as the Mac
// app does on an unlock, or on the keyboard or mouse used after a wake.
// Here, where the system says: the screen unlocked after it was locked, or
// the keyboard or mouse used after viewerAwayAfter without, on an unlocked
// screen. See presence.
func (p *portableOffice) watchForReturn(stop <-chan struct{}, started time.Time) {
	ticker := time.NewTicker(viewerPresenceInterval)
	defer ticker.Stop()
	away := false
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		idle, locked, ok := p.presence()
		if !ok {
			continue
		}
		if locked || idle >= viewerAwayAfter {
			away = true
		} else if away && idle < viewerPresenceInterval {
			away = false
			p.cameBack(started, time.Now())
		}
	}
}

// cameBack: back this soon after it was turned on may be launch, then lock,
// so it's looked at again once the grace is over: you're back if the screen
// is unlocked and used since.
func (p *portableOffice) cameBack(started, back time.Time) {
	p.mu.Lock()
	if !p.on || !p.settings.StopOnReturn || !p.since.Equal(started) {
		p.mu.Unlock()
		return
	}
	early := viewerReturnGrace - time.Since(started)
	if early <= 0 {
		p.mu.Unlock()
		p.Stop(0)
		return
	}
	if p.grace != nil {
		p.grace.Stop()
	}
	p.grace = time.AfterFunc(early, func() {
		idle, locked, ok := p.presence()
		if ok && !locked && idle < time.Since(back) && p.IsOn() {
			p.Stop(0)
		}
	})
	p.mu.Unlock()
}

// MARK: Setup

// identity is the viewer's id and key, made the first time: only when
// there's neither. One that's there but can't be read is never overwritten,
// as every link given out would stop working without a word. With the lock held.
func (p *portableOffice) identity() (string, []byte, bool) {
	id, errID := p.secrets.get(viewerIDAccount)
	text, errKey := p.secrets.get(viewerKeyAccount)
	if errID == nil && errKey == nil && viewerID.MatchString(id) && viewerKeyOf(text) != nil {
		return id, viewerKeyOf(text), true
	}
	if !p.secrets.notFound(errID) || !p.secrets.notFound(errKey) {
		p.problem = viewerKeychainProblem
		return "", nil, false
	}
	return p.newIdentity()
}

// newIdentity, with the lock held.
func (p *portableOffice) newIdentity() (string, []byte, bool) {
	idBytes, key := make([]byte, 16), make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return "", nil, false
	}
	if _, err := rand.Read(key); err != nil {
		return "", nil, false
	}
	id := base64URL(idBytes)
	if p.secrets.set(viewerIDAccount, id) != nil || p.secrets.set(viewerKeyAccount, base64URL(key)) != nil {
		return "", nil, false
	}
	p.linkRevision++
	return id, key, true
}

// MakeWriteSecret is a new write secret, kept in the secret store and
// returned this once, to paste into Vercel.
func (p *portableOffice) MakeWriteSecret() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	secret := base64URL(bytes)
	if err := p.secrets.set(viewerSecretAccount, secret); err != nil {
		return "", err
	}
	known := true
	p.secretKnown = &known
	p.identity()
	if p.creds != nil {
		p.creds.secret = secret
	}
	p.checked = ""
	return secret, nil
}

// RotateKey makes a new id and key, so every link given out stops working;
// the old id is forgotten on the viewer.
func (p *portableOffice) RotateKey() {
	p.mu.Lock()
	var old *viewerCredentials
	if p.creds != nil {
		c := *p.creds
		old = &c
	} else if base, ok := p.origin(); ok {
		id, errID := p.secrets.get(viewerIDAccount)
		secret, errSecret := p.secrets.get(viewerSecretAccount)
		if errID == nil && errSecret == nil {
			old = &viewerCredentials{base: base, id: id, secret: secret}
		}
	}
	id, key, ok := p.newIdentity()
	if !ok {
		p.problem = "Couldn't make a new key in your computer's secret store"
		p.changedNow()
		p.mu.Unlock()
		return
	}
	p.checked = ""
	if old != nil && viewerID.MatchString(old.id) {
		rotatedAt := p.linkRevision
		p.forget(old.id, *old, func(status int) {
			if viewerSuccess(status) {
				return
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			// Unless it has been rotated again since
			if p.linkRevision == rotatedAt {
				words := "Rotated, but the viewer couldn't be reached: old links show the last frame for up to 10 minutes"
				p.problem, p.checked = words, words
				p.changedNow()
			}
		})
	}
	resend := p.creds != nil
	if resend {
		p.creds.id, p.creds.key = id, key
		p.run++
		p.inFlight = false
	}
	p.changedNow()
	p.mu.Unlock()
	if resend {
		p.upload()
	}
}

// Wipe has the viewer forget every office it holds. This one is turned off first.
func (p *portableOffice) Wipe() {
	p.Stop(0)
	p.ask(http.MethodDelete, "/api/s", "Viewer wiped ✓")
}

// Check asks the viewer whether it's ready: its settings, its storage and the write secret.
func (p *portableOffice) Check() { p.ask(http.MethodGet, "/api/health", "Viewer ready ✓") }

// ask sends method to path on the viewer, and keeps how it went for setup.
func (p *portableOffice) ask(method, path, good string) {
	p.mu.Lock()
	base, ok := p.origin()
	secret, err := p.secrets.get(viewerSecretAccount)
	if !ok || err != nil {
		p.checked = viewerNotSetUpProblem
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	status := p.send(method, base+path, secret, nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checked = good
	if !viewerSuccess(status) {
		p.checked = viewerWords(status)
	}
}

// MARK: Network

// send gives the reply's status, or 0 if there was none. The body is never read.
func (p *portableOffice) send(method, address, secret string, body []byte) int {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	request, err := http.NewRequest(method, address, reader)
	if err != nil {
		return 0
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("User-Agent", "Smolcorp")
	if body != nil {
		request.Header.Set("Content-Type", "application/octet-stream")
	}
	response, err := p.client.Do(request)
	if err != nil {
		return 0
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
	return response.StatusCode
}

func viewerSuccess(status int) bool { return status >= 200 && status < 300 }

// viewerWords is what a failed reply means, in a few plain words: never its
// body or the address.
func viewerWords(status int) string {
	switch status {
	case 0:
		return "Can't reach the viewer"
	case 401:
		return "The viewer refused the write secret"
	case 503:
		return "The viewer isn't set up on Vercel yet"
	case 404, 405:
		return "No viewer at that address"
	case 413:
		return "The office is too big to send"
	case 429:
		return "The viewer is busy, trying again"
	}
	return fmt.Sprintf("The viewer said no (%d)", status)
}
