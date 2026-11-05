package desktop

import (
	"regexp"
	"strings"
	"testing"
)

// The Geode pack: its pieces and its style are in every list the office keeps, and nothing was left as a placeholder.
var geodePieces = []string{
	"geodegolem", "geodepangolin", "geodefloor", "geodeagate", "geodewall", "geodecitrine", "geodedesk", "geodemon",
	"geodevault", "geodelantern", "geodecluster", "geodebell", "geodedrip", "geodedeep", "geodetumbler", "geodespecimens",
	"geodelamp", "geodeuvlamp", "geodehalf", "geodecaliper", "geodepan", "geodehammer", "geodegrotto", "geodesorter",
	"geodedoor", "geodeprism",
}

// Its gem outfits: one for each row of office.js's GEODE_GEMS, as `geode` and the gem.
var geodeGems = []string{"ruby", "opal", "sapphire", "amber", "garnet", "topaz", "pearl", "moonstone", "malachite", "jade", "lapis", "amethyst", "obsidian"}

func TestTheGeodePackIsInEveryList(t *testing.T) {
	known := map[string]bool{}
	for _, list := range [][]string{roomStyles, wallChoices, pets, workspaces, floors, paints} {
		for _, c := range list {
			known[c] = true
		}
	}
	for _, choices := range roomChoices {
		for _, c := range choices {
			known[c] = true
		}
	}
	for _, choices := range teamChoices {
		for _, c := range choices {
			known[c] = true
		}
	}
	web, viewer := officeSources(t)
	ids := append([]string{"geode"}, geodePieces...)
	for _, gem := range geodeGems {
		ids = append(ids, "geode"+gem)
	}
	for _, id := range ids {
		if !strings.HasPrefix(id, "geode") {
			t.Errorf("%s does not start with the pack's name", id)
		}
		if !known[id] {
			t.Errorf("%s is in none of layout.go's lists", id)
		}
		if !strings.Contains(string(web), "'"+id+"'") && !strings.Contains(string(web), id+":") && !strings.Contains(string(web), id+"(") {
			t.Errorf("%s is nowhere in office.js", id)
		}
	}
	if strings.Contains(string(web), "add-piece: TODO") || strings.Contains(string(viewer), "add-piece: TODO") {
		t.Error("a placeholder is still in office.js")
	}
}

// Each gem outfit is drawn from its gem's row: the table has a row for every gem the lists know, and none they don't.
func TestEveryGeodeGemOutfitIsARowOfTheTable(t *testing.T) {
	web, _ := officeSources(t)
	src := string(web)
	at := strings.Index(src, "const GEODE_GEMS = {")
	if at < 0 {
		t.Fatal("GEODE_GEMS is gone: update this test")
	}
	table := src[at : at+strings.Index(src[at:], "\n  };")]
	rows := regexp.MustCompile(`(?m)^    ([a-z]+): \{ base: 0x[0-9a-f]{6}, light: 0x[0-9a-f]{6}, dark: 0x[0-9a-f]{6}, sheen: '[a-z]+', prop: `).FindAllStringSubmatch(table, -1)
	inTable := map[string]bool{}
	for _, row := range rows {
		inTable[row[1]] = true
	}
	outfits := map[string]bool{}
	for _, c := range teamChoices["outfit"] {
		if strings.HasPrefix(c, "geode") {
			outfits[strings.TrimPrefix(c, "geode")] = true
		}
	}
	for _, gem := range geodeGems {
		if !inTable[gem] {
			t.Errorf("%s has no row in GEODE_GEMS", gem)
		}
		if !outfits[gem] {
			t.Errorf("geode%s is not among layout.go's outfits", gem)
		}
	}
	if len(inTable) != len(geodeGems) || len(outfits) != len(geodeGems) {
		t.Errorf("GEODE_GEMS has %d rows and layout.go %d gem outfits, for %d gems: add the new gem to every list", len(inTable), len(outfits), len(geodeGems))
	}
	if !strings.Contains(src, "geode: GEODE_OUTFITS,") {
		t.Error("STYLE_DRESS no longer dresses the Geode in its gems")
	}
}
