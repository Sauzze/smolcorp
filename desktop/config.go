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
	// MaxConfirm has the office ask before max effort: the red button and a
	// chip's max bring up a question first. On until you turn it off, as max
	// uses much more of your usage; off, one tap sets it, as it always did.
	MaxConfirm bool `json:"maxConfirm"`
	// ManagerEnabled is the front desk (manager) switch, the Mac app's
	// managerEnabled: off until you turn it on. See manager.go.
	ManagerEnabled bool `json:"managerEnabled"`
	// HooksDisconnected: you disconnected the agents on purpose, so smolcorp
	// setup leaves them be. Connecting, on the setup page, clears it.
	HooksDisconnected bool `json:"hooksDisconnected,omitempty"`
	// Where the deck's sessions are told to other tools, if anywhere.
	Webhook Webhook `json:"webhook"`
	// ✨ Summary: whether a held desk offers a summary of its agent's
	// transcript, written by Claude with your own API key. Off until you turn
	// it on in setup and confirm what it sends where; and only with a key.
	SummaryOptIn bool `json:"summaryOptIn"`
	// SummaryProvider is what makes summaries: a coding agent's id ("claude",
	// "codex", …) or "apikey"; "" until one is picked or summaries are turned
	// on (see chosenSummaryProvider).
	SummaryProvider string `json:"summaryProvider,omitempty"`
	// The portable office's viewer: its address and options. Never its id,
	// key or write secret, which are in the system's secret store.
	Viewer ViewerSettings `json:"viewer"`
	// The folder new hires start in, and the agent they are.
	HireFolder string `json:"hireFolder"`
	HireAgent  string `json:"hireAgent"`
	// The terminal hires open in (see hireOpeners), the first installed when
	// it's empty or not here, and whether the agent runs in a tmux session of
	// its own there. The Mac app's choices aren't this system's, and are
	// passed over as uninstalled.
	HireTerminal string `json:"hireTerminal"`
	HireInTmux   bool   `json:"hireInTmux"`
	// Beta, Windows only: a role's Claude hire made by the hire engine in a window run by
	// the launcher (hirelauncher.go), as the tmux engine does on Linux. Off unless switched on.
	HireManagedWindows bool `json:"hireManagedWindows"`
	// The roles `smolcorp run --role` starts, as the Mac app's office hires
	// them: those that come with the app until you change them, and your own.
	Roles []Role `json:"roles"`
	// The ids of the shipped roles this config has been given: a role that
	// comes with a later version is added to your roles once, at the end, and
	// one you've deleted isn't given back. Saved as every shipped role.
	RolesOffered      []string `json:"rolesOffered"`
	HasCompletedSetup bool     `json:"hasCompletedSetup"`
	// Window is where the office window was left, and whether it stays above the others (Windows).
	Window      *WindowState `json:"window"`
	AlwaysOnTop bool         `json:"alwaysOnTop"`
	// Listen is who can reach the server: "tailscale" (this computer and the
	// Tailscale address only; the default on Windows and Linux) or "all" (every
	// interface; the default elsewhere). Read at start. See listen.go.
	Listen string `json:"listen,omitempty"`
	// Bots that work in a cloud, not in a terminal here, each at a desk of
	// its own (see CloudBot). None until you set some; one that can't be
	// read loses only itself.
	CloudBots cloudBotList `json:"cloudBots"`
	// Whether the Mac app gives the Grok Bot app's bots desks by themselves,
	// unset for on; and the app's ids of those you hid. Kept as set: nothing
	// here looks in that app, which is the Mac's alone.
	BotsAutoDetect *bool    `json:"botsAutoDetect,omitempty"`
	HiddenBots     []string `json:"hiddenBots,omitempty"`
	// The floor new bots go to, 0 for the ground floor up to 5 for 6F, when
	// no floor rule names one for them (see CloudBot.Floor); chosen in setup.
	// Until it is, the floor your bots are on (newBotsFloor).
	BotFloor int `json:"botFloor"`
	// Whether the file had BotFloor: one from before it has it worked out
	// again once the bots that were upstairs are settled (see settleBots).
	botFloorKept bool
	// The building's six floors, the ground floor first: each one's name, and
	// the style of those above it (see Floor).
	Floors floorList `json:"floors"`
	// The folders whose sessions sit on a floor of their own (see FloorRule).
	FloorRules floorRuleList `json:"floorRules"`
	// The GM's mail (mail_outbox.go): the longest message in bytes, the most one
	// sender may send in an hour, and the hours a message waits for a busy hire
	// before it is left. Each at least 1.
	MailMaxBytes  int     `json:"mailMaxBytes"`
	MailPerHour   int     `json:"mailPerHour"`
	MailWaitHours float64 `json:"mailWaitHours"`
	// Whether the app keeps its own watches on the hires (watchers_builtin.go),
	// while the manager is on; unset for on. No page setting: the file only.
	BuiltinWatches bool `json:"builtinWatches"`
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
	config := &Config{Colours: map[string]string{}, Logo: "smolcorp", Layout: standardLayout(), ActiveProfile: defaultProfile, HireAgent: string(Claude), Webhook: standardWebhook(),
		MaxConfirm: true, Viewer: standardViewerSettings(),
		MailMaxBytes: mailDefaultMaxBytes, MailPerHour: mailDefaultPerHour, MailWaitHours: mailDefaultWaitHours, BuiltinWatches: true}
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
			var floor int
			if raw, ok := fields["botFloor"]; ok && json.Unmarshal(raw, &floor) == nil && floor >= 0 && floor < floorCount {
				config.BotFloor, config.botFloorKept = floor, true
			}
		}
	}
	// At least 1 each, as on the Mac (Config.swift).
	config.MailMaxBytes = max(1, config.MailMaxBytes)
	config.MailPerHour = max(1, config.MailPerHour)
	config.MailWaitHours = max(1, config.MailWaitHours)
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
		config.Roles = checkedRoles(slices.Clone(shippedRoles))
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
	// Checked as they load: their names are drawn in the office.
	config.CloudBots = checkedCloudBots(config.CloudBots)
	// A file from before it: where the bots are, and again once those that were upstairs are settled.
	if !config.botFloorKept {
		config.BotFloor = newBotsFloor(config.CloudBots)
	}
	// Checked as they load, for the same reason.
	config.Floors = checkedFloors(config.Floors)
	config.FloorRules = checkedFloorRules(config.FloorRules)
	// Only the regular logo since setup lost the choice: an older pick is read but not kept.
	config.Logo = "smolcorp"
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

// settleBots puts the bots still "upstairs", from before bots could sit on
// any floor, each on the floor it sits on, by seats, the desks saved across a
// restart (see settledCloudBots); and with a file from before BotFloor, new
// bots then go where the bots are. Whether anything changed, to be saved.
// Once settled, there's nothing to do again. The Mac app's Config.settleBots
// does the same.
func (c *Config) settleBots(seats map[string]Seat) bool {
	if !slices.ContainsFunc(c.CloudBots, func(b CloudBot) bool { return b.Floor == cloudUpstairs }) {
		return false
	}
	c.CloudBots = settledCloudBots(c.CloudBots, seats)
	if !c.botFloorKept {
		c.BotFloor = newBotsFloor(c.CloudBots)
	}
	return true
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
