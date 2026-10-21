package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

// Config is what you set on the setup page: which project is pinned to which
// key, the deck's layout and colours, the profiles they're saved in, its logo,
// whether two fingers scroll and whether the office hires and fires. Saved as
// JSON next to the paired phones, in the Mac app's format.
type Config struct {
	// A project folder per key, or nil for a key that takes any session.
	Pins []*string `json:"pins"`
	// Only the colours that differ from the default theme.
	Colours map[string]string `json:"colours"`
	Layout  Layout            `json:"layout"`
	// Saved looks, besides Default, and the one the deck was last switched to.
	Profiles      []Profile `json:"profiles"`
	ActiveProfile string    `json:"activeProfile"`
	Logo          string    `json:"logo"`
	// Whether two fingers on the office scroll tmux and kitty panes. Off until you turn it
	// on, so a newly paired phone can do no more than the office's keys do.
	ScrollEnabled bool `json:"scrollEnabled"`
	// Whether the office may hire and fire: open a new terminal with an agent
	// in it, and stop an agent and close its pane. Off until you turn it on,
	// for the same reason.
	HiringEnabled bool `json:"hiringEnabled"`
	// MusicEnabled lets a jukebox in the office play and pause this
	// computer's music. Off until you turn it on: it presses a media key.
	MusicEnabled bool `json:"musicEnabled"`
	// Where the deck's sessions are told to other tools, if anywhere.
	Webhook Webhook `json:"webhook"`
	// ✨ Summary: whether a held desk offers a summary of its agent's
	// transcript, written by Claude with your own API key. Off until you turn
	// it on in setup and confirm what it sends where; and only with a key.
	SummaryOptIn bool `json:"summaryOptIn"`
	// The folder new hires start in, and the agent they are.
	HireFolder string `json:"hireFolder"`
	HireAgent  string `json:"hireAgent"`
	// The terminal hires open in (see hireOpeners), the first installed when
	// it's empty or not here, and whether the agent runs in a tmux session of
	// its own there. The Mac app's choices aren't this system's, and are
	// passed over as uninstalled.
	HireTerminal string `json:"hireTerminal"`
	HireInTmux   bool   `json:"hireInTmux"`
	// The roles `smolcorp run --role` starts, as the Mac app's office hires
	// them: those that come with the app until you change them, and your own.
	Roles []Role `json:"roles"`
	// The ids of the shipped roles this config has been given: a role that
	// comes with a later version is added to your roles once, at the end, and
	// one you've deleted isn't given back. Saved as every shipped role.
	RolesOffered      []string `json:"rolesOffered"`
	HasCompletedSetup bool     `json:"hasCompletedSetup"`
	// What the file held that this version doesn't read, as it was: saved
	// back with the rest, so a setting from another version, older or newer,
	// or the Mac app's, isn't lost at the next save.
	unknown map[string]json.RawMessage
}

// knownConfigKeys are the names this version reads, from its fields' tags.
func knownConfigKeys() map[string]bool {
	known := map[string]bool{}
	kind := reflect.TypeOf(Config{})
	for i := 0; i < kind.NumField(); i++ {
		if name, _, _ := strings.Cut(kind.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			known[name] = true
		}
	}
	return known
}

// The logos to choose from: the hire, a little person in a suit, or the six
// keys the deck started out with.
var logos = []string{"smolcorp", "keys"}

func loadConfig() *Config {
	config := &Config{Colours: map[string]string{}, Logo: "smolcorp", Layout: standardLayout(), ActiveProfile: defaultProfile, HireAgent: string(Claude), Webhook: standardWebhook()}
	if data, err := os.ReadFile(configFile); err == nil {
		// A field it can't read loses only itself. A file it can't read at all
		// starts from the defaults, but a copy is kept beside it first, so the
		// next save doesn't take your pins, layouts, profiles and roles with it.
		if err := json.Unmarshal(data, config); err != nil {
			_ = writePrivate(configFile+".broken", data)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(data, &fields) == nil {
			known := knownConfigKeys()
			for name, value := range fields {
				if !known[name] {
					if config.unknown == nil {
						config.unknown = map[string]json.RawMessage{}
					}
					config.unknown[name] = value
				}
			}
		}
	}
	// Layouts are checked again as they load: their buttons type into sessions.
	config.Layout = config.Layout.checked()
	for i := range config.Profiles {
		config.Profiles[i].Layout = config.Profiles[i].Layout.checked()
	}
	if config.Profiles == nil {
		config.Profiles = []Profile{}
	}
	// Checked as they load: their instructions go to agents.
	if config.Roles == nil {
		config.Roles = slices.Clone(shippedRoles)
	} else {
		// Shipped roles this file hasn't been given are added once, at the
		// end: a file from before rolesOffered was given the first five.
		offered := config.RolesOffered
		if offered == nil {
			offered = rolesOfferedBefore
		}
		roles := checkedRoles(config.Roles)
		for _, role := range shippedRoles {
			if !slices.Contains(offered, role.ID) && !slices.ContainsFunc(roles, func(r Role) bool { return r.ID == role.ID }) {
				roles = append(roles, role)
			}
		}
		config.Roles = checkedRoles(roles)
	}
	config.RolesOffered = shippedRoleIDs()
	// The mark the app had before it was Smolcorp is now the hire.
	if config.Logo == "sauzze" {
		config.Logo = "smolcorp"
	}
	if !contains(logos, config.Logo) {
		config.Logo = "smolcorp"
	}
	if config.Colours == nil {
		config.Colours = map[string]string{}
	}
	pins := make([]*string, mostKeys)
	copy(pins, config.Pins)
	config.Pins = pins
	return config
}

// save writes through a file beside the config and renames it, so a crash or
// a full disk leaves the last config whole rather than half of the new one.
func (c *Config) save() {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	if len(c.unknown) > 0 {
		var fields map[string]json.RawMessage
		if json.Unmarshal(data, &fields) == nil {
			for name, value := range c.unknown {
				if _, ours := fields[name]; !ours {
					fields[name] = value
				}
			}
			if merged, err := json.MarshalIndent(fields, "", "  "); err == nil {
				data = merged
			}
		}
	}
	_ = writePrivate(configFile, data)
}

// Pin pins folder to key index, taking it off any other key it was on.
func (c *Config) Pin(folder string, index int) {
	for i, pin := range c.Pins {
		if pin != nil && *pin == folder {
			c.Pins[i] = nil
		}
	}
	c.Pins[index] = &folder
	c.save()
}

func (c *Config) Unpin(index int) {
	c.Pins[index] = nil
	c.save()
}

// SwapPins puts first on key b and second on key a, each taken off any other
// key it was on: two desks swapped in the office. A nil project is an empty
// desk, whose key is left free.
func (c *Config) SwapPins(first *string, a int, second *string, b int) {
	for i, pin := range c.Pins {
		if pin != nil && ((first != nil && *pin == *first) || (second != nil && *pin == *second)) {
			c.Pins[i] = nil
		}
	}
	c.Pins[b] = first
	c.Pins[a] = second
	c.save()
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// existingFolder is path as a folder that's there, with ~ for home.
func existingFolder(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		path = filepath.Join(homeDir, path[1:])
	}
	if !filepath.IsAbs(path) {
		return "", false
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", false
	}
	return filepath.Clean(path), true
}
