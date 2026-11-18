package desktop

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// The pull request badge's click (ghpropen.go): what the core opens, and for whom. No test opens a real browser:
// openURL is a stand-in that writes down what it was asked.

// The address to open is the desk's own repo's pull request page and nothing else: another host, scheme, repo or
// number, any extra in the address (a user, a port, a query, a fragment, a longer path) and an empty or missing
// badge or repo are refused. A different case of the owner or name is GitHub's own and opens, rebuilt from the repo.
func TestOnlyTheDesksOwnPullRequestPageIsOpened(t *testing.T) {
	const repo = "acme/widgets"
	good := func(url string, number int) *PRBadge { return &PRBadge{Number: number, State: "OPEN", URL: url} }
	for name, tc := range map[string]struct {
		badge *PRBadge
		repo  string
		want  string
	}{
		"the pull request":           {good("https://github.com/acme/widgets/pull/42", 42), repo, "https://github.com/acme/widgets/pull/42"},
		"GitHub's own case":          {good("https://github.com/Acme/Widgets/pull/42", 42), repo, "https://github.com/acme/widgets/pull/42"},
		"the remote's case":          {good("https://github.com/acme/widgets/pull/42", 42), "Acme/Widgets", "https://github.com/Acme/Widgets/pull/42"},
		"dots and dashes in a name":  {good("https://github.com/a-b/w.x_y-z/pull/7", 7), "a-b/w.x_y-z", "https://github.com/a-b/w.x_y-z/pull/7"},
		"no badge":                   {nil, repo, ""},
		"no repo":                    {good("https://github.com/acme/widgets/pull/42", 42), "", ""},
		"another repo":               {good("https://github.com/acme/gadgets/pull/42", 42), repo, ""},
		"another owner":              {good("https://github.com/evil/widgets/pull/42", 42), repo, ""},
		"a fork of the same name":    {good("https://github.com/me/widgets/pull/42", 42), repo, ""},
		"another number":             {good("https://github.com/acme/widgets/pull/43", 42), repo, ""},
		"zero":                       {good("https://github.com/acme/widgets/pull/0", 0), repo, ""},
		"a number too big":           {good("https://github.com/acme/widgets/pull/100000001", 100000001), repo, ""},
		"http":                       {good("http://github.com/acme/widgets/pull/42", 42), repo, ""},
		"ssh":                        {good("ssh://git@github.com/acme/widgets/pull/42", 42), repo, ""},
		"a script":                   {good("javascript:alert(1)//github.com/acme/widgets/pull/42", 42), repo, ""},
		"a file":                     {good("file:///etc/passwd", 42), repo, ""},
		"another host":               {good("https://evil.test/acme/widgets/pull/42", 42), repo, ""},
		"a host that ends in it":     {good("https://notgithub.com/acme/widgets/pull/42", 42), repo, ""},
		"a host that starts with it": {good("https://github.com.evil.test/acme/widgets/pull/42", 42), repo, ""},
		"a subdomain":                {good("https://gist.github.com/acme/widgets/pull/42", 42), repo, ""},
		"www":                        {good("https://www.github.com/acme/widgets/pull/42", 42), repo, ""},
		"an enterprise host":         {good("https://github.example.com/acme/widgets/pull/42", 42), repo, ""},
		"a user in front":            {good("https://evil.test@github.com/acme/widgets/pull/42", 42), repo, ""},
		"a user and the real host":   {good("https://github.com@evil.test/acme/widgets/pull/42", 42), repo, ""},
		"a port":                     {good("https://github.com:8443/acme/widgets/pull/42", 42), repo, ""},
		"a query":                    {good("https://github.com/acme/widgets/pull/42?x=1", 42), repo, ""},
		"a fragment":                 {good("https://github.com/acme/widgets/pull/42#files", 42), repo, ""},
		"a longer path":              {good("https://github.com/acme/widgets/pull/42/files", 42), repo, ""},
		"a trailing slash":           {good("https://github.com/acme/widgets/pull/42/", 42), repo, ""},
		"another kind of page":       {good("https://github.com/acme/widgets/issues/42", 42), repo, ""},
		"a tail":                     {good("https://github.com/acme/widgets/pull/42 https://evil.test", 42), repo, ""},
		"a line break":               {good("https://github.com/acme/widgets/pull/42\nhttps://evil.test", 42), repo, ""},
		"a backslash":                {good(`https://github.com\@evil.test/acme/widgets/pull/42`, 42), repo, ""},
		"upper-case scheme":          {good("HTTPS://github.com/acme/widgets/pull/42", 42), repo, ""},
		"upper-case host":            {good("https://GitHub.com/acme/widgets/pull/42", 42), repo, ""},
		"a look-alike character":     {good("https://github.com/acme/widgets/pull/４２", 42), repo, ""},
		"nothing":                    {good("", 42), repo, ""},
	} {
		if got := prOpenURL(tc.badge, tc.repo); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

// The click: the window's own key opens the desk's pull request once, by the desk's id alone; nothing the page sends
// can name an address. A desk with no badge, one that is gone, a badge whose address isn't its repo's, a phone and a
// browser that can't start each open nothing.
func TestTheBadgeClickOpensThePullRequestFromTheWindowOnly(t *testing.T) {
	repo := aGitRepo(t)
	gitRepoCmd(t, repo, "remote", "add", "origin", "https://github.com/acme/widgets.git")
	gitRepoCmd(t, repo, "checkout", "-q", "-b", "feat/badge")

	f := &fakeGH{runs: map[string]int{}}
	p, _ := withPRs(t, f.run)

	var mu sync.Mutex
	var opened []string
	var openErr error
	saved := openURL
	t.Cleanup(func() { openURL = saved })
	openURL = func(link string) error {
		mu.Lock()
		defer mu.Unlock()
		opened = append(opened, link)
		return openErr
	}
	openedNow := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), opened...) }

	m := newMailDeck(t)
	d := m.d
	phone := pair(t, d)
	hook := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
		body:    jsonLine(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": repo}),
		headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude", "X-AgentDeck-Tab": "tty:/dev/ttys003"}})
	if hook.Code != http.StatusNoContent {
		t.Fatalf("the hook: %d %s", hook.Code, hook.Body)
	}
	click := func(session string) (int, string) {
		got := serve(t, d, call{method: "POST", target: "/desk/pr/open?session=" + session + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		return got.Code, strings.TrimSpace(got.Body.String())
	}

	// A desk whose badge isn't kept yet has nothing to open (the first look starts gh), and one that is gone.
	if code, _ := click("s1"); code != http.StatusNotFound || len(openedNow()) != 0 {
		t.Errorf("before gh answered: %d, opened %v", code, openedNow())
	}
	p.wg.Wait()
	if code, _ := click("gone"); code != http.StatusNotFound || len(openedNow()) != 0 {
		t.Errorf("a desk that is gone: %d, opened %v", code, openedNow())
	}

	// With the badge kept, a click opens it, once, the address rebuilt from the desk's repo.
	if code, text := click("s1"); code != http.StatusNoContent || text != "" {
		t.Fatalf("the click: %d %q", code, text)
	}
	if got := openedNow(); len(got) != 1 || got[0] != "https://github.com/acme/widgets/pull/42" {
		t.Errorf("opened %v", got)
	}
	// What the page sends besides the desk is not read: no address of its own, in the query or the body.
	before := len(openedNow())
	got := serve(t, d, call{method: "POST", target: "/desk/pr/open?session=s1&url=https://evil.test/&t=" + d.access.WindowKey, host: "localhost:7710",
		from: thisComputer, body: `{"url":"https://evil.test/acme/widgets/pull/42"}`})
	if got.Code != http.StatusNoContent || openedNow()[before] != "https://github.com/acme/widgets/pull/42" {
		t.Errorf("an address in the request: %d, opened %v", got.Code, openedNow())
	}

	// A phone, whoever it is, has no badge to click, and nothing opens.
	before = len(openedNow())
	refused := serve(t, d, call{method: "POST", target: "/desk/pr/open?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if refused.Code != http.StatusForbidden || len(openedNow()) != before || strings.Contains(refused.Body.String(), "github") {
		t.Errorf("phone: %d %s, opened %v", refused.Code, refused.Body, openedNow())
	}
	if got := serve(t, d, call{method: "GET", target: "/desk/pr/open?session=s1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer}); got.Code == http.StatusNoContent || len(openedNow()) != before {
		t.Errorf("a GET opened a page: %d", got.Code)
	}

	// A browser that can't start is said, not hidden.
	openErr = errors.New("no browser")
	if code, text := click("s1"); code != http.StatusBadGateway || !strings.Contains(text, "browser") {
		t.Errorf("no browser: %d %q", code, text)
	}
	openErr = nil

	// What is kept for the branch can't be bent: a badge with another repo's, another host's or another number's
	// address, put where gh's answer would be, opens nothing. (prFromGH refuses these before they are kept; this is the
	// second check, at the click.)
	before = len(openedNow())
	key := "acme/widgets\x00feat/badge"
	for name, bad := range map[string]*PRBadge{
		"another repo":   {Number: 42, State: "OPEN", URL: "https://github.com/acme/gadgets/pull/42"},
		"another host":   {Number: 42, State: "OPEN", URL: "https://evil.test/acme/widgets/pull/42"},
		"another number": {Number: 42, State: "OPEN", URL: "https://github.com/acme/widgets/pull/43"},
		"http":           {Number: 42, State: "OPEN", URL: "http://github.com/acme/widgets/pull/42"},
	} {
		p.mu.Lock()
		p.by[key] = &prEntry{answer: prAnswer{badge: bad}, has: true, at: p.now()}
		p.mu.Unlock()
		if code, _ := click("s1"); code != http.StatusNotFound || len(openedNow()) != before {
			t.Errorf("%s: %d, opened %v", name, code, openedNow()[before:])
		}
	}
}
