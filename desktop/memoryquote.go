package desktop

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The owner's sentence behind a quote (1.26, MEM4; SIERRA's rule for RT126MEM4 B1). The GM quotes a few of the
// owner's words; the note keeps the whole sentence that holds them, as the owner typed or said it, and where the
// quote sits in it (memorySource's QuoteStart and QuoteEnd). The quote is only a pointer: the sentence is what the
// note says the owner said, and what the owner's card shows.
//
// A quote that leaves out a negation of its sentence, or cuts one in two ("ne" in, "pas" out), doesn't stand as the
// owner's words: the proposal is refused as a quote and held for the owner's tap, whose card shows the whole sentence
// (memoryQuoteHold). A negation wholly inside the quote is the owner's own, and stands.

// ownerSentence finds quote (as ownerPlain gives it) in line, the owner's words as the ledger keeps them, where
// MEM3's ledger finds it (whole words, spaces folded). It gives the sentence that holds it, verbatim from line, where
// the quote starts and ends in it in bytes, and in how many places of line the quote is found: only one counts, so
// the sentence is never a guess.
func ownerSentence(line, quote string) (sentence string, start, end, found int) {
	// line with its spaces folded as ownerPlain folds them, and where each of its bytes is in line.
	var plain strings.Builder
	var at []int
	space := false
	for i, r := range line {
		if unicode.IsSpace(r) {
			space = plain.Len() > 0
			continue
		}
		if space {
			plain.WriteByte(' ')
			at = append(at, i)
			space = false
		}
		plain.WriteRune(r)
		for k := range utf8.RuneLen(r) {
			at = append(at, i+k)
		}
	}
	places := wordsAt(plain.String(), quote)
	if len(places) != 1 || quote == "" {
		return "", 0, 0, len(places)
	}
	from, to := at[places[0]], at[places[0]+len(quote)-1]+1
	// The sentence: from the end of the last one before the quote to the end of the first one after it.
	open, close, ends := 0, len(line), sentenceEnds(line)
	for _, b := range ends {
		if b <= from {
			open = b
		} else if b >= to {
			close = b
			break
		}
	}
	// A sentence after it that opens on a negation ("No, never.", "Non, jamais.") answers it, so it is part of what
	// the owner said (RT126MEM4R2 F1): it is kept and read with the quote's, as many as follow so.
	for close < len(line) && opensOnNegation(line[close:]) {
		next := len(line)
		for _, b := range ends {
			if b > close {
				next = b
				break
			}
		}
		close = next
	}
	// SIERRA's backstop (FXMEM4R4): a negation anywhere before the quote in what the owner said, outside it, holds
	// the note whatever the sentences above say, as a list's lead-in or a heading two lines up may turn the words
	// around. What's kept then runs from the paragraph of the nearest such negation to the end of the quote's
	// paragraph: the hold (quoteDropsNegation) reads the negation in it, and the owner's card shows it all.
	if at := negationBefore(line, from); at >= 0 {
		open = min(open, paragraphStart(line, at))
		close = max(close, paragraphEnd(line, to))
	}
	sentence = line[open:close]
	lead := len(sentence) - len(strings.TrimLeftFunc(sentence, unicode.IsSpace))
	sentence = strings.TrimSpace(sentence)
	return sentence, from - open - lead, to - open - lead, 1
}

// negationBefore is where the last negation of line that starts before byte from begins, or -1 for none: a word
// isNegation reads as one, as the hold reads the words kept. One that runs on into the quote ("Don't" quoted from
// its "t") starts before it too.
func negationBefore(line string, from int) int {
	at := -1
	for _, w := range sentenceWords(line) {
		if w.from >= from {
			break
		}
		if isNegation(line, w) {
			at = w.from
		}
	}
	return at
}

// paragraphStart is where the paragraph holding byte at of text starts: after the last blank line before it, or 0.
func paragraphStart(text string, at int) int {
	start := 0
	for i := 0; i < at; {
		end := strings.IndexByte(text[i:], '\n')
		if end < 0 {
			break
		}
		if end += i; strings.TrimSpace(text[i:end]) == "" {
			start = end + 1
		}
		i = end + 1
	}
	return start
}

// paragraphEnd is where the paragraph holding byte at of text ends: at the next blank line, or the end of text.
func paragraphEnd(text string, at int) int {
	for i := at; i < len(text); {
		next := strings.IndexByte(text[i:], '\n')
		if next < 0 {
			break
		}
		i += next + 1
		if end := strings.IndexByte(text[i:], '\n'); end >= 0 && strings.TrimSpace(text[i:i+end]) == "" {
			return i - 1
		}
	}
	return len(text)
}

// sentenceEnds is where each sentence of text ends, in bytes: after a run of . ! ? or …, and the closing quotes or
// brackets after it, where a space follows and then a capital, an opening quote or the end. "e.g. npm" or
// "v1.2" end nothing, and nor does a period after an initial or a title ("Mr. Dupont", "U.S. East", "M. Dupont":
// RT126MEM4R2 B1), so a sentence is never cut short of a negation by an abbreviation: when in doubt it runs on.
// Nor does an ellipsis, … or two periods or more, spaced or not ("Never push to... GitHub"), nor a period after a
// number or before one ("Never do these: 1. Push to main. 2. Skip the tests."): the owner pauses, or lists, inside a
// sentence (RT126MEM4R3 B1, B2). A bracket starts none either, as in a list's "(2)", "(b)" or "(ii)" (RT126MEM4R4
// B1), and the periods after one that ends nothing end nothing too ("to .. GitHub", "the M.. mirror").
// Splitting is for what's kept and shown: a negation before the quote holds the note whatever it says (SIERRA,
// FXMEM4R4; ownerSentence).
func sentenceEnds(text string) []int {
	var ends []int
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !strings.ContainsRune(".!?…", r) {
			i += size
			continue
		}
		j := i + size
		if r == '.' && (abbreviationBefore(text[:i]) || strings.TrimRightFunc(text[:i], unicode.IsSpace) != text[:i]) {
			for r, size := utf8.DecodeRuneInString(text[j:]); r == '.' || r == '…'; r, size = utf8.DecodeRuneInString(text[j:]) {
				j += size
			}
			i = j
			continue
		}
		for j < len(text) {
			r, size := utf8.DecodeRuneInString(text[j:])
			if !strings.ContainsRune(".!?…\"'»”’)]", r) {
				break
			}
			j += size
		}
		if run := text[i:j]; strings.ContainsRune(run, '…') || strings.Count(run, ".") >= 2 {
			i = j
			continue
		}
		rest := strings.TrimLeftFunc(text[j:], unicode.IsSpace)
		next, _ := utf8.DecodeRuneInString(rest)
		if rest == "" || (len(rest) < len(text[j:]) && (unicode.IsUpper(next) || strings.ContainsRune("\"«“'", next))) {
			ends = append(ends, j)
		}
		i = j
	}
	return ends
}

// abbreviations are the titles and short forms a period after doesn't end a sentence, in lower case, English and
// French: "Mr. Dupont", "Mme. Martin", "etc. Never…", "la Sté. Durand", "Jan. Fifteen". One that also ends sentences
// ("me.", "no.", "art.") only makes the sentence run on, so more of it is kept and read.
var abbreviations = map[string]bool{"mr": true, "mrs": true, "ms": true, "messrs": true, "dr": true, "st": true, "ste": true,
	"jr": true, "sr": true, "prof": true, "rev": true, "hon": true, "gen": true, "gov": true, "lt": true, "col": true,
	"capt": true, "sgt": true, "mme": true, "mmes": true, "mlle": true, "mlles": true, "mm": true, "me": true, "pr": true,
	"mgr": true, "inc": true, "ltd": true, "co": true, "corp": true, "vs": true, "etc": true, "cf": true, "no": true,
	"ex": true, "approx": true,
	// RT126MEM4R3 F1': French firms and addresses, with or without their accent, references, and the months.
	"sté": true, "cie": true, "ets": true, "av": true, "bd": true, "env": true, "réf": true, "ref": true, "art": true,
	"fig": true, "jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true, "aug": true, "sep": true,
	"sept": true, "oct": true, "nov": true, "dec": true, "janv": true, "fév": true, "fev": true, "févr": true,
	"fevr": true, "avr": true, "juil": true, "déc": true}

// abbreviationBefore is whether text ends in a word a period after it only shortens: a single letter, which is an
// initial (M., J., the S of U.S.), one of abbreviations, or a number, which is a list's (1. Push to main).
func abbreviationBefore(text string) bool {
	word := text[len(strings.TrimRightFunc(text, unicode.IsLetter)):]
	number := text[len(strings.TrimRightFunc(text, isWordRune)):]
	return utf8.RuneCountInString(word) == 1 || abbreviations[strings.ToLower(word)] ||
		(number != "" && strings.TrimLeftFunc(number, unicode.IsDigit) == "")
}

// opensOnNegation is whether text, the rest of a line after a sentence, starts with a negation: its first word is one.
func opensOnNegation(text string) bool {
	words := sentenceWords(text)
	return len(words) > 0 && isNegation(text, words[0])
}

// The negations a quote may not leave out (SIERRA). English: never, not (do not), no and the like, and every
// contraction in n't (don't, won't, can't), with or without its apostrophe as people type. French: ne or n' with what
// follows it (pas, plus, rien, jamais…), and jamais, aucun, rien, pas or ni on their own, as speech drops the ne.
// "non" is one too, but not as the start of a word ("non-blocking", "non-régression"). So are avoid, stop, évite and
// arrête (SIERRA, FXMEM4R4): a note quoted after one is held.
var (
	negationWords = map[string]bool{"never": true, "not": true, "no": true, "nor": true, "cannot": true, "none": true,
		"nothing": true, "nobody": true, "neither": true, "dont": true, "doesnt": true, "didnt": true, "wont": true, "cant": true,
		"isnt": true, "arent": true, "wasnt": true, "werent": true, "shouldnt": true, "mustnt": true, "couldnt": true,
		"wouldnt": true, "havent": true, "hasnt": true, "neednt": true,
		"jamais": true, "aucun": true, "aucune": true, "aucuns": true, "aucunes": true, "rien": true, "pas": true, "ni": true,
		// SIERRA's ruling (FXMEM4R4): avoid and stop, évite and arrête, as the owner types them, accent or not.
		"avoid": true, "avoids": true, "avoiding": true, "stop": true, "stops": true, "stopping": true,
		"évite": true, "évites": true, "évitez": true, "éviter": true, "evite": true, "evites": true, "evitez": true,
		"eviter": true, "arrête": true, "arrêtes": true, "arrêtez": true, "arrêter": true, "arrete": true, "arretes": true,
		"arretez": true, "arreter": true}
	// negationEnds are the words a French ne or n' goes with, the first after it in its sentence.
	negationEnds = map[string]bool{"pas": true, "plus": true, "rien": true, "jamais": true, "personne": true, "point": true,
		"guère": true, "guere": true, "aucun": true, "aucune": true, "nulle": true, "nul": true, "ni": true}
)

// negationWord is a word of a sentence: where it is, verbatim, and how it reads (sentenceWords). parts are how its
// pieces read, when a character dropped from it or an apostrophe splits it ("qu'aucun" is qu and aucun): a negation
// among them is read as one too.
type negationWord struct {
	from, to int
	word     string
	parts    []string
}

// sentenceWords are the words of a sentence, read however the owner dressed them (RT126MEM4R5 B1; FXMEM4R5), so a
// negation reads as one whatever markup, spelling or keyboard it came in:
//   - a word is letters and digits only, so markup around or in one (_never_, __Never__, ~~, `, \_, a tag) and any
//     other punctuation splits it off. Not isWordRune, which counts _ as a letter;
//   - an apostrophe inside one joins it (don't, n'utilise), typed as ' ’ ‘ ʼ ′ ´ or ` (wordApostrophe), and reads ';
//   - a zero-width character, a soft hyphen or any other unseen format character inside one is dropped from how it
//     reads, and so is an accent typed apart (NFD), as a Mac's file names have it;
//   - each letter reads as wordFold folds it: in lower case, and wide, styled, circled and ligatured letters as the
//     plain ones, as NFKC folds them.
//
// A French n' is a word of its own, so "n'utilise" is n' and utilise.
func sentenceWords(sentence string) []negationWord {
	var words []negationWord
	for i := 0; i < len(sentence); {
		r, size := utf8.DecodeRuneInString(sentence[i:])
		if wordFold(r) == "" {
			i += size
			continue
		}
		var word, part strings.Builder
		var parts []string
		cut := 0 // where a leading n' ends in sentence, if the word starts with one
		j := i
	scan:
		for j < len(sentence) {
			r, size := utf8.DecodeRuneInString(sentence[j:])
			if folded := wordFold(r); folded != "" {
				word.WriteString(folded)
				part.WriteString(folded)
				j += size
				continue
			}
			after, _ := utf8.DecodeRuneInString(sentence[j+size:])
			switch {
			case unicode.Is(unicode.M, r):
			case unicode.Is(unicode.Cf, r):
				parts = append(parts, part.String())
				part.Reset()
			case wordApostrophe(r) && j+size < len(sentence) && wordFold(after) != "":
				if word.String() == "n" && cut == 0 {
					cut = j + size
				}
				word.WriteByte('\'')
				parts = append(parts, part.String())
				part.Reset()
			default:
				break scan
			}
			j += size
		}
		if rest, ok := strings.CutPrefix(word.String(), "n'"); ok && rest != "" && cut > 0 {
			words = append(words, negationWord{i, cut, "n'", nil})
			i = cut
			continue
		}
		if len(parts) > 0 {
			parts = append(parts, part.String())
		}
		words = append(words, negationWord{i, j, word.String(), parts})
		i = j
	}
	return words
}

// wordApostrophe is a character people type as the apostrophe of don't or n'utilise: the typewriter one, the
// typographic one a Mac or a phone makes (’), the opening quote (‘), the modifier letter (ʼ), the prime (′), the
// acute accent of some European keyboards (´), the backtick and the wide one.
func wordApostrophe(r rune) bool { return strings.ContainsRune("'’‘‛ʼ′´`＇", r) }

// wordFold is how r reads in a word: "" if it isn't a letter or digit, else in lower case, and as the plain letter or
// letters for a wide (ＮＥＶＥＲ), styled (𝐍𝐞𝐯𝐞𝐫), circled (ⓝⓞ) or ligatured (ﬆop) one, as NFKC folds them. A
// local fold of NFKC for the letters a negation is written in: the module carries no Unicode normalization, and MEM2
// refuses these forms in a note anyway (memorySpecialForm, styledLetter); this reads them should one ever get past.
// An accent stays (évite): negationWords and negationEnds list each word with and without it.
func wordFold(r rune) string {
	switch {
	case wordApostrophe(r): // ʼ is a letter to Unicode; here it's the apostrophe of donʼt
		return ""
	case r >= 0xFF01 && r <= 0xFF5E: // wide forms of ASCII
		r -= 0xFF01 - '!'
	case r >= 0x1D400 && r <= 0x1D6A3: // styled Latin letters, A to z in each of thirteen styles
		if k := (r - 0x1D400) % 52; k < 26 {
			r = 'A' + k
		} else {
			r = 'a' + k - 26
		}
	case r >= 0x24B6 && r <= 0x24CF: // circled capitals
		r = 'A' + r - 0x24B6
	case r >= 0x24D0 && r <= 0x24E9: // circled small letters
		r = 'a' + r - 0x24D0
	case r >= 0xFB00 && r <= 0xFB06:
		return [...]string{"ff", "fi", "fl", "ffi", "ffl", "st", "st"}[r-0xFB00]
	}
	if plain, ok := letterlikeFold[r]; ok {
		r = plain
	}
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return ""
	}
	return string(unicode.ToLower(r))
}

// letterlikeFold are the letter-like symbols that fill the styled alphabets' gaps (ℎ for an italic h), and the long
// s, as NFKC folds them.
var letterlikeFold = map[rune]rune{'ℂ': 'C', 'ℊ': 'g', 'ℋ': 'H', 'ℌ': 'H', 'ℍ': 'H', 'ℎ': 'h', 'ℐ': 'I', 'ℑ': 'I',
	'ℒ': 'L', 'ℓ': 'l', 'ℕ': 'N', 'ℙ': 'P', 'ℚ': 'Q', 'ℛ': 'R', 'ℜ': 'R', 'ℝ': 'R', 'ℤ': 'Z', 'ℨ': 'Z', 'ℬ': 'B',
	'ℭ': 'C', 'ℯ': 'e', 'ℰ': 'E', 'ℱ': 'F', 'ℳ': 'M', 'ℴ': 'o', 'ℹ': 'i', 'ſ': 's'}

// isNegation is whether w, a word of text, is a negation on its own or the ne of one, or has one among its parts.
func isNegation(text string, w negationWord) bool {
	switch w.word {
	case "ne", "n'":
		return true
	case "non":
		return !strings.HasPrefix(text[w.to:], "-")
	}
	if negationWords[w.word] || strings.HasSuffix(w.word, "n't") {
		return true
	}
	for _, part := range w.parts {
		if part == "ne" || part == "non" || negationWords[part] {
			return true
		}
	}
	return false
}

// quoteDropsNegation is a negation of sentence the quote at [start, end) leaves out or cuts in two, as the sentence
// has it ("never", "ne … pas"), or "" when every negation of the sentence is wholly inside the quote.
func quoteDropsNegation(sentence string, start, end int) string {
	words := sentenceWords(sentence)
	used := make([]bool, len(words))
	for i, w := range words {
		if used[i] {
			continue
		}
		unit := []negationWord{w}
		switch {
		case w.word == "ne" || w.word == "n'":
			for j := i + 1; j < len(words); j++ {
				if negationEnds[words[j].word] && !used[j] {
					used[j] = true
					unit = append(unit, words[j])
					break
				}
			}
		case isNegation(sentence, w):
		default:
			continue
		}
		var shown []string
		inside := true
		for _, part := range unit {
			shown = append(shown, sentence[part.from:part.to])
			inside = inside && part.from >= start && part.to <= end
		}
		if !inside {
			return strings.Join(shown, " … ")
		}
	}
	return ""
}

// memoryQuoteHold is why a note quoting the owner is held for the owner's tap, with the sentence whole, as the
// owner's card shows it (MEM5), or a zero memoryHold: its quote leaves out a negation of the sentence, or cuts one.
func memoryQuoteHold(source memorySource) memoryHold {
	if source.QuoteEnd == 0 {
		return memoryHold{}
	}
	dropped := quoteDropsNegation(source.Quote, source.QuoteStart, source.QuoteEnd)
	if dropped == "" {
		return memoryHold{}
	}
	return memoryHold{Kind: "quote",
		English: fmt.Sprintf("The GM quoted only part of what you said, leaving out %q. What you said, whole: %q", dropped, source.Quote),
		French:  fmt.Sprintf("Le GM n'a repris qu'une partie de ce que tu as dit, sans « %s ». Ce que tu as dit, en entier : « %s »", dropped, source.Quote)}
}

// quotePointerRefusal says why a source's quote offsets don't point into its sentence, or "": none at all is MEM1's
// note from before, or Setup's; else they must hold whole characters of the sentence.
func quotePointerRefusal(source memorySource) string {
	start, end, sentence := source.QuoteStart, source.QuoteEnd, source.Quote
	switch {
	case start == 0 && end == 0:
		return ""
	case source.Tag != memoryFromSmolcorp && source.Tag != memoryFromTerminal,
		start < 0 || end <= start || end > len(sentence),
		!utf8.RuneStart(sentence[start]) || (end < len(sentence) && !utf8.RuneStart(sentence[end])):
		return "its quote doesn't point into the owner's sentence"
	}
	return ""
}
