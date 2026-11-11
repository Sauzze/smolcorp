package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ⓘ Info, in the office's own window: what an agent's session has used, on a
// card, as the Mac's SessionInfo.swift made it until M7. Only what this
// computer already has, and only what's there: a fact with no source here is
// left out of the card, not guessed at.
//
//   - Claude Code's transcript: each reply's model, effort and tokens, and how
//     long each turn took. Read once, then only what's been added since.
//   - The profile Claude Code keeps beside its settings (.claude.json): whose
//     subscription it is, and the 5-hour and weekly use Claude Code last fetched.
//   - The hook: whether the agent has an API key, or a cloud provider, in its
//     environment (X-AgentDeck-Billing). Only that one is there: never the key.
//
// Nothing is sent anywhere: the card reads it from this computer, as the
// conversation does. No key, token or credential is read, kept or answered.

// apiRate is a model's price, in dollars per million tokens.
type apiRate struct {
	input, output, cacheRead float64
	// Fast mode's prices, as a multiple of these: only where fast mode has a price.
	fast float64
	// Over this many prompt tokens a request is priced at the long ones instead (Haiku 5.5).
	longFrom                             int
	longInput, longOutput, longCacheRead float64
}

// apiPrices are Anthropic's published API prices, from
// platform.claude.com/docs/en/about-claude/pricing as of 2026-10-23, as the Mac
// had them. Cache writes are the input price × 1.25 for 5 minutes and × 2 for
// an hour, on every model. Claude Code's own cost for a session (its
// cost-state) comes out the same from these.
var apiPrices = map[string]apiRate{
	"claude-fable-5-1": {input: 10, output: 50, cacheRead: 0.25}, "claude-mythos-5-1": {input: 10, output: 50, cacheRead: 0.25},
	"claude-fable-5": {input: 10, output: 50, cacheRead: 1}, "claude-mythos-5": {input: 10, output: 50, cacheRead: 1},
	"claude-opus-5-5": {input: 4, output: 20, cacheRead: 0.20, fast: 2}, "claude-opus-5": {input: 5, output: 25, cacheRead: 0.50, fast: 2},
	"claude-opus-4-8": {input: 5, output: 25, cacheRead: 0.50, fast: 2}, "claude-opus-4-7": {input: 5, output: 25, cacheRead: 0.50},
	"claude-opus-4-6": {input: 5, output: 25, cacheRead: 0.50}, "claude-opus-4-5": {input: 5, output: 25, cacheRead: 0.50},
	"claude-sonnet-5-5": {input: 2, output: 10, cacheRead: 0.10}, "claude-sonnet-5": {input: 2, output: 10, cacheRead: 0.20},
	"claude-sonnet-4-6": {input: 3, output: 15, cacheRead: 0.30}, "claude-sonnet-4-5": {input: 3, output: 15, cacheRead: 0.30},
	"claude-haiku-5-5": {input: 0.10, output: 0.50, cacheRead: 0.01, longFrom: 100_000, longInput: 0.50, longOutput: 2.50, longCacheRead: 0.05},
	"claude-haiku-4-5": {input: 1, output: 5, cacheRead: 0.10},
}

// priceID is a model as the table names it: without Claude Code's "[1m]" or a snapshot's date.
func priceID(model string) string {
	id := strings.ToLower(strings.TrimSpace(model))
	if bracket := strings.IndexByte(id, '['); bracket >= 0 {
		id = id[:bracket]
	}
	if dash := strings.LastIndexByte(id, '-'); dash >= 0 && len(id)-dash == 9 && strings.Trim(id[dash+1:], "0123456789") == "" {
		id = id[:dash]
	}
	return id
}

// tokenCount is tokens, as a reply's usage counts them.
type tokenCount struct {
	input, output, cacheRead, write5m, write1h int
}

func (t tokenCount) cacheWrite() int { return t.write5m + t.write1h }

// prompt is what was sent: what a long prompt's price is decided by.
func (t tokenCount) prompt() int { return t.input + t.cacheRead + t.cacheWrite() }

func (t tokenCount) plus(o tokenCount, sign int) tokenCount {
	return tokenCount{t.input + sign*o.input, t.output + sign*o.output, t.cacheRead + sign*o.cacheRead,
		t.write5m + sign*o.write5m, t.write1h + sign*o.write1h}
}

// tokensOf is a reply's usage; a write not split by how long it's kept counts as 5 minutes'.
func tokensOf(usage map[string]any) tokenCount {
	count := func(object map[string]any, key string) int {
		n, _ := object[key].(float64)
		return int(n)
	}
	t := tokenCount{input: count(usage, "input_tokens"), output: count(usage, "output_tokens"), cacheRead: count(usage, "cache_read_input_tokens")}
	if split, ok := usage["cache_creation"].(map[string]any); ok {
		t.write5m, t.write1h = count(split, "ephemeral_5m_input_tokens"), count(split, "ephemeral_1h_input_tokens")
	} else {
		t.write5m = count(usage, "cache_creation_input_tokens")
	}
	return t
}

// apiCost is one request's price in dollars; false for a model, or a fast mode, with no price here.
func apiCost(t tokenCount, model string, fast bool) (float64, bool) {
	rate, ok := apiPrices[priceID(model)]
	if !ok {
		return 0, false
	}
	multiple := 1.0
	if fast {
		if rate.fast == 0 {
			return 0, false
		}
		multiple = rate.fast
	}
	input, output, read := rate.input, rate.output, rate.cacheRead
	if rate.longFrom > 0 && t.prompt() > rate.longFrom {
		input, output, read = rate.longInput, rate.longOutput, rate.longCacheRead
	}
	sum := float64(t.input)*input + float64(t.output)*output + float64(t.cacheRead)*read +
		float64(t.write5m)*input*1.25 + float64(t.write1h)*input*2
	return sum * multiple / 1_000_000, true
}

// transcriptTime is a transcript's ISO 8601 time, in ms.
func transcriptTime(text string) (float64, bool) {
	when, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return 0, false
	}
	return float64(when.UnixMilli()), true
}

// tally is what a Claude Code transcript says of its session, line by line:
// its replies' tokens and their price, the latest reply's model and effort,
// when it started, and how long its turns took.
type tally struct {
	tokens tokenCount
	// The replies' price, in dollars, at apiPrices.
	cost float64
	// Replies from a model, or in a mode, with no price here: cost leaves them out.
	unpriced int
	// The latest reply's, outside subagents.
	model, effort string
	fast          bool
	// The first line's time, in ms, 0 if none.
	startedAt float64
	// The turns' time, as Claude Code measured each, in ms.
	activeMs float64
	// Each reply counted, by its message's id: a reply is written once for each of its blocks, all with the same
	// usage, and counted once.
	counted map[string]countedReply
	unnamed int
}

type countedReply struct {
	tokens tokenCount
	cost   float64
	priced bool
}

func (t *tally) take(entry map[string]any) {
	if t.startedAt == 0 {
		if text, ok := entry["timestamp"].(string); ok {
			if at, ok := transcriptTime(text); ok {
				t.startedAt = at
			}
		}
	}
	switch entry["type"] {
	case "assistant":
		message, _ := entry["message"].(map[string]any)
		model, _ := message["model"].(string)
		if model == "" || model == "<synthetic>" {
			return
		}
		usage, _ := message["usage"].(map[string]any)
		fast := usage["speed"] == "fast"
		if entry["isSidechain"] != true {
			t.model, t.fast = model, fast
			if effort, _ := entry["effort"].(string); effort != "" {
				t.effort = effort
			}
		}
		tokens := tokensOf(usage)
		price, priced := apiCost(tokens, model, fast)
		id, ok := message["id"].(string)
		if !ok {
			t.unnamed++
			id = "\x00" + strconv.Itoa(t.unnamed) // never a message's own id
		}
		if t.counted == nil {
			t.counted = map[string]countedReply{}
		}
		if earlier, ok := t.counted[id]; ok {
			t.add(earlier, -1)
		}
		reply := countedReply{tokens, price, priced}
		t.counted[id] = reply
		t.add(reply, 1)
	case "system":
		if ms, _ := entry["durationMs"].(float64); entry["subtype"] == "turn_duration" && ms > 0 {
			t.activeMs += ms
		}
	}
}

// takeSubagent counts a subagent's replies: their tokens and price, not its model or turns.
func (t *tally) takeSubagent(other tally) {
	t.tokens = t.tokens.plus(other.tokens, 1)
	t.cost += other.cost
	t.unpriced += other.unpriced
}

func (t *tally) add(reply countedReply, sign int) {
	t.tokens = t.tokens.plus(reply.tokens, sign)
	if reply.priced {
		t.cost += float64(sign) * reply.cost
	} else {
		t.unpriced += sign
	}
}

// transcriptFollower follows a transcript as it grows: each look reads only
// what's been added, whole lines only, and only the lines a tally takes are parsed.
type transcriptFollower struct {
	path     string
	tally    tally
	offset   int64
	pending  []byte
	tooLong  bool // the line not yet ended is already too long to parse
	lastUsed time.Time
}

const (
	// Read at a time.
	followerChunk = 4 * 1024 * 1024
	// A line longer than this is a tool's whole output: skipped, not parsed.
	followerLineMost = 2 * 1024 * 1024
)

var (
	// What a line must hold to be parsed.
	followerWanted    = [][]byte{[]byte(`"assistant"`), []byte(`"turn_duration"`)}
	followerTimestamp = []byte(`"timestamp"`)
)

// update reads what's new. A file that got shorter is read again from its start.
func (f *transcriptFollower) update() {
	f.lastUsed = time.Now()
	file, info, err := openRegular(f.path)
	if err != nil {
		return
	}
	defer file.Close()
	if info.Size() < f.offset {
		*f = transcriptFollower{path: f.path, lastUsed: f.lastUsed}
	}
	if info.Size() <= f.offset {
		return
	}
	if _, err := file.Seek(f.offset, io.SeekStart); err != nil {
		return
	}
	buffer := make([]byte, min(followerChunk, info.Size()-f.offset))
	for {
		n, _ := file.Read(buffer)
		if n <= 0 {
			return
		}
		f.offset += int64(n)
		rest := buffer[:n]
		for {
			newline := bytes.IndexByte(rest, '\n')
			if newline < 0 {
				break
			}
			if !f.tooLong && len(f.pending)+newline <= followerLineMost {
				f.take(append(f.pending, rest[:newline]...))
			}
			f.pending, f.tooLong = nil, false
			rest = rest[newline+1:]
		}
		// The line not yet ended: kept for the next read, unless it's already too long to parse.
		if !f.tooLong && len(f.pending)+len(rest) <= followerLineMost {
			f.pending = append(f.pending, rest...)
		} else {
			f.pending, f.tooLong = nil, true
		}
	}
}

func (f *transcriptFollower) take(line []byte) {
	wanted := f.tally.startedAt == 0 && bytes.Contains(line, followerTimestamp)
	for _, word := range followerWanted {
		wanted = wanted || bytes.Contains(line, word)
	}
	var entry map[string]any
	if wanted && json.Unmarshal(line, &entry) == nil && entry != nil {
		f.tally.take(entry)
	}
}

// claudeAccount is whose Claude Code it is, from the profile it keeps beside
// its settings: never its keys, which it keeps in the system's own store anyway.
type claudeAccount struct {
	// Its email, or its name where there's no email.
	name string
	// "Max 20x", "Pro"…, where the profile says.
	plan         string
	subscription bool
	fiveHour     *usageWindow
	weekly       *usageWindow
	// When Claude Code fetched that use, in ms, 0 if none is shown.
	usageAt float64
}

type usageWindow struct {
	percent float64
	// In ms, 0 if not known.
	resetsAt float64
}

// Use fetched longer ago than this isn't shown.
const usageFresh = 60 * 60 * 1000.0

// claudeAccountFrom is the account in profile (a .claude.json), with the use
// Claude Code last fetched for it: only that account's, only fresh, and only
// windows that haven't reset since. False with no subscription login.
func claudeAccountFrom(profile map[string]any, now float64) (claudeAccount, bool) {
	oauth, ok := profile["oauthAccount"].(map[string]any)
	if !ok {
		return claudeAccount{}, false
	}
	text := func(key string) string {
		s, _ := oauth[key].(string)
		return strings.TrimSpace(s)
	}
	or := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	account := claudeAccount{name: or(text("emailAddress"), text("displayName")),
		plan: claudePlan(text("organizationType"), or(text("organizationRateLimitTier"), text("userRateLimitTier")))}
	account.subscription = account.plan != "" || strings.Contains(text("billingType"), "subscription")

	// Claude Code's own cache of what /usage shows, for this account only.
	cache, _ := profile["cachedUsageUtilization"].(map[string]any)
	owner, _ := cache["accountUuid"].(string)
	fetched, hasFetched := cache["fetchedAtMs"].(float64)
	windows, hasWindows := cache["utilization"].(map[string]any)
	if account.subscription && owner != "" && owner == text("accountUuid") && hasFetched && now-fetched <= usageFresh &&
		fetched <= now+60_000 && hasWindows {
		window := func(key string) *usageWindow {
			object, _ := windows[key].(map[string]any)
			percent, ok := object["utilization"].(float64)
			if !ok || percent < 0 {
				return nil
			}
			resets := 0.0
			if text, ok := object["resets_at"].(string); ok {
				resets, _ = transcriptTime(text)
			}
			// Reset since it was fetched: what's been used since isn't known.
			if resets != 0 && resets <= now {
				return nil
			}
			return &usageWindow{percent: min(percent, 100), resetsAt: resets}
		}
		account.fiveHour, account.weekly = window("five_hour"), window("seven_day")
		if account.fiveHour != nil || account.weekly != nil {
			account.usageAt = fetched
		}
	}
	return account, true
}

// claudePlan is "Max 20x" for "claude_max" with "default_claude_max_20x"; "" for no plan it knows.
func claudePlan(kind, tier string) string {
	name, ok := map[string]string{"claude_max": "Max", "claude_pro": "Pro", "claude_team": "Team", "claude_enterprise": "Enterprise"}[kind]
	if !ok {
		return ""
	}
	parts := strings.Split(tier, "_")
	if last := parts[len(parts)-1]; tier != "" && len(last) > 1 && strings.HasSuffix(last, "x") &&
		strings.Trim(last[:len(last)-1], "0123456789") == "" {
		return name + " " + last
	}
	return name
}

// agedAccount is an account read earlier, as it stands now: use that's gone stale, or a window that's reset, goes.
func agedAccount(account claudeAccount, now float64) claudeAccount {
	if account.usageAt != 0 && now-account.usageAt > usageFresh {
		account.fiveHour, account.weekly = nil, nil
	}
	if account.fiveHour != nil && account.fiveHour.resetsAt != 0 && account.fiveHour.resetsAt <= now {
		account.fiveHour = nil
	}
	if account.weekly != nil && account.weekly.resetsAt != 0 && account.weekly.resetsAt <= now {
		account.weekly = nil
	}
	if account.fiveHour == nil && account.weekly == nil {
		account.usageAt = 0
	}
	return account
}

// claudeConfigFolder is the folder of settings a transcript's Claude Code reads: ~/.claude, or a CLAUDE_CONFIG_DIR of its own.
func claudeConfigFolder(transcript, home string) string {
	for _, projects := range []string{string(filepath.Separator) + "projects" + string(filepath.Separator), "/projects/"} {
		if at := strings.LastIndex(transcript, projects); at >= 0 {
			return transcript[:at]
		}
	}
	return filepath.Join(home, ".claude")
}

// claudeProfilePath is the profile Claude Code keeps for that folder: ~/.claude.json for ~/.claude, and inside any other.
func claudeProfilePath(folder, home string) string {
	same := folder == filepath.Join(home, ".claude")
	if runtime.GOOS == "windows" { // its paths don't tell capitals apart
		same = strings.EqualFold(folder, filepath.Join(home, ".claude"))
	}
	if same {
		return filepath.Join(home, ".claude.json")
	}
	return filepath.Join(folder, ".claude.json")
}

// hasKeyHelper is whether any of these settings files names an apiKeyHelper: only whether it's there, never what it runs.
func hasKeyHelper(files []string) bool {
	for _, path := range files {
		data, err := readRegular(path)
		if err != nil {
			continue
		}
		var settings map[string]any
		if json.Unmarshal(data, &settings) != nil {
			continue
		}
		if helper, ok := settings["apiKeyHelper"].(string); ok && strings.Trim(helper, " \t") != "" {
			return true
		}
	}
	return false
}

// modelName is "Opus 5.5 · 1M" for "claude-opus-5-5[1m]"; anything else, as it is.
func modelName(model string) string {
	long := strings.HasSuffix(strings.ToLower(model), "[1m]")
	id := priceID(model)
	if !strings.HasPrefix(id, "claude-") {
		return model
	}
	parts := strings.FieldsFunc(strings.TrimPrefix(id, "claude-"), func(r rune) bool { return r == '-' })
	if len(parts) < 2 || parts[0] == "" {
		return model
	}
	for _, part := range parts[1:] {
		if strings.Trim(part, "0123456789") != "" {
			return model
		}
	}
	name := strings.ToUpper(parts[0][:1]) + parts[0][1:] + " " + strings.Join(parts[1:], ".")
	if long {
		return name + " · 1M"
	}
	return name
}

// Billing, from the hook's X-AgentDeck-Billing: how the agent pays, from its environment, never more than which.
const (
	billingAPIKey = "api-key"
	// Bedrock, Vertex or Foundry: their own prices.
	billingCloud = "cloud"
)

// infoDesk is what the card needs of a session, taken with the deck's lock.
type infoDesk struct {
	id                   string
	agent                Agent
	project, projectPath string
	transcript           string
	startedAt            float64
	model, effort        string
	turnStartedAt        float64 // 0 when not at work
}

// sessionInfos are every desk's facts, kept up to date as the card asks for them, under a lock of their own.
type sessionInfos struct {
	mu        sync.Mutex // the followers and the profiles: held while files are read
	followers map[string]*transcriptFollower
	profiles  map[string]keptProfile
	billingMu sync.Mutex // the billing only, never held for a read: a hook never waits on the card
	billing   map[string]string
}

type keptProfile struct {
	modified time.Time
	account  claudeAccount
	ok       bool
}

// Transcripts followed at most: the least recently asked for go.
const followersMost = 48

// infos is the card's facts, shared as the Mac's SessionInfo.shared was: the hook and the window's card read it.
var infos = &sessionInfos{followers: map[string]*transcriptFollower{}, profiles: map[string]keptProfile{}, billing: map[string]string{}}

// noteBilling keeps, from each hook event, the session's X-AgentDeck-Billing, or that it has none.
func (s *sessionInfos) noteBilling(header, session string) {
	if session == "" {
		return
	}
	s.billingMu.Lock()
	defer s.billingMu.Unlock()
	if header == billingAPIKey || header == billingCloud {
		s.billing[session] = header
	} else {
		delete(s.billing, session)
	}
}

// report is the card's facts for desk, now (in ms).
func (s *sessionInfos) report(desk infoDesk, now float64, home string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var read *tally
	var account *claudeAccount
	apiKey := false
	own := "" // the transcript this report follows: never the one to let go
	if desk.agent == Claude && desk.transcript != "" {
		t := s.follow(desk.transcript)
		own = desk.transcript
		subagents := strings.TrimSuffix(desk.transcript, filepath.Ext(desk.transcript)) + string(filepath.Separator) + "subagents"
		if entries, err := os.ReadDir(subagents); err == nil {
			for _, entry := range entries {
				if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".jsonl") {
					t.takeSubagent(s.follow(filepath.Join(subagents, entry.Name())))
				}
			}
		}
		read = &t
		folder := claudeConfigFolder(desk.transcript, home)
		if found, ok := s.account(claudeProfilePath(folder, home), now); ok {
			account = &found
		}
		apiKey = hasKeyHelper([]string{filepath.Join(folder, "settings.json"), filepath.Join(desk.projectPath, ".claude", "settings.json"),
			filepath.Join(desk.projectPath, ".claude", "settings.local.json")})
	}
	s.trim(own)
	s.billingMu.Lock()
	billing := s.billing[desk.id]
	s.billingMu.Unlock()
	if apiKey {
		billing = billingAPIKey
	}
	return infoCard(desk, read, account, billing, now)
}

func (s *sessionInfos) follow(path string) tally {
	follower := s.followers[path]
	if follower == nil {
		follower = &transcriptFollower{path: path}
		s.followers[path] = follower
	}
	follower.update()
	return follower.tally
}

func (s *sessionInfos) account(path string, now float64) (claudeAccount, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return claudeAccount{}, false
	}
	if kept, ok := s.profiles[path]; ok && kept.modified.Equal(info.ModTime()) {
		// The same file, a little later: a window may have reset meanwhile.
		return agedAccount(kept.account, now), kept.ok
	}
	data, err := readRegular(path)
	var profile map[string]any
	if err != nil || json.Unmarshal(data, &profile) != nil {
		return claudeAccount{}, false
	}
	account, ok := claudeAccountFrom(profile, now)
	s.profiles[path] = keptProfile{info.ModTime(), account, ok}
	return account, ok
}

// trim lets the least recently used transcripts go, but never keep, the one this report followed.
func (s *sessionInfos) trim(keep string) {
	if len(s.followers) <= followersMost {
		return
	}
	paths := make([]string, 0, len(s.followers))
	for path := range s.followers {
		if path != keep {
			paths = append(paths, path)
		}
	}
	sort.Slice(paths, func(i, j int) bool { return s.followers[paths[i]].lastUsed.Before(s.followers[paths[j]].lastUsed) })
	for _, path := range paths[:min(len(paths), len(s.followers)-followersMost)] {
		delete(s.followers, path)
	}
}

// infoCard is the card's facts, from what's been gathered: a fact with no source is left out.
func infoCard(desk infoDesk, read *tally, account *claudeAccount, billing string, now float64) map[string]any {
	card := map[string]any{"agent": desk.agent.Name(), "desk": desk.project}

	// Who pays: an API key or a cloud provider in its environment wins over the profile's login.
	switch billing {
	case billingAPIKey, billingCloud:
		card["account"] = map[string]any{"kind": billing}
	default:
		if account != nil && account.subscription {
			shown := map[string]any{"kind": "subscription"}
			if account.name != "" {
				shown["name"] = account.name
			}
			if account.plan != "" {
				shown["plan"] = account.plan
			}
			card["account"] = shown
			limits := map[string]any{}
			for key, window := range map[string]*usageWindow{"fiveHour": account.fiveHour, "weekly": account.weekly} {
				if window == nil {
					continue
				}
				shown := map[string]any{"percent": window.percent}
				if window.resetsAt != 0 {
					shown["resetsInMs"] = max(0, window.resetsAt-now)
				}
				limits[key] = shown
			}
			if len(limits) > 0 && account.usageAt != 0 {
				limits["ageMs"] = max(0, now-account.usageAt)
				card["limits"] = limits
			}
		}
	}

	// The model actually running: the latest reply's, before the one the hook named.
	model, effort := desk.model, desk.effort
	if read != nil && read.model != "" {
		model = read.model
	}
	if read != nil && read.effort != "" {
		effort = read.effort
	}
	if model != "" {
		shown := map[string]any{"id": model, "name": modelName(model)}
		if effort != "" {
			shown["effort"] = effort
		}
		if read != nil && read.fast {
			shown["fast"] = true
		}
		card["model"] = shown
	}

	// Time: since the session started; its turns, from the transcript; and on this desk, through /clear.
	spent := map[string]any{}
	started := desk.startedAt
	if read != nil && read.startedAt != 0 {
		started = read.startedAt
	}
	if started > 0 && started <= now {
		spent["sessionMs"] = now - started
	}
	if read != nil {
		running := 0.0
		if desk.turnStartedAt > 0 {
			running = max(0, now-desk.turnStartedAt)
		}
		if read.activeMs > 0 || running > 0 {
			spent["activeMs"] = read.activeMs + running
		}
		if desk.startedAt > 0 && started-desk.startedAt > 60_000 {
			spent["deskMs"] = now - desk.startedAt
		}
	}
	if len(spent) > 0 {
		card["time"] = spent
	}

	// Tokens, and their price: the bill with an API key, what they'd have cost with a subscription.
	if read != nil && read.tokens != (tokenCount{}) {
		t := read.tokens
		card["tokens"] = map[string]any{"input": t.input, "output": t.output, "cacheRead": t.cacheRead, "cacheWrite": t.cacheWrite()}
		// A cloud provider has prices of its own, and a reply with no price here would make the sum wrong.
		if billing != billingCloud && read.unpriced == 0 {
			// Billed as such only where an API key is seen; otherwise what it would cost at the API's prices.
			basis := "equivalent"
			if billing == billingAPIKey {
				basis = "api"
			}
			card["cost"] = map[string]any{"usd": math.Round(read.cost*10_000) / 10_000, "basis": basis}
		}
	}
	return card
}

// sessionInfo answers GET /session/info: session id's account, model, time and tokens, from what this computer has of them.
func (d *Deck) sessionInfo(id string) (int, any) {
	d.mu.Lock()
	s := d.sessions.Get(id)
	var desk infoDesk
	if s != nil {
		desk = infoDesk{id: s.ID, agent: s.Agent, project: s.ProjectName(), projectPath: s.Project, transcript: s.Transcript,
			startedAt: s.StartedAt, model: s.Model, effort: s.Effort}
		if s.IsBusy() {
			desk.turnStartedAt = s.TurnStartedAt
		}
	}
	d.mu.Unlock()
	if s == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	return http.StatusOK, infos.report(desk, nowMillis(), homeDir)
}
