package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// officePiece is a piece of office.js, from the line that starts with start to the first line after it that is
// end, end included: a function to run in node on stand-ins for its surroundings.
func officePiece(t *testing.T, src, start, end string) string {
	t.Helper()
	at := strings.Index(src, start)
	if at < 0 {
		t.Fatalf("office.js no longer has %q: update this test", start)
	}
	piece := src[at:]
	stop := strings.Index(piece, end)
	if stop < 0 {
		t.Fatalf("office.js's %q no longer ends with %q: update this test", start, end)
	}
	return piece[:stop+len(end)]
}

// runNodeFile runs script in node, from a file named name, or skips the test without node.
func runNodeFile(t *testing.T, name, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the office's drag can't be run")
	}
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// A desk pinned upstairs shows its folder's name, as a pinned key does downstairs, past the last desk with someone
// at it too; a floor from before pins has none.
func TestAFloorUpstairsShowsItsPinnedDesks(t *testing.T) {
	src := string(officeSource(t))
	script := `
const UPSTAIRS_MOST = 9;
const STANDARD_STATES = { idle: 1, thinking: 1 };
` + officePiece(t, src, "  function seatsOf(snapshot, ahead) {", "\n  }\n") +
		officePiece(t, src, "  function upstairsSeatsOf(given, ahead, pins = []) {", "\n  }\n") + `
const seats = upstairsSeatsOf([{ id: 's1', project: 'app', state: 'idle', desk: 0, drivable: true }], 0, ['app', null, null, null, null, null, null, 'kept']);
if (seats.length !== 8) throw new Error('8 desks, the last a pinned one, want ' + seats.length);
if (seats[0].pin !== 'app' || seats[0].session.id !== 's1') throw new Error('desk 1: ' + JSON.stringify(seats[0]));
if (seats[7].pin !== 'kept' || seats[7].session) throw new Error('desk 8, kept for its folder: ' + JSON.stringify(seats[7]));
if (seats[3].pin !== null) throw new Error('desk 4 is pinned to nothing: ' + JSON.stringify(seats[3]));
const before = upstairsSeatsOf([{ id: 's1', project: 'app', state: 'idle', desk: 2 }], 0);
if (before.length !== 3 || before.some((s) => s.pin !== null)) throw new Error('a floor from before pins: ' + JSON.stringify(before));
`
	runNodeFile(t, "pins.js", script)
}

// Held and carried on a floor upstairs, an agent is let go at another desk there, and the two swap on that floor:
// counted from 0 for the ground floor, as the computer counts floors. Downstairs, the keys swap as they always did.
func TestADragOnAFloorUpstairsSwapsThatFloorsDesks(t *testing.T) {
	src := string(officeSource(t))
	lift := officePiece(t, src, "      if (carrying) {\n        const { from, over, toLift, person, floor } = carrying;", "\n        return;\n      }\n")
	script := `
const UPSTAIRS_MOST = 9, CARRY_AFTER = 10, DESK_WIDTH = 30;
const DESKS = Array.from({ length: 9 }, (_, i) => ({ x: 40 * i, y: 0 }));
const WORKSPACES = { desk: { top: -10 } };
const workspaceOf = () => 'desk';
const roomAt = (x, y) => ({ x, y });
const at = (id) => ({ session: { id } });
// The ground floor's keys, and 3F upstairs: s2 at its second desk, desk 4 kept for a folder.
const office = { seats: [at('g0'), at('g1')], people: [at('g0'), at('g1')] };
const third = { seats: [at('s1'), at('s2'), {}, { pin: 'kept' }], people: [at('s1'), at('s2')] };
let floorShown = 3, held = null, carrying = null, finger = null, picker = null;
const inView = (fn) => (floorShown >= 2 ? fn(third) : fn(office));
const deskInView = (id) => inView((there) => there.seats.findIndex((s) => s && s.session && s.session.id === id));
const swaps = [];
const on = { held() {}, swap(...args) { swaps.push(args); } };
const floorList = () => [{}, {}, {}];
const elevatorBox = () => null;
const openMoving = () => { throw new Error('nothing was let go over the elevator'); };
` + officePiece(t, src, "    function deskBoxes(desk, there = office) {", "\n    }\n") +
		officePiece(t, src, "    function deskAt(x, y, there = office) {", "\n    }\n") +
		officePiece(t, src, "    const desksNow = () =>", ";\n") +
		officePiece(t, src, "    function carry(e) {", "\n    }\n") + `
function letGo(dropped) {
` + lift + `
}
function drag(id, from, to) {
  finger = { held: true, target: { kind: 'session', id }, x: from, y: 5 };
  carry({ clientX: from + 2, clientY: 5 });
  carry({ clientX: to, clientY: 5 });
  if (!carrying) throw new Error(id + ' was not picked up on floor ' + floorShown);
  letGo(true);
}
// 3F: s2, at desk 2, let go at desk 3, an empty one: the floor's desks swap, floor 2 counted from 0.
drag('s2', 45, 125);
if (JSON.stringify(swaps) !== JSON.stringify([[1, 3, 2]])) throw new Error('3F: ' + JSON.stringify(swaps));
if (carrying !== null) throw new Error('still carried after the drop');
// Let go where it was picked up: nothing.
drag('s1', 5, 25);
if (swaps.length !== 1) throw new Error('a drop at its own desk swapped: ' + JSON.stringify(swaps));
// Over a desk past the floor's last seat, upstairs: any of its nine.
drag('s1', 5, 325);
if (JSON.stringify(swaps[1]) !== JSON.stringify([0, 8, 2])) throw new Error('to desk 9 on 3F: ' + JSON.stringify(swaps));
// Downstairs, the keys, as before: floor 0, and only as far as there are keys.
floorShown = 1;
drag('g0', 5, 45);
if (JSON.stringify(swaps[2]) !== JSON.stringify([0, 1, 0])) throw new Error('downstairs: ' + JSON.stringify(swaps));
// The desks counted are the floor in view's: 3F's seats changing meanwhile let the carry go, the ground floor's don't.
floorShown = 3;
const was = desksNow();
third.seats[2] = at('s3');
if (desksNow() === was) throw new Error('3F changed and the carry would swap the wrong desks');
floorShown = 1;
if (desksNow().startsWith('3')) throw new Error('downstairs, the desks counted are 3F\'s');
`
	runNodeFile(t, "drag.js", script)
}

// The page sends the floor with a swap upstairs, and none for the ground floor's keys, as before floors had pins.
func TestThePageSendsTheFloorWithASwap(t *testing.T) {
	page, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "send('/act', { action: 'swap', from, to, ...(floor ? { floor } : {}) });") {
		t.Error("deck.html's swap no longer sends the floor upstairs")
	}
}
