# Protect PII in 5 Minutes with MaskChain

A practical walkthrough from zero to seeing masked sensitive data.

## Prerequisites

- Docker and Docker Compose
- curl

## Step 1: Start the stack

```bash
git clone https://github.com/bzdvdn/maskchain.git
cd maskchain
docker compose -f examples/docker-compose.yml up -d --build
```

This starts the gateway (port 8080), admin API (port 8082), Postgres, Valkey, Prometheus, and Grafana. Wait for the gateway to be ready:

```bash
curl -s http://localhost:8080/health
```

Expected response: `{"status":"ok"}`

## Step 2: Create a tenant with PII rules

Create a tenant called "demo" with PII detection for email, phone, and SSN. The `mask` action replaces sensitive values with placeholders; `block` returns a 403.

```bash
curl -s -X POST http://localhost:8082/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "slug": "demo",
    "name": "Demo Tenant",
    "auth_header": "Authorization",
    "api_keys": ["sk-demo"],
    "pii_config": {
      "enabled": true,
      "default_action": "mask",
      "rules": [
        {"label": "email", "type": "regex", "pattern": "EMAIL", "action": "mask"},
        {"label": "phone", "type": "regex", "pattern": "PHONE", "action": "mask"},
        {"label": "ssn",   "type": "regex", "pattern": "SSN",   "action": "mask"}
      ]
    }
  }'
```

Expected response (HTTP 200):

```json
{
  "slug": "demo",
  "name": "Demo Tenant",
  "api_keys": ["sk-demo"],
  "pii_config": {
    "enabled": true,
    "default_action": "mask",
    "rules": [
      {"label": "email", "action": "mask"},
      {"label": "phone", "action": "mask"},
      {"label": "ssn",   "action": "mask"}
    ]
  }
}
```

## Step 3: Send a prompt with PII

Send a chat request containing an email address and SSN. The shield scans the request body, detects the PII patterns, and replaces them with placeholders before the request is forwarded to the LLM provider.

```bash
curl -s -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-demo" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "My email is john@example.com and SSN is 123-45-6789"}]
  }'
```

When the shield detects PII with `action: mask`, the request body sent to the LLM looks like this:

```
My email is [[pii.email.0]] and SSN is [[pii.ssn.0]]
```

On the response path, the shield's unmask writer restores the original values, so the caller receives the LLM response with original text intact. If no LLM provider is reachable, you will still see shield activity in the response headers:

```
X-Shield-Status: suspicious
X-Shield-Findings: 2
```

To test the shield in isolation without an LLM, use the dedicated mask endpoint:

```bash
curl -s -X POST http://localhost:8080/api/v1/shield/mask \
  -H "Authorization: Bearer sk-demo" \
  -d "Contact me at john@example.com or 123-45-6789"
```

Expected response:

```
Contact me at [[pii.email.0]] or [[pii.ssn.0]]
```

The response header includes a `data_mask_id` you can use to unmask later:

```bash
# Unmask the masked text (pass mask_id from response header)
curl -s -X POST http://localhost:8080/api/v1/shield/unmask?mask_ids=<MASK_ID> \
  -H "Authorization: Bearer sk-demo" \
  -d "Contact me at [[pii.email.0]] or [[pii.ssn.0]]"
```

## Step 4: Send a clean prompt

A request without PII passes through the shield unmodified.

```bash
curl -s -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-demo" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
  }'
```

The response header will show `X-Shield-Status: clean` -- no findings, no masking, no latency overhead.

## Step 5: Check analytics

MaskChain records every request with shield outcomes, token counts, and latency. Query the analytics endpoint to see usage data:

```bash
curl -s http://localhost:8082/api/v1/analytics/tokens?period=day \
  -H "Authorization: Bearer sk-demo"
```

Expected response (values will vary):

```json
{
  "records": [
    {
      "tenant": "demo",
      "period": "2026-07-19",
      "prompt_tokens": 42,
      "completion_tokens": 15,
      "total_tokens": 57,
      "shield_findings": 2
    }
  ]
}
```

## What just happened?

Each request flows through this pipeline:

```
Client --> Auth --> Rate Limit --> Shield Scan --> Routing --> LLM Provider --> Response --> Unmask
```

1. **Auth** -- the API key (`sk-demo`) identifies the tenant and loads its config.
2. **Rate Limit** -- per-tenant request budget is checked (if configured).
3. **Shield Scan** -- the request body is scanned by the PII engine (regex detectors for email, phone, SSN). Matches are replaced with `[[pii.<label>.<N>]]` placeholders. Dictionary terms (if any) are masked with `[MASK_<ID>.<N>]` placeholders.
4. **Routing** -- the modified request is forwarded to the configured LLM provider based on the model name.
5. **Unmask** -- on the response path, placeholders are restored to original values. Streaming SSE responses are unmasked chunk-by-chunk.

If `action: block` is set instead of `mask`, the shield returns a 403 before the request reaches the LLM, and the caller receives `X-Shield-Status: blocked`.

## Next steps

- Browse [examples/](../examples/README.md) for more test flows (dictionary masking, streaming, seed scripts)
- Read [docs/DEPLOYMENT.md](DEPLOYMENT.md) for production setup (HA, env vars, Kubernetes)
- See [CONTRIBUTING.md](../CONTRIBUTING.md) for development setup and how to add custom detectors
- Dive into [docs/SHIELD.md](SHIELD.md) for the architecture deep-dive on the content shield pipeline
