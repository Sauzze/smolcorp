package desktop

import (
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Under the Mac app, the Grok Bot app's bots are found and given desks, as the
// Swift engine finds them (Deck.discoverBots, CloudBot.discover): the app
// reads that app's roster with its own code (GrokBotApp, over the bridge's
// bots-roster), and the core sets the cloud bots to match. At start, every
// minute, as setup opens (at most once a minute) and after setup's bot
// switches; only with "Find the Grok Bot app's bots" on. Elsewhere there's no
// such app: nothing is looked for.

// grokRow is one bot in the Grok Bot app's roster: its id, the name it has
// now, and whether it's a group chat, between bots, rather than a bot.
type grokRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsGroup bool   `json:"isGroup"`
}

// botsDiscoverEvery is how often the Grok Bot app's bots are looked for.
const botsDiscoverEvery = time.Minute

// hostBotsRoster is the Grok Bot app's bots now, read afresh by the Mac app;
// false when its roster can't be read, which isn't an app with no bots.
func hostBotsRoster(h hostBridge) ([]grokRow, bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "bots-roster"})
	if err != nil || answer.Rows == nil {
		return nil, false, err
	}
	return *answer.Rows, true, nil
}

// grokResolve finds a bot in rows, as GrokBotApp.resolve: by botID alone when
// it has one; else by name, its case aside; else by the only name that holds
// it or that it holds. Nil when none.
func grokResolve(botID, name string, rows []grokRow) *botResolution {
	if botID != "" {
		for _, row := range rows {
			if strings.EqualFold(row.ID, botID) {
				return &botResolution{BotID: row.ID, Name: row.Name, By: "id"}
			}
		}
		return nil
	}
	trim := func(s string) string {
		return strings.TrimFunc(strings.ToLower(s), func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) })
	}
	wanted := trim(name)
	if wanted == "" {
		return nil
	}
	for _, row := range rows {
		if strings.ToLower(row.Name) == wanted {
			return &botResolution{BotID: row.ID, Name: row.Name, By: "name"}
		}
	}
	var like []grokRow
	for _, row := range rows {
		if other := trim(row.Name); other != "" && (strings.Contains(other, wanted) || strings.Contains(wanted, other)) {
			like = append(like, row)
		}
	}
	if len(like) != 1 {
		return nil
	}
	return &botResolution{BotID: like[0].ID, Name: like[0].Name, By: "guess"}
}

// cloudDeskName is a name as a bot's desk says it: no slash, as it's its folder's last part.
func cloudDeskName(name string) string {
	return oneLine(strings.ReplaceAll(name, "/", "-"), roleNameLimit)
}

// appID is the bot's id in the Grok Bot app: the one it was given, else the
// bot there by its name (or the one like it). "" when the app has none.
func (b CloudBot) appID(rows []grokRow) string {
	if b.Source.BotID != "" {
		return b.Source.BotID
	}
	for _, name := range []string{b.Source.Bot, b.Name} {
		if found := grokResolve("", name, rows); name != "" && found != nil {
			return found.BotID
		}
	}
	return ""
}

// freeCloudID is an id for a bot found in the app: its name in letters,
// digits and dashes, or "grok" with its app id's start, made unique with a number.
func freeCloudID(row grokRow, taken []string) string {
	words := strings.Map(func(r rune) rune {
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return '-'
	}, strings.ToLower(row.Name))
	base := strings.Join(strings.FieldsFunc(words, func(r rune) bool { return r == '-' }), "-")
	if base == "" {
		id := strings.ToLower(row.ID)
		base = "grok-" + id[:min(8, len(id))]
	}
	base = base[:min(len(base), cloudIDLimit-3)]
	id := base
	for n := 2; slices.Contains(taken, id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// discoverCloudBots is what finding the Grok Bot app's bots makes of bots, as
// CloudBot.discover: one for each bot there that none is set for yet and
// that isn't hidden, found, at the end, while there's room; found ones gone
// from there, or hidden since, left out, unless pinned. A bot you set
// yourself is never left out. A group chat is never found, and one found
// before goes, pinned or not. A found bot takes the name it has there now.
// Also the rows there was no room for.
func discoverCloudBots(all []grokRow, bots []CloudBot, hidden []string) ([]CloudBot, []grokRow) {
	groups, here, hiddenIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var rows []grokRow
	for _, row := range all {
		if row.IsGroup {
			groups[strings.ToLower(row.ID)] = true
		} else {
			rows = append(rows, row)
			here[strings.ToLower(row.ID)] = true
		}
	}
	for _, id := range hidden {
		hiddenIDs[strings.ToLower(id)] = true
	}
	var kept []CloudBot
	for _, bot := range bots {
		// A bot set by its exact name there is given its id, so a rename there later doesn't lose it.
		if bot.Source.Kind == "grokBotApp" && bot.Source.BotID == "" {
			name := bot.Source.Bot
			if name == "" {
				name = bot.Name
			}
			if found := grokResolve("", name, rows); found != nil && found.By == "name" {
				bot.Source.BotID = found.BotID
			}
		}
		id := strings.ToLower(bot.Source.BotID)
		fromApp := bot.Source.Kind == "grokBotApp"
		if bot.Discovered && fromApp && groups[id] {
			continue
		}
		if !bot.Discovered || bot.Pinned || !fromApp || here[id] && !hiddenIDs[id] {
			kept = append(kept, bot)
		}
	}
	// Renamed in the app: a found bot's name there now, unless you gave it one (setup pins a bot it renames).
	for i, bot := range kept {
		if !bot.Discovered || bot.Source.Kind != "grokBotApp" || bot.Pinned && bot.Name != cloudDeskName(bot.Source.Bot) {
			continue
		}
		at := slices.IndexFunc(rows, func(row grokRow) bool { return strings.EqualFold(row.ID, bot.Source.BotID) })
		if at < 0 {
			continue
		}
		if name := cloudDeskName(rows[at].Name); name != "" {
			kept[i].Name, kept[i].Source.Bot = name, oneLine(rows[at].Name, 64)
		}
	}
	var noRoom []grokRow
	for _, row := range rows {
		if hiddenIDs[strings.ToLower(row.ID)] {
			continue
		}
		// Already at a desk: by its id, or by its name for a bot the app has none for.
		known := slices.ContainsFunc(kept, func(bot CloudBot) bool {
			if app := bot.appID(rows); app != "" {
				return strings.EqualFold(app, row.ID)
			}
			if bot.Source.BotID != "" {
				return false
			}
			for _, name := range []string{bot.Name, bot.Source.Bot} {
				if strings.EqualFold(name, cloudDeskName(row.Name)) || strings.EqualFold(name, row.Name) {
					return true
				}
			}
			return false
		})
		if known {
			continue
		}
		if len(kept) >= cloudBotsMost {
			noRoom = append(noRoom, row)
			continue
		}
		taken := make([]string, len(kept))
		for i, bot := range kept {
			taken[i] = bot.ID
		}
		found := standardCloudBot()
		found.ID, found.Name, found.Discovered = freeCloudID(row, taken), row.Name, true
		found.Source = CloudSource{Kind: "grokBotApp", Bot: row.Name, BotID: row.ID, Counts: "unprompted"}
		if bot, ok := found.checked(); ok {
			kept = append(kept, bot)
		}
	}
	return kept, noRoom
}

// pinnedIfChanged is bots as setup sent them, each one that was there before and is changed now pinned, as
// the Swift engine's POST /setup/bots: whether it was pinned or found aside, one sent back unchanged isn't.
func pinnedIfChanged(bots, before []CloudBot) []CloudBot {
	unmarked := func(b CloudBot) CloudBot {
		b.Pinned, b.Discovered = false, false
		return b
	}
	for i, bot := range bots {
		at := slices.IndexFunc(before, func(old CloudBot) bool { return old.ID == bot.ID })
		if at >= 0 && !bot.Pinned && unmarked(before[at]) != unmarked(bot) {
			bots[i].Pinned = true
		}
	}
	return bots
}

// botsAutoDetect is whether the Grok Bot app's bots are looked for: on until you turn it off. With the lock held.
func (c *Config) botsAutoDetect() bool { return c.BotsAutoDetect == nil || *c.BotsAutoDetect }

// discoverBots looks for the Grok Bot app's bots under the Mac app, and sets the cloud bots to match, with
// auto-detect on. From setup, at most once a minute. Without the lock.
func (d *Deck) discoverBots(fromSetup bool) {
	d.mu.Lock()
	h := d.mac.host
	due := !fromSetup || time.Since(d.mac.lastDiscovery) >= botsDiscoverEvery
	if h == nil || !d.config.botsAutoDetect() || d.mac.discovering || !due {
		d.mu.Unlock()
		return
	}
	d.mac.discovering, d.mac.lastDiscovery = true, time.Now()
	d.mu.Unlock()
	rows, readable, err := hostBotsRoster(h)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mac.discovering = false
	// An app moved or locked, or a bridge that didn't answer, isn't an app with no bots: nothing changes.
	if err != nil || !readable || !d.config.botsAutoDetect() {
		return
	}
	found, noRoom := discoverCloudBots(rows, d.config.CloudBots, d.config.HiddenBots)
	d.mac.botsNoRoom, d.mac.roster = noRoom, rows
	if slices.Equal(found, d.config.CloudBots) {
		return
	}
	d.config.CloudBots = found
	d.config.save()
	d.cloud.configure(d.config.CloudBots)
	d.changed()
}

// addMacBots is setup's cloud bots as the Swift engine's: found in the Grok Bot app, by the names they have
// there now; those you hid, by name where the app still has them; those with no room. With the lock held.
func (d *Deck) addMacBots(bots map[string]any) {
	c, roster := d.config, d.mac.roster
	bots["detects"], bots["autoDetect"] = true, c.botsAutoDetect()
	list := cloudBotsForSetup(c.CloudBots)
	for i, bot := range c.CloudBots {
		named := bot.Pinned && bot.Name != cloudDeskName(bot.Source.Bot)
		if bot.Source.Kind != "grokBotApp" || bot.Source.BotID == "" || named {
			continue
		}
		at := slices.IndexFunc(roster, func(row grokRow) bool { return strings.EqualFold(row.ID, bot.Source.BotID) })
		if at >= 0 && cloudDeskName(roster[at].Name) != "" {
			list[i]["name"] = cloudDeskName(roster[at].Name)
		}
	}
	bots["bots"] = list
	hidden := []map[string]any{}
	for _, id := range c.HiddenBots {
		var name any
		if at := slices.IndexFunc(roster, func(row grokRow) bool { return strings.EqualFold(row.ID, id) }); at >= 0 {
			name = roster[at].Name
		}
		hidden = append(hidden, map[string]any{"botId": id, "name": name})
	}
	bots["hidden"] = hidden
	noRoom := []string{}
	for _, row := range d.mac.botsNoRoom {
		noRoom = append(noRoom, row.Name)
	}
	bots["noRoom"] = append(noRoom, d.botsWithNoDesk()...)
}
