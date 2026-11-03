package desktop

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
)

// The app's own watches (the Mac's built-ins, Watchers.swift sync and check):
// for each hire at work, the lines it writes in STATUS.md (a NEEDS-REVIEW line,
// a DONE/STOP/BLOCKED/BUDGET REQUEST line), a refused permission, a usage limit
// and a wait of two minutes at a question; and, while a GM is at work, an hourly
// digest. They are set and taken away by sync at the start of each look, only
// while the manager is on and the builtinWatches setting is. They are kept in
// watchers.json like the others (so a file's place survives a restart), listed
// by GET /setup/gm/watches with "builtin", not counted in the 50, and can't be
// removed by hand. A firing is worded "<builtin>: <detail>".

const (
	// How a NEEDS-REVIEW firing is marked in the event log (its agent), so the deck and the phone can pick it up.
	builtinReviewMark = "needs-review"
	// A built-in's patterns: a line that begins with the word (a list dash may come first) (Watchers.reviewPattern, statusPattern).
	builtinReviewPattern = `^\s*(?:[-*]\s+)?NEEDS-REVIEW:`
	builtinStatusPattern = `^\s*(?:[-*]\s+)?(?:DONE|STOP|BLOCKED|BUDGET REQUEST)\b`
	builtinDonePattern   = `^\s*(?:[-*]\s+)?DONE\b`
	builtinDigestMinutes = 60.0
	builtinWaitMinutes   = 2.0
	// The most firings one built-in file watch makes at a look.
	builtinFiresMost = 10
	// Who set them: not the GM, not the owner.
	builtinBy = "app"
)

// builtinKey is what makes two built-ins the same one (Watchers.sync's key).
func builtinKey(x watch) string { return x.Builtin + "|" + x.Hire + "|" + x.Target }

// setBuiltins gives the watchers the app's own watches: on says whether they are
// wanted now, asking the folders with a session at a question.
func (w *watchers) setBuiltins(on func() bool, asking func() []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.builtinsOn, w.asking = on, asking
	w.sync = w.syncBuiltins
}

// syncBuiltins sets the built-in watches for the hires at work, and takes away
// those of hires that left, and all of them when they are switched off. What a
// new file watch finds there never fires (Watchers.sync).
func (w *watchers) syncBuiltins() {
	w.mu.Lock()
	on := w.builtinsOn
	w.mu.Unlock()
	var wanted []watch
	var hires []Hire
	if on != nil && on() {
		hires = w.hires()
		wanted = w.wantedBuiltins(hires)
	}
	wantedKeys := map[string]bool{}
	for _, x := range wanted {
		wantedKeys[builtinKey(x)] = true
	}
	// Which are new, so a file is looked at (with nothing held) only for those.
	w.mu.Lock()
	have := map[string]bool{}
	for _, x := range w.all {
		if x.Builtin != "" {
			have[builtinKey(x)] = true
		}
	}
	w.mu.Unlock()
	type state struct {
		size        int
		inode, tail uint64
		known       bool
	}
	states := map[string]state{}
	roots, home := w.roots(), w.home()
	for _, x := range wanted {
		if x.Kind != "file" || have[builtinKey(x)] {
			continue
		}
		if path, base, ok := watchInsideRoot(x.Target, roots, home); ok {
			size, inode, tail, known := watchFileState(base, path)
			states[builtinKey(x)] = state{size, inode, tail, known}
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	before := len(w.all)
	w.all = slices.DeleteFunc(w.all, func(x watch) bool { return x.Builtin != "" && !wantedKeys[builtinKey(x)] })
	changed := len(w.all) != before
	kept := map[string]bool{}
	for _, x := range w.all {
		if x.Builtin != "" {
			kept[builtinKey(x)] = true
		}
	}
	ms := math.Round(float64(w.now().UnixMilli()))
	for _, x := range wanted {
		key := builtinKey(x)
		if kept[key] {
			continue
		}
		kept[key] = true
		x.ID, x.CreatedAt = w.newID(), ms
		if s := states[key]; s.known {
			x.Offset, x.Inode, x.Tail = s.size, s.inode, s.tail
		}
		if x.Kind == "time" {
			x.Due = ms + math.Round(x.Minutes*60_000)
		}
		w.all = append(w.all, x)
		changed = true
	}
	if changed {
		w.save()
	}
}

// wantedBuiltins are the built-in watches the hires at work call for, without
// their ids (Watchers.sync).
func (w *watchers) wantedBuiltins(hires []Hire) []watch {
	roots, home := w.roots(), w.home()
	var live []Hire
	for _, h := range hires {
		if isOpen(h) {
			live = append(live, h)
		}
	}
	built := func(kind, builtin string, h Hire, target string) watch {
		return watch{Kind: kind, Builtin: builtin, Hire: h.ID, Target: target, Repeats: true, By: builtinBy}
	}
	var wanted []watch
	for _, h := range live {
		if h.Role == gmRole {
			continue
		}
		place := h.Worktree
		if place == "" {
			place = h.Folder
		}
		if place == "" {
			continue
		}
		for _, name := range []string{"STATUS.md", filepath.Join(".team", "STATUS.md")} {
			path, _, ok := watchInsideRoot(filepath.Join(place, name), roots, home)
			if !ok {
				continue
			}
			review := built("file", "needs-review", h, path)
			review.Pattern = builtinReviewPattern
			status := built("file", "status", h, path)
			status.Pattern = builtinStatusPattern
			wanted = append(wanted, review, status)
		}
		denied := built("event", "denied", h, "permissionDenied")
		limit := built("event", "limit", h, "notification")
		wait := built("idle", "wait", h, place)
		denied.Folder, limit.Folder, limit.Pattern, wait.Minutes = place, place, "^"+eventLimitDetail+"$", builtinWaitMinutes
		wanted = append(wanted, denied, limit, wait)
	}
	// The digest is the GM's, so it stops when the GM is fired.
	if i := slices.IndexFunc(live, func(h Hire) bool { return h.Role == gmRole }); i >= 0 {
		digest := built("time", "digest", live[i], "")
		digest.Minutes = builtinDigestMinutes
		wanted = append(wanted, digest)
	}
	return wanted
}

// noteAsking keeps when each folder was first seen asking, forgets those that
// stopped, and gives the state as it is now. With w.mu held.
func (w *watchers) noteAsking(asking []string, ms float64) map[string]float64 {
	now := map[string]bool{}
	for _, folder := range asking {
		now[folder] = true
	}
	if w.askingSince == nil {
		w.askingSince = map[string]float64{}
	}
	for folder := range w.askingSince {
		if !now[folder] {
			delete(w.askingSince, folder)
		}
	}
	for folder := range now {
		if _, ok := w.askingSince[folder]; !ok {
			w.askingSince[folder] = ms
		}
	}
	out := make(map[string]float64, len(w.askingSince))
	for k, v := range w.askingSince {
		out[k] = v
	}
	return out
}

// hireDisplayName is what a hire is called in a firing: its nickname, else its name, else its id (Watchers.hireName).
func hireDisplayName(id string, hires []Hire) string {
	i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == id })
	if i < 0 {
		return id
	}
	if hires[i].Nickname != "" {
		return hires[i].Nickname
	}
	return hires[i].Name
}

// builtinFileFires is one firing for each new line of a built-in file watch (up
// to ten a look), so none is lost in a burst (Watchers.check).
func builtinFileFires(x watch, path string, hits []string, hires []Hire) []watchFire {
	name := hireDisplayName(x.Hire, hires)
	done := compileWatchPattern(builtinDonePattern)
	var out []watchFire
	for _, line := range hits[:min(len(hits), builtinFiresMost)] {
		fire := watchFire{id: x.ID, folder: filepath.Dir(path),
			detail: name + ": " + filepath.Base(path) + ": " + cutEventDetail(line)}
		if x.Builtin == "needs-review" {
			fire.mark = builtinReviewMark
		}
		if watchMatches(done, line) {
			fire.done = name
		}
		out = append(out, fire)
	}
	return out
}

// waitFire is a question asked and still waiting: once for each wait. It notes
// the wait it told of in x.
func waitFire(x *watch, hires []Hire, waiting map[string]float64, now float64) (watchFire, bool) {
	started, found := 0.0, false
	for folder, at := range waiting {
		if (folder == x.Target || strings.HasPrefix(folder, x.Target+string(filepath.Separator))) && (!found || at < started) {
			started, found = at, true
		}
	}
	if !found || int64(started) <= x.LastEvent || now-started < x.Minutes*60_000 {
		return watchFire{}, false
	}
	x.LastEvent = int64(started)
	return watchFire{id: x.ID, folder: x.Target,
		detail: fmt.Sprintf("%s has waited %d min at a question", hireDisplayName(x.Hire, hires), int(x.Minutes))}, true
}

// digestText is one line for the GM: who is working, idle, waiting, and who
// finished since the last digest (Watchers.digest).
func digestText(hires []Hire, latest map[string]eventEntry, waiting map[string]float64, done []string) string {
	groups := map[string][]string{}
	for _, h := range hires {
		if h.Role == gmRole || !isOpen(h) {
			continue
		}
		place := h.Worktree
		if place == "" {
			place = h.Folder
		}
		who := h.Nickname
		if who == "" {
			who = h.Name
		}
		group := "working"
		if slices.ContainsFunc(mapKeys(waiting), func(f string) bool { return f == place || strings.HasPrefix(f, place+string(filepath.Separator)) }) {
			group = "waiting"
		} else if e, ok := quietest(place, latest); h.State == "idle" || (ok && e.Kind == "idle") {
			group = "idle"
		}
		groups[group] = append(groups[group], who)
	}
	list := func(names []string) string {
		if len(names) == 0 {
			return "none"
		}
		out := strings.Join(names[:min(len(names), 6)], ", ")
		if len(names) > 6 {
			out += fmt.Sprintf(" +%d", len(names)-6)
		}
		return out
	}
	return fmt.Sprintf("Hourly digest. Working: %s. Idle: %s. Waiting: %s. Done since last: %s.",
		list(groups["working"]), list(groups["idle"]), list(groups["waiting"]), list(done))
}

func mapKeys(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
