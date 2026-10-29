#!/usr/bin/env python3
"""A pretend Smolcorp deck for building the iPhone app, never the real one.

    ios/scripts/mock-deck.py [--port 7791] [--tls-port 7792] [--dir DIR]

- Plain HTTP on --port, as the Mac's deck on the Wi-Fi; HTTPS on --tls-port with
  a self-signed certificate made in DIR, as the phone will pin it. The
  certificate's SHA-256 is printed, and kept in DIR/fingerprint.txt.
- Pairing code 123456. The key it gives is kept in DIR/key.txt, and every route
  but /pair and /ping wants it in X-AgentDeck-Token.
- /events streams a snapshot as the Mac's does, a desk changing every few
  seconds, and a ping every 20 s.
- /dictate answers only over HTTPS, as the Mac answers only over Tailscale's.
- Advertised over Bonjour as _smolcorp._tcp with `dns-sd`, while it runs.

Everything it hears is printed. Stop it with Ctrl-C.
"""
import argparse
import hashlib
import json
import os
import secrets
import ssl
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

CODE = "123456"
GLOSSARY = ["Smolcorp", "Claude", "Claude Code", "Codex", "Gemini", "OpenCode", "Grok", "GitHub", "Tailscale", "Cloudflare", "Vercel", "Supabase"]

lock = threading.Lock()
changed = threading.Condition(lock)
state = {"version": 0, "language": "en_US", "tick": 0}
heard = []  # what came in, for the spike's own checks


def session(i, name, state_, activity, agent="claude", agent_name="Claude Code"):
    return {
        "id": f"s{i}", "name": name, "project": name, "state": state_, "effort": "high", "pending": False,
        "agent": agent, "agentName": agent_name, "dial": True, "mic": True, "dictates": False, "phoneMic": True,
        "keys": True, "fire": True, "activity": activity, "tool": None, "model": "claude-opus-5-5", "subagents": [],
        "maxPressedAt": None, "messagedAt": None, "messaged": None, "compacting": False,
        "turnStartedAt": int(time.time() * 1000) - 30_000 if state_ == "thinking" else None,
        "errorKind": None, "summary": True, "conversation": True, "voiceNote": None, "role": None,
        "choice": CHOICES.get(i) if state_ == "asking" else None,
    }


# What the asking desks offer, as the Mac's snapshot carries it: moonbase asks a question,
# smolcorp a permission, whose command only GET /choice gives, over HTTPS.
CHOICES = {
    3: {"id": "q-moonbase", "kind": "question", "question": "Which database should the app use?", "multiSelect": False,
        "questions": 1, "answerable": True, "other": True, "options": [
            {"label": "Supabase", "description": "The usual: auth and storage too"},
            {"label": "SQLite", "description": "Local only, no account"},
            {"label": "Postgres on Neon", "description": "Serverless, branches per preview"}]},
    1: {"id": "p-smolcorp", "kind": "permission", "question": "Allow Bash?", "multiSelect": False, "questions": 1,
        "answerable": True, "options": [
            {"label": "Yes", "description": ""},
            {"label": "Yes, and don't ask again", "description": "As Claude Code offers it on the Mac"},
            {"label": "No", "description": "Claude Code asks what to do instead"}]},
}
DETAILS = {"s1": ("p-smolcorp", "swift test --filter ChoiceTests && git status")}

# A made-up conversation, as GET /conversation pages it: older work first, then the latest request.
def _conversation():
    items = []
    for n in range(1, 16):
        items += [("user", f"Older request {n}: tidy up the settings copy"), ("agent", f"Done with part {n}."),
                  ("tool", "Edit: ios/Smolcorp/App/Settings.swift"), ("result", "The file has been updated.")]
    items += [
        ("user", "The summary card cuts off on the iPhone. Can you make it scroll, and keep the newest line in sight?"),
        ("agent", "Looking at how the card is laid out on the phone first."),
        ("tool", "Read: Resources/web/deck.html"), ("result", "<!doctype html>"),
        ("tool", "Grep: summary-text"), ("result", "Found 4 matches"),
        ("question", "Should the card grow with its text, or stay one size and scroll?\n1. Grow, up to half the screen\n2. Stay one size and scroll"),
        ("answer", "\"Should the card grow…\"=\"Grow, up to half the screen\""),
        ("agent", "Making it grow up to half the screen, then scroll, with the newest line kept in sight."),
        ("tool", "Edit: Resources/web/deck.html"), ("result", "The file has been updated."),
        ("tool", "Bash: swift test --filter SummaryTests"), ("result", "Error: error: 1 test failed"),
        ("agent", "One test still expected the old height. Fixing it.\n[redacted]"),
        ("tool", "Bash: swift test --filter SummaryTests"), ("result", "Test Suite 'SummaryTests' passed"),
        ("agent", "Done: the card grows up to half the screen, then scrolls, and the newest line stays in sight. " * 3),
    ]
    out = []
    for i, (kind, text) in enumerate(items):
        out.append({"id": f"{i * 100}.0", "offset": i * 100, "n": 0, "kind": kind, "text": text, "more": i == len(items) - 1})
    return out
CONVERSATION = _conversation()
NOTIFY = {"registered": False, "finished": True, "waiting": True, "gm": True}


def snapshot():
    t = state["tick"]
    cycle = ["thinking", "asking", "unread", "idle"]
    keys = [
        {"pin": None, "session": session(1, "smolcorp", cycle[t % 4], "Editing Deck.swift")},
        {"pin": None, "session": session(2, "acmebill", cycle[(t + 1) % 4], "Running tests")},
        {"pin": None, "session": session(3, "moonbase", "asking", "Reading the plan")},
        {"pin": None},
        {"pin": "orbit"},
        {"pin": None},
    ]
    return {
        "keys": keys, "overflow": 0,
        "upstairs": {"desks": [], "overflow": 0},
        "now": int(time.time() * 1000), "theme": {}, "scroll": True,
        "dictation": {"on": False, "language": state["language"], "cleanup": False},
        "phoneMic": {"on": True, "autoSend": True},
        "hiring": {"fire": True, "hire": True, "roles": []},
        "music": None, "layout": None,
    }


def ticker():
    while True:
        time.sleep(6)
        with changed:
            state["tick"] += 1
            state["version"] += 1
            changed.notify_all()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    key = ""
    secure = False

    def log_message(self, fmt, *args):
        print(f"{'https' if self.secure else 'http '} {self.address_string()} {fmt % args}", flush=True)

    def reply(self, status, body=b"", kind="text/plain; charset=utf-8"):
        if isinstance(body, (dict, list)):
            body, kind = json.dumps(body).encode(), "application/json"
        elif isinstance(body, str):
            body = body.encode()
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(body)

    def body(self):
        n = int(self.headers.get("Content-Length") or 0)
        return self.rfile.read(n) if n else b""

    def paired(self):
        given = self.headers.get("X-AgentDeck-Token") or ""
        return bool(self.key) and secrets.compare_digest(given, self.key)

    def do_GET(self):
        url = urlparse(self.path)
        if url.path == "/ping":
            return self.reply(200, {"ok": True, "secure": self.secure})
        if url.path == "/":
            return self.reply(200, "<!doctype html><title>Smolcorp</title>pairing page", "text/html; charset=utf-8")
        if not self.paired():
            return self.reply(401)
        if url.path == "/glossary":
            return self.reply(200, {"glossary": GLOSSARY, "language": state["language"]})
        if url.path == "/events":
            return self.events()
        if url.path == "/conversation":
            if not self.secure:
                return self.reply(403, "The conversation shows only over Tailscale's HTTPS address.")
            q = parse_qs(url.query)
            before = int(q.get("before", [str(len(CONVERSATION) * 100)])[0])
            older = [item for item in CONVERSATION if item["offset"] < before][-60:]
            first = older[0]["offset"] if older else 0
            return self.reply(200, {"items": older, "before": first or None, "size": len(CONVERSATION) * 100})
        if url.path == "/notifications":
            return self.reply(200, {"key": os.environ.get("MOCK_PUSH_KEY", "1") == "1", **NOTIFY})
        if url.path == "/choice":
            if not self.secure:
                return self.reply(403, "The details show only over Tailscale's HTTPS address.")
            found = DETAILS.get(parse_qs(url.query).get("session", [""])[0])
            return self.reply(200, {"id": found[0], "detail": found[1]}) if found else self.reply(404, "It isn't asking you anything.")
        self.reply(404)

    def events(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        self.close_connection = True
        try:
            with changed:
                version = state["version"]
                data = json.dumps(snapshot())
            self.wfile.write(f"data: {data}\n\n".encode())
            self.wfile.flush()
            last_ping = time.time()
            while True:
                with changed:
                    changed.wait(timeout=1)
                    fresh = state["version"] != version
                    version = state["version"]
                    data = json.dumps(snapshot()) if fresh else None
                if data:
                    self.wfile.write(f"data: {data}\n\n".encode())
                if time.time() - last_ping > 20:
                    self.wfile.write(b"event: ping\ndata: 1\n\n")
                    last_ping = time.time()
                self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError, ssl.SSLError, OSError):
            pass

    def do_POST(self):
        url = urlparse(self.path)
        query = {k: v[0] for k, v in parse_qs(url.query).items()}
        body = self.body()
        if url.path == "/pair":
            code = "".join(c for c in (json.loads(body or b"{}").get("code") or "") if c.isdigit())
            if code != CODE:
                return self.reply(403, {"error": "That code isn't valid. Get a new one on your Mac."})
            return self.reply(200, {"key": Handler.key})
        if not self.paired():
            return self.reply(401)
        heard.append({"path": url.path, "query": query, "body": body.decode(errors="replace"), "secure": self.secure})
        print(f"   {url.path} {query} {body.decode(errors='replace')}", flush=True)
        if url.path == "/act":
            if query.get("action") == "language" and query.get("lang") in ("fr_FR", "en_US"):
                with changed:
                    state["language"] = query["lang"]
                    state["version"] += 1
                    changed.notify_all()
            return self.reply(204)
        if url.path == "/focus":
            return self.reply(204)
        if url.path == "/dictate":
            if not self.secure:
                return self.reply(403, "The phone mic works only over Tailscale's HTTPS address.")
            return self.reply(204)
        if url.path == "/choose":
            if not self.secure:
                return self.reply(403, "Typed answers go only over Tailscale's HTTPS address.")
            return self.reply(204)
        if url.path == "/notifications":
            if not self.secure:
                return self.reply(403, "Notifications are set up only over Tailscale's HTTPS address.")
            given = json.loads(body or b"{}")
            NOTIFY.update({k: given.get(k, True) for k in ("finished", "waiting", "gm")}, registered=bool(given.get("token")))
            return self.reply(200, {"key": os.environ.get("MOCK_PUSH_KEY", "1") == "1", **NOTIFY})
        if url.path == "/summary":
            return self.reply(200, {"summary": "Asked: a mock summary.\nDid: nothing real.\nNot waiting on you."})
        self.reply(404)


class SecureHandler(Handler):
    secure = True


def certificate(folder):
    cert, key = os.path.join(folder, "cert.pem"), os.path.join(folder, "key.pem")
    if not os.path.exists(cert):
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1",
                        "-nodes", "-keyout", key, "-out", cert, "-days", "825", "-subj", "/CN=Smolcorp mock deck"],
                       check=True, capture_output=True)
    der = ssl.PEM_cert_to_DER_cert(open(cert).read())
    return cert, key, hashlib.sha256(der).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=7791)
    parser.add_argument("--tls-port", type=int, default=7792)
    parser.add_argument("--dir", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "build", "mock"))
    args = parser.parse_args()
    if 7710 in (args.port, args.tls_port):
        raise SystemExit("7710 is the real Smolcorp's port.")
    os.makedirs(args.dir, exist_ok=True)

    key_file = os.path.join(args.dir, "key.txt")
    if not os.path.exists(key_file):
        open(key_file, "w").write(secrets.token_hex(32))
    Handler.key = open(key_file).read().strip()

    cert, key, fingerprint = certificate(args.dir)
    open(os.path.join(args.dir, "fingerprint.txt"), "w").write(fingerprint)

    plain = ThreadingHTTPServer(("0.0.0.0", args.port), Handler)
    secure = ThreadingHTTPServer(("0.0.0.0", args.tls_port), SecureHandler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.load_cert_chain(cert, key)
    secure.socket = context.wrap_socket(secure.socket, server_side=True)

    bonjour = subprocess.Popen(["dns-sd", "-R", "Smolcorp mock", "_smolcorp._tcp", "local", str(args.port),
                                "api=1", "id=mock", f"tls={args.tls_port}"],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    for target in (plain.serve_forever, secure.serve_forever, ticker):
        threading.Thread(target=target, daemon=True).start()
    print(f"mock deck: http :{args.port}, https :{args.tls_port}, code {CODE}", flush=True)
    print(f"fingerprint {fingerprint}", flush=True)
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        pass
    finally:
        bonjour.terminate()


if __name__ == "__main__":
    main()
