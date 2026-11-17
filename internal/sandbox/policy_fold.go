package sandbox

import (
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

// How the Policy reads a path: every spelling that names the same file on a system gives the same key, so no
// spelling slips past a row. Lexical only: a link is the caller's to resolve (the profile's kernel check does it on
// a Mac), and a path that isn't whole is refused before it gets here.
//
//   - `..` and `.` and doubled slashes go (path.Clean).
//   - Mac: the firmlinks and the old links name one place, in any spelling the volume takes for them:
//     /System/Volumes/Data/x is /x, and /var, /tmp and /etc are /private/var, /private/tmp and /private/etc. The
//     volume (APFS, as made) takes two names for one when Unicode's canonical caseless match does (FXPOLICYFOLD,
//     measured name by name on this Mac's volume: TestTheMacsFoldIsUnicodesCanonicalCaselessMatch): normalization
//     and full case folding, so é decomposed is é, the Kelvin sign is k, ſ (the long s) is s, ß is ss and ﬆ is st.
//     The key is that: the name decomposed (NFD), its marks put in canonical order, each letter case folded in full,
//     then put in order and composed again (NFC). A case-sensitive volume only makes more spellings fall under a row,
//     never fewer. The tables are the volume's own Unicode, 17.0 (FXPOLICYFOLDR1: the volume takes exactly the code
//     points 17.0 assigns, TestTheMacsVolumeKnowsNoCodePointTheFoldDoesnt). A name with a code point the tables'
//     Unicode doesn't assign, which a later macOS's volume may know and fold, is denied by Decide (known).
//   - Windows: backslashes are slashes, the \\?\ prefix goes, case is folded. Linux: bytes as they are (ext4 and
//     tmpfs take a name exactly as written, so there is nothing to fold).

// spell is path as the system names it, or "" when it isn't whole (absolute): cleaned, Windows' slashes and the
// Mac's firmlinks put one way. A profile writes this; the Policy matches on its key.
func spell(goos, p string) string {
	if goos == "windows" {
		p = strings.ReplaceAll(p, `\`, "/")
		p = strings.TrimPrefix(p, "//?/")
		if len(p) < 3 || p[1] != ':' || p[2] != '/' {
			return ""
		}
		// The drive stays the drive: `..` stops at its root.
		return p[:2] + path.Clean(p[2:])
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	p = path.Clean(p)
	if goos != "darwin" {
		return p
	}
	if rest, ok := cutFolderFold(p, "/System/Volumes/Data"); ok {
		p = "/" + strings.TrimPrefix(rest, "/")
	}
	for _, old := range []string{"/var", "/tmp", "/etc"} {
		if _, ok := cutFolderFold(p, old); ok {
			p = "/private" + p
		}
	}
	return p
}

// cutFolderFold is what follows folder in p ("" or from a slash on), when p is folder or inside it in any spelling the
// Mac's volume takes for it: its first names, as many as folder has, give folder's key.
func cutFolderFold(p, folder string) (string, bool) {
	end := 0
	for range strings.Count(folder, "/") {
		if end >= len(p) || p[end] != '/' {
			return "", false
		}
		next := strings.IndexByte(p[end+1:], '/')
		if next < 0 {
			end = len(p)
		} else {
			end += 1 + next
		}
	}
	// Most paths are ASCII, and so are the folders: ASCII's fold is its lower case.
	if isASCII(p[:end]) {
		if !strings.EqualFold(p[:end], folder) {
			return "", false
		}
	} else if caseless(p[:end]) != caseless(folder) {
		return "", false
	}
	return p[end:], true
}

// key is path's folded spelling on goos, "" when it isn't whole: two spellings of one file have one key.
func key(goos, p string) string { return foldName(goos, spell(goos, p)) }

// foldName is a name, or a pattern of names, as the system compares it.
func foldName(goos, s string) string {
	switch goos {
	case "darwin":
		return caseless(s)
	case "windows":
		return strings.ToLower(s)
	}
	return s
}

// under is whether key k is base or inside it (both folded).
func under(k, base string) bool {
	return k == base || base == "/" || strings.HasPrefix(k, strings.TrimSuffix(base, "/")+"/")
}

// caseless is s as the Mac's volume compares names: Unicode's canonical caseless match (NFD, full case folding, NFD
// again), composed (NFC) so a letter and its marks stay one letter, as a pattern's `?` takes them. ASCII is only
// lower-cased.
func caseless(s string) string {
	if isASCII(s) {
		return strings.ToLower(s)
	}
	runes := ordered(decomposed(s))
	folded := make([]rune, 0, len(runes))
	for _, r := range runes {
		if to, ok := caseFolds[r]; ok {
			folded = append(folded, to...)
		} else {
			folded = append(folded, r)
		}
	}
	// A fold can change a mark's class (the iota below, U+0345, folds to ι): put them in order again.
	return compose(ordered(folded))
}

// composed is s in NFC: decomposed, its marks put in canonical order, composed again.
func composed(s string) string {
	if isASCII(s) {
		return s
	}
	return compose(ordered(decomposed(s)))
}

// known is whether every code point in s is one the tables' Unicode assigns. The Mac's volume refuses a name with
// any other (EILSEQ) while its Unicode is the tables'; a later one may take it, and fold it as the tables can't.
func known(s string) bool {
	_, ok := unknownIn(s)
	return !ok
}

// unknownIn is the first code point in s the tables' Unicode doesn't assign, if there is one (for New's error).
func unknownIn(s string) (rune, bool) {
	if isASCII(s) {
		return 0, false
	}
	for _, r := range s {
		i := sort.Search(len(assigned), func(i int) bool { return assigned[i][1] >= r })
		if i == len(assigned) || assigned[i][0] > r {
			return r, true
		}
	}
	return 0, false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Hangul's syllables, which Unicode decomposes and composes by rule.
const sBase, lBase, vBase, tBase, lCount, vCount, tCount = 0xAC00, 0x1100, 0x1161, 0x11A7, 19, 21, 28

// decomposed is s's code points fully decomposed (NFD's decomposition, before the marks are put in order).
func decomposed(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if si := r - sBase; si >= 0 && si < lCount*vCount*tCount {
			out = append(out, lBase+si/(vCount*tCount), vBase+si%(vCount*tCount)/tCount)
			if t := si % tCount; t != 0 {
				out = append(out, tBase+t)
			}
		} else if to, ok := decompositions[r]; ok {
			out = append(out, to...)
		} else {
			out = append(out, r)
		}
	}
	return out
}

// ordered is runes with each run of marks (combining class not 0) sorted by class, stably: Unicode's canonical order.
func ordered(runes []rune) []rune {
	for i := 1; i < len(runes); i++ {
		for j := i; j > 0 && combining[runes[j]] != 0 && combining[runes[j-1]] > combining[runes[j]]; j-- {
			runes[j-1], runes[j] = runes[j], runes[j-1]
		}
	}
	return runes
}

// compose is runes, decomposed and in canonical order, composed as NFC composes them: each mark onto the letter
// before it when the two have a primary composite and no mark between them has its class or none, Hangul by rule.
func compose(runes []rune) string {
	if len(runes) == 0 {
		return ""
	}
	out := []rune{runes[0]}
	starter, last := 0, int(combining[runes[0]])
	if last != 0 {
		last = 256 // a mark with no letter before it: nothing composes onto it
	}
	for _, r := range runes[1:] {
		class := int(combining[r])
		if c, ok := composite(out[starter], r); ok && (last < class || last == 0) {
			out[starter] = c
			continue
		}
		if class == 0 {
			starter = len(out)
		}
		last = class
		out = append(out, r)
	}
	return string(out)
}

// composite is the one letter NFC writes for letter and the mark (or Hangul vowel or final) after it.
func composite(letter, mark rune) (rune, bool) {
	if letter >= lBase && letter < lBase+lCount && mark >= vBase && mark < vBase+vCount {
		return sBase + ((letter-lBase)*vCount+(mark-vBase))*tCount, true
	}
	if si := letter - sBase; si >= 0 && si < lCount*vCount*tCount && si%tCount == 0 && mark > tBase && mark < tBase+tCount {
		return letter + (mark - tBase), true
	}
	c, ok := compositions[[2]rune{letter, mark}]
	return c, ok
}

// records reads a table of "|"-ended records, each a code point and what it maps to.
func records(table string) map[rune][]rune {
	m := map[rune][]rune{}
	for _, rec := range strings.Split(strings.TrimSuffix(table, "|"), "|") {
		runes := []rune(rec)
		m[runes[0]] = runes[1:]
	}
	return m
}

var (
	// decompositions maps a code point to its full canonical decomposition, from decompositionTable.
	decompositions = records(decompositionTable)
	// caseFolds maps a decomposed code point to its full case folding, decomposed, from caseFoldTable.
	caseFolds = records(caseFoldTable)
	// combining is each code point's canonical combining class that isn't 0, from combiningTable.
	combining = func() map[rune]uint8 {
		runes := []rune(combiningTable)
		m := make(map[rune]uint8, 1024)
		for i := 0; i+2 < len(runes); i += 3 {
			for r := runes[i]; r <= runes[i+1]; r++ {
				m[r] = uint8(runes[i+2])
			}
		}
		return m
	}()
)

// assigned is assignedTable's ranges, in order: the first code point, the last.
var assigned = func() [][2]rune {
	runes := []rune(assignedTable)
	out := make([][2]rune, 0, len(runes)/2)
	for i := 0; i+1 < len(runes); i += 2 {
		out = append(out, [2]rune{runes[i], runes[i+1]})
	}
	return out
}()

// compositions maps a letter and the mark after it to the one letter NFC writes, from compositionTable.
var compositions = func() map[[2]rune]rune {
	runes := []rune(compositionTable)
	m := make(map[[2]rune]rune, len(runes)/3)
	for i := 0; i+2 < len(runes); i += 3 {
		m[[2]rune{runes[i], runes[i+1]}] = runes[i+2]
	}
	return m
}()
