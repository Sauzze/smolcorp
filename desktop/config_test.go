package desktop

import (
	"reflect"
	"slices"
	"testing"
)

func TestOnlyRealColourChangesAreKept(t *testing.T) {
	got := customisations(map[string]any{
		"thinking": "#2F6BFF",       // kept, in lower case
		"asking":   "#ff8a1f",       // the default already: nothing to keep
		"unread":   "green",         // not #rrggbb
		"error":    "#ff3b30; x: y", // not only a colour
		"body":     12,              // not text
		"shadow":   "#000000",       // not one of the theme's colours
	})
	if want := map[string]string{"thinking": "#2f6bff"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kept %v, want %v", got, want)
	}
}

func TestTheThemeIsTheDefaultsWithYourChanges(t *testing.T) {
	theme := currentTheme(&Config{Colours: map[string]string{"thinking": "#2f6bff"}})
	if len(theme) != len(colours) || theme["thinking"] != "#2f6bff" || theme["body"] != standardColours["body"] {
		t.Fatalf("the theme is %v", theme)
	}
}

func TestAProjectIsPinnedToOneKeyAtATime(t *testing.T) {
	isolate(t)
	config := loadConfig()
	config.Pin("/work/api-server", 1)
	config.Pin("/work/api-server", 4)
	saved := loadConfig()
	if saved.Pins[1] != nil || saved.Pins[4] == nil || *saved.Pins[4] != "/work/api-server" {
		t.Fatalf("pins are %v, want the project on key 5 only", saved.Pins)
	}
	saved.Unpin(4)
	if loadConfig().Pins[4] != nil {
		t.Fatal("an unpinned key kept its project")
	}
}

func TestSettingsFromAnotherVersionStillLoad(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"pins": ["/work/a"], "logo": "a logo that went away", "colours": null, "somethingNew": true}`)
	config := loadConfig()
	if len(config.Pins) != mostKeys || config.Pins[0] == nil || *config.Pins[0] != "/work/a" {
		t.Errorf("pins are %v, want the saved one and %d pin slots", config.Pins, mostKeys)
	}
	if config.Logo != "smolcorp" || config.Colours == nil {
		t.Errorf("logo %q and colours %v, want the defaults", config.Logo, config.Colours)
	}
}

func TestTheOldMarkBecomesTheHire(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"logo": "sauzze"}`)
	if logo := loadConfig().Logo; logo != "smolcorp" {
		t.Errorf("a saved sauzze logo loads as %q, want smolcorp", logo)
	}
	writeFile(t, configFile, `{"logo": "keys"}`)
	if logo := loadConfig().Logo; logo != "keys" {
		t.Errorf("the six keys load as %q", logo)
	}
}

func TestPetsKeptOneAtATimeStillLoadAndThreeComeBack(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"layout": {"room": {"pet": "dog"}},
		"profiles": [{"name": "Old", "layout": {"room": {"pet": "cat", "preset": "campsite"}}, "colours": {}}]}`)
	config := loadConfig()
	if !slices.Equal(config.Layout.Room.Pet, []string{"dog"}) || len(config.Profiles) != 1 ||
		!slices.Equal(config.Profiles[0].Layout.Room.Pet, []string{"cat"}) {
		t.Fatalf("an office kept with one pet loads with %q, and its profiles %v; want [dog] and Old with [cat]",
			config.Layout.Room.Pet, config.Profiles)
	}
	config.Layout.Room.Pet = []string{"llama", "owl", "fox"}
	config.save()
	if pets := loadConfig().Layout.Room.Pet; !slices.Equal(pets, []string{"llama", "owl", "fox"}) {
		t.Errorf("three pets saved come back as %q", pets)
	}
}
