package gm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// session feeds the lines to the server and returns its answers, one per line.
func session(t *testing.T, env Env, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := serveMCP(env, strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	sc := bufio.NewScanner(&out)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("not JSON: %q", sc.Text())
		}
		got = append(got, m)
	}
	return got
}

func toolResult(t *testing.T, m map[string]any) (string, bool) {
	t.Helper()
	r, _ := m["result"].(map[string]any)
	c, _ := r["content"].([]any)
	if len(c) != 1 {
		t.Fatalf("no content: %v", m)
	}
	item := c[0].(map[string]any)
	if item["type"] != "text" {
		t.Fatalf("content type %v", item["type"])
	}
	isErr, _ := r["isError"].(bool)
	return item["text"].(string), isErr
}

func TestMCPSession(t *testing.T) {
	var paths []string
	var auth string
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		auth = r.Header.Get(tokenHeader)
		if r.URL.Path == "/setup/gm/version" {
			serve([]byte(`{"app":"Smolcorp","version":"1.24","api":2,"features":["mail"]}`), 200)(w, r)
			return
		}
		serve([]byte(`{"error":"no"}`), 403)(w, r)
	})
	env := app.env(t, withToken())
	got := session(t, env,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"version","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"watches","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"fire","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nope"}}`,
		`{"jsonrpc":"2.0","id":7,"method":"bogus"}`,
	)
	if len(got) != 7 { // the notification gets no answer
		t.Fatalf("%d answers: %v", len(got), got)
	}
	if info := got[0]["result"].(map[string]any)["serverInfo"].(map[string]any); info["name"] != "smolcorp" {
		t.Errorf("serverInfo %v", info)
	}
	want := []string{"team", "inbox", "version", "hire", "brief", "fire", "send", "outbox", "watch", "unwatch", "watches", "contacts", "pin", "unpin", "nickname", "approvals", "accounts"}
	tools := got[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(want) {
		t.Fatalf("%d tools", len(tools))
	}
	for i, n := range want {
		tl := tools[i].(map[string]any)
		if tl["name"] != n || tl["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("tool %d: %v", i, tl)
		}
	}
	text, isErr := toolResult(t, got[2])
	var v map[string]any
	if isErr || json.Unmarshal([]byte(text), &v) != nil || v["version"] != "1.24" {
		t.Errorf("version call: %v %q", isErr, text)
	}
	if auth != fakeToken {
		t.Errorf("the app was not signed in with the GM token")
	}
	if text, isErr := toolResult(t, got[3]); !isErr || !strings.Contains(text, "exit 1") {
		t.Errorf("a refusal should be an error result: %v %q", isErr, text)
	}
	if text, isErr := toolResult(t, got[4]); !isErr || !strings.Contains(text, "hire") {
		t.Errorf("fire without a hire: %v %q", isErr, text)
	}
	if got[5]["error"] == nil || got[6]["error"] == nil {
		t.Errorf("unknown tool and method should be JSON-RPC errors")
	}
	for _, g := range got {
		if b, _ := json.Marshal(g); strings.Contains(string(b), fakeToken) {
			t.Fatalf("the token is in an answer: %s", b)
		}
	}
	if len(paths) != 2 || paths[0] != "GET /setup/gm/version" || paths[1] != "GET /setup/gm/watches" {
		t.Errorf("requests %v", paths)
	}
}

func TestMCPReportsTheAppsVersion(t *testing.T) {
	was := AppVersion
	t.Cleanup(func() { AppVersion = was })
	AppVersion = "9.8.7-test"
	got := session(t, newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {}).env(t, withToken()),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`)
	info := got[0]["result"].(map[string]any)["serverInfo"].(map[string]any)
	if info["version"] != "9.8.7-test" {
		t.Errorf("serverInfo %v; want version 9.8.7-test", info)
	}
}

func TestMCPSendMatchesTheCLI(t *testing.T) {
	var got map[string]any
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		got = body(t, r)
		serve(fixtureField(t, "send.json", "answer"), 200)(w, r)
	})
	res := session(t, app.env(t, withToken()),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send","arguments":{"to":"Rev","text":"Look at login first.","id":"mcp-1"}}}`)
	if _, isErr := toolResult(t, res[0]); isErr {
		t.Fatal("send failed")
	}
	if got["to"] != "Rev" || got["text"] != "Look at login first." || got["id"] != "mcp-1" {
		t.Errorf("request %v", got)
	}
}

// No tool answers a prompt, and none rotates the token.
func TestMCPHasNoAnswerTool(t *testing.T) {
	banned := []string{"answer", "approve", "deny", "respond", "reply", "confirm", "choose", "rotate", "token", "key", "enter", "type", "paste", "tile"}
	for _, tl := range mcpTools() {
		for _, b := range banned {
			if strings.Contains(strings.ToLower(tl.Name), b) {
				t.Errorf("tool %q looks like an answer or token tool (%q)", tl.Name, b)
			}
		}
	}
	// Every route the tools can reach is in the routes table for the GM, and none is an answer route.
	for _, tl := range mcpTools() {
		if tl.Name == "approvals" { // read-only: a GET, nothing else
			var methods []string
			app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method+" "+r.URL.Path)
				serve([]byte(`{"approvals":[]}`), 200)(w, r)
			})
			session(t, app.env(t, withToken()), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"approvals"}}`)
			if len(methods) != 1 || methods[0] != "GET /setup/gm/approvals" {
				t.Errorf("approvals made %v", methods)
			}
		}
	}
	src := mcpTools()
	for _, tl := range src {
		for _, k := range []string{"answer", "key", "keys", "choice"} {
			if _, ok := tl.Props[k]; ok {
				t.Errorf("tool %q takes %q", tl.Name, k)
			}
		}
	}
}

// T735: the tools the Go app doesn't serve yet say so in their description.
func TestMCPToolsNameWhatLinuxAndWindowsLack(t *testing.T) {
	for _, tool := range mcpTools() {
		want := tool.Name == "hire" || tool.Name == "brief" || tool.Name == "fire" || tool.Name == "accounts"
		if got := strings.Contains(tool.Description, "Not served on Linux and Windows yet."); got != want {
			t.Errorf("%s: note = %v, want %v", tool.Name, got, want)
		}
	}
}

// The three tools of T745, their schemas pinned and what each one asks of the app.
func TestMCPPlaceToolsSchemasAndRequests(t *testing.T) {
	var bodies []string
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(data))
		if r.URL.Path == "/setup/gm/nickname" {
			serve([]byte(`{"project":"/work/app","nickname":"Ada"}`), 200)(w, r)
			return
		}
		serve(nil, 204)(w, r)
	})
	env := app.env(t, withToken())
	got := session(t, env,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"pin","arguments":{"desk":3,"folder":"/work/app"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"unpin","arguments":{"desk":3}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nickname","arguments":{"name":"Ada","folder":"/work/app"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nickname","arguments":{"name":""}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"unpin","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"nickname","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"pin","arguments":{"desk":"3"}}}`,
	)
	schemas := map[string]string{}
	for _, tl := range got[0]["result"].(map[string]any)["tools"].([]any) {
		m := tl.(map[string]any)
		b, _ := json.Marshal(m["inputSchema"])
		schemas[m["name"].(string)] = string(b)
	}
	for name, want := range map[string]string{
		"pin":      `{"properties":{"desk":{"description":"the desk, counted from 1 as the office shows","type":"integer"},"folder":{"description":"the project's folder","type":"string"}},"type":"object"}`,
		"unpin":    `{"properties":{"desk":{"description":"the desk, counted from 1 as the office shows","type":"integer"}},"required":["desk"],"type":"object"}`,
		"nickname": `{"properties":{"folder":{"description":"the project's folder; the server's own if left out","type":"string"},"name":{"description":"the nickname, or empty for none","type":"string"}},"required":["name"],"type":"object"}`,
	} {
		if schemas[name] != want {
			t.Errorf("%s schema:\n got %s\nwant %s", name, schemas[name], want)
		}
	}
	for i, id := range []int{1, 2, 3} {
		if text, isErr := toolResult(t, got[i+1]); isErr {
			t.Errorf("call %d: %q", id, text)
		}
	}
	if len(app.reqs) != 4 { // pin, unpin, two nicknames; the three bad calls never reach the app
		t.Fatalf("requests: %+v", app.reqs)
	}
	if q, _ := url.ParseQuery(app.reqs[0].RawQuery); app.reqs[0].Path != "/setup/pin" || q.Get("key") != "2" || q.Get("folder") != "/work/app" {
		t.Errorf("pin = %+v", app.reqs[0])
	}
	if q, _ := url.ParseQuery(app.reqs[1].RawQuery); app.reqs[1].Path != "/setup/unpin" || q.Get("key") != "2" {
		t.Errorf("unpin = %+v", app.reqs[1])
	}
	if app.reqs[2].Path != "/setup/gm/nickname" || bodies[2] != `{"folder":"/work/app","name":"Ada"}` {
		t.Errorf("nickname = %+v %s", app.reqs[2], bodies[2])
	}
	if app.reqs[3].Path != "/setup/gm/nickname" || !strings.Contains(bodies[3], `"name":""`) {
		t.Errorf("an empty name is sent as empty: %+v %s", app.reqs[3], bodies[3])
	}
	for _, i := range []int{5, 6, 7} {
		if text, isErr := toolResult(t, got[i]); !isErr {
			t.Errorf("call %d should be an error: %q", i, text)
		}
	}
}
