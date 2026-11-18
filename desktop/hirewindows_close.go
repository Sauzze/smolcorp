//go:build !windows

package desktop

import (
	"slices"
	"strconv"
	"strings"
)

// windowCloser looks at and closes a window this app opened for a hire, by
// the record its terminal gave (hirewindows.go): the Mac app's terminals,
// through the bridge (bridgeCloser), as the Mac's HireWindows.Closer.
type windowCloser interface {
	// look says "open" and the tty it shows, "gone", or "unknown" when it couldn't be asked.
	look(window string) (shown, tty string)
	// close closes it once nothing but its shell runs on tty: closed, or why it was left open.
	close(window, tty string) (closed bool, left string)
}

// bridgeCloser is the Mac app, closing only windows it opened for this engine.
type bridgeCloser struct{ h hostBridge }

func (b bridgeCloser) look(window string) (string, string) { return hostLookWindow(b.h, window) }
func (b bridgeCloser) close(window, tty string) (bool, string) {
	return hostCloseWindow(b.h, window, tty)
}

// hiresToBackgroundClosing is switching to the background under the Mac app,
// as its HireWindows.toBackground: each hire's viewers detached, on its own
// socket and session; then the windows recorded for it closed, those that
// were among its viewers. Which tty each recorded window shows is asked before
// the viewers go. Never the session nor the server, never by a name, never a
// window this app didn't open. A window closed or gone is forgotten. It says a
// line for each hire that has something to say, in the Mac's words. run is
// the hire's tmux (nil: the real one).
func hiresToBackgroundClosing(hires []Hire, closer windowCloser, run tmuxRunner) []string {
	said := []string{}
	for _, hire := range hires {
		if hire.State == "fired" || hire.State == "failed" {
			continue
		}
		name := hire.Nickname
		if name == "" {
			name = hire.Name
		}
		if hire.TmuxSocket == "" {
			said = append(said, name+" runs in its window without tmux, so it stays there.")
			continue
		}
		server := hireOwnTmux(hire, testCopyPort(), run)
		if server == nil {
			said = append(said, name+"'s tmux couldn't be reached, so its window is left.")
			continue
		}
		viewers, asked := server.clientTTYs()
		if !asked {
			said = append(said, name+"'s tmux couldn't be asked who's watching it, so its window is left.")
			continue
		}
		type shownWindow struct{ window, tty string }
		var shown []shownWindow
		var unasked, gone []string
		for _, window := range windowsOfHire(hire.ID) {
			switch state, tty := closer.look(window); state {
			case "open":
				shown = append(shown, shownWindow{window, tty})
			case "gone":
				gone = append(gone, window)
			default:
				unasked = append(unasked, window)
			}
		}
		forgetHireWindows(hire.ID, gone)
		if len(viewers) == 0 {
			continue
		}
		if !server.detachViewers() {
			said = append(said, name+"'s window couldn't be detached, so it's left.")
			continue
		}
		var lines, closed []string
		for _, s := range shown {
			if !slices.Contains(viewers, s.tty) {
				lines = append(lines, "a window Smolcorp opened for it shows something else now, so it's left")
				continue
			}
			if ok, why := closer.close(s.window, s.tty); ok {
				closed = append(closed, s.window)
			} else {
				lines = append(lines, "its window is left open: "+why)
			}
		}
		forgetHireWindows(hire.ID, closed)
		apps := []string{}
		for _, window := range unasked {
			if app := terminalOfRecord(window); !slices.Contains(apps, app) {
				apps = append(apps, app)
			}
		}
		slices.Sort(apps)
		for _, app := range apps {
			lines = append(lines, "Smolcorp couldn't ask "+app+" about the window it opened for it, so it's left")
		}
		// A window Smolcorp couldn't ask about may be one of these, so it isn't said to be someone else's.
		others := -len(unasked)
		for _, tty := range viewers {
			if !slices.ContainsFunc(shown, func(s shownWindow) bool { return s.tty == tty }) {
				others++
			}
		}
		switch {
		case others == 1:
			lines = append(lines, "its window wasn't opened by Smolcorp while it ran, so it's left open")
		case others > 1:
			lines = append(lines, strconv.Itoa(others)+" windows weren't opened by Smolcorp while it ran, so they're left open")
		}
		if len(lines) > 0 {
			said = append(said, name+": detached; "+strings.Join(lines, "; ")+".")
		}
	}
	return said
}

// terminalOfRecord is the terminal a recorded window is in, as it's said (the Mac's HireWindows.terminalName).
func terminalOfRecord(window string) string {
	switch {
	case strings.HasPrefix(window, "iterm:"):
		return "iTerm2"
	case strings.HasPrefix(window, "ghostty-id:"):
		return "Ghostty"
	}
	return "Terminal"
}
