//go:build !windows

package desktop

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// L1GUARD1FIRE: at a fire, a Take over and a Hand back, the core (never the guard in the desk's view) looks at a Linux
// desk's places no mount holds. It puts back only what the desk's guard stopped it for, what it finds set aside whole
// in the desk's found folder; it leaves anything else as it is (your own edit, your gc); one it can't put back refuses
// what it was doing; and the words reach your card. Each test works in its own temp repo, worktree and support folder;
// the desk's start and guard are the sandbox's own SaveKept, CheckKept and NoteKeptStop, run here as stand-ins.

// deskLine is what the desk writes in its registration's config.worktree: a stand-in line git never acts on.
const deskLine = "[stand-in]\n\tdesk = 1\n"

// keptHire keeps hire h's places as its Linux start does (sandbox.SaveKept), its registration read from its worktree's
// .git file, and gives them.
func keptHire(t *testing.T, h Hire) sandbox.Places {
	t.Helper()
	link, err := os.ReadFile(filepath.Join(h.Worktree, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	reg := strings.TrimSpace(strings.TrimPrefix(string(link), "gitdir:"))
	pl := sandbox.Places{GOOS: "linux", Home: homeDir, Support: supportDir, OwnID: h.ID, Root: h.Worktree,
		GitDir: filepath.Dir(filepath.Dir(reg)), OwnWorktree: filepath.Base(reg)}
	if err := sandbox.SaveKept(pl); err != nil {
		t.Fatal(err)
	}
	return pl
}

// configWorktree is the desk's registration's config.worktree.
func configWorktree(pl sandbox.Places) string {
	return filepath.Join(pl.GitDir, "worktrees", pl.OwnWorktree, "config.worktree")
}

// deskStoppedByItsGuard is the desk's guard at its next tool call (internal/guard's keptStop): it finds what changed, notes it,
// and touches nothing.
func deskStoppedByItsGuard(t *testing.T, h Hire) {
	t.Helper()
	held := sandbox.KeptFolder(supportDir, h.ID)
	changes, err := sandbox.CheckKept(held)
	if err != nil || len(changes) == 0 {
		t.Fatalf("the guard's look: %v, %+v", err, changes)
	}
	if err := sandbox.NoteKeptStop(held, changes); err != nil {
		t.Fatal(err)
	}
}

func writeKept(t *testing.T, at, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readKept(t *testing.T, at string) string {
	t.Helper()
	data, err := os.ReadFile(at)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return string(data)
}

// foundOf is the desk's found folder, where the core sets aside what it finds.
func foundOf(h Hire) string { return filepath.Join(supportDir, "hires", h.ID, "found") }

// setAsideAt is where at was set aside whole, "" when it wasn't.
func setAsideAt(t *testing.T, h Hire, at string) string {
	t.Helper()
	found, _ := filepath.Glob(filepath.Join(foundOf(h), "*") + at)
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

// inTheWay makes the desk's found folder a file, so nothing can be set aside: a put back that fails.
func inTheWay(t *testing.T, h Hire) {
	t.Helper()
	writeKept(t, foundOf(h), "in the way\n")
}

// fireRig is a hire ready to be fired from its card: its window idle, its session ending at "/exit", its git
// watched (each git call says whether at was there at that moment), the deck firing as the rig says.
func fireRig(t *testing.T, d *Deck, r *engineRig, at string) *[]bool {
	t.Helper()
	windowShows(r, pane(t, "idle-empty"), true)
	exitingPane(r, "0", true)
	var there []bool
	r.o.Git = func(dir string, args ...string) (int, string, bool) {
		_, err := os.Lstat(at)
		there = append(there, err == nil)
		return runGit(dir, args...)
	}
	d.mu.Lock()
	d.engine = r.o
	d.mu.Unlock()
	return &there
}

// A fire puts back what the desk was stopped for, before any git runs in its worktree, setting the desk's version
// aside whole; your own edit after the stop is never put back; your card and the hire's notes say which file, when,
// its lines before and after, and where the desk's version is.
func TestAFirePutsBackOnlyWhatTheDeskWasStoppedForBeforeGitAndTellsYou(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	// Your own Claude settings in the worktree, out of git's status, so the fire looks at what git says.
	writeKept(t, filepath.Join(hire.Repo, ".git", "info", "exclude"), ".claude/\n")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	owners := filepath.Join(hire.Worktree, ".claude", "settings.local.json")
	writeKept(t, owners, `{"permissions":{"allow":["your own"]}}`+"\n")
	r.o.KeepWorktree = true // its worktree stays, to show your edit in it is as you left it
	gitSaw := fireRig(t, d, r, cw)
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if status != http.StatusOK || registered(t)[0].State != "fired" {
		t.Fatalf("the fire: %d %q, registry %+v", status, said, registered(t))
	}
	if len(*gitSaw) == 0 || slices.Contains(*gitSaw, true) {
		t.Errorf("git ran with the desk's config.worktree there: %v", *gitSaw)
	}
	aside := setAsideAt(t, hire, cw)
	if readKept(t, cw) == deskLine || aside == "" || readKept(t, aside) != deskLine {
		t.Errorf("config.worktree is %q, the desk's set aside in %q", readKept(t, cw), aside)
	}
	if got := readKept(t, owners); got != `{"permissions":{"allow":["your own"]}}`+"\n" {
		t.Errorf("your own edit was put back: %q", got)
	}
	for _, want := range []string{cw, "seen at", `+ "[stand-in]"`, "set aside whole in " + `"` + aside + `"`,
		filepath.Join(hire.Worktree, ".claude"), "left as it is", "Smolcorp checked what Rex may not change."} {
		if !strings.Contains(said, want) {
			t.Errorf("your card isn't told %q:\n%s", want, said)
		}
	}
	if notes := registered(t)[0].Notes; !strings.Contains(notes, cw) || !strings.Contains(notes, aside) {
		t.Errorf("the hire's notes: %q", notes)
	}
}

// A put back that fails refuses the fire (fail closed): nothing else is done, no git runs, the desk's version stays
// where it is, and your card says which file, why and what to do. Once what was in the way is gone, the fire goes
// ahead. A copy that can't be read refuses it too.
func TestAFireWhosePutBackFailsIsRefusedWithWordsOnYourCard(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	inTheWay(t, hire)
	gitSaw := fireRig(t, d, r, cw)
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if status != http.StatusConflict || registered(t)[0].State == "fired" || !r.tmux.up || r.tmux.ran("kill-server") || len(*gitSaw) != 0 {
		t.Fatalf("a failed put back: %d %q, registry %+v, tmux %q, git %v", status, said, registered(t), verbs(r), *gitSaw)
	}
	for _, want := range []string{"Rex wasn't fired:", cw, "couldn't be set aside whole", "Move that place aside yourself", "seen at", `+ "[stand-in]"`} {
		if !strings.Contains(said, want) {
			t.Errorf("your card isn't told %q:\n%s", want, said)
		}
	}
	if readKept(t, cw) != deskLine {
		t.Errorf("config.worktree is %q", readKept(t, cw))
	}
	os.Remove(foundOf(hire))
	if status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false); status != http.StatusOK || !strings.Contains(said, "set aside whole in") ||
		registered(t)[0].State != "fired" || setAsideAt(t, hire, cw) == "" {
		t.Fatalf("once nothing was in the way: %d %q", status, said)
	}

	stopDecksOf(t)
	d, r, hire = hiredForFire(t, "Rex")
	keptHire(t, hire)
	writeKept(t, filepath.Join(sandbox.KeptFolder(supportDir, hire.ID), "kept.json"), "{")
	fireRig(t, d, r, cw)
	if status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false); status != http.StatusConflict ||
		!strings.Contains(said, "couldn't check the places it may not change") || registered(t)[0].State == "fired" {
		t.Fatalf("a copy that can't be read: %d %q", status, said)
	}
}

// Your gc between sessions is never put back, even with a note that names what it wrote (the desk can write its
// notes): the repo's shared objects/info stays as git left it, nothing is set aside, the fire goes ahead, and your
// card says it was left.
func TestAFireNeverPutsBackTheReposObjectsInfoAfterYourGc(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	info := filepath.Join(pl.GitDir, "objects", "info")
	other := filepath.Join(t.TempDir(), "objects")
	os.MkdirAll(other, 0o755)
	writeKept(t, info+"/packs", "P pack-2.pack\n")
	writeKept(t, info+"/alternates", other+"\n")
	held := sandbox.KeptFolder(supportDir, hire.ID)
	changes, err := sandbox.CheckKept(held)
	if err != nil || len(changes) != 2 {
		t.Fatalf("the gc: %v, %+v", err, changes)
	}
	var forged []byte
	for _, ch := range changes {
		line, _ := json.Marshal(map[string]string{"At": ch.At, "Seal": ch.Seal})
		forged = append(append(forged, line...), '\n')
	}
	writeKept(t, filepath.Join(filepath.Dir(held), "kept-stop.json"), string(forged))
	fireRig(t, d, r, info+"/alternates")
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if status != http.StatusOK || registered(t)[0].State != "fired" {
		t.Fatalf("the fire: %d %q", status, said)
	}
	if got := readKept(t, info+"/packs") + readKept(t, info+"/alternates"); got != "P pack-2.pack\n"+other+"\n" {
		t.Errorf("objects/info is %q", got)
	}
	if _, err := os.Lstat(foundOf(hire)); err == nil {
		t.Error("something was set aside")
	}
	if !strings.Contains(said, info+"/alternates") || strings.Count(said, "never put back") != 2 || strings.Contains(said, "restart this hire") {
		t.Errorf("your card: %s", said)
	}
}

// Take over puts back what the desk was stopped for once it's stopped, before T3 Code hears of it, and goes ahead:
// its answer says what was found. Only one that can't be put back stops it: then it isn't handed over, nor resumed.
func TestT3TakeOverPutsBackWhatTheDeskWasStoppedForAndSaysSo(t *testing.T) {
	k := newT3TakeOverRig(t)
	pl := keptHire(t, k.hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, k.hire)
	thread, said, err := t3TakeOverSaid(k.hire, k.session, k.ways())
	if err != nil || thread == "" || !slices.Contains(k.log, "call agentSessions.import") || slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("%q %v, log %q", thread, err, k.log)
	}
	aside := setAsideAt(t, k.hire, cw)
	for _, want := range []string{cw, "set aside whole in " + `"` + aside + `"`, "seen at", `+ "[stand-in]"`} {
		if aside == "" || !strings.Contains(said, want) {
			t.Errorf("you aren't told %q:\n%s", want, said)
		}
	}
	if _, err := os.Lstat(cw); err == nil || readKept(t, aside) != deskLine {
		t.Errorf("config.worktree wasn't put back, or the desk's isn't set aside in %q", aside)
	}

	k = newT3TakeOverRig(t)
	pl = keptHire(t, k.hire)
	writeKept(t, configWorktree(pl), deskLine)
	deskStoppedByItsGuard(t, k.hire)
	inTheWay(t, k.hire)
	if _, err = t3TakeOver(k.hire, k.session, k.ways()); err == nil {
		t.Fatalf("handed to T3 Code though nothing could be put back, log %q", k.log)
	}
	_, answer := t3Answer(err)
	said, _ = answer.(string)
	if !strings.Contains(said, "Ada wasn't handed to T3 Code:") || !strings.Contains(said, "Ada wasn't resumed:") ||
		slices.Contains(k.log, "call agentSessions.import") || slices.Contains(k.log, "resume "+t3TakeOverSession) ||
		readKept(t, configWorktree(pl)) != deskLine {
		t.Fatalf("a failed put back: %q, log %q", said, k.log)
	}
}

// RT126L1GUARD1R2 F1: your gc in the shared objects/info, or your git's repacking of the desk's branch folder, never
// stops a Take over, a Hand back or a fire: neither is put back, and each is only said, on your card.
func TestYourGcOrRepackNeverStopsATakeOverAHandBackOrAFire(t *testing.T) {
	gc := func(t *testing.T, pl sandbox.Places) string {
		info := filepath.Join(pl.GitDir, "objects", "info")
		writeKept(t, info+"/packs", "P pack-2.pack\n")
		return info + "/packs"
	}
	// As your git's ref packing leaves it: the desk's branch folder made again (a new inode), its refs kept.
	repack := func(t *testing.T, pl sandbox.Places) string {
		branch := filepath.Join(pl.GitDir, "refs", "heads", "hire", pl.OwnID)
		if err := os.Rename(branch, branch+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(branch, 0o755); err != nil {
			t.Fatal(err)
		}
		names, _ := os.ReadDir(branch + ".old")
		for _, n := range names {
			if err := os.Rename(filepath.Join(branch+".old", n.Name()), filepath.Join(branch, n.Name())); err != nil {
				t.Fatal(err)
			}
		}
		os.Remove(branch + ".old")
		return branch
	}
	for _, c := range []struct {
		name   string
		change func(*testing.T, sandbox.Places) string
		says   string
	}{{"gc", gc, "never put back"}, {"repack", repack, "repacked branches"}} {
		t.Run(c.name, func(t *testing.T) {
			// A branch folder is only kept when it's there at the start, as the view makes it.
			made := func(pl sandbox.Places) {
				os.MkdirAll(filepath.Join(pl.GitDir, "refs", "heads", "hire", pl.OwnID), 0o755)
			}
			k := newT3TakeOverRig(t)
			pl := keptPlaces(t, k.hire, made)
			at := c.change(t, pl)
			thread, said, err := t3TakeOverSaid(k.hire, k.session, k.ways())
			if err != nil || thread == "" || !strings.Contains(said, at) || !strings.Contains(said, c.says) || strings.Contains(said, "set aside") {
				t.Fatalf("Take over: %q %v: %s", thread, err, said)
			}

			k = newT3TakeOverRig(t)
			pl = keptPlaces(t, k.hire, made)
			at = c.change(t, pl)
			note, err := t3HandBack(k.takenOver(), k.ways())
			if err != nil || !slices.Contains(k.log, "resume "+t3TakeOverSession) || !strings.Contains(note, at) || !strings.Contains(note, c.says) {
				t.Fatalf("Hand back: %q %v, log %q", note, err, k.log)
			}

			d, r, hire := hiredForFire(t, "Rex")
			pl = keptPlaces(t, hire, made)
			at = c.change(t, pl)
			fireRig(t, d, r, at)
			status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
			if status != http.StatusOK || registered(t)[0].State != "fired" || !strings.Contains(said, at) || !strings.Contains(said, c.says) {
				t.Fatalf("the fire: %d %q", status, said)
			}
			if _, err := os.Lstat(foundOf(hire)); err == nil {
				t.Error("something was set aside")
			}
		})
	}
}

// keptPlaces is keptHire, with made done to the places first.
func keptPlaces(t *testing.T, h Hire, made func(sandbox.Places)) sandbox.Places {
	t.Helper()
	link, err := os.ReadFile(filepath.Join(h.Worktree, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	reg := strings.TrimSpace(strings.TrimPrefix(string(link), "gitdir:"))
	made(sandbox.Places{OwnID: h.ID, GitDir: filepath.Dir(filepath.Dir(reg))})
	return keptHire(t, h)
}

// A start stopped between its two renames (sandbox.SaveKept, N4) leaves only held.old: the fire looks there too, and
// puts back what the desk was stopped for.
func TestAFireFindsTheLastCopyAStartLeftAside(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	held := sandbox.KeptFolder(supportDir, hire.ID)
	if err := os.Rename(held, held+".old"); err != nil {
		t.Fatal(err)
	}
	fireRig(t, d, r, cw)
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if status != http.StatusOK || !strings.Contains(said, cw) || setAsideAt(t, hire, cw) == "" {
		t.Fatalf("the fire: %d %q", status, said)
	}
	if _, err := os.Lstat(cw); err == nil {
		t.Error("config.worktree wasn't put back")
	}
}

// Take over's answer, through Setup's route, carries what the look found, as "note": Setup shows it.
func TestT3TakeOversRouteSaysWhatWasPutBack(t *testing.T) {
	k := newT3TakeOverRig(t)
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	t3HandingStand = func(*Deck, Hire) t3Handing { return k.ways() }
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := newTestDeckAfter(t, func() { k.ownersHome(t) })
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	t3Registered(t, k.hire)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree,
		"transcript_path": k.session.Transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	pl := keptHire(t, k.hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, k.hire)
	got := serve(t, d, t3PageCall("POST", "/setup/t3code/takeover", `{"id":"`+k.hire.ID+`"}`))
	var answer struct{ Note, Thread string }
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer.Thread == "" ||
		!strings.Contains(answer.Note, cw) || !strings.Contains(answer.Note, "set aside whole in") {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
}

// Hand back puts back what the desk was stopped for before its session is resumed; what changed while T3 Code had it
// (your own Claude Code) is left as it is, and Hand back's note says both. One that can't be put back isn't resumed.
func TestT3HandBackPutsBackOnlyWhatTheDeskWasStoppedForAndSaysWhatChangedInT3Code(t *testing.T) {
	k := newT3TakeOverRig(t)
	pl := keptHire(t, k.hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, k.hire)
	h := k.takenOver()
	yours := filepath.Join(k.hire.Worktree, ".claude", "commands", "yours.md")
	writeKept(t, yours, "Your own command.\n")
	note, err := t3HandBack(h, k.ways())
	if err != nil || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("%q %v, log %q", note, err, k.log)
	}
	aside := setAsideAt(t, k.hire, cw)
	for _, want := range []string{cw, "set aside whole in " + `"` + aside + `"`, filepath.Join(k.hire.Worktree, ".claude"), "commands/yours.md", "left as it is"} {
		if aside == "" || !strings.Contains(note, want) {
			t.Errorf("Hand back's note doesn't say %q:\n%s", want, note)
		}
	}
	if _, err := os.Lstat(cw); err == nil || readKept(t, yours) != "Your own command.\n" {
		t.Errorf("config.worktree wasn't put back, or your command was: %q", readKept(t, yours))
	}

	k = newT3TakeOverRig(t)
	pl = keptHire(t, k.hire)
	writeKept(t, configWorktree(pl), deskLine)
	deskStoppedByItsGuard(t, k.hire)
	inTheWay(t, k.hire)
	note, err = t3HandBack(k.takenOver(), k.ways())
	_, answer := t3Answer(err)
	said, _ := answer.(string)
	if err == nil || !strings.Contains(said, "Ada wasn't resumed:") || slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Fatalf("a failed put back: %q %q, log %q", note, said, k.log)
	}
}

// RT126L1GUARD1FIRE B1 (probe PA): a fire whose first look put back what the desk was stopped for, then refused for
// uncommitted work, still says what it put back and where the desk's version is: no later look finds it again.
func TestAFireRefusedAfterItsFirstLookStillSaysWhatItPutBack(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	writeKept(t, filepath.Join(hire.Worktree, "unsaved.txt"), "not committed yet\n")
	fireRig(t, d, r, cw)
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	aside := setAsideAt(t, hire, cw)
	if status != http.StatusConflict || registered(t)[0].State == "fired" || !strings.Contains(said, "aren't committed") || aside == "" {
		t.Fatalf("the fire: %d %q, set aside in %q", status, said, aside)
	}
	for _, want := range []string{cw, "set aside whole in " + `"` + aside + `"`, `+ "[stand-in]"`} {
		if !strings.Contains(said, want) {
			t.Errorf("your card isn't told %q:\n%s", want, said)
		}
	}
}

// RT126L1GUARD1FIRE B1 (probe PD): Take over put back at step 2b, then T3 Code refused the worktree, so it was handed
// back: the refusal still says what was put back, once.
func TestAGivenBackTakeOverStillSaysWhatItPutBack(t *testing.T) {
	k := newT3TakeOverRig(t, "addfail")
	pl := keptHire(t, k.hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, k.hire)
	_, _, err := t3TakeOverSaid(k.hire, k.session, k.ways())
	_, answer := t3Answer(err)
	said, _ := answer.(string)
	aside := setAsideAt(t, k.hire, cw)
	if err == nil || !strings.Contains(said, "was resumed in its tmux session") || aside == "" ||
		strings.Count(said, "set aside whole in "+`"`+aside+`"`) != 1 || !strings.Contains(said, `+ "[stand-in]"`) {
		t.Fatalf("given back: %q, set aside in %q, log %q", said, aside, k.log)
	}
}

// RT126L1GUARD1FIRE B1: Hand back put back at step 4b, then its session couldn't be resumed: the refusal still says
// what was put back.
func TestAHandBackRefusedAfterItsPutBackStillSaysWhatItPutBack(t *testing.T) {
	k := newT3TakeOverRig(t)
	pl := keptHire(t, k.hire)
	cw := configWorktree(pl)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, k.hire)
	k.resumed = errors.New("tmux didn't start it.")
	_, err := t3HandBack(k.takenOver(), k.ways())
	_, answer := t3Answer(err)
	said, _ := answer.(string)
	aside := setAsideAt(t, k.hire, cw)
	if err == nil || !strings.Contains(said, "couldn't be resumed") || aside == "" ||
		!strings.Contains(said, "set aside whole in "+`"`+aside+`"`) || !strings.Contains(said, cw) {
		t.Fatalf("not resumed: %q, set aside in %q", said, aside)
	}
}

// RT126L1GUARD1FIRE B2 (probe PB): a put back that fails once the desk's version is set aside (the stand-in for a full
// disk at the write: the copy's content gone from held) puts the desk's version back as it was found and refuses the
// fire, saying so; the next fire refuses again rather than going on, and once there is room it puts back.
func TestAFireWhosePutBackFailsAfterTheSetAsideRefusesUntilItCan(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	owners := "[core]\n\towner = 1\n"
	writeKept(t, cw, owners)
	if err := sandbox.SaveKept(pl); err != nil { // its next start takes your config.worktree as its copy
		t.Fatal(err)
	}
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	content := filepath.Join(sandbox.KeptFolder(supportDir, hire.ID), fmt.Sprintf("%x", sha256.Sum256([]byte(owners))))
	if err := os.Rename(content, content+".gone"); err != nil {
		t.Fatal(err)
	}
	r.o.KeepWorktree = true // its worktree, and so its registration, stays, to show what was put back
	fireRig(t, d, r, cw)
	for try := 1; try <= 2; try++ {
		status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
		if status != http.StatusConflict || registered(t)[0].State == "fired" || !r.tmux.up {
			t.Fatalf("try %d: %d %q", try, status, said)
		}
		for _, want := range []string{"Rex wasn't fired:", cw, "couldn't be made again", "the place is as it was found", `+ "[stand-in]"`} {
			if !strings.Contains(said, want) {
				t.Errorf("try %d: your card isn't told %q:\n%s", try, want, said)
			}
		}
		if readKept(t, cw) != deskLine || setAsideAt(t, hire, cw) != "" {
			t.Fatalf("try %d: config.worktree is %q, set aside in %q", try, readKept(t, cw), setAsideAt(t, hire, cw))
		}
	}
	if err := os.Rename(content+".gone", content); err != nil {
		t.Fatal(err)
	}
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	aside := setAsideAt(t, hire, cw)
	if status != http.StatusOK || registered(t)[0].State != "fired" || readKept(t, cw) != owners || aside == "" || readKept(t, aside) != deskLine {
		t.Fatalf("with room again: %d %q, config.worktree %q, set aside in %q", status, said, readKept(t, cw), aside)
	}
}

// RT126L1GUARD1FIRE B3: the fire's second look, once its session is gone, fails closed too. The desk's guard notes a
// change after the first look (here while the fire's git looks at its worktree), and the second look can't put it
// back: the fire is refused, not fired, its worktree not removed, and your card says which place and why.
func TestAFireWhoseSecondLookCantPutBackIsRefused(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	fireRig(t, d, r, cw)
	var gitRan []string
	r.o.Git = func(dir string, args ...string) (int, string, bool) {
		if len(gitRan) == 0 { // after the first look: the desk changes its config.worktree, its guard notes it
			writeKept(t, cw, deskLine)
			deskStoppedByItsGuard(t, hire)
			inTheWay(t, hire)
		}
		gitRan = append(gitRan, strings.Join(args, " "))
		return runGit(dir, args...)
	}
	d.mu.Lock()
	d.engine = r.o
	d.mu.Unlock()
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if status != http.StatusConflict || registered(t)[0].State == "fired" || len(gitRan) == 0 ||
		slices.ContainsFunc(gitRan, func(args string) bool { return strings.HasPrefix(args, "worktree remove") }) {
		t.Fatalf("the second look's put back failed: %d %q, registry %+v, git %q", status, said, registered(t), gitRan)
	}
	for _, want := range []string{"Rex wasn't fired:", cw, "couldn't be set aside whole", `+ "[stand-in]"`} {
		if !strings.Contains(said, want) {
			t.Errorf("your card isn't told %q:\n%s", want, said)
		}
	}
	if readKept(t, cw) != deskLine {
		t.Errorf("config.worktree is %q", readKept(t, cw))
	}
}

// RT126L1GUARD1FIRE nit: when a start left its last copy beside held, a refused fire's way out names both, as removing
// held alone would have the next look use the older copy.
func TestARefusedFireNamesTheLastCopyBesideItToo(t *testing.T) {
	d, r, hire := hiredForFire(t, "Rex")
	pl := keptHire(t, hire)
	writeKept(t, configWorktree(pl), deskLine)
	deskStoppedByItsGuard(t, hire)
	held := sandbox.KeptFolder(supportDir, hire.ID)
	if err := os.Mkdir(held+".old", 0o700); err != nil {
		t.Fatal(err)
	}
	inTheWay(t, hire)
	fireRig(t, d, r, configWorktree(pl))
	status, said, _ := d.fireTab("tmux:%1@"+hire.TmuxSocket, false)
	if want := "Removing " + strconv.Quote(held) + " and " + strconv.Quote(held+".old") + " lets it go ahead"; status != http.StatusConflict || !strings.Contains(said, want) {
		t.Fatalf("%d, want %q in:\n%s", status, want, said)
	}
}

// RMSYLCHAIN B1: a fire of a sandboxed desk whose row says gone (GMEDIT1), whose second look, once its session is
// ended, can't put back. The session is known ended, so SA-M6's stop runs before the refusal: listed once, its
// sandbox let go, its socket folder taken away. The retry the refusal's words ask for, once what was in the way is
// gone, puts back and says so (its socket gone, it is found gone at once), and stops nothing a second time.
func TestAGoneDesksFireWhoseSecondLookCantPutBackStopsItsDeskAndTheRetryPutsBack(t *testing.T) {
	_, r, hire, _ := sandboxedForHandBack(t)
	windowShows(r, pane(t, "idle-empty"), true)
	stillClock(r)
	listed := stopListed(t)
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	before := readKept(t, cw)
	markedGoneByAnotherProgram(t)
	made, ok := deskSandboxOf(hire.ID)
	if !ok {
		t.Fatal("no sandbox kept")
	}
	run, changed := r.o.Tmux, false
	r.o.Tmux = func(args []string) (string, int, bool) {
		if slices.Contains(args, "kill-server") && !changed { // after the first look: the desk changed its
			// config.worktree, its guard noted it, and the second look can't put it back
			changed = true
			writeKept(t, cw, deskLine)
			deskStoppedByItsGuard(t, hire)
			inTheWay(t, hire)
		}
		return run(args)
	}
	_, err := fireEngineHire(hire, 0, r.o, false)
	refused, _ := err.(*hireRefusal)
	if !changed || refused == nil || refused.Status != http.StatusConflict || r.tmux.up ||
		!strings.Contains(refused.Message, "Janitor wasn't fired:") || !strings.Contains(refused.Message, "Move that place aside yourself and try again") {
		t.Fatalf("the second look's refusal: changed %v, %v, tmux up %v", changed, err, r.tmux.up)
	}
	if len(*listed) != 1 || (*listed)[0] != made {
		t.Errorf("not listed at its stop: %v", *listed)
	}
	if _, ok := deskSandboxOf(hire.ID); ok {
		t.Error("its sandbox is still kept")
	}
	if _, err := os.Lstat(filepath.Dir(hire.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	if readKept(t, cw) != deskLine {
		t.Errorf("config.worktree is %q", readKept(t, cw))
	}
	// The retry.
	os.Remove(foundOf(hire))
	_, err = fireEngineHire(hire, 0, r.o, false)
	refused, _ = err.(*hireRefusal)
	aside := setAsideAt(t, hire, cw)
	if refused == nil || !strings.HasPrefix(refused.Message, "Janitor is already gone.") || aside == "" ||
		readKept(t, cw) != before || readKept(t, aside) != deskLine {
		t.Fatalf("the retry: %v, config.worktree %q, set aside in %q", err, readKept(t, cw), aside)
	}
	for _, want := range []string{cw, "set aside whole in " + `"` + aside + `"`, `+ "[stand-in]"`} {
		if !strings.Contains(refused.Message, want) {
			t.Errorf("the retry doesn't say %q:\n%s", want, refused.Message)
		}
	}
	if len(*listed) != 1 {
		t.Errorf("stopped again: %v", *listed)
	}
}

// RMSYLCHAIN B1: a desk whose row says gone with its session already down (gone && !up) is looked at before it's said
// to be gone: what its guard stopped it for is put back and said; one that can't be put back still refuses, as a
// fire's look does, and the next fire, nothing in the way, puts back.
func TestAFireOfAGoneDeskWithNothingRunningStillPutsBackWhatItWasStoppedFor(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	pl := keptHire(t, hire)
	cw := configWorktree(pl)
	before := readKept(t, cw)
	writeKept(t, cw, deskLine)
	deskStoppedByItsGuard(t, hire)
	setHire(hire.ID, "fired", "")
	windowShows(r, pane(t, "idle-empty"), true)
	r.tmux.up = false
	inTheWay(t, hire)
	r.tmux.asked = nil
	_, err := fireEngineHire(hire, 0, r.o, false)
	if !r.tmux.ran("has-session") || err == nil || !strings.Contains(err.Error(), "Janitor wasn't fired:") ||
		strings.Contains(err.Error(), "already gone") || readKept(t, cw) != deskLine {
		t.Fatalf("a look that can't put back: %v, tmux %q, config.worktree %q", err, r.tmux.asked, readKept(t, cw))
	}
	os.Remove(foundOf(hire))
	_, err = fireEngineHire(hire, 0, r.o, false)
	aside := setAsideAt(t, hire, cw)
	if err == nil || !strings.HasPrefix(err.Error(), "Janitor is already gone.\nSmolcorp checked what Janitor may not change.") ||
		!strings.Contains(err.Error(), "set aside whole in "+`"`+aside+`"`) || aside == "" || readKept(t, cw) != before {
		t.Fatalf("the next fire: %v, config.worktree %q, set aside in %q", err, readKept(t, cw), aside)
	}
	// Nothing to put back: as before, only gone.
	if _, err = fireEngineHire(hire, 0, r.o, false); err == nil || err.Error() != "Janitor is already gone." {
		t.Fatalf("with nothing to put back: %v", err)
	}
}

// RMSYLCHAIN B3 (mutant E): a fire of a desk whose row says gone, its session still running, looks as any fire does.
// What its guard stopped it for before the fire (the first look) or while its session was being ended (the second) is
// put back, and its 200 (stoppedGone) says what, where the desk's version is set aside.
func TestAFireOfAGoneDeskStillRunningSaysWhatItsLooksPutBack(t *testing.T) {
	for _, when := range []string{"before the fire", "while its session ends"} {
		t.Run(when, func(t *testing.T) {
			_, r, hire, _ := sandboxedForHandBack(t)
			windowShows(r, pane(t, "idle-empty"), true)
			stillClock(r)
			pl := keptHire(t, hire)
			cw := configWorktree(pl)
			before := readKept(t, cw)
			markedGoneByAnotherProgram(t)
			gone := registered(t)[0]
			changed := false
			if when == "before the fire" {
				writeKept(t, cw, deskLine)
				deskStoppedByItsGuard(t, hire)
				changed = true
			} else {
				run := r.o.Tmux
				r.o.Tmux = func(args []string) (string, int, bool) {
					if slices.Contains(args, "kill-server") && !changed {
						changed = true
						writeKept(t, cw, deskLine)
						deskStoppedByItsGuard(t, hire)
					}
					return run(args)
				}
			}
			_, err := fireEngineHire(hire, 0, r.o, false)
			refused, _ := err.(*hireRefusal)
			aside := setAsideAt(t, hire, cw)
			if !changed || refused == nil || refused.Status != http.StatusOK || aside == "" || readKept(t, cw) != before || readKept(t, aside) != deskLine {
				t.Fatalf("the fire: changed %v, %v, config.worktree %q, set aside in %q", changed, err, readKept(t, cw), aside)
			}
			want := goneStoppedNote(gone, "tmux session") + "\nSmolcorp checked what Janitor may not change."
			if !strings.HasPrefix(refused.Message, want) || strings.Count(refused.Message, "set aside whole in "+`"`+aside+`"`) != 1 ||
				!strings.Contains(refused.Message, cw) || !strings.Contains(refused.Message, `+ "[stand-in]"`) {
				t.Errorf("its 200 doesn't say what was put back:\n%s", refused.Message)
			}
		})
	}
}

// RMSYLCHAIN B3 (mutant F): Take over's kept put back (2b) comes before CLAUDEPLANT1's listing (2c). A .claude its
// guard stopped the desk for is put back first, so 2c never holds it for you, and T3 Code is told of the worktree.
func TestTakeOverPutsBackAStoppedForClaudeFolderBeforeItsListingHoldsIt(t *testing.T) {
	freshClaudeHeld(t)
	k := newT3TakeOverRig(t)
	keptHire(t, k.hire)
	plant := filepath.Join(k.hire.Worktree, ".claude", "hooks", "pre.sh")
	writeKept(t, plant, "#!/bin/sh\n# stand-in\n")
	deskStoppedByItsGuard(t, k.hire)
	thread, said, err := t3TakeOverSaid(k.hire, k.session, k.ways())
	if err != nil || thread == "" || !slices.Contains(k.log, "call agentSessions.import") {
		t.Fatalf("%q %v, log %q", thread, err, k.log)
	}
	if _, err := os.Lstat(plant); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the desk's .claude wasn't put back: %v", err)
	}
	if why := claudeHeldRefusal(k.hire.Worktree); why != "" {
		t.Errorf("held for you: %s", why)
	}
	if !strings.Contains(said, filepath.Join(k.hire.Worktree, ".claude")) || !strings.Contains(said, "set aside whole in") {
		t.Errorf("Take over doesn't say what it put back:\n%s", said)
	}
}
