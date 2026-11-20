package desktop

import "testing"

// Off Windows, the code page comes from the locale (RT126WINSCREENR4 N3): LC_ALL, else LC_CTYPE, else LANG. A UTF-8
// locale whose language isn't Chinese, Japanese or Korean is Western; a CJK one, one unset or one in another
// encoding breaks the screen on an Ambiguous character. So does a malformed one: its language is C or 2–3 lowercase
// letters, with jpn, kor, zho and chi CJK too (RT126WINSCREENR5 N1).
func TestTheCodePageOffWindowsComesFromTheLocale(t *testing.T) {
	for _, c := range []struct {
		all, ctype, lang string
		western          bool
	}{
		{"", "", "fr_FR.UTF-8", true},
		{"", "", "en_US.utf8", true},
		{"", "", "de_DE.UTF-8@euro", true},
		{"", "", "C.UTF-8", true},
		{"", "", "ja_JP.UTF-8", false},
		{"", "", "zh_CN.UTF-8", false},
		{"", "", "zh_TW.UTF-8", false},
		{"", "", "ko_KR.UTF-8", false},
		{"", "", "yue_HK.UTF-8", false},
		{"", "", "", false},
		{"", "", "C", false},
		{"", "", "POSIX", false},
		{"", "", "fr_FR", false},
		{"", "", "fr_FR.ISO-8859-1", false},
		{"", "", ".UTF-8", false},
		{"ja_JP.UTF-8", "", "fr_FR.UTF-8", false},
		{"", "ja_JP.UTF-8", "fr_FR.UTF-8", false},
		{"fr_FR.UTF-8", "ja_JP.UTF-8", "ja_JP.UTF-8", true},
		{"", "fr_FR.UTF-8", "ja_JP.UTF-8", true},
		{"C", "", "fr_FR.UTF-8", false},
		// RT126WINSCREENR5 N1: the 3-letter codes, and malformed languages, which read Western before.
		{"", "", "jpn_JP.UTF-8", false},
		{"", "", "kor_KR.UTF-8", false},
		{"", "", "zho_CN.UTF-8", false},
		{"", "", "chi_CN.UTF-8", false},
		{"", "", "eng_GB.UTF-8", true},
		{"", "", "fra.UTF-8", true},
		{"", "", " ja_JP.UTF-8", false},
		{"", "", " fr_FR.UTF-8", false},
		{"", "", "ja-JP.UTF-8", false},
		{"", "", "fr-FR.UTF-8", false},
		{"", "", "zh-Hans.UTF-8", false},
		{"", "", "POSIX.UTF-8", false},
		{"", "", "FR_FR.UTF-8", false},
		{"", "", "Ja_JP.UTF-8", false},
		{"", "", "f_FR.UTF-8", false},
		{"", "", "fren_FR.UTF-8", false},
		{"", "", "c.UTF-8", false},
		{"", "", "f1_FR.UTF-8", false},
		{"", "", "fr\t_FR.UTF-8", false},
	} {
		env := map[string]string{"LC_ALL": c.all, "LC_CTYPE": c.ctype, "LANG": c.lang}
		page := localeCodePage(func(name string) string { return env[name] })
		if page.cjk() == c.western {
			t.Errorf("LC_ALL=%q LC_CTYPE=%q LANG=%q: %+v, Western %v", c.all, c.ctype, c.lang, page, !page.cjk())
		}
	}
}
