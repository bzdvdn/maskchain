# 04 — Provider routing, fallback & circuit breaker

**Goal:** route a model to one or more providers, with automatic fallback and a
circuit breaker on failing upstreams.

Prereq: [quickstart](../../quickstart/) is running.

## Declarative (config)

```yaml
routing:
  providers:
    - {name: openai,  api_type: openai,  base_url: "https://api.openai.com",
       api_keys: ["${OPENAI_KEY}"], timeout: 60s, priority: 1}
    - {name: groq,    api_type: proxy,   base_url: "https://api.groq.com/openai/v1",
       api_keys: ["${GROQ_API_KEY}"], timeout: 60s, priority: 2}
  rules:
    - tenant: default
      routes:
        # Tried left-to-right; a failing provider falls through to the next.
        - model: "gpt-4o-mini"
          providers: ["openai", "groq"]
```

- `api_type`: `openai | anthropic | gemini | bedrock | proxy | ollama`.
- `proxy` = any OpenAI-compatible endpoint (Groq, Mistral, vLLM, …).
- Egress retry and the circuit breaker are tuned under `egress:`:

```yaml
egress:
  max_retries: 2
  base_backoff: 100ms
  circuit_breaker: {max_failures: 5, cooldown: 30s}
```

- Per-provider egress proxy: `proxy_url: http://corp-proxy:3128` (HTTP/SOCKS5).

## Runtime CRUD (admin session)

```bash
ADMIN=http://localhost:9090
TOKEN=$(curl -s -X POST "$ADMIN/api/v1/admin/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"test"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# Upsert a provider
curl -s -X PUT "$ADMIN/api/v1/routing/providers" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"groq","api_type":"proxy","base_url":"https://api.groq.com/openai/v1",
       "api_keys":["'"${GROQ_API_KEY}"'"],"timeout":"60s","priority":2}'

# Upsert a route with fallback order
curl -s -X PUT "$ADMIN/api/v1/routing/routes" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"tenant":"default","model":"gpt-4o-mini","providers":["openai","groq"]}'

# Health + latency
curl -s "$ADMIN/api/v1/routing/providers" -H "Authorization: Bearer $TOKEN"
```

Provider secrets are stored encrypted at rest and returned masked (`sk-***xyz`).

## Notes

- `NO_ROUTE` (404) means no route matched `(tenant, model)`.
- A tripped breaker skips a provider until `cooldown` elapses, then half-opens.
- Runtime routing changes are persisted in Postgres and hot-applied.
