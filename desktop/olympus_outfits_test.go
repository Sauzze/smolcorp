package desktop

import (
	"os"
	"strings"
	"testing"
)

// The Greek gods: nine outfits from myth, no brand and nobody real, and Mount Olympus, the style that dresses everyone
// in them. Their ids are plain, so every list that names an outfit or a style has to name them too.
var olympusOutfits = []string{"olympuszeus", "olympushera", "olympusposeidon", "olympusathena", "olympusapollo", "olympusartemis",
	"olympushermes", "olympusares", "olympusaphrodite"}

func TestTheGreekGodsAreInEveryList(t *testing.T) {
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
	dress := "olympus: ['" + strings.Join(olympusOutfits, "', '") + "']"
	// The style, from pieces other styles have, and the nine it dresses each person in
	if !strings.Contains(src, "olympus: { name: 'Mount Olympus'") {
		t.Errorf("the olympus style is not in %s's ROOM_STYLES", name)
	}
	if !strings.Contains(src, dress) {
		t.Errorf("%s's STYLE_DRESS does not dress the olympus style in the nine", name)
	}
	if strings.Contains(src, "add-piece: TODO") {
		t.Errorf("a placeholder is still in %s", name)
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
	for _, id := range olympusOutfits {
		// Its colours in OUTFITS and its drawing in OUTFIT_PARTS
		if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") {
			t.Errorf("%s is not wholly in %s (OUTFITS and OUTFIT_PARTS)", id, name)
		}
		// Its place among the team's choices: an outfit missing from them is drawn as Auto
		if !strings.Contains(choices, "'"+id+"'") {
			t.Errorf("%s is not among %s's TEAM_CHOICES outfits: a team member or a desk picking it is drawn as Auto", id, name)
		}
	}
	if !styles["olympus"] || !strings.Contains(setup, "['olympus', 'Mount Olympus']") || !strings.Contains(swift, `"olympus"`) {
		t.Error("the olympus style is missing from layout.go's, setup.html's or Layout.swift's styles")
	}
	for _, id := range olympusOutfits {
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
