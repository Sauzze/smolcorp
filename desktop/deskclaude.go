package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A Claude folder per desk (SA-X8, SANDBOX-PLAN NB6 and A0.1 C2): each Claude Code desk the engine starts gets its
// own config folder, hires/<id>/claude/ in the support folder, as its CLAUDE_CONFIG_DIR. What the desk writes there
// (its transcripts, todos, shell snapshots, its .claude.json) is its own: nothing in it is run by the owner's sessions
// or another desk's, and the account's folder stays the owner's, which the sandbox denies a desk to write (§A4.1).
// Fire takes the folder away.
//
// In it, the core puts:
//   - links to what the desk reads from its account's folder (deskClaudeLinks), the sandbox keeping them read-only;
//   - a read-only copy of the account's settings.json (its user settings: hooks, permissions, env);
//   - its own .claude.json, seeded from the account's with an allowlist: who is signed in (oauthAccount, not a
//     secret), the first-run flags, its folder trusted, an API-key hire's key approved, and only the MCP servers the
//     owner ticked (N5: their env often holds keys). Nothing else of the account's file is copied.
//
// No sign-in is copied (C2): the desk uses its account's own, through CLAUDE_SECURESTORAGE_CONFIG_DIR, set to the
// account's folder, or to "" for ~/.claude's (hireStart.plan). Claude Code reads its Keychain item's name and, off a
// Mac, its .credentials.json's folder from that variable (seen in Claude Code 2.1.325's own code).

// deskClaudeName is the desk's Claude folder's name in its own folder.
const deskClaudeName = "claude"

// secureStorageEnvironment names the folder whose sign-in Claude Code uses, whatever CLAUDE_CONFIG_DIR says.
const secureStorageEnvironment = "CLAUDE_SECURESTORAGE_CONFIG_DIR"

// deskClaudeLinks are what a desk reads from its account's folder through a link, as SANDBOX-PLAN SB15 names them
// and internal/sandbox's Policy write-denies them ("the links the core put in its Claude folder").
var deskClaudeLinks = []string{"CLAUDE.md", "commands", "agents", "skills", "plugins", "output-styles"}

// deskClaudeReadable are the two places in a desk's Claude folder its own tools may read (X8SKILLS1, SIERRA's ruling of
// 15 Nov): the skills and plugins the core links (or copies) there. Claude Code tells a skill where its own files are
// (its "Base directory"), the folder it loaded it from, so a skill's references/x.md is read through the desk's folder,
// and an allow rule can't lift a deny. Everything else there stays denied to Read: its transcripts (projects), prompt
// history, todos, paste cache, shell snapshots, .claude.json and whatever a later Claude Code keeps there.
var deskClaudeReadable = []string{"skills", "plugins"}

// deskClaudeOthers are patterns, in Claude Code's rule syntax, that together match each name in a desk's Claude
// folder but deskClaudeReadable's: Claude Code's rules have no "but", so the rest is written out (namesOtherThan).
var deskClaudeOthers = namesOtherThan(deskClaudeReadable...)

// namesOtherThan gives patterns that together match one name in a folder, whatever it is, unless it is one of names,
// which they never match. names are lower-case letters from b to y, no two starting with the same one. A name is
// another one if it starts with none of their first letters; or, for one of names, it is a start of it on its own, or
// a start of it followed by anything but its next letter, or the whole of it followed by more.
//
// Claude Code matches its rules as gitignore does, through the ignore package, which knows no [!…] (nor [^…]): "any
// letter but x" is a list of ranges, from U+0001 to U+FFFF with the gap x leaves, and the gap "/" leaves, so the
// pattern stays within one name. That package matches without regard to case, so the gap is both x and X: a range
// holding X would match x too. A name is never empty and never holds U+0000, and Claude Code compares UTF-16 code
// units, of which U+FFFF is the last.
func namesOtherThan(names ...string) []string {
	firsts := ""
	for _, name := range names {
		firsts += name[:1]
	}
	patterns := []string{"[" + anyBut(firsts) + "]*"}
	for _, name := range names {
		for i := 1; i < len(name); i++ {
			patterns = append(patterns, name[:i], name[:i]+"["+anyBut(name[i:i+1])+"]*")
		}
		// ? is any one character but "/", in Claude Code's rules as in gitignore.
		patterns = append(patterns, name+"?*")
	}
	return patterns
}

// anyBut is the inside of a [range] that matches one character but "/" and the letters (lower case), in either case.
func anyBut(letters string) string {
	gaps := []rune{'/'}
	for _, letter := range letters {
		gaps = append(gaps, letter, letter-'a'+'A')
	}
	slices.Sort(gaps)
	var ranges strings.Builder
	from := rune(1)
	for _, gap := range append(gaps, 0x10000) {
		if gap > from {
			ranges.WriteRune(from)
			if gap-1 > from {
				ranges.WriteString("-")
				ranges.WriteRune(gap - 1)
			}
		}
		from = gap + 1
	}
	return ranges.String()
}

// deskPlansDirectory is where desks with their own Claude folder keep their plans, each in a folder of its own named
// by its id (deskPlansFor), which it writes with Write and reads with Read: every desk's fence denies both tools that
// Claude folder whole (hireDenyRules), so plans can't stay in its plans/. Claude Code takes a plansDirectory only
// inside the project (2.1.325 refuses any other, and its default is the denied plans/), so this is in the desk's
// place, and the sandbox forbids making a .claude folder there. makeDeskPlans makes it, ignored by git.
const deskPlansDirectory = ".smolcorp/plans"

// deskPlansFor is desk hireID's own plans folder, relative to the folder it starts in: a desk without a worktree and
// the GM share their folder with others, and only its own plans go at its fire (removeDeskPlans).
func deskPlansFor(hireID string) string {
	return deskPlansDirectory + "/" + hireID
}

// makeDeskPlans makes desk hireID's plans folder (deskPlansFor) in place, the folder a desk starts in, if it isn't
// there, and tells git to add nothing in its top folder: a plan never goes in a desk's commits, and never keeps a fired
// desk's worktree as unclean. A link at any of its names is refused, never followed.
func makeDeskPlans(place, hireID string) error {
	if !oneName(hireID) {
		return errDeskClaude
	}
	top, _, _ := strings.Cut(deskPlansDirectory, "/")
	for _, folder := range []string{filepath.Join(place, top), filepath.Join(place, filepath.FromSlash(deskPlansDirectory)),
		filepath.Join(place, filepath.FromSlash(deskPlansFor(hireID)))} {
		if err := os.Mkdir(folder, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return errDeskClaude
		}
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() {
			return errDeskClaude
		}
	}
	ignore, err := os.OpenFile(filepath.Join(place, top, ".gitignore"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return errDeskClaude
	}
	_, err = ignore.WriteString("*\n")
	if ignore.Close() != nil || err != nil {
		return errDeskClaude
	}
	return nil
}

// removeDeskPlans takes desk hireID's plans folder away from place, the folder it started in, at its fire: for a hire
// without a worktree and the GM, whose folder others share, so its plans don't stay there for any desk to read
// (SIERRA, MSYSAX8). Opened from place, never through a link: a link at .smolcorp or plans leads nowhere it removes,
// and links inside are removed as links.
func removeDeskPlans(place, hireID string) error {
	if place == "" || !oneName(hireID) {
		return errDeskClaude
	}
	root, err := os.OpenRoot(place)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, folder := range []string{strings.Split(deskPlansDirectory, "/")[0], deskPlansDirectory} {
		if info, err := root.Lstat(filepath.FromSlash(folder)); err != nil || !info.IsDir() {
			return nil // nothing made there, or not the core's: nothing to take
		}
	}
	return root.RemoveAll(filepath.FromSlash(deskPlansFor(hireID)))
}

// oneName is whether hireID is a desk's id as a folder can be named by it: one plain name.
func oneName(hireID string) bool {
	return hireID != "" && hireID != "." && hireID != ".." && filepath.Base(hireID) == hireID && !filepath.IsAbs(hireID) &&
		!strings.ContainsAny(hireID, `/\`)
}

// deskOwnClaude is desk hireID's own Claude folder, and true, when the engine made it one at its start (it goes at
// its fire): a folder there, never a link. Only the core's own path is looked at, never one a hook names.
func deskOwnClaude(hireID string) (string, bool) {
	if !oneName(hireID) {
		return "", false
	}
	folder := filepath.Clean(deskClaudeFolder(supportDir, hireID))
	info, err := os.Lstat(folder)
	return folder, err == nil && info.IsDir()
}

// isDeskClaudePlace is whether place, a transcript's (transcriptPlace), is a desk's Claude folder: hires/<id>/claude
// in the support folder, as written or as the support folder resolves. Never an account's, whatever the account list
// says.
func isDeskClaudePlace(place string) bool {
	if place == "" || filepath.Base(place) != deskClaudeName {
		return false
	}
	hires := filepath.Dir(filepath.Dir(filepath.Clean(place)))
	if hires == filepath.Join(filepath.Clean(supportDir), "hires") {
		return true
	}
	resolved, err := filepath.EvalSymlinks(supportDir)
	return err == nil && hires == filepath.Join(resolved, "hires")
}

// deskClaudeSeeded are the first-run flags a desk's .claude.json takes from its account's, besides oauthAccount and
// the ticked MCP servers: Claude Code writes the two together when its first run ends.
var deskClaudeSeeded = []string{"lastOnboardingVersion"}

// deskClaudeFolder is desk hireID's own Claude folder in support.
func deskClaudeFolder(support, hireID string) string {
	return filepath.Join(hireFolder(support, hireID), deskClaudeName)
}

// deskClaude is what a desk's Claude folder is made from.
type deskClaude struct {
	// Account is the account's config folder, "" for ~/.claude's (whose .claude.json is in the home folder).
	Account string
	// Trusted is the folder the desk works in, as prepareHire trusted it in the account's file.
	Trusted string
	// KeyTail is an API-key hire's key's tail, approved in its file (keyApproval); nil for a subscription hire.
	KeyTail *string
	// MCP are the names of the account's MCP servers the owner ticked for desks; nil for none.
	MCP []string
}

// errDeskClaude is why a desk gets no Claude folder of its own: its start is refused.
var errDeskClaude = errors.New("Smolcorp couldn't make the hire's own Claude folder")

// accountFolder is the account's config folder itself: ~/.claude for the default account.
func (c deskClaude) accountFolder() string {
	if c.Account == "" {
		return filepath.Join(homeDir, ".claude")
	}
	return c.Account
}

// makeDeskClaude makes folder, a desk's own Claude folder, from c, or nothing at all. The folder must not exist yet:
// it is new for this start, so nothing already there is ever written through.
func makeDeskClaude(folder string, c deskClaude) error {
	seeded, err := c.seed()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(folder), 0o700); err != nil {
		return errDeskClaude
	}
	if err := os.Mkdir(folder, 0o700); err != nil {
		return errDeskClaude
	}
	if err := c.fill(folder, seeded); err != nil {
		_ = os.RemoveAll(folder)
		return err
	}
	return nil
}

// fill puts the links, the settings' copy and the seeded .claude.json in folder, just made.
func (c deskClaude) fill(folder string, seeded []byte) error {
	account := c.accountFolder()
	budget := deskClaudeCopyBudget{entries: deskClaudeCopyMost, bytes: deskClaudeCopyBytesMost}
	for _, name := range deskClaudeLinks {
		// Only what the account has: a link to nothing would stop Claude Code making that folder of its own.
		if _, err := os.Lstat(filepath.Join(account, name)); err != nil {
			continue
		}
		if deskClaudeCopies && slices.Contains(deskClaudeCopied, name) {
			if err := c.copyIn(filepath.Join(account, name), filepath.Join(folder, name), &budget); err != nil {
				return err
			}
			continue
		}
		if err := os.Symlink(filepath.Join(account, name), filepath.Join(folder, name)); err != nil {
			return errDeskClaude
		}
	}
	settings, err := os.ReadFile(filepath.Join(account, "settings.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return errDeskClaude
	default:
		if err := writeNewFile(filepath.Join(folder, "settings.json"), settings, 0o400); err != nil {
			return errDeskClaude
		}
	}
	// writeNewFile (demo.go) never writes through anything already there; the umask takes nothing from these modes.
	if err := writeNewFile(claudeJSON(folder), seeded, 0o600); err != nil {
		return errDeskClaude
	}
	return nil
}

// On Linux the desk's folder gets copies, not links (FXSAL1R1): under L1's view a desk sees nothing of ~/.claude or
// an account's folder, so a link there would lead nowhere. What a Mac desk reads through a link is copied in at each
// start instead, the folder being made afresh then: a skill or a command the owner changes reaches a desk at its next
// start. plugins stays a link, as on the Mac: it is a whole install, and desks never install or update plugins
// (A4.5); under L1 it leads nowhere, as if the account had none. settings.json is a copy everywhere (fill).
var (
	// deskClaudeCopies is whether the links are copies on this system: a variable, for the tests.
	deskClaudeCopies = runtime.GOOS == "linux"
	// deskClaudeCopied are the names in deskClaudeLinks copied there.
	deskClaudeCopied = []string{"CLAUDE.md", "commands", "agents", "skills", "output-styles"}
	// deskClaudeCopyMost and deskClaudeCopyBytesMost cap what one start copies: past either, the start is refused,
	// never left half copied.
	deskClaudeCopyMost            = 10_000
	deskClaudeCopyBytesMost int64 = 64 << 20
)

// errDeskClaudeCopy is a start refused because the account's own folders are too big to copy, or can't be read.
var errDeskClaudeCopy = errors.New("Smolcorp couldn't copy the account's CLAUDE.md, skills, commands, agents and output styles into the hire's own Claude folder: they are too big or can't be read")

// deskClaudeCopyBudget is what a start may still copy.
type deskClaudeCopyBudget struct {
	entries int
	bytes   int64
}

// copyIn copies from, one of deskClaudeLinks in the account's folder, to to in the desk's. from itself is followed,
// as the Mac's link would be (an owner's dotfiles often link it); below it, nothing that is a link is followed or
// copied, nor anything but folders and plain files. Never copied: a .credentials.json or a .claude.json, wherever it
// is, and anything in a place the Policy keeps from every desk's reading (deskClaudeNeverCopied). Files are read-only
// copies, as the links are read-only under the sandbox.
func (c deskClaude) copyIn(from, to string, budget *deskClaudeCopyBudget) error {
	resolved, err := filepath.EvalSymlinks(from)
	if err != nil {
		return errDeskClaudeCopy
	}
	if c.neverCopied(resolved) {
		return nil
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return errDeskClaudeCopy
	}
	return c.copyEntry(resolved, to, info, budget)
}

// copyEntry copies the folder or plain file at from, seen as info (never through a link), to to.
func (c deskClaude) copyEntry(from, to string, info fs.FileInfo, budget *deskClaudeCopyBudget) error {
	if budget.entries--; budget.entries < 0 {
		return errDeskClaudeCopy
	}
	name := info.Name()
	if name == ".credentials.json" || strings.HasPrefix(name, ".claude.json") || c.neverCopied(from) {
		return nil
	}
	switch {
	case info.Mode().IsRegular():
		if budget.bytes -= info.Size(); budget.bytes < 0 {
			return errDeskClaudeCopy
		}
		data, err := readRegularAt(from, info)
		if err != nil {
			return errDeskClaudeCopy
		}
		if writeNewFile(to, data, 0o400) != nil {
			return errDeskClaude
		}
		return nil
	case info.IsDir():
		if os.Mkdir(to, 0o700) != nil {
			return errDeskClaude
		}
		entries, err := os.ReadDir(from)
		if err != nil {
			return errDeskClaudeCopy
		}
		for _, entry := range entries {
			inner, err := os.Lstat(filepath.Join(from, entry.Name()))
			if err != nil {
				return errDeskClaudeCopy
			}
			if err := c.copyEntry(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name()), inner, budget); err != nil {
				return err
			}
		}
		return nil
	}
	return nil // a link, a fifo, a socket: never copied
}

// readRegularAt is the plain file at path, the one info saw: never one put there since, through a link or otherwise.
func readRegularAt(path string, info fs.FileInfo) ([]byte, error) {
	file, opened, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if !os.SameFile(info, opened) {
		return nil, errNotRegular
	}
	return io.ReadAll(io.LimitReader(file, opened.Size()+1))
}

// neverCopied is whether at is in a place the Policy keeps from every desk's reading (internal/sandbox's "the account
// folders' tokens and history", "the accounts' .claude.json", "other agents' sign-ins", the support folder whole): an
// owner's link that leads a copied folder there takes nothing of it into the desk's folder.
func (c deskClaude) neverCopied(at string) bool {
	at = filepath.Clean(at)
	var places []string
	for _, account := range []string{filepath.Join(homeDir, ".claude"), c.accountFolder()} {
		for _, secret := range []string{"ide", ".credentials.json", "history.jsonl", "projects", "sessions", ".claude.json"} {
			places = append(places, filepath.Join(account, secret))
		}
	}
	places = append(places, filepath.Join(homeDir, ".claude.json"), supportDir)
	places = append(places, sandbox.CredentialFiles(homeDir)...)
	for _, place := range places {
		if resolved, err := filepath.EvalSymlinks(place); err == nil && inside(resolved, at) || inside(filepath.Clean(place), at) {
			return true
		}
	}
	return false
}

// seed is the desk's .claude.json: the allowlisted members of the account's, then its folder trusted and an
// API-key hire's key approved by the same edits prepareHire makes in the account's file.
func (c deskClaude) seed() ([]byte, error) {
	data, err := os.ReadFile(claudeJSON(c.Account))
	if err != nil {
		return nil, errHireNoConfig
	}
	var account map[string]json.RawMessage
	if json.Unmarshal(data, &account) != nil || account == nil {
		return nil, errHireTrust
	}
	seeded := map[string]json.RawMessage{"hasCompletedOnboarding": json.RawMessage("true")}
	for _, name := range append([]string{"oauthAccount"}, deskClaudeSeeded...) {
		if value, ok := account[name]; ok {
			seeded[name] = value
		}
	}
	var servers map[string]json.RawMessage
	if raw, ok := account["mcpServers"]; ok && json.Unmarshal(raw, &servers) != nil {
		return nil, errHireTrust
	}
	ticked := map[string]json.RawMessage{}
	for name, server := range servers {
		if slices.Contains(c.MCP, name) {
			ticked[name] = server
		}
	}
	if len(ticked) > 0 {
		value, err := json.Marshal(ticked)
		if err != nil {
			return nil, errHireTrust
		}
		seeded["mcpServers"] = value
	}
	text, err := json.MarshalIndent(seeded, "", "  ")
	if err != nil {
		return nil, errHireTrust
	}
	if text, err = trusting(c.Trusted, text); err != nil {
		return nil, errHireTrust
	}
	if c.KeyTail != nil {
		if text, err = approving(*c.KeyTail, text); err != nil {
			return nil, errHireNoKey
		}
	}
	return text, nil
}

// removeDeskClaude takes desk hireID's Claude folder away, at its fire or a start that failed. The desk may have
// written anything in it, links too: they are removed as links, never followed. An ID that isn't one name removes
// nothing.
func removeDeskClaude(support, hireID string) error {
	if hireID == "" || hireID == "." || hireID == ".." || filepath.Base(hireID) != hireID || filepath.IsAbs(hireID) {
		return errDeskClaude
	}
	return os.RemoveAll(deskClaudeFolder(support, hireID))
}
