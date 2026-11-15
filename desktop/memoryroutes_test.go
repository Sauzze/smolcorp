package desktop

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// MEM4 (PL126M): the GM's memory routes. The GM reads the kept notes behind the core's header, proposes a note that
// waits for the owner's tap, and withdraws a proposal; nothing it can call keeps, changes or deletes a kept note. A
// hire, a tool, an MCP key and a night job are refused, and no gate reads memory.

// The canary: a kept note whose words must reach the GM only through GET /setup/gm/memory.
var canaryNote = memoryEntry{Name: "Pelican release notes", Description: "Where the release notes are kept", Type: "reference",
	Body: "The release notes for the pelican app are in the docs folder."}

// memoryDeck is a test deck with the manager on, the GM's token, and a hire's key.
func memoryDeck(t *testing.T) (d *Deck, gm, hireKey string) {
	t.Helper()
	d = newTestDeck(t)
	managerOn(d)
	gm, hireKey = withGMAndAHire(t, d)
	return d, gm, hireKey
}

// memoryAsk is a call to a memory route as who, with the status and the answer's text.
func memoryAsk(t *testing.T, d *Deck, who caller, gm, hireKey, method, target, body string) (int, string) {
	t.Helper()
	c := asCaller(who, gm, hireKey)
	c.method, c.target, c.host, c.from, c.body = method, target, "localhost:7710", thisComputer, body
	if method != http.MethodGet {
		c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
	}
	w := serve(t, d, c)
	return w.Code, w.Body.String()
}

// readAsGM is GET /setup/gm/memory (with ?name= when name isn't "") as the GM, decoded.
func readAsGM(t *testing.T, d *Deck, gm, query string) (int, memoryAnswer, string) {
	t.Helper()
	code, text := memoryAsk(t, d, callerGM, gm, "", "GET", memoryReadPath+query, "")
	var answer memoryAnswer
	_ = json.Unmarshal([]byte(text), &answer)
	return code, answer, text
}

func proposal(fields map[string]string) string {
	data, _ := json.Marshal(fields)
	return string(data)
}

// The read: the header first, then the kept notes alone, each checked again as it's read. A waiting note is never in
// it; a kept note that fails today's checks is withheld and only counted; with a name, that note whole.
func TestTheGMReadsItsKeptNotesBehindTheHeaderAlone(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	code, answer, text := readAsGM(t, d, gm, "")
	if code != http.StatusOK || !strings.HasPrefix(text, `{"header":`+jsonString(memoryHeader)) || len(answer.Notes) != 0 || answer.Withheld != 0 {
		t.Fatalf("an empty memory: %d %s", code, text)
	}
	kept := keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	failing := keptNote(t, d, named("Fails today"))
	// Today's lists refuse what it holds now: an e-mail address is personal data.
	d.memory.mu.Lock()
	for i := range d.memory.notes {
		if d.memory.notes[i].ID == failing.ID {
			d.memory.notes[i].Body = "Write to ada@example.com before each release."
		}
	}
	d.memory.mu.Unlock()

	code, answer, text = readAsGM(t, d, gm, "")
	want := []memoryNoteAnswer{{Name: kept.Name, Type: kept.Type, Description: kept.Description}}
	if code != http.StatusOK || !strings.HasPrefix(text, `{"header":`+jsonString(memoryHeader)) || !slices.Equal(answer.Notes, want) || answer.Withheld != 1 {
		t.Errorf("the index: %d %s", code, text)
	}
	for _, never := range []string{waiting.Name, "Fails today", "ada@example.com", canaryNote.Body} {
		if strings.Contains(text, never) {
			t.Errorf("the index holds %q: %s", never, text)
		}
	}
	code, answer, text = readAsGM(t, d, gm, "?name=Pelican+release+notes")
	if code != http.StatusOK || len(answer.Notes) != 1 || answer.Notes[0].Body != canaryNote.Body || answer.Header != memoryHeader {
		t.Errorf("one note whole: %d %s", code, text)
	}
	for _, name := range []string{waiting.Name, "Fails today", "Nothing of the sort"} {
		if code, _, text := readAsGM(t, d, gm, "?name="+strings.ReplaceAll(name, " ", "+")); code != http.StatusNotFound || strings.Contains(text, "ada@") {
			t.Errorf("%q whole: %d %s", name, code, text)
		}
	}
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// A proposal: its words' source is the core's to say. A rule or a fact about the owner quotes the owner's own words
// from the ledger, which gives the tag; what the GM saw is only for a fact; the GM can't name a source or a state.
// It waits, never memory until the owner's tap, and a note about a gate says it's held for a tighten-only tap.
func TestAProposalQuotesTheOwnersWordsAndWaits(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	d.owner.submitted("gm1", "Please always run the tests with pnpm in this office.")
	d.owner.submitted("gm1", "Always ask me before you deploy anything to the store.")
	rule := map[string]string{"name": "Tests run with pnpm", "description": "Which tool runs the tests", "type": "feedback",
		"rule": "Run the tests with pnpm", "why": "The owner said so", "how": "Every time the tests run"}
	with := func(extra map[string]string) string {
		fields := map[string]string{}
		for k, v := range rule {
			fields[k] = v
		}
		for k, v := range extra {
			if v == "" {
				delete(fields, k)
			} else {
				fields[k] = v
			}
		}
		return proposal(fields)
	}
	propose := func(body string) (int, string) {
		return memoryAsk(t, d, callerGM, gm, "", "POST", memoryProposePath, body)
	}
	for what, row := range map[string]struct {
		body   string
		status int
		said   string
	}{
		"words the owner never said": {with(map[string]string{"quote": "run the tests with yarn in this office"}), http.StatusBadRequest, memoryNoQuote},
		"too short a quote":          {with(map[string]string{"quote": "pnpm"}), http.StatusBadRequest, memoryNoQuote},
		"what the GM saw, for a rule": {with(map[string]string{"seen": "package.json:12"}), http.StatusBadRequest,
			"a feedback note needs the owner's own words"},
		"both sources":            {with(map[string]string{"quote": "run the tests with pnpm", "seen": "package.json:12"}), http.StatusBadRequest, "not both"},
		"no source":               {with(nil), http.StatusBadRequest, "Say where its words came from"},
		"a tag named by the GM":   {with(map[string]string{"quote": "run the tests with pnpm", "tag": "owner"}), http.StatusBadRequest, "Send the note as JSON"},
		"a state named by the GM": {with(map[string]string{"quote": "run the tests with pnpm", "state": "kept"}), http.StatusBadRequest, "Send the note as JSON"},
		"a key in its body": {with(map[string]string{"quote": "run the tests with pnpm", "type": "project", "body": "Deploys use sk-" + "ant-fakefakefake",
			"rule": "", "why": "", "how": ""}), http.StatusBadRequest, "Memory can't keep this note"},
		"two notes":  {with(map[string]string{"quote": "run the tests with pnpm"}) + "{}", http.StatusBadRequest, "Send the note as JSON"},
		"not a note": {"[]", http.StatusBadRequest, "Send the note as JSON"},
	} {
		if code, text := propose(row.body); code != row.status || !strings.Contains(text, row.said) {
			t.Errorf("%s: %d %q, want %d and %q", what, code, text, row.status, row.said)
		}
	}
	if all := d.memory.all(); len(all) != 0 {
		t.Fatalf("a refused proposal was kept: %+v", all)
	}

	code, text := propose(with(map[string]string{"quote": "always run the tests with pnpm in this office"}))
	var got map[string]string
	_ = json.Unmarshal([]byte(text), &got)
	all := d.memory.all()
	if code != http.StatusOK || got["state"] != memoryWaiting || got["from"] != memoryFromTerminal || got["held"] != "" || len(all) != 1 ||
		all[0].State != memoryWaiting || all[0].Source != (memorySource{Tag: memoryFromTerminal, Quote: "Please always run the tests with pnpm in this office.",
		QuoteStart: 7, QuoteEnd: 52}) {
		t.Fatalf("a rule in the owner's words: %d %s, store %+v", code, text, all)
	}
	if _, answer, _ := readAsGM(t, d, gm, ""); len(answer.Notes) != 0 {
		t.Errorf("a proposal is memory before the owner's tap: %+v", answer.Notes)
	}
	if code, text := propose(with(map[string]string{"quote": "always run the tests with pnpm in this office"})); code != http.StatusConflict {
		t.Errorf("the same name again: %d %s", code, text)
	}
	// Two proposals of one name at once: one note, the others refused (RT126MEM4 N3, the name looked for and the
	// note proposed under one lock).
	twin := with(map[string]string{"name": "Twin note", "quote": "always run the tests with pnpm in this office"})
	codes := make(chan int, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			code, _ := propose(twin)
			codes <- code
		})
	}
	wg.Wait()
	close(codes)
	took := 0
	for code := range codes {
		if code == http.StatusOK {
			took++
		} else if code != http.StatusConflict {
			t.Errorf("a twin proposal: %d", code)
		}
	}
	if twins := slices.DeleteFunc(d.memory.all(), func(n gmNote) bool { return n.Name != "Twin note" }); took != 1 || len(twins) != 1 {
		t.Errorf("eight proposals of one name at once: %d taken, %d kept", took, len(twins))
	}
	// A fact may say what the GM saw; it still waits.
	fact := proposal(map[string]string{"name": "Docs folder", "description": "Where the docs are", "type": "reference",
		"body": "The user guide is in the docs folder.", "seen": "ls docs"})
	if code, text := propose(fact); code != http.StatusOK || !strings.Contains(text, `"from":"seen"`) {
		t.Errorf("a fact the GM saw: %d %s", code, text)
	}
	// A rule about a gate waits for the owner's tighten-only tap, and says so.
	gate := with(map[string]string{"name": "Ask before deploying", "rule": "Always ask the owner before you deploy anything",
		"quote": "Always ask me before you deploy anything"})
	code, text = propose(gate)
	_ = json.Unmarshal([]byte(text), &got)
	if code != http.StatusOK || got["held"] == "" || got["state"] != memoryWaiting {
		t.Errorf("a rule about a gate: %d %s", code, text)
	}
}

// A quote is only a pointer into the owner's sentence (SIERRA's rule for RT126MEM4 B1, the reviewer's probe among
// the rows): what's kept is the whole sentence, verbatim, and where the quote sits in it. A quote that leaves out a
// negation of its sentence, or cuts one in two, is refused as the owner's words and held for their tap, whose card
// shows the sentence whole; one that holds the whole negation stands. In English and in French. After a negation
// anywhere before the quote, what's kept is the paragraphs from it to the quote's, and the note is held whatever the
// sentences say (SIERRA's backstop, FXMEM4R4).
func TestAProposalKeepsTheOwnersWholeSentence(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	type sentenceRow struct {
		what, line, quote string
		status            int
		sentence, dropped string // the sentence kept, and the negation the hold names ("" for none)
	}
	rows := []sentenceRow{
		{"a cut that drops never (the reviewer's probe)", "Never use npm in this office, use pnpm.", "use npm in this office",
			http.StatusAccepted, "Never use npm in this office, use pnpm.", "Never"},
		{"a cut that keeps the whole never", "Never use npm in this office, use pnpm.", "Never use npm in this office",
			http.StatusOK, "Never use npm in this office, use pnpm.", ""},
		{"never outside the cut", "Use npm for the web app, never for the API.", "Use npm for the web app",
			http.StatusAccepted, "Use npm for the web app, never for the API.", "never"},
		{"do not, cut", "Do not run the build script on Fridays.", "run the build script on Fridays",
			http.StatusAccepted, "Do not run the build script on Fridays.", "not"},
		{"don't split at its apostrophe", "Don't use the staging database for tests.", "t use the staging database",
			http.StatusAccepted, "Don't use the staging database for tests.", "Don't"},
		{"won't, cut", "We won't keep the old changelog format.", "keep the old changelog format",
			http.StatusAccepted, "We won't keep the old changelog format.", "won't"},
		{"can't typed without its apostrophe", "You cant edit the shared lint config alone.", "edit the shared lint config alone",
			http.StatusAccepted, "You cant edit the shared lint config alone.", "cant"},
		{"no, cut", "No comments in the generated files, ever.", "comments in the generated files",
			http.StatusAccepted, "No comments in the generated files, ever.", "No"},
		{"an abbreviation ends no sentence", "Do not use e.g. yarn for the web app.", "yarn for the web app",
			http.StatusAccepted, "Do not use e.g. yarn for the web app.", "not"},
		// A negation in the sentence before was left out until SIERRA's backstop (FXMEM4R4): it's before the quote, so the
		// note is held with the whole line. The cheap error.
		{"a negation in another sentence", "Never use yarn. Use bun for the scripts folder.", "Use bun for the scripts folder",
			http.StatusAccepted, "Never use yarn. Use bun for the scripts folder.", "Never"},
		{"spaces kept as typed", "Always   use pnpm   for the docs site.", "use pnpm for the docs site",
			http.StatusOK, "Always   use pnpm   for the docs site.", ""},
		{"jamais, coupé", "N'utilise jamais npm dans ce bureau, prends pnpm.", "npm dans ce bureau",
			http.StatusAccepted, "N'utilise jamais npm dans ce bureau, prends pnpm.", "N' … jamais"},
		{"n' coupé de son jamais", "N’utilise jamais yarn pour le site, prends pnpm.", "utilise jamais yarn pour le site",
			http.StatusAccepted, "N’utilise jamais yarn pour le site, prends pnpm.", "N’ … jamais"},
		{"la négation entière", "N'utilise jamais bun dans la recette, prends pnpm.", "N'utilise jamais bun dans la recette",
			http.StatusOK, "N'utilise jamais bun dans la recette, prends pnpm.", ""},
		{"ne gardé, pas coupé", "Je ne veux vraiment pas que tu lances les tests avec npm.", "Je ne veux vraiment",
			http.StatusAccepted, "Je ne veux vraiment pas que tu lances les tests avec npm.", "ne … pas"},
		{"ne … plus", "Ne relance plus la recette le vendredi soir.", "la recette le vendredi soir",
			http.StatusAccepted, "Ne relance plus la recette le vendredi soir.", "Ne … plus"},
		{"ne … rien", "Ne change rien au fichier de configuration.", "au fichier de configuration",
			http.StatusAccepted, "Ne change rien au fichier de configuration.", "Ne … rien"},
		{"aucun hors de la citation", "Lance les tests avec bun sous aucun prétexte.", "Lance les tests avec bun",
			http.StatusAccepted, "Lance les tests avec bun sous aucun prétexte.", "aucun"},
		{"jamais hors de la citation", "Garde les notes de version courtes, jamais plus d'une page.", "Garde les notes de version courtes",
			http.StatusAccepted, "Garde les notes de version courtes, jamais plus d'une page.", "jamais"},
		// RT126MEM4R2 B1: a period after a title or an initial ends no sentence, so the negation before it is kept and
		// read. The reviewer's four probes, and the control row (no period after the title).
		{"Mr. ends no sentence", "Never send the release notes to Mr. Dupont before my review.", "Dupont before my review",
			http.StatusAccepted, "Never send the release notes to Mr. Dupont before my review.", "Never"},
		{"U.S. ends no sentence", "Don't use the U.S. East region for the staging database.", "East region for the staging database",
			http.StatusAccepted, "Don't use the U.S. East region for the staging database.", "Don't"},
		{"Dr. ends no sentence", "Do not merge Dr. Lee's branch without the tests.", "Lee's branch without the tests",
			http.StatusAccepted, "Do not merge Dr. Lee's branch without the tests.", "not"},
		{"M. ne termine aucune phrase", "N'envoie jamais les notes de version à M. Dupont avant ma relecture.", "Dupont avant ma relecture",
			http.StatusAccepted, "N'envoie jamais les notes de version à M. Dupont avant ma relecture.", "N' … jamais"},
		{"Mme sans point", "Ne merge jamais la branche de Mme Martin sans les tests.", "Martin sans les tests",
			http.StatusAccepted, "Ne merge jamais la branche de Mme Martin sans les tests.", "Ne … jamais"},
		// F1: a sentence after the quote's that opens on a negation answers it, and is kept and read with it. One that
		// doesn't is left out, as before.
		{"a no in the next sentence", "Use npm in the web folder? No, never.", "Use npm in the web folder",
			http.StatusAccepted, "Use npm in the web folder? No, never.", "No"},
		{"un non dans la phrase d'après", "Utilise npm dans le dossier web ? Non, jamais.", "Utilise npm dans le dossier web",
			http.StatusAccepted, "Utilise npm dans le dossier web ? Non, jamais.", "Non"},
		{"a next sentence with no negation", "Use bun for the scripts folder. Keep yarn for the docs.", "Use bun for the scripts folder",
			http.StatusOK, "Use bun for the scripts folder.", ""},
		{"non- starts a word, not a negation", "Run the non-blocking checks first, then the slow ones.", "then the slow ones",
			http.StatusOK, "Run the non-blocking checks first, then the slow ones.", ""},
		// F2: ni … ni, alone or after a ne.
		{"ni hors de la citation", "Ni npm ni yarn dans le dossier web, seulement pnpm.", "yarn dans le dossier web",
			http.StatusAccepted, "Ni npm ni yarn dans le dossier web, seulement pnpm.", "Ni"},
		{"ne … ni, coupé", "Je ne veux ni npm ni yarn dans la recette.", "yarn dans la recette",
			http.StatusAccepted, "Je ne veux ni npm ni yarn dans la recette.", "ne … ni"},
		// RT126MEM4R3 B1: an ellipsis ends no sentence, before a name or a capital, … or periods, spaced or not. The
		// reviewer's three probes, the lower-case control, and the cost: a negation before one is read, the tap's
		// cheap error.
		{"... then a name", "Never push to... GitHub directly from a hire's desk.", "GitHub directly from a hire's desk",
			http.StatusAccepted, "Never push to... GitHub directly from a hire's desk.", "Never"},
		{"… puis un nom", "Ne pousse jamais sur… GitHub sans revue.", "GitHub sans revue",
			http.StatusAccepted, "Ne pousse jamais sur… GitHub sans revue.", "Ne … jamais"},
		{"... then a verb", "Never... Push to main on a Friday.", "Push to main on a Friday",
			http.StatusAccepted, "Never... Push to main on a Friday.", "Never"},
		{"... then lower case", "Never... push to main on a Friday.", "push to main on a Friday",
			http.StatusAccepted, "Never... push to main on a Friday.", "Never"},
		{". . . spaced", "Never push to . . . GitHub without a review.", "GitHub without a review",
			http.StatusAccepted, "Never push to . . . GitHub without a review.", "Never"},
		{"an ellipsis runs on (the cheap error)", "Never use yarn... Use bun for the build folder.", "Use bun for the build folder",
			http.StatusAccepted, "Never use yarn... Use bun for the build folder.", "Never"},
		// B2: a list typed on one line. A period after a number, or before one, ends no sentence, so each item keeps
		// the negated lead-in. The reviewer's three probes, the second item, and the cost: a version runs on.
		{"a numbered list", "Never do these: 1. Push to main. 2. Skip the tests.", "Push to main",
			http.StatusAccepted, "Never do these: 1. Push to main. 2. Skip the tests.", "Never"},
		{"a numbered list's second item", "Never do these: 1. Merge to main. 2. Skip the linter.", "Skip the linter",
			http.StatusAccepted, "Never do these: 1. Merge to main. 2. Skip the linter.", "Never"},
		{"a numbered list, no periods after the items", "Never do these: 1. Push to main 2. Skip the tests", "Push to main",
			http.StatusAccepted, "Never do these: 1. Push to main 2. Skip the tests", "Never"},
		{"une liste numérotée", "Ne fais jamais ça : 1. Pousser sur main 2. Sauter les tests.", "Pousser sur main",
			http.StatusAccepted, "Ne fais jamais ça : 1. Pousser sur main 2. Sauter les tests.", "Ne … jamais"},
		{"a number runs on", "Stay on Node 18. Use pnpm for the web app.", "Use pnpm for the web app",
			http.StatusOK, "Stay on Node 18. Use pnpm for the web app.", ""},
		// F1': firms, addresses and months, French and English.
		{"Sté. ne termine aucune phrase", "N'envoie jamais la facture à la Sté. Durand sans mon accord.", "Durand sans mon accord",
			http.StatusAccepted, "N'envoie jamais la facture à la Sté. Durand sans mon accord.", "N' … jamais"},
		{"Cie. ne termine aucune phrase", "N'envoie jamais le devis à Durand et Cie. Avant ma relecture.", "Avant ma relecture",
			http.StatusAccepted, "N'envoie jamais le devis à Durand et Cie. Avant ma relecture.", "N' … jamais"},
		{"Jan. ends no sentence", "Never merge the API branch before Jan. Fifteen without my review.", "Fifteen without my review",
			http.StatusAccepted, "Never merge the API branch before Jan. Fifteen without my review.", "Never"},
		// RT126MEM4R4 B1: a bracket starts no sentence, so a list's second item keeps the negated lead-in. The reviewer's
		// EN, lower-case and FR probes, (a)/(b) and (i)/(ii), and a negation after a bracket, which only the sentence
		// keeps (no negation before the quote). And the cost: an aside in brackets runs on.
		{"(1) (2), the second item", "Never do these: (1) Push to main. (2) Skip the tests.", "Skip the tests",
			http.StatusAccepted, "Never do these: (1) Push to main. (2) Skip the tests.", "Never"},
		{"(1) (2) in lower case", "Never do these: (1) push to main. (2) skip the tests.", "skip the tests",
			http.StatusAccepted, "Never do these: (1) push to main. (2) skip the tests.", "Never"},
		{"(1) (2), le deuxième point", "Ne fais jamais ça : (1) Pousser sur main. (2) Sauter les tests.", "Sauter les tests",
			http.StatusAccepted, "Ne fais jamais ça : (1) Pousser sur main. (2) Sauter les tests.", "Ne … jamais"},
		{"(a) (b)", "Never do these: (a) Push to main. (b) Skip the tests.", "Skip the tests",
			http.StatusAccepted, "Never do these: (a) Push to main. (b) Skip the tests.", "Never"},
		{"(i) (ii)", "Never do these: (i) Push to main. (ii) Skip the tests.", "Skip the tests",
			http.StatusAccepted, "Never do these: (i) Push to main. (ii) Skip the tests.", "Never"},
		{"a negation in brackets after", "Push to the mirror. (Hires never push to GitHub.)", "Push to the mirror",
			http.StatusAccepted, "Push to the mirror. (Hires never push to GitHub.)", "never"},
		{"an aside in brackets runs on (the cheap error)", "Use pnpm for the web app. (Yarn is slow.) Never use npm.", "Use pnpm for the web app",
			http.StatusAccepted, "Use pnpm for the web app. (Yarn is slow.) Never use npm.", "Never"},
		// M-R4a: two periods are an ellipsis. The reviewer's row, the French, and a negation after one, which only the
		// sentence keeps.
		{".. then a name", "Never push to.. GitHub directly from a hire's desk.", "GitHub directly from a hire's desk",
			http.StatusAccepted, "Never push to.. GitHub directly from a hire's desk.", "Never"},
		{".. puis un nom", "Ne pousse jamais sur.. GitHub sans revue.", "GitHub sans revue",
			http.StatusAccepted, "Ne pousse jamais sur.. GitHub sans revue.", "Ne … jamais"},
		{".. then a negation", "Push to the mirror.. GitHub is not for hires.", "Push to the mirror",
			http.StatusAccepted, "Push to the mirror.. GitHub is not for hires.", "not"},
		// F1'': the periods after one that ends nothing (after a space, an initial) end nothing either.
		{"a space before ..", "Never push to .. GitHub without a review.", "GitHub without a review",
			http.StatusAccepted, "Never push to .. GitHub without a review.", "Never"},
		{"an initial before ..", "Never push to the M.. GitHub mirror on a Friday.", "GitHub mirror on a Friday",
			http.StatusAccepted, "Never push to the M.. GitHub mirror on a Friday.", "Never"},
		{"a space before .., then a negation", "Push to .. GitHub mirror, not to main.", "Push to .. GitHub mirror",
			http.StatusAccepted, "Push to .. GitHub mirror, not to main.", "not"},
		{"an initial before .., then a negation", "Push to the M.. GitHub is not for hires.", "Push to the M",
			http.StatusAccepted, "Push to the M.. GitHub is not for hires.", "not"},
		// SIERRA's backstop (FXMEM4R4): a negation anywhere before the quote holds the note, with the paragraphs from it
		// to the quote's shown whole. A bullet list under a heading, with or without a blank line; a negation two lines
		// up; the nearest negation's paragraph is where what's kept starts, and the quote's paragraph where it ends.
		{"a bullet list under a never heading", "Never on a Friday:\n- Push to main.\n- Skip the tests.", "Skip the tests",
			http.StatusAccepted, "Never on a Friday:\n- Push to main.\n- Skip the tests.", "Never"},
		{"a never heading, then a blank line", "Never on a Friday\n\n- Push to main.\n- Skip the tests.", "Skip the tests",
			http.StatusAccepted, "Never on a Friday\n\n- Push to main.\n- Skip the tests.", "Never"},
		{"a negation two lines up", "Don't touch the release branch.\nThe docs are fine as they are.\nPush the changelog to main.",
			"Push the changelog to main", http.StatusAccepted,
			"Don't touch the release branch.\nThe docs are fine as they are.\nPush the changelog to main.", "Don't"},
		{"une liste sous un titre jamais", "Jamais le vendredi :\n- Pousser sur main.\n- Sauter les tests.", "Sauter les tests",
			http.StatusAccepted, "Jamais le vendredi :\n- Pousser sur main.\n- Sauter les tests.", "Jamais"},
		{"un titre jamais, puis une ligne vide", "Jamais le vendredi\n\n- Pousser sur main.\n- Sauter les tests.", "Sauter les tests",
			http.StatusAccepted, "Jamais le vendredi\n\n- Pousser sur main.\n- Sauter les tests.", "Jamais"},
		{"une négation deux lignes plus haut", "Ne touche pas à la branche de release.\nLa doc est bien comme ça.\nPousse le changelog sur main.",
			"Pousse le changelog sur main", http.StatusAccepted,
			"Ne touche pas à la branche de release.\nLa doc est bien comme ça.\nPousse le changelog sur main.", "Ne … pas"},
		{"a negation in an earlier paragraph", "Never deploy on a Friday.\n\nUse pnpm for the web app.", "Use pnpm for the web app",
			http.StatusAccepted, "Never deploy on a Friday.\n\nUse pnpm for the web app.", "Never"},
		{"from the nearest negation's paragraph", "Never deploy on a Friday.\n\nThe docs are fine.\n\nDon't use yarn.\nUse pnpm for the docs site.",
			"Use pnpm for the docs site", http.StatusAccepted, "Don't use yarn.\nUse pnpm for the docs site.", "Don't"},
		{"to the end of the quote's paragraph", "Don't use yarn.\nUse pnpm for the blog site.\n\nThe docs are fine.", "Use pnpm for the blog site",
			http.StatusAccepted, "Don't use yarn.\nUse pnpm for the blog site.", "Don't"},
		{"to the end of the quote's paragraph, past its sentence", "Don't use yarn.\nUse pnpm for the wiki pages.\nKeep the docs short.",
			"Use pnpm for the wiki pages", http.StatusAccepted, "Don't use yarn.\nUse pnpm for the wiki pages.\nKeep the docs short.", "Don't"},
		{"the A1, then the B2 (the reviewer's control, now held)", "Never ship the A1. Use the B2 build instead.", "Use the B2 build instead",
			http.StatusAccepted, "Never ship the A1. Use the B2 build instead.", "Never"},
		{"a negation after, in a sentence of its own", "Use pnpm for the admin site.\n\nDeploys are not for Fridays.", "Use pnpm for the admin site",
			http.StatusOK, "Use pnpm for the admin site.", ""},
		{"a line break kept as typed (RT126MEM4R2 F3)", "Never\n\nuse bun in this office", "use bun in this office",
			http.StatusAccepted, "Never\n\nuse bun in this office", "Never"},
		// The ruling's words: avoid, stop, évite, arrête, and pas de.
		{"avoid", "Avoid these on a Friday: (1) Deploys. (2) Database migrations.", "Database migrations",
			http.StatusAccepted, "Avoid these on a Friday: (1) Deploys. (2) Database migrations.", "Avoid"},
		{"stop", "Stop doing this:\n- Merging without a review.\n- Pushing to main.", "Pushing to main",
			http.StatusAccepted, "Stop doing this:\n- Merging without a review.\n- Pushing to main.", "Stop"},
		{"évite", "Évite ça le vendredi : (1) Les mises en prod. (2) Les migrations.", "Les migrations",
			http.StatusAccepted, "Évite ça le vendredi : (1) Les mises en prod. (2) Les migrations.", "Évite"},
		{"arrête", "Arrête de faire ça :\n- Fusionner sans relecture.\n- Pousser sur main.", "Pousser sur main",
			http.StatusAccepted, "Arrête de faire ça :\n- Fusionner sans relecture.\n- Pousser sur main.", "Arrête"},
		{"pas de", "Pas de ça le vendredi : (1) Les mises en prod. (2) Les migrations de base.", "Les migrations de base",
			http.StatusAccepted, "Pas de ça le vendredi : (1) Les mises en prod. (2) Les migrations de base.", "Pas"},
		// RT126MEM4R5 B1: a negation in Markdown's underscore emphasis is a negation; a word is letters and digits only.
		// The reviewer's five probes.
		{"_Never_, one sentence", "_Never_ push to main on a Friday.", "push to main on a Friday",
			http.StatusAccepted, "_Never_ push to main on a Friday.", "Never"},
		{"_never_ mid-sentence", "You should _never_ push to main on a Friday.", "push to main on a Friday",
			http.StatusAccepted, "You should _never_ push to main on a Friday.", "never"},
		{"a __Never__ heading over a list", "__Never__ on a Friday:\n- Push to main.\n- Skip the tests.", "Skip the tests",
			http.StatusAccepted, "__Never__ on a Friday:\n- Push to main.\n- Skip the tests.", "Never"},
		{"_Don't_", "_Don't_ push to main on a Friday.", "push to main on a Friday",
			http.StatusAccepted, "_Don't_ push to main on a Friday.", "Don't"},
		{"une liste après _Jamais_", "_Jamais_ le vendredi : (1) Pousser sur main. (2) Sauter les tests.", "Sauter les tests",
			http.StatusAccepted, "_Jamais_ le vendredi : (1) Pousser sur main. (2) Sauter les tests.", "Jamais"},
		{"_Always_, the control", "_Always_ use pnpm for the web app.", "use pnpm for the web app",
			http.StatusOK, "_Always_ use pnpm for the web app.", ""},
	}
	// RT126MEM4R4's sweep: every one-line list or bullet spelling after a negated lead-in, in English and in French,
	// the second item quoted. Each is held, with the whole line kept.
	for _, list := range []struct{ spelling, english, french string }{
		{"(1)", "Never do these: (1) Push to main. (2) Skip the tests.", "Ne fais jamais ça : (1) Pousser sur main. (2) Sauter les tests."},
		{"1)", "Never do these: 1) Push to main. 2) Skip the tests.", "Ne fais jamais ça : 1) Pousser sur main. 2) Sauter les tests."},
		{"[1]", "Never do these: [1] Push to main. [2] Skip the tests.", "Ne fais jamais ça : [1] Pousser sur main. [2] Sauter les tests."},
		{"1.", "Never do these: 1. Push to main. 2. Skip the tests.", "Ne fais jamais ça : 1. Pousser sur main. 2. Sauter les tests."},
		{"a)", "Never do these: a) Push to main. b) Skip the tests.", "Ne fais jamais ça : a) Pousser sur main. b) Sauter les tests."},
		{"A)", "Never do these: A) Push to main. B) Skip the tests.", "Ne fais jamais ça : A) Pousser sur main. B) Sauter les tests."},
		{"a.", "Never do these: a. Push to main. b. Skip the tests.", "Ne fais jamais ça : a. Pousser sur main. b. Sauter les tests."},
		{"A.", "Never do these: A. Push to main. B. Skip the tests.", "Ne fais jamais ça : A. Pousser sur main. B. Sauter les tests."},
		{"i.", "Never do these: i. Push to main. ii. Skip the tests.", "Ne fais jamais ça : i. Pousser sur main. ii. Sauter les tests."},
		{"I.", "Never do these: I. Push to main. II. Skip the tests.", "Ne fais jamais ça : I. Pousser sur main. II. Sauter les tests."},
		{"Step 1.", "Never do these: Step 1. Push to main. Step 2. Skip the tests.", "Ne fais jamais ça : Étape 1. Pousser sur main. Étape 2. Sauter les tests."},
		{"-", "Never do these: - Push to main. - Skip the tests.", "Ne fais jamais ça : - Pousser sur main. - Sauter les tests."},
		{"•", "Never do these: • Push to main. • Skip the tests.", "Ne fais jamais ça : • Pousser sur main. • Sauter les tests."},
		{"*", "Never do these: * Push to main. * Skip the tests.", "Ne fais jamais ça : * Pousser sur main. * Sauter les tests."},
		{"–", "Never do these: – Push to main. – Skip the tests.", "Ne fais jamais ça : – Pousser sur main. – Sauter les tests."},
		{"—", "Never do these: — Push to main. — Skip the tests.", "Ne fais jamais ça : — Pousser sur main. — Sauter les tests."},
		{";", "Never do these: Push to main; Skip the tests.", "Ne fais jamais ça : Pousser sur main ; Sauter les tests."},
		{`"`, `Never say these: "Ship it." "Skip the tests."`, `Ne dis jamais ça : "On livre." "Sauter les tests."`},
		{"“", "Never say these: “Ship it.” “Skip the tests.”", "Ne dis jamais ça : “On livre.” “Sauter les tests.”"},
		{"«", "Never say these: «Ship it.» «Skip the tests.»", "Ne dis jamais ça : « On livre. » « Sauter les tests. »"},
		{"a never lead-in sentence, then (1)", "Never do the following. (1) Push to main. (2) Skip the tests.",
			"Ne fais jamais ce qui suit. (1) Pousser sur main. (2) Sauter les tests."},
		{"a never lead-in sentence, then A)", "Never do the following. A) Push to main. B) Skip the tests.",
			"Ne fais jamais ce qui suit. A) Pousser sur main. B) Sauter les tests."},
	} {
		rows = append(rows,
			sentenceRow{"the sweep, " + list.spelling, list.english, "Skip the tests", http.StatusAccepted, list.english, "Never"},
			sentenceRow{"le balayage, " + list.spelling, list.french, "Sauter les tests", http.StatusAccepted, list.french, "Ne … jamais"})
	}
	for i, row := range rows {
		d.owner.submitted("gm1", row.line)
		name := fmt.Sprintf("Quoted note %d", i)
		code, text := memoryAsk(t, d, callerGM, gm, "", "POST", memoryProposePath, proposal(map[string]string{"name": name,
			"description": "What the owner said", "type": "feedback", "rule": "Do as the owner said", "why": "The owner said so",
			"how": "Every time", "quote": row.quote}))
		k := slices.IndexFunc(d.memory.all(), func(n gmNote) bool { return n.Name == name })
		if code != row.status || k < 0 {
			t.Errorf("%s: %d %s, want %d and a note waiting", row.what, code, text, row.status)
			continue
		}
		note := d.memory.all()[k]
		source := note.Source
		// Withdrawn once read, so the rows never fill the waiting list.
		if err := d.memory.remove(note.ID, true); err != nil {
			t.Fatalf("%s: withdrawing it: %v", row.what, err)
		}
		if note.State != memoryWaiting || source.Quote != row.sentence || source.Quote[source.QuoteStart:source.QuoteEnd] == "" ||
			ownerPlain(source.Quote[source.QuoteStart:source.QuoteEnd]) != row.quote {
			t.Errorf("%s: kept %+v, want the sentence %q pointing at %q", row.what, note, row.sentence, row.quote)
		}
		hold := memoryQuoteHold(source)
		if row.dropped == "" {
			if hold.Kind != "" || strings.Contains(text, "Refused") {
				t.Errorf("%s: held %+v, answer %s", row.what, hold, text)
			}
			continue
		}
		// The GM is told it's refused as the owner's words; the owner's tap shows the whole sentence, in English or
		// in French.
		if !strings.Contains(text, "Refused as the owner's words") || !strings.Contains(text, strconv.Quote(row.dropped)[1:len(strconv.Quote(row.dropped))-1]) ||
			!strings.Contains(text, `"state":"waiting"`) {
			t.Errorf("%s: the answer %s", row.what, text)
		}
		if hold.Kind != "quote" || !strings.Contains(hold.English, strconv.Quote(row.sentence)) || !strings.Contains(hold.French, "« "+row.sentence+" »") ||
			!strings.Contains(hold.English, strconv.Quote(row.dropped)) || !strings.Contains(hold.French, "« "+row.dropped+" »") {
			t.Errorf("%s: the owner's tap %+v", row.what, hold)
		}
	}
	// Refused, nothing kept: a sentence past a quote's cap; a tab, kept verbatim, which MEM2 refuses as unseen; words
	// found in two places; a sentence holding an e-mail or a key, though the GM quoted around it.
	for what, row := range map[string]struct{ line, quote, said string }{
		"a sentence past the cap": {"Use pnpm for the web app in this office" + strings.Repeat(", and keep the release notes short", 30) + ".",
			"Use pnpm for the web app", memoryLongLine},
		// SIERRA's backstop: what's kept after a negation is the paragraph whole, so past the cap it's refused, though
		// the quote's own sentence is short.
		"a paragraph past the cap after a negation": {"Never deploy on a Friday.\n" + strings.Repeat("Keep the release notes short.\n", 35) +
			"Use pnpm for the web app.", "Use pnpm for the web app", memoryLongLine},
		"a tab kept as typed, which no note keeps": {"Always use pnpm\tfor the blog in this office.", "use pnpm for the blog", "can't see"},
		// A line break is kept as typed; a carriage return isn't, nor line breaks enough to push the rest out of sight.
		"a carriage return": {"Never on a Friday:\r\n- Merge the release branch.", "Merge the release branch", "can't see"},
		"a markdown heading, which MEM2 reads as markup": {"## Never on a Friday\n\n- Merge the hotfix branch.", "Merge the hotfix branch", "markup"},
		"line breaks that could hide the rest":           {"Never deploy on a Friday.\n\n\n\nUse pnpm for the landing page.", "Use pnpm for the landing page", "could hide the rest"},
		"words found twice":                              {"First run lint in the api, then run lint in the api again.", "run lint in the api", memoryQuoteTwice},
		"an e-mail around the quote":                     {"Send the weekly report to ada@example.com, and keep the changelog short.", "keep the changelog short", "Memory can't keep this note"},
		"a key around the quote":                         {"Keep the commit messages short, the key is sk-" + "ant-fakefakefake.", "Keep the commit messages short", "Memory can't keep this note"},
	} {
		d.owner.submitted("gm1", row.line)
		before := len(d.memory.all())
		code, text := memoryAsk(t, d, callerGM, gm, "", "POST", memoryProposePath, proposal(map[string]string{"name": what,
			"description": "What the owner said", "type": "feedback", "rule": "Do as the owner said", "why": "The owner said so",
			"how": "Every time", "quote": row.quote}))
		if code != http.StatusBadRequest || !strings.Contains(text, row.said) || len(d.memory.all()) != before ||
			strings.Contains(text, "ada@") || strings.Contains(text, "fakefake") {
			t.Errorf("%s: %d %s, want 400 and %q, nothing kept", what, code, text, row.said)
		}
	}
	// The store takes a pointer only into its sentence, and only for the ledger's words.
	for _, bad := range []memorySource{
		{Tag: memoryFromTerminal, Quote: "Use pnpm here.", QuoteStart: 4, QuoteEnd: 40},
		{Tag: memoryFromTerminal, Quote: "Use pnpm here.", QuoteStart: 6, QuoteEnd: 2},
		{Tag: memoryFromTerminal, Quote: "Utilise pnpm ici, à la recette.", QuoteStart: 19, QuoteEnd: 25},
		{Tag: memoryFromSeen, Quote: "package.json:12", QuoteStart: 0, QuoteEnd: 7},
	} {
		if refusal := memorySourceRefusal("project", bad); refusal == "" {
			t.Errorf("a pointer out of its sentence: %+v", bad)
		}
	}
}

// Withdraw: a note still waiting, never a kept one, whatever id is sent.
// A negation reads as one however the owner dressed it (FXMEM4R5, closing RT126MEM4R5 B1's class): markup around or
// in it, any apostrophe a keyboard types, an unseen character or a soft hyphen inside it, an accent typed apart, a
// wide, styled, circled or ligatured letter, any case. Each row, in English and in French, is read twice. The reader
// itself (ownerSentence, memoryQuoteHold) holds it, with the whole paragraph kept, whatever the route does. The route
// then gives 202, held, with that paragraph stored, or, for a character MEM2 already refuses in a note (an unseen one,
// a special or styled form, a tag), 400 with nothing kept, as before: the stricter answer stands.
func TestANegationReadsHoweverItIsDressed(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	const english, french = " do these on a Friday: (1) Push to main. (2) Skip the tests.",
		" le vendredi : (1) Pousser sur main. (2) Sauter les tests."
	const englishQuote, frenchQuote = "Skip the tests", "Sauter les tests"
	type dressedRow struct {
		what, line, quote, dropped string
		refused                    string // MEM2's refusal, or "" for 202 and held
	}
	rows := []dressedRow{
		// Markup: emphasis, strikethrough, code, an escape, a tag, and punctuation against the word.
		{"_Never_", "_Never_" + english, englishQuote, "Never", ""},
		{"_Jamais_", "_Jamais_" + french, frenchQuote, "Jamais", ""},
		{"__Never__", "__Never__" + english, englishQuote, "Never", ""},
		{"__Jamais__", "__Jamais__" + french, frenchQuote, "Jamais", ""},
		{"*Never*", "*Never*" + english, englishQuote, "Never", ""},
		{"**Jamais**", "**Jamais**" + french, frenchQuote, "Jamais", ""},
		{"~~Never~~", "~~Never~~" + english, englishQuote, "Never", ""},
		{"~Jamais~", "~Jamais~" + french, frenchQuote, "Jamais", ""},
		{"`Never`", "`Never`" + english, englishQuote, "Never", ""},
		{"`Jamais`", "`Jamais`" + french, frenchQuote, "Jamais", ""},
		{`\_Never\_`, `\_Never\_` + english, englishQuote, "Never", ""},
		{`\*Jamais\*`, `\*Jamais\*` + french, frenchQuote, "Jamais", ""},
		{"_never_ inside a name", "Run the push_never_hook first:\n- Push to main.\n- Skip the tests.", englishQuote, "never", ""},
		{"_jamais_ dans un nom", "Lance d'abord hook_jamais_main :\n- Pousser sur main.\n- Sauter les tests.", frenchQuote, "jamais", ""},
		{"Never/rarely", "Never/rarely" + english, englishQuote, "Never", ""},
		{"Jamais/rarement", "Jamais/rarement" + french, frenchQuote, "Jamais", ""},
		{"<b>Never</b>", "<b>Never</b>" + english, englishQuote, "Never", "markup"},
		{"<b>Jamais</b>", "<b>Jamais</b>" + french, frenchQuote, "Jamais", "markup"},
		// Apostrophes: typewriter, typographic, opening quote, modifier letter, prime, acute accent, backtick, wide.
		{"Don't", "Don't" + english, englishQuote, "Don't", ""},
		{"Don’t", "Don’t" + english, englishQuote, "Don’t", ""},
		{"Don‘t", "Don‘t" + english, englishQuote, "Don‘t", ""},
		{"Don′t", "Don′t" + english, englishQuote, "Don′t", ""},
		{"Don´t", "Don´t" + english, englishQuote, "Don´t", ""},
		{"Don`t", "Don`t" + english, englishQuote, "Don`t", ""},
		{"Donʼt", "Donʼt" + english, englishQuote, "Donʼt", "special form"},
		{"Don＇t", "Don＇t" + english, englishQuote, "Don＇t", "special form"},
		{"Don´t, one sentence", "Don´t push to main on a Friday.", "push to main on a Friday", "Don´t", ""},
		{"Don`t, one sentence", "Don`t push to main on a Friday.", "push to main on a Friday", "Don`t", ""},
		{"Don‘t, one sentence", "Don‘t push to main on a Friday.", "push to main on a Friday", "Don‘t", ""},
		{"N'utilise plus", "N'utilise plus ces commandes" + french, frenchQuote, "N' … plus", ""},
		{"N’utilise plus", "N’utilise plus ces commandes" + french, frenchQuote, "N’ … plus", ""},
		{"N‘utilise plus", "N‘utilise plus ces commandes" + french, frenchQuote, "N‘ … plus", ""},
		{"N′utilise plus", "N′utilise plus ces commandes" + french, frenchQuote, "N′ … plus", ""},
		{"N´utilise plus", "N´utilise plus ces commandes" + french, frenchQuote, "N´ … plus", ""},
		{"N`utilise plus", "N`utilise plus ces commandes" + french, frenchQuote, "N` … plus", ""},
		{"Nʼutilise plus", "Nʼutilise plus ces commandes" + french, frenchQuote, "Nʼ … plus", "special form"},
		{"N＇utilise plus", "N＇utilise plus ces commandes" + french, frenchQuote, "N＇ … plus", "special form"},
		// Unseen characters and a soft hyphen inside the word.
		{"a zero-width space in never", "Nev\u200ber" + english, englishQuote, "Nev\u200ber", "can't see"},
		{"une espace sans chasse dans jamais", "Jam\u200bais" + french, frenchQuote, "Jam\u200bais", "can't see"},
		{"a zero-width non-joiner in never", "Nev\u200cer" + english, englishQuote, "Nev\u200cer", "can't see"},
		{"un antiliant sans chasse dans jamais", "Jam\u200cais" + french, frenchQuote, "Jam\u200cais", "can't see"},
		{"a zero-width joiner in never", "Nev\u200der" + english, englishQuote, "Nev\u200der", "can't see"},
		{"un liant sans chasse dans jamais", "Jam\u200dais" + french, frenchQuote, "Jam\u200dais", "can't see"},
		{"a word joiner in never", "Nev\u2060er" + english, englishQuote, "Nev\u2060er", "can't see"},
		{"un gluon de mots dans jamais", "Jam\u2060ais" + french, frenchQuote, "Jam\u2060ais", "can't see"},
		{"a byte-order mark in never", "Nev\ufeffer" + english, englishQuote, "Nev\ufeffer", "can't see"},
		{"une indication d'ordre des octets dans jamais", "Jam\ufeffais" + french, frenchQuote, "Jam\ufeffais", "can't see"},
		{"a soft hyphen in never", "Nev\u00ader" + english, englishQuote, "Nev\u00ader", "can't see"},
		{"un trait d'union conditionnel dans jamais", "Ja\u00admais" + french, frenchQuote, "Ja\u00admais", "can't see"},
		{"a zero-width space in don't", "Don\u200b't" + english, englishQuote, "Don\u200b't", "can't see"},
		{"a zero-width space between never and the next word", "Never\u200bdo these on a Friday: (1) Push to main. (2) Skip the tests.", englishQuote, "Never\u200bdo", "can't see"},
		// Case, and an accent typed apart (NFD).
		{"NEVER", "NEVER" + english, englishQuote, "NEVER", ""},
		{"JAMAIS", "JAMAIS" + french, frenchQuote, "JAMAIS", ""},
		{"nEvEr", "nEvEr" + english, englishQuote, "nEvEr", ""},
		{"jAmAiS", "jAmAiS" + french, frenchQuote, "jAmAiS", ""},
		{"ÉVITE", "ÉVITE ça" + french, frenchQuote, "ÉVITE", ""},
		{"an accent typed apart in never", "Ne\u0300ver" + english, englishQuote, "Ne\u0300ver", ""},
		{"évite, l'accent à part", "E\u0301vite ça" + french, frenchQuote, "E\u0301vite", ""},
		{"arrête, l'accent à part", "Arre\u0302te de faire ça :\n- Fusionner sans relecture.\n- Sauter les tests.", frenchQuote, "Arre\u0302te", ""},
		{"ne … guère, l'accent à part", "Ne pousse gue\u0300re sur main, et surtout :\n- Pousser le vendredi.\n- Sauter les tests.", frenchQuote, "Ne … gue\u0300re", ""},
		// Wide, styled, circled and ligatured letters, as NFKC folds them.
		{"ＮＥＶＥＲ", "ＮＥＶＥＲ" + english, englishQuote, "ＮＥＶＥＲ", "wide form"},
		{"ＪＡＭＡＩＳ", "ＪＡＭＡＩＳ" + french, frenchQuote, "ＪＡＭＡＩＳ", "wide form"},
		{"𝐍𝐞𝐯𝐞𝐫", "𝐍𝐞𝐯𝐞𝐫" + english, englishQuote, "𝐍𝐞𝐯𝐞𝐫", "styled"},
		{"𝐉𝐚𝐦𝐚𝐢𝐬", "𝐉𝐚𝐦𝐚𝐢𝐬" + french, frenchQuote, "𝐉𝐚𝐦𝐚𝐢𝐬", "styled"},
		{"𝑁𝑜𝑡ℎ𝑖𝑛𝑔, italic, with ℎ in its gap", "𝑁𝑜𝑡ℎ𝑖𝑛𝑔 of this on a Friday: (1) Push to main. (2) Skip the tests.", englishQuote, "𝑁𝑜𝑡ℎ𝑖𝑛𝑔", "styled"},
		{"ⓝⓔⓥⓔⓡ", "ⓝⓔⓥⓔⓡ" + english, englishQuote, "ⓝⓔⓥⓔⓡ", "special form"},
		{"ⓙⓐⓜⓐⓘⓢ", "ⓙⓐⓜⓐⓘⓢ" + french, frenchQuote, "ⓙⓐⓜⓐⓘⓢ", "special form"},
		{"ﬆop", "ﬆop doing this:\n- Merging without a review.\n- Skip the tests.", englishQuote, "ﬆop", "special form"},
	}
	for i, row := range rows {
		// The reader: the whole paragraph, held, naming the negation as typed.
		sentence, start, end, found := ownerSentence(row.line, row.quote)
		source := memorySource{Tag: memoryFromTerminal, Quote: sentence, QuoteStart: start, QuoteEnd: end}
		hold := memoryQuoteHold(source)
		if found != 1 || sentence != row.line || hold.Kind != "quote" || !strings.Contains(hold.English, strconv.Quote(row.dropped)) ||
			!strings.Contains(hold.French, "« "+row.dropped+" »") {
			t.Errorf("%s: read as %q (%d found), held %+v, want the whole line held for %q", row.what, sentence, found, hold, row.dropped)
		}
		// The route.
		d.owner.submitted("gm1", row.line)
		name := fmt.Sprintf("Dressed note %d", i)
		before := len(d.memory.all())
		code, text := memoryAsk(t, d, callerGM, gm, "", "POST", memoryProposePath, proposal(map[string]string{"name": name,
			"description": "What the owner said", "type": "feedback", "rule": "Do as the owner said", "why": "The owner said so",
			"how": "Every time", "quote": row.quote}))
		if row.refused != "" {
			if code != http.StatusBadRequest || !strings.Contains(text, row.refused) || len(d.memory.all()) != before {
				t.Errorf("%s: %d %s, want 400 and %q, nothing kept", row.what, code, text, row.refused)
			}
			continue
		}
		k := slices.IndexFunc(d.memory.all(), func(n gmNote) bool { return n.Name == name })
		if code != http.StatusAccepted || k < 0 || !strings.Contains(text, "Refused as the owner's words") {
			t.Errorf("%s: %d %s, want 202 and a note held", row.what, code, text)
			continue
		}
		note := d.memory.all()[k]
		if err := d.memory.remove(note.ID, true); err != nil {
			t.Fatalf("%s: withdrawing it: %v", row.what, err)
		}
		if kept := memoryQuoteHold(note.Source); note.State != memoryWaiting || note.Source.Quote != row.line || kept.Kind != "quote" ||
			!strings.Contains(kept.English, strconv.Quote(row.dropped)) {
			t.Errorf("%s: kept %+v, held %+v, want the whole line held for %q", row.what, note, kept, row.dropped)
		}
	}
	// Splitting a word only adds negations: words with none, dressed the same, still stand, their sentence alone kept.
	for _, row := range []struct{ line, quote string }{
		{"_Always_ run the linter. Use pnpm for the web app.", "Use pnpm for the web app"},
		{"Use the snake_case names in the api folder.", "Use the snake_case names"},
		{"Lance d’abord les tests, puis pousse sur la branche.", "puis pousse sur la branche"},
		{"Utilise l`outil de déploiement de la recette.", "outil de déploiement"},
		{"Écris les notes de version en anglais.", "les notes de version"},
	} {
		sentence, start, end, found := ownerSentence(row.line, row.quote)
		hold := memoryQuoteHold(memorySource{Tag: memoryFromTerminal, Quote: sentence, QuoteStart: start, QuoteEnd: end})
		if found != 1 || hold.Kind != "" {
			t.Errorf("%q: read as %q (%d found), held %+v, want it standing", row.line, sentence, found, hold)
		}
	}
}

func TestTheGMWithdrawsOnlyANoteStillWaiting(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	withdraw := func(id string) (int, string) {
		return memoryAsk(t, d, callerGM, gm, "", "POST", memoryWithdrawPath+"?id="+id, "")
	}
	for _, id := range []string{kept.ID, "", "nope", "../manifest.json"} {
		if code, text := withdraw(id); code != http.StatusNotFound {
			t.Errorf("withdraw %q: %d %s", id, code, text)
		}
	}
	if code, text := withdraw(waiting.ID); code != http.StatusOK || !strings.Contains(text, `"withdrawn":true`) {
		t.Errorf("withdraw the waiting note: %d %s", code, text)
	}
	if code, _ := withdraw(waiting.ID); code != http.StatusNotFound {
		t.Errorf("withdrawn twice: %d", code)
	}
	if names := keptNames(d); !slices.Equal(names, []string{canaryNote.Name}) || len(d.memory.all()) != 1 {
		t.Errorf("after the withdrawals: kept %q, all %+v", names, d.memory.all())
	}
}

// No GM route keeps, changes or deletes a kept note: every other path under /setup/gm/memory/ is the page's alone,
// and a kept note and a waiting one stand after the GM tries every method on each.
func TestNoGMRouteKeepsOrDeletesAKeptNote(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	body := `{"id":"` + kept.ID + `","shown":"` + kept.SHA256 + `"}`
	for _, path := range []string{"/setup/gm/memory/keep", "/setup/gm/memory/answer", "/setup/gm/memory/delete", "/setup/gm/memory/forget",
		"/setup/gm/memory/edit", "/setup/gm/memory/tidy", "/setup/gm/memory/anything"} {
		for _, method := range []string{"GET", "POST", "DELETE"} {
			if routeAllows(callerGM, method, path) {
				t.Errorf("%s %s is the GM's", method, path)
			}
			for _, query := range []string{"", "?id=" + kept.ID + "&shown=" + kept.SHA256, "?id=" + waiting.ID + "&shown=" + waiting.SHA256} {
				if code, text := memoryAsk(t, d, callerGM, gm, "", method, path+query, body); code != http.StatusForbidden {
					t.Errorf("%s %s%s as the GM: %d %s", method, path, query, code, text)
				}
			}
		}
	}
	// The GM's own routes, with a kept note's id and hash, change nothing kept either.
	memoryAsk(t, d, callerGM, gm, "", "POST", memoryWithdrawPath+"?id="+kept.ID+"&shown="+kept.SHA256, body)
	memoryAsk(t, d, callerGM, gm, "", "POST", memoryReadPath+"?id="+kept.ID, body)
	all := d.memory.all()
	if len(all) != 2 || all[0].State != memoryKept || all[1].State != memoryWaiting || all[0].SHA256 != kept.SHA256 {
		t.Errorf("after the GM's tries: %+v", all)
	}
	// What the routes call on the store: a proposal, a read, and a withdrawal of a waiting note, nothing else.
	src, err := os.ReadFile("memoryroutes.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, never := range []string{".keep(", ".forget(", ".removeWhere(", ".save(", "remove(id, false)", "memoryFromOwner", "State = "} {
		if strings.Contains(string(src), never) {
			t.Errorf("memoryroutes.go calls %q", never)
		}
	}
	if strings.Count(string(src), ".remove(") != 1 || !strings.Contains(string(src), "d.memory.remove(id, true)") {
		t.Error("memoryroutes.go removes more than a waiting note")
	}
}

// Refused, a row each: a hire with its key, a tool with the setup key, an MCP key and a night job, at every memory
// route; nobody without a key gets 401.
func TestTheMemoryRoutesRefuseAHireAToolAnMCPKeyAndANightJob(t *testing.T) {
	d, gm, hireKey := memoryDeck(t)
	keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	keepNightJob(t, d, NightJob{ID: "nightly-tests", Status: nightApproved, ProposedBy: "page"})
	d.owner.submitted("gm1", "Please always run the tests with pnpm in this office, never npm.")
	note := proposal(map[string]string{"name": "Tests run with pnpm", "description": "Which tool runs the tests", "type": "feedback",
		"rule": "Run the tests with pnpm", "why": "The owner said so", "how": "Every time", "quote": "always run the tests with pnpm"})
	for _, ask := range []struct{ method, target, body string }{
		{"GET", memoryReadPath, ""},
		{"GET", memoryReadPath + "?name=Pelican+release+notes", ""},
		{"POST", memoryProposePath, note},
		{"POST", memoryWithdrawPath + "?id=" + waiting.ID, ""},
	} {
		path, _, _ := strings.Cut(ask.target, "?")
		for _, who := range []caller{callerHire, callerTool} {
			if routeAllows(who, ask.method, path) {
				t.Errorf("%s %s: the table lets in %s", ask.method, path, who.name())
			}
			if code, text := memoryAsk(t, d, who, gm, hireKey, ask.method, ask.target, ask.body); code != http.StatusForbidden ||
				strings.Contains(text, "pelican") || strings.Contains(text, "header") {
				t.Errorf("%s %s as %s: %d %s", ask.method, ask.target, who.name(), code, text)
			}
		}
		if code, _ := memoryAsk(t, d, callerNobody, gm, hireKey, ask.method, ask.target, ask.body); code != http.StatusUnauthorized {
			t.Errorf("%s %s with no key: %d", ask.method, ask.target, code)
		}
		for _, a := range []actor{mcpActor("claude-desktop"), nightActor("nightly-tests")} {
			if v := d.decide(a, action{route: &routeAsk{method: ask.method, path: path}}); v.allowed() || v.status != http.StatusForbidden ||
				v.why != memoryRouteRefusal {
				t.Errorf("%s %s by %s: %+v", ask.method, path, a, v)
			}
		}
		if v := d.decide(actorGM, action{route: &routeAsk{method: ask.method, path: path}}); !v.allowed() {
			t.Errorf("%s %s by the GM: %+v", ask.method, path, v)
		}
	}
	if all := d.memory.all(); len(all) != 2 {
		t.Errorf("a refused caller changed memory: %+v", all)
	}
}

// The reviewer's raw-path probes (RT126MEM4), made with the paths as sent (httptest reads them as a server does, never
// cleaned): the GM with a hire's key beside its token is refused; a path just past the GM's own falls to the page's
// catch-all, 403; one that only looks like a memory route isn't one, 404. No answer holds the canary.
func TestTheMemoryRoutesRawPathProbes(t *testing.T) {
	d, gm, hireKey := memoryDeck(t)
	keptNote(t, d, canaryNote)
	both := call{withoutSetupKey: true, method: "GET", target: memoryReadPath, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{gmTokenHeader: gm, hireKeyHeader: hireKey}}
	if w := serve(t, d, both); w.Code != http.StatusForbidden || strings.Contains(strings.ToLower(w.Body.String()), "pelican") {
		t.Errorf("a hire's key beside the GM's token: %d %s", w.Code, w.Body)
	}
	for target, want := range map[string]int{
		"/setup/gm/memory/":                    http.StatusForbidden,
		"/setup/gm/memory/propose/":            http.StatusForbidden,
		"/setup/gm/memory/../memory":           http.StatusForbidden,
		"/setup/gm/memory%2Fkeep":              http.StatusForbidden,
		"/setup/gm/memory/?name=Pelican+notes": http.StatusForbidden,
		"/setup/gm//memory":                    http.StatusNotFound,
		"/setup/gm/Memory":                     http.StatusNotFound,
		"/setup/gm/./memory":                   http.StatusNotFound,
	} {
		for _, method := range []string{"GET", "POST"} {
			code, text := memoryAsk(t, d, callerGM, gm, "", method, target, "")
			if code != want || strings.Contains(strings.ToLower(text), "pelican") {
				t.Errorf("%s %s as the GM: %d %s, want %d", method, target, code, text, want)
			}
		}
	}
}

// The start message grows by one sentence, the same whatever memory holds (CMDLEN): never a note.
func TestTheGMsIntroductionGrowsByOneFixedSentence(t *testing.T) {
	for _, missing := range []bool{false, true} {
		intro := gmIntroduction("/x/hires", missing)
		if strings.Count(intro, gmMemorySentence) != 1 || strings.Contains(intro, memoryHeader) {
			t.Errorf("the introduction (tools missing %v): %q", missing, intro)
		}
	}
	if !strings.Contains(gmIntroduction("/x/hires", false), "so don't look for a token or a key. "+gmMemorySentence) {
		t.Errorf("where the sentence goes: %q", gmIntroduction("/x/hires", false))
	}
}

// R-MEMDATA, by the code: the store is read only where memory.go, the routes and the records (its seal's rotation and
// Setup's line) read it. The brief, the start message, the command line, the environment, the plugin, the events and
// every gate are written by files that never reach it.
func TestOnlyTheRoutesAndTheRecordsReachTheStore(t *testing.T) {
	allowed := map[string]bool{"memory.go": true, "memoryroutes.go": true, "records.go": true, "deck.go": true, "server.go": true}
	files, _ := filepath.Glob("*.go")
	found := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "memory" {
				found[file] = true
				if !allowed[file] {
					t.Errorf("%s reads the GM's memory store", file)
				}
			}
			return true
		})
	}
	// deck.go loads it; server.go names its set-aside files in Setup's state alone.
	if src, _ := os.ReadFile("server.go"); strings.Count(string(src), "d.memory.") != 1 || !strings.Contains(string(src), "d.memory.asideText()") {
		t.Error("server.go reads more of the store than its set-aside files' line")
	}
	if !found["memoryroutes.go"] {
		t.Error("the routes don't read the store: this test reads the wrong thing")
	}
}

// No gate reaches memory (MEM2's use-time check is the wall; the gates hold whatever a note says): the gate service,
// the route table, the GM's scope, the owner's grants and taps, the manager's switch, a hire's settings and the
// fence name no memory symbol but the routes' own paths and refusal. The guard (internal/guard) is a package
// desktop imports, so it can't reach the store at all.
func TestNoGateReachesMemory(t *testing.T) {
	names := []string{"gmMemory", "gmNote", "loadMemory", "memoryIndex", "memoryIndexLine", "memoryRefusal", "memoryNoteRefusal",
		"memoryEntry", "memoryHold", "memorySource", "readMemory", "proposeMemory", "withdrawMemory", "ownerWords", "ownerLine"}
	// Fields and methods, as a selector: d.memory, d.owner, and the store's and the ledger's readers.
	selectors := []string{"memory", "owner", "kept", "quoted", "notes"}
	check := func(file string, node ast.Node) {
		ast.Inspect(node, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if slices.Contains(selectors, n.Sel.Name) {
					t.Errorf("%s reaches .%s", file, n.Sel.Name)
				}
			case *ast.Ident:
				if slices.Contains(names, n.Name) {
					t.Errorf("%s names %s", file, n.Name)
				}
			}
			return true
		})
	}
	for _, file := range []string{"gates.go", "routes.go", "gmscope.go", "grants.go", "hiresettings.go", "manager.go", "hire.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if file != "hire.go" {
			check(file, f)
			continue
		}
		// hire.go holds more than its fence: fenceRefusal alone.
		fence := false
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "fenceRefusal" {
				fence = true
				check(file, fn)
			}
		}
		if !fence {
			t.Error("fenceRefusal isn't in hire.go: this test reads the wrong file")
		}
	}
}

// A summary goes to the Summary key's provider: a call to the GM's memory is said as one, and what it gave back as
// the GM's memory, never what either holds, by the CLI, a call to the route, or the GM's MCP tools.
func TestASummaryHoldsNoMemory(t *testing.T) {
	lines := [][]byte{}
	add := func(entry map[string]any) {
		data, _ := json.Marshal(entry)
		lines = append(lines, data)
	}
	use := func(id, name string, input map[string]any) {
		add(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
			"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}})
	}
	result := func(id, text string) {
		add(map[string]any{"type": "user", "message": map[string]any{"role": "user",
			"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": text}}}})
	}
	use("t1", "Bash", map[string]any{"command": "smolcorp gm memory show 'Pelican release notes' | tail -n 3", "description": "Pelican notes"})
	result("t1", "The release notes for the pelican app are in the docs folder.")
	use("t2", "mcp__plugin_smolcorp-gm_smolcorp__memory", map[string]any{"name": "Pelican release notes"})
	result("t2", memoryHeader+"\n\nPelican release notes")
	use("t3", "mcp__plugin_smolcorp-gm_smolcorp__memory_propose", map[string]any{"name": "Pelican", "description": "Pelican docs"})
	result("t3", `{"id":"X","name":"Pelican"}`)
	use("t4", "Bash", map[string]any{"command": "curl -s localhost:7710/setup/gm/memory?name=Pelican"})
	result("t4", "pelican")
	use("t5", "Bash", map[string]any{"command": "cat notes.txt"})
	result("t5", memoryHeader+" pelican")
	use("t6", "Bash", map[string]any{"command": "ls docs"})
	result("t6", "guide.md")
	// The reviewer's N1 probe: the command's name hidden and the header cut, what the command prints minus its first
	// two lines. Every line it prints is marked.
	var printed strings.Builder
	if err := gm.RenderMemory(&printed, gm.MemoryReply{Header: memoryHeader, Notes: []gm.MemoryNote{{Name: canaryNote.Name,
		Type: canaryNote.Type, Description: canaryNote.Description}}}, false); err != nil {
		t.Fatal(err)
	}
	cut := strings.SplitN(printed.String(), "\n", 3)
	use("t7", "Bash", map[string]any{"command": "c=memory; smolcorp gm $c | tail -n +3"})
	result("t7", cut[2])
	extract := extractTranscript(lines, 4000, nil, nil)
	if strings.Contains(strings.ToLower(extract.Text), "pelican") || strings.Count(extract.Text, "RESULT: (the GM's memory)") != 6 ||
		!strings.Contains(extract.Text, "TOOL Bash: ls docs") || !strings.Contains(extract.Text, "RESULT: guide.md") {
		t.Errorf("the summary's transcript:\n%s", extract.Text)
	}
	// The office's line under the desk says it in words of its own.
	if got := activityOf("Bash", map[string]any{"command": "smolcorp  gm memory propose --name Pelican", "description": "Proposing pelican"}); got != "Using its memory" {
		t.Errorf("the activity: %q", got)
	}
}
