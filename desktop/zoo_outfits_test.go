package desktop

import (
	"os"
	"strings"
	"testing"
)

// The zoo: nine animal onesies, original and public, and the team style that dresses the whole office in them.
var zooOutfits = []string{"zoolion", "zooelephant", "zoogiraffe", "zoozebra", "zoopenguin", "zoopanda", "zoomonkey",
	"zooflamingo", "zoocroc"}

func TestTheZooOnesiesAreInEveryList(t *testing.T) {
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
	dress := "zoo: ['" + strings.Join(zooOutfits, "', '") + "']"
	// The style, from pieces other styles have, and the nine it dresses each person in
	if !strings.Contains(src, "zoo: { name: 'Zoo'") {
		t.Errorf("the zoo style is not in %s's ROOM_STYLES", name)
	}
	if !strings.Contains(src, dress) {
		t.Errorf("%s's STYLE_DRESS does not dress the zoo style in the nine", name)
	}
	if strings.Contains(src, "add-piece: TODO") {
		t.Errorf("a placeholder is still in %s", name)
	}
	// TEAM_CHOICES' outfit list alone, which a team member's or a desk's outfit is checked against (OUTFITS and
	// STYLE_DRESS name each id as well)
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
	for _, id := range zooOutfits {
		// Its colours in OUTFITS and its drawing in OUTFIT_PARTS
		if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") {
			t.Errorf("%s is not wholly in %s (OUTFITS and OUTFIT_PARTS)", id, name)
		}
		// Its place among the team's choices: an outfit missing from them is drawn as Auto
		if !strings.Contains(choices, "'"+id+"'") {
			t.Errorf("%s is not among %s's TEAM_CHOICES outfits: a team member or a desk picking it is drawn as Auto", id, name)
		}
	}
	if !styles["zoo"] || !strings.Contains(setup, "['zoo', 'Zoo']") || !strings.Contains(swift, `"zoo"`) {
		t.Error("the zoo style is missing from layout.go's, setup.html's or Layout.swift's styles")
	}
	if !strings.Contains(setup, "zoo: ['places', 'Zoo']") {
		t.Error("setup.html doesn't shelve the zoo among the places")
	}
	for _, id := range zooOutfits {
		if !strings.HasPrefix(id, "zoo") {
			t.Errorf("%s should start with zoo, the style's id, to sit on its shelf", id)
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
