package desktop

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// T3 Code's internal API (EXPERIMENTAL), for Take over and Hand back alone (t3takeover.go): the two calls its
// own pages make, agentSessions.import (the Welcome page's Import) and orchestration.forceStopThread (a
// thread's Force stop), which no t3 command offers (S794). They're T3 Code's, not a published API, so they're
// only made on the t3 versions they were read and tried on (t3TakeOverVersions).
//
// How it signs in is T3 Code's own way for a headless client: `t3 auth session issue`, the same short-lived
// session every `t3 thread` command makes for itself, here with the one scope the two calls need, a two-minute
// life and a label that says it's Smolcorp's; it's revoked with `t3 auth session revoke` as soon as the call is
// answered. The token stays in memory for that one call: never written, never logged, never in a message. No
// T3 Code setting is read or changed, and none of its files but server-runtime.json, for where its server
// listens (only on this computer: anything else is refused).

const (
	// t3CallScope is the one scope both calls need (T3 Code's RpcAuthorization).
	t3CallScope = "orchestration:operate"
	// t3CallWithin is how long one call may take, its sign-in and the import of a long session included.
	t3CallWithin = 2 * time.Minute
	// t3CallLongest is the most an answer may be.
	t3CallLongest = 4 << 20
	t3CallLabel   = "Smolcorp: Take over (EXPERIMENTAL)"
	t3CallGUID    = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// t3Home is T3 Code's own folder: %T3CODE_HOME% where it's set, else ~/.t3.
func t3Home(m t3Machine) string {
	if m.Getenv != nil && m.Getenv("T3CODE_HOME") != "" {
		return m.Getenv("T3CODE_HOME")
	}
	return filepath.Join(m.Home, ".t3")
}

// t3Origin is where T3 Code's server listens, as its server-runtime.json says: http on this computer only
// (127.0.0.1, ::1 or localhost), else "".
func t3Origin(m t3Machine) string {
	data, err := os.ReadFile(filepath.Join(t3Home(m), "userdata", "server-runtime.json"))
	if err != nil {
		return ""
	}
	var state struct {
		Origin string `json:"origin"`
	}
	if json.Unmarshal(data, &state) != nil {
		return ""
	}
	given, err := url.Parse(state.Origin)
	if err != nil || given.Scheme != "http" || given.Port() == "" || given.User != nil ||
		!slices.Contains([]string{"127.0.0.1", "::1", "localhost"}, given.Hostname()) {
		return ""
	}
	return "http://" + given.Host
}

// t3CallError is a call T3 Code refused or couldn't make: why, in a few words; never the token.
type t3CallError struct{ Why string }

func (e *t3CallError) Error() string { return e.Why }

func t3CallFailed(why string) error { return &t3CallError{Why: why} }

// t3Call makes one call of T3 Code's internal API: method with payload, and its value on success. It signs in
// with program (the t3 command) and signs out again whatever happens.
func t3Call(m t3Machine, program, method string, payload map[string]any) (map[string]any, error) {
	origin := t3Origin(m)
	if origin == "" {
		return nil, t3CallFailed("T3 Code's server isn't running on this computer")
	}
	issued := m.Run(program, []string{"auth", "session", "issue", "--scope", t3CallScope, "--ttl", "2m", "--label", t3CallLabel, "--json"}, t3Within)
	// Its output holds the token: only its error side is ever said.
	if !issued.Ended || issued.Code != 0 {
		// No answer in time, or an end with a failure after it issued: T3 Code may have issued it all the same, so
		// whatever it listed as ours is signed out (RM5b N1, FXTO2 N3).
		t3RevokeOurs(m, program)
		why := firstLine(issued.Error)
		if why == "" {
			why = "it gave no reason"
		}
		return nil, t3CallFailed("T3 Code didn't sign Smolcorp in: " + clip(why, 200))
	}
	var session struct {
		SessionID string `json:"sessionId"`
		Token     string `json:"token"`
	}
	if json.Unmarshal([]byte(issued.Output), &session) != nil || session.Token == "" || session.SessionID == "" {
		// Issued, but not in a way Smolcorp reads: signed out now, not left to its two minutes (RM5b N1).
		t3RevokeOurs(m, program)
		return nil, t3CallFailed("T3 Code didn't sign Smolcorp in the way Smolcorp knows")
	}
	defer t3Revoke(m, program, session.SessionID)
	return t3CallWith(origin, session.Token, method, payload, t3CallWithin)
}

// t3Revoke signs Smolcorp's session id out of T3 Code.
func t3Revoke(m t3Machine, program, id string) {
	m.Run(program, []string{"auth", "session", "revoke", id}, t3Within)
}

// t3RevokeOurs signs out every session T3 Code lists under Smolcorp's label, for a sign-in whose answer was lost
// or couldn't be read, so its id isn't known. The list holds no token. Calls are made one at a time (under t3Desk),
// so none of these is another call's own.
func t3RevokeOurs(m t3Machine, program string) {
	listed := m.Run(program, []string{"auth", "session", "list", "--json"}, t3Within)
	var sessions []struct {
		SessionID string `json:"sessionId"`
		Client    struct {
			Label string `json:"label"`
		} `json:"client"`
	}
	if !listed.Ended || listed.Code != 0 || json.Unmarshal([]byte(listed.Output), &sessions) != nil {
		return
	}
	for _, session := range sessions {
		if session.Client.Label == t3CallLabel && session.SessionID != "" && !strings.HasPrefix(session.SessionID, "-") {
			t3Revoke(m, program, session.SessionID)
		}
	}
}

// t3CallWith is the call itself, signed in: a ticket for the socket, then the request on T3 Code's socket,
// answered by its Exit.
func t3CallWith(origin, token, method string, payload map[string]any, within time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(within)
	client := &http.Client{Timeout: within, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ask, _ := http.NewRequest(http.MethodPost, origin+"/api/auth/websocket-ticket", strings.NewReader("{}"))
	ask.Header.Set("Authorization", "Bearer "+token)
	ask.Header.Set("Content-Type", "application/json")
	res, err := client.Do(ask)
	if err != nil {
		return nil, t3CallFailed("T3 Code's server didn't answer")
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	res.Body.Close()
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	if res.StatusCode != http.StatusOK || json.Unmarshal(body, &ticket) != nil || ticket.Ticket == "" {
		return nil, t3CallFailed("T3 Code's server didn't let Smolcorp in (" + http.StatusText(res.StatusCode) + ")")
	}
	host := strings.TrimPrefix(origin, "http://")
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, t3CallFailed("T3 Code's server didn't answer")
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	reader, err := t3SocketOpen(conn, host, "/ws?orchestrationProtocol=2&wsTicket="+url.QueryEscape(ticket.Ticket))
	if err != nil {
		return nil, err
	}
	request, _ := json.Marshal(map[string]any{"_tag": "Request", "id": "1", "tag": method, "payload": payload, "headers": []any{}})
	if t3SocketWrite(conn, 1, request, true) != nil {
		return nil, t3CallFailed("T3 Code's server closed the connection")
	}
	for {
		// A ping is answered where it comes, even between two fragments of the answer, which are kept (RM5b S1).
		op, data, err := t3SocketReadAnswering(reader, t3CallLongest, func(ping []byte) { _ = t3SocketWrite(conn, 10, ping, true) })
		if err != nil {
			return nil, t3CallFailed("T3 Code's server didn't answer the call")
		}
		switch op {
		case 8:
			return nil, t3CallFailed("T3 Code's server closed the connection before answering")
		case 1, 2:
		default:
			continue
		}
		var messages []map[string]any
		if json.Unmarshal(data, &messages) != nil {
			var one map[string]any
			if json.Unmarshal(data, &one) != nil {
				continue
			}
			messages = []map[string]any{one}
		}
		for _, message := range messages {
			switch message["_tag"] {
			case "Ping":
				pong, _ := json.Marshal(map[string]any{"_tag": "Pong"})
				_ = t3SocketWrite(conn, 1, pong, true)
			case "Exit":
				if id, _ := message["requestId"].(string); id != "1" {
					continue
				}
				exit, _ := message["exit"].(map[string]any)
				if exit["_tag"] == "Success" {
					value, _ := exit["value"].(map[string]any)
					return value, nil
				}
				return nil, t3CallFailed("T3 Code refused it (" + cmpOr(t3ErrorTag(exit), "no reason given") + ")")
			}
		}
	}
}

// t3ErrorTag is the name of the first error in what T3 Code answered (AgentSessionImportProjectNotFoundError…).
func t3ErrorTag(value any) string {
	switch v := value.(type) {
	case map[string]any:
		if tag, _ := v["_tag"].(string); strings.HasSuffix(tag, "Error") {
			return clip(tag, 80)
		}
		for _, key := range []string{"cause", "error", "failure", "left", "right"} {
			if tag := t3ErrorTag(v[key]); tag != "" {
				return tag
			}
		}
	case []any:
		for _, each := range v {
			if tag := t3ErrorTag(each); tag != "" {
				return tag
			}
		}
	}
	return ""
}

// A WebSocket as far as one call needs it (RFC 6455): the handshake, and whole frames, a message being one or
// more of them. Ours are masked, as a client's must be; the server's aren't.

// t3SocketOpen asks to upgrade conn to a WebSocket at path, and checks T3 Code's answer.
func t3SocketOpen(conn net.Conn, host, path string) (*bufio.Reader, error) {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	key := base64.StdEncoding.EncodeToString(nonce)
	ask := "GET " + path + " HTTP/1.1\r\nHost: " + host + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, ask); err != nil {
		return nil, t3CallFailed("T3 Code's server didn't answer")
	}
	reader := bufio.NewReader(conn)
	res, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		return nil, t3CallFailed("T3 Code's server didn't answer")
	}
	res.Body.Close()
	sum := sha1.Sum([]byte(key + t3CallGUID))
	if res.StatusCode != http.StatusSwitchingProtocols || res.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		return nil, t3CallFailed("T3 Code's server didn't open its socket (" + http.StatusText(res.StatusCode) + ")")
	}
	return reader, nil
}

// t3SocketWrite writes one whole frame: op (1 text, 2 binary, 8 close, 9 ping, 10 pong) with data, masked when mask.
func t3SocketWrite(w io.Writer, op byte, data []byte, mask bool) error {
	head := []byte{0x80 | op}
	bit := byte(0)
	if mask {
		bit = 0x80
	}
	switch n := len(data); {
	case n < 126:
		head = append(head, bit|byte(n))
	case n <= 0xFFFF:
		head = append(head, bit|126, byte(n>>8), byte(n))
	default:
		head = append(head, bit|127)
		head = binary.BigEndian.AppendUint64(head, uint64(n))
	}
	body := data
	if mask {
		key := make([]byte, 4)
		_, _ = rand.Read(key)
		head = append(head, key...)
		body = make([]byte, len(data))
		for i, b := range data {
			body[i] = b ^ key[i%4]
		}
	}
	_, err := w.Write(append(head, body...))
	return err
}

// t3SocketRead reads one message: its op and its data, the fragments put together, unmasked when masked; an
// error past longest bytes. A control frame (ping, pong, close) comes back on its own.
func t3SocketRead(r *bufio.Reader, longest int) (byte, []byte, error) {
	return t3SocketReadAnswering(r, longest, nil)
}

// t3SocketReadAnswering is t3SocketRead with ping, when given, answering each ping where it comes and a pong
// passed over, so a message's fragments already read are kept around them (RFC 6455 §5.4 allows control frames
// between fragments: RM5b S1). A close still comes back on its own.
func t3SocketReadAnswering(r *bufio.Reader, longest int, ping func([]byte)) (byte, []byte, error) {
	var op byte
	var message []byte
	for {
		head := make([]byte, 2)
		if _, err := io.ReadFull(r, head); err != nil {
			return 0, nil, err
		}
		final, frameOp, masked := head[0]&0x80 != 0, head[0]&0x0F, head[1]&0x80 != 0
		size := uint64(head[1] & 0x7F)
		switch size {
		case 126:
			more := make([]byte, 2)
			if _, err := io.ReadFull(r, more); err != nil {
				return 0, nil, err
			}
			size = uint64(binary.BigEndian.Uint16(more))
		case 127:
			more := make([]byte, 8)
			if _, err := io.ReadFull(r, more); err != nil {
				return 0, nil, err
			}
			size = binary.BigEndian.Uint64(more)
		}
		if size > uint64(longest) || len(message)+int(size) > longest {
			return 0, nil, errors.New("too long")
		}
		// A control frame is never fragmented, and never over 125 bytes (RFC 6455 §5.5): one that is, is refused
		// before it's read (RM5b N3).
		if frameOp >= 8 && (!final || size > 125) {
			return 0, nil, errors.New("a control frame that isn't one")
		}
		var key []byte
		if masked {
			key = make([]byte, 4)
			if _, err := io.ReadFull(r, key); err != nil {
				return 0, nil, err
			}
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(r, data); err != nil {
			return 0, nil, err
		}
		for i := range data {
			if masked {
				data[i] ^= key[i%4]
			}
		}
		if frameOp >= 8 {
			switch {
			case ping != nil && frameOp == 9:
				ping(data)
				continue
			case ping != nil && frameOp == 10:
				continue
			}
			return frameOp, data, nil
		}
		if frameOp != 0 {
			op = frameOp
		}
		message = append(message, data...)
		if final {
			return op, message, nil
		}
	}
}
