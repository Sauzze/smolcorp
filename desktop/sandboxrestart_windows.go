//go:build windows

package desktop

import "net/http"

// restartInSandbox: no desk starts in a sandbox on Windows in 1.26, so there's none to restart one in (SA-X14).
func (d *Deck) restartInSandbox(string) (int, any) { return http.StatusConflict, noSandboxToRestart }

// onOwnTmux: no desk runs on a tmux server of its own here.
func onOwnTmux(Hire) bool { return false }
