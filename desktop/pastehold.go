package desktop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The paste hold (PASTEHOLD1): while tests run on this Mac (the Mac Studio's `swift test`, whose clipboard guard,
// CLIP1, fails on any copy made during the run, the office's own paste into a desk included), the owner's test runner
// holds what the office types into desks: the phone's words (/dictate), the window's (/talk/say), a photo's path, the
// Mac's dictation, and mail to hires. Each waits, and goes once the hold ends, in the order it came for its desk: by
// `smolcorp hold off`, or on its own at its deadline, never more than holdLongest after the core began to hold, so a
// runner that crashed can't hold for ever. A hold can't be lengthened, and the next one is kept only once typing has
// gone on for holdRest: nothing waits longer than holdLongest because of holds, whoever sets them. A tap on a desk's
// buttons (/act, effort, a new conversation) types keys, not the clipboard, and isn't held.
//
// Only on a Mac, under the Mac app: Linux and Windows have no clipboard check to keep from, and no sandbox for a desk
// in 1.26, so there nothing holds. The switch is a file in the support folder, paste-hold, "<deadline, in Unix seconds>
// <the hold's id> <seal>", sealed with the hold key: new at each start of the core, in its memory and in hold-key, a
// file only the owner's account can read, which every desk's sandbox denies with the rest of the folder. A file
// without the seal, or sealed with an old key, holds nothing: a desk that can write there but not read the key can't
// hold. (A program of yours outside every sandbox reads the key as it reads Smolcorp's other keys, SANDBOX-PLAN §A5.3;
// the guard refuses a hire `smolcorp hold` and the key's file by name.) The words that wait are kept in memory only,
// never on disk; mail waits in its outbox, as it waits for a busy desk. The core looks at the file every
// holdLookEvery. Once it holds, and nothing it began typing before is still under way, it writes paste-hold-seen with
// the hold's id, sealed too: `smolcorp hold on` waits for that, so the tests start only once no paste can come.
//
// Once the hold ends, words a desk refuses because it's asking you something wait for it, for as long as it asks;
// words refused for good (the desk gone, the phone's words switched off) aren't typed, and Setup says so, by desk.

const (
	holdLongest   = 15 * time.Minute
	holdLookEvery = time.Second
	// holdMacSettle: a paste through the Mac's clipboard may put back what you had copied this long after it answered
	// (ClipboardPaste.grace and readWait): the hold is said seen only once that time has passed.
	holdMacSettle = 12 * time.Second
	// holdRest: after a hold ends, typing goes on at least this long before another hold is kept.
	holdRest = time.Minute
	// holdSeenWait is how long `smolcorp hold on` waits for the running copy to say it holds, past holdRest.
	holdSeenWait = 30 * time.Second
	// holdNoteLasts is how long Setup says which held words weren't typed.
	holdNoteLasts = time.Hour
)

var (
	holdFile    = func() string { return filepath.Join(supportDir, "paste-hold") }
	holdSeen    = func() string { return filepath.Join(supportDir, "paste-hold-seen") }
	holdKeyFile = func() string { return filepath.Join(supportDir, "hold-key") }
)

// heldWords is what the phone and the window are told of words that wait (202): they'll go, so the page doesn't
// offer them again. heldBehind: the hold has ended, and words that waited for this desk go first.
const (
	heldWords     = "On hold while tests run on this computer: typed, in order, when they end (in 15 minutes at most)."
	heldBehind    = "Typed after the words that waited for this desk during the tests, in order."
	dictationHeld = "On hold while tests run on this Mac: what you said will be typed when they end."
)

// newHoldKey is a new hold key, written to its file for `smolcorp hold`: under the Mac app only.
func newHoldKey() string {
	key := randomKey()
	_ = writePrivate(holdKeyFile(), []byte(key+"\n"))
	return key
}

// readHoldKey is the hold key from its file, "" for none.
func readHoldKey() string {
	data, _ := os.ReadFile(holdKeyFile())
	return strings.TrimSpace(string(data))
}

// holdSeal is key's seal on what.
func holdSeal(key, what string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(what))
	return hex.EncodeToString(mac.Sum(nil))
}

// holdRecord is a hold as `smolcorp hold on` writes it, sealed with key.
func holdRecord(key, id string, until time.Time) []byte {
	what := fmt.Sprintf("%d %s", until.Unix(), id)
	return []byte(what + " " + holdSeal(key, "paste-hold "+what) + "\n")
}

// seenRecord is the core's word that it holds id, sealed with key.
func seenRecord(key, id string) []byte {
	return []byte(id + " " + holdSeal(key, "paste-hold-seen "+id) + "\n")
}

// readHold reads the hold's file at path: its id and deadline, if it's a plain file sealed with key. Anything else,
// and any file with no key to check it by, holds nothing.
func readHold(path, key string) (id string, until time.Time, ok bool) {
	fields, ok := sealedFields(path, key, 3)
	if !ok {
		return "", time.Time{}, false
	}
	seconds, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || !constantTimeEqual(fields[2], holdSeal(key, "paste-hold "+fields[0]+" "+fields[1])) {
		return "", time.Time{}, false
	}
	return fields[1], time.Unix(seconds, 0), true
}

// readSeen is the id the core said it holds, at path, if sealed with key; "" otherwise.
func readSeen(path, key string) string {
	fields, ok := sealedFields(path, key, 2)
	if !ok || !constantTimeEqual(fields[1], holdSeal(key, "paste-hold-seen "+fields[0])) {
		return ""
	}
	return fields[0]
}

func sealedFields(path, key string, n int) ([]string, bool) {
	if key == "" {
		return nil, false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 512 {
		return nil, false
	}
	data, _ := os.ReadFile(path)
	fields := strings.Fields(string(data))
	return fields, len(fields) == n
}

// heldTyping is words that wait for a desk (its tab, and its name as Setup says it), since at. try types them: wait
// is true while the desk asks you something; refused is why they weren't typed, "" once typed.
type heldTyping struct {
	desk, name string
	try        func() (wait bool, refused string)
	at         time.Time
}

// holdNote is held words a desk refused once the hold ended, for Setup: never the words themselves.
type holdNote struct {
	name, why string
	at        time.Time
}

// pasteHold is the hold as the core follows it. The zero value holds nothing.
type pasteHold struct {
	// key seals the hold, made as the deck starts under the Mac app: "" elsewhere, where nothing holds. file and
	// seenFile are its deck's (holdFile and holdSeen when ""). None changes after the start.
	key            string
	file, seenFile string
	mu             sync.Mutex
	on             bool
	id             string
	until, started time.Time
	// ended is when the last hold ended, spent its id: never kept again.
	ended time.Time
	spent string
	seen  string // the id last said seen
	// waiting is the words that wait, in the order they came.
	waiting []*heldTyping
	notes   []holdNote
	// typing is typing under way, let through before the hold: the hold isn't said seen until it's done.
	typing int
	// macPasted is when a paste through the Mac app last answered (see holdMacSettle).
	macPasted time.Time
	// delivering: one delivery at a time, so nothing that waited is typed twice.
	delivering bool
}

// held makes w wait while the hold is on, or while words that waited for its desk still wait (they go first): what
// the sender is told. Else "", and the typing is under way until end.
func (h *pasteHold) held(w *heldTyping) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.on || h.waitsFor(w.desk) {
		w.at = time.Now()
		h.waiting = append(h.waiting, w)
		if h.on {
			return heldWords
		}
		return heldBehind
	}
	h.typing++
	return ""
}

func (h *pasteHold) waitsFor(desk string) bool {
	for _, w := range h.waiting {
		if w.desk == desk {
			return true
		}
	}
	return false
}

// enter is held for mail, which waits in its outbox: false while the hold is on; else under way until end.
func (h *pasteHold) enter() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.on {
		return false
	}
	h.typing++
	return true
}

func (h *pasteHold) end() {
	h.mu.Lock()
	h.typing--
	h.mu.Unlock()
}

// pastedOnMac notes a paste through the Mac app that has answered.
func (h *pasteHold) pastedOnMac() {
	h.mu.Lock()
	h.macPasted = time.Now()
	h.mu.Unlock()
}

// state is the hold as Setup shows it at now: the notes of held words not typed, from holdNoteLasts at most.
func (h *pasteHold) state(now time.Time) (on bool, until, started time.Time, waiting int, notes []holdNote) {
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.notes[:0]
	for _, n := range h.notes {
		if now.Sub(n.at) < holdNoteLasts {
			kept = append(kept, n)
		}
	}
	h.notes = kept
	return h.on, h.until, h.started, len(h.waiting), append([]holdNote(nil), kept...)
}

// look takes the hold's file at now. A hold is kept from the first look that finds it, sealed, before its deadline,
// holdRest after the last one ended, and not spent; it ends at its deadline or holdLongest after that look, whichever
// is first, or as soon as its file says otherwise. Once nothing let through before it is still typing, it is said
// seen; once it ends, what waits is typed.
func (h *pasteHold) look(now time.Time) {
	file, seenFile := h.file, h.seenFile
	if file == "" {
		file, seenFile = holdFile(), holdSeen()
	}
	id, until, sealed := readHold(file, h.key)
	h.mu.Lock()
	was := h.on
	switch {
	case was && sealed && id == h.id:
		h.until = earlierOf(h.until, until)
		h.on = now.Before(h.until)
	case was:
		h.on = false
	case sealed && id != h.spent && now.Before(until) && (h.ended.IsZero() || now.Sub(h.ended) >= holdRest):
		h.on, h.id, h.started, h.until = true, id, now, earlierOf(until, now.Add(holdLongest))
	}
	on := h.on
	if was && !on {
		h.ended, h.spent = now, h.id
	}
	seen := on && h.seen != h.id && h.typing == 0 && now.Sub(h.macPasted) >= holdMacSettle
	id, until, waiting := h.id, h.until, len(h.waiting)
	h.mu.Unlock()
	switch {
	case on && !was:
		fmt.Fprintf(os.Stderr, "smolcorp: typing into desks is on hold until %s (smolcorp hold)\n", until.Format(time.TimeOnly))
	case was && !on:
		fmt.Fprintf(os.Stderr, "smolcorp: the hold has ended: %d waiting go now, in order\n", waiting)
	}
	if seen && writePrivate(seenFile, seenRecord(h.key, id)) == nil {
		h.mu.Lock()
		h.seen = id
		h.mu.Unlock()
	}
	if !on {
		h.deliver(now)
	}
}

// deliver types what waits while the hold stays off, oldest first. Words for a desk that asks you something wait
// for it, and so do the words that came after them for that desk; other desks' go. Words refused for good are let
// go, and Setup says which desk refused them, and why.
func (h *pasteHold) deliver(now time.Time) {
	h.mu.Lock()
	if h.delivering {
		h.mu.Unlock()
		return
	}
	h.delivering = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.delivering = false
		h.mu.Unlock()
	}()
	asking := map[string]bool{}
	for {
		h.mu.Lock()
		var next *heldTyping
		for _, w := range h.waiting {
			if !asking[w.desk] {
				next = w
				break
			}
		}
		if h.on || next == nil {
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()
		wait, refused := next.try()
		h.mu.Lock()
		if wait {
			asking[next.desk] = true
		} else {
			for i, w := range h.waiting {
				if w == next {
					h.waiting = append(h.waiting[:i:i], h.waiting[i+1:]...)
					break
				}
			}
			if refused != "" {
				h.notes = append(h.notes, holdNote{name: next.name, why: refused, at: now})
			}
		}
		h.mu.Unlock()
		if refused != "" {
			fmt.Fprintln(os.Stderr, "smolcorp: words held for a desk weren't typed once the hold ended: Setup says which")
		}
	}
}

// holdChecks are Setup's lines while a hold is on, or words from one still wait, and for held words a desk refused.
func (d *Deck) holdChecks() []check {
	on, until, started, waiting, notes := d.hold.state(time.Now())
	var checks []check
	if on || waiting > 0 {
		detail := fmt.Sprintf("The tests have ended: %d still waiting go as their desks are ready (a desk asking you something gets "+
			"them once you've answered).", waiting)
		if on {
			detail = fmt.Sprintf("Tests are running on this Mac, so what the office types into desks waits until they end, %s at the latest: "+
				"%d waiting now. Mail waits in its outbox. Everything then goes, in the order it came. Set at %s with smolcorp hold, "+
				"by a program of yours outside every desk's sandbox; smolcorp hold off ends it.", d.deliveryClock(until), waiting, d.deliveryClock(started))
		}
		checks = append(checks, check{ID: "pastehold", Status: checkInfo, Title: "Typing into desks is on hold", Detail: detail})
	}
	if len(notes) > 0 {
		var said []string
		for _, n := range notes {
			said = append(said, fmt.Sprintf("%s, %s: %s", n.name, d.deliveryClock(n.at), n.why))
		}
		checks = append(checks, check{ID: "pastehold-missed", Status: checkInfo, Title: "Words held during tests weren't typed",
			Detail: "Once the tests ended, these desks didn't take what waited for them, so it wasn't typed. Say it again if " +
				"it's still needed. " + strings.Join(said, " · ")})
	}
	return checks
}

// holdDeskName is session id's desk as Setup names it.
func (d *Deck) holdDeskName(id string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s := d.sessions.Get(id); s != nil && s.Name() != "" {
		return s.Name()
	}
	return "a desk"
}

// holdTabName is the desk in tab as Setup names it.
func (d *Deck) holdTabName(tab string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.sessions.All() {
		if s.Tab == tab && s.Name() != "" {
			return s.Name()
		}
	}
	return "a desk"
}

func earlierOf(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
