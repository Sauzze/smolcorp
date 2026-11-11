package desktop

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The ant colony: nine original outfits (no brand, no real person) and the team style that dresses the whole office in
// them. Each must be wherever an outfit or a style is listed, or the Mac, the computer, setup or the office would refuse it.
var antColonyOutfits = []string{"antcolonyqueen", "antcolonyworker", "antcolonysoldier", "antcolonyleafcutter", "antcolonynurse",
	"antcolonydrone", "antcolonyarchitect", "antcolonyhoneypot", "antcolonyfarmer"}

func TestTheAntColonyIsInEveryList(t *testing.T) {
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
	// STYLE_DRESS's list runs over two lines: compared without its spaces and line breaks
	spaces := regexp.MustCompile(`\s+`)
	dress := "antcolony:['" + strings.Join(antColonyOutfits, "','") + "'],"
	// The style, from pieces other styles have, and the nine it dresses each person in
	if !strings.Contains(src, "antcolony: { name: 'Ant colony'") {
		t.Errorf("the antcolony style is not in %s's ROOM_STYLES", name)
	}
	at := strings.Index(src, "  const STYLE_DRESS = {")
	if at < 0 {
		t.Fatalf("STYLE_DRESS is gone from %s: update this test", name)
	}
	block := src[at:]
	block = block[:strings.Index(block, "\n  };")]
	if !strings.Contains(spaces.ReplaceAllString(block, ""), dress) {
		t.Errorf("%s's STYLE_DRESS does not dress the antcolony style in the nine", name)
	}
	if strings.Contains(src, "add-piece: TODO") {
		t.Errorf("a placeholder is still in %s", name)
	}
	// The team's outfit choices: TEAM_CHOICES's own outfit list, up to its colours (STYLE_DRESS names the nine too)
	at = strings.Index(src, "  const TEAM_CHOICES = {")
	if at < 0 {
		t.Fatalf("TEAM_CHOICES is gone from %s: update this test", name)
	}
	team := src[at:]
	from, to := strings.Index(team, "\n    outfit: ["), strings.Index(team, "\n    colour: [")
	if from < 0 || to < from {
		t.Fatalf("TEAM_CHOICES's outfit list is not where it was in %s: update this test", name)
	}
	team = team[from:to]
	// The team's own drawing helpers, after OUTFIT_PARTS: without them, drawing any of the nine would stop the office
	for _, helper := range []string{"costumeAntHead", "costumeAntLegs", "costumeAntSprite"} {
		if strings.Count(src, "function "+helper+"(") != 1 {
			t.Errorf("%s is not in %s just once", helper, name)
		}
	}
	for _, id := range antColonyOutfits {
		// Its colours in OUTFITS and its drawing in OUTFIT_PARTS
		if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") {
			t.Errorf("%s is not wholly in %s (OUTFITS and OUTFIT_PARTS)", id, name)
		}
		// Its place among the team's choices: an outfit missing from them is drawn as Auto
		if !strings.Contains(team, "'"+id+"'") {
			t.Errorf("%s is not among %s's TEAM_CHOICES outfits: a team member or a desk picking it is drawn as Auto", id, name)
		}
	}
	if !styles["antcolony"] || !strings.Contains(setup, "['antcolony', 'Ant colony']") || !strings.Contains(swift, `"antcolony"`) {
		t.Error("the antcolony style is missing from layout.go's, setup.html's or Layout.swift's styles")
	}
	if !strings.Contains(setup, "antcolony: ['places', 'Ant colony'],") {
		t.Error("setup.html shelves the antcolony style nowhere: it belongs among the Places")
	}
	for _, id := range antColonyOutfits {
		// Public ids: the team's own prefix, none of the private ones that scripts/check-public.sh refuses
		if !strings.HasPrefix(id, "antcolony") {
			t.Errorf("%s does not start with antcolony", id)
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
