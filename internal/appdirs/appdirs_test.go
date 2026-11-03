package appdirs

import "testing"

func TestNameOnEachSystem(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin": "AgentDeck", "linux": "sauzzedeck-pro", "windows": "SauzzeDeck Pro", "freebsd": "sauzzedeck-pro",
	} {
		if got := Name(goos); got != want {
			t.Errorf("%s: got %q, want %q", goos, got, want)
		}
	}
}
