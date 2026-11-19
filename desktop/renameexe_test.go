package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The Mac app's program is Smolcorp since 1.26 (it was AgentDeck), and its bundle identifier stays
// io.github.sauzze.agentdeck, so macOS still knows it as the same app: its Keychain items, Accessibility, Automation
// and the microphone stay granted. Info.plist names the program, build-app.sh puts the Swift product there under that
// name, and nothing that ships still looks for Contents/MacOS/AgentDeck.
func TestTheMacAppsProgramIsSmolcorpWhereverItIsNamed(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	plistValue := func(plist, key string) string {
		m := regexp.MustCompile(`<key>` + key + `</key>\s*<string>([^<]*)</string>`).FindStringSubmatch(plist)
		if m == nil {
			return ""
		}
		return m[1]
	}
	plist := read("Resources/Info.plist")
	if got := plistValue(plist, "CFBundleExecutable"); got != "Smolcorp" {
		t.Errorf("Info.plist's CFBundleExecutable is %q, want Smolcorp", got)
	}
	if got := plistValue(plist, "CFBundleIdentifier"); got != macBundleID {
		t.Errorf("Info.plist's CFBundleIdentifier is %q, want %s (unchanged, as the grants are tied to it)", got, macBundleID)
	}
	if macBundleID != "io.github.sauzze.agentdeck" {
		t.Errorf("the app's bundle identifier is %q, want io.github.sauzze.agentdeck (unchanged)", macBundleID)
	}

	// The Swift product keeps its target's name; the app's copy of it is named as Info.plist says.
	if !strings.Contains(read("Package.swift"), `.executableTarget(name: "AgentDeck", path: "Sources/AgentDeck")`) {
		t.Error(`Package.swift's executable target is no longer "AgentDeck": build-app.sh's SWIFT_PRODUCT must follow it`)
	}
	build := read("scripts/build-app.sh")
	for _, want := range []string{
		"\nSWIFT_PRODUCT=\"AgentDeck\"\n", "\nEXECUTABLE=\"Smolcorp\"\n",
		`--show-bin-path)/$SWIFT_PRODUCT"`, `cp "$BIN" "$APP/Contents/MacOS/$EXECUTABLE"`,
		`strip -S -no_code_signature_warning "$APP/Contents/MacOS/$EXECUTABLE"`,
		`--identifier io.github.sauzze.agentdeck --entitlements "$ENTITLEMENTS" "$APP"`,
	} {
		if !strings.Contains(build, want) {
			t.Errorf("scripts/build-app.sh has no %q", want)
		}
	}

	// What finds the program by its path follows it. The test copies' scripts match any program in the copy's
	// Contents/MacOS, so they need no name.
	for name, want := range map[string]string{
		".github/workflows/ci.yml": `lipo -archs "$APP/Contents/MacOS/Smolcorp"`,
		"scripts/release.sh":       `$(lipo -archs "$APP/Contents/MacOS/Smolcorp")`,
		"scripts/demo-page-run.sh": `"$APP/Contents/MacOS/Smolcorp"*)`,
		"scripts/test-copy.sh":     `*"Smolcorp Test.app/Contents/MacOS/"*) kill "$pid" ;;`,
		"packaging/FIRST-SETUP.md": "`/Applications/Smolcorp.app/Contents/MacOS/Smolcorp --setup`",
		"docs/install.md":          "`/Applications/Smolcorp.app/Contents/MacOS/Smolcorp --setup`",
	} {
		if !strings.Contains(read(name), want) {
			t.Errorf("%s has no %q", name, want)
		}
	}

	// Nothing that ships names the old program's path (the release notes keep 1.25's words, as history).
	var files []string
	for _, dir := range []string{"scripts", "docs", "packaging", "Resources", "Sources", "cmd", "internal", "desktop", ".github"} {
		err := filepath.WalkDir(filepath.Join("..", dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") || !regexp.MustCompile(`\.(go|swift|sh|md|yml|plist|html|js|mjs)$`).MatchString(path) {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	files = append(files, "../install.sh", "../README.md")
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "MacOS/AgentDeck") {
			t.Errorf("%s still names Contents/MacOS/AgentDeck, the program's name before 1.26", path)
		}
	}
}

// install.sh quits a running copy before it replaces it, and finds one under either of its program's names:
// AgentDeck, a copy from before 1.26, and Smolcorp since. Run on its own, with stand-ins for pgrep, osascript
// and sleep: no real process is looked for, asked or quit.
func TestInstallQuitsARunningCopyUnderEitherName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is the Mac's, run by bash")
	}
	data, err := os.ReadFile("../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	// From running() to the end of the block that quits the copy: the first fi at the line's start after it.
	script := string(data)
	start := strings.Index(script, "running() {")
	if start < 0 {
		t.Fatal("install.sh has no running()")
	}
	end := strings.Index(script[start:], "\nfi\n")
	if end < 0 {
		t.Fatal("install.sh's quitting block has no end")
	}
	block := script[start : start+end+len("\nfi\n")]
	if !strings.Contains(block, "pgrep -x Smolcorp") || !strings.Contains(block, "pgrep -x AgentDeck") {
		t.Fatalf("install.sh's running() doesn't look for both names:\n%s", block)
	}

	for _, row := range []struct {
		name     string
		running  []string // the process names the stand-in pgrep sees
		quits    bool     // whether osascript's quit ends them
		status   int
		asked    bool // whether it asked the app to quit
		wantSaid string
	}{
		{"nothing running", nil, true, 0, false, "PROCEEDED"},
		{"only the engine, smolcorp, and an app of someone else's", []string{"smolcorp", "Smolcorp Helper"}, true, 0, false, "PROCEEDED"},
		{"a copy from before 1.26, AgentDeck, that quits", []string{"AgentDeck"}, true, 0, true, "PROCEEDED"},
		{"a copy since 1.26, Smolcorp, that quits", []string{"Smolcorp"}, true, 0, true, "PROCEEDED"},
		{"a copy from before 1.26 that doesn't quit", []string{"AgentDeck"}, false, 1, true, "Smolcorp is still running."},
		{"a copy since 1.26 that doesn't quit", []string{"Smolcorp"}, false, 1, true, "Smolcorp is still running."},
	} {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			state := filepath.Join(dir, "running")
			log := filepath.Join(dir, "log")
			if err := os.Mkdir(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			names := strings.Join(row.running, "\n")
			if names != "" {
				names += "\n"
			}
			if err := os.WriteFile(state, []byte(names), 0o644); err != nil {
				t.Fatal(err)
			}
			// pgrep -x NAME: found only when NAME is, whole and in the same case, one of the running names.
			standIns := map[string]string{
				"pgrep": "#!/bin/sh\necho \"pgrep $*\" >> '" + log + "'\n[ \"$1\" = -x ] && [ $# -eq 2 ] || exit 3\n" +
					"grep -qxF -- \"$2\" '" + state + "'\n",
				"sleep": "#!/bin/sh\nexit 0\n",
			}
			standIns["osascript"] = "#!/bin/sh\necho \"osascript $*\" >> '" + log + "'\n"
			if row.quits {
				standIns["osascript"] += ": > '" + state + "'\n"
			}
			for name, text := range standIns {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(text), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("/bin/bash", "-c", "set -euo pipefail\nBUNDLE_ID=io.github.sauzze.agentdeck\n"+block+"echo PROCEEDED\n")
			cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + dir, "LANG=C"}
			out, err := cmd.CombinedOutput()
			status := 0
			if exit, ok := err.(*exec.ExitError); ok {
				status = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if status != row.status || !strings.Contains(string(out), row.wantSaid) {
				t.Errorf("status %d, said %q; want %d and %q", status, out, row.status, row.wantSaid)
			}
			logged, _ := os.ReadFile(log)
			if asked := strings.Contains(string(logged), `osascript -e quit app id "io.github.sauzze.agentdeck"`); asked != row.asked {
				t.Errorf("asked the app to quit: %v, want %v; calls:\n%s", asked, row.asked, logged)
			}
			if strings.Contains(string(logged), "pgrep") && !strings.Contains(string(logged), "pgrep -x Smolcorp") {
				t.Errorf("never looked for Smolcorp; calls:\n%s", logged)
			}
			if !row.asked && !strings.Contains(string(logged), "pgrep -x AgentDeck") {
				t.Errorf("didn't look for AgentDeck when no Smolcorp ran; calls:\n%s", logged)
			}
		})
	}
}
