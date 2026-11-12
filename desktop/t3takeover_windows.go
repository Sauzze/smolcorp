//go:build windows

package desktop

import (
	"errors"
	"net/http"
)

// Take over reaches a hire in a tmux session of its own, which Windows has none of: a hire here runs in a
// window of the launcher, and t3TakeOverRefusal says so before anything is stopped. These only answer the same.

func (d *Deck) t3StopInTmux(h Hire) error {
	return t3Refused(http.StatusConflict, calledHire(h)+" runs in a window of Smolcorp's launcher: Take over reaches only a hire in a tmux session of its own, for now.")
}

func (d *Deck) t3ResumeInTmux(Hire, string, []claudeProfile) (string, error) {
	return "", errors.New("a hire here runs in a window of Smolcorp's launcher, which can't resume a session yet.")
}
