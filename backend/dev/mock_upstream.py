#!/usr/bin/env python3
"""Mock Article + Chat services for trying out the assistant locally.

One server on :9100 serves both upstream contracts (the paths don't overlap):
  GET  /articles/{id}          -> article with full text
  GET  /chats/{id}/messages    -> stored history (in-memory)
  POST /chats/{id}/messages    -> append a message, returns created record

Point both ARTICLE_SERVICE_URL and CHAT_SERVICE_URL at http://localhost:9100
(that is the default in .env.example). Chat history is kept in memory and reset
when the process restarts.

Run:  python3 dev/mock_upstream.py
"""
import json
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

STORE = {}      # {chatId: [messages]}
_seq = [0]

ARTICLE_TEXT = (
    "Test-Time Gradient Guidance of Flow Policies (QGF).\n\n"
    "QGF pre-trains a reference flow policy with behavior cloning and a value "
    "function critic with TD learning, then at test time uses the value gradient "
    "to guide the reference policy toward higher-value actions. This decouples "
    "policy and critic training and moves optimization to inference time, "
    "avoiding the actor-training instability that makes flow/diffusion policies "
    "hard to fold into RL."
)


class Handler(BaseHTTPRequestHandler):
    def _json(self, code, payload):
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if m := re.fullmatch(r"/articles/([^/]+)", self.path):
            self._json(200, {
                "id": m.group(1),
                "title": "Test-Time Gradient Guidance of Flow Policies",
                "authors": "Zhou et al.",
                "content": ARTICLE_TEXT,
            })
        elif m := re.fullmatch(r"/chats/([^/]+)/messages", self.path):
            self._json(200, STORE.get(m.group(1), []))
        else:
            self._json(404, {"error": "not found"})

    def do_POST(self):
        if m := re.fullmatch(r"/chats/([^/]+)/messages", self.path):
            chat_id = m.group(1)
            n = int(self.headers.get("Content-Length", 0))
            data = json.loads(self.rfile.read(n) or b"{}")
            _seq[0] += 1
            msg = {
                "id": f"m{_seq[0]}",
                "role": data.get("role", "user"),
                "content": data.get("content", ""),
                "createdAt": "2026-01-01T00:00:00Z",
            }
            STORE.setdefault(chat_id, []).append(msg)
            print(f"[chat] saved {msg['role']}: {msg['content'][:60]!r}")
            self._json(201, msg)
        else:
            self._json(404, {"error": "not found"})

    def log_message(self, *args):
        pass  # silence default request logging


if __name__ == "__main__":
    print("mock upstream (article + chat) on http://localhost:9100")
    ThreadingHTTPServer(("", 9100), Handler).serve_forever()
