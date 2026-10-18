package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Webhook is a webhook for other tools: what the deck's sessions do, sent as
// it happens to an address you give, such as Home Assistant's, so a lamp can
// turn orange when an agent waits on you. Only addresses on your own
// network, unless you allow others. Each message says what happened, to
// which project and agent: never folders, prompts or what the agent wrote.
type Webhook struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	// Sent with each message, for the tool to check it's yours.
	Secret string `json:"secret"`
	// Whether addresses outside your network are allowed.
	Outside bool     `json:"outside"`
	Events  []string `json:"events"`
}

var (
	// What can be sent: a session waiting on you, done, failed, getting to
	// work, starting and ending.
	webhookEvents  = []string{"waiting", "done", "error", "working", "started", "ended"}
	standardEvents = []string{"waiting", "done", "error"}
	// How long a delivery may take.
	webhookTimeout = 3 * time.Second
	// A redirect isn't followed: it could lead off your network, where only
	// the address you gave was checked, and take the secret with it.
	webhookClient = &http.Client{Timeout: webhookTimeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
)

func standardWebhook() Webhook {
	return Webhook{Events: append([]string{}, standardEvents...)}
}

// UnmarshalJSON keeps only events it knows, in their order, so a file from
// another version still loads.
func (w *Webhook) UnmarshalJSON(data []byte) error {
	type plain Webhook
	loaded := plain(standardWebhook())
	loaded.Events = nil
	if err := json.Unmarshal(data, &loaded); err != nil {
		*w = standardWebhook()
		return nil
	}
	given := loaded.Events
	if given == nil {
		given = standardEvents
	}
	loaded.Events = knownEvents(given)
	*w = Webhook(loaded)
	return nil
}

// knownEvents is given's events this deck sends, in their order.
func knownEvents(given []string) []string {
	events := []string{}
	for _, event := range webhookEvents {
		if contains(given, event) {
			events = append(events, event)
		}
	}
	return events
}

// webhookProblem says why address can't be sent to, or "" if it can: a web
// address, and one on your own network unless outside.
func webhookProblem(address string, outside bool) string {
	parsed, err := url.Parse(address)
	if err != nil || (strings.ToLower(parsed.Scheme) != "http" && strings.ToLower(parsed.Scheme) != "https") || parsed.Hostname() == "" {
		return "That isn't a web address. It starts with http:// or https://."
	}
	if !outside && !isLocalHost(parsed.Hostname()) {
		return parsed.Hostname() + ` isn't on your network. Tick "Allow addresses outside your network" to send there.`
	}
	return ""
}

// isLocalHost says whether host is on your own network: this computer, a
// name without dots or ending in .local, or a private address.
func isLocalHost(host string) bool {
	name := strings.ToLower(strings.Trim(host, "[]"))
	if name == "localhost" {
		return true
	}
	for _, suffix := range []string{".local", ".localhost", ".lan", ".home.arpa"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	if ip := net.ParseIP(name); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	return !strings.Contains(name, ".") && !strings.Contains(name, ":")
}

// send posts message, and says how it went: "204", or what went wrong.
func (w Webhook) send(message map[string]any) string {
	body, err := json.Marshal(message)
	if err != nil {
		return "Nothing sent: the address isn't one it can send to."
	}
	request, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return "Nothing sent: the address isn't one it can send to."
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Smolcorp")
	if w.Secret != "" {
		request.Header.Set("X-Smolcorp-Secret", w.Secret)
	}
	response, err := webhookClient.Do(request)
	if err != nil {
		return "Couldn't reach it: " + err.Error()
	}
	response.Body.Close()
	return fmt.Sprint(response.StatusCode)
}

// describedDelivery is how a delivery went, for the setup page: "Sent: 204
// at 20:41", or what went wrong.
func describedDelivery(result string) string {
	at := time.Now().Format("15:04")
	var status int
	if _, err := fmt.Sscan(result, &status); err != nil || fmt.Sprint(status) != result {
		return result + " (" + at + ")"
	}
	if status >= 200 && status < 300 {
		return fmt.Sprintf("Sent: %d at %s", status, at)
	}
	return fmt.Sprintf("It answered %d at %s", status, at)
}

// told is what a session was when the webhook was last told.
type told struct {
	state   SessionState
	project string
	agent   Agent
	key     int
	kind    string
}

// webhookEvent is the webhook's event for a session going from before to
// after, or "" if it has none.
func webhookEvent(before, after SessionState) string {
	if before == after {
		return ""
	}
	switch after {
	case Asking:
		return "waiting"
	case Unread:
		return "done"
	case Failed:
		return "error"
	case Thinking:
		return "working"
	}
	return ""
}

// webhookMessages is what to tell the webhook about the sessions going from
// before to now: sessions started, ended, or now waiting on you, done,
// failed or at work. A session already working as it starts says only that
// it started.
func webhookMessages(before, now map[string]told, at time.Time) []map[string]any {
	message := func(event string, s told, previous SessionState) map[string]any {
		session := map[string]any{
			"project": s.project, "agent": string(s.agent), "agentName": s.agent.Name(), "state": s.state,
			"previous": nil, "key": nil,
		}
		if previous != "" {
			session["previous"] = previous
		}
		if s.key > 0 {
			session["key"] = s.key
		}
		if s.state == Failed && s.kind != "" {
			session["errorKind"] = s.kind
		}
		return map[string]any{"event": event, "at": at.UTC().Format(time.RFC3339), "session": session}
	}
	// In the order of the desks, then of their ids: one change's messages
	// always go in the same order, and a new session's start before its state
	inOrder := func(sessions map[string]told) []string {
		ids := make([]string, 0, len(sessions))
		for id := range sessions {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool {
			a, b := sessions[ids[i]], sessions[ids[j]]
			if a.key != b.key {
				return a.key < b.key
			}
			return ids[i] < ids[j]
		})
		return ids
	}
	messages := []map[string]any{}
	for _, id := range inOrder(now) {
		s := now[id]
		if old, ok := before[id]; ok {
			if event := webhookEvent(old.state, s.state); event != "" {
				messages = append(messages, message(event, s, old.state))
			}
			continue
		}
		messages = append(messages, message("started", s, ""))
		if event := webhookEvent("", s.state); event != "" && event != "working" {
			messages = append(messages, message(event, s, ""))
		}
	}
	for _, id := range inOrder(before) {
		if _, ok := now[id]; !ok {
			messages = append(messages, message("ended", before[id], before[id].state))
		}
	}
	return messages
}

// tellWebhook tells the webhook what has happened since it was last told,
// with the lock held. Nothing is news the first time the deck looks, so
// what's already running when it starts isn't sent.
func (d *Deck) tellWebhook() {
	keys, _ := d.layout()
	keyOf := map[string]int{}
	for i, key := range keys {
		if key.session != nil {
			keyOf[key.session.ID] = i + 1
		}
	}
	now := map[string]told{}
	for _, s := range d.sessions.All() {
		now[s.ID] = told{state: s.State, project: s.ProjectName(), agent: s.Agent, key: keyOf[s.ID], kind: s.ErrorKind}
	}
	before := d.told
	d.told = now
	webhook := d.config.Webhook
	if before == nil || !webhook.Enabled || webhook.URL == "" || webhookProblem(webhook.URL, webhook.Outside) != "" {
		return
	}
	for _, message := range webhookMessages(before, now, time.Now()) {
		if !contains(webhook.Events, message["event"].(string)) {
			continue
		}
		// One at a time, in order, so a quick "working" then "done" can't arrive the other way round
		select {
		case d.webhookSends <- func() {
			result := describedDelivery(webhook.send(message))
			d.webhookMu.Lock()
			d.webhookLast = result
			d.webhookMu.Unlock()
		}:
		default:
			// The address has stopped answering and messages are piling up: these go
		}
	}
}

// webhookQueue is how many messages wait to be sent at most.
const webhookQueue = 64

// sendWebhooks sends the webhook's messages, one after another, as they come.
func (d *Deck) sendWebhooks() {
	for send := range d.webhookSends {
		send()
	}
}

// testWebhook sends a test message to the webhook as it's set up, without
// the lock, and says how it went.
func (d *Deck) testWebhook(webhook Webhook) {
	result := webhookProblem(webhook.URL, webhook.Outside)
	if result == "" {
		result = describedDelivery(webhook.send(map[string]any{"event": "test", "at": time.Now().UTC().Format(time.RFC3339), "session": nil}))
	}
	d.webhookMu.Lock()
	d.webhookLast = result
	d.webhookMu.Unlock()
}

// lastDelivery is how the last delivery went, or nil before any.
func (d *Deck) lastDelivery() any {
	d.webhookMu.Lock()
	defer d.webhookMu.Unlock()
	if d.webhookLast == "" {
		return nil
	}
	return d.webhookLast
}
