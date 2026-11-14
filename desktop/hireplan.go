package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The hire plan (1.26's K2, owner's answers 9 and 13): picking a company template asks the General Manager for
// a plan, or Smolcorp's own planner when no GM is up yet. The plan is the roles to hire, a folder each and a line
// for each one's brief. The GM proposes; only the owner approves, and only from the page: the plan goes into the
// hire queue (hirequeue.go) then, and nothing was hired before.
//
//	POST /setup/hire/plan          {template, planner?: "own"}  the page: ask for a plan
//	POST /setup/hire/plan/propose  {id, steps: [{role, folder, brief}]}  the GM, or the page
//	POST /setup/hire/plan/approve  {id, rev}                    the page alone: hire it
//	POST /setup/hire/plan/cancel                                the page alone: drop the plan
//	POST /setup/hire/queue/resume, /cancel                      the page alone
//	GET  /setup/hire/queue                                      the plan and the queue, for the page, a tool and the GM

// What a plan is waiting for.
const (
	hpAsked    = "asked"    // the GM was asked and hasn't proposed yet
	hpProposed = "proposed" // ready for the owner to approve
	hpApproved = "approved" // now in the queue
)

// hirePlan is a plan for one template: By is who made it, "smolcorp" or "gm", and Rev changes with each proposal,
// so an approval is of the plan the owner was shown.
type hirePlan struct {
	ID       string     `json:"id"`
	Template string     `json:"template"`
	Name     string     `json:"name"`
	By       string     `json:"by"`
	Status   string     `json:"status"`
	Rev      int        `json:"rev"`
	Steps    []hireStep `json:"steps"`
}

const (
	hirePlanBriefLimit = 200
	hirePlanGMDown     = "The General Manager hasn't proposed a plan yet. Wait for it, or use Smolcorp's own plan."
)

func planRefusal(status int, why string) error { return &hireRefusal{Status: status, Message: why} }

// firstSentence is the first sentence of text, for a brief line.
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, ". "); i >= 0 {
		return text[:i+1]
	}
	return text
}

// ownPlan is Smolcorp's planner: the template's roles in its order, the General Manager first unless one is up,
// each in the folder for new hires, with a line saying which place it takes on the team and what its role does.
func (d *Deck) ownPlan(t CompanyTemplate) ([]hireStep, error) {
	d.mu.Lock()
	roles := slices.Clone(d.config.Roles)
	folder, _ := existingFolder(d.config.HireFolder)
	d.mu.Unlock()
	var steps []hireStep
	if !readRegistry(registryPath(), false).gmIsHired() {
		steps = append(steps, hireStep{Role: t.GM})
	}
	for _, id := range t.Roles {
		steps = append(steps, hireStep{Role: id})
	}
	seen := map[string]int{}
	for i := range steps {
		j := slices.IndexFunc(roles, func(r Role) bool { return r.ID == steps[i].Role })
		if j < 0 {
			return nil, planRefusal(http.StatusConflict, "The "+t.Name+" team needs the "+steps[i].Role+" role, and it isn't in your roles any more.")
		}
		role := roles[j]
		seen[role.ID]++
		of := 0
		for _, id := range t.Roles {
			if id == role.ID {
				of++
			}
		}
		line := "You're the " + role.Name
		if role.ID != t.GM && of > 1 {
			line += " (" + strconv.Itoa(seen[role.ID]) + " of " + strconv.Itoa(of) + ")"
		}
		line += " on the " + t.Name + " team. " + firstSentence(role.Instructions)
		steps[i].Name, steps[i].Folder, steps[i].Brief = role.Name, folder, prefixRunes(line, hirePlanBriefLimit)
	}
	return steps, nil
}

// wantedRoles is the roles a plan for t has to hire, in order: the GM first unless one is up.
func wantedRoles(t CompanyTemplate, gmUp bool) []string {
	if gmUp {
		return slices.Clone(t.Roles)
	}
	return append([]string{t.GM}, t.Roles...)
}

// stepsHireRoles is whether steps hire exactly these roles, in this order.
func stepsHireRoles(steps []hireStep, roles []string) bool {
	return slices.EqualFunc(steps, roles, func(s hireStep, id string) bool { return s.Role == id })
}

// requestBody reads a JSON request body (at most 64 KiB).
func requestBody(r io.Reader, into any) bool {
	return json.NewDecoder(io.LimitReader(r, 64<<10)).Decode(into) == nil
}

// approver is who a request counts as for an approval: the page when it is the owner's own, from the page, and
// nobody when the GM's token, a hire's key or a tool's setup key rides along with the page's cookie.
func approver(r *request) actor {
	if r.caller == callerPage && r.Header.Get(gmTokenHeader) == "" && r.Header.Get(hireKeyHeader) == "" &&
		r.Header.Get(setupKeyHeader) == "" {
		return actorPage
	}
	if r.caller == callerPage {
		return actorNobody
	}
	return actorOf(r.caller)
}

// hireQueueRoute serves the plan and queue routes (see the top of this file), or says false for any other.
func (d *Deck) hireQueueRoute(w http.ResponseWriter, r *request) bool {
	reply, q := writeStatus(w), d.hq
	path := r.URL.Path
	if !strings.HasPrefix(path, "/setup/hire/plan") && !strings.HasPrefix(path, "/setup/hire/queue") {
		return false
	}
	if r.Method == http.MethodGet && path == "/setup/hire/queue" {
		writeJSON(w, http.StatusOK, q.snapshot())
		return true
	}
	if r.Method != http.MethodPost {
		return false
	}
	fail := func(err error) { reply(refusalStatus(err), err.Error()) }
	switch path {
	case "/setup/hire/plan":
		var body struct{ Template, Planner string }
		if err := approvalRefusal(approver(r)); err != nil {
			fail(err)
			return true
		}
		requestBody(r.Body, &body)
		plan, err := d.askForPlan(body.Template, body.Planner == "own")
		if err != nil {
			fail(err)
			return true
		}
		writeJSON(w, http.StatusOK, plan)
	case "/setup/hire/plan/propose":
		if r.caller != callerGM && r.caller != callerPage {
			reply(http.StatusForbidden, routeRefusal(r.Method, path))
			return true
		}
		var body struct {
			ID    string
			Steps []hireStep
		}
		requestBody(r.Body, &body)
		plan, err := d.proposePlan(body.ID, body.Steps, r.caller == callerGM)
		if err != nil {
			fail(err)
			return true
		}
		writeJSON(w, http.StatusOK, plan)
	case "/setup/hire/plan/approve":
		var body struct {
			ID  string
			Rev int
		}
		requestBody(r.Body, &body)
		if err := d.approvePlan(body.ID, body.Rev, approver(r)); err != nil {
			fail(err)
			return true
		}
		reply(http.StatusNoContent, "")
	case "/setup/hire/plan/cancel":
		if err := approvalRefusal(approver(r)); err != nil {
			fail(err)
			return true
		}
		q.mu.Lock()
		q.plan = nil
		q.keep()
		q.mu.Unlock()
		reply(http.StatusNoContent, "")
	case "/setup/hire/queue/resume", "/setup/hire/queue/cancel":
		do := q.resume
		if path == "/setup/hire/queue/cancel" {
			do = q.cancel
		}
		if err := do(approver(r)); err != nil {
			fail(err)
			return true
		}
		reply(http.StatusNoContent, "")
	default:
		return false
	}
	return true
}

// askForPlan starts a plan for a template: Smolcorp's own at once (and a GM that isn't up is never asked), or, with
// a GM up, the GM's, asked in its inbox, which the owner sees as "asked" until it proposes.
func (d *Deck) askForPlan(templateID string, own bool) (hirePlan, error) {
	t, ok := companyTemplate(templateID)
	if !ok {
		return hirePlan{}, planRefusal(http.StatusNotFound, "That company isn't one of the templates.")
	}
	q := d.hq
	q.mu.Lock()
	waiting := q.run != nil && (q.run.State == hqRunning || q.run.State == hqPaused || q.run.State == hqStopped)
	label := ""
	if waiting {
		label = q.run.Label
	}
	q.mu.Unlock()
	if waiting {
		return hirePlan{}, planRefusal(http.StatusConflict, "A hire queue is already waiting ("+label+"): cancel it first, or let it finish.")
	}
	gmUp := readRegistry(registryPath(), false).gmIsHired()
	plan := hirePlan{ID: newID(), Template: t.ID, Name: t.Name, By: "smolcorp", Status: hpProposed, Rev: 1}
	if gmUp && !own {
		plan.By, plan.Status = gmRole, hpAsked
	} else {
		steps, err := d.ownPlan(t)
		if err != nil {
			return hirePlan{}, err
		}
		plan.Steps = steps
	}
	q.mu.Lock()
	q.plan = &plan
	q.keep()
	q.mu.Unlock()
	if plan.Status == hpAsked {
		d.events.append(eventEntry{At: time.Now().UnixMilli(), Kind: eventKindMail, Detail: "Smolcorp asks you for a hire plan for the " + t.Name +
			" team (plan " + plan.ID + "): roles " + strings.Join(t.Roles, ", ") + ", each with a folder and a line for its brief. Answer with POST /setup/hire/plan/propose; the owner approves."})
	}
	return plan, nil
}

// proposePlan is a plan's steps, from the GM or the page: the template's roles in its order, each with a folder
// and a brief line. It doesn't hire anything; it makes the plan ready for the owner to approve.
func (d *Deck) proposePlan(id string, given []hireStep, byGM bool) (hirePlan, error) {
	q := d.hq
	q.mu.Lock()
	plan := q.plan
	if plan == nil || plan.ID != id || plan.Status == hpApproved {
		q.mu.Unlock()
		return hirePlan{}, planRefusal(http.StatusConflict, "There's no plan waiting for that.")
	}
	template := plan.Template
	q.mu.Unlock()
	t, _ := companyTemplate(template)
	d.mu.Lock()
	roles := slices.Clone(d.config.Roles)
	hireFolder, _ := existingFolder(d.config.HireFolder)
	d.mu.Unlock()
	want := wantedRoles(t, readRegistry(registryPath(), false).gmIsHired())
	if !stepsHireRoles(given, want) {
		return hirePlan{}, planRefusal(http.StatusBadRequest, "The plan has to hire the "+t.Name+" team's roles, in its order.")
	}
	steps := make([]hireStep, len(given))
	for i, s := range given {
		folder, why := gmHireFolder(s.Folder)
		if why != "" {
			return hirePlan{}, planRefusal(http.StatusBadRequest, why)
		}
		if folder == "" {
			folder = hireFolder // a folder each: the folder for new hires where the proposal left it out
		}
		j := slices.IndexFunc(roles, func(r Role) bool { return r.ID == s.Role })
		if j < 0 {
			return hirePlan{}, planRefusal(http.StatusNotFound, "That role isn't there any more. Pick another.")
		}
		steps[i] = hireStep{Role: s.Role, Name: roles[j].Name, Folder: folder, Brief: prefixRunes(briefTask(s.Brief), hirePlanBriefLimit)}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.plan == nil || q.plan.ID != id || q.plan.Status == hpApproved {
		return hirePlan{}, planRefusal(http.StatusConflict, "There's no plan waiting for that.")
	}
	q.plan.Steps, q.plan.Status, q.plan.Rev = steps, hpProposed, q.plan.Rev+1
	q.plan.By = "smolcorp"
	if byGM {
		q.plan.By = gmRole
	}
	q.keep()
	return *q.plan, nil
}

// approvePlan hires the plan: it goes into the queue, approved by `by`, which has to be the page.
func (d *Deck) approvePlan(id string, rev int, by actor) error {
	q := d.hq
	if err := approvalRefusal(by); err != nil {
		return err
	}
	q.mu.Lock()
	plan := q.plan
	switch {
	case plan == nil || plan.ID != id:
		q.mu.Unlock()
		return planRefusal(http.StatusConflict, "There's no plan waiting for that.")
	case plan.Status == hpAsked:
		q.mu.Unlock()
		return planRefusal(http.StatusConflict, hirePlanGMDown)
	case plan.Status != hpProposed || plan.Rev != rev:
		q.mu.Unlock()
		return planRefusal(http.StatusConflict, "The plan changed since you looked: read it again, then approve.")
	}
	steps, name := slices.Clone(plan.Steps), plan.Name
	q.mu.Unlock()
	if _, err := q.start(steps, name, by); err != nil {
		return err
	}
	q.mu.Lock()
	if q.plan != nil && q.plan.ID == id {
		q.plan.Status = hpApproved
		q.keep()
	}
	q.mu.Unlock()
	return nil
}
