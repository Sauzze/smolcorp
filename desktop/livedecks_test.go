package desktop

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// liveDecks are the decks the tests have started (startDeck) and not yet stopped, each with the test that started
// it and its stop. A deck's mail pump (every 5 s, registryPath() under supportDir) and its rounds (the prune every
// 10 s reads host) read the package's folders and app from goroutines of their own. One still running while a test
// changes them reads them as they change: a data race that only shows once a test outlives the timer (FXCONTRACTRACE,
// the Studio's race). So every change of them (isolate, withMac and the other stand-ins for the app) is refused while
// a deck runs, at once and every time, instead of racing a timer (FXPUMPCLASS).
var liveDecks = struct {
	sync.Mutex
	decks map[*Deck]liveDeck
}{decks: map[*Deck]liveDeck{}}

type liveDeck struct {
	test string
	stop func()
}

// deckStarted counts d as running, started by the test named test, until stop has run.
func deckStarted(d *Deck, test string, stop func()) {
	liveDecks.Lock()
	defer liveDecks.Unlock()
	liveDecks.decks[d] = liveDeck{test: test, stop: stop}
}

// deckStopped says d has stopped: its mail pump, rounds and watchers have ended and been waited for.
func deckStopped(d *Deck) {
	liveDecks.Lock()
	defer liveDecks.Unlock()
	delete(liveDecks.decks, d)
}

// stopDeck stops d now, as its test's end would (startDeck), and waits for its goroutines: a test that goes on to
// change the package's folders or app, or to start a deck of its own folders, stops the deck it is done with first.
// The deck still answers what is asked of it; nothing of its own runs any more. Stopping it again does nothing.
func stopDeck(d *Deck) {
	liveDecks.Lock()
	live, ok := liveDecks.decks[d]
	liveDecks.Unlock()
	if ok {
		live.stop()
	}
}

// stopDecksOf stops every deck t and its subtests have started and that still runs (stopDeck): a test about to
// change the package's folders or app, or to start a deck of its own folders, is done with the decks it has.
func stopDecksOf(t testing.TB) {
	t.Helper()
	liveDecks.Lock()
	var stops []func()
	for _, live := range liveDecks.decks {
		if live.test == t.Name() || strings.HasPrefix(live.test, t.Name()+"/") {
			stops = append(stops, live.stop)
		}
	}
	liveDecks.Unlock()
	for _, stop := range stops {
		stop()
	}
}

// decksRunning names the tests whose decks still run, each once, in order.
func decksRunning() []string {
	liveDecks.Lock()
	defer liveDecks.Unlock()
	seen := map[string]bool{}
	var tests []string
	for _, live := range liveDecks.decks {
		if !seen[live.test] {
			seen[live.test] = true
			tests = append(tests, live.test)
		}
	}
	sort.Strings(tests)
	return tests
}

// noDeckRunning fails t at once when a deck still runs: what (isolate's folders, a stand-in for the app) would change
// what its goroutines read. The deck is stopped first (stopDeck), or each deck gets a subtest of its own, or the
// change is made before the deck starts.
func noDeckRunning(t testing.TB, what string) {
	t.Helper()
	if tests := decksRunning(); len(tests) > 0 {
		t.Fatalf("%s while a deck started by %s still runs: its mail pump and rounds read the package's folders and "+
			"app from their own goroutines (FXPUMPCLASS). Stop that deck first (stopDeck, or a subtest per deck), or make "+
			"the change before it starts", what, strings.Join(tests, ", "))
	}
}

// noDeckAtPutBack fails t when a deck still runs as what a test changed is put back at its end: the deck has outlived
// the test, or the subtest, that changed what it reads. It is stopped first, so it never reads them as they change.
func noDeckAtPutBack(t testing.TB, what string) {
	t.Helper()
	tests := decksRunning()
	if len(tests) == 0 {
		return
	}
	t.Errorf("%s put back while a deck started by %s still runs: it outlived what it reads (FXPUMPCLASS). Start it "+
		"on the t that changed them, or stop it first (stopDeck)", what, strings.Join(tests, ", "))
	liveDecks.Lock()
	var stops []func()
	for _, live := range liveDecks.decks {
		stops = append(stops, live.stop)
	}
	liveDecks.Unlock()
	for _, stop := range stops {
		stop()
	}
}

// fatalKept is a test's t whose Fatalf is kept, and ends only the goroutine it is called on, as t.Fatalf does.
type fatalKept struct {
	testing.TB
	fatal string
}

func (f *fatalKept) Helper() {}

func (f *fatalKept) Fatalf(format string, args ...any) {
	f.fatal = strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", " "))
	runtime.Goexit()
}

// fatalOf runs check with a t that keeps its Fatalf, and returns what it said, or "".
func fatalOf(t testing.TB, check func(testing.TB)) string {
	kept := &fatalKept{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		check(kept)
	}()
	<-done
	return kept.fatal
}

// A deck a test leaves running fails the next change of the package's folders at once, with the name of the test
// that started it; once it has stopped, the same change goes ahead. Nothing here waits for a timer.
func TestADeckLeftRunningFailsTheNextIsolateAtOnce(t *testing.T) {
	d := newTestDeck(t)
	// As a deck started by an earlier test and left running: counted under that test's name.
	liveDecks.Lock()
	own := liveDecks.decks[d]
	liveDecks.decks[d] = liveDeck{test: "TestAnEarlierTest", stop: own.stop}
	liveDecks.Unlock()
	said := fatalOf(t, func(t testing.TB) { noDeckRunning(t, "isolate()") })
	if !strings.Contains(said, "isolate() while a deck started by TestAnEarlierTest still runs") {
		t.Fatalf("a deck left running: %q", said)
	}
	for _, what := range []string{"withMac()", "host ="} {
		if said := fatalOf(t, func(t testing.TB) { noDeckRunning(t, what) }); !strings.HasPrefix(said, what+" while a deck started by TestAnEarlierTest") {
			t.Errorf("%s under a deck left running: %q", what, said)
		}
	}
	stopDeck(d)
	select {
	case <-d.mailPumpEnded:
	default:
		t.Error("stopDeck came back with the deck's mail pump still running")
	}
	d.rounds.mu.Lock()
	ended := d.rounds.ended
	d.rounds.mu.Unlock()
	if !ended {
		t.Error("stopDeck came back with the deck's rounds still running")
	}
	if said := fatalOf(t, func(t testing.TB) { noDeckRunning(t, "isolate()") }); said != "" {
		t.Errorf("once the deck has stopped: %q", said)
	}
	stopDeck(d) // again, as the test's end does too
}

// The decks a test starts are stopped at its end, and its next isolate() finds none running: no subtest's deck
// outlives its subtest. One a subtest starts on its parent's t (as each contract case once did, FXCONTRACTRACE) runs
// on after it, and the next case's change of the folders is refused at once, naming the test it belongs to.
func TestADecksTestEndsItAndTheNextIsolateGoesAhead(t *testing.T) {
	var d *Deck
	t.Run("a deck", func(t *testing.T) { d = newTestDeck(t) })
	if tests := decksRunning(); len(tests) != 0 {
		t.Fatalf("after its test: still running, from %v", tests)
	}
	select {
	case <-d.mailPumpEnded:
	default:
		t.Fatal("its test has ended, and its mail pump still runs")
	}
	isolate(t)
	parent := t
	t.Run("a deck on its parent's t", func(t *testing.T) { d = startDeck(parent) })
	said := fatalOf(t, func(t testing.TB) { noDeckRunning(t, "isolate()") })
	if want := "isolate() while a deck started by " + t.Name() + " still runs"; !strings.HasPrefix(said, want) {
		t.Errorf("a deck left on the parent: %q, want %q", said, want)
	}
	stopDecksOf(t)
	if tests := decksRunning(); len(tests) != 0 {
		t.Errorf("stopDecksOf left %v running", tests)
	}
}

// A test's deck is started by startDeck alone, on every system: one made with NewDeck() is never counted as running,
// nor stopped at its test's end, and its rounds would go on into the next tests (FXPUMPCLASS).
func TestEveryTestsDeckIsStartedByStartDeck(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("the tests' files: %v", err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, said := range newDecksOutsideStartDeck(fset, parsed) {
			t.Error(said)
		}
	}
}

// newDecksOutsideStartDeck says where a file names NewDeck outside startDeck: in any declaration, a function's or a
// package-level var's, called or not. A deck made through a func value, a parenthesized call or a var holding
// NewDeck is made all the same, and never counted (RT126PUMPCLASS B3).
func newDecksOutsideStartDeck(fset *token.FileSet, parsed *ast.File) []string {
	var said []string
	for _, decl := range parsed.Decls {
		where := "a package-level declaration"
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv == nil && fn.Name.Name == "startDeck" {
				continue
			}
			where = fn.Name.Name
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "NewDeck" {
				said = append(said, fmt.Sprintf("%s: %s makes a deck with NewDeck: start it with startDeck(t)", fset.Position(id.Pos()), where))
			}
			return true
		})
	}
	return said
}

// Every way of making a deck with NewDeck outside startDeck is found (the review's probe, RT126PUMPCLASS B3): called
// bare, through a func value or a var, parenthesized, in a package-level func literal, in a file for another system;
// and startDeck's own is not.
func TestADeckMadeAroundStartDeckIsFoundHoweverItIsMade(t *testing.T) {
	files := map[string]string{
		"zz_dodge_test.go": `package desktop

var zzMake = NewDeck
var zzDeck = func() *Deck { return NewDeck() }

func TestZZDodges(t *testing.T) {
	d1 := NewDeck()
	mk := NewDeck
	d2 := mk()
	d3 := (NewDeck)()
	d4 := zzMake()
	_, _, _, _ = d1, d2, d3, d4
}

func startDeck(t *testing.T) *Deck { return NewDeck() }
`,
		"zz_dodge_linux_test.go": "//go:build linux\n\npackage desktop\n\nfunc zzOnLinux() { _ = NewDeck() }\n",
	}
	want := map[string][]string{
		"zz_dodge_test.go":       {"zz_dodge_test.go:3:14: a package-level declaration", "zz_dodge_test.go:4:36: a package-level declaration", "zz_dodge_test.go:7:8: TestZZDodges", "zz_dodge_test.go:8:8: TestZZDodges", "zz_dodge_test.go:10:9: TestZZDodges"},
		"zz_dodge_linux_test.go": {"zz_dodge_linux_test.go:5:24: zzOnLinux"},
	}
	for name, source := range files {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, name, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, said := range newDecksOutsideStartDeck(fset, parsed) {
			got = append(got, strings.TrimSuffix(said, " makes a deck with NewDeck: start it with startDeck(t)"))
		}
		if !slices.Equal(got, want[name]) {
			t.Errorf("%s: found %q, want %q", name, got, want[name])
		}
	}
}

// The helpers that change the home folder or the app for a test (hooksOff, a Take over rig's ownersHome, phoneDeck's
// stand-in app) refuse at once while a deck runs, naming the test that started it, before they change anything (the
// review's swap probe, RT126PUMPCLASS B1, B2).
func TestAHelperThatChangesTheHomeOrTheAppRefusesUnderARunningDeck(t *testing.T) {
	d := newTestDeck(t)
	liveDecks.Lock()
	own := liveDecks.decks[d]
	liveDecks.decks[d] = liveDeck{test: "TestAnEarlierTest", stop: own.stop}
	liveDecks.Unlock()
	home, app, refusal := homeDir, host, hooksNotConnected
	rig := &t3TakeOverRig{t3Rig: &t3Rig{home: t.TempDir()}}
	for what, change := range map[string]func(testing.TB){
		"hooksOff()":   func(t testing.TB) { hooksOff(t) },
		"ownersHome()": func(t testing.TB) { rig.ownersHome(t) },
		"phoneDeck()":  func(t testing.TB) { phoneMacIn(t) },
	} {
		said := fatalOf(t, change)
		if want := what + " while a deck started by TestAnEarlierTest still runs"; !strings.HasPrefix(said, want) {
			t.Errorf("%s under a running deck: %q, want %q", what, said, want)
		}
		if homeDir != home || host != app || reflect.ValueOf(hooksNotConnected).Pointer() != reflect.ValueOf(refusal).Pointer() {
			t.Errorf("%s changed the home folder or the app before it refused", what)
		}
	}
	stopDeck(d)
}
