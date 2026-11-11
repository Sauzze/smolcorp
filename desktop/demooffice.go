package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	on    []demoSwitch // the switches this run turned on and left on
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
		http: &http.Client{Timeout: time.Minute}, start: startApp, every: 3 * time.Second}
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
		return fmt.Errorf("Smolcorp said: %s", strings.TrimSpace(string(data)))
	}
	if answer == nil {
		return nil
	}
	return json.Unmarshal(data, answer)
}

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
	if o.state.GMHired {
		return nil, errors.New("A General Manager is already hired. The demo hires its own: fire yours in the office first, then run the demo again.")
	}
	// Only the demo's own team in the office: its GM never finds another hire to brief, write to or fire.
	var team struct {
		Team []demoMember `json:"team"`
	}
	if err := o.call(http.MethodGet, "/setup/gm/team", nil, &team); err != nil {
		return nil, err
	}
	var open []string
	for _, m := range team.Team {
		if m.Registered && m.State != "fired" && m.State != "failed" {
			open = append(open, demoMemberName(m))
		}
	}
	if len(open) > 0 {
		return nil, fmt.Errorf("The demo runs its own team: fire or finish your hires first (%s), then run the demo again.", strings.Join(open, ", "))
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

// work turns the manager and hiring on if they're off, hires the GM into the project with the
// demo's goal, and follows the team until the GM writes its report. Should anything fail before
// the GM is hired, it turns back off what it turned on.
func (o *officeTeam) work(ctx context.Context, f demoFolder, roles []string, out io.Writer) error {
	// Ctrl-C after the yes, before the GM is hired, is a no: nothing more is turned on or hired.
	stopped := errors.New("Stopped before hiring: nothing is working.")
	for _, s := range o.off() {
		if ctx.Err() != nil {
			return o.putBack(stopped, out)
		}
		if err := o.call(http.MethodPost, s.route+"?enabled=1", nil, nil); err != nil {
			return o.putBack(err, out)
		}
		o.on = append(o.on, s)
		fmt.Fprintf(out, "Turned %s on.\n", s.name)
	}
	var hired struct {
		Desk *int `json:"desk"`
	}
	if ctx.Err() != nil {
		return o.putBack(stopped, out)
	}
	if err := o.call(http.MethodPost, "/setup/hire", demoGMHire(f, roles), &hired); err != nil {
		return o.putBack(err, out)
	}
	at := ""
	if hired.Desk != nil {
		at = fmt.Sprintf(" at desk %d", *hired.Desk+1)
	}
	fmt.Fprintf(out, "Hired the General Manager%s. It shows you its plan in its terminal: answer there to approve it.\n", at)
	fmt.Fprintln(out, "Watch the team in the office, on your phone or from Smolcorp's menu. This window follows along; Ctrl-C stops following, not the team.")
	return o.follow(ctx, f, out)
}

// putBack turns off, last first, the switches this run turned on, says so, and gives back why it had to.
func (o *officeTeam) putBack(why error, out io.Writer) error {
	fmt.Fprintln(out, why)
	for i := len(o.on) - 1; i >= 0; i-- {
		s := o.on[i]
		if err := o.call(http.MethodPost, s.route+"?enabled=0", nil, nil); err != nil {
			fmt.Fprintf(out, "Couldn't turn %s back off (%v).\n", s.name, err)
			continue
		}
		o.on = slices.Delete(o.on, i, i+1)
		fmt.Fprintf(out, "Turned %s back off: the demo hired no General Manager.\n", s.name)
	}
	return errors.New("The demo stopped before hiring its General Manager.")
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
		"answers": []string{
			"The three tasks in TASKS.md, in the demo project at " + f.repo + ", and nothing else.",
			"Only Claude Code hires on my own account, these roles: " + list + ". No API keys, no cloud, nothing paid.",
			"Your plan, before you hire. Nothing is pushed, merged, published or deleted in this demo: stop before that.",
		},
		"task": "This is Smolcorp's demo: `bill`, a small sample project made for it, in " + f.repo +
			". Everything stays in " + f.root + ".\n" +
			"1. Read TASKS.md and plan one hire per task. Show me the plan here and wait for my OK.\n" +
			"2. Hire one agent per task, each on its own branch: `smolcorp gm hire --role <role> --folder " + f.repo +
			" --worktree --task '<what done means, and the proof>'` (in single quotes: the fence refuses a $ the shell would read). Roles you may hire: " + list +
			". Never hire into another folder: the demo's fence refuses it.\n" +
			"3. Check each hire's proof yourself: `python3 -m unittest` in its worktree, and its diff.\n" +
			"4. Have a Reviewer review each branch, and get what it finds fixed by the hire.\n" +
			"5. Stop before anything ships: don't merge, push, publish or delete. The project has no remote, on purpose.\n" +
			"6. Last, write " + demoReportName + " in " + f.root + " (not in the project): each task, its branch, its proof, " +
			"its review, and what you'd ship with my OK. That file ends the demo.",
	}
}
