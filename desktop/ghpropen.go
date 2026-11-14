package desktop

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// The pull request badge's click (1.26 R2): one click on the badge on a desk opens its pull request in the browser.
//
// The window sends the desk's id and nothing else, never an address. The core looks up what it holds for that desk
// (the same answer /desk/git gave: the folder's branch and remote, and the badge kept for them) and opens the
// address only if it is exactly https://github.com/<owner>/<name>/pull/<number> of the desk's own repo. So a page
// that was tampered with, or a badge answer that went wrong, can't make the computer open another host, another
// scheme or another repo. The route is the window's alone (phoneroutes.go): a phone has no badge in 1.26.

// prPagePattern is the one address shape a badge may open: https, github.com, an owner, a name, /pull/, a number.
// No port, no user, no query, no fragment and no other path can match it.
var prPagePattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)/pull/([0-9]+)$`)

// prOpenURL is the address to open for badge on a desk whose remote is repo ("owner/name"), or "" when it must not be
// opened: no badge, a repo that isn't github.com's, an address of another shape, host, scheme or repo than the desk's,
// or a number that isn't the badge's. The owner and the name may be spelled in another case than the remote's, as
// GitHub does; what is opened is the address rebuilt from the desk's own repo.
func prOpenURL(badge *PRBadge, repo string) string {
	if badge == nil || repo == "" || badge.Number < 1 || badge.Number > prMaxNumber {
		return ""
	}
	got := prPagePattern.FindStringSubmatch(badge.URL)
	if got == nil || !strings.EqualFold(got[1]+"/"+got[2], repo) || got[3] != strconv.Itoa(badge.Number) {
		return ""
	}
	return "https://github.com/" + repo + "/pull/" + got[3]
}

// deskPROpen answers POST /desk/pr/open for session id: it opens the desk's pull request in the browser.
func (d *Deck) deskPROpen(id string) (int, any) {
	d.mu.Lock()
	s := d.sessions.Get(id)
	var folder string
	var event float64
	if s != nil {
		folder, event = cmpOr(s.Project, s.Folder), s.LastEventAt
	}
	d.mu.Unlock()
	if s == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	state := gits.get(folder, event)
	badge, _ := prs.For(state)
	link := prOpenURL(badge, githubRepoOf(state.remote))
	if link == "" {
		return http.StatusNotFound, "That desk has no pull request to open."
	}
	if err := openURL(link); err != nil {
		return http.StatusBadGateway, "Could not open your browser."
	}
	return http.StatusNoContent, ""
}

// The badge's click, in the office's own window on this computer, never a phone (phoneroutes.go). The query says
// everything; there is no body.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/desk/pr/open", access: accessWindow,
		refusal: func(d *Deck) string { return d.shownOnlyInTheWindow("The pull request badge") },
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeAnswer(w)(d.deskPROpen(session))
		}})
}
