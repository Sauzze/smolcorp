//go:build windows

package desktop

import (
	"os"
	"syscall"
	"time"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// How often to look at the console's size: Windows sends no signal when a
// console is resized.
const sizePoll = 300 * time.Millisecond

// followSize passes each change in the console's size on to the agent's,
// and reads the console's code page again at each look, for the screen.
func followSize(terminal pty.Pty, screen *launcherScreen) {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	for range time.Tick(sizePoll) {
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil && (w != width || h != height) {
			width, height = w, h
			screen.Resize(h, w)
			_ = terminal.Resize(w, h)
		}
		screen.SetCodePage(consoleCodePage())
	}
}

// consoleCodePage is the console's output code page and whether the user's
// locale is a CJK one (screenCodePage). A code page that can't be read is 0,
// which the screen takes as a CJK one.
func consoleCodePage() screenCodePage {
	output, err := windows.GetConsoleOutputCP()
	if err != nil {
		return screenCodePage{}
	}
	return screenCodePage{output: output, locale: userLocale()}
}

var getUserDefaultLCID = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLCID")

// userLocale says whether the system's ANSI code page (GetACP) or the user's
// locale (GetUserDefaultLCID: its primary language, Chinese 0x04, Japanese
// 0x11 or Korean 0x12) is a CJK one; unknown if the locale can't be read.
func userLocale() screenLocale {
	switch windows.GetACP() {
	case 932, 936, 949, 950:
		return localeCJK
	}
	if getUserDefaultLCID.Find() != nil {
		return localeUnknown
	}
	lcid, _, _ := getUserDefaultLCID.Call()
	switch lcid & 0x3ff {
	case 0:
		return localeUnknown
	case 0x04, 0x11, 0x12:
		return localeCJK
	}
	return localeNotCJK
}

// prepareConsole lets the console show the agent's colours and cursor moves,
// which arrive as escape sequences.
func prepareConsole() {
	console := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if windows.GetConsoleMode(console, &mode) == nil {
		_ = windows.SetConsoleMode(console, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.DISABLE_NEWLINE_AUTO_RETURN)
	}
}

// useCmdLine gives cmd.exe, starting a .cmd script, a command line it reads
// as meant: see cmdLine.
func useCmdLine(cmd *pty.Cmd, program string, args []string) {
	if len(args) >= 2 && args[0] == "/c" {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine(program, args[1], args[2:])}
	}
}
