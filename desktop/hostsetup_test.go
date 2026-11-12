package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// macStandIn stands in for the Mac app on the actions for what only macOS
// does, as HostBridge.swift answers them, with the shared fixture's facts:
// tab actions it can't do (as a Terminal.app tab without the app), the rest
// done. It keeps what it was asked, and is safe to ask from the deck's rounds.
type macStandIn struct {
	mu        sync.Mutex
	asked     []bridgeRequest
	facts     macFacts
	abilities tabAbilities
	refuse    map[string]string // by action: the app's no
	// The login item, the folder the panel answers ("" for none), and the app's own items (hostshell.go).
	login   bool
	picked  string
	secrets map[string]string
	// lookFailed: items the Keychain has but can't read now (locked, say), said as failed (the GM's token, M7TOK).
	lookFailed map[string]bool
	// The Grok Bot app's roster (nil: it can't be read); the jukebox: Accessibility given, the volume.
	roster   *[]grokRow
	keyboard bool
	volume   int
	// How the Mac writes a short time (time-format).
	timeFormat macTimeFormat
	// The app's items that are there but can't be read (a denied Keychain prompt), and what it tells of
	// you being back at the Mac (presence).
	unreadable []string
	presence   bridgeAnswer
	// The office's mic (hostdictation.go): a recording that won't start, and what a stop heard; a start held
	// until startWait is closed, if set (before the lock, so a stop can be answered meanwhile).
	notStarting bool
	heard       string
	startWait   chan struct{}
	// The Mac's screens (gmscreens.go), and a terminal that refuses Smolcorp its Automation permission for tiling.
	screens       []macScreen
	tileForbidden bool
}

func (m *macStandIn) call(request bridgeRequest) (bridgeAnswer, error) {
	if request.Action == "dictation-start" {
		m.mu.Lock()
		wait := m.startWait
		m.mu.Unlock()
		if wait != nil {
			<-wait
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asked = append(m.asked, request)
	if why, ok := m.refuse[request.Action]; ok {
		return bridgeAnswer{Error: why}, nil
	}
	switch request.Action {
	case "facts":
		if len(request.Apps) > keystrokeAppsMost {
			return bridgeAnswer{Error: "Not a list of apps."}, nil
		}
		facts := m.facts
		return bridgeAnswer{OK: true, Facts: &facts}, nil
	case "dock", "accessibility", "keystrokes-on", "terminal-command", "dictation", "install":
		return bridgeAnswer{OK: true}, nil
	case "install-check":
		return bridgeAnswer{OK: true, Check: &check{ID: "install", Status: checkOK, Title: "Smolcorp is in Applications", Detail: "As /Applications/Smolcorp.app."}}, nil
	case "keystroke-apps":
		list := slices.DeleteFunc(slices.Clone(request.Apps), func(app string) bool { return strings.EqualFold(app, request.App) })
		if *request.On {
			list = append(list, request.App)
		}
		return bridgeAnswer{OK: true, Apps: list}, nil
	case "glossary":
		return bridgeAnswer{OK: true, Names: request.Names}, nil
	case "abilities":
		said := make([]tabAbilities, len(request.Tabs))
		for i := range said {
			said[i] = m.abilities
		}
		return bridgeAnswer{OK: true, Abilities: said}, nil
	case "bots-resolve":
		return bridgeAnswer{OK: true, Resolved: make([]*botResolution, len(request.Bots))}, nil
	case "office", "office-ontop", "office-key", "folder-panel-close":
		return bridgeAnswer{OK: true}, nil
	case "login-item":
		if request.On != nil {
			m.login = *request.On
		}
		return bridgeAnswer{OK: true, On: m.login}, nil
	case "choose-folder":
		return bridgeAnswer{OK: true, Folder: m.picked}, nil
	case "secret-read", "secret-write", "secret-delete":
		if request.Action == "secret-read" && m.lookFailed[request.Item] {
			return bridgeAnswer{OK: true, Failed: true}, nil
		}
		if m.secrets == nil {
			m.secrets = map[string]string{}
		}
		value, found := m.secrets[request.Item]
		switch request.Action {
		case "secret-read":
			if slices.Contains(m.unreadable, request.Item) {
				return bridgeAnswer{Error: "It couldn't be read."}, nil
			}
		case "secret-write":
			m.secrets[request.Item] = request.Value
		case "secret-delete":
			// The viewer's items, as the Swift engine's: never removed this way (HostBridge.swift).
			if strings.HasPrefix(request.Item, "viewer-") {
				return bridgeAnswer{Error: "That item isn't removed this way."}, nil
			}
			delete(m.secrets, request.Item)
		}
		return bridgeAnswer{OK: true, Value: value, Found: found}, nil
	case "presence":
		answer := m.presence
		answer.OK = true
		return answer, nil
	case "bots-roster":
		if m.roster == nil {
			return bridgeAnswer{OK: true}, nil
		}
		rows := slices.Clone(*m.roster)
		return bridgeAnswer{OK: true, Rows: &rows}, nil
	case "music":
		if !m.keyboard {
			return bridgeAnswer{Error: "Keyboard access is off. Turn it on in setup, on your Mac."}, nil
		}
		return bridgeAnswer{OK: true}, nil
	case "volume":
		m.volume = *request.Level
		return bridgeAnswer{OK: true}, nil
	case "screens":
		return bridgeAnswer{OK: true, Screens: slices.Clone(m.screens)}, nil
	case "tile":
		if m.tileForbidden {
			return bridgeAnswer{OK: true, NeedsPermission: true}, nil
		}
		var placed []string
		for _, place := range request.Places {
			placed = append(placed, place.TTY)
		}
		return bridgeAnswer{OK: true, Placed: placed}, nil
	case "office-place":
		return bridgeAnswer{OK: true}, nil
	case "music-state":
		playing := true
		return bridgeAnswer{OK: true, Playing: &playing, Volume: &bridgeVolume{Level: m.volume}}, nil
	case "time-format":
		format := m.timeFormat
		return bridgeAnswer{OK: true, TimeFormat: &format}, nil
	case "dictation-start":
		// A launcher's id isn't a tab the app knows (TabAddress): its no, as RGAPA B2 found it. Busy, the core is
		// recording already: checked, then not started.
		if kind, _, _ := splitTab(request.Tab); kind == "launcher" {
			return bridgeAnswer{Error: "Not a tab."}, nil
		}
		return bridgeAnswer{OK: true, Started: !m.notStarting && !request.Busy}, nil
	case "dictation-stop":
		return bridgeAnswer{OK: true, Text: m.heard}, nil
	case "dictation-clean":
		// Cleaned up when asked, as the app's clean-up says it; off, as heard and nothing to say.
		answer := bridgeAnswer{OK: true, Text: request.Text}
		if request.On != nil && *request.On {
			answer.Note, answer.Cleaned = "cleaned in 0.9 s", true
		}
		return answer, nil
	case "insert":
		return bridgeAnswer{OK: true}, nil
	}
	return bridgeAnswer{Error: "This terminal can't do that.", Unsupported: true}, nil
}

// refusing has the stand-in say no to these actions from now on (none: to none). Its deck may be asking it now.
func (m *macStandIn) refusing(refuse map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refuse = refuse
}

// actions is what it was asked, by action, in order.
func (m *macStandIn) actions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for _, request := range m.asked {
		names = append(names, request.Action)
	}
	return names
}

// bridgeCase is one of the shared fixture's requests and the Mac app's answer.
type bridgeCase struct {
	Name    string          `json:"name"`
	Request json.RawMessage `json:"request"`
	Answer  json.RawMessage `json:"answer"`
}

func readBridgeCases(t *testing.T) []bridgeCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "bridge", "actions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Cases []bridgeCase }
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) < 10 {
		t.Fatalf("the bridge's fixture: %v, %d cases", err, len(fixture.Cases))
	}
	return fixture.Cases
}

// standInFacts is the fixture's facts, as the Mac app answers them.
func standInFacts(t *testing.T) macFacts {
	t.Helper()
	var answer bridgeAnswer
	if err := json.Unmarshal(readBridgeCases(t)[0].Answer, &answer); err != nil || answer.Facts == nil {
		t.Fatalf("the fixture's first case isn't the facts: %v", err)
	}
	return *answer.Facts
}

// withMac puts the stand-in Mac app in place for one test.
func withMac(t *testing.T) *macStandIn {
	t.Helper()
	mac := &macStandIn{facts: standInFacts(t), abilities: tabAbilities{Keys: true, Text: true, Fire: true, Screen: true}}
	saved := host
	t.Cleanup(func() { host = saved })
	host = mac
	return mac
}

// fixtureHost answers one case's answer and keeps the request. A deck's own rounds (its look for the
// Grok Bot app's bots, say) may ask it too, from their own goroutines: every request is kept, under a
// lock, and sawAction says whether one was asked, whatever came after.
type fixtureHost struct {
	mu     sync.Mutex
	asked  *bridgeRequest
	all    []string
	answer bridgeAnswer
}

func (f *fixtureHost) call(request bridgeRequest) (bridgeAnswer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = &request
	f.all = append(f.all, request.Action)
	return f.answer, nil
}

func (f *fixtureHost) sawAction(action string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.all, action)
}

// Each of the fixture's requests is what the core sends, and its answer is read as the core needs it.
func TestTheBridgeFixtureIsWhatTheCoreAsksAndReads(t *testing.T) {
	idle := func() SessionState { return Idle }
	// What the core makes of the phone's text: how it went, and whether the words may have gone all the same.
	phoneText := func(h hostBridge, tab string, agent Agent, words string, state func() SessionState) (any, error) {
		result, err := hostPhoneText(h, tab, agent, words, state)
		return []any{result, errors.Is(err, errPasteFailed)}, err
	}
	want := map[string]func(h hostBridge) (any, error){
		"setup's facts, the apps keys may go to left out of those running": func(h hostBridge) (any, error) {
			facts, err := hostFacts(h, []string{"com.example.notes"})
			return []any{facts.Accessibility, facts.TestCopy, facts.Dictation.Languages, facts.Dictation.CleanupProblem == nil,
				facts.Running, facts.Terminal.State, facts.Terminal.TestCopy, facts.OfficeKey}, err
		},
		"In the Dock, switched off": func(h hostBridge) (any, error) { return nil, hostDock(h, false) },
		"Open at login, read":       func(h hostBridge) (any, error) { return hostLoginItem(h, nil) },
		"Open at login, switched on": func(h hostBridge) (any, error) {
			on := true
			return hostLoginItem(h, &on)
		},
		"the office's window shown":       func(h hostBridge) (any, error) { return nil, hostShowOffice(h) },
		"the office's window kept on top": func(h hostBridge) (any, error) { return nil, hostOfficeOnTop(h, true) },
		"the office's keys held instead":  func(h hostBridge) (any, error) { return nil, hostOfficeKey(h, "ctrl+alt+K") },
		"the office's keys, taken by another app": func(h hostBridge) (any, error) {
			return nil, hostOfficeKey(h, "ctrl+alt+T")
		},
		"the office's keys, the Mac's own on the layout in use": func(h hostBridge) (any, error) {
			return nil, hostOfficeKey(h, "cmd+A")
		},
		"the folder panel for a floor, a folder picked": func(h hostBridge) (any, error) {
			return hostChooseFolder(h, "/Users/example/hires", folderForFloor)
		},
		"the folder panel for the hires, none picked": func(h hostBridge) (any, error) { return hostChooseFolder(h, "", folderForHires) },
		"the hires' folder panel closed":              func(h hostBridge) (any, error) { return nil, hostCloseFolderPanel(h) },
		"the Summary key, from the Keychain":          func(h hostBridge) (any, error) { return summaryStore(h).get() },
		"the GM's token, from the Keychain item the Swift engine kept it in": func(h hostBridge) (any, error) {
			return gmStoreFor(h).read()
		},
		"a new GM's token, kept in the Keychain": func(h hostBridge) (any, error) {
			return nil, gmStoreFor(h).write("fake-gm-token-other-00000000000000000000000")
		},
		"the GM's token, taken away":                  func(h hostBridge) (any, error) { return nil, gmStoreFor(h).write("") },
		"the portable office's id, from the Keychain": func(h hostBridge) (any, error) { return viewerStoreFor(h).get(viewerIDAccount) },
		// None: surely none, so a first key is made; the app's no: there but unreadable, never made again.
		"the portable office's key, none yet": func(h hostBridge) (any, error) {
			store := viewerStoreFor(h)
			_, err := store.get(viewerKeyAccount)
			return store.notFound(err), nil
		},
		"the portable office's write secret, there but unreadable": func(h hostBridge) (any, error) {
			store := viewerStoreFor(h)
			_, err := store.get(viewerSecretAccount)
			return store.notFound(err), err
		},
		"the portable office's id, kept": func(h hostBridge) (any, error) {
			return nil, viewerStoreFor(h).set(viewerIDAccount, "ZyXwVuTsRqPoNmLkJiHgFe")
		},
		"where you are, for the portable office: idle, locked, and the unlocks and wakes seen": func(h hostBridge) (any, error) {
			return hostPresence(h)
		},
		"Accessibility asked for":                     func(h hostBridge) (any, error) { return nil, hostAskAccessibility(h) },
		"the install line, a copy run from Downloads": func(h hostBridge) (any, error) { return hostInstallCheck(h) },
		"the install line, a translocated copy of one already in Applications": func(h hostBridge) (any, error) {
			return hostInstallCheck(h)
		},
		"the install line, a copy in Applications": func(h hostBridge) (any, error) { return hostInstallCheck(h) },
		"the install line, a test copy":            func(h hostBridge) (any, error) { return hostInstallCheck(h) },
		"the app moved to Applications":            func(h hostBridge) (any, error) { return nil, hostInstall(h) },
		"the app moved to Applications, refused":   func(h hostBridge) (any, error) { return nil, hostInstall(h) },
		"keys in other apps switched on":           func(h hostBridge) (any, error) { return nil, hostKeystrokesOn(h) },
		"an app allowed keys": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes"}, "com.example.mail", true)
		},
		"an app taken off, any capitals": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes", "com.example.mail"}, "COM.EXAMPLE.NOTES", false)
		},
		"a terminal never allowed keys": func(h hostBridge) (any, error) {
			return hostKeystrokeApps(h, []string{"com.example.notes"}, "com.apple.Terminal", true)
		},
		"dictation's names, checked": func(h hostBridge) (any, error) {
			return hostGlossary(h, []string{"Smolcorp", " smolcorp ", "Tailscale", ""})
		},
		"dictation got ready, with a problem":     func(h hostBridge) (any, error) { return hostPrepareDictation(h, "fr_FR") },
		"dictation in a language it doesn't know": func(h hostBridge) (any, error) { return hostPrepareDictation(h, "xx_XX") },
		"the office's mic, recording for a Terminal tab": func(h hostBridge) (any, error) {
			return hostDictationStart(h, "tty:/dev/ttys003", "fr_FR", false)
		},
		"the office's mic, recording that couldn't start": func(h hostBridge) (any, error) {
			return hostDictationStart(h, "tty:/dev/ttys004", "en_US", false)
		},
		"the office's mic, while the core is recording already": func(h hostBridge) (any, error) {
			return hostDictationStart(h, "tty:/dev/ttys003", "fr_FR", true)
		},
		"the clean-up switched off, its last problem forgotten": func(h hostBridge) (any, error) {
			return nil, hostDictationCleanupOff(h)
		},
		"the office's mic let go, what it heard": func(h hostBridge) (any, error) { return hostDictationStop(h) },
		"the office's mic let go, nothing heard": func(h hostBridge) (any, error) { return hostDictationStop(h) },
		"what it heard, cleaned up by Claude": func(h hostBridge) (any, error) {
			return hostDictationClean(h, "Fix the uh small corp build", dictationClean{cleanup: true, desks: []string{"smolcorp", "website"}})
		},
		"what it heard, the clean-up off": func(h hostBridge) (any, error) {
			return hostDictationClean(h, "hello there", dictationClean{glossary: &[]string{}})
		},
		"dictation's text, into a Terminal tab, its agent in front": func(h hostBridge) (any, error) {
			_, err := ask(h, bridgeRequest{Action: "insert", Tab: "tty:/dev/ttys003", Agent: Codex, Text: "Fix the Smolcorp build"})
			return nil, err
		},
		"dictation's text, to an agent that has ended": func(h hostBridge) (any, error) {
			answer, err := ask(h, bridgeRequest{Action: "insert", Tab: "tty:/dev/ttys005", Agent: Codex, Text: "Fix the build"})
			return errors.Is(placeError(answer, err), errAgentGone), err
		},
		"Add smolcorp to Terminal, in a test copy": func(h hostBridge) (any, error) { return nil, hostTerminalCommand(h, true) },
		"the desks' terminals, from the drivers' tables": func(h hostBridge) (any, error) {
			return hostAbilities(h, []string{"iterm:AB12", "tty:/dev/ttys003", "headless:3f2a9c1e"})
		},
		"the desks whose agent has left its tab, read only": func(h hostBridge) (any, error) {
			return hostAgentsEnded(h, []string{"tty:/dev/ttys005", "iterm:AB12"}, []Agent{Claude, Codex})
		},
		"a hire's own window closed strictly, only a shell left there": func(h hostBridge) (any, error) {
			return errors.Is(macTab{name: "tty:/dev/ttys005", host: h}.CloseStrict(Claude), errTabClosed), nil
		},
		"Grok Bots looked for in the Grok Bot app": func(h hostBridge) (any, error) {
			return hostResolveBots(h, []bridgeBot{{Bot: "Smolcorp Mini"}, {Bot: "Nobody Here"}})
		},
		"the phone's text, one line, into a Terminal tab": func(h hostBridge) (any, error) {
			return phoneText(h, "tty:/dev/ttys003", Claude, "Bonjour l’équipe « ça marche » 👩‍💻 ;", idle)
		},
		"the phone's text, several lines, as one paste in iTerm2": func(h hostBridge) (any, error) {
			return phoneText(h, "iterm:AB12", Claude, "first line\n    indented\tand a tab", idle)
		},
		"the phone's text, several lines, to an agent not known to take one paste": func(h hostBridge) (any, error) {
			return phoneText(h, "tty:/dev/ttys004", Codex, "one\ntwo", idle)
		},
		"the phone's text, to an agent that has ended": func(h hostBridge) (any, error) {
			result, err := hostPhoneText(h, "tty:/dev/ttys005", Claude, "hello", idle)
			return []any{result, errors.Is(err, errAgentGone)}, err
		},
		"the phone's text, starting with !, refused": func(h hostBridge) (any, error) {
			return phoneText(h, "tty:/dev/ttys003", Claude, "!ls", idle)
		},
		"the phone's text, held: the core said it's asking as the terminal got to it": func(h hostBridge) (any, error) {
			return phoneText(h, "tty:/dev/ttys006", Claude, "hello", func() SessionState { return Asking })
		},
		"the phone's text, several lines, ⌘V the terminal hadn't read in time": func(h hostBridge) (any, error) {
			return phoneText(h, "iterm:CD34", Claude, "one\ntwo", idle)
		},
		"the peek at the input line": func(h hostBridge) (any, error) { return hostPeek(h, "tty:/dev/ttys003", Claude) },
		"the peek, the agent not in front": func(h hostBridge) (any, error) {
			lines, err := hostPeek(h, "tty:/dev/ttys005", Claude)
			return []any{lines == nil, errors.Is(err, errAgentBehind)}, err
		},
		"the API-key account, offered for a hire (the key isn't read)": func(h hostBridge) (any, error) {
			available, notice, refusal, err := hostAPIKeyOffer(h)
			return []any{available, notice == apiKeyNotice, refusal}, err
		},
		"the API-key account's key, for the hire being made": func(h hostBridge) (any, error) { return hostAPIKeyRead(h) },
		"the processes working in a hire's worktree, or naming its session": func(h hostBridge) (any, error) {
			return hostProcesses(h, "/Users/ada/repo-hires/ada-abcdef", "6f1c2a9e-0b3d-4e5f-8a7b-1c2d3e4f5a6b")
		},
		"the processes, on a Mac that can't list them": func(h hostBridge) (any, error) {
			return hostProcesses(h, "/Users/ada/repo-hires/ada-abcdef", "")
		},
		"T3 Code's app looked for by its bundle id": func(h hostBridge) (any, error) { return hostT3App(h) },
		"T3 Code's app opened":                      func(h hostBridge) (any, error) { return nil, hostT3OpenApp(h) },
		"a hire's thread opened by T3 Code's own link": func(h hostBridge) (any, error) {
			return nil, hostT3Link(h, t3DeepLink("mcp:abc-1", "https://app.t3.codes/threads/local/mcp:abc-1"))
		},
		"any other link refused": func(h hostBridge) (any, error) {
			return nil, hostT3Link(h, "https://example.com/threads/local/mcp%3Aabc-1")
		},
		"the Grok Bot app's bots, for the core to find": func(h hostBridge) (any, error) {
			rows, readable, err := hostBotsRoster(h)
			return []any{rows, readable}, err
		},
		"the jukebox: what's playing and the volume": func(h hostBridge) (any, error) {
			playing, volume, err := hostMusicState(h)
			return []any{*playing, *volume}, err
		},
		"the jukebox's play key, without Accessibility":             func(h hostBridge) (any, error) { return nil, hostMusicKey(h, "playpause") },
		"the jukebox's volume set":                                  func(h hostBridge) (any, error) { return nil, hostSetVolume(h, 30) },
		"how the Mac writes a short time, for a webhook delivery's": func(h hostBridge) (any, error) { return hostTimeFormat(h) },
		"the Mac's screens, read afresh":                            func(h hostBridge) (any, error) { return hostScreens(h) },
		"the hires' windows tiled in Terminal, one with no window": func(h hostBridge) (any, error) {
			placed, needsPermission, err := hostTile(h, "terminal", []tilePlace{{"/dev/ttys003", tileRect{0, 30, 480, 573}}, {"/dev/ttys004", tileRect{480, 603, 480, 573}}})
			return []any{placed, needsPermission}, err
		},
		"tiling refused by iTerm2's Automation permission": func(h hostBridge) (any, error) {
			placed, needsPermission, err := hostTile(h, "iterm", []tilePlace{{"/dev/ttys005", tileRect{0, 30, 480, 573}}})
			return []any{placed, needsPermission}, err
		},
		"tiling refused in a test copy": func(h hostBridge) (any, error) {
			placed, needsPermission, err := hostTile(h, "terminal", []tilePlace{{"/dev/ttys003", tileRect{0, 30, 480, 573}}})
			return []any{placed, needsPermission}, err
		},
		"the office's window put under the team": func(h hostBridge) (any, error) {
			return nil, hostPlaceOffice(h, tileRect{0, 1750, 1440, 810}, 2560)
		},
	}
	// What the core makes of each answer.
	got := map[string]any{
		"setup's facts, the apps keys may go to left out of those running": []any{false, true, []string{"fr_FR", "en_US"}, true,
			[]runningApp{{ID: "com.example.mail", Name: "Mail"}}, "absent", true,
			&macOfficeKey{Held: "alt+cmd+Q", Labels: map[string]string{"Q": "A", "A": "Q"}}},
		"Open at login, read":                                      false,
		"Open at login, switched on":                               true,
		"the folder panel for a floor, a folder picked":            "/Users/example/project",
		"the folder panel for the hires, none picked":              "",
		"the Summary key, from the Keychain":                       "fake-summary-key",
		"the portable office's id, from the Keychain":              "AbCdEfGhIjKlMnOpQrStUv",
		"the portable office's key, none yet":                      true,
		"the portable office's write secret, there but unreadable": false,
		"where you are, for the portable office: idle, locked, and the unlocks and wakes seen": macPresence{idle: 2500 * time.Millisecond,
			unlocks: 3, wakes: 1},
		"the GM's token, from the Keychain item the Swift engine kept it in": "fake-gm-token-from-1-24-0000000000000000000",
		"an app allowed keys":                                                         []string{"com.example.notes", "com.example.mail"},
		"an app taken off, any capitals":                                              []string{"com.example.mail"},
		"a terminal never allowed keys":                                               []string{},
		"dictation's names, checked":                                                  []string{"Smolcorp", "Tailscale"},
		"dictation got ready, with a problem":                                         "Apple's model for French couldn't be downloaded.",
		"dictation in a language it doesn't know":                                     "",
		"the office's mic, recording for a Terminal tab":                              true,
		"the office's mic, recording that couldn't start":                             false,
		"the office's mic, while the core is recording already":                       false,
		"the office's mic let go, what it heard":                                      "Fix the uh small corp build",
		"the office's mic let go, nothing heard":                                      "",
		"what it heard, cleaned up by Claude":                                         dictationCleaned{text: "Fix the Smolcorp build", note: "cleaned in 0.9 s", cleaned: true},
		"what it heard, the clean-up off":                                             dictationCleaned{text: "hello there"},
		"dictation's text, to an agent that has ended":                                true,
		"the desks' terminals, from the drivers' tables":                              []tabAbilities{{true, true, true, true}, {true, true, true, true}, {}},
		"the desks whose agent has left its tab, read only":                           []bool{true, false},
		"a hire's own window closed strictly, only a shell left there":                true,
		"Grok Bots looked for in the Grok Bot app":                                    []*botResolution{{BotID: "0a1b-2c3d", Name: "Smolcorp Mini Manager", By: "guess"}, nil},
		"the phone's text, one line, into a Terminal tab":                             []any{pasteDone, false},
		"the phone's text, several lines, as one paste in iTerm2":                     []any{pasteDone, false},
		"the phone's text, several lines, to an agent not known to take one paste":    []any{pasteFailed, false},
		"the phone's text, to an agent that has ended":                                []any{pasteFailed, true},
		"the phone's text, starting with !, refused":                                  []any{pasteFailed, false},
		"the phone's text, held: the core said it's asking as the terminal got to it": []any{pasteFailed, false},
		// The app's no, but the words may be in: 502 pasteUnsaid, never a 409 that offers them again.
		"the phone's text, several lines, ⌘V the terminal hadn't read in time": []any{pasteFailed, true},
		"the peek at the input line":                                   []string{"> Bonjour l’équipe", "  ? for shortcuts"},
		"the peek, the agent not in front":                             []any{true, true},
		"the API-key account, offered for a hire (the key isn't read)": []any{true, true, ""},
		"the API-key account's key, for the hire being made":           "fk-m4c-0123456789",
		"the processes working in a hire's worktree, or naming its session": []bridgeProcess{{4242, "claude", "folder"}, {4343, "node", "add-dir"},
			{4444, "tail", "session"}},
		"the processes, on a Mac that can't list them": []bridgeProcess(nil),
		"T3 Code's app looked for by its bundle id":    true,
		"the Grok Bot app's bots, for the core to find": []any{[]grokRow{{ID: "0a1b-2c3d", Name: "Smolcorp Mini Manager"},
			{ID: "9f8e", Name: "Two bots", IsGroup: true}}, true},
		"the jukebox: what's playing and the volume": []any{true, musicVolume{level: 45}},
		"the install line, a copy run from Downloads": check{ID: "install", Status: checkFix, Title: "Smolcorp is in Applications", Fix: fixInstall,
			Label: "Move to Applications", Detail: "It's running from /Users/example/Downloads, where macOS may not keep it. In Applications it stays put, and opens at login."},
		"the install line, a translocated copy of one already in Applications": check{ID: "install", Status: checkFix, Title: "Smolcorp is in Applications",
			Fix: fixInstall, Label: "Move to Applications", Detail: "macOS is opening a temporary copy of Smolcorp because it was just downloaded. If this is the copy you just downloaded, move it to Applications. If Smolcorp is already there, quit it, run this in Terminal, then open it again: xattr -d com.apple.quarantine '/Applications/Smolcorp.app'. After that, allow Accessibility."},
		"the install line, a copy in Applications": check{ID: "install", Status: checkOK, Title: "Smolcorp is in Applications", Detail: "As /Applications/Smolcorp.app."},
		"the install line, a test copy": check{ID: "install", Status: checkOK, Title: "Smolcorp is in Applications",
			Detail: "A test copy, beside the Smolcorp you use: it stays where it was built."},
		"how the Mac writes a short time, for a webhook delivery's": macTimeFormat{"h:mm a", "AM", "PM"},
		"the Mac's screens, read afresh": []macScreen{{ID: "1", Name: "Built-in Retina Display", Frame: tileRect{0, 0, 1512, 982}, Visible: tileRect{0, 0, 1512, 950}, Main: true},
			{ID: "2", Name: "PA278CV", Frame: tileRect{-1440, 0, 1440, 2560}, Visible: tileRect{-1440, 0, 1440, 2530}}},
		"the hires' windows tiled in Terminal, one with no window": []any{[]string{"/dev/ttys003"}, false},
		"tiling refused by iTerm2's Automation permission":         []any{[]string{}, true},
	}
	cases := readBridgeCases(t)
	if len(cases) != len(want) {
		t.Errorf("the fixture has %d cases, this test %d", len(cases), len(want))
	}
	for _, c := range cases {
		run, ok := want[c.Name]
		if !ok {
			t.Errorf("%q: not a case this test knows", c.Name)
			continue
		}
		fake := &fixtureHost{}
		if err := json.Unmarshal(c.Answer, &fake.answer); err != nil {
			t.Fatal(err)
		}
		result, err := run(fake)
		if fake.answer.OK != (err == nil) {
			t.Errorf("%q: error %v, for an answer ok=%v", c.Name, err, fake.answer.OK)
		}
		if refusal := appRefusal(""); !fake.answer.OK && !errors.As(err, &refusal) {
			t.Errorf("%q: %v isn't the app's refusal", c.Name, err)
		}
		// The app says a test copy's no in the words the core's own refusal has (B1, RGAPC).
		if c.Name == "tiling refused in a test copy" && (err == nil || err.Error() != testCopyKeepsWindows) {
			t.Errorf("%q: the app said %v, want %q", c.Name, err, testCopyKeepsWindows)
		}
		if fake.asked == nil {
			t.Errorf("%q: nothing asked", c.Name)
			continue
		}
		sent, _ := json.Marshal(fake.asked)
		var asked, fixture map[string]any
		_ = json.Unmarshal(sent, &asked)
		_ = json.Unmarshal(c.Request, &fixture)
		if !reflect.DeepEqual(asked, fixture) {
			t.Errorf("%q: the core asks %s, the fixture %s", c.Name, sent, c.Request)
		}
		if expected, ok := got[c.Name]; ok && !reflect.DeepEqual(result, expected) {
			t.Errorf("%q: the core reads %#v, want %#v", c.Name, result, expected)
		}
	}
}

// Without the Mac app (Windows, Linux) nothing of the Mac's is said or served: nothing is made up.
func TestWithoutTheMacAppSetupAndTheOfficeHaveNoneOfTheMacs(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	// Ten: nine desks downstairs, and one upstairs for want of a desk.
	d.mu.Lock()
	running(d, 10)
	d.mu.Unlock()
	state := (&router{deck: d}).setupState()
	for _, field := range []string{"dictation", "keystrokes", "dock", "terminalCommand"} {
		if _, ok := state[field]; ok {
			t.Errorf("setup's state has %s", field)
		}
	}
	if _, ok := state["checklist"].(map[string]any)["accessibility"]; ok {
		t.Error("setup's checklist has accessibility")
	}
	if _, ok := state["cloudBots"].(map[string]any)["autoDetect"]; ok {
		t.Error("setup's bots have autoDetect")
	}
	for _, path := range []string{"/setup/dock?enabled=0", "/setup/accessibility", "/setup/keystrokes?enabled=1", "/setup/dictation?enabled=1",
		"/setup/terminal-command", "/setup/bots/resolve", "/setup/dictation/glossary", "/setup/keystrokes/apps?add=com.example.mail"} {
		if code, _ := pageCall(t, d, "POST", path); code != http.StatusNotFound {
			t.Errorf("POST %s: %d, want 404", path, code)
		}
	}
	d.mu.Lock()
	var snapshot map[string]any
	_ = json.Unmarshal(d.snapshot(), &snapshot)
	d.mu.Unlock()
	if _, ok := snapshot["dictation"]; ok {
		t.Error("the snapshot has dictation")
	}
	// Every desk this program drives, wherever it's told of: the keys, the floors and upstairs.
	var desks []map[string]any
	keys, upstairs, _ := deckState(t, d)
	for _, key := range keys {
		if desk, ok := key["session"].(map[string]any); ok {
			desks = append(desks, desk)
		}
	}
	desks = append(desks, upstairs...)
	floors, _ := snapshot["floors"].([]any)
	onFloors := 0
	for _, floor := range floors {
		list, _ := floor.(map[string]any)["desks"].([]any)
		for _, desk := range list {
			desks = append(desks, desk.(map[string]any))
			onFloors++
		}
	}
	if len(upstairs) == 0 || onFloors == 0 {
		t.Fatalf("%d desks upstairs, %d on the floors: not the office this test needs", len(upstairs), onFloors)
	}
	for _, desk := range desks {
		for _, field := range []string{"keys", "fire", "dictates", "choice"} {
			if _, ok := desk[field]; ok {
				t.Errorf("desk %v has %s", desk["id"], field)
			}
		}
	}
}

// Under the Mac app, setup's Mac switches are kept here and done by the app, as the Swift engine does them.
func TestTheMacsSwitchesAreKeptHereAndDoneByTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	state := (&router{deck: d}).setupState()
	if state["dock"].(map[string]any)["enabled"] != true || state["checklist"].(map[string]any)["accessibility"] != false ||
		state["cloudBots"].(map[string]any)["autoDetect"] != true || state["terminalCommand"].(terminalCmd).State != "absent" {
		t.Errorf("setup's state: %v %v %v %v", state["dock"], state["checklist"], state["cloudBots"], state["terminalCommand"])
	}
	dictation := state["dictation"].(map[string]any)
	if dictation["language"] != "en_US" || dictation["enabled"] != false || dictation["available"] != true {
		t.Errorf("dictation, before it's set: %v", dictation)
	}

	if code, text := pageCall(t, d, "POST", "/setup/dock?enabled=0"); code != http.StatusOK || loadConfig().ShowInDock == nil || *loadConfig().ShowInDock {
		t.Errorf("In the Dock off: %d %s, kept %v", code, text, loadConfig().ShowInDock)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes?enabled=yes"); code != http.StatusBadRequest {
		t.Errorf("keystrokes, neither 1 nor 0: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes?enabled=1"); code != http.StatusOK || !loadConfig().KeystrokesAllowed {
		t.Errorf("keystrokes on: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes/apps?add=com.example.mail"); code != http.StatusOK ||
		!slices.Equal(loadConfig().KeystrokeApps, []string{"com.example.mail"}) {
		t.Errorf("an app added: %d, kept %v", code, loadConfig().KeystrokeApps)
	}
	mac.refusing(map[string]string{"keystroke-apps": "Keys are never sent to a terminal."})
	if code, text := pageCall(t, d, "POST", "/setup/keystrokes/apps?add=com.apple.terminal"); code != http.StatusConflict || text != "Keys are never sent to a terminal." {
		t.Errorf("a terminal added: %d %q, want 409 in the app's words", code, text)
	}
	mac.refusing(nil)
	if code, _ := pageCall(t, d, "POST", "/setup/keystrokes/apps"); code != http.StatusBadRequest {
		t.Errorf("neither add nor remove: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/dictation?language=xx_XX"); code != http.StatusBadRequest {
		t.Errorf("an unknown language: %d", code)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/dictation?enabled=1&language=fr_FR&claude=1"); code != http.StatusOK {
		t.Errorf("dictation on: %d", code)
	}
	if c := loadConfig(); !c.DictationEnabled || c.DictationLanguage != "fr_FR" || !c.DictationForClaude {
		t.Errorf("dictation kept as %v %q %v", c.DictationEnabled, c.DictationLanguage, c.DictationForClaude)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/accessibility"); code != http.StatusNoContent {
		t.Errorf("Accessibility asked for: %d", code)
	}
	mac.refusing(map[string]string{"terminal-command": "This is a test copy of Smolcorp."})
	if code, text := pageCall(t, d, "POST", "/setup/terminal-command"); code != http.StatusConflict || text != "This is a test copy of Smolcorp." {
		t.Errorf("Add smolcorp to Terminal in a test copy: %d %q", code, text)
	}
	mac.refusing(nil)
	// What was done, not what was only read: setup's state reads the facts, the bots, the abilities, the login
	// item and whether a Summary key is kept.
	var done []string
	mac.mu.Lock()
	for _, request := range mac.asked {
		read := request.Action == "secret-read" || request.Action == "login-item" && request.On == nil
		background := request.Action == "bots-roster" // the bots looked for in the Grok Bot app, at the deck's start
		if !read && !background && request.Action != "facts" && request.Action != "bots-resolve" && request.Action != "abilities" && request.Action != "api-key-offer" {
			done = append(done, request.Action)
		}
	}
	mac.mu.Unlock()
	want := []string{"dock", "keystrokes-on", "keystroke-apps", "keystroke-apps", "dictation", "accessibility", "terminal-command"}
	if !slices.Equal(done, want) {
		t.Errorf("the app was asked %v, want %v", done, want)
	}
	// Switched on: in the event log, as on the Mac.
	found := d.events.answer(d.events.wait(t.Context(), 0, 50, 0), 0)
	data, _ := json.Marshal(found)
	if !strings.Contains(string(data), `"detail":"switched on"`) || !strings.Contains(string(data), `"detail":"app allowed: com.example.mail"`) {
		t.Errorf("the event log: %s", data)
	}
}

// The desks' abilities are asked of the app once per set of tabs, on their own, never per desk or per snapshot.
func TestTheDesksAbilitiesAreAskedOncePerSetOfTabs(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	// Both tabs in under one hold of the lock: the deck's own rounds look at the sessions on their own, and one
	// that came between two would ask about the first tab alone, then again about both.
	d.mu.Lock()
	for i, tab := range []string{"tty:/dev/ttys901", "iterm:AB12"} {
		d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m" + string(rune('0'+i)), "cwd": "/work/mac"},
			Claude, tab, "", float64(100+i))
	}
	d.mu.Unlock()
	desk := learnedDesk(t, d)
	if desk["keys"] != true || desk["fire"] != true || desk["dial"] != true || desk["mic"] != true || desk["dictates"] != false || desk["choice"] != nil {
		t.Errorf("a desk under the Mac app: %v", desk)
	}
	for range 5 {
		deckState(t, d)
	}
	count := func() int {
		n := 0
		for _, action := range mac.actions() {
			if action == "abilities" {
				n++
			}
		}
		return n
	}
	if count() != 1 {
		t.Errorf("asked %d times for the same tabs, want 1", count())
	}
	applied(d, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m9", "cwd": "/work/other"}, Claude, "tty:/dev/ttys909", 200)
	learnedDesk(t, d)
	if count() != 2 {
		t.Errorf("asked %d times after a new tab, want 2", count())
	}
	var snapshot map[string]any
	d.mu.Lock()
	_ = json.Unmarshal(d.snapshot(), &snapshot)
	d.mu.Unlock()
	if dictation, _ := snapshot["dictation"].(map[string]any); dictation["on"] != false || dictation["language"] != "" && dictation["language"] != "en_US" {
		t.Errorf("the snapshot's dictation: %v", snapshot["dictation"])
	}
}

// applied is a hook's event applied as the server applies it, with the deck's lock: its rounds read the sessions too.
func applied(d *Deck, event map[string]any, agent Agent, tab string, at float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(event, agent, tab, "", at)
}

// learnedDesk is the first desk once the app has said, or given up saying, what its terminal lets the office do.
func learnedDesk(t *testing.T, d *Deck) map[string]any {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		d.mu.Lock()
		d.snapshot()
		learning := d.mac.learning || d.mac.abilities == nil && d.mac.unanswered < abilitiesTries
		d.mu.Unlock()
		if !learning {
			break
		}
	}
	keys, _, _ := deckState(t, d)
	return keys[0]["session"].(map[string]any)
}

// A terminal the app can't type into has no dial and no mic, and no keys or fire where it can't press keys.
func TestADesksControlsAreWhatItsTerminalAllows(t *testing.T) {
	mac := withMac(t)
	mac.abilities = tabAbilities{Keys: true}
	d := newTestDeck(t)
	applied(d, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m1", "cwd": "/work/mac"}, Claude, "tty:/dev/ttys901", 100)
	if desk := learnedDesk(t, d); desk["keys"] != true || desk["mic"] != true || desk["dial"] != false || desk["fire"] != false {
		t.Errorf("keys alone: %v", desk)
	}
}

// The abilities the app didn't answer are asked again a little later, a few times at most.
func TestTheDesksAbilitiesAreAskedAgainWhenTheAppDidntAnswer(t *testing.T) {
	saved := abilitiesRetryWait
	t.Cleanup(func() { abilitiesRetryWait = saved })
	abilitiesRetryWait = 100 * time.Millisecond
	mac := withMac(t)
	mac.refusing(map[string]string{"abilities": "Not now."})
	d := newTestDeck(t)
	asked := func() int {
		return len(slices.DeleteFunc(mac.actions(), func(action string) bool { return action != "abilities" }))
	}
	waitFor := func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !done(); time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("never %s", what)
			}
		}
	}
	desk := func(id string) map[string]any {
		t.Helper()
		keys, _, _ := deckState(t, d)
		for _, key := range keys {
			if s, _ := key["session"].(map[string]any); s != nil && s["id"] == id {
				return s
			}
		}
		t.Fatalf("no desk %s", id)
		return nil
	}

	// Not answered at first, answered at the next try: the desk has its controls.
	applied(d, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m1", "cwd": "/work/mac"}, Claude, "tty:/dev/ttys901", 100)
	deckState(t, d)
	waitFor("asked", func() bool { return asked() == 1 })
	if got := desk("m1"); got["keys"] != false {
		t.Errorf("before the app has said: %v", got)
	}
	mac.refusing(nil)
	waitFor("answered at the next try", func() bool { return desk("m1")["keys"] == true })
	if asked() != 2 {
		t.Errorf("asked %d times, answered at the second: want 2", asked())
	}

	// Never answered: asked a few times, then not again until the tabs change.
	mac.refusing(map[string]string{"abilities": "Not now."})
	applied(d, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "m2", "cwd": "/work/mac"}, Claude, "tty:/dev/ttys902", 200)
	deckState(t, d)
	waitFor("given up", func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return !d.mac.learning && d.mac.unanswered >= abilitiesTries
	})
	time.Sleep(5 * abilitiesRetryWait)
	for range 3 {
		deckState(t, d)
	}
	if asked() != 2+abilitiesTries {
		t.Errorf("asked %d times in all, the new tabs never answered: want %d", asked(), 2+abilitiesTries)
	}
	if got := desk("m1"); got["keys"] != true {
		t.Errorf("what the app said before is kept: %v", got)
	}
	if got := desk("m2"); got["keys"] != false {
		t.Errorf("a tab the app never answered for: %v", got)
	}
}

// The language is the Mac's from the start, and a kept one it doesn't have isn't used; more apps than the Mac
// app keeps are left out as the config loads; In the Dock is kept only once the app has done it.
func TestTheMacsSettingsAreReadAsTheMacAppReadsThem(t *testing.T) {
	mac := withMac(t)
	isolate(t)
	apps := make([]string, 0, 60)
	for i := range 60 {
		apps = append(apps, "com.example.app"+strconv.Itoa(i))
	}
	written, _ := json.Marshal(map[string]any{"keystrokeApps": apps, "dictationLanguage": "xx_XX"})
	if err := os.WriteFile(configFile, written, 0o600); err != nil {
		t.Fatal(err)
	}
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	d.mu.Lock()
	kept := slices.Clone(d.config.KeystrokeApps)
	d.mu.Unlock()
	if len(kept) != keystrokeAppsMost || kept[0] != "com.example.app0" {
		t.Errorf("the apps kept: %d %v", len(kept), kept)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		d.mu.Lock()
		language := d.dictationLanguage()
		d.mu.Unlock()
		if language == "en_US" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the office's language at the start: %q, want the Mac's en_US", language)
		}
	}
	if state := (&router{deck: d}).setupState(); state["keystrokes"] == nil {
		t.Error("setup has none of the Mac's with the most apps kept")
	}

	mac.refusing(map[string]string{"dock": "Not now."})
	if code, _ := pageCall(t, d, "POST", "/setup/dock?enabled=0"); code != http.StatusConflict || loadConfig().ShowInDock != nil {
		t.Errorf("In the Dock off, the app saying no: %d, kept %v", code, loadConfig().ShowInDock)
	}
}

// A Grok Bot set without its id is given the one the Grok Bot app has; the bots are answered with who they are there.
func TestGrokBotsAreFoundInTheGrokBotAppThroughTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	body := `{"bots": [{"id": "mini", "name": "Mini", "source": {"kind": "grokBotApp", "bot": "Smolcorp Mini"}}, {"id": "plain", "name": "Plain"}]}`
	w := serve(t, d, call{method: "POST", target: "/setup/bots", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: body})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"resolved":null`) {
		t.Fatalf("the bots: %d %s", w.Code, w.Body)
	}
	found := &fixtureHost{answer: bridgeAnswer{OK: true, Resolved: []*botResolution{{BotID: "0a1b-2c3d", Name: "Smolcorp Mini Manager", By: "guess"}}}}
	// The deck asks the app it was made under: this one stands in for it from now on.
	d.mu.Lock()
	d.mac.host = found
	d.mu.Unlock()
	code, text := pageCall(t, d, "POST", "/setup/bots/resolve")
	var answer struct {
		Bots               []map[string]any
		Filled, Unresolved []string
	}
	_ = json.Unmarshal([]byte(text), &answer)
	if code != http.StatusOK || !slices.Equal(answer.Filled, []string{"mini"}) || len(answer.Unresolved) != 0 {
		t.Fatalf("resolve: %d %s", code, text)
	}
	if got := loadConfig().CloudBots[0].Source; got.BotID != "0a1b-2c3d" || got.Bot != "Smolcorp Mini Manager" {
		t.Errorf("kept as %+v", got)
	}
	// Not only the last request: the deck's own look for the bots (after a change in setup) asks it too.
	if !found.sawAction("bots-resolve") {
		t.Errorf("the app was never asked to resolve the bots; it was asked %q", found.all)
	}
	_ = mac
}
