"""Mock OpenAI-compatible server standing in for the big GPU pool.
Stdlib only, so it runs from a ConfigMap in a stock python image."""
import json, os, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

NAME = os.environ.get("POOL_NAME", "llm-gpu-pool")
DELAY = float(os.environ.get("DELAY_SECONDS", "1.5"))   # simulated GPU queue + big-model latency
PRICE = float(os.environ.get("USD_PER_1K_TOKENS", "0.015"))

class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass

    def _send(self, code, obj, extra=None):
        b = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(b)))
        self.send_header("x-served-by", NAME)
        for k, v in (extra or {}).items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        self._send(200, {"ok": True, "pool": NAME})

    def do_POST(self):
        n = int(self.headers.get("content-length", 0))
        body = json.loads(self.rfile.read(n) or b"{}")
        time.sleep(DELAY)
        user = next((m["content"] for m in reversed(body.get("messages", [])) if m.get("role") == "user"), "")
        text = f"[{NAME}] Frontier-model answer to: {user[:80]}"
        toks = max(1, len(user) // 4) + 40
        self._send(200, {
            "id": "mock-1", "object": "chat.completion", "model": body.get("model", "frontier-large"),
            "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": toks - 40, "completion_tokens": 40, "total_tokens": toks},
        }, {"x-est-cost-usd": f"{toks / 1000 * PRICE:.5f}"})

ThreadingHTTPServer(("0.0.0.0", int(os.environ.get("PORT", "8000"))), H).serve_forever()
