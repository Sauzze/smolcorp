package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

// The demo's real team: Smolcorp's office, through setup's API with the setup key the
// running app wrote for tools on this computer (as `smolcorp setup` reaches it), and
// real Claude Code on the owner's own account.

// officeTeam is the office as the demo asks it.
type officeTeam struct {
	base  string
	key   func() ([]byte, error)
	http  *http.Client
	start func() error // starts Smolcorp when it doesn't answer
	every time.Duration
	state demoState
	on    []demoSwitch // the switches the demo's page turned on and left on
	// noOpen: the demo's page isn't opened, its link is printed (--no-open).
	noOpen bool
	// wait is how long the demo waits for its General Manager's hire on the page.
	wait time.Duration
}

// demoState is what the demo reads of GET /setup/state.
type demoState struct {
	GMHired bool `json:"gmHired"`
	Manager struct {
		Enabled bool `json:"enabled"`
	} `json:"manager"`
	Hiring struct {
		Enabled bool `json:"enabled"`
		CanHire bool `json:"canHire"`
	} `json:"hiring"`
	Roles struct {
		Roles []struct {
			ID    string `json:"id"`
			Agent string `json:"agent"`
		} `json:"roles"`
	} `json:"roles"`
}

// demoMember is what the demo shows of a member of GET /setup/gm/team.
type demoMember struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	Role       string `json:"role"`
	Folder     string `json:"folder"`
	Worktree   string `json:"worktree"`
	Registered bool   `json:"registered"` // a hire, not a session alone
	Branch     string `json:"branch"`
	State      string `json:"state"`
}

func newOfficeTeam() *officeTeam {
	return &officeTeam{base: localBase(), key: func() ([]byte, error) { return os.ReadFile(setupKeyFile) },
		http: &http.Client{Timeout: time.Minute}, start: startApp, every: 3 * time.Second, wait: demoHireWait}
}

// call sends one request with the setup key and reads its JSON answer into answer (nil: none).
func (o *officeTeam) call(method, path string, body, answer any) error {
	key, err := o.key()
	if err != nil {
		return errors.New("no setup key: run the demo on the computer Smolcorp runs on, as yourself")
	}
	var sent io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		sent = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, o.base+path, sent)
	if err != nil {
		return err
	}
	request.Header.Set(setupKeyHeader, strings.TrimSpace(string(key)))
	request.Header.Set(setupHeader, "1")
	request.Header.Set("Content-Type", "application/json")
	response, err := o.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		return demoRefusal{response.StatusCode, strings.TrimSpace(string(data))}
	}
	if answer == nil {
		return nil
	}
	return json.Unmarshal(data, answer)
}

// demoRefusal is Smolcorp's answer to a request it refused.
type demoRefusal struct {
	status int
	said   string
}

func (r demoRefusal) Error() string { return "Smolcorp said: " + r.said }

func (o *officeTeam) up() bool {
	quick := &http.Client{Timeout: 2 * time.Second}
	response, err := quick.Get(o.base + "/pair")
	if err != nil {
		return false
	}
	response.Body.Close()
	return true
}

// ready finds the office (starting Smolcorp if it isn't running), and refuses while a GM is
// working (one per office), while any other hire is, or when no hire can open. The roles are those on Claude Code.
func (o *officeTeam) ready(out io.Writer) ([]string, error) {
	if !o.up() {
		if setupSystem == "darwin" && !setupMayOpenApp() {
			return nil, errors.New(macNotRunning)
		}
		fmt.Fprintln(out, "Smolcorp isn't running: starting it to read the office. Nothing is hired or spent before you say yes.")
		if err := o.start(); err != nil {
			return nil, fmt.Errorf("Smolcorp couldn't start: %v", err)
		}
		for deadline := time.Now().Add(setupStartWait); !o.up(); time.Sleep(500 * time.Millisecond) {
			if time.Now().After(deadline) {
				return nil, errors.New("Smolcorp didn't answer within 30 s. Open it, then run the demo again.")
			}
		}
	}
	if err := o.call(http.MethodGet, "/setup/state", nil, &o.state); err != nil {
		return nil, err
	}
	// The office as the demo needs it, read whatever the manager's switch says (the team's route is closed while it's off).
	var office struct {
		GMHired bool     `json:"gmHired"`
		Open    []string `json:"open"`
	}
	if err := o.call(http.MethodGet, "/setup/demo", nil, &office); err != nil {
		if refused := (demoRefusal{}); errors.As(err, &refused) && refused.status == http.StatusNotFound {
			return nil, errors.New(demoNotServed())
		}
		return nil, err
	}
	if o.state.GMHired || office.GMHired {
		return nil, errors.New("A General Manager is already hired. The demo hires its own: fire yours in the office first, then run the demo again.")
	}
	// Only the demo's own team in the office: its GM never finds another hire to brief, write to or fire.
	if len(office.Open) > 0 {
		return nil, fmt.Errorf("The demo runs its own team: fire or finish your hires first (%s), then run the demo again.", strings.Join(office.Open, ", "))
	}
	if !o.state.Hiring.CanHire {
		return nil, errors.New("Smolcorp can't open a hire on this computer yet: see the hiring step in Setup.")
	}
	var roles []string
	for _, r := range o.state.Roles.Roles {
		if r.ID != gmRole && agentOrClaude(r.Agent) == Claude {
			roles = append(roles, r.ID)
		}
	}
	if !slices.Contains(roles, "reviewer") {
		return nil, errors.New("The demo needs the Reviewer role on Claude Code: put it back in Setup's roles, then run the demo again.")
	}
	return roles, nil
}

// off is the switches that are off: manager, hiring, as ready read them.
func (o *officeTeam) off() []demoSwitch {
	var off []demoSwitch
	for _, s := range demoSwitches {
		if s.route == "/setup/gm/manager" && !o.state.Manager.Enabled || s.route == "/setup/hiring" && !o.state.Hiring.Enabled {
			off = append(off, s)
		}
	}
	return off
}

func (o *officeTeam) leftOn() []demoSwitch { return o.on }

// demoHireWait is how long the demo waits for you to hire its General Manager on its page.
const demoHireWait = 15 * time.Minute

// demoNotServed is why the demo can't go on with a Smolcorp that doesn't serve its page.
func demoNotServed() string {
	if setupSystem == "darwin" {
		return "Smolcorp on this Mac doesn't serve the demo's page yet, so the demo can't have you hire its General Manager here. Nothing was made."
	}
	return "This Smolcorp doesn't serve the demo's page: update it, then run the demo again. Nothing was made."
}

// demoLink is Setup at the demo, with no token: it opens signed in only in a browser Setup is already open in.
func (o *officeTeam) demoLink(f demoFolder) string {
	return o.base + "/setup?demo=" + url.QueryEscape(f.root)
}

// work has Setup open at the demo (or prints its link), waits for you to hire the General Manager there, and
// follows the team until the GM writes its report. The page's Hire turns on the switches the demo needs.
func (o *officeTeam) work(ctx context.Context, f demoFolder, roles []string, out io.Writer) error {
	off := o.off()
	// The app answers only for a demo folder it was told about (demopage.go): this one, with its roles.
	register := "/setup/demo/register?root=" + url.QueryEscape(f.root) + "&roles=" + url.QueryEscape(strings.Join(roles, ","))
	if err := o.call(http.MethodPost, register, nil, nil); err != nil {
		return fmt.Errorf("Smolcorp didn't take the demo's folder. %v\nNobody was hired. The demo folder stays: %s", err, f.root)
	}
	if o.noOpen {
		fmt.Fprintf(out, "Open the demo's page in a browser on this computer, and click Hire there:\n\n  %s\n\n"+
			"It opens signed in where Setup is already open. If it isn't, %s first, then open the link above.\n", o.demoLink(f), setupMenuWords())
	} else {
		if err := o.call(http.MethodPost, "/setup/demo/open?root="+url.QueryEscape(f.root), nil, nil); err != nil {
			return fmt.Errorf("The demo's page didn't open. %v\nNobody was hired. The demo folder stays: %s", err, f.root)
		}
		fmt.Fprintln(out, "Setup is open in your browser at the demo. Check what it says, then click Hire there: that's the one click the demo needs from you.")
	}
	fmt.Fprintln(out, "Waiting for the General Manager's hire. Nothing is hired until you click; Ctrl-C stops waiting.")
	if err := o.waitForGM(ctx, f); err != nil {
		return err
	}
	// What the page turned on for the hire, as the last word says.
	if err := o.call(http.MethodGet, "/setup/state", nil, &o.state); err == nil {
		now := o.off()
		for _, s := range off {
			if !slices.Contains(now, s) {
				o.on = append(o.on, s)
			}
		}
	}
	fmt.Fprintln(out, "You hired the General Manager. It shows you its plan in its terminal: answer there to approve it.")
	fmt.Fprintln(out, "Watch the team in the office, on your phone or from Smolcorp's menu. This window follows along; Ctrl-C stops following, not the team.")
	return o.follow(ctx, f, out)
}

// demoWaitWords is a wait as the demo says it: "15 minutes", "1 minute", "30 seconds".
func demoWaitWords(wait time.Duration) string {
	n, unit := int(wait/time.Minute), "minute"
	if n == 0 {
		n, unit = int(wait/time.Second), "second"
	}
	if n != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", n, unit)
}

// waitForGM waits, at most o.wait, for the General Manager hired into f's project. It gives up when the page was
// closed without a hire, when Smolcorp stops answering, or on Ctrl-C; the demo folder stays.
func (o *officeTeam) waitForGM(ctx context.Context, f demoFolder) error {
	stays := "The demo folder stays: " + f.root + ". Delete it, or run smolcorp demo again for a new one."
	deadline, failed := time.Now().Add(o.wait), 0
	for {
		var answer struct {
			Demo struct {
				Page string `json:"page"`
				GM   *struct {
					ID string `json:"id"`
				} `json:"gm"`
			} `json:"demo"`
		}
		if err := o.call(http.MethodGet, "/setup/demo?root="+url.QueryEscape(f.root), nil, &answer); err != nil {
			if failed++; failed >= 5 {
				return fmt.Errorf("Smolcorp stopped answering (%v). %s", err, stays)
			}
		} else {
			failed = 0
			if answer.Demo.GM != nil {
				return nil
			}
			if answer.Demo.Page == "closed" {
				return errors.New("The demo's page was closed without a hire: nobody was hired, nothing was spent. " + stays)
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("No General Manager was hired on the demo's page within %s: the demo stops waiting, and nothing was spent. %s",
				demoWaitWords(o.wait), stays)
		}
		select {
		case <-ctx.Done():
			return errors.New("Stopped waiting: nobody was hired, nothing was spent. If the demo's page is still open, close it. " + stays)
		case <-time.After(o.every):
		}
	}
}

// follow prints each change in the demo's team until the GM's report is there.
func (o *officeTeam) follow(ctx context.Context, f demoFolder, out io.Writer) error {
	shown, failed := map[string]string{}, 0
	for {
		if data, err := os.ReadFile(f.report()); err == nil {
			fmt.Fprintf(out, "\nThe General Manager's report (%s):\n\n%s\n", f.report(), strings.TrimSpace(string(data)))
			return nil
		}
		var team struct {
			Team []demoMember `json:"team"`
		}
		if err := o.call(http.MethodGet, "/setup/gm/team", nil, &team); err != nil {
			if failed++; failed >= 5 {
				return fmt.Errorf("Smolcorp stopped answering (%v). The demo folder stays: %s", err, f.root)
			}
		} else {
			failed = 0
			for _, m := range team.Team {
				if !within(f.root, resolveExisting(m.Folder)) {
					continue // someone else's hire
				}
				line := demoMemberLine(m)
				if shown[m.ID] != line {
					shown[m.ID] = line
					fmt.Fprintf(out, "%s  %s\n", time.Now().Format("15:04:05"), line)
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("\nStopped following: the team goes on in the office. %w", ctx.Err())
		case <-time.After(o.every):
		}
	}
}

func demoMemberName(m demoMember) string {
	if m.Nickname != "" {
		return m.Nickname
	}
	return m.Name
}

func demoMemberLine(m demoMember) string {
	line := fmt.Sprintf("%s (%s): %s", demoMemberName(m), m.Role, m.State)
	if m.Branch != "" {
		line += ", on " + m.Branch
	}
	return line
}

// demoGMHire is setup's hire of the GM into the project: its three questions answered, and the demo's goal as its task.
func demoGMHire(f demoFolder, roles []string) map[string]any {
	list := strings.Join(roles, ", ")
	return map[string]any{
		"role":   gmRole,
		"folder": f.repo,
		// The app holds this GM to the project (gmscope.go): its hires, mail, briefs and firings, whatever way it asks.
		"scope": f.repo,
		"answers": []string{
			"The three tasks in TASKS.md, in the demo project at " + f.repo + ", and nothing else.",
			"Only Claude Code hires on my own account, these roles: " + list + ". No API keys, no cloud, nothing paid.",
			"Your plan, before you hire. Nothing is pushed, merged, published or deleted in this demo: stop before that.",
		},
		"task": "This is Smolcorp's demo: `bill`, a small sample project made for it, in " + f.repo +
			". Everything stays in " + f.root + ".\n" +
			"1. Read TASKS.md and plan one hire per task. Show me the plan here and wait for my OK.\n" +
			"2. Hire one agent per task, each on its own branch: `smolcorp gm hire --role <role> --folder " + demoShellFolder(f.repo) +
			" --worktree --task '<what done means, and the proof>'` (in single quotes: the fence refuses a $ the shell would read). Roles you may hire: " + list +
			". Never hire into another folder: the demo's fence refuses it.\n" +
			"3. Check each hire's proof yourself: `python3 -m unittest` in its worktree, and its diff.\n" +
			"4. Have a Reviewer review each branch, and get what it finds fixed by the hire.\n" +
			"5. Stop before anything ships: don't merge, push, publish or delete. The project has no remote, on purpose.\n" +
			"6. Last, write " + demoReportName + " in " + f.root + " yourself (not in the project, and not through a hire): each task, its branch, its proof, " +
			"its review, and what you'd ship with my OK. That file ends the demo.",
	}
}
