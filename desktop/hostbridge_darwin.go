package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"time"
)

// socketBridge reaches the Mac app on its socket in the support folder, with
// the token the app gave this engine as it started it: one connection per
// request, a line of JSON each way.
type socketBridge struct {
	path, token string
	// How long one request may take: a terminal's AppleScript can take a few seconds.
	wait time.Duration
}

// bridgeAnswerMost is the longest answer read: a secret or a tab name is short.
const bridgeAnswerMost = 64 << 10

func (b *socketBridge) call(request bridgeRequest) (bridgeAnswer, error) {
	request.Token = b.token
	conn, err := net.DialTimeout("unix", b.path, 2*time.Second)
	if err != nil {
		return bridgeAnswer{}, err
	}
	defer conn.Close()
	wait := b.wait
	if wait <= 0 {
		wait = 20 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(wait))
	line, err := json.Marshal(request)
	if err != nil {
		return bridgeAnswer{}, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return bridgeAnswer{}, err
	}
	reader := bufio.NewReaderSize(conn, 4096)
	var text []byte
	for {
		part, more, err := reader.ReadLine()
		if err != nil {
			return bridgeAnswer{}, err
		}
		text = append(text, part...)
		if len(text) > bridgeAnswerMost {
			return bridgeAnswer{}, errors.New("the Mac app's answer is too long")
		}
		if !more {
			break
		}
	}
	var answer bridgeAnswer
	if err := json.Unmarshal(text, &answer); err != nil {
		return bridgeAnswer{}, err
	}
	return answer, nil
}
