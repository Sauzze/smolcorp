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
	web, viewer := officeSources(t)
	setup, swift := read("../Resources/web/setup.html"), read("../Sources/AgentDeck/Layout.swift")
	dress := "zoo: ['" + strings.Join(zooOutfits, "', '") + "']"
	// The team's outfit choices alone (TEAM_CHOICES.outfit), since OUTFITS names each id as well
	outfitChoices := func(src string) string {
		i := strings.Index(src, "const TEAM_CHOICES = {")
		if i < 0 {
			return ""
		}
		rest := src[i:]
		if j := strings.Index(rest, "\n    outfit: ["); j >= 0 {
			rest = rest[j:]
			if k := strings.Index(rest, "],\n"); k >= 0 {
				return rest[:k]
			}
		}
		return ""
	}
	for name, src := range map[string]string{"office.js": string(web), "the viewer's office.js": string(viewer)} {
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
		choices := outfitChoices(src)
		for _, id := range zooOutfits {
			// Its colours in OUTFITS, its drawing in OUTFIT_PARTS, and its place in the team's choices
			if !strings.Contains(src, id+": { style: '"+id+"'") || !strings.Contains(src, "\n      "+id+": {\n") || !strings.Contains(choices, "'"+id+"'") {
				t.Errorf("%s is not wholly in %s (OUTFITS, OUTFIT_PARTS and TEAM_CHOICES)", id, name)
			}
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
