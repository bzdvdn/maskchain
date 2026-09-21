# Clients

Minimal, dependency-light examples that call MaskChain exactly like an
OpenAI-compatible endpoint.

| Language | File | Run |
|---|---|---|
| Python (OpenAI SDK) | [`python/example.py`](python/example.py) | `pip install openai && python example.py` |
| Node 18+ (fetch) | [`node/example.mjs`](node/example.mjs) | `node example.mjs` |
| Go (stdlib) | [`go/main.go`](go/main.go) | `go run main.go` |

All three read the same environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `MASKCHAIN_URL` | `http://localhost:8080/api/v1` | Gateway base URL |
| `MASKCHAIN_KEY` | `sk-test-default` | Tenant / virtual key |
| `MASKCHAIN_MODEL` | `llama3.2` | Model routed in your config |

Point `MASKCHAIN_URL` at the `/api/v1` prefix: MaskChain also serves `/v1/*`
redirects, but clients should use the canonical path.

When switching to OpenAI, set `MASKCHAIN_MODEL=gpt-4o-mini`.

> Note: `keys`, `budgets`, `routing`, and `audit` admin endpoints require an
> **admin session token**, not a tenant key. See
> [`cookbook/05-keys-and-budgets`](../cookbook/05-keys-and-budgets/).
