//go:build !windows

package desktop

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/aymanbagabas/go-pty"
)

// followSize passes each change in the terminal's size on to the agent's.
func followSize(terminal pty.Pty, screen *launcherScreen) {
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	for range changes {
		matchSize(terminal, screen)
	}
}

// consoleCodePage: a Unix terminal has no code page the launcher can read,
// and whether it draws Ambiguous characters wide is its own setting, so the
// locale stands in for one (localeCodePage).
func consoleCodePage() screenCodePage { return localeCodePage(os.Getenv) }

// prepareConsole: a Unix terminal already reads escape sequences.
func prepareConsole() {}

// useCmdLine: only Windows starts agents through cmd.exe.
func useCmdLine(*pty.Cmd, string, []string) {}
