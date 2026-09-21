# 08 — Conversation logging (encrypted, per-tenant retention)

**Goal:** persist request/response pairs with mask-proof mappings for audit,
encrypted at rest.

Prereq: [quickstart](../../quickstart/) is running.

## Enable

Conversation logging needs a second encryption key and must be turned on.

1. Generate a key and add it to `examples/quickstart/.env`:

```bash
printf 'MASKCHAIN_CONVERSATION_KEY=%s\n' "$(openssl rand -base64 32)" >> .env
```

2. In `config-base.yaml`, set:

```yaml
conversations:
  enabled: true
  retention_days: 90
```

3. Restart:

```bash
docker compose -f ../quickstart/docker-compose.ollama.yml up -d maskchain
```

Without `MASKCHAIN_CONVERSATION_KEY` the service **fails closed** and will not start.

## Inspect

```bash
ADMIN=http://localhost:9090
TOKEN=$(curl -s -X POST "$ADMIN/api/v1/admin/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"test"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# List metadata (filter by tenant/model/status/masked)
curl -s "$ADMIN/api/v1/conversations?tenant_id=default&limit=20" \
  -H "Authorization: Bearer $TOKEN"

# Detail (decrypted payload + mask mappings)
curl -s "$ADMIN/api/v1/conversations/<id>" -H "Authorization: Bearer $TOKEN"
```

## Retention modes

Global data retention is set with `data.retention.mode` (`full` by default;
zero-retention deployments set a non-logging mode per tenant). Conversation
rows older than `retention_days` are purged.

## Notes

- Payloads are AES-256-GCM encrypted with `MASKCHAIN_CONVERSATION_KEY`.
- The UI surfaces the same data under **Traffic → Conversations**.
