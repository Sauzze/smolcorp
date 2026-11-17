//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXTABLINK (1.26.0): a desk never gets the core to drive another desk's tmux server, or its own unchecked, through a
// link. The core takes a desk's tab only when no part of its socket's path is a link, and keeps it as it checked it;
// it drives no tab that goes through a link onto a desk's socket, and never hands one to the Mac app. Everything here
// is a stand-in: two desks of the test's own, with keys of their own, each on a tmux server the test starts in its own
// socket folder, whose pane writes what it is given to a file. Runs on a Mac here, and on Linux in CI (ubuntu).

// tabLinkDesk is a stand-in desk: its id, socket, hook key, its server's pid and its pane, and the file its pane writes
// what reaches it to.
type tabLinkDesk struct {
	id, socket, key, pane, out string
	server                     int
}

// tabLinkOffice is a deck with desks ab12 and cd34 in the registry, each started as a sandboxed desk is: its own
// folder in the socket folder (deskSocketParent, the test's), a server there with its pid recorded, a key of its own.
// root is the test's folder, its links resolved; the servers are ended by their pids, and nothing else.
func tabLinkOffice(t *testing.T) (d *Deck, root string, desks map[string]*tabLinkDesk) {
	t.Helper()
	tmux := needTmux(t)
	program, err := filepath.EvalSymlinks(tmux)
	if err != nil {
		t.Fatal(err)
	}
	d = newTestDeck(t)
	// Short: a socket's path has a length limit. The system's temporary folder, its links resolved.
	dir, err := os.MkdirTemp("", "tl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	root = physical(dir)
	parent := filepath.Join(root, "p")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	saved := deskSocketParent
	deskSocketParent = func() string { return parent }
	t.Cleanup(func() { deskSocketParent = saved })
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX=") && !strings.HasPrefix(kv, "TMUX_PANE=") {
			env = append(env, kv)
		}
	}
	desks = map[string]*tabLinkDesk{}
	var hires []Hire
	for _, id := range []string{"ab12", "cd34"} {
		desk := &tabLinkDesk{id: id, socket: filepath.Join(parent, id, deskSocketFile), out: filepath.Join(root, id+".out")}
		if err := os.Mkdir(filepath.Dir(desk.socket), 0o700); err != nil {
			t.Fatal(err)
		}
		tmuxOn := func(args ...string) string {
			cmd := exec.Command(tmux, append([]string{"-S", desk.socket, "-f", os.DevNull}, args...)...)
			cmd.Env = env
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("tmux %v: %v", args, err)
			}
			return strings.TrimSpace(string(output))
		}
		tmuxOn("new-session", "-d", "-x", "80", "-y", "24", "sh", "-c",
			"stty raw -echo; : > "+desk.out+".ready; exec cat > "+desk.out)
		if desk.server, err = strconv.Atoi(tmuxOn("display-message", "-p", "#{pid}")); err != nil || desk.server <= 1 {
			t.Fatalf("%s's server: %v", id, err)
		}
		server := desk.server
		t.Cleanup(func() { _ = syscall.Kill(server, syscall.SIGTERM) })
		desk.pane = tmuxOn("display-message", "-p", "#{pane_id}")
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(desk.out + ".ready"); err == nil {
				break
			}
		}
		desks[id] = desk
		hires = append(hires, Hire{ID: id, Name: "Desk " + id, Role: "janitor", Folder: "/proj", Repo: "/proj",
			Worktree: filepath.Join(root, "wt-"+id), TmuxSocket: desk.socket, TmuxProgram: program,
			TmuxServer: desk.server, State: "working"})
	}
	if err := updateRegistry(func(f *registryFile) { f.hires = hires }); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ab12", "cd34"} {
		if desks[id].key, err = newHookKey(registryPath(), id); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, "wt-"+id), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return d, root, desks
}

// typeIntoCard types words into session id's card, as the deck types into any desk: its tab, as the deck keeps it.
func typeIntoCard(d *Deck, id, words string) error {
	d.mu.Lock()
	s := d.sessions.Get(id)
	if s == nil {
		d.mu.Unlock()
		return os.ErrNotExist
	}
	name := s.Tab
	tab := d.tab(name)
	d.mu.Unlock()
	return d.typeInto(name, tab, words)
}

// reached is whether words reached the stand-in desk's pane within wait.
func (desk *tabLinkDesk) reached(words string, wait time.Duration) bool {
	for deadline := time.Now().Add(wait); ; time.Sleep(20 * time.Millisecond) {
		if got, _ := os.ReadFile(desk.out); strings.Contains(string(got), words) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}

// relink points link at target, in place of what it pointed at.
func relink(t *testing.T, link, target string) {
	t.Helper()
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// linkAttempt is the finding's steps, as desk ab12 takes them: it reports as s-ab12 on its own tab, makes a link of
// its own (wt-ab12/lnk) to its own folder and files an event with its own key on the tab through it, then points the
// link at cd34's folder. It gives ab12's own tab, the tab through the link, and what that event was answered.
func linkAttempt(t *testing.T, d *Deck, root string, ab, cd *tabLinkDesk) (own, linked string, code int) {
	t.Helper()
	work := filepath.Join(root, "wt-ab12")
	own = "tmux:" + ab.pane + "@" + ab.socket
	if got := hookAs(t, d, ab.key, own, "SessionStart", "s-ab12", work); got != http.StatusNoContent {
		t.Fatalf("ab12 on its own tab: %d", got)
	}
	link := filepath.Join(work, "lnk")
	relink(t, link, filepath.Dir(ab.socket))
	linked = "tmux:" + ab.pane + "@" + filepath.Join(link, deskSocketFile)
	code = hookAs(t, d, ab.key, linked, "UserPromptSubmit", "s-ab12", work)
	relink(t, link, filepath.Dir(cd.socket))
	return own, linked, code
}

// The finding: desk ab12 files an event with its own key on a tab through a link of its own to its own folder, then
// points the link at cd34's. Typing into ab12's card never reaches cd34's server: the event is refused, and ab12's
// card stays on its own socket, driven there with its recorded server.
func TestADeskCantHaveTheCoreDriveAnotherDesksServerThroughALink(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab, cd := desks["ab12"], desks["cd34"]
	own, linked, code := linkAttempt(t, d, root, ab, cd)

	err := typeIntoCard(d, "s-ab12", "tablink-probe-1")
	if cd.reached("tablink-probe-1", time.Second) {
		t.Errorf("the keys typed into ab12's card reached cd34's server (the event on %s was answered %d)", linked, code)
	}
	if code != http.StatusForbidden {
		t.Errorf("ab12's event on a tab through a link: %d, not refused", code)
	}
	if s := d.sessions.Get("s-ab12"); s == nil || s.Tab != own {
		t.Errorf("ab12's card's tab: %+v, not its own socket", s)
	}
	if err != nil || !ab.reached("tablink-probe-1", 3*time.Second) {
		t.Errorf("typing into ab12's card on its own socket: %v, reached it %v", err, ab.reached("tablink-probe-1", 0))
	}
}

// N1's class: a link to the desk's own folder. Its event there is refused, so its card is never driven "unchecked"
// (a server the desk put at its own socket would answer with no pid check): only with the server its row records.
func TestADeskCantHaveItsOwnServerDrivenUncheckedThroughALink(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab := desks["ab12"]
	work := filepath.Join(root, "wt-ab12")
	link := filepath.Join(work, "lnk")
	relink(t, link, filepath.Dir(ab.socket))
	linked := "tmux:" + ab.pane + "@" + filepath.Join(link, deskSocketFile)
	code := hookAs(t, d, ab.key, linked, "SessionStart", "s-ab12", work)
	if code != http.StatusForbidden {
		t.Errorf("ab12's event on a tab through a link to its own folder: %d, not refused", code)
	}
	// Refused, it made no card: none to be driven "unchecked" there. (The guard of ab12's own card is checked by
	// TestAfterALinkIsRepointedADesksCardIsDrivenOnItsOwnServer.)
	if s := d.sessions.Get("s-ab12"); s != nil {
		_, _, socket := splitTab(s.Tab)
		_, guard := tmuxFor(socket)
		t.Errorf("ab12's refused event made a card on %s, driven with guard %q (its recorded server is %d)", s.Tab, guard, ab.server)
	}
	// A link anywhere on the way is refused too: at the socket folder, or at the socket itself.
	parentLink := filepath.Join(work, "parent")
	relink(t, parentLink, filepath.Dir(filepath.Dir(ab.socket)))
	if got := hookAs(t, d, ab.key, "tmux:"+ab.pane+"@"+filepath.Join(parentLink, "ab12", deskSocketFile), "SessionStart", "s-ab12-p", work); got != http.StatusForbidden {
		t.Errorf("ab12's event through a link to the socket folder: %d", got)
	}
	socketLink := filepath.Join(work, "s")
	relink(t, socketLink, ab.socket)
	if got := hookAs(t, d, ab.key, "tmux:"+ab.pane+"@"+socketLink, "SessionStart", "s-ab12-s", work); got != http.StatusForbidden {
		t.Errorf("ab12's event on a link to its socket: %d", got)
	}
	// The owner's key reaches no desk through a link either, at the folder or at the socket.
	owner := d.access.HookKey
	if got := hookAs(t, d, owner, linked, "SessionStart", "s-owner-l", "/proj"); got != http.StatusForbidden {
		t.Errorf("the owner's key through a link to ab12's folder: %d", got)
	}
	if got := hookAs(t, d, owner, "tmux:"+ab.pane+"@"+socketLink, "SessionStart", "s-owner-s", "/proj"); got != http.StatusForbidden {
		t.Errorf("the owner's key on a link to ab12's socket: %d", got)
	}
	// Its own socket, spelt with no link, as the core started it: taken, and kept as it was checked.
	unclean := "tmux:" + ab.pane + "@" + filepath.Dir(ab.socket) + "/./" + deskSocketFile
	if got := hookAs(t, d, ab.key, unclean, "SessionStart", "s-ab12-own", work); got != http.StatusNoContent {
		t.Fatalf("ab12 on its own socket: %d", got)
	}
	if s := d.sessions.Get("s-ab12-own"); s == nil || s.Tab != "tmux:"+ab.pane+"@"+ab.socket {
		t.Errorf("ab12's own tab is kept as %+v, not as it was checked", s)
	}
}

// A session that is on a tab through a link already (kept from before this check, say): the core drives nothing
// there once the link reaches a desk's socket, and the Mac app is never asked to focus or close it, so it can't
// follow the link either (deskOfPane would match cd34's row through it).
func TestATabThroughALinkToADesksSocketIsNeverDriven(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab, cd := desks["ab12"], desks["cd34"]
	work := filepath.Join(root, "wt-ab12")
	link := filepath.Join(work, "lnk")
	relink(t, link, filepath.Dir(ab.socket))
	linked := "tmux:" + ab.pane + "@" + filepath.Join(link, deskSocketFile)
	d.mu.Lock()
	d.sessions.byID["s-old"] = &Session{ID: "s-old", Agent: Claude, Folder: work, Tab: linked, State: Idle}
	d.mu.Unlock()
	relink(t, link, filepath.Dir(cd.socket))

	err := typeIntoCard(d, "s-old", "tablink-probe-2")
	if cd.reached("tablink-probe-2", time.Second) || err == nil {
		t.Errorf("typing into a card on %s, now pointing at cd34's folder: %v, and it reached cd34's server %v", linked, err, cd.reached("tablink-probe-2", 0))
	}
	// Pointed back at its own folder: still nothing, never "unchecked".
	relink(t, link, filepath.Dir(ab.socket))
	if err := typeIntoCard(d, "s-old", "tablink-probe-3"); err == nil || ab.reached("tablink-probe-3", time.Second) {
		t.Errorf("typing into a card on %s, pointing at ab12's own folder: %v", linked, err)
	}

	// Under the Mac app: focus, fire and close aren't asked of it for that tab.
	mac := withMac(t)
	relink(t, link, filepath.Dir(cd.socket))
	d.mu.Lock()
	tab := d.tab(linked)
	d.mu.Unlock()
	if _, ok := tab.(macTab); !ok {
		t.Fatalf("under the Mac app, the tab is %T", tab)
	}
	focused := tab.Focus()
	closed := tab.Close(Claude)
	mac.mu.Lock()
	asked := slices.Clone(mac.asked)
	mac.mu.Unlock()
	for _, r := range asked {
		if r.Tab == linked {
			t.Errorf("the Mac app was asked to %s %s, with desk %+v", r.Action, r.Tab, r.Desk)
		}
	}
	if focused == nil || closed == nil {
		t.Errorf("focus %v, close %v: not refused", focused, closed)
	}
}

// The core's own spelling of a desk's socket may go through a link the core chose (a desk outside its sandbox on a Mac
// is given /tmp/tmux-<uid>/…, /tmp being the system's link; here a link of the test's stands in for it): taken as
// the row records it, kept so (the core's lists match a desk's tab by it: hiredRoleAt), and driven there. Another
// spelling through a link to the same socket is not.
func TestTheCoresOwnSpellingOfADesksSocketCountsThroughItsLink(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab := desks["ab12"]
	work := filepath.Join(root, "wt-ab12")
	system := filepath.Join(root, "sys")
	relink(t, system, filepath.Dir(filepath.Dir(ab.socket)))
	spelt := filepath.Join(system, "ab12", deskSocketFile)
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			if f.hires[i].ID == "ab12" {
				f.hires[i].TmuxSocket = spelt
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got := hookAs(t, d, ab.key, "tmux:"+ab.pane+"@"+spelt, "SessionStart", "s-ab12", work); got != http.StatusNoContent {
		t.Fatalf("ab12 on its socket as the core spelt it: %d", got)
	}
	if s := d.sessions.Get("s-ab12"); s == nil || s.Tab != "tmux:"+ab.pane+"@"+spelt || d.hiredRoleAt(spelt) != "janitor" {
		t.Errorf("ab12's tab is kept as %+v, not as the core spelt it", s)
	}
	if err := typeIntoCard(d, "s-ab12", "tablink-probe-4"); err != nil || !ab.reached("tablink-probe-4", 3*time.Second) {
		t.Errorf("typing into ab12's card: %v", err)
	}
	other := filepath.Join(work, "sys")
	relink(t, other, filepath.Dir(filepath.Dir(ab.socket)))
	if got := hookAs(t, d, ab.key, "tmux:"+ab.pane+"@"+filepath.Join(other, "ab12", deskSocketFile), "SessionStart", "s-ab12-o", work); got != http.StatusForbidden {
		t.Errorf("ab12 through a link of its own to its socket's folder: %d", got)
	}
}

// After the attempt and the re-point, the core still pairs ab12's session with ab12 and no one else (frontDeskLive's
// deskOfTab of the tab it kept): ab12's team row has it, cd34's doesn't, and its prompt asking you is shown as ab12's.
func TestAfterALinkIsRepointedADesksSessionIsStillItsOwnOnTheTeam(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab, cd := desks["ab12"], desks["cd34"]
	own, _, _ := linkAttempt(t, d, root, ab, cd)
	if got := teamSession(d, "ab12"); got != "s-ab12" {
		t.Errorf("ab12's team row pairs %q, not its own session", got)
	}
	if got := teamSession(d, "cd34"); got == "s-ab12" {
		t.Errorf("cd34's team row pairs ab12's session")
	}
	// ab12 asks you, from its own tab: its own event, taken.
	if got := hookAs(t, d, ab.key, own, "PermissionRequest", "s-ab12", filepath.Join(root, "wt-ab12")); got != http.StatusNoContent {
		t.Errorf("ab12's prompt from its own tab: %d", got)
	}
	d.mu.Lock()
	if s := d.sessions.byID["s-ab12"]; s != nil && s.State != Asking {
		s.State = Asking // so the pairing below is looked at whatever the event was answered
	}
	d.mu.Unlock()
	shown := false
	for _, it := range d.approvalItems() {
		if it.session == "s-ab12" {
			shown = true
			if it.hire != "ab12" {
				t.Errorf("ab12's prompt is shown as %q's", it.hire)
			}
		}
	}
	if !shown {
		t.Errorf("ab12's prompt isn't shown")
	}
}

// After the attempt and the re-point, cd34's key acts on no session of ab12's: with no tab, or from cd34's own tab.
// Nor on a session kept on a tab through a link (from before this check, say) that now leads to cd34's folder: a tab
// through a link the core didn't choose is no desk's own (allows' mine).
func TestAfterALinkIsRepointedAnotherDeskCantActOnTheSession(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab, cd := desks["ab12"], desks["cd34"]
	own, linked, _ := linkAttempt(t, d, root, ab, cd)
	cdWork := filepath.Join(root, "wt-cd34")
	cdOwn := "tmux:" + cd.pane + "@" + cd.socket
	if got := hookAs(t, d, cd.key, cdOwn, "SessionStart", "s-cd34", cdWork); got != http.StatusNoContent {
		t.Fatalf("cd34 on its own tab: %d", got)
	}
	if got := hookAs(t, d, cd.key, "", "SessionEnd", "s-ab12", cdWork); got != http.StatusForbidden {
		t.Errorf("cd34's key ended ab12's session, with no tab: %d", got)
	}
	if got := hookAs(t, d, cd.key, cdOwn, "UserPromptSubmit", "s-ab12", cdWork); got != http.StatusForbidden {
		t.Errorf("cd34's key moved ab12's session to its own tab: %d", got)
	}
	if s := d.sessions.Get("s-ab12"); s == nil || s.Tab != own {
		t.Errorf("ab12's session is now %+v, not on its own tab", s)
	}

	d.mu.Lock()
	d.sessions.byID["s-kept"] = &Session{ID: "s-kept", Agent: Claude, Folder: filepath.Join(root, "wt-ab12"), Tab: linked, State: Idle}
	d.mu.Unlock()
	if got := hookAs(t, d, cd.key, "", "UserPromptSubmit", "s-kept", cdWork); got != http.StatusForbidden {
		t.Errorf("cd34's key acted on a kept session through a link to its folder, with no tab: %d", got)
	}
	if got := hookAs(t, d, cd.key, cdOwn, "UserPromptSubmit", "s-kept", cdWork); got != http.StatusForbidden {
		t.Errorf("cd34's key moved a kept session through a link to its folder to its own tab: %d", got)
	}
	if s := d.sessions.Get("s-kept"); s == nil || s.Tab != linked {
		t.Errorf("the kept session is now %+v", s)
	}
}

// After the attempt and the re-point, ab12's card is driven on its own socket, as the core started it: with the tmux
// its row records and guarded by its recorded server. Under the Mac app, a Focus of the card asks with that socket's
// tab and ab12 as the desk, with that server: never a link's spelling, never "unchecked".
func TestAfterALinkIsRepointedADesksCardIsDrivenOnItsOwnServer(t *testing.T) {
	d, root, desks := tabLinkOffice(t)
	ab, cd := desks["ab12"], desks["cd34"]
	own, _, _ := linkAttempt(t, d, root, ab, cd)
	var recorded string
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.ID == "ab12" {
			recorded = h.TmuxProgram
		}
	}
	d.mu.Lock()
	var kept string
	if s := d.sessions.Get("s-ab12"); s != nil {
		kept = s.Tab
	}
	d.mu.Unlock()
	_, _, socket := splitTab(kept)
	if got := tmuxProgramFor(socket); got != recorded {
		t.Errorf("ab12's card on %q is driven by %q, not its recorded tmux %q", socket, got, recorded)
	}
	if _, guard := tmuxFor(socket); guard != strconv.Itoa(ab.server) {
		t.Errorf("ab12's card on %q is driven with guard %q, not its recorded server %d", socket, guard, ab.server)
	}

	mac := withMac(t)
	d.mu.Lock()
	tab := d.tab(kept)
	d.mu.Unlock()
	if _, ok := tab.(macTab); !ok {
		t.Fatalf("under the Mac app, the tab is %T", tab)
	}
	_ = tab.Focus() // what the stand-in answers doesn't matter here: what it was asked does
	mac.mu.Lock()
	asked := slices.Clone(mac.asked)
	mac.mu.Unlock()
	focused := false
	for _, r := range asked {
		if r.Action != "focus" {
			continue
		}
		focused = true
		if r.Tab != own || r.Desk == nil || r.Desk.ID != "ab12" || r.Desk.Server != strconv.Itoa(ab.server) {
			t.Errorf("the Mac app was asked to focus %s, with desk %+v, not %s as ab12 with server %d", r.Tab, r.Desk, own, ab.server)
		}
	}
	if !focused {
		t.Errorf("the Mac app wasn't asked to focus ab12's card")
	}
}
