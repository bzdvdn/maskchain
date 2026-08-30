# MaskChain Examples

Prod-like local dev stack for testing mask/unmask flows.

## Quick Start

Two deployment modes — choose one:

### Split mode (separate gateway + admin containers)

```bash
docker compose -f docker-compose.split.yml up -d --build
```

### Combined mode (gateway + admin in one container)

```bash
docker compose -f docker-compose.all.yml up -d --build
```

### After starting either mode

```bash
# Seed tenant dictionaries (500 users, 50 depts, 300 projects)
./seed-tenant.sh

# Open test-prompt.md for Postman test prompts
```

## Services

### Split mode (`docker-compose.split.yml`)

| Service    | URL                          | Auth                          |
|------------|------------------------------|-------------------------------|
| Gateway    | http://localhost:8080        | Bearer <virtual-key>          |
| Admin      | http://localhost:9090        | Bearer <virtual-key> / admin session |

### Combined mode (`docker-compose.all.yml`)

| Service    | URL                          | Auth                          |
|------------|------------------------------|-------------------------------|
| Gateway    | http://localhost:8080        | Bearer <virtual-key>          |
| Admin      | http://localhost:9090        | Session cookie                |

> **Auth model (key-at-rest + virtual keys):** каждый tenant теперь аутентифицируется
> виртуальным ключом, который создаётся через admin API `/api/v1/keys`. На сервере
> хранится только SHA-256 hash ключа (не raw-значение), а сам raw показан ровно один
> раз при создании. YAML-ключи из `config.yaml` бутстрапятся в виртуальные на старте,
> поэтому `sk-test-default` из конфига продолжает работать как раньше.

### Both modes

| Service    | URL                          | Notes                         |
|------------|------------------------------|-------------------------------|
| Postgres   | postgres://test:test@localhost:5433/maskchain | port 5433 (avoid clash) |
| Valkey     | localhost:6379               | —                             |
| Grafana    | http://localhost:3000        | admin/admin (anonymous enabled) |
| Prometheus | http://localhost:9091        | —                             |

## Tenant: `default`

Pre-configured in `config.yaml` and synced to DB on startup.
Dictionaries are seeded by `seed-tenant.sh` into the tenant.
PII rules per-tenant via PIIConfig (email, phone, SSN — block by default).

Contains 3 dictionaries (exact match):

- **users** — 500 names (e.g. `James LastName42`, `Mary LastName99`)
- **departments** — 50 entries (e.g. `Engineering #1`, `Marketing #2`)
- **projects** — 300 entries (e.g. `Project-42`, `Project-15`)

PIIConfig rules:

| Rule   | Type | Action |
|--------|------|--------|
| email  | pii  | block  |
| phone  | pii  | block  |
| ssn    | pii  | block  |

Default action on engine error: `block`.

## Test Flows

See `test-prompt.md` for detailed Postman requests.

### Flow A: Mask/Unmask — PII Regex + Dictionary
1. `POST /api/v1/shield/mask` with raw CSV body → PII regex catches emails/phones/SSNs + dictionary values masked
2. `POST /api/v1/shield/unmask?mask_ids=<X-Mask-ID>` to restore
3. PII rules are per-tenant via PIIConfig (email/phone/SSN block)

### Flow B: Shield scan with dictionary matching
1. `POST /v1/chat/completions` with `Authorization: Bearer sk-test-default` (a virtual key bootstrapped from YAML)
2. Tenant `default` is identified from the virtual key's SHA-256 hash, its PIIConfig rules + dictionaries are used
3. Response shows `X-Shield-Status: suspicious` (dictionary match) or `X-Shield-Status: blocked` (PII block)

## Tenants

### Via YAML

Tenants are defined in `config.yaml` (mounted into containers). On startup they are synced to DB:

```yaml
tenants:
  default:
    name: "Default Tenant"
    auth_header: "Authorization"
    api_keys:
      - "sk-test-default"
    pii_config:
      enabled: true
      default_action: block
      rules:
        - label: email
          type: pii
          pattern: EMAIL
          action: block
        - label: phone
          type: pii
          pattern: PHONE
          action: block
        - label: ssn
          type: pii
          pattern: SSN
          action: block
```

| Field        | Type     | Description |
|--------------|----------|-------------|
| `auth_header` | string  | HTTP header for the virtual key (default: `X-Mask-Authorization`) |
| `api_keys`    | []string | Raw keys **bootstrap into virtual keys on startup** (idempotent). Not stored raw — only SHA-256 hashed. Keep for static YAML tenants; rotate/add on the fly via the admin API instead. |
| `pii_config`  | object  | PII rules per-tenant (enabled, default_action, rules) |

### Via API (virtual keys)

Create a tenant at runtime, then issue a **virtual key** for it (the old
`api_keys` field is gone from the tenant API — keys now live under `/api/v1/keys`):

```bash
# 1) Create the tenant (no api_keys field anymore)
curl -X POST http://localhost:9090/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "slug": "acme-corp",
    "name": "Acme Corp",
    "auth_header": "X-Acme-Key",
    "dictionaries": [
      {"name": "employees", "entries": ["Alice","Bob"], "match_mode": "exact"}
    ],
    "pii_config": {
      "enabled": true,
      "default_action": "block",
      "rules": [
        {"label": "ssn", "type": "pii", "pattern": "SSN", "action": "block"}
      ]
    }
  }'
```

```bash
# 2) Create a virtual key for the tenant.
#    Response contains `key` (the raw secret) exactly once — save it now.
curl -X POST http://localhost:9090/api/v1/keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "tenant_id": "acme-corp",
    "label": "prod-app",
    "allowed_models": ["gpt-4o"],
    "budget_cap": 10.0
  }'
```

Use the returned raw `key` as `Authorization: Bearer <key>` on the gateway. Only
its SHA-256 hash is stored; revoke it any time via `DELETE /api/v1/keys/:id`.
