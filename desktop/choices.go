package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"
)

// The two plain choices of first setup and Settings (T796), as the Mac app's
// Choices.swift: T3 Code on or off, and agent windows. Here, running hires in
// the background is tmux with no window, so only where there's tmux (Linux, and
// a Mac running this app); on Windows the choice is greyed, as there's no tmux.

// t3CodeAllowed is the T3 Code gate: everything of T3 Code's (a button, a
// hint, looking for it, running t3) asks this first, and does nothing when it's
// false.
func t3CodeAllowed(c *Config) bool { return c.T3CodeEnabled }

// t3Off is what T3 Code's routes say while it's off: they answer nothing
// else, and t3 is never run.
const t3Off = "T3 Code is off: turn it on in Settings."

// t3On is the gate as a route asks it, under the deck's lock.
func (d *Deck) t3On() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return t3CodeAllowed(d.config)
}

// t3LookFolders are where a t3 command may be, in the order T3 Code's own CLI
// install looks, then the PATH's folders.
func t3LookFolders(home, path string) []string {
	folders := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin"), filepath.Join(home, ".t3", "bin")}
	if runtime.GOOS != "windows" {
		folders = append(folders, "/opt/homebrew/bin", "/usr/local/bin")
	}
	for _, folder := range filepath.SplitList(path) {
		if filepath.IsAbs(folder) && !slices.Contains(folders, folder) {
			folders = append(folders, folder)
		}
	}
	return folders
}

// t3Names are the t3 command's file names here.
func t3Names() []string {
	if runtime.GOOS == "windows" {
		return []string{"t3.exe", "t3.cmd", "t3.bat"}
	}
	return []string{"t3"}
}

// t3Installed says whether a t3 command is in one of t3LookFolders. Existence
// only: it's never run. On Linux and Windows the desktop app has no id to look
// for; its installers put t3 on the PATH. On a Mac, t3Look also asks the Mac
// app for the app by its bundle id (t3MacAppFound).
func t3Installed(home, path string, isFile func(string) bool) bool {
	for _, folder := range t3LookFolders(home, path) {
		for _, name := range t3Names() {
			if isFile(filepath.Join(folder, name)) {
				return true
			}
		}
	}
	return false
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

var (
	t3LookMu    sync.Mutex
	t3LookAt    time.Time
	t3LookFound bool
	// t3Look is the look itself: a variable, for the tests.
	t3Look = func() bool {
		home, _ := os.UserHomeDir()
		return t3MacAppFound() || t3Installed(home, os.Getenv("PATH"), isRegularFile)
	}
)

// t3MacAppFound is whether T3 Code's app is on the Mac, as the Mac app's Choices.t3Installed looks for it (by
// its bundle id, through the bridge): false without the Mac app (C4 A1).
func t3MacAppFound() bool {
	if host == nil {
		return false
	}
	found, _ := hostT3App(host)
	return found
}

// t3InstalledRecently is the last look, for 30 s: setup asks for its state often.
func t3InstalledRecently() bool {
	t3LookMu.Lock()
	defer t3LookMu.Unlock()
	if !t3LookAt.IsZero() && time.Since(t3LookAt) < 30*time.Second {
		return t3LookFound
	}
	t3LookFound, t3LookAt = t3Look(), time.Now()
	return t3LookFound
}

// backgroundHires says whether hires run in the background now: chosen, and tmux here.
func backgroundHires(c *Config, hasTmux bool) bool {
	// A test copy's hires run in tmux with no window, never in one of the owner's terminal app (RTCH N1), as the Mac's.
	return (c.HiresInBackground || testCopyPort() != 0) && hasTmux
}

// choicesState is what setup shows of the two. Whether T3 Code is installed is
// looked for only in first setup, to know whether to ask, and with T3 Code on.
func choicesState(c *Config, hasTmux bool, installed func() bool) map[string]any {
	state := map[string]any{"t3Code": c.T3CodeEnabled, "background": backgroundHires(c, hasTmux), "hasTmux": hasTmux, "asked": c.ChoicesAsked}
	if !c.ChoicesAsked || c.T3CodeEnabled {
		state["t3Installed"] = installed()
	}
	return state
}

// backgroundNeedsTmux is the refusal of background hires with no tmux, in the Mac's words under the Mac app (macWords).
const backgroundNeedsTmux = "Running hires in the background needs tmux, which isn't on this computer."

// setChoices applies POST /setup/choices: t3, background and asked, each 0 or
// 1. Whether it switched hires to the background, or a refusal.
func setChoices(c *Config, query url.Values, hasTmux bool) (switched bool, status int, refusal string) {
	if query.Get("background") == "1" && !hasTmux {
		// In the Mac's words under the Mac app (macWords), as the Swift engine's DeckRouter said it.
		return false, http.StatusBadRequest, inMacWords(backgroundNeedsTmux)
	}
	was := backgroundHires(c, hasTmux)
	if query.Has("t3") {
		c.T3CodeEnabled = query.Get("t3") == "1"
	}
	if query.Has("background") {
		c.HiresInBackground = query.Get("background") == "1"
	}
	if query.Get("asked") == "1" {
		c.ChoicesAsked = true
	}
	return !was && backgroundHires(c, hasTmux), 0, ""
}

// detached is what detaching one hire's viewers came to.
type detached struct {
	own     bool // on a tmux server of its own (hireOwnTmux)
	asked   bool // its tmux said how many windows were attached
	viewers int  // how many were
	ok      bool // they're detached now, or there were none
}

// hiresToBackground detaches the viewers of each hire on its own tmux
// server, on its own socket and session: never the session or the server.
// A window this app opened for a hire is that attach itself (hire.go,
// hirecleanup.go), so it ends as it's detached; any other is left. A plain
// hire, not on a tmux server of its own, is left in its window. Each is said.
func hiresToBackground(hires []Hire, detach func(Hire) detached) []string {
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
			said = append(said, name+" isn't on a tmux server of its own, so its window is left.")
			continue
		}
		got := detach(hire)
		switch {
		case !got.own:
			said = append(said, name+" isn't on a tmux server of its own, so its window is left.")
		case !got.asked:
			said = append(said, name+"'s tmux couldn't be asked who's watching it, so its window is left.")
		case !got.ok:
			said = append(said, name+"'s window couldn't be detached, so it's left.")
		case got.viewers > 0:
			said = append(said, name+": detached. A window this computer opened for it closes with it; any other is left open.")
		}
	}
	return said
}
