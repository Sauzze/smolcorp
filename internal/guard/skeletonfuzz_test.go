package guard

import "testing"

// WIN125j: RWIN8's differential fuzz globbed 56,711 patterns setup-k<X>y and setup-<X>ey for real, in zsh 5.9, bash
// 3.2 and bash 5.3, in a folder holding setup-key. These 209 are the ones a shell filled in as the setup key and
// WIN125i's guard let through (zsh's first, then bash 3.2's; bash 5.3's were among them). Each is refused now, for
// a hire and the GM, and the demo's fence reads each as the key (PatternCouldFill). Every one of the 4,371 hits
// passes the same way (WIN125j's report).
var rwin8FuzzMisses = []string{
	`setup-[!!-[:alpha:]ey`, `setup-[!!-[:x:]ey`, `setup-[!--[:alpha:]ey`, `setup-[!--[:x:]ey`,
	`setup-[!.-[:alpha:]ey`, `setup-[!.-[:x:]ey`, `setup-[!:-[:alpha:]ey`, `setup-[!:-[:x:]ey`,
	`setup-[!:=[:alpha:]ey`, `setup-[!:=[:x:]ey`, `setup-[!=-[:alpha:]ey`, `setup-[!=-[:x:]ey`,
	`setup-[![-[:alpha:]ey`, `setup-[![-[:x:]ey`, `setup-[!e-[:alpha:]ey`, `setup-[!e-[:x:]ey`,
	`setup-[!x-[:alpha:]ey`, `setup-[!x-[:x:]ey`, `setup-[[:alpha:]!-[:alpha:]ey`, `setup-[[:alpha:]!-[:x:]ey`,
	`setup-[[:alpha:]--[:alpha:]ey`, `setup-[[:alpha:]--[:x:]ey`, `setup-[[:alpha:].-[:alpha:]ey`,
	`setup-[[:alpha:].-[:x:]ey`, `setup-[[:alpha:]:-[:alpha:]ey`, `setup-[[:alpha:]:-[:x:]ey`,
	`setup-[[:alpha:]:=[:alpha:]ey`, `setup-[[:alpha:]:=[:x:]ey`, `setup-[[:alpha:]=-[:alpha:]ey`,
	`setup-[[:alpha:]=-[:x:]ey`, `setup-[[:alpha:][-[:alpha:]ey`, `setup-[[:alpha:][-[:x:]ey`,
	`setup-[[:alpha:]e-[:alpha:]ey`, `setup-[[:alpha:]e-[:x:]ey`, `setup-[[:alpha:]x-[:alpha:]ey`,
	`setup-[[:alpha:]x-[:x:]ey`, `setup-k[!!-[:alpha:]y`, `setup-k[!!-[:x:]y`, `setup-k[!--[:alpha:]y`,
	`setup-k[!--[:x:]y`, `setup-k[!.-[:alpha:]y`, `setup-k[!.-[:x:]y`, `setup-k[!:-[:alpha:]y`, `setup-k[!:-[:x:]y`,
	`setup-k[!:=[:alpha:]y`, `setup-k[!:=[:x:]y`, `setup-k[!=-[:alpha:]y`, `setup-k[!=-[:x:]y`,
	`setup-k[![-[:alpha:]y`, `setup-k[![-[:x:]y`, `setup-k[!]y`, `setup-k[!e-[:alpha:]y`, `setup-k[!e-[:x:]y`,
	`setup-k[!x-[:alpha:]y`, `setup-k[!x-[:x:]y`, `setup-k[[:alpha:]!-[:alpha:]y`, `setup-k[[:alpha:]!-[:x:]y`,
	`setup-k[[:alpha:]--[:alpha:]y`, `setup-k[[:alpha:]--[:x:]y`, `setup-k[[:alpha:].-[:alpha:]y`,
	`setup-k[[:alpha:].-[:x:]y`, `setup-k[[:alpha:]:-[:alpha:]y`, `setup-k[[:alpha:]:-[:x:]y`,
	`setup-k[[:alpha:]:=[:alpha:]y`, `setup-k[[:alpha:]:=[:x:]y`, `setup-k[[:alpha:]=-[:alpha:]y`,
	`setup-k[[:alpha:]=-[:x:]y`, `setup-k[[:alpha:][-[:alpha:]y`, `setup-k[[:alpha:][-[:x:]y`,
	`setup-k[[:alpha:]e-[:alpha:]y`, `setup-k[[:alpha:]e-[:x:]y`, `setup-k[[:alpha:]x-[:alpha:]y`,
	`setup-k[[:alpha:]x-[:x:]y`, `setup-k[e!-[:alpha:]y`, `setup-k[e!-[:x:]y`, `setup-k[e.-[:alpha:]y`,
	`setup-k[e.-[:x:]y`, `setup-k[e:-[:alpha:]y`, `setup-k[e:-[:x:]y`, `setup-k[e:=[:alpha:]y`, `setup-k[e:=[:x:]y`,
	`setup-k[e=-[:alpha:]y`, `setup-k[e=-[:x:]y`, `setup-k[e[-[:alpha:]y`, `setup-k[e[-[:x:]y`,
	`setup-k[ee-[:alpha:]y`, `setup-k[ee-[:x:]y`, `setup-k[ex-[:alpha:]y`, `setup-k[ex-[:x:]y`, `setup-[!!![:x:]ey`,
	`setup-[!!.[:x:]ey`, `setup-[!!:[:x:]ey`, `setup-[!!=[:x:]ey`, `setup-[!![[:x:]ey`, `setup-[!!e[:x:]ey`,
	`setup-[!!x[:x:]ey`, `setup-[!-![:x:]ey`, `setup-[!-.[:x:]ey`, `setup-[!-:[:x:]ey`, `setup-[!-=[:x:]ey`,
	`setup-[!-[[:x:]ey`, `setup-[!-e[:x:]ey`, `setup-[!-x[:x:]ey`, `setup-[!.![:x:]ey`, `setup-[!..[:x:]ey`,
	`setup-[!.:[:x:]ey`, `setup-[!.=[:x:]ey`, `setup-[!.[[:x:]ey`, `setup-[!.e[:x:]ey`, `setup-[!.x[:x:]ey`,
	`setup-[!:![:x:]ey`, `setup-[!:.[:x:]ey`, `setup-[!::[:x:]ey`, `setup-[!:[[:x:]ey`, `setup-[!:e[:x:]ey`,
	`setup-[!:x[:x:]ey`, `setup-[!=![:x:]ey`, `setup-[!=.[:x:]ey`, `setup-[!=:[:x:]ey`, `setup-[!==[:x:]ey`,
	`setup-[!=[[:x:]ey`, `setup-[!=e[:x:]ey`, `setup-[!=x[:x:]ey`, `setup-[![![:x:]ey`, `setup-[![:[:x:]ey`,
	`setup-[![=[:x:]ey`, `setup-[![[[:x:]ey`, `setup-[![e[:x:]ey`, `setup-[![x[:x:]ey`, `setup-[!]![:x:]ey`,
	`setup-[!]-[:alpha:]ey`, `setup-[!]-[:x:]ey`, `setup-[!].[:x:]ey`, `setup-[!]:[:x:]ey`, `setup-[!]=[:x:]ey`,
	`setup-[!][[:x:]ey`, `setup-[!]e[:x:]ey`, `setup-[!]x[:x:]ey`, `setup-[!e![:x:]ey`, `setup-[!e.[:x:]ey`,
	`setup-[!e:[:x:]ey`, `setup-[!e=[:x:]ey`, `setup-[!e[[:x:]ey`, `setup-[!ee[:x:]ey`, `setup-[!ex[:x:]ey`,
	`setup-[!x![:x:]ey`, `setup-[!x.[:x:]ey`, `setup-[!x:[:x:]ey`, `setup-[!x=[:x:]ey`, `setup-[!x[[:x:]ey`,
	`setup-[!xe[:x:]ey`, `setup-[!xx[:x:]ey`, `setup-k[!!![:x:]y`, `setup-k[!!.[:x:]y`, `setup-k[!!:[:x:]y`,
	`setup-k[!!=[:x:]y`, `setup-k[!![:x:]y`, `setup-k[!![[:x:]y`, `setup-k[!!x[:x:]y`, `setup-k[!-![:x:]y`,
	`setup-k[!-.[:x:]y`, `setup-k[!-:[:x:]y`, `setup-k[!-=[:x:]y`, `setup-k[!-[:x:]y`, `setup-k[!-[[:x:]y`,
	`setup-k[!-x[:x:]y`, `setup-k[!.![:x:]y`, `setup-k[!..[:x:]y`, `setup-k[!.:[:x:]y`, `setup-k[!.=[:x:]y`,
	`setup-k[!.[:x:]y`, `setup-k[!.[[:x:]y`, `setup-k[!.x[:x:]y`, `setup-k[!:![:x:]y`, `setup-k[!:.[:x:]y`,
	`setup-k[!::[:x:]y`, `setup-k[!:[:x:]y`, `setup-k[!:[[:x:]y`, `setup-k[!:x[:x:]y`, `setup-k[!=![:x:]y`,
	`setup-k[!=.[:x:]y`, `setup-k[!=:[:x:]y`, `setup-k[!==[:x:]y`, `setup-k[!=[:x:]y`, `setup-k[!=[[:x:]y`,
	`setup-k[!=x[:x:]y`, `setup-k[![![:x:]y`, `setup-k[![:[:x:]y`, `setup-k[![:x:]y`, `setup-k[![=[:x:]y`,
	`setup-k[![[:x:]y`, `setup-k[![[[:x:]y`, `setup-k[![x[:x:]y`, `setup-k[!]![:x:]y`, `setup-k[!]-[:alpha:]y`,
	`setup-k[!]-[:x:]y`, `setup-k[!].[:x:]y`, `setup-k[!]:[:x:]y`, `setup-k[!]=[:x:]y`, `setup-k[!][:x:]y`,
	`setup-k[!][[:x:]y`, `setup-k[!]x[:x:]y`, `setup-k[!x![:x:]y`, `setup-k[!x.[:x:]y`, `setup-k[!x:[:x:]y`,
	`setup-k[!x=[:x:]y`, `setup-k[!x[:x:]y`, `setup-k[!x[[:x:]y`, `setup-k[!xx[:x:]y`,
}

func TestRWIN8sFuzzMissesAreRefused(t *testing.T) {
	for _, pattern := range rwin8FuzzMisses {
		for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
			if Check("cat "+pattern, groups) == nil {
				t.Errorf("let through: cat %s", pattern)
			}
		}
		if !PatternCouldFill(pattern, "setup-key", false) {
			t.Errorf("PatternCouldFill(%q): false", pattern)
		}
	}
}
