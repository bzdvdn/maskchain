# Protect PII in 5 Minutes with MaskChain

A practical walkthrough from zero to seeing masked sensitive data.

## Prerequisites

- Docker and Docker Compose
- curl

## Step 1: Start the stack

```bash
git clone https://github.com/bzdvdn/maskchain.git
cd maskchain/examples/quickstart
./quickstart.sh
```

This builds the combined binary and starts the gateway (port 8080), admin API and
UI (port 9090), Postgres, Valkey, and a local Ollama provider. The first build can
take a few minutes. Wait for readiness:

```bash
curl -s http://localhost:8080/health     # {"status":"ok"}
```

The quickstart pre-configures a `default` tenant with key `sk-test-default` and
reversible PII masking for email/phone/SSN.

## Step 2: Send a prompt with PII

```bash
curl -s -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "model": "llama3.2",
    "messages": [{"role": "user", "content": "My email is john@example.com and SSN is 123-45-6789"}]
  }'
```

The shield scans the request body, replaces PII with placeholders before the
request is forwarded, and restores the original values on the response path.
Inspect the shield outcome in the response headers:

```
X-Shield-Status: suspicious
```

To test the shield in isolation (no LLM call), use the mask endpoint:

```bash
curl -s -D - -X POST http://localhost:8080/api/v1/shield/mask \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: text/plain" \
  --data "Contact me at john@example.com or 123-45-6789"
```

Expected body (default `format=clean`):

```
Contact me at [MASK.1] or [MASK.2]
```

The response header `mask-id` restores the text later:

```bash
curl -s -X POST "http://localhost:8080/api/v1/shield/unmask?mask_ids=<mask-id>" \
  -H "Content-Type: text/plain" \
  --data "Contact me at [MASK.1] or [MASK.2]"
```

See [cookbook/01-mask-unmask](../examples/cookbook/01-mask-unmask/) for the
`clean`/`id`/`redact` formats.

## Step 3: Send a clean prompt

```bash
curl -s -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{"model": "llama3.2", "messages": [{"role": "user", "content": "What is the capital of France?"}]}'
```

The response header shows `X-Shield-Status: clean` — no findings, no masking.

## Step 4: Create another tenant (optional)

Tenants are created without inline API keys; issue a virtual key separately.

```bash
# Admin session token
TOKEN=$(curl -s -X POST http://localhost:9090/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"test"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# Create the tenant
curl -s -X POST http://localhost:9090/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "slug": "demo",
    "name": "Demo Tenant",
    "auth_header": "Authorization",
    "pii_config": {
      "enabled": true,
      "default_action": "mask",
      "rules": [
        {"label": "email", "type": "regex", "pattern": "EMAIL", "action": "mask"},
        {"label": "phone", "type": "regex", "pattern": "PHONE", "action": "mask"},
        {"label": "ssn",   "type": "regex", "pattern": "SSN",   "action": "block"}
      ]
    }
  }'

# Issue a key (raw secret shown once)
curl -s -X POST http://localhost:9090/api/v1/keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"tenant_id": "demo", "label": "tutorial", "allowed_models": ["llama3.2"]}'
```

Use the returned `key` as `Authorization: Bearer <key>` on the gateway. Note the
`ssn` rule uses `action: block`: a request containing an SSN returns HTTP 403
with `X-Shield-Status: blocked`.

## Step 5: Check analytics

```bash
curl -s "http://localhost:9090/api/v1/analytics/tokens?period=day" \
  -H "Authorization: Bearer $TOKEN"
```

The UI shows the same data at <http://localhost:9090> (**Overview → Analytics**).

## What just happened?

```
Client --> Auth --> Rate Limit --> Shield Scan --> Routing --> LLM Provider --> Response --> Unmask
```

1. **Auth** — the key resolves the tenant and loads its PII config/dictionaries.
2. **Rate Limit** — the per-tenant request budget is checked.
3. **Shield Scan** — the body is scanned by the PII regex detectors and the
   tenant's dictionaries; matches become reversible placeholders.
4. **Routing** — the masked request is forwarded to the provider for the model.
5. **Unmask** — on the response path, placeholders are restored. Streaming SSE
   responses are unmasked chunk-by-chunk. `block` rules short-circuit with 403.

## Next steps

- [examples/README.md](../examples/README.md) — quickstart, cookbook, clients, Postman
- [docs/DEPLOYMENT.md](DEPLOYMENT.md) — production setup (Helm, bare binary)
- [docs/SHIELD.md](SHIELD.md) — content shield architecture deep-dive
- [CONTRIBUTING.md](../CONTRIBUTING.md) — development setup and custom detectors
