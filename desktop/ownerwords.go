package desktop

import (
	"crypto/sha256"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// The owner-words ledger (MEM3, PL126M §1 "Provenance"): what the owner typed or said to the GM, kept so that a
// rule the GM proposes later (MEM4) can quote it, and the owner's tap (MEM5) can show where the words came from.
//
// A "user prompt" in the GM's session isn't proof of the owner: the core types there too (the GM's start message,
// a photo's prompt, a made button's text), and other programs can. So the ledger keeps two kinds of lines, each
// tagged:
//   - through Smolcorp: words typed into the GM's session (typeText) by a caller the owner's tap would let in
//     (accessOwnerTap): the Mac app's own office window, or a paired phone over Tailscale that isn't the device the
//     Windows window holds. The office in a browser (its key in the browser's history), the Windows window's device
//     (its key in a file any program here can read), any window off a Mac, and a phone over the Wi-Fi's plain http
//     keep nothing: their words are noted as the core's, so the prompt they make isn't kept as the terminal's either;
//   - typed in the GM's terminal: a UserPromptSubmit of the GM's session whose text the core didn't type itself.
//     Until A's X10 and M7, any program holding the shared hook key (every hire today) can send one, and a launcher
//     names its own role as it connects, so a program with the hook key can make its own session count as the GM's:
//     this tag is only as strong as that, and never becomes "through Smolcorp".
//
// Neither tag tells typed from pasted: a web page the owner pastes into the GM's terminal or the phone's box is
// their line, as anything they send is. ownerLineMost is the only screen.
//
// What the core knows isn't the owner's is noted by its hash, and a prompt with exactly that text is never kept:
// what it typed itself (coreTypes), mail, the GM's briefs, bot replies. Mail, briefs and bot text are never typed
// into the GM's session (mail_gate.go), so this only matters if something else does. What the core typed is kept
// apart from the rest, so mail a hire sends can't push its hashes out.
//
// Another desk's session is never recorded. Nothing here reaches a disk, the event log or a log line: it lives in
// this process alone, so a restart clears it, and lines go after a day, or sooner past ownerWordsMost.

const (
	// ownerWordsKept is how long a line, and a hash of words that aren't the owner's, is kept.
	ownerWordsKept = 24 * time.Hour
	// ownerWordsMost is how many lines are kept: the oldest goes first.
	ownerWordsMost = 200
	// ownerLineMost is the longest line kept, in bytes. A longer prompt is likelier a paste than a correction, and
	// the owner can say it again; the phone's and the window's words are shorter (phoneTextLongestBytes).
	ownerLineMost = 8 * 1024
	// notOwnerMost is how many hashes of words that aren't the owner's are kept, in each of the two sets: the oldest
	// goes first.
	notOwnerMost = 4096
	// ownerQuoteLeast is the shortest quote that counts, in characters: a word or two would be found anywhere.
	ownerQuoteLeast = 12
)

// ownerSource is where a line of the owner's came from.
type ownerSource string

const (
	ownerViaSmolcorp ownerSource = "smolcorp" // through Smolcorp, by a caller the owner's tap lets in: the Mac app's window or a phone over Tailscale
	ownerViaTerminal ownerSource = "terminal" // typed in the GM's own terminal, as its hook said
)

// ownerLine is one line of the ledger.
type ownerLine struct {
	Text   string
	Source ownerSource
	// From says it in a word, for the owner's card: "phone", "window" or "terminal".
	From    string
	Session string
	At      time.Time
}

// ownerWords is the ledger. Its zero value is ready; it has its own lock, taken with or without the Deck's, and
// never takes another while it holds its own.
type ownerWords struct {
	mu    sync.Mutex
	lines []ownerLine
	// notOwner is mail, briefs and bots' replies; typed what the core typed into the GM's session. Apart, so that
	// mail a hire sends can't push out what the core typed (RT126MEM3 N3).
	notOwner, typed map[[32]byte]time.Time
	// now is time.Now unless a test says otherwise.
	now func() time.Time
}

func (o *ownerWords) clock() time.Time {
	if o.now != nil {
		return o.now()
	}
	return time.Now()
}

// ownerPlain is text as the ledger compares it: its words, one space between them. A paste's line breaks, a
// terminal's trailing space and the like don't make words someone else's.
func ownerPlain(text string) string { return strings.Join(strings.Fields(text), " ") }

func ownerHash(text string) [32]byte { return sha256.Sum256([]byte(ownerPlain(text))) }

// forget drops what is past its day, with o.mu held.
func (o *ownerWords) forget(now time.Time) {
	cutoff := now.Add(-ownerWordsKept)
	kept := o.lines[:0]
	for _, line := range o.lines {
		if line.At.After(cutoff) {
			kept = append(kept, line)
		}
	}
	clear(o.lines[len(kept):])
	o.lines = kept
	for _, set := range []map[[32]byte]time.Time{o.notOwner, o.typed} {
		for hash, at := range set {
			if !at.After(cutoff) {
				delete(set, hash)
			}
		}
	}
}

// notOwners notes text the core knows isn't the owner's (mail, a brief, a bot's reply), so a prompt with exactly
// that text is never kept as theirs.
func (o *ownerWords) notOwners(text string) { o.note(&o.notOwner, text) }

// typedByCore notes text the core is about to type into the GM's session, as notOwners does, in a set of its own.
func (o *ownerWords) typedByCore(text string) { o.note(&o.typed, text) }

// note adds text's hash to a set, the oldest going first past notOwnerMost.
func (o *ownerWords) note(set *map[[32]byte]time.Time, text string) {
	if ownerPlain(text) == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.clock()
	o.forget(now)
	if *set == nil {
		*set = map[[32]byte]time.Time{}
	}
	if len(*set) >= notOwnerMost {
		var oldest [32]byte
		first := true
		for hash, at := range *set {
			if first || at.Before((*set)[oldest]) {
				oldest, first = hash, false
			}
		}
		delete(*set, oldest)
	}
	(*set)[ownerHash(text)] = now
}

// add keeps a line, with o.mu held. Text too long to keep, or with nothing in it, is left out.
func (o *ownerWords) add(line ownerLine) {
	if strings.TrimSpace(line.Text) == "" || len(line.Text) > ownerLineMost {
		return
	}
	line.At = o.clock()
	o.forget(line.At)
	if len(o.lines) >= ownerWordsMost {
		o.lines = append(o.lines[:0], o.lines[len(o.lines)-ownerWordsMost+1:]...)
	}
	o.lines = append(o.lines, line)
}

// heard keeps words typed into the GM's session through Smolcorp, by a caller the owner's tap lets in.
func (o *ownerWords) heard(from textFrom, session, words string) {
	where := "phone"
	if from == textFromWindow {
		where = "window"
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.add(ownerLine{Text: words, Source: ownerViaSmolcorp, From: where, Session: session})
}

// submitted keeps a prompt of the GM's session as typed in its terminal, unless the core knows the words aren't
// the owner's: it typed them itself (and kept them already when they were the owner's), or they are mail, a brief
// or a bot's reply.
func (o *ownerWords) submitted(session, prompt string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.forget(o.clock())
	hash := ownerHash(prompt)
	_, mail := o.notOwner[hash]
	_, typed := o.typed[hash]
	if mail || typed {
		return
	}
	o.add(ownerLine{Text: prompt, Source: ownerViaTerminal, From: "terminal", Session: session})
}

// quoted finds the newest line that holds quote, word for word, as whole words, and gives the line whole, for the
// owner's card to show what was around it. A quote shorter than ownerQuoteLeast characters is never found.
func (o *ownerWords) quoted(quote string) (ownerLine, bool) {
	quote = ownerPlain(quote)
	if len([]rune(quote)) < ownerQuoteLeast {
		return ownerLine{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.forget(o.clock())
	for i := len(o.lines) - 1; i >= 0; i-- {
		if holdsWords(ownerPlain(o.lines[i].Text), quote) {
			return o.lines[i], true
		}
	}
	return ownerLine{}, false
}

// holdsWords says whether quote is in text and starts and ends where words do: "deploy" isn't in "redeployed".
func holdsWords(text, quote string) bool {
	for from := 0; from < len(text); {
		i := strings.Index(text[from:], quote)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(quote)
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		after, _ := utf8.DecodeRuneInString(text[end:])
		first, size := utf8.DecodeRuneInString(quote)
		last, _ := utf8.DecodeLastRuneInString(quote)
		if !(start > 0 && isWordRune(before) && isWordRune(first)) && !(end < len(text) && isWordRune(after) && isWordRune(last)) {
			return true
		}
		from = start + size
	}
	return false
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// all is a copy of the lines kept, oldest first.
func (o *ownerWords) all() []ownerLine {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.forget(o.clock())
	return append([]ownerLine(nil), o.lines...)
}

// MARK: The Deck's side

// isGMSession is whether the session is the GM's, by the role its desk was started with: a GM the office hired,
// in tmux or a launcher, or one started by hand. With d.mu held.
func (d *Deck) isGMSession(id string) bool {
	s := d.sessions.Get(id)
	if s == nil {
		return false
	}
	role, ok := d.sessionRole(s)
	return ok && role.ID == gmRole
}

// coreTypes notes words the core is about to type into a tab as its own, where the tab holds the GM's session, so
// the prompt they make is never kept as typed in its terminal. Every way the core types into a desk calls it
// (TestEveryWayTheCoreTypesIntoADeskNotesIt): typeInto (a made button, a new conversation, /effort), typeText (the
// phone's, the window's and a photo's words) and typeDictation (the office's mic, which Claude may have tidied).
// heardOwner keeps the owner's words where they are. Without d.mu.
func (d *Deck) coreTypes(tab, words string) {
	d.mu.Lock()
	gm := slices.ContainsFunc(d.sessions.All(), func(s *Session) bool { return s.Tab == tab && d.isGMSession(s.ID) })
	d.mu.Unlock()
	if gm {
		d.owner.typedByCore(words)
	}
}

// heardOwner keeps the owner's words once typeText has put them into the GM's session: sent, or typed and not
// sent. Only where ownerTap says the owner's tap lets the caller in (accessOwnerTap, decided by the route): any
// other caller keeps nothing, and as typeText noted its words, nor does the prompt they make. Refused, or maybe
// typed, nothing is kept.
func (d *Deck) heardOwner(from textFrom, ownerTap bool, id, text string, status int, message string) {
	if !ownerTap || status >= 400 && message != from.typedNotSent() {
		return
	}
	words, ok := preparedPhoneText(text)
	if !ok {
		return
	}
	d.mu.Lock()
	gm := d.isGMSession(id)
	d.mu.Unlock()
	if gm {
		d.owner.heard(from, id, words)
	}
}

// heardInTerminal keeps a prompt of the GM's session, from its hook. With d.mu held.
func (d *Deck) heardInTerminal(event map[string]any) {
	if event["hook_event_name"] != "UserPromptSubmit" {
		return
	}
	id, _ := event["session_id"].(string)
	prompt, _ := event["prompt"].(string)
	if prompt != "" && d.isGMSession(id) {
		d.owner.submitted(id, prompt)
	}
}
