//go:build !windows

package desktop

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// A fixed "now" for the 3 s a file must have been still, and a mtime that far back.
var botNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func botFolder(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func botLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skip("no symbolic links here:", err)
	}
}

// botReplyFile leaves a file in from-bot as the bot would, last changed ago before botNow.
func botReplyFile(t *testing.T, mailbox, name, text string, ago time.Duration) string {
	t.Helper()
	inbox := filepath.Join(mailbox, "from-bot")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, botNow.Add(-ago), botNow.Add(-ago)); err != nil {
		t.Fatal(err)
	}
	return path
}

func names(replies []botReply) []string {
	var out []string
	for _, r := range replies {
		out = append(out, r.Name)
	}
	return out
}

// MARK: The folder

func TestAMailboxCannotBeInOrHoldTheSupportFolderOrAWorktree(t *testing.T) {
	shortSupport(t)
	support, _ := filepath.EvalSymlinks(supportDir)
	tree := botFolder(t)
	inTree := filepath.Join(tree, "src")
	os.Mkdir(inTree, 0o755)
	inSupport := filepath.Join(support, "box")
	os.Mkdir(inSupport, 0o755)
	away := botFolder(t)
	linkToSupport, linkToTree := filepath.Join(away, "s"), filepath.Join(away, "w")
	botLink(t, support, linkToSupport)
	botLink(t, inTree, linkToTree)
	barred := []string{supportDir, tree}

	for name, path := range map[string]string{
		"the support folder": support, "inside it": inSupport, "holding it": filepath.Dir(support),
		"a worktree": tree, "inside one": inTree, "holding one": filepath.Dir(tree),
		"a link to the support folder": linkToSupport, "a link into a worktree": linkToTree,
		"a path that goes round": filepath.Join(away, "..", filepath.Base(away), "s"),
	} {
		p := botMailboxRefusal(path, barred)
		if p == nil || !p.Never || !strings.HasPrefix(p.Why, "The mailbox can't be in or hold the app's own folder or a hire's worktree: ") {
			t.Errorf("%s (%s): %+v", name, path, p)
		}
		if _, err := botMailboxChosen(path, barred); err == nil {
			t.Errorf("%s was chosen", name)
		}
	}
	// A worktree that is not there any more still bars its place, found through its parent's real name.
	gone := filepath.Join(away, "gone")
	if p := botMailboxRefusal(filepath.Join(linkToSupport, "x"), []string{gone}); p == nil || p.Never {
		t.Errorf("a folder that is not there: %+v", p)
	}
	fine := botFolder(t)
	if p := botMailboxRefusal(fine, barred); p != nil {
		t.Errorf("a folder of its own: %+v", p)
	}
	got, err := botMailboxChosen("  "+fine+"  ", barred)
	if err != nil || got != fine {
		t.Errorf("chosen %q %v, want %q", got, err, fine)
	}
}

func TestAMailboxThatIsNotAFullPathOrAFolderOrThereIsRefused(t *testing.T) {
	shortSupport(t)
	file := filepath.Join(botFolder(t), "f")
	os.WriteFile(file, nil, 0o644)
	for _, c := range []struct {
		path  string
		never bool
		text  string
	}{
		{"relative/box", true, "The mailbox folder has to be a full path."},
		{file, true, "The mailbox isn't a folder: " + file},
		{filepath.Join(botFolder(t), "later"), false, "The mailbox folder isn't there (is it synced?): "},
	} {
		p := botMailboxRefusal(c.path, nil)
		if p == nil || p.Never != c.never || !strings.HasPrefix(p.Why, c.text) {
			t.Errorf("%s: %+v, want never=%v %q", c.path, p, c.never, c.text)
		}
	}
}

// MARK: To the bot

func TestAMessageToABotIsAFileNamedByTimeAndIDAndWrittenOnce(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	at := time.Date(2026, 11, 1, 9, 5, 7, 0, time.FixedZone("x", 3600)) // 08:05:07 UTC
	commits := 0
	commit := func() bool { commits++; return true }
	if r := botMailboxDrop("hello bot", "m-1", at, box, []string{supportDir}, commit); r.Outcome != mailSent {
		t.Fatalf("drop: %+v", r)
	}
	file := filepath.Join(box, "to-bot", "20261101T080507Z-m-1.md")
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "hello bot\n" {
		t.Fatalf("file: %q %v", data, err)
	}
	if info, _ := os.Stat(file); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("file mode %v, want 0600", info.Mode().Perm())
	}
	// The same message again, later: delivered without a second file or a second count.
	if r := botMailboxDrop("hello bot", "m-1", at.Add(time.Hour), box, []string{supportDir}, commit); r.Outcome != mailSent {
		t.Errorf("again: %+v", r)
	}
	// Another id that only ends the same way is another message.
	if r := botMailboxDrop("other", "x-m-1", at, box, []string{supportDir}, commit); r.Outcome != mailSent {
		t.Errorf("other: %+v", r)
	}
	files, _ := os.ReadDir(filepath.Join(box, "to-bot"))
	var got []string
	for _, f := range files {
		got = append(got, f.Name())
	}
	if want := "[20261101T080507Z-m-1.md 20261101T080507Z-x-m-1.md]"; fmt.Sprint(got) != want {
		t.Errorf("files %v, want %s", got, want)
	}
	if commits != 2 {
		t.Errorf("counted %d times, want 2 (not for the repeat)", commits)
	}
}

func TestAMessageToABotIsNotWrittenIfTheCountCannotBe(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	r := botMailboxDrop("x", "m-1", botNow, box, nil, func() bool { return false })
	if r.Outcome != mailTrouble || r.Note != "The outbox couldn't be written, so nothing was put in the mailbox." {
		t.Errorf("result %+v", r)
	}
	if files, _ := os.ReadDir(filepath.Join(box, "to-bot")); len(files) != 0 {
		t.Errorf("a file was written: %v", files)
	}
}

func TestADropNeverGoesThroughALinkOrIntoABarredFolder(t *testing.T) {
	shortSupport(t)
	box, elsewhere := botFolder(t), botFolder(t)
	botLink(t, elsewhere, filepath.Join(box, "to-bot"))
	r := botMailboxDrop("x", "m-1", botNow, box, nil, func() bool { return true })
	if r.Outcome != mailCannot || r.Note != "to-bot in the mailbox isn't a folder, or couldn't be made." {
		t.Errorf("a linked to-bot: %+v", r)
	}
	if files, _ := os.ReadDir(elsewhere); len(files) != 0 {
		t.Errorf("a file went through the link: %v", files)
	}
	if r := botMailboxDrop("x", "m-1", botNow, supportDir, []string{supportDir}, func() bool { t.Error("counted"); return true }); r.Outcome != mailCannot {
		t.Errorf("into the support folder: %+v", r)
	}
	if r := botMailboxDrop("x", "m-1", botNow, filepath.Join(box, "later"), nil, func() bool { return true }); r.Outcome != mailTrouble {
		t.Errorf("a folder not synced yet: %+v", r)
	}
}

// MARK: From the bot

func TestRepliesAreReadOnceInOrderAndMovedToRead(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	botReplyFile(t, box, "b.md", "second\n", 20*time.Second)
	botReplyFile(t, box, "a.txt", "first", 30*time.Second)
	botReplyFile(t, box, "c.MD", "  third  ", 10*time.Second)
	botReplyFile(t, box, "same-1.md", "tie one", 5*time.Second)
	botReplyFile(t, box, "same-0.md", "tie zero", 5*time.Second)
	replies := botMailboxCollect(box, nil, 1000, botNow)
	if got, want := fmt.Sprint(names(replies)), "[a.txt b.md c.MD same-0.md same-1.md]"; got != want {
		t.Fatalf("replies %s, want %s", got, want)
	}
	if replies[0].Text != "first" || replies[1].Text != "second" || replies[2].Text != "third" {
		t.Errorf("texts %+v", replies)
	}
	inbox := filepath.Join(box, "from-bot")
	for _, name := range names(replies) {
		if _, err := os.Stat(filepath.Join(inbox, name)); err == nil {
			t.Errorf("%s is still in from-bot", name)
		}
		if _, err := os.Stat(filepath.Join(inbox, "read", name)); err != nil {
			t.Errorf("%s is not in read/: %v", name, err)
		}
	}
	if again := botMailboxCollect(box, nil, 1000, botNow); len(again) != 0 {
		t.Errorf("read twice: %+v", again)
	}
}

func TestAFileStillArrivingIsLeftUntilItHasBeenStillThreeSeconds(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	path := botReplyFile(t, box, "new.md", "hi", 0)
	os.Chtimes(path, botNow.Add(-2900*time.Millisecond), botNow.Add(-2900*time.Millisecond))
	if got := botMailboxCollect(box, nil, 1000, botNow); len(got) != 0 {
		t.Fatalf("taken after 2.9 s: %+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("a new file was moved: %v", err)
	}
	if got := botMailboxCollect(box, nil, 1000, botNow.Add(100*time.Millisecond)); len(got) != 1 {
		t.Errorf("not taken after 3 s: %+v", got)
	}
}

func TestOnlyPlainMarkdownAndTextFilesAreRead(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	outside := botFolder(t)
	secret := filepath.Join(outside, "secret.md")
	os.WriteFile(secret, []byte("not yours"), 0o644)
	os.Chtimes(secret, botNow.Add(-time.Hour), botNow.Add(-time.Hour))
	inbox := filepath.Join(box, "from-bot")
	botReplyFile(t, box, "ok.md", "fine", time.Minute)
	botReplyFile(t, box, "data.json", "{}", time.Minute)
	botReplyFile(t, box, "noext", "x", time.Minute)
	botReplyFile(t, box, ".hidden.md", "x", time.Minute)
	os.Mkdir(filepath.Join(inbox, "folder.md"), 0o755)
	botLink(t, secret, filepath.Join(inbox, "link.md"))
	botLink(t, outside, filepath.Join(inbox, "dir-link.txt"))
	got := botMailboxCollect(box, nil, 1000, botNow)
	if want := "[ok.md]"; fmt.Sprint(names(got)) != want {
		t.Fatalf("replies %v, want %s", names(got), want)
	}
	if data, err := os.ReadFile(secret); err != nil || string(data) != "not yours" {
		t.Errorf("a file behind a link was touched: %q %v", data, err)
	}
	for _, name := range []string{"link.md", "dir-link.txt", "data.json", "noext", ".hidden.md", "folder.md"} {
		if _, err := os.Lstat(filepath.Join(inbox, name)); err != nil {
			t.Errorf("%s was moved: %v", name, err)
		}
	}
}

func TestAFromBotFolderThatIsALinkIsNotFollowed(t *testing.T) {
	shortSupport(t)
	box, elsewhere := botFolder(t), botFolder(t)
	botReplyFile(t, elsewhere, "x.md", "y", 0) // makes elsewhere/from-bot
	real := filepath.Join(elsewhere, "from-bot")
	os.Chtimes(filepath.Join(real, "x.md"), botNow.Add(-time.Hour), botNow.Add(-time.Hour))
	botLink(t, real, filepath.Join(box, "from-bot"))
	if got := botMailboxCollect(box, nil, 1000, botNow); len(got) != 0 {
		t.Errorf("read through a link: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(real, "x.md")); err != nil {
		t.Errorf("the file behind the link moved: %v", err)
	}
}

func TestNoMoreThanTwentyRepliesAreTakenInALook(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	for i := 0; i < 25; i++ {
		botReplyFile(t, box, fmt.Sprintf("r%02d.md", i), fmt.Sprintf("n%d", i), time.Duration(100-i)*time.Second)
	}
	first := botMailboxCollect(box, nil, 1000, botNow)
	if len(first) != 20 || first[0].Name != "r00.md" || first[19].Name != "r19.md" {
		t.Fatalf("first look: %d replies %v", len(first), names(first))
	}
	second := botMailboxCollect(box, nil, 1000, botNow)
	if want := "[r20.md r21.md r22.md r23.md r24.md]"; fmt.Sprint(names(second)) != want {
		t.Errorf("second look %v, want %s", names(second), want)
	}
}

func TestAReplyLongerThanTheLimitIsCutAndStillMoved(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	botReplyFile(t, box, "long.md", "0123456789", time.Minute)
	botReplyFile(t, box, "exact.md", "abcde", time.Minute)
	got := botMailboxCollect(box, nil, 5, botNow)
	byName := map[string]string{}
	for _, r := range got {
		byName[r.Name] = r.Text
	}
	if byName["long.md"] != "01234\n[cut at 5 bytes]" || byName["exact.md"] != "abcde" {
		t.Errorf("texts %q", byName)
	}
	if _, err := os.Stat(filepath.Join(box, "from-bot", "read", "long.md")); err != nil {
		t.Errorf("not moved: %v", err)
	}
}

func TestAnEmptyReplyIsMovedButNotHandedOn(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	botReplyFile(t, box, "blank.md", " \n\t", time.Minute)
	if got := botMailboxCollect(box, nil, 100, botNow); len(got) != 0 {
		t.Errorf("replies %+v", got)
	}
	if _, err := os.Stat(filepath.Join(box, "from-bot", "read", "blank.md")); err != nil {
		t.Errorf("not moved: %v", err)
	}
}

func TestAFileWithTheNameOfOneAlreadyReadKeepsBoth(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	botReplyFile(t, box, "note.md", "one", time.Minute)
	botMailboxCollect(box, nil, 100, botNow)
	botReplyFile(t, box, "note.md", "two", time.Minute)
	if got := botMailboxCollect(box, nil, 100, botNow); len(got) != 1 || got[0].Text != "two" {
		t.Fatalf("second: %+v", got)
	}
	read, _ := os.ReadDir(filepath.Join(box, "from-bot", "read"))
	if len(read) != 2 {
		t.Errorf("read/ holds %d files, want 2", len(read))
	}
	if data, _ := os.ReadFile(filepath.Join(box, "from-bot", "read", "note.md")); string(data) != "one" {
		t.Errorf("the first was overwritten: %q", data)
	}
}

func TestAFileThatChangesWhileItIsReadIsLeftForTheNextLook(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	path := botReplyFile(t, box, "busy.md", "half", time.Minute)
	grow := func(p string) {
		f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
		f.WriteString(" and the rest")
		f.Close()
		os.Chtimes(p, botNow, botNow) // the bot is still writing
	}
	got := botMailboxCollectWith(box, nil, 1000, botNow, grow)
	if len(got) != 0 {
		t.Fatalf("a changing file was taken: %+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("a changing file was moved: %v", err)
	}
	if got := botMailboxCollect(box, nil, 1000, botNow.Add(time.Minute)); len(got) != 1 || got[0].Text != "half and the rest" {
		t.Errorf("next look: %+v", got)
	}
}

func TestAFileSwappedForALinkWhileItIsReadIsNotTaken(t *testing.T) {
	shortSupport(t)
	box, outside := botFolder(t), botFolder(t)
	secret := filepath.Join(outside, "secret.md")
	os.WriteFile(secret, []byte("not yours"), 0o644)
	path := botReplyFile(t, box, "swap.md", "mine", time.Minute)
	swap := func(p string) {
		os.Remove(p)
		botLink(t, secret, p)
	}
	if got := botMailboxCollectWith(box, nil, 1000, botNow, swap); len(got) != 0 {
		t.Errorf("taken: %+v", got)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Errorf("the file behind the link moved: %v", err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was moved: %v", err)
	}
}

func TestAFileSwappedForAPipeIsLeftAndDoesNotHangTheLook(t *testing.T) {
	shortSupport(t)
	box := botFolder(t)
	path := botReplyFile(t, box, "pipe.md", "mine", time.Minute)
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(path)
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() {
		_, ok := botReadSteady(path, before, 1000, nil)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Error("a pipe was taken as the file")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("opening a pipe with nobody writing held the look")
	}
}

func TestNothingIsReadFromABarredFolder(t *testing.T) {
	shortSupport(t)
	support, _ := filepath.EvalSymlinks(supportDir)
	botReplyFile(t, support, "x.md", "y", time.Minute)
	if got := botMailboxCollect(support, []string{supportDir}, 100, botNow); len(got) != 0 {
		t.Errorf("read from the support folder: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(support, "from-bot", "x.md")); err != nil {
		t.Errorf("moved: %v", err)
	}
}

// MARK: The routes and the deck

// botDeck is a mail deck with two bots, "grok-1" (Grokky) and "grok-2", set through the page's route.
func botDeck(t *testing.T) *mailDeck {
	t.Helper()
	m := newMailDeck(t)
	body := `{"bots":[{"id":"grok-1","name":"Grokky"},{"id":"grok-2","name":"Second"}]}`
	if w := m.call(t, asPage, "POST", "/setup/bots", body); w.Code != 200 {
		t.Fatalf("bots: %d %s", w.Code, w.Body)
	}
	return m
}

func (m *mailDeck) mailbox(t *testing.T, as who, id, folder string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"id": id, "folder": folder})
	return m.call(t, as, "POST", "/setup/bots/mailbox", string(b))
}

func (m *mailDeck) boxOf(id string) string {
	m.d.mu.Lock()
	defer m.d.mu.Unlock()
	for _, b := range m.d.cloud.bots {
		if b.ID == id {
			return b.Mailbox
		}
	}
	return "?"
}

func TestThePageChoosesABotsMailboxFolder(t *testing.T) {
	m := botDeck(t)
	box := botFolder(t)
	w := m.mailbox(t, asPage, "grok-1", box)
	if w.Code != 200 {
		t.Fatalf("choose: %d %s", w.Code, w.Body)
	}
	var answer struct{ Bots []CloudBot }
	json.Unmarshal(w.Body.Bytes(), &answer)
	if len(answer.Bots) != 2 || answer.Bots[0].Mailbox != box || answer.Bots[1].Mailbox != "" {
		t.Errorf("answer %s", w.Body)
	}
	if got := m.boxOf("grok-1"); got != box {
		t.Errorf("the desks know %q", got)
	}
	// It is kept in the config, and the setup list shows it.
	if saved := loadConfig().CloudBots; len(saved) != 2 || saved[0].Mailbox != box {
		t.Errorf("saved %+v", saved)
	}
	state := m.call(t, asPage, "GET", "/setup/state", "").Body.String()
	if !strings.Contains(state, `"mailbox":"`+strings.ReplaceAll(box, `\`, `\\`)+`"`) {
		t.Errorf("setup state does not show it: %s", state)
	}
	// POST /setup/bots, whatever it says, never sets or changes it.
	body := `{"bots":[{"id":"grok-1","name":"Renamed","mailbox":"/etc"},{"id":"grok-2","name":"Second","mailbox":"/tmp"}]}`
	if w := m.call(t, asPage, "POST", "/setup/bots", body); w.Code != 200 {
		t.Fatalf("bots: %d %s", w.Code, w.Body)
	}
	if m.boxOf("grok-1") != box || m.boxOf("grok-2") != "" {
		t.Errorf("the list changed the folders: %q %q", m.boxOf("grok-1"), m.boxOf("grok-2"))
	}
	// An empty folder takes it away; a blank one too.
	if w := m.mailbox(t, asPage, "grok-1", "   "); w.Code != 200 || m.boxOf("grok-1") != "" {
		t.Errorf("clear: %d %s %q", w.Code, w.Body, m.boxOf("grok-1"))
	}
}

func TestTheMailboxRouteKeepsTheMacsAnswersAndIsForThePageOnly(t *testing.T) {
	m := botDeck(t)
	const ask = "Send {id, folder}: the bot, and the folder its mail goes through (empty for none)."
	for name, body := range map[string]string{
		"nothing": ``, "no folder": `{"id":"grok-1"}`, "no id": `{"folder":"/x"}`, "a folder that is not text": `{"id":"grok-1","folder":3}`,
		"an id that is not text": `{"id":1,"folder":""}`, "no such bot": `{"id":"nobody","folder":""}`,
	} {
		if w := m.call(t, asPage, "POST", "/setup/bots/mailbox", body); w.Code != 400 || w.Body.String() != ask {
			t.Errorf("%s: %d %q", name, w.Code, w.Body)
		}
	}
	for text, folder := range map[string]string{
		"The mailbox folder has to be a full path.":                    "box",
		"The mailbox folder isn't there (is it synced?): ":             filepath.Join(botFolder(t), "later"),
		"The mailbox can't be in or hold the app's own folder or a hi": supportDir,
	} {
		if w := m.mailbox(t, asPage, "grok-1", folder); w.Code != 400 || !strings.HasPrefix(w.Body.String(), text) {
			t.Errorf("%q: %d %q", folder, w.Code, w.Body)
		}
	}
	if m.boxOf("grok-1") != "" {
		t.Errorf("a refused folder was kept: %q", m.boxOf("grok-1"))
	}
	box := botFolder(t)
	for name, as := range map[string]who{"the GM": asGM, "a tool": asTool, "a hire": asRev} {
		if w := m.mailbox(t, as, "grok-1", box); w.Code != 403 {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	if m.boxOf("grok-1") != "" {
		t.Errorf("someone else chose it: %q", m.boxOf("grok-1"))
	}
}

func TestAHiresWorktreeBarsAMailboxChosenOnThePage(t *testing.T) {
	m := botDeck(t)
	tree := botFolder(t)
	var file map[string]any
	data, _ := os.ReadFile(registryPath())
	json.Unmarshal(data, &file)
	file["hires"].([]any)[0].(map[string]any)["worktree"] = tree
	data, _ = json.Marshal(file)
	writeRegistryFile(registryPath(), data, 0o600)
	os.Mkdir(filepath.Join(tree, "in"), 0o755)
	for _, folder := range []string{tree, filepath.Join(tree, "in"), filepath.Dir(tree)} {
		if w := m.mailbox(t, asPage, "grok-1", folder); w.Code != 400 || !strings.Contains(w.Body.String(), "a hire's worktree") {
			t.Errorf("%s: %d %s", folder, w.Code, w.Body)
		}
	}
}

func (m *mailDeck) sendToBot(t *testing.T, as who, to, id string) (int, map[string]any) {
	t.Helper()
	w := m.send(t, as, fmt.Sprintf(`{"to":%q,"text":"Please look at the report.","id":%q}`, to, id))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAMessageToABotWithAMailboxIsDeliveredAsAFile(t *testing.T) {
	m := botDeck(t)
	box := botFolder(t)
	if w := m.mailbox(t, asPage, "grok-1", box); w.Code != 200 {
		t.Fatal(w.Body)
	}
	for i, to := range []string{"grok-1", "bot:grok-1", "Grokky", "grokky"} {
		id := fmt.Sprintf("m-%d", i)
		code, out := m.sendToBot(t, asGM, to, id)
		if code != 200 || out["state"] != "delivered" || out["to"] != "bot:grok-1" || out["id"] != id {
			t.Errorf("to %q: %d %v", to, code, out)
		}
	}
	files, _ := filepath.Glob(filepath.Join(box, "to-bot", "*.md"))
	if len(files) != 4 {
		t.Fatalf("files %v", files)
	}
	name := regexp.MustCompile(`^\d{8}T\d{6}Z-m-0\.md$`)
	var found bool
	for _, f := range files {
		if name.MatchString(filepath.Base(f)) {
			found = true
			data, _ := os.ReadFile(f)
			want := "— from the GM (Smolcorp), message m-0\n\nPlease look at the report.\n"
			if string(data) != want {
				t.Errorf("file text %q, want %q", data, want)
			}
		}
	}
	if !found {
		t.Errorf("no file named by time and id: %v", files)
	}
	// Sent again with the same id: nothing new, as the outbox says.
	if code, out := m.sendToBot(t, asGM, "grok-1", "m-0"); code != 200 || out["duplicate"] != true {
		t.Errorf("again: %d %v", code, out)
	}
	if again, _ := filepath.Glob(filepath.Join(box, "to-bot", "*.md")); len(again) != 4 {
		t.Errorf("a repeat made a file: %v", again)
	}
	rows := m.outbox(t, asGM, "")
	if len(rows) != 4 || rows[0]["state"] != "delivered" || rows[0]["to"] != "bot:grok-1" || rows[0]["toName"] != "Grokky" {
		t.Errorf("outbox %v", rows)
	}
	// The other bot has no folder, and a hire may not write to a bot at all.
	if code, out := m.sendToBot(t, asGM, "grok-2", "m-nobox"); code != 409 || out != nil {
		t.Errorf("a bot without a folder: %d %v", code, out)
	}
	if code, _ := m.sendToBot(t, asRev, "grok-1", "m-h"); code != 403 {
		t.Errorf("a hire to a bot: %d", code)
	}
}

func TestAMailboxThatIsNotThereYetKeepsTheMessageQueuedAndOneThatIsBarredRefusesIt(t *testing.T) {
	m := botDeck(t)
	box := botFolder(t)
	m.mailbox(t, asPage, "grok-1", box)
	// The folder goes away (a sync that is not there yet): the message waits, counted as a try.
	os.RemoveAll(box)
	code, out := m.sendToBot(t, asGM, "grok-1", "m-late")
	if code != 200 || out["state"] != "queued" || !strings.HasPrefix(fmt.Sprint(out["note"]), "The mailbox folder isn't there (is it synced?): ") {
		t.Errorf("not there yet: %d %v", code, out)
	}
	if msg, _ := m.d.mail.get("m-late"); msg.Attempts != 1 || msg.NextAt == 0 {
		t.Errorf("the try: %+v", msg)
	}
	// Later it is there; the pump (T728) will call mailDeliverBot for the queued message.
	os.MkdirAll(box, 0o755)
	msg, _ := m.d.mail.get("m-late")
	m.d.mailDeliverBot(msg)
	if after, _ := m.d.mail.get("m-late"); after.State != mailDelivered {
		t.Errorf("after: %+v", after)
	}
	if files, _ := filepath.Glob(filepath.Join(box, "to-bot", "*-m-late.md")); len(files) != 1 {
		t.Errorf("files %v", files)
	}
	// A mailbox put inside the support folder by hand in the config is refused at once, with the reason.
	m.d.mu.Lock()
	m.d.cloud.bots[0].Mailbox = supportDir
	m.d.mu.Unlock()
	code, _ = m.sendToBot(t, asGM, "grok-1", "m-barred")
	w := m.send(t, asGM, `{"to":"grok-1","text":"x","id":"m-barred2"}`)
	if code != 409 || !strings.HasPrefix(w.Body.String(), "The mailbox can't be in or hold the app's own folder or a hire's worktree: ") {
		t.Errorf("barred: %d %s", w.Code, w.Body)
	}
	if _, kept := m.d.mail.get("m-barred2"); kept {
		t.Error("a refused message was kept")
	}
}

func TestAQueuedMessageToABotThatIsGoneFails(t *testing.T) {
	m := botDeck(t)
	m.mailbox(t, asPage, "grok-1", botFolder(t))
	os.RemoveAll(m.boxOf("grok-1"))
	m.sendToBot(t, asGM, "grok-1", "m-gone")
	m.d.mu.Lock()
	m.d.cloud.bots = nil
	m.d.mu.Unlock()
	msg, _ := m.d.mail.get("m-gone")
	m.d.mailDeliverBot(msg)
	if after, _ := m.d.mail.get("m-gone"); after.State != mailFailed || after.Note != "Grokky is gone." {
		t.Errorf("after: %+v", after)
	}
}

func TestARepliesBecomeMailLinesInTheEventLog(t *testing.T) {
	m := botDeck(t)
	box, other := botFolder(t), botFolder(t)
	m.mailbox(t, asPage, "grok-1", box)
	m.mailbox(t, asPage, "grok-2", other)
	botReplyFile(t, box, "report.md", "All green.\nNothing to do.", time.Minute)
	botReplyFile(t, other, "n.txt", "from the second", time.Minute)
	botReplyFile(t, box, "too-new.md", "wait", 0)
	m.d.botMailboxLook(botNow)
	entries, _, _ := m.d.events.entries(0, 100)
	var mail []eventEntry
	for _, e := range entries {
		if e.Kind == eventKindMail {
			mail = append(mail, e)
		}
	}
	if len(mail) != 2 {
		t.Fatalf("mail lines: %+v", mail)
	}
	first := mail[0]
	if first.Detail != "Grokky wrote report.md: All green. Nothing to do." || first.Session != "cloud-grok-1" || first.Folder != "/cloud/Grokky" ||
		first.At != botNow.UnixMilli() || first.Agent != string(Grok) {
		t.Errorf("first: %+v", first)
	}
	if mail[1].Detail != "Second wrote n.txt: from the second" {
		t.Errorf("second: %+v", mail[1])
	}
	// Read once: another look says nothing, and the one that was too new comes later.
	m.d.botMailboxLook(botNow)
	m.d.botMailboxLook(botNow.Add(time.Minute))
	entries, _, _ = m.d.events.entries(0, 100)
	var count int
	for _, e := range entries {
		if e.Kind == eventKindMail {
			count++
		}
	}
	if count != 3 {
		t.Errorf("%d mail lines, want 3", count)
	}
}

func TestAFileNameWithControlCharactersIsOneLineInTheEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no tabs in file names here")
	}
	m := botDeck(t)
	box := botFolder(t)
	m.mailbox(t, asPage, "grok-1", box)
	botReplyFile(t, box, "a\tb\x1b.md", "hi", time.Minute)
	m.d.botMailboxLook(botNow)
	entries, _, _ := m.d.events.entries(0, 100)
	if len(entries) == 0 || entries[len(entries)-1].Detail != "Grokky wrote a b .md: hi" {
		t.Errorf("entries %+v", entries)
	}
}

func TestSmolcorpGMSendReachesABotThroughTheGoServer(t *testing.T) {
	m := botDeck(t)
	box := botFolder(t)
	m.mailbox(t, asPage, "grok-1", box)
	srv := httptest.NewServer(&router{deck: m.d})
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	vars := map[string]string{"SMOLCORP_PORT": port, "SMOLCORP_GM_TOKEN": m.gm}
	client, err := gm.NewSendClient(gm.Env{Getenv: func(k string) string { return vars[k] }, SupportDir: func() (string, error) { return t.TempDir(), nil }})
	if err != nil {
		t.Fatal(err)
	}
	reply, _, err := gm.Send(client, "bot:grok-1", "Please look at the report.", "gm-bot-1")
	if err != nil || reply.State != "delivered" || reply.To != "bot:grok-1" || reply.ID != "gm-bot-1" {
		t.Fatalf("send: %+v %v", reply, err)
	}
	files, _ := filepath.Glob(filepath.Join(box, "to-bot", "*-gm-bot-1.md"))
	if len(files) != 1 {
		t.Fatalf("files %v", files)
	}
	if data, _ := os.ReadFile(files[0]); !strings.HasSuffix(string(data), "message gm-bot-1\n\nPlease look at the report.\n") {
		t.Errorf("file %q", data)
	}
	if _, _, err := gm.Send(client, "bot:grok-2", "x", "gm-bot-2"); err == nil || !strings.Contains(err.Error(), "Second has no mailbox folder") {
		t.Errorf("a bot without a folder: %v", err)
	}
}
