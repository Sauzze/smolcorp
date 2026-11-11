package desktop

import (
	"cmp"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// On a Mac, this program can run as Smolcorp.app's engine (`smolcorp core`,
// core_darwin.go): the app starts it, shows its pages in the app's windows,
// and keeps what only macOS lets an app do. The engine asks the app for those
// over the host bridge, a socket in the support folder that only the two of
// them use (each checks the other's pid; see socketBridge): keys typed
// into a Terminal, iTerm2 or Ghostty tab, the phone's text and a look at what
// the tab shows (phonetext.go), a window brought forward or closed,
// a terminal opened on a script, the app's own Keychain items (✨ Summary's key
// and the portable office's), its login item, and when you're back at the Mac
// (hostPresence);
// setup's Mac parts (hostsetup.go), the office's window and the Mac's folder
// panel (hostshell.go), and what each desk's terminal allows (hostoffice.go);
// and which processes work in a folder or name a session, read only, for Let
// go and Take over (t3process_darwin.go); T3 Code's app, found and opened by
// its bundle id, and a hire's thread opened by its link (t3code.go); the
// Grok Bot app's bots, for the core to find (grokbots.go), the jukebox
// (music_other.go), and the office's mic dictating: the Mac's microphone
// recording for a desk, and what it heard, cleaned up as asked
// (hostdictation.go). Each is
// the Mac app's own code, as the Swift engine runs it
// (HostBridge.swift); their requests and answers are Tests/fixtures/bridge/actions.json.
//
// On Windows and Linux there is no host: host stays nil, and every tab is
// driven as before.

// hostBridge is the Mac app, as the engine asks it: one request, one answer.
type hostBridge interface {
	call(bridgeRequest) (bridgeAnswer, error)
}

// host is the Mac app this engine runs under, or nil.
var host hostBridge

// bridgeRequest is one thing asked of the Mac app. The token is the
// transport's, added as it's sent.
type bridgeRequest struct {
	Token  string `json:"token,omitempty"`
	Action string `json:"action"`
	// A session's tab, as the deck names it (tty:/dev/ttys003, iterm:…, tmux:%3@…).
	Tab   string `json:"tab,omitempty"`
	Key   Key    `json:"key,omitempty"`
	Times int    `json:"times,omitempty"`
	Text  string `json:"text,omitempty"`
	Lines int    `json:"lines,omitempty"`
	Agent Agent  `json:"agent,omitempty"`
	// open: which terminal (terminal, iterm, ghostty), the folder, and a script
	// in the support folder's hires folder for the window to run. Never a command line.
	// processes: the folder looked at, and the Claude Code session whose name is looked for ("" for none).
	Terminal string `json:"terminal,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Script   string `json:"script,omitempty"`
	Session  string `json:"session,omitempty"`
	// t3-link: T3 Code's own link to a hire's thread (t3DeepLink).
	Link string `json:"link,omitempty"`
	// window-look, window-close: a window the app opened for this engine, by the record its open answered;
	// window-close: the tty it showed, as window-look said.
	Window string `json:"window,omitempty"`
	TTY    string `json:"tty,omitempty"`
	// choose-folder: what the panel is for, "hires" or "floor" (the app says it in its own words); Folder is
	// where it opens, "" for the panel's own choice.
	For string `json:"for,omitempty"`
	// secret-*: one of the app's own items (see bridgeSecrets).
	Item  string `json:"item,omitempty"`
	Value string `json:"value,omitempty"`
	// login-item, dock, office-ontop: on or off; keystroke-apps: App added or removed; terminal-command: made or removed.
	On *bool `json:"on,omitempty"`
	// office-key: the office's keys from anywhere, as config.json has them (officekey.go), "off" for none.
	Keys string `json:"keys,omitempty"`
	// facts: the apps keys may go to, left out of those running; keystroke-apps: the list App goes on or off.
	Apps []string `json:"apps,omitempty"`
	App  string   `json:"app,omitempty"`
	// glossary: dictation's names, as the page has them.
	Names []string `json:"names,omitempty"`
	// dictation: the language whose model gets ready; dictation-start: the language recorded in.
	Language string `json:"language,omitempty"`
	// dictation-start: this program is recording already, so the app checks the tab and says no (RGAPA N2).
	Busy bool `json:"busy,omitempty"`
	// dictation-clean: the clean-up's glossary as setup has it (none: the app's own), and the desks' names;
	// the text is Text, and whether to clean it up On.
	Glossary *[]string `json:"glossary,omitempty"`
	Desks    []string  `json:"desks,omitempty"`
	// abilities, agents-ended: the sessions' tabs, as the deck names them; agents-ended: each one's agent, in order.
	Tabs   []string `json:"tabs,omitempty"`
	Agents []Agent  `json:"agents,omitempty"`
	// close: only with the agent in front, never a tab with only its shell left (closeStrict).
	Strict bool `json:"strict,omitempty"`
	// bots-resolve: the Grok Bots to look for in the Grok Bot app.
	Bots []bridgeBot `json:"bots,omitempty"`
	// tile: the hires' windows to move (Terminal is the terminal), by the tty each shows; office-place: where the
	// office's window goes, in AppleScript's coordinates turned over by the main screen's height (gmscreens.go).
	Places     []tilePlace `json:"places,omitempty"`
	Office     *tileRect   `json:"office,omitempty"`
	MainHeight float64     `json:"mainHeight,omitempty"`
	// music: the jukebox's key pressed, one of musicKeys; volume: the level set, 0 to 100.
	Music string `json:"music,omitempty"`
	Level *int   `json:"level,omitempty"`
	// phone-text: the desk's state now, as this engine's sessions have it ("" for one gone). The Mac app asks
	// it on this request's own connection, from the terminal's queue, where the Swift engine looks at its own
	// sessions (Deck.typePhoneWords); the transport answers it (socketBridge). Never sent itself; nil is asking.
	deskState func() SessionState
	// How long this request may take, when longer than the transport's own: the Mac's folder panel waits
	// for you (folderPanelWait). Never sent itself.
	wait time.Duration
}

// bridgeVolume is the Mac's output volume, as the jukebox's bar shows it: 0 to 100, and whether it's muted.
type bridgeVolume struct {
	Level int  `json:"level"`
	Muted bool `json:"muted"`
}

// bridgeBot is a Grok Bot as set: its id in the Grok Bot app, if known, and its name there.
type bridgeBot struct {
	BotID string `json:"botId"`
	Bot   string `json:"bot"`
}

// bridgeAnswer is the Mac app's answer.
type bridgeAnswer struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// The terminal can't do that at all (no driver for it): errUnsupported, not a tab that's gone.
	Unsupported bool `json:"unsupported,omitempty"`
	// open: the new window's tab, as the deck will know it, and the window's
	// own record (Terminal's carries its window id), which switching hires to
	// the background closes by; "" when the terminal gave none.
	Tab    string `json:"tab,omitempty"`
	Window string `json:"window,omitempty"`
	// terminals: which of Terminal, iTerm2 and Ghostty are installed (their ids).
	Terminals []string `json:"terminals,omitempty"`
	// window-look: "open" (showing TTY), "gone", or "unknown" (its terminal couldn't be asked).
	Shown string `json:"shown,omitempty"`
	TTY   string `json:"tty,omitempty"`
	// window-close: closed, or left open and why, in a few words.
	Closed bool   `json:"closed,omitempty"`
	Left   string `json:"left,omitempty"`
	// close: fired, gone or not-stopped, as the Mac's Terminal.Fired.
	Fired string `json:"fired,omitempty"`
	// agents-ended: for each tab asked, whether its agent is known to have left it.
	Ended []bool `json:"ended,omitempty"`
	// secret-read, api-key-read: the value, and whether there is one; t3-app: whether T3 Code's app is on the Mac.
	Value string `json:"value,omitempty"`
	Found bool   `json:"found,omitempty"`
	// secret-read of the GM's token: there is an item, but the Keychain couldn't read it (Found is false).
	Failed bool `json:"failed,omitempty"`
	// api-key-offer: whether a hire may work on the API-key account, the notice beside the choice, and
	// when it may not, the refusal a hire on it gets, in the Mac app's words.
	Available bool   `json:"available,omitempty"`
	Notice    string `json:"notice,omitempty"`
	Refusal   string `json:"refusal,omitempty"`
	// login-item: whether Smolcorp opens at login.
	On bool `json:"on,omitempty"`
	// choose-folder: the folder picked in the Mac's panel, "" for none.
	Folder string `json:"folder,omitempty"`
	// facts: what only the Mac can say, for setup.
	Facts *macFacts `json:"facts,omitempty"`
	// install-check: the checklist's install line, as the app says it (SelfTest.installCheck).
	Check *check `json:"check,omitempty"`
	// time-format: how this Mac writes a short time (macclock.go).
	TimeFormat *macTimeFormat `json:"timeFormat,omitempty"`
	// keystroke-apps: the list, with the app on it or off it; glossary: the names, checked.
	Apps  []string `json:"apps,omitempty"`
	Names []string `json:"names,omitempty"`
	// dictation: why the language's model isn't ready, or nothing.
	Problem string `json:"problem,omitempty"`
	// dictation-start: whether recording started. dictation-stop: what was heard. dictation-clean: the text
	// cleaned up or as heard, how that went for the desk's note ("" for none), and whether it was cleaned.
	Started bool   `json:"started,omitempty"`
	Text    string `json:"text,omitempty"`
	Note    string `json:"note,omitempty"`
	Cleaned bool   `json:"cleaned,omitempty"`
	// abilities: each tab's, in the order asked.
	Abilities []tabAbilities `json:"abilities,omitempty"`
	// bots-resolve: who each bot is in the Grok Bot app, in the order asked; null for one not there.
	Resolved []*botResolution `json:"resolved,omitempty"`
	// bots-roster: the Grok Bot app's bots now; null when its roster can't be read.
	Rows *[]grokRow `json:"rows,omitempty"`
	// music-state: whether music is playing (true, or null when that can't be told), and the volume
	// (null where it can't be set).
	Playing *bool         `json:"playing,omitempty"`
	Volume  *bridgeVolume `json:"volume,omitempty"`
	// phone-text: "pasted". Anything else comes as the app's no, in the Swift engine's words: nothing typed.
	Pasted string `json:"pasted,omitempty"`
	// phone-text: with the app's no, the words may have been typed all the same (⌘V the terminal hadn't read in
	// time, a terminal that didn't take it or stopped answering part way): never offered again, and no Return.
	MayHaveTyped bool `json:"mayHaveTyped,omitempty"`
	// phone-text, peek: refused as the agent isn't in front in its tab: "gone" (ended) or "behind" (the shell's in front).
	Place string `json:"place,omitempty"`
	// peek: the end of what the tab shows, as the phone's peek gives it.
	Peek []string `json:"peek,omitempty"`
	// Not the answer yet: the Mac app asking the desk's state, for the phone's text (bridgeRequest.deskState).
	Ready bool `json:"ready,omitempty"`
	// processes: those working in the folder or naming the session, the first bridgeProcessesMost.
	Processes []bridgeProcess `json:"processes,omitempty"`
	// presence: how long the keyboard and mouse have gone unused, in seconds, whether the screen is locked,
	// and how many times the screen was unlocked, and the Mac or its screens woke, since the app started.
	Idle    float64 `json:"idle,omitempty"`
	Locked  bool    `json:"locked,omitempty"`
	Unlocks int     `json:"unlocks,omitempty"`
	Wakes   int     `json:"wakes,omitempty"`
	// screens: the Mac's screens, read afresh; tile: the ttys whose windows were placed, and whether the terminal
	// refused Smolcorp its Automation permission (the work then stopped there).
	Screens         []macScreen `json:"screens,omitempty"`
	Placed          []string    `json:"placed,omitempty"`
	NeedsPermission bool        `json:"needsPermission,omitempty"`
}

// bridgeProcess is a process the Mac app found working in a folder or naming a session: its pid, its name as
// the kernel keeps it, and why: "folder" (its current folder is in it), "add-dir" (given it with --add-dir) or
// "session" (an argument names the session).
type bridgeProcess struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	Why  string `json:"why"`
}

// bridgeProcessesMost is the most processes the app answers (HostBridge.processesMost).
const bridgeProcessesMost = 64

// macFacts is what setup shows of this Mac, as the Swift engine's setup state
// has it (DeckRouter.setupState): asked of the app at each setup state.
type macFacts struct {
	// Whether macOS has given Smolcorp Accessibility.
	Accessibility bool `json:"accessibility"`
	// A test copy leaves your Terminal's commands and the Accessibility prompt to the copy in use.
	TestCopy  bool         `json:"testCopy"`
	Dictation macDictation `json:"dictation"`
	Running   []runningApp `json:"running"`
	Terminal  terminalCmd  `json:"terminalCommand"`
	// The office's keys held now and the layout in use (officekey.go).
	OfficeKey *macOfficeKey `json:"officeKey,omitempty"`
}

// macDictation is Apple's speech recognition on this Mac: whether it can
// dictate at all and has the microphone; the languages it knows, the one the
// Mac's own settings suggest, and the names its clean-up knows before you
// change them; and why the clean-up didn't work last, if it didn't.
type macDictation struct {
	Available      bool     `json:"available"`
	Allowed        bool     `json:"allowed"`
	Languages      []string `json:"languages"`
	Suggested      string   `json:"suggested"`
	Glossary       []string `json:"glossary"`
	CleanupProblem *string  `json:"cleanupProblem"`
}

// runningApp is an app running on the Mac that keys could go to.
type runningApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// terminalCmd is "Add smolcorp to Terminal", as the Mac's TerminalCommand.json says it.
type terminalCmd struct {
	Available bool   `json:"available"`
	State     string `json:"state"`
	Link      string `json:"link"`
	OnPath    bool   `json:"onPath"`
	PathLine  string `json:"pathLine"`
	PathFile  string `json:"pathFile"`
	TestCopy  bool   `json:"testCopy"`
}

// tabAbilities is what a tab's terminal lets the office do, as the Mac app's
// driver for it says (Terminals.abilities): press keys, type a line, fire its
// agent; and whether it can say what the tab shows.
type tabAbilities struct {
	Keys   bool `json:"keys"`
	Text   bool `json:"text"`
	Fire   bool `json:"fire"`
	Screen bool `json:"screen"`
}

// botResolution is who a bot is in the Grok Bot app now, and how that was
// told: "id", "name", or "guess" (the one name like it).
type botResolution struct {
	BotID string `json:"botId"`
	Name  string `json:"name"`
	By    string `json:"by"`
}

// bridgeTabsMost is the most tabs asked about at once: the app refuses more.
const bridgeTabsMost = 256

// keystrokeAppsMost is the most apps keys may go to, and bridgeTextMost the
// longest text in a list the app is sent: it refuses a list with more, or
// longer (Keystrokes.AppList.most, HostBridge.strings).
const (
	keystrokeAppsMost = 50
	bridgeTextMost    = 1024
)

// bridgeSecrets are the Keychain items the app owns that the engine may ask
// for: the ✨ Summary API key, the GM's token, and the portable office's id,
// key and write secret. In a test copy the app keeps them in files of its own.
var bridgeSecrets = []string{"summary-key", "gm-token", "viewer-id", "viewer-key", "viewer-write-secret"}

// ask sends request and turns a refusal into an error.
func ask(h hostBridge, request bridgeRequest) (bridgeAnswer, error) {
	answer, err := h.call(request)
	if err != nil {
		return answer, err
	}
	if !answer.OK {
		if answer.Unsupported {
			return answer, errUnsupported
		}
		if answer.Error == "" {
			answer.Error = "the Mac app said no"
		}
		return answer, appRefusal(answer.Error)
	}
	return answer, nil
}

// hostOpenTerminal opens a window of terminal in folder running script, a
// file the engine wrote in the hires folder, and says its tab and the
// window's record.
func hostOpenTerminal(h hostBridge, terminal, folder, script string) (tab, window string, err error) {
	answer, err := ask(h, bridgeRequest{Action: "open", Terminal: terminal, Folder: folder, Script: script})
	return answer.Tab, answer.Window, err
}

// hostLookWindow asks the Mac app about a window it opened for a hire, by its
// record: "open" and the tty it shows, "gone", or "unknown" when its terminal
// couldn't be asked (an app that said no is unknown too: nothing is closed).
func hostLookWindow(h hostBridge, window string) (shown, tty string) {
	answer, err := ask(h, bridgeRequest{Action: "window-look", Window: window})
	if err != nil || (answer.Shown == "open" && !isTTYPath(answer.TTY)) {
		return "unknown", ""
	}
	if answer.Shown != "open" && answer.Shown != "gone" {
		return "unknown", ""
	}
	return answer.Shown, answer.TTY
}

// hostCloseWindow asks the Mac app to close a window it opened for a hire, once
// nothing but its shell runs on tty: closed, or why it was left open.
func hostCloseWindow(h hostBridge, window, tty string) (closed bool, left string) {
	answer, err := ask(h, bridgeRequest{Action: "window-close", Window: window, TTY: tty})
	switch {
	case err != nil:
		return false, "the terminal didn't close it"
	case answer.Closed:
		return true, ""
	case answer.Left == "":
		return false, "the terminal didn't close it"
	}
	return false, answer.Left
}

// isTTYPath is a terminal's device, as the Mac's TabAddress.isTTY reads one: /dev/tty and letters or digits.
func isTTYPath(path string) bool {
	name, ok := strings.CutPrefix(path, "/dev/tty")
	return ok && name != "" && strings.IndexFunc(name, func(r rune) bool {
		return r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) < 0
}

// hostTerminals are the terminals the Mac app can open a window of, by id.
func hostTerminals(h hostBridge) ([]string, error) {
	answer, err := ask(h, bridgeRequest{Action: "terminals"})
	return answer.Terminals, err
}

// hostReadSecret reads one of the app's own items: found is false when there is none.
func hostReadSecret(h hostBridge, item string) (value string, found bool, err error) {
	answer, err := ask(h, bridgeRequest{Action: "secret-read", Item: item})
	return answer.Value, answer.Found, err
}

// hostAPIKeyOffer asks the Mac app whether a hire may work on the API-key
// account (the Summary key in its Keychain, a test copy's own file), with its
// notice and, when it may not, its refusal. The app never reads the key for it.
func hostAPIKeyOffer(h hostBridge) (available bool, notice, refusal string, err error) {
	answer, err := ask(h, bridgeRequest{Action: "api-key-offer"})
	return answer.Available, answer.Notice, answer.Refusal, err
}

// hostAPIKeyRead is the key itself, for the hire being made, only while the
// Mac app offers it: its refusal otherwise, in its words.
func hostAPIKeyRead(h hostBridge) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "api-key-read"})
	if err == nil && (!answer.Found || answer.Value == "") {
		err = errors.New("the Mac app gave no key")
	}
	return answer.Value, err
}

// hostWriteSecret keeps value as the item, or removes it when value is empty.
func hostWriteSecret(h hostBridge, item, value string) error {
	action := "secret-write"
	if value == "" {
		action = "secret-delete"
	}
	_, err := ask(h, bridgeRequest{Action: action, Item: item, Value: value})
	return err
}

// macPresence is "stop when I'm back at the Mac", as the Mac app tells it (presence): what the Swift
// engine's portable office watched (Viewer.swift at 1da4e935). The unlocks and the wakes (the Mac's own
// and its screens') are counted by the app from its start, so a count gone up is one since last asked.
type macPresence struct {
	idle           time.Duration
	locked         bool
	unlocks, wakes int
}

// hostPresence asks the Mac app how long the keyboard and mouse have gone unused, whether the screen is
// locked, and how many unlocks and wakes it has seen.
func hostPresence(h hostBridge) (macPresence, error) {
	answer, err := ask(h, bridgeRequest{Action: "presence"})
	if err != nil || answer.Idle < 0 || answer.Unlocks < 0 || answer.Wakes < 0 {
		return macPresence{}, cmp.Or(err, errors.New("the Mac app's presence isn't one"))
	}
	// Never used since the Mac started, or more than a year: away for as long as can be told.
	idle := time.Duration(min(answer.Idle, 365*24*3600) * float64(time.Second))
	return macPresence{idle: idle, locked: answer.Locked, unlocks: answer.Unlocks, wakes: answer.Wakes}, nil
}

// hostLoginItem says whether Smolcorp opens at login; with on, it sets it first.
func hostLoginItem(h hostBridge, on *bool) (bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "login-item", On: on})
	return answer.On, err
}

// The two folder panels the app opens for setup, each in its own words (Deck.hireFolderMessage,
// Deck.floorFolderMessage): the hires' folder, and a project's folder for a floor.
const (
	folderForHires = "hires"
	folderForFloor = "floor"
)

// folderPanelWait is how long the Mac's folder panel waits for you before the app closes it, as if
// cancelled; the core waits a little longer for the app's answer, so the app always answers first.
var folderPanelWait = 10 * time.Minute

// folderPanelMargin is how much longer than the panel the core waits for the app's answer.
const folderPanelMargin = 30 * time.Second

// hostChooseFolder opens the Mac's folder panel for purpose, at start ("" for the panel's own choice), and
// waits for you: the folder picked, or "" for none (cancelled, closed, or already open, which the app
// brings forward and answers at once, as the Swift engine's).
func hostChooseFolder(h hostBridge, start, purpose string) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "choose-folder", Folder: start, For: purpose, wait: folderPanelWait + folderPanelMargin})
	return answer.Folder, err
}

// hostCloseFolderPanel closes the Mac's folder panel if it's open for the hires' folder, as if cancelled.
func hostCloseFolderPanel(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "folder-panel-close"})
	return err
}

// hostShowOffice shows the office's window on the Mac, as the menu's Show Office does.
func hostShowOffice(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "office"})
	return err
}

// hostOfficeOnTop keeps the office's window on top of other windows, or not, at once: the setting is the
// core's to keep, in the config the app reads it from as it starts.
func hostOfficeOnTop(h hostBridge, on bool) error {
	_, err := ask(h, bridgeRequest{Action: "office-ontop", On: &on})
	return err
}

// hostOfficeKey has the Mac app hold the office's keys from anywhere instead, at once ("off": none): the
// setting is the core's to keep, in the config the app reads it from as it starts. Keys macOS has switched on,
// or won't let it hold, and the Mac's own keys on the layout in use are the app's refusal, and the keys held
// before are kept.
func hostOfficeKey(h hostBridge, keys string) error {
	_, err := ask(h, bridgeRequest{Action: "office-key", Keys: keys})
	return err
}

// hostFacts is what setup shows of this Mac: apps are the apps keys may go
// to, left out of the running ones.
func hostFacts(h hostBridge, apps []string) (macFacts, error) {
	answer, err := ask(h, bridgeRequest{Action: "facts", Apps: apps})
	if err == nil && answer.Facts == nil {
		err = errors.New("the Mac app said nothing of this Mac")
	}
	if err != nil {
		return macFacts{}, err
	}
	return *answer.Facts, nil
}

// hostInstallCheck is the checklist's install line, as the Mac app says it: whether it is in Applications, or
// Move to Applications when it isn't (a downloaded or translocated copy). Only the app knows its bundle's path.
func hostInstallCheck(h hostBridge) (check, error) {
	answer, err := ask(h, bridgeRequest{Action: "install-check"})
	if err == nil && (answer.Check == nil || answer.Check.ID != "install") {
		err = errors.New("the Mac app said nothing of where it is")
	}
	if err != nil {
		return check{}, err
	}
	return *answer.Check, nil
}

// hostInstall is that line's fix: the app copied into Applications and opened there, which quits the one asked.
// A refusal says why not (a test copy stays where it was built).
func hostInstall(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "install"})
	return err
}

// hostTimeFormat is how this Mac writes a short time, asked alone (RWTC N7): a webhook delivery's time needs only
// that, not the whole facts (running apps, dictation, the terminal command) on the Mac's main queue.
func hostTimeFormat(h hostBridge) (macTimeFormat, error) {
	answer, err := ask(h, bridgeRequest{Action: "time-format"})
	if err == nil && answer.TimeFormat == nil {
		err = errors.New("the Mac app said nothing of how it writes a time")
	}
	if err != nil {
		return macTimeFormat{}, err
	}
	return *answer.TimeFormat, nil
}

// hostDock shows Smolcorp in the Dock and ⌘-Tab, or in the menu bar only, at once.
func hostDock(h hostBridge, on bool) error {
	_, err := ask(h, bridgeRequest{Action: "dock", On: &on})
	return err
}

// hostAskAccessibility asks macOS for Accessibility, as setup's button does: a system prompt.
func hostAskAccessibility(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "accessibility"})
	return err
}

// hostKeystrokesOn is pressing keys in other apps switched on: macOS is asked
// for Accessibility if Smolcorp hasn't it, but never by a test copy.
func hostKeystrokesOn(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "keystrokes-on"})
	return err
}

// hostKeystrokeApps is list with app added (add) or taken off, as the Mac's
// rules have it; a refusal says why (a terminal, a password app…).
func hostKeystrokeApps(h hostBridge, list []string, app string, add bool) ([]string, error) {
	answer, err := ask(h, bridgeRequest{Action: "keystroke-apps", Apps: list, App: app, On: &add})
	if answer.Apps == nil {
		answer.Apps = []string{}
	}
	return answer.Apps, err
}

// hostGlossary is dictation's names as its clean-up keeps them: one line
// each, no doubles, none too long, not too many.
func hostGlossary(h hostBridge, lines []string) ([]string, error) {
	answer, err := ask(h, bridgeRequest{Action: "glossary", Names: lines})
	if answer.Names == nil {
		answer.Names = []string{}
	}
	return answer.Names, err
}

// hostPrepareDictation asks for the microphone, as setup's switch does, and
// gets language's model ready: why it isn't, or "".
func hostPrepareDictation(h hostBridge, language string) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "dictation", Language: language})
	return answer.Problem, err
}

// hostTerminalCommand makes "Add smolcorp to Terminal"'s link (add) or takes it away.
func hostTerminalCommand(h hostBridge, add bool) error {
	_, err := ask(h, bridgeRequest{Action: "terminal-command", On: &add})
	return err
}

// hostAbilities is each tab's abilities, in order, as the Mac app's drivers
// have them: from their tables, never asking a terminal.
func hostAbilities(h hostBridge, tabs []string) ([]tabAbilities, error) {
	answer, err := ask(h, bridgeRequest{Action: "abilities", Tabs: tabs})
	if err == nil && len(answer.Abilities) != len(tabs) {
		err = fmt.Errorf("the Mac app said %d tabs' abilities for %d", len(answer.Abilities), len(tabs))
	}
	return answer.Abilities, err
}

// hostAgentsEnded is, for each tab in order, whether the Mac app knows its agent has left it (Deck.pruneClosedPanes'
// look): a Terminal tab by ps, iTerm2's and Ghostty's also closed. Read only; unknown is false.
func hostAgentsEnded(h hostBridge, tabs []string, agents []Agent) ([]bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "agents-ended", Tabs: tabs, Agents: agents})
	if err == nil && len(answer.Ended) != len(tabs) {
		err = fmt.Errorf("the Mac app said %d tabs' agents for %d", len(answer.Ended), len(tabs))
	}
	return answer.Ended, err
}

// hostResolveBots is who each bot is in the Grok Bot app now, read afresh, in
// order: nil for one that isn't there.
func hostResolveBots(h hostBridge, bots []bridgeBot) ([]*botResolution, error) {
	answer, err := ask(h, bridgeRequest{Action: "bots-resolve", Bots: bots})
	if err == nil && len(answer.Resolved) != len(bots) {
		err = fmt.Errorf("the Mac app said who %d bots are for %d", len(answer.Resolved), len(bots))
	}
	return answer.Resolved, err
}

// hostProcesses is the processes working in folder (or a folder inside it), or given it with --add-dir, and
// with a session, those whose arguments name it, as the Mac app's own Let go finds them (Processes.holding):
// read from the kernel by PID, nothing run, nothing signalled. An error is the app couldn't look, or wasn't asked.
func hostProcesses(h hostBridge, folder, session string) ([]bridgeProcess, error) {
	answer, err := ask(h, bridgeRequest{Action: "processes", Folder: folder, Session: session})
	if err == nil && len(answer.Processes) > bridgeProcessesMost {
		err = fmt.Errorf("the Mac app named %d processes", len(answer.Processes))
	}
	if err != nil {
		return nil, err
	}
	return answer.Processes, nil
}

// t3MacBundleID is T3 Code's app as macOS knows it (T3Code.bundleID): the "launcher" t3AppLauncher gives on a Mac.
const t3MacBundleID = "com.t3tools.t3code"

// hostT3App says whether T3 Code's app is on the Mac, by its bundle id, as the Mac app's T3Code.Machine looks.
func hostT3App(h hostBridge) (bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "t3-app"})
	return err == nil && answer.Found, err
}

// hostT3OpenApp has the Mac app open T3 Code's app, as the Swift engine's Open in T3 Code does before `t3 app`.
func hostT3OpenApp(h hostBridge) error {
	_, err := ask(h, bridgeRequest{Action: "t3-open-app"})
	return err
}

// hostT3Link has the Mac app open a hire's thread by T3 Code's own link (t3code://threads/…), with its opener:
// it takes no other link.
func hostT3Link(h hostBridge, link string) error {
	_, err := ask(h, bridgeRequest{Action: "t3-link", Link: link})
	return err
}

// hostMusicKey presses one of the jukebox's keys (musicKeys) on the Mac, as its media key does: the app's no
// without Accessibility, in its words.
func hostMusicKey(h hostBridge, key string) error {
	_, err := ask(h, bridgeRequest{Action: "music", Music: key})
	return err
}

// hostMusicState is whether music is playing on the Mac (true, or nil when that can't be told) and its
// output volume (nil where it can't be set), read afresh.
func hostMusicState(h hostBridge) (*bool, *musicVolume, error) {
	answer, err := ask(h, bridgeRequest{Action: "music-state"})
	if err != nil {
		return nil, nil, err
	}
	var volume *musicVolume
	if answer.Volume != nil {
		volume = &musicVolume{level: clampVolume(answer.Volume.Level), muted: answer.Volume.Muted}
	}
	return answer.Playing, volume, nil
}

// hostSetVolume sets the Mac's output volume, 0 to 100, as the jukebox's bar does.
func hostSetVolume(h hostBridge, level int) error {
	_, err := ask(h, bridgeRequest{Action: "volume", Level: &level})
	return err
}

// hostPhoneText has the Mac app type the phone's words into tab, a Terminal, iTerm2 or Ghostty tab, for
// agent, without Return, by the Swift engine's own code (Deck.phoneWordsRefusal, Deck.typePhoneWords: IGL),
// so with its every gate: one line as typed text and several as one paste where the terminal takes one and
// the agent is known to ask for bracketed paste, only while agent is in front there, and, through the
// clipboard, only while the desk waits at its prompt with nothing else reading its tab. The app checks the
// words again: prepared, no "!" first, no control character but a line feed and a tab, 2,000 characters at
// most. On the terminal's queue it asks deskState, where the Swift engine looks at its own sessions. Its no
// comes in the Swift engine's sentence (an appRefusal); one that may have typed is errPasteFailed.
func hostPhoneText(h hostBridge, tab string, agent Agent, words string, deskState func() SessionState) (pasteResult, error) {
	answer, err := ask(h, bridgeRequest{Action: "phone-text", Tab: tab, Agent: agent, Text: words, deskState: deskState})
	if err != nil {
		err = placeError(answer, err)
		// Only the app's own no, an agent not in front and a terminal it can't drive are known to have typed
		// nothing, unless the app says it may have. Any other failure (the connection lost, a deadline, asked
		// too many times) may have come after the words went.
		var refusal appRefusal
		if !answer.MayHaveTyped && (answer.Place != "" || errors.Is(err, errUnsupported) || errors.As(err, &refusal)) {
			return pasteFailed, err
		}
		return pasteFailed, fmt.Errorf("%w (%w)", errPasteFailed, err)
	}
	if answer.Pasted == "pasted" {
		return pasteDone, nil
	}
	return pasteFailed, fmt.Errorf("%w: the Mac app answered %q", errPasteFailed, answer.Pasted)
}

// hostPeek has the Mac app read the end of what tab shows, only while agent is in front there.
func hostPeek(h hostBridge, tab string, agent Agent) ([]string, error) {
	answer, err := ask(h, bridgeRequest{Action: "peek", Tab: tab, Agent: agent})
	if err != nil {
		return nil, placeError(answer, err)
	}
	if answer.Peek == nil {
		answer.Peek = []string{}
	}
	return answer.Peek, nil
}

// placeError is err, or the agent not in front as the deck's own errors say it.
func placeError(answer bridgeAnswer, err error) error {
	switch answer.Place {
	case "gone":
		return fmt.Errorf("%w (%w)", errAgentGone, err)
	case "behind":
		return fmt.Errorf("%w (%w)", errAgentBehind, err)
	}
	return err
}

// macTab is a session's tab when the engine runs under the Mac app. Terminal,
// iTerm2 and Ghostty are driven by the app alone (AppleScript and
// Accessibility). tmux, WezTerm and kitty take their keys from this program,
// as on Linux, with its checks; the app brings their window forward and fires
// them, as the Swift engine does, so the terminal showing a pane comes forward too.
type macTab struct {
	name string
	host hostBridge
	pane *commandTab // tmux, WezTerm or kitty; nil for a tab only the app drives
}

// paneOf is the pane this program drives itself behind tab, if any.
func paneOf(tab Tab) (commandTab, bool) {
	switch t := tab.(type) {
	case commandTab:
		return t, true
	case macTab:
		if t.pane != nil {
			return *t.pane, true
		}
	}
	return commandTab{}, false
}

func (t macTab) do(request bridgeRequest) (bridgeAnswer, error) {
	request.Tab = t.name
	return ask(t.host, request)
}

func (t macTab) Press(k Keystroke) error {
	if t.pane != nil {
		return t.pane.Press(k)
	}
	_, err := t.do(bridgeRequest{Action: "press", Key: k.Key, Times: max(k.Times, 1)})
	return err
}

func (t macTab) Type(text string) error {
	if t.pane != nil {
		return t.pane.Type(text)
	}
	_, err := t.do(bridgeRequest{Action: "type", Text: text})
	return err
}

func (t macTab) Focus() error {
	_, err := t.do(bridgeRequest{Action: "focus"})
	return err
}

func (t macTab) Scroll(lines int) error {
	if t.pane != nil {
		return t.pane.Scroll(lines)
	}
	_, err := t.do(bridgeRequest{Action: "scroll", Lines: lines})
	return err
}

func (t macTab) ScrollToBottom() error {
	if t.pane != nil {
		return t.pane.ScrollToBottom()
	}
	_, err := t.do(bridgeRequest{Action: "scroll-bottom"})
	return err
}

// errTabClosed is the Mac app's close finding no tab there, or (strict) no agent in front of it.
var errTabClosed = errors.New("the pane has closed")

// Close fires the agent as the Mac app does: errNotStopped when its tab is
// left open, as commandTab.Close says it.
func (t macTab) Close(agent Agent) error { return t.close(agent, false) }

// CloseStrict is Close only with agent in front of its tab: a tab with only its shell left is never closed, as it
// may be another's on a tty used again (errTabClosed). For a tab known only by its folder (hirefire_window.go).
func (t macTab) CloseStrict(agent Agent) error { return t.close(agent, true) }

func (t macTab) close(agent Agent, strict bool) error {
	answer, err := t.do(bridgeRequest{Action: "close", Agent: agent, Strict: strict})
	if err != nil {
		return err
	}
	switch answer.Fired {
	case "fired":
		return nil
	case "gone":
		return errTabClosed
	case "not-stopped":
		return errNotStopped
	}
	return fmt.Errorf("the Mac app answered %q", answer.Fired)
}

// hostTab is the tab the Mac app drives for name, or nil without one.
func hostTab(name, kind, id, socket string) Tab {
	if host == nil {
		return nil
	}
	tab := macTab{name: name, host: host}
	if kind == "tmux" || kind == "wezterm" || kind == "kitty" {
		tab.pane = &commandTab{kind: kind, id: id, socket: socket}
	}
	return tab
}
