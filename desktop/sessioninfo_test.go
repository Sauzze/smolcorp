package desktop

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ⓘ Info (sessioninfo.go), as the Mac's SessionInfoTests had it: the price table, a transcript's tokens and model as
// the card reads them, the account from Claude Code's profile, and the card itself, which leaves out what it has no
// source for. Then the route, the hook's billing, and the deck's approvals list. Offline.

func infoReply(id, model string, usage map[string]any, effort string, sidechain bool) string {
	entry := map[string]any{"type": "assistant", "timestamp": "2026-10-23T10:00:00.000Z", "isSidechain": sidechain,
		"message": map[string]any{"id": id, "model": model, "role": "assistant", "usage": usage,
			"content": []any{map[string]any{"type": "text", "text": "ok"}}}}
	if effort != "" {
		entry["effort"] = effort
	}
	return jsonLine(entry)
}

func infoTurn(ms int) string {
	return jsonLine(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": ms, "timestamp": "2026-10-23T10:05:00.000Z"})
}

func infoUsage(input, output, read, write5m, write1h int, fast bool) map[string]any {
	usage := map[string]any{"input_tokens": input, "output_tokens": output, "cache_read_input_tokens": read,
		"cache_creation_input_tokens": write5m + write1h,
		"cache_creation":              map[string]any{"ephemeral_5m_input_tokens": write5m, "ephemeral_1h_input_tokens": write1h}}
	if fast {
		usage["speed"] = "fast"
	}
	return usage
}

func near(a, b, within float64) bool { return math.Abs(a-b) <= within }

func priced(t *testing.T, tokens tokenCount, model string, fast bool) float64 {
	t.Helper()
	cost, ok := apiCost(tokens, model, fast)
	if !ok {
		t.Fatalf("no price for %s (fast %v)", model, fast)
	}
	return cost
}

// A finished session's own cost-state on a Mac: 82 in, 31,715 out, 3,519,619 read, 166,201 written for an hour, on
// Opus 5.5, which Claude Code priced at $2.6681598; and Sonnet 5.5's, $0.974259.
func TestThePriceTableGivesWhatClaudeCodeChargedForASession(t *testing.T) {
	if got := priced(t, tokenCount{input: 82, output: 31_715, cacheRead: 3_519_619, write1h: 166_201}, "claude-opus-5-5", false); !near(got, 2.6681598, 1e-5) {
		t.Errorf("opus: %v", got)
	}
	if got := priced(t, tokenCount{input: 90, output: 32_709, cacheRead: 2_708_685, write1h: 94_030}, "claude-sonnet-5-5", false); !near(got, 0.974259, 1e-5) {
		t.Errorf("sonnet: %v", got)
	}
}

func TestCacheWritesFastModeAndLongPrompts(t *testing.T) {
	// 5-minute writes at 1.25x the input price, hour-long ones at 2x.
	for _, c := range []struct {
		tokens tokenCount
		model  string
		fast   bool
		want   float64
	}{
		{tokenCount{write5m: 1_000_000}, "claude-opus-5", false, 6.25},
		{tokenCount{write1h: 1_000_000}, "claude-opus-5", false, 10},
		// Fast mode, twice the price, where it has one.
		{tokenCount{output: 1_000_000}, "claude-opus-5-5", true, 40},
		// Haiku 5.5 over 100K prompt tokens is priced higher, cache reads included.
		{tokenCount{input: 100_000, output: 1_000_000}, "claude-haiku-5-5", false, 0.51},
		{tokenCount{input: 1, output: 1_000_000, cacheRead: 100_000}, "claude-haiku-5-5", false, 2.5 + 0.005 + 0.0000005},
	} {
		if got := priced(t, c.tokens, c.model, c.fast); !near(got, c.want, 1e-9) {
			t.Errorf("%+v on %s: %v, want %v", c.tokens, c.model, got, c.want)
		}
	}
	if _, ok := apiCost(tokenCount{output: 1_000_000}, "claude-sonnet-5-5", true); ok {
		t.Error("fast mode has no price on Sonnet 5.5")
	}
}

func TestModelsAreFoundWithoutTheirSuffixesAndUnknownOnesHaveNoPrice(t *testing.T) {
	if priceID("claude-opus-5-5[1m]") != "claude-opus-5-5" || priceID("claude-haiku-4-5-20251001") != "claude-haiku-4-5" {
		t.Error(priceID("claude-opus-5-5[1m]"), priceID("claude-haiku-4-5-20251001"))
	}
	if apiPrices["claude-fable-5-1"].cacheRead != 0.25 || apiPrices["claude-opus-5"] == apiPrices["claude-opus-5-5"] {
		t.Error("the table")
	}
	for _, model := range []string{"gpt-5", "claude-opus-9"} {
		if _, ok := apiCost(tokenCount{input: 1}, model, false); ok {
			t.Errorf("%s priced", model)
		}
	}
}

func TestModelNames(t *testing.T) {
	for model, want := range map[string]string{"claude-opus-5-5": "Opus 5.5", "claude-opus-5-5[1m]": "Opus 5.5 · 1M",
		"claude-haiku-4-5-20251001": "Haiku 4.5", "claude-fable-5-1": "Fable 5.1", "gpt-5.5-codex": "gpt-5.5-codex"} {
		if got := modelName(model); got != want {
			t.Errorf("%s: %q, want %q", model, got, want)
		}
	}
}

func followed(path string) tally {
	f := &transcriptFollower{path: path}
	f.update()
	return f.tally
}

func TestATranscriptWithSeveralModelsCountsEachReplyOnceAndShowsTheLatestModel(t *testing.T) {
	opus := infoUsage(10, 1_000, 100_000, 0, 20_000, false)
	haiku := infoUsage(500, 50, 0, 0, 0, false)
	path := transcriptFile(t,
		jsonLine(map[string]any{"type": "file-history-snapshot", "snapshot": map[string]any{}}),
		jsonLine(map[string]any{"type": "user", "timestamp": "2026-10-23T09:00:00.000Z", "message": map[string]any{"role": "user", "content": "hi"}}),
		// A reply is written once for each of its blocks, all with the same usage.
		infoReply("m1", "claude-opus-5-5", opus, "high", false),
		infoReply("m1", "claude-opus-5-5", opus, "high", false),
		infoReply("m2", "claude-haiku-5-5", haiku, "", true),
		// Claude Code's own notice isn't a reply.
		infoReply("m3", "<synthetic>", infoUsage(0, 999, 0, 0, 0, false), "", false),
		infoTurn(90_000),
		infoReply("m4", "claude-sonnet-5-5", infoUsage(5, 200, 0, 0, 0, false), "medium", false),
		infoTurn(30_000),
	)
	got := followed(path)
	if got.tokens != (tokenCount{input: 515, output: 1_250, cacheRead: 100_000, write1h: 20_000}) || got.model != "claude-sonnet-5-5" ||
		got.effort != "medium" || got.activeMs != 120_000 || got.unpriced != 0 {
		t.Fatalf("%+v", got)
	}
	if started, _ := transcriptTime("2026-10-23T09:00:00.000Z"); got.startedAt != started {
		t.Errorf("started %v", got.startedAt)
	}
	want := priced(t, tokenCount{input: 10, output: 1_000, cacheRead: 100_000, write1h: 20_000}, "claude-opus-5-5", false) +
		priced(t, tokenCount{input: 500, output: 50}, "claude-haiku-5-5", false) +
		priced(t, tokenCount{input: 5, output: 200}, "claude-sonnet-5-5", false)
	if !near(got.cost, want, 1e-12) {
		t.Errorf("cost %v, want %v", got.cost, want)
	}
}

func TestMissingFieldsCountAsNothingAndAnUnpricedModelIsCounted(t *testing.T) {
	path := transcriptFile(t,
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"id": "a", "model": "claude-opus-5-5"}}),
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"id": "b", "model": "claude-opus-5-5",
			"usage": map[string]any{"output_tokens": 7, "cache_creation_input_tokens": 3}}}),
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"id": "c", "model": "claude-opus-9", "usage": map[string]any{"output_tokens": 1}}}),
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"model": "claude-opus-5-5"}}),
		"not json at all",
		jsonLine(map[string]any{"type": "system", "subtype": "turn_duration"}),
	)
	// A write with no split counts as 5 minutes'.
	if got := followed(path); got.tokens != (tokenCount{output: 8, write5m: 3}) || got.unpriced != 1 || got.activeMs != 0 ||
		got.startedAt != 0 || got.model != "claude-opus-5-5" {
		t.Fatalf("%+v", got)
	}
}

func TestOnlyWhatsAddedIsReadAndALineIsTakenOnceItEnds(t *testing.T) {
	path := transcriptFile(t, infoReply("m1", "claude-opus-5-5", infoUsage(0, 100, 0, 0, 0, false), "", false))
	f := &transcriptFollower{path: path}
	output := func() int { f.update(); return f.tally.tokens.output }
	appendTo := func(text string) {
		file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		_, _ = file.WriteString(text)
		file.Close()
	}
	if got := output(); got != 100 {
		t.Fatalf("%d", got)
	}
	// Half a line: not yet.
	line := infoReply("m2", "claude-opus-5-5", infoUsage(0, 20, 0, 0, 0, false), "", false)
	appendTo(line[:len(line)/2])
	if got := output(); got != 100 {
		t.Fatalf("half a line: %d", got)
	}
	appendTo(line[len(line)/2:] + "\n")
	if got, again := output(), output(); got != 120 || again != 120 {
		t.Fatalf("the whole line: %d then %d", got, again)
	}
	// A file written again from the start is read again.
	_ = os.WriteFile(path, []byte(infoReply("m9", "claude-opus-5-5", infoUsage(0, 5, 0, 0, 0, false), "", false)+"\n"), 0o600)
	if got := output(); got != 5 {
		t.Fatalf("written again: %d", got)
	}
}

func TestATooLongLineIsSkipped(t *testing.T) {
	huge := jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "content": strings.Repeat("x", followerLineMost+10) + `"assistant"`}}}})
	if got := followed(transcriptFile(t, huge, infoReply("m1", "claude-opus-5-5", infoUsage(0, 3, 0, 0, 0, false), "", false))); got.tokens.output != 3 {
		t.Fatalf("%+v", got.tokens)
	}
}

const infoNow = 1_792_400_000_000.0

func infoISO(ms float64) string { return time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339) }

func infoProfile(fetchedAgo float64, owner string, fiveResetsIn float64) map[string]any {
	data, _ := json.Marshal(map[string]any{
		"oauthAccount": map[string]any{"accountUuid": "acct-1", "emailAddress": "me@example.com", "displayName": "Me",
			"organizationType": "claude_max", "organizationRateLimitTier": "default_claude_max_20x", "billingType": "stripe_subscription"},
		"cachedUsageUtilization": map[string]any{"accountUuid": owner, "fetchedAtMs": infoNow - fetchedAgo, "utilization": map[string]any{
			"five_hour": map[string]any{"utilization": 23, "resets_at": infoISO(infoNow + fiveResetsIn)},
			"seven_day": map[string]any{"utilization": 41, "resets_at": infoISO(infoNow + 86_400_000)},
		}},
		"primaryApiKey": "must never be read"})
	var profile map[string]any
	_ = json.Unmarshal(data, &profile)
	return profile
}

func TestASubscriptionWithItsUseAsClaudeCodeLastFetchedIt(t *testing.T) {
	account, ok := claudeAccountFrom(infoProfile(120_000, "acct-1", 3_600_000), infoNow)
	if !ok || account.name != "me@example.com" || account.plan != "Max 20x" || !account.subscription || account.fiveHour == nil ||
		account.fiveHour.percent != 23 || account.weekly == nil || account.weekly.percent != 41 || account.usageAt != infoNow-120_000 ||
		!near(account.fiveHour.resetsAt, infoNow+3_600_000, 1000) {
		t.Fatalf("%+v", account)
	}
}

func TestUseThatsStaleResetOrAnotherAccountsIsLeftOut(t *testing.T) {
	stale, _ := claudeAccountFrom(infoProfile(usageFresh+1, "acct-1", 3_600_000), infoNow)
	if stale.fiveHour != nil || stale.weekly != nil || stale.usageAt != 0 {
		t.Errorf("stale: %+v", stale)
	}
	if other, _ := claudeAccountFrom(infoProfile(120_000, "someone-else", 3_600_000), infoNow); other.weekly != nil {
		t.Error("another account's use")
	}
	reset, _ := claudeAccountFrom(infoProfile(120_000, "acct-1", -1000), infoNow)
	if reset.fiveHour != nil || reset.weekly == nil || reset.weekly.percent != 41 {
		t.Errorf("reset: %+v", reset)
	}
	// Read earlier, looked at later: what's reset meanwhile goes.
	earlier, _ := claudeAccountFrom(infoProfile(120_000, "acct-1", 3_600_000), infoNow)
	if later := agedAccount(earlier, infoNow+3_700_000); later.fiveHour != nil || later.weekly != nil {
		t.Errorf("later: %+v", later)
	}
}

func TestNoLoginNoAccount(t *testing.T) {
	if _, ok := claudeAccountFrom(map[string]any{}, infoNow); ok {
		t.Error("an account with no login")
	}
	if claudePlan("claude_pro", "") != "Pro" || claudePlan("claude_max", "default_claude_max_5x") != "Max 5x" || claudePlan("api", "") != "" {
		t.Error(claudePlan("claude_pro", ""), claudePlan("claude_max", "default_claude_max_5x"), claudePlan("api", ""))
	}
}

func TestTheProfileIsTheOneBesideTheTranscriptsSettings(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"Users", "me")
	in := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	standard := claudeConfigFolder(in(".claude", "projects", "-Users-me-app", "s.jsonl"), home)
	own := claudeConfigFolder(in(".claude-work", "projects", "-x", "s.jsonl"), home)
	if standard != in(".claude") || claudeProfilePath(standard, home) != in(".claude.json") ||
		claudeProfilePath(own, home) != in(".claude-work", ".claude.json") || claudeConfigFolder("", home) != in(".claude") {
		t.Fatalf("%s %s", standard, own)
	}
}

func TestAKeyHelperIsSeenOnlyByBeingThere(t *testing.T) {
	folder := t.TempDir()
	with, without := filepath.Join(folder, "with.json"), filepath.Join(folder, "without.json")
	_ = os.WriteFile(with, []byte(`{"apiKeyHelper": "~/bin/key.sh"}`), 0o600)
	_ = os.WriteFile(without, []byte(`{"model": "opus"}`), 0o600)
	if !hasKeyHelper([]string{without, with}) || hasKeyHelper([]string{without, filepath.Join(folder, "none.json")}) {
		t.Fatal("the key helper")
	}
}

func infoTestDesk(agent Agent) infoDesk {
	return infoDesk{id: "s1", agent: agent, project: "app", projectPath: "/p/app", transcript: "/t.jsonl",
		startedAt: infoNow - 7_200_000, model: "claude-opus-5", effort: "low"}
}

func infoTally() *tally {
	t := &tally{}
	t.take(map[string]any{"type": "user", "timestamp": infoISO(infoNow - 3_600_000)})
	t.take(map[string]any{"type": "assistant", "effort": "high",
		"message": map[string]any{"id": "m", "model": "claude-opus-5-5", "usage": map[string]any{"input_tokens": 10.0, "output_tokens": 1_000.0}}})
	t.take(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 60_000.0})
	return t
}

func TestASubscriptionsCardHasItsAccountUseAndWhatTheTokensWouldHaveCost(t *testing.T) {
	account, _ := claudeAccountFrom(infoProfile(120_000, "acct-1", 3_600_000), infoNow)
	card := infoCard(infoTestDesk(Claude), infoTally(), &account, "", infoNow)
	if !reflect.DeepEqual(card["account"], map[string]any{"kind": "subscription", "name": "me@example.com", "plan": "Max 20x"}) {
		t.Errorf("account %v", card["account"])
	}
	// The latest reply's model and effort, not the hook's.
	if model, _ := card["model"].(map[string]any); model["id"] != "claude-opus-5-5" || model["effort"] != "high" || model["name"] != "Opus 5.5" {
		t.Errorf("model %v", model)
	}
	spent, _ := card["time"].(map[string]any)
	if !near(spent["sessionMs"].(float64), 3_600_000, 1000) || spent["activeMs"] != 60_000.0 || !near(spent["deskMs"].(float64), 7_200_000, 1) {
		t.Errorf("time %v", spent)
	}
	if cost, _ := card["cost"].(map[string]any); cost["basis"] != "equivalent" || !near(cost["usd"].(float64), 0.02, 0.0001) {
		t.Errorf("cost %v", cost)
	}
	limits, _ := card["limits"].(map[string]any)
	if five, _ := limits["fiveHour"].(map[string]any); five["percent"] != 23.0 || limits["ageMs"] != 120_000.0 {
		t.Errorf("limits %v", limits)
	}
	// Nothing of a key, nor the account's id.
	text, _ := json.Marshal(card)
	if strings.Contains(string(text), "never be read") || strings.Contains(string(text), "acct-1") {
		t.Errorf("%s", text)
	}
}

func TestAnAPIKeysCardHasTheCostAndNoSubscription(t *testing.T) {
	account, _ := claudeAccountFrom(infoProfile(120_000, "acct-1", 3_600_000), infoNow)
	card := infoCard(infoTestDesk(Claude), infoTally(), &account, billingAPIKey, infoNow)
	if cost, _ := card["cost"].(map[string]any); !reflect.DeepEqual(card["account"], map[string]any{"kind": "api-key"}) ||
		card["limits"] != nil || cost["basis"] != "api" {
		t.Errorf("%v", card)
	}
	// A cloud provider has its own prices: tokens, but no cost.
	if cloud := infoCard(infoTestDesk(Claude), infoTally(), nil, billingCloud, infoNow); cloud["tokens"] == nil || cloud["cost"] != nil {
		t.Errorf("cloud %v", cloud)
	}
}

func TestWhatHasNoSourceIsLeftOut(t *testing.T) {
	// Another agent: no transcript read, so no tokens, cost or account; its time, from its hooks.
	codex := infoCard(infoDesk{id: "c", agent: Codex, project: "x", projectPath: "/x", startedAt: infoNow - 60_000}, nil, nil, "", infoNow)
	if !reflect.DeepEqual(codex, map[string]any{"agent": Codex.Name(), "desk": "x", "time": map[string]any{"sessionMs": 60_000.0}}) {
		t.Errorf("codex %v", codex)
	}
	// A reply with no price: the tokens, not a wrong sum.
	unpriced := infoTally()
	unpriced.take(map[string]any{"type": "assistant", "message": map[string]any{"id": "x", "model": "claude-opus-9", "usage": map[string]any{"output_tokens": 1.0}}})
	if card := infoCard(infoTestDesk(Claude), unpriced, nil, "", infoNow); card["tokens"] == nil || card["cost"] != nil || card["account"] != nil || card["limits"] != nil {
		t.Errorf("unpriced %v", card)
	}
	// A session as old as its desk has no separate desk time.
	fresh := infoTestDesk(Claude)
	fresh.startedAt = infoNow - 3_600_000
	if spent, _ := infoCard(fresh, infoTally(), nil, "", infoNow)["time"].(map[string]any); spent["deskMs"] != nil {
		t.Errorf("desk time %v", spent)
	}
}

// The route, as the conversation's: the office's own window only. Its card reads the transcript, the profile beside
// the transcript's settings (here, the test's home) and the hook's billing, which the hook says from its environment.
func TestOnlyTheOfficesOwnWindowReadsTheInfo(t *testing.T) {
	d := newTestDeck(t)
	phone := pair(t, d)
	profile, _ := json.Marshal(infoProfile(120_000, "acct-1", 3_600_000))
	_ = os.WriteFile(filepath.Join(homeDir, ".claude.json"), profile, 0o600)
	path := transcriptFile(t, infoReply("m1", "claude-opus-5-5", infoUsage(10, 1_000, 0, 0, 0, false), "high", false))
	hook := func(billing string) {
		r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(jsonLine(map[string]any{"hook_event_name": "UserPromptSubmit",
			"session_id": "s1", "cwd": "/work/api", "transcript_path": path})))
		r.Host, r.RemoteAddr = "127.0.0.1:7710", thisComputer
		r.Header.Set(hookKeyHeader, d.access.HookKey)
		r.Header.Set("X-AgentDeck-Agent", "claude")
		r.Header.Set("X-AgentDeck-Tab", "tty:/dev/ttys003")
		if billing != "" {
			r.Header.Set("X-AgentDeck-Billing", billing)
		}
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("the hook: %d", w.Code)
		}
	}
	card := func(query string) (int, map[string]any) {
		w := serve(t, d, call{method: "GET", target: "/session/info?" + query + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		var answer map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &answer)
		return w.Code, answer
	}
	hook("")
	code, answer := card("id=s1")
	if model, _ := answer["model"].(map[string]any); code != http.StatusOK || answer["desk"] != "api" || model["name"] != "Opus 5.5" ||
		!reflect.DeepEqual(answer["account"], map[string]any{"kind": "subscription", "name": "me@example.com", "plan": "Max 20x"}) {
		t.Fatalf("%d %v", code, answer)
	}
	// From the hook: an API key in the agent's environment wins over the profile's login; then without it, it's gone.
	hook(billingAPIKey)
	if code, answer := card("session=s1"); code != http.StatusOK || !reflect.DeepEqual(answer["account"], map[string]any{"kind": "api-key"}) {
		t.Errorf("api key: %d %v", code, answer)
	}
	hook("")
	if _, answer := card("id=s1"); answer["account"].(map[string]any)["kind"] != "subscription" {
		t.Errorf("the key gone: %v", answer)
	}
	if code, _ := card("id=gone"); code != http.StatusNotFound {
		t.Errorf("no agent: %d", code)
	}
	refused := serve(t, d, call{method: "GET", target: "/session/info?id=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if refused.Code != http.StatusForbidden || strings.TrimSpace(refused.Body.String()) != "The info shows only in the office's window on your computer." {
		t.Errorf("a phone: %d %s", refused.Code, refused.Body)
	}
}

func TestTheHookSaysHowTheAgentPaysAndNeverTheKey(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, ""},
		{map[string]string{"ANTHROPIC_API_KEY": "sk-fake"}, billingAPIKey},
		{map[string]string{"ANTHROPIC_AUTH_TOKEN": "fake"}, billingAPIKey},
		{map[string]string{"ANTHROPIC_API_KEY": "sk-fake", "CLAUDE_CODE_USE_BEDROCK": "1"}, billingCloud},
		{map[string]string{"CLAUDE_CODE_USE_VERTEX": "1"}, billingCloud},
		{map[string]string{"CLAUDE_CODE_USE_FOUNDRY": "1"}, billingCloud},
	} {
		if got := hookBilling(func(name string) string { return c.env[name] }); got != c.want {
			t.Errorf("%v: %q, want %q", c.env, got, c.want)
		}
	}
}

// The deck's approvals list, as the Mac's: a paired phone through `tailscale serve`, never over the Wi-Fi, from this
// computer without the proxy, nor to the office's window. It lists who's asking, as /setup/gm/approvals does.
func TestOnlyAPairedPhoneOnTheTailnetReadsTheApprovals(t *testing.T) {
	saved := macTailscale
	t.Cleanup(func() { macTailscale = saved })
	macTailscale = true
	d, _ := phoneDeck(t)
	phone := pair(t, d)
	asksPermission(d, 30)
	ask := func(host, from, key string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "GET", target: "/approvals?t=" + key, host: host, from: from})
	}
	got := ask("my-mac.tail1234.ts.net", thisComputer, phone)
	var answer struct {
		Approvals []map[string]any
		At        float64
	}
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || len(answer.Approvals) != 1 ||
		answer.Approvals[0]["session"] != "s1" || answer.Approvals[0]["label"] != "Bash: touch" || answer.At == 0 {
		t.Fatalf("through Tailscale: %d %s", got.Code, got.Body)
	}
	for _, c := range []struct{ name, host, from, key string }{
		{"the Wi-Fi", deckOnWiFi, aPhone, phone},
		{"this computer without the proxy", "localhost:7710", thisComputer, phone},
		{"the office's window", "localhost:7710", thisComputer, d.access.WindowKey},
	} {
		if got := ask(c.host, c.from, c.key); got.Code != http.StatusForbidden ||
			strings.TrimSpace(got.Body.String()) != "The approvals show only over Tailscale's HTTPS address." {
			t.Errorf("%s: %d %s", c.name, got.Code, got.Body)
		}
	}
	if got := serve(t, d, call{method: "POST", target: "/approvals?t=" + phone, host: "my-mac.tail1234.ts.net", from: thisComputer}); got.Code != http.StatusNotFound {
		t.Errorf("a POST: %d", got.Code)
	}
}

// The roles' limits in /setup/state, as the Mac sent them: a brief file's size and name, and an answer's length.
func TestSetupStateSaysABriefsLimits(t *testing.T) {
	d := newTestDeck(t)
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	var state struct {
		Roles struct{ Limits map[string]int }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if l := state.Roles.Limits; l["briefFile"] != 32*1024 || l["briefFileName"] != 100 || l["answer"] != 2000 {
		t.Fatalf("%v", l)
	}
}
