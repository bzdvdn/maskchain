# 05 — Virtual keys & budgets

**Goal:** issue scoped keys per team/app and enforce spend limits.

Prereq: [quickstart](../../quickstart/) is running. Key/budget endpoints require
an **admin session** (not a tenant key).

```bash
ADMIN=http://localhost:9090
TOKEN=$(curl -s -X POST "$ADMIN/api/v1/admin/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"test"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
AUTH="Authorization: Bearer $TOKEN"
```

## Create a scoped key

The raw secret is returned **exactly once** — save it now.

```bash
curl -s -X POST "$ADMIN/api/v1/keys" -H "$AUTH" -H 'Content-Type: application/json' -d '{
  "tenant_id": "default",
  "label": "prod-app",
  "allowed_models": ["llama3.2"],
  "budget_cap": 10.0,
  "metadata": {"team": "platform"}
}'
```

Use the returned `key` on the gateway: `Authorization: Bearer <key>`.
Only its SHA-256 hash is stored server-side. Revoke with
`DELETE /api/v1/keys/:id`; list with `GET /api/v1/keys`.

## Create budgets

Scope is `tenant | key | model`; type is `monthly | daily | custom`.

```bash
# Tenant-wide monthly cap with alerts at 50% and 90%
curl -s -X POST "$ADMIN/api/v1/budgets" -H "$AUTH" -H 'Content-Type: application/json' -d '{
  "tenant_id": "default",
  "scope": "tenant",
  "type": "monthly",
  "soft_limit": 8.0,
  "hard_limit": 10.0,
  "currency": "USD",
  "notify_at": [50, 90]
}'

# Inspect
curl -s "$ADMIN/api/v1/budgets/tenants/default" -H "$AUTH"
curl -s "$ADMIN/api/v1/budgets/<id>/history" -H "$AUTH"
```

## What enforcement does

| Limit | Behavior |
|---|---|
| `hard_limit` reached | request rejected **before** the provider call — HTTP 429 |
| `soft_limit` reached | request allowed; alert webhook fired |
| `notify_at` % | webhook alert per threshold |

Optional global webhook: `budgets.alert_webhook_url` in config.

## Notes

- Cost comes from `analytics.cost_rates`; models without a rate count as 0.
- Streaming responses are not counted toward spend (see `docs/ACCESS_AND_BUDGETS.md`).
