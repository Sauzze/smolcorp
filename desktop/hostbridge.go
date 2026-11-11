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
// into a Terminal, iTerm2 or Ghostty tab, a window brought forward or closed,
// a terminal opened on a script, the app's own Keychain items, its login item.
// Each is the Mac app's own code, as the Swift engine runs it (HostBridge.swift).
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
	On    *bool  `json:"on,omitempty"`
}

// bridgeAnswer is the Mac app's answer.
type bridgeAnswer struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// The terminal can't do that at all (no driver for it): errUnsupported, not a tab that's gone.
	Unsupported bool `json:"unsupported,omitempty"`
	// open: the new window's tab, as the deck will know it.
	Tab string `json:"tab,omitempty"`
	// close: fired, gone or not-stopped, as the Mac's Terminal.Fired.
	Fired string `json:"fired,omitempty"`
	// secret-read: the value, and whether there is one.
	Value string `json:"value,omitempty"`
	Found bool   `json:"found,omitempty"`
	// login-item: whether Smolcorp opens at login.
	On bool `json:"on,omitempty"`
}

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
		return answer, errors.New(answer.Error)
	}
	return answer, nil
}

// hostOpenTerminal opens a window of terminal in folder running script, a
// file the engine wrote in the hires folder, and says its tab.
func hostOpenTerminal(h hostBridge, terminal, folder, script string) (string, error) {
	answer, err := ask(h, bridgeRequest{Action: "open", Terminal: terminal, Folder: folder, Script: script})
	return answer.Tab, err
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
