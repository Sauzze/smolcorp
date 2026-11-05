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
	web, viewer := officeSources(t)
	setup, swift := read("../Resources/web/setup.html"), read("../Sources/AgentDeck/Layout.swift")
	dress := "olympus: ['" + strings.Join(olympusOutfits, "', '") + "']"
	for name, src := range map[string]string{"office.js": string(web), "the viewer's office.js": string(viewer)} {
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
		at := strings.Index(src, "const TEAM_CHOICES = {")
		if at < 0 {
			t.Fatalf("TEAM_CHOICES is gone from %s: update this test", name)
		}
		team := src[at:]
		end := strings.Index(team, "\n  };")
		if end < 0 {
			t.Fatalf("TEAM_CHOICES has no end in %s: update this test", name)
		}
		team = team[:end]
		for _, id := range olympusOutfits {
			// Its colours in OUTFITS, its drawing in OUTFIT_PARTS, and its place among the team's choices
			if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") || !strings.Contains(team, "'"+id+"'") {
				t.Errorf("%s is not wholly in %s (OUTFITS, OUTFIT_PARTS and TEAM_CHOICES)", id, name)
			}
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
