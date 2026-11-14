package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// modeIs says a file has the Unix mode want. Windows has no Unix modes: Go reports any writable
// file as 0666 there, and what keeps a file yours is your profile folder's own permissions,
// so there any mode is taken as right (as saved_test.go does).
func modeIs(info os.FileInfo, want os.FileMode) bool {
	return runtime.GOOS == "windows" || info.Mode().Perm() == want
}

// isolate points every file the package reads or writes, and the home folder
// agents keep their settings in, at fresh folders, so a test never touches
// the real paired phones or an agent's real settings. It returns the home.
func isolate(t *testing.T) string {
	t.Helper()
	saved := []string{supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir, windowKeyFile, sessionsFile}
	t.Cleanup(func() {
		supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir, windowKeyFile = saved[0], saved[1], saved[2], saved[3], saved[4], saved[5], saved[6]
		sessionsFile = saved[7]
	})
	supportDir = t.TempDir()
	// The photos folder can't be listed (photo.go): listed again so the test's folder can be removed.
	support := supportDir
	t.Cleanup(func() { _ = unlistPhotos(filepath.Join(support, photosFolder), false) })
	phonesFile = filepath.Join(supportDir, "phones.json")
	configFile = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	windowKeyFile = filepath.Join(supportDir, "window-key")
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	homeDir = t.TempDir()
	// No record key from a test before (records.go): a deck started here takes its own from its stand-in Mac, or
	// from a stand-in Keychain of this test's own in front of one that has none.
	noRecordKey()
	keychain, via := &macStandIn{}, recordsVia
	recordsVia = func(h hostBridge) hostBridge {
		if _, own := h.(*macStandIn); own {
			return h
		}
		return recordsBeside{h, keychain}
	}
	t.Cleanup(func() { noRecordKey(); recordsVia = via })
	return homeDir
}

// writeRegistryFile writes a registry file as one found at the core's next start: the core reads the registry once
// and holds it (RT126SK1 B4), so what a test writes stands for a file written while it was stopped.
func writeRegistryFile(path string, data []byte, perm os.FileMode) error {
	err := os.WriteFile(path, data, perm)
	forgetHeldRegistries()
	return err
}

// recordsBeside is a stand-in Mac app that has no record key of its own, with a stand-in Keychain answering for it.
type recordsBeside struct {
	app      hostBridge
	keychain *macStandIn
}

func (r recordsBeside) call(request bridgeRequest) (bridgeAnswer, error) {
	if strings.HasPrefix(request.Action, "record-key-") {
		return r.keychain.call(request)
	}
	return r.app.call(request)
}

// noRecordKey puts the records' state back to a core's without the Mac app: no key, nothing held, nothing said.
func noRecordKey() {
	records.mu.Lock()
	records.host, records.key, records.id, records.prev, records.state = nil, nil, "", "", recordsOff
	records.first, records.rotated, records.stale, records.writers = false, false, false, nil
	records.settled, records.owed = false, false
	records.mu.Unlock()
	forgetHeldRegistries()
	recordsSaid.Lock()
	recordsSaid.said = map[string]bool{}
	recordsSaid.Unlock()
}

// scriptSettings are the settings a start script gives its Claude Code in its command line (--settings=…, one
// shell-quoted word), "" if it gives none.
func scriptSettings(script string) string {
	i := strings.Index(script, "'--settings=")
	if i < 0 {
		return ""
	}
	var word strings.Builder
	rest := script[i+len("'--settings="):]
	for {
		end := strings.IndexByte(rest, '\'')
		if end < 0 {
			return ""
		}
		word.WriteString(rest[:end])
		if !strings.HasPrefix(rest[end:], `'\''`) {
			return word.String()
		}
		word.WriteByte('\'')
		rest = rest[end+len(`'\''`):]
	}
}

// planText is a desk's start plan as the tests read it: one change a line (unset NAME, export NAME='value', the
// PATH's first folder, the API key read from the secret store), then the command, each word shell-quoted. It is
// never run, nor written anywhere: a desk's start reaches its pane only through its ticket (deskstart.go).
func planText(p deskPlan) string {
	var text strings.Builder
	for _, change := range p.Changes {
		if change.Unset {
			text.WriteString("unset " + change.Name + "\n")
		} else {
			text.WriteString("export " + change.Name + "=" + shellQuoted(change.Value) + "\n")
		}
	}
	if p.PathFirst != "" {
		text.WriteString("export PATH=" + shellQuoted(p.PathFirst) + ":\"$PATH\"\n")
	}
	if p.APIKey {
		text.WriteString("ANTHROPIC_API_KEY read from the secret store\n")
	}
	words := []string{shellQuoted(p.Agent)}
	for _, word := range p.Words {
		words = append(words, shellQuoted(word))
	}
	text.WriteString("exec " + strings.Join(words, " ") + "\n")
	return text.String()
}

// launcherSettings are the settings a launcher hire hireID's start environment holds for its Claude Code, as its
// launcher would take them once, "" if none: they never are in a file or on its command line.
func launcherSettings(d *Deck, hireID string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, pending := range d.hires {
		if pending.hireID == hireID && pending.env != nil {
			return pending.env.Settings
		}
	}
	return ""
}

// startText is h's plan as planText reads it, or why there is none.
func (h hireStart) startText() (string, error) {
	plan, err := h.plan()
	if err != nil {
		return "", err
	}
	return planText(plan), nil
}
