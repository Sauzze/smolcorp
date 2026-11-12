package desktop

// menuState is what Smolcorp's menu shows of the deck, read at once: the tray's
// on Windows and Linux (app.go), and the Mac's menu bar, which the core sends it
// to whenever it changes (core_darwin.go). The sessions and how many wait on
// you, the phones paired (not the office window's own device), the logo, the
// office's window kept on top, and the portable office.
type menuState struct {
	Sessions int          `json:"sessions"`
	Waiting  int          `json:"waiting"`
	Phones   int          `json:"phones"`
	Logo     string       `json:"logo"`
	OnTop    bool         `json:"onTop"`
	Portable portableMenu `json:"portable"`
}

// portableMenu is the portable office as the menu shows it: on, with how it's
// going (status); or off, set up or not, with a problem if it has one.
type portableMenu struct {
	On      bool   `json:"on"`
	SetUp   bool   `json:"setUp"`
	Status  string `json:"status"`
	Problem string `json:"problem"`
}

func (d *Deck) menuState() menuState {
	d.mu.Lock()
	all := d.sessions.All()
	state := menuState{Sessions: len(all), Phones: phonesOnly(d.access.Phones), Logo: d.config.Logo, OnTop: d.config.AlwaysOnTop}
	for _, s := range all {
		if s.State == Asking {
			state.Waiting++
		}
	}
	d.mu.Unlock()
	portable := d.portable
	state.Portable.On, state.Portable.SetUp = portable.IsOn(), portable.IsSetUp()
	state.Portable.Status, state.Portable.Problem = portable.Status()
	return state
}
