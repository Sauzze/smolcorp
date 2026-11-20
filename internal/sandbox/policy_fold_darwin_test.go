package sandbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXPOLICYFOLD on this Mac, with stand-ins only (owner Q5 (a)): the Mac's volume takes a name in Unicode's full case
// folding, so ſ (the long s) is s, ß is ss, ﬆ is st, ﬁ is fi and the Kelvin sign is k (FXDESKCASE's trace measured
// them). Each denied place of a stand-in desk, spelled that way, is first checked to be the same file outside any
// sandbox, then tried under the real profile, made by PrepareProfile, and put to the Policy's Decide: each must be
// refused as its plain spelling is. Each check is only "ok" or the error's name; the stand-in key is never printed.

// fullFold spells s as the Mac's volume takes it for the same name: each "ss", "st", "fi", "s" and "k" in the given
// order replaced, at most once each, by ß, ﬆ, ﬁ, ſ and the Kelvin sign.
func fullFold(s string, letters ...string) string {
	alias := map[string]string{"ss": "\u00df", "st": "\ufb06", "fi": "\ufb01", "s": "\u017f", "k": "\u212a"}
	for _, l := range letters {
		s = strings.Replace(s, l, alias[l], 1)
	}
	return s
}

func TestOnAMacAFullyFoldedSpellingOfADeniedPlaceIsDeniedToo(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	made, err := os.MkdirTemp("", "sbf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "hs")
	support := filepath.Join(home, "Ls", "ADs")
	own := filepath.Join(support, "hires", "h1")
	hires := filepath.Join(home, "P", "app-hires")
	root, other := filepath.Join(hires, "h1"), filepath.Join(hires, "h2")
	repo := filepath.Join(home, "P", "app")
	git := filepath.Join(repo, ".git")
	sockets := filepath.Join(base, "sk")
	files := map[string]string{
		filepath.Join(support, "setup-key"): "FAKEKEY", filepath.Join(support, "hires", "h2", "BRIEF.md"): "another brief",
		filepath.Join(own, "BRIEF.md"): "brief", filepath.Join(other, "main.go"): "package main", filepath.Join(repo, "main.go"): "package main",
		filepath.Join(git, "config"): "[core]\n", filepath.Join(git, "HEAD"): "ref: refs/heads/stable\n", filepath.Join(git, "refs", "heads", "main"): "1111\n",
		filepath.Join(git, "worktrees", "h1", "HEAD"): "1111\n", filepath.Join(home, ".ssh", "id_x"): "FAKEKEY",
		filepath.Join(home, ".claude", ".credentials.json"): "FAKEKEY", filepath.Join(sockets, "h2", "s"): "", filepath.Join(sockets, "h1", "f"): "",
		filepath.Join(base, "elsewhere", "x"): "",
	}
	for at, text := range files {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", OtherWorktrees: []string{"h1", "h2"}, DefaultBranch: "stable", HiresFolder: hires,
		SocketParent: sockets, OwnSockets: filepath.Join(sockets, "h1"), RemoteLogin: true})
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}

	type step struct {
		what, at, to string
		op           Op
		plain        string // the place spelled plainly, which must be refused too (allowed, for a control)
		there        bool   // whether the place is there: its spelling is then first checked to be the same file
		allowed      bool   // a control: a name the volume keeps apart from any denied one, which the desk may make
	}
	steps := []step{
		// The support folder, whole: the key by ſ, by the Kelvin sign, by both, its folders by ſ; the folder listed.
		{"read", support + "/" + fullFold("setup-key", "s"), "", Read, support + "/setup-key", true, false},
		{"read", support + "/" + fullFold("setup-key", "k"), "", Read, support + "/setup-key", true, false},
		{"read", support + "/" + fullFold("setup-key", "s", "k"), "", Read, support + "/setup-key", true, false},
		{"read", home + "/" + fullFold("Ls", "s") + "/" + fullFold("ADs", "s") + "/setup-key", "", Read, support + "/setup-key", true, false},
		{"read", support + "/hire" + fullFold("s", "s") + "/h2/BRIEF.md", "", Read, support + "/hires/h2/BRIEF.md", true, false},
		{"list", home + "/Ls/" + fullFold("ADs", "s"), "", Read, support, true, false},
		{"write", support + "/" + fullFold("added-ss", "ss"), "", Write, support + "/added-ss", false, false},
		{"rename", home + "/" + fullFold("Ls", "s"), base + "/elsewhere/L", Move, home + "/Ls", true, false},
		// The ssh keys while Remote Login answers, by ß and by ſ; the account folder's sign-in.
		{"read", home + "/" + fullFold(".ssh", "ss") + "/id_x", "", Read, home + "/.ssh/id_x", true, false},
		{"read", home + "/" + fullFold(".ssh", "s") + "/id_x", "", Read, home + "/.ssh/id_x", true, false},
		{"read", home + "/.claude/" + fullFold(".credentials.json", "s"), "", Read, home + "/.claude/.credentials.json", true, false},
		// The plants, not there yet: by ſ, by ﬁ, by ß, by the Kelvin sign.
		{"write", home + "/" + fullFold(".zshrc", "s"), "", Write, home + "/.zshrc", false, false},
		{"write", home + "/" + fullFold(".profile", "fi"), "", Write, home + "/.profile", false, false},
		{"write", home + "/" + fullFold(".gitconfig", "fi"), "", Write, home + "/.gitconfig", false, false},
		{"write", home + "/.ssh/" + fullFold("authorized_keys", "k"), "", Write, home + "/.ssh/authorized_keys", false, false},
		{"write", home + "/" + fullFold(".ssh", "ss") + "/rc", "", Write, home + "/.ssh/rc", false, false},
		// The shared git places: its config by ﬁ, the default branch, not there yet, by ﬆ and by ſ.
		{"write", git + "/" + fullFold("config", "fi"), "", Write, git + "/config", true, false},
		{"write", git + "/refs/heads/" + fullFold("stable", "st"), "", Write, git + "/refs/heads/stable", false, false},
		{"write", git + "/refs/heads/" + fullFold("stable", "s"), "", Write, git + "/refs/heads/stable", false, false},
		// Another desk's place, and the owner's checkout, by ſ in a folder's name.
		{"write", home + "/P/" + fullFold("app-hires", "s") + "/h2/main.go", "", Write, other + "/main.go", true, false},
		// The socket folder, by ſ and the Kelvin sign: a new folder in it, another desk's file in it.
		{"mkdir", base + "/" + fullFold("sk", "s", "k") + "/h9", "", Write, sockets + "/h9", false, false},
		{"unlink", sockets + "/h2/" + fullFold("s", "s"), "", Write, sockets + "/h2/s", true, false},
		// Controls: the same letters in names nothing denies are made, so a refusal above is the row's, not the letter's.
		{"write", home + "/" + fullFold(".zshrc-notes", "s"), "", Write, home + "/.zshrc-notes", false, true},
		{"write", root + "/" + fullFold("stable-fi", "st", "fi"), "", Write, root + "/stable-fi", false, true},
	}
	var args [][]string
	for _, s := range steps {
		if s.there {
			want, err1 := os.Stat(s.plain)
			got, err2 := os.Stat(s.at)
			if err1 != nil || err2 != nil || !os.SameFile(want, got) {
				t.Fatalf("%s isn't %s on this volume (%v, %v): the stand-in can't stand for it", s.at, s.plain, err1, err2)
			}
		}
		arg := []string{s.what, s.at}
		if s.to != "" {
			arg = append(arg, s.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, s := range steps {
		name := strings.TrimPrefix(s.at, base)
		wantSaid, want := "PermissionError", Deny
		if s.allowed {
			wantSaid, want = "ok", Allow
		}
		t.Logf("seatbelt %s %q: %s", s.what, name, said[i])
		if said[i] != wantSaid {
			t.Errorf("Seatbelt: %s %q: %s, want %s", s.what, name, said[i], wantSaid)
		}
		plain, _ := p.Decide(s.plain, s.op)
		folded, row := p.Decide(s.at, s.op)
		t.Logf("Decide %s %q: %v (%q); plainly: %v", s.op, name, folded, row.Row, plain)
		if plain != want || folded != want {
			t.Errorf("Decide: %s %q: %v (%q), plainly %v; want both %v", s.op, name, folded, row.Row, plain, want)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(git, "config")); string(data) != "[core]\n" {
		t.Errorf("the shared git config is %q", data)
	}
}

// The Mac's volume and the Policy's key take the same names for one (FXPOLICYFOLD), on the test's own temp folder:
// a file made for each key of every code point the tables name (and what they map to), of each one's Go simple-fold
// orbit and every code point Go's Unicode gives one (FXPOLICYFOLDR1: Go 1.27's is 17.0, so a case pair the tables
// don't know is tried too), of every mark Go or the tables know between a letter and a dot below or an acute, in
// either order (a mark whose class the tables get wrong is kept apart), of every 7th code point the tables know in
// the first two planes and 14, and of a few names whose marks come in another order, with no two keys making one
// file (the key never joins what the volume keeps apart); then each name is the file its key made (it never keeps
// apart what the volume joins). A name the volume refuses (EILSEQ: a code point its Unicode doesn't know) can't
// name a file, and is left out. Skipped on a case-sensitive volume, which takes fewer names for one: the key then
// only joins more.
func TestTheMacsFoldIsUnicodesCanonicalCaselessMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PROBE")); err != nil {
		t.Skipf("the temp folder's volume is case-sensitive: %v", err)
	}
	names := map[string]bool{}
	for _, table := range []map[rune][]rune{decompositions, caseFolds} {
		for r, to := range table {
			names[string(r)], names[string(to)] = true, true
		}
	}
	for r := range combining {
		names[string(r)] = true
	}
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) != r || (r>>16 <= 1 || r>>16 == 14) && r%7 == 0 && known(string(r)) && !unicode.Is(unicode.Co, r) {
			names[string(r)] = true
		}
		if unicode.Is(unicode.M, r) || combining[r] != 0 {
			for _, mark := range []string{"\u0323", "\u0301"} {
				names["a"+mark+string(r)+"b"], names["a"+string(r)+mark+"b"] = true, true
			}
		}
	}
	for name := range names {
		for _, r := range name {
			for o := unicode.SimpleFold(r); o != r; o = unicode.SimpleFold(o) {
				names[string(o)] = true
			}
		}
	}
	for _, s := range []string{"e\u0302\u0323", "e\u0323\u0302", "\u1ec7", "\u03b1\u0345\u0301", "\u03b1\u0301\u0345", "\u1fb4",
		"\u0391\u0301\u0345", "\u1f00\u0345", "\u1f88", "\u0130", "i\u0307", "\u1e9e", "ss", "\u00df", "\ufb06", "st", "\u212a", "\u212b"} {
		names[s] = true
	}
	madeFor, refused, failed := map[string]string{}, 0, 0
	fail := func(format string, args ...any) {
		if failed++; failed <= 40 {
			t.Errorf(format, args...)
		}
	}
	for name := range names {
		k := caseless(name)
		if _, ok := madeFor[k]; ok || strings.ContainsRune(name, '/') || strings.ContainsRune(name, 0) {
			continue
		}
		f, err := os.OpenFile(filepath.Join(dir, "q"+k), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, syscall.EILSEQ) {
			refused++
			continue
		}
		if err != nil {
			fail("%+q: the volume took its key %+q for another key's file: %v", name, k, err)
			continue
		}
		f.Close()
		madeFor[k] = name
	}
	for name := range names {
		k := caseless(name)
		if _, ok := madeFor[k]; !ok {
			continue
		}
		want, err1 := os.Stat(filepath.Join(dir, "q"+k))
		got, err2 := os.Stat(filepath.Join(dir, "q"+name))
		if errors.Is(err2, syscall.EILSEQ) {
			refused++
			continue
		}
		if err1 != nil || err2 != nil || !os.SameFile(want, got) {
			fail("%+q: not the file its key %+q made (%v, %v)", name, k, err1, err2)
		}
	}
	t.Logf("%d names, %d keys made, %d names the volume refused, %d failures", len(names), len(madeFor), refused, failed)
	if len(madeFor) < 20000 {
		t.Errorf("only %d keys tried", len(madeFor))
	}
}

// FXPOLICYFOLDR1 (RT126POLICYFOLD's accepted probe): the Mac's volume knows no code point the fold's Unicode doesn't
// assign. It refuses a name with one (EILSEQ), so a volume that takes one is on a later Unicode, which may fold it,
// and the tables must be made again from that Unicode (testdata/foldtables.py). Every such code point of the first
// two planes and plane 14 is tried, and every 61st of the others, each made and removed in the test's own folder.
func TestTheMacsVolumeKnowsNoCodePointTheFoldDoesnt(t *testing.T) {
	dir := t.TempDir()
	tried, taken := 0, []string{}
	for r := rune(1); r <= unicode.MaxRune; r++ {
		if known(string(r)) || r >= 0xd800 && r <= 0xdfff || !(r>>16 <= 1 || r>>16 == 14 || r%61 == 0) {
			continue
		}
		tried++
		at := filepath.Join(dir, "q"+string(r))
		f, err := os.OpenFile(at, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			os.Remove(at)
			taken = append(taken, fmt.Sprintf("U+%04X", r))
			continue
		}
		if !errors.Is(err, syscall.EILSEQ) {
			t.Fatalf("U+%04X: %v, want EILSEQ", r, err)
		}
	}
	t.Logf("tried %d code points Unicode %s doesn't assign; the volume took %d", tried, foldUnicode, len(taken))
	if len(taken) > 0 {
		t.Errorf("the volume takes %d code points Unicode %s doesn't assign (its Unicode is later: make the tables again "+
			"from it), the first: %s", len(taken), foldUnicode, strings.Join(taken[:min(len(taken), 20)], " "))
	}
	if tried < 100000 {
		t.Errorf("only %d code points tried", tried)
	}
}

// FXPOLICYFOLDR1 (RT126POLICYFOLD's pairs probe): two spellings are one file on the Mac's volume exactly when they
// have one key, for each pair the review tried, each in a folder of its own: the second made with O_EXCL must fail
// exactly when it stats as the first. Skipped on a case-sensitive volume.
func TestTheMacsVolumeAndTheKeyAgreeOnEachPairTheReviewTried(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PROBE")); err != nil {
		t.Skipf("the temp folder's volume is case-sensitive: %v", err)
	}
	pairs := []struct{ what, a, b string }{
		{"long s", "\u017f", "s"}, {"long s/S", "\u017f", "S"},
		{"ß/ss", "\u00df", "ss"}, {"ß/SS", "\u00df", "SS"}, {"ẞ/ß", "\u1e9e", "\u00df"}, {"ẞ/ss", "\u1e9e", "ss"}, {"ß/s", "\u00df", "s"},
		{"ﬆ/st", "\ufb06", "st"}, {"ﬆ/ST", "\ufb06", "ST"}, {"ﬅ/ﬆ", "\ufb05", "\ufb06"}, {"ﬁ/fi", "\ufb01", "fi"}, {"ﬁ/FI", "\ufb01", "FI"},
		{"ﬀ/ff", "\ufb00", "ff"}, {"ﬃ/ffi", "\ufb03", "ffi"}, {"ﬃ/ﬀi", "\ufb03", "\ufb00i"},
		{"Kelvin/k", "\u212a", "k"}, {"Kelvin/K", "\u212a", "K"},
		{"Ångström/Å", "\u212b", "\u00c5"}, {"Ångström/å", "\u212b", "\u00e5"}, {"Ångström/a+ring", "\u212b", "a\u030a"},
		{"Ohm/ω", "\u2126", "\u03c9"}, {"µ/μ", "\u00b5", "\u03bc"},
		{"ς/σ", "\u03c2", "\u03c3"}, {"ς/Σ", "\u03c2", "\u03a3"},
		{"İ/i", "\u0130", "i"}, {"İ/I", "\u0130", "I"}, {"İ/i+dot", "\u0130", "i\u0307"}, {"ı/i", "\u0131", "i"}, {"ı/I", "\u0131", "I"}, {"ı/İ", "\u0131", "\u0130"},
		{"Cherokee Ꭰ/ꭰ", "\u13a0", "\uab70"}, {"Cherokee Ᏸ/ᏸ", "\u13f0", "\u13f8"},
		{"Mtavruli Ა/ა", "\u1c90", "\u10d0"}, {"Asomtavruli Ⴀ/ⴀ", "\u10a0", "\u2d00"}, {"Mtavruli/Asomtavruli", "\u1c90", "\u10a0"},
		{"é/e+acute", "\u00e9", "e\u0301"}, {"É/e+acute", "\u00c9", "e\u0301"},
		{"ệ/e+dot+circ", "\u1ec7", "e\u0323\u0302"}, {"ệ/e+circ+dot", "\u1ec7", "e\u0302\u0323"},
		{"가/jamo", "\uac00", "\u1100\u1161"}, {"각/가+final", "\uac01", "\uac00\u11a8"}, {"각/jamo3", "\uac01", "\u1100\u1161\u11a8"},
		{"가/compat jamo", "\uac00", "\u3131\u314f"}, {"jamo old/two", "\u1100\u1100", "\u1101"},
		{"ZWJ", "ab", "a\u200db"}, {"ZWNJ", "ab", "a\u200cb"}, {"soft hyphen", "ab", "a\u00adb"}, {"BOM", "ab", "a\ufeffb"},
		{"VS16", "a", "a\ufe0f"}, {"ZWJ/ZWNJ", "a\u200db", "a\u200cb"},
		{"ᾳ/αι", "\u1fb3", "\u03b1\u03b9"}, {"ᾼ/ᾳ", "\u1fbc", "\u1fb3"}, {"ᾳ/α+ypogegrammeni", "\u1fb3", "\u03b1\u0345"}, {"ι U+1FBE/ι", "\u1fbe", "\u03b9"},
		{"ŉ/ʼn", "\u0149", "\u02bcn"}, {"ǰ/J+caron", "\u01f0", "J\u030c"}, {"ẖ/h+macron below", "\u1e96", "h\u0331"},
		{"fullwidth Ａ/A", "\uff21", "A"}, {"ǅ/ǆ", "\u01c5", "\u01c6"}, {"ǆ/dž", "\u01c6", "d\u017e"},
		{"Vithkuqi (U14)", "\U00010570", "\U00010597"}, {"Garay (U16)", "\U00010d50", "\U00010d70"},
		{"Ɤ U+A7CB/ɤ (U16)", "\ua7cb", "\u0264"}, {"Ƛ U+A7DC/ƛ (U16)", "\ua7dc", "\u019b"},
		{"Deseret", "\U00010400", "\U00010428"}, {"Adlam", "\U0001e900", "\U0001e922"},
		{"Ⅰ/ⅰ", "\u2160", "\u2170"}, {"Ⅰ/I", "\u2160", "I"}, {"Ⓐ/ⓐ", "\u24b6", "\u24d0"},
		{"և/եւ", "\u0587", "\u0565\u0582"}, {"ϑ/θ", "\u03d1", "\u03b8"}, {"ϴ/θ", "\u03f4", "\u03b8"}, {"ẛ/ṡ", "\u1e9b", "\u1e61"},
		{"ΐ U+0390/U+1FD3", "\u0390", "\u1fd3"}, {"CJK compat 豈/豈", "\uf900", "\u8c48"}, {"U+2002/U+2000", "\u2002", "\u2000"},
		{"NBSP/space", "\u00a0", " "}, {"dot/U+2024", ".", "\u2024"}, {"colon", ":", "\uff1a"},
		{"Unicode 17 A7CE/A7CF", "\ua7ce", "\ua7cf"}, {"Unicode 17 A7D2/A7D3", "\ua7d2", "\ua7d3"}, {"Unicode 17 A7D4/A7D5", "\ua7d4", "\ua7d5"},
		{"Unicode 17 16EA0/16EBB", "\U00016ea0", "\U00016ebb"},
	}
	for i, c := range pairs {
		folder := filepath.Join(dir, fmt.Sprintf("p%03d", i))
		if err := os.Mkdir(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(filepath.Join(folder, "q"+c.a), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		f.Close()
		first, err1 := os.Stat(filepath.Join(folder, "q"+c.a))
		second, err2 := os.Stat(filepath.Join(folder, "q"+c.b))
		same := err1 == nil && err2 == nil && os.SameFile(first, second)
		g, err := os.OpenFile(filepath.Join(folder, "q"+c.b), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			g.Close()
		}
		if excl := errors.Is(err, os.ErrExist); excl != same {
			t.Fatalf("%s: stats as one file %v, but O_EXCL refused: %v", c.what, same, err)
		}
		joined := caseless("q"+c.a) == caseless("q"+c.b)
		t.Logf("%-26s volume one=%-5v key one=%v", c.what, same, joined)
		if joined != same {
			t.Errorf("%s (%+q, %+q): the volume takes them for one file: %v; the key: %v", c.what, c.a, c.b, same, joined)
		}
	}
}
