// Package appdirs holds the one place that names the folder the app keeps its
// state in, so desktop, internal/gm and internal/guard can't drift apart.
// (It imports nothing, so the command-line tools can use it without
// desktop's side effect of making the folder.)
package appdirs

// The folder's name on each system. They keep the names the app had before it
// was Smolcorp (the Mac app is still AgentDeck's), so paired phones stay paired.
const (
	Mac     = "AgentDeck"
	Linux   = "sauzzedeck-pro"
	Windows = "SauzzeDeck Pro"
)

// Name is the support folder's name for a runtime.GOOS value, inside the
// user's config folder (Application Support on a Mac).
func Name(goos string) string {
	switch goos {
	case "darwin":
		return Mac
	case "windows":
		return Windows
	}
	return Linux
}
