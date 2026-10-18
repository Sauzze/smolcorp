package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The OpenCode installed, for the plugin it can load, as the Mac app's
// OpenCodeInstall: OpenCode 1 loads 1.7.2's named export, OpenCode 2 only a
// default with an id and setup(). Where it is and when it last changed say
// cheaply whether it might have been updated; its version is asked only then.

// How often to look whether OpenCode was updated, as it updates itself.
const openCodeLookInterval = time.Minute

var (
	// openCodeMu is held to write, remove or look at the plugin, by
	// connecting, disconnecting and the look once a minute alike, so none
	// undoes another. openCodeWrittenFor is the OpenCode the plugin was last
	// written for, by where it is and when it last changed.
	openCodeMu         sync.Mutex
	openCodeWrittenFor string
	openCodeVersion    = regexp.MustCompile(`(\d+)\.\d`)
	// The PATH your login shell sets, asked once.
	loginPath     []string
	loginPathOnce sync.Once
)

// openCodeProgram is the opencode program, with every link resolved, or ""
// if there's none: on the PATH, the one your login shell sets, or where its
// own installer, npm, pnpm, Bun or a Node version manager put it.
func openCodeProgram() string {
	names := []string{"opencode"}
	if runtime.GOOS == "windows" {
		names = []string{"opencode.exe", "opencode.cmd"}
	}
	for _, folder := range append(filepath.SplitList(os.Getenv("PATH")), openCodePlaces()...) {
		for _, name := range names {
			candidate := filepath.Join(folder, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
					return resolved
				}
				return candidate
			}
		}
	}
	return ""
}

// openCodePlaces are where OpenCode may be besides this program's PATH.
func openCodePlaces() []string {
	home := func(parts ...string) string { return filepath.Join(append([]string{homeDir}, parts...)...) }
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(os.Getenv("APPDATA"), "npm"), filepath.Join(os.Getenv("LOCALAPPDATA"), "pnpm"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Volta", "bin"), home(".opencode", "bin"), home(".bun", "bin")}
	}
	loginPathOnce.Do(func() { loginPath = loginShellPath() })
	nvm, _ := filepath.Glob(home(".nvm", "versions", "node", "*", "bin"))
	slices.Reverse(nvm) // the newest first
	return slices.Concat(loginPath, []string{home(".opencode", "bin"), home(".local", "bin"), home(".bun", "bin"), home(".npm-global", "bin"),
		home(".local", "share", "pnpm"), home(".volta", "bin"), home(".local", "share", "fnm", "aliases", "default", "bin"),
		home(".asdf", "shims"), home(".local", "share", "mise", "shims"), "/opt/homebrew/bin", "/usr/local/bin"}, nvm)
}

// loginShellPath is the PATH your login shell sets, which a program started
// from the desktop may not have: empty if it doesn't say within a few seconds.
func loginShellPath() []string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A marker before it, as a shell's start-up files can print things of their own
	out, err := exec.CommandContext(ctx, shell, "-lic", `printf '\n<path>%s' "$PATH"`).Output()
	if err != nil && len(out) == 0 {
		return nil
	}
	i := strings.LastIndex(string(out), "<path>")
	if i < 0 {
		return nil
	}
	return filepath.SplitList(strings.TrimSpace(string(out)[i+len("<path>"):]))
}

// openCodeStamp is where the program is and when it last changed, or "".
func openCodeStamp() string {
	program := openCodeProgram()
	if program == "" {
		return ""
	}
	info, err := os.Stat(program)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s %d", program, info.ModTime().UnixNano())
}

// openCodeMajor is OpenCode's major version, as `opencode --version` says
// it, and false if there's no OpenCode or it doesn't say within a few seconds.
func openCodeMajor() (int, bool) {
	program := openCodeProgram()
	if program == "" {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := versionCommand(ctx, program).Output()
	if err != nil {
		return 0, false
	}
	return majorVersionIn(string(out))
}

// majorVersionIn is the major version in what --version prints, as
// "opencode v2.0.32" or "1.14.0".
func majorVersionIn(text string) (int, bool) {
	found := openCodeVersion.FindStringSubmatch(text)
	if found == nil {
		return 0, false
	}
	major, err := strconv.Atoi(found[1])
	return major, err == nil
}

// openCodePluginTemplate is the plugin for the OpenCode installed: OpenCode
// 1's own for 1, and the default for 2 or one whose version can't be told.
func openCodePluginTemplate() string {
	if major, ok := openCodeMajor(); ok && major == 1 {
		return "Resources/opencode-plugin-v1.js"
	}
	return "Resources/opencode-plugin.js"
}

// lookAtOpenCode writes the plugin again if OpenCode has changed since it
// was last written, for the version it is now, while it's still connected.
func lookAtOpenCode() {
	openCodeMu.Lock()
	defer openCodeMu.Unlock()
	if isConnected(OpenCode) && openCodeStamp() != openCodeWrittenFor {
		_ = writePluginLocked()
	}
}

// forgetOpenCode removes the plugin, as disconnecting does, and what it was
// written for, so connecting again writes it afresh.
func forgetOpenCode() {
	openCodeMu.Lock()
	defer openCodeMu.Unlock()
	_ = os.Remove(openCodePlugin())
	_ = os.Remove(oldOpenCodePlugin())
	openCodeWrittenFor = ""
}

// watchOpenCode looks about once a minute whether OpenCode was updated.
func watchOpenCode() {
	go func() {
		for range time.Tick(openCodeLookInterval) {
			lookAtOpenCode()
		}
	}()
}
