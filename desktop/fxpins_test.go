package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// The setup state lists the pins above the ground floor, as `pins` lists the ground floor's, for the page to show
// and undo; the ground floor's stay where they were. The page's unpin, on any floor, frees the desk.
func TestSetupStateListsEveryFloorsPinsAndTheOwnerUnpinsThem(t *testing.T) {
	d := newTestDeck(t)
	d.config.PinOn("/work/down", 0, 2)
	d.config.PinOn("/work/mid", 2, 4)
	d.config.PinOn("/work/top", 5, 8)
	state := (&router{deck: d}).setupState()
	if pins, _ := state["pins"].([]any); len(pins) != mostKeys || pins[2] == nil || pins[4] != nil {
		t.Errorf("the ground floor's pins moved: %v", state["pins"])
	}
	listed, _ := state["floorPins"].([]any)
	if len(listed) != 2 {
		t.Fatalf("floorPins is %v, want the two upstairs", state["floorPins"])
	}
	if first, _ := listed[0].(map[string]any); first["floor"] != 2 || first["desk"] != 4 || first["folder"] != "/work/mid" || first["name"] != "mid" {
		t.Errorf("the first pin upstairs is %v", listed[0])
	}
	// The owner's own page, with no GM: frees the desk on 3F and on 6F, leaving the ground floor's.
	for _, target := range []string{"/setup/unpin?key=4&floor=2", "/setup/unpin?key=8&floor=5"} {
		if got := pinPost(t, d, target, "", ""); got != http.StatusNoContent {
			t.Fatalf("the page's %s: %d", target, got)
		}
	}
	if left, _ := (&router{deck: d}).setupState()["floorPins"].([]any); len(left) != 0 || d.config.Pins[2] == nil {
		t.Errorf("after the unpins: %v, ground %v", left, d.config.Pins)
	}
}

// The setup page lists every pinned desk, the ground floor's first, by the floor's name and the desk's number as
// the office shows it, and Unpin sends the floor and desk the pin routes count from 0 (no floor on the ground
// floor, as before). Run in node on stand-ins for the page.
func TestTheSetupPageUnpinsADeskOnAnyFloor(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	script := `
const element = (tag) => ({ tag, dataset: {}, children: [], attrs: {}, append(...c) { this.children = c; },
  replaceChildren(...c) { this.children = c; }, setAttribute(k, v) { this.attrs[k] = v; } });
const document = { createElement: element };
const ids = { pinned: element('div'), 'pinned-box': element('div') };
const $ = (id) => ids[id];
const posted = [];
const post = (path) => { posted.push(path); };
let state = {
  floors: [{ name: '1F' }, { name: '2F' }, { name: 'Servers' }],
  pins: [null, { folder: '/work/down', name: 'down' }, null],
  floorPins: [{ floor: 2, desk: 4, folder: '/work/mid', name: 'mid' }, { floor: 1, desk: 0, folder: '/work/up', name: 'up' }],
};
` + officePiece(t, src, "  function changed(host, from) {", "\n  }\n") +
		officePiece(t, src, "  function pinnedDesks(st) {", "\n  }\n") +
		officePiece(t, src, "  function renderPinned() {", "\n  }\n") + `
renderPinned();
const rows = ids.pinned.children;
if (ids['pinned-box'].hidden) throw new Error('the box is hidden with pins');
const text = rows.map((r) => r.children[0].textContent);
const want = ['down · 1F, desk 2', 'up · 2F, desk 1', 'mid · Servers, desk 5'];
if (JSON.stringify(text) !== JSON.stringify(want)) throw new Error('rows ' + JSON.stringify(text));
rows.forEach((r) => r.children[1].onclick());
const wantPosts = ['/setup/unpin?key=1', '/setup/unpin?key=0&floor=1', '/setup/unpin?key=4&floor=2'];
if (JSON.stringify(posted) !== JSON.stringify(wantPosts)) throw new Error('posts ' + JSON.stringify(posted));
if (rows[2].children[1].attrs['aria-label'] !== 'Unpin mid from Servers, desk 5') throw new Error('label ' + rows[2].children[1].attrs['aria-label']);
// Nothing pinned: the box goes.
state = { floors: state.floors, pins: [null], floorPins: [] };
renderPinned();
if (!ids['pinned-box'].hidden) throw new Error('the box shows with no pins');
`
	runNodeFile(t, "pinned.js", script)
}

// A project pinned to a desk on a floor has its desk kept there; a hire in T3 Code or a cloud bot keeps its own
// desk (the registry's, the bot's floor), and a pin on its folder takes neither one there nor holds a desk back
// from it: the desk stays kept for the folder's agents.
func TestAPinOnAT3HiresFolderOrACloudBotsDoesNotMoveIt(t *testing.T) {
	isolate(t)
	floor, desk := 2, 4
	hire := t3DeskHire(homeDir, t3DeskTestID)
	hire.T3Floor, hire.T3Desk = &floor, &desk
	t3Registered(t, hire)
	d := startDeck(t)
	id := t3DeskID(t3DeskTestID)
	withCloudBots(d, func() CloudBot { b := standardCloudBotNamed("mini", "Smolcorp mini"); b.Floor = "3"; return b }())
	atDesks := func() map[string]float64 {
		got := map[string]float64{}
		for _, one := range floorDesks(t, d, 2) {
			got[one["id"].(string)] = one["desk"].(float64)
		}
		return got
	}
	botID := ""
	for id := range atDesks() {
		if id != t3DeskID(t3DeskTestID) {
			botID = id
		}
	}
	if got := atDesks(); got[id] != 4 || botID == "" {
		t.Fatalf("3F before any pin: %v", got)
	}
	// The hire's worktree and the bot's folder pinned to the first desk of 3F.
	d.config.PinOn(hire.Worktree, 2, 0)
	if got := atDesks(); got[id] != 4 || got[botID] == 0 {
		t.Errorf("a pin on the hire's folder moved them: %v", got)
	}
	d.config.PinOn("/cloud/Smolcorp mini", 2, 1)
	if got := atDesks(); got[id] != 4 || got[botID] == 1 || len(got) != 2 {
		t.Errorf("a pin on the bot's folder moved them: %v", got)
	}
	// The desks stay kept for their folders: nobody else sits there.
	for _, who := range []string{id, botID} {
		if at := atDesks()[who]; at == 0 || at == 1 {
			t.Errorf("%s sits at a desk kept for a folder: %v", who, at)
		}
	}
}

// A GM held to one project names the folder of a project pinned on any floor, not only the ground floor's.
func TestANicknameFindsAProjectPinnedOnAnyFloor(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	gm, _ := withGMAndAHire(t, d)
	folder := filepath.Join(homeDir, "upstairs-app")
	os.MkdirAll(folder, 0o755)
	d.config.PinOn(folder, 3, 6)
	body := `{"folder":"` + folder + `","name":"Ada"}`
	got := serve(t, d, call{method: "POST", target: "/setup/gm/nickname", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		body: body, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}}).Code
	if got != http.StatusOK {
		t.Fatalf("naming a project pinned on 4F: %d, want 200 (it was 404 with only the ground floor's pins read)", got)
	}
	if d.config.Layout.Room.Team["upstairs-app"].Name != "Ada" {
		t.Errorf("the team is %+v", d.config.Layout.Room.Team)
	}
}
