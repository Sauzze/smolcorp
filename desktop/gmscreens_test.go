package desktop

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The Mac's screens of the Swift engine's TilerTests: a portrait one, the built-in, and a tall one to its left.
var (
	portraitScreen = macScreen{ID: "9", Name: "PA278CV", Frame: tileRect{0, 0, 1440, 2560}, Visible: tileRect{0, 0, 1440, 2530}, Main: true}
	builtIn        = macScreen{ID: "1", Name: "Built-in", Frame: tileRect{0, 0, 1512, 982}, Visible: tileRect{0, 0, 1512, 950}, Main: true}
	tallLeft       = macScreen{ID: "2", Name: "PA278CV", Frame: tileRect{-1440, 0, 1440, 2560}, Visible: tileRect{-1440, 0, 1440, 2530}}
)

func TestTheChosenScreenHoldsTheTeamElseTheLeftPortraitElseTheMain(t *testing.T) {
	for _, c := range []struct {
		chosen, want string
	}{{"1", "1"}, {"", "2"}, {"9", "2"}} {
		if got, _ := teamScreen([]macScreen{builtIn, tallLeft}, c.chosen); got.ID != c.want {
			t.Errorf("chosen %q: %q, want %q", c.chosen, got.ID, c.want)
		}
	}
	a := macScreen{ID: "a", Frame: tileRect{0, 0, 1920, 1080}, Visible: tileRect{0, 0, 1920, 1050}, Main: true}
	b := macScreen{ID: "b", Frame: tileRect{-1920, 0, 1920, 1080}, Visible: tileRect{-1920, 0, 1920, 1080}}
	if got, _ := teamScreen([]macScreen{b, a}, ""); got.ID != "a" {
		t.Errorf("no portrait screen: %q, want the main one", got.ID)
	}
	if _, found := teamScreen(nil, ""); found {
		t.Error("a screen among none")
	}
}

func TestTheScreensAnswerAsksOnlyWithTwoOrMore(t *testing.T) {
	one := screensAnswer([]macScreen{builtIn}, "")
	if one["ask"] != false || one["chosen"] != "1" {
		t.Errorf("one screen: %v", one)
	}
	two := screensAnswer([]macScreen{builtIn, tallLeft}, "1")
	rows, _ := two["screens"].([]map[string]any)
	if two["ask"] != true || two["chosen"] != "1" || len(rows) != 2 || rows[0]["team"] != true || rows[1]["team"] != false ||
		rows[1]["portrait"] != true || rows[1]["frame"].(map[string]any)["width"] != 1440.0 {
		t.Errorf("two screens: %v", two)
	}
}

// The ground floor's 3×3 desks over the office, on the visible part, in AppleScript's coordinates.
func TestTheGroundFloorIsThreeByThreeOverTheOffice(t *testing.T) {
	grid := gridOn(portraitScreen, 2560)
	var xs, ys []float64
	for _, cell := range grid.cells {
		xs, ys = append(xs, cell.X), append(ys, cell.Y)
		if cell.Width != 480 || cell.Height != 573 {
			t.Errorf("a cell is %v", cell)
		}
	}
	if !slices.Equal(xs, []float64{0, 480, 960, 0, 480, 960, 0, 480, 960}) || !slices.Equal(ys, []float64{30, 30, 30, 603, 603, 603, 1176, 1176, 1176}) {
		t.Errorf("cells at %v, %v", xs, ys)
	}
	if grid.office != (tileRect{0, 1750, 1440, 810}) {
		t.Errorf("the office at %v, want flush with the bottom, 16:9", grid.office)
	}
	docked := portraitScreen
	docked.Visible = tileRect{0, 100, 1440, 2430}
	if g := gridOn(docked, 2560); g.office.Y+g.office.Height != 2460 || g.cells[0].Y != 30 {
		t.Errorf("with the Dock at the bottom: office %v, first cell %v", g.office, g.cells[0])
	}
	// A screen to the left of a main one that's wider than tall: tops above the main screen's.
	main := macScreen{ID: "m", Frame: tileRect{0, 0, 2560, 1440}, Visible: tileRect{0, 0, 2560, 1410}, Main: true}
	tall := macScreen{ID: "t", Frame: tileRect{-2952, 0, 1440, 2560}, Visible: tileRect{-2952, 0, 1440, 2530}}
	screens := []macScreen{main, tall}
	if g := gridOn(tall, screensMainHeight(screens)); g.cells[0].X != -2952 || g.cells[0].Y != -1090 || flipped(flipped(g.office, 1440), 1440) != g.office {
		t.Errorf("above the main screen: %v", g.cells[0])
	}
}

func tileRow(id string, desk, floor any, socket, state string, registered bool) map[string]any {
	return map[string]any{"id": id, "registered": registered, "tmuxSocket": socket, "floor": floor, "desk": desk, "state": state}
}

func TestSeatsAreTheGroundFloorHiresOnTheirOwnTmux(t *testing.T) {
	s := "/tmp/tmux-501/smolcorp-x"
	rows := []map[string]any{tileRow("b", 4, 0, s, "working", true), tileRow("a", 0, 0, s, "working", true), tileRow("up", 1, 2, s, "working", true),
		tileRow("none", nil, 0, s, "working", true), tileRow("plain", 2, 0, "", "working", true), tileRow("gone", 3, 0, s, "fired", true),
		tileRow("hand", 5, 0, s, "working", false), tileRow("twin", 4, 0, s, "working", true), tileRow("big", 9, 0, s, "working", true)}
	if got := hireIDs(tileSeats(rows)); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("seats %q, want a and b, by desk, one to a desk", got)
	}
}

// Tiling with a stand-in Mac app: each hire's window by its tmux client's tty, over its desk's cell; one with no
// window is missed; a terminal that refuses Smolcorp stops it there; a terminal that can't place windows gets the
// team grid; nobody to tile is said.
func TestTilingMovesEachHiresWindowByTheTTYItShows(t *testing.T) {
	mac := &macStandIn{}
	grid := gridOn(portraitScreen, 2560)
	seats := []tileSeat{{"a", 0, "/s/a"}, {"b", 4, "/s/b"}, {"c", 8, "/s/c"}}
	ttys := map[string]string{"/s/a": "/dev/ttys003", "/s/b": "/dev/ttys004"}
	tty := func(socket string) string { return ttys[socket] }
	noGrid := func([]string) bool { t.Error("the team grid for a terminal that places windows"); return false }

	report := tileWith(mac, seats, "terminal", grid, tty, noGrid)
	if report.Method != "windows" || !slices.Equal(report.Placed, []string{"a", "b"}) || !slices.Equal(report.Missed, []string{"c"}) ||
		report.Message != "2 placed; 1 not (no open window found for it, or Terminal put it elsewhere: it can do that on a screen other than the main one)." ||
		!slices.Equal(report.Office, []int{0, 1750, 1440, 810}) {
		t.Errorf("Terminal: %+v", report)
	}
	asked := mac.asked[len(mac.asked)-1]
	if asked.Action != "tile" || asked.Terminal != "terminal" || len(asked.Places) != 2 || asked.Places[1] != (tilePlace{"/dev/ttys004", grid.cells[4]}) {
		t.Errorf("asked %+v", asked)
	}

	mac.tileForbidden = true
	report = tileWith(mac, seats[:2], "iterm", grid, tty, noGrid)
	if !report.NeedsPermission || len(report.Placed) != 0 || len(report.Missed) != 2 || report.Message != tileNotAllowed("iTerm") ||
		!strings.Contains(report.Message, "Privacy & Security → Automation") {
		t.Errorf("iTerm2 refused: %+v", report)
	}

	var shown []string
	report = tileWith(mac, seats, "ghostty", grid, tty, func(sockets []string) bool { shown = sockets; return true })
	if report.Method != "team-grid" || len(report.Placed) != 3 || !slices.Equal(shown, []string{"/s/a", "/s/b", "/s/c"}) ||
		!strings.Contains(report.Message, "a pane to a hire, read-only") {
		t.Errorf("Ghostty: %+v", report)
	}
	if report = tileWith(mac, seats, "ghostty", grid, tty, func([]string) bool { return false }); report.Method != "none" || !strings.Contains(report.Message, "team grid couldn't be made") {
		t.Errorf("no team grid: %+v", report)
	}
	if report = tileWith(mac, nil, "terminal", grid, tty, noGrid); report.Method != "none" || !strings.Contains(report.Message, "nothing to tile") {
		t.Errorf("nobody: %+v", report)
	}
	mac.refusing(map[string]string{"tile": "Terminal couldn't be asked for its windows."})
	if report = tileWith(mac, seats[:1], "terminal", grid, tty, noGrid); report.Message != "Terminal couldn't be asked for its windows." || len(report.Missed) != 1 {
		t.Errorf("the app couldn't ask: %+v", report)
	}
}

// The routes under the Mac app: the screens as the app reads them, a choice kept in config.json (an id the Mac
// doesn't have is refused), and a tile with nobody to place. Without the Mac app they're 404.
func TestTheGMScreensRoutesUnderTheMacApp(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	if code, _, _ := fromPage(t, d, http.MethodGet, "/setup/gm/screens", nil); code != http.StatusNotFound {
		t.Errorf("without the Mac app: %d", code)
	}
	mac := withMac(t)
	mac.screens = []macScreen{builtIn, tallLeft}
	d = newTestDeck(t)
	managerOn(d)
	code, said, _ := fromPage(t, d, http.MethodGet, "/setup/gm/screens", nil)
	if code != http.StatusOK || said["chosen"] != "2" || said["ask"] != true {
		t.Errorf("screens: %d %v", code, said)
	}
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/screen", nil); code != http.StatusBadRequest || !strings.Contains(body, "id=<id>") {
		t.Errorf("no id: %d %q", code, body)
	}
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/screen?id=7", nil); code != http.StatusConflict || !strings.Contains(body, "No screen has that id") {
		t.Errorf("an unknown id: %d %q", code, body)
	}
	if code, said, _ := fromPage(t, d, http.MethodPost, "/setup/gm/screen?id=1", nil); code != http.StatusOK || said["chosen"] != "1" || loadConfig().TeamScreen != "1" {
		t.Errorf("chose 1: %d %v, kept %q", code, said, loadConfig().TeamScreen)
	}
	code, said, body := fromPage(t, d, http.MethodPost, "/setup/gm/tile", nil)
	var report tileReport
	if code != http.StatusOK || json.Unmarshal([]byte(body), &report) != nil || report.Method != "none" || !strings.Contains(report.Message, "nothing to tile") ||
		!slices.Equal(report.Office, []int{0, 131, 1512, 851}) {
		t.Errorf("tile with nobody: %d %v", code, said)
	}
	mac.screens = nil
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/tile", nil); code != http.StatusConflict || !strings.Contains(body, "No screen was found") {
		t.Errorf("no screen: %d %q", code, body)
	}
}

// B1 (RGAPC): a test copy never tiles. The tile route is told no before the Mac is asked for a screen or anything
// else, in the sentence of the other test-copy refusals; the screens and the choice of one are still the copy's.
func TestATestCopyRefusesTiling(t *testing.T) {
	mac := withMac(t)
	mac.screens = []macScreen{builtIn, tallLeft}
	onMarkedPort(t, 7799)
	if testCopyPort() == 0 {
		t.Fatal("not a test copy")
	}
	d := newTestDeck(t)
	managerOn(d)
	code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/tile", nil)
	if code != http.StatusConflict || body != testCopyKeepsWindows || !strings.HasPrefix(body, "This is a test copy of Smolcorp: it never ") {
		t.Errorf("tile in a test copy: %d %q", code, body)
	}
	mac.mu.Lock()
	for _, asked := range mac.asked {
		if asked.Action == "screens" || asked.Action == "tile" || asked.Action == "office-place" {
			t.Errorf("the Mac was asked %q by a refused tile", asked.Action)
		}
	}
	mac.mu.Unlock()
	if code, said, _ := fromPage(t, d, http.MethodGet, "/setup/gm/screens", nil); code != http.StatusOK || said["ask"] != true {
		t.Errorf("screens in a test copy: %d %v", code, said)
	}
	// The office's own port still tiles.
	onDefaultPort(t)
	if code, _, body := fromPage(t, d, http.MethodPost, "/setup/gm/tile", nil); code != http.StatusOK || strings.Contains(body, "test copy") {
		t.Errorf("tile on the office's port: %d %q", code, body)
	}
}
