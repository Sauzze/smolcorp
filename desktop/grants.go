package desktop

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// The grant record (1.26's F-S, piece FS2): what the gates (gates.go) can't settle alone, an ask the owner answers.
// A GM, an MCP key, a night job or a hire that decide() says must wait for the owner's OK leaves an ask here; the
// owner's own tap, and nothing else, grants or refuses it; and the same actor asking the same thing again finds its
// grant, once. Silence is a no: an ask nobody answered, or answered too late, lets nothing through.
//
// The file, grants.jsonl in the support folder, holds one ask a line, written whole to a file beside it and put in
// its place (never half written), mode 0600, at most grantMaxCount asks and grantMaxBytes. Each line carries a
// signature made with a key this app holds in memory alone, new at each start: nothing on the disk can sign a line,
// so a line written or edited by anyone else, the asker too, is skipped with a word and never taken as a grant, and
// a restart leaves every ask and grant from before it void (silence is a no). The asks used or told are kept in
// memory as well, so an old copy of the file put back can't make a grant good twice. The guard's names
// (internal/guard) and a hire's fence (hiresettings.go) keep the GM and a hire away from the file besides.
//
// What the owner is shown is what is granted, whole: an ask whose text is too long to show, or has a character that
// hides or moves text (a control, a line break, a direction mark, an invisible one), is a no and never an ask
// (unshowable). The asker's text is data, shown as text, never markup.

const (
	// grantTTL is how long an ask waits for its answer, and a grant for its use.
	grantTTL = 12 * time.Hour
	// grantKeep is how long a finished ask (used, told, or run out) stays in the file, for the owner to look back.
	grantKeep = 24 * time.Hour

	// grantCoolDown is how long a no the asker was told stands before the same ask may be put to the owner again: a
	// loop can't nag the owner with a new ask after each no.
	grantCoolDown = 10 * time.Minute

	grantMaxCount = 200
	// grantMaxBytes holds grantMaxCount asks of the longest text, answered (TestAFullFileOfTheLongestAsksIsAnswered).
	grantMaxBytes = 1 << 20
	// grantMaxOpenEach is how many asks one actor may have waiting: one loop asking forever never fills the file.
	grantMaxOpenEach = 20

	// The most bytes an ask's what, amount and asker may have, each shown whole: past them it is a no, never an ask.
	grantWhatMost   = 2000
	grantAmountMost = 100
	grantActorMost  = 100
)

// An ask's state: open (waits), granted or refused (answered, not yet used or told), used (a grant spent), closed
// (a refusal told to the asker).
const (
	grantOpen    = "open"
	grantGranted = "granted"
	grantRefused = "refused"
	grantUsed    = "used"
	grantClosed  = "closed"
)

// The owner's channels, as an answer names them: the setup page's cookie (actorPage), a paired phone's key and the
// Mac app's office window's key (never the office in a browser, nor a window off a Mac: accessOwnerTap). The setup key a tool holds, the GM's token, an MCP key, a hire's key and a night job are
// none of them.
const (
	actorPhone  actor = "phone"
	actorWindow actor = "window"
)

// isOwnerTap is whether an answer comes by one of the owner's own channels.
func isOwnerTap(by actor) bool { return by == actorPage || by == actorPhone || by == actorWindow }

// label is the actor as the record shows it. An MCP key is a secret, so the record, and the approvals list the GM
// reads, hold a short fingerprint of it instead.
func (a actor) label() string {
	if a.kind() == "mcp" {
		return "mcp:" + a.keyHash()[:8]
	}
	return string(a)
}

// identity is the actor as an ask is told apart by it: an MCP key by all of its hash, so two keys that share a
// fingerprint never share a grant, and the key itself is never written.
func (a actor) identity() string {
	if a.kind() == "mcp" {
		return "mcp:" + a.keyHash()
	}
	return string(a)
}

func (a actor) keyHash() string {
	_, key, _ := strings.Cut(string(a), ":")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// grantRecord is one ask, one line of the file.
type grantRecord struct {
	ID string `json:"id"`
	// Who asked (label), what for and how much, to show; Key tells the ask from every other by all of that.
	Actor  string `json:"actor"`
	What   string `json:"what"`
	Amount string `json:"amount,omitempty"`
	Key    string `json:"key"`
	// Milliseconds: when it was asked, when it runs out, when it was answered, when it was used or told.
	At       int64  `json:"at"`
	Expires  int64  `json:"expires"`
	Answered int64  `json:"answered,omitempty"`
	Ended    int64  `json:"ended,omitempty"`
	State    string `json:"state"`
	// By is the owner's channel that answered: page, phone or window.
	By  string `json:"by,omitempty"`
	Sig string `json:"sig"`
}

var grantIDForm = regexp.MustCompile(`^g[0-9a-f]{16}$`)
var grantKeyForm = regexp.MustCompile(`^[0-9a-f]{64}$`)

// grantKey tells an ask from every other: the actor (its identity), what and the amount, whole, as the record shows
// them: nothing is cut, so a grant covers no more than the owner was shown.
func grantKey(identity, what, amount string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{identity, what, amount}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// unshowable says why text can't be shown to the owner whole and as it is, or "": longer than most bytes, not
// UTF-8, a character that hides or moves text (a control, a line break, a direction mark, an invisible or odd space,
// a blank that isn't a space or is drawn as nothing, a filler), spaces that push the rest out of sight (at an end, or
// more than three in a row), a mark drawn over a letter, letters of look-alike scripts in one word (RT126FS2b N3), more
// than three marks on a letter, a letter in a styled or wide form, or a Cyrillic or Greek word that reads as a Latin
// one beside Latin text (RT126FS2c B2, nits 1 and 2).
func unshowable(name, text string, most int) string {
	switch {
	case len(text) > most:
		return fmt.Sprintf("its %s is longer than the %d bytes the owner is shown", name, most)
	case !utf8.ValidString(text):
		return fmt.Sprintf("its %s isn't text", name)
	case strings.HasPrefix(text, " ") || strings.HasSuffix(text, " ") || strings.Contains(text, "    "):
		return fmt.Sprintf("its %s has spaces that could hide the rest", name)
	}
	marks := 0
	for _, c := range text {
		if unicode.Is(unicode.M, c) {
			marks++
		} else {
			marks = 0
		}
		switch {
		case hidesText(c):
			return fmt.Sprintf("its %s has a character the owner can't see as it is (U+%04X)", name, c)
		case overlays(c):
			return fmt.Sprintf("its %s has a mark drawn over a letter, which can make it read as another (U+%04X)", name, c)
		case marks > mostMarks:
			return fmt.Sprintf("its %s stacks more than %d marks on a letter, which can draw over the text around it", name, mostMarks)
		case styledLetter(c):
			return fmt.Sprintf("its %s has a letter in a styled or wide form, which can read as the plain one (U+%04X)", name, c)
		}
	}
	if c := mixedScripts(text); c != 0 {
		return fmt.Sprintf("its %s mixes letters of look-alike scripts (Latin, Greek, Cyrillic and others) in one word, which can read as other text (U+%04X)", name, c)
	}
	if c := latinLookAlikeWord(text); c != 0 {
		return fmt.Sprintf("its %s has a word in Cyrillic or Greek letters that look like Latin ones beside Latin text, which can read as other text (U+%04X)", name, c)
	}
	return ""
}

// mostMarks is how many combining marks one letter may carry: enough for Vietnamese (ệ written out is two) and
// a little more, never a stack tall enough to draw over the rows next to it (RT126FS2c nit 2).
const mostMarks = 3

// styledLetter is a letter or digit in a form that NFKC folds to a plain one, and that reads as it (RT126FS2c nit
// 1): fullwidth ones (ｍain, U+FF10 to U+FF5A) and the mathematical alphanumerics (ma𝗂n, U+1D400 to U+1D7FF), which
// are of no script, so mixedScripts can't see them.
func styledLetter(c rune) bool {
	return c >= 0xFF10 && c <= 0xFF5A && unicode.In(c, unicode.L, unicode.N) || c >= 0x1D400 && c <= 0x1D7FF
}

// overlays is a combining mark drawn over the letter before it (Unicode's canonical combining class 1, Overlay):
// "ma\u0338in" reads as a struck-out main.
func overlays(c rune) bool {
	for _, r := range [][2]rune{{0x0334, 0x0338}, {0x1CD4, 0x1CD4}, {0x1CE2, 0x1CE8}, {0x20D2, 0x20D3}, {0x20D8, 0x20DA},
		{0x20E5, 0x20E6}, {0x20EA, 0x20EB}, {0x10A39, 0x10A39}, {0x16AF0, 0x16AF4}, {0x1BC9E, 0x1BC9E}, {0x1D167, 0x1D169}} {
		if c >= r[0] && c <= r[1] {
			return true
		}
	}
	return false
}

// lookAlikeScripts are the scripts whose letters pass for one another's: a Cyrillic а in exаmple.com, a Coptic ⲟ in
// gⲟⲟgle.com, a Lisu ꓮ in ꓮpple (RT126FS2c nit 1).
var lookAlikeScripts = []*unicode.RangeTable{unicode.Latin, unicode.Greek, unicode.Cyrillic, unicode.Armenian, unicode.Cherokee,
	unicode.Coptic, unicode.Lisu}

// latinLookAlikes are the Cyrillic and Greek letters drawn as a Latin one, from Unicode's confusables: а с е о р х у
// ѕ і ј ӏ ԁ ԛ ԝ һ ү and their capitals that are, and Greek ο ν ι α and the capitals Α Β Ε Ζ Η Ι Κ Μ Ν Ο Ρ Τ Υ Χ.
var latinLookAlikes = []rune("асеорхуѕіјӏԁԛԝһүАВЕКМНОРСТХЅІЈԚԜҮοιναΑΒΕΖΗΙΚΜΝΟΡΤΥΧ")

// latinLookAlikeWord is the first letter of a word wholly of latinLookAlikes, in a text that has Latin letters too,
// or 0: a whole-word аррӏе.com reads as apple.com (RT126FS2c nit 1). A Cyrillic or Greek text with no Latin
// letter, and a word with one letter that looks like no Latin one (Москва's к and в), is plain text.
func latinLookAlikeWord(text string) rune {
	if !strings.ContainsFunc(text, func(c rune) bool { return unicode.Is(unicode.Latin, c) }) {
		return 0
	}
	first, all := rune(0), false
	for _, c := range text + " " {
		if unicode.In(c, unicode.L, unicode.M, unicode.N) {
			if first == 0 {
				first, all = c, true
			}
			all = all && slices.Contains(latinLookAlikes, c)
			continue
		}
		if first != 0 && all {
			return first
		}
		first = 0
	}
	return 0
}

// mixedScripts is the first letter of a word (a run of letters, marks and digits) whose letters come from two of
// lookAlikeScripts, or 0. Each script alone, and any other mix (Japanese with a Latin name), is plain text.
func mixedScripts(text string) rune {
	first := -1
	for _, c := range text {
		if !unicode.In(c, unicode.L, unicode.M, unicode.N) {
			first = -1
			continue
		}
		for i, script := range lookAlikeScripts {
			if !unicode.Is(script, c) {
				continue
			}
			if first >= 0 && first != i {
				return c
			}
			first = i
		}
	}
	return 0
}

// over is whether the ask is done with: spent, told, or run out.
func (r grantRecord) over(now int64) bool {
	return r.State == grantUsed || r.State == grantClosed || r.Expires <= now
}

// endedAt is when it was done with.
func (r grantRecord) endedAt() int64 {
	if r.State == grantUsed || r.State == grantClosed {
		return r.Ended
	}
	return r.Expires
}

var (
	errNotOwner   = errors.New("Only your own tap answers an ask: the setup page, a paired phone over Tailscale, or the office's window on your Mac.")
	errNoAsk      = errors.New("There is no such ask.")
	errAnswered   = errors.New("That ask is answered already.")
	errRunOut     = errors.New("That ask ran out. Silence is a no, so it is one.")
	errGrantsFull = errors.New("Too many asks wait for the owner.")
)

// grantEnd is how an ask was done with, kept in memory: a used grant or a told refusal, and the ask's key, until it
// would run out and its cool-down is over.
type grantEnd struct {
	state, key     string
	ended, expires int64
}

// grantStore keeps the record. One mutex orders every read and write of the file; it never takes the deck's lock,
// so a gate can ask it with that lock held (hire.go).
type grantStore struct {
	mu   sync.Mutex
	file string
	now  func() time.Time
	// word says what was skipped or couldn't be done; each word once.
	word func(string)
	told map[string]bool
	// key signs the lines: made at the first use, in memory alone, never written.
	key []byte
	// saved is whether this run has written the file: until it has, what is there is from before this start.
	saved bool
	// ended is the asks used or told by this run, by id: a line that says otherwise is an old copy. A no told is
	// cooled down from here, so an old copy without it can't skip the cool-down.
	ended map[string]grantEnd
}

func newGrantStore(file string) *grantStore {
	return &grantStore{file: file, now: time.Now, told: map[string]bool{}, ended: map[string]grantEnd{},
		word: func(text string) { log.Printf("smolcorp: the owner's asks: %s", text) }}
}

func (s *grantStore) say(text string) {
	if !s.told[text] {
		s.told[text] = true
		s.word(text)
	}
}

// signKey is this run's key, made now if there is none yet. It is never written: no file can sign a line.
func (s *grantStore) signKey() ([]byte, error) {
	if s.key != nil {
		return s.key, nil
	}
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, err
	}
	s.key = fresh
	return s.key, nil
}

func (s *grantStore) sign(r grantRecord) string {
	r.Sig = ""
	mac := hmac.New(sha256.New, s.key)
	mac.Write(recordLine(r))
	return hex.EncodeToString(mac.Sum(nil))
}

// recordLine is a record as one line of JSON, the asker's text as it is (no <, > or & made six bytes long).
func recordLine(r grantRecord) []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(r)
	return bytes.TrimSuffix(out.Bytes(), []byte("\n"))
}

// load reads the file's asks. A line that isn't one this app wrote (not whole, not signed with this folder's key, an
// id twice, a state or a time that can't be) is skipped with a word. A file bigger than it is ever written is not
// read at all.
func (s *grantStore) load() ([]grantRecord, error) {
	if _, err := s.signKey(); err != nil {
		return nil, err
	}
	file, err := os.Open(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, grantMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > grantMaxBytes {
		s.say("grants.jsonl is bigger than Smolcorp ever writes it, so none of it is used")
		return nil, nil
	}
	if !s.saved {
		if len(bytes.TrimSpace(data)) > 0 {
			s.say("grants.jsonl is from before this start, so its asks and grants are void: silence is a no")
		}
		return nil, nil
	}
	var records []grantRecord
	seen := map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		skip := func(why string) { s.say(fmt.Sprintf("grants.jsonl line %d is skipped: %s", i+1, why)) }
		var r grantRecord
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&r); err != nil || decoder.More() {
			skip("it isn't a whole record")
			continue
		}
		switch {
		case !hmac.Equal([]byte(r.Sig), []byte(s.sign(r))):
			skip("its signature doesn't match, so it was changed or isn't this folder's")
		case !grantIDForm.MatchString(r.ID) || !grantKeyForm.MatchString(r.Key) || r.Actor == "" || r.What == "":
			skip("it is missing what an ask holds")
		case !slices.Contains([]string{grantOpen, grantGranted, grantRefused, grantUsed, grantClosed}, r.State):
			skip("its state isn't one an ask has")
		case r.At <= 0 || r.Expires <= r.At || r.Expires-r.At > grantTTL.Milliseconds():
			skip("its times can't be an ask's")
		case seen[r.ID]:
			skip("its id is taken by an earlier line")
		default:
			seen[r.ID] = true
			if end, ok := s.ended[r.ID]; ok {
				r.State, r.Ended = end.state, end.ended
			}
			records = append(records, r)
		}
	}
	return records, nil
}

// save writes the asks whole: to a file beside it, mode 0600, then put in the file's place.
func (s *grantStore) save(records []grantRecord) error {
	var out bytes.Buffer
	for _, r := range records {
		r.Sig = s.sign(r)
		out.Write(recordLine(r))
		out.WriteByte('\n')
	}
	if len(records) > grantMaxCount || out.Len() > grantMaxBytes {
		return errGrantsFull
	}
	if err := writeKeptToken(s.file, out.Bytes()); err != nil {
		return err
	}
	s.saved = true
	return nil
}

// end marks records[i] done with (used or told) and keeps that in memory too, then saves.
func (s *grantStore) end(records []grantRecord, i int, state string, now int64) error {
	records[i].State, records[i].Ended = state, now
	s.ended[records[i].ID] = grantEnd{state: state, key: records[i].Key, ended: now, expires: records[i].Expires}
	for id, end := range s.ended {
		if end.expires <= now && now-end.ended >= grantCoolDown.Milliseconds() {
			delete(s.ended, id) // its line, signed with its expiry, is run out anyway, and its cool-down is over
		}
	}
	return s.save(records)
}

// tidy drops the asks done with for longer than grantKeep.
func tidy(records []grantRecord, now int64) []grantRecord {
	kept := records[:0:0]
	for _, r := range records {
		if !r.over(now) || now-r.endedAt() < grantKeep.Milliseconds() {
			kept = append(kept, r)
		}
	}
	return kept
}

// room makes room for one ask more, dropping the oldest finished ones first, and says whether there is any: the live
// asks are never dropped.
func room(records []grantRecord, now int64) ([]grantRecord, bool) {
	for len(records) >= grantMaxCount {
		oldest := -1
		for i, r := range records {
			if r.over(now) && (oldest < 0 || r.endedAt() < records[oldest].endedAt()) {
				oldest = i
			}
		}
		if oldest < 0 {
			return records, false
		}
		records = slices.Delete(records, oldest, oldest+1)
	}
	return records, true
}

func newGrantID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "g" + hex.EncodeToString(b)
}

// settle is what decide answers when a gate says the owner must be asked: a grant for this actor and this ask,
// found and used (once), allows; an ask refused is told as a no (once), and stays one for grantCoolDown; one still
// waiting stays a no; and none yet is written, in the approvals list now. An ask the owner couldn't be shown whole
// is a no and never written. v is the gate's askOwner verdict. Anything that goes wrong stays a no.
func (s *grantStore) settle(a actor, v verdict) verdict {
	if s == nil || !(a.heldAsGM() || a.kind() == "hire") {
		return v
	}
	label := a.label()
	why := unshowable("text", v.what, grantWhatMost)
	if why == "" {
		why = unshowable("amount", v.amount, grantAmountMost)
	}
	if why == "" {
		why = unshowable("asker's name", label, grantActorMost)
	}
	if why == "" && v.what == "" {
		why = "it doesn't say what it is"
	}
	if why != "" {
		return refuse(v.status, "This can't be put to the owner, so it is a no: "+why+".")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unsaved := refuse(v.status, "Smolcorp couldn't keep your ask for the owner, so this stays a no: "+v.what)
	records, err := s.load()
	if err != nil {
		return unsaved
	}
	now := s.now().UnixMilli()
	records = tidy(records, now)
	key := grantKey(a.identity(), v.what, v.amount)
	granted, refused, open, waiting, cooling := -1, -1, -1, 0, false
	for _, end := range s.ended {
		if end.key == key && end.state == grantClosed && now-end.ended < grantCoolDown.Milliseconds() {
			cooling = true
		}
	}
	for i, r := range records {
		if r.Actor == label && r.State == grantOpen && !r.over(now) {
			waiting++
		}
		if r.Key != key {
			continue
		}
		if r.State == grantClosed && now-r.Ended < grantCoolDown.Milliseconds() {
			cooling = true
		}
		if r.over(now) {
			continue
		}
		switch r.State {
		case grantGranted:
			granted = i
		case grantRefused:
			refused = i
		case grantOpen:
			open = i
		}
	}
	switch {
	case granted >= 0:
		if s.end(records, granted, grantUsed, now) != nil {
			return unsaved
		}
		return allow()
	case refused >= 0:
		if s.end(records, refused, grantClosed, now) != nil {
			return unsaved
		}
		return refuse(v.status, "The owner said no to this: "+v.what)
	case open >= 0:
		return v
	case cooling:
		return refuse(v.status, "The owner said no to this a moment ago; ask again later: "+v.what)
	case waiting >= grantMaxOpenEach:
		return refuse(v.status, errGrantsFull.Error()+" Yours wait already: "+v.what)
	}
	records, fits := room(records, now)
	if !fits {
		return refuse(v.status, errGrantsFull.Error()+" "+v.what)
	}
	records = append(records, grantRecord{ID: newGrantID(), Actor: label, What: v.what, Amount: v.amount, Key: key,
		At: now, Expires: now + grantTTL.Milliseconds(), State: grantOpen})
	if s.save(records) != nil {
		return unsaved
	}
	return v
}

// answer is the owner's yes or no to one ask, by id. by must be one of the owner's channels; the GM, an MCP key, a
// hire, a night job, a tool and anyone else are refused. A grant covers that one ask: its use is settle's, once.
func (s *grantStore) answer(by actor, id string, yes bool) (grantRecord, error) {
	if !isOwnerTap(by) {
		return grantRecord{}, errNotOwner
	}
	if s == nil {
		return grantRecord{}, errNoAsk
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.load()
	if err != nil {
		return grantRecord{}, err
	}
	now := s.now().UnixMilli()
	for i, r := range records {
		if r.ID != id {
			continue
		}
		switch {
		case r.State != grantOpen:
			return grantRecord{}, errAnswered
		case r.over(now):
			return grantRecord{}, errRunOut
		}
		records[i].State, records[i].Answered, records[i].By = grantRefused, now, string(by)
		if yes {
			records[i].State = grantGranted
		}
		if err := s.save(tidy(records, now)); err != nil {
			return grantRecord{}, err
		}
		return records[i], nil
	}
	return grantRecord{}, errNoAsk
}

// waiting is the asks the owner can answer now, the oldest first: what the approvals list shows.
func (s *grantStore) waiting() []grantRecord {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.load()
	if err != nil {
		return nil
	}
	now := s.now().UnixMilli()
	var open []grantRecord
	for _, r := range records {
		if r.State == grantOpen && !r.over(now) {
			open = append(open, r)
		}
	}
	return open
}

// asksAnswer is the asks as the approvals lists carry them, under "asks": the longest waiting first. Never a key.
// by, what and amount are the asker's text, whole and as it was keyed: a page shows them as text (textContent),
// never as markup.
func (s *grantStore) asksAnswer(now float64) []map[string]any {
	rows := []map[string]any{}
	for _, r := range s.waiting() {
		rows = append(rows, map[string]any{
			"id": r.ID, "by": r.Actor, "what": r.What, "amount": r.Amount,
			"since": r.At, "waitingSeconds": int64(max(0, now-float64(r.At)) / 1000), "expires": r.Expires,
		})
	}
	return rows
}

// The owner's routes. The asks show in the approvals lists (approvals.go: the phone's GET /approvals and setup's
// GET /setup/gm/approvals); the Mac app's office window, which those don't reach, reads them with GET /grants. The
// answer is POST /grants/answer?id=<ask>&answer=yes|no from a paired phone over Tailscale or the Mac app's office
// window, each with its key, and POST /setup/grants/answer from the setup page, with its cookie. Never the office in
// a browser (its key is in the browser's history) nor any window off a Mac, nor the paired device the Windows window
// holds (its key is in a file here): accessOwnerTap. No other caller reaches an answer: the route tables keep the
// GM's token, a hire's key and a tool's setup key out, an MCP key and a night job have no way to it, and
// grantStore.answer refuses all of them again.
func init() {
	refusal := func(d *Deck) string {
		if d.macHost() != nil {
			return "Your asks show in the office's window on your Mac, on the setup page, and on your paired phone over Tailscale's HTTPS address."
		}
		return "Your asks show on the setup page and on your paired phone over Tailscale's HTTPS address. The office on this computer can't answer them: other programs here can read its key."
	}
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/grants", access: accessOwnerTap, refusal: refusal,
		serve: func(d *Deck, w http.ResponseWriter, _ *request, _ string) {
			now := nowMillis()
			writeJSON(w, http.StatusOK, map[string]any{"at": int64(now), "asks": d.grants.asksAnswer(now)})
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/grants/answer", access: accessOwnerTap, refusal: refusal,
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			// The window here is the Mac app's own; the office in a browser, a window off a Mac and the Windows
			// window's paired device never get this far (accessOwnerTap), so a phone is a phone.
			by := actorPhone
			if r.phoneID == officeWindowID {
				by = actorWindow
			}
			d.answerGrant(w, by, r.URL.Query())
		}})
}

// answerGrant takes the owner's yes or no (the query's id and answer) as the channel by, and says how it went.
func (d *Deck) answerGrant(w http.ResponseWriter, by actor, query url.Values) {
	var yes bool
	switch query.Get("answer") {
	case "yes":
		yes = true
	case "no":
	default:
		writeStatus(w)(http.StatusBadRequest, "Answer yes or no.")
		return
	}
	if _, err := d.grants.answer(by, query.Get("id"), yes); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errNotOwner):
			status = http.StatusForbidden
		case errors.Is(err, errNoAsk):
			status = http.StatusNotFound
		case errors.Is(err, errAnswered), errors.Is(err, errRunOut):
			status = http.StatusConflict
		case errors.Is(err, errGrantsFull):
			status = http.StatusInsufficientStorage
		}
		writeStatus(w)(status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
