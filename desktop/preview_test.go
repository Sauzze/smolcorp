package desktop

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// The TV's preview address per desk (1.26 V1): declared or found, loopback http only, window only.

// A declared address is a loopback http address and nothing else; what can't be one is told why, in words.
func TestAPreviewAddressIsLoopbackHTTPOnly(t *testing.T) {
	for given, want := range map[string]string{
		"http://localhost:3000":           "http://localhost:3000",
		"http://127.0.0.1:5173/app/":      "http://localhost:5173/app/", // never on Setup's host
		"http://127.0.0.1:5173":           "http://localhost:5173",
		"  http://LOCALHOST:8080/a%20b  ": "http://localhost:8080/a%20b",
		"HTTP://localhost:3000":           "http://localhost:3000",
		"http://localhost:65535/":         "http://localhost:65535/",
	} {
		if got, why := previewURL(given); got != want || why != "" {
			t.Errorf("%q: got %q %q, want %q", given, got, why, want)
		}
	}
	for given, why := range map[string]string{
		"https://localhost:3000":                                       previewHTTPS,
		"HTTPS://127.0.0.1:3000":                                       previewHTTPS,
		"ftp://localhost:3000":                                         previewNotHTTP,
		"file:///etc/passwd":                                           previewNotHTTP,
		"javascript:alert(1)":                                          previewNotHTTP,
		"data:text/html,hi":                                            previewNotHTTP,
		"ws://localhost:3000":                                          previewNotHTTP,
		"localhost:3000":                                               previewNotHTTP,
		"//localhost:3000":                                             previewNotHTTP,
		"http:localhost:3000":                                          previewNotHTTP,
		"http:/localhost:3000":                                         previewNotHTTP,
		"http://example.com:3000":                                      previewNotLoopback,
		"http://192.168.1.10:3000":                                     previewNotLoopback,
		"http://10.0.0.1:3000":                                         previewNotLoopback,
		"http://0.0.0.0:3000":                                          previewNotLoopback,
		"http://[::1]:3000":                                            previewNotLoopback,
		"http://127.0.0.2:3000":                                        previewNotLoopback,
		"http://127.1:3000":                                            previewNotLoopback,
		"http://2130706433:3000":                                       previewNotLoopback,
		"http://0x7f.0.0.1:3000":                                       previewNotLoopback,
		"http://localhost.:3000":                                       previewNotLoopback,
		"http://localhost.evil.com:3000":                               previewNotLoopback,
		"http://127.0.0.1.nip.io:3000":                                 previewNotLoopback,
		"http://evil-localhost:3000":                                   previewNotLoopback,
		"http://localhost:3000.evil.com/":                              previewNotLoopback,
		"http://127.0.0.1:3000.evil.com/":                              previewNotLoopback,
		"http://localhost:3000@evil.com/":                              previewUserInfo,
		"http://user:pw@localhost:3000/":                               previewUserInfo,
		"http://evil.com#@localhost:3000":                              previewNotLoopback,
		"http://evil.com?@localhost:3000":                              previewNotLoopback,
		"http://localhost":                                             previewNoPort,
		"http://localhost:":                                            previewNoPort,
		"http://localhost:0":                                           previewNoPort,
		"http://localhost:03000":                                       previewNoPort,
		"http://localhost:65536":                                       previewNoPort,
		"http://localhost:" + strconv.Itoa(Port):                       previewOwnPort,
		"http://localhost:3000/?a=1":                                   previewQuery,
		"http://localhost:3000/#top":                                   previewQuery,
		"http://localhost:3000/?":                                      previewQuery,
		"http://lоcalhost:3000":                                        previewOnly, // a Cyrillic o
		"http://localhost:3000/a b":                                    previewOnly,
		"http://localhost:3000\\@evil.com":                             previewOnly,
		"http://localhost:3000/\n":                                     "", // trimmed: a line break around it is no harm
		"http://localhost:3000/a\x00":                                  previewOnly,
		"http://localhost:3000/" + strings.Repeat("a", previewURLMost): previewOnly,
	} {
		got, said := previewURL(given)
		if why == "" {
			if said != "" {
				t.Errorf("%q: refused %q", given, said)
			}
			continue
		}
		if got != "" || said != why {
			t.Errorf("%q: got %q %q, want refused with %q", given, got, said, why)
		}
	}
}

// What the system's tables say is read the same way on each: lsof's on a Mac, /proc's on Linux, the IP helper's
// on Windows. Only addresses that reach this computer's loopback make an address, never Smolcorp's own port.
func TestTheSystemsListenerTablesAreRead(t *testing.T) {
	lsof := []byte("p4242\nf21\nn*:5173\nf22\nn127.0.0.1:3000\nf23\nn[::1]:8080\nf24\nn192.168.1.10:9000\nf25\nn127.0.0.1:3000->127.0.0.1:5000\nnjunk\n")
	if got := previewURLsOf(parseLsofListeners(lsof)); !slices.Equal(got, []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}) {
		t.Errorf("lsof: %v", got)
	}
	proc := []byte(`  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 111 1 0000000000000000 100 0 0 10 0
   1: 00000000:1435 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 222 1 0000000000000000 100 0 0 10 0
   2: 0A01A8C0:2328 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 333 1 0000000000000000 100 0 0 10 0
   3: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 999 1 0000000000000000 100 0 0 10 0
   4: 0100007F:0FA0 0100007F:C350 01 00000000:00000000 00:00000000 00000000  1000        0 444 1 0000000000000000 100 0 0 10 0
`)
	proc6 := []byte(`  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:1F91 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 555 1 0000000000000000 100 0 0 10 0
`)
	ours := map[string]bool{"111": true, "222": true, "333": true, "444": true, "555": true} // 999 is someone else's
	found := append(parseProcNetTCP(proc, ours), parseProcNetTCP(proc6, ours)...)
	if got := previewURLsOf(found); !slices.Equal(got, []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8081"}) {
		t.Errorf("/proc: %v (%v)", got, found)
	}
	row4 := func(addr [4]byte, port uint16, pid uint32) []byte {
		row := make([]byte, 24)
		binary.LittleEndian.PutUint32(row, 2) // listening
		copy(row[4:], addr[:])
		binary.BigEndian.PutUint16(row[8:], port)
		binary.LittleEndian.PutUint32(row[20:], pid)
		return row
	}
	table := binary.LittleEndian.AppendUint32(nil, 4)
	table = append(table, row4([4]byte{127, 0, 0, 1}, 3000, 10)...)
	table = append(table, row4([4]byte{0, 0, 0, 0}, 5173, 11)...)
	table = append(table, row4([4]byte{127, 0, 0, 1}, 9999, 99)...) // another process's
	table = append(table, row4([4]byte{192, 168, 1, 10}, 9000, 10)...)
	row6 := make([]byte, 56)
	row6[15] = 1 // ::1
	binary.BigEndian.PutUint16(row6[20:], 8080)
	binary.LittleEndian.PutUint32(row6[52:], 11)
	table6 := append(binary.LittleEndian.AppendUint32(nil, 2), row6...) // says 2, holds 1: the short one is passed over
	pids := map[int]bool{10: true, 11: true}
	found = append(parseWindowsTCPTable(table, false, pids), parseWindowsTCPTable(table6, true, pids)...)
	if got := previewURLsOf(found); !slices.Equal(got, []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"}) {
		t.Errorf("Windows: %v", got)
	}
	if got := previewURLsOf([]tcpListener{{net.IPv4zero, Port}, {net.IPv4(127, 0, 0, 1), 0}}); len(got) != 0 {
		t.Errorf("Smolcorp's own port, or none: %v", got)
	}
	for _, port := range realSmolcorpPorts {
		if got := previewURLsOf([]tcpListener{{net.IPv4zero, port}}); len(got) != 0 {
			t.Errorf("a real Smolcorp's port %d: %v", port, got)
		}
		if _, why := previewURL("http://localhost:" + strconv.Itoa(port)); why != previewOwnPort {
			t.Errorf("a real Smolcorp's port %d declared: %q", port, why)
		}
	}
	// A desk's processes are its agent and everything under it, never its parent or a neighbour; nor another desk
	// started under it (cut), nor anything under that one.
	parents := map[int]int{100: 1, 101: 100, 102: 101, 200: 1, 103: 100, 1: 0, 104: 103, 105: 104}
	if got := descendants([]int{100}, parents, nil); !slices.Equal(got, []int{100, 101, 103, 102, 104, 105}) {
		t.Errorf("descendants: %v", got)
	}
	if got := descendants([]int{100, 200}, parents, map[int]bool{103: true}); !slices.Equal(got, []int{100, 200, 101, 102}) {
		t.Errorf("descendants of two roots, one desk cut: %v", got)
	}
}

// A launcher's own process is the one holding its connection to the app, as the system's tables say: lsof's on a
// Mac, /proc's on Linux, the IP helper's on Windows. Only that connection, in that direction, says so.
func TestTheProcessHoldingAConnectionIsRead(t *testing.T) {
	from, to := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50000}, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7710}
	lsof := []byte("p900\nf8\nn127.0.0.1:7710->127.0.0.1:50000\np901\nf3\nn127.0.0.1:50001->127.0.0.1:7710\np902\nf5\nn127.0.0.1:50000->127.0.0.1:7710\n")
	if got := parseLsofConnection(lsof, from.String(), to.String()); got != 902 {
		t.Errorf("lsof: %d, want 902", got)
	}
	if got := parseLsofConnection(lsof, "127.0.0.1:50002", to.String()); got != 0 {
		t.Errorf("lsof, no such connection: %d", got)
	}
	proc := []byte(`  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1E1E 0100007F:C350 01 00000000:00000000 00:00000000 00000000  1000        0 111 1 0000000000000000 100 0 0 10 0
   1: 0100007F:C350 0100007F:1E1E 0A 00000000:00000000 00:00000000 00000000  1000        0 222 1 0000000000000000 100 0 0 10 0
   2: 0100007F:C350 0100007F:1E1E 01 00000000:00000000 00:00000000 00000000  1000        0 333 1 0000000000000000 100 0 0 10 0
`)
	if got := parseProcNetConnection(proc, from, to); got != "333" {
		t.Errorf("/proc: %q, want 333", got)
	}
	row := func(state uint32, local, remote int, pid uint32) []byte {
		r := make([]byte, 24)
		binary.LittleEndian.PutUint32(r, state)
		copy(r[4:], []byte{127, 0, 0, 1})
		binary.BigEndian.PutUint16(r[8:], uint16(local))
		copy(r[12:], []byte{127, 0, 0, 1})
		binary.BigEndian.PutUint16(r[16:], uint16(remote))
		binary.LittleEndian.PutUint32(r[20:], pid)
		return r
	}
	table := binary.LittleEndian.AppendUint32(nil, 2)
	table = append(append(table, row(5, 7710, 50000, 900)...), row(5, 50000, 7710, 902)...)
	if got := parseWindowsTCPConnection(table, false, from, to); got != 902 {
		t.Errorf("Windows: %d, want 902", got)
	}
}

// GetExtendedTcpTable's rows with processes are read with their state too (OLD-SERVE2's serve port asks it), at its
// own place in each layout: first in an IPv4 row, after the remote port in an IPv6 one.
func TestWindowsTCPRowsSayTheirState(t *testing.T) {
	row4 := make([]byte, 24)
	binary.LittleEndian.PutUint32(row4, 5) // established
	copy(row4[4:], []byte{127, 0, 0, 1})
	binary.BigEndian.PutUint16(row4[8:], 50000)
	copy(row4[12:], []byte{127, 0, 0, 1})
	binary.BigEndian.PutUint16(row4[16:], 7711)
	binary.LittleEndian.PutUint32(row4[20:], 4120)
	rows := windowsTCPRows(append(binary.LittleEndian.AppendUint32(nil, 1), row4...), false)
	if len(rows) != 1 || rows[0].state != 5 || rows[0].pid != 4120 || rows[0].localPort != 50000 || rows[0].remotePort != 7711 ||
		!rows[0].localIP.Equal(net.IPv4(127, 0, 0, 1)) || !rows[0].remoteIP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("IPv4: %+v", rows)
	}
	row6 := make([]byte, 56)
	row6[15], row6[39] = 1, 1 // ::1 to ::1
	binary.BigEndian.PutUint16(row6[20:], 50001)
	binary.BigEndian.PutUint16(row6[44:], 7711)
	binary.LittleEndian.PutUint32(row6[48:], 7) // FIN_WAIT_2: half-closed
	binary.LittleEndian.PutUint32(row6[52:], 4120)
	rows = windowsTCPRows(append(binary.LittleEndian.AppendUint32(nil, 1), row6...), true)
	if len(rows) != 1 || rows[0].state != 7 || rows[0].pid != 4120 || rows[0].localPort != 50001 || rows[0].remotePort != 7711 ||
		!rows[0].localIP.Equal(net.IPv6loopback) || !rows[0].remoteIP.Equal(net.IPv6loopback) {
		t.Errorf("IPv6: %+v", rows)
	}
}

// previewDeck is the mail deck's hires, GM and keys, with a desk for Rev (in its worktree) and one for the Janitor.
func previewDeck(t *testing.T) *mailDeck {
	t.Helper()
	m := newMailDeck(t)
	m.d.mu.Lock()
	m.d.sessions.byID["pv-rev"] = &Session{ID: "pv-rev", Agent: Claude, Folder: "/work/app-wt/rev", Project: "/work/app-wt/rev", StartedAt: 1}
	m.d.sessions.byID["pv-jan"] = &Session{ID: "pv-jan", Agent: Claude, Folder: "/work/api", Project: "/work/api", StartedAt: 2}
	m.d.mu.Unlock()
	t.Cleanup(func() { previews.forget("pv-rev"); previews.forget("pv-jan") })
	return m
}

// windowPreview is GET /desk/preview in the office's own window.
func windowPreview(t *testing.T, d *Deck, session string) (int, previewAnswer) {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/desk/preview?session=" + session + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	var a previewAnswer
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
			t.Fatalf("not an answer: %s", w.Body)
		}
	}
	return w.Code, a
}

// Declared by the owner (the window or Setup), the GM for a hire, or a hire for its own desk alone; taken away with "".
func TestAPreviewIsDeclaredForADesk(t *testing.T) {
	m := previewDeck(t)
	d := m.d
	window := func(session, body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/desk/preview?session=" + session + "&t=" + d.access.WindowKey, host: "localhost:7710",
			from: thisComputer, body: body})
	}
	if code, a := windowPreview(t, d, "pv-rev"); code != 200 || a.URL != "" || a.Source != "" || len(a.Found) != 0 {
		t.Fatalf("before: %d %+v", code, a)
	}
	if w := window("pv-rev", `{"url":"http://localhost:3000/app"}`); w.Code != 200 {
		t.Fatalf("the window: %d %s", w.Code, w.Body)
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:3000/app" || a.Source != "declared" || a.DeclaredBy != "owner" {
		t.Errorf("declared by the owner: %+v", a)
	}
	if w := window("pv-rev", `{"url":"https://localhost:3000"}`); w.Code != 400 || strings.TrimSpace(w.Body.String()) != previewHTTPS {
		t.Errorf("https: %d %s", w.Code, w.Body)
	}
	if w := window("gone", `{"url":"http://localhost:3000"}`); w.Code != 404 {
		t.Errorf("a desk that isn't there: %d", w.Code)
	}

	post := func(as who, body string) (int, string) {
		w := m.call(t, as, "POST", "/setup/preview", body)
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	// Setup, by the desk's session.
	if code, text := post(asPage, `{"url":"http://127.0.0.1:4100","session":"pv-jan"}`); code != 200 || !strings.Contains(text, `"session":"pv-jan"`) {
		t.Errorf("the page: %d %s", code, text)
	}
	if _, a := windowPreview(t, d, "pv-jan"); a.URL != "http://localhost:4100" || a.DeclaredBy != "owner" {
		t.Errorf("the page's: %+v", a)
	}
	// The GM, for a hire by its name: that hire's desk.
	if code, text := post(asGM, `{"url":"http://localhost:4101","hire":"Rev"}`); code != 200 || !strings.Contains(text, `"session":"pv-rev"`) {
		t.Errorf("the GM: %d %s", code, text)
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:4101" || a.DeclaredBy != "gm" {
		t.Errorf("the GM's: %+v", a)
	}
	for body, want := range map[string]int{
		`{"url":"http://localhost:4101"}`:                                 400, // which desk?
		`{"url":"http://localhost:4101","hire":"Rev","session":"pv-rev"}`: 400,
		`{"url":"http://localhost:4101","hire":"Nobody"}`:                 404,
		`{"url":"http://localhost:4101","hire":"Twin"}`:                   409, // two of that name: the id
		`{"url":"http://localhost:4101","session":"gone"}`:                404,
		`{"url":"http://example.com:4101","hire":"Rev"}`:                  400,
	} {
		if code, text := post(asGM, body); code != want || text == "" {
			t.Errorf("the GM's %s: %d %q, want %d", body, code, text, want)
		}
	}
	// A hire, for its own desk: without naming it, or naming itself.
	if code, text := post(asRev, `{"url":"http://localhost:4102"}`); code != 200 || !strings.Contains(text, `"session":"pv-rev"`) {
		t.Errorf("Rev: %d %s", code, text)
	}
	if code, _ := post(asRev, `{"url":"http://localhost:4103","hire":"a1b2c3","session":"pv-rev"}`); code != 200 {
		t.Errorf("Rev naming itself: %d", code)
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:4103" || a.DeclaredBy != "hire" {
		t.Errorf("Rev's: %+v", a)
	}
	// ... and never another desk, by hire or by session: refused, and nothing changes.
	for _, body := range []string{`{"url":"http://localhost:4104","hire":"Janitor"}`, `{"url":"http://localhost:4104","hire":"d4e5f6"}`,
		`{"url":"http://localhost:4104","session":"pv-jan"}`, `{"url":"","session":"pv-jan"}`, `{"url":"http://localhost:4104","hire":"Nobody"}`} {
		if code, text := post(asRev, body); code != 403 || text != previewHireOwnOnly {
			t.Errorf("Rev for another desk, %s: %d %q", body, code, text)
		}
	}
	if _, a := windowPreview(t, d, "pv-jan"); a.URL != "http://localhost:4100" || a.DeclaredBy != "owner" {
		t.Errorf("the Janitor's desk changed: %+v", a)
	}
	// A hire with no desk in the office has nowhere to declare.
	if code, _ := post(asTwin, `{"url":"http://localhost:4105"}`); code != 409 {
		t.Errorf("Twin, with no desk: %d", code)
	}
	// A tool and a stranger: the route table's no.
	if code, _ := post(asTool, `{"url":"http://localhost:4105","session":"pv-rev"}`); code != 403 {
		t.Errorf("a tool: %d", code)
	}
	// Taken away: the TV shows what is found, here nothing.
	if code, _ := post(asRev, `{"url":""}`); code != 200 {
		t.Errorf("Rev takes it away: %d", code)
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "" || a.Declared != "" || a.DeclaredBy != "" {
		t.Errorf("after: %+v", a)
	}
	// A desk that goes takes its preview with it.
	d.mu.Lock()
	delete(d.sessions.byID, "pv-jan")
	d.mu.Unlock()
	if code, _ := windowPreview(t, d, "pv-jan"); code != 404 {
		t.Errorf("gone: %d", code)
	}
	previews.mu.Lock()
	_, kept := previews.desks["pv-jan"]
	previews.mu.Unlock()
	if kept {
		t.Error("a gone desk's preview is kept")
	}
}

// Read and declared in the office's own window alone: a phone is told so, and the GM, a hire and the page have no
// way to read it.
func TestThePreviewIsWindowOnly(t *testing.T) {
	m := previewDeck(t)
	d := m.d
	if code := m.call(t, asPage, "POST", "/setup/preview", `{"url":"http://localhost:3000","session":"pv-rev"}`).Code; code != 200 {
		t.Fatalf("declared: %d", code)
	}
	phone := pair(t, d)
	for _, method := range []string{"GET", "POST"} {
		// A GET goes without a body, as a phone's does: signed.go refuses a GET that comes with one (1.25).
		body := `{"url":"http://localhost:4000"}`
		if method == "GET" {
			body = ""
		}
		w := serve(t, d, call{method: method, target: "/desk/preview?session=pv-rev&t=" + phone, host: deckOnWiFi, from: aPhone, body: body})
		if w.Code != http.StatusForbidden || strings.TrimSpace(w.Body.String()) != "The preview shows only in the office's window on your computer." ||
			strings.Contains(w.Body.String(), "localhost:3000") {
			t.Errorf("a phone's %s: %d %s", method, w.Code, w.Body)
		}
	}
	for _, as := range []who{asGM, asRev, asPage, asTool} {
		if w := m.call(t, as, "GET", "/desk/preview?session=pv-rev", ""); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "localhost:3000") {
			t.Errorf("caller %d read it: %d %s", as, w.Code, w.Body)
		}
		if w := m.call(t, as, "GET", "/setup/preview?session=pv-rev", ""); strings.Contains(w.Body.String(), "localhost:3000") {
			t.Errorf("caller %d read it through setup: %d %s", as, w.Code, w.Body)
		}
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:3000" {
		t.Errorf("a phone changed it: %+v", a)
	}
}

// Cheap: what was found is kept, and a poll within previewFoundEvery doesn't look again; ?refresh=1 does, but not
// more often than previewRefreshGap. A desk on a tab the app finds no processes for isn't looked at at all; one
// whose tab changes is looked at again. While another desk's lookup runs, a poll answers with what it has.
func TestAFoundPreviewIsKeptAndRefreshedOnDemand(t *testing.T) {
	m := previewDeck(t)
	d := m.d
	clock := previews.now()
	looks, roots := 0, 0
	previews.mu.Lock()
	savedLook, savedRoot, savedNow := previews.look, previews.root, previews.now
	previews.look = func(previewRoot, []int) ([]tcpListener, bool) {
		looks++
		return []tcpListener{{net.IPv4zero, 5173}}, true
	}
	previews.root = func(string, *launcherTab) previewRoot { roots++; return previewRoot{pid: 4242} }
	previews.now = func() time.Time { return clock }
	previews.mu.Unlock()
	t.Cleanup(func() {
		previews.mu.Lock()
		previews.look, previews.root, previews.now = savedLook, savedRoot, savedNow
		previews.mu.Unlock()
	})
	d.mu.Lock()
	d.sessions.Get("pv-rev").Tab = "tmux:%1@/tmp/s"
	d.sessions.Get("pv-jan").Tab = "iterm:ABCD"
	d.mu.Unlock()
	refresh := func() previewAnswer {
		w := serve(t, d, call{method: "GET", target: "/desk/preview?session=pv-rev&refresh=1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		var a previewAnswer
		_ = json.Unmarshal(w.Body.Bytes(), &a)
		return a
	}
	for range 5 {
		if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:5173" || a.Source != "found" {
			t.Fatalf("found: %+v", a)
		}
	}
	if looks != 1 || roots != 1 {
		t.Errorf("five polls looked %d times (%d roots)", looks, roots)
	}
	refresh()
	if looks != 1 {
		t.Errorf("a refresh right after looked again (%d)", looks)
	}
	clock = clock.Add(previewRefreshGap)
	refresh()
	if looks != 2 {
		t.Errorf("a refresh didn't look again (%d)", looks)
	}
	clock = clock.Add(previewFoundEvery)
	windowPreview(t, d, "pv-rev")
	if looks != 3 {
		t.Errorf("an old answer wasn't looked up again (%d)", looks)
	}
	windowPreview(t, d, "pv-jan") // iTerm2: no processes the app can find
	if looks != 3 || roots != 3 {
		t.Errorf("a desk on a tab with no processes known was looked at (%d, %d roots)", looks, roots)
	}
	d.mu.Lock()
	d.sessions.Get("pv-rev").Tab = "tmux:%2@/tmp/s"
	d.mu.Unlock()
	windowPreview(t, d, "pv-rev")
	if looks != 4 {
		t.Errorf("a desk on a new tab wasn't looked at again (%d)", looks)
	}
	// Another desk's lookup under way: an old answer is given rather than waited behind it.
	clock = clock.Add(previewFoundEvery)
	previews.lookup.Lock()
	_, a := windowPreview(t, d, "pv-rev")
	previews.lookup.Unlock()
	if looks != 4 || a.URL != "http://localhost:5173" {
		t.Errorf("a poll waited for another lookup: %d %+v", looks, a)
	}
}

// With the manager off, the GM and a hire are told so on POST /setup/preview, as on every route of theirs, and
// nothing changes; the owner's Setup still declares.
func TestThePreviewRouteMindsTheManagerSwitch(t *testing.T) {
	m := previewDeck(t)
	m.d.mu.Lock()
	m.d.config.ManagerEnabled = false
	m.d.mu.Unlock()
	for _, as := range []who{asGM, asRev} {
		w := m.call(t, as, "POST", "/setup/preview", `{"url":"http://localhost:4200","hire":"Rev"}`)
		if w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != managerOffAnswer {
			t.Errorf("caller %d, manager off: %d %q", as, w.Code, w.Body)
		}
	}
	if _, a := windowPreview(t, m.d, "pv-rev"); a.Declared != "" {
		t.Errorf("declared with the manager off: %+v", a)
	}
	if w := m.call(t, asPage, "POST", "/setup/preview", `{"url":"http://localhost:4201","session":"pv-rev"}`); w.Code != 200 {
		t.Errorf("the owner, manager off: %d %s", w.Code, w.Body)
	}
}

// Setup and its cookie live on 127.0.0.1 alone, as previews are on localhost and a browser sends a host's cookies to
// every port of it. Setup at localhost (or [::1]) is sent on to 127.0.0.1 before any cookie is set or read; a request
// there carrying Setup's cookie is refused; the cookie is set at 127.0.0.1 only. Tools, the GM and hires, which carry
// no cookie, go on as before.
func TestSetupAndItsCookieLiveOn127001Only(t *testing.T) {
	m := previewDeck(t)
	d := m.d
	session := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.access.NewSetupSession()
	}
	token := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.access.NewSetupToken()
	}
	cookie := map[string]string{"Cookie": setupCookie() + "=" + session(), setupHeader: "1"}
	for _, host := range []string{"localhost:7710", "[::1]:7710"} {
		// The page and what it reads: sent to 127.0.0.1, with or without a cookie, which isn't looked at.
		for _, target := range []string{"/setup", "/setup?demo=%2Fwork", "/setup/state"} {
			for _, headers := range []map[string]string{nil, cookie} {
				w := serve(t, d, call{method: "GET", target: target, host: host, from: thisComputer, headers: headers, withoutSetupKey: true, keepHost: true})
				if w.Code != http.StatusFound || w.Header().Get("Location") != setupBase()+target || w.Header().Get("Set-Cookie") != "" {
					t.Errorf("GET %s at %s: %d %q %q", target, host, w.Code, w.Header().Get("Location"), w.Header().Get("Set-Cookie"))
				}
			}
		}
		// A one-time link redeemed at localhost: refused, and no cookie is set.
		once := token()
		w := serve(t, d, call{method: "POST", target: "/setup/session", host: host, from: thisComputer, withoutSetupKey: true, keepHost: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: once}})
		if w.Code == http.StatusNoContent || w.Header().Get("Set-Cookie") != "" {
			t.Errorf("a session opened at %s: %d %q", host, w.Code, w.Header().Get("Set-Cookie"))
		}
		// The cookie, carried to localhost: refused, and nothing changes.
		w = serve(t, d, call{method: "POST", target: "/setup/preview", host: host, from: thisComputer, withoutSetupKey: true, keepHost: true,
			headers: cookie, body: `{"url":"http://localhost:4300","session":"pv-rev"}`})
		if w.Code != http.StatusForbidden || strings.TrimSpace(w.Body.String()) != setupOnLoopbackIP() {
			t.Errorf("the cookie at %s: %d %q", host, w.Code, w.Body)
		}
		if _, a := windowPreview(t, d, "pv-rev"); a.Declared != "" {
			t.Errorf("declared with the cookie at %s: %+v", host, a)
		}
	}
	// At 127.0.0.1 the link opens a session, and its cookie is 127.0.0.1's alone (no Domain: host-only).
	w := serve(t, d, call{method: "POST", target: "/setup/session", host: "127.0.0.1:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupTokenHeader: token()}})
	set := w.Result().Cookies()
	if w.Code != http.StatusNoContent || len(set) != 1 || set[0].Name != setupCookie() || set[0].Domain != "" || set[0].Path != "/setup" {
		t.Errorf("a session at 127.0.0.1: %d %v", w.Code, set)
	}
	if w := serve(t, d, call{method: "POST", target: "/setup/preview", host: "127.0.0.1:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{"Cookie": setupCookie() + "=" + set[0].Value, setupHeader: "1"}, body: `{"url":"http://localhost:4301","session":"pv-rev"}`}); w.Code != 200 {
		t.Errorf("the cookie at 127.0.0.1: %d %s", w.Code, w.Body)
	}
	// The GM and a hire, with their headers and no cookie, still reach Setup's routes at localhost.
	for _, as := range []who{asGM, asRev} {
		c := call{method: "POST", target: "/setup/preview", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, keepHost: true,
			headers: map[string]string{setupHeader: "1"}, body: `{"url":"http://localhost:4302","hire":"Rev"}`}
		if as == asGM {
			c.headers[gmTokenHeader] = m.gm
		} else {
			c.headers[hireKeyHeader] = m.keys["a1b2c3"]
		}
		if w := serve(t, d, c); w.Code != 200 {
			t.Errorf("caller %d at localhost: %d %s", as, w.Code, w.Body)
		}
	}
	// Every way the app opens Setup is at 127.0.0.1.
	for _, link := range []string{setupAddress("o=x", ""), demoPageLink("/work", "x"), (&officeTeam{base: localBase()}).demoLink(demoFolder{root: "/work"})} {
		if !strings.HasPrefix(link, "http://127.0.0.1:"+strconv.Itoa(Port)+"/setup") {
			t.Errorf("Setup opened at %s", link)
		}
	}
}

// The CLI's own client, `smolcorp gm preview`, against the Go app's router: a hire for its own desk, the GM for a hire.
func TestTheGMClientDeclaresAPreviewOnTheGoApp(t *testing.T) {
	m := previewDeck(t)
	srv := httptest.NewServer(&router{deck: m.d})
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	env := func(vars map[string]string) gm.Env {
		vars["SMOLCORP_PORT"] = port
		return gm.Env{Getenv: func(k string) string { return vars[k] }, SupportDir: func() (string, error) { return t.TempDir(), nil }}
	}
	hire, err := gm.NewSendClient(env(map[string]string{"SMOLCORP_HIRE_KEY": m.keys["a1b2c3"]}))
	if err != nil {
		t.Fatal(err)
	}
	if a, _, err := gm.Preview(hire, "http://localhost:5173", ""); err != nil || a.Session != "pv-rev" || a.URL != "http://localhost:5173" {
		t.Fatalf("hire: %+v %v", a, err)
	}
	if _, _, err := gm.Preview(hire, "http://localhost:5173", "Janitor"); err == nil || !strings.Contains(err.Error(), "only its own desk") {
		t.Errorf("hire for another: %v", err)
	}
	boss, err := gm.NewSendClient(env(map[string]string{"SMOLCORP_GM_TOKEN": m.gm}))
	if err != nil {
		t.Fatal(err)
	}
	if a, _, err := gm.Preview(boss, "http://127.0.0.1:8080/", "Janitor"); err != nil || a.Session != "pv-jan" {
		t.Fatalf("gm: %+v %v", a, err)
	}
	if _, _, err := gm.Preview(boss, "https://localhost:8080", "Janitor"); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("gm's https: %v", err)
	}
	for session, want := range map[string]string{"pv-rev": "http://localhost:5173", "pv-jan": "http://localhost:8080/"} {
		if _, a := windowPreview(t, m.d, session); a.URL != want {
			t.Errorf("%s: %+v", session, a)
		}
	}
}
