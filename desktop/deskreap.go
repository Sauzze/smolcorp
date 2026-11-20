//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A desk's every process, ended at its stop (FXFIREREAP, RT126SWEEP2 P3 B1). Ending a desk's tmux server (kill-server,
// a SIGHUP) ends its panes, but not what they started in the background (nohup, setsid, a dev server): that goes on
// running in the desk's sandbox, with its rights, after the fire has checked the repo's objects. So at a fire, and at
// Take over's and restart's stop, once the desk's session is seen gone, every process still in its sandbox is ended
// (SIGTERM, a short grace, then SIGKILL) until a look at every process of the owner's account finds none of it left.
// Only then does the fire look at its worktree, check the objects (FIREFSCK), merge and remove. A desk that can't be
// shown empty stops there: the fire refuses, and says so.
//
// Which processes are a desk's is the kernel's to say, not their group or session (setsid leaves both): macOS's sandbox
// check, asked of each process of the owner's account, with places the core made and no desk can change. Its mark
// (<support>/hires/<id>/reap.noindex/reap-<random>, deskReapMark) only that desk's sandbox may read; its profile
// (sandbox.sb beside it) no desk's may write; the hires folder, the support folder and the mark's folder no desk's may
// read. An unsandboxed process may do all five; another desk's may do none of them; another sandbox doesn't read the
// mark, or reads one of the three folders too (FXFIREREAPR1 O1). A sandbox may also read what an extension it took
// grants (App Sandbox's do: Quick Look, Spotlight's importers): a read extension for the mark alone would make another
// sandbox answer as the desk's. No desk may issue one (internal/sandbox's profile), no app of the owner's is handed a
// file the core alone makes and names at random, and an extension for any folder above the mark opens its folder too
// (FXFIREREAPR5). The profile itself was the allowed read before, and Quick Look or Spotlight may well be handed that.
// A signal goes to that one run of the process (its audit token), never to a pid that another process has taken since
// (deskreap_darwin.go).
// No setuid program runs in a desk's sandbox, so every process of it runs as the owner on both its ids: the owner's
// account's list holds them all.
//
// "None left" is said only from two passes in a row that each list every process afresh and find none of the desk's,
// where the second finds no process gone before or during its look that the first didn't see running whole outside
// the sandbox (FXFIREREAPR1 N1). A desk's process that forks and ends between the list and its look is seen gone (a
// process being made or ending answers as one that's ended: so is it); its child, started after that list, is in the
// next one, and is seen there, or seen gone too, a pid the pass before never saw running. So a process that keeps
// moving to a new pid never lets the reap say "none left": the reap ends it once it's caught, or refuses at its time
// limit.
//
// An ended process whose parent hasn't read its end (a zombie) is listed until it does, which may be never, from any
// of the owner's apps. It runs nothing and can't fork, in a desk's sandbox or not, so it's answered for too when the
// pass before saw it ended, the same process: any child it had was made before that pass's look, so before this pass's
// list, and is in it (FXFIREREAPR2). A zombie of a desk's whose parent runs in the desk is the parent's to read: the
// parent is found, and ended, and launchd reads the zombie's end then.
//
// "The same process" is its pid and its start time, to the microsecond, never its pid alone (FXFIREREAPR3). macOS hands
// pids out in turn, but once they've gone round (past 99999, the first time since the Mac started; on a Mac that's been
// up a while, long ago), a pid just freed can be handed out again within a few forks: a pid the pass before answered
// for may be another process's, a desk's, by this pass's look. So a process gone or ended at this pass's look is
// answered for only when the look read its start time before it went and the pass before saw that same pid and start
// time running whole outside the sandbox, or ended. A look that couldn't read it costs one more pass.

// deskReapGrace is how long a desk's processes have after SIGTERM, deskReapWithin how long after SIGKILL before a stop
// refuses, and deskReapEvery how often the reap looks again: tests make them short.
var deskReapGrace, deskReapWithin, deskReapEvery = 2 * time.Second, 3 * time.Second, 50 * time.Millisecond

// reapAt is what a look asks about one desk's sandbox, every path resolved: its profile and hires folder, and its mark
// (deskReapMark), "" for a desk made before marks that has none yet: its profile is the allowed read then, and nothing
// is signalled (reapSandboxAt).
type reapAt struct{ profile, hires, mark string }

// reapRun is one run of one process: its pid, and the audit token a signal goes to, which names nothing once that
// process has ended or run another program.
type reapRun struct {
	pid   int
	token [8]uint32
	start uint64 // its start time to the microsecond, as the look first read it (0 when it couldn't): with its pid, that one process
}

// reapVerdict is what a look at one process found.
type reapVerdict int

const (
	reapNotIn       reapVerdict = iota // not in the desk's sandbox: the same process answered, running whole throughout
	reapIn                             // in it, with its run taken: a signal reaches it
	reapInUnreached                    // in it, asked by its pid, its run not taken: nothing here can end it
	reapUnknown                        // macOS didn't answer
	reapGone                           // not in it as asked, but being made, ending or ended before or while it was asked
	reapZombie                         // ended, waiting for its parent to read its end: it runs nothing, and can't fork
)

// deskReapSystem is what the reap asks the system: list every process of the owner's account, look at one (in the
// sandbox at at? the support folder is the hires folder's), signal one run. Each system's own (deskreap_darwin.go); a
// test narrows list to its own processes.
var deskReapSystem = struct {
	list   func() ([]int, error)
	look   func(pid int, at reapAt) (reapRun, reapVerdict)
	signal func(run reapRun, sig syscall.Signal) error
}{reapList, reapLook, reapSignal}

// reapSteps are a look's questions to macOS, one each (deskreap_darwin.go): a test stands in for them.
type reapSteps struct {
	// running is whether the process is running, whole: 1 made (not still being forked), and not on its way out of
	// exit(); 2 ended, waiting for its parent to read its end (a zombie); 0 neither; -1 macOS won't say. Its start time
	// to the microsecond goes with it, when read: with its pid, no other process has had it.
	running func(pid int) (state int, start uint64)
	// token is the audit token of the program the process runs now, when it can be taken.
	token func(pid int) (token [8]uint32, ok bool)
	// in is whether the process is in the desk's sandbox, asked by its token (by its pid when nil): 1 in it, 0 not (or
	// ended), -1 not answered.
	in func(token *[8]uint32, pid int) int
}

// reapLookWith is a look at process pid by steps. What macOS says of a process being made or ending isn't what its
// sandbox lets it do: both answer denied (FXFIREREAPR1). So it's in (reapIn, its run taken; reapInUnreached, asked by
// its pid) only when the same process ran whole, made, before it was asked and after (FXFIREREAPR4): a process being
// made at the first check, or another process by the end, is asked again. Not in (reapNotIn) is the same process
// running whole before and after it was asked, and, with its run taken, running the same program (its pid version,
// the token's last word, the same after); for one macOS won't say is running, that alone. An ended process waiting for
// its parent (reapZombie) runs nothing and can't fork, in a sandbox or not, so it isn't asked (FXFIREREAPR2). Not in it
// as asked, but none of those (reapGone): being made, ending or ended before or while it was asked. One that ran
// another program meanwhile is asked again, at most 8 times, then not answered (reapUnknown). The run's start time is
// as the look's first read found it before anything else (0 when it couldn't be read): with its pid, it names the
// process the look began with, whatever has that pid by the end (FXFIREREAPR3).
func reapLookWith(steps reapSteps, pid int) (reapRun, reapVerdict) {
	for range 8 {
		run := reapRun{pid: pid}
		whole, first := steps.running(pid)
		run.start = first
		if whole == 2 {
			return run, reapZombie
		}
		// No run to take: ended, ending, or not the caller's user on both its ids. Asked by its pid then, and an ended
		// process answers denied.
		token, held := steps.token(pid)
		asked := (*[8]uint32)(nil)
		if held {
			run.token, asked = token, &token
		}
		in := steps.in(asked, pid)
		if in == -1 {
			return run, reapUnknown
		}
		now, last := steps.running(pid)
		if in == 1 {
			if whole == 1 && now == 1 && last == first {
				if held {
					return run, reapIn
				}
				return run, reapInUnreached
			}
			continue // being made at the first check, or not the same process throughout: asked again
		}
		same := whole == 1 && now == 1 && last == first || whole == -1 && now == -1 && held
		if held {
			after, ok := steps.token(pid)
			if !ok {
				return run, reapGone // ended since
			}
			if after[7] != token[7] {
				continue // another program since: asked again
			}
		}
		if same {
			return run, reapNotIn
		}
		return run, reapGone
	}
	return reapRun{pid: pid}, reapUnknown
}

// deskReap ends every process still in desk h's sandbox, and looks until none is left. "" when none is, and when h has
// no sandbox to look in: off a Mac, a desk started without its sandbox (its socket isn't one in its sandbox), or one
// whose profile isn't there (no desk can remove it: deskRunsSandboxed reads its absence the same way). Otherwise why
// its stop goes no further, for the owner: a profile that isn't a plain file, a look that can't be made whole, a
// process that couldn't be ended.
func deskReap(h Hire) string {
	if !deskSandboxed || !plainHireID(h.ID) || deskSocketID(h.TmuxSocket, testCopyPort()) != h.ID {
		return ""
	}
	left, sure := reapSandboxAt(hireFolder(supportDir, h.ID), true)
	if len(left) == 0 && sure {
		return ""
	}
	log.Printf("sandbox: desk %s at its stop: its processes %v left, every process answered: %t", h.ID, left, sure)
	return reapLeftWords(h, left)
}

// reapSandboxAt ends (end), or only looks for, every process in the sandbox of the desk whose own folder is own
// (<support>/hires/<id>): what reapDesk says. (none, true) at once when no profile is there; (none, false) when
// what's there isn't a plain file, or its mark can't be read whole. A plain hire's launcher (launchersandbox_darwin.go)
// and an ACP desk (acpsandbox_darwin.go) end theirs with it; deskStoppedNow looks with it.
//
// A desk made before marks (FXFIREREAPR5) has none until a start finds nothing of it running (deskMarkAfterProfile):
// till then it's looked at by its profile, which its sandbox reads, as every desk's does, and so may another that holds
// a read extension for it. Such a process may not be the desk's, so nothing is signalled there: a desk found empty is
// said so; anything found stays as it is, and the stop says it couldn't be ended, and what ends it (firedRefusal,
// stopRefusal).
func reapSandboxAt(own string, end bool) (left []int, sure bool) {
	hires := physical(filepath.Dir(own))
	folder := filepath.Join(hires, filepath.Base(own))
	at := reapAt{profile: filepath.Join(folder, "sandbox.sb"), hires: hires}
	info, err := os.Lstat(at.profile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true
	}
	if err != nil || !info.Mode().IsRegular() {
		log.Printf("sandbox: the profile %s at its desk's stop isn't a file (%v)", at.profile, err)
		return nil, false
	}
	switch mark, err := reapMarkIn(folder); {
	case err == nil:
		at.mark = filepath.Join(folder, sandbox.ReapMarkFolder, mark)
	case errors.Is(err, fs.ErrNotExist):
		log.Printf("sandbox: desk %s has no mark (made before marks): looked at by its profile, nothing signalled", filepath.Base(own))
		end = false
	default:
		log.Printf("sandbox: desk %s's mark at its stop: %v", filepath.Base(own), err)
		return nil, false
	}
	return reapDesk(at, end)
}

// deskReapMark is the name of the mark of the desk whose own folder is own, for its Policy (Places.ReapMark), on a Mac:
// an empty file in its folder sandbox.ReapMarkFolder, named at random (sandbox.NewReapMark), which its sandbox alone
// may read and the reap asks about. The core makes it at the desk's first start, before its first profile, and the desk
// keeps it for the rest of its life: the file is its record. A desk whose profile is there without one was made before
// marks (FXFIREREAPR6): it's given a new name, for the profile its start writes, and the file is made only once that
// profile is written, if nothing of the desk runs then (deskMarkAfterProfile).
func deskReapMark(own string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", nil
	}
	mark, err := reapMarkIn(own)
	if !errors.Is(err, fs.ErrNotExist) {
		return mark, err
	}
	_, profiled := os.Lstat(filepath.Join(own, "sandbox.sb"))
	if profiled != nil && !errors.Is(profiled, fs.ErrNotExist) {
		return "", profiled
	}
	if mark, err = sandbox.NewReapMark(); err != nil {
		return "", err
	}
	if profiled == nil {
		return mark, nil // a desk made before marks: made after its profile, maybe (deskMarkAfterProfile)
	}
	return mark, makeReapMark(own, mark)
}

// makeReapMark makes the mark named mark in the desk's own folder own. Its folder comes whole, by a rename, so a start
// that fails halfway leaves no mark, or one; one made meanwhile, its mark in it, refuses the rename.
func makeReapMark(own, mark string) error {
	made, err := os.MkdirTemp(own, ".reap-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(made) // gone after the rename; what a failure before it left
	file, err := os.OpenFile(filepath.Join(made, mark), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(made, filepath.Join(own, sandbox.ReapMarkFolder))
}

// deskMarkAfterProfile makes the mark named mark (deskReapMark) of the desk whose own folder is own, once its start has
// written its profile, which allows its sandbox to read that mark, and before anything of that start runs (FXFIREREAPR6).
// Only a desk with no mark gets one here, one made before marks, and only when a look at every process of the owner's
// account (reapSandboxAt, looking only, two passes agreeing) finds none in its sandbox: its profile is the allowed read
// then, which every desk's sandbox reads, so whatever runs from an earlier start, under an earlier profile, is found.
// None found, nothing is left of its earlier sandboxes for a reap by the mark to miss, and every process of it from now
// on runs under a profile that allows the mark: this one, written before the look, or a later start's, which keeps it.
// One found, or a look that couldn't be made whole, and it keeps no mark: its stops look and signal nothing, as before.
// The start goes on then. A mark that couldn't be made leaves the same, unless a mark folder is there after all, made
// meanwhile, without this one (FXFIREREAPR7): the profile just written lets its sandbox read a mark that isn't the
// desk's, so its processes would answer the reap "not in". That refuses the start.
func deskMarkAfterProfile(own, mark string) error {
	if _, err := reapMarkIn(own); mark == "" || !errors.Is(err, fs.ErrNotExist) {
		return nil // off a Mac, or its mark is there (a bad one refused the start already)
	}
	if left, sure := reapSandboxAt(own, false); len(left) != 0 || !sure {
		log.Printf("sandbox: desk %s keeps no mark: its processes %v still run from before, every process answered: %t", filepath.Base(own), left, sure)
		return nil
	}
	if err := makeReapMark(own, mark); err != nil {
		there, again := reapMarkIn(own)
		if errors.Is(again, fs.ErrNotExist) {
			log.Printf("sandbox: desk %s's mark: %v", filepath.Base(own), err)
			return nil // no mark, as with something of it running
		}
		if again != nil {
			return fmt.Errorf("its mark couldn't be made (%v), and its mark folder is there but can't be read: %v", err, again)
		}
		if there != mark {
			return fmt.Errorf("its mark couldn't be made (%v), and its mark folder holds another, %s", err, there)
		}
	}
	log.Printf("sandbox: desk %s, made before marks, has one now: nothing of it was running", filepath.Base(own))
	return nil
}

// reapMarkIn is the name of the mark in the desk's own folder own: fs.ErrNotExist when it has no mark folder, an error
// when that isn't a folder holding one plain file named as a mark, and nothing else.
func reapMarkIn(own string) (string, error) {
	folder := filepath.Join(own, sandbox.ReapMarkFolder)
	info, err := os.Lstat(folder)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New(folder + " isn't a folder")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].Type().IsRegular() || !sandbox.ReapMarkValid(entries[0].Name()) {
		return "", errors.New(folder + " doesn't hold one mark alone")
	}
	return entries[0].Name(), nil
}

// reapPass is one pass of the reap: every process of the owner's account listed afresh, each looked at.
// Each pid answered as not in it goes with the start time its look read (0 when it couldn't).
type reapPass struct {
	in        []reapRun      // in the desk's sandbox, their runs taken
	unreached []int          // in it, their runs not taken
	whole     bool           // listed, and every process answered
	answered  map[int]uint64 // not in it, running whole throughout (reapNotIn)
	gone      map[int]uint64 // not in it, but being made, ending or ended meanwhile (reapGone)
	zombies   map[int]uint64 // ended, waiting for their parents (reapZombie)
}

// empty is whether the pass found none of the desk's, every process answered.
func (p reapPass) empty() bool { return p.whole && len(p.in) == 0 && len(p.unreached) == 0 }

// left is the pids the pass found in the desk's sandbox.
func (p reapPass) left() []int {
	left := slices.Clone(p.unreached)
	for _, run := range p.in {
		left = append(left, run.pid)
	}
	return left
}

// agrees is whether p, which found none of the desk's, agrees with before, the pass just before it, which found none
// too: every process p found gone before or during its look, or ended, before saw, the same process. Only then can no
// process of the desk's have made one this pass's list didn't have.
func (p reapPass) agrees(before *reapPass) bool {
	if before == nil || !p.empty() {
		return false
	}
	for _, ended := range []map[int]uint64{p.gone, p.zombies} {
		for pid, start := range ended {
			if !before.saw(pid, start) {
				return false
			}
		}
	}
	return true
}

// saw is whether p saw the process of pid and start time running whole outside the sandbox, or ended: a start time
// that wasn't read (0) names no process.
func (p reapPass) saw(pid int, start uint64) bool {
	return start != 0 && (p.answered[pid] == start || p.zombies[pid] == start)
}

// reapDesk ends (end) every process in the sandbox at at: SIGTERM to each run once,
// looking again every deskReapEvery, then, after deskReapGrace, SIGKILL to each run still there, looking again at once,
// until two passes in a row find none of it, the second with no process gone that the first didn't answer for (none,
// true), or deskReapWithin more has gone by. left is the pids found at the last pass; sure is false when that pass
// found none but none left couldn't be said (its processes couldn't be listed, macOS didn't answer for one, or the time
// ran out before a second pass agreed). Only looking (!end), it says at once which a pass found.
func reapDesk(at reapAt, end bool) (left []int, sure bool) {
	sys := deskReapSystem
	self := os.Getpid()
	look := func() (p reapPass) {
		pids, err := sys.list()
		if err != nil {
			log.Printf("sandbox: the reap couldn't list the processes: %v", err)
			return p
		}
		p.whole, p.answered, p.gone, p.zombies = true, map[int]uint64{}, map[int]uint64{}, map[int]uint64{}
		for _, pid := range pids {
			if pid == self {
				continue // the core: never in a desk's sandbox
			}
			switch run, verdict := sys.look(pid, at); verdict {
			case reapIn:
				p.in = append(p.in, run)
			case reapInUnreached:
				p.unreached = append(p.unreached, pid)
			case reapNotIn:
				p.answered[pid] = run.start
			case reapGone:
				p.gone[pid] = run.start
			case reapZombie:
				p.zombies[pid] = run.start
			default:
				p.whole = false
			}
		}
		return p
	}
	termed := map[reapRun]bool{}
	start := time.Now()
	var before *reapPass // the pass just before, when it found none
	for {
		p := look()
		// A process gone or ended at its look that the pass before didn't see, the same process, may have left a child
		// this pass's list didn't have: only a pass with none of those agrees with the one before.
		if p.agrees(before) {
			return nil, true
		}
		left := p.left()
		if !end && len(left) > 0 {
			return left, p.whole
		}
		waited := time.Since(start)
		if waited >= deskReapGrace+deskReapWithin {
			return left, len(left) > 0 && p.whole
		}
		if p.empty() {
			before = &p
			continue // the next pass at once: the less time between them, the fewer processes start and end meanwhile
		}
		before = nil
		killing := waited >= deskReapGrace
		for _, run := range p.in {
			sig := syscall.SIGKILL
			if !killing {
				if termed[run] {
					continue
				}
				sig, termed[run] = syscall.SIGTERM, true
			}
			// That one run only: one that has ended or run another program since is found again by the next look.
			if err := sys.signal(run, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
				log.Printf("sandbox: the reap couldn't signal process %d: %v", run.pid, err)
			}
		}
		// Once it kills, it looks again at once: a process that keeps forking is caught only by a look that comes
		// before its next fork, so the more looks, the sooner.
		if !killing {
			time.Sleep(deskReapEvery)
		}
	}
}

// reapLeftWords is why desk h's stop went no further, for the owner, to be followed by what that left as it was: the
// processes left in its sandbox, named as Activity Monitor shows them (the first two), or none known (a look that
// couldn't be made whole, or two passes that never agreed).
func reapLeftWords(h Hire, left []int) string {
	name := calledHire(h)
	if len(left) == 0 {
		return reapUnsure + name + " started has ended"
	}
	var which []string
	for _, pid := range left[:min(len(left), 2)] {
		if program := reapName(pid); program != "" {
			which = append(which, fmt.Sprintf("%s, process %d", program, pid))
		} else {
			which = append(which, fmt.Sprintf("process %d", pid))
		}
	}
	if more := len(left) - len(which); more > 0 {
		which = append(which, fmt.Sprintf("and %d more", more))
	}
	if len(left) == 1 {
		return "A program " + name + " started is still running in its sandbox and couldn't be ended (" + which[0] + ")"
	}
	return fmt.Sprintf("%d programs %s started are still running in its sandbox and couldn't be ended (%s)", len(left), name,
		strings.Join(which, "; "))
}

// firedReapRefusal ends a fire's reap refusal (hirefire.go), stopReapRefusal Take over's and restart's stop
// (t3takeover_tmux.go), through firedRefusal and stopRefusal. A refused restart or resume says its own
// (sandboxrestart.go, t3takeover.go). A retry helps when what was left ends meanwhile; a restart of the Mac ends
// whatever doesn't.
const (
	firedReapRefusal = ", so nothing was checked, merged or removed. Fire it again, and if this keeps happening, restart your Mac and then fire it."
	stopReapRefusal  = ", so nothing more was done. Try again, and if this keeps happening, restart your Mac and then try again."
	// startReapKept ends a start's or a watch's (hireengine.go, hiretmux.go), which has no one to refuse: its row stays.
	startReapKept = ", so its worktree and folders were kept. Fire it to end it, and if this keeps happening, restart your Mac and then fire it."
	// reapUnsure begins reapLeftWords' words when no program was named.
	reapUnsure = "Smolcorp couldn't make sure every program "
	// firedUnsure ends a fire's that went through when its last stop named no program (reapRestartWords): the hire is
	// fired, so its row is gone from Setup and can't be fired again there (RT126FIXREVIEW-FIREREAPR4 N1).
	firedUnsure = ". Restart your Mac to be sure."
)

// firedRefusal is a fire's refusal (hirefire.go) when the reap of desk h left why (deskReap), stopRefusal Take over's
// and restart's (t3takeover_tmux.go). A desk with no mark yet (deskMarkAfterProfile) is only looked at: what was found
// there stays running whatever the owner retries, so its words say what ends it, and what makes Smolcorp end it from
// then on (FXFIREREAPR6). A look that named nothing is the same as any desk's: a retry may find it empty.
func firedRefusal(h Hire, why string) string {
	if !reapLeftUnmarked(h, why) {
		return why + firedReapRefusal
	}
	return why + ", so nothing was checked, merged or removed. " + unmarkedWords(h, why) + "then fire " + calledHire(h) + "."
}

func stopRefusal(h Hire, why string) string {
	if !reapLeftUnmarked(h, why) {
		return why + stopReapRefusal
	}
	return why + ", so nothing more was done. " + unmarkedWords(h, why) + "then try again. Once " + calledHire(h) +
		" starts again with nothing of it running, Smolcorp will end such programs itself."
}

// unmarkedWords is why a program of desk h's that was named couldn't be ended (reapLeftWords' why), and what ends it.
func unmarkedWords(h Hire, why string) string {
	which := "it"
	if !strings.HasPrefix(why, "A program ") {
		which = "them"
	}
	name := calledHire(h)
	return name + " dates from an earlier version of Smolcorp, so Smolcorp can't end " + name + "'s programs itself: quit " +
		which + " in Activity Monitor or restart your Mac, "
}

// reapLeftUnmarked is whether why, the words of desk h's reap, named a program left in a desk with a profile and no mark
// (reapSandboxAt), which no reap signals.
func reapLeftUnmarked(h Hire, why string) bool {
	if runtime.GOOS != "darwin" || !plainHireID(h.ID) || strings.HasPrefix(why, reapUnsure) || strings.HasPrefix(why, mountedBegins) {
		return false
	}
	own := hireFolder(supportDir, h.ID)
	if _, err := os.Lstat(filepath.Join(own, "sandbox.sb")); err != nil {
		return false
	}
	_, err := reapMarkIn(own)
	return errors.Is(err, fs.ErrNotExist)
}

// firedStopNote is what a fire that went through says when its last stop couldn't show its desk empty (hirefire.go),
// why being reapLeftWords' words.
func firedStopNote(why string) string { return why + reapRestartWords(why, firedUnsure) }

// reapRestartWords ends why (reapLeftWords' words) where nothing is left to refuse: a fire that went through
// (hirefire.go: firedUnsure), a plain hire's stop (launcherKeptWords: launcherUnsure). A program named that couldn't be
// ended takes a restart of the Mac: "it", or "them" for more than one (FXFIREREAPR4 O2). None named means only that two
// looks in a row never agreed, which a burst of short-lived programs elsewhere on the Mac can cause for a few seconds,
// with nothing of the desk's left (RT126FIXREVIEW-FIREREAPR3 O1): unsure ends it then, what the owner can do.
func reapRestartWords(why, unsure string) string {
	switch {
	case strings.HasPrefix(why, reapUnsure):
		return unsure
	case strings.HasPrefix(why, "A program "):
		return ". Restart your Mac to end it."
	}
	return ". Restart your Mac to end them."
}

// reapWordsAfterColon is reapLeftWords' words to follow a colon: "a program…", not "A program…".
func reapWordsAfterColon(why string) string {
	if rest, ok := strings.CutPrefix(why, "A program "); ok {
		return "a program " + rest
	}
	return why
}
