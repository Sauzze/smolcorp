package desktop

import (
	"os"
	"strings"
	"testing"
)

// The royal court: nine original outfits (no real people, no brand) and their team style, the castle's great hall. Every
// id starts with castle, as `court` is the courtroom's.
var castleOutfits = []string{"castleking", "castlequeen", "castlejester", "castleherald", "castleknight", "castlepage", "castlecook",
	"castlefalconer", "castlelady"}

func TestTheRoyalCourtIsInEveryList(t *testing.T) {
	outfits := map[string]bool{}
	for _, c := range teamChoices["outfit"] {
		outfits[c] = true
	}
	styles := map[string]bool{}
	for _, c := range roomStyles {
		styles[c] = true
	}
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	src, name := string(officeSource(t)), "office.js"
	setup, swift := read("../Resources/web/setup.html"), read("../Sources/AgentDeck/Layout.swift")
	dress := "castle: ['" + strings.Join(castleOutfits, "', '") + "']"
	// The style, from pieces other styles have, and the nine it dresses each person in
	if !strings.Contains(src, "castle: { name: 'Castle great hall'") {
		t.Errorf("the castle style is not in %s's ROOM_STYLES", name)
	}
	if !strings.Contains(src, dress) {
		t.Errorf("%s's STYLE_DRESS does not dress the castle style in the nine", name)
	}
	if strings.Contains(src, "add-piece: TODO") {
		t.Errorf("a placeholder is still in %s", name)
	}
	// The court's own colours, which its outfits draw with
	for _, c := range []string{"CASTLE_GOLD", "CASTLE_GOLD_LIGHT", "CASTLE_GOLD_DARK", "CASTLE_STEEL"} {
		if !strings.Contains(src, c+" = 0x") {
			t.Errorf("the court's colour %s is missing from %s: its outfits can't be drawn without it", c, name)
		}
	}
	// TEAM_CHOICES' outfit list, which a team member's or a desk's outfit is checked against
	at := strings.Index(src, "const TEAM_CHOICES = {")
	if at < 0 {
		t.Fatalf("TEAM_CHOICES is gone from %s: update this test", name)
	}
	choices := src[at:]
	start := strings.Index(choices, "\n    outfit: [")
	if start < 0 {
		t.Fatalf("TEAM_CHOICES has no outfit list in %s: update this test", name)
	}
	choices = choices[start:]
	end := strings.Index(choices, "],\n")
	if end < 0 {
		t.Fatalf("TEAM_CHOICES' outfit list has no end in %s: update this test", name)
	}
	choices = choices[:end]
	for _, id := range castleOutfits {
		// Its colours in OUTFITS and its drawing in OUTFIT_PARTS
		if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") {
			t.Errorf("%s is not wholly in %s (OUTFITS and OUTFIT_PARTS)", id, name)
		}
		// Its place among the team's choices: an outfit missing from them is drawn as Auto
		if !strings.Contains(choices, "'"+id+"'") {
			t.Errorf("%s is not among %s's TEAM_CHOICES outfits: a team member or a desk picking it is drawn as Auto", id, name)
		}
	}
	if !styles["castle"] || !strings.Contains(setup, "['castle', 'Castle great hall']") || !strings.Contains(swift, `"castle"`) {
		t.Error("the castle style is missing from layout.go's, setup.html's or Layout.swift's styles")
	}
	// Its own shelf in setup, among the places, where its outfits go too
	if !strings.Contains(setup, "castle: ['places', 'Castle great hall']") {
		t.Error("setup.html has no shelf for the castle style")
	}
	for _, id := range castleOutfits {
		if !strings.HasPrefix(id, "castle") {
			t.Errorf("%s does not start with castle", id)
		}
		if !outfits[id] {
			t.Errorf("%s is not among layout.go's outfits", id)
		}
		if !strings.Contains(setup, "['"+id+"', '") {
			t.Errorf("%s has no name in setup.html", id)
		}
		if !strings.Contains(swift, `"`+id+`"`) {
			t.Errorf("%s is not among Layout.swift's outfits", id)
		}
	}
}
