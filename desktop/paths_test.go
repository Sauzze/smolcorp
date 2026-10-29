package desktop

import "testing"

func TestThePortIsSevenSevenOneZeroUnlessAskedOtherwise(t *testing.T) {
	for text, want := range map[string]int{"": 7710, "abc": 7710, "80": 7710, "70000": 7710, "4711": 4711} {
		if got := portFromEnvironment(text); got != want {
			t.Errorf("%q: %d, want %d", text, got, want)
		}
	}
}
