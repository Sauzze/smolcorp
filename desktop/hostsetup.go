package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Setup's parts that only macOS does, served under the Mac app (engine=go)
// through the bridge (hostbridge.go), as the Swift engine serves them
// (DeckRouter.swift): Accessibility, pressing keys in other apps and the apps
// they may go to, In the Dock, dictation and its names, "Add smolcorp to
// Terminal", and who the Grok Bots are in the Grok Bot app. The settings are
// kept here, in the config, in the Mac app's format; the app does what they
// say, with its own code. On Windows and Linux there is no host: none of these
// routes is served (404), and the setup state has none of these fields.

// macState is the Mac app this deck runs under, and what it last said that's needed between asks. With the
// deck's lock.
type macState struct {
	// The Mac app, or nil: the host when the deck was made, never changed after, so nothing the deck does
	// later, on any goroutine, reads the package's.
	host hostBridge
	// The dictation language the Mac suggests, and those it has, from the last facts: the suggested one is
	// used until one is chosen, and a chosen one it doesn't have isn't (as the Mac's Config load).
	suggested string
	languages []string
	// Why dictation's language isn't ready, as the app said when it got it ready last, or "".
	dictationProblem string
	// What each session's terminal lets the office do (hostoffice.go): for the tabs in abilitiesFor,
	// and whether the app is being asked now; the tabs it didn't answer for last, how many times, and
	// when it may be asked again.
	abilities     map[string]tabAbilities
	abilitiesFor  string
	learning      bool
	unansweredFor string
	unanswered    int
	askAgainAt    time.Time
	// Held from reading the apps keys may go to until the app's new list is kept, so two changes at
	// once don't lose one. Never with the deck's lock.
	appsChanging sync.Mutex
}

// eventKindKeystroke is pressing keys in other apps switched on or off, or an
// app allowed or taken off: the Mac's EventLog.Kind.keystroke.
const eventKindKeystroke = "keystroke"

// glossaryBodyMost is the longest list of names read: far past the most the clean-up keeps.
const glossaryBodyMost = 64 << 10

// macFactsNow asks the Mac app what setup shows of this Mac: nil without the
// app, or without its answer, and setup then shows what it has. Without the lock.
func (d *Deck) macFactsNow() *macFacts {
	d.mu.Lock()
	h, apps := d.mac.host, slices.Clone(d.config.KeystrokeApps)
	d.mu.Unlock()
	if h == nil {
		return nil
	}
	facts, err := hostFacts(h, apps)
	if err != nil {
		return nil
	}
	d.mu.Lock()
	d.learnedLanguages(facts)
	d.mu.Unlock()
	return &facts
}

// learnedLanguages keeps the dictation languages the Mac has, and the one it suggests, and tells the office
// when its language changes with them. With the lock held.
func (d *Deck) learnedLanguages(facts macFacts) {
	before := d.dictationLanguage()
	d.mac.suggested, d.mac.languages = facts.Dictation.Suggested, facts.Dictation.Languages
	if d.dictationLanguage() != before {
		d.changed()
	}
}

// macHost is the Mac app this deck runs under, or nil. Without the lock.
func (d *Deck) macHost() hostBridge {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mac.host
}

// addMacSetup puts what only the Mac has into setup's state, as the Swift
// engine's: nothing without facts. With the lock held.
func (d *Deck) addMacSetup(state map[string]any, facts *macFacts) {
	if facts == nil {
		return
	}
	c := d.config
	if checklist, ok := state["checklist"].(map[string]any); ok {
		checklist["accessibility"] = facts.Accessibility
	}
	state["terminalCommand"] = facts.Terminal
	glossary := facts.Dictation.Glossary
	if c.DictationGlossary != nil {
		glossary = *c.DictationGlossary
	}
	// Why the clean-up didn't work last: only while it's on.
	var cleanupProblem *string
	if c.DictationCleanup {
		cleanupProblem = facts.Dictation.CleanupProblem
	}
	state["dictation"] = map[string]any{
		"available": facts.Dictation.Available, "enabled": c.DictationEnabled, "allowed": facts.Dictation.Allowed,
		"language": d.dictationLanguage(), "claude": c.DictationForClaude, "cleanup": c.DictationCleanup,
		"glossary": orEmpty(glossary), "cleanupProblem": cleanupProblem, "problem": nilIfEmpty(d.mac.dictationProblem),
	}
	state["keystrokes"] = map[string]any{"enabled": c.KeystrokesAllowed, "accessibility": facts.Accessibility,
		"apps": orEmpty(c.KeystrokeApps), "running": orEmpty(facts.Running)}
	state["dock"] = map[string]any{"enabled": c.ShowInDock == nil || *c.ShowInDock}
	if bots, ok := state["cloudBots"].(map[string]any); ok {
		bots["autoDetect"] = c.BotsAutoDetect == nil || *c.BotsAutoDetect
	}
}

// dictationLanguage is the one chosen, or the one the Mac suggests: a chosen one the Mac doesn't have is
// left out, as the Mac app's Config load leaves it out. With the lock held.
func (d *Deck) dictationLanguage() string {
	chosen := d.config.DictationLanguage
	if chosen != "" && (d.mac.languages == nil || slices.Contains(d.mac.languages, chosen)) {
		return chosen
	}
	return d.mac.suggested
}

// orEmpty is list, or an empty one for none: JSON's [] rather than null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// appRefusal is the Mac app saying no, and why: a request it won't do, not a
// bridge that didn't answer.
type appRefusal string

func (r appRefusal) Error() string { return string(r) }

// bridgeFailed answers err: the app's refusal as a conflict, in its words; a
// bridge that didn't answer as unavailable.
func bridgeFailed(w http.ResponseWriter, err error) {
	var refusal appRefusal
	if errors.As(err, &refusal) {
		writeStatus(w)(http.StatusConflict, refusal.Error())
		return
	}
	writeStatus(w)(http.StatusServiceUnavailable, "Smolcorp couldn't ask the Mac app: "+err.Error())
}

// macSetup answers setup's routes for what only macOS does, under the Mac app,
// and says whether the route was one. Elsewhere it answers none: they're 404.
func (rt *router) macSetup(w http.ResponseWriter, r *request) bool {
	d := rt.deck
	h := d.macHost()
	if h == nil || r.Method != http.MethodPost {
		return false
	}
	query := r.URL.Query()
	path := r.URL.Path
	switch {
	// In the Dock and ⌘-Tab, or the menu bar only: at once, and kept.
	case path == "/setup/dock":
		// Kept once the app has done it: a bridge that doesn't answer leaves the setting as it was.
		on := query.Get("enabled") == "1"
		if err := hostDock(h, on); err != nil {
			bridgeFailed(w, err)
			return true
		}
		d.mu.Lock()
		d.config.ShowInDock = &on
		d.config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Setup's Accessibility step: macOS asks you, in its own window.
	case path == "/setup/accessibility":
		if err := hostAskAccessibility(h); err != nil {
			bridgeFailed(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)

	// Pressing keys in other apps: on or off from the page alone, and on asks macOS for Accessibility.
	case path == "/setup/keystrokes":
		enabled := query.Get("enabled")
		if enabled != "1" && enabled != "0" {
			writeStatus(w)(http.StatusBadRequest, "Say enabled=1 or enabled=0.")
			return true
		}
		on := enabled == "1"
		d.mu.Lock()
		changed := d.config.KeystrokesAllowed != on
		d.config.KeystrokesAllowed = on
		d.config.save()
		d.mu.Unlock()
		if changed {
			detail := "switched off"
			if on {
				detail = "switched on"
			}
			d.events.append(eventEntry{At: int64(nowMillis()), Kind: eventKindKeystroke, Detail: detail})
		}
		// The prompt is the app's to show: switched on and kept, whether it comes up or not.
		if on {
			_ = hostKeystrokesOn(h)
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The apps keys may go to: one added (?add=id) or taken off (?remove=id), from the page alone.
	case path == "/setup/keystrokes/apps":
		added, adding := query["add"]
		removed, removing := query["remove"]
		if adding == removing {
			writeStatus(w)(http.StatusBadRequest, "Say add=<app id> or remove=<app id>.")
			return true
		}
		id := slices.Concat(added, removed)[0]
		d.mac.appsChanging.Lock()
		d.mu.Lock()
		before := slices.Clone(d.config.KeystrokeApps)
		d.mu.Unlock()
		list, err := hostKeystrokeApps(h, before, id, adding)
		if err != nil {
			d.mac.appsChanging.Unlock()
			bridgeFailed(w, err)
			return true
		}
		d.mu.Lock()
		changed := !slices.Equal(list, d.config.KeystrokeApps)
		if changed {
			d.config.KeystrokeApps = list
			d.config.save()
		}
		d.mu.Unlock()
		d.mac.appsChanging.Unlock()
		if changed {
			detail := "app removed: " + id
			if adding {
				detail = "app allowed: " + id
			}
			d.events.append(eventEntry{At: int64(nowMillis()), Kind: eventKindKeystroke, Detail: detail})
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Dictation: turned on, its language, and whether Claude Code uses it. Turning it on asks for the
	// microphone, on the Mac, and gets the language's model ready.
	case path == "/setup/dictation":
		facts, err := hostFacts(h, nil)
		if err != nil {
			bridgeFailed(w, err)
			return true
		}
		language, choosing := query["language"]
		if choosing && !slices.Contains(facts.Dictation.Languages, language[0]) {
			writeStatus(w)(http.StatusBadRequest, "Unknown language")
			return true
		}
		d.mu.Lock()
		d.learnedLanguages(facts)
		if choosing {
			d.config.DictationLanguage = language[0]
		}
		if query.Has("claude") {
			d.config.DictationForClaude = query.Get("claude") == "1"
		}
		if query.Has("cleanup") {
			d.config.DictationCleanup = query.Get("cleanup") == "1"
		}
		if query.Has("enabled") {
			d.config.DictationEnabled = query.Get("enabled") == "1" && facts.Dictation.Available
		}
		d.config.save()
		d.changed()
		enabled, ready := d.config.DictationEnabled, d.dictationLanguage()
		d.mu.Unlock()
		if enabled {
			problem, err := hostPrepareDictation(h, ready)
			if err != nil {
				problem = "The Mac app didn't get dictation ready: " + err.Error()
			}
			d.mu.Lock()
			d.mac.dictationProblem = problem
			d.changed()
			d.mu.Unlock()
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// The names dictation's clean-up knows, as the page's lines.
	case path == "/setup/dictation/glossary":
		var lines []string
		data, _ := io.ReadAll(io.LimitReader(r.Body, glossaryBodyMost+1))
		if len(data) > glossaryBodyMost || json.Unmarshal(data, &lines) != nil || lines == nil {
			writeStatus(w)(http.StatusBadRequest, "That isn't a list of names.")
			return true
		}
		kept, err := hostGlossary(h, lines)
		if err != nil {
			bridgeFailed(w, err)
			return true
		}
		d.mu.Lock()
		d.config.DictationGlossary = &kept
		d.config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// "Add smolcorp to Terminal": the link to the app's command, made or removed (the page alone).
	case path == "/setup/terminal-command":
		if err := hostTerminalCommand(h, !query.Has("remove")); err != nil {
			bridgeFailed(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, rt.setupStateFor(r.caller))

	// Each Grok Bot without its id given it from the Grok Bot app, by its name or the one like it, so a
	// rename there no longer loses it.
	case path == "/setup/bots/resolve":
		rt.resolveBots(w, h)

	default:
		return false
	}
	return true
}

// resolveBots fills in the Grok Bot app's id of each Grok Bot set without
// one, and answers the bots, those filled in and those not found there.
func (rt *router) resolveBots(w http.ResponseWriter, h hostBridge) {
	d := rt.deck
	var asked []bridgeBot
	var ids []string
	d.mu.Lock()
	for _, bot := range d.config.CloudBots {
		if bot.Source.Kind == "grokBotApp" && bot.Source.BotID == "" {
			asked = append(asked, bridgeBot{Bot: bot.Source.Bot})
			ids = append(ids, bot.ID)
		}
	}
	d.mu.Unlock()
	filled, unresolved := []string{}, []string{}
	if len(asked) > 0 {
		found, err := hostResolveBots(h, asked)
		if err != nil {
			bridgeFailed(w, err)
			return
		}
		d.mu.Lock()
		for i, id := range ids {
			at := slices.IndexFunc(d.config.CloudBots, func(b CloudBot) bool { return b.ID == id })
			if found[i] == nil || !safeGrokBotID(found[i].BotID) || at < 0 || d.config.CloudBots[at].Source.BotID != "" {
				unresolved = append(unresolved, id)
				continue
			}
			d.config.CloudBots[at].Source.BotID, d.config.CloudBots[at].Source.Bot = found[i].BotID, found[i].Name
			filled = append(filled, id)
		}
		if len(filled) > 0 {
			d.config.CloudBots = checkedCloudBots(d.config.CloudBots)
			d.config.save()
			d.cloud.configure(d.config.CloudBots)
			d.changed()
		}
		d.mu.Unlock()
	}
	d.mu.Lock()
	bots := slices.Clone(d.config.CloudBots)
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"bots": botsResolved(h, bots), "filled": filled, "unresolved": unresolved})
}

// resolvedBot is a bot as kept, with who it is in the Grok Bot app now: null
// for one not from that app, or not in it.
type resolvedBot struct {
	CloudBot
	Resolved *botResolution `json:"resolved"`
}

// botsResolved is setup's list of bots: as kept, and under the Mac app each
// with who it is in the Grok Bot app, read afresh, as the Swift engine's
// (cloudBotsJSON). Elsewhere nothing looks in that app: the bots as kept.
func botsResolved(h hostBridge, bots []CloudBot) any {
	if h == nil {
		return bots
	}
	list := make([]resolvedBot, len(bots))
	var asked []bridgeBot
	var at []int
	for i, bot := range bots {
		list[i].CloudBot = bot
		if bot.Source.Kind == "grokBotApp" {
			asked = append(asked, bridgeBot{BotID: bot.Source.BotID, Bot: bot.Source.Bot})
			at = append(at, i)
		}
	}
	if len(asked) > 0 {
		// Not answered: none found, as when the app's roster can't be read.
		if found, err := hostResolveBots(h, asked); err == nil {
			for i, resolution := range found {
				list[at[i]].Resolved = resolution
			}
		}
	}
	return list
}
