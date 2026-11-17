package desktop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// `smolcorp hold`: the paste hold (pastehold.go), for a test runner on the Mac Smolcorp runs on, as you.
//
//	smolcorp hold on [MINUTES]   hold what the office types into desks, for MINUTES (1 to 15, 15 if not said);
//	                             waits until the running copy holds and nothing it typed before is still going
//	smolcorp hold off            end it: what waited goes now, in order
//	smolcorp hold                whether a hold is on
//	smolcorp hold selftest       the hold's own code, on stand-ins in a temp folder: words held, then typed in order
//	                             once it ends, by hand and at its deadline (no desk, no key, no clipboard, no app)
//
// It writes the hold sealed with the running copy's hold key, a file in the support folder no desk's sandbox
// reaches. A hold can't be longer than 15 minutes nor lengthened, and the next one is kept a minute after the last
// ended: a test run longer than that runs its last part unheld. A button tapped on a desk still types (keys, not the
// clipboard).

// holdPoll is how often `hold on` looks for the running copy's word; the tests shorten holdWait. holdSystem is the
// system it runs on.
var (
	holdPoll   = 100 * time.Millisecond
	holdWait   = holdRest + holdSeenWait
	holdSystem = runtime.GOOS
)

// holdRunning is whether a copy of Smolcorp runs for this user, as quit finds it; the tests change it.
var holdRunning = func() bool {
	pid, ok := recordedPid()
	if !ok || processGone(pid) {
		return false
	}
	running, err := thisHomeRunning()
	return err == nil && running
}

const holdUsage = "Usage: smolcorp hold on [MINUTES] | smolcorp hold off | smolcorp hold selftest | smolcorp hold"

// holdNotHere is what `smolcorp hold` says off a Mac, where nothing holds.
const holdNotHere = "The hold is for a Mac's tests (their clipboard check): on this computer the office types into desks as it comes."

// RunHold is `smolcorp hold`.
func RunHold(args []string) int { return runHold(args, os.Stdout, time.Now) }

func runHold(args []string, out io.Writer, now func() time.Time) int {
	if strings.TrimSpace(os.Getenv("SMOLCORP_HIRE_KEY")) != "" {
		// Not the fence (the hold key is, and the guard): a hire is simply told it's not its to hold.
		fmt.Fprintln(out, "The hold is for you, not for a hire.")
		return 2
	}
	if len(args) == 1 && args[0] == "selftest" {
		return holdSelfTest(out)
	}
	if holdSystem != "darwin" {
		fmt.Fprintln(out, holdNotHere)
		return 2
	}
	switch {
	case len(args) == 0:
		key := readHoldKey()
		id, until, sealed := readHold(holdFile(), key)
		switch {
		case !sealed || !now().Before(until):
			fmt.Fprintln(out, "No hold: the office types into desks as it comes.")
		case readSeen(holdSeen(), key) == id:
			fmt.Fprintf(out, "Typing into desks is on hold until %s at the latest.\n", until.Format(time.TimeOnly))
		default:
			fmt.Fprintf(out, "A hold is asked until %s; Smolcorp hasn't said it holds.\n", until.Format(time.TimeOnly))
		}
		return 0
	case args[0] == "off" && len(args) == 1:
		for _, file := range []string{holdFile(), holdSeen()} {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(out, "The hold couldn't be ended (%v).\n", err)
				return 1
			}
		}
		fmt.Fprintln(out, "Hold ended: what waited goes now, in order.")
		return 0
	case args[0] != "on" || len(args) > 2:
		fmt.Fprintln(out, holdUsage)
		return 2
	}
	minutes := int(holdLongest / time.Minute)
	if len(args) == 2 {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > minutes {
			fmt.Fprintf(out, "A hold lasts 1 to %d minutes.\n", minutes)
			return 2
		}
		minutes = n
	}
	key := readHoldKey()
	if key == "" || !holdRunning() {
		fmt.Fprintln(out, "Smolcorp isn't running: nothing to hold.")
		return 0
	}
	id, until := randomKey(), now().Add(time.Duration(minutes)*time.Minute)
	if err := writePrivate(holdFile(), holdRecord(key, id, until)); err != nil {
		fmt.Fprintf(out, "The hold couldn't be set (%v).\n", err)
		return 1
	}
	for deadline := time.Now().Add(holdWait); time.Now().Before(deadline); time.Sleep(holdPoll) {
		if readSeen(holdSeen(), key) == id {
			fmt.Fprintf(out, "Typing into desks is on hold until %s at the latest: nothing is typing now.\n", until.Format(time.TimeOnly))
			return 0
		}
	}
	fmt.Fprintf(out, "Smolcorp didn't say it holds within %d s (a version before 1.26, busy typing, or a hold ended less than a "+
		"minute ago): the hold stays asked until %s.\n", int(holdWait.Seconds()), until.Format(time.TimeOnly))
	return 1
}

// holdSelfTest runs the hold's own code (pastehold.go) on stand-ins, in a temp folder of its own with a key of its
// own, removed after: two stand-in deliveries of fake words are held, said seen, then typed in the order they came
// once the hold ends; a third goes on its own at the hold's deadline. Nothing here reaches the running copy, a desk,
// a key of the running copy's or the clipboard.
func holdSelfTest(out io.Writer) int {
	dir, err := os.MkdirTemp("", "smolcorp-hold-selftest-")
	if err != nil {
		fmt.Fprintf(out, "FAIL: no temp folder (%v)\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	h := pasteHold{key: randomKey(), file: filepath.Join(dir, "paste-hold"), seenFile: filepath.Join(dir, "paste-hold-seen")}
	var typed []string
	// send is a stand-in delivery of words: held, or typed at once, as typeText does.
	send := func(words string) string {
		w := &heldTyping{desk: "stand-in", try: func() (bool, string) { typed = append(typed, words); return false, "" }}
		says := h.held(w)
		if says == "" {
			w.try()
			h.end()
		}
		return says
	}
	set := func(key, id string, until time.Time) bool {
		return writePrivate(h.file, holdRecord(key, id, until)) == nil
	}
	failed := 0
	row := func(ok bool, what string) {
		if ok {
			fmt.Fprintln(out, "PASS:", what)
		} else {
			failed++
			fmt.Fprintln(out, "FAIL:", what)
		}
	}
	now := time.Now()
	row(set("not-the-key", "selftest-0", now.Add(time.Minute)), "a hold sealed with another key")
	h.look(now)
	row(send("fake words 0") == "" && len(typed) == 1, "holds nothing")
	typed = nil
	row(set(h.key, "selftest-1", now.Add(time.Minute)), "a hold is set, as `smolcorp hold on` sets it")
	h.look(now)
	row(readSeen(h.seenFile, h.key) == "selftest-1", "the hold is said seen")
	row(send("fake words 1") != "" && send("fake words 2") != "" && !h.enter(), "two stand-in deliveries wait, and mail would")
	h.look(now.Add(time.Second))
	row(len(typed) == 0, "nothing is typed while the hold is on")
	_ = os.Remove(h.file)
	h.look(now.Add(2 * time.Second))
	row(strings.Join(typed, "|") == "fake words 1|fake words 2", "once it ends, both are typed, in the order they came")
	typed = nil
	row(set(h.key, "selftest-2", now.Add(2*time.Minute)), "a second hold, right after")
	h.look(now.Add(3 * time.Second))
	row(send("fake words 3") == "" && len(typed) == 1, "isn't kept within a minute of the last")
	typed = nil
	later := now.Add(3*time.Second + holdRest)
	h.look(later)
	row(send("fake words 4") != "", "a minute on, it is: a fourth waits")
	h.look(now.Add(2*time.Minute + time.Second))
	row(len(typed) == 1, "at the deadline it's typed, with nobody ending the hold")
	row(set(h.key, "selftest-3", now.Add(24*time.Hour)), "a hold that says a day")
	far := now.Add(2*time.Minute + time.Second + holdRest)
	h.look(far)
	h.look(far.Add(holdLongest))
	on, _, _, _, _ := h.state(far)
	row(!on, "ends within 15 minutes all the same")
	if failed > 0 {
		fmt.Fprintf(out, "The hold's self-test: %d failed.\n", failed)
		return 1
	}
	fmt.Fprintln(out, "The hold's self-test passed.")
	return 0
}
