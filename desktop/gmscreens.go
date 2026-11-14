package desktop

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"time"
)

// The GM's screens and tiling, under the Mac app, as the Swift engine had them (Tiler.swift, DeckRouter.swift):
// GET /setup/gm/screens, POST /setup/gm/screen?id= and POST /setup/gm/tile. Tiling puts the hires' windows where
// their desks are: the ground floor's 3×3 desks become a 3×3 grid on one screen, each hire's window over its
// desk's cell, with the office's window under them. The Mac app reads its screens and moves the windows, with
// the terminal's own AppleScript (Terminal and iTerm2: the "Automation" permission, never Accessibility), over
// the bridge; the rest is here. A terminal without window scripting (Ghostty) gets one window showing a tmux
// "team" grid instead, a pane to a hire, each a read-only attach (gmtile_unix.go).
//
// Coordinates are AppleScript's: from the main screen's top-left corner, y going down. The Mac's screens come
// as NSScreen gives them, y up from the main screen's bottom, so they're turned over (flipped). The screens are
// asked for at every call and nothing is kept but the screen you chose (config.json's teamScreen).
//
// Linux and Windows have no tiling: the routes stay 404 there.

// tileRect is a rectangle: in NSScreen's coordinates as the Mac app sends a screen, in AppleScript's once flipped.
type tileRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// macScreen is one of the Mac's screens, as the app reads it (Tiler.Screen): what the display is called to the
// system (stable while it stays plugged in), its name, its frame and the part of it free of the menu bar and the
// Dock, and whether it's the main one.
type macScreen struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Frame   tileRect `json:"frame"`
	Visible tileRect `json:"visible"`
	Main    bool     `json:"main"`
}

func (s macScreen) portrait() bool { return s.Frame.Height > s.Frame.Width }

// tilePlace is a window to move: the hire's tmux client's tty, which the Mac app finds its window by, and where it goes.
type tilePlace struct {
	TTY  string   `json:"tty"`
	Rect tileRect `json:"rect"`
}

// flipped is r from NSScreen's coordinates into AppleScript's (and back: it is its own inverse).
func flipped(r tileRect, mainHeight float64) tileRect {
	return tileRect{X: r.X, Y: mainHeight - (r.Y + r.Height), Width: r.Width, Height: r.Height}
}

// screensMainHeight is the main screen's height, which the coordinates turn over by.
func screensMainHeight(screens []macScreen) float64 {
	for _, s := range screens {
		if s.Main {
			return s.Frame.Height
		}
	}
	if len(screens) > 0 {
		return screens[0].Frame.Height
	}
	return 0
}

// teamScreen is the screen that holds the team: the one chosen, if it's still plugged in, else the left-most
// portrait one, else the main one.
func teamScreen(screens []macScreen, chosen string) (macScreen, bool) {
	if chosen != "" {
		if i := slices.IndexFunc(screens, func(s macScreen) bool { return s.ID == chosen }); i >= 0 {
			return screens[i], true
		}
	}
	found := -1
	for i, s := range screens {
		if s.portrait() && (found < 0 || s.Frame.X < screens[found].Frame.X) {
			found = i
		}
	}
	if found < 0 {
		found = slices.IndexFunc(screens, func(s macScreen) bool { return s.Main })
	}
	if found < 0 && len(screens) > 0 {
		found = 0
	}
	if found < 0 {
		return macScreen{}, false
	}
	return screens[found], true
}

// tileGrid is the nine cells, by desk key, top-left first, and the office's place.
type tileGrid struct {
	cells  [9]tileRect
	office tileRect
}

// gridOn is the grid on screen's visible part: the office the full width at 16:9, flush with the bottom; the
// three rows share what's above it; the columns a third of the width each. Desk key → cell: column = key % 3,
// row = key / 3.
func gridOn(screen macScreen, mainHeight float64) tileGrid {
	area := flipped(screen.Visible, mainHeight)
	officeHeight := math.Round(area.Width * 9 / 16)
	width := math.Floor(area.Width / 3)
	height := math.Floor((area.Height - officeHeight) / 3)
	var g tileGrid
	for key := range g.cells {
		g.cells[key] = tileRect{X: area.X + float64(key%3)*width, Y: area.Y + float64(key/3)*height, Width: width, Height: height}
	}
	g.office = tileRect{X: area.X, Y: area.Y + area.Height - officeHeight, Width: area.Width, Height: officeHeight}
	return g
}

// screensAnswer is GET /setup/gm/screens: the screens, which one holds the team now, and whether to ask (two or more).
func screensAnswer(screens []macScreen, chosen string) map[string]any {
	current, found := teamScreen(screens, chosen)
	rows := []map[string]any{}
	for _, s := range screens {
		rows = append(rows, map[string]any{"id": s.ID, "name": s.Name, "main": s.Main, "portrait": s.portrait(),
			"team":  found && s.ID == current.ID,
			"frame": map[string]any{"x": s.Frame.X, "y": s.Frame.Y, "width": s.Frame.Width, "height": s.Frame.Height}})
	}
	id := ""
	if found {
		id = current.ID
	}
	return map[string]any{"screens": rows, "chosen": id, "ask": len(screens) > 1}
}

// tileSeat is a hire with a window to place: its id, its desk on the ground floor and its tmux socket.
type tileSeat struct {
	hireID string
	desk   int
	socket string
}

// tileSeats are the ground floor's hires among the team's rows (GET /setup/gm/team): those with a tmux session of
// their own and a desk, one to a desk (the first row keeps it), by desk.
func tileSeats(rows []map[string]any) []tileSeat {
	var seats []tileSeat
	taken := map[int]bool{}
	for _, row := range rows {
		id, _ := row["id"].(string)
		socket, _ := row["tmuxSocket"].(string)
		state, _ := row["state"].(string)
		floor, onFloor := row["floor"].(int)
		desk, atDesk := row["desk"].(int)
		if row["registered"] != true || id == "" || socket == "" || !onFloor || floor != 0 || !atDesk || desk < 0 || desk > 8 ||
			taken[desk] || state == "fired" || state == "failed" {
			continue
		}
		taken[desk] = true
		seats = append(seats, tileSeat{hireID: id, desk: desk, socket: socket})
	}
	slices.SortFunc(seats, func(a, b tileSeat) int { return a.desk - b.desk })
	return seats
}

// scriptableTerminal is the scripting name of a terminal that can place its windows, or "".
func scriptableTerminal(terminal string) string {
	switch terminal {
	case "terminal":
		return "Terminal"
	case "iterm":
		return "iTerm"
	}
	return ""
}

// tileReport is how the windows were placed: "windows" (each hire's own, moved with its terminal's scripting),
// "team-grid" (one window showing a tmux grid of all the hires) or "none".
type tileReport struct {
	Method          string   `json:"method"`
	Placed          []string `json:"placed"`
	Missed          []string `json:"missed"`
	NeedsPermission bool     `json:"needsPermission"`
	Message         string   `json:"message"`
	// Where the office goes, in AppleScript's coordinates, as whole numbers: x, y, width, height.
	Office []int `json:"office,omitempty"`
}

func tileNotAllowed(app string) string {
	return app + " didn't let Smolcorp move its windows. Allow Smolcorp under " + app + " in System Settings → Privacy & Security → Automation, then tile again."
}

// tileWait is how long a tile may take: each window moved with an AppleScript of its own, and the first one may
// wait for you to answer macOS's Automation question. The app gives up first (HostBridge.tileWait).
var tileWait = 100 * time.Second

// tileWith is the tiling: seats on grid as terminal allows, the tmux client's tty of a hire's socket asked of
// clientTTY, the windows moved by the Mac app, and the team grid shown by teamGrid for a terminal that can't
// place windows. A refused permission ends the work at once.
func tileWith(h hostBridge, seats []tileSeat, terminal string, grid tileGrid, clientTTY func(socket string) string,
	teamGrid func(sockets []string) bool) tileReport {
	report := tileReport{Method: "none", Placed: []string{}, Missed: []string{},
		Office: []int{int(grid.office.X), int(grid.office.Y), int(grid.office.Width), int(grid.office.Height)}}
	if len(seats) == 0 {
		report.Message = "No hire is on the ground floor in its own tmux session, so there is nothing to tile."
		return report
	}
	app := scriptableTerminal(terminal)
	if app == "" {
		sockets := make([]string, len(seats))
		for i, seat := range seats {
			sockets[i] = seat.socket
		}
		if !teamGrid(sockets) {
			report.Message = "The team grid couldn't be made: tmux wasn't found or wouldn't start it."
			return report
		}
		report.Method = "team-grid"
		for _, seat := range seats {
			report.Placed = append(report.Placed, seat.hireID)
		}
		report.Message = "This terminal can't place its windows, so the team is in one window, a pane to a hire, read-only."
		return report
	}
	report.Method = "windows"
	var places []tilePlace
	byTTY := map[string]string{}
	for _, seat := range seats {
		tty := clientTTY(seat.socket)
		if tty == "" || byTTY[tty] != "" {
			report.Missed = append(report.Missed, seat.hireID)
			continue
		}
		byTTY[tty] = seat.hireID
		places = append(places, tilePlace{TTY: tty, Rect: grid.cells[seat.desk]})
	}
	if len(places) > 0 {
		placed, needsPermission, err := hostTile(h, terminal, places)
		var refusal appRefusal
		switch {
		case err != nil:
			report.Placed, report.Missed = []string{}, hireIDs(seats)
			report.Message = app + " couldn't be asked for its windows."
			if errors.As(err, &refusal) {
				report.Message = refusal.Error()
			}
			return report
		case needsPermission:
			report.NeedsPermission = true
			report.Message = tileNotAllowed(app)
		}
		for _, place := range places {
			if slices.Contains(placed, place.TTY) {
				report.Placed = append(report.Placed, byTTY[place.TTY])
			} else {
				report.Missed = append(report.Missed, byTTY[place.TTY])
			}
		}
	}
	// By desk, as the seats are.
	report.Placed = slices.DeleteFunc(hireIDs(seats), func(id string) bool { return !slices.Contains(report.Placed, id) })
	report.Missed = slices.DeleteFunc(hireIDs(seats), func(id string) bool { return slices.Contains(report.Placed, id) })
	if report.Message == "" {
		if len(report.Missed) == 0 {
			report.Message = "The team is tiled."
		} else {
			report.Message = fmt.Sprintf("%d placed; %d not (no open window found for it, or %s put it elsewhere: it can do that on a screen other than the main one).",
				len(report.Placed), len(report.Missed), app)
		}
	}
	return report
}

func hireIDs(seats []tileSeat) []string {
	ids := make([]string, len(seats))
	for i, seat := range seats {
		ids[i] = seat.hireID
	}
	return ids
}

// hostScreens asks the Mac app for its screens, read afresh.
func hostScreens(h hostBridge) ([]macScreen, error) {
	answer, err := ask(h, bridgeRequest{Action: "screens"})
	return answer.Screens, err
}

// hostTile asks the Mac app to move terminal's windows to places, each found by the tty it shows: the ttys whose
// windows went there, and whether the terminal refused Smolcorp its Automation permission.
func hostTile(h hostBridge, terminal string, places []tilePlace) (placed []string, needsPermission bool, err error) {
	answer, err := ask(h, bridgeRequest{Action: "tile", Terminal: terminal, Places: places, wait: tileWait})
	return answer.Placed, answer.NeedsPermission, err
}

// hostPlaceOffice asks the Mac app to put the office's window at rect, in AppleScript's coordinates turned over
// by mainHeight, while it's shown.
func hostPlaceOffice(h hostBridge, rect tileRect, mainHeight float64) error {
	_, err := ask(h, bridgeRequest{Action: "office-place", Office: &rect, MainHeight: mainHeight})
	return err
}

// gmScreenRoutes answers the GM's screens and tile under the Mac app, and says whether the request was one of
// them. The route table and the manager's switch have had their say by here.
func (rt *router) gmScreenRoutes(w http.ResponseWriter, r *request) bool {
	d := rt.deck
	path := r.URL.Path
	if !(r.Method == http.MethodGet && path == "/setup/gm/screens" ||
		r.Method == http.MethodPost && (path == "/setup/gm/screen" || path == "/setup/gm/tile")) {
		return false
	}
	h := d.macHost()
	if h == nil {
		return false
	}
	id := r.URL.Query().Get("id")
	if path == "/setup/gm/screen" && id == "" {
		writeStatus(w)(http.StatusBadRequest, "Say which screen: id=<id> from /setup/gm/screens.")
		return true
	}
	// A test copy never moves another app's windows: told before the Mac is asked for anything (the Swift engine's
	// TestCopy.routeRefusal), in testcopyapps.go's words. Which screen to use and the screens are the copy's own to
	// keep and read.
	if path == "/setup/gm/tile" && refusedInTestCopy(w, testCopyKeepsWindows) {
		return true
	}
	screens, err := hostScreens(h)
	if err != nil {
		bridgeFailed(w, err)
		return true
	}
	d.mu.Lock()
	chosen, terminal := d.config.TeamScreen, d.config.HireTerminal
	d.mu.Unlock()
	switch path {
	case "/setup/gm/screens":
		writeJSON(w, http.StatusOK, screensAnswer(screens, chosen))
	case "/setup/gm/screen":
		if !slices.ContainsFunc(screens, func(s macScreen) bool { return s.ID == id }) {
			writeStatus(w)(http.StatusConflict, "No screen has that id; read /setup/gm/screens.")
			return true
		}
		d.mu.Lock()
		d.config.TeamScreen = id
		d.config.save()
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, screensAnswer(screens, id))
	default:
		screen, found := teamScreen(screens, chosen)
		if !found {
			writeStatus(w)(http.StatusConflict, "No screen was found to tile on.")
			return true
		}
		mainHeight := screensMainHeight(screens)
		grid := gridOn(screen, mainHeight)
		rows := buildTeam(readRegistry(registryPath(), false).hires, d.frontDeskLive(), 0, func(string) string { return "" })
		testPort := testCopyPort()
		report := tileWith(h, tileSeats(rows), terminal, grid, func(socket string) string { return hireClientTTY(socket, testPort) },
			func(sockets []string) bool { return showTeamGrid(h, sockets, terminal, testPort) })
		if len(report.Placed) > 0 {
			_ = hostPlaceOffice(h, grid.office, mainHeight)
		}
		writeJSON(w, http.StatusOK, report)
	}
	return true
}
