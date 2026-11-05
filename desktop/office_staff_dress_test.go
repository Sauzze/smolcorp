package desktop

import (
	"os/exec"
	"strings"
	"testing"
)

// In a style that dresses each person differently, the office's staff wore plain clothes among people all in the
// style's outfits. They're dressed from its list now, picked by their name as an agent is by its project's; in any
// other style they wear what they did. The two functions run in node on a tiny stand-in for their surroundings;
// without node, only drawPerson's call is checked.
func TestTheStaffWearTheStyleThatDressesEveryone(t *testing.T) {
	web, viewer := officeSources(t)
	for name, src := range map[string]string{"office.js": string(web), "the viewer's office.js": string(viewer)} {
		if !strings.Contains(src, "person.isStaff ? staffOutfitOf(look, crewDress, person.session.name)") {
			t.Errorf("%s's drawPerson no longer gives staffOutfitOf the staff member's name", name)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: staffOutfitOf can't be run")
	}
	src := string(web)
	fn := func(head string) string {
		at := strings.Index(src, head)
		if at < 0 {
			t.Fatalf("%q is gone from office.js: update this test", head)
		}
		body := src[at:]
		return body[:strings.Index(body, "\n  }\n")+4] + "\n"
	}
	script := `
let decor = { preset: 'p' };
const STYLE_DRESS = { p: ['a', 'b', 'c', 'd'] };
const STAFF_SHIRTS = [1, 2, 3, 4, 5];
const STAFF_DRESS = { robe: 'knight' };
const PALETTE = { pants: 6, shoes: 7 };
const dressFor = (style, accent) => ({ style: style || 'plain', shirt: accent });
` + fn("function styleDressOf(seed) {") + fn("function staffOutfitOf(look, dress, name = null) {") + `
const worn = new Set();
for (let n = 1; n <= 40; n++) {
  const got = staffOutfitOf({ phase: (n * 0.137) % 1 }, null, 'staff ' + n).style;
  if (!STYLE_DRESS.p.includes(got)) throw new Error('staff ' + n + ' wears ' + got + ', none of the style');
  if (got !== styleDressOf('staff ' + n)) throw new Error('staff ' + n + ' is not dressed as an agent of that name');
  worn.add(got);
}
if (worn.size < 2) throw new Error('every member of staff wears the same outfit');
decor = { preset: 'q' };
if (staffOutfitOf({ phase: 0.5 }, null, 'staff 1').style !== 'plain') throw new Error('staff in a style with no list are out of plain clothes');
if (staffOutfitOf({ phase: 0.5 }, 'robe', 'staff 1').style !== 'knight') throw new Error('staff no longer wear what most desks give');
`
	if out, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
