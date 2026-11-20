package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// The Mac's checks, with Apple's own codesign (/usr/bin, under SIP: no program of the owner's can replace it).

// codesignProgram is Apple's codesign; a variable, for the tests.
var codesignProgram = "/usr/bin/codesign"

// anthropicRequirement is what Claude Code's program must satisfy: signed through Apple by Anthropic's team.
const anthropicRequirement = `anchor apple generic and certificate leaf[subject.OU] = "Q6L2SF6YDW"`

// hardenedRuntimeFlag is the code directory flag the hardened runtime sets (CS_RUNTIME).
const hardenedRuntimeFlag = 0x10000

var codeDirectoryFlags = regexp.MustCompile(`(?m)^CodeDirectory .*\bflags=0x([0-9a-fA-F]+)`)

// cdHashLine is the code's digest, as codesign --display --verbose=3 shows it.
var cdHashLine = regexp.MustCompile(`(?m)^CDHash=([0-9a-f]+)$`)

// codesign runs codesign with args, and says what it printed on its errors and whether it said yes.
func codesign(args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var printed bytes.Buffer
	cmd := disclaim.Cmd(exec.CommandContext(ctx, codesignProgram, args...))
	cmd.Stdout, cmd.Stderr = &printed, &printed
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	err := cmd.Run()
	return printed.String(), err == nil
}

// signedWith is why program isn't signed to satisfy requirement with the hardened runtime, or nil.
func signedWith(program, requirement string) error {
	if _, ok := codesign("--verify", "--strict", "-R="+requirement, program); !ok {
		return errors.New("not signed as required")
	}
	shown, ok := codesign("--display", "--verbose=1", program)
	found := codeDirectoryFlags.FindStringSubmatch(shown)
	if !ok || found == nil {
		return errors.New("its signature can't be read")
	}
	flags, err := strconv.ParseUint(found[1], 16, 32)
	if err != nil || flags&hardenedRuntimeFlag == 0 {
		return errors.New("no hardened runtime")
	}
	return nil
}

// claudeSignatureRefusal is why program, a file named claude, mustn't start as Claude Code: Anthropic's signature
// and the hardened runtime, or the refusal (the reason itself isn't said: one sentence says what to do).
func claudeSignatureRefusal(program string) error {
	if signedWith(program, anthropicRequirement) != nil {
		return errNotClaudeCode
	}
	return nil
}

// stampIdentity is the part of a file's stamp only its own file has: device, inode, size and change time.
func stampIdentity(info os.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatInt(int64(st.Dev), 10) + ":" + strconv.FormatUint(st.Ino, 10) + ":" + strconv.FormatInt(st.Size, 10) +
		":" + strconv.FormatInt(st.Ctimespec.Sec, 10) + "." + strconv.FormatInt(st.Ctimespec.Nsec, 10)
}

// appSealIntact is whether the app at app still fits its seal, every program inside included, and is still the
// one it was when its digest, cdhash, was taken (re-signing it with another identity, or none, gives another).
// cdhash "" asks for the digest alone.
func appSealIntact(app, cdhash string) (digest string, intact bool) {
	if _, ok := codesign("--verify", "--deep", "--strict", app); !ok {
		return "", false
	}
	shown, ok := codesign("--display", "--verbose=3", app)
	found := cdHashLine.FindStringSubmatch(shown)
	if !ok || found == nil {
		return "", false
	}
	return found[1], cdhash == "" || found[1] == cdhash
}

// peerRunsProgram is whether process pid runs program's code, as the system says it: codesign checks the running
// process (a pid, not a file) against program's own digest, which the system holds for the process from its start
// and no program can change. A process that started another program, or program replaced since, fails. A program
// with no signature at all (a build of one's own on an Intel Mac; never the app, whose every program is signed)
// has no digest to check: the ticket's pin alone holds then.
func peerRunsProgram(pid int, program string) bool {
	shown, ok := codesign("--display", "--verbose=3", program)
	found := cdHashLine.FindStringSubmatch(shown)
	if !ok || found == nil {
		return strings.Contains(shown, "not signed at all")
	}
	_, ok = codesign("--verify", `-R=cdhash H"`+found[1]+`"`, strconv.Itoa(pid))
	return ok
}

// claudeSignaturesChecked: a Mac checks Claude Code's signature.
const claudeSignaturesChecked = true
