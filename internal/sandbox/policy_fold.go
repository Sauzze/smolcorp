package sandbox

import (
	"path"
	"strings"
	"unicode/utf8"
)

// How the Policy reads a path: every spelling that names the same file on a system gives the same key, so no
// spelling slips past a row. Lexical only: a link is the caller's to resolve (the profile's kernel check does it on
// a Mac), and a path that isn't whole is refused before it gets here.
//
//   - `..` and `.` and doubled slashes go (path.Clean).
//   - Mac: the firmlinks and the old links name one place: /System/Volumes/Data/x is /x, and /var, /tmp and /etc
//     are /private/var, /private/tmp and /private/etc. The volume is normalization-insensitive (APFS) and, as made,
//     case-insensitive: a name decomposed (NFD, as HFS+ and many tools write it) is composed (NFC), then the whole
//     is lower-cased. A case-sensitive volume only makes more spellings fall under a row, never fewer.
//   - Windows: backslashes are slashes, the \\?\ prefix goes, case is folded. Linux: bytes as they are.

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
	if folder := "/System/Volumes/Data"; inFolderFold(p, folder) {
		p = "/" + strings.TrimPrefix(p[len(folder):], "/")
	}
	for _, old := range []string{"/var", "/tmp", "/etc"} {
		if inFolderFold(p, old) {
			p = "/private" + p
		}
	}
	return p
}

// inFolderFold is whether p is folder (ASCII) or inside it, in any case, as a Mac's volume compares them.
func inFolderFold(p, folder string) bool {
	return len(p) >= len(folder) && strings.EqualFold(p[:len(folder)], folder) && (len(p) == len(folder) || p[len(folder)] == '/')
}

// key is path's folded spelling on goos, "" when it isn't whole: two spellings of one file have one key.
func key(goos, p string) string { return foldName(goos, spell(goos, p)) }

// foldName is a name, or a pattern of names, as the system compares it.
func foldName(goos, s string) string {
	switch goos {
	case "darwin":
		return strings.ToLower(composed(s))
	case "windows":
		return strings.ToLower(s)
	}
	return s
}

// under is whether key k is base or inside it (both folded).
func under(k, base string) bool {
	return k == base || base == "/" || strings.HasPrefix(k, strings.TrimSuffix(base, "/")+"/")
}

// composed is s with every decomposed letter composed again: the Unicode table's primary composites (canonical
// pairs, composition exclusions left out) and Hangul's syllables, which Unicode composes by rule. Marks after a
// letter are composed in the order written; a name with two marks in an order NFD wouldn't write is left so.
func composed(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if n := len(out); n > 0 {
			last := out[n-1]
			if c, ok := compositions[[2]rune{last, r}]; ok {
				out[n-1] = c
				continue
			}
			const sBase, lBase, vBase, tBase, lCount, vCount, tCount = 0xAC00, 0x1100, 0x1161, 0x11A7, 19, 21, 28
			if last >= lBase && last < lBase+lCount && r >= vBase && r < vBase+vCount {
				out[n-1] = sBase + ((last-lBase)*vCount+(r-vBase))*tCount
				continue
			}
			if si := last - sBase; si >= 0 && si < lCount*vCount*tCount && si%tCount == 0 && r > tBase && r < tBase+tCount {
				out[n-1] = last + (r - tBase)
				continue
			}
		}
		out = append(out, r)
	}
	return string(out)
}

// compositions maps a letter and the mark after it to the one letter NFC writes, from compositionTable.
var compositions = func() map[[2]rune]rune {
	runes := []rune(compositionTable)
	m := make(map[[2]rune]rune, len(runes)/3)
	for i := 0; i+2 < len(runes); i += 3 {
		m[[2]rune{runes[i], runes[i+1]}] = runes[i+2]
	}
	return m
}()

// compositionTable is every canonical primary composite of Unicode 16.0 but Hangul's, as triples: the letter, the
// mark, what they compose to. Made with Python's unicodedata: each code point whose decomposition is two code
// points with no tag, kept when NFC of the two gives it back.
const compositionTable = "" +
	"A\u0300ÀA\u0301ÁA\u0302ÂA\u0303ÃA\u0308ÄA\u030aÅC\u0327ÇE\u0300ÈE\u0301ÉE\u0302ÊE\u0308ËI\u0300ÌI\u0301ÍI\u0302ÎI\u0308ÏN\u0303ÑO\u0300ÒO\u0301ÓO\u0302ÔO\u0303ÕO\u0308ÖU\u0300ÙU\u0301ÚU\u0302ÛU\u0308ÜY\u0301Ýa\u0300àa\u0301áa\u0302âa\u0303ãa\u0308äa\u030aåc\u0327çe\u0300èe\u0301ée\u0302êe\u0308ëi\u0300ìi\u0301íi\u0302î" +
	"i\u0308ïn\u0303ño\u0300òo\u0301óo\u0302ôo\u0303õo\u0308öu\u0300ùu\u0301úu\u0302ûu\u0308üy\u0301ýy\u0308ÿA\u0304Āa\u0304āA\u0306Ăa\u0306ăA\u0328Ąa\u0328ąC\u0301Ćc\u0301ćC\u0302Ĉc\u0302ĉC\u0307Ċc\u0307ċC\u030cČc\u030cčD\u030cĎd\u030cďE\u0304Ēe\u0304ēE\u0306Ĕe\u0306ĕE\u0307Ėe\u0307ėE\u0328Ęe\u0328ęE\u030cĚe\u030cěG\u0302Ĝ" +
	"g\u0302ĝG\u0306Ğg\u0306ğG\u0307Ġg\u0307ġG\u0327Ģg\u0327ģH\u0302Ĥh\u0302ĥI\u0303Ĩi\u0303ĩI\u0304Īi\u0304īI\u0306Ĭi\u0306ĭI\u0328Įi\u0328įI\u0307İJ\u0302Ĵj\u0302ĵK\u0327Ķk\u0327ķL\u0301Ĺl\u0301ĺL\u0327Ļl\u0327ļL\u030cĽl\u030cľN\u0301Ńn\u0301ńN\u0327Ņn\u0327ņN\u030cŇn\u030cňO\u0304Ōo\u0304ōO\u0306Ŏo\u0306ŏO\u030bŐo\u030bő" +
	"R\u0301Ŕr\u0301ŕR\u0327Ŗr\u0327ŗR\u030cŘr\u030cřS\u0301Śs\u0301śS\u0302Ŝs\u0302ŝS\u0327Şs\u0327şS\u030cŠs\u030cšT\u0327Ţt\u0327ţT\u030cŤt\u030cťU\u0303Ũu\u0303ũU\u0304Ūu\u0304ūU\u0306Ŭu\u0306ŭU\u030aŮu\u030aůU\u030bŰu\u030bűU\u0328Ųu\u0328ųW\u0302Ŵw\u0302ŵY\u0302Ŷy\u0302ŷY\u0308ŸZ\u0301Źz\u0301źZ\u0307Żz\u0307żZ\u030cŽ" +
	"z\u030cžO\u031bƠo\u031bơU\u031bƯu\u031bưA\u030cǍa\u030cǎI\u030cǏi\u030cǐO\u030cǑo\u030cǒU\u030cǓu\u030cǔÜ\u0304Ǖü\u0304ǖÜ\u0301Ǘü\u0301ǘÜ\u030cǙü\u030cǚÜ\u0300Ǜü\u0300ǜÄ\u0304Ǟä\u0304ǟȦ\u0304Ǡȧ\u0304ǡÆ\u0304Ǣæ\u0304ǣG\u030cǦg\u030cǧK\u030cǨk\u030cǩO\u0328Ǫo\u0328ǫǪ\u0304Ǭǫ\u0304ǭƷ\u030cǮʒ\u030cǯj\u030cǰG\u0301Ǵg\u0301ǵ" +
	"N\u0300Ǹn\u0300ǹÅ\u0301Ǻå\u0301ǻÆ\u0301Ǽæ\u0301ǽØ\u0301Ǿø\u0301ǿA\u030fȀa\u030fȁA\u0311Ȃa\u0311ȃE\u030fȄe\u030fȅE\u0311Ȇe\u0311ȇI\u030fȈi\u030fȉI\u0311Ȋi\u0311ȋO\u030fȌo\u030fȍO\u0311Ȏo\u0311ȏR\u030fȐr\u030fȑR\u0311Ȓr\u0311ȓU\u030fȔu\u030fȕU\u0311Ȗu\u0311ȗS\u0326Șs\u0326șT\u0326Țt\u0326țH\u030cȞh\u030cȟA\u0307Ȧa\u0307ȧ" +
	"E\u0327Ȩe\u0327ȩÖ\u0304Ȫö\u0304ȫÕ\u0304Ȭõ\u0304ȭO\u0307Ȯo\u0307ȯȮ\u0304Ȱȯ\u0304ȱY\u0304Ȳy\u0304ȳ¨\u0301΅Α\u0301ΆΕ\u0301ΈΗ\u0301ΉΙ\u0301ΊΟ\u0301ΌΥ\u0301ΎΩ\u0301Ώϊ\u0301ΐΙ\u0308ΪΥ\u0308Ϋα\u0301άε\u0301έη\u0301ήι\u0301ίϋ\u0301ΰι\u0308ϊυ\u0308ϋο\u0301όυ\u0301ύω\u0301ώϒ\u0301ϓϒ\u0308ϔЕ\u0300ЀЕ\u0308ЁГ\u0301ЃІ\u0308ЇК\u0301Ќ" +
	"И\u0300ЍУ\u0306ЎИ\u0306Йи\u0306йе\u0300ѐе\u0308ёг\u0301ѓі\u0308їк\u0301ќи\u0300ѝу\u0306ўѴ\u030fѶѵ\u030fѷЖ\u0306Ӂж\u0306ӂА\u0306Ӑа\u0306ӑА\u0308Ӓа\u0308ӓЕ\u0306Ӗе\u0306ӗӘ\u0308Ӛә\u0308ӛЖ\u0308Ӝж\u0308ӝЗ\u0308Ӟз\u0308ӟИ\u0304Ӣи\u0304ӣИ\u0308Ӥи\u0308ӥО\u0308Ӧо\u0308ӧӨ\u0308Ӫө\u0308ӫЭ\u0308Ӭэ\u0308ӭУ\u0304Ӯу\u0304ӯУ\u0308Ӱ" +
	"у\u0308ӱУ\u030bӲу\u030bӳЧ\u0308Ӵч\u0308ӵЫ\u0308Ӹы\u0308ӹا\u0653آا\u0654أو\u0654ؤا\u0655إي\u0654ئە\u0654ۀہ\u0654ۂے\u0654ۓन\u093cऩर\u093cऱळ\u093cऴ\u09c7\u09be\u09cb\u09c7\u09d7\u09cc\u0b47\u0b56\u0b48\u0b47\u0b3e\u0b4b\u0b47\u0b57\u0b4cஒ\u0bd7ஔ\u0bc6\u0bbe\u0bca\u0bc7\u0bbe\u0bcb\u0bc6\u0bd7\u0bcc\u0c46\u0c56\u0c48\u0cbf\u0cd5\u0cc0\u0cc6\u0cd5\u0cc7\u0cc6\u0cd6\u0cc8\u0cc6\u0cc2\u0cca\u0cca\u0cd5\u0ccb\u0d46\u0d3e\u0d4a\u0d47\u0d3e\u0d4b\u0d46\u0d57\u0d4c\u0dd9\u0dca\u0dda\u0dd9\u0dcf\u0ddc\u0ddc\u0dca\u0ddd\u0dd9\u0ddf\u0dde" +
	"ဥ\u102eဦᬅ\u1b35ᬆᬇ\u1b35ᬈᬉ\u1b35ᬊᬋ\u1b35ᬌᬍ\u1b35ᬎᬑ\u1b35ᬒ\u1b3a\u1b35\u1b3b\u1b3c\u1b35\u1b3d\u1b3e\u1b35\u1b40\u1b3f\u1b35\u1b41\u1b42\u1b35\u1b43A\u0325Ḁa\u0325ḁB\u0307Ḃb\u0307ḃB\u0323Ḅb\u0323ḅB\u0331Ḇb\u0331ḇÇ\u0301Ḉç\u0301ḉD\u0307Ḋd\u0307ḋD\u0323Ḍd\u0323ḍD\u0331Ḏd\u0331ḏD\u0327Ḑd\u0327ḑD\u032dḒd\u032dḓĒ\u0300Ḕē\u0300ḕĒ\u0301Ḗē\u0301ḗE\u032dḘe\u032dḙE\u0330Ḛe\u0330ḛ" +
	"Ȩ\u0306Ḝȩ\u0306ḝF\u0307Ḟf\u0307ḟG\u0304Ḡg\u0304ḡH\u0307Ḣh\u0307ḣH\u0323Ḥh\u0323ḥH\u0308Ḧh\u0308ḧH\u0327Ḩh\u0327ḩH\u032eḪh\u032eḫI\u0330Ḭi\u0330ḭÏ\u0301Ḯï\u0301ḯK\u0301Ḱk\u0301ḱK\u0323Ḳk\u0323ḳK\u0331Ḵk\u0331ḵL\u0323Ḷl\u0323ḷḶ\u0304Ḹḷ\u0304ḹL\u0331Ḻl\u0331ḻL\u032dḼl\u032dḽM\u0301Ḿm\u0301ḿM\u0307Ṁm\u0307ṁM\u0323Ṃm\u0323ṃ" +
	"N\u0307Ṅn\u0307ṅN\u0323Ṇn\u0323ṇN\u0331Ṉn\u0331ṉN\u032dṊn\u032dṋÕ\u0301Ṍõ\u0301ṍÕ\u0308Ṏõ\u0308ṏŌ\u0300Ṑō\u0300ṑŌ\u0301Ṓō\u0301ṓP\u0301Ṕp\u0301ṕP\u0307Ṗp\u0307ṗR\u0307Ṙr\u0307ṙR\u0323Ṛr\u0323ṛṚ\u0304Ṝṛ\u0304ṝR\u0331Ṟr\u0331ṟS\u0307Ṡs\u0307ṡS\u0323Ṣs\u0323ṣŚ\u0307Ṥś\u0307ṥŠ\u0307Ṧš\u0307ṧṢ\u0307Ṩṣ\u0307ṩT\u0307Ṫt\u0307ṫ" +
	"T\u0323Ṭt\u0323ṭT\u0331Ṯt\u0331ṯT\u032dṰt\u032dṱU\u0324Ṳu\u0324ṳU\u0330Ṵu\u0330ṵU\u032dṶu\u032dṷŨ\u0301Ṹũ\u0301ṹŪ\u0308Ṻū\u0308ṻV\u0303Ṽv\u0303ṽV\u0323Ṿv\u0323ṿW\u0300Ẁw\u0300ẁW\u0301Ẃw\u0301ẃW\u0308Ẅw\u0308ẅW\u0307Ẇw\u0307ẇW\u0323Ẉw\u0323ẉX\u0307Ẋx\u0307ẋX\u0308Ẍx\u0308ẍY\u0307Ẏy\u0307ẏZ\u0302Ẑz\u0302ẑZ\u0323Ẓz\u0323ẓ" +
	"Z\u0331Ẕz\u0331ẕh\u0331ẖt\u0308ẗw\u030aẘy\u030aẙſ\u0307ẛA\u0323Ạa\u0323ạA\u0309Ảa\u0309ảÂ\u0301Ấâ\u0301ấÂ\u0300Ầâ\u0300ầÂ\u0309Ẩâ\u0309ẩÂ\u0303Ẫâ\u0303ẫẠ\u0302Ậạ\u0302ậĂ\u0301Ắă\u0301ắĂ\u0300Ằă\u0300ằĂ\u0309Ẳă\u0309ẳĂ\u0303Ẵă\u0303ẵẠ\u0306Ặạ\u0306ặE\u0323Ẹe\u0323ẹE\u0309Ẻe\u0309ẻE\u0303Ẽe\u0303ẽÊ\u0301Ếê\u0301ếÊ\u0300Ề" +
	"ê\u0300ềÊ\u0309Ểê\u0309ểÊ\u0303Ễê\u0303ễẸ\u0302Ệẹ\u0302ệI\u0309Ỉi\u0309ỉI\u0323Ịi\u0323ịO\u0323Ọo\u0323ọO\u0309Ỏo\u0309ỏÔ\u0301Ốô\u0301ốÔ\u0300Ồô\u0300ồÔ\u0309Ổô\u0309ổÔ\u0303Ỗô\u0303ỗỌ\u0302Ộọ\u0302ộƠ\u0301Ớơ\u0301ớƠ\u0300Ờơ\u0300ờƠ\u0309Ởơ\u0309ởƠ\u0303Ỡơ\u0303ỡƠ\u0323Ợơ\u0323ợU\u0323Ụu\u0323ụU\u0309Ủu\u0309ủƯ\u0301Ứ" +
	"ư\u0301ứƯ\u0300Ừư\u0300ừƯ\u0309Ửư\u0309ửƯ\u0303Ữư\u0303ữƯ\u0323Ựư\u0323ựY\u0300Ỳy\u0300ỳY\u0323Ỵy\u0323ỵY\u0309Ỷy\u0309ỷY\u0303Ỹy\u0303ỹα\u0313ἀα\u0314ἁἀ\u0300ἂἁ\u0300ἃἀ\u0301ἄἁ\u0301ἅἀ\u0342ἆἁ\u0342ἇΑ\u0313ἈΑ\u0314ἉἈ\u0300ἊἉ\u0300ἋἈ\u0301ἌἉ\u0301ἍἈ\u0342ἎἉ\u0342Ἇε\u0313ἐε\u0314ἑἐ\u0300ἒἑ\u0300ἓἐ\u0301ἔἑ\u0301ἕΕ\u0313Ἐ" +
	"Ε\u0314ἙἘ\u0300ἚἙ\u0300ἛἘ\u0301ἜἙ\u0301Ἕη\u0313ἠη\u0314ἡἠ\u0300ἢἡ\u0300ἣἠ\u0301ἤἡ\u0301ἥἠ\u0342ἦἡ\u0342ἧΗ\u0313ἨΗ\u0314ἩἨ\u0300ἪἩ\u0300ἫἨ\u0301ἬἩ\u0301ἭἨ\u0342ἮἩ\u0342Ἧι\u0313ἰι\u0314ἱἰ\u0300ἲἱ\u0300ἳἰ\u0301ἴἱ\u0301ἵἰ\u0342ἶἱ\u0342ἷΙ\u0313ἸΙ\u0314ἹἸ\u0300ἺἹ\u0300ἻἸ\u0301ἼἹ\u0301ἽἸ\u0342ἾἹ\u0342Ἷο\u0313ὀο\u0314ὁὀ\u0300ὂ" +
	"ὁ\u0300ὃὀ\u0301ὄὁ\u0301ὅΟ\u0313ὈΟ\u0314ὉὈ\u0300ὊὉ\u0300ὋὈ\u0301ὌὉ\u0301Ὅυ\u0313ὐυ\u0314ὑὐ\u0300ὒὑ\u0300ὓὐ\u0301ὔὑ\u0301ὕὐ\u0342ὖὑ\u0342ὗΥ\u0314ὙὙ\u0300ὛὙ\u0301ὝὙ\u0342Ὗω\u0313ὠω\u0314ὡὠ\u0300ὢὡ\u0300ὣὠ\u0301ὤὡ\u0301ὥὠ\u0342ὦὡ\u0342ὧΩ\u0313ὨΩ\u0314ὩὨ\u0300ὪὩ\u0300ὫὨ\u0301ὬὩ\u0301ὭὨ\u0342ὮὩ\u0342Ὧα\u0300ὰε\u0300ὲη\u0300ὴ" +
	"ι\u0300ὶο\u0300ὸυ\u0300ὺω\u0300ὼἀ\u0345ᾀἁ\u0345ᾁἂ\u0345ᾂἃ\u0345ᾃἄ\u0345ᾄἅ\u0345ᾅἆ\u0345ᾆἇ\u0345ᾇἈ\u0345ᾈἉ\u0345ᾉἊ\u0345ᾊἋ\u0345ᾋἌ\u0345ᾌἍ\u0345ᾍἎ\u0345ᾎἏ\u0345ᾏἠ\u0345ᾐἡ\u0345ᾑἢ\u0345ᾒἣ\u0345ᾓἤ\u0345ᾔἥ\u0345ᾕἦ\u0345ᾖἧ\u0345ᾗἨ\u0345ᾘἩ\u0345ᾙἪ\u0345ᾚἫ\u0345ᾛἬ\u0345ᾜἭ\u0345ᾝἮ\u0345ᾞἯ\u0345ᾟὠ\u0345ᾠὡ\u0345ᾡὢ\u0345ᾢὣ\u0345ᾣ" +
	"ὤ\u0345ᾤὥ\u0345ᾥὦ\u0345ᾦὧ\u0345ᾧὨ\u0345ᾨὩ\u0345ᾩὪ\u0345ᾪὫ\u0345ᾫὬ\u0345ᾬὭ\u0345ᾭὮ\u0345ᾮὯ\u0345ᾯα\u0306ᾰα\u0304ᾱὰ\u0345ᾲα\u0345ᾳά\u0345ᾴα\u0342ᾶᾶ\u0345ᾷΑ\u0306ᾸΑ\u0304ᾹΑ\u0300ᾺΑ\u0345ᾼ¨\u0342῁ὴ\u0345ῂη\u0345ῃή\u0345ῄη\u0342ῆῆ\u0345ῇΕ\u0300ῈΗ\u0300ῊΗ\u0345ῌ᾿\u0300῍᾿\u0301῎᾿\u0342῏ι\u0306ῐι\u0304ῑϊ\u0300ῒι\u0342ῖϊ\u0342ῗ" +
	"Ι\u0306ῘΙ\u0304ῙΙ\u0300Ὶ῾\u0300῝῾\u0301῞῾\u0342῟υ\u0306ῠυ\u0304ῡϋ\u0300ῢρ\u0313ῤρ\u0314ῥυ\u0342ῦϋ\u0342ῧΥ\u0306ῨΥ\u0304ῩΥ\u0300ῪΡ\u0314Ῥ¨\u0300῭ὼ\u0345ῲω\u0345ῳώ\u0345ῴω\u0342ῶῶ\u0345ῷΟ\u0300ῸΩ\u0300ῺΩ\u0345ῼ←\u0338↚→\u0338↛↔\u0338↮⇐\u0338⇍⇔\u0338⇎⇒\u0338⇏∃\u0338∄∈\u0338∉∋\u0338∌∣\u0338∤∥\u0338∦∼\u0338≁≃\u0338≄≅\u0338≇" +
	"≈\u0338≉=\u0338≠≡\u0338≢≍\u0338≭<\u0338≮>\u0338≯≤\u0338≰≥\u0338≱≲\u0338≴≳\u0338≵≶\u0338≸≷\u0338≹≺\u0338⊀≻\u0338⊁⊂\u0338⊄⊃\u0338⊅⊆\u0338⊈⊇\u0338⊉⊢\u0338⊬⊨\u0338⊭⊩\u0338⊮⊫\u0338⊯≼\u0338⋠≽\u0338⋡⊑\u0338⋢⊒\u0338⋣⊲\u0338⋪⊳\u0338⋫⊴\u0338⋬⊵\u0338⋭か\u3099がき\u3099ぎく\u3099ぐけ\u3099げこ\u3099ごさ\u3099ざし\u3099じす\u3099ずせ\u3099ぜそ\u3099ぞ" +
	"た\u3099だち\u3099ぢつ\u3099づて\u3099でと\u3099どは\u3099ばは\u309aぱひ\u3099びひ\u309aぴふ\u3099ぶふ\u309aぷへ\u3099べへ\u309aぺほ\u3099ぼほ\u309aぽう\u3099ゔゝ\u3099ゞカ\u3099ガキ\u3099ギク\u3099グケ\u3099ゲコ\u3099ゴサ\u3099ザシ\u3099ジス\u3099ズセ\u3099ゼソ\u3099ゾタ\u3099ダチ\u3099ヂツ\u3099ヅテ\u3099デト\u3099ドハ\u3099バハ\u309aパヒ\u3099ビヒ\u309aピフ\u3099ブフ\u309aプヘ\u3099ベヘ\u309aペ" +
	"ホ\u3099ボホ\u309aポウ\u3099ヴワ\u3099ヷヰ\u3099ヸヱ\u3099ヹヲ\u3099ヺヽ\u3099ヾ𐗒\u0307𐗉𐗚\u0307𐗤𑂙\U000110ba𑂚𑂛\U000110ba𑂜𑂥\U000110ba𑂫\U00011131\U00011127\U0001112e\U00011132\U00011127\U0001112f\U00011347\U0001133e\U0001134b\U00011347\U00011357\U0001134c𑎂\U000113c9𑎃𑎄\U000113bb𑎅𑎋\U000113c2𑎎𑎐\U000113c9𑎑\U000113c2\U000113c2\U000113c5\U000113c2\U000113b8\U000113c7\U000113c2\U000113c9\U000113c8\U000114b9\U000114ba\U000114bb\U000114b9\U000114b0\U000114bc\U000114b9\U000114bd\U000114be\U000115b8\U000115af\U000115ba\U000115b9\U000115af\U000115bb\U00011935\U00011930\U00011938\U0001611e\U0001611e\U00016121\U0001611e\U00016129\U00016122\U0001611e\U0001611f\U00016123\U00016129\U0001611f\U00016124\U0001611e\U00016120\U00016125\U00016121\U0001611f\U00016126\U00016122\U0001611f\U00016127\U00016121\U00016120\U00016128𖵨𖵨𖵩𖵩" +
	"𖵪𖵪"
