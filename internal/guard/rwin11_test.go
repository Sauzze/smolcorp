package guard

import (
	"strings"
	"testing"
)

// WIN125m (RWIN11 PART 1): a brace sequence in a value (a for list, an array, set --, an assignment) ran at
// WIN125h and was refused by RWIN10 B3's value reading as "not written plainly". It runs again, for every managed
// hire and the GM: the value reading takes {x..y} and {x..y..n} as a run that can't start a hidden name.
func TestRWIN11sBraceSequencesRun(t *testing.T) {
	runs := []string{
		"for i in {1..5}; do echo $i; done", "for i in {1..10}; do curl -s localhost:3000 >/dev/null; done",
		"for i in {01..12}; do echo $i; done", "for i in {a..e}; do echo $i; done", "for i in {0..100..10}; do echo $i; done",
		"for i in x{1..3}y; do echo $i; done", "x=({1..5})", "a=({1..3})", "set -- {1..3}", "x={1..3}",
		// The same forms, a little further from RWIN11's rows.
		"for i in {10..1}; do sleep $i; done", "for i in {-3..3}; do echo $i; done", "for i in {1..20..-5}; do echo $i; done",
		"for c in {A..Z}; do echo $c; done", "for d in {0..9}; do touch log$d.txt; done", "for i in {1..3} {a..c}; do echo $i; done",
		"for f in file{1..3}.txt; do touch $f; done", "for i in {1..3}{a..c}; do echo $i; done", "a=(img{001..060}.png)",
		"declare -a a=({1..9})", "local -a ports=({3000..3005})", "select n in {1..3}; do echo $n; break; done",
		"for i in $(seq 1 5); do echo $i; done", "for i in {1..5}; do git log -1 --skip=$i --oneline; done",
		// Too long to list as words (more than 64): read as a run too.
		"for i in {1..100}; do echo $i; done", "for i in {0..1000..10}; do echo $i; done", "set -- {1..100}",
		"for i in {a..z}{a..z}; do echo $i; done", "for i in {1..100}; do curl -s localhost:3000 >/dev/null; done", "x={1..100}",
		// For comparison, they ran before.
		"echo x{1..3}", "touch file{1..3}.txt", "for i in {a,b}; do echo $i; done", "for i in 1 2 3; do echo $i; done",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range runs {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// A brace form in a value that could fill in a name the guard keeps is still refused: a range that gives a letter
// of the name (.{a..z}laude), a sequence beside a kept name, and the braces that aren't a sequence the reading
// trusts ({+..0} fills in a . and a / in zsh; {a..b..2} and {1..3..0} are the characters . a b 2 and . 0 1 3 under
// zsh's BRACE_CCL; a number of more than ten digits; a quoted part).
func TestBraceFormsThatCouldReachAreStillRefused(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	refused := []string{
		"x=.{a..z}laude/agents; cd $x", "for d in .{a..z}laude; do rm $d/settings.json; done",
		"x=.c{a..z}aude/settings.json; rm $x", "a=(.cl{a..z}ude/settings.json); rm ${a[@]}", "set -- .{a..z}laude/agents; git -C $1 rm ../settings.json",
		"for f in {1..3}/.claude/settings.json; do rm $f; done", "x={1..3}/../.claude/agents; cd $x", "x=.{1..3}?laude; rm -r $x",
		"for i in {1..3}; do cd .claude; done", "x=setup-{1..2}key; cat $x", "x=" + dir + "setup-{1..2}; cat $x",
		"x=({+..0}claude); rm -r $x", "for i in {a..b..2}claude; do rm -r $i; done", "for i in {1..3..0}claude; do rm -r $i; done",
		"for i in {1..3..00}claude; do rm -r $i; done", "for i in {1..99999999999}claude; do rm -r $i; done",
		"for i in {1..'3'}claude; do rm -r $i; done", "for i in {.c,x}laude; do rm -r $i; done", "x={1..3}{.c,x}laude; rm -r $x",
		"read x <<< .{a..z}laude/agents; cd $x",
		// Too long to list as words.
		"rm .{a..z}laude/{a..z}{a..z}ttings.json", "for d in .{a..z}{a..z}laude; do rm -r $d; done", "set -- .{a..z}{1..9}laude; rm -r $1",
		"x=.c{a..z}{a..z}aude/settings.json; rm $x", "rm {1..100}{.c,x}laude/settings.json",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range refused {
			if Check(line, groups) == nil {
				t.Errorf("let through %q", line)
			}
		}
	}
}

// Every brace sequence the reading trusts, spliced at every place of the names the guard keeps in every value form,
// is still refused: the run it reads may be empty, so the name is still filled in.
func TestBraceSequenceSpliceIsRefused(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	seqs := []string{"{1..3}", "{a..e}", "{0..100..10}", "{01..12}", "{A..z}", "{-1..1}"}
	forms := []struct{ before, after string }{
		{"x=", "; rm $x"}, {"for x in ", "; do rm $x; done"}, {"a=(", "); rm ${a[@]}"}, {"set -- ", "; rm $1"},
		{"read x <<< ", "; rm $x"},
	}
	names := []string{".claude/settings.json", ".claude/settings.local.json", dir + "setup-key", dir + "hire-guard.json"}
	n := 0
	for _, seq := range seqs {
		for _, f := range forms {
			for _, name := range names {
				from := strings.LastIndexByte(name, '/') + 1
				if strings.HasPrefix(name, ".claude") {
					from = 1
				}
				for i := from; i < len(name); i++ {
					line := f.before + name[:i] + seq + name[i:] + f.after
					n++
					for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
						if Check(line, groups) == nil {
							t.Errorf("let through: %s", line)
						}
					}
				}
			}
		}
	}
	if n < 1000 {
		t.Errorf("only %d lines", n)
	}
}

func TestBraceSequence(t *testing.T) {
	for body, want := range map[string]bool{
		"1..5": true, "01..12": true, "a..e": true, "Z..a": true, "0..z": true, "0..100..10": true, "-3..3": true,
		"10..1..-2": true, "1234567890..1": true,
		"+..0": false, ".c,x": false, "a..b..2": false, "1..3..0": false, "1..3..-0": false, "1..3..00": false,
		"12345678901..1": false, "1..": false, "..": false, "a..ab": false, "+1..3": false, "1..3..+1": false,
		"1..2..3..4": false, "": false, "1...3": false,
	} {
		if got := braceSequence(body); got != want {
			t.Errorf("braceSequence(%q) = %v, want %v", body, got, want)
		}
	}
}
