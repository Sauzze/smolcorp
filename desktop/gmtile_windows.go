package desktop

// Windows has no Mac app, so no tiling (gmscreens.go): these are never reached there.
func hireClientTTY(string, int) string { return "" }

var showTeamGrid = func(hostBridge, []string, string, int) bool { return false }
