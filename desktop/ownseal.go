package desktop

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// Smolcorp's own programs, checked at each desk start and each hook event (SANDBOX-PLAN §3.4, SG2; §4 g8). A hire
// can write inside Smolcorp.app as the owner can: it could replace bin/smolcorp (the guard), bin/tmux or
// bin/smolcorp-spawn. A stat of the three is cheap; when one changed, the app's seal is checked again, and its
// digest must still be the one it had at the first look. If it fails, no desk starts until Smolcorp is installed
// again, and Setup says so. This detects; it doesn't prevent (§5): a change counts from the next start or event.

// ownFilesChanged is the refusal, in Setup's words.
const ownFilesChanged = "Smolcorp's own files changed. Reinstall Smolcorp."

// ownPrograms are the files in the app's bin folder looked at.
var ownPrograms = []string{"smolcorp", "tmux", "smolcorp-spawn"}

// ownAppFolder is the Smolcorp.app this program runs from, "" when it runs from no app (a build of its own, Linux,
// Windows): then there's nothing to look at. A variable, for the tests.
var ownAppFolder = func() string {
	program, err := executable()
	if err != nil {
		return ""
	}
	return appHolding(program)
}

// ownAppSealed checks the app's seal; a variable, for the tests.
var ownAppSealed = appSealIntact

// appHolding is the .app whose Contents/Resources/bin holds program, its links resolved, or "".
func appHolding(program string) string {
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	bin := filepath.Dir(program)
	resources := filepath.Dir(bin)
	contents := filepath.Dir(resources)
	app := filepath.Dir(contents)
	if filepath.Base(bin) != "bin" || filepath.Base(resources) != "Resources" || filepath.Base(contents) != "Contents" ||
		!strings.HasSuffix(app, ".app") {
		return ""
	}
	return app
}

type ownSeal struct {
	mu     sync.Mutex
	looked bool
	app    string
	stamps string
	digest string
	broken bool
}

var ownFiles = &ownSeal{}

// stampsNow are the bin folder's programs' stamps as they are now.
func (s *ownSeal) stampsNow() string {
	parts := make([]string, len(ownPrograms))
	for i, name := range ownPrograms {
		parts[i] = name + "=" + fileStamp(filepath.Join(s.app, "Contents", "Resources", "bin", name))
	}
	return strings.Join(parts, ";")
}

// changed says whether there's anything to check: the first look, or a program's stamp moved. With s.mu held.
func (s *ownSeal) changed() (stamps string, check bool) {
	if !s.looked {
		s.looked, s.app = true, ownAppFolder()
		if s.app == "" {
			return "", false
		}
		return s.stampsNow(), true
	}
	if s.app == "" || s.broken {
		return "", false
	}
	stamps = s.stampsNow()
	return stamps, stamps != s.stamps
}

// check checks the seal and keeps the verdict: the stamps it was taken on, or broken for good.
func (s *ownSeal) check(stamps string) {
	digest, intact := ownAppSealed(s.app, s.digest)
	if !intact {
		s.broken = true
		return
	}
	s.stamps, s.digest = stamps, digest
}

// refusal is why no desk starts, or nil: the seal checked now when a program changed.
func (s *ownSeal) refusal() *hireRefusal {
	s.mu.Lock() // an event's look checking the seal holds it: its verdict is this one's
	defer s.mu.Unlock()
	if stamps, check := s.changed(); check {
		s.check(stamps)
	}
	if s.broken {
		return &hireRefusal{Status: http.StatusConflict, Message: ownFilesChanged + " No desk was started."}
	}
	return nil
}

// noteEvent is a hook event's look: a stat of the programs, and when one changed, the seal checked in the
// background, so the event isn't held.
func (s *ownSeal) noteEvent() {
	if !s.mu.TryLock() {
		return // a look is under way
	}
	stamps, check := s.changed()
	if !check {
		s.mu.Unlock()
		return
	}
	go func() {
		defer s.mu.Unlock()
		s.check(stamps)
	}()
}

// ownSealRefusal is why no desk starts because Smolcorp's own files changed, or nil.
func ownSealRefusal() *hireRefusal { return ownFiles.refusal() }
