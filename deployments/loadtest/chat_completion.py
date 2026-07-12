#!/usr/bin/env python3
"""Load test for /v1/chat/completions endpoint."""

import os
import sys
import time
import json
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.request import Request, urlopen
from urllib.error import URLError

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8080")
ADMIN_TOKEN = os.environ.get("ADMIN_TOKEN", "")
CONCURRENCY = int(os.environ.get("CONCURRENCY", "10"))
REQUESTS = int(os.environ.get("REQUESTS", "50"))

PAYLOAD = json.dumps({
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello, tell me a short story about a robot."}],
    "max_tokens": 100,
    "stream": False,
}).encode()

_stats = {"ok": 0, "fail": 0, "latencies": []}
_lock = threading.Lock()


def health_check() -> bool:
    try:
        req = Request(f"{GATEWAY_URL}/health")
        with urlopen(req, timeout=10) as resp:
            return resp.status == 200
    except Exception as e:
        print(f"Health check failed: {e}", file=sys.stderr)
        return False


def send_request(_id: int):
    headers = {"Content-Type": "application/json"}
    if ADMIN_TOKEN:
        headers["X-Admin-Token"] = ADMIN_TOKEN

    req = Request(
        f"{GATEWAY_URL}/v1/chat/completions",
        data=PAYLOAD,
        headers=headers,
        method="POST",
    )

    start = time.monotonic()
    try:
        with urlopen(req, timeout=30) as resp:
            body = resp.read()
            data = json.loads(body)
            ok = resp.status == 200 and data.get("choices") and len(data["choices"]) > 0
    except Exception:
        ok = False

    elapsed = (time.monotonic() - start) * 1000  # ms

    with _lock:
        if ok:
            _stats["ok"] += 1
        else:
            _stats["fail"] += 1
        _stats["latencies"].append(elapsed)

    return ok, elapsed


def print_summary(total_time: float):
    latencies = sorted(_stats["latencies"])
    total = len(latencies)
    p50 = latencies[total // 2] if total else 0
    p95 = latencies[int(total * 0.95)] if total else 0
    p99 = latencies[int(total * 0.99)] if total else 0
    rps = total / total_time if total_time > 0 else 0

    print(f"\n{'='*50}")
    print(f"Total requests:  {total}")
    print(f"OK:              {_stats['ok']}")
    print(f"Failed:          {_stats['fail']}")
    print(f"Total time:      {total_time:.2f}s")
    print(f"RPS:             {rps:.1f}")
    print(f"p50 latency:     {p50:.1f}ms")
    print(f"p95 latency:     {p95:.1f}ms")
    print(f"p99 latency:     {p99:.1f}ms")
    print(f"{'='*50}")

    if _stats["fail"] > 0:
        sys.exit(1)


def main():
    print(f"Gateway URL: {GATEWAY_URL}")
    print(f"Concurrency: {CONCURRENCY}")
    print(f"Requests:    {REQUESTS}")

    if not health_check():
        print("Gateway is not healthy, aborting.", file=sys.stderr)
        sys.exit(1)

    print("Health check passed. Starting load test...")

    start = time.monotonic()
    with ThreadPoolExecutor(max_workers=CONCURRENCY) as pool:
        futures = [pool.submit(send_request, i) for i in range(REQUESTS)]
        for f in as_completed(futures):
            pass  # stats collected inside
    total_time = time.monotonic() - start

    print_summary(total_time)


if __name__ == "__main__":
    main()
