package desktop

import (
	"embed"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"
)

//go:embed tray-smolcorp.png tray-keys.png
var trayIcons embed.FS

// RunApp is the app: the deck's server, with an icon in the tray, the
// counterpart of the Mac app's menu bar item. It returns when you quit.
func RunApp() int {
	// First, before any key exists in this process: its memory kept from your
	// other programs (memory.go). If that fails, nothing starts.
	if err := protectMemory(); err != nil {
		return refuseUnprotected(err)
	}
	// Opened from Downloads or an unzipped folder, it installs itself and
	// starts from there (Windows), before anything names this copy.
	restarted := takeStartNotes()
	if !restarted && installAndRelaunch() {
		return 0
	}
	// Before anything is read or written: a second instance must not touch the
	// first one's keys. One started to take another's place waits for it.
	acquire := acquireInstance
	if restarted {
		acquire = waitForInstance
	}
	if running, err := acquire(); running {
		return handOverToRunning(false)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: no instance lock (%v), going on without\n", err)
	}
	recordPid()
	defer forgetPid()
	deck := NewDeck()
	refreshHooks()
	deck.mu.Lock()
	// Nothing to read for a test copy on this computer only: the firewall doesn't filter loopback (RTCH N2).
	if scope := effectiveScope(deck.config); scope != listenThisComputer {
		lookAtFirewall(scope)
	}
	deck.mu.Unlock()
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
		printSetupLink(os.Stdout, deck)
		stopPortableOnSignal(deck)
		return reportFailure(<-failed)
	}
	return runWithTray(deck, failed, firstRun)
}

// systrayRun starts the tray. A variable, for the tests.
var systrayRun = systray.Run

// runWithTray shows the tray and returns when you quit. With no session bus
// there is nothing to register it on, and systray would have godbus start
// dbus-launch, whose daemon outlives Smolcorp (SIERRA 1.23.0 m-8): the
// setup link is printed instead, as on a desktop with no tray.
func runWithTray(deck *Deck, failed chan error, firstRun bool) int {
	if sessionBusMissing() {
		withoutTray(deck, os.Stdout, firstRun)
		stopPortableOnSignal(deck)
		return reportFailure(<-failed)
	}
	// The tray starts even on a desktop with nowhere to show it, for a panel
	// that comes later; meanwhile, setup opens, and the ways back in are printed.
	go lookForTrayHost(deck, os.Stdout, firstRun)
	// smolcorp quit asks with SIGTERM, which must end this copy as the tray's Quit does.
	stopPortableOnSignal(deck)
	code := 0
	systrayRun(func() { newTray(deck, failed, &code) }, nil)
	return code
}

// RunServer is the server alone, for a computer without a desktop to show a
// tray on, such as a Linux machine you reach over SSH.
func RunServer() int {
	// First, as the app's: its memory kept from your other programs, or nothing starts.
	if err := protectMemory(); err != nil {
		return refuseUnprotected(err)
	}
	// Started to take another's place, as for the home Wi-Fi: it waits for that one.
	acquire := acquireInstance
	if takeStartNotes() {
		acquire = waitForInstance
	}
	if running, err := acquire(); running {
		return handOverToRunning(true)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: no instance lock (%v), going on without\n", err)
	}
	recordPid()
	defer forgetPid()
	deck := NewDeck()
	refreshHooks()
	watchOpenCode()
	printSetupLink(os.Stdout, deck)
	stopPortableOnSignal(deck)
	return reportFailure(Serve(deck))
}

// stopPortableOnSignal: stopped from its terminal, the portable office stops
// too, as on quit, and the viewer is asked to forget it.
func stopPortableOnSignal(deck *Deck) {
	signalOnce.Do(func() { stopPortableOnSignalNow(deck) })
}

// signalOnce: the handler is set once, whichever way in comes first.
var signalOnce sync.Once

func stopPortableOnSignalNow(deck *Deck) {
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		deck.portable.Stop(viewerQuitWait)
		deck.waitForHires(hireStopWait)
		// The GM's token's last change, kept for the next run (gmtokenstore.go).
		deck.gmKept.flush(gmTokenFlushWait)
		forgetPid()
		os.Exit(1)
	}()
}

// noTrayNewLink is the one way to a new setup link with no tray: no command
// makes one, and a second start only raises the office, so Smolcorp is
// stopped and started again, which prints a new link.
const noTrayNewLink = "run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again"

// printSetupLink says where setup is, with no tray to open it from: on this
// computer, or through a tunnel to its port, as ssh -L 7710:localhost:7710.
// The link works once, for a minute off a Mac (setupTokenLifetime), and never carries the setup key.
func printSetupLink(out io.Writer, deck *Deck) {
	fmt.Fprintf(out, "Smolcorp's setup: %s\nIt works once, for %s. For a new one, choose Open Setup… from the tray icon; with no tray, "+noTrayNewLink+".\n",
		deck.setupLink(""), demoWaitWords(setupTokenLifetime))
}

// trayHostWait is how long a desktop has to start the panel the tray's icon
// sits in, as one just signed in to may still be doing: a variable, for the tests.
var trayHostWait = 10 * time.Second

// lookForTrayHost waits a while for somewhere to show the tray's icon, and
// with none, says so, prints the ways in, and opens setup, unless it was
// open already, as on the first run.
func lookForTrayHost(deck *Deck, out io.Writer, setupOpen bool) {
	for deadline := time.Now().Add(trayHostWait); !hasTrayHost(); time.Sleep(time.Second) {
		if time.Now().After(deadline) {
			// With no Quit Smolcorp in a menu, it's stopped from its terminal, or by the system.
			stopPortableOnSignal(deck)
			withoutTray(deck, out, setupOpen)
			return
		}
	}
}

// The tray's "used first" warning and its clear, off a Mac (setuplink.go).
const (
	usedFirstMenuTitle = "⚠ Another program used your last Setup link first…"
	usedFirstSeenTitle = "    I've seen it: clear this warning"
)

// noTrayMessage is what Smolcorp prints on a desktop with no tray.
const noTrayMessage = `This desktop has no tray for Smolcorp's icon (on GNOME, the AppIndicator extension adds one).
Smolcorp is running, and its setup opens in your browser. For a new setup link, run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again.
To open the office, start Smolcorp again from your apps, or run: smolcorp
`

func withoutTray(deck *Deck, out io.Writer, setupOpen bool) {
	fmt.Fprint(out, noTrayMessage)
	printSetupLink(out, deck)
	if !setupOpen {
		deck.openSetup("")
	}
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
	deck                              *Deck
	count, setup, pair, unpair, login *systray.MenuItem
	office, onTop                     *systray.MenuItem
	// The portable office: how it is, the switch, and where to set it up.
	portableStatus, portable, viewerSetup *systray.MenuItem
	quit                                  *systray.MenuItem
	logo                                  string
	code                                  *int // what RunApp returns
	// "Show Me Around Again": the first-run guide's arrows, from the start.
	guide *systray.MenuItem
	// A Setup link used first by another program (setuplink.go): the warning, which opens Setup to see what it
	// changed, and the owner's "I've seen it", the one place it clears off a Mac.
	usedFirst, usedFirstSeen *systray.MenuItem
}

func newTray(deck *Deck, failed <-chan error, code *int) {
	t := &tray{deck: deck, code: code}
	// Restarting, the tray quits, and RunApp returns.
	trayStarted.Store(true)
	leave = systray.Quit
	systray.SetTooltip("Smolcorp")
	t.count = systray.AddMenuItem("", "")
	t.count.Disable()
	t.usedFirst = systray.AddMenuItem(usedFirstMenuTitle, "Setup lists what that program changed")
	t.usedFirstSeen = systray.AddMenuItem(usedFirstSeenTitle, "Takes the warning away, here and in Setup")
	t.usedFirst.Hide()
	t.usedFirstSeen.Hide()
	systray.AddSeparator()
	t.office = systray.AddMenuItem("Show Office", "")
	if hasNativeWindow {
		deck.mu.Lock()
		onTop := deck.config.AlwaysOnTop
		deck.mu.Unlock()
		t.onTop = systray.AddMenuItemCheckbox("Always on Top", "", onTop)
	} else {
		t.onTop = &systray.MenuItem{} // no window to keep on top: a click channel nobody sends on
	}
	systray.AddSeparator()
	t.setup = systray.AddMenuItem("Open Setup…", "")
	t.pair = systray.AddMenuItem("Pair a Phone…", "")
	t.unpair = systray.AddMenuItem("Unpair All Phones…", "")
	t.guide = systray.AddMenuItem("Show Me Around Again", "Setup's arrows to the folder for hires and your first General Manager")
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
	state := t.deck.menuState()
	if state.Sessions == 1 {
		t.count.SetTitle("1 agent session")
	} else {
		t.count.SetTitle(fmt.Sprintf("%d agent sessions", state.Sessions))
	}
	title := ""
	if state.Waiting > 0 {
		title = fmt.Sprint(state.Waiting)
	}
	systray.SetTitle(title)
	systray.SetTooltip(strings.TrimSpace("Smolcorp " + map[bool]string{true: "· " + title + " waiting on you"}[state.Waiting > 0]))
	if state.UsedFirst {
		t.usedFirst.Show()
		t.usedFirstSeen.Show()
	} else {
		t.usedFirst.Hide()
		t.usedFirstSeen.Hide()
	}
	if state.Phones > 0 {
		t.unpair.Show()
	} else {
		t.unpair.Hide()
	}
	t.updatePortable(state.Portable)
	// Set from setup too: the check box follows.
	if hasNativeWindow && state.OnTop != t.onTop.Checked() {
		if state.OnTop {
			t.onTop.Check()
		} else {
			t.onTop.Uncheck()
		}
	}
	if state.Logo != t.logo {
		if icon, err := trayIcons.ReadFile("tray-" + state.Logo + ".png"); err == nil {
			systray.SetIcon(trayIcon(icon))
			t.logo = state.Logo
		}
	}
}

// updatePortable shows the portable office as the Mac's menu does: on, with
// how it's going, and its switch; or off, with what it needs.
func (t *tray) updatePortable(portable portableMenu) {
	on, setUp := portable.On, portable.SetUp
	switch {
	case on:
		t.portableStatus.SetTitle("Portable office: ON · " + portable.Status)
		t.portableStatus.Show()
		t.portable.SetTitle("Stop portable office")
	case setUp && portable.Problem != "":
		t.portableStatus.SetTitle(portable.Problem)
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
		case <-t.office.ClickedCh:
			t.deck.openOffice()
		case <-t.onTop.ClickedCh:
			on := !t.onTop.Checked()
			t.deck.setAlwaysOnTop(on)
			if on {
				t.onTop.Check()
			} else {
				t.onTop.Uncheck()
			}
		case <-t.setup.ClickedCh:
			t.deck.openSetup("")
		case <-t.usedFirst.ClickedCh:
			t.deck.openSetup("")
		case <-t.usedFirstSeen.ClickedCh:
			t.deck.clearUsedFirst()
		case <-t.pair.ClickedCh:
			// The setup page has a code ready when it opens on #pair.
			t.deck.openSetup("pair")
		case <-t.unpair.ClickedCh:
			// The setup page asks you to confirm, then unpairs every phone.
			t.deck.openSetup("unpair")
		case <-t.portable.ClickedCh:
			if t.deck.portable.IsOn() {
				t.deck.portable.Stop(0)
			} else {
				t.deck.portable.Start()
			}
		case <-t.guide.ClickedCh:
			_ = t.deck.showGuideAgain()
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
			t.deck.waitForHires(hireStopWait)
			t.deck.gmKept.flush(gmTokenFlushWait)
			systray.Quit()
			return
		case err := <-failed:
			// A server that can't listen is no use, and an icon with nothing behind
			// it is worse than none: say why, and leave.
			message := fmt.Sprintf("Smolcorp can't start its server on port %d: %v\n\nAnother program, or an older Smolcorp, is probably using that port.", Port, err)
			fmt.Fprintln(os.Stderr, message)
			notifyUser(message)
			*t.code = 1
			systray.Quit()
			return
		}
	}
}
