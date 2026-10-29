package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// CloudBot is a bot that works in someone else's cloud, not in a terminal
// here, as a Grok Bot runs at xAI, as the Mac app's CloudBot: it has no
// hooks and nothing to type into, so its desk shows only whether it has
// news. A permanent one is always at work (blue); each new report turns it
// green for a while (see cloudDesk). One that comes and goes sits only
// between its "start" and "stop" events. Set in config.json, through POST
// /setup/bots.
type CloudBot struct {
	// Its address in /bots/<id>/event: letters, digits and dashes.
	ID string `json:"id"`
	// What its desk says.
	Name string `json:"name"`
	// The agent whose colours it wears, one of agents.
	Agent string `json:"agent"`
	// What it wears in the office, one of the team's outfits.
	Outfit string `json:"outfit"`
	// Something extra on its desk, one of cloudBadges, or none.
	Badge string `json:"badge"`
	// Always at its desk, or only between "start" and "stop".
	Permanent bool `json:"permanent"`
	// Where its desk is: "upstairs", the floor above, by the headless runs,
	// as it never has a terminal; or "ground", at a free key.
	Floor string `json:"floor"`
	// Where its reports are seen, besides the event route.
	Source CloudSource `json:"source"`
}

// safeGrokBotID is whether id can be a Grok Bot app's bot id, as the Mac
// app's GrokBotApp.safeID has it: hex digits and dashes, at most 64.
func safeGrokBotID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdefABCDEF-", r) {
			return false
		}
	}
	return true
}

// CloudSource is where a cloud bot's reports are seen besides the event
// route. The Mac app reads the Grok Bot app's own copy of its chats
// ("grokBotApp"); that app keeps it only on a Mac, so here it is kept as
// set, for the Mac app's config, and nothing is read: the event route alone
// tells a desk its news.
type CloudSource struct {
	// "grokBotApp", or "none", the event route alone.
	Kind string `json:"kind"`
	// The bot's name in that app.
	Bot string `json:"bot"`
	// Its id in that app, which a rename there doesn't change: the Mac app
	// finds the bot by it alone when it's set.
	BotID string `json:"botId"`
	// Which of its messages are reports: "unprompted", those in a turn you
	// didn't start, as its routine's; or "any", replies to you too.
	Counts string `json:"counts"`
}

const (
	cloudIDLimit = 32
	// The most cloud bots there can be.
	cloudBotsMost = 9
	// What a cloud bot's desk is told: a report, its news seen, at its desk or gone.
	cloudReport = "report"
	cloudSeen   = "seen"
	cloudStart  = "start"
	cloudStop   = "stop"
)

var (
	cloudFloors  = []string{"upstairs", "ground"}
	cloudKinds   = []string{"none", "grokBotApp"}
	cloudCounts  = []string{"unprompted", "any"}
	cloudEvents  = []string{cloudReport, cloudSeen, cloudStart, cloudStop}
	errCloudKind = errors.New("a cloud bot's part of the wrong kind")
	// What a cloud bot can have on its desk: the Mac app's role badges, which
	// office.js draws. Roles here keep the first of them only, for now; a
	// bot keeps all, so a config from the Mac app keeps its bot's.
	cloudBadges = append(slices.Clone(roleBadges), "palette", "scales", "speech", "pipe", "crown",
		"broom", "flame", "plane", "envelope", "coins", "megaphone", "lock", "toque")
)

func standardCloudBot() CloudBot {
	return CloudBot{Agent: string(Grok), Outfit: "grok", Permanent: true, Floor: "upstairs", Source: standardCloudSource()}
}

func standardCloudSource() CloudSource { return CloudSource{Kind: "none", Counts: "unprompted"} }

// UnmarshalJSON reads a bot as the Mac app does: any part left out, or null,
// is the default's, as setup or a tool of yours may send only some; a part of
// the wrong kind, or a bot that isn't an object, is an error.
func (b *CloudBot) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return errCloudKind
	}
	bot := standardCloudBot()
	for _, part := range []struct {
		key  string
		into any
	}{{"id", &bot.ID}, {"name", &bot.Name}, {"agent", &bot.Agent}, {"outfit", &bot.Outfit}, {"badge", &bot.Badge},
		{"permanent", &bot.Permanent}, {"floor", &bot.Floor}, {"source", &bot.Source}} {
		if err := readPart(raw, part.key, part.into); err != nil {
			return err
		}
	}
	*b = bot
	return nil
}

func (s *CloudSource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return errCloudKind
	}
	source := standardCloudSource()
	for key, into := range map[string]*string{"kind": &source.Kind, "bot": &source.Bot, "botId": &source.BotID, "counts": &source.Counts} {
		if err := readPart(raw, key, into); err != nil {
			return err
		}
	}
	*s = source
	return nil
}

// readPart reads raw's key into into, leaving it as it is when it's missing or null.
func readPart(raw map[string]json.RawMessage, key string, into any) error {
	value, ok := raw[key]
	if !ok || string(value) == "null" {
		return nil
	}
	if json.Unmarshal(value, into) != nil {
		return errCloudKind
	}
	return nil
}

// cloudBotList is the bots as config.json keeps them: one that can't be read
// loses only itself, as the rest of the file is kept.
type cloudBotList []CloudBot

func (l *cloudBotList) UnmarshalJSON(data []byte) error {
	var list []json.RawMessage
	json.Unmarshal(data, &list)
	bots := cloudBotList{}
	for _, raw := range list {
		var bot CloudBot
		if json.Unmarshal(raw, &bot) == nil {
			bots = append(bots, bot)
		}
	}
	*l = bots
	return nil
}

// checked is the bot as kept, and false for one that can't be: a bad id or
// no name.
func (b CloudBot) checked() (CloudBot, bool) {
	id := strings.ToLower(b.ID)
	if id == "" || len([]rune(id)) > cloudIDLimit || strings.ContainsFunc(id, func(c rune) bool { return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') }) {
		return CloudBot{}, false
	}
	// No slash: the name is its folder's last part.
	name := oneLine(strings.ReplaceAll(b.Name, "/", "-"), roleNameLimit)
	if name == "" {
		return CloudBot{}, false
	}
	source := b.Source
	if !slices.Contains(cloudKinds, source.Kind) {
		source.Kind = "none"
	}
	source.Bot = oneLine(source.Bot, 64)
	if !safeGrokBotID(source.BotID) {
		source.BotID = ""
	}
	if source.Kind == "grokBotApp" && source.Bot == "" {
		source.Bot = name
	}
	if !slices.Contains(cloudCounts, source.Counts) {
		source.Counts = "unprompted"
	}
	agent := b.Agent
	if !slices.Contains(agents, Agent(agent)) {
		agent = string(Grok)
	}
	outfit := ""
	if b.Outfit != "auto" && slices.Contains(teamChoices["outfit"], b.Outfit) {
		outfit = b.Outfit
	}
	badge := ""
	if slices.Contains(cloudBadges, b.Badge) {
		badge = b.Badge
	}
	floor := "upstairs"
	if slices.Contains(cloudFloors, b.Floor) {
		floor = b.Floor
	}
	return CloudBot{ID: id, Name: name, Agent: agent, Outfit: outfit, Badge: badge, Permanent: b.Permanent, Floor: floor, Source: source}, true
}

// checkedCloudBots is the bots as kept: each checked, each id once, at most cloudBotsMost.
func checkedCloudBots(bots []CloudBot) []CloudBot {
	kept := []CloudBot{}
	for _, bot := range bots {
		if bot, ok := bot.checked(); ok && len(kept) < cloudBotsMost && !slices.ContainsFunc(kept, func(k CloudBot) bool { return k.ID == bot.ID }) {
			kept = append(kept, bot)
		}
	}
	return kept
}

// role is what it wears, as the phone is told of a role, through the path
// office.js already draws: no name, so its desk says only its own.
func (b CloudBot) role() map[string]any {
	return map[string]any{"id": "cloud", "name": "", "badge": b.Badge, "outfit": b.Outfit}
}

// hirer is who its card upstairs says hired it: a Grok Bot is one of the
// Grok Bots. "" for no one.
func (b CloudBot) hirer() string {
	if b.Agent == string(Grok) {
		return "Grok Bots"
	}
	return ""
}

func (b CloudBot) sessionID() string { return "cloud-" + b.ID }
func (b CloudBot) tab() string       { return "cloud:" + b.ID }
func (b CloudBot) folder() string    { return "/cloud/" + b.Name }

// MARK: Its last action

// CloudAction is what a cloud bot last did, for its card: what, how many, and when.
type CloudAction struct {
	Text  string
	Count int
	At    time.Time
}

const (
	cloudReportText = "Sent a report"
	cloudTextLimit  = 60
	cloudCountMost  = 99
)

var markupTag = regexp.MustCompile(`<[^<>]*>`)

// newCloudAction is an action as kept: plain text, markup's tags gone, then
// any bracket left, on one line and cut short, or the report's words; and a
// count from 1 to cloudCountMost.
func newCloudAction(text string, count int, at time.Time) CloudAction {
	text = strings.NewReplacer("<", "", ">", "").Replace(markupTag.ReplaceAllString(text, ""))
	text = oneLine(text, cloudTextLimit)
	if text == "" {
		text = cloudReportText
	}
	return CloudAction{Text: text, Count: min(max(count, 1), cloudCountMost), At: at}
}

// json is the action for the stream: its time in milliseconds, as the office counts.
func (a CloudAction) json() map[string]any {
	return map[string]any{"text": a.Text, "count": a.Count, "at": a.At.UnixMilli()}
}

// parseCloudAction is what an event's body says, as {"type": "report",
// "text": "Sent a report", "count": 3, "at": "<ISO 8601>"}: every part but
// the type may be left out. Plain text only, cut short; a time to come is
// now. The error says why for a part that can't be read.
func parseCloudAction(body map[string]any, now time.Time) (CloudAction, error) {
	text := cloudReportText
	if given, ok := body["text"]; ok && given != nil {
		s, ok := given.(string)
		if !ok {
			return CloudAction{}, errors.New("text should be a string.")
		}
		text = s
	}
	count := 1
	if given, ok := body["count"]; ok && given != nil {
		number, ok := given.(float64)
		if !ok || number != math.Round(number) || number < 1 {
			return CloudAction{}, errors.New("count should be a whole number, 1 or more.")
		}
		count = int(min(number, cloudCountMost))
	}
	at := now
	if given, ok := body["at"]; ok && given != nil {
		s, _ := given.(string)
		date, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return CloudAction{}, errors.New("at should be an ISO 8601 time, as 2026-10-28T22:20:00Z.")
		}
		if date.Before(now) {
			at = date
		}
	}
	return newCloudAction(text, count, at), nil
}

// MARK: The desk

// cloudGreenFor is ten minutes: long enough to be seen across the room, gone
// well before an hourly routine's next run.
const cloudGreenFor = 10 * time.Minute

// cloudDesk is one cloud bot's desk, as a state: whether it's at its desk,
// when its last report came, and what it last did. Green lasts
// cloudGreenFor, or until its card is opened.
type cloudDesk struct {
	present bool
	// When its last report came, zero when it has no news.
	reportAt time.Time
	// Kept once seen, for its card; nil until its first report.
	last *CloudAction
}

func newCloudDesk(permanent bool) cloudDesk { return cloudDesk{present: permanent} }

func (d cloudDesk) same(other cloudDesk) bool {
	return d.present == other.present && d.reportAt.Equal(other.reportAt) &&
		(d.last == nil) == (other.last == nil) && (d.last == nil || *d.last == *other.last)
}

// state is what it shows: idle when it isn't at its desk.
func (d cloudDesk) state(now time.Time) SessionState {
	if !d.present {
		return Idle
	}
	if !d.reportAt.IsZero() && now.Sub(d.reportAt) < cloudGreenFor && !now.Before(d.reportAt) {
		return Unread
	}
	return Thinking
}

// nextChange is when it next changes by itself, when its green ends, or zero.
func (d cloudDesk) nextChange(now time.Time) time.Time {
	if d.state(now) != Unread {
		return time.Time{}
	}
	return d.reportAt.Add(cloudGreenFor)
}

// apply: a report comes to a bot that's away too: it sits down to give it.
// Reports not yet seen add up on its card; it's green from when the last
// arrived, whatever time it says it was sent. action is nil for the plain
// report: one, now.
func (d *cloudDesk) apply(event string, now time.Time, permanent bool, action *CloudAction) {
	switch event {
	case cloudReport:
		given := newCloudAction(cloudReportText, 1, now)
		if action != nil {
			given = *action
		}
		if !d.reportAt.IsZero() && d.last != nil {
			given.Count = min(d.last.Count+given.Count, cloudCountMost)
		}
		d.last = &given
		d.present = true
		d.reportAt = now
	case cloudSeen:
		d.reportAt = time.Time{}
	case cloudStart:
		d.present = true
	case cloudStop:
		// A permanent bot never leaves.
		d.present = permanent
		d.reportAt = time.Time{}
	}
}

// MARK: Desks

// cloudDesks are the cloud bots' desks: their sessions, as the office draws
// them, kept here rather than in Sessions, so nothing that looks for tabs,
// hooks or processes ever touches them. They sit upstairs, first, before the
// headless runs, or at free desks on the ground floor as any new session
// would; they have no controls. Used with the deck's lock held.
type cloudDesks struct {
	bots     []CloudBot
	desks    map[string]cloudDesk
	sessions map[string]*Session
	// onChange is called, with the lock held, when a desk changes.
	onChange func()
	// lock is the deck's, taken when a green ends by itself; nil, as in
	// tests, for no timer.
	lock   sync.Locker
	expiry *time.Timer
}

func newCloudDesks(bots []CloudBot, lock sync.Locker) *cloudDesks {
	c := &cloudDesks{desks: map[string]cloudDesk{}, sessions: map[string]*Session{}, onChange: func() {}, lock: lock}
	c.configure(bots)
	return c
}

// configure takes the bots as set: a bot kept keeps its desk and state; a
// new one starts now, at work if permanent.
func (c *cloudDesks) configure(given []CloudBot) {
	now := time.Now()
	before := map[string]CloudBot{}
	for _, bot := range c.bots {
		if _, ok := before[bot.ID]; !ok {
			before[bot.ID] = bot
		}
	}
	c.bots = slices.Clone(given)
	set := map[string]bool{}
	sessionIDs := map[string]bool{}
	for _, bot := range given {
		set[bot.ID], sessionIDs[bot.sessionID()] = true, true
	}
	for id := range c.desks {
		if !set[id] {
			delete(c.desks, id)
		}
	}
	for id := range c.sessions {
		if !sessionIDs[id] {
			delete(c.sessions, id)
		}
	}
	for _, bot := range given {
		old, was := before[bot.ID]
		desk, has := c.desks[bot.ID]
		if !has || !was || old.Permanent != bot.Permanent {
			if !has {
				desk = newCloudDesk(bot.Permanent)
			}
			if bot.Permanent {
				desk.present = true
			}
			c.desks[bot.ID] = desk
		}
		// A new name, agent or source starts it afresh.
		session := c.sessions[bot.sessionID()]
		if !was || old != bot || session == nil {
			startedAt := float64(now.UnixMilli())
			if session != nil {
				startedAt = session.StartedAt
			}
			agent := Agent(bot.Agent)
			if !slices.Contains(agents, agent) {
				agent = Grok
			}
			c.sessions[bot.sessionID()] = &Session{ID: bot.sessionID(), Agent: agent, Folder: bot.folder(), Project: bot.folder(),
				Tab: bot.tab(), State: Idle, StartedAt: startedAt}
		}
	}
	c.update(false)
}

// seated is the desks to seat on floor, in the order the bots were set.
func (c *cloudDesks) seated(floor string) []*Session {
	seated := []*Session{}
	for _, bot := range c.bots {
		if s := c.sessions[bot.sessionID()]; bot.Floor == floor && s != nil && s.State != Idle {
			seated = append(seated, s)
		}
	}
	return seated
}

// botOf is the bot at the desk of session id, if it's a cloud bot's.
func (c *cloudDesks) botOf(id string) (CloudBot, bool) {
	for _, bot := range c.bots {
		if bot.sessionID() == id {
			return bot, true
		}
	}
	return CloudBot{}, false
}

// apply is an event for bot id, from the event route; false for a bot there isn't.
func (c *cloudDesks) apply(event, id string, action *CloudAction) bool {
	i := slices.IndexFunc(c.bots, func(b CloudBot) bool { return b.ID == id })
	if i < 0 {
		return false
	}
	bot := c.bots[i]
	before, ok := c.desks[id]
	if !ok {
		before = newCloudDesk(bot.Permanent)
	}
	desk := before
	desk.apply(event, time.Now(), bot.Permanent, action)
	c.desks[id] = desk
	// Its card may change with no change of colour, as a second report adds up.
	c.update(!desk.same(before))
	return true
}

// last is what the bot at desk id last did, for its card, or nil.
func (c *cloudDesks) last(id string) *CloudAction {
	if bot, ok := c.botOf(id); ok {
		return c.desks[bot.ID].last
	}
	return nil
}

// seen: its card was opened, or its desk tapped: its news is seen, and
// nothing else happens. False for a desk that isn't a cloud bot's. (The Mac
// app's tap also brings the Grok Bot app forward, which is the Mac's only.)
func (c *cloudDesks) seen(id string) bool {
	bot, ok := c.botOf(id)
	if !ok {
		return false
	}
	return c.apply(cloudSeen, bot.ID, nil)
}

// update brings each desk's session in line with what it shows now, and
// plans the next change by itself.
func (c *cloudDesks) update(changed bool) {
	now := time.Now()
	for _, bot := range c.bots {
		s := c.sessions[bot.sessionID()]
		if s == nil {
			continue
		}
		state := Idle
		if desk, ok := c.desks[bot.ID]; ok {
			state = desk.state(now)
		}
		if s.State != state {
			s.State = state
			changed = true
		}
	}
	if c.expiry != nil {
		c.expiry.Stop()
		c.expiry = nil
	}
	var next time.Time
	for _, desk := range c.desks {
		if at := desk.nextChange(now); !at.IsZero() && (next.IsZero() || at.Before(next)) {
			next = at
		}
	}
	if !next.IsZero() && c.lock != nil {
		var timer *time.Timer
		timer = time.AfterFunc(max(100*time.Millisecond, next.Sub(now)+100*time.Millisecond), func() {
			c.lock.Lock()
			defer c.lock.Unlock()
			// Unless a later update planned another.
			if c.expiry == timer {
				c.update(false)
			}
		})
		c.expiry = timer
	}
	if changed {
		c.onChange()
	}
}

// MARK: The deck

// cloudEntry is a cloud bot's desk, as the phone is told of it: whether it
// has news, in its outfit, with every control off, as there's nothing here
// to type into; what it last did; and who hired it, for its card upstairs.
// With the lock held.
func (d *Deck) cloudEntry(s *Session, bot CloudBot) map[string]any {
	entry := d.sessionEntry(s)
	// Its own name, whatever it holds: a session's name is its folder's last part.
	entry["name"], entry["project"] = bot.Name, bot.Name
	for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary", "phoneMic", "conversation"} {
		entry[control] = false
	}
	entry["role"] = bot.role()
	entry["cloud"] = true
	entry["last"] = nil
	if last := d.cloud.last(s.ID); last != nil {
		entry["last"] = last.json()
	}
	// Its card upstairs: who it works for, with no desk downstairs to visit.
	entry["hiredBy"] = nil
	if hirer := bot.hirer(); hirer != "" {
		entry["hiredBy"] = map[string]any{"id": nil, "name": hirer}
	}
	return entry
}

// MARK: Its routes

// cloudBotEvent is POST /bots/<id>/event, with {"type": "report"}, "seen",
// "start" or "stop" (or ?type=): what a cloud bot's desk is told by hand. A
// report may say what it was, how many and when, for the desk's card:
// {"type": "report", "text": "Sent a report", "count": 3, "at": "<ISO 8601>"}.
func (d *Deck) cloudBotEvent(method, path string, query url.Values, body map[string]any) (int, string) {
	parts := strings.Split(path, "/")
	if method != http.MethodPost || len(parts) != 4 || parts[3] != "event" {
		return http.StatusNotFound, ""
	}
	event, ok := body["type"].(string)
	if !ok {
		event = query.Get("type")
	}
	if !slices.Contains(cloudEvents, event) {
		return http.StatusBadRequest, "Say what happened: " + strings.Join(cloudEvents, ", ") + "."
	}
	var action *CloudAction
	if event == cloudReport {
		given, err := parseCloudAction(body, time.Now())
		if err != nil {
			return http.StatusBadRequest, err.Error()
		}
		action = &given
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.cloud.apply(event, parts[2], action) {
		return http.StatusNotFound, "There's no cloud bot by that id."
	}
	return http.StatusNoContent, ""
}

// cloudBotsSent is the bots POST /setup/bots sends, as {"bots": [...]} or
// the list alone; false unless it's a list of bots this app can read.
func cloudBotsSent(body io.Reader) ([]CloudBot, bool) {
	var sent any
	if json.NewDecoder(body).Decode(&sent) != nil {
		return nil, false
	}
	if object, ok := sent.(map[string]any); ok {
		sent = object["bots"]
	}
	list, ok := sent.([]any)
	if !ok {
		return nil, false
	}
	data, _ := json.Marshal(list)
	var bots []CloudBot
	if json.Unmarshal(data, &bots) != nil {
		return nil, false
	}
	return bots, true
}
