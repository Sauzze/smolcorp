package desktop

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// The checks on a GM memory note (1.26, track MEM, piece MEM2): one function, memoryRefusal, run when a note is
// written and again each time one is read, with the lists as they are then, so a note a later list would refuse
// is withheld. They are pure: no file, no clock, no network.
//
// The hard rules they hold (PLAN.md, the MEM section):
//   - nothing the owner can't see as it is: unshowable and its one list, hidesText (grants.go, hiddentext.go),
//     reused as they are, plus the characters Unicode's NFKC would read as other ones (memorySpecialForm), refused
//     rather than folded;
//   - no markup: a note is plain sentences, never a heading, a tag, a comment or a brief's slot;
//   - no key, token or password: Summary's secret shapes (secretShapes, hasAWSSecret in summary.go) and
//     check-public's Anthropic key, and a word that names a secret only when a value follows it, so "never put a
//     token in a brief" passes; and, whatever the words around it, anything shaped like a key, its groups read
//     joined, even across fields (memoryKeyShaped, memoryGroupedKey); and in a note that names a password, a key
//     or a token, any word anywhere that could be one, read across all its fields (memoryNamedSecretValue);
//   - no personal data: an e-mail address, a phone number, a card number (Luhn), an IBAN, a home folder. A
//     person's or a client's name can't be told by a program: the owner reads every note before it stands;
//   - English or French only (Q-M3's safe default), the languages the gate words are known in: a note in another
//     language is refused, and one with too few everyday English or French words for the checks to be sure is
//     held (memoryLanguage);
//   - a note about a gated action (push, merge, publish, deploy, spend, delete, the fence), or about asking and
//     permission, is never kept on its own: it is held for the owner's tighten-only tap (Q-M1's safe default),
//     with a reason the owner's card shows. The words are read folded and every way they could be spelled out
//     (memoryReadings), and a word with a pattern in it as everything it could fill in (memoryPatternGate). Past
//     the word lists, a note is held by its grammar, whatever its verb: where it acts (into main, anywhere on the
//     computer), leave it gives (you may, can you, without, tu peux, sans), a modal whatever its subject (agents
//     can, les agents peuvent), a standing order (on Fridays), a condition (if the tests pass, quand), or a word
//     written in disguise (memoryGate). A rule (a feedback note) that names a gate, a branch, asking or permission
//     is held whatever its grammar (VICTOR, FXMEM2R3): the grammar only picks the reason the owner's card shows.
//
// The principle (VICTOR, FXMEM2R1): fail closed. A word list can always be dodged, and the owner taps every note
// before it stands, so when a check can't be sure it holds the note, or refuses what could be a secret.
//
// A refusal never repeats what it found: the words are the note's field and the kind of thing, never its value.

// memoryEntry is a note as the checks read it. A feedback note's body is three fields: the rule, why, and how to
// apply it; any other type's is Body alone. Every field is one line of plain sentences.
type memoryEntry struct {
	Name, Description, Type string
	Body                    string
	Rule, Why, How          string
}

// The sizes a note's fields may have, in bytes. The body's is its fields' together.
const (
	memoryNameMost        = 80
	memoryDescriptionMost = 200
	memoryBodyMost        = 1000
)

// memoryTypes are the kinds of note: about the owner, a rule from their correction, a fact about the work, and
// where to find something.
var memoryTypes = []string{"user", "feedback", "project", "reference"}

type memoryField struct{ name, text string }

// memoryHold is why a note waits for the owner's tap rather than standing on its own: its kind (a gate, or one of
// memoryHoldReasons' other kinds; "" when it doesn't wait), and the reason the owner's card shows as it is, in
// English and in French.
type memoryHold struct{ Kind, English, French string }

// memoryRefusal says why entry can't be memory at all ("" if it can), and else why it is held for the owner's
// tighten-only tap (a zero memoryHold if it isn't): it names a gated action, speaks of asking or permission, or
// is in words the checks can't read. When a check can't be sure, the note is held, never kept silently.
//
// It is the one check, at use as at write (SIERRA's condition, FXMEM2R3): a reader (MEM4's read of the index) calls
// memoryRefusal on every entry each time the GM reads memory, and withholds one it refuses. The owner's tap keeps a
// held note, but never changes this answer, and a note never changes a gate or the fence: the guard holds those
// when an agent acts, whatever memory says.
//
// also is other text that comes with the note, MEM1's source quote (RT126MEM1R2 B3): it is read with the note's
// fields by the reads across them alone (a key's groups, a PIN after its name, and the strict rule), so a secret's
// name in the note and its value in the quote, or the other way round, is refused as it is inside the note. It
// isn't one of the note's fields: its size, its characters, its language and its gates are the caller's to check,
// or none. With no text in also, the answer is the note's alone, as it was before.
func memoryRefusal(entry memoryEntry, also ...string) (refusal string, hold memoryHold) {
	fields, refusal := memoryFields(entry)
	if refusal != "" {
		return refusal, memoryHold{}
	}
	for _, field := range fields {
		most := memoryBodyMost
		switch field.name {
		case "name":
			most = memoryNameMost
		case "description":
			most = memoryDescriptionMost
		}
		if refusal := memoryFieldRefusal(field.name, field.text, most); refusal != "" {
			return refusal, memoryHold{}
		}
	}
	// B3 (FXMEM2R4, FXMEM2R5): a secret's name in one field and its value in another (named "Staging password",
	// with "zebra42" as its body) is read as one text too, and in a note that names a password, a key or a token,
	// no word anywhere may be one (memoryNamedSecretValue).
	texts := make([]string, len(fields))
	for i, field := range fields {
		texts[i] = field.text
	}
	readings := memoryReadTogether(texts, also)
	if slices.ContainsFunc(readings, func(texts []string) bool {
		var words []string
		for _, text := range texts {
			words = append(words, strings.Fields(text)...)
		}
		return memoryJoinedKeyStart(words) || memoryGroupedKey(words)
	}) {
		return "its fields, read one after another, have what looks like a key or a token: memory never keeps one", memoryHold{}
	}
	if slices.ContainsFunc(readings, func(texts []string) bool { return memoryPINValue(strings.Join(texts, "\n")) }) {
		return "its fields, read one after another, have what looks like a secret's value after its name: memory never keeps one", memoryHold{}
	}
	if slices.ContainsFunc(readings, memoryNamedSecretValue) {
		return memorySecretRefusal[0], memoryHold{}
	}
	unread, refusal := memoryLanguage(fields)
	if refusal != "" {
		return refusal, memoryHold{}
	}
	kind := memoryGate(fields, entry.Type == "feedback")
	if kind == "" && unread {
		kind = "language"
	}
	if kind == "" {
		return "", memoryHold{}
	}
	return "", memoryHold{Kind: kind, English: memoryHoldReasons[kind][0], French: memoryHoldReasons[kind][1]}
}

// memoryReadTogether is the ways texts, a note's fields, are read across with also, the text that comes with it
// (memoryRefusal): the fields alone when also has no text, and else also put before each field and after the last,
// since a name read just before its value (a PIN, a key's start) may be at the end of the quote or of any field
// ("the door PIN" quoted, "4321 opens it" as the body).
func memoryReadTogether(texts, also []string) [][]string {
	var more []string
	for _, text := range also {
		if strings.TrimSpace(text) != "" {
			more = append(more, text)
		}
	}
	if len(more) == 0 {
		return [][]string{texts}
	}
	readings := make([][]string, 0, len(texts)+1)
	for at := range len(texts) + 1 {
		readings = append(readings, slices.Concat(texts[:at], more, texts[at:]))
	}
	return readings
}

// memoryHoldReasons are what the owner's card says about a held note, by its kind: English, then French.
var memoryHoldReasons = map[string][2]string{
	"push":      {"This note is about pushing code, so it needs your tap.", "Cette note parle d'envoyer du code sur le dépôt : à toi de la valider."},
	"merge":     {"This note is about merging code, so it needs your tap.", "Cette note parle de fusionner du code : à toi de la valider."},
	"publish":   {"This note is about publishing or releasing, so it needs your tap.", "Cette note parle de publier une version : à toi de la valider."},
	"deploy":    {"This note is about deploying, so it needs your tap.", "Cette note parle de la mise en production : à toi de la valider."},
	"spend":     {"This note is about spending money, so it needs your tap.", "Cette note parle de dépenser de l'argent : à toi de la valider."},
	"delete":    {"This note is about deleting things, so it needs your tap.", "Cette note parle de supprimer des fichiers ou des données : à toi de la valider."},
	"fence":     {"This note is about the fence that keeps agents in their project, so it needs your tap.", "Cette note parle de la clôture qui garde les agents dans leur projet : à toi de la valider."},
	"ask":       {"This note is about permissions, so it needs your tap.", "Cette note parle d'autorisations : à toi de la valider."},
	"tighten":   {"This rule only asks agents to check with you first. Tapping it can only make things stricter: it never allows anything.", "Cette règle demande seulement aux agents de te consulter d'abord. La valider ne peut que rendre les choses plus strictes : elle n'autorise jamais rien."},
	"branch":    {"This rule is about branches, so it needs your tap.", "Cette règle parle des branches du dépôt : à toi de la valider."},
	"modal":     {"This note says what can or may be done, so it needs your tap.", "Cette note dit ce qui peut se faire : à toi de la valider."},
	"condition": {"This note says what to do if or when something happens, so it needs your tap.", "Cette note dit quoi faire dans tel ou tel cas : à toi de la valider."},
	"grant":     {"This note lets agents act without asking you first, so it needs your tap.", "Cette note laisse les agents agir sans te demander : à toi de la valider."},
	"standing":  {"This note tells agents to act on their own at a set time or when something happens, so it needs your tap.", "Cette note demande aux agents d'agir seuls à heure fixe ou dès que quelque chose se produit : à toi de la valider."},
	"spelling":  {"This note disguises a word, with *, ?, brackets or digits in place of letters, so it needs your tap.", "Cette note écrit un mot de façon déguisée, avec des *, des ?, des crochets ou des chiffres à la place des lettres : à toi de la valider."},
	"language":  {"Smolcorp couldn't read this note as English or French, the only languages it checks, so it needs your tap.", "Smolcorp n'arrive pas à lire cette note comme du français ou de l'anglais, les deux seules langues qu'il sait vérifier : à toi de la valider."},
}

// memoryFields are entry's fields, each named as a refusal says it, or why its shape is wrong.
func memoryFields(entry memoryEntry) ([]memoryField, string) {
	if !slices.Contains(memoryTypes, entry.Type) {
		return nil, "its type isn't one of " + strings.Join(memoryTypes, ", ")
	}
	fields := []memoryField{{"name", entry.Name}, {"description", entry.Description}}
	body := []memoryField{{"body", entry.Body}}
	other := []memoryField{{"rule", entry.Rule}, {"why", entry.Why}, {"how to apply it", entry.How}}
	if entry.Type == "feedback" {
		body, other = other, body
	}
	for _, field := range other {
		if field.text != "" {
			return nil, fmt.Sprintf("a %s note has no %s", entry.Type, field.name)
		}
	}
	fields = append(fields, body...)
	size := 0
	for i, field := range fields {
		if strings.TrimSpace(field.text) == "" {
			return nil, fmt.Sprintf("its %s is empty", field.name)
		}
		if i >= 2 {
			size += len(field.text)
		}
	}
	if size > memoryBodyMost {
		return nil, fmt.Sprintf("its body is longer than the %d bytes a note may have", memoryBodyMost)
	}
	return fields, ""
}

// memoryFieldRefusal says why one field's text can't be memory, or "".
func memoryFieldRefusal(name, text string, most int) string {
	if refusal := unshowable(name, text, most); refusal != "" {
		return refusal
	}
	for _, c := range text {
		switch {
		case memorySpecialForm(c):
			return fmt.Sprintf("its %s has a character in a special form that reads as another (U+%04X): write it plainly", name, c)
		case !memoryEnglishOrFrench(c):
			return fmt.Sprintf("its %s has a letter or digit that isn't English or French (U+%04X): a note is in English or French only, the languages Smolcorp can check it in", name, c)
		}
	}
	if memoryMarkup(text) {
		return fmt.Sprintf("its %s has markup (a heading, a tag, a comment or a template marker): a note is plain sentences", name)
	}
	if kind := memorySecret(text); kind != "" {
		return fmt.Sprintf("its %s has what looks like %s: memory never keeps one", name, kind)
	}
	if kind := memoryPersonal(text); kind != "" {
		return fmt.Sprintf("its %s has %s: memory never keeps personal data", name, kind)
	}
	if memoryHomeFolder.MatchString(text) && !memoryHomeNotAName(text) {
		return fmt.Sprintf("its %s names someone's home folder: write ~ for it, so no one's name is kept", name)
	}
	return ""
}

// memorySpecialForm is a character NFKC reads as another, plainer one, which unshowable's styled letters don't
// cover: ligatures, superscripts and subscripts, small capitals and modifier letters, letter-like symbols (the
// Kelvin sign), Roman numerals, circled, bracketed and squared letters, fractions, the ordinal a and o, small and
// wide punctuation. Refused rather than folded: a note says what it shows.
func memorySpecialForm(c rune) bool {
	switch {
	case c < 0x80:
		return false
	case c == 0xAA || c == 0xBA || c == 0xB2 || c == 0xB3 || c == 0xB9 || c >= 0xBC && c <= 0xBE:
		return true
	case c == 0x132 || c == 0x133 || c == 0x13F || c == 0x140 || c == 0x149 || c == 0x17F:
		return true
	case c >= 0x1C4 && c <= 0x1CC || c >= 0x1F1 && c <= 0x1F3:
		return true
	}
	for _, span := range [][2]rune{
		{0x02B0, 0x02FF}, {0x1D00, 0x1DBF}, {0x2070, 0x209F}, {0x2100, 0x218F}, {0x2460, 0x24FF},
		{0x2C7C, 0x2C7D}, {0x3200, 0x33FF}, {0xA770, 0xA770}, {0xA7F2, 0xA7F9}, {0xAB5C, 0xAB5F},
		{0xFB00, 0xFB06}, {0xFE10, 0xFE19}, {0xFE30, 0xFE6F}, {0xFF00, 0xFFEF}, {0x1D400, 0x1D7FF},
		{0x1F100, 0x1F1FF},
	} {
		if c >= span[0] && c <= span[1] {
			return true
		}
	}
	return false
}

// frenchLetters are the letters French writes beyond ASCII's.
const frenchLetters = "àâçéèêëîïôùûüÿœæÀÂÇÉÈÊËÎÏÔÙÛÜŸŒÆ"

// frenchMarks are the accents French puts on a letter, as a mark typed apart (NFD, as a Mac's file names have it).
var frenchMarks = []rune{0x300, 0x301, 0x302, 0x308, 0x327}

// memoryEnglishOrFrench is whether c, if a letter, digit or mark, is one English or French writes.
func memoryEnglishOrFrench(c rune) bool {
	switch {
	case c < 0x80, !unicode.In(c, unicode.L, unicode.M, unicode.N):
		return true
	case unicode.Is(unicode.M, c):
		return slices.Contains(frenchMarks, c)
	}
	return strings.ContainsRune(frenchLetters, c)
}

// memoryMarkupStart is how a field may not start: a heading, a rule or front matter, a quote, a tag or a list's
// mark that a Markdown reader would take as structure.
var memoryMarkupStart = []string{"#", "---", "<", ">", "* ", "- ", "+ ", "==="}

// memoryMarkupAnywhere are the markers no note holds anywhere: comments (the briefs' <!-- slot: … -->), a template
// ({{status_file}}, {{> slot}}, {% … %}), a code fence, and a tag (<system>, </brief>, <|…|>).
var (
	memoryMarkupAnywhere = []string{"<!--", "-->", "{{", "}}", "{%", "%}", "```"}
	memoryTag            = regexp.MustCompile(`<[/!?|A-Za-z]`)
)

func memoryMarkup(text string) bool {
	trimmed := strings.TrimSpace(text)
	for _, start := range memoryMarkupStart {
		if strings.HasPrefix(trimmed, start) {
			return true
		}
	}
	for _, marker := range memoryMarkupAnywhere {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return memoryTag.MatchString(text)
}

var (
	// anthropicKey is check-public's shape for an Anthropic key (scripts/check-public.sh).
	anthropicKey = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{8,}`)
	// secretValue is a word that names a secret, then what gives its value (= or :, an arrow, or is, are, est,
	// was, sont, vaut), then what may be its value.
	secretValue = regexp.MustCompile(`(?i)([a-z0-9_-]*(?:key|token|secret|passw(?:or)?d|passphrase|pwd|credentials?|jeton|mdp|cl[eé])|mot de passe)['"]?(?:\s*(?:->|=>|→)|\s*[=:]|\s+(?:is|are|was|est|sont|vaut)(?:\s*[=:])?)\s*['"]?([^\s'",;]+)`)
	// bearerValue is an Authorization header's value.
	bearerValue = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/-]{8,}`)
	// memoryLongWord is a word long enough to be a key whatever is written around it, when it mixes letters and
	// digits; a git commit's id, 40 hexadecimal digits, is the one such word allowed (an IBAN is refused as one).
	memoryLongWord = regexp.MustCompile(`[A-Za-z0-9_-]{20,}`)
	memoryCommitID = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	// memorySpacedKey is the start of a key's own shape, read in letters spaced out (s k - a n t): a key, a
	// token's prefix, an AWS key id or a JWT.
	memorySpacedKey = regexp.MustCompile(`(?i)^(?:sk-|(?:sk|rk|pk)_|re_|gh[po]_|github_?pat|xox[abpr]-|akia|eyj)`)
)

// memorySecret names the kind of secret text holds, or "".
func memorySecret(text string) string {
	if anthropicKey.MatchString(text) || hasAWSSecret(text) || slices.ContainsFunc(secretShapes, func(shape *regexp.Regexp) bool { return shape.MatchString(text) }) {
		return "a key or a token"
	}
	if bearerValue.MatchString(text) {
		return "a token"
	}
	for _, match := range secretValue.FindAllStringSubmatch(text, -1) {
		if secretLooksLikeAValue(strings.ToLower(match[1]), match[2]) {
			return "a secret's value after its name"
		}
	}
	for _, word := range memoryLongWord.FindAllString(text, -1) {
		letters := strings.ContainsFunc(word, unicode.IsLetter)
		digits := strings.ContainsFunc(word, unicode.IsDigit)
		if letters && digits && !memoryCommitID.MatchString(word) && !ibanIn(word) {
			return "a key or a token"
		}
	}
	words := strings.Fields(text)
	if memorySpacedOutKey(text) || memoryJoinedKeyStart(words) || slices.ContainsFunc(words, memoryKeyShaped) || memoryGroupedKey(words) {
		return "a key or a token"
	}
	if memoryPINValue(text) {
		return "a secret's value after its name"
	}
	return ""
}

// The lists above are a first pass. What follows reads a secret by its shape, whatever word stands before it, and
// fails closed (N1, FXMEM2R2): a word, or words written apart in groups, that looks like a key is refused.

// memoryJoiners are the signs a key's groups are written apart with (ab12-cd34, ab12.cd34, ab12/cd34): read as
// nothing when a word's length is measured, and not a sign of their own when its kinds of characters are counted.
const memoryJoiners = `-_./:+\`

var (
	// memoryKeyStart is the start of a key's own shape, read with the spaces and dots between its pieces taken out
	// (s.k.-.a.n.t, sk- ant-, s k-an t-ap): a key, a token's prefix, an AWS key id or a JWT.
	memoryKeyStart = regexp.MustCompile(`^(?:(?i:sk-|(?:sk|rk|pk)_|re_|gh[opsu]_|github_pat_|xox[abpr]-)|AKIA[0-9A-Z]|eyJ[A-Za-z0-9])`)
	// memoryPlainSegment is a piece of a path or a web address that is a word: lower case, capitalized or upper
	// case letters, digits after small or capitalized letters (v1, node20), or digits alone.
	memoryPlainSegment = regexp.MustCompile(`^(?:[0-9]+|\p{Ll}+[0-9]*|\p{Lu}\p{Ll}*[0-9]*|\p{Lu}+)$`)
	// memoryIBANStart is an IBAN's country and check digits.
	memoryIBANStart = regexp.MustCompile(`^[A-Za-z]{2}[0-9]{2}`)
	// memoryURLLogin is a web address with a name and a password before its host.
	memoryURLLogin = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
)

// memoryJoinedKeyStart is whether words, read joined from any one of them with the spaces and dots between taken
// out, start as a key does, in pieces of any length.
func memoryJoinedKeyStart(words []string) bool {
	for i := range words {
		joined := strings.ReplaceAll(strings.Join(words[i:min(i+16, len(words))], ""), ".", "")
		if memoryKeyStart.MatchString(joined) {
			return true
		}
	}
	return false
}

// memoryKeyShaped is whether word, the signs around it aside, has a key's shape: 12 characters or more that mix
// three of four kinds (lower case, upper case, digits, other signs: Zebra-Fake-42), or, its joiners aside, letters
// and digits in 20 or more, or 12 or more that change between letters and digits often and vary a lot (high
// entropy for its length: a1b2c3d4e5f6a7b8). A git commit's 40-digit id and an IBAN (refused as one later) aren't keys; nor is a
// path or web address of plain words, unless its pieces change between letters and digits as a key's do.
func memoryKeyShaped(word string) bool {
	word = strings.TrimFunc(word, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })
	compact := strings.Map(func(c rune) rune {
		if strings.ContainsRune(memoryJoiners, c) {
			return -1
		}
		return c
	}, word)
	size := utf8.RuneCountInString(compact)
	if utf8.RuneCountInString(word) < 12 || memoryCommitID.MatchString(compact) || memoryIBANShaped(compact) {
		return false
	}
	letters, digits := strings.ContainsFunc(compact, unicode.IsLetter), strings.ContainsFunc(compact, unicode.IsDigit)
	entropy := memoryEntropy(compact)
	// A key's digits may come bunched (deadbeefcafe1234): a quarter of it digits or more is enough (FXMEM2R3).
	keyLike := letters && digits && size >= 12 && entropy >= 3 &&
		(memoryInterleaved(compact) >= 3 || 4*memoryDigits(compact) >= size)
	if mixed, plain := memoryPlainPath(word); plain {
		return mixed >= 2 && keyLike
	}
	// A key of letters alone, in one case, 20 or more, that varies more than a word does (FXMEM2R3).
	lettersKey := !digits && size >= 20 && (strings.ToUpper(compact) == compact || strings.ToLower(compact) == compact) && entropy >= 3.9
	return memoryClasses(word) >= 3 || letters && digits && size >= 20 || keyLike || lettersKey
}

// memoryDigits is how many digits word has.
func memoryDigits(word string) int {
	return strings.Count(strings.Map(func(c rune) rune {
		if unicode.IsDigit(c) {
			return '0'
		}
		return -1
	}, word), "0")
}

// memoryIBANShaped is whether compact starts as an IBAN does (its country and check digits) and passes its check.
func memoryIBANShaped(compact string) bool {
	return len(compact) >= 15 && memoryIBANStart.MatchString(compact) && ibanIn(compact)
}

// memoryPlainPath is whether word is a path or a web address (it has a slash) of plain words, with no password in
// it and at most two pieces mixing letters and digits (v1, node20), and how many such pieces it has: two that
// change between letters and digits as a key's groups do (abcd1234/efgh5678) are read as a key.
func memoryPlainPath(word string) (mixed int, plain bool) {
	if !strings.ContainsAny(word, `/\`) || memoryURLLogin.MatchString(word) {
		return 0, false
	}
	for _, segment := range strings.FieldsFunc(word, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) }) {
		if !memoryPlainSegment.MatchString(segment) {
			return 0, false
		}
		if strings.ContainsFunc(segment, unicode.IsLetter) && strings.ContainsFunc(segment, unicode.IsDigit) {
			mixed++
		}
	}
	return mixed, mixed <= 2
}

// memoryClasses is how many of the four kinds of character word has: lower case, upper case, digits, and signs
// other than the joiners.
func memoryClasses(word string) int {
	var lower, upper, digit, sign int
	for _, c := range word {
		switch {
		case unicode.IsLower(c):
			lower = 1
		case unicode.IsUpper(c):
			upper = 1
		case unicode.IsDigit(c):
			digit = 1
		case !strings.ContainsRune(memoryJoiners+"'’", c): // an apostrophe is French elision (L'application), not a sign
			sign = 1
		}
	}
	return lower + upper + digit + sign
}

// memoryInterleaved is how many times word changes between a letter and a digit (a1b2 changes three times; node20
// once).
func memoryInterleaved(word string) int {
	changes, last := 0, 0
	for _, c := range word {
		kind := 0
		switch {
		case unicode.IsLetter(c):
			kind = 1
		case unicode.IsDigit(c):
			kind = 2
		}
		if kind != 0 && last != 0 && kind != last {
			changes++
		}
		if kind != 0 {
			last = kind
		}
	}
	return changes
}

// memoryEntropy is word's Shannon entropy, in bits a character: how much its characters vary.
func memoryEntropy(word string) float64 {
	counts, n := map[rune]int{}, 0
	for _, c := range word {
		counts[c]++
		n++
	}
	h := 0.0
	for _, count := range counts {
		p := float64(count) / float64(n)
		h -= p * math.Log2(p)
	}
	return h
}

// memoryGroupedKey is whether words hold a key written in groups (ab12 cd34 ef56, AbCd EfGh 1234): any run of two
// or more words next to each other that aren't everyday words, at least half of them mixing letters and digits or
// lower and upper case, or eight or more of one or two characters (9 f 3 k 2 q 8 x), is read joined and checked as
// one word (memoryKeyShaped). A sentence's end closes a run; a comma does only after a group that isn't keyish, and
// "and" or "et" between keyish groups doesn't (AB12, CD34 and EF56, FXMEM2R3).
func memoryGroupedKey(words []string) bool {
	var run []string
	var keyish []bool
	bridged := false // the run went past a comma or an "and": then only keyish groups carry it on
	for i, word := range words {
		core := strings.TrimFunc(word, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })
		lower := strings.ToLower(word)
		if (lower == "and" || lower == "et" || lower == "&") && len(keyish) > 0 && keyish[len(keyish)-1] {
			bridged = true
			continue
		}
		if !memoryGroup(core) {
			run, keyish, bridged = nil, nil, false
			continue
		}
		if bridged && !memoryKeyish(core) {
			run, keyish, bridged = nil, nil, false
		}
		run, keyish = append(run, core), append(keyish, memoryKeyish(core))
		for start := range run {
			count, short := 0, true
			for k, isKeyish := range keyish[start:] {
				if isKeyish {
					count++
				}
				short = short && utf8.RuneCountInString(run[start+k]) <= 2
			}
			n := len(run) - start
			if n >= 2 && (2*count >= n || short && n >= 8) && memoryKeyShaped(strings.Join(run[start:], "")) {
				return true
			}
		}
		end := word[len(word)-1:]
		switch {
		case strings.ContainsAny(end, ";:!?.") || end == "," && !memoryKeyish(core) || i == len(words)-1:
			run, keyish, bridged = nil, nil, false
		case end == ",":
			bridged = true
		}
	}
	return false
}

// memoryGroup is whether core could be a group of a key: letters and digits, joiners inside, and not an
// everyday word.
func memoryGroup(core string) bool {
	if core == "" || memoryEverydayWord(strings.ToLower(core)) {
		return false
	}
	return !strings.ContainsFunc(core, func(c rune) bool {
		return c > unicode.MaxASCII || !unicode.IsLetter(c) && !unicode.IsDigit(c) && !strings.ContainsRune(memoryJoiners, c)
	})
}

// memoryKeyish is whether a group mixes letters and digits, or has a capital after a small letter (AbCd).
func memoryKeyish(core string) bool {
	if strings.ContainsFunc(core, unicode.IsLetter) && strings.ContainsFunc(core, unicode.IsDigit) {
		return true
	}
	runes := []rune(core)
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			return true
		}
	}
	return false
}

var (
	// memoryPasswordWords name a password, read joined, accents and apostrophes off (mot de passe, code d'accès).
	memoryPasswordWords = []string{"password", "passwords", "passwd", "pw", "passwort", "passcode", "passcodes", "passphrase", "passphrases", "pwd", "mdp", "motdepasse", "motsdepasse", "accesscode", "accesscodes", "codedacces", "codesdacces"}
	// memoryKeyWords name a key, a token, a secret (VICTOR, FXMEM2R3) or a login (identifiant, VICTOR, FXMEM2R5), as
	// do a compound that ends with one (api_key, auth-token, apiKey).
	memoryKeyWords    = []string{"key", "keys", "apikey", "apikeys", "token", "tokens", "secret", "secrets", "cle", "cles", "clef", "clefs", "jeton", "jetons", "credential", "credentials", "creds", "identifiant", "identifiants", "cleapi", "clesapi"}
	memoryKeyCompound = regexp.MustCompile(`(?:[A-Za-z0-9][_-](?i:key|token|secret|password|pwd)s?|[a-z0-9](?:Key|Token|Secret|Password)s?)$`)
	// memoryPINWords name one too, but are everyday words as well (tests pass, pin a version): a word after one, up
	// to two, is its value only when it has a digit, a capital inside, or a sign.
	memoryPINWords = []string{"pass", "pin", "pins"}
	// memoryGivers are the words that give a value: the password is, was set to, le mot de passe est, vaut.
	memoryGivers = []string{"is", "was", "are", "were", "be", "to", "as", "est", "etait", "sont", "vaut", "valait", "reste"}
	// memoryElisions are what French elides before an apostrophe (l'équipe, qu'on, jusqu'à), and memoryEnglishEnds
	// what English writes after one (the team's, the users', we'll, they're, I've, she'd, I'm).
	memoryElisions    = []string{"l", "d", "qu", "c", "j", "n", "s", "m", "t", "jusqu", "lorsqu", "puisqu"}
	memoryEnglishEnds = []string{"", "s", "ll", "re", "ve", "d", "m"}
)

// memorySecretRefusal is why a note that names a secret and holds a word that could be one is refused
// (memoryNamedSecretValue): in English, as memoryRefusal gives it, then in French, for the owner's card. It says
// plainly that a password manager's name is refused too (VICTOR, FXMEM2R5).
var memorySecretRefusal = [2]string{
	"This note names a password or a key and holds something that looks like one, so it isn't kept. Don't put a password's value or where it's kept in memory, not even the name of the app that keeps it.",
	"Cette note parle d'un mot de passe ou d'une clé et contient quelque chose qui ressemble à un secret : elle n'est donc pas gardée. Ne mets en mémoire ni un mot de passe ni l'endroit où il est rangé, pas même le nom de l'appli qui le garde.",
}

// memorySecretWords are text's words as the secret checks read them, with = and : apart (password:zebra), and their
// cores, the signs around them off.
func memorySecretWords(text string) (words, cores []string) {
	words = strings.FieldsFunc(strings.NewReplacer("’", "'", "=", " = ", ":", " = ", "→", " = ").Replace(text), func(c rune) bool {
		return unicode.IsSpace(c) || strings.ContainsRune(`,;()"`, c)
	})
	cores = make([]string, len(words))
	for i, word := range words {
		cores[i] = strings.TrimFunc(word, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })
	}
	return words, cores
}

// memorySecretSeparators are what a secret's name may be written with between its words: mot-de-passe,
// mot_de_passe, pass.phrase, API_KEY (RT126MEM2R6 B1).
var memorySecretSeparators = strings.NewReplacer("-", "", "_", "", ".", "", " ", "")

// memorySecretLabel is word as the lists of secrets' names read it: lower case, accents off, separators out.
func memorySecretLabel(word string) string {
	return memorySecretSeparators.Replace(memoryUnaccented(strings.ToLower(word)))
}

// memorySecretName is how many words, from cores[i] on, name a secret (0 if none), and what kind: a password, its
// name read joined up to three words (mot de passe, code d'accès, mot-de-passe); a key, a token, a secret or a
// login; or a PIN.
func memorySecretName(cores []string, i int) (n int, password, key, pin bool) {
	for n := min(3, len(cores)-i); n >= 1; n-- {
		label := strings.ReplaceAll(memorySecretLabel(strings.Join(cores[i:i+n], "")), "'", "")
		if slices.Contains(memoryPasswordWords, label) {
			return n, true, false, false
		}
	}
	if i >= len(cores) {
		return 0, false, false, false
	}
	label := memorySecretLabel(cores[i])
	if before, after, cut := strings.Cut(label, "'"); cut && slices.Contains(memoryElisions, before) {
		label = after // l'identifiant
	}
	switch {
	case slices.Contains(memoryKeyWords, label) || memoryKeyCompound.MatchString(cores[i]):
		return 1, false, true, false
	case slices.Contains(memoryPINWords, label):
		return 1, false, false, true
	}
	return 0, false, false, false
}

// memorySecretPieces are core's pieces as a secret's name is looked for in them (FXMEM2R7): core is cut at any
// character that isn't a letter, a digit or an accent typed apart (login/password, mot–de–passe, Mot+de+passe,
// l'identifiant), where a small letter meets a capital (adminPw, MotDePasse), before the last of two capitals when a
// small letter follows (APIKey) and where letters meet digits (password1, 1Password).
func memorySecretPieces(core string) []string {
	runes := []rune(core)
	var pieces []string
	start, last := -1, rune(0) // last is the letter or digit before, accents typed apart aside
	for i, c := range runes {
		switch {
		case unicode.IsMark(c) && start >= 0:
			continue
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			if start >= 0 {
				pieces = append(pieces, string(runes[start:i]))
			}
			start = -1
			continue
		case start >= 0 && (unicode.IsLower(last) && unicode.IsUpper(c) ||
			unicode.IsUpper(last) && unicode.IsUpper(c) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) ||
			unicode.IsDigit(last) != unicode.IsDigit(c)):
			pieces = append(pieces, string(runes[start:i]))
			start = i
		case start < 0:
			start = i
		}
		last = c
	}
	if start >= 0 {
		pieces = append(pieces, string(runes[start:]))
	}
	return pieces
}

// memoryNamedWord is what a word of a note holds of a secret's name (memoryNamedWords): a piece of a password's name
// or of a key's, whether its last piece is one, and its other pieces.
type memoryNamedWord struct {
	password, key, last bool
	others              []string
}

// memoryNamedWords reads cores' pieces one after another (memorySecretPieces) and finds a secret's name anywhere
// among them (VICTOR, FXMEM2R7): up to three pieces read joined (mot de passe, login/mot-de-passe, adminPassPhrase),
// or any run of letters spaced out one by one (p a s s w o r d, M.D.P.). It gives, for each core, what it holds.
func memoryNamedWords(cores []string) []memoryNamedWord {
	type piece struct {
		text, label   string
		word          int
		letter        bool // a letter alone
		password, key bool
	}
	var pieces []piece
	for i, core := range cores {
		for _, text := range memorySecretPieces(core) {
			label, first := memorySecretLabel(text), []rune(text)[0]
			pieces = append(pieces, piece{text: text, label: label, word: i, letter: utf8.RuneCountInString(label) == 1 && unicode.IsLetter(first)})
		}
	}
	for i := range pieces {
		label, letters := "", true
		for n := 1; i+n <= len(pieces); n++ {
			letters = letters && pieces[i+n-1].letter
			label += pieces[i+n-1].label
			if n > 3 && !letters || len(label) > 12 {
				break
			}
			password := slices.Contains(memoryPasswordWords, label)
			if !password && !slices.Contains(memoryKeyWords, label) {
				continue
			}
			for k := i; k < i+n; k++ {
				pieces[k].password = pieces[k].password || password
				pieces[k].key = pieces[k].key || !password
			}
		}
	}
	named := make([]memoryNamedWord, len(cores))
	for _, p := range pieces {
		word := &named[p.word]
		word.password, word.key = word.password || p.password, word.key || p.key
		word.last = p.password || p.key
		if !p.password && !p.key {
			word.others = append(word.others, p.text)
		}
	}
	return named
}

// memoryPINValue is whether text names a PIN (pass, pin) with, as one of the next two words, one in no everyday list
// that looks like a value: "the door PIN 4321". A placeholder ($PIN, <…>, …) isn't one.
func memoryPINValue(text string) bool {
	words, cores := memorySecretWords(text)
	for i := range cores {
		if n, _, _, pin := memorySecretName(cores, i); n == 0 || !pin {
			continue
		}
		most := 2
		for j := i + 1; j < len(words) && most > 0; j++ {
			core := cores[j]
			if core == "" {
				continue
			}
			most--
			if strings.IndexAny(words[j], "$<%{…") == 0 || strings.Trim(strings.ToLower(core), "x") == "" {
				continue
			}
			if !memoryEverydayWord(strings.ToLower(core)) && memoryValueLooking(core) {
				return true
			}
		}
	}
	return false
}

// memoryNamedSecretValue is VICTOR's strict rule (FXMEM2R5), with no window: when a note's fields, read joined, name
// a password, a passcode, a passphrase, a key, a token, a secret, a credential or a login (mot de passe, mdp, clé,
// jeton, identifiant), the note is refused if any of its words anywhere, its name aside, could be the value:
//   - a word in no list (memoryListed), a password manager's name among them (1Password, Bitwarden), or a number of
//     four digits or more;
//   - a word that mixes letters and digits (staging-42, node20);
//   - a field that is one word alone and not an everyday one: a word of the work counts (hello, admin), as anyone
//     can pick one for a password;
//   - after a password's name, or after what gives a value (is, est, to, =), a word of the work too: "the
//     password we use is hello".
//
// The words of the lists stand anywhere else ("Passwords are kept in the keychain"), as does a small number ("the
// token expires in 30 days") and a placeholder of x's.
func memoryNamedSecretValue(texts []string) bool {
	words, cores := memorySecretWords(strings.Join(texts, "\n"))
	// names is, for each word, what it holds of a secret's name (memoryNamedWords). A word that holds one is read as
	// part of the name, and its other pieces as any other word (login/password, adminPw, api_key2, 1Password).
	names := memoryNamedWords(cores)
	named, password := false, false
	for _, name := range names {
		named, password = named || name.password || name.key, password || name.password
	}
	if !named {
		return false
	}
	given := false // the word right after a password's name, or after what gives a value, in a note with one
	for j, core := range cores {
		switch name := names[j]; {
		case core == "":
			given = given || words[j] == "="
			continue
		case name.password || name.key:
			if memoryNamedWordCouldBeASecret(core, name) {
				return true
			}
			given = name.password && name.last
			continue
		}
		afterGiver := password && given
		given = slices.Contains(memoryGivers, memoryUnaccented(strings.ToLower(core)))
		if memoryCouldBeASecret(core, memoryListed) || afterGiver && memoryCouldBeASecret(core, memoryPlainEveryday) {
			return true
		}
	}
	for _, text := range texts {
		fieldWords, fieldCores := memorySecretWords(text)
		if len(fieldWords) != 1 {
			continue
		}
		if name := memoryNamedWords(fieldCores)[0]; name.password || name.key {
			continue // the name alone (Passwords)
		}
		if memoryCouldBeASecret(fieldCores[0], memoryPlainEveryday) {
			return true
		}
	}
	return false
}

// memoryNamedWordCouldBeASecret is whether core, a word that holds a secret's name, could hold its value too: it
// mixes letters and digits (password1, api_key2, 1Password), or one of its other pieces could be one, a word of the
// work or a secret's name aside. What an apostrophe parts (l'identifiant, the token's) isn't one.
func memoryNamedWordCouldBeASecret(core string, name memoryNamedWord) bool {
	if strings.ContainsFunc(core, unicode.IsLetter) && strings.ContainsFunc(core, unicode.IsDigit) {
		return true
	}
	for _, other := range name.others {
		lower := memoryUnaccented(strings.ToLower(other))
		if slices.Contains(memoryElisions, lower) || slices.Contains(memoryEnglishEnds, lower) {
			continue
		}
		if memoryCouldBeASecret(other, memoryListedOrSecretName) {
			return true
		}
	}
	return false
}

// memoryCouldBeASecret is whether core, a word of a note that names a secret, could be its value: it mixes letters
// and digits, or it, or one of the pieces its joiners part (staging_admin, sign-in), is a number of four digits or
// more or no word listed may read. A placeholder of x's isn't one.
func memoryCouldBeASecret(core string, listed func(string) bool) bool {
	lower := strings.ToLower(core)
	if strings.Trim(lower, "x") == "" {
		return false
	}
	if strings.ContainsFunc(core, unicode.IsLetter) && strings.ContainsFunc(core, unicode.IsDigit) {
		return true
	}
	if memoryListedWord(lower, listed) {
		return false
	}
	for _, piece := range strings.FieldsFunc(lower, func(c rune) bool { return strings.ContainsRune(memoryJoiners, c) }) {
		if strings.IndexFunc(piece, func(c rune) bool { return !unicode.IsDigit(c) }) < 0 {
			if utf8.RuneCountInString(piece) >= 4 {
				return true
			}
			continue
		}
		if !memoryListedWord(piece, listed) {
			return true
		}
	}
	return false
}

// memoryListedWord is whether word (lower case) is one listed reads, whole, or with an apostrophe: French's elided
// article or pronoun before a listed word (l'équipe), or a listed word before English's ending (the team's).
func memoryListedWord(word string, listed func(string) bool) bool {
	if listed(word) {
		return true
	}
	before, after, cut := strings.Cut(word, "'")
	switch {
	case !cut:
		return false
	case slices.Contains(memoryEnglishEnds, after):
		return listed(before)
	case slices.Contains(memoryElisions, before):
		return memoryListedWord(after, listed)
	}
	return false
}

// memoryListed is whether word (lower case) is on any of the lists: the everyday English and French words, and the
// words of the work, with or without accents.
func memoryListed(word string) bool {
	return englishWords[word] || frenchWords[word] || memoryEveryday[word] || memoryWorkWords[word] || memoryEverydayFolded[memoryUnaccented(word)]
}

// memoryListedOrSecretName is memoryListed, or a secret's name (apikey, read from apiKey).
func memoryListedOrSecretName(word string) bool {
	return memoryListed(word) || slices.Contains(memoryKeyWords, word) || slices.Contains(memoryPasswordWords, word)
}

// memoryPlainEveryday is whether word (lower case) is an everyday English or French word, not only a word of the
// work.
func memoryPlainEveryday(word string) bool {
	return englishWords[word] || frenchWords[word] || memoryEveryday[word]
}

// memoryValueLooking is whether a word looks like a value rather than a word: it has a digit, a capital after its
// first letter, or a sign.
func memoryValueLooking(word string) bool {
	_, size := utf8.DecodeRuneInString(word)
	return strings.ContainsFunc(word, unicode.IsDigit) || strings.ContainsAny(word, "!@#$%^&*+=~|") ||
		strings.ContainsFunc(word[size:], unicode.IsUpper)
}

// memorySpacedOutKey is whether text spells a key's start out in short pieces, spaces between (s k - a n t -
// …, sk - ant): each run of two or more words of three characters at most is read joined, with the word after it.
func memorySpacedOutKey(text string) bool {
	words := strings.Fields(text)
	for i := range words {
		joined := ""
		for j := i; j < len(words) && len(words[j]) <= 3; j++ {
			joined += words[j]
			if j > i {
				next := joined
				if j+1 < len(words) {
					next += words[j+1]
				}
				if memorySpacedKey.MatchString(next) {
					return true
				}
			}
		}
	}
	return false
}

// secretLooksLikeAValue is whether value, written after label and what gives its value, could be the secret
// itself: for a password any word that isn't an everyday English or French one, for a key or token one of 8
// characters with a digit or a sign in it, or of 16. A placeholder ($TOKEN, <key>, …) isn't one.
func secretLooksLikeAValue(label, value string) bool {
	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, "<") || strings.HasPrefix(value, "…") || strings.Trim(value, ".x*") == "" {
		return false
	}
	if strings.Contains(label, "pass") || strings.HasSuffix(label, "pwd") || label == "mdp" || label == "mot de passe" {
		word := strings.ToLower(strings.TrimRight(value, ".!?"))
		return len(value) >= 4 && !memoryEverydayWord(word)
	}
	value = strings.TrimRight(value, ".!?")
	plain := strings.IndexFunc(value, func(c rune) bool { return !unicode.IsLetter(c) }) < 0
	return len(value) >= 16 || len(value) >= 8 && !plain
}

var (
	memoryEmail = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
	// Phone numbers: international (+33 6 12 34 56 78), French (06 12 34 56 78), North American ((415) 555-0134).
	memoryPhones = []*regexp.Regexp{
		regexp.MustCompile(`\+[1-9]\d{0,2}[\s.-]?\(?\d{1,4}\)?(?:[\s.-]?\d{2,4}){2,5}`),
		regexp.MustCompile(`(?:^|\D)0[1-9](?:[\s.-]?\d{2}){4}(?:$|\D)`),
		regexp.MustCompile(`(?:^|\D)\(?\d{3}\)?[\s.-]\d{3}[\s.-]\d{4}(?:$|\D)`),
	}
	// A run of digits, one space or dash apart at most, that a card number could be in.
	memoryDigitRun = regexp.MustCompile(`\d(?:[ -]?\d)+`)
	// An IBAN's start: its country, its check digits, then its groups.
	memoryIBAN = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{1,4})+`)
	// A home folder with its owner's name: /Users/<name>, /home/<name>, C:\Users\<name>.
	memoryHomeFolder = regexp.MustCompile(`(?i)(?:/users/|/home/|\b[a-z]:[\\/]+users[\\/]+)([^/\\\s'"]+)`)
)

// memoryPersonal names the kind of personal data text holds, or "".
func memoryPersonal(text string) string {
	if memoryEmail.MatchString(text) {
		return "an e-mail address"
	}
	for _, phone := range memoryPhones {
		for _, match := range phone.FindAllString(text, -1) {
			if countDigits(match) >= 8 {
				return "a phone number"
			}
		}
	}
	for _, run := range memoryDigitRun.FindAllString(text, -1) {
		digits := strings.NewReplacer(" ", "", "-", "").Replace(run)
		if len(digits) >= 13 && len(digits) <= 19 && luhn(digits) {
			return "a card number"
		}
	}
	for _, match := range memoryIBAN.FindAllString(text, -1) {
		if ibanIn(match) {
			return "a bank account number (an IBAN)"
		}
	}
	return ""
}

func countDigits(text string) int {
	n := 0
	for _, c := range text {
		if c >= '0' && c <= '9' {
			n++
		}
	}
	return n
}

// luhn is whether digits pass the Luhn check every card number does.
func luhn(digits string) bool {
	sum := 0
	for i := range len(digits) {
		d := int(digits[len(digits)-1-i] - '0')
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// ibanIn is whether match, or a start of it (it may run into the words after), is an IBAN: 15 to 34 letters and
// digits whose check (ISO 13616, mod 97) gives 1.
func ibanIn(match string) bool {
	compact := strings.ToUpper(strings.ReplaceAll(match, " ", ""))
	for size := min(len(compact), 34); size >= 15; size-- {
		if ibanCheck(compact[:size]) {
			return true
		}
	}
	return false
}

func ibanCheck(iban string) bool {
	rest := 0
	for _, c := range iban[4:] + iban[:4] {
		value := 0
		switch {
		case c >= '0' && c <= '9':
			value = int(c - '0')
		case c >= 'A' && c <= 'Z':
			value = int(c-'A') + 10
		default:
			return false
		}
		for _, d := range strconv.Itoa(value) {
			rest = (rest*10 + int(d-'0')) % 97
		}
	}
	return rest == 1
}

// memoryHomeNotAName is whether every home folder text names is no one's: the Mac's shared one, or a placeholder
// ($USER, %USERNAME%, ~, a … or the words "name", "you", "me").
func memoryHomeNotAName(text string) bool {
	for _, match := range memoryHomeFolder.FindAllStringSubmatch(text, -1) {
		name := strings.ToLower(match[1])
		if !slices.Contains([]string{"shared", "name", "you", "me", "…", "...", "~"}, name) && !strings.HasPrefix(name, "$") && !strings.HasPrefix(name, "%") {
			return false
		}
	}
	return true
}

// otherWords are everyday words of the languages nearest English and French (German, Spanish, Italian,
// Portuguese, Dutch) that neither writes, to tell a note in one of them.
var otherWords = wordSet("und der das nicht ist ein eine immer bitte wenn oder auch ich wir sie el los las y para siempre nunca pero esta este hacer usar cuando gli della sempre prima che sono fare questo quando senza os um uma fazer com voce isso het een niet altijd nooit voor met maar ook zijn wanneer " +
	"sin sem ohne senza puedes puoi pode kannst darfst geht frage puede puo podes preguntar chiedere perguntar fragen vragen zum zur nein aber nur noch schon muy tambien anche perche porque nao")

// memoryEveryday are the everyday English and French words a note in either is written with: Summary's own
// lists (summary.go), which stay as they are, and more, so a short plain note in either reads as one.
var memoryEveryday = wordSet("" +
	// English
	"about after again all also always am any anything are as ask at back be because been before being best both by " +
	"call came can't change check come could day did didn't does doesn't don't done down each else even every first " +
	"from get give go goes going good got had has have he her here him his how if into isn't it's its just keep kept " +
	"know last let let's like long look made many may me more most much must need needed needs never new next none " +
	"now off old once one only our out over own put rather read really right run runs said same say see set she " +
	"short show since so some soon still such sure take tell than then their them there these they thing things " +
	"those though through time too try two under until up us use used uses using very want was way well were when " +
	"where which while why will without work works would write yet you're we're i'm that's owner owner's " +
	"answer answers branch build builds code commit commits docs file files fine folder keep lines line note notes " +
	"plain reply review stored test tests written week weekly daily morning mornings evening monthly hours main " +
	// French
	"à a ai alors après as assez avant avoir bon c ça car ce ceci cela celle celui ces chaque chez ici d dans de " +
	"depuis dès donc dont du elle elles en encore entre es été être eu fait faire fois j jamais l là le leur leurs " +
	"lui m même moins n ne ni nos notre nous ont où par parce pareil partout peu peut peuvent plus plutôt pour " +
	"pourquoi puis qu quand quel quelle quelque rien s sa sans se selon ses si sinon son sont sous souvent surtout " +
	"t tard tôt toujours tous tout toute toutes très trop un une vers voilà vos votre vu y " +
	"réponse réponses court courte courtes courts fichier fichiers dossier test tests matin soir semaine jour " +
	"brief briefs manager managers vault keychain trousseau gestionnaire noter")

// memoryWorkWords are more everyday English and French words, and the words of the work, so a short ordinary note
// reads as English or French by its share of everyday words (memoryLanguage, FXMEM2R2). A tool's or a gate's name
// is an English word here too. After a password's name, or alone in a field of a note that names a secret, one
// of them is still read as its value (memoryNamedSecretValue).
var memoryWorkWords = wordSet("" +
	"able accept access act action actions add added adds ahead already another anyone anyway apply approval area " +
	"arrived asking avoid away bad base be between big bills bug bugs buy card case cases changes " +
	"changed changing charge checking checks clean cleanly clear client clients comment comments company company's " +
	"computer consent copy create created credit credits current data date default delete deploy deploying deploys " +
	"desk desks detail details doc document draft drop edit edits emergency emoji end english error errors example " +
	"except explain exports fast feature features feel fence few find fix fixed fixes flat focused folders force " +
	"format forward french friday fridays full functional function functions green guard guide guides hard help " +
	"hello hire hires hiring issue issues key keys keyboard land landed large launch less licence license " +
	"lift light lint list live local lock lockfile log login logins logs low mail mean message messages method " +
	"mobile mode model models monday mondays month name names next nothing number ok okay open order other others " +
	"outside overwrite page part pass passes password passwords pay payload people plan plans please point prefer " +
	"prefers prefix pro prod production project projects promote prune public publish pull push pushing ready " +
	"reads real redeploy relationship release remote removing renew replies repo repos report reports reset rule " +
	"rules safe says schema screen script scripts secret secrets send sharing ship shows sign size skip skipped slow " +
	"small spaces spend start started state status step steps store strict style styling submit subscribe sync " +
	"table tag task tasks team ten text tier titles today token tokens tomorrow tool tools top trash tree turn type " +
	"types unpublish update updates upgrade upload user users value values variable variables version " +
	"versions wait waiting weekday weekdays weekend whenever window windows word words writes yourself " +
	"api app apps ci cli css eas eslint expo git github html json linux mac macos node npm opus pnpm pr prettier " +
	"setup smolcorp sql sudo supabase swift tailwind typescript ui url vercel wiki yaml " +
	"accord acheter anglais attendre besoin branche branches clé client clients code commande commander commit " +
	"committer consulter crédits demande demander dépenser déploiement déployer déposer données effacer envoyer " +
	"écrire évite éviter facture français française garde garder lance lancer libre librement messages mise " +
	"modifier mot noms parle passe peine phrases principale projet projets pousser préfère prévenir publier publié " +
	"régler répondre réponds serveur sortir souscrire supprimer tournent tutoiement téléverser utilise " +
	"utiliser variables vendredi vert vieux virer volonté œuvre œuvres " +
	// FXMEM2R3: more of the commonest English and French words and the words of the work, so that ordinary notes
	// read as English or French clause by clause (RT126MEM2R3 found 7 of 20 held as unread).
	"above across actually address admin agent agents allow almost alone along alright although among amount anything " +
	"anywhere around asked assets async auth author auto available backend backup bar basic become begin behind below " +
	"beta better beyond block body bottom box break bring browser button buttons cache called calls care careful catch " +
	"cause center chart choose class classes close cloud color colors column columns command commands common commonly " +
	"compact complete component components config configure confirm connect console constant contains content context " +
	"control convert core correct cost count cover crash custom customer customers cut dark dashboard database dates " +
	"days db deal debug decide define delay deprecated describe design designer dev development device diff different " +
	"directly directory disable display double during early easy edge editor effect either email empty enable enough " +
	"entire entry env environment event events ever exact exactly examples exist expect expected expire expires " +
	"explicit export extra fail failed fails failing false fetch fetching field fields figma final fit flag flags " +
	"follow font fonts footer form forms frontend future general generate given global goal great group half handle " +
	"handler handling happen happens header heavy height hidden hide high hold home hook hooks host hour icon icons " +
	"idea ids image images import imports include including index info input inputs inside install instead item items " +
	"itself javascript jest join kebab camel snake pascal label labels language languages later layout lead learn " +
	"least leave left legacy length level library libraries limit link links load loading locale logic longer loop " +
	"lower machine maintain make makes manage manual mark match max maybe meaning menu middle migration migrations min " +
	"minutes miss mock mocks modal module modules months move multiple named naming native nav near nearly necessary " +
	"network night normal notice null object objects often older online option options otherwise outdated output pages " +
	"pair paragraph parent parse path paths pattern patterns per perhaps place platform plus port possible post " +
	"preferred present press pretty preview previous primary private problem problems process product props provide " +
	"provider query queries quick quickly quite range rate raw react reason recent record redux refactor regular " +
	"related remember render repeat replace request requests require required response responses rest result results " +
	"return reuse role root rotate rotated rotation round route routes row rows rust sample save scale screens scroll " +
	"search second section security select self sense sentence separate server servers service services session " +
	"sessions setting settings several shared sheet should side simple single site sizes slack snake soft solid " +
	"something sometimes sort source space special specific speed split sprint stable stack staging standard stay stop " +
	"storage stores storing string strings structure stuff sub support switch system tab tables take talk target tech " +
	"term terms testing theme themes thin third thought three ticket tickets timeout times tiny title together total " +
	"track true twice typed unit units unless unused upper usage useful utc valid validate validation vendor via view " +
	"views visible vitest vue want wants warning warnings watch web website weeks weird what's who whole width within " +
	"won't worker workflow wrap wrapper wrong year years zod keeps flaky flake flakes " +
	"aime aimer ajoute ajouter ancien ancienne anciens application appli arrête attention aucun aucune autre autres " +
	"avance bas base beaucoup bientôt cas certain certains chemin chose choses clair claire commence commencer comment " +
	"composant composants compte config contenu côté couleur couleurs courant d'abord date dates début demain dernier " +
	"dernière dessus dessous deux devant dit dire dis doit doivent donne donner dossiers droite écran écrans écris " +
	"écrit écrite écrits écrivent élément éléments ensuite enfin équipe erreur erreurs essai essaie étape étapes " +
	"exemple exemples facile façon faut fin fini fonction fonctions fond font format forme formulaire gauche gère " +
	"gérer grand grande grands gros haut heure heures hier image images jours langue langues lent lente lents lettre " +
	"lettres lien liens lire lis liste listes long longue longues lisible maintenant maquette maquettes manière marche " +
	"message mets mettre mieux minute minutes mode moment monde mois montre montrer mots navigateur nom nombre nouveau " +
	"nouvelle nouveaux nouvelles objet objets option options ordre page pages paramètre paramètres partie passer " +
	"pendant personne petit petite petits petites place plan plusieurs police possible premier première premiers prend " +
	"prendre presque prêt prête prochain prochaine propre propres quelqu'un question questions raison règle règles " +
	"relis relire rend rendre reste retour revue rapide rapides sait sauvegarde semaines service seul seule seulement " +
	"simple simples site sombre sombres stocke stockée stockées stocker stockés style suite suivant suivre sujet " +
	"tableau tâche tâches taille tel telle temps texte thème trois type types utilisateur utilisateurs utilisent " +
	"valeur valeurs version vérifie vérifier veut veulent vieille voir voit vrai web")

// memoryEverydayWord is whether word (lower case) is an everyday English or French one, or French's elided
// article or pronoun before an apostrophe (l', d', qu', c', j', n', s', m', t').
func memoryEverydayWord(word string) bool {
	if englishWords[word] || frenchWords[word] || memoryEveryday[word] {
		return true
	}
	if before, _, cut := strings.Cut(word, "'"); cut {
		return memoryEveryday[before] || englishWords[before] || frenchWords[before]
	}
	return false
}

// memoryEverydayUnaccented is memoryEverydayWord for a word with its accents off.
func memoryEverydayUnaccented(word string) bool {
	if memoryEverydayFolded[word] {
		return true
	}
	before, _, cut := strings.Cut(word, "'")
	return cut && memoryEverydayFolded[before]
}

// memoryEverydayFolded are the everyday English and French words with their accents off.
var memoryEverydayFolded = func() map[string]bool {
	folded := map[string]bool{}
	for _, list := range []map[string]bool{englishWords, frenchWords, memoryEveryday, memoryWorkWords} {
		for word := range list {
			folded[memoryUnaccented(word)] = true
		}
	}
	return folded
}()

// memoryLanguage says why the note is refused as in another language ("" if it isn't), or else, unread, that the
// checks can't be sure it is in English or French: in a field of three words or more, in a sentence or a clause of
// three words or more (B4, FXMEM2R3: split at . ; : , ! ? and at and, or, but, then, et, ou, mais, puis), or in the
// whole note, fewer than memoryEverydayShare of the words of memoryCountedLetters letters or more are everyday
// English or French ones (N2, FXMEM2R2); or, in any field or clause, more words are another language's than
// English's or French's (Puoi rilasciare). One English sentence beside an Italian one doesn't make it English, and
// the next language no list knows is held too. A note is refused when, in a field or in the whole note, two words or
// more are another language's, and more than are English's or French's. Its letters were checked already
// (memoryEnglishOrFrench).
func memoryLanguage(fields []memoryField) (unread bool, refusal string) {
	const refused = "it isn't in English or French: a note is in English or French only, the languages Smolcorp can check it in"
	allOurs, allOthers, allCounted, allOursCounted := 0, 0, 0, 0
	for _, field := range fields {
		for i, text := range append([]string{field.text}, memoryClauses(field.text)...) {
			words, paths := memoryLanguageWords(text)
			ours, others, counted, oursCounted := 0, 0, 0, 0
			for _, word := range words {
				everyday := memoryEverydayUnaccented(word)
				switch {
				case everyday:
					ours++
				case otherWords[word]:
					others++
				}
				if utf8.RuneCountInString(strings.ReplaceAll(word, "'", "")) >= memoryCountedLetters {
					counted++
					if everyday {
						oursCounted++
					}
				}
			}
			if i == 0 && others >= 2 && others > ours {
				return false, refused
			}
			if others > ours || len(words)+paths >= 3 && (memoryFewEveryday(oursCounted, counted) || paths > counted) {
				unread = true
			}
			if i == 0 {
				allOurs, allOthers = allOurs+ours, allOthers+others
				allCounted, allOursCounted = allCounted+counted, allOursCounted+oursCounted
			}
		}
	}
	if allOthers >= 2 && allOthers >= allOurs {
		return false, refused
	}
	return unread || allCounted >= 3 && memoryFewEveryday(allOursCounted, allCounted), ""
}

// memoryClauseBreak is where a field's sentences and clauses part: a sign followed by a space or the end (so a
// version's or a path's dot doesn't), a bracket, a quote, a dash (a spaced hyphen too), or a joining word.
var memoryClauseBreak = regexp.MustCompile(`(?i)[.;:,!?]+(?:\s|$)|[()\[\]"—–]|\s-\s|\s(?:and|or|but|then|et|ou|où|mais|puis)\s`)

// memoryClauses are text's sentences and clauses, when it has more than one.
func memoryClauses(text string) []string {
	clauses := memoryClauseBreak.Split(text, -1)
	if len(clauses) < 2 {
		return nil
	}
	return clauses
}

// memoryLanguageWords are text's words as the language check counts them, in lower case with French's accents off
// (as memoryEverydayUnaccented has them, so an accent typed apart reads the same): letters, with an
// apostrophe inside; a word with digits counts by its letters (rilasciare1 is rilasciare). A number or a commit's
// id is no word; a path, a web address or a placeholder ($HOME) is counted apart, and a field with more of them
// than words is held, so words can't hide as paths.
func memoryLanguageWords(text string) (words []string, paths int) {
	for _, token := range strings.Fields(memoryUnaccented(strings.ToLower(strings.ReplaceAll(text, "’", "'")))) {
		core := strings.TrimFunc(token, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })
		switch {
		case memoryHexWord.MatchString(core) && strings.ContainsFunc(core, unicode.IsDigit):
			continue
		case strings.ContainsAny(token, `/\@$%~`):
			paths++
			continue
		}
		words = append(words, strings.FieldsFunc(core, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' })...)
	}
	return words, paths
}

// memoryEverydayShare is the least share of everyday English or French words a field or a note is read with, and
// memoryCountedLetters how long a word must be to count (a, à, y and i are in other languages too). Tuned on the
// tests' notes: ordinary short notes in English and French pass, and "Subir cambios the same day" (three in five)
// is held.
const (
	memoryEverydayShare  = 2.0 / 3
	memoryCountedLetters = 2
)

func memoryFewEveryday(ours, counted int) bool {
	return counted > 0 && float64(ours) < memoryEverydayShare*float64(counted)
}

// memoryGates are the gated actions, each with the starts of the English and French words that name it. A word
// names it when it starts with one (after re-, un-, auto-, pre-, force- or git-), unless it starts with one of
// except, or when it is one of exact. Words written apart are read joined too (memoryReadings), so a start such
// as bacasable is "bac à sable" and enligne "en ligne".
var memoryGates = []memoryWords{
	{gate: "push", starts: []string{"push", "pouss", "upload", "envoy", "envoi"}, except: []string{"poussier"}},
	{gate: "merge", starts: []string{"merg", "fusion", "squash", "integrat"}, exact: []string{"land", "lands", "landed", "landing"}},
	{gate: "publish", starts: []string{"publish", "publie", "publia", "publions", "publiez", "publication", "release", "ship", "submit", "livr", "enligne"}},
	{gate: "deploy", starts: []string{"deploy", "deploi", "launch", "rollout", "rollsout", "rolledout", "rollingout", "production"}, exact: []string{"live", "prod", "prods"}},
	{gate: "spend", starts: []string{"spend", "spent", "pay", "paid", "purchas", "buy", "bought", "billing", "depens", "paie", "achat", "achet", "charge", "subscri", "abonn"}, except: []string{"payload"}, exact: []string{"order", "orders", "ordered", "ordering"}},
	{gate: "delete", starts: []string{"delet", "remov", "eras", "wipe", "destroy", "purge", "supprim", "suppression", "effac", "detrui", "drop", "truncat", "clear", "vider", "videz", "vidons", "ecras"}, except: []string{"dropdown", "clearly"}, exact: []string{"rm", "rmdir"}},
	{gate: "fence", starts: []string{"fence", "sandbox", "guard", "cloture", "bypass", "noverify", "outside", "bacasable", "gardefou"}, except: []string{"guardian"}},
}

// memoryAsking are the words of a note about asking the owner or their permission, whatever the action: such a
// note is held too (B5, the reviewer's own fix), after the gates, which say more.
var memoryAsking = memoryWords{
	gate:   "ask",
	starts: []string{"ask", "permission", "permit", "approv", "approuv", "approb", "authoriz", "authoris", "autoris", "whenever", "freely", "demand", "librement"},
	exact:  []string{"noneed", "goahead", "atwill", "anytime", "sansme", "sansmoi", "sansmon", "sansma", "pasbesoin", "avolonte"},
}

// memoryWords are the words that name one gate (see memoryGates).
type memoryWords struct {
	gate          string
	starts        []string
	except, exact []string
}

// names is whether word (a reading) names the gate.
func (g memoryWords) names(word string) bool {
	if slices.Contains(g.exact, word) {
		return true
	}
	for _, w := range append([]string{word}, trimPrefixes(word)...) {
		if slices.ContainsFunc(g.starts, func(s string) bool { return strings.HasPrefix(w, s) }) &&
			!slices.ContainsFunc(g.except, func(s string) bool { return strings.HasPrefix(w, s) }) {
			return true
		}
	}
	return false
}

var memoryWordPrefixes = []string{"re", "un", "auto", "pre", "force", "git"}

// memoryBranch are the words of a rule about branches (VICTOR, FXMEM2R3): a rule that names one is held.
var memoryBranch = memoryWords{gate: "branch", starts: []string{"branch"}}

// memoryGate names the gated action fields speak of, or another kind of hold ("grant", "ask", "tighten", "branch",
// "modal", "standing", "condition", "spelling"), or "". The word lists come first, as they say the most; then the
// grammar, which no verb can dodge.
//
// A rule (a feedback note, rule true) that names a gate, a branch, asking or permission is held whatever its
// grammar (VICTOR, FXMEM2R3): the grammar only picks the friendliest reason, and never lets such a rule pass. A note
// of another type that only tightens (memoryOnlyTightens) isn't held for asking.
func memoryGate(fields []memoryField, rule bool) string {
	readings, patterns := memoryReadings(fields)
	for _, word := range readings {
		for _, gate := range memoryGates {
			if gate.names(word) {
				return gate.gate
			}
		}
	}
	if gate := memoryPatternGate(patterns); gate != "" {
		return gate
	}
	texts := memoryPhraseTexts(fields)
	matches := func(phrase *regexp.Regexp) bool {
		return slices.ContainsFunc(texts, phrase.MatchString)
	}
	for _, phrase := range memoryPhraseGates {
		if matches(phrase.words) {
			return phrase.gate
		}
	}
	asking := slices.ContainsFunc(readings, memoryAsking.names)
	switch {
	case matches(memoryGrant), slices.ContainsFunc(fields, func(field memoryField) bool { return memoryWhyNot.MatchString(field.text) }):
		return "grant"
	case asking && memoryOnlyTightens(fields, texts[0]):
		if rule {
			return "tighten"
		}
	case asking:
		return memoryAsking.gate
	}
	modal := slices.ContainsFunc(texts, func(text string) bool {
		return memoryModal.MatchString(strings.ReplaceAll(text, " peut etre ", " peutetre "))
	})
	switch {
	case rule && slices.ContainsFunc(readings, memoryBranch.names):
		return memoryBranch.gate
	case modal:
		return "modal"
	case matches(memoryStanding):
		return "standing"
	case matches(memoryCondition):
		return "condition"
	case memorySpelling(fields):
		return "spelling"
	}
	return ""
}

// memoryOnlyTightens is whether a note about asking only tightens (B2, FXMEM2R3): text has a tightening phrase,
// no word that eases or narrows it, every asking word in the note is inside that phrase, and each field is one
// clause, so no second sentence can ease it ("Always ask first. Permission granted to …" is held).
func memoryOnlyTightens(fields []memoryField, text string) bool {
	if !memoryTighten.MatchString(text) || memoryLoosen.MatchString(text) {
		return false
	}
	for _, field := range fields {
		if strings.ContainsAny(strings.TrimRight(strings.TrimSpace(field.text), ".!"), ".;:,!?") {
			return false
		}
	}
	rest, _ := memoryReadings([]memoryField{{"rest", memoryTighten.ReplaceAllString(text, " ")}})
	return !slices.ContainsFunc(rest, memoryAsking.names)
}

// The grammar of a note that gives leave or a standing order, whatever its verb (N5, FXMEM2R2). The phrases are read
// in the whole note, folded, accents off, signs as spaces, and again with its digits and signs read as letters.
var (
	// memoryPhraseGates are where an action lands, which says its gate: into main (merge), to the remote (push),
	// anywhere on the computer (the fence).
	memoryPhraseGates = []struct {
		gate  string
		words *regexp.Regexp
	}{
		{"merge", regexp.MustCompile(`\b(?:into|onto) (?:the )?(?:main|master|trunk|develop|release|prod|production)\b|\b(?:dans|sur) (?:la branche )?(?:main|master)\b`)},
		{"push", regexp.MustCompile(`\b(?:to|with|onto) (?:the )?(?:remote|origin|upstream)\b|\bsur (?:le )?(?:serveur|depot distant|remote|origin)\b`)},
		{"fence", regexp.MustCompile(`\b(?:any|other|another) (?:folder|directory|dir|repo|repository|project|path|place)s?\b|\banywhere\b|\bon the (?:computer|machine|system)\b|\b(?:whole|entire) (?:computer|machine|disk|drive|system)\b|\bsystem ?wide\b|\bn'importe (?:quel|quelle|ou)\b|\bhors\b|\ben dehors\b|\bautres? (?:dossier|depot|projet|repo)s?\b|\btout le (?:disque|systeme|mac|ordinateur)\b`)},
	}
	// memoryGrant is a note that gives leave or eases a rule: you may, feel free, no need, without, allowed, ok
	// to, tu peux, pas besoin, sans, autorisé…, whatever verb follows.
	memoryGrant = regexp.MustCompile(`\byou (?:may|can|could|are (?:allowed|free|welcome|cleared)|have (?:the |my )?(?:green light|go ahead|ok|okay|blessing|permission))\b|` +
		`\byou're (?:allowed|free|welcome|cleared)\b|\bfeel free\b|\bfree to\b|\bno need\b|\bneedn't\b|\bgo ahead\b|\bgo for it\b|\bwhenever\b|\bfreely\b|` +
		`\b(?:don'?t|do not|never) (?:ask|wait|need|bother|check)\b|\bwithout\b|\ballowed\b|\bpermitted\b|\b(?:ok|okay|fine|safe) (?:to|for)\b|` +
		`\b(?:is|are|it's) (?:ok|okay|fine)\b|\bat will\b|\bany ?time\b|\bgreen light\b|\bon your own\b|\bup to you\b|\byour call\b|\bas you (?:like|wish|see fit)\b|` +
		`\bno (?:\w+ ){0,2}(?:needed|required|necessary)\b|\bnot (?:needed|required|necessary)\b|\bskip (?:asking|the ask|checking)\b|` +
		`\b(?:tu|vous|on) (?:peux|peut|pouvez|as le droit|avez le droit)\b|\ble droit de\b|\bpas besoin\b|\bn'hesitez? pas\b|\blibrement\b|\bsans\b|\bautoris|` +
		`\ba volonte\b|\bquand (?:tu veux|vous voulez|tu le souhaites|bon te semble)\b|\bcomme (?:tu veux|tu le sens|bon te semble)\b|\bpas la peine\b|\binutile de\b|` +
		`\bfeu vert\b|\blibres? de\b|\b(?:ok|d'accord) pour\b|\bpermis\b|\bpas (?:necessaire|oblige|requise?s?)\b|\b(?:aucune?|pas de?) (?:\w+ ){0,2}requise?s?\b|\bfacultati|\bwaived?\b|\bne (?:me )?demande pas\b|` +
		// B1 (FXMEM2R3): the question, the wish and the shrug that give leave, whatever the subject.
		`\b(?:can|could|may|might) (?:you|we|i|they)\b|\bwhy (?:wait|ask|bother|check|hold off)\b|\bif you (?:want|like|wish|prefer|feel like)\b|` +
		`\bas needed\b|\bdiscretion\b|\b(?:acceptable|alright|all right)\b|\btrust (?:you|them|the agents?)\b|\bneed not\b|` +
		`\b(?:don'?t|do not) hesitate\b|'s call\b|s' call\b|\b(?:peux|peut|pouvez|pourrais|pourriez|pourrait) (?:tu|vous|on|il|elle)\b|` +
		`\bpourquoi (?:attendre|demander|pas|se gener)\b|\bsi (?:tu veux|vous voulez|tu le souhaites|ca te dit|tu en as envie)\b|` +
		`\bca ne me (?:derange|gene) pas\b|\bca me va\b|\bje (?:te|vous|leur) fais confiance\b`)
	// memoryTighten and memoryLoosen: a note about asking that only tightens (always ask, ask before, demande
	// avant) and has no word that eases or narrows it (never, not, only, except, apart from, jamais, sauf, hormis…) isn't held
	// for asking: "never ask" reads as a grant.
	memoryTighten = regexp.MustCompile(`\b(?:always (?:ask|check with)|ask (?:me )?(?:first|before)|check with (?:me|the owner) (?:first|before)|demander? (?:moi )?(?:d'abord|avant)|toujours (?:me )?demander)\b`)
	memoryLoosen  = regexp.MustCompile(`\b(?:never|not|no|none|nobody|nothing|except|unless|only|just|but|otherwise|without|jamais|pas|ne|sauf|seulement|juste|mais|sinon|sans|rien|aucun|` +
		`grant|granted|grants|given|gives|stands|ok|okay|fine|free|anyway|anyhow|regardless|donne|donnee|accorde|accordee|quand meme|de toute facon|tout de meme|` +
		`other than|apart from|besides|aside from|save|excluding|hormis|a part|excepte)\b|n't\b|\bn'`)
	// memoryWhyNot is "why not" written as one phrase (a comma between, as in "why, not what", isn't one).
	memoryWhyNot = regexp.MustCompile(`(?i)\bwhy not\b|\bpourquoi pas\b`)
	// memoryModal is a modal of leave or ability, whatever its subject (B1, FXMEM2R3): agents can, the GM may, hires
	// are able to, les agents peuvent, il est possible de. "Peut-être" (maybe) is read apart first.
	memoryModal = regexp.MustCompile(`\b(?:can|cannot|could|may|might|able to|capable of|peut|peuvent|peux|pouvez|pouvons|` +
		`pourra|pourrai|pourras|pourrez|pourront|pourrons|pourrait|pourrais|pourraient|pourriez|pourrions|possible)\b`)
	// memoryCondition is a condition followed by a clause (B1, FXMEM2R3): if the tests pass, when the build is green,
	// as soon as, unless, si, quand, lorsque, dès que, au cas où.
	memoryCondition = regexp.MustCompile(`\b(?:if|when|unless|as soon as|in case|provided|si|quand|lorsque|lorsqu|des que|des qu|au cas ou|a condition|pourvu que)(?: |')[a-z]`)
	// memoryStanding is a standing order: an action at a set time or each time something happens (on Fridays,
	// after each commit, once the tests pass, when ready, le vendredi, dès que).
	memoryStanding = regexp.MustCompile(`\bon (?:mondays|tuesdays|wednesdays|thursdays|fridays|saturdays|sundays|weekends?)\b|` +
		`\bevery (?:monday|tuesday|wednesday|thursday|friday|saturday|sunday|weekend|day|night|morning|evening|week|month|hour)s?\b|` +
		`\b(?:after|following) (?:each|every)\b|\bonce\b|\bwhen (?:ready|done|green|needed|possible|it'?s (?:ready|green|done)|it is (?:ready|green|done)|you (?:like|want|wish|see fit|feel like|are ready))\b|` +
		`\b(?:le|les|chaque|tous les) (?:lundi|mardi|mercredi|jeudi|vendredi|samedi|dimanche|week end|weekend)s?\b|\b(?:apres|a) chaque\b|` +
		`\bchaque (?:fois|jour|soir|matin|semaine|mois)\b|\bdes que\b|\bune fois (?:que|qu)|\btous les (?:jours|soirs|matins|mois)\b|\bquand (?:c'est pret|tu es pret|ca te chante)\b`)
)

// memoryPhraseTexts are fields read as one text for the phrases: folded, accents off, every sign a space, in each
// way memoryVariants reads digits (as digits, then as letters).
func memoryPhraseTexts(fields []memoryField) []string {
	var texts []string
	for k := range memoryVariants {
		var words []string
		for _, field := range fields {
			var b strings.Builder
			for _, c := range memoryUnaccented(guard.Fold(strings.ReplaceAll(field.text, "’", "'"))) {
				switch letters := leetDigits[c]; {
				case c >= 'a' && c <= 'z' || c == '\'':
					b.WriteRune(c)
				case k == 0 && c >= '0' && c <= '9':
					b.WriteRune(c)
				case k > 0 && len(letters) > 0:
					b.WriteString(letters[min(k-1, len(letters)-1)])
				default:
					b.WriteByte(' ')
				}
			}
			words = append(words, strings.Fields(b.String())...)
		}
		texts = append(texts, " "+strings.Join(words, " ")+" ")
	}
	return texts
}

var (
	// memoryLeetInside is a word with digits (or $, €) between its letters, standing for letters: d3ploy, n4ke.
	memoryLeetInside = regexp.MustCompile(`[a-z][0-9$€]+[a-z]`)
	// memoryHexWord is a short git commit id or another hexadecimal number, whose digits are digits.
	memoryHexWord = regexp.MustCompile(`^[0-9a-f]{7,}$`)
	// memoryNumeronyms are the words written with a number inside that everyone writes so.
	memoryNumeronyms = []string{"i18n", "l10n", "a11y", "k8s", "e2e", "b2b", "b2c", "p2p"}
)

// memorySpelling is whether a word in fields is written in disguise, whatever it is (N3/N4, FXMEM2R2): letters
// mixed with a pattern's *, ? or brackets, or with digits standing for letters. A question mark that ends a word
// and brackets around a whole word ([WIP]) are just punctuation.
func memorySpelling(fields []memoryField) bool {
	for _, field := range fields {
		tokens := strings.Fields(memoryUnaccented(guard.Fold(field.text)))
		for i := 1; i+1 < len(tokens); i++ {
			// A pattern's sign alone between letters one or two at a time stands for a letter (D e * l o y, Pu * h).
			if strings.Trim(tokens[i], "*?") == "" && memoryShortLetters(tokens[i-1]) && memoryShortLetters(tokens[i+1]) {
				return true
			}
		}
		for _, token := range tokens {
			word := strings.TrimRight(strings.Trim(memoryUnwrapped(token), `"'(),.;:!`), "?")
			if countLetters(word) > 0 && strings.ContainsAny(word, "*?[]") {
				return true
			}
			if !memoryHexWord.MatchString(word) && !slices.Contains(memoryNumeronyms, word) && memoryLeetInside.MatchString(word) {
				return true
			}
		}
	}
	return false
}

// memoryShortLetters is whether token is one or two letters.
func memoryShortLetters(token string) bool {
	n := countLetters(token)
	return n >= 1 && n <= 2 && n == utf8.RuneCountInString(token)
}

// memoryPatternGate reads each word with a pattern in it (*, ? or […]) as the guard reads one, as everything it
// could fill in (guard.PatternCouldFill, the guard's skeleton reading): pu*h, d?ploy and me??e are push, deploy and
// merge. A word with fewer than three letters written out is just a pattern, as the guard takes one.
func memoryPatternGate(patterns []string) string {
	for _, gate := range append(slices.Clone(memoryGates), memoryAsking) {
		for _, pattern := range patterns {
			for _, word := range append([]string{pattern}, trimPrefixes(pattern)...) {
				if slices.ContainsFunc(gate.starts, func(s string) bool { return guard.PatternCouldFill(word, s, true) }) ||
					slices.ContainsFunc(gate.exact, func(s string) bool { return guard.PatternCouldFill(word, s, false) }) {
					return gate.gate
				}
			}
		}
	}
	return ""
}

func trimPrefixes(word string) []string {
	var out []string
	for _, prefix := range memoryWordPrefixes {
		if rest, cut := strings.CutPrefix(word, prefix); cut && rest != "" {
			out = append(out, rest)
		}
	}
	return out
}

// memoryRunMost is how many words next to each other are read joined: de pl oy, pu s h, d 3 p l 0 y. A word of
// signs alone (--, ...) doesn't count: it is skipped, so filler can't push the pieces apart.
const memoryRunMost = 6

// memoryReadings are the words fields could be read as, folded as the guard folds (guard.Fold: case aside), its
// accents off, the fields one after another, so a field's end runs into the next one's start:
//   - each word as written, each written with signs or digits inside it (de-ploy, d.e.p.l.o.y, pu$h), and with its
//     digits and signs read as letters (d3pl0y, mer9e, de1ete), every way an ambiguous one reads;
//   - every run of up to memoryRunMost words next to each other joined (de ploy, d e ploy, de pl oy), in each of
//     those readings, and letters spelled out apart, however many (p u b l i s h).
//
// patterns are the words and runs with a pattern in them (pl*y, de pl*y, d e p l ? y), for memoryPatternGate.
func memoryReadings(fields []memoryField) (readings, patterns []string) {
	var tokens [][]string // each word's readings, by memoryVariants
	var singles []bool    // whether a word is one character
	for _, field := range fields {
		for _, token := range strings.Fields(memoryUnaccented(guard.Fold(field.text))) {
			token = memoryUnwrapped(token)
			readings = append(readings, strings.FieldsFunc(token, func(c rune) bool { return c < 'a' || c > 'z' })...)
			readings = append(readings, leetReadings(token)...)
			if !strings.ContainsFunc(token, memoryReadable) {
				continue // a word of signs alone (--, ...) is filler: it doesn't break a run
			}
			variants := make([]string, memoryVariants)
			for k := range variants {
				variants[k] = leetReading(token, k)
			}
			tokens, singles = append(tokens, variants), append(singles, utf8.RuneCountInString(token) == 1)
			if word := strings.Trim(token, `"'(),.;:!`); strings.ContainsAny(word, "*?[") && countLetters(word) >= 3 {
				patterns = append(patterns, word)
			}
		}
	}
	for k := range memoryVariants {
		for i := range tokens {
			run := tokens[i][k]
			for j := i + 1; j < len(tokens) && j < i+memoryRunMost; j++ {
				run += tokens[j][k]
				readings = append(readings, run)
				if memoryRunPattern(run) {
					patterns = append(patterns, run)
				}
			}
		}
		spelled, n := "", 0
		for i, single := range singles {
			if !single {
				spelled, n = "", 0
				continue
			}
			if spelled, n = spelled+tokens[i][k], n+1; n > memoryRunMost {
				readings = append(readings, spelled)
				if memoryRunPattern(spelled) {
					patterns = append(patterns, spelled)
				}
			}
		}
	}
	return readings, patterns
}

// memoryRunPattern is whether a run of words is read as a pattern: it has a pattern's sign, and three letters or
// more before the first, as a word must to be read as one (de pl*y, not d* the app).
func memoryRunPattern(run string) bool {
	first := strings.IndexAny(run, "*?[")
	return first >= 0 && countLetters(run[:first]) >= 3
}

// memoryUnwrapped is token with brackets around the whole word taken off when nothing in it is a pattern's sign:
// [WIP] is a tag, not a pattern.
func memoryUnwrapped(token string) string {
	word := strings.Trim(token, `"'(),.;:!?`)
	if len(word) > 2 && word[0] == '[' && word[len(word)-1] == ']' && !strings.ContainsAny(word[1:len(word)-1], "*?[]") {
		return word[1 : len(word)-1]
	}
	return token
}

func countLetters(text string) int {
	return strings.Count(strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' {
			return 'a'
		}
		return -1
	}, text), "a")
}

// leetDigits are the digits and signs read as letters. The ones with two (1 as i or l, 6 as g or b) are read
// both ways (leetReadings), and each way through a run (memoryVariants).
var leetDigits = map[rune][]string{
	'0': {"o"}, '1': {"i", "l"}, '2': {"z"}, '3': {"e"}, '4': {"a"}, '5': {"s"}, '6': {"g", "b"}, '7': {"t"}, '8': {"b"}, '9': {"g"},
	'@': {"a"}, '$': {"s"}, '|': {"l"}, '!': {"i"}, '+': {"t"}, '€': {"e"},
}

// memoryVariants are the ways a run of words is read: its letters alone, then its digits and signs read as
// letters, the first way of each, then the second.
const memoryVariants = 3

// memoryPatternSigns are a pattern's signs, kept in a run's reading so a run is read as a pattern too (de pl*y).
const memoryPatternSigns = "*?[]"

// memoryReadable is whether c can be part of a word's reading: a letter, a digit or sign read as one, or a
// pattern's sign.
func memoryReadable(c rune) bool {
	return c >= 'a' && c <= 'z' || len(leetDigits[c]) > 0 || strings.ContainsRune(memoryPatternSigns, c)
}

// leetReading is token in the way variant reads it (see memoryVariants), its pattern signs kept.
func leetReading(token string, variant int) string {
	var b strings.Builder
	for _, c := range token {
		switch letters := leetDigits[c]; {
		case c >= 'a' && c <= 'z', strings.ContainsRune(memoryPatternSigns, c):
			b.WriteRune(c)
		case variant > 0 && len(letters) > 0:
			b.WriteString(letters[min(variant-1, len(letters)-1)])
		}
	}
	return b.String()
}

// leetReadings are token's readings with its digits and signs as letters, every way the ones with two letters
// read (the first 4 of them, then each the same way).
func leetReadings(token string) []string {
	out := []string{""}
	for _, c := range token {
		letters := []string{string(c)}
		switch {
		case c >= 'a' && c <= 'z':
		case len(leetDigits[c]) > 0:
			letters = leetDigits[c]
			if len(out) >= 16 {
				letters = letters[:1]
			}
		default:
			continue
		}
		var next []string
		for _, prefix := range out {
			for _, letter := range letters {
				next = append(next, prefix+letter)
			}
		}
		out = next
	}
	return out
}

// memoryUnaccented is folded text with French's accents taken off its letters (déployer is deployer, œ is oe),
// and an accent typed apart dropped.
var memoryUnaccented = strings.NewReplacer(
	"à", "a", "â", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e", "î", "i", "ï", "i", "ô", "o", "ù", "u",
	"û", "u", "ü", "u", "ÿ", "y", "œ", "oe", "æ", "ae",
	"\u0300", "", "\u0301", "", "\u0302", "", "\u0308", "", "\u0327", "",
).Replace
