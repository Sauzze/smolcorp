package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// MARK: The desk

func TestAPermanentBotIsBlueThenGreenForTenMinutesOnAReport(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	desk := newCloudDesk(true)
	if state := desk.state(start); state != Thinking {
		t.Fatalf("a permanent bot at rest shows %s, want blue", state)
	}
	if next := desk.nextChange(start); !next.IsZero() {
		t.Errorf("a blue desk changes by itself at %v", next)
	}
	desk.apply(cloudReport, start, true, nil)
	for _, after := range []time.Duration{0, 599 * time.Second} {
		if state := desk.state(start.Add(after)); state != Unread {
			t.Errorf("%v after a report it shows %s, want green", after, state)
		}
	}
	if next := desk.nextChange(start); !next.Equal(start.Add(cloudGreenFor)) {
		t.Errorf("its green ends at %v, want %v", next, start.Add(cloudGreenFor))
	}
	if state := desk.state(start.Add(600 * time.Second)); state != Thinking {
		t.Errorf("ten minutes after a report it shows %s, want blue", state)
	}
}

func TestATapEndsTheGreenAndANewReportStartsItAgain(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	desk := newCloudDesk(true)
	desk.apply(cloudReport, start, true, nil)
	desk.apply(cloudSeen, start.Add(5*time.Second), true, nil)
	if state := desk.state(start.Add(6 * time.Second)); state != Thinking {
		t.Errorf("seen, it shows %s, want blue", state)
	}
	later := start.Add(time.Hour)
	desk.apply(cloudReport, later, true, nil)
	if state := desk.state(later.Add(5 * time.Minute)); state != Unread {
		t.Errorf("after a new report it shows %s, want green", state)
	}
}

func TestAPermanentBotNeverLeavesAndAVisitorComesAndGoes(t *testing.T) {
	now := time.Now()
	permanent := newCloudDesk(true)
	permanent.apply(cloudStop, now, true, nil)
	if state := permanent.state(now); state != Thinking {
		t.Errorf("a permanent bot told to stop shows %s, want blue", state)
	}

	visitor := newCloudDesk(false)
	for _, step := range []struct {
		event string
		want  SessionState
		why   string
	}{{"", Idle, "away until it starts"}, {cloudStart, Thinking, "at its desk"}, {cloudStop, Idle, "gone"},
		{cloudReport, Unread, "a report brings it to its desk"}} {
		if step.event != "" {
			visitor.apply(step.event, now, false, nil)
		}
		if state := visitor.state(now); state != step.want {
			t.Errorf("a visitor: %s, want %s (%s)", state, step.want, step.why)
		}
	}
}

// MARK: Its last action

func TestAnOldReportBodyStillWorks(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	action, err := parseCloudAction(map[string]any{"type": "report"}, now)
	if err != nil || action != (CloudAction{Text: "Sent a report", Count: 1, At: now}) {
		t.Errorf("the old body reads as %+v, %v", action, err)
	}
}

func TestAReportSaysWhatHowManyAndWhen(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	var body map[string]any
	if err := json.Unmarshal([]byte(`{"type": "report", "text": "  Drafted <b>3</b> replies\n\u0007 ", "count": 3, "at": "2026-10-28T22:20:00Z"}`), &body); err != nil {
		t.Fatal(err)
	}
	action, err := parseCloudAction(body, now)
	if err != nil {
		t.Fatal(err)
	}
	if action.Text != "Drafted 3 replies" {
		t.Errorf("the text is %q: want plain text, with no markup, control characters or new lines", action.Text)
	}
	if text := newCloudAction("3 > 2, <script", 1, now).Text; text != "3  2, script" {
		t.Errorf("brackets left as %q", text)
	}
	if want := time.Date(2026, 10, 28, 22, 20, 0, 0, time.UTC); action.Count != 3 || !action.At.Equal(want) {
		t.Errorf("read %d at %v, want 3 at %v", action.Count, action.At, want)
	}
	if fractional, err := parseCloudAction(map[string]any{"at": "2026-10-28T22:20:00.250+01:00"}, now); err != nil ||
		!fractional.At.Equal(time.Date(2026, 10, 28, 21, 20, 0, 250_000_000, time.UTC)) {
		t.Errorf("fractional seconds and offsets read as %v, %v", fractional.At, err)
	}

	long, _ := parseCloudAction(map[string]any{"text": strings.Repeat("x", 500), "count": 5000.0}, now)
	if len(long.Text) != cloudTextLimit || long.Count != cloudCountMost {
		t.Errorf("a long report kept %d characters and a count of %d", len(long.Text), long.Count)
	}
	if blank, _ := parseCloudAction(map[string]any{"text": "   "}, now); blank.Text != "Sent a report" {
		t.Errorf("a blank text is %q", blank.Text)
	}
	if future, _ := parseCloudAction(map[string]any{"at": "2099-01-01T00:00:00Z"}, now); !future.At.Equal(now) {
		t.Errorf("a time to come is %v, want now", future.At)
	}
	if nulls, err := parseCloudAction(map[string]any{"text": nil, "count": nil, "at": nil}, now); err != nil || nulls != newCloudAction(cloudReportText, 1, now) {
		t.Errorf("nulls read as %+v, %v", nulls, err)
	}
}

func TestABadPartOfAReportIsRefused(t *testing.T) {
	now := time.Now()
	for _, body := range []map[string]any{{"text": 3.0}, {"count": 0.0}, {"count": 2.5}, {"count": "3"}, {"count": true},
		{"at": "yesterday"}, {"at": 1_793_000_000.0}} {
		if _, err := parseCloudAction(body, now); err == nil {
			t.Errorf("took %v", body)
		}
	}
}

func TestReportsAddUpTillSeenAndTheCardKeepsTheLast(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	desk := newCloudDesk(true)
	if desk.last != nil {
		t.Fatal("a new desk has a last action")
	}
	desk.apply(cloudReport, start, true, nil)
	if *desk.last != newCloudAction(cloudReportText, 1, start) {
		t.Errorf("the old body's report is %+v, want one report, now", *desk.last)
	}
	sent := start.Add(-time.Minute)
	arrived := start.Add(10 * time.Second)
	given := newCloudAction("Drafted replies", 2, sent)
	desk.apply(cloudReport, arrived, true, &given)
	if *desk.last != newCloudAction("Drafted replies", 3, sent) {
		t.Errorf("the second report is %+v, want 3 drafted replies", *desk.last)
	}
	if desk.state(arrived) != Unread || !desk.nextChange(arrived).Equal(arrived.Add(cloudGreenFor)) {
		t.Error("green should last from when it arrived, not the time it says")
	}

	// Past its green, still unseen: they keep adding up.
	late := start.Add(time.Hour)
	desk.apply(cloudReport, late, true, nil)
	if desk.last.Count != 4 {
		t.Errorf("a late report unseen counts %d, want 4", desk.last.Count)
	}

	desk.apply(cloudSeen, late.Add(time.Second), true, nil)
	if desk.state(late.Add(time.Second)) != Thinking || desk.last.Count != 4 {
		t.Errorf("seen, it shows %s and counts %d: want blue, the card still saying what it last did", desk.state(late), desk.last.Count)
	}
	desk.apply(cloudReport, late.Add(time.Minute), true, nil)
	if *desk.last != newCloudAction(cloudReportText, 1, late.Add(time.Minute)) {
		t.Errorf("after seen, counting starts again: %+v", *desk.last)
	}
}

func TestOpeningACloudCardMarksItSeenAndNothingElse(t *testing.T) {
	desks := newCloudDesks(checkedCloudBots([]CloudBot{{ID: "smolcorp-mini", Name: "Smolcorp mini", Permanent: true}}), nil)
	changes := 0
	desks.onChange = func() { changes++ }
	id := "cloud-smolcorp-mini"
	if desks.last(id) != nil {
		t.Fatal("a new desk has a last action")
	}
	given := newCloudAction(cloudReportText, 2, time.Now())
	desks.apply(cloudReport, "smolcorp-mini", &given)
	if seated, _ := desks.seated(); len(seated) != 1 || seated[0].State != Unread {
		t.Fatalf("after a report upstairs is %v", seated)
	}
	desks.apply(cloudReport, "smolcorp-mini", nil)
	if changes != 2 {
		t.Errorf("%d changes: a second report, still green, changes the card", changes)
	}
	if last := desks.last(id); last == nil || last.Count != 3 {
		t.Errorf("the card says %+v, want 3 reports", last)
	}

	if !desks.seen(id) {
		t.Fatal("its card opened isn't seen")
	}
	if seated, _ := desks.seated(); seated[0].State != Thinking || desks.last(id).Count != 3 {
		t.Errorf("seen, it shows %s with %d reports: want blue at once, the card unchanged", seated[0].State, desks.last(id).Count)
	}
	if desks.seen("some-terminal-session") || desks.seen("smolcorp-mini") {
		t.Error("seen took a desk that isn't a cloud bot's, by its desk's session id")
	}
}

// MARK: The bots as set

func TestBotsAreCheckedAsTheyAreKept(t *testing.T) {
	var given []CloudBot
	if err := json.Unmarshal([]byte(`[{"id": "Smolcorp-Mini", "name": "Smolcorp mini", "source": {"kind": "grokBotApp"}},
		{"id": "smolcorp-mini", "name": "Twice"},
		{"id": "bad id!", "name": "Nope"},
		{"id": "x", "name": "  "},
		{"id": "other", "name": "a/b", "outfit": "nonsense", "badge": "megaphone", "agent": "who", "permanent": false, "floor": "roof",
		 "source": {"kind": "mail", "counts": "all"}}]`), &given); err != nil {
		t.Fatal(err)
	}
	bots := checkedCloudBots(given)
	if len(bots) != 2 || bots[0].ID != "smolcorp-mini" || bots[1].ID != "other" {
		t.Fatalf("kept %+v", bots)
	}
	want := CloudBot{ID: "smolcorp-mini", Name: "Smolcorp mini", Agent: "grok", Outfit: "grok", Permanent: true, Floor: "",
		Source: CloudSource{Kind: "grokBotApp", Bot: "Smolcorp mini", Counts: "unprompted"}}
	if bots[0] != want {
		t.Errorf("the first is\n%+v\nwant\n%+v", bots[0], want)
	}
	if bots[0].sessionID() != "cloud-smolcorp-mini" || bots[0].folder() != "/cloud/Smolcorp mini" {
		t.Errorf("its desk is %s in %s", bots[0].sessionID(), bots[0].folder())
	}
	want = CloudBot{ID: "other", Name: "a-b", Agent: "grok", Outfit: "", Badge: "megaphone", Permanent: false, Floor: "",
		Source: standardCloudSource()}
	if bots[1] != want {
		t.Errorf("the second is\n%+v\nwant\n%+v: a floor there isn't is where new bots go", bots[1], want)
	}
	if ground, _ := (CloudBot{ID: "g", Name: "G", Floor: "ground"}).checked(); ground.Floor != "ground" {
		t.Errorf("the ground floor became %s", ground.Floor)
	}
	var many []CloudBot
	for i := range cloudBotsMost + 3 {
		many = append(many, CloudBot{ID: fmt.Sprintf("b%d", i), Name: "Bot"})
	}
	if kept := checkedCloudBots(many); len(kept) != cloudBotsMost {
		t.Errorf("kept %d of %d bots, want %d", len(kept), len(many), cloudBotsMost)
	}
	for _, bad := range []string{`{"id": 3, "name": "x"}`, `{"id": "x", "permanent": "yes"}`, `{"id": "x", "source": "none"}`, `"x"`, `null`} {
		var bot CloudBot
		if json.Unmarshal([]byte(bad), &bot) == nil {
			t.Errorf("read %s as a bot", bad)
		}
	}
}

func TestItsRoleDressesItWithoutAName(t *testing.T) {
	bot, _ := standardCloudBotNamed("a", "A").checked()
	if role := bot.role(); role["outfit"] != "grok" || role["name"] != "" {
		t.Errorf("its role is %v: want the grok outfit, and no name, so the desk says only the bot's own", role)
	}
	if bot.hirer() != "Grok Bots" {
		t.Errorf("a Grok Bot is hired by %q", bot.hirer())
	}
	codex := standardCloudBotNamed("b", "B")
	codex.Agent = "codex"
	if codex.hirer() != "" {
		t.Errorf("a Codex bot is hired by %q", codex.hirer())
	}
}

func standardCloudBotNamed(id, name string) CloudBot {
	bot := standardCloudBot()
	bot.ID, bot.Name = id, name
	return bot
}

func TestCloudBotsAreKeptInTheConfigAndABadOneLosesOnlyItself(t *testing.T) {
	isolate(t)
	written := `{"hireFolder": "/work", "cloudBots": [{"id": "mini", "name": "Mini", "badge": "crown", "floor": "ground"},
		{"id": "bad", "name": "Bad", "permanent": "yes"}, 7, {"id": "visitor", "name": "Visitor", "permanent": false}]}`
	if err := os.WriteFile(configFile, []byte(written), 0o600); err != nil {
		t.Fatal(err)
	}
	config := loadConfig()
	if config.HireFolder != "/work" {
		t.Errorf("the rest of the file was lost: %+v", config)
	}
	if len(config.CloudBots) != 2 || config.CloudBots[0].ID != "mini" || config.CloudBots[1].ID != "visitor" {
		t.Fatalf("kept %+v", config.CloudBots)
	}
	if mini := config.CloudBots[0]; mini.Badge != "crown" || mini.Floor != "ground" || !mini.Permanent || mini.Outfit != "grok" {
		t.Errorf("the first bot loads as %+v", mini)
	}
	config.save()
	if again := loadConfig(); len(again.CloudBots) != 2 || again.CloudBots[0] != config.CloudBots[0] || again.CloudBots[1] != config.CloudBots[1] {
		t.Errorf("saved and loaded, the bots are %+v, want %+v", again.CloudBots, config.CloudBots)
	}
	isolate(t)
	if fresh := loadConfig(); fresh.CloudBots == nil || len(fresh.CloudBots) != 0 {
		t.Errorf("a new config's bots are %#v, want none, as a list", fresh.CloudBots)
	}
}

// MARK: In the office

// withCloudBots sets bots as setup does.
func withCloudBots(d *Deck, bots ...CloudBot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config.CloudBots = checkedCloudBots(bots)
	d.cloud.configure(d.config.CloudBots)
}

// reported tells bot id's desk of a report, as its event route does.
func reported(d *Deck, id string, action *CloudAction) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cloud.apply(cloudReport, id, action)
}

func deckState(t *testing.T, d *Deck) (keys []map[string]any, upstairs []map[string]any, overflow float64) {
	t.Helper()
	d.mu.Lock()
	data := d.snapshot()
	d.mu.Unlock()
	var state struct {
		Keys     []map[string]any
		Upstairs struct {
			Desks    []map[string]any
			Overflow float64
		}
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state.Keys, state.Upstairs.Desks, state.Upstairs.Overflow
}

func TestACloudBotSitsUpstairsFirstOrAtAFreeKey(t *testing.T) {
	d := newTestDeck(t)
	running(d, 1)
	headless(d, desksPerFloor)
	upstairs := standardCloudBotNamed("mini", "Smolcorp mini")
	upstairs.Badge = "crown"
	ground := standardCloudBotNamed("helper", "Helper")
	ground.Floor, ground.Agent, ground.Outfit = "ground", "codex", "auto"
	visitor := standardCloudBotNamed("visitor", "Visitor")
	visitor.Permanent = false
	withCloudBots(d, upstairs, ground, visitor)

	keys, desks, overflow := deckState(t, d)
	if len(desks) != desksPerFloor || overflow != 1 {
		t.Fatalf("%d desks upstairs and %d over, want %d and 1: the bot first, the runs after", len(desks), int(overflow), desksPerFloor)
	}
	bot := desks[0]
	if bot["id"] != "cloud-mini" || bot["cloud"] != true || bot["name"] != "Smolcorp mini" || bot["state"] != "thinking" || bot["last"] != nil {
		t.Errorf("the bot's desk upstairs is %v", bot)
	}
	for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary", "phoneMic", "conversation"} {
		if bot[control] != false {
			t.Errorf("its %s is %v, want off", control, bot[control])
		}
	}
	if role, _ := bot["role"].(map[string]any); role["id"] != "cloud" || role["name"] != "" || role["outfit"] != "grok" || role["badge"] != "crown" {
		t.Errorf("it's dressed as %v", bot["role"])
	}
	if hired, _ := bot["hiredBy"].(map[string]any); hired["name"] != "Grok Bots" || hired["id"] != nil {
		t.Errorf("its card says it was hired by %v", bot["hiredBy"])
	}
	if desks[1]["cloud"] != nil || desks[1]["headless"] != true {
		t.Errorf("a headless run upstairs is %v", desks[1])
	}
	if len(keys) != 2 {
		t.Fatalf("%d keys, want 2: the session's and the bot on the ground floor", len(keys))
	}
	helper, _ := keys[1]["session"].(map[string]any)
	if helper["id"] != "cloud-helper" || helper["cloud"] != true || helper["agent"] != "codex" || helper["hiredBy"] != nil {
		t.Errorf("the ground floor's bot is %v", helper)
	}
	if role, _ := helper["role"].(map[string]any); role["outfit"] != "" {
		t.Errorf("an auto outfit is %v, want its agent's own", role["outfit"])
	}

	// A visitor sits down to report: 2F, where new bots go, is full, so where there's room, and nobody upstairs moves
	// for it, as nobody put it on 2F; its card says what it did.
	replied := newCloudAction("Replied", 2, time.Now())
	reported(d, "visitor", &replied)
	_, desks, _ = deckState(t, d)
	if desks[1]["id"] != "h0" {
		t.Errorf("after the visitor came, the second desk upstairs is %v, want the run that sat there", desks[1]["id"])
	}
	third := floorDesks(t, d, 2)
	if len(third) != 2 || third[1]["id"] != "cloud-visitor" || third[1]["state"] != "unread" {
		t.Fatalf("after its report, 3F is %v", third)
	}
	if last, _ := third[1]["last"].(map[string]any); last["text"] != "Replied" || last["count"] != 2.0 || last["at"] == nil {
		t.Errorf("its card says %v", third[1]["last"])
	}
}

// floorDesks is the desks the phone is told of on floor n, 0 for the ground floor.
func floorDesks(t *testing.T, d *Deck, n int) []map[string]any {
	t.Helper()
	d.mu.Lock()
	data := d.snapshot()
	d.mu.Unlock()
	var state struct {
		Floors []struct{ Desks []map[string]any }
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state.Floors[n].Desks
}

// The portable office's viewer sees a cloud bot's desk first upstairs, as the
// deck does: that it is one, what it last did and who it works for; with
// names hidden, neither its name nor its own words.
func TestTheViewerSeesACloudBotsDeskAsTheDeckDoes(t *testing.T) {
	d := newTestDeck(t)
	headless(d, 1)
	withCloudBots(d, standardCloudBotNamed("mini", "Smolcorp mini"))
	replied := newCloudAction("Replied to a secret thread", 2, time.Now())
	reported(d, "mini", &replied)
	for _, hideNames := range []bool{false, true} {
		d.mu.Lock()
		keys, overflow := d.layout()
		upstairs := d.viewerUpstairs(keys, overflow, hideNames)
		d.mu.Unlock()
		desks, _ := upstairs["desks"].([]map[string]any)
		if len(desks) != 2 {
			t.Fatalf("names hidden %v: %d desks upstairs, want the bot's and the run's", hideNames, len(desks))
		}
		bot := desks[0]
		name, text := "Smolcorp mini", "Replied to a secret thread"
		if hideNames {
			name, text = "Upstairs 1", cloudReportText
		}
		last, _ := bot["last"].(map[string]any)
		if bot["cloud"] != true || bot["name"] != name || last["text"] != text || last["count"] != 2 {
			t.Errorf("names hidden %v: the bot's desk is %v", hideNames, bot)
		}
		if hired, _ := bot["hiredBy"].(map[string]any); hired["name"] != "Grok Bots" {
			t.Errorf("names hidden %v: its card says it was hired by %v", hideNames, bot["hiredBy"])
		}
		if desks[1]["cloud"] != nil {
			t.Errorf("names hidden %v: the run upstairs is %v", hideNames, desks[1])
		}
	}
}

// A Grok Bot's id in its app is kept as the Mac app keeps it, so a config
// goes back and forth; one that can't be an id isn't.
func TestAGrokBotsIDIsKeptAsTheMacKeepsIt(t *testing.T) {
	var bots []CloudBot
	if err := json.Unmarshal([]byte(`[{"id": "mini", "name": "Mini", "source": {"kind": "grokBotApp", "bot": "Mini", "botId": "0b7e-41AF"}},
		{"id": "two", "name": "Two", "source": {"kind": "grokBotApp", "botId": "../../etc"}}, {"id": "three", "name": "Three"}]`), &bots); err != nil {
		t.Fatal(err)
	}
	kept := checkedCloudBots(bots)
	if kept[0].Source.BotID != "0b7e-41AF" || kept[1].Source.BotID != "" || kept[2].Source.BotID != "" {
		t.Errorf("the ids kept are %q, %q, %q", kept[0].Source.BotID, kept[1].Source.BotID, kept[2].Source.BotID)
	}
	saved, _ := json.Marshal(kept[0])
	if !strings.Contains(string(saved), `"botId":"0b7e-41AF"`) {
		t.Errorf("it's saved as %s", saved)
	}
}

func TestOpeningACloudBotsCardOrTappingItsDeskClearsTheGreen(t *testing.T) {
	d := newTestDeck(t)
	withCloudBots(d, standardCloudBotNamed("mini", "Mini"))
	for _, open := range []func() (int, string){
		func() (int, string) { return d.Perform("seen", "cloud-mini", nil) },
		func() (int, string) { return d.Focus("cloud-mini") },
	} {
		reported(d, "mini", nil)
		if _, desks, _ := deckState(t, d); desks[0]["state"] != "unread" {
			t.Fatalf("after a report it shows %v", desks[0]["state"])
		}
		if status, _ := open(); status != 204 {
			t.Errorf("opened: %d, want 204", status)
		}
		if _, desks, _ := deckState(t, d); desks[0]["state"] != "thinking" {
			t.Errorf("opened, it shows %v, want blue", desks[0]["state"])
		}
	}
	if status, _ := d.Perform("seen", "s1", nil); status != 404 {
		t.Errorf("seen for a desk that isn't a cloud bot's: %d, want 404", status)
	}
}

// MARK: Its routes

func TestACloudBotsNewsIsGuardedAsSetupsActionsAre(t *testing.T) {
	d := newTestDeck(t)
	withCloudBots(d, standardCloudBotNamed("mini", "Mini"))
	event := func(from, target, body string, headers map[string]string) (int, string) {
		w := serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: from, headers: headers, body: body})
		return w.Code, w.Body.String()
	}
	allowed := map[string]string{setupHeader: "1", setupKeyHeader: d.access.SetupKey}
	for _, refused := range []struct {
		from    string
		headers map[string]string
		want    int
	}{{aPhone, allowed, 404}, {thisComputer, map[string]string{setupKeyHeader: d.access.SetupKey}, 403},
		{thisComputer, map[string]string{setupHeader: "1"}, 401}, {thisComputer, map[string]string{setupHeader: "1", setupKeyHeader: d.access.HookKey}, 401}} {
		if got, _ := event(refused.from, "/bots/mini/event", `{"type": "report"}`, refused.headers); got != refused.want {
			t.Errorf("an event from %s with %v: %d, want %d", refused.from, refused.headers, got, refused.want)
		}
	}
	if last := d.cloud.last("cloud-mini"); last != nil {
		t.Fatal("a refused event reached the desk")
	}
	for _, c := range []struct {
		target, body string
		want         int
		message      string
	}{
		{"/bots/mini/event", `{"type": "report", "text": "Done", "count": 2}`, 204, ""},
		{"/bots/mini/event?type=seen", ``, 204, ""},
		{"/bots/mini/event", `{"type": "wave"}`, 400, "Say what happened: report, seen, start, stop."},
		{"/bots/mini/event", `{"type": "report", "count": 0}`, 400, "count should be a whole number, 1 or more."},
		{"/bots/mini/event", `{"type": "report", "at": "now"}`, 400, "at should be an ISO 8601 time, as 2026-10-28T22:20:00Z."},
		{"/bots/mini/event", `{"type": "report", "text": 1}`, 400, "text should be a string."},
		{"/bots/nobody/event", `{"type": "report"}`, 404, "There's no cloud bot by that id."},
		{"/bots/mini", `{"type": "report"}`, 404, ""},
		{"/bots/mini/event/more", `{"type": "report"}`, 404, ""},
	} {
		if got, message := event(thisComputer, c.target, c.body, allowed); got != c.want || message != c.message {
			t.Errorf("%s %s: %d %q, want %d %q", c.target, c.body, got, message, c.want, c.message)
		}
	}
	if last := d.cloud.last("cloud-mini"); last == nil || last.Text != "Done" || last.Count != 2 {
		t.Errorf("the desk's card says %+v", last)
	}
	if w := serve(t, d, call{method: "GET", target: "/bots/mini/event", host: "localhost:7710", from: thisComputer, headers: allowed}); w.Code != 404 {
		t.Errorf("an event by GET: %d, want 404", w.Code)
	}
}

func TestSetupSetsTheCloudBotsAllAtOnce(t *testing.T) {
	d := newTestDeck(t)
	set := func(body string) (int, string) {
		w := serve(t, d, call{method: "POST", target: "/setup/bots", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
		return w.Code, w.Body.String()
	}
	refusal := `Send the bots as a list: {"bots": [{"id": …, "name": …}]}.`
	for _, bad := range []string{``, `{}`, `{"bots": null}`, `{"bots": {"id": "x"}}`, `"x"`, `[3]`, `[{"id": "x", "name": "X", "permanent": "yes"}]`} {
		if got, message := set(bad); got != 400 || message != refusal {
			t.Errorf("setting %s: %d %q, want 400", bad, got, message)
		}
	}
	got, body := set(`{"bots": [{"id": "Mini", "name": "Smolcorp mini"}, {"id": "bad id", "name": "x"}]}`)
	want := `{"bots":[{"id":"mini","name":"Smolcorp mini","agent":"grok","outfit":"grok","badge":"","permanent":true,"floor":"",` +
		`"source":{"kind":"none","bot":"","botId":"","counts":"unprompted"},"discovered":false,"pinned":false,"mailbox":""}]}`
	if got != 200 || strings.TrimSpace(body) != want {
		t.Errorf("setting the bots: %d %s\nwant %s", got, body, want)
	}
	if got, _ := set(`[{"id": "mini", "name": "Smolcorp mini"}, {"id": "two", "name": "Two"}]`); got != 200 {
		t.Errorf("setting the bots as a list alone: %d", got)
	}
	w := serve(t, d, call{method: "GET", target: "/setup/bots", host: "localhost:7710", from: thisComputer})
	var listed struct{ Bots []CloudBot }
	if json.Unmarshal(w.Body.Bytes(), &listed); w.Code != 200 || len(listed.Bots) != 2 || listed.Bots[1].ID != "two" {
		t.Errorf("the bots as listed: %d %s", w.Code, w.Body)
	}
	if saved := loadConfig(); len(saved.CloudBots) != 2 {
		t.Errorf("the bots saved: %+v", saved.CloudBots)
	}
	if _, desks, _ := deckState(t, d); len(desks) != 2 || desks[0]["id"] != "cloud-mini" {
		t.Errorf("upstairs after setting them: %v", desks)
	}
	if w := serve(t, d, call{method: "GET", target: "/setup/bots", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}); w.Code != 401 {
		t.Errorf("the bots without the setup key: %d, want 401", w.Code)
	}
}

func TestTheLockedStateAnswerNamesThisApp(t *testing.T) {
	d := newTestDeck(t)
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true})
	if w.Code != 401 || w.Body.Len() != 0 {
		t.Errorf("the state without the setup key: %d %q, want 401 and no body", w.Code, w.Body)
	}
	if got := w.Header().Get(appHeader); got != runtime.GOOS {
		t.Errorf("the app header: %q, want %q", got, runtime.GOOS)
	}
	other := serve(t, d, call{method: "GET", target: "/setup/bots", host: "localhost:7710", from: thisComputer, withoutSetupKey: true})
	if other.Code != 401 || other.Header().Get(appHeader) != "" {
		t.Errorf("another locked path: %d %q, want 401 and no header", other.Code, other.Header().Get(appHeader))
	}
}

func TestSetupPinsABotOrPutsItOnAFloorAndKeepsTheMacAppsFields(t *testing.T) {
	d := newTestDeck(t)
	found := standardCloudBotNamed("mini", "Smolcorp mini")
	found.Discovered = true
	withCloudBots(d, found)
	override := func(body string) int {
		return serve(t, d, call{method: "POST", target: "/setup/bots/override", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body}).Code
	}
	if code := override(`{"id": "mini", "pinned": true, "floor": "4"}`); code != 200 {
		t.Fatalf("override gave %d", code)
	}
	if bot := d.config.CloudBots[0]; !bot.Pinned || !bot.Discovered || bot.Floor != "4" {
		t.Errorf("the bot is %+v", bot)
	}
	if code := override(`{"id": "nobody", "pinned": true}`); code != 404 {
		t.Errorf("a bot there isn't gave %d", code)
	}
	// The Mac app's own fields, kept through a save here.
	on := false
	d.config.BotsAutoDetect, d.config.HiddenBots = &on, []string{"aa11"}
	d.config.save()
	loaded := loadConfig()
	if loaded.BotsAutoDetect == nil || *loaded.BotsAutoDetect || !slices.Equal(loaded.HiddenBots, []string{"aa11"}) ||
		!loaded.CloudBots[0].Pinned || !loaded.CloudBots[0].Discovered {
		t.Errorf("loaded %+v %v %+v", loaded.BotsAutoDetect, loaded.HiddenBots, loaded.CloudBots)
	}
	if code := override(`{"id": "mini", "hidden": true}`); code != 200 || len(d.config.CloudBots) != 0 {
		t.Errorf("hidden gave %d, bots %v", code, d.config.CloudBots)
	}
}

// botNews posts a bot's event to /bots/<id>/event as the caller named, and gives the status.
func botNews(t *testing.T, d *Deck, as, target string, extra map[string]string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"type":"seen"}`))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(setupHeader, "1")
	switch as {
	case "page":
		d.mu.Lock()
		r.AddCookie(&http.Cookie{Name: setupCookie(), Value: d.access.NewSetupSession()})
		d.mu.Unlock()
	case "tool":
		r.Header.Set(setupKeyHeader, d.access.SetupKey)
	}
	for name, value := range extra {
		r.Header.Set(name, value)
	}
	// An empty setup header in extra means none at all.
	if value, ok := extra[setupHeader]; ok && value == "" {
		r.Header.Del(setupHeader)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w.Code
}

// The setup page's cookie may post a bot's news, as a tool's key may (the table's "/bots/*": page, tool): the Mac's
// gate lets the page through where the Go core answered 401. With nobody known it is 401 still.
func TestThePagesCookieMayPostABotsNews(t *testing.T) {
	d := newTestDeck(t)
	asked := serve(t, d, call{method: "POST", target: "/setup/bots", body: `{"bots":[{"id":"c2-bot","name":"C2 Bot"}]}`, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if asked.Code != http.StatusOK {
		t.Fatalf("the bot: %d %s", asked.Code, asked.Body)
	}
	for _, c := range []struct {
		as, target string
		want       int
	}{
		{"page", "/bots/c2-bot/event", http.StatusNoContent},
		{"tool", "/bots/c2-bot/event", http.StatusNoContent},
		{"page", "/bots/no-such-bot/event", http.StatusNotFound},
		{"", "/bots/c2-bot/event", http.StatusUnauthorized},
	} {
		if got := botNews(t, d, c.as, c.target, nil); got != c.want {
			t.Errorf("%q %s: %d, want %d", c.as, c.target, got, c.want)
		}
	}
	// Nobody else may: a hire's key and the GM's token are refused (403), alone or together, and the page's cookie
	// without the setup header is refused too (a page from another site can't send it).
	if err := os.WriteFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hireKey, err := newSendKey(registryPath(), "aaa111")
	if err != nil {
		t.Fatal(err)
	}
	gmToken := d.newGMToken()
	for name, c := range map[string]struct {
		as    string
		extra map[string]string
	}{
		"a hire key":              {"", map[string]string{hireKeyHeader: hireKey}},
		"a GM token":              {"", map[string]string{gmTokenHeader: gmToken}},
		"a hire key and GM token": {"", map[string]string{hireKeyHeader: hireKey, gmTokenHeader: gmToken}},
	} {
		if got := botNews(t, d, c.as, "/bots/c2-bot/event", c.extra); got != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", name, got)
		}
	}
	for _, as := range []string{"page", "tool"} {
		if got := botNews(t, d, as, "/bots/c2-bot/event", map[string]string{setupHeader: ""}); got != http.StatusForbidden {
			t.Errorf("%s without the setup header: %d, want 403", as, got)
		}
	}
}
