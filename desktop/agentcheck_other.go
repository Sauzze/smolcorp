//go:build !darwin

package desktop

import (
	"os"
	"strconv"
)

// Off a Mac, Claude Code isn't signed in a way this computer can check (SANDBOX-PLAN §3.4): nothing is refused.
func claudeSignatureRefusal(string) error { return nil }

// stampIdentity is the part of a file's stamp the system gives everywhere: its size.
func stampIdentity(info os.FileInfo) string { return strconv.FormatInt(info.Size(), 10) }

// appSealIntact: there is no app seal off a Mac.
func appSealIntact(string, string) (string, bool) { return "", true }

// claudeSignaturesChecked: nothing is checked off a Mac.
const claudeSignaturesChecked = false
