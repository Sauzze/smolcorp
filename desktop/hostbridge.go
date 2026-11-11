package desktop

import (
	"errors"
	"fmt"
)

// On a Mac, this program can run as Smolcorp.app's engine (`smolcorp core`,
// core_darwin.go): the app starts it, shows its pages in the app's windows,
// and keeps what only macOS lets an app do. The engine asks the app for those
// over the host bridge, a socket in the support folder that only the two of
// them use (each checks the other's pid; see socketBridge): keys typed
// into a Terminal, iTerm2 or Ghostty tab, the phone's text and a look at what
// the tab shows (phonetext.go), a window brought forward or closed,
// a terminal opened on a script, the app's own Keychain items, its login item;
// setup's Mac parts (hostsetup.go) and what each desk's terminal allows
// (hostoffice.go). Each is the Mac app's own code, as the Swift engine runs it
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
	Terminal string `json:"terminal,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Script   string `json:"script,omitempty"`
	// secret-*: one of the app's own items (see bridgeSecrets).
	Item  string `json:"item,omitempty"`
	Value string `json:"value,omitempty"`
	// login-item, dock: on or off; keystroke-apps: App added or removed; terminal-command: made or removed.
	On *bool `json:"on,omitempty"`
	// facts: the apps keys may go to, left out of those running; keystroke-apps: the list App goes on or off.
	Apps []string `json:"apps,omitempty"`
	App  string   `json:"app,omitempty"`
	// glossary: dictation's names, as the page has them.
	Names []string `json:"names,omitempty"`
	// dictation: the language whose model gets ready.
	Language string `json:"language,omitempty"`
	// abilities: the sessions' tabs, as the deck names them.
	Tabs []string `json:"tabs,omitempty"`
	// bots-resolve: the Grok Bots to look for in the Grok Bot app.
	Bots []bridgeBot `json:"bots,omitempty"`
	// phone-text: the desk's state now, as this engine's sessions have it ("" for one gone). The Mac app asks
	// it on this request's own connection, from the terminal's queue, where the Swift engine looks at its own
	// sessions (Deck.typePhoneWords); the transport answers it (socketBridge). Never sent itself; nil is asking.
	deskState func() SessionState
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
	// close: fired, gone or not-stopped, as the Mac's Terminal.Fired.
	Fired string `json:"fired,omitempty"`
	// secret-read: the value, and whether there is one.
	Value string `json:"value,omitempty"`
	Found bool   `json:"found,omitempty"`
	// login-item: whether Smolcorp opens at login.
	On bool `json:"on,omitempty"`
	// facts: what only the Mac can say, for setup.
	Facts *macFacts `json:"facts,omitempty"`
	// keystroke-apps: the list, with the app on it or off it; glossary: the names, checked.
	Apps  []string `json:"apps,omitempty"`
	Names []string `json:"names,omitempty"`
	// dictation: why the language's model isn't ready, or nothing.
	Problem string `json:"problem,omitempty"`
	// abilities: each tab's, in the order asked.
	Abilities []tabAbilities `json:"abilities,omitempty"`
	// bots-resolve: who each bot is in the Grok Bot app, in the order asked; null for one not there.
	Resolved []*botResolution `json:"resolved,omitempty"`
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
}

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

// hostWriteSecret keeps value as the item, or removes it when value is empty.
func hostWriteSecret(h hostBridge, item, value string) error {
	action := "secret-write"
	if value == "" {
		action = "secret-delete"
	}
	_, err := ask(h, bridgeRequest{Action: action, Item: item, Value: value})
	return err
}

// hostLoginItem says whether Smolcorp opens at login; with on, it sets it first.
func hostLoginItem(h hostBridge, on *bool) (bool, error) {
	answer, err := ask(h, bridgeRequest{Action: "login-item", On: on})
	return answer.On, err
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

// hostResolveBots is who each bot is in the Grok Bot app now, read afresh, in
// order: nil for one that isn't there.
func hostResolveBots(h hostBridge, bots []bridgeBot) ([]*botResolution, error) {
	answer, err := ask(h, bridgeRequest{Action: "bots-resolve", Bots: bots})
	if err == nil && len(answer.Resolved) != len(bots) {
		err = fmt.Errorf("the Mac app said who %d bots are for %d", len(answer.Resolved), len(bots))
	}
	return answer.Resolved, err
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

// Close fires the agent as the Mac app does: errNotStopped when its tab is
// left open, as commandTab.Close says it.
func (t macTab) Close(agent Agent) error {
	answer, err := t.do(bridgeRequest{Action: "close", Agent: agent})
	if err != nil {
		return err
	}
	switch answer.Fired {
	case "fired":
		return nil
	case "gone":
		return errors.New("the pane has closed")
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
