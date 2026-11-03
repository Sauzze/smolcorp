//go:build !windows

package desktop

import "strings"

// shellPath is a path as a hook command names it: in single quotes, for the
// shell the agent runs it through, where nothing in a path is special but the
// quote itself, written as quote, backslash, quote, quote. Earlier versions wrote it in double quotes,
// which a ", $, ` or \ in the path broke out of: isOurs still reads those.
func shellPath(path string) string {
	return `'` + strings.ReplaceAll(path, `'`, `'\''`) + `'`
}
