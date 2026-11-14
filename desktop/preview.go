package desktop

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The TV's preview address per desk (1.26 V1): the app a desk is building, as an address on this computer that the
// office's TV (V2) and its big preview (V3) will show. It comes two ways:
//   - declared: by the owner (the office window's Pixel Terminal, POST /desk/preview, or Setup, POST /setup/preview),
//     by the GM for one of its hires, or by a hire for its own desk alone (`smolcorp gm preview <url>`, both);
//   - found: among the TCP ports the desk's own processes listen on (its agent and everything the agent started),
//     read from the system's own tables (preview_<os>.go). Where those processes start is what the app itself
//     knows of the desk's tab (previewRootOf): its tmux pane's process, its kitty window's, its launcher's, or its
//     terminal's tty. Never a pid the hook sends, which any agent could set. Nothing is ever connected to, and no
//     other process's ports are looked at.
//
// Either way it is a loopback http address only, http://localhost:<port> or http://127.0.0.1:<port> with a path if
// you like, never Smolcorp's own port, and it is kept and shown as http://localhost:<port>: 127.0.0.1 is Setup's
// host alone, so a desk's page is never on it. The declared one wins; what was found is the suggestion. It is read in the
// office's own window alone (GET /desk/preview), never by a phone or the viewer, and never sent in events or a push.
// It is kept in memory, per desk, and goes with the desk.

const (
	previewFoundEvery = 10 * time.Second // what was found for a desk is kept this long
	previewRefreshGap = 2 * time.Second  // ?refresh=1 looks again, but not more often than this per desk
	previewURLMost    = 2048             // characters in a declared address
	previewTreeMost   = 512              // processes of one desk looked at
	previewFoundMost  = 8                // addresses found per desk
	previewKept       = 256              // desks remembered
)

// The words a declaration that can't be one is told.
const (
	previewOnly        = "A desk's preview is a loopback http address only: http://localhost:<port> or http://127.0.0.1:<port>, with a path if you like."
	previewHTTPS       = "That's https. " + previewOnly
	previewNotHTTP     = "That isn't an http address. " + previewOnly
	previewUserInfo    = "An address with a name or a password before its host can't be a preview. " + previewOnly
	previewNotLoopback = "That host isn't this computer's loopback. " + previewOnly
	previewNoPort      = "Say the port: http://localhost:3000, for one."
	previewQuery       = "A preview is an address and a path, without ? or #."
	previewBadEscape   = "That address has a % that isn't followed by two hex digits. " + previewOnly
	previewOwnPort     = "That's Smolcorp's own address, not a desk's preview."
	previewHireOwnOnly = "A hire may set only its own desk's preview."
	previewWhichDesk   = "Say which desk: \"session\" (its id) or \"hire\" (an id, name or nickname), one of them."
)

// previewURL is a declared address in one spelling, at localhost, or why it can't be a preview ("" and the words).
func previewURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if len(raw) > previewURLMost {
		return "", previewOnly
	}
	for _, r := range raw {
		// Printable ASCII only: no spaces, no control characters, no look-alike letters, no backslash a browser reads as /.
		if r <= ' ' || r >= 0x7f || r == '\\' {
			return "", previewOnly
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		if escape := url.EscapeError(""); errors.As(err, &escape) {
			return "", previewBadEscape
		}
		if scheme, _, found := strings.Cut(raw, "://"); found && strings.EqualFold(scheme, "http") {
			return "", previewNotLoopback
		}
		return "", previewNotHTTP
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
	case "https":
		return "", previewHTTPS
	default:
		return "", previewNotHTTP
	}
	if u.Opaque != "" || u.Host == "" {
		return "", previewNotHTTP
	}
	if u.User != nil {
		return "", previewUserInfo
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" {
		return "", previewNotLoopback
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != u.Port() {
		return "", previewNoPort
	}
	if smolcorpPort(port) {
		return "", previewOwnPort
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(raw, "?#") {
		return "", previewQuery
	}
	path := u.EscapedPath()
	if path != "" && !strings.HasPrefix(path, "/") {
		return "", previewOnly
	}
	// Always at localhost, never at 127.0.0.1: that is Setup's host (server.go), and a browser sends a host's
	// cookies to every port of it. localhost reaches a server listening at 127.0.0.1 all the same.
	return "http://localhost:" + strconv.Itoa(port) + path, ""
}

// smolcorpPort is whether port is this Smolcorp's, or one the real apps use (7710, 7720, 7730): each serves an office
// whose page holds its window's key, which a desk's preview must never frame. A test copy on a port of its own is
// left out: its port is any the owner picks, and the office it serves is a throwaway one.
func smolcorpPort(port int) bool { return port == Port || slices.Contains(realSmolcorpPorts, port) }

// tcpListener is one listening TCP socket: the address it listens on (unspecified for all of them) and its port.
type tcpListener struct {
	ip   net.IP
	port int
}

// previewURLsOf is the addresses a desk's listeners can be seen at here: each port listened on at the loopback or at
// every address, but Smolcorp's own, lowest first, once each, at most previewFoundMost.
func previewURLsOf(listeners []tcpListener) []string {
	var ports []int
	for _, l := range listeners {
		if l.port < 1 || l.port > 65535 || smolcorpPort(l.port) || slices.Contains(ports, l.port) {
			continue
		}
		if l.ip == nil || !(l.ip.IsUnspecified() || l.ip.Equal(net.IPv4(127, 0, 0, 1)) || l.ip.Equal(net.IPv6loopback)) {
			continue
		}
		ports = append(ports, l.port)
	}
	slices.Sort(ports)
	urls := []string{}
	for _, port := range ports[:min(len(ports), previewFoundMost)] {
		urls = append(urls, "http://localhost:"+strconv.Itoa(port))
	}
	return urls
}

// descendants is roots and every process under them, by parents (each process's parent), roots first, at most
// previewTreeMost. A process in cut, the start of another desk, is left out with everything under it: its ports are
// that desk's.
func descendants(roots []int, parents map[int]int, cut map[int]bool) []int {
	children := map[int][]int{}
	for pid, parent := range parents {
		if pid != parent {
			children[parent] = append(children[parent], pid)
		}
	}
	var tree []int
	seen := map[int]bool{}
	add := func(pid int) {
		if !seen[pid] && !cut[pid] && len(tree) < previewTreeMost {
			seen[pid] = true
			tree = append(tree, pid)
		}
	}
	for _, root := range roots {
		add(root)
	}
	for i := 0; i < len(tree) && len(tree) < previewTreeMost; i++ {
		kids := children[tree[i]]
		slices.Sort(kids)
		for _, kid := range kids {
			add(kid)
		}
	}
	return tree
}

// setOf is pids as a set.
func setOf(pids []int) map[int]bool {
	set := map[int]bool{}
	for _, pid := range pids {
		set[pid] = true
	}
	return set
}

// parseLsofListeners reads `lsof -F n` of listening TCP sockets: a line "n<host>:<port>" each ("*" for every
// address, an IPv6 one in brackets). Anything else is passed over.
func parseLsofListeners(out []byte) []tcpListener {
	var found []tcpListener
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line, isName := strings.CutPrefix(scanner.Text(), "n")
		if !isName || strings.Contains(line, "->") {
			continue
		}
		cut := strings.LastIndexByte(line, ':')
		if cut < 0 {
			continue
		}
		host, portText := strings.Trim(line[:cut], "[]"), line[cut+1:]
		port, err := strconv.Atoi(portText)
		if err != nil {
			continue
		}
		ip := net.ParseIP(host)
		if host == "*" {
			ip = net.IPv4zero
		}
		if ip != nil {
			found = append(found, tcpListener{ip: ip, port: port})
		}
	}
	return found
}

// parseProcNetTCP reads Linux's /proc/net/tcp or tcp6: the rows in the listen state (0A) whose socket is one of
// inodes.
func parseProcNetTCP(data []byte, inodes map[string]bool) []tcpListener {
	var found []tcpListener
	for i, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if i == 0 || len(fields) < 10 || fields[3] != "0A" || !inodes[fields[9]] {
			continue
		}
		if ip, port, ok := procAddress(fields[1]); ok {
			found = append(found, tcpListener{ip: ip, port: port})
		}
	}
	return found
}

// parseProcNetConnection is the socket (its inode) of the connection from one address to another, established (01),
// in Linux's /proc/net/tcp or tcp6, or "".
func parseProcNetConnection(data []byte, from, to *net.TCPAddr) string {
	for i, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if i == 0 || len(fields) < 10 || fields[3] != "01" {
			continue
		}
		localIP, localPort, ok1 := procAddress(fields[1])
		remoteIP, remotePort, ok2 := procAddress(fields[2])
		if ok1 && ok2 && localPort == from.Port && remotePort == to.Port && localIP.Equal(from.IP) && remoteIP.Equal(to.IP) {
			return fields[9]
		}
	}
	return ""
}

// procAddress reads one of /proc/net/tcp's addresses, <hex address>:<hex port>, each 32-bit word of the address in
// the machine's order (little-endian on what Smolcorp runs on).
func procAddress(field string) (net.IP, int, bool) {
	address, portText, ok := strings.Cut(field, ":")
	raw, err := hex.DecodeString(address)
	port, perr := strconv.ParseUint(portText, 16, 16)
	if !ok || err != nil || perr != nil || (len(raw) != 4 && len(raw) != 16) {
		return nil, 0, false
	}
	ip := make(net.IP, len(raw))
	for word := 0; word < len(raw); word += 4 {
		binary.BigEndian.PutUint32(ip[word:], binary.LittleEndian.Uint32(raw[word:]))
	}
	return ip, int(port), true
}

// parseLsofConnection is the process holding the connection from one address to another, from `lsof -F pn`: a line
// "p<pid>" for each process, then "n<from>-><to>" for each of its sockets. 0 for none.
func parseLsofConnection(out []byte, from, to string) int {
	pid, want := 0, "n"+from+"->"+to
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if text, isPID := strings.CutPrefix(line, "p"); isPID {
			pid, _ = strconv.Atoi(text)
		} else if line == want && pid > 0 {
			return pid
		}
	}
	return 0
}

// parseWindowsTCPTable reads GetExtendedTcpTable's answer for its listeners with their process
// (TCP_TABLE_OWNER_PID_LISTENER): a count, then rows, MIB_TCPROW_OWNER_PID (24 bytes) for IPv4 or
// MIB_TCP6ROW_OWNER_PID (56 bytes) for IPv6. Only the rows of pids are kept. An address is in network order; a port
// is too, in its word's first two bytes.
func parseWindowsTCPTable(table []byte, v6 bool, pids map[int]bool) []tcpListener {
	var found []tcpListener
	for _, row := range windowsTCPRows(table, v6) {
		if pids[row.pid] {
			found = append(found, tcpListener{ip: row.localIP, port: row.localPort})
		}
	}
	return found
}

// parseWindowsTCPConnection is the process holding the connection from one address to another, in
// GetExtendedTcpTable's answer for its connections with their process (TCP_TABLE_OWNER_PID_CONNECTIONS), or 0.
func parseWindowsTCPConnection(table []byte, v6 bool, from, to *net.TCPAddr) int {
	for _, row := range windowsTCPRows(table, v6) {
		if row.localPort == from.Port && row.remotePort == to.Port && row.localIP.Equal(from.IP) && row.remoteIP.Equal(to.IP) {
			return row.pid
		}
	}
	return 0
}

// windowsTCPRow is one row of GetExtendedTcpTable's answer with processes: its two ends, its state (MIB_TCP_STATE)
// and its process.
type windowsTCPRow struct {
	localIP, remoteIP     net.IP
	localPort, remotePort int
	state                 uint32
	pid                   int
}

// windowsTCPRows reads GetExtendedTcpTable's answer with processes: a count, then MIB_TCPROW_OWNER_PID rows (24 bytes:
// state, local address, local port, remote address, remote port, pid) or MIB_TCP6ROW_OWNER_PID ones (56 bytes:
// local address, its scope, local port, remote address, its scope, remote port, state, pid).
func windowsTCPRows(table []byte, v6 bool) []windowsTCPRow {
	if len(table) < 4 {
		return nil
	}
	size, addrLen, localAt, localPortAt, remoteAt, remotePortAt, stateAt, pidAt := 24, 4, 4, 8, 12, 16, 0, 20
	if v6 {
		size, addrLen, localAt, localPortAt, remoteAt, remotePortAt, stateAt, pidAt = 56, 16, 0, 20, 24, 44, 48, 52
	}
	count := int(binary.LittleEndian.Uint32(table))
	var rows []windowsTCPRow
	for i := 0; i < count; i++ {
		row := table[min(len(table), 4+i*size):]
		if len(row) < size {
			break
		}
		rows = append(rows, windowsTCPRow{
			localIP: slices.Clone(net.IP(row[localAt : localAt+addrLen])), remoteIP: slices.Clone(net.IP(row[remoteAt : remoteAt+addrLen])),
			localPort: int(binary.BigEndian.Uint16(row[localPortAt:])), remotePort: int(binary.BigEndian.Uint16(row[remotePortAt:])),
			state: binary.LittleEndian.Uint32(row[stateAt:]), pid: int(binary.LittleEndian.Uint32(row[pidAt:])),
		})
	}
	return rows
}

// previewRoot is where a desk's processes start, as the app itself knows the desk (previewRootOf), never as its hook
// says: a process (its tmux pane's, its kitty window's, its launcher's own), or the tty of its terminal tab, every
// process on which is the desk's. Neither when the app knows of none.
type previewRoot struct {
	pid int
	tty string
}

func (r previewRoot) known() bool { return r.pid > 1 || r.tty != "" }

// deviceTTY is a terminal's device, as a tab names it: a Mac's /dev/ttys003, Linux's /dev/pts/3.
var deviceTTY = regexp.MustCompile(`^/dev/(tty[A-Za-z0-9]+|pts/[0-9]+)$`)

// previewLooks is whether a tab is one the app can find the processes of: a tmux pane, a kitty window, a WezTerm pane
// or a terminal tab by its tty, a launcher. A headless run (its pid is only the hook's word), an iTerm2 session and a
// tab the app doesn't know are never looked at: what they listen on shows on the desk that started them, if any.
func previewLooks(tab string) bool {
	kind, _, _ := splitTab(tab)
	return slices.Contains([]string{"tmux", "kitty", "wezterm", "tty", "ghostty", "other", "launcher"}, kind)
}

// connected keeps the ends of the launcher's connection to this app: from (the launcher's) and to (the app's).
func (t *launcherTab) connected(from, to string) {
	t.conn.Lock()
	t.connFrom, t.connTo = from, to
	t.conn.Unlock()
}

// process is the launcher's own process, the one holding its connection to this app, as the system's tables say
// (connectionOwner), never as the launcher says: an agent with the hook key could connect as a launcher and name any
// process. It is found once, as a connection keeps its process; while the launcher is connected, its tab is here,
// so the process can't have ended and its pid been given to another. 0 when it can't be found.
func (t *launcherTab) process() int {
	t.conn.Lock()
	defer t.conn.Unlock()
	if t.pid == 0 && t.connFrom != "" {
		from, err := net.ResolveTCPAddr("tcp", t.connFrom)
		to, terr := net.ResolveTCPAddr("tcp", t.connTo)
		if err == nil && terr == nil && from.IP != nil && to.IP != nil {
			t.pid = connectionOwner(from, to)
		}
	}
	return t.pid
}

// previewRootOf is where a desk's processes start, from its tab, asked of its terminal now: a tmux pane's own
// process (#{pane_pid}, while the pane is still that one), a kitty window's, the tty of a WezTerm pane or of a
// terminal tab, or a launcher's own process, found from its connection to this app (launcherTab.process). Never
// from the hook.
func previewRootOf(tab string, launcher *launcherTab) previewRoot {
	kind, id, socket := splitTab(tab)
	pane := commandTab{kind: kind, id: id, socket: socket}
	switch kind {
	case "tmux":
		out, err := pane.output("display-message", "-p", "-t", id, "#{pane_id} #{pane_pid}")
		shown, pidText, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
		if pid, perr := strconv.Atoi(pidText); err == nil && perr == nil && shown == id {
			return previewRoot{pid: pid}
		}
	case "kitty":
		if window, err := pane.kittyWindow(); err == nil && window != nil {
			return previewRoot{pid: window.PID}
		}
	case "wezterm":
		if tty := pane.tty(); deviceTTY.MatchString(tty) {
			return previewRoot{tty: tty}
		}
	case "tty", "ghostty", "other":
		if deviceTTY.MatchString(id) {
			return previewRoot{tty: id}
		}
	case "launcher":
		if launcher != nil {
			return previewRoot{pid: launcher.process()}
		}
	}
	return previewRoot{}
}

// previewDesk is what is known of one desk's preview.
type previewDesk struct {
	declared, by string
	found        []string
	foundAt      time.Time
	foundTab     string      // the tab the addresses were found for
	foundRoot    previewRoot // and where its processes started then
}

// previewStore is every desk's preview, by session id. look is the system's lookup (deskListeners), one at a time;
// root is where a desk's processes start (previewRootOf).
type previewStore struct {
	mu     sync.Mutex
	desks  map[string]*previewDesk
	lookup sync.Mutex
	look   func(root previewRoot, cut []int) ([]tcpListener, bool)
	root   func(tab string, launcher *launcherTab) previewRoot
	now    func() time.Time
}

var previews = &previewStore{desks: map[string]*previewDesk{}, look: deskListeners, root: previewRootOf, now: time.Now}

// entry is a desk's, made if there is none, with p.mu held. With previewKept desks known, the ones with nothing
// declared are let go first: what was found is only kept to be cheap.
func (p *previewStore) entry(session string) *previewDesk {
	if e := p.desks[session]; e != nil {
		return e
	}
	if len(p.desks) >= previewKept {
		for id, e := range p.desks {
			if e.declared == "" {
				delete(p.desks, id)
			}
		}
	}
	e := &previewDesk{}
	p.desks[session] = e
	return e
}

// cachedFound is what was found for a desk on tab, if it is fresh enough to answer with, or (stale) at all.
func (p *previewStore) cachedFound(session, tab string, refresh, stale bool) ([]string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.desks[session]
	if e == nil || e.foundAt.IsZero() || e.foundTab != tab {
		return nil, false
	}
	age := p.now().Sub(e.foundAt)
	if stale || age < previewRefreshGap || age < previewFoundEvery && !refresh {
		return slices.Clone(e.found), true
	}
	return nil, false
}

// otherRoots is the processes other desks were last found to start at, with p.mu held: a desk's own tree leaves them
// out, as a desk can start another (a launcher run from a terminal tab that is a desk itself).
func (p *previewStore) otherRoots(session string) []int {
	var roots []int
	for id, e := range p.desks {
		if id != session && e.foundRoot.pid > 1 {
			roots = append(roots, e.foundRoot.pid)
		}
	}
	return roots
}

// found is the addresses a desk's processes listen on: kept ones while fresh, else looked up, one lookup at a time.
// A poll that would wait for another desk's lookup answers with what it has, however old, if it has anything. A desk
// whose tab the app can't find processes for has none, and nothing is looked at.
func (p *previewStore) found(session, tab string, launcher *launcherTab, refresh bool) []string {
	if !previewLooks(tab) {
		return []string{}
	}
	if urls, ok := p.cachedFound(session, tab, refresh, false); ok {
		return urls
	}
	p.mu.Lock()
	asked, look, rootOf := p.now(), p.look, p.root
	p.mu.Unlock()
	if !p.lookup.TryLock() {
		if urls, ok := p.cachedFound(session, tab, refresh, true); ok {
			return urls
		}
		p.lookup.Lock()
	}
	defer p.lookup.Unlock()
	// Another poll may have looked while this one waited: its answer is as new as this one's would be.
	p.mu.Lock()
	if e := p.desks[session]; e != nil && e.foundTab == tab && !e.foundAt.IsZero() && !e.foundAt.Before(asked) {
		urls := slices.Clone(e.found)
		p.mu.Unlock()
		return urls
	}
	cut := p.otherRoots(session)
	p.mu.Unlock()
	urls, root := []string{}, rootOf(tab, launcher)
	if root.known() {
		if listeners, ok := look(root, cut); ok {
			urls = previewURLsOf(listeners)
		}
	}
	p.mu.Lock()
	e := p.entry(session)
	e.found, e.foundAt, e.foundTab, e.foundRoot = urls, p.now(), tab, root
	p.mu.Unlock()
	return slices.Clone(urls)
}

// sweep lets the desks that have gone (live says no) go, with p.mu held.
func (p *previewStore) sweep(live func(string) bool) {
	for id := range p.desks {
		if !live(id) {
			delete(p.desks, id)
		}
	}
}

// declare keeps (or, with url "", takes away) a desk's declared address, by who ("owner", "gm" or "hire"). Desks
// that have gone (live says no) are let go first.
func (p *previewStore) declare(session, address, by string, live func(string) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sweep(live)
	e := p.entry(session)
	e.declared, e.by = address, by
	if address == "" {
		e.by = ""
	}
}

// forget lets a desk that has gone go.
func (p *previewStore) forget(session string) {
	p.mu.Lock()
	delete(p.desks, session)
	p.mu.Unlock()
}

// previewAnswer is GET /desk/preview's answer: the address to show ("" for none) and where it came from.
type previewAnswer struct {
	URL        string   `json:"url"`
	Source     string   `json:"source"` // "declared", "found" or ""
	Declared   string   `json:"declared,omitempty"`
	DeclaredBy string   `json:"declaredBy,omitempty"`
	Found      []string `json:"found"`
}

func (p *previewStore) answer(session, tab string, launcher *launcherTab, refresh bool) previewAnswer {
	answer := previewAnswer{Found: p.found(session, tab, launcher, refresh)}
	p.mu.Lock()
	if e := p.desks[session]; e != nil {
		answer.Declared, answer.DeclaredBy = e.declared, e.by
	}
	p.mu.Unlock()
	switch {
	case answer.Declared != "":
		answer.URL, answer.Source = answer.Declared, "declared"
	case len(answer.Found) > 0:
		answer.URL, answer.Source = answer.Found[0], "found"
	}
	return answer
}

// deskPreview is a desk's preview, for the window. The desks that have gone since are let go as it's asked, so a
// gone desk's declaration lingers no longer than the window's next poll.
func (d *Deck) deskPreview(session string, refresh bool) (int, any) {
	d.mu.Lock()
	s := d.sessions.Get(session)
	tab := ""
	var launcher *launcherTab
	if s != nil {
		tab = s.Tab
		if kind, id, _ := splitTab(tab); kind == "launcher" {
			launcher = d.launchers[id]
		}
	}
	previews.mu.Lock()
	previews.sweep(func(id string) bool { return d.sessions.Get(id) != nil })
	previews.mu.Unlock()
	d.mu.Unlock()
	if s == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	return http.StatusOK, previews.answer(session, tab, launcher, refresh)
}

// hireDesk is the session of the desk a hire sits at, as the team shows it (buildTeam, with every hire, so a desk
// another hire claims first is never this one's), or "".
func (d *Deck) hireDesk(h Hire) string {
	hires := readRegistry(registryPath(), false).hires
	for _, row := range buildTeam(hires, d.frontDeskLive(), 0, func(string) string { return "" }) {
		if id, _ := row["sessionId"].(string); row["id"] == h.ID && id != "" {
			return id
		}
	}
	return ""
}

// previewAsk is a declaration: the address ("" takes it away) and the desk, by session or by hire.
type previewAsk struct {
	url, session, hire string
}

// declarePreview keeps a declared address for a desk. by is who asks: "owner" (the window or the page), "gm", or
// "hire" with signer, the hire its send key was made for, which may name only its own desk.
func (d *Deck) declarePreview(by string, signer *Hire, ask previewAsk) (int, any) {
	address, why := "", ""
	if strings.TrimSpace(ask.url) != "" {
		if address, why = previewURL(ask.url); why != "" {
			return http.StatusBadRequest, why
		}
	}
	var session string
	switch {
	case by == "hire":
		if signer == nil {
			return http.StatusForbidden, previewHireOwnOnly
		}
		own := d.hireDesk(*signer)
		if ask.hire != "" {
			if h, refused := findWatchHire(ask.hire, readRegistry(registryPath(), false).hires); refused != nil || h.ID != signer.ID {
				return http.StatusForbidden, previewHireOwnOnly
			}
		}
		if ask.session != "" && ask.session != own {
			return http.StatusForbidden, previewHireOwnOnly
		}
		if own == "" {
			return http.StatusConflict, "Your desk isn't in the office right now, so there's nowhere to show it."
		}
		session = own
	case (ask.session == "") == (ask.hire == ""):
		return http.StatusBadRequest, previewWhichDesk
	case ask.hire != "":
		h, refused := findWatchHire(ask.hire, readRegistry(registryPath(), false).hires)
		if refused != nil {
			return refused.status, refused.message
		}
		if session = d.hireDesk(h); session == "" {
			return http.StatusConflict, mailHireName(h) + " has no desk in the office right now."
		}
	default:
		session = ask.session
	}
	d.mu.Lock()
	known := d.sessions.Get(session) != nil
	live := func(id string) bool { return d.sessions.Get(id) != nil }
	if known {
		previews.declare(session, address, by, live)
	}
	d.mu.Unlock()
	if !known {
		return http.StatusNotFound, "That agent has gone."
	}
	return http.StatusOK, map[string]string{"session": session, "url": address}
}

// setupPreview is POST /setup/preview, {url, session | hire}: the owner from Setup, the GM for a hire, a hire for its
// own desk (goOnlyRoutes lets in page, gm and hire). It is outside /setup/gm/ so that the owner can declare from
// Setup with the manager off; the GM and a hire are told 409 then, as on every route of theirs.
func (rt *router) setupPreview(w http.ResponseWriter, r *request) {
	rt.deck.mu.Lock()
	managerOn := rt.deck.config.ManagerEnabled
	rt.deck.mu.Unlock()
	if (r.caller == callerGM || r.caller == callerHire) && !managerOn {
		writeStatus(w)(http.StatusConflict, managerOffAnswer)
		return
	}
	asked := gmAsked(r)
	ask := previewAsk{url: stringValue(asked["url"]), session: stringValue(asked["session"]), hire: stringValue(asked["hire"])}
	switch r.caller {
	case callerPage:
		writeAnswer(w)(rt.deck.declarePreview("owner", nil, ask))
	case callerGM:
		writeAnswer(w)(rt.deck.declarePreview("gm", nil, ask))
	case callerHire:
		// A hire is whoever its send key was made for, found again here, never by anything it says.
		signer := readRegistry(registryPath(), false).hireForSendKey(r.Header.Get(hireKeyHeader))
		writeAnswer(w)(rt.deck.declarePreview("hire", signer, ask))
	default:
		writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
	}
}

// The preview, in the office's own window on this computer alone, never a phone (phoneroutes.go): GET to read a
// desk's (?refresh=1 looks again), POST {url} for the owner to declare one from the Pixel Terminal.
func init() {
	refusal := func(d *Deck) string { return d.shownOnlyInTheWindow("The preview") }
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/desk/preview", access: accessWindow, refusal: refusal,
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeAnswer(w)(d.deskPreview(session, r.URL.Query().Get("refresh") == "1"))
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/desk/preview", access: accessWindow, refusal: refusal,
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			if session == "" {
				writeStatus(w)(http.StatusBadRequest, previewWhichDesk)
				return
			}
			writeAnswer(w)(d.declarePreview("owner", nil, previewAsk{url: stringValue(decodeJSON(r.Body)["url"]), session: session}))
		}})
}
