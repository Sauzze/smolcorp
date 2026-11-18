package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Mail to and from a cloud bot through a folder, ported from the Mac's BotMailbox.swift (T404).
// A cloud bot has no window to type into, so the owner picks, on the setup page, a folder its side
// reads and writes (a synced folder, say). In it:
//
//   - to-bot/<time>-<id>.md: a message for the bot, the wire text (mailWire). Written whole under a
//     hidden name and renamed, so the bot never reads half of one.
//   - from-bot/: files the bot leaves for the GM. Each is read once, moved to from-bot/read/, and
//     becomes a mail line in the event log. Only plain .md and .txt files; a link is never followed.
//
// The folder is never inside the support folder nor a hire's worktree (nor holds one), since the
// bot's side can write there. Checked when it is chosen and again on every use. Nothing here types
// anywhere.
const (
	botMailboxToBot = "to-bot"
	botMailboxFrom  = "from-bot"
	botMailboxDone  = "read"
	// botMailboxMostPerLook is the most replies taken in one look; the rest wait for the next.
	botMailboxMostPerLook = 20
	// botMailboxSettle: a file changed this recently may still be arriving (a sync), so it is left
	// for the next look.
	botMailboxSettle = 3 * time.Second
	// The name of a file in to-bot: the time as yyyyMMdd'T'HHmmss'Z', in UTC.
	botMailboxTimeLayout = "20060102T150405Z"
)

// botReply is one file a bot left: its name as the bot left it, and what it said.
type botReply struct {
	Name string
	Text string
}

// botMailboxProblem is why a folder can't be a mailbox. Never: it never can be (the Mac's .failed).
// Otherwise it may be a folder that is not there yet (a synced one), and is tried again (.trouble).
type botMailboxProblem struct {
	Never bool
	Why   string
}

// botMailboxRefusal says whether path can be a mailbox: nil if so. barred are the folders it may not
// be in or hold (the support folder, the hires' worktrees). BotMailbox.refusal ~25-37.
func botMailboxRefusal(path string, barred []string) *botMailboxProblem {
	if !filepath.IsAbs(path) {
		return &botMailboxProblem{Never: true, Why: "The mailbox folder has to be a full path."}
	}
	info, err := os.Stat(path)
	if err != nil {
		return &botMailboxProblem{Why: "The mailbox folder isn't there (is it synced?): " + path}
	}
	if !info.IsDir() {
		return &botMailboxProblem{Never: true, Why: "The mailbox isn't a folder: " + path}
	}
	here := botCanonical(path)
	for _, folder := range barred {
		if botOverlap(here, botCanonical(folder)) {
			return &botMailboxProblem{Never: true, Why: "The mailbox can't be in or hold the app's own folder or a hire's worktree: " + folder}
		}
	}
	return nil
}

// botMailboxChosen is the path to keep for a folder the owner chose, or why not (a folder that
// isn't there is refused too). BotMailbox.chosen ~40-46.
func botMailboxChosen(path string, barred []string) (string, error) {
	full := botExpandTilde(strings.TrimSpace(path))
	if problem := botMailboxRefusal(full, barred); problem != nil {
		return "", errors.New(problem.Why)
	}
	return botCanonical(full), nil
}

func botExpandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// botCanonical is the path with every link resolved, as far as it exists: a folder that is gone is
// still found under its parent's real name (a Mac's /var is /private/var).
func botCanonical(path string) string {
	path = filepath.Clean(path)
	rest, current := "", path
	for {
		if real, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		rest, current = filepath.Join(filepath.Base(current), rest), parent
	}
}

func botOverlap(a, b string) bool {
	if runtime.GOOS == "windows" {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	inside := func(child, parent string) bool {
		sep := string(filepath.Separator)
		return child == parent || strings.HasPrefix(child, strings.TrimSuffix(parent, sep)+sep)
	}
	return inside(a, b) || inside(b, a)
}

// botRealFolder is whether path is a folder that really is one, not a link to somewhere else;
// made if create and absent.
func botRealFolder(path string, create bool) bool {
	info, err := os.Lstat(path)
	if err != nil {
		if !create || os.Mkdir(path, 0o755) != nil {
			return false
		}
		info, err = os.Lstat(path)
		if err != nil {
			return false
		}
	}
	return info.Mode().IsDir()
}

// MARK: To the bot

// botMailboxFileName is the file's name for message id sent at at: sorts by time, and ends in the
// id so it is found again.
func botMailboxFileName(id string, at time.Time) string {
	return at.UTC().Format(botMailboxTimeLayout) + "-" + id + ".md"
}

// botMailboxDrop writes text to the bot's to-bot folder. commit counts the try just before the
// write, as for a window. A message already there under its id is delivered, so a crash between
// the write and the save sends nothing twice. BotMailbox.drop ~82-103.
func botMailboxDrop(text, id string, at time.Time, mailbox string, barred []string, commit func() bool) mailResult {
	if problem := botMailboxRefusal(mailbox, barred); problem != nil {
		if problem.Never {
			return mailResult{Outcome: mailCannot, Note: problem.Why}
		}
		return mailResult{Outcome: mailTrouble, Note: problem.Why}
	}
	out := filepath.Join(mailbox, botMailboxToBot)
	if !botRealFolder(out, true) {
		return mailResult{Outcome: mailCannot, Note: botMailboxToBot + " in the mailbox isn't a folder, or couldn't be made."}
	}
	names, _ := os.ReadDir(out)
	for _, name := range names {
		if strings.HasSuffix(name.Name(), "-"+id+".md") {
			return mailResult{Outcome: mailSent}
		}
	}
	if !commit() {
		return mailResult{Outcome: mailTrouble, Note: "The outbox couldn't be written, so nothing was put in the mailbox."}
	}
	if botWritePrivately([]byte(text+"\n"), filepath.Join(out, botMailboxFileName(id, at))) != nil {
		return mailResult{Outcome: mailTrouble, Note: "The mailbox folder couldn't be written to."}
	}
	return mailResult{Outcome: mailSent}
}

// botWritePrivately writes data whole to a hidden file beside path (only the owner can read it, and
// it is new), then renames it to path.
func botWritePrivately(data []byte, path string) error {
	temp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".part")
	os.Remove(temp) // one left by a crash
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closed := f.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		os.Remove(temp)
	}
	return err
}

// MARK: From the bot

// botMailboxCollect is the new replies in from-bot, oldest first, each moved to from-bot/read before
// it is handed back, so it is never read twice (one that can't be moved is left alone and not handed
// back). A reply longer than maxBytes is cut there. BotMailbox.collect ~110-143.
func botMailboxCollect(mailbox string, barred []string, maxBytes int, now time.Time) []botReply {
	return botMailboxCollectWith(mailbox, barred, maxBytes, now, nil)
}

// botMailboxCollectWith is botMailboxCollect with a hook for the tests, called after a file is read.
func botMailboxCollectWith(mailbox string, barred []string, maxBytes int, now time.Time, afterRead func(path string)) []botReply {
	if botMailboxRefusal(mailbox, barred) != nil {
		return nil
	}
	inbox := filepath.Join(mailbox, botMailboxFrom)
	if !botRealFolder(inbox, false) {
		return nil
	}
	entries, err := os.ReadDir(inbox)
	if err != nil {
		return nil
	}
	type found struct {
		path string
		info os.FileInfo
	}
	var ready []found
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if strings.HasPrefix(name, ".") || (ext != "md" && ext != "txt") {
			continue
		}
		path := filepath.Join(inbox, name)
		// Lstat: a link is never a regular file here, whatever it points at.
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || now.Sub(info.ModTime()) < botMailboxSettle {
			continue
		}
		ready = append(ready, found{path, info})
	}
	sort.SliceStable(ready, func(i, j int) bool {
		if !ready[i].info.ModTime().Equal(ready[j].info.ModTime()) {
			return ready[i].info.ModTime().Before(ready[j].info.ModTime())
		}
		return ready[i].info.Name() < ready[j].info.Name()
	})
	if len(ready) > botMailboxMostPerLook {
		ready = ready[:botMailboxMostPerLook]
	}
	if len(ready) == 0 {
		return nil
	}
	finished := filepath.Join(inbox, botMailboxDone)
	if !botRealFolder(finished, true) {
		return nil
	}

	var replies []botReply
	for _, file := range ready {
		// Read first, then moved; only a moved file is handed back, so a failed move can't make it twice.
		data, ok := botReadSteady(file.path, file.info, maxBytes, afterRead)
		if !ok {
			continue
		}
		if os.Rename(file.path, botFree(finished, file.info.Name(), now)) != nil {
			continue
		}
		text := string(data[:min(len(data), maxBytes)])
		text = strings.Map(func(r rune) rune { return r }, text) // each bad byte becomes U+FFFD
		if len(data) > maxBytes {
			text += "\n[cut at " + strconv.Itoa(maxBytes) + " bytes]"
		}
		if text = strings.TrimSpace(text); text != "" {
			replies = append(replies, botReply{Name: file.info.Name(), Text: text})
		}
	}
	return replies
}

// botReadSteady reads up to maxBytes+1 bytes of the file that was seen as before, and answers false
// for one that is not that file any more or that changed while it was read: it is left for the next
// look.
func botReadSteady(path string, before os.FileInfo, maxBytes int, afterRead func(string)) ([]byte, bool) {
	f, err := openNoBlock(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !botSame(before, opened) {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return nil, false
	}
	if afterRead != nil {
		afterRead(path)
	}
	after, err := f.Stat()
	if err != nil || !botSame(before, after) {
		return nil, false
	}
	// And still the file at that path, not a link put there since.
	now, err := os.Lstat(path)
	if err != nil || !now.Mode().IsRegular() || !os.SameFile(before, now) || !botSame(before, now) {
		return nil, false
	}
	return data, true
}

func botSame(a, b os.FileInfo) bool { return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) }

// botFree is name in folder, or with the time before it when there already is one.
func botFree(folder, name string, at time.Time) string {
	plain := filepath.Join(folder, name)
	if _, err := os.Lstat(plain); err != nil {
		return plain
	}
	return filepath.Join(folder, fmt.Sprintf("%d-%s-%s", at.Unix(), newMailID()[:6], name))
}

// MARK: The Deck

// mailboxBarred is where a cloud bot's mailbox may not be: in or around the support folder or any
// hire's worktree (Deck.mailboxBarred ~51-53). It reads the registry, so call it without the lock.
func (d *Deck) mailboxBarred() []string {
	barred := []string{supportDir}
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Worktree != "" {
			barred = append(barred, h.Worktree)
		}
	}
	return barred
}

// botsWithMailbox is a copy of the bots that have a mailbox folder.
func (d *Deck) botsWithMailbox() []CloudBot {
	d.mu.Lock()
	defer d.mu.Unlock()
	var bots []CloudBot
	for _, b := range d.cloud.bots {
		if b.Mailbox != "" {
			bots = append(bots, b)
		}
	}
	return bots
}

// botMailboxLook looks in each bot's from-bot folder for replies, and makes each new one a mail line
// in the event log (MailRouter.collectReplies ~470-490, Deck.swift ~187-190). A look still out is not
// started again. The mail pump calls this every 5 s (mail_pump.go).
func (d *Deck) botMailboxLook(now time.Time) {
	if !d.looking.TryLock() {
		return
	}
	defer d.looking.Unlock()
	bots := d.botsWithMailbox()
	if len(bots) == 0 {
		return
	}
	barred, most := d.mailboxBarred(), d.mailLimits().MaxBytes
	for _, bot := range bots {
		for _, reply := range botMailboxCollect(bot.Mailbox, barred, most, now) {
			d.owner.notOwners(reply.Text) // a bot's words are never the owner's (ownerwords.go)
			d.events.append(eventEntry{At: now.UnixMilli(), Session: bot.sessionID(), Folder: bot.folder(), Agent: bot.Agent,
				Kind: eventKindMail, Detail: bot.Name + " wrote " + botCleanName(reply.Name) + ": " + reply.Text})
		}
	}
}

// botCleanName is a file's name with every control character made a space.
func botCleanName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cc, r) {
			return ' '
		}
		return r
	}, name)
}

// mailDeliverBot tries a queued message to a bot: it becomes a file in the bot's mailbox, whatever
// else is wired (MailRouter.attempt(_:bot:) ~357-363, and the bot part of pump ~445-455). A bot that
// is gone fails the message. The mail pump (mail_pump.go) calls this for each queued message to "bot:<id>".
func (d *Deck) mailDeliverBot(m mailMessage) {
	d.mu.Lock()
	var bot *CloudBot
	for i := range d.cloud.bots {
		if mailBotPrefix+d.cloud.bots[i].ID == m.To {
			b := d.cloud.bots[i]
			bot = &b
			break
		}
	}
	d.mu.Unlock()
	if bot == nil {
		d.mail.fail(m.ID, m.ToName+" is gone.")
		return
	}
	barred := d.mailboxBarred()
	attempt := d.mail.start(m.ID)
	if attempt == nil {
		return
	}
	result := mailResult{Outcome: mailCannot, Note: bot.Name + " has no mailbox folder any more."}
	if bot.Mailbox != "" {
		result = botMailboxDrop(attempt.Text, m.ID, time.UnixMilli(int64(m.At)), bot.Mailbox, barred, attempt.Commit)
	}
	attempt.Finish(result)
}

// MARK: The routes

// keepMailboxes gives each bot the folder it had before: POST /setup/bots never sets or changes it
// (DeckRouter.swift ~723-725).
func keepMailboxes(bots, before []CloudBot) []CloudBot {
	for i := range bots {
		bots[i].Mailbox = ""
		if j := slices.IndexFunc(before, func(b CloudBot) bool { return b.ID == bots[i].ID }); j >= 0 {
			bots[i].Mailbox = before[j].Mailbox
		}
	}
	return bots
}

// botMailboxChoose is POST /setup/bots/mailbox (page only): {id, folder}; an empty folder takes it
// away (DeckRouter.swift ~768-790). It answers the bots, as GET /setup/bots does.
func (rt *router) botMailboxChoose(w http.ResponseWriter, r *request) {
	d := rt.deck
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	var given struct {
		ID     any `json:"id"`
		Folder any `json:"folder"`
	}
	_ = json.Unmarshal(raw, &given)
	id, idOK := given.ID.(string)
	folder, folderOK := given.Folder.(string)
	known := func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return slices.ContainsFunc(d.config.CloudBots, func(b CloudBot) bool { return b.ID == id })
	}
	const ask = "Send {id, folder}: the bot, and the folder its mail goes through (empty for none)."
	if !idOK || !folderOK || !known() {
		writeStatus(w)(http.StatusBadRequest, ask)
		return
	}
	kept := ""
	if strings.TrimSpace(folder) != "" {
		var err error
		if kept, err = botMailboxChosen(folder, d.mailboxBarred()); err != nil {
			writeStatus(w)(http.StatusBadRequest, err.Error())
			return
		}
	}
	d.mu.Lock()
	bots := slices.Clone(d.config.CloudBots)
	found := false
	for i := range bots {
		if bots[i].ID == id {
			bots[i].Mailbox, found = kept, true
		}
	}
	if !found {
		d.mu.Unlock()
		writeStatus(w)(http.StatusBadRequest, ask)
		return
	}
	// A mailbox cleared or moved while the records' key can't be read would be back at the restart, mail going
	// through it again (records_heldconfig.go, RT126SK1F B1).
	if refused := d.config.heldRefusal(func(c *Config) { c.CloudBots = bots }); refused != "" {
		d.mu.Unlock()
		writeStatus(w)(http.StatusConflict, refused)
		return
	}
	d.config.CloudBots = bots
	d.config.save()
	d.cloud.configure(bots)
	d.changed()
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"bots": bots})
}
