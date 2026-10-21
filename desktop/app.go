package desktop

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"fyne.io/systray"
)

//go:embed tray-smolcorp.png tray-keys.png
var trayIcons embed.FS

// RunApp is the app: the deck's server, with an icon in the tray, the
// counterpart of the Mac app's menu bar item. It returns when you quit.
func RunApp() int {
	deck := NewDeck()
	refreshHooks()
	watchOpenCode()
	moveOpenAtLogin()
	failed := make(chan error, 1)
	go func() { failed <- Serve(deck) }()

	deck.mu.Lock()
	firstRun := !deck.config.HasCompletedSetup
	deck.mu.Unlock()
	if firstRun {
		deck.openSetup("")
	}
	if !hasDesktop() {
		printSetupLink(deck)
		stopPortableOnSignal(deck)
		return reportFailure(<-failed)
	}
	systray.Run(func() { newTray(deck, failed) }, nil)
	return 0
}

// RunServer is the server alone, for a computer without a desktop to show a
// tray on, such as a Linux machine you reach over SSH.
func RunServer() int {
	deck := NewDeck()
	refreshHooks()
	watchOpenCode()
	printSetupLink(deck)
	stopPortableOnSignal(deck)
	return reportFailure(Serve(deck))
}

// stopPortableOnSignal: stopped from its terminal, the portable office stops
// too, as on quit, and the viewer is asked to forget it.
func stopPortableOnSignal(deck *Deck) {
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		deck.portable.Stop(viewerQuitWait)
		os.Exit(1)
	}()
}

// printSetupLink says where setup is, with no tray to open it from: on this
// computer, or through a tunnel to its port, as ssh -L 7710:localhost:7710.
// Printed, not passed on a command line, it can carry the key itself.
func printSetupLink(deck *Deck) {
	fmt.Printf("Smolcorp's setup: %s\nIt works until Smolcorp restarts. Its key is also in %s.\n", deck.setupLink(""), setupKeyFile)
}

func reportFailure(err error) int {
	fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
	return 1
}

// hasDesktop: a tray needs a graphical session to sit in.
func hasDesktop() bool {
	return runtime.GOOS != "linux" || os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

type tray struct {
	deck                                      *Deck
	count, setup, pair, unpair, office, login *systray.MenuItem
	// The portable office: how it is, the switch, and where to set it up.
	portableStatus, portable, viewerSetup *systray.MenuItem
	quit                                  *systray.MenuItem
	logo                                  string
}

func newTray(deck *Deck, failed <-chan error) {
	t := &tray{deck: deck}
	systray.SetTooltip("Smolcorp")
	t.count = systray.AddMenuItem("", "")
	t.count.Disable()
	systray.AddSeparator()
	t.setup = systray.AddMenuItem("Open Setup…", "")
	t.pair = systray.AddMenuItem("Pair a Phone…", "")
	t.unpair = systray.AddMenuItem("Unpair All Phones…", "")
	systray.AddSeparator()
	t.office = systray.AddMenuItem("Show Office", "The office in your browser, as on a phone")
	systray.AddSeparator()
	t.portableStatus = systray.AddMenuItem("", "")
	t.portableStatus.Disable()
	t.portable = systray.AddMenuItem("Going away: launch portable office", "The office on your phone from anywhere, view only")
	t.viewerSetup = systray.AddMenuItem("    Set up the viewer on the setup page", "")
	systray.AddSeparator()
	t.login = systray.AddMenuItemCheckbox("Open at Login", "", isOpenAtLogin())
	systray.AddSeparator()
	t.quit = systray.AddMenuItem("Quit Smolcorp", "")

	deck.mu.Lock()
	deck.OnChange = t.update
	deck.mu.Unlock()
	deck.portable.mu.Lock()
	deck.portable.onChange = t.update
	deck.portable.mu.Unlock()
	t.update()
	go t.listen(failed)
}

// update shows the sessions, and how many are waiting on you, the way the
// Mac's menu bar title does.
func (t *tray) update() {
	d := t.deck
	d.mu.Lock()
	all := d.sessions.All()
	waiting := 0
	for _, s := range all {
		if s.State == Asking {
			waiting++
		}
	}
	phones := len(d.access.Phones.All)
	logo := d.config.Logo
	d.mu.Unlock()

	if len(all) == 1 {
		t.count.SetTitle("1 agent session")
	} else {
		t.count.SetTitle(fmt.Sprintf("%d agent sessions", len(all)))
	}
	title := ""
	if waiting > 0 {
		title = fmt.Sprint(waiting)
	}
	systray.SetTitle(title)
	systray.SetTooltip(strings.TrimSpace("Smolcorp " + map[bool]string{true: "· " + title + " waiting on you"}[waiting > 0]))
	if phones > 0 {
		t.unpair.Show()
	} else {
		t.unpair.Hide()
	}
	t.updatePortable()
	if logo != t.logo {
		if icon, err := trayIcons.ReadFile("tray-" + logo + ".png"); err == nil {
			systray.SetIcon(trayIcon(icon))
			t.logo = logo
		}
	}
}

// updatePortable shows the portable office as the Mac's menu does: on, with
// how it's going, and its switch; or off, with what it needs.
func (t *tray) updatePortable() {
	portable := t.deck.portable
	on, setUp := portable.IsOn(), portable.IsSetUp()
	status, problem := portable.Status()
	switch {
	case on:
		t.portableStatus.SetTitle("Portable office: ON · " + status)
		t.portableStatus.Show()
		t.portable.SetTitle("Stop portable office")
	case setUp && problem != "":
		t.portableStatus.SetTitle(problem)
		t.portableStatus.Show()
	default:
		t.portableStatus.Hide()
	}
	if !on {
		t.portable.SetTitle("Going away: launch portable office")
	}
	if on || setUp {
		t.portable.Enable()
		t.viewerSetup.Hide()
	} else {
		t.portable.Disable()
		t.viewerSetup.Show()
	}
}

func (t *tray) listen(failed <-chan error) {
	for {
		select {
		case <-t.setup.ClickedCh:
			t.deck.openSetup("")
		case <-t.pair.ClickedCh:
			// The setup page has a code ready when it opens on #pair.
			t.deck.openSetup("pair")
		case <-t.unpair.ClickedCh:
			// The setup page asks you to confirm, then unpairs every phone.
			t.deck.openSetup("unpair")
		case <-t.office.ClickedCh:
			t.deck.openOffice()
		case <-t.portable.ClickedCh:
			if t.deck.portable.IsOn() {
				t.deck.portable.Stop(0)
			} else {
				t.deck.portable.Start()
			}
		case <-t.viewerSetup.ClickedCh:
			t.deck.openSetup("viewer")
		case <-t.login.ClickedCh:
			setOpenAtLogin(!isOpenAtLogin())
			if isOpenAtLogin() {
				t.login.Check()
			} else {
				t.login.Uncheck()
			}
		case <-t.quit.ClickedCh:
			// The portable office stops on quit, and the viewer is asked to forget it, for up to 5 s.
			t.deck.portable.Stop(viewerQuitWait)
			systray.Quit()
			return
		case <-failed:
			t.count.SetTitle(fmt.Sprintf("Port %d is in use. Is Smolcorp already running?", Port))
		}
	}
}
