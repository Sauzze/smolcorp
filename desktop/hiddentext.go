package desktop

import (
	"slices"
	"unicode"
)

// One list of the characters the owner can't see as they are, for every text a GM or a hire writes that the owner
// reads before saying yes: a grant's words in grants.go (RT126FS2R4 B2), and a night job's name, brief and folder
// in N1's nightshift.go (RT126N1 B1), which shares it. Change it here only, and the list stays one.

// blankGlyphs are graphic characters, not default-ignorable, that a font draws as nothing: the blanks on the usual
// lists of invisible characters. The braille blank (U+2800), the musical null notehead (U+1D159) and the Khitan
// filler (U+16FE4) (RT126FS2c B2), and the Hangul fillers (U+3164, U+FFA0, U+115F, U+1160), which
// Other_Default_Ignorable_Code_Point names already but are kept here with the rest.
var blankGlyphs = []rune{0x115F, 0x1160, 0x2800, 0x3164, 0xFFA0, 0x16FE4, 0x1D159}

// hidesText is a character that shows as nothing, or as something else, or moves the text around it: all that
// isn't graphic (the C0 and C1 controls, line breaks, the format characters such as bidi marks and overrides,
// zero-width ones and the BOM, private use, and code points not assigned), every space but the plain one, and
// Unicode's default-ignorable code points (the Hangul fillers, the combining grapheme joiner, the variation
// selectors) and the blanks that are drawn as nothing (blankGlyphs), which show as nothing or as a space.
func hidesText(c rune) bool {
	return !unicode.IsGraphic(c) || unicode.Is(unicode.Zs, c) && c != ' ' ||
		unicode.In(c, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector) || slices.Contains(blankGlyphs, c)
}
