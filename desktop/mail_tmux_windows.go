//go:build windows

package desktop

import "time"

// A hire on Windows has no tmux: mail is typed through the hire's launcher (beta), see mail_launcher.go.
func (d *Deck) mailTypeIntoHire(h Hire, text string, commit func() bool) mailResult {
	return d.mailTypeIntoLauncher(h, text, commit, time.Sleep)
}
