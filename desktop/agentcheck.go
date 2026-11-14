package desktop

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Every agent CLI the core starts is checked before it runs (SANDBOX-PLAN §3.4, SG2; RT126S2 D1): a desk's agent,
// S2's sign-in, S1's status probe, the summaries and the version check alike. A hire can write every folder
// programNamed looks in (~/.local/bin, ~/.local/share/claude, npm's and nvm's), so the program found there is
// whatever a hire left. Claude Code from Anthropic's installer is a signed program: on a Mac it must carry
// Anthropic's signature (team Q6L2SF6YDW) with the hardened runtime, or nothing starts it. A Claude Code installed
// by npm, and codex, gemini or opencode, are scripts nobody signs: there is nothing to check, and they start as
// they did (§5 says so). Off a Mac, Claude Code isn't signed in a way this computer can check, so nothing is.
//
// Which of the two the owner has isn't told by a path: a hire can write a script under any folder named like npm's
// (RT126SG2 B2). So the kind is remembered (claudeInstalls): once a signed Claude Code has passed, an unsigned one is
// refused until the owner confirms it on the setup page, which no hire can reach.

// errNotClaudeCode is a program named claude that isn't Anthropic's.
var errNotClaudeCode = errors.New("Smolcorp didn't start Claude Code: the program it found isn't signed by Anthropic, " +
	"so another program may have replaced it. Reinstall Claude Code with Anthropic's installer, then try again.")

// agentSignatureCheck checks a program that calls itself Claude Code, by its file, and says why it mustn't run, or
// nil: the Mac's signature check (agentcheck_darwin.go), nothing elsewhere. A variable, for the tests: TestMain
// puts a stand-in that passes, so the stand-in agents the tests run aren't refused; the rows that check the
// refusal put the real one back.
var agentSignatureCheck = claudeSignatureRefusal

// errClaudeUnsigned is an unsigned Claude Code where a signed one ran before.
var errClaudeUnsigned = errors.New("Claude Code changed to an unsigned copy. Confirm it in Setup.")

// npmClaudeCode is the folder npm installs Claude Code in: its program there is a script nobody signs.
const npmClaudeCode = "/node_modules/@anthropic-ai/claude-code/"

// wrapsHooks is whether a variable from Smolcorp's own environment, or one a shell's start files set, could turn
// a desk's hooks off or wrap them (§4 g10): none of them is passed on to a hire's agent.
func wrapsHooks(name string) bool {
	name = strings.ToUpper(name)
	switch name {
	case "CLAUDE_CODE_SHELL_PREFIX", "CLAUDE_CODE_SHELL", "BASH_ENV", "ENV", "ZDOTDIR":
		return true
	}
	return strings.HasPrefix(name, "CLAUDE_CODE_") && strings.Contains(name, "HOOK")
}

// CheckAgentStart is why program, found for a command line's first word, mustn't be started, or nil: the check
// smolcorp run makes (launcher.go), for `smolcorp gm login` (internal/gm, whose CheckProgram cmd/smolcorp sets).
func CheckAgentStart(word, program string) error {
	return checkAgentProgram(string(agentNamed(word)), program)
}

// checkAgentProgram is why program, found as the agent called name, mustn't be started, or nil. It goes by the
// name it was looked for as, never by the file's own: Anthropic's installer links ~/.local/bin/claude to a file
// named by its version, and a link planted in its place could name any file at all. The link is resolved again
// here, so one put in its place since is followed to what it names now.
func checkAgentProgram(name, program string) error {
	if name != string(Claude) || program == "" {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	if strings.Contains(filepath.ToSlash(program), npmClaudeCode) {
		if !claudeSignaturesChecked {
			return nil // nothing is checked here (above)
		}
		return claudeInstalls.unsigned(program) // npm's Claude Code: nothing to check (§5), unless a signed one ran
	}
	err := signatureVerdicts.verdict(program, agentSignatureCheck)
	if err == nil && claudeSignaturesChecked {
		claudeInstalls.remember(claudeSigned)
	}
	return err
}

// The kinds of Claude Code install remembered.
const (
	claudeSigned   = "signed" // Anthropic's installer's: signed, and checked
	claudeUnsigned = "npm"    // npm's: a script nobody signs
)

// claudeInstalls is which kind of Claude Code this computer has, remembered in the support folder's claude-install
// (SANDBOX-PLAN §3.4, RT126SG2 B2): the first one seen, made "signed" by any signed one that passes, and "npm" again
// only by the owner's confirmation on the setup page. It's read once, then kept in memory: a hire that changes the
// file changes nothing while Smolcorp runs. A hire can neither write nor read it (the hire fence, hiresettings.go,
// and the guard, internal/guard: RMSY126k nit 1); sealing it against a program of the owner's is SK1's follow-up, as
// for config.json's security settings.
var claudeInstalls = &claudeInstallMemory{}

// claudeInstallName is the support folder's file the kind is remembered in; claudeInstallTemp the file it is written
// through (writePrivate, paths.go).
const (
	claudeInstallName = "claude-install"
	claudeInstallTemp = claudeInstallName + ".tmp"
)

type claudeInstallMemory struct {
	mu   sync.Mutex
	path string // the file it was read from: a test's support folder is new for each test
	kind string
	// refused is the unsigned Claude Code last refused, for the setup page to name.
	refused string
}

func claudeInstallFile() string { return filepath.Join(supportDir, claudeInstallName) }

// read makes sure the kind is the support folder's. With m.mu held.
func (m *claudeInstallMemory) read() {
	if path := claudeInstallFile(); m.path != path {
		m.path, m.kind, m.refused = path, "", ""
		if data, err := os.ReadFile(path); err == nil {
			if kind := strings.TrimSpace(string(data)); kind == claudeSigned || kind == claudeUnsigned {
				m.kind = kind
			}
		}
	}
}

// keep makes kind the one remembered, on disk too. With m.mu held. A write that fails is said, never silent: the
// kind still holds while Smolcorp runs, but the next start would read what the disk had before (RT126SG2F B1).
func (m *claudeInstallMemory) keep(kind string) {
	if m.kind != kind {
		m.kind = kind
		if err := writePrivate(m.path, []byte(kind+"\n")); err != nil {
			sayClaudeInstall(fmt.Sprintf("which Claude Code this computer has (%s) couldn't be written down, so the next start won't know it: %v", kind, err))
		}
	}
}

// sayClaudeInstall says what went wrong keeping which Claude Code this computer has.
var sayClaudeInstall = func(text string) { log.Printf("smolcorp: %s", text) }

// remember notes that a Claude Code of kind passed.
func (m *claudeInstallMemory) remember(kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.read()
	m.keep(kind)
	if kind == claudeSigned {
		m.refused = ""
	}
}

// unsigned is why the unsigned Claude Code at program mustn't start, or nil: refused while a signed one is
// remembered, and remembered as the kind when it's the first seen.
func (m *claudeInstallMemory) unsigned(program string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.read()
	if m.kind == claudeSigned {
		m.refused = program
		return errClaudeUnsigned
	}
	m.keep(claudeUnsigned)
	return nil
}

// waiting is the unsigned Claude Code refused since a signed one ran, for the setup page to offer, or "".
func (m *claudeInstallMemory) waiting() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.read()
	return m.refused
}

// confirm is the owner's yes, on the setup page alone: the unsigned Claude Code is the one to start from now on.
func (m *claudeInstallMemory) confirm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.read()
	m.keep(claudeUnsigned)
	m.refused = ""
}

// signatureVerdicts keeps each checked file's verdict while the file is the same one: its device, inode, size,
// modification and change times. A file written, replaced, renamed over or touched has another stamp (the change
// time can't be set back by any program), so it is checked again before it runs. The summaries and S1's probe
// run often; a signature check takes a moment.
var signatureVerdicts = &verdictCache{verdicts: map[string]cachedVerdict{}}

type cachedVerdict struct {
	stamp string
	err   error
}

type verdictCache struct {
	mu       sync.Mutex
	verdicts map[string]cachedVerdict
}

func (c *verdictCache) verdict(program string, check func(string) error) error {
	stamp := fileStamp(program)
	if stamp == "" {
		return errNotClaudeCode // gone, or not a plain file: nothing to start
	}
	c.mu.Lock()
	kept, ok := c.verdicts[program]
	c.mu.Unlock()
	if ok && kept.stamp == stamp {
		return kept.err
	}
	err := check(program)
	// Kept only when the file didn't change while it was checked.
	if fileStamp(program) == stamp {
		c.mu.Lock()
		c.verdicts[program] = cachedVerdict{stamp: stamp, err: err}
		c.mu.Unlock()
	}
	return err
}

// fileStamp is what says a file is still the one it was, or "" for one that isn't a plain file.
func fileStamp(path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return info.ModTime().String() + "|" + stampIdentity(info)
}
