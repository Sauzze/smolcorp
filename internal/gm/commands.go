package gm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// The most the app holds a long poll, and the most events it gives at once.
const (
	maxWait  = 30
	maxLimit = 1000
)

// Member is one row of the team (GET /setup/gm/team). Fields the CLI doesn't
// know are ignored; missing ones are zero. A number the app may send as null
// is a pointer.
type Member struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Nickname    string  `json:"nickname"`
	Role        string  `json:"role"`
	Account     string  `json:"account"`
	Agent       string  `json:"agent"`
	Folder      string  `json:"folder"`
	Branch      string  `json:"branch"`
	Worktree    string  `json:"worktree"`
	Registered  bool    `json:"registered"` // a hire, not a session alone
	Desk        *int    `json:"desk"`
	Floor       *int    `json:"floor"`
	Live        bool    `json:"live"`
	LiveState   string  `json:"liveState"`
	State       string  `json:"state"`
	Status      string  `json:"status"`
	IdleFor     *Millis `json:"idleFor"`
	LastEventAt *Millis `json:"lastEventAt"`
}

// TeamReply is the answer of GET /setup/gm/team.
type TeamReply struct {
	At       Millis    `json:"at"`
	Team     []Member  `json:"team"`
	Contacts []Contact `json:"contacts"` // who may write to whom (T403)
}

// Event is one line of the event log (GET /setup/gm/events).
type Event struct {
	ID      int64  `json:"id"`
	At      Millis `json:"at"` // milliseconds since 1970
	Kind    string `json:"kind"`
	Session string `json:"session"`
	Folder  string `json:"folder"`
	Agent   string `json:"agent"`
	Detail  string `json:"detail"`
}

// EventsReply is the answer of GET /setup/gm/events.
type EventsReply struct {
	Events []Event `json:"events"`
	Next   int64   `json:"next"`   // the since to send next time
	Oldest int64   `json:"oldest"` // the first id still on disk
}

// Approval is one session waiting for the owner (GET /setup/gm/approvals).
// It only shows: nothing in the GM's reach answers a prompt. Label is short
// (the tool and a file's name or a command's first word), never a command.
type Approval struct {
	Session        string  `json:"session"`
	Hire           string  `json:"hire"`
	Name           string  `json:"name"`
	Nickname       string  `json:"nickname"`
	Agent          string  `json:"agent"`
	Floor          *int    `json:"floor"`
	Desk           *int    `json:"desk"`
	Kind           string  `json:"kind"`
	Tool           string  `json:"tool"`
	Label          string  `json:"label"`
	Since          *Millis `json:"since"`
	WaitingSeconds *Millis `json:"waitingSeconds"`
	Notified       bool    `json:"notified"`
}

// ApprovalsReply is the answer of GET /setup/gm/approvals.
type ApprovalsReply struct {
	At        Millis     `json:"at"`
	Approvals []Approval `json:"approvals"`
}

// VersionInfo is the answer of GET /setup/gm/version.
type VersionInfo struct {
	App      string   `json:"app"`
	Version  string   `json:"version"`
	API      int      `json:"api"`
	Features []string `json:"features"`
}

func decodeError(err error) error {
	return &Error{Code: exitApp, Msg: fmt.Sprintf("%s's answer isn't what this command expects: %v", appName, err)}
}

// Team asks who is on the team; raw is the answer as served, for --json.
func Team(c *Client) (TeamReply, []byte, error) {
	raw, err := c.Get("/setup/gm/team", nil, requestTimeout)
	if err != nil {
		return TeamReply{}, nil, err
	}
	var t TeamReply
	if err := json.Unmarshal(raw, &t); err != nil {
		return TeamReply{}, nil, decodeError(err)
	}
	return t, raw, nil
}

// TeamHere is the team, asked as `smolcorp gm team` asks it, from this process's environment:
// the GM's token and the app's port. In a hire's environment it refuses, as the CLI does.
func TeamHere() ([]Member, error) {
	c, err := NewClient(osEnv())
	if err != nil {
		return nil, err
	}
	t, _, err := Team(c)
	return t.Team, err
}

// Events asks for the events after since, waiting up to wait seconds for one,
// at most limit of them (0 leaves the app's default).
func Events(c *Client, since int64, wait, limit int) (EventsReply, []byte, error) {
	q := url.Values{"since": {strconv.FormatInt(since, 10)}}
	if wait > 0 {
		q.Set("wait", strconv.Itoa(wait))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	timeout := requestTimeout
	if wait > 0 {
		timeout = time.Duration(wait+10) * time.Second
	}
	raw, err := c.Get("/setup/gm/events", q, timeout)
	if err != nil {
		return EventsReply{}, nil, err
	}
	var r EventsReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return EventsReply{}, nil, decodeError(err)
	}
	return r, raw, nil
}

// Version asks what the app is.
func Version(c *Client) (VersionInfo, []byte, error) {
	raw, err := c.Get("/setup/gm/version", nil, requestTimeout)
	if err != nil {
		return VersionInfo{}, nil, err
	}
	var v VersionInfo
	if err := json.Unmarshal(raw, &v); err != nil {
		return VersionInfo{}, nil, decodeError(err)
	}
	return v, raw, nil
}

// Approvals asks who is waiting for the owner; raw is the answer as served, for --json.
func Approvals(c *Client) (ApprovalsReply, []byte, error) {
	raw, err := c.Get("/setup/gm/approvals", nil, requestTimeout)
	if err != nil {
		return ApprovalsReply{}, nil, err
	}
	var r ApprovalsReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return ApprovalsReply{}, nil, decodeError(err)
	}
	return r, raw, nil
}

// displayName is the member's nickname, else its name, else its folder's name.
func (m Member) displayName() string {
	for _, s := range []string{m.Nickname, m.Name, filepath.Base(m.Folder)} {
		if s = oneLine(s); s != "" && s != "." && s != "/" {
			return s
		}
	}
	return "?"
}

// stateName is what the member is doing now: the session's, else its registry state.
func (m Member) stateName() string {
	if s := oneLine(m.LiveState); s != "" {
		return s
	}
	return oneLine(m.State)
}

// formatIdle writes seconds as 42s, 5m, 2h or 3d; nothing when unknown.
func formatIdle(n *Millis) string {
	if n == nil || *n < 0 {
		return ""
	}
	s := *n
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh", s/3600)
	}
	return fmt.Sprintf("%dd", s/86400)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func deskLabel(m Member) string {
	if m.Desk == nil {
		return "-"
	}
	// The app counts desks from 0; the office, and so the CLI, from 1.
	if m.Floor != nil {
		return fmt.Sprintf("%d/%d", *m.Floor, *m.Desk+1)
	}
	return strconv.Itoa(*m.Desk + 1)
}

// RenderTeam prints the team as a table, one row per member.
func RenderTeam(w io.Writer, t TeamReply) error {
	if len(t.Team) == 0 {
		_, err := fmt.Fprintln(w, "Nobody on the team yet.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tROLE\tACCOUNT\tSTATE\tIDLE\tFLOOR/DESK\tSTATUS")
	for _, m := range t.Team {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.displayName(), orDash(oneLine(m.Role)), orDash(oneLine(m.Account)), orDash(m.stateName()),
			orDash(formatIdle(m.IdleFor)), deskLabel(m), truncate(oneLine(m.Status), 60))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(t.Contacts) == 0 {
		return nil
	}
	fmt.Fprintln(w, "\nCONTACTS (may write to)")
	return RenderContacts(w, t.Contacts, t.Team)
}

// RenderApprovals prints the waits as a table, the longest first as the app lists them.
func RenderApprovals(w io.Writer, r ApprovalsReply) error {
	if len(r.Approvals) == 0 {
		_, err := fmt.Fprintln(w, "Nobody is waiting for you.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tFLOOR/DESK\tWAITING\tPHONE\tWHAT")
	for _, a := range r.Approvals {
		name := Member{Nickname: a.Nickname, Name: a.Name}.displayName()
		place := deskLabel(Member{Desk: a.Desk, Floor: a.Floor})
		phone := "no"
		if a.Notified {
			phone = "told"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", name, place, orDash(formatIdle(a.WaitingSeconds)), phone, orDash(truncate(oneLine(a.Label), 60)))
	}
	return tw.Flush()
}

// RenderEvents prints one line per event, times in loc, then next=<id>.
func RenderEvents(w io.Writer, r EventsReply, loc *time.Location) error {
	for _, e := range r.Events {
		fields := []string{
			"#" + strconv.FormatInt(e.ID, 10),
			time.UnixMilli(int64(e.At)).In(loc).Format("15:04:05"),
			orDash(oneLine(e.Kind)),
			orDash(filepath.Base(oneLine(e.Folder))),
		}
		if e.Folder == "" {
			fields[3] = "-"
		}
		if d := oneLine(e.Detail); d != "" {
			fields = append(fields, d)
		}
		if _, err := fmt.Fprintln(w, strings.Join(fields, " ")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "next=%d\n", r.Next)
	return err
}

// RenderVersion prints the app, its API and its features.
func RenderVersion(w io.Writer, v VersionInfo) {
	name := oneLine(v.App)
	if name == "" {
		name = appName
	}
	if v.Version != "" {
		name += " " + oneLine(v.Version)
	}
	features := "none"
	if len(v.Features) > 0 {
		features = strings.Join(v.Features, ", ")
	}
	fmt.Fprintf(w, "%s (front desk API %d)\n", name, v.API)
	fmt.Fprintf(w, "features: %s\n", features)
}

// accountsPath is the app's Claude accounts route (T202).
const accountsPath = "/setup/claude-accounts"

// RenderAccounts prints the accounts route's answer as a table: each
// account's name, folder, whether it is connected, and which is the default.
// The list is {"accounts": [...]}, or {"profiles": [...]} as in the shared
// fixture; anything else the answer carries is ignored.
func RenderAccounts(w io.Writer, raw []byte) error {
	var a struct {
		Accounts []map[string]any `json:"accounts"`
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return decodeError(err)
	}
	list := a.Accounts
	if len(list) == 0 {
		list = a.Profiles
	}
	if len(list) == 0 {
		_, err := fmt.Fprintln(w, "No Claude accounts.")
		return err
	}
	text := func(m map[string]any, key string) string {
		if v, ok := m[key].(string); ok && oneLine(v) != "" {
			return oneLine(v)
		}
		return "-"
	}
	yesNo := func(m map[string]any, key string) string {
		v, ok := m[key].(bool)
		switch {
		case !ok:
			return "-"
		case v:
			return "yes"
		}
		return "no"
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tFOLDER\tCONNECTED\tDEFAULT")
	for _, m := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", text(m, "name"), text(m, "dir"), yesNo(m, "connected"), yesNo(m, "default"))
	}
	return tw.Flush()
}

// Accounts asks the accounts route; a 404 is turned into a plain sentence.
func Accounts(c *Client) ([]byte, error) {
	raw, err := c.Get(accountsPath, nil, requestTimeout)
	if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound && !notServedHere(err) {
		return nil, &Error{Code: exitApp, Msg: fmt.Sprintf("this %s has no accounts route yet; update it.", appName)}
	}
	return raw, err
}

// TileReply is what POST /setup/gm/tile answers.
type TileReply struct {
	Method          string   `json:"method"`
	Placed          []string `json:"placed"`
	Missed          []string `json:"missed"`
	NeedsPermission bool     `json:"needsPermission"`
	Message         string   `json:"message"`
}

// Tile asks the app to tile the team's windows. A 404 is turned into a plain sentence.
func Tile(c *Client) (TileReply, []byte, error) {
	raw, err := c.Post("/setup/gm/tile", nil, nil)
	if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound && !notServedHere(err) {
		return TileReply{}, nil, &Error{Code: exitApp, Msg: fmt.Sprintf("this %s can't tile windows yet; update it.", appName)}
	}
	if err != nil {
		return TileReply{}, nil, err
	}
	var r TileReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return TileReply{}, nil, decodeError(err)
	}
	return r, raw, nil
}

// RenderTile prints what tiling did, in a line or two.
func RenderTile(w io.Writer, r TileReply) {
	fmt.Fprintf(w, "%s (%d placed, %d not)\n", r.Message, len(r.Placed), len(r.Missed))
}

// ScreenRow is one screen in GET /setup/gm/screens.
type ScreenRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Main     bool   `json:"main"`
	Portrait bool   `json:"portrait"`
	Team     bool   `json:"team"`
}

// ScreensReply is what GET /setup/gm/screens and POST /setup/gm/screen answer.
type ScreensReply struct {
	Screens []ScreenRow `json:"screens"`
	Chosen  string      `json:"chosen"`
	Ask     bool        `json:"ask"`
}

func screensCommand(raw []byte, err error) (ScreensReply, []byte, error) {
	// A 404 is an app from before screens (an unknown id is answered 409).
	if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound && !notServedHere(err) {
		return ScreensReply{}, nil, &Error{Code: exitApp, Msg: fmt.Sprintf("this %s can't choose a screen yet; update it.", appName)}
	}
	if err != nil {
		return ScreensReply{}, nil, err
	}
	var r ScreensReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return ScreensReply{}, nil, decodeError(err)
	}
	return r, raw, nil
}

// Screens reads the screens and which one holds the team.
func Screens(c *Client) (ScreensReply, []byte, error) {
	raw, err := c.Get("/setup/gm/screens", nil, 0)
	return screensCommand(raw, err)
}

// ChooseScreen saves the screen that holds the team; the app checks the id.
func ChooseScreen(c *Client, id string) (ScreensReply, []byte, error) {
	raw, err := c.Post("/setup/gm/screen", url.Values{"id": {id}}, nil)
	return screensCommand(raw, err)
}

// RenderScreens prints one line a screen, the team's marked.
func RenderScreens(w io.Writer, r ScreensReply) {
	for _, s := range r.Screens {
		mark := " "
		if s.Team {
			mark = "*"
		}
		kind := ""
		if s.Main {
			kind += " main"
		}
		if s.Portrait {
			kind += " portrait"
		}
		fmt.Fprintf(w, "%s %s  %s%s\n", mark, s.ID, s.Name, kind)
	}
	if !r.Ask {
		fmt.Fprintln(w, "One screen: nothing to choose.")
	}
}

// prettyJSON indents an answer for --json, keeping it as served.
func prettyJSON(w io.Writer, raw []byte) error {
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimSpace(raw), "", "  "); err != nil {
		return decodeError(err)
	}
	out.WriteByte('\n')
	_, err := w.Write(out.Bytes())
	return err
}
